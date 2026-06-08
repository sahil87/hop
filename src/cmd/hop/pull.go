package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// pull.go wraps `git pull`. The selection-first entry point lives in
// batch_verb.go (`hop <selection> pull`); the runners below are shared by the
// single and batch paths.

// pullSingle handles single-repo mode (rule 3 substring match → one Repo).
// Skip-not-cloned and pull failures both exit 1; success is exit 0.
func pullSingle(cmd *cobra.Command, r repos.Repo) error {
	state, err := cloneState(r.Path)
	if err != nil {
		return err
	}
	if state != stateAlreadyCloned {
		fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s not cloned\n", r.Name)
		return errSilent
	}
	ok, gitMissing, _ := pullOne(cmd, r)
	if gitMissing {
		fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
		return errSilent
	}
	if !ok {
		return errSilent
	}
	return nil
}

// pullBatch iterates targets sequentially via runBatch, counting outcomes and
// emitting `summary: pulled=N skipped=N failed=N` on stderr. Returns errSilent
// when any pull failed. On `git` missing, runBatch aborts immediately (no
// further repos attempted, no summary line emitted) per spec assumption #17.
func pullBatch(cmd *cobra.Command, targets repos.Repos) error {
	return runBatch(cmd, targets, "pull", "pulled", pullOne)
}

// pullOne runs `git pull` in r.Path via proc.RunCapture with a 10-minute
// timeout. The returned tuple is:
//   - ok: true on a successful pull
//   - gitMissing: true when git is not on PATH (caller emits the hint and
//     aborts the batch)
//   - err: the underlying error (informational; pullOne already wrote a status
//     line to stderr)
//
// pullOne writes its own per-repo status line ("pull: <name> ✓ ..." or
// "pull: <name> ✗ ...") to cmd's stderr. Git's own stderr is forwarded
// verbatim by proc.RunCapture (which sets cmd.Stderr = os.Stderr).
func pullOne(cmd *cobra.Command, r repos.Repo) (ok, gitMissing bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()
	out, err := proc.RunCapture(ctx, r.Path, "git", "pull")
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			return false, true, err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "pull: %s ✗ %v\n", r.Name, err)
		return false, false, err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "pull: %s ✓ %s\n", r.Name, lastNonEmptyLine(string(out)))
	return true, false, nil
}

// lastNonEmptyLine returns the last non-empty line of s with surrounding
// whitespace trimmed, or "" if s is empty/whitespace-only. Used to summarize a
// `git pull` / `git push` invocation's stdout (e.g., "Already up to date.",
// "Fast-forward", "Everything up-to-date").
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t != "" {
			return t
		}
	}
	return ""
}
