# TrueSkill Team-Format Support — Scoping

Date: 2026-05-25
Branch: `dev/trueskill-team-r60`
Status: scoping only — no implementation in this PR.

This document scopes what's required to add team-format ratings to
`internal/trueskill`. The current package handles 1v1 (`Update2Player`)
and N-player FFA (`UpdateMultiplayer`); teams are out of scope today and
no caller can express "this rating belongs to a player who sometimes
plays solo and sometimes in a 2v2 pod."

The headline target formats are Two-Headed Giant (2v2), draft-pod team
events, and informal commander alliances that resolve into team wins.

---

## 1. Use cases driving the work

| Use case | Format | Concrete example |
|---|---|---|
| 2-Headed Giant | 2v2, shared life total | Commander Pre-Release sealed pairs |
| Draft pod gauntlet | 8 → two pods of 4 | Friday cube draft; the 8 drafters split into two 4-player FFA pods, pod winners cross-finals |
| Team Unified Constructed | 3v3 (each player on their own board) | EDH team tournaments where 3 players' results are aggregated |
| Informal alliances | FFA with mid-game team-up | The deal-with-the-devil "we both kill Player C, then I concede to you" — already partly handled by FFA `ranks` but not as a *durable* team identity |

Use cases 1–3 are well-defined team math. Use case 4 is a special case
of FFA with negotiated ranks and is **already representable** today —
out of scope for the team-rating work.

---

## 2. Mathematical changes vs. the current package

The TrueSkill paper (Herbrich, Minka, Graepel 2007) gives a team
formula that the current implementation does not use:

**Team-level aggregates** (a team T of n_T members):

```
μ_T   = Σ_i μ_i              (sum, not average)
σ²_T  = Σ_i σ_i²             (sum, not average)
```

**Pairwise team-vs-team update**:

```
c² = (n_A + n_B) · β² + σ²_A + σ²_B
t  = (μ_A − μ_B) / c
P(A beats B) = Φ(t)
```

Note `(n_A + n_B) · β²` — performance variance now scales with the
total player count, not stays at `2β²` as in 1v1. This is the bit
`Update2Player`/`update2PlayerRaw` doesn't currently express; they hard-
code `2 * cfg.Beta * cfg.Beta`. A team-aware path needs `nA + nB`
threaded through.

**Per-member redistribution** of the team-level Δμ:

```
For each member i of team T after a team-level update producing (Δμ_T, σ'_T):
  Δμ_i = (σ_i² / σ²_T) · Δμ_T                  ; variance-share split
  σ'_i² = σ_i² · (1 − (σ_i² / σ²_T) · w_T)     ; per-member precision shrink
```

where `w_T` is the truncated-Gaussian `w` from the team-level update.
This is **net new code** — no current function does proportional
redistribution; `UpdateMultiplayer` iterates pairwise but each pair is
an individual.

**Match quality (team form)**:

```
q = √( (n_A + n_B) · β² / c² ) · exp( −(μ_A − μ_B)² / (2c²) )
```

The pairwise `MatchQuality` in `pairwise.go` is the 1v1 specialization
of this (`n_A = n_B = 1`).

---

## 3. Proposed API surface

Three new entry points, fitting alongside existing 1v1 + FFA APIs:

```go
// Team is a participant in a team-format update.
type Team struct {
    Members []Rating
}

// UpdateTeams2 runs a decisive 2-team match.
// Returns ratings for winners[i] and losers[i] in the same order they
// appeared in the inputs.
func UpdateTeams2(cfg Config, winners, losers Team) (winnersOut, losersOut Team)

// UpdateTeamDraw mirrors UpdateDraw for team form.
func UpdateTeamDraw(cfg Config, a, b Team) (aOut, bOut Team)

// UpdateMultiTeam runs an N-team match with ranks[i] = team i's
// finishing position. Pairwise team-vs-team decomposition, mirroring
// UpdateMultiplayer's structure.
func UpdateMultiTeam(cfg Config, teams []Team, ranks []int) []Team
```

Plus tracker-aware convenience:

```go
// UpdateTeam takes named-participant teams and one rank per team.
// Side-effects History per individual member, same as FFA Update.
func (ts *TrueSkillRatings) UpdateTeam(teams [][]string, ranks []int)
```

Pairwise analysis (`pairwise.go`) extends naturally:

```go
func TeamWinProbability(cfg Config, a, b Team) float64
func TeamMatchQuality(cfg Config, a, b Team) float64
```

---

## 4. Data model changes

The current `TrueSkillRatings` struct keys everything by `name string`.
A player's TEAM affiliation is purely per-match — there's no persistent
"team" entity.

**V1 model** (recommended): teams are ephemeral per-match groupings.
The tracker stays keyed by individual player name; `UpdateTeam` just
takes `[][]string` (a slice of teams, each a slice of names) and
applies the math. No schema changes. No `Teams map[string]Team`.

**V2 model** (deferred): persistent team identities — for example a
named "TeamName" whose rating is independently tracked alongside its
members. Useful for league play where a team carries reputation across
matches with rotating rosters. Out of scope until use case 4 (informal
alliances) crystallizes into requests.

---

## 5. Implementation phases

### V1: math + ephemeral teams (estimated ~250 LOC + 350 LOC test)

- New file `internal/trueskill/team.go`:
  - `Team` struct
  - `UpdateTeams2`, `UpdateTeamDraw`, `UpdateMultiTeam`
  - Variance-share redistribution helper
- Extend `pairwise.go`:
  - `TeamWinProbability`, `TeamMatchQuality`
- Extend `TrueSkillRatings`:
  - `UpdateTeam([][]string, []int)` — appends to per-member `History`
    same as FFA `Update`
- Tests against the TrueSkill paper's reference values (§7 below).

### V2: persistent team identity (deferred)

- `TrueSkillRatings.Teams map[string]Team` for named, persistent teams
- Roster-change rules (do team-rating σ values inflate when a member
  swaps in? probably yes — analogous to `InheritRating` after
  `CardDelta`).
- Team-vs-individual pairwise queries: `TeamMatchQualityVsPlayer`.

### V3: TBD (cohesion / partial participation)

- β can be reduced for long-time partner teams (less performance
  variance from coordination); the data we'd need is "games played
  together" per pair. Defer until V1 + V2 are in production.

---

## 6. Open questions to resolve before V1 implementation

1. **Drift/velocity integration**: `DetectDrift` and `RatingVelocity`
   scan per-player `History`. Should team-match `RatingDelta` entries
   carry a `Team` tag so a "drift across team matches" filter is
   possible? — recommend: yes, add an optional `TeamSize int` field to
   `RatingDelta` (0 = solo, 2 = 2v2 partner, etc.) so drift detection
   can distinguish "always plays 2v2" from "always plays solo" without
   schema breakage.

2. **Decay**: does `ApplyDecay` need a "last team-match" notion? —
   recommend: no, decay is per-PLAYER inactivity. A player active in
   solo but not team matches is still active.

3. **FFA preset (β = σ · 0.6)**: the FFA preset widens β to account
   for political noise. Does 2v2 inherit that, or use the literature
   default σ/2? — recommend: KEEP β = σ · 0.6 for team commander
   formats (the political surface is wider in 2v2 because 4 humans
   are still at the table); REVERT to σ/2 for non-commander team
   contexts (e.g. cube-draft team play). Plumb β as a per-call
   override on `Config` or accept the FFA preset as a permanent
   commander default — open question for tournament tooling.

4. **Within-team rank**: 2HG has shared life so a tie within the team
   is mandatory. Cube draft team play has individual boards so
   within-team performance is observable. Do we expose a
   `WithinTeamFinish` per member that influences variance-share split?
   — recommend: NO for V1, the standard formula already redistributes
   correctly under "team's outcome is the same for every member." V2
   could add this if cube-draft tournaments produce noisy ratings.

5. **Asymmetric team sizes** (3v2, 4v1): the math handles it (n_A and
   n_B are independent), but the FAIRNESS is dubious — a 4-player
   team carries 4× the σ² contribution. Recommend supporting in V1
   for completeness but documenting that mismatched sizes will produce
   skewed updates.

---

## 7. Reference values for testing

From the TrueSkill paper / Microsoft's reference implementation,
verify against:

**2v2, equal ratings, decisive (team A wins)**:

```
Inputs:   four players at μ=25, σ=8.333, β=σ/2, τ=σ/100, drawP=0.10
Team A wins.
Expected outputs:
  Team A members: μ ≈ 28.108, σ ≈ 7.770
  Team B members: μ ≈ 21.892, σ ≈ 7.770
```

(Each team member ends at the same rating because they entered equal —
the variance-share split is uniform.)

**2v2, mixed ratings, decisive**:

```
Inputs:
  Team A: P1(μ=30, σ=5),  P2(μ=20, σ=8)
  Team B: P3(μ=25, σ=6),  P4(μ=25, σ=6)
Team A wins.
Expected outputs (per Microsoft reference):
  P1: μ ≈ 30.667, σ ≈ 4.954
  P2: μ ≈ 22.732, σ ≈ 7.811
  P3: μ ≈ 23.890, σ ≈ 5.871
  P4: μ ≈ 23.890, σ ≈ 5.871
```

P2 absorbs more Δμ than P1 because P2's σ² share of σ²_team is
larger — variance-share split working correctly.

**Pin in tests** as `internal/trueskill/team_test.go`. Tolerance ≈ 1e-3
for the rational-approximation noise (same as `pairwise_test.go`'s
zero-DrawProb collapse test).

---

## 8. What this does NOT need

- **No DB migration** for V1: teams are ephemeral per-match groupings.
  `TrueSkillRatings` schema unchanged.
- **No frontend changes** for V1: ratings still surface per individual;
  team-format tournament UIs route through the existing leaderboard.
- **No tournament-runner overhaul**: `internal/tournament` would gain
  a "team mode" flag that switches its `Update` call from FFA to team
  form, but the surrounding round-robin / gauntlet machinery is
  unchanged.

---

## 9. Recommendation

Land V1 (ephemeral-teams math + tests against reference values + tracker
integration) as one PR. Defer V2 (persistent team identity) until a
real-world team-league tournament asks for it. V3 (cohesion factors) is
research territory and shouldn't gate any of the above.
