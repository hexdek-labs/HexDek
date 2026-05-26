# WotC Commander Precon Corpus Inventory (R60, Phase A)

## Why

This is the authoritative reference for every WotC Commander preconstructed deck published as of **2026-05-26** and the import status of each against `data/decks/wizards/`. Phase A of the background precon investigation: builds the universe so Phase B can fan out the per-deck shape scans (Phase C runs the scan on EVERY precon, not just B4-flagged ones — so this inventory drives the full scan-task generation).

The R1-R7 wave docs (PR #508, PR #523, PR #532, PR #534, PR #533, PR #535, PR #536) each imported ~15 precons and cross-validated PR #513's bracket findings. Across those 7 waves, 87 unique precons landed on disk (R4 and R5 ended up importing the same 15 IDs after a sequencing collision; R7 re-included 3 already-imported R6 decks — the per-wave 15-deck total double-counts those 18 duplicates, hence the colloquial "105 across R1-R7" reads higher than the on-disk count). The wave-by-wave growth covers ~60% of the live universe; ~40% remain unsampled, including 4 entire CLB / CMM blocks and the bulk of post-2024 set Commander products.

## Sources cross-referenced

- **WotC announcements** — `magic.wizards.com/en/news/announcements/<set>-commander-decklists` archive
- **MTG Wiki / Fandom** — per-set `/Commander_decks` subpages for the annual Commander series (C11, C13–C21) and per-set Commander tie-ins (AFR onward)
- **EDHRec set pages** — `edhrec.com/sets/<code>` for set/precon metadata
- **Scryfall set metadata** — set codes and release dates
- **Moxfield "Commander Precons" namespace** — canonical stock-list uploader (also the import surface for R1-R7)
- **MTGGoldfish / Draftsim / Card Kingdom blog** — decklist confirmation for the modern UB releases (40K, WHO, LTR, PIP, FIN)

Explicit exclusions (out of scope — NOT precons in the 100-card Commander format sense):

- Game Night Free-for-All / Game Night 2018/2019/2020 — 60-card multiplayer
- Duel Decks — 60-card head-to-head
- Conspiracy / Battlebond / Mystery Booster — drafted formats
- Commander Anthology I / II / Planechase Anthology — repackages of existing precons, not new entries
- Commander Collection Black / Green / Red — single-card showcases, not full decks
- Spider-Man (SPM, 2025) Welcome Decks — 60-card teach product per WotC's explicit "no Commander precons" announcement
- Marvel Super Heroes Commander (June 2026, 4 decks) — UNRELEASED at the time of writing

## Aggregate counts

| Status | Count | % of universe |
|--------|------:|:-------------:|
| **Total WotC Commander precons (released)** | **144** | 100% |
| Imported into `data/decks/wizards/` | **87** | **60.4%** |
| Missing (released but not imported) | **57** | **39.6%** |
| Unreleased (announced but not yet shipped) | 4+ | — |

## Breakdown by era

Eras grouped by product family. % column = imported / total released in era.

| Era | Set / Product | Released | Imported | Missing | % Coverage |
|----:|---------------|---------:|---------:|--------:|:----------:|
| Annual Commander 2011 | C11 | 5 | 2 | 3 | 40% |
| Annual Commander 2013 | C13 | 5 | 2 | 3 | 40% |
| Annual Commander 2014 | C14 | 5 | 1 | 4 | 20% |
| Annual Commander 2015 | C15 | 5 | 3 | 2 | 60% |
| Annual Commander 2016 | C16 | 5 | 1 | 4 | 20% |
| Annual Commander 2017 | C17 | 4 | 1 | 3 | 25% |
| Annual Commander 2018 | C18 | 4 | 2 | 2 | 50% |
| Annual Commander 2019 | C19 | 4 | **4** | 0 | **100%** |
| Annual Commander 2020 | C20 / Ikoria | 5 | 4 | 1 | 80% |
| Annual Commander 2021 | C21 / Strixhaven | 5 | 4 | 1 | 80% |
| Commander Legends | CMR (2020) | 2 | 0 | 2 | 0% |
| Per-set Commander | AFR (2021) | 4 | **4** | 0 | **100%** |
| Per-set Commander | MID (2021) | 2 | **2** | 0 | **100%** |
| Per-set Commander | VOW (2021) | 2 | **2** | 0 | **100%** |
| Per-set Commander | NEO (2022) | 2 | 1 | 1 | 50% |
| Per-set Commander | SNC (2022) | 5 | **5** | 0 | **100%** |
| Commander Legends | CLB (2022) | 4 | 0 | 4 | 0% |
| Universes Beyond | 40K (2022) | 4 | **4** | 0 | **100%** |
| Per-set Commander | DMU (2022) | 2 | 1 | 1 | 50% |
| Per-set Commander | BRO (2022) | 2 | **2** | 0 | **100%** |
| Commander Masters | CMM (2023) | 4 | 0 | 4 | 0% |
| Per-set Commander | ONE (2023) | 2 | **2** | 0 | **100%** |
| Per-set Commander | MOM (2023) | 5 | 3 | 2 | 60% |
| Universes Beyond | LTR (2023) | 4 | **4** | 0 | **100%** |
| Per-set Commander | WOE (2023) | 2 | 0 | 2 | 0% |
| Universes Beyond | WHO (2023) | 4 | 2 | 2 | 50% |
| Per-set Commander | LCI (2023) | 4 | 1 | 3 | 25% |
| Per-set Commander | MKM (2024) | 4 | 3 | 1 | 75% |
| Per-set Commander | OTJ (2024) | 4 | **4** | 0 | **100%** |
| Per-set Commander | MH3 (2024) | 4 | **4** | 0 | **100%** |
| Per-set Commander | BLB (2024) | 4 | **4** | 0 | **100%** |
| Per-set Commander | DSK (2024) | 4 | 3 | 1 | 75% |
| Universes Beyond | PIP / Fallout (2024) | 4 | 3 | 1 | 75% |
| Per-set Commander | DFT / Aetherdrift (2025) | 2 | 1 | 1 | 50% |
| Per-set Commander | TDM / Tarkir Dragonstorm (2025) | 5 | 1 | 4 | 20% |
| Universes Beyond | FIN / Final Fantasy (2025) | 4 | 1 | 3 | 25% |
| Per-set Commander | EoE / Edge of Eternities (2025) | 2 | **2** | 0 | **100%** |
| Secret Lair Commander | Secrets of Strixhaven (Apr 2026) | 5 | 3 | 2 | 60% |
| Secret Lair Commander | Everyone's Invited! (SL 2025) | 1 | **1** | 0 | **100%** |
| **TOTALS** | | **144** | **87** | **57** | **60.4%** |

### Coverage themes

- **9 eras at 100%**: C19 / AFR / MID / VOW / SNC / 40K / BRO / ONE / LTR / OTJ / MH3 / BLB / EoE (12 sets actually — corrects the "9" undercount). These provide the densest per-era signal for any post-fix calibration regression.
- **3 eras at 0%** (zero coverage): CMR (2020), CLB (2022), WOE (2023). 8 unsampled precons across the three blocks; CMM (2023) is also 0% if you count it (also 4 unsampled).
- **Older annual Commander coverage is the thinnest**: C11/C13/C14/C16/C17 all sit at ≤40%. The corpus tilts modern — useful for "is the bracket estimator correct on contemporary precons" questions, less useful for "does the bracket estimator regress on legacy precons."

## Ranked list (chronological by era, then alphabetical within era)

Status legend: **IMPORTED** = present in `data/decks/wizards/`; **MISSING** = released and importable but not yet in the corpus; **UNRELEASED** = announced/expected, not yet on market.

### Annual Commander series (C11–C21)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
|  1 | C11 (2011) | Counterpunch | Ghave, Guru of Spores | MISSING |
|  2 | C11 (2011) | Devour for Power | The Mimeoplasm | MISSING |
|  3 | C11 (2011) | Heavenly Inferno | Kaalia of the Vast | IMPORTED |
|  4 | C11 (2011) | Mirror Mastery | Riku of Two Reflections | IMPORTED |
|  5 | C11 (2011) | Political Puppets | Zedruu the Greathearted | MISSING |
|  6 | C13 (2013) | Eternal Bargain | Oloro, Ageless Ascetic | IMPORTED |
|  7 | C13 (2013) | Evasive Maneuvers | Derevi, Empyrial Tactician | MISSING |
|  8 | C13 (2013) | Mind Seize | Jeleva, Nephalia's Scourge | IMPORTED |
|  9 | C13 (2013) | Nature of the Beast | Marath, Will of the Wild | MISSING |
| 10 | C13 (2013) | Power Hungry | Prossh, Skyraider of Kher | MISSING |
| 11 | C14 (2014) | Built from Scratch | Daretti, Scrap Savant | IMPORTED |
| 12 | C14 (2014) | Forged in Stone | Nahiri, the Lithomancer | MISSING |
| 13 | C14 (2014) | Guided by Nature | Freyalise, Llanowar's Fury | MISSING |
| 14 | C14 (2014) | Peer Through Time | Teferi, Temporal Archmage | MISSING |
| 15 | C14 (2014) | Sworn to Darkness | Ob Nixilis of the Black Oath | MISSING |
| 16 | C15 (2015) | Call the Spirits | Daxos the Returned | MISSING |
| 17 | C15 (2015) | Plunder the Graves | Meren of Clan Nel Toth | IMPORTED |
| 18 | C15 (2015) | Seize Control | Mizzix of the Izmagnus | IMPORTED |
| 19 | C15 (2015) | Swell the Host | Ezuri, Claw of Progress | MISSING |
| 20 | C15 (2015) | Wade into Battle | Kalemne, Disciple of Iroas | IMPORTED |
| 21 | C16 (2016) | Breed Lethality | Atraxa, Praetors' Voice | IMPORTED |
| 22 | C16 (2016) | Entropic Uprising | Yidris, Maelstrom Wielder | MISSING |
| 23 | C16 (2016) | Invent Superiority | Breya, Etherium Shaper | MISSING |
| 24 | C16 (2016) | Open Hostility | Saskia the Unyielding | MISSING |
| 25 | C16 (2016) | Stalwart Unity | Kynaios and Tiro of Meletis | MISSING |
| 26 | C17 (2017) | Arcane Wizardry | Inalla, Archmage Ritualist | MISSING |
| 27 | C17 (2017) | Draconic Domination | The Ur-Dragon | MISSING |
| 28 | C17 (2017) | Feline Ferocity | Arahbo, Roar of the World | MISSING |
| 29 | C17 (2017) | Vampiric Bloodlust | Edgar Markov | IMPORTED |
| 30 | C18 (2018) | Adaptive Enchantment | Estrid, the Masked | MISSING |
| 31 | C18 (2018) | Exquisite Invention | Saheeli, the Gifted | MISSING |
| 32 | C18 (2018) | Nature's Vengeance | Lord Windgrace | IMPORTED |
| 33 | C18 (2018) | Subjective Reality | Aminatou, the Fateshifter | IMPORTED |
| 34 | C19 (2019) | Faceless Menace | Volrath, the Shapestealer | IMPORTED |
| 35 | C19 (2019) | Merciless Rage | Anje Falkenrath | IMPORTED |
| 36 | C19 (2019) | Mystic Intellect | Sevinne, the Chronoclasm | IMPORTED |
| 37 | C19 (2019) | Primal Genesis | Ghired, Conclave Exile | IMPORTED |
| 38 | C20 (2020 / IKO) | Arcane Maelstrom | Kalamax, the Stormsire | MISSING |
| 39 | C20 (2020 / IKO) | Enhanced Evolution | Otrimi, the Ever-Playful | IMPORTED |
| 40 | C20 (2020 / IKO) | Ruthless Regiment | Jirina Kudro | IMPORTED |
| 41 | C20 (2020 / IKO) | Symbiotic Swarm | Kathril, Aspect Warper | IMPORTED |
| 42 | C20 (2020 / IKO) | Timeless Wisdom | Gavi, Nest Warden | IMPORTED |
| 43 | C21 (2021 / STX) | Lorehold Legacies | Osgir, the Reconstructor | IMPORTED |
| 44 | C21 (2021 / STX) | Prismari Performance | Zaffai, Thunder Conductor | MISSING |
| 45 | C21 (2021 / STX) | Quantum Quandrix | Adrix and Nev, Twincasters | IMPORTED |
| 46 | C21 (2021 / STX) | Silverquill Statement | Breena, the Demagogue | IMPORTED |
| 47 | C21 (2021 / STX) | Witherbloom Witchcraft | Willowdusk, Essence Seer | IMPORTED |

### Commander Legends sets (CMR, CLB)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
| 48 | CMR (2020) | Arm for Battle | Kamahl, Heart of Krosa | MISSING |
| 49 | CMR (2020) | Reap the Tides | Aesi, Tyrant of Gyre Strait | MISSING |
| 50 | CLB (2022) | Draconic Dissent | Firkraag, Cunning Instigator | MISSING |
| 51 | CLB (2022) | Exit from Exile | Nalia de'Arnise | MISSING |
| 52 | CLB (2022) | Mind Flayarrrs | Captain N'ghathrod | MISSING |
| 53 | CLB (2022) | Party Time | Faldorn, Dread Wolf Herald | MISSING |

### Per-set Commander products (AFR onward)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
| 54 | AFR (2021) | Aura of Courage | Galea, Kindred Hunter | IMPORTED |
| 55 | AFR (2021) | Draconic Rage | Vrondiss, Rage of Ancients | IMPORTED |
| 56 | AFR (2021) | Dungeons of Death | Sefris of the Hidden Ways | IMPORTED |
| 57 | AFR (2021) | Planar Portal | Prosper, Tome-Bound | IMPORTED |
| 58 | MID (2021) | Coven Counters | Leinore, Autumn Sovereign | IMPORTED |
| 59 | MID (2021) | Undead Unleashed | Wilhelt, the Rotcleaver | IMPORTED |
| 60 | VOW (2021) | Spirit Squadron | Millicent, Restless Revenant | IMPORTED |
| 61 | VOW (2021) | Vampiric Bloodline | Strefan, Maurer Progenitor | IMPORTED |
| 62 | NEO (2022) | Buckle Up | Kotori, Pilot Prodigy | IMPORTED |
| 63 | NEO (2022) | Upgrades Unleashed | Light-Paws, Emperor's Voice | MISSING |
| 64 | SNC (2022) | Bedecked Brokers | Falco Spara, Pactweaver | IMPORTED |
| 65 | SNC (2022) | Cabaretti Cacophony | Jinnie Fay, Jetmir's Second | IMPORTED |
| 66 | SNC (2022) | Maestros Massacre | Anhelo, the Painter | IMPORTED |
| 67 | SNC (2022) | Obscura Operation | Kamiz, Obscura Oculus | IMPORTED |
| 68 | SNC (2022) | Riveteers Rampage | Henzie "Toolbox" Torre | IMPORTED |
| 69 | DMU (2022) | Legends' Legacy | Dihada, Binder of Wills | MISSING |
| 70 | DMU (2022) | Painbow | Jared Carthalion | IMPORTED |
| 71 | BRO (2022) | Mishra's Burnished Banner | Mishra, Claimed by Gix | IMPORTED |
| 72 | BRO (2022) | Urza's Iron Alliance | Urza, Chief Artificer | IMPORTED |
| 73 | ONE (2023) | Corrupting Influence | Ixhel, Scion of Atraxa | IMPORTED |
| 74 | ONE (2023) | Rebellion Rising | Jor Kadeen, First Goldwarden | IMPORTED |
| 75 | MOM (2023) | Call for Backup | Bright-Palm, Soul Awakener | MISSING |
| 76 | MOM (2023) | Cavalry Charge | Sidar Jabari of Zhalfir | IMPORTED |
| 77 | MOM (2023) | Divine Convocation | Kasla, the Broken Halo | IMPORTED |
| 78 | MOM (2023) | Growing Threat | Brimaz, Blight of Oreskos | IMPORTED |
| 79 | MOM (2023) | Tinker Time | Gimbal, Gremlin Prodigy | MISSING |
| 80 | WOE (2023) | Fae Dominion | Tegwyll, Duke of Splendor | MISSING |
| 81 | WOE (2023) | Virtue and Valor | Ellivere of the Wild Court | MISSING |
| 82 | LCI (2023) | Ahoy Mateys | Admiral Brass, Unsinkable | MISSING |
| 83 | LCI (2023) | Blood Rites | Clavileño, First of the Blessed | MISSING |
| 84 | LCI (2023) | Explorers of the Deep | Hakbal of the Surging Soul | MISSING |
| 85 | LCI (2023) | Veloci-Ramp-Tor | Pantlaza, Sun-Favored | IMPORTED |
| 86 | MKM (2024) | Blame Game | Nelly Borca, Impulsive Accuser | IMPORTED |
| 87 | MKM (2024) | Deadly Disguise | Kaust, Eyes of the Glade | IMPORTED |
| 88 | MKM (2024) | Deep Clue Sea | Morska, Undersea Sleuth | IMPORTED |
| 89 | MKM (2024) | Revenant Recon | Mirko, Obsessive Theorist | MISSING |
| 90 | OTJ (2024) | Desert Bloom | Yuma, Proud Protector | IMPORTED |
| 91 | OTJ (2024) | Grand Larceny | Gonti, Canny Acquisitor | IMPORTED |
| 92 | OTJ (2024) | Most Wanted | Olivia, Opulent Outlaw | IMPORTED |
| 93 | OTJ (2024) | Quick Draw | Stella Lee, Wild Card | IMPORTED |
| 94 | MH3 (2024) | Creative Energy | Satya, Aetherflux Genius | IMPORTED |
| 95 | MH3 (2024) | Eldrazi Incursion | Ulalek, Fused Atrocity | IMPORTED |
| 96 | MH3 (2024) | Graveyard Overdrive | Geralf, the Fleshver | IMPORTED |
| 97 | MH3 (2024) | Tricky Terrain | Omo, Queen of Vesuva | IMPORTED |
| 98 | BLB (2024) | Animated Army | Bello, Bard of the Brambles | IMPORTED |
| 99 | BLB (2024) | Family Matters | Zinnia, Valley's Voice | IMPORTED |
|100 | BLB (2024) | Peace Offering | Ms. Bumbleflower | IMPORTED |
|101 | BLB (2024) | Squirreled Away | Hazel of the Rootbloom | IMPORTED |
|102 | DSK (2024) | Death Toll | Winter, Cynical Opportunist | IMPORTED |
|103 | DSK (2024) | Endless Punishment | Kaalia of the Cult | IMPORTED |
|104 | DSK (2024) | Jump Scare | Zimone, Mystery Unraveler | IMPORTED |
|105 | DSK (2024) | Mind Your Manors | Valgavoth, Terror Eater | MISSING |
|106 | DFT (2025) | Eternal Might | Temmet, Naktamun's Will | MISSING |
|107 | DFT (2025) | Living Energy | Saheeli, Radiant Creator | IMPORTED |
|108 | TDM (2025) | Abzan Armor | Felothar, Dawn of the Abzan | MISSING |
|109 | TDM (2025) | Jeskai Striker | Shiko, Paragon of the Way | MISSING |
|110 | TDM (2025) | Mardu Surge | Kologhan, Tyrant of Frenzy | MISSING |
|111 | TDM (2025) | Sultai Arisen | Kotis, Sibsig Champion | MISSING |
|112 | TDM (2025) | Temur Roar | Ureni of the Unwritten | IMPORTED |
|113 | EoE (2025) | Counter Intelligence | Inspirit, Flagship Vessel | IMPORTED |
|114 | EoE (2025) | World Shaper | Hearthhull, the Worldseed | IMPORTED |

### Commander Masters (CMM)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
|115 | CMM (2023) | Eldrazi Unbound | Zhulodok, Void Gorger | MISSING |
|116 | CMM (2023) | Enduring Enchantments | Anikthea, Hand of Erebos | MISSING |
|117 | CMM (2023) | Planeswalker Party | Commodore Guff | MISSING |
|118 | CMM (2023) | Sliver Swarm | Sliver Gravemother | MISSING |

### Universes Beyond — Warhammer 40K (2022)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
|119 | 40K (2022) | Forces of the Imperium | Inquisitor Greyfax | IMPORTED |
|120 | 40K (2022) | Necron Dynasties | Szarekh, the Silent King | IMPORTED |
|121 | 40K (2022) | The Ruinous Powers | Abaddon the Despoiler | IMPORTED |
|122 | 40K (2022) | Tyranid Swarm | Magus Lucea Kane | IMPORTED |

### Universes Beyond — Lord of the Rings (2023)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
|123 | LTR (2023) | Elven Council | Galadriel, Elven-Queen | IMPORTED |
|124 | LTR (2023) | Food and Fellowship | Frodo, Sauron's Bane / Sam, Loyal Attendant | IMPORTED |
|125 | LTR (2023) | Riders of Rohan | Éowyn, Shieldmaiden | IMPORTED |
|126 | LTR (2023) | The Hosts of Mordor | Sauron, the Dark Lord | IMPORTED |

### Universes Beyond — Doctor Who (2023)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
|127 | WHO (2023) | Blast from the Past | The Fourth Doctor / Sarah Jane Smith | IMPORTED |
|128 | WHO (2023) | Masters of Evil | The Master, Multiplied / Missy | MISSING |
|129 | WHO (2023) | Paradox Power | The Thirteenth Doctor / Yasmin Khan | IMPORTED |
|130 | WHO (2023) | Timey-Wimey | The Tenth Doctor / Rose Tyler | MISSING |

### Universes Beyond — Fallout (2024)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
|131 | PIP (2024) | Hail, Caesar! | Caesar, Legion's Emperor | MISSING |
|132 | PIP (2024) | Mutant Menace | The Master, Transcendent | IMPORTED |
|133 | PIP (2024) | Science! | Dr. Madison Li | IMPORTED |
|134 | PIP (2024) | Scrappy Survivors | Nick Valentine, Private Eye | IMPORTED |

### Universes Beyond — Final Fantasy (2025)

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
|135 | FIN (2025) | Counter Blitz (Final Fantasy X) | Tidus, Yuna's Guardian / Yuna, Grand Summoner | MISSING |
|136 | FIN (2025) | Limit Break (Final Fantasy VII) | Cloud, Ex-SOLDIER | IMPORTED |
|137 | FIN (2025) | Revival Trance (Final Fantasy VI) | Terra, Herald of Hope | MISSING |
|138 | FIN (2025) | Scions & Spellcraft (Final Fantasy XIV) | Y'shtola, Night's Blessed | MISSING |

### Secret Lair Commander products

| # | Era | Precon | Commander | Status |
|---|-----|--------|-----------|:------:|
|139 | SoS (Apr 2026) | Lorehold Spirit | Quintorius, History Chaser | IMPORTED |
|140 | SoS (Apr 2026) | Prismari Artistry | Rootha, Mastering the Moment | MISSING |
|141 | SoS (Apr 2026) | Quandrix Unlimited | Zimone, Infinite Analyst | MISSING |
|142 | SoS (Apr 2026) | Silverquill Influence | Killian, Decisive Mentor | IMPORTED |
|143 | SoS (Apr 2026) | Witherbloom Pestilence | Dina, Essence Brewer | IMPORTED |
|144 | SL 2025 | Everyone's Invited! | Ms. Bumbleflower (alternate-art) | IMPORTED |

### Unreleased (announced or expected, NOT YET on market as of 2026-05-26)

| Era | Precon | Commander | Status | Notes |
|-----|--------|-----------|:------:|-------|
| Marvel Super Heroes (Jun 2026) | TBD | Doctor Doom (confirmed for one) | UNRELEASED | 4 decks; commanders not all named; release 2026-06-26 |
| Secret Lair Commander 2026 | Goblin Storm | Zada, Hedron Grinder | UNRELEASED | Uploaded to Moxfield Commander Precons namespace ahead of release |

## Phase B / C considerations

This inventory is the input for Phase B (per-deck shape-scan task generation). Notes for the Phase B planner:

1. **57 missing imports** are the immediate scan-task expansion target. Of those, the highest signal density (based on R1-R7 findings) lives in:
   - **CLB (2022, 4 decks)** — entire block unsampled; Battle for Baldur's Gate has a wildly different design philosophy from C-series precons (D&D mechanics, partner pairs, party mechanic). Expected to surface novel bracket-classifier behavior.
   - **CMM (2023, 4 decks)** — Commander Masters is a "premium" Commander set deliberately tuned hotter than annual C-series. Expected to stress-test the GC=0 ceiling (Most Wanted's R5 inverse-miss pattern).
   - **C16 (2016, 4 missing)** — only Atraxa (Breed Lethality) imported; the other four are partner-led 4-color decks. Expected to surface partner-commander synergy patterns the current corpus doesn't have.
   - **CMR (2020, 2 decks)** — original Commander Legends; 2 base precons (Arm for Battle, Reap the Tides) ship at higher power than annual C-series.
   - **Older annual C-series (C11/C13/C14/C17)** — the coldest end of the distribution; useful for B1-floor calibration (R3 surfaced 4 B1 false-positives across the existing low-power decks).
2. **Phase C scan scope:** the user noted "per-deck shape scans will run for EVERY precon (not just B4-flagged ones)." With the current 87-deck corpus already imported, that's 87 scan tasks ready to fan out; the 57 missing imports add 57 more scan tasks (144 total). The fan-out can be batched at the per-era granularity for review-ergonomics.
3. **Unreleased entries** (Marvel Super Heroes, Goblin Storm) are flagged but should be deferred to Phase B's "future waves" planning. Re-running this inventory monthly through 2026 will catch any new WotC announcements.

## Reproducing

```bash
# Re-count imported precons:
ls data/decks/wizards/*.txt | wc -l

# Find missing precons (entries marked MISSING in this doc): cross-reference
# this file's tables against the on-disk filenames.

# Re-fetch the canonical announcement page for any set:
#   https://magic.wizards.com/en/news/announcements/<set>-commander-decklists
```

## Source citations (per era — for Phase B reviewer cross-check)

- C11 / C13–C21: WotC announcement pages + MTG Wiki `/Commander_decks` subpages
- CMR / CLB: WotC announcement pages
- AFR–EoE per-set Commanders: WotC announcement pages (canonical URL: `magic.wizards.com/en/news/announcements/<set-slug>-commander-decklists`)
- CMM: WotC announcement (`commander-masters-commander-decklists`)
- 40K / LTR / WHO / PIP / FIN: WotC Universes Beyond announcement pages
- Secrets of Strixhaven Secret Lair: `magic.wizards.com/en/news/announcements/secrets-of-strixhaven-commander-decklists`
- Everyone's Invited! Secret Lair Commander 2025: Moxfield Commander Precons namespace upload (canonical-stock-list reference; WotC announcement under the Secret Lair 2025 superdrop notes)
- Marvel Super Heroes (UNRELEASED): WotC product roadmap announcements + Draftsim "MaRo Confirms New Marvel Commander Precons" coverage
- Spider-Man Welcome Decks (EXCLUDED): WotC explicit "no Commander precons" statement per the The Gamer article "MTG's Spider-Man Set Won't Have Commander Precons"
