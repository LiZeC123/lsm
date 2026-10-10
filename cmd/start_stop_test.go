package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/errs"
)

// idleCommand keeps running for long enough to be observed, on every platform.
func idleCommand() string {
	if runtime.GOOS == "windows" {
		return "ping -n 30 127.0.0.1"
	}
	return "sleep 30"
}

// serviceFixture writes a config file holding one service with an existing
// work_dir and returns the config path.
func serviceFixture(t *testing.T, name, command string) string {
	t.Helper()
	dir := t.TempDir()
	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	doc := "services:\n  " + name + ":\n    command: " + command + "\n    work_dir: " + workDir + "\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStartStopLifecycleThroughTheCLI(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())
	t.Cleanup(func() { _, _, _ = cli(t, "stop", "web", "--config", cfgPath) })

	stdout, stderr, code := cli(t, "start", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("start exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.HasPrefix(stdout, "service 'web' started (pid ") {
		t.Fatalf("start stdout = %q", stdout)
	}

	stdout, stderr, code = cli(t, "start", "web", "--config", cfgPath)
	if code != int(errs.AlreadyRunning) {
		t.Fatalf("second start exit = %d, want %d (stdout: %s)", code, errs.AlreadyRunning, stdout)
	}
	if !strings.HasPrefix(stderr, "lsm: service 'web' is already running (pid ") {
		t.Fatalf("stderr = %q", stderr)
	}

	stdout, stderr, code = cli(t, "stop", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("stop exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != "service 'web' stopped\n" {
		t.Fatalf("stop stdout = %q", stdout)
	}

	stdout, _, code = cli(t, "stop", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("second stop exit = %d, want 0", code)
	}
	if stdout != "service 'web' is not running\n" {
		t.Fatalf("second stop stdout = %q", stdout)
	}
}

func TestStartStopUnknownServiceExitsWithThree(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	for _, verb := range []string{"start", "stop"} {
		t.Run(verb, func(t *testing.T) {
			stdout, stderr, code := cli(t, verb, "ghost", "--config", cfgPath)
			if code != int(errs.NotFound) {
				t.Fatalf("exit = %d, want %d", code, errs.NotFound)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "service 'ghost' not found") {
				t.Errorf("stderr = %q", stderr)
			}
		})
	}
}

func TestStartWithAMissingWorkDirExitsWithEight(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	doc := "services:\n  web:\n    command: " + idleCommand() + "\n    work_dir: " + filepath.Join(dir, "gone") + "\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := cli(t, "start", "web", "--config", path)
	if code != int(errs.ProcessOp) {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, errs.ProcessOp, stderr)
	}
	if !strings.Contains(stderr, "not accessible") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestStartRequiresExactlyOneServiceName(t *testing.T) {
	cfgPath := serviceFixture(t, "web", idleCommand())

	for _, args := range [][]string{
		{"start", "--config", cfgPath},
		{"start", "a", "b", "--config", cfgPath},
		{"stop", "--config", cfgPath},
	} {
		_, stderr, code := cli(t, args...)
		if code != int(errs.Usage) {
			t.Errorf("%v exit = %d, want %d (stderr: %s)", args, code, errs.Usage, stderr)
		}
	}
}
