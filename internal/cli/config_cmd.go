package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/config"
	"github.com/rest-sh/restish/v2/internal/secrets"
	"github.com/spf13/cobra"
)

func (c *CLI) addConfigCommand(root *cobra.Command) {
	configCmd := &cobra.Command{
		Use:     "config",
		Short:   "Manage local Restish configuration",
		Long:    configLong,
		GroupID: rootGroupConfig,
		Example: fmt.Sprintf(`  %s config show
  %s config path
  %s config set 'cache.max_size: 500MB'`, c.commandNameOrDefault(), c.commandNameOrDefault(), c.commandNameOrDefault()),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return unknownNamedSubcommandError(cmd, "config", args[0], "")
			}
			return cmd.Help()
		},
	}
	configCmd.AddCommand(&cobra.Command{
		Use:     "path",
		Short:   "Print the active config file path",
		Long:    configPathLong,
		Example: fmt.Sprintf("  %s config path", c.commandNameOrDefault()),
		Args:    usageNoArgs,
		RunE:    c.runConfigPath,
	})
	configCmd.AddCommand(&cobra.Command{
		Use:     "trust",
		Short:   "Trust the project config discovered from this directory",
		Long:    configTrustLong,
		Example: fmt.Sprintf("  %s config trust", c.commandNameOrDefault()),
		Args:    usageNoArgs,
		RunE:    c.runConfigTrust,
	})
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Print the active config summary or redacted JSON",
		Long:  configShowLong,
		Example: fmt.Sprintf(`  %s config show
  %s config show -o json`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: usageNoArgs,
		RunE: c.runConfigShow,
	}
	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(&cobra.Command{
		Use:     "edit",
		Short:   "Open the restish config file in $VISUAL or $EDITOR",
		Long:    configEditLong,
		Example: fmt.Sprintf("  %s config edit", c.commandNameOrDefault()),
		Args:    usageNoArgs,
		RunE:    c.runConfigEdit,
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "set <patch> [patch...]",
		Short: "Patch config using shorthand syntax",
		Long:  configSetLong,
		Example: fmt.Sprintf(`  %s config set 'cache.max_size: 500MB'
  %s config set 'theme.key: #afd787'`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: usageMinimumNArgs(1),
		RunE: c.runConfigSet,
	})
	themeCmd := &cobra.Command{
		Use:   "theme",
		Short: "Manage terminal output highlighting theme",
		Long:  configThemeLong,
		Example: fmt.Sprintf(`  %s config theme list
  %s config theme set one-dark-pro
  %s config theme reset`, c.commandNameOrDefault(), c.commandNameOrDefault(), c.commandNameOrDefault()),
	}
	themeCmd.AddCommand(c.newThemeListCommand())
	themeCmd.AddCommand(c.newThemeSetCommand())
	themeCmd.AddCommand(c.newThemeResetCommand())
	configCmd.AddCommand(themeCmd)
	root.AddCommand(configCmd)
}

func (c *CLI) runConfigPath(cmd *cobra.Command, args []string) error {
	if err := rejectResponseTransformFlags(cmd); err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, c.configFilePath())
	return nil
}

func (c *CLI) runConfigShow(cmd *cobra.Command, args []string) error {
	if jsonOut, err := commandJSONOutputRequested(cmd); err != nil {
		return err
	} else if jsonOut {
		view, err := redactedConfigView(c.cfg)
		if err != nil {
			return err
		}
		return c.writePrettyJSON(view)
	}

	cfg := c.cfg
	if cfg == nil {
		cfg = &config.Config{}
	}
	style := humanTextStyleFor(c.Stdout)
	fmt.Fprintf(c.Stdout, "%s %s\n", style.key("Config file:"), c.configSourceSummary())
	if c.projectConfig != nil && c.projectConfig.Trusted {
		names := sortedProjectAPINames(c.projectConfig.APIs)
		if len(names) > 0 {
			fmt.Fprintf(c.Stdout, "%s %s (%s)\n", style.key("Project config:"), c.projectConfig.Path, strings.Join(names, ", "))
		} else {
			fmt.Fprintf(c.Stdout, "%s %s\n", style.key("Project config:"), c.projectConfig.Path)
		}
	}
	fmt.Fprintf(c.Stdout, "%s %s\n", style.key("Cache max size:"), configShowCacheMaxSize(cfg))
	fmt.Fprintf(c.Stdout, "%s %s\n", style.key("Theme:"), configShowTheme(cfg))
	fmt.Fprintf(c.Stdout, "%s %s\n", style.key("Auth profiles:"), configShowNamedMapSummary(cfg.AuthProfiles))
	fmt.Fprintf(c.Stdout, "%s %s\n", style.key("Configured plugins:"), configShowRawMessageMapSummary(cfg.Plugins))
	c.printConfigShowAPIs(style, cfg)
	return nil
}

func (c *CLI) runConfigTrust(cmd *cobra.Command, args []string) error {
	if err := rejectResponseTransformFlags(cmd); err != nil {
		return err
	}
	project := c.projectConfig
	if project == nil {
		discovered, err := discoverProjectConfig()
		if err != nil {
			return err
		}
		project = discovered
	}
	if project == nil {
		return fmt.Errorf("no %s found in this directory or its parents", projectConfigFileName)
	}
	summary, err := c.projectConfigTrustSummary(project)
	if err != nil {
		return err
	}
	if err := c.trustProjectConfig(project); err != nil {
		return err
	}
	project.Trusted = true
	style := humanTextStyleFor(c.Stdout)
	fmt.Fprintf(c.Stdout, "%s project config: %s%s\n", style.ok("Trusted"), project.Path, summary)
	return nil
}

func (c *CLI) printConfigShowAPIs(style humanTextStyle, cfg *config.Config) {
	if cfg == nil || len(cfg.APIs) == 0 {
		fmt.Fprintf(c.Stdout, "%s %s (%s)\n", style.key("APIs:"), style.warn("none"), style.hint("run \"restish api connect <name> <url>\""))
		return
	}
	names := sortedConfigKeys(cfg.APIs)
	fmt.Fprintf(c.Stdout, "%s %d\n", style.key("APIs:"), len(names))
	for _, name := range names {
		apiCfg := cfg.APIs[name]
		fmt.Fprintf(c.Stdout, "  %s\n", style.key(name))
		fmt.Fprintf(c.Stdout, "    %s %s\n", style.key("Base URL:"), configShowValue(configShowBaseURL(apiCfg)))
		fmt.Fprintf(c.Stdout, "    %s %s\n", style.key("Spec:"), configShowSpecSource(apiCfg))
		fmt.Fprintf(c.Stdout, "    %s %s\n", style.key("Profiles:"), configShowProfiles(apiCfg))
		fmt.Fprintf(c.Stdout, "    %s %s\n", style.key("Auth:"), c.configShowAPIAuthSummary(name, apiCfg))
	}
}

func configShowCacheMaxSize(cfg *config.Config) string {
	if cfg == nil || cfg.Cache.MaxSize == "" {
		return "default"
	}
	return cfg.Cache.MaxSize
}

func configShowTheme(cfg *config.Config) string {
	if cfg == nil {
		return "built-in"
	}
	if cfg.ThemeSource != "" {
		return cfg.ThemeSource
	}
	if len(cfg.Theme) > 0 {
		return "custom inline"
	}
	return "built-in"
}

func configShowNamedMapSummary[V any](items map[string]V) string {
	names := sortedConfigKeys(items)
	if len(names) == 0 {
		return "none"
	}
	return fmt.Sprintf("%d (%s)", len(names), strings.Join(names, ", "))
}

func configShowRawMessageMapSummary(items map[string]json.RawMessage) string {
	names := sortedConfigKeys(items)
	if len(names) == 0 {
		return "none"
	}
	return fmt.Sprintf("%d (%s)", len(names), strings.Join(names, ", "))
}

func sortedConfigKeys[V any](items map[string]V) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func configShowValue(value string) string {
	if value == "" {
		return "not configured"
	}
	return value
}

func configShowBaseURL(apiCfg *config.APIConfig) string {
	if apiCfg == nil {
		return ""
	}
	return apiCfg.BaseURL
}

func configShowSpecSource(apiCfg *config.APIConfig) string {
	if apiCfg == nil {
		return "auto-discover"
	}
	if len(apiCfg.SpecFiles) > 0 {
		return strings.Join(apiCfg.SpecFiles, ", ")
	}
	if apiCfg.SpecURL != "" {
		return apiCfg.SpecURL
	}
	return "auto-discover"
}

func configShowProfiles(apiCfg *config.APIConfig) string {
	if apiCfg == nil || len(apiCfg.Profiles) == 0 {
		return "default (implicit)"
	}
	return strings.Join(sortedConfigKeys(apiCfg.Profiles), ", ")
}

func (c *CLI) configShowAPIAuthSummary(apiName string, apiCfg *config.APIConfig) string {
	profileNames := []string{"default"}
	if apiCfg != nil && len(apiCfg.Profiles) > 0 {
		profileNames = sortedConfigKeys(apiCfg.Profiles)
	}
	statuses := map[string]bool{}
	sources := map[string]bool{}
	for _, profileName := range profileNames {
		auth := c.doctorAuthForProfile(apiName, profileName, profileForName(apiCfg, profileName))
		statuses[auth.Status] = true
		for _, source := range auth.Sources {
			sources[source] = true
		}
	}
	if statuses["configured-but-unresolved"] {
		return configShowAuthWithSources("configured but unresolved", sources)
	}
	if statuses["configured"] {
		if len(statuses) > 1 {
			return configShowAuthWithSources("partially configured", sources)
		}
		return configShowAuthWithSources("configured", sources)
	}
	return "none"
}

func configShowAuthWithSources(status string, sources map[string]bool) string {
	names := make([]string, 0, len(sources))
	for source := range sources {
		names = append(names, configShowAuthSourceLabel(source))
	}
	sort.Strings(names)
	if len(names) == 0 {
		return status
	}
	return fmt.Sprintf("%s (%s)", status, strings.Join(names, ", "))
}

func configShowAuthSourceLabel(source string) string {
	switch source {
	case "profile_auth":
		return "profile auth"
	case "credentials":
		return "operation credentials"
	default:
		return source
	}
}

func (c *CLI) runConfigSet(cmd *cobra.Command, args []string) error {
	if err := validateConfigPatchArgs("config set", args); err != nil {
		return err
	}
	if err := c.saveConfigShorthand("config set", nil, args); err != nil {
		return err
	}
	c.printConfigWrittenPath()
	return nil
}

func redactedConfigView(cfg *config.Config) (map[string]any, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var view map[string]any
	if err := json.Unmarshal(raw, &view); err != nil {
		return nil, err
	}
	redactSensitiveConfigValue(view)
	return view, nil
}

func redactSensitiveConfigValue(v any) {
	switch data := v.(type) {
	case map[string]any:
		if _, hasType := data["type"].(string); hasType {
			if params, ok := data["params"].(map[string]any); ok {
				for key := range params {
					if isSensitiveConfigKey(key) || key == "value" {
						params[key] = "***"
					}
				}
			}
		}
		for key, value := range data {
			if redacted, ok := redactSensitiveConfigStringList(key, value); ok {
				data[key] = redacted
				continue
			}
			if isSensitiveConfigKey(key) {
				data[key] = "***"
				continue
			}
			redactSensitiveConfigValue(value)
		}
	case []any:
		for _, item := range data {
			redactSensitiveConfigValue(item)
		}
	}
}

func redactSensitiveConfigStringList(key string, value any) ([]any, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	switch strings.ToLower(key) {
	case "headers":
		redacted := make([]any, len(items))
		for i, item := range items {
			raw, ok := item.(string)
			if !ok {
				redacted[i] = item
				continue
			}
			redacted[i] = redactPersistentHeader(raw)
		}
		return redacted, true
	case "query":
		redacted := make([]any, len(items))
		for i, item := range items {
			raw, ok := item.(string)
			if !ok {
				redacted[i] = item
				continue
			}
			redacted[i] = redactPersistentQuery(raw)
		}
		return redacted, true
	default:
		return nil, false
	}
}

func redactPersistentHeader(raw string) string {
	name, _, ok := strings.Cut(raw, ":")
	if !ok || !secrets.IsHeaderName(strings.TrimSpace(name)) {
		return raw
	}
	return strings.TrimSpace(name) + ": ***"
}

func redactPersistentQuery(raw string) string {
	name, _, ok := strings.Cut(raw, "=")
	if !ok || !secrets.IsQueryParamName(strings.TrimSpace(name)) {
		return raw
	}
	return strings.TrimSpace(name) + "=***"
}

func profileCredentialSettingSources(prof *config.ProfileConfig) []string {
	if prof == nil {
		return nil
	}
	var sources []string
	if persistentHeadersContainCredentials(prof.Headers) {
		sources = append(sources, "headers")
	}
	if persistentQueryContainCredentials(prof.Query) {
		sources = append(sources, "query")
	}
	return sources
}

func persistentHeadersContainCredentials(headers []string) bool {
	for _, header := range headers {
		name, _, ok := strings.Cut(header, ":")
		if ok && secrets.IsHeaderName(strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func persistentQueryContainCredentials(query []string) bool {
	for _, item := range query {
		name, _, ok := strings.Cut(item, "=")
		if ok && secrets.IsQueryParamName(strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func isSensitiveConfigKey(key string) bool {
	lower := strings.ToLower(key)
	return lower == "password" ||
		strings.HasSuffix(lower, "_password") ||
		lower == "secret" ||
		strings.HasSuffix(lower, "_secret") ||
		lower == "token" ||
		strings.HasSuffix(lower, "_token") ||
		lower == "api_key" ||
		lower == "apikey" ||
		lower == "access_key" ||
		lower == "private_key" ||
		lower == "client_secret"
}
