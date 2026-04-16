package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/danielgtaylor/restish/v2/internal/auth"
	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/spec"
	"github.com/spf13/cobra"
)

// addAPICommand registers the "api" subcommand tree on root.
func (c *CLI) addAPICommand(root *cobra.Command) {
	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "Manage registered API configurations",
	}
	apiCmd.AddCommand(&cobra.Command{
		Use:   "clear-auth-cache <name>",
		Short: "Delete the cached OAuth2 token for a named API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runClearAuthCache,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all configured APIs",
		Args:  cobra.NoArgs,
		RunE:  c.runAPIList,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "delete <name>",
		Short: "Remove a configured API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPIDelete,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "sync <name>",
		Short: "Force re-fetch of the cached OpenAPI spec for a named API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPISync,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "configure <name> <url>",
		Short: "Register an API and pre-populate config from its OpenAPI spec",
		Args:  cobra.ExactArgs(2),
		RunE:  c.runAPIConfigure,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "Print the config for a registered API as JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPIShow,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the restish config file in $VISUAL or $EDITOR",
		Args:  cobra.NoArgs,
		RunE:  c.runAPIEdit,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "set <name> <key> <value>",
		Short: "Set a config field for a registered API by dot-path key",
		Args:  cobra.ExactArgs(3),
		RunE:  c.runAPISet,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "content-types",
		Short: "List registered content types and their MIME types",
		Args:  cobra.NoArgs,
		RunE:  c.runAPIContentTypes,
	})
	root.AddCommand(apiCmd)
}

// runClearAuthCache deletes the token cache entry for the named API+profile.
func (c *CLI) runClearAuthCache(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}

	profileName := c.profileFromCmd(cmd)

	key := apiName + ":" + profileName
	tc := auth.NewTokenCache(c.tokenCachePath())
	if err := tc.Delete(key); err != nil {
		return fmt.Errorf("clear-auth-cache: %w", err)
	}
	fmt.Fprintf(c.Stdout, "Cleared auth cache for %q (profile %q)\n", apiName, profileName)
	return nil
}

// runAPISync force-invalidates the cached spec for an API and fetches a fresh one.
func (c *CLI) runAPISync(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}

	if err := spec.InvalidateCache(c.specCacheDir(), apiName); err != nil {
		return fmt.Errorf("api sync: invalidate cache: %w", err)
	}

	apiSpec, err := c.discoverSpec(requestContext(cmd), apiName)
	if err != nil {
		return fmt.Errorf("api sync: %w", err)
	}

	if apiSpec != nil {
		fmt.Fprintf(c.Stdout, "Synced spec for %q.\n", apiName)
	} else {
		fmt.Fprintf(c.Stdout, "No spec found for %q.\n", apiName)
	}
	return nil
}

// runAPIConfigure creates or updates the config entry for an API, pre-populating
// it from the API's OpenAPI spec x-cli-config extension if available.
func (c *CLI) runAPIConfigure(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	baseURL := args[1]

	// Run spec discovery with the supplied base URL (no existing config needed).
	discCfg := spec.DiscoverConfig{
		APIName:   apiName,
		BaseURL:   baseURL,
		CacheDir:  c.specCacheDir(),
		Version:   Version,
		Transport: c.baseHTTPTransport(),
	}
	apiSpec, _ := spec.Discover(requestContext(cmd), discCfg, c.loaders)

	// Build the API config entry.
	apiCfg := &config.APIConfig{BaseURL: baseURL}
	if apiSpec != nil {
		xcli, _ := spec.ReadXCLIConfig(apiSpec)
		if xcli == nil && apiSpec.Document != nil {
			// No x-cli-config extension — try to derive auth from the spec's
			// declared security schemes.
			xcli = spec.FallbackXCLIConfig(apiSpec.Document)
		}
		if xcli != nil {
			c.applyXCLIConfig(apiCfg, xcli.Resolve(apiSpec))
		}
	}

	// Load, update, and save the config.
	cfgPath := c.configFilePath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if cfg.APIs == nil {
		cfg.APIs = make(map[string]*config.APIConfig)
	}
	cfg.APIs[apiName] = apiCfg

	if config.HasComments(cfgPath) {
		if err := config.SaveAPIConfig(cfgPath, apiName, apiCfg); err != nil {
			return err
		}
	} else if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	if apiSpec != nil {
		fmt.Fprintf(c.Stdout, "Configured API %q with base URL %s (spec loaded — run 'restish %s --help')\n", apiName, baseURL, apiName)
	} else {
		fmt.Fprintf(c.Stdout, "Configured API %q with base URL %s (no spec found — run 'restish api sync %s' after connecting)\n", apiName, baseURL, apiName)
	}
	return nil
}

// applyXCLIConfig merges x-cli-config fields into apiCfg.
// Auth type "external-tool" is rejected: a server-provided x-cli-config
// could otherwise pre-seed arbitrary shell-command execution on the next
// authenticated request.
func (c *CLI) applyXCLIConfig(apiCfg *config.APIConfig, xcli *spec.XCLIConfig) {
	if len(xcli.Profiles) == 0 {
		return
	}
	if apiCfg.Profiles == nil {
		apiCfg.Profiles = make(map[string]*config.ProfileConfig)
	}
	for name, xp := range xcli.Profiles {
		prof := &config.ProfileConfig{
			Headers: xp.Headers,
			Query:   xp.Query,
		}
		if xp.Auth != nil {
			if xp.Auth.Type == "external-tool" {
				fmt.Fprintf(c.Stderr, "warning: x-cli-config: auth type %q is not permitted; skipping profile %q\n", xp.Auth.Type, name)
				continue
			}
			prof.Auth = &config.AuthConfig{
				Type:   xp.Auth.Type,
				Params: xp.Auth.Params,
			}
		}
		apiCfg.Profiles[name] = prof
	}
}

// runAPIShow prints the config for a named API as indented JSON,
// with secret auth params replaced by "***".
func (c *CLI) runAPIShow(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}

	// Round-trip through JSON so we can redact secrets without modifying the live config.
	raw, err := json.Marshal(c.cfg.APIs[apiName])
	if err != nil {
		return err
	}
	var view map[string]any
	if err := json.Unmarshal(raw, &view); err != nil {
		return err
	}
	c.redactAPIShowSecrets(c.cfg.APIs[apiName], view)

	data, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, string(data))
	return nil
}

// redactAPIShowSecrets replaces secret auth param values with "***" in the
// JSON view map so they are not printed in plaintext.
func (c *CLI) redactAPIShowSecrets(apiCfg *config.APIConfig, view map[string]any) {
	profiles, _ := view["profiles"].(map[string]any)
	if profiles == nil {
		return
	}
	for profName, profAny := range profiles {
		profMap, _ := profAny.(map[string]any)
		if profMap == nil {
			continue
		}
		authMap, _ := profMap["auth"].(map[string]any)
		if authMap == nil {
			continue
		}
		params, _ := authMap["params"].(map[string]any)
		if params == nil {
			continue
		}
		prof := apiCfg.Profiles[profName]
		if prof == nil || prof.Auth == nil {
			continue
		}
		handler, err := c.authHandlerFor(prof.Auth)
		if err != nil {
			continue
		}
		for _, p := range handler.Parameters() {
			if p.Secret {
				if _, ok := params[p.Name]; ok {
					params[p.Name] = "***"
				}
			}
		}
	}
}

// runAPIEdit opens the restish config file in $VISUAL or $EDITOR.
func (c *CLI) runAPIEdit(cmd *cobra.Command, args []string) error {
	cfgPath := c.configFilePath()
	editorCmd, err := c.editorCommand(cfgPath)
	if err != nil {
		return err
	}
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	return editorCmd.Run()
}

// runAPISet updates a single config field for a named API using a dot-path key.
// Supported keys: base_url, spec_url, operation_base,
// pagination.items_path, pagination.next_path,
// profiles.<name>.base_url, profiles.<name>.auth.type,
// profiles.<name>.auth.params.<param>, profiles.<name>.tls_signer.
func (c *CLI) runAPISet(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	key := args[1]
	value := args[2]

	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}

	apiCfg := c.cfg.APIs[apiName]
	if err := setAPIField(apiCfg, key, value); err != nil {
		return err
	}

	cfgPath := c.configFilePath()
	if config.HasComments(cfgPath) {
		jsonPath, err := apiConfigJSONPath(apiName, key)
		if err != nil {
			return err
		}
		return config.SaveConfigValue(cfgPath, jsonPath, value)
	}
	return config.Save(cfgPath, c.cfg)
}

// setAPIField updates a single field of apiCfg identified by a dot-path key.
// Supported keys:
//
//	base_url
//	spec_url
//	operation_base
//	pagination.items_path
//	pagination.next_path
//	profiles.<name>.base_url
//	profiles.<name>.auth.type
//	profiles.<name>.auth.params.<param>
//	profiles.<name>.tls_signer
func setAPIField(apiCfg *config.APIConfig, key, value string) error {
	parts := strings.SplitN(key, ".", 3)
	switch parts[0] {
	case "base_url":
		apiCfg.BaseURL = value
	case "spec_url":
		apiCfg.SpecURL = value
	case "operation_base":
		apiCfg.OperationBase = value
	case "pagination":
		if len(parts) < 2 {
			return fmt.Errorf("invalid key %q: expected pagination.<field>", key)
		}
		if apiCfg.Pagination == nil {
			apiCfg.Pagination = &config.PaginationConfig{}
		}
		switch parts[1] {
		case "items_path":
			apiCfg.Pagination.ItemsPath = value
		case "next_path":
			apiCfg.Pagination.NextPath = value
		default:
			return fmt.Errorf("unsupported pagination field %q", parts[1])
		}
	case "profiles":
		if len(parts) < 3 {
			return fmt.Errorf("invalid key %q: expected profiles.<name>.<field>", key)
		}
		profileName := parts[1]
		if apiCfg.Profiles == nil {
			apiCfg.Profiles = make(map[string]*config.ProfileConfig)
		}
		if apiCfg.Profiles[profileName] == nil {
			apiCfg.Profiles[profileName] = &config.ProfileConfig{}
		}
		prof := apiCfg.Profiles[profileName]
		subParts := strings.SplitN(parts[2], ".", 3)
		switch subParts[0] {
		case "base_url":
			prof.BaseURL = value
		case "tls_signer":
			prof.TLSSigner = value
		case "auth":
			if len(subParts) < 2 {
				return fmt.Errorf("invalid key %q: expected profiles.<name>.auth.<field>", key)
			}
			if prof.Auth == nil {
				prof.Auth = &config.AuthConfig{}
			}
			switch subParts[1] {
			case "type":
				prof.Auth.Type = value
			case "params":
				if len(subParts) < 3 {
					return fmt.Errorf("invalid key %q: expected profiles.<name>.auth.params.<param>", key)
				}
				if prof.Auth.Params == nil {
					prof.Auth.Params = make(map[string]string)
				}
				prof.Auth.Params[subParts[2]] = value
			default:
				return fmt.Errorf("unsupported auth field %q", subParts[1])
			}
		default:
			return fmt.Errorf("unsupported profile field %q", parts[2])
		}
	default:
		return fmt.Errorf("unsupported field %q", key)
	}
	return nil
}

func apiConfigJSONPath(apiName, key string) ([]string, error) {
	parts := strings.SplitN(key, ".", 3)
	path := []string{"apis", apiName}
	switch parts[0] {
	case "base_url", "spec_url", "operation_base":
		return append(path, parts[0]), nil
	case "pagination":
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid key %q: expected pagination.<field>", key)
		}
		switch parts[1] {
		case "items_path", "next_path":
			return append(path, "pagination", parts[1]), nil
		default:
			return nil, fmt.Errorf("unsupported pagination field %q", parts[1])
		}
	case "profiles":
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid key %q: expected profiles.<name>.<field>", key)
		}
		profileName := parts[1]
		subParts := strings.SplitN(parts[2], ".", 3)
		switch subParts[0] {
		case "base_url", "tls_signer":
			return append(path, "profiles", profileName, subParts[0]), nil
		case "auth":
			if len(subParts) < 2 {
				return nil, fmt.Errorf("invalid key %q: expected profiles.<name>.auth.<field>", key)
			}
			switch subParts[1] {
			case "type":
				return append(path, "profiles", profileName, "auth", "type"), nil
			case "params":
				if len(subParts) < 3 {
					return nil, fmt.Errorf("invalid key %q: expected profiles.<name>.auth.params.<param>", key)
				}
				return append(path, "profiles", profileName, "auth", "params", subParts[2]), nil
			default:
				return nil, fmt.Errorf("unsupported auth field %q", subParts[1])
			}
		default:
			return nil, fmt.Errorf("unsupported profile field %q", parts[2])
		}
	default:
		return nil, fmt.Errorf("unsupported field %q", key)
	}
}

// runAPIContentTypes lists all registered content types and their MIME types.
func (c *CLI) runAPIContentTypes(cmd *cobra.Command, args []string) error {
	for _, ct := range c.content.ContentTypes() {
		fmt.Fprintf(c.Stdout, "%-12s %s\n", ct.Name, strings.Join(ct.MIMETypes, ", "))
	}
	return nil
}

// runAPIList prints all configured APIs with their base URL and profile count.
func (c *CLI) runAPIList(cmd *cobra.Command, args []string) error {
	if c.cfg == nil || len(c.cfg.APIs) == 0 {
		fmt.Fprintln(c.Stdout, "No APIs configured.")
		return nil
	}
	names := make([]string, 0, len(c.cfg.APIs))
	for name := range c.cfg.APIs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		api := c.cfg.APIs[name]
		profileCount := len(api.Profiles)
		profileSuffix := fmt.Sprintf("%d profile", profileCount)
		if profileCount != 1 {
			profileSuffix += "s"
		}
		fmt.Fprintf(c.Stdout, "%-20s %-40s %s\n", name, api.BaseURL, profileSuffix)
	}
	return nil
}

// runAPIDelete removes a configured API and saves the updated config.
func (c *CLI) runAPIDelete(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}
	delete(c.cfg.APIs, apiName)
	cfgPath := c.configFilePath()
	if config.HasComments(cfgPath) {
		if err := config.DeleteAPIConfig(cfgPath, apiName); err != nil {
			return err
		}
	} else if err := config.Save(cfgPath, c.cfg); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Deleted API %q\n", apiName)
	return nil
}
