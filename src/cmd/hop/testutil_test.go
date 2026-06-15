package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
)

// TestMain forces the isTTY seam to report an interactive terminal for the whole
// package by default. The test binary runs with stdin redirected (not a TTY), so
// without this every picker-exercising test would short-circuit on the no-TTY
// guard (intake Item 3). Production's default for an interactive user IS a TTY,
// so defaulting to true models the normal case; the dedicated no-TTY tests opt
// out via withIsTTY(t, false).
func TestMain(m *testing.M) {
	isTTY = func() bool { return true }
	os.Exit(m.Run())
}

// withIsTTY swaps the package-level isTTY seam for the duration of a test,
// restoring the original on Cleanup. Lets tests simulate a non-interactive
// (no-TTY) environment without allocating a PTY.
func withIsTTY(t *testing.T, v bool) {
	t.Helper()
	prev := isTTY
	isTTY = func() bool { return v }
	t.Cleanup(func() { isTTY = prev })
}

// loadConfigForTest resolves the fixed config path via config.Resolve and
// parses it via config.Load, fataling on failure. Used by tests that need direct
// access to the parsed *config.Config (e.g., to exercise group-name predicates).
func loadConfigForTest(t *testing.T) *config.Config {
	t.Helper()
	path, err := config.Resolve()
	if err != nil {
		t.Fatalf("config.Resolve: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

// runArgs constructs a fresh root command, captures stdout/stderr buffers, executes
// with the provided args, and returns the buffers and any error from cobra.
func runArgs(t *testing.T, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	cmd := newRootCmd()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

// runCmd executes a single subcommand factory directly (without the root) and
// returns its captured buffers. Useful when you want to test a command in isolation.
func runCmd(t *testing.T, factory func() *cobra.Command, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	cmd := factory()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

// writeReposFixture writes a hop.yaml at the fixed config path under an isolated
// $HOME (t.TempDir()) and points hop at it by overriding $HOME. Returns the full
// path (<home>/.config/hop/hop.yaml). yamlBody is written verbatim.
func writeReposFixture(t *testing.T, yamlBody string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(yamlBody), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// clearConfigEnv isolates $HOME to an empty temp dir so the fixed config path
// does not resolve to a real ~/.config/hop/hop.yaml on the developer's machine.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}
