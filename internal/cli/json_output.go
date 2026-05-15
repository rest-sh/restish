package cli

import (
	"fmt"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
)

func (c *CLI) writePrettyJSON(value any) error {
	return (&output.JSONFormatter{}).Format(c.Stdout, &output.Response{Body: value}, output.ColorEnabled(c.Stdout))
}

func commandJSONOutputRequested(cmd *cobra.Command) (bool, error) {
	gf := globalFlagsFromContext(requestContext(cmd))
	if err := rejectUnsupportedResponseTransformFlags(cmd, gf, true); err != nil {
		return false, err
	}
	fmtName := gf.OutputFormat
	switch fmtName {
	case "":
		return false, nil
	case "json":
		return true, nil
	default:
		return false, fmt.Errorf("%s supports -o json for structured output, not -o %s", cmd.CommandPath(), fmtName)
	}
}

func rejectResponseTransformFlags(cmd *cobra.Command) error {
	return rejectUnsupportedResponseTransformFlags(cmd, globalFlagsFromContext(requestContext(cmd)), false)
}

func rejectUnsupportedResponseTransformFlags(cmd *cobra.Command, gf GlobalFlags, allowJSON bool) error {
	if gf.OutputFormat != "" {
		if allowJSON && gf.OutputFormat == "json" {
			// Supported structured output for this command.
		} else if allowJSON {
			return fmt.Errorf("%s supports -o json for structured output, not -o %s", cmd.CommandPath(), gf.OutputFormat)
		} else {
			return fmt.Errorf("%s does not support -o/--rsh-output-format", cmd.CommandPath())
		}
	}
	if names := unsupportedResponseTransformFlagNames(cmd, gf); len(names) > 0 {
		return fmt.Errorf("%s does not support %s", cmd.CommandPath(), strings.Join(names, ", "))
	}
	return nil
}

func unsupportedResponseTransformFlagNames(cmd *cobra.Command, gf GlobalFlags) []string {
	var names []string
	if gf.PrintSet {
		names = append(names, "--rsh-print")
	}
	if gf.Filter != "" {
		names = append(names, "-f/--rsh-filter")
	}
	if gf.FilterLang != "" {
		names = append(names, "--rsh-filter-lang")
	}
	if gf.HeadersShorthand {
		names = append(names, "--rsh-headers")
	}
	if gf.StatusShorthand {
		names = append(names, "--rsh-status")
	}
	if gf.Columns != "" {
		names = append(names, "--rsh-columns")
	}
	if gf.SortBy != "" {
		names = append(names, "--rsh-sort-by")
	}
	if gf.NoPaginate {
		names = append(names, "--rsh-no-paginate")
	}
	if gf.Collect {
		names = append(names, "--rsh-collect")
	}
	if cmd.Flags().Changed("rsh-max-pages") {
		names = append(names, "--rsh-max-pages")
	}
	if gf.MaxItems != 0 {
		names = append(names, "--rsh-max-items")
	}
	return names
}
