package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/errs"
)

// cli runs the whole command tree in process with an empty standard input and
// reports what the user would see on stdout, stderr and as the process exit
// code.
func cli(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return run(t, "", args...)
}

// run is cli with a scripted standard input, for the interactive commands.
func run(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	root := newRootCmd()
	root.SetArgs(args)
	root.SetIn(strings.NewReader(stdin))
	code = execute(root, &out, &errOut)
	return out.String(), errOut.String(), code
}

// configFixture writes a config file with two services and returns its path.
func configFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	doc := "services:\n" +
		"  web:\n    command: node ./server.js\n    work_dir: " + workDir + "\n" +
		"  api:\n    command: ./bin/api\n    work_dir: " + workDir + "\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNoArgsPrintsHelp(t *testing.T) {
	stdout, stderr, code := cli(t)
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"Usage:", "lsm [command]", "list"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help output is missing %q:\n%s", want, stdout)
		}
	}
}

func TestListWithNoConfigFilePrintsAnEmptyTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	stdout, stderr, code := cli(t, "list", "--config", path)

	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if want := "NAME  COMMAND  WORK_DIR\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestListRendersEveryServiceInSortedOrder(t *testing.T) {
	stdout, stderr, code := cli(t, "list", "--config", configFixture(t))

	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want a header plus two rows, got:\n%s", stdout)
	}
	if !strings.HasPrefix(lines[0], "NAME") {
		t.Errorf("first line is not the header: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "api") || !strings.HasPrefix(lines[2], "web") {
		t.Errorf("rows are not sorted by name:\n%s", stdout)
	}
	if !strings.Contains(lines[2], "node ./server.js") {
		t.Errorf("command column is missing:\n%s", stdout)
	}
}

func TestListReportsABrokenConfigOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("services:\n  web:\n    command: \"x\"\n   oops: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := cli(t, "list", "--config", path)
	if code != int(errs.ConfigRead) {
		t.Fatalf("exit = %d, want %d", code, errs.ConfigRead)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.HasPrefix(stderr, "lsm: cannot parse config ") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestUsageErrorsExitWithCodeTwo(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown command", []string{"bogus"}},
		{"unknown flag", []string{"list", "--bogus"}},
		{"unexpected argument", []string{"list", "extra"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := cli(t, tc.args...)
			if code != int(errs.Usage) {
				t.Fatalf("exit = %d, want %d (stderr: %s)", code, errs.Usage, stderr)
			}
			if !strings.HasPrefix(stderr, "lsm: ") {
				t.Errorf("stderr = %q, want an 'lsm: ' prefix", stderr)
			}
		})
	}
}

func TestLSMConfigEnvironmentVariableIsHonoured(t *testing.T) {
	path := configFixture(t)
	t.Setenv("LSM_CONFIG", path)

	stdout, stderr, code := cli(t, "list")
	if code != int(errs.OK) {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "web") {
		t.Fatalf("service list was not loaded from $LSM_CONFIG:\n%s", stdout)
	}
}
