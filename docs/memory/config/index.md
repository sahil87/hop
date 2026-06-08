# config — memory

| File | Topic |
|---|---|
| [search-order](search-order.md) | Single fixed path `$HOME/.config/hop/hop.yaml` (only `$HOME` consulted; no `$HOP_CONFIG`/`$XDG_CONFIG_HOME`), `Resolve` stat-then-not-found vs. `ResolveWriteTarget` no-stat, the read-vs-write split (readers error, writers auto-init), no fallback to legacy `repos.yaml` paths |
| [yaml-schema](yaml-schema.md) | Grouped schema: `config:` + `repos:` named groups (flat list or `dir`/`urls` map); URL parsing; path resolution |
| [init-bootstrap](init-bootstrap.md) | `hop config init` write target, mode 0644, embedded grouped-form starter; post-write tip text (`hop add -r <dir>`); **auto-init-on-write** (`EnsureSkeleton` minimal `repos: {}` skeleton for `hop add` write mode / `clone <url>`, `created:` announcement, read-vs-write split) |
| [add-register](add-register.md) | `hop add <dir>` (single-dir) and `hop add -r <dir>` (recursive DFS walk) — repo classification, convention/slugify group assignment, `-g` forced group, `-p` print dry-run, conflict resolution, render/merge through `internal/yamled`. (`hop config scan` was deleted, no alias.) |
