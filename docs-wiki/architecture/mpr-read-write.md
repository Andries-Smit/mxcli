---
title: MPR Read/Write
category: architecture
last-synced: 4e185f73
sources:
  - sdk/mpr/reader.go
  - sdk/mpr/writer_core.go
  - sdk/mpr/parser.go
  - sdk/mpr/writer_widgets.go
  - modelsdk.go
---

> **Do not duplicate**: the public API surface (see `README.md` and `modelsdk.go`), specific BSON field tables (see `docs/03-development/PAGE_BSON_SERIALIZATION.md`), or fix recipes (see `.claude/skills/fix-issue.md`).

## What this is

The layer that turns a `.mpr` file on disk into typed Go model elements and back. An `.mpr` is a SQLite database whose document rows hold BSON-encoded Mendix model elements, so reading and writing is two problems stacked: SQLite access, and BSON (de)serialization of polymorphic Mendix types. This layer owns both.

## How it fits

[`modelsdk.Open`](../../modelsdk.go) returns a read-only [`Reader`](../../sdk/mpr/reader.go); `OpenForWriting` wraps it in a [`Writer`](../../sdk/mpr/writer_core.go). That nesting is deliberate — a writer *is* a reader plus mutation methods, because every safe write first reads the current state. The reader opens SQLite via the pure-Go `modernc.org/sqlite` driver (no CGO), pins a single connection to dodge lock contention, and detects the storage format.

Format detection is automatic and defensive. v1 is a single-file database; v2 (Mendix 10.18+) splits metadata from per-document `.mxunit` files under `mprcontents/`. The reader first checks for the folder, then reconciles against the actual DB schema, because a `.mpr` copied without its `mprcontents/` folder would otherwise take the wrong code path. v2 writes go through a [`WriteTransaction`](../../sdk/mpr/writer_core.go) that stages files to temp paths and coordinates them with the DB transaction so a crash never leaves a half-written unit.

The reason this layer is BSON-aware rather than a generic SQLite patcher is that Mendix's BSON is irregular: IDs appear as binary blobs, base64 maps, or `$ID` fields; arrays carry a leading type marker (`2` or `3`); and `$Type` discriminators select polymorphic structs. The [parser](../../sdk/mpr/parser.go) and the [widget serializer](../../sdk/mpr/writer_widgets.go) encode these conventions exactly, because Studio Pro rejects any deviation — a wrong storage name, a malformed empty-array marker, or a numeric width mismatch all surface as load-time exceptions. See [[models/storage-vs-qualified-names]] and [[bug-patterns/bson-numeric-width]].

## See also

- [modelsdk.go](../../modelsdk.go) — public `Open` / `OpenForWriting` entry points and constructors
- [docs/03-development/PAGE_BSON_SERIALIZATION.md](../../docs/03-development/PAGE_BSON_SERIALIZATION.md) — BSON field/type tables
- [[models/storage-vs-qualified-names]] — why `$Type` strings differ from SDK names
- [[bug-patterns/bson-numeric-width]] — int32/int64 serialization hazards
- [[architecture/mdl-execution]] — the backend layer that drives these writes
- [[architecture/widget-engine]] — widget BSON, which sits on top of this
