# Playloop Report — 3 Matchups with Combat

_Self-play simulation, **5 games per matchup**, seed=42, max-turns=40._

## What this is

A Python-only, structural-AST self-play loop. Decks are drafted from the oracle dump, the parser emits typed AST, and the playloop dispatches on Effect node kinds. The turn structure is: untap → upkeep (triggers) → draw → main1 → **combat** → main2 → end.

**Combat** (new): declare-attackers → declare-blockers → first-strike damage → regular damage, with state-based actions between damage steps. Heuristic policies: attack-with-everything, block smallest-survivor / chump-only-if-lethal. Menace requires 2 blockers; trample spillovers to player; lifelink heals controller; deathtouch marks lethal.

## Matchup Results

| Matchup | Deck A | Wins A | Deck B | Wins B | Draws | Avg turns |
|---|---|---:|---|---:|---:|---:|
| **Burn vs Control** | Burn | 3 (60%) | Control | 2 (40%) | 0 (0%) | 15.80 |
| **Burn vs Creatures** | Burn | 0 (0%) | Creatures | 5 (100%) | 0 (0%) | 5.00 |
| **Burn vs Ramp** | Burn | 1 (20%) | Ramp | 4 (80%) | 0 (0%) | 6.80 |
| **Control vs Creatures** | Control | 0 (0%) | Creatures | 5 (100%) | 0 (0%) | 6.00 |
| **Control vs Ramp** | Control | 2 (40%) | Ramp | 3 (60%) | 0 (0%) | 14.80 |
| **Creatures vs Ramp** | Creatures | 5 (100%) | Ramp | 0 (0%) | 0 (0%) | 5.20 |

### Burn vs Control

- **Burn** wins: **3** (60.0%)
- **Control** wins: **2** (40.0%)
- Draws: 0 (0.0%)
- Turn stats — avg **15.80**, median 18, min/max 9/22


**Turn distribution:**

| Turns | Games | |
|---:|---:|:---|
| 9-10 | 1 | ████████████████████ |
| 11-12 | 1 | ████████████████████ |
| 17-18 | 1 | ████████████████████ |
| 19-20 | 1 | ████████████████████ |
| 21-22 | 1 | ████████████████████ |

**End reasons:**

| Count | Reason |
|---:|---|
| 3 | seat 0 is the last one standing |
| 2 | seat 1 is the last one standing |

### Burn vs Creatures

- **Burn** wins: **0** (0.0%)
- **Creatures** wins: **5** (100.0%)
- Draws: 0 (0.0%)
- Turn stats — avg **5.00**, median 5, min/max 5/5


**Turn distribution:**

| Turns | Games | |
|---:|---:|:---|
| 5-6 | 5 | ████████████████████ |

**End reasons:**

| Count | Reason |
|---:|---|
| 5 | seat 1 is the last one standing |

### Burn vs Ramp

- **Burn** wins: **1** (20.0%)
- **Ramp** wins: **4** (80.0%)
- Draws: 0 (0.0%)
- Turn stats — avg **6.80**, median 7, min/max 5/8


**Turn distribution:**

| Turns | Games | |
|---:|---:|:---|
| 5-6 | 1 | █████ |
| 7-8 | 4 | ████████████████████ |

**End reasons:**

| Count | Reason |
|---:|---|
| 4 | seat 1 is the last one standing |
| 1 | seat 0 is the last one standing |

### Control vs Creatures

- **Control** wins: **0** (0.0%)
- **Creatures** wins: **5** (100.0%)
- Draws: 0 (0.0%)
- Turn stats — avg **6.00**, median 6, min/max 5/7


**Turn distribution:**

| Turns | Games | |
|---:|---:|:---|
| 5-6 | 4 | ████████████████████ |
| 7-8 | 1 | █████ |

**End reasons:**

| Count | Reason |
|---:|---|
| 5 | seat 1 is the last one standing |

### Control vs Ramp

- **Control** wins: **2** (40.0%)
- **Ramp** wins: **3** (60.0%)
- Draws: 0 (0.0%)
- Turn stats — avg **14.80**, median 15, min/max 13/18


**Turn distribution:**

| Turns | Games | |
|---:|---:|:---|
| 13-14 | 2 | ████████████████████ |
| 15-16 | 2 | ████████████████████ |
| 17-18 | 1 | ██████████ |

**End reasons:**

| Count | Reason |
|---:|---|
| 3 | seat 1 is the last one standing |
| 2 | seat 0 is the last one standing |

### Creatures vs Ramp

- **Creatures** wins: **5** (100.0%)
- **Ramp** wins: **0** (0.0%)
- Draws: 0 (0.0%)
- Turn stats — avg **5.20**, median 5, min/max 4/6


**Turn distribution:**

| Turns | Games | |
|---:|---:|:---|
| 3-4 | 1 | █████ |
| 5-6 | 4 | ████████████████████ |

**End reasons:**

| Count | Reason |
|---:|---|
| 5 | seat 0 is the last one standing |

## Deck compositions

**Burn** — 60 cards, 10 unique:

- Chain Lightning
- Goblin Guide
- Grizzly Bears
- Lava Spike
- Lightning Bolt
- Lightning Strike
- Mountain
- Rift Bolt
- Searing Blaze
- Shock

**Control** — 60 cards, 14 unique:

- Cancel
- Counterspell
- Dark Ritual
- Dissolve
- Divination
- Doom Blade
- Grizzly Bears
- Island
- Mana Leak
- Negate
- Opt
- Ponder
- Preordain
- Swamp

**Creatures** — 60 cards, 14 unique:

- Boros Swiftblade
- Fencing Ace
- Goblin Cohort
- Goblin Guide
- Goblin Piker
- Goblin Raider
- Grizzly Bears
- Hell's Thunder
- Jackal Pup
- Keldon Raider
- Mogg Flunkies
- Mountain
- Raging Goblin
- Viashino Pyromancer

**Ramp** — 60 cards, 13 unique:

- Avenger of Zendikar
- Craterhoof Behemoth
- Cultivate
- Elvish Mystic
- Farseek
- Forest
- Kodama's Reach
- Llanowar Elves
- Primeval Titan
- Rampant Growth
- Sakura-Tribe Elder
- Solemn Simulacrum
- Wood Elves

## Unknown / unhandled AST nodes (combined)

| Count | Node kind |
|---:|---|
| 71 | `attack_trigger_unknown` |
| 20 | `parsed_effect_residual` |
| 12 | `UnknownEffect` |

## Combat keywords supported

- **flying** — only blockable by flying/reach
- **reach** — can block flyers
- **first strike** — deals damage in the first-strike step
- **double strike** — deals damage in BOTH steps
- **trample** — excess damage after lethal-assigned goes to player
- **deathtouch** — any damage marks lethal on the blocker
- **lifelink** — damage dealt heals controller
- **vigilance** — attacker does not tap
- **menace** — must be blocked by 2+ creatures
- **defender** — can block but never attacks
- **haste** — ignores summoning sickness (pre-existing)

## What works (beyond combat)

- Full turn structure: untap → upkeep → draw → main1 → combat → main2 → end.
- State-based actions between damage steps (first strike removes dead blockers before regular damage).
- Effects executing: Damage, Draw, Discard, Destroy, Buff, CreateToken, AddMana, GainLife, LoseLife, CounterMod, Tap/Untap, Sequence/Choice/Optional/Conditional control flow, ETB triggers, upkeep triggers, Tutor (basic-land), Reanimate.

## What's stubbed

- **Stack**: counterspells resolve as no-ops; spells resolve at cast time.
- **Colored mana**: single generic pool; CMC-only cost check.
- **Combat tricks**: no instants during combat (no stack in MVP).
- **Planeswalkers**: no attacking planeswalkers; attacks always target player.
- **Protection / indestructible / shroud / hexproof**: not implemented.
- **Damage-prevention / replacement effects**: not implemented.
- **Scry / Surveil / LookAt / Reveal**: no-ops (library not reordered).
- **Choice**: always picks option 1.
- **Conditional**: always executes body.
- **ExtraTurn**: ignored.
- **GainControl**: no-op.

## Files

- Simulator: `scripts/playloop.py`
- Sample game logs (first game of each matchup): `data/rules/playloop_sample_log.txt`
- This report: `data/rules/playloop_report.md`

## Next steps

1. Port combat resolver to Go so `internal/game/` can share it.
2. Colored-mana constraints (ManaSymbol.color on pool entries).
3. Instants during opponent's turn (requires a stack).
4. Planeswalker attackers / redirected damage.
5. MCTS / heuristic policy for Choice + Conditional branches.
