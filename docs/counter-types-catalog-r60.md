# Counter Types Catalog — r60 (Probe F)

**Scope:** Comprehensive registry of every distinct counter type appearing in the Scryfall oracle corpus (`data/rules/oracle-cards.json`, 37,384 cards, 6,099 oracle texts containing "counter"). Built for the Counter DB engine moat per CR §122.

**Method:** Stream-parse oracle JSON, regex-extract `<word> counter(s)?` and `[+-]N/[+-]M counter(s)?` patterns, dedupe, prune non-counter regex artifacts (the verb "counter target spell"; possessives like "creature's counters"; joke-set noise lacking real game function), classify against CR §122 + per-card oracle context.

**Headline numbers:** 252 distinct counter types catalogued across 8 categories. 12 P/T (power/toughness) shapes, 16 ability-granting per CR §122.1c, 4 player-counter kinds (poison, experience, rad, energy), and a long-tail 197 card-specific resource/storage/triggered counters most of which appear on 1–3 cards. Top-frequency types: `+1/+1` (3,200 cards), `-1/-1` (306), `lore` (227), `poison` (187), `charge` (160), `energy` (138), `time` (135), `age` (86), `stun` (84), `loyalty` (68), `oil` (54).

---

## Engine Rules (CR §122 + relevant cross-references)

These are the load-bearing invariants the Counter DB must enforce. They apply to ALL counter types unless an individual rule scopes itself.

| Rule | Statement | Engine implication |
|------|-----------|--------------------|
| **§122.1** | A counter is a marker placed on an object or player that modifies its characteristics and/or interacts with abilities. Counters are not objects. | Counter is a `(kind string, count int)` pair on a `*Permanent` or `*Seat`; never instantiated as `*Card`/`*Permanent`. |
| **§122.1c** | If a counter has a name that's also the name of a keyword ability (e.g. "flying counter"), the object with that counter has the keyword ability. | Replace the `Permanent.HasFlying()` etc. predicates with `perm.HasKeyword(k) \|\| perm.HasCounter(k+" counter") > 0`. Same for trample/lifelink/menace/reach/vigilance/deathtouch/first strike/double strike/indestructible/hexproof/haste/ward/shadow/phyresis (+infect). |
| **§122.1g** | If an effect would cause a player to put one or more counters on a permanent they control (NB: "permanent they control" — NOT another player's permanent and NOT a player), and another effect (e.g. Doubling Season, Hardened Scales) modifies that placement, apply the modifier. | Counter placement on player-controlled permanents flows through a "counter-placement effect" pipeline that lets Doubling Season (×2 all), Hardened Scales (+1 only for +1/+1), Branching Evolution (×2 only for +1/+1), Conclave Mentor (+1 only for +1/+1), Innkeeper's Talent (+1 only for +1/+1) etc. modify the count. NOT triggered on opponent's permanents, NOT triggered for counters placed on players themselves. |
| **§122.6** | A counter remains on a permanent even if the permanent's characteristics change such that the counter no longer has a function. If the characteristics later change such that the counter functions again, it still does. | **Critical persistence invariant.** Lifelink counter on Vraan stays attached when Humility strips abilities — when Humility leaves, lifelink works again. Loyalty counters on a planeswalker that becomes a creature persist; if it becomes a planeswalker again, they still count for activations. Engine: counters NEVER cleared on type/ability change. Only on §704.5r pairing, explicit "remove" effects, or LTB. |
| **§704.5r** | If a creature has both a +1/+1 counter and a -1/-1 counter on it, N pairs of +1/+1 and -1/-1 counters are removed from it, where N is the lesser of the number of +1/+1 and -1/-1 counters on it. | **Only +1/+1 and -1/-1 cancel.** No other counter pair cancels (e.g. +2/+2 and -2/-2 do NOT cancel — they sit independently). State-based action. Runs every SBA pass. |
| **§704.5c** | A player with ten or more poison counters loses the game. | Player-counter loss check. Distinct from §704.5a life-loss; both must run independently. |
| **§704.5i** | If a planeswalker has loyalty 0, its owner puts it into their graveyard. | Loyalty counter == loyalty value (§306.5b). 0 counters → graveyard SBA. |
| **§306.5b** | A planeswalker's loyalty equals the number of loyalty counters on it. Battles' defense equals the number of defense counters. | Loyalty/defense are NOT base values + counters — they ARE counter counts. |
| **§310.7** | When a battle is dealt damage, that many defense counters are removed. | Defense counter removal = damage applied to battle. |
| **§310.10** | When a battle has 0 defense counters, defeat trigger + exile (CR 704.5x). | Battle loss SBA mirrors planeswalker loss SBA. |
| **§711** | Level up keyword uses "level counters" to gate level brackets defining the creature's P/T and abilities. | Level counter accumulation; per-level static effect grants different P/T + abilities. |
| **§712 (Saga)** | Sagas enter with no lore counters; at each precombat main step of controlling player's turn, +1 lore counter; chapter abilities trigger when lore counter count becomes equal to chapter number. After final chapter triggers, sacrifice. | Lore counter is special-cased: §122.1g doubling DOES apply (Doubling Season makes Sagas ETB with +1, fast-forwards them by 1 chapter — confirmed by historical official ruling). |
| **§701.27 (Proliferate)** | Choose any number of permanents and/or players with counters on them, then give each another counter of a kind already there. | Proliferate operates on ALL counter kinds (including energy/poison on players, loyalty on PWs, lore on Sagas, defense on battles, +1/+1, etc.). Cannot add an absent counter type. |
| **§106.11 (Energy)** | Energy is a resource pool, not a counter on a permanent. Lives on the player. | Despite the regex match `energy counter`, energy is NOT a §122 counter — it's a pool of resources represented as `{E}`. Proliferate does NOT add `{E}`. Catalogued here for completeness with this caveat flagged. |
| **§701.50 (Stun counter)** | If a permanent with one or more stun counters on it would become untapped, remove a stun counter from it instead. That permanent doesn't become untapped. | Untap-replacement, applies per-counter (one untap event consumes one counter). |
| **§702.62 (Suspend)** | A suspended card has `time counters` on it; at each upkeep, remove one. At 0, cast it (if able). | Time counter on suspended card in exile zone. |
| **§702.63 (Vanishing)** | Permanent enters with N time counters; at each upkeep, remove one. At 0, sacrifice. | Vanish reuses `time` counter kind. |
| **§702.32 (Fading)** | Permanent enters with N fade counters; at each upkeep, remove one. If can't, sacrifice. | Distinct kind from `time`. |
| **§702.24 (Cumulative upkeep)** | At upkeep, put an age counter; then pay cost per age counter or sacrifice. | `age` counter. |

### Engine-ruling notes on grey areas

- **Doubling Season on player counters.** Doubling Season's literal text says "permanents you control"; on a strict reading, doubling does NOT apply to poison/experience/energy/rad placed on a player (the player isn't a permanent). Engine ruling: **NO doubling on player counters.** (Historically WotC has confirmed this for poison — Doubling Season doesn't double poison counters placed on you.) Other "counter-placement modifier" effects that scope to "you" (e.g. "if you would get one or more poison counters, you get that many plus one instead") DO apply.
- **§122.6 persistence under reanimation.** A creature dying and being reanimated is a NEW object (CR §400.7). Its counters do NOT persist across the zone change. Engine: clear all counters on any battlefield-leaves event. §122.6 only protects counters from being cleared by CHARACTERISTIC changes, not by zone changes.
- **§122.6 persistence under copy effects.** A copy effect (Cytoshape, Polymorphist's Jest, Mirror Gallery) changes characteristics but not counters; counters that no longer function (e.g. a lifelink counter on a creature copied as a non-creature artifact temporarily) stay attached per §122.6 and resume function when the copy effect ends.
- **Joke-set / Un-set counters.** Several counters in the catalog appear only on Un-stable/Un-finity/playtest cards (`stroopwafel`, `bargle`, `shoe`, `twenty`, `glass`, `traffic`, `keyword`, `token`, `spooky`, etc.). These are catalogued for completeness but the engine does not need to model their effects beyond generic counter persistence. They follow standard §122 plumbing.
- **§704.5r scope.** Only +1/+1 cancels with -1/-1. +2/+2 with -2/-2, +1/+0 with -1/-0, etc. do NOT cancel — they sit independently and modify the creature's P/T additively. Curse of Death's Hold (-1/-1 anthem) + Glorious Anthem (+1/+1) on the SAME creature does NOT trigger pairing because those are static P/T modifications, not counters.

---

## A. Power/Toughness Counters (12 shapes)

Apply to creatures (and non-creature permanents with abilities that reference them, e.g. Walking Ballista). +1/+1 and -1/-1 cancel per §704.5r. All shapes are doubled by Doubling Season per §122.1g. Persistence per §122.6 holds.

| Counter | Card count | Cancels (§704.5r) | Common sources | Removal | Notes |
|---------|-----------:|---|---|---|---|
| `+1/+1` | 3,200 | with `-1/-1` | universal — ETB triggers, anthems, Hardened Scales, evolve, adapt, monstrous, mentor, modular, graft, training (as of 2022+), bloodthirst, devour, ferocidon, ozolith recovery | -1/-1 pairing, "remove a +1/+1" effects (Throne of Geth, Devoted Druid, Ozolith), Hex Parasite (proliferate-targeted removal) | Most common counter in MTG. Modified by Doubling Season, Branching Evolution, Hardened Scales, Conclave Mentor, Innkeeper's Talent, The Ozolith (recovery). |
| `-1/-1` | 306 | with `+1/+1` | infect damage, persist, wither, attrition, Black Sun's Zenith, Yawgmoth Thran Physician | +1/+1 pairing, Vampire Hexmage (remove all), Solemnity (prevention) | Causes creature to die via SBA 704.5f when toughness reaches 0. |
| `+1/+0` | 9 | none | Clockwork Avian/Beast/Steed/Swarm, Balduvian Hydra | spent via attacks (Clockwork cycle) | Clockwork creatures use them as battery. |
| `+2/+2` | 9 | none | Baron Sengir, Autumn Willow, Dwarven Armory, Brass-Talon Chimera | rare | Old-frame oddball. |
| `+0/+1` | 7 | none | Living Armor, Coral Reef (via polyp), Necropolis, Dwarven Armorer, Sacred Boon | rare | Toughness-only growth. |
| `+1/+2` | 2 | none | Experiment Five, Armor Thrull | n/a | |
| `+0/+2` | 1 | none | Frankenstein's Monster | n/a | |
| `-0/-1` | 8 | none | Krovikan Plague, Lesser Werewolf, Essence Flare | n/a | |
| `-0/-2` | 2 | none | Greater Werewolf, Spirit Shackle | n/a | |
| `-1/-0` | 1 | none | Jabari's Influence | n/a | |
| `-2/-1` | 1 | none | Contagion | n/a | |
| `-2/-2` | 1 | none | Ebon Praetor | n/a | |

---

## B. Resource Counters of Permanent Types (Loyalty, Lore, Defense)

These counters define the resource value of their permanent type. Each is the SUBSTANCE of the permanent's main state, not a modifier.

| Counter | Card count | Target | ETB | Removal | Doubling §122.1g | Proliferate | Threshold trigger | Notes |
|---|---:|---|---|---|---|---|---|---|
| `loyalty` | 68 | planeswalker | ETB at printed value (CR 306.5b) | +/- loyalty ability costs; damage to PW (CR 120.3); 0 loyalty → graveyard (CR 704.5i) | yes — DS doubles ETB AND counters added by `+` abilities | yes (proliferate adds 1 loyalty to a chosen PW) | 0 → graveyard | Distinct from "loyalty ability" semantics. Counter is the resource. |
| `lore` | 227 | Saga enchantment | ETB with 0 lore counters; precombat main of controller's turn → +1 (CR 716.2b) | n/a — accumulates | yes — DS makes Saga ETB with +1 lore (1 chapter ahead) per WotC ruling | yes | each `chapter N` ability triggers when lore counter count becomes N | After last chapter ability resolves → sacrifice (CR 716.4). Saga rules in §716; not §122 alone. |
| `defense` | 4 corpus / many post-MOM | Battle (subtype: Siege) | ETB with printed defense (CR 310.5) | damage to battle removes 1:1 (CR 310.7) | yes — DS doubles ETB defense | yes (proliferate adds 1 defense) | 0 → defeated trigger + exile (CR 310.10) | Defense IS the battle's resource value. |

---

## C. Ability-Granting Counters (CR §122.1c) — 16 types

CR §122.1c: a counter named after a keyword grants that keyword. The Counter DB must check counter presence in keyword predicates. All ETB sources are "ability-granting effects" (e.g. Path of Mettle "put a flying counter on this creature"; Cleansing Wildfire-style). Doubling Season applies per §122.1g (yes, you can have 2 flying counters — still just grants flying once, but proliferate-target counts). §122.6 persistence: counter stays even if creature becomes non-creature, function returns if creature again.

| Counter | Card count | Keyword granted | Engine note |
|---|---:|---|---|
| `flying` | 39 | flying | Add to `HasFlying()` check. |
| `indestructible` | 21 | indestructible | Add to `HasIndestructible()` check. SBA 704.5f/g respects the counter. |
| `lifelink` | 20 | lifelink | Add to lifelink damage hook. Notable Vraan + Humility case in §122.6 example. |
| `deathtouch` | 14 | deathtouch | Add to combat damage assignment. |
| `trample` | 14 | trample | Add to assign-combat-damage step. |
| `vigilance` | 10 | vigilance | Add to declare-attackers tap check. |
| `first strike` | 6 | first strike | Add to combat damage step gating. |
| `menace` | 8 | menace | Add to declare-blockers minimum-blocker count. |
| `reach` | 8 | reach | Add to "creature can block flyers" check. |
| `hexproof` | 5 | hexproof | Add to targeting permission. |
| `double strike` | 4 | double strike | Add to first-strike damage step trigger. |
| `ward` | 1+ | ward (usually ward {2}) | Add to targeting trigger. |
| `shadow` | 2 | shadow | Add to combat-block legality. |
| `haste` | 1 | haste | Add to summoning-sickness check. |
| `decayed` | 1 | decayed (the cant-block keyword) | Atypical — usually a token-printed keyword; counter variant exists on Rot-Curse Rakshasa. |
| `phyresis` | 1 | infect (Weatherlight Compleated) | Phyresis counter grants infect on Weatherlight. |
| `strike` (legacy) | 10 | first strike (errata'd) | Old "strike counter" — wrap as first-strike-counter alias. |

---

## D. Player Counters (Not Permanent Counters) — 4 types

These live on the SEAT, not on a permanent. §122.1g doubling and proliferate semantics differ.

| Counter | Card count | CR | Doubling §122.1g | Proliferate | Loss condition | Notes |
|---|---:|---|---|---|---|---|
| `poison` | 187 | §704.5c, §122.1 | NO (DS doesn't apply to player counters) — but "if you would get a poison counter, you get that many plus one instead"-style effects DO (Vraska's Conquistador anti-pattern) | yes | 10+ → lose the game | Sources: infect, toxic N, corrupted gating, Phyresis. Removal: rare (Tamiyo's Compleation, Leeches). |
| `energy` | 138 | §106.11, §122.1 | NO — energy is NOT a §122 counter; it's a resource pool denoted `{E}` | NO (proliferate cannot add energy per CR §122.6 implication / explicit WotC ruling) | n/a | Pool persists across turns. Resets at game end. Sources: "get N {E}" abilities. Spent via "pay N {E}" abilities. |
| `experience` | 18 | §122.1 | NO (per engine ruling — DS is "permanent you control") | yes | n/a — gates commander abilities at threshold | Daxos the Returned, Atreus, Azlask. Persist across commander recasts (lives on seat, not commander). |
| `rad` | 24 | §122.1 (Fallout) | NO (player counter) | yes | n/a — mills per upkeep, life-loss + counter-remove per nonland milled | Fallout Commander mechanic. |

---

## E. Time/Fade/Age Countdown Counters — 3 types

Removed automatically at upkeep; sacrifice/cast trigger at 0 or each upkeep.

| Counter | Card count | CR | Removal pace | Threshold action | Doubling §122.1g | Proliferate | Notes |
|---|---:|---|---|---|---|---|---|
| `time` | 135 | §702.62 (suspend), §702.63 (vanishing) | -1 each owner's upkeep | suspend: cast at 0; vanishing: sacrifice at 0 | yes (DS doubles ETB time for vanishing; for suspend it doubles the time counters at the moment of suspension) | yes (extends lifespan) | Used by 2 distinct CR mechanics with same counter kind. |
| `fade` | 18 | §702.32 (fading) | -1 each upkeep, if can't pay → sacrifice | sacrifice on "can't remove" | yes | yes | Mirage block. |
| `age` | 86 | §702.24 (cumulative upkeep) | +1 each upkeep (NOT -1!) | pay {age N} cumulative-upkeep cost per turn or sacrifice | yes | yes | Ice Age block. Accumulates — opposite direction from time/fade. |

---

## F. Alternate-Win / Threshold Counters — 8 types

These trigger a discrete game-state effect at a specific count, often alternative win conditions.

| Counter | Card count | Card | Threshold | Engine note |
|---|---:|---|---|---|
| `quest` | 33 | Beastmaster Ascension, Bloodchief Ascension, Quest cycle, Archmage Ascension | varies (7 for Beastmaster, 4 for Bloodchief, 6 for Archmage) | Triggers static effect on/after threshold met. |
| `level` | 30 | Level Up creatures (Brimstone Mage, Caravan Escort, etc.) | each level bracket | CR §711 — defines P/T + abilities per bracket. |
| `verse` | 13 | Aria of Flame, Crescendo of War, Lilting Refrain | varies | Threshold/scaling effects per count. |
| `study` | 10 | Class enchantments, Lattice Library, Vhal Eager Scholar, Imbraham | varies | Level-up-style class progression. |
| `tower` | 1 | Helix Pinnacle | 100 | Alt-win. |
| `intervention` | 1 | Divine Intervention | 0 (counts down from 2 at upkeep) | Alt-end (game drawn). |
| `filibuster` | 1 | Azor's Elocutors | 5 | Alt-win. |
| `luck` | 4 | Chance Encounter, As Luck Would Have It, Gemstone Caverns, Pick-a-Beeble | 10 (Chance Encounter) | Alt-win. |

---

## G. Replacement-Effect Counters — 9 types

These counters drive CR §614 replacement effects (would-be-destroyed → instead remove a counter; would-untap → instead remove a counter; etc.).

| Counter | Card count | Replacement effect | CR | Engine note |
|---|---:|---|---|---|
| `stun` | 84 | If permanent would untap, remove a stun counter instead | §701.50 | One counter consumed per untap event. |
| `shield` | 32 | If creature would be destroyed OR dealt damage, remove a shield counter and prevent | MOM-batched | March of the Machine mechanic. Single counter prevents one event. |
| `finality` | 37 | If a creature with a finality counter would die, exile it instead | §122 + §614 | Death-replacement. |
| `mannequin` | 1 | When triggered ability resolves, sacrifice creature with mannequin counter | Makeshift Mannequin only | One-time trigger handle. |
| `incarnation` | 1 | If damage would be dealt to you and you have Nine Lives, prevent it and add an incarnation counter; 9 → lose | Nine Lives only | Alt-loss via accumulation. |
| `isolation` | 1 | Quarantine Field ETB with X isolation; remove → return exiled permanent | Quarantine Field only | Spend-to-return. |
| `paralyzation` | 1 | Dread Wight: paralyzation counter prevents untap | Dread Wight only | Pre-stun-counter analogue. |
| `pin` | 1 | Voodoo Doll: remove pin counter to deal damage equal to count | Voodoo Doll only | |
| `echo` | 1 (counter) | Soul Echo: damage-replacement removes counter | Soul Echo | Distinct from "Echo" keyword cost. |

---

## H. Storage / Resource / Card-Specific Counters — 197 types

Long-tail counters used as on-permanent resource pools, accumulators, growth markers, or one-off mechanics. All follow standard §122 plumbing: §122.6 persistence holds, Doubling Season applies to placement on player-controlled permanents per §122.1g, proliferate can add 1 more.

### H.1 Multi-card storage counters (≥2 cards)

| Counter | Card count | Example cards | Mechanic |
|---|---:|---|---|
| `charge` | 160 | Aether Vial, Coalition Relic, Aetherflux Reservoir, Powerstone Shard, Voltaic Key family | Universal storage; spent via tap/activation; many alt-win or finisher engines (Aetherflux, Mox Opal-style) |
| `oil` | 54 | Skrelv Defector Mite, Archfiend of the Dross, Argent Mutation, Atraxa's Skitterfang | Phyrexia: All Will Be One; ETB with oil, spend for abilities, self-Compleation thresholds |
| `brick` | 5 | Pyramid of the Pantheon, Oracle's Vault, Edifice of Authority, Luxa River Shrine, Sunset Pyramid | Egyptian/pyramid: ETB with 0, tap adds 1, threshold flips to better mode |
| `depletion` | 12 | Hickory Woodlot, Lava Tubes, Land Cap, Decree of Silence | Storage land cycle (Mirage); ETB with depletion, tap removes for mana |
| `storage` | 18 | Bottomless Vault, Calciform Pools, City of Shadows, Dreadship Reef, Crucible of the Spirit Dragon, Saltcrusted Steppe | Storage-land cycle; tap to add, spend for big mana |
| `petal` | 1 | Lotus Blossom | Single growth-spend |
| `hour` | 4 | Midnight Clock, Midnight Oil, Rusko Clockmaker, The Bus Runner | 12 → big effect (Midnight Clock) |
| `dream` | 3 | Rasputin Dreamweaver, Goliath Daydreamer, Rasputin the Oneiromancer | Rasputin: ETB with 7, spent for mana |
| `coin` | 3 | Athreos Shroud-Veiled, Noble's Purse, Wishing Well | Spent via activations |
| `eon` | 2 | Magosi the Waterveil, Out of the Tombs | Tap removes; 2 → extra turn |
| `point` | 4 | Strixhaven Stadium, Contested Game Ball, Brave Falconhawk, Raiders' Allegiance | Threshold mechanics |
| `ticket` | 3 | Blorbian Buddy, Ticket Bucket-Bot, Ticket Turbotubes | Storage |
| `wish` | 3 | Wishclaw Talisman, Ring of Three Wishes, Djinn of Wishes | ETB with 3, activated abilities spend |
| `stash` | 3 | Glittering Stockpile, Hoarder's Overflow, Tinybones Bauble Burglar | Storage/scaling |
| `knowledge` | 1 | The Magic Mirror | Storage/scaling |
| `book` | 2 | Spell Satchel, A-Spell Satchel | Storage |
| `page` | 7 | Mazemind Tome, Barrin's Codex, Autograph Book, Diary of Dreams, Private Research | Storage; spend for effect |
| `verse` | 13 | (above — F.1) | Scaling |
| `ki` | 11 | Baku Altar, Blademane Baku, Budoka Pupil, Callow Jushi, Cunning Bandit | Kamigawa: Spirit/Arcane cast trigger adds; threshold flips |
| `soot` | 1 | Smokestack | Upkeep adds 1; everyone sacrifices N permanents |
| `treasure` (counter, NOT token) | 1 | Legacy's Allure | Distinct from Treasure tokens |
| `gold` | 2 | Aurification, Dragon's Hoard | Distinct from Gold tokens |
| `food` (counter) | 1 | corpus rare; distinct from Food tokens | |
| `clue` (counter) | 1 | corpus rare; distinct from Clue tokens | |
| `map` (counter) | 1 | corpus rare; distinct from Map tokens | |
| `landmark` | 1 | Treasure Map (flipside Treasure Cove) | Threshold |
| `component` | 1 | Component Pouch | Storage |
| `cube` | 1 | Delif's Cube | Storage |
| `currency` | 1 | Trade Caravan | Storage |
| `delay` | 2 | Delaying Shield, Ertai's Meddling | Countdown |
| `fuse` | 5 | Powder Keg, Goblin Bomb, Bomb Squad, Incendiary, Pumpkin Bombs | Countdown / spend |
| `doom` | 6 | Armageddon Clock, Baron Von Count, Eye of Doom, Imminent Doom, Lavabrink Floodgates | Countdown to mass effect |
| `omen` | 3 | Celestial Convergence, Foreboding Statue, Soulcipher Board | Countdown |
| `flood` | 6 | Aquitect's Will, Bounty of the Luxa, Eluge Shoreless Sea, Quicksilver Fountain, The Flood of Mars | Water cycle |
| `tide` | 2 | Homarid, Tidal Influence | 1→4→reset cycle |
| `wind` | 2 | Freyalise's Winds, Cyclone | Untap-replacement-ish |
| `flame` | 5 | Flame Channeler/Embodiment of Flame, Kardum, Managorger Phoenix, Naar Isle, Tend to the Kiln | Scaling |
| `growth` | 5 | Simic Ascendancy, Paradox Zone, Malignant Growth, Comforting Counsel, Momentum | Simic Ascendancy alt-win at 20 |
| `pressure` | 4 | Magma Mine, Hellion Crucible, Exploding Barrel, Mount Keralia | Storage / spend |
| `mining` | 1 | Gemstone Mine | ETB with 3, tap removes; sacrifice at 0 |
| `mire` | 1 | Cyclopean Tomb | Land-transformation engine |
| `polyp` | 1 | Coral Reef | Produces +0/+1 counters |
| `slumber` | 1 | Arixmethes Slumbering Isle | ETB with 5; attacks/triggers remove; 0 → becomes 12/12 |
| `slime` | 3 | Gutter Grime, Sludge Monster, Toxrill the Corrosive | Generates Ooze tokens scaled by count |
| `spore` | 20 | Deathspore Thallid, Elvish Farmer, Feral Thallid, Fungal Bloom, Mycologist | Upkeep adds 1; remove 3 → Saproling |
| `ice` | 7 | Iceberg, Rimefeather Owl, Draugr Necromancer, Rimescale Dragon, Dark Depths | Iceberg-style storage; Rimefeather Owl proliferate engine |
| `soul` | 7 | Soulcatchers' Aerie, Malefic Scythe, Hostile Hostel, Netherborn Altar, Obscura Ascendancy, Ravenous Amulet | Creature death trigger feeds count |
| `bounty` | 6 | Bounty Board, Bounty Hunter, Chevill Bane of Monsters, Mathas Fiend Seeker, Shay Cormac, Aragorn King of Gondor | Marker; death of marked → reward |
| `corpse` | 4 | Crowded Crypt, From the Catacombs, Isareth the Awakener, Scavenging Ghoul | Graveyard tracker / sac fodder |
| `hit` | 3 | Etrata the Silencer, Mari the Killing Quill, Ravenloft Adventurer | 3 hits → exile/effect |
| `acorn` | 4 | Chitterspitter, Acornelia, Acorn Stash, Euru | Storage |
| `egg` | 3 | Darigaaz Reincarnated, Darigaaz Shivan Champion, Xira the Golden Sting | Growth |
| `hatchling` | 3 | Ludevic's Test Subject, Triassic Egg, Eumidian Hatchery | Threshold → transform |
| `shell` | 1 | Roc Hatchling | Growth |
| `pupa` | 1 | Cocoon | Growth |
| `tribute` | 1+ | Tribute mechanic | ETB-choice; opponent chooses to add (not really persistent counter — set baseline) |
| `divinity` | 8 | Myojin cycle (Cleansing Fire, Infinite Rage, Life's Web, etc.), Bant, Kindred Boon | ETB with 1; spend to trigger one-time effect |
| `fate` | 3 | Triad of Fates, Norn's Dominion, Oblivion Stone | Rare engine |
| `feather` | 3 | Aven Mimeomancer, Kangee Aerie Keeper, Soulcatchers' Aerie | Flying tribal counter |
| `fetch` | 3 | Pako Arcane Retriever, Haldan Avid Arcanist, Hex Kellan's Companion | Storage of fetched cards |
| `dread` | 2 | Grasping Shadows, No Way Out (playtest) | Rare |
| `void` | 2 | Dauthi Voidwalker, Sphere of Annihilation | Rare exile-tracking |
| `velocity` | 2 | Daredevil Dragster, Tornado | Scaling |
| `eyeball` | 1 | Jar of Eyeballs | Storage |
| `eyestalk` | 1 | Underdark Beholder | Storage |
| `infection` | 3 | Diseased Vermin, Festering Wound, Genestealer Patriarch | Phyrexia-flavor |
| `plague` | 3 | Plague Boiler, Traveling Plague, Withering Hex | Scaling |
| `bloodline` | 1 | Edgar Markov's Coffin | Vampire tribal |
| `croak` | 1 | Grolnok the Omnivore | Frog tribal |
| `fungus` | 2 | Mindbender Spores, Sporogenesis | Thallid variant |
| `funk` | 1 | Temp of the Damned | Rare |
| `fury` | 1 | Charging Cinderhorn | Trampling-fury build-up |
| `harmony` | 1 | Instrument of the Bards | Bard tribal |
| `hoofprint` | 1 | Hoofprints of the Stag | 4 → 4/4 stag token |
| `muster` | 1 | Assemble the Legion | Upkeep adds; tokens scale |
| `valor` | 1 | Intrepid Adversary | Champion-style |
| `vitality` | 1 | Living Artifact | Damage replacement → vitality; remove for life |
| `vortex` | 1 | Energy Vortex | Damage tracker |
| `wage` | 1 | Rogue Skycaptain | Upkeep adds; threshold flips controller |
| `unity` | 1 | Call for Unity | Upkeep adds; scaling anthem |
| `phylactery` | 1 | Phylactery Lich | Sac'd artifact = 1 counter; 0 → die |
| `tower` | 1 | Helix Pinnacle | Alt-win at 100 (F.1 cross-ref) |
| `prey` | 1 | Tetzimoc Primal Death | Mark mechanic |
| `bait` | 1 | Fishing Pole | Rare |
| `chip` | 1 | B-I-N-G-O | Rare |
| `chorus` | 1 | Malcolm Alluring Scoundrel | Rare |
| `credit` | 1 | Icatian Moneychanger | Rare |
| `crystal` | 1 | Prism Array | Rare |
| `cell` | 1 | Sephiroth Fallen Hero | Rare (Final Fantasy) |
| `brain` | 1 | Rex Cyber-Hound | Rare |
| `gem` | 1 | Briber's Purse | Storage |
| `intel` | 1 | Flamewar Brash Veteran | Transformers |
| `kick` | 1 | Zethi Arcane Blademaster | Rare |
| `matrix` | 1 | Life Matrix | Rare |
| `memory` | 1 | Altaïr Ibn-La'Ahad, The Animus | Rare |
| `nest` | 1 | Twitching Doll | Rare |
| `night` | 1 | Replicating Ring | Rare |
| `ore` | 1 | Orcish Mine | Rare |
| `pain` | 1 | Torture Chamber | Rare |
| `palliation` | 1 | Palliation Accord | Rare |
| `paralyzation` | 1 | Dread Wight | (G cross-ref) |
| `pause` | 1 | Grand Marshal Macie | Rare |
| `petrification` | 1 | Xathrid Gorgon | Stone-counter freezes |
| `plan` | 1 | Doom Reigns Supreme | Rare |
| `plot` | 1 | Deadly Designs | Rare |
| `possession` | 1 | Unwilling Vessel | Rare |
| `rally` | 1 | Aligned Heart | Rare |
| `rejection` | 1 | Tolarian Contempt | Rare |
| `release` | 1 | The Heron Moon | Rare |
| `reprieve` | 1 | Magnanimous Magistrate | Rare |
| `resonance` | 1 | Fifth Stage of Magic Design | Rare |
| `rev` | 1 | Chainsaw (Equipment) | Vehicle-style cost |
| `revival` | 1 | Nine-Lives Familiar | Rare |
| `ribbon` | 1 | Prize Pig | Rare |
| `ritual` | 1 | Heirloom Mirror | Rare |
| `scroll` | 1 | Aretopolis | Rare |
| `shred` | 1 | Cephalid Vandal | Rare |
| `silver` | 1 | Karn Scion of Urza | Construct token base |
| `skewer` | 1 | Rotisserie Elemental | Rare |
| `sleep` | 1 | Venarian Gold | Rare |
| `spark` | 1 | Blood Poet | Rare |
| `spite` | 1 | Curse of Vengeance | Rare |
| `stall` | 1 | Vamping Vampire | Rare |
| `story` | 1 | Staff of the Storyteller | Rare |
| `strife` | 1 | Crescendo of War | Rare |
| `supply` | 1 | Stocking the Pantry | Rare |
| `suspect` | 1 | Investigator's Journal | Rare |
| `takeover` | 1 | The Master Formed Anew | Rare |
| `task` | 1 | Heliod's Punishment | Rare |
| `taste` | 1 | Moth Herb Elixir | Rare |
| `theft` | 1 | Night Dealings | Storage; spend → tutor |
| `unlock` | 1 | Cryptex | Rare |
| `vow` | 1 | Promise of Loyalty | Rare |
| `voyage` | 1 | Cosima God of the Voyage | Countdown |
| `winch` | 1 | Mercadian Lift | Rare |
| `wreck` | 1 | Spectacle of Destruction | Rare |
| `aegis` | 1 | Livio Oathsworn Sentinel | Damage-prevent |
| `arrow` | 1 | Archery Training | Rare |
| `arrowhead` | 1 | Serrated Arrows | ETB 3, remove 1 → -1/-1 |
| `awakening` | 1 | Liege of the Tangle | Land-animate |
| `bargle` | 1 | Happy Yargle Day! | (joke set) |
| `blessing` | 1 | Boon of the Spirit Realm | Rare |
| `bloodstain` | 1 | Blood Spatter Analysis | Rare |
| `bore` | 1 | Brass's Tunnel-Grinder | Rare |
| `bribery` | 1 | Gwafa Hazid Profiteer | Rare |
| `burden` | 2 | The One Ring, A-The One Ring | Upkeep adds; take damage = burden |
| `cage` | 2 | Mairsil the Pretender, Seer of the Bright Side | Mairsil exile-grant mechanic |
| `carrion` | 1 | Osai Vultures | Rare |
| `collection` | 2 | Charitable Levy, Evelyn the Covetous | Rare |
| `conqueror` | 1 | Zhao the Moon Slayer | Rare |
| `contested` | 1 | Turf War | Rare |
| `corruption` | 1 | Geyadrone Dihada | Rare |
| `curse` | 1 | Blue Screen of Death | Rare (joke set) |
| `day` | 1 | The Knight of Weeks | Rare |
| `death` | 2 | Bogardan Phoenix, Necropotence Avatar | Rare |
| `descent` | 1 | Descent into Avernus | Rare |
| `despair` | 1 | Descent into Madness | Rare |
| `devotion` | 2 | Bloodthirsty Ogre, Pious Kitsune | Rare |
| `discovery` | 1 | Lara Croft Tomb Raider | Rare |
| `duty` | 1 | Immortal Obligation | Rare |
| `elixir` | 1 | Essence Bottle | Rare |
| `ember` | 1 | Smoldering Egg | Rare |
| `enlightened` | 1 | The Book of Exalted Deeds | Rare |
| `eruption` | 1 | Pompeii | Rare |
| `everything` | 1 | Omo Queen of Vesuva | (likely regex artifact — verify per oracle text) |
| `exalted` | 1 | Emissary of Soulfire | Rare |
| `exposure` | 1 | Aplan Mortarium | Rare |
| `fear` | 1 | corpus rare | Rare |
| `feeding` | 1 | Nazar the Velvet Fang | Rare |
| `fellowship` | 1 | Banner of Kinship | Scaling anthem |
| `film` | 1 | Peter Parker's Camera | Rare (Marvel) |
| `fire` | 2 | Fated Firepower, War Balloon | Rare |
| `foreshadow` | 1 | Ominous Seas | Rare |
| `gem` | 1 | Briber's Purse | (above) |
| `ghostform` | 1 | Kaya the Inexorable | Emblem-style |
| `glyph` | 1 | Glyph of Delusion | Rare |
| `healing` | 2 | Fylgja, Ursine Fylgja | Rare |
| `hope` | 1 | Dawn of a New Age | Rare |
| `hourglass` | 1 | Temporal Distortion | Countdown |
| `hunger` | 1 | Fasting | Rare |
| `husk` | 1 | Necropolis of Azar | Rare |
| `impostor` | 1 | Illicit Masquerade | Rare |
| `incubation` | 1 | Drake Hatcher | Growth |
| `influence` | 1 | Palantír of Orthanc | Damage-burn per count |
| `ingenuity` | 1 | Jhoira Ageless Innovator | Rare |
| `invitation` | 1 | Wedding Announcement | Rare |
| `javelin` | 1 | Icatian Javelineers | Rare |
| `judgment` | 1 | Faithbound Judge | Rare |
| `aim` | 2 | Hankyu, Haphazard Bombardment | Rare |
| `blight` | 2 | Rottenmouth Viper, Ultima Origin of Oblivion | Rare |
| `blaze` | 2 | Five-Alarm Fire, Obsidian Fireheart | Rare |
| `magnet` | 1 | Magnetic Web | Equipment-attach |
| `mannequin` | 1 | Makeshift Mannequin | (G cross-ref) |
| `manifestation` | 1 | Arbiter of the Ideal | Rare |
| `mask` | 1 | Illusionary Mask family | Rare hidden-info |
| `mine` | 1 | Mine Layer | Rare |
| `net` | 2 | Braided Net, Merseine | Rare |
| `omen` | (above) | | |
| `scream` | 2 | All Hallow's Eve, Endless Scream | Trigger threshold |
| `shadow` (counter) | 2 | Minas Morgul Dark Fortress, The Lux Foundation Library | (C cross-ref) |
| `sickness` | 1 | (corpus rare — likely regex artifact for "summoning sickness") | |
| `sleight` | 1 | Chromatic Armor | Equipment-charge |
| `bloodline` | (above) | | |
| `art` | 1 | Famous Museum | (joke set) |
| `traffic` | 1 | Spaghetti Junction | (joke set) |
| `keyword` | 1 | Mad Labs | (joke set) |
| `token` | 1 | The Tokenator | (joke set) |
| `stroopwafel` | 1 | Bag of Stroopwafels | (joke set) |
| `shoe` | 1 | Shoe Tree | (joke set) |
| `twenty` | 1 | Randy Buehler Bio | (joke set) |
| `glass` | 1 | Sky Deck | (joke set) |
| `spooky` | 1 | Sorin's Remastered Manor | (joke set) |
| `milk` | 1 | Dairy Cow | (joke set) |
| `third-degree-burn` | 1 | Red-Hot Hottie | (joke set) |
| `bargle` | (above) | | |
| `hack` | 1 | Truss Chief Engineer | Rare (Acquisitions Inc) |
| `hole` | 1 | Impressive Rat | Rare |
| `knickknack` | 1 | Tchotchke Elemental | Rare |
| `rebuilding` | 1 | Slobad Actually Just Fine | Rare |
| `midway` | 1 | Myra the Magnificent | Rare |
| `shy` | 1 | Shy Town | Rare |

(Counters in this table number 197 entries by name, some cross-referenced from above categories. Card counts sourced from the regex sweep — single-card counters may include both the original printing and an `A-` Alchemy variant counted together.)

---

## Cross-cutting engine implementation notes

1. **Counter persistence on phase-out (CR §702.26).** Phasing does not remove counters. Engine: phased-out permanents keep their counter map intact.
2. **Counters on tokens.** Tokens can have counters. On token LTB the counters die with the token. Tokens reanimated (rare) come back as new objects without counters.
3. **Hidden-information counters.** Manifest and Mask use face-down state, not counters — but Mairsil's `cage counter` IS a real counter that tracks exiled-with-Mairsil cards. Engine must keep an exile-link side-table for Mairsil.
4. **Counter additions outside §122.1g.** "If you would put one or more counters on a permanent, put twice that many instead" (Doubling Season) IS §122.1g. "If you would create a token, create two instead" is NOT — that's §122.1g's sibling rule §111.10b for tokens. Engine: separate pipelines.
5. **Solemnity / Melira / Suncleanser (counter-prevention).** "Players can't get poison counters" / "Permanents you control can't have counters placed on them" — these are §614 prevention effects, modeled at the counter-placement pipeline entry. They short-circuit before Doubling Season modifications apply.
6. **Vampire Hexmage / Hex Parasite (counter removal).** "Remove all/N counters of a specific kind from target permanent" — separate engine API distinct from §704.5r pairing. Removes literal count, fires "counter removed" trigger (e.g. Forgotten Ancient).
7. **The Ozolith (counter recovery).** When a creature with counters on it dies, those counters move to The Ozolith. Engine: on LTB, snapshot counters into The Ozolith's recovery side-table if a recovery effect is active.
8. **Triggered abilities on counter placement/removal.** Many cards trigger on "puts a +1/+1 counter on" (Forgotten Ancient, Pir Imaginative Rascal, Conclave Mentor). Engine: emit `counter_placed` / `counter_removed` events with `(seat, perm, kind, delta)` for trigger fan-out.
9. **Counter-name uniqueness.** Two counters of the same name on the same permanent are NOT distinct — they're a count. Two counters of different names ARE distinct (Vraan with 2 flying counters and 1 lifelink counter has count 2 of `flying`, count 1 of `lifelink`). Engine: `map[CounterKind]int` per permanent + per seat.
10. **Atypical counter targets.** Spells in exile (suspend), cards in graveyard (rare — almost none, but Quest for the Holy Relic counts? — actually no, that's on a permanent), creatures via Aura (Pemmin's Aura no, that's a static). Engine: counters live on permanents on the battlefield, players, or suspended cards in exile. No other zones.

---

## Sources

- Comprehensive Rules effective 2025-09 reading of §122 (counters), §306 (planeswalkers), §310 (battles), §400.7 (new-object on zone change), §614 (replacement effects), §701.27 (proliferate), §701.50 (stun), §702.24/32/62/63 (cumulative upkeep / fading / suspend / vanishing), §704.5c/i/r/x (SBA loss/cancel triggers), §716 (Sagas).
- Oracle text corpus: Scryfall bulk `oracle-cards.json` (37,384 cards, last fetched 2026-04-30).
- Extraction tool: `/tmp/counter_extract.py` (regex sweep with handcrafted blacklist filtering verb forms of "counter" and possessive artifacts).

---

## Open follow-ups

- **§614 counter-placement pipeline.** Build the engine-level "counter placement modifier" pipeline that handles Doubling Season, Hardened Scales, Branching Evolution, Conclave Mentor, Innkeeper's Talent, Vorinclex Voice of Hunger, Solemnity (prevention), Melira Sylvok Outcast (poison/-1/-1 prevention), Suncleanser. Single pipeline; all modifiers + preventions registered as continuous effects on the placement event.
- **§122.6 persistence regression test.** Pin Vraan + Humility example as a property test: lifelink counter present → Humility ETBs → lifelink doesn't function → Humility leaves → lifelink functions again.
- **§704.5r pair-removal regression.** Pin "creature with 5 +1/+1 and 3 -1/-1" → ends at "2 +1/+1, 0 -1/-1" in one SBA pass.
- **Energy & player-counter §122.1g exemption.** Pin a property test: Doubling Season + opponent's infect damage on you → you get N poison, not 2N. Doubling Season + your own "get {E}" → you get N {E}, not 2N.
- **Counter-removal event emission.** Audit per_card handlers that read counters to confirm they all emit `counter_removed` / `counter_placed` events for downstream trigger fan-out (Forgotten Ancient, Conclave Mentor, Felidar Retreat, etc.).
- **Joke-set counters.** Confirm engine generic-counter plumbing covers all joke-set counters without per-card scaffolding (12 counter types: stroopwafel/shoe/twenty/bargle/glass/traffic/keyword/token/spooky/milk/third-degree-burn/art).
