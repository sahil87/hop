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

// isTTY reports whether a controlling terminal is available for an interactive
// fzf selection.
//
// It probes /dev/tty — NOT os.Stdin — because fzf opens the controlling
// terminal directly for its picker UI and reads candidates (the list hop pipes
// in) from stdin separately (internal/fzf.Pick feeds a strings.Reader as fzf's
// stdin). Keying on os.Stdin would mis-report `hop </dev/null` (or any stdin
// redirect) at a real terminal as "no TTY" even though fzf could still run via
// /dev/tty. An agent or CI run with no controlling terminal has no /dev/tty to
// open, so the probe still correctly returns false there — the guard's intent.
//
// Declared as a package-level var (mirroring idea's internal/idea.IsTTY seam
// and hop's own pickResolve/pickOne/listWorktrees idiom) so tests swap it to
// simulate a non-interactive environment without allocating a PTY.
var isTTY = func() bool {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	defer f.Close()
	return term.IsTerminal(int(f.Fd()))
}
