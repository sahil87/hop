package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// withPickOne swaps the package-level pickOne fzf seam for the duration of a
// test, restoring the original on Cleanup. Mirrors withListWorktrees.
func withPickOne(t *testing.T, fn func(ctx context.Context, lines []string, query string) (string, error)) {
	t.Helper()
	prev := pickOne
	pickOne = fn
	t.Cleanup(func() { pickOne = prev })
}

// pickLineContaining returns a pickOne fake that selects the first piped line
// whose URL column (3rd tab field) equals wantURL, failing the test if none
// matches. This exercises the real buildPickerLines → map-back path.
func pickLineContaining(t *testing.T, wantURL string) func(ctx context.Context, lines []string, query string) (string, error) {
	return func(ctx context.Context, lines []string, query string) (string, error) {
		for _, line := range lines {
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) == 3 && parts[2] == wantURL {
				return line, nil
			}
		}
		t.Fatalf("no piped line carried URL %q; lines=%v", wantURL, lines)
		return "", nil
	}
}

func TestStaleReposFiltersMissingPaths(t *testing.T) {
	present := t.TempDir() // exists on disk
	missing := filepath.Join(t.TempDir(), "gone")

	rs := repos.Repos{
		{Name: "here", Group: "default", Path: present, URL: "git@h:o/here.git"},
		{Name: "gone", Group: "default", Path: missing, URL: "git@h:o/gone.git"},
	}
	got := staleRepos(rs)
	if len(got) != 1 {
		t.Fatalf("expected 1 stale repo, got %d: %v", len(got), got)
	}
	if got[0].Name != "gone" {
		t.Errorf("expected the missing-from-disk repo, got %q", got[0].Name)
	}
}

func TestConfigRmRemovesSelectedURL(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git
`
	path := writeReposFixture(t, yaml)

	withPickOne(t, pickLineContaining(t, "git@github.com:sahil87/wt.git"))

	_, stderr, err := runArgs(t, "config", "rm")
	if err != nil {
		t.Fatalf("config rm: %v", err)
	}
	if !strings.Contains(stderr.String(), "removed: git@github.com:sahil87/wt.git") {
		t.Errorf("missing removed line; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	if strings.Contains(gotStr, "wt.git") {
		t.Errorf("removed URL still present; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "hop.git") {
		t.Errorf("surviving URL missing; got:\n%s", gotStr)
	}
}

func TestConfigRmMapBackByPathOnNameCollision(t *testing.T) {
	// Two repos share the derived name "widget" but live in different groups
	// with different dirs → different Paths. The map-back must select by path,
	// not by the colliding display name.
	yaml := `repos:
  default:
    dir: /tmp/test-rm-default
    urls:
      - git@github.com:org/widget.git
  vendor:
    dir: /tmp/test-rm-vendor
    urls:
      - git@github.com:vendor/widget.git
`
	path := writeReposFixture(t, yaml)

	// Select the vendor one specifically.
	withPickOne(t, pickLineContaining(t, "git@github.com:vendor/widget.git"))

	_, stderr, err := runArgs(t, "config", "rm")
	if err != nil {
		t.Fatalf("config rm: %v", err)
	}
	if !strings.Contains(stderr.String(), "removed: git@github.com:vendor/widget.git") {
		t.Errorf("expected vendor widget removed; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	if strings.Contains(gotStr, "vendor/widget.git") {
		t.Errorf("vendor widget still present; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "org/widget.git") {
		t.Errorf("default widget should be untouched; got:\n%s", gotStr)
	}
}

func TestConfigRmStaleOnlyOffersMissingRepos(t *testing.T) {
	// One repo present on disk, one missing. --stale must offer only the
	// missing one to the picker.
	parent := t.TempDir()
	presentRepo := filepath.Join(parent, "present")
	if err := os.MkdirAll(presentRepo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:org/present.git
      - git@github.com:org/missing.git
`, parent)
	path := writeReposFixture(t, yaml)

	var offered []string
	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		offered = lines
		// Select the (only) stale entry.
		return lines[0], nil
	})

	_, stderr, err := runArgs(t, "config", "rm", "--stale")
	if err != nil {
		t.Fatalf("config rm --stale: %v", err)
	}
	if len(offered) != 1 {
		t.Fatalf("expected exactly 1 stale candidate offered, got %d: %v", len(offered), offered)
	}
	if !strings.Contains(offered[0], "git@github.com:org/missing.git") {
		t.Errorf("stale picker offered the wrong repo: %q", offered[0])
	}
	if !strings.Contains(stderr.String(), "removed: git@github.com:org/missing.git") {
		t.Errorf("expected missing repo removed; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "missing.git") {
		t.Errorf("stale URL still present; got:\n%s", got)
	}
}

func TestConfigRmStaleNothingStale(t *testing.T) {
	parent := t.TempDir()
	repoDir := filepath.Join(parent, "present")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:org/present.git
`, parent)
	path := writeReposFixture(t, yaml)
	original, _ := os.ReadFile(path)

	pickerCalled := false
	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		pickerCalled = true
		return "", nil
	})

	_, stderr, err := runArgs(t, "config", "rm", "--stale")
	if err != nil {
		t.Fatalf("config rm --stale: %v", err)
	}
	if pickerCalled {
		t.Errorf("picker invoked despite zero stale repos")
	}
	if !strings.Contains(stderr.String(), "nothing stale") {
		t.Errorf("expected 'nothing stale' message; stderr=%q", stderr.String())
	}
	// File unchanged.
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("hop.yaml modified on no-op; got:\n%s", got)
	}
}

// --- top-level `hop rm` (change mw9h) -------------------------------------

// TestTopLevelRmRemovesSelectedURL mirrors TestConfigRmRemovesSelectedURL but
// drives the canonical top-level `hop rm` (no positional) — the shared runRm
// body backs the picker path of both spellings (R3).
func TestTopLevelRmRemovesSelectedURL(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git
`
	path := writeReposFixture(t, yaml)

	withPickOne(t, pickLineContaining(t, "git@github.com:sahil87/wt.git"))

	_, stderr, err := runArgs(t, "rm")
	if err != nil {
		t.Fatalf("hop rm: %v", err)
	}
	if !strings.Contains(stderr.String(), "removed: git@github.com:sahil87/wt.git") {
		t.Errorf("missing removed line; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	if strings.Contains(gotStr, "wt.git") {
		t.Errorf("removed URL still present; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "hop.git") {
		t.Errorf("surviving URL missing; got:\n%s", gotStr)
	}
}

// TestTopLevelRmByNameSkipsPicker asserts `hop rm <name>` resolves via
// resolveByName (substring match, exactly one candidate → no fzf) and removes
// directly via RemoveURL WITHOUT invoking the pickOne seam (R4). The pickOne
// fake fails the test if called.
func TestTopLevelRmByNameSkipsPicker(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git
`
	path := writeReposFixture(t, yaml)

	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		t.Fatalf("picker invoked on `hop rm <name>` path; lines=%v", lines)
		return "", nil
	})

	// "wt" uniquely substring-matches the wt repo → resolveByName returns it
	// directly without fzf.
	_, stderr, err := runArgs(t, "rm", "wt")
	if err != nil {
		t.Fatalf("hop rm wt: %v", err)
	}
	if !strings.Contains(stderr.String(), "removed: git@github.com:sahil87/wt.git") {
		t.Errorf("expected wt removed; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	if strings.Contains(gotStr, "wt.git") {
		t.Errorf("removed URL still present; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "hop.git") {
		t.Errorf("surviving URL missing; got:\n%s", gotStr)
	}
}

// TestTopLevelRmByNameRemovesRegardlessOfDisk asserts `hop rm <name>` removes a
// registry entry even when its on-disk folder is absent — no Stat check, no
// "not cloned" error, no prompt (R5). The repo's `dir` points at a temp dir but
// the repo folder itself was never created.
func TestTopLevelRmByNameRemovesRegardlessOfDisk(t *testing.T) {
	parent := t.TempDir() // exists, but no `gone/` repo folder inside it
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:org/gone.git
`, parent)
	path := writeReposFixture(t, yaml)

	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		t.Fatalf("picker invoked on `hop rm <name>` path; lines=%v", lines)
		return "", nil
	})

	_, stderr, err := runArgs(t, "rm", "gone")
	if err != nil {
		t.Fatalf("hop rm gone (folder absent): %v", err)
	}
	if !strings.Contains(stderr.String(), "removed: git@github.com:org/gone.git") {
		t.Errorf("expected gone removed despite absent folder; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "gone.git") {
		t.Errorf("entry not removed; got:\n%s", got)
	}
}

// TestTopLevelRmByNameStripsWorktreeSuffix asserts `hop rm <name>/<wt>` strips
// the "/<wt>" suffix, resolves the parent repo, and removes it directly —
// WITHOUT entering resolveByName's worktree branch (which would force an
// on-disk clone + `wt list` check and fail here, since the folder is absent).
// This keeps the "no on-disk check" guarantee true for every input. The repo
// folder is intentionally never created; if the worktree branch ran, it would
// error with "is not cloned" instead of removing.
func TestTopLevelRmByNameStripsWorktreeSuffix(t *testing.T) {
	parent := t.TempDir() // exists, but no `gone/` repo folder inside it
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:org/gone.git
`, parent)
	path := writeReposFixture(t, yaml)

	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		t.Fatalf("picker invoked on `hop rm <name>/<wt>` path; lines=%v", lines)
		return "", nil
	})

	// "gone/feature-x" → suffix stripped → resolves the parent "gone" repo and
	// removes it, never touching the worktree/clone-state machinery.
	_, stderr, err := runArgs(t, "rm", "gone/feature-x")
	if err != nil {
		t.Fatalf("hop rm gone/feature-x (suffix should be stripped): %v", err)
	}
	if !strings.Contains(stderr.String(), "removed: git@github.com:org/gone.git") {
		t.Errorf("expected parent repo removed after stripping /<wt>; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "gone.git") {
		t.Errorf("entry not removed; got:\n%s", got)
	}
}

// TestTopLevelRmStaleWithNameIsUsageError asserts that combining --stale with a
// positional name is a usage error (exit 2) that removes nothing (R6).
func TestTopLevelRmStaleWithNameIsUsageError(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
`
	path := writeReposFixture(t, yaml)
	original, _ := os.ReadFile(path)

	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		t.Fatalf("resolution/picker should not run on a usage error; lines=%v", lines)
		return "", nil
	})

	_, stderr, err := runArgs(t, "rm", "hop", "--stale")
	var ec *errExitCode
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("expected errExitCode{code:2}, got %v", err)
	}
	if !strings.Contains(ec.msg+stderr.String(), "--stale cannot be combined with a repo name") {
		t.Errorf("expected --stale+name usage message; err=%v stderr=%q", err, stderr.String())
	}
	// File unchanged.
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("hop.yaml modified on usage error; got:\n%s", got)
	}
}

// TestRmNoTTYReturnsSentinelBeforeFzf asserts the no-name `hop rm` picker path
// returns errNoTTY (→ exit 3 via translateExit) with no TTY, never spawning fzf
// (intake Item 3 — single guard point in pickRepo). The pickOne fake fails the
// test if invoked.
func TestRmNoTTYReturnsSentinelBeforeFzf(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git
`
	path := writeReposFixture(t, yaml)
	original, _ := os.ReadFile(path)

	withIsTTY(t, false)
	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		t.Fatalf("picker invoked with no TTY; want errNoTTY before fzf (lines=%v)", lines)
		return "", nil
	})

	_, _, err := runArgs(t, "rm")
	if !errors.Is(err, errNoTTY) {
		t.Fatalf("expected errNoTTY, got %v", err)
	}
	// errNoTTY maps to the distinct exit code 3, not 130 (fzf cancel).
	if code := translateExit(err); code != 3 {
		t.Errorf("translateExit = %d, want 3", code)
	}
	// File unchanged — no removal on a no-TTY refusal.
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("hop.yaml modified on no-TTY refusal; got:\n%s", got)
	}
}

func TestConfigRmFzfMissing(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
`
	writeReposFixture(t, yaml)

	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		return "", proc.ErrNotFound
	})

	_, stderr, err := runArgs(t, "config", "rm")
	if !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), fzfMissingHint) {
		t.Errorf("missing fzf hint; stderr=%q", stderr.String())
	}
}

func TestConfigRmFzfCancelIsNoOp(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
`
	path := writeReposFixture(t, yaml)
	original, _ := os.ReadFile(path)

	// Simulate fzf Esc/Ctrl-C: a genuine *exec.ExitError with code 130, which
	// is what proc.ExitCode (via errors.As) actually matches.
	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		return "", exit130Error(t)
	})

	_, _, err := runArgs(t, "config", "rm")
	if !errors.Is(err, errFzfCancelled) {
		t.Fatalf("expected errFzfCancelled, got %v", err)
	}
	// File unchanged.
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("hop.yaml modified on cancel; got:\n%s", got)
	}
}

// exit130Error runs a tiny subprocess that exits 130 and returns the resulting
// *exec.ExitError, so proc.ExitCode classifies it the same way a real fzf
// cancellation (Esc/Ctrl-C) would.
func exit130Error(t *testing.T) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit 130").Run()
	if err == nil {
		t.Fatal("expected non-nil error from 'exit 130'")
	}
	return err
}

// TestTopLevelRmByNameDryRunPreviewsWithoutWriting asserts `hop rm <name>
// --dry-run` resolves the target via the same path as a live removal, reports
// what it would remove, and leaves hop.yaml byte-for-byte unchanged, exiting 0
// (principle №5: destructive writes support an accurate, no-write --dry-run).
func TestTopLevelRmByNameDryRunPreviewsWithoutWriting(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git
`
	path := writeReposFixture(t, yaml)
	original, _ := os.ReadFile(path)

	withPickOne(t, func(ctx context.Context, lines []string, query string) (string, error) {
		t.Fatalf("picker invoked on `hop rm <name> --dry-run` path; lines=%v", lines)
		return "", nil
	})

	_, stderr, err := runArgs(t, "rm", "wt", "--dry-run")
	if err != nil {
		t.Fatalf("hop rm wt --dry-run: %v", err)
	}
	if !strings.Contains(stderr.String(), "would remove: git@github.com:sahil87/wt.git") {
		t.Errorf("expected would-remove preview line; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), dryRunNoChanges) {
		t.Errorf("expected %q line; stderr=%q", dryRunNoChanges, stderr.String())
	}
	if strings.Contains(stderr.String(), "removed:") || strings.Contains(stderr.String(), "wrote:") {
		t.Errorf("dry-run must not emit live removed:/wrote: lines; stderr=%q", stderr.String())
	}
	// File byte-for-byte unchanged (A-009).
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("hop.yaml modified by --dry-run; got:\n%s\nwant:\n%s", got, original)
	}
}

// TestConfigRmPickerDryRunPreviewsWithoutWriting asserts the picker path
// (`hop rm --dry-run`, no positional) previews the fzf-selected entry via the
// real buildPickerLines → map-back resolution but writes nothing.
func TestConfigRmPickerDryRunPreviewsWithoutWriting(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git
`
	path := writeReposFixture(t, yaml)
	original, _ := os.ReadFile(path)

	withIsTTY(t, true)
	withPickOne(t, pickLineContaining(t, "git@github.com:sahil87/wt.git"))

	_, stderr, err := runArgs(t, "rm", "--dry-run")
	if err != nil {
		t.Fatalf("hop rm --dry-run: %v", err)
	}
	if !strings.Contains(stderr.String(), "would remove: git@github.com:sahil87/wt.git") {
		t.Errorf("expected would-remove preview for the picked entry; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), dryRunNoChanges) {
		t.Errorf("expected %q line; stderr=%q", dryRunNoChanges, stderr.String())
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("hop.yaml modified by picker --dry-run; got:\n%s", got)
	}
}

// TestRmByNameDryRunForgivingNotFound asserts the edge case (A-013): a --dry-run
// against a repo whose URL is NOT in its group previews "Nothing to remove."
// without erroring and exits 0 — mirroring the live path's forgiving not-found
// contract (WouldRemoveURL shares removeURLFromTree's ErrURLNotFound sentinel).
// It drives removeRepo directly — the shared body runRm calls — with a repo
// carrying an unregistered URL, so the not-found branch is exercised without
// depending on resolveByName's match behavior.
func TestRmByNameDryRunForgivingNotFound(t *testing.T) {
	parent := t.TempDir()
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:org/present.git
`, parent)
	path := writeReposFixture(t, yaml)
	original, _ := os.ReadFile(path)

	r := &repos.Repo{Name: "present", Group: "default", URL: "git@github.com:org/unregistered.git", Path: filepath.Join(parent, "present")}
	var stderr strings.Builder
	if err := removeRepo(&stderr, "hop rm", path, r, true); err != nil {
		t.Fatalf("dry-run removeRepo on unregistered URL should be a forgiving no-op: %v", err)
	}
	if !strings.Contains(stderr.String(), "Nothing to remove.") {
		t.Errorf("expected forgiving not-found preview; stderr=%q", stderr.String())
	}
	if strings.Contains(stderr.String(), "would remove:") {
		t.Errorf("must not claim a would-remove for an unregistered URL; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("hop.yaml modified by not-found dry-run; got:\n%s", got)
	}
}
