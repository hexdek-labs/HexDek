# Chaos Gauntlet Report

Generated: 2026-04-18T18:40:25-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36510 cards |
| Legendary Creatures | 3434 |
| Total Games | 500 |
| Seed | 77777 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 8.349s |
| Throughput | 60 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 34 (in 12 games) |
| Clean Games | 488 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 1.675s |
| Throughput | 5972 boards/sec |
| Crashes | 0 |
| Invariant Violations | 39 |
| Clean Boards | 9976 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| TriggerCompleteness | 14 |
| ResourceConservation | 20 |

### Violation Details (first 30)

#### Violation 1

- **Game**: 110 (seed 1177778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 30, Phase=ending Step=cleanup
- **Commanders**: The Seventh Doctor, Sethron, Hurloon General, The Master of Keys, Toothy, Imaginary Friend
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 931 with trigger-bearer(s) [{A-Blood Artist 2}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 30, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 938 events
  Seat 0 [alive]: life=44 library=78 hand=1 graveyard=7 exile=0 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Infiltration Lens (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - The Seventh Doctor (P/T 3/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Goblin Dirigible (P/T 4/4, dmg=0) [T]
    - Uthros, Titanic Godcore (P/T 0/0, dmg=0) [T]
    - Wrench (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Chancellor of the Spires (P/T 5/7, dmg=0)
  Seat 1 [LOST]: life=-1 library=84 hand=0 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=41 library=83 hand=4 graveyard=4 exile=0 battlefield=10 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fountain of Youth (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - The Master of Keys (P/T 4/4, dmg=0) [T]
    - Fleeting Memories (P/T 0/0, dmg=0)
    - Clue Token (P/T 0/0, dmg=0)
    - Teyo, Geometric Tactician (P/T 0/3, dmg=0)
    - creature token wall Token (P/T 0/4, dmg=0)
    - A-Blood Artist (P/T 0/1, dmg=0)
  Seat 3 [alive]: life=24 library=81 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Tolarian Academy (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Bident of Thassa (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Repository Skaab (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[918] stack_push seat=2 source=Teyo, Geometric Tactician target=seat0
[919] priority_pass seat=3 source= target=seat0
[920] priority_pass seat=0 source= target=seat0
[921] stack_resolve seat=2 source=Teyo, Geometric Tactician target=seat0
[922] parsed_effect_residual seat=2 source=Teyo, Geometric Tactician target=seat0
[923] activated_ability_resolved seat=2 source=Teyo, Geometric Tactician target=seat0
[924] phase_step seat=2 source= target=seat0
[925] declare_attackers seat=2 source= target=seat0
[926] blockers seat=3 source= target=seat0
[927] damage seat=2 source=The Master of Keys amount=4 target=seat3
[928] damage seat=2 source=Ambassador Laquatus amount=1 target=seat3
[929] damage seat=3 source=Repository Skaab amount=3 target=seat2
[930] destroy seat=2 source=Ambassador Laquatus
[931] sba_704_5g seat=2 source=Ambassador Laquatus
[932] zone_change seat=2 source=Ambassador Laquatus
[933] sba_cycle_complete seat=-1 source=
[934] phase_step seat=2 source= target=seat0
[935] pool_drain seat=2 source= amount=1 target=seat0
[936] damage_wears_off seat=3 source=Repository Skaab amount=1 target=seat0
[937] state seat=2 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 110 (seed 1177778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 30, Phase=ending Step=cleanup
- **Commanders**: The Seventh Doctor, Sethron, Hurloon General, The Master of Keys, Toothy, Imaginary Friend
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 931 with trigger-bearer(s) [{A-Blood Artist 2}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 30, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 938 events
  Seat 0 [alive]: life=44 library=78 hand=1 graveyard=7 exile=0 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Infiltration Lens (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - The Seventh Doctor (P/T 3/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Goblin Dirigible (P/T 4/4, dmg=0) [T]
    - Uthros, Titanic Godcore (P/T 0/0, dmg=0) [T]
    - Wrench (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Chancellor of the Spires (P/T 5/7, dmg=0)
  Seat 1 [LOST]: life=-1 library=84 hand=0 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=41 library=83 hand=4 graveyard=4 exile=0 battlefield=10 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fountain of Youth (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - The Master of Keys (P/T 4/4, dmg=0) [T]
    - Fleeting Memories (P/T 0/0, dmg=0)
    - Clue Token (P/T 0/0, dmg=0)
    - Teyo, Geometric Tactician (P/T 0/3, dmg=0)
    - creature token wall Token (P/T 0/4, dmg=0)
    - A-Blood Artist (P/T 0/1, dmg=0)
  Seat 3 [alive]: life=24 library=81 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Tolarian Academy (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Bident of Thassa (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Repository Skaab (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[918] stack_push seat=2 source=Teyo, Geometric Tactician target=seat0
[919] priority_pass seat=3 source= target=seat0
[920] priority_pass seat=0 source= target=seat0
[921] stack_resolve seat=2 source=Teyo, Geometric Tactician target=seat0
[922] parsed_effect_residual seat=2 source=Teyo, Geometric Tactician target=seat0
[923] activated_ability_resolved seat=2 source=Teyo, Geometric Tactician target=seat0
[924] phase_step seat=2 source= target=seat0
[925] declare_attackers seat=2 source= target=seat0
[926] blockers seat=3 source= target=seat0
[927] damage seat=2 source=The Master of Keys amount=4 target=seat3
[928] damage seat=2 source=Ambassador Laquatus amount=1 target=seat3
[929] damage seat=3 source=Repository Skaab amount=3 target=seat2
[930] destroy seat=2 source=Ambassador Laquatus
[931] sba_704_5g seat=2 source=Ambassador Laquatus
[932] zone_change seat=2 source=Ambassador Laquatus
[933] sba_cycle_complete seat=-1 source=
[934] phase_step seat=2 source= target=seat0
[935] pool_drain seat=2 source= amount=1 target=seat0
[936] damage_wears_off seat=3 source=Repository Skaab amount=1 target=seat0
[937] state seat=2 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 155 (seed 1627778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 33, Phase=combat Step=end_of_combat
- **Commanders**: Elesh Norn // The Argent Etchings, Go-Shintai of Ancient Wars, Tom Bombadil, Superior Spider-Man
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=5

<details>
<summary>Game State</summary>

```
Turn 33, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1869 events
  Seat 0 [alive]: life=29 library=79 hand=1 graveyard=7 exile=0 battlefield=12 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sejiri Steppe (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Elesh Norn // The Argent Etchings (P/T 3/5, dmg=0)
    - Ebony Horse (P/T 0/0, dmg=0)
    - Haven of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Ur-Golem's Eye (P/T 0/0, dmg=0)
    - Dragon Mask (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Voice of Victory (P/T 1/3, dmg=0) [T]
    - Manifold Key (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=40 library=0 hand=10 graveyard=71 exile=0 battlefield=0 cmdzone=0 mana=5
  Seat 2 [alive]: life=39 library=81 hand=2 graveyard=11 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blood Ogre (P/T 2/2, dmg=0) [T]
  Seat 3 [LOST]: life=-5 library=83 hand=3 graveyard=1 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1849] activated_ability_resolved seat=1 source=Arcane Spyglass target=seat0
[1850] activate_ability seat=1 source=Arcane Spyglass target=seat0
[1851] stack_push seat=1 source=Arcane Spyglass target=seat0
[1852] priority_pass seat=2 source= target=seat0
[1853] priority_pass seat=0 source= target=seat0
[1854] stack_resolve seat=1 source=Arcane Spyglass target=seat0
[1855] draw seat=1 source=Arcane Spyglass amount=1 target=seat1
[1856] activated_ability_resolved seat=1 source=Arcane Spyglass target=seat0
[1857] activate_ability seat=1 source=Arcane Spyglass target=seat0
[1858] stack_push seat=1 source=Arcane Spyglass target=seat0
[1859] priority_pass seat=2 source= target=seat0
[1860] priority_pass seat=0 source= target=seat0
[1861] stack_resolve seat=1 source=Arcane Spyglass target=seat0
[1862] draw seat=1 source=Arcane Spyglass target=seat1
[1863] activated_ability_resolved seat=1 source=Arcane Spyglass target=seat0
[1864] sba_704_5b seat=1 source=
[1865] sba_cycle_complete seat=-1 source=
[1866] seat_eliminated seat=1 source= amount=17
[1867] phase_step seat=2 source= target=seat0
[1868] phase_step seat=2 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 155 (seed 1627778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 33, Phase=combat Step=end_of_combat
- **Commanders**: Elesh Norn // The Argent Etchings, Go-Shintai of Ancient Wars, Tom Bombadil, Superior Spider-Man
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=5

<details>
<summary>Game State</summary>

```
Turn 33, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1869 events
  Seat 0 [alive]: life=29 library=79 hand=1 graveyard=7 exile=0 battlefield=12 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sejiri Steppe (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Elesh Norn // The Argent Etchings (P/T 3/5, dmg=0)
    - Ebony Horse (P/T 0/0, dmg=0)
    - Haven of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Ur-Golem's Eye (P/T 0/0, dmg=0)
    - Dragon Mask (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Voice of Victory (P/T 1/3, dmg=0) [T]
    - Manifold Key (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=40 library=0 hand=10 graveyard=71 exile=0 battlefield=0 cmdzone=0 mana=5
  Seat 2 [alive]: life=39 library=81 hand=2 graveyard=11 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blood Ogre (P/T 2/2, dmg=0) [T]
  Seat 3 [LOST]: life=-5 library=83 hand=3 graveyard=1 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1849] activated_ability_resolved seat=1 source=Arcane Spyglass target=seat0
[1850] activate_ability seat=1 source=Arcane Spyglass target=seat0
[1851] stack_push seat=1 source=Arcane Spyglass target=seat0
[1852] priority_pass seat=2 source= target=seat0
[1853] priority_pass seat=0 source= target=seat0
[1854] stack_resolve seat=1 source=Arcane Spyglass target=seat0
[1855] draw seat=1 source=Arcane Spyglass amount=1 target=seat1
[1856] activated_ability_resolved seat=1 source=Arcane Spyglass target=seat0
[1857] activate_ability seat=1 source=Arcane Spyglass target=seat0
[1858] stack_push seat=1 source=Arcane Spyglass target=seat0
[1859] priority_pass seat=2 source= target=seat0
[1860] priority_pass seat=0 source= target=seat0
[1861] stack_resolve seat=1 source=Arcane Spyglass target=seat0
[1862] draw seat=1 source=Arcane Spyglass target=seat1
[1863] activated_ability_resolved seat=1 source=Arcane Spyglass target=seat0
[1864] sba_704_5b seat=1 source=
[1865] sba_cycle_complete seat=-1 source=
[1866] seat_eliminated seat=1 source= amount=17
[1867] phase_step seat=2 source= target=seat0
[1868] phase_step seat=2 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 267 (seed 2747778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 32, Phase=combat Step=beginning_of_combat
- **Commanders**: Commissar Severina Raine, Mana Max, Afterburner, Bumi, Eclectic Earthbender, Gandalf, Westward Voyager
- **Message**: ResourceConservation: seat 0 is Lost but has ManaPool=10

<details>
<summary>Game State</summary>

```
Turn 32, Phase=combat Step=beginning_of_combat Active=seat3
Stack: 0 items, EventLog: 1983 events
  Seat 0 [LOST]: life=194 library=0 hand=11 graveyard=73 exile=0 battlefield=0 cmdzone=0 mana=10
  Seat 1 [LOST]: life=-3 library=83 hand=1 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=0 library=86 hand=2 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=31 library=82 hand=2 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Entrancing Lyre (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Blastoderm (P/T 5/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Gandalf, Westward Voyager (P/T 5/5, dmg=0) [T]
    - Barrels of Blasting Jelly (P/T 0/0, dmg=0) [T]
    - Hover Barrier (P/T 0/6, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1963] activate_ability seat=0 source=Commissar Severina Raine target=seat0
[1964] stack_push seat=0 source=Commissar Severina Raine target=seat0
[1965] priority_pass seat=3 source= target=seat0
[1966] stack_resolve seat=0 source=Commissar Severina Raine target=seat0
[1967] gain_life seat=0 source=Commissar Severina Raine amount=2 target=seat0
[1968] life_change seat=0 source=Commissar Severina Raine amount=2 target=seat0
[1969] draw seat=0 source=Commissar Severina Raine amount=1 target=seat0
[1970] activated_ability_resolved seat=0 source=Commissar Severina Raine target=seat0
[1971] activate_ability seat=0 source=Commissar Severina Raine target=seat0
[1972] stack_push seat=0 source=Commissar Severina Raine target=seat0
[1973] priority_pass seat=3 source= target=seat0
[1974] stack_resolve seat=0 source=Commissar Severina Raine target=seat0
[1975] gain_life seat=0 source=Commissar Severina Raine amount=2 target=seat0
[1976] life_change seat=0 source=Commissar Severina Raine amount=2 target=seat0
[1977] draw seat=0 source=Commissar Severina Raine target=seat0
[1978] activated_ability_resolved seat=0 source=Commissar Severina Raine target=seat0
[1979] sba_704_5b seat=0 source=
[1980] sba_cycle_complete seat=-1 source=
[1981] seat_eliminated seat=0 source= amount=16
[1982] game_end seat=3 source=
```

</details>

#### Violation 6

- **Game**: 267 (seed 2747778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 32, Phase=combat Step=beginning_of_combat
- **Commanders**: Commissar Severina Raine, Mana Max, Afterburner, Bumi, Eclectic Earthbender, Gandalf, Westward Voyager
- **Message**: ResourceConservation: seat 0 is Lost but has ManaPool=10

<details>
<summary>Game State</summary>

```
Turn 32, Phase=combat Step=beginning_of_combat Active=seat3
Stack: 0 items, EventLog: 1983 events
  Seat 0 [LOST]: life=194 library=0 hand=11 graveyard=73 exile=0 battlefield=0 cmdzone=0 mana=10
  Seat 1 [LOST]: life=-3 library=83 hand=1 graveyard=6 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=0 library=86 hand=2 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=31 library=82 hand=2 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Entrancing Lyre (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Blastoderm (P/T 5/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Gandalf, Westward Voyager (P/T 5/5, dmg=0) [T]
    - Barrels of Blasting Jelly (P/T 0/0, dmg=0) [T]
    - Hover Barrier (P/T 0/6, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1963] activate_ability seat=0 source=Commissar Severina Raine target=seat0
[1964] stack_push seat=0 source=Commissar Severina Raine target=seat0
[1965] priority_pass seat=3 source= target=seat0
[1966] stack_resolve seat=0 source=Commissar Severina Raine target=seat0
[1967] gain_life seat=0 source=Commissar Severina Raine amount=2 target=seat0
[1968] life_change seat=0 source=Commissar Severina Raine amount=2 target=seat0
[1969] draw seat=0 source=Commissar Severina Raine amount=1 target=seat0
[1970] activated_ability_resolved seat=0 source=Commissar Severina Raine target=seat0
[1971] activate_ability seat=0 source=Commissar Severina Raine target=seat0
[1972] stack_push seat=0 source=Commissar Severina Raine target=seat0
[1973] priority_pass seat=3 source= target=seat0
[1974] stack_resolve seat=0 source=Commissar Severina Raine target=seat0
[1975] gain_life seat=0 source=Commissar Severina Raine amount=2 target=seat0
[1976] life_change seat=0 source=Commissar Severina Raine amount=2 target=seat0
[1977] draw seat=0 source=Commissar Severina Raine target=seat0
[1978] activated_ability_resolved seat=0 source=Commissar Severina Raine target=seat0
[1979] sba_704_5b seat=0 source=
[1980] sba_cycle_complete seat=-1 source=
[1981] seat_eliminated seat=0 source= amount=16
[1982] game_end seat=3 source=
```

</details>

#### Violation 7

- **Game**: 273 (seed 2807778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 48, Phase=main Step=precombat_main
- **Commanders**: Kalitas, Bloodchief of Ghet, Iroh, Firebending Instructor, Jhoira of the Ghitu, Davros, Dalek Creator
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=8

<details>
<summary>Game State</summary>

```
Turn 48, Phase=main Step=precombat_main Active=seat2
Stack: 0 items, EventLog: 1996 events
  Seat 0 [alive]: life=22 library=79 hand=0 graveyard=5 exile=0 battlefield=15 cmdzone=0 mana=0
    - Survivors' Encampment (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kalitas, Bloodchief of Ghet (P/T 5/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tree of Perdition (P/T 0/13, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Death Pits of Rath (P/T 0/0, dmg=0)
    - Gateway Plaza (P/T 0/0, dmg=0) [T]
    - Cogwork Archivist (P/T 4/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Leechridden Swamp (P/T 0/0, dmg=0) [T]
    - Boulderborn Dragon (P/T 3/3, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=40 library=0 hand=13 graveyard=73 exile=0 battlefield=0 cmdzone=1 mana=8
  Seat 2 [alive]: life=40 library=79 hand=7 graveyard=7 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Izzet Boilerworks (P/T 0/0, dmg=0) [T]
    - Walking Sponge (P/T 1/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=21 library=78 hand=2 graveyard=14 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sheltered Valley (P/T 0/0, dmg=0) [T]
    - Sulfurous Springs (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1976] stack_push seat=1 source=Cryptex target=seat0
[1977] priority_pass seat=2 source= target=seat0
[1978] priority_pass seat=3 source= target=seat0
[1979] priority_pass seat=0 source= target=seat0
[1980] stack_resolve seat=1 source=Cryptex target=seat0
[1981] surveil seat=1 source=Cryptex amount=3 target=seat0
[1982] draw seat=1 source=Cryptex amount=3 target=seat1
[1983] activated_ability_resolved seat=1 source=Cryptex target=seat0
[1984] activate_ability seat=1 source=Cryptex target=seat0
[1985] stack_push seat=1 source=Cryptex target=seat0
[1986] priority_pass seat=2 source= target=seat0
[1987] priority_pass seat=3 source= target=seat0
[1988] priority_pass seat=0 source= target=seat0
[1989] stack_resolve seat=1 source=Cryptex target=seat0
[1990] surveil seat=1 source=Cryptex amount=3 target=seat0
[1991] draw seat=1 source=Cryptex amount=1 target=seat1
[1992] activated_ability_resolved seat=1 source=Cryptex target=seat0
[1993] sba_704_5b seat=1 source=
[1994] sba_cycle_complete seat=-1 source=
[1995] seat_eliminated seat=1 source= amount=19
```

</details>

#### Violation 8

- **Game**: 273 (seed 2807778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 48, Phase=main Step=precombat_main
- **Commanders**: Kalitas, Bloodchief of Ghet, Iroh, Firebending Instructor, Jhoira of the Ghitu, Davros, Dalek Creator
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=8

<details>
<summary>Game State</summary>

```
Turn 48, Phase=main Step=precombat_main Active=seat2
Stack: 0 items, EventLog: 1996 events
  Seat 0 [alive]: life=22 library=79 hand=0 graveyard=5 exile=0 battlefield=15 cmdzone=0 mana=0
    - Survivors' Encampment (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kalitas, Bloodchief of Ghet (P/T 5/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tree of Perdition (P/T 0/13, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Death Pits of Rath (P/T 0/0, dmg=0)
    - Gateway Plaza (P/T 0/0, dmg=0) [T]
    - Cogwork Archivist (P/T 4/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Leechridden Swamp (P/T 0/0, dmg=0) [T]
    - Boulderborn Dragon (P/T 3/3, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=40 library=0 hand=13 graveyard=73 exile=0 battlefield=0 cmdzone=1 mana=8
  Seat 2 [alive]: life=40 library=79 hand=7 graveyard=7 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Izzet Boilerworks (P/T 0/0, dmg=0) [T]
    - Walking Sponge (P/T 1/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=21 library=78 hand=2 graveyard=14 exile=0 battlefield=3 cmdzone=1 mana=0
    - Sheltered Valley (P/T 0/0, dmg=0) [T]
    - Sulfurous Springs (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1976] stack_push seat=1 source=Cryptex target=seat0
[1977] priority_pass seat=2 source= target=seat0
[1978] priority_pass seat=3 source= target=seat0
[1979] priority_pass seat=0 source= target=seat0
[1980] stack_resolve seat=1 source=Cryptex target=seat0
[1981] surveil seat=1 source=Cryptex amount=3 target=seat0
[1982] draw seat=1 source=Cryptex amount=3 target=seat1
[1983] activated_ability_resolved seat=1 source=Cryptex target=seat0
[1984] activate_ability seat=1 source=Cryptex target=seat0
[1985] stack_push seat=1 source=Cryptex target=seat0
[1986] priority_pass seat=2 source= target=seat0
[1987] priority_pass seat=3 source= target=seat0
[1988] priority_pass seat=0 source= target=seat0
[1989] stack_resolve seat=1 source=Cryptex target=seat0
[1990] surveil seat=1 source=Cryptex amount=3 target=seat0
[1991] draw seat=1 source=Cryptex amount=1 target=seat1
[1992] activated_ability_resolved seat=1 source=Cryptex target=seat0
[1993] sba_704_5b seat=1 source=
[1994] sba_cycle_complete seat=-1 source=
[1995] seat_eliminated seat=1 source= amount=19
```

</details>

#### Violation 9

- **Game**: 286 (seed 2937778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 48, Phase=combat Step=end_of_combat
- **Commanders**: Baeloth Barrityl, Entertainer, Omnath, Locus of Mana, Leela, Sevateem Warrior, Atraxa, Praetors' Voice
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=6

<details>
<summary>Game State</summary>

```
Turn 48, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1772 events
  Seat 0 [LOST]: life=-6 library=84 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=0 library=74 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=0 mana=6
  Seat 2 [LOST]: life=-2 library=82 hand=5 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=83 library=77 hand=1 graveyard=5 exile=0 battlefield=17 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Paleontologist's Pick-Axe // Dinosaur Headdress (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Grafted Wargear (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Obelisk of Urd (P/T 0/0, dmg=0)
    - Great Hall of the Biblioplex (P/T 0/0, dmg=0) [T]
    - Syndicate Infiltrator (P/T 3/3, dmg=0) [T]
    - Bandit's Talent (P/T 0/0, dmg=0)
    - Vampire Nighthawk (P/T 2/3, dmg=0) [T]
    - Dagger of the Worthy (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Sequestered Stash (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Atraxa, Praetors' Voice (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1752] pay_mana seat=3 source=Bandit's Talent amount=1 target=seat0
[1753] activate_ability seat=3 source=Bandit's Talent target=seat0
[1754] stack_push seat=3 source=Bandit's Talent target=seat0
[1755] priority_pass seat=1 source= target=seat0
[1756] stack_resolve seat=3 source=Bandit's Talent target=seat0
[1757] modification_effect seat=3 source=Bandit's Talent target=seat0
[1758] activated_ability_resolved seat=3 source=Bandit's Talent target=seat0
[1759] priority_pass seat=1 source= target=seat0
[1760] stack_resolve seat=3 source=Atraxa, Praetors' Voice target=seat0
[1761] enter_battlefield seat=3 source=Atraxa, Praetors' Voice target=seat0
[1762] phase_step seat=3 source= target=seat0
[1763] declare_attackers seat=3 source= target=seat0
[1764] blockers seat=1 source= target=seat0
[1765] damage seat=3 source=Syndicate Infiltrator amount=3 target=seat1
[1766] damage seat=3 source=Vampire Nighthawk amount=2 target=seat1
[1767] life_change seat=3 source=Vampire Nighthawk amount=2 target=seat0
[1768] sba_704_5a seat=1 source=
[1769] sba_cycle_complete seat=-1 source=
[1770] seat_eliminated seat=1 source= amount=10
[1771] game_end seat=3 source=
```

</details>

#### Violation 10

- **Game**: 286 (seed 2937778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 48, Phase=combat Step=end_of_combat
- **Commanders**: Baeloth Barrityl, Entertainer, Omnath, Locus of Mana, Leela, Sevateem Warrior, Atraxa, Praetors' Voice
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=6

<details>
<summary>Game State</summary>

```
Turn 48, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1772 events
  Seat 0 [LOST]: life=-6 library=84 hand=4 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=0 library=74 hand=0 graveyard=13 exile=0 battlefield=0 cmdzone=0 mana=6
  Seat 2 [LOST]: life=-2 library=82 hand=5 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=83 library=77 hand=1 graveyard=5 exile=0 battlefield=17 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Paleontologist's Pick-Axe // Dinosaur Headdress (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Grafted Wargear (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Obelisk of Urd (P/T 0/0, dmg=0)
    - Great Hall of the Biblioplex (P/T 0/0, dmg=0) [T]
    - Syndicate Infiltrator (P/T 3/3, dmg=0) [T]
    - Bandit's Talent (P/T 0/0, dmg=0)
    - Vampire Nighthawk (P/T 2/3, dmg=0) [T]
    - Dagger of the Worthy (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Sequestered Stash (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Atraxa, Praetors' Voice (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1752] pay_mana seat=3 source=Bandit's Talent amount=1 target=seat0
[1753] activate_ability seat=3 source=Bandit's Talent target=seat0
[1754] stack_push seat=3 source=Bandit's Talent target=seat0
[1755] priority_pass seat=1 source= target=seat0
[1756] stack_resolve seat=3 source=Bandit's Talent target=seat0
[1757] modification_effect seat=3 source=Bandit's Talent target=seat0
[1758] activated_ability_resolved seat=3 source=Bandit's Talent target=seat0
[1759] priority_pass seat=1 source= target=seat0
[1760] stack_resolve seat=3 source=Atraxa, Praetors' Voice target=seat0
[1761] enter_battlefield seat=3 source=Atraxa, Praetors' Voice target=seat0
[1762] phase_step seat=3 source= target=seat0
[1763] declare_attackers seat=3 source= target=seat0
[1764] blockers seat=1 source= target=seat0
[1765] damage seat=3 source=Syndicate Infiltrator amount=3 target=seat1
[1766] damage seat=3 source=Vampire Nighthawk amount=2 target=seat1
[1767] life_change seat=3 source=Vampire Nighthawk amount=2 target=seat0
[1768] sba_704_5a seat=1 source=
[1769] sba_cycle_complete seat=-1 source=
[1770] seat_eliminated seat=1 source= amount=10
[1771] game_end seat=3 source=
```

</details>

#### Violation 11

- **Game**: 316 (seed 3237778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 40, Phase=main Step=precombat_main
- **Commanders**: Zhou Yu, Chief Commander, Bilbo, Birthday Celebrant, Emet-Selch, Unsundered // Hades, Sorcerer of Eld, Vial Smasher, Gleeful Grenadier
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=9

<details>
<summary>Game State</summary>

```
Turn 40, Phase=main Step=precombat_main Active=seat2
Stack: 0 items, EventLog: 2056 events
  Seat 0 [LOST]: life=-2 library=82 hand=1 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=42 library=0 hand=10 graveyard=73 exile=0 battlefield=0 cmdzone=0 mana=9
  Seat 2 [alive]: life=10 library=82 hand=4 graveyard=2 exile=0 battlefield=12 cmdzone=0 mana=0
    - Everglades (P/T 0/0, dmg=0) [T]
    - Conjurer's Bauble (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emet-Selch, Unsundered // Hades, Sorcerer of Eld (P/T 2/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Manifest (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cauldron of Souls (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Thran Quarry (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=36 library=79 hand=0 graveyard=5 exile=0 battlefield=14 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Amulet of Vigor (P/T 0/0, dmg=0)
    - Arcane Lighthouse (P/T 0/0, dmg=0) [T]
    - Canyon Slough (P/T 0/0, dmg=0) [T]
    - Fires of Mount Doom (P/T 0/0, dmg=0)
    - Underworld Charger (P/T 3/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Vial Smasher, Gleeful Grenadier (P/T 3/2, dmg=0) [T]
    - Pit Raptor (P/T 4/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Returned Centaur (P/T 2/4, dmg=0) [T]
    - Skullclamp (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Rimrock Knight // Boulder Rush (P/T 3/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2036] stack_resolve seat=1 source=Chittering Skitterling target=seat0
[2037] draw seat=1 source=Chittering Skitterling amount=1 target=seat1
[2038] activated_ability_resolved seat=1 source=Chittering Skitterling target=seat0
[2039] activate_ability seat=1 source=Chittering Skitterling target=seat0
[2040] stack_push seat=1 source=Chittering Skitterling target=seat0
[2041] priority_pass seat=2 source= target=seat0
[2042] priority_pass seat=3 source= target=seat0
[2043] stack_resolve seat=1 source=Chittering Skitterling target=seat0
[2044] draw seat=1 source=Chittering Skitterling amount=1 target=seat1
[2045] activated_ability_resolved seat=1 source=Chittering Skitterling target=seat0
[2046] activate_ability seat=1 source=Chittering Skitterling target=seat0
[2047] stack_push seat=1 source=Chittering Skitterling target=seat0
[2048] priority_pass seat=2 source= target=seat0
[2049] priority_pass seat=3 source= target=seat0
[2050] stack_resolve seat=1 source=Chittering Skitterling target=seat0
[2051] draw seat=1 source=Chittering Skitterling target=seat1
[2052] activated_ability_resolved seat=1 source=Chittering Skitterling target=seat0
[2053] sba_704_5b seat=1 source=
[2054] sba_cycle_complete seat=-1 source=
[2055] seat_eliminated seat=1 source= amount=16
```

</details>

#### Violation 12

- **Game**: 316 (seed 3237778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 40, Phase=main Step=precombat_main
- **Commanders**: Zhou Yu, Chief Commander, Bilbo, Birthday Celebrant, Emet-Selch, Unsundered // Hades, Sorcerer of Eld, Vial Smasher, Gleeful Grenadier
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=9

<details>
<summary>Game State</summary>

```
Turn 40, Phase=main Step=precombat_main Active=seat2
Stack: 0 items, EventLog: 2056 events
  Seat 0 [LOST]: life=-2 library=82 hand=1 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=42 library=0 hand=10 graveyard=73 exile=0 battlefield=0 cmdzone=0 mana=9
  Seat 2 [alive]: life=10 library=82 hand=4 graveyard=2 exile=0 battlefield=12 cmdzone=0 mana=0
    - Everglades (P/T 0/0, dmg=0) [T]
    - Conjurer's Bauble (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emet-Selch, Unsundered // Hades, Sorcerer of Eld (P/T 2/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Manifest (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cauldron of Souls (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Thran Quarry (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=36 library=79 hand=0 graveyard=5 exile=0 battlefield=14 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Amulet of Vigor (P/T 0/0, dmg=0)
    - Arcane Lighthouse (P/T 0/0, dmg=0) [T]
    - Canyon Slough (P/T 0/0, dmg=0) [T]
    - Fires of Mount Doom (P/T 0/0, dmg=0)
    - Underworld Charger (P/T 3/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Vial Smasher, Gleeful Grenadier (P/T 3/2, dmg=0) [T]
    - Pit Raptor (P/T 4/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Returned Centaur (P/T 2/4, dmg=0) [T]
    - Skullclamp (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Rimrock Knight // Boulder Rush (P/T 3/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2036] stack_resolve seat=1 source=Chittering Skitterling target=seat0
[2037] draw seat=1 source=Chittering Skitterling amount=1 target=seat1
[2038] activated_ability_resolved seat=1 source=Chittering Skitterling target=seat0
[2039] activate_ability seat=1 source=Chittering Skitterling target=seat0
[2040] stack_push seat=1 source=Chittering Skitterling target=seat0
[2041] priority_pass seat=2 source= target=seat0
[2042] priority_pass seat=3 source= target=seat0
[2043] stack_resolve seat=1 source=Chittering Skitterling target=seat0
[2044] draw seat=1 source=Chittering Skitterling amount=1 target=seat1
[2045] activated_ability_resolved seat=1 source=Chittering Skitterling target=seat0
[2046] activate_ability seat=1 source=Chittering Skitterling target=seat0
[2047] stack_push seat=1 source=Chittering Skitterling target=seat0
[2048] priority_pass seat=2 source= target=seat0
[2049] priority_pass seat=3 source= target=seat0
[2050] stack_resolve seat=1 source=Chittering Skitterling target=seat0
[2051] draw seat=1 source=Chittering Skitterling target=seat1
[2052] activated_ability_resolved seat=1 source=Chittering Skitterling target=seat0
[2053] sba_704_5b seat=1 source=
[2054] sba_cycle_complete seat=-1 source=
[2055] seat_eliminated seat=1 source= amount=16
```

</details>

#### Violation 13

- **Game**: 316 (seed 3237778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 41, Phase=combat Step=end_of_combat
- **Commanders**: Zhou Yu, Chief Commander, Bilbo, Birthday Celebrant, Emet-Selch, Unsundered // Hades, Sorcerer of Eld, Vial Smasher, Gleeful Grenadier
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=9

<details>
<summary>Game State</summary>

```
Turn 41, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 2100 events
  Seat 0 [LOST]: life=-2 library=82 hand=1 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=42 library=0 hand=10 graveyard=73 exile=0 battlefield=0 cmdzone=0 mana=9
  Seat 2 [LOST]: life=-5 library=82 hand=4 graveyard=2 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=36 library=77 hand=0 graveyard=7 exile=0 battlefield=14 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Amulet of Vigor (P/T 0/0, dmg=0)
    - Arcane Lighthouse (P/T 0/0, dmg=0) [T]
    - Canyon Slough (P/T 0/0, dmg=0) [T]
    - Fires of Mount Doom (P/T 0/0, dmg=0)
    - Underworld Charger (P/T 3/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Vial Smasher, Gleeful Grenadier (P/T 3/2, dmg=0) [T]
    - Pit Raptor (P/T 4/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Returned Centaur (P/T 2/4, dmg=0) [T]
    - Skullclamp (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Rimrock Knight // Boulder Rush (P/T 3/1, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2080] resolve seat=3 source=Shaman's Trance target=seat0
[2081] pay_mana seat=3 source=Fires of Mount Doom amount=3 target=seat0
[2082] activate_ability seat=3 source=Fires of Mount Doom target=seat0
[2083] stack_push seat=3 source=Fires of Mount Doom target=seat0
[2084] priority_pass seat=2 source= target=seat0
[2085] stack_resolve seat=3 source=Fires of Mount Doom target=seat0
[2086] exile_top_library seat=3 source=Fires of Mount Doom target=seat0
[2087] activated_ability_resolved seat=3 source=Fires of Mount Doom target=seat0
[2088] phase_step seat=3 source= target=seat0
[2089] declare_attackers seat=3 source= target=seat0
[2090] blockers seat=2 source= target=seat0
[2091] damage seat=3 source=Pit Raptor amount=4 target=seat2
[2092] damage seat=3 source=Underworld Charger amount=3 target=seat2
[2093] damage seat=3 source=Vial Smasher, Gleeful Grenadier amount=3 target=seat2
[2094] damage seat=3 source=Returned Centaur amount=2 target=seat2
[2095] damage seat=3 source=Rimrock Knight // Boulder Rush amount=3 target=seat2
[2096] sba_704_5a seat=2 source= amount=-5
[2097] sba_cycle_complete seat=-1 source=
[2098] seat_eliminated seat=2 source= amount=12
[2099] game_end seat=3 source=
```

</details>

#### Violation 14

- **Game**: 316 (seed 3237778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 41, Phase=combat Step=end_of_combat
- **Commanders**: Zhou Yu, Chief Commander, Bilbo, Birthday Celebrant, Emet-Selch, Unsundered // Hades, Sorcerer of Eld, Vial Smasher, Gleeful Grenadier
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=9

<details>
<summary>Game State</summary>

```
Turn 41, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 2100 events
  Seat 0 [LOST]: life=-2 library=82 hand=1 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=42 library=0 hand=10 graveyard=73 exile=0 battlefield=0 cmdzone=0 mana=9
  Seat 2 [LOST]: life=-5 library=82 hand=4 graveyard=2 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=36 library=77 hand=0 graveyard=7 exile=0 battlefield=14 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Amulet of Vigor (P/T 0/0, dmg=0)
    - Arcane Lighthouse (P/T 0/0, dmg=0) [T]
    - Canyon Slough (P/T 0/0, dmg=0) [T]
    - Fires of Mount Doom (P/T 0/0, dmg=0)
    - Underworld Charger (P/T 3/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Vial Smasher, Gleeful Grenadier (P/T 3/2, dmg=0) [T]
    - Pit Raptor (P/T 4/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Returned Centaur (P/T 2/4, dmg=0) [T]
    - Skullclamp (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Rimrock Knight // Boulder Rush (P/T 3/1, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2080] resolve seat=3 source=Shaman's Trance target=seat0
[2081] pay_mana seat=3 source=Fires of Mount Doom amount=3 target=seat0
[2082] activate_ability seat=3 source=Fires of Mount Doom target=seat0
[2083] stack_push seat=3 source=Fires of Mount Doom target=seat0
[2084] priority_pass seat=2 source= target=seat0
[2085] stack_resolve seat=3 source=Fires of Mount Doom target=seat0
[2086] exile_top_library seat=3 source=Fires of Mount Doom target=seat0
[2087] activated_ability_resolved seat=3 source=Fires of Mount Doom target=seat0
[2088] phase_step seat=3 source= target=seat0
[2089] declare_attackers seat=3 source= target=seat0
[2090] blockers seat=2 source= target=seat0
[2091] damage seat=3 source=Pit Raptor amount=4 target=seat2
[2092] damage seat=3 source=Underworld Charger amount=3 target=seat2
[2093] damage seat=3 source=Vial Smasher, Gleeful Grenadier amount=3 target=seat2
[2094] damage seat=3 source=Returned Centaur amount=2 target=seat2
[2095] damage seat=3 source=Rimrock Knight // Boulder Rush amount=3 target=seat2
[2096] sba_704_5a seat=2 source= amount=-5
[2097] sba_cycle_complete seat=-1 source=
[2098] seat_eliminated seat=2 source= amount=12
[2099] game_end seat=3 source=
```

</details>

#### Violation 15

- **Game**: 311 (seed 3187778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 43, Phase=ending Step=cleanup
- **Commanders**: Phenax, God of Deception, Giott, King of the Dwarves, Arna Kennerüd, Skycaptain, Tombstone, Career Criminal
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 2484 with trigger-bearer(s) [{Vindictive Vampire 2}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 43, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2491 events
  Seat 0 [alive]: life=24 library=6 hand=7 graveyard=61 exile=0 battlefield=25 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Phenax, God of Deception (P/T 4/7, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Eon Hub (P/T 0/0, dmg=0)
    - Crystal Chimes (P/T 0/0, dmg=0)
    - Glitch Ghost Surveyor (P/T 2/2, dmg=0) [T]
    - Lash of the Tyrant (P/T 0/0, dmg=0)
    - Mishra's Workshop (P/T 0/0, dmg=0) [T]
    - Dictate of Kruphix (P/T 0/0, dmg=0)
    - Veiled Sentry (P/T 0/0, dmg=0)
    - Chariot of the Sun (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Lifebane Zombie (P/T 3/1, dmg=0) [T]
    - Dimir Cutpurse (P/T 2/2, dmg=0) [T]
    - Tomb Trawler (P/T 0/4, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lurking Informant (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abhorrent Oculus (P/T 5/5, dmg=0) [T]
    - Drifting Djinn (P/T 5/5, dmg=0) [T]
    - Underground Sea (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-4 library=82 hand=6 graveyard=1 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=56 library=79 hand=4 graveyard=5 exile=0 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Scrubland (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nomad Mythmaker (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Arna Kennerüd, Skycaptain (P/T 4/4, dmg=0) [T]
    - token Token (P/T 0/0, dmg=0)
    - Talas Lookout (P/T 3/2, dmg=0) [T]
    - token Token (P/T 0/0, dmg=0)
    - Vindictive Vampire (P/T 2/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - token Token (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - token Token (P/T 0/0, dmg=0)
  Seat 3 [LOST]: life=-21 library=80 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2471] trigger_fires seat=2 source=Arna Kennerüd, Skycaptain target=seat0
[2472] stack_push seat=2 source=Arna Kennerüd, Skycaptain target=seat0
[2473] priority_pass seat=0 source= target=seat0
[2474] stack_resolve seat=2 source=Arna Kennerüd, Skycaptain target=seat0
[2475] counter_mod seat=0 source=Arna Kennerüd, Skycaptain amount=1 target=seat0
[2476] create_token seat=2 source=Arna Kennerüd, Skycaptain amount=1 target=seat2
[2477] blockers seat=0 source= target=seat0
[2478] damage seat=0 source=Black Knight amount=2 target=seat2
[2479] damage seat=2 source=Arna Kennerüd, Skycaptain amount=4 target=seat0
[2480] life_change seat=2 source=Arna Kennerüd, Skycaptain amount=4 target=seat0
[2481] damage seat=2 source=Talas Lookout amount=3 target=seat0
[2482] damage seat=2 source=Vindictive Vampire amount=2 target=seat0
[2483] destroy seat=0 source=Black Knight
[2484] sba_704_5g seat=0 source=Black Knight
[2485] zone_change seat=0 source=Black Knight
[2486] sba_cycle_complete seat=-1 source=
[2487] phase_step seat=2 source= target=seat0
[2488] pool_drain seat=2 source= amount=6 target=seat0
[2489] damage_wears_off seat=2 source=Vindictive Vampire amount=2 target=seat0
[2490] state seat=2 source= target=seat0
```

</details>

#### Violation 16

- **Game**: 311 (seed 3187778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 43, Phase=ending Step=cleanup
- **Commanders**: Phenax, God of Deception, Giott, King of the Dwarves, Arna Kennerüd, Skycaptain, Tombstone, Career Criminal
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 2484 with trigger-bearer(s) [{Vindictive Vampire 2}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 43, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2491 events
  Seat 0 [alive]: life=24 library=6 hand=7 graveyard=61 exile=0 battlefield=25 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Phenax, God of Deception (P/T 4/7, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Eon Hub (P/T 0/0, dmg=0)
    - Crystal Chimes (P/T 0/0, dmg=0)
    - Glitch Ghost Surveyor (P/T 2/2, dmg=0) [T]
    - Lash of the Tyrant (P/T 0/0, dmg=0)
    - Mishra's Workshop (P/T 0/0, dmg=0) [T]
    - Dictate of Kruphix (P/T 0/0, dmg=0)
    - Veiled Sentry (P/T 0/0, dmg=0)
    - Chariot of the Sun (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Lifebane Zombie (P/T 3/1, dmg=0) [T]
    - Dimir Cutpurse (P/T 2/2, dmg=0) [T]
    - Tomb Trawler (P/T 0/4, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lurking Informant (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abhorrent Oculus (P/T 5/5, dmg=0) [T]
    - Drifting Djinn (P/T 5/5, dmg=0) [T]
    - Underground Sea (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-4 library=82 hand=6 graveyard=1 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=56 library=79 hand=4 graveyard=5 exile=0 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Scrubland (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nomad Mythmaker (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Arna Kennerüd, Skycaptain (P/T 4/4, dmg=0) [T]
    - token Token (P/T 0/0, dmg=0)
    - Talas Lookout (P/T 3/2, dmg=0) [T]
    - token Token (P/T 0/0, dmg=0)
    - Vindictive Vampire (P/T 2/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - token Token (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - token Token (P/T 0/0, dmg=0)
  Seat 3 [LOST]: life=-21 library=80 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2471] trigger_fires seat=2 source=Arna Kennerüd, Skycaptain target=seat0
[2472] stack_push seat=2 source=Arna Kennerüd, Skycaptain target=seat0
[2473] priority_pass seat=0 source= target=seat0
[2474] stack_resolve seat=2 source=Arna Kennerüd, Skycaptain target=seat0
[2475] counter_mod seat=0 source=Arna Kennerüd, Skycaptain amount=1 target=seat0
[2476] create_token seat=2 source=Arna Kennerüd, Skycaptain amount=1 target=seat2
[2477] blockers seat=0 source= target=seat0
[2478] damage seat=0 source=Black Knight amount=2 target=seat2
[2479] damage seat=2 source=Arna Kennerüd, Skycaptain amount=4 target=seat0
[2480] life_change seat=2 source=Arna Kennerüd, Skycaptain amount=4 target=seat0
[2481] damage seat=2 source=Talas Lookout amount=3 target=seat0
[2482] damage seat=2 source=Vindictive Vampire amount=2 target=seat0
[2483] destroy seat=0 source=Black Knight
[2484] sba_704_5g seat=0 source=Black Knight
[2485] zone_change seat=0 source=Black Knight
[2486] sba_cycle_complete seat=-1 source=
[2487] phase_step seat=2 source= target=seat0
[2488] pool_drain seat=2 source= amount=6 target=seat0
[2489] damage_wears_off seat=2 source=Vindictive Vampire amount=2 target=seat0
[2490] state seat=2 source= target=seat0
```

</details>

#### Violation 17

- **Game**: 311 (seed 3187778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 44, Phase=combat Step=beginning_of_combat
- **Commanders**: Phenax, God of Deception, Giott, King of the Dwarves, Arna Kennerüd, Skycaptain, Tombstone, Career Criminal
- **Message**: ResourceConservation: seat 0 is Lost but has ManaPool=12

<details>
<summary>Game State</summary>

```
Turn 44, Phase=combat Step=beginning_of_combat Active=seat2
Stack: 0 items, EventLog: 2569 events
  Seat 0 [LOST]: life=24 library=0 hand=11 graveyard=62 exile=0 battlefield=0 cmdzone=0 mana=12
  Seat 1 [LOST]: life=-4 library=82 hand=6 graveyard=1 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=56 library=79 hand=4 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Scrubland (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nomad Mythmaker (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Arna Kennerüd, Skycaptain (P/T 4/4, dmg=0) [T]
    - Talas Lookout (P/T 3/2, dmg=0) [T]
    - Vindictive Vampire (P/T 2/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-21 library=80 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2549] priority_pass seat=2 source= target=seat0
[2550] stack_resolve seat=0 source=Glitch Ghost Surveyor target=seat0
[2551] draw seat=0 source=Glitch Ghost Surveyor amount=1 target=seat0
[2552] activated_ability_resolved seat=0 source=Glitch Ghost Surveyor target=seat0
[2553] activate_ability seat=0 source=Glitch Ghost Surveyor target=seat0
[2554] stack_push seat=0 source=Glitch Ghost Surveyor target=seat0
[2555] priority_pass seat=2 source= target=seat0
[2556] stack_resolve seat=0 source=Glitch Ghost Surveyor target=seat0
[2557] draw seat=0 source=Glitch Ghost Surveyor amount=1 target=seat0
[2558] activated_ability_resolved seat=0 source=Glitch Ghost Surveyor target=seat0
[2559] activate_ability seat=0 source=Glitch Ghost Surveyor target=seat0
[2560] stack_push seat=0 source=Glitch Ghost Surveyor target=seat0
[2561] priority_pass seat=2 source= target=seat0
[2562] stack_resolve seat=0 source=Glitch Ghost Surveyor target=seat0
[2563] draw seat=0 source=Glitch Ghost Surveyor target=seat0
[2564] activated_ability_resolved seat=0 source=Glitch Ghost Surveyor target=seat0
[2565] sba_704_5b seat=0 source=
[2566] sba_cycle_complete seat=-1 source=
[2567] seat_eliminated seat=0 source= amount=30
[2568] game_end seat=2 source=
```

</details>

#### Violation 18

- **Game**: 311 (seed 3187778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 44, Phase=combat Step=beginning_of_combat
- **Commanders**: Phenax, God of Deception, Giott, King of the Dwarves, Arna Kennerüd, Skycaptain, Tombstone, Career Criminal
- **Message**: ResourceConservation: seat 0 is Lost but has ManaPool=12

<details>
<summary>Game State</summary>

```
Turn 44, Phase=combat Step=beginning_of_combat Active=seat2
Stack: 0 items, EventLog: 2569 events
  Seat 0 [LOST]: life=24 library=0 hand=11 graveyard=62 exile=0 battlefield=0 cmdzone=0 mana=12
  Seat 1 [LOST]: life=-4 library=82 hand=6 graveyard=1 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=56 library=79 hand=4 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Scrubland (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nomad Mythmaker (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Arna Kennerüd, Skycaptain (P/T 4/4, dmg=0) [T]
    - Talas Lookout (P/T 3/2, dmg=0) [T]
    - Vindictive Vampire (P/T 2/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-21 library=80 hand=3 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2549] priority_pass seat=2 source= target=seat0
[2550] stack_resolve seat=0 source=Glitch Ghost Surveyor target=seat0
[2551] draw seat=0 source=Glitch Ghost Surveyor amount=1 target=seat0
[2552] activated_ability_resolved seat=0 source=Glitch Ghost Surveyor target=seat0
[2553] activate_ability seat=0 source=Glitch Ghost Surveyor target=seat0
[2554] stack_push seat=0 source=Glitch Ghost Surveyor target=seat0
[2555] priority_pass seat=2 source= target=seat0
[2556] stack_resolve seat=0 source=Glitch Ghost Surveyor target=seat0
[2557] draw seat=0 source=Glitch Ghost Surveyor amount=1 target=seat0
[2558] activated_ability_resolved seat=0 source=Glitch Ghost Surveyor target=seat0
[2559] activate_ability seat=0 source=Glitch Ghost Surveyor target=seat0
[2560] stack_push seat=0 source=Glitch Ghost Surveyor target=seat0
[2561] priority_pass seat=2 source= target=seat0
[2562] stack_resolve seat=0 source=Glitch Ghost Surveyor target=seat0
[2563] draw seat=0 source=Glitch Ghost Surveyor target=seat0
[2564] activated_ability_resolved seat=0 source=Glitch Ghost Surveyor target=seat0
[2565] sba_704_5b seat=0 source=
[2566] sba_cycle_complete seat=-1 source=
[2567] seat_eliminated seat=0 source= amount=30
[2568] game_end seat=2 source=
```

</details>

#### Violation 19

- **Game**: 369 (seed 3767778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 24, Phase=combat Step=end_of_combat
- **Commanders**: Danny Pink, Monet, Sensei of the Sewers, Mazzy, Truesword Paladin, Noctis, Heir Apparent
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=6

<details>
<summary>Game State</summary>

```
Turn 24, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1030 events
  Seat 0 [alive]: life=40 library=84 hand=1 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Skilled Animator (P/T 1/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Danny Pink (P/T 4/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Tetravus (P/T 1/1, dmg=0)
  Seat 1 [LOST]: life=29 library=0 hand=20 graveyard=70 exile=0 battlefield=0 cmdzone=1 mana=6
  Seat 2 [alive]: life=40 library=85 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Jetmir's Garden (P/T 0/0, dmg=0) [T]
    - The Ooze (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Inspiring Statuary (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=3 graveyard=1 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Underdark Rift (P/T 0/0, dmg=0) [T]
    - Armageddon Clock (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - University Campus (P/T 0/0, dmg=0) [T]
    - Dreamborn Muse (P/T 2/2, dmg=0)
    - Meekstone (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1010] stack_push seat=1 source=Blitzball target=seat0
[1011] priority_pass seat=2 source= target=seat0
[1012] priority_pass seat=3 source= target=seat0
[1013] priority_pass seat=0 source= target=seat0
[1014] stack_resolve seat=1 source=Blitzball target=seat0
[1015] draw seat=1 source=Blitzball amount=2 target=seat1
[1016] activated_ability_resolved seat=1 source=Blitzball target=seat0
[1017] activate_ability seat=1 source=Blitzball target=seat0
[1018] stack_push seat=1 source=Blitzball target=seat0
[1019] priority_pass seat=2 source= target=seat0
[1020] priority_pass seat=3 source= target=seat0
[1021] priority_pass seat=0 source= target=seat0
[1022] stack_resolve seat=1 source=Blitzball target=seat0
[1023] draw seat=1 source=Blitzball amount=1 target=seat1
[1024] activated_ability_resolved seat=1 source=Blitzball target=seat0
[1025] sba_704_5b seat=1 source=
[1026] sba_cycle_complete seat=-1 source=
[1027] seat_eliminated seat=1 source= amount=8
[1028] phase_step seat=2 source= target=seat0
[1029] phase_step seat=2 source= target=seat0
```

</details>

#### Violation 20

- **Game**: 369 (seed 3767778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 24, Phase=combat Step=end_of_combat
- **Commanders**: Danny Pink, Monet, Sensei of the Sewers, Mazzy, Truesword Paladin, Noctis, Heir Apparent
- **Message**: ResourceConservation: seat 1 is Lost but has ManaPool=6

<details>
<summary>Game State</summary>

```
Turn 24, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 1030 events
  Seat 0 [alive]: life=40 library=84 hand=1 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Skilled Animator (P/T 1/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Danny Pink (P/T 4/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Tetravus (P/T 1/1, dmg=0)
  Seat 1 [LOST]: life=29 library=0 hand=20 graveyard=70 exile=0 battlefield=0 cmdzone=1 mana=6
  Seat 2 [alive]: life=40 library=85 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Jetmir's Garden (P/T 0/0, dmg=0) [T]
    - The Ooze (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Inspiring Statuary (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=3 graveyard=1 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Underdark Rift (P/T 0/0, dmg=0) [T]
    - Armageddon Clock (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - University Campus (P/T 0/0, dmg=0) [T]
    - Dreamborn Muse (P/T 2/2, dmg=0)
    - Meekstone (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1010] stack_push seat=1 source=Blitzball target=seat0
[1011] priority_pass seat=2 source= target=seat0
[1012] priority_pass seat=3 source= target=seat0
[1013] priority_pass seat=0 source= target=seat0
[1014] stack_resolve seat=1 source=Blitzball target=seat0
[1015] draw seat=1 source=Blitzball amount=2 target=seat1
[1016] activated_ability_resolved seat=1 source=Blitzball target=seat0
[1017] activate_ability seat=1 source=Blitzball target=seat0
[1018] stack_push seat=1 source=Blitzball target=seat0
[1019] priority_pass seat=2 source= target=seat0
[1020] priority_pass seat=3 source= target=seat0
[1021] priority_pass seat=0 source= target=seat0
[1022] stack_resolve seat=1 source=Blitzball target=seat0
[1023] draw seat=1 source=Blitzball amount=1 target=seat1
[1024] activated_ability_resolved seat=1 source=Blitzball target=seat0
[1025] sba_704_5b seat=1 source=
[1026] sba_cycle_complete seat=-1 source=
[1027] seat_eliminated seat=1 source= amount=8
[1028] phase_step seat=2 source= target=seat0
[1029] phase_step seat=2 source= target=seat0
```

</details>

#### Violation 21

- **Game**: 392 (seed 3997778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 46, Phase=ending Step=cleanup
- **Commanders**: Syr Konrad, the Grim, Prime Speaker Vannifar, Odric, Lunarch Marshal, Sokka, Swordmaster
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 2709 with trigger-bearer(s) [{Syr Konrad, the Grim 0}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 46, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2715 events
  Seat 0 [alive]: life=10 library=78 hand=0 graveyard=10 exile=1 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Oasis (P/T 0/0, dmg=0) [T]
    - Embalmer's Tools (P/T 0/0, dmg=0)
    - Baton of Morale (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Pelakka Predation // Pelakka Caverns (P/T 0/0, dmg=0) [T]
    - Syr Konrad, the Grim (P/T 5/4, dmg=0)
  Seat 1 [LOST]: life=0 library=10 hand=6 graveyard=78 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=40 library=32 hand=6 graveyard=52 exile=0 battlefield=10 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Torture Chamber (P/T 0/0, dmg=0) [T]
    - Captivating Crossroads (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Pang Tong, "Young Phoenix" (P/T 1/2, dmg=0) [T]
    - creature token vampire Token (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Odric, Lunarch Marshal (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Angel of Jubilation (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=40 library=78 hand=4 graveyard=3 exile=0 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Blinding Powder (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Prism Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Sokka, Swordmaster (P/T 3/3, dmg=0)
    - Ardenvale Tactician // Dizzying Swoop (P/T 2/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Daraja Griffin (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Spotcycle Scouter (P/T 3/2, dmg=0)
    - Aeolipile (P/T 0/0, dmg=0)
    - Nemesis Mask (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2695] stack_push seat=2 source=Pang Tong, "Young Phoenix" target=seat0
[2696] priority_pass seat=3 source= target=seat0
[2697] priority_pass seat=0 source= target=seat0
[2698] stack_resolve seat=2 source=Pang Tong, "Young Phoenix" target=seat0
[2699] buff seat=0 source=Pang Tong, "Young Phoenix" target=seat0
[2700] activated_ability_resolved seat=2 source=Pang Tong, "Young Phoenix" target=seat0
[2701] phase_step seat=2 source= target=seat0
[2702] declare_attackers seat=2 source= target=seat0
[2703] blockers seat=0 source= target=seat0
[2704] damage seat=2 source=Oltec Matterweaver amount=2 target=seat0
[2705] damage seat=2 source=creature token vampire Token amount=1 target=seat0
[2706] damage seat=2 source=Odric, Lunarch Marshal amount=3 target=seat0
[2707] damage seat=0 source=Syr Konrad, the Grim amount=5 target=seat2
[2708] destroy seat=2 source=Oltec Matterweaver
[2709] sba_704_5g seat=2 source=Oltec Matterweaver
[2710] zone_change seat=2 source=Oltec Matterweaver
[2711] sba_cycle_complete seat=-1 source=
[2712] phase_step seat=2 source= target=seat0
[2713] damage_wears_off seat=0 source=Syr Konrad, the Grim amount=2 target=seat0
[2714] state seat=2 source= target=seat0
```

</details>

#### Violation 22

- **Game**: 392 (seed 3997778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 46, Phase=ending Step=cleanup
- **Commanders**: Syr Konrad, the Grim, Prime Speaker Vannifar, Odric, Lunarch Marshal, Sokka, Swordmaster
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 2709 with trigger-bearer(s) [{Syr Konrad, the Grim 0}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 46, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2715 events
  Seat 0 [alive]: life=10 library=78 hand=0 graveyard=10 exile=1 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Oasis (P/T 0/0, dmg=0) [T]
    - Embalmer's Tools (P/T 0/0, dmg=0)
    - Baton of Morale (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Pelakka Predation // Pelakka Caverns (P/T 0/0, dmg=0) [T]
    - Syr Konrad, the Grim (P/T 5/4, dmg=0)
  Seat 1 [LOST]: life=0 library=10 hand=6 graveyard=78 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=40 library=32 hand=6 graveyard=52 exile=0 battlefield=10 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Torture Chamber (P/T 0/0, dmg=0) [T]
    - Captivating Crossroads (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Pang Tong, "Young Phoenix" (P/T 1/2, dmg=0) [T]
    - creature token vampire Token (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Odric, Lunarch Marshal (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Angel of Jubilation (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=40 library=78 hand=4 graveyard=3 exile=0 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Blinding Powder (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Prism Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Sokka, Swordmaster (P/T 3/3, dmg=0)
    - Ardenvale Tactician // Dizzying Swoop (P/T 2/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Daraja Griffin (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Spotcycle Scouter (P/T 3/2, dmg=0)
    - Aeolipile (P/T 0/0, dmg=0)
    - Nemesis Mask (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2695] stack_push seat=2 source=Pang Tong, "Young Phoenix" target=seat0
[2696] priority_pass seat=3 source= target=seat0
[2697] priority_pass seat=0 source= target=seat0
[2698] stack_resolve seat=2 source=Pang Tong, "Young Phoenix" target=seat0
[2699] buff seat=0 source=Pang Tong, "Young Phoenix" target=seat0
[2700] activated_ability_resolved seat=2 source=Pang Tong, "Young Phoenix" target=seat0
[2701] phase_step seat=2 source= target=seat0
[2702] declare_attackers seat=2 source= target=seat0
[2703] blockers seat=0 source= target=seat0
[2704] damage seat=2 source=Oltec Matterweaver amount=2 target=seat0
[2705] damage seat=2 source=creature token vampire Token amount=1 target=seat0
[2706] damage seat=2 source=Odric, Lunarch Marshal amount=3 target=seat0
[2707] damage seat=0 source=Syr Konrad, the Grim amount=5 target=seat2
[2708] destroy seat=2 source=Oltec Matterweaver
[2709] sba_704_5g seat=2 source=Oltec Matterweaver
[2710] zone_change seat=2 source=Oltec Matterweaver
[2711] sba_cycle_complete seat=-1 source=
[2712] phase_step seat=2 source= target=seat0
[2713] damage_wears_off seat=0 source=Syr Konrad, the Grim amount=2 target=seat0
[2714] state seat=2 source= target=seat0
```

</details>

#### Violation 23

- **Game**: 421 (seed 4287778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 37, Phase=main Step=precombat_main
- **Commanders**: Peregrin Took, Krond the Dawn-Clad, Zaffai and the Tempests, Rakdos, Lord of Riots
- **Message**: ResourceConservation: seat 0 is Lost but has ManaPool=7

<details>
<summary>Game State</summary>

```
Turn 37, Phase=main Step=precombat_main Active=seat2
Stack: 0 items, EventLog: 1832 events
  Seat 0 [LOST]: life=22 library=0 hand=7 graveyard=72 exile=0 battlefield=0 cmdzone=0 mana=7
  Seat 1 [LOST]: life=-2 library=84 hand=4 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=11 library=81 hand=5 graveyard=2 exile=0 battlefield=12 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Dowsing Device // Geode Grotto (P/T 0/0, dmg=0) [T]
    - A-Kenku Artificer (P/T 1/3, dmg=0) [T]
    - Plaza of Harmony (P/T 0/0, dmg=0) [T]
    - Fire Nation's Conquest (P/T 0/0, dmg=0)
    - Sneaky Homunculus (P/T 1/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Thunderblust (P/T 7/2, dmg=0) [T]
    - Accursed Duneyard (P/T 0/0, dmg=0) [T]
    - Lazotep Quarry (P/T 0/0, dmg=0) [T]
    - Zaffai and the Tempests (P/T 5/7, dmg=0) [T]
  Seat 3 [LOST]: life=-3 library=82 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1812] add_mana seat=0 source=Forest amount=1 target=seat0
[1813] add_mana seat=0 source=Forest amount=1 target=seat0
[1814] add_mana seat=0 source=Forest amount=1 target=seat0
[1815] add_mana seat=0 source=Forest amount=1 target=seat0
[1816] add_mana seat=0 source=Forest amount=1 target=seat0
[1817] add_mana seat=0 source=Gond Gate amount=1 target=seat0
[1818] add_mana seat=0 source=Forest amount=1 target=seat0
[1819] add_mana seat=0 source=Mirrodin's Core amount=1 target=seat0
[1820] tap seat=0 source=Bandit's Haul target=seat0
[1821] pay_mana seat=0 source=Bandit's Haul amount=2 target=seat0
[1822] activate_ability seat=0 source=Bandit's Haul target=seat0
[1823] stack_push seat=0 source=Bandit's Haul target=seat0
[1824] priority_pass seat=2 source= target=seat0
[1825] stack_resolve seat=0 source=Bandit's Haul target=seat0
[1826] draw seat=0 source=Bandit's Haul target=seat0
[1827] activated_ability_resolved seat=0 source=Bandit's Haul target=seat0
[1828] sba_704_5b seat=0 source=
[1829] sba_cycle_complete seat=-1 source=
[1830] seat_eliminated seat=0 source= amount=18
[1831] game_end seat=2 source=
```

</details>

#### Violation 24

- **Game**: 421 (seed 4287778, perm 0)
- **Invariant**: ResourceConservation
- **Turn**: 37, Phase=main Step=precombat_main
- **Commanders**: Peregrin Took, Krond the Dawn-Clad, Zaffai and the Tempests, Rakdos, Lord of Riots
- **Message**: ResourceConservation: seat 0 is Lost but has ManaPool=7

<details>
<summary>Game State</summary>

```
Turn 37, Phase=main Step=precombat_main Active=seat2
Stack: 0 items, EventLog: 1832 events
  Seat 0 [LOST]: life=22 library=0 hand=7 graveyard=72 exile=0 battlefield=0 cmdzone=0 mana=7
  Seat 1 [LOST]: life=-2 library=84 hand=4 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=11 library=81 hand=5 graveyard=2 exile=0 battlefield=12 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Dowsing Device // Geode Grotto (P/T 0/0, dmg=0) [T]
    - A-Kenku Artificer (P/T 1/3, dmg=0) [T]
    - Plaza of Harmony (P/T 0/0, dmg=0) [T]
    - Fire Nation's Conquest (P/T 0/0, dmg=0)
    - Sneaky Homunculus (P/T 1/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Thunderblust (P/T 7/2, dmg=0) [T]
    - Accursed Duneyard (P/T 0/0, dmg=0) [T]
    - Lazotep Quarry (P/T 0/0, dmg=0) [T]
    - Zaffai and the Tempests (P/T 5/7, dmg=0) [T]
  Seat 3 [LOST]: life=-3 library=82 hand=3 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1812] add_mana seat=0 source=Forest amount=1 target=seat0
[1813] add_mana seat=0 source=Forest amount=1 target=seat0
[1814] add_mana seat=0 source=Forest amount=1 target=seat0
[1815] add_mana seat=0 source=Forest amount=1 target=seat0
[1816] add_mana seat=0 source=Forest amount=1 target=seat0
[1817] add_mana seat=0 source=Gond Gate amount=1 target=seat0
[1818] add_mana seat=0 source=Forest amount=1 target=seat0
[1819] add_mana seat=0 source=Mirrodin's Core amount=1 target=seat0
[1820] tap seat=0 source=Bandit's Haul target=seat0
[1821] pay_mana seat=0 source=Bandit's Haul amount=2 target=seat0
[1822] activate_ability seat=0 source=Bandit's Haul target=seat0
[1823] stack_push seat=0 source=Bandit's Haul target=seat0
[1824] priority_pass seat=2 source= target=seat0
[1825] stack_resolve seat=0 source=Bandit's Haul target=seat0
[1826] draw seat=0 source=Bandit's Haul target=seat0
[1827] activated_ability_resolved seat=0 source=Bandit's Haul target=seat0
[1828] sba_704_5b seat=0 source=
[1829] sba_cycle_complete seat=-1 source=
[1830] seat_eliminated seat=0 source= amount=18
[1831] game_end seat=2 source=
```

</details>

#### Violation 25

- **Game**: 426 (seed 4337778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 18, Phase=ending Step=cleanup
- **Commanders**: Gonti, Lord of Luxury, Muzzio, Visionary Architect, Shiko and Narset, Unified, Drana, Kalastria Bloodchief
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 734 with trigger-bearer(s) [{Syr Konrad, the Grim 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 18, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 743 events
  Seat 0 [alive]: life=40 library=86 hand=3 graveyard=3 exile=0 battlefield=6 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amonkhet Raceway (P/T 0/0, dmg=0) [T]
    - Dragon Engine (P/T 1/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Gonti, Lord of Luxury (P/T 2/3, dmg=0)
  Seat 1 [alive]: life=28 library=84 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wizard's Rockets (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=40 library=85 hand=4 graveyard=5 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Keeper of the Cadence (P/T 2/5, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=1 graveyard=4 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Drana, Kalastria Bloodchief (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Corrupted Grafstone (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Myr Sire (P/T 1/1, dmg=0)
    - Syr Konrad, the Grim (P/T 5/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[723] priority_pass seat=1 source= target=seat0
[724] stack_resolve seat=2 source=Keeper of the Cadence target=seat0
[725] bounce seat=2 source=Keeper of the Cadence target=seat0
[726] zone_change seat=0 source=Runed Servitor
[727] enter_battlefield seat=2 source=Keeper of the Cadence target=seat0
[728] phase_step seat=2 source= target=seat0
[729] declare_attackers seat=2 source= target=seat0
[730] blockers seat=1 source= target=seat0
[731] damage seat=2 source=Impostor of the Sixth Pride amount=3 target=seat1
[732] damage seat=1 source=Muzzio, Visionary Architect amount=1 target=seat2
[733] destroy seat=1 source=Muzzio, Visionary Architect
[734] sba_704_5g seat=1 source=Muzzio, Visionary Architect
[735] zone_change seat=1 source=Muzzio, Visionary Architect
[736] destroy seat=2 source=Impostor of the Sixth Pride
[737] sba_704_5g seat=2 source=Impostor of the Sixth Pride
[738] zone_change seat=2 source=Impostor of the Sixth Pride
[739] sba_704_6d seat=1 source=Muzzio, Visionary Architect
[740] sba_cycle_complete seat=-1 source=
[741] phase_step seat=2 source= target=seat0
[742] state seat=2 source= target=seat0
```

</details>

#### Violation 26

- **Game**: 426 (seed 4337778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 18, Phase=ending Step=cleanup
- **Commanders**: Gonti, Lord of Luxury, Muzzio, Visionary Architect, Shiko and Narset, Unified, Drana, Kalastria Bloodchief
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 734 with trigger-bearer(s) [{Syr Konrad, the Grim 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 18, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 743 events
  Seat 0 [alive]: life=40 library=86 hand=3 graveyard=3 exile=0 battlefield=6 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amonkhet Raceway (P/T 0/0, dmg=0) [T]
    - Dragon Engine (P/T 1/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Gonti, Lord of Luxury (P/T 2/3, dmg=0)
  Seat 1 [alive]: life=28 library=84 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wizard's Rockets (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=40 library=85 hand=4 graveyard=5 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Keeper of the Cadence (P/T 2/5, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=1 graveyard=4 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Drana, Kalastria Bloodchief (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Corrupted Grafstone (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Myr Sire (P/T 1/1, dmg=0)
    - Syr Konrad, the Grim (P/T 5/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[723] priority_pass seat=1 source= target=seat0
[724] stack_resolve seat=2 source=Keeper of the Cadence target=seat0
[725] bounce seat=2 source=Keeper of the Cadence target=seat0
[726] zone_change seat=0 source=Runed Servitor
[727] enter_battlefield seat=2 source=Keeper of the Cadence target=seat0
[728] phase_step seat=2 source= target=seat0
[729] declare_attackers seat=2 source= target=seat0
[730] blockers seat=1 source= target=seat0
[731] damage seat=2 source=Impostor of the Sixth Pride amount=3 target=seat1
[732] damage seat=1 source=Muzzio, Visionary Architect amount=1 target=seat2
[733] destroy seat=1 source=Muzzio, Visionary Architect
[734] sba_704_5g seat=1 source=Muzzio, Visionary Architect
[735] zone_change seat=1 source=Muzzio, Visionary Architect
[736] destroy seat=2 source=Impostor of the Sixth Pride
[737] sba_704_5g seat=2 source=Impostor of the Sixth Pride
[738] zone_change seat=2 source=Impostor of the Sixth Pride
[739] sba_704_6d seat=1 source=Muzzio, Visionary Architect
[740] sba_cycle_complete seat=-1 source=
[741] phase_step seat=2 source= target=seat0
[742] state seat=2 source= target=seat0
```

</details>

#### Violation 27

- **Game**: 426 (seed 4337778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 23, Phase=ending Step=cleanup
- **Commanders**: Gonti, Lord of Luxury, Muzzio, Visionary Architect, Shiko and Narset, Unified, Drana, Kalastria Bloodchief
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 1025 with trigger-bearer(s) [{Syr Konrad, the Grim 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1034 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=3 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amonkhet Raceway (P/T 0/0, dmg=0) [T]
    - Dragon Engine (P/T 1/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Gonti, Lord of Luxury (P/T 2/3, dmg=0) [T]
    - Altar of the Pantheon (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=9 library=83 hand=3 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wizard's Rockets (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=40 library=84 hand=3 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Keeper of the Cadence (P/T 2/5, dmg=0) [T]
    - Sejiri Steppe (P/T 0/0, dmg=0) [T]
    - Kira, Great Glass-Spinner (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=0 graveyard=4 exile=0 battlefield=11 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Corrupted Grafstone (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Myr Sire (P/T 1/1, dmg=0) [T]
    - Syr Konrad, the Grim (P/T 5/4, dmg=0) [T]
    - Fire Navy Trebuchet (P/T 0/4, dmg=0)
    - Horrible Hordes (P/T 2/2, dmg=0)
    - food artifact token Token (P/T 0/0, dmg=0)
    - Nuka-Cola Vending Machine (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1014] stack_resolve seat=3 source=Corrupted Grafstone target=seat0
[1015] parsed_effect_residual seat=3 source=Corrupted Grafstone target=seat0
[1016] activated_ability_resolved seat=3 source=Corrupted Grafstone target=seat0
[1017] phase_step seat=3 source= target=seat0
[1018] declare_attackers seat=3 source= target=seat0
[1019] blockers seat=1 source= target=seat0
[1020] damage seat=3 source=Drana, Kalastria Bloodchief amount=3 target=seat1
[1021] damage seat=3 source=Myr Sire amount=1 target=seat1
[1022] damage seat=3 source=Syr Konrad, the Grim amount=5 target=seat1
[1023] damage seat=1 source=Illusion Spinners amount=4 target=seat3
[1024] destroy seat=1 source=Illusion Spinners
[1025] sba_704_5g seat=1 source=Illusion Spinners
[1026] zone_change seat=1 source=Illusion Spinners
[1027] destroy seat=3 source=Drana, Kalastria Bloodchief
[1028] sba_704_5g seat=3 source=Drana, Kalastria Bloodchief
[1029] zone_change seat=3 source=Drana, Kalastria Bloodchief
[1030] sba_704_6d seat=3 source=Drana, Kalastria Bloodchief
[1031] sba_cycle_complete seat=-1 source=
[1032] phase_step seat=3 source= target=seat0
[1033] state seat=3 source= target=seat0
```

</details>

#### Violation 28

- **Game**: 426 (seed 4337778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 23, Phase=ending Step=cleanup
- **Commanders**: Gonti, Lord of Luxury, Muzzio, Visionary Architect, Shiko and Narset, Unified, Drana, Kalastria Bloodchief
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 1025 with trigger-bearer(s) [{Syr Konrad, the Grim 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1034 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=3 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amonkhet Raceway (P/T 0/0, dmg=0) [T]
    - Dragon Engine (P/T 1/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Gonti, Lord of Luxury (P/T 2/3, dmg=0) [T]
    - Altar of the Pantheon (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=9 library=83 hand=3 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Wizard's Rockets (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=40 library=84 hand=3 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Keeper of the Cadence (P/T 2/5, dmg=0) [T]
    - Sejiri Steppe (P/T 0/0, dmg=0) [T]
    - Kira, Great Glass-Spinner (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=0 graveyard=4 exile=0 battlefield=11 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Corrupted Grafstone (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Myr Sire (P/T 1/1, dmg=0) [T]
    - Syr Konrad, the Grim (P/T 5/4, dmg=0) [T]
    - Fire Navy Trebuchet (P/T 0/4, dmg=0)
    - Horrible Hordes (P/T 2/2, dmg=0)
    - food artifact token Token (P/T 0/0, dmg=0)
    - Nuka-Cola Vending Machine (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1014] stack_resolve seat=3 source=Corrupted Grafstone target=seat0
[1015] parsed_effect_residual seat=3 source=Corrupted Grafstone target=seat0
[1016] activated_ability_resolved seat=3 source=Corrupted Grafstone target=seat0
[1017] phase_step seat=3 source= target=seat0
[1018] declare_attackers seat=3 source= target=seat0
[1019] blockers seat=1 source= target=seat0
[1020] damage seat=3 source=Drana, Kalastria Bloodchief amount=3 target=seat1
[1021] damage seat=3 source=Myr Sire amount=1 target=seat1
[1022] damage seat=3 source=Syr Konrad, the Grim amount=5 target=seat1
[1023] damage seat=1 source=Illusion Spinners amount=4 target=seat3
[1024] destroy seat=1 source=Illusion Spinners
[1025] sba_704_5g seat=1 source=Illusion Spinners
[1026] zone_change seat=1 source=Illusion Spinners
[1027] destroy seat=3 source=Drana, Kalastria Bloodchief
[1028] sba_704_5g seat=3 source=Drana, Kalastria Bloodchief
[1029] zone_change seat=3 source=Drana, Kalastria Bloodchief
[1030] sba_704_6d seat=3 source=Drana, Kalastria Bloodchief
[1031] sba_cycle_complete seat=-1 source=
[1032] phase_step seat=3 source= target=seat0
[1033] state seat=3 source= target=seat0
```

</details>

#### Violation 29

- **Game**: 426 (seed 4337778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 29, Phase=ending Step=cleanup
- **Commanders**: Gonti, Lord of Luxury, Muzzio, Visionary Architect, Shiko and Narset, Unified, Drana, Kalastria Bloodchief
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 1486 with trigger-bearer(s) [{Syr Konrad, the Grim 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1495 events
  Seat 0 [alive]: life=32 library=82 hand=0 graveyard=7 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amonkhet Raceway (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Gonti, Lord of Luxury (P/T 2/3, dmg=0) [T]
    - Altar of the Pantheon (P/T 0/0, dmg=0)
    - Kalastria Healer (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wayfarer's Bauble (P/T 0/0, dmg=0)
    - creature token zombie artifact Token (P/T 3/3, dmg=0) [T]
  Seat 1 [LOST]: life=-3 library=82 hand=3 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=37 library=82 hand=3 graveyard=6 exile=0 battlefield=10 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Sejiri Steppe (P/T 0/0, dmg=0) [T]
    - Kira, Great Glass-Spinner (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Scampering Scorcher (P/T 1/1, dmg=0)
    - creature token red elemental Token (P/T 1/1, dmg=0) [T]
    - creature token red elemental Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=40 library=82 hand=0 graveyard=6 exile=0 battlefield=12 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Corrupted Grafstone (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Syr Konrad, the Grim (P/T 5/4, dmg=0) [T]
    - Fire Navy Trebuchet (P/T 0/4, dmg=0)
    - Horrible Hordes (P/T 2/2, dmg=0) [T]
    - food artifact token Token (P/T 0/0, dmg=0)
    - Nuka-Cola Vending Machine (P/T 0/0, dmg=0)
    - Drana, Kalastria Bloodchief (P/T 4/4, dmg=0)
    - creature token colorless phyrexian myr artifact Token (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1475] citys_blessing seat=2 source= amount=11 target=seat0
[1476] phase_step seat=2 source= target=seat0
[1477] declare_attackers seat=2 source= target=seat0
[1478] blockers seat=0 source= target=seat0
[1479] damage seat=2 source=Keeper of the Cadence amount=1 target=seat0
[1480] damage seat=2 source=Kira, Great Glass-Spinner amount=2 target=seat0
[1481] damage seat=2 source=Shiko and Narset, Unified amount=4 target=seat0
[1482] damage seat=2 source=creature token red elemental Token amount=1 target=seat0
[1483] damage seat=2 source=creature token red elemental Token amount=1 target=seat0
[1484] damage seat=0 source=Ashnod, Flesh Mechanist amount=1 target=seat2
[1485] destroy seat=0 source=Ashnod, Flesh Mechanist
[1486] sba_704_5g seat=0 source=Ashnod, Flesh Mechanist
[1487] zone_change seat=0 source=Ashnod, Flesh Mechanist
[1488] destroy seat=2 source=Keeper of the Cadence
[1489] sba_704_5g seat=2 source=Keeper of the Cadence
[1490] zone_change seat=2 source=Keeper of the Cadence
[1491] sba_cycle_complete seat=-1 source=
[1492] phase_step seat=2 source= target=seat0
[1493] pool_drain seat=2 source= amount=1 target=seat0
[1494] state seat=2 source= target=seat0
```

</details>

#### Violation 30

- **Game**: 426 (seed 4337778, perm 0)
- **Invariant**: TriggerCompleteness
- **Turn**: 29, Phase=ending Step=cleanup
- **Commanders**: Gonti, Lord of Luxury, Muzzio, Visionary Architect, Shiko and Narset, Unified, Drana, Kalastria Bloodchief
- **Message**: TriggerCompleteness: death event "sba_704_5g" at index 1486 with trigger-bearer(s) [{Syr Konrad, the Grim 3}] on battlefield, but no subsequent trigger/effect event found

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1495 events
  Seat 0 [alive]: life=32 library=82 hand=0 graveyard=7 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Amonkhet Raceway (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Gonti, Lord of Luxury (P/T 2/3, dmg=0) [T]
    - Altar of the Pantheon (P/T 0/0, dmg=0)
    - Kalastria Healer (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wayfarer's Bauble (P/T 0/0, dmg=0)
    - creature token zombie artifact Token (P/T 3/3, dmg=0) [T]
  Seat 1 [LOST]: life=-3 library=82 hand=3 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=37 library=82 hand=3 graveyard=6 exile=0 battlefield=10 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Sejiri Steppe (P/T 0/0, dmg=0) [T]
    - Kira, Great Glass-Spinner (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Scampering Scorcher (P/T 1/1, dmg=0)
    - creature token red elemental Token (P/T 1/1, dmg=0) [T]
    - creature token red elemental Token (P/T 1/1, dmg=0) [T]
  Seat 3 [alive]: life=40 library=82 hand=0 graveyard=6 exile=0 battlefield=12 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Corrupted Grafstone (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Syr Konrad, the Grim (P/T 5/4, dmg=0) [T]
    - Fire Navy Trebuchet (P/T 0/4, dmg=0)
    - Horrible Hordes (P/T 2/2, dmg=0) [T]
    - food artifact token Token (P/T 0/0, dmg=0)
    - Nuka-Cola Vending Machine (P/T 0/0, dmg=0)
    - Drana, Kalastria Bloodchief (P/T 4/4, dmg=0)
    - creature token colorless phyrexian myr artifact Token (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1475] citys_blessing seat=2 source= amount=11 target=seat0
[1476] phase_step seat=2 source= target=seat0
[1477] declare_attackers seat=2 source= target=seat0
[1478] blockers seat=0 source= target=seat0
[1479] damage seat=2 source=Keeper of the Cadence amount=1 target=seat0
[1480] damage seat=2 source=Kira, Great Glass-Spinner amount=2 target=seat0
[1481] damage seat=2 source=Shiko and Narset, Unified amount=4 target=seat0
[1482] damage seat=2 source=creature token red elemental Token amount=1 target=seat0
[1483] damage seat=2 source=creature token red elemental Token amount=1 target=seat0
[1484] damage seat=0 source=Ashnod, Flesh Mechanist amount=1 target=seat2
[1485] destroy seat=0 source=Ashnod, Flesh Mechanist
[1486] sba_704_5g seat=0 source=Ashnod, Flesh Mechanist
[1487] zone_change seat=0 source=Ashnod, Flesh Mechanist
[1488] destroy seat=2 source=Keeper of the Cadence
[1489] sba_704_5g seat=2 source=Keeper of the Cadence
[1490] zone_change seat=2 source=Keeper of the Cadence
[1491] sba_cycle_complete seat=-1 source=
[1492] phase_step seat=2 source= target=seat0
[1493] pool_drain seat=2 source= amount=1 target=seat0
[1494] state seat=2 source= target=seat0
```

</details>

*... and 4 more violations not shown.*

## Invariant Violations (Nightmare Boards)

| Invariant | Count |
|-----------|-------|
| TriggerCompleteness | 30 |
| ReplacementCompleteness | 9 |

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Kaya's Ghostform | 2 | 1 | 0.67 |
| 2 | Naga Oracle | 2 | 1 | 0.67 |
| 3 | Split the Party | 2 | 1 | 0.67 |
| 4 | Ribbon Snake | 2 | 1 | 0.67 |
| 5 | Kalitas, Bloodchief of Ghet | 2 | 1 | 0.67 |
| 6 | Syr Konrad, the Grim | 2 | 1 | 0.67 |
| 7 | Noctis, Heir Apparent | 2 | 1 | 0.67 |
| 8 | True Polymorph // True Polymorph | 2 | 1 | 0.67 |
| 9 | Coils of the Medusa | 2 | 1 | 0.67 |
| 10 | A-Plate Armor | 2 | 1 | 0.67 |

## Verdict: ISSUES FOUND

**73 total issues** across 500 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 34 invariant violations in chaos games
- 0 crashes in nightmare boards
- 39 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
