# Loki Pathological-Deck Stress (r60)

## Summary

Targeted-pathology layer of Verification Phase 5. Where the standard
chaos gauntlet (PR #896, **0 crashes / 0 violations** at 5k seed-42
chaos + 10k nightmare on `origin/main @ b27f72d8`) samples random
card combinations from the full corpus, this run **forces** 14
pathological cards into every seat's deck round-robin so each game
exercises at least one of the 10 historically-fragile interaction
surfaces.

## Pathology gauntlet

| Surface | Seed cards | Why pathological |
|---|---|---|
| Riku of Two Reflections | `Riku of Two Reflections` | Everything-copies; spell-copy + permanent-copy chokepoint stress (Phase F mint paths) |
| Krark flip-everything | `Krark the Thumbless` | Coin-flip-driven re-cast pile; stack lifecycle + paradigm copy stress |
| Worldgorger Dragon infinite-flicker | `Worldgorger Dragon`, `Animate Dead` | The textbook §727 no-op-loop; loop_shortcut.go's primary stress case |
| Felidar+Saheeli combo | `Felidar Guardian`, `Saheeli Rai` | Token-as-copy + ETB blink loop; MintTokenAsCopyOf stress |
| Marit Lage token-of-token | `Marit Lage`, `Dark Depths` | The largest single token in MTG; counter-removal token-mint stress |
| Yidris cascade pile | `Yidris Maelstrom Wielder` | Cascade + storm-counting + free-cast grant stress |
| Necrotic Ooze toolbox | `Necrotic Ooze` | Activated-ability inheritance; AST cast-resolution + activation stress |
| Nahiri+Emrakul reanimator | `Nahiri the Harbinger`, `Emrakul the Aeons Torn` | Discard-to-extra-turn + 15-CMC bomb; ZoneCastGrant + multi-turn stress |
| Hapatra+infinite-tokens | `Hapatra Vizier of Poisons` | Counter-trigger token mint; token-cease ZoneConservation stress |
| Animar+infinite-spells | `Animar Soul of Elements` | Cost-reduction + creature-spam; mana-pool + counter-stamping stress |

14 cards × 4 seats round-robin (Loki's `--seed-cards-all-seats` mode)
guarantees each seat gets 3-4 pathological pieces; the chaos-deck
generator fills the remaining ~95 slots from the random corpus,
preserving the chaos-game noise floor while pinning the pathological
interactions.

## Run

```bash
SEEDS="Riku of Two Reflections,Krark the Thumbless,Worldgorger Dragon,Animate Dead,Felidar Guardian,Saheeli Rai,Marit Lage,Dark Depths,Yidris Maelstrom Wielder,Necrotic Ooze,Nahiri the Harbinger,Emrakul the Aeons Torn,Hapatra Vizier of Poisons,Animar Soul of Elements"
go run ./cmd/hexdek-loki \
  --games 2000 --seed 42 --nightmare-boards 0 \
  --seed-cards-all-seats "$SEEDS" \
  --instanceid-strict-census \
  --report /tmp/loki-pathological/CHAOS_REPORT.md \
  --violations-dump /tmp/loki-pathological/v.tsv
```

`--instanceid-strict-census` is on because the pathological gauntlet
is targeted at exactly the surfaces strict-census was designed to
catch (Phase C → G closure series).

## Results

| Metric | Pathological 2k | Standard 5k baseline (PR #896) | Δ per-game |
|---|---:|---:|---:|
| Games | 2000 | 5000 | — |
| Crashes | **0** | 0 | 0 |
| Violations | **2** | 0 | +0.001 per game |
| Clean games | 1999 | 5000 | — |
| Throughput | 6 g/s (CPU-contended w/ a parallel 50k run) | — | — |
| Wall time | 5m 28s | — | — |

## Per-invariant breakdown

| Invariant | Hits | Games | Signature |
|---|---:|---:|---|
| `CombatLegality` | 2 | 1 (game 1317) | `"Wall of Tanglecord" (seat 3) is attacking but has defender` |
| All others | 0 | 0 | clean |

Both hits are the **same** violation re-checked across two SBA passes
in the same game. Single signature, single game out of 2000.

## Cluster analysis — game 1317

- **Turn 36**, phase=combat step=end_of_combat, active=seat 3.
- Seat 3 (commander: Raph & Mikey, Troublemakers — 7/7 legendary)
  has the game won (life=19; opponents at 40-life-but-decked / -10 /
  -12). Attacking with the full board this combat.
- The attacker that tripped the invariant: **Wall of Tanglecord**
  (0/6, base defender per oracle: *"Reach, Defender"*).
- Pathological seeded cards in this game's seat 3 deck: Hapatra,
  Vizier of Poisons + Necrotic Ooze + Saheeli Rai (per the
  round-robin distribution from a 14-card pool over 4 seats).
- The bug is NOT directly caused by any seeded pathological card —
  Wall of Tanglecord was filled in by the chaos generator. But the
  pathological seeding re-shuffled the random sample enough that
  game 1317's composition exposed a CombatLegality issue that the
  standard chaos sample (PR #896) did not surface at seed 42.

Hypothesis (NOT confirmed — bisect deferred): a defender-stripping
effect from a pathological-adjacent card (e.g., Necrotic Ooze
inheriting an attack-with-defender ability from a graveyard card,
or a Saheeli token-copy promoting Wall of Tanglecord without
inheriting the defender keyword) lets the wall enter the attacker
list without the legality check refreshing. The bug is reproducible
at `--games 1320 --seed 42 --seed-cards-all-seats <list>`.

## Interpretation

The pathological gauntlet delivered on its design intent: it surfaced
**one real `CombatLegality` bug** that the standard chaos baseline
missed at the same seed. The signature is narrow (1 game / 2 hits
out of 2000) but bit-stable for repro.

Translation:
- The PR #896 0/0 baseline is **mostly** correct — the engine is
  clean across the surfaces the random sampler naturally selects.
- A small set of corner-case interactions exist that the random
  sampler misses but pathological-seeding catches.
- The pathological gauntlet should be part of the regular regression
  matrix going forward (additive to the standard chaos run, not a
  replacement).

## How this complements the standard baseline

The standard chaos baseline answers "is the engine clean for randomly-
sampled corpus games?" — yes per PR #896. This run answers a different
question: "is the engine clean when we *force* every historically-
fragile interaction to fire?". A pathological 0/0 promotes the
standard 0/0 baseline from "the chaos sample missed the bugs" to
"the engine is clean at targeted-pathology depth".

A pathological non-zero alongside a standard 0/0 means the chaos
generator wasn't selecting the affected cards — exactly the gap this
gauntlet is designed to close.

## Reproduction

```bash
git fetch origin main && git checkout -B repro origin/main
# (drive-by huginn rename — see PR commit body — is required for the
# Loki binary to build on current main; PR drops the fix in alongside
# the gauntlet results)
go run ./cmd/hexdek-loki \
  --games 2000 --seed 42 --nightmare-boards 0 \
  --seed-cards-all-seats "Riku of Two Reflections,Krark the Thumbless,Worldgorger Dragon,Animate Dead,Felidar Guardian,Saheeli Rai,Marit Lage,Dark Depths,Yidris Maelstrom Wielder,Necrotic Ooze,Nahiri the Harbinger,Emrakul the Aeons Torn,Hapatra Vizier of Poisons,Animar Soul of Elements" \
  --instanceid-strict-census
```

## Drive-by fix to unblock the gauntlet

`origin/main @ 44fae29a` doesn't compile — `huginn.ReverseIndex` was
declared as both a function (`reverse_index.go:329`) and an interface
type (`recommender.go:44`) in two recently-merged PRs that landed
without compile-checking against each other. This PR renames the
function to `LookupReverseIndex` (interface keeps the noun name; the
function gets the idiomatic imperative form) so Loki can build.
Sibling test file references updated. Internal-only — no exported-
API consumers outside the huginn package.
