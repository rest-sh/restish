package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

type authReadiness struct {
	Configured bool
	Usable     bool
	Issues     []string
}

func (r authReadiness) status(empty string) string {
	if !r.Configured {
		return empty
	}
	if len(r.Issues) == 0 {
		return "configured"
	}
	return "configured (" + strings.Join(r.Issues, "; ") + ")"
}

func (c *CLI) authConfigReadiness(ac *config.AuthConfig) authReadiness {
	if ac == nil {
		return authReadiness{}
	}
	var issues []string
	for _, value := range ac.Params {
		if !strings.HasPrefix(value, "env:") {
			continue
		}
		name := strings.TrimPrefix(value, "env:")
		if name == "" {
			issues = append(issues, "env missing: <empty>")
			continue
		}
		if _, ok := os.LookupEnv(name); !ok {
			issues = append(issues, "env missing: "+name)
		}
	}
	sort.Strings(issues)
	return authReadiness{Configured: true, Usable: len(issues) == 0, Issues: issues}
}

func (c *CLI) resolvedAuthReadiness(resolved resolvedAuthConfig) authReadiness {
	return c.authConfigReadiness(resolved.Config)
}

func (c *CLI) credentialReadiness(apiName, profileName, credentialID string, credential *config.CredentialConfig) (resolvedAuthConfig, authReadiness, error) {
	if credential == nil || (credential.Auth == nil && credential.AuthRef == "") {
		return resolvedAuthConfig{}, authReadiness{}, nil
	}
	resolved, err := c.resolveCredentialAuth(apiName, profileName, credentialID, credential)
	if err != nil {
		return resolvedAuthConfig{}, authReadiness{Configured: true, Issues: []string{err.Error()}}, err
	}
	return resolved, c.resolvedAuthReadiness(resolved), nil
}

func (c *CLI) profileAuthReadiness(apiName, profileName string, prof *config.ProfileConfig) (resolvedAuthConfig, authReadiness, error) {
	resolved, err := c.resolveProfileAuth(apiName, profileName, prof)
	if err != nil {
		return resolvedAuthConfig{}, authReadiness{Configured: true, Issues: []string{err.Error()}}, err
	}
	return resolved, c.resolvedAuthReadiness(resolved), nil
}

type operationAuthCoverage struct {
	Callable       int
	Secured        int
	FallbackByID   map[string]int
	FallbackLabels map[string]string
}

func (c *CLI) operationAuthCoverage(apiName, profileName string, prof *config.ProfileConfig, ops []spec.Operation) operationAuthCoverage {
	coverage := operationAuthCoverage{
		FallbackByID:   map[string]int{},
		FallbackLabels: map[string]string{},
	}
	for _, op := range ops {
		if len(op.CredentialAlternatives) == 0 {
			continue
		}
		coverage.Secured++
		if c.operationHasUsableNamedAlternative(apiName, profileName, prof, op.CredentialAlternatives) {
			coverage.Callable++
			continue
		}
		policy := &operationAuthPolicy{OptionalAuth: op.OptionalAuth, CredentialAlternatives: op.CredentialAlternatives}
		if !canUseProfileAuthFallback(policy) {
			continue
		}
		resolved, ready, err := c.profileAuthReadiness(apiName, profileName, prof)
		if err != nil || !ready.Usable {
			continue
		}
		requirement := op.CredentialAlternatives[0][0]
		coverage.Callable++
		coverage.FallbackByID[requirement.ID]++
		coverage.FallbackLabels[requirement.ID] = profileFallbackLabel(requirement, resolved.Config)
	}
	return coverage
}

func (c *CLI) operationHasUsableNamedAlternative(apiName, profileName string, prof *config.ProfileConfig, alternatives []spec.CredentialAlternative) bool {
	if prof == nil {
		return false
	}
	for _, alternative := range alternatives {
		ok := true
		for _, requirement := range alternative {
			credential := prof.Credentials[requirement.ID]
			resolved, ready, err := c.credentialReadiness(apiName, profileName, requirement.ID, credential)
			if err != nil || !ready.Usable || resolved.Config == nil {
				ok = false
				break
			}
			if err := credentialSatisfies(requirement, credential, resolved.Config); err != nil {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func profileFallbackLabel(requirement spec.CredentialRequirement, ac *config.AuthConfig) string {
	if profileFallbackObviouslyMatches(requirement, ac) {
		return "satisfied by profile auth fallback"
	}
	return "satisfied by profile auth fallback (unchecked auth kind)"
}

func profileFallbackObviouslyMatches(requirement spec.CredentialRequirement, ac *config.AuthConfig) bool {
	if ac == nil {
		return false
	}
	switch requirement.Kind {
	case "http-bearer":
		switch ac.Type {
		case "bearer", "oauth-client-credentials", "oauth-authorization-code", "oauth-device-code", "external-tool":
			return true
		}
	case "http-basic":
		return ac.Type == "http-basic"
	case "api-key":
		return ac.Type == "api-key"
	case "oauth2":
		return strings.HasPrefix(ac.Type, "oauth-")
	}
	return false
}

func authReadinessIssues(values ...authReadiness) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		for _, issue := range value.Issues {
			if issue == "" || seen[issue] {
				continue
			}
			seen[issue] = true
			out = append(out, issue)
		}
	}
	sort.Strings(out)
	return out
}

func formatAuthIssues(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	return fmt.Sprintf(" (%s)", strings.Join(issues, "; "))
}
