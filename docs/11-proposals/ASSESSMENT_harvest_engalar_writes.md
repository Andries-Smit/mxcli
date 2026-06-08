# Assessment: harvest engalar/dev's gen-native writes vs continue model→gen

- **Date**: 2026-06-08
- **Status**: Assessment (input to a decision; relates to [ADR-0004](../13-decisions/0004-full-codec-engine.md))
- **Trigger**: discovered that `engalar/dev` (not the stale `feat/modelsdk-core` checked earlier) has a **complete, tested gen-native write subsystem**, while we have been hand-porting model→gen.

## Correction of a prior belief

Earlier analysis concluded "engalar built the codec but never wired it into writes." That was true only of the **stale `feat/modelsdk-core`** branch. **`engalar/dev` wires the engine into writes comprehensively** — domain models, enums, constants, business events, workflows, and a full **microflow** write suite — with tests and a "dispatcher + legacy fallback".

## The two architectures

| | **Ours (this branch)** | **engalar/dev** |
|---|---|---|
| Source of truth for a write | `sdk/microflows.Microflow` **model**, built from AST by the existing executor | the **AST directly** (`flowBuilderGen`), no model intermediate |
| Backend interface | model-typed (`CreateMicroflow(*microflows.Microflow)`) | gen-typed reads (`ListMicroflowsGen`/`GetMicroflowGen`); **no** model-typed create |
| Write mechanism | `microflowToGen` (model→gen) → `codec.Encoder` → `InsertUnit` | `repos/` + `unitstore` (buffered), driven by `ctx.Microflows` |
| Executor create handler | unchanged from main | rewritten: `execCreateMicroflowGen` |
| Blast radius | low — main's executor/interface/legacy untouched; modelsdk is an alternate backend | high — changes executor↔backend contract + AST-direct handlers + adds repos/unitstore |

Our approach is the one [ADR-0004] / the backend-strategy analysis chose: **vendor the engine, keep main's architecture**. engalar's approach is closer to **adopting engalar's branch** for the write path.

## Scale (engalar/dev, microflows only, non-test)

- `flowbuilder_*_gen.go` + `cmd_microflows_create_gen.go`: **~6,255 LOC** (all activity types, ELK auto-layout, calls/pages/workflows, with `_test.go`)
- `mdl/backend/mpr/repos/`: **~4,270 LOC**; `mdl/backend/unitstore/`: ~145 LOC
- Same pattern already exists on dev for pages/workflows — so the **cumulative** duplication we'd otherwise hand-port is far larger than microflows alone.

## Harvestability — encouraging and discouraging signals

**Encouraging:** the AST statement types the flowbuilder consumes (`DeclareStmt`, `MfSetStmt`, `MfCommitStmt`, `DeleteObjectStmt`, `RollbackStmt`, `CreateListStmt`, …) are **identical on our branch**. So `flowBuilderGen` operates on the same AST we have — it is largely portable, not entangled with a divergent grammar.

**Discouraging (the real cost):** `engalar/dev`'s `modelsdk/gen`+`codec` has **diverged 162 files / 116 commits / ~8,600 insertions** from the copy we vendored. The flowbuilder calls gen setters that may only exist in dev's evolved gen. So adopting the write suite **pulls in a re-sync of the whole engine** — it is a *rebase onto engalar/dev's modelsdk*, not a surgical cherry-pick. Plus `ctx.Microflows` (repo on ExecContext) + the repos/unitstore layer + the interface change.

## What we've already built (and would partly discard under adoption)

Domain models (full ALTER), enumerations, constants, microflow skeleton/params/object-ops — all model→gen, all at strict legacy parity. engalar/dev has gen-native equivalents of all of these. Discarding the microflow skeleton/params/object-ops is a small loss; the domain-model lossless-adapter + ALTER work is more substantial and main-architecture-specific.

## Options

**A. Continue model→gen (status quo).** Keep main's architecture; finish microflows group-by-group, then pages, then workflows. *Pro:* low risk/blast radius, legacy untouched, we own it, no engine re-sync. *Con:* we re-express engalar's tested work (cumulatively large), and stay permanently diverged — every future engalar write improvement needs re-porting. Use engalar's `flowbuilder_*_gen.go` as the reference (better than the legacy serializer).

**B. Adopt engalar/dev's gen-native write subsystem.** Rebase our modelsdk onto engalar/dev's engine, pull in flowbuilder + repos + unitstore + the gen-typed interface + executor rewire. *Pro:* reuse ~10.7k tested LOC (and the same for pages/workflows later), end the duplication, re-align with engalar so future cherry-picks are cheap. *Con:* large refactor; re-syncs 162 engine files; changes the executor↔backend contract; diverges from main's executor architecture (ongoing sync cost with `origin/main`).

**C. Hybrid.** Keep model→gen for the simple types already done (entities/enums/constants); adopt engalar's gen-native path only for the complex create-only types (microflows/pages/workflows). Still requires the engine re-sync + repos/unitstore.

## Recommendation

The duplication is real and **cumulative** (microflows ≈ 6k LOC is just the first of three big complex types engalar already finished). The AST compatibility is a strong "harvest is feasible" signal. But the 162-file engine divergence means adoption is a **rebase, not a cherry-pick**, with meaningful blast radius.

**Recommend a time-boxed SPIKE before deciding**: on a throwaway branch, rebase our modelsdk onto `engalar/dev`'s `modelsdk/` engine and pull in the microflow write subsystem (flowbuilder + repos + unitstore + `ctx.Microflows` + interface), then see whether (a) it compiles and (b) `TestWriteParity_Microflow*` pass. The spike's outcome tells us whether B/C dominates A: if it integrates in ~a day, the cumulative reuse almost certainly beats hand-porting microflows+pages+workflows; if it fights the engine re-sync, A (continue, using engalar as reference) is the safer finish.
