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
	dir := t.TempDir()
	yaml := filepath.Join(dir, "hop.yaml")
	if err := os.WriteFile(yaml, []byte("repos:\n  default: []\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("HOP_CONFIG", yaml)

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

func TestConfigAddMissingHopYaml(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-hop.yaml")
	t.Setenv("HOP_CONFIG", missing)

	addDir := t.TempDir()
	_, stderr, err := runArgs(t, "config", "add", addDir)
	if err == nil || !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent, got %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "no hop.yaml found at "+missing) {
		t.Errorf("missing-config message not found; stderr=%q", got)
	}
	if !strings.Contains(got, "Run 'hop config init' first") {
		t.Errorf("missing init hint; stderr=%q", got)
	}
}

func TestConfigAddNonGitDirIsForgiving(t *testing.T) {
	dir := t.TempDir()
	yaml := filepath.Join(dir, "hop.yaml")
	original := "repos:\n  default: []\n"
	if err := os.WriteFile(yaml, []byte(original), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("HOP_CONFIG", yaml)

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
	t.Setenv("HOP_CONFIG", hopYaml)

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
	t.Setenv("HOP_CONFIG", hopYaml)

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
	t.Setenv("HOP_CONFIG", hopYaml)

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
	t.Setenv("HOP_CONFIG", hopYaml)

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
