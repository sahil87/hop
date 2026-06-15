package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// execCommand is a tiny indirection so test scaffolding doesn't have to import
// os/exec at every call site. Tests are exempt from Constitution I — only
// production code under cmd/hop and internal/* is bound by it.
var execCommand = exec.Command

// makeClonedRepoDirs pre-creates `<groupDir>/<name>/.git` for each name so
// pull/sync's cloneState check returns stateAlreadyCloned without invoking
// real git.
func makeClonedRepoDirs(t *testing.T, groupDir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(groupDir, n, ".git"), 0o755); err != nil {
			t.Fatalf("setup %s: %v", n, err)
		}
	}
}

// pullSyncYAMLBuilder writes a hop.yaml with a default and a vendor group
// pointing at temp dirs and returns the dirs so callers can pre-stage clones.
func pullSyncYAMLFixture(t *testing.T) (configPath, defaultDir, vendorDir string) {
	t.Helper()
	defaultDir = t.TempDir()
	vendorDir = t.TempDir()
	yaml := "repos:\n" +
		"  default:\n" +
		"    dir: " + defaultDir + "\n" +
		"    urls:\n" +
		"      - git@github.com:sahil87/alpha.git\n" +
		"      - git@github.com:sahil87/beta.git\n" +
		"  vendor:\n" +
		"    dir: " + vendorDir + "\n" +
		"    urls:\n" +
		"      - git@github.com:vendor/gamma.git\n"
	configPath = writeReposFixture(t, yaml)
	return configPath, defaultDir, vendorDir
}

// TestPullPluralNoActionErrors verifies a plural selection (group) with no
// action token is a usage error under the selection-first grammar — replaces
// the old `hop pull` (no positional) usage error.
func TestPullPluralNoActionErrors(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "default")
	if err == nil {
		t.Fatalf("expected usage error for plural selection with no action")
	}
	if !strings.Contains(err.Error(), "plural selection") {
		t.Fatalf("expected plural-selection hint, got %q", err.Error())
	}
}

// TestPullAllInteractiveRefused verifies an interactive action on a plural
// selection is refused — replaces the old `--all conflicts with positional`.
func TestPullAllInteractiveRefused(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "--all", "code")
	if err == nil {
		t.Fatalf("expected refusal of interactive action on plural selection")
	}
	if !strings.Contains(err.Error(), "not a batch action") {
		t.Fatalf("expected not-a-batch-action hint, got %q", err.Error())
	}
}

func TestPullSingleNotClonedExitsWithSkipMessage(t *testing.T) {
	_, defaultDir, _ := pullSyncYAMLFixture(t)
	// Don't pre-create .git for alpha — it's not cloned.
	_ = defaultDir

	_, stderr, err := runArgs(t, "alpha", "pull")
	if err == nil {
		t.Fatalf("expected error for not-cloned single repo")
	}
	if !strings.Contains(stderr.String(), "skip: alpha not cloned") {
		t.Fatalf("expected skip line, got stderr=%q", stderr.String())
	}
}

func TestPullBatchGroupSkipsNotClonedAndReportsSummary(t *testing.T) {
	_, defaultDir, _ := pullSyncYAMLFixture(t)
	// Neither alpha nor beta has a .git dir — both should be skipped.
	_ = defaultDir

	_, stderr, err := runArgs(t, "default", "pull")
	if err != nil {
		t.Fatalf("expected nil err for batch with all-skipped, got %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "skip: alpha not cloned") {
		t.Errorf("expected skip alpha line, got: %s", got)
	}
	if !strings.Contains(got, "skip: beta not cloned") {
		t.Errorf("expected skip beta line, got: %s", got)
	}
	if !strings.Contains(got, "summary: pulled=0 skipped=2 failed=0") {
		t.Errorf("expected summary line, got: %s", got)
	}
}

func TestPullBatchAllIteratesAllRepos(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, err := runArgs(t, "--all", "pull")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := stderr.String()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(got, "skip: "+name+" not cloned") {
			t.Errorf("expected skip for %s, got: %s", name, got)
		}
	}
	if !strings.Contains(got, "summary: pulled=0 skipped=3 failed=0") {
		t.Errorf("expected summary line, got: %s", got)
	}
}

func TestPullBatchOutputOrderMatchesYAMLSourceOrder(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, _ := runArgs(t, "--all", "pull")
	out := stderr.String()
	idxAlpha := strings.Index(out, "alpha")
	idxBeta := strings.Index(out, "beta")
	idxGamma := strings.Index(out, "gamma")
	if !(idxAlpha < idxBeta && idxBeta < idxGamma) {
		t.Fatalf("expected order alpha < beta < gamma in stderr; out=%q", out)
	}
}

func TestPullBatchGroupOnlyIncludesGroupMembers(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, _ := runArgs(t, "vendor", "pull")
	out := stderr.String()
	if !strings.Contains(out, "skip: gamma not cloned") {
		t.Errorf("expected gamma in vendor batch, got: %s", out)
	}
	if strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Errorf("default-group repos must not appear in vendor batch, got: %s", out)
	}
	if !strings.Contains(out, "summary: pulled=0 skipped=1 failed=0") {
		t.Errorf("expected summary, got: %s", out)
	}
}

func TestPullStdoutIsEmpty(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	stdout, _, _ := runArgs(t, "--all", "pull")
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

// TestPullToolFormErrorsInBinary verifies that a non-verb action token on a
// singular selection (tool-form) errors in the binary with the tool-form hint —
// tool-form runs in the shell via the shim's RUN_IN_PARENT plan, so the binary
// can't honor it directly. Replaces the old cobra two-positional cap test
// (the root command now accepts arbitrary args; the cap moved into runRoot).
func TestPullToolFormErrorsInBinary(t *testing.T) {
	t.Setenv("HOP_WRAPPER", "") // ensure the tool-form hint is not suppressed
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "alpha", "echo")
	if err == nil {
		t.Fatalf("expected tool-form error in the binary")
	}
	if !strings.Contains(err.Error(), "is not a hop verb") {
		t.Fatalf("expected tool-form hint, got: %v", err)
	}
}

// initBareRepoWithCommit creates a bare repo at <dir>/source.git, then clones
// it into a temp working tree, makes one commit, and pushes it back so the
// bare repo has a default branch with at least one commit. Returns the bare
// repo's file:// URL and the bare path. Used for pull/sync tests where the
// upstream needs to have content (an empty bare upstream causes `git pull` to
// fail with "no such ref was fetched").
func initBareRepoWithCommit(t *testing.T, dir string) (url, srcPath string) {
	t.Helper()
	url, srcPath = initBareRepo(t, dir)

	// Clone, commit, push back. Use exec directly here (not internal/proc) —
	// this is test scaffolding only, not production code under Constitution I.
	stage := filepath.Join(dir, "stage")
	cmds := [][]string{
		{"git", "clone", srcPath, stage},
		{"git", "-C", stage, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "--allow-empty", "-m", "init"},
		{"git", "-C", stage, "push", "origin", "HEAD:refs/heads/main"},
	}
	for _, args := range cmds {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\noutput: %s", args, err, out)
		}
	}
	return url, srcPath
}

// TestPullSingleHappyPathAgainstRealGit exercises the success path against an
// actual local bare repo with one commit. Verifies the per-repo status line,
// exit 0, and that proc.RunCapture is wired correctly.
func TestPullSingleHappyPathAgainstRealGit(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}

	_, stderr, err := runArgs(t, "source", "pull")
	if err != nil {
		t.Fatalf("hop pull source: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "pull: source ✓") {
		t.Errorf("expected success status line, got: %s", got)
	}
	if !strings.Contains(got, "Already up to date.") {
		t.Errorf("expected 'Already up to date.' summary, got: %s", got)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Errorf("expected cloned repo intact: %v", err)
	}
}
