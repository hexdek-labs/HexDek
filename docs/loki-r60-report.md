# Loki r60 — Chaos Gauntlet Report

**Date:** 2026-05-24
**Branch:** `dev/loki-r60-fuzz` (cut from `origin/main` @ 6eabfd0)
**Binary:** `cmd/hexdek-loki`
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42 --nightmare-boards 0 --report data/rules/CHAOS_REPORT_R60_FUZZ.md`
**Raw artifacts:** `data/rules/CHAOS_REPORT_R60_FUZZ.md`, `data/rules/loki-r60-fuzz.log` (gitignored under the `data/` rule).

## Headline

| Phase            | Volume      | Crashes | Invariant Hits | Clean   |
|------------------|-------------|---------|----------------|---------|
| Chaos games      | 5000 games  | **0**   | 52 (26 games)  | 4974    |
| Nightmare boards | skipped     | —       | —              | —       |

Throughput: 70 games/s. Wall time: 1m11s. **Zero panics, zero recovers.**

## Invariant breakdown — chaos games

| Invariant              | r41 (seed 41) | r44 (seed 42) | **r60 (seed 42)** | Δ vs r44 |
|------------------------|---------------|---------------|-------------------|----------|
| CardIdentity           | 832           | 260           | **2**             | **−258 / −99%** |
| ZoneConservation       | 790           | 124           | **0**             | **−124 / −100%** |
| ZoneCastGrantExpiry    | 8             | 4             | **42**            | **+38 / +950%** ⚠️ |
| TriggerCompleteness    | 8             | 4             | **6**             | +2 / +50% |
| AttachmentConsistency  | 14            | 8             | **0**             | **−8 / −100%** |
| CombatLegality         | —             | 2             | **2**             | flat |
| **Total**              | **1652**      | **402**       | **52**            | **−350 / −87%** |

r60's accumulated zone-conservation work (Adric, Pitmage, Krark, Oketra,
Bontu, Dread, the §614 / commander-redirect cleanups, plus the r60
attachment-consistency fix that landed on this same worktree as PR #84
yesterday) has driven the historical dominant cluster — CardIdentity +
ZoneConservation, 384 of r44's 402 hits — down to **2 of 52**. The two
remaining CardIdentity hits are a single signature: `"Abuelo, Ancestral
Echo" appears in both seat 1 command_zone and seat 3 exile` (game 1173,
×2). That's a §903.9b commander-redirect path on a card with a §704.5d
exile-redirect interaction — same shape as the Oketra and Bontu fixes
from r48 / r56, but on a card whose owner+controller diverge at the
moment the §614 chain runs. Low priority next.

## Worst NEW regression — ZoneCastGrantExpiry 4 → 42 (+950%)

ZoneCastGrantExpiry is now **81 % of all remaining violations** and is
the only invariant that moved meaningfully in the wrong direction since
r44. The seed-42 r41/r44 baselines both sat at 4–8 hits; the r60
breakdown shows three distinct grant shapes failing cleanup:

| ×N | Card (grant) | Zone | Duration | Source | grantTurn |
|----|--------------|------|----------|--------|-----------|
| 2 | Commander's Plate | exile | `until_end_of_turn` | The Infamous Cruelclaw | 58 |
| 2 | Huatli's Final Strike | graveyard | `while_source_on_bf` | Yawgmoth's Agenda | 34 |
| 1 | Case of the Shifting Visage | exile | `until_end_of_turn` | Narset, Enlightened Master | 56 |

Two of these are exactly the *family* the `dev/zonecast-grant-expiry-r60`
PR (#83, merged 2026-05-23) was supposed to close — `until_end_of_turn`
heist / Narset-style exile-cast grants whose EOT cleanup was missed
because the per-call-site `Duration` field wasn't stamped on the
permission. The fact that Commander's Plate (heist via Cruelclaw) and
Case of the Shifting Visage (Narset cast-from-exile) still appear
points at a third arm of `resolveModificationEffect` / `resolveResidualByText`
that builds a `FreeCastFromExilePermission` and forgets the duration
stamp, OR an `ExpireZoneCastGrants` call site that the EOT phase walker
isn't reaching for high-turn games (turn 58 / 56 — both well past
typical EOT cleanup ticks).

The Huatli's Final Strike row is a different shape: `while_source_on_bf`
with `source=Yawgmoth's Agenda`. The invariant fires only when the
grant has expired, so Agenda must have *left* the battlefield without
the grant being torn down. The new r60 `play_from_graveyard` primitive
that powers Yawgmoth's Will / Past in Flames / Recoup / Will of the
Jeskai routes through this `while_source_on_bf` shape; the
LTB-cleanup hook for that permission family is the suspect.

**Most likely root cause:** the r60 ZoneCastGrant work added new
grant-construction sites (Cruelclaw heist arm, Yawgmoth's Agenda
play-from-graveyard, Narset exile-cast) on top of the surface that
PR #83 fixed, but the corresponding cleanup wiring missed those new
sites. Bias toward auditing every `NewFreeCastFromExilePermission`
caller for both `Duration` stamping AND an unregister hook on either
EOT (`until_end_of_turn`) or source-LTB (`while_source_on_bf`).

## Other small clusters

- **TriggerCompleteness (6)** — three signatures, none new: `Bhaal,
  Lord of Murder` on `death event "sacrifice"` (×2), `Urza, Prince of
  Kroog` on `sba_704_5g` (×2), `Gerrard, Weatherlight Hero` on
  `sba_704_5g` (×1, +1). All three were called out as latent
  signatures in r57's "Marchesa / Gisa / Jenova" honorable-mention
  cluster — death-trigger dispatchers gated on `dies`/`sba_704_5g`
  but not `sacrifice` as a die-kind variant. Volume +2 vs r44 is
  inside fuzz noise.
- **CombatLegality (2)** — `"Behemoth of Vault 0" attacking with
  summoning sickness and no haste`. Same row as r44 — bit-stable
  signature, unfixed background.
- **CardIdentity (2)** — Abuelo / commander-redirect, see above.
- **AttachmentConsistency (0)**, **ZoneConservation (0)** — both
  closed for this seed at this volume. The r41/r44 dominant cluster
  is gone.

## Top correlated commanders

| Rank | Commander | Dirty games | Clean games | Correlation |
|------|-----------|-------------|-------------|-------------|
| 1 | Shrouded Shepherd // Cleave Shadows | 2 | 3 | 0.40 |
| 2 | Calix, Destiny's Hand | 2 | 4 | 0.33 |
| 3 | Boartusk Liege | 2 | 4 | 0.33 |

All correlations sit at or below 0.40 — there is no single dominant
commander driving the residual, unlike r41's Cerulean Sphinx
(game 137) or r44's Adric. The residual is a multi-source long tail
of zonecast-grant cleanup misses across the r60 grant-construction
surface area.

## Reproduction

```
git fetch origin main
git checkout -B dev/loki-r60-fuzz origin/main
go run ./cmd/hexdek-loki --games 5000 --seed 42 --nightmare-boards 0 \
  --report data/rules/CHAOS_REPORT_R60_FUZZ.md
```

Raw report and progress log are gitignored under the existing `data/`
rule.

## Recommended next moves

1. **Audit `NewFreeCastFromExilePermission` callers** for missing
   `Duration` stamp + missing unregister hook. The PR #83 fix was
   per-call-site; the residual suggests at least one more site
   (Cruelclaw, Case of Shifting Visage variant of the Narset arm).
2. **Audit `while_source_on_bf` grant family** — specifically the new
   r60 `play_from_graveyard` primitive (Yawgmoth's Agenda) — for
   LTB-cleanup wiring.
3. **TriggerCompleteness `sacrifice` dispatch widening** — single
   dispatcher fix would clear the Bhaal / Marchesa / Gisa / Jenova
   family (per r57 plan). 6 hits across 5000 games is low priority
   but the fix is cheap.
4. CombatLegality + Abuelo CardIdentity stay in the open table —
   too small to prioritize.

## Caveats

- Single seed (42). Per-game seeds are stable, so signatures map
  cleanly across r44 / r57 / r60 at the same seed.
- Nightmare boards skipped (`--nightmare-boards 0`) to keep the
  run inside the wall-clock budget; r41 found 6 board-only
  CardIdentity hits in 10000 boards and r57 found none, so the
  signal-to-cost is poor at this stage.
- "Shown details" caps at 5 per invariant kind. With only 42
  ZoneCastGrantExpiry hits in total and 5 shown details split
  across 3 signatures, the tail is small enough that the breakdown
  is likely representative.
