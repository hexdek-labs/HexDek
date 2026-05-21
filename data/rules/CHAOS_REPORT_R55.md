# Chaos Gauntlet Report

Generated: 2026-05-20T20:45:31-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36656 cards |
| Legendary Creatures | 3433 |
| Total Games | 7500 |
| Seed | 48 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 0 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 6m3.597s |
| Throughput | 21 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 346 (in 34 games) |
| Clean Games | 7466 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 2ms |
| Throughput | 0 boards/sec |
| Crashes | 0 |
| Invariant Violations | 0 |
| Clean Boards | 0 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| TriggerCompleteness | 6 |
| ZoneCastGrantExpiry | 20 |
| ZoneConservation | 108 |
| CardIdentity | 182 |
| AttachmentConsistency | 22 |
| CombatLegality | 8 |

### Violation Details (up to 5 per invariant kind, 30 shown)

#### Violation 1

- **Game**: 3431 (seed 34310049, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 7, Phase=ending Step=cleanup
- **Commanders**: Omnath, Locus of All, Gisa, Glorious Resurrector, Ratadrabik of Urborg, Melira, Sylvok Outcast
- **Message**: zone conservation violated: 2 real cards disappeared (expected 394, found 392)

<details>
<summary>Game State</summary>

```
Turn 7, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 561 events
  Seat 0 [alive]: life=40 library=88 hand=6 graveyard=1 exile=0 battlefield=1 cmdzone=1 mana=0
    - Zagoth Triome (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=88 hand=6 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tribute to Horobi // Echo of Death's Wail (P/T 3/3, dmg=0)
    - A-Dueling Coach (P/T 2/2, dmg=0)
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=0 battlefield=3 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mouth of Ronom (P/T 0/0, dmg=0) [T]
    - Changing Loyalty (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=90 hand=4 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Buried Ruin (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[541] priority_pass seat=2 source= target=seat0
[542] priority_pass seat=0 source= target=seat0
[543] priority_pass seat=1 source= target=seat0
[544] priority_pass seat=2 source= target=seat0
[545] priority_pass seat=0 source= target=seat0
[546] priority_pass seat=1 source= target=seat0
[547] priority_pass seat=2 source= target=seat0
[548] priority_pass seat=0 source= target=seat0
[549] priority_pass seat=1 source= target=seat0
[550] priority_pass seat=2 source= target=seat0
[551] priority_pass seat=0 source= target=seat0
[552] priority_pass seat=1 source= target=seat0
[553] priority_pass seat=2 source= target=seat0
[554] priority_pass seat=0 source= target=seat0
[555] priority_pass seat=1 source= target=seat0
[556] priority_pass seat=2 source= target=seat0
[557] loop_shortcut seat=0 source=no_op_loop target=seat0
[558] phase_step seat=3 source= target=seat0
[559] phase_step seat=3 source= target=seat0
[560] state seat=3 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 3431 (seed 34310049, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 7, Phase=ending Step=cleanup
- **Commanders**: Omnath, Locus of All, Gisa, Glorious Resurrector, Ratadrabik of Urborg, Melira, Sylvok Outcast
- **Message**: zone conservation violated: 2 real cards disappeared (expected 394, found 392)

<details>
<summary>Game State</summary>

```
Turn 7, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 561 events
  Seat 0 [alive]: life=40 library=88 hand=6 graveyard=1 exile=0 battlefield=1 cmdzone=1 mana=0
    - Zagoth Triome (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=88 hand=6 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tribute to Horobi // Echo of Death's Wail (P/T 3/3, dmg=0)
    - A-Dueling Coach (P/T 2/2, dmg=0)
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=0 battlefield=3 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mouth of Ronom (P/T 0/0, dmg=0) [T]
    - Changing Loyalty (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=90 hand=4 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Buried Ruin (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[541] priority_pass seat=2 source= target=seat0
[542] priority_pass seat=0 source= target=seat0
[543] priority_pass seat=1 source= target=seat0
[544] priority_pass seat=2 source= target=seat0
[545] priority_pass seat=0 source= target=seat0
[546] priority_pass seat=1 source= target=seat0
[547] priority_pass seat=2 source= target=seat0
[548] priority_pass seat=0 source= target=seat0
[549] priority_pass seat=1 source= target=seat0
[550] priority_pass seat=2 source= target=seat0
[551] priority_pass seat=0 source= target=seat0
[552] priority_pass seat=1 source= target=seat0
[553] priority_pass seat=2 source= target=seat0
[554] priority_pass seat=0 source= target=seat0
[555] priority_pass seat=1 source= target=seat0
[556] priority_pass seat=2 source= target=seat0
[557] loop_shortcut seat=0 source=no_op_loop target=seat0
[558] phase_step seat=3 source= target=seat0
[559] phase_step seat=3 source= target=seat0
[560] state seat=3 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 3431 (seed 34310049, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 8, Phase=ending Step=cleanup
- **Commanders**: Omnath, Locus of All, Gisa, Glorious Resurrector, Ratadrabik of Urborg, Melira, Sylvok Outcast
- **Message**: zone conservation violated: 4 real cards disappeared (expected 394, found 390)

<details>
<summary>Game State</summary>

```
Turn 8, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 632 events
  Seat 0 [alive]: life=40 library=87 hand=4 graveyard=1 exile=0 battlefield=2 cmdzone=1 mana=0
    - Zagoth Triome (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=88 hand=6 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tribute to Horobi // Echo of Death's Wail (P/T 3/3, dmg=0)
    - A-Dueling Coach (P/T 2/2, dmg=0)
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=0 battlefield=3 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mouth of Ronom (P/T 0/0, dmg=0) [T]
    - Changing Loyalty (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=90 hand=4 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Buried Ruin (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[612] priority_pass seat=3 source= target=seat0
[613] priority_pass seat=1 source= target=seat0
[614] priority_pass seat=2 source= target=seat0
[615] priority_pass seat=3 source= target=seat0
[616] priority_pass seat=1 source= target=seat0
[617] priority_pass seat=2 source= target=seat0
[618] priority_pass seat=3 source= target=seat0
[619] priority_pass seat=1 source= target=seat0
[620] priority_pass seat=2 source= target=seat0
[621] priority_pass seat=3 source= target=seat0
[622] priority_pass seat=1 source= target=seat0
[623] priority_pass seat=2 source= target=seat0
[624] priority_pass seat=3 source= target=seat0
[625] priority_pass seat=1 source= target=seat0
[626] priority_pass seat=2 source= target=seat0
[627] priority_pass seat=3 source= target=seat0
[628] loop_shortcut seat=0 source=no_op_loop target=seat0
[629] phase_step seat=0 source= target=seat0
[630] phase_step seat=0 source= target=seat0
[631] state seat=0 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 3431 (seed 34310049, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 8, Phase=ending Step=cleanup
- **Commanders**: Omnath, Locus of All, Gisa, Glorious Resurrector, Ratadrabik of Urborg, Melira, Sylvok Outcast
- **Message**: zone conservation violated: 4 real cards disappeared (expected 394, found 390)

<details>
<summary>Game State</summary>

```
Turn 8, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 632 events
  Seat 0 [alive]: life=40 library=87 hand=4 graveyard=1 exile=0 battlefield=2 cmdzone=1 mana=0
    - Zagoth Triome (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=88 hand=6 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tribute to Horobi // Echo of Death's Wail (P/T 3/3, dmg=0)
    - A-Dueling Coach (P/T 2/2, dmg=0)
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=0 battlefield=3 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mouth of Ronom (P/T 0/0, dmg=0) [T]
    - Changing Loyalty (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=90 hand=4 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Buried Ruin (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[612] priority_pass seat=3 source= target=seat0
[613] priority_pass seat=1 source= target=seat0
[614] priority_pass seat=2 source= target=seat0
[615] priority_pass seat=3 source= target=seat0
[616] priority_pass seat=1 source= target=seat0
[617] priority_pass seat=2 source= target=seat0
[618] priority_pass seat=3 source= target=seat0
[619] priority_pass seat=1 source= target=seat0
[620] priority_pass seat=2 source= target=seat0
[621] priority_pass seat=3 source= target=seat0
[622] priority_pass seat=1 source= target=seat0
[623] priority_pass seat=2 source= target=seat0
[624] priority_pass seat=3 source= target=seat0
[625] priority_pass seat=1 source= target=seat0
[626] priority_pass seat=2 source= target=seat0
[627] priority_pass seat=3 source= target=seat0
[628] loop_shortcut seat=0 source=no_op_loop target=seat0
[629] phase_step seat=0 source= target=seat0
[630] phase_step seat=0 source= target=seat0
[631] state seat=0 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 3431 (seed 34310049, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 9, Phase=ending Step=cleanup
- **Commanders**: Omnath, Locus of All, Gisa, Glorious Resurrector, Ratadrabik of Urborg, Melira, Sylvok Outcast
- **Message**: zone conservation violated: 5 real cards disappeared (expected 394, found 389)

<details>
<summary>Game State</summary>

```
Turn 9, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 706 events
  Seat 0 [alive]: life=40 library=87 hand=4 graveyard=1 exile=0 battlefield=2 cmdzone=1 mana=0
    - Zagoth Triome (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=87 hand=5 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tribute to Horobi // Echo of Death's Wail (P/T 3/3, dmg=0)
    - A-Dueling Coach (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=0 battlefield=3 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mouth of Ronom (P/T 0/0, dmg=0) [T]
    - Changing Loyalty (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=90 hand=4 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Buried Ruin (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[686] priority_pass seat=0 source= target=seat0
[687] priority_pass seat=2 source= target=seat0
[688] priority_pass seat=3 source= target=seat0
[689] priority_pass seat=0 source= target=seat0
[690] priority_pass seat=2 source= target=seat0
[691] priority_pass seat=3 source= target=seat0
[692] priority_pass seat=0 source= target=seat0
[693] priority_pass seat=2 source= target=seat0
[694] priority_pass seat=3 source= target=seat0
[695] priority_pass seat=0 source= target=seat0
[696] priority_pass seat=2 source= target=seat0
[697] priority_pass seat=3 source= target=seat0
[698] priority_pass seat=0 source= target=seat0
[699] priority_pass seat=2 source= target=seat0
[700] priority_pass seat=3 source= target=seat0
[701] priority_pass seat=0 source= target=seat0
[702] loop_shortcut seat=0 source=no_op_loop target=seat0
[703] phase_step seat=1 source= target=seat0
[704] phase_step seat=1 source= target=seat0
[705] state seat=1 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 3458 (seed 34580049, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: God-Eternal Bontu, Marrow-Gnawer, Alora, Cheerful Mastermind, Jeska and Kamahl
- **Message**: CardIdentity: card "God-Eternal Bontu" (ptr 0xc008bca000) appears in both seat 0 library and seat 0 command_zone

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1180 events
  Seat 0 [alive]: life=40 library=83 hand=4 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Riveteers Overlook (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cateran Slaver (P/T 5/5, dmg=0)
  Seat 1 [alive]: life=40 library=82 hand=6 graveyard=0 exile=0 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Zelyon Sword (P/T 0/0, dmg=0) [T]
    - The Tabernacle at Pendrell Vale (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=35 library=80 hand=3 graveyard=1 exile=0 battlefield=11 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wintermoon Mesa (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Liquimetal Coating (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ring of the Lucii (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Waste Land (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Spirit of the Hearth (P/T 4/5, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Meek Attack (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1160] add_mana seat=0 source=Swamp amount=1 target=seat0
[1161] add_mana seat=0 source=Swamp amount=1 target=seat0
[1162] add_mana seat=0 source=Swamp amount=1 target=seat0
[1163] add_mana seat=0 source=Riveteers Overlook amount=1 target=seat0
[1164] add_mana seat=0 source=Swamp amount=1 target=seat0
[1165] draw seat=0 source=Swamp amount=1 target=seat0
[1166] play_land seat=0 source=Swamp target=seat0
[1167] add_mana seat=0 source=Swamp amount=1 target=seat0
[1168] pay_mana seat=0 source=Cateran Slaver amount=5 target=seat0
[1169] cast seat=0 source=Cateran Slaver amount=5 target=seat0
[1170] stack_push seat=0 source=Cateran Slaver target=seat0
[1171] priority_pass seat=1 source= target=seat0
[1172] priority_pass seat=2 source= target=seat0
[1173] priority_pass seat=3 source= target=seat0
[1174] stack_resolve seat=0 source=Cateran Slaver target=seat0
[1175] enter_battlefield seat=0 source=Cateran Slaver target=seat0
[1176] phase_step seat=0 source= target=seat0
[1177] phase_step seat=0 source= target=seat0
[1178] pool_drain seat=0 source= amount=1 target=seat0
[1179] state seat=0 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 3458 (seed 34580049, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: God-Eternal Bontu, Marrow-Gnawer, Alora, Cheerful Mastermind, Jeska and Kamahl
- **Message**: CardIdentity: card "God-Eternal Bontu" (ptr 0xc008bca000) appears in both seat 0 library and seat 0 command_zone

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1180 events
  Seat 0 [alive]: life=40 library=83 hand=4 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Riveteers Overlook (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cateran Slaver (P/T 5/5, dmg=0)
  Seat 1 [alive]: life=40 library=82 hand=6 graveyard=0 exile=0 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Zelyon Sword (P/T 0/0, dmg=0) [T]
    - The Tabernacle at Pendrell Vale (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=35 library=80 hand=3 graveyard=1 exile=0 battlefield=11 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wintermoon Mesa (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Liquimetal Coating (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ring of the Lucii (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Waste Land (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Spirit of the Hearth (P/T 4/5, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Meek Attack (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1160] add_mana seat=0 source=Swamp amount=1 target=seat0
[1161] add_mana seat=0 source=Swamp amount=1 target=seat0
[1162] add_mana seat=0 source=Swamp amount=1 target=seat0
[1163] add_mana seat=0 source=Riveteers Overlook amount=1 target=seat0
[1164] add_mana seat=0 source=Swamp amount=1 target=seat0
[1165] draw seat=0 source=Swamp amount=1 target=seat0
[1166] play_land seat=0 source=Swamp target=seat0
[1167] add_mana seat=0 source=Swamp amount=1 target=seat0
[1168] pay_mana seat=0 source=Cateran Slaver amount=5 target=seat0
[1169] cast seat=0 source=Cateran Slaver amount=5 target=seat0
[1170] stack_push seat=0 source=Cateran Slaver target=seat0
[1171] priority_pass seat=1 source= target=seat0
[1172] priority_pass seat=2 source= target=seat0
[1173] priority_pass seat=3 source= target=seat0
[1174] stack_resolve seat=0 source=Cateran Slaver target=seat0
[1175] enter_battlefield seat=0 source=Cateran Slaver target=seat0
[1176] phase_step seat=0 source= target=seat0
[1177] phase_step seat=0 source= target=seat0
[1178] pool_drain seat=0 source= amount=1 target=seat0
[1179] state seat=0 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 3458 (seed 34580049, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: God-Eternal Bontu, Marrow-Gnawer, Alora, Cheerful Mastermind, Jeska and Kamahl
- **Message**: CardIdentity: card "God-Eternal Bontu" (ptr 0xc008bca000) appears in both seat 0 library and seat 0 command_zone

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1227 events
  Seat 0 [alive]: life=40 library=83 hand=4 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Riveteers Overlook (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cateran Slaver (P/T 5/5, dmg=0)
  Seat 1 [alive]: life=40 library=81 hand=7 graveyard=0 exile=0 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Zelyon Sword (P/T 0/0, dmg=0) [T]
    - The Tabernacle at Pendrell Vale (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=35 library=80 hand=3 graveyard=1 exile=0 battlefield=11 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wintermoon Mesa (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Liquimetal Coating (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ring of the Lucii (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Waste Land (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Spirit of the Hearth (P/T 4/5, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Meek Attack (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1207] add_mana seat=1 source=Swamp amount=1 target=seat0
[1208] add_mana seat=1 source=The Tabernacle at Pendrell Vale amount=1 target=seat0
[1209] add_mana seat=1 source=Swamp amount=1 target=seat0
[1210] add_mana seat=1 source=Terminal Moraine amount=1 target=seat0
[1211] add_mana seat=1 source=Swamp amount=1 target=seat0
[1212] tap seat=1 source=Zelyon Sword target=seat0
[1213] pay_mana seat=1 source=Zelyon Sword amount=3 target=seat0
[1214] activate_ability seat=1 source=Zelyon Sword target=seat0
[1215] stack_push seat=1 source=Zelyon Sword target=seat0
[1216] priority_pass seat=2 source= target=seat0
[1217] priority_pass seat=3 source= target=seat0
[1218] priority_pass seat=0 source= target=seat0
[1219] stack_resolve seat=1 source=Zelyon Sword target=seat0
[1220] parsed_effect_residual seat=1 source=Zelyon Sword target=seat0
[1221] activated_ability_resolved seat=1 source=Zelyon Sword target=seat0
[1222] draw seat=1 source=Zabaz, the Glimmerwasp // Zabaz, the Glimmerwasp amount=1 target=seat0
[1223] phase_step seat=1 source= target=seat0
[1224] phase_step seat=1 source= target=seat0
[1225] pool_drain seat=1 source= amount=4 target=seat0
[1226] state seat=1 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 3458 (seed 34580049, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: God-Eternal Bontu, Marrow-Gnawer, Alora, Cheerful Mastermind, Jeska and Kamahl
- **Message**: CardIdentity: card "God-Eternal Bontu" (ptr 0xc008bca000) appears in both seat 0 library and seat 0 command_zone

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1227 events
  Seat 0 [alive]: life=40 library=83 hand=4 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Riveteers Overlook (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cateran Slaver (P/T 5/5, dmg=0)
  Seat 1 [alive]: life=40 library=81 hand=7 graveyard=0 exile=0 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Zelyon Sword (P/T 0/0, dmg=0) [T]
    - The Tabernacle at Pendrell Vale (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=35 library=80 hand=3 graveyard=1 exile=0 battlefield=11 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wintermoon Mesa (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Liquimetal Coating (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ring of the Lucii (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Waste Land (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Spirit of the Hearth (P/T 4/5, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Meek Attack (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1207] add_mana seat=1 source=Swamp amount=1 target=seat0
[1208] add_mana seat=1 source=The Tabernacle at Pendrell Vale amount=1 target=seat0
[1209] add_mana seat=1 source=Swamp amount=1 target=seat0
[1210] add_mana seat=1 source=Terminal Moraine amount=1 target=seat0
[1211] add_mana seat=1 source=Swamp amount=1 target=seat0
[1212] tap seat=1 source=Zelyon Sword target=seat0
[1213] pay_mana seat=1 source=Zelyon Sword amount=3 target=seat0
[1214] activate_ability seat=1 source=Zelyon Sword target=seat0
[1215] stack_push seat=1 source=Zelyon Sword target=seat0
[1216] priority_pass seat=2 source= target=seat0
[1217] priority_pass seat=3 source= target=seat0
[1218] priority_pass seat=0 source= target=seat0
[1219] stack_resolve seat=1 source=Zelyon Sword target=seat0
[1220] parsed_effect_residual seat=1 source=Zelyon Sword target=seat0
[1221] activated_ability_resolved seat=1 source=Zelyon Sword target=seat0
[1222] draw seat=1 source=Zabaz, the Glimmerwasp // Zabaz, the Glimmerwasp amount=1 target=seat0
[1223] phase_step seat=1 source= target=seat0
[1224] phase_step seat=1 source= target=seat0
[1225] pool_drain seat=1 source= amount=4 target=seat0
[1226] state seat=1 source= target=seat0
```

</details>

#### Violation 10

- **Game**: 3458 (seed 34580049, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 34, Phase=ending Step=cleanup
- **Commanders**: God-Eternal Bontu, Marrow-Gnawer, Alora, Cheerful Mastermind, Jeska and Kamahl
- **Message**: CardIdentity: card "God-Eternal Bontu" (ptr 0xc008bca000) appears in both seat 0 library and seat 0 command_zone

<details>
<summary>Game State</summary>

```
Turn 34, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1308 events
  Seat 0 [alive]: life=40 library=83 hand=4 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Riveteers Overlook (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cateran Slaver (P/T 5/5, dmg=0) [T]
  Seat 1 [alive]: life=40 library=81 hand=7 graveyard=0 exile=0 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Zelyon Sword (P/T 0/0, dmg=0) [T]
    - The Tabernacle at Pendrell Vale (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=34 library=79 hand=2 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wintermoon Mesa (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Liquimetal Coating (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ring of the Lucii (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Waste Land (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Alora, Cheerful Mastermind (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Meek Attack (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1288] pay_mana seat=2 source=Alora, Cheerful Mastermind amount=7 target=seat0
[1289] cast seat=2 source=Alora, Cheerful Mastermind amount=7 target=seat0
[1290] commander_cast_from_command_zone seat=2 source=Alora, Cheerful Mastermind amount=7 target=seat0
[1291] stack_push seat=2 source=Alora, Cheerful Mastermind target=seat0
[1292] cast seat=2 source=Rocketeer Boostbuggy // Rocketeer Boostbuggy target=seat0
[1293] stack_push seat=2 source=Rocketeer Boostbuggy // Rocketeer Boostbuggy target=seat0
[1294] priority_pass seat=3 source= target=seat0
[1295] priority_pass seat=0 source= target=seat0
[1296] priority_pass seat=1 source= target=seat0
[1297] stack_resolve seat=2 source=Rocketeer Boostbuggy // Rocketeer Boostbuggy target=seat0
[1298] zone_change seat=2 source=Rocketeer Boostbuggy // Rocketeer Boostbuggy
[1299] resolve seat=2 source=Rocketeer Boostbuggy // Rocketeer Boostbuggy target=seat0
[1300] priority_pass seat=3 source= target=seat0
[1301] priority_pass seat=0 source= target=seat0
[1302] priority_pass seat=1 source= target=seat0
[1303] stack_resolve seat=2 source=Alora, Cheerful Mastermind target=seat0
[1304] enter_battlefield seat=2 source=Alora, Cheerful Mastermind target=seat0
[1305] phase_step seat=2 source= target=seat0
[1306] phase_step seat=2 source= target=seat0
[1307] state seat=2 source= target=seat0
```

</details>

#### Violation 11

- **Game**: 435 (seed 4350049, perm 0)
- **Invariant**: AttachmentConsistency
- **Turn**: 39, Phase=combat Step=end_of_combat
- **Commanders**: Prime Speaker Zegana, Tergrid, God of Fright // Tergrid's Lantern, Aatchik, Emerald Radian, Wilson, Majestic Bear
- **Message**: AttachmentConsistency: "Ghoulish Impetus" (seat 1) is attached to "creature token black zombie giant Token" which is not on any battlefield

<details>
<summary>Game State</summary>

```
Turn 39, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 1726 events
  Seat 0 [LOST]: life=-252 library=81 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=32 library=80 hand=4 graveyard=9 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Quest for the Gravelord (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Ghoulish Impetus (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tergrid, God of Fright // Tergrid's Lantern (P/T 4/5, dmg=0)
  Seat 2 [LOST]: life=-31 library=78 hand=5 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-143 library=82 hand=7 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1706] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1707] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1708] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1709] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1710] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1711] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1712] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1713] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1714] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1715] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1716] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1717] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1718] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1719] sba_704_5a seat=0 source= amount=-252
[1720] destroy seat=0 source=Incubation Druid
[1721] sba_704_5g seat=0 source=Incubation Druid
[1722] zone_change seat=0 source=Incubation Druid
[1723] sba_cycle_complete seat=-1 source=
[1724] seat_eliminated seat=0 source= amount=85
[1725] game_end seat=1 source=
```

</details>

#### Violation 12

- **Game**: 435 (seed 4350049, perm 0)
- **Invariant**: AttachmentConsistency
- **Turn**: 39, Phase=combat Step=end_of_combat
- **Commanders**: Prime Speaker Zegana, Tergrid, God of Fright // Tergrid's Lantern, Aatchik, Emerald Radian, Wilson, Majestic Bear
- **Message**: AttachmentConsistency: "Ghoulish Impetus" (seat 1) is attached to "creature token black zombie giant Token" which is not on any battlefield

<details>
<summary>Game State</summary>

```
Turn 39, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 1726 events
  Seat 0 [LOST]: life=-252 library=81 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=32 library=80 hand=4 graveyard=9 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Quest for the Gravelord (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Ghoulish Impetus (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tergrid, God of Fright // Tergrid's Lantern (P/T 4/5, dmg=0)
  Seat 2 [LOST]: life=-31 library=78 hand=5 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-143 library=82 hand=7 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1706] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1707] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1708] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1709] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1710] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1711] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1712] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1713] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1714] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1715] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1716] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1717] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1718] damage seat=1 source=creature token black zombie giant Token amount=5 target=seat0
[1719] sba_704_5a seat=0 source= amount=-252
[1720] destroy seat=0 source=Incubation Druid
[1721] sba_704_5g seat=0 source=Incubation Druid
[1722] zone_change seat=0 source=Incubation Druid
[1723] sba_cycle_complete seat=-1 source=
[1724] seat_eliminated seat=0 source= amount=85
[1725] game_end seat=1 source=
```

</details>

#### Violation 13

- **Game**: 1135 (seed 11350049, perm 0)
- **Invariant**: AttachmentConsistency
- **Turn**: 57, Phase=combat Step=end_of_combat
- **Commanders**: Raph & Leo, Sibling Rivals, Y'shtola Rhul, Zetalpa, Primal Dawn, Zaxara, the Exemplary
- **Message**: AttachmentConsistency: "Brilliant Wings" (seat 0) is attached to "Tidal Warrior" which is not on any battlefield

<details>
<summary>Game State</summary>

```
Turn 57, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 3031 events
  Seat 0 [WON]: life=9 library=66 hand=5 graveyard=12 exile=0 battlefield=16 cmdzone=0 mana=10
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Abandoned Air Temple (P/T 0/0, dmg=0) [T]
    - Brilliant Wings (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Adventuring Gear (P/T 0/0, dmg=0)
    - Ghost Ark (P/T 3/3, dmg=0)
    - Great Furnace (P/T 0/0, dmg=0) [T]
    - Catalyst Elemental (P/T 2/2, dmg=0) [T]
    - Command Beacon (P/T 0/0, dmg=0) [T]
    - Raph & Leo, Sibling Rivals (P/T 2/4, dmg=0) [T]
    - Golden-Tail Disciple (P/T 2/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-3 library=66 hand=7 graveyard=20 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-1 library=0 hand=4 graveyard=19 exile=62 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-1 library=73 hand=7 graveyard=11 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3011] add_mana seat=0 source=Terramorphic Expanse amount=1 target=seat0
[3012] add_mana seat=0 source=Mountain amount=1 target=seat0
[3013] add_mana seat=0 source=Abandoned Air Temple amount=1 target=seat0
[3014] add_mana seat=0 source=Mountain amount=1 target=seat0
[3015] add_mana seat=0 source=Great Furnace amount=1 target=seat0
[3016] add_mana seat=0 source=Command Beacon amount=1 target=seat0
[3017] add_mana seat=0 source=Plains amount=1 target=seat0
[3018] draw seat=0 source=Plains amount=1 target=seat0
[3019] play_land seat=0 source=Plains target=seat0
[3020] add_mana seat=0 source=Plains amount=1 target=seat0
[3021] phase_step seat=0 source= target=seat0
[3022] declare_attackers seat=0 source= target=seat0
[3023] blockers seat=1 source= target=seat0
[3024] damage seat=0 source=Catalyst Elemental amount=2 target=seat1
[3025] damage seat=0 source=Raph & Leo, Sibling Rivals amount=2 target=seat1
[3026] damage seat=0 source=Golden-Tail Disciple amount=2 target=seat1
[3027] sba_704_5a seat=1 source= amount=-3
[3028] sba_cycle_complete seat=-1 source=
[3029] seat_eliminated seat=1 source= amount=6
[3030] game_end seat=0 source=
```

</details>

#### Violation 14

- **Game**: 1135 (seed 11350049, perm 0)
- **Invariant**: AttachmentConsistency
- **Turn**: 57, Phase=combat Step=end_of_combat
- **Commanders**: Raph & Leo, Sibling Rivals, Y'shtola Rhul, Zetalpa, Primal Dawn, Zaxara, the Exemplary
- **Message**: AttachmentConsistency: "Brilliant Wings" (seat 0) is attached to "Tidal Warrior" which is not on any battlefield

<details>
<summary>Game State</summary>

```
Turn 57, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 3031 events
  Seat 0 [WON]: life=9 library=66 hand=5 graveyard=12 exile=0 battlefield=16 cmdzone=0 mana=10
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Abandoned Air Temple (P/T 0/0, dmg=0) [T]
    - Brilliant Wings (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Adventuring Gear (P/T 0/0, dmg=0)
    - Ghost Ark (P/T 3/3, dmg=0)
    - Great Furnace (P/T 0/0, dmg=0) [T]
    - Catalyst Elemental (P/T 2/2, dmg=0) [T]
    - Command Beacon (P/T 0/0, dmg=0) [T]
    - Raph & Leo, Sibling Rivals (P/T 2/4, dmg=0) [T]
    - Golden-Tail Disciple (P/T 2/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-3 library=66 hand=7 graveyard=20 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-1 library=0 hand=4 graveyard=19 exile=62 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-1 library=73 hand=7 graveyard=11 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3011] add_mana seat=0 source=Terramorphic Expanse amount=1 target=seat0
[3012] add_mana seat=0 source=Mountain amount=1 target=seat0
[3013] add_mana seat=0 source=Abandoned Air Temple amount=1 target=seat0
[3014] add_mana seat=0 source=Mountain amount=1 target=seat0
[3015] add_mana seat=0 source=Great Furnace amount=1 target=seat0
[3016] add_mana seat=0 source=Command Beacon amount=1 target=seat0
[3017] add_mana seat=0 source=Plains amount=1 target=seat0
[3018] draw seat=0 source=Plains amount=1 target=seat0
[3019] play_land seat=0 source=Plains target=seat0
[3020] add_mana seat=0 source=Plains amount=1 target=seat0
[3021] phase_step seat=0 source= target=seat0
[3022] declare_attackers seat=0 source= target=seat0
[3023] blockers seat=1 source= target=seat0
[3024] damage seat=0 source=Catalyst Elemental amount=2 target=seat1
[3025] damage seat=0 source=Raph & Leo, Sibling Rivals amount=2 target=seat1
[3026] damage seat=0 source=Golden-Tail Disciple amount=2 target=seat1
[3027] sba_704_5a seat=1 source= amount=-3
[3028] sba_cycle_complete seat=-1 source=
[3029] seat_eliminated seat=1 source= amount=6
[3030] game_end seat=0 source=
```

</details>

#### Violation 15

- **Game**: 2983 (seed 29830049, perm 0)
- **Invariant**: AttachmentConsistency
- **Turn**: 36, Phase=combat Step=end_of_combat
- **Commanders**: Livonya Silone, Sol'Kanar the Tainted, Mirri, Weatherlight Duelist, Splinter & Leo, Father & Son
- **Message**: AttachmentConsistency: "Dub" (seat 2) is attached to "creature token phyrexian mite Token" which is not on any battlefield

<details>
<summary>Game State</summary>

```
Turn 36, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 2159 events
  Seat 0 [LOST]: life=-8 library=81 hand=3 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=7 library=82 hand=6 graveyard=1 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=9 library=80 hand=4 graveyard=4 exile=0 battlefield=10 cmdzone=1 mana=1
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Skrelv's Hive (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Wax-Wane Witness (P/T 2/4, dmg=0)
    - Cave Tiger (P/T 2/2, dmg=0) [T]
    - Dub (P/T 0/0, dmg=0)
    - Runadi, Behemoth Caller (P/T 1/3, dmg=0) [T]
    - Buster Sword (P/T 0/0, dmg=0)
  Seat 3 [LOST]: life=-53 library=83 hand=7 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2139] pay_mana seat=2 source=Buster Sword amount=3 target=seat0
[2140] cast seat=2 source=Buster Sword amount=3 target=seat0
[2141] stack_push seat=2 source=Buster Sword target=seat0
[2142] priority_pass seat=0 source= target=seat0
[2143] stack_resolve seat=2 source=Buster Sword target=seat0
[2144] enter_battlefield seat=2 source=Buster Sword target=seat0
[2145] phase_step seat=2 source= target=seat0
[2146] declare_attackers seat=2 source= target=seat0
[2147] blockers seat=0 source= target=seat0
[2148] damage seat=2 source=creature token phyrexian mite Token amount=1 target=seat0
[2149] damage seat=2 source=creature token phyrexian mite Token amount=1 target=seat0
[2150] damage seat=2 source=creature token phyrexian mite Token amount=1 target=seat0
[2151] damage seat=2 source=Wax-Wane Witness amount=2 target=seat0
[2152] damage seat=2 source=Cave Tiger amount=2 target=seat0
[2153] damage seat=2 source=creature token phyrexian mite Token amount=1 target=seat0
[2154] damage seat=2 source=Runadi, Behemoth Caller amount=1 target=seat0
[2155] sba_704_5a seat=0 source= amount=-8
[2156] sba_cycle_complete seat=-1 source=
[2157] seat_eliminated seat=0 source= amount=15
[2158] game_end seat=2 source=
```

</details>

#### Violation 16

- **Game**: 1066 (seed 10660049, perm 0)
- **Invariant**: CombatLegality
- **Turn**: 56, Phase=combat Step=end_of_combat
- **Commanders**: Sméagol, Helpful Guide, Satya, Aetherflux Genius, Jenny, Generated Anomaly, High Marshal Arguel
- **Message**: CombatLegality: "Risen Riptide" (seat 1) is attacking with summoning sickness and no haste

<details>
<summary>Game State</summary>

```
Turn 56, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 4826 events
  Seat 0 [LOST]: life=-15 library=77 hand=7 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=7 library=77 hand=2 graveyard=9 exile=0 battlefield=21 cmdzone=0 mana=9
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Scabland (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Satya, Aetherflux Genius (P/T 3/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Sharlayan, Nation of Scholars (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Risen Riptide (P/T 0/5, dmg=0)
    - Risen Riptide (P/T 0/5, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Aether Tunnel (P/T 0/0, dmg=0)
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Risen Riptide (P/T 0/5, dmg=0) [T]
  Seat 2 [LOST]: life=-18 library=79 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-10 library=57 hand=6 graveyard=18 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[4806] priority_pass seat=0 source= target=seat0
[4807] stack_resolve seat=1 source=Satya, Aetherflux Genius target=seat0
[4808] trigger_evaluated seat=1 source=Satya, Aetherflux Genius
[4809] stack_push seat=1 source=Satya, Aetherflux Genius target=seat0
[4810] triggered_ability seat=1 source=Satya, Aetherflux Genius target=seat0
[4811] priority_pass seat=0 source= target=seat0
[4812] stack_resolve seat=1 source=Satya, Aetherflux Genius target=seat0
[4813] blockers seat=0 source= target=seat0
[4814] damage seat=1 source=Satya, Aetherflux Genius amount=3 target=seat0
[4815] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4816] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4817] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4818] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4819] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4820] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4821] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4822] sba_704_5a seat=0 source= amount=-15
[4823] sba_cycle_complete seat=-1 source=
[4824] seat_eliminated seat=0 source= amount=11
[4825] game_end seat=1 source=
```

</details>

#### Violation 17

- **Game**: 1066 (seed 10660049, perm 0)
- **Invariant**: CombatLegality
- **Turn**: 56, Phase=combat Step=end_of_combat
- **Commanders**: Sméagol, Helpful Guide, Satya, Aetherflux Genius, Jenny, Generated Anomaly, High Marshal Arguel
- **Message**: CombatLegality: "Risen Riptide" (seat 1) is attacking with summoning sickness and no haste

<details>
<summary>Game State</summary>

```
Turn 56, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 4826 events
  Seat 0 [LOST]: life=-15 library=77 hand=7 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=7 library=77 hand=2 graveyard=9 exile=0 battlefield=21 cmdzone=0 mana=9
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Scabland (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Satya, Aetherflux Genius (P/T 3/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Sharlayan, Nation of Scholars (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Risen Riptide (P/T 0/5, dmg=0)
    - Risen Riptide (P/T 0/5, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Aether Tunnel (P/T 0/0, dmg=0)
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Risen Riptide (P/T 0/5, dmg=0) [T]
  Seat 2 [LOST]: life=-18 library=79 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-10 library=57 hand=6 graveyard=18 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[4806] priority_pass seat=0 source= target=seat0
[4807] stack_resolve seat=1 source=Satya, Aetherflux Genius target=seat0
[4808] trigger_evaluated seat=1 source=Satya, Aetherflux Genius
[4809] stack_push seat=1 source=Satya, Aetherflux Genius target=seat0
[4810] triggered_ability seat=1 source=Satya, Aetherflux Genius target=seat0
[4811] priority_pass seat=0 source= target=seat0
[4812] stack_resolve seat=1 source=Satya, Aetherflux Genius target=seat0
[4813] blockers seat=0 source= target=seat0
[4814] damage seat=1 source=Satya, Aetherflux Genius amount=3 target=seat0
[4815] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4816] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4817] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4818] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4819] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4820] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4821] damage seat=1 source=Blade of the Sixth Pride amount=3 target=seat0
[4822] sba_704_5a seat=0 source= amount=-15
[4823] sba_cycle_complete seat=-1 source=
[4824] seat_eliminated seat=0 source= amount=11
[4825] game_end seat=1 source=
```

</details>

#### Violation 18

- **Game**: 1244 (seed 12440049, perm 0)
- **Invariant**: CombatLegality
- **Turn**: 48, Phase=combat Step=end_of_combat
- **Commanders**: Purraj of Urborg, Rose Tyler, The Rhystic Storyteller, Satya, Aetherflux Genius
- **Message**: CombatLegality: "Dragon-Style Twins" (seat 3) is attacking with summoning sickness and no haste

<details>
<summary>Game State</summary>

```
Turn 48, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 2656 events
  Seat 0 [LOST]: life=-66 library=78 hand=3 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-159 library=77 hand=2 graveyard=9 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=10 library=80 hand=1 graveyard=3 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=27 library=77 hand=4 graveyard=4 exile=0 battlefield=20 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Autonomous Furnace (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Torch Gauntlet (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Satya, Aetherflux Genius (P/T 3/5, dmg=0) [T]
    - Phyrexian Splicer (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Wall of Light (P/T 1/5, dmg=0)
    - Wall of Light (P/T 1/5, dmg=0)
    - Wall of Light (P/T 1/5, dmg=0)
    - Changeling Wayfinder (P/T 1/2, dmg=0) [T]
    - Changeling Wayfinder (P/T 1/2, dmg=0) [T]
    - Training Center (P/T 0/0, dmg=0) [T]
    - Sabotage Strategist (P/T 167/167, dmg=0)
    - Warbringer (P/T 3/3, dmg=0) [T]
    - Warbringer (P/T 3/3, dmg=0) [T]
    - Wasteland (P/T 0/0, dmg=0) [T]
    - Dragon-Style Twins (P/T 3/3, dmg=0)
    - Dragon-Style Twins (P/T 3/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2636] stack_push seat=3 source=Satya, Aetherflux Genius target=seat0
[2637] triggered_ability seat=3 source=Satya, Aetherflux Genius target=seat0
[2638] priority_pass seat=1 source= target=seat0
[2639] stack_resolve seat=3 source=Satya, Aetherflux Genius target=seat0
[2640] trigger_evaluated seat=3 source=Satya, Aetherflux Genius
[2641] stack_push seat=3 source=Satya, Aetherflux Genius target=seat0
[2642] triggered_ability seat=3 source=Satya, Aetherflux Genius target=seat0
[2643] priority_pass seat=1 source= target=seat0
[2644] stack_resolve seat=3 source=Satya, Aetherflux Genius target=seat0
[2645] blockers seat=1 source= target=seat0
[2646] damage seat=3 source=Satya, Aetherflux Genius amount=3 target=seat1
[2647] damage seat=3 source=Changeling Wayfinder amount=1 target=seat1
[2648] damage seat=3 source=Changeling Wayfinder amount=1 target=seat1
[2649] damage seat=3 source=Sabotage Strategist amount=167 target=seat1
[2650] damage seat=3 source=Warbringer amount=3 target=seat1
[2651] damage seat=3 source=Warbringer amount=3 target=seat1
[2652] sba_704_5a seat=1 source= amount=-159
[2653] sba_cycle_complete seat=-1 source=
[2654] seat_eliminated seat=1 source= amount=9
[2655] game_end seat=3 source=
```

</details>

#### Violation 19

- **Game**: 1244 (seed 12440049, perm 0)
- **Invariant**: CombatLegality
- **Turn**: 48, Phase=combat Step=end_of_combat
- **Commanders**: Purraj of Urborg, Rose Tyler, The Rhystic Storyteller, Satya, Aetherflux Genius
- **Message**: CombatLegality: "Dragon-Style Twins" (seat 3) is attacking with summoning sickness and no haste

<details>
<summary>Game State</summary>

```
Turn 48, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 2656 events
  Seat 0 [LOST]: life=-66 library=78 hand=3 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-159 library=77 hand=2 graveyard=9 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=10 library=80 hand=1 graveyard=3 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=27 library=77 hand=4 graveyard=4 exile=0 battlefield=20 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Autonomous Furnace (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Torch Gauntlet (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Satya, Aetherflux Genius (P/T 3/5, dmg=0) [T]
    - Phyrexian Splicer (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Wall of Light (P/T 1/5, dmg=0)
    - Wall of Light (P/T 1/5, dmg=0)
    - Wall of Light (P/T 1/5, dmg=0)
    - Changeling Wayfinder (P/T 1/2, dmg=0) [T]
    - Changeling Wayfinder (P/T 1/2, dmg=0) [T]
    - Training Center (P/T 0/0, dmg=0) [T]
    - Sabotage Strategist (P/T 167/167, dmg=0)
    - Warbringer (P/T 3/3, dmg=0) [T]
    - Warbringer (P/T 3/3, dmg=0) [T]
    - Wasteland (P/T 0/0, dmg=0) [T]
    - Dragon-Style Twins (P/T 3/3, dmg=0)
    - Dragon-Style Twins (P/T 3/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2636] stack_push seat=3 source=Satya, Aetherflux Genius target=seat0
[2637] triggered_ability seat=3 source=Satya, Aetherflux Genius target=seat0
[2638] priority_pass seat=1 source= target=seat0
[2639] stack_resolve seat=3 source=Satya, Aetherflux Genius target=seat0
[2640] trigger_evaluated seat=3 source=Satya, Aetherflux Genius
[2641] stack_push seat=3 source=Satya, Aetherflux Genius target=seat0
[2642] triggered_ability seat=3 source=Satya, Aetherflux Genius target=seat0
[2643] priority_pass seat=1 source= target=seat0
[2644] stack_resolve seat=3 source=Satya, Aetherflux Genius target=seat0
[2645] blockers seat=1 source= target=seat0
[2646] damage seat=3 source=Satya, Aetherflux Genius amount=3 target=seat1
[2647] damage seat=3 source=Changeling Wayfinder amount=1 target=seat1
[2648] damage seat=3 source=Changeling Wayfinder amount=1 target=seat1
[2649] damage seat=3 source=Sabotage Strategist amount=167 target=seat1
[2650] damage seat=3 source=Warbringer amount=3 target=seat1
[2651] damage seat=3 source=Warbringer amount=3 target=seat1
[2652] sba_704_5a seat=1 source= amount=-159
[2653] sba_cycle_complete seat=-1 source=
[2654] seat_eliminated seat=1 source= amount=9
[2655] game_end seat=3 source=
```

</details>

#### Violation 20

- **Game**: 2598 (seed 25980049, perm 0)
- **Invariant**: CombatLegality
- **Turn**: 53, Phase=combat Step=end_of_combat
- **Commanders**: Nashi, Illusion Gadgeteer, Orysa, Tide Choreographer, Miirym, Sentinel Wyrm, The Swarmlord
- **Message**: CombatLegality: "Must Be Knights" (seat 2) is attacking with summoning sickness and no haste

<details>
<summary>Game State</summary>

```
Turn 53, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 2932 events
  Seat 0 [LOST]: life=-4 library=80 hand=5 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-7 library=74 hand=3 graveyard=11 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=6 library=72 hand=0 graveyard=2 exile=1 battlefield=23 cmdzone=0 mana=9
    - Island (P/T 0/0, dmg=0) [T]
    - Thornspire Verge (P/T 0/0, dmg=0) [T]
    - Rohirrim Lancer (P/T 1/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - An Unearthly Child (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - On the Trail (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Miirym, Sentinel Wyrm (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Zur's Weirding (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Aetherwind Basker (P/T 15/15, dmg=2) [T]
    - Raph & Mikey, Troublemakers (P/T 7/7, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Roar of Resistance (P/T 0/0, dmg=0)
    - Scuttlemutt (P/T 2/2, dmg=0) [T]
    - Uphill Battle (P/T 0/0, dmg=0)
    - Wall of Opposition (P/T 0/6, dmg=0)
    - Skyshroud Ranger (P/T 1/1, dmg=0)
    - Must Be Knights (P/T 2/2, dmg=0) [T]
  Seat 3 [LOST]: life=-23 library=77 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2912] triggered_ability seat=2 source=Miirym, Sentinel Wyrm target=seat0
[2913] priority_pass seat=3 source= target=seat0
[2914] stack_resolve seat=2 source=Miirym, Sentinel Wyrm target=seat0
[2915] per_card_handler seat=0 source=Raph & Mikey, Troublemakers target=seat0
[2916] per_card_partial seat=0 source=Raph & Mikey, Troublemakers target=seat0
[2917] parser_gap seat=0 source=Raph & Mikey, Troublemakers target=seat0
[2918] blockers seat=3 source= target=seat0
[2919] damage seat=2 source=Rohirrim Lancer amount=1 target=seat3
[2920] damage seat=2 source=Miirym, Sentinel Wyrm amount=6 target=seat3
[2921] damage seat=2 source=Aetherwind Basker amount=1 target=seat3
[2922] damage seat=2 source=Aetherwind Basker amount=14 target=seat3
[2923] damage seat=2 source=Raph & Mikey, Troublemakers amount=7 target=seat3
[2924] damage seat=3 source=Azure Mage amount=2 target=seat2
[2925] sba_704_5a seat=3 source= amount=-23
[2926] destroy seat=3 source=Azure Mage
[2927] sba_704_5g seat=3 source=Azure Mage
[2928] zone_change seat=3 source=Azure Mage
[2929] sba_cycle_complete seat=-1 source=
[2930] seat_eliminated seat=3 source= amount=12
[2931] game_end seat=2 source=
```

</details>

#### Violation 21

- **Game**: 1566 (seed 15660049, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 46, Phase=ending Step=cleanup
- **Commanders**: Katara, Waterbending Master, Gandalf of the Secret Fire, Blitzwing, Cruel Tormentor // Blitzwing, Adaptive Assailant, Rev, Tithe Extractor
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 1929 with trigger-bearer(s) [{Gisa, Glorious Resurrector 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 46, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1938 events
  Seat 0 [alive]: life=27 library=80 hand=0 graveyard=10 exile=0 battlefield=8 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Furtive Homunculus (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Imaginary Pet (P/T 3/3, dmg=0)
    - Wall of Tears (P/T 0/4, dmg=0)
  Seat 1 [alive]: life=31 library=79 hand=4 graveyard=5 exile=0 battlefield=9 cmdzone=1 mana=0
    - Prismari Campus (P/T 0/0, dmg=0) [T]
    - Seachrome Coast (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Sarevok's Tome (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Honor of the Pure (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Junk Diver (P/T 1/1, dmg=0) [T]
  Seat 2 [alive]: life=29 library=79 hand=2 graveyard=10 exile=0 battlefield=8 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Planetarium of Wan Shi Tong (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Spitting Dilophosaurus (P/T 3/2, dmg=0) [T]
    - Leyline of the Void (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=21 library=76 hand=4 graveyard=4 exile=2 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mogg Cannon (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Underworld Charger (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Dodecapod (P/T 3/3, dmg=0) [T]
    - Gisa, Glorious Resurrector (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1918] trigger_fires seat=3 source=Rev, Tithe Extractor amount=2 target=seat0
[1919] stack_push seat=3 source=Rev, Tithe Extractor target=seat0
[1920] priority_pass seat=0 source= target=seat0
[1921] priority_pass seat=1 source= target=seat0
[1922] priority_pass seat=2 source= target=seat0
[1923] stack_resolve seat=3 source=Rev, Tithe Extractor target=seat0
[1924] parsed_effect_residual seat=3 source=Rev, Tithe Extractor target=seat0
[1925] damage seat=3 source=Dodecapod amount=3 target=seat0
[1926] damage seat=0 source=Imaginary Pet amount=3 target=seat3
[1927] replacement_applied seat=2 source=Leyline of the Void target=seat0
[1928] destroy seat=3 source=Rev, Tithe Extractor
[1929] sba_704_5g seat=3 source=Rev, Tithe Extractor
[1930] zone_change seat=3 source=Rev, Tithe Extractor
[1931] sba_704_6d seat=3 source=Rev, Tithe Extractor
[1932] sba_cycle_complete seat=-1 source=
[1933] phase_step seat=3 source= target=seat0
[1934] pool_drain seat=3 source= amount=1 target=seat0
[1935] damage_wears_off seat=0 source=Imaginary Pet amount=2 target=seat0
[1936] damage_wears_off seat=0 source=Wall of Tears amount=3 target=seat0
[1937] state seat=3 source= target=seat0
```

</details>

#### Violation 22

- **Game**: 1566 (seed 15660049, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 46, Phase=ending Step=cleanup
- **Commanders**: Katara, Waterbending Master, Gandalf of the Secret Fire, Blitzwing, Cruel Tormentor // Blitzwing, Adaptive Assailant, Rev, Tithe Extractor
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 1929 with trigger-bearer(s) [{Gisa, Glorious Resurrector 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 46, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1938 events
  Seat 0 [alive]: life=27 library=80 hand=0 graveyard=10 exile=0 battlefield=8 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Furtive Homunculus (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Imaginary Pet (P/T 3/3, dmg=0)
    - Wall of Tears (P/T 0/4, dmg=0)
  Seat 1 [alive]: life=31 library=79 hand=4 graveyard=5 exile=0 battlefield=9 cmdzone=1 mana=0
    - Prismari Campus (P/T 0/0, dmg=0) [T]
    - Seachrome Coast (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Sarevok's Tome (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Honor of the Pure (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Junk Diver (P/T 1/1, dmg=0) [T]
  Seat 2 [alive]: life=29 library=79 hand=2 graveyard=10 exile=0 battlefield=8 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Planetarium of Wan Shi Tong (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Spitting Dilophosaurus (P/T 3/2, dmg=0) [T]
    - Leyline of the Void (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=21 library=76 hand=4 graveyard=4 exile=2 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mogg Cannon (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Underworld Charger (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Dodecapod (P/T 3/3, dmg=0) [T]
    - Gisa, Glorious Resurrector (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1918] trigger_fires seat=3 source=Rev, Tithe Extractor amount=2 target=seat0
[1919] stack_push seat=3 source=Rev, Tithe Extractor target=seat0
[1920] priority_pass seat=0 source= target=seat0
[1921] priority_pass seat=1 source= target=seat0
[1922] priority_pass seat=2 source= target=seat0
[1923] stack_resolve seat=3 source=Rev, Tithe Extractor target=seat0
[1924] parsed_effect_residual seat=3 source=Rev, Tithe Extractor target=seat0
[1925] damage seat=3 source=Dodecapod amount=3 target=seat0
[1926] damage seat=0 source=Imaginary Pet amount=3 target=seat3
[1927] replacement_applied seat=2 source=Leyline of the Void target=seat0
[1928] destroy seat=3 source=Rev, Tithe Extractor
[1929] sba_704_5g seat=3 source=Rev, Tithe Extractor
[1930] zone_change seat=3 source=Rev, Tithe Extractor
[1931] sba_704_6d seat=3 source=Rev, Tithe Extractor
[1932] sba_cycle_complete seat=-1 source=
[1933] phase_step seat=3 source= target=seat0
[1934] pool_drain seat=3 source= amount=1 target=seat0
[1935] damage_wears_off seat=0 source=Imaginary Pet amount=2 target=seat0
[1936] damage_wears_off seat=0 source=Wall of Tears amount=3 target=seat0
[1937] state seat=3 source= target=seat0
```

</details>

#### Violation 23

- **Game**: 4474 (seed 44740049, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 59, Phase=ending Step=cleanup
- **Commanders**: Raul, Trouble Shooter, Momo, Playful Pet, Prowler, Clawed Thief, Jenova, Ancient Calamity
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 4263 with trigger-bearer(s) [{Jenova, Ancient Calamity 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 59, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 4272 events
  Seat 0 [alive]: life=18 library=66 hand=2 graveyard=14 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Raul, Trouble Shooter (P/T 0/3, dmg=0) [T]
    - Fire Nation Occupation (P/T 0/0, dmg=0)
    - Plaguecrafter's Familiar (P/T 1/1, dmg=0) [T]
    - Dulcet Sirens (P/T 1/3, dmg=0) [T]
    - Ichor Wellspring (P/T 0/0, dmg=0)
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Hissing Miasma (P/T 0/0, dmg=0)
    - Wall of Mist (P/T 0/5, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Somber Hoverguard (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Tombstone Stairwell (P/T 0/0, dmg=0)
    - creature token black zombie Token (P/T 2/2, dmg=0)
    - Jousting Lance (P/T 0/0, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=25 library=64 hand=5 graveyard=19 exile=0 battlefield=20 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - White Lotus Hideout (P/T 0/0, dmg=0) [T]
    - Contagion Engine (P/T 0/0, dmg=0) [T]
    - food artifact token Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Throne of Empires (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - creature token white soldier Token (P/T 1/1, dmg=0)
    - Laser Screwdriver (P/T 0/0, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
  Seat 2 [alive]: life=3 library=65 hand=4 graveyard=16 exile=0 battlefield=23 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Morphic Pool (P/T 0/0, dmg=0) [T]
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Prowler, Clawed Thief (P/T 2/3, dmg=0) [T]
    - Shelldock Isle (P/T 0/0, dmg=0) [T]
    - Clockwork Fox (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mer-Ek Nightblade (P/T 2/3, dmg=0) [T]
    - Conduit Pylons (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Short Sword (P/T 0/0, dmg=0)
    - Balthor the Defiled (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Desert of the Mindful (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=24 library=65 hand=4 graveyard=15 exile=0 battlefield=17 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Jenova, Ancient Calamity (P/T 0/4, dmg=0)
    - The Mycosynth Gardens (P/T 0/0, dmg=0) [T]
    - Blinkmoth Nexus (P/T 0/0, dmg=0) [T]
    - Timber Protector (P/T 4/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hidden Nursery (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Carefree Swinemaster (P/T 1/4, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0) [T]
    - Restless Cottage (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4252] destroy seat=2 source=Tombspawn
[4253] sba_704_5g seat=2 source=Tombspawn
[4254] destroy seat=2 source=Tombspawn
[4255] sba_704_5g seat=2 source=Tombspawn
[4256] destroy seat=3 source=Tombspawn
[4257] sba_704_5g seat=3 source=Tombspawn
[4258] destroy seat=3 source=Tombspawn
[4259] sba_704_5g seat=3 source=Tombspawn
[4260] destroy seat=3 source=Tombspawn
[4261] sba_704_5g seat=3 source=Tombspawn
[4262] destroy seat=3 source=Tombspawn
[4263] sba_704_5g seat=3 source=Tombspawn
[4264] destroy seat=3 source=Tombspawn
[4265] sba_704_5g seat=3 source=Tombspawn
[4266] sba_cycle_complete seat=-1 source=
[4267] phase_step seat=3 source= target=seat0
[4268] damage_wears_off seat=2 source=Tombspawn amount=1 target=seat0
[4269] damage_wears_off seat=3 source=Timber Protector amount=2 target=seat0
[4270] damage_wears_off seat=3 source=Carefree Swinemaster amount=2 target=seat0
[4271] state seat=3 source= target=seat0
```

</details>

#### Violation 24

- **Game**: 4474 (seed 44740049, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 59, Phase=ending Step=cleanup
- **Commanders**: Raul, Trouble Shooter, Momo, Playful Pet, Prowler, Clawed Thief, Jenova, Ancient Calamity
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 4263 with trigger-bearer(s) [{Jenova, Ancient Calamity 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 59, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 4272 events
  Seat 0 [alive]: life=18 library=66 hand=2 graveyard=14 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Raul, Trouble Shooter (P/T 0/3, dmg=0) [T]
    - Fire Nation Occupation (P/T 0/0, dmg=0)
    - Plaguecrafter's Familiar (P/T 1/1, dmg=0) [T]
    - Dulcet Sirens (P/T 1/3, dmg=0) [T]
    - Ichor Wellspring (P/T 0/0, dmg=0)
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Hissing Miasma (P/T 0/0, dmg=0)
    - Wall of Mist (P/T 0/5, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Somber Hoverguard (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Tombstone Stairwell (P/T 0/0, dmg=0)
    - creature token black zombie Token (P/T 2/2, dmg=0)
    - Jousting Lance (P/T 0/0, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=25 library=64 hand=5 graveyard=19 exile=0 battlefield=20 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - White Lotus Hideout (P/T 0/0, dmg=0) [T]
    - Contagion Engine (P/T 0/0, dmg=0) [T]
    - food artifact token Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Throne of Empires (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - creature token white soldier Token (P/T 1/1, dmg=0)
    - Laser Screwdriver (P/T 0/0, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
  Seat 2 [alive]: life=3 library=65 hand=4 graveyard=16 exile=0 battlefield=23 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Morphic Pool (P/T 0/0, dmg=0) [T]
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Prowler, Clawed Thief (P/T 2/3, dmg=0) [T]
    - Shelldock Isle (P/T 0/0, dmg=0) [T]
    - Clockwork Fox (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mer-Ek Nightblade (P/T 2/3, dmg=0) [T]
    - Conduit Pylons (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Short Sword (P/T 0/0, dmg=0)
    - Balthor the Defiled (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Tombspawn (P/T 2/2, dmg=0)
    - Desert of the Mindful (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=24 library=65 hand=4 graveyard=15 exile=0 battlefield=17 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Jenova, Ancient Calamity (P/T 0/4, dmg=0)
    - The Mycosynth Gardens (P/T 0/0, dmg=0) [T]
    - Blinkmoth Nexus (P/T 0/0, dmg=0) [T]
    - Timber Protector (P/T 4/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hidden Nursery (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Carefree Swinemaster (P/T 1/4, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0) [T]
    - Tombspawn (P/T 2/2, dmg=0) [T]
    - Restless Cottage (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4252] destroy seat=2 source=Tombspawn
[4253] sba_704_5g seat=2 source=Tombspawn
[4254] destroy seat=2 source=Tombspawn
[4255] sba_704_5g seat=2 source=Tombspawn
[4256] destroy seat=3 source=Tombspawn
[4257] sba_704_5g seat=3 source=Tombspawn
[4258] destroy seat=3 source=Tombspawn
[4259] sba_704_5g seat=3 source=Tombspawn
[4260] destroy seat=3 source=Tombspawn
[4261] sba_704_5g seat=3 source=Tombspawn
[4262] destroy seat=3 source=Tombspawn
[4263] sba_704_5g seat=3 source=Tombspawn
[4264] destroy seat=3 source=Tombspawn
[4265] sba_704_5g seat=3 source=Tombspawn
[4266] sba_cycle_complete seat=-1 source=
[4267] phase_step seat=3 source= target=seat0
[4268] damage_wears_off seat=2 source=Tombspawn amount=1 target=seat0
[4269] damage_wears_off seat=3 source=Timber Protector amount=2 target=seat0
[4270] damage_wears_off seat=3 source=Carefree Swinemaster amount=2 target=seat0
[4271] state seat=3 source= target=seat0
```

</details>

#### Violation 25

- **Game**: 6191 (seed 61910049, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 43, Phase=ending Step=cleanup
- **Commanders**: Ashaya, the Awoken World, Marchesa, the Black Rose, The Mox Painter, Rufus Shinra
- **Message**: TriggerCompleteness: death event "sacrifice" at index 2322 with trigger-bearer(s) [{Marchesa, the Black Rose 1}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 43, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2332 events
  Seat 0 [alive]: life=6 library=71 hand=0 graveyard=4 exile=10 battlefield=14 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Zhalfirin Void (P/T 0/0, dmg=0) [T]
    - Bag of Holding (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - City in a Bottle (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Sidequest: Raise a Chocobo // Black Chocobo (P/T 2/2, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Golden Argosy (P/T 3/6, dmg=0)
    - Blustering Barnyard (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Ashaya, the Awoken World (P/T 4/4, dmg=0)
  Seat 1 [alive]: life=10 library=72 hand=0 graveyard=13 exile=1 battlefield=12 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cover of Darkness (P/T 0/0, dmg=0)
    - Highland Lake (P/T 0/0, dmg=0) [T]
    - Volcanic Island (P/T 0/0, dmg=0) [T]
    - Dreadfeast Demon (P/T 6/6, dmg=0) [T]
    - Vivid Creek (P/T 0/0, dmg=0) [T]
    - Marchesa, the Black Rose (P/T 3/3, dmg=0)
  Seat 2 [alive]: life=9 library=80 hand=6 graveyard=3 exile=1 battlefield=8 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Headsplitter (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Encroaching Wastes (P/T 0/0, dmg=0) [T]
    - Sulfurous Mire (P/T 0/0, dmg=0) [T]
    - Urban Retreat (P/T 0/0, dmg=0) [T]
    - The Mox Painter (P/T 1/4, dmg=0) [T]
  Seat 3 [alive]: life=11 library=77 hand=4 graveyard=1 exile=0 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - The Heron Moon (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Blade of the Sixth Pride (P/T 3/1, dmg=0) [T]
    - Dowsing Dagger // Lost Vale (P/T 0/0, dmg=0) [T]
    - Rufus Shinra (P/T 2/4, dmg=0) [T]
    - Adherent's Heirloom (P/T 0/0, dmg=0)
    - token darkstar, a legendary 2/2 white and black dog Token (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Keskit, the Flesh Sculptor (P/T 1/3, dmg=0)
    - token darkstar, a legendary 2/2 white and black dog Token (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2312] pay_mana seat=1 source=Marchesa, the Black Rose amount=8 target=seat0
[2313] cast seat=1 source=Marchesa, the Black Rose amount=8 target=seat0
[2314] commander_cast_from_command_zone seat=1 source=Marchesa, the Black Rose amount=8 target=seat0
[2315] stack_push seat=1 source=Marchesa, the Black Rose target=seat0
[2316] stack_push seat=1 source=Dreadfeast Demon target=seat0
[2317] triggers_ordered seat=1 source= target=seat0
[2318] priority_pass seat=2 source= target=seat0
[2319] priority_pass seat=3 source= target=seat0
[2320] priority_pass seat=0 source= target=seat0
[2321] stack_resolve seat=1 source=Dreadfeast Demon target=seat0
[2322] sacrifice seat=1 source=Gilacorn target=seat1
[2323] zone_change seat=1 source=Gilacorn
[2324] priority_pass seat=2 source= target=seat0
[2325] priority_pass seat=3 source= target=seat0
[2326] priority_pass seat=0 source= target=seat0
[2327] stack_resolve seat=1 source=Marchesa, the Black Rose target=seat0
[2328] enter_battlefield seat=1 source=Marchesa, the Black Rose target=seat0
[2329] per_card_handler seat=0 source=Marchesa, the Black Rose target=seat0
[2330] damage_wears_off seat=0 source=Ashaya, the Awoken World amount=3 target=seat0
[2331] state seat=1 source= target=seat0
```

</details>

#### Violation 26

- **Game**: 1710 (seed 17100049, perm 0)
- **Invariant**: ZoneCastGrantExpiry
- **Turn**: 52, Phase=ending Step=end
- **Commanders**: Prosper, Tome-Bound, Taborax, Hope's Demise, Agrus Kos, Wojek Veteran, The Rani
- **Message**: ZoneCastGrantExpiry: grant for "Wirefly Hive" (zone=exile duration=until_end_of_turn grantTurn=52 sourceTimestamp=0 source=Prosper, Tome-Bound) has expired but is still in ZoneCastGrants — cleanup missed

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=end Active=seat0
Stack: 0 items, EventLog: 4599 events
  Seat 0 [WON]: life=26 library=64 hand=0 graveyard=9 exile=12 battlefield=16 cmdzone=0 mana=0
    - Burner Rocket (P/T 4/2, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Oath of Chandra (P/T 0/0, dmg=0)
    - Kickoff Celebrations (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Prosper, Tome-Bound (P/T 1/4, dmg=0) [T]
    - Raging Goblinoids (P/T 5/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Suq'Ata Assassin (P/T 1/1, dmg=0) [T]
    - Impetuous Lootmonger (P/T 2/2, dmg=0) [T]
    - Reezug, the Bonecobbler (P/T 1/3, dmg=0) [T]
    - Conduit of Storms // Conduit of Emrakul (P/T 2/3, dmg=0) [T]
    - Goblin Spy (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Treasure Token (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=77 hand=4 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=0 library=79 hand=3 graveyard=6 exile=2 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-4 library=79 hand=1 graveyard=3 exile=2 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[4579] per_card_handler seat=0 source=Prosper, Tome-Bound target=seat0
[4580] sba_704_5a seat=2 source=
[4581] sba_cycle_complete seat=-1 source=
[4582] stack_push seat=0 source=Oath of Chandra target=seat0
[4583] triggers_ordered seat=0 source= target=seat0
[4584] priority_pass seat=1 source= target=seat0
[4585] stack_resolve seat=0 source=Oath of Chandra target=seat0
[4586] commit_crime seat=0 source=Oath of Chandra amount=1 target=seat0
[4587] damage seat=0 source=Oath of Chandra amount=2 target=seat1
[4588] life_change seat=1 source=Oath of Chandra amount=-2 target=seat0
[4589] trigger_evaluated seat=0 source=Prosper, Tome-Bound
[4590] stack_push seat=0 source=Prosper, Tome-Bound target=seat0
[4591] triggered_ability seat=0 source=Prosper, Tome-Bound target=seat0
[4592] priority_pass seat=1 source= target=seat0
[4593] stack_resolve seat=0 source=Prosper, Tome-Bound target=seat0
[4594] sba_704_5a seat=1 source=
[4595] sba_cycle_complete seat=-1 source=
[4596] seat_eliminated seat=1 source= amount=6
[4597] seat_eliminated seat=2 source= amount=13
[4598] game_end seat=0 source=
```

</details>

#### Violation 27

- **Game**: 1710 (seed 17100049, perm 0)
- **Invariant**: ZoneCastGrantExpiry
- **Turn**: 52, Phase=ending Step=end
- **Commanders**: Prosper, Tome-Bound, Taborax, Hope's Demise, Agrus Kos, Wojek Veteran, The Rani
- **Message**: ZoneCastGrantExpiry: grant for "Wirefly Hive" (zone=exile duration=until_end_of_turn grantTurn=52 sourceTimestamp=0 source=Prosper, Tome-Bound) has expired but is still in ZoneCastGrants — cleanup missed

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=end Active=seat0
Stack: 0 items, EventLog: 4599 events
  Seat 0 [WON]: life=26 library=64 hand=0 graveyard=9 exile=12 battlefield=16 cmdzone=0 mana=0
    - Burner Rocket (P/T 4/2, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Oath of Chandra (P/T 0/0, dmg=0)
    - Kickoff Celebrations (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Prosper, Tome-Bound (P/T 1/4, dmg=0) [T]
    - Raging Goblinoids (P/T 5/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Suq'Ata Assassin (P/T 1/1, dmg=0) [T]
    - Impetuous Lootmonger (P/T 2/2, dmg=0) [T]
    - Reezug, the Bonecobbler (P/T 1/3, dmg=0) [T]
    - Conduit of Storms // Conduit of Emrakul (P/T 2/3, dmg=0) [T]
    - Goblin Spy (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Treasure Token (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=77 hand=4 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=0 library=79 hand=3 graveyard=6 exile=2 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-4 library=79 hand=1 graveyard=3 exile=2 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[4579] per_card_handler seat=0 source=Prosper, Tome-Bound target=seat0
[4580] sba_704_5a seat=2 source=
[4581] sba_cycle_complete seat=-1 source=
[4582] stack_push seat=0 source=Oath of Chandra target=seat0
[4583] triggers_ordered seat=0 source= target=seat0
[4584] priority_pass seat=1 source= target=seat0
[4585] stack_resolve seat=0 source=Oath of Chandra target=seat0
[4586] commit_crime seat=0 source=Oath of Chandra amount=1 target=seat0
[4587] damage seat=0 source=Oath of Chandra amount=2 target=seat1
[4588] life_change seat=1 source=Oath of Chandra amount=-2 target=seat0
[4589] trigger_evaluated seat=0 source=Prosper, Tome-Bound
[4590] stack_push seat=0 source=Prosper, Tome-Bound target=seat0
[4591] triggered_ability seat=0 source=Prosper, Tome-Bound target=seat0
[4592] priority_pass seat=1 source= target=seat0
[4593] stack_resolve seat=0 source=Prosper, Tome-Bound target=seat0
[4594] sba_704_5a seat=1 source=
[4595] sba_cycle_complete seat=-1 source=
[4596] seat_eliminated seat=1 source= amount=6
[4597] seat_eliminated seat=2 source= amount=13
[4598] game_end seat=0 source=
```

</details>

#### Violation 28

- **Game**: 1773 (seed 17730049, perm 0)
- **Invariant**: ZoneCastGrantExpiry
- **Turn**: 54, Phase=combat Step=end_of_combat
- **Commanders**: Nashi, Illusion Gadgeteer, Marisi, Breaker of the Coil, Prosper, Tome-Bound, Hidetsugu, Devouring Chaos
- **Message**: ZoneCastGrantExpiry: grant for "Pheres-Band Revelers" (zone=exile duration=until_end_of_turn grantTurn=54 sourceTimestamp=0 source=Prosper, Tome-Bound) has expired but is still in ZoneCastGrants — cleanup missed

<details>
<summary>Game State</summary>

```
Turn 54, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 2937 events
  Seat 0 [LOST]: life=-1 library=78 hand=0 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-10 library=81 hand=5 graveyard=2 exile=1 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=10 library=62 hand=2 graveyard=10 exile=7 battlefield=16 cmdzone=1 mana=5
    - Hammerheim (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Sneaking Guide (P/T 1/1, dmg=0) [T]
    - Urza's Saga (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dragonskull Summit (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Altar of the Goyf (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Rakshasa Gravecaller (P/T 3/6, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mad Prophet (P/T 2/2, dmg=0) [T]
    - Horobi, Death's Wail (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Deep Goblin Skulltaker (P/T 2/2, dmg=0)
  Seat 3 [LOST]: life=-1 library=19 hand=7 graveyard=50 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2917] priority_pass seat=3 source= target=seat0
[2918] stack_resolve seat=2 source=Altar of the Goyf target=seat0
[2919] buff seat=0 source=Altar of the Goyf target=seat0
[2920] blockers seat=3 source= target=seat0
[2921] damage seat=2 source=Prosper, Tome-Bound amount=1 target=seat3
[2922] damage seat=2 source=Rakshasa Gravecaller amount=3 target=seat3
[2923] damage seat=2 source=Horobi, Death's Wail amount=4 target=seat3
[2924] damage seat=3 source=Hidetsugu, Devouring Chaos amount=4 target=seat2
[2925] sba_704_5a seat=3 source= amount=-1
[2926] destroy seat=2 source=Prosper, Tome-Bound
[2927] sba_704_5g seat=2 source=Prosper, Tome-Bound
[2928] zone_change seat=2 source=Prosper, Tome-Bound
[2929] destroy seat=3 source=Hidetsugu, Devouring Chaos
[2930] sba_704_5g seat=3 source=Hidetsugu, Devouring Chaos
[2931] zone_change seat=3 source=Hidetsugu, Devouring Chaos
[2932] sba_704_6d seat=2 source=Prosper, Tome-Bound
[2933] sba_704_6d seat=3 source=Hidetsugu, Devouring Chaos
[2934] sba_cycle_complete seat=-1 source=
[2935] seat_eliminated seat=3 source= amount=21
[2936] game_end seat=2 source=
```

</details>

#### Violation 29

- **Game**: 1773 (seed 17730049, perm 0)
- **Invariant**: ZoneCastGrantExpiry
- **Turn**: 54, Phase=combat Step=end_of_combat
- **Commanders**: Nashi, Illusion Gadgeteer, Marisi, Breaker of the Coil, Prosper, Tome-Bound, Hidetsugu, Devouring Chaos
- **Message**: ZoneCastGrantExpiry: grant for "Pheres-Band Revelers" (zone=exile duration=until_end_of_turn grantTurn=54 sourceTimestamp=0 source=Prosper, Tome-Bound) has expired but is still in ZoneCastGrants — cleanup missed

<details>
<summary>Game State</summary>

```
Turn 54, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 2937 events
  Seat 0 [LOST]: life=-1 library=78 hand=0 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-10 library=81 hand=5 graveyard=2 exile=1 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=10 library=62 hand=2 graveyard=10 exile=7 battlefield=16 cmdzone=1 mana=5
    - Hammerheim (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Sneaking Guide (P/T 1/1, dmg=0) [T]
    - Urza's Saga (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dragonskull Summit (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Altar of the Goyf (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Rakshasa Gravecaller (P/T 3/6, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mad Prophet (P/T 2/2, dmg=0) [T]
    - Horobi, Death's Wail (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Deep Goblin Skulltaker (P/T 2/2, dmg=0)
  Seat 3 [LOST]: life=-1 library=19 hand=7 graveyard=50 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2917] priority_pass seat=3 source= target=seat0
[2918] stack_resolve seat=2 source=Altar of the Goyf target=seat0
[2919] buff seat=0 source=Altar of the Goyf target=seat0
[2920] blockers seat=3 source= target=seat0
[2921] damage seat=2 source=Prosper, Tome-Bound amount=1 target=seat3
[2922] damage seat=2 source=Rakshasa Gravecaller amount=3 target=seat3
[2923] damage seat=2 source=Horobi, Death's Wail amount=4 target=seat3
[2924] damage seat=3 source=Hidetsugu, Devouring Chaos amount=4 target=seat2
[2925] sba_704_5a seat=3 source= amount=-1
[2926] destroy seat=2 source=Prosper, Tome-Bound
[2927] sba_704_5g seat=2 source=Prosper, Tome-Bound
[2928] zone_change seat=2 source=Prosper, Tome-Bound
[2929] destroy seat=3 source=Hidetsugu, Devouring Chaos
[2930] sba_704_5g seat=3 source=Hidetsugu, Devouring Chaos
[2931] zone_change seat=3 source=Hidetsugu, Devouring Chaos
[2932] sba_704_6d seat=2 source=Prosper, Tome-Bound
[2933] sba_704_6d seat=3 source=Hidetsugu, Devouring Chaos
[2934] sba_cycle_complete seat=-1 source=
[2935] seat_eliminated seat=3 source= amount=21
[2936] game_end seat=2 source=
```

</details>

#### Violation 30

- **Game**: 2711 (seed 27110049, perm 0)
- **Invariant**: ZoneCastGrantExpiry
- **Turn**: 32, Phase=combat Step=end_of_combat
- **Commanders**: Lord Dregg, Insect Invader, Sally Sparrow, Ashling, the Limitless, Brigone, Soldier of Meletis
- **Message**: ZoneCastGrantExpiry: grant for "Trace of Abundance" (zone=exile duration=until_end_of_turn grantTurn=32 sourceTimestamp=0 source=Ashling, the Limitless) has expired but is still in ZoneCastGrants — cleanup missed

<details>
<summary>Game State</summary>

```
Turn 32, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 2251 events
  Seat 0 [LOST]: life=-18 library=84 hand=3 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-10 library=85 hand=2 graveyard=1 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=26 library=65 hand=5 graveyard=2 exile=15 battlefield=16 cmdzone=0 mana=1
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Ashling, the Limitless (P/T 2/3, dmg=0) [T]
    - Stoneskin (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ward Sliver (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - creature token green saproling Token (P/T 1/1, dmg=0) [T]
    - creature token green saproling Token (P/T 1/1, dmg=0) [T]
    - creature token green saproling Token (P/T 1/1, dmg=0) [T]
    - creature token green saproling Token (P/T 1/1, dmg=0) [T]
    - creature token green saproling Token (P/T 1/1, dmg=0) [T]
    - creature token green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [LOST]: life=-1 library=76 hand=5 graveyard=7 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2231] zone_change seat=2 source=Geothermal Crevice
[2232] per_card_handler seat=0 source=Ashling, the Limitless target=seat0
[2233] per_card_partial seat=0 source=Ashling, the Limitless target=seat0
[2234] parser_gap seat=0 source=Ashling, the Limitless target=seat0
[2235] damage seat=2 source=Clay Golem amount=4 target=seat3
[2236] damage seat=2 source=Ward Sliver amount=2 target=seat3
[2237] trigger_evaluated seat=2 source=Ashling, the Limitless
[2238] stack_push seat=2 source=Ashling, the Limitless target=seat0
[2239] triggered_ability seat=2 source=Ashling, the Limitless target=seat0
[2240] priority_pass seat=3 source= target=seat0
[2241] stack_resolve seat=2 source=Ashling, the Limitless target=seat0
[2242] sba_704_5a seat=3 source= amount=-1
[2243] sba_cycle_complete seat=-1 source=
[2244] damage seat=3 source=Cybermen Squadron amount=5 target=seat2
[2245] destroy seat=2 source=Clay Golem
[2246] sba_704_5g seat=2 source=Clay Golem
[2247] zone_change seat=2 source=Clay Golem
[2248] sba_cycle_complete seat=-1 source=
[2249] seat_eliminated seat=3 source= amount=12
[2250] game_end seat=2 source=
```

</details>

*... and 316 more violations not shown.*

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Geyadrone Dihada | 1 | 2 | 0.33 |
| 2 | Triumphant Getaway | 1 | 2 | 0.33 |
| 3 | Sister Hospitaller | 2 | 5 | 0.29 |
| 4 | Trace of Abundance | 1 | 3 | 0.25 |
| 5 | Faithful Mending | 1 | 3 | 0.25 |
| 6 | Geothermal Crevice | 2 | 6 | 0.25 |
| 7 | Satya, Aetherflux Genius | 3 | 10 | 0.23 |
| 8 | Ajani Unyielding | 1 | 4 | 0.20 |
| 9 | Ghitu Amplifier | 1 | 4 | 0.20 |
| 10 | Admiral Beckett Brass | 1 | 4 | 0.20 |

## Verdict: ISSUES FOUND

**346 total issues** across 7500 chaos games and 0 nightmare boards.
- 0 crashes in chaos games
- 346 invariant violations in chaos games
- 0 crashes in nightmare boards
- 0 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
