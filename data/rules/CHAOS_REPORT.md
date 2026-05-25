# Chaos Gauntlet Report

Generated: 2026-05-25T04:06:41-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36656 cards |
| Legendary Creatures | 3433 |
| Total Games | 10000 |
| Seed | 2718 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 3m24.432s |
| Throughput | 49 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 2 (in 1 games) |
| Clean Games | 9999 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 1.039s |
| Throughput | 9629 boards/sec |
| Crashes | 0 |
| Invariant Violations | 0 |
| Clean Boards | 10000 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| WinCondition | 2 |

### Violation Details (up to 5 per invariant kind, 2 shown)

#### Violation 1

- **Game**: 3428 (seed 34282719, perm 0)
- **Invariant**: WinCondition
- **Turn**: 39, Phase=combat Step=end_of_combat
- **Commanders**: Genevieve, Conniving Dragon, Bartolomé del Presidio, Zndrsplt, Eye of Wisdom, Hapatra, Vizier of Poisons
- **Message**: WinCondition: seat 0 lost via poison but has only 2 poison counters (< 10)

<details>
<summary>Game State</summary>

```
Turn 39, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1229 events
  Seat 0 [LOST]: life=16 library=84 hand=4 graveyard=1 exile=1 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=15 library=80 hand=7 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=18 library=79 hand=1 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=22 library=80 hand=2 graveyard=6 exile=0 battlefield=11 cmdzone=0 mana=6
    - Witherbloom Campus (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Hapatra, Vizier of Poisons (P/T 11/11, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Hero's Blade (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Blightwidow (P/T 2/4, dmg=0) [T]
    - Ley Line (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Dakmor Scorpion (P/T 2/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1209] triggered_ability seat=3 source=Hapatra, Vizier of Poisons target=seat0
[1210] stack_push seat=3 source=Hapatra, Vizier of Poisons target=seat0
[1211] priority_pass seat=1 source= target=seat0
[1212] stack_resolve seat=3 source=Hapatra, Vizier of Poisons target=seat0
[1213] counter_mod seat=0 source=Hapatra, Vizier of Poisons amount=1 target=seat0
[1214] poison seat=3 source=Blightwidow amount=2 target=seat1
[1215] damage seat=3 source=Dakmor Scorpion amount=2 target=seat1
[1216] destroy seat=3 source=Broodguard Elite
[1217] sba_704_5f seat=3 source=Broodguard Elite
[1218] zone_change seat=3 source=Broodguard Elite
[1219] triggered_ability seat=3 source=Broodguard Elite target=seat0
[1220] stack_push seat=3 source=Broodguard Elite target=seat0
[1221] triggers_ordered seat=3 source= target=seat0
[1222] priority_pass seat=1 source= target=seat0
[1223] stack_resolve seat=3 source=Broodguard Elite target=seat0
[1224] counter_mod seat=0 source=Broodguard Elite amount=1 target=seat0
[1225] sba_704_6c seat=1 source=Hapatra, Vizier of Poisons amount=26
[1226] sba_cycle_complete seat=-1 source=
[1227] seat_eliminated seat=1 source=
[1228] game_end seat=3 source=
```

</details>

#### Violation 2

- **Game**: 3428 (seed 34282719, perm 0)
- **Invariant**: WinCondition
- **Turn**: 39, Phase=combat Step=end_of_combat
- **Commanders**: Genevieve, Conniving Dragon, Bartolomé del Presidio, Zndrsplt, Eye of Wisdom, Hapatra, Vizier of Poisons
- **Message**: WinCondition: seat 0 lost via poison but has only 2 poison counters (< 10)

<details>
<summary>Game State</summary>

```
Turn 39, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 1229 events
  Seat 0 [LOST]: life=16 library=84 hand=4 graveyard=1 exile=1 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=15 library=80 hand=7 graveyard=10 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [LOST]: life=18 library=79 hand=1 graveyard=8 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [WON]: life=22 library=80 hand=2 graveyard=6 exile=0 battlefield=11 cmdzone=0 mana=6
    - Witherbloom Campus (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Hapatra, Vizier of Poisons (P/T 11/11, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Hero's Blade (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Blightwidow (P/T 2/4, dmg=0) [T]
    - Ley Line (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Dakmor Scorpion (P/T 2/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1209] triggered_ability seat=3 source=Hapatra, Vizier of Poisons target=seat0
[1210] stack_push seat=3 source=Hapatra, Vizier of Poisons target=seat0
[1211] priority_pass seat=1 source= target=seat0
[1212] stack_resolve seat=3 source=Hapatra, Vizier of Poisons target=seat0
[1213] counter_mod seat=0 source=Hapatra, Vizier of Poisons amount=1 target=seat0
[1214] poison seat=3 source=Blightwidow amount=2 target=seat1
[1215] damage seat=3 source=Dakmor Scorpion amount=2 target=seat1
[1216] destroy seat=3 source=Broodguard Elite
[1217] sba_704_5f seat=3 source=Broodguard Elite
[1218] zone_change seat=3 source=Broodguard Elite
[1219] triggered_ability seat=3 source=Broodguard Elite target=seat0
[1220] stack_push seat=3 source=Broodguard Elite target=seat0
[1221] triggers_ordered seat=3 source= target=seat0
[1222] priority_pass seat=1 source= target=seat0
[1223] stack_resolve seat=3 source=Broodguard Elite target=seat0
[1224] counter_mod seat=0 source=Broodguard Elite amount=1 target=seat0
[1225] sba_704_6c seat=1 source=Hapatra, Vizier of Poisons amount=26
[1226] sba_cycle_complete seat=-1 source=
[1227] seat_eliminated seat=1 source=
[1228] game_end seat=3 source=
```

</details>

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Graceful Restoration | 1 | 6 | 0.14 |
| 2 | Identity Crisis | 1 | 11 | 0.08 |
| 3 | Etherium Abomination | 1 | 13 | 0.07 |
| 4 | Infuse with Vitality | 1 | 14 | 0.07 |
| 5 | Ichor Aberration | 1 | 15 | 0.06 |
| 6 | Ral's Staticaster | 1 | 18 | 0.05 |
| 7 | Hapatra, Vizier of Poisons | 1 | 18 | 0.05 |
| 8 | Genevieve, Conniving Dragon | 1 | 20 | 0.05 |
| 9 | Bartolomé del Presidio | 1 | 22 | 0.04 |
| 10 | Witherbloom Campus | 1 | 35 | 0.03 |

## Verdict: ISSUES FOUND

**2 total issues** across 10000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 2 invariant violations in chaos games
- 0 crashes in nightmare boards
- 0 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
