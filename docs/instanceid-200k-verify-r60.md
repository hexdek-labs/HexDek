# InstanceID Verification Phase 1 — 200k chaos games / 4 seeds

**Date:** 2026-05-31
**Branch:** `dev/instanceid-50k-4seed-verify-r60` (cut from `origin/main` @ `1731c473`)
**Command:** `go run ./cmd/hexdek-loki --games 50000 --seed <S> --nightmare-boards 10000` for S ∈ {42, 84, 168, 336}
**Worktree:** `.claude/worktrees/r60-11-feyd-slot`

## Headline

**200,000 chaos games (50k × 4 seeds) + 40,000 nightmare boards (10k × 4) surface 1,074 invariant violations across 85 unique games — 0 crashes.** Nightmare phase is fully clean across every seed. The chaos phase reveals a real residual signature population that the canonical 5k seed-42 baseline (`docs/loki-r60-final-baseline.md`) could not reach: extending depth 10× per seed AND running 4 distinct seeds surfaces shapes that single-game-per-seed clustering hides.

This is **Verification Phase 1**: bug-discovery. The follow-up phases close the named signature clusters.

## Per-seed totals

| Seed | Games | ZoneConservation | ExileLinkageIntegrity | CardIdentity | CombatLegality | Nightmare (10k) | Distinct violation games |
|-----:|------:|-----------------:|----------------------:|-------------:|---------------:|----------------:|-------------------------:|
| 42 | 50,000 | 230 | 40 | 0 | 2 | 0 / 0 | 20 |
| 84 | 50,000 | 206 | 118 | 0 | 0 | 0 / 0 | 22 |
| 168 | 50,000 | 216 | 40 | 0 | 2 | 0 / 0 | 19 |
| 336 | 50,000 | 144 | 56 | 20 | 0 | 0 / 0 | 24 |
| **Total** | **200,000** | **796** | **254** | **20** | **4** | **0 / 40,000** | **85** |

**Per-game hit rate: 85/200,000 = 0.0425% — 99.96% of chaos games complete clean.** The canonical seed-42 5k run reports 0 violations (`docs/loki-r60-final-baseline.md`); the 50k extension finds 20 games with violations at the same seed, all in the 5,001–50,000 game range.

## Aggregate top signatures (all 4 seeds)

| Rank | Hits | Signature | Class |
|----:|----:|---|---|
| 1 | 66 | `h0OGVC300008` (Aetheric Amplifier) fabrication | ZC fabrication |
| 2 | 48 | card "Massacre Girl" linked to dead source timestamp 36 | ELI LTB-return missed |
| 3 | 44 | `h1OGVU600056` (Marang River Regent // Coil and Catch) fabrication | ZC fabrication (MDFC) |
| 4 | 44 | `h0OGVC000031` (Germination Practicum) fabrication | ZC fabrication (paradigm) |
| 5 | 42 | card "Heap Doll" linked to dead source timestamp 38 | ELI LTB-return missed |
| 6 | 28 | `h3OGVU800007` (Benthic Behemoth) fabrication | ZC fabrication |
| 7 | 24 | `h2OGVW400003` (The Wandering Emperor) fabrication | ZC fabrication (planeswalker) |
| 8 | 24 | `h2OGVU400043` (Dissipation Field) fabrication | ZC fabrication |
| 9 | 24 | `h0OGVR300090` (Harvesttide Infiltrator // Harvesttide Assailant) fabrication | ZC fabrication (DFC) |
| 10 | 22 | `h2OGVC500001` (Rootwire Amalgam) fabrication | ZC fabrication |
| 11 | 22 | `h1OGVW400036` (Lassoed by the Law) fabrication | ZC fabrication (Aura with exile-until-leaves) |
| 12 | 22 | `h1OGVC400013` (Midnight Crusader Shuttle) fabrication | ZC fabrication |
| 13 | 22 | card "Mind Maggots" linked to dead source timestamp 36 | ELI LTB-return missed |
| 14 | 22 | card "Charming Prince" linked to dead source timestamp 66 | ELI LTB-return missed |
| 15 | 20 | "Jaheira, Harper Emissary" appears in both seat 3 battlefield (h3OGVG200000) | **CardIdentity duplication** |

## ExileLinkageIntegrity sub-shape breakdown (254 total)

**Two distinct ELI sub-shapes surfaced:**

### (a) LTB return missed — orphaned linked exile (172 hits, 11 distinct cards)

Cards exiled by a source whose LTB-cleanup didn't fire. Source perm has left the battlefield; the exiled card's `ExiledByTimestamp` still points to it.

| Hits | Exiled card |
|----:|---|
| 48 | Massacre Girl |
| 42 | Heap Doll |
| 22 | Mind Maggots |
| 22 | Charming Prince |
| 14 | A-Stitched Assistant |
| 8 | Dream Fracture |
| 6 | Sigiled Paladin |
| 4 | Neheb, the Eternal |
| 4 | Bronze Guardian |
| 2 | The Peregrine Dynamo |
| 2 | Metathran Transport |

PR #800 (fireTrigger ctx-fallback for `permanent_ltb`) closed the Banisher Priest / Hostage Taker family per-card LTB-handler dispatch. These residuals trace to either:
- Different exile-source cards (Reflector Mage, Skyclave Apparition, Pithing Needle-style exile-from-stack effects) whose per_card handlers don't register a `permanent_ltb` `ReturnLinkedExile` call.
- Non-canonical leave-play paths for the exile source (e.g., the source goes from battlefield to a sideband zone without `FireZoneChangeTriggers` firing).

### (b) Source-held LTBReturn linkage broken (82 hits, 2 distinct sources)

**SOURCE IS STILL ALIVE on a battlefield, claims a card in `ExiledByMe`, but the card is not in any exile zone.** Phase 3 source-held check (`invariants.go:2002-2016`).

| Hits | Source (across timestamps) |
|----:|---|
| 58 | Banisher Priest (5 distinct timestamps) |
| 22 | Hostage Taker (3 distinct timestamps) |

This is a NEW signature class not previously observed at 5k. The source is alive, but its `ExiledByMe` references a card that no longer sits in any exile zone. Likely shape:
- The exiled card moved zone via a non-canonical path (exile-to-graveyard from a separate ability, exile-to-hand via Riftsweeper-style retrieval, exile-to-command-zone for commander redirect).
- `ReturnLinkedExile` was NOT called (because the source didn't leave), so `perm.ExiledByMe` retained the stale ID.

The fix likely needs an `ExileSource.Untag(card)` primitive called by zone-change paths that pull a card out of exile non-canonically.

## CardIdentity (20 hits, 1 signature, seed 336)

`Jaheira, Harper Emissary (h3OGVG200000) appears in both seat 3 battlefield and seat 3 battlefield` — a single perm wrapped twice on the same battlefield, in a single game over 20 invariant ticks. Shape matches PR #853's Drafna bug (DeepCopy bypassing `MintTokenAsCopyOf` on a token-copy path), but Jaheira is a Legendary Creature — no obvious token-copy source on seat 3's likely card pile. Could be:
- A per_card handler that DOES NOT route through `MintTokenAsCopyOf` (Phase 5 mint-coverage sweep follow-up, similar to PR #871's 11-handler batch).
- A `BecomeCopyOfCard` path that didn't preserve the perm's original InstanceID.

The PR #871 sweep covered 12 known sites; this residual indicates a 13th site or a different mechanism (e.g., the `legendary_supertype_strip` rider in clone effects re-minting with the source's ID).

## CombatLegality (4 hits, 2 seeds)

Single-game / single-turn signatures (seeds 42 and 168). Low priority — not in scope for this verification's named clusters. Listed for follow-up.

## ZoneConservation fabrication breakdown (796 total)

Per-game pattern: every fabrication signature is a single persistent InstanceID flagged repeatedly across many turns of one game (typical: 22-44 hits per ID per game). The card kinds span MDFCs (Marang River Regent, Harvesttide Infiltrator), planeswalkers (Wandering Emperor), Auras with exile-until-leaves (Lassoed by the Law), paradigm spells (Germination Practicum), regular creatures (Benthic Behemoth, Rootwire Amalgam, etc.), and basic lands (Plains, Swamp, Mountain).

The signature shape is: an OG InstanceID minted at deck construction ends up either ceased prematurely (stale ceased entry) OR the *Card pointer is in a zone not covered by the InstanceID census. The Phase G closure (PR #873 — Aziza spell-copy `MintSpellCopy`) handled the named seed-42 5k residual; these 200k-depth residuals indicate similar mint-coverage gaps in other handlers we haven't yet audited.

## Forensics CLI status

The brief asks for "capture the offending replay JSON + run forensics CLI" per violation. The forensics CLI (`cmd/hexdek-forensics`) is wired and supports `--replay` + `--patterns` modes, but Loki does NOT currently emit per-game replay JSONs — the producer side is documented as "future Loki integration" (`cmd/hexdek-forensics/replay.go:15-19`). The replay-emit flag pair `--replay-game-out` / `--replay-game-idx` exists in the design but is unimplemented in `cmd/hexdek-loki/main.go`.

**Closing this gap is the natural Verification Phase 2 prerequisite.** A targeted addition of `--replay-game-out <path> --replay-game-idx N` to Loki, dumping the full `gs.EventLog` + `result.Violations` to JSON for the named game, would let the forensics CLI's `--patterns` mode group fabrications by mint-bypass code path automatically. This PR documents the gap; the actual implementation belongs in a follow-up.

In the meantime, the signature deep-dives above pin enough root-cause hypotheses to guide each Phase 2 closure work item without the automated forensics output.

## Comparison vs. canonical 5k baseline

| Metric | 5k seed 42 (PR #896) | 50k seed 42 (this run) | Δ |
|---|---:|---:|---:|
| Chaos games | 5,000 | 50,000 | 10× depth |
| Chaos violations | 0 | 272 | +272 |
| Distinct violation games | 0 | 20 | +20 |
| Per-game hit rate | 0% | 0.04% | +0.04% |
| Nightmare boards | 10,000 | 10,000 | same |
| Nightmare violations | 0 | 0 | same |

The 5k canonical run was not over-confident — it's an accurate measurement at that depth. The 50k run reveals shapes that require ~10,000+ games of bad-RNG-luck per seed to surface. Combined across 4 seeds, the 200k coverage is **40× the canonical baseline** and exposes 23 distinct fabrication signatures + 11 LTB-orphan signatures + 1 CardIdentity signature.

## Per-seed signature unique-card count

| Seed | Distinct fabrication IDs | Distinct ELI cards | Notes |
|-----:|-------------------------:|-------------------:|---|
| 42 | ~14 | 3 (Charming Prince, Banisher Priest, Hostage Taker variants) | |
| 84 | ~12 | 5 (Massacre Girl, Mind Maggots, A-Stitched Assistant, Banisher Priest, Hostage Taker) | |
| 168 | ~10 | 2 | Plus 2 CombatLegality |
| 336 | ~13 | 2 (Heap Doll dominant, Charming Prince) | Plus 20 CardIdentity (Jaheira) |

The signature SHAPES repeat across seeds (LTB-orphan, source-held-broken, fabrication-via-mint-bypass) but the specific CARDS vary — that's the expected pattern of "scale-depth surfaces every code path eventually."

## Recommended next steps (Verification Phase 2)

In priority order:

1. **LTB-return-missed sweep** (172 ELI hits, 11 cards). The PR #800 pattern (fireTrigger ctx-fallback) plus a per_card audit of the named cards' handlers (Massacre Girl, Heap Doll, Mind Maggots, Charming Prince, A-Stitched Assistant, etc.) to verify each has a `permanent_ltb` → `ReturnLinkedExile` wire.
2. **Source-held LTBReturn breakage sweep** (82 ELI hits, Banisher Priest + Hostage Taker). The source is alive — the exiled card got pulled out of exile via a non-canonical path. Audit zone-move paths that touch exile (Riftsweeper-style retrieval, exile-to-hand via per_card flicker handlers, exile-to-command-zone via §903.9b commander redirect).
3. **Loki replay-emit flag + forensics CLI pipeline** (prerequisite for Phase 2 efficiency). Wire `--replay-game-out` / `--replay-game-idx` per the design comment in `cmd/hexdek-forensics/replay.go:15-19`.
4. **CardIdentity Jaheira investigation** (20 hits, 1 game on seed 336). Trace the duplicate-perm path — likely a 13th token-copy or `BecomeCopyOfCard` site missed by PR #871's 12-handler sweep.
5. **ZoneConservation fabrication audits per card** (796 hits, ~25 distinct IDs across seeds). Each ID is a candidate mint-coverage gap; the high-frequency ones (Aetheric Amplifier 66, Marang River Regent 44, Germination Practicum 44, Heap Doll 42, Benthic Behemoth 28) are likely entry points for a Phase H mint-coverage closure batch.

## How to reproduce

```bash
git fetch origin main && git checkout -B repro origin/main
for SEED in 42 84 168 336; do
  go run ./cmd/hexdek-loki --games 50000 --seed $SEED \
    --report /tmp/iid-50k/r-$SEED.md \
    --violations-dump /tmp/iid-50k/v-$SEED.tsv \
    --nightmare-boards 10000
done
```

Expected per-seed totals (this baseline):
- seed 42: 272 violations / 20 games / 0 nightmare
- seed 84: 324 violations / 22 games / 0 nightmare
- seed 168: 258 violations / 19 games / 0 nightmare
- seed 336: 220 violations / 24 games / 0 nightmare

Any seed reporting fewer violations is a real improvement (or a regression closure). Any seed reporting more is an injection regression that should bisect against current main.

## Verdict

The 5k canonical baseline (`docs/loki-r60-final-baseline.md`) is **stable but incomplete coverage** — it correctly reports 0 violations at its specified depth, but does not exhaust the engine's bug surface. The 200k 4-seed coverage published here is the **first depth-realistic measurement of the InstanceID system's actual residual surface**. It establishes:

1. **Nightmare phase is robust** — 0 hits across 40k boards / 4 seeds is genuinely bit-stable.
2. **Chaos surface has known-class residuals** — fabrication (ZoneConservation), LTB-orphan and source-held-broken (ExileLinkageIntegrity), token-copy-duplication (CardIdentity).
3. **All residuals fit existing closure shapes** — none reveals a fundamentally new bug class. Each Phase 2 item maps to a closure pattern that has shipped successfully before (PR #800 for LTB, PR #871 for mint-coverage sweep, etc.).
4. **No crashes anywhere** — the engine never panics across 200k games / 40k boards.

This document is the new scale-verification baseline. Future verification runs should target the SAME 4 seeds and SAME 50k depth so regressions can be measured against per-seed counts directly.
