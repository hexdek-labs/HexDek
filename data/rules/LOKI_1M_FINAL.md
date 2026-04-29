# Chaos Gauntlet Report

Generated: 2026-04-17T16:53:45-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36510 cards |
| Legendary Creatures | 3434 |
| Total Games | 1000000 |
| Seed | 42 |
| Permutations | 10 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 3h39m49.457s |
| Throughput | 76 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 2432 (in 175 games) |
| Clean Games | 999825 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 1.438s |
| Throughput | 6955 boards/sec |
| Crashes | 0 |
| Invariant Violations | 0 |
| Clean Boards | 10000 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| SBACompleteness | 2432 |

### Violation Details (first 30)

#### Violation 1

- **Game**: 1421 (seed 14210043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: Celeborn the Wise, High Marshal Arguel, Myrel, Shield of Argive, Human Torch
- **Message**: seat 3 has creature "Arcbound Tracker" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 981 events
  Seat 0 [alive]: life=20 library=81 hand=2 graveyard=4 exile=0 battlefield=10 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Infinite Hourglass (P/T 0/0, dmg=0)
    - Rushwood Grove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Disciple of Freyalise // Garden of Freyalise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Celeborn the Wise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-2 library=84 hand=2 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=37 library=82 hand=4 graveyard=3 exile=0 battlefield=9 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Runesword (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Elixir (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=81 hand=1 graveyard=1 exile=0 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Valleymaker (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Cephalid Looter (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Human Torch (P/T 3/2, dmg=0) [T]
    - Arcbound Tracker (P/T 0/0, dmg=0)
    - Cradle Guard (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[961] priority_pass seat=0 source= target=seat0
[962] priority_pass seat=2 source= target=seat0
[963] stack_resolve seat=3 source=Cradle Guard target=seat0
[964] enter_battlefield seat=3 source=Cradle Guard target=seat0
[965] phase_step seat=3 source= target=seat0
[966] attackers seat=3 source= target=seat0
[967] trigger_fires seat=3 source=Human Torch target=seat0
[968] stack_push seat=3 source=Human Torch target=seat0
[969] priority_pass seat=0 source= target=seat0
[970] priority_pass seat=2 source= target=seat0
[971] stack_resolve seat=3 source=Human Torch target=seat0
[972] modification_effect seat=3 source=Human Torch target=seat0
[973] blockers seat=0 source= target=seat0
[974] damage seat=3 source=Shiko and Narset, Unified amount=4 target=seat0
[975] damage seat=3 source=Valleymaker amount=5 target=seat0
[976] damage seat=3 source=Cephalid Looter amount=2 target=seat0
[977] damage seat=3 source=Human Torch amount=3 target=seat0
[978] phase_step seat=3 source= target=seat0
[979] pool_drain seat=3 source= amount=2 target=seat0
[980] state seat=3 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 1421 (seed 14210043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: Celeborn the Wise, High Marshal Arguel, Myrel, Shield of Argive, Human Torch
- **Message**: seat 3 has creature "Arcbound Tracker" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 981 events
  Seat 0 [alive]: life=20 library=81 hand=2 graveyard=4 exile=0 battlefield=10 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Infinite Hourglass (P/T 0/0, dmg=0)
    - Rushwood Grove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Disciple of Freyalise // Garden of Freyalise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Celeborn the Wise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-2 library=84 hand=2 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=37 library=82 hand=4 graveyard=3 exile=0 battlefield=9 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Runesword (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Elixir (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=81 hand=1 graveyard=1 exile=0 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Valleymaker (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Cephalid Looter (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Human Torch (P/T 3/2, dmg=0) [T]
    - Arcbound Tracker (P/T 0/0, dmg=0)
    - Cradle Guard (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[961] priority_pass seat=0 source= target=seat0
[962] priority_pass seat=2 source= target=seat0
[963] stack_resolve seat=3 source=Cradle Guard target=seat0
[964] enter_battlefield seat=3 source=Cradle Guard target=seat0
[965] phase_step seat=3 source= target=seat0
[966] attackers seat=3 source= target=seat0
[967] trigger_fires seat=3 source=Human Torch target=seat0
[968] stack_push seat=3 source=Human Torch target=seat0
[969] priority_pass seat=0 source= target=seat0
[970] priority_pass seat=2 source= target=seat0
[971] stack_resolve seat=3 source=Human Torch target=seat0
[972] modification_effect seat=3 source=Human Torch target=seat0
[973] blockers seat=0 source= target=seat0
[974] damage seat=3 source=Shiko and Narset, Unified amount=4 target=seat0
[975] damage seat=3 source=Valleymaker amount=5 target=seat0
[976] damage seat=3 source=Cephalid Looter amount=2 target=seat0
[977] damage seat=3 source=Human Torch amount=3 target=seat0
[978] phase_step seat=3 source= target=seat0
[979] pool_drain seat=3 source= amount=2 target=seat0
[980] state seat=3 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 1421 (seed 14210043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: Celeborn the Wise, High Marshal Arguel, Myrel, Shield of Argive, Human Torch
- **Message**: seat 3 has creature "Arcbound Tracker" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1013 events
  Seat 0 [alive]: life=20 library=80 hand=1 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Infinite Hourglass (P/T 0/0, dmg=0)
    - Rushwood Grove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Disciple of Freyalise // Garden of Freyalise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Celeborn the Wise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-2 library=84 hand=2 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=34 library=82 hand=4 graveyard=3 exile=0 battlefield=9 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Runesword (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Elixir (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=81 hand=1 graveyard=1 exile=0 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Valleymaker (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Cephalid Looter (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Human Torch (P/T 3/2, dmg=0) [T]
    - Arcbound Tracker (P/T 0/0, dmg=0)
    - Cradle Guard (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[993] add_mana seat=0 source=Forest amount=1 target=seat0
[994] add_mana seat=0 source=Forest amount=1 target=seat0
[995] add_mana seat=0 source=Forest amount=1 target=seat0
[996] add_mana seat=0 source=Forest amount=1 target=seat0
[997] add_mana seat=0 source=Forest amount=1 target=seat0
[998] add_mana seat=0 source=Forest amount=1 target=seat0
[999] add_mana seat=0 source=Forest amount=1 target=seat0
[1000] cast seat=0 source=Will of the Jeskai // Will of the Jeskai target=seat0
[1001] stack_push seat=0 source=Will of the Jeskai // Will of the Jeskai target=seat0
[1002] priority_pass seat=2 source= target=seat0
[1003] priority_pass seat=3 source= target=seat0
[1004] stack_resolve seat=0 source=Will of the Jeskai // Will of the Jeskai target=seat0
[1005] resolve seat=0 source=Will of the Jeskai // Will of the Jeskai target=seat0
[1006] phase_step seat=0 source= target=seat0
[1007] attackers seat=0 source= target=seat0
[1008] blockers seat=2 source= target=seat0
[1009] damage seat=0 source=Celeborn the Wise amount=3 target=seat2
[1010] phase_step seat=0 source= target=seat0
[1011] pool_drain seat=0 source= amount=9 target=seat0
[1012] state seat=0 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 1421 (seed 14210043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: Celeborn the Wise, High Marshal Arguel, Myrel, Shield of Argive, Human Torch
- **Message**: seat 3 has creature "Arcbound Tracker" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1013 events
  Seat 0 [alive]: life=20 library=80 hand=1 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Infinite Hourglass (P/T 0/0, dmg=0)
    - Rushwood Grove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Disciple of Freyalise // Garden of Freyalise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Celeborn the Wise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-2 library=84 hand=2 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=34 library=82 hand=4 graveyard=3 exile=0 battlefield=9 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Runesword (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Elixir (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=81 hand=1 graveyard=1 exile=0 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Valleymaker (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Cephalid Looter (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Human Torch (P/T 3/2, dmg=0) [T]
    - Arcbound Tracker (P/T 0/0, dmg=0)
    - Cradle Guard (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[993] add_mana seat=0 source=Forest amount=1 target=seat0
[994] add_mana seat=0 source=Forest amount=1 target=seat0
[995] add_mana seat=0 source=Forest amount=1 target=seat0
[996] add_mana seat=0 source=Forest amount=1 target=seat0
[997] add_mana seat=0 source=Forest amount=1 target=seat0
[998] add_mana seat=0 source=Forest amount=1 target=seat0
[999] add_mana seat=0 source=Forest amount=1 target=seat0
[1000] cast seat=0 source=Will of the Jeskai // Will of the Jeskai target=seat0
[1001] stack_push seat=0 source=Will of the Jeskai // Will of the Jeskai target=seat0
[1002] priority_pass seat=2 source= target=seat0
[1003] priority_pass seat=3 source= target=seat0
[1004] stack_resolve seat=0 source=Will of the Jeskai // Will of the Jeskai target=seat0
[1005] resolve seat=0 source=Will of the Jeskai // Will of the Jeskai target=seat0
[1006] phase_step seat=0 source= target=seat0
[1007] attackers seat=0 source= target=seat0
[1008] blockers seat=2 source= target=seat0
[1009] damage seat=0 source=Celeborn the Wise amount=3 target=seat2
[1010] phase_step seat=0 source= target=seat0
[1011] pool_drain seat=0 source= amount=9 target=seat0
[1012] state seat=0 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 1421 (seed 14210043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 34, Phase=ending Step=cleanup
- **Commanders**: Celeborn the Wise, High Marshal Arguel, Myrel, Shield of Argive, Human Torch
- **Message**: seat 3 has creature "Arcbound Tracker" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1047 events
  Seat 0 [alive]: life=17 library=80 hand=1 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Infinite Hourglass (P/T 0/0, dmg=0)
    - Rushwood Grove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Disciple of Freyalise // Garden of Freyalise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Celeborn the Wise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-2 library=84 hand=2 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=34 library=81 hand=3 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Runesword (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Elixir (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Knight of the Tusk (P/T 3/7, dmg=0)
  Seat 3 [alive]: life=40 library=81 hand=1 graveyard=1 exile=0 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Valleymaker (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Cephalid Looter (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Human Torch (P/T 3/2, dmg=0) [T]
    - Arcbound Tracker (P/T 0/0, dmg=0)
    - Cradle Guard (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1027] add_mana seat=2 source=Plains amount=1 target=seat0
[1028] pay_mana seat=2 source=Knight of the Tusk amount=6 target=seat0
[1029] cast seat=2 source=Knight of the Tusk amount=6 target=seat0
[1030] stack_push seat=2 source=Knight of the Tusk target=seat0
[1031] priority_pass seat=3 source= target=seat0
[1032] priority_pass seat=0 source= target=seat0
[1033] stack_resolve seat=2 source=Knight of the Tusk target=seat0
[1034] enter_battlefield seat=2 source=Knight of the Tusk target=seat0
[1035] phase_step seat=2 source= target=seat0
[1036] attackers seat=2 source= target=seat0
[1037] trigger_fires seat=2 source=Myrel, Shield of Argive target=seat0
[1038] stack_push seat=2 source=Myrel, Shield of Argive target=seat0
[1039] priority_pass seat=3 source= target=seat0
[1040] priority_pass seat=0 source= target=seat0
[1041] stack_resolve seat=2 source=Myrel, Shield of Argive target=seat0
[1042] modification_effect seat=2 source=Myrel, Shield of Argive target=seat0
[1043] blockers seat=0 source= target=seat0
[1044] damage seat=2 source=Myrel, Shield of Argive amount=3 target=seat0
[1045] phase_step seat=2 source= target=seat0
[1046] state seat=2 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 1421 (seed 14210043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 34, Phase=ending Step=cleanup
- **Commanders**: Celeborn the Wise, High Marshal Arguel, Myrel, Shield of Argive, Human Torch
- **Message**: seat 3 has creature "Arcbound Tracker" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1047 events
  Seat 0 [alive]: life=17 library=80 hand=1 graveyard=5 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Infinite Hourglass (P/T 0/0, dmg=0)
    - Rushwood Grove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Disciple of Freyalise // Garden of Freyalise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Celeborn the Wise (P/T 3/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-2 library=84 hand=2 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=34 library=81 hand=3 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Runesword (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Elixir (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Knight of the Tusk (P/T 3/7, dmg=0)
  Seat 3 [alive]: life=40 library=81 hand=1 graveyard=1 exile=0 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Valleymaker (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Cephalid Looter (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Human Torch (P/T 3/2, dmg=0) [T]
    - Arcbound Tracker (P/T 0/0, dmg=0)
    - Cradle Guard (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1027] add_mana seat=2 source=Plains amount=1 target=seat0
[1028] pay_mana seat=2 source=Knight of the Tusk amount=6 target=seat0
[1029] cast seat=2 source=Knight of the Tusk amount=6 target=seat0
[1030] stack_push seat=2 source=Knight of the Tusk target=seat0
[1031] priority_pass seat=3 source= target=seat0
[1032] priority_pass seat=0 source= target=seat0
[1033] stack_resolve seat=2 source=Knight of the Tusk target=seat0
[1034] enter_battlefield seat=2 source=Knight of the Tusk target=seat0
[1035] phase_step seat=2 source= target=seat0
[1036] attackers seat=2 source= target=seat0
[1037] trigger_fires seat=2 source=Myrel, Shield of Argive target=seat0
[1038] stack_push seat=2 source=Myrel, Shield of Argive target=seat0
[1039] priority_pass seat=3 source= target=seat0
[1040] priority_pass seat=0 source= target=seat0
[1041] stack_resolve seat=2 source=Myrel, Shield of Argive target=seat0
[1042] modification_effect seat=2 source=Myrel, Shield of Argive target=seat0
[1043] blockers seat=0 source= target=seat0
[1044] damage seat=2 source=Myrel, Shield of Argive amount=3 target=seat0
[1045] phase_step seat=2 source= target=seat0
[1046] state seat=2 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 1421 (seed 14210043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 35, Phase=ending Step=cleanup
- **Commanders**: Celeborn the Wise, High Marshal Arguel, Myrel, Shield of Argive, Human Torch
- **Message**: seat 3 has creature "Arcbound Tracker" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 35, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1089 events
  Seat 0 [LOST]: life=-1 library=80 hand=1 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-2 library=84 hand=2 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=34 library=81 hand=3 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Runesword (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Elixir (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Knight of the Tusk (P/T 3/7, dmg=0)
  Seat 3 [alive]: life=40 library=80 hand=1 graveyard=1 exile=0 battlefield=16 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Valleymaker (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Cephalid Looter (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Human Torch (P/T 3/2, dmg=0) [T]
    - Arcbound Tracker (P/T 0/0, dmg=0)
    - Cradle Guard (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1069] phase_step seat=3 source= target=seat0
[1070] attackers seat=3 source= target=seat0
[1071] trigger_fires seat=3 source=Human Torch target=seat0
[1072] stack_push seat=3 source=Human Torch target=seat0
[1073] priority_pass seat=0 source= target=seat0
[1074] priority_pass seat=2 source= target=seat0
[1075] stack_resolve seat=3 source=Human Torch target=seat0
[1076] modification_effect seat=3 source=Human Torch target=seat0
[1077] blockers seat=0 source= target=seat0
[1078] damage seat=3 source=Shiko and Narset, Unified amount=4 target=seat0
[1079] damage seat=3 source=Valleymaker amount=5 target=seat0
[1080] damage seat=3 source=Cephalid Looter amount=2 target=seat0
[1081] damage seat=3 source=Human Torch amount=3 target=seat0
[1082] damage seat=3 source=Cradle Guard amount=4 target=seat0
[1083] sba_704_5a seat=0 source= amount=-1
[1084] sba_cycle_complete seat=-1 source=
[1085] seat_eliminated seat=0 source= amount=11
[1086] phase_step seat=3 source= target=seat0
[1087] pool_drain seat=3 source= amount=9 target=seat0
[1088] state seat=3 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 1421 (seed 14210043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 35, Phase=ending Step=cleanup
- **Commanders**: Celeborn the Wise, High Marshal Arguel, Myrel, Shield of Argive, Human Torch
- **Message**: seat 3 has creature "Arcbound Tracker" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 35, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1089 events
  Seat 0 [LOST]: life=-1 library=80 hand=1 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-2 library=84 hand=2 graveyard=5 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 2 [alive]: life=34 library=81 hand=3 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Runesword (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Elixir (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Knight of the Tusk (P/T 3/7, dmg=0)
  Seat 3 [alive]: life=40 library=80 hand=1 graveyard=1 exile=0 battlefield=16 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Shiko and Narset, Unified (P/T 4/4, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Valleymaker (P/T 5/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Xenagos, the Reveler (P/T 0/3, dmg=0)
    - Cephalid Looter (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Human Torch (P/T 3/2, dmg=0) [T]
    - Arcbound Tracker (P/T 0/0, dmg=0)
    - Cradle Guard (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1069] phase_step seat=3 source= target=seat0
[1070] attackers seat=3 source= target=seat0
[1071] trigger_fires seat=3 source=Human Torch target=seat0
[1072] stack_push seat=3 source=Human Torch target=seat0
[1073] priority_pass seat=0 source= target=seat0
[1074] priority_pass seat=2 source= target=seat0
[1075] stack_resolve seat=3 source=Human Torch target=seat0
[1076] modification_effect seat=3 source=Human Torch target=seat0
[1077] blockers seat=0 source= target=seat0
[1078] damage seat=3 source=Shiko and Narset, Unified amount=4 target=seat0
[1079] damage seat=3 source=Valleymaker amount=5 target=seat0
[1080] damage seat=3 source=Cephalid Looter amount=2 target=seat0
[1081] damage seat=3 source=Human Torch amount=3 target=seat0
[1082] damage seat=3 source=Cradle Guard amount=4 target=seat0
[1083] sba_704_5a seat=0 source= amount=-1
[1084] sba_cycle_complete seat=-1 source=
[1085] seat_eliminated seat=0 source= amount=11
[1086] phase_step seat=3 source= target=seat0
[1087] pool_drain seat=3 source= amount=9 target=seat0
[1088] state seat=3 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 1715 (seed 17150043, perm 3)
- **Invariant**: SBACompleteness
- **Turn**: 31, Phase=ending Step=cleanup
- **Commanders**: The Majestic Duo, Trelasarra, Moon Dancer, Jetmir, Nexus of Revels, Iname, Life Aspect
- **Message**: seat 0 has creature "Spike Weaver" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 746 events
  Seat 0 [alive]: life=40 library=80 hand=1 graveyard=8 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cryptic Coat (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - The Majestic Duo (P/T 2/2, dmg=0) [T]
    - Apes of Rath (P/T 5/4, dmg=0) [T]
    - Pheres-Band Warchief (P/T 3/3, dmg=0)
    - Raucous Audience (P/T 2/1, dmg=0) [T]
    - Level Up (P/T 0/0, dmg=0)
    - Spike Weaver (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=40 library=82 hand=0 graveyard=2 exile=0 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sledding Otter-Penguin (P/T 2/3, dmg=0) [T]
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Brightcap Badger // Fungus Frolic (P/T 4/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Multiform Wonder (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Selesnya Sentry (P/T 3/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Trelasarra, Moon Dancer (P/T 2/2, dmg=0)
  Seat 2 [LOST]: life=-7 library=83 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=14 library=83 hand=5 graveyard=7 exile=0 battlefield=3 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forge of Heroes (P/T 0/0, dmg=0) [T]
    - Screaming Shield (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[726] priority_pass seat=1 source= target=seat0
[727] priority_pass seat=3 source= target=seat0
[728] stack_resolve seat=0 source=Spike Weaver target=seat0
[729] counter_mod seat=0 source=Spike Weaver amount=1 target=seat0
[730] enter_battlefield seat=0 source=Spike Weaver target=seat0
[731] phase_step seat=0 source= target=seat0
[732] attackers seat=0 source= target=seat0
[733] trigger_fires seat=0 source=Apes of Rath target=seat0
[734] stack_push seat=0 source=Apes of Rath target=seat0
[735] priority_pass seat=1 source= target=seat0
[736] priority_pass seat=3 source= target=seat0
[737] stack_resolve seat=0 source=Apes of Rath target=seat0
[738] stun seat=0 source=Apes of Rath target=seat0
[739] blockers seat=3 source= target=seat0
[740] damage seat=0 source=The Majestic Duo amount=2 target=seat3
[741] damage seat=0 source=Apes of Rath amount=5 target=seat3
[742] damage seat=0 source=Pheres-Band Warchief amount=3 target=seat3
[743] phase_step seat=0 source= target=seat0
[744] pool_drain seat=0 source= amount=1 target=seat0
[745] state seat=0 source= target=seat0
```

</details>

#### Violation 10

- **Game**: 1715 (seed 17150043, perm 3)
- **Invariant**: SBACompleteness
- **Turn**: 31, Phase=ending Step=cleanup
- **Commanders**: The Majestic Duo, Trelasarra, Moon Dancer, Jetmir, Nexus of Revels, Iname, Life Aspect
- **Message**: seat 0 has creature "Spike Weaver" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 746 events
  Seat 0 [alive]: life=40 library=80 hand=1 graveyard=8 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cryptic Coat (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - The Majestic Duo (P/T 2/2, dmg=0) [T]
    - Apes of Rath (P/T 5/4, dmg=0) [T]
    - Pheres-Band Warchief (P/T 3/3, dmg=0)
    - Raucous Audience (P/T 2/1, dmg=0) [T]
    - Level Up (P/T 0/0, dmg=0)
    - Spike Weaver (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=40 library=82 hand=0 graveyard=2 exile=0 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sledding Otter-Penguin (P/T 2/3, dmg=0) [T]
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Brightcap Badger // Fungus Frolic (P/T 4/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Multiform Wonder (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Selesnya Sentry (P/T 3/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Trelasarra, Moon Dancer (P/T 2/2, dmg=0)
  Seat 2 [LOST]: life=-7 library=83 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [alive]: life=14 library=83 hand=5 graveyard=7 exile=0 battlefield=3 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forge of Heroes (P/T 0/0, dmg=0) [T]
    - Screaming Shield (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[726] priority_pass seat=1 source= target=seat0
[727] priority_pass seat=3 source= target=seat0
[728] stack_resolve seat=0 source=Spike Weaver target=seat0
[729] counter_mod seat=0 source=Spike Weaver amount=1 target=seat0
[730] enter_battlefield seat=0 source=Spike Weaver target=seat0
[731] phase_step seat=0 source= target=seat0
[732] attackers seat=0 source= target=seat0
[733] trigger_fires seat=0 source=Apes of Rath target=seat0
[734] stack_push seat=0 source=Apes of Rath target=seat0
[735] priority_pass seat=1 source= target=seat0
[736] priority_pass seat=3 source= target=seat0
[737] stack_resolve seat=0 source=Apes of Rath target=seat0
[738] stun seat=0 source=Apes of Rath target=seat0
[739] blockers seat=3 source= target=seat0
[740] damage seat=0 source=The Majestic Duo amount=2 target=seat3
[741] damage seat=0 source=Apes of Rath amount=5 target=seat3
[742] damage seat=0 source=Pheres-Band Warchief amount=3 target=seat3
[743] phase_step seat=0 source= target=seat0
[744] pool_drain seat=0 source= amount=1 target=seat0
[745] state seat=0 source= target=seat0
```

</details>

#### Violation 11

- **Game**: 1715 (seed 17150043, perm 3)
- **Invariant**: SBACompleteness
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: The Majestic Duo, Trelasarra, Moon Dancer, Jetmir, Nexus of Revels, Iname, Life Aspect
- **Message**: seat 0 has creature "Spike Weaver" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 791 events
  Seat 0 [alive]: life=40 library=80 hand=1 graveyard=8 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cryptic Coat (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - The Majestic Duo (P/T 2/2, dmg=0) [T]
    - Apes of Rath (P/T 5/4, dmg=0) [T]
    - Pheres-Band Warchief (P/T 3/3, dmg=0)
    - Raucous Audience (P/T 2/1, dmg=0) [T]
    - Level Up (P/T 0/0, dmg=0)
    - Spike Weaver (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=40 library=81 hand=0 graveyard=2 exile=0 battlefield=15 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sledding Otter-Penguin (P/T 2/3, dmg=0) [T]
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Brightcap Badger // Fungus Frolic (P/T 4/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Multiform Wonder (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Selesnya Sentry (P/T 3/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Trelasarra, Moon Dancer (P/T 2/2, dmg=0) [T]
    - Daring Archaeologist (P/T 3/3, dmg=0)
  Seat 2 [LOST]: life=-7 library=83 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=0 library=83 hand=5 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[771] stack_resolve seat=1 source=Daring Archaeologist target=seat0
[772] enter_battlefield seat=1 source=Daring Archaeologist target=seat0
[773] stack_push seat=1 source=Daring Archaeologist target=seat0
[774] priority_pass seat=3 source= target=seat0
[775] priority_pass seat=0 source= target=seat0
[776] stack_resolve seat=1 source=Daring Archaeologist target=seat0
[777] phase_step seat=1 source= target=seat0
[778] attackers seat=1 source= target=seat0
[779] blockers seat=3 source= target=seat0
[780] damage seat=1 source=Sledding Otter-Penguin amount=2 target=seat3
[781] damage seat=1 source=Brightcap Badger // Fungus Frolic amount=4 target=seat3
[782] damage seat=1 source=Multiform Wonder amount=3 target=seat3
[783] damage seat=1 source=Selesnya Sentry amount=3 target=seat3
[784] damage seat=1 source=Trelasarra, Moon Dancer amount=2 target=seat3
[785] sba_704_5a seat=3 source=
[786] sba_cycle_complete seat=-1 source=
[787] seat_eliminated seat=3 source= amount=3
[788] phase_step seat=1 source= target=seat0
[789] pool_drain seat=1 source= amount=4 target=seat0
[790] state seat=1 source= target=seat0
```

</details>

#### Violation 12

- **Game**: 1715 (seed 17150043, perm 3)
- **Invariant**: SBACompleteness
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: The Majestic Duo, Trelasarra, Moon Dancer, Jetmir, Nexus of Revels, Iname, Life Aspect
- **Message**: seat 0 has creature "Spike Weaver" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 791 events
  Seat 0 [alive]: life=40 library=80 hand=1 graveyard=8 exile=0 battlefield=11 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cryptic Coat (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - The Majestic Duo (P/T 2/2, dmg=0) [T]
    - Apes of Rath (P/T 5/4, dmg=0) [T]
    - Pheres-Band Warchief (P/T 3/3, dmg=0)
    - Raucous Audience (P/T 2/1, dmg=0) [T]
    - Level Up (P/T 0/0, dmg=0)
    - Spike Weaver (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=40 library=81 hand=0 graveyard=2 exile=0 battlefield=15 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sledding Otter-Penguin (P/T 2/3, dmg=0) [T]
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Brightcap Badger // Fungus Frolic (P/T 4/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Multiform Wonder (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Selesnya Sentry (P/T 3/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Trelasarra, Moon Dancer (P/T 2/2, dmg=0) [T]
    - Daring Archaeologist (P/T 3/3, dmg=0)
  Seat 2 [LOST]: life=-7 library=83 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=0 library=83 hand=5 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[771] stack_resolve seat=1 source=Daring Archaeologist target=seat0
[772] enter_battlefield seat=1 source=Daring Archaeologist target=seat0
[773] stack_push seat=1 source=Daring Archaeologist target=seat0
[774] priority_pass seat=3 source= target=seat0
[775] priority_pass seat=0 source= target=seat0
[776] stack_resolve seat=1 source=Daring Archaeologist target=seat0
[777] phase_step seat=1 source= target=seat0
[778] attackers seat=1 source= target=seat0
[779] blockers seat=3 source= target=seat0
[780] damage seat=1 source=Sledding Otter-Penguin amount=2 target=seat3
[781] damage seat=1 source=Brightcap Badger // Fungus Frolic amount=4 target=seat3
[782] damage seat=1 source=Multiform Wonder amount=3 target=seat3
[783] damage seat=1 source=Selesnya Sentry amount=3 target=seat3
[784] damage seat=1 source=Trelasarra, Moon Dancer amount=2 target=seat3
[785] sba_704_5a seat=3 source=
[786] sba_cycle_complete seat=-1 source=
[787] seat_eliminated seat=3 source= amount=3
[788] phase_step seat=1 source= target=seat0
[789] pool_drain seat=1 source= amount=4 target=seat0
[790] state seat=1 source= target=seat0
```

</details>

#### Violation 13

- **Game**: 1715 (seed 17150043, perm 3)
- **Invariant**: SBACompleteness
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: The Majestic Duo, Trelasarra, Moon Dancer, Jetmir, Nexus of Revels, Iname, Life Aspect
- **Message**: seat 0 has creature "Spike Weaver" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 825 events
  Seat 0 [alive]: life=40 library=79 hand=1 graveyard=9 exile=0 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cryptic Coat (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Apes of Rath (P/T 5/4, dmg=0) [T]
    - Pheres-Band Warchief (P/T 3/3, dmg=0)
    - Raucous Audience (P/T 2/1, dmg=0) [T]
    - Level Up (P/T 0/0, dmg=0)
    - Spike Weaver (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=37 library=81 hand=0 graveyard=3 exile=0 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sledding Otter-Penguin (P/T 2/3, dmg=0) [T]
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Brightcap Badger // Fungus Frolic (P/T 4/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Multiform Wonder (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Selesnya Sentry (P/T 3/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Trelasarra, Moon Dancer (P/T 2/2, dmg=0) [T]
  Seat 2 [LOST]: life=-7 library=83 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=0 library=83 hand=5 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[805] priority_pass seat=1 source= target=seat0
[806] stack_resolve seat=0 source=Ezio Auditore da Firenze // Ezio Auditore da Firenze target=seat0
[807] resolve seat=0 source=Ezio Auditore da Firenze // Ezio Auditore da Firenze target=seat0
[808] phase_step seat=0 source= target=seat0
[809] attackers seat=0 source= target=seat0
[810] blockers seat=1 source= target=seat0
[811] damage seat=0 source=The Majestic Duo amount=2 target=seat1
[812] damage seat=0 source=Pheres-Band Warchief amount=3 target=seat1
[813] damage seat=1 source=Daring Archaeologist amount=3 target=seat0
[814] destroy seat=0 source=The Majestic Duo
[815] sba_704_5g seat=0 source=The Majestic Duo
[816] zone_change seat=0 source=The Majestic Duo
[817] destroy seat=1 source=Daring Archaeologist
[818] sba_704_5g seat=1 source=Daring Archaeologist
[819] zone_change seat=1 source=Daring Archaeologist
[820] sba_704_6d seat=0 source=The Majestic Duo
[821] sba_cycle_complete seat=-1 source=
[822] phase_step seat=0 source= target=seat0
[823] pool_drain seat=0 source= amount=5 target=seat0
[824] state seat=0 source= target=seat0
```

</details>

#### Violation 14

- **Game**: 1715 (seed 17150043, perm 3)
- **Invariant**: SBACompleteness
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: The Majestic Duo, Trelasarra, Moon Dancer, Jetmir, Nexus of Revels, Iname, Life Aspect
- **Message**: seat 0 has creature "Spike Weaver" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 825 events
  Seat 0 [alive]: life=40 library=79 hand=1 graveyard=9 exile=0 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cryptic Coat (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Apes of Rath (P/T 5/4, dmg=0) [T]
    - Pheres-Band Warchief (P/T 3/3, dmg=0)
    - Raucous Audience (P/T 2/1, dmg=0) [T]
    - Level Up (P/T 0/0, dmg=0)
    - Spike Weaver (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=37 library=81 hand=0 graveyard=3 exile=0 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Sledding Otter-Penguin (P/T 2/3, dmg=0) [T]
    - Terminal Moraine (P/T 0/0, dmg=0) [T]
    - Brightcap Badger // Fungus Frolic (P/T 4/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Multiform Wonder (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Selesnya Sentry (P/T 3/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Trelasarra, Moon Dancer (P/T 2/2, dmg=0) [T]
  Seat 2 [LOST]: life=-7 library=83 hand=6 graveyard=4 exile=0 battlefield=0 cmdzone=1 mana=0
  Seat 3 [LOST]: life=0 library=83 hand=5 graveyard=7 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[805] priority_pass seat=1 source= target=seat0
[806] stack_resolve seat=0 source=Ezio Auditore da Firenze // Ezio Auditore da Firenze target=seat0
[807] resolve seat=0 source=Ezio Auditore da Firenze // Ezio Auditore da Firenze target=seat0
[808] phase_step seat=0 source= target=seat0
[809] attackers seat=0 source= target=seat0
[810] blockers seat=1 source= target=seat0
[811] damage seat=0 source=The Majestic Duo amount=2 target=seat1
[812] damage seat=0 source=Pheres-Band Warchief amount=3 target=seat1
[813] damage seat=1 source=Daring Archaeologist amount=3 target=seat0
[814] destroy seat=0 source=The Majestic Duo
[815] sba_704_5g seat=0 source=The Majestic Duo
[816] zone_change seat=0 source=The Majestic Duo
[817] destroy seat=1 source=Daring Archaeologist
[818] sba_704_5g seat=1 source=Daring Archaeologist
[819] zone_change seat=1 source=Daring Archaeologist
[820] sba_704_6d seat=0 source=The Majestic Duo
[821] sba_cycle_complete seat=-1 source=
[822] phase_step seat=0 source= target=seat0
[823] pool_drain seat=0 source= amount=5 target=seat0
[824] state seat=0 source= target=seat0
```

</details>

#### Violation 15

- **Game**: 1796 (seed 17960043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 28, Phase=ending Step=cleanup
- **Commanders**: Anthousa, Setessan Hero, Carmen, Cruel Skymarcher, Donatello, Way with Machines, Amy Rose
- **Message**: seat 2 has creature "Rampaging Monument" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 721 events
  Seat 0 [alive]: life=38 library=85 hand=2 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Homeward Path (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Twinblade Slasher (P/T 1/1, dmg=0) [T]
    - Arm-Mounted Anchor (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=50 library=83 hand=2 graveyard=2 exile=0 battlefield=9 cmdzone=1 mana=0
    - Grasping Dunes (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Knight of Meadowgrain (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lys Alana Scarblade (P/T 2/2, dmg=0) [T]
    - Hex Parasite (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Insatiable Hemophage (P/T 3/3, dmg=0)
  Seat 2 [alive]: life=27 library=83 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Cloudseeder (P/T 1/1, dmg=0) [T]
    - Frostwalk Bastion (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rampaging Monument (P/T 0/0, dmg=0)
  Seat 3 [LOST]: life=-6 library=86 hand=4 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[701] untap_done seat=2 source=Frostwalk Bastion target=seat0
[702] untap_done seat=2 source=Island target=seat0
[703] draw seat=2 source=Island amount=1 target=seat0
[704] play_land seat=2 source=Island target=seat0
[705] add_mana seat=2 source=Island amount=1 target=seat0
[706] add_mana seat=2 source=Island amount=1 target=seat0
[707] add_mana seat=2 source=Island amount=1 target=seat0
[708] pay_mana seat=2 source=Rampaging Monument amount=4 target=seat0
[709] cast seat=2 source=Rampaging Monument amount=4 target=seat0
[710] stack_push seat=2 source=Rampaging Monument target=seat0
[711] priority_pass seat=0 source= target=seat0
[712] priority_pass seat=1 source= target=seat0
[713] stack_resolve seat=2 source=Rampaging Monument target=seat0
[714] enter_battlefield seat=2 source=Rampaging Monument target=seat0
[715] phase_step seat=2 source= target=seat0
[716] attackers seat=2 source= target=seat0
[717] blockers seat=0 source= target=seat0
[718] damage seat=2 source=Cloudseeder amount=1 target=seat0
[719] phase_step seat=2 source= target=seat0
[720] state seat=2 source= target=seat0
```

</details>

#### Violation 16

- **Game**: 1796 (seed 17960043, perm 7)
- **Invariant**: SBACompleteness
- **Turn**: 28, Phase=ending Step=cleanup
- **Commanders**: Anthousa, Setessan Hero, Carmen, Cruel Skymarcher, Donatello, Way with Machines, Amy Rose
- **Message**: seat 2 has creature "Rampaging Monument" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 721 events
  Seat 0 [alive]: life=38 library=85 hand=2 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Homeward Path (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Twinblade Slasher (P/T 1/1, dmg=0) [T]
    - Arm-Mounted Anchor (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=50 library=83 hand=2 graveyard=2 exile=0 battlefield=9 cmdzone=1 mana=0
    - Grasping Dunes (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Knight of Meadowgrain (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lys Alana Scarblade (P/T 2/2, dmg=0) [T]
    - Hex Parasite (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Insatiable Hemophage (P/T 3/3, dmg=0)
  Seat 2 [alive]: life=27 library=83 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Cloudseeder (P/T 1/1, dmg=0) [T]
    - Frostwalk Bastion (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rampaging Monument (P/T 0/0, dmg=0)
  Seat 3 [LOST]: life=-6 library=86 hand=4 graveyard=3 exile=0 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[701] untap_done seat=2 source=Frostwalk Bastion target=seat0
[702] untap_done seat=2 source=Island target=seat0
[703] draw seat=2 source=Island amount=1 target=seat0
[704] play_land seat=2 source=Island target=seat0
[705] add_mana seat=2 source=Island amount=1 target=seat0
[706] add_mana seat=2 source=Island amount=1 target=seat0
[707] add_mana seat=2 source=Island amount=1 target=seat0
[708] pay_mana seat=2 source=Rampaging Monument amount=4 target=seat0
[709] cast seat=2 source=Rampaging Monument amount=4 target=seat0
[710] stack_push seat=2 source=Rampaging Monument target=seat0
[711] priority_pass seat=0 source= target=seat0
[712] priority_pass seat=1 source= target=seat0
[713] stack_resolve seat=2 source=Rampaging Monument target=seat0
[714] enter_battlefield seat=2 source=Rampaging Monument target=seat0
[715] phase_step seat=2 source= target=seat0
[716] attackers seat=2 source= target=seat0
[717] blockers seat=0 source= target=seat0
[718] damage seat=2 source=Cloudseeder amount=1 target=seat0
[719] phase_step seat=2 source= target=seat0
[720] state seat=2 source= target=seat0
```

</details>

#### Violation 17

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 28, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 703 events
  Seat 0 [alive]: life=40 library=82 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=42 library=84 hand=3 graveyard=4 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0)
  Seat 2 [alive]: life=20 library=85 hand=3 graveyard=3 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=85 hand=4 graveyard=6 exile=0 battlefield=5 cmdzone=0 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Kenessos, Priest of Thassa (P/T 1/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[683] damage seat=0 source=ED-E, Lonesome Eyebot amount=2 target=seat2
[684] damage seat=2 source=Elesh Norn // The Argent Etchings amount=3 target=seat0
[685] trigger_fires seat=2 source=Elesh Norn // The Argent Etchings amount=3 target=seat0
[686] stack_push seat=2 source=Elesh Norn // The Argent Etchings target=seat0
[687] priority_pass seat=0 source= target=seat0
[688] priority_pass seat=1 source= target=seat0
[689] priority_pass seat=3 source= target=seat0
[690] stack_resolve seat=2 source=Elesh Norn // The Argent Etchings target=seat0
[691] modification_effect seat=2 source=Elesh Norn // The Argent Etchings target=seat0
[692] destroy seat=0 source=ED-E, Lonesome Eyebot
[693] sba_704_5g seat=0 source=ED-E, Lonesome Eyebot
[694] zone_change seat=0 source=ED-E, Lonesome Eyebot
[695] destroy seat=2 source=Elesh Norn // The Argent Etchings
[696] sba_704_5g seat=2 source=Elesh Norn // The Argent Etchings
[697] zone_change seat=2 source=Elesh Norn // The Argent Etchings
[698] sba_704_6d seat=0 source=ED-E, Lonesome Eyebot
[699] sba_704_6d seat=2 source=Elesh Norn // The Argent Etchings
[700] sba_cycle_complete seat=-1 source=
[701] phase_step seat=0 source= target=seat0
[702] state seat=0 source= target=seat0
```

</details>

#### Violation 18

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 28, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 703 events
  Seat 0 [alive]: life=40 library=82 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=42 library=84 hand=3 graveyard=4 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0)
  Seat 2 [alive]: life=20 library=85 hand=3 graveyard=3 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=85 hand=4 graveyard=6 exile=0 battlefield=5 cmdzone=0 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Kenessos, Priest of Thassa (P/T 1/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[683] damage seat=0 source=ED-E, Lonesome Eyebot amount=2 target=seat2
[684] damage seat=2 source=Elesh Norn // The Argent Etchings amount=3 target=seat0
[685] trigger_fires seat=2 source=Elesh Norn // The Argent Etchings amount=3 target=seat0
[686] stack_push seat=2 source=Elesh Norn // The Argent Etchings target=seat0
[687] priority_pass seat=0 source= target=seat0
[688] priority_pass seat=1 source= target=seat0
[689] priority_pass seat=3 source= target=seat0
[690] stack_resolve seat=2 source=Elesh Norn // The Argent Etchings target=seat0
[691] modification_effect seat=2 source=Elesh Norn // The Argent Etchings target=seat0
[692] destroy seat=0 source=ED-E, Lonesome Eyebot
[693] sba_704_5g seat=0 source=ED-E, Lonesome Eyebot
[694] zone_change seat=0 source=ED-E, Lonesome Eyebot
[695] destroy seat=2 source=Elesh Norn // The Argent Etchings
[696] sba_704_5g seat=2 source=Elesh Norn // The Argent Etchings
[697] zone_change seat=2 source=Elesh Norn // The Argent Etchings
[698] sba_704_6d seat=0 source=ED-E, Lonesome Eyebot
[699] sba_704_6d seat=2 source=Elesh Norn // The Argent Etchings
[700] sba_cycle_complete seat=-1 source=
[701] phase_step seat=0 source= target=seat0
[702] state seat=0 source= target=seat0
```

</details>

#### Violation 19

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 29, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 741 events
  Seat 0 [alive]: life=40 library=82 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=43 library=83 hand=2 graveyard=5 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=13 library=85 hand=3 graveyard=3 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=85 hand=4 graveyard=6 exile=0 battlefield=5 cmdzone=0 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Kenessos, Priest of Thassa (P/T 1/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[721] priority_pass seat=0 source= target=seat0
[722] stack_resolve seat=1 source=Marketback Walker target=seat0
[723] enter_battlefield seat=1 source=Marketback Walker target=seat0
[724] cast seat=1 source=Mahadi, Emporium Master // Mahadi, Emporium Master target=seat0
[725] stack_push seat=1 source=Mahadi, Emporium Master // Mahadi, Emporium Master target=seat0
[726] priority_pass seat=2 source= target=seat0
[727] priority_pass seat=3 source= target=seat0
[728] priority_pass seat=0 source= target=seat0
[729] stack_resolve seat=1 source=Mahadi, Emporium Master // Mahadi, Emporium Master target=seat0
[730] resolve seat=1 source=Mahadi, Emporium Master // Mahadi, Emporium Master target=seat0
[731] phase_step seat=1 source= target=seat0
[732] attackers seat=1 source= target=seat0
[733] blockers seat=2 source= target=seat0
[734] damage seat=1 source=Katsumasa, the Animator amount=3 target=seat2
[735] damage seat=1 source=Shadowmage Infiltrator amount=1 target=seat2
[736] damage seat=1 source=Harvest Hand // Scrounged Scythe amount=2 target=seat2
[737] damage seat=1 source=Venser, Corpse Puppet amount=1 target=seat2
[738] life_change seat=1 source=Venser, Corpse Puppet amount=1 target=seat0
[739] phase_step seat=1 source= target=seat0
[740] state seat=1 source= target=seat0
```

</details>

#### Violation 20

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 29, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 741 events
  Seat 0 [alive]: life=40 library=82 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=43 library=83 hand=2 graveyard=5 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=13 library=85 hand=3 graveyard=3 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=85 hand=4 graveyard=6 exile=0 battlefield=5 cmdzone=0 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Kenessos, Priest of Thassa (P/T 1/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[721] priority_pass seat=0 source= target=seat0
[722] stack_resolve seat=1 source=Marketback Walker target=seat0
[723] enter_battlefield seat=1 source=Marketback Walker target=seat0
[724] cast seat=1 source=Mahadi, Emporium Master // Mahadi, Emporium Master target=seat0
[725] stack_push seat=1 source=Mahadi, Emporium Master // Mahadi, Emporium Master target=seat0
[726] priority_pass seat=2 source= target=seat0
[727] priority_pass seat=3 source= target=seat0
[728] priority_pass seat=0 source= target=seat0
[729] stack_resolve seat=1 source=Mahadi, Emporium Master // Mahadi, Emporium Master target=seat0
[730] resolve seat=1 source=Mahadi, Emporium Master // Mahadi, Emporium Master target=seat0
[731] phase_step seat=1 source= target=seat0
[732] attackers seat=1 source= target=seat0
[733] blockers seat=2 source= target=seat0
[734] damage seat=1 source=Katsumasa, the Animator amount=3 target=seat2
[735] damage seat=1 source=Shadowmage Infiltrator amount=1 target=seat2
[736] damage seat=1 source=Harvest Hand // Scrounged Scythe amount=2 target=seat2
[737] damage seat=1 source=Venser, Corpse Puppet amount=1 target=seat2
[738] life_change seat=1 source=Venser, Corpse Puppet amount=1 target=seat0
[739] phase_step seat=1 source= target=seat0
[740] state seat=1 source= target=seat0
```

</details>

#### Violation 21

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 30, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 30, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 787 events
  Seat 0 [alive]: life=40 library=82 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=43 library=83 hand=2 graveyard=5 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=13 library=84 hand=2 graveyard=3 exile=0 battlefield=10 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Biblioplex Tomekeeper (P/T 3/4, dmg=0)
    - Forbidding Spirit (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=4 graveyard=6 exile=0 battlefield=5 cmdzone=0 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Kenessos, Priest of Thassa (P/T 1/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[767] priority_pass seat=1 source= target=seat0
[768] stack_resolve seat=2 source=Biblioplex Tomekeeper target=seat0
[769] modification_effect seat=2 source=Biblioplex Tomekeeper target=seat0
[770] pay_mana seat=2 source=Forbidding Spirit amount=3 target=seat0
[771] cast seat=2 source=Forbidding Spirit amount=3 target=seat0
[772] stack_push seat=2 source=Forbidding Spirit target=seat0
[773] priority_pass seat=3 source= target=seat0
[774] priority_pass seat=0 source= target=seat0
[775] priority_pass seat=1 source= target=seat0
[776] stack_resolve seat=2 source=Forbidding Spirit target=seat0
[777] enter_battlefield seat=2 source=Forbidding Spirit target=seat0
[778] stack_push seat=2 source=Forbidding Spirit target=seat0
[779] priority_pass seat=3 source= target=seat0
[780] priority_pass seat=0 source= target=seat0
[781] priority_pass seat=1 source= target=seat0
[782] stack_resolve seat=2 source=Forbidding Spirit target=seat0
[783] modification_effect seat=2 source=Forbidding Spirit target=seat0
[784] phase_step seat=2 source= target=seat0
[785] phase_step seat=2 source= target=seat0
[786] state seat=2 source= target=seat0
```

</details>

#### Violation 22

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 30, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 30, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 787 events
  Seat 0 [alive]: life=40 library=82 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=43 library=83 hand=2 graveyard=5 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=13 library=84 hand=2 graveyard=3 exile=0 battlefield=10 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Biblioplex Tomekeeper (P/T 3/4, dmg=0)
    - Forbidding Spirit (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=4 graveyard=6 exile=0 battlefield=5 cmdzone=0 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Kenessos, Priest of Thassa (P/T 1/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[767] priority_pass seat=1 source= target=seat0
[768] stack_resolve seat=2 source=Biblioplex Tomekeeper target=seat0
[769] modification_effect seat=2 source=Biblioplex Tomekeeper target=seat0
[770] pay_mana seat=2 source=Forbidding Spirit amount=3 target=seat0
[771] cast seat=2 source=Forbidding Spirit amount=3 target=seat0
[772] stack_push seat=2 source=Forbidding Spirit target=seat0
[773] priority_pass seat=3 source= target=seat0
[774] priority_pass seat=0 source= target=seat0
[775] priority_pass seat=1 source= target=seat0
[776] stack_resolve seat=2 source=Forbidding Spirit target=seat0
[777] enter_battlefield seat=2 source=Forbidding Spirit target=seat0
[778] stack_push seat=2 source=Forbidding Spirit target=seat0
[779] priority_pass seat=3 source= target=seat0
[780] priority_pass seat=0 source= target=seat0
[781] priority_pass seat=1 source= target=seat0
[782] stack_resolve seat=2 source=Forbidding Spirit target=seat0
[783] modification_effect seat=2 source=Forbidding Spirit target=seat0
[784] phase_step seat=2 source= target=seat0
[785] phase_step seat=2 source= target=seat0
[786] state seat=2 source= target=seat0
```

</details>

#### Violation 23

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 31, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 822 events
  Seat 0 [alive]: life=40 library=82 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=43 library=83 hand=2 graveyard=5 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=13 library=84 hand=2 graveyard=4 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Biblioplex Tomekeeper (P/T 3/4, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emperor's Vanguard (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[802] priority_pass seat=1 source= target=seat0
[803] priority_pass seat=2 source= target=seat0
[804] stack_resolve seat=3 source=Emperor's Vanguard target=seat0
[805] enter_battlefield seat=3 source=Emperor's Vanguard target=seat0
[806] phase_step seat=3 source= target=seat0
[807] attackers seat=3 source= target=seat0
[808] blockers seat=2 source= target=seat0
[809] damage seat=3 source=Kenessos, Priest of Thassa amount=1 target=seat2
[810] damage seat=2 source=Forbidding Spirit amount=3 target=seat3
[811] destroy seat=2 source=Forbidding Spirit
[812] sba_704_5g seat=2 source=Forbidding Spirit
[813] zone_change seat=2 source=Forbidding Spirit
[814] destroy seat=3 source=Kenessos, Priest of Thassa
[815] sba_704_5g seat=3 source=Kenessos, Priest of Thassa
[816] zone_change seat=3 source=Kenessos, Priest of Thassa
[817] sba_704_6d seat=3 source=Kenessos, Priest of Thassa
[818] sba_cycle_complete seat=-1 source=
[819] phase_step seat=3 source= target=seat0
[820] pool_drain seat=3 source= amount=1 target=seat0
[821] state seat=3 source= target=seat0
```

</details>

#### Violation 24

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 31, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 822 events
  Seat 0 [alive]: life=40 library=82 hand=1 graveyard=7 exile=0 battlefield=6 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=43 library=83 hand=2 graveyard=5 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=13 library=84 hand=2 graveyard=4 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Biblioplex Tomekeeper (P/T 3/4, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emperor's Vanguard (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[802] priority_pass seat=1 source= target=seat0
[803] priority_pass seat=2 source= target=seat0
[804] stack_resolve seat=3 source=Emperor's Vanguard target=seat0
[805] enter_battlefield seat=3 source=Emperor's Vanguard target=seat0
[806] phase_step seat=3 source= target=seat0
[807] attackers seat=3 source= target=seat0
[808] blockers seat=2 source= target=seat0
[809] damage seat=3 source=Kenessos, Priest of Thassa amount=1 target=seat2
[810] damage seat=2 source=Forbidding Spirit amount=3 target=seat3
[811] destroy seat=2 source=Forbidding Spirit
[812] sba_704_5g seat=2 source=Forbidding Spirit
[813] zone_change seat=2 source=Forbidding Spirit
[814] destroy seat=3 source=Kenessos, Priest of Thassa
[815] sba_704_5g seat=3 source=Kenessos, Priest of Thassa
[816] zone_change seat=3 source=Kenessos, Priest of Thassa
[817] sba_704_6d seat=3 source=Kenessos, Priest of Thassa
[818] sba_cycle_complete seat=-1 source=
[819] phase_step seat=3 source= target=seat0
[820] pool_drain seat=3 source= amount=1 target=seat0
[821] state seat=3 source= target=seat0
```

</details>

#### Violation 25

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 848 events
  Seat 0 [alive]: life=40 library=81 hand=0 graveyard=8 exile=0 battlefield=7 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
    - Ovalchase Dragster (P/T 6/1, dmg=0)
  Seat 1 [alive]: life=43 library=83 hand=2 graveyard=5 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=13 library=84 hand=2 graveyard=4 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Biblioplex Tomekeeper (P/T 3/4, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emperor's Vanguard (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[828] draw seat=0 source=Racers' Ring // Racers' Ring amount=1 target=seat0
[829] pay_mana seat=0 source=Ovalchase Dragster amount=4 target=seat0
[830] cast seat=0 source=Ovalchase Dragster amount=4 target=seat0
[831] stack_push seat=0 source=Ovalchase Dragster target=seat0
[832] priority_pass seat=1 source= target=seat0
[833] priority_pass seat=2 source= target=seat0
[834] priority_pass seat=3 source= target=seat0
[835] stack_resolve seat=0 source=Ovalchase Dragster target=seat0
[836] enter_battlefield seat=0 source=Ovalchase Dragster target=seat0
[837] cast seat=0 source=Racers' Ring // Racers' Ring target=seat0
[838] stack_push seat=0 source=Racers' Ring // Racers' Ring target=seat0
[839] priority_pass seat=1 source= target=seat0
[840] priority_pass seat=2 source= target=seat0
[841] priority_pass seat=3 source= target=seat0
[842] stack_resolve seat=0 source=Racers' Ring // Racers' Ring target=seat0
[843] resolve seat=0 source=Racers' Ring // Racers' Ring target=seat0
[844] phase_step seat=0 source= target=seat0
[845] phase_step seat=0 source= target=seat0
[846] pool_drain seat=0 source= amount=1 target=seat0
[847] state seat=0 source= target=seat0
```

</details>

#### Violation 26

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 848 events
  Seat 0 [alive]: life=40 library=81 hand=0 graveyard=8 exile=0 battlefield=7 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
    - Ovalchase Dragster (P/T 6/1, dmg=0)
  Seat 1 [alive]: life=43 library=83 hand=2 graveyard=5 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Shadowmage Infiltrator (P/T 1/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=13 library=84 hand=2 graveyard=4 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Biblioplex Tomekeeper (P/T 3/4, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emperor's Vanguard (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[828] draw seat=0 source=Racers' Ring // Racers' Ring amount=1 target=seat0
[829] pay_mana seat=0 source=Ovalchase Dragster amount=4 target=seat0
[830] cast seat=0 source=Ovalchase Dragster amount=4 target=seat0
[831] stack_push seat=0 source=Ovalchase Dragster target=seat0
[832] priority_pass seat=1 source= target=seat0
[833] priority_pass seat=2 source= target=seat0
[834] priority_pass seat=3 source= target=seat0
[835] stack_resolve seat=0 source=Ovalchase Dragster target=seat0
[836] enter_battlefield seat=0 source=Ovalchase Dragster target=seat0
[837] cast seat=0 source=Racers' Ring // Racers' Ring target=seat0
[838] stack_push seat=0 source=Racers' Ring // Racers' Ring target=seat0
[839] priority_pass seat=1 source= target=seat0
[840] priority_pass seat=2 source= target=seat0
[841] priority_pass seat=3 source= target=seat0
[842] stack_resolve seat=0 source=Racers' Ring // Racers' Ring target=seat0
[843] resolve seat=0 source=Racers' Ring // Racers' Ring target=seat0
[844] phase_step seat=0 source= target=seat0
[845] phase_step seat=0 source= target=seat0
[846] pool_drain seat=0 source= amount=1 target=seat0
[847] state seat=0 source= target=seat0
```

</details>

#### Violation 27

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 890 events
  Seat 0 [alive]: life=40 library=81 hand=0 graveyard=8 exile=0 battlefield=7 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
    - Ovalchase Dragster (P/T 6/1, dmg=0)
  Seat 1 [alive]: life=44 library=82 hand=1 graveyard=6 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Alora, Cheerful Assassin (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=7 library=84 hand=2 graveyard=5 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emperor's Vanguard (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[870] stack_resolve seat=1 source=Alora, Cheerful Assassin target=seat0
[871] enter_battlefield seat=1 source=Alora, Cheerful Assassin target=seat0
[872] phase_step seat=1 source= target=seat0
[873] attackers seat=1 source= target=seat0
[874] blockers seat=2 source= target=seat0
[875] damage seat=1 source=Katsumasa, the Animator amount=3 target=seat2
[876] damage seat=1 source=Shadowmage Infiltrator amount=1 target=seat2
[877] damage seat=1 source=Harvest Hand // Scrounged Scythe amount=2 target=seat2
[878] damage seat=1 source=Venser, Corpse Puppet amount=1 target=seat2
[879] life_change seat=1 source=Venser, Corpse Puppet amount=1 target=seat0
[880] damage seat=2 source=Biblioplex Tomekeeper amount=3 target=seat1
[881] destroy seat=1 source=Shadowmage Infiltrator
[882] sba_704_5g seat=1 source=Shadowmage Infiltrator
[883] zone_change seat=1 source=Shadowmage Infiltrator
[884] destroy seat=2 source=Biblioplex Tomekeeper
[885] sba_704_5g seat=2 source=Biblioplex Tomekeeper
[886] zone_change seat=2 source=Biblioplex Tomekeeper
[887] sba_cycle_complete seat=-1 source=
[888] phase_step seat=1 source= target=seat0
[889] state seat=1 source= target=seat0
```

</details>

#### Violation 28

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 890 events
  Seat 0 [alive]: life=40 library=81 hand=0 graveyard=8 exile=0 battlefield=7 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
    - Ovalchase Dragster (P/T 6/1, dmg=0)
  Seat 1 [alive]: life=44 library=82 hand=1 graveyard=6 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Alora, Cheerful Assassin (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=7 library=84 hand=2 graveyard=5 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emperor's Vanguard (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[870] stack_resolve seat=1 source=Alora, Cheerful Assassin target=seat0
[871] enter_battlefield seat=1 source=Alora, Cheerful Assassin target=seat0
[872] phase_step seat=1 source= target=seat0
[873] attackers seat=1 source= target=seat0
[874] blockers seat=2 source= target=seat0
[875] damage seat=1 source=Katsumasa, the Animator amount=3 target=seat2
[876] damage seat=1 source=Shadowmage Infiltrator amount=1 target=seat2
[877] damage seat=1 source=Harvest Hand // Scrounged Scythe amount=2 target=seat2
[878] damage seat=1 source=Venser, Corpse Puppet amount=1 target=seat2
[879] life_change seat=1 source=Venser, Corpse Puppet amount=1 target=seat0
[880] damage seat=2 source=Biblioplex Tomekeeper amount=3 target=seat1
[881] destroy seat=1 source=Shadowmage Infiltrator
[882] sba_704_5g seat=1 source=Shadowmage Infiltrator
[883] zone_change seat=1 source=Shadowmage Infiltrator
[884] destroy seat=2 source=Biblioplex Tomekeeper
[885] sba_704_5g seat=2 source=Biblioplex Tomekeeper
[886] zone_change seat=2 source=Biblioplex Tomekeeper
[887] sba_cycle_complete seat=-1 source=
[888] phase_step seat=1 source= target=seat0
[889] state seat=1 source= target=seat0
```

</details>

#### Violation 29

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 34, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 933 events
  Seat 0 [alive]: life=40 library=81 hand=0 graveyard=8 exile=0 battlefield=7 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
    - Ovalchase Dragster (P/T 6/1, dmg=0)
  Seat 1 [alive]: life=44 library=82 hand=1 graveyard=6 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Alora, Cheerful Assassin (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=7 library=83 hand=0 graveyard=6 exile=0 battlefield=10 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Loyal Retainers (P/T 1/1, dmg=0)
    - Tomb Trawler (P/T 0/4, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emperor's Vanguard (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[913] pay_mana seat=2 source=Tomb Trawler amount=2 target=seat0
[914] cast seat=2 source=Tomb Trawler amount=2 target=seat0
[915] stack_push seat=2 source=Tomb Trawler target=seat0
[916] priority_pass seat=3 source= target=seat0
[917] priority_pass seat=0 source= target=seat0
[918] priority_pass seat=1 source= target=seat0
[919] stack_resolve seat=2 source=Tomb Trawler target=seat0
[920] modification_effect seat=2 source=Tomb Trawler target=seat0
[921] enter_battlefield seat=2 source=Tomb Trawler target=seat0
[922] cast seat=2 source=Azlask, the Swelling Scourge // Azlask, the Swelling Scourge target=seat0
[923] stack_push seat=2 source=Azlask, the Swelling Scourge // Azlask, the Swelling Scourge target=seat0
[924] priority_pass seat=3 source= target=seat0
[925] priority_pass seat=0 source= target=seat0
[926] priority_pass seat=1 source= target=seat0
[927] stack_resolve seat=2 source=Azlask, the Swelling Scourge // Azlask, the Swelling Scourge target=seat0
[928] resolve seat=2 source=Azlask, the Swelling Scourge // Azlask, the Swelling Scourge target=seat0
[929] phase_step seat=2 source= target=seat0
[930] phase_step seat=2 source= target=seat0
[931] pool_drain seat=2 source= amount=2 target=seat0
[932] state seat=2 source= target=seat0
```

</details>

#### Violation 30

- **Game**: 2201 (seed 22010043, perm 2)
- **Invariant**: SBACompleteness
- **Turn**: 34, Phase=ending Step=cleanup
- **Commanders**: ED-E, Lonesome Eyebot, Venser, Corpse Puppet, Elesh Norn // The Argent Etchings, Kenessos, Priest of Thassa
- **Message**: seat 0 has creature "Clockwork Vorrac" on battlefield with toughness=0 (layer=1) — SBA 704.5f missed (base=0/0, counters=map[], mods=<none>)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 933 events
  Seat 0 [alive]: life=40 library=81 hand=0 graveyard=8 exile=0 battlefield=7 cmdzone=1 mana=0
    - Quicksand (P/T 0/0, dmg=0) [T]
    - Planar Nexus (P/T 0/0, dmg=0) [T]
    - Naya Panorama (P/T 0/0, dmg=0) [T]
    - Voldaren Estate (P/T 0/0, dmg=0) [T]
    - Esper Panorama (P/T 0/0, dmg=0) [T]
    - Clockwork Vorrac (P/T 0/0, dmg=0)
    - Ovalchase Dragster (P/T 6/1, dmg=0)
  Seat 1 [alive]: life=44 library=82 hand=1 graveyard=6 exile=0 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Katsumasa, the Animator (P/T 3/3, dmg=0) [T]
    - Harvest Hand // Scrounged Scythe (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Venser, Corpse Puppet (P/T 1/3, dmg=0) [T]
    - Marketback Walker (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Alora, Cheerful Assassin (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=7 library=83 hand=0 graveyard=6 exile=0 battlefield=10 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Humility (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Detection Tower (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Loyal Retainers (P/T 1/1, dmg=0)
    - Tomb Trawler (P/T 0/4, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=3 graveyard=6 exile=0 battlefield=6 cmdzone=1 mana=0
    - Thespian's Stage (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Emperor's Vanguard (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[913] pay_mana seat=2 source=Tomb Trawler amount=2 target=seat0
[914] cast seat=2 source=Tomb Trawler amount=2 target=seat0
[915] stack_push seat=2 source=Tomb Trawler target=seat0
[916] priority_pass seat=3 source= target=seat0
[917] priority_pass seat=0 source= target=seat0
[918] priority_pass seat=1 source= target=seat0
[919] stack_resolve seat=2 source=Tomb Trawler target=seat0
[920] modification_effect seat=2 source=Tomb Trawler target=seat0
[921] enter_battlefield seat=2 source=Tomb Trawler target=seat0
[922] cast seat=2 source=Azlask, the Swelling Scourge // Azlask, the Swelling Scourge target=seat0
[923] stack_push seat=2 source=Azlask, the Swelling Scourge // Azlask, the Swelling Scourge target=seat0
[924] priority_pass seat=3 source= target=seat0
[925] priority_pass seat=0 source= target=seat0
[926] priority_pass seat=1 source= target=seat0
[927] stack_resolve seat=2 source=Azlask, the Swelling Scourge // Azlask, the Swelling Scourge target=seat0
[928] resolve seat=2 source=Azlask, the Swelling Scourge // Azlask, the Swelling Scourge target=seat0
[929] phase_step seat=2 source= target=seat0
[930] phase_step seat=2 source= target=seat0
[931] pool_drain seat=2 source= amount=2 target=seat0
[932] state seat=2 source= target=seat0
```

</details>

*... and 2402 more violations not shown.*

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Humility | 175 | 5705 | 0.03 |
| 2 | Genju of the Realm | 4 | 236 | 0.02 |
| 3 | Naked Singularity | 2 | 168 | 0.01 |
| 4 | Sphinx of the Steel Wind | 2 | 278 | 0.01 |
| 5 | Mythos of Snapdax | 2 | 338 | 0.01 |
| 6 | Naya Battlemage | 3 | 567 | 0.01 |
| 7 | Villainous Wealth | 1 | 229 | 0.00 |
| 8 | Necra Disciple | 1 | 259 | 0.00 |
| 9 | Ivorytusk Fortress | 1 | 269 | 0.00 |
| 10 | Scarland Thrinax | 1 | 269 | 0.00 |

## Verdict: ISSUES FOUND

**2432 total issues** across 1000000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 2432 invariant violations in chaos games
- 0 crashes in nightmare boards
- 0 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
