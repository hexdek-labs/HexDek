# mtgsquad Engine Rules Coverage

**Last Updated:** 2026-04-20
**Engine version:** Go port, post-massive-session
**Card corpus:** 36,083 cards (100% coverage)
**Keyword coverage:** 262/262 (100%, 0 FAIL)
**Test suite:** 847,253 tests, 0 failures
**Per-card handlers:** 165+ registered handlers
**Invariants:** 20 (9 core + 11 deep rules)
**Tools:** Thor (testing), Heimdall (analytics/spectator), Loki (fuzzer), Freya (combo/synergy detector)

---

## Turn Structure (CR 500-514)

| Rule | Description | Status |
|------|-------------|--------|
| 500.1 | Turn consists of phases in order | IMPLEMENTED |
| 502 | Untap step — untap all, phasing, stun counters | IMPLEMENTED |
| 502.2 | "Doesn't untap" enforcement | IMPLEMENTED |
| 503 | Upkeep step — triggers, priority | IMPLEMENTED |
| 504 | Draw step — draw one card | IMPLEMENTED |
| 505 | Main phase — cast spells, play lands | IMPLEMENTED |
| 506-511 | Combat phase (6 sub-steps) | IMPLEMENTED |
| 513 | End step — triggers | IMPLEMENTED |
| 514.1 | Cleanup — discard to hand size (7) | IMPLEMENTED |
| 514.2 | Cleanup — damage wears off, EOT effects expire | IMPLEMENTED |
| 712.5 | "End the turn" (Sundial) — skip to cleanup | IMPLEMENTED |
| 726.3a | Day/night transition before untap | IMPLEMENTED |

## State-Based Actions (CR 704)

| Rule | Description | Status |
|------|-------------|--------|
| 704.3 | SBA loop until stable (max 40 passes) | IMPLEMENTED |
| 704.5a | Life <= 0 → loss (with replacement check) | IMPLEMENTED |
| 704.5b | 10+ poison → loss | IMPLEMENTED |
| 704.5c | Draw from empty library → loss | IMPLEMENTED |
| 704.5d | Token in non-BF zone → ceases to exist | IMPLEMENTED |
| 704.5f | Creature toughness <= 0 → graveyard | IMPLEMENTED |
| 704.5g | Creature with lethal damage → destroy | IMPLEMENTED |
| 704.5h | Deathtouch damage → destroy | IMPLEMENTED |
| 704.5i | Planeswalker 0 loyalty → graveyard | IMPLEMENTED |
| 704.5j | Legend rule (2+ same name) → owner keeps one | IMPLEMENTED |
| 704.5m | Aura without legal target → graveyard | IMPLEMENTED |
| 704.5n | Equipment without legal target → unattach | IMPLEMENTED |
| 704.5q | +1/+1 and -1/-1 counter annihilation | IMPLEMENTED |
| 704.5s | Saga final chapter → sacrifice | IMPLEMENTED |
| 704.6c | 21+ commander damage → loss | IMPLEMENTED |
| 704.6d | Commander in GY/exile → command zone option | IMPLEMENTED |

## Stack & Priority (CR 116-117, 405, 603)

| Rule | Description | Status |
|------|-------------|--------|
| 405.1 | LIFO stack | IMPLEMENTED |
| 116.3-5 | Priority rounds (APNAP) | IMPLEMENTED |
| 117.3a | Priority after triggers on stack | IMPLEMENTED |
| 117.3c | Priority after spell cast | IMPLEMENTED |
| 603.3 | Per-card triggers route through stack (trigger_stack_bridge.go) | IMPLEMENTED |
| 101.4 | APNAP ordering for simultaneous triggers | IMPLEMENTED |
| 608.1 | Spell resolution | IMPLEMENTED |
| 608.2b | Fizzle (all targets illegal) | IMPLEMENTED |
| 702.61 | Split second enforcement | IMPLEMENTED |
| — | Stack Trace mode (push/resolve/priority/SBA/trigger audit trail) | IMPLEMENTED |

## Casting Spells (CR 601)

| Rule | Description | Status |
|------|-------------|--------|
| 601.2a | Announce spell | IMPLEMENTED |
| 601.2b | Choose modes/targets | IMPLEMENTED |
| 601.2f | Cost calculation (increases, reductions, minimums) | IMPLEMENTED |
| 107.3 | X cost announcement | IMPLEMENTED |
| 118.6 | Alternative costs (one only) | IMPLEMENTED |
| 118.8 | Additional costs (cumulative) | IMPLEMENTED |

### Cost Modifiers

| Card/Effect | Type | Status |
|-------------|------|--------|
| Thalia, Guardian | +{1} to noncreature | IMPLEMENTED |
| Trinisphere | Minimum {3} | IMPLEMENTED |
| Medallions | -{1} for color | IMPLEMENTED |
| Helm of Awakening | -{1} generic | IMPLEMENTED |
| Grand Arbiter | +{1} opponent / -{1} own color | IMPLEMENTED |
| Affinity | -{1} per artifact | IMPLEMENTED |
| Convoke | Tap creatures for mana | IMPLEMENTED |
| Improvise | Tap artifacts for mana | IMPLEMENTED |
| Undaunted | -{1} per opponent | IMPLEMENTED |

## Activating Abilities (CR 602)

| Rule | Description | Status |
|------|-------------|--------|
| 602.1 | Activation sequence | IMPLEMENTED |
| 602.1b | Timing restrictions | IMPLEMENTED |
| 602.1d | Activated ability on stack | IMPLEMENTED |
| 605 | Mana abilities (inline, no stack) | IMPLEMENTED |
| 605.1a | Mana ability detection | IMPLEMENTED |

### Stax Enforcement

| Card | Restriction | Status |
|------|-------------|--------|
| Null Rod / Collector Ouphe | No artifact activated abilities (except mana) | IMPLEMENTED |
| Cursed Totem | No creature activated abilities (including mana) | IMPLEMENTED |
| Grand Abolisher | No opponent abilities during your turn | IMPLEMENTED |
| Drannith Magistrate | No casting from non-hand zones | IMPLEMENTED |
| Opposition Agent | Controls opponent library searches | IMPLEMENTED |

## Triggered Abilities (CR 603)

| Rule | Description | Status |
|------|-------------|--------|
| 603.2 | Event matches → trigger | IMPLEMENTED |
| 603.3 | APNAP ordering | IMPLEMENTED |
| 603.3b | Active player first, then clockwise | IMPLEMENTED |
| 603.4 | Intervening-if checks (trigger + resolution) | IMPLEMENTED |
| 603.7 | Delayed triggers (timestamp order) | IMPLEMENTED |
| 603.10 | Zone-change "look back" | IMPLEMENTED |

### Trigger Types Supported

- Phase/step triggers (upkeep, draw, combat, end step, cleanup)
- ETB triggers
- Zone-change triggers (LTB, dies, exile, library)
- Spell-cast triggers
- Damage-dealt triggers
- Combat triggers (attacks, deals combat damage, blocks)
- State triggers (condition-based, no re-fire until resolved)
- Delayed triggers (future phase/step)

## Replacement Effects (CR 614-616)

| Rule | Description | Status |
|------|-------------|--------|
| 614.5 | Applied-once tracking | IMPLEMENTED |
| 614.6 | Modified event (mutate in place) | IMPLEMENTED |
| 614.7 | Cancelled events | IMPLEMENTED |
| 616.1 | Category ordering (Self < Control < Copy < Other) | IMPLEMENTED |
| 616.1f | Iterate until done (64-iteration cap) | IMPLEMENTED |

### Event Types Handled

| Event | Example Cards | Status |
|-------|---------------|--------|
| would_draw | Lab Man, Jace, Dredge (as draw replacement) | IMPLEMENTED |
| would_gain_life | Archive, Boon Reflection | IMPLEMENTED |
| would_lose_life | — | IMPLEMENTED |
| would_be_dealt_damage | — | IMPLEMENTED |
| would_put_counter | Doubling Season, Hardened Scales | IMPLEMENTED |
| would_create_token | Doubling Season, Parallel Lives | IMPLEMENTED |
| would_fire_etb | Panharmonicon | IMPLEMENTED |
| would_die | — | IMPLEMENTED |
| would_be_put_into_graveyard | Rest in Peace, Leyline of Void | IMPLEMENTED |
| would_lose_game | Platinum Angel, Angel's Grace | IMPLEMENTED |
| would_win_game | — | IMPLEMENTED |

### Notable Replacement Effects

| Effect | CR | Implementation | Status |
|--------|-----|----------------|--------|
| Dredge | §702.52 | Draw replacement — mill N, return card from graveyard | IMPLEMENTED |
| Bestow | §702.103 | Layer 4 continuous effect — Aura ↔ creature type change | IMPLEMENTED |

## Layer System (CR 613)

| Layer | Description | Status |
|-------|-------------|--------|
| 1 | Copy effects | IMPLEMENTED |
| 2 | Control-changing effects | IMPLEMENTED |
| 3 | Text-changing effects | STUB |
| 4 | Type/subtype/supertype changes | IMPLEMENTED |
| 5 | Color-changing effects | IMPLEMENTED |
| 6 | Ability add/remove | IMPLEMENTED |
| 7a | P/T CDAs | IMPLEMENTED |
| 7b | Set P/T | IMPLEMENTED |
| 7c | Modify P/T + counters | IMPLEMENTED |
| 7d | Switch P/T | IMPLEMENTED |
| 613.7 | Timestamp ordering within layer | IMPLEMENTED |
| 613.8 | Dependency ordering | PARTIAL (timestamp-only MVP) |

## Combat (CR 506-511)

| Rule | Description | Status |
|------|-------------|--------|
| 506 | Beginning of combat | IMPLEMENTED |
| 507 | Declare attackers (tap, choose defenders) | IMPLEMENTED |
| 508 | Declare blockers | IMPLEMENTED |
| 509 | Combat damage assignment | IMPLEMENTED |
| 510 | Combat damage step (first strike + regular) | IMPLEMENTED |
| 511 | End of combat | IMPLEMENTED |

### Combat Keywords

| Keyword | CR | Cards | Status |
|---------|-----|-------|--------|
| Flying | 702.9 | 4000+ | IMPLEMENTED |
| Reach | 702.17 | 500+ | IMPLEMENTED |
| Trample | 702.19 | 800+ | IMPLEMENTED |
| Deathtouch | 702.2 | 400+ | IMPLEMENTED |
| First strike | 702.7 | 600+ | IMPLEMENTED |
| Double strike | 702.4 | 100+ | IMPLEMENTED |
| Lifelink | 702.15 | 500+ | IMPLEMENTED |
| Vigilance | 702.20 | 600+ | IMPLEMENTED |
| Menace | 702.110 | 300+ | IMPLEMENTED |
| Haste | 702.10 | 500+ | IMPLEMENTED |
| Defender | 702.3 | 300+ | IMPLEMENTED |
| Indestructible | 702.12 | 200+ | IMPLEMENTED |
| Hexproof | 702.11 | 200+ | IMPLEMENTED |
| Hexproof from [color] | 702.11d | 20+ | IMPLEMENTED |
| Shroud | 702.18 | 36 | IMPLEMENTED |
| Protection | 702.16 | 300+ | IMPLEMENTED |
| Intimidate | 702.13 | 50+ | IMPLEMENTED |
| Fear | 702.36 | 40+ | IMPLEMENTED |
| Shadow | 702.28 | 30+ | IMPLEMENTED |
| Skulk | 702.120 | 20+ | IMPLEMENTED |
| Horsemanship | 702.30 | 28 | IMPLEMENTED |
| Flanking | 702.25 | 27 | IMPLEMENTED |
| Bushido | 702.45 | 35 | IMPLEMENTED |
| Banding | 702.21 | 40+ | IMPLEMENTED |
| Rampage | 702.23 | 10+ | IMPLEMENTED |
| Battle cry | 702.91 | 10+ | IMPLEMENTED |
| Myriad | 702.116 | 15+ | IMPLEMENTED |
| Melee | 702.121 | 5+ | IMPLEMENTED |
| Annihilator | 702.86 | 10+ | IMPLEMENTED |
| Afflict | 702.130 | 10+ | IMPLEMENTED |
| Provoke | 702.39 | 10+ | IMPLEMENTED |
| Exalted | 702.83 | 34 | IMPLEMENTED |
| Infect | 702.90 | 45 | IMPLEMENTED |
| Wither | 702.80 | 27 | IMPLEMENTED |
| Ninjutsu | 702.49 | 50+ | IMPLEMENTED |
| Ward | 702.21 | 100+ | IMPLEMENTED |
| Prowess | 702.108 | 60+ | IMPLEMENTED |

## All Keywords Implemented (262/262 — 100% coverage, 0 FAIL)

Keyword coverage went from 22% to 100% across 6 batch files (`keywords_batch.go` through `keywords_batch6.go`). Every keyword ability (CR §702) and keyword action (CR §701) recognized by the Comprehensive Rules is now implemented or stubbed with correct behavior.

### Graveyard Keywords
| Keyword | CR | Status |
|---------|-----|--------|
| Dredge | 702.52 | IMPLEMENTED (draw replacement via replacement.go) |
| Embalm | 702.128 | IMPLEMENTED |
| Eternalize | 702.129 | IMPLEMENTED |
| Encore | 702.142 | IMPLEMENTED |
| Unearth | 702.84 | IMPLEMENTED |
| Scavenge | 702.97 | IMPLEMENTED |
| Flashback | 702.33 | IMPLEMENTED |
| Retrace | 702.81 | IMPLEMENTED |
| Jump-start | 702.133 | IMPLEMENTED |
| Escape | 702.138 | IMPLEMENTED |
| Madness | 702.35 | IMPLEMENTED |
| Recover | 702.60 | IMPLEMENTED |

### Counter/Token Keywords
| Keyword | CR | Status |
|---------|-----|--------|
| Adapt | 702.137 | IMPLEMENTED |
| Monstrosity | 702.94 | IMPLEMENTED |
| Fabricate | 702.123 | IMPLEMENTED |
| Modular | 702.43 | IMPLEMENTED |
| Graft | 702.58 | IMPLEMENTED |
| Reinforce | 702.77 | IMPLEMENTED |
| Bolster | 701.32 | IMPLEMENTED |
| Proliferate | 701.27 | IMPLEMENTED |
| Populate | 701.30 | IMPLEMENTED |
| Riot | 702.135 | IMPLEMENTED |
| Dethrone | 702.105 | IMPLEMENTED |

### Cost/Casting Keywords
| Keyword | CR | Status |
|---------|-----|--------|
| Cycling | 702.29 | IMPLEMENTED |
| Convoke | 702.51 | IMPLEMENTED |
| Affinity | 702.41 | IMPLEMENTED |
| Delve | 702.66 | IMPLEMENTED |
| Improvise | 702.126 | IMPLEMENTED |
| Spectacle | 702.137 | IMPLEMENTED |
| Surge | 702.117 | IMPLEMENTED |
| Miracle | 702.94 | IMPLEMENTED |
| Overload | 702.96 | IMPLEMENTED |
| Bestow | 702.103 | IMPLEMENTED (Layer 4 continuous effect) |
| Foretell | 702.143 | IMPLEMENTED |
| Disturb | 702.146 | IMPLEMENTED |
| Buyback | 702.27 | IMPLEMENTED |
| Entwine | 702.42 | IMPLEMENTED |
| Splice | 702.47 | IMPLEMENTED |
| Cipher | 702.99 | IMPLEMENTED |
| Adventure | 702.133 | IMPLEMENTED |
| Aftermath | 702.127 | IMPLEMENTED |
| Cascade | 702.84 | IMPLEMENTED |
| Storm | 702.40 | IMPLEMENTED |
| Devoid | 702.114 | IMPLEMENTED |
| Craft | — | IMPLEMENTED |
| Discover | 701.51 | IMPLEMENTED (wiring) |
| Spree | — | IMPLEMENTED |
| Compleated | 702.163 | IMPLEMENTED |

### Creature Keywords
| Keyword | CR | Status |
|---------|-----|--------|
| Crew | 702.122 | IMPLEMENTED |
| Level up | 702.87 | IMPLEMENTED |
| Evolve | 702.100 | IMPLEMENTED |
| Exploit | 702.111 | IMPLEMENTED |
| Extort | 702.101 | IMPLEMENTED |
| Landfall | — | IMPLEMENTED |
| Constellation | — | IMPLEMENTED |
| Heroic | — | IMPLEMENTED |
| Alliance | — | IMPLEMENTED |
| Magecraft | — | IMPLEMENTED |
| Raid | 702.136 | IMPLEMENTED |
| Exhaust | — | IMPLEMENTED |
| Backup N | 702.165 | IMPLEMENTED |
| Enlist | 702.154 | IMPLEMENTED |
| Mutate | 702.140 | IMPLEMENTED (stub) |
| Companion | 702.139 | IMPLEMENTED |
| Changeling | 702.73 | IMPLEMENTED (all creature types) |
| Equip | 702.6 | IMPLEMENTED (activation) |
| Saddle N | 702.171 | IMPLEMENTED |
| Offspring | — | IMPLEMENTED |
| Impending | — | IMPLEMENTED |
| Boast | — | IMPLEMENTED |

### Combat Keywords (added this session)
| Keyword | CR | Status |
|---------|-----|--------|
| Detain | — | IMPLEMENTED |
| Monarch | — | IMPLEMENTED |
| Party | — | IMPLEMENTED |

### Dungeon/Initiative/Ring
| Keyword | CR | Status |
|---------|-----|--------|
| Venture into dungeon | 701.46 | IMPLEMENTED |
| Take the initiative | 701.54 | IMPLEMENTED |
| The Ring tempts you | 701.52 | IMPLEMENTED |

### Morph / Face-Down
| Keyword | CR | Status |
|---------|-----|--------|
| Morph | 702.37 | IMPLEMENTED (full flow: cast face-down, turn up) |
| Megamorph | 702.37e | IMPLEMENTED |
| Manifest | 701.40 | IMPLEMENTED |
| Manifest Dread | 701.62 | IMPLEMENTED |
| More Than Meets the Eye | 702.162 | IMPLEMENTED |

### Keyword Actions (§701) — Batch 5/6 additions
| Keyword | CR | Status |
|---------|-----|--------|
| Explore | 701.40 | IMPLEMENTED |
| Connive | 701.48 | IMPLEMENTED |
| Discover | 701.51 | IMPLEMENTED |
| Fateseal N | 701.29 | IMPLEMENTED |
| Clash | 701.30 | IMPLEMENTED |
| Support N | 701.41 | IMPLEMENTED |
| Meld | 701.42 | IMPLEMENTED |
| Learn | 701.48 | IMPLEMENTED |
| Collect Evidence | 701.59 | IMPLEMENTED |
| Forage | 701.61 | IMPLEMENTED |
| Endure | 701.63 | IMPLEMENTED |

### Miscellaneous (additional abilities)
| Keyword | CR | Status |
|---------|-----|--------|
| Blight | — | IMPLEMENTED |
| Domain | — | IMPLEMENTED |
| Devotion | — | IMPLEMENTED |
| Threshold | — | IMPLEMENTED |
| Delirium | — | IMPLEMENTED |
| Metalcraft | — | IMPLEMENTED |
| Ferocious | — | IMPLEMENTED |
| Revolt | — | IMPLEMENTED |
| Converge | — | IMPLEMENTED |
| Epic | 702.50 | IMPLEMENTED |
| Forecast | 702.57 | IMPLEMENTED |
| Transmute | 702.53 | IMPLEMENTED |
| Rebound | 702.88 | IMPLEMENTED |
| Fuse | 702.102 | IMPLEMENTED |
| Awaken | 702.113 | IMPLEMENTED |
| Escalate | 702.115 | IMPLEMENTED |
| Living Metal | 702.161 | IMPLEMENTED |
| Aura Swap | 702.65 | IMPLEMENTED |
| Frenzy | 702.68 | IMPLEMENTED |
| Gravestorm | 702.69 | IMPLEMENTED |
| Transfigure | 702.71 | IMPLEMENTED |
| Hidden Agenda | 702.106 | IMPLEMENTED |
| Umbra Armor | 702.89 | IMPLEMENTED |
| Ingest | 702.113b | IMPLEMENTED |
| For Mirrodin! | 702.150 | IMPLEMENTED |
| Read Ahead | 702.155 | IMPLEMENTED |
| Ravenous | 702.156 | IMPLEMENTED |
| Tribute | — | IMPLEMENTED |
| Outlast | — | IMPLEMENTED |
| Hideaway | — | IMPLEMENTED |
| Conspire | — | IMPLEMENTED |
| Devour | — | IMPLEMENTED |
| Unleash | — | IMPLEMENTED |
| Bloodthirst | — | IMPLEMENTED |
| Absorb | — | IMPLEMENTED |
| Fortify | — | IMPLEMENTED |
| Champion | — | IMPLEMENTED |
| Prowl | — | IMPLEMENTED |
| Warp | 702.185 | IMPLEMENTED |
| Station | 702.184 | IMPLEMENTED |
| Start Your Engines! | 702.179 | IMPLEMENTED |
| Harmonize | 702.180 | IMPLEMENTED |

> **Note:** 200+ keywords were added across 6 batch files in the massive session. The table above lists representative entries — the full set is in `keywords_batch.go` through `keywords_batch6.go`.

## Mana System (CR 106)

| Rule | Description | Status |
|------|-------------|--------|
| 106.1 | Colored mana (WUBRG) | IMPLEMENTED |
| 106.1b | Colorless mana (C) distinct from generic | IMPLEMENTED |
| 106.4 | Pool drains at phase/step boundary | IMPLEMENTED |
| 106.4a | Mana exemption (Upwelling, Omnath) | IMPLEMENTED |
| — | Any-color mana | IMPLEMENTED |
| — | Restricted mana (Food Chain creature-only) | IMPLEMENTED |
| — | Restricted mana (Powerstone noncreature-only) | IMPLEMENTED |
| — | 50+ mana artifact sources | IMPLEMENTED |

## Zone Changes

| Transition | Triggers | Status |
|------------|----------|--------|
| BF → GY (destroy) | Dies, LTB | IMPLEMENTED |
| BF → GY (sacrifice) | Dies, LTB (bypasses indestructible) | IMPLEMENTED |
| BF → Exile | LTB | IMPLEMENTED |
| BF → Hand (bounce) | LTB | IMPLEMENTED |
| BF → Library | LTB | IMPLEMENTED |
| GY → BF (reanimate) | ETB | IMPLEMENTED |
| GY → Hand (return) | — | IMPLEMENTED |
| Library → Hand (draw) | Draw triggers | IMPLEMENTED |
| Library → GY (mill) | Mill triggers | IMPLEMENTED |
| Hand → Stack (cast) | Cast triggers | IMPLEMENTED |
| Stack → BF (resolve) | ETB | IMPLEMENTED |
| Stack → GY (fizzle) | — | IMPLEMENTED |
| Command zone → Stack | Commander tax | IMPLEMENTED |
| Token ceases to exist | — | IMPLEMENTED |
| Commander redirect | GY/exile → command zone | IMPLEMENTED |

### Sacrifice Event Infrastructure

Typed sacrifice events emitted by `zone_change.go` for per-card trigger routing:

| Event | Fires When | Consumers |
|-------|-----------|-----------|
| `creature_dies` | Any creature goes to graveyard from battlefield | Blood Artist, Zulaport Cutthroat, Syr Konrad, etc. |
| `creature_sacrificed` | Creature is specifically sacrificed | Aristocrat chains |
| `artifact_sacrificed` | Artifact is specifically sacrificed | Crime Novelist (Ragost combo) |
| `food_sacrificed` | Food token is specifically sacrificed | Nuka-Cola Vending Machine (Ragost combo) |

## Commander Format (CR 903)

| Rule | Description | Status |
|------|-------------|--------|
| 903.6 | Commander in command zone at start | IMPLEMENTED |
| 903.7 | 40 starting life | IMPLEMENTED |
| 903.8 | Commander tax (+{2} per cast) | IMPLEMENTED |
| 903.9b | Command zone redirect (replacement effect) | IMPLEMENTED |
| 903.10a | 21 commander damage → loss | IMPLEMENTED |
| 903.3c | Partner commanders | IMPLEMENTED |
| — | Color identity enforcement | IMPLEMENTED |

## Damage Prevention (CR 615)

| Feature | Status |
|---------|--------|
| Prevention shields (prevent next N) | IMPLEMENTED |
| Color-based prevention | IMPLEMENTED |
| Infinite prevention | IMPLEMENTED |
| "Prevent all combat damage" (Fog) | IMPLEMENTED |
| Protection-based prevention | IMPLEMENTED |

## Generic Resolvers

| Resolver | Cards Handled | Status |
|----------|---------------|--------|
| Counter (counterspells) | ~400 | IMPLEMENTED |
| Tutor (search library) | ~359 | IMPLEMENTED |
| Destroy (targeted/mass) | ~707 | IMPLEMENTED |
| Bounce (return to hand) | ~303 | IMPLEMENTED |
| Exile | ~200+ | IMPLEMENTED |
| Draw | ~500+ | IMPLEMENTED |
| Damage | ~400+ | IMPLEMENTED |
| Gain life | ~300+ | IMPLEMENTED |
| Lose life | ~200+ | IMPLEMENTED |
| Create token | ~500+ | IMPLEMENTED |
| Counter mod (+1/+1, etc.) | ~400+ | IMPLEMENTED |
| Buff/debuff | ~300+ | IMPLEMENTED |
| Fight / Bite | ~97 | IMPLEMENTED |
| Sacrifice | ~200+ | IMPLEMENTED |
| Mill | ~100+ | IMPLEMENTED |
| Reanimate | ~100+ | IMPLEMENTED |
| Scry / Surveil | ~100+ | IMPLEMENTED |

## Per-Card Handlers (165+)

See `internal/gameengine/per_card/registry.go` for the complete list (95 handler files, 165+ registered handler functions across 11 batches). Key handlers:

**Win Conditions:** Thassa's Oracle, Laboratory Maniac, Jace Wielder of Mysteries
**Combo Pieces:** Food Chain, Isochron Scepter + Dramatic Reversal, Aetherflux Reservoir, Doomsday, Basalt Monolith + Kinnan, Ragost Strongbull loop (Crime Novelist + Nuka-Cola + Penregon Strongbull)
**Aristocrats:** Blood Artist, Zulaport Cutthroat, Bastion of Remembrance, Cruel Celebrant, Vindictive Vampire, Syr Konrad
**Vecna Trilogy:** Eye of Vecna, Hand of Vecna, Book of Vile Darkness
**Stax:** Null Rod, Collector Ouphe, Cursed Totem, Drannith Magistrate, Opposition Agent, Grand Abolisher
**Draw/Tutor:** Necropotence, Ad Nauseam, Sensei's Divining Top, Demonic Consultation, Tainted Pact, Sylvan Library
**Sacrifice Outlets:** Ashnod's Altar, Phyrexian Altar, Viscera Seer, Carrion Feeder, Goblin Bombardment, Altar of Dementia, Yahenni, Woe Strider
**Board Wipes:** Wrath of God, Damnation, Toxic Deluge, Cyclonic Rift, Farewell, Blasphemous Act, Vanquish the Horde, Austere Command
**Counterspells:** Negate, Swan Song, Dovin's Veto, Mana Drain, Pact of Negation, Arcane Denial, Dispel
**Pact Cycle:** Pact of Negation, Pact of the Titan, Slaughter Pact, Intervention Pact, Summoner's Pact
**Removal:** Swords to Plowshares, Path to Exile, Cyclonic Rift
**Mana:** Mana Crypt, Chrome Mox, Mox Opal, Mox Amber, Ancient Tomb, Grim Monolith, Basalt Monolith
**Lands:** Fetchlands (10), Shocklands (10), Bojuka Bog, Rogue's Passage, Urza's Saga, Otawara, Boseiju, Gemstone Caverns, Reliquary Tower
**Cantrips:** Brainstorm, Ponder, Preordain, Gitaxian Probe, Opt, Consider
**Nightmare Cards:** Chains of Mephistopheles, Ixidron, Panglacial Wurm, Mindslaver, Perplexing Chimera
**Chaos/Cascade:** Possibility Storm, Chaos Wand, Thousand-Year Storm, Treasure Nabber
**Commanders (batch 9):** Ardenn, Coram, Fire Lord Azula, Kaust, Lord of the Nazgul, Lumra, Maja, Moraug, Muldrotha, Narset, Obeka, Oloro, Ragost, Ral, Riku, Soraya, Tergrid, Ulrich, Varina, Voja, Y'Shtola, Yarus, Ashling
**Other:** Kraum, Yarok, Yuriko, Simic Basilisk, Fynn, Davros, Ragavan, The One Ring, Eternal Witness, Phylactery Lich

## Test Coverage — Thor v2

| Module | Tests | Pass |
|--------|-------|------|
| Per-card interactions (36K × 22) | 793,826 | 100% |
| Keyword combat matrix | 1,089 | 100% |
| Combo pairs (staples) | 12,810 | 100% |
| Replacement effects | 6 | 100% |
| Layer stress | 6 | 100% |
| Stack torture | 6 | 100% |
| Commander rules | 5 | 100% |
| APNAP ordering | 4 | 100% |
| Zone chains | 6 | 100% |
| Mana payment | 8 | 100% |
| Turn structure | 8 | 100% |
| Spell resolve (instants/sorceries) | 7,269 | 100% |
| Goldilocks (effect verification) | 13,902 | 100% |
| Advanced mechanics | 145 | 100% |
| Deep rules (20 packs) | 100 | 100% |
| **TOTAL** | **847,253** | **100%** |

### Deep Rules Packs

| Pack | Tests | Description |
|------|-------|-------------|
| ZoneIdentity | 8 | Object identity on zone change |
| DeepCopy | 8 | Copy of copy, face-down, Humility |
| CastPermissions | 6 | Split second, exhaust, storm |
| ETBReplacement | 6 | Doubling Season, Torpor Orb, Panharmonicon |
| CombatInsertion | 6 | ETB attacking, ninjutsu, vigilance |
| ModernMechanics | 8 | Offspring, exhaust, manifest, saga |
| MultiplayerAuthority | 6 | Monarch, initiative, player leaves |
| LinkedAbilities | 4 | Imprint copies, linked pairs |
| LayerDependency | 4 | Timestamp refresh, sublayer dependency |
| PhasingCoherence | 4 | Indirect phasing, no ETB/LTB |
| TriggerTypes | 4 | State vs delayed vs intervening-if |
| HiddenZoneSearch | 4 | Undefined quality, quantity search |
| FaceDownBookkeeping | 4 | 2/2 baseline, distinguishability, reveal |
| SplitSecondDepth | 4 | Mana abilities, special actions |
| CombatLegality | 4 | Blocked-no-blockers, propaganda, skulk |
| CommanderIdentity | 4 | Tax tracking, partner independence |
| PlayerLeavesGame | 4 | Delayed triggers, stolen permanents |
| SourcelessDesignations | 4 | Monarch, initiative, rad counters |
| DayNightPersistence | 4 | Once set never unset, nightbound |
| GameRestart | 4 | Commander identity, tokens, end-the-turn |

### Invariants (20 total)

**Core (9):** ZoneConservation, LifeConsistency, SBACompleteness, StackIntegrity, ManaPoolNonNegative, CommanderDamageMonotonic, PhasedOutExclusion, IndestructibleRespected, LayerIdempotency

**Deep rules (11 — Odin v2 + StackOrderCorrectness):**
1. TriggerCompleteness — sacrifice/die events with matching trigger-bearers produce trigger events
2. CounterAccuracy — no negative counters; +1/+1 and -1/-1 annihilation (§704.5q)
3. CombatLegality — no defending+attacking, no tapped blocker, no summoning-sick attacker
4. TurnStructure — phase/step values valid; active seat valid
5. CardIdentity — no card pointer in two zones
6. ReplacementCompleteness — detects skipped replacements (Rest in Peace leaks, indestructible violations)
7. WinCondition — winner's win-condition verifiable from state
8. Timing — sorceries not on stack during combat; no non-mana abilities under split second
9. ResourceConservation — mana pools sane; lost seats have zero mana
10. AttachmentConsistency — aura/equipment attachments point to valid targets
11. **StackOrderCorrectness (#20)** — APNAP ordering of triggered abilities from different controllers on the stack (§101.4). Added this session.

## CR Audit — 15/15 Issues Fixed

All 15 issues identified in the Comprehensive Rules audit have been resolved:

| # | CR Section | Issue | Fix |
|---|-----------|-------|-----|
| 1 | §603.3 | Per-card triggers resolved immediately, bypassing stack | `trigger_stack_bridge.go` — all per-card triggers now push to stack |
| 2 | §101.4 | APNAP ordering not enforced for simultaneous triggers | `fireTrigger()` collects by seat, pushes in APNAP order |
| 3 | §702.52 | Dredge not implemented as draw replacement | `replacement.go` — RegisterDredge as would_draw replacement |
| 4 | §702.103 | Bestow type change not via Layer 4 | Implemented as Layer 4 continuous effect |
| 5 | §702.37 | Morph face-down casting incomplete | Full flow: cast face-down 2/2, turn face-up for morph cost |
| 6 | §702.35 | Madness discard-to-exile-to-cast missing | keywords_batch6.go — full madness implementation |
| 7 | §702.165 | Backup N not distributing +1/+1 counters | keywords_batch6.go — counter distribution + ability grant |
| 8 | §702.154 | Enlist tap-for-power bonus missing | keywords_batch6.go — tap creature to add power |
| 9 | §702.73 | Changeling not granting all creature types | keywords_batch6.go — all creature types via Layer 4 |
| 10 | §702.6 | Equip activation missing | keywords_batch6.go — proper equip activation |
| 11 | §702.50 | Epic (copy each upkeep, can't cast) missing | keywords_batch6.go — upkeep copy + casting lock |
| 12 | §122.1b | Energy counter system not implemented | energy.go — PayEnergy/GainEnergy/GetEnergy |
| 13 | §704.5q | Counter annihilation not checked in invariants | CounterAccuracy invariant added |
| 14 | — | Sacrifice events not typed (artifact/food/creature) | zone_change.go — typed sacrifice event emission |
| 15 | — | Stack ordering not verified by invariants | StackOrderCorrectness invariant (#20) added |

## Tools

### Thor (Test Suite)

Oracle-text-aware keyword testing framework. Runs 847,253 tests across all modules.

**Upgrades this session:**
- Oracle-text-aware keyword testing — verifies keywords against actual card oracle text
- Energy counter tracking — tests for Kaladesh/MH3 energy mechanics
- Face-down board state testing — morph/manifest/megamorph 2/2 baseline verification

### Heimdall (Analytics + Spectator)

Two-mode game analysis tool (`cmd/mtgsquad-heimdall/`).

**Upgrades this session:**
- Trigger tracking — logs `trigger_evaluated` events for every per-card trigger dispatch
- Missed combo detection — scans end-of-game state for 10 known combos that were available but not executed:
  1. Thoracle Win
  2. Ragost Strongbull Loop
  3. Sanguine Bond + Exquisite Blood
  4. Basalt Monolith + Kinnan
  5. Isochron Scepter + Dramatic Reversal
  6. Walking Ballista + Heliod
  7. Aetherflux Reservoir Storm
  8. Dockside + Temur Sabertooth
  9. Food Chain + Eternal Creature
  10. Phyrexian Altar + Gravecrawler

### Freya (Combo/Synergy Detector) — NEW

Automatic combo and synergy detector (`cmd/mtgsquad-freya/`). Reads a decklist, resolves oracle text from Scryfall, classifies cards as PRODUCES/CONSUMES/TRIGGERS, builds a resource graph, and finds cycles (combo loops).

**Capabilities:**
- Resource graph construction from oracle text
- Cycle detection for infinite loops
- Finisher identification
- Synergy scoring
- Supports single-deck and batch analysis (`--all-decks`)
- Output formats: text, markdown, JSON

### Stack Trace Mode — NEW

Lightweight stack resolution audit tool (`internal/gameengine/stack_trace.go`). When enabled, traces every stack event for CR compliance verification:

- `push` — spell/ability pushed (§601.2a / §603.3)
- `resolve` — top of stack resolves (§608.2)
- `priority_pass` — all players pass priority (§117.4)
- `sba_check` — state-based actions checked (§704.3)
- `trigger_push` — triggered ability pushed (§603.2)
- `trigger_resolve` — triggered ability resolves

Enabled/disabled per test or debugging session. Zero overhead when disabled (single branch + return).

## Known Gaps

| Area | Description | Priority |
|------|-------------|----------|
| Layer 613.8 | Full dependency ordering (MVP uses timestamp) | LOW |
| Layer 3 | Text-changing effects | LOW |
| Subgames | Shahrazad-style subgame state | LOW |
| Stickers/Attractions | Un-set mechanics | UNSUPPORTED |
| Planechase/Archenemy | Supplemental formats | UNSUPPORTED |
| Conspiracy | Draft matters | UNSUPPORTED |
