# InstanceID Phase F — 5K Loki seed-42 verification

**Date:** 2026-05-30
**Branch:** `dev/zonecons-phase-f-5k-verify-r60` (cut from `origin/main` @ `c79ed512`)
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42 --violations-dump …`
**Worktree:** `.claude/worktrees/r60-2-attachment-consistency`
**Raw log:** `/tmp/loki-phase-f-5k/run.log`
**Violation dump:** `/tmp/loki-phase-f-5k/violations-5k.tsv` (34 entries)

## Headline

**0 crashes / 34 violations in 1 game across 5,000 games — −96.6% vs the Phase E baseline of ~1,000 ZoneConservation residuals on the same seed/depth.**

The §400.7c fabrication cluster that Phase F (#851) targeted is **closed at scale** for the dominant signature it was scoped against:

- Game 411 / `h1OGVR200096` / Distemper of the Blood / 46-hit single-ID replay → **0 hits.** Gone.
- All 286 disappearance-arm hits Phase E flagged as deferred for Phase F+ → **0 hits.** Either absorbed by the universal token-cessation chokepoint in PR #773 / sweep in PR #776 over the 25k run, by PR #853 (Drafna token-copy chokepoint), or never reproducible at this specific seed once the dominant fabrication noise was removed.

Residual: **a single sibling fabrication signature, 34 hits, game 2762.** Same SHAPE as the original Phase F cluster (one ID, single game, replay across many turns), distinct CAUSE.

## Run output

```
=== CHAOS GAMES COMPLETE ===
  games:           5000
  duration:        1m41.913s
  throughput:      49 games/sec
  crashes:         0 (in 0 games)
  violations:      34 (in 1 games)
  clean games:     4999

=== NIGHTMARE BOARDS COMPLETE ===
  boards:          10000
  duration:        2.079s
  throughput:      4810 boards/sec
  crashes:         0
  violations:      0
  clean boards:    10000
```

## Trajectory vs. CLAUDE.md / PR-776 baselines (seed 42, 5000 games)

| Stage | Date | Total | Fabrication (game 411 cluster) | Disappearance | Other | Notes |
|-------|------|------:|-------------------------------:|--------------:|------:|-------|
| Phase D (PR #773) | 2026-05-30 | 98,230 (~3.93/game) | — | — | — | Census still over-counting via mint-coverage gaps |
| Phase E (PR #776) | 2026-05-30 | ~1,000 | 52 | 286 | — | "338 distinct unique bugs"; per Phase E doc the 52 + 286 split |
| Phase F local 500-game | 2026-05-30 | 56 → 10 | 46 → 0 | 10 | — | Single-seed pre-merge verification (`docs/...` in #851) |
| **Phase F 5K verify (this run)** | **2026-05-30** | **34** | **0** | **0** | **34 (Lash Out / game 2762 / Aziza-pod)** | **−96.6% vs Phase E** |

Nightmare phase is bit-stable clean: 0 / 10,000 boards.

## Residual: Aziza, Mage Tower Captain (game 2762)

All 34 hits are the same fabrication signature replaying across turns 44–60:

```
ZoneConservation: InstanceID "h1OGVR200056" (Lash Out) present in a zone but not in (Minted - Ceased) — fabrication or stale ceased entry
```

**Game 2762 pod:** Rona, Herald of Invasion · **Aziza, Mage Tower Captain** · Jaya Ballard, Task Mage · The Mechanist, Aerial Artisan.

Aziza is the suspect — surfaced during the Phase F audit and intentionally deferred. `internal/gameengine/per_card/aziza_mage_tower_captain.go` builds a §707.2 spell-copy stack item by aliasing the original `*Card` pointer directly:

```go
// Push the copy. IsCopy=true so CR §707.10 ceases the spell post-
// resolution and so the engine knows to skip a re-cast pipeline.
copyItem := &gameengine.StackItem{
    Controller: casterSeat,
    Card:       castCard,   // ← original *Card pointer, not DeepCopy'd
    IsCopy:     true,
    Targets:    originatingTargets,
    CostMeta:   map[string]interface{}{},
}
gs.Stack = append(gs.Stack, copyItem)
```

When the copy resolves, `stack.go:1312`'s §707.10 cease path fires
`MarkInstanceIDCeased(item.Card.InstanceID)` — but `item.Card` IS the
source `*Card`. The source's `InstanceID` retires while the underlying
card is still in seat 1's hand / graveyard / wherever. From that point
every invariant tick walks seat 1's zones, finds Lash Out present, and
flags fabrication.

This is structurally worse than the 10 sites Phase F's `MintSpellCopy`
chokepoint closed (`alania` / `zada` / `krark` / `mica` / `mendicant` /
`rootha` / `kalamax` / `ivy` / `fire_lord_azula` / `ulalek` — those at
least called `DeepCopy()` first, sharing the ID via inheritance). Aziza
shares the pointer outright.

## Phase G scope (deferred)

Route Aziza through `MintSpellCopy`. Same one-line patch as the 10 sites
Phase F closed:

```go
copyCard := gameengine.MintSpellCopy(gs, castCard)
copyItem := &gameengine.StackItem{
    Controller: casterSeat,
    Card:       copyCard,
    IsCopy:     true,
    Targets:    originatingTargets,
    CostMeta:   map[string]interface{}{},
}
```

Plus a quick property sweep — grep
`grep -n "Card:\s*castCard\|Card:\s*spellCard\|StackItem{.*Card:[^.]*$" internal/gameengine/per_card/`
for any sibling handler aliasing the source `*Card` pointer. Aziza is
the only known case from the Phase F audit, but the 5k surface
demonstrates a single such site is enough to dominate residual counts
when smaller leaks have been retired.

## Verdict

**The 52-hit §400.7c fabrication cluster Phase F targeted is closed.**
The closure scales: 5,000 games / seed 42 reports 0 Distemper hits and
0 disappearance-arm hits across the seed-42 chaos surface. The 34
residual hits are a single deferred sibling site, queued as Phase G.

The CLAUDE.md issue-log entry for §400.7c fabrications can move to the
Resolved table; the deferred Aziza signature should open as a new Open
row.

— Loki r60 / Phase F / 5K verify
