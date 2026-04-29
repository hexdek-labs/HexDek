# Fuzz Report

Generated: 2026-04-16T23:25:40-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Games | 100000 |
| Seed | 7777 |
| Seats | 4 |
| Duration | 7m10.18s |
| Throughput | 232 games/sec |

## Results

| Metric | Count |
|--------|-------|
| Crashes | 0 |
| Games with violations | 4 |
| Total violations | 4 |

### Violations by Invariant

| Invariant | Count |
|-----------|-------|
| SBACompleteness | 4 |

### Violation Details (first 4)

#### Violation 1

- **Game**: 14127 (seed 14134778)
- **Invariant**: SBACompleteness
- **Turn**: 54, Phase=combat Step=end_of_combat
- **Events**: 1248
- **Message**: seat 2 has creature "Stitcher's Supplier" on battlefield with toughness=0 — SBA 704.5f missed

<details>
<summary>Game State</summary>

```
Turn 54, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1248 events
  Seat 0 [LOST]: life=-14 library=77 hand=3 graveyard=11 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-4 library=80 hand=0 graveyard=11 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=20 library=76 hand=1 graveyard=6 exile=0 battlefield=16 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Tangled Islet (P/T 0/0, dmg=0) [T]
    - Alchemist's Refuge (P/T 0/0, dmg=0) [T]
    - Opulent Palace (P/T 0/0, dmg=0) [T]
    - Haunted Mire (P/T 0/0, dmg=0) [T]
    - Summon: Titan (P/T 7/7, dmg=0) [T]
    - Breeding Pool (P/T 0/0, dmg=0) [T]
    - Aesi, Tyrant of Gyre Strait (P/T 5/5, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Deadeye Navigator (P/T 5/5, dmg=0) [T]
    - Stitcher's Supplier (P/T 0/0, dmg=0)
    - Sol Ring (P/T 0/0, dmg=0) [T]
    - Kishla Village (P/T 0/0, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Necropolis Fiend (P/T 4/5, dmg=0)
  Seat 3 [LOST]: life=0 library=58 hand=5 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1228] draw seat=2 source=Necropolis Fiend amount=1 target=seat0
[1229] pay_mana seat=2 source=Necropolis Fiend amount=9 target=seat0
[1230] cast seat=2 source=Necropolis Fiend amount=9 target=seat0
[1231] stack_push seat=2 source=Necropolis Fiend target=seat0
[1232] priority_pass seat=0 source= target=seat0
[1233] stack_resolve seat=2 source=Necropolis Fiend target=seat0
[1234] buff seat=0 source=Necropolis Fiend amount=-1 target=seat0
[1235] enter_battlefield seat=2 source=Necropolis Fiend target=seat0
[1236] phase_step seat=2 source= target=seat0
[1237] attackers seat=2 source= target=seat0
[1238] blockers seat=0 source= target=seat0
[1239] damage seat=2 source=Summon: Titan amount=7 target=seat0
[1240] damage seat=2 source=Aesi, Tyrant of Gyre Strait amount=5 target=seat0
[1241] damage seat=2 source=Sin, Spira's Punishment amount=7 target=seat0
[1242] damage seat=2 source=Deadeye Navigator amount=5 target=seat0
[1243] damage seat=2 source=Grappling Kraken amount=5 target=seat0
[1244] sba_704_5a seat=0 source= amount=-14
[1245] sba_cycle_complete seat=-1 source=
[1246] seat_eliminated seat=0 source= amount=8
[1247] game_end seat=2 source=
```

</details>

#### Violation 2

- **Game**: 77451 (seed 77458778)
- **Invariant**: SBACompleteness
- **Turn**: 47, Phase=combat Step=end_of_combat
- **Events**: 1017
- **Message**: seat 2 has creature "Stitcher's Supplier" on battlefield with toughness=0 — SBA 704.5f missed

<details>
<summary>Game State</summary>

```
Turn 47, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1017 events
  Seat 0 [LOST]: life=-1 library=83 hand=2 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-11 library=79 hand=0 graveyard=9 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=40 library=78 hand=0 graveyard=6 exile=0 battlefield=15 cmdzone=0 mana=0
    - Llanowar Wastes (P/T 0/0, dmg=0) [T]
    - Golgari Rot Farm (P/T 0/0, dmg=0) [T]
    - Sunken Hollow (P/T 0/0, dmg=0) [T]
    - Awaken the Honored Dead (P/T 0/0, dmg=0)
    - Tangled Islet (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Aesi, Tyrant of Gyre Strait (P/T 5/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 8/8, dmg=0) [T]
    - Stitcher's Supplier (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Simic Growth Chamber (P/T 0/0, dmg=0) [T]
    - Command Beacon (P/T 0/0, dmg=0) [T]
    - Necropolis Fiend (P/T 4/5, dmg=0)
  Seat 3 [LOST]: life=0 library=59 hand=6 graveyard=7 exile=2 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[997] untap_done seat=2 source=Island target=seat0
[998] untap_done seat=2 source=Simic Growth Chamber target=seat0
[999] untap_done seat=2 source=Command Beacon target=seat0
[1000] draw seat=2 source=Necropolis Fiend amount=1 target=seat0
[1001] pay_mana seat=2 source=Necropolis Fiend amount=10 target=seat0
[1002] cast seat=2 source=Necropolis Fiend amount=10 target=seat0
[1003] stack_push seat=2 source=Necropolis Fiend target=seat0
[1004] priority_pass seat=3 source= target=seat0
[1005] stack_resolve seat=2 source=Necropolis Fiend target=seat0
[1006] buff seat=0 source=Necropolis Fiend amount=-1 target=seat0
[1007] enter_battlefield seat=2 source=Necropolis Fiend target=seat0
[1008] phase_step seat=2 source= target=seat0
[1009] attackers seat=2 source= target=seat0
[1010] blockers seat=3 source= target=seat0
[1011] damage seat=2 source=Aesi, Tyrant of Gyre Strait amount=5 target=seat3
[1012] damage seat=2 source=Sin, Spira's Punishment amount=8 target=seat3
[1013] sba_704_5a seat=3 source=
[1014] sba_cycle_complete seat=-1 source=
[1015] seat_eliminated seat=3 source= amount=5
[1016] game_end seat=2 source=
```

</details>

#### Violation 3

- **Game**: 88070 (seed 88077778)
- **Invariant**: SBACompleteness
- **Turn**: 57, Phase=combat Step=end_of_combat
- **Events**: 1400
- **Message**: seat 3 has creature "Mutable Explorer" on battlefield with toughness=0 — SBA 704.5f missed

<details>
<summary>Game State</summary>

```
Turn 57, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1400 events
  Seat 0 [LOST]: life=-22 library=56 hand=5 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-5 library=79 hand=1 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-1 library=80 hand=0 graveyard=8 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=3 library=74 hand=1 graveyard=7 exile=0 battlefield=17 cmdzone=0 mana=0
    - Overgrown Tomb (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Perpetual Timepiece (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Yavimaya Coast (P/T 0/0, dmg=0) [T]
    - Awaken the Honored Dead (P/T 0/0, dmg=0)
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Command Beacon (P/T 0/0, dmg=0) [T]
    - Glacier Godmaw (P/T 6/6, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Colossal Grave-Reaver (P/T 7/6, dmg=0) [T]
    - Mutable Explorer (P/T 0/0, dmg=0)
    - Alchemist's Refuge (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Necropolis Fiend (P/T 4/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1380] untap_done seat=3 source=Alchemist's Refuge target=seat0
[1381] draw seat=3 source=Retreat to Hagra amount=1 target=seat0
[1382] pay_mana seat=3 source=Necropolis Fiend amount=9 target=seat0
[1383] cast seat=3 source=Necropolis Fiend amount=9 target=seat0
[1384] stack_push seat=3 source=Necropolis Fiend target=seat0
[1385] priority_pass seat=0 source= target=seat0
[1386] stack_resolve seat=3 source=Necropolis Fiend target=seat0
[1387] buff seat=0 source=Necropolis Fiend amount=-1 target=seat0
[1388] enter_battlefield seat=3 source=Necropolis Fiend target=seat0
[1389] phase_step seat=3 source= target=seat0
[1390] attackers seat=3 source= target=seat0
[1391] blockers seat=0 source= target=seat0
[1392] damage seat=3 source=Grappling Kraken amount=5 target=seat0
[1393] damage seat=3 source=Glacier Godmaw amount=6 target=seat0
[1394] damage seat=3 source=Colossal Grave-Reaver amount=7 target=seat0
[1395] damage seat=3 source=Sin, Spira's Punishment amount=7 target=seat0
[1396] sba_704_5a seat=0 source= amount=-22
[1397] sba_cycle_complete seat=-1 source=
[1398] seat_eliminated seat=0 source= amount=11
[1399] game_end seat=3 source=
```

</details>

#### Violation 4

- **Game**: 95274 (seed 95281778)
- **Invariant**: SBACompleteness
- **Turn**: 46, Phase=combat Step=end_of_combat
- **Events**: 1100
- **Message**: seat 3 has creature "Mutable Explorer" on battlefield with toughness=0 — SBA 704.5f missed

<details>
<summary>Game State</summary>

```
Turn 46, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1100 events
  Seat 0 [LOST]: life=-8 library=62 hand=2 graveyard=5 exile=1 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-8 library=84 hand=0 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=-20 library=77 hand=0 graveyard=11 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=52 library=77 hand=0 graveyard=6 exile=0 battlefield=16 cmdzone=0 mana=0
    - Golgari Rot Farm (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Tangled Islet (P/T 0/0, dmg=0) [T]
    - Haunted Mire (P/T 0/0, dmg=0) [T]
    - Dimir Aqueduct (P/T 0/0, dmg=0) [T]
    - Underground River (P/T 0/0, dmg=0) [T]
    - Grappling Kraken (P/T 5/6, dmg=0) [T]
    - Endless Sands (P/T 0/0, dmg=0) [T]
    - Sin, Spira's Punishment (P/T 7/7, dmg=0) [T]
    - Mutable Explorer (P/T 0/0, dmg=0)
    - Yarok, the Desecrated (P/T 3/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Tireless Provisioner (P/T 3/2, dmg=0) [T]
    - Aesi, Tyrant of Gyre Strait (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Necropolis Fiend (P/T 4/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1080] pay_mana seat=3 source=Necropolis Fiend amount=9 target=seat0
[1081] cast seat=3 source=Necropolis Fiend amount=9 target=seat0
[1082] stack_push seat=3 source=Necropolis Fiend target=seat0
[1083] priority_pass seat=2 source= target=seat0
[1084] stack_resolve seat=3 source=Necropolis Fiend target=seat0
[1085] buff seat=0 source=Necropolis Fiend amount=-1 target=seat0
[1086] enter_battlefield seat=3 source=Necropolis Fiend target=seat0
[1087] phase_step seat=3 source= target=seat0
[1088] attackers seat=3 source= target=seat0
[1089] blockers seat=2 source= target=seat0
[1090] damage seat=3 source=Grappling Kraken amount=5 target=seat2
[1091] damage seat=3 source=Sin, Spira's Punishment amount=7 target=seat2
[1092] damage seat=3 source=Yarok, the Desecrated amount=3 target=seat2
[1093] life_change seat=3 source=Yarok, the Desecrated amount=3 target=seat0
[1094] damage seat=3 source=Tireless Provisioner amount=3 target=seat2
[1095] damage seat=3 source=Aesi, Tyrant of Gyre Strait amount=5 target=seat2
[1096] sba_704_5a seat=2 source= amount=-20
[1097] sba_cycle_complete seat=-1 source=
[1098] seat_eliminated seat=2 source= amount=10
[1099] game_end seat=3 source=
```

</details>


## Verdict: VIOLATIONS FOUND

Review the violations above and investigate.
