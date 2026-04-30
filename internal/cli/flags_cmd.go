package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var flagReferenceGroups = []struct {
	Name  string
	Group string
	Short string
}{
	{Name: "request", Group: flagGroupRequest, Short: "Show request construction flags"},
	{Name: "output", Group: flagGroupOutput, Short: "Show output and filtering flags"},
	{Name: "auth", Group: flagGroupAuth, Short: "Show auth and profile flags"},
	{Name: "tls", Group: flagGroupTLS, Short: "Show TLS flags"},
	{Name: "pagination", Group: flagGroupPaging, Short: "Show pagination and streaming flags"},
	{Name: "cache", Group: flagGroupCache, Short: "Show cache and retry flags"},
	{Name: "general", Group: flagGroupGeneral, Short: "Show general flags"},
}

func (c *CLI) addFlagsCommand(root *cobra.Command) {
	flagsCmd := &cobra.Command{
		Use:     "flags",
		Short:   "Show Restish global flag reference",
		GroupID: rootGroupHelp,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.printFlagReference(cmd.Root().PersistentFlags(), "")
		},
	}
	for _, item := range flagReferenceGroups {
		item := item
		flagsCmd.AddCommand(&cobra.Command{
			Use:   item.Name,
			Short: item.Short,
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.printFlagReference(cmd.Root().PersistentFlags(), item.Group)
			},
		})
	}
	root.AddCommand(flagsCmd)
}

func (c *CLI) printFlagReference(flags *pflag.FlagSet, group string) error {
	if flags == nil {
		return nil
	}
	if group == "" {
		usage := strings.TrimSpace(groupedFlagUsages(flags))
		if usage != "" {
			fmt.Fprintln(c.Stdout, usage)
		}
		return nil
	}

	filtered := pflag.NewFlagSet("", pflag.ContinueOnError)
	filtered.SortFlags = flags.SortFlags
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden || flagGroupFor(flag) != group {
			return
		}
		filtered.AddFlag(flag)
	})
	usage := strings.TrimSpace(groupedFlagUsages(filtered))
	if usage != "" {
		fmt.Fprintln(c.Stdout, usage)
	}
	return nil
}
