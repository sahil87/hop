package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateHome points $HOME at an empty temp dir so the fixed config path does
// not resolve to a real ~/.config/hop/hop.yaml on the developer's machine.
// Returns the home dir.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// writeFixedConfig writes body to <home>/.config/hop/hop.yaml and returns the path.
func writeFixedConfig(t *testing.T, home, body string) string {
	t.Helper()
	path := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return path
}

func TestResolveReturnsFixedPathWhenPresent(t *testing.T) {
	home := isolateHome(t)
	path := writeFixedConfig(t, home, "")

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestResolveNotFound(t *testing.T) {
	home := isolateHome(t)

	_, err := Resolve()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "hop: no hop.yaml found at " + filepath.Join(home, ".config", "hop", "hop.yaml") + ". Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter."
	if err.Error() != want {
		t.Fatalf("error mismatch:\n  want: %s\n  got:  %s", want, err.Error())
	}
}

// TestResolveIgnoresLegacyEnvVars verifies that none of $HOP_CONFIG,
// $XDG_CONFIG_HOME, or $REPOS_YAML move the resolved path — only the fixed
// $HOME-relative path is consulted (clean break, no env override).
func TestResolveIgnoresLegacyEnvVars(t *testing.T) {
	home := isolateHome(t)
	// Point the (removed) env vars at real files; they must be ignored.
	bogus := filepath.Join(t.TempDir(), "bogus.yaml")
	if err := os.WriteFile(bogus, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("HOP_CONFIG", bogus)
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(bogus))
	t.Setenv("REPOS_YAML", bogus)

	// No file at the fixed path → must be not-found, ignoring every env var.
	_, err := Resolve()
	if err == nil {
		t.Fatalf("expected not-found (env vars ignored), got nil")
	}
	want := filepath.Join(home, ".config", "hop", "hop.yaml")
	if got := err.Error(); !strings.Contains(got, want) {
		t.Fatalf("expected fixed-path not-found error mentioning %q; got %q", want, got)
	}
}

func TestResolveHomeUnset(t *testing.T) {
	t.Setenv("HOME", "")
	os.Unsetenv("HOME")

	_, err := Resolve()
	if err == nil {
		t.Fatalf("expected error when $HOME unset, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "$HOME is not set") {
		t.Fatalf("expected '$HOME is not set' error; got %q", got)
	}
}

func TestResolveWriteTargetReturnsFixedPathNoStat(t *testing.T) {
	home := isolateHome(t)
	// No file on disk — ResolveWriteTarget must still return the fixed path.
	got, err := ResolveWriteTarget()
	if err != nil {
		t.Fatalf("ResolveWriteTarget: %v", err)
	}
	want := filepath.Join(home, ".config", "hop", "hop.yaml")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveWriteTargetHomeUnset(t *testing.T) {
	t.Setenv("HOME", "")
	os.Unsetenv("HOME")

	_, err := ResolveWriteTarget()
	if err == nil {
		t.Fatalf("expected error when $HOME unset, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "$HOME is not set") {
		t.Fatalf("expected '$HOME is not set' error; got %q", got)
	}
}
