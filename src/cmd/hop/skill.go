package main

import (
	_ "embed"

	"github.com/spf13/cobra"
)

//go:generate ../../../scripts/sync-skill.sh

// skillBundle is the canonical agent skill bundle, copied into this package dir
// from docs/site/skill.md by scripts/sync-skill.sh and embedded at build time.
// The Go module root is src/ and docs/site/ sits above it, so //go:embed cannot
// reach the canonical file directly — the sync step copies it here first (see
// scripts/sync-skill.sh). The committed copy is what a clean `go build ./...`
// compiles; the drift-guard test in skill_test.go keeps it byte-honest against
// docs/site/skill.md on every `go test`.
//
//go:embed skill.md
var skillBundle []byte

// skillLong is the help text for `hop skill`. It documents what the bundle is
// (the standard's "usage briefing" genre) and how the command behaves, so a
// human reading `hop skill -h` understands it emits the raw bundle to stdout.
const skillLong = `Print hop's agent skill bundle — a stable, one-page usage briefing for an
agent driving an installed hop, byte-identical to the repo's canonical
docs/site/skill.md (also rendered at https://shll.ai/hop/skill).

The bundle is raw markdown written verbatim to stdout: no rendering, no pager,
no added framing (stdout is data). It is embedded in the binary at build time,
so it is offline and version-locked to this release.`

// newSkillCmd builds the visible `hop skill` subcommand mandated by the toolkit
// skill standard (shll standards skill). It writes the embedded bundle bytes
// verbatim to stdout, leaves stderr empty, and exits 0. Unlike help-dump it is
// NOT hidden — the standard says each tool "exposes" the subcommand and the
// bundle is a published page, so visibility serves agent discovery (principle
// №10). The name is fixed by the standard as exactly "skill".
func newSkillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skill",
		Short: "print hop's agent skill bundle (usage briefing) to stdout",
		Long:  skillLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := cmd.OutOrStdout().Write(skillBundle)
			return err
		},
	}
}
