// Package fsx holds the small filesystem helpers shared by the lsm packages.
package fsx

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path with data without ever exposing a partially
// written file (dev.md §3.4).
//
// The bytes go into a temporary file in the same directory, are flushed to disk
// and only then renamed over the target, so an interrupted write leaves either
// the old or the new content behind — never a truncated file.
//
// Missing parent directories are created with dirMode. When path already exists
// its permission bits are preserved, otherwise mode is applied. The returned
// error is deliberately unclassified, leaving the exit code to the caller.
func WriteFileAtomic(path string, data []byte, mode, dirMode fs.FileMode) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("cannot create directory '%s': %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".lsm-*.tmp")
	if err != nil {
		return fmt.Errorf("cannot create a temporary file in '%s': %w", dir, err)
	}
	tmpName := tmp.Name()
	// Every failure path removes the temporary file; after a successful rename
	// the deferred calls are harmless no-ops.
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("cannot write '%s': %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("cannot flush '%s': %w", tmpName, err)
	}
	if err := tmp.Chmod(permissionsFor(path, mode)); err != nil {
		return fmt.Errorf("cannot set the permissions of '%s': %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cannot close '%s': %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("cannot replace '%s': %w", path, err)
	}
	return nil
}

// permissionsFor keeps the mode of an existing file so that a hand-hardened
// 0600 file does not silently become world readable on the next write.
func permissionsFor(path string, mode fs.FileMode) fs.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return mode
}
