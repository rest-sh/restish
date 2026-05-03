package cli

import (
	"fmt"
	"net/http"
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
	clearCacheCmd := &cobra.Command{
		Use:   "clear-cache [api]",
		Short: "Delete cached API auth tokens",
		Args:  cobra.MaximumNArgs(1),
		RunE:  c.runAPIAuthClearCache,
	}
	clearCacheCmd.Flags().Bool("all-profiles", false, "Delete cached auth tokens for every profile of the named API")
	clearCacheCmd.Flags().String("auth-profile", "", "Delete cached auth tokens for a shared auth profile instead of an API")
	cmd.AddCommand(clearCacheCmd)
	inspectCmd := &cobra.Command{
		Use:   "inspect <api>",
		Short: "Inspect the auth material applied for an API profile",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPIAuthInspect,
	}
	inspectCmd.Flags().String("rsh-credential", "", "Credential ID to inspect instead of profile-level auth")
	inspectCmd.Flags().String("rsh-operation", "", "Operation ID or command name to inspect")
	inspectCmd.Flags().String("raw-header", "", "Print one raw header value for scripts, e.g. Authorization")
	cmd.AddCommand(inspectCmd)
	return cmd
}

func (c *CLI) runAPIAuthList(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	profileName := c.profileFromCmd(cmd)
	apiCfg, prof, err := c.apiProfileForAuth(apiName, profileName)
	if err != nil {
		return err
	}
	set, hasOps := c.cachedOperationSetForAPI(apiName, apiCfg, profileName)
	fmt.Fprintf(c.Stdout, "API: %s\nProfile: %s\n", apiName, profileName)
	if prof.Auth != nil || prof.AuthRef != "" {
		fmt.Fprintln(c.Stdout, "Profile auth: configured")
	} else {
		fmt.Fprintln(c.Stdout, "Profile auth: none")
	}
	if hasOps {
		configured := configuredCredentialsForProfile(prof)
		callable, secured := authCoverageCounts(set.Operations, configured)
		fmt.Fprintf(c.Stdout, "Callable secured operations: %d/%d\n", callable, secured)
	} else {
		fmt.Fprintf(c.Stdout, "Operation metadata: unavailable (run \"restish api sync %s\" to refresh)\n", apiName)
	}
	var ids []string
	for id := range prof.Credentials {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		fmt.Fprintln(c.Stdout, "Credentials: none")
	} else {
		fmt.Fprintln(c.Stdout, "Credentials:")
		for _, id := range ids {
			credential := prof.Credentials[id]
			status := "empty"
			if credential.Auth != nil || credential.AuthRef != "" {
				status = "configured"
			}
			fmt.Fprintf(c.Stdout, "  %s: %s", id, status)
			if len(credential.Satisfies) > 0 {
				fmt.Fprintf(c.Stdout, " (satisfies: %s)", strings.Join(credential.Satisfies, ", "))
			}
			fmt.Fprintln(c.Stdout)
		}
	}
	if hasOps {
		c.printAPIAuthRequirementSummary(set.Operations, prof)
	}
	return nil
}

func (c *CLI) runAPIAuthAdd(cmd *cobra.Command, args []string) error {
	apiName, credentialID := args[0], args[1]
	profileName := c.profileFromCmd(cmd)
	apiCfg, prof, err := c.apiProfileForAuth(apiName, profileName)
	if err != nil {
		return err
	}
	if prof.Credentials == nil {
		prof.Credentials = map[string]*config.CredentialConfig{}
	}
	if prof.Credentials[credentialID] == nil {
		prof.Credentials[credentialID] = &config.CredentialConfig{}
	}
	defaultNeeds := c.cachedCredentialDefaultNeeds(apiName, apiCfg, profileName, credentialID)
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
			prof.Credentials[credentialID].Satisfies = defaultNeeds
			if prof.Credentials[credentialID].Auth.Params == nil {
				prof.Credentials[credentialID].Auth.Params = map[string]string{}
			}
			if prof.Credentials[credentialID].Auth.Params["scopes"] == "" {
				prof.Credentials[credentialID].Auth.Params["scopes"] = strings.Join(defaultNeeds, " ")
			}
		}
	}
	if err := c.saveAPIAuthConfig(apiName, apiCfg); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Added credential %q to API %q profile %q\n", credentialID, apiName, profileName)
	return nil
}

func (c *CLI) runAPIAuthRemove(cmd *cobra.Command, args []string) error {
	apiName, credentialID := args[0], args[1]
	profileName := c.profileFromCmd(cmd)
	apiCfg, prof, err := c.apiProfileForAuth(apiName, profileName)
	if err != nil {
		return err
	}
	if prof.Credentials != nil {
		delete(prof.Credentials, credentialID)
		if len(prof.Credentials) == 0 {
			prof.Credentials = nil
		}
	}
	if err := c.saveAPIAuthConfig(apiName, apiCfg); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Removed credential %q from API %q profile %q\n", credentialID, apiName, profileName)
	return nil
}

func (c *CLI) runAPIAuthInspect(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	profileName := c.profileFromCmd(cmd)
	apiCfg, prof, err := c.apiProfileForAuth(apiName, profileName)
	if err != nil {
		return err
	}
	credentialID, _ := cmd.Flags().GetString("rsh-credential")
	rawHeader, _ := cmd.Flags().GetString("raw-header")
	if operation, _ := cmd.Flags().GetString("rsh-operation"); operation != "" {
		if credentialID != "" {
			return fmt.Errorf("--rsh-operation and --rsh-credential are mutually exclusive")
		}
		return c.runAPIAuthInspectOperation(cmd, apiName, profileName, apiCfg, prof, operation, rawHeader)
	}

	resolved, selectedCredential, err := c.resolveAuthInspectionConfig(apiName, profileName, prof, credentialID)
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
	if rawHeader != "" {
		value := req.Header.Get(rawHeader)
		if value == "" {
			return fmt.Errorf("auth did not set header %q", rawHeader)
		}
		fmt.Fprintln(c.Stdout, value)
		return nil
	}
	if selectedCredential != "" {
		fmt.Fprintf(c.Stdout, "Credential: %s\n", selectedCredential)
	}
	fmt.Fprintf(c.Stdout, "Auth type: %s\n", resolved.Config.Type)
	for _, name := range sortedHeaderKeys(req.Header) {
		values := req.Header[name]
		for _, value := range values {
			if isSensitiveHeader(name) || isAuthInspectionSensitiveHeader(resolved.Config, name) {
				value = "<redacted>"
			}
			fmt.Fprintf(c.Stdout, "%s: %s\n", name, value)
		}
	}
	if req.URL.RawQuery != "" {
		fmt.Fprintf(c.Stdout, "Query: %s\n", redactedRequestURL(req.URL))
	}
	return nil
}

func (c *CLI) runAPIAuthInspectOperation(cmd *cobra.Command, apiName, profileName string, apiCfg *config.APIConfig, prof *config.ProfileConfig, operationName, rawHeader string) error {
	op, ok, err := c.cachedOperationForAPI(apiName, apiCfg, profileName, operationName)
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
		selected = []selectedOperationAuth{{requirement: spec.CredentialRequirement{ID: selectedCredential}, resolved: resolved}}
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
	c.printAuthInspectionRequest(req, selectedOperationAuthConfigs(selected))
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

func (c *CLI) operationAuthInspectionRequest(cmd *cobra.Command, apiName, profileName string, selected []selectedOperationAuth) (*http.Request, error) {
	authOpts, err := c.authHandlerOptionsFromCmd(cmd)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
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
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := handler.Authenticate(requestContext(cmd), req, c.authContext(requestContext(cmd), apiName, profileName, params, resolved.CacheKey, false)); err != nil {
		return nil, fmt.Errorf("building auth inspection: %w", err)
	}
	return req, nil
}

func (c *CLI) printAuthInspectionRequest(req *http.Request, configs []*config.AuthConfig) {
	for _, ac := range configs {
		if ac != nil {
			fmt.Fprintf(c.Stdout, "Auth type: %s\n", ac.Type)
		}
	}
	for _, name := range sortedHeaderKeys(req.Header) {
		values := req.Header[name]
		for _, value := range values {
			if isSensitiveHeader(name) || authInspectionSensitiveHeader(configs, name) {
				value = "<redacted>"
			}
			fmt.Fprintf(c.Stdout, "%s: %s\n", name, value)
		}
	}
	if req.URL.RawQuery != "" {
		fmt.Fprintf(c.Stdout, "Query: %s\n", redactedRequestURL(req.URL))
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

func selectedOperationCredentialIDs(selected []selectedOperationAuth) []string {
	ids := make([]string, 0, len(selected))
	for _, item := range selected {
		if item.requirement.ID != "" {
			ids = append(ids, item.requirement.ID)
		}
	}
	return ids
}

func selectedOperationAuthConfigs(selected []selectedOperationAuth) []*config.AuthConfig {
	configs := make([]*config.AuthConfig, 0, len(selected))
	for _, item := range selected {
		configs = append(configs, item.resolved.Config)
	}
	return configs
}

func (c *CLI) apiProfileForAuth(apiName, profileName string) (*config.APIConfig, *config.ProfileConfig, error) {
	apiCfg, err := c.requireAPI(apiName)
	if err != nil {
		return nil, nil, fmt.Errorf("%w; run \"restish api list\" to see configured APIs", err)
	}
	if apiCfg.Profiles == nil || apiCfg.Profiles[profileName] == nil {
		return nil, nil, fmt.Errorf("API %q has no profile %q; configured profiles: %s", apiName, profileName, profileNames(apiCfg.Profiles))
	}
	return apiCfg, apiCfg.Profiles[profileName], nil
}

func (c *CLI) cachedOperationSetForAPI(apiName string, apiCfg *config.APIConfig, profileName string) (spec.OperationSet, bool) {
	if apiCfg == nil {
		return spec.OperationSet{}, false
	}
	return spec.LoadOperationSetFromCache(c.specCacheDir(), apiName, Version, apiCfg.SpecFiles, spec.OperationOptions{
		BaseURL:         effectiveProfileBaseURL(apiCfg, profileName),
		OperationBase:   effectiveOperationBase(apiCfg, profileName),
		ServerVariables: effectiveServerVariables(apiCfg, profileName),
	})
}

func (c *CLI) cachedOperationForAPI(apiName string, apiCfg *config.APIConfig, profileName, value string) (spec.Operation, bool, error) {
	set, ok := c.cachedOperationSetForAPI(apiName, apiCfg, profileName)
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
	deprecated bool
}

func (c *CLI) printAPIAuthRequirementSummary(ops []spec.Operation, prof *config.ProfileConfig) {
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
				if credential.Auth != nil || credential.AuthRef != "" {
					status = "configured"
				} else {
					status = "empty"
				}
				satisfies = credential.Satisfies
			}
		}
		var parts []string
		parts = append(parts, status)
		if len(summary.needs) > 0 {
			parts = append(parts, "needs "+strings.Join(summary.needs, " "))
		}
		if len(satisfies) > 0 {
			parts = append(parts, "satisfies "+strings.Join(satisfies, " "))
		}
		if !authRequirementKindSupported(summary.kind) {
			parts = append(parts, "unsupported "+summary.kind)
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

func (c *CLI) cachedCredentialDefaultNeeds(apiName string, apiCfg *config.APIConfig, profileName, credentialID string) []string {
	set, ok := c.cachedOperationSetForAPI(apiName, apiCfg, profileName)
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

func (c *CLI) saveAPIAuthConfig(apiName string, apiCfg *config.APIConfig) error {
	return c.saveAPIConfig("api auth", apiName, c.cfg, apiCfg)
}
