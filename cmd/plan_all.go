package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ffreis/platform-runner/internal/executor"
	"github.com/ffreis/platform-runner/internal/runner"
)

var planAllConcurrency int

var planAllCmd = &cobra.Command{
	Use:   "plan-all",
	Short: "Run terraform plan for all configured repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := buildDeps(cmd)
		if err != nil {
			return err
		}
		defer d.log.Sync() //nolint:errcheck

		r := runner.NewRunner(d.cfg, &executor.TerraformExecutor{}, runner.RunnerOptions{
			Workspace:   d.workspace,
			Concurrency: planAllConcurrency,
			DryRun:      d.dryRun,
			Token:       d.token,
			Log:         d.log,
		})

		report, err := r.PlanAll(cmd.Context())
		if err != nil {
			return fmt.Errorf("plan-all failed: %w", err)
		}

		fmt.Println(report.Summary())
		for _, res := range report.Results {
			if res.HasChanges {
				fmt.Printf("  CHANGES  %s [%s]\n", res.Repo, res.Env)
			} else if res.Status == runner.RepoStatusFailed {
				fmt.Printf("  FAILED   %s [%s]: %s\n", res.Repo, res.Env, res.ErrMsg)
			} else {
				fmt.Printf("  ok       %s [%s]\n", res.Repo, res.Env)
			}
		}

		if report.HasFailures() {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	planAllCmd.Flags().IntVar(&planAllConcurrency, "concurrency", 5, "Number of concurrent workers")
}
