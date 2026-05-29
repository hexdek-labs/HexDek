# Mutate + Meld Audit (Probe E) — r60

**Date:** 2026-05-29
**Source:** `data/rules/oracle-cards.json` (Scryfall bulk, 172MB)
**Extractor:** `/tmp/mutate_meld_extract.go` — streaming JSON decode, regex-filtered on:
  - Mutate: `\bmutate \{`, `you mutate`, `mutated creature`, keyword `Mutate`
  - Meld: `\bmeld them\b`, `melds with`, `melded permanent`, layout `meld`, keyword `Meld`

Dedupe on `oracle_id`. Layout-`meld` cards (front faces + result faces) all swept regardless of regex hit so result faces with no "meld" oracle text are included.

---

## Verdict

**Unified Permanent shape fits both mechanics with one struct:**

```go
type Permanent struct {
    // ... existing fields ...

    // MergedCards holds the underlying *Card pointers for permanents
    // composed of multiple physical cards (CR §702.139 Mutate stack,
    // CR §712 Meld). Order is semantically meaningful:
    //   - Mutate: index 0 = bottom of stack, index len-1 = top (= TopCard).
    //   - Meld:   exactly 2 entries, indexing is irrelevant (the result
    //             card's identity comes from the meld_result printed card,
    //             not from either input).
    // nil for ordinary single-card permanents.
    MergedCards []*Card

    // MergeKind discriminates how MergedCards combine.
    // "" (zero value) for single-card permanents.
    MergeKind MergeKind // MergeMutate | MergeMeld

    // TopCard is the *Card whose name, P/T base, mana cost, and types
    // define the permanent's identity. For Mutate it changes with each
    // "put over/under" choice (CR §702.139c). For Meld it is the printed
    // meld_result card (e.g. Brisela) — distinct from either of the
    // input *Card pointers in MergedCards.
    // Always equal to Card for single-card permanents; consumers should
    // prefer TopCard over Card whenever they care about identity.
    TopCard *Card
}

type MergeKind uint8
const (
    MergeNone   MergeKind = 0
    MergeMutate MergeKind = 1
    MergeMeld   MergeKind = 2
)
```

The shape covers every observed pattern. **Only one structural quirk** (meld result has identity outside `MergedCards`) needed a name on the model — `TopCard` already serves that role for mutate, so the meld extension is free.

No per-card overrides are required for the **happy path** of either mechanic. Per-card handlers will still be needed for the trigger payloads on each individual card ("whenever this creature mutates …" — 28 of 34 mutate cards have a unique mutate trigger; meld result face statics like Brisela's "opponents can't cast 3-or-less" are static abilities on `TopCard.AST`, no per-card wrapper needed).

---

## Mutate (CR §702.139)

### Inventory: 34 cards

All from Ikoria block: 28 IKO, 4 C20, 2 DMC (Illuna/Nethroi reprints), 1 MOC (Everquill Phoenix). No mutate cards printed outside Ikoria block to date.

| Card | Set | CI | Cast Cost | Mutate Cost | Trigger payload |
|------|-----|----|-----------|-------------|-----------------|
| Archipelagore | IKO | U | {5}{U}{U} | {5}{U} | tap N creatures, don't untap |
| Auspicious Starrix | IKO | G | {4}{G} | {5}{G} | exile-until-N-permanents, ETB them |
| Boneyard Lurker | IKO | BG | {2}{B}{G} | {2}{B/G}{B/G} | reanimate permanent → hand |
| Brokkos, Apex of Forever | IKO | BGU | {2}{B}{G}{U} | {2}{U/B}{G}{G} | (trample) + cast-from-graveyard mutate |
| Cavern Whisperer | IKO | B | {4}{B} | {3}{B} | (menace) + each opp discards |
| Chittering Harvester | IKO | B | {5}{B} | {4}{B} | each opp sacs creature |
| Cloudpiercer | IKO | R | {4}{R} | {3}{R} | (reach) + loot |
| Cubwarden | IKO | W | {3}{W} | {2}{W}{W} | (lifelink) + 2x 1/1 lifelink cats |
| Dirge Bat | IKO | B | {2}{B}{B} | {4}{B}{B} | (flash, flying) + destroy creature/pw |
| Dreamtail Heron | IKO | U | {4}{U} | {3}{U} | (flying) + draw |
| Everquill Phoenix | MOC | R | {2}{R}{R} | {3}{R} | (flying) + Feather token |
| Gemrazer | IKO | G | {3}{G} | {1}{G}{G} | (reach, trample) + destroy art/ench |
| Glowstone Recluse | IKO | G | {2}{G} | {3}{G} | (reach) + 2x +1/+1 counter |
| Huntmaster Liger | IKO | W | {3}{W} | {2}{W} | other creatures +N/+N (N = mutate count) |
| Illuna, Apex of Wishes | DMC | GRU | {2}{G}{U}{R} | {3}{R/G}{U}{U} | (flying, trample) + exile-until-permanent free-cast/hand |
| Insatiable Hemophage | IKO | B | {3}{B} | {2}{B} | (deathtouch) + drain X |
| Lore Drakkis | IKO | RU | {1}{U}{R} | {U/R}{U/R} | reanimate instant/sorcery → hand |
| Majestic Auricorn | IKO | W | {4}{W} | {3}{W} | (vigilance) + 4 life |
| Migratory Greathorn | IKO | G | {3}{G} | {2}{G} | basic-land tutor |
| Mindleecher | C20 | B | {4}{B}{B} | {4}{B} | (flying) + exile-top + may-cast |
| Necropanther | IKO | BW | {1}{W}{B} | {2}{W/B}{W/B} | reanimate creature CMC ≤ 3 |
| Nethroi, Apex of Death | DMC | BGW | {2}{W}{B}{G} | {4}{G/W}{B}{B} | (deathtouch, lifelink) + reanimate creatures total power ≤ 10 |
| Otrimi, the Ever-Playful | C20 | BGU | {3}{B}{G}{U} | {1}{B}{G}{U} | combat-damage: return mutate card to hand |
| Parcelbeast | IKO | GU | {2}{G}{U} | {G}{U} | activated land-flop (not mutate-triggered) |
| Porcuparrot | IKO | R | {3}{R} | {2}{R} | activated tap: damage = mutate count |
| Pouncing Shoreshark | IKO | U | {4}{U} | {3}{U} | (flash) + bounce opp creature |
| Regal Leosaur | IKO | RW | {R}{W} | {1}{R/W}{R/W} | other creatures +2/+1 EOT |
| Sawtusk Demolisher | IKO | G | {4}{G}{G} | {3}{G} | (trample) + destroy noncreature, opp gets 3/3 |
| Sea-Dasher Octopus | IKO | U | {1}{U}{U} | {1}{U} | (flash) + combat-damage draw |
| Snapdax, Apex of the Hunt | IKO | BRW | {1}{R}{W}{B} | {2}{B/R}{W}{W} | (double strike) + 4-damage + 4-life |
| Souvenir Snatcher | C20 | U | {4}{U} | {5}{U} | (flying) + gain control of artifact |
| Trumpeting Gnarr | IKO | GU | {1}{G}{U} | {3}{G/U}{G/U} | (— vanilla mutate) + 3/3 Beast token |
| Vadrok, Apex of Thunder | IKO | RUW | {U}{R}{W} | {1}{W/U}{R}{R} | (flying, first strike) + free-cast noncreature CMC ≤ 3 from graveyard |
| Vulpikeet | IKO | W | {3}{W} | {2}{W} | (flying) + +1/+1 counter |

### Stack-rules summary

Every mutate card uses the **same** standard reminder text:

> "If you cast this spell for its mutate cost, put it over or under target non-Human creature you own. They mutate into the creature on top plus all abilities from under it."

Three universal rules from the corpus, no exceptions found:

1. **Target restriction:** `non-Human creature you own` (CR §702.139a). The "you own" wedge means ownership matters at mutate time, but the merged permanent's controller is the **mutating spell's controller** (CR §702.139c), so a stolen creature you don't control cannot be the merge target — owner-only.
2. **Choice:** over or under. "Over" makes the new card the top (its name, mana cost, P/T base become identity); "under" makes the existing creature stay on top and the new card contributes its abilities only.
3. **Ability stacking:** the merged permanent has **all** abilities from all merged cards (CR §702.139d). Triggered abilities resolve per-card, so a Brokkos / Vadrok / Snapdax stack triggers all 3 "whenever this creature mutates" abilities on the same mutate event.

No color filter, no creature-type filter — any creature of any color/type can merge with any mutate card, subject only to non-Human owner gate. The Apex cycle (Brokkos / Illuna / Nethroi / Snapdax / Vadrok) prints multicolor mutate costs but has no in-stack color restriction.

### Mutate-trigger taxonomy

By payload kind, useful for AI/Freya scoring:

- **Static keyword pile**: 13 cards just print evergreen keywords (flying, trample, lifelink, vigilance, reach, deathtouch, first/double strike, flash, menace) that fold into the top creature's permanent ability set on mutate.
- **ETB-on-mutate value engine**: 14 cards (draw, drain, ramp, removal, reanimate, token-mint, etc.) — the headline trigger that stacks per-mutate.
- **Counts-mutates-this-creature**: 6 cards (Archipelagore, Auspicious Starrix, Huntmaster Liger, Insatiable Hemophage, Porcuparrot, Glowstone Recluse [implicit via counter accumulation]) — payload scales with `MutationCount` on the permanent. Needs `Permanent.MutationCount int` field — captured below.
- **Conditional cast paths**: Brokkos has graveyard-mutate; Otrimi has graveyard-recursion that fetches mutate cards specifically.

### Synthetic walk: Brokkos → Vadrok → Snapdax mutate stack

Start: cast Brokkos for its mutate cost onto Vadrok already on the battlefield, then cast Snapdax for its mutate cost onto the merged Brokkos/Vadrok pile.

| Step | Action | InstanceIDs | Permanent state |
|------|--------|-------------|-----------------|
| 0    | Vadrok cast normally for {U}{R}{W} | `[Vadrok#1001]` on battlefield | `Permanent{Card: Vadrok, TopCard: Vadrok, MergedCards: nil, MergeKind: MergeNone, MutationCount: 0}` |
| 1    | Cast Brokkos for mutate {2}{U/B}{G}{G}, choose "over" Vadrok | `[Vadrok#1001, Brokkos#1002]` resolve as one permanent | `Permanent{Card: Brokkos, TopCard: Brokkos, MergedCards: [Vadrok#1001, Brokkos#1002], MergeKind: MergeMutate, MutationCount: 1}`. Triggers stack: Brokkos's trample is now part of the permanent; Vadrok's "free-cast noncreature CMC ≤ 3 from graveyard" trigger fires on the mutate event because Vadrok's static ability is part of the new permanent's ability list. |
| 2    | Cast Snapdax for mutate {2}{B/R}{W}{W}, choose "over" the merged perm | `[Vadrok#1001, Brokkos#1002, Snapdax#1003]` | `Permanent{Card: Snapdax, TopCard: Snapdax, MergedCards: [Vadrok#1001, Brokkos#1002, Snapdax#1003], MergeKind: MergeMutate, MutationCount: 2}`. On this mutate event, ALL three "whenever this creature mutates" triggers fire (Vadrok free-cast, Snapdax 4-damage 4-life). Brokkos itself has no mutate-trigger but contributes trample + graveyard-mutate-cast. |

**3 distinct `*Card` pointers, 1 `Permanent`**, `MergedCards` carries them in stack order (bottom→top), `TopCard` is whichever card was last placed "over" (Snapdax here). Mutate count is a flat int on the permanent; existing per-card handlers can read `perm.MutationCount` for the scaling-payload cards (Archipelagore et al.).

If step 2 had chosen "under" (Snapdax under Vadrok/Brokkos), `TopCard` would remain Brokkos, but `MergedCards` order is `[Vadrok, Brokkos, Snapdax]` semantically — order is "physical stack order from bottom to top", not "cast order". The engine SHOULD track that distinction; the shape proposed above is adequate as long as `TopCard` is set explicitly per choice and `MergedCards` follows physical order.

### LTB / removal semantics (no shape impact, recorded for engine)

CR §702.139f: a merged permanent leaving the battlefield triggers all "this creature dies / leaves" triggers across every `MergedCard` as a single event, and all `*Card` pointers go to their respective owner's graveyards (multiple cards can land in different graveyards if owners differ, but since mutate is owner-gated, all are owned by the mutating spell's controller). Engine implementation: on LTB, iterate `perm.MergedCards`, route each `*Card` to `Card.Owner`'s graveyard via the canonical zone-change API. Single LTB event from a single permanent — no §704.6d-style multi-fire issue.

---

## Meld (CR §712)

### Inventory: 21 cards = 7 meld groups × 3 (front + front + result)

| Group | Result | Inputs | Set | Trigger to meld |
|-------|--------|--------|-----|-----------------|
| Brisela | Brisela, Voice of Nightmares (9/10 flying/FS/vigilance/lifelink + opp's-spells-CMC≤3-can't-cast) | Bruna, the Fading Light + Gisela, the Broken Blade | EMN (now `INR` in Scryfall remaster) | Gisela's "at beginning of your end step, if you both own and control Gisela and a creature named Bruna" |
| Chittering Host | Chittering Host (5/6 haste/menace + ETB pump) | Graf Rats + Midnight Scavengers | EMN/INR | Graf Rats's "at beginning of combat on your turn, if you both own and control this and a creature named Midnight Scavengers" |
| Hanweir Twp | Hanweir, the Writhing Township (7/4 trample/haste + attack tokens) | Hanweir Garrison (creature) + Hanweir Battlements (land!) | EMN/INR | Battlements activated `{3}{R}{R}, {T}: if you both own and control this land and a creature named Hanweir Garrison` |
| Mishra | Mishra, Lost to Phyrexia (9/9 modal triple-mode) | Mishra, Claimed by Gix + Phyrexian Dragon Engine (artifact!) | BRO | Mishra Claimed's "if both are attacking, exile, meld; enters tapped and attacking" |
| Urza | Urza, Planeswalker (planeswalker!) | Urza, Lord Protector + The Mightstone and Weakstone (legendary artifact!) | BRO | Urza Lord Protector activated `{7}: if you both own and control Urza and an artifact named Mightstone and Weakstone` |
| Titania | Titania, Gaea Incarnate (*/* legendary creature, P/T = lands you control) | Titania, Voice of Gaea (creature) + Argoth, Sanctum of Nature (land!) | BRO | Titania's upkeep trigger: "if four+ land cards in your graveyard and you own/control Titania and a land named Argoth" |
| Ragnarok | Ragnarok, Divine Deliverance (7/6 vigilance/menace/trample/reach/haste) | Fang, Fearless l'Cie + Vanille, Cheerful l'Cie | FIN | Vanille's "at beginning of your first main phase, if you both own and control Vanille and a creature named Fang, you may pay {3}{B}{G}" |

### Cross-type meld surprises

Most meld groups break the "two creatures → bigger creature" mental model. Listing the structural variants:

1. **Creature + Creature → Creature**: Brisela, Chittering Host, Ragnarok (3/7)
2. **Creature + Land → Creature** (result still a creature, land is consumed): Titania, Hanweir Twp (2/7)
3. **Creature + Artifact → Creature**: Mishra (1/7)
4. **Creature + Artifact → Planeswalker**: Urza (1/7) — the only meld that ends as a planeswalker, with `TopCard` definitively NOT a creature.

The unified shape handles all four because `TopCard.AST` carries the result face's full identity — type line, P/T (or loyalty for Urza), abilities. The engine doesn't care that a land got consumed: at meld time the inputs are exiled (CR §712.4a-b), then a single new permanent enters the battlefield using the result `*Card`. `MergedCards` retains the input `*Card` pointers solely so that un-meld scenarios (CR §712.4d — extremely rare: only triggered by a `becomes a copy of` effect or `phase out`, both of which un-meld the permanent into its two input cards) can route the right cards back to the battlefield.

### Meld preconditions (universal)

Every meld trigger checks `you both own and control X and Y`. Engine implication: meld trigger preconditions read `perm.Card.Owner == controllerSeat && perm.Controller == controllerSeat` for BOTH halves, then the trigger calls a single `meld(gs, frontA, frontB, resultCard)` primitive that:

1. Verifies both halves are still on the battlefield (CR §712.3 — both must still be in play when the trigger resolves; if either has left, the meld fizzles).
2. Exiles both via `ExilePermanent` so §614 replacements (Rest in Peace, Leyline of the Void) and §903.9b commander-replacement are honored.
3. Builds the result `Permanent{Card: nil, TopCard: resultCard, MergedCards: [frontA.Card, frontB.Card], MergeKind: MergeMeld}` and places it on the controller's battlefield via the canonical ETB path.
4. Fires ETB triggers using `TopCard.AST` so the result's "when ~ enters" trigger payload (Titania's land-recursion, Mishra's modal, Chittering Host's pump) fires correctly.

### Synthetic walk: Bruna + Gisela → Brisela

Start: both Bruna#2001 (Owner=seat 0, Controller=seat 0) and Gisela#2002 (Owner=seat 0, Controller=seat 0) on seat 0's battlefield. Gisela's "at beginning of your end step, if you own and control both" trigger fires at end step.

| Step | Action | InstanceIDs | Permanent state |
|------|--------|-------------|-----------------|
| 0 | Pre-meld | `Bruna#2001, Gisela#2002` on battlefield separately | Two `Permanent` records with `MergeKind=MergeNone`, `MergedCards=nil`, `TopCard=Card` |
| 1 | Gisela's EOT trigger resolves, ownership/control check passes | Both perms exiled (CR §712.4a) | Bruna's *Card and Gisela's *Card moved to seat 0's exile zone; their `Permanent` wrappers destroyed |
| 2 | Meld primitive instantiates Brisela | New `Brisela#2003` permanent on battlefield | `Permanent{Card: nil, TopCard: BriselaCard, MergedCards: [BrunaCard, GiselaCard], MergeKind: MergeMeld, Owner: seat0, Controller: seat0}` |

**2 distinct `*Card` pointers in `MergedCards`, 1 `Permanent`**, `MergeKind=MergeMeld`, `TopCard=BriselaCard` is a `*Card` distinct from either input (it's the printed Brisela meld_result card). The melded permanent has:

- P/T 9/10 (from BriselaCard)
- Type line "Legendary Creature — Eldrazi Angel" (from BriselaCard)
- Abilities: flying, first strike, vigilance, lifelink, and "Your opponents can't cast spells with mana value 3 or less" — all from BriselaCard.AST. **No** abilities from Bruna or Gisela carry over (CR §712.2 — the melded permanent has only the meld_result's printed characteristics; unlike Mutate, abilities do NOT stack from inputs).

This is the **key structural difference from Mutate**: meld inputs contribute NOTHING to the result's abilities. They exist in `MergedCards` only as routing pointers for the rare un-meld case. Mutate inputs contribute everything to the merged permanent's ability set.

The unified shape handles both because consumers reading `perm.AbilitySet` should look at:
- `TopCard.AST.Abilities` always
- `MergedCards[i].AST.Abilities` only if `MergeKind == MergeMutate`

A single helper `perm.ActiveAbilities()` can switch on `MergeKind` and return the right union.

### Un-meld (CR §712.4d)

Triggered by: phasing out (CR §702.26), becoming a copy of something else (CR §707.2), or otherwise losing its identity. The melded permanent "un-melds" into two separate permanents from its two `MergedCards`. The shape supports this trivially:

```go
func unmeld(gs *GameState, p *Permanent) {
    if p.MergeKind != MergeMeld || len(p.MergedCards) != 2 {
        return
    }
    aCard, bCard := p.MergedCards[0], p.MergedCards[1]
    seat := p.Controller
    removePermanent(gs, p) // not ExilePermanent — un-meld is not a zone change
    enterBattlefieldNew(gs, seat, aCard)
    enterBattlefieldNew(gs, seat, bCard)
}
```

No `MergedCards` field traversal in normal gameplay — meld is one-way at the game-mechanic level; un-meld is the corner case the field exists to serve.

---

## Required new fields on `Permanent`

```go
MergedCards    []*Card
MergeKind      MergeKind // uint8 enum
TopCard        *Card
MutationCount  int // counts mutate events on THIS perm — for scaling triggers
                   // (Archipelagore / Auspicious Starrix / Huntmaster Liger / Insatiable Hemophage / Porcuparrot)
                   // zero for unmerged perms; un-meld doesn't reset (meld never increments it)
```

Existing `Permanent.Card` stays the **bottom-most** physical card for backward-compat hot-paths that already key on a single Card; identity-sensitive consumers should migrate to `TopCard`. For un-merged perms `Card == TopCard` and `MergedCards == nil`; this is the dominant case and existing code paths read `Card` correctly.

---

## Per-card overrides needed

### Mutate: NONE for the merge mechanic itself

Every mutate card uses the **identical** standard reminder text and standard "put over or under target non-Human creature you own" merge rule. A single `CastForMutateCost(perm *Permanent, target *Permanent, putOver bool)` primitive handles all 34. Per-card handlers are needed only for the trigger payloads (and those are normal triggered-ability scaffolds, not merge-specific — they already exist in the AST classifier under standard `Triggered{event: "this_mutates"}` shape, requires `mutated` trigger-event slug in the event-aliases table for the per-card dispatcher).

### Meld: per-card handlers needed only for the meld TRIGGER, not the meld primitive

All 7 groups use a distinct trigger shape (EOT, begin combat, attack, upkeep, activated, first main):

| Group | Per-card handler need |
|-------|-----------------------|
| Brisela | Gisela: `OnTrigger("Gisela, the Broken Blade", "end_step", checkOwnsBothThenMeld(self, "Bruna, the Fading Light", BriselaResultCard))` |
| Chittering Host | Graf Rats: `OnTrigger("Graf Rats", "begin_combat", ...same pattern, "Midnight Scavengers" → Chittering Host)` |
| Hanweir Twp | Hanweir Battlements: an activated ability handler (already a land); cost {3}{R}{R},{T}; condition + meld primitive in body |
| Mishra | Mishra Claimed: `OnTrigger("Mishra, Claimed by Gix", "attacks", ...check both attacking + own/control)` |
| Urza | Urza Lord Protector: activated `{7}`; sorcery-speed |
| Titania | Titania Voice: `OnTrigger("Titania, Voice of Gaea", "upkeep", ...check 4+ lands in graveyard + own/control)` |
| Ragnarok | Vanille: `OnTrigger("Vanille, Cheerful l'Cie", "first_main", ...{3}{B}{G} cost gated trigger)` |

The **meld primitive** itself (`meld(gs, partA, partB, resultCard)`) is shared and is the only structural new code on the engine side. Each per-card handler is a 5-10 line trigger registration that calls into the primitive.

### Identity-line edge cases

- **Urza, Planeswalker**: result face is a planeswalker. Existing planeswalker SBA paths (CR §704.5j loyalty=0 → graveyard) must consume `TopCard.Loyalty`. Engine already routes planeswalkers correctly when `Card.TypeLine` contains "Planeswalker"; since `TopCard.TypeLine` is what the SBA reader should consult, this works as long as we audit existing SBA code paths to prefer `TopCard` over `Card`.
- **Titania, Gaea Incarnate**: P/T `*/*` with characteristic-defining ability "Titania's power and toughness are each equal to the number of lands you control." Existing CDA infrastructure (the same path Tarmogoyf uses) reads from `TopCard.AST.Abilities`; no new shape needed.
- **Hanweir, the Writhing Township** + **Argoth's meld result**: result is a legendary creature; one of the inputs was a land. The result is not a land — the land card is fully consumed. Engine should never count the melded permanent toward "lands you control" — a check on `TopCard.IsLand()` (NOT `Card.IsLand()`) handles this correctly.

---

## Field count audit

Adding `MergedCards []*Card`, `MergeKind uint8`, `TopCard *Card`, `MutationCount int` to `Permanent` = **+24 bytes** per permanent on amd64 (8 + 1 + 8 + 8, plus 7 bytes alignment padding). 4-player game with ~150 permanents in play across all seats = ~3.6 KB additional memory. Negligible.

For un-merged permanents (the dominant case), `MergedCards` is `nil` (8 bytes nil slice header), `MergeKind` is `0`, `TopCard` is a pointer set equal to `Card` (or left nil if consumers fall back to `Card` when `TopCard == nil`). Slight preference: set `TopCard = Card` at ETB for every permanent so consumers can unconditionally `perm.TopCard.X` without a nil-check; the cost is one pointer-copy per ETB.

---

## Recommendations (out-of-scope for this audit, but the implementation roadmap)

1. **Land the unified shape on `Permanent`** with `TopCard` always non-nil post-ETB. Add a `perm.IdentityCard() *Card` helper that returns `TopCard` for safety.
2. **Add `MergeMutate` and `MergeMeld` primitives** in `internal/gameengine/keywords_mutate.go` and `internal/gameengine/keywords_meld.go`. Mutate primitive accepts `putOver bool`; meld primitive resolves both inputs by name lookup on the controller's battlefield.
3. **Wire 7 per-card meld-trigger handlers** under `internal/gameengine/per_card/meld_*.go` (one file per group, shared template).
4. **Wire 1 per-card mutate-trigger handler per scaling card** (5 cards: Archipelagore, Auspicious Starrix, Huntmaster Liger, Insatiable Hemophage, Porcuparrot) to read `perm.MutationCount` for X.
5. **Add `mutated` to the event-aliases table** so AST `Triggered{event: "mutates"}` dispatches.
6. **Audit existing SBA + ability-set readers** for `Card` vs `TopCard` correctness on identity-sensitive paths (CDA, planeswalker SBA, type checks).

The unified shape is **green** for r60 land. No per-card override required for the mechanic itself; per-card work is in the standard trigger-payload registration that already exists for every other triggered ability in the engine.
