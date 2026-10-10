package config

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/LiZeC123/lsm/internal/errs"
	"gopkg.in/yaml.v3"
)

const (
	dirMode  os.FileMode = 0o755
	fileMode os.FileMode = 0o644
)

// Save writes the configuration to path atomically (dev.md §3.4).
//
// The document is encoded into a temporary file in the same directory, flushed
// to disk and only then renamed over the target. An interrupted write therefore
// leaves either the old or the new config in place, never a truncated one.
func (c *Config) Save(path string) error {
	if err := c.Validate(); err != nil {
		return err
	}

	data, err := encode(c)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return errs.Wrap(errs.ConfigWrite, err, "cannot create config directory '%s'", dir)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return errs.Wrap(errs.ConfigWrite, err, "cannot create a temporary file in '%s'", dir)
	}
	tmpName := tmp.Name()
	// Every failure path cleans the temporary file up; after a successful
	// rename both deferred calls are harmless no-ops.
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		return errs.Wrap(errs.ConfigWrite, err, "cannot write '%s'", tmpName)
	}
	if err := tmp.Sync(); err != nil {
		return errs.Wrap(errs.ConfigWrite, err, "cannot flush '%s'", tmpName)
	}
	if err := tmp.Chmod(existingMode(path)); err != nil {
		return errs.Wrap(errs.ConfigWrite, err, "cannot set the permissions of '%s'", tmpName)
	}
	if err := tmp.Close(); err != nil {
		return errs.Wrap(errs.ConfigWrite, err, "cannot close '%s'", tmpName)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return errs.Wrap(errs.ConfigWrite, err, "cannot replace '%s'", path)
	}
	return nil
}

// encode renders the config as YAML with two-space indentation.
func encode(c *Config) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return nil, errs.Wrap(errs.ConfigWrite, err, "cannot encode config")
	}
	if err := enc.Close(); err != nil {
		return nil, errs.Wrap(errs.ConfigWrite, err, "cannot encode config")
	}
	return buf.Bytes(), nil
}

// existingMode keeps the permissions of an already present config file, so a
// hand-hardened 0600 config does not silently become world readable.
func existingMode(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return fileMode
}
