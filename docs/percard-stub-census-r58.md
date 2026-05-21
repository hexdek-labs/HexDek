# Per-Card Stub Census — R58

Refresh of the per_card stub-pattern survey post-R57. Compares against
`docs/percard-stub-census-r56.md`.

## Headline

The campaign continues converging. R57 shipped two batches —
the stale-partial sweep (Ozai / Kruphix / Zaffai / Aminatou / Tannuk /
Sen Triplets, 6 ports via R54/R55 primitives) and the mana-primitive
followup (Ashling exemption + Magus / Cradle / Sanctum
AddManaPerCount + Cavern AddRestrictedMana, 5 ports). All flag-set
breadcrumbs whose engine primitive had already shipped are now drained
to the primitive registries.

The unrescued `gen_*.go` partial count crossed below 40 for the first
time since the campaign started (R46 baseline ~96, R53 = 52, R56 =
44, R58 = **38**). The remaining 38 are concentrated in four engine-
pipeline boundaries that R54–R57 didn't open.

## Counts (R58)

| Metric | R53 | R56 | R58 | Δ since R56 |
|---|---:|---:|---:|---:|
| Total per_card non-test files                | 919 | 930 | 931 | +1 |
| Files with `emitPartial` in body             | 379 | 363 | 357 | **−6** |
| Total `emitPartial` call sites               | 448 | 431 | 425 | **−6** |
| `gen_*.go` with `emitPartial` in body        | 60  | 51  | 45  | **−6** |
| `custom_*.go` with `emitPartial` in body     | 32  | 31  | 31  | 0 |
| Hand-written with `emitPartial`              | 287 | 281 | 281 | 0 |
| Unrescued `gen_*.go` partials                | 52  | 44  | **38** | **−6** |
| TODO / FIXME / XXX marker files              | 9   | 8   | 9   | +1 |
| Strict pure stubs                            | 1   | 1   | 1   | 0 (Dargo unchanged) |

Strict pure-stub count is still 1 (gen_dargo_the_shipwrecker.go,
engine-deep alt-cost). The R57 stale-partial sweep cleared exactly
the 6 cards flagged in R56 — model and reality agreed.

The R57 mana-primitive batch (5 ports — Ashling / Magus / Cradle /
Sanctum / Cavern) landed in non-gen / hand-written single-card files,
which is why hand-written + custom counts didn't move. The
emitPartial reduction came entirely from the gen_ side.

## Trajectory in three releases

```
R46  ~96 unrescued gen_ partials (~151 "stub-shaped" gen files; 12 pure stubs)
              │  R46 batch: 8 ports
R53  52 unrescued     ~448 emitPartial sites
              │  R54 ships damage-replacement + Layer 7b primitives (~6 + 5 ports)
              │  R55 ships mana-pool exemption + ZoneCastPolicy + dynamic L7b (~25 ports)
R56  44 unrescued     ~431 emitPartial sites
              │  R57 stale-partial sweep (6 ports)
              │  R57 mana-primitive follow-ups (5 ports)
R58  38 unrescued     ~425 emitPartial sites
```

Cycle rate is converging: R54+R55 = ~30 ports in two cycles; R57 = 11
ports in one cycle. The "easy stale-partial" backlog is now empty.

## Remaining 38 unrescued partials — boundary breakdown

The 38 still-unrescued partials cluster around **four** engine
boundaries that R54–R57 did not open. Closing any of them unlocks
3–6 cards at once. Counts below sum to 38 (with two cards counted
twice for double-boundary effects).

### Boundary A — Generalized `would_fire_trigger(trigger_kind)` (5 cards)

Today the engine exposes only `would_fire_etb_trigger` (the
replacement event consumed by Clara/Katara for the ETB-trigger-
doubler half). Non-ETB triggered abilities (attack, dies, draw, etc.)
bypass that slot entirely.

  - **Clara Oswald** — non-ETB Doctor trigger doubling
  - **Katara, the Fearless** — non-ETB Ally trigger doubling
  - **Nadu, Winged Wisdom** — `creature_targeted` trigger fan-out
    (granted ability on every creature you control)
  - **Cloud, Midgar Mercenary** — trigger-doubling-when-equipped
  - **Storm, Force of Nature** — storm-keyword grant consumption
    at cast resolve

Recommended engine surface: extend the existing
`would_fire_etb_trigger` ReplacementEvent kind to a generic
`would_fire_trigger` with a `trigger_kind` discriminator. The
ReplacementEffect registration shape already covers it; the change
is in the dispatcher path (`FireCardTrigger` opens a replacement
window per kind).

### Boundary B — DFC back-face cast / activate surface (5 cards)

DFCs whose back face is a sorcery / class / saga / planeswalker
need a "back face is cast / activated separately" surface. The
engine has `TransformPermanent` (front-back swap of an existing
permanent) but lacks the back-face-cast pipeline.

  - **Norman Osborn / Green Goblin** — back-face graveyard-cast
    cost reduction + combat-damage connive
  - **Extus, Oriq Overlord / Awaken the Blood Avatar** — back face
    token + opponent sac rider as a separate-card surface
  - **Kuja, Genome Sorcerer / Trance Kuja, Fate Defied** — DFC
    transform-back-and-stay-as-sorcery
  - **Lluwen, Exchange Student / Pest Friend** — prepare keyword
    resolves back face directly, skipping stack and mana cost
  - **Tam, Observant Sequencer / Deep Sight** — same prepare-keyword
    shape as Lluwen

### Boundary C — Cast-pipeline alt-costs not covered by ZoneCastPolicy (5 cards)

ZoneCastPolicy (R55) handles source-zone alt-costs cleanly but
doesn't model sacrifice-as-additional-cost, speed riders, or
counter-fueled alt-casts.

  - **Dargo, the Shipwrecker** — sac-as-additional-cost
  - **Jaxis, the Troublemaker** — blitz alt-cost cast pipeline
  - **Lara Croft, Tomb Raider** — play-from-exile-with-discovery-
    counter-this-turn
  - **Ashling, Flame Dancer** — unspent red mana retention to next
    instant/sorcery cast (not the simpler R-keep ManaPoolExemption
    pattern Ashling FoF used in R57; this is a one-shot consume)
  - **Fire Lord Zuko** — firebending mana until end of combat
    (lifetime not modeled)

### Boundary D — Combat-restriction predicates (3 cards)

Combat-declare layer hook (a `would_declare_attacker_to_X` /
`would_declare_blocker_for_Y` predicate slot) is not yet exposed.

  - **Raphael, Ninja Destroyer** — must-be-blocked restriction
  - **Thrun, Breaker of Silence** — protection-from-nongreen
    targeting opponents' spells/abilities
  - **Jasmine Boreal of the Seven** — no-ability vs ability block
    restriction

### Other (20 cards) — single-card snowflakes

The remaining 20 are unique mechanics that don't cluster:

  - Shiko and Narset, Unified — spell-copy-with-new-targets
  - Raph & Mikey, Troublemakers — bottom-in-random-order RNG
  - Archelos, Lagoon Mystic — ETB-tap symmetric replacement
  - Ty Lee, Chi Blocker — "while you control" lock release
  - Alpharael, Stonechosen — Void condition arming via warp tracking
  - The Reaper King, No More — heuristic target choice (cosmetic
    partial; could just drop the breadcrumb)
  - Silvar, Devourer of the Free — partner-tutor target shape
  - Alaundo the Seer — per-card time-counter (suspend) tracking
  - The Reality Chip — play-from-top-of-library while attached
  - Jon Irenicus, Shattered One — control-change donate
  - The Wandering Minstrel — lands-enter-untapped static (AST owns)
  - Zidane, Tantalus Thief — opp-gain-control treasure event
  - The Master, Transcendent — per-card mill-this-turn tracking
  - The Second Doctor — no_max_hand_size global static + cleanup-
    step enforcement
  - Zethi, Arcane Blademaster — multikicker count + exiled card pile
  - The Twelfth Doctor — demonstrate-grant on first non-hand cast
  - The Master of Keys — enchantment-escape grant static
  - Toph, the First Metalbender — return-on-die-or-exile rider on
    earthbent lands
  - Toph, Hardheaded Teacher — earthbend-on-cast trigger helper
  - Sokrates, Athenian Teacher — combat-damage→draws conversion

## Top 10 R58+ targets

Ranked by leverage:

### Engine-surface batch (one primitive, multiple consumers)

#### 1. Generalized `would_fire_trigger` (Boundary A, 5 cards)

R56 top-10 #8. R57 didn't touch this. Single engine surface unlocks
Clara, Katara, Nadu, Cloud, Storm Force of Nature.

#### 2. DFC back-face cast surface (Boundary B, 5 cards)

Larger engine surface but unblocks 5 single-card snowflakes plus
the prepare-keyword pattern shared by Lluwen + Tam (and Tam's
back-face is already partially-implemented).

#### 3. Combat-declare predicate slot (Boundary D, 3 cards)

Smaller scope than Boundary A or B but the engine already has
hook points in `combat.go`; could be the cheapest engine surface
to extend.

### Single-card snowflakes with clean fixes

#### 4. The Reaper King, No More — cosmetic partial cleanup

The partial documents heuristic targeting; the behavior is correct.
Drop the breadcrumb. 1-line fix.

#### 5. Per-card mill-this-turn stamp (The Master, Transcendent)

R56 top-10 #10. 1-line stamp in `mill()` (state.go) unlocks Master
Transcendent and potentially a couple sibling cards.

#### 6. Archelos, Lagoon Mystic — accept current approximation

The retroactive-tap-on-ETB approach in the current handler is the
correct strategic approximation; the printed-text "true ETB-tap
replacement" is the only literal-text gap. The handler could drop
the partial and document the approximation as the intended
implementation.

### Stale-partial-style swap ports using existing primitives

#### 7. The Reality Chip — `RegisterZoneCastPolicy(zone="library", top-only)`

Extending ZoneCastPolicy to support library-top-card-only casts
(or just registering with a Predicate that the cast pipeline gates
via a "first card only" check) lets Reality Chip route through the
R55 primitive. Borderline stale-partial — the primitive almost
covers it.

#### 8. The Wandering Minstrel — Layer 6 keyword refresh on lands

"Lands you control enter untapped" is a static the current handler
delegates to AST. If the AST keyword pipeline isn't applying this
unconditionally, Layer 6 grant-keyword refresh on `permanent_etb`
gated to lands closes the gap. Approach mirrors R55 Toph 1stMB
land-type grant.

### Genuinely new R59+ engine surfaces

#### 9. Alt-cost: sacrifice-as-additional-cost (Boundary C, Dargo + 1)

Dargo and a few non-gen cards use sacrifice as an additional cost.
The R55 ZoneCastPolicy doesn't model this — would require either
a new ZoneCastPolicy field (`SacrificeRequirement`) or a separate
"alt-cost-additional" primitive.

#### 10. Cast-pipeline "mana retention until end of [phase]" (Boundary C subset)

Ashling Flame Dancer and Fire Lord Zuko share a "keep unspent mana
for next instant/sorcery cast / next combat" rider. Could be a
focused extension of the R55 mana-pool exemption with a phase or
event-scoped duration (currently exemptions are permanent until
unregister).

## Trajectory recommendation

R57's pattern (one engine cycle's primitives drained into one batch
cycle's consumer ports) worked. The next two cycles should:

  - **R58**: open Boundary A (`would_fire_trigger`). 5 card unlocks
    for one engine surface. Highest leverage in the remaining set.
  - **R59**: consumer batch for Boundary A + clean up the cosmetic /
    swap-only items in #4 / #6 / #7 / #8 of this top-10. Should
    bring unrescued count from 38 → ~28.

If R58/R59 follow this rhythm, the unrescued count crosses below 30
by mid-2026-05, and the campaign moves into snowflake-only territory.

Methodology: emitPartial body-grep (non-comment), strict pure-stub
heuristic (≤50 lines + mutator absence + no custom override).
Counts captured 2026-05-20 from main at `ef25581`.
