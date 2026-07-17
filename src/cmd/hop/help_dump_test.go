package main

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// runHelpDump builds a fresh root (mirroring main()'s wiring: Version set on
// the root), executes `help-dump` through it so cmd.Root() resolves to the
// wired root, and returns the captured stdout bytes plus the parsed Doc.
func runHelpDump(t *testing.T, ver string) ([]byte, Doc) {
	t.Helper()
	root := newRootCmd()
	root.Version = ver
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs([]string{"help-dump"})
	if err := root.Execute(); err != nil {
		t.Fatalf("hop help-dump: %v (stderr: %s)", err, stderr.String())
	}
	raw := stdout.Bytes()
	var doc Doc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal help-dump output: %v\noutput:\n%s", err, raw)
	}
	return raw, doc
}

// findChild returns the named direct child of node, or a zero Node with ok=false.
func findChild(node Node, name string) (Node, bool) {
	for _, c := range node.Commands {
		if c.Name == name {
			return c, true
		}
	}
	return Node{}, false
}

// TestHelpDumpDocFields asserts the top-level document fields: tool, schema
// version, and version sourced from rootCmd.Version.
func TestHelpDumpDocFields(t *testing.T) {
	_, doc := runHelpDump(t, "1.2.3")

	if doc.Tool != "hop" {
		t.Errorf("tool = %q, want %q", doc.Tool, "hop")
	}
	if doc.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", doc.SchemaVersion)
	}
	if doc.Version != "1.2.3" {
		t.Errorf("version = %q, want %q (must reflect rootCmd.Version)", doc.Version, "1.2.3")
	}
}

// TestHelpDumpEnvelopeShape asserts the emitted envelope matches the shll
// help-dump standard exactly: keys {tool, version, schema_version, root} and
// NO captured_at (the standard forbids it — the shll.ai puller owns the capture
// timestamp). Asserting against the raw JSON, not the Go struct, so a
// re-introduced captured_at field is caught even if the struct changes.
func TestHelpDumpEnvelopeShape(t *testing.T) {
	raw, _ := runHelpDump(t, "dev")

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("unmarshal help-dump envelope: %v\noutput:\n%s", err, raw)
	}
	if _, present := top["captured_at"]; present {
		t.Errorf("captured_at present in envelope; the standard forbids it (the shll.ai puller stamps it)\noutput:\n%s", raw)
	}
	got := make(map[string]bool, len(top))
	for k := range top {
		got[k] = true
	}
	for _, want := range []string{"tool", "version", "schema_version", "root"} {
		if !got[want] {
			t.Errorf("envelope missing required key %q; got keys %v", want, keysOf(top))
		}
		delete(got, want)
	}
	for extra := range got {
		t.Errorf("envelope has unexpected key %q; standard shape is {tool, version, schema_version, root}", extra)
	}
}

// keysOf returns the sorted key set of a raw-JSON object, for error messages.
// Sorting makes failure output deterministic regardless of Go's map iteration
// order.
func keysOf(m map[string]json.RawMessage) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// TestHelpDumpVersionNotHardcoded asserts version tracks whatever the root's
// Version is — including the unstamped "dev" default.
func TestHelpDumpVersionNotHardcoded(t *testing.T) {
	_, doc := runHelpDump(t, "dev")
	if doc.Version != "dev" {
		t.Errorf("version = %q, want %q (must read rootCmd.Version, not hardcode)", doc.Version, "dev")
	}
}

// TestHelpDumpFiltersChildren asserts the walk includes real subcommands and
// excludes cobra's auto-generated completion/help and hidden commands
// (including help-dump itself).
func TestHelpDumpFiltersChildren(t *testing.T) {
	_, doc := runHelpDump(t, "dev")

	if _, ok := findChild(doc.Root, "clone"); !ok {
		t.Errorf("expected `clone` present in root.commands")
	}
	for _, excluded := range []string{"completion", "help", "help-dump"} {
		if _, ok := findChild(doc.Root, excluded); ok {
			t.Errorf("expected %q absent from root.commands", excluded)
		}
	}
}

// TestHelpDumpLeavesSerializeEmptyArray asserts leaf nodes serialize commands
// as [] rather than null.
func TestHelpDumpLeavesSerializeEmptyArray(t *testing.T) {
	raw, _ := runHelpDump(t, "dev")
	if bytes.Contains(raw, []byte(`"commands":null`)) || bytes.Contains(raw, []byte(`"commands": null`)) {
		t.Errorf("found null commands in output; leaves must serialize as []\noutput:\n%s", raw)
	}
}

// TestHelpDumpSubcommandNode asserts a known leaf subcommand node carries the
// expected name/path/short and that its text contains its usage line.
func TestHelpDumpSubcommandNode(t *testing.T) {
	_, doc := runHelpDump(t, "dev")

	update, ok := findChild(doc.Root, "update")
	if !ok {
		t.Fatalf("expected `update` present in root.commands")
	}
	if update.Name != "update" {
		t.Errorf("update.name = %q, want %q", update.Name, "update")
	}
	if update.Path != "hop update" {
		t.Errorf("update.path = %q, want %q", update.Path, "hop update")
	}
	// Compare against the command's registered Short rather than a literal copy,
	// so editing `update`'s description doesn't break this help-dump wiring test.
	if want := newUpdateCmd().Short; update.Short != want {
		t.Errorf("update.short = %q, want %q", update.Short, want)
	}
	// `update` has no Long, so text == UsageString() alone, which includes the
	// usage line `hop update [flags]`.
	if !strings.Contains(update.Text, "hop update") {
		t.Errorf("update.text does not contain its usage; got:\n%s", update.Text)
	}
}

// TestHelpDumpRootTextUsesLong asserts the uniform text rule for a node with a
// non-empty Long: text begins with the Long narrative followed by a blank line
// then the usage block.
func TestHelpDumpRootTextUsesLong(t *testing.T) {
	_, doc := runHelpDump(t, "dev")

	if !strings.HasPrefix(doc.Root.Text, rootLong+"\n\n") {
		t.Errorf("root.text should begin with rootLong + blank line; got prefix:\n%.200q", doc.Root.Text)
	}
	if !strings.Contains(doc.Root.Text, "Usage:") {
		t.Errorf("root.text should contain the usage block; got:\n%s", doc.Root.Text)
	}
}

// TestHelpDumpNotInUserHelp asserts the hidden subcommand does not appear in
// `hop --help`.
func TestHelpDumpNotInUserHelp(t *testing.T) {
	stdout, _, err := runArgs(t, "--help")
	if err != nil {
		t.Fatalf("hop --help: %v", err)
	}
	if strings.Contains(stdout.String(), "help-dump") {
		t.Errorf("help-dump should be hidden from --help; got:\n%s", stdout.String())
	}
}
