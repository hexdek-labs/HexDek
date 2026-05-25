# Composition-Strength TrueSkill Prior — Design Proposal

**Date:** 2026-05-25
**Branch:** `dev/cross-composition-elo-prior-r60`
**Status:** **DESIGN ONLY — no implementation in this PR**
**Source:** PR #322 (`docs/seat-bias-meta-study-r60.md`) — proved
that composition shifts winrate by ~10× more than seat position
does (LandsMatter 23pp swing across compositions vs 2.4pp across
seats), making a composition-aware prior the next natural step.

## Problem statement

TrueSkill (and ELO) updates assume that the expected outcome of a
match can be derived from each player's intrinsic skill. In Commander
self-play, the meta-study proved this assumption is broken — the
4-archetype POD shifts each deck's winrate by 10–25 percentage points
before player skill even enters. Concrete example:

- Lord Windgrace wins **27%** in C1 (mill/voltron/spellslinger/lands)
- Aesi wins **~50%+** in C5 (lands/blink/reanimator/stax)

Both are LandsMatter decks of comparable absolute power, but the
"intrinsic skill" inferred from raw winrate will be miscalibrated by
20+ rating points if the prior doesn't condition on the pod.

A composition-conditioned prior asks: **given the 4-archetype pod
present at the table, what's the expected baseline winrate for each
deck?** TrueSkill then updates skill estimates against the residual
(observed − expected), which IS the player-skill signal.

## What we already have

The tournament runner already populates two structures that are
close-to-but-not-quite what's needed:

```go
// internal/tournament/results.go
MatchupMatrix map[string]map[string]int  // [A][B] = wins by A when both played
MatchupGames  map[string]map[string]int  // [A][B] = games where both A and B played
```

Pairwise winrate `MatchupMatrix[A][B] / MatchupGames[A][B]` gives
A's win share specifically against B. It's the foundation but doesn't
capture 3-way interactions (e.g. Mill+Aristocrats racing each other
suppresses LandsMatter's winrate via a mechanism neither pairwise
matchup captures).

The meta-study also produced per-(commander, seat, composition)
winrate data that's currently only in `/tmp/seat-bias-meta/*.log` —
needs persistence to inform a prior.

## Design options

### Option 1 — Pairwise matchup table

**Structure:** SQLite table `archetype_matchup_prior(archetype_a TEXT,
archetype_b TEXT, wins INTEGER, games INTEGER)`. Keyed
case-insensitive on `(archetype_a, archetype_b)` with both directions
stored (so `(Mill, Voltron)` and `(Voltron, Mill)` are separate rows).

**Prior calculation:** For each deck X in pod {X, Y, Z, W}:

```
expected_X = mean(matchup_winrate(X.arch, Y.arch),
                  matchup_winrate(X.arch, Z.arch),
                  matchup_winrate(X.arch, W.arch))
```

**Pros:**
- Tractable: 22 × 22 = 484 cells (only ~250 unique after symmetry)
- Reuses existing `MatchupMatrix` data — bootstrappable today
- Generalizes to any pod composition we haven't tested

**Cons:**
- Misses 3-way and 4-way interactions (the LandsMatter swing IS one)
- Assumes pairwise winrate decomposes additively — meta-study data
  shows it doesn't

**Storage:** ~250 rows × ~32 bytes ≈ 8KB. Negligible.

### Option 2 — Full 4-archetype composition lookup

**Structure:** Hash table keyed by sorted 4-archetype tuple
`(arch_a, arch_b, arch_c, arch_d)` → map[archetype → expected_winrate].

**Prior calculation:** Direct lookup; fallback to Option 1 when
composition unseen.

**Pros:**
- Captures exactly what the meta-study showed: full-pod effects
- No assumption of pairwise additivity

**Cons:**
- C(22, 4) = 7,315 possible 4-archetype combinations
- To get 1500 games per cell (matching the meta-study density),
  that's ~11 million games — completely intractable
- The vast majority of cells will be empty, requiring fallback to
  Option 1 anyway
- High-traffic compositions get oversampled; tail compositions never
  measured

**Storage:** Sparse — actual size depends on how many compositions
get played. ~10–100K rows in practice.

### Option 3 — Tiered fallback (recommended)

**Structure:** Both tables from Options 1 & 2 coexist. Each prior
query tries them in order:

```
expected_X = lookup_in_full_composition_table(pod, X.arch)
          if not present:
              fall back to mean of pairwise matchups
          if pair table also sparse:
              fall back to archetype baseline (mean self-play winrate)
```

**Pros:**
- Best of both worlds: full-pod accuracy where data exists,
  pairwise approximation elsewhere
- Self-correcting: full-pod table gets filled organically by live
  games, gracefully replacing the pairwise approximation
- Bootstrap-able from meta-study + showmatch data

**Cons:**
- More complex to implement (3 query paths, fallback selection
  logic, two SQLite tables to maintain)
- Confidence weighting needs care (a 50g full-pod number shouldn't
  override a 5000g pairwise approximation)

**Storage:** Pairwise (~8KB) + sparse full-pod (~10–100KB) = ~100KB.

## How TrueSkill consumes the prior

Standard TrueSkill update for player X in game outcome:

```
prior_mean_X    = μ_X   // current rating mean
prior_variance  = σ²_X
update_X = bayes_update(prior, observed_rank_X)
```

With composition prior, instead of using μ_X directly as the
predicted-skill input, use:

```
adjusted_μ_X = μ_X + composition_offset(X.arch, pod)
```

Where `composition_offset` is the deviation of this composition's
expected winrate from X's mean self-play winrate. The update equation
is unchanged — only the predicted-skill input shifts.

**Critical**: when adjusted_μ_X is what feeds the rank-prediction,
the residual (observed − predicted) cleanly separates "player skill"
from "deck-in-composition baseline." Without this, TrueSkill
incorrectly attributes the composition's tilt to skill.

## Data population strategy

### Bootstrap (today)

1. Extract meta-study per-(commander, composition) winrate from PR
   #322's logs into a structured CSV.
2. Map each commander → archetype via `internal/hat/poker.go`'s
   `ArchetypeXxx` constants.
3. Populate the full-pod table for the 5 meta-study compositions
   × 5 seeds = 25 (pod, deck) cells × 4 decks per cell = 100
   composition-prior entries.
4. Populate the pairwise table from existing showmatch
   `MatchupMatrix` data (already aggregated by the tournament
   runner).

### Live data accrual

- Every finished showmatch game (post-PR #321 persistence) updates
  both tables: increment full-pod cell + pairwise matchup cells.
- Periodic recomputation (e.g., nightly cron) refreshes prior values
  from the underlying counts.
- After ~100K live games, the full-pod table should cover the
  ~5–10% of compositions that actually get played (long-tail
  compositions still fall through to pairwise).

### Active sampling

If the expanded study (PR #377 scope) executes, its 100K games
across 23 compositions become high-density seed data for the
full-pod table. Each composition in that study would contribute
~1500g per (pod, deck) cell — sufficient for the prior to be
production-trustworthy.

## SQLite schema sketch

```sql
CREATE TABLE archetype_matchup_prior (
    archetype_a TEXT NOT NULL,
    archetype_b TEXT NOT NULL,
    wins        INTEGER NOT NULL DEFAULT 0,
    games       INTEGER NOT NULL DEFAULT 0,
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (archetype_a, archetype_b)
);
CREATE INDEX idx_matchup_prior_a ON archetype_matchup_prior(archetype_a);

CREATE TABLE composition_pod_prior (
    -- sorted-tuple of archetypes joined with "|" so PRIMARY KEY is stable
    pod_key      TEXT NOT NULL,
    -- archetype this row's winrate is for
    archetype    TEXT NOT NULL,
    wins         INTEGER NOT NULL DEFAULT 0,
    games        INTEGER NOT NULL DEFAULT 0,
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (pod_key, archetype)
);
CREATE INDEX idx_pod_prior_key ON composition_pod_prior(pod_key);
```

The `pod_key` is canonicalized as `sort(archetypes).join("|")` so
lookups are O(1) regardless of seat order.

## API sketch

```go
// internal/trueskill/composition_prior.go

type CompositionPrior interface {
    // ExpectedWinrate returns the deck's expected winrate (0..1) in
    // a pod with the given archetypes. Falls back through the
    // tiered hierarchy: full-pod → pairwise → archetype baseline.
    ExpectedWinrate(deckArchetype string, pod []string) float64

    // ObserveGame updates the prior with a finished game's outcome.
    // Called from the tournament/showmatch runners after every game.
    ObserveGame(pod []string, winnerArchetype string) error

    // Confidence returns the priors's per-cell confidence (0..1)
    // based on how many games the relevant cell has. Used by
    // TrueSkill to weight the prior against its own σ estimate.
    Confidence(deckArchetype string, pod []string) float64
}
```

## Integration with existing TrueSkill code

`internal/trueskill/rating.go` currently does standard
skill-based updates. The hook would be in the `Update` function:

```go
// Before computing expected outcome from raw μ values:
for i, player := range players {
    adjustment := prior.ExpectedWinrate(player.Archetype, podArchetypes) - 0.25
    adjusted_mu[i] = player.Mu + adjustment * compositionWeight
}
// Then run the standard TrueSkill update with adjusted_mu.
```

The `compositionWeight` knob (0..1) controls how aggressively the
prior shapes the update — start at 0.5 (split the difference between
raw-skill and pod-conditioned), tune from data.

## Risks + open questions

1. **The prior could absorb player skill.** If a strong player consistently
   wins with Mill, the full-pod table's "Mill in {Mill, Voltron, ...}"
   cell drifts up. The prior interprets this as "Mill is strong in
   that composition" rather than "this player is strong." Mitigation:
   require per-player ELO updates to use the prior at game-START
   only, never re-fit the prior from player-specific game outcomes.
2. **Cold start.** A new (commander, composition) cell has 0 games. The
   pairwise fallback uses ~250 cells of meta-study + showmatch data,
   so cold start always has SOMETHING to fall back on, but the
   archetype-baseline tier is the only universal floor.
3. **Drift detection.** As the meta changes (new cards, new decks),
   old composition priors go stale. Need either a decay weighting
   (older games count less) or a manual refresh cadence.
4. **Cycle: the prior depends on TrueSkill, TrueSkill depends on the
   prior.** If both are updated simultaneously from the same games,
   they can diverge. Recommend updating prior counts first, then
   TrueSkill with the snapshot.
5. **Hat is the same across all seats in our data.** The prior
   conditions on archetype, not hat. Live games with mixed hats
   (human + bot + bot, etc.) would need a separate dimension or a
   re-fit on live data.
6. **22-archetype space is small but sparsely populated.** Empty
   pairwise cells happen for rare matchups (e.g., GroupHug vs
   Selfmill — neither showmatch nor meta-study has played that
   combo). The fallback chain handles this; just noting that the
   prior's accuracy is proportional to data coverage.

## What this PR does NOT do

- No code changes.
- No SQLite migrations.
- No data extraction from meta-study logs.
- No TrueSkill modification.

## Green-light criteria for execution

Before implementing, please confirm:

1. **Option choice.** Option 1 (pair-only, ~250 rows, fast to ship)
   vs Option 3 (tiered fallback, recommended). Option 2 alone is
   not viable.
2. **Bootstrap source.** Extract from meta-study logs only, or
   wait for the expanded study (PR #377) to fill the table densely
   first?
3. **compositionWeight default.** 0.5 (recommended starting point)
   or a different value?
4. **Cadence.** Run prior recomputation nightly via cron, on-write
   per-game, or only on explicit recompute commands?

Once those four are decided, implementation can branch off this PR
in the order: schema migration → bootstrap data load → API + tests →
TrueSkill hook → live aggregator.
