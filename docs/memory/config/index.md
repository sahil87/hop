# config — memory

| File | Topic |
|---|---|
| [search-order](search-order.md) | Single fixed path `$HOME/.config/hop/hop.yaml` (only `$HOME` consulted; no `$HOP_CONFIG`/`$XDG_CONFIG_HOME`), `Resolve` stat-then-not-found vs. `ResolveWriteTarget` no-stat, the read-vs-write split (readers error, writers auto-init), no fallback to legacy `repos.yaml` paths |
| [yaml-schema](yaml-schema.md) | Grouped schema: `config:` + `repos:` named groups (flat list or `dir`/`urls` map); URL parsing; path resolution |
| [init-bootstrap](init-bootstrap.md) | `hop config init` write target, mode 0644, embedded grouped-form starter; post-write tip text; **auto-init-on-write** (`EnsureSkeleton` minimal `repos: {}` skeleton for `hop add` / `scan --write` / `clone <url>`, `created:` announcement, read-vs-write split) |
| [scan](scan.md) | `hop config scan <dir>` — DFS walk, repo classification, slugify-based group invention, conflict resolution, render/merge through `internal/yamled` |
