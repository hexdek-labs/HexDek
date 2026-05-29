# Counter DB — System Complete (r60)

Status: **COMPLETE** as of Counter DB Phase 8 (Energy + Experience Seat-resource carveout).

The Counter DB is the §122 / §306 / §714 / §106.11 counter-handling subsystem
that replaced HexDek's ad-hoc per-card counter wiring with a registry-driven
pipeline: a canonical type registry, a doubling-replacement walk, a proliferate
primitive, pair-removal SBAs, and Seat-resource carveouts for the two non-§122
"counter-shaped" pools (energy + experience).

The system underwrites every counter interaction in the engine — +1/+1 stacking,
Doubling Season / Hardened Scales / Primal Vigor / Vorinclex amplification,
proliferate target picking, loyalty / lore / time / stun / shield / oil
trackers, the 230-card long-tail of per-card counters, and the §106.11 / Phase-8
resource-pool carveouts for energy and experience.

This doc is the completion ledger: every phase, its PR, its scope, and the load-
bearing CR §-citations the phase pinned. Reading top-to-bottom gives the system's
biography from "no registry" to "230+ types, full proliferate pipeline,
Seat-resource carveouts enforced".

---

## Phase-by-phase ledger

| Phase | PR    | Scope                                                                 | CR §-citations                              |
|-------|-------|-----------------------------------------------------------------------|---------------------------------------------|
| 1     | #751  | Foundation — registry shell + 10 most-common counter types            | §122, §306, §714, §702.61                   |
| 2     | #752  | Keyword grants (lifelink/deathtouch/flying/…) + §122.1c persistence   | §122.1c, §122.6, §702.x (per keyword)       |
| 3     | #754  | Pair-removal SBA per §704.5q                                          | §704.5q, §704.5r                            |
| 4     | #756  | Proliferate primitive + player counter family (poison/exp/rad) + InstanceID Phase 4 | §701.27, §122.1d, §810, §106.11        |
| 5     | #757  | Long-tail 230 counter types (alt-win, oil, finality, mannequin, etc.) | §122, §614, §711, §701.55, §701.58          |
| 6     | #761  | Doubling pipeline + §122.1g integration + §306.5g loyalty doubling    | §122.1g, §306.5g, §614.5                    |
| 7     | #763  | Sagas + Battles counter handling                                      | §714, §310, §306.5g                         |
| 8     | (this PR) | Energy + Experience Seat-resource carveout (FINAL)                | §106.11, §122.1g (negative — exclusion), §701.27 (negative) |

---

## Type registry size

| Source                                | Entries |
|---------------------------------------|---------|
| `registry_init.go` (Phase 1/2/4 core) | 23      |
| `registry_longtail.go` (Phase 5)      | 227     |
| **Total registered counter types**    | **250+** |

Excluded from the registry by design (Phase 8 / §106.11 carveout):
- **energy** — CR §106.11 resource pool; lives at `Seat.EnergyCounters`
- **experience** — Phase 8 Seat-resource analog; lives at `Seat.XPCounters`

Both excluded types return `false` from `counters.IsProliferateEligibleType`
and `counters.ErrUnknownCounterType` from `counters.AddCountersWithDoublers`,
so the doubling pipeline and the proliferate primitive both refuse to touch
them. Per-card handlers route through `gameengine.GainEnergy` / `PayEnergy` /
`GainXP` (which keep the typed `Seat.EnergyCounters` / `Seat.XPCounters`
mirrors in sync with the legacy `Flags["energy_counters"]` /
`Flags["experience_counters"]` storage).

---

## §-citation index

The Counter DB pins these CR sections via load-bearing tests (one test per
section, asserting the rule's observable effect):

- **§106.11** — Energy is a resource pool, not a §122 counter. Tests:
  `TestPhase8EnergyAbsentFromRegistry`, `TestPhase8EnergyNotProliferateEligible`,
  `TestPhase8EnergyApplyDoublersIdentity`, `TestPhase8EnergyAddCountersRejected`.
- **§122** — Generic §122 counter shape. Tests across `counters_property_test.go`,
  `longtail_test.go`.
- **§122.1c** — Keyword counter grants. Tests: `keyword_grants_test.go`,
  `TestEveryKeywordCounterHasCRCitation`.
- **§122.1d** — Poison counters on players. Test: `TestPlayerCountersNoDoubling`
  (negative — DS does NOT double poison), `TestProliferate_PoisonEligible`.
- **§122.1g** — Doubling Season replacement effect. Tests: `doubling_test.go`,
  `TestPhase8ExperienceApplyDoublersIdentity` (Phase 8 negative).
- **§122.6** — Counter persistence through type changes. Test:
  `counters_property_test.go`.
- **§306** — Loyalty counters. Test: `doubling_test.go` (Doubling Season +
  planeswalker ETB).
- **§306.5g** — Loyalty doubling at ETB routes through §122.1g pipeline.
- **§310** — Battles. Phase 7 tests in `sagas_battles_test.go`.
- **§614** — Replacement-effect counters (finality, mannequin, incarnation,
  echo, paralyzation, pin). Tests in `longtail_test.go`.
- **§614.5** — Self-replacement counter doubling order. Test: `doubling_test.go`.
- **§701.27** — Proliferate. Tests: `proliferate_test.go`,
  `TestPhase8EnergyNotProliferateEligible`, `TestPhase8ExperienceNotProliferateEligible`.
- **§701.55** — Stun counter untap replacement.
- **§701.58** — Shield counter destruction / damage replacement.
- **§702.x** — Per-keyword grant cites (one per keyword counter).
- **§704.5q** — `+1/+1` ↔ `-1/-1` pair-removal SBA. Test: `pair_removal_test.go`.
- **§704.5r** — Only `+1/+1` and `-1/-1` cancel.
- **§711** — Level-up counter brackets. Test: `longtail_test.go`.
- **§714** — Sagas + lore counters. Phase 7 tests.
- **§810** — Two-Headed Giant poison cap. Test: `TestPlayerCountersNoDoubling`.

---

## Phase 8 — implementation summary

Phase 8 enforces the Probe F design decision that **energy** (§106.11) and
**experience** (per the Seat-resource framing) are NOT §122 counters. The
narrowed §122 generic player-counter family is now exactly **poison + rad**.

Changes shipped in this PR:

1. **`internal/gameengine/counters/registry_init.go`** — removed the
   `experience` `CounterTypeDef`. Comment expanded to document both the
   §106.11 energy exclusion (carried over from Phase 4) and the Phase 8
   experience carveout, with rationale (keeping the registry to exactly the
   proliferate / §122.1g-eligible set so AI target pickers and per_card
   handlers' registry probes are load-bearing for legality).

2. **`internal/gameengine/energy.go`** — `GainEnergy` and `PayEnergy` now keep
   `Seat.EnergyCounters` (typed mirror) in sync with `Seat.Flags["energy_counters"]`
   inline on every mutation. Doc updated to surface the §106.11 + §122.1g
   exclusion semantics.

3. **`internal/gameengine/xp.go`** — new canonical XP helpers:
   - `GainXP(gs, seat, amount)` — bypasses §122 pipeline, mirrors typed field.
   - `GetXP(gs, seat)` — returns canonical Flags value.
   - `SyncSeatResourcePools(gs)` — backfills both typed mirrors from Flags
     storage for legacy direct-Flags writes used by per_card handlers wired
     in Phases 1–7.

4. **`internal/gameengine/proliferate.go`** —
   - `BuildGreedyProliferateTargets` no longer emits an experience choice for
     the controller (Phase 8 carveout).
   - The engine-level `Proliferate` wrapper now pre-filters targets through
     `counters.IsProliferateEligibleType`, so a stray ineligible kind in a
     per_card-built target list no longer short-circuits subsequent valid
     choices via the primitive's first-error abort.

5. **Property tests** —
   - `internal/gameengine/counters/energy_xp_carveout_test.go` (10 tests):
     registry absence, proliferate exclusion, AddCountersWithDoublers
     rejection, ApplyDoublers identity, and the narrowed-§122-player-counter
     family pin (poison + rad).
   - `internal/gameengine/energy_xp_seat_sync_test.go` (6 tests): typed-mirror
     sync on gain / pay / failed-pay, XP sync on gain, `SyncSeatResourcePools`
     backfill, `GetXP` reads canonical Flags.

6. **Existing test updates** — `TestProliferate_ExperienceEligible` flipped
   to `TestProliferate_ExperienceExcluded`; `TestPlayerCountersNoDoubling`
   narrowed to {poison, rad}; `TestProliferate_AllPlayerCounterTypes`
   (engine-level) updated to assert experience stays unchanged across a
   proliferate event.

### Cards covered

Phase 8's structural carveout retroactively governs all energy- and
XP-producing/consuming per_card handlers from prior phases. Energy: ~30+
Kaladesh-block / MH3 cards (Aether Hub, Glimmer of Genius, Servant of the
Conduit, Aetherborn Marauder, Aethersphere Harvester, Harnessed Lightning,
Dynavolt Tower, Whirler Virtuoso, Bristling Hydra, Rogue Refiner, Rishkar,
Peema Renegade, Live Fast, Era of Innovation, Empyreal Voyager, Hum of the
Radix, Decoction Module, Energy Refractor, Energy Tap, Era of Enlightenment,
Inventor's Apprentice, Multiform Wonder, Dr. Madison Li, Rex Cyber Hound,
Satya Aetherflux Genius, Saheeli Radiant Creator, Volatile Stormdrake,
Yawgmoth Thran Physician, and the broader §106.11 mechanic set). Experience:
Daxos the Returned, Mizzix of the Izmagnus, Daretti Scrap Savant, Ezuri Claw
of Progress, Meren of Clan Nel Toth, Vish Kal Blood Arbiter, Toph Earthbending
Master, Katara Waterbending Master, Minthara Merciless Soul, Otharri Suns'
Glory, Azlask Swelling Scourge, Saskia Unyielding (XP-mode).

None of these handlers needed mechanical changes — they already wrote to
`Seat.Flags["energy_counters"]` / `Seat.Flags["experience_counters"]` directly
(bypassing the §122 registry), and the canonical helpers + typed-field sync
keep the read side coherent.

---

## Closing notes

The Counter DB system is now feature-complete. Future counter work (new
mechanic introductions, new per-card replacement counters, new keyword-counter
families) falls into the long-tail `registry_longtail.go` registration pattern
established by Phase 5 — no further architectural phases are planned.

The companion InstanceID design (#747) drove its own parallel phase rollout
(Phases 1–9). The two systems were intentionally co-developed: Counter DB
needs InstanceID for proliferate-driven counter lineage (PlacedByInstanceID
on every CounterStack), and InstanceID needs Counter DB for the §122 SBA pass
that walks counter-driven creature stats.

Counter DB Phase 8 + InstanceID Phase 9 close the original implementation
plan (#748 / #747). Long live the registry.
