package guardian

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, dir, name, contents string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable temp %s: %v", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		t.Fatalf("rename executable %s -> %s: %v", tmpPath, path, err)
	}
	return path
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func TestGuardianRunner_Check_PassedNoFailures(t *testing.T) {
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "args.txt")
	tokenFile := filepath.Join(tmp, "token.txt")

	stub := `#!/bin/sh
set -eu
{
  for a in "$@"; do printf '%s\n' "$a"; done
} > "` + argsFile + `"
printf '%s' "${GITHUB_TOKEN:-}" > "` + tokenFile + `"
printf '%s' '{"passed":true,"failures":[]}'
`
	bin := writeExecutable(t, tmp, "guardian", stub)

	r := &GuardianRunner{Binary: bin, RulesDir: "/rules", Token: "tok123"}
	result, err := r.Check(context.Background(), "acme/repo")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if result == nil || !result.Passed {
		t.Fatalf("expected Passed=true, got %+v", result)
	}
	if got := strings.TrimSpace(strings.Join(readLines(t, tokenFile), "\n")); got != "tok123" {
		t.Fatalf("expected GITHUB_TOKEN to be %q, got %q", "tok123", got)
	}

	args := readLines(t, argsFile)
	if !slices.Contains(args, "check") || !slices.Contains(args, "--repo") || !slices.Contains(args, "acme/repo") {
		t.Fatalf("unexpected args: %v", args)
	}
	if !slices.Contains(args, "--rules") || !slices.Contains(args, "/rules") {
		t.Fatalf("unexpected args: %v", args)
	}
	if !slices.Contains(args, "--format") || !slices.Contains(args, "json") {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestGuardianRunner_Check_FailuresForceFail(t *testing.T) {
	tmp := t.TempDir()
	stub := `#!/bin/sh
set -eu
printf '%s' '{"passed":true,"failures":[{"x":1}]}'
`
	bin := writeExecutable(t, tmp, "guardian", stub)

	r := &GuardianRunner{Binary: bin, RulesDir: "/rules"}
	result, err := r.Check(context.Background(), "acme/repo")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if result == nil || result.Passed {
		t.Fatalf("expected Passed=false when failures exist, got %+v", result)
	}
}

func TestGuardianRunner_Check_InvalidJSON_TreatedAsFailure(t *testing.T) {
	tmp := t.TempDir()
	stub := `#!/bin/sh
set -eu
printf '%s' 'not-json'
`
	bin := writeExecutable(t, tmp, "guardian", stub)

	r := &GuardianRunner{Binary: bin, RulesDir: "/rules"}
	result, err := r.Check(context.Background(), "acme/repo")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if result == nil || result.Passed {
		t.Fatalf("expected Passed=false for invalid json, got %+v", result)
	}
	if !strings.Contains(result.ErrMsg, "parsing guardian output") {
		t.Fatalf("expected ErrMsg to mention parsing, got %q", result.ErrMsg)
	}
}

func TestGuardianRunner_Check_CommandFailure_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	stub := `#!/bin/sh
set -eu
printf '%s\n' 'oops' 1>&2
exit 7
`
	bin := writeExecutable(t, tmp, "guardian", stub)

	r := &GuardianRunner{Binary: bin, RulesDir: "/rules"}
	result, err := r.Check(context.Background(), "acme/repo")
	if err == nil {
		t.Fatalf("expected error on non-zero exit")
	}
	if result == nil || result.Passed {
		t.Fatalf("expected Passed=false on error, got %+v", result)
	}
	if !strings.Contains(result.ErrMsg, "oops") {
		t.Fatalf("expected ErrMsg to contain stderr, got %q", result.ErrMsg)
	}
}
