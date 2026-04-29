# Chaos Gauntlet Report

Generated: 2026-04-18T18:35:35-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36510 cards |
| Legendary Creatures | 3434 |
| Total Games | 1000 |
| Seed | 12345 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 15.044s |
| Throughput | 66 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 1038 (in 507 games) |
| Clean Games | 493 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 1.573s |
| Throughput | 6359 boards/sec |
| Crashes | 0 |
| Invariant Violations | 49 |
| Clean Boards | 9971 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| ResourceConservation | 1024 |
| TriggerCompleteness | 14 |

### Violation Details (first 30)

#### Violation 1

- **Game**: 3 (seed 42346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 33, Phase=combat Step=end_of_combat
- **Commanders**: Mannichi, the Fevered Dream, Kykar, Zephyr Awakener, Alibou, Ancient Witness, General Marhault Elsdragon
- **Message**: ResourceConservation: seat 1 ManaPool=6 but typed Mana.Total()=7 — desync

<details>
<summary>Game State</summary>

```
Turn 33, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 1115 events
  Seat 0 [LOST]: life=-8 library=82 hand=0 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [WON]: life=10 library=80 hand=0 graveyard=2 exile=0 battlefield=17 cmdzone=0 mana=6
    - Plains (P/T 0/0, dmg=0) [T]
    - Benalish Trapper (P/T 1/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Rarity (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mogis's Chosen (P/T 5/4, dmg=0) [T]
    - Kykar, Zephyr Awakener (P/T 3/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Loxodon Peacekeeper (P/T 4/4, dmg=0) [T]
    - Study (P/T 0/0, dmg=0) [T]
    - Island of Wak-Wak (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Court of Ardenvale (P/T 0/0, dmg=0)
    - Waste Land (P/T 0/0, dmg=0) [T]
    - Hookblade (P/T 0/0, dmg=0)
    - Silvercoat Lion (P/T 2/2, dmg=0)
  Seat 2 [LOST]: life=-11 library=86 hand=2 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-8 library=84 hand=6 graveyard=2 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1095] enter_battlefield seat=1 source=Silvercoat Lion target=seat0
[1096] tap seat=1 source=Benalish Trapper target=seat0
[1097] pay_mana seat=1 source=Benalish Trapper amount=1 target=seat0
[1098] activate_ability seat=1 source=Benalish Trapper target=seat0
[1099] stack_push seat=1 source=Benalish Trapper target=seat0
[1100] priority_pass seat=0 source= target=seat0
[1101] stack_resolve seat=1 source=Benalish Trapper target=seat0
[1102] tap seat=0 source=Benalish Trapper target=seat0
[1103] activated_ability_resolved seat=1 source=Benalish Trapper target=seat0
[1104] phase_step seat=1 source= target=seat0
[1105] declare_attackers seat=1 source= target=seat0
[1106] blockers seat=0 source= target=seat0
[1107] damage seat=1 source=Rarity amount=2 target=seat0
[1108] damage seat=1 source=Mogis's Chosen amount=5 target=seat0
[1109] damage seat=1 source=Kykar, Zephyr Awakener amount=3 target=seat0
[1110] damage seat=1 source=Loxodon Peacekeeper amount=4 target=seat0
[1111] sba_704_5a seat=0 source= amount=-8
[1112] sba_cycle_complete seat=-1 source=
[1113] seat_eliminated seat=0 source= amount=13
[1114] game_end seat=1 source=
```

</details>

#### Violation 2

- **Game**: 3 (seed 42346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 33, Phase=combat Step=end_of_combat
- **Commanders**: Mannichi, the Fevered Dream, Kykar, Zephyr Awakener, Alibou, Ancient Witness, General Marhault Elsdragon
- **Message**: ResourceConservation: seat 1 ManaPool=6 but typed Mana.Total()=7 — desync

<details>
<summary>Game State</summary>

```
Turn 33, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 1115 events
  Seat 0 [LOST]: life=-8 library=82 hand=0 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [WON]: life=10 library=80 hand=0 graveyard=2 exile=0 battlefield=17 cmdzone=0 mana=6
    - Plains (P/T 0/0, dmg=0) [T]
    - Benalish Trapper (P/T 1/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Rarity (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mogis's Chosen (P/T 5/4, dmg=0) [T]
    - Kykar, Zephyr Awakener (P/T 3/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Loxodon Peacekeeper (P/T 4/4, dmg=0) [T]
    - Study (P/T 0/0, dmg=0) [T]
    - Island of Wak-Wak (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Court of Ardenvale (P/T 0/0, dmg=0)
    - Waste Land (P/T 0/0, dmg=0) [T]
    - Hookblade (P/T 0/0, dmg=0)
    - Silvercoat Lion (P/T 2/2, dmg=0)
  Seat 2 [LOST]: life=-11 library=86 hand=2 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-8 library=84 hand=6 graveyard=2 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1095] enter_battlefield seat=1 source=Silvercoat Lion target=seat0
[1096] tap seat=1 source=Benalish Trapper target=seat0
[1097] pay_mana seat=1 source=Benalish Trapper amount=1 target=seat0
[1098] activate_ability seat=1 source=Benalish Trapper target=seat0
[1099] stack_push seat=1 source=Benalish Trapper target=seat0
[1100] priority_pass seat=0 source= target=seat0
[1101] stack_resolve seat=1 source=Benalish Trapper target=seat0
[1102] tap seat=0 source=Benalish Trapper target=seat0
[1103] activated_ability_resolved seat=1 source=Benalish Trapper target=seat0
[1104] phase_step seat=1 source= target=seat0
[1105] declare_attackers seat=1 source= target=seat0
[1106] blockers seat=0 source= target=seat0
[1107] damage seat=1 source=Rarity amount=2 target=seat0
[1108] damage seat=1 source=Mogis's Chosen amount=5 target=seat0
[1109] damage seat=1 source=Kykar, Zephyr Awakener amount=3 target=seat0
[1110] damage seat=1 source=Loxodon Peacekeeper amount=4 target=seat0
[1111] sba_704_5a seat=0 source= amount=-8
[1112] sba_cycle_complete seat=-1 source=
[1113] seat_eliminated seat=0 source= amount=13
[1114] game_end seat=1 source=
```

</details>

#### Violation 3

- **Game**: 9 (seed 102346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 43, Phase=combat Step=end_of_combat
- **Commanders**: Rasaad, Shadow Monk, Tom Bombadil, Pavel Maliki, Alora, Cheerful Thief
- **Message**: ResourceConservation: seat 2 ManaPool=0 but typed Mana.Total()=5 — desync

<details>
<summary>Game State</summary>

```
Turn 43, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1573 events
  Seat 0 [LOST]: life=-1 library=85 hand=3 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-5 library=77 hand=7 graveyard=0 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=18 library=78 hand=7 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Embereth Veteran (P/T 2/1, dmg=0) [T]
    - Goblin Blast-Runner (P/T 1/2, dmg=0) [T]
    - Goblin Bushwhacker (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Pavel Maliki (P/T 5/3, dmg=0) [T]
    - Ceaseless Searblades (P/T 2/4, dmg=0) [T]
    - Aetheric Amplifier (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-8 library=81 hand=5 graveyard=3 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1553] activated_ability_resolved seat=2 source=Embereth Veteran target=seat0
[1554] pay_mana seat=2 source=Embereth Veteran amount=1 target=seat0
[1555] activate_ability seat=2 source=Embereth Veteran target=seat0
[1556] stack_push seat=2 source=Embereth Veteran target=seat0
[1557] priority_pass seat=1 source= target=seat0
[1558] stack_resolve seat=2 source=Embereth Veteran target=seat0
[1559] parsed_effect_residual seat=2 source=Embereth Veteran target=seat0
[1560] activated_ability_resolved seat=2 source=Embereth Veteran target=seat0
[1561] phase_step seat=2 source= target=seat0
[1562] declare_attackers seat=2 source= target=seat0
[1563] blockers seat=1 source= target=seat0
[1564] damage seat=2 source=Embereth Veteran amount=2 target=seat1
[1565] damage seat=2 source=Goblin Blast-Runner amount=1 target=seat1
[1566] damage seat=2 source=Goblin Bushwhacker amount=1 target=seat1
[1567] damage seat=2 source=Pavel Maliki amount=5 target=seat1
[1568] damage seat=2 source=Ceaseless Searblades amount=2 target=seat1
[1569] sba_704_5a seat=1 source= amount=-5
[1570] sba_cycle_complete seat=-1 source=
[1571] seat_eliminated seat=1 source= amount=15
[1572] game_end seat=2 source=
```

</details>

#### Violation 4

- **Game**: 9 (seed 102346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 43, Phase=combat Step=end_of_combat
- **Commanders**: Rasaad, Shadow Monk, Tom Bombadil, Pavel Maliki, Alora, Cheerful Thief
- **Message**: ResourceConservation: seat 2 ManaPool=0 but typed Mana.Total()=5 — desync

<details>
<summary>Game State</summary>

```
Turn 43, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1573 events
  Seat 0 [LOST]: life=-1 library=85 hand=3 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-5 library=77 hand=7 graveyard=0 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=18 library=78 hand=7 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Embereth Veteran (P/T 2/1, dmg=0) [T]
    - Goblin Blast-Runner (P/T 1/2, dmg=0) [T]
    - Goblin Bushwhacker (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Pavel Maliki (P/T 5/3, dmg=0) [T]
    - Ceaseless Searblades (P/T 2/4, dmg=0) [T]
    - Aetheric Amplifier (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-8 library=81 hand=5 graveyard=3 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1553] activated_ability_resolved seat=2 source=Embereth Veteran target=seat0
[1554] pay_mana seat=2 source=Embereth Veteran amount=1 target=seat0
[1555] activate_ability seat=2 source=Embereth Veteran target=seat0
[1556] stack_push seat=2 source=Embereth Veteran target=seat0
[1557] priority_pass seat=1 source= target=seat0
[1558] stack_resolve seat=2 source=Embereth Veteran target=seat0
[1559] parsed_effect_residual seat=2 source=Embereth Veteran target=seat0
[1560] activated_ability_resolved seat=2 source=Embereth Veteran target=seat0
[1561] phase_step seat=2 source= target=seat0
[1562] declare_attackers seat=2 source= target=seat0
[1563] blockers seat=1 source= target=seat0
[1564] damage seat=2 source=Embereth Veteran amount=2 target=seat1
[1565] damage seat=2 source=Goblin Blast-Runner amount=1 target=seat1
[1566] damage seat=2 source=Goblin Bushwhacker amount=1 target=seat1
[1567] damage seat=2 source=Pavel Maliki amount=5 target=seat1
[1568] damage seat=2 source=Ceaseless Searblades amount=2 target=seat1
[1569] sba_704_5a seat=1 source= amount=-5
[1570] sba_cycle_complete seat=-1 source=
[1571] seat_eliminated seat=1 source= amount=15
[1572] game_end seat=2 source=
```

</details>

#### Violation 5

- **Game**: 7 (seed 82346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 37, Phase=combat Step=end_of_combat
- **Commanders**: Jamie McCrimmon, Iraxxa, Empress of Mars, Kamachal, Ship's Mascot, Polukranos, Unchained
- **Message**: ResourceConservation: seat 0 ManaPool=5 but typed Mana.Total()=6 — desync

<details>
<summary>Game State</summary>

```
Turn 37, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 1374 events
  Seat 0 [WON]: life=32 library=78 hand=4 graveyard=2 exile=0 battlefield=13 cmdzone=0 mana=5
    - Forest (P/T 0/0, dmg=0) [T]
    - Tromell, Seymour's Butler (P/T 2/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Jamie McCrimmon (P/T 2/2, dmg=0) [T]
    - Guildless Commons (P/T 0/0, dmg=0) [T]
    - Spiked Baloth (P/T 4/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Adaptive Snapjaw (P/T 6/2, dmg=0) [T]
    - Saproling Burst (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Temur War Shaman (P/T 4/5, dmg=0) [T]
    - Wakeroot Elemental (P/T 5/5, dmg=0) [T]
  Seat 1 [LOST]: life=-8 library=82 hand=7 graveyard=0 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=-7 library=81 hand=4 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-4 library=86 hand=3 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1354] parsed_effect_residual seat=0 source=Saproling Burst target=seat0
[1355] activated_ability_resolved seat=0 source=Saproling Burst target=seat0
[1356] activate_ability seat=0 source=Saproling Burst target=seat0
[1357] stack_push seat=0 source=Saproling Burst target=seat0
[1358] priority_pass seat=2 source= target=seat0
[1359] stack_resolve seat=0 source=Saproling Burst target=seat0
[1360] parsed_effect_residual seat=0 source=Saproling Burst target=seat0
[1361] activated_ability_resolved seat=0 source=Saproling Burst target=seat0
[1362] phase_step seat=0 source= target=seat0
[1363] declare_attackers seat=0 source= target=seat0
[1364] blockers seat=2 source= target=seat0
[1365] damage seat=0 source=Jamie McCrimmon amount=2 target=seat2
[1366] damage seat=0 source=Spiked Baloth amount=4 target=seat2
[1367] damage seat=0 source=Adaptive Snapjaw amount=6 target=seat2
[1368] damage seat=0 source=Temur War Shaman amount=4 target=seat2
[1369] damage seat=0 source=Wakeroot Elemental amount=5 target=seat2
[1370] sba_704_5a seat=2 source= amount=-7
[1371] sba_cycle_complete seat=-1 source=
[1372] seat_eliminated seat=2 source= amount=11
[1373] game_end seat=0 source=
```

</details>

#### Violation 6

- **Game**: 7 (seed 82346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 37, Phase=combat Step=end_of_combat
- **Commanders**: Jamie McCrimmon, Iraxxa, Empress of Mars, Kamachal, Ship's Mascot, Polukranos, Unchained
- **Message**: ResourceConservation: seat 0 ManaPool=5 but typed Mana.Total()=6 — desync

<details>
<summary>Game State</summary>

```
Turn 37, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 1374 events
  Seat 0 [WON]: life=32 library=78 hand=4 graveyard=2 exile=0 battlefield=13 cmdzone=0 mana=5
    - Forest (P/T 0/0, dmg=0) [T]
    - Tromell, Seymour's Butler (P/T 2/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Jamie McCrimmon (P/T 2/2, dmg=0) [T]
    - Guildless Commons (P/T 0/0, dmg=0) [T]
    - Spiked Baloth (P/T 4/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Adaptive Snapjaw (P/T 6/2, dmg=0) [T]
    - Saproling Burst (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Temur War Shaman (P/T 4/5, dmg=0) [T]
    - Wakeroot Elemental (P/T 5/5, dmg=0) [T]
  Seat 1 [LOST]: life=-8 library=82 hand=7 graveyard=0 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=-7 library=81 hand=4 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-4 library=86 hand=3 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1354] parsed_effect_residual seat=0 source=Saproling Burst target=seat0
[1355] activated_ability_resolved seat=0 source=Saproling Burst target=seat0
[1356] activate_ability seat=0 source=Saproling Burst target=seat0
[1357] stack_push seat=0 source=Saproling Burst target=seat0
[1358] priority_pass seat=2 source= target=seat0
[1359] stack_resolve seat=0 source=Saproling Burst target=seat0
[1360] parsed_effect_residual seat=0 source=Saproling Burst target=seat0
[1361] activated_ability_resolved seat=0 source=Saproling Burst target=seat0
[1362] phase_step seat=0 source= target=seat0
[1363] declare_attackers seat=0 source= target=seat0
[1364] blockers seat=2 source= target=seat0
[1365] damage seat=0 source=Jamie McCrimmon amount=2 target=seat2
[1366] damage seat=0 source=Spiked Baloth amount=4 target=seat2
[1367] damage seat=0 source=Adaptive Snapjaw amount=6 target=seat2
[1368] damage seat=0 source=Temur War Shaman amount=4 target=seat2
[1369] damage seat=0 source=Wakeroot Elemental amount=5 target=seat2
[1370] sba_704_5a seat=2 source= amount=-7
[1371] sba_cycle_complete seat=-1 source=
[1372] seat_eliminated seat=2 source= amount=11
[1373] game_end seat=0 source=
```

</details>

#### Violation 7

- **Game**: 8 (seed 92346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 40, Phase=combat Step=end_of_combat
- **Commanders**: Inniaz, the Gale Force, Trelasarra, Moon Dancer, Shadowheart, Dark Justiciar, Deadpool, Trading Card
- **Message**: ResourceConservation: seat 3 ManaPool=1 but typed Mana.Total()=4 — desync

<details>
<summary>Game State</summary>

```
Turn 40, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1431 events
  Seat 0 [LOST]: life=-24 library=81 hand=1 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-13 library=84 hand=4 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-2 library=81 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=35 library=75 hand=1 graveyard=4 exile=0 battlefield=18 cmdzone=0 mana=1
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Deadpool, Trading Card (P/T 5/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Bloodsworn Steward (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Liliana, Death's Majesty (P/T 0/5, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Polluted Dead (P/T 3/3, dmg=0) [T]
    - Undead Leotau (P/T 3/4, dmg=0) [T]
    - Isolation Cell (P/T 0/0, dmg=0)
    - Banewhip Punisher (P/T 2/2, dmg=0) [T]
    - Ilharg, the Raze-Boar (P/T 6/6, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Devourer of Destiny (P/T 6/6, dmg=0) [T]
    - Knight of Infamy (P/T 2/1, dmg=0) [T]
    - Gimli of the Glittering Caves (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1411] priority_pass seat=0 source= target=seat0
[1412] stack_resolve seat=3 source=Deadpool, Trading Card target=seat0
[1413] draw seat=3 source=Deadpool, Trading Card amount=1 target=seat3
[1414] activated_ability_resolved seat=3 source=Deadpool, Trading Card target=seat0
[1415] phase_step seat=3 source= target=seat0
[1416] declare_attackers seat=3 source= target=seat0
[1417] blockers seat=0 source= target=seat0
[1418] damage seat=3 source=Deadpool, Trading Card amount=5 target=seat0
[1419] damage seat=3 source=Bloodsworn Steward amount=4 target=seat0
[1420] damage seat=3 source=creature token black zombie Token amount=2 target=seat0
[1421] damage seat=3 source=Polluted Dead amount=3 target=seat0
[1422] damage seat=3 source=Undead Leotau amount=3 target=seat0
[1423] damage seat=3 source=Banewhip Punisher amount=2 target=seat0
[1424] damage seat=3 source=Ilharg, the Raze-Boar amount=6 target=seat0
[1425] damage seat=3 source=Devourer of Destiny amount=6 target=seat0
[1426] damage seat=3 source=Knight of Infamy amount=2 target=seat0
[1427] sba_704_5a seat=0 source= amount=-24
[1428] sba_cycle_complete seat=-1 source=
[1429] seat_eliminated seat=0 source= amount=11
[1430] game_end seat=3 source=
```

</details>

#### Violation 8

- **Game**: 8 (seed 92346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 40, Phase=combat Step=end_of_combat
- **Commanders**: Inniaz, the Gale Force, Trelasarra, Moon Dancer, Shadowheart, Dark Justiciar, Deadpool, Trading Card
- **Message**: ResourceConservation: seat 3 ManaPool=1 but typed Mana.Total()=4 — desync

<details>
<summary>Game State</summary>

```
Turn 40, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1431 events
  Seat 0 [LOST]: life=-24 library=81 hand=1 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-13 library=84 hand=4 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-2 library=81 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=35 library=75 hand=1 graveyard=4 exile=0 battlefield=18 cmdzone=0 mana=1
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Deadpool, Trading Card (P/T 5/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Bloodsworn Steward (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Liliana, Death's Majesty (P/T 0/5, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Polluted Dead (P/T 3/3, dmg=0) [T]
    - Undead Leotau (P/T 3/4, dmg=0) [T]
    - Isolation Cell (P/T 0/0, dmg=0)
    - Banewhip Punisher (P/T 2/2, dmg=0) [T]
    - Ilharg, the Raze-Boar (P/T 6/6, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Devourer of Destiny (P/T 6/6, dmg=0) [T]
    - Knight of Infamy (P/T 2/1, dmg=0) [T]
    - Gimli of the Glittering Caves (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1411] priority_pass seat=0 source= target=seat0
[1412] stack_resolve seat=3 source=Deadpool, Trading Card target=seat0
[1413] draw seat=3 source=Deadpool, Trading Card amount=1 target=seat3
[1414] activated_ability_resolved seat=3 source=Deadpool, Trading Card target=seat0
[1415] phase_step seat=3 source= target=seat0
[1416] declare_attackers seat=3 source= target=seat0
[1417] blockers seat=0 source= target=seat0
[1418] damage seat=3 source=Deadpool, Trading Card amount=5 target=seat0
[1419] damage seat=3 source=Bloodsworn Steward amount=4 target=seat0
[1420] damage seat=3 source=creature token black zombie Token amount=2 target=seat0
[1421] damage seat=3 source=Polluted Dead amount=3 target=seat0
[1422] damage seat=3 source=Undead Leotau amount=3 target=seat0
[1423] damage seat=3 source=Banewhip Punisher amount=2 target=seat0
[1424] damage seat=3 source=Ilharg, the Raze-Boar amount=6 target=seat0
[1425] damage seat=3 source=Devourer of Destiny amount=6 target=seat0
[1426] damage seat=3 source=Knight of Infamy amount=2 target=seat0
[1427] sba_704_5a seat=0 source= amount=-24
[1428] sba_cycle_complete seat=-1 source=
[1429] seat_eliminated seat=0 source= amount=11
[1430] game_end seat=3 source=
```

</details>

#### Violation 9

- **Game**: 5 (seed 62346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 33, Phase=combat Step=end_of_combat
- **Commanders**: Farideh, Devil's Chosen, Kasla, the Broken Halo, Tuknir Deathlock, Arcades Sabboth
- **Message**: ResourceConservation: seat 3 ManaPool=9 but typed Mana.Total()=11 — desync

<details>
<summary>Game State</summary>

```
Turn 33, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1771 events
  Seat 0 [LOST]: life=-1 library=87 hand=5 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=66 hand=2 graveyard=15 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-1 library=85 hand=7 graveyard=2 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=24 library=75 hand=1 graveyard=7 exile=0 battlefield=15 cmdzone=0 mana=9
    - Plains (P/T 0/0, dmg=0) [T]
    - Arcades Sabboth (P/T 7/9, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Kitesail Scout (P/T 1/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Rocksteady, Crash Courser (P/T 7/7, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Caduceus, Staff of Hermes (P/T 0/0, dmg=0)
    - Soldevi Excavations (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1751] stack_resolve seat=3 source=Arcades Sabboth target=seat0
[1752] buff seat=0 source=Arcades Sabboth target=seat0
[1753] activated_ability_resolved seat=3 source=Arcades Sabboth target=seat0
[1754] pay_mana seat=3 source=Arcades Sabboth amount=1 target=seat0
[1755] activate_ability seat=3 source=Arcades Sabboth target=seat0
[1756] stack_push seat=3 source=Arcades Sabboth target=seat0
[1757] priority_pass seat=1 source= target=seat0
[1758] stack_resolve seat=3 source=Arcades Sabboth target=seat0
[1759] buff seat=0 source=Arcades Sabboth target=seat0
[1760] activated_ability_resolved seat=3 source=Arcades Sabboth target=seat0
[1761] phase_step seat=3 source= target=seat0
[1762] declare_attackers seat=3 source= target=seat0
[1763] blockers seat=1 source= target=seat0
[1764] damage seat=3 source=Arcades Sabboth amount=7 target=seat1
[1765] damage seat=3 source=Kitesail Scout amount=1 target=seat1
[1766] damage seat=3 source=Rocksteady, Crash Courser amount=7 target=seat1
[1767] sba_704_5a seat=1 source= amount=-14
[1768] sba_cycle_complete seat=-1 source=
[1769] seat_eliminated seat=1 source= amount=15
[1770] game_end seat=3 source=
```

</details>

#### Violation 10

- **Game**: 5 (seed 62346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 33, Phase=combat Step=end_of_combat
- **Commanders**: Farideh, Devil's Chosen, Kasla, the Broken Halo, Tuknir Deathlock, Arcades Sabboth
- **Message**: ResourceConservation: seat 3 ManaPool=9 but typed Mana.Total()=11 — desync

<details>
<summary>Game State</summary>

```
Turn 33, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1771 events
  Seat 0 [LOST]: life=-1 library=87 hand=5 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=66 hand=2 graveyard=15 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-1 library=85 hand=7 graveyard=2 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=24 library=75 hand=1 graveyard=7 exile=0 battlefield=15 cmdzone=0 mana=9
    - Plains (P/T 0/0, dmg=0) [T]
    - Arcades Sabboth (P/T 7/9, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Kitesail Scout (P/T 1/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Rocksteady, Crash Courser (P/T 7/7, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Caduceus, Staff of Hermes (P/T 0/0, dmg=0)
    - Soldevi Excavations (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1751] stack_resolve seat=3 source=Arcades Sabboth target=seat0
[1752] buff seat=0 source=Arcades Sabboth target=seat0
[1753] activated_ability_resolved seat=3 source=Arcades Sabboth target=seat0
[1754] pay_mana seat=3 source=Arcades Sabboth amount=1 target=seat0
[1755] activate_ability seat=3 source=Arcades Sabboth target=seat0
[1756] stack_push seat=3 source=Arcades Sabboth target=seat0
[1757] priority_pass seat=1 source= target=seat0
[1758] stack_resolve seat=3 source=Arcades Sabboth target=seat0
[1759] buff seat=0 source=Arcades Sabboth target=seat0
[1760] activated_ability_resolved seat=3 source=Arcades Sabboth target=seat0
[1761] phase_step seat=3 source= target=seat0
[1762] declare_attackers seat=3 source= target=seat0
[1763] blockers seat=1 source= target=seat0
[1764] damage seat=3 source=Arcades Sabboth amount=7 target=seat1
[1765] damage seat=3 source=Kitesail Scout amount=1 target=seat1
[1766] damage seat=3 source=Rocksteady, Crash Courser amount=7 target=seat1
[1767] sba_704_5a seat=1 source= amount=-14
[1768] sba_cycle_complete seat=-1 source=
[1769] seat_eliminated seat=1 source= amount=15
[1770] game_end seat=3 source=
```

</details>

#### Violation 11

- **Game**: 2 (seed 32346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 47, Phase=combat Step=end_of_combat
- **Commanders**: Balmor, Battlemage Captain, Atarka, World Render, Susan Foreman, Shiko and Narset, Unified
- **Message**: ResourceConservation: seat 3 ManaPool=2 but typed Mana.Total()=9 — desync

<details>
<summary>Game State</summary>

```
Turn 47, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1712 events
  Seat 0 [LOST]: life=-3 library=75 hand=1 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-4 library=82 hand=1 graveyard=9 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-10 library=81 hand=2 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=12 library=75 hand=2 graveyard=9 exile=0 battlefield=12 cmdzone=1 mana=2
    - Island (P/T 0/0, dmg=0) [T]
    - Raucous Carnival (P/T 0/0, dmg=0) [T]
    - Karoo (P/T 0/0, dmg=0) [T]
    - Thawing Glaciers (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Brutal Deceiver (P/T 2/2, dmg=0) [T]
    - Loxodon Mender (P/T 3/3, dmg=0) [T]
    - Izzet Boilerworks (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1692] activate_ability seat=3 source=Brutal Deceiver target=seat0
[1693] stack_push seat=3 source=Brutal Deceiver target=seat0
[1694] priority_pass seat=0 source= target=seat0
[1695] stack_resolve seat=3 source=Brutal Deceiver target=seat0
[1696] look_at seat=3 source=Brutal Deceiver amount=1 target=seat0
[1697] activated_ability_resolved seat=3 source=Brutal Deceiver target=seat0
[1698] phase_step seat=3 source= target=seat0
[1699] declare_attackers seat=3 source= target=seat0
[1700] blockers seat=0 source= target=seat0
[1701] damage seat=3 source=Balduvian War-Makers amount=3 target=seat0
[1702] damage seat=3 source=Brutal Deceiver amount=2 target=seat0
[1703] damage seat=3 source=Loxodon Mender amount=3 target=seat0
[1704] damage seat=0 source=Stratozeppelid amount=4 target=seat3
[1705] sba_704_5a seat=0 source= amount=-3
[1706] destroy seat=3 source=Balduvian War-Makers
[1707] sba_704_5g seat=3 source=Balduvian War-Makers
[1708] zone_change seat=3 source=Balduvian War-Makers
[1709] sba_cycle_complete seat=-1 source=
[1710] seat_eliminated seat=0 source= amount=14
[1711] game_end seat=3 source=
```

</details>

#### Violation 12

- **Game**: 2 (seed 32346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 47, Phase=combat Step=end_of_combat
- **Commanders**: Balmor, Battlemage Captain, Atarka, World Render, Susan Foreman, Shiko and Narset, Unified
- **Message**: ResourceConservation: seat 3 ManaPool=2 but typed Mana.Total()=9 — desync

<details>
<summary>Game State</summary>

```
Turn 47, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1712 events
  Seat 0 [LOST]: life=-3 library=75 hand=1 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-4 library=82 hand=1 graveyard=9 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-10 library=81 hand=2 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=12 library=75 hand=2 graveyard=9 exile=0 battlefield=12 cmdzone=1 mana=2
    - Island (P/T 0/0, dmg=0) [T]
    - Raucous Carnival (P/T 0/0, dmg=0) [T]
    - Karoo (P/T 0/0, dmg=0) [T]
    - Thawing Glaciers (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Brutal Deceiver (P/T 2/2, dmg=0) [T]
    - Loxodon Mender (P/T 3/3, dmg=0) [T]
    - Izzet Boilerworks (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1692] activate_ability seat=3 source=Brutal Deceiver target=seat0
[1693] stack_push seat=3 source=Brutal Deceiver target=seat0
[1694] priority_pass seat=0 source= target=seat0
[1695] stack_resolve seat=3 source=Brutal Deceiver target=seat0
[1696] look_at seat=3 source=Brutal Deceiver amount=1 target=seat0
[1697] activated_ability_resolved seat=3 source=Brutal Deceiver target=seat0
[1698] phase_step seat=3 source= target=seat0
[1699] declare_attackers seat=3 source= target=seat0
[1700] blockers seat=0 source= target=seat0
[1701] damage seat=3 source=Balduvian War-Makers amount=3 target=seat0
[1702] damage seat=3 source=Brutal Deceiver amount=2 target=seat0
[1703] damage seat=3 source=Loxodon Mender amount=3 target=seat0
[1704] damage seat=0 source=Stratozeppelid amount=4 target=seat3
[1705] sba_704_5a seat=0 source= amount=-3
[1706] destroy seat=3 source=Balduvian War-Makers
[1707] sba_704_5g seat=3 source=Balduvian War-Makers
[1708] zone_change seat=3 source=Balduvian War-Makers
[1709] sba_cycle_complete seat=-1 source=
[1710] seat_eliminated seat=0 source= amount=14
[1711] game_end seat=3 source=
```

</details>

#### Violation 13

- **Game**: 1 (seed 22346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 59, Phase=combat Step=end_of_combat
- **Commanders**: Liara of the Flaming Fist, Hylda of the Icy Crown, Rakdos, Patron of Chaos, Valgavoth, Terror Eater
- **Message**: ResourceConservation: seat 3 ManaPool=1 but typed Mana.Total()=11 — desync

<details>
<summary>Game State</summary>

```
Turn 59, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 2916 events
  Seat 0 [LOST]: life=-1 library=78 hand=5 graveyard=11 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-9 library=73 hand=1 graveyard=8 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=0 library=79 hand=5 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=73 library=73 hand=0 graveyard=12 exile=0 battlefield=17 cmdzone=0 mana=1
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Quietus Spike (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Spawning Pool (P/T 0/0, dmg=0) [T]
    - Reliquary Tower (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Foundry of the Consuls (P/T 0/0, dmg=0) [T]
    - Valgavoth, Terror Eater (P/T 9/9, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - creature token black goblin rogue Token (P/T 1/1, dmg=0) [T]
    - creature token black goblin rogue Token (P/T 1/1, dmg=0) [T]
    - creature token black goblin rogue Token (P/T 1/1, dmg=0) [T]
    - Oath of the Grey Host (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2896] activated_ability_resolved seat=3 source=Spawning Pool target=seat0
[2897] pay_mana seat=3 source=Spawning Pool amount=2 target=seat0
[2898] activate_ability seat=3 source=Spawning Pool target=seat0
[2899] stack_push seat=3 source=Spawning Pool target=seat0
[2900] priority_pass seat=1 source= target=seat0
[2901] stack_resolve seat=3 source=Spawning Pool target=seat0
[2902] parsed_effect_residual seat=3 source=Spawning Pool target=seat0
[2903] activated_ability_resolved seat=3 source=Spawning Pool target=seat0
[2904] phase_step seat=3 source= target=seat0
[2905] declare_attackers seat=3 source= target=seat0
[2906] blockers seat=1 source= target=seat0
[2907] damage seat=3 source=Valgavoth, Terror Eater amount=9 target=seat1
[2908] life_change seat=3 source=Valgavoth, Terror Eater amount=9 target=seat0
[2909] damage seat=3 source=creature token black goblin rogue Token amount=1 target=seat1
[2910] damage seat=3 source=creature token black goblin rogue Token amount=1 target=seat1
[2911] damage seat=3 source=creature token black goblin rogue Token amount=1 target=seat1
[2912] sba_704_5a seat=1 source= amount=-9
[2913] sba_cycle_complete seat=-1 source=
[2914] seat_eliminated seat=1 source= amount=18
[2915] game_end seat=3 source=
```

</details>

#### Violation 14

- **Game**: 1 (seed 22346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 59, Phase=combat Step=end_of_combat
- **Commanders**: Liara of the Flaming Fist, Hylda of the Icy Crown, Rakdos, Patron of Chaos, Valgavoth, Terror Eater
- **Message**: ResourceConservation: seat 3 ManaPool=1 but typed Mana.Total()=11 — desync

<details>
<summary>Game State</summary>

```
Turn 59, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 2916 events
  Seat 0 [LOST]: life=-1 library=78 hand=5 graveyard=11 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-9 library=73 hand=1 graveyard=8 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=0 library=79 hand=5 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=73 library=73 hand=0 graveyard=12 exile=0 battlefield=17 cmdzone=0 mana=1
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Quietus Spike (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Spawning Pool (P/T 0/0, dmg=0) [T]
    - Reliquary Tower (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Foundry of the Consuls (P/T 0/0, dmg=0) [T]
    - Valgavoth, Terror Eater (P/T 9/9, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - creature token black goblin rogue Token (P/T 1/1, dmg=0) [T]
    - creature token black goblin rogue Token (P/T 1/1, dmg=0) [T]
    - creature token black goblin rogue Token (P/T 1/1, dmg=0) [T]
    - Oath of the Grey Host (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2896] activated_ability_resolved seat=3 source=Spawning Pool target=seat0
[2897] pay_mana seat=3 source=Spawning Pool amount=2 target=seat0
[2898] activate_ability seat=3 source=Spawning Pool target=seat0
[2899] stack_push seat=3 source=Spawning Pool target=seat0
[2900] priority_pass seat=1 source= target=seat0
[2901] stack_resolve seat=3 source=Spawning Pool target=seat0
[2902] parsed_effect_residual seat=3 source=Spawning Pool target=seat0
[2903] activated_ability_resolved seat=3 source=Spawning Pool target=seat0
[2904] phase_step seat=3 source= target=seat0
[2905] declare_attackers seat=3 source= target=seat0
[2906] blockers seat=1 source= target=seat0
[2907] damage seat=3 source=Valgavoth, Terror Eater amount=9 target=seat1
[2908] life_change seat=3 source=Valgavoth, Terror Eater amount=9 target=seat0
[2909] damage seat=3 source=creature token black goblin rogue Token amount=1 target=seat1
[2910] damage seat=3 source=creature token black goblin rogue Token amount=1 target=seat1
[2911] damage seat=3 source=creature token black goblin rogue Token amount=1 target=seat1
[2912] sba_704_5a seat=1 source= amount=-9
[2913] sba_cycle_complete seat=-1 source=
[2914] seat_eliminated seat=1 source= amount=18
[2915] game_end seat=3 source=
```

</details>

#### Violation 15

- **Game**: 0 (seed 12346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 49, Phase=combat Step=end_of_combat
- **Commanders**: Nicanzil, Current Conductor, Sosuke, Son of Seshiro, Baldin, Century Herdmaster, Kami of the Crescent Moon
- **Message**: ResourceConservation: seat 0 ManaPool=1 but typed Mana.Total()=10 — desync

<details>
<summary>Game State</summary>

```
Turn 49, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 1957 events
  Seat 0 [WON]: life=31 library=71 hand=7 graveyard=5 exile=0 battlefield=17 cmdzone=0 mana=1
    - Forest (P/T 0/0, dmg=0) [T]
    - Secluded Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Loch Larent (P/T 0/0, dmg=0) [T]
    - Rayne, Academy Chancellor (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Diary of Dreams (P/T 0/0, dmg=0) [T]
    - Willbender (P/T 1/2, dmg=0) [T]
    - Nicanzil, Current Conductor (P/T 2/3, dmg=0) [T]
    - Sharktocrab (P/T 4/4, dmg=0) [T]
    - Echoes of Eternity (P/T 0/0, dmg=0)
    - Dimir Informant (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Throne of the High City (P/T 0/0, dmg=0) [T]
    - Griffin Canyon (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-7 library=83 hand=2 graveyard=3 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=0 library=80 hand=2 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-2 library=73 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1937] activated_ability_resolved seat=0 source=Diary of Dreams target=seat0
[1938] pay_mana seat=0 source=Sharktocrab amount=4 target=seat0
[1939] activate_ability seat=0 source=Sharktocrab target=seat0
[1940] stack_push seat=0 source=Sharktocrab target=seat0
[1941] priority_pass seat=3 source= target=seat0
[1942] stack_resolve seat=0 source=Sharktocrab target=seat0
[1943] modification_effect seat=0 source=Sharktocrab target=seat0
[1944] activated_ability_resolved seat=0 source=Sharktocrab target=seat0
[1945] phase_step seat=0 source= target=seat0
[1946] declare_attackers seat=0 source= target=seat0
[1947] blockers seat=3 source= target=seat0
[1948] damage seat=0 source=Rayne, Academy Chancellor amount=1 target=seat3
[1949] damage seat=0 source=Willbender amount=1 target=seat3
[1950] damage seat=0 source=Nicanzil, Current Conductor amount=2 target=seat3
[1951] damage seat=0 source=Sharktocrab amount=4 target=seat3
[1952] damage seat=0 source=Dimir Informant amount=1 target=seat3
[1953] sba_704_5a seat=3 source= amount=-2
[1954] sba_cycle_complete seat=-1 source=
[1955] seat_eliminated seat=3 source= amount=12
[1956] game_end seat=0 source=
```

</details>

#### Violation 16

- **Game**: 0 (seed 12346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 49, Phase=combat Step=end_of_combat
- **Commanders**: Nicanzil, Current Conductor, Sosuke, Son of Seshiro, Baldin, Century Herdmaster, Kami of the Crescent Moon
- **Message**: ResourceConservation: seat 0 ManaPool=1 but typed Mana.Total()=10 — desync

<details>
<summary>Game State</summary>

```
Turn 49, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 1957 events
  Seat 0 [WON]: life=31 library=71 hand=7 graveyard=5 exile=0 battlefield=17 cmdzone=0 mana=1
    - Forest (P/T 0/0, dmg=0) [T]
    - Secluded Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Loch Larent (P/T 0/0, dmg=0) [T]
    - Rayne, Academy Chancellor (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Diary of Dreams (P/T 0/0, dmg=0) [T]
    - Willbender (P/T 1/2, dmg=0) [T]
    - Nicanzil, Current Conductor (P/T 2/3, dmg=0) [T]
    - Sharktocrab (P/T 4/4, dmg=0) [T]
    - Echoes of Eternity (P/T 0/0, dmg=0)
    - Dimir Informant (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Throne of the High City (P/T 0/0, dmg=0) [T]
    - Griffin Canyon (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-7 library=83 hand=2 graveyard=3 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=0 library=80 hand=2 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-2 library=73 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1937] activated_ability_resolved seat=0 source=Diary of Dreams target=seat0
[1938] pay_mana seat=0 source=Sharktocrab amount=4 target=seat0
[1939] activate_ability seat=0 source=Sharktocrab target=seat0
[1940] stack_push seat=0 source=Sharktocrab target=seat0
[1941] priority_pass seat=3 source= target=seat0
[1942] stack_resolve seat=0 source=Sharktocrab target=seat0
[1943] modification_effect seat=0 source=Sharktocrab target=seat0
[1944] activated_ability_resolved seat=0 source=Sharktocrab target=seat0
[1945] phase_step seat=0 source= target=seat0
[1946] declare_attackers seat=0 source= target=seat0
[1947] blockers seat=3 source= target=seat0
[1948] damage seat=0 source=Rayne, Academy Chancellor amount=1 target=seat3
[1949] damage seat=0 source=Willbender amount=1 target=seat3
[1950] damage seat=0 source=Nicanzil, Current Conductor amount=2 target=seat3
[1951] damage seat=0 source=Sharktocrab amount=4 target=seat3
[1952] damage seat=0 source=Dimir Informant amount=1 target=seat3
[1953] sba_704_5a seat=3 source= amount=-2
[1954] sba_cycle_complete seat=-1 source=
[1955] seat_eliminated seat=3 source= amount=12
[1956] game_end seat=0 source=
```

</details>

#### Violation 17

- **Game**: 16 (seed 172346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 38, Phase=combat Step=end_of_combat
- **Commanders**: Witch-king, Bringer of Ruin, Serah Farron // Crystallized Serah, Jaws, Relentless Predator, Ukkima, Stalking Shadow
- **Message**: ResourceConservation: seat 3 ManaPool=3 but typed Mana.Total()=9 — desync

<details>
<summary>Game State</summary>

```
Turn 38, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1255 events
  Seat 0 [LOST]: life=-1 library=80 hand=7 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-2 library=86 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=0 library=83 hand=5 graveyard=2 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=2 library=72 hand=2 graveyard=7 exile=0 battlefield=17 cmdzone=0 mana=3
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Arcade Cabinet (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Ukkima, Stalking Shadow (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kitchen Imp (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shrike Harpy (P/T 2/2, dmg=0) [T]
    - Jushi Apprentice // Tomoya the Revealer (P/T 1/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ravenous Chupacabra (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Laboratory Maniac (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Blasted Landscape (P/T 0/0, dmg=0) [T]
    - Numai Outcast (P/T 1/1, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1235] pay_life seat=3 source=Numai Outcast amount=5 target=seat0
[1236] activate_ability seat=3 source=Numai Outcast target=seat0
[1237] stack_push seat=3 source=Numai Outcast target=seat0
[1238] priority_pass seat=0 source= target=seat0
[1239] stack_resolve seat=3 source=Numai Outcast target=seat0
[1240] regenerate seat=3 source=Numai Outcast target=seat0
[1241] activated_ability_resolved seat=3 source=Numai Outcast target=seat0
[1242] phase_step seat=3 source= target=seat0
[1243] declare_attackers seat=3 source= target=seat0
[1244] blockers seat=0 source= target=seat0
[1245] damage seat=3 source=Ukkima, Stalking Shadow amount=2 target=seat0
[1246] damage seat=3 source=Kitchen Imp amount=2 target=seat0
[1247] damage seat=3 source=Shrike Harpy amount=2 target=seat0
[1248] damage seat=3 source=Ravenous Chupacabra amount=2 target=seat0
[1249] damage seat=3 source=Laboratory Maniac amount=2 target=seat0
[1250] damage seat=3 source=Numai Outcast amount=1 target=seat0
[1251] sba_704_5a seat=0 source= amount=-1
[1252] sba_cycle_complete seat=-1 source=
[1253] seat_eliminated seat=0 source= amount=8
[1254] game_end seat=3 source=
```

</details>

#### Violation 18

- **Game**: 16 (seed 172346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 38, Phase=combat Step=end_of_combat
- **Commanders**: Witch-king, Bringer of Ruin, Serah Farron // Crystallized Serah, Jaws, Relentless Predator, Ukkima, Stalking Shadow
- **Message**: ResourceConservation: seat 3 ManaPool=3 but typed Mana.Total()=9 — desync

<details>
<summary>Game State</summary>

```
Turn 38, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1255 events
  Seat 0 [LOST]: life=-1 library=80 hand=7 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-2 library=86 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=0 library=83 hand=5 graveyard=2 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=2 library=72 hand=2 graveyard=7 exile=0 battlefield=17 cmdzone=0 mana=3
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Arcade Cabinet (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Ukkima, Stalking Shadow (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kitchen Imp (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shrike Harpy (P/T 2/2, dmg=0) [T]
    - Jushi Apprentice // Tomoya the Revealer (P/T 1/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ravenous Chupacabra (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Laboratory Maniac (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Blasted Landscape (P/T 0/0, dmg=0) [T]
    - Numai Outcast (P/T 1/1, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1235] pay_life seat=3 source=Numai Outcast amount=5 target=seat0
[1236] activate_ability seat=3 source=Numai Outcast target=seat0
[1237] stack_push seat=3 source=Numai Outcast target=seat0
[1238] priority_pass seat=0 source= target=seat0
[1239] stack_resolve seat=3 source=Numai Outcast target=seat0
[1240] regenerate seat=3 source=Numai Outcast target=seat0
[1241] activated_ability_resolved seat=3 source=Numai Outcast target=seat0
[1242] phase_step seat=3 source= target=seat0
[1243] declare_attackers seat=3 source= target=seat0
[1244] blockers seat=0 source= target=seat0
[1245] damage seat=3 source=Ukkima, Stalking Shadow amount=2 target=seat0
[1246] damage seat=3 source=Kitchen Imp amount=2 target=seat0
[1247] damage seat=3 source=Shrike Harpy amount=2 target=seat0
[1248] damage seat=3 source=Ravenous Chupacabra amount=2 target=seat0
[1249] damage seat=3 source=Laboratory Maniac amount=2 target=seat0
[1250] damage seat=3 source=Numai Outcast amount=1 target=seat0
[1251] sba_704_5a seat=0 source= amount=-1
[1252] sba_cycle_complete seat=-1 source=
[1253] seat_eliminated seat=0 source= amount=8
[1254] game_end seat=3 source=
```

</details>

#### Violation 19

- **Game**: 12 (seed 132346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 43, Phase=combat Step=end_of_combat
- **Commanders**: The Crafter, Mikey & Leo, Chaos & Order, Myojin of Night's Reach and Grim Betrayal, Saruman of Many Colors
- **Message**: ResourceConservation: seat 1 ManaPool=5 but typed Mana.Total()=6 — desync

<details>
<summary>Game State</summary>

```
Turn 43, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 1462 events
  Seat 0 [LOST]: life=-4 library=79 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [WON]: life=33 library=73 hand=9 graveyard=6 exile=0 battlefield=12 cmdzone=0 mana=5
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Keys to the House (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Inkshape Demonstrator (P/T 3/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Bramble Creeper (P/T 0/3, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Mikey & Leo, Chaos & Order (P/T 2/2, dmg=0) [T]
    - Inquisitor's Flail (P/T 0/0, dmg=0)
    - Needleshot Gourna (P/T 3/6, dmg=0) [T]
  Seat 2 [LOST]: life=-8 library=83 hand=1 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-4 library=80 hand=1 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1442] add_mana seat=1 source=Forest amount=1 target=seat0
[1443] add_mana seat=1 source=Plains amount=1 target=seat0
[1444] tap seat=1 source=Keys to the House target=seat0
[1445] pay_mana seat=1 source=Keys to the House amount=1 target=seat0
[1446] activate_ability seat=1 source=Keys to the House target=seat0
[1447] stack_push seat=1 source=Keys to the House target=seat0
[1448] priority_pass seat=0 source= target=seat0
[1449] stack_resolve seat=1 source=Keys to the House target=seat0
[1450] tutor seat=1 source=generic_tutor amount=1 target=seat0
[1451] activated_ability_resolved seat=1 source=Keys to the House target=seat0
[1452] phase_step seat=1 source= target=seat0
[1453] declare_attackers seat=1 source= target=seat0
[1454] blockers seat=0 source= target=seat0
[1455] damage seat=1 source=Inkshape Demonstrator amount=3 target=seat0
[1456] damage seat=1 source=Mikey & Leo, Chaos & Order amount=2 target=seat0
[1457] damage seat=1 source=Needleshot Gourna amount=3 target=seat0
[1458] sba_704_5a seat=0 source= amount=-4
[1459] sba_cycle_complete seat=-1 source=
[1460] seat_eliminated seat=0 source= amount=10
[1461] game_end seat=1 source=
```

</details>

#### Violation 20

- **Game**: 12 (seed 132346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 43, Phase=combat Step=end_of_combat
- **Commanders**: The Crafter, Mikey & Leo, Chaos & Order, Myojin of Night's Reach and Grim Betrayal, Saruman of Many Colors
- **Message**: ResourceConservation: seat 1 ManaPool=5 but typed Mana.Total()=6 — desync

<details>
<summary>Game State</summary>

```
Turn 43, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 1462 events
  Seat 0 [LOST]: life=-4 library=79 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [WON]: life=33 library=73 hand=9 graveyard=6 exile=0 battlefield=12 cmdzone=0 mana=5
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Keys to the House (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Inkshape Demonstrator (P/T 3/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Bramble Creeper (P/T 0/3, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Mikey & Leo, Chaos & Order (P/T 2/2, dmg=0) [T]
    - Inquisitor's Flail (P/T 0/0, dmg=0)
    - Needleshot Gourna (P/T 3/6, dmg=0) [T]
  Seat 2 [LOST]: life=-8 library=83 hand=1 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-4 library=80 hand=1 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1442] add_mana seat=1 source=Forest amount=1 target=seat0
[1443] add_mana seat=1 source=Plains amount=1 target=seat0
[1444] tap seat=1 source=Keys to the House target=seat0
[1445] pay_mana seat=1 source=Keys to the House amount=1 target=seat0
[1446] activate_ability seat=1 source=Keys to the House target=seat0
[1447] stack_push seat=1 source=Keys to the House target=seat0
[1448] priority_pass seat=0 source= target=seat0
[1449] stack_resolve seat=1 source=Keys to the House target=seat0
[1450] tutor seat=1 source=generic_tutor amount=1 target=seat0
[1451] activated_ability_resolved seat=1 source=Keys to the House target=seat0
[1452] phase_step seat=1 source= target=seat0
[1453] declare_attackers seat=1 source= target=seat0
[1454] blockers seat=0 source= target=seat0
[1455] damage seat=1 source=Inkshape Demonstrator amount=3 target=seat0
[1456] damage seat=1 source=Mikey & Leo, Chaos & Order amount=2 target=seat0
[1457] damage seat=1 source=Needleshot Gourna amount=3 target=seat0
[1458] sba_704_5a seat=0 source= amount=-4
[1459] sba_cycle_complete seat=-1 source=
[1460] seat_eliminated seat=0 source= amount=10
[1461] game_end seat=1 source=
```

</details>

#### Violation 21

- **Game**: 14 (seed 152346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 42, Phase=combat Step=end_of_combat
- **Commanders**: Jyoti, Moag Ancient, The Master, Multiplied, Zirda, the Dawnwaker, Gorma, the Gullet
- **Message**: ResourceConservation: seat 0 ManaPool=0 but typed Mana.Total()=10 — desync

<details>
<summary>Game State</summary>

```
Turn 42, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 1413 events
  Seat 0 [WON]: life=39 library=78 hand=0 graveyard=2 exile=0 battlefield=19 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Maze of Shadows (P/T 0/0, dmg=0) [T]
    - Aurora Shifter (P/T 1/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Volrath's Gardens (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Jyoti, Moag Ancient (P/T 2/4, dmg=0) [T]
    - Deathrender (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Teferi, Mage of Zhalfir (P/T 3/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Duplicant (P/T 2/4, dmg=0) [T]
    - The Misty Stepper (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Dread Linnorm // Scale Deflection (P/T 7/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sword of Fire and Ice and War and Peace (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=84 hand=3 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-10 library=78 hand=4 graveyard=1 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-13 library=78 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1393] activate_ability seat=0 source=Volrath's Gardens target=seat0
[1394] stack_push seat=0 source=Volrath's Gardens target=seat0
[1395] priority_skipped seat=2 source= target=seat0
[1396] stack_resolve seat=0 source=Volrath's Gardens target=seat0
[1397] gain_life seat=0 source=Volrath's Gardens amount=2 target=seat0
[1398] life_change seat=0 source=Volrath's Gardens amount=2 target=seat0
[1399] activated_ability_resolved seat=0 source=Volrath's Gardens target=seat0
[1400] phase_step seat=0 source= target=seat0
[1401] declare_attackers seat=0 source= target=seat0
[1402] blockers seat=2 source= target=seat0
[1403] damage seat=0 source=Aurora Shifter amount=1 target=seat2
[1404] damage seat=0 source=Jyoti, Moag Ancient amount=2 target=seat2
[1405] damage seat=0 source=Teferi, Mage of Zhalfir amount=3 target=seat2
[1406] damage seat=0 source=Duplicant amount=2 target=seat2
[1407] damage seat=0 source=The Misty Stepper amount=2 target=seat2
[1408] damage seat=0 source=Dread Linnorm // Scale Deflection amount=7 target=seat2
[1409] sba_704_5a seat=2 source= amount=-10
[1410] sba_cycle_complete seat=-1 source=
[1411] seat_eliminated seat=2 source= amount=14
[1412] game_end seat=0 source=
```

</details>

#### Violation 22

- **Game**: 14 (seed 152346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 42, Phase=combat Step=end_of_combat
- **Commanders**: Jyoti, Moag Ancient, The Master, Multiplied, Zirda, the Dawnwaker, Gorma, the Gullet
- **Message**: ResourceConservation: seat 0 ManaPool=0 but typed Mana.Total()=10 — desync

<details>
<summary>Game State</summary>

```
Turn 42, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 1413 events
  Seat 0 [WON]: life=39 library=78 hand=0 graveyard=2 exile=0 battlefield=19 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Maze of Shadows (P/T 0/0, dmg=0) [T]
    - Aurora Shifter (P/T 1/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Volrath's Gardens (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Jyoti, Moag Ancient (P/T 2/4, dmg=0) [T]
    - Deathrender (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Teferi, Mage of Zhalfir (P/T 3/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Duplicant (P/T 2/4, dmg=0) [T]
    - The Misty Stepper (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Dread Linnorm // Scale Deflection (P/T 7/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sword of Fire and Ice and War and Peace (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=84 hand=3 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-10 library=78 hand=4 graveyard=1 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-13 library=78 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1393] activate_ability seat=0 source=Volrath's Gardens target=seat0
[1394] stack_push seat=0 source=Volrath's Gardens target=seat0
[1395] priority_skipped seat=2 source= target=seat0
[1396] stack_resolve seat=0 source=Volrath's Gardens target=seat0
[1397] gain_life seat=0 source=Volrath's Gardens amount=2 target=seat0
[1398] life_change seat=0 source=Volrath's Gardens amount=2 target=seat0
[1399] activated_ability_resolved seat=0 source=Volrath's Gardens target=seat0
[1400] phase_step seat=0 source= target=seat0
[1401] declare_attackers seat=0 source= target=seat0
[1402] blockers seat=2 source= target=seat0
[1403] damage seat=0 source=Aurora Shifter amount=1 target=seat2
[1404] damage seat=0 source=Jyoti, Moag Ancient amount=2 target=seat2
[1405] damage seat=0 source=Teferi, Mage of Zhalfir amount=3 target=seat2
[1406] damage seat=0 source=Duplicant amount=2 target=seat2
[1407] damage seat=0 source=The Misty Stepper amount=2 target=seat2
[1408] damage seat=0 source=Dread Linnorm // Scale Deflection amount=7 target=seat2
[1409] sba_704_5a seat=2 source= amount=-10
[1410] sba_cycle_complete seat=-1 source=
[1411] seat_eliminated seat=2 source= amount=14
[1412] game_end seat=0 source=
```

</details>

#### Violation 23

- **Game**: 18 (seed 192346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 40, Phase=combat Step=end_of_combat
- **Commanders**: Aradesh, the Founder, Reki, the History of Kamigawa, G'raha Tia, Scion Reborn, Fangorn, Tree Shepherd
- **Message**: ResourceConservation: seat 2 ManaPool=1 but typed Mana.Total()=7 — desync

<details>
<summary>Game State</summary>

```
Turn 40, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1898 events
  Seat 0 [LOST]: life=-5 library=81 hand=1 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-11 library=78 hand=4 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=44 library=77 hand=1 graveyard=3 exile=0 battlefield=18 cmdzone=0 mana=1
    - Island (P/T 0/0, dmg=0) [T]
    - Dread of Night (P/T 0/0, dmg=0)
    - Idyllic Grange (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - G'raha Tia, Scion Reborn (P/T 2/3, dmg=0) [T]
    - Wintermoon Mesa (P/T 0/0, dmg=0) [T]
    - Horrible Hordes (P/T 2/2, dmg=0) [T]
    - Echoing Deeps (P/T 0/0, dmg=0) [T]
    - Ring of Three Wishes (P/T 0/0, dmg=0) [T]
    - Defiling Daemogoth (P/T 5/4, dmg=0) [T]
    - Aerie Worshippers (P/T 2/4, dmg=0) [T]
    - Soulcoil Viper (P/T 2/3, dmg=0) [T]
    - Favored of Iroas (P/T 2/2, dmg=0) [T]
    - Surge Mare (P/T 0/5, dmg=0)
    - Erebos's Titan (P/T 5/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Whispergear Sneak (P/T 1/1, dmg=0)
  Seat 3 [LOST]: life=-4 library=84 hand=3 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1878] activate_ability seat=2 source=Soulcoil Viper target=seat0
[1879] stack_push seat=2 source=Soulcoil Viper target=seat0
[1880] priority_pass seat=1 source= target=seat0
[1881] stack_resolve seat=2 source=Soulcoil Viper target=seat0
[1882] reanimate seat=2 source=Soulcoil Viper target=seat0
[1883] activated_ability_resolved seat=2 source=Soulcoil Viper target=seat0
[1884] phase_step seat=2 source= target=seat0
[1885] declare_attackers seat=2 source= target=seat0
[1886] blockers seat=1 source= target=seat0
[1887] damage seat=2 source=G'raha Tia, Scion Reborn amount=2 target=seat1
[1888] life_change seat=2 source=G'raha Tia, Scion Reborn amount=2 target=seat0
[1889] damage seat=2 source=Horrible Hordes amount=2 target=seat1
[1890] damage seat=2 source=Defiling Daemogoth amount=5 target=seat1
[1891] damage seat=2 source=Aerie Worshippers amount=2 target=seat1
[1892] damage seat=2 source=Favored of Iroas amount=2 target=seat1
[1893] damage seat=2 source=Erebos's Titan amount=5 target=seat1
[1894] sba_704_5a seat=1 source= amount=-11
[1895] sba_cycle_complete seat=-1 source=
[1896] seat_eliminated seat=1 source= amount=10
[1897] game_end seat=2 source=
```

</details>

#### Violation 24

- **Game**: 18 (seed 192346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 40, Phase=combat Step=end_of_combat
- **Commanders**: Aradesh, the Founder, Reki, the History of Kamigawa, G'raha Tia, Scion Reborn, Fangorn, Tree Shepherd
- **Message**: ResourceConservation: seat 2 ManaPool=1 but typed Mana.Total()=7 — desync

<details>
<summary>Game State</summary>

```
Turn 40, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1898 events
  Seat 0 [LOST]: life=-5 library=81 hand=1 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-11 library=78 hand=4 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=44 library=77 hand=1 graveyard=3 exile=0 battlefield=18 cmdzone=0 mana=1
    - Island (P/T 0/0, dmg=0) [T]
    - Dread of Night (P/T 0/0, dmg=0)
    - Idyllic Grange (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - G'raha Tia, Scion Reborn (P/T 2/3, dmg=0) [T]
    - Wintermoon Mesa (P/T 0/0, dmg=0) [T]
    - Horrible Hordes (P/T 2/2, dmg=0) [T]
    - Echoing Deeps (P/T 0/0, dmg=0) [T]
    - Ring of Three Wishes (P/T 0/0, dmg=0) [T]
    - Defiling Daemogoth (P/T 5/4, dmg=0) [T]
    - Aerie Worshippers (P/T 2/4, dmg=0) [T]
    - Soulcoil Viper (P/T 2/3, dmg=0) [T]
    - Favored of Iroas (P/T 2/2, dmg=0) [T]
    - Surge Mare (P/T 0/5, dmg=0)
    - Erebos's Titan (P/T 5/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Whispergear Sneak (P/T 1/1, dmg=0)
  Seat 3 [LOST]: life=-4 library=84 hand=3 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1878] activate_ability seat=2 source=Soulcoil Viper target=seat0
[1879] stack_push seat=2 source=Soulcoil Viper target=seat0
[1880] priority_pass seat=1 source= target=seat0
[1881] stack_resolve seat=2 source=Soulcoil Viper target=seat0
[1882] reanimate seat=2 source=Soulcoil Viper target=seat0
[1883] activated_ability_resolved seat=2 source=Soulcoil Viper target=seat0
[1884] phase_step seat=2 source= target=seat0
[1885] declare_attackers seat=2 source= target=seat0
[1886] blockers seat=1 source= target=seat0
[1887] damage seat=2 source=G'raha Tia, Scion Reborn amount=2 target=seat1
[1888] life_change seat=2 source=G'raha Tia, Scion Reborn amount=2 target=seat0
[1889] damage seat=2 source=Horrible Hordes amount=2 target=seat1
[1890] damage seat=2 source=Defiling Daemogoth amount=5 target=seat1
[1891] damage seat=2 source=Aerie Worshippers amount=2 target=seat1
[1892] damage seat=2 source=Favored of Iroas amount=2 target=seat1
[1893] damage seat=2 source=Erebos's Titan amount=5 target=seat1
[1894] sba_704_5a seat=1 source= amount=-11
[1895] sba_cycle_complete seat=-1 source=
[1896] seat_eliminated seat=1 source= amount=10
[1897] game_end seat=2 source=
```

</details>

#### Violation 25

- **Game**: 11 (seed 122346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 24, Phase=main Step=precombat_main
- **Commanders**: Gollum, Obsessed Stalker, Rory Williams, Ozox, the Clattering King, Flamewar, Brash Veteran // Flamewar, Streetwise Operative
- **Message**: ResourceConservation: seat 3 is Lost but has ManaPool=6

<details>
<summary>Game State</summary>

```
Turn 24, Phase=main Step=precombat_main Active=seat0
Stack: 0 items, EventLog: 1524 events
  Seat 0 [alive]: life=40 library=86 hand=3 graveyard=1 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Arc Spitter (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Sarevok's Tome (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Purging Scythe (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Gollum, Obsessed Stalker (P/T 1/1, dmg=0)
  Seat 1 [alive]: life=47 library=84 hand=2 graveyard=3 exile=0 battlefield=9 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Command Mine (P/T 0/0, dmg=0) [T]
    - Rory Williams (P/T 13/13, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sword of Hearth and Home (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Alms Collector (P/T 3/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lu Xun, Scholar General (P/T 1/3, dmg=0)
  Seat 2 [alive]: life=40 library=78 hand=5 graveyard=6 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Festering Goblin (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Dockside Chef (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Ozox, the Clattering King (P/T 3/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amulet of Kroog (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=18 library=0 hand=20 graveyard=71 exile=0 battlefield=0 cmdzone=1 mana=6

```

</details>

<details>
<summary>Recent Events</summary>

```
[1504] stack_push seat=3 source=Cryptex target=seat0
[1505] priority_pass seat=0 source= target=seat0
[1506] priority_pass seat=1 source= target=seat0
[1507] priority_pass seat=2 source= target=seat0
[1508] stack_resolve seat=3 source=Cryptex target=seat0
[1509] surveil seat=3 source=Cryptex amount=3 target=seat0
[1510] draw seat=3 source=Cryptex amount=3 target=seat3
[1511] activated_ability_resolved seat=3 source=Cryptex target=seat0
[1512] activate_ability seat=3 source=Cryptex target=seat0
[1513] stack_push seat=3 source=Cryptex target=seat0
[1514] priority_pass seat=0 source= target=seat0
[1515] priority_pass seat=1 source= target=seat0
[1516] priority_pass seat=2 source= target=seat0
[1517] stack_resolve seat=3 source=Cryptex target=seat0
[1518] surveil seat=3 source=Cryptex amount=3 target=seat0
[1519] draw seat=3 source=Cryptex amount=1 target=seat3
[1520] activated_ability_resolved seat=3 source=Cryptex target=seat0
[1521] sba_704_5b seat=3 source=
[1522] sba_cycle_complete seat=-1 source=
[1523] seat_eliminated seat=3 source= amount=7
```

</details>

#### Violation 26

- **Game**: 11 (seed 122346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 24, Phase=main Step=precombat_main
- **Commanders**: Gollum, Obsessed Stalker, Rory Williams, Ozox, the Clattering King, Flamewar, Brash Veteran // Flamewar, Streetwise Operative
- **Message**: ResourceConservation: seat 3 is Lost but has ManaPool=6

<details>
<summary>Game State</summary>

```
Turn 24, Phase=main Step=precombat_main Active=seat0
Stack: 0 items, EventLog: 1524 events
  Seat 0 [alive]: life=40 library=86 hand=3 graveyard=1 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Arc Spitter (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Sarevok's Tome (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Purging Scythe (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Gollum, Obsessed Stalker (P/T 1/1, dmg=0)
  Seat 1 [alive]: life=47 library=84 hand=2 graveyard=3 exile=0 battlefield=9 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Command Mine (P/T 0/0, dmg=0) [T]
    - Rory Williams (P/T 13/13, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sword of Hearth and Home (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Alms Collector (P/T 3/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lu Xun, Scholar General (P/T 1/3, dmg=0)
  Seat 2 [alive]: life=40 library=78 hand=5 graveyard=6 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Festering Goblin (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Dockside Chef (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Ozox, the Clattering King (P/T 3/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amulet of Kroog (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=18 library=0 hand=20 graveyard=71 exile=0 battlefield=0 cmdzone=1 mana=6

```

</details>

<details>
<summary>Recent Events</summary>

```
[1504] stack_push seat=3 source=Cryptex target=seat0
[1505] priority_pass seat=0 source= target=seat0
[1506] priority_pass seat=1 source= target=seat0
[1507] priority_pass seat=2 source= target=seat0
[1508] stack_resolve seat=3 source=Cryptex target=seat0
[1509] surveil seat=3 source=Cryptex amount=3 target=seat0
[1510] draw seat=3 source=Cryptex amount=3 target=seat3
[1511] activated_ability_resolved seat=3 source=Cryptex target=seat0
[1512] activate_ability seat=3 source=Cryptex target=seat0
[1513] stack_push seat=3 source=Cryptex target=seat0
[1514] priority_pass seat=0 source= target=seat0
[1515] priority_pass seat=1 source= target=seat0
[1516] priority_pass seat=2 source= target=seat0
[1517] stack_resolve seat=3 source=Cryptex target=seat0
[1518] surveil seat=3 source=Cryptex amount=3 target=seat0
[1519] draw seat=3 source=Cryptex amount=1 target=seat3
[1520] activated_ability_resolved seat=3 source=Cryptex target=seat0
[1521] sba_704_5b seat=3 source=
[1522] sba_cycle_complete seat=-1 source=
[1523] seat_eliminated seat=3 source= amount=7
```

</details>

#### Violation 27

- **Game**: 11 (seed 122346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 44, Phase=combat Step=end_of_combat
- **Commanders**: Gollum, Obsessed Stalker, Rory Williams, Ozox, the Clattering King, Flamewar, Brash Veteran // Flamewar, Streetwise Operative
- **Message**: ResourceConservation: seat 2 ManaPool=1 but typed Mana.Total()=9 — desync

<details>
<summary>Game State</summary>

```
Turn 44, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 3147 events
  Seat 0 [LOST]: life=-1 library=82 hand=1 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-8 library=65 hand=6 graveyard=13 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=60 library=32 hand=9 graveyard=31 exile=0 battlefield=30 cmdzone=0 mana=1
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amulet of Kroog (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Anchovy & Banana Pizza (P/T 0/0, dmg=0)
    - Loreseeker's Stone (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Moxite Refinery (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Iron Star (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Palantír of Orthanc (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Graveyard Marshal (P/T 3/2, dmg=0) [T]
    - Sibsig Host (P/T 2/6, dmg=0) [T]
    - Ammit Eternal (P/T 5/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Honed Khopesh (P/T 0/0, dmg=0)
    - Noxious Gearhulk (P/T 5/4, dmg=0) [T]
    - Ozox, the Clattering King (P/T 3/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - creature token zombie Token (P/T 2/2, dmg=0) [T]
    - creature token zombie Token (P/T 2/2, dmg=0) [T]
    - creature token zombie Token (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kami of Jealous Thirst (P/T 1/3, dmg=0)
  Seat 3 [LOST]: life=18 library=0 hand=20 graveyard=71 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3127] phase_step seat=2 source= target=seat0
[3128] declare_attackers seat=2 source= target=seat0
[3129] blockers seat=1 source= target=seat0
[3130] damage seat=1 source=Rory Williams amount=3 target=seat2
[3131] life_change seat=1 source=Rory Williams amount=3 target=seat0
[3132] destroy seat=2 source=creature token zombie Token
[3133] sba_704_5g seat=2 source=creature token zombie Token
[3134] sba_cycle_complete seat=-1 source=
[3135] damage seat=2 source=Graveyard Marshal amount=3 target=seat1
[3136] damage seat=2 source=Sibsig Host amount=2 target=seat1
[3137] damage seat=2 source=Ammit Eternal amount=5 target=seat1
[3138] damage seat=2 source=Noxious Gearhulk amount=5 target=seat1
[3139] damage seat=2 source=Ozox, the Clattering King amount=3 target=seat1
[3140] damage seat=2 source=creature token zombie Token amount=2 target=seat1
[3141] damage seat=2 source=creature token zombie Token amount=2 target=seat1
[3142] damage seat=2 source=creature token zombie Token amount=2 target=seat1
[3143] sba_704_5a seat=1 source= amount=-8
[3144] sba_cycle_complete seat=-1 source=
[3145] seat_eliminated seat=1 source= amount=14
[3146] game_end seat=2 source=
```

</details>

#### Violation 28

- **Game**: 11 (seed 122346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 44, Phase=combat Step=end_of_combat
- **Commanders**: Gollum, Obsessed Stalker, Rory Williams, Ozox, the Clattering King, Flamewar, Brash Veteran // Flamewar, Streetwise Operative
- **Message**: ResourceConservation: seat 2 ManaPool=1 but typed Mana.Total()=9 — desync

<details>
<summary>Game State</summary>

```
Turn 44, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 3147 events
  Seat 0 [LOST]: life=-1 library=82 hand=1 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-8 library=65 hand=6 graveyard=13 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=60 library=32 hand=9 graveyard=31 exile=0 battlefield=30 cmdzone=0 mana=1
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amulet of Kroog (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Anchovy & Banana Pizza (P/T 0/0, dmg=0)
    - Loreseeker's Stone (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Moxite Refinery (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Iron Star (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Palantír of Orthanc (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Graveyard Marshal (P/T 3/2, dmg=0) [T]
    - Sibsig Host (P/T 2/6, dmg=0) [T]
    - Ammit Eternal (P/T 5/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Honed Khopesh (P/T 0/0, dmg=0)
    - Noxious Gearhulk (P/T 5/4, dmg=0) [T]
    - Ozox, the Clattering King (P/T 3/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - creature token zombie Token (P/T 2/2, dmg=0) [T]
    - creature token zombie Token (P/T 2/2, dmg=0) [T]
    - creature token zombie Token (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kami of Jealous Thirst (P/T 1/3, dmg=0)
  Seat 3 [LOST]: life=18 library=0 hand=20 graveyard=71 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3127] phase_step seat=2 source= target=seat0
[3128] declare_attackers seat=2 source= target=seat0
[3129] blockers seat=1 source= target=seat0
[3130] damage seat=1 source=Rory Williams amount=3 target=seat2
[3131] life_change seat=1 source=Rory Williams amount=3 target=seat0
[3132] destroy seat=2 source=creature token zombie Token
[3133] sba_704_5g seat=2 source=creature token zombie Token
[3134] sba_cycle_complete seat=-1 source=
[3135] damage seat=2 source=Graveyard Marshal amount=3 target=seat1
[3136] damage seat=2 source=Sibsig Host amount=2 target=seat1
[3137] damage seat=2 source=Ammit Eternal amount=5 target=seat1
[3138] damage seat=2 source=Noxious Gearhulk amount=5 target=seat1
[3139] damage seat=2 source=Ozox, the Clattering King amount=3 target=seat1
[3140] damage seat=2 source=creature token zombie Token amount=2 target=seat1
[3141] damage seat=2 source=creature token zombie Token amount=2 target=seat1
[3142] damage seat=2 source=creature token zombie Token amount=2 target=seat1
[3143] sba_704_5a seat=1 source= amount=-8
[3144] sba_cycle_complete seat=-1 source=
[3145] seat_eliminated seat=1 source= amount=14
[3146] game_end seat=2 source=
```

</details>

#### Violation 29

- **Game**: 10 (seed 112346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 49, Phase=combat Step=end_of_combat
- **Commanders**: Mathise, Surge Channeler, Codie, Vociferous Codex, Spectacular Spider-Man, The Pride of Hull Clade
- **Message**: ResourceConservation: seat 1 ManaPool=1 but typed Mana.Total()=3 — desync

<details>
<summary>Game State</summary>

```
Turn 49, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 2316 events
  Seat 0 [LOST]: life=-7 library=78 hand=1 graveyard=7 exile=1 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=31 library=77 hand=0 graveyard=3 exile=0 battlefield=20 cmdzone=0 mana=1
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hidden Footblade (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Carnival of Souls (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Codie, Vociferous Codex (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Food Coma (P/T 0/0, dmg=0)
    - Feldon of the Third Path (P/T 2/3, dmg=0) [T]
    - Viashino Spearhunter (P/T 2/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mishra's Helix (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Colossal Rattlewurm (P/T 6/5, dmg=0) [T]
    - Diamond Kaleidoscope (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thraximundar (P/T 6/6, dmg=0) [T]
    - Shadow the Hedgehog (P/T 4/2, dmg=0) [T]
    - Storm the Vault // Vault of Catlacan (P/T 0/0, dmg=0) [T]
    - Master of Cruelties (P/T 1/4, dmg=0)
  Seat 2 [LOST]: life=0 library=76 hand=2 graveyard=10 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=0 library=82 hand=0 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2296] enter_battlefield seat=1 source=Master of Cruelties target=seat0
[2297] pay_mana seat=1 source=Colossal Rattlewurm amount=2 target=seat0
[2298] activate_ability seat=1 source=Colossal Rattlewurm target=seat0
[2299] stack_push seat=1 source=Colossal Rattlewurm target=seat0
[2300] priority_pass seat=2 source= target=seat0
[2301] stack_resolve seat=1 source=Colossal Rattlewurm target=seat0
[2302] tutor seat=1 source=generic_tutor target=seat0
[2303] activated_ability_resolved seat=1 source=Colossal Rattlewurm target=seat0
[2304] phase_step seat=1 source= target=seat0
[2305] declare_attackers seat=1 source= target=seat0
[2306] trigger_fires seat=1 source=Thraximundar target=seat0
[2307] stack_push seat=1 source=Thraximundar target=seat0
[2308] priority_pass seat=2 source= target=seat0
[2309] stack_resolve seat=1 source=Thraximundar target=seat0
[2310] blockers seat=2 source= target=seat0
[2311] damage seat=1 source=Viashino Spearhunter amount=2 target=seat2
[2312] sba_704_5a seat=2 source=
[2313] sba_cycle_complete seat=-1 source=
[2314] seat_eliminated seat=2 source= amount=10
[2315] game_end seat=1 source=
```

</details>

#### Violation 30

- **Game**: 10 (seed 112346, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 49, Phase=combat Step=end_of_combat
- **Commanders**: Mathise, Surge Channeler, Codie, Vociferous Codex, Spectacular Spider-Man, The Pride of Hull Clade
- **Message**: ResourceConservation: seat 1 ManaPool=1 but typed Mana.Total()=3 — desync

<details>
<summary>Game State</summary>

```
Turn 49, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 2316 events
  Seat 0 [LOST]: life=-7 library=78 hand=1 graveyard=7 exile=1 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=31 library=77 hand=0 graveyard=3 exile=0 battlefield=20 cmdzone=0 mana=1
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hidden Footblade (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Carnival of Souls (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Codie, Vociferous Codex (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Food Coma (P/T 0/0, dmg=0)
    - Feldon of the Third Path (P/T 2/3, dmg=0) [T]
    - Viashino Spearhunter (P/T 2/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mishra's Helix (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Colossal Rattlewurm (P/T 6/5, dmg=0) [T]
    - Diamond Kaleidoscope (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thraximundar (P/T 6/6, dmg=0) [T]
    - Shadow the Hedgehog (P/T 4/2, dmg=0) [T]
    - Storm the Vault // Vault of Catlacan (P/T 0/0, dmg=0) [T]
    - Master of Cruelties (P/T 1/4, dmg=0)
  Seat 2 [LOST]: life=0 library=76 hand=2 graveyard=10 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=0 library=82 hand=0 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2296] enter_battlefield seat=1 source=Master of Cruelties target=seat0
[2297] pay_mana seat=1 source=Colossal Rattlewurm amount=2 target=seat0
[2298] activate_ability seat=1 source=Colossal Rattlewurm target=seat0
[2299] stack_push seat=1 source=Colossal Rattlewurm target=seat0
[2300] priority_pass seat=2 source= target=seat0
[2301] stack_resolve seat=1 source=Colossal Rattlewurm target=seat0
[2302] tutor seat=1 source=generic_tutor target=seat0
[2303] activated_ability_resolved seat=1 source=Colossal Rattlewurm target=seat0
[2304] phase_step seat=1 source= target=seat0
[2305] declare_attackers seat=1 source= target=seat0
[2306] trigger_fires seat=1 source=Thraximundar target=seat0
[2307] stack_push seat=1 source=Thraximundar target=seat0
[2308] priority_pass seat=2 source= target=seat0
[2309] stack_resolve seat=1 source=Thraximundar target=seat0
[2310] blockers seat=2 source= target=seat0
[2311] damage seat=1 source=Viashino Spearhunter amount=2 target=seat2
[2312] sba_704_5a seat=2 source=
[2313] sba_cycle_complete seat=-1 source=
[2314] seat_eliminated seat=2 source= amount=10
[2315] game_end seat=1 source=
```

</details>

*... and 1008 more violations not shown.*

## Invariant Violations (Nightmare Boards)

| Invariant | Count |
|-----------|-------|
| TriggerCompleteness | 40 |
| ReplacementCompleteness | 9 |

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Littjara Mirrorlake | 3 | 0 | 1.00 |
| 2 | March of the Returned | 3 | 0 | 1.00 |
| 3 | Eternal Taskmaster | 5 | 0 | 1.00 |
| 4 | Aura Mutation | 3 | 0 | 1.00 |
| 5 | Acererak the Archlich | 4 | 0 | 1.00 |
| 6 | Ithilien Kingfisher | 5 | 0 | 1.00 |
| 7 | Take the Bait | 3 | 0 | 1.00 |
| 8 | Rammas Echor, Ancient Shield | 3 | 0 | 1.00 |
| 9 | Demonic Covenant | 8 | 0 | 1.00 |
| 10 | Pious Kitsune | 3 | 0 | 1.00 |

## Verdict: ISSUES FOUND

**1087 total issues** across 1000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 1038 invariant violations in chaos games
- 0 crashes in nightmare boards
- 49 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
