// Package update implements `hop update` — self-upgrade via Homebrew.
//
// All subprocess invocations route through internal/proc per Constitution
// Principle I (no direct os/exec outside internal/proc). The brew formula is
// referenced by its fully-qualified name (sahil87/tap/hop) to avoid a name
// collision with the Homebrew core `hop` cask.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sahil87/hop/internal/proc"
)

// brewFormula is the fully-qualified tap formula. The fully-qualified form
// disambiguates against the `hop` cask (an HWP document viewer) that would
// otherwise shadow it on `brew info hop`.
const brewFormula = "sahil87/tap/hop"

// Timeout policy follows the shll update standard's brew-handling clause:
// never SIGKILL a package-manager subprocess mid-transaction, never impose a
// short hard timeout on `brew upgrade`. Hence there is NO upgrade timeout at
// all (the foreground `brew upgrade` runs unbounded — the user watches brew's
// own progress and can Ctrl-C), and the bounded calls below cancel gracefully
// via proc.RunGraceful (SIGTERM + grace, never SIGKILL-first).
const (
	// brewUpdateTimeout bounds the captured `brew update --quiet` refresh. It
	// has no visible progress, so an unbounded hang would look like a frozen
	// `hop update` — but brew update mutates tap git state, so the bound is
	// generous and enforced gracefully, not a short hard kill.
	brewUpdateTimeout = 10 * time.Minute
	// brewInfoTimeout bounds the read-only `brew info --json=v2` query. Not a
	// mutation, so a short bound is fine; it rides the same graceful path for
	// consistency (SIGTERM first costs nothing).
	brewInfoTimeout = 30 * time.Second
)

// Test seams. These unexported package-level indirections exist so tests can
// observe which brew subcommands are invoked (and force the brew code path)
// without refactoring internal/proc. In production they default to the real
// internal/proc calls and isBrewInstalled, so behavior — including Constitution
// Principle I's explicit-argument-slice convention — is identical. Tests swap
// these out and restore them via t.Cleanup/defer.
var (
	// brewRun routes the captured brew calls (`brew update`, `brew info`)
	// through proc.RunGraceful so a context cancellation delivers SIGTERM +
	// grace instead of the exec.CommandContext default SIGKILL — brew must
	// never be hard-killed mid-transaction (shll update standard).
	brewRun = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return proc.RunGraceful(ctx, name, args...)
	}
	brewRunForeground = func(ctx context.Context, dir, name string, args ...string) (int, error) {
		return proc.RunForeground(ctx, dir, name, args...)
	}
	brewInstalledCheck = isBrewInstalled
)

// Run self-updates the hop binary via Homebrew.
//
// currentVersion is the binary's reported version (e.g. "v0.0.3"). The leading
// "v" is stripped before comparison since `brew info` reports the bare form.
//
// skipBrewUpdate gates ONLY the internal `brew update --quiet` tap-metadata
// refresh. When true, that refresh is skipped silently; the `brew info` version
// check, the up-to-date short-circuit, and the `brew upgrade` are unaffected and
// still run. When false (the default), behavior is byte-for-byte identical to
// before this flag existed.
//
// out and errOut receive only the WRAPPER messages this package emits ("Current
// version:", "Already up to date", error hints, etc.). Subprocess stdout/stderr
// from `brew update`, `brew info`, and `brew upgrade` is intentionally NOT
// routed through these writers — internal/proc owns subprocess stream routing
// (proc.Run pipes child stderr to the parent's os.Stderr; proc.RunForeground
// inherits all three streams). The split is deliberate: subprocess streams
// are large and tty-aware (brew prints colored progress); the wrapper messages
// are small and may be redirected for tests or embedding. Callers in production
// should pass os.Stdout / os.Stderr to keep the two consistent.
//
// Returns nil on success or no-op (not a brew install, already up to date).
// Returns proc.ErrNotFound when brew is missing on PATH (callers should map
// this to errSilent so cobra does not double-print). Returns a wrapped error
// for other brew failures.
func Run(currentVersion string, skipBrewUpdate bool, out, errOut io.Writer) error {
	if !brewInstalledCheck() {
		fmt.Fprintf(out, "hop %s was not installed via Homebrew.\n", currentVersion)
		fmt.Fprintln(out, "Update manually, or reinstall with: brew install "+brewFormula)
		return nil
	}

	fmt.Fprintf(out, "Current version: %s\n", currentVersion)
	fmt.Fprintln(out, "Checking for updates...")

	if !skipBrewUpdate {
		ctx, cancel := context.WithTimeout(context.Background(), brewUpdateTimeout)
		_, err := brewRun(ctx, "brew", "update", "--quiet")
		cancel()
		if err != nil {
			if errors.Is(err, proc.ErrNotFound) {
				fmt.Fprintln(errOut, "hop update: brew not found on PATH.")
				return err
			}
			return fmt.Errorf("brew update failed: %w", err)
		}
	}

	latest, err := brewLatestVersion()
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(errOut, "hop update: brew not found on PATH.")
			return err
		}
		return fmt.Errorf("could not determine latest version: %w", err)
	}

	if normalizeVersion(latest) == normalizeVersion(currentVersion) {
		fmt.Fprintf(out, "Already up to date (%s).\n", currentVersion)
		return nil
	}

	fmt.Fprintf(out, "Updating %s → v%s...\n", currentVersion, normalizeVersion(latest))

	// No deadline on `brew upgrade` — deliberately. It runs foreground with
	// inherited stdio, so the user watches brew's own progress and can Ctrl-C
	// (user-initiated SIGINT, which brew handles). A wrapper-imposed deadline
	// would SIGKILL brew mid-transaction (the exec.CommandContext default),
	// which can corrupt the keg between `brew unlink` and `brew link` — the
	// exact incident the shll update standard's brew-handling clause bans.
	code, err := brewRunForeground(context.Background(), "", "brew", "upgrade", brewFormula)
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(errOut, "hop update: brew not found on PATH.")
			return err
		}
		return fmt.Errorf("brew upgrade failed: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("brew upgrade exited with code %d", code)
	}

	fmt.Fprintf(out, "Updated to v%s.\n", normalizeVersion(latest))
	return nil
}

// brewLatestVersion queries Homebrew for the latest stable version of the
// tap formula. Returns the bare version string (e.g. "0.0.3") with no `v`
// prefix — that's how brew reports it in `versions.stable`.
func brewLatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), brewInfoTimeout)
	defer cancel()
	out, err := brewRun(ctx, "brew", "info", "--json=v2", brewFormula)
	if err != nil {
		return "", err
	}
	var info struct {
		Formulae []struct {
			Versions struct {
				Stable string `json:"stable"`
			} `json:"versions"`
		} `json:"formulae"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", err
	}
	if len(info.Formulae) == 0 || info.Formulae[0].Versions.Stable == "" {
		return "", errors.New("no stable version found in brew info output")
	}
	return info.Formulae[0].Versions.Stable, nil
}

// isBrewInstalled checks whether the running binary lives under a Cellar
// directory, which is the canonical signature of a Homebrew install. The
// symlink at /opt/homebrew/bin/hop (or /usr/local/bin/hop on Intel) resolves
// through to .../Cellar/hop/<version>/bin/hop.
func isBrewInstalled() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	real, err := filepath.EvalSymlinks(self)
	if err != nil {
		return false
	}
	return strings.Contains(real, "/Cellar/")
}

// normalizeVersion strips a single leading "v" so we can compare the binary's
// `git describe`-derived version (e.g. "v0.0.3") against brew's bare report
// ("0.0.3"). It does NOT do semver parsing — string equality after normalize
// is sufficient because both sides come from the same canonical source (the
// release tag).
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}
