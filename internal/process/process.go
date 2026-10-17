// Package process handles the platform specific side of service management:
// launching a detached process, stopping it, checking whether it is alive, and
// the PID files that tie those operations together.
//
// Everything in this file is platform independent; Launch, Stop and IsAlive are
// implemented separately for Unix (process_unix.go) and Windows
// (process_windows.go).
package process

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
)

// ErrNotRunning reports that the recorded process no longer exists, either
// because it exited on its own or because the recorded PID was recycled by an
// unrelated process.
var ErrNotRunning = errors.New("process is not running")

// ErrExitedImmediately reports that the service died inside the startup grace
// window. Because the command always runs through a shell, a missing binary or
// a broken command line can only be observed this way (dev.md §6.3).
var ErrExitedImmediately = errors.New("exited immediately after start")

const (
	// startupGrace is how long Launch waits before concluding that the service
	// died right after start (dev.md §6.3).
	startupGrace = 150 * time.Millisecond

	// stopTimeout is how long Stop waits for a graceful exit before escalating.
	// Unix sends SIGTERM first; Windows terminates forcefully right away
	// (dev.md §6.4, §6.5).
	stopTimeout = 10 * time.Second

	// stopPollInterval is the polling granularity while waiting for a process to
	// disappear.
	stopPollInterval = 50 * time.Millisecond

	// killGrace is the short additional wait after the forceful kill, so that a
	// restart does not race with a process that is still being torn down.
	killGrace = time.Second
)

// startupFailure reports that the service died inside the startup grace window.
// Its message is only the shell's own reason, so that a caller wrapping it can
// phrase the sentence without repeating itself, while errors.Is still matches
// ErrExitedImmediately.
type startupFailure struct {
	reason string
}

func (e *startupFailure) Error() string { return e.reason }

func (e *startupFailure) Is(target error) bool { return target == ErrExitedImmediately }

// awaitStartup starts cmd and waits out the startup grace period. It returns the
// new PID when the service is still alive, or a startupFailure when the shell
// exited at once.
func awaitStartup(cmd *exec.Cmd) (int, error) {
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("cannot start process: %w", err)
	}
	pid := cmd.Process.Pid

	// The shell itself always exists, so a missing binary or a broken command
	// line only shows up as a shell that exits immediately (dev.md §6.3).
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return 0, &startupFailure{reason: exitReason(err)}
	case <-time.After(startupGrace):
		return pid, nil
	}
}

// Run executes svc in the foreground, connected to the given standard streams,
// and returns the exit status of the command.
//
// Unlike Launch it does not detach the process, does not discard the streams
// and does not wait out the startup grace period: the caller sees exactly what
// running the command in a terminal would show. A non-nil error means the
// command could not be launched at all; a command that ran and failed is
// reported through the status with a nil error, so the caller can mirror it.
func Run(svc config.Service, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := shellCommand(svc)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitStatus(exitErr), nil
		}
		return 0, fmt.Errorf("cannot run command: %w", err)
	}
	return 0, nil
}

// waitUntil polls check until it reports true or the timeout expires.
func waitUntil(check func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if check() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(stopPollInterval)
	}
}

// exitReason renders why an already finished process stopped.
func exitReason(err error) string {
	if err == nil {
		return "the command finished immediately"
	}
	return err.Error()
}
