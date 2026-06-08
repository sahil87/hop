package main

import (
	"bytes"
	"strings"
	"testing"
)

// shimPlanYAML defines a registry with a `web` group (two repos: frontend,
// backend) used to exercise the --shim-plan classifier across singular
// (repo) and plural (group/--all) selections.
const shimPlanYAML = `repos:
  web:
    dir: /tmp/shim-plan-test
    urls:
      - git@github.com:me/frontend.git
      - git@github.com:me/backend.git
`

// runPlan invokes runShimPlan with the given args against the shimPlanYAML
// fixture, returning stdout, stderr, and the exit code.
func runPlan(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	writeReposFixture(t, shimPlanYAML)
	var out, errOut bytes.Buffer
	code = runShimPlan(&out, &errOut, args)
	return out.String(), errOut.String(), code
}

// TestShimPlanCDBareSelection verifies a bare selection (no action) classifies
// as CD\n<path>.
func TestShimPlanCDBareSelection(t *testing.T) {
	stdout, _, code := runPlan(t, "frontend")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.HasPrefix(stdout, planCD+"\n") {
		t.Fatalf("expected CD plan, got %q", stdout)
	}
	if !strings.Contains(stdout, "/tmp/shim-plan-test/frontend") {
		t.Fatalf("expected resolved frontend path, got %q", stdout)
	}
}

// TestShimPlanCDExplicitVerb verifies `<repo> cd` classifies as CD.
func TestShimPlanCDExplicitVerb(t *testing.T) {
	stdout, _, code := runPlan(t, "frontend", "cd")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.HasPrefix(stdout, planCD+"\n") {
		t.Fatalf("expected CD plan, got %q", stdout)
	}
}

// TestShimPlanRunInParentToolForm verifies a non-verb action token (tool-form)
// classifies as RUN_IN_PARENT\n<path> so the shim cds there and runs the words.
func TestShimPlanRunInParentToolForm(t *testing.T) {
	stdout, _, code := runPlan(t, "frontend", "git", "pull")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.HasPrefix(stdout, planRunInParent+"\n") {
		t.Fatalf("expected RUN_IN_PARENT plan, got %q", stdout)
	}
	if !strings.Contains(stdout, "/tmp/shim-plan-test/frontend") {
		t.Fatalf("expected resolved frontend path, got %q", stdout)
	}
}

// TestShimPlanPassthroughWhereOpen verifies the builtin where/open verbs (binary
// owns them) classify as PASSTHROUGH.
func TestShimPlanPassthroughWhereOpen(t *testing.T) {
	for _, verb := range []string{"where", "open"} {
		stdout, _, code := runPlan(t, "frontend", verb)
		if code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", verb, code)
		}
		if strings.TrimSpace(stdout) != planPassthrough {
			t.Fatalf("%s: expected PASSTHROUGH, got %q", verb, stdout)
		}
	}
}

// TestShimPlanPassthroughBatchVerbSingular verifies a batch verb on a singular
// selection classifies as PASSTHROUGH (the binary runs the batch machinery).
func TestShimPlanPassthroughBatchVerbSingular(t *testing.T) {
	for _, verb := range []string{"pull", "push", "sync"} {
		stdout, _, code := runPlan(t, "frontend", verb)
		if code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", verb, code)
		}
		if strings.TrimSpace(stdout) != planPassthrough {
			t.Fatalf("%s: expected PASSTHROUGH, got %q", verb, stdout)
		}
	}
}

// TestShimPlanPassthroughSubcommand verifies a known cobra subcommand at $1
// classifies as PASSTHROUGH (the binary handles add/rm/clone/ls/...).
func TestShimPlanPassthroughSubcommand(t *testing.T) {
	for _, sub := range []string{"add", "rm", "clone", "ls", "config", "update", "shell-init", "help"} {
		stdout, _, code := runPlan(t, sub)
		if code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", sub, code)
		}
		if strings.TrimSpace(stdout) != planPassthrough {
			t.Fatalf("%s: expected PASSTHROUGH, got %q", sub, stdout)
		}
	}
}

// TestShimPlanPassthroughFlag verifies a flag other than --all classifies as
// PASSTHROUGH (so --help/--version/-h reach cobra).
func TestShimPlanPassthroughFlag(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "--version", "-v"} {
		stdout, _, code := runPlan(t, flag)
		if code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", flag, code)
		}
		if strings.TrimSpace(stdout) != planPassthrough {
			t.Fatalf("%s: expected PASSTHROUGH, got %q", flag, stdout)
		}
	}
}

// TestShimPlanPassthroughCompletion verifies __complete* classifies as
// PASSTHROUGH (defense-in-depth; the shim forwards it directly anyway).
func TestShimPlanPassthroughCompletion(t *testing.T) {
	stdout, _, code := runPlan(t, "__complete", "frontend", "")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout) != planPassthrough {
		t.Fatalf("expected PASSTHROUGH, got %q", stdout)
	}
}

// TestShimPlanPluralAllBatchVerbPassthrough verifies `--all <batch-verb>`
// classifies as PASSTHROUGH (the binary fans out the batch).
func TestShimPlanPluralAllBatchVerbPassthrough(t *testing.T) {
	stdout, _, code := runPlan(t, "--all", "pull")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout) != planPassthrough {
		t.Fatalf("expected PASSTHROUGH, got %q", stdout)
	}
}

// TestShimPlanPluralGroupBatchVerbPassthrough verifies `<group> <batch-verb>`
// classifies as PASSTHROUGH.
func TestShimPlanPluralGroupBatchVerbPassthrough(t *testing.T) {
	stdout, _, code := runPlan(t, "web", "sync")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout) != planPassthrough {
		t.Fatalf("expected PASSTHROUGH, got %q", stdout)
	}
}

// TestShimPlanPluralNoActionErrors verifies a plural selection (group) with no
// action token is a usage error (exit 2) — a plural selection has no single cd
// target.
func TestShimPlanPluralNoActionErrors(t *testing.T) {
	stdout, stderr, code := runPlan(t, "web")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on usage error, got %q", stdout)
	}
	if !strings.Contains(stderr, "plural selection") {
		t.Fatalf("expected plural-selection hint, got %q", stderr)
	}
}

// TestShimPlanPluralInteractiveRefused verifies an interactive (non-batch)
// action on a plural selection is refused with exit 2 (the guard).
func TestShimPlanPluralInteractiveRefused(t *testing.T) {
	for _, sel := range [][]string{{"--all", "code", "."}, {"web", "cd"}, {"web", "open"}} {
		stdout, stderr, code := runPlan(t, sel...)
		if code != 2 {
			t.Fatalf("%v: expected exit 2, got %d (stdout=%q)", sel, code, stdout)
		}
		if stdout != "" {
			t.Fatalf("%v: expected empty stdout, got %q", sel, stdout)
		}
		if !strings.Contains(stderr, "not a batch action") {
			t.Fatalf("%v: expected not-a-batch-action hint, got %q", sel, stderr)
		}
	}
}

// TestShimPlanGracefulDegradation verifies an unrecognized invocation that looks
// like a flag classifies as PASSTHROUGH (intake §10 — old-shim calls degrade
// gracefully; cobra prints the normal error on the subsequent passthrough).
func TestShimPlanGracefulDegradation(t *testing.T) {
	stdout, _, code := runPlan(t, "--some-old-flag", "arg")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout) != planPassthrough {
		t.Fatalf("expected PASSTHROUGH for unrecognized flag, got %q", stdout)
	}
}

// TestExtractShimPlan verifies the argv split that main() uses to detect and
// strip the --shim-plan flag.
func TestExtractShimPlan(t *testing.T) {
	rest, ok := extractShimPlan([]string{"hop", "--shim-plan", "frontend", "git", "pull"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if strings.Join(rest, " ") != "frontend git pull" {
		t.Fatalf("expected rest [frontend git pull], got %v", rest)
	}

	if _, ok := extractShimPlan([]string{"hop", "ls"}); ok {
		t.Fatalf("expected ok=false when --shim-plan absent")
	}

	// --shim-plan with no trailing args yields empty rest, ok=true.
	rest, ok = extractShimPlan([]string{"hop", "--shim-plan"})
	if !ok || len(rest) != 0 {
		t.Fatalf("expected ok=true and empty rest, got ok=%v rest=%v", ok, rest)
	}
}
