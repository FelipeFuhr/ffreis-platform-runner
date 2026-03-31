package repos

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceDir(t *testing.T) {
	w := &Workspace{
		Repo:    "myorg/my-repo",
		RootDir: "/tmp/workspace",
	}
	got := w.Dir()
	want := filepath.Join("/tmp/workspace", "myorg-my-repo")
	if got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestWorkspaceDir_SlashReplacement(t *testing.T) {
	w := &Workspace{
		Repo:    "example-org/infra-repo",
		RootDir: "/workspace",
	}
	got := w.Dir()
	want := "/workspace/example-org-infra-repo"
	if got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestWorkspaceRemoteURL_DoesNotEmbedToken(t *testing.T) {
	w := &Workspace{
		Repo:    "myorg/my-repo",
		RootDir: "/tmp/workspace",
		Token:   "ghp_exampletoken",
	}

	url := w.remoteURL()
	if strings.Contains(url, w.Token) {
		t.Fatalf("remoteURL() must not embed token, got %q", url)
	}
	if url != "https://github.com/myorg/my-repo.git" {
		t.Fatalf("unexpected remoteURL(): got %q", url)
	}
}

func TestWorkspace_Remove_NonExistent(t *testing.T) {
	tmp, err := os.MkdirTemp("", "workspace-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	w := &Workspace{
		Repo:    "nonexistent/repo",
		RootDir: tmp,
	}

	// Dir does not exist — Remove should return nil (idempotent).
	if err := w.Remove(); err != nil {
		t.Errorf("Remove() on non-existent dir returned error: %v", err)
	}
}

func TestWorkspace_Remove_ExistingDir(t *testing.T) {
	tmp, err := os.MkdirTemp("", "workspace-remove-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	w := &Workspace{
		Repo:    "myorg/repo",
		RootDir: tmp,
	}

	// Create the directory.
	if err := os.MkdirAll(w.Dir(), 0o750); err != nil {
		t.Fatalf("creating workspace dir: %v", err)
	}

	if err := w.Remove(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Stat(w.Dir()); !os.IsNotExist(err) {
		t.Errorf("expected dir to be removed, but it still exists")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func setupExistingClone(t *testing.T, workspaceRoot string, repo string) (string, string) {
	t.Helper()

	baseDir := t.TempDir()
	remotePath := filepath.Join(baseDir, "remote.git")
	if err := os.MkdirAll(remotePath, 0o755); err != nil {
		t.Fatalf("MkdirAll(remote): %v", err)
	}
	runGit(t, remotePath, "init", "--bare")

	workTree := filepath.Join(baseDir, "worktree")
	if err := os.MkdirAll(workTree, 0o755); err != nil {
		t.Fatalf("MkdirAll(worktree): %v", err)
	}
	runGit(t, workTree, "init")
	runGit(t, workTree, "config", "user.email", "test@example.com")
	runGit(t, workTree, "config", "user.name", "Test")
	runGit(t, workTree, "commit", "--allow-empty", "-m", "initial")
	runGit(t, workTree, "branch", "-M", "main")
	runGit(t, workTree, "remote", "add", "origin", remotePath)
	runGit(t, workTree, "push", "-u", "origin", "main")

	localClone := filepath.Join(workspaceRoot, strings.ReplaceAll(repo, "/", "-"))
	runGit(t, workspaceRoot, "clone", remotePath, localClone)

	return remotePath, workTree
}

func TestWorkspaceValidate(t *testing.T) {
	t.Run("missing root dir", func(t *testing.T) {
		err := (&Workspace{Repo: "acme/repo"}).validate()
		if err == nil || err.Error() != "RootDir is required" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing repo", func(t *testing.T) {
		err := (&Workspace{RootDir: t.TempDir()}).validate()
		if err == nil || err.Error() != "Repo is required" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid repo format", func(t *testing.T) {
		err := (&Workspace{Repo: "bad/repo!", RootDir: t.TempDir()}).validate()
		if err == nil || !strings.Contains(err.Error(), "Repo must be in org/repo format") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestShouldSanitizeOriginURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "tokenized github url", url: "https://x-access-token:abc@github.com/acme/repo.git", want: true},
		{name: "credentialed github url", url: "https://user:pass@github.com/acme/repo.git", want: true},
		{name: "plain github url", url: "https://github.com/acme/repo.git", want: false},
		{name: "credentialed local url", url: "https://user:pass@example.com/repo.git", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSanitizeOriginURL(tt.url); got != tt.want {
				t.Fatalf("shouldSanitizeOriginURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestWorkspaceGitEnv(t *testing.T) {
	if err := os.Setenv("HOME", "/tmp/home-test"); err != nil {
		t.Fatalf("Setenv(): %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("HOME") })

	w := &Workspace{Token: "secret"}
	env := strings.Join(w.gitEnv(), "\n")

	if !strings.Contains(env, "PATH="+fixedGitPath) {
		t.Fatalf("expected fixed PATH, got %q", env)
	}
	if !strings.Contains(env, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("expected GIT_TERMINAL_PROMPT in env: %q", env)
	}
	if !strings.Contains(env, "HOME=/tmp/home-test") {
		t.Fatalf("expected HOME in env: %q", env)
	}
	if strings.Contains(env, "secret") {
		t.Fatalf("expected token not to appear in plain text env: %q", env)
	}
	if !strings.Contains(env, "GIT_CONFIG_VALUE_0=AUTHORIZATION: basic ") {
		t.Fatalf("expected auth header config in env: %q", env)
	}
}

func TestGitDirExists(t *testing.T) {
	root := t.TempDir()
	exists, err := gitDirExists(root)
	if err != nil {
		t.Fatalf("gitDirExists() unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected false without .git")
	}

	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git): %v", err)
	}
	exists, err = gitDirExists(root)
	if err != nil {
		t.Fatalf("gitDirExists() unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected true with .git")
	}
}

func TestSanitizeOutput(t *testing.T) {
	err := sanitizeOutput("token=secret", "secret")
	if err == nil || err.Error() != "token=***" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceEnsure_ExistingCloneFetchesLatest(t *testing.T) {
	workspaceRoot := t.TempDir()
	remotePath, workTree := setupExistingClone(t, workspaceRoot, "acme/repo")

	clonePath := filepath.Join(workspaceRoot, "acme-repo")
	runGit(t, workTree, "commit", "--allow-empty", "-m", "second")
	runGit(t, workTree, "push", "origin", "main")

	w := &Workspace{Repo: "acme/repo", RootDir: workspaceRoot}
	if err := w.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() unexpected error: %v", err)
	}

	headCmd := exec.Command("git", "-C", clonePath, "rev-parse", "HEAD")
	headOut, err := headCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse HEAD failed: %v\n%s", err, headOut)
	}
	remoteCmd := exec.Command("git", "-C", remotePath, "rev-parse", "refs/heads/main")
	remoteOut, err := remoteCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse remote failed: %v\n%s", err, remoteOut)
	}
	if strings.TrimSpace(string(headOut)) != strings.TrimSpace(string(remoteOut)) {
		t.Fatalf("clone HEAD = %s, want %s", strings.TrimSpace(string(headOut)), strings.TrimSpace(string(remoteOut)))
	}
}

func TestWorkspaceEnsure_SanitizesTokenizedOrigin(t *testing.T) {
	workspaceRoot := t.TempDir()
	_, _ = setupExistingClone(t, workspaceRoot, "acme/repo")

	clonePath := filepath.Join(workspaceRoot, "acme-repo")
	runGit(t, clonePath, "remote", "set-url", "origin", "https://x-access-token:secret@github.com/acme/repo.git")

	w := &Workspace{Repo: "acme/repo", RootDir: workspaceRoot}
	if err := w.sanitizeOrigin(context.Background(), clonePath); err != nil {
		t.Fatalf("sanitizeOrigin() unexpected error: %v", err)
	}

	cmd := exec.Command("git", "-C", clonePath, "remote", "get-url", "origin")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("remote get-url failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "https://github.com/acme/repo.git" {
		t.Fatalf("origin url = %q", got)
	}
}

func TestGitBinaryAndNewGitCmd(t *testing.T) {
	bin, err := gitBinary()
	if err != nil {
		t.Fatalf("gitBinary() unexpected error: %v", err)
	}
	if !filepath.IsAbs(bin) {
		t.Fatalf("expected absolute git path, got %q", bin)
	}

	w := &Workspace{Token: "secret"}
	cmd, err := w.newGitCmd(context.Background(), "status")
	if err != nil {
		t.Fatalf("newGitCmd() unexpected error: %v", err)
	}
	if cmd.Path != bin {
		t.Fatalf("cmd.Path = %q, want %q", cmd.Path, bin)
	}
	if len(cmd.Env) == 0 {
		t.Fatal("expected command env to be set")
	}
}

func TestResetToLatestFetched_FallsBackToFetchHead(t *testing.T) {
	workspaceRoot := t.TempDir()
	_, workTree := setupExistingClone(t, workspaceRoot, "acme/repo")
	clonePath := filepath.Join(workspaceRoot, "acme-repo")

	runGit(t, workTree, "commit", "--allow-empty", "-m", "next")
	runGit(t, workTree, "push", "origin", "main")
	runGit(t, clonePath, "fetch", "--depth", "1", "origin")
	runGit(t, clonePath, "remote", "set-head", "origin", "--delete")

	w := &Workspace{Repo: "acme/repo", RootDir: workspaceRoot}
	if err := w.resetToLatestFetched(context.Background(), clonePath); err != nil {
		t.Fatalf("resetToLatestFetched() unexpected error: %v", err)
	}
}

func TestResetToLatestFetched_OriginHead(t *testing.T) {
	workspaceRoot := t.TempDir()
	_, workTree := setupExistingClone(t, workspaceRoot, "acme/repo")
	clonePath := filepath.Join(workspaceRoot, "acme-repo")

	runGit(t, workTree, "commit", "--allow-empty", "-m", "next")
	runGit(t, workTree, "push", "origin", "main")
	runGit(t, clonePath, "fetch", "--depth", "1", "origin")

	w := &Workspace{Repo: "acme/repo", RootDir: workspaceRoot}
	if err := w.resetToLatestFetched(context.Background(), clonePath); err != nil {
		t.Fatalf("resetToLatestFetched() unexpected error: %v", err)
	}
}

func TestClone_CreateParentError(t *testing.T) {
	root := t.TempDir()
	parentFile := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	w := &Workspace{Repo: "acme/repo", RootDir: root}
	err := w.clone(context.Background(), filepath.Join(parentFile, "repo"))
	if err == nil || !strings.Contains(err.Error(), "creating workspace parent dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchAndHardResetErrors(t *testing.T) {
	w := &Workspace{Repo: "acme/repo", RootDir: t.TempDir(), Token: "secret"}

	if err := w.fetch(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil || !strings.Contains(err.Error(), "git fetch failed") {
		t.Fatalf("unexpected fetch error: %v", err)
	}

	err := w.hardReset(context.Background(), filepath.Join(t.TempDir(), "missing"), "HEAD")
	if err == nil {
		t.Fatal("expected hardReset() to fail")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected token to be redacted, got %v", err)
	}
}

func TestSanitizeOrigin_IgnoresGetURLFailure(t *testing.T) {
	w := &Workspace{Repo: "acme/repo", RootDir: t.TempDir()}
	if err := w.sanitizeOrigin(context.Background(), filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("sanitizeOrigin() should ignore get-url failure, got %v", err)
	}
}

func TestEnsure_InvalidWorkspace(t *testing.T) {
	w := &Workspace{}
	err := w.Ensure(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid workspace") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsure_CloneSuccessViaGitURLRewrite(t *testing.T) {
	root := t.TempDir()
	remotePath := filepath.Join(root, "remote.git")
	workTree := filepath.Join(root, "worktree")
	homeDir := filepath.Join(root, "home")
	workspaceRoot := filepath.Join(root, "workspace")

	if err := os.MkdirAll(remotePath, 0o755); err != nil {
		t.Fatalf("MkdirAll(remote): %v", err)
	}
	if err := os.MkdirAll(workTree, 0o755); err != nil {
		t.Fatalf("MkdirAll(worktree): %v", err)
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(home): %v", err)
	}
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace): %v", err)
	}

	runGit(t, remotePath, "init", "--bare")
	runGit(t, workTree, "init")
	runGit(t, workTree, "config", "user.email", "test@example.com")
	runGit(t, workTree, "config", "user.name", "Test")
	runGit(t, workTree, "commit", "--allow-empty", "-m", "initial")
	runGit(t, workTree, "branch", "-M", "main")
	runGit(t, workTree, "remote", "add", "origin", remotePath)
	runGit(t, workTree, "push", "-u", "origin", "main")

	gitConfig := "[url \"" + remotePath + "\"]\n\tinsteadOf = https://github.com/acme/repo.git\n"
	if err := os.WriteFile(filepath.Join(homeDir, ".gitconfig"), []byte(gitConfig), 0o644); err != nil {
		t.Fatalf("WriteFile(.gitconfig): %v", err)
	}

	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatalf("Setenv(HOME): %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	w := &Workspace{Repo: "acme/repo", RootDir: workspaceRoot}
	if err := w.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, "acme-repo", ".git")); err != nil {
		t.Fatalf("expected cloned repo, stat error: %v", err)
	}
}
