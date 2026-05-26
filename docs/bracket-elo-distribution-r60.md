# Bracket-vs-ELO distribution (r60 snapshot)

Source: `data/hexdek-snapshot.db` table `showmatch_elo` (1319 decks, snapshot date 2026-05-26).

## Global summary (raw ratings)

- decks: **1319**
- min: **-3289.26**
- max: **3368.56**
- mean: **-176.62**
- median: **-186.34**
- stdev: **1495.75**

Per-bracket breakdown:

| Bracket | Decks | Min | Max | Mean | Median | Stdev |
|---:|---:|---:|---:|---:|---:|---:|
| B0 | 12 | -2136.0 | 2151.4 | 1175.6 | 1424.0 | 1142.1 |
| B1 | 85 | -2645.2 | 3139.9 | 625.9 | 712.3 | 1367.8 |
| B2 | 620 | -3171.3 | 3368.6 | 43.9 | 51.1 | 1426.3 |
| B3 | 404 | -3130.6 | 3287.8 | -194.0 | -248.0 | 1374.7 |
| B4 | 89 | -3091.1 | 2883.8 | -576.3 | -765.9 | 1478.4 |
| B5 | 109 | -3289.3 | 2088.7 | -1815.1 | -2063.4 | 1179.3 |

**Observation:** mean rating decreases monotonically as bracket rises (B0 +1175 → B5 −1815). This is the expected showmatch shape: higher-bracket decks face stiffer pods, so lifetime win-rates settle lower than the 1500 starting point even for archetypally strong builds.

## Per-bracket histograms (20-bin ASCII)

### Bracket 0 — 12 decks

```
   -2136.0 …  -1921.7 | ##########                               1
   -1921.7 …  -1707.3 |                                          0
   -1707.3 …  -1492.9 |                                          0
   -1492.9 …  -1278.6 |                                          0
   -1278.6 …  -1064.2 |                                          0
   -1064.2 …   -849.8 |                                          0
    -849.8 …   -635.4 |                                          0
    -635.4 …   -421.1 |                                          0
    -421.1 …   -206.7 |                                          0
    -206.7 …      7.7 |                                          0
       7.7 …    222.1 |                                          0
     222.1 …    436.4 |                                          0
     436.4 …    650.8 |                                          0
     650.8 …    865.2 | ####################                     2
     865.2 …   1079.5 |                                          0
    1079.5 …   1293.9 | ##########                               1
    1293.9 …   1508.3 | ######################################## 4
    1508.3 …   1722.7 | ##########                               1
    1722.7 …   1937.0 |                                          0
    1937.0 …   2151.4 | ##############################           3
```

### Bracket 1 — 85 decks

```
   -2645.2 …  -2356.0 | ########                                 2
   -2356.0 …  -2066.7 | ########                                 2
   -2066.7 …  -1777.5 | ############                             3
   -1777.5 …  -1488.2 |                                          0
   -1488.2 …  -1199.0 | ########                                 2
   -1199.0 …   -909.7 |                                          0
    -909.7 …   -620.5 | ################                         4
    -620.5 …   -331.2 | ################################         8
    -331.2 …    -41.9 | ############################             7
     -41.9 …    247.3 | ########                                 2
     247.3 …    536.6 | ########################                 6
     536.6 …    825.8 | ####################################     9
     825.8 …   1115.1 | ############################             7
    1115.1 …   1404.3 | ########################                 6
    1404.3 …   1693.6 | ######################################## 10
    1693.6 …   1982.8 | ################                         4
    1982.8 …   2272.1 | ############                             3
    2272.1 …   2561.4 | ############                             3
    2561.4 …   2850.6 | ################                         4
    2850.6 …   3139.9 | ############                             3
```

### Bracket 2 — 620 decks

```
   -3171.3 …  -2844.3 | ########                                 12
   -2844.3 …  -2517.3 | ########                                 12
   -2517.3 …  -2190.3 | ##########                               15
   -2190.3 …  -1863.3 | ##############                           22
   -1863.3 …  -1536.3 | ###################                      30
   -1536.3 …  -1209.3 | ######################                   34
   -1209.3 …   -882.3 | ############################             44
    -882.3 …   -555.3 | ##############################           47
    -555.3 …   -228.3 | ############################             43
    -228.3 …     98.6 | ######################################## 62
      98.6 …    425.6 | ##############################           46
     425.6 …    752.6 | ###################################      55
     752.6 …   1079.6 | ##############################           47
    1079.6 …   1406.6 | ############################             44
    1406.6 …   1733.6 | ##############                           22
    1733.6 …   2060.6 | ##################                       28
    2060.6 …   2387.6 | #############                            20
    2387.6 …   2714.6 | #############                            20
    2714.6 …   3041.6 | #######                                  11
    3041.6 …   3368.6 | ####                                     6
```

### Bracket 3 — 404 decks

```
   -3130.6 …  -2809.7 | ######                                   6
   -2809.7 …  -2488.8 | ###########                              11
   -2488.8 …  -2167.9 | ################                         16
   -2167.9 …  -1846.9 | ##################                       18
   -1846.9 …  -1526.0 | ######################                   22
   -1526.0 …  -1205.1 | ##########################               26
   -1205.1 …   -884.2 | ###############################          31
    -884.2 …   -563.3 | #################################        33
    -563.3 …   -242.3 | ######################################## 40
    -242.3 …     78.6 | ###############################          31
      78.6 …    399.5 | #####################################    37
     399.5 …    720.4 | #########################                25
     720.4 …   1041.4 | ################################         32
    1041.4 …   1362.3 | ######################                   22
    1362.3 …   1683.2 | ################                         16
    1683.2 …   2004.1 | ##############                           14
    2004.1 …   2325.1 | #########                                9
    2325.1 …   2646.0 | #####                                    5
    2646.0 …   2966.9 | #####                                    5
    2966.9 …   3287.8 | #####                                    5
```

### Bracket 4 — 89 decks

```
   -3091.1 …  -2792.3 | #########                                2
   -2792.3 …  -2493.6 | ##################                       4
   -2493.6 …  -2194.9 | ###########################              6
   -2194.9 …  -1896.1 | ##################                       4
   -1896.1 …  -1597.4 | ######################################## 9
   -1597.4 …  -1298.6 | ####################################     8
   -1298.6 …   -999.9 | ###############################          7
    -999.9 …   -701.2 | ###########################              6
    -701.2 …   -402.4 | ###########################              6
    -402.4 …   -103.7 | ######################                   5
    -103.7 …    195.1 | ####################################     8
     195.1 …    493.8 | ###############################          7
     493.8 …    792.6 | #############                            3
     792.6 …   1091.3 | #########                                2
    1091.3 …   1390.0 | #########                                2
    1390.0 …   1688.8 | ####                                     1
    1688.8 …   1987.5 | ####                                     1
    1987.5 …   2286.3 | ####                                     1
    2286.3 …   2585.0 | #########                                2
    2585.0 …   2883.8 | ######################                   5
```

### Bracket 5 — 109 decks

```
   -3289.3 …  -3020.4 | ###################################      13
   -3020.4 …  -2751.5 | ######################################## 15
   -2751.5 …  -2482.6 | ###########################              10
   -2482.6 …  -2213.7 | #############################            11
   -2213.7 …  -1944.8 | #####################                    8
   -1944.8 …  -1675.9 | ###########################              10
   -1675.9 …  -1407.0 | ###########################              10
   -1407.0 …  -1138.1 | ################                         6
   -1138.1 …   -869.2 | ################                         6
    -869.2 …   -600.3 | #############                            5
    -600.3 …   -331.4 | ########                                 3
    -331.4 …    -62.5 | ########                                 3
     -62.5 …    206.4 | #####                                    2
     206.4 …    475.3 | ###                                      1
     475.3 …    744.2 | #####                                    2
     744.2 …   1013.1 |                                          0
    1013.1 …   1282.0 |                                          0
    1282.0 …   1550.9 | #####                                    2
    1550.9 …   1819.8 | ###                                      1
    1819.8 …   2088.7 | ###                                      1
```

## Top 5 / bottom 5 outliers per bracket

Z-score = (rating − bracket_mean) / bracket_stdev. Positive z = playing **above** the pack; negative z = playing below.

### Bracket 0 (n=12, μ=1175.6, σ=1142.1)

**Top 5 (playing above pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Sigarda, Font of Blessings | `belgarathrk` | 2151.4 | 237172 | 60528-176644 | +0.85 |
| Sigarda, Font of Blessings | `belgarathrk` | 2118.5 | 236899 | 60463-176436 | +0.83 |
| Sigarda, Font of Blessings | `belgarathrk` | 2069.4 | 308515 | 81043-227472 | +0.78 |
| Queen Marchesa | `versions` | 1531.0 | 416609 | 112839-303770 | +0.31 |
| Queen Marchesa | `versions` | 1479.1 | 253942 | 64198-189744 | +0.27 |

**Bottom 5 (playing below pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Angel's Grace | `versions` | -2136.0 | 315213 | 77695-237518 | -2.90 |
| Adriana, Captain of the Guard | `versions` | 709.7 | 287231 | 70960-216271 | -0.41 |
| Adriana, Captain of the Guard | `versions` | 780.1 | 260804 | 65801-195003 | -0.35 |
| Queen Marchesa | `versions` | 1245.1 | 312522 | 81798-230724 | +0.06 |
| Queen Marchesa | `versions` | 1310.9 | 415383 | 113303-302080 | +0.12 |

### Bracket 1 (n=85, μ=625.9, σ=1367.8)

**Top 5 (playing above pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Fynn, the Fangbearer | `moxfield` | 3139.9 | 290584 | 74855-215729 | +1.84 |
| Bre of Clan Stoutarm | `moxfield` | 3127.3 | 290662 | 74675-215987 | +1.83 |
| Giada, Font of Hope | `moxfield` | 2904.5 | 288676 | 73741-214935 | +1.67 |
| Ureni of the Unwritten | `moxfield` | 2823.2 | 290153 | 74022-216131 | +1.61 |
| Skithiryx, the Blight Dragon | `moxfield` | 2811.8 | 285661 | 72931-212730 | +1.60 |

**Bottom 5 (playing below pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Wyleth, Soul of Steel | `moxfield` | -2645.2 | 285263 | 69842-215421 | -2.39 |
| Svella, Ice Shaper | `moxfield` | -2394.3 | 279371 | 68687-210684 | -2.21 |
| Gorma, the Gullet | `moxfield` | -2186.7 | 275383 | 67789-207594 | -2.06 |
| Yahenni, Undying Partisan | `moxfield` | -2133.4 | 279017 | 68604-210413 | -2.02 |
| Narset, Enlightened Master | `moxfield` | -2005.4 | 276796 | 68091-208705 | -1.92 |

### Bracket 2 (n=620, μ=43.9, σ=1426.3)

**Top 5 (playing above pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Ajani, Nacatl Pariah // Ajani, Nacatl Avenger | `moxfield` | 3368.6 | 158619 | 106553-52066 | +2.33 |
| Chandra, Fire of Kaladesh // Chandra, Roaring Flame | `moxfield` | 3248.9 | 218523 | 86286-132237 | +2.25 |
| Lathril, Blade of the Elves | `moxfield` | 3174.2 | 264485 | 76144-188341 | +2.19 |
| Edgar Markov | `moxfield` | 3162.9 | 282171 | 76614-205557 | +2.19 |
| Edgar Markov | `moxfield` | 3112.7 | 295596 | 75945-219651 | +2.15 |

**Bottom 5 (playing below pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Azami, Lady of Scrolls | `moxfield` | -3171.3 | 291230 | 66712-224518 | -2.25 |
| Meria, Scholar of Antiquity | `moxfield` | -3148.6 | 212814 | 21032-191782 | -2.24 |
| Selenia, Dark Angel | `moxfield` | -3125.8 | 233324 | 29979-203345 | -2.22 |
| Golbez, Crystal Collector | `moxfield` | -3103.5 | 292230 | 71488-220742 | -2.21 |
| Uurg, Spawn of Turg | `moxfield` | -3068.6 | 292227 | 71316-220911 | -2.18 |

### Bracket 3 (n=404, μ=-194.0, σ=1374.7)

**Top 5 (playing above pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Ajani, Nacatl Pariah // Ajani, Nacatl Avenger | `moxfield` | 3287.8 | 196428 | 97811-98617 | +2.53 |
| Edgar Markov | `moxfield` | 3098.1 | 294737 | 75459-219278 | +2.39 |
| Dr. Eggman | `moxfield` | 3053.1 | 293558 | 75067-218491 | +2.36 |
| Giada, Font of Hope | `moxfield` | 3052.5 | 292277 | 74691-217586 | +2.36 |
| Lathril, Blade of the Elves | `moxfield` | 3048.6 | 287811 | 76012-211799 | +2.36 |

**Bottom 5 (playing below pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Glarb, Calamity's Augur | `moxfield` | -3130.6 | 293878 | 71567-222311 | -2.14 |
| Quintorius, History Chaser | `moxfield` | -3039.2 | 292690 | 71597-221093 | -2.07 |
| Thrasios, Triton Hero | `moxfield` | -2982.4 | 292913 | 71473-221440 | -2.03 |
| Tergrid, God of Fright // Tergrid's Lantern | `moxfield` | -2912.9 | 292249 | 71542-220707 | -1.98 |
| Daretti, Scrap Savant | `moxfield` | -2912.3 | 292037 | 71323-220714 | -1.98 |

### Bracket 4 (n=89, μ=-576.3, σ=1478.4)

**Top 5 (playing above pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Ureni of the Unwritten | `moxfield` | 2883.8 | 287241 | 73305-213936 | +2.34 |
| Urza, Chief Artificer | `moxfield` | 2781.8 | 287078 | 73287-213791 | +2.27 |
| Edgar Markov | `moxfield` | 2721.5 | 286751 | 73118-213633 | +2.23 |
| Edgar Markov | `moxfield` | 2671.5 | 284308 | 72382-211926 | +2.20 |
| Edgar Markov | `moxfield` | 2642.9 | 286055 | 72872-213183 | +2.18 |

**Bottom 5 (playing below pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Nicol Bolas, the Ravager // Nicol Bolas, the Arisen | `moxfield` | -3091.1 | 293157 | 71413-221744 | -1.70 |
| Rowan, Scion of War | `moxfield` | -2951.1 | 290253 | 71175-219078 | -1.61 |
| Quintorius, History Chaser | `moxfield` | -2635.4 | 289587 | 71086-218501 | -1.39 |
| Yawgmoth, Thran Physician | `moxfield` | -2580.2 | 286249 | 70514-215735 | -1.36 |
| Arcum Dagsson | `moxfield` | -2569.8 | 287069 | 70613-216456 | -1.35 |

### Bracket 5 (n=109, μ=-1815.1, σ=1179.3)

**Top 5 (playing above pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Kinnan, Bonder Prodigy | `moxfield` | 2088.7 | 273651 | 69554-204097 | +3.31 |
| Valgavoth, Harrower of Souls | `moxfield` | 1732.0 | 255278 | 64524-190754 | +3.01 |
| Sisay, Weatherlight Captain | `moxfield` | 1448.9 | 260166 | 65841-194325 | +2.77 |
| Maelstrom Wanderer | `moxfield` | 1283.3 | 269011 | 68305-200706 | +2.63 |
| Sisay, Weatherlight Captain | `moxfield` | 614.4 | 250561 | 63308-187253 | +2.06 |

**Bottom 5 (playing below pack):**

| Commander | Owner | Rating | Games | W-L | z |
|---|---|---:|---:|---|---:|
| Thrasios, Triton Hero | `moxfield` | -3289.3 | 248827 | 39065-209762 | -1.25 |
| Meria, Scholar of Antiquity | `moxfield` | -3231.1 | 187539 | 11013-176526 | -1.20 |
| Tymna the Weaver | `moxfield` | -3217.8 | 291719 | 70617-221102 | -1.19 |
| Glarb, Calamity's Augur | `moxfield` | -3203.9 | 284624 | 59752-224872 | -1.18 |
| Vial Smasher the Fierce | `moxfield` | -3203.3 | 279756 | 57961-221795 | -1.18 |

## Freya synergy cross-reference

**Status: not available in this snapshot.**

`data/hexdek-snapshot.db` ships the rating table + 3 gauntlet rows + 10 deck_meta rows, but neither a per-deck Freya `DeckProfile` cache nor a synergy-score column. The 1,319 showmatch decks are referenced by `deck_key` only — the `deck` table itself is empty in this snapshot, so raw_json → Freya analysis is not reproducible from the snapshot alone.

To answer the correlation question end-to-end we would need either:

1. The raw deck JSONs co-keyed by `deck_key` (re-run Freya across all 1,319), or
2. A pre-computed `deck_freya_profile` table written by the deckbuilder service (synergy %, archetype, primary roles, power tier counts) joined on `deck_key`.

Option (2) is the cheap path — Freya already emits this in the JSON profile; wiring a snapshot-export of the relevant scalars (synergy_pct, archetype, power_tier_counts) would let this analysis run on every snapshot without re-parsing oracle text. **Recommend filing as a snapshot-schema follow-up.**

**Proxy observation from outlier table:** the top outliers are dominated by low-bracket decks (B2/B3) with extremely high game counts (>150K games each — these are clearly long-running gauntlet anchors, not casual entries). Bottom outliers in B4/B5 are similarly high-volume. So the visible outlier-magnitude is largely a **sample-size artifact** — decks with many games have ratings far from 1500 simply because they've had time to drift. A Freya synergy correlation study should normalize by games played before drawing conclusions.

## Floor-at-0 transformation (linear shift, NOT clamp)

Goal per 7174n1c: user-facing graphs should sit in Quadrant II (positive Y only, no negative ELO values shown). Linear shift preserves **all** relative gaps — the difference between any two decks is identical before and after.

```
MIN_RATING_OFFSET = 3289.26
rating_display    = rating_raw + MIN_RATING_OFFSET
```

**This is a display transform.** Internal storage stays in the raw signed ELO space so deltas continue to compose with the showmatch update math.

### Before vs after (global)

| Metric | Raw | Shifted |
|---|---:|---:|
| min | -3289.26 | 0.00 |
| max | 3368.56 | 6657.82 |
| mean | -176.62 | 3112.63 |
| median | -186.34 | 3102.91 |
| stdev | 1495.75 | 1495.75 |

Stdev and any pairwise gap are invariant under linear shift — confirmed numerically.

### Before vs after (per bracket means)

| Bracket | Raw mean | Shifted mean |
|---:|---:|---:|
| B0 | 1175.6 | 4464.9 |
| B1 | 625.9 | 3915.2 |
| B2 | 43.9 | 3333.2 |
| B3 | -194.0 | 3095.3 |
| B4 | -576.3 | 2713.0 |
| B5 | -1815.1 | 1474.1 |

### Worst / best decks (shifted)

| | Commander | Bracket | Raw | Shifted |
|---|---|---:|---:|---:|
| worst | Thrasios, Triton Hero | B5 | -3289.3 | 0.00 |
| best | Ajani, Nacatl Pariah // Ajani, Nacatl Avenger | B2 | 3368.6 | 6657.82 |

Worst lands at exactly 0.00; everything else strictly positive. Quadrant II shape satisfied.

## Recommended next steps

1. Add a `deck_freya_profile` snapshot table (synergy_pct, archetype, power_tier_counts, primary_roles) so this analysis can include the actual synergy correlation rather than the proxy note above.
2. Wire `MIN_RATING_OFFSET` into the showmatch screen + Heimdall summary render layer — keep raw signed ELO in storage / deltas, only shift at display time.
3. Normalize outlier scans by `games` (z-score on rating-per-100-games) before publishing a 'breakout decks' list — current top/bottom is sample-size dominated.

