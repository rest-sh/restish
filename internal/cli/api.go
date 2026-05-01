package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/request"
	"github.com/rest-sh/restish/v2/internal/spec"
	"github.com/spf13/cobra"
)

// addAPICommand registers the "api" subcommand tree on root.
func (c *CLI) addAPICommand(root *cobra.Command) {
	apiCmd := &cobra.Command{
		Use:     "api",
		Short:   "Manage registered API configurations",
		GroupID: rootGroupConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown api command %q", args[0])
			}
			return cmd.Help()
		},
	}
	apiCmd.AddCommand(c.newAPIAuthCommand())
	apiCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all configured APIs",
		Args:  cobra.NoArgs,
		RunE:  c.runAPIList,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a configured API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPIRemove,
	})
	syncCmd := &cobra.Command{
		Use:   "sync <name>",
		Short: "Force re-fetch of the cached OpenAPI spec for a named API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPISync,
	}
	syncCmd.Flags().Bool("allow-cross-origin-spec", false, "Allow Link-header spec discovery from another host for this sync run")
	apiCmd.AddCommand(syncCmd)
	connectCmd := &cobra.Command{
		Use:   "connect <name> <url> [setup-expression ...]",
		Short: "Connect Restish to an API and discover generated commands",
		Args:  cobra.MinimumNArgs(2),
		RunE:  c.runAPIConnect,
	}
	connectCmd.Flags().Bool("allow-cross-origin-spec", false, "Allow Link-header spec discovery from another host; private and loopback IP literals are still rejected")
	connectCmd.Flags().Bool("no-discover", false, "Register the API locally without network spec discovery")
	connectCmd.Flags().String("spec", "", "OpenAPI spec URL or local file to use instead of discovery")
	connectCmd.Flags().Bool("replace", false, "Replace existing profiles with discovered OpenAPI/x-cli-config defaults")
	apiCmd.AddCommand(connectCmd)
	apiCmd.AddCommand(&cobra.Command{
		Use:   "inspect <name>",
		Short: "Print the config for a registered API as JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAPIInspect,
	})
	apiCmd.AddCommand(&cobra.Command{
		Use:   "set <name> <key> <value> | <name> <path:value>",
		Short: "Set API config using key/value or shorthand path:value syntax",
		Args:  cobra.MinimumNArgs(2),
		RunE:  c.runAPISet,
	})
	root.AddCommand(apiCmd)
}

// runClearAuthCache deletes the token cache entry for the named API+profile.
func (c *CLI) runAPIAuthClearCache(cmd *cobra.Command, args []string) error {
	authProfile, _ := cmd.Flags().GetString("auth-profile")
	tc := auth.NewTokenCache(c.tokenCachePath())
	if authProfile != "" {
		if len(args) > 0 {
			return fmt.Errorf("--auth-profile cannot be used with an API argument")
		}
		if c.cfg == nil || c.cfg.AuthProfiles == nil || c.cfg.AuthProfiles[authProfile] == nil {
			return fmt.Errorf("unknown auth profile %q", authProfile)
		}
		if err := tc.DeletePrefix("auth_profile:" + authProfile + ":"); err != nil {
			return fmt.Errorf("auth clear-cache: %w", err)
		}
		fmt.Fprintf(c.Stdout, "Cleared auth cache for auth profile %q\n", authProfile)
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("api auth clear-cache requires an API name or --auth-profile <name>")
	}
	apiName := args[0]
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}

	profileName := c.profileFromCmd(cmd)
	allProfiles, _ := cmd.Flags().GetBool("all-profiles")

	if allProfiles {
		if err := tc.DeletePrefix(apiName + ":"); err != nil {
			return fmt.Errorf("auth clear-cache: %w", err)
		}
		for _, prof := range c.cfg.APIs[apiName].Profiles {
			resolved, err := c.resolveProfileAuth(apiName, "", prof)
			if err != nil {
				return err
			}
			if resolved.Ref != "" {
				if err := tc.DeletePrefix("auth_profile:" + resolved.Ref + ":"); err != nil {
					return fmt.Errorf("auth clear-cache: %w", err)
				}
			}
		}
		fmt.Fprintf(c.Stdout, "Cleared auth cache for %q (all profiles)\n", apiName)
		return nil
	}
	key := apiName + ":" + profileName
	if err := tc.Delete(key); err != nil {
		return fmt.Errorf("auth clear-cache: %w", err)
	}
	if prof := c.cfg.APIs[apiName].Profiles[profileName]; prof != nil {
		resolved, err := c.resolveProfileAuth(apiName, profileName, prof)
		if err != nil {
			return err
		}
		if resolved.CacheKey != "" {
			if err := tc.Delete(resolved.CacheKey); err != nil {
				return fmt.Errorf("auth clear-cache: %w", err)
			}
		}
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

	allowCrossOrigin, _ := cmd.Flags().GetBool("allow-cross-origin-spec")
	transport, closer, err := c.discoveryTransport(requestContext(cmd), apiCfg, c.profileFromCmd(cmd))
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}
	discCfg := spec.DiscoverConfig{
		APIName:          apiName,
		BaseURL:          apiCfg.BaseURL,
		SpecURL:          apiCfg.SpecURL,
		SpecFiles:        apiCfg.SpecFiles,
		CacheDir:         c.specCacheDir(),
		OperationBase:    apiCfg.OperationBase,
		ServerVariables:  effectiveServerVariables(apiCfg, c.profileFromCmd(cmd)),
		Version:          Version,
		Transport:        transport,
		AllowCrossOrigin: apiCfg.AllowCrossOriginSpec || allowCrossOrigin,
		ForceRefresh:     true,
	}
	apiSpec, err := spec.Discover(requestContext(cmd), discCfg, c.loaders)
	if err != nil {
		return fmt.Errorf("api sync %q failed: %w\nhint: API registration and existing spec cache were left unchanged; check the network and spec_url, then retry api sync", apiName, err)
	}

	if apiSpec != nil {
		fmt.Fprintf(c.Stdout, "Synced spec for %q.\n", apiName)
	} else {
		fmt.Fprintf(c.Stdout, "No spec found for %q.\n", apiName)
	}
	return nil
}

// runAPIConnect creates or updates the config entry for an API, pre-populating
// it from the API's OpenAPI spec x-cli-config extension if available.
func (c *CLI) runAPIConnect(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	baseURL, err := normalizeAPIBaseURL(args[1])
	if err != nil {
		return err
	}
	allowCrossOrigin, _ := cmd.Flags().GetBool("allow-cross-origin-spec")
	noDiscover, _ := cmd.Flags().GetBool("no-discover")
	explicitSpec, _ := cmd.Flags().GetString("spec")
	replaceProfiles, _ := cmd.Flags().GetBool("replace")
	promptAnswers, setupExprs, err := parseAPIConfigureSetupExpressions(args[2:])
	if err != nil {
		return err
	}
	if noDiscover && explicitSpec != "" {
		return fmt.Errorf("--no-discover cannot be used with --spec")
	}

	if isBuiltinCommandName(apiName) {
		return fmt.Errorf("API name %q conflicts with a built-in command; choose a different name", apiName)
	}
	if err := config.ValidateAPIName(apiName); err != nil {
		return fmt.Errorf("invalid API name %q: %w", apiName, err)
	}

	cfgPath := c.configFilePath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	var existingAPI *config.APIConfig
	if cfg.APIs != nil {
		existingAPI = cfg.APIs[apiName]
	}

	// Build the API config entry.
	apiCfg := &config.APIConfig{
		BaseURL:              baseURL,
		AllowCrossOriginSpec: allowCrossOrigin,
	}
	applyExplicitSpec(apiCfg, explicitSpec)

	var apiSpec *spec.APISpec
	if !noDiscover {
		transport, closer, err := c.discoveryTransport(requestContext(cmd), apiCfg, "default")
		if err != nil {
			return err
		}
		if closer != nil {
			defer closer.Close()
		}
		discCfg := spec.DiscoverConfig{
			APIName:          apiName,
			BaseURL:          baseURL,
			SpecURL:          apiCfg.SpecURL,
			SpecFiles:        apiCfg.SpecFiles,
			CacheDir:         c.specCacheDir(),
			ServerVariables:  nil,
			Version:          Version,
			Transport:        transport,
			AllowCrossOrigin: allowCrossOrigin,
			ForceRefresh:     true,
		}
		discovered, discoverErr := spec.Discover(requestContext(cmd), discCfg, c.loaders)
		if discoverErr != nil && !errors.Is(discoverErr, spec.ErrNoSpecFound) {
			return fmt.Errorf("discovering API spec for %q: %w", apiName, discoverErr)
		}
		apiSpec = discovered
	}
	if apiSpec != nil {
		discovery := newConfigureAuthDiscovery(apiSpec, baseURL)
		c.printAPIDiscovery(apiName, baseURL, discovery)
		xcli, _ := spec.ReadXCLIConfig(apiSpec)
		fallbackXCLI := false
		if xcli == nil {
			// No x-cli-config extension — try to derive auth from the spec's
			// declared security schemes.
			xcli = spec.FallbackXCLIConfig(apiSpec)
			fallbackXCLI = true
		}
		if xcli != nil {
			xcli = xcli.Normalize()
			if !replaceProfiles {
				removeExistingXCLIProfiles(xcli, existingAPI)
			}
			if !fallbackXCLI {
				if err := c.promptXCLIConfig(requestContext(cmd), xcli, promptAnswers); err != nil {
					return err
				}
			}
			if len(xcli.Profiles) > 0 {
				c.applyXCLIConfig(apiCfg, xcli.Resolve(apiSpec))
				if fallbackXCLI {
					if err := c.configureFallbackAuth(requestContext(cmd), apiCfg, discovery, promptAnswers); err != nil {
						return err
					}
				}
			}
		}
	}
	if !replaceProfiles {
		if err := preserveExistingProfiles(apiCfg, existingAPI); err != nil {
			return err
		}
	}
	preservedProfiles := preservedProfileNames(existingAPI, replaceProfiles)
	if len(setupExprs) > 0 {
		work := &config.Config{APIs: map[string]*config.APIConfig{apiName: apiCfg}}
		if _, err := c.buildAPIPatchOperations(work, apiName, setupExprs); err != nil {
			return err
		}
	}
	if apiSpec != nil {
		discovery := newConfigureAuthDiscovery(apiSpec, baseURL)
		c.printAuthCoverage("default", discovery, configuredCredentials(apiCfg, "default"))
	}

	// Load, update, and save the config.
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
	c.cfg = cfg
	if len(preservedProfiles) > 0 {
		fmt.Fprintf(c.Stdout, "Preserved existing profile(s): %s (use --replace to recreate from discovered defaults)\n", strings.Join(preservedProfiles, ", "))
	}
	if apiSpec != nil {
		opCount := connectedOperationCount(apiSpec, apiCfg)
		fmt.Fprintf(c.Stdout, "Connected API %q with base URL %s (%d operations discovered — run 'restish %s --help')\n", apiName, baseURL, opCount, apiName)
	} else if noDiscover {
		fmt.Fprintf(c.Stdout, "Connected API %q with base URL %s (discovery skipped — run 'restish api sync %s' later)\n", apiName, baseURL, apiName)
	} else {
		fmt.Fprintf(c.Stdout, "Connected API %q with base URL %s (no spec found — run 'restish api sync %s' after connecting)\n", apiName, baseURL, apiName)
	}
	return nil
}

func applyExplicitSpec(apiCfg *config.APIConfig, raw string) {
	if raw == "" {
		return
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		apiCfg.SpecURL = raw
		return
	}
	apiCfg.SpecFiles = []string{raw}
}

func connectedOperationCount(apiSpec *spec.APISpec, apiCfg *config.APIConfig) int {
	if apiSpec == nil {
		return 0
	}
	ops, err := apiSpec.Operations(apiCfg.BaseURL, apiCfg.OperationBase)
	if err != nil {
		return 0
	}
	return len(ops)
}

func preservedProfileNames(existingAPI *config.APIConfig, replace bool) []string {
	if replace || existingAPI == nil || len(existingAPI.Profiles) == 0 {
		return nil
	}
	names := make([]string, 0, len(existingAPI.Profiles))
	for name := range existingAPI.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func removeExistingXCLIProfiles(xcli *spec.XCLIConfig, existingAPI *config.APIConfig) {
	if xcli == nil || len(xcli.Profiles) == 0 || existingAPI == nil || len(existingAPI.Profiles) == 0 {
		return
	}
	for name := range existingAPI.Profiles {
		delete(xcli.Profiles, name)
	}
}

func preserveExistingProfiles(apiCfg, existingAPI *config.APIConfig) error {
	if apiCfg == nil || existingAPI == nil || len(existingAPI.Profiles) == 0 {
		return nil
	}
	existing, err := cloneAPIConfig(existingAPI)
	if err != nil {
		return err
	}
	if apiCfg.Profiles == nil {
		apiCfg.Profiles = map[string]*config.ProfileConfig{}
	}
	for name, profile := range existing.Profiles {
		apiCfg.Profiles[name] = profile
	}
	return nil
}

func cloneAPIConfig(src *config.APIConfig) (*config.APIConfig, error) {
	if src == nil {
		return nil, nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}
	var dst config.APIConfig
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

func normalizeAPIBaseURL(raw string) (string, error) {
	normalized, err := request.Normalize(raw, "")
	if err != nil {
		return "", err
	}
	return normalized, nil
}

type configurePromptAnswers struct {
	profiles    map[string]map[string]string
	credentials map[string]map[string]map[string]string
}

func parseAPIConfigureSetupExpressions(args []string) (configurePromptAnswers, []setExpression, error) {
	if len(args) == 0 {
		return configurePromptAnswers{}, nil, nil
	}
	answers := configurePromptAnswers{
		profiles:    map[string]map[string]string{},
		credentials: map[string]map[string]map[string]string{},
	}
	var patchArgs []string
	for _, arg := range args {
		key, raw, appendOp, err := parseShorthandAssignment(arg)
		if err != nil {
			return configurePromptAnswers{}, nil, err
		}
		if !strings.HasPrefix(key, "prompt.") {
			patchArgs = append(patchArgs, arg)
			continue
		}
		if appendOp {
			return configurePromptAnswers{}, nil, fmt.Errorf("invalid shorthand %q: prompt answers cannot use append", arg)
		}
		trimmed := strings.TrimPrefix(key, "prompt.")
		if trimmed == "" {
			return configurePromptAnswers{}, nil, fmt.Errorf("invalid prompt answer %q: expected prompt.<name> or prompt.<profile>.<name>", arg)
		}
		value, err := parseConfigCLIValue(raw)
		if err != nil {
			return configurePromptAnswers{}, nil, err
		}
		valueString, ok := value.(string)
		if !ok {
			return configurePromptAnswers{}, nil, fmt.Errorf("invalid prompt answer %q: value must be a string", arg)
		}
		if parseCredentialPromptAnswer(answers, trimmed, valueString) {
			continue
		}
		profileName := "default"
		promptName := trimmed
		if first, rest, ok := strings.Cut(trimmed, "."); ok {
			profileName = first
			promptName = rest
		}
		if profileName == "" || promptName == "" {
			return configurePromptAnswers{}, nil, fmt.Errorf("invalid prompt answer %q: expected prompt.<name> or prompt.<profile>.<name>", arg)
		}
		if answers.profiles[profileName] == nil {
			answers.profiles[profileName] = map[string]string{}
		}
		answers.profiles[profileName][promptName] = valueString
	}
	exprs, err := parseAPISetExpressions(patchArgs)
	if err != nil {
		return configurePromptAnswers{}, nil, err
	}
	return answers, exprs, nil
}

func parseCredentialPromptAnswer(answers configurePromptAnswers, trimmed, value string) bool {
	if rest, ok := strings.CutPrefix(trimmed, "credentials."); ok {
		credentialID, promptName, ok := strings.Cut(rest, ".")
		if !ok || credentialID == "" || promptName == "" {
			return false
		}
		setCredentialPromptAnswer(answers, "default", credentialID, promptName, value)
		return true
	}
	profileName, rest, ok := strings.Cut(trimmed, ".credentials.")
	if !ok || profileName == "" {
		return false
	}
	credentialID, promptName, ok := strings.Cut(rest, ".")
	if !ok || credentialID == "" || promptName == "" {
		return false
	}
	setCredentialPromptAnswer(answers, profileName, credentialID, promptName, value)
	return true
}

func setCredentialPromptAnswer(answers configurePromptAnswers, profileName, credentialID, promptName, value string) {
	if answers.credentials[profileName] == nil {
		answers.credentials[profileName] = map[string]map[string]string{}
	}
	if answers.credentials[profileName][credentialID] == nil {
		answers.credentials[profileName][credentialID] = map[string]string{}
	}
	answers.credentials[profileName][credentialID][promptName] = value
}

func (a configurePromptAnswers) answer(profileName, name string) (string, bool) {
	if len(a.profiles) == 0 {
		return "", false
	}
	if profileAnswers := a.profiles[profileName]; profileAnswers != nil {
		if value, ok := profileAnswers[name]; ok {
			return value, true
		}
	}
	if profileName != "default" {
		if profileAnswers := a.profiles["default"]; profileAnswers != nil {
			if value, ok := profileAnswers[name]; ok {
				return value, true
			}
		}
	}
	return "", false
}

func (a configurePromptAnswers) answerCredential(profileName, credentialID, name string) (string, bool) {
	if profileAnswers := a.credentials[profileName]; profileAnswers != nil {
		if credentialAnswers := profileAnswers[credentialID]; credentialAnswers != nil {
			if value, ok := credentialAnswers[name]; ok {
				return value, true
			}
		}
	}
	if profileName != "default" {
		if profileAnswers := a.credentials["default"]; profileAnswers != nil {
			if credentialAnswers := profileAnswers[credentialID]; credentialAnswers != nil {
				if value, ok := credentialAnswers[name]; ok {
					return value, true
				}
			}
		}
	}
	return a.answer(profileName, name)
}

func (a configurePromptAnswers) hasCredentialAnswer(profileName, credentialID string) bool {
	if profileAnswers := a.credentials[profileName]; profileAnswers != nil && len(profileAnswers[credentialID]) > 0 {
		return true
	}
	if profileName != "default" {
		if profileAnswers := a.credentials["default"]; profileAnswers != nil && len(profileAnswers[credentialID]) > 0 {
			return true
		}
	}
	return false
}

func (c *CLI) promptXCLIConfig(ctx context.Context, xcli *spec.XCLIConfig, answers configurePromptAnswers) error {
	if xcli == nil || len(xcli.Profiles) == 0 {
		return nil
	}
	profileNames := make([]string, 0, len(xcli.Profiles))
	for name := range xcli.Profiles {
		profileNames = append(profileNames, name)
	}
	sort.Strings(profileNames)

	for _, profileName := range profileNames {
		profile := xcli.Profiles[profileName]
		if profile == nil {
			continue
		}
		if err := c.promptXCLIParams(ctx, profileName, profile.Prompt, &profile.Params, &profile.PromptValues, &profile.PromptedParams, answers); err != nil {
			return err
		}
		var credentialIDs []string
		for id := range profile.Credentials {
			credentialIDs = append(credentialIDs, id)
		}
		sort.Strings(credentialIDs)
		for _, id := range credentialIDs {
			credential := profile.Credentials[id]
			if credential == nil {
				continue
			}
			if err := c.promptXCLIParams(ctx, profileName, credential.Prompt, &credential.Params, &credential.PromptValues, &credential.PromptedParams, answers); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *CLI) promptXCLIParams(ctx context.Context, profileName string, prompts map[string]spec.XCLIPromptVar, params *map[string]string, promptValues *map[string]string, promptedParams *map[string]bool, answers configurePromptAnswers) error {
	if len(prompts) == 0 {
		return nil
	}
	if *params == nil {
		*params = map[string]string{}
	}
	if *promptValues == nil {
		*promptValues = map[string]string{}
	}
	if *promptedParams == nil {
		*promptedParams = map[string]bool{}
	}
	promptNames := make([]string, 0, len(prompts))
	for name := range prompts {
		promptNames = append(promptNames, name)
	}
	sort.Strings(promptNames)

	for _, name := range promptNames {
		var value string
		if answer, ok := answers.answer(profileName, name); ok {
			var err error
			value, err = validateXCLIPromptValue(name, strings.TrimSpace(answer), prompts[name])
			if err != nil {
				return fmt.Errorf("x-cli-config prompt %q: %w", name, err)
			}
		} else {
			var err error
			value, err = c.readXCLIPrompt(ctx, profileName, name, prompts[name])
			if err != nil {
				return fmt.Errorf("x-cli-config prompt %q: %w", name, err)
			}
		}
		(*promptValues)[name] = value
		if !prompts[name].Exclude {
			(*params)[name] = value
			(*promptedParams)[name] = true
		}
	}
	return nil
}

func (c *CLI) readXCLIPrompt(ctx context.Context, profileName, name string, prompt spec.XCLIPromptVar) (string, error) {
	label := xcliPromptLabel(profileName, name, prompt)
	for {
		var (
			value string
			err   error
		)
		if xcliPromptLooksSecret(name) {
			value, err = c.Secret(ctx, label)
		} else {
			value, err = c.Prompt(ctx, label)
		}
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value == "" && prompt.Default != nil {
			value = fmt.Sprint(prompt.Default)
		}
		validated, validateErr := validateXCLIPromptValue(name, value, prompt)
		if validateErr != nil {
			fmt.Fprintf(c.Stderr, "%v.\n", validateErr)
			continue
		}
		return validated, nil
	}
}

func validateXCLIPromptValue(name, value string, prompt spec.XCLIPromptVar) (string, error) {
	if value == "" && prompt.Default != nil {
		value = fmt.Sprint(prompt.Default)
	}
	if value == "" {
		return "", fmt.Errorf("%s is required; please enter a non-empty value", name)
	}
	if len(prompt.Enum) > 0 && !xcliPromptValueAllowed(value, prompt.Enum) {
		return "", fmt.Errorf("%s must be one of: %s", name, xcliPromptEnumList(prompt.Enum))
	}
	return value, nil
}

func xcliPromptLabel(profileName, name string, prompt spec.XCLIPromptVar) string {
	label := name
	if prompt.Description != "" {
		label = prompt.Description
	}
	if profileName != "" && profileName != "default" {
		label = profileName + " " + label
	}
	if len(prompt.Enum) > 0 {
		label += " (" + xcliPromptEnumList(prompt.Enum) + ")"
	} else if prompt.Example != "" {
		label += " (example: " + prompt.Example + ")"
	}
	if prompt.Default != nil {
		label += " [" + fmt.Sprint(prompt.Default) + "]"
	}
	return label + ": "
}

func xcliPromptValueAllowed(value string, enum []any) bool {
	for _, candidate := range enum {
		if value == fmt.Sprint(candidate) {
			return true
		}
	}
	return false
}

func xcliPromptEnumList(enum []any) string {
	values := make([]string, 0, len(enum))
	for _, candidate := range enum {
		values = append(values, fmt.Sprint(candidate))
	}
	return strings.Join(values, ", ")
}

func xcliPromptLooksSecret(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "password") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "auth_token") ||
		strings.Contains(lower, "access_key") ||
		strings.Contains(lower, "credential") ||
		strings.Contains(lower, "credentials") ||
		strings.Contains(lower, "passphrase") ||
		strings.Contains(lower, "bearer") ||
		strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "apikey") ||
		strings.Contains(lower, "private_key")
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
				c.warnf("x-cli-config: auth type %q is not permitted; skipping profile %q", xp.Auth.Type, name)
				continue
			}
			prof.Auth = &config.AuthConfig{
				Type:   xp.Auth.Type,
				Params: xp.Auth.Params,
			}
		}
		if len(xp.Credentials) > 0 {
			prof.Credentials = make(map[string]*config.CredentialConfig, len(xp.Credentials))
			for credentialID, xc := range xp.Credentials {
				if xc == nil {
					continue
				}
				credential := &config.CredentialConfig{
					AuthRef:   xc.AuthRef,
					Satisfies: append([]string(nil), xc.Satisfies...),
				}
				if xc.Auth != nil {
					if xc.Auth.Type == "external-tool" {
						c.warnf("x-cli-config: auth type %q is not permitted; skipping credential %q in profile %q", xc.Auth.Type, credentialID, name)
						continue
					}
					credential.Auth = &config.AuthConfig{
						Type:   xc.Auth.Type,
						Params: xc.Auth.Params,
					}
				}
				prof.Credentials[credentialID] = credential
			}
			if len(prof.Credentials) == 0 {
				prof.Credentials = nil
			}
		}
		apiCfg.Profiles[name] = prof
	}
}

func (c *CLI) discoveryTransport(ctx context.Context, apiCfg *config.APIConfig, profileName string) (http.RoundTripper, interface{ Close() error }, error) {
	gf := globalFlagsFromContext(ctx)
	if gf.Insecure {
		c.warnf("TLS certificate verification is disabled (--rsh-insecure); connections are not secure")
	}
	tlsMinVersion, err := request.TLSVersionFromString(gf.TLSMinVersion)
	if err != nil {
		return nil, nil, err
	}
	tlsSignerParams, err := parseKVStrings(gf.TLSSignerParams)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid tls signer param: %w", err)
	}
	opts := request.Options{
		Transport:       c.baseHTTPTransport(),
		Insecure:        gf.Insecure,
		ClientCertPath:  gf.ClientCert,
		ClientKeyPath:   gf.ClientKey,
		TLSSignerName:   gf.TLSSigner,
		TLSSignerParams: tlsSignerParams,
		CACertPath:      gf.CACert,
		TLSMinVersion:   tlsMinVersion,
		UserAgent:       "restish/" + Version,
		Logger:          diagnosticPrefixWriter(c.Stderr),
	}
	if apiCfg != nil {
		if profileName == "" {
			profileName = "default"
		}
		if prof := profileForName(apiCfg, profileName); prof != nil {
			if opts.TLSSignerName == "" {
				opts.TLSSignerName = prof.TLSSigner
			}
			opts.TLSSignerParams = mergeTLSSignerParams(opts.TLSSignerParams, prof.TLSSignerParams)
			if opts.CACertPath == "" {
				opts.CACertPath = prof.CACertPath
			}
			if opts.ClientCertPath == "" {
				opts.ClientCertPath = prof.ClientCertPath
			}
			if opts.ClientKeyPath == "" {
				opts.ClientKeyPath = prof.ClientKeyPath
			}
		}
	}
	opts, err = c.resolveTLSSigner(opts)
	if err != nil {
		return nil, nil, err
	}
	transport := request.BuildTransport(opts)
	closer, _ := transport.(interface{ Close() error })
	return transport, closer, nil
}

// runAPIInspect prints the config for a named API as indented JSON,
// with secret auth params replaced by "***".
func (c *CLI) runAPIInspect(cmd *cobra.Command, args []string) error {
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

// runConfigEdit opens the restish config file in $VISUAL or $EDITOR.
func (c *CLI) runConfigEdit(cmd *cobra.Command, args []string) error {
	cfgPath := c.configFilePath()
	oldCfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	editorCmd, err := c.editorCommand(cfgPath)
	if err != nil {
		return err
	}
	editorCmd.Stdin = c.Stdin
	editorCmd.Stdout = c.Stdout
	editorCmd.Stderr = c.Stderr
	if err := editorCmd.Run(); err != nil {
		return err
	}
	newCfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	for _, apiName := range apiNamesWithSpecCacheRelevantChanges(oldCfg, newCfg) {
		if err := spec.InvalidateCache(c.specCacheDir(), apiName); err != nil {
			return fmt.Errorf("config edit: invalidate spec cache for %q: %w", apiName, err)
		}
	}
	c.cfg = newCfg
	return nil
}

func apiNamesWithSpecCacheRelevantChanges(oldCfg, newCfg *config.Config) []string {
	namesSeen := map[string]struct{}{}
	if oldCfg != nil {
		for name := range oldCfg.APIs {
			namesSeen[name] = struct{}{}
		}
	}
	if newCfg != nil {
		for name := range newCfg.APIs {
			namesSeen[name] = struct{}{}
		}
	}
	var changed []string
	for name := range namesSeen {
		var oldAPI, newAPI *config.APIConfig
		if oldCfg != nil {
			oldAPI = oldCfg.APIs[name]
		}
		if newCfg != nil {
			newAPI = newCfg.APIs[name]
		}
		if apiSpecCacheRelevantFieldsChanged(oldAPI, newAPI) {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	return changed
}

// runAPISet updates a single config field for a named API using a dot-path key.
// Supported keys: base_url, spec_url, allow_cross_origin_spec, operation_base,
// server_variables.<name>,
// pagination.items_path, pagination.next_path,
// profiles.<name>.base_url, profiles.<name>.auth.type,
// profiles.<name>.auth.params.<param>, profiles.<name>.tls_signer,
// profiles.<name>.server_variables.<name>,
// profiles.<name>.credentials.<id>.auth_ref,
// profiles.<name>.credentials.<id>.satisfies,
// profiles.<name>.credentials.<id>.auth.type,
// profiles.<name>.credentials.<id>.auth.params.<param>.
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
	if apiSpecCacheRelevantFieldsChanged(c.cfg.APIs[apiName], work.APIs[apiName]) {
		if err := spec.InvalidateCache(c.specCacheDir(), apiName); err != nil {
			return fmt.Errorf("api set: invalidate spec cache: %w", err)
		}
	}
	c.cfg = work
	return nil
}

func apiSpecCacheRelevantFieldsChanged(oldAPI, newAPI *config.APIConfig) bool {
	if oldAPI == nil || newAPI == nil {
		return oldAPI != newAPI
	}
	return oldAPI.BaseURL != newAPI.BaseURL ||
		oldAPI.SpecURL != newAPI.SpecURL ||
		oldAPI.OperationBase != newAPI.OperationBase ||
		!reflect.DeepEqual(oldAPI.SpecFiles, newAPI.SpecFiles) ||
		!reflect.DeepEqual(oldAPI.ServerVariables, newAPI.ServerVariables) ||
		!profileServerVariablesEqual(oldAPI.Profiles, newAPI.Profiles)
}

func profileServerVariablesEqual(a, b map[string]*config.ProfileConfig) bool {
	names := map[string]struct{}{}
	for name := range a {
		names[name] = struct{}{}
	}
	for name := range b {
		names[name] = struct{}{}
	}
	for name := range names {
		var av, bv map[string]string
		if a[name] != nil {
			av = a[name].ServerVariables
		}
		if b[name] != nil {
			bv = b[name].ServerVariables
		}
		if !reflect.DeepEqual(av, bv) {
			return false
		}
	}
	return true
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
	apiKeyCommandLayout
	apiKeyServerVariable
	apiKeyPaginationItemsPath
	apiKeyPaginationNextPath
	apiKeyProfileBaseURL
	apiKeyProfileHeaders
	apiKeyProfileQuery
	apiKeyProfileTLSSigner
	apiKeyProfileServerVariable
	apiKeyProfileAuthRef
	apiKeyProfileAuthType
	apiKeyProfileAuthParam
	apiKeyProfileCredentialAuthRef
	apiKeyProfileCredentialAuthType
	apiKeyProfileCredentialAuthParam
	apiKeyProfileCredentialSatisfies
)

type resolvedAPIConfigKey struct {
	kind         apiConfigKeyKind
	jsonPath     []string
	profileName  string
	credentialID string
	paramName    string
	varName      string
	descriptor   apiSetFieldDescriptor
}

type apiSetFieldDescriptor struct {
	Kind    apiConfigKeyKind
	Pattern string
	Append  bool
}

var apiSetFieldDescriptors = []apiSetFieldDescriptor{
	{Kind: apiKeyBaseURL, Pattern: "base_url"},
	{Kind: apiKeySpecURL, Pattern: "spec_url"},
	{Kind: apiKeyAllowCrossOriginSpec, Pattern: "allow_cross_origin_spec"},
	{Kind: apiKeyOperationBase, Pattern: "operation_base"},
	{Kind: apiKeyCommandLayout, Pattern: "command_layout"},
	{Kind: apiKeyServerVariable, Pattern: "server_variables.<var>"},
	{Kind: apiKeyPaginationItemsPath, Pattern: "pagination.items_path"},
	{Kind: apiKeyPaginationNextPath, Pattern: "pagination.next_path"},
	{Kind: apiKeyProfileBaseURL, Pattern: "profiles.<name>.base_url"},
	{Kind: apiKeyProfileHeaders, Pattern: "profiles.<name>.headers", Append: true},
	{Kind: apiKeyProfileQuery, Pattern: "profiles.<name>.query", Append: true},
	{Kind: apiKeyProfileTLSSigner, Pattern: "profiles.<name>.tls_signer"},
	{Kind: apiKeyProfileServerVariable, Pattern: "profiles.<name>.server_variables.<var>"},
	{Kind: apiKeyProfileAuthRef, Pattern: "profiles.<name>.auth_ref"},
	{Kind: apiKeyProfileAuthType, Pattern: "profiles.<name>.auth.type"},
	{Kind: apiKeyProfileAuthParam, Pattern: "profiles.<name>.auth.params.<param>"},
	{Kind: apiKeyProfileCredentialAuthRef, Pattern: "profiles.<name>.credentials.<id>.auth_ref"},
	{Kind: apiKeyProfileCredentialSatisfies, Pattern: "profiles.<name>.credentials.<id>.satisfies"},
	{Kind: apiKeyProfileCredentialAuthType, Pattern: "profiles.<name>.credentials.<id>.auth.type"},
	{Kind: apiKeyProfileCredentialAuthParam, Pattern: "profiles.<name>.credentials.<id>.auth.params.<param>"},
}

var apiSetFieldDescriptorByKind = func() map[apiConfigKeyKind]apiSetFieldDescriptor {
	out := make(map[apiConfigKeyKind]apiSetFieldDescriptor, len(apiSetFieldDescriptors))
	for _, desc := range apiSetFieldDescriptors {
		out[desc.Kind] = desc
	}
	return out
}()

func resolvedAPIKey(kind apiConfigKeyKind, jsonPath []string) resolvedAPIConfigKey {
	return resolvedAPIConfigKey{kind: kind, jsonPath: jsonPath, descriptor: apiSetFieldDescriptorByKind[kind]}
}

func apiSetSupportedFields() string {
	patterns := make([]string, 0, len(apiSetFieldDescriptors))
	for _, desc := range apiSetFieldDescriptors {
		patterns = append(patterns, desc.Pattern)
	}
	return strings.Join(patterns, ", ")
}

func resolveAPIConfigKey(apiName, key string) (resolvedAPIConfigKey, error) {
	parts := strings.SplitN(key, ".", 3)
	basePath := []string{"apis"}
	if apiName != "" {
		basePath = append(basePath, apiName)
	}

	switch parts[0] {
	case "base_url":
		return resolvedAPIKey(apiKeyBaseURL, append(basePath, "base_url")), nil
	case "spec_url":
		return resolvedAPIKey(apiKeySpecURL, append(basePath, "spec_url")), nil
	case "allow_cross_origin_spec":
		return resolvedAPIKey(apiKeyAllowCrossOriginSpec, append(basePath, "allow_cross_origin_spec")), nil
	case "operation_base":
		return resolvedAPIKey(apiKeyOperationBase, append(basePath, "operation_base")), nil
	case "command_layout":
		return resolvedAPIKey(apiKeyCommandLayout, append(basePath, "command_layout")), nil
	case "server_variables":
		if len(parts) < 2 || parts[1] == "" {
			return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected server_variables.<name>", key)
		}
		return resolvedAPIConfigKey{
			kind:     apiKeyServerVariable,
			jsonPath: append(basePath, "server_variables", parts[1]),
			varName:  parts[1],
		}, nil
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
		case "server_variables":
			if len(subParts) < 2 || subParts[1] == "" {
				return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected profiles.<name>.server_variables.<var>", key)
			}
			return resolvedAPIConfigKey{
				kind:        apiKeyProfileServerVariable,
				jsonPath:    append(basePath, "profiles", profileName, "server_variables", subParts[1]),
				profileName: profileName,
				varName:     subParts[1],
			}, nil
		case "auth_ref":
			return resolvedAPIConfigKey{
				kind:        apiKeyProfileAuthRef,
				jsonPath:    append(basePath, "profiles", profileName, "auth_ref"),
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
		case "credentials":
			if len(subParts) < 3 {
				return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected profiles.<name>.credentials.<id>.<field>", key)
			}
			credentialID := subParts[1]
			credentialRest := subParts[2]
			if credentialID == "" || credentialRest == "" {
				return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected profiles.<name>.credentials.<id>.<field>", key)
			}
			credentialParts := strings.SplitN(credentialRest, ".", 3)
			switch credentialParts[0] {
			case "auth_ref":
				return resolvedAPIConfigKey{
					kind:         apiKeyProfileCredentialAuthRef,
					jsonPath:     append(basePath, "profiles", profileName, "credentials", credentialID, "auth_ref"),
					profileName:  profileName,
					credentialID: credentialID,
				}, nil
			case "satisfies":
				return resolvedAPIConfigKey{
					kind:         apiKeyProfileCredentialSatisfies,
					jsonPath:     append(basePath, "profiles", profileName, "credentials", credentialID, "satisfies"),
					profileName:  profileName,
					credentialID: credentialID,
				}, nil
			case "auth":
				if len(credentialParts) < 2 {
					return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected profiles.<name>.credentials.<id>.auth.<field>", key)
				}
				switch credentialParts[1] {
				case "type":
					return resolvedAPIConfigKey{
						kind:         apiKeyProfileCredentialAuthType,
						jsonPath:     append(basePath, "profiles", profileName, "credentials", credentialID, "auth", "type"),
						profileName:  profileName,
						credentialID: credentialID,
					}, nil
				case "params":
					if len(credentialParts) < 3 {
						return resolvedAPIConfigKey{}, fmt.Errorf("invalid key %q: expected profiles.<name>.credentials.<id>.auth.params.<param>", key)
					}
					return resolvedAPIConfigKey{
						kind:         apiKeyProfileCredentialAuthParam,
						jsonPath:     append(basePath, "profiles", profileName, "credentials", credentialID, "auth", "params", credentialParts[2]),
						profileName:  profileName,
						credentialID: credentialID,
						paramName:    credentialParts[2],
					}, nil
				default:
					return resolvedAPIConfigKey{}, fmt.Errorf("unsupported credential auth field %q; supported: type, params.<param>", credentialParts[1])
				}
			default:
				return resolvedAPIConfigKey{}, fmt.Errorf("unsupported credential field %q; supported: auth_ref, satisfies, auth.type, auth.params.<param>", credentialParts[0])
			}
		default:
			return resolvedAPIConfigKey{}, fmt.Errorf("unsupported profile field %q; supported: base_url, headers, query, tls_signer, server_variables.<var>, auth_ref, auth.type, auth.params.<param>, credentials.<id>.auth_ref, credentials.<id>.satisfies, credentials.<id>.auth.type, credentials.<id>.auth.params.<param>", parts[2])
		}
	default:
		return resolvedAPIConfigKey{}, fmt.Errorf("unsupported field %q; supported: %s", key, apiSetSupportedFields())
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
		resolved.descriptor = apiSetFieldDescriptorByKind[resolved.kind]

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

func ensureCredential(apiCfg *config.APIConfig, profileName, credentialID string) *config.CredentialConfig {
	prof := ensureProfile(apiCfg, profileName)
	if prof.Credentials == nil {
		prof.Credentials = make(map[string]*config.CredentialConfig)
	}
	if prof.Credentials[credentialID] == nil {
		prof.Credentials[credentialID] = &config.CredentialConfig{}
	}
	return prof.Credentials[credentialID]
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
	case apiKeyCommandLayout:
		apiCfg.CommandLayout = ""
	case apiKeyServerVariable:
		if apiCfg.ServerVariables != nil {
			delete(apiCfg.ServerVariables, resolved.varName)
		}
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
	case apiKeyProfileServerVariable:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.ServerVariables != nil {
			delete(p.ServerVariables, resolved.varName)
		}
	case apiKeyProfileAuthType:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Auth != nil {
			p.Auth.Type = ""
		}
	case apiKeyProfileAuthParam:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Auth != nil && p.Auth.Params != nil {
			delete(p.Auth.Params, resolved.paramName)
		}
	case apiKeyProfileCredentialAuthRef:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Credentials != nil && p.Credentials[resolved.credentialID] != nil {
			p.Credentials[resolved.credentialID].AuthRef = ""
		}
	case apiKeyProfileCredentialAuthType:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Credentials != nil {
			if credential := p.Credentials[resolved.credentialID]; credential != nil && credential.Auth != nil {
				credential.Auth.Type = ""
			}
		}
	case apiKeyProfileCredentialAuthParam:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Credentials != nil {
			if credential := p.Credentials[resolved.credentialID]; credential != nil && credential.Auth != nil && credential.Auth.Params != nil {
				delete(credential.Auth.Params, resolved.paramName)
			}
		}
	case apiKeyProfileCredentialSatisfies:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Credentials != nil && p.Credentials[resolved.credentialID] != nil {
			p.Credentials[resolved.credentialID].Satisfies = nil
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
		if !resolved.descriptor.Append {
			return fmt.Errorf("append is not supported for %s", resolved.descriptor.Pattern)
		}
		prof.Headers = append(prof.Headers, v)
	case apiKeyProfileQuery:
		if !resolved.descriptor.Append {
			return fmt.Errorf("append is not supported for %s", resolved.descriptor.Pattern)
		}
		prof.Query = append(prof.Query, v)
	default:
		return fmt.Errorf("append is only supported for %s[] and %s[]", apiSetFieldDescriptorByKind[apiKeyProfileHeaders].Pattern, apiSetFieldDescriptorByKind[apiKeyProfileQuery].Pattern)
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
		if err := config.ValidateOperationBase(v); err != nil {
			return fmt.Errorf("operation_base %w", err)
		}
		apiCfg.OperationBase = v
	case apiKeyCommandLayout:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("command_layout must be a string")
		}
		if err := config.ValidateCommandLayout(v); err != nil {
			return fmt.Errorf("command_layout %w", err)
		}
		apiCfg.CommandLayout = v
	case apiKeyServerVariable:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("server_variables.%s must be a string", resolved.varName)
		}
		if apiCfg.ServerVariables == nil {
			apiCfg.ServerVariables = map[string]string{}
		}
		apiCfg.ServerVariables[resolved.varName] = v
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
	case apiKeyProfileServerVariable:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.server_variables.%s must be a string", resolved.profileName, resolved.varName)
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		if prof.ServerVariables == nil {
			prof.ServerVariables = map[string]string{}
		}
		prof.ServerVariables[resolved.varName] = v
	case apiKeyProfileAuthRef:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.auth_ref must be a string", resolved.profileName)
		}
		if v != "" && (c.cfg == nil || c.cfg.AuthProfiles == nil || c.cfg.AuthProfiles[v] == nil) {
			return fmt.Errorf("profiles.%s.auth_ref: unknown auth profile %q", resolved.profileName, v)
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		if prof.Auth != nil && v != "" {
			return fmt.Errorf("profiles.%s.auth_ref cannot be set while auth is configured", resolved.profileName)
		}
		prof.AuthRef = v
	case apiKeyProfileAuthType:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.auth.type must be a string", resolved.profileName)
		}
		prof := ensureProfile(apiCfg, resolved.profileName)
		if prof.AuthRef != "" {
			return fmt.Errorf("profiles.%s.auth cannot be set while auth_ref is configured", resolved.profileName)
		}
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
		if prof.AuthRef != "" {
			return fmt.Errorf("profiles.%s.auth cannot be set while auth_ref is configured", resolved.profileName)
		}
		if prof.Auth == nil {
			prof.Auth = &config.AuthConfig{}
		}
		if prof.Auth.Params == nil {
			prof.Auth.Params = map[string]string{}
		}
		prof.Auth.Params[resolved.paramName] = v
	case apiKeyProfileCredentialAuthRef:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.credentials.%s.auth_ref must be a string", resolved.profileName, resolved.credentialID)
		}
		if v != "" && (c.cfg == nil || c.cfg.AuthProfiles == nil || c.cfg.AuthProfiles[v] == nil) {
			return fmt.Errorf("profiles.%s.credentials.%s.auth_ref: unknown auth profile %q", resolved.profileName, resolved.credentialID, v)
		}
		credential := ensureCredential(apiCfg, resolved.profileName, resolved.credentialID)
		if credential.Auth != nil && v != "" {
			return fmt.Errorf("profiles.%s.credentials.%s.auth_ref cannot be set while auth is configured", resolved.profileName, resolved.credentialID)
		}
		credential.AuthRef = v
	case apiKeyProfileCredentialAuthType:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.credentials.%s.auth.type must be a string", resolved.profileName, resolved.credentialID)
		}
		credential := ensureCredential(apiCfg, resolved.profileName, resolved.credentialID)
		if credential.AuthRef != "" {
			return fmt.Errorf("profiles.%s.credentials.%s.auth cannot be set while auth_ref is configured", resolved.profileName, resolved.credentialID)
		}
		if credential.Auth == nil {
			credential.Auth = &config.AuthConfig{}
		}
		candidate := &config.AuthConfig{Type: v, Params: map[string]string{}}
		if _, err := c.authHandlerFor(candidate, authHandlerOptions{}); err != nil {
			return fmt.Errorf("invalid credentials.%s.auth.type %q: %w", resolved.credentialID, v, err)
		}
		credential.Auth.Type = v
	case apiKeyProfileCredentialAuthParam:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("profiles.%s.credentials.%s.auth.params.%s must be a string", resolved.profileName, resolved.credentialID, resolved.paramName)
		}
		credential := ensureCredential(apiCfg, resolved.profileName, resolved.credentialID)
		if credential.AuthRef != "" {
			return fmt.Errorf("profiles.%s.credentials.%s.auth cannot be set while auth_ref is configured", resolved.profileName, resolved.credentialID)
		}
		if credential.Auth == nil {
			credential.Auth = &config.AuthConfig{}
		}
		if credential.Auth.Params == nil {
			credential.Auth.Params = map[string]string{}
		}
		credential.Auth.Params[resolved.paramName] = v
	case apiKeyProfileCredentialSatisfies:
		arr, err := anyToStringSlice(value)
		if err != nil {
			return fmt.Errorf("profiles.%s.credentials.%s.satisfies must be a string array", resolved.profileName, resolved.credentialID)
		}
		credential := ensureCredential(apiCfg, resolved.profileName, resolved.credentialID)
		credential.Satisfies = arr
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
	case apiKeyCommandLayout:
		return apiCfg.CommandLayout, nil
	case apiKeyServerVariable:
		if apiCfg.ServerVariables != nil {
			return apiCfg.ServerVariables[resolved.varName], nil
		}
		return "", nil
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
	case apiKeyProfileServerVariable:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.ServerVariables != nil {
			return p.ServerVariables[resolved.varName], nil
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
	case apiKeyProfileCredentialAuthRef:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Credentials != nil && p.Credentials[resolved.credentialID] != nil {
			return p.Credentials[resolved.credentialID].AuthRef, nil
		}
		return "", nil
	case apiKeyProfileCredentialAuthType:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Credentials != nil {
			if credential := p.Credentials[resolved.credentialID]; credential != nil && credential.Auth != nil {
				return credential.Auth.Type, nil
			}
		}
		return "", nil
	case apiKeyProfileCredentialAuthParam:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Credentials != nil {
			if credential := p.Credentials[resolved.credentialID]; credential != nil && credential.Auth != nil && credential.Auth.Params != nil {
				return credential.Auth.Params[resolved.paramName], nil
			}
		}
		return "", nil
	case apiKeyProfileCredentialSatisfies:
		if p := apiCfg.Profiles[resolved.profileName]; p != nil && p.Credentials != nil && p.Credentials[resolved.credentialID] != nil {
			return p.Credentials[resolved.credentialID].Satisfies, nil
		}
		return []string{}, nil
	default:
		return nil, fmt.Errorf("unsupported field")
	}
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

// runAPIRemove removes a configured API and saves the updated config.
func (c *CLI) runAPIRemove(cmd *cobra.Command, args []string) error {
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
	fmt.Fprintf(c.Stdout, "Removed API %q\n", apiName)
	return nil
}
