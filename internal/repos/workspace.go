package repos

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// remoteURL builds the authenticated remote URL without logging the token.
func (w *Workspace) remoteURL() string {
	return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", w.Token, w.Repo)
}

// Ensure clones the repo if not present, or fetches and resets to HEAD if already cloned.
func (w *Workspace) Ensure(ctx context.Context) error {
	dir := w.Dir()

	_, err := os.Stat(filepath.Join(dir, ".git"))
	if os.IsNotExist(err) {
		// Clone fresh.
		if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
			return fmt.Errorf("creating workspace parent dir: %w", err)
		}
		cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", w.remoteURL(), dir)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone failed: %w", sanitizeOutput(string(out), w.Token))
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking workspace dir %q: %w", dir, err)
	}

	// Repo already cloned — fetch and reset.
	fetchCmd := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth", "1", "origin")
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w", sanitizeOutput(string(out), w.Token))
	}

	resetCmd := exec.CommandContext(ctx, "git", "-C", dir, "reset", "--hard", "HEAD")
	if out, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %w", sanitizeOutput(string(out), w.Token))
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
