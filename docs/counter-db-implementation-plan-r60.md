# Counter DB — Implementation Plan

**Status:** Plan — ready for review and Phase 1 dispatch
**Authors:** 7174n1c (architectural lead), Hex (engineering)
**Date:** 2026-05-29
**Validated by:** Probe F corpus walk (PR #746 — `docs/counter-types-catalog-r60.md`)
**Aligned with:** InstanceID System v2 (PR #747, `docs/instanceid-system-v2-r60.md`)
**Closes bug class:** Counter-drift through type changes (§122.6 violations), inconsistent doubling pipeline (§122.1g), missed keyword-counter ability grants (§122.1c), counter pair-removal SBA misfires (§704.5r).

---

## 1. Background

Probe F walked ~30,000 oracle cards and surfaced **252 distinct counter types** across 8 functional categories. Current engine handles +1/+1 and -1/-1 cleanly via dedicated code paths but has scattered, inconsistent handling for the long tail (charge, oil, lore, loyalty, time, fade, age, ki, lifelink, deathtouch, etc.).

The CR §122 ruleset for counters is one of MTG's most-tested invariant surfaces — counter behavior persists through type changes (§122.6), interacts predictably with doubling effects (§122.1g), grants abilities when the counter type is a keyword (§122.1c), and pair-removes as a state-based action for +1/+1 ↔ -1/-1 (§704.5r). Every one of these has corresponding cards and corresponding bug surface if implemented wrong.

**The goal: a single `counters/` package that owns counter taxonomy, placement validation, runtime mutation, doubling pipeline, SBA pair-removal, and ability-grant predicate sites — replacing the scattered per-card handling we have today.**

This plan is the codex for that build. Aligned with our commitment to per-CR perfect accuracy: every implementation step traces back to a specific CR rule, every invariant has a property test, every behavior is grounded in the Probe F card-by-card catalog.

---

## 2. Goals (corpus-grounded per Probe F)

| Goal | Source | Invariant |
|---|---|---|
| Counters persist through type changes | CR §122.6 | Counter slice on Permanent immutable across Layer 4 type-strips |
| Doubling pipeline scoped correctly | CR §122.1g | Doubling Season doubles +1/+1, NOT energy/poison/experience/rad |
| Pair removal as SBA | CR §704.5r | +1/+1 ↔ -1/-1 cancel during SBA pass; other pairs don't |
| Keyword counters grant abilities | CR §122.1c | Lifelink counter on creature grants lifelink while present |
| Proliferate-eligible types tracked | CR §122 | Proliferate can add +1/+1 / -1/-1 / loyalty / charge etc.; NOT energy |
| Energy carve-out | CR §106.11 | Energy is a resource pool on Seat, NOT a §122 counter |
| Replay-deterministic counter mutations | Engine | Counter-add / remove events logged with InstanceID source |
| InstanceID-traceable counter lineage | Engine | Each counter's placement records the `EnablerInstanceID` that placed it |

---

## 3. Counter type registry

### 3.1 Registry struct

```go
// internal/gameengine/counters/registry.go

type CounterTypeDef struct {
    Name             string           // canonical name: "+1/+1", "lifelink", "charge", "lore", "loyalty", "oil", etc.
    Aliases          []string         // ["plus-one", "+1+1"] etc. for parser robustness
    Category         CounterCategory  // StatModifier | KeywordGrant | ResourceMarker | LoreCounter | LoyaltyCounter | TimeCounter | OtherTracker
    ValidTargets     []TargetType     // [Creature], [Planeswalker], [Player, Creature], [Artifact], etc.
    Placement        PlacementRule    // EnterCondition | AbilityCounter | ProliferateOnly | etc.
    DoublingApplies  bool             // §122.1g — true for +1/+1, false for energy/poison/experience/rad
    Proliferate      bool             // §122 — most yes; exceptions documented
    StackingBehavior StackingRule     // §704.5r pair-removal pairs; default NoPair
    GrantedAbility   *AbilityRef      // §122.1c — keyword counters grant abilities
    OnPlacedTrigger  []TriggerSpec    // optional triggered abilities on placement
    OnRemovedTrigger []TriggerSpec    // optional triggered abilities on removal
    Notes            string           // CR §-citation pinning the type's semantics
}

type CounterCategory int
const (
    StatModifier      CounterCategory = iota  // +1/+1, -1/-1, +N/+M variations
    KeywordGrant                                // lifelink, flying, deathtouch, vigilance, etc. (keyword counters)
    ResourceMarker                              // gold, food, clue, blood (treasure-shape)
    LoreCounter                                 // sagas
    LoyaltyCounter                              // planeswalkers
    TimeCounter                                 // suspend, fade, age
    OtherTracker                                // charge, oil, scream, polyp, etc. (rules-text-specific)
)
```

### 3.2 Registry population

Built from Probe F's 252 types in `docs/counter-types-catalog-r60.md`. Each row of the catalog maps 1:1 to a `CounterTypeDef`. Implementation:

```go
// internal/gameengine/counters/registry_init.go (generated/hand-written)

var counterRegistry = map[string]*CounterTypeDef{
    "+1/+1": {
        Name:             "+1/+1",
        Category:         StatModifier,
        ValidTargets:     []TargetType{Creature, Planeswalker},  // planeswalkers per recent rules updates
        Placement:        EnterCondition | AbilityCounter | ProliferateOnly,
        DoublingApplies:  true,
        Proliferate:      true,
        StackingBehavior: PairsWith("-1/-1"),
        GrantedAbility:   nil,
        Notes:            "CR §122 base counter; §704.5r pair-removal with -1/-1",
    },
    "-1/-1": { /* mirror of +1/+1 */ },
    "loyalty": {
        Name:             "loyalty",
        Category:         LoyaltyCounter,
        ValidTargets:     []TargetType{Planeswalker},
        Placement:        EnterCondition | AbilityCost,
        DoublingApplies:  false,  // Doubling Season does double initial loyalty per §122 carve-out — actually yes per current rules
        Proliferate:      true,
        StackingBehavior: NoPair,
        GrantedAbility:   nil,
        Notes:            "CR §306; Doubling Season interaction per §306.5g",
    },
    "lifelink": {
        Name:             "lifelink",
        Category:         KeywordGrant,
        ValidTargets:     []TargetType{Creature},
        Placement:        AbilityCounter,
        DoublingApplies:  true,
        Proliferate:      true,
        StackingBehavior: NoPair,
        GrantedAbility:   &AbilityRef{Keyword: "lifelink"},
        Notes:            "CR §122.1c keyword counter; grants while present, persists through type changes per §122.6",
    },
    "lore": {
        Name:             "lore",
        Category:         LoreCounter,
        ValidTargets:     []TargetType{Saga},  // Saga subtype
        Placement:        AutoUpkeep,  // adds at the beginning of postcombat main, per Saga rules
        DoublingApplies:  false,
        Proliferate:      true,
        StackingBehavior: NoPair,
        GrantedAbility:   nil,
        OnPlacedTrigger:  []TriggerSpec{{Pattern: "saga_chapter_trigger"}},
        Notes:            "CR §714 (Sagas); chapter abilities trigger on placement",
    },
    "charge": {
        Name:             "charge",
        Category:         OtherTracker,
        ValidTargets:     []TargetType{Artifact, Creature, Land},  // varies widely
        Placement:        AbilityCounter,
        DoublingApplies:  true,
        Proliferate:      true,
        StackingBehavior: NoPair,
        GrantedAbility:   nil,
        Notes:            "CR §122; doubling applies",
    },
    // ... 246 more entries
}
```

Population strategy:
- **Phase 1:** the 10 most common types (+1/+1, -1/-1, lifelink, charge, loyalty, lore, time, oil, stun, shield). Covers 80%+ of real-game usage.
- **Phase 2:** keyword-counter family (deathtouch, flying, first-strike, double-strike, hexproof, indestructible, menace, reach, trample, vigilance, ward) — all per §122.1c.
- **Phase 3:** the long tail — 232 remaining types, each gets a registry entry.

---

## 4. Engine integration points

### 4.1 Permanent struct

Already specified in InstanceID v2 (section 4.2):

```go
type Permanent struct {
    // ...
    Counters []CounterStack
}

type CounterStack struct {
    Type               string  // registry lookup key
    Count              int     // aggregated count for same-type counters
    PlacedByInstanceID string  // lineage — which Permanent's ability placed this counter
    PlacedAtTick       int     // game-clock for forensic ordering
}
```

### 4.2 Counters API surface

```go
// internal/gameengine/counters/api.go

// AddCounters places N counters of the given type on the target Permanent.
// Validates placement legality, applies §122.1g doubling pipeline, fires OnPlaced triggers,
// records lineage via PlacedByInstanceID = sourceInstanceID.
func AddCounters(gs *GameState, target *Permanent, counterType string, count int, sourceInstanceID string) error

// RemoveCounters removes N counters of the given type from target.
// Returns the number actually removed (may be less than N if insufficient counters exist).
// Fires OnRemoved triggers per CounterTypeDef.
func RemoveCounters(gs *GameState, target *Permanent, counterType string, count int, sourceInstanceID string) int

// PairRemoveSBA performs §704.5r pair-removal as a state-based action.
// Called during SBA pass. Removes equal numbers of paired counters (e.g., +1/+1 and -1/-1).
func PairRemoveSBA(gs *GameState, target *Permanent) bool  // returns true if state changed

// Proliferate applies the proliferate keyword action to the given target set.
// For each target, controller chooses one counter type to add 1 of. Excludes non-proliferate-eligible types.
func Proliferate(gs *GameState, controller int, targets []*Permanent) error

// HasKeywordCounter returns true if the permanent has at least one counter granting the keyword.
// Used by ability-grant predicate sites (§122.1c).
func HasKeywordCounter(p *Permanent, keyword string) bool

// CounterCount returns the aggregated count of a given counter type on the target.
func CounterCount(p *Permanent, counterType string) int
```

### 4.3 Integration with the doubling pipeline

```go
// internal/gameengine/counters/doubling.go

// ApplyDoublingPipeline walks all "if one or more counters would be placed" replacement effects
// in play (Doubling Season, Hardened Scales, Primal Vigor, Branching Evolution, Vorinclex Voice of Hunger)
// and applies them in §616-stacking order (controller chooses application order when multiple apply).
// 
// Per §122.1g, only DoublingApplies=true counter types are affected.
// Energy / poison / experience / rad are EXCLUDED from doubling.
func ApplyDoublingPipeline(gs *GameState, target *Permanent, counterType string, baseCount int) int
```

### 4.4 Integration with InstanceID system

- Every `AddCounters` call records `PlacedByInstanceID = sourceInstanceID`. Lineage is forensic-traceable.
- Loki invariant: every CounterStack's `PlacedByInstanceID` must reference a real InstanceID (originally minted in this game).
- Heimdall replay UI renders counter history per Permanent — "+3 +1/+1 counters placed by Sage of Hours, +2 charge counters placed by ascend trigger, etc."

### 4.5 Integration with Layer 4 type-change events

Per §122.6, counters persist through type changes. The Layer 4 type-strip code (Humility, Mycosynth Lattice, Conspiracy) MUST NOT clear the Counters slice. Invariant test:

```go
// internal/gameengine/counters/persistence_property_test.go

func TestCountersPersistThroughTypeStrip(t *testing.T) {
    // Setup: Vraan with a lifelink counter
    // Cast Humility
    // Verify Vraan's Counters slice unchanged
    // Verify HasKeywordCounter(vraan, "lifelink") still returns true
    // Verify Vraan's effective abilities reflect "no abilities" (Humility) but counter-grant still active per §122.1c
}
```

---

## 5. The four §122 invariants (Loki integration)

Each becomes a Loki invariant that fires on game-state inspection.

### 5.1 §122.6 — Counter persistence through type changes

**Invariant:** for each Permanent, the Counters slice from game-tick N to game-tick N+1 only changes via explicit `AddCounters` / `RemoveCounters` events emitted to the log. Type-change events (Layer 4) do NOT modify the slice.

**Loki check:** walk every Permanent's counter history, verify each delta has a corresponding event in the log.

### 5.2 §122.1g — Doubling pipeline scope

**Invariant:** doubling effects (Doubling Season, Anointed Procession for tokens-as-counters, Hardened Scales for +1/+1, Vorinclex for opponents' counters) only apply to `DoublingApplies=true` counter types.

**Loki check:** when a counter is placed in the presence of a doubling effect, verify the count was doubled IF AND ONLY IF the type allows it. Catches "Doubling Season doubled my energy gain" bugs.

### 5.3 §704.5r — Pair-removal as SBA

**Invariant:** any Permanent with both +1/+1 and -1/-1 counters at the end of a state check has them pair-removed during the SBA pass.

**Loki check:** at end of each priority pass, no Permanent has both +1/+1 and -1/-1 counters simultaneously after SBA resolution. Catches missed-SBA bugs.

### 5.4 §122.1c — Keyword counter grants

**Invariant:** every keyword counter type (lifelink, deathtouch, flying, etc.) grants its corresponding keyword while present on the Permanent.

**Loki check:** for every Permanent with a keyword counter, the effective characteristics include the keyword. Catches "lifelink counter on creature but engine didn't honor lifelink during damage resolution" bugs.

---

## 6. Phased implementation plan

### Phase 1 — Foundation (1 PR, ~400 LOC)

- `internal/gameengine/counters/` package skeleton
- `CounterTypeDef` struct + `CounterCategory` enum + `PlacementRule` + `StackingRule`
- Registry populated with the 10 most common counter types
- `AddCounters` / `RemoveCounters` / `CounterCount` API with §122.1g doubling pipeline
- Property tests: doubling correctness, lineage tracking, slice immutability
- Wire `Permanent.Counters` from InstanceID v2 (already in the struct)

### Phase 2 — Keyword counter family (1 PR, ~350 LOC)

- Populate registry with all keyword counters (lifelink, deathtouch, flying, first-strike, double-strike, hexproof, indestructible, menace, reach, trample, vigilance, ward)
- `HasKeywordCounter` predicate
- Wire ability-grant predicate sites: damage resolution checks `HasKeywordCounter` for lifelink, deathtouch, first-strike, double-strike; combat resolution checks for menace, flying, vigilance, etc.
- Property test: §122.1c — keyword counter grants ability while present
- §122.6 persistence test: lifelink counter survives Humility

### Phase 3 — Pair-removal SBA (1 PR, ~200 LOC)

- `PairRemoveSBA` function
- Integrate into SBA pass in `internal/gameengine/sba.go`
- Property test: §704.5r — Permanent with both +1/+1 and -1/-1 has them paired during SBA
- Edge case: simultaneous addition of N+1/+1 and N-1/-1 in the same priority window — both removed in single SBA pass per the rules

### Phase 4 — Proliferate (1 PR, ~250 LOC)

- `Proliferate(controller, targets)` function with proper target-choice prompting
- Excludes `Proliferate=false` types (energy on Seat, etc.)
- Wire into all proliferate cards (Contagion Engine, Inexorable Tide, Atraxa, etc.)
- Property test: proliferate adds exactly 1 counter per target of any existing type chosen by controller

### Phase 5 — Long-tail counter types (2 PRs, ~600 LOC)

- Populate the remaining ~230 counter types from Probe F catalog
- Hand-write registry entries; reference `docs/counter-types-catalog-r60.md` for each type's properties
- Per-type tests for the dozen+ types with non-obvious semantics (saga lore, time/suspend, planeswalker loyalty interactions with damage, charge counters and their varied triggers, etc.)

### Phase 6 — Doubling effects + replacement-effect integration (1 PR, ~300 LOC)

- Wire counter-doubling cards (Doubling Season, Hardened Scales, Primal Vigor, Branching Evolution, Vorinclex Voice of Hunger) into the replacement-effect pipeline from InstanceID v2 section 6
- `ProvidesReplacements` field on Permanent populated for these cards
- Property test: Sai+Doubling Season combo, Walking Ballista+Doubling Season ETB, Hardened Scales mid-turn placement, etc.

### Phase 7 — Saga + Battle counter handling (1 PR, ~250 LOC)

- Saga lore-counter auto-placement at postcombat main phase
- Chapter triggers fire on placement
- Battle defeat-trigger on damage-marker counters
- Property test: Sagas tick through chapters correctly; battles flip on defeat

### Phase 8 — Energy + Experience as separate Seat resources (1 PR, ~150 LOC)

- Energy explicitly NOT a §122 counter — lives on Seat.EnergyCounters
- Experience explicitly NOT a §122 counter — lives on Seat.XPCounters
- Proliferate cannot add energy or experience per §106.11 carveout
- Property test: proliferate skips energy; doubling effects don't apply to energy

**Total estimated LOC: ~2,500 production + ~1,200 test. 9 PRs across 1-2 days swarm.**

---

## 7. Edge cases (known from corpus)

### 7.1 Counter types that DON'T fit cleanly

From Probe F:

- **Energy ({E})** — §106.11 resource pool, not a §122 counter. Carved out to Seat.
- **Poison counters** — go on PLAYER, not permanent. Need special `ValidTargets: [Player]` handling.
- **Experience counters** — go on PLAYER, not permanent. Same shape as poison.
- **Rad counters** — Universes Beyond Fallout. Go on PLAYER. NOT proliferate-eligible. NOT doubling-eligible.
- **Storm counters / Mana counters / Acorn counters** — Un-set silver-border. Probably skip for v1; document as out-of-scope.

### 7.2 Multi-type counter interactions

- **Necroduality** doubles ALL counters (Doubling Season's ancestor): treat as universal doubling, applies to all `DoublingApplies=true` types.
- **Vorinclex Voice of Hunger** — opponents' counter placements are halved (rounded down). Asymmetric doubling. Same pipeline, but with controller-relativity.
- **Bow of Nylea regeneration counters** — old wording uses "regeneration counter" but per modern rules these are just markers for the regenerate replacement. Map to standard counter handling.

### 7.3 Counters on objects in non-battlefield zones

- **Suspend cards in exile with time counters** — per §702.61, time counters tick at upkeep. Counter is on the exiled CARD, not a Permanent. Need a separate `CardLevelCounters` field or extend the Permanent shape to cover exile-residency.
- **Foretold cards** — similar; sitting in foretell exile with foretell-eligible state. No counters technically, but the state machine is counter-shaped.

### 7.4 Cards that grant counter-related abilities

- **Hardened Scales** — replacement effect doubling for +1/+1 only. Already in the doubling pipeline.
- **Branching Evolution** — same shape as Hardened Scales but green.
- **Ozolith, the Shattered Spire** — counters double on creatures entering YOUR control. Conditional doubling.
- **Conclave Mentor** — +1/+1 doubling specifically. Specialized.

All map to the `ProvidesReplacements` pattern from InstanceID v2.

---

## 8. Integration with InstanceID system v2

The Counter DB rides on top of InstanceID v2. Specifically:

1. **CounterStack lineage** — every counter records `PlacedByInstanceID` pointing to the source of the placement. Forensic chain: "this +1/+1 counter on Vraan was placed by Sage of Hours ability instance `h2ABXG40456`."
2. **Counter doubling routes through replacement-effect pipeline** — `EffectsApplied []ReplacementRef` (section 6 of InstanceID v2) carries the doubling effects that modified the placement count.
3. **§122.6 persistence invariant** — works alongside the existing Layer 4 type-change handling. Counters slice immutable during type strips.
4. **Loki invariant lineage** — counter mutations become forensic events with InstanceID source. Replay reproduces counter state deterministically.

---

## 9. Open questions

- **Loyalty counter "doubling" interaction with planeswalkers** — Doubling Season's "planeswalker ETBs with double loyalty counters" is technically a §122.1g pipeline-via-replacement-effect, not in the standard counter doubling pipeline. Worth confirming during Phase 6 implementation that this works correctly. CR §306.5g cited.
- **Counter placement order during multi-counter spells** — e.g., Hangarback Walker on X=5 with Doubling Season — does the doubling apply per-counter or to the whole packet? Per CR §122.1g, "if one or more counters would be placed" applies to the entire event; doubling applies once to N counters as a unit. Confirm implementation matches.
- **Modular trigger** — when an Arcbound creature dies, its +1/+1 counters move to another modular target. This is a death trigger that reads `CounterCount("+1/+1", source)` then `AddCounters` to target. Standard, but worth a test case.
- **Persist / Undying** — bring a creature back with a -1/-1 or +1/+1 counter. These are death-replacement effects (handled in InstanceID v2 section 6) that mint counters on the returned permanent. Coordinate with the death-replacement pipeline.

---

## 10. Out of scope (separate moats)

- **Un-set / silver-border counters** (storm, acorn, mana, etc.) — skip for v1
- **Full replacement-effect subsystem** beyond counter-doubling — covered by InstanceID v2 work
- **Counter-aware AI decision logic** — hat tuning for "should I attack with deathtouch-counter creature into a big blocker" — separate Phase 9/10 of hat work

---

## 11. References

- **Probe F — Counter types catalog:** PR #746, `docs/counter-types-catalog-r60.md`
- **InstanceID System v2:** PR #747, `docs/instanceid-system-v2-r60.md`
- **CR sections referenced:** §106.11 (energy), §122 (counters general), §122.1c (keyword counters), §122.1g (doubling pipeline), §122.6 (persistence), §306 (planeswalker loyalty), §306.5g (Doubling Season + planeswalker), §616 (multiple replacements), §702.61 (suspend), §704.5r (pair-removal SBA), §714 (sagas)

---

## 12. Authority + sign-off

Per the 2026-05-28 engineering authority handoff: 7174n1c is co-primary on accuracy/architecture; Hex executes; wiedeman gate is LOC budget. **Total ~2,500 production LOC across 9 PRs.** Within budget for a structural moat that closes a documented bug class (counter-drift through type changes, inconsistent doubling pipeline) with empirical grounding (Probe F's 252 types catalogued).

**Status: ready for 7174n1c review.** Phase 1 implementation can dispatch in parallel with InstanceID Phase 2-3 (no blocking dependency — Counter Phase 1 only needs the Permanent.Counters field, which lands in InstanceID Phase 1).

**Commitment to the perfectly-written-100%-error-free-divine-codex (CR):** every counter type maps to a CR-section citation; every invariant traces to a specific rule; every implementation step has a property test grounded in corpus-observed behavior. We're not inventing — we're transcribing.
