package config

import (
	"context"
	"fmt"
)

var newDynamoLoader = NewDynamoLoader

// RepoConfig describes one repository to be managed.
type RepoConfig struct {
	Name         string            // "org/repo" format
	Environments []string          // e.g. ["dev", "prod"]
	TFWorkingDir string            // path inside repo to Terraform root, default "."
	TFVars       map[string]string // extra var=value pairs
	TemplateRef  string            // which template set applies, default "default"
	Enabled      bool              // if false, skip this repo
}

// Loader loads repo configurations from a source.
type Loader interface {
	Load(ctx context.Context) ([]RepoConfig, error)
}

// Load returns a Loader backed by DynamoDB if tableName is set,
// otherwise falls back to YAML at fallbackPath.
func Load(ctx context.Context, tableName, fallbackPath string) ([]RepoConfig, error) {
	var loader Loader
	if tableName != "" {
		dl, err := newDynamoLoader(ctx, tableName)
		if err != nil {
			return nil, fmt.Errorf("creating dynamo loader: %w", err)
		}
		loader = dl
	} else {
		loader = &YAMLLoader{Path: fallbackPath}
	}
	return loader.Load(ctx)
}
