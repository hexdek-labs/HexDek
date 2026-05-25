package trueskill

import "math"

// DecayConfig tunes how inactive ratings drift back toward the baseline.
// All fields are interpreted in "days inactive past the grace period."
// Pass [DefaultDecayConfig] for the FFA-tuned preset.
//
// Rationale: TrueSkill's σ is a posterior uncertainty estimate calibrated
// to the RATE at which the player is generating evidence. When a player
// stops playing, their σ implicitly becomes stale — the system "still
// thinks" it has tight information about a player who hasn't shown up
// in months. Decay re-introduces uncertainty (σ inflation) and slowly
// pulls μ back toward the prior (BaseMu) so a returning player gets
// re-measured against current opponents rather than coasting on
// year-old form.
//
// Decay is INTENTIONALLY conservative — short trips (vacation, season
// gaps) shouldn't penalize active players. The GraceDays buffer skips
// decay entirely for short absences; the HalfLifeDays scaling means the
// μ-drift is gradual even past grace.
type DecayConfig struct {
	// GraceDays — number of inactive days before decay kicks in at all.
	// Within the grace window the rating is returned unchanged.
	GraceDays int

	// HalfLifeDays — exponential decay half-life for μ → BaseMu drift,
	// measured in inactive days PAST the grace period. After exactly one
	// half-life past grace, μ has moved halfway from its current value
	// toward BaseMu. After two half-lives, three-quarters of the way, etc.
	HalfLifeDays int

	// SigmaInflationPerDay — linear σ regrowth per inactive day past
	// grace. Capped by MaxSigma so a player can't inflate beyond the
	// initial prior. Small (~0.05) so a year of inactivity gives ~18
	// units of inflation — comfortably below the σ₀ ≈ 8.33 cap.
	SigmaInflationPerDay float64

	// BaseMu — the prior mean μ pulls back toward (typically the
	// default rating's μ, 25.0).
	BaseMu float64

	// MaxSigma — σ cap after inflation. Typically the default prior σ
	// (σ₀ ≈ 8.33); never let an inactive player display LESS confident
	// than a brand-new player.
	MaxSigma float64
}

// DefaultDecayConfig returns the FFA-tuned decay preset:
//   - 14-day grace (skip decay for short breaks).
//   - 90-day half-life (μ drifts halfway to base after 3 months past grace).
//   - 0.05 σ/day inflation (~1.5 σ per month past grace).
//   - BaseMu = 25, MaxSigma = σ₀ ≈ 8.33 (prior values).
func DefaultDecayConfig() DecayConfig {
	return DecayConfig{
		GraceDays:            14,
		HalfLifeDays:         90,
		SigmaInflationPerDay: 0.05,
		BaseMu:               defaultMu,
		MaxSigma:             defaultSigma,
	}
}

// Decayed returns this rating after `daysInactive` days of no activity,
// per the supplied DecayConfig. Pure function — does not mutate r.
//
// Math:
//
//	d = max(0, daysInactive − GraceDays)
//	μ' = BaseMu + (μ − BaseMu) * (1/2)^(d / HalfLifeDays)
//	σ' = min(σ + d * SigmaInflationPerDay, MaxSigma)
//
// If daysInactive ≤ GraceDays the rating is returned unchanged. A
// rating already at exactly BaseMu stays at BaseMu (μ-drift is
// proportional to displacement from BaseMu). σ never decreases.
func (r Rating) Decayed(daysInactive int, cfg DecayConfig) Rating {
	if daysInactive <= cfg.GraceDays {
		return r
	}
	d := float64(daysInactive - cfg.GraceDays)

	muNew := r.Mu
	if cfg.HalfLifeDays > 0 {
		factor := math.Pow(0.5, d/float64(cfg.HalfLifeDays))
		muNew = cfg.BaseMu + (r.Mu-cfg.BaseMu)*factor
	}

	sigmaNew := r.Sigma + d*cfg.SigmaInflationPerDay
	if cfg.MaxSigma > 0 && sigmaNew > cfg.MaxSigma {
		sigmaNew = cfg.MaxSigma
	}
	if sigmaNew < r.Sigma {
		// Defensive: never let decay decrease σ.
		sigmaNew = r.Sigma
	}
	return Rating{Mu: muNew, Sigma: sigmaNew}
}

// ApplyDecay mutates the tracker in place, replacing every rating named
// in `daysInactive` with its decayed counterpart. Players not present
// in the map are untouched. The caller (tournament runner / server)
// owns the timestamps and computes inactive-days per player; this
// package is pure rating math and intentionally has no clock.
//
// Use [DefaultDecayConfig] for the FFA-tuned preset, or build a custom
// [DecayConfig] for a different policy (e.g. faster decay for ladder
// seasons).
func (ts *TrueSkillRatings) ApplyDecay(daysInactive map[string]int, cfg DecayConfig) {
	for name, days := range daysInactive {
		r, ok := ts.Ratings[name]
		if !ok {
			continue
		}
		ts.Ratings[name] = r.Decayed(days, cfg)
	}
}
