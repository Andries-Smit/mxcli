# Proposal: Performance & complexity anti-pattern detection

**Status:** Draft
**Date:** 2026-06-18

## Problem Statement

The catalog + graph analysis answers *structural* and *architectural* questions
(what references what, communities, god nodes, layering). A natural follow-up is
**performance and complexity anti-patterns** — the things a reviewer flags in a
go-live audit but that nothing automated catches today:

1. **Retrieves without an index** — XPath/database retrieves filtering on an
   unindexed attribute.
2. **Too much data in memory / non-persistent-entity (NPE) overuse** — pulling
   large lists into memory, or modelling with NPEs where it doesn't scale.
3. **Runtime processing that belongs in the database** — looping/aggregating in a
   microflow what an OQL query or a bulk DML / `EXECUTE DATABASE QUERY` would do
   set-based.
4. **Overly complex documents** — high cyclomatic complexity, oversized
   microflows, deep nesting, complexity hotspots.

These are primarily **lint rules** (a `PERF` series) plus a section in
`mxcli report` and the graph builtins, built on data the catalog already has —
or, where it doesn't, on two clearly-scoped additions.

## Feasibility per use case (what's ready vs. missing)

| # | Detection | Catalog data | Status |
|---|-----------|--------------|--------|
| 4 | **Overly complex** — `Complexity > N`, `ActivityCount > N`, plus centrality hotspots (`graph_god_nodes`) | `microflows.Complexity`, `microflows.ActivityCount`, `graph_*` | **Ready now** — packaging only |
| 2a | **NPE overuse** — count/ratio of non-persistable entities per module; NPEs with many attributes/associations | `entities.EntityType` (persistable vs non-persistable) | **Ready now** for counts; |
| 2b | **Too much in memory** — `retrieve` with no range/limit feeding a `loop`; large list ops | `activities` (loop/retrieve), `refs` (retrieve edges) | **Mostly ready** — needs activity-shape inspection |
| 3 | **Runtime → DB** — retrieve-then-loop-to-aggregate; nested loops doing a join; change-in-loop | `activities`, `refs` | **Mostly ready** — activity-pattern matching (some already in `patterns-data-processing` skill) |
| 1 | **Retrieve without index** — filter attribute(s) of a retrieve constraint not covered by an entity index | needs **(a)** entity indexes in the catalog and **(b)** which attribute(s) a retrieve constrains | **Needs two additions** (below) |

### Missing data — two scoped additions

- **Entity indexes** (for #1). The SDK already parses entity indexes (`ALTER
  ENTITY … ADD INDEX` exists), but they aren't cataloged. Add an `entity_indexes`
  table (Entity, IndexedAttributes) populated from the domain model. Small,
  self-contained.
- **Constraint → attribute resolution** (for #1, and sharpens #3). Knowing *which
  attribute* a `retrieve … where [Attr = …]` filters on requires resolving the
  XPath/expression. This is the **expression-edge frontier** already scoped in
  [`PROPOSAL_expression_type_checking.md`](PROPOSAL_expression_type_checking.md)
  (its `AttributeAccessExpr` → catalog resolution). So **#1 is gated behind that
  proposal**; until then, an approximate index check can run on the raw
  `xpath_expressions` table (string match of attribute names), flagged as
  best-effort.

## Delivery

A `PERF` lint-rule series (built-in Go rules + the metrics above), surfaced three
ways — all reading existing/added catalog data, no new BSON writes:

- **`mxcli lint`** — new rules, e.g.:
  - `PERF001` retrieve without index (best-effort until expression edges land)
  - `PERF002` unbounded retrieve feeding a loop
  - `PERF003` aggregate/count computed by looping in a microflow (suggest OQL/aggregate)
  - `PERF004` nested loops doing a list-join (O(N²); suggest a keyed retrieve)
  - `PERF005` change/commit inside a loop (suggest batch commit)
  - `QUAL/PERF` complexity: `Complexity > 10`, `ActivityCount > 25`
  - `PERF006` NPE overuse (ratio of non-persistable entities, large NPEs)
- **`mxcli report`** — a "Performance & complexity" category with the scored
  findings, alongside the existing categories.
- **Starlark builtins** — expose the raw facts (e.g. `entity_indexes()`,
  `complexity_of(asset)`, the existing activity/centrality builtins) so teams can
  tune thresholds to their own standards (mxcli ships sensible defaults, not a
  mandate — same philosophy as the graph builtins).

Many of these patterns are already described prose-only in the
`patterns-data-processing` and `assess-quality` skills; this proposal turns the
detectable ones into enforced rules.

## Dependencies & sequencing

1. **Ship-now tier** (no new data): complexity rules (#4), NPE-count rule (#2a),
   and the activity-pattern rules (#2b, #3, #5) that match `activities` shapes.
2. **Index tier**: add the `entity_indexes` catalog table → enables a best-effort
   `PERF001` against `xpath_expressions`.
3. **Precise tier**: once the expression type-checker lands, `PERF001`/`#3` become
   exact (real attribute resolution instead of string matching).

## Test Plan

- Rule unit tests with seeded catalog rows (the `mdl/linter/rules/*_test.go`
  pattern): a microflow with a loop+retrieve triggers `PERF002`; an indexed vs.
  unindexed constraint triggers/clears `PERF001`; a `Complexity` over threshold
  triggers the complexity rule.
- `mdl-examples/` scripts demonstrating each anti-pattern and its fix, runnable in
  Studio Pro.

## Open Questions

1. **Thresholds.** Defaults for complexity/activity/NPE-ratio — fixed, or
   config-driven via `.mxcli-lint.yaml`? Proposed: sensible defaults, overridable.
2. **`PERF001` before expression edges.** Ship the best-effort string-match index
   check now (with a clear "approximate" note), or wait for exact resolution?
   Proposed: ship best-effort, upgrade in place.
3. **Severity.** Performance findings as `warning` (not `error`) so they don't
   block CI while teams calibrate.
