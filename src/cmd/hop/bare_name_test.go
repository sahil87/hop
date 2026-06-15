package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestBareNameHint verifies the binary's 1-arg form returns a code-2
// errExitCode whose message is the bareNameHint constant verbatim, with
// empty stdout (no path leak on the error path). HOP_WRAPPER is cleared so the
// hint is NOT suppressed (intake Item 4 — suppression is the inverse case,
// covered by TestBareNameHintSuppressedUnderHopWrapper).
func TestBareNameHint(t *testing.T) {
	t.Setenv("HOP_WRAPPER", "")
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "hop")
	if err == nil {
		t.Fatalf("expected error from 1-arg bare form (cobra positional `hop`)")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if withCode.msg != bareNameHint {
		t.Fatalf("hint mismatch:\n  want: %s\n  got:  %s", bareNameHint, withCode.msg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on bare-name error, got: %q", stdout.String())
	}
}

// TestBareNameCdVerb verifies `hop <name> cd` returns a code-2 errExitCode
// whose message equals the cdHint constant verbatim and contains the new
// `cd "$(hop "<name>" where)"` fallback example. Stdout is empty.
func TestBareNameCdVerb(t *testing.T) {
	t.Setenv("HOP_WRAPPER", "")
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "hop", "cd")
	if err == nil {
		t.Fatalf("expected error from 2-arg cd verb (cobra positionals `hop cd`)")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if withCode.msg != cdHint {
		t.Fatalf("hint mismatch:\n  want: %s\n  got:  %s", cdHint, withCode.msg)
	}
	// Pin the new wording — the fallback example uses the new repo-first form.
	if !strings.Contains(withCode.msg, `cd "$(hop "<name>" where)"`) {
		t.Fatalf("expected hint to contain new repo-first fallback example, got: %s", withCode.msg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on cd-verb error, got: %q", stdout.String())
	}
}

// TestBareNameWhereVerb verifies `hop <name> where` resolves and prints the
// path to stdout, with empty stderr (no diagnostic noise on the happy path).
func TestBareNameWhereVerb(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, stderr, err := runArgs(t, "hop", "where")
	if err != nil {
		t.Fatalf("hop hop where: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "/tmp/test-repos/hop" {
		t.Fatalf("expected /tmp/test-repos/hop, got %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr on where-verb happy path, got: %q", stderr.String())
	}
}

// TestBareNameToolForm verifies tool-form attempts at the binary error with
// the parameterized tool-form hint matching `fmt.Sprintf(toolFormHintFmt, args[1])`
// byte-for-byte. Stdout is empty.
func TestBareNameToolForm(t *testing.T) {
	t.Setenv("HOP_WRAPPER", "")
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "hop", "cursor")
	if err == nil {
		t.Fatalf("expected error from tool-form attempt (`hop hop cursor`)")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	want := fmt.Sprintf(toolFormHintFmt, "cursor")
	if withCode.msg != want {
		t.Fatalf("tool-form hint mismatch:\n  want: %s\n  got:  %s", want, withCode.msg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on tool-form error, got: %q", stdout.String())
	}
}

// TestToolFormWithExtraArgsErrorsInBinary verifies that 3+ args (a tool-form
// invocation with arguments, e.g. `hop hop cursor extra`) reach RunE under the
// selection-first grammar — the root now accepts arbitrary args (the former
// cobra MaximumNArgs(2) cap was removed because tool-form actions carry their
// own argv). The binary can't run tools (that's the shim's RUN_IN_PARENT path),
// so it returns the tool-form hint via *errExitCode.
func TestToolFormWithExtraArgsErrorsInBinary(t *testing.T) {
	t.Setenv("HOP_WRAPPER", "")
	writeReposFixture(t, singleRepoYAML)

	_, _, err := runArgs(t, "hop", "cursor", "extra")
	if err == nil {
		t.Fatalf("expected tool-form error from `hop hop cursor extra`")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode (RunE ran the tool-form branch), got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if !strings.Contains(withCode.msg, "is not a hop verb") {
		t.Fatalf("expected tool-form hint, got: %s", withCode.msg)
	}
}

// --- HOP_WRAPPER hint suppression (intake 1x1u Item 4) --------------------

// TestBareNameHintSuppressedUnderHopWrapper asserts that with HOP_WRAPPER=1 the
// shell-only bare-name hint TEXT is suppressed but the exit-2 code is KEPT
// (mirror wt's "suppress the hint, not the error"). An empty errExitCode msg
// makes translateExit exit 2 without printing.
func TestBareNameHintSuppressedUnderHopWrapper(t *testing.T) {
	t.Setenv("HOP_WRAPPER", "1")
	writeReposFixture(t, singleRepoYAML)

	_, _, err := runArgs(t, "hop")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("exit code must stay 2 under HOP_WRAPPER, got %d", withCode.code)
	}
	if withCode.msg != "" {
		t.Fatalf("expected suppressed (empty) hint under HOP_WRAPPER=1, got: %q", withCode.msg)
	}
}

// TestCdVerbHintSuppressedUnderHopWrapper asserts the same for the `cd` verb.
func TestCdVerbHintSuppressedUnderHopWrapper(t *testing.T) {
	t.Setenv("HOP_WRAPPER", "1")
	writeReposFixture(t, singleRepoYAML)

	_, _, err := runArgs(t, "hop", "cd")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("exit code must stay 2 under HOP_WRAPPER, got %d", withCode.code)
	}
	if withCode.msg != "" {
		t.Fatalf("expected suppressed (empty) cd hint under HOP_WRAPPER=1, got: %q", withCode.msg)
	}
}

// TestToolFormHintSuppressedUnderHopWrapper asserts the same for tool-form.
func TestToolFormHintSuppressedUnderHopWrapper(t *testing.T) {
	t.Setenv("HOP_WRAPPER", "1")
	writeReposFixture(t, singleRepoYAML)

	_, _, err := runArgs(t, "hop", "cursor")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("exit code must stay 2 under HOP_WRAPPER, got %d", withCode.code)
	}
	if withCode.msg != "" {
		t.Fatalf("expected suppressed (empty) tool-form hint under HOP_WRAPPER=1, got: %q", withCode.msg)
	}
}
