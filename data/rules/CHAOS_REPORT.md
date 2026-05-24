# Chaos Gauntlet Report

Generated: 2026-05-24T13:32:59-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36656 cards |
| Legendary Creatures | 3433 |
| Total Games | 5000 |
| Seed | 43 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 1m9.99s |
| Throughput | 71 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 1 (in 1 games) |
| Clean Games | 4999 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 1.058s |
| Throughput | 9456 boards/sec |
| Crashes | 0 |
| Invariant Violations | 0 |
| Clean Boards | 10000 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| SBACompleteness | 1 |

### Violation Details (up to 5 per invariant kind, 1 shown)

#### Violation 1

- **Game**: 1003 (seed 10030044, perm 0)
- **Invariant**: SBACompleteness
- **Turn**: 53, Phase=beginning Step=upkeep
- **Commanders**: Pianna, Nomad Captain, Scarlet Spider, Ben Reilly, Kuja, Genome Sorcerer // Trance Kuja, Fate Defied, Kudo, King Among Bears
- **Message**: seat 1 has creature "District Mascot" on battlefield with toughness=0 (layer=0) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 53, Phase=beginning Step=upkeep Active=seat0
Stack: 0 items, EventLog: 2956 events
  Seat 0 [alive]: life=2 library=75 hand=5 graveyard=10 exile=0 battlefield=9 cmdzone=1 mana=0
    - Blinkmoth Nexus (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Horizon of Progress (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Abbey Matron (P/T 1/3, dmg=0)
  Seat 1 [alive]: life=5 library=79 hand=2 graveyard=9 exile=0 battlefield=9 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Arch of Orazca (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Deserted Temple (P/T 0/0, dmg=0) [T]
    - Kuldotha Flamefiend (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - District Mascot (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=5 library=78 hand=5 graveyard=7 exile=0 battlefield=16 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Diamond Valley (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wizard Token (P/T 0/1, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Wizard Token (P/T 0/1, dmg=0)
    - Wizard Token (P/T 0/1, dmg=0)
    - Ally Encampment (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kuja, Genome Sorcerer // Trance Kuja, Fate Defied (P/T 3/4, dmg=0) [T]
    - Wizard Token (P/T 0/1, dmg=0)
    - Wizard Token (P/T 0/1, dmg=0)
    - Wizard Token (P/T 0/1, dmg=0) [T]
    - Wizard Token (P/T 0/1, dmg=0) [T]
  Seat 3 [LOST]: life=0 library=76 hand=0 graveyard=6 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2936] add_mana seat=3 source=Plains amount=1 target=seat0
[2937] tap seat=3 source=Stern Judge target=seat0
[2938] activate_ability seat=3 source=Stern Judge target=seat0
[2939] stack_push seat=3 source=Stern Judge target=seat0
[2940] priority_pass seat=0 source= target=seat0
[2941] priority_pass seat=1 source= target=seat0
[2942] priority_pass seat=2 source= target=seat0
[2943] stack_resolve seat=3 source=Stern Judge target=seat0
[2944] lose_life seat=3 source=Stern Judge amount=1 target=seat0
[2945] life_change seat=0 source=Stern Judge amount=-1 target=seat0
[2946] lose_life seat=3 source=Stern Judge amount=1 target=seat1
[2947] life_change seat=1 source=Stern Judge amount=-1 target=seat0
[2948] lose_life seat=3 source=Stern Judge amount=1 target=seat2
[2949] life_change seat=2 source=Stern Judge amount=-1 target=seat0
[2950] lose_life seat=3 source=Stern Judge amount=1 target=seat3
[2951] life_change seat=3 source=Stern Judge amount=-1 target=seat0
[2952] activated_ability_resolved seat=3 source=Stern Judge target=seat0
[2953] sba_704_5a seat=3 source=
[2954] sba_cycle_complete seat=-1 source=
[2955] seat_eliminated seat=3 source= amount=15
```

</details>

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Terminal Agony | 1 | 11 | 0.08 |
| 2 | Kuja, Genome Sorcerer // Trance Kuja, Fate Defied | 1 | 13 | 0.07 |
| 3 | Scarlet Spider, Ben Reilly | 1 | 14 | 0.07 |
| 4 | Kudo, King Among Bears | 1 | 15 | 0.06 |
| 5 | Graf Rats | 1 | 17 | 0.06 |
| 6 | Eccentric Farmer | 1 | 17 | 0.06 |
| 7 | Bristling Backwoods | 1 | 18 | 0.05 |
| 8 | Brightcap Badger // Fungus Frolic | 1 | 18 | 0.05 |
| 9 | Kuldotha Flamefiend | 1 | 18 | 0.05 |
| 10 | Ixalli's Lorekeeper | 1 | 18 | 0.05 |

## Verdict: ISSUES FOUND

**1 total issues** across 5000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 1 invariant violations in chaos games
- 0 crashes in nightmare boards
- 0 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
