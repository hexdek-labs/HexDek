# Precon Shape Scans — Group A (decks 1-30)

Phase C reads of `data/decks/wizards/*.txt`, sorted ascending, rows 1-30. One section per deck: intended archetype, the cards that ARE the deck (punch-up shape), Freya's `measured_bracket` call, my verdict on whether the engine read it correctly, and a 2-3 sentence reasoning. Verdict legend: **engine-correct** = bracket+archetype both look right; **engine-off** = bracket is miscalibrated (most often a B4 false-positive); **classification-mismatch** = bracket reasonable but the archetype tag is wrong; **unclear** = signals don't resolve cleanly.

Source data: `data/decks/wizards/freya/<stem>.profile.json` + `.strategy.json` (regenerated from `go run ./cmd/hexdek-freya/ --all-decks data/decks/wizards/` at the head of this branch). Cross-references to R1-R7 calibration findings inlined where relevant.

---

### Bello, Bard of the Brambles — Animated Army (Bloomburrow / BLB)
- **Intended archetype:** Naya enchantments-as-creatures midrange. Bello's gimmick: noncreature permanents with MV ≥ 4 also gain "this is a creature" with P/T equal to MV.
- **Punch-up shape:** Bello + value noncreature permanents that become 4/4+ bodies (Esika's Chariot, Smothering Tithe, Vanquisher's Banner) + ETB doublers (Warstorm Surge) + enchantress draw (Setessan Champion). 5-mana noncreatures are the sweet spot — a stock Big Score that ramps in the early game becomes a 5/5 attacker post-Bello.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** B2 is right for a stock landfall-noncreature midrange precon with 0 GCs / no fast mana / no real combo. The "Greater Good + Starstorm + Big Score" win-line Freya surfaces is incidental (3 mid-curve red spells, not a closed loop), but it doesn't trip the B4 floor predicates so the bracket holds.

### Galea, Kindler of Hope — Aura of Courage (Forgotten Realms / AFR)
- **Intended archetype:** Bant Auras/Equipment Voltron. Galea casts the top card of the library if it's an Aura or Equipment.
- **Punch-up shape:** Galea + Sword cycle (Sword of Feast and Famine et al.) + cheap repeatable buffs (Mantle of the Wayfarer, Sword of the Animist) + tutors that put auras/equipment on top (Sram, Senior Edificer for card flow + Open the Armory). Voltron win condition is commander damage, not a combo.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Voltron + 4 win lines + 0 GCs + C-grade mana base — B2 lines up exactly with how the deck plays. The Freya "Voltron" archetype tag is accurate (rare among stock precons; most read as midrange).

### Perrie, the Pulverizer — Bedecked Brokers (Streets of New Capenna / SNC)
- **Intended archetype:** Bant +1/+1 counters / Shield-counter tribal. Perrie distributes counters when shields trigger.
- **Punch-up shape:** Perrie + cheap proliferators (Inexorable Tide, Pollenbright Wings) + Bant counter payoffs (Bilbo Birthday Celebrant, Hardened Scales) + Brokers shield-counter generators (Aven Mimeomancer, Brokers Charm). Wincon is the counter density itself, not a closed loop.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Stock precon, B2 stands. 6 win lines + 0 GCs + 50%-power-pct + C-grade mana base. Freya's "midrange" archetype tag is one beat off — this reads as a tribal counters deck — but the bracket isn't sensitive to that distinction.

### Nelly Borca, Impulsive Accuser — Blame Game (Karlov Manor / MKM)
- **Intended archetype:** Mardu goad / suspect / draw-on-attack. Nelly draws when an opponent attacks another opponent.
- **Punch-up shape:** Nelly + goad enablers (Disrupt Decorum, Bident of Thassa) + suspect interaction + curse-the-board (Curse of Bloodletting, Bedevil) + the Mardu Etali-style combat reset. Win condition is incidental damage off goaded armies + draw-engine snowball.
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off (B4 false-positive)
- **Reasoning:** Documented in R5 (PR #533) and R7 cumulative tracker — Blame Game trips the `Tuned-redundancy floor` predicate at raw score 4 (10 finishers + 7 fast-mana pieces), and the floor lifts to B4 despite zero GCs and zero true-infinite combos. A goad-and-draw precon with 55% power and 40% cmdr-synergy is clearly Core-tier; this is a known engine-side fix surface, not a real B4.

### Sarah Jane Smith — Blast From The Past (Doctor Who)
- **Intended archetype:** Esper Doctor's companion + Historic spells. Sarah Jane returns historic spells from graveyard when a Doctor enters.
- **Punch-up shape:** Sarah Jane + Doctor commanders / Doctor's companions theme + sagas + legendaries + Doctor Who flavor cards (The First Doctor, The Night of the Doctor) + Gallifrey Stands. Recurring historics is the engine; combat damage off the cumulative-value pile is the wincon.
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off (B4 false-positive)
- **Reasoning:** Documented in R1 (PR #508) as the original B4 FP case alongside Urza. Scattered Groves + Jo Grant gets flagged as a 2-card "determined" combo because of Jo Grant's land-recursion trigger, but it's not a closed loop — the deck is a casual companion-and-historic value pile. D-grade mana base (D!) is itself disqualifying for an actual B4; this is the same combo-detector and floor-predicate FP cluster that surfaced again in R7.

### Atraxa, Praetors' Voice — Breed Lethality (Commander 2016)
- **Intended archetype:** 4-color Superfriends / proliferate / +1/+1 counters. Atraxa proliferates everything on attack.
- **Punch-up shape:** Atraxa + planeswalkers (Tezzeret, Sorin, Ajani Steadfast) + counter payoffs (Custodi Soulbinders, Skullbriar) + proliferate density (Inexorable Tide, Contagion Engine). Wincon is grinding planeswalker ults or scaling Soulbinders into a lethal army.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** classification-mismatch
- **Reasoning:** B2 is right for a stock C16 precon (A-grade mana, 12 win lines, no GCs), but Freya's "Tribal" archetype call is wrong — this is the canonical superfriends/proliferate precon, not tribal in any meaningful sense. The Custodi Soulbinders + Ghave combo Freya highlights is real but the deck doesn't tutor for it.

### Kotori, Pilot Prodigy — Buckle Up (Kamigawa: Neon Dynasty / NEO)
- **Intended archetype:** Bant Vehicles tribal. Kotori makes vehicles enter as artifact creatures without needing crew.
- **Punch-up shape:** Kotori + cheap vehicles (Smuggler's Copter, Esika's Chariot, Mobilizer Mech) + crew-as-needed creatures + artifact synergy support (Sai, Master Thopterist, Sram). Wincon is wide vehicle attacks; commander damage is a viable secondary line (Kotori isn't a hot commander but the deck supports a Voltron-ish pivot).
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** A-grade mana, 88.7% commander synergy (high for a stock precon — the deck is genuinely vehicle-tribal-tight), 60% power, 0 GCs, no real combos. Solid B2 read; the Artifacts/Midrange hybrid archetype tag is fine.

### Daretti, Scrap Savant — Built From Scratch (Commander 2014)
- **Intended archetype:** Mono-red artifact reanimator / discard-recur. Daretti's −2 reanimates artifacts from graveyard.
- **Punch-up shape:** Daretti + discard outlets (Faithless Looting, Smoldering Crater) + Trading Post for graveyard-to-hand + big-mana artifact finishers (Steel Hellkite, Bosh, Wurmcoil Engine) + Goblin Welder–style swap effects. Wincon is recur-and-loop big artifacts into the board.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** A-grade mana but mono-red and the artifact-loop engines are shallow (depth 3, LOW/MEDIUM redundancy). 60% power and 0 GCs — B2 fits. The Faithless Looting + Trading Post + Daretti win-line Freya names is accurate: that IS the deck's core loop, but it grinds value rather than wins outright, so the B2 stamp holds.

### Kitt Kanto, Mayhem Diva — Cabaretti Cacophony (Streets of New Capenna / SNC)
- **Intended archetype:** Naya go-wide tokens with combat tricks. Kitt Kanto distributes +1/+1 counters and incentivizes attacks elsewhere.
- **Punch-up shape:** Kitt Kanto + token-makers (March of the Multitudes, Call the Coppercoats, Martial Coup, Thunderfoot Baloth as anthem-finisher) + treasure / coin-counter alliance cards (Phabine, Rose Room Treasurer) + +1/+1 anthems. Wincon is a wide swing turn after multiple anthem stack-ups.
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off (B4 false-positive)
- **Reasoning:** Documented in R7 (this branch's PR #536). Trips the `Winning-combo floor` predicate via Rose Room Treasurer + Scute Swarm flagged as a 2-card "infinite_damage" combo — but Scute Swarm requires lands to enter, not arbitrary engines, and the combo isn't closed. 59 "win lines" detected on a token precon is also a finisher-counter blowup. 50.8% commander synergy and 0 GCs both contradict the B4 call.

### Sidar Jabari of Zhalfir — Cavalry Charge (March of the Machine / MOM)
- **Intended archetype:** Mardu Knights tribal with phasing protection. Sidar Jabari phases out attackers to dodge blockers + removal.
- **Punch-up shape:** Sidar Jabari + Knight tribal payoffs (Knight Exemplar, Worthy Knight) + Mardu anthems (Knights' Charge, Elspeth Sun's Champion) + flicker / phasing tricks. Wincon is grinding combat damage off resilient knight bodies.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Stock Knight tribal precon; 58% power, C-grade mana, 0 GCs, no real combos. The "midrange" archetype tag is reasonable — knights are a creature-combat midrange shell — though "Knight Tribal" would be more precise. Bracket reads correctly.

### Ixhel, Scion of Atraxa — Corrupting Influence (Phyrexia: All Will Be One / ONE)
- **Intended archetype:** Sultai Toxic / Corrupted / Proliferate. Ixhel rewards opponents being "corrupted" (3+ poison counters) with card draw + drains.
- **Punch-up shape:** Ixhel + cheap toxic creatures (Tyvar's Stand–style infect support, Glistening Sphere) + proliferate enablers (Inexorable Tide, Karn's Bastion, Vraska's Fall) + Phyrexian board wipes (Carrion Call, Noxious Assault). Wincon is the poison drip itself, not a closed loop.
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off (B4 false-positive)
- **Reasoning:** 11.5% commander synergy + 45% power + C-grade mana + 0 GCs do not assemble a B4. 37 "win lines" on a stock toxic precon is the same finisher-detector blowup the `Tuned-redundancy floor` predicate keeps tripping (R3 documented Forces of the Imperium / Squirreled Away the same way). The toxic axis doesn't even register in Freya's themes ("blink" is the only theme tag); the archetype call ("Midrange / Tribal") misses the actual design.

### Inspirit, Flagship Vessel — Counter Intelligence (Edge of Eternities / EoE)
- **Intended archetype:** Esper Spacecraft (new Vehicle subtype) artifact precon. Inspirit stations crew onto spacecraft cheaply.
- **Punch-up shape:** Inspirit + spacecraft (the new MV-counter-on-station artifact subtype) + counter manipulators (Bilbo Birthday Celebrant analogs, +1/+1 counter doublers) + artifact protection (Darksteel Reactor as a wincon, Threefold Thunderhulk for board impact). Wincon is grinding spacecraft anthems or scaling Reactor for the win.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** 98.3% commander synergy is the highest in group A — the deck is genuinely Inspirit-centric. B2 is the right call for a stock EoE precon with 0 GCs and C-grade mana. The "Lonely Sandbar + Mindless Automaton" combo Freya flags is the same cycling-land FP cluster that PR #530 partially closed; Mindless Automaton sacs counters but doesn't loop with a cycling-once Sandbar — incidental detector noise.

### Leinore, Autumn Sovereign — Coven Counters (Innistrad: Midnight Hunt / MID)
- **Intended archetype:** Bant Coven (3 different power) / +1/+1 counters / human-angel tribal. Leinore puts counters on creatures and rewards coven (3 different-power creatures).
- **Punch-up shape:** Leinore + Coven-trigger enablers (Brutal Cathar, Sigardian Savior) + +1/+1 counter doublers (Doubling Season analogs not stock; Gyre Sage stock) + angel/human payoffs (Bastion Protector, Angel of Glory's Rise). Wincon is wide attacks with anthem-stacked humans + angel finishers.
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off (B4 false-positive)
- **Reasoning:** Angel of Glory's Rise + Moorland Rescuer flagged as `infinite` is the engine over-reading a 2-card reanimator combo that requires graveyard fuel + mass-creature loop and isn't actually closed on this precon. 41.9% commander synergy + 52% power + 0 GCs + no real combos + A-grade mana (the deck does have ok manabase though) — stock human-coven tribal does NOT play at B4. Same Tuned-redundancy floor or Winning-combo floor surface as the R7 findings.

### Satya, Aetherflux Genius — Creative Energy (Modern Horizons 3 / MH3)
- **Intended archetype:** Jeskai Energy / Artifacts. Satya creates clone tokens of energy-producing artifacts.
- **Punch-up shape:** Satya + energy producers (Aetherflux Reservoir adjacent — though that specific card isn't stock; the deck runs Esperzoa-style flickers) + cheap artifact creatures + energy payoffs (Aethergeode Miner, Crackling Drake analogs). Wincon is Satya cloning a value engine + grinding card advantage through energy.
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off (B4 false-positive)
- **Reasoning:** Documented in R3 as part of the 3/15 B4 FP cluster. Same shape as the other Tuned-redundancy-floor cases: 14 win lines + 1 GC (Farewell — the bare minimum to NOT trip the GC=0 ceiling, but the floor predicate doesn't care) + the Aethergeode Miner + Combustible Gearhulk "combo" which is just "if you have both on the battlefield, value happens." 58% power + 47.5% commander synergy clearly says Core not Optimized.

### Kaust, Eyes of the Glade — Deadly Disguise (Karlov Manor / MKM)
- **Intended archetype:** Sultai Disguise / Cloak / Face-down +1/+1 counters tribal. Kaust supports the disguise mechanic.
- **Punch-up shape:** Kaust + disguise creatures (Krosan Cloudscraper, Brokers Veteran adjacent) + face-down tricks + +1/+1 counter payoffs. The R5 doc flagged this as one of the rare R7-era decks where the bracket call is right but the cmdr_syn is suspiciously low.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** 25% power and F-grade mana — the engine reads this as the floor of B2, which matches what a low-power face-down precon plays like. 2 GCs (Jeska's Will + Seedborn Muse) raise eyebrows for a stock precon but Freya doesn't lift the bracket on GC=1-3 without corroborating signals, so B2 holds. The Krosan Cloudscraper + Showstopping Surprise "infinite_damage" flag is a creature-stat overflow that requires specific board state; not a real combo, but it doesn't lift the bracket here.

### Winter, Cynical Opportunist — Death Toll (Duskmourn / DSK)
- **Intended archetype:** Golgari Survival Horror aristocrats. Winter drains opponents when permanents go to graveyards.
- **Punch-up shape:** Winter + token-makers (Wrenn and Seven, Grist) + sac outlets + death-trigger payoffs + Survival Horror Duskmourn flavor cards. Wincon is the drain itself; combat is a secondary line.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Clean B2 read on a stock Golgari aristocrats precon. 55% power, B-grade mana, 5 win lines, 0 GCs, no flagged combos. The "midrange" classification is fine — aristocrats fits inside midrange's umbrella for stock-precon purposes; calling this out as a separate archetype tag is the kind of thing the deckbuilder confirmation flow (PR #421) would resolve.

### Morska, Undersea Sleuth — Deep Clue Sea (Karlov Manor / MKM)
- **Intended archetype:** Bant Clues / Investigate value-grind. Morska draws when clues are sacrificed for value.
- **Punch-up shape:** Morska + investigate generators (Tireless Tracker, Ulvenwald Mysteries, Killer Service) + clue-sac payoffs (Graf Mole, Sophia Dogged Detective) + token-makers to fuel investigate. Wincon is the Aristocrats-style drain off clue sacs amplified by Morska's draw engine.
- **Freya measured_bracket:** 3 Upgraded
- **Your verdict:** engine-correct
- **Reasoning:** Documented in R5 as one of the rare engine-lifts-correctly cases. 93.5% commander synergy is the second-highest in this batch — the deck IS genuinely Morska-centric and the investigate-aristocrats engine is real. The Sophia + Tracker + Graf Mole + Killer Service "combo" Freya highlights isn't a closed infinite, but it's a strong 4-card value chain that justifies the B3 lift. 1 GC (Farewell) + 20 win lines + 25 Draw tags is honest Upgraded territory.

### Yuma, Proud Protector — Desert Bloom (Outlaws of Thunder Junction / OTJ)
- **Intended archetype:** Naya Deserts / Landfall / Sand tribal. Yuma turns deserts into 3/3 Sand creatures.
- **Punch-up shape:** Yuma + desert lands (the new OTJ subtype) + landfall payoffs (Titania, Avenger of Zendikar) + recursion (Sevinne's Reclamation, World Shaper). Wincon is the landfall engine itself plus sand-creature combat.
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off
- **Reasoning:** The Titania + Sand Scout `infinite_tokens` combo IS mechanically real (Sand Scout's attack trigger returns lands; Titania triggers when lands die — bounce-and-replay loop), but a stock precon doesn't reliably assemble it (no tutors, D-grade mana, only 45% power). This is the kind of borderline case where the engine's "infinite combo = B4 minimum" rule from the WotC carveout over-rates the practical play pattern. 48 win lines on a landfall precon is the same finisher-detector blowup that drove the R7 B4 FPs.

### Kasla, the Broken Halo — Divine Convocation (March of the Machine / MOM)
- **Intended archetype:** Bant Convoke / Incubate token midrange. Kasla puts halo counters on creatures for end-step damage.
- **Punch-up shape:** Kasla + convoke creatures (Knight of the New Coalition adjacent) + incubate tokens + ETB blink payoffs (Mistmeadow Vanisher) + token-makers. Wincon is the halo-counter damage stacking + go-wide token swarm.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** 58% power + 0 GCs + 15 win lines + B-grade mana — clean B2 read on a stock convoke precon. Mistmeadow Vanisher + Cloud of Faeries flagged as `infinite_mana` is the closest the deck comes to a real combo (Vanisher blinks Cloud of Faeries which untaps lands), but it's not assembled without tutors, and Freya correctly doesn't lift the bracket on a 1-piece-determined-loop without GC corroboration.

### Vrondiss, Rage of Ancients — Draconic Rage (Adventures in the Forgotten Realms / AFR)
- **Intended archetype:** Jund Dragons tribal with self-damage. Vrondiss creates 4/4 elementals whenever a creature you control is dealt damage.
- **Punch-up shape:** Vrondiss + dragon tribal payoffs (Skyline Despot, Bogardan Hellkite, Terror of Mount Velus) + ETB doublers (Warstorm Surge) + cheap pings to trigger Vrondiss's elemental factory. Wincon is wide dragon attacks + Warstorm-style ETB pings.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** 11.7% commander synergy is low (most cards are generic dragons not optimized around Vrondiss's specific self-damage axis — the precon is more "dragons" than "Vrondiss"), but the B2 call is right: 48% power, B-grade mana, 0 GCs, 11 win lines, no real combos. Stock dragon tribal sits comfortably at Core.

### Sefris of the Hidden Ways — Dungeons of Death (Forgotten Realms / AFR)
- **Intended archetype:** Esper Dungeon-venture reanimator. Sefris triggers dungeon-venture when a creature dies, and reanimates a CMC ≤ 3 creature on full-dungeon completion.
- **Punch-up shape:** Sefris + dungeon-venture enablers (Acererak the Archlich, AFR-set dungeon cards) + small-creature sac fodder + reanimation payoffs (Eternal Dragon, Karmic Guide, Sharuum the Hegemon adjacent). Wincon is loop reanimation of value targets via repeated dungeon completions.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** 58% power, B-grade mana, 0 GCs, 7 win lines — clean B2 read. The Eternal Dragon + Unburial Rites "combo" Freya flags is a real value engine but stock doesn't tutor for it; the deck plays as grindy aristocrats/reanimator at Core power, exactly as the engine reads it.

### Ulalek, Fused Atrocity — Eldrazi Incursion (Modern Horizons 3 / MH3)
- **Intended archetype:** 5-color Eldrazi tribal (Devoid/Colorless ramp). Ulalek copies cast triggers of Eldrazi spells.
- **Punch-up shape:** Ulalek + classic Eldrazi (Eldrazi Displacer, Drowner of Hope, Warping Wail, Azlask) + colorless ramp (Eldrazi Temple, Ugin's Labyrinth adjacent) + scion-token generators. Wincon is big-mana Eldrazi turns + Ulalek's copy-trigger amplification.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Eldrazi Displacer + Drowner of Hope IS a real infinite-mana combo (Displacer untaps for 3, Drowner makes scions that sac for mana — net positive when configured), and Freya correctly flags it. But the bracket correctly doesn't lift because the precon has 0 GCs, 48% power, only 9 win lines, and no tutor density to assemble the combo reliably. The combo-floor predicate correctly didn't fire here — useful counterexample to the over-aggressive combo-floor cases (World Shaper, Cabaretti Cacophony) where it DID fire on weaker combo evidence.

### Galadriel, Elven-Queen — Elven Council (The Lord of the Rings: Tales of Middle-earth / LTR)
- **Intended archetype:** Bant Elves tribal with The Ring tempts mechanic. Galadriel mills/ascend-pumps elf-heavy boards.
- **Punch-up shape:** Galadriel + elf tribal payoffs (Elvish Archdruid, Radagast, Wizard of Wilds) + The Ring tempts triggers + go-wide token-makers (Hornet Queen, Sylvan Offering) + auras / pump (Lothlórien Blade). Wincon is wide elf swarms with anthem-pump combat damage.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Stock elf tribal with The Ring flavor; 63% power, A-grade mana, 0 GCs, no real combos. B2 fits. Colossal Whale + Lothlórien Blade gets flagged because the Blade adds combat-damage triggers; the line is "real" but only as a finisher, not a combo. The "Tribal" archetype tag is accurate even if Freya leaves it at "Midrange / Tribal" hybrid.

### Valgavoth, Harrower of Souls — Endless Punishment (Duskmourn / DSK)
- **Intended archetype:** Rakdos Horror-discard reanimator. Valgavoth grows when an opponent discards/sacs/loses life and reanimates after death.
- **Punch-up shape:** Valgavoth + discard outlets + opponent-discard effects (Tinybones, Trinket Thief adjacent) + recursion (Decree of Pain, Bastion of Remembrance) + Rakdos value (Rakdos, Lord of Riots; Kaervek). Wincon is Valgavoth as a recurring threat plus drain-and-recur value attrition.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** 68% power and A-grade mana are notable for a stock B2 — this is one of the better-tuned modern precons. 0 GCs and 7 win lines keep the bracket honest at Core, but the deck is right at the B2/B3 boundary. Canyon Slough + Grab the Prize flagged as a 2-card win-line is a cycling-land FP cluster (same shape as Counter Intelligence + Blast from the Past); not a real combo, and correctly doesn't lift the bracket.

### Otrimi, the Ever-Playful — Enhanced Evolution (Commander 2020)
- **Intended archetype:** Sultai Mutate tribal. Otrimi enables mutate-from-graveyard.
- **Punch-up shape:** Otrimi + mutate creatures (Migratory Greathorn, Trumpeting Gnarr, Auspicious Starrix) + graveyard fillers + recursion enablers. Wincon is stacked mutate triggers building one giant value-creature.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Classic mutate precon; 50% power, B-grade mana, 0 GCs, 6 win lines. B2 is the right read. Freya's "Midrange" archetype tag misses the mutate-tribal identity (33.9% commander synergy reflects that the precon includes a lot of generic Sultai value alongside the mutate package), but the bracket call is correct.

### Oloro, Ageless Ascetic — Eternal Bargain (Commander 2013)
- **Intended archetype:** Esper Lifegain control. Oloro gains 2 life per upkeep + drain payoff via Vizkopa Guildmage / Aetherflux.
- **Punch-up shape:** Oloro + lifegain-as-resource (Vizkopa Guildmage IS stock; Exquisite Blood / Sanguine Bond — Freya flags Sanguine Bond is stock but Exquisite Blood is missing for true_infinite) + control wipes (Toxic Deluge, Nevinyrral's Disk) + Esper artifact value (Sharuum, Thopter Foundry). Wincon is the Sanguine Bond / Exquisite Blood drain loop on upgrade, or just outlasting opponents with grindy lifegain pre-upgrade.
- **Freya measured_bracket:** **1 Exhibition**
- **Your verdict:** engine-off (B1 is too cold)
- **Reasoning:** This is the ONLY B1 measurement in group A and it reads as the engine under-rating a stock C13 precon that does have a real upgrade-target combo (Sanguine Bond → Exquisite Blood) and a soft Vizkopa Guildmage + Oloro line. Stock precon with 50% power + B-grade mana + 22 threats + 2 near-true-infinite combo lines should sit at B2 Core, not B1 Exhibition. Possible engine-side cause: lifegain archetype + low win-line count + no GCs may be tripping a too-aggressive B1 floor on archetypes Freya reads as "value-grindy with no clean wincon." Worth flagging for the lifegain-archetype calibration pass.

### Morophon, the Boundless — Everyone's Invited! (Secret Lair Commander 2025)
- **Intended archetype:** 5-color "any" tribal — Morophon names a creature type at ETB and supports it.
- **Punch-up shape:** Morophon + flexible tribal payoffs (The Bears of Littjara, Kindred Dominance) + generic ramp (Farseek) + wide-tribal anthems (Avenger of Zendikar, Kinsbaile Cavalier). Wincon is whatever tribe you commit to + Kindred Dominance as a one-sided wipe.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Generic Secret Lair tribal precon; 55% power, C-grade mana, 0 GCs, 7 win lines. B2 read is right. The "Tribal" archetype tag matches the design (Morophon's design IS "pick a tribe"); cmdr_syn 48.4% reflects that without a named tribe, half the cards aren't synergy-bound to Morophon specifically.

### Kadena, Slinking Sorcerer — Faceless Menace (Commander 2019)
- **Intended archetype:** Sultai Morph / Manifest / Face-down tribal. Kadena makes the first face-down creature each turn free.
- **Punch-up shape:** Kadena + morph creatures (Voidmage Apprentice, Ainok Survivalist) + manifest enablers + flip-up payoffs + Sultai value (Vraska the Unseen). Wincon is value-grind off cheap morphs flipping into threats + Kadena rebate.
- **Freya measured_bracket:** 2 Core
- **Your verdict:** engine-correct
- **Reasoning:** Stock face-down precon; 43% power and D-grade mana (low) but 1 GC (Seedborn Muse — a stretch on a casual face-down precon, but Freya's GC list has it). 18.6% commander synergy reflects that the precon's face-down package is thin; most cards are generic Sultai value. B2 is the right call — the GC=1 ceiling rule correctly keeps it from lifting.

### Zinnia, Valley's Voice — Family Matters (Bloomburrow / BLB)
- **Intended archetype:** Bant Frogs/Otters/Bloomburrow-tribe go-wide token build. Zinnia bounces creatures back for ETB-trigger reuse.
- **Punch-up shape:** Zinnia + cheap creatures with strong ETBs (Restoration Angel — though that's outside Bant; Junk Winder is stock; Siege-Gang Commander likely subbed for Bloomburrow flavor) + Bloomburrow tribal payoffs + token-doublers + go-wide finishers (Elspeth Sun's Champion, Martial Coup).
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off (B4 false-positive)
- **Reasoning:** Documented in R2 (PR #523) as the original B4 FP for Family Matters. Restoration Angel + Junk Winder flagged as `infinite_mana` is a real-ish combo (blink loops creatures with ETB mana production) but requires a specific board state and a stock precon doesn't reliably assemble it. 31.1% commander synergy and 58% power with 0 GCs do not justify Optimized. Same Tuned-redundancy / Winning-combo floor surface.

### Sam, Loyal Attendant — Food and Fellowship (The Lord of the Rings / LTR)
- **Intended archetype:** 4-color Food / Halfling LOTR precon. Sam supports food-token generation and Halfling tribal.
- **Punch-up shape:** Sam + Food token-makers (Gilded Goose, Rapacious Guest, Lobelia, Defender of Bag End) + lifegain-via-food payoffs (Dawn of Hope is a stock Halfling-era inclusion) + Halfling tribal supports + token doublers. Wincon is wide Halfling swarms + food-fueled lifegain attrition.
- **Freya measured_bracket:** **4 Optimized**
- **Your verdict:** engine-off (B4 false-positive)
- **Reasoning:** 53 "win lines" on a stock food/Halfling precon is the canonical finisher-detector blowup. Rapacious Guest + Gilded Goose flagged as a 2-card combo is a consume-once Food shape — same false-positive cluster R3 PR #532 surfaced on Squirreled Away ("287 detected loops on food tokens"). 63% power + 55% commander synergy + 0 GCs + B-grade mana cannot assemble a real B4. Food-token consume-once detection is the same fix surface as the cycling-land FPs from Counter Intelligence and Blast from the Past.

---

## Group A aggregate summary

**75 decks total in `data/decks/wizards/`**; this scan covers rows 1-30 (sorted ascending). Verdicts:

| Verdict | Count | Decks |
|---|:-:|---|
| engine-correct | 19/30 | Animated Army, Aura of Courage, Bedecked Brokers, Buckle Up, Built From Scratch, Cavalry Charge, Counter Intelligence, Deadly Disguise, Death Toll, Deep Clue Sea, Divine Convocation, Draconic Rage, Dungeons of Death, Eldrazi Incursion, Elven Council, Endless Punishment, Enhanced Evolution, Everyone's Invited!, Kadena Faceless Menace |
| engine-off (B4 FP) | 9/30 | Blame Game, Blast From The Past, Cabaretti Cacophony, Corrupting Influence, Coven Counters, Creative Energy, Desert Bloom, Family Matters, Food and Fellowship |
| engine-off (B1 too cold) | 1/30 | Eternal Bargain |
| classification-mismatch | 1/30 | Breed Lethality (B2 right, "Tribal" should be "Superfriends") |

**B4 false-positive rate: 9/30 (30%)** — consistent with the R7 finding that the cumulative B4 FP rate is trending up, and well above the R1+R2 baseline of ~10%. Every group-A B4 FP reads on the same two predicates (`Tuned-redundancy floor` or `Winning-combo floor`); the fix surface PR #513 covers (gate the floor predicates on GC/true-infinite/tutor density) would correctly demote MOST of them.

**Food and Fellowship is a new B4 FP catch in this scan** — not in R1, R2, R3, R5, or R7's tables. Same shape as Squirreled Away (R3): the Food-token consume-once detector flags 50+ "win lines" off a stock food precon, and Rapacious Guest + Gilded Goose lifts the bracket via Winning-combo floor. Worth adding to the cumulative B4-FP tracker.

**Eternal Bargain (Oloro) is the only B1 case in the corpus.** Possibly an over-aggressive B1 floor on lifegain archetypes with no clean wincon — recommend a separate lifegain-archetype calibration probe.
