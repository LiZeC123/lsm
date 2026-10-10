package cmd

import (
	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/spf13/cobra"
)

// argsNone and argsExact wrap cobra's positional argument validators and tag
// their failures as usage errors, so a mistyped command line exits with code 2
// (dev.md §9.1) instead of the generic code 1.
func argsNone() cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.NoArgs(cmd, args); err != nil {
			return errs.Classify(errs.Usage, err)
		}
		return nil
	}
}

func argsExact(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(n)(cmd, args); err != nil {
			return errs.Classify(errs.Usage, err)
		}
		return nil
	}
}
