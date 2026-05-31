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

// TestRunSkipBrewUpdate verifies the --skip-brew-update semantics by overriding
// the package's brew-invocation seams with a recorder. The recorder forces the
// brew code path (brewInstalledCheck → true) and reports a NEWER stable version
// than currentVersion via the `brew info --json=v2` stub, so the up-to-date
// short-circuit is NOT hit and the upgrade runs. It asserts both directions:
// with the flag set, `brew update` is omitted but `brew upgrade` still runs;
// with the flag absent (default), both `brew update` and `brew upgrade` run.
func TestRunSkipBrewUpdate(t *testing.T) {
	// recorder accumulates a normalized "name args..." string per invocation.
	var recorded []string

	// brewInfoJSON reports a stable version ("0.0.2") newer than the
	// currentVersion ("v0.0.1") passed to Run, so normalizeVersion equality
	// is false and Run proceeds to the upgrade step.
	const brewInfoJSON = `{"formulae":[{"versions":{"stable":"0.0.2"}}]}`

	// run installs the recording seams and forces the brew code path. Callers
	// read the package-level `recorded` slice after invoking Run.
	run := func(t *testing.T) {
		t.Helper()
		recorded = nil

		origRun, origForeground, origInstalled := brewRun, brewRunForeground, brewInstalledCheck
		t.Cleanup(func() {
			brewRun = origRun
			brewRunForeground = origForeground
			brewInstalledCheck = origInstalled
		})

		brewInstalledCheck = func() bool { return true }
		brewRun = func(_ context.Context, name string, args ...string) ([]byte, error) {
			recorded = append(recorded, strings.TrimSpace(name+" "+strings.Join(args, " ")))
			// Only `brew info` reads the returned bytes; everything else
			// ignores stdout, so returning the JSON unconditionally is safe.
			return []byte(brewInfoJSON), nil
		}
		brewRunForeground = func(_ context.Context, _, name string, args ...string) (int, error) {
			recorded = append(recorded, strings.TrimSpace(name+" "+strings.Join(args, " ")))
			return 0, nil
		}
	}

	hasPrefix := func(invocations []string, prefix string) bool {
		for _, inv := range invocations {
			if strings.HasPrefix(inv, prefix) {
				return true
			}
		}
		return false
	}

	// Skip path: brew update omitted, brew upgrade still runs.
	t.Run("skip", func(t *testing.T) {
		run(t)
		var stdout, stderr bytes.Buffer
		if err := Run("v0.0.1", true, &stdout, &stderr); err != nil {
			t.Fatalf("Run(skip) returned err: %v", err)
		}
		if hasPrefix(recorded, "brew update") {
			t.Errorf("with skipBrewUpdate=true, expected NO 'brew update' invocation, got: %v", recorded)
		}
		if !hasPrefix(recorded, "brew upgrade") {
			t.Errorf("with skipBrewUpdate=true, expected a 'brew upgrade' invocation, got: %v", recorded)
		}
	})

	// Default path (regression guard): both brew update and brew upgrade run.
	t.Run("default", func(t *testing.T) {
		run(t)
		var stdout, stderr bytes.Buffer
		if err := Run("v0.0.1", false, &stdout, &stderr); err != nil {
			t.Fatalf("Run(default) returned err: %v", err)
		}
		if !hasPrefix(recorded, "brew update") {
			t.Errorf("with skipBrewUpdate=false, expected a 'brew update' invocation, got: %v", recorded)
		}
		if !hasPrefix(recorded, "brew upgrade") {
			t.Errorf("with skipBrewUpdate=false, expected a 'brew upgrade' invocation, got: %v", recorded)
		}
	})
}

func TestIsBrewInstalledReturnsBool(t *testing.T) {
	// Smoke test: the function must not panic on whatever `os.Executable`
	// returns in the test process. The actual return value depends on the
	// environment — in CI it's false; on a developer machine running `go
	// test` from a brew install of go it's still false (the *go* test binary
	// lives under a temp dir, not /Cellar/). We just assert it doesn't crash.
	_ = isBrewInstalled()
}
