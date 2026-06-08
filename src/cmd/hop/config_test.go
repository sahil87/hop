package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sahil87/hop/internal/config"
	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/scan"
)

func TestConfigInitWritesStarter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".config", "hop", "hop.yaml")

	stdout, stderr, err := runArgs(t, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v", err)
	}
	if !strings.Contains(stdout.String(), "Created "+target) {
		t.Fatalf("expected 'Created %s' on stdout, got %q", target, stdout.String())
	}

	// Verify the post-init stderr tip is the new two-line wording.
	stderrStr := stderr.String()
	wantLine1 := "Edit the file to add your repos, or run `hop add -r <dir>` to populate from existing on-disk repos."
	wantLine2 := "Tip: to sync this config across machines, keep it in your dotfiles and symlink ~/.config/hop/hop.yaml to it."
	if !strings.Contains(stderrStr, wantLine1) {
		t.Errorf("init tip line 1 mismatch.\nwant: %q\ngot: %q", wantLine1, stderrStr)
	}
	if !strings.Contains(stderrStr, wantLine2) {
		t.Errorf("init tip line 2 mismatch.\nwant: %q\ngot: %q", wantLine2, stderrStr)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("expected mode 0644, got %o", info.Mode().Perm())
	}
}

func TestConfigInitRefusesOverwrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, _, err := runArgs(t, "config", "init")
	if err == nil {
		t.Fatalf("expected refusal, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' message, got %q", err.Error())
	}
}

func TestConfigWherePrintsResolvedPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".config", "hop", "hop.yaml")

	stdout, _, err := runArgs(t, "config", "where")
	if err != nil {
		t.Fatalf("config where: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != target {
		t.Fatalf("expected %q, got %q", target, got)
	}
}

func TestConfigWhereDoesNotErrorOnMissingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No file written — `where` must still print the fixed path regardless of
	// existence (no stat).
	want := filepath.Join(home, ".config", "hop", "hop.yaml")

	stdout, _, err := runArgs(t, "config", "where")
	if err != nil {
		t.Fatalf("config where on missing file: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestConfigPathSubcommandRemoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".config", "hop", "hop.yaml")
	stdout, _, _ := runArgs(t, "config", "path")
	// The old handler would have printed the resolved write target on stdout.
	// We assert the new behavior: stdout MUST NOT be just the resolved path.
	if strings.TrimSpace(stdout.String()) == target {
		t.Fatalf("config path appears to still call the old handler (stdout = %q)", stdout.String())
	}
}

// --- recursive add (-r) tests ----------------------------------------------

// withFakeGitRunner swaps in a fake git runner for a test and restores the
// production one on cleanup. Tests that don't need real git but exercise
// `hop add` end-to-end (vs. just buildScanPlan) use this seam.
func withFakeGitRunner(t *testing.T, fake scan.GitRunner) {
	t.Helper()
	orig := gitRunner
	gitRunner = fake
	t.Cleanup(func() { gitRunner = orig })
}

// makeRepoDir creates dir/.git so the scan classifier sees it as a normal
// repo. Mirrors internal/scan/scan_test.go's makeRepo helper.
func makeRepoDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("makeRepoDir: %v", err)
	}
}

// fakeURLForDir returns a deterministic fake `git remote get-url` runner
// that maps each canonical dir to a pre-supplied URL.
func fakeURLForDir(t *testing.T, urlByDir map[string]string) scan.GitRunner {
	return func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		switch {
		case len(args) == 1 && args[0] == "remote":
			if _, ok := urlByDir[dir]; ok {
				return []byte("origin\n"), nil
			}
			return nil, errors.New("test fake: unknown dir " + dir)
		case len(args) == 3 && args[0] == "remote" && args[1] == "get-url" && args[2] == "origin":
			if u, ok := urlByDir[dir]; ok {
				return []byte(u + "\n"), nil
			}
			return nil, errors.New("test fake: unknown dir " + dir)
		}
		return nil, errors.New("test fake: unexpected args " + strings.Join(args, " "))
	}
}

// TestRecursiveAddMissingHopYamlPrintModeStillErrors verifies that print mode
// (-p) STILL errors on a missing config — print mode never touches the file, so
// there is nothing to auto-init (R2). The surfaced message is config.Resolve()'s
// refined two-path hint.
func TestRecursiveAddMissingHopYamlPrintModeStillErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No config file written — the fixed path does not exist.
	missing := filepath.Join(home, ".config", "hop", "hop.yaml")

	scanRoot := t.TempDir()
	_, stderr, err := runArgs(t, "add", "-r", "-p", scanRoot)
	if err == nil || !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent, got %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "no hop.yaml found at "+missing) {
		t.Errorf("missing-config message not found; stderr=%q", got)
	}
	if !strings.Contains(got, "Run 'hop add <dir>'") {
		t.Errorf("missing refined hop-add hint; stderr=%q", got)
	}
	// Print mode must NOT have created the file.
	if _, statErr := os.Stat(missing); statErr == nil {
		t.Errorf("print mode created the config; it must not")
	}
}

// TestRecursiveAddWriteModeAutoInits verifies that `hop add -r` on a fresh
// machine auto-creates the skeleton (announced via created:) and then merges the
// discovered repos (R1/R3).
func TestRecursiveAddWriteModeAutoInits(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "hop", "hop.yaml")
	// No config — auto-init must create it.

	// A convention repo under ~/code so it lands in the default group.
	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "add", "-r", filepath.Join(home, "code"))
	if err != nil {
		t.Fatalf("add -r (fresh env): %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "created: "+configPath) {
		t.Errorf("missing created: announcement; stderr=%q", got)
	}
	if !strings.Contains(got, "wrote: "+configPath) {
		t.Errorf("missing wrote: line; stderr=%q", got)
	}
	contents, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("config not created: %v", readErr)
	}
	if !strings.Contains(string(contents), "git@github.com:sahil87/hop.git") {
		t.Errorf("discovered URL not merged; got:\n%s", contents)
	}
}

func TestRecursiveAddInvalidDepth(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	scanRoot := t.TempDir()
	_, stderr, err := runArgs(t, "add", "-r", "--depth", "0", scanRoot)
	var ec *errExitCode
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("expected errExitCode{code:2}, got %v", err)
	}
	if !strings.Contains(stderr.String(), "hop add: --depth must be >= 1.") {
		t.Errorf("missing depth-validation message; stderr=%q", stderr.String())
	}
}

func TestRecursiveAddZeroReposPrintMode(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{}))

	scanRoot := t.TempDir()
	stdout, stderr, err := runArgs(t, "add", "-r", "-p", scanRoot)
	if err != nil {
		t.Fatalf("add -r -p: %v", err)
	}
	gotErr := stderr.String()
	if !strings.Contains(gotErr, "found 0 repos. Nothing to add.") {
		t.Errorf("missing zero-repos line; stderr=%q", gotErr)
	}
	// Header still printed; existing yaml content still on stdout.
	gotOut := stdout.String()
	if !strings.Contains(gotOut, "# hop config — generated by 'hop add -r -p "+scanRoot+"'") {
		t.Errorf("missing print-mode header; stdout=%q", gotOut)
	}
	if !strings.Contains(gotOut, "(UTC).") {
		t.Errorf("missing UTC suffix in header; stdout=%q", gotOut)
	}
	// New-spelling trailer (replaces scan's "Run with --write to merge ...").
	if !strings.Contains(gotErr, "Run without --print to merge into") {
		t.Errorf("missing new-spelling print trailer; stderr=%q", gotErr)
	}
}

func TestRecursiveAddConventionMatchPrintMode(t *testing.T) {
	clearConfigEnv(t)
	// Use an isolated HOME so $HOME-based code_root is deterministic.
	home := t.TempDir()
	t.Setenv("HOME", home)

	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "config:\n  code_root: ~/code\nrepos:\n  default: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	scanRoot := filepath.Join(home, "code")
	repoDir := filepath.Join(scanRoot, "sahil87", "hop")
	makeRepoDir(t, repoDir)

	canonRepo, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	stdout, stderr, err := runArgs(t, "add", "-r", "-p", scanRoot)
	if err != nil {
		t.Fatalf("add -r -p: %v", err)
	}
	gotOut := stdout.String()
	gotErr := stderr.String()

	// URL must land in the default group.
	if !strings.Contains(gotOut, "git@github.com:sahil87/hop.git") {
		t.Errorf("URL not in stdout YAML; stdout=%q", gotOut)
	}
	// Summary mentions matched convention.
	if !strings.Contains(gotErr, "matched convention (default): 1") {
		t.Errorf("expected matched-convention summary line; stderr=%q", gotErr)
	}
	// Header references the user-supplied arg verbatim, new spelling.
	if !strings.Contains(gotOut, "'hop add -r -p "+scanRoot+"'") {
		t.Errorf("header user-arg mismatch; stdout=%q", gotOut)
	}
	// Print mode must NOT have mutated the file.
	if got, _ := os.ReadFile(hopYaml); string(got) != original {
		t.Errorf("print mode mutated hop.yaml; got:\n%s", got)
	}
}

func TestRecursiveAddNonConventionInventsGroup(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "config:\n  code_root: ~/code\nrepos:\n  default: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// repo at ~/vendor/forks/hop — non-convention.
	repoDir := filepath.Join(home, "vendor", "forks", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	scanRoot := filepath.Join(home, "vendor")
	stdout, stderr, err := runArgs(t, "add", "-r", "-p", scanRoot)
	if err != nil {
		t.Fatalf("add -r -p: %v", err)
	}
	gotOut := stdout.String()
	gotErr := stderr.String()
	if !strings.Contains(gotOut, "forks:") {
		t.Errorf("expected 'forks:' invented group; stdout=%q", gotOut)
	}
	if !strings.Contains(gotOut, "dir: ~/vendor/forks") {
		t.Errorf("expected ~-substituted dir; stdout=%q", gotOut)
	}
	if !strings.Contains(gotErr, "invented groups: 1 (forks)") {
		t.Errorf("expected invented-group summary; stderr=%q", gotErr)
	}
}

func TestRecursiveAddWriteMode(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "# top comment\nconfig:\n  code_root: ~/code\n\nrepos:\n  default: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	stdout, stderr, err := runArgs(t, "add", "-r", filepath.Join(home, "code"))
	if err != nil {
		t.Fatalf("add -r: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("write mode should have empty stdout; got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "wrote: "+hopYaml) {
		t.Errorf("missing 'wrote:' trailer; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(hopYaml)
	gotStr := string(got)
	if !strings.Contains(gotStr, "git@github.com:sahil87/hop.git") {
		t.Errorf("URL not merged into hop.yaml; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "# top comment") {
		t.Errorf("comments not preserved; got:\n%s", gotStr)
	}
}

func TestRecursiveAddGitMissingPropagates(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(hopYaml, []byte("repos:\n  default: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	repoDir := filepath.Join(home, "code", "owner", "x")
	makeRepoDir(t, repoDir)

	withFakeGitRunner(t, func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		return nil, proc.ErrNotFound
	})

	_, stderr, err := runArgs(t, "add", "-r", filepath.Join(home, "code"))
	if !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), gitMissingHint) {
		t.Errorf("missing git-hint; stderr=%q", stderr.String())
	}
}

// TestSingleDirAddPrintDryRun verifies the NEW single-dir dry-run: `hop add -p
// <dir>` (no -r) renders the one-repo plan to stdout and writes nothing (R2).
func TestSingleDirAddPrintDryRun(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "config:\n  code_root: ~/code\nrepos:\n  default: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	stdout, stderr, err := runArgs(t, "add", "-p", repoDir)
	if err != nil {
		t.Fatalf("add -p (single dir): %v\nstderr: %s", err, stderr.String())
	}
	gotOut := stdout.String()
	// The rendered plan contains the URL and the header.
	if !strings.Contains(gotOut, "git@github.com:sahil87/hop.git") {
		t.Errorf("single-dir dry-run did not render the URL; stdout=%q", gotOut)
	}
	if !strings.Contains(gotOut, "# hop config — generated by 'hop add -r -p "+repoDir+"'") {
		t.Errorf("missing print-mode header; stdout=%q", gotOut)
	}
	// No write: file byte-for-byte unchanged.
	if got, _ := os.ReadFile(hopYaml); string(got) != original {
		t.Errorf("single-dir dry-run mutated hop.yaml; got:\n%s", got)
	}
	// Print trailer present on stderr.
	if !strings.Contains(stderr.String(), "Run without --print to merge into") {
		t.Errorf("missing print trailer; stderr=%q", stderr.String())
	}
}

// TestAddForcedGroupAutoCreates verifies -g <name> auto-creates a missing group
// (announced via `created group:`), places the repo into it, and writes (R5).
func TestAddForcedGroupAutoCreates(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No 'vendor' group exists yet.
	original := "config:\n  code_root: ~/code\nrepos:\n  default: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// A convention repo (would normally land in default) — -g must override that.
	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "add", "-g", "vendor", repoDir)
	if err != nil {
		t.Fatalf("add -g vendor: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "created group: vendor") {
		t.Errorf("missing 'created group: vendor' announcement; stderr=%q", got)
	}
	contents, _ := os.ReadFile(hopYaml)
	gotStr := string(contents)
	if !strings.Contains(gotStr, "vendor:") {
		t.Errorf("expected 'vendor:' group in hop.yaml; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "git@github.com:sahil87/hop.git") {
		t.Errorf("URL not placed into forced group; got:\n%s", gotStr)
	}
	// The convention default group must NOT have received the URL.
	if strings.Contains(gotStr, "default:\n    - git@github.com:sahil87/hop.git") {
		t.Errorf("forced-group repo leaked into default; got:\n%s", gotStr)
	}
}

// TestAddForcedGroupExistingNoAnnouncement verifies that when -g names an
// already-existing group, repos are appended with NO `created group:` line (R5).
func TestAddForcedGroupExistingNoAnnouncement(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// 'work' already exists as an empty flat group.
	original := "config:\n  code_root: ~/code\nrepos:\n  default: []\n  work: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "add", "-g", "work", repoDir)
	if err != nil {
		t.Fatalf("add -g work: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if strings.Contains(got, "created group:") {
		t.Errorf("must NOT announce created group: for an existing group; stderr=%q", got)
	}
	contents, _ := os.ReadFile(hopYaml)
	if !strings.Contains(string(contents), "git@github.com:sahil87/hop.git") {
		t.Errorf("URL not appended to existing group; got:\n%s", contents)
	}
}

// TestRecursiveAddForcedGroup verifies -r combined with -g forces every
// discovered repo into the named (auto-created) group (R1/R5).
func TestRecursiveAddForcedGroup(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "config:\n  code_root: ~/code\nrepos:\n  default: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Two repos under ~/clients (non-convention).
	repoA := filepath.Join(home, "clients", "acme", "site")
	repoB := filepath.Join(home, "clients", "globex", "api")
	makeRepoDir(t, repoA)
	makeRepoDir(t, repoB)
	canonA, _ := filepath.EvalSymlinks(repoA)
	canonB, _ := filepath.EvalSymlinks(repoB)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonA: "git@github.com:acme/site.git",
		canonB: "git@github.com:globex/api.git",
	}))

	_, stderr, err := runArgs(t, "add", "-r", "-g", "work", filepath.Join(home, "clients"))
	if err != nil {
		t.Fatalf("add -r -g work: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "created group: work") {
		t.Errorf("missing 'created group: work'; stderr=%q", got)
	}
	// S2: a user-named -g group is "forced", not "invented" — the summary must
	// say `forced group: work` and MUST NOT mislabel it as `invented groups:`.
	if !strings.Contains(got, "forced group: work") {
		t.Errorf("missing 'forced group: work' summary line; stderr=%q", got)
	}
	if strings.Contains(got, "invented groups:") {
		t.Errorf("user-named -g group must not be labeled 'invented groups:'; stderr=%q", got)
	}
	contents, _ := os.ReadFile(hopYaml)
	gotStr := string(contents)
	if !strings.Contains(gotStr, "work:") {
		t.Errorf("expected 'work:' group; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "git@github.com:acme/site.git") || !strings.Contains(gotStr, "git@github.com:globex/api.git") {
		t.Errorf("both repos should be in the forced group; got:\n%s", gotStr)
	}
	// No invented per-parent-dir groups (acme/globex) should appear — -g forces all.
	if strings.Contains(gotStr, "acme:") || strings.Contains(gotStr, "globex:") {
		t.Errorf("-g must override invented-group logic; got:\n%s", gotStr)
	}
}

// TestAddForcedGroupPrintRendersFlatList is the M1 regression guard: in print
// mode the forced group (`-g <name>`) MUST render as a flat list
// `<name>: [urls…]` with NO `dir` key — byte-identical to what write mode
// produces — and the rendered output MUST itself be loadable. The old bug
// synthesized a map-shaped group with `dir: ""`, which both misrepresented the
// write and failed to load (`group '<name>' has empty 'dir'`). Covers both the
// single-dir (`-g -p`) and recursive (`-r -g -p`) variants (R2/R5; A-002/A-013/
// A-019).
func TestAddForcedGroupPrintRendersFlatList(t *testing.T) {
	const url = "git@github.com:sahil87/hop.git"

	cases := []struct {
		name string
		args func(repoDir string) []string // CLI args for the print invocation
	}{
		{
			name: "single-dir -g -p",
			args: func(repoDir string) []string { return []string{"add", "-g", "vendor", "-p", repoDir} },
		},
		{
			name: "recursive -r -g -p",
			args: func(repoDir string) []string { return []string{"add", "-r", "-g", "vendor", "-p", repoDir} },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
			if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			// No 'vendor' group exists yet — forces the create/synthesize path.
			const original = "config:\n  code_root: ~/code\nrepos:\n  default: []\n"
			if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}

			// A convention repo (would normally land in default) — -g overrides.
			repoDir := filepath.Join(home, "code", "sahil87", "hop")
			makeRepoDir(t, repoDir)
			canonRepo, _ := filepath.EvalSymlinks(repoDir)
			withFakeGitRunner(t, fakeURLForDir(t, map[string]string{canonRepo: url}))

			// --- Print mode: render to stdout, write nothing. ---
			stdout, stderr, err := runArgs(t, tc.args(repoDir)...)
			if err != nil {
				t.Fatalf("print invocation: %v\nstderr: %s", err, stderr.String())
			}
			printed := stdout.String()

			// Flat shape: `vendor:` followed by a `- <url>` list item, and NO
			// `dir:` key anywhere in the rendered output.
			if !strings.Contains(printed, "vendor:") {
				t.Fatalf("rendered plan has no 'vendor:' group; stdout=%q", printed)
			}
			if strings.Contains(printed, "dir:") {
				t.Errorf("forced group must render flat (no 'dir:' key); stdout=%q", printed)
			}
			// Correct M1 shape: a flat list under the group key carrying the URL,
			// e.g. `vendor: ['…']` — NOT a map-shaped `vendor:\n  dir: ""\n …`.
			if !strings.Contains(printed, url) {
				t.Errorf("forced group should render the URL in a flat list; stdout=%q", printed)
			}
			wantFlat := fmt.Sprintf("vendor: ['%s']", url)
			if !strings.Contains(printed, wantFlat) {
				t.Errorf("forced group should render as a flat list %q; stdout=%q", wantFlat, printed)
			}
			// Print must not touch the file.
			if got, _ := os.ReadFile(hopYaml); string(got) != original {
				t.Errorf("print mode mutated hop.yaml; got:\n%s", got)
			}

			// --- Loadability: the rendered YAML body (header stripped) parses
			// without the empty-'dir' error. ---
			body := stripPrintHeader(printed)
			loadable := filepath.Join(t.TempDir(), "rendered.yaml")
			if err := os.WriteFile(loadable, []byte(body), 0o644); err != nil {
				t.Fatalf("write rendered: %v", err)
			}
			cfg, err := config.Load(loadable)
			if err != nil {
				t.Fatalf("rendered plan does not load (M1 regression): %v\nbody:\n%s", err, body)
			}
			if !groupHasURL(cfg, "vendor", url) {
				t.Errorf("rendered 'vendor' group missing URL after load; body:\n%s", body)
			}

			// --- Write parity: write mode produces the same flat shape. ---
			writeArgs := stripPrintFlag(tc.args(repoDir))
			if _, werr, err := runArgs(t, writeArgs...); err != nil {
				t.Fatalf("write invocation: %v\nstderr: %s", err, werr.String())
			}
			written, _ := os.ReadFile(hopYaml)
			if !strings.Contains(string(written), "vendor:") || strings.Contains(string(written), "dir:") {
				t.Errorf("write mode produced a different shape than print; got:\n%s", written)
			}
			// The rendered print body equals the bytes write produced (both go
			// through RenderScan into the identical flat node).
			if body != string(written) {
				t.Errorf("print render != write bytes (shape parity broken):\nprint:\n%q\nwrite:\n%q", body, written)
			}
		})
	}
}

// stripPrintHeader removes the two-line `# hop config …` print-mode header,
// returning just the YAML body that RenderScan produced.
func stripPrintHeader(printed string) string {
	lines := strings.SplitAfter(printed, "\n")
	var body []string
	skip := 0
	for _, l := range lines {
		if skip < 2 && strings.HasPrefix(l, "#") {
			skip++
			continue
		}
		body = append(body, l)
	}
	return strings.Join(body, "")
}

// stripPrintFlag returns args with the "-p" element removed (to derive the
// equivalent write invocation from a print invocation).
func stripPrintFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "-p" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// groupHasURL reports whether cfg has a group named name containing url.
func groupHasURL(cfg *config.Config, name, url string) bool {
	for _, g := range cfg.Groups {
		if g.Name != name {
			continue
		}
		for _, u := range g.URLs {
			if u == url {
				return true
			}
		}
	}
	return false
}

// TestConfigScanUnknownCommand verifies the hard break (R7): `hop config scan`
// is gone, so cobra returns an unknown-command error (no alias).
func TestConfigScanUnknownCommand(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	_, _, err := runArgs(t, "config", "scan", "/tmp")
	if err == nil {
		t.Fatalf("expected unknown-command error for deleted `config scan`, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected cobra 'unknown command' error; got %q", err.Error())
	}
}

func TestSlugifyGroupName(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"forks", "forks", true},
		{"My Stuff!", "my-stuff", true},
		{"9-experiments", "g9-experiments", true},
		{"___", "", false},
		{"///", "", false},
		{"!@#$", "", false},
		{"alpha", "alpha", true},
		{"with_underscore", "with_underscore", true},
		{"--leading-trailing--", "leading-trailing", true},
		{"_leading_trailing_", "leading_trailing", true},
	}
	for _, c := range cases {
		got, ok := slugifyGroupName(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("slugifyGroupName(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestRecursiveAddSlugifyEmptySkipsGracefully(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(hopYaml, []byte("config:\n  code_root: ~/code\nrepos:\n  default: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Pathological parent base: all underscores → slug empty after trim.
	pathological := filepath.Join(home, "elsewhere", "___", "hop")
	makeRepoDir(t, pathological)
	canonRepo, _ := filepath.EvalSymlinks(pathological)

	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "add", "-r", "-p", filepath.Join(home, "elsewhere"))
	if err != nil {
		t.Fatalf("add -r -p: %v", err)
	}
	if !strings.Contains(stderr.String(), "skip: ") || !strings.Contains(stderr.String(), "cannot derive group name") {
		t.Errorf("expected slugify-fail skip line; stderr=%q", stderr.String())
	}
}

func TestRecursiveAddConflictResolutionDirMismatch(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Existing 'vendor' group with dir ~/old-vendor.
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := `config:
  code_root: ~/code
repos:
  default: []
  vendor:
    dir: ~/old-vendor
    urls:
      - git@github.com:vendor/old.git
`
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Make the existing dir resolvable so EvalSymlinks succeeds in
	// canonicalForCompare.
	if err := os.MkdirAll(filepath.Join(home, "old-vendor"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Repo at ~/elsewhere/vendor/thing — parent basename is 'vendor', so it
	// slugifies to 'vendor', colliding with the existing group whose dir
	// (~/old-vendor) differs from ~/elsewhere/vendor.
	newRepo := filepath.Join(home, "elsewhere", "vendor", "thing")
	makeRepoDir(t, newRepo)
	canonNew, _ := filepath.EvalSymlinks(newRepo)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonNew: "git@github.com:vendor/thing.git",
	}))

	stdout, stderr, err := runArgs(t, "add", "-r", "-p", filepath.Join(home, "elsewhere"))
	if err != nil {
		t.Fatalf("add -r -p: %v", err)
	}
	if !strings.Contains(stdout.String(), "vendor-2:") {
		t.Errorf("expected 'vendor-2' suffix in stdout; stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "note: invented group 'vendor' already exists in hop.yaml") {
		t.Errorf("missing conflict-resolution note; stderr=%q", stderr.String())
	}
}

func TestRecursiveAddHeaderUTCFormat(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(hopYaml, []byte("repos:\n  default: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	scanRoot := t.TempDir()
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{}))

	// Capture both possible UTC dates around the run to avoid a midnight-edge
	// race: the header is stamped during runArgs, so if the UTC day rolls
	// between capture and assertion the test would flake.
	dateBefore := time.Now().UTC().Format("2006-01-02")
	stdout, _, err := runArgs(t, "add", "-r", "-p", scanRoot)
	if err != nil {
		t.Fatalf("add -r -p: %v", err)
	}
	dateAfter := time.Now().UTC().Format("2006-01-02")
	if !strings.Contains(stdout.String(), dateBefore+" (UTC).") &&
		!strings.Contains(stdout.String(), dateAfter+" (UTC).") {
		t.Errorf("expected UTC date %q or %q in header; stdout=%q", dateBefore, dateAfter, stdout.String())
	}
}

func TestAddRequiresExactlyOneArg(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	// No positional → cobra ExactArgs(1) error (cobra returns its own error;
	// runArgs returns it without translateExit applied, so we just check err
	// is non-nil).
	_, _, err := runArgs(t, "add")
	if err == nil {
		t.Fatalf("expected error from cobra for missing arg, got nil")
	}
}

func TestConfigSubcommandsListedUnderConfigHelp(t *testing.T) {
	stdout, _, err := runArgs(t, "config", "--help")
	if err != nil {
		t.Fatalf("config --help: %v", err)
	}
	gotOut := stdout.String()
	// Cobra renders the subcommand list with each name as a left-anchored cell;
	// asserting on the full Short line (rather than the bare token) avoids false
	// positives from substrings that appear inside other subcommands' Shorts —
	// e.g. the literal "print" appears in `where`'s Short "print the resolved
	// hop.yaml path", so a bare strings.Contains(gotOut, "print") would pass
	// even if the `print` subcommand were never registered.
	wants := []string{
		"bootstrap a starter hop.yaml",                   // init
		"print the resolved hop.yaml path",               // where
		"print the resolved hop.yaml contents to stdout", // print
	}
	for _, line := range wants {
		if !strings.Contains(gotOut, line) {
			t.Errorf("expected %q in config --help; got:\n%s", line, gotOut)
		}
	}
	// `scan` is deleted (no alias) — its Short must NOT appear under config.
	if strings.Contains(gotOut, "scan a directory for git repos") {
		t.Errorf("deleted `config scan` Short still in config --help; got:\n%s", gotOut)
	}
	// `add` and `rm` are hidden aliases under config (promoted to top-level).
	// They MUST NOT appear in `config --help`.
	if strings.Contains(gotOut, "register on-disk repos into hop.yaml") {
		t.Errorf("hidden alias `config add` Short leaked into config --help; got:\n%s", gotOut)
	}
	if strings.Contains(gotOut, "remove a registered repo from hop.yaml") {
		t.Errorf("hidden alias `config rm` Short leaked into config --help; got:\n%s", gotOut)
	}
}

// --- config print tests ---------------------------------------------------

func TestConfigPrintEmitsFileBytes(t *testing.T) {
	// Fixture exercises comment preservation and inline whitespace — the raw-
	// bytes contract means stdout must equal these bytes exactly.
	body := "# top comment\nconfig:\n  code_root: ~/code  # inline comment\nrepos:\n  default:\n    - git@github.com:foo/bar.git\n"
	writeReposFixture(t, body)

	stdout, stderr, err := runArgs(t, "config", "print")
	if err != nil {
		t.Fatalf("config print: %v", err)
	}
	if got := stdout.String(); got != body {
		t.Errorf("stdout mismatch (want byte-exact match)\nwant: %q\ngot:  %q", body, got)
	}
	if got := stderr.String(); got != "" {
		t.Errorf("expected empty stderr, got %q", got)
	}
}

func TestConfigPrintNoConfigErrors(t *testing.T) {
	// Isolate $HOME so the fixed path does NOT resolve to a real
	// ~/.config/hop/hop.yaml on the developer's machine.
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, _, err := runArgs(t, "config", "print")
	if err == nil {
		t.Fatalf("expected 'no hop.yaml found' error, got nil")
	}
	if !strings.Contains(err.Error(), "no hop.yaml found") {
		t.Errorf("expected 'no hop.yaml found' in error; got %q", err.Error())
	}
	// Read commands surface the refined two-path hint (R2 / Assumption 11):
	// point at both `hop add <dir>` and `hop config init`.
	if !strings.Contains(err.Error(), "Run 'hop add <dir>'") {
		t.Errorf("expected refined hop-add hint; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "'hop config init' for a starter") {
		t.Errorf("expected config-init starter hint; got %q", err.Error())
	}
}
