package trueskill

// composition_update.go — wires the CompositionPrior (PR #403) into
// the actual TrueSkill rating-update path.
//
// Per the PR #398 design: when a TrueSkill update happens, instead of
// feeding the raw μ values into the rank-prediction, feed adjusted
// values μ + Δ where Δ encodes the composition's effect on each
// archetype's expected winrate. The Gaussian update is invariant to
// uniform shifts but NOT to differential shifts — so the residual
// (observed rank minus prior-adjusted prediction) cleanly separates
// "player skill" from "deck-in-composition baseline."
//
// Mathematical sketch for player i in a 4-archetype pod:
//
//   raw_μ_i      = current rating mean
//   expected_i   = CompositionPrior.ExpectedWinrate(arch_i, pod)
//   confidence_i = CompositionPrior.Confidence(arch_i, pod)
//   offset_i     = Weight × confidence_i × MuOffsetScale ×
//                  (expected_i - 1/podSize)
//   shifted_μ_i  = raw_μ_i + offset_i
//
// Run standard TrueSkill with shifted_μ values, observe Δshifted_μ_i,
// apply that delta back to raw_μ_i. Confidence gates the offset so
// cold-start cells don't shift at all (the prior knows nothing yet).

// CompositionUpdateConfig controls how aggressively a CompositionPrior
// shapes each TrueSkill update.
type CompositionUpdateConfig struct {
	// Prior is the composition-conditioned expected-winrate model.
	// When nil, UpdateWithComposition behaves identically to Update.
	Prior *CompositionPrior

	// Weight scales the composition offset from 0 (ignore the prior,
	// vanilla TrueSkill) to 1 (apply the prior's full offset). 0.5 is
	// the design's recommended starting point — "split the difference
	// between raw-skill and pod-conditioned."
	Weight float64

	// MuOffsetScale converts a winrate-deviation (in probability
	// units 0..1) into μ-points. Default 10 makes a 25pp composition
	// advantage shift μ by 2.5 — about 30% of σ=8.33, large enough to
	// be measurable but small enough not to swamp the player-skill
	// signal in any single game.
	MuOffsetScale float64
}

// DefaultCompositionUpdateConfig returns the recommended starting
// config for a non-nil prior. Weight=0.5, MuOffsetScale=10. Tune
// from live-data results.
func DefaultCompositionUpdateConfig(prior *CompositionPrior) CompositionUpdateConfig {
	return CompositionUpdateConfig{
		Prior:         prior,
		Weight:        0.5,
		MuOffsetScale: 10.0,
	}
}

// ComputeCompositionOffset returns the μ-shift the prior would apply
// for one deck in the given pod. Exposed so functional callers
// (e.g. showmatch.updateELO, which uses Update2Player directly
// rather than a TrueSkillRatings instance) can compute the offset
// inline without re-implementing the Confidence × Weight × Scale ×
// (ExpectedWinrate - uniform) formula.
//
// Returns 0 when prior is nil, weight ≤ 0, or the prior has no data
// for this cell (confidence = 0). Safe to call on any input.
func ComputeCompositionOffset(prior *CompositionPrior, archetype string, pod []string, cfg CompositionUpdateConfig) float64 {
	if prior == nil || cfg.Weight <= 0 || archetype == "" {
		return 0
	}
	scale := cfg.MuOffsetScale
	if scale == 0 {
		scale = 10.0
	}
	podSize := prior.podSizeOrDefault()
	uniform := 1.0 / float64(podSize)
	confidence := prior.Confidence(archetype, pod)
	if confidence == 0 {
		return 0
	}
	expected := prior.ExpectedWinrate(archetype, pod)
	return cfg.Weight * confidence * scale * (expected - uniform)
}

// UpdateMultiplayerWithOffsets is UpdateMultiplayer with composition-
// prior μ-offsets baked in. The offsets are added to each rating's μ
// before the rank-prediction, then the resulting Δμ is applied back
// to the raw μ (NOT the shifted μ). σ updates flow through unchanged
// — the Gaussian-precision update is invariant to constant μ shifts.
//
// Use this in place of the older pairwise-chained
// Update2PlayerWithOffsets loop for FFA pods. The old loop did one
// Update2Player call per (winner, loser_i) pair and chained the
// winner's accumulated rating, which over-credited the winner AND
// gave each loser a full 1v1 decisive-loss signal — wrong for 4P FFA
// where each loser only "lost" 1/3 of a game's worth of signal.
//
// offsets[i] is the prior-derived μ-shift for participant i (compute
// via ComputeCompositionOffset). Length must equal len(ratings); if
// not, this function falls back to vanilla UpdateMultiplayer.
func UpdateMultiplayerWithOffsets(cfg Config, ratings []Rating, ranks []int, offsets []float64) []Rating {
	n := len(ratings)
	if n < 2 || len(ranks) != n {
		out := make([]Rating, n)
		copy(out, ratings)
		return out
	}
	if len(offsets) != n {
		return UpdateMultiplayer(cfg, ratings, ranks)
	}
	shifted := make([]Rating, n)
	for i, r := range ratings {
		shifted[i] = Rating{Mu: r.Mu + offsets[i], Sigma: r.Sigma}
	}
	updated := UpdateMultiplayer(cfg, shifted, ranks)
	out := make([]Rating, n)
	for i := range ratings {
		out[i] = Rating{
			Mu:    ratings[i].Mu + (updated[i].Mu - shifted[i].Mu),
			Sigma: updated[i].Sigma,
		}
	}
	return out
}

// UpdateWithComposition runs a TrueSkill rating update with a
// composition-prior adjustment applied to each participant's μ before
// the rank-prediction.
//
// participantNames, ranks: same shape as Update (seat order; ranks[i]
// = finishing position of participant i, 0 = winner).
//
// archetypes: the archetype label for participant i, used to query
// the prior. Length must equal len(participantNames) — if not, this
// function degrades to standard Update.
//
// If cfg.Prior is nil, cfg.Weight ≤ 0, or any per-cell confidence is
// 0 (cold-start), the offset for that cell is 0 — the update for
// that player matches standard TrueSkill exactly.
//
// As with Update, RatingDelta is appended to History[name] for each
// participant.
func (ts *TrueSkillRatings) UpdateWithComposition(
	participantNames []string,
	ranks []int,
	archetypes []string,
	cfg CompositionUpdateConfig,
) {
	n := len(participantNames)
	if n < 2 || len(ranks) != n {
		return
	}
	// Length-mismatched archetypes → fall back to standard Update so
	// we don't silently mis-attribute archetype offsets.
	if len(archetypes) != n {
		ts.Update(participantNames, ranks)
		return
	}
	// Nil-prior or zero-weight: short-circuit to standard Update.
	if cfg.Prior == nil || cfg.Weight <= 0 {
		ts.Update(participantNames, ranks)
		return
	}
	if cfg.MuOffsetScale == 0 {
		cfg.MuOffsetScale = 10.0
	}

	if ts.History == nil {
		ts.History = make(map[string][]RatingDelta, n)
	}

	// Snapshot current ratings.
	before := make([]Rating, n)
	for i, name := range participantNames {
		before[i] = ts.Ratings[name]
		ts.Games[name]++
	}

	// Compute per-participant offsets from the prior. Self-mirror
	// seats (an archetype appearing more than once in the pod) get
	// the same offset — the prior's ExpectedWinrate already skips
	// mirror seats from the averaging.
	podSize := cfg.Prior.podSizeOrDefault()
	uniformWinrate := 1.0 / float64(podSize)
	offsets := make([]float64, n)
	for i, arch := range archetypes {
		expected := cfg.Prior.ExpectedWinrate(arch, archetypes)
		confidence := cfg.Prior.Confidence(arch, archetypes)
		offsets[i] = cfg.Weight * confidence * cfg.MuOffsetScale *
			(expected - uniformWinrate)
	}

	// Build shifted ratings: μ_i + offset_i, σ unchanged.
	shifted := make([]Rating, n)
	for i := range before {
		shifted[i] = Rating{Mu: before[i].Mu + offsets[i], Sigma: before[i].Sigma}
	}

	// Run the standard TrueSkill update on the shifted ratings.
	updatedShifted := UpdateMultiplayer(ts.cfg, shifted, ranks)

	// Recover raw μ updates: the change in μ from the shifted update
	// applies back to the raw μ. σ updates flow through unchanged
	// (the Gaussian-precision update is offset-invariant).
	for i, name := range participantNames {
		muDelta := updatedShifted[i].Mu - shifted[i].Mu
		newRating := Rating{
			Mu:    before[i].Mu + muDelta,
			Sigma: updatedShifted[i].Sigma,
		}
		ts.Ratings[name] = newRating
		ts.History[name] = append(ts.History[name], RatingDelta{
			Game:        ts.Games[name],
			MuBefore:    before[i].Mu,
			MuAfter:     newRating.Mu,
			SigmaBefore: before[i].Sigma,
			SigmaAfter:  newRating.Sigma,
			Rank:        ranks[i],
		})
	}
}
