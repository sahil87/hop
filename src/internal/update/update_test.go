package update

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v0.0.3", "0.0.3"},
		{"0.0.3", "0.0.3"},
		{"", ""},
		{"v", ""},
		{"vvv1.0.0", "vv1.0.0"}, // only one leading "v" is stripped
	}
	for _, c := range cases {
		if got := normalizeVersion(c.in); got != c.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRunNonBrewInstall confirms that when the running binary is NOT installed
// via Homebrew, Run prints a manual-update hint to its `out` writer and
// returns nil without invoking brew. We cannot easily simulate "brew install"
// inside the test process, but we CAN observe that go's `go test` binary
// doesn't live under /Cellar/, so isBrewInstalled returns false here — making
// this assertion stable in CI and on developer machines.
func TestRunNonBrewInstall(t *testing.T) {
	if isBrewInstalled() {
		t.Skip("test binary appears to be brew-installed; non-brew code path not exercised")
	}
	var stdout, stderr bytes.Buffer
	if err := Run("v0.0.3", false, &stdout, &stderr); err != nil {
		t.Fatalf("Run on non-brew install returned err: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"v0.0.3 was not installed via Homebrew",
		"brew install sahil87/tap/hop",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected stdout to contain %q, got:\n%s", want, out)
		}
	}
	if got := stderr.String(); got != "" {
		t.Errorf("expected empty stderr, got: %q", got)
	}
}

func TestIsBrewInstalledReturnsBool(t *testing.T) {
	// Smoke test: the function must not panic on whatever `os.Executable`
	// returns in the test process. The actual return value depends on the
	// environment — in CI it's false; on a developer machine running `go
	// test` from a brew install of go it's still false (the *go* test binary
	// lives under a temp dir, not /Cellar/). We just assert it doesn't crash.
	// Call the concrete default directly (isBrewInstalled is now a swappable
	// seam, so exercising defaultIsBrewInstalled tests the real logic).
	_ = defaultIsBrewInstalled()
}

// brewCall records a single subprocess invocation captured by the fake proc
// seams below.
type brewCall struct {
	name string
	args []string
}

// installFakeBrew swaps the package-level proc seams (and forces the
// brew-installed path) for the duration of a test, restoring them on cleanup.
// runProc returns infoJSON for `brew info` and empty bytes for everything
// else; runForeground reports exit 0. Every invocation is appended to *calls.
func installFakeBrew(t *testing.T, infoJSON string, calls *[]brewCall) {
	t.Helper()

	origIsBrew := isBrewInstalled
	origRunProc := runProc
	origRunForeground := runForeground
	t.Cleanup(func() {
		isBrewInstalled = origIsBrew
		runProc = origRunProc
		runForeground = origRunForeground
	})

	isBrewInstalled = func() bool { return true }
	runProc = func(_ context.Context, name string, args ...string) ([]byte, error) {
		*calls = append(*calls, brewCall{name: name, args: args})
		if len(args) > 0 && args[0] == "info" {
			return []byte(infoJSON), nil
		}
		return nil, nil
	}
	runForeground = func(_ context.Context, _, name string, args ...string) (int, error) {
		*calls = append(*calls, brewCall{name: name, args: args})
		return 0, nil
	}
}

// invoked reports whether any recorded call matches name + a leading subcommand.
func invoked(calls []brewCall, name, sub string) bool {
	for _, c := range calls {
		if c.name == name && len(c.args) > 0 && c.args[0] == sub {
			return true
		}
	}
	return false
}

// TestRunSkipsBrewUpdate confirms the cross-toolkit contract: with
// skipBrewUpdate=true, `brew update` is NOT invoked, but the `brew info`
// version check and `brew upgrade` still run. The fake reports a newer stable
// version than currentVersion so the "already up to date" short-circuit does
// not fire and the upgrade path is exercised.
func TestRunSkipsBrewUpdate(t *testing.T) {
	const infoJSON = `{"formulae":[{"versions":{"stable":"0.0.4"}}]}`
	var calls []brewCall
	installFakeBrew(t, infoJSON, &calls)

	var stdout, stderr bytes.Buffer
	if err := Run("v0.0.3", true, &stdout, &stderr); err != nil {
		t.Fatalf("Run with skipBrewUpdate returned err: %v", err)
	}

	if invoked(calls, "brew", "update") {
		t.Errorf("expected `brew update` to be SKIPPED, but it was invoked; calls=%v", calls)
	}
	if !invoked(calls, "brew", "info") {
		t.Errorf("expected `brew info` version check to still run; calls=%v", calls)
	}
	if !invoked(calls, "brew", "upgrade") {
		t.Errorf("expected `brew upgrade` to still run; calls=%v", calls)
	}
	if out := stdout.String(); !strings.Contains(out, "Updated to v0.0.4.") {
		t.Errorf("expected stdout to report the upgrade, got:\n%s", out)
	}
}

// TestRunDefaultRunsBrewUpdate is the contrapositive: with skipBrewUpdate=false
// (the default), `brew update` MUST run alongside the info check and upgrade.
func TestRunDefaultRunsBrewUpdate(t *testing.T) {
	const infoJSON = `{"formulae":[{"versions":{"stable":"0.0.4"}}]}`
	var calls []brewCall
	installFakeBrew(t, infoJSON, &calls)

	var stdout, stderr bytes.Buffer
	if err := Run("v0.0.3", false, &stdout, &stderr); err != nil {
		t.Fatalf("Run with default flag returned err: %v", err)
	}

	if !invoked(calls, "brew", "update") {
		t.Errorf("expected `brew update` to run by default; calls=%v", calls)
	}
	if !invoked(calls, "brew", "info") {
		t.Errorf("expected `brew info` version check to run; calls=%v", calls)
	}
	if !invoked(calls, "brew", "upgrade") {
		t.Errorf("expected `brew upgrade` to run; calls=%v", calls)
	}
}

// TestRunSkipBrewUpdateStillShortCircuits confirms the "already up to date"
// short-circuit is preserved when the flag is set: matching versions mean the
// info check runs but no upgrade happens.
func TestRunSkipBrewUpdateStillShortCircuits(t *testing.T) {
	const infoJSON = `{"formulae":[{"versions":{"stable":"0.0.3"}}]}`
	var calls []brewCall
	installFakeBrew(t, infoJSON, &calls)

	var stdout, stderr bytes.Buffer
	if err := Run("v0.0.3", true, &stdout, &stderr); err != nil {
		t.Fatalf("Run returned err: %v", err)
	}

	if invoked(calls, "brew", "update") {
		t.Errorf("expected `brew update` to be skipped; calls=%v", calls)
	}
	if !invoked(calls, "brew", "info") {
		t.Errorf("expected `brew info` version check to still run; calls=%v", calls)
	}
	if invoked(calls, "brew", "upgrade") {
		t.Errorf("expected NO `brew upgrade` on an up-to-date binary; calls=%v", calls)
	}
	if out := stdout.String(); !strings.Contains(out, "Already up to date (v0.0.3).") {
		t.Errorf("expected the up-to-date short-circuit, got:\n%s", out)
	}
}
