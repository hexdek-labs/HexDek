# Chaos Gauntlet Report

Generated: 2026-05-19T10:51:02-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36656 cards |
| Legendary Creatures | 3433 |
| Total Games | 5000 |
| Seed | 41 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 3m39.577s |
| Throughput | 23 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 1117 (in 49 games) |
| Clean Games | 4951 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 2.451s |
| Throughput | 4079 boards/sec |
| Crashes | 0 |
| Invariant Violations | 6 |
| Clean Boards | 9997 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| CardIdentity | 412 |
| ZoneCastGrantExpiry | 8 |
| AttachmentConsistency | 23 |
| TriggerCompleteness | 10 |
| ZoneConservation | 664 |

### Violation Details (first 30)

#### Violation 1

- **Game**: 109 (seed 1090042, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 48, Phase=ending Step=cleanup
- **Commanders**: Jaxis, the Troublemaker, Obeka, Splitter of Seconds, Ravos, Soultender, Sidar Jabari
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 4277 with trigger-bearer(s) [{Jaxis, the Troublemaker 0}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 48, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 4284 events
  Seat 0 [alive]: life=17 library=78 hand=2 graveyard=7 exile=1 battlefield=10 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Diamond Valley (P/T 0/0, dmg=0) [T]
    - Phyrexia's Core (P/T 0/0, dmg=0) [T]
    - Macetail Hystrodon (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Jaxis, the Troublemaker (P/T 2/3, dmg=0) [T]
  Seat 1 [alive]: life=16 library=80 hand=6 graveyard=4 exile=0 battlefield=10 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Survivors' Encampment (P/T 0/0, dmg=0) [T]
    - Obeka, Splitter of Seconds (P/T 2/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Markov Enforcer (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=6 library=70 hand=0 graveyard=0 exile=14 battlefield=14 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Jalum Tome (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Throwing Knife (P/T 0/0, dmg=0)
    - Chthonian Nightmare (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Cultivator's Caravan (P/T 5/5, dmg=0)
    - Griffnaut Tracker (P/T 3/2, dmg=0)
    - Fire Nation Ambushers (P/T 3/2, dmg=0)
    - Enslaved Horror (P/T 4/4, dmg=0)
    - Liliana's Steward (P/T 1/2, dmg=0)
  Seat 3 [alive]: life=20 library=77 hand=0 graveyard=9 exile=0 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Carrier Pigeons (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Felidar Retreat (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Righteous Indignation (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Sidar Jabari (P/T 2/2, dmg=0) [T]
    - Endoskeleton (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4264] add_mana seat=0 source=Mountain amount=1 target=seat0
[4265] add_mana seat=0 source=Diamond Valley amount=1 target=seat0
[4266] add_mana seat=0 source=Phyrexia's Core amount=1 target=seat0
[4267] add_mana seat=0 source=Mountain amount=1 target=seat0
[4268] draw seat=0 source=Loot, the Pathfinder // Loot, the Pathfinder amount=1 target=seat0
[4269] phase_step seat=0 source= target=seat0
[4270] declare_attackers seat=0 source= target=seat0
[4271] blockers seat=2 source= target=seat0
[4272] damage seat=0 source=Goblin Berserker amount=2 target=seat2
[4273] damage seat=0 source=Macetail Hystrodon amount=4 target=seat2
[4274] damage seat=0 source=Jaxis, the Troublemaker amount=2 target=seat2
[4275] damage seat=2 source=Enslaved Horror amount=4 target=seat0
[4276] destroy seat=0 source=Goblin Berserker
[4277] sba_704_5g seat=0 source=Goblin Berserker
[4278] zone_change seat=0 source=Goblin Berserker
[4279] sba_cycle_complete seat=-1 source=
[4280] phase_step seat=0 source= target=seat0
[4281] pool_drain seat=0 source= amount=8 target=seat0
[4282] damage_wears_off seat=2 source=Enslaved Horror amount=2 target=seat0
[4283] state seat=0 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 109 (seed 1090042, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 48, Phase=ending Step=cleanup
- **Commanders**: Jaxis, the Troublemaker, Obeka, Splitter of Seconds, Ravos, Soultender, Sidar Jabari
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 4277 with trigger-bearer(s) [{Jaxis, the Troublemaker 0}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 48, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 4284 events
  Seat 0 [alive]: life=17 library=78 hand=2 graveyard=7 exile=1 battlefield=10 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Diamond Valley (P/T 0/0, dmg=0) [T]
    - Phyrexia's Core (P/T 0/0, dmg=0) [T]
    - Macetail Hystrodon (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Jaxis, the Troublemaker (P/T 2/3, dmg=0) [T]
  Seat 1 [alive]: life=16 library=80 hand=6 graveyard=4 exile=0 battlefield=10 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Survivors' Encampment (P/T 0/0, dmg=0) [T]
    - Obeka, Splitter of Seconds (P/T 2/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Markov Enforcer (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=6 library=70 hand=0 graveyard=0 exile=14 battlefield=14 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Jalum Tome (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Throwing Knife (P/T 0/0, dmg=0)
    - Chthonian Nightmare (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Cultivator's Caravan (P/T 5/5, dmg=0)
    - Griffnaut Tracker (P/T 3/2, dmg=0)
    - Fire Nation Ambushers (P/T 3/2, dmg=0)
    - Enslaved Horror (P/T 4/4, dmg=0)
    - Liliana's Steward (P/T 1/2, dmg=0)
  Seat 3 [alive]: life=20 library=77 hand=0 graveyard=9 exile=0 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Carrier Pigeons (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Felidar Retreat (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Righteous Indignation (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Sidar Jabari (P/T 2/2, dmg=0) [T]
    - Endoskeleton (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4264] add_mana seat=0 source=Mountain amount=1 target=seat0
[4265] add_mana seat=0 source=Diamond Valley amount=1 target=seat0
[4266] add_mana seat=0 source=Phyrexia's Core amount=1 target=seat0
[4267] add_mana seat=0 source=Mountain amount=1 target=seat0
[4268] draw seat=0 source=Loot, the Pathfinder // Loot, the Pathfinder amount=1 target=seat0
[4269] phase_step seat=0 source= target=seat0
[4270] declare_attackers seat=0 source= target=seat0
[4271] blockers seat=2 source= target=seat0
[4272] damage seat=0 source=Goblin Berserker amount=2 target=seat2
[4273] damage seat=0 source=Macetail Hystrodon amount=4 target=seat2
[4274] damage seat=0 source=Jaxis, the Troublemaker amount=2 target=seat2
[4275] damage seat=2 source=Enslaved Horror amount=4 target=seat0
[4276] destroy seat=0 source=Goblin Berserker
[4277] sba_704_5g seat=0 source=Goblin Berserker
[4278] zone_change seat=0 source=Goblin Berserker
[4279] sba_cycle_complete seat=-1 source=
[4280] phase_step seat=0 source= target=seat0
[4281] pool_drain seat=0 source= amount=8 target=seat0
[4282] damage_wears_off seat=2 source=Enslaved Horror amount=2 target=seat0
[4283] state seat=0 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 10, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 2 real cards disappeared (expected 398, found 396)

<details>
<summary>Game State</summary>

```
Turn 10, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 626 events
  Seat 0 [alive]: life=40 library=90 hand=7 graveyard=1 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=90 hand=6 graveyard=1 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0)
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=1 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=87 hand=4 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[606] priority_pass seat=2 source= target=seat0
[607] priority_pass seat=0 source= target=seat0
[608] priority_pass seat=1 source= target=seat0
[609] priority_pass seat=2 source= target=seat0
[610] priority_pass seat=0 source= target=seat0
[611] priority_pass seat=1 source= target=seat0
[612] priority_pass seat=2 source= target=seat0
[613] priority_pass seat=0 source= target=seat0
[614] priority_pass seat=1 source= target=seat0
[615] priority_pass seat=2 source= target=seat0
[616] loop_shortcut seat=0 source=no_op_loop target=seat0
[617] phase_step seat=3 source= target=seat0
[618] declare_attackers seat=3 source= target=seat0
[619] blockers seat=1 source= target=seat0
[620] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[621] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat3
[622] phase_step seat=3 source= target=seat0
[623] damage_wears_off seat=1 source=Baron Bertram Graywater amount=2 target=seat0
[624] damage_wears_off seat=3 source=Friendly Teddy amount=3 target=seat0
[625] state seat=3 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 10, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 2 real cards disappeared (expected 398, found 396)

<details>
<summary>Game State</summary>

```
Turn 10, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 626 events
  Seat 0 [alive]: life=40 library=90 hand=7 graveyard=1 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=90 hand=6 graveyard=1 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0)
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=1 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=87 hand=4 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[606] priority_pass seat=2 source= target=seat0
[607] priority_pass seat=0 source= target=seat0
[608] priority_pass seat=1 source= target=seat0
[609] priority_pass seat=2 source= target=seat0
[610] priority_pass seat=0 source= target=seat0
[611] priority_pass seat=1 source= target=seat0
[612] priority_pass seat=2 source= target=seat0
[613] priority_pass seat=0 source= target=seat0
[614] priority_pass seat=1 source= target=seat0
[615] priority_pass seat=2 source= target=seat0
[616] loop_shortcut seat=0 source=no_op_loop target=seat0
[617] phase_step seat=3 source= target=seat0
[618] declare_attackers seat=3 source= target=seat0
[619] blockers seat=1 source= target=seat0
[620] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[621] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat3
[622] phase_step seat=3 source= target=seat0
[623] damage_wears_off seat=1 source=Baron Bertram Graywater amount=2 target=seat0
[624] damage_wears_off seat=3 source=Friendly Teddy amount=3 target=seat0
[625] state seat=3 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 11, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 2 real cards disappeared (expected 398, found 396)

<details>
<summary>Game State</summary>

```
Turn 11, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 636 events
  Seat 0 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=90 hand=6 graveyard=1 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0)
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=1 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=87 hand=4 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[616] loop_shortcut seat=0 source=no_op_loop target=seat0
[617] phase_step seat=3 source= target=seat0
[618] declare_attackers seat=3 source= target=seat0
[619] blockers seat=1 source= target=seat0
[620] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[621] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat3
[622] phase_step seat=3 source= target=seat0
[623] damage_wears_off seat=1 source=Baron Bertram Graywater amount=2 target=seat0
[624] damage_wears_off seat=3 source=Friendly Teddy amount=3 target=seat0
[625] state seat=3 source= target=seat0
[626] turn_start seat=0 source= target=seat0
[627] untap_done seat=0 source=Plains target=seat0
[628] add_mana seat=0 source=Plains amount=1 target=seat0
[629] draw seat=0 source=Knight of Cliffhaven amount=1 target=seat0
[630] phase_step seat=0 source= target=seat0
[631] phase_step seat=0 source= target=seat0
[632] pool_drain seat=0 source= amount=1 target=seat0
[633] zone_change seat=0 source=Archon of Falling Stars
[634] discard seat=0 source=Archon of Falling Stars target=seat0
[635] state seat=0 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 11, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 2 real cards disappeared (expected 398, found 396)

<details>
<summary>Game State</summary>

```
Turn 11, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 636 events
  Seat 0 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=90 hand=6 graveyard=1 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0)
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=1 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=87 hand=4 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[616] loop_shortcut seat=0 source=no_op_loop target=seat0
[617] phase_step seat=3 source= target=seat0
[618] declare_attackers seat=3 source= target=seat0
[619] blockers seat=1 source= target=seat0
[620] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[621] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat3
[622] phase_step seat=3 source= target=seat0
[623] damage_wears_off seat=1 source=Baron Bertram Graywater amount=2 target=seat0
[624] damage_wears_off seat=3 source=Friendly Teddy amount=3 target=seat0
[625] state seat=3 source= target=seat0
[626] turn_start seat=0 source= target=seat0
[627] untap_done seat=0 source=Plains target=seat0
[628] add_mana seat=0 source=Plains amount=1 target=seat0
[629] draw seat=0 source=Knight of Cliffhaven amount=1 target=seat0
[630] phase_step seat=0 source= target=seat0
[631] phase_step seat=0 source= target=seat0
[632] pool_drain seat=0 source= amount=1 target=seat0
[633] zone_change seat=0 source=Archon of Falling Stars
[634] discard seat=0 source=Archon of Falling Stars target=seat0
[635] state seat=0 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 12, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 2 real cards disappeared (expected 398, found 396)

<details>
<summary>Game State</summary>

```
Turn 12, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 682 events
  Seat 0 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=1 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=37 library=87 hand=4 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[662] priority_pass seat=3 source= target=seat0
[663] priority_pass seat=0 source= target=seat0
[664] priority_pass seat=2 source= target=seat0
[665] priority_pass seat=3 source= target=seat0
[666] priority_pass seat=0 source= target=seat0
[667] priority_pass seat=2 source= target=seat0
[668] priority_pass seat=3 source= target=seat0
[669] priority_pass seat=0 source= target=seat0
[670] priority_pass seat=2 source= target=seat0
[671] priority_pass seat=3 source= target=seat0
[672] priority_pass seat=0 source= target=seat0
[673] loop_shortcut seat=0 source=no_op_loop target=seat0
[674] draw seat=1 source=Corrosive Mentor amount=1 target=seat0
[675] phase_step seat=1 source= target=seat0
[676] declare_attackers seat=1 source= target=seat0
[677] blockers seat=3 source= target=seat0
[678] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat3
[679] speed_advance seat=1 source= amount=1 target=seat0
[680] phase_step seat=1 source= target=seat0
[681] state seat=1 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 12, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 2 real cards disappeared (expected 398, found 396)

<details>
<summary>Game State</summary>

```
Turn 12, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 682 events
  Seat 0 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=89 hand=5 graveyard=0 exile=1 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=37 library=87 hand=4 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[662] priority_pass seat=3 source= target=seat0
[663] priority_pass seat=0 source= target=seat0
[664] priority_pass seat=2 source= target=seat0
[665] priority_pass seat=3 source= target=seat0
[666] priority_pass seat=0 source= target=seat0
[667] priority_pass seat=2 source= target=seat0
[668] priority_pass seat=3 source= target=seat0
[669] priority_pass seat=0 source= target=seat0
[670] priority_pass seat=2 source= target=seat0
[671] priority_pass seat=3 source= target=seat0
[672] priority_pass seat=0 source= target=seat0
[673] loop_shortcut seat=0 source=no_op_loop target=seat0
[674] draw seat=1 source=Corrosive Mentor amount=1 target=seat0
[675] phase_step seat=1 source= target=seat0
[676] declare_attackers seat=1 source= target=seat0
[677] blockers seat=3 source= target=seat0
[678] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat3
[679] speed_advance seat=1 source= amount=1 target=seat0
[680] phase_step seat=1 source= target=seat0
[681] state seat=1 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 13, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 4 real cards disappeared (expected 398, found 394)

<details>
<summary>Game State</summary>

```
Turn 13, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 730 events
  Seat 0 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=88 hand=4 graveyard=0 exile=1 battlefield=5 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=87 hand=4 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[710] priority_pass seat=1 source= target=seat0
[711] priority_pass seat=3 source= target=seat0
[712] priority_pass seat=0 source= target=seat0
[713] priority_pass seat=1 source= target=seat0
[714] priority_pass seat=3 source= target=seat0
[715] priority_pass seat=0 source= target=seat0
[716] priority_pass seat=1 source= target=seat0
[717] priority_pass seat=3 source= target=seat0
[718] priority_pass seat=0 source= target=seat0
[719] priority_pass seat=1 source= target=seat0
[720] priority_pass seat=3 source= target=seat0
[721] priority_pass seat=0 source= target=seat0
[722] priority_pass seat=1 source= target=seat0
[723] priority_pass seat=3 source= target=seat0
[724] priority_pass seat=0 source= target=seat0
[725] priority_pass seat=1 source= target=seat0
[726] loop_shortcut seat=0 source=no_op_loop target=seat0
[727] phase_step seat=2 source= target=seat0
[728] phase_step seat=2 source= target=seat0
[729] state seat=2 source= target=seat0
```

</details>

#### Violation 10

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 13, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 4 real cards disappeared (expected 398, found 394)

<details>
<summary>Game State</summary>

```
Turn 13, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 730 events
  Seat 0 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=88 hand=4 graveyard=0 exile=1 battlefield=5 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=87 hand=4 graveyard=0 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[710] priority_pass seat=1 source= target=seat0
[711] priority_pass seat=3 source= target=seat0
[712] priority_pass seat=0 source= target=seat0
[713] priority_pass seat=1 source= target=seat0
[714] priority_pass seat=3 source= target=seat0
[715] priority_pass seat=0 source= target=seat0
[716] priority_pass seat=1 source= target=seat0
[717] priority_pass seat=3 source= target=seat0
[718] priority_pass seat=0 source= target=seat0
[719] priority_pass seat=1 source= target=seat0
[720] priority_pass seat=3 source= target=seat0
[721] priority_pass seat=0 source= target=seat0
[722] priority_pass seat=1 source= target=seat0
[723] priority_pass seat=3 source= target=seat0
[724] priority_pass seat=0 source= target=seat0
[725] priority_pass seat=1 source= target=seat0
[726] loop_shortcut seat=0 source=no_op_loop target=seat0
[727] phase_step seat=2 source= target=seat0
[728] phase_step seat=2 source= target=seat0
[729] state seat=2 source= target=seat0
```

</details>

#### Violation 11

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 14, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 5 real cards disappeared (expected 398, found 393)

<details>
<summary>Game State</summary>

```
Turn 14, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 780 events
  Seat 0 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=89 hand=7 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=88 hand=4 graveyard=0 exile=1 battlefield=5 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=86 hand=3 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[760] priority_pass seat=1 source= target=seat0
[761] priority_pass seat=2 source= target=seat0
[762] priority_pass seat=0 source= target=seat0
[763] priority_pass seat=1 source= target=seat0
[764] priority_pass seat=2 source= target=seat0
[765] priority_pass seat=0 source= target=seat0
[766] priority_pass seat=1 source= target=seat0
[767] priority_pass seat=2 source= target=seat0
[768] priority_pass seat=0 source= target=seat0
[769] priority_pass seat=1 source= target=seat0
[770] priority_pass seat=2 source= target=seat0
[771] loop_shortcut seat=0 source=no_op_loop target=seat0
[772] phase_step seat=3 source= target=seat0
[773] declare_attackers seat=3 source= target=seat0
[774] blockers seat=1 source= target=seat0
[775] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[776] speed_advance seat=3 source= amount=1 target=seat0
[777] phase_step seat=3 source= target=seat0
[778] pool_drain seat=3 source= amount=2 target=seat0
[779] state seat=3 source= target=seat0
```

</details>

#### Violation 12

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 14, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 5 real cards disappeared (expected 398, found 393)

<details>
<summary>Game State</summary>

```
Turn 14, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 780 events
  Seat 0 [alive]: life=40 library=89 hand=7 graveyard=2 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=89 hand=7 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=88 hand=4 graveyard=0 exile=1 battlefield=5 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=86 hand=3 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[760] priority_pass seat=1 source= target=seat0
[761] priority_pass seat=2 source= target=seat0
[762] priority_pass seat=0 source= target=seat0
[763] priority_pass seat=1 source= target=seat0
[764] priority_pass seat=2 source= target=seat0
[765] priority_pass seat=0 source= target=seat0
[766] priority_pass seat=1 source= target=seat0
[767] priority_pass seat=2 source= target=seat0
[768] priority_pass seat=0 source= target=seat0
[769] priority_pass seat=1 source= target=seat0
[770] priority_pass seat=2 source= target=seat0
[771] loop_shortcut seat=0 source=no_op_loop target=seat0
[772] phase_step seat=3 source= target=seat0
[773] declare_attackers seat=3 source= target=seat0
[774] blockers seat=1 source= target=seat0
[775] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[776] speed_advance seat=3 source= amount=1 target=seat0
[777] phase_step seat=3 source= target=seat0
[778] pool_drain seat=3 source= amount=2 target=seat0
[779] state seat=3 source= target=seat0
```

</details>

#### Violation 13

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 15, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 5 real cards disappeared (expected 398, found 393)

<details>
<summary>Game State</summary>

```
Turn 15, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 790 events
  Seat 0 [alive]: life=40 library=88 hand=7 graveyard=3 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=89 hand=7 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=88 hand=4 graveyard=0 exile=1 battlefield=5 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=86 hand=3 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[770] priority_pass seat=2 source= target=seat0
[771] loop_shortcut seat=0 source=no_op_loop target=seat0
[772] phase_step seat=3 source= target=seat0
[773] declare_attackers seat=3 source= target=seat0
[774] blockers seat=1 source= target=seat0
[775] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[776] speed_advance seat=3 source= amount=1 target=seat0
[777] phase_step seat=3 source= target=seat0
[778] pool_drain seat=3 source= amount=2 target=seat0
[779] state seat=3 source= target=seat0
[780] turn_start seat=0 source= target=seat0
[781] untap_done seat=0 source=Plains target=seat0
[782] add_mana seat=0 source=Plains amount=1 target=seat0
[783] draw seat=0 source=Academy Rector amount=1 target=seat0
[784] phase_step seat=0 source= target=seat0
[785] phase_step seat=0 source= target=seat0
[786] pool_drain seat=0 source= amount=1 target=seat0
[787] zone_change seat=0 source=Enraged Giant
[788] discard seat=0 source=Enraged Giant target=seat0
[789] state seat=0 source= target=seat0
```

</details>

#### Violation 14

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 15, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 5 real cards disappeared (expected 398, found 393)

<details>
<summary>Game State</summary>

```
Turn 15, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 790 events
  Seat 0 [alive]: life=40 library=88 hand=7 graveyard=3 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=89 hand=7 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=88 hand=4 graveyard=0 exile=1 battlefield=5 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=86 hand=3 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[770] priority_pass seat=2 source= target=seat0
[771] loop_shortcut seat=0 source=no_op_loop target=seat0
[772] phase_step seat=3 source= target=seat0
[773] declare_attackers seat=3 source= target=seat0
[774] blockers seat=1 source= target=seat0
[775] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[776] speed_advance seat=3 source= amount=1 target=seat0
[777] phase_step seat=3 source= target=seat0
[778] pool_drain seat=3 source= amount=2 target=seat0
[779] state seat=3 source= target=seat0
[780] turn_start seat=0 source= target=seat0
[781] untap_done seat=0 source=Plains target=seat0
[782] add_mana seat=0 source=Plains amount=1 target=seat0
[783] draw seat=0 source=Academy Rector amount=1 target=seat0
[784] phase_step seat=0 source= target=seat0
[785] phase_step seat=0 source= target=seat0
[786] pool_drain seat=0 source= amount=1 target=seat0
[787] zone_change seat=0 source=Enraged Giant
[788] discard seat=0 source=Enraged Giant target=seat0
[789] state seat=0 source= target=seat0
```

</details>

#### Violation 15

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 16, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 5 real cards disappeared (expected 398, found 393)

<details>
<summary>Game State</summary>

```
Turn 16, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 805 events
  Seat 0 [alive]: life=40 library=88 hand=7 graveyard=3 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=88 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=88 hand=4 graveyard=0 exile=1 battlefield=5 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=86 hand=3 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[785] phase_step seat=0 source= target=seat0
[786] pool_drain seat=0 source= amount=1 target=seat0
[787] zone_change seat=0 source=Enraged Giant
[788] discard seat=0 source=Enraged Giant target=seat0
[789] state seat=0 source= target=seat0
[790] turn_start seat=1 source= target=seat0
[791] untap_done seat=1 source=Swamp target=seat0
[792] untap_done seat=1 source=Baron Bertram Graywater target=seat0
[793] add_mana seat=1 source=Swamp amount=1 target=seat0
[794] draw seat=1 source=Wall of Shadows amount=1 target=seat0
[795] phase_step seat=1 source= target=seat0
[796] declare_attackers seat=1 source= target=seat0
[797] blockers seat=3 source= target=seat0
[798] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat3
[799] speed_advance seat=1 source= amount=2 target=seat0
[800] phase_step seat=1 source= target=seat0
[801] pool_drain seat=1 source= amount=1 target=seat0
[802] zone_change seat=1 source=Dragonscale General
[803] discard seat=1 source=Dragonscale General target=seat0
[804] state seat=1 source= target=seat0
```

</details>

#### Violation 16

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 16, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 5 real cards disappeared (expected 398, found 393)

<details>
<summary>Game State</summary>

```
Turn 16, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 805 events
  Seat 0 [alive]: life=40 library=88 hand=7 graveyard=3 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=88 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=88 hand=4 graveyard=0 exile=1 battlefield=5 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=86 hand=3 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[785] phase_step seat=0 source= target=seat0
[786] pool_drain seat=0 source= amount=1 target=seat0
[787] zone_change seat=0 source=Enraged Giant
[788] discard seat=0 source=Enraged Giant target=seat0
[789] state seat=0 source= target=seat0
[790] turn_start seat=1 source= target=seat0
[791] untap_done seat=1 source=Swamp target=seat0
[792] untap_done seat=1 source=Baron Bertram Graywater target=seat0
[793] add_mana seat=1 source=Swamp amount=1 target=seat0
[794] draw seat=1 source=Wall of Shadows amount=1 target=seat0
[795] phase_step seat=1 source= target=seat0
[796] declare_attackers seat=1 source= target=seat0
[797] blockers seat=3 source= target=seat0
[798] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat3
[799] speed_advance seat=1 source= amount=2 target=seat0
[800] phase_step seat=1 source= target=seat0
[801] pool_drain seat=1 source= amount=1 target=seat0
[802] zone_change seat=1 source=Dragonscale General
[803] discard seat=1 source=Dragonscale General target=seat0
[804] state seat=1 source= target=seat0
```

</details>

#### Violation 17

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 17, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 6 real cards disappeared (expected 398, found 392)

<details>
<summary>Game State</summary>

```
Turn 17, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 851 events
  Seat 0 [alive]: life=40 library=88 hand=7 graveyard=3 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=88 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=87 hand=3 graveyard=0 exile=1 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=86 hand=3 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[831] priority_pass seat=1 source= target=seat0
[832] priority_pass seat=3 source= target=seat0
[833] priority_pass seat=0 source= target=seat0
[834] priority_pass seat=1 source= target=seat0
[835] priority_pass seat=3 source= target=seat0
[836] priority_pass seat=0 source= target=seat0
[837] priority_pass seat=1 source= target=seat0
[838] priority_pass seat=3 source= target=seat0
[839] priority_pass seat=0 source= target=seat0
[840] priority_pass seat=1 source= target=seat0
[841] priority_pass seat=3 source= target=seat0
[842] priority_pass seat=0 source= target=seat0
[843] priority_pass seat=1 source= target=seat0
[844] priority_pass seat=3 source= target=seat0
[845] priority_pass seat=0 source= target=seat0
[846] priority_pass seat=1 source= target=seat0
[847] loop_shortcut seat=0 source=no_op_loop target=seat0
[848] phase_step seat=2 source= target=seat0
[849] phase_step seat=2 source= target=seat0
[850] state seat=2 source= target=seat0
```

</details>

#### Violation 18

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 17, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 6 real cards disappeared (expected 398, found 392)

<details>
<summary>Game State</summary>

```
Turn 17, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 851 events
  Seat 0 [alive]: life=40 library=88 hand=7 graveyard=3 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=88 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=87 hand=3 graveyard=0 exile=1 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=86 hand=3 graveyard=0 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[831] priority_pass seat=1 source= target=seat0
[832] priority_pass seat=3 source= target=seat0
[833] priority_pass seat=0 source= target=seat0
[834] priority_pass seat=1 source= target=seat0
[835] priority_pass seat=3 source= target=seat0
[836] priority_pass seat=0 source= target=seat0
[837] priority_pass seat=1 source= target=seat0
[838] priority_pass seat=3 source= target=seat0
[839] priority_pass seat=0 source= target=seat0
[840] priority_pass seat=1 source= target=seat0
[841] priority_pass seat=3 source= target=seat0
[842] priority_pass seat=0 source= target=seat0
[843] priority_pass seat=1 source= target=seat0
[844] priority_pass seat=3 source= target=seat0
[845] priority_pass seat=0 source= target=seat0
[846] priority_pass seat=1 source= target=seat0
[847] loop_shortcut seat=0 source=no_op_loop target=seat0
[848] phase_step seat=2 source= target=seat0
[849] phase_step seat=2 source= target=seat0
[850] state seat=2 source= target=seat0
```

</details>

#### Violation 19

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 18, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 6 real cards disappeared (expected 398, found 392)

<details>
<summary>Game State</summary>

```
Turn 18, Phase=ending Step=cleanup Active=seat3
Stack: 1 items, EventLog: 1467 events
  Seat 0 [alive]: life=40 library=88 hand=7 graveyard=3 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=88 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=87 hand=3 graveyard=0 exile=1 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=85 hand=3 graveyard=0 exile=0 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1447] priority_pass seat=2 source= target=seat0
[1448] priority_pass seat=0 source= target=seat0
[1449] priority_pass seat=1 source= target=seat0
[1450] priority_pass seat=2 source= target=seat0
[1451] priority_pass seat=0 source= target=seat0
[1452] priority_pass seat=1 source= target=seat0
[1453] priority_pass seat=2 source= target=seat0
[1454] priority_pass seat=0 source= target=seat0
[1455] priority_pass seat=1 source= target=seat0
[1456] priority_pass seat=2 source= target=seat0
[1457] priority_pass seat=0 source= target=seat0
[1458] priority_pass seat=1 source= target=seat0
[1459] priority_pass seat=2 source= target=seat0
[1460] priority_pass seat=0 source= target=seat0
[1461] priority_pass seat=1 source= target=seat0
[1462] priority_pass seat=2 source= target=seat0
[1463] priority_pass seat=0 source= target=seat0
[1464] priority_pass seat=1 source= target=seat0
[1465] priority_pass seat=2 source= target=seat0
[1466] state seat=3 source= target=seat0
```

</details>

#### Violation 20

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 18, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 6 real cards disappeared (expected 398, found 392)

<details>
<summary>Game State</summary>

```
Turn 18, Phase=ending Step=cleanup Active=seat3
Stack: 1 items, EventLog: 1467 events
  Seat 0 [alive]: life=40 library=88 hand=7 graveyard=3 exile=0 battlefield=1 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=88 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=87 hand=3 graveyard=0 exile=1 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=85 hand=3 graveyard=0 exile=0 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1447] priority_pass seat=2 source= target=seat0
[1448] priority_pass seat=0 source= target=seat0
[1449] priority_pass seat=1 source= target=seat0
[1450] priority_pass seat=2 source= target=seat0
[1451] priority_pass seat=0 source= target=seat0
[1452] priority_pass seat=1 source= target=seat0
[1453] priority_pass seat=2 source= target=seat0
[1454] priority_pass seat=0 source= target=seat0
[1455] priority_pass seat=1 source= target=seat0
[1456] priority_pass seat=2 source= target=seat0
[1457] priority_pass seat=0 source= target=seat0
[1458] priority_pass seat=1 source= target=seat0
[1459] priority_pass seat=2 source= target=seat0
[1460] priority_pass seat=0 source= target=seat0
[1461] priority_pass seat=1 source= target=seat0
[1462] priority_pass seat=2 source= target=seat0
[1463] priority_pass seat=0 source= target=seat0
[1464] priority_pass seat=1 source= target=seat0
[1465] priority_pass seat=2 source= target=seat0
[1466] state seat=3 source= target=seat0
```

</details>

#### Violation 21

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 19, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 6 real cards disappeared (expected 398, found 392)

<details>
<summary>Game State</summary>

```
Turn 19, Phase=ending Step=cleanup Active=seat0
Stack: 2 items, EventLog: 2444 events
  Seat 0 [alive]: life=40 library=87 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=88 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=87 hand=3 graveyard=0 exile=1 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=85 hand=3 graveyard=0 exile=0 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2424] priority_pass seat=3 source= target=seat0
[2425] priority_pass seat=1 source= target=seat0
[2426] priority_pass seat=2 source= target=seat0
[2427] priority_pass seat=3 source= target=seat0
[2428] priority_pass seat=1 source= target=seat0
[2429] priority_pass seat=2 source= target=seat0
[2430] priority_pass seat=3 source= target=seat0
[2431] priority_pass seat=1 source= target=seat0
[2432] priority_pass seat=2 source= target=seat0
[2433] priority_pass seat=3 source= target=seat0
[2434] priority_pass seat=1 source= target=seat0
[2435] priority_pass seat=2 source= target=seat0
[2436] priority_pass seat=3 source= target=seat0
[2437] priority_pass seat=1 source= target=seat0
[2438] priority_pass seat=2 source= target=seat0
[2439] priority_pass seat=3 source= target=seat0
[2440] priority_pass seat=1 source= target=seat0
[2441] priority_pass seat=2 source= target=seat0
[2442] priority_pass seat=3 source= target=seat0
[2443] state seat=0 source= target=seat0
```

</details>

#### Violation 22

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 19, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 6 real cards disappeared (expected 398, found 392)

<details>
<summary>Game State</summary>

```
Turn 19, Phase=ending Step=cleanup Active=seat0
Stack: 2 items, EventLog: 2444 events
  Seat 0 [alive]: life=40 library=87 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=88 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
  Seat 2 [alive]: life=40 library=87 hand=3 graveyard=0 exile=1 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=85 hand=3 graveyard=0 exile=0 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2424] priority_pass seat=3 source= target=seat0
[2425] priority_pass seat=1 source= target=seat0
[2426] priority_pass seat=2 source= target=seat0
[2427] priority_pass seat=3 source= target=seat0
[2428] priority_pass seat=1 source= target=seat0
[2429] priority_pass seat=2 source= target=seat0
[2430] priority_pass seat=3 source= target=seat0
[2431] priority_pass seat=1 source= target=seat0
[2432] priority_pass seat=2 source= target=seat0
[2433] priority_pass seat=3 source= target=seat0
[2434] priority_pass seat=1 source= target=seat0
[2435] priority_pass seat=2 source= target=seat0
[2436] priority_pass seat=3 source= target=seat0
[2437] priority_pass seat=1 source= target=seat0
[2438] priority_pass seat=2 source= target=seat0
[2439] priority_pass seat=3 source= target=seat0
[2440] priority_pass seat=1 source= target=seat0
[2441] priority_pass seat=2 source= target=seat0
[2442] priority_pass seat=3 source= target=seat0
[2443] state seat=0 source= target=seat0
```

</details>

#### Violation 23

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 20, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 9 real cards disappeared (expected 398, found 389)

<details>
<summary>Game State</summary>

```
Turn 20, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2873 events
  Seat 0 [alive]: life=40 library=87 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=87 hand=6 graveyard=3 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=37 library=87 hand=3 graveyard=0 exile=1 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=85 hand=3 graveyard=0 exile=0 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2853] priority_pass seat=2 source= target=seat0
[2854] priority_pass seat=3 source= target=seat0
[2855] priority_pass seat=0 source= target=seat0
[2856] priority_pass seat=2 source= target=seat0
[2857] priority_pass seat=3 source= target=seat0
[2858] priority_pass seat=0 source= target=seat0
[2859] priority_pass seat=2 source= target=seat0
[2860] priority_pass seat=3 source= target=seat0
[2861] priority_pass seat=0 source= target=seat0
[2862] priority_pass seat=2 source= target=seat0
[2863] priority_pass seat=3 source= target=seat0
[2864] priority_pass seat=0 source= target=seat0
[2865] loop_shortcut seat=0 source=no_op_loop target=seat0
[2866] phase_step seat=1 source= target=seat0
[2867] declare_attackers seat=1 source= target=seat0
[2868] blockers seat=2 source= target=seat0
[2869] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat2
[2870] speed_advance seat=1 source= amount=3 target=seat0
[2871] phase_step seat=1 source= target=seat0
[2872] state seat=1 source= target=seat0
```

</details>

#### Violation 24

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 20, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 9 real cards disappeared (expected 398, found 389)

<details>
<summary>Game State</summary>

```
Turn 20, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2873 events
  Seat 0 [alive]: life=40 library=87 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=87 hand=6 graveyard=3 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=37 library=87 hand=3 graveyard=0 exile=1 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=85 hand=3 graveyard=0 exile=0 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2853] priority_pass seat=2 source= target=seat0
[2854] priority_pass seat=3 source= target=seat0
[2855] priority_pass seat=0 source= target=seat0
[2856] priority_pass seat=2 source= target=seat0
[2857] priority_pass seat=3 source= target=seat0
[2858] priority_pass seat=0 source= target=seat0
[2859] priority_pass seat=2 source= target=seat0
[2860] priority_pass seat=3 source= target=seat0
[2861] priority_pass seat=0 source= target=seat0
[2862] priority_pass seat=2 source= target=seat0
[2863] priority_pass seat=3 source= target=seat0
[2864] priority_pass seat=0 source= target=seat0
[2865] loop_shortcut seat=0 source=no_op_loop target=seat0
[2866] phase_step seat=1 source= target=seat0
[2867] declare_attackers seat=1 source= target=seat0
[2868] blockers seat=2 source= target=seat0
[2869] damage seat=1 source=Baron Bertram Graywater amount=3 target=seat2
[2870] speed_advance seat=1 source= amount=3 target=seat0
[2871] phase_step seat=1 source= target=seat0
[2872] state seat=1 source= target=seat0
```

</details>

#### Violation 25

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 21, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 12 real cards disappeared (expected 398, found 386)

<details>
<summary>Game State</summary>

```
Turn 21, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2983 events
  Seat 0 [alive]: life=40 library=87 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=87 hand=6 graveyard=3 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=37 library=86 hand=0 graveyard=0 exile=1 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Winding Canyons (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=85 hand=3 graveyard=0 exile=0 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2963] priority_pass seat=1 source= target=seat0
[2964] priority_pass seat=3 source= target=seat0
[2965] priority_pass seat=0 source= target=seat0
[2966] priority_pass seat=1 source= target=seat0
[2967] priority_pass seat=3 source= target=seat0
[2968] priority_pass seat=0 source= target=seat0
[2969] priority_pass seat=1 source= target=seat0
[2970] priority_pass seat=3 source= target=seat0
[2971] priority_pass seat=0 source= target=seat0
[2972] priority_pass seat=1 source= target=seat0
[2973] priority_pass seat=3 source= target=seat0
[2974] priority_pass seat=0 source= target=seat0
[2975] priority_pass seat=1 source= target=seat0
[2976] priority_pass seat=3 source= target=seat0
[2977] priority_pass seat=0 source= target=seat0
[2978] priority_pass seat=1 source= target=seat0
[2979] loop_shortcut seat=0 source=no_op_loop target=seat0
[2980] phase_step seat=2 source= target=seat0
[2981] phase_step seat=2 source= target=seat0
[2982] state seat=2 source= target=seat0
```

</details>

#### Violation 26

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 21, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 12 real cards disappeared (expected 398, found 386)

<details>
<summary>Game State</summary>

```
Turn 21, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2983 events
  Seat 0 [alive]: life=40 library=87 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=87 hand=6 graveyard=3 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=37 library=86 hand=0 graveyard=0 exile=1 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Winding Canyons (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=85 hand=3 graveyard=0 exile=0 battlefield=6 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2963] priority_pass seat=1 source= target=seat0
[2964] priority_pass seat=3 source= target=seat0
[2965] priority_pass seat=0 source= target=seat0
[2966] priority_pass seat=1 source= target=seat0
[2967] priority_pass seat=3 source= target=seat0
[2968] priority_pass seat=0 source= target=seat0
[2969] priority_pass seat=1 source= target=seat0
[2970] priority_pass seat=3 source= target=seat0
[2971] priority_pass seat=0 source= target=seat0
[2972] priority_pass seat=1 source= target=seat0
[2973] priority_pass seat=3 source= target=seat0
[2974] priority_pass seat=0 source= target=seat0
[2975] priority_pass seat=1 source= target=seat0
[2976] priority_pass seat=3 source= target=seat0
[2977] priority_pass seat=0 source= target=seat0
[2978] priority_pass seat=1 source= target=seat0
[2979] loop_shortcut seat=0 source=no_op_loop target=seat0
[2980] phase_step seat=2 source= target=seat0
[2981] phase_step seat=2 source= target=seat0
[2982] state seat=2 source= target=seat0
```

</details>

#### Violation 27

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 22, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 13 real cards disappeared (expected 398, found 385)

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 3037 events
  Seat 0 [alive]: life=40 library=87 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=34 library=87 hand=6 graveyard=3 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=37 library=86 hand=0 graveyard=0 exile=1 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Winding Canyons (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=84 hand=2 graveyard=0 exile=0 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3017] priority_pass seat=1 source= target=seat0
[3018] priority_pass seat=2 source= target=seat0
[3019] priority_pass seat=0 source= target=seat0
[3020] priority_pass seat=1 source= target=seat0
[3021] priority_pass seat=2 source= target=seat0
[3022] priority_pass seat=0 source= target=seat0
[3023] priority_pass seat=1 source= target=seat0
[3024] priority_pass seat=2 source= target=seat0
[3025] priority_pass seat=0 source= target=seat0
[3026] priority_pass seat=1 source= target=seat0
[3027] priority_pass seat=2 source= target=seat0
[3028] loop_shortcut seat=0 source=no_op_loop target=seat0
[3029] phase_step seat=3 source= target=seat0
[3030] declare_attackers seat=3 source= target=seat0
[3031] blockers seat=1 source= target=seat0
[3032] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[3033] speed_advance seat=3 source= amount=3 target=seat0
[3034] phase_step seat=3 source= target=seat0
[3035] pool_drain seat=3 source= amount=1 target=seat0
[3036] state seat=3 source= target=seat0
```

</details>

#### Violation 28

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 22, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 13 real cards disappeared (expected 398, found 385)

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 3037 events
  Seat 0 [alive]: life=40 library=87 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=34 library=87 hand=6 graveyard=3 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=37 library=86 hand=0 graveyard=0 exile=1 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Winding Canyons (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=84 hand=2 graveyard=0 exile=0 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3017] priority_pass seat=1 source= target=seat0
[3018] priority_pass seat=2 source= target=seat0
[3019] priority_pass seat=0 source= target=seat0
[3020] priority_pass seat=1 source= target=seat0
[3021] priority_pass seat=2 source= target=seat0
[3022] priority_pass seat=0 source= target=seat0
[3023] priority_pass seat=1 source= target=seat0
[3024] priority_pass seat=2 source= target=seat0
[3025] priority_pass seat=0 source= target=seat0
[3026] priority_pass seat=1 source= target=seat0
[3027] priority_pass seat=2 source= target=seat0
[3028] loop_shortcut seat=0 source=no_op_loop target=seat0
[3029] phase_step seat=3 source= target=seat0
[3030] declare_attackers seat=3 source= target=seat0
[3031] blockers seat=1 source= target=seat0
[3032] damage seat=3 source=Friendly Teddy amount=2 target=seat1
[3033] speed_advance seat=3 source= amount=3 target=seat0
[3034] phase_step seat=3 source= target=seat0
[3035] pool_drain seat=3 source= amount=1 target=seat0
[3036] state seat=3 source= target=seat0
```

</details>

#### Violation 29

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 23, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 14 real cards disappeared (expected 398, found 384)

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3077 events
  Seat 0 [alive]: life=40 library=86 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=34 library=87 hand=6 graveyard=3 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=37 library=86 hand=0 graveyard=0 exile=1 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Winding Canyons (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=84 hand=2 graveyard=0 exile=0 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3057] priority_pass seat=1 source= target=seat0
[3058] priority_pass seat=2 source= target=seat0
[3059] priority_pass seat=3 source= target=seat0
[3060] priority_pass seat=1 source= target=seat0
[3061] priority_pass seat=2 source= target=seat0
[3062] priority_pass seat=3 source= target=seat0
[3063] priority_pass seat=1 source= target=seat0
[3064] priority_pass seat=2 source= target=seat0
[3065] priority_pass seat=3 source= target=seat0
[3066] priority_pass seat=1 source= target=seat0
[3067] priority_pass seat=2 source= target=seat0
[3068] priority_pass seat=3 source= target=seat0
[3069] priority_pass seat=1 source= target=seat0
[3070] priority_pass seat=2 source= target=seat0
[3071] priority_pass seat=3 source= target=seat0
[3072] loop_shortcut seat=0 source=no_op_loop target=seat0
[3073] draw seat=0 source=Three Bowls of Porridge amount=1 target=seat0
[3074] phase_step seat=0 source= target=seat0
[3075] phase_step seat=0 source= target=seat0
[3076] state seat=0 source= target=seat0
```

</details>

#### Violation 30

- **Game**: 420 (seed 4200042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 23, Phase=ending Step=cleanup
- **Commanders**: Breya, Etherium Shaper, Baron Bertram Graywater, Alela, Cunning Conqueror, SP//dr, Piloted by Peni
- **Message**: zone conservation violated: 14 real cards disappeared (expected 398, found 384)

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3077 events
  Seat 0 [alive]: life=40 library=86 hand=7 graveyard=3 exile=0 battlefield=2 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=34 library=87 hand=6 graveyard=3 exile=0 battlefield=3 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Baron Bertram Graywater (P/T 3/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=37 library=86 hand=0 graveyard=0 exile=1 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Unholy Indenture (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Winding Canyons (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=34 library=84 hand=2 graveyard=0 exile=0 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Misty Rainforest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3057] priority_pass seat=1 source= target=seat0
[3058] priority_pass seat=2 source= target=seat0
[3059] priority_pass seat=3 source= target=seat0
[3060] priority_pass seat=1 source= target=seat0
[3061] priority_pass seat=2 source= target=seat0
[3062] priority_pass seat=3 source= target=seat0
[3063] priority_pass seat=1 source= target=seat0
[3064] priority_pass seat=2 source= target=seat0
[3065] priority_pass seat=3 source= target=seat0
[3066] priority_pass seat=1 source= target=seat0
[3067] priority_pass seat=2 source= target=seat0
[3068] priority_pass seat=3 source= target=seat0
[3069] priority_pass seat=1 source= target=seat0
[3070] priority_pass seat=2 source= target=seat0
[3071] priority_pass seat=3 source= target=seat0
[3072] loop_shortcut seat=0 source=no_op_loop target=seat0
[3073] draw seat=0 source=Three Bowls of Porridge amount=1 target=seat0
[3074] phase_step seat=0 source= target=seat0
[3075] phase_step seat=0 source= target=seat0
[3076] state seat=0 source= target=seat0
```

</details>

*... and 1087 more violations not shown.*

## Invariant Violations (Nightmare Boards)

| Invariant | Count |
|-----------|-------|
| CardIdentity | 6 |

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Nevinyrral, Urborg Tyrant | 7 | 6 | 0.54 |
| 2 | Scarland Thrinax | 1 | 2 | 0.33 |
| 3 | Jorubai Murk Lurker | 1 | 2 | 0.33 |
| 4 | Glacierwood Siege | 1 | 2 | 0.33 |
| 5 | Revel of the Fallen God | 1 | 2 | 0.33 |
| 6 | Finest Hour | 1 | 2 | 0.33 |
| 7 | Horrid Shadowspinner | 1 | 2 | 0.33 |
| 8 | Ashiok, Dream Render | 1 | 2 | 0.33 |
| 9 | Baleful Strix | 2 | 4 | 0.33 |
| 10 | A-Shattered Seraph | 1 | 2 | 0.33 |

## Verdict: ISSUES FOUND

**1123 total issues** across 5000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 1117 invariant violations in chaos games
- 0 crashes in nightmare boards
- 6 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
