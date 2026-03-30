package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/ffreis/platform-runner/internal/config"
	"github.com/ffreis/platform-runner/internal/executor"
)

const (
	testRepoA = "example-org/repo-a"
	testRepoB = "example-org/repo-b"

	testEnvDev  = "dev"
	testEnvProd = "prod"

	gitTestUserEmail    = "test@test.com"
	gitTestUserName     = "Test"
	gitInitialCommitMsg = "init"
	gitDefaultBranch    = "master"
)

// mockExecutor is a test double for executor.Executor.
type mockExecutor struct {
	planResult  *executor.ExecResult
	planErr     error
	applyResult *executor.ExecResult
	applyErr    error
}

func (m *mockExecutor) Plan(_ context.Context, _ executor.ExecOptions) (*executor.ExecResult, error) {
	return m.planResult, m.planErr
}

func (m *mockExecutor) Apply(_ context.Context, opts executor.ExecOptions) (*executor.ExecResult, error) {
	if !opts.Confirm {
		return nil, errors.New(errApplyConfirmRequired)
	}
	if opts.DryRun {
		return &executor.ExecResult{}, nil
	}
	return m.applyResult, m.applyErr
}

// countingMockExecutor delegates Plan to a function.
type countingMockExecutor struct {
	planFn func(opts executor.ExecOptions) (*executor.ExecResult, error)
}

func (m *countingMockExecutor) Plan(_ context.Context, opts executor.ExecOptions) (*executor.ExecResult, error) {
	return m.planFn(opts)
}

func (m *countingMockExecutor) Apply(_ context.Context, opts executor.ExecOptions) (*executor.ExecResult, error) {
	if !opts.Confirm {
		return nil, errors.New(errApplyConfirmRequired)
	}
	return &executor.ExecResult{}, nil
}

func testLogger() *zap.Logger {
	return zap.NewNop()
}

func twoEnabledRepos() []config.RepoConfig {
	return []config.RepoConfig{
		{Name: testRepoA, Environments: []string{testEnvDev}, Enabled: true},
		{Name: testRepoB, Environments: []string{testEnvProd}, Enabled: true},
	}
}

// repoSafeName converts "org/repo" to "org-repo".
func repoSafeName(repo string) string {
	return strings.ReplaceAll(repo, "/", "-")
}

// runCmd is a small helper to run git commands, fataling on error.
func runCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("runCmd %v in %s: %v\n%s", args, dir, err, out)
	}
}

// setupLocalRemote creates a bare git repo (the "remote") and clones it into the
// workspace directory for the given repo. This allows Workspace.Ensure to run
// git fetch + git reset without network access.
func setupLocalRemote(t *testing.T, baseDir, wsDir string, rc config.RepoConfig) {
	t.Helper()

	// Create the bare "remote" repo.
	remotePath := baseDir + "/remotes/" + repoSafeName(rc.Name) + ".git"
	if err := os.MkdirAll(remotePath, 0o750); err != nil {
		t.Fatalf("mkdir remote: %v", err)
	}
	runCmd(t, remotePath, "git", "init", "--bare")

	// Create a temp work tree so we can make an initial commit.
	workTree := baseDir + "/worktree/" + repoSafeName(rc.Name)
	if err := os.MkdirAll(workTree, 0o750); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	runCmd(t, workTree, "git", "init")
	runCmd(t, workTree, "git", "config", "user.email", gitTestUserEmail)
	runCmd(t, workTree, "git", "config", "user.name", gitTestUserName)
	runCmd(t, workTree, "git", "commit", "--allow-empty", "-m", gitInitialCommitMsg)
	runCmd(t, workTree, "git", "remote", "add", "origin", remotePath)
	runCmd(t, workTree, "git", "push", "origin", "HEAD:"+gitDefaultBranch)

	// Clone from the local bare remote into the expected workspace location.
	cloneTarget := wsDir + "/" + repoSafeName(rc.Name)
	runCmd(t, wsDir, "git", "clone", remotePath, cloneTarget)
}

// preCreateWorkspace sets up local git repos with valid origins in the workspace.
func preCreateWorkspace(t *testing.T, wsDir string, repos []config.RepoConfig) {
	t.Helper()
	baseDir := t.TempDir()
	for _, rc := range repos {
		setupLocalRemote(t, baseDir, wsDir, rc)
	}
}

func TestPlanAll_AllSuccess(t *testing.T) {
	mock := &mockExecutor{
		planResult: &executor.ExecResult{ExitCode: 0, HasChanges: false},
	}

	cfg := twoEnabledRepos()
	wsDir := t.TempDir()
	preCreateWorkspace(t, wsDir, cfg)

	r := NewRunner(cfg, mock, RunnerOptions{
		Workspace:   wsDir,
		Concurrency: 2,
		Log:         testLogger(),
	})

	report, err := r.PlanAll(context.Background())
	if err != nil {
		t.Fatalf("PlanAll() unexpected top-level error: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if len(report.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(report.Results))
	}
	if report.HasFailures() {
		for _, res := range report.Results {
			if res.Status == RepoStatusFailed {
				t.Logf("failure: repo=%s err=%s", res.Repo, res.ErrMsg)
			}
		}
		t.Errorf("expected no failures")
	}
}

func TestPlanAll_OneFailure_OthersRun(t *testing.T) {
	callCount := 0
	mock := &countingMockExecutor{
		planFn: func(_ executor.ExecOptions) (*executor.ExecResult, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("simulated plan failure")
			}
			return &executor.ExecResult{ExitCode: 0}, nil
		},
	}

	cfg := []config.RepoConfig{
		{Name: testRepoA, Environments: []string{testEnvDev}, Enabled: true},
		{Name: testRepoB, Environments: []string{testEnvProd}, Enabled: true},
	}

	wsDir := t.TempDir()
	preCreateWorkspace(t, wsDir, cfg)

	r := NewRunner(cfg, mock, RunnerOptions{
		Workspace:   wsDir,
		Concurrency: 1, // sequential so failure ordering is deterministic
		Log:         testLogger(),
	})

	report, err := r.PlanAll(context.Background())
	if err != nil {
		t.Fatalf("PlanAll() unexpected top-level error: %v", err)
	}
	if len(report.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(report.Results))
	}
	if !report.HasFailures() {
		t.Errorf("expected HasFailures=true (one plan failed)")
	}
}

func TestPlanAll_SkipsDisabledRepos(t *testing.T) {
	mock := &mockExecutor{
		planResult: &executor.ExecResult{ExitCode: 0},
	}

	cfg := []config.RepoConfig{
		{Name: "example-org/enabled-repo", Environments: []string{testEnvDev}, Enabled: true},
		{Name: "example-org/disabled-repo", Environments: []string{testEnvProd}, Enabled: false},
	}

	wsDir := t.TempDir()
	preCreateWorkspace(t, wsDir, []config.RepoConfig{cfg[0]})

	r := NewRunner(cfg, mock, RunnerOptions{
		Workspace:   wsDir,
		Concurrency: 2,
		Log:         testLogger(),
	})

	report, err := r.PlanAll(context.Background())
	if err != nil {
		t.Fatalf("PlanAll() unexpected top-level error: %v", err)
	}

	if len(report.Results) != 1 {
		t.Errorf("expected 1 result (disabled repo skipped), got %d", len(report.Results))
	}
	for _, res := range report.Results {
		if res.Repo == "example-org/disabled-repo" {
			t.Errorf("disabled repo should not appear in results")
		}
	}
}

func TestApplyAll_RequiresConfirm(t *testing.T) {
	mock := &mockExecutor{
		applyResult: &executor.ExecResult{ExitCode: 0},
	}

	r := NewRunner(twoEnabledRepos(), mock, RunnerOptions{
		Workspace:   t.TempDir(),
		Concurrency: 2,
		Log:         testLogger(),
	})

	_, err := r.ApplyAll(context.Background(), false)
	if err == nil {
		t.Fatal("expected error when confirm=false, got nil")
	}
}

func TestSyncTemplate_DryRun(t *testing.T) {
	mock := &mockExecutor{}

	cfg := []config.RepoConfig{
		{Name: testRepoA, Environments: []string{testEnvDev}, Enabled: true},
	}

	wsDir := t.TempDir()
	templateDir := t.TempDir()
	preCreateWorkspace(t, wsDir, cfg)

	r := NewRunner(cfg, mock, RunnerOptions{
		TemplateDir: templateDir,
		Workspace:   wsDir,
		Concurrency: 1,
		DryRun:      true,
		Log:         testLogger(),
	})

	report, err := r.SyncTemplate(context.Background())
	if err != nil {
		t.Fatalf("SyncTemplate() unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if len(report.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(report.Results))
	}
}
