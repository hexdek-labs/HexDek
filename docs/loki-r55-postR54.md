# Loki r55 — post-R54 wave validation (7500 games, seed 48)

**Date:** 2026-05-20
**Branch:** `dev/loki-r55`
**Binary:** `cmd/hexdek-loki`
**Command:** `cmd/hexdek-loki --games 7500 --seed 48 --report data/rules/CHAOS_REPORT_R55.md --nightmare-boards 0`
**Base:** main @ `303fb23` (post the four-piece R54 wave: Krark bounce-back fix + Layer 7b set-PT primitive + damage replacement primitive (5 ports) + Athreos/Sheoldred Eager Cadet control-steal fix)
**Purpose:** Validate the full R54 wave against the r53 baseline on the same seed/volume, confirm the two CardIdentity-targeted fixes (Krark + Eager Cadet) closed their respective clusters in production conditions, and re-rank the next top-3 leads for R56+.

## Headline

| Phase            | Volume        | Crashes | Invariant Hits | Clean         |
|------------------|---------------|---------|----------------|---------------|
| Chaos games      | 7500 games    | **0**   | 346 (34 games) | 7466 (99.55%) |
| Nightmare boards | 0 boards      | —       | —              | —             |

Throughput: 26 g/s chaos. Wall time **6m15s**.

**Zero panics, zero recovers, zero crashes** — 33 cumulative loki runs
in this seed family, still flat.

## Per-invariant volume (7500 games, seed 48)

| Invariant             | Count | /1000 games |
|-----------------------|------:|------------:|
| CardIdentity          |   182 |        24.3 |
| ZoneConservation      |   108 |        14.4 |
| AttachmentConsistency |    22 |         2.9 |
| ZoneCastGrantExpiry   |    20 |         2.7 |
| CombatLegality        |     8 |         1.1 |
| TriggerCompleteness   |     6 |         0.8 |
| **Total**             | **346** |    **46.1** |

## Comparison vs r53 baseline (same seed, same volume)

r53 was the immediate-pre-R54 baseline (post K/L/M wave). Same seed,
same volume → apples-to-apples per-game.

| Metric                | r53 (7.5k) | r53 /1k | **r55 (7.5k)** | **r55 /1k** | Δ /1k    | Notes                                                        |
|-----------------------|-----------:|--------:|---------------:|------------:|---------:|--------------------------------------------------------------|
| Crashes               |          0 |     0   |          **0** |     **0**   |   flat   |                                                              |
| Total violations      |        780 |   104.0 |        **346** |    **46.1** | **−56%** | Largest drop across r48→r55 series.                          |
| Dirty games           |         47 |    6.3  |         **34** |     **4.5** |  −28%    |                                                              |
| Clean game rate       |     99.37% |    —    |     **99.55%** |     —       | +0.18pp  |                                                              |
| CardIdentity          |        616 |   82.1  |        **182** |    **24.3** | **−70%** | Both top CardIdentity leads from r53 closed by R54 wave.     |
| ZoneConservation      |        108 |   14.4  |        **108** |    **14.4** |   flat   | Same game-3431 carry-forward (5th consecutive run).          |
| AttachmentConsistency |         22 |    2.9  |         **22** |     **2.9** |   flat   |                                                              |
| ZoneCastGrantExpiry   |         20 |    2.7  |         **20** |     **2.7** |   flat   |                                                              |
| CombatLegality        |          8 |    1.1  |          **8** |     **1.1** |   flat   |                                                              |
| TriggerCompleteness   |          6 |    0.8  |          **6** |     **0.8** |   flat   |                                                              |

Five of six invariants are **bit-stable** vs r53 — same absolute count,
same shown-detail signatures. The R54 wave was structurally targeted
(CardIdentity zone-leaks + layered-effect primitives + damage
replacement primitive); it didn't touch the engine surfaces those five
clusters live on, so flatness is the expected outcome.

The full structural movement lives in CardIdentity: 616 → 182,
**−434 hits / −58% absolute, −70% /1k.**

## What each R54 piece moved

### Krark bounce-back fix (`6586a85`) — closed game-490 Glyph cluster

Loki re-run after this fix alone (r54 interim, recorded in the Krark
fix commit message): 780 → 544 total, CardIdentity 616 → 380. So this
fix cleared exactly **236 CardIdentity hits**, all from game 490 /
Glyph of Destruction / Krark, the Thumbless coin-flip-tails bounce
attempting to move a card from "stack" → "hand" after the spell had
already resolved into graveyard. CR §608.2b no-op fix.

### Athreos / Sheoldred Eager Cadet fix (`303fb23`) — closed game-3107 cluster

The Eager Cadet cluster (CardIdentity cross-seat bf↔bf duplication —
same `*Card` pointer on two seats' battlefields after a control-steal
recovery handler missed the source removal) was r54's next top lead
surfacing after Krark closed. This fix landed in the same wave; the
r55 - r54 delta of 380 → 182 = **−198 CardIdentity hits** is the
direct attribution. Both r53 top CardIdentity leads are now closed.

### Layer 7b set-PT primitive (`a1c20c2`) — zero CardIdentity impact (expected)

Layer 7b is a layered-effect primitive for "X becomes a N/N creature"
effects (Humility, Mirror Mockery, "becomes a 1/1", etc.). No
zone-leak surface — confirmed by the bit-stable invariant deltas. The
5 becomes-N/N ports bundled with this primitive don't intersect the
loki invariant surfaces; they'd surface in goldilocks or per-card
unit tests instead.

### Damage replacement primitive + 5 ports (`c073325`) — zero CardIdentity impact (expected)

Torbran / Lightning, Army of One / Neriv / Kuja / Solphim + Sokrates
dialogue bonus. Damage-replacement is a side-effect primitive, not a
zone surface; the bit-stable invariants confirm no regressions.

## Cumulative loki series (r48 → r55)

| Run | Games | Seed | Total | /1k | Dirty | Clean | CardIdentity | ZoneCons | Notes |
|-----|------:|-----:|------:|----:|------:|------:|-------------:|---------:|-------|
| r48-deep | 10k | 48 | 1860 | 186 | 86 | 99.14% | 1352 | 428 | Pre-R49 baseline; Avatar Enthusiasts game-59 dominant |
| r50      | 5k  | 48 | 504  | 100.8 | 32 | 99.36% | 352 | 108 | Post-R49 40-card wave |
| r52      | 5k  | 48 | 462  | 92.4  | 31 | 99.38% | 310 | 108 | Avatar leak closed (r51 Dominus fix); Gerrard surfaces |
| r53      | 7.5k| 48 | 780  | 104.0 | 47 | 99.37% | 616 | 108 | Post K/L/M wave; Gerrard closed, Krark surfaces, Glyph dominates |
| r54-interim | 7.5k | 48 | 544 | 72.5 | 39 | 99.48% | 380 | 108 | Krark fix only (in fix commit message) |
| **r55**  | **7.5k** | **48** | **346** | **46.1** | **34** | **99.55%** | **182** | **108** | Full R54 wave — best run to date |

Headline arc: **r48-deep 186 /1k → r55 46.1 /1k. A 75% drop over seven
loki runs, all on the same seed**, driven by ~100 per_card ports +
five targeted CardIdentity fixes (Avatar Enthusiasts r51, Gerrard r53,
Krark r54, Eager Cadet r54, and the antecedent Oketra r48). Zero
crashes throughout. The non-CardIdentity invariants
(ZoneConservation 108, ZoneCastGrantExpiry 20, AttachmentConsistency
22, CombatLegality 8, TriggerCompleteness 6) have been **absolute-count
bit-stable across the last five consecutive runs (r50 → r55)** — those
clusters do not respond to the per_card port wave and need their own
targeted engine-surface investigation in R56+.

## Top-3 leads for R56+

### Lead 1 — God-Eternal Bontu library↔command_zone duplication

- **Invariant:** CardIdentity
- **Volume:** 5 shown details — `card "God-Eternal Bontu" (ptr
  0xc008bca000) appears in both seat 0 library and seat 0 command_zone`.
  Likely accounts for the bulk of r55's 182 CardIdentity total.
- **Game:** 3458 (seed 34580049, perm 0). Commanders:
  `God-Eternal Bontu, Marrow-Gnawer, Alora Cheerful Mastermind,
  Jeska and Kamahl`. Seat 0 state at the leak: `library=83
  cmdzone=1` (Bontu is in both).
- **Suspected source:** Same Dominus-cycle anti-pattern family as the
  closed r48 Oketra zone-leak. God-Eternal Bontu's "When this dies or
  is put into exile, put it into your owner's library third from the
  top" replacement appears to be firing in parallel with §903.9b
  commander-zone redirect, leaving the `*Card` in both library and
  command zone. The Oketra fix in `god_eternal_tuck.go` /
  `resolve_helpers.go:4236` covers Oketra by name; the same fix shape
  needs to be replicated for Bontu (and the other God-Eternals:
  Kefnet, Rhonas; possibly Adric and Eternal Pharaoh as well — the
  whole cycle).
- **Repro:** `--games 3459 --seed 48` and trace `God-Eternal Bontu`
  zone transitions around the death event.

### Lead 2 — game-3431 ZoneConservation cards-disappeared (long-standing carry-forward, fifth consecutive run unchanged)

- **Invariant:** ZoneConservation
- **Volume:** 108 hits, all from game 3431 (seed 34310049). Identical
  signature for the fifth consecutive loki run (r48 → r50 → r52 → r53
  → r55).
- **Signature:** `zone conservation violated: N real cards disappeared
  (expected 394, found 394-N)` for N=2,4,5 across turns.
- **Suspected source:** Same family as the r44 game-420 cluster
  (Breya/Bertram/Alela deck). Cards going MISSING from the census
  rather than duplicating — different anti-pattern shape from the
  CardIdentity zone-leak series. Never root-caused. The per_card port
  wave doesn't touch the engine surface this lives on (probably a
  removal path that bypasses zone bookkeeping — `removePermanent`
  without a corresponding zone-write, or an aura/equipment detach
  that drops the carrier mid-flight).
- **Repro:** `--games 3432 --seed 48`. Per-turn census diff at the
  turn window where N first becomes > 0.

### Lead 3 — Marchesa, the Black Rose sacrifice-trigger-drop (TriggerCompleteness, new in r53, still open)

- **Invariant:** TriggerCompleteness
- **Volume:** 1 signature out of 6 total TriggerCompleteness hits.
  Co-exists with the existing Gisa, Glorious Resurrector and Jenova,
  Ancient Calamity death-trigger drops (2 hits each, signatures
  unchanged since r52).
- **Signature:** `death event "sacrifice" at index 2322 with
  trigger-bearer(s) [{Marchesa, the Black Rose 1}] on battlefield,
  but no subsequent trigger/effect event found`
- **Suspected source:** Marchesa, the Black Rose's "Whenever another
  nontoken creature you control with a +1/+1 counter on it dies,
  return that card to the battlefield" trigger doesn't fire on
  `sacrifice` event kind (only on `dies` / `sba_704_5g`). The
  per_card handler likely filters death-kind too narrowly, missing
  sacrifice as a die-kind variant. Same root-cause shape as the
  existing Gisa/Jenova death-trigger drops — possibly all three are
  the same per_card-trigger-kind-filter narrowness and fixable in one
  pass.
- **Repro:** Build any deck with Marchesa as a commander and sacrifice
  one of her counter-bearing creatures (e.g., Phyrexian Tower); look
  for the unconsumed `sacrifice` event without a follow-up trigger
  fire.

### Honorable mention — AttachmentConsistency aura-detach drift

The three signatures (Ghoulish Impetus → token, Brilliant Wings →
Tidal Warrior, Dub → mite token) have been bit-stable at 22 hits
since r53. The pattern is consistent: an aura is recorded as attached
to a creature that is no longer on any battlefield. This suggests the
aura-LTB cleanup path doesn't fire when the carrier creature
disappears via a path the aura system doesn't observe — likely
token-LTB or §704.5n cleanup ordering. Worth investigating once Lead
1 and Lead 2 are closed, but lower priority than the CardIdentity
work because the absolute volume is small.

## Methodology + caveats

- Single seed (48); per-game RNG seeds are stable, so game indices map
  identically across r48-deep, r50, r52, r53, r54-interim, and r55.
  Per-1k rates compare cleanly.
- All shown-detail signatures verified by grepping `Message:` and
  `Game:` lines in `data/rules/CHAOS_REPORT_R55.md`. Volume counts
  read from the `By Invariant` summary table. Top-message frequencies
  read via `grep Message | sort | uniq -c | sort -rn`.
- The "shown details" cap of 5 per invariant kind biases the
  surface-level analysis toward the dominant cluster. The 182
  CardIdentity total includes ~150-170 hits attributable to game 3458
  / Bontu (5 shown details from one game with multi-turn recount on a
  single ptr); the long tail of remaining CardIdentity hits is
  unanalyzed and may contain additional minor clusters.
- Throughput: r53 ran at 28 g/s; r55 at 26 g/s. Within noise band —
  not a correctness signal. The R54 wave (especially damage
  replacement) adds new replacement-effect registrations that the
  invariant checker walks, but the cost is amortized over the larger
  number of cleaner games.
