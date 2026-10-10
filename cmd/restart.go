package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart a service",
		Long: `Restart a service.

A running service is stopped first and then started again; a service that is
not running is simply started.`,
		Args: argsExact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSession()
			if err != nil {
				return err
			}
			name := args[0]
			result, err := s.mgr.Restart(name)
			if err != nil {
				return err
			}

			verb := "started"
			if result.WasRunning {
				verb = "restarted"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "service '%s' %s (pid %d)\n", name, verb, result.PID)
			return nil
		},
	}
}
