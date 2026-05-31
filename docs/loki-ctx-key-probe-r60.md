# Chaos Gauntlet Report

Generated: 2026-05-30T21:39:15-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 35875 cards |
| Legendary Creatures | 3403 |
| Total Games | 500 |
| Seed | 1234 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 0 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 20.778s |
| Throughput | 24 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 2 (in 1 games) |
| Clean Games | 499 |

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
| ZoneConservation | 2 |

### Violation Details (up to 30 per invariant kind, 1 shown)

#### Violation 1

- **Game**: 241 (seed 2411235, perm 0)
- **Invariant**: ZoneConservation
- **Turn**: 51, Phase=main Step=precombat_main
- **Commanders**: Vazi, Keen Negotiator, Losheel, Clockwork Scholar, The Swarmweaver, Lyra Dawnbringer
- **Message**: ZoneConservation: InstanceID "h2TKVC000145" (Insect Token) is minted and not ceased but is absent from every zone — card disappeared

<details>
<summary>Game State</summary>

```
Turn 51, Phase=main Step=precombat_main Active=seat0
Stack: 0 items, EventLog: 4336 events
  Seat 0 [alive]: life=22 library=79 hand=3 graveyard=11 exile=0 battlefield=6 cmdzone=1 mana=0
    - Swamp (P/T 0/0, dmg=0) [T]
    - Swamp (P/T 0/0, dmg=0) [T]
    - Mountain (P/T 0/0, dmg=0) [T]
    - Zinnia, Valley's Voice (P/T 1/3, dmg=0) [T]
    - Bloodchief Ascension (P/T 0/0, dmg=0)
    - Riot Gear (P/T 0/0, dmg=0)
  Seat 1 [alive]: life=28 library=79 hand=3 graveyard=9 exile=0 battlefield=11 cmdzone=0 mana=0
    - Plains (P/T 0/0, dmg=0) [T]
    - Plains (P/T 0/0, dmg=0) [T]
    - Flumph (P/T 0/4, dmg=0)
    - Skyblade of the Legion (P/T 1/3, dmg=0) [T]
    - Sheltering Prayers (P/T 0/0, dmg=0)
    - Multiversal Passage (P/T 0/0, dmg=0) [T]
    - Losheel, Clockwork Scholar (P/T 2/4, dmg=0) [T]
    - Myr Token (P/T 1/1, dmg=0) [T]
    - Dread Statuary (P/T 0/0, dmg=0) [T]
    - Acclaimed Contender (P/T 3/3, dmg=0)
    - Myr Token (P/T 1/1, dmg=0)
  Seat 2 [LOST]: life=0 library=78 hand=5 graveyard=2 exile=0 battlefield=2 cmdzone=0 mana=0
    - Insect Token (P/T 1/1, dmg=0)
    - Insect Token (P/T 1/1, dmg=0)
  Seat 3 [LOST]: life=-3 library=80 hand=3 graveyard=4 exile=0 battlefield=0 cmdzone=0 mana=0

```

</details>

<details>
<summary>Recent Events</summary>

```
[4316] priority_pass seat=1 source= target=seat0
[4317] stack_resolve seat=0 source=Zinnia, Valley's Voice target=seat0
[4318] trigger_evaluated seat=-1 source=token_created
[4319] create_token seat=2 source=Insect Token target=seat0
[4320] trigger_evaluated seat=-1 source=nonland_permanent_etb
[4321] trigger_evaluated seat=0 source=Zinnia, Valley's Voice
[4322] stack_push seat=0 source=Zinnia, Valley's Voice target=seat0
[4323] triggered_ability seat=0 source=Zinnia, Valley's Voice target=seat0
[4324] priority_pass seat=1 source= target=seat0
[4325] stack_resolve seat=0 source=Zinnia, Valley's Voice target=seat0
[4326] trigger_evaluated seat=-1 source=token_created
[4327] create_token seat=2 source=Insect Token target=seat0
[4328] per_card_handler seat=0 source=The Swarmweaver target=seat0
[4329] per_card_handler seat=0 source=The Swarmweaver target=seat0
[4330] trigger_evaluated seat=-1 source=nonland_permanent_etb
[4331] trigger_evaluated seat=0 source=Zinnia, Valley's Voice
[4332] stack_push seat=0 source=Zinnia, Valley's Voice target=seat0
[4333] triggered_ability seat=0 source=Zinnia, Valley's Voice target=seat0
[4334] priority_pass seat=1 source= target=seat0
[4335] stack_resolve seat=0 source=Zinnia, Valley's Voice target=seat0
```

</details>

*... and 1 more violations not shown.*

## Top Cards Correlated with Violations

Cards that appeared disproportionately in violation games vs clean games.
Only cards appearing in 3+ total games are shown.

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|-----------------|-------------|-------------|
| 1 | Bound by Moonsilver | 1 | 2 | 0.33 |
| 2 | Etched Familiar | 1 | 2 | 0.33 |
| 3 | Spore Burst | 1 | 2 | 0.33 |
| 4 | Toby, Beastie Befriender | 1 | 2 | 0.33 |
| 5 | Eidolon of the Great Revel | 1 | 2 | 0.33 |
| 6 | Lavaclaw Reaches | 1 | 2 | 0.33 |
| 7 | Teroh's Faithful | 1 | 2 | 0.33 |
| 8 | Prismatic Strands | 1 | 2 | 0.33 |
| 9 | Reverse the Sands | 1 | 2 | 0.33 |
| 10 | Bloodchief Ascension | 1 | 2 | 0.33 |

## Verdict: ISSUES FOUND

**2 total issues** across 500 chaos games and 0 nightmare boards.
- 0 crashes in chaos games
- 2 invariant violations in chaos games
- 0 crashes in nightmare boards
- 0 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
