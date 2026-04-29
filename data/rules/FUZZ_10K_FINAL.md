# Fuzz Report

Generated: 2026-04-16T22:20:22-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Games | 10000 |
| Seed | 1337 |
| Seats | 4 |
| Duration | 23.777s |
| Throughput | 421 games/sec |

## Results

| Metric | Count |
|--------|-------|
| Crashes | 0 |
| Games with violations | 1 |
| Total violations | 10 |

### Violations by Invariant

| Invariant | Count |
|-----------|-------|
| ZoneConservation | 10 |

### Violation Details (first 10)

#### Violation 1

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 55, Phase=ending Step=cleanup
- **Events**: 1554
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 362, found 374) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 55, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1554 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=30 library=57 hand=0 graveyard=7 exile=0 battlefield=25 cmdzone=1 mana=0
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=27 library=77 hand=0 graveyard=9 exile=0 battlefield=13 cmdzone=1 mana=0
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arlinn Kord // Arlinn, Embraced by the Moon (P/T 0/3, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Skarrg, the Rage Pits (P/T 0/0, dmg=0) [T]
    - Outland Liberator // Frenzied Trapbreaker (P/T 2/2, dmg=0) [T]
    - Valakut Awakening // Valakut Stoneforge (P/T 0/0, dmg=0) [T]
    - Anara, Wolvid Familiar (P/T 4/4, dmg=0) [T]
    - Highland Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1534] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1535] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1536] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1537] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1538] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1539] damage seat=2 source=King Darien XLVIII amount=2 target=seat3
[1540] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1541] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1542] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1543] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1544] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[1545] destroy seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha
[1546] sba_704_5g seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha
[1547] zone_change seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha
[1548] sba_704_6d seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha
[1549] sba_cycle_complete seat=-1 source=
[1550] phase_step seat=2 source= target=seat0
[1551] pool_drain seat=2 source= amount=1 target=seat0
[1552] damage_wears_off seat=2 source=Verdant Force amount=4 target=seat0
[1553] state seat=2 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 55, Phase=ending Step=cleanup
- **Events**: 1554
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 362, found 374) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 55, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1554 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=30 library=57 hand=0 graveyard=7 exile=0 battlefield=25 cmdzone=1 mana=0
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=27 library=77 hand=0 graveyard=9 exile=0 battlefield=13 cmdzone=1 mana=0
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arlinn Kord // Arlinn, Embraced by the Moon (P/T 0/3, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Skarrg, the Rage Pits (P/T 0/0, dmg=0) [T]
    - Outland Liberator // Frenzied Trapbreaker (P/T 2/2, dmg=0) [T]
    - Valakut Awakening // Valakut Stoneforge (P/T 0/0, dmg=0) [T]
    - Anara, Wolvid Familiar (P/T 4/4, dmg=0) [T]
    - Highland Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1534] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1535] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1536] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1537] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1538] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1539] damage seat=2 source=King Darien XLVIII amount=2 target=seat3
[1540] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1541] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1542] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1543] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1544] damage seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha amount=4 target=seat2
[1545] destroy seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha
[1546] sba_704_5g seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha
[1547] zone_change seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha
[1548] sba_704_6d seat=3 source=Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha
[1549] sba_cycle_complete seat=-1 source=
[1550] phase_step seat=2 source= target=seat0
[1551] pool_drain seat=2 source= amount=1 target=seat0
[1552] damage_wears_off seat=2 source=Verdant Force amount=4 target=seat0
[1553] state seat=2 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 56, Phase=ending Step=cleanup
- **Events**: 1585
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 362, found 374) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 56, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1585 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=24 library=57 hand=0 graveyard=7 exile=0 battlefield=25 cmdzone=1 mana=0
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=27 library=76 hand=0 graveyard=9 exile=0 battlefield=14 cmdzone=1 mana=0
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arlinn Kord // Arlinn, Embraced by the Moon (P/T 0/3, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Skarrg, the Rage Pits (P/T 0/0, dmg=0) [T]
    - Outland Liberator // Frenzied Trapbreaker (P/T 2/2, dmg=0) [T]
    - Valakut Awakening // Valakut Stoneforge (P/T 0/0, dmg=0) [T]
    - Anara, Wolvid Familiar (P/T 4/4, dmg=0) [T]
    - Highland Forest (P/T 0/0, dmg=0) [T]
    - Timber Wolves (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1565] untap_done seat=3 source=Highland Forest target=seat0
[1566] draw seat=3 source=Timber Wolves amount=1 target=seat0
[1567] pay_mana seat=3 source=Timber Wolves amount=1 target=seat0
[1568] cast seat=3 source=Timber Wolves amount=1 target=seat0
[1569] stack_push seat=3 source=Timber Wolves target=seat0
[1570] priority_pass seat=2 source= target=seat0
[1571] stack_resolve seat=3 source=Timber Wolves target=seat0
[1572] enter_battlefield seat=3 source=Timber Wolves target=seat0
[1573] phase_step seat=3 source= target=seat0
[1574] attackers seat=3 source= target=seat0
[1575] trigger_fires seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1576] stack_push seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1577] priority_pass seat=2 source= target=seat0
[1578] stack_resolve seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1579] blockers seat=2 source= target=seat0
[1580] damage seat=3 source=Outland Liberator // Frenzied Trapbreaker amount=2 target=seat2
[1581] damage seat=3 source=Anara, Wolvid Familiar amount=4 target=seat2
[1582] phase_step seat=3 source= target=seat0
[1583] pool_drain seat=3 source= amount=8 target=seat0
[1584] state seat=3 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 56, Phase=ending Step=cleanup
- **Events**: 1585
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 362, found 374) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 56, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1585 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=24 library=57 hand=0 graveyard=7 exile=0 battlefield=25 cmdzone=1 mana=0
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=27 library=76 hand=0 graveyard=9 exile=0 battlefield=14 cmdzone=1 mana=0
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arlinn Kord // Arlinn, Embraced by the Moon (P/T 0/3, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Skarrg, the Rage Pits (P/T 0/0, dmg=0) [T]
    - Outland Liberator // Frenzied Trapbreaker (P/T 2/2, dmg=0) [T]
    - Valakut Awakening // Valakut Stoneforge (P/T 0/0, dmg=0) [T]
    - Anara, Wolvid Familiar (P/T 4/4, dmg=0) [T]
    - Highland Forest (P/T 0/0, dmg=0) [T]
    - Timber Wolves (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1565] untap_done seat=3 source=Highland Forest target=seat0
[1566] draw seat=3 source=Timber Wolves amount=1 target=seat0
[1567] pay_mana seat=3 source=Timber Wolves amount=1 target=seat0
[1568] cast seat=3 source=Timber Wolves amount=1 target=seat0
[1569] stack_push seat=3 source=Timber Wolves target=seat0
[1570] priority_pass seat=2 source= target=seat0
[1571] stack_resolve seat=3 source=Timber Wolves target=seat0
[1572] enter_battlefield seat=3 source=Timber Wolves target=seat0
[1573] phase_step seat=3 source= target=seat0
[1574] attackers seat=3 source= target=seat0
[1575] trigger_fires seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1576] stack_push seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1577] priority_pass seat=2 source= target=seat0
[1578] stack_resolve seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1579] blockers seat=2 source= target=seat0
[1580] damage seat=3 source=Outland Liberator // Frenzied Trapbreaker amount=2 target=seat2
[1581] damage seat=3 source=Anara, Wolvid Familiar amount=4 target=seat2
[1582] phase_step seat=3 source= target=seat0
[1583] pool_drain seat=3 source= amount=8 target=seat0
[1584] state seat=3 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 57, Phase=ending Step=cleanup
- **Events**: 1649
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 362, found 376) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 57, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1649 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=24 library=56 hand=0 graveyard=8 exile=0 battlefield=27 cmdzone=1 mana=0
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=12 library=76 hand=0 graveyard=10 exile=0 battlefield=13 cmdzone=1 mana=0
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arlinn Kord // Arlinn, Embraced by the Moon (P/T 0/3, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Skarrg, the Rage Pits (P/T 0/0, dmg=0) [T]
    - Outland Liberator // Frenzied Trapbreaker (P/T 2/2, dmg=0) [T]
    - Valakut Awakening // Valakut Stoneforge (P/T 0/0, dmg=0) [T]
    - Anara, Wolvid Familiar (P/T 4/4, dmg=0) [T]
    - Highland Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1629] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1630] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1631] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1632] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1633] damage seat=2 source=King Darien XLVIII amount=2 target=seat3
[1634] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1635] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1636] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1637] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1638] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1639] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1640] damage seat=3 source=Timber Wolves amount=1 target=seat2
[1641] destroy seat=3 source=Timber Wolves
[1642] sba_704_5g seat=3 source=Timber Wolves
[1643] zone_change seat=3 source=Timber Wolves
[1644] sba_cycle_complete seat=-1 source=
[1645] phase_step seat=2 source= target=seat0
[1646] pool_drain seat=2 source= amount=4 target=seat0
[1647] damage_wears_off seat=2 source=Verdant Force amount=1 target=seat0
[1648] state seat=2 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 57, Phase=ending Step=cleanup
- **Events**: 1649
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 362, found 376) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 57, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1649 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=24 library=56 hand=0 graveyard=8 exile=0 battlefield=27 cmdzone=1 mana=0
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=12 library=76 hand=0 graveyard=10 exile=0 battlefield=13 cmdzone=1 mana=0
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arlinn Kord // Arlinn, Embraced by the Moon (P/T 0/3, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Skarrg, the Rage Pits (P/T 0/0, dmg=0) [T]
    - Outland Liberator // Frenzied Trapbreaker (P/T 2/2, dmg=0) [T]
    - Valakut Awakening // Valakut Stoneforge (P/T 0/0, dmg=0) [T]
    - Anara, Wolvid Familiar (P/T 4/4, dmg=0) [T]
    - Highland Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1629] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1630] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1631] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1632] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1633] damage seat=2 source=King Darien XLVIII amount=2 target=seat3
[1634] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1635] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1636] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1637] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1638] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1639] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1640] damage seat=3 source=Timber Wolves amount=1 target=seat2
[1641] destroy seat=3 source=Timber Wolves
[1642] sba_704_5g seat=3 source=Timber Wolves
[1643] zone_change seat=3 source=Timber Wolves
[1644] sba_cycle_complete seat=-1 source=
[1645] phase_step seat=2 source= target=seat0
[1646] pool_drain seat=2 source= amount=4 target=seat0
[1647] damage_wears_off seat=2 source=Verdant Force amount=1 target=seat0
[1648] state seat=2 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 58, Phase=ending Step=cleanup
- **Events**: 1680
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 362, found 376) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 58, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1680 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=18 library=56 hand=0 graveyard=8 exile=0 battlefield=27 cmdzone=1 mana=0
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=12 library=75 hand=0 graveyard=10 exile=0 battlefield=14 cmdzone=1 mana=0
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arlinn Kord // Arlinn, Embraced by the Moon (P/T 0/3, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Skarrg, the Rage Pits (P/T 0/0, dmg=0) [T]
    - Outland Liberator // Frenzied Trapbreaker (P/T 2/2, dmg=0) [T]
    - Valakut Awakening // Valakut Stoneforge (P/T 0/0, dmg=0) [T]
    - Anara, Wolvid Familiar (P/T 4/4, dmg=0) [T]
    - Highland Forest (P/T 0/0, dmg=0) [T]
    - Beast Whisperer (P/T 2/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1660] untap_done seat=3 source=Highland Forest target=seat0
[1661] draw seat=3 source=Beast Whisperer amount=1 target=seat0
[1662] pay_mana seat=3 source=Beast Whisperer amount=4 target=seat0
[1663] cast seat=3 source=Beast Whisperer amount=4 target=seat0
[1664] stack_push seat=3 source=Beast Whisperer target=seat0
[1665] priority_pass seat=2 source= target=seat0
[1666] stack_resolve seat=3 source=Beast Whisperer target=seat0
[1667] enter_battlefield seat=3 source=Beast Whisperer target=seat0
[1668] phase_step seat=3 source= target=seat0
[1669] attackers seat=3 source= target=seat0
[1670] trigger_fires seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1671] stack_push seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1672] priority_pass seat=2 source= target=seat0
[1673] stack_resolve seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1674] blockers seat=2 source= target=seat0
[1675] damage seat=3 source=Outland Liberator // Frenzied Trapbreaker amount=2 target=seat2
[1676] damage seat=3 source=Anara, Wolvid Familiar amount=4 target=seat2
[1677] phase_step seat=3 source= target=seat0
[1678] pool_drain seat=3 source= amount=5 target=seat0
[1679] state seat=3 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 58, Phase=ending Step=cleanup
- **Events**: 1680
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 362, found 376) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 58, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1680 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=18 library=56 hand=0 graveyard=8 exile=0 battlefield=27 cmdzone=1 mana=0
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=0) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=12 library=75 hand=0 graveyard=10 exile=0 battlefield=14 cmdzone=1 mana=0
    - Kessig Wolf Run (P/T 0/0, dmg=0) [T]
    - Weaver of Blossoms // Blossom-Clad Werewolf (P/T 2/3, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Arlinn Kord // Arlinn, Embraced by the Moon (P/T 0/3, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Skarrg, the Rage Pits (P/T 0/0, dmg=0) [T]
    - Outland Liberator // Frenzied Trapbreaker (P/T 2/2, dmg=0) [T]
    - Valakut Awakening // Valakut Stoneforge (P/T 0/0, dmg=0) [T]
    - Anara, Wolvid Familiar (P/T 4/4, dmg=0) [T]
    - Highland Forest (P/T 0/0, dmg=0) [T]
    - Beast Whisperer (P/T 2/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1660] untap_done seat=3 source=Highland Forest target=seat0
[1661] draw seat=3 source=Beast Whisperer amount=1 target=seat0
[1662] pay_mana seat=3 source=Beast Whisperer amount=4 target=seat0
[1663] cast seat=3 source=Beast Whisperer amount=4 target=seat0
[1664] stack_push seat=3 source=Beast Whisperer target=seat0
[1665] priority_pass seat=2 source= target=seat0
[1666] stack_resolve seat=3 source=Beast Whisperer target=seat0
[1667] enter_battlefield seat=3 source=Beast Whisperer target=seat0
[1668] phase_step seat=3 source= target=seat0
[1669] attackers seat=3 source= target=seat0
[1670] trigger_fires seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1671] stack_push seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1672] priority_pass seat=2 source= target=seat0
[1673] stack_resolve seat=3 source=Outland Liberator // Frenzied Trapbreaker target=seat0
[1674] blockers seat=2 source= target=seat0
[1675] damage seat=3 source=Outland Liberator // Frenzied Trapbreaker amount=2 target=seat2
[1676] damage seat=3 source=Anara, Wolvid Familiar amount=4 target=seat2
[1677] phase_step seat=3 source= target=seat0
[1678] pool_drain seat=3 source= amount=5 target=seat0
[1679] state seat=3 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 59, Phase=combat Step=end_of_combat
- **Events**: 1743
- **Message**: zone conservation suspicious: 16 extra real cards appeared (expected 349, found 365) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 59, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1743 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=18 library=55 hand=0 graveyard=8 exile=0 battlefield=30 cmdzone=1 mana=2
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=2) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Railway Brawler (P/T 5/5, dmg=0)
  Seat 3 [LOST]: life=-5 library=75 hand=0 graveyard=11 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1723] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1724] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1725] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1726] damage seat=2 source=King Darien XLVIII amount=2 target=seat3
[1727] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1728] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1729] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1730] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1731] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1732] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1733] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1734] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1735] damage seat=3 source=Beast Whisperer amount=2 target=seat2
[1736] sba_704_5a seat=3 source= amount=-5
[1737] destroy seat=3 source=Beast Whisperer
[1738] sba_704_5g seat=3 source=Beast Whisperer
[1739] zone_change seat=3 source=Beast Whisperer
[1740] sba_cycle_complete seat=-1 source=
[1741] seat_eliminated seat=3 source= amount=13
[1742] game_end seat=2 source=
```

</details>

#### Violation 10

- **Game**: 1852 (seed 1853338)
- **Invariant**: ZoneConservation
- **Turn**: 59, Phase=combat Step=end_of_combat
- **Events**: 1743
- **Message**: zone conservation suspicious: 16 extra real cards appeared (expected 349, found 365) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 59, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1743 events
  Seat 0 [LOST]: life=0 library=80 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-14 library=77 hand=4 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=18 library=55 hand=0 graveyard=8 exile=0 battlefield=30 cmdzone=1 mana=2
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Temple of Plenty (P/T 0/0, dmg=0) [T]
    - Leafkin Druid (P/T 0/3, dmg=0) [T]
    - Commander's Insignia (P/T 0/0, dmg=0)
    - Canopy Vista (P/T 0/0, dmg=0) [T]
    - Blossoming Sands (P/T 0/0, dmg=0) [T]
    - Doubling Season (P/T 0/0, dmg=0)
    - Garruk's Uprising (P/T 0/0, dmg=0)
    - Avacyn's Pilgrim (P/T 1/1, dmg=0) [T]
    - Graypelt Refuge (P/T 0/0, dmg=0) [T]
    - Verdant Force (P/T 7/7, dmg=2) [T]
    - Farhaven Elf (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Intangible Virtue (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - creature white soldier Token (P/T 1/1, dmg=0) [T]
    - King Darien XLVIII (P/T 2/3, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Conclave Tribunal (P/T 0/0, dmg=0)
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - creature green saproling Token (P/T 1/1, dmg=0) [T]
    - Railway Brawler (P/T 5/5, dmg=0)
  Seat 3 [LOST]: life=-5 library=75 hand=0 graveyard=11 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1723] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1724] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1725] damage seat=2 source=creature white soldier Token amount=1 target=seat3
[1726] damage seat=2 source=King Darien XLVIII amount=2 target=seat3
[1727] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1728] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1729] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1730] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1731] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1732] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1733] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1734] damage seat=2 source=creature green saproling Token amount=1 target=seat3
[1735] damage seat=3 source=Beast Whisperer amount=2 target=seat2
[1736] sba_704_5a seat=3 source= amount=-5
[1737] destroy seat=3 source=Beast Whisperer
[1738] sba_704_5g seat=3 source=Beast Whisperer
[1739] zone_change seat=3 source=Beast Whisperer
[1740] sba_cycle_complete seat=-1 source=
[1741] seat_eliminated seat=3 source= amount=13
[1742] game_end seat=2 source=
```

</details>


## Verdict: VIOLATIONS FOUND

Review the violations above and investigate.
