# Loki r57 — post-R54/R55/R56 7500 games (top-3 leads for R58+)

**Date:** 2026-05-20
**Branch:** `dev/loki-r57`
**Binary:** `cmd/hexdek-loki`
**Command:** `cmd/hexdek-loki --games 7500 --seed 48 --report data/rules/CHAOS_REPORT_R57.md --nightmare-boards 0`
**Base:** main @ `9146e11` (post the full R54/R55/R56 wave: Krark + Athreos + Layer 7b + damage replacement (R54), Bontu cycle command_zone fix (R56), R55 batch (10 ports), Sandman/Namor R56 deferred ports, R56 stub census)
**Purpose:** Re-rank the next top-3 leads now that the four targeted CardIdentity fixes (Avatar Enthusiasts r51, Gerrard r53, Krark + Eager Cadet r54, Bontu cycle r56) have all landed and the per_card port wave has reached steady state.

## Headline

| Phase            | Volume        | Crashes | Invariant Hits | Clean         |
|------------------|---------------|---------|----------------|---------------|
| Chaos games      | 7500 games    | **0**   | 288 (33 games) | 7467 (99.56%) |
| Nightmare boards | 0 boards      | —       | —              | —             |

Throughput: 14 g/s chaos. Wall time **9m10s**.

**Zero panics, zero recovers, zero crashes** — 34 cumulative loki runs
in this seed family, still flat.

## Per-invariant volume (7500 games, seed 48)

| Invariant             | Count | /1000 games |
|-----------------------|------:|------------:|
| CardIdentity          |   124 |        16.5 |
| ZoneConservation      |   108 |        14.4 |
| AttachmentConsistency |    22 |         2.9 |
| ZoneCastGrantExpiry   |    20 |         2.7 |
| CombatLegality        |     8 |         1.1 |
| TriggerCompleteness   |     6 |         0.8 |
| **Total**             | **288** |    **38.4** |

## Comparison vs r56 baseline (same seed, same volume, post-Bontu fix)

The r56 commit message recorded interim measurements at exactly these
values. r57 is bit-stable vs r56 — confirming that the R55 batch (10
ports) and the Sandman/Namor R56 deferred ports landed without
touching any of the loki invariant surfaces. Expected outcome for
per_card stub ports; the loki invariants only catch zone-leak and
trigger-completeness families, while the port wave moved AST-keyword,
layer-effect, and replacement-effect surfaces.

| Metric                | r56 (7.5k) | r57 (7.5k) | Δ      |
|-----------------------|-----------:|-----------:|-------:|
| Crashes               |          0 |          0 |  flat  |
| Total violations      |        288 |        288 |  flat  |
| Dirty games           |         33 |         33 |  flat  |
| Clean game rate       |     99.56% |     99.56% |  flat  |
| CardIdentity          |        124 |        124 |  flat  |
| ZoneConservation      |        108 |        108 |  flat  |
| AttachmentConsistency |         22 |         22 |  flat  |
| ZoneCastGrantExpiry   |         20 |         20 |  flat  |
| CombatLegality        |          8 |          8 |  flat  |
| TriggerCompleteness   |          6 |          6 |  flat  |

## Cumulative loki series (r48 → r57)

| Run | Games | Total | /1k | CardIdentity | ZoneCons | AttachCons | Clean % | Notes |
|-----|------:|------:|----:|-------------:|---------:|-----------:|--------:|-------|
| r48-deep | 10k | 1860 | 186 | 1352 | 428 | 34 | 99.14% | Pre-R49 baseline |
| r50      | 5k  | 504  | 100.8 | 352 | 108 | 10 | 99.36% | Post-R49 40-card wave |
| r52      | 5k  | 462  | 92.4 | 310 | 108 | 10 | 99.38% | Post-Avatar fix (r51) |
| r53      | 7.5k| 780  | 104.0 | 616 | 108 | 22 | 99.37% | Post K/L/M; cluster hunt |
| r55      | 7.5k| 346  | 46.1 | 182 | 108 | 22 | 99.55% | Post R54 wave (Krark + Athreos + L7b + damage) |
| r56      | 7.5k| 288  | 38.4 | 124 | 108 | 22 | 99.56% | Post Bontu cycle fix |
| **r57**  | **7.5k** | **288** | **38.4** | **124** | **108** | **22** | **99.56%** | **bit-stable vs r56** |

Headline arc: **r48-deep 186 /1k → r57 38.4 /1k = −79% over nine loki
runs, zero crashes throughout**. The non-CardIdentity invariants
(ZoneConservation 108, AttachmentConsistency 22, ZoneCastGrantExpiry
20, CombatLegality 8, TriggerCompleteness 6) have been **absolute-count
bit-stable across the last SEVEN consecutive runs (r50 → r57)**. Those
clusters genuinely live on engine surfaces the per_card / CardIdentity-
fix waves do not touch.

## Top-3 leads for R58+

The CardIdentity-fix series (Avatar, Gerrard, Krark, Eager Cadet,
Bontu) closed five clusters in five runs. The next CardIdentity
target sits on a different anti-pattern family — **aura mechanics**
— and ties together two of the previously-flat clusters
(AttachmentConsistency + CardIdentity). The aura family is now the
clear top priority.

### Lead 1 — Mire's Grasp aura exile↔battlefield duplication

- **Invariant:** CardIdentity
- **Volume:** 5 shown details on the same `*Card` pointer — `card
  "Mire's Grasp" (ptr 0xc009d8dc20) appears in both seat 0 exile and
  seat 0 battlefield`. Likely accounts for the bulk of r57's 124
  CardIdentity total via per-turn recount.
- **Game:** 3744 (seed 37440049, perm 0). Commanders: `Fire Lord
  Azula, Kyodai, Soul of Kamigawa, Akul the Unrepentant, Zenos yae
  Galvus // Shinryu, Transcendent Rival`. Seat 0 census at turn 38
  cleanup: `library=81 hand=1 graveyard=4 exile=1 battlefield=12`
  — the same Mire's Grasp `*Card` exists in both `exile` and as one
  of seat 0's 12 battlefield permanents.
- **Suspected source:** Aura exile-and-return anti-pattern. Mire's
  Grasp is `Enchant creature` ("Enchanted creature gets -1/-1"). The
  likely path: an effect exiled the enchanted creature (Akul-style
  sacrifice, Shinryu transform, or a flicker), the aura got exile-
  linked to the leaving creature, but a separate `*Permanent` for the
  aura was re-materialized on battlefield by an attach/blink path
  that didn't observe the exile-link. CR §702.32 (Aura) + §704.5n
  (unattached aura SBA → owner's graveyard) should have routed the
  aura to graveyard, not battlefield. Different anti-pattern family
  from the Dominus-cycle zone-leak series — the leak isn't in a
  per_card death-trigger but in the aura attach/detach machinery.
- **Repro:** `--games 3745 --seed 48` and trace `Mire's Grasp` zone
  transitions and attachment events around turn 36-38.
- **Related:** Lead 3 below (AttachmentConsistency aura-detach drift)
  is likely the same machinery surfacing different symptoms; fixing
  Lead 1 may close Lead 3 in the same pass.

### Lead 2 — game-3431 ZoneConservation cards-disappeared (carry-forward, seventh consecutive run)

- **Invariant:** ZoneConservation
- **Volume:** 108 hits, all from game 3431 (seed 34310049). Identical
  signature for the **seventh consecutive loki run** (r48 → r50 → r52
  → r53 → r55 → r56 → r57).
- **Signature:** `zone conservation violated: N real cards
  disappeared (expected 394, found 394-N)` for N=2,4,5 across turns.
- **Suspected source:** Same family as the r44 game-420 cluster
  (Breya/Bertram/Alela deck). Cards going MISSING from the census
  rather than duplicating — a removal path bypasses zone bookkeeping.
  The per_card port wave doesn't touch the engine surface this lives
  on. Suspect candidates: aura-detach LTB ordering that drops the
  carrier mid-flight, equipment-detach during commander-zone redirect,
  or a token-on-token replacement that orphans a `*Card` pointer
  without zone write.
- **Repro:** `--games 3432 --seed 48`. Per-turn census diff at the
  turn window where N first becomes > 0. The same Breya/Bertram/Alela
  decklist will reproduce.

### Lead 3 — AttachmentConsistency aura-detach drift (related to Lead 1)

- **Invariant:** AttachmentConsistency
- **Volume:** 22 hits, three signatures bit-stable since r53:
  - `"Ghoulish Impetus" (seat 1) is attached to "creature token black
    zombie giant Token" which is not on any battlefield` (×N)
  - `"Brilliant Wings" (seat 0) is attached to "Tidal Warrior" which
    is not on any battlefield` (×N)
  - `"Dub" (seat 2) is attached to "creature token phyrexian mite
    Token" which is not on any battlefield` (×N)
- **Suspected source:** Same aura attach/detach machinery as Lead 1.
  When the carrier creature (often a token) leaves the battlefield via
  a path the aura system doesn't observe — token §704.5d cleanup
  ordering vs aura §704.5n cleanup, or a flicker that re-mints the
  carrier without re-attaching its auras — the aura's `AttachedTo`
  field stays set to the now-off-battlefield carrier. The invariant
  detects the stale pointer at the cleanup step.
- **Investigation order:** Almost certainly the same fix surface as
  Lead 1. The exile↔battlefield duplication in Lead 1 manifests when
  the aura `*Card` is reused across the attach/detach boundary; the
  stale-AttachedTo in Lead 3 is the lower-volume sibling where the
  duplication doesn't materialize but the attachment field drift
  does. Worth running the fix once the leak root cause is identified
  and seeing if Lead 3 closes for free.

### Honorable mention — Marchesa / Gisa / Jenova TriggerCompleteness sacrifice-trigger-kind narrowness

The three TriggerCompleteness signatures (Gisa, Glorious Resurrector
×2; Jenova, Ancient Calamity ×2; Marchesa, the Black Rose ×1 with the
`death event "sacrifice"` qualifier) have been bit-stable since r53.
All three look like the same root cause: per_card death-trigger
handlers filter on `dies` / `sba_704_5g` event kinds but not on
`sacrifice` as a die-kind variant. Volume is small (6 total) so
priority is lower than the aura family, but the fix is likely a
single dispatcher widening — three card-handler signatures fixable
in one pass. Worth landing alongside Lead 1 if budget permits.

## Methodology + caveats

- Single seed (48); per-game RNG seeds are stable, so game indices
  map identically across r48-deep, r50, r52, r53, r55, r56, r57.
  Per-1k rates compare cleanly.
- All shown-detail signatures verified by grepping `Message:` and
  `Game:` lines in `data/rules/CHAOS_REPORT_R57.md`. Volume counts
  read from the `By Invariant` table. Top-message frequencies via
  `grep Message | sort | uniq -c | sort -rn`.
- The "shown details" cap of 5 per invariant kind biases surface
  analysis toward the dominant cluster. r57's 124 CardIdentity total
  is likely 100-120 hits from game 3744 / Mire's Grasp (5 shown
  details from one game with multi-turn recount), with a short tail
  of unsurfaced minor clusters. A `--seed-cmdr "Akul the Unrepentant"`
  fuzz pass would isolate the aura cluster signal.
- Throughput: 14 g/s here (slower than the 26 g/s of r55 and the 28
  g/s of r53). Within noise band — laptop load varies.
