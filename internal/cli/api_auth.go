package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
	"github.com/spf13/cobra"
)

func (c *CLI) newAPIAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage API auth credentials",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list <api>",
		Short: "List configured auth credentials for an API profile",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPIAuthList,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "add <api> <credential-id>",
		Short: "Add an empty credential binding to an API profile",
		Args:  cobra.ExactArgs(2),
		RunE:  c.runAPIAuthAdd,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "remove <api> <credential-id>",
		Short: "Remove a credential binding from an API profile",
		Args:  cobra.ExactArgs(2),
		RunE:  c.runAPIAuthRemove,
	})
	logoutCmd := &cobra.Command{
		Use:   "logout [api]",
		Short: "Delete cached API auth tokens",
		Args:  cobra.MaximumNArgs(1),
		RunE:  c.runAPIAuthLogout,
	}
	addAPIAuthLogoutFlags(logoutCmd)
	cmd.AddCommand(logoutCmd)
	clearCacheCmd := &cobra.Command{
		Use:    "clear-cache [api]",
		Short:  "Delete cached API auth tokens",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.warnf("api auth clear-cache is deprecated; use api auth logout")
			return c.runAPIAuthLogout(cmd, args)
		},
	}
	addAPIAuthLogoutFlags(clearCacheCmd)
	cmd.AddCommand(clearCacheCmd)
	headerCmd := &cobra.Command{
		Use:   "header <api> <header> [credential-id]",
		Short: "Print one auth header value for an API profile",
		Args:  cobra.RangeArgs(2, 3),
		RunE:  c.runAPIAuthHeader,
	}
	headerCmd.Flags().String("rsh-credential", "", "Credential ID to inspect instead of profile-level auth")
	headerCmd.Flags().String("rsh-operation", "", "Operation ID or command name to inspect")
	cmd.AddCommand(headerCmd)
	inspectCmd := &cobra.Command{
		Use:   "inspect <api>",
		Short: "Inspect the auth material applied for an API profile",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPIAuthInspect,
	}
	inspectCmd.Flags().String("rsh-credential", "", "Credential ID to inspect instead of profile-level auth")
	inspectCmd.Flags().String("rsh-operation", "", "Operation ID or command name to inspect")
	inspectCmd.Flags().Bool("redact", false, "Redact sensitive auth values for shareable output")
	inspectCmd.Flags().String("raw-header", "", "Deprecated: use \"restish api auth header <api> <header>\"")
	_ = inspectCmd.Flags().MarkHidden("raw-header")
	cmd.AddCommand(inspectCmd)
	return cmd
}

func addAPIAuthLogoutFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all-profiles", false, "Delete cached auth tokens for every profile of the named API")
	cmd.Flags().String("auth-profile", "", "Delete cached auth tokens for a shared auth profile instead of an API")
}

func (c *CLI) runAPIAuthList(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	profileName := c.profileFromCmd(cmd)
	apiCfg, prof, err := c.apiProfileForAuth(apiName, profileName, false)
	if err != nil {
		return err
	}
	set, hasOps := c.cachedOperationSetForAPI(requestContext(cmd), apiName, apiCfg, profileName)
	_, profileReady, profileErr := c.profileAuthReadiness(apiName, profileName, prof)
	coverage := operationAuthCoverage{}
	if hasOps {
		coverage = c.operationAuthCoverage(apiName, profileName, prof, set.Operations)
	}
	var ids []string
	for id := range prof.Credentials {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if jsonOut, err := commandJSONOutputRequested(cmd); err != nil {
		return err
	} else if jsonOut {
		type apiAuthCredentialEntry struct {
			ID        string   `json:"id"`
			Status    string   `json:"status"`
			Satisfies []string `json:"satisfies,omitempty"`
		}
		type apiAuthListOutput struct {
			API                          string                   `json:"api"`
			Profile                      string                   `json:"profile"`
			ProfileAuth                  string                   `json:"profile_auth"`
			ProfileAuthError             string                   `json:"profile_auth_error,omitempty"`
			OperationMetadataAvailable   bool                     `json:"operation_metadata_available"`
			CallableSecuredOperations    int                      `json:"callable_secured_operations,omitempty"`
			TotalSecuredOperations       int                      `json:"total_secured_operations,omitempty"`
			OperationMetadataRefreshHint string                   `json:"operation_metadata_refresh_hint,omitempty"`
			Credentials                  []apiAuthCredentialEntry `json:"credentials"`
		}
		out := apiAuthListOutput{
			API:                        apiName,
			Profile:                    profileName,
			ProfileAuth:                "none",
			OperationMetadataAvailable: hasOps,
			Credentials:                make([]apiAuthCredentialEntry, 0, len(ids)),
		}
		if profileErr != nil {
			out.ProfileAuth = "configured"
			out.ProfileAuthError = profileErr.Error()
		} else if profileReady.Configured {
			out.ProfileAuth = profileReady.status("none")
		}
		if hasOps {
			out.CallableSecuredOperations = coverage.Callable
			out.TotalSecuredOperations = coverage.Secured
		} else {
			out.OperationMetadataRefreshHint = fmt.Sprintf("run \"restish api sync %s\" to refresh", apiName)
		}
		for _, id := range ids {
			credential := prof.Credentials[id]
			_, ready, _ := c.credentialReadiness(apiName, profileName, id, credential)
			var satisfies []string
			if credential != nil {
				satisfies = append([]string(nil), credential.Satisfies...)
			}
			out.Credentials = append(out.Credentials, apiAuthCredentialEntry{
				ID:        id,
				Status:    ready.status("empty"),
				Satisfies: satisfies,
			})
		}
		return c.writePrettyJSON(out)
	}
	fmt.Fprintf(c.Stdout, "API: %s\nProfile: %s\n", apiName, profileName)
	if profileErr != nil {
		fmt.Fprintf(c.Stdout, "Profile auth: configured (%s)\n", profileErr)
	} else if profileReady.Configured {
		fmt.Fprintf(c.Stdout, "Profile auth: %s\n", profileReady.status("none"))
	} else {
		fmt.Fprintln(c.Stdout, "Profile auth: none")
	}
	if hasOps {
		fmt.Fprintf(c.Stdout, "Callable secured operations: %d/%d\n", coverage.Callable, coverage.Secured)
	} else {
		fmt.Fprintf(c.Stdout, "Operation metadata: unavailable (run \"restish api sync %s\" to refresh)\n", apiName)
	}
	if len(ids) == 0 {
		fmt.Fprintln(c.Stdout, "Credentials: none")
	} else {
		fmt.Fprintln(c.Stdout, "Credentials:")
		for _, id := range ids {
			credential := prof.Credentials[id]
			_, ready, _ := c.credentialReadiness(apiName, profileName, id, credential)
			status := ready.status("empty")
			fmt.Fprintf(c.Stdout, "  %s: %s", id, status)
			if len(credential.Satisfies) > 0 {
				fmt.Fprintf(c.Stdout, " (satisfies: %s)", strings.Join(credential.Satisfies, ", "))
			}
			fmt.Fprintln(c.Stdout)
		}
	}
	if hasOps {
		coverage := c.operationAuthCoverage(apiName, profileName, prof, set.Operations)
		c.printAPIAuthRequirementSummary(apiName, profileName, set.Operations, prof, coverage)
	}
	return nil
}

func (c *CLI) runAPIAuthAdd(cmd *cobra.Command, args []string) error {
	apiName, credentialID := args[0], args[1]
	profileName := c.profileFromCmd(cmd)
	apiCfg, prof, err := c.apiProfileForAuth(apiName, profileName, true)
	if err != nil {
		return err
	}
	if prof.Credentials == nil {
		prof.Credentials = map[string]*config.CredentialConfig{}
	}
	if prof.Credentials[credentialID] == nil {
		prof.Credentials[credentialID] = &config.CredentialConfig{}
	}
	defaultNeeds := c.cachedCredentialDefaultNeeds(requestContext(cmd), apiName, apiCfg, profileName, credentialID)
	if prof.Credentials[credentialID].Auth == nil {
		if authCfg, ok, err := c.cachedAuthConfigForCredential(apiName, apiCfg, credentialID); err != nil {
			return err
		} else if ok {
			prof.Credentials[credentialID].Auth = authCfg
		}
	}
	if prof.Credentials[credentialID].Auth != nil {
		if err := c.promptAuthParams(requestContext(cmd), profileName, credentialID, prof.Credentials[credentialID].Auth, defaultNeeds, configurePromptAnswers{}); err != nil {
			return err
		}
		if len(defaultNeeds) > 0 {
			if prof.Credentials[credentialID].Auth.Params == nil {
				prof.Credentials[credentialID].Auth.Params = map[string]string{}
			}
			if prof.Credentials[credentialID].Auth.Params["scopes"] == "" {
				prof.Credentials[credentialID].Auth.Params["scopes"] = strings.Join(defaultNeeds, " ")
			}
			prof.Credentials[credentialID].Satisfies = authSatisfiesValues(defaultNeeds, prof.Credentials[credentialID].Auth)
		}
	}
	if err := c.saveAPIAuthCredentialConfig(apiName, profileName, credentialID, prof.Credentials[credentialID]); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Added credential %q to API %q profile %q\n", credentialID, apiName, profileName)
	return nil
}

func (c *CLI) runAPIAuthRemove(cmd *cobra.Command, args []string) error {
	apiName, credentialID := args[0], args[1]
	profileName := c.profileFromCmd(cmd)
	_, prof, err := c.apiProfileForAuth(apiName, profileName, false)
	if err != nil {
		return err
	}
	if prof.Credentials == nil || prof.Credentials[credentialID] == nil {
		return fmt.Errorf("profile %q of API %q has no credential %q", profileName, apiName, credentialID)
	}
	if err := c.removeAPIAuthCredentialConfig(apiName, profileName, credentialID); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Removed credential %q from API %q profile %q\n", credentialID, apiName, profileName)
	return nil
}

func (c *CLI) runAPIAuthInspect(cmd *cobra.Command, args []string) error {
	if err := rejectResponseTransformFlags(cmd); err != nil {
		return err
	}
	apiName := args[0]
	if looksLikeURLArgument(apiName) {
		return fmt.Errorf("api auth inspect expects an API name, not a URL\nv2 form: restish api auth header <api-name> Authorization")
	}
	profileName := c.profileFromCmd(cmd)
	apiCfg, prof, err := c.apiProfileForAuth(apiName, profileName, false)
	if err != nil {
		return err
	}
	credentialID, _ := cmd.Flags().GetString("rsh-credential")
	rawHeader, _ := cmd.Flags().GetString("raw-header")
	redact, _ := cmd.Flags().GetBool("redact")
	if operation, _ := cmd.Flags().GetString("rsh-operation"); operation != "" {
		if credentialID != "" {
			return fmt.Errorf("--rsh-operation and --rsh-credential are mutually exclusive")
		}
		return c.runAPIAuthInspectOperation(cmd, apiName, profileName, apiCfg, prof, operation, rawHeader, redact)
	}

	if rawHeader != "" {
		resolved, _, err := c.resolveAuthInspectionConfig(apiName, profileName, prof, credentialID)
		if err != nil {
			return err
		}
		if resolved.Config == nil {
			return fmt.Errorf("profile %q of API %q has no auth config", profileName, apiName)
		}
		req, err := c.authInspectionRequest(cmd, apiName, profileName, resolved)
		if err != nil {
			return err
		}
		value := req.Header.Get(rawHeader)
		if value == "" {
			return fmt.Errorf("auth did not set header %q", rawHeader)
		}
		fmt.Fprintln(c.Stdout, value)
		return nil
	}

	targets, err := c.authInspectionTargets(apiName, profileName, prof, credentialID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("profile %q of API %q has no auth config", profileName, apiName)
	}
	for i, target := range targets {
		if i > 0 {
			fmt.Fprintln(c.Stdout)
		}
		if target.Label != "" {
			fmt.Fprintf(c.Stdout, "%s\n", target.Label)
		}
		if target.Resolved.Config == nil {
			return fmt.Errorf("profile %q of API %q has no auth config", profileName, apiName)
		}
		req, err := c.authInspectionRequest(cmd, apiName, profileName, target.Resolved)
		if err != nil {
			return err
		}
		c.printAuthInspectionRequest(req, []*config.AuthConfig{target.Resolved.Config}, redact)
	}
	return nil
}

type authInspectionTarget struct {
	Label    string
	Resolved resolvedAuthConfig
}

func (c *CLI) authInspectionTargets(apiName, profileName string, prof *config.ProfileConfig, credentialID string) ([]authInspectionTarget, error) {
	if credentialID != "" {
		resolved, _, err := c.resolveAuthInspectionConfig(apiName, profileName, prof, credentialID)
		return []authInspectionTarget{{Label: fmt.Sprintf("Credential: %s", credentialID), Resolved: resolved}}, err
	}

	var targets []authInspectionTarget
	resolved, err := c.resolveProfileAuth(apiName, profileName, prof)
	if err != nil {
		return nil, err
	}
	if resolved.Config != nil {
		targets = append(targets, authInspectionTarget{Resolved: resolved})
	}

	ids := configuredCredentialIDs(prof)
	if len(ids) == 0 {
		if len(targets) == 0 && prof != nil && len(prof.Credentials) > 0 {
			return nil, fmt.Errorf("profile %q of API %q has credentials but none have auth configured; run \"restish api auth list %s\"", profileName, apiName, apiName)
		}
		return targets, nil
	}
	for _, id := range ids {
		resolved, err := c.resolveCredentialAuth(apiName, profileName, id, prof.Credentials[id])
		if err != nil {
			return nil, fmt.Errorf("credential %q: %w", id, err)
		}
		targets = append(targets, authInspectionTarget{
			Label:    fmt.Sprintf("Credential: %s", id),
			Resolved: resolved,
		})
	}
	if len(targets) > 1 && targets[0].Label == "" {
		targets[0].Label = "Profile auth:"
	}
	return targets, nil
}

func (c *CLI) runAPIAuthInspectOperation(cmd *cobra.Command, apiName, profileName string, apiCfg *config.APIConfig, prof *config.ProfileConfig, operationName, rawHeader string, redact bool) error {
	op, ok, err := c.cachedOperationForAPI(requestContext(cmd), apiName, apiCfg, profileName, operationName)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("operation %q not found in cached metadata for API %q; run \"restish api sync %s\"", operationName, apiName, apiName)
	}
	if op.NoAuth {
		if rawHeader != "" {
			return fmt.Errorf("operation %q has security: [] and does not send auth header %q", operationName, rawHeader)
		}
		fmt.Fprintf(c.Stdout, "Operation: %s\nAuth: none (security: [])\n", op.ID)
		return nil
	}

	var selected []selectedOperationAuth
	if len(op.CredentialAlternatives) > 0 {
		var selectedOK bool
		selected, selectedOK, err = c.planOperationAuth(apiName, profileName, prof, &operationAuthPolicy{
			OptionalAuth:           op.OptionalAuth,
			CredentialAlternatives: op.CredentialAlternatives,
		})
		if err != nil {
			return err
		}
		if !selectedOK || len(selected) == 0 {
			if rawHeader != "" {
				return fmt.Errorf("operation %q did not select auth header %q", op.ID, rawHeader)
			}
			fmt.Fprintf(c.Stdout, "Operation: %s\nAuth: none\n", op.ID)
			return nil
		}
	} else {
		resolved, selectedCredential, err := c.resolveAuthInspectionConfig(apiName, profileName, prof, "")
		if err != nil {
			return err
		}
		if resolved.Config == nil {
			return fmt.Errorf("profile %q of API %q has no auth config", profileName, apiName)
		}
		selected = []selectedOperationAuth{{requirement: spec.CredentialRequirement{ID: selectedCredential}, resolved: resolved, source: "profile auth"}}
	}

	req, err := c.operationAuthInspectionRequest(cmd, apiName, profileName, selected)
	if err != nil {
		return err
	}
	if rawHeader != "" {
		value := req.Header.Get(rawHeader)
		if value == "" {
			return fmt.Errorf("auth did not set header %q", rawHeader)
		}
		fmt.Fprintln(c.Stdout, value)
		return nil
	}
	fmt.Fprintf(c.Stdout, "Operation: %s\n", op.ID)
	fmt.Fprintf(c.Stdout, "Credentials: %s\n", strings.Join(selectedOperationCredentialIDs(selected), ", "))
	fmt.Fprintf(c.Stdout, "Source: %s\n", strings.Join(selectedOperationSources(selected), ", "))
	c.printAuthInspectionRequest(req, selectedOperationAuthConfigs(selected), redact)
	return nil
}

func (c *CLI) runAPIAuthHeader(cmd *cobra.Command, args []string) error {
	apiName, headerName := args[0], args[1]
	if looksLikeURLArgument(apiName) {
		return fmt.Errorf("api auth header expects an API name, not a URL")
	}
	credentialID, _ := cmd.Flags().GetString("rsh-credential")
	if len(args) == 3 {
		if credentialID != "" {
			return fmt.Errorf("pass credential ID either as an argument or --rsh-credential, not both")
		}
		credentialID = args[2]
	}
	profileName := c.profileFromCmd(cmd)
	apiCfg, prof, err := c.apiProfileForAuth(apiName, profileName, false)
	if err != nil {
		return err
	}
	if operation, _ := cmd.Flags().GetString("rsh-operation"); operation != "" {
		if credentialID != "" {
			return fmt.Errorf("--rsh-operation and credential ID are mutually exclusive")
		}
		return c.runAPIAuthInspectOperation(cmd, apiName, profileName, apiCfg, prof, operation, headerName, false)
	}

	resolved, _, err := c.resolveAuthInspectionConfig(apiName, profileName, prof, credentialID)
	if err != nil {
		return err
	}
	if resolved.Config == nil {
		return fmt.Errorf("profile %q of API %q has no auth config", profileName, apiName)
	}
	req, err := c.authInspectionRequest(cmd, apiName, profileName, resolved)
	if err != nil {
		return err
	}
	value := req.Header.Get(headerName)
	if value == "" {
		return fmt.Errorf("auth did not set header %q", headerName)
	}
	fmt.Fprintln(c.Stdout, value)
	return nil
}

func (c *CLI) resolveAuthInspectionConfig(apiName, profileName string, prof *config.ProfileConfig, credentialID string) (resolvedAuthConfig, string, error) {
	if credentialID != "" {
		credential := prof.Credentials[credentialID]
		if credential == nil {
			return resolvedAuthConfig{}, "", fmt.Errorf("profile %q of API %q has no credential %q", profileName, apiName, credentialID)
		}
		resolved, err := c.resolveCredentialAuth(apiName, profileName, credentialID, credential)
		return resolved, credentialID, err
	}

	resolved, err := c.resolveProfileAuth(apiName, profileName, prof)
	if err != nil || resolved.Config != nil {
		return resolved, "", err
	}

	ids := configuredCredentialIDs(prof)
	switch len(ids) {
	case 0:
		if len(prof.Credentials) > 0 {
			return resolvedAuthConfig{}, "", fmt.Errorf("profile %q of API %q has credentials but none have auth configured; run \"restish api auth list %s\"", profileName, apiName, apiName)
		}
		return resolvedAuthConfig{}, "", nil
	case 1:
		credentialID := ids[0]
		resolved, err := c.resolveCredentialAuth(apiName, profileName, credentialID, prof.Credentials[credentialID])
		return resolved, credentialID, err
	default:
		return resolvedAuthConfig{}, "", fmt.Errorf("profile %q of API %q has multiple configured credentials (%s); pass --rsh-credential <id>", profileName, apiName, strings.Join(ids, ", "))
	}
}

func looksLikeURLArgument(arg string) bool {
	return strings.Contains(arg, "://") || strings.HasPrefix(arg, "/") || strings.ContainsAny(arg, "?#")
}

func configuredCredentialIDs(prof *config.ProfileConfig) []string {
	if prof == nil || len(prof.Credentials) == 0 {
		return nil
	}
	ids := make([]string, 0, len(prof.Credentials))
	for id, credential := range prof.Credentials {
		if credential != nil && (credential.Auth != nil || credential.AuthRef != "") {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func isAuthInspectionSensitiveHeader(ac *config.AuthConfig, name string) bool {
	return ac != nil &&
		ac.Type == "api-key" &&
		strings.EqualFold(ac.Params["in"], "header") &&
		strings.EqualFold(ac.Params["name"], name)
}

func isAuthInspectionSensitiveQueryParam(ac *config.AuthConfig, name string) bool {
	return ac != nil &&
		ac.Type == "api-key" &&
		strings.EqualFold(ac.Params["in"], "query") &&
		strings.EqualFold(ac.Params["name"], name)
}

func (c *CLI) operationAuthInspectionRequest(cmd *cobra.Command, apiName, profileName string, selected []selectedOperationAuth) (*http.Request, error) {
	authOpts, err := c.authHandlerOptionsFromCmd(cmd)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(requestContext(cmd), "GET", "http://example.com", nil)
	for _, item := range selected {
		step, err := c.operationAuthStep(apiName, profileName, item, authOpts)
		if err != nil {
			return nil, err
		}
		if err := c.applyOperationAuthInspectionStep(req, step); err != nil {
			return nil, err
		}
	}
	return req, nil
}

func (c *CLI) applyOperationAuthInspectionStep(req *http.Request, s operationAuthStep) error {
	params, err := c.buildAuthParams(s.rawParams)
	if err != nil {
		return err
	}
	if s.authType == "external-tool" {
		if err := c.ensureExternalToolApproved(req.Context(), s.apiName, s.profileName, params["commandline"]); err != nil {
			return err
		}
	}
	return s.handler.Authenticate(req.Context(), req, c.authContext(req.Context(), s.apiName, s.profileName, params, s.cacheKey, false))
}

func (c *CLI) authInspectionRequest(cmd *cobra.Command, apiName, profileName string, resolved resolvedAuthConfig) (*http.Request, error) {
	authOpts, err := c.authHandlerOptionsFromCmd(cmd)
	if err != nil {
		return nil, err
	}
	handler, err := c.authHandlerFor(resolved.Config, authOpts)
	if err != nil {
		return nil, err
	}
	params, err := c.buildAuthParams(resolved.Config.Params)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(requestContext(cmd), "GET", "http://example.com", nil)
	if err := handler.Authenticate(requestContext(cmd), req, c.authContext(requestContext(cmd), apiName, profileName, params, resolved.CacheKey, false)); err != nil {
		return nil, fmt.Errorf("building auth inspection: %w", err)
	}
	return req, nil
}

func (c *CLI) printAuthInspectionRequest(req *http.Request, configs []*config.AuthConfig, redact bool) {
	for _, ac := range configs {
		if ac != nil {
			fmt.Fprintf(c.Stdout, "Auth type: %s\n", ac.Type)
		}
	}
	for _, name := range sortedHeaderKeys(req.Header) {
		values := req.Header[name]
		for _, value := range values {
			if redact && (isSensitiveHeader(name) || authInspectionSensitiveHeader(configs, name)) {
				value = "<redacted>"
			}
			fmt.Fprintf(c.Stdout, "%s: %s\n", name, value)
		}
	}
	if req.URL.RawQuery != "" {
		query := req.URL.String()
		if redact {
			query = redactedAuthInspectionURL(req.URL, configs)
		}
		fmt.Fprintf(c.Stdout, "Query: %s\n", query)
	}
}

func authInspectionSensitiveHeader(configs []*config.AuthConfig, name string) bool {
	for _, ac := range configs {
		if isAuthInspectionSensitiveHeader(ac, name) {
			return true
		}
	}
	return false
}

func authInspectionSensitiveQueryParam(configs []*config.AuthConfig, name string) bool {
	for _, ac := range configs {
		if isAuthInspectionSensitiveQueryParam(ac, name) {
			return true
		}
	}
	return false
}

func redactedAuthInspectionURL(u *url.URL, configs []*config.AuthConfig) string {
	if u == nil {
		return ""
	}
	copyURL := *u
	q := copyURL.Query()
	for name := range q {
		if isSensitiveQueryParam(name) || authInspectionSensitiveQueryParam(configs, name) {
			q.Set(name, "<redacted>")
		}
	}
	copyURL.RawQuery = q.Encode()
	return copyURL.String()
}

func selectedOperationCredentialIDs(selected []selectedOperationAuth) []string {
	ids := make([]string, 0, len(selected))
	for _, item := range selected {
		if item.requirement.ID != "" {
			ids = append(ids, item.requirement.ID)
		}
	}
	return ids
}

func selectedOperationSources(selected []selectedOperationAuth) []string {
	sources := make([]string, 0, len(selected))
	for _, item := range selected {
		source := item.source
		if source == "" {
			source = selectedAuthSourceCredential(item.resolved)
		}
		sources = append(sources, source)
	}
	return sources
}

func selectedOperationAuthConfigs(selected []selectedOperationAuth) []*config.AuthConfig {
	configs := make([]*config.AuthConfig, 0, len(selected))
	for _, item := range selected {
		configs = append(configs, item.resolved.Config)
	}
	return configs
}

func (c *CLI) apiProfileForAuth(apiName, profileName string, create bool) (*config.APIConfig, *config.ProfileConfig, error) {
	apiCfg, err := c.requireAPI(apiName)
	if err != nil {
		return nil, nil, fmt.Errorf("%w; run \"restish api list\" to see configured APIs", err)
	}
	if apiCfg.Profiles != nil && apiCfg.Profiles[profileName] != nil {
		return apiCfg, apiCfg.Profiles[profileName], nil
	}
	if profileName == "default" {
		prof := &config.ProfileConfig{}
		if create {
			if apiCfg.Profiles == nil {
				apiCfg.Profiles = map[string]*config.ProfileConfig{}
			}
			apiCfg.Profiles[profileName] = prof
		}
		return apiCfg, prof, nil
	}
	if create {
		if apiCfg.Profiles == nil {
			apiCfg.Profiles = map[string]*config.ProfileConfig{}
		}
		prof := &config.ProfileConfig{}
		apiCfg.Profiles[profileName] = prof
		return apiCfg, prof, nil
	}
	if apiCfg.Profiles == nil || apiCfg.Profiles[profileName] == nil {
		return nil, nil, fmt.Errorf("API %q has no profile %q; configured profiles: %s", apiName, profileName, profileNames(apiCfg.Profiles))
	}
	return apiCfg, apiCfg.Profiles[profileName], nil
}

func (c *CLI) cachedOperationSetForAPI(ctx context.Context, apiName string, apiCfg *config.APIConfig, profileName string) (spec.OperationSet, bool) {
	if set, _, ok := c.cachedOperationSetStatusForAPI(apiName, apiCfg, profileName); ok {
		return set, true
	}
	set, ok, _ := c.operationSetForAPI(ctx, apiName, apiCfg, profileName, false)
	return set, ok
}

func (c *CLI) cachedOperationSetStatusForAPI(apiName string, apiCfg *config.APIConfig, profileName string) (spec.OperationSet, spec.OperationCacheStatus, bool) {
	if apiCfg == nil {
		return spec.OperationSet{}, spec.OperationCacheStatus{}, false
	}
	opts := spec.OperationOptions{
		BaseURL:         effectiveProfileBaseURL(apiCfg, profileName),
		OperationBase:   effectiveOperationBase(apiCfg, profileName),
		ServerVariables: effectiveServerVariables(apiCfg, profileName),
		Warnf:           c.warnf,
	}
	if set, status, ok := spec.LoadOperationSetFromCacheStatus(c.specCacheDir(), apiName, Version, apiCfg.SpecFiles, opts, true); ok {
		return set, status, true
	}
	opts.BaseURL = apiCfg.BaseURL
	opts.OperationBase = apiCfg.OperationBase
	return spec.LoadOperationSetFromCacheStatus(c.specCacheDir(), apiName, Version, apiCfg.SpecFiles, opts, true)
}

func (c *CLI) operationSetForAPI(ctx context.Context, apiName string, apiCfg *config.APIConfig, profileName string, forceRefresh bool) (spec.OperationSet, bool, error) {
	if apiCfg == nil {
		return spec.OperationSet{}, false, nil
	}
	opts := spec.OperationOptions{
		BaseURL:         effectiveProfileBaseURL(apiCfg, profileName),
		OperationBase:   effectiveOperationBase(apiCfg, profileName),
		ServerVariables: effectiveServerVariables(apiCfg, profileName),
		Warnf:           c.warnf,
	}
	if !forceRefresh {
		if set, _, ok := spec.LoadOperationSetFromCacheStatus(c.specCacheDir(), apiName, Version, apiCfg.SpecFiles, opts, true); ok {
			c.warnOperationSetWarnings(set)
			return set, true, nil
		}
	}
	var s *spec.APISpec
	var err error
	if forceRefresh {
		s, err = c.discoverSpec(ctx, apiName)
	} else {
		s, err = spec.LoadFromCache(c.specCacheDir(), apiName, Version, apiCfg.SpecFiles, c.loaders)
	}
	if err != nil || s == nil {
		if !spec.HasLocalSpecFiles(apiCfg.SpecFiles) {
			return spec.OperationSet{}, false, err
		}
		s, err = spec.Discover(ctx, spec.DiscoverConfig{
			APIName:         apiName,
			BaseURL:         apiCfg.BaseURL,
			SpecFiles:       apiCfg.SpecFiles,
			CacheDir:        c.specCacheDir(),
			OperationBase:   apiCfg.OperationBase,
			ServerVariables: effectiveServerVariables(apiCfg, profileName),
			Version:         Version,
			ForceRefresh:    true,
		}, c.loaders)
		if err != nil || s == nil {
			return spec.OperationSet{}, false, err
		}
	}
	set, err := s.OperationSet(opts)
	if err != nil {
		return spec.OperationSet{}, false, err
	}
	_ = spec.StoreOperationSetInCache(c.specCacheDir(), apiName, Version, opts, set)
	return set, true, nil
}

func (c *CLI) warnOperationSetWarnings(set spec.OperationSet) {
	for _, warning := range set.Warnings {
		c.warnf("%s", warning)
	}
}

func (c *CLI) cachedOperationForAPI(ctx context.Context, apiName string, apiCfg *config.APIConfig, profileName, value string) (spec.Operation, bool, error) {
	set, ok := c.cachedOperationSetForAPI(ctx, apiName, apiCfg, profileName)
	if !ok {
		return spec.Operation{}, false, nil
	}
	for _, op := range set.Operations {
		if op.ID == value || operationCommandName(op) == value {
			return op, true, nil
		}
	}
	return spec.Operation{}, false, nil
}

func operationCommandName(op spec.Operation) string {
	if op.XCLI.Name != "" {
		return op.XCLI.Name
	}
	if name := toKebabCase(op.ID); name != "" {
		return name
	}
	return fallbackOperationName(op.Method, op.Path)
}

func configuredCredentialsForProfile(prof *config.ProfileConfig) map[string]bool {
	out := map[string]bool{}
	if prof == nil {
		return out
	}
	for id, credential := range prof.Credentials {
		if credential != nil && (credential.Auth != nil || credential.AuthRef != "") {
			out[id] = true
		}
	}
	return out
}

type authRequirementSummary struct {
	id         string
	kind       string
	needs      []string
	opCount    int
	external   bool
	undeclared bool
	deprecated bool
}

func (c *CLI) printAPIAuthRequirementSummary(apiName, profileName string, ops []spec.Operation, prof *config.ProfileConfig, coverage operationAuthCoverage) {
	summaries := authRequirementSummaries(ops)
	if len(summaries) == 0 {
		return
	}
	fmt.Fprintln(c.Stdout, "Declared credentials:")
	for _, summary := range summaries {
		status := "missing"
		var satisfies []string
		if prof != nil && prof.Credentials != nil {
			if credential := prof.Credentials[summary.id]; credential != nil {
				_, ready, _ := c.credentialReadiness(apiName, profileName, summary.id, credential)
				status = ready.status("empty")
				satisfies = credential.Satisfies
			}
		}
		var parts []string
		parts = append(parts, status)
		if status == "missing" && coverage.FallbackByID[summary.id] > 0 {
			parts = append(parts, coverage.FallbackLabels[summary.id])
		}
		if len(summary.needs) > 0 {
			parts = append(parts, "needs "+strings.Join(summary.needs, " "))
		}
		if len(satisfies) > 0 {
			parts = append(parts, "satisfies "+strings.Join(satisfies, " "))
		}
		if !authRequirementKindSupported(summary.kind) {
			parts = append(parts, "unsupported "+summary.kind)
		}
		if summary.undeclared {
			parts = append(parts, "undeclared security scheme")
		}
		if summary.deprecated {
			parts = append(parts, "deprecated")
		}
		if summary.external {
			parts = append(parts, "URI-backed")
		}
		operationWord := "operations"
		if summary.opCount == 1 {
			operationWord = "operation"
		}
		parts = append(parts, fmt.Sprintf("%d %s", summary.opCount, operationWord))
		fmt.Fprintf(c.Stdout, "  %s: %s\n", summary.id, strings.Join(parts, ", "))
	}
}

func authRequirementSummaries(ops []spec.Operation) []authRequirementSummary {
	byID := map[string]*authRequirementSummary{}
	opSeen := map[string]map[int]bool{}
	for opIndex, op := range ops {
		for _, alternative := range op.CredentialAlternatives {
			for _, requirement := range alternative {
				summary := byID[requirement.ID]
				if summary == nil {
					summary = &authRequirementSummary{id: requirement.ID, kind: requirement.Kind}
					byID[requirement.ID] = summary
				}
				summary.needs = mergeStringSet(summary.needs, requirement.Needs)
				summary.external = summary.external || requirement.External
				summary.undeclared = summary.undeclared || requirement.Undeclared
				summary.deprecated = summary.deprecated || requirement.Deprecated
				if opSeen[requirement.ID] == nil {
					opSeen[requirement.ID] = map[int]bool{}
				}
				if !opSeen[requirement.ID][opIndex] {
					summary.opCount++
					opSeen[requirement.ID][opIndex] = true
				}
			}
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]authRequirementSummary, 0, len(ids))
	for _, id := range ids {
		sort.Strings(byID[id].needs)
		out = append(out, *byID[id])
	}
	return out
}

func mergeStringSet(existing, values []string) []string {
	seen := map[string]bool{}
	for _, v := range existing {
		seen[v] = true
	}
	for _, v := range values {
		if v != "" && !seen[v] {
			existing = append(existing, v)
			seen[v] = true
		}
	}
	return existing
}

func authRequirementKindSupported(kind string) bool {
	switch kind {
	case "api-key", "http-basic", "http-bearer", "oauth2":
		return true
	default:
		return false
	}
}

func (c *CLI) cachedCredentialDefaultNeeds(ctx context.Context, apiName string, apiCfg *config.APIConfig, profileName, credentialID string) []string {
	set, ok := c.cachedOperationSetForAPI(ctx, apiName, apiCfg, profileName)
	if !ok {
		return nil
	}
	needs := credentialNeedDefaults(set.Operations)[credentialID]
	return append([]string(nil), needs...)
}

func (c *CLI) cachedAuthConfigForCredential(apiName string, apiCfg *config.APIConfig, credentialID string) (*config.AuthConfig, bool, error) {
	if apiCfg == nil {
		return nil, false, nil
	}
	apiSpec, err := spec.LoadFromCache(c.specCacheDir(), apiName, Version, apiCfg.SpecFiles, c.loaders)
	if err != nil || apiSpec == nil {
		return nil, false, err
	}
	xcli := (&spec.XCLIConfig{Profiles: map[string]*spec.XCLIProfile{
		"default": {
			Credentials: map[string]*spec.XCLICredential{
				credentialID: {},
			},
		},
	}}).Resolve(apiSpec)
	if xcli == nil || xcli.Profiles["default"] == nil || xcli.Profiles["default"].Credentials[credentialID] == nil {
		return nil, false, nil
	}
	auth := xcli.Profiles["default"].Credentials[credentialID].Auth
	if auth == nil {
		return nil, false, nil
	}
	return &config.AuthConfig{Type: auth.Type, Params: auth.Params}, true, nil
}

func (c *CLI) saveAPIAuthCredentialConfig(apiName, profileName, credentialID string, credential *config.CredentialConfig) error {
	return c.saveConfigMutation("api auth", func(cfg *config.Config) error {
		apiCfg := cfg.APIs[apiName]
		if apiCfg == nil {
			return fmt.Errorf("unknown API %q", apiName)
		}
		if apiCfg.Profiles == nil {
			apiCfg.Profiles = map[string]*config.ProfileConfig{}
		}
		prof := apiCfg.Profiles[profileName]
		if prof == nil {
			prof = &config.ProfileConfig{}
			apiCfg.Profiles[profileName] = prof
		}
		if prof.Credentials == nil {
			prof.Credentials = map[string]*config.CredentialConfig{}
		}
		if credential == nil {
			credential = &config.CredentialConfig{}
		}
		prof.Credentials[credentialID] = credential
		return nil
	})
}

func (c *CLI) removeAPIAuthCredentialConfig(apiName, profileName, credentialID string) error {
	return c.saveConfigMutation("api auth", func(cfg *config.Config) error {
		apiCfg := cfg.APIs[apiName]
		if apiCfg == nil {
			return fmt.Errorf("unknown API %q", apiName)
		}
		prof := profileForName(apiCfg, profileName)
		if prof == nil || prof.Credentials == nil || prof.Credentials[credentialID] == nil {
			return fmt.Errorf("profile %q of API %q has no credential %q", profileName, apiName, credentialID)
		}
		delete(prof.Credentials, credentialID)
		if len(prof.Credentials) == 0 {
			prof.Credentials = nil
		}
		return nil
	})
}
