package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/spf13/cobra"
)

func (c *CLI) addConfigCommand(root *cobra.Command) {
	configCmd := &cobra.Command{
		Use:     "config",
		Short:   "Manage local Restish configuration",
		GroupID: rootGroupConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown config command %q", args[0])
			}
			return cmd.Help()
		},
	}
	configCmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the active config file path",
		Args:  cobra.NoArgs,
		RunE:  c.runConfigPath,
	})
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Print the active config summary or redacted JSON",
		Args:  cobra.NoArgs,
		RunE:  c.runConfigShow,
	}
	showCmd.Flags().Bool("json", false, "Print the full redacted config as JSON")
	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the restish config file in $VISUAL or $EDITOR",
		Args:  cobra.NoArgs,
		RunE:  c.runConfigEdit,
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "set <patch> [patch...]",
		Short: "Patch config using shorthand syntax",
		Args:  cobra.MinimumNArgs(1),
		RunE:  c.runConfigSet,
	})
	themeCmd := &cobra.Command{
		Use:   "theme",
		Short: "Manage readable output highlighting theme",
	}
	themeCmd.AddCommand(c.newThemeSetCommand())
	themeCmd.AddCommand(c.newThemeResetCommand())
	configCmd.AddCommand(themeCmd)
	root.AddCommand(configCmd)
}

func (c *CLI) runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(c.Stdout, c.configFilePath())
	return nil
}

func (c *CLI) runConfigShow(cmd *cobra.Command, args []string) error {
	asJSON, _ := cmd.Flags().GetBool("json")
	if asJSON {
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
	fmt.Fprintf(c.Stdout, "Config file: %s\n", c.configFilePath())
	fmt.Fprintf(c.Stdout, "APIs: %d\n", apiCount)
	fmt.Fprintf(c.Stdout, "Auth profiles: %d\n", authProfileCount)
	fmt.Fprintf(c.Stdout, "Plugins: %d\n", pluginCount)
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

func (c *CLI) addContentTypesCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:     "content-types",
		Short:   "List registered content types and their MIME types",
		GroupID: rootGroupUtility,
		Args:    cobra.NoArgs,
		RunE:    c.runContentTypes,
	})
}

func (c *CLI) runContentTypes(cmd *cobra.Command, args []string) error {
	for _, ct := range c.content.ContentTypes() {
		fmt.Fprintf(c.Stdout, "%-12s %s\n", ct.Name, strings.Join(ct.MIMETypes, ", "))
	}
	return nil
}
