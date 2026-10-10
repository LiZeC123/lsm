//go:build !windows

package process

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
)

// waitGoneFor polls until pid disappears, so the tests do not race with the
// kernel reaping a just-killed process.
func waitGoneFor(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pid %d did not disappear within %v", pid, timeout)
}

func TestLaunchProducesADetachedProcess(t *testing.T) {
	svc := config.Service{Command: "sleep 30", WorkDir: t.TempDir()}
	pid, err := Launch(svc)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })

	info := PIDInfo{PID: pid}
	if !IsAlive(info) {
		t.Fatal("the launched process should be alive")
	}

	// Setsid must have made the child its own process group leader, which is
	// what lets Stop signal the whole tree.
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatalf("Getpgid: %v", err)
	}
	if pgid != pid {
		t.Fatalf("pgid = %d, want the pid %d", pgid, pid)
	}

	if err := Stop(info); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if IsAlive(info) {
		t.Fatal("the process should be gone after Stop")
	}
}

func TestStopKillsTheWholeProcessGroup(t *testing.T) {
	dir := t.TempDir()
	childPIDFile := dir + "/child.pid"
	svc := config.Service{
		Command: `sleep 30 & echo $! > ` + childPIDFile + `; sleep 30`,
		WorkDir: dir,
	}

	pid, err := Launch(svc)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })

	child := readPID(t, childPIDFile)
	if !IsAlive(PIDInfo{PID: child}) {
		t.Fatal("the background child should be running")
	}

	if err := Stop(PIDInfo{PID: pid}); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if IsAlive(PIDInfo{PID: child}) {
		t.Fatal("the child must be terminated together with the service")
	}
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no pid appeared in %s", path)
	return 0
}

func TestLaunchReportsACommandThatCannotRun(t *testing.T) {
	svc := config.Service{Command: "definitely-not-a-real-binary --flag", WorkDir: t.TempDir()}
	pid, err := Launch(svc)
	if err == nil {
		t.Fatalf("Launch must fail, got pid %d", pid)
	}
	if !errors.Is(err, ErrExitedImmediately) {
		t.Fatalf("error %v does not match ErrExitedImmediately", err)
	}
	// The reason the shell reported has to reach the user, but the message must
	// not repeat the sentence the service layer adds around it.
	if got, want := err.Error(), "exit status 127"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestStopReportsAnAlreadyDeadProcess(t *testing.T) {
	svc := config.Service{Command: "sleep 30", WorkDir: t.TempDir()}
	pid, err := Launch(svc)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	waitGoneFor(t, pid, 2*time.Second)

	if err := Stop(PIDInfo{PID: pid}); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop = %v, want ErrNotRunning", err)
	}
}

// A recycled PID must never be signalled. A process lsm did not start is not a
// process group leader, which is exactly what the guard keys on.
func TestStopRefusesToSignalAForeignProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	if pgid, err := syscall.Getpgid(pid); err != nil || pgid == pid {
		t.Skipf("the helper process unexpectedly is a group leader (pgid %d, err %v)", pgid, err)
	}

	if err := Stop(PIDInfo{PID: pid}); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop = %v, want ErrNotRunning", err)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("the foreign process was killed: %v", err)
	}
}

func TestIsAliveRejectsNonPositivePIDs(t *testing.T) {
	for _, pid := range []int{0, -1} {
		if IsAlive(PIDInfo{PID: pid}) {
			t.Errorf("IsAlive(%d) = true, want false", pid)
		}
	}
}

func TestStopRejectsAnInvalidPID(t *testing.T) {
	if err := Stop(PIDInfo{PID: 0}); err == nil {
		t.Fatal("Stop must reject pid 0")
	}
}
