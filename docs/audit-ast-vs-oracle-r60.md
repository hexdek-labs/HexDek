# Audit: AST Dataset vs Scryfall Oracle Corpus (R60)

Cross-references every entry in `data/rules/ast_dataset.jsonl` against
`data/rules/oracle-cards.json` and reports per-category discrepancies.
**Findings-only — no data files were modified.**

## Headline

| Metric | Value |
|---|---:|
| AST entries scanned | 31963 |
| Oracle entries indexed | 35708 |
| Total discrepancies | 1405 |
| `missing_in_oracle` | 0 |
| `oracle_text_drift` | 840 |
| `type_line_drift` | 6 |
| `cmc_drift` | 6 |
| `mana_cost_drift` | 500 |
| `ast_keyword_hallucination` | 53 |

## Categories

### `missing_in_oracle` — 0 findings

_None detected._

### `oracle_text_drift` — 840 findings

| # | Card | Detail |
|---:|---|---|
| 1 | A-Alrund, God of the Cosmos // A-Hakka, Whispering Raven | AST≠oracle starting at byte 313: ast=flying whenever hakka, whispering raven deals combat damage … \| oracle=// flying whenever hakka, whispering raven deals combat dama… |
| 2 | A-Binding Geist // A-Spectral Binding | AST≠oracle starting at byte 193: ast=enchant creature enchanted creature gets -2/-0. if spectral … \| oracle=// enchant creature enchanted creature gets -2/-0. if spectr… |
| 3 | A-Blessed Hippogriff // A-Tyr's Blessing | AST≠oracle starting at byte 117: ast=target creature gains indestructible until end of turn. (the… \| oracle=// target creature gains indestructible until end of turn. (… |
| 4 | A-Brine Comber // A-Brinebound Gift | AST≠oracle starting at byte 218: ast=enchant creature whenever this aura enters or enchanted crea… \| oracle=// enchant creature whenever this aura enters or enchanted c… |
| 5 | A-Devoted Grafkeeper // A-Departed Soulkeeper | AST≠oracle starting at byte 232: ast=flying if departed soulkeeper would be put into a graveyard … \| oracle=// flying if departed soulkeeper would be put into a graveya… |
| 6 | A-Dorothea, Vengeful Victim // A-Dorothea's Retribution | AST≠oracle starting at byte 182: ast=enchant creature enchanted creature has "whenever this creat… \| oracle=// enchant creature enchanted creature has "whenever this cr… |
| 7 | A-Emerald Dragon // A-Dissonant Wave | AST≠oracle starting at byte 17: ast=counter target activated or triggered ability from a noncrea… \| oracle=// counter target activated or triggered ability from a nonc… |
| 8 | A-Gutter Skulker // A-Gutter Shortcut | AST≠oracle starting at byte 159: ast=enchant creature enchanted creature gets +3/+0. it can't be … \| oracle=// enchant creature enchanted creature gets +3/+0. it can't … |
| 9 | A-Lantern Bearer // A-Lanterns' Lift | AST≠oracle starting at byte 101: ast=enchant creature enchanted creature gets +1/+1 and has flyin… \| oracle=// enchant creature enchanted creature gets +1/+1 and has fl… |
| 10 | A-Mischievous Catgeist // A-Catlike Curiosity | AST≠oracle starting at byte 170: ast=enchant creature enchanted creature gets +1/+1. enchanted cr… \| oracle=// enchant creature enchanted creature gets +1/+1. enchanted… |
| 11 | A-Rowan, Scholar of Sparks // A-Will, Scholar of Frost | AST≠oracle starting at byte 377: ast=instant and sorcery spells you cast cost {1} less to cast. +… \| oracle=// instant and sorcery spells you cast cost {1} less to cast… |
| 12 | A-Young Blue Dragon // A-Sand Augury | AST≠oracle starting at byte 7: ast=scry 1, then draw a card. (then exile this card. you may cas… \| oracle=// scry 1, then draw a card. (then exile this card. you may … |
| 13 | A-Young Red Dragon // A-Bathe in Gold | AST≠oracle starting at byte 7: ast=create a treasure token. (then exile this card. you may cast… \| oracle=// create a treasure token. (then exile this card. you may c… |
| 14 | Aang, Swift Savior // Aang and La, Ocean's Fury | AST≠oracle starting at byte 200: ast=reach, trample whenever aang and la attack, put a +1/+1 coun… \| oracle=// reach, trample whenever aang and la attack, put a +1/+1 c… |
| 15 | Aang, at the Crossroads // Aang, Destined Savior | AST≠oracle starting at byte 333: ast=flying land creatures you control have vigilance. at the beg… \| oracle=// flying land creatures you control have vigilance. at the … |
| 16 | Aberrant Researcher // Perfected Form | AST≠oracle starting at byte 201: ast=flying \| oracle=// flying |
| 17 | Abigale, Poet Laureate // Heroic Stanza | AST≠oracle starting at byte 150: ast=put a +1/+1 counter on target creature. \| oracle=// put a +1/+1 counter on target creature. |
| 18 | Accident-Prone Apprentice // Amphibian Accident | AST≠oracle starting at byte 158: ast=until end of turn, target creature loses all abilities and b… \| oracle=// until end of turn, target creature loses all abilities an… |
| 19 | Accursed Witch // Infectious Curse | AST≠oracle starting at byte 190: ast=enchant player spells you cast that target enchanted player … \| oracle=// enchant player spells you cast that target enchanted play… |
| 20 | Aclazotz, Deepest Betrayal // Temple of the Dead | AST≠oracle starting at byte 314: ast=(transforms from aclazotz, deepest betrayal.) {t}: add {b}. … \| oracle=// (transforms from aclazotz, deepest betrayal.) {t}: add {b… |
| 21 | Adventurous Eater // Have a Bite | AST≠oracle starting at byte 112: ast=put a +1/+1 counter on target creature. you gain 1 life. \| oracle=// put a +1/+1 counter on target creature. you gain 1 life. |
| 22 | Aetherblade Agent // Gitaxian Mindstinger | AST≠oracle starting at byte 121: ast=deathtouch whenever this creature deals combat damage to a p… \| oracle=// deathtouch whenever this creature deals combat damage to … |
| 23 | Afflicted Deserter // Werewolf Ransacker | AST≠oracle starting at byte 92: ast=whenever this creature transforms into werewolf ransacker, y… \| oracle=// whenever this creature transforms into werewolf ransacker… |
| 24 | Agadeem's Awakening // Agadeem, the Undercrypt | AST≠oracle starting at byte 131: ast=as this land enters, you may pay 3 life. if you don't, it en… \| oracle=// as this land enters, you may pay 3 life. if you don't, it… |
| 25 | Ajani, Nacatl Pariah // Ajani, Nacatl Avenger | AST≠oracle starting at byte 210: ast=+2: put a +1/+1 counter on each cat you control. 0: create a… \| oracle=// +2: put a +1/+1 counter on each cat you control. 0: creat… |
| 26 | Akki Lavarunner // Tok-Tok, Volcano Born | AST≠oracle starting at byte 67: ast=protection from red if a red source would deal damage to a p… \| oracle=// protection from red if a red source would deal damage to … |
| 27 | Akoum Warrior // Akoum Teeth | AST≠oracle starting at byte 8: ast=this land enters tapped. {t}: add {r}. \| oracle=// this land enters tapped. {t}: add {r}. |
| 28 | Albiorix, Goose Tyrant // Wild Goose Chase | AST≠oracle starting at byte 143: ast=draw two cards, then discard two cards. create a food token. \| oracle=// draw two cards, then discard two cards. create a food tok… |
| 29 | Alive // Well | AST≠oracle starting at byte 111: ast=you gain 2 life for each creature you control. fuse (you may… \| oracle=// you gain 2 life for each creature you control. fuse (you … |
| 30 | Alluring Suitor // Deadly Dancer | AST≠oracle starting at byte 69: ast=trample when this creature transforms into deadly dancer, ad… \| oracle=// trample when this creature transforms into deadly dancer,… |
| 31 | Alrund, God of the Cosmos // Hakka, Whispering Raven | AST≠oracle starting at byte 311: ast=flying whenever hakka deals combat damage to a player, retur… \| oracle=// flying whenever hakka deals combat damage to a player, re… |
| 32 | Altar of Bhaal // Bone Offering | AST≠oracle starting at byte 139: ast=create a tapped 4/1 black skeleton creature token with menac… \| oracle=// create a tapped 4/1 black skeleton creature token with me… |
| 33 | Altar of the Wretched // Wretched Bonemass | AST≠oracle starting at byte 246: ast=wretched bonemass's power and toughness are each equal to th… \| oracle=// wretched bonemass's power and toughness are each equal to… |
| 34 | Ambitious Farmhand // Seasoned Cathar | AST≠oracle starting at byte 250: ast=lifelink \| oracle=// lifelink |
| 35 | Amethyst Dragon // Explosive Crystal | AST≠oracle starting at byte 14: ast=explosive crystal deals 4 damage divided as you choose among… \| oracle=// explosive crystal deals 4 damage divided as you choose am… |
| 36 | Animating Faerie // Bring to Life | AST≠oracle starting at byte 7: ast=target noncreature artifact you control becomes a 0/0 artifa… \| oracle=// target noncreature artifact you control becomes a 0/0 art… |
| 37 | Appeal // Authority | AST≠oracle starting at byte 113: ast=aftermath (cast this spell only from your graveyard. then ex… \| oracle=// aftermath (cast this spell only from your graveyard. then… |
| 38 | Aquatic Alchemist // Bubble Up | AST≠oracle starting at byte 109: ast=put target instant or sorcery card from your graveyard on to… \| oracle=// put target instant or sorcery card from your graveyard on… |
| 39 | Arcee, Sharpshooter // Arcee, Acrobatic Coupe | AST≠oracle starting at byte 203: ast=living metal (during your turn, this vehicle is also a creat… \| oracle=// living metal (during your turn, this vehicle is also a cr… |
| 40 | Archangel Avacyn // Avacyn, the Purifier | AST≠oracle starting at byte 223: ast=flying when this creature transforms into avacyn, the purifi… \| oracle=// flying when this creature transforms into avacyn, the pur… |
| 41 | Ardenvale Tactician // Dizzying Swoop | AST≠oracle starting at byte 7: ast=tap up to two target creatures. (then exile this card. you m… \| oracle=// tap up to two target creatures. (then exile this card. yo… |
| 42 | Arguel's Blood Fast // Temple of Aclazotz | AST≠oracle starting at byte 133: ast=(transforms from arguel's blood fast.) {t}: add {b}. {t}, sa… \| oracle=// (transforms from arguel's blood fast.) {t}: add {b}. {t},… |
| 43 | Arlinn Kord // Arlinn, Embraced by the Moon | AST≠oracle starting at byte 157: ast=+1: creatures you control get +1/+1 and gain trample until e… \| oracle=// +1: creatures you control get +1/+1 and gain trample unti… |
| 44 | Arlinn, the Pack's Hope // Arlinn, the Moon's Fury | AST≠oracle starting at byte 297: ast=nightbound (if a player casts at least two spells during the… \| oracle=// nightbound (if a player casts at least two spells during … |
| 45 | Armed // Dangerous | AST≠oracle starting at byte 138: ast=all creatures able to block target creature this turn do so.… \| oracle=// all creatures able to block target creature this turn do … |
| 46 | Ashling, Rekindled // Ashling, Rimebound | AST≠oracle starting at byte 207: ast=whenever this creature transforms into ashling, rimebound an… \| oracle=// whenever this creature transforms into ashling, rimebound… |
| 47 | Assault // Battery | AST≠oracle starting at byte 38: ast=create a 3/3 green elephant creature token. \| oracle=// create a 3/3 green elephant creature token. |
| 48 | Assure // Assemble | AST≠oracle starting at byte 94: ast=create three 2/2 green and white elf knight creature tokens … \| oracle=// create three 2/2 green and white elf knight creature toke… |
| 49 | Augmenter Pugilist // Echoing Equation | AST≠oracle starting at byte 78: ast=choose target creature you control. each other creature you … \| oracle=// choose target creature you control. each other creature y… |
| 50 | Autumnal Gloom // Ancient of the Equinox | AST≠oracle starting at byte 162: ast=trample, hexproof \| oracle=// trample, hexproof |

_… 790 more (run with `--top 0` to dump all)._

### `type_line_drift` — 6 findings

| # | Card | Detail |
|---:|---|---|
| 1 | Blink | AST=Enchantment — Saga \| oracle=Card |
| 2 | Cunning | AST=Enchantment — Aura \| oracle=Card |
| 3 | Earth Rumble | AST=Sorcery \| oracle=Card |
| 4 | Fast // Furious | AST=Instant // Sorcery \| oracle=Instant // Instant |
| 5 | Inferno | AST=Instant \| oracle=Card |
| 6 | Red Herring | AST=Artifact Creature — Clue Fish \| oracle=Creature — Fish |

### `cmc_drift` — 6 findings

| # | Card | Detail |
|---:|---|---|
| 1 | Blink | 4 (AST) vs 0 (oracle) |
| 2 | Cunning | 2 (AST) vs 0 (oracle) |
| 3 | Earth Rumble | 4 (AST) vs 0 (oracle) |
| 4 | Fast // Furious | 8 (AST) vs 5 (oracle) |
| 5 | Inferno | 7 (AST) vs 0 (oracle) |
| 6 | Pick Your Poison | 1 (AST) vs 5 (oracle) |

### `mana_cost_drift` — 500 findings

| # | Card | Detail |
|---:|---|---|
| 1 | A-Alrund, God of the Cosmos // A-Hakka, Whispering Raven | AST= \| oracle={3}{U}{U} // {1}{U} |
| 2 | A-Binding Geist // A-Spectral Binding | AST= \| oracle={1}{U} // |
| 3 | A-Brine Comber // A-Brinebound Gift | AST= \| oracle={1}{W}{U} // |
| 4 | A-Devoted Grafkeeper // A-Departed Soulkeeper | AST= \| oracle={W}{U} // |
| 5 | A-Dorothea, Vengeful Victim // A-Dorothea's Retribution | AST= \| oracle={W}{U} // |
| 6 | A-Gutter Skulker // A-Gutter Shortcut | AST= \| oracle={2}{U} // |
| 7 | A-Lantern Bearer // A-Lanterns' Lift | AST= \| oracle={U} // |
| 8 | A-Mischievous Catgeist // A-Catlike Curiosity | AST= \| oracle={1}{U} // |
| 9 | A-Rowan, Scholar of Sparks // A-Will, Scholar of Frost | AST= \| oracle={2}{R} // {4}{U} |
| 10 | Aang, Swift Savior // Aang and La, Ocean's Fury | AST= \| oracle={1}{W}{U} // |
| 11 | Aang, at the Crossroads // Aang, Destined Savior | AST= \| oracle={2}{G}{W}{U} // |
| 12 | Aberrant Researcher // Perfected Form | AST= \| oracle={3}{U} // |
| 13 | Accursed Witch // Infectious Curse | AST= \| oracle={3}{B} // |
| 14 | Aclazotz, Deepest Betrayal // Temple of the Dead | AST= \| oracle={3}{B}{B} // |
| 15 | Aetherblade Agent // Gitaxian Mindstinger | AST= \| oracle={1}{B} // |
| 16 | Afflicted Deserter // Werewolf Ransacker | AST= \| oracle={3}{R} // |
| 17 | Agadeem's Awakening // Agadeem, the Undercrypt | AST= \| oracle={X}{B}{B}{B} // |
| 18 | Ajani, Nacatl Pariah // Ajani, Nacatl Avenger | AST= \| oracle={1}{W} // |
| 19 | Akoum Warrior // Akoum Teeth | AST= \| oracle={5}{R} // |
| 20 | Alluring Suitor // Deadly Dancer | AST= \| oracle={2}{R} // |
| 21 | Alrund, God of the Cosmos // Hakka, Whispering Raven | AST= \| oracle={3}{U}{U} // {1}{U} |
| 22 | Altar of the Wretched // Wretched Bonemass | AST= \| oracle={2}{B} // |
| 23 | Ambitious Farmhand // Seasoned Cathar | AST= \| oracle={1}{W} // |
| 24 | Arcee, Sharpshooter // Arcee, Acrobatic Coupe | AST= \| oracle={1}{R}{W} // |
| 25 | Archangel Avacyn // Avacyn, the Purifier | AST= \| oracle={3}{W}{W} // |
| 26 | Arguel's Blood Fast // Temple of Aclazotz | AST= \| oracle={1}{B} // |
| 27 | Arlinn Kord // Arlinn, Embraced by the Moon | AST= \| oracle={2}{R}{G} // |
| 28 | Arlinn, the Pack's Hope // Arlinn, the Moon's Fury | AST= \| oracle={2}{R}{G} // |
| 29 | Ashling, Rekindled // Ashling, Rimebound | AST= \| oracle={1}{R} // |
| 30 | Augmenter Pugilist // Echoing Equation | AST= \| oracle={1}{G}{G} // {3}{U}{U} |
| 31 | Autumnal Gloom // Ancient of the Equinox | AST= \| oracle={2}{G} // |
| 32 | Avabruck Caretaker // Hollowhenge Huntmaster | AST= \| oracle={4}{G}{G} // |
| 33 | Avacynian Missionaries // Lunarch Inquisitors | AST= \| oracle={3}{W} // |
| 34 | Avatar Aang // Aang, Master of Elements | AST= \| oracle={R}{G}{W}{U} // |
| 35 | Ayara, Widow of the Realm // Ayara, Furnace Queen | AST= \| oracle={1}{B}{B} // |
| 36 | Azor's Gateway // Sanctum of the Sun | AST= \| oracle={2} // |
| 37 | Azusa's Many Journeys // Likeness of the Seeker | AST= \| oracle={1}{G} // |
| 38 | Baithook Angler // Hook-Haunt Drifter | AST= \| oracle={1}{U} // |
| 39 | Bala Ged Recovery // Bala Ged Sanctuary | AST= \| oracle={2}{G} // |
| 40 | Balamb Garden, SeeD Academy // Balamb Garden, Airborne | AST= \| oracle=// |
| 41 | Ballista Watcher // Ballista Wielder | AST= \| oracle={2}{R}{R} // |
| 42 | Baneblade Scoundrel // Baneclaw Marauder | AST= \| oracle={3}{B} // |
| 43 | Barkchannel Pathway // Tidechannel Pathway | AST= \| oracle=// |
| 44 | Befriending the Moths // Imperial Moth | AST= \| oracle={3}{W} // |
| 45 | Behold the Unspeakable // Vision of the Unspeakable | AST= \| oracle={3}{U}{U} // |
| 46 | Beloved Beggar // Generous Soul | AST= \| oracle={1}{W} // |
| 47 | Bereaved Survivor // Dauntless Avenger | AST= \| oracle={2}{W} // |
| 48 | Beyeen Veil // Beyeen Coast | AST= \| oracle={1}{U} // |
| 49 | Binding Geist // Spectral Binding | AST= \| oracle={2}{U} // |
| 50 | Biolume Egg // Biolume Serpent | AST= \| oracle={2}{U} // |

_… 450 more (run with `--top 0` to dump all)._

### `ast_keyword_hallucination` — 53 findings

| # | Card | Detail |
|---:|---|---|
| 1 | Abby, Merciless Soldier | AST keywords absent from oracle: partner-survivors |
| 2 | Aboroth | AST keywords absent from oracle: cumulative upkeep-put a -1/-1 counter on this creature |
| 3 | Angel's Grace | AST keywords absent from oracle: split_second |
| 4 | April O'Neil, Live on the Scene | AST keywords absent from oracle: partner-character |
| 5 | Arcbound Wanderer | AST keywords absent from oracle: modular-sunburst |
| 6 | Arcee, Sharpshooter // Arcee, Acrobatic Coupe | AST keywords absent from oracle: convert void |
| 7 | Atreus, Impulsive Son | AST keywords absent from oracle: partner-father |
| 8 | Attune with Aether | AST keywords absent from oracle: energy_get |
| 9 | Bjorna, Nightfall Alchemist | AST keywords absent from oracle: partner-friends |
| 10 | Bolshack Dragon | AST keywords absent from oracle: double strike |
| 11 | Braid of Fire | AST keywords absent from oracle: cumulative upkeep-add |
| 12 | Cunning | AST keywords absent from oracle: enchant creature |
| 13 | Donatello, the Brains | AST keywords absent from oracle: partner-character |
| 14 | Dystopia | AST keywords absent from oracle: cumulative upkeep-pay |
| 15 | Ellie, Brick Master | AST keywords absent from oracle: partner-survivors |
| 16 | Ellie, Vengeful Hunter | AST keywords absent from oracle: partner-survivors |
| 17 | Elmar, Ulvenwald Informant | AST keywords absent from oracle: partner-friends |
| 18 | Flamewar, Brash Veteran // Flamewar, Streetwise Operative | AST keywords absent from oracle: convert void |
| 19 | Gallowbraid | AST keywords absent from oracle: cumulative upkeep-pay |
| 20 | Glacial Chasm | AST keywords absent from oracle: cumulative upkeep-pay |
| 21 | Glimmer of Genius | AST keywords absent from oracle: energy_get |
| 22 | Hargilde, Kindly Runechanter | AST keywords absent from oracle: partner-friends |
| 23 | Herald of Leshrac | AST keywords absent from oracle: cumulative upkeep-gain control of a land you don't control |
| 24 | Highspire Infusion | AST keywords absent from oracle: energy_get |
| 25 | Infernal Darkness | AST keywords absent from oracle: cumulative upkeep-pay |
| 26 | Inner Sanctum | AST keywords absent from oracle: cumulative upkeep-pay |
| 27 | Jetfire, Ingenious Scientist // Jetfire, Air Guardian | AST keywords absent from oracle: convert void |
| 28 | Joel, Resolute Survivor | AST keywords absent from oracle: partner-survivors |
| 29 | Jötun Grunt | AST keywords absent from oracle: cumulative upkeep-put two cards |
| 30 | Karplusan Minotaur | AST keywords absent from oracle: cumulative upkeep-flip a coin |
| 31 | Kratos, Stoic Father | AST keywords absent from oracle: partner-father |
| 32 | Leonardo, the Balance | AST keywords absent from oracle: partner-character |
| 33 | Leyline of the Guildpact | AST keywords absent from oracle: leyline |
| 34 | Michelangelo, the Heart | AST keywords absent from oracle: partner-character |
| 35 | Morinfen | AST keywords absent from oracle: cumulative upkeep-pay |
| 36 | Othelm, Sigardian Outcast | AST keywords absent from oracle: partner-friends |
| 37 | Phyrexian Soulgorger | AST keywords absent from oracle: cumulative upkeep-sacrifice a creature |
| 38 | Polar Kraken | AST keywords absent from oracle: cumulative upkeep-sacrifice a land |
| 39 | Psychic Vortex | AST keywords absent from oracle: cumulative upkeep-draw a card |
| 40 | Red Herring | AST keywords absent from oracle: haste |
| 41 | Satya, Aetherflux Genius | AST keywords absent from oracle: energy_get |
| 42 | Sheltering Ancient | AST keywords absent from oracle: cumulative upkeep-put a +1/+1 counter on a creature an opponent controls |
| 43 | Sojourner's Companion | AST keywords absent from oracle: artifact_landcycling |
| 44 | Sophina, Spearsage Deserter | AST keywords absent from oracle: partner-friends |
| 45 | Splinter, the Mentor | AST keywords absent from oracle: partner-character |
| 46 | Static Prison | AST keywords absent from oracle: energy_get |
| 47 | Sudden Spoiling | AST keywords absent from oracle: split_second |
| 48 | Thought Lash | AST keywords absent from oracle: cumulative upkeep-exile the top card of your library |
| 49 | Tune the Narrative | AST keywords absent from oracle: energy_get |
| 50 | Vexing Sphinx | AST keywords absent from oracle: cumulative upkeep-discard a card |

_… 3 more (run with `--top 0` to dump all)._

## Reading this report

- **`missing_in_oracle`** — the AST dataset contains a card name with no
  matching entry in `oracle-cards.json`. Usually a renamed Scryfall
  entry (split/adventure faces, errata names) or a card removed from
  the corpus. Action: re-fetch oracle-cards.json then re-audit before
  re-ingesting the AST dataset.
- **`oracle_text_drift`** — the AST entry's cached oracle text disagrees
  with the current Scryfall text (after whitespace normalization). The
  snippet shows the first byte where the strings diverge — useful for
  spotting reminder-text revisions vs substantive errata.
- **`type_line_drift` / `cmc_drift` / `mana_cost_drift`** — metadata
  drift, usually a Scryfall-side normalization (case, em-dash) but
  occasionally a real type-line addition (e.g., Battle subtype) that
  the AST has not re-ingested.
- **`ast_keyword_hallucination`** — the AST contains `Keyword` nodes
  whose `name` field does not appear as a substring in the current
  oracle text. Strongest signal of parser error: an ability was
  asserted that the printed card does not have. Common false-positive
  sources are filtered via the parser-internal alias skip list.

## Reproducing this report

```
go run ./cmd/audit-ast-oracle --out docs/audit-ast-vs-oracle-r60.md --top 50
```
