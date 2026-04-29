# Combo Detections

Static-analysis scan of the card pool for common infinite / engine patterns.
Heuristic, NOT a solver — every entry is a candidate for human review.

## Summary

- Cards scanned: **31,639**
- Parse errors (skipped): **0**
- 2-card pair candidates: **375,805**

| Category | Count |
|---|---:|
| Mana-positive engines (infinite-mana candidates) | 1,239 |
| Untap triggers | 213 |
| Untap-on-activation (pair fuel) | 58 |
| Storm engines (trigger-per-cast) | 1,508 |
| Iterative draw engines | 280 |
| Doublers (replacement effects on resources) | 37 |

## Mana-positive engines (infinite-mana candidates) (1,239)

| Card | Confidence | Reason |
|---|---:|---|
| A-Canopy Tactician | 0.85 | {T} (cmc 0) → 3 mana |
| Ancient Spring | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Ancient Tomb | 0.85 | {T} (cmc 0) → 2 mana |
| Apprentice Wizard | 0.85 | {u} + {T} (cmc 1) → 3 mana |
| Arid Archway | 0.85 | {T} (cmc 0) → 2 mana |
| Arixmethes, Slumbering Isle | 0.85 | {T} (cmc 0) → 2 mana |
| Ashnod's Altar | 0.85 | sac creature (cmc 0) → 2 mana |
| Azorius Chancery | 0.85 | {T} (cmc 0) → 2 mana |
| Balduvian Trading Post | 0.85 | {T} (cmc 0) → 2 mana |
| Basal Sliver | 0.85 | free (cmc 0) → 2 mana |
| Basal Thrull | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Basalt Monolith | 0.85 | {T} (cmc 0) → 3 mana |
| Blood Vassal | 0.85 | sac this (cmc 0) → 2 mana |
| Bog Witch | 0.85 | {b} + {T} + discard 1 (cmc 1) → 3 mana |
| Boros Garrison | 0.85 | {T} (cmc 0) → 2 mana |
| Brass Infiniscope | 0.85 | {T} (cmc 0) → 2 mana |
| Canopy Tactician | 0.85 | {T} (cmc 0) → 3 mana |
| Careful Cultivation | 0.85 | free (cmc 0) → 2 mana |
| Catalyst Elemental | 0.85 | sac this (cmc 0) → 2 mana |
| Chromatic Orrery | 0.85 | {T} (cmc 0) → 5 mana |
| Circle of Elders | 0.85 | free (cmc 0) → 3 mana |
| Codsworth, Handy Helper | 0.85 | {T} (cmc 0) → 2 mana |
| Composite Golem | 0.85 | sac this (cmc 0) → 5 mana |
| Coral Atoll | 0.85 | {T} (cmc 0) → 2 mana |
| Cormela, Glamour Thief | 0.85 | {1} + {T} (cmc 1) → 3 mana |
| Crosis's Attendant | 0.85 | {1} + sac this (cmc 1) → 3 mana |
| Cryptic Trilobite | 0.85 | free (cmc 0) → 2 mana |
| Crystal Vein | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Dalakos, Crafter of Wonders | 0.85 | {T} (cmc 0) → 2 mana |
| Darigaaz's Attendant | 0.85 | {1} + sac this (cmc 1) → 3 mana |
| Dimir Aqueduct | 0.85 | {T} (cmc 0) → 2 mana |
| Dormant Volcano | 0.85 | {T} (cmc 0) → 2 mana |
| Dreamstone Hedron | 0.85 | {T} (cmc 0) → 3 mana |
| Dromar's Attendant | 0.85 | {1} + sac this (cmc 1) → 3 mana |
| Dwarven Ruins | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Ebon Stronghold | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Eldrazi Temple | 0.85 | {T} (cmc 0) → 2 mana |
| Elfhame Druid | 0.85 | {T} (cmc 0) → 2 mana |
| Elvish Aberration | 0.85 | {T} (cmc 0) → 3 mana |
| Endrider Catalyzer | 0.85 | free (cmc 0) → 2 mana |
| Evendo Brushrazer | 0.85 | {T} + sac land (cmc 0) → 2 mana |
| Everglades | 0.85 | {T} (cmc 0) → 2 mana |
| Fanatic of Rhonas | 0.85 | free (cmc 0) → 4 mana |
| Find the Path | 0.85 | free (cmc 0) → 2 mana |
| Fyndhorn Elder | 0.85 | {T} (cmc 0) → 2 mana |
| Gaea's Touch | 0.85 | sac this (cmc 0) → 2 mana |
| Generator Servant | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Geothermal Crevice | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Glade of the Pump Spells | 0.85 | {T} (cmc 0) → 2 mana |
| Golgari Rot Farm | 0.85 | {T} (cmc 0) → 2 mana |
| Grand Architect | 0.85 | free (cmc 0) → 2 mana |
| Greenweaver Druid | 0.85 | {T} (cmc 0) → 2 mana |
| Grim Monolith | 0.85 | {T} (cmc 0) → 3 mana |
| Grinning Ignus | 0.85 | {r} (cmc 1) → 3 mana |
| Gruul Turf | 0.85 | {T} (cmc 0) → 2 mana |
| Guildless Commons | 0.85 | {T} (cmc 0) → 2 mana |
| Gyre Engineer | 0.85 | {T} (cmc 0) → 2 mana |
| Hargilde, Kindly Runechanter | 0.85 | {T} (cmc 0) → 2 mana |
| Havenwood Battleground | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Hedron Archive | 0.85 | {T} (cmc 0) → 2 mana |
| Heritage Druid | 0.85 | free (cmc 0) → 3 mana |
| Hickory Woodlot | 0.85 | {T} (cmc 0) → 2 mana |
| Ichor Elixir | 0.85 | {T} (cmc 0) → 2 mana |
| Irrigation Ditch | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Izzet Boilerworks | 0.85 | {T} (cmc 0) → 2 mana |
| James, Wandering Dad // Follow Him | 0.85 | {T} (cmc 0) → 2 mana |
| Jasmine Boreal of the Seven | 0.85 | {T} (cmc 0) → 2 mana |
| Jegantha, the Wellspring | 0.85 | {T} (cmc 0) → 5 mana |
| Joraga Treespeaker | 0.85 | {T} (cmc 0) → 2 mana |
| Jungle Basin | 0.85 | {T} (cmc 0) → 2 mana |
| Karoo | 0.85 | {T} (cmc 0) → 2 mana |
| Knotvine Mystic | 0.85 | {1} + {T} (cmc 1) → 3 mana |
| Kozilek's Channeler | 0.85 | {T} (cmc 0) → 2 mana |
| Krark-Clan Ironworks | 0.85 | sac artifact (cmc 0) → 2 mana |
| Krark-Clan Stoker | 0.85 | {T} + sac artifact (cmc 0) → 2 mana |
| Lake of the Dead | 0.85 | {T} + sac swamp (cmc 0) → 4 mana |
| Lavabrink Floodgates | 0.85 | {T} (cmc 0) → 2 mana |
| Lavinia, Foil to Conspiracy | 0.85 | {T} (cmc 0) → 2 mana |
| Llanowar Tribe | 0.85 | {T} (cmc 0) → 3 mana |
| Lys Alana Dignitary | 0.85 | {T} (cmc 0) → 2 mana |
| Magus Lucea Kane | 0.85 | free (cmc 0) → 2 mana |
| Mana Crypt | 0.85 | {T} (cmc 0) → 2 mana |
| Mana Vault | 0.85 | {T} (cmc 0) → 3 mana |
| Master of Dark Rites | 0.85 | {T} + sac another (cmc 0) → 3 mana |
| Mishra's Workshop | 0.85 | {T} (cmc 0) → 3 mana |
| Morgue Toad | 0.85 | sac this (cmc 0) → 2 mana |
| Muraganda Raceway | 0.85 | free (cmc 0) → 2 mana |
| Myr Reservoir | 0.85 | {T} (cmc 0) → 2 mana |
| Nantuko Elder | 0.85 | {T} (cmc 0) → 2 mana |
| Omen Hawker | 0.85 | {T} (cmc 0) → 2 mana |
| Orochi Merge-Keeper | 0.85 | free (cmc 0) → 2 mana |
| Orzhov Basilica | 0.85 | {T} (cmc 0) → 2 mana |
| Overeager Apprentice | 0.85 | sac this + discard 1 (cmc 0) → 3 mana |
| Palladium Myr | 0.85 | {T} (cmc 0) → 2 mana |
| Peat Bog | 0.85 | {T} (cmc 0) → 2 mana |
| Phyrexian Tower | 0.85 | {T} + sac creature (cmc 0) → 2 mana |
| Pit Automaton | 0.85 | {T} (cmc 0) → 2 mana |
| Planeswalkerificate | 0.85 | free (cmc 0) → 2 mana |
| Rakdos Carnarium | 0.85 | {T} (cmc 0) → 2 mana |
| Ramos, Dragon Engine | 0.85 | free (cmc 0) → 10 mana |
| Reckless Barbarian | 0.85 | sac this (cmc 0) → 2 mana |
| Remote Farm | 0.85 | {T} (cmc 0) → 2 mana |
| Renowned Weaponsmith | 0.85 | {T} (cmc 0) → 2 mana |
| Ring of the Lucii | 0.85 | {T} (cmc 0) → 2 mana |
| Rith's Attendant | 0.85 | {1} + sac this (cmc 1) → 3 mana |
| Rosheen Meanderer | 0.85 | {T} (cmc 0) → 4 mana |
| Ruins of Trokair | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Runaway Steam-Kin | 0.85 | free (cmc 0) → 3 mana |
| Runecarved Obelisk | 0.85 | {T} (cmc 0) → 2 mana |
| Sachi, Daughter of Seshiro | 0.85 | free (cmc 0) → 2 mana |
| Sandstone Needle | 0.85 | {T} (cmc 0) → 2 mana |
| Saprazzan Skerry | 0.85 | {T} (cmc 0) → 2 mana |
| Satyr Hedonist | 0.85 | {r} + sac this (cmc 1) → 3 mana |
| Scorched Ruins | 0.85 | {T} (cmc 0) → 4 mana |
| Scorned Villager // Moonscarred Werewolf | 0.85 | {T} (cmc 0) → 2 mana |
| Selesnya Sanctuary | 0.85 | {T} (cmc 0) → 2 mana |
| Shrine of the Forsaken Gods | 0.85 | {T} (cmc 0) → 2 mana |
| Simic Growth Chamber | 0.85 | {T} (cmc 0) → 2 mana |
| Sisay's Ring | 0.85 | {T} (cmc 0) → 2 mana |
| Snapping Voidcraw | 0.85 | {T} (cmc 0) → 2 mana |
| Sol Ring | 0.85 | {T} (cmc 0) → 2 mana |
| Sol Talisman | 0.85 | {T} (cmc 0) → 2 mana |
| Soldevi Excavations | 0.85 | {T} (cmc 0) → 2 mana |
| Soldevi Machinist | 0.85 | {T} (cmc 0) → 2 mana |
| Steelswarm Operator | 0.85 | {T} (cmc 0) → 2 mana |
| Stonespeaker Crystal | 0.85 | {T} (cmc 0) → 2 mana |
| Sulfur Vent | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Sunastian Falconer | 0.85 | {T} (cmc 0) → 2 mana |
| Svyelunite Temple | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Tablet of Discovery | 0.85 | {T} (cmc 0) → 2 mana |
| Teferi's Isle | 0.85 | {T} (cmc 0) → 2 mana |
| Temple of the False God | 0.85 | {T} (cmc 0) → 2 mana |
| The Enigma Jewel // Locus of Enlightenment | 0.85 | {T} (cmc 0) → 2 mana |
| The Eternity Elevator | 0.85 | {T} (cmc 0) → 3 mana |
| The Great Henge | 0.85 | {T} (cmc 0) → 2 mana |
| The Mightstone and Weakstone | 0.85 | {T} (cmc 0) → 2 mana |
| Thran Dynamo | 0.85 | {T} (cmc 0) → 3 mana |
| Timeless Lotus | 0.85 | {T} (cmc 0) → 5 mana |
| Tin Street Gossip | 0.85 | {T} (cmc 0) → 2 mana |
| Tinder Farm | 0.85 | {T} + sac this (cmc 0) → 2 mana |
| Tinder Wall | 0.85 | sac this (cmc 0) → 2 mana |
| Transmogrant Altar | 0.85 | {b} + {T} + sac creature (cmc 1) → 3 mana |
| Treva's Attendant | 0.85 | {1} + sac this (cmc 1) → 3 mana |
| Troyan, Gutsy Explorer | 0.85 | {T} (cmc 0) → 2 mana |
| Ulvenwald Captive // Ulvenwald Abomination | 0.85 | {T} (cmc 0) → 2 mana |
| Undermountain Adventurer | 0.85 | {T} (cmc 0) → 2 mana |
| Untaidake, the Cloud Keeper | 0.85 | {T} + pay 2 life (cmc 0) → 2 mana |
| Ur-Golem's Eye | 0.85 | {T} (cmc 0) → 2 mana |
| Vessel of Volatility | 0.85 | {1}{r} + sac this (cmc 2) → 4 mana |
| Weather Maker | 0.85 | {T} (cmc 0) → 2 mana |
| Weaver of Currents | 0.85 | {T} (cmc 0) → 2 mana |
| Whisperer of the Wilds | 0.85 | free (cmc 0) → 2 mana |
| Witch Engine | 0.85 | {T} (cmc 0) → 4 mana |
| Worn Powerstone | 0.85 | {T} (cmc 0) → 2 mana |
| Yawgmoth's Day Planner | 0.85 | {T} + pay 2 life (cmc 0) → 2 mana |
| A Realm Reborn | 0.60 | free (cmc 0) → 1 mana |
| A-Base Camp | 0.60 | {T} (cmc 0) → 1 mana |
| A-Bretagard Stronghold | 0.60 | {T} (cmc 0) → 1 mana |
| A-Carnelian Orb of Dragonkind | 0.60 | {T} (cmc 0) → 1 mana |
| A-Dungeon Descent | 0.60 | {T} (cmc 0) → 1 mana |
| A-Hall of Tagsin | 0.60 | {T} (cmc 0) → 1 mana |
| A-Jade Orb of Dragonkind | 0.60 | {T} (cmc 0) → 1 mana |
| A-Lantern of Revealing | 0.60 | {T} (cmc 0) → 1 mana |
| A-Lapis Orb of Dragonkind | 0.60 | {T} (cmc 0) → 1 mana |
| A-Llanowar Loamspeaker | 0.60 | {T} (cmc 0) → 1 mana |
| A-Skemfar Elderhall | 0.60 | {T} (cmc 0) → 1 mana |
| A-Spell Satchel | 0.60 | {T} (cmc 0) → 1 mana |
| Abandoned Air Temple | 0.60 | {T} (cmc 0) → 1 mana |
| Abandoned Outpost | 0.60 | {T} (cmc 0) → 1 mana |
| Abstergo Entertainment | 0.60 | {T} (cmc 0) → 1 mana |
| Abstruse Interference | 0.60 | free (cmc 0) → 1 mana |
| Abundant Countryside | 0.60 | {T} (cmc 0) → 1 mana |
| Abundant Growth | 0.60 | free (cmc 0) → 1 mana |
| Academy Ruins | 0.60 | {T} (cmc 0) → 1 mana |
| Access Tunnel | 0.60 | {T} (cmc 0) → 1 mana |
| Accomplished Alchemist | 0.60 | {T} (cmc 0) → 1 mana |
| Accursed Duneyard | 0.60 | {T} (cmc 0) → 1 mana |
| Aclazotz, Deepest Betrayal // Temple of the Dead | 0.60 | {T} (cmc 0) → 1 mana |
| Adarkar Wastes | 0.60 | {T} (cmc 0) → 1 mana |
| Adherent's Heirloom | 0.60 | {T} (cmc 0) → 1 mana |
| Adventurer's Inn | 0.60 | {T} (cmc 0) → 1 mana |
| Adverse Conditions | 0.60 | free (cmc 0) → 1 mana |
| Aether Hub | 0.60 | {T} (cmc 0) → 1 mana |
| Aetheric Amplifier | 0.60 | {T} (cmc 0) → 1 mana |
| Agility Bobblehead | 0.60 | {T} (cmc 0) → 1 mana |
| Agna Qel'a | 0.60 | {T} (cmc 0) → 1 mana |
| Akoum Warrior // Akoum Teeth | 0.60 | {T} (cmc 0) → 1 mana |
| Alchemist's Refuge | 0.60 | {T} (cmc 0) → 1 mana |
| All-Fates Scroll | 0.60 | {T} (cmc 0) → 1 mana |
| Alloy Myr | 0.60 | {T} (cmc 0) → 1 mana |
| Ally Encampment | 0.60 | {T} (cmc 0) → 1 mana |
| Alpine Moon | 0.60 | free (cmc 0) → 1 mana |
| Amonkhet Raceway | 0.60 | {T} (cmc 0) → 1 mana |
| An-Havva Township | 0.60 | {T} (cmc 0) → 1 mana |
| Ancient Cornucopia | 0.60 | {T} (cmc 0) → 1 mana |
| Ancient Den | 0.60 | {T} (cmc 0) → 1 mana |
| Ancient Ziggurat | 0.60 | {T} (cmc 0) → 1 mana |
| Animal Attendant | 0.60 | {T} (cmc 0) → 1 mana |
| Animal Sanctuary | 0.60 | {T} (cmc 0) → 1 mana |
| Arbor Adherent | 0.60 | {T} (cmc 0) → 1 mana |
| … | | +1039 more in JSON |

## Untap triggers (213)

| Card | Confidence | Reason |
|---|---:|---|
| A-Raiyuu, Storm's Edge | 0.80 | triggered on attack_alone → untap |
| Akki Battle Squad | 0.80 | triggered on group_attack → untap |
| Anzrag, the Quake-Mole | 0.80 | triggered on becomes_blocked → untap |
| Awakening | 0.80 | triggered on phase → untap |
| Azusa's Many Journeys // Likeness of the Seeker | 0.80 | triggered on becomes_blocked → untap |
| Bloodthirster | 0.80 | triggered on combat_damage_player → untap |
| Brightfield Mustang | 0.80 | triggered on attack_while_saddled → untap |
| Cat-Owl | 0.80 | triggered on attack → untap |
| Civic Gardener | 0.80 | triggered on attack → untap |
| Copperhorn Scout | 0.80 | triggered on attack → untap |
| Corridor Monitor | 0.80 | triggered on etb → untap |
| Dauntless Aven | 0.80 | triggered on attack → untap |
| Deep-Slumber Titan | 0.80 | triggered on dealt_damage → untap |
| Deepchannel Duelist | 0.80 | triggered on phase → untap |
| Deepway Navigator | 0.80 | triggered on etb → untap |
| Goatnapper | 0.80 | triggered on etb → untap |
| Grim Reaper's Sprint | 0.80 | triggered on etb → untap |
| Hyrax Tower Scout | 0.80 | triggered on etb → untap |
| Initiate's Companion | 0.80 | triggered on combat_damage_player → untap |
| Intruder Alarm | 0.80 | triggered on creature_etb_any → untap |
| Jangling Automaton | 0.80 | triggered on attack → untap |
| Jorn, God of Winter // Kaldring, the Rimestaff | 0.80 | triggered on attack → untap |
| Liege of the Axe | 0.80 | triggered on turned_face_up → untap |
| Lita, Mechanical Engineer | 0.80 | triggered on phase → untap |
| Merfolk Skyscout | 0.80 | triggered on attack_or_block → untap |
| Merrow Commerce | 0.80 | triggered on phase → untap |
| Militia Rallier | 0.80 | triggered on attack → untap |
| Moraug, Fury of Akoum | 0.80 | triggered on phase → untap |
| Nature's Will | 0.80 | triggered on group_combat_damage_player → untap |
| Out of Time | 0.80 | triggered on etb → untap |
| Pine Walker | 0.80 | triggered on turned_face_up → untap |
| Plargg, Dean of Chaos // Augusta, Dean of Order | 0.80 | triggered on you_attack → untap |
| Port Razer | 0.80 | triggered on combat_damage_player → untap |
| Preston Garvey, Minuteman | 0.80 | triggered on attack → untap |
| Prize Pig | 0.80 | triggered on gain_life → untap |
| Raggadragga, Goreguts Boss | 0.80 | triggered on attack → untap |
| Raiyuu, Storm's Edge | 0.80 | triggered on attack_alone → untap |
| Silkenfist Fighter | 0.80 | triggered on becomes_blocked → untap |
| Silkenfist Order | 0.80 | triggered on becomes_blocked → untap |
| Sky Hussar | 0.80 | triggered on etb → untap |
| Sparring Mummy | 0.80 | triggered on etb → untap |
| Sparring Regimen | 0.80 | triggered on you_attack → untap |
| The Watcher in the Water | 0.80 | triggered on tribe_you_control_dies → untap |
| Thistledown Players | 0.80 | triggered on attack → untap |
| Tifa, Martial Artist | 0.80 | triggered on group_combat_damage_player → untap |
| Unstoppable Plan | 0.80 | triggered on phase → untap |
| Urtet, Remnant of Memnarch | 0.80 | triggered on phase → untap |
| Veteran Beastrider | 0.80 | triggered on phase → untap |
| Village Bell-Ringer | 0.80 | triggered on etb → untap |
| Voltaic Servant | 0.80 | triggered on phase → untap |
| Wilderness Reclamation | 0.80 | triggered on phase → untap |
| Xolatoyac, the Smiling Flood | 0.80 | triggered on phase → untap |
| Zephyr Winder | 0.80 | triggered on combat_damage_player → untap |
| Act of Heroism | 0.50 | static ability referencing untap |
| Aim High | 0.50 | static ability referencing untap |
| Alarum | 0.50 | static ability referencing untap |
| Battle Cry | 0.50 | static ability referencing untap |
| Bear Umbra | 0.50 | static ability referencing untap |
| Benefactor's Draught | 0.50 | static ability referencing untap |
| Blind with Anger | 0.50 | static ability referencing untap |
| Blinkmoth Infusion | 0.50 | static ability referencing untap |
| Breaking Wave | 0.50 | static ability referencing untap |
| Burst of Energy | 0.50 | static ability referencing untap |
| Cacophodon | 0.50 | static ability referencing untap |
| Call to Glory | 0.50 | static ability referencing untap |
| Chain Stasis | 0.50 | static ability referencing untap |
| Dazzling Theater // Prop Room | 0.50 | static ability referencing untap |
| Deceiver Exarch | 0.50 | static ability referencing untap |
| Disciple of the Ring | 0.50 | static ability referencing untap |
| Disharmony | 0.50 | static ability referencing untap |
| Djeru's Resolve | 0.50 | static ability referencing untap |
| Dramatic Reversal | 0.50 | static ability referencing untap |
| Dream's Grip | 0.50 | static ability referencing untap |
| Drumbellower | 0.50 | static ability referencing untap |
| Dryad's Caress | 0.50 | static ability referencing untap |
| Elite Interceptor // Rejoinder | 0.50 | static ability referencing untap |
| Emerald Charm | 0.50 | static ability referencing untap |
| Fear of Missing Out | 0.50 | static ability referencing untap |
| Flash Conscription | 0.50 | static ability referencing untap |
| Flash Thompson, Spider-Fan | 0.50 | static ability referencing untap |
| Flying Crane Technique | 0.50 | static ability referencing untap |
| Foxfire | 0.50 | static ability referencing untap |
| Fury of the Horde | 0.50 | static ability referencing untap |
| Gandalf the Grey | 0.50 | static ability referencing untap |
| Gerrard's Command | 0.50 | static ability referencing untap |
| Ghostly Touch | 0.50 | static ability referencing untap |
| Gift of Growth | 0.50 | static ability referencing untap |
| Glamermite | 0.50 | static ability referencing untap |
| Great Train Heist | 0.50 | static ability referencing untap |
| Hidden Strings | 0.50 | static ability referencing untap |
| Infuse | 0.50 | static ability referencing untap |
| Inspirit | 0.50 | static ability referencing untap |
| Insurrection | 0.50 | static ability referencing untap |
| Intellectual Offering | 0.50 | static ability referencing untap |
| Ivorytusk Fortress | 0.50 | static ability referencing untap |
| Join Shields | 0.50 | static ability referencing untap |
| Jolt | 0.50 | static ability referencing untap |
| Legolas's Quick Reflexes | 0.50 | static ability referencing untap |
| Lightning Runner | 0.50 | static ability referencing untap |
| Lost Jitte | 0.50 | static ability referencing untap |
| Mobilize | 0.50 | static ability referencing untap |
| Molten Note | 0.50 | static ability referencing untap |
| Moment of Valor | 0.50 | static ability referencing untap |
| Murkfiend Liege | 0.50 | static ability referencing untap |
| Ohabi Caleria | 0.50 | static ability referencing untap |
| Ornamental Courage | 0.50 | static ability referencing untap |
| Overpowering Attack | 0.50 | static ability referencing untap |
| Prepare // Fight | 0.50 | static ability referencing untap |
| Prophet of Kruphix | 0.50 | static ability referencing untap |
| Provoke | 0.50 | static ability referencing untap |
| Psychic Puppetry | 0.50 | static ability referencing untap |
| Quest for Renewal | 0.50 | static ability referencing untap |
| Rally of Wings | 0.50 | static ability referencing untap |
| Rally the Righteous | 0.50 | static ability referencing untap |
| Rally the Troops | 0.50 | static ability referencing untap |
| Ray of Command | 0.50 | static ability referencing untap |
| Ready // Willing | 0.50 | static ability referencing untap |
| Refocus | 0.50 | static ability referencing untap |
| Relentless Assault | 0.50 | static ability referencing untap |
| Reset | 0.50 | static ability referencing untap |
| Retreat to Coralhelm | 0.50 | static ability referencing untap |
| Roar of the Kha | 0.50 | static ability referencing untap |
| Rude Awakening | 0.50 | static ability referencing untap |
| Rustler Rampage | 0.50 | static ability referencing untap |
| Sage's Nouliths | 0.50 | static ability referencing untap |
| Savage Beating | 0.50 | static ability referencing untap |
| Seedborn Muse | 0.50 | static ability referencing untap |
| Seize the Day | 0.50 | static ability referencing untap |
| Sokka's Haiku | 0.50 | static ability referencing untap |
| Spidery Grasp | 0.50 | static ability referencing untap |
| Spinal Embrace | 0.50 | static ability referencing untap |
| Steady Aim | 0.50 | static ability referencing untap |
| Take the Bait | 0.50 | static ability referencing untap |
| Temporary Insanity | 0.50 | static ability referencing untap |
| The Thirteenth Doctor | 0.50 | static ability referencing untap |
| Thoughtweft Gambit | 0.50 | static ability referencing untap |
| Threaten | 0.50 | static ability referencing untap |
| Tidal Bore | 0.50 | static ability referencing untap |
| To Arms! | 0.50 | static ability referencing untap |
| Toils of Night and Day | 0.50 | static ability referencing untap |
| Tori D'Avenant, Fury Rider | 0.50 | static ability referencing untap |
| Twiddle | 0.50 | static ability referencing untap |
| Twitch | 0.50 | static ability referencing untap |
| Unity of Purpose | 0.50 | static ability referencing untap |
| Unswerving Sloth | 0.50 | static ability referencing untap |
| Unwinding Clock | 0.50 | static ability referencing untap |
| Vitalize | 0.50 | static ability referencing untap |
| Waves of Aggression | 0.50 | static ability referencing untap |
| White Plume Adventurer | 0.50 | static ability referencing untap |
| Woodland Guidance | 0.50 | static ability referencing untap |
| Word of Seizing | 0.50 | static ability referencing untap |
| Ahn-Crop Champion | 0.40 | regex: untap clause inside a trigger |
| All-Out Assault | 0.40 | regex: untap clause inside a trigger |
| Aurelia, the Warleader | 0.40 | regex: untap clause inside a trigger |
| Beacon Hawk | 0.40 | regex: untap clause inside a trigger |
| Beledros Witherbloom | 0.40 | regex: untap clause inside a trigger |
| Betor, Kin to All | 0.40 | regex: untap clause inside a trigger |
| Bounding Krasis | 0.40 | regex: untap clause inside a trigger |
| Breath of Fury | 0.40 | regex: untap clause inside a trigger |
| Bringer of the Red Dawn | 0.40 | regex: untap clause inside a trigger |
| Bumi, Unleashed | 0.40 | regex: untap clause inside a trigger |
| Captain of the Mists | 0.40 | regex: untap clause inside a trigger |
| Chakram Retriever | 0.40 | regex: untap clause inside a trigger |
| Combat Celebrant | 0.40 | regex: untap clause inside a trigger |
| Component Collector | 0.40 | regex: untap clause inside a trigger |
| Coral Trickster | 0.40 | regex: untap clause inside a trigger |
| Curse of Bounty | 0.40 | regex: untap clause inside a trigger |
| Curse of Inertia | 0.40 | regex: untap clause inside a trigger |
| Derevi, Empyrial Tactician | 0.40 | regex: untap clause inside a trigger |
| Dross Scorpion | 0.40 | regex: untap clause inside a trigger |
| Elmar, Ulvenwald Informant | 0.40 | regex: untap clause inside a trigger |
| Esper Sojourners | 0.40 | regex: untap clause inside a trigger |
| Faces of the Past | 0.40 | regex: untap clause inside a trigger |
| Full Throttle | 0.40 | regex: untap clause inside a trigger |
| Granite Witness | 0.40 | regex: untap clause inside a trigger |
| Hellkite Charger | 0.40 | regex: untap clause inside a trigger |
| Hematite Talisman | 0.40 | regex: untap clause inside a trigger |
| Hexplate Wallbreaker | 0.40 | regex: untap clause inside a trigger |
| Innocuous Researcher | 0.40 | regex: untap clause inside a trigger |
| Janjeet Sentry | 0.40 | regex: untap clause inside a trigger |
| Karlach, Fury of Avernus | 0.40 | regex: untap clause inside a trigger |
| Karrthus, Tyrant of Jund | 0.40 | regex: untap clause inside a trigger |
| Lapis Lazuli Talisman | 0.40 | regex: untap clause inside a trigger |
| Magus of the Unseen | 0.40 | regex: untap clause inside a trigger |
| Malachite Talisman | 0.40 | regex: untap clause inside a trigger |
| Merrow Reejerey | 0.40 | regex: untap clause inside a trigger |
| Mirran Spy | 0.40 | regex: untap clause inside a trigger |
| Nacre Talisman | 0.40 | regex: untap clause inside a trigger |
| Najeela, the Blade-Blossom | 0.40 | regex: untap clause inside a trigger |
| Norritt | 0.40 | regex: untap clause inside a trigger |
| Onyx Talisman | 0.40 | regex: untap clause inside a trigger |
| Paradox Engine | 0.40 | regex: untap clause inside a trigger |
| Pestermite | 0.40 | regex: untap clause inside a trigger |
| Raphael, Tag Team Tough | 0.40 | regex: untap clause inside a trigger |
| Reveille Squad | 0.40 | regex: untap clause inside a trigger |
| Scourge of the Throne | 0.40 | regex: untap clause inside a trigger |
| Sewer-veillance Cam | 0.40 | regex: untap clause inside a trigger |
| Soldevi Golem | 0.40 | regex: untap clause inside a trigger |
| Stinging Lionfish | 0.40 | regex: untap clause inside a trigger |
| Stone-Seeder Hierophant | 0.40 | regex: untap clause inside a trigger |
| … | | +13 more in JSON |

## Untap-on-activation (pair fuel) (58)

| Card | Confidence | Reason |
|---|---:|---|
| Aggravated Assault | 0.70 | {3}{r}{r} → untap |
| Aphetto Alchemist | 0.70 | {T} → untap |
| Arbor Elf | 0.70 | {T} → untap |
| Beledros Witherbloom | 0.70 | pay 10 life → untap |
| Blossom Dryad | 0.70 | {T} → untap |
| Clever Conjurer | 0.70 | free → untap |
| Clock of Omens | 0.70 | free → untap |
| Companion of the Trials | 0.70 | {1}{w} → untap |
| Deserted Temple | 0.70 | {1} + {T} → untap |
| Earthcraft | 0.70 | free → untap |
| Ebony Horse | 0.70 | {2} + {T} → untap |
| Elvish Scout | 0.70 | {g} + {T} → untap |
| Filigree Sages | 0.70 | {2}{u} → untap |
| Fyndhorn Brownie | 0.70 | {2}{g} + {T} → untap |
| Galvanic Key | 0.70 | {3} + {T} → untap |
| Greenside Watcher | 0.70 | {T} → untap |
| Griffin Canyon | 0.70 | {T} → untap |
| High Alert | 0.70 | {2}{w}{u} → untap |
| Hope Tender | 0.70 | {1} + {T} → untap |
| Ith, High Arcanist | 0.70 | {T} → untap |
| Jandor's Saddlebags | 0.70 | {3} + {T} → untap |
| Juniper Order Druid | 0.70 | {T} → untap |
| Krosan Restorer | 0.70 | {T} → untap |
| Ley Druid | 0.70 | {T} → untap |
| Llanowar Druid | 0.70 | {T} + sac this → untap |
| Magewright's Stone | 0.70 | {1} + {T} → untap |
| Magus of the Unseen | 0.70 | {1}{u} + {T} → untap |
| Maze of Ith | 0.70 | {T} → untap |
| Maze of Shadows | 0.70 | {T} → untap |
| Minamo, School at Water's Edge | 0.70 | {u} + {T} → untap |
| Myr Galvanizer | 0.70 | {1} + {T} → untap |
| Najeela, the Blade-Blossom | 0.70 | {w}{u}{b}{r}{g} → untap |
| Nature's Chosen | 0.70 | free → untap |
| Norritt | 0.70 | {T} → untap |
| Oboro Breezecaller | 0.70 | {2} → untap |
| Overtaker | 0.70 | {3}{u} + {T} + discard 1 → untap |
| Patriar's Seal | 0.70 | {1} + {T} → untap |
| Patron of the Orochi | 0.70 | {T} → untap |
| Portent Tracker | 0.70 | {T} → untap |
| Quirion Ranger | 0.70 | free → untap |
| Riptide Chronologist | 0.70 | {u} + sac this → untap |
| Rustvine Cultivator | 0.70 | {T} → untap |
| Scryb Ranger | 0.70 | free → untap |
| Sculptor of Winter | 0.70 | {T} → untap |
| Seeker of Skybreak | 0.70 | {T} → untap |
| Silvanus's Invoker | 0.70 | free → untap |
| Staff of Domination | 0.70 | {3} + {T} → untap |
| Stone-Seeder Hierophant | 0.70 | {T} → untap |
| Thaumatic Compass // Spires of Orazca | 0.70 | {T} → untap |
| Tidewater Minion | 0.70 | {T} → untap |
| Trade Caravan | 0.70 | free → untap |
| Vigean Graftmage | 0.70 | {1}{u} → untap |
| Voltaic Construct | 0.70 | {2} → untap |
| Voltaic Key | 0.70 | {1} + {T} → untap |
| Voyaging Satyr | 0.70 | {T} → untap |
| Wakeroot Elemental | 0.70 | {g}{g}{g}{g}{g} → untap |
| Wirewood Lodge | 0.70 | {g} + {T} → untap |
| Wirewood Symbiote | 0.70 | free → untap |

## Storm engines (trigger-per-cast) (1,508)

| Card | Confidence | Reason |
|---|---:|---|
| Aeve, Progenitor Ooze | 0.90 | has Storm keyword |
| All of History, All at Once | 0.90 | has Storm keyword |
| Amphibian Downpour | 0.90 | has Storm keyword |
| Astral Steel | 0.90 | has Storm keyword |
| Brain Freeze | 0.90 | has Storm keyword |
| Chatterstorm | 0.90 | has Storm keyword |
| Dragonstorm | 0.90 | has Storm keyword |
| Elemental Eruption | 0.90 | has Storm keyword |
| Empty the Warrens | 0.90 | has Storm keyword |
| Fiery Encore | 0.90 | has Storm keyword |
| Flusterstorm | 0.90 | has Storm keyword |
| Galvanic Relay | 0.90 | has Storm keyword |
| Grapeshot | 0.90 | has Storm keyword |
| Ground Rift | 0.90 | has Storm keyword |
| Haze of Rage | 0.90 | has Storm keyword |
| Hindering Touch | 0.90 | has Storm keyword |
| Hunting Pack | 0.90 | has Storm keyword |
| Ignite Memories | 0.90 | has Storm keyword |
| Limitless Rekindling | 0.90 | has Storm keyword |
| Mind's Desire | 0.90 | has Storm keyword |
| Mordor on the March | 0.90 | has Storm keyword |
| Radstorm | 0.90 | has Storm keyword |
| Reaping the Graves | 0.90 | has Storm keyword |
| Scattershot | 0.90 | has Storm keyword |
| Spreading Insurrection | 0.90 | has Storm keyword |
| Sprouting Vines | 0.90 | has Storm keyword |
| Storm Seeker | 0.90 | has Storm keyword |
| Storm of Memories | 0.90 | has Storm keyword |
| Storm of Steel | 0.90 | has Storm keyword |
| Storm's Wrath | 0.90 | has Storm keyword |
| Stormscale Scion | 0.90 | has Storm keyword |
| Tempest Technique | 0.90 | has Storm keyword |
| Temporal Fissure | 0.90 | has Storm keyword |
| Tendrils of Agony | 0.90 | has Storm keyword |
| Volcanic Awakening | 0.90 | has Storm keyword |
| Weather the Storm | 0.90 | has Storm keyword |
| Wing Shards | 0.90 | has Storm keyword |
| A-Devoted Grafkeeper // A-Departed Soulkeeper | 0.70 | trigger on cast_any |
| A-Dragon's Rage Channeler | 0.70 | trigger on cast_filtered |
| A-Leyline of Resonance | 0.70 | trigger on cast_filtered |
| A-Master of Winds | 0.70 | trigger on cast_any |
| A-Rockslide Sorcerer | 0.70 | trigger on cast_any |
| A-Sorcerer Class | 0.70 | trigger on cast_filtered |
| A-Umara Mystic | 0.70 | trigger on cast_any |
| A-Vega, the Watcher | 0.70 | trigger on cast_any |
| A-Vivi Ornitier | 0.70 | trigger on cast_filtered |
| Aang, the Last Airbender | 0.70 | trigger on cast_filtered |
| Aberrant Manawurm | 0.70 | trigger on cast_filtered |
| Abigale, Poet Laureate // Heroic Stanza | 0.70 | trigger on cast_filtered |
| Academy Wall | 0.70 | trigger on cast_filtered |
| Accident-Prone Apprentice // Amphibian Accident | 0.70 | trigger on cast_filtered |
| Ace Flockbringer | 0.70 | trigger on cast_filtered |
| Adaptive Training Post | 0.70 | trigger on cast_filtered |
| Adeliz, the Cinder Wind | 0.70 | trigger on cast_filtered |
| Aerial Extortionist | 0.70 | trigger on cast_filtered |
| Aetherflux Conduit | 0.70 | trigger on cast_any |
| Aetherflux Reservoir | 0.70 | trigger on cast_any |
| Alania, Divergent Storm | 0.70 | trigger on cast_any |
| Alchemist's Talent | 0.70 | trigger on cast_any |
| Alela, Artful Provocateur | 0.70 | trigger on cast_filtered |
| Alela, Cunning Conqueror | 0.70 | trigger on cast_any |
| Alistair, the Brigadier | 0.70 | trigger on cast_filtered |
| Ambling Stormshell | 0.70 | trigger on cast_filtered |
| Ancient Cornucopia | 0.70 | trigger on cast_any |
| Angel of Unity | 0.70 | trigger on cast_filtered |
| Animar, Soul of Elements | 0.70 | trigger on cast_filtered |
| Appa, Steadfast Guardian | 0.70 | trigger on cast_any |
| Aquatic Alchemist // Bubble Up | 0.70 | trigger on cast_any |
| Arcane Bombardment | 0.70 | trigger on cast_any |
| Arcbound Tracker | 0.70 | trigger on cast_filtered |
| Arcee, Sharpshooter // Arcee, Acrobatic Coupe | 0.70 | trigger on cast_any |
| Archmage of Echoes | 0.70 | trigger on cast_filtered |
| Archmage of Runes | 0.70 | trigger on cast_filtered |
| Arena Trickster | 0.70 | trigger on cast_any |
| Argothian Enchantress | 0.70 | trigger on cast_filtered |
| Aria of Flame | 0.70 | trigger on cast_filtered |
| Arixmethes, Slumbering Isle | 0.70 | trigger on cast_any |
| Arjun, the Shifting Flame | 0.70 | trigger on cast_any |
| Armory Paladin | 0.70 | trigger on cast_filtered |
| Artificer's Assistant | 0.70 | trigger on cast_filtered |
| Artist's Talent | 0.70 | trigger on cast_filtered |
| Atmosphere Surgeon | 0.70 | trigger on cast_filtered |
| Aurora Phoenix | 0.70 | trigger on cast_any |
| Aven Wind Mage | 0.70 | trigger on cast_filtered |
| Aziza, Mage Tower Captain | 0.70 | trigger on cast_filtered |
| Baku Altar | 0.70 | trigger on cast_filtered |
| Balmor, Battlemage Captain | 0.70 | trigger on cast_filtered |
| Baral and Kari Zev | 0.70 | trigger on cast_any |
| Basim Ibn Ishaq | 0.70 | trigger on cast_filtered |
| Battery Bearer | 0.70 | trigger on cast_filtered |
| Battlegate Mimic | 0.70 | trigger on cast_any |
| Battlewand Oak | 0.70 | trigger on cast_filtered |
| Beamsplitter Mage | 0.70 | trigger on cast_filtered |
| Beast Whisperer | 0.70 | trigger on cast_filtered |
| Bill Potts | 0.70 | trigger on cast_filtered |
| Biotransference | 0.70 | trigger on cast_filtered |
| Black Waltz No. 3 | 0.70 | trigger on cast_filtered |
| Blademane Baku | 0.70 | trigger on cast_filtered |
| Blazing Bomb | 0.70 | trigger on cast_filtered |
| Blessed Spirits | 0.70 | trigger on cast_filtered |
| Blightcaster | 0.70 | trigger on cast_filtered |
| Blightwing Bandit | 0.70 | trigger on cast_any |
| Blistercoil Weird | 0.70 | trigger on cast_filtered |
| Blisterspit Gremlin | 0.70 | trigger on cast_filtered |
| Blood Funnel | 0.70 | trigger on cast_filtered |
| Bloodlord of Vaasgoth | 0.70 | trigger on cast_filtered |
| Bloodsky Berserker | 0.70 | trigger on cast_any |
| Bloodstone Goblin | 0.70 | trigger on cast_any |
| Boar-q-pine | 0.70 | trigger on cast_filtered |
| Bohn, Beguiling Balladeer | 0.70 | trigger on cast_any |
| Bontu's Monument | 0.70 | trigger on cast_filtered |
| Boreal Outrider | 0.70 | trigger on cast_filtered |
| Bothersome Quasit | 0.70 | trigger on cast_filtered |
| Bounteous Kirin | 0.70 | trigger on cast_filtered |
| Brass's Tunnel-Grinder // Tecutlan, the Searing Rift | 0.70 | trigger on cast_filtered |
| Breath of the Sleepless | 0.70 | trigger on cast_filtered |
| Breeches, the Blastmaker | 0.70 | trigger on cast_any |
| Bria, Riptide Rogue | 0.70 | trigger on cast_filtered |
| Briarknit Kami | 0.70 | trigger on cast_filtered |
| Brimaz, Blight of Oreskos | 0.70 | trigger on cast_filtered |
| Brimstone Roundup | 0.70 | trigger on cast_any |
| Brineborn Cutthroat | 0.70 | trigger on cast_any |
| Budoka Pupil // Ichiga, Who Topples Oaks | 0.70 | trigger on cast_filtered |
| Burning Prophet | 0.70 | trigger on cast_filtered |
| Burning Vengeance | 0.70 | trigger on cast_any |
| Bygone Bishop | 0.70 | trigger on cast_filtered |
| Cabal Paladin | 0.70 | trigger on cast_filtered |
| Cabaretti Revels | 0.70 | trigger on cast_filtered |
| Caldera Pyremaw | 0.70 | trigger on cast_filtered |
| Callow Jushi // Jaraku the Interloper | 0.70 | trigger on cast_filtered |
| Captain Ripley Vance | 0.70 | trigger on cast_any |
| Cathar's Companion | 0.70 | trigger on cast_filtered |
| Celestial Ancient | 0.70 | trigger on cast_filtered |
| Celestial Kirin | 0.70 | trigger on cast_filtered |
| Chakra Meditation | 0.70 | trigger on cast_filtered |
| Chakram Retriever | 0.70 | trigger on cast_any |
| Champions of the Perfect | 0.70 | trigger on cast_filtered |
| Chancellor of Tales | 0.70 | trigger on cast_filtered |
| Charmbreaker Devils | 0.70 | trigger on cast_filtered |
| Cherished Hatchling | 0.70 | trigger on cast_filtered |
| Chrome Host Seedshark | 0.70 | trigger on cast_filtered |
| Chronicle of Victory | 0.70 | trigger on cast_any |
| Chulane, Teller of Tales | 0.70 | trigger on cast_filtered |
| Clarion Spirit | 0.70 | trigger on cast_any |
| Cloudhoof Kirin | 0.70 | trigger on cast_filtered |
| Cloven Casting | 0.70 | trigger on cast_filtered |
| Communal Brewing | 0.70 | trigger on cast_filtered |
| Consuming Aberration | 0.70 | trigger on cast_any |
| Contemplation | 0.70 | trigger on cast_any |
| Convention Maro | 0.70 | trigger on cast_any |
| Coralhelm Chronicler | 0.70 | trigger on cast_filtered |
| Coruscation Mage | 0.70 | trigger on cast_filtered |
| Cosmogrand Zenith | 0.70 | trigger on cast_any |
| Crackling Cyclops | 0.70 | trigger on cast_filtered |
| Cruel Witness | 0.70 | trigger on cast_filtered |
| Cryptic Pursuit | 0.70 | trigger on cast_filtered |
| Cunning Bandit // Azamuki, Treachery Incarnate | 0.70 | trigger on cast_filtered |
| Cunning Breezedancer | 0.70 | trigger on cast_filtered |
| Curse of Echoes | 0.70 | trigger on cast_filtered |
| Cursed Recording | 0.70 | trigger on cast_filtered |
| Customs Depot | 0.70 | trigger on cast_filtered |
| D'Avenant Trapper | 0.70 | trigger on cast_filtered |
| Daring Archaeologist | 0.70 | trigger on cast_filtered |
| Dawnhart Geist | 0.70 | trigger on cast_filtered |
| Daxos the Returned | 0.70 | trigger on cast_filtered |
| Deeproot Champion | 0.70 | trigger on cast_filtered |
| Deeproot Waters | 0.70 | trigger on cast_filtered |
| Defiler of Dreams | 0.70 | trigger on cast_filtered |
| Defiler of Faith | 0.70 | trigger on cast_filtered |
| Defiler of Flesh | 0.70 | trigger on cast_filtered |
| Defiler of Instinct | 0.70 | trigger on cast_filtered |
| Defiler of Vigor | 0.70 | trigger on cast_filtered |
| Devoted Grafkeeper // Departed Soulkeeper | 0.70 | trigger on cast_any |
| Diamond Knight | 0.70 | trigger on cast_any |
| Diamond Mare | 0.70 | trigger on cast_any |
| Diary of Dreams | 0.70 | trigger on cast_filtered |
| Digsite Engineer | 0.70 | trigger on cast_filtered |
| Diligent Excavator | 0.70 | trigger on cast_filtered |
| Diregraf Colossus | 0.70 | trigger on cast_filtered |
| Dirgur Focusmage // Braingeyser | 0.70 | trigger on cast_filtered |
| Discreet Retreat | 0.70 | trigger on cast_any |
| Djinn of the Fountain | 0.70 | trigger on cast_filtered |
| Docent of Perfection // Final Iteration | 0.70 | trigger on cast_filtered |
| Don Andres, the Renegade | 0.70 | trigger on cast_filtered |
| Donal, Herald of Wings | 0.70 | trigger on cast_filtered |
| Doomskar Oracle | 0.70 | trigger on cast_any |
| Door of Destinies | 0.70 | trigger on cast_any |
| Double Down | 0.70 | trigger on cast_filtered |
| Double Vision | 0.70 | trigger on cast_any |
| Dovin's Acuity | 0.70 | trigger on cast_filtered |
| Dr. Madison Li | 0.70 | trigger on cast_filtered |
| Dragon Typhoon | 0.70 | trigger on cast_filtered |
| Dragon's Rage Channeler | 0.70 | trigger on cast_filtered |
| Dragonsoul Prodigy | 0.70 | trigger on cast_any |
| Dragonweave Tapestry | 0.70 | trigger on cast_filtered |
| Dream Spoilers | 0.70 | trigger on cast_any |
| Dreamcatcher | 0.70 | trigger on cast_filtered |
| Dreamspoiler Witches | 0.70 | trigger on cast_any |
| Dreamstalker Manticore | 0.70 | trigger on cast_any |
| Druid of Horns | 0.70 | trigger on cast_filtered |
| … | | +1308 more in JSON |

## Iterative draw engines (280)

| Card | Confidence | Reason |
|---|---:|---|
| A-Omnath, Locus of Creation | 0.65 | trigger on etb → draw/damage |
| Abundant Growth | 0.65 | trigger on etb → draw/damage |
| Aerial Extortionist | 0.65 | trigger on cast_filtered → draw/damage |
| Alchemist's Vial | 0.65 | trigger on etb → draw/damage |
| Angelic Gift | 0.65 | trigger on etb → draw/damage |
| Archmage of Runes | 0.65 | trigger on cast_filtered → draw/damage |
| Arctic Wolves | 0.65 | trigger on etb → draw/damage |
| Arcum's Astrolabe | 0.65 | trigger on etb → draw/damage |
| Argothian Enchantress | 0.65 | trigger on cast_filtered → draw/damage |
| Astral Wingspan | 0.65 | trigger on etb → draw/damage |
| Baleful Strix | 0.65 | trigger on etb → draw/damage |
| Basim Ibn Ishaq | 0.65 | trigger on cast_filtered → draw/damage |
| Beast Whisperer | 0.65 | trigger on cast_filtered → draw/damage |
| Bestial Fury | 0.65 | trigger on etb → draw/damage |
| Black Waltz No. 3 | 0.65 | trigger on cast_filtered → draw/damage |
| Blood Sun | 0.65 | trigger on etb → draw/damage |
| Bomat Bazaar Barge | 0.65 | trigger on etb → draw/damage |
| Bonds of Mortality | 0.65 | trigger on etb → draw/damage |
| Bookwurm | 0.65 | trigger on etb → draw/damage |
| Carrier Pigeons | 0.65 | trigger on etb → draw/damage |
| Cartouche of Knowledge | 0.65 | trigger on etb → draw/damage |
| Carven Caryatid | 0.65 | trigger on etb → draw/damage |
| Casey Jones, Vigilante | 0.65 | trigger on etb → draw/damage |
| Cavalier of Gales | 0.65 | trigger on etb → draw/damage |
| Champions of the Perfect | 0.65 | trigger on cast_filtered → draw/damage |
| Chosen by Heliod | 0.65 | trigger on etb → draw/damage |
| Cloudblazer | 0.65 | trigger on etb → draw/damage |
| Cloudkin Seer | 0.65 | trigger on etb → draw/damage |
| Combat Thresher | 0.65 | trigger on etb → draw/damage |
| Conciliator's Duelist | 0.65 | trigger on etb → draw/damage |
| Confounding Conundrum | 0.65 | trigger on etb → draw/damage |
| Council of Advisors | 0.65 | trigger on etb → draw/damage |
| Coveted Jewel | 0.65 | trigger on etb → draw/damage |
| Crackling Drake | 0.65 | trigger on etb → draw/damage |
| Defiler of Dreams | 0.65 | trigger on cast_filtered → draw/damage |
| Demonic Lore | 0.65 | trigger on etb → draw/damage |
| Didact Echo | 0.65 | trigger on etb → draw/damage |
| Dovin's Acuity | 0.65 | trigger on etb → draw/damage |
| Dragon Mantle | 0.65 | trigger on etb → draw/damage |
| Dragonweave Tapestry | 0.65 | trigger on cast_filtered → draw/damage |
| Eerie Gravestone | 0.65 | trigger on etb → draw/damage |
| Elite Guardmage | 0.65 | trigger on etb → draw/damage |
| Elsewhere Flask | 0.65 | trigger on etb → draw/damage |
| Elvish Visionary | 0.65 | trigger on etb → draw/damage |
| Enchantress's Presence | 0.65 | trigger on cast_filtered → draw/damage |
| Energy Refractor | 0.65 | trigger on etb → draw/damage |
| Eternity Snare | 0.65 | trigger on etb → draw/damage |
| Fate Foretold | 0.65 | trigger on etb → draw/damage |
| Fblthp, the Lost | 0.65 | trigger on etb → draw/damage |
| Feather of Flight | 0.65 | trigger on etb → draw/damage |
| Flight of Fancy | 0.65 | trigger on etb → draw/damage |
| Frog Tongue | 0.65 | trigger on etb → draw/damage |
| Future Flight | 0.65 | trigger on etb → draw/damage |
| Gadwick, the Wizened | 0.65 | trigger on etb → draw/damage |
| Gallant Citizen | 0.65 | trigger on etb → draw/damage |
| Generous Stray | 0.65 | trigger on etb → draw/damage |
| Gladewalker Ritualist | 0.65 | trigger on etb → draw/damage |
| Golden Egg | 0.65 | trigger on etb → draw/damage |
| Grisly Transformation | 0.65 | trigger on etb → draw/damage |
| Ground Seal | 0.65 | trigger on etb → draw/damage |
| Gryff Vanguard | 0.65 | trigger on etb → draw/damage |
| Guidelight Matrix | 0.65 | trigger on etb → draw/damage |
| Guild Globe | 0.65 | trigger on etb → draw/damage |
| Happily Ever After | 0.65 | trigger on etb → draw/damage |
| Haru-Onna | 0.65 | trigger on etb → draw/damage |
| Heart of a Duelist | 0.65 | trigger on etb → draw/damage |
| Helpful Hunter | 0.65 | trigger on etb → draw/damage |
| Hidetsugu and Kairi | 0.65 | trigger on etb → draw/damage |
| Hobble | 0.65 | trigger on etb → draw/damage |
| Hulldrifter | 0.65 | trigger on etb → draw/damage |
| Ice-Fang Coatl | 0.65 | trigger on etb → draw/damage |
| Indris, the Hydrostatic Surge | 0.65 | trigger on cast_filtered → draw/damage |
| Inspiring Overseer | 0.65 | trigger on etb → draw/damage |
| Instant Ramen | 0.65 | trigger on etb → draw/damage |
| Invasion of Dominaria // Serra Faithkeeper | 0.65 | trigger on etb → draw/damage |
| Jhoira, Weatherlight Captain | 0.65 | trigger on cast_filtered → draw/damage |
| Joraga Visionary | 0.65 | trigger on etb → draw/damage |
| Jungle Barrier | 0.65 | trigger on etb → draw/damage |
| Kaleidostone | 0.65 | trigger on etb → draw/damage |
| Karametra's Favor | 0.65 | trigger on etb → draw/damage |
| Kavu Climber | 0.65 | trigger on etb → draw/damage |
| Kenrith's Transformation | 0.65 | trigger on etb → draw/damage |
| Kindly Customer | 0.65 | trigger on etb → draw/damage |
| Krovikan Fetish | 0.65 | trigger on etb → draw/damage |
| Krovikan Plague | 0.65 | trigger on etb → draw/damage |
| Lashknife Barrier | 0.65 | trigger on etb → draw/damage |
| Lembas | 0.65 | trigger on etb → draw/damage |
| Liliana's Contract | 0.65 | trigger on etb → draw/damage |
| Lithoform Blight | 0.65 | trigger on etb → draw/damage |
| Llanowar Visionary | 0.65 | trigger on etb → draw/damage |
| Lofty Dreams | 0.65 | trigger on etb → draw/damage |
| Longshot, Rebel Bowman | 0.65 | trigger on cast_filtered → draw/damage |
| Lozhan, Dragons' Legacy | 0.65 | trigger on cast_filtered → draw/damage |
| Masked Admirers | 0.65 | trigger on etb → draw/damage |
| Merchant of Secrets | 0.65 | trigger on etb → draw/damage |
| Messenger Falcons | 0.65 | trigger on etb → draw/damage |
| Mightstone's Animation | 0.65 | trigger on etb → draw/damage |
| Mistmeadow Council | 0.65 | trigger on etb → draw/damage |
| Mulldrifter | 0.65 | trigger on etb → draw/damage |
| Multani's Acolyte | 0.65 | trigger on etb → draw/damage |
| Muse Drake | 0.65 | trigger on etb → draw/damage |
| Nerd Rage | 0.65 | trigger on etb → draw/damage |
| New Perspectives | 0.65 | trigger on etb → draw/damage |
| Nimble Innovator | 0.65 | trigger on etb → draw/damage |
| Noggle Ransacker | 0.65 | trigger on etb → draw/damage |
| Nylea's Presence | 0.65 | trigger on etb → draw/damage |
| Omen of the Sea | 0.65 | trigger on etb → draw/damage |
| Omnath, Locus of Creation | 0.65 | trigger on etb → draw/damage |
| Omni-Cheese Pizza | 0.65 | trigger on etb → draw/damage |
| Orysa, Tide Choreographer | 0.65 | trigger on etb → draw/damage |
| Papalymo Totolymo | 0.65 | trigger on cast_filtered → draw/damage |
| Pentarch Ward | 0.65 | trigger on etb → draw/damage |
| Phyrexian Gargantua | 0.65 | trigger on etb → draw/damage |
| Pond Prophet | 0.65 | trigger on etb → draw/damage |
| Potion of Healing | 0.65 | trigger on etb → draw/damage |
| Priest of Ancient Lore | 0.65 | trigger on etb → draw/damage |
| Proft's Eidetic Memory | 0.65 | trigger on etb → draw/damage |
| Prophetic Prism | 0.65 | trigger on etb → draw/damage |
| Pyknite | 0.65 | trigger on etb → draw/damage |
| Quantum Riddler | 0.65 | trigger on etb → draw/damage |
| Reki, the History of Kamigawa | 0.65 | trigger on cast_filtered → draw/damage |
| Rhox Oracle | 0.65 | trigger on etb → draw/damage |
| Ribskiff | 0.65 | trigger on etb → draw/damage |
| Ritual of Steel | 0.65 | trigger on etb → draw/damage |
| Riverwise Augur | 0.65 | trigger on etb → draw/damage |
| Roving Harper | 0.65 | trigger on etb → draw/damage |
| Rune of Flight | 0.65 | trigger on etb → draw/damage |
| Rune of Might | 0.65 | trigger on etb → draw/damage |
| Rune of Mortality | 0.65 | trigger on etb → draw/damage |
| Rune of Speed | 0.65 | trigger on etb → draw/damage |
| Rune of Sustenance | 0.65 | trigger on etb → draw/damage |
| Sage of Ancient Lore // Werewolf of Ancient Hunger | 0.65 | trigger on etb → draw/damage |
| Salt Road Packbeast | 0.65 | trigger on etb → draw/damage |
| Sarulf's Packmate | 0.65 | trigger on etb → draw/damage |
| Satyr Enchanter | 0.65 | trigger on cast_filtered → draw/damage |
| Scavenged Weaponry | 0.65 | trigger on etb → draw/damage |
| Scourgemark | 0.65 | trigger on etb → draw/damage |
| Search Party Captain | 0.65 | trigger on etb → draw/damage |
| Setessan Training | 0.65 | trigger on etb → draw/damage |
| Shaman of Spring | 0.65 | trigger on etb → draw/damage |
| Sheltering Boughs | 0.65 | trigger on etb → draw/damage |
| Shielding Plax | 0.65 | trigger on etb → draw/damage |
| Silvergill Adept | 0.65 | trigger on etb → draw/damage |
| Sisay's Ingenuity | 0.65 | trigger on etb → draw/damage |
| Skyscanner | 0.65 | trigger on etb → draw/damage |
| Sleeper Dart | 0.65 | trigger on etb → draw/damage |
| Soaring Show-Off | 0.65 | trigger on etb → draw/damage |
| Spare Supplies | 0.65 | trigger on etb → draw/damage |
| Spark Rupture | 0.65 | trigger on etb → draw/damage |
| Spirited Companion | 0.65 | trigger on etb → draw/damage |
| Spreading Seas | 0.65 | trigger on etb → draw/damage |
| Storyteller Pixie | 0.65 | trigger on cast_filtered → draw/damage |
| Stratus Walk | 0.65 | trigger on etb → draw/damage |
| Striped Bears | 0.65 | trigger on etb → draw/damage |
| Stupefying Touch | 0.65 | trigger on etb → draw/damage |
| Surtr, Fiery Jötun | 0.65 | trigger on cast_filtered → draw/damage |
| Sythis, Harvest's Hand | 0.65 | trigger on cast_filtered → draw/damage |
| Tainted Well | 0.65 | trigger on etb → draw/damage |
| The Destined Black Mage | 0.65 | trigger on cast_filtered → draw/damage |
| Thought Monitor | 0.65 | trigger on etb → draw/damage |
| Tome Raider | 0.65 | trigger on etb → draw/damage |
| Traveler's Cloak | 0.65 | trigger on etb → draw/damage |
| Treacherous Blessing | 0.65 | trigger on etb → draw/damage |
| Tsabo's Web | 0.65 | trigger on etb → draw/damage |
| Unquestioned Authority | 0.65 | trigger on etb → draw/damage |
| Urabrask // The Great Work | 0.65 | trigger on cast_filtered → draw/damage |
| Urban Utopia | 0.65 | trigger on etb → draw/damage |
| Vampirism | 0.65 | trigger on etb → draw/damage |
| Vault Plunderer | 0.65 | trigger on etb → draw/damage |
| Vedalken Archmage | 0.65 | trigger on cast_filtered → draw/damage |
| Venat, Heart of Hydaelyn // Hydaelyn, the Mothercrystal | 0.65 | trigger on cast_filtered → draw/damage |
| Wall of Blossoms | 0.65 | trigger on etb → draw/damage |
| Wall of Omens | 0.65 | trigger on etb → draw/damage |
| Weapons Vendor | 0.65 | trigger on etb → draw/damage |
| Wedding Invitation | 0.65 | trigger on etb → draw/damage |
| Well of Ideas | 0.65 | trigger on etb → draw/damage |
| Whirlwind of Thought | 0.65 | trigger on cast_filtered → draw/damage |
| Wistful Selkie | 0.65 | trigger on etb → draw/damage |
| Woodland Acolyte // Mend the Wilds | 0.65 | trigger on etb → draw/damage |
| Zendikar Resurgent | 0.65 | trigger on cast_filtered → draw/damage |
| A-Vega, the Watcher | 0.45 | regex: draw trigger on cast |
| Academy Wall | 0.45 | regex: draw trigger on cast |
| Alphinaud Leveilleur | 0.45 | regex: draw trigger on cast |
| Anax and Cymede & Kynaios and Tiro | 0.45 | regex: draw trigger on cast |
| Archmage Emeritus | 0.45 | regex: draw trigger on cast |
| Artist's Talent | 0.45 | regex: draw trigger on cast |
| Ashling, Flame Dancer | 0.45 | regex: draw trigger on cast |
| Battery Bearer | 0.45 | regex: draw trigger on cast |
| Bladecoil Serpent | 0.45 | regex: draw trigger on cast |
| Brass Infiniscope | 0.45 | regex: draw trigger on cast |
| Case of the Ransacked Lab | 0.45 | regex: draw trigger on cast |
| Chakra Meditation | 0.45 | regex: draw trigger on cast |
| Chronicle of Victory | 0.45 | regex: draw trigger on cast |
| Chulane, Teller of Tales | 0.45 | regex: draw trigger on cast |
| Clockwork Servant | 0.45 | regex: draw trigger on cast |
| Coralhelm Chronicler | 0.45 | regex: draw trigger on cast |
| Customs Depot | 0.45 | regex: draw trigger on cast |
| Discreet Retreat | 0.45 | regex: draw trigger on cast |
| Diviner's Wand | 0.45 | regex: draw trigger on cast |
| Dreamcatcher | 0.45 | regex: draw trigger on cast |
| … | | +80 more in JSON |

## Doublers (replacement effects on resources) (37)

| Card | Confidence | Reason |
|---|---:|---|
| Adrix and Nev, Twincasters | 0.85 | tokens doubled, counter replacement, token replacement |
| Aether Refinery | 0.85 | counter replacement, token replacement |
| Anointed Procession | 0.85 | tokens doubled, token replacement |
| Doubling Season | 0.85 | tokens doubled, counter replacement, token replacement |
| Exalted Sunborn | 0.85 | tokens doubled, token replacement |
| Michelangelo, Weirdness to 11 | 0.85 | counter replacement, token replacement |
| Mondrak, Glory Dominus | 0.85 | tokens doubled, counter replacement, token replacement |
| Parallel Lives | 0.85 | tokens doubled, token replacement |
| Primal Vigor | 0.85 | tokens doubled, counter replacement, token replacement |
| Benevolent Hydra | 0.70 | counter replacement |
| Branching Evolution | 0.70 | counter replacement |
| Caradora, Heart of Alacria | 0.70 | counter replacement |
| Conclave Mentor | 0.70 | counter replacement |
| Corpsejack Menace | 0.70 | counter replacement |
| Hardened Scales | 0.70 | counter replacement |
| High Score | 0.70 | counter replacement |
| Innkeeper's Talent | 0.70 | counter replacement |
| Izzet Generatorium | 0.70 | counter replacement |
| Kami of Whispered Hopes | 0.70 | counter replacement |
| Lae'zel, Vlaakith's Champion | 0.70 | counter replacement |
| Loading Zone | 0.70 | counter replacement |
| Mauhúr, Uruk-hai Captain | 0.70 | counter replacement |
| Mowu, Loyal Companion | 0.70 | counter replacement |
| Ozolith, the Shattered Spire | 0.70 | counter replacement |
| Pir, Imaginative Rascal | 0.70 | counter replacement |
| Prairie Dog | 0.70 | counter replacement |
| Procrastinate | 0.70 | counter replacement |
| Secrets of the Key | 0.70 | token replacement |
| Solid Ground | 0.70 | counter replacement |
| Stonesplitter Bolt | 0.70 | token replacement |
| Tainted Adversary | 0.70 | tokens doubled |
| Tekuthal, Inquiry Dominus | 0.70 | counter replacement |
| The Earth Crystal | 0.70 | counter replacement |
| Tomb of Horrors Adventurer | 0.70 | token replacement |
| Vorinclex, Monstrous Raider | 0.70 | counter replacement |
| Winding Constrictor | 0.70 | counter replacement |
| Zabaz, the Glimmerwasp | 0.70 | counter replacement |

## Top 2-card combo pairs

Cross-product of detection buckets. Confidence is a coarse blend of
each side's individual confidence — not a solver score.

| # | Card A | Card B | Confidence | Reason |
|---:|---|---|---:|---|
| 1 | Azusa's Many Journeys // Likeness of the Seeker | Nantuko Elder | 0.90 | untap-trigger (triggered on becomes_blocked → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 2 | Corridor Monitor | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 3 | Tifa, Martial Artist | Nantuko Elder | 0.90 | untap-trigger (triggered on group_combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 4 | Preston Garvey, Minuteman | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 5 | Zephyr Winder | Nantuko Elder | 0.90 | untap-trigger (triggered on combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 6 | Prize Pig | Nantuko Elder | 0.90 | untap-trigger (triggered on gain_life → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 7 | Brightfield Mustang | Nantuko Elder | 0.90 | untap-trigger (triggered on attack_while_saddled → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 8 | Intruder Alarm | Nantuko Elder | 0.90 | untap-trigger (triggered on creature_etb_any → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 9 | Sky Hussar | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 10 | Veteran Beastrider | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 11 | Sparring Regimen | Nantuko Elder | 0.90 | untap-trigger (triggered on you_attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 12 | Xolatoyac, the Smiling Flood | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 13 | Merfolk Skyscout | Nantuko Elder | 0.90 | untap-trigger (triggered on attack_or_block → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 14 | Copperhorn Scout | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 15 | Unstoppable Plan | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 16 | Sparring Mummy | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 17 | A-Raiyuu, Storm's Edge | Nantuko Elder | 0.90 | untap-trigger (triggered on attack_alone → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 18 | Anzrag, the Quake-Mole | Nantuko Elder | 0.90 | untap-trigger (triggered on becomes_blocked → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 19 | Pine Walker | Nantuko Elder | 0.90 | untap-trigger (triggered on turned_face_up → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 20 | Awakening | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 21 | Akki Battle Squad | Nantuko Elder | 0.90 | untap-trigger (triggered on group_attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 22 | Silkenfist Order | Nantuko Elder | 0.90 | untap-trigger (triggered on becomes_blocked → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 23 | Urtet, Remnant of Memnarch | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 24 | Moraug, Fury of Akoum | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 25 | Port Razer | Nantuko Elder | 0.90 | untap-trigger (triggered on combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 26 | Goatnapper | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 27 | Wilderness Reclamation | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 28 | Thistledown Players | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 29 | Dauntless Aven | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 30 | Lita, Mechanical Engineer | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 31 | Deepway Navigator | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 32 | Out of Time | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 33 | Raiyuu, Storm's Edge | Nantuko Elder | 0.90 | untap-trigger (triggered on attack_alone → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 34 | Nature's Will | Nantuko Elder | 0.90 | untap-trigger (triggered on group_combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 35 | Grim Reaper's Sprint | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 36 | Civic Gardener | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 37 | Plargg, Dean of Chaos // Augusta, Dean of Order | Nantuko Elder | 0.90 | untap-trigger (triggered on you_attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 38 | Raggadragga, Goreguts Boss | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 39 | Merrow Commerce | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 40 | The Watcher in the Water | Nantuko Elder | 0.90 | untap-trigger (triggered on tribe_you_control_dies → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 41 | Jangling Automaton | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 42 | Cat-Owl | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 43 | Jorn, God of Winter // Kaldring, the Rimestaff | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 44 | Silkenfist Fighter | Nantuko Elder | 0.90 | untap-trigger (triggered on becomes_blocked → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 45 | Hyrax Tower Scout | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 46 | Deepchannel Duelist | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 47 | Voltaic Servant | Nantuko Elder | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 48 | Militia Rallier | Nantuko Elder | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 49 | Village Bell-Ringer | Nantuko Elder | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 50 | Liege of the Axe | Nantuko Elder | 0.90 | untap-trigger (triggered on turned_face_up → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 51 | Deep-Slumber Titan | Nantuko Elder | 0.90 | untap-trigger (triggered on dealt_damage → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 52 | Bloodthirster | Nantuko Elder | 0.90 | untap-trigger (triggered on combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 53 | Initiate's Companion | Nantuko Elder | 0.90 | untap-trigger (triggered on combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 54 | Azusa's Many Journeys // Likeness of the Seeker | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on becomes_blocked → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 55 | Corridor Monitor | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 56 | Tifa, Martial Artist | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on group_combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 57 | Preston Garvey, Minuteman | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 58 | Zephyr Winder | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 59 | Prize Pig | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on gain_life → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 60 | Brightfield Mustang | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack_while_saddled → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 61 | Intruder Alarm | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on creature_etb_any → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 62 | Sky Hussar | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 63 | Veteran Beastrider | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 64 | Sparring Regimen | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on you_attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 65 | Xolatoyac, the Smiling Flood | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 66 | Merfolk Skyscout | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack_or_block → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 67 | Copperhorn Scout | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 68 | Unstoppable Plan | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 69 | Sparring Mummy | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 70 | A-Raiyuu, Storm's Edge | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack_alone → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 71 | Anzrag, the Quake-Mole | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on becomes_blocked → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 72 | Pine Walker | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on turned_face_up → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 73 | Awakening | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 74 | Akki Battle Squad | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on group_attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 75 | Silkenfist Order | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on becomes_blocked → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 76 | Urtet, Remnant of Memnarch | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 77 | Moraug, Fury of Akoum | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 78 | Port Razer | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 79 | Goatnapper | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 80 | Wilderness Reclamation | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 81 | Thistledown Players | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 82 | Dauntless Aven | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 83 | Lita, Mechanical Engineer | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 84 | Deepway Navigator | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 85 | Out of Time | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 86 | Raiyuu, Storm's Edge | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack_alone → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 87 | Nature's Will | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on group_combat_damage_player → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 88 | Grim Reaper's Sprint | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 89 | Civic Gardener | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 90 | Plargg, Dean of Chaos // Augusta, Dean of Order | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on you_attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 91 | Raggadragga, Goreguts Boss | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 92 | Merrow Commerce | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 93 | The Watcher in the Water | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on tribe_you_control_dies → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 94 | Jangling Automaton | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 95 | Cat-Owl | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 96 | Jorn, God of Winter // Kaldring, the Rimestaff | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on attack → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 97 | Silkenfist Fighter | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on becomes_blocked → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 98 | Hyrax Tower Scout | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on etb → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 99 | Deepchannel Duelist | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| 100 | Voltaic Servant | Selesnya Sanctuary | 0.90 | untap-trigger (triggered on phase → untap) refunds mana engine ({T} (cmc 0) → 2 mana) |
| … | | | | +375705 more in JSON |
