package cmd

import (
	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/process"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <name>",
		Short: "Run a configured service in the foreground",
		Long: `Run a configured service in the foreground, attached to the terminal.

This is meant for temporary debugging. The command is interpreted by the system
shell in the configured work_dir, exactly like start, but its standard input,
output and error streams stay connected to the terminal instead of being
discarded. lsm exits together with the command and returns its exit status.

No PID file is written, and the background instance managed by start and stop is
neither consulted nor disturbed.`,
		Args: argsExact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSession()
			if err != nil {
				return err
			}
			name := args[0]

			svc, ok := s.cfg.Get(name)
			if !ok {
				return errs.New(errs.NotFound, "service '%s' not found", name)
			}
			// Without this check a missing directory would only surface as an
			// opaque chdir error from the shell launch.
			if err := config.CheckWorkDir(svc.WorkDir); err != nil {
				return errs.Classify(errs.ProcessOp, err)
			}

			status, err := process.Run(svc, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return errs.Wrap(errs.ProcessOp, err, "service '%s' failed to run", name)
			}
			// The command's status wins over lsm's own exit code contract: a
			// foreground run must look like the command was run directly.
			if status != 0 {
				return &errs.ExitStatus{Status: status}
			}
			return nil
		},
	}
}
