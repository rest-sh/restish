package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rest-sh/restish/v2/internal/config"
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
		Args:    cobra.NoArgs,
		RunE:    c.runConfigPath,
	})
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Print the active config summary or redacted JSON",
		Long:  configShowLong,
		Example: fmt.Sprintf(`  %s config show
  %s config show -o json`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: cobra.NoArgs,
		RunE: c.runConfigShow,
	}
	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(&cobra.Command{
		Use:     "edit",
		Short:   "Open the restish config file in $VISUAL or $EDITOR",
		Long:    configEditLong,
		Example: fmt.Sprintf("  %s config edit", c.commandNameOrDefault()),
		Args:    cobra.NoArgs,
		RunE:    c.runConfigEdit,
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "set <patch> [patch...]",
		Short: "Patch config using shorthand syntax",
		Long:  configSetLong,
		Example: fmt.Sprintf(`  %s config set 'cache.max_size: 500MB'
  %s config set 'theme.key: #afd787'`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: cobra.MinimumNArgs(1),
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

	apiCount := 0
	authProfileCount := 0
	pluginCount := 0
	if c.cfg != nil {
		apiCount = len(c.cfg.APIs)
		authProfileCount = len(c.cfg.AuthProfiles)
		pluginCount = len(c.cfg.Plugins)
	}
	style := humanTextStyleFor(c.Stdout)
	fmt.Fprintf(c.Stdout, "%s %s\n", style.key("Config file:"), c.configFilePath())
	fmt.Fprintf(c.Stdout, "%s %d\n", style.key("APIs:"), apiCount)
	fmt.Fprintf(c.Stdout, "%s %d\n", style.key("Auth profiles:"), authProfileCount)
	fmt.Fprintf(c.Stdout, "%s %d\n", style.key("Plugins:"), pluginCount)
	return nil
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
