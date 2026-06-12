package judge

// registry.go — the Hex Judge dimension registry (phase 3 final).
//
// One entry per correctness dimension: where its checks live, which
// hook they run at, and how they execute. This is the single
// authoritative map of the Judge's coverage — the integration pin
// (whole_test.go) asserts every registered dimension actually reports
// through LogViolation, so an entry here is a tested claim, not
// documentation.
//
// Execution modes:
//
//   - "inline"          — runs inside live games at the named hook,
//     ride-along style (default-off / nil-receiver-off per home).
//   - "post_game"       — runs once on the completed game state.
//   - "scenario_driver" — independence-based checker: re-derives
//     expectations from first principles and drives the ENGINE'S REAL
//     machinery (effect resolution, trigger dispatch, SBA sweeps)
//     inside controlled scenarios. Deliberately NOT run inline on live
//     boards: its expectations are sound only against the controlled
//     spec (live-board interpretation would false-positive on targets/
//     replacements/doublers the spec pins). Opt-in by invocation —
//     default-off trivially.

// DimensionRegistration describes one dimension's registered presence.
type DimensionRegistration struct {
	// Dimension is the Dimension* constant.
	Dimension string
	// Surfaces are the Surface* tags its violations carry.
	Surfaces []string
	// Home is the package/file owning the checks.
	Home string
	// Hook names where the checks attach.
	Hook string
	// Mode is the execution mode (see file comment).
	Mode string
}

// Dimensions returns the Judge dimension registry. Order is the
// HEX-JUDGE.md table order.
func Dimensions() []DimensionRegistration {
	return []DimensionRegistration{
		{
			Dimension: DimensionLegality,
			Surfaces:  []string{SurfaceLegality},
			Home:      "judge/legality.go (deck) + gameengine/legality.go (action ride-along)",
			Hook:      "deck submission + action announce/finish (record-time tap)",
			Mode:      "inline",
		},
		{
			Dimension: DimensionConservation,
			Surfaces:  []string{SurfaceInvariants},
			Home:      "gameengine/invariants.go (InstanceID strict census)",
			Hook:      "invariant table (per action) + feynman post-game hook",
			Mode:      "inline",
		},
		{
			Dimension: DimensionStateIntegrity,
			Surfaces:  []string{SurfaceInvariants, SurfaceFeynman, SurfaceSeatOutcome},
			Home:      "gameengine invariant table + judge/stateintegrity.go + seat-outcome checker",
			Hook:      "invariant table (per action) + post-game snapshot + elimination/SBA ride-along",
			Mode:      "inline",
		},
		{
			Dimension: DimensionProgression,
			Surfaces:  []string{SurfaceProgression},
			Home:      "judge/progression (trigger-correctness: fire/phantom/controller-gate/intervening-if/delayed)",
			Hook:      "engine trigger-fire + SBA machinery, driven inside controlled scenarios",
			Mode:      "scenario_driver",
		},
		{
			Dimension: DimensionOutcome,
			Surfaces:  []string{SurfaceOutcome},
			Home:      "judge/outcome (AST expected-delta interpreter + harness)",
			Hook:      "engine effect resolution, driven inside controlled scenarios",
			Mode:      "scenario_driver",
		},
		{
			Dimension: DimensionLiveness,
			Surfaces:  []string{SurfaceLiveness},
			Home:      "judge/liveness.go (snapshot checks + driver watchdog helper)",
			Hook:      "driver per-game wall-clock watchdog + post-game cap-contract/turn/event scan",
			Mode:      "scenario_driver", // no engine hooks; off everywhere except an invoking driver
		},
	}
}
