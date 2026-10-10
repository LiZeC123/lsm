package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/LiZeC123/lsm/internal/errs"
	"gopkg.in/yaml.v3"
)

// Load reads and validates the configuration file at path.
//
// A missing file is not an error: it yields an empty configuration so that a
// fresh install can already run `lsm ls` (dev.md §10.4).
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{Services: map[string]Service{}}, nil
	}
	if err != nil {
		return nil, errs.Wrap(errs.ConfigRead, err, "cannot read config '%s'", path)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, errs.Wrap(errs.ConfigRead, err, "cannot parse config '%s'", path)
	}
	if cfg.Services == nil {
		cfg.Services = map[string]Service{}
	}
	cfg.normalize()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// normalize rewrites stored paths to forward slashes so that a hand-edited
// config keeps the on-disk convention of dev.md §3.4. It is a no-op on Unix.
func (c *Config) normalize() {
	for name, svc := range c.Services {
		svc.WorkDir = filepath.ToSlash(svc.WorkDir)
		c.Services[name] = svc
	}
}

// Get returns the named service and whether it exists.
func (c *Config) Get(name string) (Service, bool) {
	svc, ok := c.Services[name]
	return svc, ok
}
