---
title: Why MDL is SQL-shaped
category: rationale
last-synced: 4e185f73
sources:
  - .claude/skills/design-mdl-syntax.md
  - docs/13-decisions/0003-mdl-is-sql-shaped.md
  - docs/11-proposals/PROPOSAL_mdl_syntax_design_guidelines.md
  - docs/01-project/MDL_QUICK_REFERENCE.md
---

> **Do not duplicate**: the syntax design checklist (see the `design-mdl-syntax` skill), specific statement syntax (see `MDL_QUICK_REFERENCE`), or the full alternatives-and-consequences record. [ADR-0003](../../docs/13-decisions/0003-mdl-is-sql-shaped.md) is the immutable decision; this page is mutable synthesis that points back to it.

## What this is

MDL deliberately looks like SQL: standard `CREATE` / `ALTER` / `DROP` / `SHOW` / `DESCRIBE` verbs, qualified `Module.Element` names everywhere, `( Key: value, ... )` property lists, keywords instead of symbols. The shape is not an accident of taste — it is the chosen design tradition for a language whose primary audience is citizen developers, not software engineers.

## How it fits

Mendix's core users are business analysts and citizen developers, who reject syntax that reads as cryptic or mathematical. The decisive insight is that this is a *solved* design problem with a long lineage. SQL was deliberately shaped for business analysts — SEQUEL stood for "Structured English Query Language" — and BASIC was shaped for learners. Both traded concision for readability and won. MDL inherits that goal directly: statements you can read aloud and reason about.

The microflow control-flow constructs (`loop`, `if`, `retrieve where`) are not a strain against the SQL shape — they follow the well-trodden PL/SQL pattern, where SQL is wrapped in an imperative shell. PL/SQL, T-SQL, and PL/pgSQL have stress-tested that composition for decades for exactly this audience.

The central trade-off is **verbosity**: SQL-shape produces longer statements and higher token counts than a JSON DSL or a symbolic Go fluent API would. That cost is accepted on purpose — concision is sacrificed for readability and reviewable one-line diffs. A welcome side effect is LLM fitness: SQL is heavily represented in training corpora, so one example usually generalises to variants. Alternatives (JSON, YAML, Go-only, custom or schema DSLs) were weighed and rejected; see [ADR-0003](../../docs/13-decisions/0003-mdl-is-sql-shaped.md) for the authoritative record.

## See also

- [ADR-0003: MDL is SQL-shaped](../../docs/13-decisions/0003-mdl-is-sql-shaped.md) — the immutable decision record
- [[architecture/mdl-execution]] — how an MDL statement reaches storage
- [[positioning/vs-typescript-sdk]] — why a DSL instead of a code-only SDK
