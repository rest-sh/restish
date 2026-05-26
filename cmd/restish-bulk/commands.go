package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/danielgtaylor/shorthand/v2"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
)

type app struct {
	client       *pluginClient
	schemaCache  map[string]any
	schemaMisses map[string]bool
}

func run(client *pluginClient, args []string) error {
	a := &app{client: client}
	root := a.newRootCmd()
	root.SetArgs(args)
	root.SetOut(client.StdoutWriter())
	root.SetErr(client.StderrWriter())
	return root.Execute()
}

func (a *app) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "bulk",
		Short:         "Client-side bulk resource management",
		Long:          "Check out collections of remote API resources to disk, track local and remote changes, diff them, and push updates back in bulk.\n\nUse `restish bulk init` on a list endpoint that returns resource URLs and versions. Then use `restish bulk status`, `restish bulk diff`, `restish bulk pull`, and `restish bulk push` in the checkout directory.",
		Example:       "  restish bulk init https://api.rest.sh/books\n  restish bulk status\n  restish bulk diff\n  restish bulk pull\n  restish bulk push",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.AddCommand(a.newInitCmd())
	root.AddCommand(a.newListCmd())
	root.AddCommand(a.newStatusCmd())
	root.AddCommand(a.newDiffCmd())
	root.AddCommand(a.newResetCmd())
	root.AddCommand(a.newPullCmd())
	root.AddCommand(a.newPushCmd())
	polishBulkHelp(root)
	return root
}

func (a *app) colorEnabled() bool {
	return a != nil && a.client != nil && a.client.term.Color
}

func (a *app) colorizeDiff(diff string) string {
	if !a.colorEnabled() {
		return diff
	}
	lexer := lexers.Get("diff")
	if lexer == nil {
		return diff
	}
	colored, err := output.HighlightWithLexer(lexer, []byte(diff))
	if err != nil {
		return diff
	}
	return string(colored)
}

func (a *app) colorizeJSON(data []byte) []byte {
	if !a.colorEnabled() {
		return data
	}
	colored, err := output.HighlightWithLexer(output.ReadableLexer, data)
	if err != nil {
		return data
	}
	return colored
}

func (a *app) statusLine(changed changedFile) string {
	return changed.StringColor(a.colorEnabled())
}

func (a *app) progress(text string) error {
	if a == nil || a.client == nil {
		return nil
	}
	return a.client.Progress(text)
}

func (a *app) collectFiles(meta *Meta, args []string, match string, includeDeleted bool) ([]string, error) {
	warn := func(text string) error {
		if a == nil || a.client == nil {
			return nil
		}
		return a.client.Warn(text)
	}
	return collectFilesWithOptions(meta, args, match, includeDeleted, warn, a.schemaExample)
}

func polishBulkHelp(root *cobra.Command) {
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(&cobra.Command{
		Use:                "help [command]",
		Short:              "Help about any command",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := root
			if len(args) > 0 {
				found, _, err := root.Find(args)
				if err != nil {
					return err
				}
				target = found
			}
			return target.Help()
		},
	})
	helpTemplate := `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasAvailableSubCommands}}Usage:
{{if .Runnable}}  restish {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}{{if .Runnable}}
{{end}}  restish {{.CommandPath}} [command]{{end}}

{{end}}{{with .Example}}Examples:
{{.}}

{{end}}{{if .HasAvailableSubCommands}}Available Commands:
{{range .Commands}}{{if and (ne .Name "help") .IsAvailableCommand}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}

{{end}}{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}{{if .HasAvailableInheritedFlags}}Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}{{if .HasHelpSubCommands}}Additional help topics:
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}

{{end}}{{if .HasAvailableSubCommands}}Use "restish {{.CommandPath}} [command] --help" for more information about a command.
{{end}}`
	setBulkHelpTemplate(root, helpTemplate)
}

func setBulkHelpTemplate(cmd *cobra.Command, tmpl string) {
	cmd.SetHelpTemplate(tmpl)
	for _, child := range cmd.Commands() {
		setBulkHelpTemplate(child, tmpl)
	}
}

func (a *app) newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init URL",
		Aliases: []string{"i"},
		Short:   "Initialize a new bulk checkout",
		Long:    "Initialize a bulk checkout from a list endpoint that returns each resource URL and version.\n\nUse `-f` to project or filter the list response before URL extraction. Use `--url-template` when the list items contain IDs or fields that need to be turned into resource URLs.",
		Example: "  restish bulk init https://api.rest.sh/books\n  restish bulk init https://api.example.com/users --url-template '/users/{id}'\n  restish bulk status",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter, _ := cmd.Flags().GetString("filter")
			template, _ := cmd.Flags().GetString("url-template")
			jobs, _ := cmd.Flags().GetInt("jobs")
			meta := &Meta{
				URL:         args[0],
				Filter:      filter,
				URLTemplate: template,
				Files:       map[string]*File{},
			}
			if err := meta.save(); err != nil {
				return err
			}
			return a.pull(meta, jobs)
		},
	}
	cmd.Flags().StringP("filter", "f", "", "Filter/project the list response before extracting url/version")
	cmd.Flags().String("url-template", "", "URL template to build resource links, e.g. /users/{id}")
	cmd.Flags().IntP("jobs", "j", defaultJobs, "Maximum concurrent resource requests")
	return cmd
}

func (a *app) newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List checked out files",
		Long:    "List files tracked by the current bulk checkout.\n\nUse `--match` to restrict files by expression and `-f` to print projected content from each matching JSON file.",
		Example: "  restish bulk list\n  restish bulk list --match 'id contains book'\n  restish bulk list -f title",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			match, _ := cmd.Flags().GetString("match")
			filterExpr, _ := cmd.Flags().GetString("filter")
			files, err := a.collectFiles(meta, nil, match, false)
			if err != nil {
				return err
			}
			for _, path := range files {
				if err := a.client.WriteStdout([]byte(path + "\n")); err != nil {
					return err
				}
				if filterExpr == "" {
					continue
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				var content any
				if err := json.Unmarshal(data, &content); err != nil {
					return fmt.Errorf("%s contains invalid JSON: %w", path, err)
				}
				res, _, err := shorthand.GetPath(filterExpr, content, shorthand.GetOptions{})
				if err != nil || isFalsey(res) {
					continue
				}
				formatted, err := prettyJSON(res)
				if err != nil {
					return err
				}
				formatted = a.colorizeJSON(formatted)
				if err := a.client.WriteStdout(append(formatted, '\n')); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringP("match", "m", "", "Expression to match")
	cmd.Flags().StringP("filter", "f", "", "Show projected content for each matched file")
	return cmd
}

func (a *app) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"st"},
		Short:   "Show local and remote added/changed/removed files",
		Long:    "Show local and remote added, changed, and removed resources for the current checkout.\n\nUse this before `bulk pull` or `bulk push` to see whether the remote API or local files have changed since the last recorded version.",
		Example: "  restish bulk status\n  restish bulk diff\n  restish bulk pull",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			files, err := collectFiles(meta, nil, "", false)
			if err != nil {
				return err
			}
			local, remote, err := a.getChanged(meta, files)
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			if len(remote) > 0 {
				fmt.Fprintf(&buf, "Remote changes on %s\n  (use \"restish bulk pull\" to update)\n", normalizedBaseURL(meta.URL))
				for _, changed := range remote {
					fmt.Fprintln(&buf, a.statusLine(changed))
				}
			} else {
				fmt.Fprintf(&buf, "You are up to date with %s\n", normalizedBaseURL(meta.URL))
			}
			if len(local) == 0 {
				fmt.Fprintln(&buf, "No local changes")
			} else {
				fmt.Fprintln(&buf, "Local changes:")
				fmt.Fprintln(&buf, "  (use \"restish bulk reset [file]...\" to undo)")
				fmt.Fprintln(&buf, "  (use \"restish bulk diff [file]...\" to view changes)")
				for _, changed := range local {
					fmt.Fprintln(&buf, a.statusLine(changed))
				}
			}
			return a.client.WriteStdout(buf.Bytes())
		},
	}
}

func (a *app) newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "diff [file...]",
		Aliases: []string{"di"},
		Short:   "Show local or remote diffs",
		Long:    "Show local diffs for tracked files, or remote diffs with `--remote`.\n\nPass file names to focus the diff. Use `--match` to select files by expression when file paths are inconvenient.",
		Example: "  restish bulk diff\n  restish bulk diff books/123.json\n  restish bulk diff --remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			match, _ := cmd.Flags().GetString("match")
			remote, _ := cmd.Flags().GetBool("remote")
			if remote {
				return a.remoteDiff(meta)
			}
			files, err := a.collectFiles(meta, args, match, true)
			if err != nil {
				return err
			}
			return a.localDiff(meta, files)
		},
	}
	cmd.Flags().StringP("match", "m", "", "Expression to match")
	cmd.Flags().Bool("remote", false, "Show remote diffs instead of local")
	return cmd
}

func (a *app) newResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reset [file...]",
		Aliases: []string{"re"},
		Short:   "Undo local changes to files",
		Long:    "Undo local changes in the current checkout by restoring tracked files to their last recorded version.\n\nPass file names or use `--match` to limit what is reset. This changes local files only; it does not send requests to the remote API.",
		Example: "  restish bulk status\n  restish bulk reset books/123.json\n  restish bulk reset --match 'id == \"123\"'",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			match, _ := cmd.Flags().GetString("match")
			files, err := a.collectFiles(meta, args, match, true)
			if err != nil {
				return err
			}
			for _, name := range files {
				f := meta.Files[name]
				if f == nil || f.VersionLocal == "" {
					continue
				}
				if err := f.reset(); err != nil {
					return err
				}
			}
			return meta.save()
		},
	}
	cmd.Flags().StringP("match", "m", "", "Expression to match")
	return cmd
}

func (a *app) newPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pull",
		Aliases: []string{"pl"},
		Short:   "Pull remote updates without overwriting local changes",
		Long:    "Fetch remote changes for the current checkout without overwriting local edits.\n\nUse this after `bulk status` reports remote changes. `--jobs` controls how many resource requests run concurrently.",
		Example: "  restish bulk status\n  restish bulk pull\n  restish bulk pull --jobs 8",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			jobs, _ := cmd.Flags().GetInt("jobs")
			return a.pull(meta, jobs)
		},
	}
	cmd.Flags().IntP("jobs", "j", defaultJobs, "Maximum concurrent resource requests")
	return cmd
}

func (a *app) newPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "push",
		Aliases: []string{"ps"},
		Short:   "Upload local changes to the remote server",
		Long:    "Upload local changes from the current checkout to the remote API.\n\nBy default, bulk uses recorded `ETag`, `Last-Modified`, or version preconditions when available so remote changes are not silently overwritten. Use `--force` only when you intentionally want to push without those guards.",
		Example: "  restish bulk status\n  restish bulk diff\n  restish bulk push\n  restish bulk push --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			jobs, _ := cmd.Flags().GetInt("jobs")
			force, _ := cmd.Flags().GetBool("force")
			return a.push(meta, jobs, pushOptions{Force: force})
		},
	}
	cmd.Flags().IntP("jobs", "j", defaultJobs, "Maximum concurrent resource requests")
	cmd.Flags().Bool("force", false, "Push without ETag/Last-Modified or matching version preconditions")
	return cmd
}
