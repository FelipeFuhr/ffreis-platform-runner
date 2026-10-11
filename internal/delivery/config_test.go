package delivery

import (
	"strings"
	"testing"
)

func TestProductConfig_Validate_AllRequiredFieldsSet(t *testing.T) {
	cfg := ProductConfig{
		InfraRepo:    "acme/acme-infra",
		WebsiteRepo:  "acme/acme-website",
		CompilerRepo: "acme/acme-website-compiler",
		PackerRepo:   "acme/acme-website-packer",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestProductConfig_Validate_ReportsEachMissingField(t *testing.T) {
	cfg := ProductConfig{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty config")
	}
	for _, want := range []string{"infra_repo", "website_repo", "compiler_repo", "packer_repo"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error %q missing %q", err.Error(), want)
		}
	}
}

func TestProductConfig_Validate_OnlyMissingReportsPartial(t *testing.T) {
	cfg := ProductConfig{
		InfraRepo:   "acme/acme-infra",
		WebsiteRepo: "acme/acme-website",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for partial config")
	}
	if strings.Contains(err.Error(), "infra_repo") || strings.Contains(err.Error(), "website_repo") {
		t.Fatalf("Validate() error should not mention set fields: %v", err)
	}
	if !strings.Contains(err.Error(), "compiler_repo") || !strings.Contains(err.Error(), "packer_repo") {
		t.Fatalf("Validate() error should mention unset fields: %v", err)
	}
}
