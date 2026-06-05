package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/scan"
	"github.com/sahil87/hop/internal/yamled"
)

// addLong is the cobra Long help for `hop add <dir>` and its hidden alias
// `hop config add <dir>`.
const addLong = `Register a single on-disk repo into hop.yaml.

The non-recursive, single-directory sibling of 'hop config scan': it classifies
just <dir> and, when it is a normal git repo with a remote, merges its URL into
hop.yaml using the same group convention scan uses (convention layout → the
'default' group; otherwise an invented group keyed off the parent dir basename).

Unlike scan, add writes by default — you named a specific directory.

A non-git directory is a no-op (a clear message, exit 0), not an error.

Examples:
  hop add ~/code/acme/widget   register one existing repo into hop.yaml`

// newAddCmd returns the cobra factory for the canonical top-level `hop add <dir>`.
func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <dir>",
		Short: "register a single on-disk repo into hop.yaml",
		Long:  addLong,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, "hop add", args[0])
		},
	}
}

// newConfigAddCmd returns the cobra factory for the hidden alias
// `hop config add <dir>`. It shares runAdd with the canonical top-level command
// but is Hidden (disappears from --help and self-filters from help-dump) and
// keeps emitting its historical "hop config add:" stderr prefix.
func newConfigAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "add <dir>",
		Short:  "register a single on-disk repo into hop.yaml",
		Long:   addLong,
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, "hop config add", args[0])
		},
	}
}

// runAdd validates <dir>, resolves hop.yaml, classifies the single dir, and
// (for a normal repo) merges its URL into hop.yaml via buildScanPlan +
// MergeScan. cmdName is the per-path stderr prefix ("hop add" for the canonical
// top-level command, "hop config add" for the hidden alias). Returns errSilent
// / *errExitCode on user-visible failures; nil on success and on every
// forgiving no-op (non-git dir, worktree/bare skip, already-registered).
func runAdd(cmd *cobra.Command, cmdName, userArg string) error {
	stderr := cmd.ErrOrStderr()

	// 1. Validate <dir>: filepath.Clean → EvalSymlinks → os.Stat (directory).
	canonicalDir, ok := validateConfigDir(userArg, cmdName, stderr)
	if !ok {
		return &errExitCode{code: 2}
	}

	// 2. Resolve the write target and auto-init the config when absent. add is a
	//    write-command: it carries the user's intent (a specific dir to register),
	//    so a missing config is bootstrapped with a minimal skeleton rather than
	//    erroring. The only ResolveWriteTarget error is $HOME-unset — an
	//    environment failure, surfaced as before.
	configPath, err := config.ResolveWriteTarget()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return errSilent
	}
	created, err := config.EnsureSkeleton(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return errSilent
	}
	if created {
		fmt.Fprintf(stderr, "created: %s\n", configPath)
	}

	// 3. Load existing config (used for the convention check + dedup).
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return errSilent
	}

	// 4. Classify the single dir (reuses scan's classify + remote inspection).
	found, skipReason, isRepo, err := scan.ClassifyOne(context.Background(), canonicalDir, scan.Options{GitRunner: gitRunner})
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(stderr, gitMissingHint)
			return errSilent
		}
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return errSilent
	}
	if !isRepo {
		// Forgiving: plain dir (no skip reason) or a worktree/bare/no-remote
		// candidate. Message + exit 0 (Assumptions 10/11).
		fmt.Fprintln(stderr, addSkipMessage(cmdName, userArg, skipReason))
		return nil
	}

	// 5. Build a one-element scan plan (convention → default; else invented).
	plan, _ := buildScanPlan(cfg, []scan.Found{found}, stderr)

	// 6. Idempotency: a URL already registered anywhere produces an empty plan
	//    (buildScanPlan drops the dup). No write; report + exit 0.
	if planIsEmpty(plan) {
		fmt.Fprintf(stderr, "%s: %s already registered in %s. Nothing to add.\n", cmdName, found.URL, configPath)
		return nil
	}

	// 7. Write by default (Assumption 7).
	if err := yamled.MergeScan(configPath, plan); err != nil {
		fmt.Fprintf(stderr, "%s: write %s: %v\n", cmdName, configPath, err)
		return errSilent
	}
	fmt.Fprintf(stderr, "added: %s\nwrote: %s\n", found.URL, configPath)
	return nil
}

// addSkipMessage returns the forgiving stderr line for a dir that is not a
// registrable normal repo. cmdName is the per-path prefix. A plain directory
// (empty skipReason) gets the explicit "not a git repo" wording; classified
// candidates (worktree, bare repo, no remote) report their reason.
func addSkipMessage(cmdName, userArg, skipReason string) string {
	if skipReason == "" {
		return fmt.Sprintf("%s: '%s' is not a git repo. Nothing to add.", cmdName, userArg)
	}
	return fmt.Sprintf("%s: '%s' is a %s — skipping. Nothing to add.", cmdName, userArg, skipReason)
}

// planIsEmpty reports whether a ScanPlan would add no URLs (every candidate was
// deduped away). Used for the idempotent "already registered" path.
func planIsEmpty(plan yamled.ScanPlan) bool {
	if len(plan.DefaultURLs) > 0 {
		return false
	}
	for _, ig := range plan.InventedGroups {
		if len(ig.URLs) > 0 {
			return false
		}
	}
	return true
}
