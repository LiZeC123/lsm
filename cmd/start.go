package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <name>",
		Short: "Start a configured service",
		Long: `Start a configured service in the background.

The service is launched through the system shell in its configured work_dir,
detached from the terminal, so it keeps running after lsm exits. Its standard
streams are discarded.`,
		Args: argsExact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadManager()
			if err != nil {
				return err
			}
			name := args[0]
			pid, err := mgr.Start(name)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "service '%s' started (pid %d)\n", name, pid)
			return nil
		},
	}
}
