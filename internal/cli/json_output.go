package cli

import (
	"github.com/rest-sh/restish/v2/internal/output"
)

func (c *CLI) writePrettyJSON(value any) error {
	return (&output.JSONFormatter{}).Format(c.Stdout, &output.Response{Body: value}, output.ColorEnabled(c.Stdout))
}
