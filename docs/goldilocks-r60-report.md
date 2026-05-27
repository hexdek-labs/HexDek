# Thor Goldilocks — R60 Sweep Report

**Latest re-run: 2026-05-26 (still zero)** — third consecutive sterile sweep, absorbing the ~70 additional merges since the May-25 confirmation (precon-vibes R3-R7 wave, PR #530 cycling-loop coalesce, PR #529 bracket-vs-measured refactor, plus all the per_card consumer wiring waves landing this week). Re-ran from a fresh `dev/goldilocks-r60-sweep` branch on `origin/main`; invocation `go run ./cmd/hexdek-thor/ --goldilocks` reports the identical zero shown below — **31,963 tests, 0 fails, 262.4 ms wall time** (~121,794 tests/s). No new failure categories, no changes to the "Top-10 specific card examples per category" section (still N/A), no additions to CLAUDE.md's issue log warranted. The goldilocks-suite chapter is settled at zero.

**Date:** 2026-05-25 (original sweep) / **2026-05-26 (re-run confirmation)**
**Branch:** `dev/goldilocks-r60-sweep` (built from `origin/main` @ `00bb5bf` on May-25; re-built from current `origin/main` on May-26)
**Invocation:** `hexdek-thor -goldilocks --failures-csv /tmp/goldilocks-r60-sweep.csv` (May-25) / `go run ./cmd/hexdek-thor/ --goldilocks` (May-26)
**Runtime:** 390 ms (May-25, effect+keyword, 82,991 cards/s effect phase) / **262.4 ms (May-26, 121,794 tests/s)**
**Supersedes:** the May 24 baseline previously held in this file (19 invariant
fails); that baseline is preserved in `goldilocks-r60-post-engine-clean.md`
and the path to zero is documented in `goldilocks-r60-zero.md`.

## Headline

```
ZERO FAILURES — fully sterile.
```

| metric                  |       count |
| ----------------------- | ----------: |
| cards loaded (oracle)   |      35,708 |
| cards w/ testable AST   |      31,963 |
| effect tests run        |      30,341 |
| effect passes           |  **30,341** |
| effect dead-effects     |           0 |
| effect panics           |           0 |
| **effect invariants**   |       **0** |
| effect unverified       |           0 |
| skipped (no abilities)  |       4,106 |
| keyword tests           |       2,013 |
| keyword passes          |   **2,013** |
| keyword failures        |           0 |
| keyword panics          |           0 |
| failures.csv rows       |  0 (header only) |

## Failures by invariant

None. The `/tmp/goldilocks-r60-sweep.csv` artifact contains only its header row.

## Top-10 specific card examples per category

Not applicable — there are no failing cards in any category. All 35,708
oracle entries either passed (31,963 with abilities) or were intentionally
skipped (4,106 vanilla / land / placeholder).

## Deltas vs prior reports

| run                                                | date       | invariant | dead-effect | panics | keyword | total |
| -------------------------------------------------- | ---------- | --------: | ----------: | -----: | ------: | ----: |
| R36 baseline (`goldilocks-r36-report.md`)          | 2026-05-17 |         2 |          61 |      0 |       0 |    64 |
| R37 rider rebuild (in-report)                      | 2026-05-17 |         2 |          16 |      0 |       0 |    19 |
| R41 (`goldilocks-r41-report.md`)                   | 2026-05-19 |         1 |          16 |      0 |       0 |    18 |
| R60 baseline (PR #102, prior content of this file) | 2026-05-24 |        19 |           0 |      0 |       0 |    19 |
| R60 post-engine-clean (PR #218)                    | 2026-05-24 |         1 |           0 |      0 |       0 |     1 |
| R60 zero-confirm (PR #237)                         | 2026-05-24 |     **0** |       **0** |  **0** |   **0** | **0** |
| R60 sweep (post-#443)                              | 2026-05-25 |         0 |           0 |      0 |       0 |     0 |
| **R60 re-run (this update, post-precon-vibes-R7 + #530 cycling-coalesce)** | 2026-05-26 | **0** | **0** | **0** | **0** | **0** |

**Cumulative delta vs the R36 starting point: 64 → 0 (−100 %).**
**Delta vs R41: 18 → 0 (−100 %).**
**Delta vs the R60 baseline previously stored in this file: 19 → 0 (−100 %).**

### Trajectory vs the 2026-05-08 keyword_dead fix

| Date | Goldilocks failures | Δ vs prior | Notes |
|------|--------------------:|--:|-------|
| 2026-05-08 (pre-fix) | **1,915** | — | baseline before `makeKeywordGameState` `RetainEvents:true` + combat-scaffold rewrite (CLAUDE.md issue log) |
| 2026-05-08 (post-fix) | **54** | −1,861 (−97.2%) | keyword_dead specifically 1,795 → 0; 54 long-tail non-keyword paths remained |
| 2026-05-25 (zero-confirm sweep) | **0** | −54 (−100%) | residual 54 closed via incidental r60 per_card / engine work |
| **2026-05-26 (this re-run)** | **0** | 0 | three consecutive sterile sweeps; the 1,915 → 0 chapter is settled |

The 24-hour gap between the May 24 zero-confirmation
(`goldilocks-r60-zero.md`, PR #237) and this sweep absorbed ~30 additional
merges (latest landed: #443 Freya NLP oracle, #442 spellbook import, #441
Loki r60 follow-up fuzz, #440 cEDH seat-bias gauntlet, #436 Playwright e2e
suite, and the composition-prior / archetype-tag / hat self-trigger
sub-waves listed in CLAUDE.md "Done 2026-05-25"). None of those merges
re-introduced any goldilocks failure.

## New failure categories vs CLAUDE.md history

**None.** The R36/R41 historical buckets and the R60 invariant clusters
that were live earlier in r60 are all listed below for completeness;
every one of them now reads zero against the current corpus:

| historical bucket                                      | source              | r60 sweep |
| ------------------------------------------------------ | ------------------- | --------: |
| `ability_word` dead static (Threshold / Metalcraft / Coven / Heroic / Magecraft / Spell Mastery / Constellation / Domain / Revolt / Delirium / Raid / Valiant / ...) | R36 §1 (45 hits)    |         0 |
| `sacrifice` triggered on EOT/upkeep, scaffold misses controller/turn (Pestilence / Pyrohemia / Withering Wisps / Task Mage Assembly / Planar Engineering) | R36 §2, R41 (5)     |         0 |
| `modification_effect` triggered, P/T modify unobserved (Lord of Tresserhorn / Scourge of Numai / Reaver Drone / Fathom Fleet Boarder) | R36 §3, R41 (4)     |         0 |
| `exile` triggered, graveyard not seeded (Soul-Guide Lantern / Fishing Gear) | R36 §4, R41 (2)     |         0 |
| `lose_life` static recurring/global (Fraying Omnipotence / Pox Plague) | R36 §5, R41 (2)     |         0 |
| `create_token` triggered, controller condition unmet (Nightsquad Commando / Trynn) | R36 §6, R41 (2)     |         0 |
| `destroy` activated, target type missing (Demonic Hordes) | R36 §7, R41 (1)     |         0 |
| `sacrifice` activated (Roving Actuator) | R36 §8, R41 (1)     |         0 |
| `TurnStructure` — Lost-without-LossReason (Phage the Untouchable) | R36 §9, R60 baseline (2) |         0 |
| `CardIdentity` — DFC duplicate pointer (Etali, Primal Conqueror // Etali, Primal Sickness) | R36 §10, R41 (1)    |         0 |
| `ZoneCastGrantExpiry` — graveyard/exile cast grant outlived source | R60 baseline (17), Loki r41 (8) |         0 |
| `ResourceConservation` — Lost seat retained ManaPool | R60 post-engine-clean residual (1) |         0 |

No new failure category (not previously listed in r36/r41/r60 reports or
the CLAUDE.md issue log) surfaced in this sweep. The sweep finds zero
candidates of any kind.

## Recommendations

1. **Do not gate further engine work on goldilocks signal.** The
   deterministic per-card surface has been sterile for two consecutive
   sweeps (PR #237 zero-confirm and this run) across ~30 intervening
   merges. The signal has saturated; new bug categories will surface from
   the multi-card / multi-turn surface, not from single-card scaffolds.

2. **Lean on Loki for the live signal.** The CLAUDE.md Resolved table
   shows the productive r60 bug discoveries (CardIdentity, SBA-cap draw,
   TriggerCompleteness, ZoneCastGrant LTB) all came from Loki fuzz, not
   goldilocks. Continue investing in extended-seed Loki sweeps and the
   nightmare-board phase added in the 2026-05-24 cluster. The most recent
   3-seed × 5K verification (PR #441, 2026-05-25) is clean — the next
   useful step is wider-seed depth (10K+ games × 10+ canonical seeds),
   which has already been kicked off per the "Loki r60 canonical-final"
   entry showing 0/0/0 across 100K chaos + 100K nightmare.

3. **Keep goldilocks in the regression matrix, not the discovery loop.**
   Run it post-merge as a tripwire (fast: 390 ms) to catch accidental
   re-introduction of dead effects / per-card invariant breaks during
   future per_card handler work. Don't expect it to produce new findings
   on its own at this point.

4. **The condition/trigger unbucketed-node corpus audit (Resolved
   2026-05-24, all 4 eras swept) remains the standing followup for
   coverage expansion** — that work is in scaffold land, not invariant
   land, and is unaffected by this goldilocks zero.

5. **No new entries needed in CLAUDE.md Issue Log Open table.** This
   sweep produced no bugs. The Resolved table already documents the
   complete 19 → 0 path that closed earlier r60 sweeps.

## Run details

- AST corpus: `data/rules/ast_dataset.jsonl` (47 MB, 31,963 cards)
- Oracle corpus: `data/rules/oracle-cards.json` (165 MB, 35,708 cards)
- Workers: 10 (`runtime.NumCPU()` on this host)
- Phases: off (default)
- Scaffold flag: off (default)
- CSV: `/tmp/goldilocks-r60-sweep.csv` (header row only, 0 failure rows)
- Binary: `/tmp/hexdek-thor-r60-sweep` built from `origin/main` @ `00bb5bf`

## Reproduction

```bash
git checkout origin/main
go build -o /tmp/hexdek-thor ./cmd/hexdek-thor/
/tmp/hexdek-thor -goldilocks --failures-csv /tmp/goldilocks.csv
```

Expected: `ZERO FAILURES — fully sterile.`, 0 rows in
`/tmp/goldilocks.csv` beyond the CSV header.
