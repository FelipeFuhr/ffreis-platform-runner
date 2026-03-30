package template

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DiffStatus describes the relationship between a template file and a repo file.
type DiffStatus string

const (
	DiffSame       DiffStatus = "same"
	DiffSourceOnly DiffStatus = "source_only" // file in template but not in repo
	DiffConflict   DiffStatus = "conflict"    // both have file, content differs, not auto-safe
	DiffSafe       DiffStatus = "safe"        // content differs but file is in safe-update list
)

// FileDiff describes the diff status for a single file.
type FileDiff struct {
	Path     string
	Status   DiffStatus
	Template string // template content
	Repo     string // repo content (empty if source_only)
}

// Diff compares the template directory against the repo directory.
// safePatterns is a list of glob patterns for files that may be overwritten unconditionally.
func Diff(templateDir, repoDir string, safePatterns []string) ([]FileDiff, error) {
	var diffs []FileDiff

	err := filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		diff, err := diffOneFile(templateDir, repoDir, path, safePatterns)
		if err != nil {
			return err
		}
		diffs = append(diffs, diff)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking template dir %q: %w", templateDir, err)
	}

	return diffs, nil
}

func diffOneFile(templateDir, repoDir, templatePath string, safePatterns []string) (FileDiff, error) {
	// Relative path within the template.
	rel, err := filepath.Rel(templateDir, templatePath)
	if err != nil {
		return FileDiff{}, fmt.Errorf("computing relative path: %w", err)
	}

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return FileDiff{}, fmt.Errorf("reading template file %q: %w", templatePath, err)
	}

	repoPath := filepath.Join(repoDir, rel)
	repoContent, err := os.ReadFile(repoPath)
	if os.IsNotExist(err) {
		return FileDiff{
			Path:     rel,
			Status:   DiffSourceOnly,
			Template: string(templateContent),
		}, nil
	}
	if err != nil {
		return FileDiff{}, fmt.Errorf("reading repo file %q: %w", repoPath, err)
	}

	if bytes.Equal(templateContent, repoContent) {
		return FileDiff{
			Path:     rel,
			Status:   DiffSame,
			Template: string(templateContent),
			Repo:     string(repoContent),
		}, nil
	}

	status, err := diffStatusForChange(rel, safePatterns)
	if err != nil {
		return FileDiff{}, err
	}

	return FileDiff{
		Path:     rel,
		Status:   status,
		Template: string(templateContent),
		Repo:     string(repoContent),
	}, nil
}

func diffStatusForChange(relPath string, safePatterns []string) (DiffStatus, error) {
	// Content differs — check if path matches a safe pattern.
	safe, err := matchesSafePattern(relPath, safePatterns)
	if err != nil {
		return "", fmt.Errorf("matching safe patterns for %q: %w", relPath, err)
	}
	if safe {
		return DiffSafe, nil
	}
	return DiffConflict, nil
}

// matchesSafePattern returns true if the path matches any of the safe glob patterns.
func matchesSafePattern(path string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, path)
		if err != nil {
			return false, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if matched {
			return true, nil
		}
		// Also match base name alone for patterns without directory separators.
		base := filepath.Base(path)
		if base != path {
			matched, err = filepath.Match(pattern, base)
			if err != nil {
				return false, fmt.Errorf("invalid pattern %q: %w", pattern, err)
			}
			if matched {
				return true, nil
			}
		}
	}
	return false, nil
}
