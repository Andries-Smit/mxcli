# Proposal: Multi-Version Pluggable Widget Support

**Status:** Draft
**Date:** 2026-06-12

## Problem Statement

mxcli must create pluggable-widget instances (ComboBox, DataGrid2, Gallery, …)
that Studio Pro accepts **without `CE0463` "the definition of this widget has
changed"** — for whatever Mendix version *and* whatever installed widget version
a project is on.

### How widget creation actually works today (both engines)

Resolution is **three layers**, not a single frozen template:

1. **`def.json` / `WidgetRegistry`** — tells the engine *which MDL keyword routes
   into which widget property key* (+ object-list / child-slot structure). Built-ins
   (ComboBox/DataGrid/Gallery/filters) are hand-crafted in
   `sdk/widgets/definitions/*.def.json`; everything else is **extracted from the
   project's `.mpk`** by `mxcli widget init` → `.mxcli/widgets/<name>.def.json`
   (`RefreshWidgetDefinitions`, auto-refreshes on drift). Derived from the project.
2. **`GetTemplateFullBSON`** — loads the static `templates/mendix-11.6/*.json` as the
   Type+Object **base skeleton**.
3. **`augmentFromMPK`** (`AugmentTemplate`) — reconciles that skeleton's **property
   set against the installed `.mpk`**: adds keys in the `.mpk` but missing from the
   template, removes stale ones, emitting *both* a `PropertyType` (Type) and a
   `WidgetProperty` (Object) per added key.

**The schema is already version-reconciled.** Measured: a ComboBox created by the
legacy engine on a **10.24** project (installed ComboBox `2.4.3`, base template
`11.6`) has **56 `PropertyKey`s — identical** to a pristine Studio-Pro 2.4.3
ComboBox (0 stale, 0 missing), and produces **no `CE0463`**. So the long-standing
"frozen 11.6 templates cause `CE0463` on 10.x" belief (encoded in
`sdk/versions/*.yaml` and `WIDGET_BSON_VERSION_COMPATIBILITY.md`) is **not the
schema** — `augmentFromMPK` handles it.

### So what actually breaks?

Two real, narrow gaps remain:

- **Dirty Object defaults.** `augmentFromMPK` reconciles property *keys*; it does
  **not** clean a template's Object *default values*. The modelsdk engine's
  `CE0463` this cycle came from a `combobox.json` extracted from a *configured*
  Studio-Pro instance (a `System.Language` association ComboBox), so its neutral
  defaults carried `optionsSourceType:"association"` + a baked-in datasource. The
  builder applied `attribute: Country` on top without resetting them → an Object
  inconsistent with the schema → `CE0463`. The fix that worked (commit `827bffd4b`)
  was simply swapping in the legacy template's **clean/neutral** Object defaults.
- **No cross-version guarantee.** Nothing tests that creation stays `CE0463`-free
  as Mendix minors and widget versions move; regressions surface in the field.

This proposal is **internal architecture only** — no new MDL syntax. It hardens the
existing three-layer mechanism rather than replacing it.

## BSON Investigation: the CE0463 tolerance spike

We measured **what `CE0463` actually checks**, because the prevailing assumption
(in `WIDGET_BSON_VERSION_COMPATIBILITY.md`) was wrong.

**Method.** Decode a real widget unit (`pymongo`), mutate one dimension, re-encode,
`mx check`. Every test has a no-op round-trip **control** proving the decode→encode
is byte-faithful. mxbuild **10.24.19** (`test-1024`, loads clean) + **11.10.0**
(`test6-app`). Confirmed across **ComboBox, DataGrid2, Gallery**.

| Mutation | Layer | Result |
|---|---|---|
| Add `AllowUpload` to all `WidgetValueType` (10.24, 504×) | envelope field | **tolerated** — 0 errors |
| Remove `AllowUpload` (11.10, 522×) | envelope field | **tolerated** — no new CE0463 |
| Reverse `WidgetObject.Properties` order (all 3 widgets) | Object ordering | **tolerated** — 0 CE0463 |
| Rename one `WidgetPropertyType.PropertyKey` (all 3) | **schema** | **CE0463** (18 DataGrid2, 15 Gallery) |
| Object carries instance-specific values inconsistent with schema | **Object↔schema** | **CE0463** (the modelsdk dirty-defaults case) |

**Conclusion.** `CE0463` is triggered by **schema mismatch** (embedded
`CustomWidgetType` `PropertyKey` set ≠ installed widget) and by **Object↔schema
inconsistency**. It is **NOT** triggered by envelope field presence/absence
(`AllowUpload`) or `Properties` ordering — Studio Pro tolerates those. The doc's
11.9 `AllowUpload`/ordering "envelope fragility" fixes worked because they were
*holistic template re-extractions*, not because those fields matter.

### Two independent version axes

| Axis | Source | Drives | Handled by |
|---|---|---|---|
| **Widget version** (ComboBox `2.4.3`@10.24 vs `2.5.0`@11.10) | installed `.mpk` | **schema** (`PropertyKeys`) — *what CE0463 checks* | `augmentFromMPK` reconciles it ✓ (proven 56=56) |
| **Mendix version** | runtime infra | **envelope** (`WidgetValueType` field set, ordering) | tolerated — no action needed |

They move independently (update Mendix without widgets, or a widget `.mpk` without
the project's existing instances → Studio Pro shows `CE0463` on those stale
instances = the normal "Update widget" prompt). The authoritative schema for a
*newly created* instance is the **currently installed `.mpk`** — which augment
already uses.

## Proposed Direction

The existing `def.json`-routing + static-base + `augmentFromMPK` pipeline already
delivers version-correct **schemas**. So the work is **hardening**, not
replacement:

1. **Guarantee clean, neutral Object defaults.** The template's Object must be a
   *fresh, unconfigured* widget's defaults (empty refs/datasources, type-default
   primitives) — never lifted from a configured instance. Either (a) re-extract all
   embedded templates from freshly-dropped Studio-Pro widgets, or, more robustly,
   (b) **synthesize the neutral Object from the (augmented) Type** so it is
   correct-by-construction and instance-bleed is impossible. This directly removes
   the only `CE0463` class we actually hit.
2. **Ensure both engines augment.** legacy and modelsdk both call `augmentFromMPK`;
   keep them on the same code path (consolidate the two `*/widgets` loaders) so
   schema reconciliation can't silently diverge.
3. **Do *not* build a per-Mendix-version envelope model.** The spike shows the
   envelope is tolerated; the current superset (`augment.go` defaults incl.
   `AllowUpload`) is sufficient.
4. **Cross-version validation matrix.** Per installed mxbuild (10.24 / 11.9 / 11.10
   / 11.11): create one of each pluggable widget on a fresh project, assert
   `mx check` reports **0 `CE0463`**. Converts version drift into a failing test.

### `GenerateFromMPK` as an evaluated alternative (not the spine)

A pure ".mpk → Type+Object, no static base" path (`GenerateFromMPK`) is attractive
(no embedded templates to maintain) but is **largely redundant** with the existing
augment, and engalar's note that it produces "subtly different BSON" is an unquantified
risk. Recommend evaluating it *after* (1)–(4) land: if a `GenerateFromMPK` Type is
structurally equal to an augmented one for the same widget+version, the static base
can eventually be retired. Until proven, static-base + augment stays.

### Why the other alternatives are rejected

| Alternative | Why rejected |
|---|---|
| Extract-from-instance (engalar's `extract.go`) | Lifts a real instance's Object → the dirty-defaults `CE0463` we reproduced; also inherits stale schema if instances lag the `.mpk`. |
| Per-Mendix-version envelope model | Spike shows the envelope is tolerated — effort on a non-problem. |
| Bulk-sync legacy templates into modelsdk | A point fix for one symptom (dirty defaults); doesn't address neutral-Object guarantee or validation. (The committed `827bffd4b` band-aid is exactly this for ComboBox.) |

## Implementation Plan

Behind the existing `backend.WidgetObjectBuilder` interface — no executor or MDL
changes. Applies to both engines.

| File | Change |
|------|--------|
| `modelsdk/widgets/generate.go` (+ `sdk/widgets`) | Neutral-Object synthesis from the augmented Type (default `WidgetValue` per `PropertyType`); make it the Object source instead of the template's stored Object |
| `modelsdk/widgets/templates/`, `sdk/widgets/templates/` | Re-extract or demote; once synthesis lands, the stored Object becomes redundant |
| `modelsdk/widgets/loader.go`, `sdk/widgets/loader.go` | Consolidate the two loaders so both engines share one augment path |
| `sdk/widgets/augment.go` / `modelsdk/widgets/augment.go` | Reconcile to a single implementation |
| `mdl/backend/widgetobj/builder.go` | Ensure the builder fully overrides every neutral-Object slot it sets (no bleed-through) |
| `modelsdk/widgets/multiversion_test.go` (new) | Cross-version `mx check` matrix |
| `docs/03-development/WIDGET_BSON_VERSION_COMPATIBILITY.md`, `sdk/versions/*.yaml` | Correct the "envelope fragile / frozen schema" framing |

### Phasing

1. **Neutral-Object synthesis** for ComboBox → `mx check` clean on 10.24 + 11.10
   (removes the dirty-defaults class). De-risks the core fix.
2. Extend synthesis to object-list widgets (DataGrid2 columns, Gallery items).
3. Consolidate the two engines' widget loaders / augment onto one path.
4. Cross-version validation matrix; then evaluate retiring the static base via
   `GenerateFromMPK`.

## Version Compatibility

Not a version-gated feature — it is the mechanism that keeps pluggable widgets
correct across versions, and its deliverable is the validation matrix above. The
schema axis is already handled by `augmentFromMPK`; this proposal protects the
Object axis and proves the whole thing per Mendix minor.

Non-pluggable (`Forms$`) widgets need **no work** — already version-resilient via
the codec's declarative `VersionInfos` metadata (reflection-data); multi-version =
regenerate gen from the target version's reflection-data.

## Test Plan

- **Tolerance regression**: unit tests asserting `PropertyKey` rename → `CE0463`
  vs envelope field/ordering → tolerated (lock in the spike; catch any future
  dependence on envelope exactness).
- **Neutral-Object**: assert a synthesized Object has no instance-specific values
  (no populated `AttributeRef`/`DataSource`, type-default primitives).
- **Per-version matrix**: create ComboBox/DataGrid2/Gallery on fresh 10.24 / 11.9 /
  11.10 / 11.11 projects, `mx check`, assert 0 `CE0463`.
- **Augment fidelity** (lock in the measured result): augmented Type `PropertyKey`
  set == pristine Studio-Pro Type for the same widget+version (56=56 for ComboBox
  2.4.3 today).

## Open Questions

1. **Object-default version sensitivity.** Schema is version-reconciled and the
   envelope is tolerated — is the neutral *Object* ever version-sensitive in a way
   `augment` + synthesis don't cover? The matrix would catch it; flag if found.
2. **`GenerateFromMPK` fidelity.** Is its Type structurally equal to an augmented
   one (then the static base can retire), or does it drift on schema (must stay
   augment-based)? Evaluate in Phase 4.
3. **Generalization.** Spike + augment-fidelity covered ComboBox/DataGrid2/Gallery.
   Confirm on nested-CustomWidget widgets (charts, Maps) and on 12.x when available.
4. **Doc + version-YAML correction**, and an **ADR** for "installed `.mpk` (via
   augment) is the authoritative widget schema source" once Phase 1 lands.

## Relationship to existing artifacts

- Memory: `reference_ce0463_tolerance_spike.md` (tolerance + augment-boundary
  evidence), `reference_modelsdk_pluggable_widgets.md` (engine/registry history).
- Corrects `docs/03-development/WIDGET_BSON_VERSION_COMPATIBILITY.md` (on the
  trigger) and the `pluggable_widgets` notes in `sdk/versions/*.yaml`.
- No overlap with the property-*editing* widget proposals
  (`PROPOSAL_update_builtin_widget_properties`, `PROPOSAL_widget_property_visibility`,
  `PROPOSAL_v0_12_0_widget_consolidation`).
