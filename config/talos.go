package config

import (
	"github.com/rotisserie/eris"
	"github.com/talos-systems/talos/pkg/client/config"
)

func Talos() (*config.Config, error) {
	configPath, err := config.GetDefaultPath()
	if err != nil {
		return nil, eris.Wrap(err, "failed to construct default Talos config path")
	}

	cfg, err := config.Open(configPath)
	if err != nil {
		return nil, eris.Wrap(err, "failed to read Talos config")
	}

	return cfg, nil
}
