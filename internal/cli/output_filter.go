package cli

import (
	"fmt"

	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/spf13/cobra"
)

// filterOutput applies the active CLI filter expression to doc.
func (c *CLI) filterOutput(cmd *cobra.Command, filterExpr string, doc map[string]any, lang filter.Lang) (any, bool, error) {
	filtered, err := filter.Apply(filterExpr, doc, lang)
	if err != nil {
		return nil, false, fmt.Errorf("filter: %w", err)
	}
	return filtered, false, nil
}
