//go:build !windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/LiZeC123/lsm/internal/config"
)

// shellPath interprets a service command. The command string is handed over as
// a single argv entry, so lsm never parses or rewrites it: quoting, pipes,
// redirections and variable expansion are the shell's business (dev.md §6.1).
const shellPath = "/bin/sh"

// shellCommand builds the shell invocation that interprets a service command.
// The command string is handed over as a single argv entry, so lsm never parses
// or rewrites it: quoting, pipes, redirections and variable expansion are the
// shell's business (dev.md §6.1).
func shellCommand(svc config.Service) *exec.Cmd {
	cmd := exec.Command(shellPath, "-c", svc.Command)
	cmd.Dir = svc.WorkDir
	return cmd
}

// Launch starts svc detached from the terminal and returns its PID.
//
// The child becomes the leader of a new session, so it survives lsm exiting and
// can later be signalled as a whole process group. The standard streams are
// deliberately left nil, which wires them to /dev/null and keeps the service
// from polluting the lsm output (dev.md §6.1).
func Launch(svc config.Service) (int, error) {
	cmd := shellCommand(svc)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	return awaitStartup(cmd)
}

// exitStatus renders the status of a finished command the way a shell reports
// it: a process killed by a signal has no exit code of its own on Unix, so it
// is mapped to 128+signal (dev.md §5.11).
func exitStatus(err *exec.ExitError) int {
	if code := err.ExitCode(); code >= 0 {
		return code
	}
	if status, ok := err.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return 1
}

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
	if waitUntil(func() bool { return !groupAlive(pgid) }, stopTimeout) {
		return nil
	}

	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("cannot send SIGKILL to process group %d: %w", pgid, err)
	}
	waitUntil(func() bool { return !groupAlive(pgid) }, killGrace)
	return nil
}

// groupAlive reports whether the process group still has a member.
func groupAlive(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
