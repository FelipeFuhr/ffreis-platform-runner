package executor

import "context"

// ExecOptions configures a single executor invocation.
type ExecOptions struct {
	WorkDir string
	Env     map[string]string // AWS credentials etc; shell env NOT inherited
	TFVars  map[string]string
	DryRun  bool
	Confirm bool // for apply: require explicit confirmation flag
}

// ExecResult holds the output of a completed executor run.
type ExecResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	HasChanges bool // true if terraform plan detected changes
}

// Executor runs a command for a repository+environment and returns structured output.
type Executor interface {
	Plan(ctx context.Context, opts ExecOptions) (*ExecResult, error)
	Apply(ctx context.Context, opts ExecOptions) (*ExecResult, error)
}
