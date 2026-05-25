package trueskill

import (
	"math"
	"sort"
)

// RatingVelocity summarizes the rate of change in a player's μ over a
// recent window of games. The headline metric is MuPerGame — the signed
// per-game slope of μ — which is positive for a player whose rating is
// climbing and negative for one whose rating is falling. R2 (the
// coefficient of determination of the linear fit) tells callers whether
// the trajectory is a sustained trend (R² near 1) or a random walk
// (R² near 0).
//
// Use cases:
//
//   - Smurf detection: a player with miscalibrated initial μ produces
//     a high-|MuPerGame| trajectory (either direction) with high R² as
//     the rating system races toward their true skill.
//   - Genuine improvement / decline: a sustained positive (or negative)
//     MuPerGame with high R² over a long window indicates real skill
//     change rather than initial-rating correction.
//   - Volatility flag: high |MuPerGame| with LOW R² indicates a noisy
//     player whose rating is bouncing without a clear direction.
//
// Distinct from DetectDrift: DetectDrift returns a threshold-based
// magnitude flag (|Δμ|/σ); RatingVelocity returns a signed slope with
// trend strength. Drift is the alert; velocity is the dashboard number.
type RatingVelocity struct {
	Name        string  `json:"name"`
	Window      int     `json:"window"`        // actual number of games used
	MuPerGame   float64 `json:"mu_per_game"`   // signed OLS slope of μ vs game index
	NetMuChange float64 `json:"net_mu_change"` // last_μ − first_μ over the window
	FirstMu     float64 `json:"first_mu"`
	LastMu      float64 `json:"last_mu"`
	R2          float64 `json:"r2"` // 0..1; 1 = perfectly linear, 0 = no trend
}

// RatingVelocity returns the trend in μ for the named player over the
// most recent `window` games. If the player's history has fewer than
// `window` entries, all available entries are used and the Window field
// reflects the actual count. Returns the zero value if the player isn't
// tracked or has fewer than 2 history entries (slope is undefined for
// a single point).
//
// Pass window ≤ 0 to use the full history.
func (ts *TrueSkillRatings) RatingVelocity(name string, window int) RatingVelocity {
	history, ok := ts.History[name]
	if !ok || len(history) < 2 {
		return RatingVelocity{}
	}
	if window <= 0 || window > len(history) {
		window = len(history)
	}
	recent := history[len(history)-window:]

	// OLS slope of μ_after vs game-index (0..window-1). Pure linear
	// trajectories produce slope = (last − first) / (window − 1).
	n := float64(len(recent))
	var sumX, sumY, sumXY, sumX2 float64
	for i, d := range recent {
		x := float64(i)
		y := d.MuAfter
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	var slope float64
	if denom != 0 {
		slope = (n*sumXY - sumX*sumY) / denom
	}

	// R² = 1 − SS_res / SS_tot.
	meanY := sumY / n
	intercept := meanY - slope*(sumX/n)
	var ssRes, ssTot float64
	for i, d := range recent {
		x := float64(i)
		y := d.MuAfter
		pred := intercept + slope*x
		ssRes += (y - pred) * (y - pred)
		ssTot += (y - meanY) * (y - meanY)
	}
	var r2 float64
	switch {
	case ssTot == 0 && ssRes == 0:
		// Perfectly flat trajectory — slope is 0 and the constant fit
		// is perfect. Conventional choice: R² = 1.
		r2 = 1
	case ssTot == 0:
		// Shouldn't happen (ssRes > 0 with ssTot = 0 is impossible for
		// a least-squares fit through the mean), but guard anyway.
		r2 = 0
	default:
		r2 = 1 - ssRes/ssTot
		if r2 < 0 {
			r2 = 0
		}
	}

	first := recent[0].MuAfter
	last := recent[len(recent)-1].MuAfter
	return RatingVelocity{
		Name:        name,
		Window:      window,
		MuPerGame:   slope,
		NetMuChange: last - first,
		FirstMu:     first,
		LastMu:      last,
		R2:          r2,
	}
}

// TopVelocities returns the n players with the highest |MuPerGame| over
// the given window, sorted by absolute velocity descending. Players
// with fewer than 2 history entries are skipped. Pass n ≤ 0 to return
// all tracked players. This is the headline "who's moving fastest"
// dashboard query — high |velocity| with high R² is the smurf /
// improvement signal; high |velocity| with low R² is volatility.
func (ts *TrueSkillRatings) TopVelocities(window, n int) []RatingVelocity {
	out := make([]RatingVelocity, 0, len(ts.History))
	for name, history := range ts.History {
		if len(history) < 2 {
			continue
		}
		out = append(out, ts.RatingVelocity(name, window))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return math.Abs(out[i].MuPerGame) > math.Abs(out[j].MuPerGame)
	})
	if n > 0 && n < len(out) {
		out = out[:n]
	}
	return out
}
