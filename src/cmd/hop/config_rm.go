package main

import (
	"bufio"
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

// abortedNoChanges is the stderr line printed when the interactive consent
// prompt on `hop rm <name>` is declined (bare Enter — the [y/N] default — or any
// non-affirmative input). An answered "no" is a benign no-op, so removal writes
// nothing and exits 0 (hop's forgiving exit-0 convention), NOT 130 (that is an
// fzf-style user cancellation, which this is not).
const abortedNoChanges = "aborted: no changes written"

// errConsentRequired signals that `hop rm <name>` reached the consent gate with
// no controlling TTY to prompt on and no `--yes` (and no `--dry-run`, which is
// checked first and needs no consent). It maps to exit 3 in translateExit
// (main.go) — reusing hop's documented "a terminal was required and none is
// present" code — but carries a consent-specific message (consentRequiredMsg)
// naming --yes, NOT the generic noTTYHint (whose "pass a repo name" advice is
// wrong here: a name was already passed). It is distinct from errNoTTY (the
// picker's no-TTY refusal) so each surfaces the correct next step.
var errConsentRequired = errors.New("consent required for removal")

// consentRequiredMsg is the exact stderr line printed on the no-TTY consent
// refusal (change clc4). It follows the what/why/next shape and hop's
// cmdName-prefixed stderr voice; the prefix is threaded in by runRm.
const consentRequiredMsg = "consent required for removal — re-run with --yes (or preview with --dry-run)"

// rmLong is the cobra Long help for `hop rm [<name>]` and its hidden alias
// `hop config rm [--stale]`.
const rmLong = `Remove a registered repo from hop.yaml.

With no argument, pipes the registered repos through fzf and removes the
selected entry's URL from its group. With a <name>, resolves it via the same
match-or-fzf algorithm used by 'hop <name> where' and removes that entry
directly — naming a repo prunes it even if its folder is already gone. Removal
always targets a whole repo entry; any '/<worktree>' suffix on <name> is
ignored (worktrees are not registry entries).

Because 'hop rm <name>' writes the registry, it asks for consent first. On a
terminal it shows the resolved match and prompts 'Proceed? [y/N]' (default No);
answer y/yes to remove, anything else aborts with no change. Pass --yes/-y to
skip the prompt (for scripts and agents). With no terminal and no --yes, the
removal is refused (exit 3) rather than run unattended — re-run with --yes, or
preview with --dry-run. The interactive picker (no <name>) needs no prompt: the
pick itself is the consent.

Removing a group's last URL leaves the (now-empty) group as a placeholder, so
it stays a valid 'hop clone --group' target.

With --stale, the picker is pre-filtered to repos whose resolved path no longer
exists on disk — the quick way to prune entries for repos you have deleted.
--stale is a picker-scoping flag and cannot be combined with a <name>.

With --dry-run, the target is resolved through the same path as a live removal
but nothing is written — hop reports which entry it would remove and exits 0,
leaving hop.yaml untouched. --dry-run needs no consent (it writes nothing), so
it is never prompted or refused.

Examples:
  hop rm                 pick any registered repo to remove
  hop rm widget          remove the repo matching 'widget' (prompts on a terminal)
  hop rm widget --yes    remove without the confirmation prompt
  hop rm --stale         pick among only the repos missing from disk
  hop rm widget --dry-run  preview the removal without writing hop.yaml`

// newRmCmd returns the cobra factory for the canonical top-level
// `hop rm [<name>]`. With no positional it drives the fzf picker; with a
// <name> it resolves via resolveByName and removes directly. --stale combined
// with a positional is a usage error (exit 2).
func newRmCmd() *cobra.Command {
	var stale bool
	var dryRun bool
	var yes bool
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
			return runRm(cmd, "hop rm", stale, dryRun, yes, name)
		},
	}
	cmd.Flags().BoolVar(&stale, "stale", false, "limit the picker to repos whose resolved path no longer exists on disk")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the removal without writing hop.yaml")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt on 'hop rm <name>' (consent for automation)")
	return cmd
}

// newConfigRmCmd returns the cobra factory for the hidden alias
// `hop config rm [--stale] [--dry-run]`. It shares runRm with the canonical
// top-level command but is Hidden, accepts no positional (the historical NoArgs
// shape), and keeps emitting its "hop config rm:" stderr prefix. It registers NO
// --yes flag: the alias is picker-only, so it has no consent point (its consent
// is the pick), and a meaningless flag would be surface bloat (Constitution VI).
// It passes yes=false to runRm, which is irrelevant on the picker path anyway.
func newConfigRmCmd() *cobra.Command {
	var stale bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:    "rm [--stale] [--dry-run]",
		Short:  "remove a registered repo from hop.yaml via an interactive picker",
		Long:   rmLong,
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRm(cmd, "hop config rm", stale, dryRun, false, "")
		},
	}
	cmd.Flags().BoolVar(&stale, "stale", false, "limit the picker to repos whose resolved path no longer exists on disk")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the removal without writing hop.yaml")
	return cmd
}

// runRm removes a registered repo from hop.yaml. cmdName is the per-path stderr
// prefix ("hop rm" for the canonical command, "hop config rm" for the hidden
// alias). When name is non-empty it resolves the repo via resolveByName and
// removes it directly (skipping the picker, no on-disk check) — gated by a
// consent step (change clc4): --dry-run needs no consent (checked first), then
// --yes skips the prompt, then a missing TTY refuses (errConsentRequired →
// exit 3), else an interactive [y/N] prompt runs. When name is empty it loads
// the registry, optionally filters to stale repos, and runs the fzf picker (the
// pick is itself the consent, so yes is ignored there). When dryRun is set, the
// target is resolved via the same path as a live removal but the YAML write is
// skipped (principle №5 preview). Returns errSilent / errFzfCancelled /
// errConsentRequired on the relevant failure paths; nil on success and on every
// forgiving no-op (nothing to remove, nothing stale, RemoveURL not-found, a
// declined prompt).
func runRm(cmd *cobra.Command, cmdName string, stale, dryRun, yes bool, name string) error {
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
		// Consent gate (change clc4). Order matters:
		//   1. --dry-run writes nothing, so it needs no consent — take the
		//      (unchanged) preview path before the gate.
		//   2. --yes is flag-based consent (principle №1) — skip the prompt.
		//   3. No TTY and no --yes → refuse fast (no hang, no unattended write)
		//      with a consent-specific message naming --yes → exit 3.
		//   4. Otherwise prompt on the terminal; a declined prompt is a benign
		//      exit-0 no-op.
		if !dryRun && !yes {
			if !isTTY() {
				fmt.Fprintf(stderr, "%s: %s\n", cmdName, consentRequiredMsg)
				return errConsentRequired
			}
			if !confirmRemoval(cmd, stderr, repo) {
				fmt.Fprintln(stderr, abortedNoChanges)
				return nil
			}
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

// confirmRemoval runs the interactive consent prompt for `hop rm <name>` on a
// terminal (change clc4). It writes the resolved match preview and a
// `Proceed? [y/N]` prompt to stderr (stdout stays empty — principle №2), then
// reads a single line from cmd.InOrStdin() (cobra's injectable stdin, so
// seam-injected tests feed input without a PTY). It returns true only for a
// trimmed, case-insensitive `y`/`yes`; everything else — including bare Enter
// (the [y/N] default is No) and a read error / EOF — returns false. No
// subprocess is spawned and the input never reaches a shell (Constitution I).
func confirmRemoval(cmd *cobra.Command, stderr io.Writer, repo *repos.Repo) bool {
	fmt.Fprintf(stderr, "remove: %s  (%s)\n", repo.Name, repo.URL)
	fmt.Fprint(stderr, "Proceed? [y/N] ")
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
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
