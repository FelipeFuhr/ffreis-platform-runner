package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// DefaultSafePatterns are the file patterns that may be overwritten unconditionally.
var DefaultSafePatterns = []string{"*.md", "Makefile", ".github/workflows/*.yml"}

// SyncOptions configures a template sync operation.
type SyncOptions struct {
	TemplateDir  string
	RepoDir      string
	SafePatterns []string // if nil, DefaultSafePatterns is used
	DryRun       bool
	Log          *zap.Logger
}

// SyncResult reports the outcome of a sync operation.
type SyncResult struct {
	Applied   []string // files written
	Skipped   []string // conflicts skipped
	Unchanged []string // same content
}

// Sync compares the template directory against the repo directory and applies safe updates.
func Sync(ctx context.Context, opts SyncOptions) (*SyncResult, error) {
	_ = ctx

	patterns := safePatterns(opts)
	log := loggerOrNop(opts.Log)

	diffs, err := Diff(opts.TemplateDir, opts.RepoDir, patterns)
	if err != nil {
		return nil, fmt.Errorf("diffing template against repo: %w", err)
	}

	result := &SyncResult{}

	for _, d := range diffs {
		if err := applyDiff(result, opts, d, log); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func safePatterns(opts SyncOptions) []string {
	if opts.SafePatterns != nil {
		return opts.SafePatterns
	}
	return DefaultSafePatterns
}

func loggerOrNop(log *zap.Logger) *zap.Logger {
	if log != nil {
		return log
	}
	return zap.NewNop()
}

func applyDiff(result *SyncResult, opts SyncOptions, d FileDiff, log *zap.Logger) error {
	switch d.Status {
	case DiffSourceOnly, DiffSafe:
		result.Applied = append(result.Applied, d.Path)
		return writeIfNotDryRun(opts, d, log)
	case DiffConflict:
		log.Warn("skipping conflicting file", zap.String("file", d.Path))
		result.Skipped = append(result.Skipped, d.Path)
		return nil
	case DiffSame:
		result.Unchanged = append(result.Unchanged, d.Path)
		return nil
	default:
		return nil
	}
}

func writeIfNotDryRun(opts SyncOptions, d FileDiff, log *zap.Logger) error {
	if opts.DryRun {
		return nil
	}

	dest := filepath.Join(opts.RepoDir, d.Path)
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("creating directory for %q: %w", dest, err)
	}
	if err := os.WriteFile(dest, []byte(d.Template), 0o600); err != nil {
		return fmt.Errorf("writing file %q: %w", dest, err)
	}
	log.Info("applied template file", zap.String("file", d.Path), zap.String("status", string(d.Status)))
	return nil
}
