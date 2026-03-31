package repos

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var repoNamePattern = regexp.MustCompile(`\A[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+\z`)

// Workspace manages the local clone of a repository.
type Workspace struct {
	Repo    string // "org/repo"
	RootDir string // base workspace directory
	Token   string // GitHub token for clone auth (never logged)
}

// Dir returns the local path for this repo's clone.
// Slashes in the repo name are replaced with dashes.
func (w *Workspace) Dir() string {
	safeName := strings.ReplaceAll(w.Repo, "/", "-")
	return filepath.Join(w.RootDir, safeName)
}

// remoteURL builds the repository URL without embedding credentials.
func (w *Workspace) remoteURL() string {
	return fmt.Sprintf("https://github.com/%s.git", w.Repo)
}

const fixedGitPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

var (
	gitBinaryOnce sync.Once
	gitBinaryPath string
	gitBinaryErr  error
)

func gitBinary() (string, error) {
	gitBinaryOnce.Do(func() {
		// Intentionally search only fixed system directories.
		for _, dir := range []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"} {
			path := filepath.Join(dir, "git")
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.IsDir() {
				continue
			}
			if info.Mode()&0o111 == 0 {
				continue
			}
			gitBinaryPath = path
			return
		}
		gitBinaryErr = fmt.Errorf("git binary not found in fixed system PATH")
	})
	return gitBinaryPath, gitBinaryErr
}

func (w *Workspace) gitEnv() []string {
	// Do not inherit the parent environment. In particular, keep PATH fixed so
	// subprocess execution cannot be influenced by writable directories.
	env := []string{
		"PATH=" + fixedGitPath,
		"GIT_TERMINAL_PROMPT=0",
	}
	if home := os.Getenv("HOME"); home != "" {
		env = append(env, "HOME="+home)
	}

	if w.Token == "" {
		return env
	}

	// Provide authentication without embedding the token in the remote URL.
	// This avoids persisting secrets to .git/config and reduces accidental leaks.
	//
	// NOTE: environment variables may still be readable by processes running as
	// the same user, but this is strictly better than storing the token in URLs.
	auth := "x-access-token:" + w.Token
	encoded := base64.StdEncoding.EncodeToString([]byte(auth))

	env = append(env,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
		"GIT_CONFIG_VALUE_0=AUTHORIZATION: basic "+encoded,
	)
	return env
}

func (w *Workspace) validate() error {
	if w.RootDir == "" {
		return fmt.Errorf("RootDir is required")
	}
	if w.Repo == "" {
		return fmt.Errorf("Repo is required")
	}
	if !repoNamePattern.MatchString(w.Repo) {
		return fmt.Errorf("Repo must be in org/repo format with safe characters: %q", w.Repo)
	}
	return nil
}

func shouldSanitizeOriginURL(originURL string) bool {
	if strings.Contains(originURL, "x-access-token:") {
		return true
	}
	// URLs can embed credentials as: https://user:pass@host/...
	if strings.Contains(originURL, "://") && strings.Contains(originURL, "@") {
		return strings.Contains(originURL, "github.com")
	}
	return false
}

func (w *Workspace) newGitCmd(ctx context.Context, args ...string) (*exec.Cmd, error) {
	bin, err := gitBinary()
	if err != nil {
		return nil, err
	}
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	// `bin` is resolved from a fixed allowlist of system directories in `gitBinary()`.
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = w.gitEnv()
	return cmd, nil
}

func (w *Workspace) clone(ctx context.Context, dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return fmt.Errorf("creating workspace parent dir: %w", err)
	}
	cmd, err := w.newGitCmd(ctx, "clone", "--depth", "1", w.remoteURL(), dir)
	if err != nil {
		return err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w", sanitizeOutput(string(out), w.Token))
	}
	return nil
}

func (w *Workspace) sanitizeOrigin(ctx context.Context, dir string) error {
	// If the repo was cloned previously with an embedded token URL, sanitize it.
	// Do not overwrite non-GitHub origins (tests use local bare remotes).
	getURLCmd, err := w.newGitCmd(ctx, "-C", dir, "remote", "get-url", "origin")
	if err != nil {
		return err
	}
	out, err := getURLCmd.CombinedOutput()
	if err != nil {
		// Best-effort: if we can't read the origin, don't fail the whole Ensure.
		return nil
	}

	originURL := strings.TrimSpace(string(out))
	if !shouldSanitizeOriginURL(originURL) {
		return nil
	}

	setURLCmd, err := w.newGitCmd(ctx, "-C", dir, "remote", "set-url", "origin", w.remoteURL())
	if err != nil {
		return err
	}
	if out, err := setURLCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git remote set-url failed: %w", sanitizeOutput(string(out), w.Token))
	}
	return nil
}

func (w *Workspace) fetch(ctx context.Context, dir string) error {
	fetchCmd, err := w.newGitCmd(ctx, "-C", dir, "fetch", "--depth", "1", "origin")
	if err != nil {
		return err
	}
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w", sanitizeOutput(string(out), w.Token))
	}
	return nil
}

func (w *Workspace) hardReset(ctx context.Context, dir, target string) error {
	resetCmd, err := w.newGitCmd(ctx, "-C", dir, "reset", "--hard", target)
	if err != nil {
		return err
	}
	if out, err := resetCmd.CombinedOutput(); err != nil {
		return sanitizeOutput(string(out), w.Token)
	}
	return nil
}

func (w *Workspace) resetToLatestFetched(ctx context.Context, dir string) error {
	if err := w.hardReset(ctx, dir, "origin/HEAD"); err == nil {
		return nil
	}
	// Fallback: FETCH_HEAD is populated by the previous fetch invocation.
	// Some repos may not have origin/HEAD configured locally.
	if err := w.hardReset(ctx, dir, "FETCH_HEAD"); err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}
	return nil
}

func gitDirExists(dir string) (bool, error) {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Ensure clones the repo if not present, or fetches and resets to HEAD if already cloned.
func (w *Workspace) Ensure(ctx context.Context) error {
	if err := w.validate(); err != nil {
		return fmt.Errorf("invalid workspace: %w", err)
	}

	dir := w.Dir()

	exists, err := gitDirExists(dir)
	if err != nil {
		return fmt.Errorf("checking workspace dir %q: %w", dir, err)
	}
	if !exists {
		return w.clone(ctx, dir)
	}

	if err := w.sanitizeOrigin(ctx, dir); err != nil {
		return err
	}
	if err := w.fetch(ctx, dir); err != nil {
		return err
	}
	if err := w.resetToLatestFetched(ctx, dir); err != nil {
		return err
	}

	return nil
}

// Remove deletes the local clone directory. Returns nil if the directory does not exist.
func (w *Workspace) Remove() error {
	dir := w.Dir()
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing workspace dir %q: %w", dir, err)
	}
	return nil
}

// sanitizeOutput wraps an error message, ensuring the token is not included.
func sanitizeOutput(output, token string) error {
	if token != "" {
		output = strings.ReplaceAll(output, token, "***")
	}
	return fmt.Errorf("%s", output)
}
