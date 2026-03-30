package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ffreis/platform-runner/internal/executor"
	"github.com/ffreis/platform-runner/internal/runner"
)

var validateRulesDir string

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Run platform-guardian validation for all configured repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := buildDeps(cmd)
		if err != nil {
			return err
		}
		defer d.log.Sync() //nolint:errcheck

		r := runner.NewRunner(d.cfg, &executor.TerraformExecutor{}, runner.RunnerOptions{
			RulesDir:    validateRulesDir,
			Workspace:   d.workspace,
			Concurrency: 5,
			DryRun:      d.dryRun,
			Token:       d.token,
			Log:         d.log,
		})

		report, err := r.Validate(cmd.Context())
		if err != nil {
			return fmt.Errorf("validate failed: %w", err)
		}

		fmt.Println(report.Summary())
		for _, res := range report.Results {
			if res.Status == runner.RepoStatusFailed {
				fmt.Printf("  FAILED  %s: %s\n", res.Repo, res.ErrMsg)
			} else {
				fmt.Printf("  ok      %s\n", res.Repo)
			}
		}

		if report.HasFailures() {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	validateCmd.Flags().StringVar(&validateRulesDir, "rules-dir", "", "Path to guardian rules directory")
}
