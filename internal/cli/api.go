package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
	"github.com/spf13/cobra"
)

// addAPICommand registers the "api" subcommand tree on root.
func (c *CLI) addAPICommand(root *cobra.Command) {
	apiCmd := &cobra.Command{
		Use:     "api",
		Short:   "Manage registered API configurations",
		GroupID: rootGroupConfig,
	}
	clearAuthCmd := &cobra.Command{
		Use:   "clear-auth-cache <name>",
		Short: "Delete the cached OAuth2 token for a named API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runClearAuthCache,
	}
	clearAuthCmd.Flags().Bool("all", false, "Delete cached auth tokens for every profile of the named API")
	apiCmd.AddCommand(clearAuthCmd)
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
	syncCmd := &cobra.Command{
		Use:   "sync <name>",
		Short: "Force re-fetch of the cached OpenAPI spec for a named API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPISync,
	}
	syncCmd.Flags().Bool("allow-cross-origin-spec", false, "Allow Link-header spec discovery from another host for this sync run")
	apiCmd.AddCommand(syncCmd)
	apiCmd.AddCommand(&cobra.Command{
		Use:   "add <name> <url> [path:value ...]",
		Short: "Register a new API quickly; optional shorthand expressions set nested fields",
		Args:  cobra.MinimumNArgs(2),
		RunE:  c.runAPIAdd,
	})
	configureCmd := &cobra.Command{
		Use:   "configure <name> <url>",
		Short: "Register an API and pre-populate config from its OpenAPI spec",
		Args:  cobra.ExactArgs(2),
		RunE:  c.runAPIConfigure,
	}
	configureCmd.Flags().Bool("allow-cross-origin-spec", false, "Allow Link-header spec discovery from another host; private and loopback IP literals are still rejected")
	apiCmd.AddCommand(configureCmd)
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
		Use:   "set <name> <key> <value> | <name> <path:value>",
		Short: "Set API config using key/value or shorthand path:value syntax",
		Args:  cobra.MinimumNArgs(2),
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
	allProfiles, _ := cmd.Flags().GetBool("all")

	tc := auth.NewTokenCache(c.tokenCachePath())
	if allProfiles {
		if err := tc.DeletePrefix(apiName + ":"); err != nil {
			return fmt.Errorf("clear-auth-cache: %w", err)
		}
		fmt.Fprintf(c.Stdout, "Cleared auth cache for %q (all profiles)\n", apiName)
		return nil
	}
	key := apiName + ":" + profileName
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
	apiCfg := c.cfg.APIs[apiName]

	if err := spec.InvalidateCache(c.specCacheDir(), apiName); err != nil {
		return fmt.Errorf("api sync: invalidate cache: %w", err)
	}

	allowCrossOrigin, _ := cmd.Flags().GetBool("allow-cross-origin-spec")
	discCfg := spec.DiscoverConfig{
		APIName:          apiName,
		BaseURL:          apiCfg.BaseURL,
		SpecURL:          apiCfg.SpecURL,
		SpecFiles:        apiCfg.SpecFiles,
		CacheDir:         c.specCacheDir(),
		OperationBase:    apiCfg.OperationBase,
		Version:          Version,
		Transport:        c.baseHTTPTransport(),
		AllowCrossOrigin: apiCfg.AllowCrossOriginSpec || allowCrossOrigin,
	}
	apiSpec, err := spec.Discover(requestContext(cmd), discCfg, c.loaders)
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

func (c *CLI) runAPIAdd(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	baseURL := args[1]

	if isBuiltinCommandName(apiName) {
		return fmt.Errorf("API name %q conflicts with a built-in command; choose a different name", apiName)
	}
	if c.cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if c.cfg.APIs == nil {
		c.cfg.APIs = map[string]*config.APIConfig{}
	}
	if _, exists := c.cfg.APIs[apiName]; exists {
		return fmt.Errorf("API %q already exists", apiName)
	}

	work, err := cloneConfig(c.cfg)
	if err != nil {
		return err
	}
	if work.APIs == nil {
		work.APIs = map[string]*config.APIConfig{}
	}
	work.APIs[apiName] = &config.APIConfig{BaseURL: baseURL}

	exprs, err := parseAPISetExpressions(args[2:])
	if err != nil {
		return err
	}
	ops, err := c.buildAPIPatchOperations(work, apiName, exprs)
	if err != nil {
		return err
	}
	ops = append([]config.ConfigPatchOperation{{Path: []string{"apis", apiName}, Value: work.APIs[apiName]}}, ops...)

	cfgPath := c.configFilePath()
	if err := config.SaveConfigValues(cfgPath, ops); err != nil {
		return err
	}
	c.cfg = work
	return nil
}

// runAPIConfigure creates or updates the config entry for an API, pre-populating
// it from the API's OpenAPI spec x-cli-config extension if available.
func (c *CLI) runAPIConfigure(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	baseURL := args[1]
	allowCrossOrigin, _ := cmd.Flags().GetBool("allow-cross-origin-spec")

	if isBuiltinCommandName(apiName) {
		return fmt.Errorf("API name %q conflicts with a built-in command; choose a different name", apiName)
	}

	// Run spec discovery with the supplied base URL (no existing config needed).
	discCfg := spec.DiscoverConfig{
		APIName:          apiName,
		BaseURL:          baseURL,
		CacheDir:         c.specCacheDir(),
		Version:          Version,
		Transport:        c.baseHTTPTransport(),
		AllowCrossOrigin: allowCrossOrigin,
		ForceRefresh:     true,
	}
	apiSpec, _ := spec.Discover(requestContext(cmd), discCfg, c.loaders)

	// Build the API config entry.
	apiCfg := &config.APIConfig{
		BaseURL:              baseURL,
		AllowCrossOriginSpec: allowCrossOrigin,
	}
	if apiSpec != nil {
		xcli, _ := spec.ReadXCLIConfig(apiSpec)
		if xcli == nil {
			// No x-cli-config extension — try to derive auth from the spec's
			// declared security schemes.
			xcli = spec.FallbackXCLIConfig(apiSpec)
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

	if config.NeedsPatchToPreserveFormatting(cfgPath) {
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
		handler, err := c.authHandlerFor(prof.Auth, authHandlerOptions{})
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
	editorCmd.Stdin = c.Stdin
	editorCmd.Stdout = c.Stdout
	editorCmd.Stderr = c.Stderr
	return editorCmd.Run()
}

// runAPISet updates a single config field for a named API using a dot-path key.
// Supported keys: base_url, spec_url, allow_cross_origin_spec, operation_base,
// pagination.items_path, pagination.next_path,
// profiles.<name>.base_url, profiles.<name>.auth.type,
// profiles.<name>.auth.params.<param>, profiles.<name>.tls_signer.
func (c *CLI) runAPISet(cmd *cobra.Command, args []string) error {
	apiName := args[0]

	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}

	exprs, err := parseAPISetExpressions(args[1:])
	if err != nil {
		return err
	}
	work, err := cloneConfig(c.cfg)
	if err != nil {
		return err
	}
	ops, err := c.buildAPIPatchOperations(work, apiName, exprs)
	if err != nil {
		return err
	}

	cfgPath := c.configFilePath()
	if err := config.SaveConfigValues(cfgPath, ops); err != nil {
		return err
	}
	c.cfg = work
	return nil
}

type setExpression struct {
	key    string
	value  any
	append bool
	delete bool
}

func parseAPISetExpressions(args []string) ([]setExpression, error) {
	if len(args) == 0 {
		return nil, nil
	}
	if len(args) == 2 && !strings.Contains(args[0], ":") {
		v, err := parseConfigCLIValue(args[1])
		if err != nil {
			return nil, err
		}
		expr := setExpression{key: args[0], value: v}
		if strings.TrimSpace(args[1]) == "undefined" {
			expr.delete = true
			expr.value = nil
		}
		return []setExpression{expr}, nil
	}
	out := make([]setExpression, 0, len(args))
	for _, arg := range args {
		key, raw, appendOp, err := parseShorthandAssignment(arg)
		if err != nil {
			return nil, err
		}
		expr := setExpression{key: key, append: appendOp}
		if strings.TrimSpace(raw) == "undefined" {
			if appendOp {
				return nil, fmt.Errorf("invalid shorthand %q: append expressions cannot use undefined", arg)
			}
			expr.delete = true
			out = append(out, expr)
			continue
		}
		v, err := parseConfigCLIValue(raw)
		if err != nil {
			return nil, err
		}
		expr.value = v
		out = append(out, expr)
	}
	return out, nil
}

func parseShorthandAssignment(expr string) (string, string, bool, error) {
	key, raw, ok := strings.Cut(expr, ":")
	if !ok {
		return "", "", false, fmt.Errorf("invalid shorthand %q: expected path:value", expr)
	}
	key = strings.TrimSpace(key)
	raw = strings.TrimSpace(raw)
	if key == "" {
		return "", "", false, fmt.Errorf("invalid shorthand %q: empty path", expr)
	}
	if raw == "" {
		return "", "", false, fmt.Errorf("invalid shorthand %q: empty value", expr)
	}
	appendOp := false
	if strings.HasSuffix(key, "[]") {
		appendOp = true
		key = strings.TrimSuffix(key, "[]")
		key = strings.TrimSpace(key)
		if key == "" {
			return "", "", false, fmt.Errorf("invalid shorthand %q: empty path", expr)
		}
	}
	return key, raw, appendOp, nil
}

func parseConfigCLIValue(raw string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v, nil
	}
	return raw, nil
}

type apiConfigKeyKind int

const (
	apiKeyBaseURL apiConfigKeyKind = iota + 1
	apiKeySpecURL
	apiKeyAllowCrossOriginSpec
	apiKeyOperationBase
	apiKeyPaginationItemsPath
	apiKeyPaginationNextPath
	apiKeyProfileBaseURL
	apiKeyProfileHeaders
	apiKeyProfileQuery
	apiKeyProfileTLSSigner
	apiKeyProfileAuthType
	apiKeyProfileAuthParam
)

type resolvedAPIConfigKey struct {
	kind        apiConfigKeyKind
	jsonPath    []string
	profileName string
	paramName   string
}

func resolveAPIConfigKey(apiName, key string) (resolvedAPIConfigKey, error) {
	parts := strings.SplitN(key, ".", 3)
	basePath := []string{"apis"}
	if apiName != "" {
		basePath = append(basePath, apiName)
	}

	switch parts[0] {
	case "base_url":
		return resolvedAPIConfigKey{kind: apiKeyBaseURL, jsonPath: append(basePath, "base_url")}, nil
	case "spec_url":
		return resolvedAPIConfigKey{kind: apiKeySpecURL, jsonPath: append(basePath, "spec_url")}, nil
	case "allow_cross_origin_spec":
		return resolvedAPIConfigKey{kind: apiKeyAllowCrossOriginSpec, jsonPath: append(basePath, "allow_cross_origin_spec")}, nil
	case "operation_base":
		return resolvedAPIConfigKey{kind: apiKeyOperationBase, jsonPath: append(basePath, "operation_base")}, nil
	case "pagination":
		if len(parts) < 2 {
			return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected pagination.<field>", key)
		}
		switch parts[1] {
		case "items_path":
			return resolvedAPIConfigKey{kind: apiKeyPaginationItemsPath, jsonPath: append(basePath, "pagination", "items_path")}, nil
		case "next_path":
			return resolvedAPIConfigKey{kind: apiKeyPaginationNextPath, jsonPath: append(basePath, "pagination", "next_path")}, nil
		default:
			return resolvedAPIConfigKey{}, fmt.Errorf("unsupported pagination field %q; supported: items_path, next_path", parts[1])
		}
	case "profiles":
		if len(parts) < 3 {
			return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected profiles.<name>.<field>", key)
		}
		profileName := parts[1]
		subParts := strings.SplitN(parts[2], ".", 3)
		switch subParts[0] {
		case "base_url":
			return resolvedAPIConfigKey{
				kind:        apiKeyProfileBaseURL,
				jsonPath:    append(basePath, "profiles", profileName, "base_url"),
				profileName: profileName,
			}, nil
		case "headers":
			return resolvedAPIConfigKey{
				kind:        apiKeyProfileHeaders,
				jsonPath:    append(basePath, "profiles", profileName, "headers"),
				profileName: profileName,
			}, nil
		case "query":
			return resolvedAPIConfigKey{
				kind:        apiKeyProfileQuery,
				jsonPath:    append(basePath, "profiles", profileName, "query"),
				profileName: profileName,
			}, nil
		case "tls_signer":
			return resolvedAPIConfigKey{
				kind:        apiKeyProfileTLSSigner,
				jsonPath:    append(basePath, "profiles", profileName, "tls_signer"),
				profileName: profileName,
			}, nil
		case "auth":
			if len(subParts) < 2 {
				return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected profiles.<name>.auth.<field>", key)
			}
			switch subParts[1] {
			case "type":
				return resolvedAPIConfigKey{
					kind:        apiKeyProfileAuthType,
					jsonPath:    append(basePath, "profiles", profileName, "auth", "type"),
					profileName: profileName,
				}, nil
			case "params":
				if len(subParts) < 3 {
					return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected profiles.<name>.auth.params.<param>", key)
				}
				return resolvedAPIConfigKey{
					kind:        apiKeyProfileAuthParam,
					jsonPath:    append(basePath, "profiles", profileName, "auth", "params", subParts[2]),
					profileName: profileName,
					paramName:   subParts[2],
				}, nil
			default:
				return resolvedAPIConfigKey{}, fmt.Errorf("unsupported auth field %q; supported: type, params.<param>", subParts[1])
			}
		default:
			return resolvedAPIConfigKey{}, fmt.Errorf("unsupported profile field %q; supported: base_url, headers, query, tls_signer, auth.type, auth.params.<param>", parts[2])
		}
	default:
		return resolvedAPIConfigKey{}, fmt.Errorf("unsupported field %q; supported: base_url, spec_url, allow_cross_origin_spec, operation_base, pagination.items_path, pagination.next_path, profiles.<name>.base_url, profiles.<name>.headers, profiles.<name>.query, profiles.<name>.tls_signer, profiles.<name>.auth.type, profiles.<name>.auth.params.<param>", key)
	}
}

func cloneConfig(src *config.Config) (*config.Config, error) {
	if src == nil {
		return &config.Config{}, nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}
	var dst config.Config
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

func (c *CLI) buildAPIPatchOperations(work *config.Config, apiName string, exprs []setExpression) ([]config.ConfigPatchOperation, error) {
	if len(exprs) == 0 {
		return nil, nil
	}
	if work.APIs == nil || work.APIs[apiName] == nil {
		return nil, fmt.Errorf("unknown API %q", apiName)
	}
	apiCfg := work.APIs[apiName]
	ops := make([]config.ConfigPatchOperation, 0, len(exprs))
	for _, expr := range exprs {
		resolved, err := resolveAPIConfigKey(apiName, expr.key)
		if err != nil {
			return nil, err
		}

		switch {
		case expr.delete:
			if err := deleteAPIField(apiCfg, resolved); err != nil {
				return nil, err
			}
			ops = append(ops, config.ConfigPatchOperation{Path: resolved.jsonPath, Delete: true})
		case expr.append:
			if err := appendAPIField(apiCfg, resolved, expr.value); err != nil {
				return nil, err
			}
			finalValue, err := apiFieldValue(apiCfg, resolved)
			if err != nil {
				return nil, err
			}
			ops = append(ops, config.ConfigPatchOperation{Path: resolved.jsonPath, Value: finalValue})
		default:
			if err := setAPIFieldValue(c, apiCfg, resolved, expr.value); err != nil {
				return nil, err
			}
			finalValue, err := apiFieldValue(apiCfg, resolved)
			if err != nil {
				return nil, err
			}
			ops = append(ops, config.ConfigPatchOperation{Path: resolved.jsonPath, Value: finalValue})
		}
	}
	return ops, nil
}

func ensureProfile(apiCfg *config.APIConfig, profileName string) *config.ProfileConfig {
	if apiCfg.Profiles == nil {
		apiCfg.Profiles = make(map[string]*config.ProfileConfig)
	}
	if apiCfg.Profiles[profileName] == nil {
		apiCfg.Profiles[profileName] = &config.ProfileConfig{}
	}
	return apiCfg.Profiles[profileName]
}

func deleteAPIField(apiCfg *config.APIConfig, resolved resolvedAPIConfigKey) error {
	switch resolved.kind {
	case apiKeyBaseURL:
		apiCfg.BaseURL = ""
	case apiKeySpecURL:
		apiCfg.SpecURL = ""
	case apiKeyAllowCrossOriginSpec:
		apiCfg.AllowCrossOriginSpec = false
	case apiKeyOperationBase:
		apiCfg.OperationBase = ""
	case apiKeyPaginationItemsPath:
		if apiCfg.Pagination != nil {
			apiCfg.Pagination.ItemsPath = ""
		}
	case apiKeyPaginationNextPath:
		if apiCfg.Pagination != nil {
			apiCfg.Pagination.NextPath = ""
		}
	case apiKeyProfileBaseURL:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil {
			p.BaseURL = ""
		}
	case apiKeyProfileHeaders:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil {
			p.Headers = nil
		}
	case apiKeyProfileQuery:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil {
			p.Query = nil
		}
	case apiKeyProfileTLSSigner:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil {
			p.TLSSigner = ""
		}
	case apiKeyProfileAuthType:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Auth != nil {
			p.Auth.Type = ""
		}
	case apiKeyProfileAuthParam:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Auth != nil && p.Auth.Params != nil {
			delete(p.Auth.Params, resolved.paramName)
		}
	default:
		return fmt.Errorf("unsupported field %q", strings.Join(resolved.jsonPath, "."))
	}
	return nil
}

func appendAPIField(apiCfg *config.APIConfig, resolved resolvedAPIConfigKey, value any) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("append expects a string value")
	}
	prof := ensureProfile(apiCfg, resolved.profileName)
	switch resolved.kind {
	case apiKeyProfileHeaders:
		prof.Headers = append(prof.Headers, v)
	case apiKeyProfileQuery:
		prof.Query = append(prof.Query, v)
	default:
		return fmt.Errorf("append is only supported for profiles.<name>.headers[] and profiles.<name>.query[]")
	}
	return nil
}

func setAPIFieldValue(c *CLI, apiCfg *config.APIConfig, resolved resolvedAPIConfigKey, value any) error {
	switch resolved.kind {
	case apiKeyBaseURL:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("base_url must be a string")
		}
		apiCfg.BaseURL = v
	case apiKeySpecURL:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("spec_url must be a string")
		}
		apiCfg.SpecURL = v
	case apiKeyAllowCrossOriginSpec:
		b, ok := value.(bool)
		if !ok {
			return fmt.Errorf("allow_cross_origin_spec must be a boolean")
		}
		apiCfg.AllowCrossOriginSpec = b
	case apiKeyOperationBase:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("operation_base must be a string")
		}
		apiCfg.OperationBase = v
	case apiKeyPaginationItemsPath:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("pagination.items_path must be a string")
		}
		if apiCfg.Pagination == nil {
			apiCfg.Pagination = &config.PaginationConfig{}
		}
		apiCfg.Pagination.ItemsPath = v
	case apiKeyPaginationNextPath:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("pagination.next_path must be a string")
		}
		if apiCfg.Pagination == nil {
			apiCfg.Pagination = &config.PaginationConfig{}
		}
		apiCfg.Pagination.NextPath = v
	case apiKeyProfileBaseURL:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.base_url must be a string", resolved.profileName)
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		prof.BaseURL = v
	case apiKeyProfileHeaders:
		arr, err := anyToStringSlice(value)
		if err != nil {
			return fmt.Errorf("profiles.%s.headers must be a string array", resolved.profileName)
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		prof.Headers = arr
	case apiKeyProfileQuery:
		arr, err := anyToStringSlice(value)
		if err != nil {
			return fmt.Errorf("profiles.%s.query must be a string array", resolved.profileName)
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		prof.Query = arr
	case apiKeyProfileTLSSigner:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.tls_signer must be a string", resolved.profileName)
		}
		if v != "" {
			if _, ok := c.pluginForHook(v, "tls-signer"); !ok {
				return fmt.Errorf("tls_signer %q is not a registered tls-signer plugin", v)
			}
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		prof.TLSSigner = v
	case apiKeyProfileAuthType:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.auth.type must be a string", resolved.profileName)
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		if prof.Auth == nil {
			prof.Auth = &config.AuthConfig{}
		}
		candidate := &config.AuthConfig{Type: v, Params: map[string]string{}}
		if _, err := c.authHandlerFor(candidate, authHandlerOptions{}); err != nil {
			return fmt.Errorf("invalid auth.type %q: %w", v, err)
		}
		prof.Auth.Type = v
	case apiKeyProfileAuthParam:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.auth.params.%s must be a string", resolved.profileName, resolved.paramName)
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		if prof.Auth == nil {
			prof.Auth = &config.AuthConfig{}
		}
		if prof.Auth.Params == nil {
			prof.Auth.Params = map[string]string{}
		}
		prof.Auth.Params[resolved.paramName] = v
	default:
		return fmt.Errorf("unsupported field %q", strings.Join(resolved.jsonPath, "."))
	}
	return nil
}

func anyToStringSlice(value any) ([]string, error) {
	switch v := value.(type) {
	case string:
		return []string{v}, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("array element is not a string")
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("not a string slice")
	}
}

func apiFieldValue(apiCfg *config.APIConfig, resolved resolvedAPIConfigKey) (any, error) {
	switch resolved.kind {
	case apiKeyBaseURL:
		return apiCfg.BaseURL, nil
	case apiKeySpecURL:
		return apiCfg.SpecURL, nil
	case apiKeyAllowCrossOriginSpec:
		return apiCfg.AllowCrossOriginSpec, nil
	case apiKeyOperationBase:
		return apiCfg.OperationBase, nil
	case apiKeyPaginationItemsPath:
		if apiCfg.Pagination == nil {
			return "", nil
		}
		return apiCfg.Pagination.ItemsPath, nil
	case apiKeyPaginationNextPath:
		if apiCfg.Pagination == nil {
			return "", nil
		}
		return apiCfg.Pagination.NextPath, nil
	case apiKeyProfileBaseURL:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil {
			return p.BaseURL, nil
		}
		return "", nil
	case apiKeyProfileHeaders:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil {
			return p.Headers, nil
		}
		return []string{}, nil
	case apiKeyProfileQuery:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil {
			return p.Query, nil
		}
		return []string{}, nil
	case apiKeyProfileTLSSigner:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil {
			return p.TLSSigner, nil
		}
		return "", nil
	case apiKeyProfileAuthType:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Auth != nil {
			return p.Auth.Type, nil
		}
		return "", nil
	case apiKeyProfileAuthParam:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Auth != nil && p.Auth.Params != nil {
			return p.Auth.Params[resolved.paramName], nil
		}
		return "", nil
	default:
		return nil, fmt.Errorf("unsupported field")
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
	if config.NeedsPatchToPreserveFormatting(cfgPath) {
		if err := config.DeleteAPIConfig(cfgPath, apiName); err != nil {
			return err
		}
	} else if err := config.Save(cfgPath, c.cfg); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Deleted API %q\n", apiName)
	return nil
}
