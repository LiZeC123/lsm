package cmd

import (
	"fmt"

	"github.com/LiZeC123/lsm/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the lsm version",
		Long: `Print the lsm version.

Release binaries carry the tag, the git revision and the build time; a binary
built from source with plain ` + "`go build`" + ` reports "lsm dev".`,
		Args: argsNone(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return nil
		},
	}
}
