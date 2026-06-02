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
