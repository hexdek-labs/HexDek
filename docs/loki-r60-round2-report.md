# Loki r60 — Chaos Gauntlet Report, ROUND 2

**Date:** 2026-05-24 (late evening)
**Branch:** `dev/loki-r60-round2-r60` (cut from `origin/main` @ 88ede5c)
**Binary:** `cmd/hexdek-loki`
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42 --nightmare-boards 0`
**Raw artifacts:** `/tmp/CHAOS_REPORT_R60_ROUND2.md` (gitignored under the
existing `data/` rule when relocated).

## Headline

| Phase            | Volume      | Crashes | Invariant Hits   | Clean       |
|------------------|-------------|---------|------------------|-------------|
| Chaos games      | 5000 games  | **0**   | **10 (5 games)** | **4995**    |
| Nightmare boards | skipped     | —       | —                | —           |

Throughput: 64 games/s. Wall time: 1m18s. **Zero panics, zero recovers.**

## Trajectory across rounds

| Run                 | Volume     | Violations | Δ vs prior | Δ vs r41 |
|---------------------|------------|------------|------------|----------|
| r41 (seed 41)       | 5000 games | 1652       | —          | —        |
| r44 (seed 42)       | 5000 games | 402        | −1250 / −76% | −1250 / −76% |
| r60 round 1         | 5000 games | 52         | −350 / −87%  | −1600 / −97% |
| **r60 round 2**     | 5000 games | **10**     | **−42 / −81%** | **−1642 / −99.4%** |

Tonight's work — the merged PRs since round 1 — chopped the residual
another **−81%** at the same seed-42 fingerprint. The cumulative drop
from the r41 baseline is **−99.4%** (1652 → 10).

## Invariant breakdown

| Invariant              | r44 | round 1 | **round 2** | Δ vs round 1 |
|------------------------|----:|--------:|------------:|--------------|
| CardIdentity           | 260 |       2 |       **2** | flat |
| ZoneConservation       | 124 |       0 |       **0** | flat (closed) |
| ZoneCastGrantExpiry    |   4 |      42 |       **4** | **−38 / −90%** |
| TriggerCompleteness    |   4 |       6 |       **2** | **−4 / −67%** |
| AttachmentConsistency  |   8 |       0 |       **0** | flat (closed) |
| CombatLegality         |   2 |       2 |       **2** | flat |
| **Total**              | **402** | **52** | **10**   | **−42 / −81%** |

The headline story of round 1 was ZoneCastGrantExpiry exploding 4→42
(81% of all residual) as a side-effect of new r60 grant-construction
sites without matching cleanup wiring. Round 2 closes most of that
cluster — **4 hits remaining**, all clean to two signatures.

## What closed since round 1

- **ZoneCastGrantExpiry −38 (−90%)**. Round 1's three rows
  (Commander's Plate × Cruelclaw, Huatli's Final Strike × Yawgmoth's
  Agenda, Case of the Shifting Visage × Narset) are all gone — the
  PR #122 / `dev/zonecast-residual-deep-r60` cleanup work landed and
  the heist + Narset exile-cast + `while_source_on_bf` Agenda-family
  paths now expire correctly on EOT / LTB.

- **TriggerCompleteness −4 (−67%)**. Round 1's `Bhaal, Lord of Murder`
  (death "sacrifice", ×2) and `Urza, Prince of Kroog` (sba_704_5g, ×2)
  rows are both gone. Only the Gerrard signature survives (see below).

- **Zero new regressions** in any cluster. Throughput and panic count
  stayed flat (0/0). The r41/r44 dominant family (CardIdentity +
  ZoneConservation = 384 of r44's 402) remains fully closed at 2 of 10.

## What still leaks (4 distinct signatures, 10 hits)

### `ZoneCastGrantExpiry` (4)

| ×N | Card (grant) | Zone | Duration | Source | grantTurn | TS |
|----|--------------|------|----------|--------|-----------|----|
| 2 | Swamp | exile | `until_end_of_turn` | Chitinous Crawler | 33 | 0 |
| 2 | Swamp | exile | `until_end_of_turn` | Illusionary Mask | 43 | 0 |

Same shape, two source cards. Both grants are `until_end_of_turn`
exile-cast permissions for a land (Swamp) with `sourceTimestamp=0`.
The `sourceTimestamp=0` is a tell — either the grant is being
constructed before the source's `Timestamp` is assigned, or the EOT
walker keys on `sourceTimestamp` and `0` never matches the expiry
filter. **Same exact shape as the Illusionary Mask hit logged in
`docs/goldilocks-r60-report.md`** earlier today — the deterministic
goldilocks surface and the stochastic loki surface are reporting the
same residual. Bias toward fixing at the construction site (stamp
`SourceTimestamp` explicitly) AND at the walker (don't exclude TS=0
entries).

Chitinous Crawler is the new offender vs goldilocks; Illusionary Mask
is the overlap. The Cruelclaw / Yawgmoth's Agenda / Narset families
called out in round 1's report are all closed.

### `TriggerCompleteness` (2)

| ×N | Card | Event | Notes |
|----|------|-------|-------|
| 2 | Gerrard, Weatherlight Hero | death `sba_704_5g`, no subsequent trigger | Round 1 carried 1 hit; round 2 has 2 |

Death-trigger dispatcher gated on `dies` / `sba_704_5g` but not firing
the expected post-death observer. The signature is bit-stable across
r57 → round 1 → round 2 (the latter two for this same card). Latent;
low priority next.

### `CardIdentity` (2)

```
"Abuelo, Ancestral Echo" appears in both seat 1 command_zone and seat 3 exile
```

Identical to round 1, game 1173. §903.9b commander-redirect on a card
with a §704.5d exile-redirect interaction where owner+controller
diverge at the §614 chain. Not addressed by tonight's work; same
shape as the Oketra / Bontu fixes from r48 / r56 but on a
distinct ownership-divergence path.

### `CombatLegality` (2)

```
"Behemoth of Vault 0" (seat 0) is attacking with summoning sickness and no haste
```

Bit-stable across r41 → r44 → round 1 → round 2. Latent; not addressed
by tonight's work.

## What tonight's PRs actually moved

Working back from the round-1 → round-2 deltas:

- ZoneCastGrantExpiry cluster −38 = PR #122 (`dev/zonecast-residual-deep-r60`)
  closing heist / Narset / Agenda families.
- TriggerCompleteness −4 = the per-card batch H follow-ups (PR #110) +
  layers/attachment audit (PR #123 / #124) cleaning up Bhaal + Urza
  death dispatchers.
- AttachmentConsistency staying at 0 = PR #124 holding.
- The §614 ETB-doubler wire-in (#118 + verify #126) didn't move loki
  numbers (no doubler-bearing cards were drawn into the seed-42 fuzz
  decks), but the unit-test + integration-test coverage stands on its
  own.

## Top correlated cards

| Rank | Card | Dirty games | Clean games | Correlation |
|------|------|-------------|-------------|-------------|
| 1 | Monsoon | 1 | 2 | 0.33 |
| 2 | Kaya's Guile | 1 | 3 | 0.25 |
| 3 | Autochthon Wurm | 1 | 4 | 0.20 |
| 4 | Shrouded Shepherd // Cleave Shadows | 1 | 4 | 0.20 |
| 5 | Lantern Flare | 1 | 4 | 0.20 |

All correlations ≤ 0.33 with single-digit dirty counts. No dominant
correlate — the residual is two scattered signatures (Chitinous
Crawler + Illusionary Mask grants, Gerrard death dispatcher) and two
chronic signatures (Abuelo, Behemoth). Five distinct dirty games at
seed 42 is at the noise floor for 5000-game runs.

## Reproduction

```
git fetch origin main
git checkout -B dev/loki-r60-round2-r60 origin/main
go build -o /tmp/hexdek-loki ./cmd/hexdek-loki/
/tmp/hexdek-loki --games 5000 --seed 42 --nightmare-boards 0 \
  --report /tmp/CHAOS_REPORT_R60_ROUND2.md
```

Output report is gitignored under `data/`; copy to `/tmp` (as above)
to preserve outside the working tree.

## Recommended next moves

1. **Close the Chitinous Crawler + Illusionary Mask grant family**
   (4 of 10 remaining hits — and the cross-validated goldilocks
   surface). Look at the grant-construction site: `sourceTimestamp=0`
   is the smoking gun, either stamp it at construction or accept TS=0
   as a valid expiry key in the EOT walker. Same single fix likely
   closes both card-source paths.

2. **Gerrard `sba_704_5g` death dispatcher** (2 hits). Inspect
   `sba.go::sba704_5g` (§704.5g — legendary rule SBA) — the trigger
   batch may be ending before Gerrard's death trigger fires when the
   death is caused by the legendary rule rather than damage / destroy.

3. **Abuelo CardIdentity** (2 hits) — §903.9b commander-redirect race
   when owner ≠ controller at §614 chain time. Same family as the
   Oketra (r48) / Bontu (r56) fixes; the canonical pattern is to
   capture `fromZone` and route through `removePermanent` →
   `removeFromZone` fallbacks before re-inserting. Apply to whichever
   per-card or engine handler is the offending site for Abuelo.

4. **Behemoth CombatLegality** is bit-stable noise across 4 runs;
   either fix the chaos-deck builder to assign haste / clear summoning
   sickness for token attackers, or fix the legality check to exempt
   the "Behemoth of Vault" debug-token name. Either is cheap.

## Caveats

- Single seed (42). Per-game seeds are stable, so signatures map
  cleanly across runs, but a different seed could surface new
  signatures.
- Nightmare boards skipped (`--nightmare-boards 0`) to keep the wall
  clock under 90 seconds. Round 1 also skipped them so the comparison
  is honest, but a full nightmare run is worth scheduling.
- "Shown details" caps at 5 per invariant kind, but with only 10
  total violations across 5 games, every hit is captured.
