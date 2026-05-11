package cli

import (
	"fmt"
	"time"

	"github.com/rest-sh/restish/v2/internal/cache"
	"github.com/spf13/cobra"
)

// addCacheCommand registers the "cache" subcommand tree on root.
func (c *CLI) addCacheCommand(root *cobra.Command) {
	cacheCmd := &cobra.Command{
		Use:     "cache",
		Short:   "Manage the HTTP response cache",
		GroupID: rootGroupConfig,
	}
	cacheCmd.AddCommand(c.newCacheInfoCmd(), c.newCacheClearCmd())
	root.AddCommand(cacheCmd)
}

func (c *CLI) newCacheInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Print cache directory, size, entry count, and oldest entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := c.cacheDir()
			dc, err := cache.New(dir, cache.DefaultMaxBytes, "")
			if err != nil {
				return err
			}
			info, err := dc.Info()
			if err != nil {
				return err
			}
			fmt.Fprintf(c.Stdout, "Directory: %s\n", dir)
			fmt.Fprintf(c.Stdout, "Size:      %s\n", formatBytes(info.SizeBytes))
			fmt.Fprintf(c.Stdout, "Entries:   %d\n", info.EntryCount)
			if !info.OldestEntry.IsZero() {
				fmt.Fprintf(c.Stdout, "Oldest:    %s\n", info.OldestEntry.Format(time.RFC3339))
			}
			return nil
		},
	}
}

func (c *CLI) newCacheClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear [api]",
		Short: "Delete cached HTTP responses, not OAuth tokens (omit API to clear all)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := c.cacheDir()
			dc, err := cache.New(dir, cache.DefaultMaxBytes, "")
			if err != nil {
				return err
			}

			if len(args) == 1 {
				apiName := args[0]
				if _, err := c.requireAPI(apiName); err != nil {
					return err
				}
				if err := dc.ClearNamespacePrefix(apiName + ":"); err != nil {
					return err
				}
				fmt.Fprintf(c.Stdout, "Cache cleared for API %q.\n", args[0])
				return nil
			}

			if err := dc.Clear(""); err != nil {
				return err
			}
			fmt.Fprintln(c.Stdout, "Cache cleared.")
			return nil
		},
	}
}

// formatBytes returns a human-readable byte size string.
func formatBytes(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GiB", float64(n)/gb)
	case n >= mb:
		return fmt.Sprintf("%.1f MiB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.1f KiB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
