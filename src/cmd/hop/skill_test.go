package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// skillCanonicalPath is the canonical bundle relative to this test file
// (src/cmd/hop/), i.e. three levels up under docs/site/. Single-sourced so the
// drift guard and the budget guard read the same file.
const skillCanonicalPath = "../../../docs/site/skill.md"

// skillMaxLines is the standard's hard budget for the bundle (shll standards
// skill: "Bounded — ≤150 lines"). Pinned here so a future edit can't silently
// blow it.
const skillMaxLines = 150

// runSkill builds a fresh root (mirroring main()'s wiring), executes the given
// args through it, and returns captured stdout/stderr plus the RunE error. The
// visible `skill` command resolves as a normal child of the wired root.
func runSkill(t *testing.T, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	root := newRootCmd()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	err = root.Execute()
	return stdout, stderr, err
}

// TestSkillEmbedMatchesCanonical is the drift guard: the embedded skill.md bytes
// MUST equal the canonical docs/site/skill.md. When someone edits the canonical
// file without re-running scripts/sync-skill.sh, this fails on every
// `go test ./...` and in the CI PR workflow, naming the fix.
func TestSkillEmbedMatchesCanonical(t *testing.T) {
	canonical, err := os.ReadFile(filepath.FromSlash(skillCanonicalPath))
	if err != nil {
		t.Fatalf("read canonical %s: %v", skillCanonicalPath, err)
	}
	if !bytes.Equal(skillBundle, canonical) {
		t.Errorf("embedded src/cmd/hop/skill.md has drifted from canonical docs/site/skill.md "+
			"(embedded len=%d, canonical len=%d) — run `just sync-skill` (or scripts/sync-skill.sh) and commit the refreshed copy",
			len(skillBundle), len(canonical))
	}
}

// TestSkillInvocationContract pins the standard's invocation contract: `hop
// skill` exits 0 (RunE returns nil), writes the embedded bundle byte-identically
// to stdout, and leaves stderr empty.
func TestSkillInvocationContract(t *testing.T) {
	stdout, stderr, err := runSkill(t, "skill")
	if err != nil {
		t.Fatalf("hop skill: unexpected error: %v (stderr: %s)", err, stderr.String())
	}
	if !bytes.Equal(stdout.Bytes(), skillBundle) {
		t.Errorf("stdout is not byte-identical to the embedded bundle (stdout len=%d, bundle len=%d)",
			stdout.Len(), len(skillBundle))
	}
	if stderr.Len() != 0 {
		t.Errorf("hop skill wrote to stderr, want empty: %q", stderr.String())
	}
}

// TestSkillRejectsArgs confirms `cobra.NoArgs` — `hop skill <extra>` is a usage
// error (RunE never runs), so the command takes no positionals or flags.
func TestSkillRejectsArgs(t *testing.T) {
	_, _, err := runSkill(t, "skill", "extra")
	if err == nil {
		t.Fatal("hop skill extra: want an error (cobra.NoArgs), got nil")
	}
}

// TestSkillVisible asserts the command is exposed (NOT Hidden), so it appears in
// `hop --help` and the help-dump tree — the standard says each tool "exposes"
// the subcommand.
func TestSkillVisible(t *testing.T) {
	cmd := newSkillCmd()
	if cmd.Use != "skill" {
		t.Errorf("Use = %q, want exactly %q", cmd.Use, "skill")
	}
	if cmd.Hidden {
		t.Error("skill command is Hidden, want visible (the standard exposes it)")
	}
}

// TestSkillBudget is the budget guard: the bundle MUST be ≤150 lines (the
// standard's hard budget). Reads the canonical file so an over-long edit fails
// even before the sync step runs.
func TestSkillBudget(t *testing.T) {
	canonical, err := os.ReadFile(filepath.FromSlash(skillCanonicalPath))
	if err != nil {
		t.Fatalf("read canonical %s: %v", skillCanonicalPath, err)
	}
	// Count newline-terminated lines plus a trailing partial line, if any.
	lines := bytes.Count(canonical, []byte{'\n'})
	if len(canonical) > 0 && canonical[len(canonical)-1] != '\n' {
		lines++
	}
	if lines > skillMaxLines {
		t.Errorf("skill bundle is %d lines, want ≤ %d (the standard's hard budget)", lines, skillMaxLines)
	}
}
