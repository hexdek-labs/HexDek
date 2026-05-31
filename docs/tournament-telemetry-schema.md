# Tournament Telemetry Schema

> **Derived analytics layer over a `TournamentResult`.** Surfaces convergence, win-rate trajectories, archetype performance, and ELO distribution — the signals a gauntlet operator needs to decide "did this run produce stable rankings, or do I need more games?"

| | |
|---|---|
| **Telemetry schema version** | `1.0.0` (`tournament.TelemetrySchemaVersion`) |
| **Producer** | `tournament.ComputeTelemetry(r, archetypes) *Telemetry` |
| **Source** | A finished `TournamentResult` (any `Mode`) plus an optional per-deck `Archetype` slice |
| **Related schema** | [`docs/tournament-output-schema.md`](tournament-output-schema.md) — the underlying result shape |

---

## Envelope

```json
{
  "schema_version": "1.0.0",
  "source_mode": "swiss",
  "games": 12,
  "crashes": 0,
  ...
}
```

- `schema_version` — semver, bumped on breaking shape changes per the same policy as `TournamentResult`'s envelope.
- `source_mode` — copies the producing `TournamentResult.Mode` so downstream tooling knows whether per-round trajectories are present.

---

## Sections

### `commanders` — per-deck summary row

Sorted by ELO descending; ties broken by name ascending.

```json
{
  "name": "Atraxa",
  "games": 12,
  "wins": 6,
  "win_rate": 0.5,
  "elo": 1620.0,
  "trueskill_mu": 30.5,
  "trueskill_sigma": 3.8,
  "archetype": "Combo / Infinite"
}
```

`archetype` is `omitempty` — only present when `ComputeTelemetry` was passed a parallel `[]Archetype` slice and the deck's tag isn't `ArchetypeUnknown`.

### `convergence` — headline "is this gauntlet done?" signal

```json
{
  "initial_sigma": 8.333,
  "mean_sigma": 4.025,
  "min_sigma": 3.8,
  "max_sigma": 4.2,
  "convergence_ratio": 0.517,
  "converged": true
}
```

- `initial_sigma` — TrueSkill's per-deck σ at the start of any session (25/3). Pinned for reference.
- `mean_sigma` — final mean σ across all commanders. Lower = more converged.
- `min_sigma` / `max_sigma` — surfaces the tightest- and loosest-known decks. A large gap means some decks were under-played relative to others.
- `convergence_ratio` = `1 - mean_sigma / initial_sigma`. **0.0** = no convergence (every deck still at the prior); **1.0** = fully resolved. Clamped at 0 on the low end.
- `converged` — `true` when `convergence_ratio >= 0.5` (the `SigmaConvergedRatio` constant). Headline yes/no signal.

### `elo_distribution` — ratings spread

```json
{
  "min": 1400.0,
  "max": 1620.0,
  "mean": 1500.0,
  "std_dev": 80.6,
  "spread": 220.0
}
```

A tight ELO spread means the decks haven't differentiated — either because they're genuinely close or because the gauntlet is under-run. A wide spread + high `convergence_ratio` is the calibration goal.

### `archetype_matrix` (optional) — per-archetype rollup

Populated only when `ComputeTelemetry` is called with a non-nil archetypes slice. Keys are the canonical `Archetype` string values (see `internal/tournament/archetype.go`).

```json
{
  "Combo / Infinite": {"decks": 2, "games": 24, "wins": 8, "win_rate": 0.333},
  "Aggro / Go Wide":  {"decks": 1, "games": 12, "wins": 3, "win_rate": 0.250},
  "Control":          {"decks": 1, "games": 12, "wins": 1, "win_rate": 0.083}
}
```

Aggregates by archetype across every deck tagged with that archetype. Decks tagged `ArchetypeUnknown` are excluded from the matrix.

### `rounds` (optional) — per-round trajectories

Populated only for round-structured modes that emit `RoundSnapshots` (currently Swiss; DE / Round-Robin / Balanced-Pool may opt in later). One entry per round in chronological order.

```json
[
  {
    "round": 1,
    "sigma_mean": 6.675,
    "mu_spread": 3.0,
    "win_rates": {"Atraxa": 1.0, "Krenko": 0.0, "Yuriko": 0.0, "Edric": 0.0},
    "omw": {"Atraxa": 0.0, "Krenko": 0.33, "Yuriko": 0.33, "Edric": 0.33}
  },
  {
    "round": 2,
    "sigma_mean": 5.7,
    "mu_spread": 5.5,
    "win_rates": {"Atraxa": 1.0, "Krenko": 0.5, "Yuriko": 0.0, "Edric": 0.0},
    "omw": {"Atraxa": 0.20, "Krenko": 0.25, "Yuriko": 0.50, "Edric": 0.50}
  }
]
```

- `sigma_mean` — per-round mean σ. Should fall monotonically as the tournament converges; a flat trajectory means later rounds aren't adding information.
- `mu_spread` — per-round `max(μ) - min(μ)`. Should grow as decks differentiate.
- `win_rates` — per-deck cumulative win rate AT END-OF-ROUND. Render across rounds for the heatmap.
- `omw` — per-deck Opponent Match-Win % at end-of-round (Swiss-only). Shows how each deck's strength-of-schedule evolved.

---

## Underlying `RoundSnapshot` (raw capture)

`TournamentResult.RoundSnapshots` is the raw end-of-round capture. `ComputeTelemetry` derives the `rounds` section from it; consumers who need the raw mu/sigma/ELO per round per deck can read `RoundSnapshots` directly.

```json
{
  "round": 1,
  "ratings": {
    "Atraxa": {"elo": 1525, "trueskill_mu": 27.0, "trueskill_sigma": 6.5}
  },
  "cumulative_wins": {"Atraxa": 1},
  "cumulative_games": {"Atraxa": 1},
  "omw": {"Atraxa": 0.0}
}
```

Emitted by `RunSwiss` at end-of-round (after `runRoundOutcomes` has absorbed the round's games). Other round-structured runners can adopt the same pattern; the `captureSwissRoundSnapshot` helper is the reference implementation.

---

## How to use this

### "Did my 500-game gauntlet converge?"

```go
tel := tournament.ComputeTelemetry(result, deckArchetypes)
if !tel.Convergence.Converged {
    log.Printf("UNDER-CONVERGED: mean σ=%.2f (ratio %.2f, need 0.5+). Add more games.",
        tel.Convergence.MeanSigma, tel.Convergence.ConvergenceRatio)
}
```

### "Which archetype dominated?"

```go
type ranked struct{ name string; wr float64 }
var ranks []ranked
for arch, stats := range tel.ArchetypeMatrix {
    ranks = append(ranks, ranked{arch, stats.WinRate})
}
sort.Slice(ranks, func(i, j int) bool { return ranks[i].wr > ranks[j].wr })
```

### "Is the sigma curve still falling, or has it plateaued?"

If `tel.Rounds[N].SigmaMean ≈ tel.Rounds[N-1].SigmaMean`, the late rounds aren't adding rating information — either the format converged earlier than expected (a win), or the matchmaking is stuck in a local optimum (re-pair).

### "Are any decks under-played?"

```go
gap := tel.Convergence.MaxSigma - tel.Convergence.MinSigma
if gap > 1.0 {
    log.Printf("σ gap of %.2f — some decks need more games to catch up", gap)
}
```

---

## Bump policy

Same as `TournamentResult` envelope — semver on wire format:

- **Patch** (`1.0.0 → 1.0.1`) — bug fix in a field's semantics that consumers shouldn't have to react to.
- **Minor** (`1.0.0 → 1.1.0`) — new optional field. Old consumers still work.
- **Major** (`1.0.0 → 2.0.0`) — field removed, type changed, or semantics flipped.

When bumping, update:

1. `TelemetrySchemaVersion` const in `internal/tournament/telemetry.go`
2. The `TestComputeTelemetry_BasicShape` schema-version assertion
3. This doc — add a new dated section to the audit trail below

---

## Audit trail

| Date | Schema version | Change |
|:---|:---:|:---|
| 2026-05-30 | `1.0.0` | Initial canonical schema. Adds Telemetry derived layer + RoundSnapshot capture in Swiss + ComputeTelemetry helper. Tests against `testdata/swiss_converged_fixture.json`. |

---

## See also

- [`internal/tournament/telemetry.go`](../internal/tournament/telemetry.go) — Telemetry struct + ComputeTelemetry
- [`internal/tournament/telemetry_test.go`](../internal/tournament/telemetry_test.go) — regressions
- [`internal/tournament/testdata/swiss_converged_fixture.json`](../internal/tournament/testdata/swiss_converged_fixture.json) — the canonical fixture
- [`docs/tournament-output-schema.md`](tournament-output-schema.md) — the underlying TournamentResult schema
- [`docs/trueskill-tuning-r60.md`](trueskill-tuning-r60.md) — context on the σ values referenced here
