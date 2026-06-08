# architecture — memory

| File | Topic |
|---|---|
| [package-layout](package-layout.md) | `src/cmd/hop/` + `src/internal/<pkg>/`, cobra wiring, pre-cobra `--shim-plan` classifier (`shim_plan.go`), reoriented batch verbs (`batch_verb.go`), `help_dump.go` cobra-tree producer, conventions, gyo0 line-reduction |
| [wrapper-boundaries](wrapper-boundaries.md) | `internal/proc` security choke point (now exposes `RunCapture`), `internal/fzf` wrapper, `internal/yamled` comment-preserving YAML edits, `internal/scan` git invocation routing |
