package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func requestContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}
