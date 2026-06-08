package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// push.go wraps `git push`. The selection-first entry point lives in
// batch_verb.go (`hop <selection> push`); the runners below are shared by the
// single and batch paths.

// pushSingle handles single-repo mode (rule 3 substring match → one Repo).
// Skip-not-cloned and push failures both exit 1; success is exit 0.
func pushSingle(cmd *cobra.Command, r repos.Repo) error {
	state, err := cloneState(r.Path)
	if err != nil {
		return err
	}
	if state != stateAlreadyCloned {
		fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s not cloned\n", r.Name)
		return errSilent
	}
	ok, gitMissing, _ := pushOne(cmd, r)
	if gitMissing {
		fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
		return errSilent
	}
	if !ok {
		return errSilent
	}
	return nil
}

// pushBatch iterates targets sequentially via runBatch, counting outcomes and
// emitting `summary: pushed=N skipped=N failed=N` on stderr. Returns errSilent
// when any push failed. On `git` missing, runBatch aborts immediately (no
// further repos attempted, no summary line emitted) — same behavior as pull.
func pushBatch(cmd *cobra.Command, targets repos.Repos) error {
	return runBatch(cmd, targets, "push", "pushed", pushOne)
}

// pushOne runs `git push` in r.Path via proc.RunCapture with a 10-minute
// timeout. The returned tuple is:
//   - ok: true on a successful push
//   - gitMissing: true when git is not on PATH (caller emits the hint and
//     aborts the batch)
//   - err: the underlying error (informational; pushOne already wrote a status
//     line to stderr)
//
// pushOne writes its own per-repo status line ("push: <name> ✓ ..." or
// "push: <name> ✗ ...") to cmd's stderr. Git's own stderr is forwarded
// verbatim by proc.RunCapture (which sets cmd.Stderr = os.Stderr).
func pushOne(cmd *cobra.Command, r repos.Repo) (ok, gitMissing bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()
	out, err := proc.RunCapture(ctx, r.Path, "git", "push")
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			return false, true, err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "push: %s ✗ %v\n", r.Name, err)
		return false, false, err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "push: %s ✓ %s\n", r.Name, lastNonEmptyLine(string(out)))
	return true, false, nil
}
