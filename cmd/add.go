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
		Use:   "add",
		Short: "Add a service interactively",
		Long: `Add a service by answering three questions.

The name must be unique and may only contain letters, digits, '-' and '_'. The
command is stored verbatim and later handed to the system shell. The working
directory must exist; a relative answer is resolved against the current
directory.`,
		Args: argsNone(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newSession()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			prompter := ui.NewPrompter(cmd.InOrStdin(), out)

			name, err := prompter.AskValidated("Service name", func(answer string) (string, error) {
				return validateNewName(s.cfg, answer)
			})
			if err != nil {
				return inputError("add", err)
			}

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

// validateNewName applies the dev.md §5.7 name rules: non-empty, unique and
// within the allowed charset.
func validateNewName(cfg *config.Config, answer string) (string, error) {
	switch {
	case answer == "":
		return "", errors.New("the service name must not be empty")
	case !config.ValidName(answer):
		return "", fmt.Errorf("invalid service name '%s': must match %s and be 1-%d characters",
			answer, config.NamePattern, config.MaxNameLen)
	}
	if _, exists := cfg.Get(answer); exists {
		return "", fmt.Errorf("service '%s' already exists", answer)
	}
	return answer, nil
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
