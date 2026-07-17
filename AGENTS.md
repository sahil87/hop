# AGENTS.md

hop is a small Go CLI that turns one config file (`hop.yaml`) into a personal
directory of git repos — locate, open, list, clone, and batch-update them.
Part of the [sahil87 toolkit](https://shll.ai); tool page: <https://shll.ai/hop>.

## Toolkit standards (binding)

This repo conforms to the sahil87 toolkit standards. Read the canonical
standard — do not work from summaries or restatements.

- Enumerate: `shll standards` · read one: `shll standards <name>`
  (current set: `principles`, `help-dump`, `readme-extraction`, `skill`)
- Canonical source if `shll` is unavailable: the sahil87/shll repo's
  `docs/site/standards/` tree, rendered on <https://shll.ai>
- Before changing the CLI surface, help output, `README.md`, or `docs/site/`,
  check the change against the standards governing that surface
  (binding per `fab/project/constitution.md` § Toolkit Standards)

## Documentation map

- `docs/specs/index.md` — pre-implementation design intent (CLI surface,
  config resolution, architecture, build & release)
- `docs/memory/index.md` — post-implementation behavior; the authoritative
  source of truth for how hop actually works

## Working on this repo

- Source lives under `src/` (`src/cmd/hop`, `src/internal/*`); build with
  `just build`, test with `just test`
- Changes flow through the fab pipeline: `fab/project/constitution.md` holds
  the binding project principles; `/fab-*` skills live in `.claude/skills/`
