package cmd

import (
	"github.com/LiZeC123/lsm/internal/config"
	"github.com/LiZeC123/lsm/internal/service"
)

// loadManager resolves the config path, loads the configuration and builds the
// service manager every operational subcommand needs.
func loadManager() (*service.Manager, *config.Config, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, nil, err
	}
	return service.New(cfg, path), cfg, nil
}
