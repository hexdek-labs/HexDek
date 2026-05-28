# Chaos Gauntlet Report

Generated: 2026-05-27T21:36:43-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36656 cards |
| Legendary Creatures | 3433 |
| Total Games | 10000 |
| Seed | 42 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 0 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 1m37.268s |
| Throughput | 103 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 110088 (in 2203 games) |
| Clean Games | 7797 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 0s |
| Throughput | 0 boards/sec |
| Crashes | 0 |
| Invariant Violations | 0 |
| Clean Boards | 0 |

## Invariant Violations (Chaos Games)

### By Invariant

| Invariant | Count |
|-----------|-------|
| CardIdentity | 78494 |
| ZoneConservation | 21374 |
| ExileLinkageIntegrity | 10220 |

### Violation Details (up to 30 per invariant kind, 90 shown)

#### Violation 1

- **Game**: 7 (seed 70043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 22, Phase=ending Step=cleanup
- **Commanders**: Kalitas, Bloodchief of Ghet, The Gitrog, Ravenous Ride, Greven, Predator Captain, Gandalf of the Secret Fire
- **Message**: CardIdentity: card "Kalitas, Bloodchief of Ghet" (ptr 0x36fdb0668000) appears in both seat 0 command_zone and seat 0 battlefield

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 842 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=3 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kalitas, Bloodchief of Ghet (P/T 5/5, dmg=0)
  Seat 1 [alive]: life=38 library=86 hand=4 graveyard=2 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wriggling Grub (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hostage Taker (P/T 2/3, dmg=0)
  Seat 2 [alive]: life=21 library=74 hand=7 graveyard=7 exile=0 battlefield=8 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Arguel's Blood Fast // Temple of Aclazotz (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Agate-Blade Assassin (P/T 1/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Cursebound Witch (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=39 library=84 hand=7 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Serum Sovereign (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[822] destroy seat=3 source=Ty Lee, Chi Blocker
[823] sba_704_5g seat=3 source=Ty Lee, Chi Blocker
[824] zone_change seat=3 source=Ty Lee, Chi Blocker
[825] trigger_evaluated seat=1 source=Hostage Taker
[826] stack_push seat=1 source=Hostage Taker target=seat0
[827] triggered_ability seat=1 source=Hostage Taker target=seat0
[828] priority_pass seat=3 source= target=seat0
[829] priority_pass seat=0 source= target=seat0
[830] priority_pass seat=2 source= target=seat0
[831] stack_resolve seat=1 source=Hostage Taker target=seat0
[832] zone_change seat=0 source=Kalitas, Bloodchief of Ghet
[833] exile_linked_returned seat=0 source=Hostage Taker amount=1 target=seat0
[834] per_card_handler seat=0 source=Hostage Taker target=seat0
[835] sba_cycle_complete seat=-1 source=
[836] phase_step seat=3 source= target=seat0
[837] zone_change seat=3 source=Lava-Field Overlord
[838] discard seat=3 source=Lava-Field Overlord target=seat0
[839] damage_wears_off seat=1 source=Hostage Taker amount=2 target=seat0
[840] cleanup_loop seat=3 source= target=seat0
[841] state seat=3 source= target=seat0
```

</details>

#### Violation 2

- **Game**: 7 (seed 70043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 42, Phase=ending Step=cleanup
- **Commanders**: Kalitas, Bloodchief of Ghet, The Gitrog, Ravenous Ride, Greven, Predator Captain, Gandalf of the Secret Fire
- **Message**: CardIdentity: card "Kalitas, Bloodchief of Ghet" (ptr 0x36fdb0668000) appears in both seat 0 command_zone and seat 3 battlefield

<details>
<summary>Game State</summary>

```
Turn 42, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2800 events
  Seat 0 [alive]: life=30 library=80 hand=5 graveyard=0 exile=4 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lotus Field (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=30 library=79 hand=3 graveyard=0 exile=8 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Planar Portal (P/T 0/0, dmg=0) [T]
    - creature token black and green worm Token (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=1 library=59 hand=6 graveyard=1 exile=17 battlefield=14 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Arguel's Blood Fast // Temple of Aclazotz (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tocasia's Dig Site (P/T 0/0, dmg=0) [T]
    - Zodiac Goat (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - The First Eruption (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Greven, Predator Captain (P/T 5/5, dmg=0)
  Seat 3 [alive]: life=33 library=79 hand=4 graveyard=0 exile=7 battlefield=10 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Origin of the Hidden Ones (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Rubblebelt Boar (P/T 3/3, dmg=0) [T]
    - Rustic Clachan (P/T 0/0, dmg=0) [T]
    - Yuffie, Materia Hunter (P/T 3/3, dmg=0) [T]
    - Kalitas, Bloodchief of Ghet (P/T 5/5, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2780] damage seat=3 source=Rubblebelt Boar amount=3 target=seat1
[2781] trigger_evaluated seat=2 source=Greven, Predator Captain
[2782] stack_push seat=2 source=Greven, Predator Captain target=seat0
[2783] triggered_ability seat=2 source=Greven, Predator Captain target=seat0
[2784] priority_pass seat=3 source= target=seat0
[2785] priority_pass seat=0 source= target=seat0
[2786] priority_pass seat=1 source= target=seat0
[2787] stack_resolve seat=2 source=Greven, Predator Captain target=seat0
[2788] speed_advance seat=3 source= amount=2 target=seat0
[2789] damage seat=3 source=Yuffie, Materia Hunter amount=3 target=seat1
[2790] trigger_evaluated seat=2 source=Greven, Predator Captain
[2791] stack_push seat=2 source=Greven, Predator Captain target=seat0
[2792] triggered_ability seat=2 source=Greven, Predator Captain target=seat0
[2793] priority_pass seat=3 source= target=seat0
[2794] priority_pass seat=0 source= target=seat0
[2795] priority_pass seat=1 source= target=seat0
[2796] stack_resolve seat=2 source=Greven, Predator Captain target=seat0
[2797] phase_step seat=3 source= target=seat0
[2798] pool_drain seat=3 source= amount=3 target=seat0
[2799] state seat=3 source= target=seat0
```

</details>

#### Violation 3

- **Game**: 7 (seed 70043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 43, Phase=ending Step=cleanup
- **Commanders**: Kalitas, Bloodchief of Ghet, The Gitrog, Ravenous Ride, Greven, Predator Captain, Gandalf of the Secret Fire
- **Message**: CardIdentity: card "Kalitas, Bloodchief of Ghet" (ptr 0x36fdb0668000) appears in both seat 0 battlefield and seat 3 battlefield

<details>
<summary>Game State</summary>

```
Turn 43, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2851 events
  Seat 0 [alive]: life=30 library=79 hand=5 graveyard=1 exile=4 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lotus Field (P/T 0/0, dmg=0) [T]
    - Kalitas, Bloodchief of Ghet (P/T 5/5, dmg=0)
  Seat 1 [alive]: life=30 library=79 hand=3 graveyard=0 exile=8 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Planar Portal (P/T 0/0, dmg=0) [T]
    - creature token black and green worm Token (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=1 library=59 hand=6 graveyard=1 exile=17 battlefield=14 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Arguel's Blood Fast // Temple of Aclazotz (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tocasia's Dig Site (P/T 0/0, dmg=0) [T]
    - Zodiac Goat (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - The First Eruption (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Greven, Predator Captain (P/T 5/5, dmg=0)
  Seat 3 [alive]: life=33 library=79 hand=4 graveyard=0 exile=7 battlefield=10 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Origin of the Hidden Ones (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Rubblebelt Boar (P/T 3/3, dmg=0) [T]
    - Rustic Clachan (P/T 0/0, dmg=0) [T]
    - Yuffie, Materia Hunter (P/T 3/3, dmg=0) [T]
    - Kalitas, Bloodchief of Ghet (P/T 5/5, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2831] pay_mana seat=0 source=Kalitas, Bloodchief of Ghet amount=7 target=seat0
[2832] cast seat=0 source=Kalitas, Bloodchief of Ghet amount=7 target=seat0
[2833] commander_cast_from_command_zone seat=0 source=Kalitas, Bloodchief of Ghet amount=7 target=seat0
[2834] stack_push seat=0 source=Kalitas, Bloodchief of Ghet target=seat0
[2835] cast seat=0 source=Sarkhan's Resolve // Sarkhan's Resolve target=seat0
[2836] stack_push seat=0 source=Sarkhan's Resolve // Sarkhan's Resolve target=seat0
[2837] priority_pass seat=1 source= target=seat0
[2838] priority_pass seat=2 source= target=seat0
[2839] priority_pass seat=3 source= target=seat0
[2840] stack_resolve seat=0 source=Sarkhan's Resolve // Sarkhan's Resolve target=seat0
[2841] zone_change seat=0 source=Sarkhan's Resolve // Sarkhan's Resolve
[2842] resolve seat=0 source=Sarkhan's Resolve // Sarkhan's Resolve target=seat0
[2843] priority_pass seat=1 source= target=seat0
[2844] priority_pass seat=2 source= target=seat0
[2845] priority_pass seat=3 source= target=seat0
[2846] stack_resolve seat=0 source=Kalitas, Bloodchief of Ghet target=seat0
[2847] enter_battlefield seat=0 source=Kalitas, Bloodchief of Ghet target=seat0
[2848] phase_step seat=0 source= target=seat0
[2849] phase_step seat=0 source= target=seat0
[2850] state seat=0 source= target=seat0
```

</details>

#### Violation 4

- **Game**: 6 (seed 60043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 24, Phase=ending Step=cleanup
- **Commanders**: Oyobi, Who Split the Heavens, Invasion of Ikoria // Zilortha, Apex of Ikoria, Phenax, God of Deception, Mondrak, Glory Dominus
- **Message**: CardIdentity: card "Morph" (ptr 0x36fdb231e900) appears in both seat 3 exile and seat 3 battlefield

<details>
<summary>Game State</summary>

```
Turn 24, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 921 events
  Seat 0 [alive]: life=39 library=85 hand=0 graveyard=5 exile=0 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urborg, Tomb of Yawgmoth (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - A-Lantern of Revealing (P/T 0/0, dmg=0) [T]
    - Branch of Vitu-Ghazi (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Serum Powder (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=40 library=85 hand=0 graveyard=4 exile=0 battlefield=10 cmdzone=0 mana=0
    - A-Base Camp (P/T 0/0, dmg=0) [T]
    - The Mycosynth Gardens (P/T 0/0, dmg=0) [T]
    - Invasion of Ikoria // Zilortha, Apex of Ikoria (P/T 8/8, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Invigorating Boon (P/T 0/0, dmg=0)
    - Somberwald Dryad (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Argothian Treefolk (P/T 3/5, dmg=0) [T]
    - Thinking Cap (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=38 library=84 hand=5 graveyard=2 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Phenax, God of Deception (P/T 4/7, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=42 library=74 hand=3 graveyard=12 exile=2 battlefield=6 cmdzone=1 mana=0
    - Urza's Power Plant (P/T 0/0, dmg=0) [T]
    - Mana Confluence (P/T 0/0, dmg=0) [T]
    - Ashnod's Battle Gear (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Morph (P/T 2/2, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[901] zone_change seat=3 source=Plains
[902] zone_change seat=3 source=Lumbering Battlement
[903] possibility_storm seat=3 source=Possibility Storm target=seat0
[904] zone_cast_grant_registered seat=3 source=Possibility Storm target=seat0
[905] zone_change seat=3 source=Plains
[906] zone_change seat=3 source=Waildrifter // Waildrifter
[907] zone_change seat=3 source=Plains
[908] zone_change seat=3 source=Pin Trading
[909] zone_change seat=3 source=Plains
[910] stack_push seat=3 source=Morph target=seat0
[911] priority_pass seat=0 source= target=seat0
[912] priority_pass seat=1 source= target=seat0
[913] priority_pass seat=2 source= target=seat0
[914] stack_resolve seat=3 source=Morph target=seat0
[915] enter_battlefield seat=3 source=Morph target=seat0
[916] phase_step seat=3 source= target=seat0
[917] phase_step seat=3 source= target=seat0
[918] pool_drain seat=3 source= amount=2 target=seat0
[919] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[920] state seat=3 source= target=seat0
```

</details>

#### Violation 5

- **Game**: 6 (seed 60043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 25, Phase=ending Step=cleanup
- **Commanders**: Oyobi, Who Split the Heavens, Invasion of Ikoria // Zilortha, Apex of Ikoria, Phenax, God of Deception, Mondrak, Glory Dominus
- **Message**: CardIdentity: card "Garruk's Uprising // Garruk's Uprising" (ptr 0x36fdb086b320) appears in both seat 0 graveyard and seat 0 exile

<details>
<summary>Game State</summary>

```
Turn 25, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 970 events
  Seat 0 [alive]: life=39 library=83 hand=0 graveyard=6 exile=2 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urborg, Tomb of Yawgmoth (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - A-Lantern of Revealing (P/T 0/0, dmg=0) [T]
    - Branch of Vitu-Ghazi (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Serum Powder (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=40 library=85 hand=0 graveyard=4 exile=0 battlefield=10 cmdzone=0 mana=0
    - A-Base Camp (P/T 0/0, dmg=0) [T]
    - The Mycosynth Gardens (P/T 0/0, dmg=0) [T]
    - Invasion of Ikoria // Zilortha, Apex of Ikoria (P/T 8/8, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Invigorating Boon (P/T 0/0, dmg=0)
    - Somberwald Dryad (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Argothian Treefolk (P/T 3/5, dmg=0) [T]
    - Thinking Cap (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=38 library=84 hand=5 graveyard=2 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Phenax, God of Deception (P/T 4/7, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=42 library=74 hand=3 graveyard=12 exile=2 battlefield=6 cmdzone=1 mana=0
    - Urza's Power Plant (P/T 0/0, dmg=0) [T]
    - Mana Confluence (P/T 0/0, dmg=0) [T]
    - Ashnod's Battle Gear (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Morph (P/T 2/2, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[950] priority_pass seat=0 source= target=seat0
[951] priority_pass seat=1 source= target=seat0
[952] priority_pass seat=3 source= target=seat0
[953] stack_resolve seat=2 source=Possibility Storm target=seat0
[954] zone_change seat=0 source=Garruk's Uprising // Garruk's Uprising
[955] zone_change seat=0 source=Bloodsoaked Reveler // Bloodsoaked Reveler
[956] possibility_storm seat=0 source=Possibility Storm target=seat0
[957] zone_cast_grant_registered seat=0 source=Possibility Storm target=seat0
[958] stack_push seat=0 source=Garruk's Uprising // Garruk's Uprising target=seat0
[959] priority_pass seat=1 source= target=seat0
[960] priority_pass seat=2 source= target=seat0
[961] priority_pass seat=3 source= target=seat0
[962] stack_resolve seat=0 source=Garruk's Uprising // Garruk's Uprising target=seat0
[963] zone_change seat=0 source=Garruk's Uprising // Garruk's Uprising
[964] resolve seat=0 source=Garruk's Uprising // Garruk's Uprising target=seat0
[965] phase_step seat=0 source= target=seat0
[966] phase_step seat=0 source= target=seat0
[967] pool_drain seat=0 source= amount=3 target=seat0
[968] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[969] state seat=0 source= target=seat0
```

</details>

#### Violation 6

- **Game**: 5 (seed 50043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 49, Phase=ending Step=cleanup
- **Commanders**: Grimgrin, Corpse-Born, The Grand Goatnapper, Farmer Cotton, Rasaad yn Bashir
- **Message**: CardIdentity: card "Nightmare's Thirst" (ptr 0x36fdb05f6900) appears in both seat 0 graveyard and seat 0 exile

<details>
<summary>Game State</summary>

```
Turn 49, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2568 events
  Seat 0 [alive]: life=12 library=39 hand=3 graveyard=10 exile=46 battlefield=2 cmdzone=1 mana=0
    - Library of Alexandria (P/T 0/0, dmg=0) [T]
    - Phyrexian Lens (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=7 library=75 hand=3 graveyard=5 exile=3 battlefield=12 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Grand Goatnapper (P/T 3/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Skullslither Worm (P/T 3/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Bojuka Bog (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Smokespew Invoker (P/T 3/1, dmg=0) [T]
    - Spawning Pool (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=28 library=75 hand=1 graveyard=13 exile=3 battlefield=5 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - White Lotus Hideout (P/T 0/0, dmg=0) [T]
    - Barbary Apes (P/T 2/2, dmg=0) [T]
    - Rinoa, Angel Wing (P/T 2/4, dmg=0) [T]
    - Conclave's Blessing (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=15 library=76 hand=0 graveyard=9 exile=3 battlefield=12 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Progenitor Exarch (P/T 1/2, dmg=0) [T]
    - token incubator artifact Token (P/T 3/3, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Rasaad yn Bashir (P/T 0/3, dmg=0)
    - Slith Ascendant (P/T 7/7, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Nyx Lotus (P/T 0/0, dmg=0) [T]
    - Adventurer's Inn (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Knowledge Pool (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2548] cast seat=0 source=Phyrexian Lens amount=3 target=seat0
[2549] trigger_evaluated seat=3 source=Knowledge Pool
[2550] stack_push seat=3 source=Knowledge Pool target=seat0
[2551] triggered_ability seat=3 source=Knowledge Pool target=seat0
[2552] priority_pass seat=0 source= target=seat0
[2553] priority_pass seat=1 source= target=seat0
[2554] priority_pass seat=2 source= target=seat0
[2555] stack_resolve seat=3 source=Knowledge Pool target=seat0
[2556] zone_change seat=0 source=Phyrexian Lens
[2557] zone_cast_grant_registered seat=0 source=Knowledge Pool target=seat0
[2558] per_card_handler seat=0 source=Knowledge Pool target=seat0
[2559] stack_push seat=0 source=Phyrexian Lens target=seat0
[2560] priority_pass seat=1 source= target=seat0
[2561] priority_pass seat=2 source= target=seat0
[2562] priority_pass seat=3 source= target=seat0
[2563] stack_resolve seat=0 source=Phyrexian Lens target=seat0
[2564] enter_battlefield seat=0 source=Phyrexian Lens target=seat0
[2565] phase_step seat=0 source= target=seat0
[2566] phase_step seat=0 source= target=seat0
[2567] state seat=0 source= target=seat0
```

</details>

#### Violation 7

- **Game**: 16 (seed 160043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 21, Phase=ending Step=cleanup
- **Commanders**: Palladia-Mors, Sita Varma, Masked Racer, Jason Bright, Glowing Prophet, Spider-UK
- **Message**: CardIdentity: card "Brian Kibler Decklist" (ptr 0x36fdd250bb00) appears in both seat 2 graveyard and seat 2 exile

<details>
<summary>Game State</summary>

```
Turn 21, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 651 events
  Seat 0 [alive]: life=40 library=85 hand=3 graveyard=3 exile=0 battlefield=6 cmdzone=1 mana=0
    - Drownyard Temple (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Generous Plunderer (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=40 library=86 hand=7 graveyard=1 exile=0 battlefield=4 cmdzone=1 mana=0
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Darksteel Pendant (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - War Tax (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=40 library=84 hand=1 graveyard=5 exile=2 battlefield=7 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mogg Cannon (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=84 hand=5 graveyard=1 exile=0 battlefield=6 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - The Monumental Facade (P/T 0/0, dmg=0) [T]
    - Amaranthine Wall (P/T 0/6, dmg=0)
    - Forge of Heroes (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[631] stack_push seat=2 source=Brian Kibler Decklist target=seat0
[632] priority_pass seat=3 source= target=seat0
[633] priority_pass seat=0 source= target=seat0
[634] priority_pass seat=1 source= target=seat0
[635] stack_resolve seat=2 source=Brian Kibler Decklist target=seat0
[636] zone_change seat=2 source=Brian Kibler Decklist
[637] resolve seat=2 source=Brian Kibler Decklist target=seat0
[638] tap seat=2 source=Mogg Cannon target=seat0
[639] activate_ability seat=2 source=Mogg Cannon target=seat0
[640] stack_push seat=2 source=Mogg Cannon target=seat0
[641] priority_pass seat=3 source= target=seat0
[642] priority_pass seat=0 source= target=seat0
[643] priority_pass seat=1 source= target=seat0
[644] stack_resolve seat=2 source=Mogg Cannon target=seat0
[645] buff seat=0 source=Mogg Cannon amount=1 target=seat0
[646] activated_ability_resolved seat=2 source=Mogg Cannon target=seat0
[647] phase_step seat=2 source= target=seat0
[648] phase_step seat=2 source= target=seat0
[649] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[650] state seat=2 source= target=seat0
```

</details>

#### Violation 8

- **Game**: 16 (seed 160043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 23, Phase=ending Step=cleanup
- **Commanders**: Palladia-Mors, Sita Varma, Masked Racer, Jason Bright, Glowing Prophet, Spider-UK
- **Message**: CardIdentity: card "Rune of Protection: Red" (ptr 0x36fdd1974a20) appears in both seat 0 exile and seat 0 battlefield

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 805 events
  Seat 0 [alive]: life=40 library=83 hand=3 graveyard=3 exile=2 battlefield=8 cmdzone=1 mana=0
    - Drownyard Temple (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Generous Plunderer (P/T 2/2, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Rune of Protection: Red (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=40 library=86 hand=7 graveyard=1 exile=0 battlefield=4 cmdzone=1 mana=0
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Darksteel Pendant (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - War Tax (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=38 library=84 hand=1 graveyard=5 exile=2 battlefield=7 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mogg Cannon (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=5 graveyard=1 exile=0 battlefield=7 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - The Monumental Facade (P/T 0/0, dmg=0) [T]
    - Amaranthine Wall (P/T 0/6, dmg=0)
    - Forge of Heroes (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Rupture Spire (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[785] priority_pass seat=3 source= target=seat0
[786] stack_resolve seat=0 source=Rune of Protection: Red target=seat0
[787] replacement_seen seat=0 source=Rune of Protection: Red target=seat0
[788] activated_ability_resolved seat=0 source=Rune of Protection: Red target=seat0
[789] phase_step seat=0 source= target=seat0
[790] declare_attackers seat=0 source= target=seat0
[791] trigger_fires seat=0 source=Generous Plunderer target=seat0
[792] triggered_ability seat=0 source=Generous Plunderer target=seat0
[793] stack_push seat=0 source=Generous Plunderer target=seat0
[794] priority_pass seat=1 source= target=seat0
[795] priority_pass seat=2 source= target=seat0
[796] priority_pass seat=3 source= target=seat0
[797] stack_resolve seat=0 source=Generous Plunderer target=seat0
[798] parsed_effect_residual seat=0 source=Generous Plunderer target=seat0
[799] blockers seat=2 source= target=seat0
[800] damage seat=0 source=Generous Plunderer amount=2 target=seat2
[801] speed_advance seat=0 source= amount=1 target=seat0
[802] phase_step seat=0 source= target=seat0
[803] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[804] state seat=0 source= target=seat0
```

</details>

#### Violation 9

- **Game**: 16 (seed 160043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 31, Phase=ending Step=cleanup
- **Commanders**: Palladia-Mors, Sita Varma, Masked Racer, Jason Bright, Glowing Prophet, Spider-UK
- **Message**: CardIdentity: card "Seasoned Cathar // Seasoned Cathar" (ptr 0x36fdd1974b40) appears in both seat 0 graveyard and seat 0 exile

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1627 events
  Seat 0 [alive]: life=37 library=80 hand=3 graveyard=4 exile=4 battlefield=11 cmdzone=1 mana=0
    - Drownyard Temple (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Generous Plunderer (P/T 2/2, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Rune of Protection: Red (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Horizon Canopy (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=40 library=84 hand=7 graveyard=2 exile=1 battlefield=5 cmdzone=1 mana=0
    - Hall of Oracles (P/T 0/0, dmg=0) [T]
    - Darksteel Pendant (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - War Tax (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=34 library=80 hand=1 graveyard=6 exile=6 battlefield=8 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mogg Cannon (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Amphin Cutthroat (P/T 2/4, dmg=0) [T]
  Seat 3 [alive]: life=40 library=81 hand=4 graveyard=2 exile=1 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - The Monumental Facade (P/T 0/0, dmg=0) [T]
    - Amaranthine Wall (P/T 0/6, dmg=0)
    - Forge of Heroes (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Rupture Spire (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1607] priority_pass seat=3 source= target=seat0
[1608] stack_resolve seat=0 source=Seasoned Cathar // Seasoned Cathar target=seat0
[1609] zone_change seat=0 source=Seasoned Cathar // Seasoned Cathar
[1610] resolve seat=0 source=Seasoned Cathar // Seasoned Cathar target=seat0
[1611] phase_step seat=0 source= target=seat0
[1612] declare_attackers seat=0 source= target=seat0
[1613] trigger_fires seat=0 source=Generous Plunderer target=seat0
[1614] triggered_ability seat=0 source=Generous Plunderer target=seat0
[1615] stack_push seat=0 source=Generous Plunderer target=seat0
[1616] priority_pass seat=1 source= target=seat0
[1617] priority_pass seat=2 source= target=seat0
[1618] priority_pass seat=3 source= target=seat0
[1619] stack_resolve seat=0 source=Generous Plunderer target=seat0
[1620] parsed_effect_residual seat=0 source=Generous Plunderer target=seat0
[1621] blockers seat=2 source= target=seat0
[1622] damage seat=0 source=Generous Plunderer amount=2 target=seat2
[1623] speed_advance seat=0 source= amount=3 target=seat0
[1624] phase_step seat=0 source= target=seat0
[1625] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[1626] state seat=0 source= target=seat0
```

</details>

#### Violation 10

- **Game**: 14 (seed 140043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 22, Phase=ending Step=cleanup
- **Commanders**: Captain America, Super-Soldier, Zoyowa Lava-Tongue, Kykar, Wind's Fury, Nezahal, Primal Tide
- **Message**: CardIdentity: card "Captain America, Super-Soldier" (ptr 0x36fdd0c090e0) appears in both seat 0 command_zone and seat 0 battlefield

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1799 events
  Seat 0 [alive]: life=21 library=85 hand=3 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urza's Miter (P/T 0/0, dmg=0)
    - Captain America, Super-Soldier (P/T 3/2, dmg=0)
  Seat 1 [alive]: life=38 library=87 hand=0 graveyard=2 exile=0 battlefield=11 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Myr Galvanizer (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Zoyowa Lava-Tongue (P/T 2/2, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Butcher of Malakir (P/T 5/4, dmg=0) [T]
    - Flailing Ogre (P/T 3/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - Desert (P/T 0/0, dmg=0) [T]
    - Sen Triplets (P/T 3/3, dmg=0)
  Seat 2 [alive]: life=16 library=84 hand=3 graveyard=2 exile=0 battlefield=7 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dream Stalker (P/T 1/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Pentad Prism (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Faerie Duelist (P/T 1/2, dmg=0)
    - Pinnacle Monk // Mystic Peak (P/T 2/2, dmg=0) [T]
  Seat 3 [alive]: life=40 library=84 hand=0 graveyard=5 exile=0 battlefield=8 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Apprentice Wizard (P/T 0/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1779] stack_resolve seat=1 source=Sen Triplets target=seat0
[1780] sba_cycle_complete seat=-1 source=
[1781] phase_step seat=0 source= target=seat0
[1782] trigger_evaluated seat=1 source=Zoyowa Lava-Tongue
[1783] stack_push seat=1 source=Zoyowa Lava-Tongue target=seat0
[1784] triggered_ability seat=1 source=Zoyowa Lava-Tongue target=seat0
[1785] priority_pass seat=0 source= target=seat0
[1786] priority_pass seat=2 source= target=seat0
[1787] priority_pass seat=3 source= target=seat0
[1788] stack_resolve seat=1 source=Zoyowa Lava-Tongue target=seat0
[1789] trigger_evaluated seat=1 source=Zoyowa Lava-Tongue
[1790] stack_push seat=1 source=Zoyowa Lava-Tongue target=seat0
[1791] triggered_ability seat=1 source=Zoyowa Lava-Tongue target=seat0
[1792] priority_pass seat=0 source= target=seat0
[1793] priority_pass seat=2 source= target=seat0
[1794] priority_pass seat=3 source= target=seat0
[1795] stack_resolve seat=1 source=Zoyowa Lava-Tongue target=seat0
[1796] pool_drain seat=0 source= amount=1 target=seat0
[1797] damage_wears_off seat=1 source=Sen Triplets amount=2 target=seat0
[1798] state seat=0 source= target=seat0
```

</details>

#### Violation 11

- **Game**: 14 (seed 140043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 29, Phase=ending Step=cleanup
- **Commanders**: Captain America, Super-Soldier, Zoyowa Lava-Tongue, Kykar, Wind's Fury, Nezahal, Primal Tide
- **Message**: CardIdentity: card "Captain America, Super-Soldier" (ptr 0x36fdd0c090e0) appears in both seat 0 command_zone and seat 3 battlefield

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 3039 events
  Seat 0 [alive]: life=21 library=84 hand=3 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urza's Miter (P/T 0/0, dmg=0)
    - Viridian Claw (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=17 library=85 hand=0 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Myr Galvanizer (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Zoyowa Lava-Tongue (P/T 2/2, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Flailing Ogre (P/T 3/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - Desert (P/T 0/0, dmg=0) [T]
    - Sen Triplets (P/T 3/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=16 library=82 hand=2 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dream Stalker (P/T 1/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Pentad Prism (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Faerie Duelist (P/T 1/2, dmg=0) [T]
    - Pinnacle Monk // Mystic Peak (P/T 2/2, dmg=0) [T]
    - Grasping Dunes (P/T 0/0, dmg=0) [T]
    - Kykar, Wind's Fury (P/T 3/3, dmg=0) [T]
    - Spirit (P/T 1/1, dmg=0)
    - Skyship Weatherlight (P/T 0/0, dmg=0)
    - Spirit (P/T 1/1, dmg=0)
  Seat 3 [alive]: life=8 library=82 hand=0 graveyard=6 exile=0 battlefield=11 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Butcher of Malakir (P/T 5/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Captain America, Super-Soldier (P/T 3/2, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3019] damage seat=3 source=Roil Elemental amount=3 target=seat1
[3020] speed_advance seat=3 source= amount=4 target=seat0
[3021] damage seat=3 source=Butcher of Malakir amount=5 target=seat1
[3022] phase_step seat=3 source= target=seat0
[3023] trigger_evaluated seat=1 source=Zoyowa Lava-Tongue
[3024] stack_push seat=1 source=Zoyowa Lava-Tongue target=seat0
[3025] triggered_ability seat=1 source=Zoyowa Lava-Tongue target=seat0
[3026] priority_pass seat=3 source= target=seat0
[3027] priority_pass seat=0 source= target=seat0
[3028] priority_pass seat=2 source= target=seat0
[3029] stack_resolve seat=1 source=Zoyowa Lava-Tongue target=seat0
[3030] trigger_evaluated seat=1 source=Zoyowa Lava-Tongue
[3031] stack_push seat=1 source=Zoyowa Lava-Tongue target=seat0
[3032] triggered_ability seat=1 source=Zoyowa Lava-Tongue target=seat0
[3033] priority_pass seat=3 source= target=seat0
[3034] priority_pass seat=0 source= target=seat0
[3035] priority_pass seat=2 source= target=seat0
[3036] stack_resolve seat=1 source=Zoyowa Lava-Tongue target=seat0
[3037] pool_drain seat=3 source= amount=8 target=seat0
[3038] state seat=3 source= target=seat0
```

</details>

#### Violation 12

- **Game**: 14 (seed 140043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 30, Phase=ending Step=cleanup
- **Commanders**: Captain America, Super-Soldier, Zoyowa Lava-Tongue, Kykar, Wind's Fury, Nezahal, Primal Tide
- **Message**: CardIdentity: card "Captain America, Super-Soldier" (ptr 0x36fdd0c090e0) appears in both seat 0 battlefield and seat 3 battlefield

<details>
<summary>Game State</summary>

```
Turn 30, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3109 events
  Seat 0 [alive]: life=21 library=83 hand=3 graveyard=5 exile=0 battlefield=8 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Memorial to Glory (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urza's Miter (P/T 0/0, dmg=0)
    - Viridian Claw (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Captain America, Super-Soldier (P/T 3/2, dmg=0)
  Seat 1 [alive]: life=17 library=85 hand=0 graveyard=3 exile=0 battlefield=11 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Myr Galvanizer (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Zoyowa Lava-Tongue (P/T 2/2, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Flailing Ogre (P/T 3/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - Desert (P/T 0/0, dmg=0) [T]
    - Sen Triplets (P/T 3/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=16 library=82 hand=2 graveyard=3 exile=0 battlefield=12 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dream Stalker (P/T 1/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Pentad Prism (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Faerie Duelist (P/T 1/2, dmg=0) [T]
    - Pinnacle Monk // Mystic Peak (P/T 2/2, dmg=0) [T]
    - Grasping Dunes (P/T 0/0, dmg=0) [T]
    - Kykar, Wind's Fury (P/T 3/3, dmg=0) [T]
    - Spirit (P/T 1/1, dmg=0)
    - Skyship Weatherlight (P/T 0/0, dmg=0)
    - Spirit (P/T 1/1, dmg=0)
  Seat 3 [alive]: life=8 library=82 hand=0 graveyard=6 exile=0 battlefield=11 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Roil Elemental (P/T 3/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Butcher of Malakir (P/T 5/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Captain America, Super-Soldier (P/T 3/2, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3089] triggered_ability seat=3 source=Roil Elemental target=seat0
[3090] priority_pass seat=0 source= target=seat0
[3091] priority_pass seat=1 source= target=seat0
[3092] priority_pass seat=2 source= target=seat0
[3093] stack_resolve seat=3 source=Roil Elemental target=seat0
[3094] trigger_evaluated seat=1 source=Zoyowa Lava-Tongue
[3095] stack_push seat=1 source=Zoyowa Lava-Tongue target=seat0
[3096] triggered_ability seat=1 source=Zoyowa Lava-Tongue target=seat0
[3097] priority_pass seat=0 source= target=seat0
[3098] priority_pass seat=2 source= target=seat0
[3099] priority_pass seat=3 source= target=seat0
[3100] stack_resolve seat=1 source=Zoyowa Lava-Tongue target=seat0
[3101] trigger_evaluated seat=1 source=Zoyowa Lava-Tongue
[3102] stack_push seat=1 source=Zoyowa Lava-Tongue target=seat0
[3103] triggered_ability seat=1 source=Zoyowa Lava-Tongue target=seat0
[3104] priority_pass seat=0 source= target=seat0
[3105] priority_pass seat=2 source= target=seat0
[3106] priority_pass seat=3 source= target=seat0
[3107] stack_resolve seat=1 source=Zoyowa Lava-Tongue target=seat0
[3108] state seat=0 source= target=seat0
```

</details>

#### Violation 13

- **Game**: 22 (seed 220043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 31, Phase=ending Step=cleanup
- **Commanders**: Starke of Rath, Ezio Auditore da Firenze, Elrond, Master of Healing, Odric, Blood-Cursed
- **Message**: CardIdentity: card "Trained Condor" (ptr 0x36fdd64c86c0) appears in both seat 1 exile and seat 1 battlefield

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 889 events
  Seat 0 [alive]: life=36 library=83 hand=3 graveyard=6 exile=5 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=25 library=83 hand=4 graveyard=2 exile=2 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Rakdos Guildgate (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ezio Auditore da Firenze (P/T 3/2, dmg=0) [T]
    - Captain's Claws (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blightwing Bandit (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Trained Condor (P/T 2/1, dmg=0)
  Seat 2 [alive]: life=22 library=83 hand=3 graveyard=2 exile=1 battlefield=10 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Saprazzan Cove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elemental Bond (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Elrond, Master of Healing (P/T 4/4, dmg=0) [T]
    - Blighted Cataract (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=30 library=81 hand=3 graveyard=8 exile=0 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Quintorius, Loremaster (P/T 3/5, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[869] priority_pass seat=0 source= target=seat0
[870] stack_resolve seat=1 source=Trained Condor target=seat0
[871] enter_battlefield seat=1 source=Trained Condor target=seat0
[872] equip seat=1 source=Captain's Claws amount=2 target=seat0
[873] phase_step seat=1 source= target=seat0
[874] declare_attackers seat=1 source= target=seat0
[875] blockers seat=2 source= target=seat0
[876] damage seat=1 source=Ezio Auditore da Firenze amount=3 target=seat2
[877] trigger_fires seat=1 source=Ezio Auditore da Firenze amount=3 target=seat2
[878] triggered_ability seat=1 source=Ezio Auditore da Firenze target=seat0
[879] stack_push seat=1 source=Ezio Auditore da Firenze target=seat0
[880] priority_pass seat=2 source= target=seat0
[881] priority_pass seat=3 source= target=seat0
[882] priority_pass seat=0 source= target=seat0
[883] stack_resolve seat=1 source=Ezio Auditore da Firenze target=seat0
[884] parsed_effect_residual seat=1 source=Ezio Auditore da Firenze target=seat0
[885] damage seat=1 source=Blightwing Bandit amount=2 target=seat2
[886] phase_step seat=1 source= target=seat0
[887] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[888] state seat=1 source= target=seat0
```

</details>

#### Violation 14

- **Game**: 22 (seed 220043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 39, Phase=ending Step=cleanup
- **Commanders**: Starke of Rath, Ezio Auditore da Firenze, Elrond, Master of Healing, Odric, Blood-Cursed
- **Message**: CardIdentity: card "Sunbeam Spellbomb" (ptr 0x36fdd64c3d40) appears in both seat 1 graveyard and seat 1 exile

<details>
<summary>Game State</summary>

```
Turn 39, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1343 events
  Seat 0 [alive]: life=36 library=81 hand=4 graveyard=6 exile=5 battlefield=1 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=11 library=77 hand=6 graveyard=3 exile=7 battlefield=9 cmdzone=0 mana=0
    - Rakdos Guildgate (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ezio Auditore da Firenze (P/T 3/2, dmg=0) [T]
    - Captain's Claws (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blightwing Bandit (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Trained Condor (P/T 2/1, dmg=0) [T]
    - Geological Appraiser (P/T 3/2, dmg=0) [T]
  Seat 2 [alive]: life=5 library=78 hand=0 graveyard=2 exile=7 battlefield=15 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Saprazzan Cove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elemental Bond (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Elrond, Master of Healing (P/T 4/4, dmg=0) [T]
    - Blighted Cataract (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Wild Elephant (P/T 3/3, dmg=0) [T]
    - Emperor Mihail II (P/T 3/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Bronze Cudgels (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=30 library=78 hand=3 graveyard=11 exile=2 battlefield=3 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1323] priority_pass seat=0 source= target=seat0
[1324] stack_resolve seat=1 source=Trained Condor target=seat0
[1325] grant_ability seat=0 source=Trained Condor target=seat0
[1326] blockers seat=2 source= target=seat0
[1327] damage seat=1 source=Ezio Auditore da Firenze amount=3 target=seat2
[1328] trigger_fires seat=1 source=Ezio Auditore da Firenze amount=3 target=seat2
[1329] triggered_ability seat=1 source=Ezio Auditore da Firenze target=seat0
[1330] stack_push seat=1 source=Ezio Auditore da Firenze target=seat0
[1331] priority_pass seat=2 source= target=seat0
[1332] priority_pass seat=3 source= target=seat0
[1333] priority_pass seat=0 source= target=seat0
[1334] stack_resolve seat=1 source=Ezio Auditore da Firenze target=seat0
[1335] parsed_effect_residual seat=1 source=Ezio Auditore da Firenze target=seat0
[1336] damage seat=1 source=Blightwing Bandit amount=2 target=seat2
[1337] damage seat=1 source=Trained Condor amount=2 target=seat2
[1338] damage seat=1 source=Geological Appraiser amount=3 target=seat2
[1339] phase_step seat=1 source= target=seat0
[1340] pool_drain seat=1 source= amount=2 target=seat0
[1341] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[1342] state seat=1 source= target=seat0
```

</details>

#### Violation 15

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 20, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: CardIdentity: card "Charging Cinderhorn" (ptr 0x36fdd40ac480) appears in both seat 3 exile and seat 3 battlefield

<details>
<summary>Game State</summary>

```
Turn 20, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1085 events
  Seat 0 [alive]: life=38 library=77 hand=4 graveyard=6 exile=3 battlefield=7 cmdzone=1 mana=0
    - Kazuul's Fury // Kazuul's Cliffs (P/T 0/0, dmg=0) [T]
    - Shrine of the Forsaken Gods (P/T 0/0, dmg=0) [T]
    - Hunter's Blowgun (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Earth Elemental (P/T 4/5, dmg=0)
  Seat 1 [alive]: life=40 library=87 hand=5 graveyard=3 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=32 library=86 hand=4 graveyard=0 exile=1 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Orzhova, the Church of Deals (P/T 0/0, dmg=0) [T]
    - Agatha's Soul Cauldron (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Neva, Stalked by Nightmares (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=85 hand=3 graveyard=0 exile=2 battlefield=9 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Page, Loose Leaf (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Goblin Brawler (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Charging Cinderhorn (P/T 4/2, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1065] activate_ability seat=3 source=Page, Loose Leaf target=seat0
[1066] stack_push seat=3 source=Page, Loose Leaf target=seat0
[1067] priority_pass seat=0 source= target=seat0
[1068] priority_pass seat=1 source= target=seat0
[1069] priority_pass seat=2 source= target=seat0
[1070] stack_resolve seat=3 source=Page, Loose Leaf target=seat0
[1071] per_card_failed seat=0 source=Page, Loose Leaf target=seat0
[1072] tutor seat=3 source=generic_tutor target=seat0
[1073] activated_ability_resolved seat=3 source=Page, Loose Leaf target=seat0
[1074] activate_ability seat=3 source=Page, Loose Leaf target=seat0
[1075] stack_push seat=3 source=Page, Loose Leaf target=seat0
[1076] priority_pass seat=0 source= target=seat0
[1077] priority_pass seat=1 source= target=seat0
[1078] priority_pass seat=2 source= target=seat0
[1079] stack_resolve seat=3 source=Page, Loose Leaf target=seat0
[1080] per_card_failed seat=0 source=Page, Loose Leaf target=seat0
[1081] tutor seat=3 source=generic_tutor target=seat0
[1082] activated_ability_resolved seat=3 source=Page, Loose Leaf target=seat0
[1083] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[1084] state seat=3 source= target=seat0
```

</details>

#### Violation 16

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 23, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: CardIdentity: card "Magnetic Snuffler" (ptr 0x36fdd409e900) appears in both seat 2 exile and seat 2 battlefield

<details>
<summary>Game State</summary>

```
Turn 23, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1224 events
  Seat 0 [alive]: life=36 library=74 hand=4 graveyard=7 exile=4 battlefield=9 cmdzone=0 mana=0
    - Kazuul's Fury // Kazuul's Cliffs (P/T 0/0, dmg=0) [T]
    - Shrine of the Forsaken Gods (P/T 0/0, dmg=0) [T]
    - Hunter's Blowgun (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Earth Elemental (P/T 4/5, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mana Max, Afterburner (P/T 4/4, dmg=0)
  Seat 1 [alive]: life=40 library=86 hand=6 graveyard=3 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=32 library=84 hand=3 graveyard=0 exile=3 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Orzhova, the Church of Deals (P/T 0/0, dmg=0) [T]
    - Agatha's Soul Cauldron (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Neva, Stalked by Nightmares (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Magnetic Snuffler (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=36 library=85 hand=3 graveyard=0 exile=2 battlefield=9 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Page, Loose Leaf (P/T 0/2, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Goblin Brawler (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Charging Cinderhorn (P/T 4/2, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1204] priority_pass seat=1 source= target=seat0
[1205] stack_resolve seat=2 source=Magnetic Snuffler target=seat0
[1206] enter_battlefield seat=2 source=Magnetic Snuffler target=seat0
[1207] triggered_ability seat=2 source=Magnetic Snuffler target=seat0
[1208] citys_blessing seat=2 source= amount=10 target=seat0
[1209] stack_push seat=2 source=Magnetic Snuffler target=seat0
[1210] triggers_ordered seat=2 source= target=seat0
[1211] priority_pass seat=3 source= target=seat0
[1212] priority_pass seat=0 source= target=seat0
[1213] priority_pass seat=1 source= target=seat0
[1214] stack_resolve seat=2 source=Magnetic Snuffler target=seat0
[1215] phase_step seat=2 source= target=seat0
[1216] declare_attackers seat=2 source= target=seat0
[1217] blockers seat=0 source= target=seat0
[1218] damage seat=2 source=Neva, Stalked by Nightmares amount=2 target=seat0
[1219] speed_advance seat=2 source= amount=2 target=seat0
[1220] phase_step seat=2 source= target=seat0
[1221] pool_drain seat=2 source= amount=1 target=seat0
[1222] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[1223] state seat=2 source= target=seat0
```

</details>

#### Violation 17

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 25, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: CardIdentity: card "Chandra, Awakened Inferno" (ptr 0x36fdd40818c0) appears in both seat 0 exile and seat 0 battlefield

<details>
<summary>Game State</summary>

```
Turn 25, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1644 events
  Seat 0 [alive]: life=32 library=72 hand=3 graveyard=7 exile=6 battlefield=11 cmdzone=0 mana=0
    - Kazuul's Fury // Kazuul's Cliffs (P/T 0/0, dmg=0) [T]
    - Shrine of the Forsaken Gods (P/T 0/0, dmg=0) [T]
    - Hunter's Blowgun (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Earth Elemental (P/T 5/6, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mana Max, Afterburner (P/T 4/4, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Chandra, Awakened Inferno (P/T 0/6, dmg=0)
  Seat 1 [alive]: life=40 library=86 hand=6 graveyard=3 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=23 library=84 hand=3 graveyard=0 exile=3 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Orzhova, the Church of Deals (P/T 0/0, dmg=0) [T]
    - Agatha's Soul Cauldron (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Magnetic Snuffler (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=36 library=83 hand=3 graveyard=4 exile=4 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1624] damage_wears_off seat=1 source=Vedalken Entrancer amount=3 target=seat0
[1625] damage_wears_off seat=1 source=Island amount=3 target=seat0
[1626] damage_wears_off seat=1 source=Island amount=3 target=seat0
[1627] damage_wears_off seat=2 source=Swamp amount=3 target=seat0
[1628] damage_wears_off seat=2 source=Orzhova, the Church of Deals amount=3 target=seat0
[1629] damage_wears_off seat=2 source=Agatha's Soul Cauldron amount=3 target=seat0
[1630] damage_wears_off seat=2 source=Plains amount=3 target=seat0
[1631] damage_wears_off seat=2 source=Swamp amount=3 target=seat0
[1632] damage_wears_off seat=2 source=Swamp amount=3 target=seat0
[1633] damage_wears_off seat=2 source=Possibility Storm amount=3 target=seat0
[1634] damage_wears_off seat=2 source=Plains amount=3 target=seat0
[1635] damage_wears_off seat=2 source=Magnetic Snuffler amount=3 target=seat0
[1636] damage_wears_off seat=3 source=Mountain amount=3 target=seat0
[1637] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[1638] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[1639] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[1640] damage_wears_off seat=3 source=Trinisphere amount=3 target=seat0
[1641] damage_wears_off seat=3 source=Mountain amount=3 target=seat0
[1642] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[1643] state seat=0 source= target=seat0
```

</details>

#### Violation 18

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: CardIdentity: card "Ambergris, Citadel Agent" (ptr 0x36fdd4087320) appears in both seat 0 graveyard and seat 0 exile

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2282 events
  Seat 0 [alive]: life=18 library=60 hand=1 graveyard=12 exile=16 battlefield=14 cmdzone=0 mana=0
    - Kazuul's Fury // Kazuul's Cliffs (P/T 0/0, dmg=0) [T]
    - Shrine of the Forsaken Gods (P/T 0/0, dmg=0) [T]
    - Hunter's Blowgun (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mana Max, Afterburner (P/T 5/5, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Chandra, Awakened Inferno (P/T 0/6, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Earth Elemental (P/T 4/5, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Seer's Sundial (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=84 hand=7 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=14 library=81 hand=4 graveyard=2 exile=5 battlefield=8 cmdzone=1 mana=0
    - Orzhova, the Church of Deals (P/T 0/0, dmg=0) [T]
    - Agatha's Soul Cauldron (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=26 library=79 hand=3 graveyard=6 exile=8 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2262] damage_wears_off seat=1 source=Vedalken Entrancer amount=3 target=seat0
[2263] damage_wears_off seat=1 source=Island amount=3 target=seat0
[2264] damage_wears_off seat=1 source=Island amount=3 target=seat0
[2265] damage_wears_off seat=2 source=Orzhova, the Church of Deals amount=3 target=seat0
[2266] damage_wears_off seat=2 source=Agatha's Soul Cauldron amount=3 target=seat0
[2267] damage_wears_off seat=2 source=Plains amount=3 target=seat0
[2268] damage_wears_off seat=2 source=Swamp amount=3 target=seat0
[2269] damage_wears_off seat=2 source=Swamp amount=3 target=seat0
[2270] damage_wears_off seat=2 source=Possibility Storm amount=3 target=seat0
[2271] damage_wears_off seat=2 source=Plains amount=3 target=seat0
[2272] damage_wears_off seat=2 source=Bolas's Citadel amount=3 target=seat0
[2273] damage_wears_off seat=3 source=Mountain amount=3 target=seat0
[2274] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[2275] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[2276] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[2277] damage_wears_off seat=3 source=Trinisphere amount=3 target=seat0
[2278] damage_wears_off seat=3 source=Mountain amount=3 target=seat0
[2279] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[2280] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[2281] state seat=0 source= target=seat0
```

</details>

#### Violation 19

- **Game**: 25 (seed 250043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 42, Phase=ending Step=cleanup
- **Commanders**: Davros, Dalek Creator, Neriv, Crackling Vanguard, Carth the Lion, Albiorix, Goose Tyrant // Wild Goose Chase
- **Message**: CardIdentity: card "Feedback" (ptr 0x36fddc2af560) appears in both seat 3 graveyard and seat 3 exile

<details>
<summary>Game State</summary>

```
Turn 42, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2259 events
  Seat 0 [alive]: life=34 library=81 hand=7 graveyard=3 exile=0 battlefield=7 cmdzone=1 mana=0
    - Cinder Marsh (P/T 0/0, dmg=0) [T]
    - War Cadence (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=75 hand=4 graveyard=5 exile=7 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=11 library=67 hand=7 graveyard=6 exile=2 battlefield=16 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Demolition Field (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Journeyer's Kite (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Carth the Lion (P/T 3/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mercadian Atlas (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Galadhrim Brigade (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=17 library=79 hand=4 graveyard=5 exile=5 battlefield=8 cmdzone=0 mana=0
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Demilich (P/T 4/3, dmg=0) [T]
    - Llanowar Vanguard (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Albiorix, Goose Tyrant // Wild Goose Chase (P/T 3/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2239] trigger_evaluated seat=1 source=Neriv, Crackling Vanguard
[2240] stack_push seat=1 source=Neriv, Crackling Vanguard target=seat0
[2241] triggered_ability seat=1 source=Neriv, Crackling Vanguard target=seat0
[2242] priority_pass seat=3 source= target=seat0
[2243] priority_pass seat=0 source= target=seat0
[2244] priority_pass seat=2 source= target=seat0
[2245] stack_resolve seat=1 source=Neriv, Crackling Vanguard target=seat0
[2246] blockers seat=1 source= target=seat0
[2247] damage seat=3 source=Demilich amount=1 target=seat1
[2248] damage seat=3 source=Albiorix, Goose Tyrant // Wild Goose Chase amount=3 target=seat1
[2249] sba_704_5a seat=1 source=
[2250] destroy seat=1 source=Camel
[2251] sba_704_5g seat=1 source=Camel
[2252] zone_change seat=1 source=Camel
[2253] sba_cycle_complete seat=-1 source=
[2254] seat_eliminated seat=1 source= amount=9
[2255] phase_step seat=3 source= target=seat0
[2256] pool_drain seat=3 source= amount=2 target=seat0
[2257] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[2258] state seat=3 source= target=seat0
```

</details>

#### Violation 20

- **Game**: 25 (seed 250043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 44, Phase=ending Step=cleanup
- **Commanders**: Davros, Dalek Creator, Neriv, Crackling Vanguard, Carth the Lion, Albiorix, Goose Tyrant // Wild Goose Chase
- **Message**: CardIdentity: card "Underhanded Designs" (ptr 0x36fddc2a9c20) appears in both seat 2 graveyard and seat 2 exile

<details>
<summary>Game State</summary>

```
Turn 44, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2533 events
  Seat 0 [alive]: life=28 library=80 hand=7 graveyard=3 exile=0 battlefield=8 cmdzone=1 mana=0
    - Cinder Marsh (P/T 0/0, dmg=0) [T]
    - War Cadence (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=0 library=75 hand=4 graveyard=5 exile=7 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=12 library=60 hand=5 graveyard=8 exile=10 battlefield=19 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Demolition Field (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Journeyer's Kite (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Carth the Lion (P/T 3/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mercadian Atlas (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Galadhrim Brigade (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fynn, the Fangbearer (P/T 1/3, dmg=0)
    - Wirewood Herald (P/T 1/1, dmg=0)
  Seat 3 [alive]: life=16 library=79 hand=4 graveyard=6 exile=5 battlefield=7 cmdzone=0 mana=0
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Llanowar Vanguard (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Albiorix, Goose Tyrant // Wild Goose Chase (P/T 3/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2513] stack_push seat=2 source=Fynn, the Fangbearer target=seat0
[2514] triggered_ability seat=2 source=Fynn, the Fangbearer target=seat0
[2515] priority_pass seat=3 source= target=seat0
[2516] priority_pass seat=0 source= target=seat0
[2517] stack_resolve seat=2 source=Fynn, the Fangbearer target=seat0
[2518] phase_step seat=2 source= target=seat0
[2519] triggered_ability seat=2 source=Mercadian Atlas target=seat0
[2520] stack_push seat=2 source=Mercadian Atlas target=seat0
[2521] triggers_ordered seat=2 source= target=seat0
[2522] priority_pass seat=3 source= target=seat0
[2523] priority_pass seat=0 source= target=seat0
[2524] stack_resolve seat=2 source=Mercadian Atlas target=seat0
[2525] zone_change seat=2 source=Snapping Sailback
[2526] draw seat=2 source=Mercadian Atlas amount=1 target=seat2
[2527] pool_drain seat=2 source= amount=1 target=seat0
[2528] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2529] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2530] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2531] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2532] state seat=2 source= target=seat0
```

</details>

#### Violation 21

- **Game**: 25 (seed 250043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Davros, Dalek Creator, Neriv, Crackling Vanguard, Carth the Lion, Albiorix, Goose Tyrant // Wild Goose Chase
- **Message**: CardIdentity: card "Soul Net" (ptr 0x36fddc2938c0) appears in both seat 0 exile and seat 0 battlefield

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3201 events
  Seat 0 [alive]: life=21 library=76 hand=7 graveyard=5 exile=2 battlefield=9 cmdzone=1 mana=0
    - Cinder Marsh (P/T 0/0, dmg=0) [T]
    - War Cadence (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Soul Net (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=0 library=75 hand=4 graveyard=5 exile=7 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=3 library=51 hand=6 graveyard=9 exile=16 battlefield=23 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Demolition Field (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Journeyer's Kite (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Carth the Lion (P/T 3/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mercadian Atlas (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Galadhrim Brigade (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fynn, the Fangbearer (P/T 1/3, dmg=0) [T]
    - Wirewood Herald (P/T 1/1, dmg=0) [T]
    - Snapping Sailback (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Meltstrider's Gear (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=5 library=75 hand=6 graveyard=6 exile=7 battlefield=8 cmdzone=0 mana=0
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Llanowar Vanguard (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Albiorix, Goose Tyrant // Wild Goose Chase (P/T 3/3, dmg=0) [T]
    - Path of Discovery (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[3181] zone_change seat=0 source=Mountain
[3182] zone_change seat=0 source=Praetor's Grasp
[3183] zone_change seat=0 source=Celestus Sanctifier // Celestus Sanctifier
[3184] zone_change seat=0 source=Lightning-Core Excavator
[3185] possibility_storm seat=0 source=Possibility Storm target=seat0
[3186] zone_cast_grant_registered seat=0 source=Possibility Storm target=seat0
[3187] zone_change seat=0 source=Lofty Denial
[3188] zone_change seat=0 source=Mountain
[3189] zone_change seat=0 source=Celestus Sanctifier // Celestus Sanctifier
[3190] zone_change seat=0 source=Island
[3191] zone_change seat=0 source=Praetor's Grasp
[3192] stack_push seat=0 source=Soul Net target=seat0
[3193] priority_pass seat=2 source= target=seat0
[3194] priority_pass seat=3 source= target=seat0
[3195] stack_resolve seat=0 source=Soul Net target=seat0
[3196] enter_battlefield seat=0 source=Soul Net target=seat0
[3197] phase_step seat=0 source= target=seat0
[3198] phase_step seat=0 source= target=seat0
[3199] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[3200] state seat=0 source= target=seat0
```

</details>

#### Violation 22

- **Game**: 39 (seed 390043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 40, Phase=ending Step=cleanup
- **Commanders**: Kathril, Aspect Warper, Raul, Trouble Shooter, Hunding Gjornersen, Voja, Jaws of the Conclave
- **Message**: CardIdentity: card "Ajani's Mantra" (ptr 0x36fdb16b50e0) appears in both seat 3 exile and seat 3 battlefield

<details>
<summary>Game State</summary>

```
Turn 40, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 1342 events
  Seat 0 [alive]: life=33 library=81 hand=4 graveyard=6 exile=0 battlefield=8 cmdzone=0 mana=0
    - Thousand Moons Smithy // Barracks of the Thousand (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Deeproot Champion (P/T 1/1, dmg=0) [T]
    - Adventurer's Inn (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Kathril, Aspect Warper (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=27 library=81 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Jace's Archivist (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ivory Tower (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Carrion Wall (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=30 library=81 hand=2 graveyard=6 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Balduvian Frostwaker (P/T 1/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sentinels of Glen Elendra (P/T 2/3, dmg=0) [T]
    - Sliver Hive (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=35 library=80 hand=6 graveyard=4 exile=2 battlefield=7 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Naya Battlemage (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Gruul Signet (P/T 0/0, dmg=0) [T]
    - Acolyte of Bahamut (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Ajani's Mantra (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1322] zone_cast_grant_registered seat=3 source=Possibility Storm target=seat0
[1323] zone_change seat=3 source=Forest
[1324] zone_change seat=3 source=Saheeli's Lattice // Mastercraft Raptor
[1325] zone_change seat=3 source=Detritivore
[1326] zone_change seat=3 source=Knowledge Pool
[1327] zone_change seat=3 source=Multani
[1328] zone_change seat=3 source=Plains
[1329] zone_change seat=3 source=Forest
[1330] zone_change seat=3 source=Rapier Wit // Rapier Wit
[1331] stack_push seat=3 source=Ajani's Mantra target=seat0
[1332] priority_pass seat=0 source= target=seat0
[1333] priority_pass seat=1 source= target=seat0
[1334] priority_pass seat=2 source= target=seat0
[1335] stack_resolve seat=3 source=Ajani's Mantra target=seat0
[1336] enter_battlefield seat=3 source=Ajani's Mantra target=seat0
[1337] phase_step seat=3 source= target=seat0
[1338] phase_step seat=3 source= target=seat0
[1339] pool_drain seat=3 source= amount=1 target=seat0
[1340] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[1341] state seat=3 source= target=seat0
```

</details>

#### Violation 23

- **Game**: 39 (seed 390043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 41, Phase=ending Step=cleanup
- **Commanders**: Kathril, Aspect Warper, Raul, Trouble Shooter, Hunding Gjornersen, Voja, Jaws of the Conclave
- **Message**: CardIdentity: card "Encroaching Dragonstorm" (ptr 0x36fdb12638c0) appears in both seat 0 hand and seat 0 exile

<details>
<summary>Game State</summary>

```
Turn 41, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1434 events
  Seat 0 [alive]: life=33 library=79 hand=4 graveyard=6 exile=2 battlefield=9 cmdzone=0 mana=0
    - Thousand Moons Smithy // Barracks of the Thousand (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Deeproot Champion (P/T 1/1, dmg=0) [T]
    - Adventurer's Inn (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Kathril, Aspect Warper (P/T 3/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=27 library=81 hand=6 graveyard=4 exile=0 battlefield=7 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Jace's Archivist (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ivory Tower (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Carrion Wall (P/T 3/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=26 library=81 hand=2 graveyard=6 exile=0 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Balduvian Frostwaker (P/T 1/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sentinels of Glen Elendra (P/T 2/3, dmg=0) [T]
    - Sliver Hive (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=35 library=80 hand=6 graveyard=4 exile=2 battlefield=7 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Naya Battlemage (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Gruul Signet (P/T 0/0, dmg=0) [T]
    - Acolyte of Bahamut (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Ajani's Mantra (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1414] priority_pass seat=1 source= target=seat0
[1415] priority_pass seat=2 source= target=seat0
[1416] priority_pass seat=3 source= target=seat0
[1417] stack_resolve seat=0 source=Encroaching Dragonstorm target=seat0
[1418] bounce seat=0 source=Encroaching Dragonstorm target=seat0
[1419] zone_change seat=0 source=Encroaching Dragonstorm
[1420] priority_pass seat=1 source= target=seat0
[1421] priority_pass seat=2 source= target=seat0
[1422] priority_pass seat=3 source= target=seat0
[1423] stack_resolve seat=0 source=Encroaching Dragonstorm target=seat0
[1424] tutor seat=0 source=generic_tutor target=seat0
[1425] phase_step seat=0 source= target=seat0
[1426] declare_attackers seat=0 source= target=seat0
[1427] blockers seat=2 source= target=seat0
[1428] damage seat=0 source=Deeproot Champion amount=1 target=seat2
[1429] damage seat=0 source=Kathril, Aspect Warper amount=3 target=seat2
[1430] phase_step seat=0 source= target=seat0
[1431] pool_drain seat=0 source= amount=3 target=seat0
[1432] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[1433] state seat=0 source= target=seat0
```

</details>

#### Violation 24

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 26, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: CardIdentity: card "Alloy Golem" (ptr 0x36fdb18e37a0) appears in both seat 3 exile and seat 3 battlefield

<details>
<summary>Game State</summary>

```
Turn 26, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 914 events
  Seat 0 [alive]: life=43 library=84 hand=3 graveyard=3 exile=0 battlefield=7 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Toucan-Puffin (P/T 2/2, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=39 library=85 hand=5 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=38 library=85 hand=0 graveyard=4 exile=0 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=27 library=83 hand=2 graveyard=3 exile=2 battlefield=10 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Druid of Purification (P/T 2/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Alloy Golem (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[894] triggered_ability seat=3 source=Alloy Golem target=seat0
[895] citys_blessing seat=3 source= amount=10 target=seat0
[896] stack_push seat=3 source=Alloy Golem target=seat0
[897] triggers_ordered seat=3 source= target=seat0
[898] priority_pass seat=0 source= target=seat0
[899] priority_pass seat=1 source= target=seat0
[900] priority_pass seat=2 source= target=seat0
[901] stack_resolve seat=3 source=Alloy Golem target=seat0
[902] modification_effect seat=3 source=Alloy Golem target=seat0
[903] parser_gap seat=3 source=Alloy Golem target=seat0
[904] phase_step seat=3 source= target=seat0
[905] declare_attackers seat=3 source= target=seat0
[906] blockers seat=0 source= target=seat0
[907] damage seat=3 source=Druid of Purification amount=2 target=seat0
[908] speed_advance seat=3 source= amount=3 target=seat0
[909] damage seat=3 source=Vorinclex, Monstrous Raider amount=6 target=seat0
[910] phase_step seat=3 source= target=seat0
[911] pool_drain seat=3 source= amount=1 target=seat0
[912] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[913] state seat=3 source= target=seat0
```

</details>

#### Violation 25

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 29, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: CardIdentity: card "Bog-Strider Ash" (ptr 0x36fdb17d6d80) appears in both seat 2 exile and seat 2 battlefield

<details>
<summary>Game State</summary>

```
Turn 29, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1065 events
  Seat 0 [alive]: life=46 library=83 hand=4 graveyard=3 exile=0 battlefield=7 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Toucan-Puffin (P/T 2/2, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=39 library=84 hand=5 graveyard=3 exile=0 battlefield=7 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=38 library=83 hand=0 graveyard=4 exile=2 battlefield=11 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Bog-Strider Ash (P/T 2/4, dmg=0)
  Seat 3 [alive]: life=22 library=83 hand=2 graveyard=3 exile=2 battlefield=10 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Druid of Purification (P/T 2/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Alloy Golem (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1045] zone_change seat=2 source=Deconstruct
[1046] zone_change seat=2 source=Forest
[1047] trigger_evaluated seat=0 source=Kellan, the Kid
[1048] stack_push seat=0 source=Kellan, the Kid target=seat0
[1049] triggered_ability seat=0 source=Kellan, the Kid target=seat0
[1050] priority_pass seat=2 source= target=seat0
[1051] priority_pass seat=3 source= target=seat0
[1052] priority_pass seat=1 source= target=seat0
[1053] stack_resolve seat=0 source=Kellan, the Kid target=seat0
[1054] stack_push seat=2 source=Bog-Strider Ash target=seat0
[1055] priority_pass seat=3 source= target=seat0
[1056] priority_pass seat=0 source= target=seat0
[1057] priority_pass seat=1 source= target=seat0
[1058] stack_resolve seat=2 source=Bog-Strider Ash target=seat0
[1059] enter_battlefield seat=2 source=Bog-Strider Ash target=seat0
[1060] phase_step seat=2 source= target=seat0
[1061] phase_step seat=2 source= target=seat0
[1062] pool_drain seat=2 source= amount=1 target=seat0
[1063] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[1064] state seat=2 source= target=seat0
```

</details>

#### Violation 26

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 31, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: CardIdentity: card "Boxing Ring" (ptr 0x36fdb179ec60) appears in both seat 0 exile and seat 0 battlefield

<details>
<summary>Game State</summary>

```
Turn 31, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1233 events
  Seat 0 [alive]: life=37 library=81 hand=4 graveyard=3 exile=2 battlefield=9 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Toucan-Puffin (P/T 2/2, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=39 library=84 hand=5 graveyard=3 exile=0 battlefield=7 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=38 library=83 hand=0 graveyard=4 exile=2 battlefield=11 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Bog-Strider Ash (P/T 2/4, dmg=0)
  Seat 3 [alive]: life=17 library=81 hand=1 graveyard=3 exile=4 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Druid of Purification (P/T 2/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Alloy Golem (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1213] fight seat=0 source=Boxing Ring target=seat0
[1214] tap seat=0 source=Boxing Ring target=seat0
[1215] activate_ability seat=0 source=Boxing Ring target=seat0
[1216] stack_push seat=0 source=Boxing Ring target=seat0
[1217] priority_pass seat=1 source= target=seat0
[1218] priority_pass seat=2 source= target=seat0
[1219] priority_pass seat=3 source= target=seat0
[1220] stack_resolve seat=0 source=Boxing Ring target=seat0
[1221] create_token seat=0 source=Boxing Ring amount=1 target=seat0
[1222] activated_ability_resolved seat=0 source=Boxing Ring target=seat0
[1223] phase_step seat=0 source= target=seat0
[1224] declare_attackers seat=0 source= target=seat0
[1225] blockers seat=3 source= target=seat0
[1226] damage seat=0 source=Kellan, the Kid amount=3 target=seat3
[1227] damage seat=0 source=Toucan-Puffin amount=2 target=seat3
[1228] phase_step seat=0 source= target=seat0
[1229] pool_drain seat=0 source= amount=3 target=seat0
[1230] damage_wears_off seat=0 source=Boxing Ring amount=6 target=seat0
[1231] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[1232] state seat=0 source= target=seat0
```

</details>

#### Violation 27

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 39, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: CardIdentity: card "Vitu-Ghazi Guildmage" (ptr 0x36fdb179f7a0) appears in both seat 0 graveyard and seat 0 exile

<details>
<summary>Game State</summary>

```
Turn 39, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1922 events
  Seat 0 [alive]: life=37 library=78 hand=4 graveyard=4 exile=4 battlefield=12 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Toucan-Puffin (P/T 2/2, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - creature token green centaur Token (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=17 library=80 hand=5 graveyard=3 exile=4 battlefield=11 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
    - Spirited Simulacrum (P/T 2/1, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Hostage Taker (P/T 2/3, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=34 library=79 hand=0 graveyard=7 exile=6 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=12 library=78 hand=0 graveyard=5 exile=6 battlefield=13 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Alloy Golem (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1902] priority_pass seat=3 source= target=seat0
[1903] stack_resolve seat=1 source=Hostage Taker target=seat0
[1904] destroy seat=0 source=Vitu-Ghazi Guildmage
[1905] sba_704_5g seat=0 source=Vitu-Ghazi Guildmage
[1906] zone_change seat=0 source=Vitu-Ghazi Guildmage
[1907] trigger_evaluated seat=1 source=Hostage Taker
[1908] stack_push seat=1 source=Hostage Taker target=seat0
[1909] triggered_ability seat=1 source=Hostage Taker target=seat0
[1910] priority_pass seat=0 source= target=seat0
[1911] priority_pass seat=2 source= target=seat0
[1912] priority_pass seat=3 source= target=seat0
[1913] stack_resolve seat=1 source=Hostage Taker target=seat0
[1914] sba_704_6d seat=0 source=Kellan, the Kid
[1915] sba_cycle_complete seat=-1 source=
[1916] sba_cycle_complete seat=-1 source=
[1917] sba_cycle_complete seat=-1 source=
[1918] phase_step seat=0 source= target=seat0
[1919] damage_wears_off seat=1 source=Vv'viza, Orbital Overseer amount=3 target=seat0
[1920] damage_wears_off seat=1 source=Hostage Taker amount=2 target=seat0
[1921] state seat=0 source= target=seat0
```

</details>

#### Violation 28

- **Game**: 19 (seed 190043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 22, Phase=ending Step=cleanup
- **Commanders**: Ulamog, the Ceaseless Hunger, Myrel, Shield of Argive, Endrek Sahr, Master Breeder, Tourach, Dread Cantor
- **Message**: CardIdentity: card "Arni Brokenbrow // Arni Brokenbrow" (ptr 0x36fdd4aa6000) appears in both seat 2 graveyard and seat 2 exile

<details>
<summary>Game State</summary>

```
Turn 22, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 785 events
  Seat 0 [alive]: life=40 library=84 hand=1 graveyard=9 exile=0 battlefield=5 cmdzone=1 mana=0
    - Trenchpost (P/T 0/0, dmg=0) [T]
    - Forsaken Crossroads (P/T 0/0, dmg=0) [T]
    - Sunscorched Desert (P/T 0/0, dmg=0) [T]
    - Iron Spider, Stark Upgrade (P/T 4/5, dmg=0) [T]
    - Ugin's Labyrinth (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=24 library=86 hand=3 graveyard=0 exile=0 battlefield=19 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Hookblade (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Crusade (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Gustcloak Cavalier (P/T 2/2, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Myr Matrix (P/T 0/0, dmg=0)
    - creature token soldier Token (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=28 library=85 hand=2 graveyard=3 exile=2 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Echoing Deeps (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Faerie Macabre (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Fire Nation Engineer (P/T 2/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Endrek Sahr, Master Breeder (P/T 2/2, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=3 graveyard=4 exile=0 battlefield=6 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tourach, Dread Cantor (P/T 2/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[765] triggered_ability seat=1 source=Myrel, Shield of Argive target=seat0
[766] priority_pass seat=2 source= target=seat0
[767] priority_pass seat=3 source= target=seat0
[768] priority_pass seat=0 source= target=seat0
[769] stack_resolve seat=1 source=Myrel, Shield of Argive target=seat0
[770] trigger_evaluated seat=1 source=Myrel, Shield of Argive
[771] stack_push seat=1 source=Myrel, Shield of Argive target=seat0
[772] triggered_ability seat=1 source=Myrel, Shield of Argive target=seat0
[773] priority_pass seat=2 source= target=seat0
[774] priority_pass seat=3 source= target=seat0
[775] priority_pass seat=0 source= target=seat0
[776] stack_resolve seat=1 source=Myrel, Shield of Argive target=seat0
[777] blockers seat=1 source= target=seat0
[778] damage seat=2 source=Faerie Macabre amount=2 target=seat1
[779] speed_advance seat=2 source= amount=3 target=seat0
[780] damage seat=2 source=Fire Nation Engineer amount=2 target=seat1
[781] damage seat=2 source=Endrek Sahr, Master Breeder amount=2 target=seat1
[782] phase_step seat=2 source= target=seat0
[783] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[784] state seat=2 source= target=seat0
```

</details>

#### Violation 29

- **Game**: 19 (seed 190043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 24, Phase=ending Step=cleanup
- **Commanders**: Ulamog, the Ceaseless Hunger, Myrel, Shield of Argive, Endrek Sahr, Master Breeder, Tourach, Dread Cantor
- **Message**: CardIdentity: card "Vampire Nocturnus Avatar" (ptr 0x36fdd4a8d320) appears in both seat 0 graveyard and seat 0 exile

<details>
<summary>Game State</summary>

```
Turn 24, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1045 events
  Seat 0 [alive]: life=40 library=81 hand=2 graveyard=10 exile=1 battlefield=6 cmdzone=1 mana=0
    - Trenchpost (P/T 0/0, dmg=0) [T]
    - Forsaken Crossroads (P/T 0/0, dmg=0) [T]
    - Sunscorched Desert (P/T 0/0, dmg=0) [T]
    - Iron Spider, Stark Upgrade (P/T 5/6, dmg=0) [T]
    - Ugin's Labyrinth (P/T 0/0, dmg=0) [T]
    - Spawning Bed (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=22 library=86 hand=3 graveyard=0 exile=0 battlefield=19 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Hookblade (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Crusade (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Gustcloak Cavalier (P/T 2/2, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Myr Matrix (P/T 0/0, dmg=0)
    - creature token soldier Token (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=28 library=85 hand=2 graveyard=3 exile=2 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Echoing Deeps (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Faerie Macabre (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Fire Nation Engineer (P/T 2/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Endrek Sahr, Master Breeder (P/T 2/2, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=40 library=82 hand=3 graveyard=4 exile=0 battlefield=7 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Tourach, Dread Cantor (P/T 2/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1025] zone_change seat=0 source=Fabled Passage
[1026] zone_change seat=0 source=Primal Storm
[1027] zone_change seat=0 source=Dion, Bahamut's Dominant // Dion, Bahamut's Dominant
[1028] zone_change seat=0 source=Strictly Better // Strictly Better (cont'd)
[1029] zone_change seat=0 source=Archaeological Dig
[1030] zone_change seat=0 source=Blank Card
[1031] zone_change seat=0 source=Big Apple, 3 a.m.
[1032] zone_change seat=0 source=Lightstall Inquisitor // Lightstall Inquisitor
[1033] zone_change seat=0 source=My Tendrils Run Deep
[1034] stack_push seat=0 source=Vampire Nocturnus Avatar target=seat0
[1035] priority_pass seat=1 source= target=seat0
[1036] priority_pass seat=2 source= target=seat0
[1037] priority_pass seat=3 source= target=seat0
[1038] stack_resolve seat=0 source=Vampire Nocturnus Avatar target=seat0
[1039] zone_change seat=0 source=Vampire Nocturnus Avatar
[1040] resolve seat=0 source=Vampire Nocturnus Avatar target=seat0
[1041] phase_step seat=0 source= target=seat0
[1042] phase_step seat=0 source= target=seat0
[1043] pool_drain seat=0 source= amount=1 target=seat0
[1044] state seat=0 source= target=seat0
```

</details>

#### Violation 30

- **Game**: 46 (seed 460043, perm 0)
- **Invariant**: CardIdentity
- **Turn**: 24, Phase=ending Step=cleanup
- **Commanders**: Phobos, Taeko, the Patient Avalanche, Toothy and Zndrsplt, Miles Morales // Ultimate Spider-Man
- **Message**: CardIdentity: card "Bhaal, Lord of Murder // Bhaal, Lord of Murder" (ptr 0x36fddd9f0240) appears in both seat 0 graveyard and seat 0 exile

<details>
<summary>Game State</summary>

```
Turn 24, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 758 events
  Seat 0 [alive]: life=33 library=77 hand=3 graveyard=10 exile=7 battlefield=6 cmdzone=0 mana=0
    - A-Thran Portal (P/T 0/0, dmg=0) [T]
    - Network Terminal (P/T 0/0, dmg=0) [T]
    - Phobos (P/T 3/2, dmg=0) [T]
    - Hellion Crucible (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Desert Cenote (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=37 library=82 hand=4 graveyard=2 exile=3 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Hostile Hostel // Creeping Inn (P/T 3/7, dmg=0) [T]
    - Phyrexian Splicer (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=27 library=79 hand=3 graveyard=2 exile=3 battlefield=9 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - The Everflowing Well // The Myriad Pools (P/T 0/0, dmg=0) [T]
    - Energy Field (P/T 0/0, dmg=0)
    - Spectral Adversary (P/T 2/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Toothy and Zndrsplt (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sword of War and Peace (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=30 library=79 hand=4 graveyard=1 exile=3 battlefield=9 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Battlegate Mimic (P/T 2/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Miles Morales // Ultimate Spider-Man (P/T 1/2, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Knowledge Pool (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[738] priority_pass seat=1 source= target=seat0
[739] priority_pass seat=2 source= target=seat0
[740] stack_resolve seat=3 source=Knowledge Pool target=seat0
[741] zone_change seat=0 source=Stalking Tiger Avatar
[742] zone_cast_grant_registered seat=0 source=Knowledge Pool target=seat0
[743] per_card_handler seat=0 source=Knowledge Pool target=seat0
[744] stack_push seat=0 source=Stalking Tiger Avatar target=seat0
[745] priority_pass seat=1 source= target=seat0
[746] priority_pass seat=2 source= target=seat0
[747] priority_pass seat=3 source= target=seat0
[748] stack_resolve seat=0 source=Stalking Tiger Avatar target=seat0
[749] zone_change seat=0 source=Stalking Tiger Avatar
[750] resolve seat=0 source=Stalking Tiger Avatar target=seat0
[751] phase_step seat=0 source= target=seat0
[752] declare_attackers seat=0 source= target=seat0
[753] blockers seat=1 source= target=seat0
[754] damage seat=0 source=Phobos amount=3 target=seat1
[755] phase_step seat=0 source= target=seat0
[756] pool_drain seat=0 source= amount=3 target=seat0
[757] state seat=0 source= target=seat0
```

</details>

#### Violation 31

- **Game**: 6 (seed 60043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 38, Phase=ending Step=cleanup
- **Commanders**: Oyobi, Who Split the Heavens, Invasion of Ikoria // Zilortha, Apex of Ikoria, Phenax, God of Deception, Mondrak, Glory Dominus
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 393, found 404) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 38, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2049 events
  Seat 0 [alive]: life=29 library=78 hand=1 graveyard=7 exile=6 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urborg, Tomb of Yawgmoth (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - A-Lantern of Revealing (P/T 0/0, dmg=0) [T]
    - Branch of Vitu-Ghazi (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Serum Powder (P/T 0/0, dmg=0)
    - Gerrard's Battle Cry (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=26 library=78 hand=0 graveyard=6 exile=6 battlefield=12 cmdzone=0 mana=0
    - A-Base Camp (P/T 0/0, dmg=0) [T]
    - The Mycosynth Gardens (P/T 0/0, dmg=0) [T]
    - Invasion of Ikoria // Zilortha, Apex of Ikoria (P/T 8/8, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Invigorating Boon (P/T 0/0, dmg=0)
    - Somberwald Dryad (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Argothian Treefolk (P/T 3/5, dmg=0) [T]
    - Thinking Cap (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Manaplasm (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=21 library=80 hand=6 graveyard=3 exile=2 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Phenax, God of Deception (P/T 4/7, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=27 library=47 hand=3 graveyard=35 exile=8 battlefield=7 cmdzone=1 mana=0
    - Urza's Power Plant (P/T 0/0, dmg=0) [T]
    - Mana Confluence (P/T 0/0, dmg=0) [T]
    - Ashnod's Battle Gear (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Morph (P/T 2/2, dmg=0) [T]
    - Locket of Yesterdays (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2029] zone_change seat=1 source=Forest
[2030] zone_change seat=1 source=The Paradise Bird
[2031] possibility_storm seat=1 source=Possibility Storm target=seat0
[2032] zone_cast_grant_registered seat=1 source=Possibility Storm target=seat0
[2033] zone_change seat=1 source=Forest
[2034] stack_push seat=1 source=Manaplasm target=seat0
[2035] priority_pass seat=2 source= target=seat0
[2036] priority_pass seat=3 source= target=seat0
[2037] priority_pass seat=0 source= target=seat0
[2038] stack_resolve seat=1 source=Manaplasm target=seat0
[2039] enter_battlefield seat=1 source=Manaplasm target=seat0
[2040] phase_step seat=1 source= target=seat0
[2041] declare_attackers seat=1 source= target=seat0
[2042] blockers seat=2 source= target=seat0
[2043] damage seat=1 source=Somberwald Dryad amount=2 target=seat2
[2044] damage seat=1 source=Argothian Treefolk amount=3 target=seat2
[2045] phase_step seat=1 source= target=seat0
[2046] pool_drain seat=1 source= amount=3 target=seat0
[2047] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[2048] state seat=1 source= target=seat0
```

</details>

#### Violation 32

- **Game**: 6 (seed 60043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 41, Phase=ending Step=cleanup
- **Commanders**: Oyobi, Who Split the Heavens, Invasion of Ikoria // Zilortha, Apex of Ikoria, Phenax, God of Deception, Mondrak, Glory Dominus
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 393, found 405) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 41, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2463 events
  Seat 0 [alive]: life=19 library=77 hand=1 graveyard=8 exile=7 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urborg, Tomb of Yawgmoth (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - A-Lantern of Revealing (P/T 0/0, dmg=0) [T]
    - Branch of Vitu-Ghazi (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Serum Powder (P/T 0/0, dmg=0)
    - Gerrard's Battle Cry (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=16 library=78 hand=0 graveyard=6 exile=6 battlefield=12 cmdzone=0 mana=0
    - A-Base Camp (P/T 0/0, dmg=0) [T]
    - The Mycosynth Gardens (P/T 0/0, dmg=0) [T]
    - Invasion of Ikoria // Zilortha, Apex of Ikoria (P/T 8/8, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Invigorating Boon (P/T 0/0, dmg=0)
    - Somberwald Dryad (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Argothian Treefolk (P/T 3/5, dmg=0) [T]
    - Thinking Cap (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Manaplasm (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=19 library=79 hand=6 graveyard=4 exile=2 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Phenax, God of Deception (P/T 4/7, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=17 library=39 hand=4 graveyard=42 exile=8 battlefield=7 cmdzone=1 mana=0
    - Urza's Power Plant (P/T 0/0, dmg=0) [T]
    - Mana Confluence (P/T 0/0, dmg=0) [T]
    - Ashnod's Battle Gear (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Morph (P/T 2/2, dmg=0) [T]
    - Locket of Yesterdays (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2443] zone_change seat=0 source=Prehistoric Pet
[2444] stack_push seat=0 source=Littjara target=seat0
[2445] priority_pass seat=1 source= target=seat0
[2446] priority_pass seat=2 source= target=seat0
[2447] priority_pass seat=3 source= target=seat0
[2448] stack_resolve seat=0 source=Littjara target=seat0
[2449] zone_change seat=0 source=Littjara
[2450] resolve seat=0 source=Littjara target=seat0
[2451] pay_mana seat=0 source=Gerrard's Battle Cry amount=3 target=seat0
[2452] activate_ability seat=0 source=Gerrard's Battle Cry target=seat0
[2453] stack_push seat=0 source=Gerrard's Battle Cry target=seat0
[2454] priority_pass seat=1 source= target=seat0
[2455] priority_pass seat=2 source= target=seat0
[2456] priority_pass seat=3 source= target=seat0
[2457] stack_resolve seat=0 source=Gerrard's Battle Cry target=seat0
[2458] buff seat=0 source=Gerrard's Battle Cry amount=1 target=seat0
[2459] activated_ability_resolved seat=0 source=Gerrard's Battle Cry target=seat0
[2460] phase_step seat=0 source= target=seat0
[2461] phase_step seat=0 source= target=seat0
[2462] state seat=0 source= target=seat0
```

</details>

#### Violation 33

- **Game**: 6 (seed 60043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 46, Phase=ending Step=cleanup
- **Commanders**: Oyobi, Who Split the Heavens, Invasion of Ikoria // Zilortha, Apex of Ikoria, Phenax, God of Deception, Mondrak, Glory Dominus
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 393, found 406) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 46, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2812 events
  Seat 0 [alive]: life=9 library=76 hand=1 graveyard=8 exile=7 battlefield=10 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urborg, Tomb of Yawgmoth (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - A-Lantern of Revealing (P/T 0/0, dmg=0) [T]
    - Branch of Vitu-Ghazi (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Serum Powder (P/T 0/0, dmg=0)
    - Gerrard's Battle Cry (P/T 0/0, dmg=0)
    - Rustic Clachan (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=6 library=75 hand=0 graveyard=7 exile=8 battlefield=13 cmdzone=0 mana=0
    - A-Base Camp (P/T 0/0, dmg=0) [T]
    - The Mycosynth Gardens (P/T 0/0, dmg=0) [T]
    - Invasion of Ikoria // Zilortha, Apex of Ikoria (P/T 8/8, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Invigorating Boon (P/T 0/0, dmg=0)
    - Somberwald Dryad (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Argothian Treefolk (P/T 3/5, dmg=0) [T]
    - Thinking Cap (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Manaplasm (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=5 library=78 hand=7 graveyard=5 exile=2 battlefield=7 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Phenax, God of Deception (P/T 4/7, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=7 library=31 hand=4 graveyard=49 exile=8 battlefield=8 cmdzone=1 mana=0
    - Urza's Power Plant (P/T 0/0, dmg=0) [T]
    - Mana Confluence (P/T 0/0, dmg=0) [T]
    - Ashnod's Battle Gear (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Morph (P/T 2/2, dmg=0) [T]
    - Locket of Yesterdays (P/T 0/0, dmg=0)
    - Sejiri Steppe (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2792] zone_change seat=1 source=Forest
[2793] zone_change seat=1 source=Forest
[2794] zone_change seat=1 source=Forest
[2795] stack_push seat=1 source=Eureka Moment // Eureka Moment target=seat0
[2796] priority_pass seat=2 source= target=seat0
[2797] priority_pass seat=3 source= target=seat0
[2798] priority_pass seat=0 source= target=seat0
[2799] stack_resolve seat=1 source=Eureka Moment // Eureka Moment target=seat0
[2800] zone_change seat=1 source=Eureka Moment // Eureka Moment
[2801] resolve seat=1 source=Eureka Moment // Eureka Moment target=seat0
[2802] phase_step seat=1 source= target=seat0
[2803] declare_attackers seat=1 source= target=seat0
[2804] blockers seat=2 source= target=seat0
[2805] damage seat=1 source=Somberwald Dryad amount=2 target=seat2
[2806] damage seat=1 source=Argothian Treefolk amount=3 target=seat2
[2807] damage seat=1 source=Manaplasm amount=1 target=seat2
[2808] phase_step seat=1 source= target=seat0
[2809] pool_drain seat=1 source= amount=7 target=seat0
[2810] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[2811] state seat=1 source= target=seat0
```

</details>

#### Violation 34

- **Game**: 6 (seed 60043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 47, Phase=beginning Step=upkeep
- **Commanders**: Oyobi, Who Split the Heavens, Invasion of Ikoria // Zilortha, Apex of Ikoria, Phenax, God of Deception, Mondrak, Glory Dominus
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 362, found 375) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 47, Phase=beginning Step=upkeep Active=seat2
Stack: 0 items, EventLog: 2849 events
  Seat 0 [LOST]: life=-1 library=76 hand=1 graveyard=8 exile=7 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-4 library=75 hand=0 graveyard=7 exile=8 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=5 library=78 hand=7 graveyard=6 exile=2 battlefield=6 cmdzone=0 mana=4
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Phenax, God of Deception (P/T 4/7, dmg=0)
    - Possibility Storm (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-3 library=31 hand=4 graveyard=49 exile=8 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2829] priority_pass seat=0 source= target=seat0
[2830] priority_pass seat=1 source= target=seat0
[2831] stack_resolve seat=2 source=Bolas's Citadel target=seat0
[2832] lose_life seat=2 source=Bolas's Citadel amount=10 target=seat0
[2833] life_change seat=0 source=Bolas's Citadel amount=-10 target=seat0
[2834] lose_life seat=2 source=Bolas's Citadel amount=10 target=seat1
[2835] life_change seat=1 source=Bolas's Citadel amount=-10 target=seat0
[2836] lose_life seat=2 source=Bolas's Citadel amount=10 target=seat3
[2837] life_change seat=3 source=Bolas's Citadel amount=-10 target=seat0
[2838] activated_ability_resolved seat=2 source=Bolas's Citadel target=seat0
[2839] sba_704_5a seat=0 source= amount=-1
[2840] sba_704_5a seat=1 source= amount=-4
[2841] sba_704_5a seat=3 source= amount=-3
[2842] battle_protector_assigned seat=1 source=Invasion of Ikoria // Zilortha, Apex of Ikoria target=seat2
[2843] sba_cycle_complete seat=-1 source=
[2844] seat_eliminated seat=0 source= amount=10
[2845] seat_eliminated seat=1 source= amount=13
[2846] seat_eliminated seat=3 source= amount=8
[2847] zone_cast_grant_expired seat=2 source=Bolas's Citadel target=seat0
[2848] game_end seat=2 source=
```

</details>

#### Violation 35

- **Game**: 22 (seed 220043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 43, Phase=ending Step=cleanup
- **Commanders**: Starke of Rath, Ezio Auditore da Firenze, Elrond, Master of Healing, Odric, Blood-Cursed
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 377, found 388) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 43, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1569 events
  Seat 0 [alive]: life=36 library=80 hand=5 graveyard=6 exile=5 battlefield=1 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=1 library=74 hand=5 graveyard=3 exile=11 battlefield=11 cmdzone=0 mana=0
    - Rakdos Guildgate (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ezio Auditore da Firenze (P/T 3/2, dmg=0) [T]
    - Captain's Claws (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blightwing Bandit (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Trained Condor (P/T 2/1, dmg=0) [T]
    - Geological Appraiser (P/T 3/2, dmg=0) [T]
    - Enraged Flamecaster (P/T 3/2, dmg=0)
    - Dungeon Shade (P/T 1/1, dmg=0)
  Seat 2 [LOST]: life=-2 library=76 hand=0 graveyard=2 exile=9 battlefield=0 cmdzone=0 mana=0
  Seat 3 [alive]: life=30 library=76 hand=3 graveyard=11 exile=4 battlefield=4 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dragon Egg (P/T 0/2, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1549] blockers seat=2 source= target=seat0
[1550] damage seat=1 source=Ezio Auditore da Firenze amount=3 target=seat2
[1551] trigger_fires seat=1 source=Ezio Auditore da Firenze amount=3 target=seat2
[1552] triggered_ability seat=1 source=Ezio Auditore da Firenze target=seat0
[1553] stack_push seat=1 source=Ezio Auditore da Firenze target=seat0
[1554] priority_pass seat=2 source= target=seat0
[1555] priority_pass seat=3 source= target=seat0
[1556] priority_pass seat=0 source= target=seat0
[1557] stack_resolve seat=1 source=Ezio Auditore da Firenze target=seat0
[1558] parsed_effect_residual seat=1 source=Ezio Auditore da Firenze target=seat0
[1559] damage seat=1 source=Blightwing Bandit amount=2 target=seat2
[1560] damage seat=1 source=Trained Condor amount=2 target=seat2
[1561] damage seat=1 source=Geological Appraiser amount=3 target=seat2
[1562] sba_704_5a seat=2 source= amount=-2
[1563] sba_cycle_complete seat=-1 source=
[1564] seat_eliminated seat=2 source= amount=16
[1565] phase_step seat=1 source= target=seat0
[1566] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[1567] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[1568] state seat=1 source= target=seat0
```

</details>

#### Violation 36

- **Game**: 22 (seed 220043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Starke of Rath, Ezio Auditore da Firenze, Elrond, Master of Healing, Odric, Blood-Cursed
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 376, found 387) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1930 events
  Seat 0 [LOST]: life=-2 library=77 hand=7 graveyard=7 exile=5 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=1 library=71 hand=6 graveyard=4 exile=11 battlefield=12 cmdzone=0 mana=0
    - Rakdos Guildgate (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ezio Auditore da Firenze (P/T 3/2, dmg=0) [T]
    - Captain's Claws (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blightwing Bandit (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Trained Condor (P/T 2/1, dmg=0) [T]
    - Geological Appraiser (P/T 3/2, dmg=0) [T]
    - Enraged Flamecaster (P/T 3/2, dmg=0) [T]
    - Dungeon Shade (P/T 1/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-2 library=76 hand=0 graveyard=2 exile=9 battlefield=0 cmdzone=0 mana=0
  Seat 3 [alive]: life=16 library=73 hand=3 graveyard=13 exile=4 battlefield=5 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Dragon Egg (P/T 0/2, dmg=0)
    - Vassal's Duty (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1910] grant_ability seat=0 source=Trained Condor target=seat0
[1911] blockers seat=0 source= target=seat0
[1912] damage seat=1 source=Ezio Auditore da Firenze amount=3 target=seat0
[1913] trigger_fires seat=1 source=Ezio Auditore da Firenze amount=3 target=seat0
[1914] triggered_ability seat=1 source=Ezio Auditore da Firenze target=seat0
[1915] stack_push seat=1 source=Ezio Auditore da Firenze target=seat0
[1916] priority_pass seat=3 source= target=seat0
[1917] priority_pass seat=0 source= target=seat0
[1918] stack_resolve seat=1 source=Ezio Auditore da Firenze target=seat0
[1919] parsed_effect_residual seat=1 source=Ezio Auditore da Firenze target=seat0
[1920] damage seat=1 source=Blightwing Bandit amount=2 target=seat0
[1921] damage seat=1 source=Trained Condor amount=2 target=seat0
[1922] damage seat=1 source=Geological Appraiser amount=3 target=seat0
[1923] damage seat=1 source=Enraged Flamecaster amount=3 target=seat0
[1924] damage seat=1 source=Dungeon Shade amount=6 target=seat0
[1925] sba_704_5a seat=0 source= amount=-2
[1926] sba_cycle_complete seat=-1 source=
[1927] seat_eliminated seat=0 source= amount=1
[1928] phase_step seat=1 source= target=seat0
[1929] state seat=1 source= target=seat0
```

</details>

#### Violation 37

- **Game**: 22 (seed 220043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 54, Phase=combat Step=end_of_combat
- **Commanders**: Starke of Rath, Ezio Auditore da Firenze, Elrond, Master of Healing, Odric, Blood-Cursed
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 372, found 383) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 54, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 2045 events
  Seat 0 [LOST]: life=-2 library=77 hand=7 graveyard=7 exile=5 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=1 library=70 hand=7 graveyard=4 exile=11 battlefield=12 cmdzone=0 mana=0
    - Rakdos Guildgate (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Ezio Auditore da Firenze (P/T 3/2, dmg=0) [T]
    - Captain's Claws (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Blightwing Bandit (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Trained Condor (P/T 2/1, dmg=0) [T]
    - Geological Appraiser (P/T 3/2, dmg=0) [T]
    - Enraged Flamecaster (P/T 3/2, dmg=0) [T]
    - Dungeon Shade (P/T 6/6, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-2 library=76 hand=0 graveyard=2 exile=9 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=0 library=72 hand=4 graveyard=14 exile=4 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2025] trigger_fires seat=1 source=Ezio Auditore da Firenze amount=3 target=seat3
[2026] triggered_ability seat=1 source=Ezio Auditore da Firenze target=seat0
[2027] stack_push seat=1 source=Ezio Auditore da Firenze target=seat0
[2028] priority_pass seat=3 source= target=seat0
[2029] stack_resolve seat=1 source=Ezio Auditore da Firenze target=seat0
[2030] parsed_effect_residual seat=1 source=Ezio Auditore da Firenze target=seat0
[2031] damage seat=1 source=Blightwing Bandit amount=2 target=seat3
[2032] damage seat=1 source=Trained Condor amount=2 target=seat3
[2033] damage seat=1 source=Geological Appraiser amount=2 target=seat3
[2034] damage seat=1 source=Enraged Flamecaster amount=3 target=seat3
[2035] damage seat=1 source=Dungeon Shade amount=6 target=seat3
[2036] sba_704_5a seat=3 source=
[2037] destroy seat=3 source=Dragon Egg
[2038] sba_704_5g seat=3 source=Dragon Egg
[2039] zone_change seat=3 source=Dragon Egg
[2040] triggered_ability seat=3 source=Dragon Egg target=seat0
[2041] pending_triggers_purged_on_leave seat=3 source= amount=1
[2042] seat_eliminated seat=3 source= amount=4
[2043] game_end seat=1 source=
[2044] sba_cycle_complete seat=-1 source=
```

</details>

#### Violation 38

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 396, found 407) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2282 events
  Seat 0 [alive]: life=18 library=60 hand=1 graveyard=12 exile=16 battlefield=14 cmdzone=0 mana=0
    - Kazuul's Fury // Kazuul's Cliffs (P/T 0/0, dmg=0) [T]
    - Shrine of the Forsaken Gods (P/T 0/0, dmg=0) [T]
    - Hunter's Blowgun (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mana Max, Afterburner (P/T 5/5, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Chandra, Awakened Inferno (P/T 0/6, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Earth Elemental (P/T 4/5, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Seer's Sundial (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=22 library=84 hand=7 graveyard=4 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=14 library=81 hand=4 graveyard=2 exile=5 battlefield=8 cmdzone=1 mana=0
    - Orzhova, the Church of Deals (P/T 0/0, dmg=0) [T]
    - Agatha's Soul Cauldron (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=26 library=79 hand=3 graveyard=6 exile=8 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2262] damage_wears_off seat=1 source=Vedalken Entrancer amount=3 target=seat0
[2263] damage_wears_off seat=1 source=Island amount=3 target=seat0
[2264] damage_wears_off seat=1 source=Island amount=3 target=seat0
[2265] damage_wears_off seat=2 source=Orzhova, the Church of Deals amount=3 target=seat0
[2266] damage_wears_off seat=2 source=Agatha's Soul Cauldron amount=3 target=seat0
[2267] damage_wears_off seat=2 source=Plains amount=3 target=seat0
[2268] damage_wears_off seat=2 source=Swamp amount=3 target=seat0
[2269] damage_wears_off seat=2 source=Swamp amount=3 target=seat0
[2270] damage_wears_off seat=2 source=Possibility Storm amount=3 target=seat0
[2271] damage_wears_off seat=2 source=Plains amount=3 target=seat0
[2272] damage_wears_off seat=2 source=Bolas's Citadel amount=3 target=seat0
[2273] damage_wears_off seat=3 source=Mountain amount=3 target=seat0
[2274] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[2275] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[2276] damage_wears_off seat=3 source=Swamp amount=3 target=seat0
[2277] damage_wears_off seat=3 source=Trinisphere amount=3 target=seat0
[2278] damage_wears_off seat=3 source=Mountain amount=3 target=seat0
[2279] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[2280] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[2281] state seat=0 source= target=seat0
```

</details>

#### Violation 39

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 35, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 396, found 408) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 35, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2424 events
  Seat 0 [alive]: life=8 library=58 hand=1 graveyard=14 exile=16 battlefield=13 cmdzone=1 mana=0
    - Kazuul's Fury // Kazuul's Cliffs (P/T 0/0, dmg=0) [T]
    - Shrine of the Forsaken Gods (P/T 0/0, dmg=0) [T]
    - Hunter's Blowgun (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Chandra, Awakened Inferno (P/T 0/6, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Earth Elemental (P/T 4/5, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Seer's Sundial (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=12 library=83 hand=7 graveyard=5 exile=0 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=14 library=79 hand=3 graveyard=1 exile=8 battlefield=10 cmdzone=1 mana=0
    - Agatha's Soul Cauldron (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wispweaver Angel (P/T 5/5, dmg=0)
    - Magnetic Snuffler (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=16 library=79 hand=3 graveyard=6 exile=8 battlefield=6 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2404] triggered_ability seat=2 source=Magnetic Snuffler target=seat0
[2405] reanimate seat=2 source=Wispweaver Angel target=seat0
[2406] stack_push seat=2 source=Agatha's Soul Cauldron target=seat0
[2407] stack_push seat=2 source=Magnetic Snuffler target=seat0
[2408] triggers_ordered seat=2 source= target=seat0
[2409] priority_pass seat=3 source= target=seat0
[2410] priority_pass seat=0 source= target=seat0
[2411] priority_pass seat=1 source= target=seat0
[2412] stack_resolve seat=2 source=Magnetic Snuffler target=seat0
[2413] priority_pass seat=3 source= target=seat0
[2414] priority_pass seat=0 source= target=seat0
[2415] priority_pass seat=1 source= target=seat0
[2416] stack_resolve seat=2 source=Agatha's Soul Cauldron target=seat0
[2417] counter_mod seat=0 source=Agatha's Soul Cauldron amount=1 target=seat0
[2418] sba_704_6d seat=0 source=Mana Max, Afterburner
[2419] sba_cycle_complete seat=-1 source=
[2420] phase_step seat=2 source= target=seat0
[2421] phase_step seat=2 source= target=seat0
[2422] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2423] state seat=2 source= target=seat0
```

</details>

#### Violation 40

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 36, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 396, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 36, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2502 events
  Seat 0 [alive]: life=8 library=55 hand=1 graveyard=14 exile=19 battlefield=13 cmdzone=1 mana=0
    - Kazuul's Fury // Kazuul's Cliffs (P/T 0/0, dmg=0) [T]
    - Shrine of the Forsaken Gods (P/T 0/0, dmg=0) [T]
    - Hunter's Blowgun (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Chandra, Awakened Inferno (P/T 0/6, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Earth Elemental (P/T 4/5, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Seer's Sundial (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=12 library=80 hand=7 graveyard=5 exile=3 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=14 library=76 hand=3 graveyard=1 exile=11 battlefield=10 cmdzone=1 mana=0
    - Agatha's Soul Cauldron (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wispweaver Angel (P/T 5/5, dmg=0)
    - Magnetic Snuffler (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=16 library=74 hand=2 graveyard=6 exile=13 battlefield=8 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Knowledge Pool (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2482] priority_pass seat=2 source= target=seat0
[2483] stack_resolve seat=3 source=Knowledge Pool target=seat0
[2484] enter_battlefield seat=3 source=Knowledge Pool target=seat0
[2485] zone_change seat=0 source=Skullmead Cauldron
[2486] zone_change seat=0 source=Bucolic Ranch
[2487] zone_change seat=0 source=For Each of You, a Gift
[2488] zone_change seat=1 source=Leveler
[2489] zone_change seat=1 source=Island
[2490] zone_change seat=1 source=Island
[2491] zone_change seat=2 source=Swamp
[2492] zone_change seat=2 source=Swamp
[2493] zone_change seat=2 source=Plains
[2494] zone_change seat=3 source=Mountain
[2495] zone_change seat=3 source=Mountain
[2496] zone_change seat=3 source=Swamp
[2497] per_card_handler seat=0 source=Knowledge Pool target=seat0
[2498] phase_step seat=3 source= target=seat0
[2499] phase_step seat=3 source= target=seat0
[2500] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[2501] state seat=3 source= target=seat0
```

</details>

#### Violation 41

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 39, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 380, found 394) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 39, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2769 events
  Seat 0 [LOST]: life=-2 library=52 hand=1 graveyard=16 exile=19 battlefield=0 cmdzone=0 mana=0
  Seat 1 [alive]: life=2 library=79 hand=7 graveyard=5 exile=3 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=10 library=74 hand=3 graveyard=1 exile=14 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wispweaver Angel (P/T 5/5, dmg=0) [T]
    - Underworld Sentinel (P/T 4/5, dmg=0)
  Seat 3 [alive]: life=1 library=74 hand=2 graveyard=6 exile=13 battlefield=8 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Knowledge Pool (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2749] priority_pass seat=1 source= target=seat0
[2750] stack_resolve seat=3 source=Knowledge Pool target=seat0
[2751] zone_change seat=2 source=Underworld Sentinel
[2752] zone_cast_grant_registered seat=2 source=Knowledge Pool target=seat0
[2753] zone_cast_grant_registered seat=2 source=Knowledge Pool target=seat0
[2754] zone_cast_grant_registered seat=2 source=Knowledge Pool target=seat0
[2755] per_card_handler seat=0 source=Knowledge Pool target=seat0
[2756] stack_push seat=2 source=Underworld Sentinel target=seat0
[2757] priority_pass seat=3 source= target=seat0
[2758] priority_pass seat=1 source= target=seat0
[2759] stack_resolve seat=2 source=Underworld Sentinel target=seat0
[2760] enter_battlefield seat=2 source=Underworld Sentinel target=seat0
[2761] phase_step seat=2 source= target=seat0
[2762] declare_attackers seat=2 source= target=seat0
[2763] blockers seat=3 source= target=seat0
[2764] damage seat=2 source=Wispweaver Angel amount=5 target=seat3
[2765] speed_advance seat=2 source= amount=4 target=seat0
[2766] phase_step seat=2 source= target=seat0
[2767] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2768] state seat=2 source= target=seat0
```

</details>

#### Violation 42

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 41, Phase=ending Step=cleanup
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: zone conservation suspicious: 15 extra real cards appeared (expected 380, found 395) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 41, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2871 events
  Seat 0 [LOST]: life=-2 library=52 hand=1 graveyard=16 exile=19 battlefield=0 cmdzone=0 mana=0
  Seat 1 [alive]: life=2 library=77 hand=7 graveyard=5 exile=5 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Vedalken Entrancer (P/T 1/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Quick-Draw Dagger (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=10 library=72 hand=3 graveyard=3 exile=14 battlefield=9 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wispweaver Angel (P/T 5/5, dmg=0) [T]
    - Underworld Sentinel (P/T 4/5, dmg=0)
  Seat 3 [alive]: life=1 library=73 hand=2 graveyard=6 exile=13 battlefield=10 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trinisphere (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Knowledge Pool (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lady Orca (P/T 7/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2851] zone_cast_grant_registered seat=1 source=Knowledge Pool target=seat0
[2852] per_card_handler seat=0 source=Knowledge Pool target=seat0
[2853] stack_push seat=1 source=Quick-Draw Dagger target=seat0
[2854] priority_pass seat=2 source= target=seat0
[2855] priority_pass seat=3 source= target=seat0
[2856] stack_resolve seat=1 source=Quick-Draw Dagger target=seat0
[2857] enter_battlefield seat=1 source=Quick-Draw Dagger target=seat0
[2858] triggered_ability seat=1 source=Quick-Draw Dagger target=seat0
[2859] stack_push seat=1 source=Quick-Draw Dagger target=seat0
[2860] triggers_ordered seat=1 source= target=seat0
[2861] priority_pass seat=2 source= target=seat0
[2862] priority_pass seat=3 source= target=seat0
[2863] stack_resolve seat=1 source=Quick-Draw Dagger target=seat0
[2864] modification_effect seat=1 source=Quick-Draw Dagger target=seat0
[2865] parser_gap seat=1 source=Quick-Draw Dagger target=seat0
[2866] draw seat=1 source=Hunted by The Family amount=1 target=seat0
[2867] phase_step seat=1 source= target=seat0
[2868] phase_step seat=1 source= target=seat0
[2869] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[2870] state seat=1 source= target=seat0
```

</details>

#### Violation 43

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 42, Phase=beginning Step=upkeep
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: zone conservation suspicious: 15 extra real cards appeared (expected 364, found 379) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 42, Phase=beginning Step=upkeep Active=seat2
Stack: 0 items, EventLog: 2908 events
  Seat 0 [LOST]: life=-2 library=52 hand=1 graveyard=16 exile=19 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-8 library=77 hand=7 graveyard=5 exile=5 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=10 library=72 hand=3 graveyard=4 exile=14 battlefield=8 cmdzone=1 mana=5
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wispweaver Angel (P/T 5/5, dmg=0)
    - Underworld Sentinel (P/T 4/5, dmg=0)
  Seat 3 [LOST]: life=-9 library=73 hand=2 graveyard=6 exile=13 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2888] stack_push seat=2 source=Bolas's Citadel target=seat0
[2889] priority_pass seat=3 source= target=seat0
[2890] priority_pass seat=1 source= target=seat0
[2891] stack_resolve seat=2 source=Bolas's Citadel target=seat0
[2892] lose_life seat=2 source=Bolas's Citadel amount=10 target=seat1
[2893] life_change seat=1 source=Bolas's Citadel amount=-10 target=seat0
[2894] lose_life seat=2 source=Bolas's Citadel amount=10 target=seat3
[2895] life_change seat=3 source=Bolas's Citadel amount=-10 target=seat0
[2896] activated_ability_resolved seat=2 source=Bolas's Citadel target=seat0
[2897] sba_704_5a seat=1 source= amount=-8
[2898] sba_704_5a seat=3 source= amount=-9
[2899] sba_cycle_complete seat=-1 source=
[2900] seat_eliminated seat=1 source= amount=6
[2901] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2902] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2903] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2904] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2905] seat_eliminated seat=3 source= amount=10
[2906] zone_cast_grant_expired seat=2 source=Bolas's Citadel target=seat0
[2907] game_end seat=2 source=
```

</details>

#### Violation 44

- **Game**: 25 (seed 250043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 53, Phase=ending Step=cleanup
- **Commanders**: Davros, Dalek Creator, Neriv, Crackling Vanguard, Carth the Lion, Albiorix, Goose Tyrant // Wild Goose Chase
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 379, found 390) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 53, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 3323 events
  Seat 0 [LOST]: life=15 library=76 hand=7 graveyard=5 exile=2 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=0 library=75 hand=4 graveyard=5 exile=7 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=3 library=47 hand=7 graveyard=10 exile=18 battlefield=24 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Demolition Field (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Journeyer's Kite (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Carth the Lion (P/T 3/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mercadian Atlas (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Galadhrim Brigade (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Fynn, the Fangbearer (P/T 1/3, dmg=0) [T]
    - Wirewood Herald (P/T 1/1, dmg=0) [T]
    - Snapping Sailback (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Meltstrider's Gear (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=5 library=75 hand=6 graveyard=6 exile=7 battlefield=8 cmdzone=0 mana=0
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Llanowar Vanguard (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Albiorix, Goose Tyrant // Wild Goose Chase (P/T 3/3, dmg=0) [T]
    - Path of Discovery (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[3303] stack_push seat=2 source=Fynn, the Fangbearer target=seat0
[3304] triggered_ability seat=2 source=Fynn, the Fangbearer target=seat0
[3305] priority_pass seat=3 source= target=seat0
[3306] priority_pass seat=0 source= target=seat0
[3307] stack_resolve seat=2 source=Fynn, the Fangbearer target=seat0
[3308] poison seat=2 source=Fynn, the Fangbearer amount=2 target=seat0
[3309] sba_704_5c seat=0 source= amount=10
[3310] sba_cycle_complete seat=-1 source=
[3311] seat_eliminated seat=0 source= amount=9
[3312] phase_step seat=2 source= target=seat0
[3313] triggered_ability seat=2 source=Mercadian Atlas target=seat0
[3314] stack_push seat=2 source=Mercadian Atlas target=seat0
[3315] triggers_ordered seat=2 source= target=seat0
[3316] priority_pass seat=3 source= target=seat0
[3317] stack_resolve seat=2 source=Mercadian Atlas target=seat0
[3318] zone_change seat=2 source=Infernal Sovereign
[3319] draw seat=2 source=Mercadian Atlas amount=1 target=seat2
[3320] pool_drain seat=2 source= amount=8 target=seat0
[3321] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[3322] state seat=2 source= target=seat0
```

</details>

#### Violation 45

- **Game**: 25 (seed 250043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 54, Phase=combat Step=end_of_combat
- **Commanders**: Davros, Dalek Creator, Neriv, Crackling Vanguard, Carth the Lion, Albiorix, Goose Tyrant // Wild Goose Chase
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 355, found 368) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 54, Phase=combat Step=end_of_combat Active=seat3
Stack: 0 items, EventLog: 3426 events
  Seat 0 [LOST]: life=15 library=76 hand=7 graveyard=5 exile=2 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=0 library=75 hand=4 graveyard=5 exile=7 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=0 library=44 hand=7 graveyard=10 exile=21 battlefield=0 cmdzone=0 mana=0
  Seat 3 [WON]: life=5 library=69 hand=4 graveyard=7 exile=14 battlefield=10 cmdzone=0 mana=0
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Llanowar Vanguard (P/T 1/5, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Albiorix, Goose Tyrant // Wild Goose Chase (P/T 3/3, dmg=0) [T]
    - Path of Discovery (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Knowledge Pool (P/T 0/0, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[3406] stack_resolve seat=3 source=Llanowar Vanguard target=seat0
[3407] buff seat=0 source=Llanowar Vanguard target=seat0
[3408] activated_ability_resolved seat=3 source=Llanowar Vanguard target=seat0
[3409] phase_step seat=3 source= target=seat0
[3410] declare_attackers seat=3 source= target=seat0
[3411] blockers seat=2 source= target=seat0
[3412] damage seat=3 source=Albiorix, Goose Tyrant // Wild Goose Chase amount=3 target=seat2
[3413] trigger_evaluated seat=2 source=Fynn, the Fangbearer
[3414] stack_push seat=2 source=Fynn, the Fangbearer target=seat0
[3415] triggered_ability seat=2 source=Fynn, the Fangbearer target=seat0
[3416] priority_pass seat=3 source= target=seat0
[3417] stack_resolve seat=2 source=Fynn, the Fangbearer target=seat0
[3418] sba_704_5a seat=2 source=
[3419] sba_cycle_complete seat=-1 source=
[3420] seat_eliminated seat=2 source= amount=24
[3421] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[3422] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3423] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3424] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[3425] game_end seat=3 source=
```

</details>

#### Violation 46

- **Game**: 39 (seed 390043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 59, Phase=combat Step=end_of_combat
- **Commanders**: Kathril, Aspect Warper, Raul, Trouble Shooter, Hunding Gjornersen, Voja, Jaws of the Conclave
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 363, found 374) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 59, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 2754 events
  Seat 0 [LOST]: life=-11 library=74 hand=5 graveyard=7 exile=4 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-3 library=75 hand=7 graveyard=4 exile=2 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=13 library=71 hand=0 graveyard=11 exile=10 battlefield=12 cmdzone=0 mana=3
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sentinels of Glen Elendra (P/T 2/3, dmg=0) [T]
    - Sliver Hive (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Hunding Gjornersen (P/T 5/4, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Hexplate Golem (P/T 5/7, dmg=0) [T]
    - Dream Prowler (P/T 1/5, dmg=0)
  Seat 3 [LOST]: life=-5 library=74 hand=6 graveyard=4 exile=6 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2734] zone_cast_grant_registered seat=2 source=Possibility Storm target=seat0
[2735] zone_change seat=2 source=Island
[2736] zone_change seat=2 source=Delusions of Mediocrity
[2737] zone_change seat=2 source=Expedition Healer // Expedition Healer
[2738] stack_push seat=2 source=Dream Prowler target=seat0
[2739] priority_pass seat=0 source= target=seat0
[2740] stack_resolve seat=2 source=Dream Prowler target=seat0
[2741] enter_battlefield seat=2 source=Dream Prowler target=seat0
[2742] phase_step seat=2 source= target=seat0
[2743] declare_attackers seat=2 source= target=seat0
[2744] blockers seat=0 source= target=seat0
[2745] damage seat=2 source=Sentinels of Glen Elendra amount=2 target=seat0
[2746] damage seat=2 source=Hunding Gjornersen amount=5 target=seat0
[2747] damage seat=2 source=Hexplate Golem amount=5 target=seat0
[2748] sba_704_5a seat=0 source= amount=-11
[2749] sba_cycle_complete seat=-1 source=
[2750] seat_eliminated seat=0 source= amount=10
[2751] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2752] zone_cast_grant_expired seat=2 source=Bolas's Citadel target=seat0
[2753] game_end seat=2 source=
```

</details>

#### Violation 47

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 37, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 11 extra real cards appeared (expected 395, found 406) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 37, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 1743 events
  Seat 0 [alive]: life=34 library=79 hand=3 graveyard=3 exile=4 battlefield=12 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Toucan-Puffin (P/T 2/2, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Vitu-Ghazi Guildmage (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=29 library=80 hand=5 graveyard=3 exile=4 battlefield=11 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
    - Spirited Simulacrum (P/T 2/1, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Hostage Taker (P/T 2/3, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=34 library=79 hand=0 graveyard=7 exile=6 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=12 library=79 hand=0 graveyard=4 exile=6 battlefield=13 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Alloy Golem (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Damage Control Crew (P/T 3/3, dmg=0)
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1723] damage seat=1 source=Hostage Taker amount=2 target=seat2
[1724] destroy seat=2 source=Morph
[1725] sba_704_5g seat=2 source=Morph
[1726] zone_change seat=2 source=Morph
[1727] trigger_evaluated seat=1 source=Hostage Taker
[1728] stack_push seat=1 source=Hostage Taker target=seat0
[1729] triggered_ability seat=1 source=Hostage Taker target=seat0
[1730] priority_pass seat=2 source= target=seat0
[1731] priority_pass seat=3 source= target=seat0
[1732] priority_pass seat=0 source= target=seat0
[1733] stack_resolve seat=1 source=Hostage Taker target=seat0
[1734] zone_change seat=3 source=Vorinclex, Monstrous Raider
[1735] exile_linked_returned seat=0 source=Hostage Taker amount=1 target=seat0
[1736] per_card_handler seat=0 source=Hostage Taker target=seat0
[1737] sba_cycle_complete seat=-1 source=
[1738] phase_step seat=2 source= target=seat0
[1739] pool_drain seat=2 source= amount=3 target=seat0
[1740] damage_wears_off seat=1 source=Hostage Taker amount=2 target=seat0
[1741] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[1742] state seat=2 source= target=seat0
```

</details>

#### Violation 48

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 40, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 395, found 407) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 40, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1992 events
  Seat 0 [alive]: life=31 library=78 hand=4 graveyard=4 exile=4 battlefield=12 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Toucan-Puffin (P/T 2/2, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - creature token green centaur Token (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=17 library=77 hand=6 graveyard=4 exile=6 battlefield=12 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Storyteller Pixie (P/T 3/3, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=34 library=79 hand=0 graveyard=7 exile=6 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [alive]: life=12 library=78 hand=0 graveyard=5 exile=6 battlefield=13 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Alloy Golem (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[1972] trigger_evaluated seat=1 source=Hostage Taker
[1973] stack_push seat=1 source=Hostage Taker target=seat0
[1974] triggered_ability seat=1 source=Hostage Taker target=seat0
[1975] priority_pass seat=2 source= target=seat0
[1976] priority_pass seat=3 source= target=seat0
[1977] priority_pass seat=0 source= target=seat0
[1978] stack_resolve seat=1 source=Hostage Taker target=seat0
[1979] stack_push seat=1 source=Spirited Simulacrum target=seat0
[1980] triggers_ordered seat=1 source= target=seat0
[1981] priority_pass seat=2 source= target=seat0
[1982] priority_pass seat=3 source= target=seat0
[1983] priority_pass seat=0 source= target=seat0
[1984] stack_resolve seat=1 source=Spirited Simulacrum target=seat0
[1985] zone_change seat=1 source=Iron Lance
[1986] seek seat=1 source=Spirited Simulacrum target=seat0
[1987] sba_cycle_complete seat=-1 source=
[1988] phase_step seat=1 source= target=seat0
[1989] damage_wears_off seat=0 source=creature token green centaur Token amount=2 target=seat0
[1990] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[1991] state seat=1 source= target=seat0
```

</details>

#### Violation 49

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 44, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 13 extra real cards appeared (expected 395, found 408) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 44, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2295 events
  Seat 0 [alive]: life=31 library=77 hand=5 graveyard=5 exile=4 battlefield=12 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=17 library=75 hand=6 graveyard=4 exile=8 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Storyteller Pixie (P/T 3/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Ugin's Construct (P/T 4/5, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=24 library=78 hand=0 graveyard=7 exile=6 battlefield=12 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0)
  Seat 3 [alive]: life=3 library=77 hand=1 graveyard=5 exile=6 battlefield=13 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Alloy Golem (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2275] priority_pass seat=3 source= target=seat0
[2276] priority_pass seat=0 source= target=seat0
[2277] stack_resolve seat=1 source=Ugin's Construct target=seat0
[2278] phase_step seat=1 source= target=seat0
[2279] declare_attackers seat=1 source= target=seat0
[2280] trigger_fires seat=1 source=Vv'viza, Orbital Overseer target=seat0
[2281] triggered_ability seat=1 source=Vv'viza, Orbital Overseer target=seat0
[2282] stack_push seat=1 source=Vv'viza, Orbital Overseer target=seat0
[2283] priority_pass seat=2 source= target=seat0
[2284] priority_pass seat=3 source= target=seat0
[2285] priority_pass seat=0 source= target=seat0
[2286] stack_resolve seat=1 source=Vv'viza, Orbital Overseer target=seat0
[2287] create_token seat=1 source=Vv'viza, Orbital Overseer amount=1 target=seat1
[2288] blockers seat=3 source= target=seat0
[2289] damage seat=1 source=Vv'viza, Orbital Overseer amount=4 target=seat3
[2290] damage seat=1 source=Hostage Taker amount=2 target=seat3
[2291] damage seat=1 source=Storyteller Pixie amount=3 target=seat3
[2292] phase_step seat=1 source= target=seat0
[2293] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[2294] state seat=1 source= target=seat0
```

</details>

#### Violation 50

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 45, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 14 extra real cards appeared (expected 395, found 409) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 45, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2387 events
  Seat 0 [alive]: life=31 library=77 hand=5 graveyard=5 exile=4 battlefield=12 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=17 library=75 hand=6 graveyard=4 exile=8 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Storyteller Pixie (P/T 3/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Ugin's Construct (P/T 4/5, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=24 library=76 hand=0 graveyard=7 exile=8 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=3 library=77 hand=1 graveyard=5 exile=6 battlefield=13 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Alloy Golem (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2367] stack_push seat=2 source=Entourage of Trest target=seat0
[2368] priority_pass seat=3 source= target=seat0
[2369] priority_pass seat=0 source= target=seat0
[2370] priority_pass seat=1 source= target=seat0
[2371] stack_resolve seat=2 source=Entourage of Trest target=seat0
[2372] enter_battlefield seat=2 source=Entourage of Trest target=seat0
[2373] triggered_ability seat=2 source=Entourage of Trest target=seat0
[2374] stack_push seat=2 source=Entourage of Trest target=seat0
[2375] triggers_ordered seat=2 source= target=seat0
[2376] priority_pass seat=3 source= target=seat0
[2377] priority_pass seat=0 source= target=seat0
[2378] priority_pass seat=1 source= target=seat0
[2379] stack_resolve seat=2 source=Entourage of Trest target=seat0
[2380] modification_effect seat=2 source=Entourage of Trest target=seat0
[2381] parser_gap seat=2 source=Entourage of Trest target=seat0
[2382] phase_step seat=2 source= target=seat0
[2383] phase_step seat=2 source= target=seat0
[2384] pool_drain seat=2 source= amount=1 target=seat0
[2385] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2386] state seat=2 source= target=seat0
```

</details>

#### Violation 51

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 46, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 16 extra real cards appeared (expected 395, found 411) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 46, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2536 events
  Seat 0 [alive]: life=31 library=77 hand=5 graveyard=5 exile=4 battlefield=12 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0)
  Seat 1 [alive]: life=11 library=75 hand=6 graveyard=4 exile=8 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Storyteller Pixie (P/T 3/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Ugin's Construct (P/T 4/5, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=24 library=76 hand=0 graveyard=7 exile=8 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=4 library=74 hand=0 graveyard=7 exile=10 battlefield=13 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Luxa River Shrine (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2516] damage seat=3 source=Alloy Golem amount=4 target=seat1
[2517] damage seat=3 source=Vorinclex, Monstrous Raider amount=6 target=seat1
[2518] damage seat=1 source=Ugin's Construct amount=4 target=seat3
[2519] destroy seat=3 source=Alloy Golem
[2520] sba_704_5g seat=3 source=Alloy Golem
[2521] zone_change seat=3 source=Alloy Golem
[2522] trigger_evaluated seat=1 source=Hostage Taker
[2523] stack_push seat=1 source=Hostage Taker target=seat0
[2524] triggered_ability seat=1 source=Hostage Taker target=seat0
[2525] priority_pass seat=3 source= target=seat0
[2526] priority_pass seat=0 source= target=seat0
[2527] priority_pass seat=2 source= target=seat0
[2528] stack_resolve seat=1 source=Hostage Taker target=seat0
[2529] sba_cycle_complete seat=-1 source=
[2530] phase_step seat=3 source= target=seat0
[2531] pool_drain seat=3 source= amount=4 target=seat0
[2532] damage_wears_off seat=1 source=Ugin's Construct amount=4 target=seat0
[2533] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[2534] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[2535] state seat=3 source= target=seat0
```

</details>

#### Violation 52

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 47, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 17 extra real cards appeared (expected 395, found 412) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 47, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2600 events
  Seat 0 [alive]: life=34 library=75 hand=4 graveyard=5 exile=6 battlefield=15 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Archaeological Dig (P/T 0/0, dmg=0) [T]
    - Shadow of the Second Sun (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=11 library=75 hand=6 graveyard=4 exile=8 battlefield=14 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Storyteller Pixie (P/T 3/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Ugin's Construct (P/T 4/5, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=21 library=76 hand=0 graveyard=7 exile=8 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=4 library=74 hand=0 graveyard=7 exile=10 battlefield=13 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Luxa River Shrine (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2580] zone_change seat=0 source=Shadow of the Second Sun
[2581] zone_change seat=0 source=Ajani's Welcome
[2582] possibility_storm seat=0 source=Possibility Storm target=seat0
[2583] zone_cast_grant_registered seat=0 source=Possibility Storm target=seat0
[2584] stack_push seat=0 source=Shadow of the Second Sun target=seat0
[2585] priority_pass seat=1 source= target=seat0
[2586] priority_pass seat=2 source= target=seat0
[2587] priority_pass seat=3 source= target=seat0
[2588] stack_resolve seat=0 source=Shadow of the Second Sun target=seat0
[2589] enter_battlefield seat=0 source=Shadow of the Second Sun target=seat0
[2590] per_card_handler seat=0 source=Shadow of the Second Sun target=seat0
[2591] per_card_handler seat=0 source=Shadow of the Second Sun target=seat0
[2592] phase_step seat=0 source= target=seat0
[2593] declare_attackers seat=0 source= target=seat0
[2594] blockers seat=2 source= target=seat0
[2595] damage seat=0 source=Kellan, the Kid amount=3 target=seat2
[2596] phase_step seat=0 source= target=seat0
[2597] pool_drain seat=0 source= amount=1 target=seat0
[2598] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[2599] state seat=0 source= target=seat0
```

</details>

#### Violation 53

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 48, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 18 extra real cards appeared (expected 395, found 413) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 48, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2691 events
  Seat 0 [alive]: life=34 library=75 hand=4 graveyard=5 exile=6 battlefield=15 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Archaeological Dig (P/T 0/0, dmg=0) [T]
    - Shadow of the Second Sun (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=11 library=73 hand=6 graveyard=5 exile=10 battlefield=16 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragonfire Blade (P/T 0/0, dmg=0)
    - Bilbo's Ring (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Vv'viza, Orbital Overseer (P/T 4/4, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
    - Storyteller Pixie (P/T 3/3, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Ugin's Construct (P/T 4/5, dmg=0) [T]
    - token lander Token (P/T 0/0, dmg=0)
    - Wall Crawl (P/T 0/0, dmg=0)
    - creature token spider Token (P/T 2/1, dmg=0)
    - token lander Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=10 library=76 hand=0 graveyard=7 exile=8 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0)
  Seat 3 [alive]: life=4 library=74 hand=0 graveyard=7 exile=10 battlefield=13 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Luxa River Shrine (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2671] priority_pass seat=2 source= target=seat0
[2672] priority_pass seat=3 source= target=seat0
[2673] priority_pass seat=0 source= target=seat0
[2674] stack_resolve seat=1 source=Vv'viza, Orbital Overseer target=seat0
[2675] create_token seat=1 source=Vv'viza, Orbital Overseer amount=1 target=seat1
[2676] blockers seat=2 source= target=seat0
[2677] damage seat=1 source=Vv'viza, Orbital Overseer amount=4 target=seat2
[2678] damage seat=1 source=Hostage Taker amount=2 target=seat2
[2679] damage seat=1 source=Storyteller Pixie amount=3 target=seat2
[2680] damage seat=1 source=Ugin's Construct amount=4 target=seat2
[2681] damage seat=2 source=Entourage of Trest amount=4 target=seat1
[2682] destroy seat=1 source=Hostage Taker
[2683] sba_704_5g seat=1 source=Hostage Taker
[2684] zone_change seat=1 source=Hostage Taker
[2685] zone_cast_grant_expired seat=1 source=Hostage Taker target=seat0
[2686] sba_cycle_complete seat=-1 source=
[2687] phase_step seat=1 source= target=seat0
[2688] damage_wears_off seat=2 source=Entourage of Trest amount=2 target=seat0
[2689] zone_cast_grant_expired seat=1 source=Possibility Storm target=seat0
[2690] state seat=1 source= target=seat0
```

</details>

#### Violation 54

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 50, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 19 extra real cards appeared (expected 385, found 404) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 50, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2839 events
  Seat 0 [alive]: life=34 library=75 hand=4 graveyard=5 exile=6 battlefield=15 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Archaeological Dig (P/T 0/0, dmg=0) [T]
    - Shadow of the Second Sun (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=1 library=73 hand=6 graveyard=5 exile=10 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=10 library=75 hand=1 graveyard=7 exile=8 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0) [T]
  Seat 3 [alive]: life=5 library=72 hand=0 graveyard=7 exile=12 battlefield=14 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Luxa River Shrine (P/T 0/0, dmg=0) [T]
    - Spider-Man, Brooklyn Visionary (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2819] enter_battlefield seat=3 source=Spider-Man, Brooklyn Visionary target=seat0
[2820] triggered_ability seat=3 source=Spider-Man, Brooklyn Visionary target=seat0
[2821] stack_push seat=3 source=Spider-Man, Brooklyn Visionary target=seat0
[2822] triggers_ordered seat=3 source= target=seat0
[2823] priority_pass seat=0 source= target=seat0
[2824] priority_pass seat=1 source= target=seat0
[2825] priority_pass seat=2 source= target=seat0
[2826] stack_resolve seat=3 source=Spider-Man, Brooklyn Visionary target=seat0
[2827] tutor seat=3 source=generic_tutor target=seat0
[2828] phase_step seat=3 source= target=seat0
[2829] declare_attackers seat=3 source= target=seat0
[2830] blockers seat=1 source= target=seat0
[2831] damage seat=3 source=Vorinclex, Monstrous Raider amount=6 target=seat1
[2832] sba_704_6c seat=1 source=Vorinclex, Monstrous Raider amount=24
[2833] sba_cycle_complete seat=-1 source=
[2834] seat_eliminated seat=1 source= amount=16
[2835] phase_step seat=3 source= target=seat0
[2836] pool_drain seat=3 source= amount=4 target=seat0
[2837] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[2838] state seat=3 source= target=seat0
```

</details>

#### Violation 55

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 20 extra real cards appeared (expected 385, found 405) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2959 events
  Seat 0 [alive]: life=33 library=74 hand=4 graveyard=5 exile=6 battlefield=17 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Archaeological Dig (P/T 0/0, dmg=0) [T]
    - Shadow of the Second Sun (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=1 library=73 hand=6 graveyard=5 exile=10 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=7 library=73 hand=1 graveyard=7 exile=10 battlefield=14 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0) [T]
    - Keeper of Fables (P/T 4/5, dmg=0)
  Seat 3 [alive]: life=5 library=72 hand=0 graveyard=7 exile=12 battlefield=14 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Luxa River Shrine (P/T 0/0, dmg=0) [T]
    - Spider-Man, Brooklyn Visionary (P/T 4/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[2939] trigger_evaluated seat=0 source=Kellan, the Kid
[2940] stack_push seat=0 source=Kellan, the Kid target=seat0
[2941] triggered_ability seat=0 source=Kellan, the Kid target=seat0
[2942] priority_pass seat=2 source= target=seat0
[2943] priority_pass seat=3 source= target=seat0
[2944] stack_resolve seat=0 source=Kellan, the Kid target=seat0
[2945] stack_push seat=2 source=Keeper of Fables target=seat0
[2946] priority_pass seat=3 source= target=seat0
[2947] priority_pass seat=0 source= target=seat0
[2948] stack_resolve seat=2 source=Keeper of Fables target=seat0
[2949] enter_battlefield seat=2 source=Keeper of Fables target=seat0
[2950] phase_step seat=2 source= target=seat0
[2951] declare_attackers seat=2 source= target=seat0
[2952] blockers seat=0 source= target=seat0
[2953] damage seat=2 source=Entourage of Trest amount=4 target=seat0
[2954] speed_advance seat=2 source= amount=3 target=seat0
[2955] phase_step seat=2 source= target=seat0
[2956] pool_drain seat=2 source= amount=1 target=seat0
[2957] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2958] state seat=2 source= target=seat0
```

</details>

#### Violation 56

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 53, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 21 extra real cards appeared (expected 385, found 406) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 53, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 3152 events
  Seat 0 [alive]: life=33 library=74 hand=4 graveyard=5 exile=6 battlefield=17 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Archaeological Dig (P/T 0/0, dmg=0) [T]
    - Shadow of the Second Sun (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=1 library=73 hand=6 graveyard=5 exile=10 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=2 library=72 hand=2 graveyard=8 exile=10 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0) [T]
  Seat 3 [alive]: life=6 library=70 hand=0 graveyard=8 exile=14 battlefield=14 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
    - Terramorphic Expanse (P/T 0/0, dmg=0) [T]
    - Windswept Heath (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Elvish Branchbender (P/T 2/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Vorinclex, Monstrous Raider (P/T 6/6, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Luxa River Shrine (P/T 0/0, dmg=0) [T]
    - Spider-Man, Brooklyn Visionary (P/T 4/3, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[3132] damage seat=3 source=Vorinclex, Monstrous Raider amount=1 target=seat2
[3133] damage seat=3 source=Spider-Man, Brooklyn Visionary amount=4 target=seat2
[3134] damage seat=2 source=Keeper of Fables amount=4 target=seat3
[3135] trigger_fires seat=2 source=Keeper of Fables amount=4 target=seat3
[3136] triggered_ability seat=2 source=Keeper of Fables target=seat0
[3137] stack_push seat=2 source=Keeper of Fables target=seat0
[3138] priority_pass seat=3 source= target=seat0
[3139] priority_pass seat=0 source= target=seat0
[3140] stack_resolve seat=2 source=Keeper of Fables target=seat0
[3141] zone_change seat=2 source=Forest
[3142] draw seat=2 source=Keeper of Fables amount=1 target=seat2
[3143] destroy seat=2 source=Keeper of Fables
[3144] sba_704_5g seat=2 source=Keeper of Fables
[3145] zone_change seat=2 source=Keeper of Fables
[3146] sba_cycle_complete seat=-1 source=
[3147] phase_step seat=3 source= target=seat0
[3148] pool_drain seat=3 source= amount=7 target=seat0
[3149] damage_wears_off seat=3 source=Vorinclex, Monstrous Raider amount=4 target=seat0
[3150] zone_cast_grant_expired seat=3 source=Possibility Storm target=seat0
[3151] state seat=3 source= target=seat0
```

</details>

#### Violation 57

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 54, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 21 extra real cards appeared (expected 371, found 392) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 54, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3190 events
  Seat 0 [alive]: life=36 library=73 hand=5 graveyard=5 exile=6 battlefield=18 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Archaeological Dig (P/T 0/0, dmg=0) [T]
    - Shadow of the Second Sun (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=1 library=73 hand=6 graveyard=5 exile=10 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=2 library=72 hand=2 graveyard=8 exile=10 battlefield=13 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0) [T]
  Seat 3 [LOST]: life=3 library=70 hand=0 graveyard=8 exile=14 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3170] add_mana seat=0 source=Plains amount=1 target=seat0
[3171] tap seat=0 source=Boxing Ring target=seat0
[3172] activate_ability seat=0 source=Boxing Ring target=seat0
[3173] stack_push seat=0 source=Boxing Ring target=seat0
[3174] priority_pass seat=2 source= target=seat0
[3175] priority_pass seat=3 source= target=seat0
[3176] stack_resolve seat=0 source=Boxing Ring target=seat0
[3177] create_token seat=0 source=Boxing Ring amount=1 target=seat0
[3178] activated_ability_resolved seat=0 source=Boxing Ring target=seat0
[3179] draw seat=0 source=Mystic Forge amount=1 target=seat0
[3180] phase_step seat=0 source= target=seat0
[3181] declare_attackers seat=0 source= target=seat0
[3182] blockers seat=3 source= target=seat0
[3183] damage seat=0 source=Kellan, the Kid amount=3 target=seat3
[3184] sba_704_6c seat=3 source=Kellan, the Kid amount=21
[3185] sba_cycle_complete seat=-1 source=
[3186] seat_eliminated seat=3 source= amount=14
[3187] phase_step seat=0 source= target=seat0
[3188] pool_drain seat=0 source= amount=8 target=seat0
[3189] state seat=0 source= target=seat0
```

</details>

#### Violation 58

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 55, Phase=ending Step=cleanup
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 22 extra real cards appeared (expected 371, found 393) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 55, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 3271 events
  Seat 0 [alive]: life=32 library=73 hand=5 graveyard=5 exile=6 battlefield=18 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Archaeological Dig (P/T 0/0, dmg=0) [T]
    - Shadow of the Second Sun (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=1 library=73 hand=6 graveyard=5 exile=10 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=2 library=70 hand=1 graveyard=8 exile=12 battlefield=15 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Transmogrifying Licid (P/T 2/2, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - The Huntsman's Redemption (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Thought Dissector (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Budoka Gardener // Dokai, Weaver of Life (P/T 2/1, dmg=0) [T]
    - Entourage of Trest (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Awakener Druid (P/T 1/1, dmg=0)
  Seat 3 [LOST]: life=3 library=70 hand=0 graveyard=8 exile=14 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3251] stack_resolve seat=0 source=Kellan, the Kid target=seat0
[3252] stack_push seat=2 source=Awakener Druid target=seat0
[3253] priority_pass seat=0 source= target=seat0
[3254] stack_resolve seat=2 source=Awakener Druid target=seat0
[3255] enter_battlefield seat=2 source=Awakener Druid target=seat0
[3256] triggered_ability seat=2 source=Awakener Druid target=seat0
[3257] stack_push seat=2 source=Awakener Druid target=seat0
[3258] triggers_ordered seat=2 source= target=seat0
[3259] priority_pass seat=0 source= target=seat0
[3260] stack_resolve seat=2 source=Awakener Druid target=seat0
[3261] parsed_effect_residual seat=2 source=Awakener Druid target=seat0
[3262] phase_step seat=2 source= target=seat0
[3263] declare_attackers seat=2 source= target=seat0
[3264] blockers seat=0 source= target=seat0
[3265] damage seat=2 source=Entourage of Trest amount=4 target=seat0
[3266] speed_advance seat=2 source= amount=4 target=seat0
[3267] phase_step seat=2 source= target=seat0
[3268] pool_drain seat=2 source= amount=4 target=seat0
[3269] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[3270] state seat=2 source= target=seat0
```

</details>

#### Violation 59

- **Game**: 40 (seed 400043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 56, Phase=combat Step=end_of_combat
- **Commanders**: Kellan, the Kid, Vv'viza, Orbital Overseer, Budoka Gardener // Dokai, Weaver of Life, Vorinclex, Monstrous Raider
- **Message**: zone conservation suspicious: 22 extra real cards appeared (expected 356, found 378) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 56, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 3306 events
  Seat 0 [WON]: life=35 library=72 hand=6 graveyard=5 exile=6 battlefield=19 cmdzone=0 mana=8
    - Plains (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Archway Commons (P/T 0/0, dmg=0) [T]
    - Boxing Ring (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Kellan, the Kid (P/T 3/3, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Archaeological Dig (P/T 0/0, dmg=0) [T]
    - Shadow of the Second Sun (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - treasure artifact token Token (P/T 0/0, dmg=0)
    - treasure artifact token Token (P/T 0/0, dmg=0)
  Seat 1 [LOST]: life=1 library=73 hand=6 graveyard=5 exile=10 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=-1 library=70 hand=1 graveyard=8 exile=12 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=3 library=70 hand=0 graveyard=8 exile=14 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3286] add_mana seat=0 source=Archway Commons amount=1 target=seat0
[3287] add_mana seat=0 source=Island amount=1 target=seat0
[3288] add_mana seat=0 source=Archaeological Dig amount=1 target=seat0
[3289] add_mana seat=0 source=Plains amount=1 target=seat0
[3290] tap seat=0 source=Boxing Ring target=seat0
[3291] activate_ability seat=0 source=Boxing Ring target=seat0
[3292] stack_push seat=0 source=Boxing Ring target=seat0
[3293] priority_pass seat=2 source= target=seat0
[3294] stack_resolve seat=0 source=Boxing Ring target=seat0
[3295] create_token seat=0 source=Boxing Ring amount=1 target=seat0
[3296] activated_ability_resolved seat=0 source=Boxing Ring target=seat0
[3297] draw seat=0 source=Watchful Automaton amount=1 target=seat0
[3298] phase_step seat=0 source= target=seat0
[3299] declare_attackers seat=0 source= target=seat0
[3300] blockers seat=2 source= target=seat0
[3301] damage seat=0 source=Kellan, the Kid amount=3 target=seat2
[3302] sba_704_5a seat=2 source= amount=-1
[3303] sba_cycle_complete seat=-1 source=
[3304] seat_eliminated seat=2 source= amount=15
[3305] game_end seat=0 source=
```

</details>

#### Violation 60

- **Game**: 19 (seed 190043, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 35, Phase=ending Step=cleanup
- **Commanders**: Ulamog, the Ceaseless Hunger, Myrel, Shield of Argive, Endrek Sahr, Master Breeder, Tourach, Dread Cantor
- **Message**: zone conservation suspicious: 12 extra real cards appeared (expected 387, found 399) — possible copy bug

<details>
<summary>Game State</summary>

```
Turn 35, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3163 events
  Seat 0 [alive]: life=27 library=65 hand=6 graveyard=14 exile=10 battlefield=10 cmdzone=1 mana=0
    - Trenchpost (P/T 0/0, dmg=0) [T]
    - Forsaken Crossroads (P/T 0/0, dmg=0) [T]
    - Sunscorched Desert (P/T 0/0, dmg=0) [T]
    - Iron Spider, Stark Upgrade (P/T 8/9, dmg=0) [T]
    - Ugin's Labyrinth (P/T 0/0, dmg=0) [T]
    - Spawning Bed (P/T 0/0, dmg=0) [T]
    - Temple of the False God (P/T 0/0, dmg=0) [T]
    - A-Hall of Tagsin (P/T 0/0, dmg=0) [T]
    - Mouth of Ronom (P/T 0/0, dmg=0) [T]
    - Mirror Shield (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=2 library=82 hand=1 graveyard=1 exile=3 battlefield=109 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Hookblade (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Crusade (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Myrel, Shield of Argive (P/T 3/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Gustcloak Cavalier (P/T 2/2, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Ojer Taq, Deepest Foundation // Temple of Civilization (P/T 6/6, dmg=0) [T]
    - Myr Matrix (P/T 0/0, dmg=0)
    - creature token soldier Token (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - creature token colorless myr artifact Token (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Steadfast Cathar (P/T 2/1, dmg=0) [T]
    - creature token soldier Token (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - creature token colorless myr artifact Token (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - creature token soldier Token (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Soldier (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - creature token soldier Token (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
    - Soldier (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=3 library=79 hand=2 graveyard=7 exile=8 battlefield=8 cmdzone=0 mana=0
    - Echoing Deeps (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Faerie Macabre (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Fire Nation Engineer (P/T 2/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Endrek Sahr, Master Breeder (P/T 2/2, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
  Seat 3 [LOST]: life=-13 library=80 hand=3 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3143] triggered_ability seat=2 source=Possibility Storm target=seat0
[3144] priority_pass seat=0 source= target=seat0
[3145] priority_pass seat=1 source= target=seat0
[3146] stack_resolve seat=2 source=Possibility Storm target=seat0
[3147] zone_change seat=0 source=The Grim Captain // The Grim Captain
[3148] zone_change seat=0 source=Cemetery Protector // Cemetery Protector
[3149] possibility_storm seat=0 source=Possibility Storm target=seat0
[3150] zone_cast_grant_registered seat=0 source=Possibility Storm target=seat0
[3151] stack_push seat=0 source=The Grim Captain // The Grim Captain target=seat0
[3152] priority_pass seat=1 source= target=seat0
[3153] priority_pass seat=2 source= target=seat0
[3154] stack_resolve seat=0 source=The Grim Captain // The Grim Captain target=seat0
[3155] zone_change seat=0 source=The Grim Captain // The Grim Captain
[3156] resolve seat=0 source=The Grim Captain // The Grim Captain target=seat0
[3157] phase_step seat=0 source= target=seat0
[3158] phase_step seat=0 source= target=seat0
[3159] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[3160] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[3161] zone_cast_grant_expired seat=0 source=Possibility Storm target=seat0
[3162] state seat=0 source= target=seat0
```

</details>

#### Violation 61

- **Game**: 18 (seed 180043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 42, Phase=beginning Step=upkeep
- **Commanders**: Mana Max, Afterburner, Toothy, Imaginary Friend, Neva, Stalked by Nightmares, Lady Orca
- **Message**: ExileLinkageIntegrity: card "Skullmead Cauldron" in seat 0 exile is linked to source timestamp 57 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 42, Phase=beginning Step=upkeep Active=seat2
Stack: 0 items, EventLog: 2908 events
  Seat 0 [LOST]: life=-2 library=52 hand=1 graveyard=16 exile=19 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-8 library=77 hand=7 graveyard=5 exile=5 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=10 library=72 hand=3 graveyard=4 exile=14 battlefield=8 cmdzone=1 mana=5
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wispweaver Angel (P/T 5/5, dmg=0)
    - Underworld Sentinel (P/T 4/5, dmg=0)
  Seat 3 [LOST]: life=-9 library=73 hand=2 graveyard=6 exile=13 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2888] stack_push seat=2 source=Bolas's Citadel target=seat0
[2889] priority_pass seat=3 source= target=seat0
[2890] priority_pass seat=1 source= target=seat0
[2891] stack_resolve seat=2 source=Bolas's Citadel target=seat0
[2892] lose_life seat=2 source=Bolas's Citadel amount=10 target=seat1
[2893] life_change seat=1 source=Bolas's Citadel amount=-10 target=seat0
[2894] lose_life seat=2 source=Bolas's Citadel amount=10 target=seat3
[2895] life_change seat=3 source=Bolas's Citadel amount=-10 target=seat0
[2896] activated_ability_resolved seat=2 source=Bolas's Citadel target=seat0
[2897] sba_704_5a seat=1 source= amount=-8
[2898] sba_704_5a seat=3 source= amount=-9
[2899] sba_cycle_complete seat=-1 source=
[2900] seat_eliminated seat=1 source= amount=6
[2901] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2902] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2903] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2904] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2905] seat_eliminated seat=3 source= amount=10
[2906] zone_cast_grant_expired seat=2 source=Bolas's Citadel target=seat0
[2907] game_end seat=2 source=
```

</details>

#### Violation 62

- **Game**: 60 (seed 600043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 33, Phase=ending Step=cleanup
- **Commanders**: Bebop & Rocksteady, Green Goblin, Revenant, Balan, Wandering Knight, Wilson, Urbane Bear
- **Message**: ExileLinkageIntegrity: card "Tromell, Seymour's Butler" in seat 0 exile is linked to source timestamp 49 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 33, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1357 events
  Seat 0 [alive]: life=24 library=77 hand=4 graveyard=5 exile=3 battlefield=9 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Bebop & Rocksteady (P/T 7/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Disciple of Phenax (P/T 1/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cabal Coffers (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=13 library=85 hand=0 graveyard=5 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 2 [alive]: life=31 library=80 hand=5 graveyard=5 exile=3 battlefield=6 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mishra's Factory (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=17 library=79 hand=1 graveyard=4 exile=7 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1337] damage seat=0 source=Bebop & Rocksteady amount=7 target=seat3
[1338] damage seat=0 source=Disciple of Phenax amount=1 target=seat3
[1339] damage seat=3 source=Cleric of the Forward Order amount=2 target=seat0
[1340] sba_704_6c seat=3 source=Bebop & Rocksteady amount=21
[1341] sba_cycle_complete seat=-1 source=
[1342] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1343] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1344] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1345] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1346] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1347] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1348] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1349] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1350] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1351] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1352] seat_eliminated seat=3 source= amount=11
[1353] phase_step seat=0 source= target=seat0
[1354] pool_drain seat=0 source= amount=7 target=seat0
[1355] damage_wears_off seat=0 source=Disciple of Phenax amount=2 target=seat0
[1356] state seat=0 source= target=seat0
```

</details>

#### Violation 63

- **Game**: 55 (seed 550043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 21, Phase=ending Step=cleanup
- **Commanders**: Éowyn, Fearless Knight, Kethek, Crucible Goliath, Narset, Enlightened Exile, Urza, Powerstone Prodigy
- **Message**: ExileLinkageIntegrity: card "Pulsemage Advocate" in seat 0 exile is linked to source timestamp 28 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 21, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 939 events
  Seat 0 [alive]: life=40 library=86 hand=6 graveyard=2 exile=1 battlefield=3 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=37 library=85 hand=5 graveyard=1 exile=0 battlefield=8 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Turtle Lair (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Manalith (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Kethek, Crucible Goliath (P/T 4/4, dmg=0)
  Seat 2 [alive]: life=38 library=87 hand=4 graveyard=3 exile=0 battlefield=5 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Emissary of Soulfire (P/T 1/4, dmg=0) [T]
    - Siege Veteran (P/T 2/2, dmg=0)
  Seat 3 [alive]: life=40 library=83 hand=0 graveyard=9 exile=0 battlefield=8 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Urza, Powerstone Prodigy (P/T 1/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plated Seastrider (P/T 1/4, dmg=0) [T]
    - Fishing Pole (P/T 0/0, dmg=0) [T]
    - Mechanical Glider (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[919] phase_step seat=1 source= target=seat0
[920] priority_pass seat=2 source= target=seat0
[921] priority_pass seat=3 source= target=seat0
[922] priority_pass seat=0 source= target=seat0
[923] priority_pass seat=2 source= target=seat0
[924] priority_pass seat=3 source= target=seat0
[925] priority_pass seat=0 source= target=seat0
[926] stack_resolve seat=1 source=Kethek, Crucible Goliath target=seat0
[927] enter_battlefield seat=1 source=Kethek, Crucible Goliath target=seat0
[928] triggered_ability seat=1 source=Kethek, Crucible Goliath target=seat0
[929] stack_push seat=1 source=Kethek, Crucible Goliath target=seat0
[930] triggers_ordered seat=1 source= target=seat0
[931] priority_pass seat=2 source= target=seat0
[932] priority_pass seat=3 source= target=seat0
[933] priority_pass seat=0 source= target=seat0
[934] stack_resolve seat=1 source=Kethek, Crucible Goliath target=seat0
[935] sacrifice seat=1 source=Hostage Taker target=seat1
[936] zone_cast_grant_expired seat=1 source=Hostage Taker target=seat0
[937] zone_change seat=1 source=Hostage Taker
[938] state seat=1 source= target=seat0
```

</details>

#### Violation 64

- **Game**: 67 (seed 670043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 35, Phase=ending Step=cleanup
- **Commanders**: Lae'zel, Illithid Thrall, Korvold, Fae-Cursed King, Prosper, Tome-Bound, Zo-Zu the Punisher
- **Message**: ExileLinkageIntegrity: card "Silent-Chant Zubera" in seat 0 exile is linked to source timestamp 36 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 35, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2000 events
  Seat 0 [alive]: life=30 library=82 hand=5 graveyard=4 exile=3 battlefield=4 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Lodestone Needle // Guidestone Compass (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Gold Mine (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=30 library=81 hand=7 graveyard=7 exile=0 battlefield=5 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Korvold, Fae-Cursed King (P/T 6/6, dmg=0)
  Seat 2 [alive]: life=31 library=83 hand=1 graveyard=3 exile=1 battlefield=10 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Bounty Board (P/T 0/0, dmg=0) [T]
    - Artifact Unknown Shores (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Sting-Slinger (P/T 3/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Screeching Harpy (P/T 2/2, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - The Black Gate (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=26 library=82 hand=2 graveyard=4 exile=0 battlefield=10 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Knollspine Invocation (P/T 0/0, dmg=0)
    - Everflowing Chalice (P/T 0/0, dmg=0) [T]
    - Door of Destinies (P/T 0/0, dmg=0)
    - Ancient Runes (P/T 0/0, dmg=0)
    - Arni Metalbrow (P/T 3/3, dmg=0) [T]
    - Glóin, Dwarf Emissary (P/T 3/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1980] triggers_ordered seat=1 source= target=seat0
[1981] priority_pass seat=2 source= target=seat0
[1982] priority_pass seat=3 source= target=seat0
[1983] priority_pass seat=0 source= target=seat0
[1984] stack_resolve seat=1 source=Korvold, Fae-Cursed King target=seat0
[1985] sacrifice seat=1 source=Swamp target=seat1
[1986] zone_change seat=1 source=Swamp
[1987] trigger_evaluated seat=1 source=Korvold, Fae-Cursed King
[1988] stack_push seat=1 source=Korvold, Fae-Cursed King target=seat0
[1989] triggered_ability seat=1 source=Korvold, Fae-Cursed King target=seat0
[1990] priority_pass seat=2 source= target=seat0
[1991] priority_pass seat=3 source= target=seat0
[1992] priority_pass seat=0 source= target=seat0
[1993] stack_resolve seat=1 source=Korvold, Fae-Cursed King target=seat0
[1994] zone_change seat=1 source=Clara Oswald
[1995] per_card_handler seat=0 source=Korvold, Fae-Cursed King target=seat0
[1996] zone_change seat=1 source=Clara Oswald
[1997] discard seat=1 source=Clara Oswald target=seat0
[1998] cleanup_loop seat=1 source= target=seat0
[1999] state seat=1 source= target=seat0
```

</details>

#### Violation 65

- **Game**: 103 (seed 1030043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 44, Phase=ending Step=cleanup
- **Commanders**: Emmara, Voice of the Conclave, Heartless Hidetsugu, Gwen Stacy // Ghost-Spider, Lu Bu, Master-at-Arms
- **Message**: ExileLinkageIntegrity: card "Primal Storm" in seat 0 exile is linked to source timestamp 49 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 44, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2457 events
  Seat 0 [alive]: life=17 library=75 hand=4 graveyard=2 exile=3 battlefield=13 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Crosswinds (P/T 0/0, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Skittering Surveyor (P/T 1/2, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Sylvan Echoes (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Emmara, Voice of the Conclave (P/T 2/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=15 library=78 hand=6 graveyard=5 exile=4 battlefield=7 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Sparring Collar (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Skitter of Lizards (P/T 2/2, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=19 library=77 hand=0 graveyard=10 exile=4 battlefield=9 cmdzone=0 mana=0
    - Mountain Valley (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Sequestered Stash (P/T 0/0, dmg=0) [T]
    - Gwen Stacy // Ghost-Spider (P/T 2/1, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Windstorm Drake (P/T 3/3, dmg=0)
  Seat 3 [LOST]: life=0 library=76 hand=4 graveyard=3 exile=5 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2437] pool_drain seat=2 source= amount=2 target=seat0
[2438] activate_ability seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2439] stack_push seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2440] priority_pass seat=0 source= target=seat0
[2441] priority_pass seat=1 source= target=seat0
[2442] stack_resolve seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2443] activated_ability_resolved seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2444] activate_ability seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2445] stack_push seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2446] priority_pass seat=0 source= target=seat0
[2447] priority_pass seat=1 source= target=seat0
[2448] stack_resolve seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2449] activated_ability_resolved seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2450] activate_ability seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2451] stack_push seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2452] priority_pass seat=0 source= target=seat0
[2453] priority_pass seat=1 source= target=seat0
[2454] stack_resolve seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2455] activated_ability_resolved seat=2 source=Gwen Stacy // Ghost-Spider target=seat0
[2456] state seat=2 source= target=seat0
```

</details>

#### Violation 66

- **Game**: 110 (seed 1100043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: M'Odo, the Gnarled Oracle, Optimus Prime, Hero // Optimus Prime, Autobot Leader, Lo and Li, Twin Tutors, Saint Traft and Rem Karolus
- **Message**: ExileLinkageIntegrity: card "Circuit Mender" in seat 0 exile is linked to source timestamp 52 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2625 events
  Seat 0 [alive]: life=20 library=74 hand=5 graveyard=7 exile=4 battlefield=9 cmdzone=0 mana=0
    - Vivid Creek (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - M'Odo, the Gnarled Oracle (P/T 0/3, dmg=0)
    - Welcome to . . . // Jurassic Park (P/T 0/0, dmg=0) [T]
    - Flailing Drake (P/T 2/3, dmg=0) [T]
    - Academy Ruins (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=7 library=75 hand=1 graveyard=4 exile=6 battlefield=16 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Benevolent Ancestor (P/T 0/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Optimus Prime, Hero // Optimus Prime, Autobot Leader (P/T 4/8, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Memorial to War (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Corrupted Crossroads (P/T 0/0, dmg=0) [T]
    - Hostage Taker (P/T 2/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=28 library=69 hand=5 graveyard=11 exile=8 battlefield=11 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Biotransference (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Nephalia Academy (P/T 0/0, dmg=0) [T]
    - Luxior, Giada's Gift (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Phyrexian Etchings (P/T 0/0, dmg=0)
    - Dire Mimic (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-1 library=74 hand=1 graveyard=7 exile=8 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2605] stack_resolve seat=1 source=Leashling target=seat0
[2606] enter_battlefield seat=1 source=Leashling target=seat0
[2607] activate_ability seat=1 source=Leashling target=seat0
[2608] stack_push seat=1 source=Leashling target=seat0
[2609] priority_pass seat=2 source= target=seat0
[2610] priority_pass seat=0 source= target=seat0
[2611] stack_resolve seat=1 source=Leashling target=seat0
[2612] bounce seat=1 source=Leashling target=seat1
[2613] zone_change seat=1 source=Leashling
[2614] trigger_evaluated seat=1 source=Leashling
[2615] trigger_evaluated seat=1 source=Hostage Taker
[2616] stack_push seat=1 source=Hostage Taker target=seat0
[2617] triggered_ability seat=1 source=Hostage Taker target=seat0
[2618] priority_pass seat=2 source= target=seat0
[2619] priority_pass seat=0 source= target=seat0
[2620] stack_resolve seat=1 source=Hostage Taker target=seat0
[2621] activated_ability_resolved seat=1 source=Leashling target=seat0
[2622] pool_drain seat=1 source= amount=1 target=seat0
[2623] damage_wears_off seat=1 source=Optimus Prime, Hero // Optimus Prime, Autobot Leader amount=2 target=seat0
[2624] state seat=1 source= target=seat0
```

</details>

#### Violation 67

- **Game**: 140 (seed 1400043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 53, Phase=ending Step=end
- **Commanders**: Feldon, Ronom Excavator, Kalamax, the Stormsire, Shiko and Narset, Unified, Missy
- **Message**: ExileLinkageIntegrity: card "Scorched Ruins" in seat 0 exile is linked to source timestamp 44 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 53, Phase=ending Step=end Active=seat1
Stack: 0 items, EventLog: 3588 events
  Seat 0 [LOST]: life=0 library=73 hand=5 graveyard=11 exile=7 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=19 library=74 hand=7 graveyard=6 exile=6 battlefield=9 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Evolution Vat (P/T 576495937749385216/576495937749385216, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Thorn Thallid (P/T 2/2, dmg=0) [T]
    - Kalamax, the Stormsire (P/T 4/4, dmg=0)
  Seat 2 [LOST]: life=0 library=75 hand=2 graveyard=6 exile=5 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=0 library=74 hand=1 graveyard=7 exile=4 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3568] activated_ability_resolved seat=1 source=Thorn Thallid target=seat0
[3569] sba_704_5a seat=3 source=
[3570] sba_cycle_complete seat=-1 source=
[3571] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3572] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[3573] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3574] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3575] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3576] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3577] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3578] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3579] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3580] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3581] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[3582] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[3583] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3584] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[3585] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[3586] seat_eliminated seat=3 source= amount=13
[3587] game_end seat=1 source=
```

</details>

#### Violation 68

- **Game**: 141 (seed 1410043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 51, Phase=ending Step=cleanup
- **Commanders**: Brigone, Soldier of Meletis, Rayne, Academy Chancellor, Tegan Jovanka, Patron of the Akki
- **Message**: ExileLinkageIntegrity: card "Crumbling Vestige" in seat 0 exile is linked to source timestamp 42 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 51, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2151 events
  Seat 0 [alive]: life=2 library=65 hand=5 graveyard=11 exile=7 battlefield=15 cmdzone=0 mana=0
    - Brigone, Soldier of Meletis (P/T 2/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Nurturing Pixie (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Urza's Cave (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Salvation Swan (P/T 3/3, dmg=0) [T]
    - Zealots en-Dal (P/T 2/4, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=18 library=75 hand=6 graveyard=6 exile=6 battlefield=9 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Glen Elendra Guardian (P/T 2/3, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Nuisance Engine (P/T 0/0, dmg=0) [T]
    - creature token colorless pest artifact Token (P/T 0/1, dmg=0)
  Seat 2 [alive]: life=18 library=75 hand=3 graveyard=9 exile=4 battlefield=9 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Maze of Ith (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Tegan Jovanka (P/T 2/2, dmg=0) [T]
    - Reinforced Bulwark (P/T 0/4, dmg=0) [T]
    - Opal Palace (P/T 0/0, dmg=0) [T]
    - Steppe Glider (P/T 2/4, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
    - Daily Bugle Building (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-4 library=76 hand=6 graveyard=5 exile=4 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2131] damage seat=0 source=Salvation Swan amount=3 target=seat3
[2132] damage seat=0 source=Zealots en-Dal amount=2 target=seat3
[2133] sba_704_5a seat=3 source= amount=-4
[2134] sba_cycle_complete seat=-1 source=
[2135] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2136] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2137] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2138] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2139] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2140] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2141] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2142] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2143] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2144] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2145] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2146] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2147] seat_eliminated seat=3 source= amount=10
[2148] phase_step seat=0 source= target=seat0
[2149] pool_drain seat=0 source= amount=9 target=seat0
[2150] state seat=0 source= target=seat0
```

</details>

#### Violation 69

- **Game**: 161 (seed 1610043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 46, Phase=ending Step=cleanup
- **Commanders**: Jacob Frye, The Fifteenth Doctor, Sarevok, Deadly Usurper, Y'shtola, Night's Blessed
- **Message**: ExileLinkageIntegrity: card "Swamp" in seat 0 exile is linked to source timestamp 48 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 46, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3630 events
  Seat 0 [alive]: life=5 library=66 hand=4 graveyard=9 exile=10 battlefield=20 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Prototype Portal (P/T 0/0, dmg=0) [T]
    - creature token colorless eldrazi scion Token (P/T 1/1, dmg=0) [T]
    - Kraken of the Straits (P/T 6/6, dmg=0) [T]
    - Boseiju, Who Shelters All (P/T 0/0, dmg=0) [T]
    - Kraken of the Straits (P/T 6/6, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Dread Reaper (P/T 6/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Phyrexian Prowler (P/T 3/3, dmg=0) [T]
    - Sage-Eye Avengers (P/T 4/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Rumbling Sentry (P/T 3/6, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Pith Driller (P/T 2/4, dmg=0)
    - Corrupt Official (P/T 3/1, dmg=0)
  Seat 1 [alive]: life=15 library=71 hand=4 graveyard=12 exile=6 battlefield=8 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Riptide Turtle (P/T 0/5, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ancient Tomb (P/T 0/0, dmg=0) [T]
    - Edgewall Inn (P/T 0/0, dmg=0) [T]
    - Ivory Tower (P/T 0/0, dmg=0)
    - Inquisitor's Flail (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=4 library=74 hand=3 graveyard=5 exile=5 battlefield=10 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Pit of Offerings (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-5 library=64 hand=7 graveyard=9 exile=8 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3610] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3611] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3612] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3613] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3614] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3615] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3616] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3617] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3618] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3619] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3620] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3621] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[3622] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[3623] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3624] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[3625] seat_eliminated seat=3 source= amount=17
[3626] phase_step seat=0 source= target=seat0
[3627] pool_drain seat=0 source= amount=2 target=seat0
[3628] damage_wears_off seat=0 source=Phyrexian Prowler amount=2 target=seat0
[3629] state seat=0 source= target=seat0
```

</details>

#### Violation 70

- **Game**: 185 (seed 1850043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 42, Phase=ending Step=cleanup
- **Commanders**: Slivdrazi Monstrosity, Galadriel, Light of Valinor, Tidus, Yuna's Guardian, Lucy MacLean, Positively Armed
- **Message**: ExileLinkageIntegrity: card "Plains" in seat 0 exile is linked to source timestamp 45 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 42, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 1463 events
  Seat 0 [alive]: life=10 library=76 hand=6 graveyard=3 exile=3 battlefield=10 cmdzone=0 mana=0
    - Arid Archway (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Hedgewitch's Mask (P/T 0/0, dmg=0)
    - Tolaria West (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Kavaron Skywarden (P/T 4/5, dmg=0) [T]
    - Dreadship Reef (P/T 0/0, dmg=0) [T]
    - Slivdrazi Monstrosity (P/T 8/8, dmg=0) [T]
    - Urza's Mine (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=31 library=79 hand=5 graveyard=2 exile=3 battlefield=10 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Oakhollow Village (P/T 0/0, dmg=0) [T]
    - Blast Zone (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Academy Rector (P/T 1/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Tomb of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Galadriel, Light of Valinor (P/T 3/3, dmg=0) [T]
  Seat 2 [alive]: life=36 library=79 hand=7 graveyard=4 exile=3 battlefield=5 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Rushwood Grove (P/T 0/0, dmg=0) [T]
    - Candles of Leng (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=3 library=75 hand=3 graveyard=4 exile=5 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1443] phase_step seat=0 source= target=seat0
[1444] declare_attackers seat=0 source= target=seat0
[1445] blockers seat=3 source= target=seat0
[1446] damage seat=0 source=Kavaron Skywarden amount=4 target=seat3
[1447] speed_advance seat=0 source= amount=4 target=seat0
[1448] damage seat=0 source=Slivdrazi Monstrosity amount=8 target=seat3
[1449] sba_704_6c seat=3 source=Slivdrazi Monstrosity amount=24
[1450] sba_cycle_complete seat=-1 source=
[1451] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1452] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1453] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1454] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1455] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1456] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1457] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1458] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1459] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1460] seat_eliminated seat=3 source= amount=13
[1461] phase_step seat=0 source= target=seat0
[1462] state seat=0 source= target=seat0
```

</details>

#### Violation 71

- **Game**: 196 (seed 1960043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 41, Phase=ending Step=cleanup
- **Commanders**: Breena, the Demagogue, Neerdiv, Devious Diver, Jason Bright, Glowing Prophet, April O'Neil, Human Element
- **Message**: ExileLinkageIntegrity: card "Plains" in seat 0 exile is linked to source timestamp 47 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 41, Phase=ending Step=cleanup Active=seat3
Stack: 0 items, EventLog: 2066 events
  Seat 0 [alive]: life=26 library=78 hand=3 graveyard=6 exile=5 battlefield=8 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Grixis Panorama (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=26 library=77 hand=5 graveyard=8 exile=6 battlefield=5 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Silundi Vision // Silundi Isle (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=28 library=78 hand=4 graveyard=8 exile=6 battlefield=6 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=7 library=74 hand=6 graveyard=6 exile=6 battlefield=7 cmdzone=1 mana=0
    - Crucible of the Spirit Dragon (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[2046] stack_push seat=3 source=Galecaster Colossus target=seat0
[2047] priority_pass seat=0 source= target=seat0
[2048] priority_pass seat=1 source= target=seat0
[2049] priority_pass seat=2 source= target=seat0
[2050] stack_resolve seat=3 source=Galecaster Colossus target=seat0
[2051] bounce seat=3 source=Galecaster Colossus target=seat3
[2052] zone_change seat=3 source=Dark Maze
[2053] trigger_evaluated seat=3 source=Dark Maze
[2054] activated_ability_resolved seat=3 source=Galecaster Colossus target=seat0
[2055] activate_ability seat=3 source=Galecaster Colossus target=seat0
[2056] stack_push seat=3 source=Galecaster Colossus target=seat0
[2057] priority_pass seat=0 source= target=seat0
[2058] priority_pass seat=1 source= target=seat0
[2059] priority_pass seat=2 source= target=seat0
[2060] stack_resolve seat=3 source=Galecaster Colossus target=seat0
[2061] bounce seat=3 source=Galecaster Colossus target=seat3
[2062] zone_change seat=3 source=Galecaster Colossus
[2063] trigger_evaluated seat=3 source=Galecaster Colossus
[2064] activated_ability_resolved seat=3 source=Galecaster Colossus target=seat0
[2065] state seat=3 source= target=seat0
```

</details>

#### Violation 72

- **Game**: 246 (seed 2460043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Vikya, Scorching Stalwart, Sheoldred, the Apocalypse, Prince Imrahil the Fair, Rashida Scalebane
- **Message**: ExileLinkageIntegrity: card "Split-Tail Miko" in seat 0 exile is linked to source timestamp 59 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2783 events
  Seat 0 [alive]: life=16 library=76 hand=0 graveyard=10 exile=3 battlefield=11 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Boros Guildgate (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Sun-Blessed Peak (P/T 0/0, dmg=0) [T]
    - Trampled Lotus (P/T 0/0, dmg=0)
    - Meteor Crater (P/T 0/0, dmg=0) [T]
    - Vikya, Scorching Stalwart (P/T 2/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Spirit en-Dal (P/T 2/1, dmg=0) [T]
  Seat 1 [alive]: life=19 library=70 hand=6 graveyard=7 exile=11 battlefield=8 cmdzone=0 mana=0
    - Throne of the High City (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cuombajj Witches (P/T 1/3, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Sheoldred, the Apocalypse (P/T 4/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=6 library=70 hand=0 graveyard=7 exile=11 battlefield=16 cmdzone=0 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Magus of the Future (P/T 5/3, dmg=0) [T]
    - Magus of the Future (P/T 5/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Hollowhenge Spirit (P/T 5/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Darksteel Monolith (P/T 0/0, dmg=0)
    - Prince Imrahil the Fair (P/T 5/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Graaz, Unstoppable Juggernaut (P/T 7/5, dmg=0)
  Seat 3 [LOST]: life=-7 library=71 hand=2 graveyard=8 exile=11 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2763] triggered_ability seat=3 source=Kjeldoran Elite Guard target=seat0
[2764] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2765] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2766] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2767] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2768] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2769] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2770] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2771] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2772] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2773] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2774] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2775] pending_triggers_purged_on_leave seat=3 source= amount=1
[2776] seat_eliminated seat=3 source= amount=11
[2777] sba_cycle_complete seat=-1 source=
[2778] phase_step seat=2 source= target=seat0
[2779] pool_drain seat=2 source= amount=1 target=seat0
[2780] damage_wears_off seat=2 source=Magus of the Future amount=2 target=seat0
[2781] zone_cast_grant_expired seat=2 source=Possibility Storm target=seat0
[2782] state seat=2 source= target=seat0
```

</details>

#### Violation 73

- **Game**: 256 (seed 2560043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 36, Phase=ending Step=cleanup
- **Commanders**: Marvin, Murderous Mimic, Gomif, Fast Racer, Grunn, the Lonely King, Baron Sengir
- **Message**: ExileLinkageIntegrity: card "Mountain" in seat 1 exile is linked to source timestamp 60 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 36, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 2161 events
  Seat 0 [LOST]: life=-22 library=84 hand=2 graveyard=2 exile=4 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=17 library=79 hand=6 graveyard=1 exile=5 battlefield=9 cmdzone=0 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Gomif, Fast Racer (P/T 0/2, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Baton of Morale (P/T 0/0, dmg=0)
    - Treasure Map // Treasure Cove (P/T 0/0, dmg=0) [T]
    - Deranged Whelp (P/T 2/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=22 library=78 hand=5 graveyard=2 exile=4 battlefield=10 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Swiftfoot Boots (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Camera Launcher (P/T 109/109, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Unnatural Growth (P/T 0/0, dmg=0)
    - The Tabernacle at Pendrell Vale (P/T 0/0, dmg=0) [T]
    - Grunn, the Lonely King (P/T 5/5, dmg=0)
  Seat 3 [LOST]: life=-68 library=77 hand=3 graveyard=7 exile=4 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2141] activated_ability_resolved seat=2 source=Camera Launcher target=seat0
[2142] activate_ability seat=2 source=Camera Launcher target=seat0
[2143] stack_push seat=2 source=Camera Launcher target=seat0
[2144] priority_pass seat=1 source= target=seat0
[2145] stack_resolve seat=2 source=Camera Launcher target=seat0
[2146] counter_mod seat=0 source=Camera Launcher amount=1 target=seat0
[2147] activated_ability_resolved seat=2 source=Camera Launcher target=seat0
[2148] activate_ability seat=2 source=Camera Launcher target=seat0
[2149] stack_push seat=2 source=Camera Launcher target=seat0
[2150] priority_pass seat=1 source= target=seat0
[2151] stack_resolve seat=2 source=Camera Launcher target=seat0
[2152] counter_mod seat=0 source=Camera Launcher amount=1 target=seat0
[2153] activated_ability_resolved seat=2 source=Camera Launcher target=seat0
[2154] activate_ability seat=2 source=Camera Launcher target=seat0
[2155] stack_push seat=2 source=Camera Launcher target=seat0
[2156] priority_pass seat=1 source= target=seat0
[2157] stack_resolve seat=2 source=Camera Launcher target=seat0
[2158] counter_mod seat=0 source=Camera Launcher amount=1 target=seat0
[2159] activated_ability_resolved seat=2 source=Camera Launcher target=seat0
[2160] state seat=2 source= target=seat0
```

</details>

#### Violation 74

- **Game**: 255 (seed 2550043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 55, Phase=ending Step=cleanup
- **Commanders**: Anafenza, the Foremost, Caelorna, Coral Tyrant, Mogis, God of Slaughter, Lynde, Cheerful Tormentor
- **Message**: ExileLinkageIntegrity: card "Warriors' Lesson" in seat 0 exile is linked to source timestamp 36 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 55, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 3084 events
  Seat 0 [alive]: life=3 library=74 hand=1 graveyard=3 exile=14 battlefield=16 cmdzone=0 mana=0
    - Skyclave Cleric // Skyclave Basilica (P/T 10/12, dmg=0) [T]
    - Temple of the False God (P/T 2/2, dmg=0) [T]
    - Deep Dish Pizza (P/T 2/2, dmg=0)
    - Ishgard, the Holy See // Faith & Grief (P/T 1/1, dmg=0) [T]
    - Forest (P/T 1/1, dmg=0) [T]
    - Anafenza, the Foremost (P/T 4/4, dmg=0) [T]
    - Foe-liage (P/T 4/4, dmg=0) [T]
    - Thopter Architect (P/T 2/3, dmg=0) [T]
    - Bloodthirsty Aerialist (P/T 2/3, dmg=0) [T]
    - Swamp (P/T 1/1, dmg=0) [T]
    - Grasping Longneck (P/T 4/2, dmg=0) [T]
    - Dream Devourer (P/T 0/3, dmg=0)
    - Forest (P/T 1/1, dmg=0) [T]
    - Apostle of Invasion (P/T 4/4, dmg=0) [T]
    - Tangle Golem (P/T 5/4, dmg=0) [T]
    - Plains (P/T 1/1, dmg=0) [T]
  Seat 1 [alive]: life=15 library=63 hand=1 graveyard=12 exile=10 battlefield=17 cmdzone=0 mana=0
    - Echoing Cavern (P/T 0/0, dmg=0) [T]
    - Scroll Rack (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Caelorna, Coral Tyrant (P/T 0/8, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Thought Courier (P/T 1/1, dmg=0) [T]
    - Dungeon Descent (P/T 0/0, dmg=0) [T]
    - Griffin Canyon (P/T 0/0, dmg=0) [T]
    - Maze of Shadows (P/T 0/0, dmg=0) [T]
    - Mox Opal (P/T 0/0, dmg=0) [T]
    - Brotherhood Vertibird (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Chandra's Dragonmech (P/T 4/4, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Fountain of Ichor (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=20 library=68 hand=7 graveyard=7 exile=8 battlefield=14 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Curse of Opulence (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Wirefly Hive (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Maddening Imp (P/T 1/1, dmg=0) [T]
    - Iron Maiden (P/T 0/0, dmg=0)
    - Shatterskull Smashing // Shatterskull, the Hammer Pass (P/T 0/0, dmg=0) [T]
    - Consulate Turret (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mogis, God of Slaughter (P/T 7/5, dmg=0)
  Seat 3 [LOST]: life=-1 library=71 hand=1 graveyard=8 exile=10 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3064] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3065] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3066] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3067] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3068] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[3069] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[3070] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[3071] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3072] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[3073] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3074] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3075] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3076] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[3077] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3078] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[3079] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3080] seat_eliminated seat=3 source= amount=14
[3081] phase_step seat=0 source= target=seat0
[3082] pool_drain seat=0 source= amount=7 target=seat0
[3083] state seat=0 source= target=seat0
```

</details>

#### Violation 75

- **Game**: 325 (seed 3250043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 51, Phase=beginning Step=upkeep
- **Commanders**: Sin, Spira's Punishment, Maximus, Knight Apparent, Tymaret, Chosen from Death, MJ, Rising Star
- **Message**: ExileLinkageIntegrity: card "Plague Reaver" in seat 0 exile is linked to source timestamp 72 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 51, Phase=beginning Step=upkeep Active=seat2
Stack: 0 items, EventLog: 4032 events
  Seat 0 [LOST]: life=-4 library=72 hand=0 graveyard=4 exile=11 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-1 library=75 hand=0 graveyard=9 exile=4 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=22 library=72 hand=2 graveyard=12 exile=11 battlefield=8 cmdzone=1 mana=5
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cryptic Spires (P/T 0/0, dmg=0) [T]
    - Dissection Tools (P/T 0/0, dmg=0)
    - Lead Golem (P/T 3/5, dmg=0) [T]
    - Crypt of Agadeem (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Spawning Pool (P/T 0/0, dmg=0) [T]
    - Canoptek Scarab Swarm (P/T 1/1, dmg=0)
  Seat 3 [LOST]: life=-9 library=71 hand=3 graveyard=7 exile=9 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[4012] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4013] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4014] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4015] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4016] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4017] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[4018] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4019] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4020] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4021] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4022] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[4023] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[4024] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4025] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4026] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4027] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4028] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4029] seat_eliminated seat=3 source= amount=13
[4030] zone_cast_grant_expired seat=2 source=Bolas's Citadel target=seat0
[4031] game_end seat=2 source=
```

</details>

#### Violation 76

- **Game**: 328 (seed 3280043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 52, Phase=ending Step=cleanup
- **Commanders**: Valgavoth, Terror Eater, Kozilek, the Broken Reality, Najeela, the Blade-Blossom, Moseo, Vein's New Dean
- **Message**: ExileLinkageIntegrity: card "Ghost Ark" in seat 0 exile is linked to source timestamp 54 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 52, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 2335 events
  Seat 0 [alive]: life=64 library=76 hand=3 graveyard=3 exile=5 battlefield=15 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Merrow Bonegnawer (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - A-Dungeon Descent (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Valgavoth, Terror Eater (P/T 9/9, dmg=0) [T]
    - Bog Wreckage (P/T 0/0, dmg=0) [T]
    - Fallen Cleric (P/T 4/2, dmg=0) [T]
    - Sandstone Oracle (P/T 4/4, dmg=0)
    - Curse of the Restless Dead (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=31 library=77 hand=3 graveyard=0 exile=16 battlefield=3 cmdzone=1 mana=0
    - Potatoes (P/T 0/0, dmg=0)
    - Mage-Ring Network (P/T 0/0, dmg=0) [T]
    - Arid Archway (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=15 library=75 hand=6 graveyard=5 exile=3 battlefield=12 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Niall Silvain (P/T 2/2, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Warrior Token (P/T 1/1, dmg=0) [T]
    - Warrior Token (P/T 1/1, dmg=0) [T]
  Seat 3 [LOST]: life=6 library=72 hand=0 graveyard=10 exile=4 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2315] declare_attackers seat=0 source= target=seat0
[2316] blockers seat=3 source= target=seat0
[2317] damage seat=0 source=Valgavoth, Terror Eater amount=9 target=seat3
[2318] speed_advance seat=0 source= amount=3 target=seat0
[2319] damage seat=0 source=Fallen Cleric amount=4 target=seat3
[2320] sba_704_6c seat=3 source=Valgavoth, Terror Eater amount=27
[2321] sba_cycle_complete seat=-1 source=
[2322] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2323] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2324] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2325] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2326] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2327] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2328] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2329] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2330] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2331] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2332] seat_eliminated seat=3 source= amount=11
[2333] phase_step seat=0 source= target=seat0
[2334] state seat=0 source= target=seat0
```

</details>

#### Violation 77

- **Game**: 324 (seed 3240043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 58, Phase=combat Step=end_of_combat
- **Commanders**: Vladimir and Godfrey, Myojin of Towering Might, Iroh, Grand Lotus, Averna, the Chaos Bloom
- **Message**: ExileLinkageIntegrity: card "Liu Bei, Lord of Shu" in seat 0 exile is linked to source timestamp 43 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 58, Phase=combat Step=end_of_combat Active=seat2
Stack: 0 items, EventLog: 4791 events
  Seat 0 [LOST]: life=-6 library=73 hand=3 graveyard=4 exile=6 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=-1 library=74 hand=0 graveyard=6 exile=5 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=17 library=71 hand=6 graveyard=5 exile=10 battlefield=13 cmdzone=0 mana=7
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Unstable Amulet (P/T 0/0, dmg=0) [T]
    - Tooth of Chiss-Goria (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Magus of the Future (P/T 2/3, dmg=0) [T]
    - Possibility Storm (P/T 0/0, dmg=0)
    - Wirewood Lodge (P/T 0/0, dmg=0) [T]
    - Iroh, Grand Lotus (P/T 5/5, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Old Gnawbone (P/T 7/7, dmg=0) [T]
  Seat 3 [LOST]: life=-10 library=71 hand=6 graveyard=6 exile=9 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[4771] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4772] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4773] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4774] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4775] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4776] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4777] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[4778] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4779] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[4780] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4781] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4782] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4783] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4784] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4785] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[4786] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[4787] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4788] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4789] seat_eliminated seat=3 source= amount=10
[4790] game_end seat=2 source=
```

</details>

#### Violation 78

- **Game**: 332 (seed 3320043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 24, Phase=ending Step=cleanup
- **Commanders**: Svyelun of Sea and Sky, Shagrat, Loot Bearer, Hei Bai, Forest Guardian, Tolsimir, Midnight's Light
- **Message**: ExileLinkageIntegrity: card "Bookwurm" in seat 2 exile is linked to source timestamp 31 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 24, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 736 events
  Seat 0 [alive]: life=31 library=86 hand=6 graveyard=4 exile=0 battlefield=3 cmdzone=1 mana=0
    - Island (P/T 0/0, dmg=0) [T]
    - Jade Monolith (P/T 0/0, dmg=0)
    - Bonders' Enclave (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=31 library=85 hand=2 graveyard=5 exile=0 battlefield=8 cmdzone=1 mana=0
    - The Monumental Facade (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Witch's Oven (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trespasser's Curse (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - food artifact token Token (P/T 0/0, dmg=0)
    - food artifact token Token (P/T 0/0, dmg=0)
  Seat 2 [alive]: life=39 library=84 hand=5 graveyard=3 exile=1 battlefield=5 cmdzone=1 mana=0
    - Wizards' School (P/T 0/0, dmg=0) [T]
    - Gladecover Scout (P/T 1/1, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=29 library=84 hand=2 graveyard=5 exile=0 battlefield=6 cmdzone=1 mana=0
    - Plaza of Harmony (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Gate to Manorborn (P/T 0/0, dmg=0) [T]
    - Druid of the Spade (P/T 2/3, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Clockwork Steed (P/T 0/3, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[716] zone_change seat=1 source=Hostage Taker
[717] activate_ability seat=1 source=Witch's Oven target=seat0
[718] stack_push seat=1 source=Witch's Oven target=seat0
[719] priority_pass seat=2 source= target=seat0
[720] priority_pass seat=3 source= target=seat0
[721] priority_pass seat=0 source= target=seat0
[722] stack_resolve seat=1 source=Witch's Oven target=seat0
[723] create_token seat=1 source=Witch's Oven amount=1 target=seat1
[724] activated_ability_resolved seat=1 source=Witch's Oven target=seat0
[725] phase_step seat=1 source= target=seat0
[726] phase_step seat=1 source= target=seat0
[727] trigger_fires seat=3 source=Clockwork Steed target=seat0
[728] triggered_ability seat=3 source=Clockwork Steed target=seat0
[729] stack_push seat=3 source=Clockwork Steed target=seat0
[730] priority_pass seat=1 source= target=seat0
[731] priority_pass seat=2 source= target=seat0
[732] priority_pass seat=0 source= target=seat0
[733] stack_resolve seat=3 source=Clockwork Steed target=seat0
[734] counter_mod seat=0 source=Clockwork Steed amount=1 target=seat0
[735] state seat=1 source= target=seat0
```

</details>

#### Violation 79

- **Game**: 351 (seed 3510043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 58, Phase=combat Step=end_of_combat
- **Commanders**: Karlach, Raging Tiefling, The Lady of Otaria, Sigarda, Heron's Grace, Ao, the Dawn Sky
- **Message**: ExileLinkageIntegrity: card "Invasion of Regatha // Disciples of the Inferno" in seat 0 exile is linked to source timestamp 71 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 58, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 2278 events
  Seat 0 [WON]: life=4 library=71 hand=4 graveyard=13 exile=5 battlefield=7 cmdzone=1 mana=2
    - Mountain (P/T 0/0, dmg=0) [T]
    - Thriving Bluff (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Path of Ancestry (P/T 0/0, dmg=0) [T]
    - Locthwain Gargoyle (P/T 2/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=-4 library=75 hand=5 graveyard=10 exile=3 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=0 library=79 hand=7 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=0 library=72 hand=6 graveyard=6 exile=3 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2258] stack_push seat=0 source=Magic Missile // Magic Missile target=seat0
[2259] priority_pass seat=3 source= target=seat0
[2260] stack_resolve seat=0 source=Magic Missile // Magic Missile target=seat0
[2261] zone_change seat=0 source=Magic Missile // Magic Missile
[2262] resolve seat=0 source=Magic Missile // Magic Missile target=seat0
[2263] phase_step seat=0 source= target=seat0
[2264] declare_attackers seat=0 source= target=seat0
[2265] blockers seat=3 source= target=seat0
[2266] damage seat=0 source=Locthwain Gargoyle amount=2 target=seat3
[2267] sba_704_5a seat=3 source=
[2268] sba_cycle_complete seat=-1 source=
[2269] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2270] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2271] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2272] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2273] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2274] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2275] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2276] seat_eliminated seat=3 source= amount=12
[2277] game_end seat=0 source=
```

</details>

#### Violation 80

- **Game**: 349 (seed 3490043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 34, Phase=ending Step=cleanup
- **Commanders**: Eshki Dragonclaw, Hanna, Ship's Navigator, Bushi Tenderfoot // Kenzo the Hardhearted, Vazi, Keen Negotiator
- **Message**: ExileLinkageIntegrity: card "Summon: Bahamut // Summon: Bahamut" in seat 0 exile is linked to source timestamp 36 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 34, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 1916 events
  Seat 0 [alive]: life=34 library=79 hand=7 graveyard=5 exile=5 battlefield=6 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Eshki Dragonclaw (P/T 6/6, dmg=0) [T]
    - Lynx (P/T 2/1, dmg=0)
  Seat 1 [alive]: life=34 library=79 hand=5 graveyard=4 exile=3 battlefield=8 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Avacynian Priest (P/T 1/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Samite Pilgrim (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Hanna, Ship's Navigator (P/T 1/2, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=38 library=80 hand=5 graveyard=6 exile=4 battlefield=4 cmdzone=0 mana=0
    - Flooded Strand (P/T 0/0, dmg=0) [T]
    - Bushi Tenderfoot // Kenzo the Hardhearted (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Gem Bazaar (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=32 library=79 hand=6 graveyard=2 exile=5 battlefield=9 cmdzone=1 mana=0
    - Rainbow Vale (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ground Seal (P/T 0/0, dmg=0)
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Dragon Trainer (P/T 1/1, dmg=0)
    - creature token dragon Token (P/T 4/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[1896] stack_resolve seat=1 source=Samite Pilgrim target=seat0
[1897] prevent seat=0 source=Samite Pilgrim target=seat0
[1898] activated_ability_resolved seat=1 source=Samite Pilgrim target=seat0
[1899] activate_ability seat=1 source=Samite Pilgrim target=seat0
[1900] stack_push seat=1 source=Samite Pilgrim target=seat0
[1901] priority_pass seat=2 source= target=seat0
[1902] priority_pass seat=3 source= target=seat0
[1903] priority_pass seat=0 source= target=seat0
[1904] stack_resolve seat=1 source=Samite Pilgrim target=seat0
[1905] prevent seat=0 source=Samite Pilgrim target=seat0
[1906] activated_ability_resolved seat=1 source=Samite Pilgrim target=seat0
[1907] activate_ability seat=1 source=Samite Pilgrim target=seat0
[1908] stack_push seat=1 source=Samite Pilgrim target=seat0
[1909] priority_pass seat=2 source= target=seat0
[1910] priority_pass seat=3 source= target=seat0
[1911] priority_pass seat=0 source= target=seat0
[1912] stack_resolve seat=1 source=Samite Pilgrim target=seat0
[1913] prevent seat=0 source=Samite Pilgrim target=seat0
[1914] activated_ability_resolved seat=1 source=Samite Pilgrim target=seat0
[1915] state seat=1 source= target=seat0
```

</details>

#### Violation 81

- **Game**: 358 (seed 3580043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 39, Phase=combat Step=end_of_combat
- **Commanders**: Six, Cosima, God of the Voyage // The Omenkeel, Altanak, the Thrice-Called, Dromoka, the Eternal
- **Message**: ExileLinkageIntegrity: card "Kozilek's Command" in seat 0 exile is linked to source timestamp 43 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 39, Phase=combat Step=end_of_combat Active=seat0
Stack: 0 items, EventLog: 1582 events
  Seat 0 [WON]: life=9 library=56 hand=2 graveyard=25 exile=5 battlefield=15 cmdzone=0 mana=9
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Rofellos, Llanowar Emissary (P/T 2/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Emrakul's Influence (P/T 0/0, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Rhox Pummeler (P/T 6/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Six (P/T 2/4, dmg=0) [T]
    - creature token pest Token (P/T 1/1, dmg=0) [T]
    - creature token pest Token (P/T 1/1, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 1 [LOST]: life=8 library=79 hand=1 graveyard=6 exile=4 battlefield=0 cmdzone=0 mana=0
  Seat 2 [LOST]: life=-3 library=80 hand=1 graveyard=8 exile=5 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-8 library=78 hand=0 graveyard=6 exile=6 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1562] damage seat=0 source=creature token pest Token amount=1 target=seat3
[1563] sba_704_5a seat=3 source= amount=-8
[1564] sba_cycle_complete seat=-1 source=
[1565] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1566] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1567] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1568] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1569] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[1570] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1571] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1572] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1573] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1574] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[1575] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1576] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1577] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1578] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[1579] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[1580] seat_eliminated seat=3 source= amount=12
[1581] game_end seat=0 source=
```

</details>

#### Violation 82

- **Game**: 390 (seed 3900043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 59, Phase=ending Step=cleanup
- **Commanders**: Altaïr Ibn-La'Ahad, Brimaz, Blight of Oreskos, Vega, the Watcher, Strago and Relm
- **Message**: ExileLinkageIntegrity: card "Torrent of Lava" in seat 0 exile is linked to source timestamp 58 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 59, Phase=ending Step=cleanup Active=seat2
Stack: 0 items, EventLog: 4035 events
  Seat 0 [alive]: life=24 library=73 hand=7 graveyard=8 exile=5 battlefield=7 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Planetarium of Wan Shi Tong (P/T 0/0, dmg=0) [T]
    - Unstable Frontier (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - A-Town (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=14 library=72 hand=6 graveyard=6 exile=4 battlefield=10 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Petrified Hamlet (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Scarecrow (P/T 4/4, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=1 library=73 hand=5 graveyard=10 exile=3 battlefield=7 cmdzone=1 mana=0
    - Apple of Eden, Isu Relic (P/T 0/0, dmg=0)
    - Mirrodin's Core (P/T 0/0, dmg=0) [T]
    - Hallowed Healer (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Champions of the Shoal (P/T 4/6, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Demolition Field (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=0 library=57 hand=6 graveyard=15 exile=3 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[4015] priority_pass seat=0 source= target=seat0
[4016] priority_pass seat=1 source= target=seat0
[4017] stack_resolve seat=2 source=Hallowed Healer target=seat0
[4018] prevent seat=0 source=Hallowed Healer target=seat0
[4019] activated_ability_resolved seat=2 source=Hallowed Healer target=seat0
[4020] activate_ability seat=2 source=Hallowed Healer target=seat0
[4021] stack_push seat=2 source=Hallowed Healer target=seat0
[4022] priority_pass seat=0 source= target=seat0
[4023] priority_pass seat=1 source= target=seat0
[4024] stack_resolve seat=2 source=Hallowed Healer target=seat0
[4025] prevent seat=0 source=Hallowed Healer target=seat0
[4026] activated_ability_resolved seat=2 source=Hallowed Healer target=seat0
[4027] activate_ability seat=2 source=Hallowed Healer target=seat0
[4028] stack_push seat=2 source=Hallowed Healer target=seat0
[4029] priority_pass seat=0 source= target=seat0
[4030] priority_pass seat=1 source= target=seat0
[4031] stack_resolve seat=2 source=Hallowed Healer target=seat0
[4032] prevent seat=0 source=Hallowed Healer target=seat0
[4033] activated_ability_resolved seat=2 source=Hallowed Healer target=seat0
[4034] state seat=2 source= target=seat0
```

</details>

#### Violation 83

- **Game**: 410 (seed 4100043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 27, Phase=main Step=precombat_main
- **Commanders**: Karlach, Tiefling Punisher, The Fifth Doctor, Braids, Conjurer Adept, Umaro, Raging Yeti
- **Message**: ExileLinkageIntegrity: card "Field of Ruin" in seat 0 exile is linked to source timestamp 24 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 27, Phase=main Step=precombat_main Active=seat2
Stack: 0 items, EventLog: 2420 events
  Seat 0 [LOST]: life=0 library=41 hand=1 graveyard=47 exile=3 battlefield=0 cmdzone=0 mana=0
  Seat 1 [LOST]: life=0 library=81 hand=0 graveyard=2 exile=6 battlefield=0 cmdzone=0 mana=0
  Seat 2 [WON]: life=23 library=81 hand=1 graveyard=4 exile=7 battlefield=11 cmdzone=0 mana=4
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Braids, Conjurer Adept (P/T 2/2, dmg=0)
    - Expedition Diviner (P/T 3/2, dmg=0)
    - Wolfhunter's Quiver (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Magus of the Future (P/T 2/3, dmg=0)
    - Tezzeret's Strider (P/T 3/1, dmg=0)
    - Helm of the Gods (P/T 0/0, dmg=0)
  Seat 3 [LOST]: life=0 library=82 hand=1 graveyard=8 exile=4 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2400] activated_ability_resolved seat=2 source=Wolfhunter's Quiver target=seat0
[2401] sba_704_5a seat=3 source=
[2402] sba_cycle_complete seat=-1 source=
[2403] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2404] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2405] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2406] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2407] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2408] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2409] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2410] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2411] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2412] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2413] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2414] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2415] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2416] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2417] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2418] seat_eliminated seat=3 source= amount=4
[2419] game_end seat=2 source=
```

</details>

#### Violation 84

- **Game**: 436 (seed 4360043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 60, Phase=ending Step=cleanup
- **Commanders**: Torbran, Thane of Red Fell, Junji, the Midnight Sky, Arvad of the Weatherlight, Ruxa, Patient Professor
- **Message**: ExileLinkageIntegrity: card "Mountain" in seat 0 exile is linked to source timestamp 34 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 60, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 2629 events
  Seat 0 [LOST]: life=-7 library=74 hand=4 graveyard=8 exile=9 battlefield=0 cmdzone=1 mana=0
  Seat 1 [alive]: life=12 library=72 hand=6 graveyard=6 exile=7 battlefield=11 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Promising Vein (P/T 0/0, dmg=0) [T]
    - Dauthi Embrace (P/T 0/0, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Consulate Skygate (P/T 0/4, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Pelakka Predation // Pelakka Caverns (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Crypt Ripper (P/T 2/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=9 library=71 hand=4 graveyard=11 exile=9 battlefield=10 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Fountainport (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Arvad of the Weatherlight (P/T 3/4, dmg=0) [T]
  Seat 3 [LOST]: life=-1 library=73 hand=0 graveyard=8 exile=9 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2609] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2610] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2611] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2612] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2613] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2614] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2615] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2616] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2617] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2618] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2619] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2620] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2621] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2622] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2623] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2624] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2625] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2626] seat_eliminated seat=3 source= amount=15
[2627] phase_step seat=1 source= target=seat0
[2628] state seat=1 source= target=seat0
```

</details>

#### Violation 85

- **Game**: 452 (seed 4520043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 19, Phase=ending Step=cleanup
- **Commanders**: Jenson Carthalion, Druid Exile, Missy, Junji, the Midnight Sky, Kira, Great Glass-Spinner
- **Message**: ExileLinkageIntegrity: card "Ravenous Brute Head" in seat 2 exile is linked to source timestamp 28 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 19, Phase=ending Step=cleanup Active=seat1
Stack: 0 items, EventLog: 729 events
  Seat 0 [alive]: life=38 library=85 hand=4 graveyard=3 exile=0 battlefield=7 cmdzone=0 mana=0
    - Yavimaya Hollow (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Ondu Inversion // Ondu Skyruins (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Unstable Amulet (P/T 0/0, dmg=0) [T]
    - Perilous Landscape (P/T 0/0, dmg=0) [T]
    - Jenson Carthalion, Druid Exile (P/T 2/2, dmg=0)
  Seat 1 [alive]: life=30 library=82 hand=7 graveyard=3 exile=1 battlefield=7 cmdzone=1 mana=0
    - Mountain (P/T 0/0, dmg=0) [T]
    - creature token white human Token (P/T 1/1, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Eye of Vecna (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Lazotep Quarry (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=40 library=87 hand=2 graveyard=3 exile=1 battlefield=5 cmdzone=1 mana=0
    - Grasping Shadows // Shadows' Lair (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Bucolic Ranch (P/T 0/0, dmg=0) [T]
    - Diamond City (P/T 0/0, dmg=0) [T]
    - Belbe's Armor (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=38 library=85 hand=1 graveyard=3 exile=0 battlefield=8 cmdzone=0 mana=0
    - Conduit Pylons (P/T 0/0, dmg=0) [T]
    - Potatoes (P/T 0/0, dmg=0)
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Kira, Great Glass-Spinner (P/T 2/2, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Illusionary Wall (P/T 7/4, dmg=0)

```

</details>

<details>
<summary>Recent Events</summary>

```
[709] draw seat=1 source=Slagwoods Bridge // Slagwoods Bridge amount=1 target=seat0
[710] play_land seat=1 source=Lazotep Quarry target=seat0
[711] add_mana seat=1 source=Lazotep Quarry amount=1 target=seat0
[712] phase_step seat=1 source= target=seat0
[713] declare_attackers seat=1 source= target=seat0
[714] blockers seat=3 source= target=seat0
[715] damage seat=3 source=Illusionary Wall amount=7 target=seat1
[716] destroy seat=1 source=Hostage Taker
[717] sba_704_5g seat=1 source=Hostage Taker
[718] zone_change seat=1 source=Hostage Taker
[719] zone_cast_grant_expired seat=1 source=Hostage Taker target=seat0
[720] sba_cycle_complete seat=-1 source=
[721] damage seat=1 source=creature token white human Token amount=1 target=seat3
[722] speed_advance seat=1 source= amount=3 target=seat0
[723] phase_step seat=1 source= target=seat0
[724] pool_drain seat=1 source= amount=5 target=seat0
[725] zone_change seat=1 source=Archival Whorl
[726] discard seat=1 source=Archival Whorl target=seat0
[727] cleanup_loop seat=1 source= target=seat0
[728] state seat=1 source= target=seat0
```

</details>

#### Violation 86

- **Game**: 469 (seed 4690043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 50, Phase=combat Step=end_of_combat
- **Commanders**: Brinelin, the Moon Kraken, Wolverine, Best There Is, Jace, Vryn's Prodigy // Jace, Telepath Unbound, Shisato, Whispering Hunter
- **Message**: ExileLinkageIntegrity: card "Island" in seat 0 exile is linked to source timestamp 54 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 50, Phase=combat Step=end_of_combat Active=seat1
Stack: 0 items, EventLog: 4013 events
  Seat 0 [LOST]: life=0 library=77 hand=2 graveyard=8 exile=6 battlefield=0 cmdzone=1 mana=0
  Seat 1 [WON]: life=36 library=73 hand=4 graveyard=1 exile=4 battlefield=17 cmdzone=0 mana=10
    - Mountain (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Wolverine, Best There Is (P/T 2/2, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Gruul Guildgate (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ryusei, the Falling Star (P/T 5/5, dmg=0) [T]
    - Safe Haven (P/T 0/0, dmg=0) [T]
    - Ironroot Treefolk (P/T 3/5, dmg=0) [T]
    - Fire Urchin (P/T 1/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Ulamog's Crusher (P/T 8/8, dmg=2) [T]
    - Vorapede (P/T 5/4, dmg=0)
    - Forest (P/T 0/0, dmg=0) [T]
    - Borderland Minotaur (P/T 4/3, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
  Seat 2 [LOST]: life=-2 library=56 hand=6 graveyard=22 exile=9 battlefield=0 cmdzone=0 mana=0
  Seat 3 [LOST]: life=-17 library=68 hand=1 graveyard=18 exile=8 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[3993] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3994] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3995] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3996] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[3997] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[3998] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[3999] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4000] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4001] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[4002] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4003] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4004] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4005] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4006] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[4007] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[4008] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[4009] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[4010] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[4011] seat_eliminated seat=3 source= amount=7
[4012] game_end seat=1 source=
```

</details>

#### Violation 87

- **Game**: 499 (seed 4990043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 59, Phase=beginning Step=upkeep
- **Commanders**: Ultima, Origin of Oblivion, The Unknown Wizard, Cherubael, The Scarab God
- **Message**: ExileLinkageIntegrity: card "Torpor Orb" in seat 0 exile is linked to source timestamp 51 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 59, Phase=beginning Step=upkeep Active=seat0
Stack: 0 items, EventLog: 2346 events
  Seat 0 [alive]: life=11 library=63 hand=7 graveyard=12 exile=8 battlefield=13 cmdzone=1 mana=0
    - Mirrex (P/T 0/0, dmg=0) [T]
    - Northampton Farm (P/T 0/0, dmg=0) [T]
    - Golem's Heart (P/T 0/0, dmg=0)
    - Tarnation Vista (P/T 0/0, dmg=0) [T]
    - Echoing Cavern (P/T 0/0, dmg=0) [T]
    - Monument to Perfection (P/T 0/0, dmg=0) [T]
    - Sea Gate Wreckage (P/T 0/0, dmg=0) [T]
    - Spawning Bed (P/T 0/0, dmg=0) [T]
    - Grixis Panorama (P/T 0/0, dmg=0) [T]
    - Command Tower (P/T 0/0, dmg=0) [T]
    - Myriad Landscape (P/T 0/0, dmg=0) [T]
    - Haunted Fengraf (P/T 0/0, dmg=0) [T]
    - Glimmerpost (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=12 library=74 hand=6 graveyard=6 exile=9 battlefield=10 cmdzone=1 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Ebon Stronghold (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Flowstone Armor (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Turntimber Grove (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - A-Lapis Orb of Dragonkind (P/T 0/0, dmg=0)
    - Chalice of the Void (P/T 0/0, dmg=0)
    - Skyhunter Skirmisher (P/T 1/1, dmg=0)
  Seat 2 [alive]: life=40 library=73 hand=0 graveyard=11 exile=4 battlefield=12 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Cherubael (P/T 4/4, dmg=0) [T]
    - Forge of Heroes (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Bloodline Pretender (P/T 3/3, dmg=0) [T]
    - Wall of Vipers (P/T 2/4, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Undertaker (P/T 1/1, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Sky Skiff (P/T 2/3, dmg=0)
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-1 library=73 hand=3 graveyard=3 exile=4 battlefield=0 cmdzone=1 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2326] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2327] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2328] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2329] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2330] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2331] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2332] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2333] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2334] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2335] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2336] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2337] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2338] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2339] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2340] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2341] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2342] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2343] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2344] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[2345] seat_eliminated seat=3 source= amount=15
```

</details>

#### Violation 88

- **Game**: 526 (seed 5260043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 49, Phase=beginning Step=upkeep
- **Commanders**: Pashalik Mons, Stangg, Major Teroh, Nevinyrral, Urborg Tyrant
- **Message**: ExileLinkageIntegrity: card "Expand the Sphere // Expand the Sphere" in seat 0 exile is linked to source timestamp 46 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 49, Phase=beginning Step=upkeep Active=seat2
Stack: 0 items, EventLog: 1801 events
  Seat 0 [LOST]: life=-6 library=70 hand=3 graveyard=8 exile=10 battlefield=0 cmdzone=1 mana=0
  Seat 1 [LOST]: life=-9 library=76 hand=3 graveyard=6 exile=7 battlefield=0 cmdzone=1 mana=0
  Seat 2 [WON]: life=10 library=76 hand=0 graveyard=12 exile=8 battlefield=4 cmdzone=1 mana=4
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Bolas's Citadel (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=-9 library=77 hand=5 graveyard=6 exile=4 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[1781] sba_cycle_complete seat=-1 source=
[1782] seat_eliminated seat=0 source= amount=6
[1783] seat_eliminated seat=1 source= amount=9
[1784] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1785] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1786] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[1787] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[1788] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[1789] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1790] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1791] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1792] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[1793] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1794] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1795] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1796] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[1797] zone_cast_grant_expired seat=1 source=Knowledge Pool target=seat0
[1798] seat_eliminated seat=3 source= amount=9
[1799] zone_cast_grant_expired seat=2 source=Bolas's Citadel target=seat0
[1800] game_end seat=2 source=
```

</details>

#### Violation 89

- **Game**: 531 (seed 5310043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 32, Phase=ending Step=cleanup
- **Commanders**: The Keeper of Kaldra, Fain, the Broker, Mona Lisa, Science Geek, The Rebellious Intelligence
- **Message**: ExileLinkageIntegrity: card "Primal Storm" in seat 0 exile is linked to source timestamp 38 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 32, Phase=ending Step=cleanup Active=seat0
Stack: 0 items, EventLog: 992 events
  Seat 0 [alive]: life=25 library=80 hand=7 graveyard=1 exile=4 battlefield=6 cmdzone=1 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Akoum Warrior // Akoum Teeth (P/T 4/5, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Gorilla Shaman (P/T 1/1, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 1 [alive]: life=36 library=80 hand=6 graveyard=6 exile=3 battlefield=4 cmdzone=1 mana=0
    - Evolving Wilds (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Silver Deputy (P/T 1/2, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=33 library=81 hand=3 graveyard=3 exile=3 battlefield=9 cmdzone=0 mana=0
    - Forest (P/T 0/0, dmg=0) [T]
    - Jund Panorama (P/T 0/0, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Endless Sands (P/T 0/0, dmg=0) [T]
    - Traveling Botanist (P/T 2/3, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
    - Mona Lisa, Science Geek (P/T 1/3, dmg=0) [T]
    - Genesis (P/T 4/4, dmg=0) [T]
    - Forest (P/T 0/0, dmg=0) [T]
  Seat 3 [alive]: life=39 library=81 hand=5 graveyard=4 exile=3 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Island (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Noxious Bayou (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]

```

</details>

<details>
<summary>Recent Events</summary>

```
[972] priority_pass seat=2 source= target=seat0
[973] priority_pass seat=3 source= target=seat0
[974] stack_resolve seat=0 source=Gorilla Shaman target=seat0
[975] activated_ability_resolved seat=0 source=Gorilla Shaman target=seat0
[976] play_land seat=0 source=Plains target=seat0
[977] add_mana seat=0 source=Plains amount=1 target=seat0
[978] pay_mana seat=0 source=Gorilla Shaman amount=1 target=seat0
[979] activate_ability seat=0 source=Gorilla Shaman target=seat0
[980] stack_push seat=0 source=Gorilla Shaman target=seat0
[981] priority_pass seat=1 source= target=seat0
[982] priority_pass seat=2 source= target=seat0
[983] priority_pass seat=3 source= target=seat0
[984] stack_resolve seat=0 source=Gorilla Shaman target=seat0
[985] activated_ability_resolved seat=0 source=Gorilla Shaman target=seat0
[986] phase_step seat=0 source= target=seat0
[987] declare_attackers seat=0 source= target=seat0
[988] blockers seat=2 source= target=seat0
[989] damage seat=0 source=Gorilla Shaman amount=1 target=seat2
[990] phase_step seat=0 source= target=seat0
[991] state seat=0 source= target=seat0
```

</details>

#### Violation 90

- **Game**: 541 (seed 5410043, perm 0)
- **Invariant**: ExileLinkageIntegrity
- **Turn**: 54, Phase=beginning Step=draw
- **Commanders**: Massacre Girl, The Multifaceted Phyrexian, Piru, the Volatile, Marath, Will of the Wild
- **Message**: ExileLinkageIntegrity: card "Myrkul's Edict" in seat 0 exile is linked to source timestamp 48 which is no longer on any battlefield — LTB return missed (orphaned linked exile)

<details>
<summary>Game State</summary>

```
Turn 54, Phase=beginning Step=draw Active=seat0
Stack: 0 items, EventLog: 2565 events
  Seat 0 [alive]: life=2 library=75 hand=4 graveyard=11 exile=4 battlefield=6 cmdzone=0 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Massacre Girl (P/T 4/4, dmg=0)
  Seat 1 [alive]: life=8 library=75 hand=7 graveyard=9 exile=3 battlefield=4 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
  Seat 2 [alive]: life=8 library=73 hand=0 graveyard=14 exile=5 battlefield=9 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Trepanation Blade (P/T 0/0, dmg=0)
    - Catapult Fodder // Catapult Captain (P/T 1/5, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Daretti, Scrap Savant (P/T 0/3, dmg=0)
    - Plains (P/T 0/0, dmg=0) [T]
  Seat 3 [LOST]: life=7 library=0 hand=11 graveyard=73 exile=8 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[2545] activated_ability_resolved seat=3 source=Marath, Will of the Wild target=seat0
[2546] sba_704_5b seat=3 source=
[2547] sba_cycle_complete seat=-1 source=
[2548] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2549] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2550] zone_cast_grant_expired seat=0 source=Knowledge Pool target=seat0
[2551] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2552] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2553] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2554] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2555] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2556] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2557] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2558] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2559] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2560] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2561] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2562] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[2563] zone_cast_grant_expired seat=3 source=Knowledge Pool target=seat0
[2564] seat_eliminated seat=3 source= amount=12
```

</details>

*... and 109998 more violations not shown.*

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Reputable Merchant | 4 | 0 | 1.00 |
| 2 | Bant Battlemage | 3 | 0 | 1.00 |
| 3 | Golden Ratio | 3 | 0 | 1.00 |
| 4 | Brokers Initiate | 3 | 1 | 0.75 |
| 5 | Trace of Abundance | 3 | 1 | 0.75 |
| 6 | Guided Passage | 3 | 1 | 0.75 |
| 7 | Raging Kavu | 3 | 1 | 0.75 |
| 8 | Aminatou, the Fateshifter | 3 | 1 | 0.75 |
| 9 | Profit // Loss | 5 | 2 | 0.71 |
| 10 | Tower Gargoyle | 5 | 2 | 0.71 |

## Verdict: ISSUES FOUND

**110088 total issues** across 10000 chaos games and 0 nightmare boards.
- 0 crashes in chaos games
- 110088 invariant violations in chaos games
- 0 crashes in nightmare boards
- 0 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
