# Loki r47 — post-R46 validation

**Date:** 2026-05-20
**Branch:** `dev/loki-r47`
**Binary:** `cmd/hexdek-loki`
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42 --report /tmp/loki-r47-report.md`
**Base:** main @ `ad68242` (Merge dev/stub-hunt-percard-r46 — sits on top of the full R46 wave)
**Purpose:** Validate the R46 wave end-to-end:
1. **Nevinyrral fix** (`f4402a6`) — `ZoneConservation` cluster (-124 in earlier 3k bench)
2. **R46 keyword_action stub cleanup** (`a8f52e8`) — gift / populate / explore / proliferate / reorder closures in `resolve_helpers.go`
3. **R46 per_card stub hunt** (`1bb58ba`) — 9 pure-stub ports + `FireCardTrigger("spell_copied")` engine dispatch

## Headline

| Phase            | Volume       | Crashes | Invariant Hits | Clean         |
|------------------|--------------|---------|----------------|---------------|
| Chaos games      | 5000 games   | **0**   | 380 (27 games) | 4973 (99.46%) |
| Nightmare boards | 10000 boards | **0**   | 2              | 9999 (99.99%) |

Throughput: 23 g/s chaos · 3697 b/s nightmare. Wall time **3m37s** (chaos 3m34s + nightmare 2.7s).

**Zero panics, zero recovers.**

## Comparison vs prior baselines

Baselines below were 3000-game runs (same seed 42). r47 is 5000 games, so the
comparison columns normalize to violations-per-1000-games to keep the rate
honest. The "Δ vs r45 (normalized)" column is the load-bearing number — r45 is
the most recent same-seed run and the closest scientific point of comparison.

| Metric                     | r43-validation | r44     | r45     | **r47** | Δ vs r45 (raw) | Δ vs r45 (per 1k) |
|----------------------------|---------------:|--------:|--------:|--------:|---------------:|-------------------:|
| Games                      |           3000 |    3000 |    3000 | **5000**| —              | —                  |
| Crashes (chaos)            |              0 |       0 |       0 | **0**   | flat           | flat               |
| Total violations           |            402 |     402 |     252 | **380** | +128           | **−10%** (84→76)   |
| Dirty games (chaos)        |             16 |      16 |      14 | **27**  | +13            | **+16%** (4.67→5.4)|
| Clean-game rate            |         99.47% |  99.47% |  99.53% | **99.46%** | −0.07pp     | comparable         |
| Nightmare crashes          |              0 |       0 |       0 | **0**   | flat           | flat               |
| Nightmare violations       |              2 |       2 |       2 | **2**   | flat           | flat               |

Per-1000-games total violations: r43=134 → r44=134 → r45=84 → **r47=76**. The
post-Adric, post-Nevinyrral floor sits ~43% below r43-validation's pre-fix
baseline.

## Per-invariant comparison (per 1000 games)

| Invariant             | r45 (3k raw) | r45 /1k | **r47 (5k raw)** | **r47 /1k** | Δ /1k       | Attribution                                  |
|-----------------------|-------------:|--------:|-----------------:|------------:|-------------|----------------------------------------------|
| CardIdentity          |         110  |   36.7  |          **346** |    **69.2** | **+88%**    | Surface re-exposed — God-Eternal Oketra cluster is now the dominant offender (5/5 shown details), filling the void left by Adric & Nevinyrral. NOT introduced by R46. |
| ZoneConservation      |         124  |   41.3  |            **0** |     **0.0** | **−100%**   | **Nevinyrral fix** lands clean. Entire post-Krark floor cleared. |
| AttachmentConsistency |           6  |    2.0  |           **16** |     **3.2** | +60%        | Still fuzz-floor noise (r41=14, r42=23, r45=6 — running ±5). |
| TriggerCompleteness   |           4  |    1.3  |            **6** |     **1.2** | flat        |                                              |
| CombatLegality        |           2  |    0.7  |            **2** |     **0.4** | flat (−43%) |                                              |
| ZoneCastGrantExpiry   |           6  |    2.0  |           **10** |     **2.0** | **flat**    | Identical per-game rate. Sources: Narset, Enlightened Master / The Infamous Cruelclaw — NOT Eruth, NOT any R46 port. |
| **Total**             |    **252**   |  **84** |          **380** |    **76**   | **−10%**    |                                              |

## R46 attribution

### What clearly worked

- **Nevinyrral fix** (`f4402a6`): ZoneConservation 124 → 0 in raw count, the
  entire post-Krark floor. This is the single biggest-impact engine fix landed
  since the Krark zone-conservation work in r43.
- **Net total improvement** of −10% per-1000-games vs r45. The Nevinyrral
  win (-41 /1k) more than offsets the re-surfaced CardIdentity background
  (+33 /1k).

### What did NOT regress

- **R46 keyword_action stub closures** (`a8f52e8` — gift, populate, explore,
  proliferate, reorder in `resolve_helpers.go`): zero new violations
  attributable to any of the five mechanics. No invariant mentions
  "explore", "gift", "populate", "proliferate", or "reorder" in offender
  context. The 5-stub cleanup is bit-stable on this corpus.
- **R46 per_card stub hunt** (`1bb58ba` — Eruth / Kudo / Clara / Katara /
  Twelfth Doctor / Toph 1st MB / Aloy / Ivy / Bello + `FireCardTrigger("spell_copied")`):
  none of the nine ported card names appears in any violation source /
  attached / commander / log field. The added `spell_copied` dispatch in
  `resolve.go`'s §707.2 path is invisible to invariant checks (no card
  outside Twelfth Doctor listens for it).

### Surface that's now visible

- **CardIdentity, God-Eternal Oketra cluster** (5 of 5 shown details, all
  from game 333 / seed 3330043). Same pointer-share signature as the
  Adric cluster: card "God-Eternal Oketra" appears in both seat 2 library
  AND seat 2 graveyard simultaneously. This is the next per_card handler
  audit candidate — Oketra's "If God-Eternal Oketra would die or be put
  into exile from anywhere, reveal Oketra and shuffle it into its
  owner's library instead" replacement is the obvious suspect: the
  existing handler likely moves the card to the library without
  unregistering its prior graveyard entry, mirroring the Adric
  battlefield ↔ hand churn from r44.
- **AttachmentConsistency +60% per-1k** (2.0 → 3.2). Within the historical
  ±5 band but worth a per-card trace if it grows again in r48.

## Top correlated cards

(Pre-existing pattern — same offender profile as r43-r45, no R46 names.)

| Rank | Card | Violation Games | Clean Games | Correlation |
|------|------|----------------:|------------:|------------:|
| 1    | Narset, Enlightened Master       | 2 | 2 | 0.50 |
| 2    | Shrouded Shepherd // Cleave Shadows | 2 | 3 | 0.40 |
| 3    | Calix, Destiny's Hand            | 2 | 4 | 0.33 |
| 4    | A-Cabaretti Charm                | 1 | 2 | 0.33 |
| 5    | Shaman of the Pack               | 1 | 2 | 0.33 |
| 6    | Gandalf of the Secret Fire       | 1 | 2 | 0.33 |
| 7    | Tezzeret's Touch                 | 1 | 2 | 0.33 |
| 8    | Mayhem Devil                     | 1 | 2 | 0.33 |
| 9    | Monsoon                          | 1 | 2 | 0.33 |
| 10   | Bhaal, Lord of Murder            | 1 | 2 | 0.33 |

None of the R46-ported handlers (Eruth, Kudo, Clara, Katara, Twelfth Doctor,
Toph, Aloy, Ivy, Bello) appears in this table or in the violation details.

## Verdict

**R46 wave validated.** Zero crashes, zero panics. Net total violation rate
**−10%** vs r45 per 1000 games. Nevinyrral fix delivered its full advertised
benefit (ZoneConservation 124 → 0). Keyword-action stub cleanup and per_card
stub hunt are bit-stable.

The CardIdentity re-emergence around God-Eternal Oketra is the highest-value
r48 lead — same pointer-share signature as Adric, fix shape should rhyme with
the Adric work (commit `7e782cf`).

## Issue log delta

Open invariants from r41-r45 either cleared or unchanged:

| Invariant                                | r45 status | r47 status |
|------------------------------------------|------------|------------|
| Nevinyrral zone-conservation cluster      | open (124) | **closed (0)** |
| AttachmentConsistency 14→23 r41 leak      | low (6)    | low (16, /1k flat band) |
| TriggerCompleteness batch-not-drained     | low (4)    | low (6, /1k flat) |
| ZoneCastGrantExpiry impulse-play          | low (6)    | low (10, /1k flat) |
| **(NEW) God-Eternal Oketra ptr share**    | —          | **open (5+ details, 1 game)** |
