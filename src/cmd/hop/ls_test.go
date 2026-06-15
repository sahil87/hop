package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/hop/internal/proc"
)

const lsYAML = `repos:
  default:
    dir: /tmp/test-ls
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/beta.git
`

func TestLsListsAllRepos(t *testing.T) {
	writeReposFixture(t, lsYAML)

	stdout, _, err := runArgs(t, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("expected alpha and beta in output, got %q", out)
	}
	if !strings.Contains(out, "/tmp/test-ls/alpha") {
		t.Fatalf("expected path in output, got %q", out)
	}
}

func TestLsEmptyConfig(t *testing.T) {
	writeReposFixture(t, "")

	stdout, _, err := runArgs(t, "ls")
	if err != nil {
		t.Fatalf("ls (empty): %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
}

// TestLsDefaultDoesNotInvokeWt locks in the backward-compat contract: plain
// `hop ls` never invokes wt. We swap the seam to a fatal sentinel — any
// invocation fails the test loudly.
func TestLsDefaultDoesNotInvokeWt(t *testing.T) {
	writeReposFixture(t, lsYAML)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		t.Fatalf("listWorktrees called from default `hop ls`; want NOT called (repoPath=%q)", repoPath)
		return nil, nil
	})

	if _, _, err := runArgs(t, "ls"); err != nil {
		t.Fatalf("ls: %v", err)
	}
}

// makeMixedRegistry builds a hop.yaml + on-disk fixture with three repos:
//   - clonedA (cloned, .git present)
//   - clonedB (cloned, .git present)
//   - uncloned (registered but no on-disk presence)
//
// Returns the parent dir so callers can compose paths.
func makeMixedRegistry(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	for _, n := range []string{"clonedA", "clonedB"} {
		if err := os.MkdirAll(filepath.Join(parent, n, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:sahil87/clonedA.git
      - git@github.com:sahil87/clonedB.git
      - git@github.com:sahil87/uncloned.git
`, parent)
	writeReposFixture(t, yaml)
	return parent
}

func TestLsTreesAcrossMixedRegistry(t *testing.T) {
	parent := makeMixedRegistry(t)

	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		switch repoPath {
		case filepath.Join(parent, "clonedA"):
			return []WtEntry{
				{Name: "main", Path: filepath.Join(parent, "clonedA"), IsMain: true},
				{Name: "feat-x", Path: filepath.Join(parent, "clonedA.worktrees/feat-x"), Dirty: true},
				{Name: "hotfix", Path: filepath.Join(parent, "clonedA.worktrees/hotfix"), Unpushed: 2},
			}, nil
		case filepath.Join(parent, "clonedB"):
			return []WtEntry{{Name: "main", Path: filepath.Join(parent, "clonedB"), IsMain: true}}, nil
		}
		t.Fatalf("unexpected listWorktrees call with repoPath=%q", repoPath)
		return nil, nil
	})

	stdout, _, err := runArgs(t, "ls", "--trees")
	if err != nil {
		t.Fatalf("ls --trees: %v", err)
	}
	out := stdout.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 rows, got %d: %q", len(lines), out)
	}
	// Source order: clonedA, clonedB, uncloned.
	if !strings.HasPrefix(lines[0], "clonedA") || !strings.Contains(lines[0], "3 trees") {
		t.Errorf("line 0 = %q, want clonedA with 3 trees", lines[0])
	}
	if !strings.Contains(lines[0], "feat-x*") {
		t.Errorf("line 0 missing dirty marker `feat-x*`: %q", lines[0])
	}
	if !strings.Contains(lines[0], "hotfix↑2") {
		t.Errorf("line 0 missing unpushed marker `hotfix↑2`: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "clonedB") || !strings.Contains(lines[1], "1 tree ") {
		t.Errorf("line 1 = %q, want clonedB with `1 tree`", lines[1])
	}
	if !strings.HasPrefix(lines[2], "uncloned") || !strings.Contains(lines[2], "(not cloned)") {
		t.Errorf("line 2 = %q, want uncloned `(not cloned)`", lines[2])
	}
}

func TestLsTreesPerRowFailureDegradesGracefully(t *testing.T) {
	parent := makeMixedRegistry(t)

	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		if repoPath == filepath.Join(parent, "clonedA") {
			// listWorktrees returns errors verbatim; the "wt list failed:"
			// prefix is added by runLsTrees. See unmarshalWtEntries' contract
			// in wt_list.go.
			return nil, fmt.Errorf("corrupt .git/worktrees")
		}
		if repoPath == filepath.Join(parent, "clonedB") {
			return []WtEntry{{Name: "main", Path: repoPath, IsMain: true}}, nil
		}
		return nil, nil
	})

	stdout, _, err := runArgs(t, "ls", "--trees")
	if err != nil {
		t.Fatalf("ls --trees with per-row failure: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "clonedA") || !strings.Contains(out, "(wt list failed:") {
		t.Errorf("expected clonedA row with `(wt list failed:`, got: %q", out)
	}
	// Guard against prefix duplication regressing: the "wt list" label should
	// appear exactly once per row (not "wt list failed: wt list: corrupt ...").
	if strings.Count(out, "wt list") != 1 {
		t.Errorf("expected single 'wt list' label in output, got: %q", out)
	}
	// clonedB row still present despite clonedA failure.
	if !strings.Contains(out, "clonedB") || !strings.Contains(out, "1 tree") {
		t.Errorf("expected clonedB row to survive: %q", out)
	}
	// Uncloned row still present.
	if !strings.Contains(out, "(not cloned)") {
		t.Errorf("expected uncloned row to survive: %q", out)
	}
}

func TestLsTreesMissingWtFailsFast(t *testing.T) {
	makeMixedRegistry(t)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return nil, proc.ErrNotFound
	})

	_, stderr, err := runArgs(t, "ls", "--trees")
	if !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent for missing wt, got %v", err)
	}
	if !strings.Contains(stderr.String(), wtMissingHint) {
		t.Errorf("expected stderr to contain %q, got %q", wtMissingHint, stderr.String())
	}
}

// --- hop ls --json (intake 1x1u Item 2) -----------------------------------

func TestLsJSONDefaultEmitsRepoArray(t *testing.T) {
	writeReposFixture(t, lsYAML)

	stdout, _, err := runArgs(t, "ls", "--json")
	if err != nil {
		t.Fatalf("ls --json: %v", err)
	}
	var got []lsRepoJSON
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d: %+v", len(got), got)
	}
	// Source order: alpha, beta.
	if got[0].Name != "alpha" || got[1].Name != "beta" {
		t.Fatalf("expected alpha,beta order, got %s,%s", got[0].Name, got[1].Name)
	}
	if got[0].Path != "/tmp/test-ls/alpha" {
		t.Errorf("alpha path = %q, want /tmp/test-ls/alpha", got[0].Path)
	}
	if got[0].URL != "git@github.com:sahil87/alpha.git" {
		t.Errorf("alpha url = %q", got[0].URL)
	}
	if got[0].Group != "default" {
		t.Errorf("alpha group = %q, want default", got[0].Group)
	}
	// Default mode omits the trees-only fields.
	if got[0].Cloned != nil || got[0].Worktrees != nil || got[0].Error != "" {
		t.Errorf("default --json must omit cloned/worktrees/error, got %+v", got[0])
	}
}

func TestLsJSONEmptyEmitsEmptyArray(t *testing.T) {
	writeReposFixture(t, "")

	stdout, _, err := runArgs(t, "ls", "--json")
	if err != nil {
		t.Fatalf("ls --json (empty): %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "[]" {
		t.Fatalf("expected `[]` for empty list, got %q", got)
	}
}

func TestLsJSONDefaultDoesNotInvokeWt(t *testing.T) {
	writeReposFixture(t, lsYAML)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		t.Fatalf("listWorktrees called from `hop ls --json` (no --trees); want NOT called (repoPath=%q)", repoPath)
		return nil, nil
	})
	if _, _, err := runArgs(t, "ls", "--json"); err != nil {
		t.Fatalf("ls --json: %v", err)
	}
}

func TestLsJSONTreesNestsWorktreesAndStates(t *testing.T) {
	parent := makeMixedRegistry(t)

	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		switch repoPath {
		case filepath.Join(parent, "clonedA"):
			return []WtEntry{
				{Name: "main", Path: filepath.Join(parent, "clonedA"), IsMain: true},
				{Name: "feat-x", Path: filepath.Join(parent, "clonedA.worktrees/feat-x"), Dirty: true},
				{Name: "hotfix", Path: filepath.Join(parent, "clonedA.worktrees/hotfix"), Unpushed: 2},
			}, nil
		case filepath.Join(parent, "clonedB"):
			return []WtEntry{{Name: "main", Path: filepath.Join(parent, "clonedB"), IsMain: true}}, nil
		}
		t.Fatalf("unexpected listWorktrees call with repoPath=%q", repoPath)
		return nil, nil
	})

	stdout, _, err := runArgs(t, "ls", "--json", "--trees")
	if err != nil {
		t.Fatalf("ls --json --trees: %v", err)
	}
	var got []lsRepoJSON
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(got))
	}
	// clonedA: cloned, 3 worktrees, dirty/unpushed populated.
	a := got[0]
	if a.Name != "clonedA" || a.Cloned == nil || !*a.Cloned {
		t.Fatalf("clonedA should be cloned:true, got %+v", a)
	}
	if len(a.Worktrees) != 3 {
		t.Fatalf("clonedA expected 3 worktrees, got %d", len(a.Worktrees))
	}
	if a.Worktrees[1].Name != "feat-x" || a.Worktrees[1].Dirty == nil || !*a.Worktrees[1].Dirty {
		t.Errorf("feat-x should have dirty:true, got %+v", a.Worktrees[1])
	}
	if a.Worktrees[2].Unpushed == nil || *a.Worktrees[2].Unpushed != 2 {
		t.Errorf("hotfix should have unpushed:2, got %+v", a.Worktrees[2])
	}
	// clonedB: cloned, 1 worktree.
	b := got[1]
	if b.Name != "clonedB" || b.Cloned == nil || !*b.Cloned || len(b.Worktrees) != 1 {
		t.Errorf("clonedB should be cloned with 1 worktree, got %+v", b)
	}
	// uncloned: cloned:false, no worktrees.
	u := got[2]
	if u.Name != "uncloned" {
		t.Fatalf("expected uncloned at index 2, got %q", u.Name)
	}
	if u.Cloned == nil || *u.Cloned {
		t.Errorf("uncloned should be cloned:false, got %+v", u.Cloned)
	}
	if u.Worktrees != nil {
		t.Errorf("uncloned must omit worktrees, got %+v", u.Worktrees)
	}
}

func TestLsJSONTreesPerRowFailureSurfacesErrorField(t *testing.T) {
	parent := makeMixedRegistry(t)

	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		if repoPath == filepath.Join(parent, "clonedA") {
			return nil, fmt.Errorf("corrupt .git/worktrees")
		}
		if repoPath == filepath.Join(parent, "clonedB") {
			return []WtEntry{{Name: "main", Path: repoPath, IsMain: true}}, nil
		}
		return nil, nil
	})

	stdout, _, err := runArgs(t, "ls", "--json", "--trees")
	if err != nil {
		t.Fatalf("ls --json --trees with per-row failure: %v", err)
	}
	var got []lsRepoJSON
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(got) != 3 {
		t.Fatalf("array must never abort: expected 3 repos, got %d", len(got))
	}
	// clonedA carries an error field; the array survives.
	if got[0].Name != "clonedA" || !strings.Contains(got[0].Error, "wt list failed:") {
		t.Errorf("clonedA should carry error field, got %+v", got[0])
	}
	if got[0].Worktrees != nil {
		t.Errorf("clonedA with an error should omit worktrees, got %+v", got[0].Worktrees)
	}
	// clonedB row survives with its worktree.
	if got[1].Name != "clonedB" || len(got[1].Worktrees) != 1 {
		t.Errorf("clonedB should survive with 1 worktree, got %+v", got[1])
	}
	// uncloned row survives.
	if got[2].Name != "uncloned" || got[2].Cloned == nil || *got[2].Cloned {
		t.Errorf("uncloned should survive as cloned:false, got %+v", got[2])
	}
}

func TestLsJSONTreesMissingWtFailsFastNoJSON(t *testing.T) {
	makeMixedRegistry(t)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return nil, proc.ErrNotFound
	})

	stdout, stderr, err := runArgs(t, "ls", "--json", "--trees")
	if !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent (fail-fast) for missing wt, got %v", err)
	}
	if !strings.Contains(stderr.String(), wtMissingHint) {
		t.Errorf("expected wtMissingHint on stderr, got %q", stderr.String())
	}
	// No JSON emitted on the fail-fast path.
	if strings.TrimSpace(stdout.String()) != "" {
		t.Errorf("expected NO JSON on fail-fast, got stdout: %q", stdout.String())
	}
}

func TestFormatTreesRowSingularPluralizesCorrectly(t *testing.T) {
	tests := []struct {
		name    string
		entries []WtEntry
		want    string
	}{
		{
			name:    "single main",
			entries: []WtEntry{{Name: "main", IsMain: true}},
			want:    "1 tree  (main)",
		},
		{
			name:    "two with no flags",
			entries: []WtEntry{{Name: "main"}, {Name: "feat-x"}},
			want:    "2 trees  (main, feat-x)",
		},
		{
			name:    "dirty and unpushed",
			entries: []WtEntry{{Name: "feat", Dirty: true, Unpushed: 3}},
			want:    "1 tree  (feat*↑3)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTreesRow(tt.entries)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
