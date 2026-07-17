package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
	"github.com/sahil87/hop/internal/fzf"
	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
	"github.com/sahil87/hop/internal/yamled"
)

// pickOne is the seam for the interactive fzf selection. Defaults to fzf.Pick;
// tests inject a fake to exercise the map-back + RemoveURL integration without
// a real fzf binary. Mirrors the listWorktrees / runInteractive seam idiom.
var pickOne = fzf.Pick

// dryRunNoChanges is the trailing stderr line a `--dry-run` removal prints in
// place of the live `removed:`/`wrote:` status lines, signalling that the
// preview shared the real resolution path but wrote nothing (principle №5:
// destructive writes support an accurate --dry-run that touches no file).
const dryRunNoChanges = "dry-run: no changes written"

// rmLong is the cobra Long help for `hop rm [<name>]` and its hidden alias
// `hop config rm [--stale]`.
const rmLong = `Remove a registered repo from hop.yaml.

With no argument, pipes the registered repos through fzf and removes the
selected entry's URL from its group. With a <name>, resolves it via the same
match-or-fzf algorithm used by 'hop <name> where' and removes that entry
directly — naming a repo prunes it even if its folder is already gone. Removal
always targets a whole repo entry; any '/<worktree>' suffix on <name> is
ignored (worktrees are not registry entries).

Removing a group's last URL leaves the (now-empty) group as a placeholder, so
it stays a valid 'hop clone --group' target.

With --stale, the picker is pre-filtered to repos whose resolved path no longer
exists on disk — the quick way to prune entries for repos you have deleted.
--stale is a picker-scoping flag and cannot be combined with a <name>.

With --dry-run, the target is resolved through the same path as a live removal
but nothing is written — hop reports which entry it would remove and exits 0,
leaving hop.yaml untouched.

Examples:
  hop rm                 pick any registered repo to remove
  hop rm widget          remove the repo matching 'widget' directly (no picker)
  hop rm --stale         pick among only the repos missing from disk
  hop rm widget --dry-run  preview the removal without writing hop.yaml`

// newRmCmd returns the cobra factory for the canonical top-level
// `hop rm [<name>]`. With no positional it drives the fzf picker; with a
// <name> it resolves via resolveByName and removes directly. --stale combined
// with a positional is a usage error (exit 2).
func newRmCmd() *cobra.Command {
	var stale bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "rm [<name>]",
		Short: "remove a registered repo from hop.yaml",
		Long:  rmLong,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			if name != "" && stale {
				return &errExitCode{code: 2, msg: "hop rm: --stale cannot be combined with a repo name."}
			}
			return runRm(cmd, "hop rm", stale, dryRun, name)
		},
	}
	cmd.Flags().BoolVar(&stale, "stale", false, "limit the picker to repos whose resolved path no longer exists on disk")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the removal without writing hop.yaml")
	return cmd
}

// newConfigRmCmd returns the cobra factory for the hidden alias
// `hop config rm [--stale]`. It shares runRm with the canonical top-level
// command but is Hidden, accepts no positional (the historical NoArgs shape),
// and keeps emitting its "hop config rm:" stderr prefix.
func newConfigRmCmd() *cobra.Command {
	var stale bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:    "rm [--stale]",
		Short:  "remove a registered repo from hop.yaml via an interactive picker",
		Long:   rmLong,
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRm(cmd, "hop config rm", stale, dryRun, "")
		},
	}
	cmd.Flags().BoolVar(&stale, "stale", false, "limit the picker to repos whose resolved path no longer exists on disk")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the removal without writing hop.yaml")
	return cmd
}

// runRm removes a registered repo from hop.yaml. cmdName is the per-path stderr
// prefix ("hop rm" for the canonical command, "hop config rm" for the hidden
// alias). When name is non-empty it resolves the repo via resolveByName and
// removes it directly (skipping the picker, no on-disk check, no prompt);
// otherwise it loads the registry, optionally filters to stale repos, and runs
// the fzf picker. When dryRun is set, the target is resolved via the same path
// as a live removal but the YAML write is skipped (principle №5 preview).
// Returns errSilent / errFzfCancelled on the relevant failure paths; nil on
// success and on every forgiving no-op (nothing to remove, nothing stale,
// RemoveURL not-found).
func runRm(cmd *cobra.Command, cmdName string, stale, dryRun bool, name string) error {
	stderr := cmd.ErrOrStderr()

	// Precondition: resolve hop.yaml. On miss, mirror scan/add's two-line
	// message pointing at ResolveWriteTarget; on load failure, prefix with the
	// command name. Both return errSilent (consistent stderr voice).
	configPath, err := config.Resolve()
	if err != nil {
		bootstrap, werr := config.ResolveWriteTarget()
		if werr != nil {
			// The config path can't even be computed (only happens when $HOME
			// is unset). Surface the original resolver error so the user gets
			// the actionable cause instead of a misleading "no hop.yaml found".
			fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
			return errSilent
		}
		fmt.Fprintf(stderr, "%s: no hop.yaml found at %s.\nRun 'hop config init' first, then re-run rm.\n", cmdName, bootstrap)
		return errSilent
	}

	// Positional path (new): resolve <name> via the shared match-or-fzf helper
	// and remove it directly — no picker, no on-disk check, no prompt. The entry
	// is removed regardless of whether the folder still exists.
	if name != "" {
		// Strip any "/<wt>" worktree suffix before resolving. Removal operates on
		// the registry (a repo's URL in hop.yaml); worktrees are not registry
		// entries, so the suffix is meaningless here — and resolveByName's
		// worktree branch would otherwise force an on-disk clone + `wt list`
		// check (breaking the "no on-disk check" guarantee) and then remove the
		// whole parent repo anyway. Stripping makes the guarantee hold for every
		// input and removes that footgun. Repo names in hop.yaml are URL
		// basenames with no "/", so a first-"/" split is unambiguous.
		repoName := name
		if idx := strings.Index(repoName, "/"); idx >= 0 {
			repoName = repoName[:idx]
		}
		repo, err := resolveOne(cmd, repoName)
		if err != nil {
			// resolveOne already wrote fzfMissingHint + returned errSilent on
			// missing fzf; errFzfCancelled and *errExitCode propagate verbatim.
			return err
		}
		return removeRepo(stderr, cmdName, configPath, repo, dryRun)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return errSilent
	}
	rs, err := repos.FromConfig(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return errSilent
	}

	candidates := rs
	if stale {
		candidates = staleRepos(rs)
		if len(candidates) == 0 {
			fmt.Fprintf(stderr, "%s: nothing stale — every registered repo exists on disk.\n", cmdName)
			return nil
		}
	}
	if len(candidates) == 0 {
		fmt.Fprintf(stderr, "%s: no repos registered in %s. Nothing to remove.\n", cmdName, configPath)
		return nil
	}

	repo, err := pickRepo(cmdName, candidates)
	if err != nil {
		if errors.Is(err, errFzfMissing) {
			fmt.Fprintln(stderr, fzfMissingHint)
			return errSilent
		}
		// errFzfCancelled (Esc/Ctrl-C) propagates to translateExit → exit 130.
		return err
	}

	return removeRepo(stderr, cmdName, configPath, repo, dryRun)
}

// removeRepo drops repo's URL from its group via yamled.RemoveURL and emits the
// shared status lines. The forgiving not-found case (Assumption 10) reports +
// exits 0; a real write failure returns errSilent. Shared by the picker path
// and the positional <name> path so both speak the same stderr voice.
//
// When dryRun is set, it previews via yamled.WouldRemoveURL — the read-only
// half of RemoveURL, so the same locate/not-found contract applies — and writes
// nothing: it prints `would remove:` (+ a `dry-run: no changes written` line)
// on a would-succeed, or the same forgiving "Nothing to remove." on a not-found,
// exiting 0 in both cases (principle №5 preview).
func removeRepo(stderr io.Writer, cmdName, configPath string, repo *repos.Repo, dryRun bool) error {
	if dryRun {
		if err := yamled.WouldRemoveURL(configPath, repo.Group, repo.URL); err != nil {
			if errors.Is(err, yamled.ErrURLNotFound) || errors.Is(err, yamled.ErrGroupNotFound) {
				fmt.Fprintf(stderr, "%s: %s not found in %s. Nothing to remove.\n", cmdName, repo.URL, configPath)
				return nil
			}
			fmt.Fprintf(stderr, "%s: previewing removal from %s failed: %v\n", cmdName, configPath, err)
			return errSilent
		}
		fmt.Fprintf(stderr, "would remove: %s\n%s\n", repo.URL, dryRunNoChanges)
		return nil
	}
	if err := yamled.RemoveURL(configPath, repo.Group, repo.URL); err != nil {
		if errors.Is(err, yamled.ErrURLNotFound) || errors.Is(err, yamled.ErrGroupNotFound) {
			fmt.Fprintf(stderr, "%s: %s not found in %s. Nothing to remove.\n", cmdName, repo.URL, configPath)
			return nil
		}
		fmt.Fprintf(stderr, "%s: removing from %s failed: %v\n", cmdName, configPath, err)
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
// same match-back approach resolve.go uses. cmdName is the per-path prefix used
// in error wording. Returns errFzfMissing / errFzfCancelled (translated by
// callers).
func pickRepo(cmdName string, rs repos.Repos) (*repos.Repo, error) {
	// TTY guard: the no-name `hop rm` / `hop config rm` picker needs a terminal.
	// With no TTY, fail fast with the distinct errNoTTY sentinel instead of
	// spawning fzf (intake Item 3 — single guard point per fzf seam).
	if !isTTY() {
		return nil, errNoTTY
	}
	lines := buildPickerLines(rs)
	selected, err := pickOne(context.Background(), lines, "")
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			return nil, errFzfMissing
		}
		if code, ok := proc.ExitCode(err); ok && code == 130 {
			return nil, errFzfCancelled
		}
		return nil, fmt.Errorf("%s: fzf failed: %w", cmdName, err)
	}

	// The selected line is the tab-delimited triple we piped in
	// (display\tpath\turl); Path is unique per repo, so the path column
	// disambiguates repos that share a derived name.
	parts := strings.SplitN(selected, "\t", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("%s: malformed fzf selection %q", cmdName, selected)
	}
	chosenPath := parts[1]
	for i := range rs {
		if rs[i].Path == chosenPath {
			return &rs[i], nil
		}
	}
	return nil, fmt.Errorf("%s: selection %q not found in repo list", cmdName, selected)
}
