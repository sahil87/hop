package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const rootLong = `hop — locate, open, and operate on repos from hop.yaml.

Grammar: hop <selection> <action>
  <selection> = a repo name (substring → fzf on ambiguity), a repo/worktree, a
                group name, or --all (every cloned repo).
  <action>    = a builtin verb (cd, open, where), a batch verb (pull, push,
                sync), any PATH binary (git pull, code .), or a shell
                alias/function (p). Omit the action to cd into the selection.

Getting started:
  1. Run ` + "`hop config init`" + ` to create a starter hop.yaml.
  2. Edit it to list your repos (each entry: name + git URL + parent dir).
  3. For interactive use, install the shim: eval "$(hop shell-init zsh)"

Cheat sheet:
  hop                       fzf picker, print selection
  hop <name>                cd into the repo (shell function — needs ` + "`eval \"$(hop shell-init zsh)\"`" + `)
  hop <name>/<wt>           same, but rooted at <wt> (a worktree of <name> per ` + "`wt list --json`" + `)
  hop <name> cd             same — explicit verb form
  hop <name> where          echo abs path of matching repo (or worktree, with /<wt> suffix)
  hop <name> open           open the repo in an app (delegates to wt's menu; "Open here" cds the parent shell)
  hop <name> git pull       run ` + "`git pull`" + ` with cwd = <name>'s repo dir (any PATH binary works)
  hop <name> code .         open the editor in <name>'s repo dir (tool-form — runs in the parent shell)
  hop <name> p              run the shell alias/function ` + "`p`" + ` in <name>'s repo dir
  hop <name> pull           run ` + "`git pull`" + ` in the named repo (batch verb, selection-first)
  hop <name> push           run ` + "`git push`" + ` in the named repo
  hop <name> sync           auto-commit dirty tree, then ` + "`git pull --rebase`" + ` + ` + "`git push`" + `
  hop <group> pull          run ` + "`git pull`" + ` in every cloned repo of <group>
  hop --all pull            run ` + "`git pull`" + ` in every cloned repo
  hop --all sync            run sync in every cloned repo
  hop clone <name>          git clone the repo if it isn't already on disk
  hop clone <url>           ad-hoc clone: clone the URL, register it in hop.yaml, print landed path
  hop clone --all           clone every repo from hop.yaml that isn't already on disk
  hop clone                 fzf picker, then clone if missing
  hop ls                    list all repos
  hop ls --trees            list all repos with worktree summaries (fans out ` + "`wt list --json`" + `)
  hop add <dir>             register an existing on-disk repo into hop.yaml
  hop rm [<name>]           remove a repo from hop.yaml (fzf picker if no name)
  hop shell-init <shell>    emit shell integration (zsh or bash). Use: eval "$(hop shell-init zsh)"
  hop config init           bootstrap a starter hop.yaml
  hop config where          print the resolved hop.yaml path
  hop config print          print the resolved hop.yaml contents to stdout
  hop config scan <dir>     scan a directory for git repos and populate hop.yaml
  hop update                self-update the hop binary via Homebrew
  hop -h | --help           show this help
  hop -v | --version        print version

Notes:
  - ` + "`hop <name>`" + ` and ` + "`hop <name> cd`" + ` require shell integration (a binary can't change
    its parent shell's cwd). Without it, use:  cd "$(hop <name> where)"
  - Tool-form (` + "`hop <name> <tool> ...`" + `) and ` + "`hop <name> open`" + `'s "Open here" choice run in
    the parent shell via the shim. Scripts/CI that bypass the shim must use
    ` + "`hop <name> where`" + ` for path resolution and run tools themselves.
  - ` + "`pull`" + `, ` + "`push`" + `, ` + "`sync`" + ` accept a repo, a worktree, a group, or ` + "`--all`" + ` as the
    selection. ` + "`sync`" + ` is ` + "`pull --rebase`" + ` + ` + "`push`" + ` (linear history, no auto-resolve on
    conflict), auto-committing a dirty tree first.
  - A plural selection (` + "`--all`" + ` or a group) accepts only ` + "`pull`/`push`/`sync`" + ` —
    cd, open, where, and arbitrary tools across many repos are refused.
  - On ambiguous or no-match queries, fzf opens prefilled with your query.
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
		return &errExitCode{code: 2, msg: bareNameHint}
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
		return &errExitCode{code: 2, msg: cdHint}
	case "open":
		if len(args) > 2 {
			return &errExitCode{code: 2, msg: fmt.Sprintf("hop: '%s open' takes no extra arguments", selection)}
		}
		return runOpen(cmd, selection)
	case "pull", "push", "sync":
		return runBatchVerb(cmd, action, selection, false)
	default:
		return &errExitCode{code: 2, msg: fmt.Sprintf(toolFormHintFmt, action)}
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
