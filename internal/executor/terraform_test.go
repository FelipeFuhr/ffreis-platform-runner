package executor

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

func stubTerraformScript() string {
	// Minimal, deterministic stub of the terraform CLI used by tests.
	return `#!/bin/sh
set -eu

subcmd="${1:-}"
shift || true

if [ -n "${TF_STUB_ARGS_FILE:-}" ]; then
  {
    printf '%s\n' "$subcmd"
    for a in "$@"; do printf '%s\n' "$a"; done
  } > "$TF_STUB_ARGS_FILE"
fi

if [ -n "${TF_STUB_PWD_FILE:-}" ]; then
  pwd > "$TF_STUB_PWD_FILE"
fi

case "$subcmd" in
  plan)
    if [ -n "${TF_STUB_PLAN_STDOUT:-}" ]; then printf '%s' "$TF_STUB_PLAN_STDOUT"; fi
    if [ -n "${TF_STUB_PLAN_STDERR:-}" ]; then printf '%s' "$TF_STUB_PLAN_STDERR" 1>&2; fi
    exit "${TF_STUB_PLAN_EXIT_CODE:-0}"
    ;;
  apply)
    if [ -n "${TF_STUB_APPLY_STDOUT:-}" ]; then printf '%s' "$TF_STUB_APPLY_STDOUT"; fi
    if [ -n "${TF_STUB_APPLY_STDERR:-}" ]; then printf '%s' "$TF_STUB_APPLY_STDERR" 1>&2; fi
    exit "${TF_STUB_APPLY_EXIT_CODE:-0}"
    ;;
  *)
    printf '%s\n' "unexpected subcommand: $subcmd" 1>&2
    exit 111
    ;;
esac
`
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

func TestTerraformExecutor_Plan_NoChanges(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	writeExecutable(t, binDir, "terraform", stubTerraformScript())

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)

	argsFile := filepath.Join(tmp, "args.txt")
	pwdFile := filepath.Join(tmp, "pwd.txt")
	workDir := filepath.Join(tmp, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work dir: %v", err)
	}

	env := map[string]string{
		"PATH":                binDir + string(os.PathListSeparator) + origPath,
		"TF_STUB_ARGS_FILE":   argsFile,
		"TF_STUB_PWD_FILE":    pwdFile,
		"TF_STUB_PLAN_STDOUT": "ok",
	}

	ex := &TerraformExecutor{}
	result, err := ex.Plan(context.Background(), ExecOptions{
		WorkDir: workDir,
		Env:     env,
		TFVars: map[string]string{
			"foo": "bar",
		},
	})
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("Plan() expected non-nil result")
	}
	if result.ExitCode != 0 {
		t.Fatalf("Plan() expected ExitCode=0, got %d", result.ExitCode)
	}
	if result.HasChanges {
		t.Fatalf("Plan() expected HasChanges=false")
	}
	if result.Stdout != "ok" {
		t.Fatalf("Plan() expected stdout %q, got %q", "ok", result.Stdout)
	}

	lines := readLines(t, argsFile)
	if len(lines) == 0 || lines[0] != terraformSubcommandPlan {
		t.Fatalf("expected args first line to be %q, got %v", terraformSubcommandPlan, lines)
	}
	if !slices.Contains(lines, "-out="+terraformPlanFile) {
		t.Fatalf("expected args to contain -out=%s, got %v", terraformPlanFile, lines)
	}
	if !slices.Contains(lines, "-detailed-exitcode") {
		t.Fatalf("expected args to contain -detailed-exitcode, got %v", lines)
	}
	if !slices.Contains(lines, "-input=false") {
		t.Fatalf("expected args to contain -input=false, got %v", lines)
	}
	if !slices.Contains(lines, "-var") || !slices.Contains(lines, "foo=bar") {
		t.Fatalf("expected args to include tfvar foo=bar, got %v", lines)
	}

	gotPwd := strings.TrimSpace(strings.Join(readLines(t, pwdFile), "\n"))
	if gotPwd != workDir {
		t.Fatalf("expected child working dir %q, got %q", workDir, gotPwd)
	}
}

func TestTerraformExecutor_Plan_HasChanges(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	writeExecutable(t, binDir, "terraform", stubTerraformScript())

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)

	env := map[string]string{
		"PATH":                   binDir + string(os.PathListSeparator) + origPath,
		"TF_STUB_PLAN_EXIT_CODE": "2",
	}

	ex := &TerraformExecutor{}
	result, err := ex.Plan(context.Background(), ExecOptions{Env: env})
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}
	if result == nil || !result.HasChanges || result.ExitCode != 2 {
		t.Fatalf("expected ExitCode=2 and HasChanges=true, got %+v", result)
	}
}

func TestTerraformExecutor_Plan_ExitCodeError_ReturnsResult(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	writeExecutable(t, binDir, "terraform", stubTerraformScript())

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)

	env := map[string]string{
		"PATH":                   binDir + string(os.PathListSeparator) + origPath,
		"TF_STUB_PLAN_EXIT_CODE": "1",
		"TF_STUB_PLAN_STDERR":    "bad",
	}

	ex := &TerraformExecutor{}
	result, err := ex.Plan(context.Background(), ExecOptions{Env: env})
	if err == nil {
		t.Fatalf("expected error for exit code 1")
	}
	if result == nil {
		t.Fatalf("expected non-nil result on non-zero exit")
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode=1, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "bad") {
		t.Fatalf("expected stderr to contain %q, got %q", "bad", result.Stderr)
	}
}

func TestTerraformExecutor_Plan_CommandNotFound(t *testing.T) {
	ex := &TerraformExecutor{}
	result, err := ex.Plan(context.Background(), ExecOptions{
		Env: map[string]string{
			"PATH": "/nonexistent",
		},
	})
	if err == nil {
		t.Fatalf("expected error when terraform is not found")
	}
	if result != nil {
		t.Fatalf("expected nil result on lookpath error, got %+v", result)
	}
}

func TestTerraformExecutor_Apply_DryRun(t *testing.T) {
	ex := &TerraformExecutor{}
	result, err := ex.Apply(context.Background(), ExecOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Apply(DryRun=true) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.ExitCode != 0 || result.Stdout != "" || result.Stderr != "" || result.HasChanges {
		t.Fatalf("expected zero result in dry-run mode, got %+v", result)
	}
}

func TestTerraformExecutor_Apply_RequiresConfirm(t *testing.T) {
	ex := &TerraformExecutor{}
	_, err := ex.Apply(context.Background(), ExecOptions{Confirm: false})
	if err == nil {
		t.Fatalf("expected error when Confirm=false")
	}
	if !strings.Contains(err.Error(), errApplyConfirmRequired) {
		t.Fatalf("expected error to contain %q, got %v", errApplyConfirmRequired, err)
	}
}

func TestTerraformExecutor_Apply_Success(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	writeExecutable(t, binDir, "terraform", stubTerraformScript())

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)

	argsFile := filepath.Join(tmp, "args.txt")
	env := map[string]string{
		"PATH":                 binDir + string(os.PathListSeparator) + origPath,
		"TF_STUB_ARGS_FILE":    argsFile,
		"TF_STUB_APPLY_STDOUT": "applied",
	}

	ex := &TerraformExecutor{}
	result, err := ex.Apply(context.Background(), ExecOptions{Confirm: true, Env: env})
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if result == nil || result.ExitCode != 0 || result.Stdout != "applied" {
		t.Fatalf("expected success result, got %+v", result)
	}

	lines := readLines(t, argsFile)
	if len(lines) == 0 || lines[0] != terraformSubcommandApply {
		t.Fatalf("expected args first line to be %q, got %v", terraformSubcommandApply, lines)
	}
	if !slices.Contains(lines, "-auto-approve") || !slices.Contains(lines, "-input=false") || !slices.Contains(lines, terraformPlanFile) {
		t.Fatalf("expected apply args to contain flags and plan file, got %v", lines)
	}
}

func TestTerraformExecutor_Apply_ExitCodeError_ReturnsResult(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	writeExecutable(t, binDir, "terraform", stubTerraformScript())

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)

	env := map[string]string{
		"PATH":                    binDir + string(os.PathListSeparator) + origPath,
		"TF_STUB_APPLY_EXIT_CODE": "1",
		"TF_STUB_APPLY_STDERR":    "nope",
	}

	ex := &TerraformExecutor{}
	result, err := ex.Apply(context.Background(), ExecOptions{Confirm: true, Env: env})
	if err == nil {
		t.Fatalf("expected error for exit code 1")
	}
	if result == nil || result.ExitCode != 1 {
		t.Fatalf("expected non-nil result with ExitCode=1, got %+v", result)
	}
	if !strings.Contains(result.Stderr, "nope") {
		t.Fatalf("expected stderr to contain %q, got %q", "nope", result.Stderr)
	}
}

func TestBuildEnv_DoesNotInheritShellEnv(t *testing.T) {
	env := buildEnv(map[string]string{
		"FOO": "bar",
	})
	if len(env) != 1 || env[0] != "FOO=bar" {
		t.Fatalf("unexpected env slice: %v", env)
	}
}
