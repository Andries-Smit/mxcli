---
title: PED Mutation Constraints
category: mental-model
last-synced: cfcf1ea12
sources:
  - docs/03-development/PED_MCP_CAPABILITIES.md
  - mdl/backend/mcp/workflow.go
  - mdl/backend/mcp/domainmodel.go
  - mdl/backend/mcp/microflow.go
---

> **Do not duplicate**: the exhaustive capability/limitation matrix and tool list
> (`docs/03-development/PED_MCP_CAPABILITIES.md` is canonical), or what a specific
> backend method does (read source). This page is the *mental model* a contributor
> needs before extending the MCP backend; the facts live in the capability doc.

## What this is

The MCP backend writes through Studio Pro's embedded MCP server (PED), not by
serialising BSON. PED exposes **simplified, strict constructors** and a
**constrained patch API** (`ped_update_document` set/add/remove ops). The shapes
PED accepts are deliberately narrower than the on-disk model, so a capability that
exists in BSON is not automatically expressible over MCP. The governing discipline
is: **probe the schema, map exactly, reject what can't be expressed, and verify by
re-validating** — never guess a field name, and never assume "accepted" means
"valid."

## How it fits

A handful of invariants, learned empirically, explain most of what the backend can
and cannot do. They recur across the domain-model, microflow, and workflow slices:

- **Constructors are simplified and strict.** `ped_get_schema` is the source of
  truth for field names and element `$Type`s — map from it, not from the BSON
  layout or the TypeScript SDK. A constructor often omits properties the full model
  element has: PED's `byDatabaseQuery` is only `entity`+`xPathConstraint`+
  `takeOnlyFirst` (no sorting, no custom range); `CastAction` exposes only an output
  variable (no input or target type); attribute `type` and a multi-user-task's
  string outcomes are likewise pared down. When a request can't be expressed,
  **reject it with an actionable error** rather than silently mis-build — a
  half-built element is worse than a refusal.

- **Only primitive and reference properties can be set directly.** A `set` on a
  *nested element* path is refused ("only allowed to set primitive or reference
  properties directly"). You change an attribute's documentation or an activity's
  page by setting a leaf (`/taskPage/page`, `/documentation`); you cannot swap the
  *kind* of a nested element (an attribute's type, a user-targeting's XPath↔
  Microflow) because that is an element replacement, not a leaf set.

- **Array edits have their own rules.** `add`/`remove` by index work, but an `add`
  with an explicit index is validated against the array's *original* (pre-batch)
  length, so incrementing indices can't grow a flow; an index-less `add` appends and
  PED may auto-position by element type. Some elements are structural and
  unremovable (a workflow's Start/End). These combine into non-obvious recipes — a
  flow rewrite inserts new middles at index 1 in reverse order; an attribute or
  activity "replace" is remove-then-add, never a set-by-index.

- **Acceptance is not validity.** A write can succeed and still leave an invalid
  document. `ped_check_errors` returning "No errors found." is the real gate — treat
  it as the pass/fail signal, and confirm the *element shapes* validate, not just
  that the call returned 200.

- **Verify the exact payload, not a proxy.** When the live transport is flaky,
  reproduce the precise op sequence the mapper emits via raw `ped_create_document` /
  `ped_update_document` against a live document, rather than trusting a green test
  that exercised a different path. (And beware non-idempotent ops: a timed-out
  response may have applied server-side — retrying compounds it.)

The throughline: PED is a *model API with opinions*, not a serialiser. Extending the
backend is a loop of probe → map → reject-the-inexpressible → validate, and the
constraints above are why the capability doc reads as a list of "supported, with
these specific gaps" rather than "everything BSON can hold."

## See also

- [../../docs/03-development/PED_MCP_CAPABILITIES.md](../../docs/03-development/PED_MCP_CAPABILITIES.md) — canonical tool matrix, capability gaps, and per-feature shapes
- [[architecture/mcp-backend]] — where this write path sits (hybrid local-read / live-PED-write)
- [[architecture/mpr-read-write]] — the other write backend, which *does* serialise BSON directly
- [[models/storage-vs-qualified-names]] — a related place where Mendix naming surprises you
