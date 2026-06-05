# Intake: Fix relative-dir handling in `hop add` / `hop config scan`

**Change**: 260605-c92v-fix-relative-dir-args
**Created**: 2026-06-05
**Status**: Draft

## Origin

<!-- How was this change initiated? Include the user's raw input/prompt, the interaction
     mode (one-shot vs. conversational), and key decisions from the conversation. -->

> User reported: "I am unable to add a repo to hop config." Running `hop config add outbox`
> (and later `hop add fab-kit`) from `~/code/sahil87` produced:
>
> ```
> skip: fab-kit: cannot derive group name from parent dir '.'
> hop add: git@github.com:sahil87/fab-kit.git already registered in /home/sahil/.config/hop/hop.yaml. Nothing to add.
> ```

Conversational diagnosis during a `/fab-discuss` session. The reported symptom bundled
**three** distinct problems; investigation separated them:

1. **`fzf failed: exit status 1`** on `hop add outbox` — a **stale/partial shell shim** in the
   user's interactive session (`_hop_dispatch` reported `command not found`). **Not a code bug** —
   resolved by the user reloading the shim (`eval "$(hop shell-init zsh)"`). Out of scope here.
2. **`cannot derive group name from parent dir '.'`** — a **real bug** in relative-path handling.
3. **`already registered. Nothing to add.`** printed for a repo that was never registered — a
   **real bug**: a misleading message. The `fab-kit` run shows lines (2) and (3) *contradicting each
   other* in the same invocation, proving both are live.

This change fixes (2) and (3). The shim issue (1) and a stale `$HOP_CONFIG` header comment in the
generated `hop.yaml` (the env var was removed in #36) are explicitly out of scope.

## Why

**Problem (Bug A — relative paths break group derivation and convention matching).**
`validateConfigDir` (in `src/cmd/hop/config_scan.go`, shared by both `hop add <dir>` and
`hop config scan <dir>`) validates the argument via `filepath.Clean` → `filepath.EvalSymlinks` →
`os.Stat`, but **never converts the path to absolute**. `filepath.EvalSymlinks` preserves a relative
input as relative (e.g. `EvalSymlinks("fab-kit")` → `"fab-kit"`). The returned `canonicalDir` then
flows into `scan.ClassifyOne` / `scan.Walk`, so `Found.Path` is relative. Two downstream consumers
then break:

- `buildScanPlan` computes `parentDir := filepath.Dir(f.Path)` → `filepath.Dir("fab-kit")` = `"."`,
  then `filepath.Base(".")` = `"."`, which `slugifyGroupName` rejects → the `skip:
  cannot derive group name from parent dir '.'` line (`config_scan.go:250`).
- `matchesConvention` compares the relative `Found.Path` against the **absolute** convention path
  `<code_root>/<org>/<name>` (e.g. `/home/sahil/code/sahil87/fab-kit`). A relative path can never
  equal an absolute one, so a repo that *should* match convention (and land in the `default` group)
  is misrouted into the invented-group branch — which is where Bug A's slugify failure occurs.

**Problem (Bug B — "already registered" misreports skipped candidates).**
`runAdd` (in `src/cmd/hop/config_add.go`) calls `buildScanPlan`, then checks `planIsEmpty(plan)`.
An empty plan triggers the message `"%s: %s already registered in %s. Nothing to add."`. But a plan
is *also* empty when its only candidate was **skipped for a non-dedup reason** (e.g. the slugify
failure from Bug A). So a repo that was never in `hop.yaml` is reported as "already registered." The
`fab-kit` repro shows the contradiction directly: line 1 says the group name couldn't be derived,
line 2 says the URL is already registered — mutually exclusive.

**Consequence if unfixed.** `hop add <relative-dir>` and `hop config scan <relative-dir>` are
unusable from a parent directory using a bare repo name — the single most natural invocation
(`cd ~/code/sahil87 && hop add fab-kit`). The contradictory "already registered" message actively
misleads: the user believes the repo is registered when it is not.

**Why this approach.** Bug A is the root cause; the absolute-path resolution belongs in the shared
`validateConfigDir` helper so both `add` and `scan` are fixed in one place (consistent with
Constitution IV — wrap once, don't duplicate). Bug B is a latent correctness issue independent of A
(it would still misreport on any future slugify failure), so it is fixed at the same time by
distinguishing "candidate skipped" from "candidate deduped" in `runAdd`. Both surface from the same
user action, so a single `fix:` change keeps the work cohesive.

## What Changes

### 1. `validateConfigDir` resolves to an absolute path

**File**: `src/cmd/hop/config_scan.go` (`validateConfigDir`, ~line 148).

Make the validated directory absolute before returning it. The current sequence is
`filepath.Clean` → `filepath.EvalSymlinks` → `os.Stat`. Insert an absolutization step so a relative
argument becomes absolute. Preferred shape:

```go
func validateConfigDir(userArg, cmdName string, stderr io.Writer) (canonical string, ok bool) {
	abs, err := filepath.Abs(userArg)
	if err != nil {
		fmt.Fprintf(stderr, "%s: '%s' is not a directory.\n", cmdName, userArg)
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		fmt.Fprintf(stderr, "%s: '%s' is not a directory.\n", cmdName, userArg)
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "%s: '%s' is not a directory.\n", cmdName, userArg)
		return "", false
	}
	return resolved, true
}
```

Notes:
- `filepath.Abs` joins with the process CWD and runs `filepath.Clean`, so the explicit
  `filepath.Clean` call becomes redundant and is dropped.
- `EvalSymlinks` on an absolute path returns an absolute path (it already did for absolute inputs —
  that is why `hop add /home/sahil/code/sahil87/outbox` worked in testing). Absolute-arg behavior is
  unchanged.
- The error wording (`'%s' is not a directory.`) and `userArg`-verbatim echo are preserved.
- The spec doc comment on `validateConfigDir` ("filepath.Clean → filepath.EvalSymlinks → os.Stat")
  must be updated to reflect "filepath.Abs → filepath.EvalSymlinks → os.Stat".

**Effect on `add`**: `Found.Path` is now absolute, so `filepath.Dir` yields a real parent
(`/home/sahil/code/sahil87`), `filepath.Base` yields `sahil87`, and convention matching works — a
`sahil87/<name>` repo under `code_root: ~/code` correctly lands in the `default` group instead of an
invented group.

**Effect on `scan`**: identical fix path. `hop config scan .` (or any relative root) now walks
absolute paths, so discovered repos get correct group derivation and convention matching.

### 2. `runAdd` only reports "already registered" on a genuine duplicate

**File**: `src/cmd/hop/config_add.go` (`runAdd`, ~lines 120–128).

Today `runAdd` builds a one-element plan and treats an empty plan as "already registered." After
Bug A is fixed, the slugify path is far less likely to fire — but the message is still incorrect for
*any* skip-to-empty-plan case. Make the dedup determination explicit rather than inferred from plan
emptiness.

Approach: after classifying the single repo and confirming `isRepo`, check whether `found.URL`
already exists in the loaded config (`cfg`) **before** building the plan. Only that condition
warrants the "already registered" message. If the URL is not present yet but the plan still comes
back empty, that means the candidate was skipped for another reason (e.g. a genuine slugify failure
on a pathological parent dir) — surface a skip/no-op message that does **not** claim prior
registration.

```go
// 5. Idempotency: report "already registered" ONLY when the URL is a genuine
//    duplicate of an existing entry — not merely because the plan is empty.
if urlAlreadyRegistered(cfg, found.URL) {
	fmt.Fprintf(stderr, "%s: %s already registered in %s. Nothing to add.\n", cmdName, found.URL, configPath)
	return nil
}

// 6. Build a one-element scan plan (convention → default; else invented).
plan, _ := buildScanPlan(cfg, []scan.Found{found}, stderr)

// 7. If the plan is still empty here, the candidate was skipped for a non-dedup
//    reason (buildScanPlan already emitted a `skip:` line). Do not claim it was
//    already registered.
if planIsEmpty(plan) {
	fmt.Fprintf(stderr, "%s: '%s' could not be registered (see skip above). Nothing to add.\n", cmdName, userArg)
	return nil
}
```

`urlAlreadyRegistered(cfg, url)` is a small helper that scans `cfg.Groups[*].URLs` for an exact URL
match (the same set `buildScanPlan` builds into `existingURLs`). Exact-match semantics — no URL
normalization — matching `buildScanPlan`'s existing dedup behavior so the two stay consistent.

> Design note considered and rejected: threading a structured skip-reason out of `buildScanPlan`
> back into `runAdd`. That is a larger refactor of the scan-plan return contract and is not needed —
> a pre-plan dedup check plus a corrected fallback message resolves the user-visible defect with
> minimal surface change. Recorded as a Tentative assumption for `/fab-clarify` review.

### 3. Regression tests

**File**: `src/cmd/hop/config_add_test.go` (and/or `config_scan_test.go` for the shared helper).

- **Relative-arg add**: from a known CWD, `hop add <relative-name>` for a convention-layout repo not
  yet in config writes the URL into the `default` group (no "cannot derive group name", no false
  "already registered"). Drives the absolute-path fix.
- **Skip-vs-dup message**: assert that a repo *not* in config that fails to register does **not**
  print "already registered"; and that a repo *already* in config **does** print "already
  registered." Drives Bug B.
- **Absolute-arg parity**: existing absolute-path behavior is unchanged (regression guard).
- Tests use the existing injectable `gitRunner` / `scan.Options{GitRunner: ...}` seam and a temp
  config — no real `git` or real `~/.config/hop/hop.yaml` writes (Constitution: No Database; tests
  must not touch the user's live config).

## Affected Memory

<!-- Implementation-only bug fix. No spec-level behavior changes to memory. -->

- `cli/subcommands`: (modify) — only if the relative-vs-absolute argument behavior of
  `hop add` / `hop config scan` is documented there; update the description of how `<dir>` is
  resolved. Confirm during hydrate; likely no change needed (the documented contract is
  "register/scan a directory" — relative now works as expected rather than as a new behavior).

## Impact

- **`src/cmd/hop/config_scan.go`** — `validateConfigDir` (shared by `add` + `scan`); doc comment.
- **`src/cmd/hop/config_add.go`** — `runAdd` dedup/skip branch; new `urlAlreadyRegistered` helper.
- **`src/cmd/hop/config_add_test.go`**, **`src/cmd/hop/config_scan_test.go`** — regression tests.
- **No** changes to `internal/scan`, `internal/config`, `internal/yamled` — the bug is entirely in
  the CLI layer's path handling and message logic.
- **No** new flags, env vars, or subcommands (Constitution III / VI satisfied).
- **No** dependency, schema, or YAML-format changes — fully backward compatible. Absolute-path
  invocations behave identically; relative-path invocations now work instead of failing.

## Open Questions

- ~~Exact wording of the new "could not be registered" fallback message in `runAdd` (Bug B).~~
  <!-- clarified: user confirmed `'<dir>' could not be registered (see skip above). Nothing to add.` -->
- Whether `cli/subcommands` memory actually documents argument resolution (confirm at hydrate).

## Clarifications

### Session 2026-06-05 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 7 | Confirmed | Lighter approach: pre-plan dedup check + corrected fallback message |
| 8 | Confirmed | Message wording: `'<dir>' could not be registered (see skip above). Nothing to add.` |
| 3 | Confirmed | — |
| 4 | Confirmed | — |
| 5 | Confirmed | — |
| 6 | Confirmed | — |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Root cause is `validateConfigDir` returning a relative path; fix is `filepath.Abs` before `EvalSymlinks` in the shared helper | Verified in-session: relative arg `outbox`/`fab-kit` fails with `'.'`, absolute arg `/home/.../outbox` succeeds (`added`). Diagnosed against source. | S:95 R:80 A:95 D:90 |
| 2 | Certain | Fix belongs in shared `validateConfigDir` so both `add` and `scan` are corrected once | Both commands call the same helper (`config_scan.go:54`, `config_add.go:72`); Constitution IV (wrap once, don't duplicate). | S:90 R:80 A:95 D:90 |
| 3 | Certain | Bug B (`planIsEmpty` → "already registered") is a distinct, real defect fixed in the same change | Clarified — user confirmed. `fab-kit` repro shows the message printed for an unregistered repo, contradicting the preceding skip line. | S:95 R:75 A:85 D:75 |
| 4 | Certain | Out of scope: the `fzf` shim failure (stale `_hop_dispatch` in the user's session) and the stale `$HOP_CONFIG` header comment | Clarified — user confirmed. Shim is an environment issue (not code); user already fixed it by reloading. Header comment is an unrelated doc nit from #36. | S:95 R:90 A:80 D:80 |
| 5 | Certain | `urlAlreadyRegistered` uses exact URL match (no normalization) | Clarified — user confirmed. Mirrors `buildScanPlan`'s existing `existingURLs` dedup semantics so add and scan stay consistent. | S:95 R:70 A:85 D:75 |
| 6 | Certain | Change type is `fix` | Clarified — user confirmed. Bug fix to existing behavior; no new capability. | S:95 R:95 A:95 D:95 |
| 7 | Certain | Fix Bug B via a pre-plan dedup check + corrected fallback message, rather than threading a structured skip-reason out of `buildScanPlan` | Clarified — user confirmed. Smaller surface; avoids refactoring the scan-plan return contract. | S:95 R:55 A:65 D:50 |
| 8 | Certain | Fallback message wording: `'<dir>' could not be registered (see skip above). Nothing to add.` | Clarified — user confirmed the proposed default. | S:95 R:90 A:55 D:55 |

8 assumptions (8 certain, 0 confident, 0 tentative, 0 unresolved).
