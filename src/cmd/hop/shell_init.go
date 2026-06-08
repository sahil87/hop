package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// posixInit is the shared portion of `hop shell-init zsh` and `hop shell-init bash`.
// Both shells understand `[[ ]]`, `local`, and `printf '%s' | sed`. The completion
// suffix (cobra-generated zsh or bash completion) is appended per-shell at the end.
//
// The shim is a LOGIC-FREE interpreter of a fixed 3-keyword protocol emitted by
// the binary's hidden `--shim-plan` flag. It hard-codes ZERO subcommand names —
// the subcommand list lives only in cobra, so shim/binary name-drift is
// structurally impossible (the root cause of hop's "stale shim" bug class).
//
// Flow in hop():
//
//  1. $1 is __complete*  → forward to the binary directly. The cobra-generated
//     completion script calls `hop __complete ...`; this must NOT go through
//     --shim-plan (which would classify the completion request, not answer it).
//  2. otherwise          → ask the binary to classify the user's argv:
//     plan="$(command hop --shim-plan "$@")" || return $?
//     and `case` on the first line over the 3 protocol keywords:
//     CD <path>            → cd -- <path>                  (`hop webapp`, `hop webapp cd`)
//     RUN_IN_PARENT <path> → cd -- <path>; shift; "$@"     (`hop webapp git pull`, `hop webapp code .`)
//     PASSTHROUGH          → _hop_passthrough "$@"         (binary owns it: add/rm/clone/ls/
//     config/update/shell-init/where/open/pull/push/sync/--help/--version/...)
//
// SECURITY (Constitution I): the shim runs the user's already-parsed words
// ("$@") — never `eval` of binary output. The binary emits only the fixed
// vocabulary plus a path used solely as a quoted `cd` operand. There is no
// re-parsing of binary stdout as code, so no shell-injection surface.
//
// The PASSTHROUGH arm provides a unified cd side-channel via WT_CD_FILE: it runs
// the binary with WT_CD_FILE pointing at a temp file, then cds there if the
// binary (or a tool it execs, e.g. `wt open`'s "Open here") wrote a path. This
// collapses the former three handoffs (where=stdout, open=WT_CD_FILE,
// clone=conditional-stdout) into ONE channel. `where` still prints to stdout for
// scripts; `open`/`clone <url>` route their cd-target through WT_CD_FILE.
const posixInit = `# hop shell integration — emit via: eval "$(hop shell-init <shell>)"
# Installs: hop function (a logic-free interpreter of the binary's --shim-plan
# protocol), h alias, completion. Hard-codes zero subcommand names.

hop() {
  case "$1" in
    __complete*)
      # Cobra-internal completion entrypoints (__complete, __completeNoDesc).
      # These must reach the binary directly — routing them through --shim-plan
      # would classify the completion request instead of answering it.
      command hop "$@"
      return $?
      ;;
  esac

  local plan
  plan="$(command hop --shim-plan "$@")" || return $?
  case "${plan%%$'\n'*}" in
    CD)
      cd -- "$(printf '%s' "$plan" | sed -n 2p)"
      ;;
    RUN_IN_PARENT)
      cd -- "$(printf '%s' "$plan" | sed -n 2p)" || return $?
      shift
      "$@"
      ;;
    PASSTHROUGH)
      _hop_passthrough "$@"
      ;;
    *)
      # Defensive: an unrecognized plan (e.g. a future protocol keyword reaching
      # an older shim) falls back to the binary so cobra prints a normal error.
      command hop "$@"
      ;;
  esac
}

# _hop_passthrough runs the binary while providing a unified cd side-channel.
# It points WT_CD_FILE at a temp file; commands that resolve a cd-target write
# the path there (today: ` + "`wt open`" + `'s "Open here", and ` + "`hop clone <url>`" + `). The
# shim cds there after the command exits if the file is non-empty. We do NOT
# capture stdout via $(...) — that would swallow interactive menus (wt) and
# block on stdin. Commands that don't write the file (where, ls, config, ...)
# leave it empty and no cd occurs.
_hop_passthrough() {
  local cdfile target rc
  cdfile="$(mktemp -t hop-cd.XXXXXX)" || { command hop "$@"; return $?; }
  WT_CD_FILE="$cdfile" command hop "$@"
  rc=$?
  target=""
  if [[ -s "$cdfile" ]]; then
    target="$(cat "$cdfile")"
  fi
  rm -f "$cdfile"
  if (( rc != 0 )); then
    return $rc
  fi
  if [[ -n "$target" ]]; then
    cd -- "$target"
  fi
}

h() { hop "$@"; }

`

func newShellInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init <shell>",
		Short: "emit shell integration (zsh or bash)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return &errExitCode{code: 2, msg: "hop shell-init: missing shell. Supported: zsh, bash"}
			}
			shell := args[0]
			if shell != "zsh" && shell != "bash" {
				return &errExitCode{code: 2, msg: fmt.Sprintf("hop shell-init: unsupported shell '%s'. Supported: zsh, bash", shell)}
			}
			out := cmd.OutOrStdout()
			// io.WriteString avoids vet's printf-args check (posixInit contains
			// shell `printf` directives like %s, which vet would otherwise flag
			// as a missing format argument to fmt.Fprint).
			if _, err := io.WriteString(out, posixInit); err != nil {
				return fmt.Errorf("hop shell-init: write: %w", err)
			}
			// Append cobra-generated completion. rootForCompletion is set in main();
			// in tests that run RunE without main(), it may be nil.
			if rootForCompletion != nil {
				switch shell {
				case "zsh":
					if err := rootForCompletion.GenZshCompletion(out); err != nil {
						return fmt.Errorf("hop shell-init: zsh completion: %w", err)
					}
					// Cobra registers the completion only for `hop`; share it with the
					// `h` alias so tab completion works there too.
					fmt.Fprint(out, "\ncompdef _hop h\n")
				case "bash":
					if err := rootForCompletion.GenBashCompletionV2(out, true); err != nil {
						return fmt.Errorf("hop shell-init: bash completion: %w", err)
					}
					// Bash's `complete` accepts multiple command names; mirror compdef
					// for the h alias. The cobra-generated function is __start_hop.
					fmt.Fprint(out, "\ncomplete -o default -F __start_hop h\n")
				}
			}
			return nil
		},
	}
}
