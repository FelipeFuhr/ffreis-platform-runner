package runner

import "testing"

func TestRunReportSummary(t *testing.T) {
	report := &RunReport{
		Results: []RepoResult{
			{Status: RepoStatusSuccess},
			{Status: RepoStatusFailed},
			{Status: RepoStatusSkipped},
			{Status: RepoStatusSuccess},
		},
	}

	if got := report.Summary(); got != "2 succeeded, 1 failed, 1 skipped" {
		t.Fatalf("Summary() = %q", got)
	}
}

func TestRunReportHasFailures(t *testing.T) {
	t.Run("true when any failed", func(t *testing.T) {
		report := &RunReport{Results: []RepoResult{{Status: RepoStatusSuccess}, {Status: RepoStatusFailed}}}
		if !report.HasFailures() {
			t.Fatal("expected HasFailures() to be true")
		}
	})

	t.Run("false when none failed", func(t *testing.T) {
		report := &RunReport{Results: []RepoResult{{Status: RepoStatusSuccess}, {Status: RepoStatusSkipped}}}
		if report.HasFailures() {
			t.Fatal("expected HasFailures() to be false")
		}
	})
}
