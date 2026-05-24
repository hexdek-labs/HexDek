package trueskill

import (
	"math"
	"sort"
)

const (
	defaultMu    = 25.0
	defaultSigma = defaultMu / 3.0

	// ffaBetaScale tunes the performance-noise parameter for 4-player
	// free-for-all Commander pods. Microsoft's 1v1 default is β = σ/2
	// (≈4.17), calibrated for chess/Halo where game outcomes are a
	// relatively crisp function of skill. Commander adds three sources of
	// per-game noise that don't exist in 1v1: political negotiations
	// (kingmaking, threat-assessment), mana variance (flood/screw is far
	// more decisive in a slower format), and table position effects (the
	// player most threatening from seat 0 absorbs more aggression than
	// the same skill level from seat 2). Bumping β to σ·0.6 (≈5.0) adds
	// ~20% performance-noise so each update is appropriately humbler.
	// Convergence stays fast because 4-player FFA already produces ~3×
	// the pairwise comparisons per game vs 1v1.
	ffaBetaScale = 0.6
)

type Rating struct {
	Mu    float64 `json:"mu"`
	Sigma float64 `json:"sigma"`
}

func DefaultRating() Rating {
	return Rating{Mu: defaultMu, Sigma: defaultSigma}
}

func (r Rating) Conservative() float64 {
	return r.Mu - 3*r.Sigma
}

type Config struct {
	Beta            float64
	Tau             float64
	DrawProbability float64
}

// DefaultConfig returns the Microsoft TrueSkill 1v1 reference config:
// β = σ/2, τ = σ/100, draw probability 2%. Use this for head-to-head
// (Update2Player) call sites that want the literature defaults. For the
// 4-player FFA pod context HexDek primarily runs in, prefer
// [DefaultFFAConfig].
func DefaultConfig() Config {
	return Config{
		Beta:            defaultSigma / 2.0,
		Tau:             defaultSigma / 100.0,
		DrawProbability: 0.02,
	}
}

// DefaultFFAConfig returns the TrueSkill config tuned for HexDek's
// 4-player Commander pod context. The only divergence from
// [DefaultConfig] is β: we widen it from σ/2 to σ·0.6 (see
// ffaBetaScale's documentation for the FFA-specific noise sources that
// motivate this). τ and draw probability are unchanged — deck skill is
// (in self-play) static, and decisive games dominate over genuine
// multi-way draws. NewTrueSkillRatings uses this preset by default.
func DefaultFFAConfig() Config {
	c := DefaultConfig()
	c.Beta = defaultSigma * ffaBetaScale
	return c
}

// normPDF is the standard normal probability density function.
func normPDF(x float64) float64 {
	return math.Exp(-x*x/2) / math.Sqrt(2*math.Pi)
}

// normCDF is the standard normal cumulative distribution function.
func normCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

// vWin is the truncated Gaussian correction for a decisive (non-draw) result.
func vWin(t, epsilon float64) float64 {
	denom := normCDF(t - epsilon)
	if denom < 1e-15 {
		return -t + epsilon
	}
	return normPDF(t-epsilon) / denom
}

// wWin is the truncated Gaussian precision correction for a decisive result.
func wWin(t, epsilon float64) float64 {
	v := vWin(t, epsilon)
	return v * (v + t - epsilon)
}

// drawMargin computes the epsilon threshold from draw probability.
func drawMargin(drawProb, beta float64) float64 {
	return math.Sqrt(2) * beta * inverseNormCDF((1+drawProb)/2)
}

// inverseNormCDF is the quantile function (inverse CDF) of the standard normal.
// Uses the rational approximation from Abramowitz & Stegun.
func inverseNormCDF(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}
	if p < 0.5 {
		return -rationalApprox(math.Sqrt(-2 * math.Log(p)))
	}
	return rationalApprox(math.Sqrt(-2 * math.Log(1-p)))
}

func rationalApprox(t float64) float64 {
	c0, c1, c2 := 2.515517, 0.802853, 0.010328
	d1, d2, d3 := 1.432788, 0.189269, 0.001308
	return t - (c0+c1*t+c2*t*t)/(1+d1*t+d2*t*t+d3*t*t*t)
}

// Update2Player runs a single TrueSkill update for a decisive (non-draw)
// head-to-head result. Returns the updated winner and loser ratings.
func Update2Player(cfg Config, winner, loser Rating) (Rating, Rating) {
	sigmaW2 := winner.Sigma*winner.Sigma + cfg.Tau*cfg.Tau
	sigmaL2 := loser.Sigma*loser.Sigma + cfg.Tau*cfg.Tau

	c := math.Sqrt(2*cfg.Beta*cfg.Beta + sigmaW2 + sigmaL2)
	t := (winner.Mu - loser.Mu) / c
	epsilon := drawMargin(cfg.DrawProbability, cfg.Beta)

	v := vWin(t, epsilon/c)
	w := wWin(t, epsilon/c)

	wMu := winner.Mu + (sigmaW2/c)*v
	lMu := loser.Mu - (sigmaL2/c)*v

	wSigma := math.Sqrt(math.Max(sigmaW2*(1-(sigmaW2/(c*c))*w), 1e-6))
	lSigma := math.Sqrt(math.Max(sigmaL2*(1-(sigmaL2/(c*c))*w), 1e-6))

	return Rating{Mu: wMu, Sigma: wSigma}, Rating{Mu: lMu, Sigma: lSigma}
}

// UpdateDraw runs a TrueSkill update for a drawn result between two players.
func UpdateDraw(cfg Config, a, b Rating) (Rating, Rating) {
	sigmaA2 := a.Sigma*a.Sigma + cfg.Tau*cfg.Tau
	sigmaB2 := b.Sigma*b.Sigma + cfg.Tau*cfg.Tau

	c := math.Sqrt(2*cfg.Beta*cfg.Beta + sigmaA2 + sigmaB2)
	t := (a.Mu - b.Mu) / c
	epsilon := drawMargin(cfg.DrawProbability, cfg.Beta)

	// Draw v/w functions: the truncation is two-sided.
	absT := math.Abs(t)
	epsC := epsilon / c
	denomHi := normCDF(epsC - absT)
	denomLo := normCDF(-epsC - absT)
	denom := denomHi - denomLo
	if denom < 1e-15 {
		return a, b
	}
	v := (normPDF(absT-epsC) - normPDF(absT+epsC)) / denom
	if t < 0 {
		v = -v
	}
	w := (normPDF(absT-epsC)*(absT-epsC) + normPDF(absT+epsC)*(absT+epsC)) / denom

	aMu := a.Mu + (sigmaA2/c)*v
	bMu := b.Mu - (sigmaB2/c)*v
	aSigma := math.Sqrt(math.Max(sigmaA2*(1-(sigmaA2/(c*c))*w), 1e-6))
	bSigma := math.Sqrt(math.Max(sigmaB2*(1-(sigmaB2/(c*c))*w), 1e-6))

	return Rating{Mu: aMu, Sigma: aSigma}, Rating{Mu: bMu, Sigma: bSigma}
}

// UpdateMultiplayer processes a multiplayer free-for-all game result using
// pairwise decomposition. ranks[i] is the finishing position of player i
// (0 = winner, 1 = second, etc). Returns updated ratings in the same order.
func UpdateMultiplayer(cfg Config, ratings []Rating, ranks []int) []Rating {
	n := len(ratings)
	if n < 2 || len(ranks) != n {
		out := make([]Rating, n)
		copy(out, ratings)
		return out
	}

	// Build sorted order by rank (best to worst).
	type indexed struct {
		origIdx int
		rank    int
		rating  Rating
	}
	sorted := make([]indexed, n)
	for i := range ratings {
		sorted[i] = indexed{origIdx: i, rank: ranks[i], rating: ratings[i]}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].rank < sorted[j].rank
	})

	// Accumulate mu/sigma deltas from pairwise updates.
	muDelta := make([]float64, n)
	sigmaNew := make([]float64, n)
	for i := range sigmaNew {
		sigmaNew[i] = math.Sqrt(ratings[i].Sigma*ratings[i].Sigma + cfg.Tau*cfg.Tau)
	}

	for k := 0; k < len(sorted)-1; k++ {
		wi := sorted[k].origIdx
		li := sorted[k+1].origIdx

		w := Rating{Mu: ratings[wi].Mu + muDelta[wi], Sigma: sigmaNew[wi]}
		l := Rating{Mu: ratings[li].Mu + muDelta[li], Sigma: sigmaNew[li]}

		var wNew, lNew Rating
		if sorted[k].rank == sorted[k+1].rank {
			wNew, lNew = UpdateDraw(cfg, w, l)
		} else {
			wNew, lNew = update2PlayerRaw(cfg, w, l)
		}

		muDelta[wi] += wNew.Mu - w.Mu
		muDelta[li] += lNew.Mu - l.Mu
		sigmaNew[wi] = wNew.Sigma
		sigmaNew[li] = lNew.Sigma
	}

	out := make([]Rating, n)
	for i := range ratings {
		out[i] = Rating{
			Mu:    ratings[i].Mu + muDelta[i],
			Sigma: sigmaNew[i],
		}
	}
	return out
}

// update2PlayerRaw is the core 2-player update without dynamics factor
// (caller pre-inflates sigma). Used internally by UpdateMultiplayer.
func update2PlayerRaw(cfg Config, winner, loser Rating) (Rating, Rating) {
	sigmaW2 := winner.Sigma * winner.Sigma
	sigmaL2 := loser.Sigma * loser.Sigma

	c := math.Sqrt(2*cfg.Beta*cfg.Beta + sigmaW2 + sigmaL2)
	t := (winner.Mu - loser.Mu) / c
	epsilon := drawMargin(cfg.DrawProbability, cfg.Beta)

	v := vWin(t, epsilon/c)
	w := wWin(t, epsilon/c)

	wMu := winner.Mu + (sigmaW2/c)*v
	lMu := loser.Mu - (sigmaL2/c)*v

	wSigma := math.Sqrt(math.Max(sigmaW2*(1-(sigmaW2/(c*c))*w), 1e-6))
	lSigma := math.Sqrt(math.Max(sigmaL2*(1-(sigmaL2/(c*c))*w), 1e-6))

	return Rating{Mu: wMu, Sigma: wSigma}, Rating{Mu: lMu, Sigma: lSigma}
}

// InheritRating creates a new rating for a deck version derived from a parent.
// The mu carries over; sigma is inflated proportional to the card delta.
func InheritRating(parent Rating, cardDelta int) Rating {
	inflation := math.Min(float64(cardDelta)*0.5, defaultSigma/2)
	return Rating{
		Mu:    parent.Mu,
		Sigma: math.Min(parent.Sigma+inflation, defaultSigma),
	}
}

// TrueSkillRatings tracks per-commander TrueSkill ratings across a tournament.
// Mirrors the ELORatings API for drop-in integration.
//
// History is an append-only per-name log of every Update the player took
// part in. It's the data source for DetectDrift (smurf / sandbagging /
// rapid-skill-shift detection). The append happens unconditionally in
// Update; callers who don't need drift detection can ignore it (the per-
// game memory cost is one RatingDelta struct per participant — ~64
// bytes, negligible at tournament scales).
type TrueSkillRatings struct {
	Ratings map[string]Rating
	Games   map[string]int
	History map[string][]RatingDelta
	cfg     Config
}

// NewTrueSkillRatings constructs a fresh ratings tracker for the named
// participants, configured with [DefaultFFAConfig] (HexDek's 4-player
// pod preset). Use [NewTrueSkillRatingsWithConfig] if you need a custom
// config (e.g. a 1v1 dueling tournament).
func NewTrueSkillRatings(names []string) *TrueSkillRatings {
	return NewTrueSkillRatingsWithConfig(names, DefaultFFAConfig())
}

// NewTrueSkillRatingsWithConfig is the explicit-config constructor. The
// no-arg form above wraps this with the FFA preset.
func NewTrueSkillRatingsWithConfig(names []string, cfg Config) *TrueSkillRatings {
	ts := &TrueSkillRatings{
		Ratings: make(map[string]Rating, len(names)),
		Games:   make(map[string]int, len(names)),
		History: make(map[string][]RatingDelta, len(names)),
		cfg:     cfg,
	}
	for _, n := range names {
		ts.Ratings[n] = DefaultRating()
	}
	return ts
}

// Update processes a single multiplayer game. participantNames are in seat
// order; ranks[i] is the finishing position of participant i (0=winner).
// If all ranks are equal, it's treated as a draw.
//
// As a side effect, one RatingDelta is appended to History[name] for
// each participant — the data DetectDrift consumes. Pre-allocate History
// at construction time (NewTrueSkillRatingsWithConfig does this) so this
// is allocation-free for hot tournament loops.
func (ts *TrueSkillRatings) Update(participantNames []string, ranks []int) {
	if len(participantNames) < 2 || len(ranks) != len(participantNames) {
		return
	}
	if ts.History == nil {
		// Defensive: cope with TrueSkillRatings constructed via struct
		// literal (some legacy serialization paths) rather than through
		// the constructor.
		ts.History = make(map[string][]RatingDelta, len(participantNames))
	}

	before := make([]Rating, len(participantNames))
	for i, name := range participantNames {
		before[i] = ts.Ratings[name]
		ts.Games[name]++
	}

	updated := UpdateMultiplayer(ts.cfg, before, ranks)

	for i, name := range participantNames {
		ts.Ratings[name] = updated[i]
		ts.History[name] = append(ts.History[name], RatingDelta{
			Game:        ts.Games[name],
			MuBefore:    before[i].Mu,
			MuAfter:     updated[i].Mu,
			SigmaBefore: before[i].Sigma,
			SigmaAfter:  updated[i].Sigma,
			Rank:        ranks[i],
		})
	}
}

type TrueSkillEntry struct {
	Commander    string
	Mu           float64
	Sigma        float64
	Conservative float64
	Games        int
}

func (ts *TrueSkillRatings) Snapshot() []TrueSkillEntry {
	entries := make([]TrueSkillEntry, 0, len(ts.Ratings))
	for name, r := range ts.Ratings {
		entries = append(entries, TrueSkillEntry{
			Commander:    name,
			Mu:           r.Mu,
			Sigma:        r.Sigma,
			Conservative: r.Conservative(),
			Games:        ts.Games[name],
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Conservative > entries[j].Conservative
	})
	return entries
}
