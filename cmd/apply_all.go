package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ffreis/platform-runner/internal/executor"
	"github.com/ffreis/platform-runner/internal/runner"
)

var (
	applyAllConcurrency int
	applyAllConfirm     bool
)

var applyAllCmd = &cobra.Command{
	Use:   "apply-all",
	Short: "Run terraform apply for all configured repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := buildDeps(cmd)
		if err != nil {
			return err
		}
		defer d.log.Sync() //nolint:errcheck

		if d.dryRun && applyAllConfirm {
			return fmt.Errorf("--dry-run and --confirm are mutually exclusive")
		}

		r := runner.NewRunner(d.cfg, &executor.TerraformExecutor{}, runner.RunnerOptions{
			Workspace:   d.workspace,
			Concurrency: applyAllConcurrency,
			DryRun:      d.dryRun,
			Token:       d.token,
			Log:         d.log,
		})

		report, err := r.ApplyAll(cmd.Context(), applyAllConfirm)
		if err != nil {
			return fmt.Errorf("apply-all failed: %w", err)
		}

		fmt.Println(report.Summary())
		for _, res := range report.Results {
			if res.Status == runner.RepoStatusFailed {
				fmt.Printf("  FAILED  %s [%s]: %s\n", res.Repo, res.Env, res.ErrMsg)
			} else {
				fmt.Printf("  ok      %s [%s]\n", res.Repo, res.Env)
			}
		}

		return nil
	},
}

func init() {
	applyAllCmd.Flags().IntVar(&applyAllConcurrency, "concurrency", 3, "Number of concurrent workers")
	applyAllCmd.Flags().BoolVar(&applyAllConfirm, "confirm", false, "Confirm apply (required to actually run)")
}
