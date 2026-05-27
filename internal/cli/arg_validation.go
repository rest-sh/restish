package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func usageNoArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return newUsageError(fmt.Errorf("%s does not accept arguments; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
}

func usageExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		switch {
		case len(args) == n:
			return nil
		case len(args) < n:
			return newUsageError(fmt.Errorf("%s is missing required argument(s); run %q for examples", cmd.CommandPath(), cmd.CommandPath()+" --help"))
		default:
			return newUsageError(fmt.Errorf("%s received too many arguments; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
		}
	}
}

func usageMinimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) >= n {
			return nil
		}
		return newUsageError(fmt.Errorf("%s is missing required argument(s); run %q for examples", cmd.CommandPath(), cmd.CommandPath()+" --help"))
	}
}

func usageMaximumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) <= n {
			return nil
		}
		return newUsageError(fmt.Errorf("%s received too many arguments; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
	}
}

func usageRangeArgs(min, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		switch {
		case len(args) >= min && len(args) <= max:
			return nil
		case len(args) < min:
			return newUsageError(fmt.Errorf("%s is missing required argument(s); run %q for examples", cmd.CommandPath(), cmd.CommandPath()+" --help"))
		default:
			return newUsageError(fmt.Errorf("%s received too many arguments; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
		}
	}
}
