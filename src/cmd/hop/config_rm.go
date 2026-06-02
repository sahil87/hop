package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
	"github.com/sahil87/hop/internal/fzf"
	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
	"github.com/sahil87/hop/internal/yamled"
)

// rmCmdName is the CLI prefix used in stderr messages for `hop config rm`.
const rmCmdName = "hop config rm"

// pickOne is the seam for the interactive fzf selection. Defaults to fzf.Pick;
// tests inject a fake to exercise the map-back + RemoveURL integration without
// a real fzf binary. Mirrors the listWorktrees / runInteractive seam idiom.
var pickOne = fzf.Pick

// rmLong is the cobra Long help for `hop config rm [--stale]`.
const rmLong = `Remove a registered repo from hop.yaml via an interactive picker.

Pipes the registered repos through fzf; the selected entry's URL is removed
from its group. Removing a group's last URL leaves the (now-empty) group as a
placeholder, so it stays a valid 'hop clone --group' target.

With --stale, the picker is pre-filtered to repos whose resolved path no longer
exists on disk — the quick way to prune entries for repos you have deleted.

Examples:
  hop config rm           pick any registered repo to remove
  hop config rm --stale   pick among only the repos missing from disk`

// newConfigRmCmd returns the cobra factory for `hop config rm [--stale]`.
func newConfigRmCmd() *cobra.Command {
	var stale bool
	cmd := &cobra.Command{
		Use:   "rm [--stale]",
		Short: "remove a registered repo from hop.yaml via an interactive picker",
		Long:  rmLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigRm(cmd, stale)
		},
	}
	cmd.Flags().BoolVar(&stale, "stale", false, "limit the picker to repos whose resolved path no longer exists on disk")
	return cmd
}

// runConfigRm loads the registry, optionally filters to stale repos, runs the
// fzf picker, maps the selection back to its Repo, and removes that repo's URL
// from its group via yamled.RemoveURL. Returns errSilent / errFzfCancelled on
// the relevant failure paths; nil on success and on every forgiving no-op
// (nothing to remove, nothing stale, RemoveURL not-found).
func runConfigRm(cmd *cobra.Command, stale bool) error {
	stderr := cmd.ErrOrStderr()

	// Precondition: resolve hop.yaml. On miss, mirror scan/add's two-line
	// message pointing at ResolveWriteTarget; on load failure, prefix with the
	// command name. Both return errSilent (consistent stderr voice).
	configPath, err := config.Resolve()
	if err != nil {
		bootstrap, werr := config.ResolveWriteTarget()
		if werr != nil {
			bootstrap = "$XDG_CONFIG_HOME/hop/hop.yaml"
		}
		fmt.Fprintf(stderr, "%s: no hop.yaml found at %s.\nRun 'hop config init' first, then re-run rm.\n", rmCmdName, bootstrap)
		return errSilent
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", rmCmdName, err)
		return errSilent
	}
	rs, err := repos.FromConfig(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", rmCmdName, err)
		return errSilent
	}

	candidates := rs
	if stale {
		candidates = staleRepos(rs)
		if len(candidates) == 0 {
			fmt.Fprintf(stderr, "%s: nothing stale — every registered repo exists on disk.\n", rmCmdName)
			return nil
		}
	}
	if len(candidates) == 0 {
		fmt.Fprintf(stderr, "%s: no repos registered in %s. Nothing to remove.\n", rmCmdName, configPath)
		return nil
	}

	repo, err := pickRepo(candidates)
	if err != nil {
		if errors.Is(err, errFzfMissing) {
			fmt.Fprintln(stderr, fzfMissingHint)
			return errSilent
		}
		// errFzfCancelled (Esc/Ctrl-C) propagates to translateExit → exit 130.
		return err
	}

	if err := yamled.RemoveURL(configPath, repo.Group, repo.URL); err != nil {
		// Forgiving not-found (Assumption 10): report + exit 0.
		if errors.Is(err, yamled.ErrURLNotFound) || errors.Is(err, yamled.ErrGroupNotFound) {
			fmt.Fprintf(stderr, "%s: %s not found in %s. Nothing to remove.\n", rmCmdName, repo.URL, configPath)
			return nil
		}
		fmt.Fprintf(stderr, "%s: removing from %s failed: %v\n", rmCmdName, configPath, err)
		return errSilent
	}

	fmt.Fprintf(stderr, "removed: %s\nwrote: %s\n", repo.URL, configPath)
	return nil
}

// staleRepos returns the subset of rs whose resolved Path does not exist on
// disk. Per Assumption 8 this checks the repo's own Path only — worktrees are
// not consulted.
func staleRepos(rs repos.Repos) repos.Repos {
	var out repos.Repos
	for _, r := range rs {
		if _, err := os.Stat(r.Path); errors.Is(err, os.ErrNotExist) {
			out = append(out, r)
		}
	}
	return out
}

// pickRepo builds the picker lines, runs the fzf selection (single-select), and
// maps the chosen line back to its source Repo by the unique path column — the
// same match-back approach resolve.go uses. Returns errFzfMissing /
// errFzfCancelled (translated by callers).
func pickRepo(rs repos.Repos) (*repos.Repo, error) {
	lines := buildPickerLines(rs)
	selected, err := pickOne(context.Background(), lines, "")
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			return nil, errFzfMissing
		}
		if code, ok := proc.ExitCode(err); ok && code == 130 {
			return nil, errFzfCancelled
		}
		return nil, fmt.Errorf("%s: fzf failed: %w", rmCmdName, err)
	}

	// The selected line is the tab-delimited triple we piped in
	// (display\tpath\turl); Path is unique per repo, so the path column
	// disambiguates repos that share a derived name.
	parts := strings.SplitN(selected, "\t", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("%s: malformed fzf selection %q", rmCmdName, selected)
	}
	chosenPath := parts[1]
	for i := range rs {
		if rs[i].Path == chosenPath {
			return &rs[i], nil
		}
	}
	return nil, fmt.Errorf("%s: selection %q not found in repo list", rmCmdName, selected)
}
