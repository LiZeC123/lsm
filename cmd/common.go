package cmd

import (
	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/service"
)

// session bundles what the operational subcommands need: the resolved config
// path, the loaded configuration and the service manager built on top of it.
type session struct {
	path string
	cfg  *config.Config
	mgr  *service.Manager
}

// newSession resolves the config path, loads the configuration and builds the
// manager.
func newSession() (*session, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return &session{path: path, cfg: cfg, mgr: service.New(cfg, path)}, nil
}
