# Per-Card Stub Census — R53

Refresh of the stub-pattern survey across `internal/gameengine/per_card/`.
Compares against the R46 baseline (which seeded the campaign at ~151
auto-generated `gen_*.go` stub-shaped handler files) and the R46 scope
report's targeted "12 pure stubs" subset
(`docs/stub-hunt-percard-r46.md`).

## Headline

The pure-stub population — files where the registered handler is
register-and-bail with no engine mutation — has been **almost completely
cleared**. Of 154 `gen_*.go` files in the directory today, **exactly one
strict pure stub remains** by the heuristic below, and that one (Dargo,
the Shipwrecker) is a known engine-deep alt-cost case.

The remaining "stub-pattern" surface in the directory is now dominated
by **partial-residual handlers** — files where 80%+ of the printed text
is implemented and one or two clauses point at engine-deep territory
(damage replacement, mana-pool empty, cast-pipeline alt-costs, Layer 7
set-PT, etc.) that the per-card surface can't reach on its own.

## Methodology

Three categories scanned across `internal/gameengine/per_card/*.go`
(excluding `*_test.go`):

1. **`emitPartial` calls** in function bodies — placeholder/partial
   markers grep'd as `^\s+emitPartial\(`. Comments mentioning
   `emitPartial` are excluded.
2. **`TODO` / `FIXME` / `XXX`** comment markers (case-sensitive).
3. **Pure stubs** — `gen_*.go` files ≤ 50 lines that have at least
   one `emitPartial` call AND no engine-mutator calls in the body
   (`AddCounter`, `MoveCard`, `DealDamage`, `LoseLife`/`GainLife`,
   `SacrificePermanent`, `TransformPermanent`, `CreateCreatureToken`,
   `CreateTreasureToken`, `RegisterReplacement`, `RegisterContinuousEffect`,
   `PushStackItem`, `FireCardTrigger`, `Surveil`/`ApplyDiscover`/`Investigate`/
   `Proliferate`, `RegisterDelayedTrigger`, `enterBattlefieldWithETB`,
   `createPermanent`, `tutorToHand`, `drawOne`, etc.) AND no sibling
   `custom_*.go` file overriding the handler.

Counts captured 2026-05-20 from `dev/percard-census-r53` rebased onto
main at `c9fdcc6`.

## Counts (R53)

| Metric | Count |
|---|---|
| Total per_card `.go` files (excl. tests) | **919** |
| `gen_*.go` files                          | **154** |
| `custom_*.go` files                       | **77** |
| `zz_*_register.go` files                  | **8** |
| Hand-written single-card files            | **681** |
| Files with `emitPartial(...)` in body     | **379** |
| Total `emitPartial(...)` call sites       | **448** |
| `gen_*.go` with `emitPartial` in body     | **60** |
| `custom_*.go` with `emitPartial` in body  | **32** |
| Hand-written with `emitPartial` in body   | **287** |
| `TODO` / `FIXME` / `XXX` markers          | **9** (narrative comments, none actionable) |
| Strict pure stubs (≤50 lines, no mutators, no custom override) | **1** |
| Unrescued `gen_*.go` with `emitPartial` and no sibling `custom_*.go` | **52** |

## Delta vs. R46

The R46 scope report focused on the dirtiest tail of the generated
handlers — it cataloged **12 pure stubs** (small `gen_*.go` files whose
entire body was `emit`/`emitPartial`) and ported 10 of them. Of those:

- All 12 are now covered. Eight ports landed in R46 itself; the four
  marked "engine-side correct" (Hamza, Rakdos, Jasmine, The Master
  Multiplied) remain breadcrumbs because the real implementation lives
  in `cost_modifiers.go` / SBA / mana pipeline — those breadcrumbs are
  pointing at the right place.

Since R46 the campaign has shipped ~13 follow-up batches:

| Batch | Release | Cards |
|---|---|---|
| `dev/percard-stubs-batchA-r49` | R49 | 10 deck-pool ports |
| `dev/percard-stubs-batchB-r49` | R49 | 10 complex handlers |
| `dev/percard-stubs-batchC-r49` | R49 | 10 commander cleanups |
| `dev/percard-stubs-batchD-r49` | R49 | 10 engine-piece ports (this report's branch lineage starts here) |
| `dev/percard-stubs-batchE-r49` | R49 | 8 defensive utility |
| `dev/percard-stubs-batchF-r50` | R50 | 10 fresh ports |
| `dev/percard-stubs-batchG-r50` | R50 | 10 commander cleanups |
| `dev/percard-stubs-batchH-r50` | R50 | 10 tribal lords |
| `dev/percard-stubs-batchH-r51` | R51 | 10 fresh ports (concurrent reuse of "H" label) |
| `dev/percard-stubs-batchI-r51` | R51 | 10 trigger-only LTB cleanups |
| `dev/percard-stubs-batchJ-r51` | R51 | 4 cheap-CMC ports |
| `dev/percard-stubs-batchK-r52` | R52 | 10 fresh ports |
| `dev/percard-stubs-batchM-r52` | R52 | 10 enchantment-form ports (this branch's parent) |
| (batchL-r52 branched but unmerged at census time) | — | — |

That's ~120+ cards ported from the original generator pool. The drop
from "~151 stub-shaped gen files" to **1 strict pure stub** is the
campaign's headline outcome.

## Remaining stub-pattern surface (52 unrescued gen_*.go)

These are `gen_*.go` files that still carry an `emitPartial` and have
no `custom_*.go` override. Sorted by file size (ascending = closer to
pure stub). The "Partial reason" column is the first `emitPartial`
message — usually the load-bearing engine gap.

| Lines | Card | Partial reason |
|------:|------|----------------|
| 47 | **Dargo, the Shipwrecker** | sac-as-additional-cost alt-cost cast pipeline |
| 51 | Jasmine Boreal of the Seven | tap-for-restricted-mana (engine mana-tag) |
| 56 | Aminatou, Veil Piercer | enchantment-miracle grant in hand (cast-path) |
| 59 | The Twelfth Doctor | demonstrate grant on first non-hand cast (cast-pipeline) |
| 67 | Raphael, Ninja Destroyer | phase-boundary mana persist (until-eot pass) |
| 69 | The Master of Keys | enchantment-escape grant (AST static layer) |
| 70 | Torbran, Thane of Red Fell | red-source-damage +2 (DealDamage replacement hook) |
| 73 | Cloud, Midgar Mercenary | trigger-doubling-when-equipped (trigger dispatch) |
| 76 | Katara, the Fearless | non-ETB triggered abilities (engine only exposes `would_fire_etb_trigger`) |
| 77 | Shiko and Narset, Unified | spell-copy-with-new-targets (stack pipeline) |
| 83 | Tam, Observant Sequencer / Deep Sight | prepare keyword (back-face resolution skipping stack) |
| 84 | Raph & Mikey, Troublemakers | bottom-in-random-order (RNG seed routing) |
| 86 | Ozai, the Phoenix King | unspent-mana-to-red (ManaEmpty replacement hook) |
| 88 | Clara Oswald | non-ETB triggered abilities (same gap as Katara) |
| 90 | Norman Osborn / Green Goblin | DFC transform + back-face graveyard-cast cost reduction |
| 90 | Storm, Force of Nature | storm-keyword grant consumption (cast pipeline) |
| 92 | Toph, the First Metalbender | return-on-die-or-exile replacement (Layer 4 type-grant) |
| 93 | Kruphix, God of Horizons | unspent-mana-becomes-colorless (ManaEmpty hook) |
| 94 | Archelos, Lagoon Mystic | ETB-tap symmetric replacement |
| 94 | Lightning, Army of One | damage-doubling replacement |
| 98 | Thrun, Breaker of Silence | §702.16 protection-from-nongreen |
| 101 | Ty Lee, Chi Blocker | "while you control" lock revert (LTB cleanup) |
| 102 | Extus, Oriq Overlord / Awaken the Blood Avatar | back-face token + sac rider separate-card surface |
| 104 | Alpharael, Stonechosen | warp-spell tracking (Void condition arming) |
| 111 | The Reaper King, No More | heuristic target choice (full target-prompt integration) |
| 114 | Neriv, Heart of the Storm | damage doubling for ETB-this-turn creatures (DealDamage hook) |
| 117 | The Jolly Balloon Man | haste static (AST owns it) |
| 122 | Nadu, Winged Wisdom | granted-target-trigger requires engine target dispatcher |
| 125 | Sen Triplets | upkeep cast-lock + hand-reveal + play-from-opponent-hand |
| 125 | The Capitoline Triad | emblem base-9/9 grant (Layer 7b set-PT) |
| 129 | Tannuk, Steadfast Second | hand-card warp {2}{R} alt-cost grant |
| 130 | Silvar, Devourer of the Free | partner-tutor target shape |
| 135 | Alaundo the Seer | per-card time-counter tracking |
| 137 | Kuja, Genome Sorcerer / Trance Kuja, Fate Defied | DFC transform hook |
| 139 | The Reality Chip | blue-pip enforcement on {2}{U} (mana-symbol path) |
| 147 | Jon Irenicus, Shattered One | control-change donate (legend rule + attachment pipeline) |
| 149 | Inspirit, Flagship Vessel | station activated + 8+ Spacecraft transition (Layer 4) |
| 150 | The Wandering Minstrel | lands-enter-untapped static (AST owns it) |
| 151 | Cecily, Haunted Mage | free I/S cast at hand ≥ 11 (alt-cost cast pipeline) |
| 153 | Ashling, Flame Dancer | unspent-red mana retention (per-phase mana pool) |
| 153 | Lara Croft, Tomb Raider | play-from-exile with discovery-counter (cast pipeline) |
| 156 | Zaffai and the Tempests | alt-cost free-cast wiring (engine cast pipeline) |
| 156 | Zidane, Tantalus Thief | opponent-gain-control "treasure trigger" event |
| 158 | Jaxis, the Troublemaker | blitz alt-cost cast pipeline |
| 159 | The Master, Transcendent | per-card mill-this-turn tracking |
| 160 | The Second Doctor | "can't attack Doctor's controller" combat-declare hook |
| 160 | Zethi, Arcane Blademaster | copy-and-cast-without-paying (cast pipeline) |
| 163 | Fire Lord Zuko | firebending mana until end of combat (mana-pool phase persist) |
| 165 | Lluwen, Exchange Student / Pest Friend | DFC transform + prepare keyword |
| 169 | Toph, Hardheaded Teacher | earthbend-on-cast trigger helper |
| 177 | Infinite Guideline Station | 12+ Spacecraft layered type-change |
| 196 | Sokrates, Athenian Teacher | combat-damage→draws conversion replacement |

**Pattern observation:** the remaining 52 are dominated by five
engine-pipeline boundaries that per-card hooks can't cross on their own:

1. **Cast-pipeline alt-costs** (free/blitz/warp/miracle/discovery
   casts): ~10 cards. Dargo, Cecily, Tannuk, Jaxis, Lara Croft,
   Zaffai, Zethi, Aminatou, Lluwen, Tam.
2. **`DealDamage` replacement hooks** (damage doubling or +N): ~4
   cards. Torbran, Neriv, Lightning, Sokrates.
3. **`ManaEmpty` replacement hooks** (mana retention / type conversion):
   ~4 cards. Kruphix, Ozai, Ashling, Fire Lord Zuko.
4. **Layer-4 type-grants / Layer-7b set-PT** (everything-counters,
   Spacecraft transitions, Capitoline emblem, Bello's elementals):
   ~6 cards. Toph 1st, Capitoline emblem, Inspirit, Infinite
   Guideline, Bello (non-gen).
5. **Triggered-ability dispatch surface** (non-ETB trigger doubling,
   target-trigger fan-out, cast-lock): ~5 cards. Clara, Katara, Nadu,
   Sen Triplets, Cloud.

Closing any one of those five boundaries at the engine layer would
unblock multiple cards at once and is the highest-leverage next move
for the campaign.

## Top 10 high-value remaining targets (R53 → R54+ scope)

Ranked by combined criteria: (a) impact when ported (engine-piece
weight, format-staple status), (b) tractability (engine primitives
exist or can be added in a small surgery), (c) avoids overlap with
known concurrent work.

### 1. Torbran, Thane of Red Fell

**Why high value:** universal red damage +2. Modal Burn / Mono-Red
staple. Currently the seat flag is set at ETB but `DealDamage` /
`MarkedDamage` never consult it. Wiring the consumer in `state.go`'s
`DealDamage` and the combat damage step is a small, surgical change
that unlocks the entire damage-doubling boundary (also lights up
Lightning, Army of One; partially Neriv).

**Engine surface needed:** wrapper in `DealDamage` and the combat
damage step that reads `gs.Flags["torbran_red_damage_*"]` and adds
`bonus × N` to each damage instance whose source is a red permanent
controlled by Torbran's controller. Add `IsRedSource(p)` helper.

### 2. Sokrates, Athenian Teacher

**Why high value:** Dialogue is a unique combat-damage replacement
("prevent that damage, both players draw half"). The handler stamps
`sokrates_dialogue_until_eot` on a target creature; the combat damage
path doesn't honor the flag. Wiring it once also creates the
infrastructure for similar damage-replacement riders (e.g.
Spellbinder/Voltaic Visionary).

**Engine surface needed:** combat damage step (`combat.go`'s
`applyCombatDamageToPlayer`) checks `attacker.Flags["sokrates_dialogue_until_eot"]`
and routes through a `PreventDamageAndConvert` helper.

### 3. Kruphix, God of Horizons + Ozai, the Phoenix King (engine pair)

**Why high value:** both block on the same gap — `ManaEmpty`
replacement. Kruphix converts unspent to colorless, Ozai converts to
red. Adding a per-seat `mana_empty_replacement` slot (slice of
{filter: func, transform: func}) in the mana pipeline closes both
plus Ashling (red mana retention) and Fire Lord Zuko (until-combat-end
mana retention).

**Engine surface needed:** `mana.EmptyPool(seat)` becomes
`mana.EmptyPool(seat)` → iterate replacements → apply transforms →
otherwise zero.

### 4. Sen Triplets

**Why high value:** "play with opponent's hand revealed + cast spells
and play lands from opponent's hand" is one of the strongest cEDH
hate cards on the metagame. The handler currently records the upkeep
opponent choice but the cast / play-from-opponent-hand surface is
absent. Wiring this also reaches Mind's Dilation, Knowledge Pool,
and other "cast from elsewhere" effects.

**Engine surface needed:** new "alt-cast-source" zone permission
slot on `Seat.Flags`. `CastSpell` consults it; cards in the source
zone become castable by the holder.

### 5. Nadu, Winged Wisdom

**Why high value:** infamous broken interaction printed in Modern
Horizons 3. Every targeting event on one of your creatures triggers
a reveal-and-play-or-draw. The handler is fully wired EXCEPT it
listens for `creature_targeted` — an event the engine doesn't fire
today. Adding `FireCardTrigger("creature_targeted", ctx)` in the
target-selection completion path is a small one-line change that
unlocks Nadu, future "becomes the target of" cards (Hexproof
counter-tricks), and Bruvac the Grandiloquent's mill-doubling.

**Engine surface needed:** single `FireCardTrigger("creature_targeted", ...)`
call in the target resolution pipeline.

### 6. Cecily, Haunted Mage

**Why high value:** "Free instant/sorcery cast when hand ≥ 11" is the
header on a popular Maximum Hand Size archetype. Closes the same
boundary as #5 in batch H (Bruvac partial) and #4 in batch K (mass-
draw payoffs).

**Engine surface needed:** alt-cost cast-pipeline hook reading
`seat.Flags["cecily_free_is_cast_available"]`. The cast handler
zeros the cost when the flag is set; the gen handler consumes the
flag after one cast per turn.

### 7. Archelos, Lagoon Mystic

**Why high value:** symmetric ETB-tapped / ETB-untapped replacement
on EVERY other permanent. Format staple in Slow / Stax Commander
shells. Currently the flag is set; nothing reads it. Adding the
ETB-tap consumer in `createPermanent` + `enterBattlefieldWithETB`
unlocks Archelos, Sphere of Resistance variants, and the partial
on Imperial Recruiter-style "ETB tapped" boards.

**Engine surface needed:** post-create-permanent hook that scans
seat flags for "etb_tapped_replacement" and applies `Tapped = true`
(or its inverse) before the ETB trigger fires.

### 8. Clara Oswald + Katara, the Fearless (engine pair)

**Why high value:** non-ETB trigger doubling — the engine today only
exposes `would_fire_etb_trigger` (which Clara/Katara consume). Most
other triggers (attack, dies, draw, etc.) bypass the replacement
slot, so Doctor's "attack" triggers and Ally combat triggers don't
double. Closing this enables tribal-trigger-doubler design space
broadly.

**Engine surface needed:** generalize `would_fire_etb_trigger` into
`would_fire_trigger` with a `trigger_kind` discriminator. The
existing replacement-registration shape already covers it.

### 9. The Reality Chip

**Why high value:** "look at top of library + cast from top of
library" is a powerful Modal/Affinity card-draw conduit. Already
partially wired (ETB look-at-top flag) but the cast-from-top
permission needs zone-cast-permission infrastructure (analogous to
Eruth, Tormented Prophet's exile-cast permission — same primitive,
different source zone).

**Engine surface needed:** `ZoneCastPermission` already exists for
exile; extend to support `Library` as the source zone.

### 10. The Master, Transcendent

**Why high value:** {T}: reanimate a creature card "milled this
turn." The current activation gates on global `Turn.Milled > 0`,
which over-approximates. Adding per-card mill-this-turn tracking
(`Card.Flags["milled_turn"] = gs.Turn` stamp in `mill()`) tightens
this and unlocks several other cards keyed on milled-this-turn (e.g.
Underworld Hermit, Splinterfright partials).

**Engine surface needed:** stamp `card.MilledTurn = gs.Turn` (or
flag-equivalent) in the mill path. Per-card filters check
`card.MilledTurn == gs.Turn`.

## Notes on the campaign trajectory

The campaign is in its **partial-cleanup phase**. The vast majority
of stub-pattern surface remaining is single-clause partials on
otherwise-complete handlers. Five engine-pipeline boundaries
account for most of the 52 remaining unrescued gen_*.go partials;
adding a per_card stub-batch port is now a lower-leverage move than
adding one engine-side primitive (per the top-10 ranking above).

Recommended R54+ rhythm: alternate one engine-surface batch (closing
a `DealDamage` / `ManaEmpty` / `creature_targeted` / cast-pipeline-
alt-cost surface) with one per_card batch consuming the newly-opened
primitive. Each engine surface tends to unlock 4-6 cards at once.
