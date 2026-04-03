package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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

	profileName, _ := cmd.Flags().GetString("rsh-profile")
	if profileName == "" {
		profileName = os.Getenv("RSH_PROFILE")
	}
	if profileName == "" {
		profileName = "default"
	}

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

	apiSpec, err := c.discoverSpec(context.Background(), apiName)
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
		Transport: http.DefaultTransport,
	}
	apiSpec, _ := spec.Discover(context.Background(), discCfg, c.loaders)

	// Build the API config entry.
	apiCfg := &config.APIConfig{BaseURL: baseURL}
	if apiSpec != nil {
		if xcli, err := spec.ReadXCLIConfig(apiSpec); err == nil && xcli != nil {
			applyXCLIConfig(apiCfg, xcli)
		}
	}

	// Load, update, and save the config.
	cfgPath := c.configFilePath()
	cfg, _ := config.Load(cfgPath)
	if cfg == nil {
		cfg = &config.Config{}
	}
	if cfg.APIs == nil {
		cfg.APIs = make(map[string]*config.APIConfig)
	}
	cfg.APIs[apiName] = apiCfg

	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Configured API %q with base URL %s\n", apiName, baseURL)
	return nil
}

// applyXCLIConfig merges x-cli-config fields into apiCfg.
func applyXCLIConfig(apiCfg *config.APIConfig, xcli *spec.XCLIConfig) {
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
			prof.Auth = &config.AuthConfig{
				Type:   xp.Auth.Type,
				Params: xp.Auth.Params,
			}
		}
		apiCfg.Profiles[name] = prof
	}
}

// runAPIShow prints the config for a named API as indented JSON.
func (c *CLI) runAPIShow(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}
	data, err := json.MarshalIndent(c.cfg.APIs[apiName], "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, string(data))
	return nil
}

// runAPIEdit opens the restish config file in $VISUAL or $EDITOR.
func (c *CLI) runAPIEdit(cmd *cobra.Command, args []string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		return fmt.Errorf("no editor found; set $VISUAL or $EDITOR")
	}
	cfgPath := c.configFilePath()
	editorCmd := exec.Command(editor, cfgPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	return editorCmd.Run()
}

// runAPISet updates a single config field for a named API using a dot-path key.
// Supported keys: base_url, spec_url, profiles.<name>.base_url.
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

	return config.Save(c.configFilePath(), c.cfg)
}

// setAPIField updates a single field of apiCfg identified by a dot-path key.
func setAPIField(apiCfg *config.APIConfig, key, value string) error {
	parts := strings.SplitN(key, ".", 3)
	switch parts[0] {
	case "base_url":
		apiCfg.BaseURL = value
	case "spec_url":
		apiCfg.SpecURL = value
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
		switch parts[2] {
		case "base_url":
			prof.BaseURL = value
		default:
			return fmt.Errorf("unsupported profile field %q", parts[2])
		}
	default:
		return fmt.Errorf("unsupported field %q", key)
	}
	return nil
}

// runAPIContentTypes lists all registered content types and their MIME types.
func (c *CLI) runAPIContentTypes(cmd *cobra.Command, args []string) error {
	for _, ct := range c.content.ContentTypes() {
		fmt.Fprintf(c.Stdout, "%-12s %s\n", ct.Name, strings.Join(ct.MIMETypes, ", "))
	}
	return nil
}
