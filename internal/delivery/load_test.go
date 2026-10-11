package delivery

import (
	"os"
	"path/filepath"
	"testing"
)

const testYAML = `
infra_repo: acme/acme-infra
website_repo: acme/acme-website
compiler_repo: acme/acme-website-compiler
packer_repo: acme/acme-website-packer
domain_name: acme.example.com
www_domain_name: www.acme.example.com
route53_zone_name: acme.example.com
publish_prefix: live
`

const testJSON = `{
  "infra_repo": "acme/acme-infra",
  "website_repo": "acme/acme-website",
  "compiler_repo": "acme/acme-website-compiler",
  "packer_repo": "acme/acme-website-packer",
  "domain_name": "acme.example.com"
}`

func writeConfig(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	return path
}

func TestLoad_YAML(t *testing.T) {
	path := writeConfig(t, "delivery.yaml", testYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.InfraRepo != "acme/acme-infra" {
		t.Fatalf("InfraRepo: got %q", cfg.InfraRepo)
	}
	if cfg.DomainName != "acme.example.com" {
		t.Fatalf("DomainName: got %q", cfg.DomainName)
	}
	if cfg.PublishPrefix != "live" {
		t.Fatalf("PublishPrefix: got %q", cfg.PublishPrefix)
	}
}

func TestLoad_JSON(t *testing.T) {
	path := writeConfig(t, "delivery.json", testJSON)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.WebsiteRepo != "acme/acme-website" {
		t.Fatalf("WebsiteRepo: got %q", cfg.WebsiteRepo)
	}
	if cfg.DomainName != "acme.example.com" {
		t.Fatalf("DomainName: got %q", cfg.DomainName)
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	if _, err := Load(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeConfig(t, "bad.yaml", "not: [valid")

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := writeConfig(t, "bad.json", "{not valid json")

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
