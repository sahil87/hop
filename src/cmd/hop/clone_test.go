package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestCloneStateAlreadyCloned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myrepo")
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := cloneState(path)
	if err != nil {
		t.Fatalf("cloneState: %v", err)
	}
	if got != stateAlreadyCloned {
		t.Fatalf("expected stateAlreadyCloned, got %v", got)
	}
}

func TestCloneStateMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope")
	got, err := cloneState(path)
	if err != nil {
		t.Fatalf("cloneState: %v", err)
	}
	if got != stateMissing {
		t.Fatalf("expected stateMissing, got %v", got)
	}
}

func TestCloneStatePathExistsNotGit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plainfile")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := cloneState(path)
	if err != nil {
		t.Fatalf("cloneState: %v", err)
	}
	if got != statePathExistsNotGit {
		t.Fatalf("expected statePathExistsNotGit, got %v", got)
	}
}

func TestLooksLikeURL(t *testing.T) {
	cases := map[string]bool{
		"git@github.com:sahil87/hop.git": true,
		"https://github.com/sahil87/hop": true,
		"ssh://git@host/repo.git":        true,
		"file:///tmp/foo":                true,
		"hop":                            false,
		"some-name":                      false,
		"weird:colon-in-name":            false, // no @ — treated as name
		"weird@host":                     false, // no : — treated as name
	}
	for in, want := range cases {
		if got := looksLikeURL(in); got != want {
			t.Errorf("looksLikeURL(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestCloneURLMissingGroupErrors verifies the typo-catching contract is
// preserved (R6 / Assumption 9): against a PRE-EXISTING config, a typo'd
// --group still errors via findGroup==nil. EnsureGroup runs only on a freshly
// auto-created skeleton, so it does NOT fire here (the config already exists).
func TestCloneURLMissingGroupErrors(t *testing.T) {
	yaml := `repos:
  default:
    - git@github.com:sahil87/hop.git
`
	configPath := writeReposFixture(t, yaml)

	_, stderr, err := runArgs(t, "clone", "--group", "nonexistent", "git@github.com:foo/bar.git")
	if err == nil {
		t.Fatalf("expected error for missing group")
	}
	if !strings.Contains(stderr.String(), "no 'nonexistent' group") {
		t.Fatalf("expected missing-group message, got %q", stderr.String())
	}
	// The typo'd group must NOT have been created on the existing config.
	if strings.Contains(stderr.String(), "created:") {
		t.Errorf("auto-init fired on a pre-existing config; stderr=%q", stderr.String())
	}
	contents, _ := os.ReadFile(configPath)
	if strings.Contains(string(contents), "nonexistent") {
		t.Errorf("typo'd group was created on existing config; got:\n%s", contents)
	}
}

// TestCloneURLFreshEnvAutoInitsDefaultGroup verifies that `hop clone <url>` on a
// fresh machine (no config) auto-creates the config, ensures the default group
// exists, clones the repo, and registers the URL (R6). Uses a local bare repo to
// avoid the network.
func TestCloneURLFreshEnvAutoInitsDefaultGroup(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "hop", "hop.yaml")

	url, _ := initBareRepo(t, t.TempDir())

	_, stderr, err := runArgs(t, "clone", url)
	if err != nil {
		t.Fatalf("clone <url> (fresh env): %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "created: "+configPath) {
		t.Errorf("missing created: announcement; stderr=%q", got)
	}
	// The default group exists and the URL is registered under it.
	urls := readYAMLURLs(t, configPath, "default")
	found := false
	for _, u := range urls {
		if u == url {
			found = true
		}
	}
	if !found {
		t.Errorf("URL %q not registered under default; got %v", url, urls)
	}
}

// TestCloneURLFreshEnvCreatesNamedGroup verifies that `--group <name>` on a fresh
// machine creates that group (not just default) and registers the URL there (R6).
func TestCloneURLFreshEnvCreatesNamedGroup(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "hop", "hop.yaml")

	url, _ := initBareRepo(t, t.TempDir())

	_, stderr, err := runArgs(t, "clone", "--group", "vendor", url)
	if err != nil {
		t.Fatalf("clone --group vendor (fresh env): %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "created: "+configPath) {
		t.Errorf("missing created: announcement; stderr=%q", got)
	}
	// The vendor group was created and the URL registered there.
	urls := readYAMLURLs(t, configPath, "vendor")
	if len(urls) != 1 || urls[0] != url {
		t.Errorf("expected vendor.urls = [%s], got %v", url, urls)
	}
}

// TestCloneURLFreshEnvNoAddStillCreatesConfig pins the intentional interaction
// between --no-add and fresh-env auto-init: --no-add suppresses the URL
// write-back, NOT the config file's creation. The skeleton + target group are
// still needed for path resolution (findGroup / resolveAdHocPath), so the file
// is created and announced even though nothing is registered into it. This is a
// deliberate behavior (R6 does not carve out --no-add); the test exists to keep
// it from changing silently.
func TestCloneURLFreshEnvNoAddStillCreatesConfig(t *testing.T) {
	clearConfigEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "hop", "hop.yaml")

	url, _ := initBareRepo(t, t.TempDir())

	_, stderr, err := runArgs(t, "clone", "--no-add", url)
	if err != nil {
		t.Fatalf("clone --no-add <url> (fresh env): %v\nstderr: %s", err, stderr.String())
	}
	// The config file is still created and announced.
	if got := stderr.String(); !strings.Contains(got, "created: "+configPath) {
		t.Errorf("missing created: announcement; stderr=%q", got)
	}
	if _, statErr := os.Stat(configPath); statErr != nil {
		t.Fatalf("expected config created at %s: %v", configPath, statErr)
	}
	// But --no-add means the URL is NOT registered: default exists (ensured) yet empty.
	urls := readYAMLURLs(t, configPath, "default")
	if len(urls) != 0 {
		t.Errorf("expected default empty (--no-add suppresses write-back), got %v", urls)
	}
}

// initBareRepo creates a bare git repo at <dir>/source.git and returns its
// file:// URL. Skips the test if `git` is not on PATH.
func initBareRepo(t *testing.T, dir string) (url, srcPath string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	srcPath = filepath.Join(dir, "source.git")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "init", "--bare", srcPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\noutput: %s", err, out)
	}
	url = "file://" + srcPath
	return url, srcPath
}

// fixtureGroup configures hop.yaml with a single group, ad-hoc-clone friendly:
// dir is a temp directory the test owns. Returns hop.yaml path and the dir.
func fixtureGroup(t *testing.T, group string, dir bool) (configPath, groupDir string) {
	t.Helper()
	groupDir = t.TempDir()
	var yaml string
	if dir {
		yaml = "repos:\n  " + group + ":\n    dir: " + groupDir + "\n    urls: []\n"
	} else {
		yaml = "config:\n  code_root: " + groupDir + "\nrepos:\n  " + group + ": []\n"
	}
	configPath = writeReposFixture(t, yaml)
	return configPath, groupDir
}

// readYAMLURLs returns the URL list for the given group at path. Supports both
// flat-list and map-shaped groups.
func readYAMLURLs(t *testing.T, path, group string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(root.Content) == 0 {
		return nil
	}
	top := root.Content[0]
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "repos" {
			repos := top.Content[i+1]
			for j := 0; j+1 < len(repos.Content); j += 2 {
				if repos.Content[j].Value != group {
					continue
				}
				body := repos.Content[j+1]
				if body.Kind == yaml.SequenceNode {
					var out []string
					for _, c := range body.Content {
						out = append(out, c.Value)
					}
					return out
				}
				if body.Kind == yaml.MappingNode {
					for k := 0; k+1 < len(body.Content); k += 2 {
						if body.Content[k].Value == "urls" {
							v := body.Content[k+1]
							var out []string
							for _, c := range v.Content {
								out = append(out, c.Value)
							}
							return out
						}
					}
				}
			}
		}
	}
	return nil
}

func TestCloneURLAdHocHappyPath(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepo(t, tmp)
	configPath, groupDir := fixtureGroup(t, "default", true)

	stdout, stderr, err := runArgs(t, "clone", url)
	if err != nil {
		t.Fatalf("clone <url>: %v\nstderr: %s", err, stderr.String())
	}

	// stdout: the resolved path
	gotPath := strings.TrimSpace(stdout.String())
	wantPath := filepath.Join(groupDir, "source")
	if gotPath != wantPath {
		t.Fatalf("stdout = %q, want %q", gotPath, wantPath)
	}

	// On disk: the path exists and has .git
	if _, err := os.Stat(filepath.Join(wantPath, ".git")); err != nil {
		t.Fatalf("expected cloned repo at %s: %v", wantPath, err)
	}

	// hop.yaml: URL appended
	urls := readYAMLURLs(t, configPath, "default")
	found := false
	for _, u := range urls {
		if u == url {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("URL %q not found in default.urls; got %v", url, urls)
	}
}

func TestCloneURLNoAdd(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepo(t, tmp)
	configPath, groupDir := fixtureGroup(t, "default", true)

	stdout, _, err := runArgs(t, "clone", "--no-add", url)
	if err != nil {
		t.Fatalf("clone --no-add: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != filepath.Join(groupDir, "source") {
		t.Errorf("stdout = %q", got)
	}

	// hop.yaml NOT modified
	urls := readYAMLURLs(t, configPath, "default")
	if len(urls) != 0 {
		t.Errorf("expected hop.yaml unchanged, got urls = %v", urls)
	}
}

func TestCloneURLNoCD(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepo(t, tmp)
	configPath, _ := fixtureGroup(t, "default", true)

	stdout, _, err := runArgs(t, "clone", "--no-cd", url)
	if err != nil {
		t.Fatalf("clone --no-cd: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Errorf("expected empty stdout, got %q", got)
	}

	// hop.yaml IS updated
	urls := readYAMLURLs(t, configPath, "default")
	if len(urls) != 1 || urls[0] != url {
		t.Errorf("expected default.urls = [%s], got %v", url, urls)
	}
}

func TestCloneURLNameOverride(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepo(t, tmp)
	_, groupDir := fixtureGroup(t, "default", true)

	stdout, _, err := runArgs(t, "clone", "--name", "my-fork", url)
	if err != nil {
		t.Fatalf("clone --name: %v", err)
	}
	wantPath := filepath.Join(groupDir, "my-fork")
	if got := strings.TrimSpace(stdout.String()); got != wantPath {
		t.Errorf("stdout = %q, want %q", got, wantPath)
	}
	if _, err := os.Stat(filepath.Join(wantPath, ".git")); err != nil {
		t.Errorf("expected cloned repo at %s: %v", wantPath, err)
	}
}

func TestCloneURLGroupOverride(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepo(t, tmp)
	vendorDir := t.TempDir()
	yaml := "repos:\n  default: []\n  vendor:\n    dir: " + vendorDir + "\n    urls: []\n"
	configPath := writeReposFixture(t, yaml)

	stdout, _, err := runArgs(t, "clone", "--group", "vendor", url)
	if err != nil {
		t.Fatalf("clone --group vendor: %v", err)
	}
	wantPath := filepath.Join(vendorDir, "source")
	if got := strings.TrimSpace(stdout.String()); got != wantPath {
		t.Errorf("stdout = %q, want %q", got, wantPath)
	}

	// URL appended to vendor.urls, not default
	vendorURLs := readYAMLURLs(t, configPath, "vendor")
	if len(vendorURLs) != 1 || vendorURLs[0] != url {
		t.Errorf("expected vendor.urls = [%s], got %v", url, vendorURLs)
	}
	defaultURLs := readYAMLURLs(t, configPath, "default")
	if len(defaultURLs) != 0 {
		t.Errorf("expected default.urls empty, got %v", defaultURLs)
	}
}

func TestCloneURLAlreadyCloned(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepo(t, tmp)
	configPath, groupDir := fixtureGroup(t, "default", true)

	// Pre-create the target with a .git dir.
	target := filepath.Join(groupDir, "source")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	stdout, stderr, err := runArgs(t, "clone", url)
	if err != nil {
		t.Fatalf("clone (already cloned): %v", err)
	}
	if !strings.Contains(stderr.String(), "skip: already cloned at "+target) {
		t.Errorf("expected skip message, got stderr: %s", stderr.String())
	}
	// stdout still prints the path.
	if got := strings.TrimSpace(stdout.String()); got != target {
		t.Errorf("stdout = %q, want %q", got, target)
	}
	// URL still appended (registers existing checkout).
	urls := readYAMLURLs(t, configPath, "default")
	if len(urls) != 1 || urls[0] != url {
		t.Errorf("expected default.urls = [%s], got %v", url, urls)
	}
}

func TestCloneURLDuplicateInGroup(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepo(t, tmp)
	groupDir := t.TempDir()

	// Pre-populate hop.yaml with the URL already in default.
	yaml := "repos:\n  default:\n    dir: " + groupDir + "\n    urls:\n      - " + url + "\n"
	configPath := writeReposFixture(t, yaml)

	// Pre-clone the target so we hit stateAlreadyCloned (which is the path
	// where the duplicate-URL skip is observable).
	target := filepath.Join(groupDir, "source")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, stderr, err := runArgs(t, "clone", url)
	if err != nil {
		t.Fatalf("clone (duplicate): %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already registered in 'default'") {
		t.Errorf("expected duplicate-registered message, got: %s", stderr.String())
	}

	// hop.yaml still has only one entry — no duplicate appended.
	urls := readYAMLURLs(t, configPath, "default")
	if len(urls) != 1 {
		t.Errorf("expected 1 url, got %d: %v", len(urls), urls)
	}
}

func TestCloneURLPathConflict(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepo(t, tmp)
	configPath, groupDir := fixtureGroup(t, "default", true)

	// Pre-create a non-git directory at the target.
	target := filepath.Join(groupDir, "source")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Drop a regular file inside so the dir is not empty (not strictly required).
	if err := os.WriteFile(filepath.Join(target, "README"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, stderr, err := runArgs(t, "clone", url)
	if err == nil {
		t.Fatal("expected error for path-exists-not-git")
	}
	if !strings.Contains(stderr.String(), "exists but is not a git repo") {
		t.Errorf("expected path-conflict message, got: %s", stderr.String())
	}

	// hop.yaml unchanged.
	urls := readYAMLURLs(t, configPath, "default")
	if len(urls) != 0 {
		t.Errorf("expected default.urls empty, got %v", urls)
	}
}
