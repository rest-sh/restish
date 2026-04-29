package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

type configureAuthDiscovery struct {
	info         spec.APIInfo
	sourceURL    string
	schemes      []spec.SecuritySchemeSummary
	operations   []spec.Operation
	opCounts     map[string]int
	needDefaults map[string][]string
}

func newConfigureAuthDiscovery(apiSpec *spec.APISpec, baseURL string) configureAuthDiscovery {
	var d configureAuthDiscovery
	if apiSpec == nil {
		return d
	}
	d.sourceURL = apiSpec.SourceURL
	if info, err := apiSpec.Info(); err == nil {
		d.info = info
	}
	if schemes, err := apiSpec.SecuritySchemeSummaries(); err == nil {
		d.schemes = schemes
	}
	if ops, err := apiSpec.Operations(baseURL, ""); err == nil {
		d.operations = ops
		d.opCounts = credentialOperationCounts(ops)
		d.needDefaults = credentialNeedDefaults(ops)
	}
	return d
}

func (c *CLI) printAPIDiscovery(apiName, baseURL string, d configureAuthDiscovery) {
	title := d.info.Title
	if title == "" {
		title = apiName
	}
	fmt.Fprintf(c.Stdout, "Discovered %s\n", title)
	fmt.Fprintf(c.Stdout, "Base URL: %s\n", baseURL)
	if d.sourceURL != "" {
		fmt.Fprintf(c.Stdout, "OpenAPI:  %s\n", d.sourceURL)
	}
	if len(d.schemes) == 0 {
		fmt.Fprintln(c.Stdout)
		return
	}
	fmt.Fprintln(c.Stdout)
	fmt.Fprintf(c.Stdout, "This API declares %d auth scheme(s):\n\n", len(d.schemes))
	for _, scheme := range d.schemes {
		count := d.opCounts[scheme.ID]
		suffix := ""
		if scheme.GlobalDefault {
			suffix = ", global default"
		}
		if !scheme.Supported {
			if suffix == "" {
				suffix = ", unsupported"
			} else {
				suffix += ", unsupported"
			}
		}
		fmt.Fprintf(c.Stdout, "  %-14s %-32s %3d operations%s\n", scheme.ID, scheme.Detail, count, suffix)
	}
	fmt.Fprintln(c.Stdout)
}

func (c *CLI) configureFallbackAuth(ctx context.Context, apiCfg *config.APIConfig, d configureAuthDiscovery, answers configurePromptAnswers) error {
	if apiCfg == nil || apiCfg.Profiles == nil || apiCfg.Profiles["default"] == nil || len(d.schemes) == 0 {
		return nil
	}
	prof := apiCfg.Profiles["default"]
	if prof.Credentials == nil {
		prof.Credentials = map[string]*config.CredentialConfig{}
	}

	supported := supportedSchemeIDs(d.schemes)
	if len(supported) == 0 {
		return nil
	}

	fmt.Fprintf(c.Stdout, "Configure auth for profile %q.\n\n", "default")
	configured := map[string]bool{}
	for _, scheme := range d.schemes {
		credential := prof.Credentials[scheme.ID]
		if !scheme.Supported || credential == nil || credential.Auth == nil {
			delete(prof.Credentials, scheme.ID)
			continue
		}
		configure := true
		if len(supported) > 1 && !answers.hasCredentialAnswer("default", scheme.ID) {
			def := scheme.GlobalDefault
			ok, err := c.promptYesNoDefault(ctx, fmt.Sprintf("Configure %s? %s ", scheme.ID, yesNoDefaultSuffix(def)), def)
			if err != nil {
				return err
			}
			configure = ok
		}
		if !configure {
			delete(prof.Credentials, scheme.ID)
			if prof.Auth != nil && (scheme.GlobalDefault || len(supported) == 1) {
				prof.Auth = nil
			}
			continue
		}
		defaultNeeds := d.needDefaults[scheme.ID]
		if err := c.promptAuthParams(ctx, "default", scheme.ID, credential.Auth, defaultNeeds, answers); err != nil {
			return err
		}
		if len(defaultNeeds) > 0 {
			credential.Satisfies = defaultNeeds
			if credential.Auth.Params == nil {
				credential.Auth.Params = map[string]string{}
			}
			if credential.Auth.Params["scopes"] == "" {
				credential.Auth.Params["scopes"] = strings.Join(defaultNeeds, " ")
			}
		}
		configured[scheme.ID] = true
		if prof.Auth != nil && (scheme.GlobalDefault || len(supported) == 1) {
			prof.Auth.Params = cloneAuthParams(credential.Auth.Params)
		}
	}
	if len(prof.Credentials) == 0 {
		prof.Credentials = nil
	}
	if len(supported) > 1 && !configuredGlobalScheme(d.schemes, configured) {
		prof.Auth = nil
	}
	c.printAuthCoverage("default", d, configured)
	return nil
}

func configuredGlobalScheme(schemes []spec.SecuritySchemeSummary, configured map[string]bool) bool {
	for _, scheme := range schemes {
		if scheme.GlobalDefault && configured[scheme.ID] {
			return true
		}
	}
	return false
}

func (c *CLI) promptAuthParams(ctx context.Context, profileName, credentialID string, ac *config.AuthConfig, defaultNeeds []string, answers configurePromptAnswers) error {
	if ac == nil {
		return nil
	}
	if ac.Params == nil {
		ac.Params = map[string]string{}
	}
	handler, err := c.authHandlerFor(ac, authHandlerOptions{})
	if err != nil {
		return err
	}
	for _, p := range configureAuthPromptParams(handler, defaultNeeds) {
		if ac.Params[p.Name] != "" {
			continue
		}
		if answer, ok := answers.answerCredential(profileName, credentialID, p.Name); ok {
			ac.Params[p.Name] = strings.TrimSpace(answer)
			if ac.Params[p.Name] == "" && p.Required {
				return fmt.Errorf("%s is required for credential %s", p.Name, credentialID)
			}
			continue
		}
		value, err := c.readAuthParam(ctx, p, defaultNeeds)
		if err != nil {
			return fmt.Errorf("%s %s: %w", credentialID, p.Name, err)
		}
		if value != "" {
			ac.Params[p.Name] = value
		}
	}
	return nil
}

func configureAuthPromptParams(handler auth.Handler, defaultNeeds []string) []auth.Param {
	params := handler.Parameters()
	var out []auth.Param
	for _, p := range params {
		switch p.Name {
		case "in", "name", "token_url", "authorize_url", "issuer_url", "device_authorization_url", "redirect_port", "auth_method":
			continue
		case "password":
			p.Required = true
		case "scopes":
			if len(defaultNeeds) == 0 {
				continue
			}
		}
		if p.Required || p.Name == "scopes" {
			out = append(out, p)
		}
	}
	return out
}

func (c *CLI) readAuthParam(ctx context.Context, p auth.Param, defaultNeeds []string) (string, error) {
	label := authParamLabel(p, defaultNeeds)
	var (
		value string
		err   error
	)
	if p.Secret {
		value, err = c.Secret(ctx, label)
	} else {
		value, err = c.Prompt(ctx, label)
	}
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" && p.Name == "scopes" && len(defaultNeeds) > 0 {
		value = strings.Join(defaultNeeds, " ")
	}
	if value == "" && p.Required {
		return "", fmt.Errorf("required value was empty")
	}
	return value, nil
}

func authParamLabel(p auth.Param, defaultNeeds []string) string {
	switch p.Name {
	case "client_id":
		return "Client ID: "
	case "client_secret":
		return "Client Secret: "
	case "username":
		return "Username: "
	case "password":
		return "Password: "
	case "token":
		return "Token: "
	case "value":
		return "API key: "
	case "scopes":
		return fmt.Sprintf("Scopes [%s]: ", strings.Join(defaultNeeds, " "))
	default:
		if p.Description != "" {
			return p.Description + ": "
		}
		return p.Name + ": "
	}
}

func (c *CLI) promptYesNoDefault(ctx context.Context, label string, def bool) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	fmt.Fprint(c.Stderr, label)
	src, cleanup := c.promptSource()
	defer cleanup()
	line, err := readPromptLine(src)
	if errors.Is(err, io.EOF) && line == "" {
		return false, nil
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("confirm: %w", err)
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	if answer == "" {
		return def, nil
	}
	return answer == "y" || answer == "yes", nil
}

func yesNoDefaultSuffix(def bool) string {
	if def {
		return "[Y/n]"
	}
	return "[y/N]"
}

func supportedSchemeIDs(schemes []spec.SecuritySchemeSummary) []string {
	var ids []string
	for _, scheme := range schemes {
		if scheme.Supported {
			ids = append(ids, scheme.ID)
		}
	}
	return ids
}

func credentialOperationCounts(ops []spec.Operation) map[string]int {
	counts := map[string]int{}
	for _, op := range ops {
		seen := map[string]bool{}
		for _, alternative := range op.CredentialAlternatives {
			for _, requirement := range alternative {
				seen[requirement.ID] = true
			}
		}
		for id := range seen {
			counts[id]++
		}
	}
	return counts
}

func credentialNeedDefaults(ops []spec.Operation) map[string][]string {
	needs := map[string]map[string]bool{}
	for _, op := range ops {
		for _, alternative := range op.CredentialAlternatives {
			for _, requirement := range alternative {
				if len(requirement.Needs) == 0 {
					continue
				}
				if needs[requirement.ID] == nil {
					needs[requirement.ID] = map[string]bool{}
				}
				for _, need := range requirement.Needs {
					needs[requirement.ID][need] = true
				}
			}
		}
	}
	out := map[string][]string{}
	for id, values := range needs {
		for value := range values {
			out[id] = append(out[id], value)
		}
		sort.Strings(out[id])
	}
	return out
}

func (c *CLI) printAuthCoverage(profileName string, d configureAuthDiscovery, configured map[string]bool) {
	if len(d.schemes) == 0 {
		return
	}
	var configuredIDs, skippedIDs []string
	for _, scheme := range d.schemes {
		if configured[scheme.ID] {
			configuredIDs = append(configuredIDs, scheme.ID)
		} else {
			skippedIDs = append(skippedIDs, scheme.ID)
		}
	}
	callable, secured := authCoverageCounts(d.operations, configured)
	fmt.Fprintf(c.Stdout, "\nAuth coverage for profile %q:\n", profileName)
	fmt.Fprintf(c.Stdout, "  configured: %s\n", formatIDList(configuredIDs))
	fmt.Fprintf(c.Stdout, "  skipped:    %s\n", formatIDList(skippedIDs))
	fmt.Fprintf(c.Stdout, "  callable:   %d/%d secured operations\n\n", callable, secured)
}

func configuredCredentials(apiCfg *config.APIConfig, profileName string) map[string]bool {
	out := map[string]bool{}
	if apiCfg == nil || apiCfg.Profiles == nil || apiCfg.Profiles[profileName] == nil {
		return out
	}
	for id, credential := range apiCfg.Profiles[profileName].Credentials {
		if credential != nil && credential.Auth != nil {
			out[id] = true
		}
	}
	return out
}

func authCoverageCounts(ops []spec.Operation, configured map[string]bool) (int, int) {
	var callable, secured int
	for _, op := range ops {
		if len(op.CredentialAlternatives) == 0 {
			continue
		}
		secured++
		for _, alternative := range op.CredentialAlternatives {
			ok := true
			for _, requirement := range alternative {
				if !configured[requirement.ID] {
					ok = false
					break
				}
			}
			if ok {
				callable++
				break
			}
		}
	}
	return callable, secured
}

func formatIDList(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

func cloneAuthParams(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
