package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const rootLong = `hop — locate, open, and operate on repos from hop.yaml.

Grammar: hop <selection> [action]
  <selection>   a repo name (substring; fzf on ambiguity), a <name>/<wt>
                worktree, a group name, or --all (every cloned repo)
  [action]      builtin verb (cd, where, open), batch verb (pull, push,
                sync), or any PATH tool / shell alias (git pull, code ., p).
                Omitted: cd into the selection.

Getting started:
  1. Wire the shell shim:  shll shell-setup
  2. Build the config:     hop add -r ~/code

Examples:
  hop                     fzf picker, print the selection
  hop <name>              cd into the repo (needs the shell shim)
  hop <name>/<wt>         same, rooted at worktree <wt>
  hop <name> where        print the repo's absolute path
  hop <name> open         open the repo in an app (wt's menu)
  hop <name> code .       run any PATH tool in the repo dir
  hop <name> sync         auto-commit, git pull --rebase, git push
  hop <group> pull        batch verb across a group's cloned repos
  hop --all sync          batch verb across every cloned repo

Management subcommands (clone, ls, add, rm, config, ...) are listed
below — see ` + "`hop <command> -h`" + ` for each.

Notes:
  - cd and tool-form run in your parent shell via the shim (shll shell-setup,
    or eval "$(hop shell-init zsh)"). Without it, use cd "$(hop <name> where)".
  - A group or --all accepts only pull/push/sync; cd, open, where, and
    arbitrary tools are single-repo.
  - Ambiguous or no-match queries open fzf prefilled with your query.
  - Config lives at ~/.config/hop/hop.yaml.`

// bareNameHint is the exact stderr line printed when the binary is invoked
// with a single positional (the bare-name `hop <name>` shorthand for cd).
// Bare-name cd is shell-only — the shim emits a CD plan; the binary errors.
const bareNameHint = `hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where`

// cdHint is the exact stderr line printed when the binary is invoked as
// `hop <name> cd` (explicit cd verb at $2). Same shape as bareNameHint —
// shell-only, error in the binary.
const cdHint = `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"`

// toolFormHintFmt is the format string for the tool-form error: when the binary
// receives `hop <name> <tool>` directly (the shim normally routes this through
// the RUN_IN_PARENT plan). Tool-form runs in the parent shell, so the binary
// can't honor it — it points the user at the shim. %s is the would-be tool name.
const toolFormHintFmt = `hop: '%s' is not a hop verb (cd, where, open, pull, push, sync). Tool-form runs in your shell — install the shim: eval "$(hop shell-init zsh)"`

// shellOnlyHintErr builds the exit-2 usage error for the shell-only forms
// (bare-name cd, the `cd` verb, tool-form). When HOP_WRAPPER=1 the shim is known
// present, so the "install the shim" hint is redundant noise — we suppress the
// hint TEXT but KEEP the exit-2 code, because the invocation is still a form the
// binary genuinely cannot fulfill on its own (cd/tool-form run in the parent
// shell). Mirrors wt's WT_WRAPPER pattern in apps.go (suppress the hint, not the
// error). An empty msg makes translateExit exit 2 without printing anything.
func shellOnlyHintErr(hint string) *errExitCode {
	if os.Getenv("HOP_WRAPPER") == "1" {
		return &errExitCode{code: 2}
	}
	return &errExitCode{code: 2, msg: hint}
}

func newRootCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:           "hop",
		Short:         "locate, open, and operate on repos from hop.yaml.",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Selection-first grammar (reached directly via the shim's PASSTHROUGH
		// plan, or by scripts bypassing the shim):
		//   0 args                       → bare picker (print path)
		//   --all <batch-verb>           → plural batch (pull/push/sync only)
		//   1 arg                        → bare-name cd (shell-only) → error
		//   2 args, $2=where             → resolve $1, print path
		//   2 args, $2=open              → wt menu (binary passthrough)
		//   2 args, $2=pull/push/sync    → batch verb over selection $1
		//   2 args, $2=cd                → cd-verb (shell-only) → error
		//   2 args, otherwise            → tool-form (shell-only) → error
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd, args, all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "select every cloned repo from hop.yaml (plural selection — use with pull/push/sync)")

	cmd.AddCommand(
		newAddCmd(),
		newRmCmd(),
		newCloneCmd(),
		newLsCmd(),
		newShellInitCmd(),
		newConfigCmd(),
		newUpdateCmd(),
		newSkillCmd(),
		newHelpDumpCmd(),
	)

	return cmd
}

// runRoot implements the selection-first grammar for the bare `hop` command.
// pull/push/sync are action tokens here, not subcommands — they dispatch into
// the batch machinery (see batch_verb.go). Plural selection is --all (flag) or
// an exact group name at $1.
func runRoot(cmd *cobra.Command, args []string, all bool) error {
	// Plural selection via --all: action is args[0..]. Only batch verbs / where
	// are meaningful; everything else (including no action) is a usage error.
	if all {
		return runPluralSelection(cmd, allSelectionFlag, args)
	}

	switch len(args) {
	case 0:
		return resolveAndPrint(cmd, "")
	case 1:
		// Could be a plural group selection with no action (error) or a
		// singular bare-name cd (shell-only error). Disambiguate on group-ness.
		if isConfiguredGroupName(args[0]) {
			return runPluralSelection(cmd, args[0], nil)
		}
		return shellOnlyHintErr(bareNameHint)
	}

	selection, action := args[0], args[1]

	// Plural selection via exact group name.
	if isConfiguredGroupName(selection) {
		return runPluralSelection(cmd, selection, args[1:])
	}

	switch action {
	case "where":
		return resolveAndPrint(cmd, selection)
	case "cd":
		return shellOnlyHintErr(cdHint)
	case "open":
		if len(args) > 2 {
			return &errExitCode{code: 2, msg: fmt.Sprintf("hop: '%s open' takes no extra arguments", selection)}
		}
		return runOpen(cmd, selection)
	case "pull", "push", "sync":
		return runBatchVerb(cmd, action, selection, false)
	default:
		return shellOnlyHintErr(fmt.Sprintf(toolFormHintFmt, action))
	}
}

// runPluralSelection dispatches a plural selection (--all when selection is
// allSelectionFlag, else the exact group name). Only pull/push/sync are
// permitted; any other action (including where) — or no action — is a usage
// error (exit 2). This
// mirrors the --shim-plan plural guard so the direct-binary path and the shim
// path enforce the same rule.
func runPluralSelection(cmd *cobra.Command, selection string, action []string) error {
	label := selection
	if len(action) == 0 {
		return &errExitCode{code: 2, msg: fmt.Sprintf("hop: '%s' is a plural selection — it needs a batch action (pull, push, sync).", label)}
	}
	verb := action[0]
	switch verb {
	case "pull", "push", "sync":
		return runBatchVerb(cmd, verb, selection, selection == allSelectionFlag)
	case "where":
		return &errExitCode{code: 2, msg: fmt.Sprintf("hop: '%s where' is not supported — 'where' resolves a single path. Use 'hop ls' to list every repo.", label)}
	default:
		return &errExitCode{code: 2, msg: fmt.Sprintf("hop: '%s %s' refused — '%s' is not a batch action. Plural selections accept only pull, push, or sync.", label, verb, verb)}
	}
}
