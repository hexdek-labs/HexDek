package trueskill

import (
	"math"
	"sort"
)

const (
	defaultMu    = 25.0
	defaultSigma = 25.0 / 3.0
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

func DefaultConfig() Config {
	return Config{
		Beta:            defaultMu / 6.0,
		Tau:             defaultMu / 300.0,
		DrawProbability: 0.02,
	}
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
type TrueSkillRatings struct {
	Ratings map[string]Rating
	Games   map[string]int
	cfg     Config
}

func NewTrueSkillRatings(names []string) *TrueSkillRatings {
	ts := &TrueSkillRatings{
		Ratings: make(map[string]Rating, len(names)),
		Games:   make(map[string]int, len(names)),
		cfg:     DefaultConfig(),
	}
	for _, n := range names {
		ts.Ratings[n] = DefaultRating()
	}
	return ts
}

// Update processes a single multiplayer game. participantNames are in seat
// order; ranks[i] is the finishing position of participant i (0=winner).
// If all ranks are equal, it's treated as a draw.
func (ts *TrueSkillRatings) Update(participantNames []string, ranks []int) {
	if len(participantNames) < 2 || len(ranks) != len(participantNames) {
		return
	}
	for _, name := range participantNames {
		ts.Games[name]++
	}

	ratings := make([]Rating, len(participantNames))
	for i, name := range participantNames {
		ratings[i] = ts.Ratings[name]
	}

	updated := UpdateMultiplayer(ts.cfg, ratings, ranks)
	for i, name := range participantNames {
		ts.Ratings[name] = updated[i]
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
