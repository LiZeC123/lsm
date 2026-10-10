// Package service orchestrates the config and process modules into the
// operations the CLI exposes: start, stop and restart of a named service.
//
// The manager returns data and classified errors only; turning those into the
// exact stdout / stderr wording and exit codes is the cmd layer's job
// (dev.md §2.1).
package service

import (
	"errors"
	"time"

	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/errs"
	"github.com/LiZeC123/lsm/internal/process"
)

// Manager ties a loaded configuration to its PID file directory.
type Manager struct {
	cfg        *config.Config
	configPath string
}

// New builds a manager for an already loaded configuration. configPath is used
// to locate the sibling run/ directory that holds the PID files.
func New(cfg *config.Config, configPath string) *Manager {
	return &Manager{cfg: cfg, configPath: configPath}
}

// Config exposes the loaded configuration.
func (m *Manager) Config() *config.Config { return m.cfg }

// pidPath returns the PID file location of a service.
func (m *Manager) pidPath(name string) string {
	return config.PIDPath(m.configPath, name)
}

// lookup returns a configured service or a "not found" error (exit code 3).
func (m *Manager) lookup(name string) (config.Service, error) {
	svc, ok := m.cfg.Get(name)
	if !ok {
		return config.Service{}, errs.New(errs.NotFound, "service '%s' not found", name)
	}
	return svc, nil
}

// Start launches a service and returns the PID of the new process.
//
// It refuses to start an already running service. A stale PID file is ignored
// and overwritten (dev.md §7.3).
func (m *Manager) Start(name string) (int, error) {
	svc, err := m.lookup(name)
	if err != nil {
		return 0, err
	}
	// dev.md §5.2 classifies a broken work_dir on start as a process failure.
	if err := config.CheckWorkDir(svc.WorkDir); err != nil {
		return 0, errs.Classify(errs.ProcessOp, err)
	}

	pidPath := m.pidPath(name)
	if err := m.ensureNotRunning(name, pidPath); err != nil {
		return 0, err
	}

	pid, err := process.Launch(svc)
	if err != nil {
		if errors.Is(err, process.ErrExitedImmediately) {
			return 0, errs.Wrap(errs.ProcessOp, err, "service '%s' exited immediately after start", name)
		}
		return 0, errs.Wrap(errs.ProcessOp, err, "service '%s' failed to start", name)
	}

	info := process.PIDInfo{
		PID:       pid,
		StartedAt: time.Now().Unix(),
		WorkDir:   svc.WorkDir,
		Command:   svc.Command,
	}
	if err := process.WritePIDFile(pidPath, info); err != nil {
		// The service is already running; killing it just because its
		// bookkeeping file could not be written would be worse than reporting
		// the problem and leaving it up.
		return 0, err
	}
	return pid, nil
}

// ensureNotRunning fails when the service is up, and silently accepts a stale
// PID file, which Start then overwrites.
func (m *Manager) ensureNotRunning(name, pidPath string) error {
	info, err := process.ReadPIDFile(pidPath)
	if err != nil {
		return err
	}
	if info != nil && process.IsAlive(*info) {
		return errs.New(errs.AlreadyRunning,
			"service '%s' is already running (pid %d)", name, info.PID)
	}
	return nil
}

// StopResult reports what Stop actually did, so the cmd layer can print either
// "stopped" or "is not running" (dev.md §5.3).
type StopResult struct {
	WasRunning bool
	PID        int
}

// Stop terminates a service.
func (m *Manager) Stop(name string) (StopResult, error) {
	if _, err := m.lookup(name); err != nil {
		return StopResult{}, err
	}

	pidPath := m.pidPath(name)
	info, err := process.ReadPIDFile(pidPath)
	if err != nil {
		return StopResult{}, err
	}
	if info == nil {
		return StopResult{}, nil // never started through lsm
	}

	err = process.Stop(*info)
	if errors.Is(err, process.ErrNotRunning) {
		// Stale PID file: drop it and report the service as not running.
		if err := process.RemovePIDFile(pidPath); err != nil {
			return StopResult{}, err
		}
		return StopResult{}, nil
	}
	if err != nil {
		return StopResult{}, errs.Wrap(errs.ProcessOp, err, "service '%s' failed to stop", name)
	}

	if err := process.RemovePIDFile(pidPath); err != nil {
		return StopResult{}, err
	}
	return StopResult{WasRunning: true, PID: info.PID}, nil
}

// RestartResult reports whether the service had to be stopped first.
type RestartResult struct {
	WasRunning bool
	PID        int
}

// Restart stops a running service and starts it again. A service that was not
// running is simply started (dev.md §5.4).
func (m *Manager) Restart(name string) (RestartResult, error) {
	stopped, err := m.Stop(name)
	if err != nil {
		return RestartResult{}, err
	}
	pid, err := m.Start(name)
	if err != nil {
		return RestartResult{}, err
	}
	return RestartResult{WasRunning: stopped.WasRunning, PID: pid}, nil
}
