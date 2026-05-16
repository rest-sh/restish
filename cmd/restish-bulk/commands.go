package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielgtaylor/shorthand/v2"
	"github.com/spf13/cobra"
)

type app struct {
	client *pluginClient
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
	return root
}

func (a *app) newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init URL",
		Aliases: []string{"i"},
		Short:   "Initialize a new bulk checkout",
		Long:    "Initialize a bulk checkout from a list endpoint that returns each resource URL and version, optionally transforming the list with -f and --url-template first.",
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
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			match, _ := cmd.Flags().GetString("match")
			filterExpr, _ := cmd.Flags().GetString("filter")
			files, err := collectFiles(meta, nil, match, false)
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
					fmt.Fprintln(&buf, changed.String())
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
					fmt.Fprintln(&buf, changed.String())
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
			files, err := collectFiles(meta, args, match, true)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadMeta()
			if err != nil {
				return err
			}
			match, _ := cmd.Flags().GetString("match")
			files, err := collectFiles(meta, args, match, true)
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
