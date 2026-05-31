# Loki r60 Fuzz Report — 5000 games / seed 42 (Fresh main)

**Date:** 2026-05-30
**Branch:** `dev/loki-r60-fuzz-fresh` (cut from `origin/main` @ `b3ae63bd`)
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42`
**Worktree:** `.claude/worktrees/r60-11-feyd-slot`
**Raw log:** `/tmp/loki-r60/run.log`
**Auto-generated report:** `/tmp/loki-r60/CHAOS_REPORT.md`
**Violation dump:** `/tmp/loki-r60/violations.tsv` (268 entries, one per line, tab-separated `game<TAB>turn<TAB>invariant<TAB>message`)

## Headline

**0 crashes / 268 violations across 45 games (out of 5000) — REGRESSION vs the 2026-05-26 0/0 baseline.**

The clean-on-seed-42 r60-round-3 status documented in this file's prior revision and in `docs/loki-r60-canonical-final.md` (2026-05-25, "engine officially clean") has been broken by the InstanceID Phase 5-9 / A-E rollout that shipped 2026-05-27..2026-05-30 (commits `e3cd053a`, `b76b5357`, `48a56c97`, `1643f9e4`, `b8ffdefb`, `5ead6140`, `117a95c8`, `ae55f16e`). The new symptoms are NOT pre-existing engine bugs — they are bugs newly **observable** because two previously off-by-default census-style invariants are now on by default:

1. **`ZoneConservation` strict-census disappearance check** — flipped from gated → default-on by `5ead6140` (`SetStrictCensusDefault(true)` in `internal/gameengine/state.go:638`). The 25k Phase C verification reported clean, but the 5k seed-42 chaos surface still has residual mint-coverage gaps.
2. **`ExileLinkageIntegrity` invariant** — registered in `invariants.go:87`, introduced in the InstanceID Phase 3 / Phase 4 work. Did not exist as a tracked invariant in r41-r60-round3 baselines.

Nightmare phase is still bit-stable clean: 0 violations / 0 crashes across 10000 boards.

## Run Output

```
=== CHAOS GAMES COMPLETE ===
  games:           5000
  duration:        2m12.424s
  throughput:      38 games/sec
  crashes:         0 (in 0 games)
  violations:      268 (in 45 games)
  clean games:     4955

=== NIGHTMARE BOARDS COMPLETE ===
  boards:          10000
  duration:        779ms
  throughput:      12834 boards/sec
  crashes:         0
  violations:      0
  clean boards:    10000

Verdict: 268 invariant violations and 0 crashes across 45 problematic games (out of 5000 total).
```

## Top Invariant Clusters (by frequency)

| Invariant | Hits | Games | Notes |
|-----------|-----:|------:|-------|
| `ZoneConservation` (fabrication arm) | 120 | 5 | InstanceID present in a zone but not in `(Minted − Ceased)` — fabricated ID or stale ceased entry. 6 distinct fabricated IDs, all `OG` provenance (oracle-card mint). Dominated by 2 IDs across 2 games. |
| `ZoneConservation` (disappearance arm) | 72 | 37 | InstanceID minted and not ceased but absent from every zone — strict-census "card disappeared". Token-heavy: food / treasure / soldier / zombie / mite / clue / phyrexian-mite. |
| `ExileLinkageIntegrity` | 72 | 4 | Card in exile linked to a source timestamp no longer on any battlefield → LTB-return missed (orphaned linked exile). 4 distinct sources; 2 dominate. |
| `CardIdentity` | 4 | 1 | Spikeshell Harrier present twice on the same battlefield, single game. |
| `TriggerCompleteness`, `SBACompleteness`, `LifeConsistency`, `AttachmentConsistency`, `CombatLegality`, `ResourceConservation`, `ReplacementCompleteness`, `StackIntegrity`, `ZoneCastGrantExpiry`, `WinCondition`, `LayerOrdering` | 0 | — | Clean. |

### Top Offending Cards / IDs

**Fabrication arm — top fabricated InstanceIDs**:

| InstanceID | Hits | Provenance | Game |
|------------|-----:|-----------|-----:|
| `h1OGVR200096` | 46 | seat-1 OG / role R2 | 411 |
| `h1OGVR200056` | 34 | seat-1 OG / role R2 | 2762 |
| `h2OGVU100098` | 13 | seat-2 OG / role U1 | 3589 |
| `h0OGVU100070` | 10 | seat-0 OG / role U1 | 3688 |
| `h2OGVC000097` | 9 | seat-2 OG / role C0 | 3589 |
| `h0OGVC000094` | 8 | seat-0 OG / role C0 | 3688 |

Pattern: the same fabricated ID repeats every invariant tick in a single game (game 411 alone contributes 46 of 120 fabrication hits). Suggests a single mint-bypass site per game replaying through the priority loop — a hand-rolled `*Card` or a mint helper missing from one of the InstanceID-v2 phase wires.

**Disappearance arm — top disappeared card kinds (per-name aggregation)**:

| Card | Hits |
|------|-----:|
| food artifact token | 7 |
| treasure artifact token | 6 |
| creature token colorless soldier artifact | 5 |
| creature token zombie | 4 |
| creature token phyrexian mite | 4 |
| Tolarian Sentinel | 2 |
| token treasure | 2 |
| token clue | 2 |
| Saprazzan Legate | 2 |
| Rushwood Grove | 2 |
| Queen's Bay Soldier | 2 |
| Mountain | 2 |
| Harbin, Vanguard Aviator | 2 |

Tokens dominate the disappearance arm — exactly the surface called out in `internal/gameengine/state.go:631-635` as the gap-walk target. Some genuine non-token cards leak too (Saprazzan Legate, Rushwood Grove, Harbin Vanguard Aviator, Tolarian Sentinel), so this isn't strictly a token-bookkeeping problem.

**`ExileLinkageIntegrity` — top orphaned-linked-exile sources**:

| Card | Hits | Game |
|------|-----:|-----:|
| Prison Barricade | 42 | 2164 |
| Leonardo, Sewer Samurai // Leonardo, Sewer Samurai | 26 | 2029 |
| Myr Prototype | 2 | 1044 |
| Great Hall of the Biblioplex | 2 | 149 |

Pattern: a permanent with a linked-exile effect (Prison Barricade exiles a creature until it leaves; Leonardo similar) leaves the battlefield without firing its LTBReturn → exiled card is now orphaned, the source's `ExiledByMe` set still claims the linkage. Re-runs every invariant tick on the same game (game 2164 = 42 hits) so total volume amplifies one missed LTB-cleanup site.

## Trajectory vs. CLAUDE.md Baselines (seed 42, 5000 games)

| Milestone | Date | Total | Clusters | Δ vs Prev | Note |
|-----------|------|------:|----------|----------:|------|
| **r41** baseline | 2026-05-19 | 1652 | mostly `CardIdentity` (Cerulean Sphinx zone-leak) | — | Cerulean Sphinx + paradigm-copy era |
| **r44** | 2026-05-19 | 402 | `CardIdentity` (Adric/Oketra), `ZoneConservation` (Krark paradigm) | −1250 / −76% | Cerulean Sphinx closed |
| **r60 round 1** | 2026-05-24 | 52 | `ZoneCastGrantExpiry`, `TriggerCompleteness`, `CardIdentity`, `CombatLegality` | −350 / −87% | Round-1 cleanups landed |
| **r60 round 2** | 2026-05-24 | 10 | `ZoneCastGrantExpiry` (4), `TriggerCompleteness` (2), `CardIdentity` (2), `CombatLegality` (2) | −42 / −81% | PR #106 / #110 / #124 + trigger-cap |
| **r60 round 3** | 2026-05-26 | 0 | — | −10 / −100% | First fully-clean 5k seed-42 run; pre-strict-census |
| **r60 fresh (THIS RUN)** | 2026-05-30 | **268** | `ZoneConservation` strict-census (192) + `ExileLinkageIntegrity` (72) + `CardIdentity` (4) | **+268 NEW signal** | Strict-census flipped on; InstanceID v2 Phases 5-E shipped |

**Δ vs r41 baseline: −1384 / −83.8%.** The absolute regression vs the 2026-05-26 zero is real, but it's mostly visibility expansion rather than re-introduced bugs: the engine's behavior shifted only modestly while the invariant suite's coverage expanded substantially.

## Worst New Regression Cluster

**`ZoneConservation` strict-census** is the worst NEW regression cluster — 192 hits, 39 unique games, did not exist as an observable signal before `5ead6140` (2026-05-29). It splits into two failure modes:

1. **Fabrication arm (120 hits)** — observable since Phase 4 (`invariants.go:288-297`, always-on). The 5 affected games each have a *persistent* fabricated ID (replays every invariant tick), indicating a small number of hand-rolled-`*Card` or mint-bypass sites in the per_card / resolve corpus. High-leverage fix: bisect `h1OGVR200096` in game 411 → find the one mint site that's emitting a `Card` without `MintOGInstanceID` / `MintTKInstanceID`.
2. **Disappearance arm (72 hits)** — newly observable since `5ead6140`. Token-heavy. The Phase D / Phase E private-zone + tail-edge sweeps closed the bulk surface (per `docs/instanceid-phase-c-verification.md`), but residual leakage remains around token destruction / sac-outlet / cleanup paths that don't always call the cease-helper.

**`ExileLinkageIntegrity` (72 hits, 4 games)** is the worst genuinely-new ENGINE regression cluster (not just a new check):
- Prison Barricade and Leonardo, Sewer Samurai exile-on-ETB → return-on-LTB Auras / permanents have a real LTB-cleanup gap. The source's `gs.ExiledByMe[srcTimestamp]` linkage entry is not being torn down when the source leaves the battlefield, so the exiled card stays orphaned.
- Same anti-pattern as the 2026-05-27 closure for r41-era ZoneCastGrantExpiry / Yawgmoth's Agenda (PR #106) — LTB plumbing exists, but not all leave-play paths call it. Likely needs an analogous sweep across `DestroyPermanent` / `ExilePermanent` / `sacrificePermanentImpl` / `BouncePermanent` / `destroyPermSBA` / `sacrificePermSBA` / `HandleSeatElimination` to call a `ReleaseLinkedExiles(gs, perm.Timestamp)` helper (mirror of `ExpireSourceGrants`).

## Recommended Next Fix Targets

In priority order:

1. **Prison Barricade + Leonardo, Sewer Samurai LTB-linkage sweep** (`ExileLinkageIntegrity`, 68/72 hits). High-leverage — 2 cards account for 94% of the cluster. Locate the per_card handlers for both; verify whether they call `ReturnLinkedExile` / `ReleaseLinkedExiles` on every LTB path. If the helper doesn't exist in the LTB pipeline, follow the `ExpireSourceGrants` pattern and add it to all 7 leave-play sites. Retain a regression in `internal/gameengine/exile_linkage_ltb_r60_test.go`.
2. **Persistent-fabricated-ID bisect for game 411 / `h1OGVR200096`** (`ZoneConservation` fabrication, 46 hits — single largest signature in the run). Reproducer: `--games 412 --seed 42 --invariant zone-conservation`. Walk the event log for the fabricated ID's first appearance; the preceding `*Card` construction is the mint-bypass site. Likely a struct-literal `Card{}` or a `copyCard` / `cloneCard` helper that doesn't run the mint helper.
3. **Token disappearance sweep** (`ZoneConservation` strict-census, 72 hits, token-heavy). Audit the 4 token kinds with the highest counts (food, treasure, colorless-soldier-artifact, zombie) for a destroy / sac / cleanup path that doesn't call `CeaseInstanceID`. The Phase E tail-edge sweep (`ae55f16e`) closed several but not all; expect another small batch here.
4. **Spikeshell Harrier dup investigation** (`CardIdentity`, 4 hits, single game). Likely same anti-pattern as the closed Adric / Oketra / Dread / Athreos races — a per_card handler reading a `*Card` that's already moved zones. Low-priority (1 game) but worth a 30-minute look.

## Comparison Notes

The CLAUDE.md issue-log already records the `removePermanent` API-misuse sweep (2026-05-25 row, etrata / bilbo / thassa still standing with 4 call sites) — none of those sites are surfacing here because they don't bypass the InstanceID mint flow. Both new clusters in this report are mint-coverage / linkage gaps, not the legacy `removePermanent` family. Wave-1 LOC cleanup (`cc01309f`) did not regress invariants — its `−74 LOC` touched Platinum Angel + structural surface, not LTB plumbing.

## How to Reproduce

```bash
git fetch origin main && git checkout -B repro origin/main
go run ./cmd/hexdek-loki --games 5000 --seed 42
# Same command focused on a single invariant family:
go run ./cmd/hexdek-loki --games 5000 --seed 42 --invariant exile-linkage-integrity
go run ./cmd/hexdek-loki --games 5000 --seed 42 --invariant zone-conservation
# Single-game reproducers for the dominant signatures:
go run ./cmd/hexdek-loki --games 412  --seed 42 --invariant zone-conservation       # game 411  → h1OGVR200096 fabrication
go run ./cmd/hexdek-loki --games 2165 --seed 42 --invariant exile-linkage-integrity # game 2164 → Prison Barricade orphan
go run ./cmd/hexdek-loki --games 2030 --seed 42 --invariant exile-linkage-integrity # game 2029 → Leonardo, Sewer Samurai orphan
```
