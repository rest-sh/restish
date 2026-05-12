package cli

import (
	"fmt"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
)

func (c *CLI) writePrettyJSON(value any) error {
	return (&output.JSONFormatter{}).Format(c.Stdout, &output.Response{Body: value}, output.ColorEnabled(c.Stdout))
}

func commandJSONOutputRequested(cmd *cobra.Command) (bool, error) {
	fmtName := globalFlagsFromContext(requestContext(cmd)).OutputFormat
	switch fmtName {
	case "":
		return false, nil
	case "json":
		return true, nil
	default:
		return false, fmt.Errorf("%s supports -o json for structured output, not -o %s", cmd.CommandPath(), fmtName)
	}
}
