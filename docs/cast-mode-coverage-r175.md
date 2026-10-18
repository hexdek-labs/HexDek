# Cast-mode clause coverage — r175 measurement

**Date:** 2026-10-18 · **Corpus:** live `~/hexdek/data/rules/ast_dataset.jsonl` from
192.168.1.206, exported 2026-10-16, **32,417 card rows** (+1 manifest line = 32,418
lines). The repo's local `data/rules/ast_dataset.jsonl` is from June and was NOT used.
**This is a measurement only — no production code was changed.**

---

## 1. Calibration — reproduced

Scanning every AST node with `__ast_type__ in {Modification, ModificationEffect}` and
`kind == "cast_mode_check"`:

```
cards: 32417
cards with cast_mode_check: 152
  kicked 98   from_graveyard 14   bargained 9   five_or_more_mana_spent 9
  additional_cost_paid 8   from_exile 4   foretold 3   madness_cost_paid 3
  prowl_cost_paid 2   surge_cost_paid 1   from_hand 1
```

**152 — exact match** to `internal/gameengine/cast_mode_r137.go`, mode-for-mode.
Traversal confirmed correct before going further.

## 2. Headline answer

**The premise is TRUE, and the gap is larger than the 152 suggests.**

| | count | of 32,417 |
|---|---|---|
| Oracle text carries a cast-mode conditional clause | **434** | 1.34% |
| → **A.** AST carries `cast_mode_check` | 152 | 35% of 434 |
| → **B.** AST carries a *different but engine-evaluable* node | 46 | 11% |
| → **C.** AST carries nothing the engine can evaluate | **236** | 54% |

Classifying bucket C:

| class | count |
|---|---|
| **GENUINE MISS** | **197** |
| handled elsewhere — runtime raw-text re-parser | 21 |
| handled elsewhere — dedicated `per_card` handler | 18 |
| unclassified | 0 |

**Genuine misses: 197 of the 434 clause-bearing cards (45%), against a denominator
of 32,417 rows.** Of those 197, **181** concern the card's own cast mode and
**16** ask about another spell's cast mode.

Against the 152 the engine knows about, there are **197 more** cards expressing
the same idea in a phrasing nothing reads.

## 3. It is worse than "silently does nothing"

The dominant miss shape is **not** an absent node. It is a node the exporter *did*
emit, in a shape the engine fails **closed** on.

`Oran-Rief Recluse` — "When this creature enters, **if it was kicked**, destroy
target creature with flying" — exports as:

```
Triggered{ trigger: etb,
  effect: Conditional{
     condition: Condition{kind:"if", args:["it was kicked"]},
     body: Destroy{...} } }
```

`evalCondition` (`internal/gameengine/resolve.go:417`) has **no `case "if"`**. Its
tail (`resolve.go:618-632`) fails closed and logs `condition_unrecognized`. So the
card does nothing **whether or not it was kicked** — paying the kicker buys
nothing. Verified by reading the full case list of `evalCondition`, and by
confirming `internal/astload/loader.go:347 decodeCondition` is a pass-through that
performs no normalisation.

Unrecognised `Condition` kinds across the 434: `if` (107), `etb_if` (13),
`conditional` (5), `was_kicked` (3), `no_mana_spent_to_cast` (2). Only `kicked`
(46) has a case.

## 4. What each bucket is, and how I checked it

**A — `cast_mode_check`, 152.** The documented baseline. Note (from
`cast_mode_r137.go` itself, not re-derived): only ONE reader exists — split second
— and the `if_intervening_tail` path still assumes the condition true. A is
"represented", not "evaluated".

**B — different structured node, 46.** All 46 are `Condition{kind:"kicked",
args:[N]}`, which the exporter produces for the wording "*If this creature was
kicked, it enters with N +1/+1 counters*" (Kavu Primarch, Aether Figment,
Cragplate Baloth, …). `evalCondition` has `case "kicked"` reading
`src.Flags["kicked"]`, stamped by `keywords_batch.go:130`. **Verified by code
read**, not runtime-tested. The headline point: the same English idea has **three**
different exporter outputs depending on the sentence it sits in. That is the
structural cause of the gap.

**C-handled, runtime raw-text re-parser — 21.** `Modification{kind:
"conditional_effect"}` keeps the clause as raw text, and `resolveConditionalEffect`
/ `parseConditionText` (`internal/gameengine/conditional_parser.go`) re-parse it at
runtime with `reKicked`, `reYouCastIt`, `reSelfWasCast`, `reManaSpent`.
**Verified** by replaying those exact Go regexes against each card's real
`conditional_effect` argument string; 21 of 24 matched (the 3 that did not —
Earwig Squad, Rocket-Powered Goblin Glider, Void Mirror — stay in the miss list).
*Caveats:* this path rescues only the **gate**; if `parseEffectText` returns nil the
body still does nothing. And it mis-rescues — `Nightscape Battlemage`'s "*it was
kicked with its {2}{U} kicker*" matches the bare `reKicked` prefix, so kicking with
the **other** kicker fires the wrong mode. That is a correctness bug, not a
silence bug.

**C-handled, `per_card` — 18.** The card name is registered in
`internal/gameengine/per_card/`. **Verified** registration by grep for each; gate
logic read in full for two (`breachingLeviathanETB` gates on
`perm.Flags["was_cast"]`, `troublemakerOupheETB` on `permWasBargained`). The other
16 are **inferred from registration alone**.

**Unclassified — 0, after resolution.** Ten genuine misses carry
`Modification{kind:"custom", args:["<slug>"]}` — Orim's Chant
(`orims_chant_kicked_no_attacks`), Beseech the Mirror
(`beseech_mirror_cast_for_free_if_bargained_mv_4_or_less`), Vines of Vastwood,
Verdeloth the Ancient, Thunderous Debut, Hunting Wilds, Once and Future, Radiant
Performer, Secret of Bloodbending, Nine-Lives Familiar. I grepped all ten slugs
across `internal/`: **0 hits each**, so nothing implements them;
`resolveResidualByText` receives the underscored slug as prose. Counted as genuine
misses with that evidence stated rather than parked as unknown.

## 5. Deliberately excluded from the 434

**41 cards** matching "*if you've cast another spell this turn*" (Arclight Phoenix,
Stormwing Entity, the "*if you've cast a noncreature spell this turn*" Marvel
cycle, the Adamant-adjacent cost-reducers). That is turn history, not a property of
how **this** spell was cast, and it needs a spell-count tracker rather than a
`StackItem` read. Reported separately so the owner can fold it in if he disagrees —
most of them are *also* unevaluated, via the same `Condition{kind:"if"}`.

## 6. Vocabulary the exporter never learned

Built by reading the corpus, not from memory: I enumerated every distinct tail of
`if <self> was …` across all 32,417 rows. The phrasings that produce **no**
`cast_mode_check`:

| phrasing | genuine-miss cards (§7a primary label) | exporter output |
|---|---|---|
| "if **it** was kicked" (pronoun, inside a trigger) | 74 | `Condition{kind:"if"}`, raw text |
| "if you cast it from your hand / a graveyard / exile" | 34 | `Condition{kind:"if"}` or `etb_if` |
| "Adamant — if at least three X mana was spent…" | 20 | `parsed_tail`, raw text |
| "if its prowl / surge / spectacle / madness / freerunning / sneak / mayhem cost was paid" | 16 | `Condition{kind:"if"}` / `if_intervening_tail` |
| "if you cast it" (bare — cast vs. reanimated) | 15 | `Condition{kind:"if"}` |
| "if this spell was cast using **Teamwork**" | 15 | **nothing** (no grammar at all) |
| "if it was bargained" (pronoun) | 5 | `Condition{kind:"if"}` |
| "if it was embalmed" / "if it was the first spell you've cast this game" | 2 | **nothing** |

The pattern is mechanical. The exporter emits `cast_mode_check` for the
**"if this spell was …"** sentence form. Swap the subject to a pronoun ("it") or to
the permanent ("this creature"), or move the clause into a triggered ability's
intervening-if, and it degrades to raw text.

---

## 7. The reviewable list

### 7a. Genuine misses — the card's own cast mode (181)


#### kicked — 74 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| A-Sprouting Goblin | When Sprouting Goblin enters, if it was kicked, search your library for a land card with a basic land type, reveal it, put it into your hand, then shu | `Condition{kind:if}` |
| Ana Battlemage | When this creature enters, if it was kicked with its {2}{U} kicker, target player discards three cards. | `Condition{kind:if}` |
| Archangel of Wrath | When this creature enters, if it was kicked, it deals 2 damage to any target. | `Condition{kind:if}` |
| Balduvian Atrocity | When this creature enters, if it was kicked, return target creature card with mana value 3 or less from your graveyard to the battlefield. | `Condition{kind:if}` |
| Battlewing Mystic | When this creature enters, if it was kicked, discard your hand, then draw two cards. | `Condition{kind:if}` |
| Benalish Emissary | When this creature enters, if it was kicked, destroy target land. | `Condition{kind:if}` |
| Benalish Sleeper | When this creature enters, if it was kicked, each player sacrifices a creature of their choice. | `Condition{kind:if}` |
| Bog Badger | When this creature enters, if it was kicked, creatures you control gain menace until end of turn. | `Condition{kind:if}` |
| Bubble Snare | When this Aura enters, if it was kicked, tap enchanted creature. | `Condition{kind:if}` |
| Caligo Skin-Witch | When this creature enters, if it was kicked, each opponent discards two cards. | `Condition{kind:if}` |
| Citanul Woodreaders | When this creature enters, if it was kicked, draw two cards. | `Condition{kind:if}` |
| Cleaving Skyrider | When this creature enters, if it was kicked, it deals X damage to any target, where X is the number of attacking creatures. | `Condition{kind:if}` |
| Cunning Geysermage | When this creature enters, if it was kicked, return up to one other target creature to its owner's hand. | `Condition{kind:if}` |
| Elvish Hydromancer | When this creature enters, if it was kicked, create a token that's a copy of target creature you control. | `Condition{kind:if}` |
| Ertai's Trickery | Counter target spell if it was kicked. | `Modification{typed_spell_effect}` |
| Excavation Elephant | When this creature enters, if it was kicked, return target artifact card from your graveyard to your hand. | `Condition{kind:if}` |
| Gatekeeper of Malakir | When this creature enters, if it was kicked, target player sacrifices a creature of their choice. | `Condition{kind:if}` |
| Ghitu Amplifier | When this creature enters, if it was kicked, return target creature an opponent controls to its owner's hand. | `Condition{kind:if}` |
| Ghitu Chronicler | When this creature enters, if it was kicked, return target instant or sorcery card from your graveyard to your hand. | `Condition{kind:if}` |
| Gigantiform | When this Aura enters, if it was kicked, you may search your library for a card named Gigantiform, put it onto the battlefield, then shuffle. | `Condition{kind:if}` |
| Goblin Bushwhacker | When this creature enters, if it was kicked, creatures you control get +1/+0 and gain haste until end of turn. | `Condition{kind:if}` |
| Goblin Ruinblaster | When this creature enters, if it was kicked, destroy target nonbasic land. | `Condition{kind:if}` |
| Heartstabber Mosquito | When this creature enters, if it was kicked, destroy target creature. | `Condition{kind:if}` |
| Hunting Wilds | If this spell was kicked, untap all Forests put onto the battlefield this way. | `Modification{custom}` |
| Illuminate | If this spell was kicked with its {2}{R} kicker, it deals X damage to that creature's controller. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Josu Vess, Lich Knight | When Josu Vess enters, if it was kicked, create eight 2/2 black Zombie Knight creature tokens with menace. | `Condition{kind:if}` |
| Juniper Order Rootweaver | When this creature enters, if it was kicked, put a +1/+1 counter on target creature you control. | `Condition{kind:if}` |
| Kangee, Aerie Keeper | When Kangee enters, if it was kicked, put X feather counters on it. | `Condition{kind:if}` |
| Keldon Overseer | When this creature enters, if it was kicked, gain control of target creature until end of turn. | `Condition{kind:if}` |
| Keldon Strike Team | When this creature enters, if it was kicked, create two 1/1 white Soldier creature tokens. | `Condition{kind:was_kicked}` |
| Kitesail Cleric | When this creature enters, if it was kicked, tap up to two target creatures. | `Condition{kind:if}` |
| Kor Aeronaut | When this creature enters, if it was kicked, target creature gains flying until end of turn. | `Condition{kind:if}` |
| Kor Sanctifiers | When this creature enters, if it was kicked, destroy target artifact or enchantment. | `Condition{kind:if}` |
| Krosan Druid | When this creature enters, if it was kicked, you gain 10 life. | `Condition{kind:if}` |
| Mold Shambler | When this creature enters, if it was kicked, destroy target noncreature permanent. | `Condition{kind:if}` |
| Monstrous War-Leech | As this creature enters, if it was kicked, mill four cards. | `Condition{kind:if}` |
| Murasa Sproutling | When this creature enters, if it was kicked, return target card with a kicker ability from your graveyard to your hand. | `Condition{kind:if}` |
| Nullpriest of Oblivion | When this creature enters, if it was kicked, return target creature card from your graveyard to the battlefield. | `Condition{kind:if}` |
| Oran-Rief Recluse | When this creature enters, if it was kicked, destroy target creature with flying. | `Condition{kind:if}` |
| Orim's Chant | If this spell was kicked, creatures can't attack this turn. | `Modification{custom}` |
| Phyrexian Missionary | When this creature enters, if it was kicked, return target creature card from your graveyard to your hand. | `Condition{kind:if}` |
| Phyrexian Warhorse | When this creature enters, if it was kicked, create a 1/1 white Soldier creature token. | `Condition{kind:was_kicked}` |
| Roost of Drakes | When this enchantment enters, if it was kicked, create a 2/2 blue Drake creature token with flying. | `Condition{kind:if}` |
| Scorch Rider | When this creature enters, if it was kicked, it gains haste until end of turn. | `Condition{kind:if}` |
| Scourge of the Skyclaves | When you cast this spell, if it was kicked, each player loses half their life, rounded up. | `Modification{cast_trigger_tail/self_calculated_pt}` |
| Sergeant-at-Arms | When this creature enters, if it was kicked, create two 1/1 white Soldier creature tokens. | `Condition{kind:was_kicked}` |
| Sheoldred's Restoration | If this spell was kicked, you gain life equal to that card's mana value. | `Condition{kind:conditional}` |
| Shivan Emissary | When this creature enters, if it was kicked, destroy target nonblack creature. | `Condition{kind:if}` |
| Skizzik | At the beginning of the end step, if this creature wasn't kicked, sacrifice it. | `Condition{kind:if}` |
| Sowing Mycospawn | When you cast this spell, if it was kicked, exile target land. | `Modification{cast_trigger_tail}` |
| Sphinx of Lost Truths | Then if it wasn't kicked, discard three cards. | `Condition{kind:if}` |
| Sprouting Goblin | When this creature enters, if it was kicked, search your library for a land card with a basic land type, reveal it, put it into your hand, then shuffl | `Condition{kind:if}` |
| Stormscape Battlemage | When this creature enters, if it was kicked with its {W} kicker, you gain 3 life. | `Condition{kind:if}` |
| Sun-Blessed Healer | When this creature enters, if it was kicked, return target nonland permanent card with mana value 2 or less from your graveyard to the battlefield. | `Condition{kind:if}` |
| Sunscape Battlemage | When this creature enters, if it was kicked with its {1}{G} kicker, destroy target creature with flying. | `Condition{kind:if}` |
| Tajuru Paragon | When this creature enters, if it was kicked, reveal the top six cards of your library. | `Condition{kind:if}` |
| Tazeem Roilmage | When this creature enters, if it was kicked, return target instant or sorcery card from your graveyard to your hand. | `Condition{kind:if}` |
| Tempest Owl | When this creature enters, if it was kicked, tap up to three target permanents. | `Condition{kind:if}` |
| Thieving Skydiver | When this creature enters, if it was kicked, gain control of target artifact with mana value X or less. | `Condition{kind:if}` |
| Thornscape Battlemage | When this creature enters, if it was kicked with its {R} kicker, it deals 2 damage to any target. | `Condition{kind:if}` |
| Thunderscape Battlemage | When this creature enters, if it was kicked with its {1}{B} kicker, target player discards two cards. | `Condition{kind:if}` |
| Tide Shaper | When this creature enters, if it was kicked, target land becomes an Island for as long as this creature remains on the battlefield. | `Modification{become_basic_land/self_buff}` |
| Tolarian Emissary | When this creature enters, if it was kicked, destroy target enchantment. | `Condition{kind:if}` |
| Torch Slinger | When this creature enters, if it was kicked, it deals 2 damage to target creature. | `Condition{kind:if}` |
| Tourach, Dread Cantor | When Tourach enters, if it was kicked, target opponent discards two cards at random. | `Condition{kind:if}` |
| Urborg Emissary | When this creature enters, if it was kicked, return target permanent to its owner's hand. | `Condition{kind:if}` |
| Verdeloth the Ancient | When Verdeloth enters, if it was kicked, create X 1/1 green Saproling creature tokens. | `Modification{custom}` |
| Verduran Emissary | When this creature enters, if it was kicked, destroy target artifact. | `Condition{kind:if}` |
| Vigorous Charge | Whenever that creature deals combat damage this turn, if this spell was kicked, you gain life equal to that damage. | `Modification{typed_spell_effect/untyped_effect}` |
| Vines of Vastwood | If this spell was kicked, that creature gets +4/+4 until end of turn. | `Modification{custom}` |
| Vineshaper Prodigy | When this creature enters, if it was kicked, look at the top three cards of your library. | `Condition{kind:if}` |
| Wastescape Battlemage | When you cast this spell, if it was kicked with its {G} kicker, exile target artifact or enchantment an opponent controls. | `Modification{cast_trigger_tail}` |
| Waterspout Elemental | When this creature enters, if it was kicked, return all other creatures to their owners' hands and you skip your next turn. | `Condition{kind:if}` |
| Yavimaya Iconoclast | When this creature enters, if it was kicked, it gets +1/+1 and gains haste until end of turn. | `Condition{kind:if}` |

#### cast from a named zone — 34 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| Amped Raptor | Then if you cast it from your hand, exile cards from the top of your library until you exile a nonland card. | `Condition{kind:if}` |
| Angel of the Dire Hour | When this creature enters, if you cast it from your hand, exile all attacking creatures. | `Condition{kind:if}` |
| Annie Flash, the Veteran | When Annie Flash enters, if you cast it, return target permanent card with mana value 3 or less from your graveyard to the battlefield tapped. | `Condition{kind:if}` |
| Antiquities on the Loose | Then if this spell was cast from anywhere other than your hand, put a +1/+1 counter on each Spirit you control. | `Condition{kind:if}` |
| Coal Stoker | When this creature enters, if you cast it from your hand, add {R}{R}{R}. | `Condition{kind:if}` |
| Deathbringer Regent | When this creature enters, if you cast it from your hand and there are five or more other creatures on the battlefield, destroy all other creatures. | `Condition{kind:if}` |
| Dread Cacodemon | When this creature enters, if you cast it from your hand, destroy all creatures your opponents control, then tap all other creatures you control. | `Condition{kind:if}` |
| Feasting Troll King | When this creature enters, if you cast it from your hand, create three Food tokens. | `Condition{kind:if}` |
| From Father to Son | If this spell was cast from a graveyard, put that card onto the battlefield instead. | `Condition{kind:conditional}` |
| Furnace Dragon | When this creature enters, if you cast it from your hand, exile all artifacts. | `Condition{kind:if}` |
| Hypnox | When this creature enters, if you cast it from your hand, exile all cards from target opponent's hand. | `Condition{kind:if}` |
| Iname as One | When Iname as One enters, if you cast it from your hand, you may search your library for a Spirit permanent card, put it onto the battlefield, then sh | `Condition{kind:if/paid_optional_cost}` |
| Increasing Ambition | If this spell was cast from a graveyard, instead search your library for two cards and put those cards into your hand. | `Condition{kind:conditional}` |
| Myojin of Blooming Dawn | Myojin of Blooming Dawn enters with an indestructible counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Myojin of Cleansing Fire | Myojin of Cleansing Fire enters with a divinity counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Myojin of Cryptic Dreams | Myojin of Cryptic Dreams enters with an indestructible counter on it if you cast it from your hand. | `Condition{kind:etb_if/repeat_n}` |
| Myojin of Grim Betrayal | Myojin of Grim Betrayal enters with an indestructible counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Myojin of Infinite Rage | Myojin of Infinite Rage enters with a divinity counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Myojin of Life's Web | Myojin of Life's Web enters with a divinity counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Myojin of Night's Reach | Myojin of Night's Reach enters with a divinity counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Myojin of Roaring Blades | Myojin of Roaring Blades enters with an indestructible counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Myojin of Seeing Winds | Myojin of Seeing Winds enters with a divinity counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Myojin of Towering Might | Myojin of Towering Might enters with an indestructible counter on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Otterball Antics | If this spell was cast from anywhere other than your hand, put a +1/+1 counter on that creature. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Patched Plaything | This creature enters with two -1/-1 counters on it if you cast it from your hand. | `Condition{kind:etb_if}` |
| Radiant Performer | When this creature enters, if you cast it from your hand, choose target spell or ability that targets only a single permanent or player. | `Modification{custom}` |
| Reiver Demon | When this creature enters, if you cast it from your hand, destroy all nonartifact, nonblack creatures. | `Condition{kind:if}` |
| Retrieve the Esper | Then if this spell was cast from a graveyard, put two +1/+1 counters on that token. | `Condition{kind:if}` |
| Rocket-Powered Goblin Glider | When this Equipment enters, if it was cast from your graveyard, attach it to target creature you control. | `Modification{conditional_effect/equip_buff_grant}` |
| Scion of Vitu-Ghazi | When this creature enters, if you cast it from your hand, create a 1/1 white Bird creature token with flying, then populate. | `Condition{kind:if}` |
| See the Truth | If this spell was cast from anywhere other than your hand, put each of those cards into your hand instead. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Soulhunter Rakshasa | When this creature enters, if you cast it from your hand, it deals 1 damage to target opponent for each Swamp you control. | `Condition{kind:if}` |
| Transpose | If this spell was cast from your hand, create a 0/1 black Wizard creature token with "Whenever you cast a noncreature spell, this token deals 1 damage | `Condition{kind:conditional}` |
| Wakening Sun's Avatar | When this creature enters, if you cast it from your hand, destroy all non-Dinosaur creatures. | `Condition{kind:if}` |

#### mana spent to cast (incl. Adamant cycle) — 20 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| Ardenvale Paladin | Adamant — If at least three white mana was spent to cast this spell, this creature enters with a +1/+1 counter on it. | `Modification{parsed_tail}` |
| Cauldron's Gift | Adamant — If at least three black mana was spent to cast this spell, mill four cards. | `Condition{kind:paid_optional_cost}` |
| Desecrate Reality | Adamant — If at least three colorless mana was spent to cast this spell, return a permanent card with an odd mana value from your graveyard to the bat | `Condition{kind:for_each}` |
| Embereth Paladin | Adamant — If at least three red mana was spent to cast this spell, this creature enters with a +1/+1 counter on it. | `Modification{parsed_tail}` |
| Foreboding Fruit | Adamant — If at least three black mana was spent to cast this spell, create a Food token. | `Modification{parsed_tail/typed_spell_effect}` |
| Freestrider Commando | This creature enters with two +1/+1 counters on it if it wasn't cast or no mana was spent to cast it. | `Condition{kind:etb_if}` |
| Garenbrig Paladin | Adamant — If at least three green mana was spent to cast this spell, this creature enters with a +1/+1 counter on it. | `Modification{block_restriction_power/parsed_tail}` |
| Locthwain Paladin | Adamant — If at least three black mana was spent to cast this spell, this creature enters with a +1/+1 counter on it. | `Modification{parsed_tail}` |
| Nix | Counter target spell if no mana was spent to cast it. | `Modification{typed_spell_effect}` |
| Once and Future | Adamant — If at least three green mana was spent to cast this spell, instead return those cards to your hand and exile Once and Future. | `Modification{custom}` |
| Outmuscle | Adamant — If at least three green mana was spent to cast this spell, the creature you control gains indestructible until end of turn. | `Modification{parsed_tail/typed_spell_effect}` |
| Rally for the Throne | Adamant — If at least three white mana was spent to cast this spell, you gain 1 life for each creature you control. | `Modification{parsed_tail/typed_spell_effect}` |
| Satoru, the Infiltrator | Whenever Satoru and/or one or more other nontoken creatures you control enter, if none of them were cast or no mana was spent to cast them, draw a car | `Condition{kind:if}` |
| Searing Barrage | Adamant — If at least three red mana was spent to cast this spell, Searing Barrage deals 3 damage to that creature's controller. | `Modification{parsed_tail/typed_spell_effect}` |
| Silverflame Ritual | Adamant — If at least three white mana was spent to cast this spell, creatures you control gain vigilance until end of turn. | `Modification{parsed_tail/typed_spell_effect}` |
| Slaying Fire | Adamant — If at least three red mana was spent to cast this spell, it deals 4 damage instead. | `Modification{parsed_tail/typed_spell_effect}` |
| Sundering Stroke | If at least seven red mana was spent to cast this spell, instead Sundering Stroke deals 7 damage to each of those permanents and/or players. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Turn into a Pumpkin | Adamant — If at least three blue mana was spent to cast this spell, create a Food token. | `Modification{parsed_tail/typed_spell_effect}` |
| Unexplained Vision | Adamant — If at least three blue mana was spent to cast this spell, scry 3. | `Modification{parsed_tail/typed_spell_effect}` |
| Vantress Paladin | Adamant — If at least three blue mana was spent to cast this spell, this creature enters with a +1/+1 counter on it. | `Modification{parsed_tail}` |

#### a named alternative/additional cost was paid — 16 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| Baleful Mastery | If the {1}{B} cost was paid, an opponent draws a card. | `Modification{if_intervening_tail/rather_than_pay/typed_spell_effect}` |
| Celestial Reunion | If this spell's additional cost was paid and the revealed card is the chosen type, put that card onto the battlefield instead of putting it into your  | `Modification{additional_cost/if_intervening_tail/typed_spell_effect}` |
| Cinder Strike | It deals 4 damage to that creature instead if this spell's additional cost was paid. | `Modification{additional_cost/typed_spell_effect}` |
| Devastating Mastery | If the {2}{W}{W} cost was paid, an opponent chooses up to two nonland permanents they control and returns them to their owner's hand. | `Modification{if_intervening_tail/rather_than_pay/typed_spell_effect}` |
| Earwig Squad | When this creature enters, if its prowl cost was paid, search target opponent's library for three cards and exile them. | `Modification{conditional_effect}` |
| Fervent Mastery | If the {2}{R}{R} cost was paid, an opponent discards any number of cards, then draws that many cards. | `Modification{if_intervening_tail/parsed_effect_residual/rather_than_pay/t}` |
| Ingenious Mastery | If the {2}{U} cost was paid, you draw three cards, then an opponent creates two Treasure tokens and they scry 2. | `Modification{if_intervening_tail/rather_than_pay}` |
| Latchkey Faerie | When this creature enters, if its prowl cost was paid, draw a card. | `Condition{kind:if}` |
| Monastery Raid | If this spell's freerunning cost was paid, exile the top X cards of your library instead. | `Modification{if_intervening_tail/impulse_play/typed_spell_effect}` |
| Rafter Demon | When this creature enters, if its spectacle cost was paid, each opponent discards a card. | `Condition{kind:if}` |
| Reckless Bushwhacker | When this creature enters, if its surge cost was paid, other creatures you control get +1/+0 and gain haste until end of turn. | `Condition{kind:if}` |
| Sandman's Quicksand | If this spell's mayhem cost was paid, creatures your opponents control get -2/-2 until end of turn instead. | `Modification{anthem/if_intervening_tail}` |
| Secret of Bloodbending | If this spell's additional cost was paid, you control that player during their next turn instead. | `Modification{custom}` |
| The Last Ronin's Technique | If this spell's sneak cost was paid, they enter tapped and attacking. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Tyrant of Valakut | When this creature enters, if its surge cost was paid, it deals 3 damage to any target. | `Condition{kind:if}` |
| Verdant Mastery | Put one of them onto the battlefield tapped under an opponent's control if the {3}{G} cost was paid. | `Modification{parsed_effect_residual/put_effect/rather_than_pay/typed_spel}` |

#### "if you cast it" (cast vs. put onto the battlefield) — 15 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| Arcane Proxy | When this creature enters, if you cast it, exile target instant or sorcery card with mana value less than or equal to this creature's power from your  | `Condition{kind:if}` |
| Cacophony Unleashed | When this enchantment enters, if you cast it, destroy all nonenchantment creatures. | `Condition{kind:if}` |
| Cresting Mosasaurus | When this creature enters, if you cast it, return each non-Dinosaur creature to its owner's hand. | `Condition{kind:if}` |
| Crystalline Entity | When this creature enters, if you cast it, destroy all nonartifact creatures. | `Condition{kind:if}` |
| Iridescent Tiger | When this creature enters, if you cast it, add {W}{U}{B}{R}{G}. | `Condition{kind:if}` |
| King Solomon's Frogs | When King Solomon's Frogs enters, if you cast it, for each opponent, exile up to one target permanent that player controls with mana value 3 or greate | `Condition{kind:for_each/if}` |
| Lutri, the Spellchaser | When Lutri enters, if you cast it, copy target instant or sorcery spell you control. | `Condition{kind:if}` |
| Marina Vendrell's Grimoire | When Marina Vendrell's Grimoire enters, if you cast it, draw five cards. | `Condition{kind:if}` |
| Nine-Lives Familiar | This creature enters with eight revival counters on it if you cast it. | `Modification{custom}` |
| Null Summoner | When this creature enters, if you cast it, target opponent reveals their hand. | `Condition{kind:if/threshold}` |
| Shard of the Nightbringer | Drain Life — When this creature enters, if you cast it, target opponent loses half their life, rounded up. | `Modification{parsed_tail/typed_spell_effect}` |
| Sigardian Savior | When this creature enters, if you cast it, return up to two target creature cards with mana value 2 or less from your graveyard to the battlefield. | `Condition{kind:if}` |
| Territory Forge | When this artifact enters, if you cast it, exile target artifact or land. | `Condition{kind:if}` |
| Woodcaller Automaton | When this creature enters, if you cast it, untap target land you control. | `Condition{kind:if}` |
| Zacama, Primal Calamity | When Zacama enters, if you cast it, untap all lands you control. | `Condition{kind:if}` |

#### cast using Teamwork — 15 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| Atlantis Attacks | If this spell was cast using teamwork, choose both instead. | `Modification{if_intervening_tail/modal_header_orphan/parsed_effect_residu}` |
| Beast Mode | Also put a +1/+1 counter on that creature if this spell was cast using teamwork. | `Modification{typed_spell_effect}` |
| Crossover Collaboration | If this spell was cast using teamwork, create a Treasure token. | `Modification{if_intervening_tail/typed_spell_effect/until_next_turn}` |
| Cruel Alliance | If this spell was cast using teamwork, instead exile target creature and you gain 3 life. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Earth's Mightiest Heroes | If this spell was cast using teamwork, put any number of creature cards from among them onto the battlefield instead. | `Modification{if_intervening_tail/parsed_effect_residual/typed_spell_effec}` |
| Go Nuts! | If this spell was cast using teamwork, choose both instead. | `Modification{if_intervening_tail/modal_header_orphan/typed_spell_effect}` |
| HULK SMASH! | If this spell was cast using teamwork, choose both instead. | `Modification{if_intervening_tail/modal_header_orphan/typed_spell_effect}` |
| Helicarrier Strike | If this spell was cast using teamwork, it deals 4 damage to that creature instead. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Heroic Teamwork | If this spell was cast using teamwork, draw a card. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Murdock's Crusade | If this spell was cast using teamwork, choose both instead. | `Modification{if_intervening_tail/modal_header_orphan/parsed_effect_residu}` |
| Repulsor Blast | If this spell was cast using teamwork, it also deals 2 damage to that creature's controller. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Team Tactics | If this spell was cast using teamwork, that creature also gains trample until end of turn. | `Modification{if_intervening_tail/typed_spell_effect}` |
| Too Evil to Stay Dead | If this spell was cast using teamwork, instead choose target creature card in your graveyard. | `Modification{if_intervening_tail/parsed_effect_residual/typed_spell_effec}` |
| We Say Thee Nay! | Counter that spell unless its controller pays {4} instead if this spell was cast using teamwork. | `Modification{parsed_effect_residual/typed_spell_effect}` |
| Widow's Bite | If this spell was cast using teamwork, choose both instead. | `Modification{if_intervening_tail/modal_header_orphan/typed_spell_effect}` |

#### bargained — 5 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| Agatha's Champion | When this creature enters, if it was bargained, it fights up to one target creature you don't control. | `Condition{kind:if}` |
| Beseech the Mirror | If this spell was bargained, you may cast the exiled card without paying its mana cost if that spell's mana value is 4 or less. | `Modification{custom}` |
| Diminisher Witch | When this creature enters, if it was bargained, create a Cursed Role token attached to target creature an opponent controls. | `Condition{kind:if}` |
| Realm-Scorcher Hellkite | When this creature enters, if it was bargained, add four mana in any combination of colors. | `Condition{kind:if}` |
| Thunderous Debut | If this spell was bargained, put the revealed cards onto the battlefield. | `Modification{custom}` |

#### first spell this game — 1 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| Codespell Cleric | When this creature enters, if it was the second spell you cast this turn, put a +1/+1 counter on target creature. | `Condition{kind:if}` |

#### embalmed — 1 cards

| card | oracle sentence | what the AST carries instead |
|---|---|---|
| Vizier of Many Faces | You may have this creature enter as a copy of any creature on the battlefield, except if this creature was embalmed, the token has no mana cost, it's  | `Modification{etb_may_copy}` |

### 7b. Genuine misses — clause is about ANOTHER spell (16)

These ask about a *different* spell's cast mode, so `cast_mode_check` (which reads the card's own `StackItem`) is not the right node for them either. Listed for completeness.

| card | oracle sentence |
|---|---|
| Alex Wilder, Runaway | Whenever Alex Wilder or another creature you control enters, if you cast it from anywhere other than your hand, it gets +2/+0 and gains haste until en |
| Blazing Bomb | Whenever you cast a noncreature spell, if at least four mana was spent to cast it, put a +1/+1 counter on this creature. |
| Bloodstone Goblin | Whenever you cast a spell, if that spell was kicked, this creature gets +1/+1 and gains menace until end of turn. |
| Boromir, Warden of the Tower | Whenever an opponent casts a spell, if no mana was spent to cast it, counter that spell. |
| Hallar, the Firefletcher | Whenever you cast a spell, if that spell was kicked, put a +1/+1 counter on Hallar, then Hallar deals damage equal to the number of +1/+1 counters on  |
| Mirror of Life Trapping | Whenever a creature enters, if it was cast, exile it, then return all other permanent cards exiled with this artifact to the battlefield under their o |
| Prompto Argentum | Selfie Shot — Whenever you cast a noncreature spell, if at least four mana was spent to cast it, create a Treasure token. |
| Raggadragga, Goreguts Boss | Whenever you cast a spell, if at least seven mana was spent to cast it, untap target creature. |
| Roiling Vortex | Whenever a player casts a spell, if no mana was spent to cast that spell, this enchantment deals 5 damage to that player. |
| Sahagin | Whenever you cast a noncreature spell, if at least four mana was spent to cast it, put a +1/+1 counter on this creature and it can't be blocked this t |
| The Emperor of Palamecia // The Lord Master of Hell | Whenever you cast a noncreature spell, if at least four mana was spent to cast it, put a +1/+1 counter on The Emperor of Palamecia. |
| The Prima Vista | Whenever you cast a noncreature spell, if at least four mana was spent to cast it, The Prima Vista becomes an artifact creature until end of turn. |
| The Sibsig Ceremony | Whenever a creature you control enters, if you cast it, destroy that creature, then create a 2/2 black Zombie Druid creature token. |
| Ultros, Obnoxious Octopus | Whenever you cast a noncreature spell, if at least four mana was spent to cast it, tap target creature an opponent controls and put a stun counter on  |
| Vexing Bauble | Whenever a player casts a spell, if no mana was spent to cast it, counter that spell. |
| Void Mirror | Whenever a player casts a spell, if no colored mana was spent to cast it, counter that spell. |

---

## 8. Why my own number may be shakier than it looks

1. **The net is phrase-matched, so recall is not provable.** I built it by
   enumerating real corpus tails rather than guessing, and it is a strict superset
   of the 152 (all 152 fall inside the 434 — checked). But a wording that uses
   none of my anchors is invisible to me exactly as it is to the exporter. The 197
   is a **floor**, not a ceiling.
2. **16 of the 18 `per_card` acquittals are registration-only.** I confirmed the
   handler exists and read two in full. A registered handler that ignores the gate
   would be a miss I scored as a pass.
3. **The 21 runtime rescues are gate-only.** I replayed the Go gate regexes
   faithfully, but did not verify that `parseEffectText` produces a working body
   for any of them. Some are probably half-dead.
4. **B's 46 are verified by code read, not by running a game.** `case "kicked"`
   reads `src.Flags["kicked"]`; I did not confirm the flag is live on every path
   that can put those permanents onto the battlefield.
5. **The self/other split (181/16) is heuristic**, done with a regex on the
   sentence. Two cards (Beseech the Mirror, Transpose) were manually moved after
   mis-bucketing. Treat the 197 total as the solid number and the split as
   indicative.
6. **Double-faced cards count as one row**, and A-side Alchemy rebalances
   (`A-Sprouting Goblin`) count separately from their originals, so a handful of
   the 197 are near-duplicates of each other.

## 9. Reproduction

Probe scripts are in this session's scratchpad, not committed. The two that matter:
the calibration walks `ast` for `__ast_type__ ∈ {Modification, ModificationEffect}`
with `kind == "cast_mode_check"`; the scan regex-matches `oracle_text` with
parenthesised reminder text stripped, splits on sentence boundaries, then re-walks
the AST for both `Modification.kind` and `Condition.kind`. Keying on `node`
instead of `__ast_type__` returns zero.

---

## Independent verification and the engine-side half (Hex, 2026-10-18)

### The central claim, re-checked

The dominant failure above is not a missing node but the engine
**failing closed** on a node the exporter did emit. Verified directly:

```
$ awk '/^func evalCondition/,/^}/' internal/gameengine/resolve.go | grep -n 'case "if"\|case "kicked"'
60:  case "kicked":          <- present
                             <- there is NO case "if"
```

and the function's tail (resolve.go:618-633):

```go
// Fail closed: an unrecognized condition kind must not silently
// satisfy its gate. Log the kind so the unhandled set is a
// measurable burn-down list instead of invisible wrongness.
... gs.LogEvent(Event{Kind: "condition_unrecognized", ...})
return false
```

So a card exporting `Condition{kind:"if", args:["it was kicked"]}`
evaluates **false unconditionally** — it does nothing whether or not it
was kicked. Confirmed.

### The burn-down list was never readable

The comment promises the unhandled set is "a measurable burn-down list
instead of invisible wrongness." It is not, and that is a second finding:

```
$ grep -c 'condition_unrecognized' server.log      # whole log, not marker-scoped
0
```

`LogEvent` writes into `gs.EventLog`, a per-game in-memory structure, and
never to `server.log`. Under `EventLogNone` it retains nothing at all;
under any policy the grinder discards it with the game. **Nobody has ever
been able to read the list the comment describes.** Same class as the
r144 swallowed keywords: a diagnostic that exists, is correct, and
produces no reachable output. Making it emit a per-segment census would
turn this whole document into a live counter.

### The engine-side coverage number (`cast_mode_coverage_r175_test.go`)

The corpus scan above answers "did the exporter emit a node". The
complementary question is "given the node, can the engine evaluate it".
Driving the real `castModeSatisfied` over every mode in the census, in
both directions:

```
cast-mode coverage: 7/11 modes decidable, 130/152 CARDS covered (85.5%)
decidable: bargained from_exile from_graveyard from_hand kicked
           madness_cost_paid surge_cost_paid
UNDECIDABLE — condition assumed TRUE, card gets its bonus free:
  five_or_more_mana_spent (9 cards)   additional_cost_paid (8)
  foretold (3)                        prowl_cost_paid (2)
```

A mode counts as covered only if the engine answers BOTH directions AND
the answers differ — a mode that always returns the same thing is not
evaluating anything. The test is a RATCHET: it fails if coverage
regresses below 4 undecidable modes, and also fails if it improves,
telling you to lower the baseline so the gain is held.

### Transport, tested separately from the destination

A decidable mode is worthless if the handler never receives a StackItem.
`currentResolvingSpell` is the only route and it rejects several shapes:

```
spell mid-resolution:   allow=true  decided=true    <- works
permanent ETB trigger:  decided=false               <- assumed TRUE
nothing resolving:      decided=false               <- correct
```

For a permanent's ETB the spell has already left the stack, so the
resolving item is the trigger (which carries a Source) and
`currentResolvingSpell` returns nil. Kicker specifically survives this
because the separate `Condition{kind:"kicked"}` path reads
`Flags["kicked"]` off the permanent. **The exposed gap is any
ETB-triggered `cast_mode_check` whose mode has no permanent-flag
equivalent.**

**A note against my own test:** its first cut built the handler argument
as a `map[string]interface{}` that looked like a Modification. `modShape`
accepts only `*gameast.Modification` / `*gameast.ModificationEffect`, so
every call came back undecided and the test "proved" the entire path was
inert. It was proving its own bug. Fixed before reporting, and the
comment in the helper now says so.

### Status

**Filed, not fixed.** Closing any of this changes which cards do what,
which moves game outcomes and breaks ELO comparability. The three
separate decisions are: (1) add `case "if"` with a raw-text normaliser,
(2) track the four unevaluable modes on the StackItem, (3) reach the
cast-mode state from an ETB trigger. (1) is the largest by card count and
the only one that is currently *wrong* rather than *over-generous*.
