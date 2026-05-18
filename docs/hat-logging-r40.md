# YggdrasilHat Decision Logging Audit — R40

**Scope:** `internal/hat/` — verify each AI decision logs enough context for post-game analysis (deck archetype, board state, options considered, scores, chosen option, budget tier).
**Date:** 2026-05-18
**Branch:** `dev/hat-logging-r40`
**Worktree:** `/private/tmp/hexdek-hat-logging-r40`

---

## TL;DR

The hat had **three** logging channels — `h.logf` (in-memory `DecisionLog`, nil in tournament play), `gs.LogEvent` (engine event log, persistent), and the `convictionRing` (process-wide ring for live admin inspection). Pre-R40, only `CAST`, `ACTIVATE`, and combat `ATTACK` decisions emitted meaningful traces; **mulligan, blocker assignment, attack-target selection, and modal-choice decisions were completely silent**, and the conviction sampler dropped the hat's archetype belief on the floor. Post-game analyzers like heimdall could see the *outcome* of those calls (in the game state) but not the *reasoning*.

R40 closes those four gaps and adds the missing archetype field to the conviction event. Five fixes shipped, four tests added, all hat + gameengine tests green.

---

## Pre-R40 Logging Surface (verified)

| Channel | Function | Persistence | Used by |
|---|---|---|---|
| `h.logf(fmt, …)` | `yggdrasil.go:2635` | In-memory `*h.DecisionLog`, nil in prod | Test harness only |
| `gs.LogEvent(Event)` | engine | `gs.EventLog` (cap 50000); harvested post-game by tournament runner | heimdall, tournament runner |
| `pushConvictionEvent(e)` | `conviction_telemetry.go:99` | Process-wide ring (1024 events); exposed via `/api/admin/conviction-events` | Admin/live debug |
| `h.recordDecisionTier(tier)` | `yggdrasil.go:1858` | In-memory counter; exposed via `MjolnirStats()` | Telemetry summary |

**Important:** `DecisionLog` is in-memory only and is `nil` in tournament play. Persistent decision capture relies on `gs.LogEvent`. Pre-R40, the hat used `LogEvent` only for the conviction sampler and a few combat-time events.

---

## Decision Catalog (post-R40)

| Decision | Function (`file:line`) | Pre-R40 trace | Post-R40 trace |
|---|---|---|---|
| Mulligan | `ChooseMulligan` (`yggdrasil.go:2669`) | none | `hat_decision_mulligan` event + `logf` (lands/combo/ve/stars/cuttables/decision/archetype/turn) |
| Land play | `ChooseLandToPlay` (`yggdrasil.go:2967`) | none | unchanged — deferred (see "Out of scope") |
| Cast selection | `ChooseCastFromHand` (`yggdrasil.go:3126`) | `logf` (position, interaction risk, combo lines) | unchanged |
| Activation | `ChooseActivation` (`yggdrasil.go:3536`) | `logf` + UCB1 stats | unchanged |
| Attacker selection | `ChooseAttackers` (`yggdrasil.go:3840`) | `logf` (stance, threshold, per-attacker tags, prune reasons, final count) | unchanged |
| Attack target | `bestTarget` (`yggdrasil.go:1189-1614`) | none | `hat_decision_attack_target` event + `logf` (chosen seat/score, top 3 candidates + scores, archetype) |
| Blocker assignment | `AssignBlockers` (`yggdrasil.go:4351-4865`) | none | `hat_decision_block` event + `logf` (attackers, blockers committed, incoming, residual life, poison, existential commander, archetype) |
| Spell target | `ChooseTarget` (`yggdrasil.go:5020`) | `logf` (tutor score, removal-targeted candidates) | unchanged |
| Modal choice | `ChooseMode` (`yggdrasil.go:5293-5322`) | none | `hat_decision_mode` event + `logf` (chosen idx/score, top 4 modes + scores, position, archetype) |
| Conviction sample | `recordConvictionSample` (`conviction.go:70`) | `conviction_diagnostic` event + ring | now stamped with `Archetype` field on both channels |

---

## Post-Game Reconstruction Matrix (post-R40)

For each of the six analyst questions:

| Artifact | Pre-R40 | Post-R40 |
|---|---|---|
| (a) Hat's belief about its own archetype | implicit / partial | **explicit** — stamped on every hat_decision_* event and on every conviction sample |
| (b) Opponent archetypes (3rd Eye) | partial — only surfaces in combat logs when `prof.Confidence > 0.5` (`yggdrasil.go:1567, 5153`) | unchanged (out of scope; flagged below) |
| (c) Board state hash / signature | none at decision time | partial — turn + life + incoming + permanent counts embedded in mulligan/block events; full board hash deferred |
| (d) Candidate options + scores | only CAST / ACTIVATE | **CAST, ACTIVATE, ATTACK_TARGET, MODE** all log top-N candidates with scores; MULLIGAN and BLOCK log decision context but not "options not taken" (mulligan is a binary; block enumeration would be too large) |
| (e) Chosen option + final score | partial | **all four new events stamp chosen + score** |
| (f) Budget tier + complexity override | tracked in `MjolnirStats()` aggregate | unchanged (per-decision tier still aggregate-only — see "Out of scope") |

---

## Fixes Shipped

### 1. `ConvictionEvent.Archetype` field (HIGH)
- **File:** `internal/hat/conviction_telemetry.go` (new field), `internal/hat/conviction.go` (populated from `h.Strategy.Archetype` on both the `gs.LogEvent` and the `pushConvictionEvent` call sites).
- **Why:** Pre-R40, an analyst correlating "did win-line-extinct fire disproportionately on combo decks?" had to rejoin the conviction ring against the deck index. Now every sample is self-describing.
- **JSON shape:** `"archetype":"combo"` with `omitempty` so older clients see the field absent rather than wrong.

### 2. `emitDecisionEvent` helper (foundation)
- **File:** `internal/hat/yggdrasil.go:2641` (next to `logf`).
- **Contract:** stamps every persisted hat decision with `archetype` (from `h.Strategy`) and `turn` (from `gs.Turn`) so each event row is self-describing. Nil-safe on `gs` and on `details`. Empty archetype string when no strategy is loaded (honest "no belief" rather than a fabricated default).

### 3. Mulligan logging (HIGH)
- **File:** `ChooseMulligan` in `internal/hat/yggdrasil.go:2669`.
- **Shape:** Named return + `defer` so all 8 existing return paths participate in the emitted event without threading a reason string through each branch.
- **Fields:** hand_size, lands, combo, value_engines, stars, cuttables, mulliganed, archetype, turn.
- **Why this shape:** the hand-stat helper (`handStatsForLog`) is extracted to keep the deferred line readable; recomputing hand stats inside the defer is cheap (bounded hand size).

### 4. Blocker assignment summary (HIGH)
- **File:** `AssignBlockers` in `internal/hat/yggdrasil.go:4351-4865`, log added at the post-loop `return out` site.
- **Fields:** attackers, attackers_blocked, blockers_committed, residual_incoming, life_before, residual_life, poison_added, existential_commander, archetype, turn.
- **Why this shape:** the analyst's first question is "did the hat trade efficiently or chump-block badly?" The summary row answers that without re-simulating the combat step. Per-blocker detail is left out — it's reconstructible from the resulting `damage` events on the same turn.

### 5. Attack-target candidate pool (MED)
- **File:** `bestTarget` in `internal/hat/yggdrasil.go:1602` (after the candidate sort, before `selectAmongTop`).
- **Fields:** chosen_seat, chosen_score, top_seats (top 3), top_scores (top 3, aligned), candidate_n, archetype, turn.
- **Why top 3:** 4-player Commander caps the eligible-defender pool at 3 (you can't attack yourself). Logging all candidates is exhaustive without bloating the event log.

### 6. Modal-choice scoring (MED)
- **File:** `ChooseMode` in `internal/hat/yggdrasil.go:5293-5322`, log added after the score sort.
- **Fields:** chosen_idx, chosen_score, top_indices (top 4), top_scores (top 4), mode_count, position, archetype, turn.
- **Note:** `chosen_idx` is the index into the *original* mode list (what the engine sees), not the sorted position. Top_indices is sorted by score desc, so analysts can read "the hat picked idx=2 because mode 2 scored 0.42 vs. mode 0 at 0.31".

---

## Tests Added

`internal/hat/decision_logging_test.go` (new, 4 cases):

| Test | What it locks |
|---|---|
| `TestConvictionEvent_ArchetypeRoundTrip` | New `Archetype` field round-trips intact through the conviction ring; empty value omitted (omitempty contract) |
| `TestEmitDecisionEvent_StampsArchetypeAndTurn` | Helper stamps `archetype` (from Strategy) and `turn` (from gs) on every event without caller having to remember |
| `TestEmitDecisionEvent_NoStrategyEmitsEmptyArchetype` | Honest "no belief" empty string when no strategy is loaded; does not fabricate a label |
| `TestEmitDecisionEvent_NilGameStateIsNoop` | Defensive guard: nil gs does not panic |

Full hat + gameengine + per_card test suites green post-changes.

---

## Out of Scope (Flagged for Future Rounds)

- **Per-decision budget tier label.** `MjolnirStats()` aggregates tier counts but the individual hat_decision_* events don't yet record which tier was in effect. A 1-line field on `emitDecisionEvent` would close it.
- **Board state signature / hash.** A stable hash (permanent count + life totals + commander damage clocks + zone sizes) attached to every decision event would let analysts dedupe and bucket identical positions. Out of scope here because hash design itself wants its own round.
- **Opponent profile snapshot.** Currently the 3rd-Eye output only surfaces in combat decisions where `prof.Confidence > 0.5`. A snapshot field (`{seat:archetype:confidence:threat}` quartet) on every decision event would close the "what did the hat *think* about each opponent at this moment" gap.
- **Land-play decision (`ChooseLandToPlay`).** Still silent. Lower priority than the four shipped here — color-demand analysis is largely deterministic from deck list, but ramp-vs-utility tradeoffs would benefit.
- **Spell-target candidates.** `ChooseTarget` logs the chosen target via `TUTOR` line but not the rejected ones. Same gap as attack target, lower analytic value.

---

## Verdict

The four shipped decision sites and the conviction archetype field were the highest-ROI gaps — mulligan and blocker decisions in particular were entirely opaque pre-R40 and had high diagnostic value (mulligan = deck-fit signal; blocker = archetype-telling). The `emitDecisionEvent` helper now establishes a consistent contract every future decision site can adopt with one line.

---

## Verdict Matrix

| Claim | Pre-R40 | Post-R40 | Evidence |
|---|---|---|---|
| Mulligan decisions are analyzable | ❌ silent | ✅ `hat_decision_mulligan` | `yggdrasil.go:2670-2691` (deferred emit) |
| Blocker assignments are analyzable | ❌ silent | ✅ `hat_decision_block` | `yggdrasil.go:4866-4895` |
| Attack-target choices show alternatives | ❌ chosen only | ✅ top 3 + scores | `yggdrasil.go:1615-1640` |
| Modal-choice scoring is visible | ❌ silent | ✅ top 4 + scores | `yggdrasil.go:5322-5347` |
| Conviction samples carry archetype | ❌ dropped | ✅ field on event + ring | `conviction.go:143-155`, `conviction_telemetry.go:60-67` |
| Every hat-decision event is self-describing | n/a | ✅ archetype + turn stamped uniformly | `yggdrasil.go:2641-2675` |
