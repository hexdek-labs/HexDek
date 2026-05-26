# Precon Shape Scans — Group B (rows 31-60)

Per-precon shape analysis for the second 30-deck slice of the WotC Commander precon corpus (`data/decks/wizards/`). Each entry pairs WotC's intended archetype with the deck's actual punch-up shape (the 3-5 cards or piece-combos a player can realistically lean on to close), Freya's `measured_bracket`, and a verdict against the engine's call.

**Verdict legend:**
- **engine-correct** — measured_bracket matches what the deck plausibly plays as
- **off** — measured_bracket reads obviously hot or cold; engine miscall
- **classification-mismatch** — bracket is plausibly right but `archetype` label is wrong
- **unclear** — defensible call, ambiguous deck shape

Data drawn from `data/decks/wizards/freya/<slug>.strategy.json` + `<slug>_freya.md` regenerated 2026-05-26.

---

### Inquisitor Greyfax — Forces of the Imperium (Warhammer 40,000)

- **Intended archetype:** Esper detain/tokens midrange. WotC's design is "tap-down control with token sub-theme using Inquisition keyword".
- **Punch-up shape:** Defenders of Humanity + For the Emperor!/Bastion Protector token-army pumps; Greyfax detain triggers stacking with anthems; Exterminatus / Fell the Mighty as catch-up wipes.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 22 detected "finisher lines" is the same anthem-pairing inflation flagged in R6 (PR #535) — the trace shows the Tuned-redundancy floor *could* fire here, but on this re-run measured stayed at B2. As a stock Esper detain deck without GC or true-infinite, B2 is the right call; the precon is a slow grind, not an Optimized assembly.

### Gonti, Canny Acquisitor — Grand Larceny (Outlaws of Thunder Junction)

- **Intended archetype:** Esper steal-from-opponent value midrange. WotC's "outlaw" tribal touched, but the engine is Gonti's "exile and cast" theft loop.
- **Punch-up shape:** Gonti commander damage + theft loop; Villainous Wealth as an X-cost finisher; theft-of-opponent-finishers as the punch-up vector (your win conditions are whatever you steal).
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** Synergy 61%, single-finisher detection (Villainous Wealth), no real combos. WotC built this as a "fun jank with stolen wincons" precon — B2 is exactly where it should land. The 21 commander damage line + a single X-cost burn is the entire win plan.

### Disa the Restless — Graveyard Overdrive (Modern Horizons 3)

- **Intended archetype:** Jund Lhurgoyf graveyard tribal. Disa makes Lhurgoyf tokens whose power scales with creature-cards in graveyards.
- **Punch-up shape:** Disa tokens (variable-power Lhurgoyfs); Necrogoyf/Pyrogoyf/Mortivore as power-based threats; Final Act + Chandra's Ignition as graveyard-fueled board wipes that double as finishers.
- **Freya measured_bracket:** 2 Core
- **Verdict:** off (engine reading is a known false-positive class)
- **Reasoning:** This re-run lands at B2, but R6 (PR #535) traced this exact deck as a B4 false-positive via the Tuned-redundancy floor and identified it as the second counter-example to PR #513's recommended fix (9% tutor density satisfies the proposed `tutorDensity ≥ 0.08` disjunct). The B2 here is the correct "vibes" call; the engine just doesn't reliably produce it.

### Brimaz, Blight of Oreskos — Growing Threat (March of the Machine)

- **Intended archetype:** Orzhov Phyrexian token aggro with incubate sub-theme. Brimaz incubates on attack; incubator transformations grow the board.
- **Punch-up shape:** Brimaz attack triggers feeding incubator tokens; Moira and Teshar graveyard-recur loops on small artifact creatures; combat damage with creature pressure as the close.
- **Freya measured_bracket:** 2 Core
- **Verdict:** classification-mismatch
- **Reasoning:** Freya labels this `artifacts` but the deck's engine is the incubate/Phyrexian token aggro plan — artifacts are the substrate (incubator tokens are artifacts) rather than the goal. Bracket itself is fine at B2; archetype label drifts because incubator counts inflate the artifact-card weighting.

### Kaalia of the Vast — Heavenly Inferno (Commander 2011)

- **Intended archetype:** Mardu Angel/Demon/Dragon tribal beats. Kaalia cheats a fatty into play on attack; you swing with the big thing.
- **Punch-up shape:** Kaalia attack trigger fetching Akroma/Dread Cacodemon/Malfegor; Comet Storm as X-burn closer; commander damage from Kaalia or a cheated-in legendary creature.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 11% synergy reading is harsh (Freya doesn't recognize Kaalia's three-creature-type tribal anchor as commander synergy — same pattern flagged on Vrondiss in R1), but the B2 bracket call is right. The 16 "win lines" are mostly cycling-land false combos (consume-once loop bug from PR #513/#523/#535) — irrelevant to the bracket conclusion but worth noting.

### Zimone, Mystery Unraveler — Jump Scare! (Duskmourn)

- **Intended archetype:** Simic lands-matter midrange with big-mana finishers. Zimone makes a Fractal whose +1/+1 counters scale with lands you control.
- **Punch-up shape:** Giggling Skitterspike + any large creature = power-based lethal; Beanstalk Giant / Ashaya / Worldspine Wurm as the large-power partners; Zimone's Fractal as a backup commander-damage threat.
- **Freya measured_bracket:** 2 Core
- **Verdict:** classification-mismatch
- **Reasoning:** Freya labels `lands matter` correctly, but the *power-based-lethal* punch-up shape (huge creature + Skitterspike's "trigger when this would deal damage to a player" or similar voltron-style finisher) suggests the deck plays more like big-mana ramp than literal lands-matter. Bracket call at B2 is fine; archetype label is the closer-to-truth of the available options but slightly misses the actual win vector.

### Cloud, Ex-SOLDIER — Limit Break (Final Fantasy)

- **Intended archetype:** Boros equipment/voltron with "monstrosity-like" power amplification. Cloud is a 4-CMC voltron commander that scales with equipment.
- **Punch-up shape:** Cloud + Buster Sword (signature artifact); Tifa, Martial Artist for extra-combat; Bastion Protector for anthem-style protection; Hellkite Tyrant as theft+damage closer.
- **Freya measured_bracket:** 2 Core
- **Verdict:** classification-mismatch
- **Reasoning:** Freya calls this `artifacts` archetype — partially right (equipment is the substrate) but the deck plays as voltron with extra-combat amplification, not artifact-payoff. B2 bracket is correct for stock product. Synergy 82% reflects the tight equipment-payoff loop.

### Saheeli, Radiant Creator — Living Energy (Aetherdrift)

- **Intended archetype:** Izzet artifact tokens + blink. Saheeli copies artifacts; the deck blinks them for value.
- **Punch-up shape:** Conjurer's Closet blink chain on Combustible Gearhulk / Pia and Kiran Nalaar / Territorial Aetherkite / Reckless Fireweaver = repeatable damage/drain; Lightning Runner for extra combats.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** Synergy 84% on `artifacts` is well-classified. The 8 detected win lines are real blink-payoff loops (not cycling false-positives), and the deck genuinely plays out as repeatable-value Izzet tokens. B2 is the right floor for a precon-as-shipped of this shape.

### Osgir, the Reconstructor — Lorehold Legacies (Commander 2021)

- **Intended archetype:** Boros artifact reanimator. Osgir makes a token copy of an exiled artifact from the graveyard.
- **Punch-up shape:** Reconstruct History exile-then-recur fuel; Osgir + Daretti, Scrap Savant graveyard-to-tokens loop; Hellkite Tyrant / Steel Hellkite as artifact-creature finishers; Jor Kadeen for anthem-style pump.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 100% synergy is the corpus high — Freya correctly identifies that nearly every card feeds Osgir's artifact-recursion plan. B2 floor is right; the chain depth (3 / max 3) and the Osgir→Daretti→Pia Nalaar loop reflect a real value engine but not a B4 closing speed.

### Quintorius, History Chaser — Lorehold Spirit (Secrets of Strixhaven)

- **Intended archetype:** Boros spirits + spellslinger graveyard recursion. Quintorius makes Spirit tokens when a noncreature card leaves your graveyard.
- **Punch-up shape:** Sevinne's Reclamation recurring Angel of Indemnity (and other targets); Moonshaker Cavalry as token-flying anthem closer; Balefire Liege for anthem damage; Karmic Guide + Reveillark as a missing-piece combo partial (only one half present per Freya).
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** Freya labels this `combo` because of the Karmic Guide + Reveillark partial — slight overreach (one missing piece doesn't make a combo deck), but the B2 bracket is right. Real shape is Spirit tribal/spellslinger midrange that grinds value off Quintorius's graveyard payoff.

### Anhelo, the Painter — Maestros Massacre (Streets of New Capenna)

- **Intended archetype:** Grixis spellslinger / storm-lite reanimator. Anhelo copies the first instant/sorcery you cast each turn for the cost of sacrificing a creature.
- **Punch-up shape:** Anhelo copy of a board wipe or X-spell for double-value; Anhelo + sacrifice outlet trades; commander damage via Anhelo's 4/3 menace body; 21 commander damage is the only explicit close path Freya found.
- **Freya measured_bracket:** 2 Core
- **Verdict:** off (too cold on shape, right on bracket)
- **Reasoning:** Freya labels archetype `storm` (defensible — Anhelo is a storm-payoff commander) but detected ZERO finisher lines and only one win line (commander damage). The deck genuinely has a real spell-doubling engine that the classifier missed; a human reading would put more than a single line of "find a thing to copy" between this and any other Grixis midrange. B2 bracket is fine; the storm-line detection is the weak spot.

### Anje Falkenrath — Merciless Rage (Commander 2019)

- **Intended archetype:** Rakdos madness/discard-matters. Anje is a discard-to-loot enabler for Vampire Madness cards.
- **Punch-up shape:** Anje loot loop + Madness payoffs; K'rrik, Son of Yawgmoth as a Phyrexian black-mana payoff threat; Nightmare Unmaking / In Garruk's Wake as catch-up wipes; Archfiend of Spite as a discard-rewarded big body.
- **Freya measured_bracket:** 2 Core
- **Verdict:** classification-mismatch
- **Reasoning:** Freya labels `midrange` — defensible but loses the Madness/discard theme that's the deck's actual engine. 18 win lines are mostly cycling-land false combos (consume-once loop bug). B2 bracket is correct; the deck plays as a slow Rakdos value pile in stock form.

### Jeleva, Nephalia's Scourge — Mind Seize (Commander 2013)

- **Intended archetype:** Grixis "cast spells from the top of opponents' decks" sub-theme around Jeleva exiling cards on attack.
- **Punch-up shape:** Jeleva exile-and-cast on combat; Decree of Pain / Starstorm / Infest as wipes; Army of the Damned as token swarm; Illusionist's Gambit / Grixis Charm for swing-back combat tricks.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 69% synergy reflects Jeleva's clear deck-feature anchor (lots of high-CMC spells to exile-and-cast). Stock 2013 precon-paced; no real combos, no GC. B2 is the right home; the 9 win lines are wipes + a couple of mass-pump finishers that read accurately.

### Riku of Two Reflections — Mirror Mastery (Commander 2011)

- **Intended archetype:** Temur copy-spells / copy-creatures value. Riku's trigger doubles creature ETBs and instant/sorcery resolutions.
- **Punch-up shape:** Riku doubling a ramp spell or an ETB creature like Acidic Slime; Garruk Wildspeaker untap-lands / make-tokens; Intet, the Dreamer commander damage as backup.
- **Freya measured_bracket:** 2 Core
- **Verdict:** off (too cold on shape, bracket reasonable)
- **Reasoning:** Only 4 win lines and 2 finishers detected — extremely sparse. R3 (PR #532) flagged Mirror Mastery as a B1 false-positive on its prior run (the deck has commander_synergy that ought to keep it above the score-ladder's "nothing detected" floor). On this regenerated run it lands at B2 which is defensible; the older B1 reading was the bug.

### Mishra, Eminent One — Mishra's Burnished Banner (The Brothers' War)

- **Intended archetype:** Mardu artifact aristocrats. Mishra makes copy tokens of artifacts when artifacts ETB; the deck sacrifices artifacts for value.
- **Punch-up shape:** Fain, the Broker + Farid + Oni-Cult Anvil sacrifice loop (3-step token cycle Freya correctly detected); Terisiare's Devastation as a reset; Workshop Elders / Traxos as artifact-creature beaters.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 90% synergy on `artifacts` is well-classified. The 3-card sacrifice loop is a real engine; B2 is the appropriate floor for a deck whose finisher list (4 entries) tops out at "artifact-creature beats with a giant Atog-style closer." No GC, no true-infinite — bracket call is right.

### Olivia, Opulent Outlaw — Most Wanted (Outlaws of Thunder Junction)

- **Intended archetype:** Mardu Vampire/outlaw aristocrats. Olivia rewards casting outlaw spells with treasure tokens.
- **Punch-up shape:** Deadly Dispute + Kamber, the Plunderer + Canyon Slough card-into-token loop; Fain, the Broker as a treasure-to-token engine; Life Insurance loop; Angrath's Marauders for damage doubling.
- **Freya measured_bracket:** 2 Core
- **Verdict:** off (engine reading is a known cold miss on this deck)
- **Reasoning:** R5 flagged Most Wanted as the inverse miss — power_pct 81, mana A, synergy 87% but measures at B2 because of the GC=0 ceiling. The 20 detected "loop" win lines are real Aristocrats value engines (Deadly Dispute + a sac outlet + Kamber), not cycling false-positives. The right call for this deck is closer to B3; B2 is the engine's well-documented "no GC, no true-infinite → can't lift" pathology.

### The Wise Mothman — Mutant Menace (Fallout)

- **Intended archetype:** Sultai proliferate/-1/-1-counters Mutant tribal. Wise Mothman gives experience counters; the deck proliferates rad counters and -1/-1 counters.
- **Punch-up shape:** Piper Wright + Tireless Tracker clue-token loop; Inexorable Tide for proliferate-based poison closes; Agent Frank Horrigan commander damage; Watchful Radstag mill-token mutual loop.
- **Freya measured_bracket:** 2 Core
- **Verdict:** classification-mismatch
- **Reasoning:** Freya labels `selfmill` — partial fit (rad counters mill rad-cards into yards) but misses the proliferate / -1/-1 axis that's central to Mutant tribal. B2 bracket is right; archetype label needs a "proliferate" or "rads" tag to capture the actual engine.

### Sevinne, the Chronoclasm — Mystic Intellect (Commander 2019)

- **Intended archetype:** Jeskai spellslinger flashback recursion. Sevinne gives a copy of flashback spells; the deck reuses instants/sorceries.
- **Punch-up shape:** Sevinne's Reclamation / Mystic Retrieval recur-from-graveyard loop; Devil's Play as X-burn closer; Dusk // Dawn as a recur-able wipe; commander damage from Sevinne's protection-when-copied trigger.
- **Freya measured_bracket:** 1 Exhibition
- **Verdict:** off (the iconic B1 false-positive flagged across R1 + R3 docs)
- **Reasoning:** Synergy 75%, 3 partial-combo notes detected (Dockside + Temur Sabertooth class), 5 win lines — multiple cross-references say "B2 with shape". The B1 measured call is the cold-floor bug pattern documented in R1 PR #513 §2 and reconfirmed in R3 PR #532's 3 additional B1 false-positives (Mirror Mastery / Eternal Bargain / Planar Portal). Same root cause: score ladder has no positive contribution to award a flashback-recursion deck.

### Lord Windgrace — Nature's Vengeance (Commander 2018)

- **Intended archetype:** Jund lands-matter / landfall midrange. Windgrace recurs lands + sacrifices them for card advantage.
- **Punch-up shape:** Windgrace +2 land discard / +0 land-from-graveyard; Whiptongue Hydra as Flying-sweeper; Gaze of Granite for X-cost wipe; Flameblast Dragon as X-burn closer; Tooth and Nail as a missing-piece big-finish partial (Freya flagged Avenger present, Tooth+Craterhoof absent).
- **Freya measured_bracket:** 2 Core
- **Verdict:** classification-mismatch
- **Reasoning:** Freya labels `counters matter` — incorrect; this is straightforward lands-matter Jund. The 45% synergy reading misses that nearly every card in the deck cares about lands going to/from yards. Bracket B2 is right; archetype label is wrong.

### Szarekh, the Silent King — Necron Dynasties (Warhammer 40,000)

- **Intended archetype:** Mono-Black Necron artifact tribal. Szarekh exiles a creature when it dies and returns it as a Necron token; the deck plays a reanimator-tokens loop.
- **Punch-up shape:** Skorpekh Lord + Dread Return loop on artifact-creature targets; Tomb Fortress for repeatable reanimation; The War in Heaven and Their Number Is Legion as token-flood finishers; Mutilate as a black-mana wipe.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 95% synergy — the second-highest in this batch, reflecting tight Necron tribal cohesion. R1 originally flagged this deck's 103 win_lines as a token-pollution data smell; this re-run reports 103 lines still but the bracket conclusion (B2) is unaffected because GC=0 keeps the ceiling firing. The win-line count is noisy but the bracket call is right.

### Kamiz, Obscura Oculus — Obscura Operation (Streets of New Capenna)

- **Intended archetype:** Esper Connive/ninjas/unblockable tribal. Kamiz rewards unblocked attackers with surveil + +1/+1 counters.
- **Punch-up shape:** Identity Thief blink chain on Tivit (infinite mana) / Misfortune Teller (infinite mana) / Aerial Extortionist / Archon of Coronation = repeatable damage/drain; Tivit + Identity Thief is a genuine infinite-mana line; Austere Command / Dusk // Dawn / Nightmare Unmaking as wipes.
- **Freya measured_bracket:** 2 Core
- **Verdict:** off (engine missing genuine combo signal)
- **Reasoning:** This deck ships with a real **infinite-mana** combo (Identity Thief + Tivit/Misfortune Teller) — Freya labels those as "BLINK COMBO: ... infinite mana" win lines but does not register them as `true_infinites`, so the GC=0 ceiling caps at B2. By WotC's own bracket rules, a stock precon with a true infinite-mana combo is at minimum B3; the engine should be lifting this one but the combo-class taxonomy isn't recognizing Tivit's "+treasure for each card type" loop as infinite.

### Jared Carthalion — Painbow (Dominaria United)

- **Intended archetype:** 5-color (WUBRG) goodstuff value midrange. Jared's monarchy + scaling-ability text is the anchor.
- **Punch-up shape:** Multicolor goodstuff finishers — Rienne, Angel of Rebirth / O-Kagachi / Illuna, Apex of Wishes as commander-damage-class threats; Time Wipe / Merciless Eviction / Iridian Maelstrom as multi-mode wipes; combat damage backed by 5-color anthem effects.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 68% synergy on `midrange` is well-classified. 0 combo notes (the only-precon-in-batch with zero) — this deck has no real combo line, just a pile of multicolor value cards. B2 is exactly right for what is essentially "WUBRG goodstuff precon."

### The Thirteenth Doctor — Paradox Power (Doctor Who)

- **Intended archetype:** Sultai time-travel/historic. Doctor + companion partner-pair (Thirteenth + Yasmin Khan) leverages historic-card payoffs.
- **Punch-up shape:** Frost Fair Lure Fish + Flaming Tyrannosaurus power-based lethal; Last Night Together extra-combat; combat damage with Doctor + Companion paired threats; 21 commander damage as backup.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 63% synergy on `midrange` is sensible. The 4 detected win lines accurately reflect the deck's actual paths to lethal (creature-power-based, extra-combat, raw combat). B2 is right for a Doctor Who precon-as-shipped.

### Ms. Bumbleflower — Peace Offering (Bloomburrow)

- **Intended archetype:** Bant Bear Pillowfort/group-hug-with-strings. Ms. Bumbleflower hands out food/clue/treasure to opponents in exchange for triggers and value.
- **Punch-up shape:** Triskaidekaphile as a draw-7-cards / 13-in-hand alt-win; Simic Ascendancy as a counter-accumulator alt-win; Twenty-Toed Toad as a tribal anthem; Bloodroot Apothecary for proliferate-flavored pressure.
- **Freya measured_bracket:** 2 Core
- **Verdict:** off (R6-documented inverse miss)
- **Reasoning:** R6 (PR #535) flagged this deck as the SECOND inverse miss after R5's Most Wanted — power_pct=81 / mana A / synergy 80% with B2 measure because the GC=0 ceiling fires. The "3 Direct win condition" detections (Triskaidekaphile / Simic Ascendancy) are real B3 alt-wins; vibes call should be B3-B4. The engine's GC=0 ceiling has a known calibration bug at the high-cross-signal end.

### Prosper, Tome-Bound — Planar Portal (Adventures in the Forgotten Realms)

- **Intended archetype:** Rakdos exile-top-of-library Treasure ramp. Prosper exiles top card on attack, lets you cast from exile, generates treasure.
- **Punch-up shape:** Lorcan, Warlock Collector + Fiendlash power-based lethal; Dream Pillager attack-trigger value; Lorcan as a commander-damage threat alongside Prosper.
- **Freya measured_bracket:** 2 Core
- **Verdict:** off (R3-documented B1 false-positive on the prior run, now corrected to B2)
- **Reasoning:** R3 (PR #532) flagged this deck as a B1 false-positive with power_pct 73, synergy 87%, mana A — the strongest power signal in R3 measuring at B1. This re-run hits B2 which is the correct floor. The prior B1 reading was the score-ladder-too-cold pathology that R3 + R6 docs both flagged as needing the same kind of fix as the B4 over-firing case.

### Meren of Clan Nel Toth — Plunder the Graves (Commander 2015)

- **Intended archetype:** Golgari graveyard reanimator. Meren's experience-counter recursion on creature deaths is the engine.
- **Punch-up shape:** Pathbreaker Ibex / Overwhelming Stampede / Eldrazi Monument as mass-pump finishers; Mycoloth + Eldrazi Monument token-army-then-pump; Terastodon + Jarad sacrifice-for-damage; Verdant Force / Eater of Hope as Meren-reanimation targets.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 39% synergy is low for a deck whose every card feeds Meren's plan — the synergy scorer likely undervalues "stuff dies" graveyard-feeder cards relative to direct payoffs. Bracket is correct at B2 though; the 15 win lines accurately reflect the deck's "anthem the army, swing through" finish pattern. Stock 2015 precon paced.

### Ghired, Conclave Exile — Primal Genesis (Commander 2019)

- **Intended archetype:** Naya populate tribal. Ghired makes a Rhino token on attack, then the deck doubles tokens via populate.
- **Punch-up shape:** Heart-Piercer Manticore + any large creature = power-based lethal (recurring); Doomed Artisan / Wayfaring Temple / Desolation Twin as token-makers; Giant Adephage / Tectonic Hellion as power-pressure threats.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 70% synergy on `midrange` is well-classified (though "populate tribal" would be more precise than midrange). The 10 win lines are mostly real Manticore-pair lethal lines + a wipe; bracket call at B2 is right.

### Adrix and Nev, Twincasters — Quantum Quandrix (Commander 2021)

- **Intended archetype:** Simic Fractals / token doubling. Adrix and Nev double every token you make.
- **Punch-up shape:** Garruk, Primal Hunter + Return of the Wildspeaker double-token-then-mass-pump; Hydra Broodmaster X-cost tokens; Guardian Augmenter for token anthem; Adrix and Nev commander-damage as a backup.
- **Freya measured_bracket:** 2 Core
- **Verdict:** engine-correct
- **Reasoning:** 53% synergy reading on `counters matter` is the wrong sub-label (this is tokens-matter, not counters), but bracket is right. The 12 win lines accurately catch the token-doubling combos. B2 is the appropriate floor for the stock precon-as-shipped.

### Stella Lee, Wild Card — Quick Draw (Outlaws of Thunder Junction)

- **Intended archetype:** Izzet spellslinger / clue-tokens midrange. Stella Lee rewards casting your third+ instant/sorcery each turn with copies.
- **Punch-up shape:** Stella Lee third-spell copy chain; Pyretic Charge as a mass-pump finisher; Shark Typhoon + Big Score draw cycle; 21 commander damage from Stella Lee's combat profile.
- **Freya measured_bracket:** 2 Core
- **Verdict:** off (engine too cold on shape)
- **Reasoning:** Synergy 90% (high) but only 4 win lines + 1 finisher detected — the deck's actual win plan (Stella Lee's "copy the third spell" chain into burn or token wipes) is invisible to the win-line classifier. B2 bracket is correct for stock product; the missing win-line detection is the classifier weakness.

### Neyali, Suns' Vanguard — Rebellion Rising (Phyrexia: All Will Be One)

- **Intended archetype:** Naya Rebel/token aggro with Phyrexian Toxic sub-theme. Neyali grants double-strike and impulse-draw on token attacks.
- **Punch-up shape:** Call the Coppercoats + Goldnight Commander token-army + mass-pump; Finale of Glory X-cost tokens; Heroic Reinforcements anthem-on-ETB; Castle Embereth land-anthem; Jor Kadeen Mardu artifact-anthem.
- **Freya measured_bracket:** 2 Core
- **Verdict:** classification-mismatch
- **Reasoning:** Freya labels `artifacts` — incorrect; this is Soldier/Rebel token aggro with the Phyrexian Toxic sub-theme. Bracket B2 is right (23 anthem-pair win lines is real token-army shape). Archetype tag is wrong because the deck has SOME Phyrexian artifacts but isn't artifacts-payoff at all.
