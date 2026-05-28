# Widget BSON Version Compatibility

How mxcli's pluggable-widget BSON output stays in sync (or doesn't) with
specific Mendix versions, and how to extend support to a new minor release.

## Two-layer model

mxcli's widget BSON output is assembled from two sources with very different
version-resilience characteristics:

```
┌────────────────────────────────────────────────────────────────────┐
│                       Widget BSON output                           │
│                                                                    │
│   ┌─────────────────────────────────┐                              │
│   │ Widget-specific structure       │  ← project's .mpk file       │
│   │ (PropertyKeys, sub-properties,  │  version-tracked per widget  │
│   │  attribute types, etc.)         │                              │
│   └─────────────────────────────────┘                              │
│                                                                    │
│   ┌─────────────────────────────────┐                              │
│   │ Mendix BSON envelope shape      │  ← embedded templates        │
│   │ (WidgetValueType fields,        │  tied to Mendix 11.6 base    │
│   │  CustomWidget envelope,         │  patched manually for 11.9   │
│   │  array marker conventions)      │                              │
│   └─────────────────────────────────┘                              │
└────────────────────────────────────────────────────────────────────┘
```

**Widget-specific structure** is version-resilient out of the box. `widget init`
parses each project's installed `.mpk` files to derive the per-widget shape;
`sdk/widgets/augment.go` syncs additions and removals between the embedded
template and the installed widget. A new pluggable widget version (e.g.
DataGrid `2.30.1` → `2.31.0`) just works after `mxcli widget init`.

**Mendix BSON envelope shape** is brittle. The embedded templates at
`sdk/widgets/templates/mendix-11.6/*.json` were extracted at Mendix 11.6 and
manually patched as gaps surfaced. Each new Mendix minor that adds a field
to `CustomWidgets$WidgetValueType` or restructures the envelope requires
another round of patching.

## Why this split exists

`.mpk` files declare the widget's own contract (what properties exist, what
types they accept, what defaults the widget author chose). They're shipped
by widget authors and versioned independently of Mendix.

The BSON envelope (`CustomWidgets$WidgetValueType` field set, `WidgetObject`
ordering rules, `Forms$Appearance` structure, etc.) is Mendix runtime infra
that evolves with Mendix itself. There's no per-project file declaring it —
Studio Pro hardcodes the expected shape, and "right" depends on which
Mendix version is reading the BSON.

## Fixes landed for Mendix 11.9 compatibility

| Commit | Field / behavior | Why 11.9 cared |
|---|---|---|
| `ec99cdff` | `AllowUpload: false` on every `WidgetValueType` | Field added in some 11.x; absence triggers CE0463 across every widget |
| `b1f4de3a` | `WidgetObject.Properties` order = `WidgetType.PropertyTypes` order | Studio Pro 11.9 checks for matching ordering; bulk-reordered 5 templates |
| `7e6fee84` | Filter widgets carry full CustomWidget envelope (`Appearance`, `ConditionalEditabilitySettings`, `ConditionalVisibilitySettings`, `LabelTemplate`) | Studio Pro flags incomplete envelopes on nested CustomWidgets |
| `f9818394` | `TextTemplate.Template.Items` populated from `PropertyType.ValueType.Translations` defaults; `Editable: "Always"` on filter widgets | Studio Pro copies translation defaults at widget creation; mxcli left them empty |
| `aea000b7` | `columnsFilterable` and `sortable` Boolean values aligned with their `PropertyType.ValueType.DefaultValue` | Template-extraction bug: stored `false` vs schema-default `true`; Studio Pro detects mismatch |
| `4ea402c2` | Object-list item TextTemplate slots emit `null` (not placeholder `" "` ClientTemplate) when unset | `createDefaultWidgetValue` + `overlayItemValue` were manufacturing `Text: " "` ClientTemplates for every TextTemplate-typed sub-property of every object-list item. Studio Pro CE0463 on Accordion, AreaChart, Maps engine-routed widgets (#548) |

After the first five fixes the v0.10 acceptance fixture
(`mdl-examples/doctype-tests/31-pluggable-datagrid-gallery-v010-examples.mdl`)
emits zero CE0463 errors on a fresh Mendix 11.9 project. The sixth fix
extends this to the engine-routed widgets in
`mdl-examples/doctype-tests/32-pluggable-widget-object-lists-v010.mdl`
— `mx check` reports zero CE0463 across all six failing widget instances
(`acc2`, `map1`, `map2`, `menu1`, `menu2`, `chart1`). Remaining errors on
that fixture are authoring issues (missing `menuTrigger` child slot
keyword, missing Maps API key), not BSON drift.

## What's version-stable vs version-fragile

| Element | Source | Version-fragile? |
|---|---|---|
| Widget PropertyKeys (top-level) | MPK XML via `widget init` | ✓ stable |
| Widget property types (Attribute / Expression / TextTemplate / etc.) | MPK XML | ✓ stable |
| Object-list properties (`columns`, `groups`, `series`...) | MPK XML | ✓ stable |
| Sub-property trees inside object lists | MPK XML | ✓ stable |
| Widget version (DataGrid 2.22 → 2.30) | MPK file metadata + augmentation | ✓ stable |
| `CustomWidgets$WidgetValueType` field set | Embedded template + manual patches | ✗ fragile |
| `CustomWidgets$WidgetObject` Properties array ordering | Embedded template | ✗ fragile |
| Required CustomWidget envelope fields | Embedded template + filter widget builder | ✗ fragile |
| TextTemplate default translation population | Embedded template | ✗ fragile |
| Boolean property default consistency | Embedded template | ✗ fragile |

## Onboarding a new Mendix minor (e.g. 11.10, 12.0)

The CE0463 fix methodology used for 11.9 generalizes. Steps:

1. **Download mxbuild for the target version**:
   ```bash
   mxcli setup mxbuild --version 11.10.0
   ```

2. **Run the v0.10 fixture against a fresh 11.10 project**:
   ```bash
   mxcli new TestApp --version 11.10.0
   mxcli widget init -p TestApp/TestApp.mpr
   mxcli exec mdl-examples/doctype-tests/31-pluggable-datagrid-gallery-v010-examples.mdl -p TestApp/TestApp.mpr
   ```

3. **Check with mx**:
   ```bash
   ~/.mxcli/mxbuild/11.10.0/modeler/mx check TestApp/TestApp.mpr
   ```

4. **For each new CE0463** (or other widget validation error):

   Use the **"Studio Pro Update Widget" diff** methodology documented in
   [`.claude/skills/debug-bson.md`](../../.claude/skills/debug-bson.md#ce0463-widget-definition-changed):
   - Snapshot the failing BSON
   - Open in Studio Pro 11.10
   - "Update widget" on one failing widget instance
   - Snapshot again
   - Diff (with UUID normalization)
   - The diff tells you exactly what to patch in the embedded templates or
     the filter widget builder

   Each pattern that appears (new envelope field, ordering change, default
   value) typically yields a one-line fix and unblocks dozens of widgets.

5. **Add a non-regression test** — see "Cross-version validation" below.

## Where the patches live

- **Embedded templates**: `sdk/widgets/templates/mendix-11.6/*.json` —
  for envelope-level fixes that apply to every widget instance loaded from
  the embedded template. Most CE0463 fixes land here as bulk-edits across
  files (the `AllowUpload` fix added 581 fields across 30 files in one go).

- **Filter widget builder**: `mdl/backend/mpr/datagrid_builder.go`
  (`buildFilterWidgetBSON`, `buildMinimalFilterWidgetBSON`, the
  `defaultEmptyAppearance` helper) — for the CustomWidget envelope mxcli
  constructs around filter widgets inside DataGrid columns.

- **WidgetValueType serializer**: `sdk/mpr/writer_widgets_custom.go`
  (`serializeWidgetValueType`) — for the structured-data path (not the
  RawType clone path) when building widget BSON from typed inputs.

- **Template augmentation**: `sdk/widgets/augment.go`
  (`createDefaultValueType`) — for MPK-derived widget templates when no
  embedded template exists.

When patching a field, **update all four paths** if the field is supposed to
be ubiquitous. The CE0463 fixes for `AllowUpload` touched the embedded JSON,
both serializers, and the augment helper.

### Gotcha: `$ID` placeholders must be unique

When bulk-adding entries with a `$ID` field to embedded templates (e.g.
`Texts$Translation` entries inside `TextTemplate.Template.Items`), each
entry **must** have a unique placeholder `$ID` value. The loader's
`collectIDs` remapping (in `sdk/widgets/loader.go`) treats identical `$ID`
strings as the same logical entity and remaps them to a single new UUID at
load time. Multiple widget instances on a page then end up referencing the
same UUID, triggering `Duplicate Guid in unit page ...` from `mx
update-widgets` and a subsequent `Root unit not found` corruption.

**Convention**: follow the `placeholderID()` function in
`sdk/widgets/augment.go` — `aa000000000000000000000000XXXXXX` with a unique
counter per entry. Caught by the integration test
`TestMxCheck_DoctypeScripts`, fixed in commit
[`8ead1cff`](https://github.com/mendixlabs/mxcli/commit/8ead1cff).

## 11.10 onboarding result: no envelope drift

Running the v0.10 acceptance fixtures (`31-…`, `32-…`) and the broader
`30-pluggable-widget-examples.mdl` through `exec` + `mx check` on **both**
Mendix 11.9 (`test5-app`) and 11.10 (`test6-app`) produces **identical**
CE0463 sets:

| Fixture | 11.9 | 11.10 |
|---|---|---|
| `30-pluggable-widget-examples` | `tf1` (#605) | `tf1` (#605) |
| `31-…datagrid-gallery-v010` | `dgDyn` | `dgDyn` |
| `32-…object-lists-v010` | none | none |

So the 11.6-based envelope is **stable across 11.9 → 11.10** — no field was
added to `WidgetValueType`, no ordering changed, no `Appearance`/`TextTemplate`
convention shifted. The two residual CE0463s appear on *both* versions, so they
are **version-independent widget bugs** (tracked separately: `tf1` = #605,
`dgDyn` = a `datagrid` keyword-form regression), not envelope drift.

Practically: v0.12.0 Stream A's planned per-version conditionals (threading
`MendixVersion` into the serializer, gating `AllowUpload`/`Appearance` shape)
turned out **unnecessary for 11.9 ↔ 11.10** — there is nothing to conditionalize.
The conditionals would only become necessary if a future minor introduces a
delta, and the gate below is what would surface that.

## Cross-version validation gate

`make check-widget-versions` (script: `scripts/check-widget-versions.sh`) runs
a widget fixture through `exec` + `mx check` against several Mendix versions and
**fails if the CE0463 set differs between them** — i.e. it detects envelope
drift specifically, ignoring version-independent bugs (which appear on every
version). It does *not* require zero CE0463.

```bash
# Defaults to test5-app (11.9) + test6-app (11.10); override the project paths:
make check-widget-versions \
  MX_PROJECT_119=/path/to/app-11.9.mpr \
  MX_PROJECT_1110=/path/to/app-11.10.mpr

# Or run the script directly against any fixture + version set:
scripts/check-widget-versions.sh mdl-examples/doctype-tests/31-pluggable-datagrid-gallery-v010-examples.mdl \
  11.9.0:../ModelSDKGo/mx-test-projects/test5-app/test5.mpr \
  11.10.0:../ModelSDKGo/mx-test-projects/test6-app/test6.mpr
```

Each version needs its mxbuild installed (`~/.mxcli/mxbuild/<ver>/`) and a
reference project with the fixture's widgets (`.mpk`) installed. The 11.10
`mx` binary's libSkiaSharp crash is handled automatically (the script checks
via `scripts/mx-check.sh`). This catches version drift the moment it happens,
rather than at user-report time. The long-term replacement (build-time
templates per version) is tracked under the unified schema registry effort
([#529](https://github.com/mendixlabs/mxcli/issues/529), Phase 5).

## The long-term answer

The brittleness of the embedded-template layer is exactly what the unified
schema registry proposal addresses
([`docs/11-proposals/UNIFIED_SCHEMA_REGISTRY.md`](../11-proposals/UNIFIED_SCHEMA_REGISTRY.md)).
Phase 4 of that proposal replaces the embedded `mendix-11.6/*.json`
snapshots with templates generated at build time from
`mx dump-mpr` output, parameterized by Mendix version. New Mendix release
support becomes "run codegen against `mx` from that version" rather than
manual patching after CE0463 reports come in.

In the meantime, this doc + the `.claude/skills/debug-bson.md` methodology
keep the patch cadence manageable.

## References

- [`.claude/skills/debug-bson.md`](../../.claude/skills/debug-bson.md) — investigation procedure for CE0463 and related widget BSON errors
- [`docs/03-development/PAGE_BSON_SERIALIZATION.md`](PAGE_BSON_SERIALIZATION.md) — page-level BSON serialization design
- [`docs/03-development/BSON_TOOLING_GUIDE.md`](BSON_TOOLING_GUIDE.md) — `mxcli bson dump` reference
- [Issue #541](https://github.com/mendixlabs/mxcli/issues/541) — the CE0463 case study that motivated this doc
- [Issue #529](https://github.com/mendixlabs/mxcli/issues/529) — unified schema registry proposal (long-term fix)
