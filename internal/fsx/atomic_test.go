package fsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomicCreatesTheFileAndItsParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "file.txt")
	if err := WriteFileAtomic(path, []byte("hello\n"), 0o644, 0o700); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("content = %q", data)
	}

	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("directory mode = %o, want 700", got)
	}
}

func TestWriteFileAtomicAppliesTheModeToANewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := WriteFileAtomic(path, []byte("x"), 0o600, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestWriteFileAtomicKeepsTheModeOfAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o644, 0o700); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want the preserved 600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want the replaced value", data)
	}
}

func TestWriteFileAtomicLeavesNoTemporaryFilesBehind(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFileAtomic(filepath.Join(dir, "file.txt"), []byte("x"), 0o644, 0o700); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".lsm-") {
			t.Errorf("leftover temporary file %q", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("want exactly one file, got %d", len(entries))
	}
}

func TestWriteFileAtomicReportsFailures(t *testing.T) {
	// A file used as if it were a directory makes MkdirAll fail.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := WriteFileAtomic(filepath.Join(blocker, "file.txt"), []byte("x"), 0o644, 0o700)
	if err == nil {
		t.Fatal("want an error when the parent path is a file")
	}
	if !strings.Contains(err.Error(), "cannot create directory") {
		t.Fatalf("unhelpful error: %v", err)
	}
}
