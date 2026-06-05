package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	wantLine1 := "Edit the file to add your repos, or run `hop config scan <dir>` to populate from existing on-disk repos."
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

// --- config scan tests ----------------------------------------------------

// withFakeGitRunner swaps in a fake git runner for a test and restores the
// production one on cleanup. Tests that don't need real git but exercise
// runConfigScan end-to-end (vs. just buildScanPlan) use this seam.
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

// TestConfigScanMissingHopYamlPrintModeStillErrors verifies that print mode
// (no --write) STILL errors on a missing config — print mode never touches the
// file, so there is nothing to auto-init (R4 / Assumption 7). The surfaced
// message is now config.Resolve()'s refined two-path hint.
func TestConfigScanMissingHopYamlPrintModeStillErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No config file written — the fixed path does not exist.
	missing := filepath.Join(home, ".config", "hop", "hop.yaml")

	scanRoot := t.TempDir()
	_, stderr, err := runArgs(t, "config", "scan", scanRoot)
	if err == nil || !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent, got %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "no hop.yaml found at "+missing) {
		t.Errorf("missing-config message not found; stderr=%q", got)
	}
	// Refined message points at hop add (and config init); the old
	// "Run 'hop config init' first, then re-run scan." gate is gone.
	if !strings.Contains(got, "Run 'hop add <dir>'") {
		t.Errorf("missing refined hop-add hint; stderr=%q", got)
	}
	if strings.Contains(got, "re-run scan") {
		t.Errorf("old scan-gate message still present; stderr=%q", got)
	}
	// Print mode must NOT have created the file.
	if _, statErr := os.Stat(missing); statErr == nil {
		t.Errorf("print mode created the config; it must not")
	}
}

// TestConfigScanWriteModeAutoInits verifies that --write on a fresh machine
// auto-creates the skeleton (announced via created:) and then merges the scan
// results (R4).
func TestConfigScanWriteModeAutoInits(t *testing.T) {
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

	_, stderr, err := runArgs(t, "config", "scan", filepath.Join(home, "code"), "--write")
	if err != nil {
		t.Fatalf("scan --write (fresh env): %v\nstderr: %s", err, stderr.String())
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
		t.Errorf("scanned URL not merged; got:\n%s", contents)
	}
}

func TestConfigScanDirNotADirectory(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	notADir := filepath.Join(t.TempDir(), "notadir.txt")
	if err := os.WriteFile(notADir, []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, stderr, err := runArgs(t, "config", "scan", notADir)
	var ec *errExitCode
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("expected errExitCode{code:2}, got %v", err)
	}
	want := "hop config scan: '" + notADir + "' is not a directory."
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("missing not-a-directory message; stderr=%q", stderr.String())
	}
}

func TestConfigScanDirDoesNotExist(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	missing := "/no/such/path-test-xyz"
	_, stderr, err := runArgs(t, "config", "scan", missing)
	var ec *errExitCode
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("expected errExitCode{code:2}, got %v", err)
	}
	want := "hop config scan: '" + missing + "' is not a directory."
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("missing not-a-directory message; stderr=%q", stderr.String())
	}
}

func TestConfigScanInvalidDepth(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	scanRoot := t.TempDir()
	_, stderr, err := runArgs(t, "config", "scan", scanRoot, "--depth", "0")
	var ec *errExitCode
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("expected errExitCode{code:2}, got %v", err)
	}
	if !strings.Contains(stderr.String(), "hop config scan: --depth must be >= 1.") {
		t.Errorf("missing depth-validation message; stderr=%q", stderr.String())
	}
}

func TestConfigScanZeroReposPrintMode(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{}))

	scanRoot := t.TempDir()
	stdout, stderr, err := runArgs(t, "config", "scan", scanRoot)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	gotErr := stderr.String()
	if !strings.Contains(gotErr, "found 0 repos. Nothing to add.") {
		t.Errorf("missing zero-repos line; stderr=%q", gotErr)
	}
	// Header still printed; existing yaml content still on stdout.
	gotOut := stdout.String()
	if !strings.Contains(gotOut, "# hop config — generated by 'hop config scan "+scanRoot+"'") {
		t.Errorf("missing print-mode header; stdout=%q", gotOut)
	}
	if !strings.Contains(gotOut, "(UTC).") {
		t.Errorf("missing UTC suffix in header; stdout=%q", gotOut)
	}
}

func TestConfigScanConventionMatchPrintMode(t *testing.T) {
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

	stdout, stderr, err := runArgs(t, "config", "scan", scanRoot)
	if err != nil {
		t.Fatalf("scan: %v", err)
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
	// Header references the user-supplied arg verbatim.
	if !strings.Contains(gotOut, "'hop config scan "+scanRoot+"'") {
		t.Errorf("header user-arg mismatch; stdout=%q", gotOut)
	}
}

func TestConfigScanNonConventionInventsGroup(t *testing.T) {
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
	stdout, stderr, err := runArgs(t, "config", "scan", scanRoot)
	if err != nil {
		t.Fatalf("scan: %v", err)
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

func TestConfigScanWriteMode(t *testing.T) {
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

	stdout, stderr, err := runArgs(t, "config", "scan", filepath.Join(home, "code"), "--write")
	if err != nil {
		t.Fatalf("scan --write: %v", err)
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

func TestConfigScanGitMissingPropagates(t *testing.T) {
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

	_, stderr, err := runArgs(t, "config", "scan", filepath.Join(home, "code"))
	if !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), gitMissingHint) {
		t.Errorf("missing git-hint; stderr=%q", stderr.String())
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

func TestConfigScanSlugifyEmptySkipsGracefully(t *testing.T) {
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

	_, stderr, err := runArgs(t, "config", "scan", filepath.Join(home, "elsewhere"))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !strings.Contains(stderr.String(), "skip: ") || !strings.Contains(stderr.String(), "cannot derive group name") {
		t.Errorf("expected slugify-fail skip line; stderr=%q", stderr.String())
	}
}

func TestConfigScanConflictResolutionDirMismatch(t *testing.T) {
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

	stdout, stderr, err := runArgs(t, "config", "scan", filepath.Join(home, "elsewhere"))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !strings.Contains(stdout.String(), "vendor-2:") {
		t.Errorf("expected 'vendor-2' suffix in stdout; stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "note: invented group 'vendor' already exists in hop.yaml") {
		t.Errorf("missing conflict-resolution note; stderr=%q", stderr.String())
	}
}

func TestConfigScanHeaderUTCFormat(t *testing.T) {
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
	stdout, _, err := runArgs(t, "config", "scan", scanRoot)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	dateAfter := time.Now().UTC().Format("2006-01-02")
	if !strings.Contains(stdout.String(), dateBefore+" (UTC).") &&
		!strings.Contains(stdout.String(), dateAfter+" (UTC).") {
		t.Errorf("expected UTC date %q or %q in header; stdout=%q", dateBefore, dateAfter, stdout.String())
	}
}

func TestConfigScanRequiresExactlyOneArg(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	// No positional → cobra ExactArgs(1) error (cobra returns its own error;
	// runArgs returns it without translateExit applied, so we just check err
	// is non-nil).
	_, _, err := runArgs(t, "config", "scan")
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
		"bootstrap a starter hop.yaml",                         // init
		"print the resolved hop.yaml path",                     // where
		"scan a directory for git repos and populate hop.yaml", // scan
		"print the resolved hop.yaml contents to stdout",       // print
	}
	for _, line := range wants {
		if !strings.Contains(gotOut, line) {
			t.Errorf("expected %q in config --help; got:\n%s", line, gotOut)
		}
	}
	// `add` and `rm` are now hidden aliases under config (change mw9h promoted
	// them to top-level). They MUST NOT appear in `config --help`.
	if strings.Contains(gotOut, "register a single on-disk repo into hop.yaml") {
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
