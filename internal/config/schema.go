// Package config handles the lsm YAML configuration file: schema definition,
// loading and validation, atomic saving and cross-platform path resolution.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/LiZeC123/lsm/internal/errs"
)

// Config is the root document of config.yaml (dev.md §3.1).
type Config struct {
	Services map[string]Service `yaml:"services"`
}

// Service describes one managed background service.
type Service struct {
	Command string `yaml:"command"`
	WorkDir string `yaml:"work_dir"`
}

const (
	// NamePattern is the only charset accepted in a service name (dev.md §3.3).
	NamePattern = `^[A-Za-z0-9_-]+$`
	// MaxNameLen is the maximum accepted service name length.
	MaxNameLen = 64
)

var nameRe = regexp.MustCompile(NamePattern)

// ValidName reports whether name satisfies the service name rules of §3.3.
func ValidName(name string) bool {
	return len(name) >= 1 && len(name) <= MaxNameLen && nameRe.MatchString(name)
}

// Names returns every configured service name in a stable, sorted order.
func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Services))
	for name := range c.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Validate checks the schema rules of dev.md §3.3 without touching the
// filesystem, so it is also usable before writing a config back to disk.
func (c *Config) Validate() error {
	for _, name := range c.Names() {
		svc := c.Services[name]
		if !ValidName(name) {
			return errs.New(errs.ConfigRead,
				"invalid service name '%s': must match %s and be 1-%d characters",
				name, NamePattern, MaxNameLen)
		}
		if strings.TrimSpace(svc.Command) == "" {
			return errs.New(errs.ConfigRead, "service '%s': command must not be empty", name)
		}
		if strings.TrimSpace(svc.WorkDir) == "" {
			return errs.New(errs.ConfigRead, "service '%s': work_dir must not be empty", name)
		}
		if !filepath.IsAbs(svc.WorkDir) {
			return errs.New(errs.ConfigRead,
				"service '%s': work_dir '%s' must be an absolute path", name, svc.WorkDir)
		}
	}
	return nil
}

// CheckWorkDir verifies that dir exists and is a directory. It is deliberately
// left unclassified: the caller attaches the exit code its context requires,
// which dev.md sets to 8 for start / restart / run (§5.2, §5.11); add and
// update re-ask instead of exiting (§5.7, §5.9).
//
// Load itself only validates the schema (see Validate), so a service whose
// work_dir has gone missing — a deleted directory or an unmounted volume — is
// still listed by `lsm ls` / `lsm ps` and can still be removed with `lsm del`.
func CheckWorkDir(dir string) error {
	shown := filepath.ToSlash(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("work_dir '%s' is not accessible: %w", shown, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("work_dir '%s' is not a directory", shown)
	}
	return nil
}
