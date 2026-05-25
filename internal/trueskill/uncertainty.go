package trueskill

import "sort"

// Interval returns the symmetric skill interval [μ − k·σ, μ + k·σ] for
// this rating. k is the number of standard deviations: k=1 covers ≈68%
// of the posterior, k=1.96 ≈95%, k=2 ≈95.4%, k=3 ≈99.7%. Use this for
// "true skill is in [a, b] with X% confidence" UI displays. The existing
// Conservative() is equivalent to the lower bound at k=3.
func (r Rating) Interval(k float64) (lo, hi float64) {
	return r.Mu - k*r.Sigma, r.Mu + k*r.Sigma
}

// SkillInterval95 returns the canonical 95% credible interval
// (μ ± 1.96σ). Convenience wrapper over Interval(1.96) for the most
// common display case.
func (r Rating) SkillInterval95() (lo, hi float64) {
	return r.Interval(1.96)
}

// RankInterval expresses the uncertainty in a player's leaderboard
// position. Center is the player's current rank ordered by
// Conservative() (matches Snapshot ordering). Best is the
// best-case rank (smallest number, highest finish) the player could
// hold given a k-σ CI on every player's skill; Worst is the worst-case
// rank (largest number, lowest finish). The "rank 8 ± 2" display
// renders as Center=8, Best=6, Worst=10 (a Plus/Minus of 2).
//
// Best/Worst are computed by interval overlap: a player Q is
// "definitely above P" if Q's pessimistic skill (μ_Q − k·σ_Q) still
// exceeds P's optimistic skill (μ_P + k·σ_P). Such players bound P's
// best-possible rank from above. Symmetrically a player Q is
// "definitely below P" if Q's optimistic skill < P's pessimistic
// skill — those bound P's worst-possible rank from below.
//
// Ranks are 1-indexed (rank 1 = first place).
type RankInterval struct {
	Player string
	Center int
	Best   int
	Worst  int
}

// PlusMinus returns the half-width of the rank interval, i.e. the
// largest of (Center − Best) and (Worst − Center). UI surfaces use it
// for "rank 8 ± 2"-style displays where the interval is summarized by
// one number. When the interval is asymmetric the wider side wins,
// since the smaller side is an under-statement of uncertainty.
func (ri RankInterval) PlusMinus() int {
	below := ri.Center - ri.Best
	above := ri.Worst - ri.Center
	if below > above {
		return below
	}
	return above
}

// RankInterval returns the BestRank/WorstRank window for the named
// player at k standard deviations of CI on every player's skill. k=1.96
// gives a ≈95% interval per player. Returns the zero RankInterval if
// the player isn't tracked.
//
// Algorithm: order all players by Conservative() to assign Center (the
// "official" rank). For Best/Worst, walk all OTHER players and ask
// whether the k-σ intervals overlap with the target's interval.
// Non-overlapping intervals on the "above" side force the target down
// the Best ranking; non-overlapping intervals on the "below" side keep
// them above in the Worst ranking.
func (ts *TrueSkillRatings) RankInterval(name string, k float64) RankInterval {
	target, ok := ts.Ratings[name]
	if !ok {
		return RankInterval{}
	}

	// Center: rank by Conservative (matches Snapshot ordering).
	type entry struct {
		name string
		r    Rating
	}
	entries := make([]entry, 0, len(ts.Ratings))
	for n, r := range ts.Ratings {
		entries = append(entries, entry{n, r})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].r.Conservative() > entries[j].r.Conservative()
	})
	center := 0
	for i, e := range entries {
		if e.name == name {
			center = i + 1 // 1-indexed
			break
		}
	}

	pLo, pHi := target.Interval(k)

	// Best: 1 + (count of players who are definitely above the target).
	// Definitely above ⇔ that player's lo > target's hi.
	// Worst: total − (count of players who are definitely below).
	// Definitely below ⇔ that player's hi < target's lo.
	definitelyAbove := 0
	definitelyBelow := 0
	for _, e := range entries {
		if e.name == name {
			continue
		}
		oLo, oHi := e.r.Interval(k)
		if oLo > pHi {
			definitelyAbove++
		}
		if oHi < pLo {
			definitelyBelow++
		}
	}
	best := definitelyAbove + 1
	worst := len(entries) - definitelyBelow

	return RankInterval{
		Player: name,
		Center: center,
		Best:   best,
		Worst:  worst,
	}
}

// LeaderboardWithIntervals returns the full leaderboard ordered by
// Conservative() with each entry's rank interval attached. The most
// common consumer pattern — render a table with one "rank N ± Δ"
// column per row — collapses into a single call.
func (ts *TrueSkillRatings) LeaderboardWithIntervals(k float64) []RankInterval {
	out := make([]RankInterval, 0, len(ts.Ratings))
	for name := range ts.Ratings {
		out = append(out, ts.RankInterval(name, k))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Center < out[j].Center
	})
	return out
}
