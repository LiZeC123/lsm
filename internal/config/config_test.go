package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LiZeC123/lsm/internal/errs"
)

// scratch returns a temporary directory that also serves as an existing
// work_dir for the tests.
func scratch(t *testing.T) (dir, workDir string) {
	t.Helper()
	dir = t.TempDir()
	workDir = filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", workDir, err)
	}
	return dir, workDir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadMissingFileYieldsAnEmptyConfig(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Services == nil {
		t.Fatal("Services must be non-nil so callers can add to it")
	}
	if len(cfg.Services) != 0 {
		t.Fatalf("want no services, got %d", len(cfg.Services))
	}
}

func TestSaveLoadRoundTripKeepsTrickyCommands(t *testing.T) {
	dir, workDir := scratch(t)
	path := filepath.Join(dir, "config.yaml")

	want := map[string]Service{
		"quotes":    {Command: `node ./server.js --bind "127.0.0.1:8080"`, WorkDir: workDir},
		"pipeline":  {Command: `cat in.txt | grep -v "^#" | sort > out.txt`, WorkDir: workDir},
		"backslash": {Command: `C:\Python311\python.exe app.py`, WorkDir: workDir},
		"dollar":    {Command: `sh -c 'echo $HOME && echo "a b"'`, WorkDir: workDir},
		"multiline": {Command: "line1\nline2", WorkDir: workDir},
	}
	if err := (&Config{Services: want}).Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Services) != len(want) {
		t.Fatalf("got %d services, want %d", len(got.Services), len(want))
	}
	for name, wantSvc := range want {
		gotSvc, ok := got.Services[name]
		if !ok {
			t.Errorf("service %q disappeared", name)
			continue
		}
		if gotSvc != wantSvc {
			t.Errorf("service %q = %+v, want %+v", name, gotSvc, wantSvc)
		}
	}
}

func TestSaveIsDeterministicAndSorted(t *testing.T) {
	dir, workDir := scratch(t)
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")

	cfg := &Config{Services: map[string]Service{
		"web":    {Command: "a", WorkDir: workDir},
		"api":    {Command: "b", WorkDir: workDir},
		"worker": {Command: "c", WorkDir: workDir},
	}}
	if err := cfg.Save(first); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := cfg.Save(second); err != nil {
		t.Fatalf("Save: %v", err)
	}

	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("output is not deterministic:\n%s\n---\n%s", a, b)
	}
	if !strings.Contains(string(a), "services:\n") {
		t.Fatalf("unexpected document layout:\n%s", a)
	}
	if idxAPI, idxWeb := strings.Index(string(a), "  api:"), strings.Index(string(a), "  web:"); idxAPI > idxWeb {
		t.Fatalf("services must be written in sorted order:\n%s", a)
	}
	if !strings.HasSuffix(string(a), "\n") {
		t.Fatal("config must end with a newline")
	}
}

func TestSaveUsesTwoSpaceIndentation(t *testing.T) {
	dir, workDir := scratch(t)
	path := filepath.Join(dir, "config.yaml")
	cfg := &Config{Services: map[string]Service{"web": {Command: "run", WorkDir: workDir}}}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "services:\n  web:\n    command: run\n    work_dir: " + workDir + "\n"
	if got := string(data); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSaveCreatesTheDirectoryAndLeavesNoTemporaryFiles(t *testing.T) {
	dir, workDir := scratch(t)
	nested := filepath.Join(dir, "deeper", "still")
	path := filepath.Join(nested, "config.yaml")

	cfg := &Config{Services: map[string]Service{"web": {Command: "run", WorkDir: workDir}}}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(nested)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "config.yaml" {
			t.Errorf("unexpected leftover file %q", e.Name())
		}
	}
}

func TestSavePreservesTheModeOfAnExistingConfig(t *testing.T) {
	dir, workDir := scratch(t)
	path := filepath.Join(dir, "config.yaml")
	cfg := &Config{Services: map[string]Service{"web": {Command: "run", WorkDir: workDir}}}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestSaveRejectsAnInvalidConfigWithoutTouchingTheFile(t *testing.T) {
	dir, workDir := scratch(t)
	path := filepath.Join(dir, "config.yaml")

	good := &Config{Services: map[string]Service{"web": {Command: "run", WorkDir: workDir}}}
	if err := good.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	bad := &Config{Services: map[string]Service{"bad name": {Command: "run", WorkDir: workDir}}}
	err = bad.Save(path)
	if err == nil {
		t.Fatal("Save must reject a config with an invalid service name")
	}
	if got := errs.CodeOf(err); got != errs.ConfigRead {
		t.Fatalf("CodeOf = %d, want %d", got, errs.ConfigRead)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("config was modified despite the failure:\n%s", after)
	}
}

func TestLoadReportsYAMLSyntaxErrors(t *testing.T) {
	dir, _ := scratch(t)
	path := filepath.Join(dir, "config.yaml")
	write(t, path, "services:\n  web:\n    command: \"x\"\n   bad indent: 1\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load must fail on a syntax error")
	}
	if got := errs.CodeOf(err); got != errs.ConfigRead {
		t.Fatalf("CodeOf = %d, want %d", got, errs.ConfigRead)
	}
	if !strings.Contains(err.Error(), "cannot parse config") {
		t.Fatalf("unhelpful message: %v", err)
	}
}

func TestLoadValidatesSchema(t *testing.T) {
	dir, workDir := scratch(t)
	path := filepath.Join(dir, "config.yaml")

	tests := []struct {
		name    string
		doc     string
		wantMsg string
	}{
		{
			name:    "invalid name",
			doc:     "services:\n  \"bad name\":\n    command: run\n    work_dir: " + workDir + "\n",
			wantMsg: "invalid service name",
		},
		{
			name:    "name too long",
			doc:     "services:\n  " + strings.Repeat("a", 65) + ":\n    command: run\n    work_dir: " + workDir + "\n",
			wantMsg: "invalid service name",
		},
		{
			name:    "empty command",
			doc:     "services:\n  web:\n    command: \"\"\n    work_dir: " + workDir + "\n",
			wantMsg: "command must not be empty",
		},
		{
			name:    "missing command",
			doc:     "services:\n  web:\n    work_dir: " + workDir + "\n",
			wantMsg: "command must not be empty",
		},
		{
			name:    "empty work_dir",
			doc:     "services:\n  web:\n    command: run\n    work_dir: \"\"\n",
			wantMsg: "work_dir must not be empty",
		},
		{
			name:    "relative work_dir",
			doc:     "services:\n  web:\n    command: run\n    work_dir: relative/dir\n",
			wantMsg: "must be an absolute path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			write(t, path, tc.doc)
			_, err := Load(path)
			if err == nil {
				t.Fatal("Load must fail")
			}
			if got := errs.CodeOf(err); got != errs.ConfigRead {
				t.Fatalf("CodeOf = %d, want %d", got, errs.ConfigRead)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q does not contain %q", err, tc.wantMsg)
			}
		})
	}
}

// Load must not touch the filesystem: a service whose work_dir disappeared has
// to stay visible in `lsm list` / `lsm ps` and removable through `lsm del`.
func TestLoadAcceptsAServiceWhoseWorkDirIsGone(t *testing.T) {
	dir, _ := scratch(t)
	path := filepath.Join(dir, "config.yaml")
	missing := filepath.Join(dir, "unmounted")
	write(t, path,
		"services:\n  web:\n    command: run\n    work_dir: "+missing+"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load must tolerate a missing work_dir, got: %v", err)
	}
	if _, ok := cfg.Get("web"); !ok {
		t.Fatal("the service must still be listed")
	}
}

func TestCheckWorkDir(t *testing.T) {
	dir, workDir := scratch(t)
	file := filepath.Join(dir, "a-file")
	write(t, file, "x")

	if err := CheckWorkDir(workDir); err != nil {
		t.Errorf("CheckWorkDir(%q) = %v, want nil", workDir, err)
	}

	err := CheckWorkDir(filepath.Join(dir, "gone"))
	if err == nil || !strings.Contains(err.Error(), "is not accessible") {
		t.Errorf("missing directory: got %v", err)
	}

	err = CheckWorkDir(file)
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("regular file: got %v", err)
	}

	// CheckWorkDir stays unclassified so that start can exit 8 and add can
	// exit 6 for the very same condition.
	if got := errs.CodeOf(CheckWorkDir(file)); got != errs.General {
		t.Errorf("CodeOf = %d, want the unclassified %d", got, errs.General)
	}
}

func TestLoadToleratesAnEmptyDocument(t *testing.T) {
	dir, _ := scratch(t)
	path := filepath.Join(dir, "config.yaml")
	write(t, path, "")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Services) != 0 {
		t.Fatalf("want no services, got %d", len(cfg.Services))
	}
}

func TestNamesIsSorted(t *testing.T) {
	cfg := &Config{Services: map[string]Service{"c": {}, "a": {}, "b": {}}}
	got := strings.Join(cfg.Names(), ",")
	if got != "a,b,c" {
		t.Fatalf("Names() = %q, want %q", got, "a,b,c")
	}
}

func TestValidName(t *testing.T) {
	valid := []string{"web", "web-1", "web_1", "A", strings.Repeat("a", 64), "123"}
	for _, name := range valid {
		if !ValidName(name) {
			t.Errorf("ValidName(%q) = false, want true", name)
		}
	}
	invalid := []string{"", " ", "a b", "a/b", "a.b", "a:b", "日本語", strings.Repeat("a", 65)}
	for _, name := range invalid {
		if ValidName(name) {
			t.Errorf("ValidName(%q) = true, want false", name)
		}
	}
}
