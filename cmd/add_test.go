package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
)

// addFixture prepares a config path and an existing working directory.
func addFixture(t *testing.T) (cfgPath, workDir string) {
	t.Helper()
	dir := t.TempDir()
	workDir = filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "config.yaml"), workDir
}

func TestAddInteractively(t *testing.T) {
	cfgPath, workDir := addFixture(t)
	stdin := "web\nnode ./server.js --port 3000\n" + workDir + "\n"

	stdout, stderr, code := run(t, stdin, "add", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	// The three questions in the order documented in dev.md §5.7, then the
	// confirmation line.
	for _, want := range []string{"Service name: ", "Command: ", "Working directory: ", "Service 'web' added.\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	svc, ok := cfg.Get("web")
	if !ok {
		t.Fatal("the service was not persisted")
	}
	if svc.Command != "node ./server.js --port 3000" {
		t.Errorf("command = %q", svc.Command)
	}
	if svc.WorkDir != filepath.ToSlash(workDir) {
		t.Errorf("work_dir = %q, want %q", svc.WorkDir, workDir)
	}
}

func TestAddRejectsInvalidAnswersAndAsksAgain(t *testing.T) {
	cfgPath, workDir := addFixture(t)
	stdin := strings.Join([]string{
		"bad name!",            // invalid name
		"",                     // still empty
		"web",                  // accepted
		"",                     // empty command
		"node server.js",       // accepted
		"/nope/does-not-exist", // missing directory
		workDir,                // accepted
	}, "\n") + "\n"

	stdout, stderr, code := run(t, stdin, "add", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Count(stdout, "Service name: ") != 3 {
		t.Errorf("the name question should have been asked three times:\n%s", stdout)
	}
	if strings.Count(stdout, "Command: ") != 2 {
		t.Errorf("the command question should have been asked twice:\n%s", stdout)
	}
	if strings.Count(stdout, "Working directory: ") != 2 {
		t.Errorf("the directory question should have been asked twice:\n%s", stdout)
	}
	for _, want := range []string{"must not be empty", "invalid service name", "is not accessible"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing the %q hint:\n%s", want, stdout)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if svc, _ := cfg.Get("web"); svc.Command != "node server.js" {
		t.Fatalf("service = %+v", svc)
	}
}

func TestAddRejectsADuplicateName(t *testing.T) {
	cfgPath, workDir := addFixture(t)
	// Pre-populate the config with a service that the interactive session will
	// then try to reuse.
	existing := &config.Config{Services: map[string]config.Service{
		"web": {Command: "run", WorkDir: filepath.ToSlash(workDir)},
	}}
	if err := existing.Save(cfgPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	stdin := strings.Join([]string{"web", "web2", "run2", workDir}, "\n") + "\n"
	stdout, stderr, code := run(t, stdin, "add", "--config", cfgPath)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "service 'web' already exists") {
		t.Fatalf("stdout is missing the duplicate hint:\n%s", stdout)
	}
	if n := strings.Count(stdout, "Service name: "); n != 2 {
		t.Fatalf("the name question should have been asked twice, got %d:\n%s", n, stdout)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := cfg.Get("web"); !ok {
		t.Error("the original service must be kept")
	}
	if svc, ok := cfg.Get("web2"); !ok || svc.Command != "run2" {
		t.Fatalf("the new service was not added: %+v", svc)
	}
}

func TestAddWithClosedInputFails(t *testing.T) {
	cfgPath, _ := addFixture(t)

	stdout, stderr, code := run(t, "", "add", "--config", cfgPath)
	if code != int(errs.General) {
		t.Fatalf("exit = %d, want %d", code, errs.General)
	}
	if !strings.Contains(stderr, "input ended") {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stdout, "added") {
		t.Fatalf("nothing must be added, got %q", stdout)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("no config file must be written, got %v", err)
	}
}

func TestAddRejectsStrayArguments(t *testing.T) {
	cfgPath, _ := addFixture(t)

	if _, stderr, code := run(t, "", "add", "extra", "--config", cfgPath); code != int(errs.Usage) {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, errs.Usage, stderr)
	}
}

func TestResolveWorkDir(t *testing.T) {
	workDir := t.TempDir()

	got, err := resolveWorkDir(workDir)
	if err != nil {
		t.Fatalf("resolveWorkDir: %v", err)
	}
	if got != filepath.ToSlash(workDir) {
		t.Fatalf("got %q, want %q", got, workDir)
	}

	// "." must resolve to the current directory, which is how a relative answer
	// is turned into a storable absolute path.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got, err = resolveWorkDir(".")
	if err != nil {
		t.Fatalf("resolveWorkDir(.): %v", err)
	}
	if got != filepath.ToSlash(cwd) {
		t.Fatalf("got %q, want %q", got, cwd)
	}

	for _, bad := range []string{"", filepath.Join(workDir, "gone")} {
		if _, err := resolveWorkDir(bad); err == nil {
			t.Errorf("resolveWorkDir(%q) must fail", bad)
		}
	}
}
