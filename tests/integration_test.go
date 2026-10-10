// Package tests holds the end-to-end integration suite.
//
// Every test drives the real lsm binary in a temporary directory, so the whole
// path is exercised: cobra argument handling, config load and save, process
// launch, PID files and the exit codes documented in dev.md §9.1.
package tests

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// binPath is the lsm binary built once by TestMain.
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lsm-integration-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create a temporary directory: %v\n", err)
		os.Exit(1)
	}

	binPath = filepath.Join(dir, "lsm"+exeSuffix())
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = ".." // the module root
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "cannot build lsm: %v\n%s\n", err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// idleCommand is a command that keeps running long enough to be observed, using
// tools that exist on the platform (dev.md §10.2).
func idleCommand() string {
	if runtime.GOOS == "windows" {
		return "ping -n 300 127.0.0.1"
	}
	return "sleep 300"
}

// quotedIdleCommand also prints to stdout, so the test covers both a command
// with quoting and the fact that the output is discarded.
func quotedIdleCommand() string {
	if runtime.GOOS == "windows" {
		return `echo "a b" && ping -n 300 127.0.0.1`
	}
	return `echo "a b" && sleep 300`
}

// updatedIdleCommand is a second long-running command, distinct from
// idleCommand, so an update can be shown to have really changed the stored
// value (dev.md §5.9).
func updatedIdleCommand() string {
	if runtime.GOOS == "windows" {
		return "ping -n 240 127.0.0.1"
	}
	return "sleep 240"
}

// result is what a single lsm invocation produced.
type result struct {
	stdout string
	stderr string
	code   int
}

// lsm runs the binary under test with scripted standard input.
func lsm(t *testing.T, stdin string, args ...string) result {
	t.Helper()

	cmd := exec.Command(binPath, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("cannot run lsm %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// workspace prepares a config path and an existing working directory.
func workspace(t *testing.T, workDirName string) (cfgPath, workDir string) {
	t.Helper()
	dir := t.TempDir()
	workDir = filepath.Join(dir, workDirName)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "config.yaml"), workDir
}

// psStatus extracts the STATUS column of a service from `lsm ps` output.
func psStatus(t *testing.T, psOut, name string) string {
	t.Helper()
	for _, line := range strings.Split(psOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == name {
			return fields[1]
		}
	}
	t.Fatalf("no ps row for %q in:\n%s", name, psOut)
	return ""
}

// TestFullLifecycle walks the sequence documented in dev.md §10.2.
func TestFullLifecycle(t *testing.T) {
	cfgPath, workDir := workspace(t, "work")
	const name = "test-svc"

	// A fresh installation must show an empty table rather than an error.
	got := lsm(t, "", "ls", "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("ls on a fresh install: exit %d (stderr: %s)", got.code, got.stderr)
	}
	if !strings.HasPrefix(got.stdout, "NAME") {
		t.Fatalf("ls output has no header:\n%s", got.stdout)
	}

	// 1. add, with the answers injected through stdin.
	got = lsm(t, name+"\n"+idleCommand()+"\n"+workDir+"\n", "add", "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("add: exit %d (stderr: %s)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "Service '"+name+"' added.") {
		t.Fatalf("add output = %q", got.stdout)
	}

	// 2. ls confirms the entry.
	got = lsm(t, "", "ls", "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("ls: exit %d (stderr: %s)", got.code, got.stderr)
	}
	// The stored work_dir is normalised to forward slashes (dev.md §3.4), so
	// compare against that form rather than the OS one.
	if !strings.Contains(got.stdout, name) || !strings.Contains(got.stdout, filepath.ToSlash(workDir)) {
		t.Fatalf("ls does not show the new service:\n%s", got.stdout)
	}

	// 2b. update the command, keeping the working directory.
	got = lsm(t, updatedIdleCommand()+"\n\n", "update", name, "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("update: exit %d (stderr: %s)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "Service '"+name+"' updated.") {
		t.Fatalf("update output = %q", got.stdout)
	}

	got = lsm(t, "", "ls", "--config", cfgPath)
	if !strings.Contains(got.stdout, updatedIdleCommand()) {
		t.Fatalf("ls does not show the updated command:\n%s", got.stdout)
	}

	// 3. start, observe, stop, observe.
	got = lsm(t, "", "start", name, "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("start: exit %d (stderr: %s)", got.code, got.stderr)
	}

	// The updated command is what actually got started.
	pidFile := filepath.Join(filepath.Dir(cfgPath), "run", name+".pid")
	recorded, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("cannot read the pid file: %v", err)
	}
	if !strings.Contains(string(recorded), updatedIdleCommand()) {
		t.Fatalf("the pid file does not record the updated command:\n%s", recorded)
	}
	if !strings.HasPrefix(got.stdout, "service '"+name+"' started (pid ") {
		t.Fatalf("start output = %q", got.stdout)
	}

	got = lsm(t, "", "ps", "--config", cfgPath)
	if status := psStatus(t, got.stdout, name); status != "running" {
		t.Fatalf("ps status = %q, want running:\n%s", status, got.stdout)
	}

	got = lsm(t, "", "stop", name, "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("stop: exit %d (stderr: %s)", got.code, got.stderr)
	}
	if got.stdout != "service '"+name+"' stopped\n" {
		t.Fatalf("stop output = %q", got.stdout)
	}

	got = lsm(t, "", "ps", "--config", cfgPath)
	if status := psStatus(t, got.stdout, name); status != "stopped" {
		t.Fatalf("ps status = %q, want stopped:\n%s", status, got.stdout)
	}

	// 4. clean up.
	got = lsm(t, "y\n", "del", name, "--force", "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("del: exit %d (stderr: %s)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "Service '"+name+"' deleted.") {
		t.Fatalf("del output = %q", got.stdout)
	}

	got = lsm(t, "", "ls", "--config", cfgPath)
	if strings.Contains(got.stdout, name) {
		t.Fatalf("the service is still listed:\n%s", got.stdout)
	}
}

// TestWorkDirWithSpacesAndQuotedCommand covers the manual acceptance list of
// dev.md §10.4.
func TestWorkDirWithSpacesAndQuotedCommand(t *testing.T) {
	cfgPath, workDir := workspace(t, "work dir with spaces")
	const name = "spaced"

	if got := lsm(t, name+"\n"+quotedIdleCommand()+"\n"+workDir+"\n", "add", "--config", cfgPath); got.code != 0 {
		t.Fatalf("add: exit %d (stderr: %s)", got.code, got.stderr)
	}

	got := lsm(t, "", "start", name, "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("start: exit %d (stderr: %s)", got.code, got.stderr)
	}
	t.Cleanup(func() { _ = lsm(t, "", "stop", name, "--config", cfgPath) })

	// The service prints to stdout; lsm must not let it through (dev.md §1.1).
	if strings.Contains(got.stdout+got.stderr, "a b") {
		t.Fatalf("service output leaked into the lsm streams: %q / %q", got.stdout, got.stderr)
	}

	if got := lsm(t, "", "stop", name, "--config", cfgPath); got.code != 0 || !strings.Contains(got.stdout, "stopped") {
		t.Fatalf("stop: exit %d, stdout %q (stderr: %s)", got.code, got.stdout, got.stderr)
	}
}

// TestExitCodes covers the exit code contract of dev.md §9.1 end to end.
func TestExitCodes(t *testing.T) {
	cfgPath, workDir := workspace(t, "work")
	const name = "codes"

	if got := lsm(t, name+"\n"+idleCommand()+"\n"+workDir+"\n", "add", "--config", cfgPath); got.code != 0 {
		t.Fatalf("add: exit %d (stderr: %s)", got.code, got.stderr)
	}

	if got := lsm(t, "", "start", "ghost", "--config", cfgPath); got.code != 3 {
		t.Errorf("start unknown service: exit %d, want 3", got.code)
	}
	if got := lsm(t, "", "stop", "ghost", "--config", cfgPath); got.code != 3 {
		t.Errorf("stop unknown service: exit %d, want 3", got.code)
	}
	if got := lsm(t, "", "ls", "--bogus"); got.code != 2 {
		t.Errorf("unknown flag: exit %d, want 2", got.code)
	}
	if got := lsm(t, "", "bogus"); got.code != 2 {
		t.Errorf("unknown command: exit %d, want 2", got.code)
	}

	broken := filepath.Join(filepath.Dir(cfgPath), "broken.yaml")
	if err := os.WriteFile(broken, []byte("services:\n  web:\n    command: \"x\"\n   oops: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lsm(t, "", "ls", "--config", broken); got.code != 6 {
		t.Errorf("broken config: exit %d, want 6", got.code)
	}

	if got := lsm(t, "", "start", name, "--config", cfgPath); got.code != 0 {
		t.Fatalf("start: exit %d (stderr: %s)", got.code, got.stderr)
	}
	t.Cleanup(func() { _ = lsm(t, "", "stop", name, "--config", cfgPath) })

	if got := lsm(t, "", "start", name, "--config", cfgPath); got.code != 4 {
		t.Errorf("start while running: exit %d, want 4", got.code)
	}
	if got := lsm(t, "", "stop", name, "--config", cfgPath); got.code != 0 {
		t.Fatalf("stop: exit %d (stderr: %s)", got.code, got.stderr)
	}
	// Stopping something that is already stopped is a success (dev.md §5.3).
	if got := lsm(t, "", "stop", name, "--config", cfgPath); got.code != 0 {
		t.Errorf("stop while stopped: exit %d, want 0", got.code)
	}
}

// TestStalePIDFileIsSelfHealing covers dev.md §7.3 through the real binary.
func TestStalePIDFileIsSelfHealing(t *testing.T) {
	cfgPath, workDir := workspace(t, "work")
	const name = "stale"

	if got := lsm(t, name+"\n"+idleCommand()+"\n"+workDir+"\n", "add", "--config", cfgPath); got.code != 0 {
		t.Fatalf("add: exit %d (stderr: %s)", got.code, got.stderr)
	}

	pidPath := filepath.Join(filepath.Dir(cfgPath), "run", name+".pid")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o700); err != nil {
		t.Fatal(err)
	}
	stale := fmt.Sprintf("{\n  \"pid\": 4294967294,\n  \"started_at\": 1,\n  \"work_dir\": %q,\n  \"command\": %q\n}\n",
		workDir, idleCommand())
	if err := os.WriteFile(pidPath, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	// ps reports it as stopped and leaves the file alone.
	got := lsm(t, "", "ps", "--config", cfgPath)
	if status := psStatus(t, got.stdout, name); status != "stopped" {
		t.Fatalf("ps status = %q, want stopped:\n%s", status, got.stdout)
	}
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("ps must not remove the file: %v", err)
	}

	// stop reports "not running" and cleans the file up.
	got = lsm(t, "", "stop", name, "--config", cfgPath)
	if got.code != 0 {
		t.Fatalf("stop: exit %d (stderr: %s)", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "is not running") {
		t.Fatalf("stop output = %q", got.stdout)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("the stale pid file must be gone, got %v", err)
	}

	// start on a stale PID file must work.
	if got := lsm(t, "", "start", name, "--config", cfgPath); got.code != 0 {
		t.Fatalf("start: exit %d (stderr: %s)", got.code, got.stderr)
	}
	t.Cleanup(func() { _ = lsm(t, "", "stop", name, "--config", cfgPath) })
}

// TestRestartKeepsTheServiceRunning covers dev.md §5.4 end to end.
func TestRestartKeepsTheServiceRunning(t *testing.T) {
	cfgPath, workDir := workspace(t, "work")
	const name = "restarted"

	if got := lsm(t, name+"\n"+idleCommand()+"\n"+workDir+"\n", "add", "--config", cfgPath); got.code != 0 {
		t.Fatalf("add: exit %d (stderr: %s)", got.code, got.stderr)
	}
	t.Cleanup(func() { _ = lsm(t, "", "stop", name, "--config", cfgPath) })

	if got := lsm(t, "", "restart", name, "--config", cfgPath); got.code != 0 {
		t.Fatalf("restart of a stopped service: exit %d (stderr: %s)", got.code, got.stderr)
	}
	if got := lsm(t, "", "restart", name, "--config", cfgPath); got.code != 0 {
		t.Fatalf("restart of a running service: exit %d (stderr: %s)", got.code, got.stderr)
	}

	got := lsm(t, "", "ps", "--config", cfgPath)
	if status := psStatus(t, got.stdout, name); status != "running" {
		t.Fatalf("ps status = %q, want running:\n%s", status, got.stdout)
	}
}
