# Cluster Priority Report

Pool: **31,655 real cards**, total EDH play-weight: **343.2**

Each cluster's weight = sum of its cards' EDH play-weights (`1/√edhrec_rank`; Sol Ring=1.0, rank-100=0.1, rank-10,000=0.01). Cards with no `edhrec_rank` (~17% of pool, mostly fringe sets) contribute 0.

## Engine-build curve — by play impact vs by raw count

Same handler count, ranked two ways. The play-weighted curve is what
actually matters for an engine targeting competitive commander.

| Handlers built | % cards covered (count-ranked) | % play-weight covered (weight-ranked) |
|---:|---:|---:|
| Top 10 | 47.1% | 46.8% |
| Top 30 | 55.1% | 55.6% |
| Top 50 | 60.0% | 60.8% |
| Top 100 | 66.6% | 67.5% |
| Top 200 | 72.9% | 73.8% |
| Top 500 | 80.8% | 81.7% |
| Top 1,000 | 86.0% | 87.4% |

## Top 30 clusters by play-weight (build these handlers FIRST)

| # | Weight | Cards | Effect signature | Top-played samples |
|---:|---:|---:|---|---|
| 1 | 77.20 | 7,366 | `UNPARSED` | Cultivate (#20), Farseek (#22), Chaos Warp (#29), … +7363 |
| 2 | 17.36 | 680 | `act_mana` + `act_tap` + `mana_ability(out=1_color)` | Reliquary Tower (#10), Rogue's Passage (#19), Thought Vessel (#21), … +677 |
| 3 | 16.19 | 1,938 | `KEYWORDS_ONLY` | Counterspell (#16), Negate (#53), Seedborn Muse (#279), … +1935 |
| 4 | 11.62 | 1,466 | `act_mana` | Skullclamp (#44), Whispersilk Cloak (#313), Mondrak, Glory Dominus (#404), … +1463 |
| 5 | 10.52 | 958 | `act_mana` + `act_tap` | Polluted Delta (#38), Flooded Strand (#41), Misty Rainforest (#42), … +955 |
| 6 | 7.72 | 602 | `draw(n=1)` | Rhystic Study (#40), Esper Sentinel (#75), Mystic Remora (#94), … +599 |
| 7 | 6.23 | 638 | `trig_etb` | Eternal Witness (#99), Accursed Marauder (#488), Massacre Wurm (#514), … +635 |
| 8 | 5.04 | 486 | `token` + `token_other` | Rampaging Baloths (#358), Loyal Apprentice (#710), Ophiomancer (#782), … +483 |
| 9 | 4.76 | 490 | `counter_p1p1(n=1)` | Snakeskin Veil (#479), Unbreakable Formation (#518), Champion of Lambholt (#530), … +487 |
| 10 | 3.92 | 116 | `act_mana` + `act_tap` + `mana_ability(out=1_any)` | Command Tower (#2), Arcane Signet (#3), Exotic Orchard (#9), … +113 |
| 11 | 2.51 | 218 | `act_mana` + `draw(n=1)` | Faerie Mastermind (#343), Jetmir's Garden (#345), Ketria Triome (#349), … +215 |
| 12 | 2.48 | 298 | `conditional_static` | Opposition Agent (#512), Serra Ascendant (#964), Promise of Loyalty (#1070), … +295 |
| 13 | 2.32 | 70 | `act_mana` + `act_tap` + `mana_ability(out=2_color)` | Dimir Signet (#137), Rakdos Signet (#155), Izzet Signet (#164), … +67 |
| 14 | 1.95 | 33 | `act_mana` + `act_tap` + `mana_ability(out=2_C)` | Sol Ring (#1), Ancient Tomb (#64), Temple of the False God (#70), … +30 |
| 15 | 1.94 | 127 | `cost_reduce(n=1)` | Herald's Horn (#139), Foundry Inspector (#254), Jet Medallion (#285), … +124 |
| 16 | 1.73 | 207 | `trig_attack` | Etali, Primal Storm (#257), Shared Animosity (#663), World Shaper (#1264), … +204 |
| 17 | 1.45 | 84 | `act_mana` + `act_tap` + `draw(n=1)` + `mana_ability(out=1_color)` | Mind Stone (#31), War Room (#150), Bonders' Enclave (#544), … +81 |
| 18 | 1.44 | 139 | `library_bottom` | Dig Through Time (#625), Experimental Augury (#1308), Planar Genesis (#1507), … +136 |
| 19 | 1.43 | 54 | `act_mana` + `equip_keyword` | Swiftfoot Boots (#12), Lightning Greaves (#13), Mithril Coat (#240), … +51 |
| 20 | 1.32 | 156 | `etb_with_counters` | Mossborn Hydra (#884), Kalonian Hydra (#1104), Stonecoil Serpent (#1593), … +153 |
| 21 | 1.25 | 149 | `trig_death` | Scrap Trawler (#843), Myr Retriever (#869), Meren of Clan Nel Toth (#1469), … +146 |
| 22 | 1.22 | 87 | `destroy(target=artifact)` | Vandalblast (#98), Putrefy (#326), Nature's Claim (#363), … +84 |
| 23 | 1.19 | 95 | `token_copy` + `token_other` | Second Harvest (#588), Irenicus's Vile Duplication (#954), Rite of Replication (#972), … +92 |
| 24 | 1.18 | 89 | `free_cast` | Deflecting Swat (#81), Flawless Maneuver (#181), Rishkar's Expertise (#246), … +86 |
| 25 | 1.18 | 62 | `exile(target=creature)` | Swords to Plowshares (#11), Path to Exile (#15), Reality Shift (#270), … +59 |
| 26 | 1.17 | 95 | `act_mana` + `act_tap` + `draw(n=1)` | Sensei's Divining Top (#221), Loran of the Third Path (#359), Fiery Islet (#800), … +92 |
| 27 | 1.11 | 99 | `destroy(target=creature)` | Pongify (#160), Rapid Hybridization (#213), Terminate (#219), … +96 |
| 28 | 1.10 | 100 | `modal_one` | Red Elemental Blast (#421), Sheoldred's Edict (#1186), Heliod's Intervention (#1354), … +97 |
| 29 | 1.07 | 89 | `draw(n=1)` + `trig_etb` | Guardian Project (#297), Tribute to the World Tree (#563), Mentor of the Meek (#604), … +86 |
| 30 | 1.07 | 117 | `trig_phase` + `trig_upkeep` | Land Tax (#414), Scrawling Crawler (#713), Descent into Avernus (#875), … +114 |

## Top 50 singleton clusters by play-weight

These are cards with truly unique effect signatures — each one needs its
own custom handler. Ranked here by EDH inclusion, so the most competitively
impactful unique cards bubble to the top of the custom-handler queue.

| # | Weight | EDHRec rank | Card | Why it's a singleton |
|---:|---:|---:|---|---|
| 1 | 0.209 | 23 | Blasphemous Act | `cost_reduce(n=1)` + `damage(n=13,target=each_creature)` |
| 2 | 0.167 | 36 | Solemn Simulacrum | `draw(n=1)` + `trig_etb` + `tutor_land(to=hand)` |
| 3 | 0.132 | 57 | Reanimate | `lifeloss_self(n=var)` + `reanimate(filter=creature)` |
| 4 | 0.114 | 77 | Boseiju, Who Endures | `act_mana` + `act_tap` + `destroy(target=artifact)` + `mana_ability(out=1_color)` |
| 5 | 0.113 | 78 | Fierce Guardianship | `counter_spell(target=noncreature)` + `free_cast` |
| 6 | 0.104 | 92 | Garruk's Uprising | `draw(n=1)` + `static_creatures` + `trig_etb` |
| 7 | 0.102 | 96 | Phyrexian Arena | `draw(n=1)` + `lifeloss_self(n=1)` + `trig_phase` + `trig_upkeep` |
| 8 | 0.093 | 115 | Deadly Rollick | `exile(target=creature)` + `free_cast` |
| 9 | 0.087 | 132 | Deadly Dispute | `addl_cost_sac` + `sac(type=artifact)` + `token_other` + `treasure` |
| 10 | 0.087 | 133 | Black Market Connections | `draw(n=1)` + `lifeloss_self(n=1)` + `token` + `token_other` + `treasure` + `trig_phase` |
| 11 | 0.083 | 144 | Blood Artist | `lifegain(n=1)` + `lifeloss_target(n=1)` + `trig_death` |
| 12 | 0.080 | 156 | Anguished Unmaking | `exile(target=nonland_perm)` + `lifeloss_self(n=3)` |
| 13 | 0.078 | 165 | Austere Command | `destroy_all(target=creatures)` + `modal_two` |
| 14 | 0.069 | 208 | Phyrexian Tower | `act_mana` + `act_tap` + `mana_ability(out=1_color)` + `sac(type=creature)` |
| 15 | 0.068 | 214 | Crop Rotation | `addl_cost_sac` + `sac(type=other)` + `tutor_land(to=hand)` |
| 16 | 0.066 | 229 | Fell the Profane // Fell Mire | `act_mana` + `act_tap` + `destroy(target=creature)` + `mana_ability(out=1_color)` |
| 17 | 0.065 | 235 | The Great Henge | `act_mana` + `act_tap` + `counter_p1p1(n=1)` + `draw(n=1)` + `lifegain(n=2)` + `mana_ability(out=2_color)` |
| 18 | 0.064 | 247 | Syr Konrad, the Grim | `act_mana` + `damage(n=1,target=each_opp)` + `trig_death` |
| 19 | 0.062 | 256 | Viscera Seer | `sac(type=creature)` + `scry(n=1)` |
| 20 | 0.062 | 260 | Bolas's Citadel | `act_mana` + `act_tap` + `lifeloss_each(n=10)` + `may_peek_top` |
| 21 | 0.060 | 281 | Inventors' Fair | `act_mana` + `act_tap` + `lifegain(n=1)` + `mana_ability(out=1_color)` + `trig_phase` + `trig_upkeep` |
| 22 | 0.059 | 291 | Braids, Arisen Nightmare | `draw(n=1)` + `sac(type=artifact)` + `trig_endstep` + `trig_phase` |
| 23 | 0.055 | 335 | Bastion of Remembrance | `lifegain(n=1)` + `lifeloss_each(n=1)` + `token` + `token_other` + `trig_etb` |
| 24 | 0.054 | 342 | Witch Enchanter // Witch-Blessed Meadow | `act_mana` + `act_tap` + `destroy(target=artifact)` + `mana_ability(out=1_color)` + `trig_etb` |
| 25 | 0.053 | 356 | Pact of Negation | `counter_spell(target=any)` + `self_lose` |
| 26 | 0.052 | 369 | Aetherflux Reservoir | `damage(n=50,target=any)` + `lifegain(n=1)` + `trig_cast_any` + `trig_cast_filtered` |
| 27 | 0.052 | 374 | Toski, Bearer of Secrets | `draw(n=1)` + `must_attack` |
| 28 | 0.050 | 400 | Thassa's Oracle | `library_bottom` + `peek_top(n=x)` + `trig_etb` + `win_game` |
| 29 | 0.050 | 406 | Lotho, Corrupt Shirriff | `lifeloss_self(n=1)` + `token_other` + `treasure` + `trig_player_cast` |
| 30 | 0.050 | 408 | Minas Tirith | `act_mana` + `act_tap` + `draw(n=1)` + `etb_tapped` + `mana_ability(out=1_color)` |
| 31 | 0.049 | 416 | Demand Answers | `addl_cost_sac` + `sac(type=artifact)` + `self_discard(n=1)` |
| 32 | 0.047 | 455 | Goblin Bombardment | `damage(n=1,target=any)` + `sac(type=creature)` |
| 33 | 0.047 | 462 | Fanatic of Rhonas | `act_mana` + `act_tap` + `mana_ability(out=2_color)` + `token_other` |
| 34 | 0.046 | 463 | Selvala, Heart of the Wilds | `act_mana` + `act_tap` + `draw(n=1)` + `mana_var` + `trig_etb` |
| 35 | 0.046 | 464 | Warren Soultrader | `sac(type=creature)` + `token_other` + `treasure` |
| 36 | 0.046 | 469 | Walking Ballista | `act_mana` + `counter_p1p1(n=1)` + `damage(n=1,target=any)` + `etb_with_counters` |
| 37 | 0.046 | 478 | Oracle of Mul Daya | `play_from_top` |
| 38 | 0.045 | 490 | Birgi, God of Storytelling // Harnfel, Horn of Bounty | `self_discard(n=1)` + `trig_cast_any` |
| 39 | 0.045 | 501 | Necropotence | `self_discard(n=1)` + `skip_step` |
| 40 | 0.045 | 503 | Conjurer's Closet | `exile(target=creature)` + `flicker_target` + `trig_endstep` + `trig_phase` |
| 41 | 0.044 | 505 | High Market | `act_mana` + `act_tap` + `lifegain(n=1)` + `mana_ability(out=1_color)` + `sac(type=creature)` |
| 42 | 0.044 | 528 | Disciple of Freyalise // Garden of Freyalise | `act_mana` + `act_tap` + `mana_ability(out=1_color)` + `sac(type=creature)` + `trig_etb` |
| 43 | 0.043 | 545 | Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun | `act_mana` + `act_tap` + `library_bottom` + `mana_ability(out=1_color)` + `mana_for_each` + `trig_etb` |
| 44 | 0.042 | 559 | Beastmaster Ascension | `anthem(p=5,t=5)` + `conditional_static` + `counter_typed(kind=quest)` + `trig_attack` |
| 45 | 0.042 | 577 | Tibalt's Trickery | `counter_spell(target=any)` + `random_choice` |
| 46 | 0.042 | 579 | Laboratory Maniac | `draw(n=1)` + `win_game` |
| 47 | 0.041 | 584 | Deathrite Shaman | `act_mana` + `act_tap` + `exile(target=creature)` + `lifegain(n=2)` + `lifeloss_each(n=2)` |
| 48 | 0.041 | 603 | Carrion Feeder | `combat_restriction` + `counter_p1p1(n=1)` + `sac(type=creature)` |
| 49 | 0.040 | 621 | Puresteel Paladin | `act_mana` + `as_long_as` + `conditional_static` + `draw(n=1)` + `trig_etb` |
| 50 | 0.040 | 622 | Bitterblossom | `lifeloss_self(n=1)` + `token` + `token_other` + `trig_phase` + `trig_upkeep` |

## What this means

- Building the **top 10 play-weighted handlers** covers **46.8%** of all EDH play (vs 47.1% of card count).
- Top 100 weighted handlers cover **67.5%** of EDH play.
- Top 30 singleton handlers (the truly unique custom cards in EDH) cover the most-played one-of-a-kind effects in the format.

Handler build order should follow this report, not the raw count report.
Build a top-10 weighted handler → an EDH cage can simulate ~46.8% of the play patterns it'll actually encounter.