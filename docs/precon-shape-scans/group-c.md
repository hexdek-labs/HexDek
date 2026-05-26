# Precon Shape Scans — Group C (Phase C, rows 61-87)

Per-deck shape scan for the third 27-deck slice of the WotC Commander precon
corpus under `data/decks/wizards/`. Sort is `ls data/decks/wizards/*.txt | sort`,
rows 61-87 inclusive. Companion to the Phase A inventory (PR #538) and the
group-A / group-B scans.

Each entry:
- **Intended archetype** — what the precon is shaped to *do* per the box / the
  commander's printed engine
- **Punch-up shape** — 3-5 cards or piece-combos that name the deck's
  highest-leverage interactions (the would-be wincons / engines / pivots)
- **Freya measured_bracket** — `measured_bracket` from `freya/<slug>.profile.json`,
  with the declared bracket in parens (precons stamp to B2 by default)
- **Verdict** — `engine-correct` (Freya's call matches reality), `off` (Freya
  is wrong in the bracket-direction sense), `classification-mismatch` (the
  archetype label is wrong but bracket-feel is fine), `unclear` (signal is
  ambiguous, deck doesn't fit a clean bucket)
- **Reasoning** — 2-3 sentences justifying the verdict

---

### Éowyn, Shieldmaiden — The Lord of the Rings Commander (LTR)

- **Intended archetype:** Boros human tribal go-wide; Éowyn buffs creatures with mana value ≤ X equal to the number of opponents and tokens she's made, leveraging the LTR human/Rohan subtheme.
- **Punch-up shape:** Forth Eorlingas! (mass token + initiative), Unbreakable Formation + Marshal's Anthem (anthem floor), Éomer + Gilraen recursion, Erkenbrand for combat ramp.
- **Freya measured_bracket:** B2 Core (declared B2). 0 GC, 24 win-lines, synergy 38%, archetype `Tribal`.
- **Verdict:** engine-correct.
- **Reasoning:** Stock LTR-era precon with no fast mana, no tutors, and a curve top-heavy on 4-CMC tokens. Freya's archetype label leans `Tribal` rather than the more accurate "boros go-wide" but the bracket call is right — this is the cleanest example of a stamped-B2 precon that *plays* B2. Win-line count of 24 is the usual "every Rohan creature counts" tribal inflation, not a real B3 signal.

### Henzie "Toolbox" Torre — Streets of New Capenna (SNC)

- **Intended archetype:** Jund (BRG) blitz / sacrifice value — Henzie gives 4+ CMC creatures blitz so they ETB-then-die-then-redraw on the same turn.
- **Punch-up shape:** Artisan of Kozilek (blitz → reanimate + free 10/9), Warstorm Surge (every ETB → damage), Woodfall Primus (blitz → free Vindicate), Giant Adephage (token doubler on death), Stalking Vengeance (sacrifice → 5 to face).
- **Freya measured_bracket:** B2 Core (declared B2). 0 GC, 10 win-lines (Warstorm Surge piece-combos dominate), synergy 38%.
- **Verdict:** engine-correct.
- **Reasoning:** Henzie precon is a textbook B2 "play big creatures, value through sacrifice" deck — no infinites, no tutors, no fast mana. The Warstorm Surge engine pieces drive the win-line count without forming a true loop. Freya correctly identifies the win-line cluster but doesn't promote it to B3 because there's no consistency vehicle behind it.

### Jirina Kudro — Commander 2020 (Ikoria)

- **Intended archetype:** Mardu (WBR) human tribal — Jirina gives +2/+0 to humans and makes 3 humans on ETB; pure go-wide tribal with Mardu wipes for defensive reset.
- **Punch-up shape:** Bastion of Remembrance (token-drain), Call the Coppercoats (mass tokens at instant speed), Nahiri the Harbinger (looter + ult), General's Enforcer (commander protection), Cleansing Nova / Citywide Bust resets.
- **Freya measured_bracket:** B2 Core (declared B2). 0 GC, 12 win-lines, synergy 14% (low — many humans don't read as "synergy" because the theme is the tribe itself).
- **Verdict:** engine-correct.
- **Reasoning:** Old-school C20 precon with a clean human-tribal floor and no upgrade signal. The 14% synergy score is the classic tribal-detection blind spot (Freya counts thematic keyword matches, not creature-type tribe membership) but the bracket call lands correctly at B2. No fast mana, no infinites, no GCs.

### Dr. Madison Li — Fallout (PIP)

- **Intended archetype:** Esper (WUB) artifact-reanimator value — Li returns an artifact creature from graveyard on ETB and at end step, leveraging the Fallout robot / scientist subtheme + recursion loops.
- **Punch-up shape:** Nuka-Cola Vending Machine + Mystic Forge (mana-engine combo), Liberty Prime Recharged (5/5 with anthem-on-attack), Wake the Past (mass artifact reanimate), Mechanized Production (alt-win), Open the Vaults (mass artifact / enchantment return).
- **Freya measured_bracket:** **B4 Optimized (declared B2). 0 GC, 16 win-lines, synergy 98%.**
- **Verdict:** **off** (false-positive B4).
- **Reasoning:** Same family as Urza's Iron Alliance / Buckle Up — Freya's tuned-redundancy / synergy floor lifts an artifact precon with high commander synergy to B4 even though there are 0 Game Changers and the listed "win-lines" are Nuka-Cola + Mystic Forge value engines, not true infinites. Mechanized Production *is* an alt-win, but it's a 7-CMC enchantment with no tutor support — far from a B4 wincon. Classified-correct as artifacts but mis-bracketed; this is the calibration surface PR #538 flagged for follow-up.

### Dogmeat, Ever Loyal — Fallout (PIP)

- **Intended archetype:** Naya (RGW) "junk Voltron" / equipment-attachment value — Dogmeat sacrifices a token to grant a permanent type counter and trigger payoffs across multiple Fallout subthemes (junk artifacts, food, settlements).
- **Punch-up shape:** Vault 21: House Gambit (sacrifice-token engine), Grim Reaper's Sprint (Voltron pump), Mister Gutsy (junk-artifact ETB), Cait Cage Brawler (combat anchor), Scavenger Grounds (graveyard hoser).
- **Freya measured_bracket:** **B1 Exhibition (declared B2). 0 GC, 4 win-lines, synergy 100%.**
- **Verdict:** off (false-negative — likely should be B2).
- **Reasoning:** 100% synergy + a coherent Voltron classification, but Freya only counts 4 win-lines and 2 finishers because the Fallout subthemes (food, junk, settlements, robots) are scattered and few cards individually trigger Freya's "finisher" heuristic. The deck is a perfectly normal B2 precon; the B1 call says "we found nothing that closes the game" which is the simulator under-rating "stuff happens" precons — the same systematic bias documented across the precon-vibes R1-R7 sweep.

### Mizzix of the Izmagnus — Commander 2015

- **Intended archetype:** Izzet (UR) spellslinger — Mizzix accumulates experience counters that make instants/sorceries cheaper; the deck is a value-tide of cantrips into a Comet Storm / Mizzix's Mastery finisher.
- **Punch-up shape:** Mizzix's Mastery (overload → free flashback every instant/sorcery in graveyard), Comet Storm (X-spell finisher with Mizzix discount), Etherium-Horn Sorcerer (cascade machine), Stolen Goods (theft engine), Mizzix herself (cost reducer).
- **Freya measured_bracket:** **B1 Exhibition (declared B2). 0 GC, 4 win-lines, synergy 73%.**
- **Verdict:** off (false-negative — should be B2).
- **Reasoning:** Mizzix's Mastery + a single Comet Storm reads to Freya as "1 finisher" because Mastery's flashback effect is structural rather than name-listed; the engine itself is the kill condition. Same Exhibition under-rating as Dogmeat — synergy reads high (73%), the commander IS the wincon, but the simulator wants a discrete finisher card to commit to a Core call. Bracket-correct deck mis-rated by one notch.

### Killian, Decisive Mentor — Strixhaven Commander (Silverquill Influence)

- **Intended archetype:** Orzhov (WB) enchantress / Aura-Voltron — Killian gives creatures targeted by your spells/abilities a 2-mana discount, supporting an Aura-attachment kill plan + Pearl-Ear value engine.
- **Punch-up shape:** Sram Senior Edificer (Aura-cantrip), Forum Filibuster, Winds of Rath (Aura-protected wipe), Herald of Amity (lifelink anchor), Pearl-Ear Imperial Advisor (sneak attack / tutor).
- **Freya measured_bracket:** **B1 Exhibition (declared B2). 0 GC, 3 win-lines, synergy 82%.**
- **Verdict:** off (false-negative — should be B2).
- **Reasoning:** Same simulator under-rating — Killian's engine *is* the gameplan (cheap creatures → swing with Auras → win), so card-level finisher counting bottoms out at "Winds of Rath" alone. The 82% synergy and `Enchantress` archetype label are both correct; the bracket should be B2 (this is a normal Strixhaven precon, not Exhibition).

### Breena, the Demagogue — Commander 2021 (Strixhaven, Silverquill Statement)

- **Intended archetype:** Orzhov (WB) angel/cleric "monarch & politics" — Breena draws cards and pumps creatures when an opponent with most life is attacked, supporting a midrange angel/cleric tribal with token payoff.
- **Punch-up shape:** Magister of Worth (will-of-the-council wipe), Felisa Fang of Silverquill (token spawner on death), Deathbringer Liege (white-black creature buff + free removal), Angel of Serenity (mass exile), Breena's draw engine.
- **Freya measured_bracket:** **B1 Exhibition (declared B2). 0 GC, 5 win-lines, synergy 51%.**
- **Verdict:** off (false-negative — should be B2).
- **Reasoning:** Third consecutive Strixhaven-era WB precon hitting the same Exhibition under-rate. Deathbringer Liege + Felisa is a real value engine; Magister of Worth and Angel of Serenity are real finishers. Freya found 5 win-lines and 3 finishers but the score-ladder still falls below the B2 threshold — same simulator bias as Dogmeat / Mizzix / Killian.

### Millicent, Restless Revenant — Innistrad: Crimson Vow (VOW, Spirit Squadron)

- **Intended archetype:** Bant (WUG) spirit tribal token-anthem — Millicent makes a 1/1 flying spirit whenever you cast a spirit or a creature you control dies; flying go-wide with spirit-anthem payoffs.
- **Punch-up shape:** Karmic Guide (mass reanimate of small spirits), Storm of Souls (mass spirit-from-graveyard), Hallowed Spiritkeeper (death → token swarm), Drogskol Captain (anthem + hexproof), Custodi Squire recursion.
- **Freya measured_bracket:** B2 Core (declared B2). 0 GC, 11 win-lines, synergy 48%.
- **Verdict:** engine-correct.
- **Reasoning:** Stock Innistrad spirit-tribal precon; no fast mana, no infinites, no Game Changers. Freya correctly identifies the spirit/recursion engine through win-line clustering (Ghostly Pilferer + Reconnaissance Mission, Storm of Souls, etc.) and lands on B2 — the deck is exactly what it claims to be.

### Hazel of the Rootbloom — Bloomburrow (BLB, Squirreled Away)

- **Intended archetype:** Golgari (BG) squirrel tribal sacrifice-tokens — Hazel produces food + squirrels; the precon stacks aristocrats payoffs around the constant token production.
- **Punch-up shape:** Ravenous Squirrel (sac outlet + scale), Chittering Witch (sac → -1/-1 spreader), Deadly Dispute / Woe Strider (Treasure/Scout sacrifice engines), Moonstone Eulogist (squirrel anthem), Beastmaster Ascension finisher.
- **Freya measured_bracket:** **B4 Optimized (declared B2). 0 GC, 298 win-lines, synergy 52%.**
- **Verdict:** **off** (false-positive B4, driven by win-line blow-up).
- **Reasoning:** 298 win-lines is the same "every sacrifice combination is a piece-combo" inflation pattern PR #523/#530 partially addressed but didn't fully resolve for sacrifice-aristocrats decks. Chittering Witch + Deadly Dispute + Woe Strider + Gilded Goose + Moonstone Eulogist combinatorially explode the pair-detection. Real bracket is B2; the lift is calibration noise, not a tournament signal.

### Aminatou, the Fateshifter — Commander 2018 (Subjective Reality)

- **Intended archetype:** Esper (WUB) blink + top-deck manipulation — Aminatou's 0-loyalty draws, +1 blinks a permanent, -6 swaps libraries; the precon supports a soft blink/value engine.
- **Punch-up shape:** Yennett Cryptic Sovereign (top-deck reveal trigger), Djinn of Wishes (X cards from top), Enigma Sphinx (cascade), Akroma's Vengeance (wipe + cycle), Aminatou's +1 blink loop.
- **Freya measured_bracket:** **B1 Exhibition (declared B2). 0 GC, 4 win-lines, synergy 54%.**
- **Verdict:** off (false-negative — should be B2; matches the precon-vibes baseline R1 finding for this exact deck).
- **Reasoning:** Aminatou C18 has been the canonical Exhibition under-rate since the R1 baseline (`precon-vibes-baseline-r60.md` row 3). Same root cause: the engine is "value through blinking + top manipulation" — no card individually counts as a finisher to Freya's heuristic, so the score ladder falls below the Core threshold. Bracket is B2 by inspection (and by WotC stamp).

### Kathril, Aspect Warper — Commander 2020 (Symbiotic Swarm)

- **Intended archetype:** Abzan (WBG) keyword-counter tribal — Kathril gives all your creatures keyword counters based on what's in your graveyard; the precon leans on self-mill + reanimate to fuel Kathril ETB.
- **Punch-up shape:** Unburial Rites + Soul of Innistrad (mass reanimate engine), Nyx Weaver (self-mill + recursion), Odric Lunarch Marshal (keyword spreader), Akroma Angel of Wrath (8 keywords in graveyard = the full Kathril bomb), Kathril ETB itself.
- **Freya measured_bracket:** B2 Core (declared B2). 0 GC, 7 win-lines, synergy 69%.
- **Verdict:** engine-correct.
- **Reasoning:** Stock C20 precon at exactly the right bracket. The Akroma/Kathril + Soul of Innistrad axis is a real engine but slow and singleton — no fast mana, no tutors, no determined infinites. Freya's 7 win-lines and 69% synergy land cleanly in the B2 band.

### Ureni of the Unwritten — Tarkir: Dragonstorm (TDM, Temur Roar)

- **Intended archetype:** Temur (URG) dragon tribal big-mana — Ureni cheats dragons into play off your top three cards when you cast a 4+ CMC spell; the precon is a stomp-pile of 5+ CMC dragons.
- **Punch-up shape:** Stormbreath Dragon (protection + monstrous), Scourge of the Throne (extra-combat dragon), Dragonlord Atarka (mass burn), Lathliss Dragon Queen (token engine on dragon ETB), Become the Avalanche / Storm's Wrath wipes.
- **Freya measured_bracket:** B2 Core (declared B2). 0 GC, 12 win-lines, synergy 15%.
- **Verdict:** engine-correct.
- **Reasoning:** Recent (TDM) dragon precon shaped exactly to the WotC B2 template — big creatures, no fast mana, a few wipes, no infinites. The 15% synergy score is the usual tribal-as-themes blind spot (dragon-tribe doesn't register as a Freya theme). Bracket call is right; the deck is the textbook "play 6-drops and swing" precon.

### Sauron, Lord of the Rings — The Lord of the Rings Commander (LTR, The Hosts of Mordor)

- **Intended archetype:** Grixis (UBR) orc/wraith tribal discard-amber — Sauron's commander spells force discard and reanimate; the LTR set's Amass + One Ring + Ring-tempts-you mechanic is the spine.
- **Punch-up shape:** The Balrog of Moria (big finisher + discard payoff), Summons of Saruman (mass orc tokens), Moria Scavenger (recursion), Goblin Dark-Dwellers (free flashback), Notion Thief (GC card on the discard plan), Cavern-Hoard Dragon finisher.
- **Freya measured_bracket:** **B4 Optimized (declared B2). 1 GC (Notion Thief), 24 win-lines, synergy 33%.**
- **Verdict:** off (false-positive B4 — likely the single GC + finisher density tripping the floor).
- **Reasoning:** Notion Thief alone shouldn't lift this to B4 — the WotC GC=1-3 ceiling rules already cap at B3 unless a winning combo is present. The lift is probably the tuned-redundancy floor (8 finishers + sometimes fast mana) firing on LTR's dense top-end. Same calibration surface as Urza's Iron Alliance. Real bracket is B2 (or generously B3 for the discard subtheme); B4 is too hot.

### Abaddon the Despoiler — Warhammer 40,000 Commander (40K, The Ruinous Powers)

- **Intended archetype:** Grixis (UBR) chaos/aspiring-champion tribal with cascade — Abaddon has cascade X (X=damage dealt by you to opponents); supports a chaos-marines + horus-heresy plan.
- **Punch-up shape:** Lord of Change (free flying body with massive cantrip body), Be'lakor the Dark Master (demon-tutor engine), Aspiring Champion (chaos undivided tribal), The Horus Heresy (mass-damage finisher), Mandate of Abaddon / Blight Grenade kills.
- **Freya measured_bracket:** B2 Core (declared B2). 0 GC, 11 win-lines, synergy 0% (themes empty — chaos isn't a Freya theme).
- **Verdict:** engine-correct.
- **Reasoning:** 40K precon at correct bracket. The 0% synergy + empty themes are the same tribal/setting blind spot (chaos / aspiring champion are 40K-specific subtypes Freya doesn't tag). Freya correctly finds 11 win-lines and several finishers but doesn't over-promote — the cascade-X engine is real but slow and inconsistent without tutors.

### Gavi, Nest Warden — Commander 2020 (Timeless Wisdom)

- **Intended archetype:** Jeskai (URW) cycling-matters tribal — Gavi makes a 2/2 dino token when you cycle for the first time each turn; the precon is the canonical cycling/Astral Drift engine.
- **Punch-up shape:** Astral Drift (cycling → blink), Curator of Mysteries (cycling rate-loose), Valiant Rescuer (cycling token), Mercurial Chemister + cycling-payoff loops, Decree of Justice / Akroma's Vengeance cycle-or-cast modal finishers.
- **Freya measured_bracket:** **B3 Upgraded (declared B2). 1 GC (Fierce Guardianship), 39 win-lines, synergy 62%.**
- **Verdict:** unclear (defensible either B2 or B3).
- **Reasoning:** Fierce Guardianship is a real GC (and cEDH-tier free counter); 39 win-lines reflect the genuine cycling-engine pair-density that PR #530 partially de-clustered for true infinites. Gavi C20 plays harder than its B2 stamp suggests — the cycling engine produces tokens and value at a quick clip. Calling this B3 is defensible; calling B2 is also defensible. Freya's B3 is a credible reading, not a clear miss.

### Omo, Queen of Vesuva — Modern Horizons 3 (MH3, Tricky Terrain)

- **Intended archetype:** Simic (UG) land-creature land-tribal — Omo gives lands creature types and supports a "lands as creatures" go-wide via Scute Swarm / Hydra Broodmaster / Avenger of Zendikar token storms.
- **Punch-up shape:** Scute Swarm (landfall token blowup), Avenger of Zendikar (mass plant tokens), Hydra Broodmaster (X-mana finisher), Jyoti Moag Ancient (animate-lands + landfall), Aggressive Biomancy (counters + tokens), Vivien Reid planeswalker support.
- **Freya measured_bracket:** B2 Core (declared B2). 0 GC, 14 win-lines, synergy 16%.
- **Verdict:** engine-correct.
- **Reasoning:** Recent (MH3) land-creature precon at the right bracket. The 16% synergy is the lands-matter blind spot (Freya's themes don't include "lands as creatures" as a first-class tag). Scute Swarm / Avenger combos are real but tutor-less; no fast mana, no infinites. B2 lands cleanly.

