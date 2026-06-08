package main

import (
	"errors"
	"strings"
	"testing"
)

// emitZsh runs `hop shell-init zsh` with rootForCompletion set (mirrors main())
// and returns the emitted shim text (including the appended cobra completion).
func emitZsh(t *testing.T) string {
	t.Helper()
	rootForCompletion = newRootCmd()
	t.Cleanup(func() { rootForCompletion = nil })
	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	return stdout.String()
}

// shimOnly returns just the hop-authored shim portion (posixInit), stripping the
// cobra-generated completion script that shell-init appends. Cobra's completion
// machinery contains its own `eval` calls and `_describe` plumbing that are NOT
// part of hop's protocol shim — assertions about the shim's shape must scope to
// this portion only. The cobra zsh completion begins with `_hop()` (the
// completion function); posixInit defines `hop()`/`_hop_passthrough`/`h()` and
// ends before it.
func shimOnly(out string) string {
	// The cobra-generated completion is appended after posixInit, beginning with
	// the `#compdef hop` directive (zsh) or `# bash completion ...` banner.
	for _, marker := range []string{"#compdef hop", "# bash completion"} {
		if idx := strings.Index(out, marker); idx >= 0 {
			return out[:idx]
		}
	}
	return out
}

// TestShellInitZshContainsHopFunctionAndHAlias asserts the emitted shim defines
// the hop() interpreter and the h alias, and that the cobra-generated _hop
// completion is appended.
func TestShellInitZshContainsHopFunctionAndHAlias(t *testing.T) {
	out := emitZsh(t)
	if !strings.Contains(out, "hop()") {
		t.Fatalf("expected `hop()` function, got:\n%s", out)
	}
	if !strings.Contains(out, `h() { hop "$@"; }`) {
		t.Fatalf("expected `h()` alias, got:\n%s", out)
	}
	if !strings.Contains(out, "_hop") {
		t.Fatalf("expected cobra-generated _hop completion, got:\n%s", out)
	}
}

// TestShellInitZshDropsHiAlias asserts the `hi` alias was removed (intake §8).
// `command hop` is the raw escape hatch.
func TestShellInitZshDropsHiAlias(t *testing.T) {
	out := emitZsh(t)
	if strings.Contains(out, "hi()") {
		t.Fatalf("expected `hi()` alias to be removed, got:\n%s", out)
	}
	if strings.Contains(out, "compdef _hop h hi") {
		t.Fatalf("expected completion registration to drop `hi`, got:\n%s", out)
	}
	if !strings.Contains(out, "compdef _hop h\n") {
		t.Fatalf("expected `compdef _hop h` (h alias only), got:\n%s", out)
	}
}

// TestShellInitZshCallsShimPlan asserts the shim asks the binary to classify the
// user's argv via the hidden --shim-plan flag (the core of the protocol).
func TestShellInitZshCallsShimPlan(t *testing.T) {
	out := emitZsh(t)
	if !strings.Contains(out, `plan="$(command hop --shim-plan "$@")" || return $?`) {
		t.Fatalf("expected `plan=\"$(command hop --shim-plan \"$@\")\" || return $?`, got:\n%s", out)
	}
}

// TestShellInitZshCasesOnThreeKeywords asserts the shim branches on exactly the
// 3 protocol keywords (CD / RUN_IN_PARENT / PASSTHROUGH) — the entire vocabulary
// of the shim. This is what makes name-drift structurally impossible.
func TestShellInitZshCasesOnThreeKeywords(t *testing.T) {
	out := emitZsh(t)
	for _, kw := range []string{"CD)", "RUN_IN_PARENT)", "PASSTHROUGH)"} {
		if !strings.Contains(out, kw) {
			t.Fatalf("expected protocol case `%s`, got:\n%s", kw, out)
		}
	}
}

// TestShellInitZshRunInParentRunsUserWords asserts the RUN_IN_PARENT arm cds to
// the resolved path then runs the user's already-parsed words (`shift; "$@"`),
// NOT an eval of binary output. This is the security-critical contract
// (Constitution I — no shell-injection surface).
func TestShellInitZshRunInParentRunsUserWords(t *testing.T) {
	out := emitZsh(t)
	// The arm shifts off the selection and runs the remaining words verbatim.
	if !strings.Contains(out, "shift") {
		t.Fatalf("expected `shift` in RUN_IN_PARENT arm, got:\n%s", out)
	}
	if !strings.Contains(out, `"$@"`) {
		t.Fatalf("expected `\"$@\"` (run user's literal words) in RUN_IN_PARENT arm, got:\n%s", out)
	}
}

// TestShellInitZshNeverEvalsBinaryOutput asserts the shim NEVER pipes binary
// output into eval (Constitution I). The plan path is parsed with sed and used
// only as a quoted cd operand; the action is the user's already-parsed "$@".
func TestShellInitZshNeverEvalsBinaryOutput(t *testing.T) {
	// Scope to the hop-authored shim's executable lines (drop comments — the
	// leading banner legitimately contains `eval "$(hop shell-init ...)"`, which
	// is the install instruction, not the shim evaling binary output). The
	// appended cobra completion has its own `eval` plumbing (not a hop concern).
	out := shimOnly(emitZsh(t))
	for _, line := range strings.Split(out, "\n") {
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "#") {
			continue // comment line
		}
		if strings.Contains(code, "eval ") || strings.Contains(code, "eval\t") || code == "eval" {
			t.Fatalf("expected NO executable `eval` in the shim (shell-injection surface), got line: %q", line)
		}
	}
}

// TestShellInitZshHardCodesNoSubcommandNames asserts the shim contains no cobra
// subcommand-name case list — the permanent fix for the stale-shim bug class
// (intake §3). The only literals the shim branches on are the 3 protocol
// keywords. We check the former hard-coded list is gone.
func TestShellInitZshHardCodesNoSubcommandNames(t *testing.T) {
	out := emitZsh(t)
	// The old shim had a case arm like `add|rm|clone|pull|...|completion)`.
	for _, frag := range []string{"add|rm|clone", "|completion)", "_hop_dispatch"} {
		if strings.Contains(out, frag) {
			t.Fatalf("expected NO subcommand case-list fragment %q in the protocol shim, got:\n%s", frag, out)
		}
	}
	// Spot-check that individual subcommand names do not appear as case arms.
	for _, name := range []string{"clone)", "add)", "pull)", "push)", "sync)", "ls)"} {
		if strings.Contains(out, name) {
			t.Fatalf("expected NO `%s` case arm (subcommand names live only in cobra), got:\n%s", name, out)
		}
	}
}

// TestShellInitZshDropsDashRRewrite asserts the shim no longer rewrites the
// user-facing form to the binary's `-R` shape (the -R flag is removed; tool-form
// is native grammar via RUN_IN_PARENT).
func TestShellInitZshDropsDashRRewrite(t *testing.T) {
	out := emitZsh(t)
	if strings.Contains(out, "-R") {
		t.Fatalf("expected NO `-R` rewrite in the shim (flag removed), got:\n%s", out)
	}
}

// TestShellInitZshRoutesCompletionToBinary asserts __complete* introspection
// goes directly to the binary, NOT through --shim-plan (which would classify
// the completion request instead of answering it).
func TestShellInitZshRoutesCompletionToBinary(t *testing.T) {
	out := shimOnly(emitZsh(t))
	if !strings.Contains(out, "__complete*)") {
		t.Fatalf("expected `__complete*)` case forwarding completion to the binary, got:\n%s", out)
	}
	// The __complete arm must NOT route through --shim-plan (which would classify
	// the completion request instead of answering it). Slice the arm body and
	// assert it forwards directly to the binary.
	armStart := strings.Index(out, "__complete*)")
	armEnd := strings.Index(out[armStart:], ";;")
	if armEnd < 0 {
		t.Fatalf("could not find end of __complete arm")
	}
	arm := out[armStart : armStart+armEnd]
	// Check executable lines only — the arm's comment legitimately explains why
	// it avoids --shim-plan, so a substring match on the whole arm would false-fire.
	for _, line := range strings.Split(arm, "\n") {
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "#") {
			continue
		}
		if strings.Contains(code, "--shim-plan") {
			t.Fatalf("expected __complete arm to NOT invoke --shim-plan, got line: %q", line)
		}
	}
	if !strings.Contains(arm, `command hop "$@"`) {
		t.Fatalf("expected __complete arm to forward `command hop \"$@\"`, got arm:\n%s", arm)
	}
}

// TestShellInitZshPassthroughUsesUnifiedCDChannel asserts the PASSTHROUGH arm
// provides the unified WT_CD_FILE side-channel (collapsing the former
// where/open/clone handoffs into one) and never captures stdout via $(...)
// (which would swallow wt's interactive menu).
func TestShellInitZshPassthroughUsesUnifiedCDChannel(t *testing.T) {
	out := emitZsh(t)
	if !strings.Contains(out, "_hop_passthrough") {
		t.Fatalf("expected `_hop_passthrough` helper, got:\n%s", out)
	}
	if !strings.Contains(out, "mktemp -t hop-cd.XXXXXX") {
		t.Errorf("expected `mktemp -t hop-cd.XXXXXX` temp file, got:\n%s", out)
	}
	if !strings.Contains(out, `WT_CD_FILE="$cdfile" command hop "$@"`) {
		t.Errorf("expected `WT_CD_FILE=\"$cdfile\" command hop \"$@\"`, got:\n%s", out)
	}
	if !strings.Contains(out, `[[ -s "$cdfile" ]]`) {
		t.Errorf("expected `[[ -s \"$cdfile\" ]]` non-empty test, got:\n%s", out)
	}
	if !strings.Contains(out, `rm -f "$cdfile"`) {
		t.Errorf("expected `rm -f \"$cdfile\"` cleanup, got:\n%s", out)
	}
	if strings.Contains(out, `target="$(command hop`) {
		t.Errorf("expected NO stdout capture of `command hop` (would swallow wt's menu), got:\n%s", out)
	}
}

// TestShellInitZshOmitsLegacyShape asserts the collapsed protocol shim no longer
// emits the legacy precedence-ladder constructs.
func TestShellInitZshOmitsLegacyShape(t *testing.T) {
	out := emitZsh(t)
	for _, frag := range []string{`command -v "$1"`, `type "$1"`, "is a shell builtin", "_hop_dispatch", "WT_WRAPPER"} {
		if strings.Contains(out, frag) {
			t.Errorf("expected legacy fragment %q to be removed, got:\n%s", frag, out)
		}
	}
}

func TestShellInitBashEmitsFunctionAndCompletion(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "bash")
	if err != nil {
		t.Fatalf("shell-init bash: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "hop()") {
		t.Fatalf("expected `hop()` function in output, got:\n%s", out)
	}
	if !strings.Contains(out, `h() { hop "$@"; }`) {
		t.Fatalf("expected `h()` alias, got:\n%s", out)
	}
	// Bash uses `complete -F __start_hop` (not `compdef`), sharing with `h` only.
	if !strings.Contains(out, "complete -o default -F __start_hop h\n") {
		t.Fatalf("expected bash `complete -F __start_hop h` (no hi), got:\n%s", out)
	}
	if strings.Contains(out, "__start_hop h hi") {
		t.Fatalf("expected bash completion to drop the `hi` alias, got:\n%s", out)
	}
	if !strings.Contains(out, "__start_hop") {
		t.Fatalf("expected cobra-generated `__start_hop` completion fn, got:\n%s", out)
	}
	if !strings.Contains(out, "--shim-plan") {
		t.Fatalf("expected bash shim to call --shim-plan, got:\n%s", out)
	}
}

func TestShellInitMissingShell(t *testing.T) {
	_, _, err := runArgs(t, "shell-init")
	if err == nil {
		t.Fatalf("expected error when no shell arg")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T", err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit 2, got %d", withCode.code)
	}
	if !strings.Contains(withCode.msg, "missing shell") {
		t.Fatalf("unexpected message: %q", withCode.msg)
	}
	if !strings.Contains(withCode.msg, "zsh") || !strings.Contains(withCode.msg, "bash") {
		t.Fatalf("expected message to mention both zsh and bash: %q", withCode.msg)
	}
}

func TestShellInitUnsupportedShell(t *testing.T) {
	_, _, err := runArgs(t, "shell-init", "fish")
	if err == nil {
		t.Fatalf("expected error for unsupported shell")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T", err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit 2, got %d", withCode.code)
	}
	if !strings.Contains(withCode.msg, "unsupported shell 'fish'") {
		t.Fatalf("unexpected message: %q", withCode.msg)
	}
}
