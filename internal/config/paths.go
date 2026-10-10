package config

import (
	"os"
	"path/filepath"

	"github.com/LiZeC123/lsm/internal/errs"
)

// AppDirName is the directory lsm owns inside the user configuration directory.
const AppDirName = "lsm"

// ConfigFileName is the default configuration file name.
const ConfigFileName = "config.yaml"

// RunDirName is the sub-directory holding the per-service PID files.
const RunDirName = "run"

// DefaultPath returns the platform default configuration file location,
// <os.UserConfigDir()>/lsm/config.yaml (dev.md §4.1):
//
//	Linux   ~/.config/lsm/config.yaml
//	macOS   ~/Library/Application Support/lsm/config.yaml
//	Windows %AppData%\lsm\config.yaml
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", errs.Wrap(errs.ConfigRead, err, "cannot determine the user config directory")
	}
	return filepath.Join(dir, AppDirName, ConfigFileName), nil
}

// ResolvePath applies the override precedence of dev.md §4.2, highest first:
//
//  1. the --config flag
//  2. the LSM_CONFIG environment variable
//  3. the platform default path
func ResolvePath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if env := os.Getenv("LSM_CONFIG"); env != "" {
		return env, nil
	}
	return DefaultPath()
}

// RunDir returns the directory that holds the PID files. It always sits next to
// the active configuration file so that a custom --config stays self-contained
// (dev.md §4.3).
func RunDir(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), RunDirName)
}

// PIDPath returns the PID file path of a single service.
func PIDPath(configPath, name string) string {
	return filepath.Join(RunDir(configPath), name+".pid")
}
