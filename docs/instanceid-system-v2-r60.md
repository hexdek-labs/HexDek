# InstanceID System — Engine-Wide Object Identity & Lineage (v2)

**Status:** Design v2 — ready for review and Phase 1 implementation
**Authors:** 7174n1c (architectural lead), Hex (engineering)
**Date:** 2026-05-29
**Supersedes:** `docs/instanceid-system-r60.md` (v1, 2026-05-28)
**Validated by:** 6 corpus probes (PRs #741-746)
**Closes bug class:** `*Card` pointer aliasing (CardIdentity invariant family), ZoneConservation copy-tracking ambiguity, ZoneCastGrantExpiry source-lifetime conflation, ExileLinkageIntegrity LTB-vs-cast-grant false positives.

---

## 1. Problem statement

The 25k-game layer-stress sweep (2026-05-28) surfaced **3324 CardIdentity invariant hits, 1516 ZoneConservation hits, 736 ExileLinkageIntegrity hits**. Probe C (PR #741) ruled out Loki harness pointer-aliasing — these are **genuine engine bugs** rooted in `*Card` pointer-identity being a leaky abstraction for MTG card identity.

This design replaces pointer-based identity with explicit **InstanceID strings** carrying provenance, color, CMC, visibility state, and lineage links. Per-printing data stays on the Card struct unchanged; per-instance identity becomes its own first-class system.

**Core principle:** **Identity vs Characteristics is the clean cut.** Identity persists across zone changes per CR §400.7. Characteristics can change at runtime (Layer 1-7, copy effects, type-strips). Separating them removes the bug class.

---

## 2. The four mint paths (validated by Probe E)

| Path | What lives on battlefield | InstanceID shape | Probe-validated cards |
|---|---|---|---|
| 1: Copy a spell on the stack | Token (per §706.10b) | One TK ID, SourceInstanceID = original spell | Riku, Mirari, Twincast, storm |
| 2: Copy a permanent directly | Token | One TK ID, SourceInstanceID = copied permanent | Saheeli Decimator, Mirror Image, Clone, Phantasmal Image, Helm of the Host |
| 3: ETB-as-copy | Original card wearing snapshot | One OG ID + CopiableSnapshot on Permanent | Body Double, Phyrexian Metamorph, Spark Double, Sakashima |
| 4: Mutate / Meld | Multiple cards as one Permanent | N stacked OG IDs + MergedCards/MergeKind on Permanent | ~40 Ikoria mutate creatures, 16 meld groups (Brisela, Hanweir, Urza, etc.) |

**Key Probe E finding — 4 cross-type meld surprises:** 2 meld groups consume LAND input, 1 consumes ARTIFACT, 1 (Urza set) ENDS AS PLANESWALKER. The unified `MergedCards/MergeKind/TopCard` shape handles all four. Zero per-card overrides needed for the mechanics; 7 meld-trigger handlers + 5 scaling-mutate-trigger handlers are standard trigger-payload work.

---

## 3. InstanceID format

```
<prefix><seat><provenance><visibility><color><cmc><seq5>
```

| Field | Width | Values | Meaning |
|---|---|---|---|
| prefix | 1 char | `h` (HexDek), reserved: `t` tournament, `s` simulation | Namespace |
| seat | 1 char | `0`–`3` | Owner-seat at mint time (immutable; survives control changes) |
| provenance | 2 char | `OG` `TK` `CP` `AB` | What it is (not how it got there) |
| visibility | 1 char | `V` visible / `H` hidden | Face-down info-leak protection |
| color | 1–5 char | `W` `U` `B` `R` `G` (canonical WUBRG) or `C` colorless | Printed color identity |
| cmc | 1–2 char | `0`–`16` | Printed CMC (X-spells encode non-X portion; cost reductions don't affect) |
| seq5 | 5 char | `00000`–`99999` | Per-(game, seat) monotonic counter; never reused |

**Examples:**

| Instance | InstanceID |
|---|---|
| Original Lightning Bolt in seat 0's hand | `h0OGVR100042` |
| Riku copy of Cultivate in seat 2 | `h2CPVG303958` |
| Sai thopter token in seat 1 | `h1TKVC101234` |
| Atraxa token (5-color) | `h0TKVWUBRG404071` |
| Manifested face-down 2/2 in seat 2 | `h2OGHUR603501` (back face would normally be Niv-Mizzet; H = hidden) |
| Etali's triggered ability instance | `h3ABVRG403901` |

**Format invariants pinned by property tests:**

- **Uniqueness per (game, seat).** No two `Card` objects in one game-seat share an InstanceID. Property test asserts this.
- **Provenance → required lineage fields.**
  - `OG`: SourceInstanceID and EnablerInstanceID empty
  - `TK`/`CP`/`AB`: EnablerInstanceID non-empty
  - `CP`: SourceInstanceID non-empty (must reference the spell being copied)
- **InstanceID persists across zone changes.** Hand → stack → graveyard → exile → battlefield all preserve. The ID changes only on cease (§707.10 copies dissolving) or phase-in-restore (which doesn't actually re-mint — see §11).
- **No cross-seat sharing.** An InstanceID starting with `h2` never appears in seat 1's zone lists. Control changes don't re-stamp — only `Permanent.Controller` flips.
- **Front-face encoding for DFCs** per CR §712.6c. MDFC characteristics in non-battlefield zones default to front face; InstanceID matches.

---

## 4. Struct extensions

### 4.1 Card

```go
type Card struct {
    // Per-printing identity (static, set at deck-load — UNCHANGED)
    Name             string
    ScryfallID       string
    Types            []string
    OracleText       string
    ManaCost         string
    PrintedCMC       int
    PrintedColors    []ManaColor
    Power, Toughness string
    // ... existing printed data unchanged

    // Per-instance identity (runtime, set at mint)
    InstanceID         string
    Provenance         Provenance       // OG | TK | CP | AB
    Visibility         Visibility       // Visible | Hidden
    SourceInstanceID   string           // empty for OG; required for CP; optional for TK
    EnablerInstanceID  string           // empty only for OG
    EnablerHistory     []string         // append-only log for re-copy chains (Vesuvan, Lazav, Volrath)
    ActiveFace         FaceIndex        // Front | Back (DFC face flag)

    // Back-face data (DFC support)
    BackFaceCharacteristics *Characteristics  // non-nil only for DFC/MDFC cards
}
```

### 4.2 Permanent

```go
type Permanent struct {
    // ... existing fields (ID, Controller, Card pointer, tapped, summoning-sick, etc.)

    // Exile linkage (per §400.7e — source-Permanent owns the linkage, not a global table)
    ExiledByMe []string  // InstanceIDs of cards this permanent is currently exiling
    ExileLinkageKind LinkageKind  // LTBReturn | CastGrant | PermanentExile (per linkage record, not per permanent)

    // Copy state (CopyMechanism list per Probe A — Mirage Mirror has 2)
    CopyMechanisms          []CopyMechanism
    CopiedTargetInstanceID  string
    CopiableSnapshot        *CopiableCharacteristics  // frozen at copy moment per §706.2
    CopyHistory             []CopyEvent  // append-only

    // Mutate + Meld unified
    MergedCards []string    // InstanceIDs of all cards merged into this permanent
    MergeKind   MergeKind   // Mutate | Meld | None
    TopCard     *Card       // top of mutate stack (Mutate only; Meld has single combined result)

    // Per-card overrides surfaced by Probe A
    BypassesLegendRule bool          // Sakashima-shape exemption
    AttachedTokenIDs   []string      // Helm of the Host repeated mints, attached-creature tracker

    // Counters (per Probe F — 252 types, §122 invariants enforced)
    Counters []CounterStack

    // Replacement-effect provider (cards that PROVIDE replacements like Doubling Season)
    ProvidesReplacements []ReplacementSpec
}

type CopyMechanism struct {
    TriggerSource   CopyTrigger      // ETB | Upkeep | Activated | EventCondition | EOT | CombatBegin | Other
    Duration        CopyDuration     // Permanent | UntilEOT | UntilNextCopy | UntilNextUpkeep | UntilNextEvent
    Restriction     *CopyRestriction // target-type filter or other constraint
    PerCardOverride string           // names a handler in per_card registry for unusual mechanics
}

type CopyEvent struct {
    AtGameTick   int
    SourceID     string
    EnablerID    string
    Mechanism    int  // index into CopyMechanisms
}

type LinkageKind int
const (
    LTBReturn       LinkageKind = iota  // Banisher Priest / Oblivion Ring shape — return on source leaves
    CastGrant                           // Etali / Mind's Desire — cast permission window, no return
    PermanentExile                      // Settle the Wreckage / disturb-cast — exiled forever
)
```

### 4.3 AbilityInstance (new)

```go
type AbilityInstance struct {
    InstanceID        string   // AB-provenance ID; lives on the stack
    SourceInstanceID  string   // permanent that owns the ability
    EnablerInstanceID string   // triggering event (combat begin, ETB, etc.) — empty for activated
    AbilityID         string   // which ability on the source (multi-ability cards)
    Controller        int      // seat that controls per CR §603.7d
    PushedAt          int      // game-clock at stack push

    // Trigger-time captured metadata (deviation pattern from storm/X-spell/etc)
    TriggerMetadata map[string]any  // e.g., {"storm_count": 12} or {"x_value": 5} or {"ascension_active": true}

    // Delayed-trigger support (§603.7c, see Section 11)
    DelayedUntil      *DelayedCondition  // nil for immediate stack push; non-nil for delayed
    DelayedCreatedAt  int
    DelayedExpiresAt  int  // game-tick after which this delayed trigger is dropped (e.g., end of turn for EOT triggers)
}
```

Each StackItem gains an optional `Ability *AbilityInstance` (nil for spells).

### 4.4 Seat (lazy-init subsystem registry per Probe D)

```go
type Seat struct {
    // ... existing fields

    // Lazy-init subsystem flags — false by default, flipped when a card cares
    DayNightActive   bool   // wakes when daybound/nightbound or "if it is day/night" card touched
    MonarchActive    bool
    InitiativeHolder bool
    AscendActive     bool   // wakes when ascend or city's-blessing card touched
    CurrentDungeon   *Dungeon
    RingTempts       int    // 0 = ring hasn't tempted yet
    EnergyCounters   int    // §106.11 resource pool (NOT a §122 counter; can't proliferate)
    XPCounters       int    // Experience counters
    ForetellExile    []*Card // separate exile bucket for foretold cards
    HasCityBlessing  bool   // computed: 10+ permanents → Ascend grants
}

// Game-level registry for dormant hooks
type Game struct {
    // ...
    SubsystemHooks []DormantHook  // wakes on first matching card-in-play event
}
```

**Probe D activator counts (per subsystem):**

| Subsystem | Activator candidates |
|---|---|
| Day/Night | 86 |
| Monarch | 50 |
| Initiative | 23 |
| Ascend / City's Blessing | 31 (delegated together) |
| Dungeons | 46 |
| Ring tempts | 54 |
| Energy | 118 |
| Experience | 15 |
| Foretell | 61 |

**Net 484 candidate cards.** Dormant cost is zero; activation flips one bool on Seat.

---

## 5. Copy Mechanism Registry (refactored per Probe A)

**Original v1 design:** single `CopyClass` enum (4 values).

**Probe A finding:** Mirage Mirror (and similar) has TWO independent copy mechanisms — upkeep-permanent + activated-temporary. Single enum doesn't express this. Refactored to **list of mechanisms** per Permanent.

```go
type CopyMechanism struct {
    TriggerSource   CopyTrigger
    Duration        CopyDuration
    Restriction     *CopyRestriction
    PerCardOverride string
}
```

**Per-card-override registry (5 model-breakers from Probe A):**

| Card | Override |
|---|---|
| Enolc | `partner_conditional_copy` — copies only if specific partner condition met |
| Ertai's Meddling | `exile_zone_snapshot` — motivates a `SourceZone` field on CopyMechanism |
| Soulflayer | NOT a copy effect — keyword-grant. Goes through `GrantedAbilities` instead. |
| Mirage Mirror (multi-arm) | Standard `CopyMechanisms []CopyMechanism` — 2 entries (Upkeep+Activated) |
| TokenCopy+rider hybrids | `CopyTarget enum: Self / OtherPermanent / NewToken` — distinguishes copy-the-target-permanent vs mint-a-new-token-as-copy-of-target |

**Sakashima legend bypass:** `BypassesLegendRule bool` on Permanent (per-card override, not a CopyMechanism field).

**Spark Double counter rider:** PerCardOverride = `"spark_double_p1p1_rider"` — handler grants +1/+1 counter at the moment of copy, snapshot includes the counter.

**Phantasmal Image fragile-on-target rider:** PerCardOverride = `"phantasmal_image_fragile_rider"` — handler adds the "becomes target → sacrifice" triggered ability to the copy.

---

## 6. Replacement-Effect Layer (per Probe B)

**Probe B finding:** **3,883 cards match replacement-effect patterns** across 6 §614 families. InstanceID-relevant subset narrowed to **343 cards** (mint-doubling 40, counter-doubling 37, zone-redirect 266).

**Design — `EffectsApplied` slice on relevant event types:**

```go
type TokenMintEvent struct {
    SourceInstanceID  string  // who minted the token (Sai's ability instance)
    TargetSeat        int
    BaseCharacteristics CopiableCharacteristics
    
    // Replacement-effect stack
    EffectsApplied []ReplacementRef  // ordered per §616 application choice
    FinalCount     int               // resolved count after all replacements
}

type ReplacementRef struct {
    Source         string         // InstanceID of the Permanent providing the replacement
    Modification   ReplacementOp  // Double | Halve | Redirect | ZoneSubstitute | Skip
    AppliedAtTick  int
}
```

**Sai+Mondrak+Anointed+DS walkthrough validated in Probe B:**

1. Sai trigger creates 1 Thopter token (base) → `TokenMintEvent{FinalCount: 1, EffectsApplied: []}`
2. Mondrak Glory Dominus applies first → `FinalCount: 2, EffectsApplied: [Mondrak]`
3. Anointed Procession applies → `FinalCount: 4, EffectsApplied: [Mondrak, Anointed]`
4. Doubling Season applies → `FinalCount: 8, EffectsApplied: [Mondrak, Anointed, DoublingSeason]`
5. Engine mints 8 distinct TK InstanceIDs (`h2TKVC10XXXX` each), all sharing `EnablerInstanceID = Sai-ability-instance`

**Per §616, the affected event's CONTROLLER chooses application order when multiple replacements apply.** The engine prompts (or AI decides) at the replacement-resolution step.

**Six model-breakers from Probe B** documented in `docs/replacement-effect-audit-r60.md` (PR #742) with proposed handlers.

---

## 7. Exile linkage via Permanent.ExiledByMe (per 7174n1c simplification)

**Original v1 design:** global linkage table tracking exile relationships, with a `linkage_type` enum to distinguish LTB-return from cast-grant from permanent-exile.

**Refactored:** each source Permanent owns its own `ExiledByMe []string`. LTB triggers walk this slice on leave-battlefield. No global table.

**Per CR §400.7e:** the "broken link" case dissolves because the LTB trigger fires on ANY zone-change-out-of-battlefield (death, bounce, exile, commander-zone). Banisher Priest bounced by Cyclonic Rift still fires its return trigger.

**LinkageKind distinguishes three patterns:**

- `LTBReturn`: source Permanent owns the exile; LTB returns. Banisher Priest, Oblivion Ring, Karmic Guide.
- `CastGrant`: cast-permission window bound to the AbilityInstance's lifetime, not the source Permanent's. Etali, Mind's Desire, Bolas's Citadel.
- `PermanentExile`: no return mechanism. Settle the Wreckage, disturb-cast originals, foretold cards that never get cast.

**Invariant rewrite:** ExileLinkageIntegrity check becomes two-pronged:
1. **Source-held linkages** (LTBReturn): for every card in exile tagged LTBReturn, verify the source Permanent exists with this card's InstanceID in its `ExiledByMe`. If source absent OR list-mismatch → bug.
2. **Self-managed linkages** (CastGrant, PermanentExile): no source-Permanent back-reference required. State machine (cast-window-open / cast-window-closed / permanently-exiled) is sufficient.

This collapses ~736 hits from the 25k sweep into "LTBReturn handler bugs" + "invariant-logic false positives" — two cleanly separable repair tasks.

---

## 8. MergedCards unified (Mutate + Meld, per Probe E)

Per CR §702.139 (Mutate) and CR §712 (Meld):

```go
type Permanent struct {
    // ...
    MergedCards []string    // InstanceIDs of all cards in the merge
    MergeKind   MergeKind   // Mutate | Meld | None
    TopCard     *Card       // top of mutate stack (Mutate); ignored for Meld
}
```

**Probe E validation:**

- **Mutate** (~40 Ikoria cards): MergeKind = Mutate. MergedCards = stack of InstanceIDs top-to-bottom. TopCard determines name/color/CMC per §702.139a. On leave-play, unmerge — each MergedCard moves to graveyard individually per §702.139d.
- **Meld** (~16 cards across multiple sets): MergeKind = Meld. MergedCards = exactly 2 InstanceIDs. TopCard not used (Meld has a single combined result card with its own characteristics). On leave-play, both cards separate per §712.3.

**Probe E surprise findings — cross-type melds:**
- 2 meld groups consume LAND input (land becomes part of the meld stack)
- 1 consumes ARTIFACT
- 1 (Urza set) RESULTS IN a PLANESWALKER (Brisela-shape but planeswalker output)

**All four fit the unified shape.** Zero per-card overrides for the mechanics themselves; standard trigger-payload work for the meld-trigger handlers (7 of them).

---

## 9. Subsystem activation registry (per Probe D)

**484 activator candidates across 10 subsystems.** Each subsystem is a dormant hook on `Game.SubsystemHooks` that wakes when a triggering card enters any zone (or for Foretell, is in foretell exile).

**Hook structure:**

```go
type DormantHook struct {
    Subsystem        Subsystem  // DayNight | Monarch | Initiative | Ascend | Dungeon | RingTempts | Energy | XP | Foretell
    Active           bool       // false until first activation
    ActivationEvent  EventSpec  // pattern that wakes it (e.g., "card with 'daybound' enters any zone")
    OnActivate       func(*Game) // sets up the subsystem's state machine
}
```

**At engine start:** all hooks dormant. Cost: nothing.

**On any card-zone-change event:** engine walks hooks, fires `OnActivate` for any matching event whose subsystem isn't already active. Once active, the subsystem stays active for the rest of the game (sticky per CR — Monarch once gained is always tracked until passed, etc.).

**For Loki invariants:** dormant subsystems are not checked. Smaller invariant surface per game when subsystems don't activate.

**Counter DB ties in here:** Energy and Experience counter pools live on Seat (not §122 counters per Probe F finding); they're tracked by the Energy/Experience subsystems specifically.

---

## 10. Counter DB (per Probe F)

**Probe F finding: 252 distinct counter types** (well above the 80-150 we estimated). Long tail is real — many are single-card Un-set/oddball but follow the same §122 plumbing.

**Counter type registry:**

```go
type CounterTypeDef struct {
    Name             string
    ValidTargets     []TargetType
    Placement        PlacementRule
    DoublingApplies  bool             // §122.1g — true for +1/+1, false for energy/poison/experience/rad
    Proliferate      bool             // §122 — most yes, some no
    StackingBehavior StackingRule     // §704.5r — +1/+1 ↔ -1/-1 cancel; others usually don't
    GrantedAbility   *AbilityRef      // §122.1c — keyword counters grant abilities (lifelink counter grants lifelink)
}
```

**Engine rules locked from Probe F:**

- **§122.6 — counters persist through type changes.** Lifelink counter on Vraan stays even after Humility strips creature type. Counter slice on Permanent immutable across Layer 4 type-changes; only counter-add/remove events modify the slice. **This is a Loki invariant.**
- **§122.1g — doubling pipeline scoped to player-controlled permanents only.** Doubling Season doesn't double energy/poison/experience/rad counters.
- **§704.5r — pair-removal strictly +1/+1 ↔ -1/-1.** Other counter pairs don't cancel as SBA.
- **§122.1c — ability-grant predicate sites enumerated.** Lifelink counter grants lifelink; flying counter grants flying; etc.

**Energy carveout:** NOT a §122 counter. It's a §106.11 resource pool. Proliferate can't add {E}. Energy goes on Seat, not Permanent.

**Four open follow-ups identified in Probe F report** (PR #746) for the full Counter DB build.

---

## 11. Delayed triggers (the messy one — flagged by 7174n1c)

**Per CR §603.7c:** delayed triggered abilities are abilities that wait to fire until a future event. They are NOT on the stack continuously — they exist in a "pending pool" waiting for their trigger condition.

**Examples:**
- "At the beginning of the next end step, sacrifice ~" (Pact of Negation's owe-trigger, Strionic Resonator interactions)
- "When that creature dies this turn..." (Glissa Sunslayer scoped tracking)
- "Until your next upkeep, ~" (recurring delayed pattern)
- Suspend "remove a time counter at upkeep; when 0, cast" (CR §702.61)
- Madness pending costs

**Why messy:** delayed triggers span turns, condition on FUTURE events that may not happen, might never fire (Pact paid during upkeep), interact with copy effects (delayed trigger copies), and survive their source leaving play in some cases.

**Design — delayed trigger pool on Game struct:**

```go
type Game struct {
    // ...
    DelayedTriggers []AbilityInstance  // pending pool; each has DelayedUntil set
}

type DelayedCondition struct {
    EventType    EventType    // BeginUpkeep | BeginEndStep | CreatureDies | SpellCast | etc
    EventFilter  EventFilter  // optional further qualification (e.g., "creature dies that was Glissa's target")
    OneShot      bool         // most are; some are recurring (until next upkeep, etc.)
    Recurring    *Recurrence  // for recurring delayed triggers
}
```

**Lifecycle:**

1. **Creation:** when an ability that sets up a delayed trigger resolves, the engine mints a fresh AbilityInstance with `DelayedUntil` set, pushes it onto `DelayedTriggers` pool (NOT the stack).
2. **Source independence (§112.7a):** the AbilityInstance lives independently of its source Permanent. Source can die; delayed trigger still fires.
3. **Evaluation:** on every event, engine walks `DelayedTriggers`, checks each one's condition. Matching triggers move from pool → stack, then resolve normally.
4. **Expiry:** if `DelayedExpiresAt` is set and reached without firing (e.g., end-of-turn-tagged trigger that didn't condition-match), trigger is dropped.
5. **One-shot vs recurring:** OneShot triggers are removed from the pool after firing. Recurring (e.g., "at the beginning of each upkeep until ~") stay in the pool until expiry.

**Invariants for delayed triggers:**

- Every entry in `DelayedTriggers` must have a non-nil `DelayedUntil`.
- AbilityInstance.InstanceID must be unique across DelayedTriggers and active StackItems (no double-residence).
- Expired delayed triggers cleaned up at appropriate phase boundary (CR §514 cleanup step for EOT-scoped).
- Pact-shape "owed payment" triggers are delayed triggers tagged with `OneShot: true, EventType: BeginUpkeep, EventFilter: SeatMatchesController`.

**Copy interaction:** a delayed trigger can be copied (Strionic Resonator copies a triggered ability). The copy is a fresh AbilityInstance with its own InstanceID, also in the DelayedTriggers pool, with the same DelayedUntil. Both fire when condition matches.

**Suspend implementation as a delayed trigger:** card sits in exile with time counters. "At beginning of upkeep, remove a time counter" is a RECURRING delayed trigger. "When last counter removed, cast for free" is a ONE-SHOT delayed trigger with condition "this card has 0 time counters."

**This makes suspend a clean delayed-trigger composition rather than a special mechanism.**

---

## 12. Empirical validation from probes

| Probe | PR | Corpus walked | Cards classified | Key finding |
|---|---|---|---|---|
| A — Copy mechanisms | #745 | ~30k cards | ~150 (estimated by patterns) | 5 model-breakers, multi-arm CopyMechanism list-shape needed |
| B — Replacement effects | #742 | 37,384 cards | 3,883 matched, 343 InstanceID-relevant | Sai+Mondrak+Anointed+DS chain produces 8 distinct InstanceIDs sharing origin |
| C — Loki seed-deck | #741 | Code path | n/a (forensic) | **Harness CLEAN. 3324 CardIdentity hits ARE real engine bugs.** |
| D — Subsystem activation | #743 | ~30k cards | 484 activators | 10 dormant hooks; CityBlessing delegated to Ascend |
| E — Mutate + Meld | #744 | ~30k cards | ~60 (mutate 40 + meld 16) | 4 cross-type meld surprises; unified shape holds |
| F — Counter types | #746 | ~30k cards | 252 distinct types | §122.6/§122.1g/§704.5r/§122.1c invariants locked; energy carveout |

**Total corpus coverage: ~30,000 cards walked, ~4,500 classified across 6 dimensions.** The design isn't speculative; every architectural decision has empirical grounding.

---

## 13. Implementation phases

Phase counts updated based on probe findings.

### Phase 1 — Foundation (1 PR, ~400 LOC)

- Add lineage fields to `Card` struct
- Add `mintInstanceID(seat, prov, color, cmc) string` helper with per-game-seat seq5 counter
- Update deck-load to mint OG IDs
- Update Loki seed-deck-injector if needed (Probe C: clean, no fix needed)
- Property test: format-validity regex, OG-mint uniqueness per (game, seat)
- Backwards-compat: empty InstanceID treated as legacy mode

### Phase 2 — Token + copy mints + AbilityInstance (1 PR, ~500 LOC)

- Update all token-creation sites to mint TK with lineage
- Update all spell-copy + permanent-copy sites to mint CP
- Add `AbilityInstance` struct + StackItem.Ability field
- Update trigger-push + activation-push to mint AB with TriggerMetadata
- Property test: TK/CP/AB IDs unique, lineage populated correctly

### Phase 3 — Linkage refactor + ExiledByMe (1 PR, ~250 LOC)

- Add `ExiledByMe []string` + `LinkageKind` to Permanent
- Migrate Banisher Priest / Oblivion Ring / Karmic Guide handlers to use source-held linkage
- Rewrite ZoneCastGrantExpiry to bind to AbilityInstance lifetime
- Property test: each LinkageKind's expected end-state pinned

### Phase 4 — Invariant migration (1 PR, ~300 LOC)

- Rewrite CardIdentity invariant to use InstanceID equality
- Rewrite ZoneConservation to use minted-ID-set tracking
- Rewrite ExileLinkageIntegrity per two-pronged check (LTBReturn vs self-managed)
- Run 25k-game layer-stress sweep — confirm hit counts collapse

### Phase 5 — Copy + replacement-effect handlers (2 PRs, ~600 LOC)

- Implement `CopyMechanisms []CopyMechanism` on Permanent
- Wire 5 per-card-override handlers from Probe A
- Implement `EffectsApplied []ReplacementRef` on token-mint events
- Wire Sai+Mondrak+Anointed+DS chaining
- Property test: §616 stacking simulation produces correct counts

### Phase 6 — Subsystem activation registry (1 PR, ~250 LOC)

- Implement dormant-hook registry per Probe D
- Wire 484 activator cards to their respective subsystems
- Tests for each subsystem activation path

### Phase 7 — Counter DB foundation (1 PR, ~400 LOC)

- Implement `CounterTypeDef` registry with 252 types from Probe F
- §122.6 persistence invariant
- §122.1g doubling pipeline
- §704.5r pair-removal SBA
- Energy carveout (separate from §122)

### Phase 8 — Mutate + Meld + Delayed triggers (1 PR, ~350 LOC)

- Implement `MergedCards/MergeKind/TopCard` on Permanent
- Wire 7 meld-trigger + 5 scaling-mutate-trigger handlers
- Delayed trigger pool + condition evaluation
- Suspend reimplemented as delayed-trigger composition

### Phase 9 — Spectator + Heimdall (1 PR, ~250 LOC)

- Lineage fields on spectator API payloads
- Heimdall lineage tree rendering
- Loki log format updated

**Total estimated LOC: ~3,300 production + ~1,500 test. 10 PRs across 2-3 days swarm.** Manageable, bounded, each phase independently verifiable.

---

## 14. Open questions

- **Subsystem activation event spec** — concrete event signature for "card enters any zone with daybound supertype" vs "permanent enters battlefield with daybound supertype" — needs precise wording for the hook predicate. Will resolve at Phase 6 implementation time.
- **TriggerMetadata schema discipline** — `map[string]any` is flexible but un-typed. Worth defining a `TriggerMetadataSpec` per common pattern (storm_count, x_value, etc.) for type-safety and documentation. Resolve at Phase 2.
- **§616 stacking-order prompt UX** — when multiple replacements apply, controller chooses order. CLI vs UI prompt vs AI deterministic order — Phase 5 decision.
- **Replay determinism with random4** — confirmed: seq5 is a deterministic counter not random. Replays reproducible from same RNG seed.

## 15. Out of scope (parallel moats)

- **Full replacement-effect subsystem** beyond InstanceID-relevant subset (damage redirection, life-gain replacement, etc.) — separate moat
- **Continuous-effect layer system completeness** — covered by #732 + future work
- **AI-side InstanceID-aware decision logging** — optional Phase 9b
- **xMage parity** — Phase 1 still pending Josh greenlight

---

## 16. References

- **Probe A — Copy mechanism audit:** PR #745, `docs/copy-mechanism-audit-r60.md`
- **Probe B — Replacement effect audit:** PR #742, `docs/replacement-effect-audit-r60.md`
- **Probe C — Loki seed-deck forensic:** PR #741, `docs/loki-seed-deck-forensic-r60.md`
- **Probe D — Subsystem activation audit:** PR #743, `docs/subsystem-activation-audit-r60.md`
- **Probe E — Mutate + Meld audit:** PR #744, `docs/mutate-meld-audit-r60.md`
- **Probe F — Counter types catalog:** PR #746, `docs/counter-types-catalog-r60.md`

**CR sections referenced:** §106.11 (energy as resource pool), §108.3 (ownership), §111.8 (tokens cease on leave), §112.7a (ability independence), §122 (counters), §202.3b (CMC), §400.7 (zone changes, new object), §400.7e (broken-link exile), §603.7c (delayed triggers), §603.10 (ability resolution), §606.1 (mana abilities), §613 (layer system), §614 (replacement effects), §616 (multiple replacements), §702.25 (phasing), §702.61 (suspend), §702.103 (bestow), §702.139 (mutate), §702.143 (foretell), §702.146 (disturb), §704.5j (legend rule), §704.5r (counter pair removal), §706 (copy effects), §707.10 (copies cease), §712 (meld), §712.6c (MDFC front-face default), §720.4 (Karn restart), §800.4a (player loss in multiplayer), §903 (commanders)

---

## 17. Authority

Per the 2026-05-28 engineering authority handoff: 7174n1c is co-primary on accuracy/architecture. Hex executes. wiedeman gate is LOC budget — total ~3,300 production LOC across 9 phases. **Within budget for the foundational moat that closes the pointer-identity bug class.**

**Status: v2 ready for review.** Three submarine passes + 6 corpus probes have grounded the design. Phase 1 implementation can begin when greenlit.
