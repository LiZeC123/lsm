package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
)

// copyStdinCommand copies its standard input to standard output, using the
// tools the platform shell provides.
func copyStdinCommand() string {
	if runtime.GOOS == "windows" {
		return "more"
	}
	return "cat"
}

func TestRunShowsTheCommandOutput(t *testing.T) {
	cfgPath := serviceFixture(t, "web", "echo hello-run")

	stdout, stderr, code := cli(t, "run", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "hello-run") {
		t.Fatalf("the command output must reach stdout: %q", stdout)
	}
	// lsm adds nothing of its own to a foreground run.
	if strings.Contains(stdout, "lsm:") {
		t.Fatalf("stdout contains lsm output: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	// A foreground run is not a managed background service.
	if _, err := os.Stat(config.PIDPath(cfgPath, "web")); !os.IsNotExist(err) {
		t.Fatalf("run must not write a PID file, got %v", err)
	}
}

func TestRunUsesTheConfiguredWorkDir(t *testing.T) {
	cfgPath, workDir := servicePaths(t)
	cfg := &config.Config{Services: map[string]config.Service{
		"web": {Command: "echo hello > marker.txt", WorkDir: filepath.ToSlash(workDir)},
	}}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, stderr, code := cli(t, "run", "web", "--config", cfgPath); code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(workDir, "marker.txt")); err != nil {
		t.Fatalf("the command did not run in the configured work_dir: %v", err)
	}
}

func TestRunConnectsStdin(t *testing.T) {
	cfgPath := serviceFixture(t, "web", copyStdinCommand())

	stdout, stderr, code := run(t, "hello-stdin\n", "run", "web", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "hello-stdin") {
		t.Fatalf("stdin was not passed through: %q", stdout)
	}
}

// The command's own exit status is what lsm exits with, so a foreground run is
// as scriptable as running the command directly.
func TestRunPropagatesTheCommandExitStatus(t *testing.T) {
	cfgPath := serviceFixture(t, "web", "exit 7")

	stdout, stderr, code := cli(t, "run", "web", "--config", cfgPath)
	if code != 7 {
		t.Fatalf("exit = %d, want the command's status 7 (stderr: %s)", code, stderr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("a failing command must not produce lsm output: stdout %q, stderr %q", stdout, stderr)
	}
}

func TestRunUnknownServiceExitsWithThree(t *testing.T) {
	cfgPath := serviceFixture(t, "web", "echo hi")

	stdout, stderr, code := cli(t, "run", "ghost", "--config", cfgPath)
	if code != int(errs.NotFound) {
		t.Fatalf("exit = %d, want %d", code, errs.NotFound)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "service 'ghost' not found") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunWithAMissingWorkDirExitsWithEight(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{Services: map[string]config.Service{
		"web": {Command: "echo hi", WorkDir: filepath.ToSlash(filepath.Join(dir, "gone"))},
	}}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, stderr, code := cli(t, "run", "web", "--config", cfgPath)
	if code != int(errs.ProcessOp) {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, errs.ProcessOp, stderr)
	}
	if !strings.Contains(stderr, "not accessible") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestRunRequiresExactlyOneServiceName(t *testing.T) {
	cfgPath := serviceFixture(t, "web", "echo hi")

	for _, args := range [][]string{
		{"run", "--config", cfgPath},
		{"run", "a", "b", "--config", cfgPath},
	} {
		if _, stderr, code := cli(t, args...); code != int(errs.Usage) {
			t.Errorf("%v exit = %d, want %d (stderr: %s)", args, code, errs.Usage, stderr)
		}
	}
}
