package guardian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// GuardianRunner runs platform-guardian checks against repositories.
type GuardianRunner struct {
	Binary   string // path to platform-guardian binary, or "platform-guardian"
	RulesDir string // path to rules directory
	Token    string // GitHub token (never logged)
}

// GuardianResult holds the outcome of a guardian check for a single repo.
type GuardianResult struct {
	Repo   string
	Passed bool
	Output string
	ErrMsg string
}

// guardianOutput represents the JSON output from platform-guardian.
type guardianOutput struct {
	Failures []interface{} `json:"failures"`
	Passed   bool          `json:"passed"`
}

// Check runs `platform-guardian check` for the given repo and returns the result.
func (g *GuardianRunner) Check(ctx context.Context, repo string) (*GuardianResult, error) {
	binary := g.Binary
	if binary == "" {
		binary = "platform-guardian"
	}

	// Token is injected via environment variable, NOT via --token flag.
	// CLI arguments are visible in /proc/<pid>/cmdline to other processes
	// running as the same user; environment variables are not.
	args := []string{
		"check",
		"--repo", repo,
		"--rules", g.RulesDir,
		"--format", "json",
	}

	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	// exec.Command does not invoke a shell; binary and args are passed as-is.
	cmd := exec.CommandContext(ctx, binary, args...)
	// Build a minimal environment: only GITHUB_TOKEN.
	// Parent shell environment is not forwarded to prevent accidental
	// credential or path leakage into the child process.
	env := []string{}
	if g.Token != "" {
		env = append(env, "GITHUB_TOKEN="+g.Token)
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &GuardianResult{
		Repo:   repo,
		Output: stdout.String(),
	}

	if err != nil {
		result.Passed = false
		result.ErrMsg = stderr.String()
		return result, fmt.Errorf("running platform-guardian: %w", err)
	}

	// Parse JSON output to determine if any failures exist.
	var output guardianOutput
	if jsonErr := json.Unmarshal(stdout.Bytes(), &output); jsonErr != nil {
		// If we can't parse JSON, treat as failure.
		result.Passed = false
		result.ErrMsg = fmt.Sprintf("parsing guardian output: %v", jsonErr)
		return result, nil
	}

	result.Passed = output.Passed && len(output.Failures) == 0

	return result, nil
}
