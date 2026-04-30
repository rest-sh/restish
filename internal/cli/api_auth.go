package cli

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/config"
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
	fmt.Fprintf(c.Stdout, "API: %s\nProfile: %s\n", apiName, profileName)
	if prof.Auth != nil || prof.AuthRef != "" {
		fmt.Fprintln(c.Stdout, "Profile auth: configured")
	} else {
		fmt.Fprintln(c.Stdout, "Profile auth: none")
	}
	var ids []string
	for id := range prof.Credentials {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		fmt.Fprintln(c.Stdout, "Credentials: none")
		return nil
	}
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
	_ = apiCfg
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
	_, prof, err := c.apiProfileForAuth(apiName, profileName)
	if err != nil {
		return err
	}
	credentialID, _ := cmd.Flags().GetString("rsh-credential")
	rawHeader, _ := cmd.Flags().GetString("raw-header")
	if operation, _ := cmd.Flags().GetString("rsh-operation"); operation != "" && credentialID == "" {
		return fmt.Errorf("--rsh-operation is not implemented yet; pass --rsh-credential to inspect a specific credential")
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

func (c *CLI) apiProfileForAuth(apiName, profileName string) (*config.APIConfig, *config.ProfileConfig, error) {
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return nil, nil, fmt.Errorf("unknown API %q; run \"restish api list\" to see configured APIs", apiName)
	}
	apiCfg := c.cfg.APIs[apiName]
	if apiCfg.Profiles == nil || apiCfg.Profiles[profileName] == nil {
		return nil, nil, fmt.Errorf("API %q has no profile %q; configured profiles: %s", apiName, profileName, profileNames(apiCfg.Profiles))
	}
	return apiCfg, apiCfg.Profiles[profileName], nil
}

func (c *CLI) saveAPIAuthConfig(apiName string, apiCfg *config.APIConfig) error {
	cfgPath := c.configFilePath()
	if config.NeedsPatchToPreserveFormatting(cfgPath) {
		return config.SaveAPIConfig(cfgPath, apiName, apiCfg)
	}
	if c.cfg.APIs == nil {
		c.cfg.APIs = map[string]*config.APIConfig{}
	}
	c.cfg.APIs[apiName] = apiCfg
	return config.Save(cfgPath, c.cfg)
}
