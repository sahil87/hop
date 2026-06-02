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

// addCmdName is the CLI prefix used in stderr messages for `hop config add`
// (matches scan's "hop config scan: ..." wording convention).
const addCmdName = "hop config add"

// addLong is the cobra Long help for `hop config add <dir>`.
const addLong = `Register a single on-disk repo into hop.yaml.

The non-recursive, single-directory sibling of 'hop config scan': it classifies
just <dir> and, when it is a normal git repo with a remote, merges its URL into
hop.yaml using the same group convention scan uses (convention layout → the
'default' group; otherwise an invented group keyed off the parent dir basename).

Unlike scan, add writes by default — you named a specific directory.

A non-git directory is a no-op (a clear message, exit 0), not an error.

Examples:
  hop config add ~/code/acme/widget   register one existing repo into hop.yaml`

// newConfigAddCmd returns the cobra factory for `hop config add <dir>`.
func newConfigAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <dir>",
		Short: "register a single on-disk repo into hop.yaml",
		Long:  addLong,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigAdd(cmd, args[0])
		},
	}
	return cmd
}

// runConfigAdd validates <dir>, resolves hop.yaml, classifies the single dir,
// and (for a normal repo) merges its URL into hop.yaml via buildScanPlan +
// MergeScan. Returns errSilent / *errExitCode on user-visible failures; nil on
// success and on every forgiving no-op (non-git dir, worktree/bare skip,
// already-registered).
func runConfigAdd(cmd *cobra.Command, userArg string) error {
	stderr := cmd.ErrOrStderr()

	// 1. Validate <dir>: filepath.Clean → EvalSymlinks → os.Stat (directory).
	canonicalDir, ok := validateConfigDir(userArg, addCmdName, stderr)
	if !ok {
		return &errExitCode{code: 2}
	}

	// 2. Resolve hop.yaml (precondition mirrors scan's two-line message).
	configPath, err := config.Resolve()
	if err != nil {
		bootstrap, werr := config.ResolveWriteTarget()
		if werr != nil {
			bootstrap = "$XDG_CONFIG_HOME/hop/hop.yaml"
		}
		fmt.Fprintf(stderr, "%s: no hop.yaml found at %s.\nRun 'hop config init' first, then re-run add.\n", addCmdName, bootstrap)
		return errSilent
	}

	// 3. Load existing config (used for the convention check + dedup).
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", addCmdName, err)
		return errSilent
	}

	// 4. Classify the single dir (reuses scan's classify + remote inspection).
	found, skipReason, isRepo, err := scan.ClassifyOne(context.Background(), canonicalDir, scan.Options{GitRunner: gitRunner})
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(stderr, gitMissingHint)
			return errSilent
		}
		fmt.Fprintf(stderr, "%s: %v\n", addCmdName, err)
		return errSilent
	}
	if !isRepo {
		// Forgiving: plain dir (no skip reason) or a worktree/bare/no-remote
		// candidate. Message + exit 0 (Assumptions 10/11).
		fmt.Fprintln(stderr, addSkipMessage(userArg, skipReason))
		return nil
	}

	// 5. Build a one-element scan plan (convention → default; else invented).
	plan, _ := buildScanPlan(cfg, []scan.Found{found}, stderr)

	// 6. Idempotency: a URL already registered anywhere produces an empty plan
	//    (buildScanPlan drops the dup). No write; report + exit 0.
	if planIsEmpty(plan) {
		fmt.Fprintf(stderr, "%s: %s already registered in %s. Nothing to add.\n", addCmdName, found.URL, configPath)
		return nil
	}

	// 7. Write by default (Assumption 7).
	if err := yamled.MergeScan(configPath, plan); err != nil {
		fmt.Fprintf(stderr, "%s: write %s: %v\n", addCmdName, configPath, err)
		return errSilent
	}
	fmt.Fprintf(stderr, "added: %s\nwrote: %s\n", found.URL, configPath)
	return nil
}

// addSkipMessage returns the forgiving stderr line for a dir that is not a
// registrable normal repo. A plain directory (empty skipReason) gets the
// explicit "not a git repo" wording; classified candidates (worktree, bare
// repo, no remote) report their reason.
func addSkipMessage(userArg, skipReason string) string {
	if skipReason == "" {
		return fmt.Sprintf("%s: '%s' is not a git repo. Nothing to add.", addCmdName, userArg)
	}
	return fmt.Sprintf("%s: '%s' is a %s — skipping. Nothing to add.", addCmdName, userArg, skipReason)
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
