// Package cmd implements the lsm command line interface: cobra command
// definitions, user interaction and exit code mapping.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/version"
	"github.com/spf13/cobra"
)

// configFlag backs the global --config option.
var configFlag string

// resolveConfigPath applies the dev.md §4.2 lookup precedence.
func resolveConfigPath() (string, error) {
	return config.ResolvePath(configFlag)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "lsm",
		Short: "Manage long-running background services from a YAML config",
		Long: `lsm (Lsm iS Manager) starts, stops and inspects the background services
registered in a YAML configuration file.

Every service is a name, a shell command and a working directory. Services are
launched detached from the terminal, so they keep running after lsm exits.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Setting Version makes cobra provide --version; the template is
		// replaced so that `lsm --version` and `lsm version` print the same
		// line (dev.md §5.9).
		Version: version.Version,
	}
	root.SetVersionTemplate(version.String() + "\n")
	root.PersistentFlags().StringVar(&configFlag, "config", "",
		"config file path (default $LSM_CONFIG, then the platform config directory)")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return errs.Classify(errs.Usage, err)
	})
	root.AddCommand(
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newRunCmd(),
		newPsCmd(),
		newLsCmd(),
		newUpdateCmd(),
		newAddCmd(),
		newDelCmd(),
		newVersionCmd(),
	)
	return root
}

// Execute runs the CLI against the real process streams and returns the exit
// code (dev.md §9.1).
func Execute() int {
	return execute(newRootCmd(), os.Stdout, os.Stderr)
}

// execute is the testable core of Execute: it wires the command tree to the
// given streams, prints any error and maps it to an exit code.
func execute(root *cobra.Command, stdout, stderr io.Writer) int {
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	if err == nil {
		return int(errs.OK)
	}
	// `lsm run` mirrors the exit status of the command it started, including
	// the cases where that status collides with lsm's own codes. Nothing is
	// printed here: the command's own output already went to the terminal.
	var exitStatus *errs.ExitStatus
	if errors.As(err, &exitStatus) {
		return exitStatus.Status
	}
	if isUsageError(err) {
		err = errs.Classify(errs.Usage, err)
	}
	// Errors go to stderr as a single "lsm: <message>" line, never a stack.
	fmt.Fprintf(stderr, "lsm: %v\n", err)
	debugCause(stderr, err)
	return int(errs.CodeOf(err))
}

// isUsageError reports whether cobra failed to understand the command line
// itself rather than the operation it was asked to perform. Flag and argument
// failures are already tagged where they are raised; this covers the errors
// cobra produces internally.
func isUsageError(err error) bool {
	for _, prefix := range []string{"unknown command", "unknown flag", "unknown shorthand flag"} {
		if strings.HasPrefix(err.Error(), prefix) {
			return true
		}
	}
	return false
}

// debugCause prints the unwrapped error chain when LSM_DEBUG=1 (dev.md §9.2).
func debugCause(w io.Writer, err error) {
	if os.Getenv("LSM_DEBUG") != "1" {
		return
	}
	for cause := errors.Unwrap(err); cause != nil; cause = errors.Unwrap(cause) {
		fmt.Fprintf(w, "lsm: debug: %v\n", cause)
	}
}
