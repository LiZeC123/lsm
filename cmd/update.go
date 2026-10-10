package cmd

import (
	"fmt"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/ui"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <name>",
		Short: "Update a service's command or working directory",
		Long: `Update the command or the working directory of a configured service.

Each question first prints the current value; answering with an empty line keeps
that value unchanged. A relative working directory answer is resolved against
the current directory.

Updating a running service is allowed: a process that is already up keeps the
old command until it is restarted.`,
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
			// The status decides whether the change needs a "takes effect on
			// restart" note, and surfaces a broken PID file before any editing.
			status, err := s.mgr.StatusOf(name)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			prompter := ui.NewPrompter(cmd.InOrStdin(), out)

			fmt.Fprintf(out, "Current command: %s\n", svc.Command)
			answer, err := prompter.Ask("New command (empty to keep)")
			if err != nil {
				return inputError("update", err)
			}
			command := svc.Command
			if answer != "" {
				command = answer
			}

			fmt.Fprintf(out, "Current working directory: %s\n", svc.WorkDir)
			workDir, err := prompter.AskValidated("New working directory (empty to keep)",
				func(answer string) (string, error) {
					if answer == "" {
						return svc.WorkDir, nil
					}
					return resolveWorkDir(answer)
				})
			if err != nil {
				return inputError("update", err)
			}

			if command == svc.Command && workDir == svc.WorkDir {
				fmt.Fprintf(out, "Service '%s' unchanged.\n", name)
				return nil
			}

			s.cfg.Services[name] = config.Service{Command: command, WorkDir: workDir}
			if err := s.cfg.Save(s.path); err != nil {
				return err
			}
			fmt.Fprintf(out, "Service '%s' updated.\n", name)

			// A process that is already up cannot adopt a new command, so set
			// the expectation instead of leaving it implicit.
			if status.Running {
				fmt.Fprintf(out,
					"Note: service '%s' is running (pid %d); the change takes effect on the next restart.\n",
					name, status.PID)
			}
			return nil
		},
	}
}
