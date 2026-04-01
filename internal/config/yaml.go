package config

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// yamlFile represents the top-level structure of the YAML config file.
type yamlFile struct {
	Repos []yamlRepo `yaml:"repos"`
}

// yamlRepo represents a single repo entry in the YAML config.
type yamlRepo struct {
	Name         string            `yaml:"name"`
	Environments []string          `yaml:"environments"`
	TFWorkingDir string            `yaml:"tf_working_dir"`
	TFVars       map[string]string `yaml:"tf_vars"`
	TemplateRef  string            `yaml:"template_ref"`
	Enabled      bool              `yaml:"enabled"`
}

// YAMLLoader loads RepoConfig items from a YAML file.
type YAMLLoader struct {
	Path string
}

// Load reads the YAML file and returns enabled RepoConfig entries.
func (y *YAMLLoader) Load(_ context.Context) ([]RepoConfig, error) {
	data, err := os.ReadFile(y.Path)
	if err != nil {
		return nil, fmt.Errorf("reading YAML config %q: %w", y.Path, err)
	}

	var f yamlFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing YAML config %q: %w", y.Path, err)
	}

	var configs []RepoConfig
	for _, r := range f.Repos {
		if !r.Enabled {
			continue
		}
		rc := RepoConfig(r)
		configs = append(configs, rc)
	}

	return configs, nil
}
