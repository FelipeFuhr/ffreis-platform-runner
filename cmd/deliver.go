package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FelipeFuhr/ffreis-platform-cli/pkg/ui"

	"github.com/ffreis/platform-runner/internal/delivery"
	"github.com/ffreis/platform-runner/internal/logging"
	"github.com/ffreis/platform-runner/internal/repos"
)

var (
	deliverConfirm         bool
	deliverEnv             = "prod"
	deliverOrg             string
	deliverProfile         string
	deliverConfigPath      string
	deliverInfraRepo       string
	deliverWebsiteRepo     string
	deliverCompilerRepo    string
	deliverPackerRepo      string
	deliverDomainName      string
	deliverWWWDomainName   string
	deliverRoute53ZoneName string
	deliverPublishPrefix   string
)

// deliverWorkspaces holds the local clone directories for one delivery run.
type deliverWorkspaces struct {
	infraDir, websiteDir, compilerDir, packerDir string
}

// deliverRunOpts bundles the pass-through options for `make go-deliver` so
// deliverMakeArgs stays within the project's parameter-count guideline.
type deliverRunOpts struct {
	Env, Org, Profile string
}

var deliverCmd = &cobra.Command{
	Use:   "deliver",
	Short: "Run a product's infra+website delivery flow through the shared runner workspace",
	Long: `deliver orchestrates an infra + website delivery for a single product by
ensuring four repos (infra, website, compiler, packer) in the local workspace
and invoking "make go-deliver" in the infra repo. The product's repos and
domain settings are read from a YAML or JSON config file supplied with
--delivery-config; see configs/delivery.yaml.example. Requires --confirm.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagDryRun {
			return fmt.Errorf("--dry-run is not supported for deliver")
		}
		if !deliverConfirm {
			return fmt.Errorf("--confirm is required")
		}

		product, err := loadDeliverProduct()
		if err != nil {
			return err
		}

		presenter, err := ui.New(flagUI)
		if err != nil {
			return fmt.Errorf("building ui: %w", err)
		}
		log, err := logging.New(flagLogLevel, presenter.Interactive())
		if err != nil {
			return fmt.Errorf("building logger: %w", err)
		}

		token := flagToken
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}

		out := newCommandOutput(cmd.OutOrStdout(), presenter)
		out.Header("Platform Runner Delivery", "workspace "+flagWorkspace)
		out.Summary("Repos",
			"infra="+product.InfraRepo,
			"website="+product.WebsiteRepo,
			"compiler="+product.CompilerRepo,
			"packer="+product.PackerRepo,
		)
		out.Blank()

		out.Status("info", "sync", "ensuring local workspaces")
		ws, err := ensureDeliverWorkspaces(cmd.Context(), flagWorkspace, token, product)
		if err != nil {
			return err
		}
		out.Status("ok", "sync", "workspace repos ready")

		opts := deliverRunOpts{Env: deliverEnv, Org: deliverOrg, Profile: deliverProfile}
		args := deliverMakeArgs(opts, product, ws)

		out.Blank()
		out.Status("info", "deliver", "running make "+strings.Join(args, " "))
		if err := runGoDeliver(cmd.Context(), ws.infraDir, args, cmd); err != nil {
			log.Error("delivery failed", "error", err)
			return fmt.Errorf("delivery failed: %w", err)
		}

		out.Blank()
		out.Status("ok", "live", "delivery completed through platform-runner")
		return nil
	},
}

// loadDeliverProduct loads the per-product delivery settings from
// --delivery-config, applies any CLI flag overrides, and validates that the
// repos required to orchestrate a delivery are present.
func loadDeliverProduct() (*delivery.ProductConfig, error) {
	if strings.TrimSpace(deliverConfigPath) == "" {
		return nil, fmt.Errorf("--delivery-config is required")
	}

	product, err := delivery.Load(deliverConfigPath)
	if err != nil {
		return nil, fmt.Errorf("loading delivery config: %w", err)
	}

	applyDeliverOverrides(product)

	if err := product.Validate(); err != nil {
		return nil, fmt.Errorf("invalid delivery config: %w", err)
	}
	return product, nil
}

// applyDeliverOverrides lets individual CLI flags override the values loaded
// from the delivery config, matching the override behavior the per-product
// repo flags used to provide.
func applyDeliverOverrides(p *delivery.ProductConfig) {
	if deliverInfraRepo != "" {
		p.InfraRepo = deliverInfraRepo
	}
	if deliverWebsiteRepo != "" {
		p.WebsiteRepo = deliverWebsiteRepo
	}
	if deliverCompilerRepo != "" {
		p.CompilerRepo = deliverCompilerRepo
	}
	if deliverPackerRepo != "" {
		p.PackerRepo = deliverPackerRepo
	}
	if deliverDomainName != "" {
		p.DomainName = deliverDomainName
	}
	if deliverWWWDomainName != "" {
		p.WWWDomainName = deliverWWWDomainName
	}
	if deliverRoute53ZoneName != "" {
		p.Route53ZoneName = deliverRoute53ZoneName
	}
	if deliverPublishPrefix != "" {
		p.PublishPrefix = deliverPublishPrefix
	}
}

// ensureDeliverWorkspaces clones (or updates) the four repos a delivery needs
// into the local workspace and returns their resolved directories.
func ensureDeliverWorkspaces(ctx context.Context, root, token string, product *delivery.ProductConfig) (deliverWorkspaces, error) {
	ensure := func(repo string) (string, error) {
		w := &repos.Workspace{Repo: repo, RootDir: root, Token: token}
		if err := w.Ensure(ctx); err != nil {
			return "", err
		}
		return w.Dir(), nil
	}

	var ws deliverWorkspaces
	var err error
	if ws.infraDir, err = ensure(product.InfraRepo); err != nil {
		return ws, err
	}
	if ws.websiteDir, err = ensure(product.WebsiteRepo); err != nil {
		return ws, err
	}
	if ws.compilerDir, err = ensure(product.CompilerRepo); err != nil {
		return ws, err
	}
	if ws.packerDir, err = ensure(product.PackerRepo); err != nil {
		return ws, err
	}
	return ws, nil
}

// deliverMakeArgs builds the "make go-deliver" argument list from the
// product's config and the resolved workspace directories.
func deliverMakeArgs(opts deliverRunOpts, product *delivery.ProductConfig, ws deliverWorkspaces) []string {
	args := []string{"go-deliver", "ENV=" + opts.Env}
	if strings.TrimSpace(opts.Org) != "" {
		args = append(args, "ORG="+opts.Org)
	}
	if strings.TrimSpace(opts.Profile) != "" {
		args = append(args, "PROFILE="+opts.Profile)
	}
	args = append(args,
		"WEBSITE_ROOT="+ws.websiteDir,
		"WEBSITE_COMPILER_ROOT="+ws.compilerDir,
		"WEBSITE_PACKER_ROOT="+ws.packerDir,
	)
	if strings.TrimSpace(product.PublishPrefix) != "" {
		args = append(args, "WEBSITE_PUBLISH_PREFIX="+product.PublishPrefix)
	}
	if strings.TrimSpace(product.DomainName) != "" {
		args = append(args, "DOMAIN_NAME="+product.DomainName)
	}
	if strings.TrimSpace(product.WWWDomainName) != "" {
		args = append(args, "WWW_DOMAIN_NAME="+product.WWWDomainName)
	}
	if strings.TrimSpace(product.Route53ZoneName) != "" {
		args = append(args, "ROUTE53_ZONE_NAME="+product.Route53ZoneName)
	}
	return args
}

// runGoDeliver runs "make <args...>" in dir, streaming output through cmd.
func runGoDeliver(ctx context.Context, dir string, args []string, cmd *cobra.Command) error {
	//nolint:gosec // G204: "make" is a fixed binary name; args is built from the
	// resolved delivery config and workspace paths, not raw user/shell input.
	makeCmd := exec.CommandContext(ctx, "make", args...)
	makeCmd.Dir = dir
	makeCmd.Stdout = cmd.OutOrStdout()
	makeCmd.Stderr = cmd.ErrOrStderr()
	makeCmd.Env = os.Environ()
	return makeCmd.Run()
}

func init() {
	deliverCmd.Flags().BoolVar(&deliverConfirm, "confirm", false, "Confirm the delivery run")
	deliverCmd.Flags().StringVar(&deliverEnv, "env", "prod", "Environment to deliver")
	deliverCmd.Flags().StringVar(&deliverOrg, "org", "", "Org passed through to make go-deliver")
	deliverCmd.Flags().StringVar(&deliverProfile, "profile", "", "AWS profile passed through to make go-deliver")
	deliverCmd.Flags().StringVar(&deliverConfigPath, "delivery-config", "", "Path to a YAML or JSON product delivery config (required)")
	deliverCmd.Flags().StringVar(&deliverInfraRepo, "infra-repo", "", "Infra repo in org/repo format (overrides the delivery config)")
	deliverCmd.Flags().StringVar(&deliverWebsiteRepo, "website-repo", "", "Website repo in org/repo format (overrides the delivery config)")
	deliverCmd.Flags().StringVar(&deliverCompilerRepo, "compiler-repo", "", "Website compiler repo in org/repo format (overrides the delivery config)")
	deliverCmd.Flags().StringVar(&deliverPackerRepo, "packer-repo", "", "Website packer repo in org/repo format (overrides the delivery config)")
	deliverCmd.Flags().StringVar(&deliverDomainName, "domain-name", "", "domain_name override passed through to go-deliver")
	deliverCmd.Flags().StringVar(&deliverWWWDomainName, "www-domain-name", "", "www_domain_name override passed through to go-deliver")
	deliverCmd.Flags().StringVar(&deliverRoute53ZoneName, "route53-zone-name", "", "route53_zone_name override passed through to go-deliver")
	deliverCmd.Flags().StringVar(&deliverPublishPrefix, "publish-prefix", "", "Website publish prefix override passed through to go-deliver")
}
