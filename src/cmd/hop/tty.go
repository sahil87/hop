package main

import (
	"errors"
	"os"

	"golang.org/x/term"
)

// errNoTTY signals that an interactive fzf selection was requested with no
// controlling TTY to pick with (e.g. an AI agent or CI script driving hop). It
// maps to the distinct exit code 3 in both translateExit (main.go) and
// shimResolveErr (shim_plan.go) — NOT 130 (fzf cancel), so an agent can tell
// "there is no terminal" apart from "the user pressed Esc" (intake 1x1u Item 3).
var errNoTTY = errors.New("no tty for interactive selection")

// noTTYHint is the exact stderr line printed when fzf would be spawned with no
// TTY. It points the caller at the two non-interactive escapes: name the repo
// directly, or enumerate repos as data via `hop ls --json`.
const noTTYHint = "hop: no TTY for interactive selection — pass a repo name or use `hop ls --json`"

// isTTY reports whether stdin is connected to an interactive terminal. fzf reads
// its candidate list from stdin and needs a real terminal to pick with, so the
// guard keys on stdin's fd. Declared as a package-level var (mirroring idea's
// internal/idea.IsTTY seam and hop's own pickResolve/pickOne/listWorktrees
// idiom) so tests swap it to simulate a non-interactive environment without
// allocating a PTY.
var isTTY = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
