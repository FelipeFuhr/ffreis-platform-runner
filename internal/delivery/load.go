package delivery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads a ProductConfig from a YAML or JSON file at path. The format is
// chosen by file extension: ".json" is parsed as JSON, anything else
// (".yaml", ".yml", or no recognized extension) is parsed as YAML.
func Load(path string) (*ProductConfig, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("config path is required")
	}

	//nolint:gosec // G304: path is an operator-supplied --delivery-config flag
	// value, the same trust level as the rest of this CLI's config flags.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading delivery config %q: %w", path, err)
	}

	var cfg ProductConfig
	if strings.EqualFold(filepath.Ext(path), ".json") {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing JSON delivery config %q: %w", path, err)
		}
		return &cfg, nil
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML delivery config %q: %w", path, err)
	}
	return &cfg, nil
}
