package main

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// helpDumpSchemaVersion is the version of the help/<tool>.json contract this
// producer emits. It is pinned to the frozen shll.ai contract (see
// help/wt.json reference); bump only when the document shape changes.
const helpDumpSchemaVersion = 1

// Doc is the top-level help-dump document. Field order matches the frozen
// shll.ai contract: tool, version, captured_at, schema_version, root.
type Doc struct {
	Tool string `json:"tool"`
	// Version is read from rootCmd.Version (ldflag-injected main.version);
	// "dev" in an unstamped local build. Never hardcoded.
	Version string `json:"version"`
	// CapturedAt is left empty by the producer to keep the dump
	// deterministic/testable; CI injects a date-floored UTC value.
	CapturedAt    string `json:"captured_at"`
	SchemaVersion int    `json:"schema_version"`
	Root          Node   `json:"root"`
}

// Node describes a single command in the cobra tree. Commands is always a
// non-nil slice so leaves serialize as [] rather than null.
type Node struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Short    string `json:"short"`
	Usage    string `json:"usage"`
	Text     string `json:"text"`
	Commands []Node `json:"commands"`
}

// buildHelpDoc constructs the help-dump document from the live root command,
// so version (rootCmd.Version) and the full subcommand tree are present.
func buildHelpDoc(root *cobra.Command) Doc {
	return Doc{
		Tool:          "hop",
		Version:       root.Version,
		CapturedAt:    "",
		SchemaVersion: helpDumpSchemaVersion,
		Root:          buildNode(root),
	}
}

// buildNode reads structured cobra fields (not regex-parsed -h text) into a
// Node and recurses into non-skipped children.
func buildNode(cmd *cobra.Command) Node {
	node := Node{
		Name:     cmd.Name(),
		Path:     cmd.CommandPath(),
		Short:    cmd.Short,
		Usage:    cmd.UseLine(),
		Text:     nodeText(cmd),
		Commands: []Node{},
	}
	for _, child := range cmd.Commands() {
		if shouldSkipChild(child) {
			continue
		}
		node.Commands = append(node.Commands, buildNode(child))
	}
	return node
}

// nodeText builds the uniform `text` field: Long + blank line + usage when
// Long is set, else the usage block alone.
func nodeText(cmd *cobra.Command) string {
	if cmd.Long != "" {
		return cmd.Long + "\n\n" + cmd.UsageString()
	}
	return cmd.UsageString()
}

// shouldSkipChild filters cobra's auto-generated completion/help commands,
// any hidden command (which drops help-dump itself), and — defensively —
// additional help-topic pseudo-commands.
func shouldSkipChild(c *cobra.Command) bool {
	switch c.Name() {
	case "completion", "help":
		return true
	}
	return c.Hidden || c.IsAdditionalHelpTopicCommand()
}

// newHelpDumpCmd returns the hidden `hop help-dump` subcommand. It marshals the
// help tree of the live root command to stdout with 2-space indentation. Being
// Hidden, it self-filters out of its own output.
func newHelpDumpCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "help-dump",
		Short:  "emit the CLI help tree as JSON (hidden, for build-time publishing)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			doc := buildHelpDoc(cmd.Root())
			out, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return err
			}
			out = append(out, '\n')
			_, err = cmd.OutOrStdout().Write(out)
			return err
		},
	}
}
