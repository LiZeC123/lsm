package cmd

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/ui"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Add a service interactively",
		Long: `Add a service whose name is given on the command line.

The command and the working directory are asked interactively. The name must be
unique and may only contain letters, digits, '-' and '_'. The command is stored
verbatim and later handed to the system shell. The working directory must exist;
a relative answer is resolved against the current directory.`,
		Args: argsExact(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSession()
			if err != nil {
				return err
			}
			name := args[0]
			if err := checkNewName(s.cfg, name); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			prompter := ui.NewPrompter(cmd.InOrStdin(), out)

			command, err := prompter.AskValidated("Command", func(answer string) (string, error) {
				if answer == "" {
					return "", errors.New("the command must not be empty")
				}
				return answer, nil
			})
			if err != nil {
				return inputError("add", err)
			}

			workDir, err := prompter.AskValidated("Working directory", resolveWorkDir)
			if err != nil {
				return inputError("add", err)
			}

			s.cfg.Services[name] = config.Service{Command: command, WorkDir: workDir}
			if err := s.cfg.Save(s.path); err != nil {
				return err
			}
			fmt.Fprintf(out, "Service '%s' added.\n", name)
			return nil
		},
	}
}

// checkNewName applies the dev.md §5.7 name rules to the name given on the
// command line: non-empty, within the allowed charset and not already used.
// Invalid names are a usage error, while an existing name is a conflict with
// the current configuration and gets the generic exit code.
func checkNewName(cfg *config.Config, name string) error {
	switch {
	case name == "":
		return errs.New(errs.Usage, "the service name must not be empty")
	case !config.ValidName(name):
		return errs.New(errs.Usage, "invalid service name '%s': must match %s and be 1-%d characters",
			name, config.NamePattern, config.MaxNameLen)
	}
	if _, exists := cfg.Get(name); exists {
		return errs.New(errs.General, "service '%s' already exists", name)
	}
	return nil
}

// resolveWorkDir turns an interactive answer into the value stored on disk.
// Relative answers are resolved against the current directory and the result is
// normalised to forward slashes (dev.md §3.4).
func resolveWorkDir(answer string) (string, error) {
	if answer == "" {
		return "", errors.New("the working directory must not be empty")
	}
	abs, err := filepath.Abs(answer)
	if err != nil {
		return "", fmt.Errorf("cannot resolve '%s': %v", answer, err)
	}
	if err := config.CheckWorkDir(abs); err != nil {
		return "", err
	}
	return filepath.ToSlash(abs), nil
}

// inputError turns an exhausted input stream into a message a user can act on.
func inputError(verb string, err error) error {
	if errors.Is(err, ui.ErrInputClosed) {
		return errs.New(errs.General, "%s: input ended before the service was complete", verb)
	}
	return err
}
