package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/repos"
)

// runBatchVerb is the selection-first entry point for the reoriented batch
// action tokens (intake §5): `hop <selection> pull|push|sync`,
// `hop <group> <verb>`, and `hop --all <verb>`.
//
// It resolves the selection via resolveTargets (repo substring / exact group /
// --all — the same resolver `hop pull` used before reorientation) and dispatches
// to the existing single/batch runners in pull.go, push.go, and sync.go. The
// per-repo status lines, batch summary, and exit-code policy are preserved
// unchanged — only the entry point moved from a cobra subcommand to an action
// token after a selection.
//
// verb is one of "pull", "push", "sync". all is true only for the --all plural
// selection; otherwise selection is a repo/worktree substring or an exact group
// name. The -m/--message override that `hop sync` exposed as a flag is dropped
// in the reoriented form — sync uses the fixed default commit message
// (defaultSyncCommitMessage); pass a custom message via `hop <name> git commit`
// before `hop <name> push`.
func runBatchVerb(cmd *cobra.Command, verb, selection string, all bool) error {
	query := selection
	if all {
		query = ""
	}

	targets, mode, err := resolveTargets(query, all)
	if err != nil {
		if errors.Is(err, errFzfMissing) {
			fmt.Fprintln(cmd.ErrOrStderr(), fzfMissingHint)
			return errSilent
		}
		return err
	}

	switch verb {
	case "pull":
		if mode == modeSingle {
			return pullSingle(cmd, targets[0])
		}
		return pullBatch(cmd, targets)
	case "push":
		if mode == modeSingle {
			return pushSingle(cmd, targets[0])
		}
		return pushBatch(cmd, targets)
	case "sync":
		op := func(cmd *cobra.Command, r repos.Repo) (ok, gitMissing bool, err error) {
			return syncOne(cmd, r, defaultSyncCommitMessage)
		}
		if mode == modeSingle {
			return syncSingle(cmd, targets[0], op)
		}
		return syncBatch(cmd, targets, op)
	default:
		// Unreachable: callers pass only pull/push/sync.
		return &errExitCode{code: 2, msg: fmt.Sprintf("hop: unknown batch verb %q", verb)}
	}
}
