package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/internal/auth"
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
