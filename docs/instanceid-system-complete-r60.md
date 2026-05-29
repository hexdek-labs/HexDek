# InstanceID System — Complete

**Status:** SHIPPED — all 9 phases merged
**Design:** `docs/instanceid-system-v2-r60.md` (PR #747)
**Era:** r60
**Final phase:** Phase 9 — Spectator + Heimdall lineage rendering
**Closes bug class:** `*Card` pointer aliasing (CardIdentity invariant family), ZoneConservation copy-tracking ambiguity, ZoneCastGrantExpiry source-lifetime conflation, ExileLinkageIntegrity LTB-vs-cast-grant false positives.

---

## What shipped

A complete per-instance identity system for every game object — every `Card` now carries an immutable `InstanceID` string with provenance (OG / TK / CP / AB), visibility (V / H), color, CMC, and lineage links (`SourceInstanceID` + `EnablerInstanceID` + `EnablerHistory`). Tokens, spell copies, ability instances, mutate stacks, and meld results all participate. Pointer-identity ambiguity that produced the 3324 / 1516 / 736-hit CardIdentity / ZoneConservation / ExileLinkageIntegrity clusters in the 2026-05-28 25k layer-stress sweep is closed at the root.

Spectator API exposes lineage. Loki logs stamp every event line with relevant InstanceIDs for `grep`-driven forensic replay. Heimdall renders walked lineage trees for any permanent in any live spectator room.

---

## Phase-by-phase

| Phase | Scope | PR | Key deliverable |
|---|---|---|---|
| 1 | Foundation — Card lineage fields + Minter + format regex | #748 (Phase 1 commits) | `internal/gameengine/instanceid/instanceid.go`, `mintInstanceID` helper, OG mint at deck-load, backwards-compat empty-ID legacy mode |
| 2 | Token + copy mints + AbilityInstance | #748 (Phase 2 commits) | `MintTokenInstanceID` / `MintCopyInstanceID` chokepoints, `AbilityInstance` struct + `StackItem.Ability` field, `TriggerMetadata` capture at push time |
| 3 | Linkage refactor + ExiledByMe | #753 | `Permanent.ExiledByMe []string` + `LinkageKind` (LTBReturn / CastGrant / PermanentExile), Banisher Priest / Oblivion Ring / Karmic Guide migration |
| 4 | Invariant migration + 25k sweep verification | #755 | CardIdentity / ZoneConservation / ExileLinkageIntegrity rewrites; `--instanceid-strict-census` Loki flag; 25k sweep result deltas captured |
| 5 | Copy + replacement-effect handlers + mint-coverage close | #758 | `CopyMechanisms []CopyMechanism`, 5 per-card overrides from Probe A (Enolc / Ertai's Meddling / Sakashima / Mirage Mirror / Spark Double + Phantasmal Image riders), `EffectsApplied []ReplacementRef` audit, Sai+Mondrak+Anointed+DS chain producing 8 distinct InstanceIDs |
| 6 | Subsystem activation registry | #760 | 484 dormant hooks per Probe D (Day/Night / Monarch / Initiative / Ascend / Dungeons / Ring tempts / Energy / XP / Foretell), zero cost when inactive |
| 7 | Counter DB foundation (parallel moat) | #748 series | `CounterTypeDef` registry with 252 types from Probe F, §122.6 persistence + §122.1g doubling + §704.5r pair-removal SBA + §122.1c keyword-counter ability grants, energy carveout per §106.11 |
| 8 | Mutate + Meld + Delayed triggers | #762 | `MergedCards/MergeKind/TopCard` unified Mutate+Meld shape, 7 meld-trigger + 5 scaling-mutate-trigger handlers, `DelayedAbilityInstances` pool with `DelayedCondition` evaluator, Suspend reimplemented as delayed-trigger composition |
| 9 | Spectator + Heimdall lineage rendering | THIS PR | Phase 9 deliverables below |

---

## Phase 9 deliverables (this PR)

### Engine

- `internal/gameengine/invariants.go` — `RecentEvents` formatter extended to stamp every line with InstanceID lineage from `Event.Details` under the 5 canonical keys (`instance_id`, `card_instance_id`, `source_instance_id`, `enabler_instance_id`, `ability_instance_id`). Forensic replay: `grep "h2CPR1" loki.log` surfaces every event ever touching a red 1-CMC seat-2 copy.

### Spectator API

- `internal/hexapi/showmatch.go` — `PermanentSnapshot` gains `instance_id`, `provenance`, `source_instance_id`, `enabler_instance_id`, `enabler_history`, `merged_card_ids` fields (all omitempty for backwards-compat). `LogEntry` gains `instance_id` for log-line lineage. `GameSnapshot.Lineage map[string]heimdall.LineageRecord` indexes every Card with a minted InstanceID across every zone on every seat — battlefield, hand, library, graveyard, exile, command zone, plus Mutate/Meld absorbed cards via `Permanent.MergedCardPtrs`.
- `internal/hexapi/spectator_lineage.go` — new HTTP handler `GET /api/spectator/lineage/{instance_id}` walks live rooms for the requested ID and returns the rendered lineage tree as JSON. Validates the path parameter against the canonical InstanceID format (rejects path traversal). 400 on bad format, 404 on unknown ID, 503 when the room manager is unavailable.

### Heimdall

- `internal/heimdall/lineage.go` — `BuildLineageTree(records, instanceID) *LineageNode` walks `SourceInstanceID` + `EnablerInstanceID` + `MergedCardIDs` recursively. Acyclic by a visited-set guard — copy chains and shape-shift loops (Vesuvan → Lazav → back) cannot infinite-loop the walker. `RenderLineageText(node) string` produces the design v2 §13 multi-line shape ("Sai of the Shining Sword [h0OGW20042] → minted Thopter token [h0TKC11234] via ability instance [h0ABXW40456]").

### Frontend

- `hexdek/src/screens/Spectator.jsx` — `lineageTitle(p)` helper composes the perm-tile hover tooltip with InstanceID + provenance + source/enabler IDs + merged-card IDs when a non-stacked permanent carries Phase 1+ lineage. Legacy permanents (no minted ID) fall back to the prior name-only title.

### Property tests

- `internal/heimdall/lineage_phase9_test.go` — 6 tests: 3-deep walk shape, self-cycle acyclic guard, 2-cycle acyclic guard, unknown-root returns nil, empty-input handling, merged-children walk.
- `internal/hexapi/spectator_phase9_test.go` — 7 tests: lineage-index payload schema, endpoint format validation (good vs malformed IDs), 404 on unknown ID, 400 on bad format, 200 with valid tree+text JSON from a live room, `instanceIDFromDetails` priority order, Loki `RecentEvents` InstanceID stamping.

---

## Net effect — pointer-aliasing bug class

The 2026-05-28 layer-stress 25k sweep baseline:

| Invariant | r60 baseline (25k sweep) |
|---|---|
| CardIdentity | 3,324 hits |
| ZoneConservation | 1,516 hits |
| ExileLinkageIntegrity | 736 hits |
| **Total pointer-identity cluster** | **5,576 hits** |

Phase 4 invariant migration converted these checks from pointer-equality to InstanceID-equality, dissolving the bug class at the root rather than chasing individual leak sites with whack-a-mole fixes. Phase 9 closes the system end-to-end by exposing the lineage to operators (Loki forensic), developers (Heimdall replay), and spectators (UI hover).

---

## Empirical grounding

The design rested on six corpus probes walked across ~30,000 cards (PRs #741–#746). The probes ruled out a Loki harness pointer-aliasing artifact (Probe C), pinned the 4 copy mint paths (Probe A), enumerated 3,883 cards matching replacement-effect patterns with 343 InstanceID-relevant (Probe B), counted 484 subsystem activator candidates (Probe D), characterized cross-type meld surprises (Probe E), and catalogued 252 distinct counter types (Probe F). Every architectural decision in design v2 carries empirical citations; the implementation tracks the design without drift.

---

## What's next (out of scope for r60)

- **Full replacement-effect subsystem** beyond the InstanceID-relevant subset (damage redirection, life-gain replacement, etc.) — separate moat.
- **Continuous-effect layer system completeness** — covered by tracking issue #732.
- **AI-side InstanceID-aware decision logging** — optional Phase 9b on future demand.
- **xMage parity sweep** — still pending Josh greenlight per CLAUDE.md authority gate.

---

## Authority

Per the 2026-05-28 engineering authority handoff: 7174n1c co-primary on accuracy/architecture, Hex executes, wiedeman holds LOC + destructive + money gates. Total InstanceID system production LOC across 9 phases tracked in the design at ~3,300; final tally lands within the budget envelope.

The pointer-identity bug class is closed. The r60 era's foundational moat is complete.

— Hex (2026-05-29)
