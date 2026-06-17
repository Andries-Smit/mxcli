# mxcli MCP Backend — Known Issues

Discovered during page writing tests against Mendix 11.11.0.
mxcli version: v0.12.0-290-g9a58b1692 (2026-06-17)
Repo: https://github.com/mendixlabs/mxcli

> **Status (2026-06-17): all three resolved** on the `mcp` branch (commit
> `a70e50b73`). All three were mxcli `pg_write_page` payload gaps — **not PED
> limitations** — confirmed by capturing a Studio-Pro-generated edit page via
> `pg_read_page` (fixtures in `mdl/backend/mcp/testdata/pg-page-contact-newedit*.json`)
> and matching the real shapes. See the per-issue **Resolution** notes below.

---

## Issue 1: Page parameter entity type not preserved (stored as `DataTypes$UnknownType`)

### Summary

Creating a page with a typed parameter via `mxcli exec --mcp` loses the entity type.
The parameter is created but stored as `DataTypes$UnknownType` instead of the declared entity type.

### Reproduce

```mdl
create or modify page MyFirstModule."Contact_Edit" (
  params: { $Contact: MyFirstModule.Contact },
  title: 'Edit Contact',
  layout: Atlas_Core.Atlas_TopBar
) {
  dynamictext txt (content: 'test')
}
```

```bash
./mxcli exec page.mdl -p FeeDemo.mpr \
  --mcp http://localhost:7782/mcp --mcp-dial "[::1]:7782"
```

`DESCRIBE PAGE MyFirstModule.Contact_Edit` then shows:

```
Params: { $Contact: DataTypes$UnknownType }
```

instead of:

```
Params: { $Contact: MyFirstModule.Contact }
```

### Root cause (diagnosed via PED)

The `Pages$PageParameter` PED constructor only exposes `name`:

```json
{
  "type": "$constructor",
  "elementType": "Pages$PageParameter",
  "properties": {
    "name": { "type": "string" }
  }
}
```

`parameterType` is an element-typed property, so it cannot be `set` after creation either:

```
ERROR: '[0]/parameters/0/parameterType':
It is only allowed to set primitive or reference properties directly.
```

There is no PED path to create a typed page parameter.

### Fix needed

Add `parameterType` to the `Pages$PageParameter` PED constructor, or support `set`
on element-typed `parameterType` after add.

### Resolution (FIXED)

**The root cause above is incorrect.** Pages are written via `pg_write_page`, not
`ped_create_document`, so the `Pages$PageParameter` PED *constructor* schema is
irrelevant. `pg_read_page` of a known-good page shows the real shape — a nested
element, not a flat field:

```json
"parameterType": { "$Type": "DataTypes$ObjectType", "entity": "MyFirstModule.Contact" }, "isRequired": true
```

mxcli was sending a flat `entity` field (`page.go` `pageParameters`), which
`pg_write_page` ignores → `UnknownType`. Fixed to emit the nested `parameterType`.
No PED/Studio Pro change was needed.

---

## Issue 2: Client actions not supported by MCP backend

### Summary

Any page button using a built-in client action fails at execution time with:

```
client action *pages.<ActionType> is not yet supported by the MCP backend
```

Affected actions: `save_changes`, `cancel_changes`, `close_page`.

### Reproduce

```mdl
actionbutton btnSave (caption: 'Save', action: save_changes, buttonstyle: primary)
```

Error:
```
Error: failed to create page: page "Contact_Edit": button "btnSave":
client action *pages.SaveChangesClientAction is not yet supported by the MCP backend
```

Same error for:
- `cancel_changes` → `*pages.CancelChangesClientAction`
- `close_page` → `*pages.ClosePageClientAction`

Microflow actions (`action: microflow Module.Name`) work correctly.

### Impact

Edit pages cannot have working Save/Cancel buttons without wrapping the logic in
a microflow. This makes standard CRUD edit pages impossible to author end-to-end
via mxcli without a workaround.

### Workaround

Use a commit + close-page microflow and wire the button to it:

```mdl
actionbutton btnSave (
  caption: 'Save',
  action: microflow MyModule.ACT_Contact_Save,
  buttonstyle: primary
)
```

Where `ACT_Contact_Save` commits the object and closes the page.

### Resolution (FIXED)

`mapClientAction` (`page_widgets.go`) now handles `Pages$SaveChangesClientAction`,
`Pages$CancelChangesClientAction`, and `Pages$ClosePageClientAction`. Shapes
captured from a generated edit page:

```json
"action": {"$Type":"Pages$SaveChangesClientAction","disabledDuringExecution":true,"syncAutomatically":false,"closePage":true}
"action": {"$Type":"Pages$CancelChangesClientAction","disabledDuringExecution":true,"closePage":true}
```

The microflow workaround is no longer required for standard Save/Cancel/Close buttons.

---

## Issue 3: `designproperties:` silently dropped on page creation via MCP (mxcli bug)

### Summary

Design properties specified in MDL are silently lost when creating a page via `mxcli exec --mcp`. No error is returned — the page is created, but all `designproperties:` are absent in the model.

### Reproduce

```mdl
create or modify page MyFirstModule."Theme_Test" (
  title: 'Theme Test',
  layout: Atlas_Core.Atlas_TopBar
) {
  layoutgrid outerGrid (
    designproperties: ['Row gap': 'Large', 'Column gap': 'Medium']
  ) {
    row row1 {
      column col1 (desktopwidth: autofill) {
        dynamictext txt (
          content: 'Hello',
          designproperties: ['Color': 'Brand Primary', 'Weight': 'Bold']
        )
      }
    }
  }
}
```

`DESCRIBE PAGE` after exec shows no design properties on any widget. `mxcli check --references` passes, exec completes without error — silent drop.

### Diagnosis

Unlike client actions (Issue 2) which return an explicit `not yet supported by the MCP backend` error, design properties produce no error at all. This means Studio Pro's MCP handler never received them — the MDL-to-PED translation in `CreatePage` is not including design properties in the JSON payload.

Supporting evidence:
- `DESCRIBE PAGE` correctly reads design properties from `.mpr` on pages created in Studio Pro (read path works)
- `mxcli check` accepts `designproperties:` syntax (parser knows about it)
- No error from Studio Pro → payload accepted, field simply absent

This is a mxcli translation bug, not a PED limitation. Likely in the page builder's MCP serialization path (`mdl/executor/cmd_styling.go` or equivalent).

### Fix needed

Include `designproperties` in the PED `CreatePage` JSON payload when building pages via `--mcp`.

### Workaround

Use `style:` with raw CSS or `class:` with Atlas/Bootstrap utility class names. Both survive write via MCP:

```mdl
container ctnCard (
  style: 'background-color: var(--brand-primary); padding: 24px;',
  class: 'card'
)
```

### Environment

- mxcli version: v0.12.0-290-g9a58b1692
- Mendix: 11.11.0
- OS: macOS (darwin/arm64)

### Resolution (FIXED)

Correct diagnosis. `pageAppearance` hardcoded an empty `designProperties` and
didn't accept the widget's properties at all (the location was the MCP backend's
`page_widgets.go`, not `cmd_styling.go`). Design properties are now overlaid in
`mapPageWidget` (one place, every widget type) as the object `pg_write_page`
expects — keyed `"<kind>:<DisplayName>"` (`option:`→string, `toggle:`→bool):

```json
"designProperties": {"option:Column gap":"Medium","option:Background color":"Background Secondary","toggle:Cards style":true}
```

(Compound/nested design properties aren't expressible in MDL, so they're not emitted.)
