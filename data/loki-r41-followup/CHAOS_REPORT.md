# Chaos Gauntlet Report

Generated: 2026-05-19T08:56:35-07:00

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
| Duration | 3m21.004s |
| Throughput | 25 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 1255 (in 59 games) |
| Clean Games | 4941 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 3.365s |
| Throughput | 2972 boards/sec |
| Crashes | 0 |
| Invariant Violations | 6 |
| Clean Boards | 9997 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| ZoneConservation | 824 |
| CardIdentity | 392 |
| TriggerCompleteness | 8 |
| ZoneCastGrantExpiry | 8 |
| AttachmentConsistency | 23 |

### Violation Details (first 30)

#### Violation 1

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 47, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 398, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 47, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2005 events
  Seat 0 [alive]: life=35 library=80 hand=3 graveyard=6 exile=0 battlefield=10 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=33 library=80 hand=7 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=13 library=55 hand=7 graveyard=18 exile=2 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=33 library=81 hand=3 graveyard=10 exile=0 battlefield=5 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1985] sba_704_5e seat=2 source= amount=1 target=seat0
[1986] sba_cycle_complete seat=-1 source=
[1987] play_land seat=2 source=Crumbling Vestige target=seat0
[1988] pay_mana seat=2 source=Wu Admiral amount=5 target=seat0
[1989] cast seat=2 source=Wu Admiral amount=5 target=seat0
[1990] stack_push seat=2 source=Wu Admiral target=seat0
[1991] priority_pass seat=3 source= target=seat0
[1992] priority_pass seat=0 source= target=seat0
[1993] priority_pass seat=1 source= target=seat0
[1994] stack_resolve seat=2 source=Wu Admiral target=seat0
[1995] enter_battlefield seat=2 source=Wu Admiral target=seat0
[1996] phase_step seat=2 source= target=seat0
[1997] declare_attackers seat=2 source= target=seat0
[1998] blockers seat=3 source= target=seat0
[1999] damage seat=2 source=Kotori, Pilot Prodigy amount=2 target=seat3
[2000] phase_step seat=2 source= target=seat0
[2001] pool_drain seat=2 source= amount=5 target=seat0
[2002] zone_change seat=2 source=Riding Red Hare
[2003] discard seat=2 source=Riding Red Hare target=seat0
[2004] state seat=2 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 47, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 398, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 47, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2005 events
  Seat 0 [alive]: life=35 library=80 hand=3 graveyard=6 exile=0 battlefield=10 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=33 library=80 hand=7 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=13 library=55 hand=7 graveyard=18 exile=2 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=33 library=81 hand=3 graveyard=10 exile=0 battlefield=5 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1985] sba_704_5e seat=2 source= amount=1 target=seat0
[1986] sba_cycle_complete seat=-1 source=
[1987] play_land seat=2 source=Crumbling Vestige target=seat0
[1988] pay_mana seat=2 source=Wu Admiral amount=5 target=seat0
[1989] cast seat=2 source=Wu Admiral amount=5 target=seat0
[1990] stack_push seat=2 source=Wu Admiral target=seat0
[1991] priority_pass seat=3 source= target=seat0
[1992] priority_pass seat=0 source= target=seat0
[1993] priority_pass seat=1 source= target=seat0
[1994] stack_resolve seat=2 source=Wu Admiral target=seat0
[1995] enter_battlefield seat=2 source=Wu Admiral target=seat0
[1996] phase_step seat=2 source= target=seat0
[1997] declare_attackers seat=2 source= target=seat0
[1998] blockers seat=3 source= target=seat0
[1999] damage seat=2 source=Kotori, Pilot Prodigy amount=2 target=seat3
[2000] phase_step seat=2 source= target=seat0
[2001] pool_drain seat=2 source= amount=5 target=seat0
[2002] zone_change seat=2 source=Riding Red Hare
[2003] discard seat=2 source=Riding Red Hare target=seat0
[2004] state seat=2 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 48, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 398, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 48, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2113 events
  Seat 0 [alive]: life=35 library=80 hand=3 graveyard=6 exile=0 battlefield=10 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=33 library=80 hand=7 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=13 library=55 hand=7 graveyard=18 exile=2 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=23 library=80 hand=3 graveyard=10 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Champion of the Weird (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2093] stack_resolve seat=3 source=Champion of the Weird target=seat0
[2094] parsed_effect_residual seat=3 source=Champion of the Weird target=seat0
[2095] activated_ability_resolved seat=3 source=Champion of the Weird target=seat0
[2096] activate_ability seat=3 source=Champion of the Weird target=seat0
[2097] stack_push seat=3 source=Champion of the Weird target=seat0
[2098] priority_pass seat=0 source= target=seat0
[2099] priority_pass seat=1 source= target=seat0
[2100] priority_pass seat=2 source= target=seat0
[2101] stack_resolve seat=3 source=Champion of the Weird target=seat0
[2102] parsed_effect_residual seat=3 source=Champion of the Weird target=seat0
[2103] activated_ability_resolved seat=3 source=Champion of the Weird target=seat0
[2104] activate_ability seat=3 source=Champion of the Weird target=seat0
[2105] stack_push seat=3 source=Champion of the Weird target=seat0
[2106] priority_pass seat=0 source= target=seat0
[2107] priority_pass seat=1 source= target=seat0
[2108] priority_pass seat=2 source= target=seat0
[2109] stack_resolve seat=3 source=Champion of the Weird target=seat0
[2110] parsed_effect_residual seat=3 source=Champion of the Weird target=seat0
[2111] activated_ability_resolved seat=3 source=Champion of the Weird target=seat0
[2112] state seat=3 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 48, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 398, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 48, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2113 events
  Seat 0 [alive]: life=35 library=80 hand=3 graveyard=6 exile=0 battlefield=10 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=33 library=80 hand=7 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=13 library=55 hand=7 graveyard=18 exile=2 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=23 library=80 hand=3 graveyard=10 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Champion of the Weird (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2093] stack_resolve seat=3 source=Champion of the Weird target=seat0
[2094] parsed_effect_residual seat=3 source=Champion of the Weird target=seat0
[2095] activated_ability_resolved seat=3 source=Champion of the Weird target=seat0
[2096] activate_ability seat=3 source=Champion of the Weird target=seat0
[2097] stack_push seat=3 source=Champion of the Weird target=seat0
[2098] priority_pass seat=0 source= target=seat0
[2099] priority_pass seat=1 source= target=seat0
[2100] priority_pass seat=2 source= target=seat0
[2101] stack_resolve seat=3 source=Champion of the Weird target=seat0
[2102] parsed_effect_residual seat=3 source=Champion of the Weird target=seat0
[2103] activated_ability_resolved seat=3 source=Champion of the Weird target=seat0
[2104] activate_ability seat=3 source=Champion of the Weird target=seat0
[2105] stack_push seat=3 source=Champion of the Weird target=seat0
[2106] priority_pass seat=0 source= target=seat0
[2107] priority_pass seat=1 source= target=seat0
[2108] priority_pass seat=2 source= target=seat0
[2109] stack_resolve seat=3 source=Champion of the Weird target=seat0
[2110] parsed_effect_residual seat=3 source=Champion of the Weird target=seat0
[2111] activated_ability_resolved seat=3 source=Champion of the Weird target=seat0
[2112] state seat=3 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 49, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 398, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 49, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2169 events
  Seat 0 [alive]: life=35 library=79 hand=2 graveyard=7 exile=0 battlefield=11 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=32 library=80 hand=7 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=13 library=55 hand=7 graveyard=18 exile=2 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=23 library=80 hand=3 graveyard=10 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Champion of the Weird (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2149] stack_resolve seat=0 source=Vulshok Berserker target=seat0
[2150] enter_battlefield seat=0 source=Vulshok Berserker target=seat0
[2151] tap seat=0 source=Blasting Station target=seat0
[2152] sacrifice seat=0 source=Vulshok Berserker target=seat0
[2153] zone_change seat=0 source=Vulshok Berserker
[2154] activate_ability seat=0 source=Blasting Station target=seat0
[2155] stack_push seat=0 source=Blasting Station target=seat0
[2156] priority_pass seat=1 source= target=seat0
[2157] priority_pass seat=2 source= target=seat0
[2158] priority_pass seat=3 source= target=seat0
[2159] stack_resolve seat=0 source=Blasting Station target=seat0
[2160] commit_crime seat=0 source=Blasting Station amount=1 target=seat0
[2161] damage seat=0 source=Blasting Station amount=1 target=seat1
[2162] life_change seat=1 source=Blasting Station amount=-1 target=seat0
[2163] speed_advance seat=0 source= amount=4 target=seat0
[2164] activated_ability_resolved seat=0 source=Blasting Station target=seat0
[2165] phase_step seat=0 source= target=seat0
[2166] phase_step seat=0 source= target=seat0
[2167] pool_drain seat=0 source= amount=1 target=seat0
[2168] state seat=0 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 49, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 398, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 49, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2169 events
  Seat 0 [alive]: life=35 library=79 hand=2 graveyard=7 exile=0 battlefield=11 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=32 library=80 hand=7 graveyard=5 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=13 library=55 hand=7 graveyard=18 exile=2 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=23 library=80 hand=3 graveyard=10 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Champion of the Weird (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2149] stack_resolve seat=0 source=Vulshok Berserker target=seat0
[2150] enter_battlefield seat=0 source=Vulshok Berserker target=seat0
[2151] tap seat=0 source=Blasting Station target=seat0
[2152] sacrifice seat=0 source=Vulshok Berserker target=seat0
[2153] zone_change seat=0 source=Vulshok Berserker
[2154] activate_ability seat=0 source=Blasting Station target=seat0
[2155] stack_push seat=0 source=Blasting Station target=seat0
[2156] priority_pass seat=1 source= target=seat0
[2157] priority_pass seat=2 source= target=seat0
[2158] priority_pass seat=3 source= target=seat0
[2159] stack_resolve seat=0 source=Blasting Station target=seat0
[2160] commit_crime seat=0 source=Blasting Station amount=1 target=seat0
[2161] damage seat=0 source=Blasting Station amount=1 target=seat1
[2162] life_change seat=1 source=Blasting Station amount=-1 target=seat0
[2163] speed_advance seat=0 source= amount=4 target=seat0
[2164] activated_ability_resolved seat=0 source=Blasting Station target=seat0
[2165] phase_step seat=0 source= target=seat0
[2166] phase_step seat=0 source= target=seat0
[2167] pool_drain seat=0 source= amount=1 target=seat0
[2168] state seat=0 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 50, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 398, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 50, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2236 events
  Seat 0 [alive]: life=35 library=79 hand=2 graveyard=7 exile=0 battlefield=11 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=32 library=79 hand=7 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=13 library=55 hand=7 graveyard=18 exile=2 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=23 library=80 hand=3 graveyard=10 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Champion of the Weird (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2216] priority_pass seat=0 source= target=seat0
[2217] stack_resolve seat=1 source=Mutavault target=seat0
[2218] modification_effect seat=1 source=Mutavault target=seat0
[2219] parser_gap seat=1 source=Mutavault target=seat0
[2220] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2221] pay_mana seat=1 source=Mutavault amount=1 target=seat0
[2222] activate_ability seat=1 source=Mutavault target=seat0
[2223] stack_push seat=1 source=Mutavault target=seat0
[2224] priority_pass seat=2 source= target=seat0
[2225] priority_pass seat=3 source= target=seat0
[2226] priority_pass seat=0 source= target=seat0
[2227] stack_resolve seat=1 source=Mutavault target=seat0
[2228] modification_effect seat=1 source=Mutavault target=seat0
[2229] parser_gap seat=1 source=Mutavault target=seat0
[2230] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2231] phase_step seat=1 source= target=seat0
[2232] phase_step seat=1 source= target=seat0
[2233] zone_change seat=1 source=Serendib Djinn
[2234] discard seat=1 source=Serendib Djinn target=seat0
[2235] state seat=1 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 50, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 398, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 50, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2236 events
  Seat 0 [alive]: life=35 library=79 hand=2 graveyard=7 exile=0 battlefield=11 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=32 library=79 hand=7 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=13 library=55 hand=7 graveyard=18 exile=2 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0)
  Seat 3 [alive]: life=23 library=80 hand=3 graveyard=10 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Champion of the Weird (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2216] priority_pass seat=0 source= target=seat0
[2217] stack_resolve seat=1 source=Mutavault target=seat0
[2218] modification_effect seat=1 source=Mutavault target=seat0
[2219] parser_gap seat=1 source=Mutavault target=seat0
[2220] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2221] pay_mana seat=1 source=Mutavault amount=1 target=seat0
[2222] activate_ability seat=1 source=Mutavault target=seat0
[2223] stack_push seat=1 source=Mutavault target=seat0
[2224] priority_pass seat=2 source= target=seat0
[2225] priority_pass seat=3 source= target=seat0
[2226] priority_pass seat=0 source= target=seat0
[2227] stack_resolve seat=1 source=Mutavault target=seat0
[2228] modification_effect seat=1 source=Mutavault target=seat0
[2229] parser_gap seat=1 source=Mutavault target=seat0
[2230] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2231] phase_step seat=1 source= target=seat0
[2232] phase_step seat=1 source= target=seat0
[2233] zone_change seat=1 source=Serendib Djinn
[2234] discard seat=1 source=Serendib Djinn target=seat0
[2235] state seat=1 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 51, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 398, found 410) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 51, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2340 events
  Seat 0 [alive]: life=30 library=79 hand=2 graveyard=7 exile=0 battlefield=11 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=32 library=79 hand=7 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=11 library=52 hand=5 graveyard=19 exile=2 battlefield=20 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0)
    - Silent Attendant (P/T 0/2, dmg=0)
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=80 hand=3 graveyard=11 exile=0 battlefield=5 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2320] priority_pass seat=3 source= target=seat0
[2321] priority_pass seat=0 source= target=seat0
[2322] priority_pass seat=1 source= target=seat0
[2323] stack_resolve seat=2 source=Andúril, Narsil Reforged target=seat0
[2324] enter_battlefield seat=2 source=Andúril, Narsil Reforged target=seat0
[2325] cast seat=2 source=My Laughter Echoes target=seat0
[2326] stack_push seat=2 source=My Laughter Echoes target=seat0
[2327] priority_pass seat=3 source= target=seat0
[2328] priority_pass seat=0 source= target=seat0
[2329] priority_pass seat=1 source= target=seat0
[2330] stack_resolve seat=2 source=My Laughter Echoes target=seat0
[2331] zone_change seat=2 source=My Laughter Echoes
[2332] resolve seat=2 source=My Laughter Echoes target=seat0
[2333] phase_step seat=2 source= target=seat0
[2334] declare_attackers seat=2 source= target=seat0
[2335] blockers seat=0 source= target=seat0
[2336] damage seat=2 source=Kotori, Pilot Prodigy amount=2 target=seat0
[2337] damage seat=2 source=Wu Admiral amount=3 target=seat0
[2338] phase_step seat=2 source= target=seat0
[2339] state seat=2 source= target=seat0
```

</details>

#### Violation 10

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 51, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 398, found 410) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 51, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2340 events
  Seat 0 [alive]: life=30 library=79 hand=2 graveyard=7 exile=0 battlefield=11 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=32 library=79 hand=7 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=11 library=52 hand=5 graveyard=19 exile=2 battlefield=20 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0)
    - Silent Attendant (P/T 0/2, dmg=0)
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=80 hand=3 graveyard=11 exile=0 battlefield=5 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2320] priority_pass seat=3 source= target=seat0
[2321] priority_pass seat=0 source= target=seat0
[2322] priority_pass seat=1 source= target=seat0
[2323] stack_resolve seat=2 source=Andúril, Narsil Reforged target=seat0
[2324] enter_battlefield seat=2 source=Andúril, Narsil Reforged target=seat0
[2325] cast seat=2 source=My Laughter Echoes target=seat0
[2326] stack_push seat=2 source=My Laughter Echoes target=seat0
[2327] priority_pass seat=3 source= target=seat0
[2328] priority_pass seat=0 source= target=seat0
[2329] priority_pass seat=1 source= target=seat0
[2330] stack_resolve seat=2 source=My Laughter Echoes target=seat0
[2331] zone_change seat=2 source=My Laughter Echoes
[2332] resolve seat=2 source=My Laughter Echoes target=seat0
[2333] phase_step seat=2 source= target=seat0
[2334] declare_attackers seat=2 source= target=seat0
[2335] blockers seat=0 source= target=seat0
[2336] damage seat=2 source=Kotori, Pilot Prodigy amount=2 target=seat0
[2337] damage seat=2 source=Wu Admiral amount=3 target=seat0
[2338] phase_step seat=2 source= target=seat0
[2339] state seat=2 source= target=seat0
```

</details>

#### Violation 11

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 398, found 410) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2382 events
  Seat 0 [alive]: life=30 library=79 hand=2 graveyard=7 exile=0 battlefield=11 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=32 library=79 hand=7 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=11 library=52 hand=5 graveyard=19 exile=2 battlefield=20 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0)
    - Silent Attendant (P/T 0/2, dmg=0)
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=79 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fludge, Gunk Guardian (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2362] priority_pass seat=0 source= target=seat0
[2363] priority_pass seat=1 source= target=seat0
[2364] priority_pass seat=2 source= target=seat0
[2365] stack_resolve seat=3 source=Fludge, Gunk Guardian target=seat0
[2366] enter_battlefield seat=3 source=Fludge, Gunk Guardian target=seat0
[2367] stack_push seat=3 source=Fludge, Gunk Guardian target=seat0
[2368] triggers_ordered seat=3 source= target=seat0
[2369] priority_pass seat=0 source= target=seat0
[2370] priority_pass seat=1 source= target=seat0
[2371] priority_pass seat=2 source= target=seat0
[2372] stack_resolve seat=3 source=Fludge, Gunk Guardian target=seat0
[2373] untyped_effect seat=3 source=Fludge, Gunk Guardian target=seat0
[2374] stack_push seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2375] triggers_ordered seat=3 source= target=seat0
[2376] priority_pass seat=0 source= target=seat0
[2377] priority_pass seat=1 source= target=seat0
[2378] priority_pass seat=2 source= target=seat0
[2379] stack_resolve seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2380] conditional_effect_false seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2381] state seat=3 source= target=seat0
```

</details>

#### Violation 12

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 398, found 410) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2382 events
  Seat 0 [alive]: life=30 library=79 hand=2 graveyard=7 exile=0 battlefield=11 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=32 library=79 hand=7 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=11 library=52 hand=5 graveyard=19 exile=2 battlefield=20 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0)
    - Silent Attendant (P/T 0/2, dmg=0)
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=79 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fludge, Gunk Guardian (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2362] priority_pass seat=0 source= target=seat0
[2363] priority_pass seat=1 source= target=seat0
[2364] priority_pass seat=2 source= target=seat0
[2365] stack_resolve seat=3 source=Fludge, Gunk Guardian target=seat0
[2366] enter_battlefield seat=3 source=Fludge, Gunk Guardian target=seat0
[2367] stack_push seat=3 source=Fludge, Gunk Guardian target=seat0
[2368] triggers_ordered seat=3 source= target=seat0
[2369] priority_pass seat=0 source= target=seat0
[2370] priority_pass seat=1 source= target=seat0
[2371] priority_pass seat=2 source= target=seat0
[2372] stack_resolve seat=3 source=Fludge, Gunk Guardian target=seat0
[2373] untyped_effect seat=3 source=Fludge, Gunk Guardian target=seat0
[2374] stack_push seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2375] triggers_ordered seat=3 source= target=seat0
[2376] priority_pass seat=0 source= target=seat0
[2377] priority_pass seat=1 source= target=seat0
[2378] priority_pass seat=2 source= target=seat0
[2379] stack_resolve seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2380] conditional_effect_false seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2381] state seat=3 source= target=seat0
```

</details>

#### Violation 13

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 53, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 398, found 410) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 53, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2445 events
  Seat 0 [alive]: life=30 library=78 hand=2 graveyard=7 exile=0 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=31 library=79 hand=7 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=11 library=52 hand=5 graveyard=19 exile=2 battlefield=20 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0)
    - Silent Attendant (P/T 0/2, dmg=0)
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=79 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fludge, Gunk Guardian (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2425] priority_pass seat=2 source= target=seat0
[2426] priority_pass seat=3 source= target=seat0
[2427] stack_resolve seat=0 source=Krark, the Thumbless target=seat0
[2428] enter_battlefield seat=0 source=Krark, the Thumbless target=seat0
[2429] tap seat=0 source=Blasting Station target=seat0
[2430] sacrifice seat=0 source=Krark, the Thumbless target=seat0
[2431] zone_change seat=0 source=Krark, the Thumbless
[2432] activate_ability seat=0 source=Blasting Station target=seat0
[2433] stack_push seat=0 source=Blasting Station target=seat0
[2434] priority_pass seat=1 source= target=seat0
[2435] priority_pass seat=2 source= target=seat0
[2436] priority_pass seat=3 source= target=seat0
[2437] stack_resolve seat=0 source=Blasting Station target=seat0
[2438] commit_crime seat=0 source=Blasting Station amount=1 target=seat0
[2439] damage seat=0 source=Blasting Station amount=1 target=seat1
[2440] life_change seat=1 source=Blasting Station amount=-1 target=seat0
[2441] activated_ability_resolved seat=0 source=Blasting Station target=seat0
[2442] sba_704_6d seat=0 source=Krark, the Thumbless
[2443] sba_cycle_complete seat=-1 source=
[2444] state seat=0 source= target=seat0
```

</details>

#### Violation 14

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 53, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 398, found 410) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 53, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2445 events
  Seat 0 [alive]: life=30 library=78 hand=2 graveyard=7 exile=0 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=31 library=79 hand=7 graveyard=6 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=11 library=52 hand=5 graveyard=19 exile=2 battlefield=20 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0)
    - Silent Attendant (P/T 0/2, dmg=0)
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=79 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fludge, Gunk Guardian (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2425] priority_pass seat=2 source= target=seat0
[2426] priority_pass seat=3 source= target=seat0
[2427] stack_resolve seat=0 source=Krark, the Thumbless target=seat0
[2428] enter_battlefield seat=0 source=Krark, the Thumbless target=seat0
[2429] tap seat=0 source=Blasting Station target=seat0
[2430] sacrifice seat=0 source=Krark, the Thumbless target=seat0
[2431] zone_change seat=0 source=Krark, the Thumbless
[2432] activate_ability seat=0 source=Blasting Station target=seat0
[2433] stack_push seat=0 source=Blasting Station target=seat0
[2434] priority_pass seat=1 source= target=seat0
[2435] priority_pass seat=2 source= target=seat0
[2436] priority_pass seat=3 source= target=seat0
[2437] stack_resolve seat=0 source=Blasting Station target=seat0
[2438] commit_crime seat=0 source=Blasting Station amount=1 target=seat0
[2439] damage seat=0 source=Blasting Station amount=1 target=seat1
[2440] life_change seat=1 source=Blasting Station amount=-1 target=seat0
[2441] activated_ability_resolved seat=0 source=Blasting Station target=seat0
[2442] sba_704_6d seat=0 source=Krark, the Thumbless
[2443] sba_cycle_complete seat=-1 source=
[2444] state seat=0 source= target=seat0
```

</details>

#### Violation 15

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 54, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 398, found 410) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 54, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2522 events
  Seat 0 [alive]: life=30 library=78 hand=2 graveyard=7 exile=0 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=31 library=78 hand=7 graveyard=6 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=11 library=52 hand=5 graveyard=19 exile=2 battlefield=20 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0)
    - Silent Attendant (P/T 0/2, dmg=0)
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=79 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fludge, Gunk Guardian (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2502] priority_pass seat=0 source= target=seat0
[2503] stack_resolve seat=1 source=Mutavault target=seat0
[2504] modification_effect seat=1 source=Mutavault target=seat0
[2505] parser_gap seat=1 source=Mutavault target=seat0
[2506] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2507] play_land seat=1 source=Island target=seat0
[2508] add_mana seat=1 source=Island amount=1 target=seat0
[2509] pay_mana seat=1 source=Mutavault amount=1 target=seat0
[2510] activate_ability seat=1 source=Mutavault target=seat0
[2511] stack_push seat=1 source=Mutavault target=seat0
[2512] priority_pass seat=2 source= target=seat0
[2513] priority_pass seat=3 source= target=seat0
[2514] priority_pass seat=0 source= target=seat0
[2515] stack_resolve seat=1 source=Mutavault target=seat0
[2516] modification_effect seat=1 source=Mutavault target=seat0
[2517] parser_gap seat=1 source=Mutavault target=seat0
[2518] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2519] phase_step seat=1 source= target=seat0
[2520] phase_step seat=1 source= target=seat0
[2521] state seat=1 source= target=seat0
```

</details>

#### Violation 16

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 54, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 398, found 410) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 54, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2522 events
  Seat 0 [alive]: life=30 library=78 hand=2 graveyard=7 exile=0 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=31 library=78 hand=7 graveyard=6 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=11 library=52 hand=5 graveyard=19 exile=2 battlefield=20 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0)
    - Silent Attendant (P/T 0/2, dmg=0)
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=79 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fludge, Gunk Guardian (P/T 5/5, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2502] priority_pass seat=0 source= target=seat0
[2503] stack_resolve seat=1 source=Mutavault target=seat0
[2504] modification_effect seat=1 source=Mutavault target=seat0
[2505] parser_gap seat=1 source=Mutavault target=seat0
[2506] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2507] play_land seat=1 source=Island target=seat0
[2508] add_mana seat=1 source=Island amount=1 target=seat0
[2509] pay_mana seat=1 source=Mutavault amount=1 target=seat0
[2510] activate_ability seat=1 source=Mutavault target=seat0
[2511] stack_push seat=1 source=Mutavault target=seat0
[2512] priority_pass seat=2 source= target=seat0
[2513] priority_pass seat=3 source= target=seat0
[2514] priority_pass seat=0 source= target=seat0
[2515] stack_resolve seat=1 source=Mutavault target=seat0
[2516] modification_effect seat=1 source=Mutavault target=seat0
[2517] parser_gap seat=1 source=Mutavault target=seat0
[2518] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2519] phase_step seat=1 source= target=seat0
[2520] phase_step seat=1 source= target=seat0
[2521] state seat=1 source= target=seat0
```

</details>

#### Violation 17

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 55, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 398, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 55, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2671 events
  Seat 0 [alive]: life=30 library=78 hand=2 graveyard=7 exile=0 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=22 library=78 hand=7 graveyard=6 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=48 hand=3 graveyard=19 exile=2 battlefield=27 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0)
    - Splendor Mare (P/T 3/3, dmg=0)
    - Wingrattle Scarecrow (P/T 2/2, dmg=0)
    - Lord Jyscal Guado (P/T 2/1, dmg=0)
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=79 hand=3 graveyard=11 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2651] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2652] priority_pass seat=3 source= target=seat0
[2653] priority_pass seat=0 source= target=seat0
[2654] priority_pass seat=1 source= target=seat0
[2655] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2656] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2657] triggers_ordered seat=2 source= target=seat0
[2658] priority_pass seat=3 source= target=seat0
[2659] priority_pass seat=0 source= target=seat0
[2660] priority_pass seat=1 source= target=seat0
[2661] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2662] create_token seat=2 source=Lord Jyscal Guado amount=1 target=seat2
[2663] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2664] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2665] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2666] priority_pass seat=3 source= target=seat0
[2667] priority_pass seat=0 source= target=seat0
[2668] priority_pass seat=1 source= target=seat0
[2669] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2670] state seat=2 source= target=seat0
```

</details>

#### Violation 18

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 55, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 398, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 55, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2671 events
  Seat 0 [alive]: life=30 library=78 hand=2 graveyard=7 exile=0 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=22 library=78 hand=7 graveyard=6 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=48 hand=3 graveyard=19 exile=2 battlefield=27 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0)
    - Splendor Mare (P/T 3/3, dmg=0)
    - Wingrattle Scarecrow (P/T 2/2, dmg=0)
    - Lord Jyscal Guado (P/T 2/1, dmg=0)
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=79 hand=3 graveyard=11 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2651] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2652] priority_pass seat=3 source= target=seat0
[2653] priority_pass seat=0 source= target=seat0
[2654] priority_pass seat=1 source= target=seat0
[2655] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2656] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2657] triggers_ordered seat=2 source= target=seat0
[2658] priority_pass seat=3 source= target=seat0
[2659] priority_pass seat=0 source= target=seat0
[2660] priority_pass seat=1 source= target=seat0
[2661] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2662] create_token seat=2 source=Lord Jyscal Guado amount=1 target=seat2
[2663] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2664] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2665] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2666] priority_pass seat=3 source= target=seat0
[2667] priority_pass seat=0 source= target=seat0
[2668] priority_pass seat=1 source= target=seat0
[2669] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2670] state seat=2 source= target=seat0
```

</details>

#### Violation 19

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 56, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 398, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 56, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2716 events
  Seat 0 [alive]: life=30 library=78 hand=2 graveyard=7 exile=0 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=22 library=78 hand=7 graveyard=6 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=48 hand=3 graveyard=19 exile=2 battlefield=27 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0)
    - Splendor Mare (P/T 3/3, dmg=0)
    - Wingrattle Scarecrow (P/T 2/2, dmg=0)
    - Lord Jyscal Guado (P/T 2/1, dmg=0)
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=78 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Scarwood Hag (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2696] priority_pass seat=3 source= target=seat0
[2697] priority_pass seat=0 source= target=seat0
[2698] priority_pass seat=1 source= target=seat0
[2699] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2700] stack_push seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2701] triggers_ordered seat=3 source= target=seat0
[2702] priority_pass seat=0 source= target=seat0
[2703] priority_pass seat=1 source= target=seat0
[2704] priority_pass seat=2 source= target=seat0
[2705] stack_resolve seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2706] conditional_effect_false seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2707] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2708] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2709] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2710] priority_pass seat=3 source= target=seat0
[2711] priority_pass seat=0 source= target=seat0
[2712] priority_pass seat=1 source= target=seat0
[2713] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2714] pool_drain seat=3 source= amount=1 target=seat0
[2715] state seat=3 source= target=seat0
```

</details>

#### Violation 20

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 56, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 398, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 56, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2716 events
  Seat 0 [alive]: life=30 library=78 hand=2 graveyard=7 exile=0 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=22 library=78 hand=7 graveyard=6 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=48 hand=3 graveyard=19 exile=2 battlefield=27 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0)
    - Splendor Mare (P/T 3/3, dmg=0)
    - Wingrattle Scarecrow (P/T 2/2, dmg=0)
    - Lord Jyscal Guado (P/T 2/1, dmg=0)
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=78 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Scarwood Hag (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2696] priority_pass seat=3 source= target=seat0
[2697] priority_pass seat=0 source= target=seat0
[2698] priority_pass seat=1 source= target=seat0
[2699] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2700] stack_push seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2701] triggers_ordered seat=3 source= target=seat0
[2702] priority_pass seat=0 source= target=seat0
[2703] priority_pass seat=1 source= target=seat0
[2704] priority_pass seat=2 source= target=seat0
[2705] stack_resolve seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2706] conditional_effect_false seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[2707] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2708] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2709] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2710] priority_pass seat=3 source= target=seat0
[2711] priority_pass seat=0 source= target=seat0
[2712] priority_pass seat=1 source= target=seat0
[2713] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2714] pool_drain seat=3 source= amount=1 target=seat0
[2715] state seat=3 source= target=seat0
```

</details>

#### Violation 21

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 57, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 398, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 57, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2774 events
  Seat 0 [alive]: life=30 library=77 hand=2 graveyard=7 exile=0 battlefield=13 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Brittle Effigy (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=78 hand=7 graveyard=6 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=48 hand=3 graveyard=19 exile=2 battlefield=27 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0)
    - Splendor Mare (P/T 3/3, dmg=0)
    - Wingrattle Scarecrow (P/T 2/2, dmg=0)
    - Lord Jyscal Guado (P/T 2/1, dmg=0)
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=78 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Scarwood Hag (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2754] stack_resolve seat=0 source=Brittle Effigy target=seat0
[2755] enter_battlefield seat=0 source=Brittle Effigy target=seat0
[2756] phase_step seat=0 source= target=seat0
[2757] phase_step seat=0 source= target=seat0
[2758] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2759] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2760] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2761] priority_pass seat=0 source= target=seat0
[2762] priority_pass seat=1 source= target=seat0
[2763] priority_pass seat=3 source= target=seat0
[2764] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2765] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2766] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2767] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2768] priority_pass seat=0 source= target=seat0
[2769] priority_pass seat=1 source= target=seat0
[2770] priority_pass seat=3 source= target=seat0
[2771] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2772] pool_drain seat=0 source= amount=2 target=seat0
[2773] state seat=0 source= target=seat0
```

</details>

#### Violation 22

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 57, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 398, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 57, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2774 events
  Seat 0 [alive]: life=30 library=77 hand=2 graveyard=7 exile=0 battlefield=13 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Brittle Effigy (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=78 hand=7 graveyard=6 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=48 hand=3 graveyard=19 exile=2 battlefield=27 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0)
    - Splendor Mare (P/T 3/3, dmg=0)
    - Wingrattle Scarecrow (P/T 2/2, dmg=0)
    - Lord Jyscal Guado (P/T 2/1, dmg=0)
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=78 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Scarwood Hag (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2754] stack_resolve seat=0 source=Brittle Effigy target=seat0
[2755] enter_battlefield seat=0 source=Brittle Effigy target=seat0
[2756] phase_step seat=0 source= target=seat0
[2757] phase_step seat=0 source= target=seat0
[2758] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2759] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2760] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2761] priority_pass seat=0 source= target=seat0
[2762] priority_pass seat=1 source= target=seat0
[2763] priority_pass seat=3 source= target=seat0
[2764] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2765] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2766] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2767] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2768] priority_pass seat=0 source= target=seat0
[2769] priority_pass seat=1 source= target=seat0
[2770] priority_pass seat=3 source= target=seat0
[2771] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2772] pool_drain seat=0 source= amount=2 target=seat0
[2773] state seat=0 source= target=seat0
```

</details>

#### Violation 23

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 58, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 398, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 58, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2867 events
  Seat 0 [alive]: life=30 library=77 hand=2 graveyard=7 exile=0 battlefield=13 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Brittle Effigy (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=77 hand=7 graveyard=7 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=48 hand=3 graveyard=19 exile=2 battlefield=27 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0)
    - Splendor Mare (P/T 3/3, dmg=0)
    - Wingrattle Scarecrow (P/T 2/2, dmg=0)
    - Lord Jyscal Guado (P/T 2/1, dmg=0)
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=78 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Scarwood Hag (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2847] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2848] phase_step seat=1 source= target=seat0
[2849] phase_step seat=1 source= target=seat0
[2850] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2851] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2852] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2853] priority_pass seat=1 source= target=seat0
[2854] priority_pass seat=3 source= target=seat0
[2855] priority_pass seat=0 source= target=seat0
[2856] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2857] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2858] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2859] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2860] priority_pass seat=1 source= target=seat0
[2861] priority_pass seat=3 source= target=seat0
[2862] priority_pass seat=0 source= target=seat0
[2863] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2864] zone_change seat=1 source=Irma, Part-Time Mutant
[2865] discard seat=1 source=Irma, Part-Time Mutant target=seat0
[2866] state seat=1 source= target=seat0
```

</details>

#### Violation 24

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 58, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 398, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 58, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2867 events
  Seat 0 [alive]: life=30 library=77 hand=2 graveyard=7 exile=0 battlefield=13 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Brittle Effigy (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=77 hand=7 graveyard=7 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=48 hand=3 graveyard=19 exile=2 battlefield=27 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0)
    - Splendor Mare (P/T 3/3, dmg=0)
    - Wingrattle Scarecrow (P/T 2/2, dmg=0)
    - Lord Jyscal Guado (P/T 2/1, dmg=0)
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=78 hand=3 graveyard=11 exile=0 battlefield=7 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Scarwood Hag (P/T 1/1, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2847] activated_ability_resolved seat=1 source=Mutavault target=seat0
[2848] phase_step seat=1 source= target=seat0
[2849] phase_step seat=1 source= target=seat0
[2850] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2851] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2852] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2853] priority_pass seat=1 source= target=seat0
[2854] priority_pass seat=3 source= target=seat0
[2855] priority_pass seat=0 source= target=seat0
[2856] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2857] trigger_evaluated seat=2 source=Lord Jyscal Guado
[2858] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[2859] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[2860] priority_pass seat=1 source= target=seat0
[2861] priority_pass seat=3 source= target=seat0
[2862] priority_pass seat=0 source= target=seat0
[2863] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[2864] zone_change seat=1 source=Irma, Part-Time Mutant
[2865] discard seat=1 source=Irma, Part-Time Mutant target=seat0
[2866] state seat=1 source= target=seat0
```

</details>

#### Violation 25

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 59, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 398, found 412) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 59, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 3138 events
  Seat 0 [alive]: life=11 library=77 hand=2 graveyard=7 exile=0 battlefield=13 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Brittle Effigy (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=77 hand=7 graveyard=7 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=12 library=45 hand=2 graveyard=20 exile=2 battlefield=31 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0) [T]
    - Splendor Mare (P/T 3/3, dmg=0) [T]
    - Wingrattle Scarecrow (P/T 2/2, dmg=0) [T]
    - Lord Jyscal Guado (P/T 2/1, dmg=0) [T]
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Sorcerer's Wand (P/T 0/0, dmg=0)
    - Vizier of Remedies (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=78 hand=3 graveyard=12 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3118] stack_resolve seat=2 source=Sorcerer's Wand target=seat0
[3119] parsed_effect_residual seat=2 source=Sorcerer's Wand target=seat0
[3120] activated_ability_resolved seat=2 source=Sorcerer's Wand target=seat0
[3121] activate_ability seat=2 source=Sorcerer's Wand target=seat0
[3122] stack_push seat=2 source=Sorcerer's Wand target=seat0
[3123] priority_pass seat=3 source= target=seat0
[3124] priority_pass seat=0 source= target=seat0
[3125] priority_pass seat=1 source= target=seat0
[3126] stack_resolve seat=2 source=Sorcerer's Wand target=seat0
[3127] parsed_effect_residual seat=2 source=Sorcerer's Wand target=seat0
[3128] activated_ability_resolved seat=2 source=Sorcerer's Wand target=seat0
[3129] activate_ability seat=2 source=Sorcerer's Wand target=seat0
[3130] stack_push seat=2 source=Sorcerer's Wand target=seat0
[3131] priority_pass seat=3 source= target=seat0
[3132] priority_pass seat=0 source= target=seat0
[3133] priority_pass seat=1 source= target=seat0
[3134] stack_resolve seat=2 source=Sorcerer's Wand target=seat0
[3135] parsed_effect_residual seat=2 source=Sorcerer's Wand target=seat0
[3136] activated_ability_resolved seat=2 source=Sorcerer's Wand target=seat0
[3137] state seat=2 source= target=seat0
```

</details>

#### Violation 26

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 59, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 398, found 412) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 59, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 3138 events
  Seat 0 [alive]: life=11 library=77 hand=2 graveyard=7 exile=0 battlefield=13 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Brittle Effigy (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=77 hand=7 graveyard=7 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=12 library=45 hand=2 graveyard=20 exile=2 battlefield=31 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0) [T]
    - Splendor Mare (P/T 3/3, dmg=0) [T]
    - Wingrattle Scarecrow (P/T 2/2, dmg=0) [T]
    - Lord Jyscal Guado (P/T 2/1, dmg=0) [T]
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Sorcerer's Wand (P/T 0/0, dmg=0)
    - Vizier of Remedies (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=78 hand=3 graveyard=12 exile=0 battlefield=6 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3118] stack_resolve seat=2 source=Sorcerer's Wand target=seat0
[3119] parsed_effect_residual seat=2 source=Sorcerer's Wand target=seat0
[3120] activated_ability_resolved seat=2 source=Sorcerer's Wand target=seat0
[3121] activate_ability seat=2 source=Sorcerer's Wand target=seat0
[3122] stack_push seat=2 source=Sorcerer's Wand target=seat0
[3123] priority_pass seat=3 source= target=seat0
[3124] priority_pass seat=0 source= target=seat0
[3125] priority_pass seat=1 source= target=seat0
[3126] stack_resolve seat=2 source=Sorcerer's Wand target=seat0
[3127] parsed_effect_residual seat=2 source=Sorcerer's Wand target=seat0
[3128] activated_ability_resolved seat=2 source=Sorcerer's Wand target=seat0
[3129] activate_ability seat=2 source=Sorcerer's Wand target=seat0
[3130] stack_push seat=2 source=Sorcerer's Wand target=seat0
[3131] priority_pass seat=3 source= target=seat0
[3132] priority_pass seat=0 source= target=seat0
[3133] priority_pass seat=1 source= target=seat0
[3134] stack_resolve seat=2 source=Sorcerer's Wand target=seat0
[3135] parsed_effect_residual seat=2 source=Sorcerer's Wand target=seat0
[3136] activated_ability_resolved seat=2 source=Sorcerer's Wand target=seat0
[3137] state seat=2 source= target=seat0
```

</details>

#### Violation 27

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 60, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 398, found 412) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 60, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 3177 events
  Seat 0 [alive]: life=11 library=77 hand=2 graveyard=7 exile=0 battlefield=13 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Brittle Effigy (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=77 hand=7 graveyard=7 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=12 library=45 hand=2 graveyard=20 exile=2 battlefield=31 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0) [T]
    - Splendor Mare (P/T 3/3, dmg=0) [T]
    - Wingrattle Scarecrow (P/T 2/2, dmg=0) [T]
    - Lord Jyscal Guado (P/T 2/1, dmg=0) [T]
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Sorcerer's Wand (P/T 0/0, dmg=0)
    - Vizier of Remedies (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=77 hand=3 graveyard=12 exile=0 battlefield=7 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3157] priority_pass seat=3 source= target=seat0
[3158] priority_pass seat=0 source= target=seat0
[3159] priority_pass seat=1 source= target=seat0
[3160] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[3161] stack_push seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[3162] triggers_ordered seat=3 source= target=seat0
[3163] priority_pass seat=0 source= target=seat0
[3164] priority_pass seat=1 source= target=seat0
[3165] priority_pass seat=2 source= target=seat0
[3166] stack_resolve seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[3167] conditional_effect_false seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[3168] trigger_evaluated seat=2 source=Lord Jyscal Guado
[3169] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[3170] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[3171] priority_pass seat=3 source= target=seat0
[3172] priority_pass seat=0 source= target=seat0
[3173] priority_pass seat=1 source= target=seat0
[3174] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[3175] pool_drain seat=3 source= amount=6 target=seat0
[3176] state seat=3 source= target=seat0
```

</details>

#### Violation 28

- **Game**: 181 (seed 1810042, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 60, Phase=ending Step=cleanup
- **Commanders**: Krark, the Thumbless, Myojin of Seeing Winds, Kotori, Pilot Prodigy, Fludge, Gunk Guardian
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 398, found 412) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 60, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 3177 events
  Seat 0 [alive]: life=11 library=77 hand=2 graveyard=7 exile=0 battlefield=13 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mithril Coat (P/T 0/0, dmg=0)
    - Sarpadian Empires, Vol. VII (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blasting Station (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Brittle Effigy (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=77 hand=7 graveyard=7 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mutavault (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Rhystic Deluge (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Edifice of Authority (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=12 library=45 hand=2 graveyard=20 exile=2 battlefield=31 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Transmogrifying Wand (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Kabira Takedown // Kabira Plateau (P/T 0/0, dmg=0) [T]
    - Undertow (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Kotori, Pilot Prodigy (P/T 2/4, dmg=0) [T]
    - Crumbling Vestige (P/T 0/0, dmg=0) [T]
    - Wu Admiral (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Lotus Guardian (P/T 4/4, dmg=0) [T]
    - Silent Attendant (P/T 0/2, dmg=0) [T]
    - Andúril, Narsil Reforged (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Skyscanner (P/T 1/1, dmg=0) [T]
    - Splendor Mare (P/T 3/3, dmg=0) [T]
    - Wingrattle Scarecrow (P/T 2/2, dmg=0) [T]
    - Lord Jyscal Guado (P/T 2/1, dmg=0) [T]
    - Thraben Valiant (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Sorcerer's Wand (P/T 0/0, dmg=0)
    - Vizier of Remedies (P/T 2/1, dmg=0)
    - token clue Token (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=23 library=77 hand=3 graveyard=12 exile=0 battlefield=7 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mindless Conscription (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3157] priority_pass seat=3 source= target=seat0
[3158] priority_pass seat=0 source= target=seat0
[3159] priority_pass seat=1 source= target=seat0
[3160] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[3161] stack_push seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[3162] triggers_ordered seat=3 source= target=seat0
[3163] priority_pass seat=0 source= target=seat0
[3164] priority_pass seat=1 source= target=seat0
[3165] priority_pass seat=2 source= target=seat0
[3166] stack_resolve seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[3167] conditional_effect_false seat=3 source=Growing Rites of Itlimoc // Itlimoc, Cradle of the Sun target=seat0
[3168] trigger_evaluated seat=2 source=Lord Jyscal Guado
[3169] stack_push seat=2 source=Lord Jyscal Guado target=seat0
[3170] triggered_ability seat=2 source=Lord Jyscal Guado target=seat0
[3171] priority_pass seat=3 source= target=seat0
[3172] priority_pass seat=0 source= target=seat0
[3173] priority_pass seat=1 source= target=seat0
[3174] stack_resolve seat=2 source=Lord Jyscal Guado target=seat0
[3175] pool_drain seat=3 source= amount=6 target=seat0
[3176] state seat=3 source= target=seat0
```

</details>

#### Violation 29

- **Game**: 490 (seed 4900042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 60, Phase=ending Step=cleanup
- **Commanders**: Donatello, Mutant Mechanic, The Weekly Princess, The Actualizer, Saskia the Unyielding
- **Message**: CardIdentity: card "Lich's Mastery" (ptr 0xc00d9be120) appears in both seat 2 exile and seat 2 battlefield

<details>
<summary>Game State</summary>

```
Turn 60, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2318 events
  Seat 0 [alive]: life=13 library=69 hand=2 graveyard=12 exile=1 battlefield=11 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - White Lotus Hideout (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cryptic Caves (P/T 0/0, dmg=0) [T]
    - Phyrexian Vault (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=12 library=74 hand=2 graveyard=9 exile=0 battlefield=11 cmdzone=1 mana=0
    - Racers' Ring (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blood Rites (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Boggart Shenanigans (P/T 0/0, dmg=0)
    - Seer's Lantern (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dread Statuary (P/T 0/0, dmg=0) [T]
    - Tarnished Citadel (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=4 library=77 hand=2 graveyard=5 exile=1 battlefield=16 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Armory of Iroas (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Chardalyn Dragon (P/T 4/4, dmg=0) [T]
    - Onyx Talisman (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Painted Bluffs (P/T 0/0, dmg=0) [T]
    - Gurmag Rakshasa (P/T 5/5, dmg=0) [T]
    - Keldon Megaliths (P/T 0/0, dmg=0) [T]
    - The Actualizer (P/T 3/3, dmg=0) [T]
    - Lich's Mastery (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=13 library=75 hand=3 graveyard=8 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Novel Nunchaku (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Gunky Runner (P/T 5/5, dmg=0) [T]
    - Wooded Ridgeline (P/T 0/0, dmg=0) [T]
    - Mudbutton Clanger (P/T 1/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Saskia the Unyielding (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2298] triggered_ability seat=2 source=Lich's Mastery target=seat0
[2299] priority_pass seat=3 source= target=seat0
[2300] priority_pass seat=0 source= target=seat0
[2301] priority_pass seat=1 source= target=seat0
[2302] stack_resolve seat=2 source=Lich's Mastery target=seat0
[2303] zone_change seat=2 source=Lich's Mastery
[2304] per_card_handler seat=0 source=Lich's Mastery target=seat0
[2305] priority_pass seat=0 source= target=seat0
[2306] priority_pass seat=1 source= target=seat0
[2307] priority_pass seat=2 source= target=seat0
[2308] phase_step seat=3 source= target=seat0
[2309] priority_pass seat=0 source= target=seat0
[2310] priority_pass seat=1 source= target=seat0
[2311] priority_pass seat=2 source= target=seat0
[2312] priority_pass seat=0 source= target=seat0
[2313] priority_pass seat=1 source= target=seat0
[2314] priority_pass seat=2 source= target=seat0
[2315] stack_resolve seat=3 source=Saskia the Unyielding target=seat0
[2316] enter_battlefield seat=3 source=Saskia the Unyielding target=seat0
[2317] state seat=3 source= target=seat0
```

</details>

#### Violation 30

- **Game**: 490 (seed 4900042, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 60, Phase=ending Step=cleanup
- **Commanders**: Donatello, Mutant Mechanic, The Weekly Princess, The Actualizer, Saskia the Unyielding
- **Message**: CardIdentity: card "Lich's Mastery" (ptr 0xc00d9be120) appears in both seat 2 exile and seat 2 battlefield

<details>
<summary>Game State</summary>

```
Turn 60, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2318 events
  Seat 0 [alive]: life=13 library=69 hand=2 graveyard=12 exile=1 battlefield=11 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - White Lotus Hideout (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Cryptic Caves (P/T 0/0, dmg=0) [T]
    - Phyrexian Vault (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=12 library=74 hand=2 graveyard=9 exile=0 battlefield=11 cmdzone=1 mana=0
    - Racers' Ring (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blood Rites (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Boggart Shenanigans (P/T 0/0, dmg=0)
    - Seer's Lantern (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dread Statuary (P/T 0/0, dmg=0) [T]
    - Tarnished Citadel (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=4 library=77 hand=2 graveyard=5 exile=1 battlefield=16 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Armory of Iroas (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Chardalyn Dragon (P/T 4/4, dmg=0) [T]
    - Onyx Talisman (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Painted Bluffs (P/T 0/0, dmg=0) [T]
    - Gurmag Rakshasa (P/T 5/5, dmg=0) [T]
    - Keldon Megaliths (P/T 0/0, dmg=0) [T]
    - The Actualizer (P/T 3/3, dmg=0) [T]
    - Lich's Mastery (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=13 library=75 hand=3 graveyard=8 exile=0 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Novel Nunchaku (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Gunky Runner (P/T 5/5, dmg=0) [T]
    - Wooded Ridgeline (P/T 0/0, dmg=0) [T]
    - Mudbutton Clanger (P/T 1/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Saskia the Unyielding (P/T 3/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2298] triggered_ability seat=2 source=Lich's Mastery target=seat0
[2299] priority_pass seat=3 source= target=seat0
[2300] priority_pass seat=0 source= target=seat0
[2301] priority_pass seat=1 source= target=seat0
[2302] stack_resolve seat=2 source=Lich's Mastery target=seat0
[2303] zone_change seat=2 source=Lich's Mastery
[2304] per_card_handler seat=0 source=Lich's Mastery target=seat0
[2305] priority_pass seat=0 source= target=seat0
[2306] priority_pass seat=1 source= target=seat0
[2307] priority_pass seat=2 source= target=seat0
[2308] phase_step seat=3 source= target=seat0
[2309] priority_pass seat=0 source= target=seat0
[2310] priority_pass seat=1 source= target=seat0
[2311] priority_pass seat=2 source= target=seat0
[2312] priority_pass seat=0 source= target=seat0
[2313] priority_pass seat=1 source= target=seat0
[2314] priority_pass seat=2 source= target=seat0
[2315] stack_resolve seat=3 source=Saskia the Unyielding target=seat0
[2316] enter_battlefield seat=3 source=Saskia the Unyielding target=seat0
[2317] state seat=3 source= target=seat0
```

</details>

*... and 1225 more violations not shown.*

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
| 2 | Revel of the Fallen God | 1 | 2 | 0.33 |
| 3 | Baleful Strix | 2 | 4 | 0.33 |
| 4 | Jorubai Murk Lurker | 1 | 2 | 0.33 |
| 5 | Finest Hour | 1 | 2 | 0.33 |
| 6 | Aura Mutation | 1 | 2 | 0.33 |
| 7 | Kheru Dreadmaw | 1 | 2 | 0.33 |
| 8 | Voracious Cobra | 1 | 2 | 0.33 |
| 9 | Horrid Shadowspinner | 1 | 2 | 0.33 |
| 10 | Ancient Spider | 1 | 2 | 0.33 |

## Verdict: ISSUES FOUND

**1261 total issues** across 5000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 1255 invariant violations in chaos games
- 0 crashes in nightmare boards
- 6 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
