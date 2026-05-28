package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rest-sh/restish/v2/internal/cache"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/spf13/cobra"
)

// addCacheCommand registers the "cache" subcommand tree on root.
func (c *CLI) addCacheCommand(root *cobra.Command) {
	cacheCmd := &cobra.Command{
		Use:     "cache",
		Short:   "Manage the HTTP response cache",
		Long:    cacheLong,
		GroupID: rootGroupConfig,
		Example: fmt.Sprintf(`  %s cache info
  %s cache clear
  %s cache clear demo`, c.commandNameOrDefault(), c.commandNameOrDefault(), c.commandNameOrDefault()),
		RunE: unknownSubcommandRun("cache"),
	}
	cacheCmd.AddCommand(c.newCacheInfoCmd(), c.newCacheClearCmd())
	root.AddCommand(cacheCmd)
}

func (c *CLI) newCacheInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Print cache directory, size, entry count, oldest entry, and largest hosts",
		Long:  cacheInfoLong,
		Example: fmt.Sprintf(`  %s cache info
  %s cache info -o json`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: usageNoArgs,
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
			if jsonOut, err := commandJSONOutputRequested(cmd); err != nil {
				return err
			} else if jsonOut {
				type cacheInfoOutput struct {
					Directory      string                  `json:"directory"`
					SizeBytes      int64                   `json:"size_bytes"`
					Size           string                  `json:"size"`
					Entries        int                     `json:"entries"`
					OldestEntry    string                  `json:"oldest_entry,omitempty"`
					TopAPIProfiles []cacheAPIProfileOutput `json:"top_api_profiles"`
					TopHosts       []cacheBreakdownOutput  `json:"top_hosts"`
				}
				out := cacheInfoOutput{
					Directory:      dir,
					SizeBytes:      info.SizeBytes,
					Size:           formatBytes(info.SizeBytes),
					Entries:        info.EntryCount,
					TopAPIProfiles: cacheAPIProfileOutputs(info.Namespaces, info.SizeBytes, c.cfg, c.projectConfig, 10),
					TopHosts:       cacheBreakdownOutputs(info.Hosts, info.SizeBytes, 10),
				}
				if !info.OldestEntry.IsZero() {
					out.OldestEntry = info.OldestEntry.Format(time.RFC3339)
				}
				return c.writePrettyJSON(out)
			}
			style := humanTextStyleFor(c.Stdout)
			fmt.Fprintf(c.Stdout, "%s %s\n", style.key("Directory:"), dir)
			fmt.Fprintf(c.Stdout, "%s      %s\n", style.key("Size:"), formatBytes(info.SizeBytes))
			fmt.Fprintf(c.Stdout, "%s   %d\n", style.key("Entries:"), info.EntryCount)
			if !info.OldestEntry.IsZero() {
				fmt.Fprintf(c.Stdout, "%s    %s\n", style.key("Oldest:"), info.OldestEntry.Format(time.RFC3339))
			}
			if c.stdoutIsTerminal() {
				width, height := cacheTreemapSize(c.Stdout)
				printCacheTreemap(c.Stdout, style, "Usage map by API/profile:", info.Namespaces, c.cfg, c.projectConfig, width, height, 10)
			}
			printCacheAPIBreakdown(c.Stdout, style, "Largest APIs/profiles:", info.Namespaces, info.SizeBytes, c.cfg, c.projectConfig, 10)
			printCacheBreakdown(c.Stdout, style, "Largest hosts:", info.Hosts, info.SizeBytes, 10)
			return nil
		},
	}
}

type cacheBreakdownOutput struct {
	Name        string `json:"name"`
	SizeBytes   int64  `json:"size_bytes"`
	Size        string `json:"size"`
	Percent     string `json:"percent"`
	Entries     int    `json:"entries"`
	OldestEntry string `json:"oldest_entry,omitempty"`
}

type cacheAPIProfileOutput struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	API         string `json:"api,omitempty"`
	Profile     string `json:"profile,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
	Size        string `json:"size"`
	Percent     string `json:"percent"`
	Entries     int    `json:"entries"`
	OldestEntry string `json:"oldest_entry,omitempty"`
}

type cacheNamespaceDetails struct {
	name      string
	namespace string
	api       string
	profile   string
}

const cacheSizeColumnWidth = 9

func cacheBreakdownOutputs(items []cache.Breakdown, totalBytes int64, limit int) []cacheBreakdownOutput {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]cacheBreakdownOutput, 0, len(items))
	for _, item := range items {
		entry := cacheBreakdownOutput{
			Name:      item.Name,
			SizeBytes: item.SizeBytes,
			Size:      formatBytes(item.SizeBytes),
			Percent:   formatCachePercent(item.SizeBytes, totalBytes),
			Entries:   item.EntryCount,
		}
		if !item.OldestEntry.IsZero() {
			entry.OldestEntry = item.OldestEntry.Format(time.RFC3339)
		}
		out = append(out, entry)
	}
	return out
}

func cacheAPIProfileOutputs(items []cache.Breakdown, totalBytes int64, cfg *config.Config, project *projectConfigState, limit int) []cacheAPIProfileOutput {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]cacheAPIProfileOutput, 0, len(items))
	for _, item := range items {
		details := cacheNamespaceInfo(item.Name, cfg, project)
		entry := cacheAPIProfileOutput{
			Name:      details.name,
			Namespace: details.namespace,
			API:       details.api,
			Profile:   details.profile,
			SizeBytes: item.SizeBytes,
			Size:      formatBytes(item.SizeBytes),
			Percent:   formatCachePercent(item.SizeBytes, totalBytes),
			Entries:   item.EntryCount,
		}
		if !item.OldestEntry.IsZero() {
			entry.OldestEntry = item.OldestEntry.Format(time.RFC3339)
		}
		out = append(out, entry)
	}
	return out
}

func printCacheBreakdown(w io.Writer, style humanTextStyle, title string, items []cache.Breakdown, totalBytes int64, limit int) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(w, style.key(title))
	shown := len(items)
	if limit > 0 && shown > limit {
		shown = limit
	}
	fmt.Fprintf(w, "  %s\n", style.hint(fmt.Sprintf("%-36s %*s  %6s  %s", "Name", cacheSizeColumnWidth, "Size", "%", "Entries")))
	for _, item := range items[:shown] {
		entryWord := "entries"
		if item.EntryCount == 1 {
			entryWord = "entry"
		}
		fmt.Fprintf(w, "  %-36s %*s  %6s  %d %s\n", item.Name, cacheSizeColumnWidth, formatBytes(item.SizeBytes), formatCachePercent(item.SizeBytes, totalBytes), item.EntryCount, entryWord)
	}
	if hidden := len(items) - shown; hidden > 0 {
		fmt.Fprintf(w, "  %s\n", style.hint(fmt.Sprintf("...and %d more", hidden)))
	}
}

func printCacheAPIBreakdown(w io.Writer, style humanTextStyle, title string, items []cache.Breakdown, totalBytes int64, cfg *config.Config, project *projectConfigState, limit int) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintln(w, style.key(title))
	shown := len(items)
	if limit > 0 && shown > limit {
		shown = limit
	}
	fmt.Fprintf(w, "  %s\n", style.hint(fmt.Sprintf("%-32s %*s  %6s  %s", "Name", cacheSizeColumnWidth, "Size", "%", "Entries")))
	for _, item := range items[:shown] {
		details := cacheNamespaceInfo(item.Name, cfg, project)
		entryWord := "entries"
		if item.EntryCount == 1 {
			entryWord = "entry"
		}
		fmt.Fprintf(w, "  %-32s %*s  %6s  %d %s\n", details.name, cacheSizeColumnWidth, formatBytes(item.SizeBytes), formatCachePercent(item.SizeBytes, totalBytes), item.EntryCount, entryWord)
	}
	if hidden := len(items) - shown; hidden > 0 {
		fmt.Fprintf(w, "  %s\n", style.hint(fmt.Sprintf("...and %d more", hidden)))
	}
}

func cacheNamespaceInfo(namespace string, cfg *config.Config, project *projectConfigState) cacheNamespaceDetails {
	details := cacheNamespaceDetails{
		name:      namespace,
		namespace: namespace,
	}
	if namespace == "" || namespace == "_" {
		details.name = "(direct URL requests)"
		return details
	}
	apiName, profileName, ok := strings.Cut(namespace, ":")
	if !ok || apiName == "" || profileName == "" {
		return details
	}
	if logicalAPIName, ok := projectCacheNamespaceAPI(apiName, project); ok {
		apiName = logicalAPIName
	}
	details.api = apiName
	details.profile = profileName
	if cfg != nil && cfg.APIs != nil && cfg.APIs[apiName] != nil {
		details.name = fmt.Sprintf("%s (%s)", apiName, profileName)
	} else {
		details.name = fmt.Sprintf("%s (%s, unregistered)", apiName, profileName)
	}
	return details
}

func projectCacheNamespaceAPI(stateName string, project *projectConfigState) (string, bool) {
	if project == nil || !project.Trusted || project.Namespace == "" {
		return "", false
	}
	prefix := "project-" + project.Namespace + "-"
	apiName, ok := strings.CutPrefix(stateName, prefix)
	if !ok || apiName == "" || !project.APIs[apiName] {
		return "", false
	}
	return apiName, true
}

func formatCachePercent(sizeBytes, totalBytes int64) string {
	if sizeBytes <= 0 || totalBytes <= 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", float64(sizeBytes)*100/float64(totalBytes))
}

func (c *CLI) newCacheClearCmd() *cobra.Command {
	var direct bool
	cmd := &cobra.Command{
		Use:   "clear [api-or-namespace]",
		Short: "Delete cached HTTP responses, not OAuth tokens (omit API to clear all)",
		Long:  cacheClearLong,
		Example: fmt.Sprintf(`  %s cache clear
  %s cache clear demo
  %s cache clear --direct`, c.commandNameOrDefault(), c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: func(cmd *cobra.Command, args []string) error {
			if direct && len(args) > 0 {
				return newUsageError(fmt.Errorf("--direct cannot be used with an API or namespace argument; use either %q or %q", cmd.CommandPath()+" --direct", cmd.CommandPath()+" "+args[0]))
			}
			return usageMaximumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := c.cacheDir()
			dc, err := cache.New(dir, cache.DefaultMaxBytes, "")
			if err != nil {
				return err
			}

			if direct {
				if err := dc.ClearNamespaces([]string{"_"}); err != nil {
					return err
				}
				style := humanTextStyleFor(c.Stdout)
				fmt.Fprintf(c.Stdout, "%s for direct URL requests.\n", style.ok("Cache cleared"))
				return nil
			}

			if len(args) == 1 {
				apiName := args[0]
				registered := c.cfg != nil && c.cfg.APIs != nil && c.cfg.APIs[apiName] != nil
				namespace := apiName
				if registered {
					namespace = c.apiStateName(apiName)
				}
				cleared, err := dc.ClearNamespacePrefix(namespace + ":")
				if err != nil {
					return err
				}
				if !registered && cleared == 0 {
					return fmt.Errorf("unknown API or cached namespace %q; run %q to see cached API/profile namespaces, or omit the argument to clear every HTTP cache entry", apiName, c.commandNameOrDefault()+" cache info")
				}
				style := humanTextStyleFor(c.Stdout)
				if registered {
					fmt.Fprintf(c.Stdout, "%s for API %q.\n", style.ok("Cache cleared"), args[0])
				} else {
					fmt.Fprintf(c.Stdout, "%s for cached namespace %q.\n", style.ok("Cache cleared"), args[0])
				}
				return nil
			}

			if err := dc.Clear(""); err != nil {
				return err
			}
			style := humanTextStyleFor(c.Stdout)
			fmt.Fprintf(c.Stdout, "%s.\n", style.ok("Cache cleared"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&direct, "direct", false, "Clear cached responses for direct URL requests that are not associated with a registered API")
	return cmd
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
