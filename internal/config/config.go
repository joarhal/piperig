package config

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

const configFile = ".piperig.yaml"

// Config holds project-level configuration.
type Config struct {
	Interpreters map[string]string `yaml:"interpreters"`
	Env          map[string]string `yaml:"env"`
}

// Default returns a Config with built-in interpreter mappings.
func Default() *Config {
	return &Config{
		Interpreters: map[string]string{
			".py": "python",
			".sh": "bash",
			".js": "node",
			".ts": "npx tsx",
			".rb": "ruby",
		},
		Env: make(map[string]string),
	}
}

// Load reads .piperig.yaml from the current working directory.
// If the file does not exist, it returns Default() with no error.
func Load() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Default(), nil
		}
		return nil, err
	}

	var file Config
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	// Start from defaults, overlay with file values
	cfg := Default()
	for ext, cmd := range file.Interpreters {
		cfg.Interpreters[ext] = cmd
	}
	for k, v := range file.Env {
		cfg.Env[k] = v
	}
	return cfg, nil
}
