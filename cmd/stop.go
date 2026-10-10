package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running service",
		Long: `Stop a running service.

The whole process group is asked to terminate with SIGTERM and is killed with
SIGKILL if it has not exited after ten seconds. Stopping a service that is not
running succeeds and reports that fact.`,
		Args: argsExact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSession()
			if err != nil {
				return err
			}
			name := args[0]
			result, err := s.mgr.Stop(name)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if !result.WasRunning {
				fmt.Fprintf(out, "service '%s' is not running\n", name)
				return nil
			}
			fmt.Fprintf(out, "service '%s' stopped\n", name)
			return nil
		},
	}
}
