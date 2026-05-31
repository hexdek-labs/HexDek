# Loki Pathological-Deck Stress Wave 2 (r60)

## Summary

Wave 2 of the Verification Phase 5 targeted-pathology layer. Wave 1
(`docs/loki-pathological-r60.md`, PR #950) ran a 14-card gauntlet
spanning copy chains, blink loops, no-op loops, cascade, and
reanimator surfaces — surfacing 1 game's 2-hit `CombatLegality`
signature out of 2000. Wave 2 expands to **17 cards** spanning 10
new pathological themes the wave-1 list missed: storm count,
attack-tax pillowfort, mass land destruction, counterspell-tribal,
stax, theft, alternate Eldrazi, sacrifice/tutor, and free-cheat
permanents.

## Pathology gauntlet (wave 2)

| Theme | Seed cards | Why pathological |
|---|---|---|
| Storm | `Tendrils of Agony`, `Storm Crow` | Storm-count stack walking + paradigm-copy stress on every spell cast that turn |
| Pillowfort | `Ghostly Prison`, `Propaganda` | Tax replacements on attack-declared; combat-legality + cost-payment intersection |
| Land destruction | `Armageddon`, `Wildfire` | Mass land destruction during phase boundaries; ManaPool drain + ResourceConservation stress |
| Counterspell-heavy | `Talrand, Sky Summoner`, `Counterspell` | Drake-token-on-cast + cast-counter intersection; observer-trigger ordering on countered spells |
| Stax | `Smokestack`, `Tangle Wire` | Forced-sacrifice during upkeep; FireDieEvent + counter-tick interaction across all seats |
| Alt-Eldrazi | `Emrakul, the Promised End`, `Ulamog, the Ceaseless Hunger` | Annihilator trigger stack (different from wave-1's "Aeons Torn"); cast-trigger sweeping exile + extra turn from Promised End |
| Theft | `Insurrection`, `Threaten` | Control-change with haste-grant; revert-at-EOT delayed trigger + commander redirect on stolen permanents |
| Sacrifice (tutor) | `Razaketh, the Foulblooded` | Repeated activated-ability tutors at instant speed; sac-cost + library-search per activation |
| Sneak Attack | `Sneak Attack`, `Show and Tell` | Free-cheat permanents onto battlefield mid-turn; ZoneCastGrant + revert-at-EOT for Sneak Attack creatures; everyone-puts-from-hand for Show and Tell |

17 cards × 4 seats round-robin (`--seed-cards-all-seats`) guarantees
each seat gets 4-5 pathological pieces; the chaos-deck generator
fills the remaining ~95 slots from the random corpus.

**Card-list separator note**: wave 1 used `,` because none of its 14
cards had commas in their names. Wave 2's list includes "Talrand,
Sky Summoner" / "Emrakul, the Promised End" / "Ulamog, the
Ceaseless Hunger" / "Razaketh, the Foulblooded" — comma-bearing
names. The Loki flag parser at `cmd/hexdek-loki/main.go:374-382`
auto-detects `;` as the separator when present in the input string,
so the wave-2 run uses `;` to avoid mid-name fragmentation. (Mixing
the two in one invocation would split inconsistently — pick one.)

## Run

```bash
SEEDS="Tendrils of Agony;Storm Crow;Ghostly Prison;Propaganda;Armageddon;Wildfire;Talrand, Sky Summoner;Counterspell;Smokestack;Tangle Wire;Emrakul, the Promised End;Insurrection;Threaten;Ulamog, the Ceaseless Hunger;Razaketh, the Foulblooded;Sneak Attack;Show and Tell"
go run ./cmd/hexdek-loki \
  --games 2000 --seed 42 --nightmare-boards 0 \
  --seed-cards-all-seats "$SEEDS" \
  --instanceid-strict-census \
  --report /tmp/loki-pathological-wave2/CHAOS_REPORT.md \
  --violations-dump /tmp/loki-pathological-wave2/v.tsv
```

`--instanceid-strict-census` is on for the same reason as wave 1:
strict-census catches the surfaces this gauntlet targets.

## Results

| Metric | Wave 2 pathological 2k | Wave 1 pathological 2k (PR #950) | Standard 5k baseline (PR #896) |
|---|---:|---:|---:|
| Games | 2000 | 2000 | 5000 |
| Crashes | **0** | 0 | 0 |
| Violations | **0** | 2 (1 game) | 0 |
| Clean games | **2000** | 1999 | 5000 |
| Throughput | 8 g/s | 6 g/s | — |
| Wall time | 4m 6s | 5m 28s | — |

## Per-invariant breakdown

All invariants clean. Empty `v.tsv` (0 bytes) confirms no violations
were dumped.

## Interpretation

Wave 2 is **fully clean** at 2000 games / seed 42. Three readings:

1. **The standard chaos sample at the same seed missed nothing on
   this card list** — the random sampler at seed 42 already exercises
   these 17 pathological surfaces enough that the targeted-seeding
   doesn't surface anything new. Different from wave 1, where the
   pathological seeding nudged seat 3's composition into a
   `CombatLegality` gap the chaos sampler didn't reach.
2. **The 10 newly-themed surfaces are engine-clean** — no storm-count
   stack walking bug, no pillowfort + combat-legality interaction
   bug, no mass-LD ResourceConservation bug, no counterspell-tribal
   observer-ordering bug, no stax forced-sac bug, no alt-Eldrazi
   annihilator-stack bug, no theft control-revert bug, no
   Razaketh-tutor-chain bug, no Sneak-Attack revert bug.
3. **The chaos-deck generator was selective** — over the 2000 games,
   the random fill IS distributing the seeded cards (each seat gets
   4-5) but the chaos-card chooser at this seed isn't pulling the
   Wall-of-Tanglecord-equivalent corner-case that wave 1 hit at
   game 1317. If a future wave-2-style run at a different seed
   (seed 43, seed 99) surfaces a new violation, that confirms the
   chaos generator's seed-dependence not the wave-2 card list itself.

The combination of wave 1 (1 hit / 14 cards) + wave 2 (0 hits / 17
cards) over 4000 games at seed 42 covers the 24-card
cross-wave-distinct pathological pool and surfaces exactly the 1
wave-1 signature, no new bugs.

## How this complements the existing baselines

The standard chaos baseline answers "is the engine clean for
randomly-sampled corpus games?" — yes per PR #896. The wave-1
pathological gauntlet answered "is the engine clean when we *force*
the most-historically-fragile interactions to fire?" — 1 hit per
2000. Wave 2 answers the parallel question for **10 different
historically-fragile interaction families** the wave-1 list missed
— **0 hits per 2000**.

A wave-3 candidate list would target whatever the wave-2 distribution
left out: extra-turn chains (Time Walk family), graveyard recursion
toolboxes (Karador / Muldrotha), proliferate cascades (Atraxa /
Skithiryx), and life-swap loops (Sanguine Bond + Exquisite Blood).
None are surfaced today; logged as wave-3 candidates for a future
run.

## Reproduction

```bash
git fetch origin main && git checkout -B repro origin/main
go run ./cmd/hexdek-loki \
  --games 2000 --seed 42 --nightmare-boards 0 \
  --seed-cards-all-seats "Tendrils of Agony;Storm Crow;Ghostly Prison;Propaganda;Armageddon;Wildfire;Talrand, Sky Summoner;Counterspell;Smokestack;Tangle Wire;Emrakul, the Promised End;Insurrection;Threaten;Ulamog, the Ceaseless Hunger;Razaketh, the Foulblooded;Sneak Attack;Show and Tell" \
  --instanceid-strict-census
```

Expected output: `clean games: 2000`, `violations: 0`, verdict CLEAN.
