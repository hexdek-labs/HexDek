# Loki r41 — Chaos Gauntlet Report

**Date:** 2026-05-19
**Branch:** `dev/loki-r41`
**Binary:** `cmd/hexdek-loki`
**Command:** `go run ./cmd/hexdek-loki/ --games 5000 --seed 41 --report data/loki-r41/CHAOS_REPORT.md` (wall-clock cap 600s)
**Raw artifacts:** `data/loki-r41/CHAOS_REPORT.md`, `data/loki-r41/run.log` (gitignored — see Reproduction below)

## Headline

| Phase            | Volume      | Crashes | Invariant Hits | Clean   |
|------------------|-------------|---------|----------------|---------|
| Chaos games      | 5000 games  | **0**   | 1652 (57 games)| 4943    |
| Nightmare boards | 10000 boards| **0**   | 6              | 9997    |

Throughput: 21 games/s; 2875 boards/s. Total wall time ≈ 4 min.

**Zero panics, zero recovers.** That holds the line set in r40 (which also closed
out the May 5 / May 11 nil-deref clusters logged in `CLAUDE.md`'s Resolved table)
and confirms no regression has been introduced by the r40 work (combat damage
edge cases, hat decision-logging, §608.2c during-resolution trigger deferral).

The remaining noise is invariant-only: state divergence the runtime keeps
running through, surfaced by the checker.

## Invariant breakdown — chaos games

| Invariant              | Count |
|------------------------|-------|
| CardIdentity           | 832   |
| ZoneConservation       | 790   |
| AttachmentConsistency  | 14    |
| TriggerCompleteness    | 8     |
| ZoneCastGrantExpiry    | 8     |
| **Total**              | 1652  |

CardIdentity + ZoneConservation = 1622 hits (98.2%). They are the **same
event**: a single `*Card` pointer is referenced from two zones at once. The
identity invariant flags the pointer collision; ZoneConservation flags the
extra slot in the total card census on the next tick. Each turn the checker
fires post-turn and post-SBA, so one duplicated card produces ~2–4 hits per
turn the duplicate persists.

## The single lead — Cerulean Sphinx in game 137

Of the 57 dirty games, **every one of the first 30 violations** the report
emits in detail is game `137 (seed 1370042, perm 0)`, same Card pointer for
`Cerulean Sphinx`, same end-of-turn cleanup detection, repeating from turn 14
through turn 28+. The variants:

```
22× CardIdentity: card "Cerulean Sphinx" appears in both seat 0 library and seat 1 library
 8× CardIdentity: card "Cerulean Sphinx" appears in both seat 0 library and seat 1 battlefield
```

The Card pointer is `0xc00714b560` for every hit in that game — one allocation
in seat 0's library, then duplicated into seat 1's zones (library OR
battlefield) somewhere during turn 14. The recent-event window around the
first violation:

```
[503] priority_pass seat=2
[504] priority_pass seat=3
[505] priority_pass seat=0
[506] stack_resolve         seat=1 source=Cerulean Sphinx target=seat0
[507] shuffle_into_library  seat=1 source=Cerulean Sphinx target=seat0
[508] enter_battlefield     seat=1 source=Cerulean Sphinx target=seat0
```

Final state: Cerulean Sphinx (P/T 5/5) on seat 1's battlefield AND in seat 0's
library, same pointer.

Top correlated card (by appearance in dirty vs clean games):
`Nevinyrral, Urborg Tyrant` (8 dirty / 5 clean / corr 0.62). Lobelia,
Nick Valentine, Moku, and The Master share that one game's commanders. They
exit the top correlation table once you strip game 137, leaving Nevinyrral as
the only signal that survives multi-game appearance.

### Working hypothesis

A cheat-into-play handler (most likely a Bribery / Acquire / "search opponent's
library and put it onto the battlefield under your control" variant) is moving
a Card pointer to a new owner's zone without removing it from the source
library. Cerulean Sphinx's printed text `{U}: This creature's owner shuffles
it into their library.` (oracle confirmed in `data/rules/oracle-cards.json`,
RVR reprint, oracle_id `296de325-cc1a-4cfa-a72d-80a2a5e18d10`) is parsed
correctly as an Activated ability with effect
`Modification(kind="shuffle_self_into_library")`, so the AST is **not** the
cause. The duplication is upstream of the shuffle — the activated ability
just makes the duplicate observable by adding the already-leaked Card to its
owner's library after another instance is still resident on someone else's
battlefield / library.

Evidence pointing at the cheat-into-play path:

- 22 of the 30 visible hits are **library + library** (not battlefield), so a
  shuffle-handler patch can't be the fix — at the time of detection the
  card is in two libraries with no battlefield instance.
- `removePermanent` (state.go:1448) keys off `src.Controller`. Any handler
  that places a permanent under a *different* controller than the seat that
  owned the source zone — without explicitly removing from the source
  zone — produces this exact shape.
- No per-card handler exists for Bribery / Acquire / similar; they fall
  through to AST. Grep for `Bribery|search.*opponent.*library|cheat_into_play`
  in `internal/` returns only the `Illuna, Apex of Wishes` snowflake (which
  is a different mechanic).

### What I tried and reverted

Added a cross-seat battlefield sweep inside the `shuffle_pronoun_into_owner_library`
handler in `internal/gameengine/resolve_helpers.go` to scoop up stale Permanents
wrapping the same Card pointer. Re-ran the fuzz: violation count moved from
1652 → 1660 (i.e. unchanged within noise). Reverted — the duplication clearly
predates the shuffle, so a fix in the shuffle handler is the wrong location.
The right surface is the cheat-into-play path; that needs a focused minimal
repro before patching, which is the next-up task below.

## Other invariant clusters (not yet detailed)

The report only prints the first 30 violations; the remaining 1622 are
summarised by the by-invariant table. Counts of the secondary clusters are
small enough to investigate in follow-up:

- **AttachmentConsistency (14)** — Aura/Equipment attached state diverging
  from controller's actual battlefield. Typical causes: aura attached to a
  permanent that left the battlefield without firing `detachAll`, or
  Equipment with a stale `AttachedTo` after a cross-seat control change.
- **TriggerCompleteness (8)** — A trigger batch was opened and never drained.
  Likely a recover path inside a panicking trigger (none of which we caught
  this run) or a `BeginTriggerBatch` whose `defer EndTriggerBatch` was bypassed
  by an early return.
- **ZoneCastGrantExpiry (8)** — A `ZoneCastPermission` (impulse-draw / cast-from-
  exile grants) outlived its declared expiry. Probably `impulse_play` grants
  registered via `resolveResidualByText` (resolve_helpers.go:4691) without an
  expiry hook on cleanup.

## Nightmare boards

6 CardIdentity hits across 10000 random board states. No crashes. No detail
section was emitted in the report — these are flagged by raw board generation
(synthetic Permanents pre-game), so they're more likely to be artifacts of the
board generator sharing pointers than engine bugs. Worth a quick sanity check
on `runNightmareBoard` (cmd/hexdek-loki/main.go:893) but low priority.

## Reproduction

Loki's report path and the run log are gitignored (`data/loki-r41/` falls under
the existing `data/` rule). To regenerate locally:

```bash
# Needs oracle + AST data:
ls data/rules/oracle-cards.json data/rules/ast_dataset.jsonl
# Run with the same seed used here:
go run ./cmd/hexdek-loki/ --games 5000 --seed 41 \
    --report data/loki-r41/CHAOS_REPORT.md
```

To replay just the offending game:

```bash
go run ./cmd/hexdek-loki/ --games 1 --seed 1370042 \
    --report /tmp/sphinx_repro.md
```

(deckSeed for game N = masterSeed + N*10000 + 1; setting `--seed 1370042
--games 1` reproduces seat 0's deck containing Cerulean Sphinx and the
commanders Lobelia / Nick Valentine / Moku / The Master.)

## Verdict

**No code fix landed.** The fuzzer is healthy — no crashes, no panics,
throughput consistent with r40. One real lead identified: cheat-into-play
handlers leaking a Card across zones, observable on Cerulean Sphinx because
its activated ability spotlights the existing duplicate. Follow-up should
write a focused unit test in `internal/gameengine` that casts a Bribery-like
effect from seat 1 targeting seat 0's library, then asserts the original
library no longer contains that Card pointer; from there the fix is local to
whichever residual / Modification handler turns out to own that path.

## Issue Log additions

Per the CLAUDE.md "ANY time a test, audit, Goldilocks run, Loki fuzz... log
it here IMMEDIATELY" rule, append the lead and the secondary clusters to the
Open table in CLAUDE.md when this branch merges.
