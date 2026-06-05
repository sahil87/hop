package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/hop/internal/proc"
)

// fakeGitNotFound is a GitRunner that always reports git missing on PATH.
func fakeGitNotFound(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return nil, proc.ErrNotFound
}

func TestConfigAddDirNotADirectory(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	missing := "/no/such/path-add-test-xyz"
	_, stderr, err := runArgs(t, "config", "add", missing)
	var ec *errExitCode
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("expected errExitCode{code:2}, got %v", err)
	}
	want := "hop config add: '" + missing + "' is not a directory."
	if !strings.Contains(stderr.String(), want) {
		t.Errorf("missing not-a-directory message; stderr=%q", stderr.String())
	}
}

// TestConfigAddMissingHopYamlAutoInits verifies the gate flip (R3): on a fresh
// machine with no config, `hop config add` no longer errors — it auto-creates a
// minimal skeleton (announced via `created:`) and then proceeds. Here the target
// is a plain (non-git) dir, so after creation the forgiving "not a git repo"
// no-op fires (exit 0).
func TestConfigAddMissingHopYamlAutoInits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "hop", "hop.yaml")

	addDir := t.TempDir()
	_, stderr, err := runArgs(t, "config", "add", addDir)
	if err != nil {
		t.Fatalf("expected forgiving exit 0 after auto-init, got %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "created: "+configPath) {
		t.Errorf("missing created: announcement; stderr=%q", got)
	}
	// The old gate message must be gone.
	if strings.Contains(got, "Run 'hop config init' first") {
		t.Errorf("old init-first gate message still present; stderr=%q", got)
	}
	// The config file was created with the minimal skeleton.
	contents, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("config not created: %v", readErr)
	}
	if string(contents) != "repos: {}\n" {
		t.Errorf("skeleton content = %q, want %q", string(contents), "repos: {}\n")
	}
	// Non-git dir → forgiving no-op message after creation.
	if !strings.Contains(got, "is not a git repo") {
		t.Errorf("missing 'not a git repo' no-op message; stderr=%q", got)
	}
}

// TestTopLevelAddAutoInitsAndRegisters verifies the canonical top-level
// `hop add` auto-creates the config on a fresh machine and registers the repo
// (R3): a real convention repo is added and the `created:`, `added:`, and
// `wrote:` lines all appear.
func TestTopLevelAddAutoInitsAndRegisters(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "hop", "hop.yaml")
	// No config file — auto-init must create it.

	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "add", repoDir)
	if err != nil {
		t.Fatalf("hop add (fresh env): %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "created: "+configPath) {
		t.Errorf("missing created: announcement; stderr=%q", got)
	}
	if !strings.Contains(got, "added: git@github.com:sahil87/hop.git") {
		t.Errorf("missing added line; stderr=%q", got)
	}
	if !strings.Contains(got, "wrote: "+configPath) {
		t.Errorf("missing wrote line; stderr=%q", got)
	}
	contents, _ := os.ReadFile(configPath)
	if !strings.Contains(string(contents), "git@github.com:sahil87/hop.git") {
		t.Errorf("URL not merged into hop.yaml; got:\n%s", contents)
	}
}

// TestAddAutoInitIsIdempotent verifies that a second write-command invocation
// does NOT re-announce `created:` — the file already exists after the first run
// (R3 / idempotency).
func TestAddAutoInitIsIdempotent(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "hop", "hop.yaml")

	// First add: a non-git dir is enough to trigger the auto-init create path.
	plain := t.TempDir()
	_, stderr1, err := runArgs(t, "add", plain)
	if err != nil {
		t.Fatalf("first add: %v", err)
	}
	if !strings.Contains(stderr1.String(), "created: "+configPath) {
		t.Fatalf("first add should announce created:; stderr=%q", stderr1.String())
	}

	// Second add: config now exists → must NOT re-announce created:.
	plain2 := t.TempDir()
	_, stderr2, err := runArgs(t, "add", plain2)
	if err != nil {
		t.Fatalf("second add: %v", err)
	}
	if strings.Contains(stderr2.String(), "created:") {
		t.Errorf("second add re-announced created: (not idempotent); stderr=%q", stderr2.String())
	}
}

func TestConfigAddNonGitDirIsForgiving(t *testing.T) {
	original := "repos:\n  default: []\n"
	yaml := writeReposFixture(t, original)

	// A plain directory with no .git.
	plain := t.TempDir()
	_, stderr, err := runArgs(t, "config", "add", plain)
	if err != nil {
		t.Fatalf("expected forgiving exit 0, got %v", err)
	}
	if !strings.Contains(stderr.String(), "is not a git repo") {
		t.Errorf("expected 'not a git repo' message; stderr=%q", stderr.String())
	}
	// File unchanged.
	got, _ := os.ReadFile(yaml)
	if string(got) != original {
		t.Errorf("hop.yaml modified for non-git dir; got:\n%s", got)
	}
}

func TestConfigAddConventionRepoLandsInDefault(t *testing.T) {
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

	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "config", "add", repoDir)
	if err != nil {
		t.Fatalf("config add: %v", err)
	}
	if !strings.Contains(stderr.String(), "added: git@github.com:sahil87/hop.git") {
		t.Errorf("missing added line; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "wrote: "+hopYaml) {
		t.Errorf("missing wrote line; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(hopYaml)
	gotStr := string(got)
	if !strings.Contains(gotStr, "git@github.com:sahil87/hop.git") {
		t.Errorf("URL not merged into hop.yaml; got:\n%s", gotStr)
	}
	// Convention match → lands under default (no invented group).
	if strings.Contains(gotStr, "hop:") && !strings.Contains(gotStr, "default:") {
		t.Errorf("expected URL under default group; got:\n%s", gotStr)
	}
}

func TestConfigAddNonConventionInventsGroup(t *testing.T) {
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

	// repo at ~/vendor/forks/hop — non-convention; parent basename "forks".
	repoDir := filepath.Join(home, "vendor", "forks", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "config", "add", repoDir)
	if err != nil {
		t.Fatalf("config add: %v", err)
	}
	if !strings.Contains(stderr.String(), "added:") {
		t.Errorf("missing added line; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(hopYaml)
	gotStr := string(got)
	if !strings.Contains(gotStr, "forks:") {
		t.Errorf("expected 'forks:' invented group; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "dir: ~/vendor/forks") {
		t.Errorf("expected ~-substituted dir; got:\n%s", gotStr)
	}
}

func TestConfigAddAlreadyRegisteredIsIdempotent(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "config:\n  code_root: ~/code\nrepos:\n  default:\n    - git@github.com:sahil87/hop.git\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "config", "add", repoDir)
	if err != nil {
		t.Fatalf("expected forgiving exit 0, got %v", err)
	}
	if !strings.Contains(stderr.String(), "already registered") {
		t.Errorf("expected 'already registered' message; stderr=%q", stderr.String())
	}
	// No write: file byte-for-byte unchanged.
	got, _ := os.ReadFile(hopYaml)
	if string(got) != original {
		t.Errorf("hop.yaml modified despite dup; got:\n%s", got)
	}
}

// --- top-level `hop add` (change mw9h) ------------------------------------

// TestTopLevelAddDirNotADirectory mirrors TestConfigAddDirNotADirectory but
// drives the canonical top-level `hop add`, asserting the per-path stderr
// prefix is `hop add:` (NOT `hop config add:`) — Assumption 8 / R8.
func TestTopLevelAddDirNotADirectory(t *testing.T) {
	writeReposFixture(t, "repos:\n  default: []\n")

	missing := "/no/such/path-topadd-test-xyz"
	_, stderr, err := runArgs(t, "add", missing)
	var ec *errExitCode
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("expected errExitCode{code:2}, got %v", err)
	}
	got := stderr.String()
	want := "hop add: '" + missing + "' is not a directory."
	if !strings.Contains(got, want) {
		t.Errorf("expected canonical `hop add:` prefix; want %q, stderr=%q", want, got)
	}
	if strings.Contains(got, "hop config add:") {
		t.Errorf("top-level `hop add` must not emit the alias prefix; stderr=%q", got)
	}
}

// TestTopLevelAddConventionRepoLandsInDefault mirrors the config-add convention
// test but through the top-level command, asserting identical merge behavior
// (R1) — the shared runAdd body backs both spellings.
func TestTopLevelAddConventionRepoLandsInDefault(t *testing.T) {
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

	repoDir := filepath.Join(home, "code", "sahil87", "hop")
	makeRepoDir(t, repoDir)
	canonRepo, _ := filepath.EvalSymlinks(repoDir)
	withFakeGitRunner(t, fakeURLForDir(t, map[string]string{
		canonRepo: "git@github.com:sahil87/hop.git",
	}))

	_, stderr, err := runArgs(t, "add", repoDir)
	if err != nil {
		t.Fatalf("hop add: %v", err)
	}
	if !strings.Contains(stderr.String(), "added: git@github.com:sahil87/hop.git") {
		t.Errorf("missing added line; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "wrote: "+hopYaml) {
		t.Errorf("missing wrote line; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(hopYaml)
	if !strings.Contains(string(got), "git@github.com:sahil87/hop.git") {
		t.Errorf("URL not merged into hop.yaml; got:\n%s", got)
	}
}

func TestConfigAddGitMissingPropagates(t *testing.T) {
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
	withFakeGitRunner(t, fakeGitNotFound)

	_, stderr, err := runArgs(t, "config", "add", repoDir)
	if !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), gitMissingHint) {
		t.Errorf("missing git-hint; stderr=%q", stderr.String())
	}
}
