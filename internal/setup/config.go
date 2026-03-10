package setup

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func readExistingConfig(cwd string) (*Config, error) {
	path := filepath.Join(cwd, ConfigPath)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func marshalConfig(cfg Config) ([]byte, error) {
	body, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return append([]byte(configFileHeader+"\n"), body...), nil
}
