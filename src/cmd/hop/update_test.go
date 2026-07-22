package main

import (
	"strings"
	"testing"
)

// TestUpdateCobraWiring asserts that `hop update` is registered, accepts no
// args, and reaches the internal/update.Run code path. We exploit the fact
// that `go test` binaries do not live under /Cellar/, so the function
// short-circuits to the "not installed via Homebrew" branch — exercising the
// cobra plumbing without hitting brew.
//
// runArgs captures cmd.OutOrStdout() via SetOut, and update.Run writes its
// wrapper messages to the writer the cobra wrapper passes in (cmd.OutOrStdout()),
// so the captured buffer reflects the user-visible output.
func TestUpdateCobraWiring(t *testing.T) {
	stdout, _, err := runArgs(t, "update")
	if err != nil {
		t.Fatalf("hop update: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "was not installed via Homebrew") {
		t.Fatalf("expected non-brew hint in stdout, got:\n%s", got)
	}
}

// TestUpdateHelpAdvertisesSkipBrewUpdate pins the exact literal
// `--skip-brew-update` in `hop update --help` output — a frozen textual
// contract: shll discovers the flag via strings.Contains on the help text
// (shll update standard), so the substring must never drift.
func TestUpdateHelpAdvertisesSkipBrewUpdate(t *testing.T) {
	stdout, _, err := runArgs(t, "update", "--help")
	if err != nil {
		t.Fatalf("hop update --help: %v", err)
	}
	if !strings.Contains(stdout.String(), "--skip-brew-update") {
		t.Fatalf("expected literal `--skip-brew-update` in update --help, got:\n%s", stdout.String())
	}
}

func TestUpdateRejectsArgs(t *testing.T) {
	_, _, err := runArgs(t, "update", "extra")
	if err == nil {
		t.Fatalf("expected error from `update extra` (cobra.NoArgs)")
	}
}

func TestUpdateAppearsInHelp(t *testing.T) {
	stdout, _, err := runArgs(t, "--help")
	if err != nil {
		t.Fatalf("hop --help: %v", err)
	}
	// rootLong no longer carries per-subcommand cheat-sheet rows — `update`
	// is advertised by cobra's Available Commands section via its Short.
	if !strings.Contains(stdout.String(), "self-update the hop binary via Homebrew") {
		t.Fatalf("expected `update` Short in --help Available Commands, got:\n%s", stdout.String())
	}
}
