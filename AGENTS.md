# Agent Context

**This repo:** `ffreis-platform-runner` — platform runner service that executes
platform tasks (deployments, maintenance, health checks) with isolation, logging,
and status reporting. Containerized.

## Non-obvious facts

- **Logs to stderr, results to stdout.** Never mix diagnostic text with result output.

- **Includes a Containerfile** for OCI image builds — the binary is intended to run
  in containers, not only locally.

- **Test-spawned `git` subprocesses must set an explicit minimal `cmd.Env`**
  (`minimalGitEnv()` in `internal/repos/workspace_test.go` and
  `internal/runner/runner_test.go`; inlined in `cmd/commands_test.go`) —
  never the inherited ambient environment. Under a real git hook (e.g. this
  repo's own `pre-push`), `GIT_DIR`/`GIT_WORK_TREE` are set in the parent
  process and leak into any child `git` that inherits `os.Environ()`,
  redirecting fixture-repo operations onto the real outer repo instead of
  the test's temp dir (observed: `fatal: cannot force update the branch
  'main' used by worktree...`). Mirrors the isolation `Workspace.gitEnv()`
  already applies to the production code path. Confirmed once as a real
  incident: a few failed push attempts before this fix actually wrote ~34
  empty commits onto a branch and a stray `[user]` block into the shared
  `.git/config` — both had to be cleaned up by hand (`git reset --soft`,
  `git config --unset`). If you ever see unexplained empty commits or a
  wrong committer identity in this repo, check for this leak first.

- **`.pre-commit-config.yaml`'s `golangci-lint` pin must track the binary
  version `make lint`/CI actually use**, not drift independently — the
  fleet-shared lefthook `pre-commit-compat` step runs `.pre-commit-config.yaml`
  as-is via `pre-commit run --hook-stage pre-commit` on any commit touching
  `.go` files. A stale v1.x pin can't parse this repo's v2 `.golangci.yml`
  schema (`version: "2"`) and fails "Can't read config" even though `make
  lint` itself passes clean.

- **A hardcoded `go-version:`/`GO_VERSION` in workflow YAML does not track
  `go.mod`'s `toolchain` directive.** Bumping `go.mod` alone to clear a
  govulncheck stdlib CVE is not enough if any workflow (`ci.yml`,
  `devops-go-ci.yml`, `code-quality.yml`, `devops-security.yml`) also
  hardcodes the version — and the `Containerfile`'s `FROM golang:X-alpine`
  builder-stage tag is a separate pin again, one Trivy scans independently
  of govulncheck on the built image. All of these must move together.

## Structure

```
cmd/platform-runner/   ← Cobra CLI entry point
cmd/                   ← task execution commands
Containerfile
```

## Build/run

```bash
make build
./bin/platform-runner <task>
```

## Public repo — private-repo hygiene

This is a **public** GitHub repository. When writing commit messages, PR titles,
PR descriptions, or any other user-visible text, **never name private repos** —
website content, inventory, infra, Lambda, or data repos that are not publicly
listed. Use generic terms instead: "the fleet inventory", "a private consumer",
"internal infra", "private data repo", etc.

## Keeping this file current

- **If you discover a fact not reflected here:** add it before finishing your task.
- **If something here is wrong or outdated:** correct it in the same commit as the code change.
- **If you rename a file, command, or concept referenced here:** update the reference.
