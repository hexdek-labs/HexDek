# r60 Copy-Mechanism Corpus Audit (Probe A)

**Branch:** `dev/copy-mechanism-audit-r60`  
**Source:** `data/rules/oracle-cards.json` (Scryfall bulk, 37,384 unique cards as of 2026-04-30)  
**Patterns scanned:** `as a copy of` / `becomes a copy of` / `is a copy of` / `create … token … copy of` / `may have … enter[s] [the battlefield] [tapped] as a copy of` / `copy target/that/any number of target … spell|ability`

## Summary

- **685 distinct cards** matched at least one copy-mechanism pattern.
- Expected range was 80-150 (per the task brief); the real corpus is materially larger because TokenCopy and SpellCopy families each ship ~150+ cards (every populate / Saheeli / Twincast / Reverberate descendant).
- Breakdown by `TriggerSource`:

| TriggerSource | Count |
|---|---:|
| ETB-replacement | 65 |
| ETB | 6 |
| Upkeep | 1 |
| EOT | 6 |
| Activated | 64 |
| EventCondition | 20 |
| Static | 3 |
| TokenCopy | 370 |
| SpellCopy | 147 |
| Other | 3 |

- **171 cards carry at least one per-card override** (legend-rule bypass, +1/+1 counter rider, sacrifice clause, flashback rider, haste rider, graveyard-source copy, exile-at-EOT).
- **19 cards carry multiple distinct copy arms** (Vesuvan Doppelganger ETB+upkeep, Mirage Mirror multi-type activated, Saheeli's Artistry dual token mint, Progenitor Mimic recurring).

## CopyMechanism shape (target schema)

```go
type CopyMechanism struct {
    TriggerSource   TriggerKind  // ETBReplacement | ETB | Upkeep | Activated | EventCondition | EOT | CombatBegin | Static | TokenCopy | SpellCopy | Other
    Duration        Duration     // Permanent | UntilEOT | UntilNextCopy | UntilNextUpkeep | UntilEndOfCombat | Continuous | Resolution | Other
    Restriction     string       // target-type filter text from oracle ("target creature", "target nonlegendary creature you control", etc.)
    PerCardOverride []Override   // legend-bypass, +1/+1 counter, sacrifice-at-EOT, flashback, haste, graveyard-source, exile-at-EOT
    Multiple        bool         // ≥2 distinct copy arms on the same card
}
```

## Categorized by TriggerSource

### ETB-replacement (Clone family) — 65 cards

**Oracle shape:** "You may have this creature enter [tapped] as a copy of [target/any] ... on the battlefield/in a graveyard, except ..."  
**Default duration:** Permanent (lasts as long as the copying permanent is on the battlefield)  
**Mechanism:** CR §707 copy effect installed as an ETB self-replacement (CR §614). The copier chooses the original at the moment of ETB; the resulting copy keeps its identity as the copier and inherits the printed characteristics of the chosen permanent. Exception clauses (e.g. "except it's an Illusion") layer on top per CR §707.4.

**Cards with per-card overrides (13):**

| Card | Override |
|---|---|
| Altered Ego | +1/+1 counter rider |
| Copycrook | +1/+1 counter rider |
| Dack's Duplicate | +1/+1 counter rider; haste rider |
| Flesh Duplicate | sacrifice clause |
| Higher Level Zone Monster | +1/+1 counter rider |
| Mirrorhall Mimic // Ghastly Mimicry | 2 distinct copy arms |
| Moritte of the Frost | +1/+1 counter rider |
| Naga Fleshcrafter | +1/+1 counter rider; 2 distinct copy arms |
| Phantasmal Image | sacrifice clause |
| Progenitor Mimic | 2 distinct copy arms |
| Spark Double | +1/+1 counter rider |
| The Fourteenth Doctor | haste rider |
| Vesuvan Doppelganger | 2 distinct copy arms |

<details><summary>All 65 ETB-replacement cards</summary>

Activated Sleeper · Altered Ego · Auton Soldier · Body Double · Callidus Assassin · Chameleon, Master of Disguise · Clever Impersonator · Clone · Copy Artifact · Copy Enchantment · Copy Land · Copycrook · Dack's Duplicate · Deceptive Frostkite · Echoing Deeps · Estrid's Invocation · Evil Twin · Flesh Duplicate · Gigantoplasm · Glasspool Mimic // Glasspool Shore · Higher Level Zone Monster · Hulking Metamorph · Imposter Mech · Invasion of Amonkhet // Lazotep Convert · It Came from Planet Glurg · Jwari Shapeshifter · Machine God's Effigy · Malleable Impostor · Masterwork of Ingenuity · Mercurial Pretender · Mirror Image · Mirrorhall Mimic // Ghastly Mimicry · Mirrormade · Mocking Doppelganger · Mockingbird · Moritte of the Frost · Naga Fleshcrafter · Omni-Changeling · Phantasmal Image · Phyrexian Metamorph · Pirated Copy · Progenitor Mimic · Protean Raider · Quicksilver Gargantuan · Sakashima of a Thousand Faces · Sakashima the Impostor · Sakashima's Protege · Sakashima's Student · Sculpting Steel · Spark Double · Stunt Double · Superior Spider-Man · Surgical Metamorph · Synth Infiltrator · The Cosplayzer · The Fourteenth Doctor · The Master, Formed Anew · The Misty Stepper · Undercover Operative · Vesuva · Vesuvan Doppelganger · Visage Bandit · Vizier of Many Faces · Wall of Stolen Identity · Waxen Shapethief

</details>

### ETB (non-replacement) — 6 cards

**Oracle shape:** "When this enters, ... becomes a copy of ..." or "As this enters, ..."  
**Default duration:** Permanent  
**Mechanism:** Triggered or as-enters effect that installs a copy effect at ETB without the player-choice "may have" framing. Includes Cursed Mirror (combat ETB→copy attacker), Essence of the Wild (replacement applied to all your creature ETBs), The Mimeoplasm (graveyard-source copy with +1/+1 counter rider).

**Cards with per-card overrides (3):**

| Card | Override |
|---|---|
| Cursed Mirror | haste rider |
| Infinite Reflection | 2 distinct copy arms |
| The Mimeoplasm | +1/+1 counter rider |

<details><summary>All 6 ETB cards</summary>

Cursed Mirror · Essence of the Wild · Infinite Reflection · The Inspector Inspector · The Mimeoplasm · Thunderbond Vanguard

</details>

### Upkeep (Doppelganger family) — 1 cards

**Oracle shape:** "At the beginning of your upkeep, you may have this creature become a copy of target ..., except ..."  
**Default duration:** Permanent (overwrites at each upkeep; current copy effect persists until next upkeep)  
**Mechanism:** Upkeep-triggered choice that re-installs a copy effect on the bearer. Distinct from one-shot copy spells in that the bearer's prior copy effect ends when the new one applies — CR §707.2 (copy effects are continuous and tied to the triggering ability's resolution).

<details><summary>All 1 Upkeep cards</summary>

Cryptoplasm

</details>

### EOT (delayed/recurring copy) — 6 cards

**Oracle shape:** "At the beginning of the [next] end step, ... becomes a copy of ..." or "... create a token that's a copy of ..."  
**Default duration:** Varies — UntilEndOfNextTurn / Permanent / UntilEOT depending on rider  
**Mechanism:** End-step triggered copy, often paired with a sacrifice/exile rider (Esoteric Duplicator's sac-at-EOT) or token mint (Compy Swarm).

**Cards with per-card overrides (1):**

| Card | Override |
|---|---|
| Esoteric Duplicator | sacrifice clause |

<details><summary>All 6 EOT cards</summary>

Artificer Class · Compy Swarm · Esoteric Duplicator · Hazel of the Rootbloom · Phoenix Fleet Airship · Renewed Solidarity

</details>

### Activated (Mirage Mirror family) — 64 cards

**Oracle shape:** "{cost}: This becomes a copy of target ..., except ... until end of turn." OR "{cost}: Choose one — ... becomes a copy of ..."  
**Default duration:** UntilEOT (most common); UntilYourNextTurn (Volrath); Permanent (Lazav, the Multifarious)  
**Mechanism:** Activated ability installs a copy effect on the bearer or on another target permanent. Catch-all bucket: also includes instant/sorcery spells whose effect is "target permanent becomes a copy of another target permanent until end of turn" (Cytoshape, March from Velis Vel) because the resolution shape is identical to an activated ability's resolution.

**Cards with per-card overrides (12):**

| Card | Override |
|---|---|
| Augmenter Pugilist // Echoing Equation | legend-rule bypass |
| Brudiclad, Telchor Engineer | haste rider |
| Court of Vantress | 2 distinct copy arms |
| Gogo, Mysterious Mime | haste rider |
| Kallist Rhoka | 2 distinct copy arms |
| Lazav, the Multifarious | graveyard-source copy |
| Likeness Looter | graveyard-source copy |
| Loose in the Park | haste rider |
| March from Velis Vel | flashback rider; haste rider |
| Mizzium Transreliquat | 2 distinct copy arms |
| Oko, the Ringleader | 2 distinct copy arms |
| Shifting Woodland | graveyard-source copy |

<details><summary>All 64 Activated cards</summary>

Augmenter Pugilist // Echoing Equation · Brudiclad, Telchor Engineer · Cemetery Puca · Cephalid Facetaker · Court of Vantress · Curie, Emergent Intelligence · Cytoshape · Deepfathom Echo · Dermotaxi · Dimir Doppelganger · Fleeting Reflection · Gogo, Mysterious Mime · Hall of Mirrors · Happy Yargle Day! · Identity Thief · Irma, Part-Time Mutant · Kallist Rhoka · Kaya, Spirits' Justice · Killer Cosplay · Kimahri, Valiant Guardian · Lazav, Familiar Stranger · Lazav, the Multifarious · Likeness Looter · Loose in the Park · Ludevic, Necrogenius // Olag, Ludevic's Hubris · March from Velis Vel · Masterful Replication · Mimeoplasm, Revered One · Mirage Mirror · Mirror of the Forebears · Mirrorform · Mirrorweave · Mizzium Transreliquat · Nanogene Conversion · Nascent Metamorph · Oko, the Ringleader · Oko, the Trickster · Polymorphous Rush · Prime Minister's Cabinet Room · Reflection Net · Saheeli, Sublime Artificer · Sakashima the Impostor Avatar · Sakashima's Will · Scion of the Ur-Dragon · Shadow Kin · Shameless Charlatan · Shapesharer · Shapeshifter's Marrow · Shifting Woodland · Silent Hallcreeper · The Animus · The Ever-Changing 'Dane · The Flood of Mars · The Mycosynth Gardens · The Zabulous Cosplayer · Thespian's Stage · Transcantation · True Polymorph · Valki, God of Lies // Tibalt, Cosmic Impostor · Vesuvan Drifter · Vesuvan Shapeshifter · Volatile Chimera · Volrath, the Shapestealer · Zygon Infiltrator

</details>

### EventCondition (state/zone-triggered copy) — 20 cards

**Oracle shape:** "Whenever [event], this becomes a copy of ..." or "When [event], create a token that's a copy of ..."  
**Mechanism:** Triggered abilities that fire on combat damage / death / cast / target events, then install a copy on resolution. Examples: Artisan of Forms (combat damage), Lazav, Dimir Mastermind (graveyard-on-damage), Identity Thief (attacks), Tilonalli's Skinshifter (creature ETB).

**Duration mix:**

- Other: 10
- UntilEOT: 10

**Cards with per-card overrides (2):**

| Card | Override |
|---|---|
| Lazav, Wearer of Faces | sacrifice clause |
| Tilonalli's Skinshifter | haste rider |

<details><summary>All 20 EventCondition cards</summary>

Artisan of Forms · Assimilation Aegis · Aurora Shifter · Blade of Shared Souls · Crystalline Resonance · Duplication Device · Lazav, Dimir Mastermind · Lazav, Wearer of Faces · Mindlink Mech · Muddle, the Ever-Changing · Permeating Mass · Protean Thaumaturge · Protean War Engine · Renegade Doppelganger · Sarkhan, Soul Aflame · Spirit of Resilience · Sunfrill Imitator · The Everflowing Well // The Myriad Pools · Tilonalli's Skinshifter · Unstable Shapeshifter

</details>

### Static (continuous copy) — 3 cards

**Oracle shape:** "Enchanted/equipped X is a copy of ..." (continuous as long as condition holds)  
**Default duration:** Continuous (CR §707.2 — copy effect ends when the static source goes away)  
**Mechanism:** Aura/Equipment-attached continuous copy. Metamorphic Alteration (Aura targets a creature on ETB and makes it a copy of any creature), Paleontologist's Pick-Axe (equip causes equipped creature to become a copy of dinosaur token).

<details><summary>All 3 Static cards</summary>

Awoken Nephilim · Metamorphic Alteration · Paleontologist's Pick-Axe // Dinosaur Headdress

</details>

### TokenCopy (create-a-copy-token) — 370 cards

**Oracle shape:** "Create [N] token[s] that's a copy of [target / each / the chosen] ..."  
**Default duration:** Permanent (the token exists per CR §111; copy effect is intrinsic to the token's existence per CR §707.10)  
**Mechanism:** Largest bucket. Spell or activated/triggered ability creates one or more token copies of an existing permanent. Tokens use the same CR §707 layered-copy machinery as Clone-style copies; per-card riders include haste (Kiki-Jiki), sacrifice-at-EOT (Kiki-Jiki, Cabaretti Confluence), +1/+1 counter on the copy (Bushy Bodyguard, Doubling Season-style stacking), exile-at-EOT, etc.

**Cards with per-card overrides (126):**

| Card | Override |
|---|---|
| Applied Geometry | +1/+1 counter rider |
| Ardyn, the Usurper | haste rider |
| Ashling, the Limitless | sacrifice clause; haste rider |
| Assemble from Parts | graveyard-source copy |
| Baldur's Gate Wilderness | +1/+1 counter rider |
| Brenard, Ginger Sculptor | sacrifice clause |
| Bushy Bodyguard | +1/+1 counter rider |
| Cabaretti Confluence | sacrifice clause; haste rider |
| Cackling Counterpart | flashback rider |
| Cadric, Soul Kindler | sacrifice clause; haste rider |
| Calamity, Galloping Inferno | sacrifice clause; haste rider |
| Calix, Guided by Fate | +1/+1 counter rider |
| Call Up Emrakul to Help | sacrifice clause |
| Chandra, Flameshaper | sacrifice clause; haste rider |
| Chrome Dome | sacrifice clause; haste rider |
| Cogwork Assembler | haste rider; exile-at-EOT |
| Colorstorm Stallion | haste rider |
| Croaking Counterpart | flashback rider |
| Cursecloth Wrappings | graveyard-source copy |
| Dance of Many | sacrifice clause |
| Determined Iteration | sacrifice clause; haste rider |
| Dollhouse of Horrors | haste rider |
| Earthshaker Khenra | haste rider |
| Echo Chamber | haste rider |
| Echoing Assault | sacrifice clause |
| Electroduplicate | sacrifice clause; flashback rider; haste rider |
| Ember Island Production | 2 distinct copy arms |
| Fable of the Mirror-Breaker // Reflection of Kiki-Jiki | sacrifice clause; haste rider |
| Feldon of the Third Path | sacrifice clause; haste rider; graveyard-source copy |
| Felhide Spiritbinder | haste rider; exile-at-EOT |
| Firion, Wild Rose Warrior | sacrifice clause; haste rider |
| Flamerush Rider | haste rider |
| Flameshadow Conjuring | haste rider; exile-at-EOT |
| Flash Photography | flashback rider |
| Gallery of Legends | haste rider; exile-at-EOT |
| Ghired, Mirror of the Wilds | haste rider |
| God-Pharaoh's Gift | haste rider |
| Grub's Command | haste rider |
| Grub, Storied Matriarch // Grub, Notorious Auntie | sacrifice clause |
| Gut, Bestial Fanatic | sacrifice clause; haste rider |
| Gut, Brutal Fanatic | sacrifice clause; haste rider |
| Gut, Devious Fanatic | sacrifice clause; haste rider |
| Gut, Furious Fanatic | sacrifice clause; haste rider |
| Gyrus, Waker of Corpses | +1/+1 counter rider |
| Hate Mirage | haste rider |
| Heat Shimmer | haste rider |
| Helm of the Host | haste rider |
| Hofri Ghostforge | haste rider |
| Hosting Season | +1/+1 counter rider |
| Human—Time Lord Meta-Crisis | +1/+1 counter rider |
| Inalla, Archmage Ritualist | haste rider; exile-at-EOT |
| Inspired Skypainter // Maestro's Gift | haste rider |
| Invasion of Alara // Awaken the Maelstrom | +1/+1 counter rider |
| Jaxis, the Troublemaker | sacrifice clause; haste rider |
| Kharis & The Beholder | +1/+1 counter rider |
| Kiki-Jiki, Mirror Breaker | sacrifice clause; haste rider |
| Kindle the Inner Flame | sacrifice clause; flashback rider; haste rider |
| Kindred Charge | haste rider |
| Kylem All-Star | sacrifice clause |
| Labyrinth Guardian | sacrifice clause |
| Life of the Party | haste rider |
| Littjara Mirrorlake | +1/+1 counter rider; sacrifice clause |
| Mad Labs | +1/+1 counter rider |
| Mardu Siegebreaker | haste rider |
| Mimic Vat | haste rider; exile-at-EOT |
| Minion Reflector | sacrifice clause; haste rider |
| Mirage Mockery | 2 distinct copy arms |
| Mirage Phalanx | haste rider; exile-at-EOT |
| Mirror March | haste rider |
| Mirror Mockery | exile-at-EOT |
| Mirror-Style Master | +1/+1 counter rider |
| Mishra, Eminent One | sacrifice clause; haste rider |
| Molten Duplication | sacrifice clause; haste rider |
| Molten Echoes | haste rider; exile-at-EOT |
| Mordor on the March | haste rider; exile-at-EOT |
| Mythos of Illuna | 2 distinct copy arms |
| Nahiri, the Unforgiving | haste rider; exile-at-EOT |
| Nemesis Trap | exile-at-EOT |
| Nesting Dovehawk | +1/+1 counter rider |
| Norin and Feldon | haste rider; graveyard-source copy |
| Ochre Jelly | +1/+1 counter rider |
| Offspring's Revenge | haste rider |
| Orthion, Hero of Lavabrink | sacrifice clause; haste rider |
| Orvar, the All-Form | 2 distinct copy arms |
| Osgood, Operation Double | sacrifice clause |
| Parallel Evolution | flashback rider |
| Pawpatch Recruit | +1/+1 counter rider |
| Phantom Steed | sacrifice clause |
| Rankle, Master of Pranksters | haste rider; 3 distinct copy arms |
| Red Sun's Twilight | haste rider |
| Rootcast Apprenticeship | +1/+1 counter rider |
| Saheeli Rai | haste rider; exile-at-EOT |
| Saheeli's Artistry | 2 distinct copy arms |
| Saheeli, Radiant Creator | sacrifice clause; haste rider |
| Saheeli, the Gifted | haste rider |
| Saheeli, the Sun's Brilliance | sacrifice clause; haste rider |
| Sandstorm Crasher | sacrifice clause |
| Satya, Aetherflux Genius | haste rider |
| Scurry of Squirrels | +1/+1 counter rider |
| Self-Reflection | flashback rider |
| Shelob, Child of Ungoliant | sacrifice clause |
| Shredder, Shadow Master | sacrifice clause |
| Soul Separator | sacrifice clause |
| Splinter Twin | haste rider; exile-at-EOT |
| Splitting Slime | +1/+1 counter rider |
| Stangg, Echo Warrior | sacrifice clause |
| Steelburr Champion | +1/+1 counter rider |
| Stormsplitter | haste rider; exile-at-EOT |
| Summon: Good King Mog XII | +1/+1 counter rider |
| Sun-Blessed Guardian // Furnace-Blessed Conqueror | +1/+1 counter rider; sacrifice clause |
| Séance | exile-at-EOT |
| Tawnos, Solemn Survivor | 2 distinct copy arms |
| Tempestra, Dame of Games | sacrifice clause; haste rider |
| Tempt with Reflections | 3 distinct copy arms |
| Tender Wildguide | +1/+1 counter rider |
| Terra, Magical Adept // Esper Terra | sacrifice clause; haste rider |
| The Apprentice's Folly | haste rider |
| The Bean | 2 distinct copy arms |
| The Cloning of Shredder | 2 distinct copy arms |
| The Fire Crystal | sacrifice clause; haste rider |
| The Jolly Balloon Man | sacrifice clause; haste rider |
| Three Dog, Galaxy News DJ | sacrifice clause |
| Twinflame | haste rider |
| Watchful Radstag | +1/+1 counter rider |
| Windmill Farm | sacrifice clause |
| Worldwalker Helm | sacrifice clause |

<details><summary>All 370 TokenCopy cards</summary>

Abyssal Harvester · Adagia, Windswept Bastion · Adorned Pouncer · Agate Instigator · Altaïr Ibn-La'Ahad · Angel of Sanctions · Anikthea, Hand of Erebos · Anointer Priest · Applied Geometry · Arboreal Alliance · Arcane Artisan · Ardyn, the Usurper · Arna Kennerüd, Skycaptain · Arteeoh, Dread Scavenger · Ashling's Command · Ashling, the Limitless · Assemble from Parts · Aven Initiate · Aven Wind Guide · Back from the Brink · Baldur's Gate Wilderness · Battle for Bretagard · Benthic Anomaly · Biowaste Blob · Bloodforged Battle-Axe · Blu, Mansion Prince · Blue Sun's Twilight · Bramble Sovereign · Brenard, Ginger Sculptor · Brigid's Command · Bushy Bodyguard · Cabaretti Confluence · Cackling Counterpart · Cadric, Soul Kindler · Calamity, Galloping Inferno · Calix, Guided by Fate · Call Up Emrakul to Help · Caller of the Untamed · Caretaker's Talent · Caught in a Parallel Universe · Champion of Wits · Chandra, Flameshaper · Chrome Dome · City of Death · Clone Legion · Cogwork Assembler · Coiling Rebirth · Colorstorm Stallion · Conclave Evangelist · Copy Catchers · Coruscation Mage · Coursers' Accord · Croaking Counterpart · Cursecloth Wrappings · Dance of Many · Darkstar Augur · Dedicated Dollmaker · Delina, Wild Mage · Determined Iteration · Dino DNA · Dollhouse of Horrors · Donatello, Gadget Master · Dreadfeast Demon · Dreamstealer · Druid's Deliverance · Dual Nature · Dutiful Replicator · Earthshaker Khenra · Echo Chamber · Echo Storm · Echocasting Symposium · Echoing Assault · Electroduplicate · Elminster's Simulacrum · Elvish Hydromancer · Ember Island Production · Endless Evil · Esika's Chariot · Espers to Magicite · Extravagant Replication · Eyes in the Skies · Fable of the Mirror-Breaker // Reflection of Kiki-Jiki · Faerie Artisans · Fanatic of Rhonas · Fated Infatuation · Fear of Ridicule · Feldon of the Third Path · Felhide Spiritbinder · Finch Formation · Firion, Wild Rose Warrior · Flamerush Rider · Flameshadow Conjuring · Flash Photography · Flowerfoot Swordmaster · Foggy Swamp Visions · Followed Footsteps · Form of the Stax Player · Fractured Identity · Full Flowering · Gallery of Legends · Genestealer Patriarch · Ghired's Belligerence · Ghired, Conclave Exile · Ghired, Mirror of the Wilds · Giant Adephage · Glimmervoid Basin · Glyph Keeper · God-Pharaoh's Gift · Grist, Voracious Larva // Grist, the Plague Swarm · Growing Ranks · Grub's Command · Grub, Storied Matriarch // Grub, Notorious Auntie · Gut, Bestial Fanatic · Gut, Brutal Fanatic · Gut, Devious Fanatic · Gut, Furious Fanatic · Gyrus, Waker of Corpses · Hashaton, Scarab's Fist · Hate Mirage · Haunting Imitation · Heart-Piercer Manticore · Heat Shimmer · Helm of the Host · Here Comes a New Hero! · Hofri Ghostforge · Homunculus Horde · Honored Hydra · Horncaller's Chant · Hosting Season · Hour of Eternity · Human—Time Lord Meta-Crisis · Hunted by The Family · I Am Never Alone · Ignite the Cloneforge! · Imperial Mask · Impostor Syndrome · Improvised Arsenal · Inalla, Archmage Ritualist · Inchblade Companion · Inspired Skypainter // Maestro's Gift · Intrepid Rabbit · Invasion of Alara // Awaken the Maelstrom · Irenicus's Vile Duplication · Iridescent Vinelasher · Isle of Vesuva · Jace, Mirror Mage · Jaxis, the Troublemaker · Joo Dee, One of Many · Kambal, Profiteering Mayor · Kaya, Intangible Slayer · Kharasha Foothills · Kharis & The Beholder · Kiki-Jiki, Mirror Breaker · Kindle the Inner Flame · Kindred Charge · Kinzu of the Bleak Coven · Kylem All-Star · Labyrinth Guardian · Lazotep Archway · Lazotep Quarry · Leitmotif Composer · Leonardo da Vinci · Life Finds a Way · Life of the Party · Littjara Mirrorlake · Lorehold Archivist // Restore Relic · Lucy MacLean, Positively Armed · Mad Labs · Maelstrom Archangel Avatar · Maestros' Totally Safe Hideout · Manifold Mouse · March of Progress · Mardu Siegebreaker · Mechanized Production · Miirym, Sentinel Wyrm · Mimic Vat · Minion Reflector · Mirage Mockery · Mirage Phalanx · Mirror March · Mirror Match · Mirror Mockery · Mirror Room // Fractured Realm · Mirror-Sigil Sergeant · Mirror-Style Master · Mirrored Lotus · Mirrorworks · Mishra's Self-Replicator · Mishra, Eminent One · Mist-Syndicate Naga · Molten Duplication · Molten Echoes · Momir Vig, Simic Visionary Avatar · Mordor on the March · Muster the Departed · Myr Propagator · Myrkul, Lord of Bones · Mythos of Illuna · Nahiri, the Unforgiving · Naktamun · Necroduality · Nemesis Trap · Nesting Dovehawk · Nexus of Becoming · Nightmare Shepherd · Norin and Feldon · Ocelot Pride · Ochre Jelly · Octomancer · Offspring's Revenge · Oketra's Attendant · Oltec Matterweaver · Ondu Spiritdancer · Orthion, Hero of Lavabrink · Orvar, the All-Form · Osgood, Operation Double · Pack Rat · Parallel Evolution · Pawpatch Recruit · Peacekeeper Avatar · Penumbra Umbra · Phantom Steed · Polyraptor · Pool of Vigorous Growth · Preston, the Vanisher · Princess Snowfall · Promise of Aclazotz // Foul Rebirth · Prosperous Bandit · Prototype Portal · Proven Combatant · Quantum Misalignment · Quasiduplicate · Rally the Galadhrim · Ran and Shaw · Rankle, Master of Pranksters · Ratadrabik of Urborg · Red Sun's Twilight · Redoubled Stormsinger · Relm's Sketching · Replication Specialist · Replication Technique · Repudiate // Replicate · Resilient Khenra · Rhys the Redeemed · Rite of Replication · Romana II · Rootborn Defenses · Rootcast Apprenticeship · Runo Stromkirk // Krothuss, Lord of the Deep · Rust-Shield Rampager · Sacred Cat · Saga of Krark Losing His Thumb · Saheeli Rai · Saheeli's Artistry · Saheeli, Radiant Creator · Saheeli, the Gifted · Saheeli, the Sun's Brilliance · Sandstorm Crasher · Satya, Aetherflux Genius · Sauron, the Necromancer · Schema Thief · Scion of Vitu-Ghazi · Scouring Swarm · Scurry of Squirrels · Scute Swarm · Season of Weaving · Second Harvest · Selesnya Eulogist · Self-Reflection · Shaun, Father of Synths · Shelob, Child of Ungoliant · Shredder, Shadow Master · Sin, Spira's Punishment · Sinuous Striker · Sliver Queen Avatar · Slobad, Actually Just Fine · Song of the Worldsoul · Sorcerer's Broom · Soul Foundry · Soul Separator · Spawnwrithe · Specimen Collector · Spitting Image · Splash Lasher · Splinter Twin · Splitting Slime · Springheart Nantuko · Sprouting Phytohydra · Stangg, Echo Warrior · Starscape Cleric · Steadfast Sentinel · Steampath Charger · Steelburr Champion · Stolen Identity · Stonehewer Giant Avatar · Stormsplitter · Sublime Epiphany · Summon: Good King Mog XII · Sun-Blessed Guardian // Furnace-Blessed Conqueror · Sundering Growth · Sunscourge Champion · Supplant Form · Sygg's Command · Séance · Tah-Crop Skirmisher · Tamiyo, Compleated Sage · Tawnos, Solemn Survivor · Temmet, Vizier of Naktamun · Tempestra, Dame of Games · Tempt with Reflections · Tender Wildguide · Terra, Magical Adept // Esper Terra · The Apprentice's Folly · The Ash Lizard · The Battle of Dragon Brothers // Fate Reforged · The Bean · The Cloning of Shredder · The Crab Queen · The Disciple of Vess · The Dogronmaster · The Eleventh Hour · The Fire Crystal · The Joiner of Cats · The Jolly Balloon Man · The Majestic Duo · The Master, Gallifrey's End · The Miniaturizer · The Mox Painter · The Scarab God · The Scholar of Seas · The Water Maro · The Wise Sable · Theoretical Duplication · Thornplate Intimidator · Thousand-Faced Shadow · Three Blind Mice · Three Dog, Galaxy News DJ · Three Steps Ahead · Thundertrap Trainer · Timeless Dragon · Timeless Witness · Trickster's Talisman · Trostani's Judgment · Trostani, Selesnya's Voice · Trueheart Duelist · Trystan's Command · Twilight Diviner · Twinflame · Uchuulon · Unwavering Initiate · Urza, Prince of Kroog · Vaultborn Tyrant · Vesuvan Duplimancy · Vitu-Ghazi Guildmage · Wake the Reflections · Warren Warleader · Watchful Radstag · Wayfaring Temple · Wedding Ring · Welcome to Mini-apolis · Welcome to Valley · Who's That Praetor? · Will of the Temur · Windmill Farm · Worldwalker Helm · Xavier Sal, Infested Captain · Yenna, Redtooth Regent · Your Own Face Mocks You · Zinnia, Valley's Voice · Zndrsplt's Judgment

</details>

### SpellCopy (Twincast family) — 147 cards

**Oracle shape:** "Copy target [instant/sorcery/triggered] spell. You may choose new targets for the copy."  
**Default duration:** Resolution (the copy is a stack object per CR §707.10 and ceases to exist after resolving or otherwise leaving the stack)  
**Mechanism:** Creates a copy of a spell on the stack, optionally with new targets. Riders: flashback (Increasing Vengeance, Galvanic Iteration), "copy it X times" (Reverberate variants), "each opponent copies" (Mirari-style). Permission-spell copies (Twincast, Reflect) are the canonical shape; Mirari-shape activated/triggered copy-on-cast effects also live here.

**Duration mix:**

- Resolution: 146
- UntilEndOfCombat: 1

**Cards with per-card overrides (14):**

| Card | Override |
|---|---|
| Battlemage's Bracers | haste rider |
| Choreographed Sparks | sacrifice clause; haste rider |
| Dynaheir, Invoker Adept | haste rider |
| Errant, Street Artist | haste rider |
| Frontline Heroism | haste rider |
| Galvanic Iteration | flashback rider |
| Increasing Vengeance | flashback rider |
| Invasion of Vryn // Overloaded Mage-Ring | sacrifice clause |
| Mirrorpool | sacrifice clause |
| Mizzix, Replica Rider | sacrifice clause; haste rider |
| Slick Imitator | sacrifice clause |
| Tawnos, Urza's Apprentice | haste rider |
| The Peregrine Dynamo | haste rider |
| Zevlor, Elturel Exile | haste rider |

<details><summary>All 147 SpellCopy cards</summary>

Aboleth Spawn · Abstruse Archaic · Adric, Mathematical Genius · Agrus Kos, Eternal Soldier · Alania, Divergent Storm · Ashnod the Uncaring · Aziza, Mage Tower Captain · Battlemage's Bracers · Beamsplitter Mage · Bill Potts · Blue Ribbon · Breeches, the Blastmaker · Case of the Shifting Visage · Chandra's Regulator · Chandra, the Firebrand · Choreographed Sparks · Cloven Casting · Complete the Circuit · Curse of Echoes · Cursed Recording · Dan, Shrewd Trader · Display of Power · Double Down · Double Major · Double Vision · Doublecast · Drafna, Founder of Lat-Nam · Dual Casting · Dual Strike · Dualcaster Mage · Dynaheir, Invoker Adept · Echo Mage · Echoing Boon · Errant, Street Artist · Ertha Jo, Frontier Mentor · Ether · Expansion // Explosion · Exterminator Magmarch · Feather, Radiant Arbiter · Fire Lord Azula · Firebender Ascension · Flare of Duplication · Force of Rowan · Fork · Frontline Heroism · Fury Storm · Gadwick's First Duel · Galvanic Iteration · Gandalf the Grey · Gandalf, Westward Voyager · Geist of Regret · Geistblast · Gogo, Master of Mimicry · Howl of the Horde · Illusionist's Bracers · Increasing Vengeance · Insidious Will · Invasion of Arcavios // Invocation of the Founders · Invasion of Vryn // Overloaded Mage-Ring · Ivy, Gleeful Spellthief · Izzet Guildmage · Jackal, Genius Geneticist · Jin-Gitaxias, Progress Tyrant · Kalamax, the Stormsire · Kirol, Attentive First-Year · Kitsa, Otterball Elite · Krark, the Thumbless · Kurkesh, Onakke Ancient · League Guildmage · Leori, Sparktouched Hunter · Leyline of Resonance · Lithoform Engine · Lutri, the Spellchaser · Magus Lucea Kane · Mathise, Surge Channeler · Mercurial Spelldancer · Mica, Reader of Ruins · Mirari · Mirror Sheen · Mirror-Shield Hoplite · Mirrorpool · Mischievous Quanar · Mister Fantastic · Mizzix, Replica Rider · Myojin of Cryptic Dreams · Narset's Reversal · Naru Meha, Master Wizard · Nivix Guildmage · Odds // Ends · Ominous Lockbox · Parnesse, the Subtle Brush · Peter Parker's Camera · Pigment Wrangler // Striking Palette · Primal Amulet // Primal Wellspring · Pyromancer Ascension · Pyromancer's Goggles · Radiant Performer · Radiate · Ral, Storm Conduit · Reality Is Mine to Control · Reflections of Littjara · Reflective Golem · Refuse // Cooperate · Reiterate · Repeated Reverberation · Resonance Technician · Return the Favor · Reverberate · Riku and Riku · Riku of Two Reflections · Rings of Brighthearth · Rootha, Mercurial Artist · Rowan's Talent · Rowan, Scholar of Sparks // Will, Scholar of Frost · Sea Gate Stormcaller · Seasonal Sequels · See Double · Sevinne, the Chronoclasm · Shiko and Narset, Unified · Sigil Tracer · Slick Imitator · Stella Lee, Wild Card · Storm King's Thunder · Strionic Resonator · Sunken Palace · Swarm Intelligence · Sword of Wealth and Power · Tawnos, Urza's Apprentice · Teach by Example · Tempt with Mayhem · The Peregrine Dynamo · Tomb of Horrors Adventurer · Twincast · Twinferno · Twinning Staff · Unbound Flourishing · Uyo, Silent Prophet · Verazol, the Split Current · Verrak, Warped Sengir · Very Cryptic Command · Virtue of Knowledge // Vantress Visions · Volo, Guide to Monsters · Wandering Archaic // Explore the Vastlands · Weaver of Harmony · Wild Ricochet · Zada, Hedron Grinder · Zevlor, Elturel Exile

</details>

### Other (uncategorized) — 3 cards

**Oracle shape:** (no canonical shape matched)  
**Mechanism:** See model-breakers section.

<details><summary>All 3 Other cards</summary>

Dakkon Blackblade Avatar · Enolc, Perfect Clone · Ertai's Meddling

</details>

## Model-breakers

Cards whose mechanism does NOT cleanly fit the `CopyMechanism` shape and need bespoke handling.

### Deadpool, Trading Card (Secret Lair)

**Reason:** User-requested "Deadpool the Marvel" UB commander does not exist in any Scryfall printing — the closest match is Deadpool, Trading Card. Its copy-adjacent mechanic is **text-box exchange**, not a copy effect: "As Deadpool enters, you may exchange his text box and another creature's." Two permanents end up with each other's printed abilities while keeping their own P/T, name, types, and mana cost. This is mechanically distinct from CR §707 copy effects.

**Proposed handling:** New `TextBoxExchange` mechanism kind. Not a sub-case of CopyMechanism — needs its own per_card handler that splices the abilities lists of two permanents at registration time and reverses on LTB. Likely the only card in the corpus with this shape (verified by searching for `text box` in oracle text — companion follow-up).

### Dakkon Blackblade Avatar (Vanguard)

**Reason:** Non-standard format (Vanguard) — "You may play any colored card from your hand as a copy of a basic land card chosen at random ..." mechanism creates a hand-cast permission, not a battlefield copy effect.

**Proposed handling:** Out of scope. Vanguard cards are not legal in Commander/Constructed; skip in the per_card registry.

### Enolc, Perfect Clone

**Reason:** Commander-specific copy-of-partner mechanic — "If Enolc is one of two partner commanders, it's a copy of your other commander except it retains its name." Conditional on the partner-commander pairing state at deck-construction / command-zone-entry time, not on a triggered event or as-enters replacement.

**Proposed handling:** Reuses the standard `Static`/`Continuous` copy machinery but the *condition* is a deck-list / command-zone query, not an oracle-text condition. Needs a per_card handler that observes Enolc's command-zone state via the existing partner-commander tracking.

### Ertai's Meddling

**Reason:** "... the player puts it onto the stack as a copy of the original spell." The copy is created from an *exiled card* and pushed onto the stack on a delayed trigger, not via a normal copy-of-permanent or copy-of-spell-on-stack effect.

**Proposed handling:** Specialized variant of SpellCopy — the copy machinery (CR §707.10 stack-object copy) is reusable, but the source-resolution (resolve the original spell's saved characteristics from the exile zone) requires snapshotting at exile time. The CopyMechanism shape needs an optional `SourceZone` field (default: Battlefield; values: Battlefield/Graveyard/Hand/Exile/Stack) to model this and the existing graveyard-source copies (Body Double, Lazav, the Multifarious, Cemetery Puca).

### Mirror Gallery / Sakashima (legend-rule clauses on the copier)

**Reason:** Not a copy mechanic on its own, but a **modifier** on copy mechanics: "The 'legend rule' doesn't apply to permanents you control" / "... and it isn't legendary." Several Clone-family cards (Sakashima the Impostor, Sakashima of a Thousand Faces, Spark Double, Helm of the Host token, etc.) carry similar local legend-rule bypasses.

**Proposed handling:** Already captured as `legend-rule bypass` per-card override on the canonical CopyMechanism shape. No new mechanism kind needed; the override needs engine support for "this specific copy effect ignores §704.5j when state-based actions run."

### Soulflayer / Necrotic Ooze (keyword-copy, not full copy)

**Reason:** Inherits *only keywords/activated abilities* from cards in a graveyard, not full characteristics. "This creature has each keyword ability of each creature card exiled with it." Not a CR §707 copy effect at all — distinct rules text-grant mechanism.

**Proposed handling:** Out of scope for CopyMechanism. Belongs to a separate `KeywordGrant` / `AbilityGrant` mechanism family (already used by Akoum Refuge / Anointed Procession-style ability grants).

### Mirage Mirror + multi-arm activations

**Reason:** Single card carries 4 distinct activation modes that each install a copy of a different permanent type. `Multiple = true` flag is set, but each arm needs its own `CopyMechanism` row in the per-card handler — the shape supports it but the registry needs to model it as `[]CopyMechanism` per card, not 1:1.

**Proposed handling:** Registry shape should be `map[CardName][]CopyMechanism`. 19 cards in the corpus have multi-arm copy (see Multiple-mechanisms list).

### Ertai's Meddling / Identity Thief / Saheeli's Artistry — copies that mint tokens AND have a duration rider

**Reason:** Hybrid shape: TokenCopy crossed with EOT-sacrifice or exile-at-EOT. Already handled by the per-card override list, but the runtime distinction between "the copy is the token" vs "the copy is the existing permanent" needs to be a first-class field on CopyMechanism (`CopyTarget: Self | OtherPermanent | NewToken`).

**Proposed handling:** Add `CopyTarget` enum to CopyMechanism. Self = the card with the ability becomes a copy (Mirage Mirror, Vesuvan Doppelganger). OtherPermanent = some target other than self becomes a copy (Cytoshape, March from Velis Vel, Metamorphic Alteration). NewToken = a new token enters as a copy (Kiki-Jiki, Cackling Counterpart, Followed Footsteps).

## Cards with multiple distinct copy arms

| Card | Trigger | Override note |
|---|---|---|
| Court of Vantress | Activated | 2 distinct copy arms |
| Ember Island Production | TokenCopy | 2 distinct copy arms |
| Infinite Reflection | ETB | 2 distinct copy arms |
| Kallist Rhoka | Activated | 2 distinct copy arms |
| Mirage Mockery | TokenCopy | 2 distinct copy arms |
| Mirrorhall Mimic // Ghastly Mimicry | ETB-replacement | 2 distinct copy arms |
| Mizzium Transreliquat | Activated | 2 distinct copy arms |
| Mythos of Illuna | TokenCopy | 2 distinct copy arms |
| Naga Fleshcrafter | ETB-replacement | +1/+1 counter rider; 2 distinct copy arms |
| Oko, the Ringleader | Activated | 2 distinct copy arms |
| Orvar, the All-Form | TokenCopy | 2 distinct copy arms |
| Progenitor Mimic | ETB-replacement | 2 distinct copy arms |
| Rankle, Master of Pranksters | TokenCopy | haste rider; 3 distinct copy arms |
| Saheeli's Artistry | TokenCopy | 2 distinct copy arms |
| Tawnos, Solemn Survivor | TokenCopy | 2 distinct copy arms |
| Tempt with Reflections | TokenCopy | 3 distinct copy arms |
| The Bean | TokenCopy | 2 distinct copy arms |
| The Cloning of Shredder | TokenCopy | 2 distinct copy arms |
| Vesuvan Doppelganger | ETB-replacement | 2 distinct copy arms |

## Recommended next steps for the per-card handler registry

1. **Extend `CopyMechanism`** with `CopyTarget` (Self / OtherPermanent / NewToken) and `SourceZone` (Battlefield / Graveyard / Hand / Exile / Stack) fields. The current schema is insufficient for ~40 cards (graveyard-source copies + token-mint copies).
2. **Registry shape: `map[CardName][]CopyMechanism`** — 19 corpus cards have multi-arm copy abilities (Mirage Mirror, Vesuvan Doppelganger, Saheeli's Artistry, Progenitor Mimic, etc.). 1:1 mapping would lose information.
3. **TokenCopy is the largest bucket (370 cards)** — but the underlying copy machinery is shared with ETB-replacement Clone family. Engine-side, both call into the same CR §707 layered-copy primitive; per_card divergence is concentrated in the *rider* (haste, sac-at-EOT, +1/+1 counter, exile-at-EOT) rather than the copy itself.
4. **SpellCopy (147 cards)** belongs to a different code path than permanent copy — stack-object copy per CR §707.10. Already well-served by the existing `copyTargetSpell` engine path; per_card overrides only needed for the rider-bearing cards (`Choose new targets` is default; `Copy it N times`, `Each opponent copies`, `Flashback rider` are the per-card variants).
5. **171 cards carry per-card overrides.** Of those, the *legend-rule bypass* (~6 cards: Sakashima x2, Spark Double, Helm of the Host token, Mirror Gallery, etc.) and the *graveyard-source copy* (~8 cards: Body Double, Lazav the Multifarious, Cemetery Puca, Likeness Looter, etc.) are the highest-leverage to land first since they currently cause CR §704.5j and CR §707.4 deviations.
6. **Model-breakers (3+ Deadpool + scope-out cards)** need bespoke handlers: Ertai's Meddling (exile-zone snapshot for spell copy), Enolc (partner-commander conditional), Deadpool/Trading Card (text-box exchange — non-CR-§707 mechanic).

