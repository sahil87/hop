package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// Per-worktree glyphs used in `hop ls --trees` rows. Single-rune so width
// stays predictable across terminals.
const (
	wtDirtyGlyph    = "*"
	wtUnpushedGlyph = "↑"
)

func newLsCmd() *cobra.Command {
	var trees bool
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list all repos as aligned name/path columns",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := loadRepos()
			if err != nil {
				return err
			}
			// JSON mode: --json composes with --trees. An empty list emits `[]`
			// (valid JSON) rather than nothing — JSON consumers expect a value.
			if jsonOut {
				if trees {
					return runLsTreesJSON(cmd, rs)
				}
				return runLsJSON(cmd.OutOrStdout(), rs)
			}
			if len(rs) == 0 {
				return nil
			}
			padWidth := longestName(rs) + 2
			if trees {
				return runLsTrees(cmd, rs, padWidth)
			}
			return runLsPlain(cmd.OutOrStdout(), rs, padWidth)
		},
	}
	cmd.Flags().BoolVar(&trees, "trees", false, "list worktrees per repo via `wt list --json`")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable JSON (composes with --trees)")
	return cmd
}

// longestName returns the byte length of the longest Name in rs. Used for
// the left-aligned name column shared between `hop ls` and `hop ls --trees`.
func longestName(rs repos.Repos) int {
	max := 0
	for _, r := range rs {
		if n := len(r.Name); n > max {
			max = n
		}
	}
	return max
}

// padName left-aligns name to width totalWidth (= longestName + 2). The two
// trailing spaces preserve the existing `hop ls` separator convention.
func padName(name string, totalWidth int) string {
	if pad := totalWidth - len(name); pad > 0 {
		return name + strings.Repeat(" ", pad)
	}
	return name + "  "
}

func runLsPlain(out io.Writer, rs repos.Repos, padWidth int) error {
	for _, r := range rs {
		fmt.Fprintln(out, padName(r.Name, padWidth)+r.Path)
	}
	return nil
}

// runLsTrees fans `wt list --json` across each cloned repo in source order
// and emits a per-row summary. Non-cloned repos surface `(not cloned)`
// without invoking wt. Per-repo wt-list failures degrade gracefully as
// inline `(wt list failed: <err>)` rows — the table is never aborted by a
// single corrupt `.git`.
//
// The exception is the missing-`wt` case: if the FIRST `wt list` invocation
// returns proc.ErrNotFound, we fail fast with the standard `wtMissingHint`
// (matches `hop <name> open`'s wording) and exit 1. Subsequent invocations
// within the same run can't hit ErrNotFound — we abort on the first.
func runLsTrees(cmd *cobra.Command, rs repos.Repos, padWidth int) error {
	out := cmd.OutOrStdout()
	for _, r := range rs {
		state, err := cloneState(r.Path)
		if err != nil {
			return err
		}
		if state != stateAlreadyCloned {
			fmt.Fprintln(out, padName(r.Name, padWidth)+"(not cloned)")
			continue
		}
		entries, err := listWorktrees(context.Background(), r.Path)
		if err != nil {
			if errors.Is(err, proc.ErrNotFound) {
				fmt.Fprintln(cmd.ErrOrStderr(), wtMissingHint)
				return errSilent
			}
			fmt.Fprintln(out, padName(r.Name, padWidth)+fmt.Sprintf("(wt list failed: %v)", err))
			continue
		}
		fmt.Fprintln(out, padName(r.Name, padWidth)+formatTreesRow(entries))
	}
	return nil
}

// --- JSON output (intake 1x1u Item 2) -------------------------------------
//
// Field naming mirrors the sibling `wt list --json` convention (Constitution
// IV — wrap, don't reinvent; toolchain consistency so agents that already parse
// wt output reuse the shape). A purpose-built output struct is used rather than
// marshalling repos.Repo / WtEntry directly: repos.Repo has no JSON tags and
// exposes Dir; WtEntry uses value Dirty/Unpushed, but wt's contract for
// per-worktree status is pointer-field + omitempty (so "not computed" is
// distinguishable from zero). lsWorktreeJSON mirrors that pointer contract.

// lsRepoJSON is one repo object in `hop ls --json` output. In --trees mode the
// Worktrees / Cloned / Error fields describe the repo's on-disk worktree state;
// in default mode they are omitted (omitempty) so the object is just the
// {name, path, url, group} quad.
type lsRepoJSON struct {
	Name      string           `json:"name"`
	Path      string           `json:"path"`
	URL       string           `json:"url"`
	Group     string           `json:"group"`
	Cloned    *bool            `json:"cloned,omitempty"`
	Worktrees []lsWorktreeJSON `json:"worktrees,omitempty"`
	Error     string           `json:"error,omitempty"`
}

// lsWorktreeJSON is one entry in a repo's nested `worktrees` array. Dirty and
// Unpushed are pointers with omitempty so "status not computed" (nil → key
// omitted) is distinguishable from a computed clean/zero value — mirroring wt's
// list --json schema.
type lsWorktreeJSON struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Dirty    *bool  `json:"dirty,omitempty"`
	Unpushed *int   `json:"unpushed,omitempty"`
}

// writeJSON marshals v as 2-space-indented JSON with a trailing newline,
// matching wt's `json.MarshalIndent(entries, "", "  ")` emission style.
func writeJSON(out io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("hop ls: marshal json: %w", err)
	}
	_, err = fmt.Fprintln(out, string(b))
	return err
}

// runLsJSON emits the default `hop ls --json` array of {name, path, url, group}
// objects in YAML source order. An empty list emits `[]`.
func runLsJSON(out io.Writer, rs repos.Repos) error {
	objs := make([]lsRepoJSON, 0, len(rs))
	for _, r := range rs {
		objs = append(objs, lsRepoJSON{Name: r.Name, Path: r.Path, URL: r.URL, Group: r.Group})
	}
	return writeJSON(out, objs)
}

// runLsTreesJSON emits `hop ls --json --trees`: each repo object nests a
// `worktrees` array. The cloned-state / failure representations mirror the
// text-mode behaviors in runLsTrees:
//   - non-cloned repo → cloned:false, worktrees omitted.
//   - per-repo wt-list failure → an `error` string field; the array is never
//     aborted (matches text mode's never-abort contract).
//   - the FIRST wt-list invocation hitting proc.ErrNotFound (wt missing) keeps
//     the fail-fast: wtMissingHint to stderr, exit 1, NO JSON emitted.
//
// Ordering preserves YAML source order.
func runLsTreesJSON(cmd *cobra.Command, rs repos.Repos) error {
	objs := make([]lsRepoJSON, 0, len(rs))
	for _, r := range rs {
		obj := lsRepoJSON{Name: r.Name, Path: r.Path, URL: r.URL, Group: r.Group}

		state, err := cloneState(r.Path)
		if err != nil {
			return err
		}
		if state != stateAlreadyCloned {
			notCloned := false
			obj.Cloned = &notCloned
			objs = append(objs, obj)
			continue
		}
		cloned := true
		obj.Cloned = &cloned

		entries, err := listWorktrees(context.Background(), r.Path)
		if err != nil {
			if errors.Is(err, proc.ErrNotFound) {
				fmt.Fprintln(cmd.ErrOrStderr(), wtMissingHint)
				return errSilent
			}
			obj.Error = fmt.Sprintf("wt list failed: %v", err)
			objs = append(objs, obj)
			continue
		}
		obj.Worktrees = worktreesJSON(entries)
		objs = append(objs, obj)
	}
	return writeJSON(cmd.OutOrStdout(), objs)
}

// worktreesJSON maps wt entries to the JSON worktree shape. Dirty/Unpushed are
// always populated (non-nil) here because hop's listWorktrees calls
// `wt list --json` without --status, so wt omits those fields and hop's
// value-typed WtEntry defaults them to false/0; the pointer+omitempty schema is
// the forward-compatible mirror of wt's contract (present-with-zero today, ready
// to omit if hop ever surfaces uncomputed status).
func worktreesJSON(entries []WtEntry) []lsWorktreeJSON {
	out := make([]lsWorktreeJSON, len(entries))
	for i, e := range entries {
		dirty := e.Dirty
		unpushed := e.Unpushed
		out[i] = lsWorktreeJSON{
			Name:     e.Name,
			Path:     e.Path,
			Dirty:    &dirty,
			Unpushed: &unpushed,
		}
	}
	return out
}

// formatTreesRow renders the per-repo summary `<N> tree(s)  (<wt-list>)`.
// Each wt is `name[*][↑N]`: `*` if dirty, `↑N` if Unpushed > 0.
func formatTreesRow(entries []WtEntry) string {
	noun := "trees"
	if len(entries) == 1 {
		noun = "tree"
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		flags := ""
		if e.Dirty {
			flags += wtDirtyGlyph
		}
		if e.Unpushed > 0 {
			flags += fmt.Sprintf("%s%d", wtUnpushedGlyph, e.Unpushed)
		}
		parts[i] = e.Name + flags
	}
	return fmt.Sprintf("%d %s  (%s)", len(entries), noun, strings.Join(parts, ", "))
}
