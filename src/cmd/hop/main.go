// Command hop is a CLI for locating, opening, and operating on repos from hop.yaml.
// See `hop --help` for the user-facing surface; the canonical contract for this
// binary lives in the active fab change spec (under fab/changes/) until hydrated.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is the binary version, overridden via -ldflags "-X main.version=..." at build time.
var version = "dev"

// rootForCompletion holds a reference to the root cobra.Command so shell-init
// can call GenZshCompletion without threading rootCmd through every factory.
var rootForCompletion *cobra.Command

func main() {
	rootCmd := newRootCmd()
	rootCmd.Version = version
	rootForCompletion = rootCmd

	// --shim-plan must be handled before cobra parses argv: the action token
	// after the selection (e.g. `git pull`, `code .`) is an arbitrary child
	// command line, not a hop flag/subcommand. We classify the user's argv and
	// emit the fixed 3-keyword protocol the shell shim interprets (see
	// shim_plan.go). The shim runs the user's already-parsed words itself —
	// the binary never execs them (Constitution I: no shell-injection surface).
	if rest, ok := extractShimPlan(os.Args); ok {
		os.Exit(runShimPlan(os.Stdout, os.Stderr, rest))
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(translateExit(err))
	}
}

// translateExit maps errors returned from RunE to the spec's exit codes.
// Exit codes per docs/specs/cli-surface.md §"Exit Code Conventions":
//
//	0 success, 1 application error, 2 usage error, 3 no TTY, 130 user cancelled.
//
// Sentinels:
//   - errNoTTY         → 3 (interactive selection requested with no terminal)
//   - errFzfCancelled  → 130
//   - errSilent        → 1 (caller already wrote stderr)
//   - errExitCode{...} → custom code (used by `hop cd` to exit 2, `shell-init` for 2, etc.)
//
// Default: print the error to stderr and exit 1.
func translateExit(err error) int {
	if err == nil {
		return 0
	}
	var withCode *errExitCode
	if errors.As(err, &withCode) {
		if withCode.msg != "" {
			fmt.Fprintln(os.Stderr, withCode.msg)
		}
		return withCode.code
	}
	if errors.Is(err, errNoTTY) {
		fmt.Fprintln(os.Stderr, noTTYHint)
		return 3
	}
	if errors.Is(err, errFzfCancelled) {
		return 130
	}
	if errors.Is(err, errSilent) {
		return 1
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

// errExitCode carries an explicit exit code plus an optional stderr message.
// Used by subcommands that need to exit with codes other than 0 or 1
// (e.g. `hop cd` exits 2, `hop shell-init bash` exits 2).
type errExitCode struct {
	code int
	msg  string
}

func (e *errExitCode) Error() string { return e.msg }
