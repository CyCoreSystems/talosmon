package config

import (
	"os"

	"github.com/CyCoreSystems/talosmon/machine"
	"github.com/rotisserie/eris"
	"gopkg.in/yaml.v2"
)

// Config describes the talosmon configuration
type Config struct {
	Clusters []Cluster `json:"clusters"`
}

// Cluster describes a cluster configuration
type Cluster struct {
	Name string `json:"name"`

	// Config describes the Talos configuration
	Config TalosConfig `json:"config"`

	// Machines is the list of machines in the cluster
	Machines []*machine.Spec `json:"machines"`
}

type TalosConfig struct {

	// File is the configuration file.  It defaults to $HOME/.talos/config.
	File string `json:"file"`

	// Context is the cluster context inside the config file.  If not defined, it will use the Talos configuration default.
	Context string `json:"context"`
}

func Load(name string) (*Config, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, eris.Wrap(err, "failed to open configuration file")
	}

	cfg := new(Config)
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, eris.Wrap(err, "failed to parse configuration file")
	}
	return cfg, nil
}
