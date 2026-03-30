package executor

import (
	"context"
	"errors"
	"testing"
)

// mockExecutor is a test double implementing Executor.
type mockExecutor struct {
	planResult  *ExecResult
	planErr     error
	applyResult *ExecResult
	applyErr    error
	applyCalled bool
}

func (m *mockExecutor) Plan(_ context.Context, _ ExecOptions) (*ExecResult, error) {
	return m.planResult, m.planErr
}

func (m *mockExecutor) Apply(_ context.Context, opts ExecOptions) (*ExecResult, error) {
	if opts.DryRun {
		// Simulate dry-run: do NOT set applyCalled.
		return &ExecResult{}, nil
	}
	if !opts.Confirm {
		return nil, errors.New(errApplyConfirmRequired)
	}
	m.applyCalled = true
	return m.applyResult, m.applyErr
}

func TestMockPlan_HasChanges(t *testing.T) {
	mock := &mockExecutor{
		planResult: &ExecResult{
			ExitCode:   2,
			HasChanges: true,
		},
	}

	result, err := mock.Plan(context.Background(), ExecOptions{})
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}
	if result.ExitCode != 2 {
		t.Errorf("expected ExitCode=2, got %d", result.ExitCode)
	}
	if !result.HasChanges {
		t.Errorf("expected HasChanges=true")
	}
}

func TestMockApply_DryRun_NoExec(t *testing.T) {
	mock := &mockExecutor{
		applyResult: &ExecResult{ExitCode: 0},
	}

	opts := ExecOptions{DryRun: true, Confirm: true}
	result, err := mock.Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply(DryRun=true) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// applyCalled should be false because dry-run short-circuits.
	if mock.applyCalled {
		t.Errorf("expected applyCalled=false in dry-run mode")
	}
}

func TestMockApply_RequiresConfirm(t *testing.T) {
	mock := &mockExecutor{
		applyResult: &ExecResult{ExitCode: 0},
	}

	opts := ExecOptions{DryRun: false, Confirm: false}
	_, err := mock.Apply(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error when Confirm=false, got nil")
	}
	if mock.applyCalled {
		t.Errorf("expected applyCalled=false when Confirm=false")
	}
}
