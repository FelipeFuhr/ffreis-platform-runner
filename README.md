# ffreis-platform-runner

<!-- ffreis-badges:start -->
[![CI](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/FelipeFuhr/ffreis-badges/main/badges/ffreis-platform-runner/ci.json)](https://github.com/FelipeFuhr/ffreis-platform-runner/actions) [![License](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/FelipeFuhr/ffreis-badges/main/badges/ffreis-platform-runner/license.json)](https://github.com/FelipeFuhr/ffreis-platform-runner/blob/main/LICENSE)
<!-- ffreis-badges:end -->

`platform-runner` is a Go CLI that operates a configured set of platform repositories in bulk. From a single config source (a YAML file or a DynamoDB table), it clones each repo into a local workspace and runs the same operation across all of them concurrently: Terraform `plan`, Terraform `apply`, template-file sync, and `platform-guardian` validation. It is built to run both locally and inside an OCI container. Diagnostics go to stderr and machine-readable results go to stdout, so the two streams never mix.

## What it does

The runner reads a list of `RepoConfig` entries, each describing one `org/repo`, the environments it supports (e.g. `[dev, staging, prod]`), an optional Terraform working directory, extra Terraform vars, a template ref, and an `enabled` flag. Disabled repos are skipped; a repo with no environments is skipped with a warning. For each enabled repo (or repo × environment), it ensures a shallow clone exists under the workspace directory (clone if absent, otherwise fetch and hard-reset to the latest fetched HEAD), then runs the requested action through a bounded worker pool. Panics in any per-repo task are recovered and recorded as a failed result rather than crashing the process. The final report is printed to stdout; commands that gate on failures exit non-zero if any repo failed.

Config source resolution: the `--config` value is treated as a YAML file path if it points at an existing file, otherwise as a DynamoDB table name. GitHub auth comes from `--token` or the `GITHUB_TOKEN` env var; the token is injected via git's `extraheader` config (not embedded in remote URLs) and is redacted from error output.

Commands:

- `plan-all` — `terraform plan` across all enabled repos × environments. Flags `plan` warnings when changes are detected; exits non-zero on any failure. `--concurrency` (default 5).
- `apply-all` — `terraform apply` across all enabled repos × environments. Requires `--confirm` to actually run; `--dry-run` and `--confirm` are mutually exclusive. `--concurrency` (default 3).
- `sync-template` — copies files from `--template-dir` (required) into each repo. `--safe-patterns` lists globs safe to overwrite unconditionally.
- `validate` — runs the external `platform-guardian` binary per repo (token passed via env, not flag) and reports pass/fail. `--rules-dir` selects the guardian rules directory.
- `deliver` — orchestrates an infra + website delivery for one product by ensuring its four repos (infra, website, compiler, packer) in the workspace and invoking `make go-deliver` in the infra repo. The product's repos and domain settings come from a YAML or JSON file passed via `--delivery-config` (required) — see `configs/delivery.yaml.example`; individual `--infra-repo`/`--website-repo`/`--compiler-repo`/`--packer-repo`/`--domain-name`/`--www-domain-name`/`--route53-zone-name`/`--publish-prefix` flags override the config. Requires `--confirm`.
- `version` — prints version, commit, and build time (injected at link time).

Persistent flags: `--config`, `--log-level` (debug/info/warn/error, default info), `--dry-run`, `--workspace` (default `./workspace`), `--token`, `--ui` (auto/plain/rich).

## Usage

```bash
make build
./bin/platform-runner --help

# Plan every repo in a YAML-defined fleet
./bin/platform-runner plan-all --config configs/repos.yaml

# Apply (requires explicit confirmation)
./bin/platform-runner apply-all --config configs/repos.yaml --confirm

# Load the fleet from a DynamoDB table instead of YAML
./bin/platform-runner plan-all --config my-runner-table

# Sync template files / validate against platform-guardian
./bin/platform-runner sync-template --config configs/repos.yaml --template-dir ./template
./bin/platform-runner validate --config configs/repos.yaml --rules-dir ./rules

# Deliver a product's infra + website (requires explicit confirmation)
./bin/platform-runner deliver --delivery-config ./my-product-delivery.yaml --confirm
```

Config format — see `configs/repos.yaml.example`:

```yaml
repos:
  - name: your-org/infra-repo
    environments: [dev, staging, prod]
    tf_working_dir: terraform/
    template_ref: terraform-infra
    enabled: true
  - name: your-org/legacy-service
    enabled: false
```

Delivery config format — see `configs/delivery.yaml.example` (fictional placeholder values; the real, product-specific file is never committed to this public repo — create it locally or load it from a private config store):

```yaml
infra_repo: your-org/acme-infra
website_repo: your-org/acme-website
compiler_repo: your-org/acme-website-compiler
packer_repo: your-org/acme-website-packer
domain_name: acme.example.com
www_domain_name: www.acme.example.com
route53_zone_name: acme.example.com
publish_prefix: live
```

A `.json` file with the same keys is accepted too (format is chosen by file extension). `infra_repo`, `website_repo`, `compiler_repo`, and `packer_repo` are required; the domain fields are optional and fall back to `make go-deliver`'s own defaults when left blank.

External tools expected on `PATH` at runtime: `git`, `terraform` (for plan/apply), `platform-guardian` (for validate), and `make` (for deliver, which also expects a `go-deliver` target in the infra repo's Makefile).

### Container

The `Containerfile` is a multi-stage build (builder → test → distroless `final`); the binary runs in containers as well as locally.

```bash
make container-build              # build ghcr.io/ffreis/platform-runner:dev (final stage)
make container-run ARGS="version" # run the image with arguments
make container-test               # build the test stage and run tests in-container
```

The Makefile drives the container engine through `scripts/run`, which prefers `podman` and falls back to `docker`.

## Development

Requires Go 1.25 (toolchain go1.25.11). Common targets:

```bash
make build         # compile to bin/platform-runner
make test          # go test -race (see note below)
make test-short    # unit tests, -short, no live AWS
make ci            # fmt-check + vet + lint + nakedgo + test + sec
make coverage-gate # enforce coverage floor (default 75%)
make help          # list all targets
```

`make ci` runs gofmt check, `go vet`, `golangci-lint`, the `nakedgo` analyzer (flags goroutines lacking a `defer recover()`), the test suite, and `govulncheck`. Lefthook hooks and CI scripts are pulled from the shared platform-standards repo.

> Note: `make test` intentionally omits `-shuffle=on`. Enabling it surfaces a pre-existing test-isolation issue — `captureStdout()` in `cmd/` swaps `os.Stdout` globally and races with `t.Parallel()` tests. Re-enable shuffle once `captureStdout` is made concurrency-safe.

## License

MIT. See [LICENSE](LICENSE).
