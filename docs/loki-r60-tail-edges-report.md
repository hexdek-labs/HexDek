# Loki r60 Tail-Edges Report — top-3 signature investigation

**Date:** 2026-05-30
**Branch:** `dev/lokifuzz-tail-edges-r60` (built from `origin/main` HEAD `842d05da`)
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42 --nightmare-boards 0`
**Pre-investigation baseline:** 224 violations / 0 crashes (PR #781's 268 absorbed `-44` via `f374f26b` registry LTB-dispatch fix that landed post-#781)

## Top-3 unique signatures by frequency

| Rank | Hits | Invariant | Signature |
|-----:|-----:|-----------|-----------|
| 1 | 46 | ZoneConservation (fabrication) | `h1OGVR200096` present-but-never-minted (game 411, persistent turns 23-45) |
| 2 | 34 | ZoneConservation (fabrication) | `h1OGVR200056` present-but-never-minted (game 2762, persistent) |
| 3 | 26 | ExileLinkageIntegrity | Leonardo, Sewer Samurai // Leonardo, Sewer Samurai in seat 0 exile, source timestamp 68 missing (game 2029, persistent turns 48+) |

(Followed by 4 fabrication signatures at 7–13 hits each, then ~24 disappearance signatures at 1–2 hits each. CardIdentity is a single-game Spikeshell Harrier same-zone duplicate, 4 hits.)

## Engine-path tracing

### Signature #1 + #2 — `h1OGVR200096` / `h1OGVR200056` fabrication

Both IDs decode as `seat 1 / OG provenance / Visible / color R / CMC 2 / sequence #96 (or #56)`. The IDs persist on the same card across many turns, meaning the *Card pointer they're stamped onto is alive and re-observed every invariant tick — i.e. a single mint-bypass site per game, not a transient.

Hypothesis (not confirmed): a `Card.DeepCopy()` / clone path mints an OG-provenance ID without routing through `MintOGInstanceID` → `RecordMintedInstanceID`. Candidates: `phases.go:946` `copyCard := card.DeepCopy()`, the `MintCopyInstanceID` family in `keywords_batch3.go:207` / `keywords_batch6.go:894` / `keywords_demonstrate.go:215` / `keywords_storm_rider.go:168` / `keywords_stubs_tail.go:240` — each is supposed to route through `MintCopyInstanceID` but a sibling site may construct a `Card{...}` literal with `Provenance: ProvOG` from an earlier code path and never register.

**Not fixed in this PR.** Tractable next step: instrument `RecordMintedInstanceID` to log a stack trace when called, run game 411 in isolation, diff against a known-clean game to find the unbalanced mint site. Estimated 1-2 hours of focused bisect work; not landed here because the bisect requires the event-log forensics surface that isn't part of the Loki binary today.

### Signature #3 — Leonardo, Sewer Samurai ExileLinkageIntegrity orphan

Card "Leonardo, Sewer Samurai // Leonardo, Sewer Samurai" (the art-series DFC entry, both faces named identically and with empty oracle text) sits in seat 0's exile with `ExiledByTimestamp = 68`. No live perm has timestamp 68, so `checkExileLinkageIntegrity`'s legacy backstop fires.

The post-#781 dispatch fix `f374f26b` added the `ctx["perm"]` fallback so `permanent_ltb` triggers reach the leaving perm's own handlers. That closed the Banisher Priest family `DestroyPermanent` / `ExilePermanent` / `BouncePermanent` / `sacrificePermanentImpl` paths. **The residual signal is a leave-play path that bypasses `FireZoneChangeTriggers` entirely — `HandleSeatElimination` (CR §800.4a)** — which had its `ExpireSourceGrants` cleanup wired (PR #106) but never released LinkedExile bookkeeping.

**Fix shipped here:** new `ReleaseSourceLinkedExiles(gs, perm)` primitive in `internal/gameengine/state.go` clears each card's `ExiledByTimestamp` plus the source's `LinkedExile` / `ExiledByMe` slices, emitting an `exile_linked_released_on_leave` event for forensics. Called from `HandleSeatElimination` for every detached perm right after `ExpireSourceGrants`. The exiled cards stay in their owners' exile zones (NOT routed back to battlefield per §406.7) because routing `MoveCard` into a battlefield mid-sweep would race with the in-progress seat removal — that's tracked as an engine-correctness TODO in the helper's docstring.

**Result:** the seat-elimination scenario is now invariant-clean (3 regressions in `internal/gameengine/seat_elimination_linked_exile_r60_test.go` pin the behavior), but the live Loki signature on game 2029 is unchanged at 26 hits — the source perm (timestamp 68) is leaving via a path OTHER than seat elimination. The most likely candidates per the standing CLAUDE.md issue-log row (2026-05-25 `removePermanent` API-misuse sweep) are the 4 unaddressed sites: `etrata_the_silencer.go` (×2), `gen_bilbo_birthday_celebrant.go` (×1), `thassa_deep_dwelling.go` (×1) — all use `removePermanent` directly, bypassing the canonical battlefield-exit pipeline that would otherwise fire `permanent_ltb`. Also worth auditing: `keywords_batch6.go:293/323` (mutate-eat), `keywords_batch6.go:2031/2032` (meld), and `keywords_misc.go:2847` (one-off bounce primitive).

**Deeper investigation needed:** the residual 30 ExileLinkageIntegrity hits split across 3 sources (Leonardo 26 / Myr Prototype 2 / Great Hall of the Biblioplex 2). Without per-game event-log forensics it's not possible to tell which of the candidate bypass paths fired in game 2029 / game 1044 / game 149 respectively. Recommend: add `--trace-game-id` to Loki that dumps the full event log for one specific game number; replay game 2029; find the LTB-equivalent event preceding the first orphan flag. Estimated 2-4 hours total for the trace tooling + bisect.

## What shipped in this PR

1. **`ReleaseSourceLinkedExiles` helper + HandleSeatElimination wire** — `internal/gameengine/state.go` (+50 LOC) and `internal/gameengine/multiplayer.go` (+12 LOC). Closes the §800.4a seat-elimination subset of the ExileLinkageIntegrity cluster. 3 regression tests in `seat_elimination_linked_exile_r60_test.go`:
   - `TestSeatElimination_ReleasesLinkedExileOnLeavingSource` — bit-stable Leonardo-style scenario, post-elim assertions pin the linkage clear-down.
   - `TestSeatElimination_PreservesLinkedExileOnSurvivingSources` — scope guard, survivor-side Banisher Priest stays untouched.
   - `TestSeatElimination_NoOpWhenLeavingPermHasNoLinkedExile` — defensive no-op for the common case (most leaving perms have no exile linkage).

2. **Full `gameengine` test suite stays green** (1.97 s, all packages pass).

## What didn't ship — needs deeper investigation

| Cluster | Hits | Why deferred |
|---------|-----:|--------------|
| Fabrication `h1OGVR200096` (game 411) | 46 | Mint-bypass bisect needs event-log forensics tooling that doesn't exist yet; speculative without it. |
| Fabrication `h1OGVR200056` (game 2762) | 34 | Same shape as above; bisect once for both. |
| Leonardo Sewer Samurai live residual (game 2029) | 26 | LTB-bypass path other than seat-elimination; needs per-game trace to identify which of 4+ candidate `removePermanent` sites fires. |
| Other 4 fabrication signatures | 7+11+11+11 | Same speculation surface as #1/#2. |
| Token disappearance cluster | ~50 (token-heavy) | Distributed across many 1-2 hit signatures; needs per-token mint-vs-cease audit. Phase E sweep closed bulk; residual is long-tail. |
| CardIdentity Spikeshell Harrier | 4 | Single-game, dup-on-same-zone; per-card handler race (likely a known anti-pattern from the etrata/bilbo/thassa family). |

## Trajectory

| Run | Date | Violations | Δ | Notes |
|-----|------|----------:|--:|-------|
| r60 round 3 | 2026-05-26 | 0 | — | First fully-clean 5k seed-42 run; pre-strict-census |
| r60 fresh (PR #781) | 2026-05-30 | 268 | +268 | Strict-census flipped on; ExileLinkageIntegrity invariant added |
| r60 post-`f374f26b` (rerun this branch baseline) | 2026-05-30 | 224 | −44 (−16%) | Registry LTB-dispatch fix (Banisher Priest family closed for canonical leave-play paths) |
| **r60 post-`ReleaseSourceLinkedExiles` (this PR)** | 2026-05-30 | **224** | **0 live impact** | Seat-elim subset closed (test-pinned); live signature is a different leave-play bypass path |

Net: invariant-suite **correctness** improved (3 new pinned regressions for §800.4a + §406.7 interaction); invariant-suite **violation count** unchanged because the seat-elimination subset wasn't being exercised by seed 42's 5k game corpus. The fix is preventative for cards-shapes that haven't yet appeared in chaos play but absolutely will once test-game variance grows.

## How to reproduce

```bash
git fetch origin main && git checkout -B repro origin/main
git cherry-pick <this-PR-commit>
go run ./cmd/hexdek-loki --games 5000 --seed 42 --nightmare-boards 0
# Expected: 224 violations / 0 crashes — same as pre-cherry-pick.
go test ./internal/gameengine/ -run TestSeatElimination_Releases -count=1 -v
# Expected: 3 new tests pass.
```
