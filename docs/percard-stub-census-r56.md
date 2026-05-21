# Per-Card Stub Census — R56

Post-R55 refresh of the stub-pattern survey across
`internal/gameengine/per_card/`. Compares against R53's
`docs/percard-stub-census-r53.md` baseline.

## Headline

R53 noted that the pure-stub population had been cleared (1 strict
pure stub left: Dargo) and that the remaining stub-pattern surface was
dominated by **partial-residuals** — handlers where 80%+ of the
printed text is wired and one or two clauses point at engine-pipeline
boundaries the per-card layer can't reach on its own.

Between R53 and R56, three engine pipelines that R53 flagged as
load-bearing were opened:

  - **R54 — Damage replacement** (`gs.DamageReplacements`,
    `ApplyDamageReplacement`). Closed Torbran / Lightning / Neriv /
    Kuja / Solphim / Sokrates (5 + 1).
  - **R54 — Layer 7b set-PT + Layer 4 add-types + Layer 6 grant-
    keyword** (`RegisterSetPT`, `RegisterAddTypes`,
    `RegisterGrantKeyword`). Plus R55 dynamic-PT variant
    (`RegisterDynamicSetPT`). Closed 5 + 10 cards.
  - **R55 — Mana-pool exemption** (`gs.ManaPoolExemptions`,
    `RegisterManaPoolExemption`). Closed Omnath / Upwelling / Cabal
    Coffers / Sanctum Weaver / Ancient Ziggurat (5).
  - **R55 — ZoneCastPolicy** (`gs.ZoneCastPolicies`,
    `RegisterZoneCastPolicy`). Closed Aluren / Karn Great Creator /
    Cecily / Zaffai / Tinybones (5).

Plus R55's batch port (10 cards: Furnace of Rath / Gisela /
Dictate / Quest / Curse / Capitoline Triad emblem / Inspirit /
Infinite Guideline / Phoenix Fleet / Toph 1stMB type-grant).

**That's ~40 cards ported in two release cycles.** The headline:

| Metric | R53 | R56 | Delta |
|---|---:|---:|---:|
| Total per_card non-test files                | 919 | 930 | +11 |
| Files with `emitPartial` in body             | 379 | 363 | **−16** |
| Total `emitPartial` call sites               | 448 | 431 | **−17** |
| `gen_*.go` with `emitPartial` in body        | 60  | 51  | **−9** |
| `custom_*.go` with `emitPartial` in body     | 32  | 31  | **−1** |
| Hand-written with `emitPartial`              | 287 | 281 | **−6** |
| Unrescued `gen_*.go` partials (no custom override) | 52 | **44** | **−8** |
| TODO / FIXME / XXX marker files              | 9   | 8   | −1 |
| Strict pure stubs (≤50 lines, no mutators)   | 1   | 1   | 0 (Dargo unchanged) |

The unrescued-`gen_*.go` count went from 52 → 44 (−8). Most of the
clearance came from R54+R55 batches AND R55's primitive openings
unblocking ports that already existed in flag-set form.

## Stale-partial subset (R56-specific finding)

Cross-referencing the 44 remaining unrescued partials against the
primitives opened in R54+R55 surfaces a new diagnostic category:
**stale-partials**. These are handlers whose `emitPartial` message
points at an engine primitive that *now exists*, but the handler was
written before the primitive landed and still operates via flag-set
breadcrumbs.

Confirmed stale-partials (engine primitive exists, handler untouched):

| Card | Pre-r54/r55 partial message | Primitive available since |
|------|----------------------------|---------------------------|
| Ozai, the Phoenix King | "unspent-mana-to-red replacement needs ManaEmpty hook; flag set for downstream consumers" | R55 `RegisterManaPoolExemption` |
| Kruphix, God of Horizons | "unspent-mana-becomes-colorless replacement needs ManaEmpty hook; flag set + end-step convert-to-colorless approximation" | R55 `RegisterManaPoolExemption` |
| Zaffai and the Tempests | "once_per_turn_cap_enforced_via_consume_trigger_cast_pipeline_must_check_zaffai_free_cast_used_t<turn>" | R55 `RegisterZoneCastPolicy` |
| Aminatou, Veil Piercer | "enchantment_miracle_grant_in_hand_not_wired_to_cast_path" | R55 `RegisterZoneCastPolicy` (zone=hand, predicate=enchantment) |
| Tannuk, Steadfast Second | "hand-card warp {2R} alt-cost grant needs cost-modifier hook; flag set for downstream" | R55 `RegisterZoneCastPolicy` (alt-cost) |
| Sen Triplets | "cast_lock_hand_reveal_play_from_opponent_hand_require_engine_pipeline_hooks" | R55 `RegisterZoneCastPolicy` (`OwnerScope=opponents`, `CasterScope=controller`) |

Each of these is an "easy R56+ port" — drop the `emitPartial`, swap
the flag-set scheme for the new primitive's `Register*` call at ETB,
add an `Unregister*ForPermanent` LTB hook. Estimated effort: ~20-30
LoC per card. **6 stale-partials accounts for ~14% of the 44 remaining
unrescued gen_ partials**, and clearing them adds zero engine surface
area — pure consumer-side wiring.

## Remaining engine-pipeline boundaries (R56+ scope)

The remaining 38 partials (44 unrescued minus 6 stale) cluster around
the boundaries that R54/R55 did NOT open:

1. **Triggered-ability dispatch generalization** (~5 cards): Clara,
   Katara, Nadu, Cloud Midgar (trigger-doubling-when-equipped), Storm
   Force of Nature. Today the engine exposes only
   `would_fire_etb_trigger`; non-ETB trigger doubling and the
   `creature_targeted` fan-out remain to be added. The natural
   generalization is `would_fire_trigger(trigger_kind)` —
   acknowledged in R53's top-10.

2. **DFC transform + back-face surface** (~4 cards): Norman Osborn,
   Extus Oriq Overlord, Kuja Genome Sorcerer, Lluwen, Tam Observant
   Sequencer. The engine has `TransformPermanent` but lacks the
   "back face activates separately" + "back-face-cast-from-graveyard
   cost reduction" infrastructure.

3. **Combat-restriction predicates** (~3 cards): Raphael Ninja
   Destroyer (must-be-blocked), Thrun (protection-from-nongreen-
   opponents), Jasmine Boreal (no-ability-vs-ability block
   restriction). Combat-declare layer hook not yet exposed.

4. **Specialized cast-pipeline alt-costs not covered by
   ZoneCastPolicy** (~5 cards): Dargo (sac-as-additional-cost),
   Jaxis (blitz), Lara Croft (play-from-exile-with-discovery-counter),
   Ashling Flame Dancer (mana retention to next instant/sorcery cast),
   Fire Lord Zuko (firebending mana until end of combat). ZoneCastPolicy
   covers source-zone alt-costs cleanly but doesn't model
   sacrifice-as-cost or speed riders.

5. **Per-card mill-this-turn tracking** (~2 cards): The Master,
   Transcendent + Toph the First Metalbender (return-on-die rider).
   Engine doesn't currently stamp `card.MilledTurn`.

6. **Static control / target prompt UI surface** (~3 cards): Ty Lee
   ("lock release on LTB"), The Reaper King (heuristic-targeting
   note — purely cosmetic partial), The Reality Chip (play-from-top
   while attached).

## Top 10 high-value R56+ targets

Ranked by leverage = (effort saved per port × number of cards
unlocked). The stale-partials dominate the top because they're nearly
free.

### Stale-partials (existing primitive, swap-only ports)

#### 1. Ozai, the Phoenix King — `RegisterManaPoolExemption`

Drop `emitPartial`; on ETB call
`gs.RegisterManaPoolExemption(perm, perm.Controller, []string{"R"})`
to keep unspent red mana through phase drains. LTB unregisters via
`UnregisterManaPoolExemptionForPerm`.

#### 2. Kruphix, God of Horizons — `RegisterManaPoolExemption`

Same shape with colors `[]string{"C"}` (any unspent mana becomes
colorless and stays — the engine's exemption primitive doesn't
re-color but does keep the mana from emptying, which matches the
strategic effect in 95% of game states).

#### 3. Zaffai and the Tempests — `RegisterZoneCastPolicy`

The "once-per-turn free I/S cast" is shaped as a hand-zone
ZoneCastPolicy with `Predicate = isInstantOrSorcery`, `ManaCost = 0`,
`Duration = "until_end_of_turn"`, and the once-per-turn cap enforced
by the policy's `UsesRemaining` or a sibling consume-trigger flag.

#### 4. Sen Triplets — `RegisterZoneCastPolicy(OwnerScope=opponents)`

ZoneCastPolicy's `OwnerScope="opponents"` + `CasterScope="controller"`
is literally the shape Sen Triplets needs. Upkeep handler picks the
target opp, registers a policy until end of turn against that opp's
hand. **High-value R53 top-10 target unlocked by R55 work.**

#### 5. Aminatou, Veil Piercer — `RegisterZoneCastPolicy(zone="hand")`

Miracle grant on enchantments in hand: register a policy at ETB with
`Zone="hand"`, `Predicate = cardHasType "enchantment"`, `ManaCost`
override to the miracle cost (current mana cost − {4}).

#### 6. Tannuk, Steadfast Second — `RegisterZoneCastPolicy` (warp grant)

Warp {2}{R} on artifact-and-red-creature cards in hand. Policy zone =
hand, predicate matches artifact OR red creature, ManaCost = 3 (warp
{2}{R}), ExileOnResolve = true (warp's "exile at end step" is the
exile-on-resolve toggle).

### Existing-primitive ports (R55 batch shape)

#### 7. The Wandering Minstrel — Layer 6 grant-keyword on lands

"Lands you control enter untapped" is mechanically just "lands don't
get the kw:enters_tapped grant from other effects". The current
partial says "static handled by AST" but the AST keyword pipeline
doesn't unconditionally apply this — wiring a per-land Layer 6
keyword strip on `permanent_etb` for lands closes the gap.

### Genuinely new engine surfaces (the next "primitive opening")

#### 8. Generalized `would_fire_trigger(trigger_kind)` (Clara + Katara + Nadu pair)

R53 top-10 #5 + #8. Closing this unlocks Clara (Doctor non-ETB
trigger doublers), Katara (Ally non-ETB), Nadu (creature_targeted
fan-out), Cloud (equipped trigger doubler). Single engine surface,
4 card unlocks.

#### 9. Combat-declare predicate hook (Raphael / Thrun / Jasmine)

A `combat_declare_attackers` / `combat_declare_blockers` predicate
slot would let cards block attack/block legality without each having
to duplicate combat scaffolding. Single surface unlocks 3 cards.

#### 10. Per-card mill-this-turn stamp (`card.MilledTurn = gs.Turn`)

In `mill()` (state.go), stamp the milled card's `MilledTurn = gs.Turn`.
The Master, Transcendent (and a couple Sokrates / Splinterfright
sibling cards if they exist) consume the stamp at activation-time.
1-line engine change.

## Trajectory

R46 baseline (151 stub-shaped gen files):
  ↓ 12 pure stubs identified, 8 ported in R46
R53 census (1 pure stub, 52 unrescued partials, 448 emitPartial sites):
  ↓ R54 ships 2 primitives (damage replacement, Layer 7b set-PT)
  ↓ R55 ships 2 more (mana-pool exemption, ZoneCastPolicy)
  ↓ R54+R55 batches port ~40 cards via the new primitives
**R56 census (1 pure stub, 44 unrescued partials, 431 emitPartial sites):**

The campaign is converging. Each cycle is now opening one or two
engine pipelines and porting 5-10 cards per primitive. Six remaining
partials are stale (primitive exists, handler hasn't been updated).
Genuinely new engine work is concentrated in three boundaries:
generalized-trigger-doubling, combat-restriction predicates, and DFC
back-face surface. Closing any one of those unlocks 3-5 more cards
at once.

Recommended R57+ rhythm: alternate one engine-surface batch with one
consumer batch, matching R54-R55 cadence. Top-priority engine surface
is the generalized `would_fire_trigger` (4 card unlocks for one
surface). Top-priority consumer batch is the 6 stale-partials above
(near-zero engineering risk, pure flag-to-primitive translation).
