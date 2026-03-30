package config

import (
	"context"
	"os"
	"testing"
)

const (
	testYAML = `
repos:
  - name: example-org/infra-repo
    environments: [dev, prod]
    tf_working_dir: terraform/
    template_ref: terraform-infra
    enabled: true
  - name: example-org/app-service
    environments: [prod]
    enabled: true
  - name: example-org/legacy-service
    enabled: false
`

	disabledYAML = `
repos:
  - name: example-org/disabled-repo
    enabled: false
`
)

func loadReposFromYAML(t *testing.T, yaml string) []RepoConfig {
	t.Helper()

	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(yaml); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}

	loader := &YAMLLoader{Path: f.Name()}
	repos, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	return repos
}

func TestYAMLLoad(t *testing.T) {
	repos := loadReposFromYAML(t, testYAML)

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos (enabled only), got %d", len(repos))
	}

	// Verify legacy-service (enabled=false) is excluded.
	for _, r := range repos {
		if r.Name == "example-org/legacy-service" {
			t.Errorf("disabled repo example-org/legacy-service should be excluded")
		}
	}
}

func TestYAMLLoad_Disabled(t *testing.T) {
	repos := loadReposFromYAML(t, disabledYAML)

	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}
}

func TestYAMLLoad_Fields(t *testing.T) {
	repos := loadReposFromYAML(t, testYAML)

	// Find example-org/infra-repo and verify its fields.
	var infra *RepoConfig
	for i := range repos {
		if repos[i].Name == "example-org/infra-repo" {
			infra = &repos[i]
			break
		}
	}
	if infra == nil {
		t.Fatalf("example-org/infra-repo not found in results")
	}

	if len(infra.Environments) != 2 {
		t.Errorf("expected 2 environments, got %d", len(infra.Environments))
	}
	if infra.Environments[0] != "dev" || infra.Environments[1] != "prod" {
		t.Errorf("unexpected environments: %v", infra.Environments)
	}
	if infra.TFWorkingDir != "terraform/" {
		t.Errorf("expected tf_working_dir=terraform/, got %q", infra.TFWorkingDir)
	}
	if infra.TemplateRef != "terraform-infra" {
		t.Errorf("expected template_ref=terraform-infra, got %q", infra.TemplateRef)
	}
	if !infra.Enabled {
		t.Errorf("expected enabled=true")
	}
}
