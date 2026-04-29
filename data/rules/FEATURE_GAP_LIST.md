# Engine Feature Gap List — compiled from 20 deck requirements

## ⚠️ PRIOR GAUNTLET DATA INVALIDATED (2026-04-16)

Project owner declared all pre-2026-04-17 gauntlet winrate data **invalid as gameplay signal**. The engine was too broken in too many orthogonal dimensions (inert mana rocks, unenforced additional costs, stubbed per-card effects, missing storm body-effects, missing partner commander wiring, missing Day/Night, etc.) for any deck-strength comparison to be meaningful. Symptom that should have raised the alarm earlier: games ending with all 4 commanders still on the battlefield — indicating removal, commander damage, and board wipes weren't resolving correctly.

**What REMAINS valid**: throughput benchmarks (232.8 g/s Phase 11, 1083 g/s post-Phase 12) — those measure turn-processing speed, orthogonal to correctness.

**What's INVALID**: every deck winrate cited prior to 2026-04-17 (Tergrid 1% / Moraug 40% / Ral Stormoff 7% / Kraum+Tymna 0% / etc.). Do not use these as baselines or for pre/post comparison. Post-wave-#2+wave-#3 winrates are the first real data point.

**Going forward**: correctness checkpoint before any new gauntlet cycle. See "Correctness Gate" section at bottom of this doc.

---



**Purpose**: Comprehensive inventory of engine capabilities required by the 20-deck test portfolio but not currently implemented. Drives the post-Phase-12 feature-patching wave.

**Status**: Compiled 2026-04-16. Feeds the nightmare list exercise.

**Authoritative ordering**: Tier 1 = blocks multiple decks OR is load-bearing for cEDH validation. Tier 2 = blocks one deck's core strategy. Tier 3 = nice-to-have / partial coverage acceptable.

---

## TIER 1 — BLOCKS MULTIPLE B4/B5 DECKS

### 1. Storm + Cast-Count Infrastructure (CRITICAL)
**Status**: Not implemented in either engine.
**Blocks**: Narset (Storm Prowess), Kraum+Tymna (cEDH Combo), Ral Stormoff (cEDH Stormoff), partial Varina/Voja/Maja.
**Scope**:
- `GameState.SpellsCastThisTurn int` (global, resets each turn_start)
- `Seat.SpellsCastThisTurn int` + `Seat.SpellsCastLastTurn int` (per-seat; resets at seat's turn_start)
- Increment on every cast
- CR §702.40 storm keyword — when storm spell casts, copy self for (Game.SpellsCastThisTurn − 1) times
- Cards affected: Brain Freeze, Grapeshot, Tendrils of Agony, Mind's Desire, Haze of Rage, Inner Fire
- Cast-trigger observers: Storm-Kiln Artist, Runaway Steam-Kin, Birgi (mana gen per cast), Monastery Mentor, Third Path Iconoclast, Adeline, Young Pyromancer, Niv-Mizzet Parun
**Estimated scope**: 7-12 hr (Python + Go)

### 2. Cast-From-Graveyard Mechanic (CRITICAL)
**Status**: Not implemented.
**Blocks**: Kraum+Tymna (Underworld Breach combo), Ral (Breach/Past in Flames/Mizzix's Mastery), Moraug (Underworld Breach), Ashling (Chainer Nightmare Adept).
**Scope**:
- `cast_from_zone(seat, card, zone)` helper — zone != hand
- CR §702.133 Flashback keyword
- CR §702.150 Encore keyword
- Underworld Breach granting flashback to instants/sorceries
- Chainer, Nightmare Adept granting "may cast from graveyard"
- Mizzix's Mastery overload mass-flashback
- Past in Flames flashback grant
- Dauthi Voidwalker cast-from-exile
**Estimated scope**: 6-8 hr

### 3. Day/Night + DFC Transform (CRITICAL for werewolf deck + some lands)
**Status**: Partially implemented (face-down shipped today, DFC AST support partial, transform+day/night not wired).
**Blocks**: Werewolf deck (core strategy), minor impact on MDFC lands in multiple decks (Sin, Tergrid, Ashling).
**Scope**:
- CR §726 Day/Night mechanic — game-state flag
- Daybound/Nightbound keywords
- DFC transform (§712) — flip between two faces, layer-1 override similar to face-down
- Cast-count condition hooks (night→day at ≥2 casts this turn; day→night at 0 casts)
- Transform triggered abilities fire on transition
- Cards affected: Ulrich, Tovolar, Arlinn Kord, Mayor of Avabruck, Village Messenger, Reckless Waif, Scorned Villager, Wolfbitten Captive, Geier Reach Bandit, Hermit of the Natterknolls
**Estimated scope**: 8-12 hr (Python + Go; reuses some face-down infra)

### 4. Per-Card Snowflake Handler Port (Go critical)
**Status**: Python has 1,079 per-card handlers. Go has 20 runtime dispatchers.
**Blocks**: ~300-500 cards in the 20-deck portfolio (snowflakes don't generalize from AST — need card-specific logic).
**Scope**: Port Python per_card.py runtime handlers to Go. MVP subset: ~50 highest-impact cards from the 20-deck portfolio:
- Doomsday (pile construction)
- Cloudstone Curio
- Underworld Breach
- Bazaar of Baghdad
- Final Fortune
- Karn, the Great Creator
- Grim Monolith (untap cost)
- Mana Vault (upkeep damage)
- Notion Thief
- Opposition Agent
- Thassa's Oracle (already partial)
- Tergrid (parser gap — needs parser fix too)
- Aetherflux Reservoir
- Felidar Sovereign
- Test of Endurance
- Approach of the Second Sun
- Sundial of the Infinite
- Teferi's Protection (needs phasing)
- Timetwister / Wheel of Fortune / Windfall (mass wheel)
- Echo of Eons
- Past in Flames
- Mizzix's Mastery
- Bonus Round
- Flare of Duplication
- Reiterate
- Displacer Kitten
- Storm-Kiln Artist
- Runaway Steam-Kin
- Birgi, God of Storytelling
- Hellkite Tyrant
- Goldspan Dragon
- Ragavan, Nimble Pilferer
- Jeska, Thrice Reborn
- Magda, Brazen Outlaw
- Niv-Mizzet, Parun
- Polymorph / Chaotic Transformation / Synthetic Destiny
- Progenitor Mimic
- Seedborn Muse
- Yarok, the Desecrated (ETB doubler — §614 handles, verify)
- Worldgorger Dragon (handler exists)
- Food Chain (handler exists)
- Sneak Attack (delayed trigger)
**Estimated scope**: 15-25 hr across multiple agents (parallelizable by handler batch)

### 5. Partner Commanders (§903.3c)
**Status**: Python commander format supports single commander. Partner support architected in multiplayer agent but not fully wired.
**Blocks**: Kraum+Tymna (cEDH Combo B5 — requires partner).
**Scope**:
- `Seat.CommanderNames: list[str]` (already exists)
- Both commanders in command zone at game start
- Each has independent commander tax + commander damage tracking
- Partner-specific rules (must have partner keyword, cardinality 2)
**Estimated scope**: 2-4 hr

---

## TIER 2 — BLOCKS ONE DECK'S CORE STRATEGY

### 6. Banding Keyword (legacy)
**Status**: Not implemented, parser probably emits UnknownEffect.
**Blocks**: Soraya the Falconer deck.
**Scope**:
- CR §702.21 Banding keyword
- Block assignment modification (attacker with banding chooses damage assignment)
- 25× Tempest Hawk + banding tribal = evasion + custom block damage
**Estimated scope**: 3-5 hr

### 7. Phasing (CR §702.26)
**Status**: Not implemented.
**Blocks**: Teferi's Protection (appears in 3+ decks), some exile-then-return effects.
**Scope**:
- `Permanent.Phased bool` flag
- Phased permanents are treated as if they don't exist (can't be targeted, don't trigger)
- Auto-phase-in at controller's next untap step
- Teferi's Protection = phase out all permanents you control + you gain protection from everything + you have shroud
**Estimated scope**: 4-6 hr

### 8. Goad (CR §702.128)
**Status**: Not implemented.
**Blocks**: Riku (Mob Rule, Disrupt Decorum), multiplayer politics effects.
**Scope**:
- `Permanent.Goaded map[int]bool` (goaded to attack which seats?)
- Goaded creature must attack each combat if able
- Can't attack its goader
- Multiple goads possible
**Estimated scope**: 3-5 hr

### 9. Tergrid Trigger Payoff (parser gap)
**Status**: Parser emits UnknownEffect on "put that card onto battlefield under your control".
**Blocks**: Tergrid deck (1.1% winrate at 1000-game gauntlet).
**Scope**: Python parser extension to recognize the payoff pattern.
**Estimated scope**: 1-2 hr

### 10. Doomsday Pile Construction
**Status**: Not implemented.
**Blocks**: Kraum+Tymna uses Doomsday as primary tutor-win line.
**Scope**:
- Per-card handler: exile all but 5 cards, pay 5 life, stack 5 in order
- Specific piles depend on deck (Lab Maniac + Brainstorm + LED + ... is canonical)
**Estimated scope**: 2-3 hr

### 11. Cloudstone Curio Flicker Pattern
**Status**: Not implemented.
**Blocks**: cEDH decks (Kraum+Tymna), flicker-enchantment combos.
**Scope**:
- Triggered when nonland permanent enters under your control
- "May return another nonland permanent you control to its owner's hand"
- Combo with Displacer Kitten + cheap permanents = infinite flicker
**Estimated scope**: 1-2 hr

### 12. Karn, the Great Creator + Sideboard Access
**Status**: Not implemented.
**Blocks**: Kraum+Tymna stax piece, Commander-format wishing.
**Scope**:
- Static: "activated abilities of artifacts your opponents control can't be activated"
- −2: exile target nontoken artifact, then you may Wish (reveal a card outside the game)
- Requires sideboard/wishboard concept (Python doesn't have it; Go doesn't have it)
- MVP: Wish can pull from a designated "wishboard" list (deck file metadata)
**Estimated scope**: 3-4 hr

### 13. Aetherflux Reservoir
**Status**: Partial (lifegain triggers work but "pay 50 life → 50 damage" activation needs handler).
**Blocks**: Oloro deck (primary wincon).
**Scope**: Per-card activated ability handler.
**Estimated scope**: 1 hr

### 14. Felidar Sovereign / Test of Endurance / Approach of the Second Sun
**Status**: Not implemented (alt-wincon triggers).
**Blocks**: Oloro deck (win conditions).
**Scope**:
- Felidar Sovereign: "at beginning of your upkeep, if you have 40+ life, win the game"
- Test of Endurance: "50+ life" variant
- Approach of the Second Sun: "cast from library, 7 turns later, if cast again → win"
**Estimated scope**: 2-3 hr combined

---

## TIER 3 — NICE-TO-HAVE / PARTIAL COVERAGE

### 15. §613.8 Dependency Rule
**Status**: Skipped (matches Python). Pure timestamp ordering only.
**Blocks**: Conspiracy-class layer interactions (rare in practice).
**Scope**: §613.8 dependency detection and ordering within a layer.
**Estimated scope**: 6-8 hr — defer indefinitely.

### 16. Modal Spell Choice Selection
**Status**: Greedy (always picks index 0).
**Blocks**: Riku deck modality (15+ modal cards picking suboptimally).
**Scope**: Hat method picks best mode based on game state.
**Estimated scope**: 2-3 hr in PokerHat.

### 17. Suspend Keyword (CR §702.62)
**Status**: Not implemented.
**Blocks**: Jhoira of the Ghitu (legacy, 1 card in Obeka deck).
**Scope**: Time counters, upkeep removal, cast at zero counters.
**Estimated scope**: 3-4 hr.

### 18. Encore Keyword (CR §702.145)
**Status**: Not implemented.
**Blocks**: Some Coram-adjacent cards.
**Scope**: Token copy creation + sacrifice at end of turn.
**Estimated scope**: 2-3 hr.

### 19. Mandatory Draw+Discard (Bazaar of Baghdad)
**Status**: Needs verification.
**Blocks**: Madness engines, dredge, Kraum+Tymna graveyard shell.
**Scope**: Single-card handler verification.
**Estimated scope**: 30 min verification.

### 20. Compound Untap Cost (Grim Monolith)
**Status**: Needs verification.
**Scope**: Untap ability with mana cost.
**Estimated scope**: 30 min verification.

### 21. Upkeep-Damage on Mana Rocks (Mana Vault, Braid of Fire)
**Status**: Needs verification.
**Blocks**: Proper resolution of these mana rocks.
**Scope**: Upkeep-triggered abilities + damage self.
**Estimated scope**: 1 hr verification.

---

## B5 cEDH ADDITIONS (Lumra Good Luck + Azula Combat Cast Combo)

Features surfaced by cEDH decks #3-4 loaded after initial gap compile.

### Lumra mono-green lands-combo (B5)
- **Battle card type (§311)** — Invasion of Ikoria. Completely new permanent subtype with defense counters + transform-on-defeat. Affects zero decks currently except this one. Scope: 4-6 hr.
- **Cost floor modifier** — Trinisphere "spells cost at least 3". Hooks into cost calc layer before alt-cost application. Scope: 1-2 hr.
- **Activated-ability permission deny (global static)** — Null Rod, Collector Ouphe: "activated abilities of artifacts can't be activated unless mana ability". Needs ability-type introspection at activation time. Scope: 2 hr.
- **Uncounterable creature spells (layer-6 grant)** — Allosaurus Shepherd: "creature spells you control can't be countered". Static grant to cast attempts. Scope: 1-2 hr.
- **Creatures-are-lands type change (layer 4)** — Ashaya, Soul of the Wild. Reverse direction of typical creature-land. Scope: 1-2 hr.
- **Per-permanent upkeep tax** — The Tabernacle at Pendrell Vale: "all creatures have 'upkeep: sacrifice this unless you pay 1'". Mass granted triggered ability. Scope: 2 hr.
- **Creature-count scaling mana** — Gaea's Cradle: "tap: add G per creature you control". ScalingAmount tied to zone count. Scope: 1 hr.
- **Distinct-land-count trigger** — Field of the Dead: landfall conditional on 7+ different-named lands creating 2/2 Zombie. Needs distinct-name counter. Scope: 1-2 hr.
- **Extra land drop effects** — Exploration, Burgeoning. Raise per-turn land drop cap. Scope: 1 hr.
- **Cast-from-exile grants (landfall-adjacent)** — Nissa, Resurgent Animist: landfall-triggered "may cast creature/land from exile". Scope: 2 hr.
- **Mass-return-lands-from-graveyard** — Aftermath Analyst, Famished Worldsire. Pattern: sac + return all lands from gy tapped. Scope: 1-2 hr.
- **Alternate-cost tap-creature-for-mana** — Earthcraft: "tap untapped creature: untap target basic". Uses creature tap as untap-land cost. Scope: 2 hr.
- **Counter-hate "all spells on stack can't be countered"** — Vexing Bauble variant. Scope: 1 hr.

### Azula Combat Cast Combo (B5) — additions on top of Kraum+Tymna shared gaps
- **Split Second (§702.61)** — Trickbind. No spells or abilities can be cast while on stack. Scope: 2 hr.
- **Play-from-opponent's-library** — Praetor's Grasp. Zone-traversal + alternate-castable grant. Scope: 2 hr.
- **Give-permanent-to-opponent** — Wishclaw Talisman, Donate pattern. Control change via activation. Scope: 1-2 hr.
- **Dualcaster Mage + copy-creature-spell lines** — trigger-on-stack + copy target spell. Partial coverage via Storm-Kiln Artist infra. Scope: 1-2 hr.
- **Alternate-cast reveal (Valley Floodcaller style)** — storm-adjacent cast counter triggers. Covered by storm infra (Tier 1 #1).
- **Scry as bonus rider** (pervasive) — already partial, needs sweep.

**Total added scope**: ~25-35 hr across both decks' unique features. Mostly small handlers, one big item (Battle card subtype) that's deck-specific.

### Ardenn+Rograkh Big Stick equipment voltron (B5, cEDH #5)

Equipment subsystem — largely missing from Go engine, partial in Python. This deck forces the equipment family as a core feature.

- **Equip ability (§702.6)** — sorcery-speed cost to attach equipment to creature you control. Core primitive. Scope: 2-3 hr.
- **Free equip cost modifier** — Puresteel Paladin "equip abilities cost {0}". Hooks into equip-cost calc. Scope: 1 hr.
- **Attach-on-ETB commander** — Ardenn: "at beginning of combat, you may attach any number of Auras and Equipment you control to target creatures/permanents." Batch reattach. Scope: 2 hr.
- **Attack-trigger token-copy of equipment** — Bloodforged Battle-Axe: "whenever equipped creature attacks, create a token that's a copy of this artifact." Equipment-specific attack trigger + token clone. Scope: 2 hr.
- **Activated instant-tutor from battlefield** — Sunforger: "{3}{R}{W}, unattach: search library for instant with CMC ≤ 4, cast without paying mana cost." Unique activated tutor + alt-cost cast. Scope: 2-3 hr.
- **Equipment-granted combat damage triggers** — Sword of Feast and Famine / Fire and Ice / Forge and Frontier: "when equipped creature deals combat damage to a player, DO X." Expanded §510.2 attacker-trigger wiring. Scope: 2 hr.
- **Aura-as-equipment "Mantle of the Ancients" style** — returns all aura/equipment from graveyard as ETB. Zone-transition aggregate. Scope: 1 hr.
- **Urza Tron land identification (CR §107.3)** — Urza's Mine + Power Plant + Tower give extra mana when all three are tapped together. Named-set detection. Scope: 1-2 hr.
- **Replacement-effect draw redirect (Uba Mask)** — "if a player would draw, instead exile that card face down; they may play it until EOT." Major replacement effect. Scope: 2-3 hr.
- **Counter-spell-replace-with-random (Tibalt's Trickery)** — "counter target spell; its controller exiles top 3, they may cast first non-land from exile without paying." Replacement + cascade-adjacent. Scope: 1-2 hr.
- **Steal activated abilities (Treasure Nabber)** — "whenever opponent activates mana ability of artifact, gain control of that artifact until EOT." Control-change on trigger. Scope: 2 hr.
- **Equipment with P/T-boost auras** — All That Glitters, Eldrazi Conscription, Leyline Axe. Layer-7 continuous modifications from aura; partially covered. Verify: 1 hr.
- **The Aetherspark planeswalker-artifact hybrid** — loyalty counter activated abilities on an artifact. Scope: 1-2 hr.

**Total added scope**: ~20-25 hr for the equipment family. High reuse potential — equipment infra covers 30+ cards in this deck alone, plus Digital Foundation / future tribal voltron decks.

### Y'shtola Esper Thoracle control (B5, cEDH #6)

Classic Esper Ad Nauseam + Thoracle pile — much of its feature set overlaps Kraum+Tymna, but surfaces a dense cluster of denial/replacement effects worth cataloging separately.

- **Bloodchief Ascension (quest counter class §122.1e)** — "whenever opponent loses life or mill, put quest counter; at 4 counters becomes active; when active, opponent loses 2 per draw+mill+trigger." Needs quest counter type + activation threshold + repeated replacement. Scope: 2-3 hr.
- **Hullbreaker Horror** — "whenever you cast a spell, return target nonland permanent to owner's hand." Self-ETB trigger on self + cast-trigger bounce loop with free-cost spells = infinite bounce combo. Scope: 1-2 hr (reuses cast-trigger infra from storm).
- **Drannith Magistrate** — "players can't cast spells from anywhere other than their hand." Zone-cast restriction global static. Blocks commander casts, flashback, Underworld Breach, cascade, suspend — extremely load-bearing. Scope: 2 hr.
- **Grand Abolisher** — "during your turn, opponents can't cast spells or activate abilities of artifacts/creatures." Turn-scoped opponent-action deny. Scope: 1-2 hr.
- **Orim's Chant / Silence / Ranger-Captain of Eos sac pulse** — "players can't cast spells this turn" / "opponents can't cast noncreature spells this turn". Turn-scoped spell-cast lock. Shared primitive. Scope: 2 hr.
- **Notion Thief (§614 draw replacement)** — "if opponent would draw card except first draw step, instead you draw." Replacement on opposing draws. Scope: 1-2 hr.
- **Opposition Agent (§701.19 search replacement)** — "whenever opponent searches library, you search instead and may put card anywhere." Replacement on search effects — affects tutors, fetchlands, etc. Scope: 2-3 hr.
- **Plunge into Darkness self-reveal-X** — "pay any amount of life, exile top X cards, choose any number, put them into hand, lose 2 per chosen." X-cost + chooser-is-controller pattern. Scope: 1-2 hr.
- **Smothering Tithe draw-or-discard opponent choice** — "when opponent draws, unless they pay {2}, create a Treasure." Opponent-makes-payment-choice-on-draw-trigger. Scope: 1 hr (covered by draw-trigger infra).
- **Street Wraith cycling-for-2-life** — "pay 2 life: draw a card, exile this from hand." Cycling variant. Scope: 30 min.
- **Mishra's Bauble delayed draw** — "sacrifice: look at top of any library. Draw at beginning of next upkeep." Delayed trigger + library peek. Scope: 1 hr.
- **Soul-Guide Lantern graveyard hate** — tap-exile-target-card + sac-exile-all-graveyards-of-one-player. Zone manipulation activated. Scope: 1 hr.
- **March of Otherworldly Light convoke-adjacent reveal** — "as an additional cost, reveal X white cards from hand; exile target nonland permanent with CMC X." Reveal-cost pattern. Scope: 1-2 hr.

**Total added scope**: ~18-25 hr. Most items are small, reuse existing draw/cast/search trigger infrastructure. Biggest new primitive is quest-counter class (Bloodchief Ascension opens the door for Luminarch Ascension, Bloodghast-style ascension sagas, etc.).

### Yarok ETB-doubler blink (B5, cEDH #7)

Sultai Yarok Aluren shell — wins via Thoracle but combos through chained ETB triggers (Yarok doubles every ETB, Aluren makes small creatures free, Acererak self-returns for infinite mill loops, Displacer Kitten blinks for free on every noncreature cast).

- **Yarok, the Desecrated (ETB doubler §614 modification)** — "if a permanent entering the battlefield causes a triggered ability of a permanent you control to trigger, that ability triggers an additional time." Modifies triggered ability resolution count. Must stack with Panharmonicon/Parallel Lives/Doubling Season. Per-card handler ported? Verify. Scope: 1-2 hr verification or 3 hr fresh.
- **Aluren (global alt-cast grant)** — "any player may cast creature spells with CMC ≤ 3 without paying their mana cost, any time they could cast an instant." Alternate-cost + timing-override + CMC-gate + global grant. Major primitive — reusable for Omen Machine, Possibility Storm, Mind's Dilation, etc. Scope: 3-4 hr.
- **Acererak self-return ETB loop** — "when this ETBs, venture / mill target; when this leaves battlefield, return to hand." Self-bounce loop with Yarok = infinite ETB mill. ETB+LTB same card. Scope: 1-2 hr.
- **Flesh Duplicate** — "enters as a copy of another creature you control except CMC 2 higher; if you do, scry 2" — conditional clone + rider. Scope: 1 hr.
- **Sylvan Library (draw-replacement + multi-choice cost)** — "at beginning of draw step, may draw 2 additional cards; if you drew them this way, return 2 to top of library or pay 4 life for each you don't return." Draw replacement + multi-unit per-card payment choice. Scope: 2 hr.
- **Destiny Spinner (static uncounterable)** — "creature and enchantment spells you control can't be countered." Similar to Allosaurus Shepherd but different type scope. Reuses primitive. Scope: 30 min.
- **Carpet of Flowers (opponent-state mana)** — "at beginning of your precombat main, may add G for each Island an opponent controls." Reads opposing battlefield state for mana gen. Scope: 1 hr.
- **Culling the Weak / Culling Ritual sac-for-mana rituals** — alternate-cost ritual with sacrifice component. Scope: 1 hr (covered by broad alt-cost work).
- **Deathrite Shaman three-mode graveyard activator** — tap ability with 3 modes, each reading any graveyard. Zone-read-across-seats + modal activated. Scope: 1 hr.
- **Wild Growth (enchant land mana boost)** — aura on basic land adding mana when tapped. Aura-granted triggered mana ability. Scope: 1 hr.
- **Horizon/Cycling lands (Nurturing Peatland, Waterlogged Grove)** — sac-for-life + draw. Single-use lands with sac-activated. Scope: 30 min.
- **Spellseeker ETB tutor** — CMC ≤ 2 instant/sorcery tutor on ETB, chained with Yarok-doubling = two tutors per ETB. Scope: 1 hr.
- **Eldritch Evolution / Natural Order evolution tutors** — sacrifice creature, tutor creature with CMC ≤ X+2. Shared primitive with Lumra. Scope: covered.

**Critical stress test**: Yarok decks break the naive trigger engine — Yarok-doubled ETBs feed back into themselves (Yarok ETBs → Yarok triggers off itself 2x due to self-application? No, Yarok doesn't trigger off its own ETB, but Acererak+Yarok+blink = 4 ETBs per blink, Aluren-free cast chains produce dozens of ETBs per turn). Engine needs **chain-depth tracking + trigger batch resolution** to not blow the stack in pathological cases.

**Total added scope**: ~15-20 hr. Big wins: Aluren primitive unlocks several archetype cards, Sylvan Library is a long-standing omission, Yarok-doubling verification is load-bearing for blink decks generally.

### Kinnan Basalt Monolith turbo (B5, cEDH #8)

Simic Kinnan+Thrasios — the canonical untap-loop combo deck. Wins via infinite mana (Kinnan + Basalt/Grim Monolith = infinite colorless, filter via Bloom Tender or lands; Freed from the Real / Pemmin's Aura on a mana creature = infinite colored) drained through Walking Ballista / Thrasios draw-into-Thoracle / Tidespout Tyrant bounce lock.

- **Kinnan mana-generation replacement (§605 augmentation)** — "whenever you tap a nonland permanent for mana, add one mana of any type that permanent produced." Modifies the mana added by a mana ability before it enters the pool. Doesn't use the stack. Critical primitive. Scope: 2-3 hr.
- **Kinnan activated (top-of-library creature reveal-to-battlefield)** — "{2}{G}{U}: reveal top card; if creature, put onto battlefield, else put on bottom." Activation with library-peek + conditional ETB. Scope: 1-2 hr.
- **Untap-self aura (Freed from the Real / Pemmin's Aura)** — aura grants "{U}: untap enchanted creature" activated ability. Combined with mana-ability creature = infinite untap loop. Aura-granted activated ability primitive. Scope: 1-2 hr.
- **Thousand-Year Elixir / Concordant Crossroads (global haste grant)** — "creatures you control gain haste" static. Scope: 1 hr (small). Thousand-Year Elixir also has "{1}, {T}: untap target creature" — separate activated ability on artifact.
- **Enduring Vitality (broad creature-mana grant)** — "creatures you control can tap for mana as though they had an ability that said 'tap: add one mana of any color.'" Grants mana ability to every creature. Broader than Earthcraft. Scope: 2 hr.
- **Bloom Tender color-count mana** — "tap: add one mana of each color of a permanent you control." Scales with color coverage of board — similar to Gaea's Cradle scaling pattern but per-color. Scope: 1 hr (covered by scaling mana infra).
- **Candelabra of Tawnos X-mass-untap** — "X, tap: untap X target lands." X-cost activated with target count. Scope: 1 hr.
- **Tidespout Tyrant cast-trigger bounce** — "whenever you cast a spell, may bounce target nonland permanent." Reuses Hullbreaker Horror primitive. Scope: covered.
- **Animist's Awakening X-cost land reveal** — "reveal top X cards; put each land card revealed onto battlefield tapped, rest to bottom in random order." Library-peek-filter-play-onto-battlefield pattern. Scope: 1-2 hr.
- **Sacrificial lands (Saprazzan Skerry, Hickory Woodlot)** — "enters tapped, tap: add one mana, sacrifice after 3 uses" (counter-tracked single-use). Counter-per-use mana land. Scope: 1 hr.
- **Transmute Artifact** — "{U}, sacrifice an artifact: search for artifact with CMC ≤ sacrificed's CMC." CMC-comparative tutor. Scope: 1 hr.
- **Tezzeret the Seeker planeswalker (-X tutor)** — loyalty activated tutor. Uses planeswalker primitive (verify). Scope: 1 hr verification.
- **Survival of the Fittest** — "{G}, discard creature: search for creature card." Cost-with-discard + tutor. Scope: 1 hr.
- **Scroll Rack (hand-deck-swap)** — "{1}, tap: exile any number from hand face-down; put top X of library into hand; put exiled cards on top in any order." Complex hand manipulation. Scope: 2 hr.
- **Sensei's Divining Top (deck-peek-shuffle)** — three-mode activated. Already covered by other decks' SDT usage? Verify: 30 min.
- **Drift of Phantasms / Trophy Mage / Spellseeker ETB transmute/tutor variants** — card-type-constrained tutors. Scope: 1 hr shared.

**Critical combo chains to exercise**: Kinnan + Basalt Monolith (tap {3} → +1 from Kinnan = 4 mana; untap for 3; net +1 per cycle = infinite). Kinnan + Thousand-Year Elixir on Bloom Tender = infinite all-color mana. Freed from the Real on any tap-for-mana creature = infinite. Walking Ballista with infinite mana = lethal. These are "does the engine correctly detect and resolve loop conditions" tests — if the engine doesn't cap loop depth or detect fixed-points, it hangs.

**Total added scope**: ~15-22 hr. Kinnan's mana augmentation is the single most important primitive here — many "X matters" cards (Gyruda, Morophon, Phyrexian Altar, Mana Reflection, Doubling Cube) follow similar patterns.

**cEDH portfolio COMPLETE**: 8/8 target decks loaded.

| # | Commander(s) | Archetype | Primary wincon | Engine stress vector |
|---|--------------|-----------|----------------|---------------------|
| 1 | Kraum + Tymna | Doomsday/Breach control-combo | Thoracle or Doomsday pile | cast-from-graveyard (Breach), partner tax, §614 depth |
| 2 | Ral, Monsoon Mage | Pure storm | Grapeshot/Brain Freeze/Tendrils | storm keyword, cast-count scaling, ritual chains |
| 3 | Fire Lord Azula | Combat-cast-copy combo | Dualcaster Mage + copy loop | split-second (Trickbind), give-to-opponent tutors |
| 4 | Lumra | Mono-G lands combo | Food Chain creature, Field of the Dead | Battle subtype, cost-floor mod, landfall-cast-exile |
| 5 | Ardenn + Rograkh | Equipment voltron | Colossus Hammer one-shot / Bloodforged | equipment family (equip, free-equip, attach-ETB batch) |
| 6 | Y'shtola | Esper Thoracle control | Thoracle + Consultation | draw/search replacements, stack locks, quest counters |
| 7 | Yarok | Sultai ETB blink | Thoracle via Aluren loop | §614 trigger-count mod, Aluren alt-cast, chain depth |
| 8 | Kinnan + Thrasios | Simic untap turbo | Walking Ballista infinite mana | §605 mana augmentation, untap-aura loops |

**Coverage analysis**: Storm, combat, equipment, lands, blink, control, turbo, combat-cast-copy. Every major cEDH engine subsystem is under test. 7 of 8 use partner or Thoracle-backdoor; 6 of 8 use Thoracle shell. Feature overlap is high on the disruption package (Orcish Bowmasters, Mental Misstep, Force of Negation) and draw-trigger detection (Rhystic Study, Mystic Remora, Esper Sentinel), which means fixing those primitives fixes them for 6+ decks at once — maximum leverage.

### BONUS: Muldrotha Hermit Druid graveyard turbo (B5, cEDH #9)

Sultai graveyard combo — Hermit Druid dumps the library into graveyard in one activation, then Muldrotha recurs the pile back. Secondary wins via Necrotic Ooze (imports all creature activated abilities from all graveyards → Phyrexian Devourer + Walking Ballista lethal) and Food Chain + Misthollow Griffin (cast-from-exile infinite mana).

- **Muldrotha graveyard-cast grant (zone-cast with per-type rate limit)** — "during each of your turns, you may play a land and cast a permanent spell from your graveyard of each permanent type." Zone-cast permission + per-turn per-permanent-type counter (1 artifact, 1 creature, 1 enchantment, 1 land, 1 planeswalker, 1 battle per turn). Major primitive. Scope: 3-4 hr.
- **Hermit Druid mill-until-nonbasic** — "{1}{G}, tap: reveal cards from top of your library until you reveal a non-basic-land; put all revealed nonland cards into graveyard, add revealed lands to your hand." If deck has zero basics → mills entire library. Combo with Thoracle or Necrotic Ooze outlet. Scope: 1-2 hr.
- **Necrotic Ooze (mass activated ability import)** — "has all activated abilities of all creature cards in all graveyards." Dynamic ability grant reading all seats' graveyards. Huge primitive — every creature's activated ability becomes available. Scope: 3-4 hr (hard, because abilities resolve as if on Ooze).
- **Chains of Mephistopheles (draw replacement)** — "if a player would draw a card except the first draw each of their draw steps, they discard a card instead; if they can't, they put the top card of their library into their graveyard instead." Multi-layer replacement: discard-else-mill. Scope: 1-2 hr.
- **Misthollow Griffin exile-cast static** — "you may cast this from exile." Self-static zone-cast grant. Food Chain + Misthollow Griffin = infinite creature-exile-cast = infinite mana. Scope: 1 hr.
- **Sanctum Weaver enchantment-count mana** — "tap: add X mana where X is the number of enchantments you control." Scaling mana per permanent class. Same primitive family as Gaea's Cradle. Scope: covered.
- **Tyvar, Jubilant Brawler mass mana-ability grant** — "creatures you control have 'tap: add G'." Similar to Enduring Vitality. Covered. Scope: 30 min verification.
- **Quirion Ranger basic-bounce-untap** — "return Forest you control to its owner's hand: untap target creature." Cost with permanent-return-to-own-hand. Scope: 1 hr.
- **Razaketh, the Foulblooded (sac-for-tutor)** — "pay 2 life, sac another creature: search library for a card and put it into hand." Repeatable sac tutor. Scope: 1 hr.
- **Fauna Shaman / Survival of the Fittest discard-tutor** — "{G}, tap, discard creature: search library for creature card." Already covered. Scope: covered.
- **Neoform / Eldritch Evolution / Chord of Calling** — sacrifice + CMC-constrained tutor. Covered by Lumra. Scope: covered.
- **Takenuma, Abandoned Mire (channel ability)** — "{2}{B}, discard this land: return target creature or planeswalker from your graveyard to your hand." Alternative activation from hand. Scope: 1 hr.
- **Animate Dead / Necromancy reanimation auras** — aura entering attaches to graveyard creature, returns it. Aura-ETB-from-graveyard + reanimation pattern. Scope: 1-2 hr.
- **Undercity Sewers / Rejuvenating Springs / Undergrowth Stadium (surveil lands)** — "enters tapped, taps for 2 colors, when enters surveil 1/2." Surveil keyword. Scope: 1 hr.

**Critical combo chains**: Hermit Druid → mill entire library → Thoracle from gy via Muldrotha → exile library → win. Or: Hermit Druid → mill → Necrotic Ooze + Phyrexian Devourer + Walking Ballista (all in gy) → Ooze imports Devourer exile-library-for-+/+ + Ballista's remove-counter-ping → lethal. These are **state-machine tests** — the engine must detect when no more valid actions exist (library empty, nothing to mill) and not hang.

**Total added scope**: ~15-20 hr. Necrotic Ooze is the spicy one — dynamic ability inheritance across zones is not a pattern any other card in the portfolio uses. Muldrotha's per-type-per-turn tracker is a nice primitive reused by Karador, Sidisi, and other graveyard commanders.

**cEDH portfolio extended**: 9 decks. Covers every major graveyard-recursion shell too. Hermit Druid specifically gates on "deck has zero basics" which is a deck-construction constraint the engine doesn't currently validate but should (or just let it mill non-infinitely in malformed decks).

---

## PARSER-SIDE GAPS (Python parser, affects both engines via shared dataset)

1. **Tergrid trigger payoff** — "put that card onto battlefield under your control"
2. **Demonic Consultation payoff** — partially `parsed_effect_residual`
3. **Storm keyword** — emit structured Storm ability
4. **Banding keyword** — recognize + emit typed node
5. **Phasing keyword** — same
6. **Goad keyword** — same
7. **Daybound / Nightbound** — same (+ transform pattern)
8. **Cast-from-graveyard flashback variants** — Flashback, Encore, Jump-Start
9. **Cast-count scaling** — "for each other spell cast this turn" as `ScalingAmount` kind
10. **Final Fortune pattern** — "take an extra turn. At end of that turn, you lose the game."

---

## SUMMARY

**Tier 1 total scope**: ~40-65 hr
**Tier 2 total scope**: ~20-32 hr
**Tier 3 total scope**: ~15-22 hr (defer most)
**Parser-side gaps**: ~10-15 hr

**Total feature-patching work**: **85-134 hours of agent work**, probably 30-50 hours wall-clock with multiple parallel agents.

**Priority order for agent launches**:
1. Tier 1 items 1-4 in parallel (independent subsystems)
2. Tier 1 item 5 + Tier 2 items 6-10 after Tier 1 lands
3. Tier 2 items 11-14 in parallel (per-card handlers)
4. Tier 3 items deferred post-nightmare-list

**Post-feature-patching**: Nightmare list exercise runs against all 20 decks with proper sample-size gauntlets. Expected outcome: B3/B4 decks play their themes correctly, B5 cEDH decks produce real combo-win pressure, engine produces usable meta data.

Only after feature patches land AND nightmare list validates → archive Python → Go-only → Phase 13 50k benchmark (cosmetic metric).

---

## CORRECTNESS GATE (added 2026-04-16)

Before any post-wave-#3 gauntlet cycle is treated as gameplay signal, the engine must pass the following unit-level correctness tests. Each is a focused, isolated scenario with an asserted outcome.

### Combat & removal
1. **Commander lethal damage** — seat with 14 life takes 21 damage from Commander X in a single source → seat loses, `end_reason="commander_damage"`, event stream shows `commander_damage_accum` and `seat_loss`.
2. **Creature kill via combat damage** — 3/3 blocks 4/4, both die, §704.5g triggers cleanly, both go to graveyard, creature-death-triggered abilities fire.
3. **Removal spell resolution** — Swords to Plowshares on a 3/3, target exiled, controller gains 3 life, creature is in exile zone not graveyard.
4. **Board wipe** — Wrath of God resolves, ALL creatures across all seats move to graveyard simultaneously, no regeneration triggers fire for non-regenerable creatures.
5. **Indestructible interaction** — Wrath of God + indestructible creature on board → indestructible creature survives, everything else dies.

### Mana & cost
6. **Sol Ring taps for {C}{C}** — Sol Ring on battlefield, tap it, mana pool has 2 colorless.
7. **Mox Diamond produces chosen color** — Mox Diamond ETB (discard land), tap for {W} OR {U} OR {B} OR {R} OR {G} based on activation choice.
8. **Lotus Petal consumed on activation** — tap, sacrifice, add one color, permanent now in graveyard.
9. **Food Chain creature-restricted mana** — exile a creature, gain (CMC+1) creature-restricted mana, payable on creature spells, NOT payable on sorcery.
10. **Mana pool drains at phase boundaries** — generate 2 mana in precombat main, transition to combat step → mana pool is empty unless Upwelling/Omnath present.
11. **Omnath retains green across phases** — Omnath on board, generate {G}{G}{G} in main 1, transition to main 2 → green mana still in pool, other colors would have drained.
12. **Treasure taps for any color** — Treasure token on battlefield, tap + sacrifice → +1 mana of any chosen color.

### Casting & costs
13. **Force of Will alternative cost** — exile a blue card + pay 1 life → Force of Will casts for free, counterspell resolves.
14. **Fierce Guardianship conditional free-cast** — commander on battlefield → Fierce Guardianship casts for free.
15. **Deadly Rollick alt-cost when commander in play** — commander on battlefield → Deadly Rollick casts for free; otherwise costs 4B.
16. **Eldritch Evolution sac-cost** — sacrifice creature X, exile self, search library for creature CMC ≤ X.cmc+2, put onto battlefield.
17. **Jump-start discard-cost** — cast flashback/jump-start spell from graveyard → discard a card + cast cost paid, spell resolves.
18. **Storm copies deal body damage** — cast Grapeshot with 4 prior spells cast this turn → 5 storm copies resolve, 5 total damage dealt.
19. **Tendrils of Agony storm body-effect** — cast Tendrils with 9 prior spells → 10 total copies each drain 2 life + you gain 2 life → 20 life drained.
20. **Commander cast-from-command-zone tax** — cast commander 3x from command zone, each cast adds +2 mana to the next cost.

### State-based actions
21. **Thassa's Oracle win** — Thassa's Oracle ETB, library size ≤ devotion to blue → seat wins, game over.
22. **Library-empty loss** — seat must draw a card but library is empty → seat loses, `end_reason="library_empty"`.
23. **Legend rule** — two copies of legendary permanent X on battlefield under same controller → controller chooses one to keep, other goes to graveyard.
24. **Zero-toughness SBA kill** — creature at 0 toughness → moves to graveyard at next SBA check.

### Partner / multi-commander
25. **Partner commander damage tracked separately** — Kraum deals 15 damage, Tymna deals 10 damage to same seat → seat at no-loss state. Kraum deals 21 alone → seat loses.
26. **Both partner commanders in command zone at start** — Kraum+Tymna deck starts with exactly 2 commanders in command zone.

### Day/Night / DFC (post wave #2)
27. **Day/Night state initialization** — game starts at `"neither"` → first daybound creature entering sets to `"day"`.
28. **Night→day transition** — during your turn, 2+ spells cast → end-turn transition sets state to `"day"`; daybound creatures transform to night faces.
29. **Ral Monsoon Mage DFC loads** — `cedh_stormoff_b5_ral.txt` parses, Ral enters command zone as "Ral, Monsoon Mage" (front face), castable.

### Per-card (post wave #2)
30. **Food Chain + Misthollow Griffin infinite loop** — exile Griffin for 3 creature-mana, cast Griffin from exile for 3 mana → mana-neutral loop, engine detects + terminates after depth cap.
31. **Hullbreaker Horror cast-trigger bounce** — cast any spell → Hullbreaker triggers, bounces target nonland permanent.
32. **Aetherflux Reservoir lifegain + damage** — cast a spell → gain 1 life; at 50 life, pay 50 life for 50 damage activation.

**Target**: 32/32 green before trusting gauntlet winrates as gameplay signal. Running 9-deck × 100-game with a failing correctness gate is worse than running no gauntlet at all — it produces confident-looking numbers that are gibberish.

---

## NOTED FOR LATER (2026-04-16 rulings from 7174n1c)

### Fellwar Stone strategic counterplay (edge case)

Fellwar Stone's oracle: "{T}: Add one mana of any color a land an opponent controls could produce." The runtime check (`_opponent_land_colors`) evaluates opposing lands at tap time.

**Not yet handled (deliberately deferred)**: opponents can't strategically tap their own lands BEFORE Fellwar activates to deny color coverage. Example: seat Y has only Forests. Seat X taps Fellwar, green is available. Seat Y could theoretically preemptively tap all their Forests during a prior priority window (if it were their turn) to remove green from the pool of "could produce" — but this is strategic counterplay that violates no rule, and rules engines don't typically model it.

**Ignore for now.** Future enhancement: a hat-driven "Fellwar denial" strategic evaluator that considers whether denying a color to an opposing Fellwar activation is worth the tempo cost of sandbagging. Unlikely to matter in any gauntlet scenario in the 9-deck portfolio.

### Land tap ability unique ruling (note)

Lands with `{T}: Add X mana` are mana abilities per CR §605.1 — they don't use the stack. Opponents can't respond to a land tapping. This is already handled correctly in `fill_mana_pool` (no stack_push event for land taps; just direct pool modification). Included here as a note for future agents so nobody tries to wire priority handling around land taps.

### Flip-face-up triggered ability body-effect parsing (note)

Face-down family cards (morph/manifest/disguise/cloak) correctly emit `Triggered(event='turned_face_up', ...)` AST nodes when a "When ~ is turned face up, ..." ability is present. **Labeling is correct.** However, the EFFECT body of those triggers often parses as `UnknownEffect` because the effect text doesn't match existing parser dictionary entries. 70 face-down-family cards currently affected.

Examples of unhandled effect bodies on `turned_face_up` triggers:
- "each opponent skips their next untap step" (Brine Elemental) — skip-untap-step effect
- "if it's your turn, gain control of another target creature until end of turn" (Jeering Instigator) — conditional gain-control
- "you may draw a card" (Hystrodon — triggered on combat damage, not face-up, but same body pattern) — 'you may' prefix throws draw regex
- "destroy target artifact or enchantment an opponent controls" (Ainok Survivalist) — parses as `Destroy` correctly, contrast showing it's a per-body issue
- "until your next turn, up to one target creature gains vigilance and reach" (Vivien Champion) — keyword-grant duration
- "if it's a creature, put two +1/+1 counters on it" (multi-manifest cards) — conditional on card-type

**Scope**: this is a generic triggered-ability-body-effect parser coverage issue, not face-down specific. Any triggered ability with an uncommon effect body exhibits it. Adding parser rules for the top 20-30 phrases would close most of the gap.

**Priority**: medium. Face-down decks play their gameplan correctly at the mechanic level; the UnknownEffect bodies only cost specific card-unique interactions. Add after higher-priority body-effect work (cast-from-graveyard, additional cost enforcement).

Estimated time to build the full correctness suite: 3-4 hours (atomic tests, each focused). Estimated time to pass all 32 after wave #2 + wave #3: dependent on agent deliveries. If all three wave #2 agents land clean + wave #3 lands clean, 22/32 should be green on first run, with the rest being integration-level edge cases that need targeted fixes.
