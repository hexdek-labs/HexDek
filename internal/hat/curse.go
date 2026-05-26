package hat

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CurseDNA holds the evolvable personality parameters for a single deck.
type CurseDNA struct {
	DeckKey         string  `json:"deck_key"`
	Generation      int     `json:"generation"`
	GamesPlayed     int     `json:"games_played"`
	Fitness         float64 `json:"fitness"`

	Aggression      float64 `json:"aggression"`        // attack threshold [0,1]
	ComboPat        float64 `json:"combo_patience"`     // how long to wait for full combo [0,1]
	ThreatParanoia  float64 `json:"threat_paranoia"`    // opponent threat weighting [0,1]
	ResourceGreed   float64 `json:"resource_greed"`     // card advantage vs tempo [0,1]
	PoliticalMemory float64 `json:"political_memory"`   // grudge/gratitude decay rate [0,1]
	DrainAffinity   float64 `json:"drain_affinity"`     // aristocrats drain engine weighting [0,1]
	ArtifactAffinity float64 `json:"artifact_affinity"` // artifact synergy weighting [0,1]

	// New axes (handler-coverage-3 / curse-expansion).
	LandGreed             float64 `json:"land_greed"`              // ramp/lands vs board dev [0,1]
	EquipmentAffinity     float64 `json:"equipment_affinity"`      // Voltron equip routing [0,1]
	GraveyardExploitation float64 `json:"graveyard_exploitation"`  // recursion/reanimator bias [0,1]
	CounterplayTiming     float64 `json:"counterplay_timing"`      // hold interaction vs slam threats [0,1]
	TokenPressure         float64 `json:"token_pressure"`          // go-wide vs go-tall [0,1]
}

// NumCurseAxes is the number of evolvable [0,1] personality axes on a
// CurseDNA. The axisIndexes() method below returns them in the same
// order as CurseTraitKeys, and CurseAxisStats arrays are sized to
// NumCurseAxes so per-axis EMA correlations can be computed per-deck.
const NumCurseAxes = 12

const (
	CursePopSize    = 8
	CurseEvolveAt   = 100  // games per deck before evolution step
	CurseMutationSD = 0.05 // gaussian mutation standard deviation
	CurseKillCount  = 2    // bottom N killed per evolution
	curseImmigrationInterval = 10 // every N generations, inject fresh random DNA
	dimStatsAlpha    = 0.04 // EMA decay for dimension stats (~25-game half-life)
	dimStatsMinN     = 20   // minimum games before applying corrections
	curseCrossoverRate = 0.5 // per-gene probability of taking from parent B during crossover
)

// DimensionStats tracks EMA correlations between evaluator dimension scores
// and game outcomes. After enough games, WeightCorrections returns per-dimension
// multiplicative adjustments to bias the evaluator toward dimensions that
// actually predict wins for this deck.
type DimensionStats struct {
	MeanDim  [NumDimensions]float64 `json:"mean_dim"`
	MeanSqDim [NumDimensions]float64 `json:"mean_sq_dim"`
	CrossDim [NumDimensions]float64 `json:"cross_dim"`
	MeanScore float64               `json:"mean_score"`
	MeanSqScore float64             `json:"mean_sq_score"`
	N         int                    `json:"n"`
}

// RecordGame updates the running EMA statistics with one game's data.
func (ds *DimensionStats) RecordGame(dimMeans [NumDimensions]float64, outcome float64) {
	ds.N++
	alpha := dimStatsAlpha
	if ds.N <= 1 {
		for d := 0; d < NumDimensions; d++ {
			ds.MeanDim[d] = dimMeans[d]
			ds.MeanSqDim[d] = dimMeans[d] * dimMeans[d]
			ds.CrossDim[d] = dimMeans[d] * outcome
		}
		ds.MeanScore = outcome
		ds.MeanSqScore = outcome * outcome
		return
	}
	for d := 0; d < NumDimensions; d++ {
		ds.MeanDim[d] += alpha * (dimMeans[d] - ds.MeanDim[d])
		ds.MeanSqDim[d] += alpha * (dimMeans[d]*dimMeans[d] - ds.MeanSqDim[d])
		ds.CrossDim[d] += alpha * (dimMeans[d]*outcome - ds.CrossDim[d])
	}
	ds.MeanScore += alpha * (outcome - ds.MeanScore)
	ds.MeanSqScore += alpha * (outcome*outcome - ds.MeanSqScore)
}

// Correlation returns Pearson's r for dimension d versus game outcome.
func (ds *DimensionStats) Correlation(d int) float64 {
	if ds.N < dimStatsMinN || d < 0 || d >= NumDimensions {
		return 0
	}
	varD := ds.MeanSqDim[d] - ds.MeanDim[d]*ds.MeanDim[d]
	varO := ds.MeanSqScore - ds.MeanScore*ds.MeanScore
	if varD < 1e-10 || varO < 1e-10 {
		return 0
	}
	cov := ds.CrossDim[d] - ds.MeanDim[d]*ds.MeanScore
	r := cov / math.Sqrt(varD*varO)
	if r < -1 {
		r = -1
	}
	if r > 1 {
		r = 1
	}
	return r
}

// WeightCorrections returns per-dimension multiplicative corrections
// derived from outcome correlations. Positive correlation → boost weight,
// negative → suppress. Max swing: ±20%.
func (ds *DimensionStats) WeightCorrections() [NumDimensions]float64 {
	var corr [NumDimensions]float64
	for d := 0; d < NumDimensions; d++ {
		corr[d] = 1.0
		if ds.N >= dimStatsMinN {
			corr[d] = 1.0 + ds.Correlation(d)*0.20
		}
	}
	return corr
}

// CurseAxisStats is the per-axis analogue of DimensionStats: it tracks
// the EMA correlation between each personality axis value (as it was
// played in a game) and the resulting placement score, so the genetic
// algorithm can adapt mutation pressure per-axis. Axes with strong
// positive correlation get reduced mutation SD (lock in good values);
// axes with strong negative correlation get boosted SD (encourage
// escape from a local minimum).
type CurseAxisStats struct {
	MeanAxis    [NumCurseAxes]float64 `json:"mean_axis"`
	MeanSqAxis  [NumCurseAxes]float64 `json:"mean_sq_axis"`
	CrossAxis   [NumCurseAxes]float64 `json:"cross_axis"`
	MeanScore   float64               `json:"mean_score"`
	MeanSqScore float64               `json:"mean_sq_score"`
	N           int                   `json:"n"`
}

// RecordGame updates the running EMA with one (axis-vector, outcome) pair.
func (as *CurseAxisStats) RecordGame(axes [NumCurseAxes]float64, outcome float64) {
	as.N++
	if as.N <= 1 {
		for i := 0; i < NumCurseAxes; i++ {
			as.MeanAxis[i] = axes[i]
			as.MeanSqAxis[i] = axes[i] * axes[i]
			as.CrossAxis[i] = axes[i] * outcome
		}
		as.MeanScore = outcome
		as.MeanSqScore = outcome * outcome
		return
	}
	alpha := dimStatsAlpha
	for i := 0; i < NumCurseAxes; i++ {
		as.MeanAxis[i] += alpha * (axes[i] - as.MeanAxis[i])
		as.MeanSqAxis[i] += alpha * (axes[i]*axes[i] - as.MeanSqAxis[i])
		as.CrossAxis[i] += alpha * (axes[i]*outcome - as.CrossAxis[i])
	}
	as.MeanScore += alpha * (outcome - as.MeanScore)
	as.MeanSqScore += alpha * (outcome*outcome - as.MeanSqScore)
}

// Correlation returns Pearson's r for axis i versus outcome.
func (as *CurseAxisStats) Correlation(i int) float64 {
	if as.N < dimStatsMinN || i < 0 || i >= NumCurseAxes {
		return 0
	}
	varA := as.MeanSqAxis[i] - as.MeanAxis[i]*as.MeanAxis[i]
	varO := as.MeanSqScore - as.MeanScore*as.MeanScore
	if varA < 1e-10 || varO < 1e-10 {
		return 0
	}
	cov := as.CrossAxis[i] - as.MeanAxis[i]*as.MeanScore
	r := cov / math.Sqrt(varA*varO)
	if r < -1 {
		r = -1
	}
	if r > 1 {
		r = 1
	}
	return r
}

// MutationScale returns the per-axis mutation SD multiplier. Strong
// positive correlation shrinks SD toward 0.5× (lock in winning genes);
// strong negative correlation widens SD toward 1.5× (escape losing
// region). Returns 1.0 below the minimum sample threshold.
func (as *CurseAxisStats) MutationScale(i int) float64 {
	if as.N < dimStatsMinN {
		return 1.0
	}
	r := as.Correlation(i)
	// r in [-1, 1]; map to scale in [0.5, 1.5] with negative correlation
	// widening and positive correlation narrowing.
	return 1.0 - r*0.5
}

// axes packs a CurseDNA's evolvable personality axes into a fixed-size
// vector in the same order as CurseTraitKeys, suitable for CurseAxisStats.
func (d *CurseDNA) axes() [NumCurseAxes]float64 {
	return [NumCurseAxes]float64{
		d.Aggression,
		d.ComboPat,
		d.ThreatParanoia,
		d.ResourceGreed,
		d.PoliticalMemory,
		d.DrainAffinity,
		d.ArtifactAffinity,
		d.LandGreed,
		d.EquipmentAffinity,
		d.GraveyardExploitation,
		d.CounterplayTiming,
		d.TokenPressure,
	}
}

// CursePool maintains a population of DNA variants for a single deck.
type CursePool struct {
	DeckKey    string                      `json:"deck_key"`
	Population [CursePopSize]CurseDNA    `json:"population"`
	GameCount  int                         `json:"game_count"`
	TotalGames int                         `json:"total_games"`
	Bracket    int                         `json:"bracket"`    // power bracket (1-5) for fitness normalization
	GenCount   int                         `json:"gen_count"`  // total generations evolved
	DimStats   DimensionStats              `json:"dim_stats"`  // T3.2 outcome-correlated dimension learning
	AxisStats  CurseAxisStats              `json:"axis_stats"` // per-axis outcome correlation for adaptive mutation

	// Constraints is the deck owner's curse override map: trait key →
	// target value in [0,1]. When set, the genetic algorithm clamps the
	// trait to within ±curseConstraintBand of the target after every
	// mutation/immigration so evolution settles around fixed anchors.
	// Valid keys are listed in CurseTraitKeys.
	Constraints map[string]float64 `json:"constraints,omitempty"`

	rng        *rand.Rand // not serialized; caller injects via InitPool or SetRNG
}

// CurseTraitKeys is the set of trait names accepted as constraint keys.
// Order matches CurseDNA.axes() and the radar-axis layout in the frontend.
var CurseTraitKeys = []string{
	"aggression",
	"combo_patience",
	"threat_paranoia",
	"resource_greed",
	"political_memory",
	"drain_affinity",
	"artifact_affinity",
	"land_greed",
	"equipment_affinity",
	"graveyard_exploitation",
	"counterplay_timing",
	"token_pressure",
}

// curseConstraintBand is the half-width of the band around a constraint
// target that mutated trait values are clamped into.
const curseConstraintBand = 0.1

// IsValidCurseTrait reports whether key is a recognized constraint trait.
func IsValidCurseTrait(key string) bool {
	for _, k := range CurseTraitKeys {
		if k == key {
			return true
		}
	}
	return false
}

// SetRNG assigns the random source used for selection and evolution.
// Must be called after deserializing a pool from JSON.
func (pool *CursePool) SetRNG(rng *rand.Rand) {
	pool.rng = rng
}

// SelectForGame picks a DNA variant using fitness-proportional selection.
func (pool *CursePool) SelectForGame() (dna *CurseDNA, idx int) {
	totalFitness := 0.0
	for i := range pool.Population {
		f := pool.Population[i].Fitness
		if f < 0.01 {
			f = 0.01 // floor to prevent starvation
		}
		totalFitness += f
	}
	roll := pool.rng.Float64() * totalFitness
	cumulative := 0.0
	for i := range pool.Population {
		f := pool.Population[i].Fitness
		if f < 0.01 {
			f = 0.01
		}
		cumulative += f
		if roll <= cumulative {
			return &pool.Population[i], i
		}
	}
	return &pool.Population[CursePopSize-1], CursePopSize - 1
}

// PlacementScore converts a finish position to a fitness value in [0,1].
// For 4-player Commander (the canonical pod size), the scale rewards
// survival to top-2 disproportionately to acknowledge that "I survived
// to the 1v1 final" is a meaningful skill the binary W/L view obscures:
//
//	1st (won)            → 1.0
//	2nd (runner-up)      → 0.85   (small gap from 1st — survival rewarded)
//	3rd                  → 0.45   (still positive credit for not being knocked out first)
//	4th (knocked out)    → 0.0
//
// Design rationale: with the previous linear scale (1.0 / 0.667 / 0.333 / 0.0),
// the mean per-game score across 4 random placements was 0.500. The new
// scale shifts that mean to 0.575, slightly inflating the credit a typical
// game contributes to a deck's TrueSkill update. The asymmetry favors
// survival-oriented archetypes (control / politics / monarchy / wither walls)
// over glass-cannon archetypes (combo-or-fizzle) — a deliberate design
// statement that surviving the chaos is worth more than the linear default
// implies.
//
// For other player counts (rare in EDH, possible in skirmishes), falls
// back to the linear scale: 1 - (placement-1)/(totalPlayers-1). This
// keeps the function correct for 2-player (1.0 / 0.0 binary) and arbitrary
// N without needing per-N tuning.
//
// Reduces noise vs binary win/loss (a 3rd-place deck still gets 0.45,
// not 0.0, so a single unlucky game doesn't drag rating as hard as binary).
func PlacementScore(placement, totalPlayers int) float64 {
	if totalPlayers <= 1 || placement <= 0 {
		return 0.5
	}
	// 4-player Commander — canonical pod size, survival-weighted scale.
	if totalPlayers == 4 {
		switch placement {
		case 1:
			return 1.00
		case 2:
			return 0.85
		case 3:
			return 0.45
		case 4:
			return 0.00
		default:
			return 0.0 // out-of-range placements clamp to last
		}
	}
	// Other player counts — linear fallback.
	return 1.0 - float64(placement-1)/float64(totalPlayers-1)
}

// RecordResult updates fitness after a game and possibly triggers evolution.
// score is a [0,1] fitness value — use PlacementScore() for 4-player games
// instead of binary 0/1 to reduce variance.
func (pool *CursePool) RecordResult(idx int, score float64) {
	dna := &pool.Population[idx]
	dna.GamesPlayed++
	pool.GameCount++
	pool.TotalGames++

	// Bracket-normalize: compare against expected performance for this bracket.
	normalized := score
	if pool.Bracket > 0 {
		expected := expectedBracketScore(pool.Bracket)
		if expected > 0 {
			normalized = score / expected
			if normalized > 2.0 {
				normalized = 2.0
			}
		}
	}

	// Rolling average fitness (EMA).
	if dna.GamesPlayed <= 1 {
		dna.Fitness = normalized
	} else {
		alpha := 2.0 / (float64(min(dna.GamesPlayed, 50)) + 1.0)
		dna.Fitness = dna.Fitness*(1-alpha) + normalized*alpha
	}

	// Per-axis outcome-correlation EMA. Records the axis vector as it
	// was played (i.e. the DNA's current values) against the raw [0,1]
	// placement score, so axis correlations stay comparable across
	// brackets even when fitness is bracket-normalized.
	pool.AxisStats.RecordGame(dna.axes(), score)

	if pool.GameCount >= CurseEvolveAt {
		pool.evolve()
	}
}

// expectedBracketScore returns the expected average placement score for a
// bracket tier. Derived from grinder data: B1 WR≈28.9%, B5 WR≈19.3%.
func expectedBracketScore(bracket int) float64 {
	switch bracket {
	case 1:
		return 0.58
	case 2:
		return 0.52
	case 3:
		return 0.48
	case 4:
		return 0.44
	case 5:
		return 0.39
	default:
		return 0.50
	}
}

// crossover performs uniform single-axis crossover from parents a and b
// into a fresh child. Each gene independently is taken from b with
// probability curseCrossoverRate, otherwise from a. Non-evolvable
// metadata (DeckKey, Generation tracking) comes from a.
func (pool *CursePool) crossover(a, b *CurseDNA) CurseDNA {
	pick := func(va, vb float64) float64 {
		if pool.rng.Float64() < curseCrossoverRate {
			return vb
		}
		return va
	}
	return CurseDNA{
		DeckKey:               a.DeckKey,
		Generation:            a.Generation,
		Aggression:            pick(a.Aggression, b.Aggression),
		ComboPat:              pick(a.ComboPat, b.ComboPat),
		ThreatParanoia:        pick(a.ThreatParanoia, b.ThreatParanoia),
		ResourceGreed:         pick(a.ResourceGreed, b.ResourceGreed),
		PoliticalMemory:       pick(a.PoliticalMemory, b.PoliticalMemory),
		DrainAffinity:         pick(a.DrainAffinity, b.DrainAffinity),
		ArtifactAffinity:      pick(a.ArtifactAffinity, b.ArtifactAffinity),
		LandGreed:             pick(a.LandGreed, b.LandGreed),
		EquipmentAffinity:     pick(a.EquipmentAffinity, b.EquipmentAffinity),
		GraveyardExploitation: pick(a.GraveyardExploitation, b.GraveyardExploitation),
		CounterplayTiming:     pick(a.CounterplayTiming, b.CounterplayTiming),
		TokenPressure:         pick(a.TokenPressure, b.TokenPressure),
	}
}

// evolve runs one generation of genetic selection on the population.
// Sort by fitness, kill bottom CurseKillCount, fill those slots with
// children produced by uniform crossover of two top performers, mutate
// the children (per-axis SD scaled by AxisStats correlation), clamp,
// reset game counter. Every curseImmigrationInterval generations,
// inject fresh random DNA to prevent convergent collapse.
func (pool *CursePool) evolve() {
	pool.GenCount++

	// Build index sorted by fitness (ascending).
	indices := make([]int, CursePopSize)
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(a, b int) bool {
		return pool.Population[indices[a]].Fitness < pool.Population[indices[b]].Fitness
	})

	// Per-axis mutation SD multipliers (driven by AxisStats correlations).
	// Strong positive corr → smaller SD, strong negative → larger SD.
	scales := [NumCurseAxes]float64{}
	for i := 0; i < NumCurseAxes; i++ {
		scales[i] = pool.AxisStats.MutationScale(i)
	}
	mutate := func(v float64, axis int) float64 {
		return clampUnit(v + pool.rng.NormFloat64()*CurseMutationSD*scales[axis])
	}

	// Kill bottom, fill with crossover children of two top performers.
	for k := 0; k < CurseKillCount; k++ {
		loserIdx := indices[k]
		parentAIdx := indices[CursePopSize-1-k]
		parentBIdx := indices[CursePopSize-1-((k+1)%CurseKillCount)]
		if parentBIdx == parentAIdx {
			parentBIdx = indices[CursePopSize-1]
		}

		child := pool.crossover(&pool.Population[parentAIdx], &pool.Population[parentBIdx])
		child.GamesPlayed = 0
		child.Fitness = 0.25
		child.Generation++

		child.Aggression = mutate(child.Aggression, 0)
		child.ComboPat = mutate(child.ComboPat, 1)
		child.ThreatParanoia = mutate(child.ThreatParanoia, 2)
		child.ResourceGreed = mutate(child.ResourceGreed, 3)
		child.PoliticalMemory = mutate(child.PoliticalMemory, 4)
		child.DrainAffinity = mutate(child.DrainAffinity, 5)
		child.ArtifactAffinity = mutate(child.ArtifactAffinity, 6)
		child.LandGreed = mutate(child.LandGreed, 7)
		child.EquipmentAffinity = mutate(child.EquipmentAffinity, 8)
		child.GraveyardExploitation = mutate(child.GraveyardExploitation, 9)
		child.CounterplayTiming = mutate(child.CounterplayTiming, 10)
		child.TokenPressure = mutate(child.TokenPressure, 11)

		// Snap any owner-locked trait back into its constraint band.
		pool.applyConstraintsToDNA(&child)

		pool.Population[loserIdx] = child
	}

	// Immigration: every N generations, replace one mid-tier individual
	// with fresh random DNA. Prevents convergent collapse.
	if pool.GenCount%curseImmigrationInterval == 0 {
		immigrantIdx := indices[CurseKillCount] // lowest non-killed slot
		immigrant := CurseDNA{
			DeckKey:               pool.DeckKey,
			Aggression:            pool.rng.Float64(),
			ComboPat:              pool.rng.Float64(),
			ThreatParanoia:        pool.rng.Float64(),
			ResourceGreed:         pool.rng.Float64(),
			PoliticalMemory:       pool.rng.Float64(),
			DrainAffinity:         pool.rng.Float64(),
			ArtifactAffinity:      pool.rng.Float64(),
			LandGreed:             pool.rng.Float64(),
			EquipmentAffinity:     pool.rng.Float64(),
			GraveyardExploitation: pool.rng.Float64(),
			CounterplayTiming:     pool.rng.Float64(),
			TokenPressure:         pool.rng.Float64(),
			Fitness:               0.25,
		}
		pool.applyConstraintsToDNA(&immigrant)
		pool.Population[immigrantIdx] = immigrant
	}

	pool.GameCount = 0
}

// InitPool creates a fresh population with random parameters for a new deck.
// If pool.Constraints is non-nil at seed time, locked traits are seeded
// from the constraint value (then clamped into the constraint band) instead
// of randomized. Use InitPoolWithConstraints for that path.
func InitPool(deckKey string, rng *rand.Rand) CursePool {
	return InitPoolWithConstraints(deckKey, rng, nil)
}

// InitPoolWithConstraints creates a fresh population, seeding any locked
// traits from the constraint map instead of random. Constraint keys not
// in CurseTraitKeys are ignored.
func InitPoolWithConstraints(deckKey string, rng *rand.Rand, constraints map[string]float64) CursePool {
	pool := CursePool{DeckKey: deckKey, rng: rng}
	if len(constraints) > 0 {
		pool.Constraints = make(map[string]float64, len(constraints))
		for k, v := range constraints {
			if IsValidCurseTrait(k) {
				pool.Constraints[k] = clampUnit(v)
			}
		}
	}

	seed := func(key string) float64 {
		if t, ok := pool.Constraints[key]; ok {
			return clampUnit(t)
		}
		return rng.Float64()
	}

	for i := range pool.Population {
		pool.Population[i] = CurseDNA{
			DeckKey:               deckKey,
			Aggression:            seed("aggression"),
			ComboPat:              seed("combo_patience"),
			ThreatParanoia:        seed("threat_paranoia"),
			ResourceGreed:         seed("resource_greed"),
			PoliticalMemory:       seed("political_memory"),
			DrainAffinity:         seed("drain_affinity"),
			ArtifactAffinity:      seed("artifact_affinity"),
			LandGreed:             seed("land_greed"),
			EquipmentAffinity:     seed("equipment_affinity"),
			GraveyardExploitation: seed("graveyard_exploitation"),
			CounterplayTiming:     seed("counterplay_timing"),
			TokenPressure:         seed("token_pressure"),
			Fitness:               0.25, // baseline expected winrate in 4-player
		}
		pool.applyConstraintsToDNA(&pool.Population[i])
	}
	return pool
}

// InitPoolWithBracket creates a fresh population with bracket info for normalization.
func InitPoolWithBracket(deckKey string, bracket int, rng *rand.Rand) CursePool {
	pool := InitPool(deckKey, rng)
	pool.Bracket = bracket
	return pool
}

// clampUnit constrains v to [0.0, 1.0].
func clampUnit(v float64) float64 {
	if v < 0.0 {
		return 0.0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}

// clampRange constrains v to [lo, hi].
func clampRange(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// applyConstraintsToDNA clamps each constrained trait on dna into the
// band [target-curseConstraintBand, target+curseConstraintBand], itself
// clipped to [0,1]. Unknown or unconstrained traits are left untouched.
func (pool *CursePool) applyConstraintsToDNA(dna *CurseDNA) {
	if len(pool.Constraints) == 0 {
		return
	}
	clampTo := func(v, target float64) float64 {
		lo := clampUnit(target - curseConstraintBand)
		hi := clampUnit(target + curseConstraintBand)
		return clampRange(v, lo, hi)
	}
	if t, ok := pool.Constraints["aggression"]; ok {
		dna.Aggression = clampTo(dna.Aggression, t)
	}
	if t, ok := pool.Constraints["combo_patience"]; ok {
		dna.ComboPat = clampTo(dna.ComboPat, t)
	}
	if t, ok := pool.Constraints["threat_paranoia"]; ok {
		dna.ThreatParanoia = clampTo(dna.ThreatParanoia, t)
	}
	if t, ok := pool.Constraints["resource_greed"]; ok {
		dna.ResourceGreed = clampTo(dna.ResourceGreed, t)
	}
	if t, ok := pool.Constraints["political_memory"]; ok {
		dna.PoliticalMemory = clampTo(dna.PoliticalMemory, t)
	}
	if t, ok := pool.Constraints["drain_affinity"]; ok {
		dna.DrainAffinity = clampTo(dna.DrainAffinity, t)
	}
	if t, ok := pool.Constraints["artifact_affinity"]; ok {
		dna.ArtifactAffinity = clampTo(dna.ArtifactAffinity, t)
	}
	if t, ok := pool.Constraints["land_greed"]; ok {
		dna.LandGreed = clampTo(dna.LandGreed, t)
	}
	if t, ok := pool.Constraints["equipment_affinity"]; ok {
		dna.EquipmentAffinity = clampTo(dna.EquipmentAffinity, t)
	}
	if t, ok := pool.Constraints["graveyard_exploitation"]; ok {
		dna.GraveyardExploitation = clampTo(dna.GraveyardExploitation, t)
	}
	if t, ok := pool.Constraints["counterplay_timing"]; ok {
		dna.CounterplayTiming = clampTo(dna.CounterplayTiming, t)
	}
	if t, ok := pool.Constraints["token_pressure"]; ok {
		dna.TokenPressure = clampTo(dna.TokenPressure, t)
	}
}

// ApplyConstraintsToAll re-clamps every member of the population to the
// pool's current Constraints map. Use after mutating Constraints (e.g.
// from a PATCH handler) so existing population members snap to the new
// bands without waiting for evolution.
func (pool *CursePool) ApplyConstraintsToAll() {
	for i := range pool.Population {
		pool.applyConstraintsToDNA(&pool.Population[i])
	}
}

// sanitizeKey makes a deck key safe for use as a filename.
func sanitizeKey(key string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return r.Replace(key)
}

// SavePool writes a single pool to disk as JSON.
func SavePool(dir string, pool *CursePool) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("savepool: mkdir %s: %w", dir, err)
	}
	fname := filepath.Join(dir, sanitizeKey(pool.DeckKey)+".json")
	data, err := json.Marshal(pool)
	if err != nil {
		return err
	}
	return os.WriteFile(fname, data, 0644)
}

// LoadPool reads a single pool from disk and restores it.
func LoadPool(dir, deckKey string, rng *rand.Rand) (*CursePool, error) {
	fname := filepath.Join(dir, sanitizeKey(deckKey)+".json")
	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var pool CursePool
	if err := json.Unmarshal(data, &pool); err != nil {
		return nil, err
	}
	pool.rng = rng
	return &pool, nil
}

// SaveAllPools writes every pool to disk.
func SaveAllPools(dir string, pools map[string]*CursePool) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("saveallpools: mkdir %s: %w", dir, err)
	}
	for _, pool := range pools {
		if err := SavePool(dir, pool); err != nil {
			return err
		}
	}
	return nil
}

// LoadAllPools reads all pool files from a directory.
func LoadAllPools(dir string, rng *rand.Rand) (map[string]*CursePool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*CursePool), nil
		}
		return nil, err
	}
	pools := make(map[string]*CursePool, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var pool CursePool
		if err := json.Unmarshal(data, &pool); err != nil {
			continue
		}
		pool.rng = rng
		pools[pool.DeckKey] = &pool
	}
	return pools, nil
}
