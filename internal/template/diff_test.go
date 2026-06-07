package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchesSafePattern(t *testing.T) {
	t.Run("matches by basename", func(t *testing.T) {
		matched, err := matchesSafePattern(filepath.Join(".github", "workflows", "ci.yml"), []string{"ci.yml"})
		if err != nil {
			t.Fatalf("matchesSafePattern() unexpected error: %v", err)
		}
		if !matched {
			t.Fatal("expected basename match")
		}
	})

	t.Run("invalid pattern", func(t *testing.T) {
		_, err := matchesSafePattern("README.md", []string{"["})
		if err == nil || !strings.Contains(err.Error(), `invalid pattern "["`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDiffStatusForChange(t *testing.T) {
	status, err := diffStatusForChange("README.md", []string{"*.md"})
	if err != nil {
		t.Fatalf("diffStatusForChange() unexpected error: %v", err)
	}
	if status != DiffSafe {
		t.Fatalf("status = %q, want %q", status, DiffSafe)
	}

	status, err = diffStatusForChange("main.tf", []string{"*.md"})
	if err != nil {
		t.Fatalf("diffStatusForChange() unexpected error: %v", err)
	}
	if status != DiffConflict {
		t.Fatalf("status = %q, want %q", status, DiffConflict)
	}
}

func TestDiff_MissingTemplateDir(t *testing.T) {
	_, err := Diff(filepath.Join(t.TempDir(), "missing"), t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "walking template dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiff_OneFileVariants(t *testing.T) {
	templateDir, repoDir, templatePath, repoPath := setupDiffDirs(t)

	t.Run("source only when repo file absent", func(t *testing.T) {
		diff := mustDiffOneFile(t, templateDir, repoDir, templatePath, []string{"*.md"})
		if diff.Status != DiffSourceOnly {
			t.Fatalf("status = %q, want %q", diff.Status, DiffSourceOnly)
		}
	})

	t.Run("same when repo file matches template", func(t *testing.T) {
		mustWriteFile(t, repoPath, "template")
		diff := mustDiffOneFile(t, templateDir, repoDir, templatePath, []string{"*.md"})
		if diff.Status != DiffSame {
			t.Fatalf("status = %q, want %q", diff.Status, DiffSame)
		}
	})

	t.Run("safe when repo file differs and matches safe pattern", func(t *testing.T) {
		mustWriteFile(t, repoPath, "repo")
		diff := mustDiffOneFile(t, templateDir, repoDir, templatePath, []string{"*.md"})
		if diff.Status != DiffSafe {
			t.Fatalf("status = %q, want %q", diff.Status, DiffSafe)
		}
	})
}

// setupDiffDirs creates a temporary template dir and repo dir with a single
// template file, returning the relevant paths for diff tests.
func setupDiffDirs(t *testing.T) (templateDir, repoDir, templatePath, repoPath string) {
	t.Helper()
	root := t.TempDir()
	templateDir = filepath.Join(root, "template")
	repoDir = filepath.Join(root, "repo")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templateDir): %v", err)
	}
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(repoDir): %v", err)
	}
	templatePath = filepath.Join(templateDir, "README.md")
	mustWriteFile(t, templatePath, "template")
	repoPath = filepath.Join(repoDir, "README.md")
	return templateDir, repoDir, templatePath, repoPath
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func mustDiffOneFile(t *testing.T, templateDir, repoDir, templatePath string, safePatterns []string) FileDiff {
	t.Helper()
	diff, err := diffOneFile(templateDir, repoDir, templatePath, safePatterns)
	if err != nil {
		t.Fatalf("diffOneFile() unexpected error: %v", err)
	}
	return diff
}
