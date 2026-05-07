package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestOnboardWritesNormalOutputToStdout(t *testing.T) {
	stdout, stderr, err := executeRootCommand(t, nil, "onboard")
	if err != nil {
		t.Fatalf("execute onboard: %v", err)
	}
	if !strings.Contains(stdout, "Codebase Exploration") {
		t.Fatalf("stdout missing onboard content:\n%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got:\n%s", stderr)
	}
}

func TestInitWritesSuccessMessageToStdout(t *testing.T) {
	root := initGitRepo(t)
	restore := chdir(t, root)
	defer restore()

	stdout, stderr, err := executeRootCommand(t, nil, "init")
	if err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if !strings.Contains(stdout, "Initialized treelines in") {
		t.Fatalf("stdout missing init success message:\n%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got:\n%s", stderr)
	}
}

func executeRootCommand(t *testing.T, configure func(), args ...string) (string, string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	previousOut := rootCmd.OutOrStdout()
	previousErr := rootCmd.ErrOrStderr()
	previousQuiet := flagQuiet
	previousVerbose := flagVerbose

	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)
	flagQuiet = false
	flagVerbose = false
	if configure != nil {
		configure()
	}

	_, err := rootCmd.ExecuteC()

	rootCmd.SetOut(previousOut)
	rootCmd.SetErr(previousErr)
	rootCmd.SetArgs(nil)
	flagQuiet = previousQuiet
	flagVerbose = previousVerbose

	return stdout.String(), stderr.String(), err
}
