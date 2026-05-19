# Chaos Gauntlet Report

Generated: 2026-05-19T09:57:35-07:00

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
| Duration | 2m51.083s |
| Throughput | 29 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 1113 (in 50 games) |
| Clean Games | 4950 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 2.173s |
| Throughput | 4603 boards/sec |
| Crashes | 0 |
| Invariant Violations | 6 |
| Clean Boards | 9997 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| CardIdentity | 410 |
| ZoneConservation | 664 |
| TriggerCompleteness | 8 |
| ZoneCastGrantExpiry | 8 |
| AttachmentConsistency | 23 |

### Violation Details (first 30)

#### Violation 1

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 3223 events
  Seat 0 [alive]: life=35 library=79 hand=4 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=24 library=79 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=77 hand=6 graveyard=7 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=19 library=79 hand=0 graveyard=8 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Storm Fleet Aerialist (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[3203] stack_resolve seat=1 source=Disruptive Pitmage target=seat0
[3204] commit_crime seat=1 source=Disruptive Pitmage amount=1 target=seat0
[3205] counter_spell seat=1 source=generic_counter target=seat2
[3206] enter_battlefield seat=1 source=Disruptive Pitmage target=seat0
[3207] trigger_evaluated seat=1 source=Roil Elemental
[3208] stack_push seat=1 source=Roil Elemental target=seat0
[3209] triggered_ability seat=1 source=Roil Elemental target=seat0
[3210] priority_pass seat=2 source= target=seat0
[3211] priority_pass seat=3 source= target=seat0
[3212] priority_pass seat=0 source= target=seat0
[3213] stack_resolve seat=1 source=Roil Elemental target=seat0
[3214] priority_pass seat=3 source= target=seat0
[3215] priority_pass seat=0 source= target=seat0
[3216] priority_pass seat=1 source= target=seat0
[3217] stack_resolve seat=2 source=Ice Out target=seat0
[3218] zone_change seat=2 source=Ice Out
[3219] resolve seat=2 source=Ice Out target=seat0
[3220] phase_step seat=2 source= target=seat0
[3221] phase_step seat=2 source= target=seat0
[3222] state seat=2 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 3223 events
  Seat 0 [alive]: life=35 library=79 hand=4 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=24 library=79 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=77 hand=6 graveyard=7 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=19 library=79 hand=0 graveyard=8 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Storm Fleet Aerialist (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[3203] stack_resolve seat=1 source=Disruptive Pitmage target=seat0
[3204] commit_crime seat=1 source=Disruptive Pitmage amount=1 target=seat0
[3205] counter_spell seat=1 source=generic_counter target=seat2
[3206] enter_battlefield seat=1 source=Disruptive Pitmage target=seat0
[3207] trigger_evaluated seat=1 source=Roil Elemental
[3208] stack_push seat=1 source=Roil Elemental target=seat0
[3209] triggered_ability seat=1 source=Roil Elemental target=seat0
[3210] priority_pass seat=2 source= target=seat0
[3211] priority_pass seat=3 source= target=seat0
[3212] priority_pass seat=0 source= target=seat0
[3213] stack_resolve seat=1 source=Roil Elemental target=seat0
[3214] priority_pass seat=3 source= target=seat0
[3215] priority_pass seat=0 source= target=seat0
[3216] priority_pass seat=1 source= target=seat0
[3217] stack_resolve seat=2 source=Ice Out target=seat0
[3218] zone_change seat=2 source=Ice Out
[3219] resolve seat=2 source=Ice Out target=seat0
[3220] phase_step seat=2 source= target=seat0
[3221] phase_step seat=2 source= target=seat0
[3222] state seat=2 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 53, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 53, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 3475 events
  Seat 0 [alive]: life=35 library=79 hand=4 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=21 library=79 hand=5 graveyard=4 exile=0 battlefield=8 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=77 hand=6 graveyard=7 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=19 library=78 hand=0 graveyard=10 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3455] tutor seat=3 source=generic_tutor target=seat0
[3456] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3457] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[3458] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[3459] priority_pass seat=0 source= target=seat0
[3460] priority_pass seat=1 source= target=seat0
[3461] priority_pass seat=2 source= target=seat0
[3462] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[3463] tutor seat=3 source=generic_tutor target=seat0
[3464] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3465] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[3466] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[3467] priority_pass seat=0 source= target=seat0
[3468] priority_pass seat=1 source= target=seat0
[3469] priority_pass seat=2 source= target=seat0
[3470] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[3471] tutor seat=3 source=generic_tutor target=seat0
[3472] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3473] damage_wears_off seat=1 source=Roil Elemental amount=1 target=seat0
[3474] state seat=3 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 53, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 53, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 3475 events
  Seat 0 [alive]: life=35 library=79 hand=4 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=21 library=79 hand=5 graveyard=4 exile=0 battlefield=8 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=77 hand=6 graveyard=7 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=19 library=78 hand=0 graveyard=10 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3455] tutor seat=3 source=generic_tutor target=seat0
[3456] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3457] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[3458] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[3459] priority_pass seat=0 source= target=seat0
[3460] priority_pass seat=1 source= target=seat0
[3461] priority_pass seat=2 source= target=seat0
[3462] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[3463] tutor seat=3 source=generic_tutor target=seat0
[3464] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3465] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[3466] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[3467] priority_pass seat=0 source= target=seat0
[3468] priority_pass seat=1 source= target=seat0
[3469] priority_pass seat=2 source= target=seat0
[3470] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[3471] tutor seat=3 source=generic_tutor target=seat0
[3472] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3473] damage_wears_off seat=1 source=Roil Elemental amount=1 target=seat0
[3474] state seat=3 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 54, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 54, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3480 events
  Seat 0 [alive]: life=35 library=78 hand=5 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=21 library=79 hand=5 graveyard=4 exile=0 battlefield=8 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=77 hand=6 graveyard=7 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=19 library=78 hand=0 graveyard=10 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3460] priority_pass seat=1 source= target=seat0
[3461] priority_pass seat=2 source= target=seat0
[3462] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[3463] tutor seat=3 source=generic_tutor target=seat0
[3464] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3465] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[3466] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[3467] priority_pass seat=0 source= target=seat0
[3468] priority_pass seat=1 source= target=seat0
[3469] priority_pass seat=2 source= target=seat0
[3470] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[3471] tutor seat=3 source=generic_tutor target=seat0
[3472] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3473] damage_wears_off seat=1 source=Roil Elemental amount=1 target=seat0
[3474] state seat=3 source= target=seat0
[3475] turn_start seat=0 source= target=seat0
[3476] draw seat=0 source=The Mana Rig amount=1 target=seat0
[3477] phase_step seat=0 source= target=seat0
[3478] phase_step seat=0 source= target=seat0
[3479] state seat=0 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 54, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 54, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3480 events
  Seat 0 [alive]: life=35 library=78 hand=5 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=21 library=79 hand=5 graveyard=4 exile=0 battlefield=8 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=77 hand=6 graveyard=7 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=19 library=78 hand=0 graveyard=10 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3460] priority_pass seat=1 source= target=seat0
[3461] priority_pass seat=2 source= target=seat0
[3462] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[3463] tutor seat=3 source=generic_tutor target=seat0
[3464] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3465] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[3466] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[3467] priority_pass seat=0 source= target=seat0
[3468] priority_pass seat=1 source= target=seat0
[3469] priority_pass seat=2 source= target=seat0
[3470] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[3471] tutor seat=3 source=generic_tutor target=seat0
[3472] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[3473] damage_wears_off seat=1 source=Roil Elemental amount=1 target=seat0
[3474] state seat=3 source= target=seat0
[3475] turn_start seat=0 source= target=seat0
[3476] draw seat=0 source=The Mana Rig amount=1 target=seat0
[3477] phase_step seat=0 source= target=seat0
[3478] phase_step seat=0 source= target=seat0
[3479] state seat=0 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 55, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 55, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 3755 events
  Seat 0 [alive]: life=35 library=78 hand=5 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=21 library=78 hand=2 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=3
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0)
    - Howling Galefang (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=32 library=77 hand=6 graveyard=7 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=78 hand=0 graveyard=10 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3735] add_mana seat=1 source=Forest amount=1 target=seat0
[3736] activate_ability seat=1 source=Silvanus's Invoker target=seat0
[3737] stack_push seat=1 source=Silvanus's Invoker target=seat0
[3738] priority_pass seat=2 source= target=seat0
[3739] priority_pass seat=3 source= target=seat0
[3740] priority_pass seat=0 source= target=seat0
[3741] stack_resolve seat=1 source=Silvanus's Invoker target=seat0
[3742] untap seat=0 source=Silvanus's Invoker target=seat0
[3743] activated_ability_resolved seat=1 source=Silvanus's Invoker target=seat0
[3744] add_mana seat=1 source=Forest amount=1 target=seat0
[3745] activate_ability seat=1 source=Silvanus's Invoker target=seat0
[3746] stack_push seat=1 source=Silvanus's Invoker target=seat0
[3747] priority_pass seat=2 source= target=seat0
[3748] priority_pass seat=3 source= target=seat0
[3749] priority_pass seat=0 source= target=seat0
[3750] stack_resolve seat=1 source=Silvanus's Invoker target=seat0
[3751] untap seat=0 source=Silvanus's Invoker target=seat0
[3752] activated_ability_resolved seat=1 source=Silvanus's Invoker target=seat0
[3753] add_mana seat=1 source=Forest amount=1 target=seat0
[3754] state seat=1 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 55, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 55, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 3755 events
  Seat 0 [alive]: life=35 library=78 hand=5 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=21 library=78 hand=2 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=3
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0)
    - Howling Galefang (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=32 library=77 hand=6 graveyard=7 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=78 hand=0 graveyard=10 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3735] add_mana seat=1 source=Forest amount=1 target=seat0
[3736] activate_ability seat=1 source=Silvanus's Invoker target=seat0
[3737] stack_push seat=1 source=Silvanus's Invoker target=seat0
[3738] priority_pass seat=2 source= target=seat0
[3739] priority_pass seat=3 source= target=seat0
[3740] priority_pass seat=0 source= target=seat0
[3741] stack_resolve seat=1 source=Silvanus's Invoker target=seat0
[3742] untap seat=0 source=Silvanus's Invoker target=seat0
[3743] activated_ability_resolved seat=1 source=Silvanus's Invoker target=seat0
[3744] add_mana seat=1 source=Forest amount=1 target=seat0
[3745] activate_ability seat=1 source=Silvanus's Invoker target=seat0
[3746] stack_push seat=1 source=Silvanus's Invoker target=seat0
[3747] priority_pass seat=2 source= target=seat0
[3748] priority_pass seat=3 source= target=seat0
[3749] priority_pass seat=0 source= target=seat0
[3750] stack_resolve seat=1 source=Silvanus's Invoker target=seat0
[3751] untap seat=0 source=Silvanus's Invoker target=seat0
[3752] activated_ability_resolved seat=1 source=Silvanus's Invoker target=seat0
[3753] add_mana seat=1 source=Forest amount=1 target=seat0
[3754] state seat=1 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 56, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 56, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 3820 events
  Seat 0 [alive]: life=35 library=78 hand=5 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=21 library=78 hand=2 graveyard=5 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0)
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=76 hand=5 graveyard=8 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=78 hand=0 graveyard=10 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3800] stack_resolve seat=1 source=Disruptive Pitmage target=seat0
[3801] commit_crime seat=1 source=Disruptive Pitmage amount=2 target=seat0
[3802] counter_spell seat=1 source=generic_counter target=seat2
[3803] enter_battlefield seat=1 source=Disruptive Pitmage target=seat0
[3804] trigger_evaluated seat=1 source=Roil Elemental
[3805] stack_push seat=1 source=Roil Elemental target=seat0
[3806] triggered_ability seat=1 source=Roil Elemental target=seat0
[3807] priority_pass seat=2 source= target=seat0
[3808] priority_pass seat=3 source= target=seat0
[3809] priority_pass seat=0 source= target=seat0
[3810] stack_resolve seat=1 source=Roil Elemental target=seat0
[3811] priority_pass seat=3 source= target=seat0
[3812] priority_pass seat=0 source= target=seat0
[3813] priority_pass seat=1 source= target=seat0
[3814] stack_resolve seat=2 source=Genestealer Locus target=seat0
[3815] zone_change seat=2 source=Genestealer Locus
[3816] resolve seat=2 source=Genestealer Locus target=seat0
[3817] phase_step seat=2 source= target=seat0
[3818] phase_step seat=2 source= target=seat0
[3819] state seat=2 source= target=seat0
```

</details>

#### Violation 10

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 56, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 56, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 3820 events
  Seat 0 [alive]: life=35 library=78 hand=5 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=21 library=78 hand=2 graveyard=5 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0)
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=76 hand=5 graveyard=8 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=78 hand=0 graveyard=10 exile=0 battlefield=24 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3800] stack_resolve seat=1 source=Disruptive Pitmage target=seat0
[3801] commit_crime seat=1 source=Disruptive Pitmage amount=2 target=seat0
[3802] counter_spell seat=1 source=generic_counter target=seat2
[3803] enter_battlefield seat=1 source=Disruptive Pitmage target=seat0
[3804] trigger_evaluated seat=1 source=Roil Elemental
[3805] stack_push seat=1 source=Roil Elemental target=seat0
[3806] triggered_ability seat=1 source=Roil Elemental target=seat0
[3807] priority_pass seat=2 source= target=seat0
[3808] priority_pass seat=3 source= target=seat0
[3809] priority_pass seat=0 source= target=seat0
[3810] stack_resolve seat=1 source=Roil Elemental target=seat0
[3811] priority_pass seat=3 source= target=seat0
[3812] priority_pass seat=0 source= target=seat0
[3813] priority_pass seat=1 source= target=seat0
[3814] stack_resolve seat=2 source=Genestealer Locus target=seat0
[3815] zone_change seat=2 source=Genestealer Locus
[3816] resolve seat=2 source=Genestealer Locus target=seat0
[3817] phase_step seat=2 source= target=seat0
[3818] phase_step seat=2 source= target=seat0
[3819] state seat=2 source= target=seat0
```

</details>

#### Violation 11

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 57, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 57, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 4065 events
  Seat 0 [alive]: life=35 library=78 hand=5 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=17 library=78 hand=2 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0)
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=76 hand=5 graveyard=8 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=77 hand=0 graveyard=10 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Fire Nation Warship (P/T 4/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4045] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[4046] tutor seat=3 source=generic_tutor target=seat0
[4047] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[4048] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[4049] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[4050] priority_pass seat=0 source= target=seat0
[4051] priority_pass seat=1 source= target=seat0
[4052] priority_pass seat=2 source= target=seat0
[4053] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[4054] tutor seat=3 source=generic_tutor target=seat0
[4055] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[4056] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[4057] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[4058] priority_pass seat=0 source= target=seat0
[4059] priority_pass seat=1 source= target=seat0
[4060] priority_pass seat=2 source= target=seat0
[4061] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[4062] tutor seat=3 source=generic_tutor target=seat0
[4063] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[4064] state seat=3 source= target=seat0
```

</details>

#### Violation 12

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 57, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 57, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 4065 events
  Seat 0 [alive]: life=35 library=78 hand=5 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=17 library=78 hand=2 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0)
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=76 hand=5 graveyard=8 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=77 hand=0 graveyard=10 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Fire Nation Warship (P/T 4/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4045] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[4046] tutor seat=3 source=generic_tutor target=seat0
[4047] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[4048] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[4049] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[4050] priority_pass seat=0 source= target=seat0
[4051] priority_pass seat=1 source= target=seat0
[4052] priority_pass seat=2 source= target=seat0
[4053] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[4054] tutor seat=3 source=generic_tutor target=seat0
[4055] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[4056] activate_ability seat=3 source=Khalni Heart Expedition target=seat0
[4057] stack_push seat=3 source=Khalni Heart Expedition target=seat0
[4058] priority_pass seat=0 source= target=seat0
[4059] priority_pass seat=1 source= target=seat0
[4060] priority_pass seat=2 source= target=seat0
[4061] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[4062] tutor seat=3 source=generic_tutor target=seat0
[4063] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[4064] state seat=3 source= target=seat0
```

</details>

#### Violation 13

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 58, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 58, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 4080 events
  Seat 0 [alive]: life=35 library=77 hand=5 graveyard=10 exile=0 battlefield=1 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=17 library=78 hand=2 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0)
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=76 hand=5 graveyard=8 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=77 hand=0 graveyard=10 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Fire Nation Warship (P/T 4/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4060] priority_pass seat=2 source= target=seat0
[4061] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[4062] tutor seat=3 source=generic_tutor target=seat0
[4063] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[4064] state seat=3 source= target=seat0
[4065] turn_start seat=0 source= target=seat0
[4066] draw seat=0 source=Swamp amount=1 target=seat0
[4067] play_land seat=0 source=Swamp target=seat0
[4068] trigger_evaluated seat=1 source=Roil Elemental
[4069] stack_push seat=1 source=Roil Elemental target=seat0
[4070] triggered_ability seat=1 source=Roil Elemental target=seat0
[4071] priority_pass seat=0 source= target=seat0
[4072] priority_pass seat=2 source= target=seat0
[4073] priority_pass seat=3 source= target=seat0
[4074] stack_resolve seat=1 source=Roil Elemental target=seat0
[4075] add_mana seat=0 source=Swamp amount=1 target=seat0
[4076] phase_step seat=0 source= target=seat0
[4077] phase_step seat=0 source= target=seat0
[4078] pool_drain seat=0 source= amount=1 target=seat0
[4079] state seat=0 source= target=seat0
```

</details>

#### Violation 14

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 58, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 58, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 4080 events
  Seat 0 [alive]: life=35 library=77 hand=5 graveyard=10 exile=0 battlefield=1 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=17 library=78 hand=2 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0)
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=76 hand=5 graveyard=8 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=77 hand=0 graveyard=10 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Fire Nation Warship (P/T 4/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4060] priority_pass seat=2 source= target=seat0
[4061] stack_resolve seat=3 source=Khalni Heart Expedition target=seat0
[4062] tutor seat=3 source=generic_tutor target=seat0
[4063] activated_ability_resolved seat=3 source=Khalni Heart Expedition target=seat0
[4064] state seat=3 source= target=seat0
[4065] turn_start seat=0 source= target=seat0
[4066] draw seat=0 source=Swamp amount=1 target=seat0
[4067] play_land seat=0 source=Swamp target=seat0
[4068] trigger_evaluated seat=1 source=Roil Elemental
[4069] stack_push seat=1 source=Roil Elemental target=seat0
[4070] triggered_ability seat=1 source=Roil Elemental target=seat0
[4071] priority_pass seat=0 source= target=seat0
[4072] priority_pass seat=2 source= target=seat0
[4073] priority_pass seat=3 source= target=seat0
[4074] stack_resolve seat=1 source=Roil Elemental target=seat0
[4075] add_mana seat=0 source=Swamp amount=1 target=seat0
[4076] phase_step seat=0 source= target=seat0
[4077] phase_step seat=0 source= target=seat0
[4078] pool_drain seat=0 source= amount=1 target=seat0
[4079] state seat=0 source= target=seat0
```

</details>

#### Violation 15

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 59, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 59, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 4361 events
  Seat 0 [alive]: life=20 library=77 hand=5 graveyard=10 exile=0 battlefield=1 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=17 library=77 hand=1 graveyard=5 exile=0 battlefield=13 cmdzone=0 mana=3
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0) [T]
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Slagwurm Armor (P/T 0/0, dmg=0)
    - Weapon Rack (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=32 library=76 hand=5 graveyard=8 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=77 hand=0 graveyard=10 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Fire Nation Warship (P/T 4/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4341] add_mana seat=1 source=Forest amount=1 target=seat0
[4342] activate_ability seat=1 source=Silvanus's Invoker target=seat0
[4343] stack_push seat=1 source=Silvanus's Invoker target=seat0
[4344] priority_pass seat=2 source= target=seat0
[4345] priority_pass seat=3 source= target=seat0
[4346] priority_pass seat=0 source= target=seat0
[4347] stack_resolve seat=1 source=Silvanus's Invoker target=seat0
[4348] untap seat=0 source=Silvanus's Invoker target=seat0
[4349] activated_ability_resolved seat=1 source=Silvanus's Invoker target=seat0
[4350] add_mana seat=1 source=Forest amount=1 target=seat0
[4351] activate_ability seat=1 source=Silvanus's Invoker target=seat0
[4352] stack_push seat=1 source=Silvanus's Invoker target=seat0
[4353] priority_pass seat=2 source= target=seat0
[4354] priority_pass seat=3 source= target=seat0
[4355] priority_pass seat=0 source= target=seat0
[4356] stack_resolve seat=1 source=Silvanus's Invoker target=seat0
[4357] untap seat=0 source=Silvanus's Invoker target=seat0
[4358] activated_ability_resolved seat=1 source=Silvanus's Invoker target=seat0
[4359] add_mana seat=1 source=Forest amount=1 target=seat0
[4360] state seat=1 source= target=seat0
```

</details>

#### Violation 16

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 59, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 59, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 4361 events
  Seat 0 [alive]: life=20 library=77 hand=5 graveyard=10 exile=0 battlefield=1 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=17 library=77 hand=1 graveyard=5 exile=0 battlefield=13 cmdzone=0 mana=3
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0) [T]
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Slagwurm Armor (P/T 0/0, dmg=0)
    - Weapon Rack (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=32 library=76 hand=5 graveyard=8 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=10 library=77 hand=0 graveyard=10 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Fire Nation Warship (P/T 4/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4341] add_mana seat=1 source=Forest amount=1 target=seat0
[4342] activate_ability seat=1 source=Silvanus's Invoker target=seat0
[4343] stack_push seat=1 source=Silvanus's Invoker target=seat0
[4344] priority_pass seat=2 source= target=seat0
[4345] priority_pass seat=3 source= target=seat0
[4346] priority_pass seat=0 source= target=seat0
[4347] stack_resolve seat=1 source=Silvanus's Invoker target=seat0
[4348] untap seat=0 source=Silvanus's Invoker target=seat0
[4349] activated_ability_resolved seat=1 source=Silvanus's Invoker target=seat0
[4350] add_mana seat=1 source=Forest amount=1 target=seat0
[4351] activate_ability seat=1 source=Silvanus's Invoker target=seat0
[4352] stack_push seat=1 source=Silvanus's Invoker target=seat0
[4353] priority_pass seat=2 source= target=seat0
[4354] priority_pass seat=3 source= target=seat0
[4355] priority_pass seat=0 source= target=seat0
[4356] stack_resolve seat=1 source=Silvanus's Invoker target=seat0
[4357] untap seat=0 source=Silvanus's Invoker target=seat0
[4358] activated_ability_resolved seat=1 source=Silvanus's Invoker target=seat0
[4359] add_mana seat=1 source=Forest amount=1 target=seat0
[4360] state seat=1 source= target=seat0
```

</details>

#### Violation 17

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 60, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 60, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 4433 events
  Seat 0 [alive]: life=20 library=77 hand=5 graveyard=10 exile=0 battlefield=1 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=17 library=77 hand=1 graveyard=5 exile=0 battlefield=14 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0) [T]
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Slagwurm Armor (P/T 0/0, dmg=0)
    - Weapon Rack (P/T 0/0, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=75 hand=4 graveyard=9 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Hero of Bladehold (P/T 3/4, dmg=0)
  Seat 3 [alive]: life=10 library=77 hand=0 graveyard=10 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Fire Nation Warship (P/T 4/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4413] zone_change seat=2 source=Mox Ruby
[4414] resolve seat=2 source=Mox Ruby target=seat0
[4415] pay_mana seat=2 source=Hero of Bladehold amount=4 target=seat0
[4416] cast seat=2 source=Hero of Bladehold amount=4 target=seat0
[4417] stack_push seat=2 source=Hero of Bladehold target=seat0
[4418] priority_pass seat=3 source= target=seat0
[4419] priority_pass seat=0 source= target=seat0
[4420] priority_pass seat=1 source= target=seat0
[4421] stack_resolve seat=2 source=Hero of Bladehold target=seat0
[4422] enter_battlefield seat=2 source=Hero of Bladehold target=seat0
[4423] trigger_evaluated seat=1 source=Roil Elemental
[4424] stack_push seat=1 source=Roil Elemental target=seat0
[4425] triggered_ability seat=1 source=Roil Elemental target=seat0
[4426] priority_pass seat=2 source= target=seat0
[4427] priority_pass seat=3 source= target=seat0
[4428] priority_pass seat=0 source= target=seat0
[4429] stack_resolve seat=1 source=Roil Elemental target=seat0
[4430] phase_step seat=2 source= target=seat0
[4431] phase_step seat=2 source= target=seat0
[4432] state seat=2 source= target=seat0
```

</details>

#### Violation 18

- **Game**: 404 (seed 4040042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 60, Phase=ending Step=cleanup
- **Commanders**: Azula, Ruthless Firebender, Primo, the Indivisible, Zurgo and Ojutai, Empress Galina
- **Message**: CardIdentity: card "Disruptive Pitmage" (ptr 0xc003cfea20) appears in both seat 1 hand and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 60, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 4433 events
  Seat 0 [alive]: life=20 library=77 hand=5 graveyard=10 exile=0 battlefield=1 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=17 library=77 hand=1 graveyard=5 exile=0 battlefield=14 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Psychic Corrosion (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Silvanus's Invoker (P/T 3/2, dmg=0) [T]
    - Sentinel of the Pearl Trident (P/T 3/3, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Shared Fate (P/T 0/0, dmg=0)
    - Frilled Cave-Wurm (P/T 2/5, dmg=0) [T]
    - Howling Galefang (P/T 4/4, dmg=0)
    - Disruptive Pitmage (P/T 1/1, dmg=0) [T]
    - Slagwurm Armor (P/T 0/0, dmg=0)
    - Weapon Rack (P/T 0/0, dmg=0) [T]
    - Disruptive Pitmage (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=32 library=75 hand=4 graveyard=9 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fyndhorn Bow (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Hero of Bladehold (P/T 3/4, dmg=0)
  Seat 3 [alive]: life=10 library=77 hand=0 graveyard=10 exile=0 battlefield=26 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Empress Galina (P/T 1/3, dmg=0) [T]
    - Primo, the Indivisible (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terrarion (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hordewing Skaab (P/T 3/3, dmg=0) [T]
    - Cabaretti Courtyard (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Proft's Eidetic Memory (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Khalni Heart Expedition (P/T 0/0, dmg=0)
    - Vibrating Sphere (P/T 0/0, dmg=0)
    - Drake Hatchling (P/T 1/3, dmg=0) [T]
    - Fire Nation Warship (P/T 4/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4413] zone_change seat=2 source=Mox Ruby
[4414] resolve seat=2 source=Mox Ruby target=seat0
[4415] pay_mana seat=2 source=Hero of Bladehold amount=4 target=seat0
[4416] cast seat=2 source=Hero of Bladehold amount=4 target=seat0
[4417] stack_push seat=2 source=Hero of Bladehold target=seat0
[4418] priority_pass seat=3 source= target=seat0
[4419] priority_pass seat=0 source= target=seat0
[4420] priority_pass seat=1 source= target=seat0
[4421] stack_resolve seat=2 source=Hero of Bladehold target=seat0
[4422] enter_battlefield seat=2 source=Hero of Bladehold target=seat0
[4423] trigger_evaluated seat=1 source=Roil Elemental
[4424] stack_push seat=1 source=Roil Elemental target=seat0
[4425] triggered_ability seat=1 source=Roil Elemental target=seat0
[4426] priority_pass seat=2 source= target=seat0
[4427] priority_pass seat=3 source= target=seat0
[4428] priority_pass seat=0 source= target=seat0
[4429] stack_resolve seat=1 source=Roil Elemental target=seat0
[4430] phase_step seat=2 source= target=seat0
[4431] phase_step seat=2 source= target=seat0
[4432] state seat=2 source= target=seat0
```

</details>

#### Violation 19

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

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

#### Violation 20

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

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

#### Violation 21

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

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

#### Violation 22

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

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

#### Violation 23

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

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

#### Violation 24

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

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

#### Violation 25

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

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

#### Violation 26

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

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

#### Violation 27

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

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

#### Violation 28

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

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

#### Violation 29

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

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

#### Violation 30

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
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Friendly Teddy (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

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

*... and 1083 more violations not shown.*

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
| 2 | Jorubai Murk Lurker | 1 | 2 | 0.33 |
| 3 | Kheru Dreadmaw | 1 | 2 | 0.33 |
| 4 | Scarland Thrinax | 1 | 2 | 0.33 |
| 5 | Glacierwood Siege | 1 | 2 | 0.33 |
| 6 | Golden-Tail Trainer | 1 | 2 | 0.33 |
| 7 | A-Shattered Seraph | 1 | 2 | 0.33 |
| 8 | Ancient Spider | 1 | 2 | 0.33 |
| 9 | Baleful Strix | 2 | 4 | 0.33 |
| 10 | Enduring Scalelord | 1 | 2 | 0.33 |

## Verdict: ISSUES FOUND

**1119 total issues** across 5000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 1113 invariant violations in chaos games
- 0 crashes in nightmare boards
- 6 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
