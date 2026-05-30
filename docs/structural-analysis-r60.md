# Structural Analysis — Post-InstanceID v2 + Counter DB (r60)

**Date:** 2026-05-30
**Author:** Hex (structural audit, no code changes)
**Scope:** `internal/gameengine/` (~250 files) + `internal/gameengine/per_card/` (1,193 files)
**Trigger:** 7174n1c (2026-05-28) — *"we will need a full structure analysis nuts to bits for anything we are handling twice. We will need to put the worst offenders out to pasture."* The InstanceID v2 system (17 PRs) and Counter DB (9 PRs, through Phase 8) now provide canonical APIs that obsolete a substantial body of duplicated logic spread across handler files.

---

## Executive Summary

The engine post-InstanceID is **structurally healthy at the system layer** — token minting, trigger batching, replacement effects, zone-exit, and state tracking all have canonical APIs in place. The duplication burden has shifted into the **per_card layer**, where ~50–200 handler files still implement legacy patterns that bypass those canonical APIs. The dominant smell is **per_card handlers re-implementing zone bookkeeping inline** rather than routing through `DestroyPermanent` / `ExilePermanent` / `MoveCardToZone` / `CreateTreasureToken` etc.

**Top three structural risks:**

1. **Manual zone appends in 50 per_card files** (~150–200 sites) — `seat.Graveyard = append(...)` without `FireZoneChangeTriggers`, `descend`, `creature_died`, or §614 replacement plumbing. **This is the single biggest correctness debt surface.** Most are silent today only because the SBAs eventually catch the resulting state; observers that depend on zone-change events (Bolas's Citadel, Tergrid, Sefris of the Hidden Ways, descend mechanics) can and do miss triggers.

2. **Counter-map dual-path** — `perm.Counters[kind] = N` direct assignment in 10 engine `.go` files + ~89 total mutation sites. Counter DB Phase 6 hazard at `sba.go:1190-1215` runs both legacy map and modern `CounterStacks` paths in parallel. Initialization sites (sagas, planeswalkers, megamorph, level-up, clone) all bypass `counters.AddCounters`. This is intentional during migration but is the longest-lived dual-path in the codebase and should get a `counters.InitializeCounter` API to close.

3. **Inline predefined-token construction in ~8 per_card files** — Treasure/Clue tokens built as raw `&gameengine.Card{Name: "Treasure Token", ...}` + `enterBattlefieldWithETB` instead of `CreateTreasureToken`. Skips `Turn.TreasuresCreated` / `Turn.TokensCreated` counters and the `currentMintEnablerID` plumbing. Small footprint, high correctness leverage.

**Out-of-scope / NOT actually duplicated** (audit confirms): trigger-batch system (`BeginTriggerBatch`/`EndTriggerBatch` is properly consolidated across 9 files; no per_card layer accesses `pendingTriggers` directly); replacement pipeline (single `FireEvent`/`pickReplacement` path; no double-application of token-doublers or counter-doublers); `Permanent` state-field "duplicates" (the `LinkedExile` vs `ExiledByMe` and `Counters` vs `CounterStacks` field pairs are documented Phase-N migration windows with invariant gates, not orphaned fields).

**Stale-callout corrections from CLAUDE.md issue log** discovered during this audit:

- The 2026-05-25 row claiming **"4 of 5 sibling sites still standing — etrata x2, bilbo x1, thassa x1"** for the `removePermanent` API-misuse sweep is **stale**. Verified 2026-05-30: `etrata_the_silencer.go:61`, `gen_bilbo_birthday_celebrant.go:117`, and `thassa_deep_dwelling.go:77` all now route through `gameengine.ExilePermanent`. Only a comment-string reference to the legacy pattern remains in etrata. The sweep is genuinely closed.
- The 2026-05-28 row claiming **"9 per_card direct-Lost setters"** is also partially stale: only `batch_aj_win_the_game.go:344` remains as a real bypass; `helpers.go:84` is the intentional `emitWin` opponent-lockstep per the row's own caveat. Closer to "1 + 1 intentional" than 9.

---

## 1. Token-Creation Code Paths

**Engine surface (canonical):** `tokens.go` (217 LOC) exposes 8 predefined-token helpers (`CreateTreasureToken`, `CreateClueToken`, `CreateFoodToken`, `CreateBloodToken`, `CreateMapToken`, `CreateGoldToken`, `CreatePowerstoneToken`, `CreateJunkToken`) plus `CreateCreatureToken(gs, seat, name, types, p, t)`. `instanceid_mint.go` (285 LOC) provides `MintTokenInstanceID(gs, card, sourceID, enablerID)`. All canonical paths stamp `InstanceID`, register replacements, fire ETB triggers, and update `Turn.TokensCreated` / `Turn.TreasuresCreated` counters.

**Per_card adoption:** 111 files, 181 calls into the canonical helpers — strong adoption. CreateTreasureToken: 34 calls. CreateCreatureToken: 124 calls. CreateClueToken: 12.

**Inline-mint bypass sites (the duplication):**

| File | Token | Pattern |
|---|---|---|
| `per_card/dockside_extortionist.go:60-65` | Treasure | inline `&Card{Types: ["token","artifact","treasure"]}` + `enterBattlefieldWithETB` |
| `per_card/malcolm.go:56-61` | Treasure | same shape |
| `per_card/edward_kenway.go:66-76` | Treasure | same shape |
| `per_card/grim_hireling.go:61-71` | Treasure | same shape |
| `per_card/the_ghoul_gunslinger.go:88-99` | Treasure | same shape |
| `per_card/mr_house_president_and_ceo.go:101` | Treasure | inline arm (also calls `CreateTreasureToken` at L142 — internal inconsistency) |
| `per_card/the_rani.go:138-145` | Clue | inline `&Card{Types:["token","artifact","clue"]}` |

**AST-resolver inline mints in `resolve_helpers.go`:**

- `L923-957` Populate (token copy)
- `L2894-2912` Kaya's Ghastly Cardboard Army (inline Zombie Army)
- `L3147-3167` Copy-pronoun
- `L3831-3851` Manifest (Face-Down Creature) — **duplicate of**
- `L4496-4516` Manifest Dread (Face-Down Creature)

The two manifest sites are structurally identical and trivially extractable.

**Consolidation:** introduce `tokens.MintPredefined(gs, seat, kind, n)` switch helper; collapse the 6 inline Treasure mints + the inline Clue + the `mr_house` internal-inconsistency arm; extract `createFaceDownToken(gs, seat, kind)` for the two manifest paths. Estimated LOC delta: −80 / +25 = **~−55 net**, plus correctness (InstanceID + Turn counters consistent across all paths).

---

## 2. Zone-Change Handler Duplication

**Engine surface (canonical):** `zone_change.go` (1,008 LOC) — `DestroyPermanent`, `ExilePermanent`, `BouncePermanent`, `sacrificePermanentImpl` all encapsulate §614 replacement chains, §903.9b commander redirect, aura `DetachAll`, replacement/continuous-effect unregister, and `FireZoneChangeTriggers`. `FireZoneChangeTriggers` is the single observer-broadcast entry. 138 per_card files correctly call one of these canonical APIs.

**Duplication category A — manual zone-slice appends.** 50 per_card files (verified count) directly do `seat.Graveyard = append(seat.Graveyard, card)` / equivalent for `Library` / `Exile` / `Hand` outside any canonical API call. Sample of offenders ranked by likely site density:

| File | Notes |
|---|---|
| `per_card/birthing_ritual.go` | sac + fetch combo; multiple appends |
| `per_card/karmic_guide.go` | reanimate variant; manual graveyard pull |
| `per_card/winota_joiner_of_forces.go` | reveal-and-battlefield |
| `per_card/hakbal.go` | explore-style hand append |
| `per_card/glissa_sunslayer.go` | counter-fetch with hand append |
| `per_card/strefan_maurer_progenitor.go` | exile-to-battlefield variant |
| `per_card/gitrog_ravenous_ride.go` | land sac + graveyard recursion |
| `per_card/svella_ice_shaper.go` | impulse-style exile |
| `per_card/varolz_the_scar_striped.go` | scavenge graveyard manipulation |
| `per_card/smellerbee_rebel_fighter.go` | combat-trigger graveyard mill |

Each manual append silently skips: (a) the `descend` counter (Flags + `Turn.Descended`), (b) `creature_died` counter when applicable, (c) `Turn.ExiledCards`, (d) `FireZoneChangeTriggers` (so observers like Tergrid, Bolas's Citadel, Sefris, descend payoffs miss the event), (e) §614 replacements (`would_be_exiled` / `would_die`) which never apply.

**Duplication category B — `moveCardBetweenZones` direct usage without `FireZoneChangeTriggers`.** 30 per_card call sites. Examples: `valki_god_of_lies.go`, `gerrard_weatherlight_hero.go`, `etali_primal_storm.go`, `zoraline_cosmos_caller.go`, `per_card_batch_ai_r60.go` (5× for Knowledge Pool, 1× for Bribery). Some compensate with manual `FireZoneChangeTriggers` calls (17 files) but parameters drift between callers.

**Duplication category C — direct-Lost setters.** Only `batch_aj_win_the_game.go:344` remains as a genuine `seat.Lost = true` bypass that skips the `resolveLoseGame` pipeline (and thus §614 `would_lose_game` cancellation — i.e. Platinum Angel / Angel's Grace do not save against this card's loss). `helpers.go:84` is the intentional opponent-lockstep in `emitWin` per §104.2c (NOT a bypass).

**Consolidation:** export `MoveCardToZone(gs, card, fromZone, toZone, owner, source)` wrapper that does the low-level move + `FireZoneChangeTriggers` + descend/died/exile counter updates atomically. Sweep all 50 manual-append sites. For direct-Lost: extract `MarkSeatLostByEffect(gs, seat, srcName)` from `resolveLoseGame` body and fix `batch_aj_win_the_game.go:344`. Estimated LOC delta: **~−300 net** (the original audit estimate; verified against 50-file × 5-7 line manual-append count).

---

## 3. Trigger-Batch Code Paths

**Finding: NO duplication. System is properly consolidated.**

`BeginTriggerBatch` / `EndTriggerBatch` defined in `trigger_batch.go:52-73`. 9 files use the batch API: `etb_apnap_batch_r60_test.go` (7 references), `trigger_batch.go` (12 self-references), `trigger_batch_test.go` (16), `etb_dispatch.go` (3), `stack.go` (4), `state.go` (2), `phases.go` (1), `per_card_hooks.go` (1), `zone_change.go` (1). The 4 `stack.go` references are spell-resolution boundaries; the `etb_dispatch.go` 3 are the ETB phase driver; remaining ones are zone/phase/permanent-enter boundaries — all correct.

**Zero per_card files** call `BeginTriggerBatch` directly. All trigger pushes go through `PushTriggeredAbility(gs, perm, effect)` which respects the batch flag transparently. The two `pendingTriggers` direct-access sites — `multiplayer.go:503-516` (seat-elimination purge per §603.3b) and `stack.go:1062` (reentry guard) — are legitimate system-level operations, not duplication.

**No consolidation needed.** This is a healthy subsystem — call out as a positive control for what the other surfaces should aspire to.

---

## 4. Counter Logic Duplication

**Engine surface (canonical):** `counters.AddCounters(target, kind, n, source)` from the Counter DB. `CounterStacks` field on `Permanent` (state.go:1416) carries source-attributed instance data; legacy `Counters map[string]int` (state.go:1406) coexists during migration.

**Duplication shape — direct `perm.Counters[kind] = N` assignments.** Verified counts via grep: 46 `perm.Counters[` mutations + 43 `p.Counters[` mutations across 10 engine `.go` files (89 total sites, mostly initialization-shaped). Production sites of note:

| File:Line | Purpose | Migration status |
|---|---|---|
| `sba.go:1205-1206` | §704.5q +1/+1 vs −1/−1 annihilation | **Phase 6 dual-path:** runs both `counters.PairRemoveSBA` AND legacy `Counters[]` write (comment at L1192 acknowledges intentional) |
| `resolve.go:1779` | megamorph +1/+1 grant | inline map write |
| `stack.go:1508,1516` | planeswalker loyalty / battle defense initialization | inline map write |
| `etb_dispatch.go:188` | saga chapter counter init | inline map write |
| `keywords_battle.go:162` | battle defense counter mutation | inline map write |
| `keywords_batch2.go:284` | haunt source-seat storage | utility counter (semi-legitimate map use) |
| `keywords_batch6.go:533` | planeswalker activation clearing loyalty | inline map write |
| `chaos.go:332-333` | chaos +1/+1 baseline | inline map write |
| `resolve_helpers.go:2475` | level-up level counter init | inline map write |
| `clone.go:65` | copy snapshot counter replication | inline map write |

**Per_card layer:** mostly clean — handlers use `AddCounter` / `RemoveCounter` helpers. Test-only files reach into `Counters["flying"]` etc. for assertions, which is fine.

**The real hazard:** every initialization site above stamps `Counters[]` without a corresponding `CounterStacks` push, so source-attribution is missing for ~all sagas, planeswalkers, megamorph creatures, battles, levelers, and clones. Phase 6+ counter-doublers (Primal Vigor, Branching Evolution, Vorinclex) work correctly for *modifications* (via `AddCountersToPermanent`) but the *initial* counter is invisible to the doubler chain when stamped via direct map.

**Consolidation:** introduce `counters.InitializeCounter(target, kind, count, sourceID)` that synchronously writes both `Counters[]` and `CounterStacks` with a stamped source-instance. Migrate the 10 listed initialization sites. After that, the legacy dual-path at `sba.go:1190-1215` can be retired in a follow-up. Estimated LOC delta: **+30 / −60 = ~−30 net** plus large correctness win (counter-doublers finally observe ETB-stamped counters).

---

## 5. Replacement-Effect Application

**Finding: NO double-application detected. Pipeline is clean.**

Central dispatcher `FireEvent(gs, ev)` at `replacement.go:312-364` with `pickReplacement` loop + 25-iteration §616.1f cap. Token-doubling sources (Doubling Season, Anointed Procession, Parallel Lives) and counter-doubling sources (Primal Vigor, Branching Evolution, Vorinclex — registered in `counter_doublers.go:48-85`) all route through a single `FireEvent` chain per event. Audit trail recorded in `PendingTokenMintChain` (InstanceID Phase 5) and via `StampDoublingSeasonAudit` (`counter_doublers.go:386-400`).

Damage-replacement: `ApplyDamageReplacement(gs, ctx)` at `damage_replacement.go:129` called from combat sites (`combat.go:1628, 1795`) and inline from `DealDamage` (`state.go:2259`). Single canonical path; no per_card handler re-applies.

**No structural duplication.** One latent hazard documented in §4: counter-doubler replacements apply correctly on `would_put_counter` events, but ETB-stamped initial counters never *fire* the event because initialization sites bypass `counters.AddCounters`. Closing §4 closes this implicit hazard too.

---

## 6. State-Tracking Duplication on `Permanent`

**Finding: All dual-path fields are documented, time-bounded Phase-N migrations with invariant gates. No orphans.**

| Field pair | Location | Status |
|---|---|---|
| `LinkedExile []*Card` vs `ExiledByMe []string` + `LinkageKind` | state.go:1470 vs 1485-1493 | InstanceID Phase 4 migration window. `ExileLinkageIntegrity` invariant at `invariants.go:1921-1980` validates both shapes in parallel. Phase 4 retire planned. |
| `Counters map[string]int` vs `CounterStacks []counters.CounterStack` | state.go:1406 vs 1416 | Counter DB Phase 1 migration window. Dual-consumption acknowledged at `sba.go:1190-1215`. See §4 for closure path. |
| `MergedCards`, `MergeKind`, `TopCard`, `MergedCardPtrs` | state.go:1549-1574 | InstanceID Phase 8 fresh; no legacy field to retire. |
| `CopyMechanisms []CopyMechanism` | state.go:1510 | Modern Phase 5 registry. **No legacy `CopyClass` flag exists** (audit confirmed). |

**GameState side:** searched for parallel maps (`gs.Linkage`, `gs.PrevID`, `gs.CardMap`, exile trackers). None exist. The original 2026-05-28 7174n1c brief asked about `ExiledByMe vs old linkage map` — confirmed there is no old GameState-level linkage map; exile linkage is fully Permanent-held per design v2 §7.

**No consolidation needed on Permanent state fields.** The dual-path situations are scheduled-for-retire under Phase 4 (linkage) and via the §4 consolidation here (counters). Worth a follow-up audit after each phase closure.

---

## Top-10 Worst Offenders (Consolidation Recommendations)

Ranked by **(estimated LOC affected) × (correctness impact)**. Each gets a proposed cleanup-PR shape.

### 1. Manual zone-append sweep (50 per_card files)

- **PR title:** `refactor(per_card): MoveCardToZone sweep — eliminate 150+ manual zone appends`
- **Files touched:** export `MoveCardToZone(gs, card, fromZone, toZone, owner, source)` in `zone_change.go`; sweep ~50 per_card files
- **LOC est:** −480 / +180 = **−300 net**
- **Correctness payoff:** fixes silent descend/died/exiled counter miscount; fires zone-change triggers for Tergrid/Sefris/Bolas's Citadel/descend payoffs on currently-silent paths
- **Risk:** moderate — many files, requires verification each call site identifies the right `fromZone`
- **Sequencing:** split into two PRs by zone (graveyard appends first, library/hand/exile second) to keep diff reviewable

### 2. Counter initialization API (10 engine files)

- **PR title:** `feat(counters): InitializeCounter — close ETB/spell-resolution counter dual-path`
- **Files touched:** `internal/counters/*.go` (new API); `sba.go`, `resolve.go`, `stack.go`, `etb_dispatch.go`, `keywords_battle.go`, `keywords_batch6.go`, `chaos.go`, `resolve_helpers.go`, `clone.go`
- **LOC est:** +30 new API / −60 migrated sites = **−30 net**
- **Correctness payoff:** counter-doublers finally observe saga/planeswalker/battle/megamorph initial counters; one-off ETB-stamped counter-targeted triggers fire correctly
- **Risk:** low — central API, all call sites are well-localized
- **Follow-up:** retire `sba.go:1190-1215` legacy dual-path in a separate PR after this lands

### 3. Predefined-token inline-mint cleanup (8 sites across 7 files)

- **PR title:** `refactor(tokens): retire inline Treasure/Clue mint sites — use canonical helpers`
- **Files touched:** `tokens.go` (add `MintPredefined` switch helper); `dockside_extortionist.go`, `malcolm.go`, `edward_kenway.go`, `grim_hireling.go`, `the_ghoul_gunslinger.go`, `mr_house_president_and_ceo.go`, `the_rani.go`
- **LOC est:** **−55 net**
- **Correctness payoff:** consistent `Turn.TreasuresCreated`/`Turn.TokensCreated` updates; `currentMintEnablerID` plumbed everywhere
- **Risk:** very low — pure delegation

### 4. Face-down token shared helper (2 sites in `resolve_helpers.go`)

- **PR title:** `refactor(resolve): extract createFaceDownToken — dedupe manifest + manifest dread`
- **Files touched:** `resolve_helpers.go` only
- **LOC est:** **−30 net**
- **Correctness payoff:** future face-down-token edge cases only need to be fixed once
- **Risk:** negligible

### 5. Direct-Lost setter cleanup (1 site)

- **PR title:** `fix(per_card): batch_aj_win_the_game — route through MarkSeatLostByEffect`
- **Files touched:** `multiplayer.go` (extract `MarkSeatLostByEffect` from `resolveLoseGame` body); `batch_aj_win_the_game.go`
- **LOC est:** +15 / −5 = **+10 net** (mostly extraction overhead)
- **Correctness payoff:** Platinum Angel + Angel's Grace properly cancel the loss; `LossReason` + `LostByEffect` flag stamped
- **Risk:** low — surgical fix, one card

### 6. `moveCardBetweenZones` direct-use audit (30 sites)

- **PR title:** `refactor(per_card): replace moveCardBetweenZones direct-use with MoveCardToZone wrapper`
- **Files touched:** 30 per_card files (overlaps partially with PR #1)
- **LOC est:** **−60 net** (after PR #1 lands, ~10 remain — most route to graveyard via paths #1 also catches)
- **Sequencing:** do AFTER PR #1; many sites get consolidated into the same canonical call

### 7. `mr_house` internal inconsistency (1 file)

- **PR title:** `refactor(per_card): mr_house — unify Treasure mint arms`
- **Files touched:** `mr_house_president_and_ceo.go`
- **LOC est:** **−10 net**
- **Note:** the file calls BOTH the inline pattern (L101) AND `CreateTreasureToken` (L142). Pick the canonical one — kill the inline arm. **Folds naturally into PR #3.**

### 8. Stale CLAUDE.md issue-log entries

- **PR title:** `docs(claude-md): retire stale per_card sweep callouts`
- **Files touched:** `CLAUDE.md` (issue log Open/Resolved tables)
- **LOC est:** ~20 lines of doc reorg, **net 0 LOC**
- **Payoff:** keeps the issue log trustworthy. Two stale rows identified by this audit:
  - 2026-05-25 "4 of 5 sibling sites still standing — etrata x2, bilbo x1, thassa x1" — all 4 actually closed
  - 2026-05-28 "9 per_card direct-Lost setters" — 7 of 9 actually fixed already, only 1 + 1-intentional remain
- **Risk:** none — pure cleanup

### 9. `FireZoneChangeTriggers` parameter drift (17 files)

- **PR title:** `refactor(per_card): centralize FireZoneChangeTriggers via MoveCardToZone`
- **Status:** **subsumed by PR #1.** Not a standalone PR — listing here for visibility. The 17 manual `FireZoneChangeTriggers` calls all become internal to `MoveCardToZone` after the wrapper lands.

### 10. Counter-DB dual-path retirement (deferred)

- **PR title:** `refactor(counters): retire legacy Counters map writes in sba annihilation`
- **Files touched:** `sba.go`, possibly `counters/*.go`
- **LOC est:** **−15 net**
- **Sequencing:** **must come AFTER PR #2** (init API) and after a sweep to confirm zero per_card handlers still write `Counters[]` directly. Tracked separately, not part of this initial wave.

---

## Proposed Cleanup-PR Sequence

**Wave 1 (low-risk, high-leverage):**
1. **PR #3** — Predefined-token inline-mint cleanup (−55 LOC, very low risk)
2. **PR #4** — Face-down token shared helper (−30 LOC, negligible risk)
3. **PR #5** — Direct-Lost setter cleanup + `MarkSeatLostByEffect` extraction (+10 LOC, real correctness win)
4. **PR #8** — Stale CLAUDE.md issue-log cleanup (doc only)

**Wave 2 (medium-risk, biggest payoff):**
5. **PR #1a** — Export `MoveCardToZone`, sweep graveyard appends (~−180 LOC subset)
6. **PR #1b** — Sweep library/hand/exile appends (~−120 LOC subset)

**Wave 3 (counter-system migration close-out):**
7. **PR #2** — `counters.InitializeCounter` API + 10-site migration (−30 LOC, large correctness win)
8. **PR #10** — Retire `sba.go` legacy Counters-map dual-path (−15 LOC; only after PR #2 + verification sweep)

**Total estimated LOC delta across all waves: ~−540 net**, with the bulk coming from PR #1 (manual zone-append sweep). Correctness wins concentrated in PR #1 (zone-change observer triggers), PR #2 (counter-doubler initial-counter observation), and PR #5 (Platinum Angel cancellation on batch_aj cards).

---

## What Is NOT Worth Pasturing

For completeness, the following were investigated and found to be **healthy or intentional**:

- **Trigger-batch system** — properly consolidated, zero per_card bleed (§3).
- **Replacement pipeline** — single canonical `FireEvent` path, no double-apply (§5).
- **Permanent dual-path state fields** — documented Phase-N migration windows with invariant gates (§6).
- **`CreateCreatureToken` inline `&Card{}` construction in per_card** — creature tokens are heterogeneous by design; the helper exists for callers that want it, but inline construction is acceptable and accounts for 124 of 181 token calls. NOT a duplication target.
- **The two `pendingTriggers` direct accesses** in `multiplayer.go` (seat elimination) and `stack.go` (reentry guard) — legitimate system-level operations, not duplication.

---

## Verification Method

- File-system surveys via `find` + `grep` on the worktree
- Cross-referenced against CLAUDE.md issue-log entries 2026-05-23 → 2026-05-28
- Spot-verified file:line citations for top offenders (etrata/bilbo/thassa, batch_aj_win_the_game, sba.go counter dual-path, tokens.go helper surface)
- Three parallel sub-audits (token, zone-change, trigger/counter/replacement/state) merged and cross-checked
- No code modified; report only

**Next action:** await go-ahead on cleanup-PR sequence; recommend Wave 1 ships first as it is fully no-risk LOC reduction with one correctness fix (PR #5).
