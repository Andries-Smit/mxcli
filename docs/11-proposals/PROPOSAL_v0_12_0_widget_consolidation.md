# v0.12.0 Implementation Plan: Widget Path Consolidation

**Status:** Draft
**Milestone:** [v0.12.0](https://github.com/mendixlabs/mxcli/milestone/5)
**Builds on:** [UNIFIED_SCHEMA_REGISTRY.md](UNIFIED_SCHEMA_REGISTRY.md) (Phase 4 — Native/pluggable dispatch)
**Closes (in part):** #529 (Phase 4), #574, #541, #566, #568, #569, #570

## Goal

One source of truth for widget BSON, derived from each project's installed
`.mpk` files, used by every widget builder. After v0.12.0:

- A single MDL script targeting DataGrid produces BSON that matches the
  project's installed `.mpk` version exactly — no per-Mendix-version
  embedded snapshots, no hand-coded BSON patches.
- The `datagrid`, `gallery`, `combobox` keyword forms and the
  `pluggablewidget '<id>'` form route through the same engine and produce
  byte-equivalent BSON.
- `mdl/backend/mpr/datagrid_builder.go` (1467 lines) and the
  `sdk/widgets/templates/mendix-11.6/*.json` snapshots are deleted.

## Findings from investigation that shape the plan

Two architectural facts uncovered while scoping this milestone:

### 1. The engine already builds from `.mpk` without baselines

`mdl/executor/widget_engine.go` constructs full BSON for 24 marketplace
widgets (Accordion, Badge, Maps, PopupMenu, Tooltip, Timeline, AreaChart,
etc.) that have no entry under `sdk/widgets/templates/mendix-11.6/`. The
envelope conventions (`AllowUpload`, `Properties` ordering, `ClientTemplate`
translation defaults) are encoded in Go in `serializeWidgetValueForRawType`
and `widget_engine.go`'s child-slot / object-list dispatch. The 9 embedded
JSON templates exist only because the keyword path (`datagrid_builder.go`)
predates the engine and clones from a hand-curated snapshot.

**Implication**: We don't need per-Mendix-version embedded baselines. The
correct path is to migrate the keyword forms to the engine, then delete
the embedded snapshots.

### 2. `.mpk` files carry no migration logic

Confirmed against DataGrid 3.4.0, VideoPlayer, AreaChart, TimeSeries,
PieChart. The XML is declarative current-version state only — no
`<rename>`, `<deprecated>`, `<since>`, `<obsoletes>` elements. The
`editorConfig.js` exports exactly `check`, `getCustomCaption`, `getPreview`,
`getProperties` — no `migrate` / `onUpgrade` / `version` hook.

Studio Pro's "Update widget" is therefore mostly mechanical: drop
properties no longer in the schema, fill defaults for newly-added ones.
Genuine renames/restructures would have to be hardcoded in the modeler
binary (we have no evidence of this; if it exists, it's invisible from
mxcli's vantage).

**Implication**: `sdk/widgets/augment.go` already handles the additive
and subtractive cases. The remaining "rename / restructure" case is rare,
can't be auto-detected from the `.mpk`, and is everyone's problem (Studio
Pro can't see it either). mxcli's job is to detect drift and report —
not to perform structural migration.

## Strategy

The right ordering is **engine consolidation first**, then version
awareness only where it's actually needed.

```
Stream B (consolidation) ─── first ───┐
                                       │
Stream A (per-version envelope) ─ after ─┴─→ delete embedded snapshots
                                       │
Stream C (issue-queue cleanup) ─ parallel ─┘
```

### Stream B — Engine feature parity with the keyword path

**B1a. `headerSlots[]` in `.def.json`**
Engine routes child keywords (`CONTROLBAR`) to a named parent property
(`filtersPlaceholder` for DataGrid, `filters` for Gallery). New
`.def.json` field; existing files unaffected.

**B1b. Object-list item with child widgets → `customContent` slot**
When an item in an object list (e.g. DataGrid `column`) has nested child
widgets, the engine routes them to the item's `customContent` /
`template` sub-property. Eliminates `DataGridColumnSpec.ChildWidgets` as
a special case.

**B1c. Per-column filter slot**
The engine recognizes a nested `filter { textfilter … }` block inside an
object-list item as filling that item's `filter` sub-property.
Eliminates `DataGridColumnSpec.FilterWidget` as a special case.

**B1d. Caption/Content `texttemplate` placeholders**
Extract `pageBuilder.buildClientTemplateParams` (currently in
`cmd_pages_builder_v3_widgets.go`) into a reusable engine helper, wired
through the `texttemplate` operation so any pluggable widget's
`Caption: '{1}'` with `CaptionParams: [{1} = attr]` resolves correctly.

**B2. Switch dispatch**
`cmd_pages_builder_v3.go:271` — `datagrid`, `gallery`, `combobox` cases
route to `pluggableEngine.Build(def, w)`. Same dispatch path as the
marketplace widgets.

**B3. Delete the keyword-specific code**
- `mdl/backend/mpr/datagrid_builder.go` (1467 lines)
- `sdk/widgets/templates/mendix-11.6/*.json`
- `sdk/widgets/loader.go` template-loading paths (simplified to a thin
  wrapper over `.def.json` + `.mpk`)

### Stream A — Per-Mendix-version envelope conditionals

Only after Stream B is the engine the single point that needs version
awareness. Today `serializeWidgetValueForRawType` hardcodes 11.9
conventions.

**A1. Thread `MendixVersion` through the engine**
Already available via project metadata; pass it into the serializer.

**A2. Conditionalize envelope fields where they differ across versions**
Examples (from `WIDGET_BSON_VERSION_COMPATIBILITY.md`):
- `AllowUpload` exists in 11.9+ only — gate emission on version
- Filter widget envelope (`Forms$Appearance`, etc.) shape evolved
- `TextTemplate` default `Translations` conventions

Each becomes a conditional in Go rather than a per-version JSON snapshot.

**A3. Validation gate**
Doctype fixtures pass `mx check` on **both** Mendix 11.9 and 11.10 with
zero CE0463. Use `mx-test-projects/test5-app` (CE0463 reference) and a
fresh 11.10 project for the matrix.

### Stream C — Issue-queue cleanups (parallel with B)

- **#574** — VideoPlayer + Timeline TextTemplate visibility. Phase-1 fix:
  hand-author `propertyVisibility[]` in their two `.def.json` files;
  engine nulls/populates TextTemplate based on current property values.
  The JS extractor is deferred to v0.13+ (only worth it if more widgets
  hit the same pattern).

- **#541** — Gallery CE0463 on filter/textfilter combination. Likely
  free after Stream B since the engine handles Gallery the same way it
  handles DataGrid. Validated in A3.

- **#566** — `MENUTRIGGER` grammar keyword. Small ANTLR change in
  `MDLPage.g4 widgetTypeV3`. Unblocks PopupMenu test cases currently
  commented out with TODOs in `30-pluggable-widget-examples.test.mdl`.

- **#568** — Version-aware widget property gating. Reads the `.mpk`
  version from `.def.json` (already extracted) and refuses writes to
  properties absent in that version. Reuses A1's `MendixVersion`
  threading.

- **#569** — `mxcli syntax` see-also link to `schema show <widget>`.
  Trivial after Stream B exposes the engine catalog tables.

- **#570** — Classify widget BSON drift after `.mpk` upgrade. Implements
  the three-bucket framework (additive / subtractive / rename) from the
  finding above. Outputs an actionable `mxcli check --post-widget-upgrade`
  report. Builds on `augment.go`'s existing diff logic.

## Critical path

```
B1a (CONTROLBAR / headerSlots) ─→ B1b ─→ B1c ─→ B1d ─→ B2 ─→ B3
                                                          │
                                                          ↓
                                                         A1 ─→ A2 ─→ A3
                                                          │
        C1 (#574) ─→ C2 (#541) ─────────────────────────┤
        C3 (#566 grammar) ───────────────────────────────┤
        C4 (#568) C5 (#569) C6 (#570) ──── after A1 ─────┘
```

A3 is the milestone's acceptance gate: doctype fixtures pass `mx check`
zero-CE0463 on 11.9 + 11.10.

## Smallest first PR

**B1a-DataGrid only**: implement `headerSlots[]` in `.def.json` for the
DataGrid widget only, and route CONTROLBAR through it in the engine.
Verify the engine produces byte-equivalent BSON to `datagrid_builder.go`
for `mdl-examples/doctype-tests/31-pluggable-datagrid-gallery-v010-examples.mdl`
PD02 (the controlbar test case).

Acceptance for this PR:
1. Both forms produce identical BSON for PD02 (use
   `mxcli bson compare`).
2. `mx check` reports zero errors against both outputs.
3. No regressions in the existing doctype fixture suite.

That single PR is the dry run for the engine-can-do-everything claim.
After it lands, B1b-d and B2 are mechanical extensions of the same
pattern.

## Trade-offs and decisions

| Question | Decision | Rationale |
|---|---|---|
| Do we need per-version embedded templates? | No | The engine builds envelopes in Go; per-version differences become conditionals in `serializeWidgetValueForRawType`. Embedded JSON snapshots become tech debt to delete. |
| Source of truth for widget properties? | Each project's installed `.mpk` | `widget init` already extracts to `.def.json`. The proposal's "use BSON template extracted from the project" is satisfied today for the engine path; just needs the keyword path migrated. |
| How to handle widget version migrations? | Re-extract; trust Studio Pro's "Update widget" for non-mechanical cases | `.mpk` carries no migration logic. The additive/subtractive case is automatic via `augment.go`. Renames are detect-and-report only (#570). |
| Stream A or Stream B first? | Stream B | Engine already handles marketplace widgets without baselines; consolidating the keyword path into the engine is the actual gap. Per-version envelope conditionals (Stream A) are smaller and bound by Stream B's footprint. |
| Path 1 (per-version baselines) vs Path 2 (per-project capture)? | Neither | Earlier framings — both turned out unnecessary given the engine's existing capability. |

## Non-goals for v0.12.0

- **Platform schema codegen** (UNIFIED_SCHEMA_REGISTRY Phase 4). Out of
  scope here; the registry runtime for *widget* schemas is sufficient
  for the consolidation goal.
- **Generic JS extractor for conditional visibility** (#574 Phase 2).
  Hand-authored `propertyVisibility[]` for the two known cases is
  enough for v0.12.0; the extractor lands when the population of
  hand-authored entries justifies the tooling cost.
- **Cross-version rename / restructure migration**. Studio Pro can't do
  this from the `.mpk` either; mxcli will detect and report drift only.
- **Pages and security expressions in `mxcli check`**. Tracked under
  v0.13.0 (#580).

## References

- [UNIFIED_SCHEMA_REGISTRY.md](UNIFIED_SCHEMA_REGISTRY.md) — umbrella
  proposal; this plan implements Phase 4 specifically.
- [WIDGET_BSON_VERSION_COMPATIBILITY.md](../03-development/WIDGET_BSON_VERSION_COMPATIBILITY.md)
  — five-fix patch ledger that motivated this consolidation.
- [PROPOSAL_widget_property_visibility.md](PROPOSAL_widget_property_visibility.md)
  — #574 design; Phase 1 lands in v0.12.0, Phase 2 deferred.
- #64 — `pluggablewidget` engine-path `AttributeRef` bug, the concrete
  motivating case for consolidation.
- #578 — `datagrid` keyword-path column TextTemplate bug, demonstrating
  that the two paths drift independently and need different fixes today.
