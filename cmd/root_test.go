package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestBuildDeps_LoadsConfigAndTokenFallback(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
repos:
  - name: acme/repo
    environments: ["dev"]
    enabled: true
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	origConfig := flagConfig
	origLogLevel := flagLogLevel
	origDryRun := flagDryRun
	origWorkspace := flagWorkspace
	origToken := flagToken
	t.Cleanup(func() {
		flagConfig = origConfig
		flagLogLevel = origLogLevel
		flagDryRun = origDryRun
		flagWorkspace = origWorkspace
		flagToken = origToken
	})

	if err := os.Setenv("GITHUB_TOKEN", "envtok"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("GITHUB_TOKEN") })

	flagConfig = cfgPath
	flagLogLevel = "info"
	flagDryRun = true
	flagWorkspace = "/tmp/ws"
	flagToken = ""

	c := &cobra.Command{}
	c.SetContext(context.Background())
	d, err := buildDeps(c)
	if err != nil {
		t.Fatalf("buildDeps() unexpected error: %v", err)
	}
	if d == nil {
		t.Fatalf("expected non-nil deps")
	}
	if d.token != "envtok" {
		t.Fatalf("expected token from env, got %q", d.token)
	}
	if !d.dryRun || d.workspace != "/tmp/ws" {
		t.Fatalf("unexpected deps: %+v", d)
	}
	if len(d.cfg) != 1 || d.cfg[0].Name != "acme/repo" {
		t.Fatalf("unexpected config: %+v", d.cfg)
	}
}

func TestRootCmd_Help(t *testing.T) {
	var out bytes.Buffer
	origOut := rootCmd.OutOrStdout()
	origErr := rootCmd.ErrOrStderr()
	origArgs := rootCmd.Args
	t.Cleanup(func() {
		rootCmd.SetOut(origOut)
		rootCmd.SetErr(origErr)
		rootCmd.SetArgs(nil)
		rootCmd.Args = origArgs
	})

	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd --help unexpected error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected help output")
	}
}
