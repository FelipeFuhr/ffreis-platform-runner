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

func TestDiff_ErrorAndVariants(t *testing.T) {
	t.Run("missing template dir", func(t *testing.T) {
		_, err := Diff(filepath.Join(t.TempDir(), "missing"), t.TempDir(), nil)
		if err == nil || !strings.Contains(err.Error(), "walking template dir") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("diff one file variants", func(t *testing.T) {
		root := t.TempDir()
		templateDir := filepath.Join(root, "template")
		repoDir := filepath.Join(root, "repo")
		if err := os.MkdirAll(templateDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(templateDir): %v", err)
		}
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(repoDir): %v", err)
		}

		templatePath := filepath.Join(templateDir, "README.md")
		if err := os.WriteFile(templatePath, []byte("template"), 0o644); err != nil {
			t.Fatalf("WriteFile(template): %v", err)
		}

		diff, err := diffOneFile(templateDir, repoDir, templatePath, []string{"*.md"})
		if err != nil {
			t.Fatalf("diffOneFile() unexpected error: %v", err)
		}
		if diff.Status != DiffSourceOnly {
			t.Fatalf("status = %q, want %q", diff.Status, DiffSourceOnly)
		}

		repoPath := filepath.Join(repoDir, "README.md")
		if err := os.WriteFile(repoPath, []byte("template"), 0o644); err != nil {
			t.Fatalf("WriteFile(repo): %v", err)
		}
		diff, err = diffOneFile(templateDir, repoDir, templatePath, []string{"*.md"})
		if err != nil {
			t.Fatalf("diffOneFile() unexpected error: %v", err)
		}
		if diff.Status != DiffSame {
			t.Fatalf("status = %q, want %q", diff.Status, DiffSame)
		}

		if err := os.WriteFile(repoPath, []byte("repo"), 0o644); err != nil {
			t.Fatalf("WriteFile(repo): %v", err)
		}
		diff, err = diffOneFile(templateDir, repoDir, templatePath, []string{"*.md"})
		if err != nil {
			t.Fatalf("diffOneFile() unexpected error: %v", err)
		}
		if diff.Status != DiffSafe {
			t.Fatalf("status = %q, want %q", diff.Status, DiffSafe)
		}
	})
}
