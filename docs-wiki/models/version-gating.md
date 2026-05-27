---
title: Version Gating
category: mental-model
last-synced: 4e185f73
sources:
  - sdk/versions/registry.go
  - sdk/versions/mendix-11.yaml
  - mdl/executor/cmd_features.go
  - .claude/skills/version-awareness.md
---

> **Do not duplicate**: the procedure for adding a feature gate (see `.claude/skills/version-awareness.md`), the per-feature `min_version` table (read the YAML files), or specific Mendix release-note diffs (link out to Mendix docs).

## What this is

Not every MDL capability works in every Mendix version. mxcli encodes "which feature is available from which version" in a single declarative source of truth — a feature registry of YAML files keyed by `area.name` with a `min_version` (and optional `max_version` and `workaround`). The executor consults this registry before it writes anything, so a project never receives BSON its Studio Pro can't open.

## How it fits

The registry exists so version knowledge lives in **data, not scattered conditionals**. One YAML per major version (`mendix-9`, `mendix-10`, `mendix-11`) is embedded into the binary and flattened into a queryable index. `IsAvailable(area, name, projectVersion)` answers the only question that matters at write time.

`checkFeature()` is the enforcement contract. A handler calls it before any BSON mutation; if the connected project's version is below the feature's `min_version`, it returns an actionable error naming the requirement and a hint, and the write never happens. This makes the gate fail *early and loudly* rather than producing a file that breaks on open. Two deliberate escape hatches keep it from over-blocking: if no project is connected, or if the registry fails to load, the check passes — gating is a guardrail for real writes, not a hard dependency.

The same registry powers discovery (`show features`, `show features added since x.y`) so an author can check availability *before* writing, and tests pin examples to a floor with `-- @version:` directives.

When does a feature need a gate? Only when it depends on a metamodel construct or BSON shape introduced in a specific release. A feature that emits structure valid across all supported versions stays ungated — adding a needless entry just creates maintenance drag. The test is concrete: would an older Studio Pro reject the output? If yes, gate it (and ideally supply a `workaround`).

## See also

- [../../.claude/skills/version-awareness.md](../../.claude/skills/version-awareness.md) — how to check versions and the add-a-gate procedure
- [../../sdk/versions/registry.go](../../sdk/versions/registry.go) — registry loading, `IsAvailable`, semver comparison
- [../../mdl/executor/cmd_features.go](../../mdl/executor/cmd_features.go) — `checkFeature()` pre-check and `show features`
- [[architecture/mdl-execution]] — where feature checks sit in the execution pipeline
