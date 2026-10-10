//go:build !windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
)

const (
	// shellPath interprets a service command. The command string is handed over
	// as a single argv entry, so lsm never parses or rewrites it: quoting,
	// pipes, redirections and variable expansion are the shell's business
	// (dev.md §6.1).
	shellPath = "/bin/sh"

	// startupGrace is how long Launch waits before concluding that the service
	// died right after start (dev.md §6.3).
	startupGrace = 150 * time.Millisecond

	// stopTimeout is how long Stop waits after SIGTERM before escalating to
	// SIGKILL (dev.md §6.4).
	stopTimeout = 10 * time.Second

	// stopPollInterval is the polling granularity while waiting for the process
	// group to disappear.
	stopPollInterval = 50 * time.Millisecond

	// killGrace is the short additional wait after SIGKILL, so that a restart
	// does not race with a process that is still being torn down.
	killGrace = time.Second
)

// Launch starts svc detached from the terminal and returns its PID.
//
// The child becomes the leader of a new session, so it survives lsm exiting and
// can later be signalled as a whole process group. The standard streams are
// deliberately left nil, which wires them to /dev/null and keeps the service
// from polluting the lsm output (dev.md §6.1).
func Launch(svc config.Service) (int, error) {
	cmd := exec.Command(shellPath, "-c", svc.Command)
	cmd.Dir = svc.WorkDir
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

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

// startupFailure reports that the service died inside the startup grace window.
// Its message is only the shell's own reason, so that a caller wrapping it can
// phrase the sentence without repeating itself, while errors.Is still matches
// ErrExitedImmediately.
type startupFailure struct {
	reason string
}

func (e *startupFailure) Error() string { return e.reason }

func (e *startupFailure) Is(target error) bool { return target == ErrExitedImmediately }

// IsAlive reports whether the recorded process still exists (dev.md §6.6).
func IsAlive(info PIDInfo) bool {
	if info.PID <= 0 {
		return false
	}
	err := syscall.Kill(info.PID, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Stop terminates the process group recorded in info (dev.md §6.4).
//
// It sends SIGTERM to the whole group, waits up to stopTimeout and escalates to
// SIGKILL. ErrNotRunning is returned when the process is already gone or when
// the recorded PID no longer looks like a process lsm started: a recycled PID
// must never be signalled.
func Stop(info PIDInfo) error {
	if info.PID <= 0 {
		return errors.New("invalid pid")
	}
	pid := info.PID

	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return ErrNotRunning
	}
	// Setsid makes the launched shell its own process group leader, so a PGID
	// that differs from the PID means this PID now belongs to something else.
	if pgid != pid {
		return ErrNotRunning
	}

	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("cannot send SIGTERM to process group %d: %w", pgid, err)
	}
	if waitGone(pgid, stopTimeout) {
		return nil
	}

	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("cannot send SIGKILL to process group %d: %w", pgid, err)
	}
	waitGone(pgid, killGrace)
	return nil
}

// groupAlive reports whether the process group still has a member.
func groupAlive(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// waitGone polls until the process group no longer exists and reports whether
// it disappeared within timeout.
func waitGone(pgid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !groupAlive(pgid) {
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
