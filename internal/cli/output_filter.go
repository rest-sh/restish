package cli

import (
	"fmt"

	"github.com/danielgtaylor/restish/v2/internal/filter"
	"github.com/spf13/cobra"
)

// filterOutput applies the active CLI filter expression to doc and, when
// --rsh-raw is set, writes the filtered value directly to stdout.
func (c *CLI) filterOutput(cmd *cobra.Command, filterExpr string, doc map[string]any, lang filter.Lang) (any, bool, error) {
	rawMode, _ := cmd.Flags().GetBool("rsh-raw")

	filtered, err := filter.Apply(filterExpr, doc, lang)
	if err != nil {
		return nil, false, fmt.Errorf("filter: %w", err)
	}
	if rawMode {
		return nil, true, c.writeRaw(filtered)
	}
	return filtered, false, nil
}
