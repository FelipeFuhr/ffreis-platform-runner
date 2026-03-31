package logging

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNew_InvalidLevel(t *testing.T) {
	t.Parallel()

	_, err := New("not-a-level")
	if err == nil {
		t.Fatalf("expected error for invalid level")
	}
}

func TestNew_ValidLevel(t *testing.T) {
	t.Parallel()

	log, err := New("info")
	if err != nil {
		t.Fatalf("New(info) unexpected error: %v", err)
	}
	if log == nil {
		t.Fatalf("expected non-nil logger")
	}
	_ = log.Sync()
}

func TestWithRepo_AddsFields(t *testing.T) {
	t.Parallel()

	core, obs := observer.New(zapcore.InfoLevel)
	base := zap.New(core)

	child := WithRepo(base, "acme/repo", "dev")
	child.Info("hello")

	entries := obs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["repo"] != "acme/repo" || fields["env"] != "dev" {
		t.Fatalf("expected repo/env fields, got %v", fields)
	}
}
