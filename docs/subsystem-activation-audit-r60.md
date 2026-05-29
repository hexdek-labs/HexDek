# r60 Subsystem Activation Audit (Probe D)

Audit of `data/rules/oracle-cards.json` (Scryfall bulk default-cards snapshot, 37,384 records, scanned 2026-05-29) for cards that **activate** one of the 10 lazy-init subsystems planned for the r60 dormant-hooks registry. A card is treated as an *activator* if its oracle text causes the subsystem to become live for the game (first venture, first {E} produced, keyword printed, etc.). Cards that merely *consume* an already-live subsystem (`pay {E}`, `if you have the city's blessing`, `while you're the monarch`) are deliberately excluded — the engine never needs to wake the subsystem on their behalf.

**Method**: per-subsystem activator regex against `oracle_text`, deduped by `oracle_id` (drops reprints), DFC faces scanned independently, non-en / memorabilia / funny / token / emblem layouts excluded, activation event classified by line-shape heuristic (keyword / ETB / trigger / activated / phase).

## Summary

| Subsystem | Activator cards |
|---|---:|
| DayNight | 86 |
| Monarch | 50 |
| Initiative | 23 |
| Ascend | 31 |
| Dungeons | 46 |
| RingTempts | 54 |
| Energy | 118 |
| Experience | 15 |
| Foretell | 61 |
| CitysBlessing | 0 |
| **Total (sum, before cross-subsystem dedupe)** | **484** |

## DayNight (86 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| Angel of Eternal Dawn | YMID | Creature — Angel | ETB_trigger | When this creature enters, it becomes day. |
| Arlinn, the Moon's Fury | MID | Legendary Planeswalker — Arlinn | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Arlinn, the Pack's Hope | MID | Legendary Planeswalker — Arlinn | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Avabruck Caretaker | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Ballista Watcher | VOW | Creature — Human Soldier Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Ballista Wielder | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Baneblade Scoundrel | MID | Creature — Human Rogue Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Baneclaw Marauder | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Bird Admirer | MID | Creature — Human Archer Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Blossom-Clad Werewolf | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Brimstone Vandal | MID | Creature — Devil | ETB_replacement | If it's neither day nor night, it becomes day as this creature enters. |
| Brutal Cathar | MID | Creature — Human Soldier Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Burly Breaker | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Celestus Sanctifier | MID | Creature — Human Cleric | ETB_replacement | If it's neither day nor night, it becomes day as this creature enters. |
| Child of the Pack | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Component Collector | MID | Creature — Homunculus | ETB_replacement | If it's neither day nor night, it becomes day as this creature enters. |
| Curse of Leeches | MID | Enchantment — Aura Curse | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Dire-Strain Anarchist | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Dire-Strain Brawler | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Dire-Strain Demolisher | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Fangblade Brigand | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Fangblade Eviscerator | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Fearful Villager | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Fearsome Werewolf | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Firmament Sage | MID | Creature — Human Wizard | ETB_replacement | If it's neither day nor night, it becomes day as this creature enters. |
| Frenzied Trapbreaker | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Gavony Dawnguard | MID | Creature — Human Soldier | ETB_replacement | If it's neither day nor night, it becomes day as this creature enters. |
| Graveyard Glutton | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Graveyard Trespasser | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Harvesttide Assailant | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Harvesttide Infiltrator | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Hollowhenge Huntmaster | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Hookhand Mariner | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Hound Tamer | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Howlpack Avenger | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Howlpack Piper | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Ill-Tempered Loner | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Infestation Expert | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Infested Werewolf | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Into the Night | VOW | Sorcery | resolve_or_static | It becomes night. Discard any number of cards, then draw that many cards plus one. |
| Kessig Naturalist | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Lambholt Raconteur | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Lambholt Ravager | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Leeching Lurker | MID | Creature — Leech Horror | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Lord of the Ulvenwald | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Moonlit Ambusher | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Moonrage Brute | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Oakshade Stalker | VOW | Creature — Human Ranger Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Obsessive Astronomer | MID | Creature — Human Wizard | ETB_replacement | If it's neither day nor night, it becomes day as this creature enters. |
| Outland Liberator | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Rahilda, Feral Outlaw | YMID | Legendary Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Rahilda, Wanted Cutthroat | YMID | Legendary Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Reckless Stormseeker | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Riphook Raider | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Savage Packmate | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Seafaring Werewolf | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Shady Traveler | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Spellrune Howler | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Spellrune Painter | MID | Creature — Human Shaman Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Stalking Predator | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Storm-Charged Slasher | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Sunrise Cavalier | MID | Creature — Human Knight | ETB_replacement | If it's neither day nor night, it becomes day as this creature enters. |
| Sunstreak Phoenix | MID | Creature — Phoenix | ETB_replacement | If it's neither day nor night, it becomes day as this creature enters. |
| Suspicious Stowaway | MID | Creature — Human Rogue Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Tavern Ruffian | MID | Creature — Human Warrior Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Tavern Smasher | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| The Celestus | MID | Legendary Artifact | ETB_replacement | If it's neither day nor night, it becomes day as The Celestus enters. |
| Tireless Hauler | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Tovolar's Huntmaster | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Tovolar's Packleader | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Tovolar, Dire Overlord | MID | Legendary Creature — Human Werewolf | upkeep_trigger | At the beginning of your upkeep, if you control three or more Wolves and/or Werewolves, it becomes night. Then transform any number of Human Werewolves you control. |
| Tovolar, the Midnight Scourge | MID | Legendary Creature — Werewolf | keyword_ability | Nightbound |
| Unnatural Moonrise | MID | Sorcery | resolve_or_static | It becomes night. Until end of turn, target creature gets +1/+0 and gains trample and "Whenever this creature deals combat damage to a player, draw a card." |
| Untamed Pup | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Vadrik, Astral Archmage | MID | Legendary Creature — Human Wizard | ETB_replacement | If it's neither day nor night, it becomes day as Vadrik enters. |
| Village Reavers | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Village Watch | MID | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Volatile Arsonist | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Weary Prisoner | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Weaver of Blossoms | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Wedding Crasher | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Werewhat | MB2 | Creature — Werewolf | keyword_ability | Daybound |
| Wildsong Howler | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Wing Shredder | MID | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |
| Wolfkin Outcast | VOW | Creature — Human Werewolf | keyword_ability | Daybound (If a player casts no spells during their own turn, it becomes night next turn.) |
| Wrathful Jailbreaker | VOW | Creature — Werewolf | keyword_ability | Nightbound (If a player casts at least two spells during their own turn, it becomes day next turn.) |

## Monarch (50 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| Aragorn, King of Gondor | LTC | Legendary Creature — Human Noble | ETB_trigger | When Aragorn enters, you become the monarch. |
| Archivist of Gondor | LTC | Creature — Human Advisor | combat_damage_trigger | When your commander deals combat damage to a player, if there is no monarch, you become the monarch. |
| Archon of Coronation | NCC | Creature — Archon | ETB_trigger | When this creature enters, you become the monarch. |
| Azure Fleet Admiral | LCC | Creature — Human Pirate | ETB_trigger | When this creature enters, you become the monarch. |
| Canal Courier | CN2 | Creature — Human Rogue | ETB_trigger | When this creature enters, you become the monarch. |
| Champions of Minas Tirith | LTC | Creature — Human Soldier | ETB_trigger | When this creature enters, you become the monarch. |
| Coin of Fate | FIC | Artifact | resolve_or_static | {3}{W}, {T}, Exile two creature cards from your graveyard, Sacrifice this artifact: An opponent chooses one of the exiled cards. You put that card on the bottom of your library and return the other… |
| Court of Ambition | CMR | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Ardenvale | WOC | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Bounty | CMR | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Cunning | CMR | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Embereth | WOC | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Garenbrig | WOC | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Grace | CMR | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Ire | CMR | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Locthwain | WOC | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Court of Vantress | WOC | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Crimson Fleet Commodore | CMM | Creature — Ogre Pirate | ETB_trigger | When this creature enters, you become the monarch. |
| Crown of Gondor | LTC | Legendary Artifact — Equipment | ETB_trigger | When a legendary creature you control enters, if there is no monarch, you become the monarch. |
| Crown-Hunter Hireling | CN2 | Creature — Ogre Mercenary | ETB_trigger | When this creature enters, you become the monarch. |
| Custodi Lich | NCC | Creature — Zombie Cleric | ETB_trigger | When this creature enters, you become the monarch. |
| Dawnglade Regent | CMR | Creature — Elk | ETB_trigger | When this creature enters, you become the monarch. |
| Denethor, Stone Seer | LTC | Legendary Creature — Human Noble | resolve_or_static | {3}{R}, {T}, Sacrifice Denethor: Target player becomes the monarch. Denethor deals 3 damage to any target. |
| Emberwilde Captain | CMR | Creature — Djinn Pirate | ETB_trigger | When this creature enters, you become the monarch. |
| Entourage of Trest | CMM | Creature — Elf Soldier | ETB_trigger | When this creature enters, you become the monarch. |
| Fall from Favor | CMM | Enchantment — Aura | ETB_trigger | When this Aura enters, tap enchanted creature and you become the monarch. |
| Faramir, Steward of Gondor | LTC | Legendary Creature — Human Noble | ETB_trigger | Whenever a legendary creature you control with mana value 4 or greater enters, you become the monarch. |
| Fealty to the Realm | LTC | Enchantment — Aura | ETB_trigger | When this Aura enters, you become the monarch. |
| Feast of Succession | CMM | Sorcery | resolve_or_static | All creatures get -4/-4 until end of turn. You become the monarch. |
| Forth Eorlingas! | LTC | Sorcery | trigger | Whenever one or more creatures you control deal combat damage to one or more players this turn, you become the monarch. |
| Garland, Royal Kidnapper | FIC | Legendary Creature — Human Knight | ETB_trigger | When Garland enters, target opponent becomes the monarch. |
| Grave Venerations | ECC | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Jared Carthalion, True Heir | CMR | Legendary Creature — Human Warrior | ETB_trigger | When Jared Carthalion enters, target opponent becomes the monarch. You can't become the monarch this turn. |
| Keeper of Keys | CN2 | Creature — Human Rogue Mutant | ETB_trigger | When this creature enters, you become the monarch. |
| Knights of the Black Rose | MOC | Creature — Human Knight | ETB_trigger | When this creature enters, you become the monarch. |
| Marchesa's Decree | CN2 | Enchantment | ETB_trigger | When this enchantment enters, you become the monarch. |
| Oath of Eorl | LTC | Enchantment — Saga | resolve_or_static | III — Put an indestructible counter on up to one target Human. You become the monarch. |
| Palace Jailer | CMM | Creature — Human Soldier | ETB_trigger | When this creature enters, you become the monarch. |
| Palace Sentinels | CMM | Creature — Human Soldier | ETB_trigger | When this creature enters, you become the monarch. |
| Paliano | MOC | Plane — Fiora | trigger | When one or more creatures you control deal combat damage to a player, if there is no monarch, you become the monarch. |
| Protector of the Crown | CN2 | Creature — Giant Soldier | ETB_trigger | When this creature enters, you become the monarch. |
| Queen Marchesa | OTC | Legendary Creature — Human Assassin | ETB_trigger | When Queen Marchesa enters, you become the monarch. |
| Regal Behemoth | LCC | Creature — Dinosaur | ETB_trigger | When this creature enters, you become the monarch. |
| Regal Sliver | CMM | Creature — Sliver | resolve_or_static | Sliver creatures you control have "When this creature enters, Slivers you control get +1/+1 until end of turn if you're the monarch. Otherwise, you become the monarch." |
| Skyline Despot | CMM | Creature — Dragon | ETB_trigger | When this creature enters, you become the monarch. |
| Starscream, Seeker Leader | BOT | Legendary Artifact — Vehicle | combat_damage_trigger | Whenever Starscream deals combat damage to a player, if there is no monarch, that player becomes the monarch. |
| Staunch Throneguard | CMM | Artifact Creature — Construct | ETB_trigger | When this creature enters, you become the monarch. |
| Thorn of the Black Rose | CMM | Creature — Human Assassin | ETB_trigger | When this creature enters, you become the monarch. |
| Throne of the High City | MKC | Land | activated_ability | {4}, {T}, Sacrifice this land: You become the monarch. |
| Éomer, King of Rohan | LTC | Legendary Creature — Human Noble | ETB_trigger | When Éomer enters, target player becomes the monarch. Éomer deals damage equal to its power to any target. |

## Initiative (23 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| Aarakocra Sneak | CLB | Creature — Bird Rogue | ETB_trigger | When this creature enters, you take the initiative. |
| Avenging Hunter | CLB | Creature — Dragon Ranger | ETB_trigger | When this creature enters, you take the initiative. |
| Bloodboil Sorcerer | CLB | Creature — Human Shaman Sorcerer | ETB_trigger | When this creature enters, you take the initiative. |
| Caves of Chaos Adventurer | CLB | Creature — Human Barbarian | ETB_trigger | When this creature enters, you take the initiative. |
| Dungeoneer's Pack | CLB | Artifact | activated_ability | {2}, {T}, Sacrifice this artifact: You take the initiative, gain 3 life, draw a card, and create a Treasure token. Activate only as a sorcery. (A Treasure token is an artifact with "{T}, Sacrifice … |
| Explore the Underdark | CLB | Sorcery | resolve_or_static | You take the initiative. |
| Feywild Caretaker | CLB | Creature — Orc Wizard | ETB_trigger | When this creature enters, you take the initiative. |
| From the Catacombs | CLB | Sorcery | resolve_or_static | Put target creature card from a graveyard onto the battlefield under your control with a corpse counter on it. You take the initiative. If that creature would leave the battlefield, exile it instea… |
| Goliath Paladin | CLB | Creature — Giant Knight | ETB_trigger | When this creature enters, you take the initiative. |
| Loot Dispute | CLB | Enchantment | ETB_trigger | When this enchantment enters, you take the initiative and create a Treasure token. |
| Passageway Seer | CLB | Creature — Tiefling Warlock | ETB_trigger | When this creature enters, you take the initiative. |
| Ravenloft Adventurer | CLB | Creature — Human Rogue Assassin | ETB_trigger | When this creature enters, you take the initiative. |
| Rilsa Rael, Kingpin | CLB | Legendary Creature — Human Rogue | ETB_trigger | When Rilsa Rael enters, you take the initiative. |
| Sarevok's Tome | CLB | Artifact — Book | ETB_trigger | When this artifact enters, you take the initiative. |
| Seasoned Dungeoneer | CLB | Creature — Human Warrior | ETB_trigger | When this creature enters, you take the initiative. |
| Stirring Bard | CLB | Creature — Dragon Bard | ETB_trigger | When this creature enters, you take the initiative. |
| Tomb of Horrors Adventurer | CLB | Creature — Elf Monk | ETB_trigger | When this creature enters, you take the initiative. |
| Trailblazer's Torch | CLB | Artifact — Equipment | ETB_trigger | When this Equipment enters, you take the initiative. |
| Undercellar Sweep | CLB | Enchantment | ETB_trigger | When this enchantment enters, you take the initiative. |
| Underdark Explorer | CLB | Creature — Lizard Warrior | ETB_trigger | When this creature enters, you take the initiative. |
| Undermountain Adventurer | CLB | Creature — Giant Warrior | ETB_trigger | When this creature enters, you take the initiative. |
| Vicious Battlerager | CLB | Creature — Dwarf Barbarian | ETB_trigger | When this creature enters, you take the initiative. |
| White Plume Adventurer | CLB | Creature — Orc Cleric | ETB_trigger | When this creature enters, you take the initiative. |

## Ascend (31 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| A-Ocelot Pride | MH3 | Creature — Cat | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Andúril, Narsil Reforged | LTC | Legendary Artifact — Equipment | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Arch of Orazca | LCC | Land | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Ascend from Avernus | CLB | Sorcery | resolve_or_static | Return all creature and planeswalker cards with mana value X or less from your graveyard to the battlefield. Exile Ascend from Avernus. |
| Deadeye Brawler | RIX | Creature — Human Pirate | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Detective of the Month | MKC | Creature — Human Detective | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Dusk Charger | RIX | Creature — Horse | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Expel from Orazca | RIX | Instant | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Golden Demise | RIX | Sorcery | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Illustrious Wanderglyph | LCC | Artifact Creature — Golem | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Kumena's Awakening | RIX | Enchantment | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Mausoleum Harpy | RIX | Creature — Harpy | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Ocelot Pride | MH3 | Creature — Cat | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Orazca Relic | AFC | Artifact | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Pride of Conquerors | RIX | Instant | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Radiant Destiny | LCC | Enchantment | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Resplendent Griffin | RIX | Creature — Griffin | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Secrets of the Golden City | RIX | Sorcery | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Skymarcher Aspirant | RIX | Creature — Vampire Soldier | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Slippery Scoundrel | RIX | Creature — Human Pirate | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Snubhorn Sentry | RIX | Creature — Dinosaur | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Spire Winder | RIX | Creature — Snake | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Storm Fleet Swashbuckler | RIX | Creature — Human Pirate | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Temur Elevator | MB2 | Land | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Tendershoot Dryad | SOC | Creature — Dryad | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| The Golden City of Orazca | MOC | Plane — Ixalan | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Tilonalli's Summoner | RIX | Creature — Human Shaman | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Timestream Navigator | LCC | Creature — Human Pirate Wizard | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Twilight Prophet | MKC | Creature — Vampire Cleric | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Vona's Hunger | RIX | Instant | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |
| Wayward Swordtooth | LCC | Creature — Dinosaur | keyword_ability | Ascend (If you control ten or more permanents, you get the city's blessing for the rest of the game.) |

## Dungeons (46 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| A-Acererak the Archlich | AFR | Legendary Creature — Zombie Wizard | ETB_trigger | When Acererak the Archlich enters, if you haven't completed Tomb of Annihilation, return Acererak the Archlich to its owner's hand and venture into the dungeon. |
| A-Cloister Gargoyle | AFR | Artifact Creature — Gargoyle | ETB_trigger | When Cloister Gargoyle enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| A-Dungeon Descent | AFR | Land | activated_ability | {1}, {T}, Tap an untapped legendary creature you control: Venture into the dungeon. Activate only as a sorcery. |
| A-Ellywick Tumblestrum | AFR | Legendary Planeswalker — Ellywick | resolve_or_static | +1: Venture into the dungeon. (Enter the first room or advance to the next room.) |
| A-Fates' Reversal | AFR | Sorcery | resolve_or_static | Return up to one target creature card from your graveyard to your hand. Venture into the dungeon. (Enter the first room or advance to the next room.) |
| A-Find the Path | AFR | Enchantment — Aura | ETB_trigger | When Find the Path enters, venture into the dungeon. |
| A-Precipitous Drop | AFR | Enchantment — Aura | ETB_trigger | When Precipitous Drop enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| A-Triumphant Adventurer | AFR | Creature — Human Knight | attack_trigger | Whenever Triumphant Adventurer attacks, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Acererak the Archlich | AFR | Legendary Creature — Zombie Wizard | ETB_trigger | When Acererak enters, if you haven't completed Tomb of Annihilation, return Acererak to its owner's hand and venture into the dungeon. |
| Bar the Gate | AFR | Instant | resolve_or_static | Counter target creature or planeswalker spell. Venture into the dungeon. (Enter the first room or advance to the next room.) |
| Barrowin of Clan Undurr | AFR | Legendary Creature — Dwarf Cleric | ETB_trigger | When Barrowin enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Clattering Skeletons | AFR | Creature — Skeleton | dies_trigger | When this creature dies, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Cloister Gargoyle | AFR | Artifact Creature — Gargoyle | ETB_trigger | When this creature enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Delver's Torch | AFR | Artifact — Equipment | attack_trigger | Whenever equipped creature attacks, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Displacer Beast | AFR | Creature — Cat Beast | ETB_trigger | When this creature enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Dungeon Descent | AFR | Land | activated_ability | {4}, {T}, Tap an untapped legendary creature you control: Venture into the dungeon. Activate only as a sorcery. (Enter the first room or advance to the next room.) |
| Dungeon Map | AFR | Artifact | activated_ability | {3}, {T}: Venture into the dungeon. Activate only as a sorcery. (Enter the first room or advance to the next room.) |
| Eccentric Apprentice | AFR | Creature — Tiefling Wizard | ETB_trigger | When this creature enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Ellywick Tumblestrum | AFR | Legendary Planeswalker — Ellywick | resolve_or_static | +1: Venture into the dungeon. (Enter the first room or advance to the next room.) |
| Fates' Reversal | AFR | Sorcery | resolve_or_static | Return up to one target creature card from your graveyard to your hand. Venture into the dungeon. (Enter the first room or advance to the next room.) |
| Fifty Feet of Rope | AFR | Artifact | resolve_or_static | Rappel Down — {4}, {T}: Venture into the dungeon. Activate only as a sorcery. (Enter the first room or advance to the next room.) |
| Find the Path | AFR | Enchantment — Aura | ETB_trigger | When this Aura enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Fly | AFR | Enchantment — Aura | resolve_or_static | Enchanted creature has flying and "Whenever this creature deals combat damage to a player, venture into the dungeon." (Enter the first room or advance to the next room.) |
| Immovable Rod | AFC | Artifact | trigger | Whenever this artifact becomes untapped, venture into the dungeon. |
| Intrepid Outlander | AFR | Creature — Orc Ranger | resolve_or_static | Pack tactics — Whenever this creature attacks, if you attacked with creatures with total power 6 or greater this combat, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Keen-Eared Sentry | AFR | Creature — Human Soldier | resolve_or_static | Each opponent can't venture into the dungeon more than once each turn. |
| Kick in the Door | AFR | Sorcery | resolve_or_static | Put a +1/+1 counter on target creature. That creature gains haste until end of turn and can't be blocked by Walls this turn. Venture into the dungeon. (Enter the first room or advance to the next r… |
| Midnight Pathlighter | AFC | Creature — Human Wizard | trigger | Whenever one or more creatures you control deal combat damage to a player, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Nadaar, Selfless Paladin | AFR | Legendary Creature — Dragon Knight | ETB_trigger | Whenever Nadaar enters or attacks, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Planar Ally | AFR | Creature — Angel | attack_trigger | Whenever this creature attacks, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Precipitous Drop | AFR | Enchantment — Aura | ETB_trigger | When this Aura enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Radiant Solar | AFC | Creature — Angel | ETB_trigger | Whenever this creature or another nontoken creature you control enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Ranger's Hawk | AFR | Creature — Bird | activated_ability | {3}, {T}, Tap another untapped creature you control: Venture into the dungeon. Activate only as a sorcery. (Enter the first room or advance to the next room.) |
| Secret Door | AFR | Artifact Creature — Wall | resolve_or_static | {4}{U}: Venture into the dungeon. Activate only as a sorcery. (Enter the first room or advance to the next room.) |
| Sefris of the Hidden Ways | AFC | Legendary Creature — Human Wizard | trigger | Whenever one or more creature cards are put into your graveyard from anywhere, venture into the dungeon. This ability triggers only once each turn. (To venture into the dungeon, enter the first roo… |
| Shortcut Seeker | AFR | Creature — Human Rogue | combat_damage_trigger | Whenever this creature deals combat damage to a player, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Thorough Investigation | AFC | Enchantment | trigger | Whenever you sacrifice a Clue, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Triumphant Adventurer | AFR | Creature — Human Knight | attack_trigger | Whenever this creature attacks, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Varis, Silverymoon Ranger | AFR | Legendary Creature — Human Elf Ranger | cast_trigger | Whenever you cast a creature or planeswalker spell, venture into the dungeon. This ability triggers only once each turn. (To venture into the dungeon, enter the first room or advance to the next ro… |
| Veteran Dungeoneer | AFR | Creature — Human Warrior | ETB_trigger | When this creature enters, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Wandering Troubadour | AFR | Creature — Dragon Bard | phase_trigger | At the beginning of your end step, if you had a land enter the battlefield under your control this turn, venture into the dungeon. (Enter the first room or advance to the next room.) |
| You Find a Cursed Idol | AFR | Sorcery | resolve_or_static | • Steal Its Eyes — Create a Treasure token and venture into the dungeon. (Enter the first room or advance to the next room.) |
| Yuan-Ti Fang-Blade | AFR | Creature — Snake Rogue | combat_damage_trigger | Whenever this creature deals combat damage to a player, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Yuan-Ti Malison | AFR | Creature — Snake Rogue | combat_damage_trigger | Whenever this creature deals combat damage to a player, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Zalto, Fire Giant Duke | AFR | Legendary Creature — Giant Barbarian | damage_trigger | Whenever Zalto is dealt damage, venture into the dungeon. (Enter the first room or advance to the next room.) |
| Zombie Ogre | AFR | Creature — Zombie Ogre | phase_trigger | At the beginning of your end step, if a creature died this turn, venture into the dungeon. (Enter the first room or advance to the next room.) |

## RingTempts (54 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| Aragorn, Company Leader | LTR | Legendary Creature — Human Ranger | trigger | Whenever the Ring tempts you, if you chose a creature other than Aragorn as your Ring-bearer, put your choice of a counter from among first strike, vigilance, deathtouch, and lifelink on Aragorn. |
| Bilbo, Retired Burglar | LTR | Legendary Creature — Halfling Rogue | ETB_trigger | When Bilbo enters or leaves the battlefield, the Ring tempts you. |
| Birthday Escape | LTR | Sorcery | resolve_or_static | Draw a card. The Ring tempts you. |
| Bombadil's Song | LTR | Instant | resolve_or_static | Target creature you control gets +1/+1 and gains hexproof until end of turn. The Ring tempts you. (A creature with hexproof can't be the target of spells or abilities your opponents control.) |
| Boromir, Warden of the Tower | LTR | Legendary Creature — Human Soldier | resolve_or_static | Sacrifice Boromir: Creatures you control gain indestructible until end of turn. The Ring tempts you. |
| Breaking of the Fellowship | LTR | Sorcery | resolve_or_static | Target creature an opponent controls deals damage equal to its power to another target creature that player controls. The Ring tempts you. |
| Call of the Ring | LTR | Enchantment | upkeep_trigger | At the beginning of your upkeep, the Ring tempts you. |
| Claim the Precious | LTR | Sorcery | resolve_or_static | Destroy target creature. The Ring tempts you. |
| Dreadful as the Storm | LTR | Instant | resolve_or_static | Target creature has base power and toughness 5/5 until end of turn. The Ring tempts you. |
| Dúnedain Rangers | LTR | Creature — Human Ranger | resolve_or_static | Landfall — Whenever a land you control enters, if you don't control a Ring-bearer, the Ring tempts you. |
| Elrond, Lord of Rivendell | LTR | Legendary Creature — Elf Noble | ETB_trigger | Whenever Elrond or another creature you control enters, scry 1. If this is the second time this ability has resolved this turn, the Ring tempts you. |
| Enraged Huorn | LTR | Creature — Treefolk | ETB_trigger | When this creature enters, the Ring tempts you. |
| Faramir, Field Commander | LTR | Legendary Creature — Human Soldier | trigger | Whenever the Ring tempts you, if you chose a creature other than Faramir as your Ring-bearer, create a 1/1 white Human Soldier creature token. |
| Fiery Inscription | LTR | Enchantment | ETB_trigger | When this enchantment enters, the Ring tempts you. |
| Frodo Baggins | LTR | Legendary Creature — Halfling Scout | ETB_trigger | Whenever Frodo Baggins or another legendary creature you control enters, the Ring tempts you. |
| Frodo, Adventurous Hobbit | LTC | Legendary Creature — Halfling Scout | attack_trigger | Whenever Frodo attacks, if you gained 3 or more life this turn, the Ring tempts you. Then if Frodo is your Ring-bearer and the Ring has tempted you two or more times this game, draw a card. |
| Frodo, Sauron's Bane | LTR | Legendary Creature — Halfling Citizen | resolve_or_static | {B}{B}{B}: If Frodo is a Scout, it becomes a Halfling Rogue with "Whenever this creature deals combat damage to a player, that player loses the game if the Ring has tempted you four or more times t… |
| Galadriel of Lothlórien | LTR | Legendary Creature — Elf Noble | trigger | Whenever the Ring tempts you, if you chose a creature other than Galadriel as your Ring-bearer, scry 3. |
| Galadriel, Elven-Queen | LTC | Legendary Creature — Elf Noble | resolve_or_static | Will of the council — At the beginning of combat on your turn, if another Elf entered the battlefield under your control this turn, starting with you, each player votes for dominion or guidance. If… |
| Gandalf, Friend of the Shire | LTR | Legendary Creature — Avatar Wizard | trigger | Whenever the Ring tempts you, if you chose a creature other than Gandalf as your Ring-bearer, draw a card. |
| Glorious Gale | LTR | Instant | resolve_or_static | Counter target creature spell. If it was a legendary spell, the Ring tempts you. |
| Gollum's Bite | LTR | Instant | resolve_or_static | {3}{B}, Exile this card from your graveyard: The Ring tempts you. Activate only as a sorcery. |
| Gollum, Patient Plotter | LTR | Legendary Creature — Halfling Horror | trigger | When Gollum leaves the battlefield, the Ring tempts you. |
| Horses of the Bruinen | LTR | Sorcery | resolve_or_static | Return up to two target creatures to their owners' hands. Scry 1. The Ring tempts you. |
| In the Darkness Bind Them | LTC | Enchantment — Saga | resolve_or_static | I, II, III — Create a 3/3 black Wraith creature token with menace. The Ring tempts you. |
| Inherited Envelope | LTR | Artifact | ETB_trigger | When this artifact enters, the Ring tempts you. |
| Mirrormere Guardian | LTR | Creature — Dwarf Soldier | dies_trigger | When this creature dies, the Ring tempts you. |
| Nazgûl | LTR | Creature — Wraith Knight | ETB_trigger | When this creature enters, the Ring tempts you. |
| Now for Wrath, Now for Ruin! | LTR | Sorcery | resolve_or_static | Put a +1/+1 counter on each creature you control. They gain vigilance until end of turn. The Ring tempts you. |
| One Ring to Rule Them All | LTR | Enchantment — Saga | resolve_or_static | I — The Ring tempts you, then each player mills cards equal to your Ring-bearer's power. |
| Ranger's Firebrand | LTR | Sorcery | resolve_or_static | Ranger's Firebrand deals 2 damage to any target. The Ring tempts you. |
| Rangers of Ithilien | LTR | Creature — Human Ranger | ETB_trigger | When this creature enters, gain control of up to one target creature with lesser power for as long as you control this creature. Then the Ring tempts you. |
| Relentless Rohirrim | LTR | Creature — Human Knight | ETB_trigger | When this creature enters, the Ring tempts you. |
| Ringsight | LTR | Sorcery | resolve_or_static | The Ring tempts you. Search your library for a card that shares a color with a legendary creature you control, reveal it, put it into your hand, then shuffle. |
| Ringwraiths | LTR | Creature — Wraith Knight | trigger | When the Ring tempts you, return this card from your graveyard to your hand. |
| Rohirrim Lancer | LTR | Creature — Human Knight | dies_trigger | When this creature dies, the Ring tempts you. |
| Sam's Desperate Rescue | LTR | Sorcery | resolve_or_static | Return target creature card from your graveyard to your hand. The Ring tempts you. |
| Samwise the Stouthearted | LTR | Legendary Creature — Halfling Peasant | ETB_trigger | When Samwise enters, choose up to one target permanent card in your graveyard that was put there from the battlefield this turn. Return it to your hand. Then the Ring tempts you. |
| Sauron's Ransom | LTR | Instant | resolve_or_static | Choose an opponent. They look at the top four cards of your library and separate them into a face-down pile and a face-up pile. Put one pile into your hand and the other into your graveyard. The Ri… |
| Sauron, Lord of the Rings | LTC | Legendary Creature — Avatar Horror | dies_trigger | Whenever a commander an opponent controls dies, the Ring tempts you. |
| Sauron, the Dark Lord | LTR | Legendary Creature — Avatar Horror | combat_damage_trigger | Whenever an Army you control deals combat damage to a player, the Ring tempts you. |
| Scroll of Isildur | LTR | Enchantment — Saga | resolve_or_static | I — Gain control of up to one target artifact for as long as you control this Saga. The Ring tempts you. |
| Shortcut to Mushrooms | LTR | Enchantment | ETB_trigger | When this enchantment enters, the Ring tempts you. |
| Slip On the Ring | LTR | Instant | resolve_or_static | Exile target creature you own, then return it to the battlefield under your control. The Ring tempts you. |
| Sméagol, Helpful Guide | LTR | Legendary Creature — Halfling Horror | phase_trigger | At the beginning of your end step, if a creature died under your control this turn, the Ring tempts you. |
| Soothing of Sméagol | LTR | Instant | resolve_or_static | Return target nontoken creature to its owner's hand. The Ring tempts you. |
| Stalwarts of Osgiliath | LTR | Creature — Human Soldier | ETB_trigger | When this creature enters, the Ring tempts you. |
| The Black Breath | LTR | Sorcery | resolve_or_static | Creatures your opponents control get -1/-1 until end of turn. The Ring tempts you. |
| The Ring Goes South | LTR | Sorcery | resolve_or_static | The Ring tempts you. Then reveal cards from the top of your library until you reveal X land cards, where X is the number of legendary creatures you control. Put those land cards onto the battlefiel… |
| There and Back Again | LTR | Enchantment — Saga | resolve_or_static | I — Up to one target creature can't block for as long as you control this Saga. The Ring tempts you. |
| Took Reaper | LTR | Creature — Halfling Peasant | dies_trigger | When this creature dies, the Ring tempts you. |
| Uruk-hai Berserker | LTR | Creature — Orc Berserker | ETB_trigger | When this creature enters, the Ring tempts you. |
| War of the Last Alliance | LTR | Enchantment — Saga | resolve_or_static | III — Creatures you control gain double strike until end of turn. The Ring tempts you. |
| Witch-king of Angmar | LTR | Legendary Creature — Wraith Noble | trigger | Whenever one or more creatures deal combat damage to you, each opponent sacrifices a creature of their choice that dealt combat damage to you this turn. The Ring tempts you. |

## Energy (118 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| A-Galvanic Discharge | MH3 | Instant | resolve_or_static | Choose target creature or planeswalker. You get {E}{E}, then you may pay any amount of {E}. Galvanic Discharge deals that much damage to that permanent. |
| Aether Chaser | AER | Creature — Human Artificer | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Aether Herder | AER | Creature — Elf Artificer Druid | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Aether Hub | KLD | Land | ETB_trigger | When this land enters, you get {E} (an energy counter). |
| Aether Inspector | AER | Creature — Dwarf Artificer | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Aether Meltdown | KLD | Enchantment — Aura | ETB_trigger | When this Aura enters, you get {E}{E} (two energy counters). |
| Aether Poisoner | AER | Creature — Human Artificer | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Aether Refinery | M3C | Artifact | activated_ability | {T}: You get {E}, then you may pay one or more {E}. If you do, create an X/X black Aetherborn creature token, where X is the amount of {E} paid this way. |
| Aether Spike | MH3 | Instant | resolve_or_static | Choose target spell. You get {E}{E} (two energy counters), then you may pay any amount of {E}. Counter that spell unless its controller pays {1} for each {E} paid this way. |
| Aether Swooper | AER | Creature — Vedalken Artificer | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Aether Theorist | KLD | Creature — Vedalken Rogue | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Aethergeode Miner | AER | Creature — Dwarf Scout | attack_trigger | Whenever this creature attacks, you get {E}{E} (two energy counters). |
| Aethersphere Harvester | AER | Artifact — Vehicle | ETB_trigger | When this Vehicle enters, you get {E}{E} (two energy counters). |
| Aethersquall Ancient | KLD | Creature — Leviathan | upkeep_trigger | At the beginning of your upkeep, you get {E}{E}{E} (three energy counters). |
| Aetherstorm Roc | KLD | Creature — Bird | ETB_trigger | Whenever this creature or another creature you control enters, you get {E} (an energy counter). |
| Aetherstream Leopard | AER | Creature — Cat | ETB_trigger | When this creature enters, you get {E} (an energy counter). |
| Aethertorch Renegade | KLD | Creature — Human Rogue | ETB_trigger | When this creature enters, you get {E}{E}{E}{E} (four energy counters). |
| Aetherwind Basker | AER | Creature — Lizard | ETB_trigger | Whenever this creature enters or attacks, you get {E} (an energy counter) for each creature you control. |
| Aetherworks Marvel | KLD | Legendary Artifact | trigger | Whenever a permanent you control is put into a graveyard, you get {E} (an energy counter). |
| Amped Raptor | MH3 | Creature — Dinosaur | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). Then if you cast it from your hand, exile cards from the top of your library until you exile a nonland card. You may cast that card … |
| Architect of the Untamed | KLD | Creature — Elf Artificer Druid | resolve_or_static | Landfall — Whenever a land you control enters, you get {E} (an energy counter). |
| Assaultron Dominator | PIP | Artifact Creature — Robot | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Attune with Aether | KLD | Sorcery | resolve_or_static | Search your library for a basic land card, reveal it, put it into your hand, then shuffle. You get {E}{E} (two energy counters). |
| Automated Assembly Line | PIP | Artifact | trigger | Whenever one or more artifact creatures you control deal combat damage to a player, you get {E} (an energy counter). |
| Behemoth of Vault 0 | PIP | Artifact Creature — Robot | ETB_trigger | When this creature enters, you get {E}{E}{E}{E} (four energy counters). |
| Bespoke Battlewagon | MH3 | Artifact — Vehicle | activated_ability | {T}: You get {E}{E} (two energy counters). |
| Blaster Hulk | M3C | Artifact Creature — Pirate | attack_trigger | Whenever this creature attacks, you get {E}{E}, then you may pay eight {E}. When you do, this creature deals 8 damage divided as you choose among up to eight targets. |
| Bristling Hydra | KLD | Creature — Hydra | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Brotherhood Scribe | PIP | Creature — Human Artificer | resolve_or_static | Metalcraft — {T}: You get {E} (an energy counter). Activate only if you control three or more artifacts. |
| Chthonian Nightmare | MH3 | Enchantment | ETB_trigger | When this enchantment enters, you get {E}{E}{E} (three energy counters). |
| Conduit Goblin | MH3 | Creature — Goblin Warrior | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Confiscation Coup | KLD | Sorcery | resolve_or_static | Choose target artifact or creature. You get {E}{E}{E}{E} (four energy counters), then you may pay an amount of {E} equal to that permanent's mana value. If you do, gain control of it. |
| Consul's Shieldguard | KLD | Creature — Dwarf Soldier | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Consulate Surveillance | KLD | Enchantment | ETB_trigger | When this enchantment enters, you get {E}{E}{E}{E} (four energy counters). |
| Consulate Turret | AER | Artifact | activated_ability | {T}: You get {E} (an energy counter). |
| Conversion Apparatus | M3C | Artifact | activated_ability | {3}, {T}: You get {E}{E}{E} (three energy counters). |
| Cyclops Superconductor | MH3 | Creature — Cyclops Wizard | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Deadlock Trap | KLD | Artifact | ETB_trigger | When this artifact enters, you get {E}{E} (two energy counters). |
| Decoction Module | KLD | Artifact | ETB_trigger | Whenever a creature you control enters, you get {E} (an energy counter). |
| Demon of Dark Schemes | KLD | Creature — Demon | dies_trigger | Whenever another creature dies, you get {E} (an energy counter). |
| Die Young | KLD | Sorcery | resolve_or_static | Choose target creature. You get {E}{E} (two energy counters), then you may pay any amount of {E}. The creature gets -1/-1 until end of turn for each {E} paid this way. |
| Dr. Madison Li | PIP | Legendary Creature — Human Scientist | cast_trigger | Whenever you cast an artifact spell, you get {E} (an energy counter). |
| Dynavolt Tower | KLD | Artifact | cast_trigger | Whenever you cast an instant or sorcery spell, you get {E}{E} (two energy counters). |
| Eddytrail Hawk | KLD | Creature — Bird | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Electrostatic Pummeler | KLD | Artifact Creature — Construct | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Electrozoa | MH3 | Creature — Jellyfish | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Emissary of Soulfire | MH3 | Creature — Djinn Monk | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Era of Innovation | KLD | Enchantment | ETB_trigger | Whenever an artifact or Artificer you control enters, you may pay {1}. If you do, you get {E}{E} (two energy counters). |
| Fabrication Module | KLD | Artifact | activated_ability | {4}, {T}: You get {E}. |
| Filigree Racer | M3C | Artifact — Vehicle | ETB_trigger | When this Vehicle enters, you get {E}{E}{E}{E} (four energy counters). |
| Galvanic Discharge | MH3 | Instant | resolve_or_static | Choose target creature or planeswalker. You get {E}{E}{E} (three energy counters), then you may pay any amount of {E}. Galvanic Discharge deals that much damage to that permanent. |
| Glassblower's Puzzleknot | KLD | Artifact | ETB_trigger | When this artifact enters, scry 2, then you get {E}{E}. (You get two energy counters. To scry 2, look at the top two cards of your library, then put any number of them on the bottom and the rest on… |
| Glimmer of Genius | KLD | Instant | resolve_or_static | Scry 2, then draw two cards. You get {E}{E} (two energy counters). |
| Glint-Sleeve Siphoner | AER | Creature — Human Rogue | ETB_trigger | Whenever this creature enters or attacks, you get {E} (an energy counter). |
| Gonti's Aether Heart | AER | Legendary Artifact | ETB_trigger | Whenever Gonti's Aether Heart or another artifact you control enters, you get {E}{E} (two energy counters). |
| Gonti's Machinations | AER | Enchantment | trigger | Whenever you lose life for the first time each turn, you get {E}. (You get an energy counter. Damage causes loss of life.) |
| Greenbelt Rampager | AER | Creature — Elephant | ETB_trigger | When this creature enters, pay {E}{E} (two energy counters). If you can't, return this creature to its owner's hand and you get {E}. |
| HELIOS One | PIP | Land | activated_ability | {1}, {T}: You get {E} (an energy counter). |
| Harnessed Lightning | KLD | Instant | resolve_or_static | Choose target creature. You get {E}{E}{E} (three energy counters), then you may pay any amount of {E}. Harnessed Lightning deals that much damage to that creature. |
| Hexgold Slith | MH3 | Creature — Slith | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Highspire Infusion | AER | Instant | resolve_or_static | Target creature gets +3/+3 until end of turn. You get {E}{E} (two energy counters). |
| Hightide Hermit | KLD | Creature — Crab | ETB_trigger | When this creature enters, you get {E}{E}{E}{E} (four energy counters). |
| Inspired Inventor | MH3 | Creature — Human Artificer | resolve_or_static | • You get {E}{E}{E} (three energy counters). |
| Inventor's Axe | MH3 | Artifact — Equipment | ETB_trigger | When this Equipment enters, you get {E}{E} (two energy counters). |
| Janjeet Sentry | KLD | Creature — Vedalken Soldier | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Jolted Awake | MH3 | Sorcery | resolve_or_static | Choose up to one target artifact or creature card in your graveyard. You get {E}{E} (two energy counters). Then you may pay an amount of {E} equal to that card's mana value. If you do, return it fr… |
| Lathnu Hellion | KLD | Creature — Hellion | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Liberty Prime, Recharged | PIP | Legendary Artifact Creature — Robot | activated_ability | {2}, {T}, Sacrifice an artifact: You get {E}{E} and draw a card. |
| Lightning Runner | AER | Creature — Human Warrior | attack_trigger | Whenever this creature attacks, you get {E}{E} (two energy counters), then you may pay eight {E}. If you pay, untap all creatures you control, and after this phase, there is an additional combat ph… |
| Localized Destruction | M3C | Sorcery | resolve_or_static | You get {E} (an energy counter), then you may pay one or more {E}. If you do, each creature you control with power equal to the amount of {E} paid this way gains indestructible until end of turn. |
| Longtusk Cub | KLD | Creature — Cat | combat_damage_trigger | Whenever this creature deals combat damage to a player, you get {E}{E} (two energy counters). |
| Longtusk Stalker | J21 | Creature — Cat | ETB_trigger | Whenever this creature enters or attacks, you get {E}. |
| Maulfist Doorbuster | KLD | Creature — Human Warrior | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Maximus, Knight Apparent | SLD | Legendary Creature — Human Knight | activated_ability | {1}, Sacrifice an artifact: You get {E}{E} (two energy counters). |
| Minister of Inquiries | KLD | Creature — Vedalken Advisor | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Multiform Wonder | KLD | Artifact Creature — Construct | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Nissa, Worldsoul Speaker | DRC | Legendary Creature — Elf Druid | resolve_or_static | Landfall — Whenever a land you control enters, you get {E}{E} (two energy counters). |
| Phyrexian Ironworks | MH3 | Artifact | attack_trigger | Whenever you attack, you get {E} (an energy counter). |
| Pia Nalaar, Chief Mechanic | DRC | Legendary Creature — Human Artificer | trigger | Whenever one or more artifact creatures you control deal combat damage to a player, you get {E}{E} (two energy counters). |
| Plasma Caster | PIP | Artifact — Equipment | attack_trigger | Whenever equipped creature attacks, you get {E}{E} (two energy counters). |
| Primal Prayers | MH3 | Enchantment | ETB_trigger | When this enchantment enters, you get {E}{E} (two energy counters). |
| Razorfield Ripper | M3C | Artifact Creature — Equipment Rhino | attack_trigger | Whenever this creature or equipped creature attacks, you get {E} (an energy counter), then it gets +X/+X until end of turn, where X is the amount of {E} you have. |
| Rex, Cyber-Hound | PIP | Legendary Artifact Creature — Robot Dog | combat_damage_trigger | Whenever Rex deals combat damage to a player, they mill two cards and you get {E}{E} (two energy counters). |
| Riddle Gate Gargoyle | MH3 | Artifact Creature — Gargoyle | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Riparian Tiger | KLD | Creature — Cat | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Rogue Refiner | AER | Creature — Human Rogue | ETB_trigger | When this creature enters, draw a card and you get {E}{E} (two energy counters). |
| Roil Cartographer | MH3 | Creature — Merfolk Rogue | resolve_or_static | Landfall — Whenever a land you control enters, you get {E} (an energy counter). |
| Sage of Shaila's Claim | KLD | Creature — Elf Druid | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Saheeli, Radiant Creator | DRC | Legendary Creature — Human Artificer | cast_trigger | Whenever you cast an Artificer or artifact spell, you get {E} (an energy counter). |
| Satya, Aetherflux Genius | M3C | Legendary Creature — Human Artificer | attack_trigger | Whenever Satya attacks, create a tapped and attacking token that's a copy of up to one other target nontoken creature you control. You get {E}{E} (two energy counters). At the beginning of the next… |
| Scrapper Champion | AER | Creature — Human Artificer | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Sentry Bot | PIP | Artifact Creature — Robot | ETB_trigger | When this creature enters, you get {E} for each creature attacking you. |
| Servant of the Conduit | KLD | Creature — Elf Druid | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Shielded Aether Thief | AER | Creature — Vedalken Rogue | trigger | Whenever this creature blocks, you get {E} (an energy counter). |
| Shipwreck Moray | AER | Creature — Fish | ETB_trigger | When this creature enters, you get {E}{E}{E}{E} (four energy counters). |
| Smelted Chargebug | MH3 | Artifact Creature — Insect | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Solar Transformer | MH3 | Artifact | ETB_trigger | When this artifact enters, you get {E}{E}{E} (three energy counters). |
| Solstice Zealot | MH3 | Creature — Rhino Cleric | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Spontaneous Artist | KLD | Creature — Human Rogue | ETB_trigger | When this creature enters, you get {E} (an energy counter). |
| Static Prison | MH3 | Enchantment | ETB_trigger | When this enchantment enters, exile target nonland permanent an opponent controls until this enchantment leaves the battlefield. You get {E}{E} (two energy counters). |
| Stone Idol Generator | M3C | Artifact | attack_trigger | Whenever a creature you control attacks, you get {E} (an energy counter). |
| T-45 Power Armor | SLD | Artifact — Equipment | ETB_trigger | When this Equipment enters, you get {E}{E} (two energy counters). |
| Tempest Harvester | MH3 | Creature — Merfolk Wizard | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Territorial Aetherkite | DRC | Creature — Cat Dragon | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). Then you may pay one or more {E}. When you do, this creature deals that much damage to each other creature. |
| Thriving Grubs | KLD | Creature — Gremlin | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Thriving Ibex | KLD | Creature — Goat | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Thriving Rats | KLD | Creature — Rat | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Thriving Rhino | KLD | Creature — Rhino | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Thriving Skyclaw | MH3 | Creature — Cat Dragon | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Thriving Turtle | KLD | Creature — Turtle | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Tune the Narrative | MH3 | Instant | resolve_or_static | Draw a card. You get {E}{E} (two energy counters). |
| Unstable Amulet | MH3 | Artifact | ETB_trigger | When this artifact enters, you get {E}{E} (two energy counters). |
| Vault 112: Sadistic Simulation | PIP | Enchantment — Saga | resolve_or_static | I, II — Tap up to one target creature and put a stun counter on it. You get {E}{E} (two energy counters). |
| Volatile Stormdrake | MH3 | Creature — Drake | ETB_trigger | When this creature enters, exchange control of this creature and target creature an opponent controls. If you do, you get {E}{E}{E}{E}, then sacrifice that creature unless you pay an amount of {E} … |
| Voltaic Brawler | KLD | Creature — Human Warrior | ETB_trigger | When this creature enters, you get {E}{E} (two energy counters). |
| Voltstorm Angel | MH3 | Creature — Angel | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |
| Wheel of Potential | MH3 | Sorcery | resolve_or_static | You get {E}{E}{E} (three energy counters), then you may pay any amount of {E}. |
| Whirler Virtuoso | KLD | Creature — Vedalken Artificer | ETB_trigger | When this creature enters, you get {E}{E}{E} (three energy counters). |

## Experience (15 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| Aang, Airbending Master | TLE | Legendary Creature — Human Avatar Ally | trigger | Whenever one or more creatures you control leave the battlefield without dying, you get an experience counter. |
| Azlask, the Swelling Scourge | M3C | Legendary Creature — Eldrazi | dies_trigger | Whenever Azlask or another colorless creature you control dies, you get an experience counter. |
| Azula, Ruthless Firebender | TLE | Legendary Creature — Human Noble | attack_trigger | Whenever Azula attacks, you may discard a card. Then you get an experience counter for each player who discarded a card this turn. |
| Daxos the Returned | C15 | Legendary Creature — Zombie Soldier | cast_trigger | Whenever you cast an enchantment spell, you get an experience counter. |
| Ezuri, Claw of Progress | 2X2 | Legendary Creature — Phyrexian Elf Warrior | ETB_trigger | Whenever a creature you control with power 2 or less enters, you get an experience counter. |
| Kalemne, Disciple of Iroas | CM2 | Legendary Creature — Giant Soldier | cast_trigger | Whenever you cast a creature spell with mana value 5 or greater, you get an experience counter. |
| Katara, Waterbending Master | TLE | Legendary Creature — Human Warrior Ally | cast_trigger | Whenever you cast a spell during an opponent's turn, you get an experience counter. |
| Kelsien, the Plague | C20 | Legendary Creature — Human Assassin | activated_ability | {T}: Kelsien deals 1 damage to target creature you don't control. When that creature dies this turn, you get an experience counter. |
| Kratos, Stoic Father | SLD | Legendary Creature — God Warrior | attack_trigger | Whenever you attack with one or more Gods and whenever a God dies, you get an experience counter. |
| Meren of Clan Nel Toth | TDC | Legendary Creature — Human Shaman | dies_trigger | Whenever another creature you control dies, you get an experience counter. |
| Minthara, Merciless Soul | CLB | Legendary Creature — Elf Cleric | phase_trigger | At the beginning of your end step, if a permanent you controlled left the battlefield this turn, you get an experience counter. |
| Mizzix of the Izmagnus | CMM | Legendary Creature — Goblin Wizard | cast_trigger | Whenever you cast an instant or sorcery spell with mana value greater than the number of experience counters you have, you get an experience counter. |
| Otharri, Suns' Glory | ONC | Legendary Creature — Phoenix | attack_trigger | Whenever Otharri attacks, you get an experience counter. Then create a 2/2 red Rebel creature token that's tapped and attacking for each experience counter you have. |
| Toph, Earthbending Master | TLE | Legendary Creature — Human Warrior Ally | resolve_or_static | Landfall — Whenever a land you control enters, you get an experience counter. |
| Zuko, Firebending Master | TLE | Legendary Creature — Human Noble Ally | cast_trigger | Whenever you cast a spell during combat, you get an experience counter. |

## Foretell (61 cards)

| Card | Set | Type | Activation | Oracle snippet |
|---|---|---|---|---|
| A-Cosmos Charger | KHM | Creature — Horse Spirit | keyword_ability | Foretell {U} |
| A-Return Upon the Tide | KHM | Sorcery | keyword_ability | Foretell {3}{B} |
| Alrund's Epiphany | KHM | Sorcery | keyword_ability | Foretell {4}{U}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Augury Raven | KHM | Creature — Bird | keyword_ability | Foretell {1}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Battle Mammoth | CLB | Creature — Elephant | keyword_ability | Foretell {2}{G}{G} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Behold the Multiverse | KHM | Instant | keyword_ability | Foretell {1}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Bohn, Beguiling Balladeer | SLX | Legendary Creature — Human Bard | resolve_or_static | Each nonland card in your hand without foretell has foretell. Its foretell cost is equal to its mana cost reduced by {2}. (During your turn, you may pay {2} and exile it from your hand face down. C… |
| Cosmic Intervention | KHC | Instant | keyword_ability | Foretell {1}{W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Cosmos Charger | KHM | Creature — Horse Spirit | keyword_ability | Foretell {2}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Crush the Weak | KHM | Sorcery | keyword_ability | Foretell {R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Delayed Blast Fireball | CLB | Instant | keyword_ability | Foretell {4}{R}{R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Demon Bolt | CLB | Instant | keyword_ability | Foretell {R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Depart the Realm | KHM | Instant | keyword_ability | Foretell {U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Doomskar | KHM | Sorcery | keyword_ability | Foretell {1}{W}{W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Doomskar Oracle | KHM | Creature — Human Cleric | keyword_ability | Foretell {W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Doomskar Titan | KHM | Creature — Giant Berserker | keyword_ability | Foretell {4}{R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Dream Devourer | KHM | Creature — Demon Cleric | resolve_or_static | Each nonland card in your hand without foretell has foretell. Its foretell cost is equal to its mana cost reduced by {2}. (During your turn, you may pay {2} and exile it from your hand face down. C… |
| Dual Strike | KHM | Instant | keyword_ability | Foretell {R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Dwarven Reinforcements | KHM | Sorcery | keyword_ability | Foretell {1}{R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Ethereal Valkyrie | KHC | Creature — Spirit Angel | ETB_trigger | Whenever this creature enters or attacks, draw a card, then exile a card from your hand face down. It becomes foretold. Its foretell cost is its mana cost reduced by {2}. (On a later turn, you may … |
| Frost Fair Lure Fish | WHO | Creature — Fish | keyword_ability | Foretell {3}{U}{R} |
| Glorious Protector | CLB | Creature — Angel Cleric | keyword_ability | Foretell {2}{W} |
| Gods' Hall Guardian | KHM | Creature — Cat | keyword_ability | Foretell {3}{W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Green Slime | CLB | Creature — Ooze | keyword_ability | Foretell {G} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Haunting Voyage | ECC | Sorcery | keyword_ability | Foretell {5}{B}{B} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Impending Flux | WHO | Sorcery | keyword_ability | Foretell {1}{R}{R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Iron Verdict | KHM | Instant | keyword_ability | Foretell {W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Jarl of the Forsaken | KHM | Creature — Zombie Cleric | keyword_ability | Foretell {1}{B} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Karfell Harbinger | KHM | Creature — Zombie Wizard | activated_ability | {T}: Add {U}. Spend this mana only to foretell a card from your hand or cast an instant or sorcery spell. |
| Kaya's Onslaught | KHM | Instant | keyword_ability | Foretell {W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Lifestream's Blessing | FIC | Instant | keyword_ability | Foretell {4}{G} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Lupine Harbingers | YMID | Creature — Wolf | keyword_ability | Foretell {4}{G}{G} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Mammoth Growth | KHM | Instant | keyword_ability | Foretell {G} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Mystic Reflection | KHM | Instant | keyword_ability | Foretell {U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Niko Defies Destiny | KHM | Enchantment — Saga | resolve_or_static | II — Add {W}{U}. Spend this mana only to foretell cards or cast spells that have foretell. |
| Poison the Cup | KHM | Instant | keyword_ability | Foretell {1}{B} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Quakebringer | KHM | Creature — Giant Berserker | keyword_ability | Foretell {2}{R}{R} |
| Ranar the Ever-Watchful | KHC | Legendary Creature — Spirit Warrior | resolve_or_static | The first card you foretell each turn costs {0} to foretell. |
| Ravenform | LCC | Sorcery | keyword_ability | Foretell {U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Return Upon the Tide | KHM | Sorcery | keyword_ability | Foretell {3}{B} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Rise of the Dread Marn | KHM | Instant | keyword_ability | Foretell {B} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Sage of the Beyond | OTC | Creature — Spirit Giant | keyword_ability | Foretell {4}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Sarulf's Packmate | KHM | Creature — Wolf | keyword_ability | Foretell {1}{G} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Saw It Coming | KHM | Instant | keyword_ability | Foretell {1}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Scorn Effigy | KHM | Artifact Creature — Scarecrow | keyword_ability | Foretell {0} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Shepherd of the Cosmos | KHM | Creature — Angel Warrior | keyword_ability | Foretell {3}{W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Singing Towers of Darillium | WHO | Plane — Darillium | resolve_or_static | Each nonland card in your hand without foretell has foretell. Its foretell cost is equal to its mana cost reduced by {2}. (During your turn, you may pay {2} and exile it from your hand face down. C… |
| Skull Raid | KHM | Sorcery | keyword_ability | Foretell {1}{B} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Sozin's Comet | TLA | Sorcery | keyword_ability | Foretell {2}{R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Spectral Deluge | KHC | Sorcery | keyword_ability | Foretell {1}{U}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Starnheim Unleashed | KHM | Sorcery | keyword_ability | Foretell {X}{X}{W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Stoic Farmer | KHC | Creature — Dwarf Peasant | keyword_ability | Foretell {1}{W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Struggle for Skemfar | KHM | Sorcery | keyword_ability | Foretell {G} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Surge of Brilliance | WHO | Instant | keyword_ability | Foretell {1}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Tales of the Ancestors | KHC | Sorcery | keyword_ability | Foretell {1}{U} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Tergrid's Shadow | KHM | Instant | keyword_ability | Foretell {2}{B}{B} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| The Foretold Soldier | WHO | Creature — Alien Zombie Soldier | keyword_ability | Foretell {1}{G} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Ultimate Magic: Holy | FIC | Instant | keyword_ability | Foretell {2}{W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Ultimate Magic: Meteor | FIC | Sorcery | keyword_ability | Foretell {5}{R} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Vengeful Reaper | KHM | Creature — Angel Cleric | keyword_ability | Foretell {1}{B} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |
| Warhorn Blast | KHM | Instant | keyword_ability | Foretell {2}{W} (During your turn, you may pay {2} and exile this card from your hand face down. Cast it on a later turn for its foretell cost.) |

## CitysBlessing (0 cards)

_No activators in current oracle corpus._

## Dormant-hooks registry recommendation

One row per subsystem. The **activation trigger** column lists the engine event(s) that must flip the subsystem's `active` flag from false to true; once live, the subsystem's state, SBAs, and continuous effects run normally. If no activator ever fires for a game, the subsystem stays dormant and its hot path is skipped entirely.

| Subsystem | Activation trigger (engine event) | Notes |
|---|---|---|
| DayNight | first `daybound`/`nightbound` permanent enters battlefield OR first oracle effect resolves whose body contains `it becomes day` / `it becomes night` | Once the day/night designation is set, it persists for the rest of the game (CR §726.3); the subsystem cannot deactivate. |
| Monarch | first `becomes the monarch` effect resolves (Throne of the High City ETB, Court of * ETB, combat-damage trigger that designates a new monarch, Marchesa / Custodi Lich-style ETBs) | Once active, registers per-EOT card-draw replacement + combat-damage-to-player monarch-transfer replacement on every seat for the rest of the game. |
| Initiative | first `take the initiative` effect resolves (White Plume Adventurer ETB, Initiation Ceremony, Caves of Chaos Adventurer, Vanquish the Horde-shape sources) | Mirrors Monarch state machine + automatically activates Dungeons (Undercity) on first combat-damage-to-player transfer. |
| Ascend | first cast or ETB of a card with the `ascend` keyword | Activates per-seat permanent count tracking; once a seat reaches 10 permanents the city's blessing flag flips on (one-way, sticks for the game). |
| Dungeons | first `venture into the dungeon` / `venture into the Undercity` effect resolves | Initiative activation implicitly activates Dungeons (via Undercity); standalone venture activates same registry path. |
| RingTempts | first `the Ring tempts you` effect resolves (Frodo Sauron's Bane ETB, The One Ring ETB, etc.) | Activates ring-tracker (per-seat ring-bearer designation + Ring's amulet-style ability ladder). |
| Energy | first effect resolves that gives a player `{E}` or creates an energy counter | Spenders (`pay {E}`) cannot activate the subsystem — if no producer has fired, the pool is empty and the spender is unaffordable. Skipping the subsystem's bookkeeping until first production is safe. |
| Experience | first effect gives a player an experience counter | Tiny universe: 6 legendary creatures + a handful of non-legendary sources. Activator is always the granting effect's resolution. |
| Foretell | first card with the `foretell` keyword is cast or otherwise placed in a foretell exile zone | Foretell zone is per-seat; activate the foretell tracker on first usage (covers `cast .* from your foretell` consumer pattern automatically). |
| CitysBlessing | delegated entirely to the Ascend subsystem | No independent activation surface in printed oracle text. Cards that read `if you have the city's blessing` are pure consumers and never activate the tracker on their own. |

## Next steps

1. Wire dormant-hook flags into `GameState` (one bool per subsystem; default false).
2. Add activation barriers in each subsystem's hot path: `if !gs.Flags["subsystem_<name>_active"] { return }`.
3. Set the flag inside the canonical activation event (per the registry above) and emit a `subsystem_activated` log event for audit replay.
4. Goldilocks regression: assert that for a deck containing no activator for subsystem X, the hot path executes zero times across a full game.
