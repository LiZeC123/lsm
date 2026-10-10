package config

import (
	"bytes"

	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/fsx"
	"gopkg.in/yaml.v3"
)

const (
	dirMode  = 0o755
	fileMode = 0o644
)

// Save writes the configuration to path atomically (dev.md §3.4). An
// interrupted write leaves either the old or the new config in place, never a
// truncated one.
func (c *Config) Save(path string) error {
	if err := c.Validate(); err != nil {
		return err
	}

	data, err := encode(c)
	if err != nil {
		return err
	}
	if err := fsx.WriteFileAtomic(path, data, fileMode, dirMode); err != nil {
		return errs.Classify(errs.ConfigWrite, err)
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
