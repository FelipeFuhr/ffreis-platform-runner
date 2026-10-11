// Package delivery loads per-product settings for an infra+website delivery
// run from an external YAML or JSON file, so no product-specific repo names
// or domains are ever compiled into the binary.
package delivery

import (
	"fmt"
	"strings"
)

// ProductConfig describes the repos and domain settings a single product
// needs for `deliver` to ensure its workspace and invoke `make go-deliver`
// in its infra repo.
type ProductConfig struct {
	InfraRepo       string `yaml:"infra_repo"        json:"infra_repo"`
	WebsiteRepo     string `yaml:"website_repo"      json:"website_repo"`
	CompilerRepo    string `yaml:"compiler_repo"     json:"compiler_repo"`
	PackerRepo      string `yaml:"packer_repo"       json:"packer_repo"`
	DomainName      string `yaml:"domain_name"       json:"domain_name"`
	WWWDomainName   string `yaml:"www_domain_name"   json:"www_domain_name"`
	Route53ZoneName string `yaml:"route53_zone_name" json:"route53_zone_name"`
	PublishPrefix   string `yaml:"publish_prefix"    json:"publish_prefix"`
}

// Validate checks that every repo required to orchestrate a delivery is set.
// Domain settings are optional: go-deliver falls back to its own defaults
// when they are left blank.
func (c *ProductConfig) Validate() error {
	missing := make([]string, 0, 4)
	if strings.TrimSpace(c.InfraRepo) == "" {
		missing = append(missing, "infra_repo")
	}
	if strings.TrimSpace(c.WebsiteRepo) == "" {
		missing = append(missing, "website_repo")
	}
	if strings.TrimSpace(c.CompilerRepo) == "" {
		missing = append(missing, "compiler_repo")
	}
	if strings.TrimSpace(c.PackerRepo) == "" {
		missing = append(missing, "packer_repo")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}
