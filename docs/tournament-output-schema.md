# Tournament Output Schema

> **Canonical JSON shape emitted by every `internal/tournament/` entry point.** Pinned by `internal/tournament/results.go` (struct + `json:` tags), `internal/tournament/schema.go` (validator + version + mode constants), and `internal/tournament/schema_test.go` + `schema_integration_test.go` (regressions).

| | |
|---|---|
| **Current schema version** | `1.0.0` (`tournament.SchemaVersion`) |
| **Validator entrypoint** | `tournament.ValidateSchema(r) error` |
| **Source of truth** | `internal/tournament/results.go` struct field tags |
| **Mode discriminator** | `mode` field — one of `rotate`, `pool`, `lazy_pool`, `swiss`, `double_elim`, `balanced_pool`, `round_robin` |

---

## Envelope

Every TournamentResult JSON document carries two envelope fields that consumers SHOULD check on deserialization before reading further:

```json
{
  "schema_version": "1.0.0",
  "mode": "rotate",
  ...
}
```

- `schema_version` — bumped on breaking shape changes (field removed, semantic flipped, type changed). Adding new optional fields does NOT bump it. Consumers that don't recognize the version should refuse the payload.
- `mode` — identifies which entry function produced the result. The validator uses this to decide which mode-specific fields are required vs forbidden (cross-mode leakage detection).

---

## Universal fields

These appear on every result regardless of mode. The `omitempty` column shows whether the field is omitted when zero/empty in the wire format.

| JSON key | Go field | Type | omitempty | Semantics |
|:---|:---|:---|:---:|:---|
| `schema_version` | `SchemaVersion` | string | no | semver, current = `1.0.0` |
| `mode` | `Mode` | string | no | one of the seven mode strings |
| `games` | `Games` | int | no | successful (non-crashed) game count |
| `crashes` | `Crashes` | int | no | panicked / recovered games |
| `crash_logs` | `CrashLogs` | []string | yes | one string per crash + aggregator drops |
| `draws` | `Draws` | int | no | games ended with no decisive winner |
| `avg_turns` | `AvgTurns` | float64 | no | mean turn count across `games` |
| `commander_names` | `CommanderNames` | []string | no | parallel to deck index |
| `wins_by_commander` | `WinsByCommander` | map[string]int | no | name → win count |
| `total_concessions` | `TotalConcessions` | int | no | conviction concession events summed |
| `duration_ns` | `Duration` | int64 (nanoseconds) | no | wall-clock for the Run call |
| `games_per_second` | `GamesPerSecond` | float64 | no | `games / duration_seconds` |
| `n_seats` | `NSeats` | int | no | seat count per game |
| `turn_distribution` | `TurnDistribution` | [4]int | no | [0]=1-5, [1]=6-10, [2]=11-20, [3]=21+ |
| `elo` | `ELO` | []ELOEntry | yes | per-commander ELO, sorted desc |
| `trueskill` | `TrueSkill` | []TrueSkillEntry | yes | per-commander Bayesian rating |

### Per-pair / per-seat fields (rotate / round-robin / swiss / DE / balanced)

These are populated when the mode actually tracks them; pool and lazy-pool modes ship lighter aggregates and skip several of these:

| JSON key | Type | When populated |
|:---|:---|:---|
| `wins_by_seat` | []int | rotate (post-rotation seat bias) |
| `wins_by_commander_by_seat` | map[string][]int | rotate |
| `games_by_commander_by_seat` | map[string][]int | rotate |
| `elimination_by_commander_by_slot` | map[string][]int | every mode that runs full aggregation |
| `avg_turn_to_win` | map[string]float64 | every mode that runs full aggregation |
| `matchup_matrix` | map[string]map[string]int | every mode that runs full aggregation |
| `matchup_games` | map[string]map[string]int | every mode that runs full aggregation |
| `games_played_by_commander` | map[string]int | every mode (esp. round-robin where decks play subsets) |

### Audit / analytics-gated fields

| JSON key | Type | Gate |
|:---|:---|:---|
| `total_mode_changes` | int | PokerHat usage (any mode) |
| `audit_violations` | int | `AuditEnabled = true` |
| `parser_gap_snippets` | map[string]int | `AuditEnabled = true` |
| `analyses` | []GameAnalysis | `AnalyticsEnabled = true` |
| `card_rankings` | []CardRanking | `AnalyticsEnabled = true` |
| `matchup_details` | []MatchupDetail | `AnalyticsEnabled = true` |
| `kill_records` | []KillRecord | `AuditEnabled = true` |
| `concession_records` | []ConcessionRecord | always (lightweight) |

---

## Mode-specific population

The validator enforces these as both "must populate" and "must NOT populate" rules. Cross-mode leakage (e.g. rotate-mode result carrying `swiss_standings`) is rejected — it indicates a corrupted or hand-merged payload.

| Field | rotate | pool | lazy_pool | swiss | double_elim | balanced_pool | round_robin |
|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `swiss_standings` | — | — | — | **required** | forbidden | forbidden | — |
| `elimination_standings` | — | — | — | forbidden | **required** | forbidden | — |
| `balanced_pod_plan` | — | — | — | forbidden | forbidden | **required** | — |

`—` = optional / absent. `required` = must be non-nil when `games > 0`. `forbidden` = must be nil (cross-mode leakage).

---

## Conservation invariants

The validator checks the following on every result with `games > 0`:

- `sum(wins_by_commander) + draws ≤ games` — wins + draws can't exceed total games. (Equality is the strict case for modes that fully account for every game; pool/lazy_pool aggregators may have crashed games shifted into the `crashes` count instead.)
- `n_seats >= 2` — every mode needs at least 2 seats.
- `games`, `crashes`, `draws` all non-negative.
- `commander_names` non-empty.
- `wins_by_commander` non-nil (empty map allowed when no decisive games yet).

---

## Bump policy

The schema version follows semver semantics applied to wire-format compatibility:

- **Patch bump** (`1.0.0 → 1.0.1`) — bug fix in a field's semantics that consumers shouldn't have to react to (e.g. a counter that was being double-counted now counts once). No code change required by consumers.
- **Minor bump** (`1.0.0 → 1.1.0`) — new optional field added. Old consumers continue to work (they ignore the new field); new consumers can opt to use it.
- **Major bump** (`1.0.0 → 2.0.0`) — field removed, field type changed, or field semantics flipped (e.g. an int that was 0-indexed becomes 1-indexed). Old consumers MUST be updated.

When bumping, update:

1. `SchemaVersion` const in `internal/tournament/results.go`.
2. `TestSchema_RoundTripJSON` if the shape changed.
3. `docs/tournament-output-schema.md` (this file) — record the change in a new dated section + the diff vs the previous version.
4. Any downstream consumer that pinned the old version.

---

## Example minimal valid documents

### Rotate (default)

```json
{
  "schema_version": "1.0.0",
  "mode": "rotate",
  "games": 100,
  "crashes": 0,
  "draws": 0,
  "avg_turns": 12.4,
  "commander_names": ["A", "B", "C", "D"],
  "wins_by_commander": {"A": 30, "B": 25, "C": 22, "D": 23},
  "wins_by_seat": [25, 27, 24, 24],
  "total_concessions": 4,
  "duration_ns": 120000000000,
  "games_per_second": 0.83,
  "n_seats": 4,
  "turn_distribution": [0, 20, 70, 10],
  "elo": [...],
  "trueskill": [...]
}
```

### Swiss

```json
{
  "schema_version": "1.0.0",
  "mode": "swiss",
  "games": 12,
  "n_seats": 4,
  ...,
  "swiss_standings": [
    {"commander_name": "A", "points": 9, "games": 3, "wins": 3, "losses": 0, "draws": 0, "byes": 0, "win_rate": 1.0},
    ...
  ]
}
```

### Double elimination

```json
{
  "schema_version": "1.0.0",
  "mode": "double_elim",
  ...,
  "elimination_standings": [
    {"commander_name": "A", "final_rank": 1, "losses_at_end": 0, "pods_played": 5, "game_wins": 5, "game_losses": 0, "game_draws": 0},
    ...
  ]
}
```

### Balanced pool

```json
{
  "schema_version": "1.0.0",
  "mode": "balanced_pool",
  ...,
  "balanced_pod_plan": [
    {"deck_indices": [0, 3, 4, 7], "commander_names": ["A", "D", "E", "H"], "pod_strength": 260.0},
    ...
  ]
}
```

---

## Validator usage

```go
result, err := tournament.Run(cfg)
if err != nil {
    return err
}
if err := tournament.ValidateSchema(result); err != nil {
    // Refuse to ship a corrupt result downstream. ValidateSchema joins
    // every violation into one error message so logs show the full gap.
    return fmt.Errorf("tournament result corrupt: %w", err)
}
```

The validator is also useful inside golden-file regression tests — round-tripping a stored JSON through Unmarshal + ValidateSchema confirms the shape is still consumable by the current codebase.

---

## Audit trail

| Date | Schema version | Change |
|:---|:---:|:---|
| 2026-05-30 | `1.0.0` | Initial canonical schema. Added `SchemaVersion` + `Mode` envelope, `json:` tags on every field of TournamentResult, ValidateSchema helper, per-mode integration regressions. Before this point, the struct marshaled with Go-default capitalized keys and only Swiss/DE/BalancedPool standings had explicit tags. |

---

## See also

- [`internal/tournament/results.go`](../internal/tournament/results.go) — TournamentResult struct + envelope constants
- [`internal/tournament/schema.go`](../internal/tournament/schema.go) — ValidateSchema + Mode constants
- [`internal/tournament/schema_test.go`](../internal/tournament/schema_test.go) — validator unit tests
- [`internal/tournament/schema_integration_test.go`](../internal/tournament/schema_integration_test.go) — per-mode end-to-end validation
