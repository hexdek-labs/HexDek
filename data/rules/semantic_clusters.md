# Semantic Cluster Report

Pool: **31,655 real cards** (filtered: tokens, schemes, planes, silver-bordered, bios).

## Headline

- **Unique semantic clusters: 4,950**
- KEYWORDS_ONLY (vanilla + pure keyword cards, single canonical handler): **1,938** (6.1%)
- UNPARSED (effects the analyzer can't bucket → custom-handler candidates): **7,366** (23.3%)
- Singletons (clusters of one — truly unique effect signatures): **3,459** clusters / 3,459 cards (10.9%)

## Engine-build curve

| Handlers built | Cards covered | % of pool |
|---:|---:|---:|
| Top 10 | 14,922 | 47.1% |
| Top 30 | 17,437 | 55.1% |
| Top 50 | 18,986 | 60.0% |
| Top 100 | 21,080 | 66.6% |
| Top 200 | 23,083 | 72.9% |
| Top 500 | 25,582 | 80.8% |
| Top 1,000 | 27,214 | 86.0% |
| Top 2,000 | 28,705 | 90.7% |

## Top 30 clusters (highest-ROI handlers to build first)

| # | Cards | Effect signature | Sample cards |
|---:|---:|---|---|
| 1 | 7,366 | `UNPARSED` | Sensory Deprivation, Torrent of Fire, Waterknot, … +7363 |
| 2 | 1,938 | `KEYWORDS_ONLY` | Storm Crow, Ravnica at War, Wei Strike Force, … +1935 |
| 3 | 1,466 | `act_mana` | Palinchron, Whispering Shade, Food Fight, … +1463 |
| 4 | 958 | `act_mana` + `act_tap` | Walking Sponge, Hua Tuo, Honored Physician, Magma Sliver, … +955 |
| 5 | 680 | `act_mana` + `act_tap` + `mana_ability(out=1_color)` | Shizo, Death's Storehouse, Timber Gorge, Game Trail, … +677 |
| 6 | 638 | `trig_etb` | Disposal Mummy, Toluz, Clever Conductor, Pheres-Band Brawler, … +635 |
| 7 | 602 | `draw(n=1)` | Veil of Summer, Kindred Discovery, Theft of Dreams, … +599 |
| 8 | 490 | `counter_p1p1(n=1)` | Yorvo, Lord of Garenbrig, Optimistic Scavenger, Thran Vigil, … +487 |
| 9 | 486 | `token` + `token_other` | Lattice Library, Tyranid Harridan, Doomsday Confluence, … +483 |
| 10 | 298 | `conditional_static` | Static Orb, Guul Draz Vampire, Tanglewalker, … +295 |
| 11 | 218 | `act_mana` + `draw(n=1)` | Savai Triome, Rampaging War Mammoth, Shefet Archfiend, … +215 |
| 12 | 207 | `trig_attack` | Murk Dwellers, Steadfast Cathar, Ondu Rising, … +204 |
| 13 | 156 | `etb_with_counters` | Gravetiller Wurm, Flycatcher Giraffid, Savage Firecat, … +153 |
| 14 | 149 | `trig_death` | Sultai Emissary, Friendly Teddy, Meren of Clan Nel Toth, … +146 |
| 15 | 147 | `as_long_as` + `conditional_static` | Summit Apes, Wingrattle Scarecrow, Grixis Grimblade, … +144 |
| 16 | 139 | `library_bottom` | Harald, King of Skemfar, Green Sun's Twilight, Collected Company, … +136 |
| 17 | 127 | `cost_reduce(n=1)` | Starnheim Courser, Mocking Sprite, Brighthearth Banneret, … +124 |
| 18 | 126 | `combat_restriction` | Fear of Infinity, Desperate Castaways, Hulking Cyclops, … +123 |
| 19 | 121 | `restriction` | The Foretold Soldier, Lydia Frye, Bog Rats, … +118 |
| 20 | 117 | `trig_phase` + `trig_upkeep` | Hag of Inner Weakness, Justice, Anowon, the Ruin Sage, … +114 |
| 21 | 116 | `act_mana` + `act_tap` + `mana_ability(out=1_any)` | Lantern of Revealing, Divergent Growth, Flywheel Racer, … +113 |
| 22 | 113 | `self_discard(n=1)` | Patchwork Gnomes, Party Thrasher, Snow Day, … +110 |
| 23 | 104 | `prevent_damage` | Comeuppance, Gideon's Intervention, Master Apothecary, … +101 |
| 24 | 103 | `token` + `token_other` + `trig_etb` | Nimble Thopterist, Wedgelight Rammer, Nightsquad Commando, … +100 |
| 25 | 100 | `modal_one` | Blinding Beam, Bloodline Culling, Steel Sabotage, … +97 |
| 26 | 99 | `destroy(target=creature)` | Vivisection Evangelist, Stand Up for Yourself, Misfortune's Gain, … +96 |
| 27 | 95 | `act_mana` + `act_tap` + `draw(n=1)` | Krovikan Sorcerer, Fiery Islet, Illuminated Folio, … +92 |
| 28 | 95 | `token_copy` + `token_other` | Endless Evil, Espers to Magicite, Mist-Syndicate Naga, … +92 |
| 29 | 92 | `counter_p1p1(n=1)` + `trig_etb` | Foe-liage, General Leo Cristophe, Sazh Katzroy, … +89 |
| 30 | 91 | `aura_keyword` | Ghostly Touch, Sky Tether, Robe of Mirrors, … +88 |

## Cluster size distribution

| Cluster size | Number of clusters | Cards covered |
|---:|---:|---:|
| 1 | 3,459 | 3,459 |
| 2-5 | 1,031 | 2,814 |
| 6-20 | 323 | 3,353 |
| 21-100 | 113 | 5,164 |
| 101-500 | 17 | 3,217 |
| 501+ | 7 | 13,648 |

## What this means

To get every printed card to GREEN, the engine needs **4,949 effect handlers** (minus the UNPARSED bucket, which still needs analyzer iteration before it's actionable).

But the distribution is heavy-tailed — the top 30 handlers cover **55.1%** of the pool, and the top 200 cover **72.9%**.

The 3,459 singleton clusters are the real custom-handler queue. Most of those are jank that won't see play; the EDHRec play-rate weighting pass (next step, pending green light) will rank them by competitive relevance so we build handlers in deck-impact order rather than alphabetically.
