# Fuzz Report

Generated: 2026-04-16T21:23:02-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Games | 20 |
| Seed | 42 |
| Seats | 4 |
| Duration | 97ms |
| Throughput | 206 games/sec |

## Results

| Metric | Count |
|--------|-------|
| Crashes | 0 |
| Games with violations | 20 |
| Total violations | 626 |

### Violations by Invariant

| Invariant | Count |
|-----------|-------|
| StackIntegrity | 2 |
| ZoneConservation | 624 |

### Violation Details (first 50)

#### Violation 1

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 22, Phase=ending Step=cleanup
- **Events**: 475
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 475 events
  Seat 0 [alive]: life=40 library=86 hand=1 graveyard=4 exile=0 battlefield=11 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=38 library=86 hand=5 graveyard=4 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=66 hand=6 graveyard=3 exile=0 battlefield=4 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[455] blockers seat=1 source= target=seat0
[456] damage seat=0 source=Tovolar, Dire Overlord // Tovolar, the Midnight Scourge amount=3 target=seat1
[457] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat1
[458] damage seat=0 source=creature green wolf Token amount=2 target=seat1
[459] damage seat=0 source=creature green wolf Token amount=2 target=seat1
[460] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat1
[461] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat1
[462] damage seat=1 source=Lord of the Nazgûl amount=4 target=seat0
[463] sba_704_5a seat=1 source=
[464] destroy seat=0 source=Tovolar, Dire Overlord // Tovolar, the Midnight Scourge
[465] sba_704_5g seat=0 source=Tovolar, Dire Overlord // Tovolar, the Midnight Scourge
[466] zone_change seat=0 source=Tovolar, Dire Overlord // Tovolar, the Midnight Scourge
[467] destroy seat=1 source=Lord of the Nazgûl
[468] sba_704_5g seat=1 source=Lord of the Nazgûl
[469] zone_change seat=1 source=Lord of the Nazgûl
[470] sba_704_6d seat=1 source=Lord of the Nazgûl
[471] sba_cycle_complete seat=-1 source=
[472] seat_eliminated seat=1 source= amount=6
[473] phase_step seat=0 source= target=seat0
[474] state seat=0 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 22, Phase=ending Step=cleanup
- **Events**: 475
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 475 events
  Seat 0 [alive]: life=40 library=86 hand=1 graveyard=4 exile=0 battlefield=11 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=38 library=86 hand=5 graveyard=4 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=66 hand=6 graveyard=3 exile=0 battlefield=4 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[455] blockers seat=1 source= target=seat0
[456] damage seat=0 source=Tovolar, Dire Overlord // Tovolar, the Midnight Scourge amount=3 target=seat1
[457] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat1
[458] damage seat=0 source=creature green wolf Token amount=2 target=seat1
[459] damage seat=0 source=creature green wolf Token amount=2 target=seat1
[460] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat1
[461] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat1
[462] damage seat=1 source=Lord of the Nazgûl amount=4 target=seat0
[463] sba_704_5a seat=1 source=
[464] destroy seat=0 source=Tovolar, Dire Overlord // Tovolar, the Midnight Scourge
[465] sba_704_5g seat=0 source=Tovolar, Dire Overlord // Tovolar, the Midnight Scourge
[466] zone_change seat=0 source=Tovolar, Dire Overlord // Tovolar, the Midnight Scourge
[467] destroy seat=1 source=Lord of the Nazgûl
[468] sba_704_5g seat=1 source=Lord of the Nazgûl
[469] zone_change seat=1 source=Lord of the Nazgûl
[470] sba_704_6d seat=1 source=Lord of the Nazgûl
[471] sba_cycle_complete seat=-1 source=
[472] seat_eliminated seat=1 source= amount=6
[473] phase_step seat=0 source= target=seat0
[474] state seat=0 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 23, Phase=ending Step=cleanup
- **Events**: 490
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 490 events
  Seat 0 [alive]: life=40 library=86 hand=1 graveyard=4 exile=0 battlefield=11 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=38 library=85 hand=5 graveyard=5 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=66 hand=6 graveyard=3 exile=0 battlefield=4 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[470] sba_704_6d seat=1 source=Lord of the Nazgûl
[471] sba_cycle_complete seat=-1 source=
[472] seat_eliminated seat=1 source= amount=6
[473] phase_step seat=0 source= target=seat0
[474] state seat=0 source= target=seat0
[475] turn_start seat=2 source= target=seat0
[476] untap_done seat=2 source=Sol Ring target=seat0
[477] untap_done seat=2 source=Forest target=seat0
[478] untap_done seat=2 source=Swamp target=seat0
[479] draw seat=2 source=Harrow amount=1 target=seat0
[480] pay_mana seat=2 source=Harrow amount=3 target=seat0
[481] cast seat=2 source=Harrow amount=3 target=seat0
[482] stack_push seat=2 source=Harrow target=seat0
[483] priority_pass seat=3 source= target=seat0
[484] priority_pass seat=0 source= target=seat0
[485] stack_resolve seat=2 source=Harrow target=seat0
[486] resolve seat=2 source=Harrow target=seat0
[487] phase_step seat=2 source= target=seat0
[488] phase_step seat=2 source= target=seat0
[489] state seat=2 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 23, Phase=ending Step=cleanup
- **Events**: 490
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 490 events
  Seat 0 [alive]: life=40 library=86 hand=1 graveyard=4 exile=0 battlefield=11 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=38 library=85 hand=5 graveyard=5 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=66 hand=6 graveyard=3 exile=0 battlefield=4 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[470] sba_704_6d seat=1 source=Lord of the Nazgûl
[471] sba_cycle_complete seat=-1 source=
[472] seat_eliminated seat=1 source= amount=6
[473] phase_step seat=0 source= target=seat0
[474] state seat=0 source= target=seat0
[475] turn_start seat=2 source= target=seat0
[476] untap_done seat=2 source=Sol Ring target=seat0
[477] untap_done seat=2 source=Forest target=seat0
[478] untap_done seat=2 source=Swamp target=seat0
[479] draw seat=2 source=Harrow amount=1 target=seat0
[480] pay_mana seat=2 source=Harrow amount=3 target=seat0
[481] cast seat=2 source=Harrow amount=3 target=seat0
[482] stack_push seat=2 source=Harrow target=seat0
[483] priority_pass seat=3 source= target=seat0
[484] priority_pass seat=0 source= target=seat0
[485] stack_resolve seat=2 source=Harrow target=seat0
[486] resolve seat=2 source=Harrow target=seat0
[487] phase_step seat=2 source= target=seat0
[488] phase_step seat=2 source= target=seat0
[489] state seat=2 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 24, Phase=ending Step=cleanup
- **Events**: 513
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 24, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 513 events
  Seat 0 [alive]: life=40 library=86 hand=1 graveyard=4 exile=0 battlefield=11 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=35 library=85 hand=5 graveyard=5 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=65 hand=6 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[493] untap_done seat=3 source=Tranquil Expanse target=seat0
[494] draw seat=3 source=Thunderfoot Baloth amount=1 target=seat0
[495] pay_mana seat=3 source=Farhaven Elf amount=3 target=seat0
[496] cast seat=3 source=Farhaven Elf amount=3 target=seat0
[497] stack_push seat=3 source=Farhaven Elf target=seat0
[498] priority_pass seat=0 source= target=seat0
[499] priority_pass seat=2 source= target=seat0
[500] stack_resolve seat=3 source=Farhaven Elf target=seat0
[501] enter_battlefield seat=3 source=Farhaven Elf target=seat0
[502] stack_push seat=3 source=Farhaven Elf target=seat0
[503] priority_pass seat=0 source= target=seat0
[504] priority_pass seat=2 source= target=seat0
[505] stack_resolve seat=3 source=Farhaven Elf target=seat0
[506] tutor seat=3 source=Farhaven Elf target=seat0
[507] phase_step seat=3 source= target=seat0
[508] attackers seat=3 source= target=seat0
[509] blockers seat=2 source= target=seat0
[510] damage seat=3 source=Boromir, Warden of the Tower amount=3 target=seat2
[511] phase_step seat=3 source= target=seat0
[512] state seat=3 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 24, Phase=ending Step=cleanup
- **Events**: 513
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 24, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 513 events
  Seat 0 [alive]: life=40 library=86 hand=1 graveyard=4 exile=0 battlefield=11 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=35 library=85 hand=5 graveyard=5 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=65 hand=6 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[493] untap_done seat=3 source=Tranquil Expanse target=seat0
[494] draw seat=3 source=Thunderfoot Baloth amount=1 target=seat0
[495] pay_mana seat=3 source=Farhaven Elf amount=3 target=seat0
[496] cast seat=3 source=Farhaven Elf amount=3 target=seat0
[497] stack_push seat=3 source=Farhaven Elf target=seat0
[498] priority_pass seat=0 source= target=seat0
[499] priority_pass seat=2 source= target=seat0
[500] stack_resolve seat=3 source=Farhaven Elf target=seat0
[501] enter_battlefield seat=3 source=Farhaven Elf target=seat0
[502] stack_push seat=3 source=Farhaven Elf target=seat0
[503] priority_pass seat=0 source= target=seat0
[504] priority_pass seat=2 source= target=seat0
[505] stack_resolve seat=3 source=Farhaven Elf target=seat0
[506] tutor seat=3 source=Farhaven Elf target=seat0
[507] phase_step seat=3 source= target=seat0
[508] attackers seat=3 source= target=seat0
[509] blockers seat=2 source= target=seat0
[510] damage seat=3 source=Boromir, Warden of the Tower amount=3 target=seat2
[511] phase_step seat=3 source= target=seat0
[512] state seat=3 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 25, Phase=ending Step=cleanup
- **Events**: 565
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 25, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 565 events
  Seat 0 [alive]: life=40 library=85 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=18 library=85 hand=5 graveyard=5 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=65 hand=6 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[545] draw seat=0 source=Weaver of Blossoms // Blossom-Clad Werewolf amount=1 target=seat0
[546] play_land seat=0 source=Weaver of Blossoms // Blossom-Clad Werewolf target=seat0
[547] pay_mana seat=0 source=Rhonas's Monument amount=3 target=seat0
[548] cast seat=0 source=Rhonas's Monument amount=3 target=seat0
[549] stack_push seat=0 source=Rhonas's Monument target=seat0
[550] priority_pass seat=2 source= target=seat0
[551] priority_pass seat=3 source= target=seat0
[552] stack_resolve seat=0 source=Rhonas's Monument target=seat0
[553] enter_battlefield seat=0 source=Rhonas's Monument target=seat0
[554] phase_step seat=0 source= target=seat0
[555] attackers seat=0 source= target=seat0
[556] blockers seat=2 source= target=seat0
[557] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat2
[558] damage seat=0 source=creature green wolf Token amount=2 target=seat2
[559] damage seat=0 source=creature green wolf Token amount=2 target=seat2
[560] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[561] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat2
[562] phase_step seat=0 source= target=seat0
[563] pool_drain seat=0 source= amount=4 target=seat0
[564] state seat=0 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 25, Phase=ending Step=cleanup
- **Events**: 565
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 25, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 565 events
  Seat 0 [alive]: life=40 library=85 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=18 library=85 hand=5 graveyard=5 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=65 hand=6 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[545] draw seat=0 source=Weaver of Blossoms // Blossom-Clad Werewolf amount=1 target=seat0
[546] play_land seat=0 source=Weaver of Blossoms // Blossom-Clad Werewolf target=seat0
[547] pay_mana seat=0 source=Rhonas's Monument amount=3 target=seat0
[548] cast seat=0 source=Rhonas's Monument amount=3 target=seat0
[549] stack_push seat=0 source=Rhonas's Monument target=seat0
[550] priority_pass seat=2 source= target=seat0
[551] priority_pass seat=3 source= target=seat0
[552] stack_resolve seat=0 source=Rhonas's Monument target=seat0
[553] enter_battlefield seat=0 source=Rhonas's Monument target=seat0
[554] phase_step seat=0 source= target=seat0
[555] attackers seat=0 source= target=seat0
[556] blockers seat=2 source= target=seat0
[557] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat2
[558] damage seat=0 source=creature green wolf Token amount=2 target=seat2
[559] damage seat=0 source=creature green wolf Token amount=2 target=seat2
[560] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[561] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat2
[562] phase_step seat=0 source= target=seat0
[563] pool_drain seat=0 source= amount=4 target=seat0
[564] state seat=0 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 26, Phase=ending Step=cleanup
- **Events**: 580
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 26, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 580 events
  Seat 0 [alive]: life=40 library=85 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=18 library=84 hand=5 graveyard=6 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=65 hand=6 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[560] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[561] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat2
[562] phase_step seat=0 source= target=seat0
[563] pool_drain seat=0 source= amount=4 target=seat0
[564] state seat=0 source= target=seat0
[565] turn_start seat=2 source= target=seat0
[566] untap_done seat=2 source=Sol Ring target=seat0
[567] untap_done seat=2 source=Forest target=seat0
[568] untap_done seat=2 source=Swamp target=seat0
[569] draw seat=2 source=Territory Culler amount=1 target=seat0
[570] pay_mana seat=2 source=Victimize amount=3 target=seat0
[571] cast seat=2 source=Victimize amount=3 target=seat0
[572] stack_push seat=2 source=Victimize target=seat0
[573] priority_pass seat=3 source= target=seat0
[574] priority_pass seat=0 source= target=seat0
[575] stack_resolve seat=2 source=Victimize target=seat0
[576] resolve seat=2 source=Victimize target=seat0
[577] phase_step seat=2 source= target=seat0
[578] phase_step seat=2 source= target=seat0
[579] state seat=2 source= target=seat0
```

</details>

#### Violation 10

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 26, Phase=ending Step=cleanup
- **Events**: 580
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 26, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 580 events
  Seat 0 [alive]: life=40 library=85 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=18 library=84 hand=5 graveyard=6 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=65 hand=6 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[560] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[561] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat2
[562] phase_step seat=0 source= target=seat0
[563] pool_drain seat=0 source= amount=4 target=seat0
[564] state seat=0 source= target=seat0
[565] turn_start seat=2 source= target=seat0
[566] untap_done seat=2 source=Sol Ring target=seat0
[567] untap_done seat=2 source=Forest target=seat0
[568] untap_done seat=2 source=Swamp target=seat0
[569] draw seat=2 source=Territory Culler amount=1 target=seat0
[570] pay_mana seat=2 source=Victimize amount=3 target=seat0
[571] cast seat=2 source=Victimize amount=3 target=seat0
[572] stack_push seat=2 source=Victimize target=seat0
[573] priority_pass seat=3 source= target=seat0
[574] priority_pass seat=0 source= target=seat0
[575] stack_resolve seat=2 source=Victimize target=seat0
[576] resolve seat=2 source=Victimize target=seat0
[577] phase_step seat=2 source= target=seat0
[578] phase_step seat=2 source= target=seat0
[579] state seat=2 source= target=seat0
```

</details>

#### Violation 11

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 27, Phase=ending Step=cleanup
- **Events**: 599
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 27, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 599 events
  Seat 0 [alive]: life=40 library=85 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=14 library=84 hand=5 graveyard=6 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=64 hand=6 graveyard=3 exile=0 battlefield=6 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[579] state seat=2 source= target=seat0
[580] turn_start seat=3 source= target=seat0
[581] untap_done seat=3 source=Arid Archway target=seat0
[582] untap_done seat=3 source=Fortified Village target=seat0
[583] untap_done seat=3 source=Tranquil Expanse target=seat0
[584] draw seat=3 source=Champion of Lambholt amount=1 target=seat0
[585] pay_mana seat=3 source=Champion of Lambholt amount=3 target=seat0
[586] cast seat=3 source=Champion of Lambholt amount=3 target=seat0
[587] stack_push seat=3 source=Champion of Lambholt target=seat0
[588] priority_pass seat=0 source= target=seat0
[589] priority_pass seat=2 source= target=seat0
[590] stack_resolve seat=3 source=Champion of Lambholt target=seat0
[591] enter_battlefield seat=3 source=Champion of Lambholt target=seat0
[592] phase_step seat=3 source= target=seat0
[593] attackers seat=3 source= target=seat0
[594] blockers seat=2 source= target=seat0
[595] damage seat=3 source=Boromir, Warden of the Tower amount=3 target=seat2
[596] damage seat=3 source=Farhaven Elf amount=1 target=seat2
[597] phase_step seat=3 source= target=seat0
[598] state seat=3 source= target=seat0
```

</details>

#### Violation 12

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 27, Phase=ending Step=cleanup
- **Events**: 599
- **Message**: zone conservation violated: 4 real cards disappeared (expected 378, found 374)

<details>
<summary>Game State</summary>

```
Turn 27, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 599 events
  Seat 0 [alive]: life=40 library=85 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=14 library=84 hand=5 graveyard=6 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=64 hand=6 graveyard=3 exile=0 battlefield=6 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[579] state seat=2 source= target=seat0
[580] turn_start seat=3 source= target=seat0
[581] untap_done seat=3 source=Arid Archway target=seat0
[582] untap_done seat=3 source=Fortified Village target=seat0
[583] untap_done seat=3 source=Tranquil Expanse target=seat0
[584] draw seat=3 source=Champion of Lambholt amount=1 target=seat0
[585] pay_mana seat=3 source=Champion of Lambholt amount=3 target=seat0
[586] cast seat=3 source=Champion of Lambholt amount=3 target=seat0
[587] stack_push seat=3 source=Champion of Lambholt target=seat0
[588] priority_pass seat=0 source= target=seat0
[589] priority_pass seat=2 source= target=seat0
[590] stack_resolve seat=3 source=Champion of Lambholt target=seat0
[591] enter_battlefield seat=3 source=Champion of Lambholt target=seat0
[592] phase_step seat=3 source= target=seat0
[593] attackers seat=3 source= target=seat0
[594] blockers seat=2 source= target=seat0
[595] damage seat=3 source=Boromir, Warden of the Tower amount=3 target=seat2
[596] damage seat=3 source=Farhaven Elf amount=1 target=seat2
[597] phase_step seat=3 source= target=seat0
[598] state seat=3 source= target=seat0
```

</details>

#### Violation 13

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 28, Phase=ending Step=cleanup
- **Events**: 648
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 648 events
  Seat 0 [alive]: life=40 library=84 hand=0 graveyard=4 exile=0 battlefield=14 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=40 library=64 hand=6 graveyard=3 exile=0 battlefield=6 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[628] priority_pass seat=2 source= target=seat0
[629] priority_pass seat=3 source= target=seat0
[630] stack_resolve seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha target=seat0
[631] unknown_effect seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha target=seat0
[632] draw seat=0 source=Command Tower amount=1 target=seat0
[633] play_land seat=0 source=Command Tower target=seat0
[634] phase_step seat=0 source= target=seat0
[635] attackers seat=0 source= target=seat0
[636] blockers seat=2 source= target=seat0
[637] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat2
[638] damage seat=0 source=creature green wolf Token amount=2 target=seat2
[639] damage seat=0 source=creature green wolf Token amount=2 target=seat2
[640] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[641] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat2
[642] sba_704_5a seat=2 source= amount=-3
[643] sba_cycle_complete seat=-1 source=
[644] seat_eliminated seat=2 source= amount=3
[645] phase_step seat=0 source= target=seat0
[646] pool_drain seat=0 source= amount=8 target=seat0
[647] state seat=0 source= target=seat0
```

</details>

#### Violation 14

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 28, Phase=ending Step=cleanup
- **Events**: 648
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 648 events
  Seat 0 [alive]: life=40 library=84 hand=0 graveyard=4 exile=0 battlefield=14 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=40 library=64 hand=6 graveyard=3 exile=0 battlefield=6 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[628] priority_pass seat=2 source= target=seat0
[629] priority_pass seat=3 source= target=seat0
[630] stack_resolve seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha target=seat0
[631] unknown_effect seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha target=seat0
[632] draw seat=0 source=Command Tower amount=1 target=seat0
[633] play_land seat=0 source=Command Tower target=seat0
[634] phase_step seat=0 source= target=seat0
[635] attackers seat=0 source= target=seat0
[636] blockers seat=2 source= target=seat0
[637] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat2
[638] damage seat=0 source=creature green wolf Token amount=2 target=seat2
[639] damage seat=0 source=creature green wolf Token amount=2 target=seat2
[640] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[641] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat2
[642] sba_704_5a seat=2 source= amount=-3
[643] sba_cycle_complete seat=-1 source=
[644] seat_eliminated seat=2 source= amount=3
[645] phase_step seat=0 source= target=seat0
[646] pool_drain seat=0 source= amount=8 target=seat0
[647] state seat=0 source= target=seat0
```

</details>

#### Violation 15

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 29, Phase=ending Step=cleanup
- **Events**: 672
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 672 events
  Seat 0 [alive]: life=32 library=84 hand=0 graveyard=4 exile=0 battlefield=14 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=40 library=63 hand=5 graveyard=3 exile=0 battlefield=8 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[652] untap_done seat=3 source=Farhaven Elf target=seat0
[653] draw seat=3 source=Blossoming Sands amount=1 target=seat0
[654] play_land seat=3 source=Blossoming Sands target=seat0
[655] pay_mana seat=3 source=Veteran Beastrider amount=4 target=seat0
[656] cast seat=3 source=Veteran Beastrider amount=4 target=seat0
[657] stack_push seat=3 source=Veteran Beastrider target=seat0
[658] priority_pass seat=0 source= target=seat0
[659] stack_resolve seat=3 source=Veteran Beastrider target=seat0
[660] buff seat=0 source=Veteran Beastrider amount=1 target=seat0
[661] buff seat=0 source=Veteran Beastrider amount=1 target=seat0
[662] buff seat=0 source=Veteran Beastrider amount=1 target=seat0
[663] enter_battlefield seat=3 source=Veteran Beastrider target=seat0
[664] phase_step seat=3 source= target=seat0
[665] attackers seat=3 source= target=seat0
[666] blockers seat=0 source= target=seat0
[667] damage seat=3 source=Boromir, Warden of the Tower amount=4 target=seat0
[668] damage seat=3 source=Farhaven Elf amount=2 target=seat0
[669] damage seat=3 source=Champion of Lambholt amount=2 target=seat0
[670] phase_step seat=3 source= target=seat0
[671] state seat=3 source= target=seat0
```

</details>

#### Violation 16

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 29, Phase=ending Step=cleanup
- **Events**: 672
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 672 events
  Seat 0 [alive]: life=32 library=84 hand=0 graveyard=4 exile=0 battlefield=14 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=40 library=63 hand=5 graveyard=3 exile=0 battlefield=8 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Boromir, Warden of the Tower (P/T 3/3, dmg=0)
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[652] untap_done seat=3 source=Farhaven Elf target=seat0
[653] draw seat=3 source=Blossoming Sands amount=1 target=seat0
[654] play_land seat=3 source=Blossoming Sands target=seat0
[655] pay_mana seat=3 source=Veteran Beastrider amount=4 target=seat0
[656] cast seat=3 source=Veteran Beastrider amount=4 target=seat0
[657] stack_push seat=3 source=Veteran Beastrider target=seat0
[658] priority_pass seat=0 source= target=seat0
[659] stack_resolve seat=3 source=Veteran Beastrider target=seat0
[660] buff seat=0 source=Veteran Beastrider amount=1 target=seat0
[661] buff seat=0 source=Veteran Beastrider amount=1 target=seat0
[662] buff seat=0 source=Veteran Beastrider amount=1 target=seat0
[663] enter_battlefield seat=3 source=Veteran Beastrider target=seat0
[664] phase_step seat=3 source= target=seat0
[665] attackers seat=3 source= target=seat0
[666] blockers seat=0 source= target=seat0
[667] damage seat=3 source=Boromir, Warden of the Tower amount=4 target=seat0
[668] damage seat=3 source=Farhaven Elf amount=2 target=seat0
[669] damage seat=3 source=Champion of Lambholt amount=2 target=seat0
[670] phase_step seat=3 source= target=seat0
[671] state seat=3 source= target=seat0
```

</details>

#### Violation 17

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 30, Phase=ending Step=cleanup
- **Events**: 735
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 30, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 735 events
  Seat 0 [alive]: life=32 library=83 hand=0 graveyard=5 exile=0 battlefield=14 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=26 library=63 hand=5 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[715] blockers seat=3 source= target=seat0
[716] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=3 target=seat3
[717] damage seat=0 source=creature green wolf Token amount=2 target=seat3
[718] damage seat=0 source=creature green wolf Token amount=2 target=seat3
[719] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat3
[720] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat3
[721] damage seat=0 source=Craterhoof Behemoth amount=5 target=seat3
[722] damage seat=3 source=Boromir, Warden of the Tower amount=3 target=seat0
[723] damage seat=3 source=Veteran Beastrider amount=3 target=seat0
[724] destroy seat=0 source=creature green wolf Token
[725] sba_704_5g seat=0 source=creature green wolf Token
[726] zone_change seat=0 source=creature green wolf Token
[727] destroy seat=3 source=Boromir, Warden of the Tower
[728] sba_704_5g seat=3 source=Boromir, Warden of the Tower
[729] zone_change seat=3 source=Boromir, Warden of the Tower
[730] sba_cycle_complete seat=-1 source=
[731] phase_step seat=0 source= target=seat0
[732] damage_wears_off seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=3 target=seat0
[733] damage_wears_off seat=3 source=Veteran Beastrider amount=2 target=seat0
[734] state seat=0 source= target=seat0
```

</details>

#### Violation 18

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 30, Phase=ending Step=cleanup
- **Events**: 735
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 30, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 735 events
  Seat 0 [alive]: life=32 library=83 hand=0 graveyard=5 exile=0 battlefield=14 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=26 library=63 hand=5 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[715] blockers seat=3 source= target=seat0
[716] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=3 target=seat3
[717] damage seat=0 source=creature green wolf Token amount=2 target=seat3
[718] damage seat=0 source=creature green wolf Token amount=2 target=seat3
[719] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat3
[720] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat3
[721] damage seat=0 source=Craterhoof Behemoth amount=5 target=seat3
[722] damage seat=3 source=Boromir, Warden of the Tower amount=3 target=seat0
[723] damage seat=3 source=Veteran Beastrider amount=3 target=seat0
[724] destroy seat=0 source=creature green wolf Token
[725] sba_704_5g seat=0 source=creature green wolf Token
[726] zone_change seat=0 source=creature green wolf Token
[727] destroy seat=3 source=Boromir, Warden of the Tower
[728] sba_704_5g seat=3 source=Boromir, Warden of the Tower
[729] zone_change seat=3 source=Boromir, Warden of the Tower
[730] sba_cycle_complete seat=-1 source=
[731] phase_step seat=0 source= target=seat0
[732] damage_wears_off seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=3 target=seat0
[733] damage_wears_off seat=3 source=Veteran Beastrider amount=2 target=seat0
[734] state seat=0 source= target=seat0
```

</details>

#### Violation 19

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 31, Phase=ending Step=cleanup
- **Events**: 762
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 762 events
  Seat 0 [alive]: life=27 library=83 hand=0 graveyard=5 exile=0 battlefield=14 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=26 library=62 hand=5 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[742] draw seat=3 source=Ancient Greenwarden amount=1 target=seat0
[743] pay_mana seat=3 source=Queen Allenal of Ruadach amount=3 target=seat0
[744] cast seat=3 source=Queen Allenal of Ruadach amount=3 target=seat0
[745] stack_push seat=3 source=Queen Allenal of Ruadach target=seat0
[746] priority_pass seat=0 source= target=seat0
[747] stack_resolve seat=3 source=Queen Allenal of Ruadach target=seat0
[748] enter_battlefield seat=3 source=Queen Allenal of Ruadach target=seat0
[749] destroy seat=3 source=Queen Allenal of Ruadach
[750] sba_704_5f seat=3 source=Queen Allenal of Ruadach
[751] zone_change seat=3 source=Queen Allenal of Ruadach
[752] sba_cycle_complete seat=-1 source=
[753] phase_step seat=3 source= target=seat0
[754] attackers seat=3 source= target=seat0
[755] blockers seat=0 source= target=seat0
[756] damage seat=3 source=Farhaven Elf amount=1 target=seat0
[757] damage seat=3 source=Champion of Lambholt amount=1 target=seat0
[758] damage seat=3 source=Veteran Beastrider amount=3 target=seat0
[759] phase_step seat=3 source= target=seat0
[760] pool_drain seat=3 source= amount=1 target=seat0
[761] state seat=3 source= target=seat0
```

</details>

#### Violation 20

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 31, Phase=ending Step=cleanup
- **Events**: 762
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 762 events
  Seat 0 [alive]: life=27 library=83 hand=0 graveyard=5 exile=0 battlefield=14 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=26 library=62 hand=5 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[742] draw seat=3 source=Ancient Greenwarden amount=1 target=seat0
[743] pay_mana seat=3 source=Queen Allenal of Ruadach amount=3 target=seat0
[744] cast seat=3 source=Queen Allenal of Ruadach amount=3 target=seat0
[745] stack_push seat=3 source=Queen Allenal of Ruadach target=seat0
[746] priority_pass seat=0 source= target=seat0
[747] stack_resolve seat=3 source=Queen Allenal of Ruadach target=seat0
[748] enter_battlefield seat=3 source=Queen Allenal of Ruadach target=seat0
[749] destroy seat=3 source=Queen Allenal of Ruadach
[750] sba_704_5f seat=3 source=Queen Allenal of Ruadach
[751] zone_change seat=3 source=Queen Allenal of Ruadach
[752] sba_cycle_complete seat=-1 source=
[753] phase_step seat=3 source= target=seat0
[754] attackers seat=3 source= target=seat0
[755] blockers seat=0 source= target=seat0
[756] damage seat=3 source=Farhaven Elf amount=1 target=seat0
[757] damage seat=3 source=Champion of Lambholt amount=1 target=seat0
[758] damage seat=3 source=Veteran Beastrider amount=3 target=seat0
[759] phase_step seat=3 source= target=seat0
[760] pool_drain seat=3 source= amount=1 target=seat0
[761] state seat=3 source= target=seat0
```

</details>

#### Violation 21

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 32, Phase=ending Step=cleanup
- **Events**: 810
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 810 events
  Seat 0 [alive]: life=27 library=82 hand=0 graveyard=5 exile=0 battlefield=15 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
    - Hermit of the Natterknolls // Lone Wolf of the Natterknolls (P/T 2/3, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=6 library=62 hand=5 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[790] stack_resolve seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha target=seat0
[791] unknown_effect seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha target=seat0
[792] draw seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=1 target=seat0
[793] pay_mana seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=3 target=seat0
[794] cast seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=3 target=seat0
[795] stack_push seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls target=seat0
[796] priority_pass seat=3 source= target=seat0
[797] stack_resolve seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls target=seat0
[798] enter_battlefield seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls target=seat0
[799] phase_step seat=0 source= target=seat0
[800] attackers seat=0 source= target=seat0
[801] blockers seat=3 source= target=seat0
[802] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat3
[803] damage seat=0 source=creature green wolf Token amount=2 target=seat3
[804] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat3
[805] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat3
[806] damage seat=0 source=Craterhoof Behemoth amount=5 target=seat3
[807] phase_step seat=0 source= target=seat0
[808] pool_drain seat=0 source= amount=5 target=seat0
[809] state seat=0 source= target=seat0
```

</details>

#### Violation 22

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 32, Phase=ending Step=cleanup
- **Events**: 810
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 810 events
  Seat 0 [alive]: life=27 library=82 hand=0 graveyard=5 exile=0 battlefield=15 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
    - Hermit of the Natterknolls // Lone Wolf of the Natterknolls (P/T 2/3, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=6 library=62 hand=5 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[790] stack_resolve seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha target=seat0
[791] unknown_effect seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha target=seat0
[792] draw seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=1 target=seat0
[793] pay_mana seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=3 target=seat0
[794] cast seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=3 target=seat0
[795] stack_push seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls target=seat0
[796] priority_pass seat=3 source= target=seat0
[797] stack_resolve seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls target=seat0
[798] enter_battlefield seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls target=seat0
[799] phase_step seat=0 source= target=seat0
[800] attackers seat=0 source= target=seat0
[801] blockers seat=3 source= target=seat0
[802] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat3
[803] damage seat=0 source=creature green wolf Token amount=2 target=seat3
[804] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat3
[805] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat3
[806] damage seat=0 source=Craterhoof Behemoth amount=5 target=seat3
[807] phase_step seat=0 source= target=seat0
[808] pool_drain seat=0 source= amount=5 target=seat0
[809] state seat=0 source= target=seat0
```

</details>

#### Violation 23

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 33, Phase=ending Step=cleanup
- **Events**: 839
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 839 events
  Seat 0 [alive]: life=23 library=82 hand=0 graveyard=5 exile=0 battlefield=15 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
    - Hermit of the Natterknolls // Lone Wolf of the Natterknolls (P/T 2/3, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=6 library=61 hand=5 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0) [T]
    - Felidar Retreat (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[819] pay_mana seat=3 source=Felidar Retreat amount=4 target=seat0
[820] cast seat=3 source=Felidar Retreat amount=4 target=seat0
[821] stack_push seat=3 source=Felidar Retreat target=seat0
[822] priority_pass seat=0 source= target=seat0
[823] stack_resolve seat=3 source=Felidar Retreat target=seat0
[824] enter_battlefield seat=3 source=Felidar Retreat target=seat0
[825] phase_step seat=3 source= target=seat0
[826] attackers seat=3 source= target=seat0
[827] blockers seat=0 source= target=seat0
[828] damage seat=3 source=Farhaven Elf amount=1 target=seat0
[829] damage seat=3 source=Champion of Lambholt amount=1 target=seat0
[830] damage seat=3 source=Veteran Beastrider amount=3 target=seat0
[831] damage seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=2 target=seat3
[832] destroy seat=3 source=Farhaven Elf
[833] sba_704_5g seat=3 source=Farhaven Elf
[834] zone_change seat=3 source=Farhaven Elf
[835] sba_cycle_complete seat=-1 source=
[836] phase_step seat=3 source= target=seat0
[837] damage_wears_off seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=1 target=seat0
[838] state seat=3 source= target=seat0
```

</details>

#### Violation 24

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 33, Phase=ending Step=cleanup
- **Events**: 839
- **Message**: zone conservation violated: 7 real cards disappeared (expected 378, found 371)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 839 events
  Seat 0 [alive]: life=23 library=82 hand=0 graveyard=5 exile=0 battlefield=15 cmdzone=0 mana=0
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
    - Hermit of the Natterknolls // Lone Wolf of the Natterknolls (P/T 2/3, dmg=0)
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=6 library=61 hand=5 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Fortified Village (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Champion of Lambholt (P/T 1/1, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Veteran Beastrider (P/T 3/4, dmg=0) [T]
    - Felidar Retreat (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[819] pay_mana seat=3 source=Felidar Retreat amount=4 target=seat0
[820] cast seat=3 source=Felidar Retreat amount=4 target=seat0
[821] stack_push seat=3 source=Felidar Retreat target=seat0
[822] priority_pass seat=0 source= target=seat0
[823] stack_resolve seat=3 source=Felidar Retreat target=seat0
[824] enter_battlefield seat=3 source=Felidar Retreat target=seat0
[825] phase_step seat=3 source= target=seat0
[826] attackers seat=3 source= target=seat0
[827] blockers seat=0 source= target=seat0
[828] damage seat=3 source=Farhaven Elf amount=1 target=seat0
[829] damage seat=3 source=Champion of Lambholt amount=1 target=seat0
[830] damage seat=3 source=Veteran Beastrider amount=3 target=seat0
[831] damage seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=2 target=seat3
[832] destroy seat=3 source=Farhaven Elf
[833] sba_704_5g seat=3 source=Farhaven Elf
[834] zone_change seat=3 source=Farhaven Elf
[835] sba_cycle_complete seat=-1 source=
[836] phase_step seat=3 source= target=seat0
[837] damage_wears_off seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=1 target=seat0
[838] state seat=3 source= target=seat0
```

</details>

#### Violation 25

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 34, Phase=combat Step=end_of_combat
- **Events**: 897
- **Message**: zone conservation violated: 14 real cards disappeared (expected 378, found 364)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 897 events
  Seat 0 [WON]: life=23 library=81 hand=0 graveyard=6 exile=0 battlefield=15 cmdzone=0 mana=7
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
    - Hermit of the Natterknolls // Lone Wolf of the Natterknolls (P/T 2/3, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-16 library=61 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[877] draw seat=0 source=Gamble amount=1 target=seat0
[878] pay_mana seat=0 source=Gamble amount=1 target=seat0
[879] cast seat=0 source=Gamble amount=1 target=seat0
[880] stack_push seat=0 source=Gamble target=seat0
[881] priority_pass seat=3 source= target=seat0
[882] stack_resolve seat=0 source=Gamble target=seat0
[883] resolve seat=0 source=Gamble target=seat0
[884] phase_step seat=0 source= target=seat0
[885] attackers seat=0 source= target=seat0
[886] blockers seat=3 source= target=seat0
[887] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat3
[888] damage seat=0 source=creature green wolf Token amount=2 target=seat3
[889] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat3
[890] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat3
[891] damage seat=0 source=Craterhoof Behemoth amount=5 target=seat3
[892] damage seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=2 target=seat3
[893] sba_704_5a seat=3 source= amount=-16
[894] sba_cycle_complete seat=-1 source=
[895] seat_eliminated seat=3 source= amount=7
[896] game_end seat=0 source=
```

</details>

#### Violation 26

- **Game**: 7 (seed 7043)
- **Invariant**: ZoneConservation
- **Turn**: 34, Phase=combat Step=end_of_combat
- **Events**: 897
- **Message**: zone conservation violated: 14 real cards disappeared (expected 378, found 364)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 897 events
  Seat 0 [WON]: life=23 library=81 hand=0 graveyard=6 exile=0 battlefield=15 cmdzone=0 mana=7
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Rockfall Vale (P/T 0/0, dmg=0) [T]
    - Tovolar's Huntmaster // Tovolar's Packleader (P/T 6/6, dmg=0) [T]
    - creature green wolf Token (P/T 2/2, dmg=0) [T]
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Geier Reach Bandit // Vildin-Pack Alpha (P/T 3/2, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Craterhoof Behemoth (P/T 5/5, dmg=0) [T]
    - Hermit of the Natterknolls // Lone Wolf of the Natterknolls (P/T 2/3, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=86 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-3 library=84 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=-16 library=61 hand=5 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[877] draw seat=0 source=Gamble amount=1 target=seat0
[878] pay_mana seat=0 source=Gamble amount=1 target=seat0
[879] cast seat=0 source=Gamble amount=1 target=seat0
[880] stack_push seat=0 source=Gamble target=seat0
[881] priority_pass seat=3 source= target=seat0
[882] stack_resolve seat=0 source=Gamble target=seat0
[883] resolve seat=0 source=Gamble target=seat0
[884] phase_step seat=0 source= target=seat0
[885] attackers seat=0 source= target=seat0
[886] blockers seat=3 source= target=seat0
[887] damage seat=0 source=Tovolar's Huntmaster // Tovolar's Packleader amount=6 target=seat3
[888] damage seat=0 source=creature green wolf Token amount=2 target=seat3
[889] damage seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat3
[890] damage seat=0 source=Geier Reach Bandit // Vildin-Pack Alpha amount=3 target=seat3
[891] damage seat=0 source=Craterhoof Behemoth amount=5 target=seat3
[892] damage seat=0 source=Hermit of the Natterknolls // Lone Wolf of the Natterknolls amount=2 target=seat3
[893] sba_704_5a seat=3 source= amount=-16
[894] sba_cycle_complete seat=-1 source=
[895] seat_eliminated seat=3 source= amount=7
[896] game_end seat=0 source=
```

</details>

#### Violation 27

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 33, Phase=ending Step=cleanup
- **Events**: 706
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 706 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=83 hand=2 graveyard=3 exile=0 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=40 library=63 hand=6 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=1 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[686] untap_done seat=2 source=Dauntless Escort target=seat0
[687] draw seat=2 source=Doubling Season amount=1 target=seat0
[688] pay_mana seat=2 source=Intangible Virtue amount=2 target=seat0
[689] cast seat=2 source=Intangible Virtue amount=2 target=seat0
[690] stack_push seat=2 source=Intangible Virtue target=seat0
[691] priority_pass seat=3 source= target=seat0
[692] priority_pass seat=0 source= target=seat0
[693] priority_pass seat=1 source= target=seat0
[694] stack_resolve seat=2 source=Intangible Virtue target=seat0
[695] enter_battlefield seat=2 source=Intangible Virtue target=seat0
[696] phase_step seat=2 source= target=seat0
[697] attackers seat=2 source= target=seat0
[698] blockers seat=0 source= target=seat0
[699] damage seat=2 source=Dauntless Escort amount=3 target=seat0
[700] sba_704_5a seat=0 source=
[701] sba_cycle_complete seat=-1 source=
[702] seat_eliminated seat=0 source= amount=6
[703] phase_step seat=2 source= target=seat0
[704] pool_drain seat=2 source= amount=1 target=seat0
[705] state seat=2 source= target=seat0
```

</details>

#### Violation 28

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 33, Phase=ending Step=cleanup
- **Events**: 706
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 706 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=83 hand=2 graveyard=3 exile=0 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=40 library=63 hand=6 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=1 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[686] untap_done seat=2 source=Dauntless Escort target=seat0
[687] draw seat=2 source=Doubling Season amount=1 target=seat0
[688] pay_mana seat=2 source=Intangible Virtue amount=2 target=seat0
[689] cast seat=2 source=Intangible Virtue amount=2 target=seat0
[690] stack_push seat=2 source=Intangible Virtue target=seat0
[691] priority_pass seat=3 source= target=seat0
[692] priority_pass seat=0 source= target=seat0
[693] priority_pass seat=1 source= target=seat0
[694] stack_resolve seat=2 source=Intangible Virtue target=seat0
[695] enter_battlefield seat=2 source=Intangible Virtue target=seat0
[696] phase_step seat=2 source= target=seat0
[697] attackers seat=2 source= target=seat0
[698] blockers seat=0 source= target=seat0
[699] damage seat=2 source=Dauntless Escort amount=3 target=seat0
[700] sba_704_5a seat=0 source=
[701] sba_cycle_complete seat=-1 source=
[702] seat_eliminated seat=0 source= amount=6
[703] phase_step seat=2 source= target=seat0
[704] pool_drain seat=2 source= amount=1 target=seat0
[705] state seat=2 source= target=seat0
```

</details>

#### Violation 29

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 34, Phase=ending Step=cleanup
- **Events**: 756
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 756 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=83 hand=2 graveyard=3 exile=0 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=30 library=63 hand=6 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[736] stack_resolve seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[737] unknown_effect seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[738] draw seat=3 source=Scalding Tarn amount=1 target=seat0
[739] play_land seat=3 source=Scalding Tarn target=seat0
[740] pay_mana seat=3 source=Blasphemous Act amount=9 target=seat0
[741] cast seat=3 source=Blasphemous Act amount=9 target=seat0
[742] stack_push seat=3 source=Blasphemous Act target=seat0
[743] priority_pass seat=1 source= target=seat0
[744] priority_pass seat=2 source= target=seat0
[745] stack_resolve seat=3 source=Blasphemous Act target=seat0
[746] resolve seat=3 source=Blasphemous Act target=seat0
[747] phase_step seat=3 source= target=seat0
[748] attackers seat=3 source= target=seat0
[749] blockers seat=2 source= target=seat0
[750] damage seat=3 source=Mayor of Avabruck // Howlpack Alpha amount=1 target=seat2
[751] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[752] damage seat=3 source=Wolfir Silverheart amount=4 target=seat2
[753] damage seat=3 source=Packsong Pup amount=1 target=seat2
[754] phase_step seat=3 source= target=seat0
[755] state seat=3 source= target=seat0
```

</details>

#### Violation 30

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 34, Phase=ending Step=cleanup
- **Events**: 756
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 756 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=83 hand=2 graveyard=3 exile=0 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=30 library=63 hand=6 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[736] stack_resolve seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[737] unknown_effect seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[738] draw seat=3 source=Scalding Tarn amount=1 target=seat0
[739] play_land seat=3 source=Scalding Tarn target=seat0
[740] pay_mana seat=3 source=Blasphemous Act amount=9 target=seat0
[741] cast seat=3 source=Blasphemous Act amount=9 target=seat0
[742] stack_push seat=3 source=Blasphemous Act target=seat0
[743] priority_pass seat=1 source= target=seat0
[744] priority_pass seat=2 source= target=seat0
[745] stack_resolve seat=3 source=Blasphemous Act target=seat0
[746] resolve seat=3 source=Blasphemous Act target=seat0
[747] phase_step seat=3 source= target=seat0
[748] attackers seat=3 source= target=seat0
[749] blockers seat=2 source= target=seat0
[750] damage seat=3 source=Mayor of Avabruck // Howlpack Alpha amount=1 target=seat2
[751] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[752] damage seat=3 source=Wolfir Silverheart amount=4 target=seat2
[753] damage seat=3 source=Packsong Pup amount=1 target=seat2
[754] phase_step seat=3 source= target=seat0
[755] state seat=3 source= target=seat0
```

</details>

#### Violation 31

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 35, Phase=ending Step=cleanup
- **Events**: 788
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 35, Phase=ending Step=cleanup Active=seat1
Stack: 1 items, EventLog: 788 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=82 hand=2 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=16 library=63 hand=6 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[768] commander_cast_from_command_zone seat=1 source=Sin, Spira's Punishment amount=7 target=seat0
[769] stack_push seat=1 source=Sin, Spira's Punishment target=seat0
[770] phase_step seat=1 source= target=seat0
[771] priority_pass seat=2 source= target=seat0
[772] priority_pass seat=3 source= target=seat0
[773] attackers seat=1 source= target=seat0
[774] priority_pass seat=2 source= target=seat0
[775] priority_pass seat=3 source= target=seat0
[776] blockers seat=2 source= target=seat0
[777] priority_pass seat=2 source= target=seat0
[778] priority_pass seat=3 source= target=seat0
[779] damage seat=1 source=Deadeye Navigator amount=5 target=seat2
[780] damage seat=1 source=Grappling Kraken amount=5 target=seat2
[781] damage seat=1 source=Genesis amount=4 target=seat2
[782] priority_pass seat=2 source= target=seat0
[783] priority_pass seat=3 source= target=seat0
[784] phase_step seat=1 source= target=seat0
[785] priority_pass seat=2 source= target=seat0
[786] priority_pass seat=3 source= target=seat0
[787] state seat=1 source= target=seat0
```

</details>

#### Violation 32

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 35, Phase=ending Step=cleanup
- **Events**: 788
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 35, Phase=ending Step=cleanup Active=seat1
Stack: 1 items, EventLog: 788 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=82 hand=2 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=16 library=63 hand=6 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[768] commander_cast_from_command_zone seat=1 source=Sin, Spira's Punishment amount=7 target=seat0
[769] stack_push seat=1 source=Sin, Spira's Punishment target=seat0
[770] phase_step seat=1 source= target=seat0
[771] priority_pass seat=2 source= target=seat0
[772] priority_pass seat=3 source= target=seat0
[773] attackers seat=1 source= target=seat0
[774] priority_pass seat=2 source= target=seat0
[775] priority_pass seat=3 source= target=seat0
[776] blockers seat=2 source= target=seat0
[777] priority_pass seat=2 source= target=seat0
[778] priority_pass seat=3 source= target=seat0
[779] damage seat=1 source=Deadeye Navigator amount=5 target=seat2
[780] damage seat=1 source=Grappling Kraken amount=5 target=seat2
[781] damage seat=1 source=Genesis amount=4 target=seat2
[782] priority_pass seat=2 source= target=seat0
[783] priority_pass seat=3 source= target=seat0
[784] phase_step seat=1 source= target=seat0
[785] priority_pass seat=2 source= target=seat0
[786] priority_pass seat=3 source= target=seat0
[787] state seat=1 source= target=seat0
```

</details>

#### Violation 33

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 36, Phase=ending Step=cleanup
- **Events**: 815
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 36, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 815 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=82 hand=2 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0)
  Seat 2 [alive]: life=16 library=62 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=37 library=83 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[795] cast seat=2 source=Garruk's Uprising amount=3 target=seat0
[796] stack_push seat=2 source=Garruk's Uprising target=seat0
[797] priority_pass seat=3 source= target=seat0
[798] priority_pass seat=1 source= target=seat0
[799] stack_resolve seat=2 source=Garruk's Uprising target=seat0
[800] enter_battlefield seat=2 source=Garruk's Uprising target=seat0
[801] stack_push seat=2 source=Garruk's Uprising target=seat0
[802] priority_pass seat=3 source= target=seat0
[803] priority_pass seat=1 source= target=seat0
[804] stack_resolve seat=2 source=Garruk's Uprising target=seat0
[805] priority_pass seat=2 source= target=seat0
[806] priority_pass seat=3 source= target=seat0
[807] stack_resolve seat=1 source=Sin, Spira's Punishment target=seat0
[808] enter_battlefield seat=1 source=Sin, Spira's Punishment target=seat0
[809] phase_step seat=2 source= target=seat0
[810] attackers seat=2 source= target=seat0
[811] blockers seat=3 source= target=seat0
[812] damage seat=2 source=Dauntless Escort amount=3 target=seat3
[813] phase_step seat=2 source= target=seat0
[814] state seat=2 source= target=seat0
```

</details>

#### Violation 34

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 36, Phase=ending Step=cleanup
- **Events**: 815
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 36, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 815 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=82 hand=2 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0)
  Seat 2 [alive]: life=16 library=62 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=37 library=83 hand=0 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[795] cast seat=2 source=Garruk's Uprising amount=3 target=seat0
[796] stack_push seat=2 source=Garruk's Uprising target=seat0
[797] priority_pass seat=3 source= target=seat0
[798] priority_pass seat=1 source= target=seat0
[799] stack_resolve seat=2 source=Garruk's Uprising target=seat0
[800] enter_battlefield seat=2 source=Garruk's Uprising target=seat0
[801] stack_push seat=2 source=Garruk's Uprising target=seat0
[802] priority_pass seat=3 source= target=seat0
[803] priority_pass seat=1 source= target=seat0
[804] stack_resolve seat=2 source=Garruk's Uprising target=seat0
[805] priority_pass seat=2 source= target=seat0
[806] priority_pass seat=3 source= target=seat0
[807] stack_resolve seat=1 source=Sin, Spira's Punishment target=seat0
[808] enter_battlefield seat=1 source=Sin, Spira's Punishment target=seat0
[809] phase_step seat=2 source= target=seat0
[810] attackers seat=2 source= target=seat0
[811] blockers seat=3 source= target=seat0
[812] damage seat=2 source=Dauntless Escort amount=3 target=seat3
[813] phase_step seat=2 source= target=seat0
[814] state seat=2 source= target=seat0
```

</details>

#### Violation 35

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 37, Phase=ending Step=cleanup
- **Events**: 867
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 37, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 867 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=82 hand=2 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0)
  Seat 2 [alive]: life=6 library=62 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=37 library=82 hand=0 graveyard=5 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[847] stack_resolve seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[848] unknown_effect seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[849] draw seat=3 source=Heroic Intervention amount=1 target=seat0
[850] pay_mana seat=3 source=Heroic Intervention amount=2 target=seat0
[851] cast seat=3 source=Heroic Intervention amount=2 target=seat0
[852] stack_push seat=3 source=Heroic Intervention target=seat0
[853] priority_pass seat=1 source= target=seat0
[854] priority_pass seat=2 source= target=seat0
[855] stack_resolve seat=3 source=Heroic Intervention target=seat0
[856] resolve seat=3 source=Heroic Intervention target=seat0
[857] phase_step seat=3 source= target=seat0
[858] attackers seat=3 source= target=seat0
[859] blockers seat=2 source= target=seat0
[860] damage seat=3 source=Mayor of Avabruck // Howlpack Alpha amount=1 target=seat2
[861] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[862] damage seat=3 source=Wolfir Silverheart amount=4 target=seat2
[863] damage seat=3 source=Packsong Pup amount=1 target=seat2
[864] phase_step seat=3 source= target=seat0
[865] pool_drain seat=3 source= amount=7 target=seat0
[866] state seat=3 source= target=seat0
```

</details>

#### Violation 36

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 37, Phase=ending Step=cleanup
- **Events**: 867
- **Message**: zone conservation violated: 6 real cards disappeared (expected 378, found 372)

<details>
<summary>Game State</summary>

```
Turn 37, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 867 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=82 hand=2 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0)
  Seat 2 [alive]: life=6 library=62 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Commander's Sphere (P/T 0/0, dmg=0)
    - Dauntless Escort (P/T 3/3, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=37 library=82 hand=0 graveyard=5 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[847] stack_resolve seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[848] unknown_effect seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[849] draw seat=3 source=Heroic Intervention amount=1 target=seat0
[850] pay_mana seat=3 source=Heroic Intervention amount=2 target=seat0
[851] cast seat=3 source=Heroic Intervention amount=2 target=seat0
[852] stack_push seat=3 source=Heroic Intervention target=seat0
[853] priority_pass seat=1 source= target=seat0
[854] priority_pass seat=2 source= target=seat0
[855] stack_resolve seat=3 source=Heroic Intervention target=seat0
[856] resolve seat=3 source=Heroic Intervention target=seat0
[857] phase_step seat=3 source= target=seat0
[858] attackers seat=3 source= target=seat0
[859] blockers seat=2 source= target=seat0
[860] damage seat=3 source=Mayor of Avabruck // Howlpack Alpha amount=1 target=seat2
[861] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[862] damage seat=3 source=Wolfir Silverheart amount=4 target=seat2
[863] damage seat=3 source=Packsong Pup amount=1 target=seat2
[864] phase_step seat=3 source= target=seat0
[865] pool_drain seat=3 source= amount=7 target=seat0
[866] state seat=3 source= target=seat0
```

</details>

#### Violation 37

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 38, Phase=ending Step=cleanup
- **Events**: 910
- **Message**: zone conservation violated: 13 real cards disappeared (expected 378, found 365)

<details>
<summary>Game State</summary>

```
Turn 38, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 910 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=81 hand=1 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Mutable Explorer (P/T 1/1, dmg=0)
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=37 library=82 hand=0 graveyard=5 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[890] priority_pass seat=3 source= target=seat0
[891] stack_resolve seat=1 source=Mutable Explorer target=seat0
[892] enter_battlefield seat=1 source=Mutable Explorer target=seat0
[893] stack_push seat=1 source=Mutable Explorer target=seat0
[894] priority_pass seat=2 source= target=seat0
[895] priority_pass seat=3 source= target=seat0
[896] stack_resolve seat=1 source=Mutable Explorer target=seat0
[897] modification_effect seat=1 source=Mutable Explorer target=seat0
[898] phase_step seat=1 source= target=seat0
[899] attackers seat=1 source= target=seat0
[900] blockers seat=2 source= target=seat0
[901] damage seat=1 source=Deadeye Navigator amount=5 target=seat2
[902] damage seat=1 source=Grappling Kraken amount=5 target=seat2
[903] damage seat=1 source=Genesis amount=4 target=seat2
[904] damage seat=1 source=Sin, Spira's Punishment amount=7 target=seat2
[905] sba_704_5a seat=2 source= amount=-15
[906] sba_cycle_complete seat=-1 source=
[907] seat_eliminated seat=2 source= amount=7
[908] phase_step seat=1 source= target=seat0
[909] state seat=1 source= target=seat0
```

</details>

#### Violation 38

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 38, Phase=ending Step=cleanup
- **Events**: 910
- **Message**: zone conservation violated: 13 real cards disappeared (expected 378, found 365)

<details>
<summary>Game State</summary>

```
Turn 38, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 910 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=81 hand=1 graveyard=4 exile=0 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedron Crab (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Mutable Explorer (P/T 1/1, dmg=0)
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=37 library=82 hand=0 graveyard=5 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Mayor of Avabruck // Howlpack Alpha (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[890] priority_pass seat=3 source= target=seat0
[891] stack_resolve seat=1 source=Mutable Explorer target=seat0
[892] enter_battlefield seat=1 source=Mutable Explorer target=seat0
[893] stack_push seat=1 source=Mutable Explorer target=seat0
[894] priority_pass seat=2 source= target=seat0
[895] priority_pass seat=3 source= target=seat0
[896] stack_resolve seat=1 source=Mutable Explorer target=seat0
[897] modification_effect seat=1 source=Mutable Explorer target=seat0
[898] phase_step seat=1 source= target=seat0
[899] attackers seat=1 source= target=seat0
[900] blockers seat=2 source= target=seat0
[901] damage seat=1 source=Deadeye Navigator amount=5 target=seat2
[902] damage seat=1 source=Grappling Kraken amount=5 target=seat2
[903] damage seat=1 source=Genesis amount=4 target=seat2
[904] damage seat=1 source=Sin, Spira's Punishment amount=7 target=seat2
[905] sba_704_5a seat=2 source= amount=-15
[906] sba_cycle_complete seat=-1 source=
[907] seat_eliminated seat=2 source= amount=7
[908] phase_step seat=1 source= target=seat0
[909] state seat=1 source= target=seat0
```

</details>

#### Violation 39

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 39, Phase=ending Step=cleanup
- **Events**: 968
- **Message**: zone conservation violated: 13 real cards disappeared (expected 378, found 365)

<details>
<summary>Game State</summary>

```
Turn 39, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 968 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=35 library=81 hand=1 graveyard=6 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=37 library=81 hand=0 graveyard=6 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]
    - Oakshade Stalker // Moonlit Ambusher (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[948] attackers seat=3 source= target=seat0
[949] blockers seat=1 source= target=seat0
[950] damage seat=3 source=Mayor of Avabruck // Howlpack Alpha amount=1 target=seat1
[951] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=2 target=seat1
[952] damage seat=3 source=Wolfir Silverheart amount=4 target=seat1
[953] damage seat=3 source=Packsong Pup amount=1 target=seat1
[954] damage seat=1 source=Mutable Explorer amount=1 target=seat3
[955] destroy seat=1 source=Hedron Crab
[956] sba_704_5g seat=1 source=Hedron Crab
[957] zone_change seat=1 source=Hedron Crab
[958] destroy seat=1 source=Mutable Explorer
[959] sba_704_5g seat=1 source=Mutable Explorer
[960] zone_change seat=1 source=Mutable Explorer
[961] destroy seat=3 source=Mayor of Avabruck // Howlpack Alpha
[962] sba_704_5g seat=3 source=Mayor of Avabruck // Howlpack Alpha
[963] zone_change seat=3 source=Mayor of Avabruck // Howlpack Alpha
[964] sba_cycle_complete seat=-1 source=
[965] phase_step seat=3 source= target=seat0
[966] pool_drain seat=3 source= amount=6 target=seat0
[967] state seat=3 source= target=seat0
```

</details>

#### Violation 40

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 39, Phase=ending Step=cleanup
- **Events**: 968
- **Message**: zone conservation violated: 13 real cards disappeared (expected 378, found 365)

<details>
<summary>Game State</summary>

```
Turn 39, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 968 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=35 library=81 hand=1 graveyard=6 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=37 library=81 hand=0 graveyard=6 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]
    - Oakshade Stalker // Moonlit Ambusher (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[948] attackers seat=3 source= target=seat0
[949] blockers seat=1 source= target=seat0
[950] damage seat=3 source=Mayor of Avabruck // Howlpack Alpha amount=1 target=seat1
[951] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=2 target=seat1
[952] damage seat=3 source=Wolfir Silverheart amount=4 target=seat1
[953] damage seat=3 source=Packsong Pup amount=1 target=seat1
[954] damage seat=1 source=Mutable Explorer amount=1 target=seat3
[955] destroy seat=1 source=Hedron Crab
[956] sba_704_5g seat=1 source=Hedron Crab
[957] zone_change seat=1 source=Hedron Crab
[958] destroy seat=1 source=Mutable Explorer
[959] sba_704_5g seat=1 source=Mutable Explorer
[960] zone_change seat=1 source=Mutable Explorer
[961] destroy seat=3 source=Mayor of Avabruck // Howlpack Alpha
[962] sba_704_5g seat=3 source=Mayor of Avabruck // Howlpack Alpha
[963] zone_change seat=3 source=Mayor of Avabruck // Howlpack Alpha
[964] sba_cycle_complete seat=-1 source=
[965] phase_step seat=3 source= target=seat0
[966] pool_drain seat=3 source= amount=6 target=seat0
[967] state seat=3 source= target=seat0
```

</details>

#### Violation 41

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 40, Phase=ending Step=cleanup
- **Events**: 1004
- **Message**: zone conservation violated: 13 real cards disappeared (expected 378, found 365)

<details>
<summary>Game State</summary>

```
Turn 40, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1004 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=35 library=80 hand=0 graveyard=7 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Watery Grave (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=21 library=81 hand=0 graveyard=7 exile=0 battlefield=12 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[984] stack_push seat=1 source=Gifts Ungiven target=seat0
[985] priority_pass seat=3 source= target=seat0
[986] stack_resolve seat=1 source=Gifts Ungiven target=seat0
[987] resolve seat=1 source=Gifts Ungiven target=seat0
[988] phase_step seat=1 source= target=seat0
[989] attackers seat=1 source= target=seat0
[990] blockers seat=3 source= target=seat0
[991] damage seat=1 source=Deadeye Navigator amount=3 target=seat3
[992] damage seat=1 source=Grappling Kraken amount=5 target=seat3
[993] damage seat=1 source=Genesis amount=4 target=seat3
[994] damage seat=1 source=Sin, Spira's Punishment amount=7 target=seat3
[995] damage seat=3 source=Oakshade Stalker // Moonlit Ambusher amount=3 target=seat1
[996] destroy seat=3 source=Oakshade Stalker // Moonlit Ambusher
[997] sba_704_5g seat=3 source=Oakshade Stalker // Moonlit Ambusher
[998] zone_change seat=3 source=Oakshade Stalker // Moonlit Ambusher
[999] sba_cycle_complete seat=-1 source=
[1000] phase_step seat=1 source= target=seat0
[1001] pool_drain seat=1 source= amount=4 target=seat0
[1002] damage_wears_off seat=1 source=Deadeye Navigator amount=3 target=seat0
[1003] state seat=1 source= target=seat0
```

</details>

#### Violation 42

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 40, Phase=ending Step=cleanup
- **Events**: 1004
- **Message**: zone conservation violated: 13 real cards disappeared (expected 378, found 365)

<details>
<summary>Game State</summary>

```
Turn 40, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1004 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=35 library=80 hand=0 graveyard=7 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Watery Grave (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=21 library=81 hand=0 graveyard=7 exile=0 battlefield=12 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[984] stack_push seat=1 source=Gifts Ungiven target=seat0
[985] priority_pass seat=3 source= target=seat0
[986] stack_resolve seat=1 source=Gifts Ungiven target=seat0
[987] resolve seat=1 source=Gifts Ungiven target=seat0
[988] phase_step seat=1 source= target=seat0
[989] attackers seat=1 source= target=seat0
[990] blockers seat=3 source= target=seat0
[991] damage seat=1 source=Deadeye Navigator amount=3 target=seat3
[992] damage seat=1 source=Grappling Kraken amount=5 target=seat3
[993] damage seat=1 source=Genesis amount=4 target=seat3
[994] damage seat=1 source=Sin, Spira's Punishment amount=7 target=seat3
[995] damage seat=3 source=Oakshade Stalker // Moonlit Ambusher amount=3 target=seat1
[996] destroy seat=3 source=Oakshade Stalker // Moonlit Ambusher
[997] sba_704_5g seat=3 source=Oakshade Stalker // Moonlit Ambusher
[998] zone_change seat=3 source=Oakshade Stalker // Moonlit Ambusher
[999] sba_cycle_complete seat=-1 source=
[1000] phase_step seat=1 source= target=seat0
[1001] pool_drain seat=1 source= amount=4 target=seat0
[1002] damage_wears_off seat=1 source=Deadeye Navigator amount=3 target=seat0
[1003] state seat=1 source= target=seat0
```

</details>

#### Violation 43

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 41, Phase=ending Step=cleanup
- **Events**: 1036
- **Message**: zone conservation violated: 13 real cards disappeared (expected 378, found 365)

<details>
<summary>Game State</summary>

```
Turn 41, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1036 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=26 library=80 hand=0 graveyard=7 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Watery Grave (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=21 library=80 hand=0 graveyard=7 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1016] untap_done seat=3 source=Scalding Tarn target=seat0
[1017] stack_push seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1018] priority_pass seat=1 source= target=seat0
[1019] stack_resolve seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1020] unknown_effect seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1021] stack_push seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1022] priority_pass seat=1 source= target=seat0
[1023] stack_resolve seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1024] unknown_effect seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1025] draw seat=3 source=Kessig Wolf Run amount=1 target=seat0
[1026] play_land seat=3 source=Kessig Wolf Run target=seat0
[1027] phase_step seat=3 source= target=seat0
[1028] attackers seat=3 source= target=seat0
[1029] blockers seat=1 source= target=seat0
[1030] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat1
[1031] damage seat=3 source=Wolfir Silverheart amount=4 target=seat1
[1032] damage seat=3 source=Packsong Pup amount=1 target=seat1
[1033] phase_step seat=3 source= target=seat0
[1034] pool_drain seat=3 source= amount=10 target=seat0
[1035] state seat=3 source= target=seat0
```

</details>

#### Violation 44

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 41, Phase=ending Step=cleanup
- **Events**: 1036
- **Message**: zone conservation violated: 13 real cards disappeared (expected 378, found 365)

<details>
<summary>Game State</summary>

```
Turn 41, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1036 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=26 library=80 hand=0 graveyard=7 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Watery Grave (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=21 library=80 hand=0 graveyard=7 exile=0 battlefield=13 cmdzone=0 mana=0
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Stomping Ground (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arcane Signet (P/T 0/0, dmg=0) [T]
    - Arid Mesa (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wolfir Silverheart (P/T 4/4, dmg=0) [T]
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Sheltered Thicket (P/T 0/0, dmg=0) [T]
    - Packsong Pup (P/T 1/1, dmg=0) [T]
    - Scalding Tarn (P/T 0/0, dmg=0) [T]
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1016] untap_done seat=3 source=Scalding Tarn target=seat0
[1017] stack_push seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1018] priority_pass seat=1 source= target=seat0
[1019] stack_resolve seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1020] unknown_effect seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1021] stack_push seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1022] priority_pass seat=1 source= target=seat0
[1023] stack_resolve seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1024] unknown_effect seat=0 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha target=seat0
[1025] draw seat=3 source=Kessig Wolf Run amount=1 target=seat0
[1026] play_land seat=3 source=Kessig Wolf Run target=seat0
[1027] phase_step seat=3 source= target=seat0
[1028] attackers seat=3 source= target=seat0
[1029] blockers seat=1 source= target=seat0
[1030] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat1
[1031] damage seat=3 source=Wolfir Silverheart amount=4 target=seat1
[1032] damage seat=3 source=Packsong Pup amount=1 target=seat1
[1033] phase_step seat=3 source= target=seat0
[1034] pool_drain seat=3 source= amount=10 target=seat0
[1035] state seat=3 source= target=seat0
```

</details>

#### Violation 45

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 42, Phase=combat Step=end_of_combat
- **Events**: 1062
- **Message**: zone conservation violated: 26 real cards disappeared (expected 378, found 352)

<details>
<summary>Game State</summary>

```
Turn 42, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 1062 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=26 library=79 hand=0 graveyard=7 exile=0 battlefield=13 cmdzone=0 mana=9
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Watery Grave (P/T 0/0, dmg=0) [T]
    - Opulent Palace (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=0 library=80 hand=0 graveyard=7 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1042] untap_done seat=1 source=Underground River target=seat0
[1043] untap_done seat=1 source=Deadeye Navigator target=seat0
[1044] untap_done seat=1 source=Grappling Kraken target=seat0
[1045] untap_done seat=1 source=Genesis target=seat0
[1046] untap_done seat=1 source=Llanowar Wastes target=seat0
[1047] untap_done seat=1 source=Sin, Spira's Punishment target=seat0
[1048] untap_done seat=1 source=Watery Grave target=seat0
[1049] draw seat=1 source=Opulent Palace amount=1 target=seat0
[1050] play_land seat=1 source=Opulent Palace target=seat0
[1051] phase_step seat=1 source= target=seat0
[1052] attackers seat=1 source= target=seat0
[1053] blockers seat=3 source= target=seat0
[1054] damage seat=1 source=Deadeye Navigator amount=5 target=seat3
[1055] damage seat=1 source=Grappling Kraken amount=5 target=seat3
[1056] damage seat=1 source=Genesis amount=4 target=seat3
[1057] damage seat=1 source=Sin, Spira's Punishment amount=7 target=seat3
[1058] sba_704_5a seat=3 source=
[1059] sba_cycle_complete seat=-1 source=
[1060] seat_eliminated seat=3 source= amount=13
[1061] game_end seat=1 source=
```

</details>

#### Violation 46

- **Game**: 0 (seed 43)
- **Invariant**: ZoneConservation
- **Turn**: 42, Phase=combat Step=end_of_combat
- **Events**: 1062
- **Message**: zone conservation violated: 26 real cards disappeared (expected 378, found 352)

<details>
<summary>Game State</summary>

```
Turn 42, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 1062 events
  Seat 0 [LOST]: life=0 library=83 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=26 library=79 hand=0 graveyard=7 exile=0 battlefield=13 cmdzone=0 mana=9
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hinterland Harbor (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Watery Grave (P/T 0/0, dmg=0) [T]
    - Opulent Palace (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-15 library=62 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=0 library=80 hand=0 graveyard=7 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1042] untap_done seat=1 source=Underground River target=seat0
[1043] untap_done seat=1 source=Deadeye Navigator target=seat0
[1044] untap_done seat=1 source=Grappling Kraken target=seat0
[1045] untap_done seat=1 source=Genesis target=seat0
[1046] untap_done seat=1 source=Llanowar Wastes target=seat0
[1047] untap_done seat=1 source=Sin, Spira's Punishment target=seat0
[1048] untap_done seat=1 source=Watery Grave target=seat0
[1049] draw seat=1 source=Opulent Palace amount=1 target=seat0
[1050] play_land seat=1 source=Opulent Palace target=seat0
[1051] phase_step seat=1 source= target=seat0
[1052] attackers seat=1 source= target=seat0
[1053] blockers seat=3 source= target=seat0
[1054] damage seat=1 source=Deadeye Navigator amount=5 target=seat3
[1055] damage seat=1 source=Grappling Kraken amount=5 target=seat3
[1056] damage seat=1 source=Genesis amount=4 target=seat3
[1057] damage seat=1 source=Sin, Spira's Punishment amount=7 target=seat3
[1058] sba_704_5a seat=3 source=
[1059] sba_cycle_complete seat=-1 source=
[1060] seat_eliminated seat=3 source= amount=13
[1061] game_end seat=1 source=
```

</details>

#### Violation 47

- **Game**: 9 (seed 9043)
- **Invariant**: ZoneConservation
- **Turn**: 28, Phase=ending Step=cleanup
- **Events**: 611
- **Message**: zone conservation violated: 3 real cards disappeared (expected 378, found 375)

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 611 events
  Seat 0 [LOST]: life=-9 library=84 hand=7 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=65 hand=6 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Reliquary Tower (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Ajani, Caller of the Pride (P/T 0/4, dmg=0)
  Seat 2 [alive]: life=40 library=85 hand=0 graveyard=1 exile=0 battlefield=14 cmdzone=0 mana=0
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Hinterland Logger // Timber Shredder (P/T 2/1, dmg=0) [T]
    - Mossfire Valley (P/T 0/0, dmg=0) [T]
    - Cemetery Prowler (P/T 3/4, dmg=0)
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 5/5, dmg=0) [T]
    - Cinder Glade (P/T 0/0, dmg=0) [T]
    - Ohran Frostfang (P/T 2/6, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Masked Vandal (P/T 1/3, dmg=0)
    - Packsong Pup (P/T 1/1, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=2 graveyard=2 exile=1 battlefield=10 cmdzone=0 mana=0
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Accursed Duneyard (P/T 0/0, dmg=0) [T]
    - Fell the Profane // Fell Mire (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Lord of the Nazgûl (P/T 4/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[591] cast seat=2 source=Packsong Pup amount=2 target=seat0
[592] stack_push seat=2 source=Packsong Pup target=seat0
[593] priority_pass seat=3 source= target=seat0
[594] priority_pass seat=0 source= target=seat0
[595] priority_pass seat=1 source= target=seat0
[596] stack_resolve seat=2 source=Packsong Pup target=seat0
[597] enter_battlefield seat=2 source=Packsong Pup target=seat0
[598] phase_step seat=2 source= target=seat0
[599] attackers seat=2 source= target=seat0
[600] blockers seat=0 source= target=seat0
[601] damage seat=2 source=Hinterland Logger // Timber Shredder amount=2 target=seat0
[602] damage seat=2 source=Cemetery Prowler amount=3 target=seat0
[603] damage seat=2 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=5 target=seat0
[604] damage seat=2 source=Ohran Frostfang amount=2 target=seat0
[605] sba_704_5a seat=0 source= amount=-9
[606] sba_cycle_complete seat=-1 source=
[607] seat_eliminated seat=0 source= amount=3
[608] phase_step seat=2 source= target=seat0
[609] pool_drain seat=2 source= amount=1 target=seat0
[610] state seat=2 source= target=seat0
```

</details>

#### Violation 48

- **Game**: 9 (seed 9043)
- **Invariant**: ZoneConservation
- **Turn**: 28, Phase=ending Step=cleanup
- **Events**: 611
- **Message**: zone conservation violated: 3 real cards disappeared (expected 378, found 375)

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 611 events
  Seat 0 [LOST]: life=-9 library=84 hand=7 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=40 library=65 hand=6 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Reliquary Tower (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Ajani, Caller of the Pride (P/T 0/4, dmg=0)
  Seat 2 [alive]: life=40 library=85 hand=0 graveyard=1 exile=0 battlefield=14 cmdzone=0 mana=0
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Hinterland Logger // Timber Shredder (P/T 2/1, dmg=0) [T]
    - Mossfire Valley (P/T 0/0, dmg=0) [T]
    - Cemetery Prowler (P/T 3/4, dmg=0)
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 5/5, dmg=0) [T]
    - Cinder Glade (P/T 0/0, dmg=0) [T]
    - Ohran Frostfang (P/T 2/6, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Masked Vandal (P/T 1/3, dmg=0)
    - Packsong Pup (P/T 1/1, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=2 graveyard=2 exile=1 battlefield=10 cmdzone=0 mana=0
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Accursed Duneyard (P/T 0/0, dmg=0) [T]
    - Fell the Profane // Fell Mire (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Lord of the Nazgûl (P/T 4/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[591] cast seat=2 source=Packsong Pup amount=2 target=seat0
[592] stack_push seat=2 source=Packsong Pup target=seat0
[593] priority_pass seat=3 source= target=seat0
[594] priority_pass seat=0 source= target=seat0
[595] priority_pass seat=1 source= target=seat0
[596] stack_resolve seat=2 source=Packsong Pup target=seat0
[597] enter_battlefield seat=2 source=Packsong Pup target=seat0
[598] phase_step seat=2 source= target=seat0
[599] attackers seat=2 source= target=seat0
[600] blockers seat=0 source= target=seat0
[601] damage seat=2 source=Hinterland Logger // Timber Shredder amount=2 target=seat0
[602] damage seat=2 source=Cemetery Prowler amount=3 target=seat0
[603] damage seat=2 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=5 target=seat0
[604] damage seat=2 source=Ohran Frostfang amount=2 target=seat0
[605] sba_704_5a seat=0 source= amount=-9
[606] sba_cycle_complete seat=-1 source=
[607] seat_eliminated seat=0 source= amount=3
[608] phase_step seat=2 source= target=seat0
[609] pool_drain seat=2 source= amount=1 target=seat0
[610] state seat=2 source= target=seat0
```

</details>

#### Violation 49

- **Game**: 9 (seed 9043)
- **Invariant**: ZoneConservation
- **Turn**: 29, Phase=ending Step=cleanup
- **Events**: 641
- **Message**: zone conservation violated: 3 real cards disappeared (expected 378, found 375)

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 641 events
  Seat 0 [LOST]: life=-9 library=84 hand=7 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=33 library=65 hand=6 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Reliquary Tower (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Ajani, Caller of the Pride (P/T 0/4, dmg=0)
  Seat 2 [alive]: life=40 library=85 hand=0 graveyard=1 exile=0 battlefield=14 cmdzone=0 mana=0
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Hinterland Logger // Timber Shredder (P/T 2/1, dmg=0) [T]
    - Mossfire Valley (P/T 0/0, dmg=0) [T]
    - Cemetery Prowler (P/T 3/4, dmg=0)
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 5/5, dmg=0) [T]
    - Cinder Glade (P/T 0/0, dmg=0) [T]
    - Ohran Frostfang (P/T 2/6, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Masked Vandal (P/T 1/3, dmg=0)
    - Packsong Pup (P/T 1/1, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=1 graveyard=3 exile=1 battlefield=11 cmdzone=0 mana=0
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Accursed Duneyard (P/T 0/0, dmg=0) [T]
    - Fell the Profane // Fell Mire (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Lord of the Nazgûl (P/T 4/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Minas Morgul, Dark Fortress (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[621] untap_done seat=3 source=Nazgûl target=seat0
[622] draw seat=3 source=Counterspell amount=1 target=seat0
[623] play_land seat=3 source=Minas Morgul, Dark Fortress target=seat0
[624] pay_mana seat=3 source=Counterspell amount=2 target=seat0
[625] cast seat=3 source=Counterspell amount=2 target=seat0
[626] stack_push seat=3 source=Counterspell target=seat0
[627] priority_pass seat=1 source= target=seat0
[628] priority_pass seat=2 source= target=seat0
[629] stack_resolve seat=3 source=Counterspell target=seat0
[630] resolve seat=3 source=Counterspell target=seat0
[631] phase_step seat=3 source= target=seat0
[632] attackers seat=3 source= target=seat0
[633] blockers seat=1 source= target=seat0
[634] damage seat=3 source=Nazgûl amount=1 target=seat1
[635] damage seat=3 source=Nazgûl amount=1 target=seat1
[636] damage seat=3 source=Lord of the Nazgûl amount=4 target=seat1
[637] damage seat=3 source=Nazgûl amount=1 target=seat1
[638] phase_step seat=3 source= target=seat0
[639] pool_drain seat=3 source= amount=5 target=seat0
[640] state seat=3 source= target=seat0
```

</details>

#### Violation 50

- **Game**: 9 (seed 9043)
- **Invariant**: ZoneConservation
- **Turn**: 29, Phase=ending Step=cleanup
- **Events**: 641
- **Message**: zone conservation violated: 3 real cards disappeared (expected 378, found 375)

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 641 events
  Seat 0 [LOST]: life=-9 library=84 hand=7 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=33 library=65 hand=6 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Reliquary Tower (P/T 0/0, dmg=0) [T]
    - Tranquil Expanse (P/T 0/0, dmg=0) [T]
    - Ajani, Caller of the Pride (P/T 0/4, dmg=0)
  Seat 2 [alive]: life=40 library=85 hand=0 graveyard=1 exile=0 battlefield=14 cmdzone=0 mana=0
    - Forgotten Cave (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Hinterland Logger // Timber Shredder (P/T 2/1, dmg=0) [T]
    - Mossfire Valley (P/T 0/0, dmg=0) [T]
    - Cemetery Prowler (P/T 3/4, dmg=0)
    - Game Trail (P/T 0/0, dmg=0) [T]
    - Rhonas's Monument (P/T 0/0, dmg=0)
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha (P/T 5/5, dmg=0) [T]
    - Cinder Glade (P/T 0/0, dmg=0) [T]
    - Ohran Frostfang (P/T 2/6, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Masked Vandal (P/T 1/3, dmg=0)
    - Packsong Pup (P/T 1/1, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=1 graveyard=3 exile=1 battlefield=11 cmdzone=0 mana=0
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Accursed Duneyard (P/T 0/0, dmg=0) [T]
    - Fell the Profane // Fell Mire (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Rogue's Passage (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Lord of the Nazgûl (P/T 4/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nazgûl (P/T 1/2, dmg=0) [T]
    - Minas Morgul, Dark Fortress (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[621] untap_done seat=3 source=Nazgûl target=seat0
[622] draw seat=3 source=Counterspell amount=1 target=seat0
[623] play_land seat=3 source=Minas Morgul, Dark Fortress target=seat0
[624] pay_mana seat=3 source=Counterspell amount=2 target=seat0
[625] cast seat=3 source=Counterspell amount=2 target=seat0
[626] stack_push seat=3 source=Counterspell target=seat0
[627] priority_pass seat=1 source= target=seat0
[628] priority_pass seat=2 source= target=seat0
[629] stack_resolve seat=3 source=Counterspell target=seat0
[630] resolve seat=3 source=Counterspell target=seat0
[631] phase_step seat=3 source= target=seat0
[632] attackers seat=3 source= target=seat0
[633] blockers seat=1 source= target=seat0
[634] damage seat=3 source=Nazgûl amount=1 target=seat1
[635] damage seat=3 source=Nazgûl amount=1 target=seat1
[636] damage seat=3 source=Lord of the Nazgûl amount=4 target=seat1
[637] damage seat=3 source=Nazgûl amount=1 target=seat1
[638] phase_step seat=3 source= target=seat0
[639] pool_drain seat=3 source= amount=5 target=seat0
[640] state seat=3 source= target=seat0
```

</details>


*... and 576 more violations not shown.*

## Verdict: VIOLATIONS FOUND

Review the violations above and investigate.
