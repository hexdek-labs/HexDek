# Chaos Gauntlet Report

Generated: 2026-05-24T15:32:09-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36656 cards |
| Legendary Creatures | 3433 |
| Total Games | 470 |
| Seed | 1337 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 5.693s |
| Throughput | 83 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 10 (in 1 games) |
| Clean Games | 469 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 730ms |
| Throughput | 13697 boards/sec |
| Crashes | 0 |
| Invariant Violations | 0 |
| Clean Boards | 10000 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| SBACompleteness | 10 |

### Violation Details (up to 5 per invariant kind, 5 shown)

#### Violation 1

- **Game**: 465 (seed 4651338, perm 0)
- **Invariant**: SBACompleteness
- **Turn**: 56, Phase=ending Step=cleanup
- **Commanders**: Leonardo, Sewer Samurai, Dina, Essence Brewer, Shilgengar, Sire of Famine, Sliver Gravemother
- **Message**: seat 0 has life=0, Lost=false, no loss-prevention — SBA 704.5a missed

<details>
<summary>Game State</summary>

```
Turn 56, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 4351 events
  Seat 0 [alive]: life=0 library=76 hand=0 graveyard=8 exile=1 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mister Gutsy (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Leonardo, Sewer Samurai (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mishra's Foundry (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=13 library=76 hand=7 graveyard=10 exile=0 battlefield=5 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Scalding Tongs (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fountainport (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=5 library=77 hand=7 graveyard=6 exile=1 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Curse of Silence (P/T 0/0, dmg=0)
    - Golden Guardian // Gold-Forge Garrison (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - City of Shadows (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=4 library=75 hand=4 graveyard=1 exile=0 battlefield=16 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Vampire Bats (P/T 0/1, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Boot Nipper (P/T 2/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Nim Devourer (P/T 4/1, dmg=0) [T]
    - Screams of the Damned (P/T 0/0, dmg=0)
    - Journey to Eternity // Atzal, Cave of Eternity (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Smoldering Spires (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Clifftop Retreat (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Geothermal Bog (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4331] priority_pass seat=0 source= target=seat0
[4332] priority_pass seat=1 source= target=seat0
[4333] priority_pass seat=2 source= target=seat0
[4334] priority_pass seat=0 source= target=seat0
[4335] priority_pass seat=1 source= target=seat0
[4336] priority_pass seat=2 source= target=seat0
[4337] priority_pass seat=0 source= target=seat0
[4338] priority_pass seat=1 source= target=seat0
[4339] priority_pass seat=2 source= target=seat0
[4340] priority_pass seat=0 source= target=seat0
[4341] priority_pass seat=1 source= target=seat0
[4342] priority_pass seat=2 source= target=seat0
[4343] loop_shortcut seat=0 source=no_op_loop target=seat0
[4344] phase_step seat=3 source= target=seat0
[4345] declare_attackers seat=3 source= target=seat0
[4346] blockers seat=0 source= target=seat0
[4347] damage seat=3 source=Boot Nipper amount=2 target=seat0
[4348] damage seat=3 source=Nim Devourer amount=4 target=seat0
[4349] phase_step seat=3 source= target=seat0
[4350] state seat=3 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 465 (seed 4651338, perm 0)
- **Invariant**: SBACompleteness
- **Turn**: 56, Phase=ending Step=cleanup
- **Commanders**: Leonardo, Sewer Samurai, Dina, Essence Brewer, Shilgengar, Sire of Famine, Sliver Gravemother
- **Message**: seat 0 has life=0, Lost=false, no loss-prevention — SBA 704.5a missed

<details>
<summary>Game State</summary>

```
Turn 56, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 4351 events
  Seat 0 [alive]: life=0 library=76 hand=0 graveyard=8 exile=1 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mister Gutsy (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Leonardo, Sewer Samurai (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mishra's Foundry (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=13 library=76 hand=7 graveyard=10 exile=0 battlefield=5 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Scalding Tongs (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fountainport (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=5 library=77 hand=7 graveyard=6 exile=1 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Curse of Silence (P/T 0/0, dmg=0)
    - Golden Guardian // Gold-Forge Garrison (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - City of Shadows (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=4 library=75 hand=4 graveyard=1 exile=0 battlefield=16 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Vampire Bats (P/T 0/1, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Boot Nipper (P/T 2/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Nim Devourer (P/T 4/1, dmg=0) [T]
    - Screams of the Damned (P/T 0/0, dmg=0)
    - Journey to Eternity // Atzal, Cave of Eternity (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Smoldering Spires (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Clifftop Retreat (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Geothermal Bog (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4331] priority_pass seat=0 source= target=seat0
[4332] priority_pass seat=1 source= target=seat0
[4333] priority_pass seat=2 source= target=seat0
[4334] priority_pass seat=0 source= target=seat0
[4335] priority_pass seat=1 source= target=seat0
[4336] priority_pass seat=2 source= target=seat0
[4337] priority_pass seat=0 source= target=seat0
[4338] priority_pass seat=1 source= target=seat0
[4339] priority_pass seat=2 source= target=seat0
[4340] priority_pass seat=0 source= target=seat0
[4341] priority_pass seat=1 source= target=seat0
[4342] priority_pass seat=2 source= target=seat0
[4343] loop_shortcut seat=0 source=no_op_loop target=seat0
[4344] phase_step seat=3 source= target=seat0
[4345] declare_attackers seat=3 source= target=seat0
[4346] blockers seat=0 source= target=seat0
[4347] damage seat=3 source=Boot Nipper amount=2 target=seat0
[4348] damage seat=3 source=Nim Devourer amount=4 target=seat0
[4349] phase_step seat=3 source= target=seat0
[4350] state seat=3 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 465 (seed 4651338, perm 0)
- **Invariant**: SBACompleteness
- **Turn**: 57, Phase=ending Step=cleanup
- **Commanders**: Leonardo, Sewer Samurai, Dina, Essence Brewer, Shilgengar, Sire of Famine, Sliver Gravemother
- **Message**: seat 0 has life=0, Lost=false, no loss-prevention — SBA 704.5a missed

<details>
<summary>Game State</summary>

```
Turn 57, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 4574 events
  Seat 0 [alive]: life=0 library=75 hand=0 graveyard=8 exile=1 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mister Gutsy (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Leonardo, Sewer Samurai (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mishra's Foundry (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=13 library=76 hand=7 graveyard=10 exile=0 battlefield=5 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Scalding Tongs (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fountainport (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=5 library=77 hand=7 graveyard=6 exile=1 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Curse of Silence (P/T 0/0, dmg=0)
    - Golden Guardian // Gold-Forge Garrison (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - City of Shadows (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=3 library=75 hand=4 graveyard=1 exile=0 battlefield=16 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Vampire Bats (P/T 0/1, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Boot Nipper (P/T 2/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Nim Devourer (P/T 4/1, dmg=0) [T]
    - Screams of the Damned (P/T 0/0, dmg=0)
    - Journey to Eternity // Atzal, Cave of Eternity (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Smoldering Spires (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Clifftop Retreat (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Geothermal Bog (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4554] priority_pass seat=3 source= target=seat0
[4555] priority_pass seat=1 source= target=seat0
[4556] priority_pass seat=2 source= target=seat0
[4557] priority_pass seat=3 source= target=seat0
[4558] priority_pass seat=1 source= target=seat0
[4559] priority_pass seat=2 source= target=seat0
[4560] priority_pass seat=3 source= target=seat0
[4561] priority_pass seat=1 source= target=seat0
[4562] priority_pass seat=2 source= target=seat0
[4563] priority_pass seat=3 source= target=seat0
[4564] loop_shortcut seat=0 source=no_op_loop target=seat0
[4565] phase_step seat=0 source= target=seat0
[4566] declare_attackers seat=0 source= target=seat0
[4567] blockers seat=3 source= target=seat0
[4568] damage seat=0 source=Leonardo, Sewer Samurai amount=1 target=seat3
[4569] damage seat=0 source=Mister Gutsy amount=1 target=seat3
[4570] damage seat=0 source=Leonardo, Sewer Samurai amount=1 target=seat3
[4571] phase_step seat=0 source= target=seat0
[4572] damage_wears_off seat=3 source=Vampire Bats amount=2 target=seat0
[4573] state seat=0 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 465 (seed 4651338, perm 0)
- **Invariant**: SBACompleteness
- **Turn**: 57, Phase=ending Step=cleanup
- **Commanders**: Leonardo, Sewer Samurai, Dina, Essence Brewer, Shilgengar, Sire of Famine, Sliver Gravemother
- **Message**: seat 0 has life=0, Lost=false, no loss-prevention — SBA 704.5a missed

<details>
<summary>Game State</summary>

```
Turn 57, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 4574 events
  Seat 0 [alive]: life=0 library=75 hand=0 graveyard=8 exile=1 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mister Gutsy (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Leonardo, Sewer Samurai (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mishra's Foundry (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=13 library=76 hand=7 graveyard=10 exile=0 battlefield=5 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Scalding Tongs (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fountainport (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=5 library=77 hand=7 graveyard=6 exile=1 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Curse of Silence (P/T 0/0, dmg=0)
    - Golden Guardian // Gold-Forge Garrison (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - City of Shadows (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=3 library=75 hand=4 graveyard=1 exile=0 battlefield=16 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Vampire Bats (P/T 0/1, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Boot Nipper (P/T 2/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Nim Devourer (P/T 4/1, dmg=0) [T]
    - Screams of the Damned (P/T 0/0, dmg=0)
    - Journey to Eternity // Atzal, Cave of Eternity (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Smoldering Spires (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Clifftop Retreat (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Geothermal Bog (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4554] priority_pass seat=3 source= target=seat0
[4555] priority_pass seat=1 source= target=seat0
[4556] priority_pass seat=2 source= target=seat0
[4557] priority_pass seat=3 source= target=seat0
[4558] priority_pass seat=1 source= target=seat0
[4559] priority_pass seat=2 source= target=seat0
[4560] priority_pass seat=3 source= target=seat0
[4561] priority_pass seat=1 source= target=seat0
[4562] priority_pass seat=2 source= target=seat0
[4563] priority_pass seat=3 source= target=seat0
[4564] loop_shortcut seat=0 source=no_op_loop target=seat0
[4565] phase_step seat=0 source= target=seat0
[4566] declare_attackers seat=0 source= target=seat0
[4567] blockers seat=3 source= target=seat0
[4568] damage seat=0 source=Leonardo, Sewer Samurai amount=1 target=seat3
[4569] damage seat=0 source=Mister Gutsy amount=1 target=seat3
[4570] damage seat=0 source=Leonardo, Sewer Samurai amount=1 target=seat3
[4571] phase_step seat=0 source= target=seat0
[4572] damage_wears_off seat=3 source=Vampire Bats amount=2 target=seat0
[4573] state seat=0 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 465 (seed 4651338, perm 0)
- **Invariant**: SBACompleteness
- **Turn**: 58, Phase=ending Step=cleanup
- **Commanders**: Leonardo, Sewer Samurai, Dina, Essence Brewer, Shilgengar, Sire of Famine, Sliver Gravemother
- **Message**: seat 0 has life=0, Lost=false, no loss-prevention — SBA 704.5a missed

<details>
<summary>Game State</summary>

```
Turn 58, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 4590 events
  Seat 0 [alive]: life=0 library=75 hand=0 graveyard=8 exile=1 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mister Gutsy (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Leonardo, Sewer Samurai (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Ghost Quarter (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mishra's Foundry (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=13 library=75 hand=7 graveyard=11 exile=0 battlefield=5 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Scalding Tongs (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fountainport (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=5 library=77 hand=7 graveyard=6 exile=1 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Curse of Silence (P/T 0/0, dmg=0)
    - Golden Guardian // Gold-Forge Garrison (P/T 4/4, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - City of Shadows (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=3 library=75 hand=4 graveyard=1 exile=0 battlefield=16 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Vampire Bats (P/T 0/1, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Boot Nipper (P/T 2/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Nim Devourer (P/T 4/1, dmg=0) [T]
    - Screams of the Damned (P/T 0/0, dmg=0)
    - Journey to Eternity // Atzal, Cave of Eternity (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Smoldering Spires (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Clifftop Retreat (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Geothermal Bog (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[4570] damage seat=0 source=Leonardo, Sewer Samurai amount=1 target=seat3
[4571] phase_step seat=0 source= target=seat0
[4572] damage_wears_off seat=3 source=Vampire Bats amount=2 target=seat0
[4573] state seat=0 source= target=seat0
[4574] turn_start seat=1 source= target=seat0
[4575] untap_done seat=1 source=Forest target=seat0
[4576] untap_done seat=1 source=Swamp target=seat0
[4577] untap_done seat=1 source=Forest target=seat0
[4578] untap_done seat=1 source=Fountainport target=seat0
[4579] add_mana seat=1 source=Forest amount=1 target=seat0
[4580] add_mana seat=1 source=Swamp amount=1 target=seat0
[4581] add_mana seat=1 source=Forest amount=1 target=seat0
[4582] add_mana seat=1 source=Fountainport amount=1 target=seat0
[4583] draw seat=1 source=Clinging Darkness amount=1 target=seat0
[4584] phase_step seat=1 source= target=seat0
[4585] phase_step seat=1 source= target=seat0
[4586] pool_drain seat=1 source= amount=4 target=seat0
[4587] zone_change seat=1 source=Grievous Wound
[4588] discard seat=1 source=Grievous Wound target=seat0
[4589] state seat=1 source= target=seat0
```

</details>

*... and 5 more violations not shown.*

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Serene Offering | 1 | 2 | 0.33 |
| 2 | Take Up the Shield | 1 | 2 | 0.33 |
| 3 | Quarrel's End | 1 | 2 | 0.33 |
| 4 | Goreclaw, Terror of Qal Sisma | 1 | 2 | 0.33 |
| 5 | A-Death-Priest of Myrkul | 1 | 2 | 0.33 |
| 6 | Valiant Guard | 1 | 2 | 0.33 |
| 7 | Grim Tutor | 1 | 2 | 0.33 |
| 8 | Boot Nipper | 1 | 2 | 0.33 |
| 9 | Oswald Fiddlebender | 1 | 2 | 0.33 |
| 10 | Thought Gorger | 1 | 2 | 0.33 |

## Verdict: ISSUES FOUND

**10 total issues** across 470 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 10 invariant violations in chaos games
- 0 crashes in nightmare boards
- 0 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
