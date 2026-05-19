# Chaos Gauntlet Report

Generated: 2026-05-19T08:18:11-07:00

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
| Duration | 3m54.593s |
| Throughput | 21 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 1652 (in 57 games) |
| Clean Games | 4943 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 3.479s |
| Throughput | 2875 boards/sec |
| Crashes | 0 |
| Invariant Violations | 6 |
| Clean Boards | 9997 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| CardIdentity | 832 |
| AttachmentConsistency | 14 |
| ZoneConservation | 790 |
| TriggerCompleteness | 8 |
| ZoneCastGrantExpiry | 8 |

### Violation Details (first 30)

#### Violation 1

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 14, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 14, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 523 events
  Seat 0 [alive]: life=38 library=87 hand=4 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=2 graveyard=8 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Nick Valentine, Private Eye (P/T 2/2, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Cerulean Sphinx (P/T 5/5, dmg=0)
  Seat 2 [alive]: life=36 library=89 hand=6 graveyard=2 exile=0 battlefield=3 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=7 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[503] priority_pass seat=2 source= target=seat0
[504] priority_pass seat=3 source= target=seat0
[505] priority_pass seat=0 source= target=seat0
[506] stack_resolve seat=1 source=Cerulean Sphinx target=seat0
[507] shuffle_into_library seat=1 source=Cerulean Sphinx target=seat0
[508] enter_battlefield seat=1 source=Cerulean Sphinx target=seat0
[509] trigger_evaluated seat=1 source=Genesis Chamber
[510] stack_push seat=1 source=Genesis Chamber target=seat0
[511] triggered_ability seat=1 source=Genesis Chamber target=seat0
[512] priority_pass seat=2 source= target=seat0
[513] priority_pass seat=3 source= target=seat0
[514] priority_pass seat=0 source= target=seat0
[515] stack_resolve seat=1 source=Genesis Chamber target=seat0
[516] phase_step seat=1 source= target=seat0
[517] declare_attackers seat=1 source= target=seat0
[518] blockers seat=2 source= target=seat0
[519] damage seat=1 source=Nick Valentine, Private Eye amount=2 target=seat2
[520] speed_advance seat=1 source= amount=1 target=seat0
[521] phase_step seat=1 source= target=seat0
[522] state seat=1 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 14, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 14, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 523 events
  Seat 0 [alive]: life=38 library=87 hand=4 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=2 graveyard=8 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Nick Valentine, Private Eye (P/T 2/2, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Cerulean Sphinx (P/T 5/5, dmg=0)
  Seat 2 [alive]: life=36 library=89 hand=6 graveyard=2 exile=0 battlefield=3 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=7 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[503] priority_pass seat=2 source= target=seat0
[504] priority_pass seat=3 source= target=seat0
[505] priority_pass seat=0 source= target=seat0
[506] stack_resolve seat=1 source=Cerulean Sphinx target=seat0
[507] shuffle_into_library seat=1 source=Cerulean Sphinx target=seat0
[508] enter_battlefield seat=1 source=Cerulean Sphinx target=seat0
[509] trigger_evaluated seat=1 source=Genesis Chamber
[510] stack_push seat=1 source=Genesis Chamber target=seat0
[511] triggered_ability seat=1 source=Genesis Chamber target=seat0
[512] priority_pass seat=2 source= target=seat0
[513] priority_pass seat=3 source= target=seat0
[514] priority_pass seat=0 source= target=seat0
[515] stack_resolve seat=1 source=Genesis Chamber target=seat0
[516] phase_step seat=1 source= target=seat0
[517] declare_attackers seat=1 source= target=seat0
[518] blockers seat=2 source= target=seat0
[519] damage seat=1 source=Nick Valentine, Private Eye amount=2 target=seat2
[520] speed_advance seat=1 source= amount=1 target=seat0
[521] phase_step seat=1 source= target=seat0
[522] state seat=1 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 15, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 15, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 561 events
  Seat 0 [alive]: life=38 library=87 hand=4 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=2 graveyard=8 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Nick Valentine, Private Eye (P/T 2/2, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Cerulean Sphinx (P/T 5/5, dmg=0)
  Seat 2 [alive]: life=36 library=88 hand=5 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=85 hand=7 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[541] triggered_ability seat=1 source=Genesis Chamber target=seat0
[542] priority_pass seat=2 source= target=seat0
[543] priority_pass seat=3 source= target=seat0
[544] priority_pass seat=0 source= target=seat0
[545] stack_resolve seat=1 source=Genesis Chamber target=seat0
[546] add_mana seat=2 source=Ancient Tomb amount=1 target=seat0
[547] phase_step seat=2 source= target=seat0
[548] declare_attackers seat=2 source= target=seat0
[549] blockers seat=1 source= target=seat0
[550] damage seat=2 source=Moku, Meandering Drummer amount=2 target=seat1
[551] damage seat=1 source=Cerulean Sphinx amount=5 target=seat2
[552] destroy seat=2 source=Moku, Meandering Drummer
[553] sba_704_5g seat=2 source=Moku, Meandering Drummer
[554] zone_change seat=2 source=Moku, Meandering Drummer
[555] sba_704_6d seat=2 source=Moku, Meandering Drummer
[556] sba_cycle_complete seat=-1 source=
[557] phase_step seat=2 source= target=seat0
[558] pool_drain seat=2 source= amount=1 target=seat0
[559] damage_wears_off seat=1 source=Cerulean Sphinx amount=2 target=seat0
[560] state seat=2 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 15, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 15, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 561 events
  Seat 0 [alive]: life=38 library=87 hand=4 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=2 graveyard=8 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Nick Valentine, Private Eye (P/T 2/2, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Cerulean Sphinx (P/T 5/5, dmg=0)
  Seat 2 [alive]: life=36 library=88 hand=5 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=85 hand=7 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[541] triggered_ability seat=1 source=Genesis Chamber target=seat0
[542] priority_pass seat=2 source= target=seat0
[543] priority_pass seat=3 source= target=seat0
[544] priority_pass seat=0 source= target=seat0
[545] stack_resolve seat=1 source=Genesis Chamber target=seat0
[546] add_mana seat=2 source=Ancient Tomb amount=1 target=seat0
[547] phase_step seat=2 source= target=seat0
[548] declare_attackers seat=2 source= target=seat0
[549] blockers seat=1 source= target=seat0
[550] damage seat=2 source=Moku, Meandering Drummer amount=2 target=seat1
[551] damage seat=1 source=Cerulean Sphinx amount=5 target=seat2
[552] destroy seat=2 source=Moku, Meandering Drummer
[553] sba_704_5g seat=2 source=Moku, Meandering Drummer
[554] zone_change seat=2 source=Moku, Meandering Drummer
[555] sba_704_6d seat=2 source=Moku, Meandering Drummer
[556] sba_cycle_complete seat=-1 source=
[557] phase_step seat=2 source= target=seat0
[558] pool_drain seat=2 source= amount=1 target=seat0
[559] damage_wears_off seat=1 source=Cerulean Sphinx amount=2 target=seat0
[560] state seat=2 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 16, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 16, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 601 events
  Seat 0 [alive]: life=38 library=87 hand=4 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=2 graveyard=8 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Nick Valentine, Private Eye (P/T 2/2, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Cerulean Sphinx (P/T 5/5, dmg=0)
  Seat 2 [alive]: life=36 library=88 hand=5 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=84 hand=7 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[581] commander_cast_from_command_zone seat=3 source=The Master, Gallifrey's End amount=4 target=seat0
[582] stack_push seat=3 source=The Master, Gallifrey's End target=seat0
[583] phase_step seat=3 source= target=seat0
[584] priority_pass seat=0 source= target=seat0
[585] priority_pass seat=1 source= target=seat0
[586] priority_pass seat=2 source= target=seat0
[587] phase_step seat=3 source= target=seat0
[588] priority_pass seat=0 source= target=seat0
[589] priority_pass seat=1 source= target=seat0
[590] priority_pass seat=2 source= target=seat0
[591] stack_resolve seat=3 source=The Master, Gallifrey's End target=seat0
[592] enter_battlefield seat=3 source=The Master, Gallifrey's End target=seat0
[593] trigger_evaluated seat=1 source=Genesis Chamber
[594] stack_push seat=1 source=Genesis Chamber target=seat0
[595] triggered_ability seat=1 source=Genesis Chamber target=seat0
[596] priority_pass seat=3 source= target=seat0
[597] priority_pass seat=0 source= target=seat0
[598] priority_pass seat=2 source= target=seat0
[599] stack_resolve seat=1 source=Genesis Chamber target=seat0
[600] state seat=3 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 16, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 16, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 601 events
  Seat 0 [alive]: life=38 library=87 hand=4 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=2 graveyard=8 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Nick Valentine, Private Eye (P/T 2/2, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Cerulean Sphinx (P/T 5/5, dmg=0)
  Seat 2 [alive]: life=36 library=88 hand=5 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=84 hand=7 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[581] commander_cast_from_command_zone seat=3 source=The Master, Gallifrey's End amount=4 target=seat0
[582] stack_push seat=3 source=The Master, Gallifrey's End target=seat0
[583] phase_step seat=3 source= target=seat0
[584] priority_pass seat=0 source= target=seat0
[585] priority_pass seat=1 source= target=seat0
[586] priority_pass seat=2 source= target=seat0
[587] phase_step seat=3 source= target=seat0
[588] priority_pass seat=0 source= target=seat0
[589] priority_pass seat=1 source= target=seat0
[590] priority_pass seat=2 source= target=seat0
[591] stack_resolve seat=3 source=The Master, Gallifrey's End target=seat0
[592] enter_battlefield seat=3 source=The Master, Gallifrey's End target=seat0
[593] trigger_evaluated seat=1 source=Genesis Chamber
[594] stack_push seat=1 source=Genesis Chamber target=seat0
[595] triggered_ability seat=1 source=Genesis Chamber target=seat0
[596] priority_pass seat=3 source= target=seat0
[597] priority_pass seat=0 source= target=seat0
[598] priority_pass seat=2 source= target=seat0
[599] stack_resolve seat=1 source=Genesis Chamber target=seat0
[600] state seat=3 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 17, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 17, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 675 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plague Dogs (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=2 graveyard=8 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Nick Valentine, Private Eye (P/T 2/2, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Cerulean Sphinx (P/T 5/5, dmg=0)
  Seat 2 [alive]: life=36 library=88 hand=5 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=84 hand=7 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[655] priority_pass seat=0 source= target=seat0
[656] priority_pass seat=2 source= target=seat0
[657] priority_pass seat=3 source= target=seat0
[658] stack_resolve seat=1 source=Genesis Chamber target=seat0
[659] tap seat=0 source=Lobelia, Defender of Bag End target=seat0
[660] sacrifice seat=0 source=Soultether Golem target=seat0
[661] zone_change seat=0 source=Soultether Golem
[662] activate_ability seat=0 source=Lobelia, Defender of Bag End target=seat0
[663] stack_push seat=0 source=Lobelia, Defender of Bag End target=seat0
[664] priority_pass seat=1 source= target=seat0
[665] priority_pass seat=2 source= target=seat0
[666] priority_pass seat=3 source= target=seat0
[667] stack_resolve seat=0 source=Lobelia, Defender of Bag End target=seat0
[668] until_duration_effect seat=0 source=Lobelia, Defender of Bag End target=seat0
[669] gain_life seat=0 source=Lobelia, Defender of Bag End amount=2 target=seat0
[670] activated_ability_resolved seat=0 source=Lobelia, Defender of Bag End target=seat0
[671] phase_step seat=0 source= target=seat0
[672] phase_step seat=0 source= target=seat0
[673] pool_drain seat=0 source= amount=1 target=seat0
[674] state seat=0 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 17, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 17, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 675 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plague Dogs (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=2 graveyard=8 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Nick Valentine, Private Eye (P/T 2/2, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Cerulean Sphinx (P/T 5/5, dmg=0)
  Seat 2 [alive]: life=36 library=88 hand=5 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=40 library=84 hand=7 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[655] priority_pass seat=0 source= target=seat0
[656] priority_pass seat=2 source= target=seat0
[657] priority_pass seat=3 source= target=seat0
[658] stack_resolve seat=1 source=Genesis Chamber target=seat0
[659] tap seat=0 source=Lobelia, Defender of Bag End target=seat0
[660] sacrifice seat=0 source=Soultether Golem target=seat0
[661] zone_change seat=0 source=Soultether Golem
[662] activate_ability seat=0 source=Lobelia, Defender of Bag End target=seat0
[663] stack_push seat=0 source=Lobelia, Defender of Bag End target=seat0
[664] priority_pass seat=1 source= target=seat0
[665] priority_pass seat=2 source= target=seat0
[666] priority_pass seat=3 source= target=seat0
[667] stack_resolve seat=0 source=Lobelia, Defender of Bag End target=seat0
[668] until_duration_effect seat=0 source=Lobelia, Defender of Bag End target=seat0
[669] gain_life seat=0 source=Lobelia, Defender of Bag End amount=2 target=seat0
[670] activated_ability_resolved seat=0 source=Lobelia, Defender of Bag End target=seat0
[671] phase_step seat=0 source= target=seat0
[672] phase_step seat=0 source= target=seat0
[673] pool_drain seat=0 source= amount=1 target=seat0
[674] state seat=0 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 18, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 18, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 711 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plague Dogs (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=3 graveyard=8 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=88 hand=5 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=84 hand=7 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[691] stack_resolve seat=1 source=Cerulean Sphinx target=seat0
[692] shuffle_into_library seat=1 source=Cerulean Sphinx target=seat0
[693] activated_ability_resolved seat=1 source=Cerulean Sphinx target=seat0
[694] draw seat=1 source=Jadzi, Steward of Fate // Oracle's Gift amount=1 target=seat0
[695] phase_step seat=1 source= target=seat0
[696] declare_attackers seat=1 source= target=seat0
[697] blockers seat=3 source= target=seat0
[698] damage seat=1 source=Nick Valentine, Private Eye amount=2 target=seat3
[699] damage seat=1 source=Fear of Impostors amount=3 target=seat3
[700] speed_advance seat=1 source= amount=2 target=seat0
[701] damage seat=3 source=The Master, Gallifrey's End amount=4 target=seat1
[702] destroy seat=1 source=Nick Valentine, Private Eye
[703] sba_704_5g seat=1 source=Nick Valentine, Private Eye
[704] zone_change seat=1 source=Nick Valentine, Private Eye
[705] sba_704_6d seat=1 source=Nick Valentine, Private Eye
[706] sba_cycle_complete seat=-1 source=
[707] phase_step seat=1 source= target=seat0
[708] pool_drain seat=1 source= amount=3 target=seat0
[709] damage_wears_off seat=3 source=The Master, Gallifrey's End amount=2 target=seat0
[710] state seat=1 source= target=seat0
```

</details>

#### Violation 10

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 18, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 18, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 711 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plague Dogs (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=3 graveyard=8 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=88 hand=5 graveyard=3 exile=0 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=84 hand=7 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[691] stack_resolve seat=1 source=Cerulean Sphinx target=seat0
[692] shuffle_into_library seat=1 source=Cerulean Sphinx target=seat0
[693] activated_ability_resolved seat=1 source=Cerulean Sphinx target=seat0
[694] draw seat=1 source=Jadzi, Steward of Fate // Oracle's Gift amount=1 target=seat0
[695] phase_step seat=1 source= target=seat0
[696] declare_attackers seat=1 source= target=seat0
[697] blockers seat=3 source= target=seat0
[698] damage seat=1 source=Nick Valentine, Private Eye amount=2 target=seat3
[699] damage seat=1 source=Fear of Impostors amount=3 target=seat3
[700] speed_advance seat=1 source= amount=2 target=seat0
[701] damage seat=3 source=The Master, Gallifrey's End amount=4 target=seat1
[702] destroy seat=1 source=Nick Valentine, Private Eye
[703] sba_704_5g seat=1 source=Nick Valentine, Private Eye
[704] zone_change seat=1 source=Nick Valentine, Private Eye
[705] sba_704_6d seat=1 source=Nick Valentine, Private Eye
[706] sba_cycle_complete seat=-1 source=
[707] phase_step seat=1 source= target=seat0
[708] pool_drain seat=1 source= amount=3 target=seat0
[709] damage_wears_off seat=3 source=The Master, Gallifrey's End amount=2 target=seat0
[710] state seat=1 source= target=seat0
```

</details>

#### Violation 11

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 19, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 19, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 751 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plague Dogs (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=3 graveyard=8 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=87 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=37 library=84 hand=7 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[731] commander_cast_from_command_zone seat=2 source=Moku, Meandering Drummer amount=4 target=seat0
[732] stack_push seat=2 source=Moku, Meandering Drummer target=seat0
[733] phase_step seat=2 source= target=seat0
[734] priority_pass seat=3 source= target=seat0
[735] priority_pass seat=0 source= target=seat0
[736] priority_pass seat=1 source= target=seat0
[737] phase_step seat=2 source= target=seat0
[738] priority_pass seat=3 source= target=seat0
[739] priority_pass seat=0 source= target=seat0
[740] priority_pass seat=1 source= target=seat0
[741] stack_resolve seat=2 source=Moku, Meandering Drummer target=seat0
[742] enter_battlefield seat=2 source=Moku, Meandering Drummer target=seat0
[743] trigger_evaluated seat=1 source=Genesis Chamber
[744] stack_push seat=1 source=Genesis Chamber target=seat0
[745] triggered_ability seat=1 source=Genesis Chamber target=seat0
[746] priority_pass seat=2 source= target=seat0
[747] priority_pass seat=3 source= target=seat0
[748] priority_pass seat=0 source= target=seat0
[749] stack_resolve seat=1 source=Genesis Chamber target=seat0
[750] state seat=2 source= target=seat0
```

</details>

#### Violation 12

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 19, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 19, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 751 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plague Dogs (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=3 graveyard=8 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=87 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=37 library=84 hand=7 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[731] commander_cast_from_command_zone seat=2 source=Moku, Meandering Drummer amount=4 target=seat0
[732] stack_push seat=2 source=Moku, Meandering Drummer target=seat0
[733] phase_step seat=2 source= target=seat0
[734] priority_pass seat=3 source= target=seat0
[735] priority_pass seat=0 source= target=seat0
[736] priority_pass seat=1 source= target=seat0
[737] phase_step seat=2 source= target=seat0
[738] priority_pass seat=3 source= target=seat0
[739] priority_pass seat=0 source= target=seat0
[740] priority_pass seat=1 source= target=seat0
[741] stack_resolve seat=2 source=Moku, Meandering Drummer target=seat0
[742] enter_battlefield seat=2 source=Moku, Meandering Drummer target=seat0
[743] trigger_evaluated seat=1 source=Genesis Chamber
[744] stack_push seat=1 source=Genesis Chamber target=seat0
[745] triggered_ability seat=1 source=Genesis Chamber target=seat0
[746] priority_pass seat=2 source= target=seat0
[747] priority_pass seat=3 source= target=seat0
[748] priority_pass seat=0 source= target=seat0
[749] stack_resolve seat=1 source=Genesis Chamber target=seat0
[750] state seat=2 source= target=seat0
```

</details>

#### Violation 13

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 20, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 20, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 816 events
  Seat 0 [alive]: life=36 library=85 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plague Dogs (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=3 graveyard=8 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=87 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=37 library=82 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[796] stack_push seat=1 source=Genesis Chamber target=seat0
[797] triggered_ability seat=1 source=Genesis Chamber target=seat0
[798] priority_pass seat=3 source= target=seat0
[799] priority_pass seat=0 source= target=seat0
[800] priority_pass seat=2 source= target=seat0
[801] stack_resolve seat=1 source=Genesis Chamber target=seat0
[802] phase_step seat=3 source= target=seat0
[803] declare_attackers seat=3 source= target=seat0
[804] blockers seat=0 source= target=seat0
[805] damage seat=3 source=The Master, Gallifrey's End amount=4 target=seat0
[806] speed_advance seat=3 source= amount=4 target=seat0
[807] damage seat=3 source=Reinforced Ronin amount=2 target=seat0
[808] damage seat=0 source=Plague Dogs amount=3 target=seat3
[809] destroy seat=3 source=Reinforced Ronin
[810] sba_704_5g seat=3 source=Reinforced Ronin
[811] zone_change seat=3 source=Reinforced Ronin
[812] sba_cycle_complete seat=-1 source=
[813] phase_step seat=3 source= target=seat0
[814] damage_wears_off seat=0 source=Plague Dogs amount=2 target=seat0
[815] state seat=3 source= target=seat0
```

</details>

#### Violation 14

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 20, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 20, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 816 events
  Seat 0 [alive]: life=36 library=85 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plague Dogs (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=38 library=82 hand=3 graveyard=8 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=87 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=37 library=82 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[796] stack_push seat=1 source=Genesis Chamber target=seat0
[797] triggered_ability seat=1 source=Genesis Chamber target=seat0
[798] priority_pass seat=3 source= target=seat0
[799] priority_pass seat=0 source= target=seat0
[800] priority_pass seat=2 source= target=seat0
[801] stack_resolve seat=1 source=Genesis Chamber target=seat0
[802] phase_step seat=3 source= target=seat0
[803] declare_attackers seat=3 source= target=seat0
[804] blockers seat=0 source= target=seat0
[805] damage seat=3 source=The Master, Gallifrey's End amount=4 target=seat0
[806] speed_advance seat=3 source= amount=4 target=seat0
[807] damage seat=3 source=Reinforced Ronin amount=2 target=seat0
[808] damage seat=0 source=Plague Dogs amount=3 target=seat3
[809] destroy seat=3 source=Reinforced Ronin
[810] sba_704_5g seat=3 source=Reinforced Ronin
[811] zone_change seat=3 source=Reinforced Ronin
[812] sba_cycle_complete seat=-1 source=
[813] phase_step seat=3 source= target=seat0
[814] damage_wears_off seat=0 source=Plague Dogs amount=2 target=seat0
[815] state seat=3 source= target=seat0
```

</details>

#### Violation 15

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 21, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 21, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 871 events
  Seat 0 [alive]: life=36 library=83 hand=4 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=82 hand=3 graveyard=8 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=87 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=37 library=82 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[851] triggered_ability seat=1 source=Genesis Chamber target=seat0
[852] priority_pass seat=0 source= target=seat0
[853] priority_pass seat=2 source= target=seat0
[854] priority_pass seat=3 source= target=seat0
[855] stack_resolve seat=1 source=Genesis Chamber target=seat0
[856] add_mana seat=0 source=Swamp amount=1 target=seat0
[857] phase_step seat=0 source= target=seat0
[858] declare_attackers seat=0 source= target=seat0
[859] blockers seat=3 source= target=seat0
[860] damage seat=0 source=Lobelia, Defender of Bag End amount=2 target=seat3
[861] damage seat=3 source=Coalition Warbrute amount=3 target=seat0
[862] destroy seat=0 source=Lobelia, Defender of Bag End
[863] sba_704_5g seat=0 source=Lobelia, Defender of Bag End
[864] zone_change seat=0 source=Lobelia, Defender of Bag End
[865] sba_704_6d seat=0 source=Lobelia, Defender of Bag End
[866] sba_cycle_complete seat=-1 source=
[867] phase_step seat=0 source= target=seat0
[868] pool_drain seat=0 source= amount=4 target=seat0
[869] damage_wears_off seat=3 source=Coalition Warbrute amount=2 target=seat0
[870] state seat=0 source= target=seat0
```

</details>

#### Violation 16

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 21, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 21, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 871 events
  Seat 0 [alive]: life=36 library=83 hand=4 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=82 hand=3 graveyard=8 exile=0 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Fear of Impostors (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=87 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=37 library=82 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[851] triggered_ability seat=1 source=Genesis Chamber target=seat0
[852] priority_pass seat=0 source= target=seat0
[853] priority_pass seat=2 source= target=seat0
[854] priority_pass seat=3 source= target=seat0
[855] stack_resolve seat=1 source=Genesis Chamber target=seat0
[856] add_mana seat=0 source=Swamp amount=1 target=seat0
[857] phase_step seat=0 source= target=seat0
[858] declare_attackers seat=0 source= target=seat0
[859] blockers seat=3 source= target=seat0
[860] damage seat=0 source=Lobelia, Defender of Bag End amount=2 target=seat3
[861] damage seat=3 source=Coalition Warbrute amount=3 target=seat0
[862] destroy seat=0 source=Lobelia, Defender of Bag End
[863] sba_704_5g seat=0 source=Lobelia, Defender of Bag End
[864] zone_change seat=0 source=Lobelia, Defender of Bag End
[865] sba_704_6d seat=0 source=Lobelia, Defender of Bag End
[866] sba_cycle_complete seat=-1 source=
[867] phase_step seat=0 source= target=seat0
[868] pool_drain seat=0 source= amount=4 target=seat0
[869] damage_wears_off seat=3 source=Coalition Warbrute amount=2 target=seat0
[870] state seat=0 source= target=seat0
```

</details>

#### Violation 17

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 22, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 895 events
  Seat 0 [alive]: life=36 library=83 hand=4 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=81 hand=4 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=87 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=37 library=82 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[875] untap_done seat=1 source=Fear of Impostors target=seat0
[876] untap_done seat=1 source=Island target=seat0
[877] add_mana seat=1 source=Island amount=1 target=seat0
[878] add_mana seat=1 source=Island amount=1 target=seat0
[879] add_mana seat=1 source=Island amount=1 target=seat0
[880] add_mana seat=1 source=Island amount=1 target=seat0
[881] draw seat=1 source=Phyrexian Processor amount=1 target=seat0
[882] phase_step seat=1 source= target=seat0
[883] declare_attackers seat=1 source= target=seat0
[884] blockers seat=3 source= target=seat0
[885] damage seat=1 source=Fear of Impostors amount=3 target=seat3
[886] damage seat=3 source=Coalition Warbrute amount=3 target=seat1
[887] destroy seat=1 source=Fear of Impostors
[888] sba_704_5g seat=1 source=Fear of Impostors
[889] zone_change seat=1 source=Fear of Impostors
[890] sba_cycle_complete seat=-1 source=
[891] phase_step seat=1 source= target=seat0
[892] pool_drain seat=1 source= amount=4 target=seat0
[893] damage_wears_off seat=3 source=Coalition Warbrute amount=3 target=seat0
[894] state seat=1 source= target=seat0
```

</details>

#### Violation 18

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 22, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 895 events
  Seat 0 [alive]: life=36 library=83 hand=4 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=81 hand=4 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=87 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Moku, Meandering Drummer (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=37 library=82 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[875] untap_done seat=1 source=Fear of Impostors target=seat0
[876] untap_done seat=1 source=Island target=seat0
[877] add_mana seat=1 source=Island amount=1 target=seat0
[878] add_mana seat=1 source=Island amount=1 target=seat0
[879] add_mana seat=1 source=Island amount=1 target=seat0
[880] add_mana seat=1 source=Island amount=1 target=seat0
[881] draw seat=1 source=Phyrexian Processor amount=1 target=seat0
[882] phase_step seat=1 source= target=seat0
[883] declare_attackers seat=1 source= target=seat0
[884] blockers seat=3 source= target=seat0
[885] damage seat=1 source=Fear of Impostors amount=3 target=seat3
[886] damage seat=3 source=Coalition Warbrute amount=3 target=seat1
[887] destroy seat=1 source=Fear of Impostors
[888] sba_704_5g seat=1 source=Fear of Impostors
[889] zone_change seat=1 source=Fear of Impostors
[890] sba_cycle_complete seat=-1 source=
[891] phase_step seat=1 source= target=seat0
[892] pool_drain seat=1 source= amount=4 target=seat0
[893] damage_wears_off seat=3 source=Coalition Warbrute amount=3 target=seat0
[894] state seat=1 source= target=seat0
```

</details>

#### Violation 19

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 23, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 928 events
  Seat 0 [alive]: life=36 library=83 hand=4 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=81 hand=4 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=86 hand=5 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=82 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[908] priority_pass seat=3 source= target=seat0
[909] priority_pass seat=0 source= target=seat0
[910] priority_pass seat=1 source= target=seat0
[911] stack_resolve seat=2 source=Scrap target=seat0
[912] zone_change seat=2 source=Scrap
[913] resolve seat=2 source=Scrap target=seat0
[914] phase_step seat=2 source= target=seat0
[915] declare_attackers seat=2 source= target=seat0
[916] blockers seat=3 source= target=seat0
[917] damage seat=2 source=Moku, Meandering Drummer amount=2 target=seat3
[918] damage seat=3 source=Coalition Warbrute amount=3 target=seat2
[919] destroy seat=2 source=Moku, Meandering Drummer
[920] sba_704_5g seat=2 source=Moku, Meandering Drummer
[921] zone_change seat=2 source=Moku, Meandering Drummer
[922] sba_704_6d seat=2 source=Moku, Meandering Drummer
[923] sba_cycle_complete seat=-1 source=
[924] phase_step seat=2 source= target=seat0
[925] pool_drain seat=2 source= amount=1 target=seat0
[926] damage_wears_off seat=3 source=Coalition Warbrute amount=2 target=seat0
[927] state seat=2 source= target=seat0
```

</details>

#### Violation 20

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 23, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 928 events
  Seat 0 [alive]: life=36 library=83 hand=4 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=38 library=81 hand=4 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=86 hand=5 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=82 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[908] priority_pass seat=3 source= target=seat0
[909] priority_pass seat=0 source= target=seat0
[910] priority_pass seat=1 source= target=seat0
[911] stack_resolve seat=2 source=Scrap target=seat0
[912] zone_change seat=2 source=Scrap
[913] resolve seat=2 source=Scrap target=seat0
[914] phase_step seat=2 source= target=seat0
[915] declare_attackers seat=2 source= target=seat0
[916] blockers seat=3 source= target=seat0
[917] damage seat=2 source=Moku, Meandering Drummer amount=2 target=seat3
[918] damage seat=3 source=Coalition Warbrute amount=3 target=seat2
[919] destroy seat=2 source=Moku, Meandering Drummer
[920] sba_704_5g seat=2 source=Moku, Meandering Drummer
[921] zone_change seat=2 source=Moku, Meandering Drummer
[922] sba_704_6d seat=2 source=Moku, Meandering Drummer
[923] sba_cycle_complete seat=-1 source=
[924] phase_step seat=2 source= target=seat0
[925] pool_drain seat=2 source= amount=1 target=seat0
[926] damage_wears_off seat=3 source=Coalition Warbrute amount=2 target=seat0
[927] state seat=2 source= target=seat0
```

</details>

#### Violation 21

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 24, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 24, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 973 events
  Seat 0 [alive]: life=36 library=83 hand=4 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=31 library=81 hand=4 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=86 hand=5 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=81 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[953] priority_pass seat=0 source= target=seat0
[954] priority_pass seat=1 source= target=seat0
[955] priority_pass seat=2 source= target=seat0
[956] stack_resolve seat=3 source=Street Riot target=seat0
[957] enter_battlefield seat=3 source=Street Riot target=seat0
[958] trigger_evaluated seat=1 source=Genesis Chamber
[959] stack_push seat=1 source=Genesis Chamber target=seat0
[960] triggered_ability seat=1 source=Genesis Chamber target=seat0
[961] priority_pass seat=3 source= target=seat0
[962] priority_pass seat=0 source= target=seat0
[963] priority_pass seat=2 source= target=seat0
[964] stack_resolve seat=1 source=Genesis Chamber target=seat0
[965] phase_step seat=3 source= target=seat0
[966] declare_attackers seat=3 source= target=seat0
[967] blockers seat=1 source= target=seat0
[968] damage seat=3 source=The Master, Gallifrey's End amount=4 target=seat1
[969] damage seat=3 source=Coalition Warbrute amount=3 target=seat1
[970] phase_step seat=3 source= target=seat0
[971] pool_drain seat=3 source= amount=1 target=seat0
[972] state seat=3 source= target=seat0
```

</details>

#### Violation 22

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 24, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 24, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 973 events
  Seat 0 [alive]: life=36 library=83 hand=4 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=31 library=81 hand=4 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=86 hand=5 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=81 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[953] priority_pass seat=0 source= target=seat0
[954] priority_pass seat=1 source= target=seat0
[955] priority_pass seat=2 source= target=seat0
[956] stack_resolve seat=3 source=Street Riot target=seat0
[957] enter_battlefield seat=3 source=Street Riot target=seat0
[958] trigger_evaluated seat=1 source=Genesis Chamber
[959] stack_push seat=1 source=Genesis Chamber target=seat0
[960] triggered_ability seat=1 source=Genesis Chamber target=seat0
[961] priority_pass seat=3 source= target=seat0
[962] priority_pass seat=0 source= target=seat0
[963] priority_pass seat=2 source= target=seat0
[964] stack_resolve seat=1 source=Genesis Chamber target=seat0
[965] phase_step seat=3 source= target=seat0
[966] declare_attackers seat=3 source= target=seat0
[967] blockers seat=1 source= target=seat0
[968] damage seat=3 source=The Master, Gallifrey's End amount=4 target=seat1
[969] damage seat=3 source=Coalition Warbrute amount=3 target=seat1
[970] phase_step seat=3 source= target=seat0
[971] pool_drain seat=3 source= amount=1 target=seat0
[972] state seat=3 source= target=seat0
```

</details>

#### Violation 23

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 25, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 25, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1027 events
  Seat 0 [alive]: life=36 library=82 hand=4 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abstergo Entertainment (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=31 library=81 hand=4 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=86 hand=5 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=81 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1007] priority_pass seat=2 source= target=seat0
[1008] priority_pass seat=3 source= target=seat0
[1009] stack_resolve seat=0 source=Lobelia, Defender of Bag End target=seat0
[1010] enter_battlefield seat=0 source=Lobelia, Defender of Bag End target=seat0
[1011] trigger_evaluated seat=1 source=Genesis Chamber
[1012] stack_push seat=1 source=Genesis Chamber target=seat0
[1013] triggered_ability seat=1 source=Genesis Chamber target=seat0
[1014] priority_pass seat=0 source= target=seat0
[1015] priority_pass seat=2 source= target=seat0
[1016] priority_pass seat=3 source= target=seat0
[1017] stack_resolve seat=1 source=Genesis Chamber target=seat0
[1018] stack_push seat=0 source=Lobelia, Defender of Bag End target=seat0
[1019] triggers_ordered seat=0 source= target=seat0
[1020] priority_pass seat=1 source= target=seat0
[1021] priority_pass seat=2 source= target=seat0
[1022] priority_pass seat=3 source= target=seat0
[1023] stack_resolve seat=0 source=Lobelia, Defender of Bag End target=seat0
[1024] parsed_effect_residual seat=0 source=Lobelia, Defender of Bag End target=seat0
[1025] pool_drain seat=0 source= amount=2 target=seat0
[1026] state seat=0 source= target=seat0
```

</details>

#### Violation 24

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 25, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 25, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1027 events
  Seat 0 [alive]: life=36 library=82 hand=4 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abstergo Entertainment (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=31 library=81 hand=4 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=86 hand=5 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=81 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1007] priority_pass seat=2 source= target=seat0
[1008] priority_pass seat=3 source= target=seat0
[1009] stack_resolve seat=0 source=Lobelia, Defender of Bag End target=seat0
[1010] enter_battlefield seat=0 source=Lobelia, Defender of Bag End target=seat0
[1011] trigger_evaluated seat=1 source=Genesis Chamber
[1012] stack_push seat=1 source=Genesis Chamber target=seat0
[1013] triggered_ability seat=1 source=Genesis Chamber target=seat0
[1014] priority_pass seat=0 source= target=seat0
[1015] priority_pass seat=2 source= target=seat0
[1016] priority_pass seat=3 source= target=seat0
[1017] stack_resolve seat=1 source=Genesis Chamber target=seat0
[1018] stack_push seat=0 source=Lobelia, Defender of Bag End target=seat0
[1019] triggers_ordered seat=0 source= target=seat0
[1020] priority_pass seat=1 source= target=seat0
[1021] priority_pass seat=2 source= target=seat0
[1022] priority_pass seat=3 source= target=seat0
[1023] stack_resolve seat=0 source=Lobelia, Defender of Bag End target=seat0
[1024] parsed_effect_residual seat=0 source=Lobelia, Defender of Bag End target=seat0
[1025] pool_drain seat=0 source= amount=2 target=seat0
[1026] state seat=0 source= target=seat0
```

</details>

#### Violation 25

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 26, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 26, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1050 events
  Seat 0 [alive]: life=36 library=82 hand=4 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abstergo Entertainment (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=31 library=80 hand=4 graveyard=10 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=86 hand=5 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=81 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1030] untap_done seat=1 source=Island target=seat0
[1031] untap_done seat=1 source=Island target=seat0
[1032] add_mana seat=1 source=Island amount=1 target=seat0
[1033] add_mana seat=1 source=Island amount=1 target=seat0
[1034] add_mana seat=1 source=Island amount=1 target=seat0
[1035] add_mana seat=1 source=Island amount=1 target=seat0
[1036] draw seat=1 source=Vapor Snag amount=1 target=seat0
[1037] pay_mana seat=1 source=Vapor Snag amount=1 target=seat0
[1038] cast seat=1 source=Vapor Snag amount=1 target=seat0
[1039] stack_push seat=1 source=Vapor Snag target=seat0
[1040] priority_pass seat=2 source= target=seat0
[1041] priority_pass seat=3 source= target=seat0
[1042] priority_pass seat=0 source= target=seat0
[1043] stack_resolve seat=1 source=Vapor Snag target=seat0
[1044] zone_change seat=1 source=Vapor Snag
[1045] resolve seat=1 source=Vapor Snag target=seat0
[1046] phase_step seat=1 source= target=seat0
[1047] phase_step seat=1 source= target=seat0
[1048] pool_drain seat=1 source= amount=3 target=seat0
[1049] state seat=1 source= target=seat0
```

</details>

#### Violation 26

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 26, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 26, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1050 events
  Seat 0 [alive]: life=36 library=82 hand=4 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abstergo Entertainment (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=31 library=80 hand=4 graveyard=10 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=86 hand=5 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=37 library=81 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1030] untap_done seat=1 source=Island target=seat0
[1031] untap_done seat=1 source=Island target=seat0
[1032] add_mana seat=1 source=Island amount=1 target=seat0
[1033] add_mana seat=1 source=Island amount=1 target=seat0
[1034] add_mana seat=1 source=Island amount=1 target=seat0
[1035] add_mana seat=1 source=Island amount=1 target=seat0
[1036] draw seat=1 source=Vapor Snag amount=1 target=seat0
[1037] pay_mana seat=1 source=Vapor Snag amount=1 target=seat0
[1038] cast seat=1 source=Vapor Snag amount=1 target=seat0
[1039] stack_push seat=1 source=Vapor Snag target=seat0
[1040] priority_pass seat=2 source= target=seat0
[1041] priority_pass seat=3 source= target=seat0
[1042] priority_pass seat=0 source= target=seat0
[1043] stack_resolve seat=1 source=Vapor Snag target=seat0
[1044] zone_change seat=1 source=Vapor Snag
[1045] resolve seat=1 source=Vapor Snag target=seat0
[1046] phase_step seat=1 source= target=seat0
[1047] phase_step seat=1 source= target=seat0
[1048] pool_drain seat=1 source= amount=3 target=seat0
[1049] state seat=1 source= target=seat0
```

</details>

#### Violation 27

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 27, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 27, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1095 events
  Seat 0 [alive]: life=36 library=82 hand=4 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abstergo Entertainment (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=31 library=80 hand=4 graveyard=10 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=85 hand=4 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Crown-Hunter Hireling (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=37 library=81 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1075] stack_resolve seat=2 source=Crown-Hunter Hireling target=seat0
[1076] enter_battlefield seat=2 source=Crown-Hunter Hireling target=seat0
[1077] trigger_evaluated seat=1 source=Genesis Chamber
[1078] stack_push seat=1 source=Genesis Chamber target=seat0
[1079] triggered_ability seat=1 source=Genesis Chamber target=seat0
[1080] priority_pass seat=2 source= target=seat0
[1081] priority_pass seat=3 source= target=seat0
[1082] priority_pass seat=0 source= target=seat0
[1083] stack_resolve seat=1 source=Genesis Chamber target=seat0
[1084] stack_push seat=2 source=Crown-Hunter Hireling target=seat0
[1085] triggers_ordered seat=2 source= target=seat0
[1086] priority_pass seat=3 source= target=seat0
[1087] priority_pass seat=0 source= target=seat0
[1088] priority_pass seat=1 source= target=seat0
[1089] stack_resolve seat=2 source=Crown-Hunter Hireling target=seat0
[1090] modification_effect seat=2 source=Crown-Hunter Hireling target=seat0
[1091] parser_gap seat=2 source=Crown-Hunter Hireling target=seat0
[1092] phase_step seat=2 source= target=seat0
[1093] phase_step seat=2 source= target=seat0
[1094] state seat=2 source= target=seat0
```

</details>

#### Violation 28

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 27, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 27, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1095 events
  Seat 0 [alive]: life=36 library=82 hand=4 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abstergo Entertainment (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=31 library=80 hand=4 graveyard=10 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=36 library=85 hand=4 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Crown-Hunter Hireling (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=37 library=81 hand=5 graveyard=4 exile=0 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Coalition Warbrute (P/T 3/4, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1075] stack_resolve seat=2 source=Crown-Hunter Hireling target=seat0
[1076] enter_battlefield seat=2 source=Crown-Hunter Hireling target=seat0
[1077] trigger_evaluated seat=1 source=Genesis Chamber
[1078] stack_push seat=1 source=Genesis Chamber target=seat0
[1079] triggered_ability seat=1 source=Genesis Chamber target=seat0
[1080] priority_pass seat=2 source= target=seat0
[1081] priority_pass seat=3 source= target=seat0
[1082] priority_pass seat=0 source= target=seat0
[1083] stack_resolve seat=1 source=Genesis Chamber target=seat0
[1084] stack_push seat=2 source=Crown-Hunter Hireling target=seat0
[1085] triggers_ordered seat=2 source= target=seat0
[1086] priority_pass seat=3 source= target=seat0
[1087] priority_pass seat=0 source= target=seat0
[1088] priority_pass seat=1 source= target=seat0
[1089] stack_resolve seat=2 source=Crown-Hunter Hireling target=seat0
[1090] modification_effect seat=2 source=Crown-Hunter Hireling target=seat0
[1091] parser_gap seat=2 source=Crown-Hunter Hireling target=seat0
[1092] phase_step seat=2 source= target=seat0
[1093] phase_step seat=2 source= target=seat0
[1094] state seat=2 source= target=seat0
```

</details>

#### Violation 29

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 28, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1125 events
  Seat 0 [alive]: life=36 library=82 hand=4 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abstergo Entertainment (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=31 library=80 hand=4 graveyard=10 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=32 library=85 hand=4 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Crown-Hunter Hireling (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=37 library=80 hand=6 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1105] add_mana seat=3 source=Mountain amount=1 target=seat0
[1106] add_mana seat=3 source=Mountain amount=1 target=seat0
[1107] add_mana seat=3 source=Mountain amount=1 target=seat0
[1108] add_mana seat=3 source=Swamp amount=1 target=seat0
[1109] add_mana seat=3 source=Matzalantli, the Great Door // The Core amount=1 target=seat0
[1110] draw seat=3 source=Havoc Devils amount=1 target=seat0
[1111] phase_step seat=3 source= target=seat0
[1112] declare_attackers seat=3 source= target=seat0
[1113] blockers seat=2 source= target=seat0
[1114] damage seat=3 source=The Master, Gallifrey's End amount=4 target=seat2
[1115] damage seat=3 source=Coalition Warbrute amount=3 target=seat2
[1116] damage seat=2 source=Crown-Hunter Hireling amount=4 target=seat3
[1117] destroy seat=3 source=Coalition Warbrute
[1118] sba_704_5g seat=3 source=Coalition Warbrute
[1119] zone_change seat=3 source=Coalition Warbrute
[1120] sba_cycle_complete seat=-1 source=
[1121] phase_step seat=3 source= target=seat0
[1122] pool_drain seat=3 source= amount=6 target=seat0
[1123] damage_wears_off seat=2 source=Crown-Hunter Hireling amount=3 target=seat0
[1124] state seat=3 source= target=seat0
```

</details>

#### Violation 30

- **Game**: 137 (seed 1370042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 28, Phase=ending Step=cleanup
- **Commanders**: Lobelia, Defender of Bag End, Nick Valentine, Private Eye, Moku, Meandering Drummer, The Master, Gallifrey's End
- **Message**: CardIdentity: card "Cerulean Sphinx" (ptr 0xc00714b560) appears in both seat 0 library and seat 1 library

<details>
<summary>Game State</summary>

```
Turn 28, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1125 events
  Seat 0 [alive]: life=36 library=82 hand=4 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Abstergo Entertainment (P/T 0/0, dmg=0) [T]
    - Lobelia, Defender of Bag End (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=31 library=80 hand=4 graveyard=10 exile=0 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Genesis Chamber (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=32 library=85 hand=4 graveyard=4 exile=0 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Crown-Hunter Hireling (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=37 library=80 hand=6 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Master, Gallifrey's End (P/T 4/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Matzalantli, the Great Door // The Core (P/T 0/0, dmg=0) [T]
    - Street Riot (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1105] add_mana seat=3 source=Mountain amount=1 target=seat0
[1106] add_mana seat=3 source=Mountain amount=1 target=seat0
[1107] add_mana seat=3 source=Mountain amount=1 target=seat0
[1108] add_mana seat=3 source=Swamp amount=1 target=seat0
[1109] add_mana seat=3 source=Matzalantli, the Great Door // The Core amount=1 target=seat0
[1110] draw seat=3 source=Havoc Devils amount=1 target=seat0
[1111] phase_step seat=3 source= target=seat0
[1112] declare_attackers seat=3 source= target=seat0
[1113] blockers seat=2 source= target=seat0
[1114] damage seat=3 source=The Master, Gallifrey's End amount=4 target=seat2
[1115] damage seat=3 source=Coalition Warbrute amount=3 target=seat2
[1116] damage seat=2 source=Crown-Hunter Hireling amount=4 target=seat3
[1117] destroy seat=3 source=Coalition Warbrute
[1118] sba_704_5g seat=3 source=Coalition Warbrute
[1119] zone_change seat=3 source=Coalition Warbrute
[1120] sba_cycle_complete seat=-1 source=
[1121] phase_step seat=3 source= target=seat0
[1122] pool_drain seat=3 source= amount=6 target=seat0
[1123] damage_wears_off seat=2 source=Crown-Hunter Hireling amount=3 target=seat0
[1124] state seat=3 source= target=seat0
```

</details>

*... and 1622 more violations not shown.*

## Invariant Violations (Nightmare Boards)

| Invariant | Count |
|-----------|-------|
| CardIdentity | 6 |

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Nevinyrral, Urborg Tyrant | 8 | 5 | 0.62 |
| 2 | Enduring Scalelord | 1 | 2 | 0.33 |
| 3 | Scarland Thrinax | 1 | 2 | 0.33 |
| 4 | Ashiok, Dream Render | 1 | 2 | 0.33 |
| 5 | Unite the Coalition | 1 | 2 | 0.33 |
| 6 | Cromat | 1 | 2 | 0.33 |
| 7 | Kheru Goldkeeper | 1 | 2 | 0.33 |
| 8 | Benthic Djinn | 1 | 2 | 0.33 |
| 9 | Kingpin's Pet | 1 | 2 | 0.33 |
| 10 | Baleful Strix | 2 | 4 | 0.33 |

## Verdict: ISSUES FOUND

**1658 total issues** across 5000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 1652 invariant violations in chaos games
- 0 crashes in nightmare boards
- 6 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
