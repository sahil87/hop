package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/sahil87/hop/internal/config"
)

// Shim-plan protocol keywords. The shell shim (see shell_init.go) branches on
// exactly these three tokens — they are the entire vocabulary of the
// binary↔shim contract. The list lives here (and only here on the binary side)
// so the shim hard-codes ZERO subcommand names: name-drift is structurally
// impossible (intake assumptions #2, #3).
const (
	planCD           = "CD"
	planRunInParent  = "RUN_IN_PARENT"
	planPassthrough  = "PASSTHROUGH"
	shimPlanFlag     = "--shim-plan"
	allSelectionFlag = "--all"
)

// batchVerbs are the reoriented action tokens (intake §5). They run inside the
// binary (group/--all fan-out, per-repo summaries, exit-code policy) so the
// shim sees PASSTHROUGH for them.
var batchVerbs = map[string]bool{"pull": true, "push": true, "sync": true}

// extractShimPlan reports whether args (typically os.Args, args[0]=binary name)
// requests the hidden --shim-plan classification. When found, rest is the
// user's original argv (everything after --shim-plan) — i.e. the words the user
// actually typed at the shim, e.g. ["webapp", "git", "pull"].
//
// --shim-plan is a hidden internal flag (mirrors help-dump): the shell shim
// emits `command hop --shim-plan "$@"` and interprets the printed plan. It is
// handled before cobra parses argv because the action token after the selection
// (e.g. `git pull`, `code .`) is an arbitrary child command line, not a hop
// flag — cobra must never try to parse it.
func extractShimPlan(args []string) (rest []string, ok bool) {
	for i := 1; i < len(args); i++ {
		if args[i] == shimPlanFlag {
			return args[i+1:], true
		}
	}
	return nil, false
}

// runShimPlan classifies the user's argv and writes one plan line-group to out,
// returning the process exit code. It NEVER executes the user's action — it
// only resolves a path (for CD / RUN_IN_PARENT) and emits the fixed vocabulary.
// The shim runs the already-parsed `"$@"` itself, so there is no shell-injection
// surface (Constitution I).
//
// Classification (first match wins):
//
//	no args                       → CD   (bare picker resolves a path)
//	$1 is __complete*             → PASSTHROUGH (defense-in-depth; the shim
//	                                 forwards __complete* directly)
//	$1 is a known cobra subcommand → PASSTHROUGH
//	$1 is a flag (other than --all) → PASSTHROUGH
//	$1 == --all                   → plural selection (action = $2..)
//	otherwise                     → selection = $1; action = $2..
//
// Plural selection (--all or an exact group name) permits only the batch verbs
// (pull/push/sync); any other action — including `where` and a bare plural with
// no action — is a usage error (exit 2). Singular selection: no action / `cd`
// → CD; `where`/`open`/batch verb → PASSTHROUGH; any other action token →
// RUN_IN_PARENT.
func runShimPlan(out, errOut io.Writer, args []string) int {
	// No args: bare `hop` → cd into the fzf-picked repo.
	if len(args) == 0 {
		return emitCD(out, errOut, "")
	}

	first := args[0]

	// __complete* and any flag other than --all are binary-direct concerns.
	if isCompletionToken(first) {
		fmt.Fprintln(out, planPassthrough)
		return 0
	}
	if first == allSelectionFlag {
		return classifyPlural(out, errOut, "", true, args[1:])
	}
	if len(first) > 0 && first[0] == '-' {
		fmt.Fprintln(out, planPassthrough)
		return 0
	}
	if isKnownSubcommand(first) {
		fmt.Fprintln(out, planPassthrough)
		return 0
	}

	// $1 is a selection. An exact configured group name is a plural selection;
	// otherwise it is a singular repo/worktree selection.
	if isConfiguredGroupName(first) {
		return classifyPlural(out, errOut, first, false, args[1:])
	}
	return classifySingular(out, errOut, first, args[1:])
}

// classifySingular handles a repo/worktree selection ($1) with the remaining
// action tokens. Bare or `cd` → CD; `where`/`open`/batch verb → PASSTHROUGH
// (the binary owns those); any other token → RUN_IN_PARENT.
func classifySingular(out, errOut io.Writer, selection string, action []string) int {
	if len(action) == 0 {
		return emitCD(out, errOut, selection)
	}
	switch verb := action[0]; {
	case verb == "cd":
		return emitCD(out, errOut, selection)
	case verb == "where" || verb == "open" || batchVerbs[verb]:
		fmt.Fprintln(out, planPassthrough)
		return 0
	default:
		return emitRunInParent(out, errOut, selection)
	}
}

// classifyPlural handles a plural selection (--all when all is true, else the
// exact group name in selection). Only the batch verbs (pull/push/sync) are
// meaningful across N repos — anything else (cd/open/where/tool, or no action
// at all) is a usage error. Batch verbs are run by the binary (PASSTHROUGH);
// the reoriented `hop <selection> <verb>` argv is what the runner parses.
// `where` resolves a single path, so it is refused on a plural selection too
// (the direct-binary runRoot path enforces the same rule).
func classifyPlural(out, errOut io.Writer, selection string, all bool, action []string) int {
	label := selection
	if all {
		label = allSelectionFlag
	}
	if len(action) == 0 {
		fmt.Fprintf(errOut, "hop: '%s' is a plural selection — it needs a batch action (pull, push, sync). A plural selection has no single directory to cd into.\n", label)
		return 2
	}
	verb := action[0]
	if batchVerbs[verb] {
		fmt.Fprintln(out, planPassthrough)
		return 0
	}
	fmt.Fprintf(errOut, "hop: '%s %s' refused — '%s' is not a batch action. Plural selections accept only pull, push, or sync (running an interactive action across many repos is not supported).\n", label, verb, verb)
	return 2
}

// emitCD resolves selection to a single repo/worktree path and prints
// CD\n<path>. Resolution failures (fzf missing/cancelled, no match) surface to
// errOut with the matching exit code so the shim's `|| return $?` propagates.
func emitCD(out, errOut io.Writer, selection string) int {
	repo, err := resolveByName(selection)
	if err != nil {
		return shimResolveErr(errOut, err)
	}
	fmt.Fprintf(out, "%s\n%s\n", planCD, repo.Path)
	return 0
}

// emitRunInParent resolves selection to a single repo/worktree path and prints
// RUN_IN_PARENT\n<path>. The shim cd's there and runs the user's literal action
// words; the binary does NOT exec the action.
func emitRunInParent(out, errOut io.Writer, selection string) int {
	repo, err := resolveByName(selection)
	if err != nil {
		return shimResolveErr(errOut, err)
	}
	fmt.Fprintf(out, "%s\n%s\n", planRunInParent, repo.Path)
	return 0
}

// shimResolveErr maps a resolveByName error to a stderr message + exit code,
// mirroring translateExit's policy so the shim path behaves like every other
// binary entry point.
func shimResolveErr(errOut io.Writer, err error) int {
	if errors.Is(err, errFzfMissing) {
		fmt.Fprintln(errOut, fzfMissingHint)
		return 1
	}
	if errors.Is(err, errNoTTY) {
		fmt.Fprintln(errOut, noTTYHint)
		return 3
	}
	if errors.Is(err, errFzfCancelled) {
		return 130
	}
	var withCode *errExitCode
	if errors.As(err, &withCode) {
		if withCode.msg != "" {
			fmt.Fprintln(errOut, withCode.msg)
		}
		return withCode.code
	}
	fmt.Fprintln(errOut, err)
	return 1
}

// isCompletionToken reports whether tok is one of cobra's __complete* internal
// entrypoints (__complete, __completeNoDesc).
func isCompletionToken(tok string) bool {
	return len(tok) >= len("__complete") && tok[:len("__complete")] == "__complete"
}

// isKnownSubcommand reports whether name is a registered cobra subcommand on the
// root command (the single source of truth — the shim hard-codes none of these).
// `help` and `completion` are cobra built-ins and also count as subcommands.
//
// It reuses the root command main() already built (rootForCompletion) to avoid
// rebuilding the whole cobra tree on the shim-plan hot path; only tests and
// other non-main entrypoints (where rootForCompletion is nil) fall back to
// newRootCmd().
func isKnownSubcommand(name string) bool {
	root := rootForCompletion
	if root == nil {
		root = newRootCmd()
	}
	for _, c := range root.Commands() {
		if c.Name() == name {
			return true
		}
		for _, alias := range c.Aliases {
			if alias == name {
				return true
			}
		}
	}
	// Cobra's auto-added help/completion commands are registered lazily; treat
	// them as subcommands so `hop help` / `hop completion ...` pass through.
	return name == "help" || name == "completion"
}

// isConfiguredGroupName reports whether name is an EXACT configured group name
// in hop.yaml (case-sensitive — mirrors resolveTargets rule 2). Load failures
// are treated as "not a group" so classification falls through to singular
// selection without erroring during dispatch.
func isConfiguredGroupName(name string) bool {
	path, err := config.Resolve()
	if err != nil {
		return false
	}
	cfg, err := config.Load(path)
	if err != nil {
		return false
	}
	return hasConfiguredGroup(cfg, name)
}
