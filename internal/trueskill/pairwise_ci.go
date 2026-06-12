package trueskill

// MatchCandidateCI augments MatchCandidate with a CI on WinProbability.
// Lo/Hi flank Point (which equals MatchCandidate.WinProbability) at the
// requested k-σ skill posterior bounds.
type MatchCandidateCI struct {
	MatchCandidate
	WinProbabilityLo float64 `json:"win_probability_lo"`
	WinProbabilityHi float64 `json:"win_probability_hi"`
	CIWidth          float64 `json:"ci_width"` // Hi − Lo; convenience for sorting/filtering
}
