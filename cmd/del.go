package cmd

import (
	"fmt"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/process"
	"github.com/LiZeC123/lsm/internal/ui"
	"github.com/spf13/cobra"
)

func newDelCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "del <name>",
		Short: "Delete a service configuration",
		Long: `Delete a service from the configuration after a confirmation.

A running service is not touched unless --force is given, in which case it is
stopped before the confirmation is asked. The PID file is removed as well.`,
		Args: argsExact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSession()
			if err != nil {
				return err
			}
			name := args[0]

			status, err := s.mgr.StatusOf(name)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if status.Running {
				if !force {
					return errs.New(errs.General,
						"Service '%s' is running (pid %d). Stop it first, or use --force.",
						name, status.PID)
				}
				fmt.Fprintf(out, "Stopping service '%s' (pid %d)...\n", name, status.PID)
				if _, err := s.mgr.Stop(name); err != nil {
					return err
				}
			}

			prompter := ui.NewPrompter(cmd.InOrStdin(), out)
			confirmed, err := prompter.Confirm(fmt.Sprintf("Delete service '%s'?", name))
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}

			delete(s.cfg.Services, name)
			if err := s.cfg.Save(s.path); err != nil {
				return err
			}
			// The PID file may still be there, for instance after a --force
			// stop of a service that was already gone (dev.md §5.8).
			if err := process.RemovePIDFile(config.PIDPath(s.path, name)); err != nil {
				return err
			}
			fmt.Fprintf(out, "Service '%s' deleted.\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "stop the service before deleting it")
	return cmd
}
