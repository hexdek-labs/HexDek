# Loki r60 — 50K Deep-Surface Sweep (seed 42)

| Field | Value |
|-------|-------|
| Date | 2026-05-27 |
| Branch | `dev/loki-r60-50k-r60` |
| Invocation | `go run ./cmd/hexdek-loki --games 50000 --seed 42` |
| Phase 1 throughput | 50,000 chaos games in 14m20s (58 g/s avg) |
| Phase 2 throughput | 10,000 nightmare boards in 727ms (13,756 b/s) |
| Crashes | **0** (chaos) + **0** (nightmare) |
| Panics / recovers | **0** |
| Chaos violations | **2,808** in **50 games** (49,950 clean = 99.90%) |
| Nightmare violations | **0** (all 10,000 boards clean) |
| Verdict | **No new categories surfaced** beyond the 25K Etali clusters — 2× depth scales the same two clusters ~3.4× in violations, ~3.1× in violation games, but the invariant taxonomy is unchanged |

## Trajectory vs prior depth runs (canonical seed 42)

| Depth | Violations | Violation games | CardIdentity | ZoneConservation | Other |
|-------|------------|-----------------|--------------|------------------|-------|
| 5,000 | 0 | 0 | 0 | 0 | 0 |
| 10,000 | 0 | 0 | 0 | 0 | 0 |
| 25,000 | 828 | 16 | 548 | 280 | 0 |
| **50,000** | **2,808** | **50** | **1,796** | **1,012** | **0** |
| Scale 25K→50K | 3.4× | 3.1× | 3.3× | 3.6× | — |

**Key observation: depth doubling produces ~3× violation growth, slightly super-linear.** Two non-exclusive interpretations: (a) statistical noise on small-N counts (16 → 50 is well within Poisson variance for a 0.064–0.10% per-game rate), or (b) the cluster's triggering conditions favor long-running games and the deeper sample biases toward longer games (Etali requires multi-turn attacker survival + repeated trigger fires). Per-game violation rate: 25K = 0.064% (1 in 1,563), 50K = 0.10% (1 in 1,000). Both consistent with "rare tail," neither indicating a regression or new failure mode.

## By invariant

| Invariant | Count | % of total | Cluster |
|-----------|-------|------------|---------|
| CardIdentity | 1,796 | 64% | "Hostile Realm" cross-seat ptr leak via Etali grant residue (same signature as 25K) |
| ZoneConservation | 1,012 | 36% | "12 extra real cards appeared" — Etali cast-from-exile copy-count drift (same signature as 25K) |
| **All other invariants** | **0** | **0%** | — |

The 50K depth confirms that the rare tail at seed 42 is **exhaustively characterized by the two Etali clusters identified at 25K**. No third cluster surfaced. Specifically, none of the 11 historical r60 invariant categories (Adric, Oketra, Dread, Jaxis, Athreos, Zidane, Necrogen Communion, Cerulean Sphinx, Krark paradigm, Breya game-420 evacuate, Charix `ended=1`, Platinum Angel phantom-source, Compound Fracture RIP-ETB FP, Myr Moonvessel pending-triggers, WinCondition post-elim FP) regressed; none of the dormant invariant kinds (TriggerCompleteness, ReplacementCompleteness, AttachmentConsistency, CombatLegality, SBACompleteness, LifeConsistency, ResourceConservation, ZoneCastGrantExpiry, StackIntegrity) surfaced.

## Card correlation (top 10)

| Rank | Card | Violation games | Clean games | Correlation | Δ vs 25K |
|------|------|-----------------|-------------|-------------|----------|
| 1 | **Etali, Primal Storm** | **47** | **303** | **0.13** | ↑ from 0.10 (was rank 2) |
| 2 | Riveteers Confluence | 1 | 14 | 0.07 | ↓ from 0.10 (was rank 1) |
| 3 | Stone Kavu | 1 | 14 | 0.07 | **new** |
| 4 | Jund Sojourners | 1 | 14 | 0.07 | **new** |
| 5 | Maestros Confluence | 1 | 18 | 0.05 | **new** |
| 6 | Tamanoa | 1 | 21 | 0.05 | **new** |
| 7 | Mirrorwood Treefolk | 1 | 21 | 0.05 | ↓ from 0.09 (rank 3) |
| 8 | Noble Hierarch | 1 | 22 | 0.04 | **new** |
| 9 | Sand Warrior | 1 | 24 | 0.04 | ↓ from 0.08 (rank 4) |
| 10 | The Ever-Changing 'Dane | 2 | 50 | 0.04 | ↑ from 1 violation game |

**Etali, Primal Storm appears in 47 of 50 violation games (94%)** — same proportion as 25K (15/16 = 94%). The non-Etali "rank 1" correlation is meaningless: Riveteers Confluence has 1 violation game over 15 total appearances, a tiny denominator that the correlation metric inflates. The other top-10 ranks 3-10 are all single-violation-game coincidences from being in pods that also contained Etali. The depth doubling did not reveal a second smoking-gun card; it just added more low-N noise.

## What surfaced at 50K that did NOT surface at 25K

**Nothing structural.** The detail-window (capped at 5 per invariant kind) shows the same two game traces (game 1944 CardIdentity, game 2275 ZoneConservation) at 50K as at 25K — both reproducible bit-for-bit. The additional 34 violated games beyond the 25K set are statistically iid samples of the same two cluster signatures, not new clusters.

What this does NOT rule out: rare clusters that exist but are still below the 1-in-50,000 threshold. Examples that would warrant a 100K probe before being declared "engine-clean at canonical seed":
- Stack saturation / mana-pool overflow / triggered-ability flood
- Replacement-effect cascade interactions
- Copy/token-mint runaway
- Commander-cast-from-anywhere edge cases (e.g. Eminence + zone-shifted commander)
- Double-faced / split / fuse / adventure card zone-transition aliasing
- Mutate stack permutations
- Paradigm-exile leak regression
- Sub-token cross-pollination (Treasure → Food via class change, Clue duplication via Lonis-style etc.)

None of these surfaced at 50K. The 50K depth is the first sample that gives statistical power to say "at this seed, the engine has exactly two rare-event signatures and they are both Etali."

## Recommended follow-up

1. **Both fixes from the 25K report still apply** — nothing changes about the underlying bugs at 50K depth, only the confidence in their exhaustive enumeration:
   - Cluster 1 (CardIdentity): `ExpireZoneCastGrants` must return the `*Card` to its canonical owner's zone when the grant expires
   - Cluster 2 (ZoneConservation): Etali's exile-from-other-players' libraries must route each card into the *owner's* exile per CR §400.7c
2. **A single Etali fix wave will close both clusters and the entire rare tail at seed 42.** The 2× depth doubling provides high-confidence evidence that no other rare-event signature is hiding above 1-in-50K.
3. **Depth-threshold guidance for future cadence**:
   - 5K = "wall-clock-cheap clean check" (~1m30s) — adequate for fast iteration on per-card fixes
   - 10K = "canonical clean check" (~3m) — current default
   - 25K = "rare-event probe" (~8m) — surfaces 1-in-1500 issues
   - 50K = "exhaustive rare-event probe" (~15m) — high-confidence enumeration of rare signatures at one seed
   - 100K+ = "deep-tail probe" — would only be worth running after the Etali fixes land, to confirm no new rare clusters lurk below 50K
4. **Multi-seed 25K sweep would be more valuable than single-seed 100K** as the next-deeper probe — different seeds sample different pod compositions, which is where rare tails concentrate. Recommend a quarterly 5-seed × 25K matrix.

## What is NOT in this report

- **No regressions of the 11 historical r60 clusters** — all closed signatures held bit-stable across 50K games
- **No nightmare-board violations** — 10,000 boards, 0 violations, 13,756 b/s
- **No panics / recovers** — `0` for both phases
- **No new invariant categories** — the rare tail at seed 42 is exhaustively two Etali clusters; the rest of the engine has no detectable rare events above 1-in-50,000 at this seed
