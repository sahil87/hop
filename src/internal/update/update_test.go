package update

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
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

// TestRunBrewTimeoutPolicy pins the shll update standard's brew-handling
// clause at the package's seams: the foreground `brew upgrade` context
// carries NO deadline (a wrapper deadline would end in SIGKILL mid-
// transaction — the standard's motivating incident), while the captured
// `brew update` carries a generous graceful bound and the read-only
// `brew info` stays bounded. Deadlines survive cancel(), so inspecting the
// captured contexts after Run returns is safe. The SIGTERM+grace mechanics
// themselves are pinned at the proc level (internal/proc RunGraceful tests) —
// here we pin only what is observable at the seam: deadline presence/absence.
func TestRunBrewTimeoutPolicy(t *testing.T) {
	const brewInfoJSON = `{"formulae":[{"versions":{"stable":"0.0.2"}}]}`

	// Capture the context passed to each brew invocation, keyed by the
	// normalized "name args..." string the sibling test also uses.
	ctxByInvocation := map[string]context.Context{}

	origRun, origForeground, origInstalled := brewRun, brewRunForeground, brewInstalledCheck
	t.Cleanup(func() {
		brewRun = origRun
		brewRunForeground = origForeground
		brewInstalledCheck = origInstalled
	})
	brewInstalledCheck = func() bool { return true }
	brewRun = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		ctxByInvocation[strings.TrimSpace(name+" "+strings.Join(args, " "))] = ctx
		return []byte(brewInfoJSON), nil
	}
	brewRunForeground = func(ctx context.Context, _, name string, args ...string) (int, error) {
		ctxByInvocation[strings.TrimSpace(name+" "+strings.Join(args, " "))] = ctx
		return 0, nil
	}

	var stdout, stderr bytes.Buffer
	if err := Run("v0.0.1", false, &stdout, &stderr); err != nil {
		t.Fatalf("Run returned err: %v", err)
	}

	// MUST: no deadline on the foreground `brew upgrade`.
	upgradeCtx, ok := ctxByInvocation["brew upgrade sahil87/tap/hop"]
	if !ok {
		t.Fatalf("no `brew upgrade` invocation recorded; got: %v", keysOf(ctxByInvocation))
	}
	if d, has := upgradeCtx.Deadline(); has {
		t.Errorf("expected NO deadline on the `brew upgrade` context, got deadline %v", d)
	}

	// Generous graceful bound on the captured `brew update`.
	updateCtx, ok := ctxByInvocation["brew update --quiet"]
	if !ok {
		t.Fatalf("no `brew update` invocation recorded; got: %v", keysOf(ctxByInvocation))
	}
	deadline, has := updateCtx.Deadline()
	if !has {
		t.Fatalf("expected a deadline on the `brew update` context (bounded, graceful)")
	}
	if remaining := time.Until(deadline); remaining < 5*time.Minute {
		t.Errorf("expected a GENEROUS `brew update` bound (>=~10m), got ~%v remaining", remaining)
	}

	// Read-only `brew info` stays bounded.
	infoCtx, ok := ctxByInvocation["brew info --json=v2 sahil87/tap/hop"]
	if !ok {
		t.Fatalf("no `brew info` invocation recorded; got: %v", keysOf(ctxByInvocation))
	}
	if _, has := infoCtx.Deadline(); !has {
		t.Errorf("expected a deadline on the read-only `brew info` context")
	}
}

// keysOf lists a ctx-map's invocation keys for failure messages.
func keysOf(m map[string]context.Context) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestIsBrewInstalledReturnsBool(t *testing.T) {
	// Smoke test: the function must not panic on whatever `os.Executable`
	// returns in the test process. The actual return value depends on the
	// environment — in CI it's false; on a developer machine running `go
	// test` from a brew install of go it's still false (the *go* test binary
	// lives under a temp dir, not /Cellar/). We just assert it doesn't crash.
	_ = isBrewInstalled()
}
