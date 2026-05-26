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
