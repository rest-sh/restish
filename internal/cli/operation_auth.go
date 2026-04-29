package cli

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

type operationAuthPolicy struct {
	OptionalAuth           bool
	CredentialAlternatives []spec.CredentialAlternative
	Override               string
}

type selectedOperationAuth struct {
	requirement spec.CredentialRequirement
	resolved    resolvedAuthConfig
}

func (c *CLI) planOperationAuth(apiName, profileName string, prof *config.ProfileConfig, policy *operationAuthPolicy) ([]selectedOperationAuth, bool, error) {
	if policy != nil && strings.TrimSpace(policy.Override) != "" {
		return c.planOperationAuthOverride(apiName, profileName, prof, policy)
	}
	if policy == nil || len(policy.CredentialAlternatives) == 0 {
		return nil, false, nil
	}
	if prof == nil {
		if policy.OptionalAuth {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("operation requires credentials for API %q but profile %q is not configured", apiName, profileName)
	}

	var missing []string
	var needErrors []string
	for _, alternative := range policy.CredentialAlternatives {
		selected := make([]selectedOperationAuth, 0, len(alternative))
		alternativeMissing := false
		alternativeNeedErrors := false
		for _, requirement := range alternative {
			credential := prof.Credentials[requirement.ID]
			if credential == nil {
				alternativeMissing = true
				missing = append(missing, requirement.ID)
				continue
			}
			if err := credentialSatisfies(requirement, credential); err != nil {
				alternativeNeedErrors = true
				needErrors = append(needErrors, err.Error())
				continue
			}
			resolved, err := c.resolveCredentialAuth(apiName, profileName, requirement.ID, credential)
			if err != nil {
				return nil, false, err
			}
			if resolved.Config == nil {
				alternativeMissing = true
				missing = append(missing, requirement.ID)
				continue
			}
			selected = append(selected, selectedOperationAuth{requirement: requirement, resolved: resolved})
		}
		if !alternativeMissing && !alternativeNeedErrors {
			if err := rejectConflictingSelectedAuth(selected); err != nil {
				return nil, false, err
			}
			return selected, true, nil
		}
	}

	if canUseProfileAuthFallback(policy) {
		resolved, err := c.resolveProfileAuth(apiName, profileName, prof)
		if err != nil {
			return nil, false, err
		}
		if resolved.Config != nil {
			return []selectedOperationAuth{{requirement: policy.CredentialAlternatives[0][0], resolved: resolved}}, true, nil
		}
	}
	if policy.OptionalAuth {
		return nil, true, nil
	}

	if len(needErrors) > 0 {
		sort.Strings(needErrors)
		return nil, false, fmt.Errorf("profile %q of API %q has credential bindings that do not satisfy this operation: %s", profileName, apiName, strings.Join(uniqueStrings(needErrors), "; "))
	}
	sort.Strings(missing)
	return nil, false, fmt.Errorf("profile %q of API %q is missing credential bindings for this operation: %s", profileName, apiName, strings.Join(uniqueStrings(missing), ", "))
}

func (c *CLI) planOperationAuthOverride(apiName, profileName string, prof *config.ProfileConfig, policy *operationAuthPolicy) ([]selectedOperationAuth, bool, error) {
	override := strings.TrimSpace(policy.Override)
	if strings.EqualFold(override, "anonymous") {
		if policy.OptionalAuth {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf(`security override "anonymous" is not valid for this operation`)
	}
	requested, err := parseSecurityOverride(override)
	if err != nil {
		return nil, false, err
	}
	alternative, ok := matchingSecurityAlternative(policy.CredentialAlternatives, requested)
	if !ok {
		return nil, false, fmt.Errorf("security override %q does not match this operation; valid values: %s", override, strings.Join(securityOverrideCandidates(policy.OptionalAuth, policy.CredentialAlternatives), ", "))
	}
	if prof == nil {
		return nil, false, fmt.Errorf("security override %q requires profile %q of API %q to configure credential bindings", override, profileName, apiName)
	}
	selected, missing, needErrors, err := c.selectOperationAlternative(apiName, profileName, prof, alternative)
	if err != nil {
		return nil, false, err
	}
	if len(needErrors) > 0 {
		sort.Strings(needErrors)
		return nil, false, fmt.Errorf("security override %q is not satisfied by profile %q of API %q: %s", override, profileName, apiName, strings.Join(uniqueStrings(needErrors), "; "))
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, false, fmt.Errorf("security override %q requires missing credential bindings in profile %q of API %q: %s", override, profileName, apiName, strings.Join(uniqueStrings(missing), ", "))
	}
	if err := rejectConflictingSelectedAuth(selected); err != nil {
		return nil, false, err
	}
	return selected, true, nil
}

func (c *CLI) selectOperationAlternative(apiName, profileName string, prof *config.ProfileConfig, alternative spec.CredentialAlternative) ([]selectedOperationAuth, []string, []string, error) {
	selected := make([]selectedOperationAuth, 0, len(alternative))
	var missing []string
	var needErrors []string
	for _, requirement := range alternative {
		credential := prof.Credentials[requirement.ID]
		if credential == nil {
			missing = append(missing, requirement.ID)
			continue
		}
		if err := credentialSatisfies(requirement, credential); err != nil {
			needErrors = append(needErrors, err.Error())
			continue
		}
		resolved, err := c.resolveCredentialAuth(apiName, profileName, requirement.ID, credential)
		if err != nil {
			return nil, nil, nil, err
		}
		if resolved.Config == nil {
			missing = append(missing, requirement.ID)
			continue
		}
		selected = append(selected, selectedOperationAuth{requirement: requirement, resolved: resolved})
	}
	return selected, missing, needErrors, nil
}

func (c *CLI) resolveCredentialAuth(apiName, profileName, credentialID string, credential *config.CredentialConfig) (resolvedAuthConfig, error) {
	if credential == nil {
		return resolvedAuthConfig{}, nil
	}
	if credential.Auth != nil && credential.AuthRef != "" {
		return resolvedAuthConfig{}, fmt.Errorf("credential %q in profile %q of API %q has both auth and auth_ref", credentialID, profileName, apiName)
	}
	if credential.AuthRef == "" {
		return resolvedAuthConfig{
			Config:   credential.Auth,
			CacheKey: apiName + ":" + profileName + ":credential:" + credentialID,
		}, nil
	}
	if c.cfg == nil || c.cfg.AuthProfiles == nil || c.cfg.AuthProfiles[credential.AuthRef] == nil {
		return resolvedAuthConfig{}, fmt.Errorf("credential %q in profile %q of API %q references unknown auth profile %q", credentialID, profileName, apiName, credential.AuthRef)
	}
	ac := c.cfg.AuthProfiles[credential.AuthRef]
	return resolvedAuthConfig{
		Config:   ac,
		Ref:      credential.AuthRef,
		CacheKey: sharedAuthCacheKey(credential.AuthRef, ac),
	}, nil
}

func (c *CLI) operationAuthCallbacks(apiName, profileName string, selected []selectedOperationAuth, opts authHandlerOptions) (authCallbacks, error) {
	if len(selected) == 0 {
		return authCallbacks{}, nil
	}
	steps := make([]operationAuthStep, 0, len(selected))
	for _, item := range selected {
		step, err := c.operationAuthStep(apiName, profileName, item, opts)
		if err != nil {
			return authCallbacks{}, err
		}
		steps = append(steps, step)
	}
	callbacks := authCallbacks{
		OnRequest: func(req *http.Request) error {
			for _, step := range steps {
				if err := c.applyOperationAuthStep(req, step, false); err != nil {
					return err
				}
			}
			return nil
		},
	}
	for _, step := range steps {
		if step.forceCapable {
			callbacks.OnUnauthorized = func(req *http.Request) error {
				for _, step := range steps {
					if err := c.applyOperationAuthStep(req, step, step.forceCapable); err != nil {
						return err
					}
				}
				return nil
			}
			break
		}
	}
	return callbacks, nil
}

type operationAuthStep struct {
	handler      auth.Handler
	rawParams    map[string]string
	cacheKey     string
	forceCapable bool
	secretKeys   map[string]bool
	apiName      string
	profileName  string
	authType     string
}

func (c *CLI) operationAuthStep(apiName, profileName string, selected selectedOperationAuth, opts authHandlerOptions) (operationAuthStep, error) {
	handler, err := c.authHandlerFor(selected.resolved.Config, opts)
	if err != nil {
		return operationAuthStep{}, err
	}
	secretKeys := make(map[string]bool)
	for _, p := range handler.Parameters() {
		if p.Secret {
			secretKeys[p.Name] = true
		}
	}
	_, forceCapable := handler.(auth.ForceCapable)
	return operationAuthStep{
		handler:      handler,
		rawParams:    selected.resolved.Config.Params,
		cacheKey:     selected.resolved.CacheKey,
		forceCapable: forceCapable,
		secretKeys:   secretKeys,
		apiName:      apiName,
		profileName:  profileName,
		authType:     selected.resolved.Config.Type,
	}, nil
}

func (c *CLI) applyOperationAuthStep(req *http.Request, s operationAuthStep, force bool) error {
	params, err := c.buildAuthParams(s.rawParams)
	if err != nil {
		return err
	}
	if s.authType == "external-tool" {
		if err := c.ensureExternalToolApproved(req.Context(), s.apiName, s.profileName, params["commandline"]); err != nil {
			return err
		}
	}
	if err := s.handler.Authenticate(req.Context(), req, c.authContext(req.Context(), s.apiName, s.profileName, params, s.cacheKey, force)); err != nil {
		return err
	}
	return c.runAuthHookPlugins(s.apiName, s.profileName, s.rawParams, s.secretKeys, req)
}

func credentialSatisfies(requirement spec.CredentialRequirement, credential *config.CredentialConfig) error {
	if len(requirement.Needs) == 0 {
		return nil
	}
	have := make(map[string]bool, len(credential.Satisfies))
	for _, value := range credential.Satisfies {
		have[value] = true
	}
	var missing []string
	for _, need := range requirement.Needs {
		if !have[need] {
			missing = append(missing, need)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s missing required values %s", requirement.ID, strings.Join(missing, ", "))
	}
	return nil
}

func canUseProfileAuthFallback(policy *operationAuthPolicy) bool {
	return policy != nil &&
		len(policy.CredentialAlternatives) == 1 &&
		len(policy.CredentialAlternatives[0]) == 1
}

func parseSecurityOverride(value string) (map[string]bool, error) {
	parts := strings.Split(value, "+")
	out := make(map[string]bool, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			return nil, fmt.Errorf("invalid security override %q", value)
		}
		if out[id] {
			return nil, fmt.Errorf("invalid security override %q: duplicate credential %q", value, id)
		}
		out[id] = true
	}
	return out, nil
}

func matchingSecurityAlternative(alternatives []spec.CredentialAlternative, requested map[string]bool) (spec.CredentialAlternative, bool) {
	for _, alternative := range alternatives {
		if len(alternative) != len(requested) {
			continue
		}
		matches := true
		for _, requirement := range alternative {
			if !requested[requirement.ID] {
				matches = false
				break
			}
		}
		if matches {
			return alternative, true
		}
	}
	return nil, false
}

func securityOverrideCandidates(optional bool, alternatives []spec.CredentialAlternative) []string {
	candidates := make([]string, 0, len(alternatives)+1)
	for _, alternative := range alternatives {
		var ids []string
		for _, requirement := range alternative {
			ids = append(ids, requirement.ID)
		}
		if len(ids) > 0 {
			candidates = append(candidates, strings.Join(ids, "+"))
		}
	}
	if optional {
		candidates = append(candidates, "anonymous")
	}
	return candidates
}

func rejectConflictingSelectedAuth(selected []selectedOperationAuth) error {
	seen := map[string]string{}
	for _, item := range selected {
		key := authMutationKey(item.resolved.Config)
		if key == "" {
			continue
		}
		if prev := seen[key]; prev != "" {
			return fmt.Errorf("selected credentials %q and %q both write %s", prev, item.requirement.ID, key)
		}
		seen[key] = item.requirement.ID
	}
	return nil
}

func authMutationKey(ac *config.AuthConfig) string {
	if ac == nil {
		return ""
	}
	switch ac.Type {
	case "api-key":
		location := strings.ToLower(ac.Params["in"])
		name := strings.ToLower(ac.Params["name"])
		if location == "" || name == "" {
			return ""
		}
		switch location {
		case "header", "query", "cookie":
			return location + ":" + name
		default:
			return ""
		}
	case "http-basic", "oauth-client-credentials", "oauth-authorization-code", "oauth-device-code":
		return "header:authorization"
	default:
		return ""
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := values[:0]
	var prev string
	for i, value := range values {
		if i > 0 && value == prev {
			continue
		}
		out = append(out, value)
		prev = value
	}
	return out
}
