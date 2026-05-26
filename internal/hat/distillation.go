package hat

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// Genetic→Neural Distillation
//
// The distillation loop creates a feedback cycle between the Curse genetic
// algorithm (cheap per-deck parameter exploration) and the neural evaluator
// (general position understanding):
//
//   1. Harvest: read evolved Curse pools, identify high-fitness DNA members
//   2. Enrich: tag neural training samples with DNA params + fitness weight
//   3. Seed: initialize new Curse pools using neural predictions instead of
//      random params (warm start from generalized knowledge)
//   4. Cycle: periodically trigger distillation when enough high-fitness data
//      accumulates, re-seed underperforming pools
// ---------------------------------------------------------------------------

// DistillationConfig holds paths and tuning parameters for the distillation loop.
type DistillationConfig struct {
	CurseDir       string  // directory containing per-deck curse pool JSON files
	TrainingDir     string  // directory for neural training data
	ModelPath       string  // path to the neural evaluator model.json
	FitnessQuartile float64 // top N% fitness threshold for harvesting (default 0.25 = top 25%)
	MinPoolsForSeed int     // minimum evolved pools needed before seeding works (default 5)
	MinGensForHarvest int   // minimum generations a pool must have before harvest (default 3)
	CycleInterval   time.Duration // minimum time between distillation cycles (default 30min)
	ReseedThreshold float64 // fitness below which a pool is considered underperforming (default 0.35)
}

// DefaultDistillationConfig returns sensible defaults for the distillation system.
func DefaultDistillationConfig(baseDir string) DistillationConfig {
	return DistillationConfig{
		CurseDir:         filepath.Join(baseDir, "data/curse"),
		TrainingDir:       filepath.Join(baseDir, "data/training"),
		ModelPath:         filepath.Join(baseDir, "data/training/model.json"),
		FitnessQuartile:   0.25,
		MinPoolsForSeed:   5,
		MinGensForHarvest: 3,
		CycleInterval:     30 * time.Minute,
		ReseedThreshold:   0.35,
	}
}

// HighFitnessDNA represents a single high-performing DNA member extracted
// from the Curse population, annotated with its source deck metadata.
type HighFitnessDNA struct {
	DNA       CurseDNA `json:"dna"`
	Archetype string    `json:"archetype"`
	Bracket   int       `json:"bracket"`
	GenCount  int       `json:"gen_count"`
}

// DistillationManifest is the output of a harvest pass — a collection of
// high-fitness DNA members grouped by archetype, used to seed the neural
// training pipeline and initialize new pools.
type DistillationManifest struct {
	HarvestTime   time.Time        `json:"harvest_time"`
	TotalPools    int              `json:"total_pools"`
	HarvestedDNA  []HighFitnessDNA `json:"harvested_dna"`
	MeanFitness   float64          `json:"mean_fitness"`
	TopFitness    float64          `json:"top_fitness"`
}

// DNAEnrichedSample extends the PivotEnrichedSample with DNA parameters
// and a fitness-based weight multiplier for the training loss.
type DNAEnrichedSample struct {
	PivotEnrichedSample
	DNAParams    [7]float64 `json:"dna_params"`     // [aggression, combo_pat, threat, greed, political, drain, artifact]
	FitnessWeight float64   `json:"fitness_weight"` // 1.0=baseline, >1.0=high-fitness games get more weight
}

// DistillationManager coordinates the genetic→neural feedback loop.
// It runs non-blocking: the Cycle method is called periodically and spawns
// a goroutine if enough time has passed and data is available.
type DistillationManager struct {
	config    DistillationConfig
	rng       *rand.Rand
	mu        sync.Mutex
	lastCycle int64 // unix timestamp of last completed cycle
	running   int32 // atomic flag
	manifest  *DistillationManifest // latest harvest result (for seeding)
}

// NewDistillationManager creates a new distillation manager.
func NewDistillationManager(cfg DistillationConfig, rng *rand.Rand) *DistillationManager {
	return &DistillationManager{
		config: cfg,
		rng:    rng,
	}
}

// ---------------------------------------------------------------------------
// 1. Distillation Harvester
// ---------------------------------------------------------------------------

// HarvestHighFitness reads all evolved Curse pools from disk, identifies
// DNA members in the top quartile of fitness (across all decks), and returns
// them as a DistillationManifest. Pools with fewer than MinGensForHarvest
// generations are excluded (not enough evolution to be meaningful).
func (dm *DistillationManager) HarvestHighFitness(strategyLookup func(deckKey string) *StrategyProfile) (*DistillationManifest, error) {
	pools, err := LoadAllPools(dm.config.CurseDir, dm.rng)
	if err != nil {
		return nil, err
	}

	// Collect all DNA with enough evolution history.
	type annotatedDNA struct {
		dna       CurseDNA
		archetype string
		bracket   int
		genCount  int
	}
	var candidates []annotatedDNA

	for _, pool := range pools {
		if pool.GenCount < dm.config.MinGensForHarvest {
			continue
		}
		arch := ""
		bracket := pool.Bracket
		if strategyLookup != nil {
			if sp := strategyLookup(pool.DeckKey); sp != nil {
				arch = sp.Archetype
				if sp.Bracket > 0 {
					bracket = sp.Bracket
				}
			}
		}
		for _, dna := range pool.Population {
			if dna.GamesPlayed < 10 {
				continue // not enough games to trust fitness
			}
			candidates = append(candidates, annotatedDNA{
				dna:       dna,
				archetype: arch,
				bracket:   bracket,
				genCount:  pool.GenCount,
			})
		}
	}

	if len(candidates) == 0 {
		return &DistillationManifest{
			HarvestTime: time.Now(),
			TotalPools:  len(pools),
		}, nil
	}

	// Sort by fitness descending to find top quartile.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dna.Fitness > candidates[j].dna.Fitness
	})

	cutoff := int(math.Ceil(float64(len(candidates)) * dm.config.FitnessQuartile))
	if cutoff < 1 {
		cutoff = 1
	}
	topCandidates := candidates[:cutoff]

	harvested := make([]HighFitnessDNA, len(topCandidates))
	totalFitness := 0.0
	topFitness := 0.0
	for i, c := range topCandidates {
		harvested[i] = HighFitnessDNA{
			DNA:       c.dna,
			Archetype: c.archetype,
			Bracket:   c.bracket,
			GenCount:  c.genCount,
		}
		totalFitness += c.dna.Fitness
		if c.dna.Fitness > topFitness {
			topFitness = c.dna.Fitness
		}
	}

	manifest := &DistillationManifest{
		HarvestTime:  time.Now(),
		TotalPools:   len(pools),
		HarvestedDNA: harvested,
		MeanFitness:  totalFitness / float64(len(harvested)),
		TopFitness:   topFitness,
	}

	dm.mu.Lock()
	dm.manifest = manifest
	dm.mu.Unlock()

	return manifest, nil
}

// ---------------------------------------------------------------------------
// 2. DNA→Training Enrichment
// ---------------------------------------------------------------------------

// DNAParamsFromCurse extracts the 7 evolvable parameters as a fixed array.
func DNAParamsFromCurse(dna *CurseDNA) [7]float64 {
	return [7]float64{
		dna.Aggression,
		dna.ComboPat,
		dna.ThreatParanoia,
		dna.ResourceGreed,
		dna.PoliticalMemory,
		dna.DrainAffinity,
		dna.ArtifactAffinity,
	}
}

// FitnessToWeight converts a DNA fitness score into a training loss weight.
// High-fitness DNA games get higher weight (up to 2x), baseline fitness
// (0.5) gets weight 1.0, low fitness gets reduced weight (minimum 0.5).
func FitnessToWeight(fitness float64) float64 {
	// Linear mapping: fitness 0.0 → weight 0.5, fitness 0.5 → weight 1.0, fitness 1.0 → weight 2.0
	w := 0.5 + fitness*1.5
	if w < 0.5 {
		w = 0.5
	}
	if w > 2.0 {
		w = 2.0
	}
	return w
}

// EnrichWithDNA takes pivot-enriched training samples and annotates them
// with the DNA parameters and fitness weight from the Curse that played
// the game. This provides the neural net with signal about which personality
// configurations produce good outcomes.
func EnrichWithDNA(samples []PivotEnrichedSample, dna *CurseDNA) []DNAEnrichedSample {
	params := DNAParamsFromCurse(dna)
	weight := FitnessToWeight(dna.Fitness)

	enriched := make([]DNAEnrichedSample, len(samples))
	for i, s := range samples {
		enriched[i] = DNAEnrichedSample{
			PivotEnrichedSample: s,
			DNAParams:           params,
			FitnessWeight:       weight,
		}
	}
	return enriched
}

// AppendDNAEnrichedSamples writes DNA-enriched samples to a JSONL file.
func AppendDNAEnrichedSamples(path string, samples []DNAEnrichedSample) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("dna-samples: mkdir %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck — append-only JSONL; encode-loop errors propagate via the return below.
	enc := json.NewEncoder(f)
	for _, s := range samples {
		if err := enc.Encode(s); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 3. Neural→Curse Seeding
// ---------------------------------------------------------------------------

// archetypeDNACentroid computes the average DNA parameters for a given
// archetype from the distillation manifest. Returns zero values and false
// if no data is available for the archetype.
func archetypeDNACentroid(manifest *DistillationManifest, archetype string) ([7]float64, bool) {
	var sum [7]float64
	count := 0

	for _, h := range manifest.HarvestedDNA {
		if !strings.EqualFold(h.Archetype, archetype) {
			continue
		}
		params := DNAParamsFromCurse(&h.DNA)
		for i := range sum {
			sum[i] += params[i]
		}
		count++
	}

	if count == 0 {
		return sum, false
	}

	for i := range sum {
		sum[i] /= float64(count)
	}
	return sum, true
}

// bracketDNACentroid computes the average DNA parameters for a given
// bracket level from the distillation manifest.
func bracketDNACentroid(manifest *DistillationManifest, bracket int) ([7]float64, bool) {
	var sum [7]float64
	count := 0

	for _, h := range manifest.HarvestedDNA {
		if h.Bracket != bracket {
			continue
		}
		params := DNAParamsFromCurse(&h.DNA)
		for i := range sum {
			sum[i] += params[i]
		}
		count++
	}

	if count == 0 {
		return sum, false
	}

	for i := range sum {
		sum[i] /= float64(count)
	}
	return sum, true
}

// SeedDNAFromManifest predicts good starting DNA parameters for a new deck
// based on its Freya profile (archetype, bracket, commander themes). Instead
// of random [0,1] initialization, it uses the distilled knowledge from
// high-fitness evolved pools to provide a warm start.
//
// Priority: archetype match > bracket match > global mean > random fallback.
func (dm *DistillationManager) SeedDNAFromManifest(archetype string, bracket int) [7]float64 {
	dm.mu.Lock()
	manifest := dm.manifest
	dm.mu.Unlock()

	if manifest == nil || len(manifest.HarvestedDNA) == 0 {
		// No distillation data available — return midpoint initialization.
		return [7]float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}
	}

	// Try archetype-specific centroid first (most informative).
	if archetype != "" {
		if centroid, ok := archetypeDNACentroid(manifest, archetype); ok {
			return centroid
		}
	}

	// Fall back to bracket-specific centroid.
	if bracket > 0 {
		if centroid, ok := bracketDNACentroid(manifest, bracket); ok {
			return centroid
		}
	}

	// Global centroid across all high-fitness DNA.
	var sum [7]float64
	for _, h := range manifest.HarvestedDNA {
		params := DNAParamsFromCurse(&h.DNA)
		for i := range sum {
			sum[i] += params[i]
		}
	}
	for i := range sum {
		sum[i] /= float64(len(manifest.HarvestedDNA))
	}
	return sum
}

// InitPoolSeeded creates a new Curse pool with warm-started DNA parameters
// derived from distilled high-fitness populations. The population is
// initialized around the predicted centroid with slight gaussian noise to
// maintain diversity.
func (dm *DistillationManager) InitPoolSeeded(deckKey string, archetype string, bracket int, rng *rand.Rand) CursePool {
	centroid := dm.SeedDNAFromManifest(archetype, bracket)

	pool := CursePool{DeckKey: deckKey, Bracket: bracket, rng: rng}
	for i := range pool.Population {
		pool.Population[i] = CurseDNA{
			DeckKey:               deckKey,
			Aggression:            clampUnit(centroid[0] + rng.NormFloat64()*0.1),
			ComboPat:              clampUnit(centroid[1] + rng.NormFloat64()*0.1),
			ThreatParanoia:        clampUnit(centroid[2] + rng.NormFloat64()*0.1),
			ResourceGreed:         clampUnit(centroid[3] + rng.NormFloat64()*0.1),
			PoliticalMemory:       clampUnit(centroid[4] + rng.NormFloat64()*0.1),
			DrainAffinity:         clampUnit(centroid[5] + rng.NormFloat64()*0.1),
			ArtifactAffinity:      clampUnit(centroid[6] + rng.NormFloat64()*0.1),
			// Centroid is sized for the original 7 axes; randomize the
			// expansion axes so seeded pools don't ship at axis=0.
			LandGreed:             rng.Float64(),
			EquipmentAffinity:     rng.Float64(),
			GraveyardExploitation: rng.Float64(),
			CounterplayTiming:     rng.Float64(),
			TokenPressure:         rng.Float64(),
			Fitness:               0.25,
		}
	}
	return pool
}

// ---------------------------------------------------------------------------
// 4. Distillation Cycle
// ---------------------------------------------------------------------------

// TryCycle checks whether enough time has passed and data is available to
// run a distillation cycle. If so, it spawns a non-blocking goroutine to:
//   1. Harvest high-fitness DNA from evolved pools
//   2. Write DNA-enriched training data for the neural pipeline
//   3. Re-seed underperforming pools with better starting points
//
// Returns true if a cycle was triggered, false if skipped.
func (dm *DistillationManager) TryCycle(strategyLookup func(string) *StrategyProfile) bool {
	now := time.Now().Unix()
	lastCycle := atomic.LoadInt64(&dm.lastCycle)

	if now-lastCycle < int64(dm.config.CycleInterval.Seconds()) {
		return false
	}

	if !atomic.CompareAndSwapInt32(&dm.running, 0, 1) {
		return false // already running
	}

	go dm.runCycle(strategyLookup)
	return true
}

// runCycle executes the full distillation pass.
func (dm *DistillationManager) runCycle(strategyLookup func(string) *StrategyProfile) {
	defer atomic.StoreInt32(&dm.running, 0)

	t0 := time.Now()
	log.Printf("[distillation] cycle starting")

	// Step 1: Harvest high-fitness DNA.
	manifest, err := dm.HarvestHighFitness(strategyLookup)
	if err != nil {
		log.Printf("[distillation] harvest failed: %v", err)
		return
	}

	if len(manifest.HarvestedDNA) == 0 {
		log.Printf("[distillation] no high-fitness DNA found (pools=%d), skipping cycle",
			manifest.TotalPools)
		atomic.StoreInt64(&dm.lastCycle, time.Now().Unix())
		return
	}

	log.Printf("[distillation] harvested %d high-fitness DNA from %d pools (mean=%.3f, top=%.3f)",
		len(manifest.HarvestedDNA), manifest.TotalPools,
		manifest.MeanFitness, manifest.TopFitness)

	// Step 2: Save manifest for future seeding and training enrichment.
	manifestPath := filepath.Join(dm.config.TrainingDir, "distillation_manifest.json")
	if err := SaveDistillationManifest(manifestPath, manifest); err != nil {
		log.Printf("[distillation] failed to save manifest: %v", err)
	}

	// Step 3: Re-seed underperforming pools.
	reseeded := dm.reseedUnderperformers(strategyLookup)
	if reseeded > 0 {
		log.Printf("[distillation] re-seeded %d underperforming pools", reseeded)
	}

	elapsed := time.Since(t0)
	atomic.StoreInt64(&dm.lastCycle, time.Now().Unix())
	log.Printf("[distillation] cycle completed in %v", elapsed)
}

// reseedUnderperformers finds pools with low average fitness and reinjects
// seeded DNA based on the latest manifest. Returns the number of pools
// reseeded.
func (dm *DistillationManager) reseedUnderperformers(strategyLookup func(string) *StrategyProfile) int {
	pools, err := LoadAllPools(dm.config.CurseDir, dm.rng)
	if err != nil {
		return 0
	}

	dm.mu.Lock()
	manifest := dm.manifest
	dm.mu.Unlock()

	if manifest == nil || len(manifest.HarvestedDNA) < dm.config.MinPoolsForSeed {
		return 0
	}

	reseeded := 0
	for _, pool := range pools {
		if pool.GenCount < dm.config.MinGensForHarvest {
			continue // too young to judge
		}

		// Calculate pool's average fitness.
		avgFitness := 0.0
		for _, dna := range pool.Population {
			avgFitness += dna.Fitness
		}
		avgFitness /= float64(CursePopSize)

		if avgFitness >= dm.config.ReseedThreshold {
			continue // performing adequately
		}

		// Get archetype for seeding.
		archetype := ""
		bracket := pool.Bracket
		if strategyLookup != nil {
			if sp := strategyLookup(pool.DeckKey); sp != nil {
				archetype = sp.Archetype
				if sp.Bracket > 0 {
					bracket = sp.Bracket
				}
			}
		}

		// Replace the bottom half of the population with seeded DNA.
		centroid := dm.SeedDNAFromManifest(archetype, bracket)
		indices := make([]int, CursePopSize)
		for i := range indices {
			indices[i] = i
		}
		sort.Slice(indices, func(a, b int) bool {
			return pool.Population[indices[a]].Fitness < pool.Population[indices[b]].Fitness
		})

		// Replace bottom half.
		replaceCount := CursePopSize / 2
		for k := 0; k < replaceCount; k++ {
			idx := indices[k]
			pool.Population[idx] = CurseDNA{
				DeckKey:               pool.DeckKey,
				Aggression:            clampUnit(centroid[0] + pool.rng.NormFloat64()*0.08),
				ComboPat:              clampUnit(centroid[1] + pool.rng.NormFloat64()*0.08),
				ThreatParanoia:        clampUnit(centroid[2] + pool.rng.NormFloat64()*0.08),
				ResourceGreed:         clampUnit(centroid[3] + pool.rng.NormFloat64()*0.08),
				PoliticalMemory:       clampUnit(centroid[4] + pool.rng.NormFloat64()*0.08),
				DrainAffinity:         clampUnit(centroid[5] + pool.rng.NormFloat64()*0.08),
				ArtifactAffinity:      clampUnit(centroid[6] + pool.rng.NormFloat64()*0.08),
				LandGreed:             pool.rng.Float64(),
				EquipmentAffinity:     pool.rng.Float64(),
				GraveyardExploitation: pool.rng.Float64(),
				CounterplayTiming:     pool.rng.Float64(),
				TokenPressure:         pool.rng.Float64(),
				Fitness:               0.25,
			}
		}

		if err := SavePool(dm.config.CurseDir, pool); err == nil {
			reseeded++
		}
	}

	return reseeded
}

// ---------------------------------------------------------------------------
// Manifest persistence
// ---------------------------------------------------------------------------

// SaveDistillationManifest writes the harvest manifest to disk.
func SaveDistillationManifest(path string, manifest *DistillationManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("save-manifest: mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadDistillationManifest reads a previously saved manifest.
func LoadDistillationManifest(path string) (*DistillationManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest DistillationManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// TryLoadManifest attempts to load an existing manifest and populate the
// manager's internal state for seeding. Returns true if a manifest was loaded.
func (dm *DistillationManager) TryLoadManifest() bool {
	path := filepath.Join(dm.config.TrainingDir, "distillation_manifest.json")
	manifest, err := LoadDistillationManifest(path)
	if err != nil {
		return false
	}
	dm.mu.Lock()
	dm.manifest = manifest
	dm.mu.Unlock()
	return true
}

// IsRunning returns whether a distillation cycle is in progress.
func (dm *DistillationManager) IsRunning() bool {
	return atomic.LoadInt32(&dm.running) == 1
}

// ManifestSize returns the number of harvested DNA entries in the current manifest.
func (dm *DistillationManager) ManifestSize() int {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if dm.manifest == nil {
		return 0
	}
	return len(dm.manifest.HarvestedDNA)
}

// Status returns a human-readable status string.
func (dm *DistillationManager) Status() string {
	dm.mu.Lock()
	manifest := dm.manifest
	dm.mu.Unlock()

	manifestInfo := "no manifest"
	if manifest != nil {
		manifestInfo = fmt.Sprintf("harvested=%d pools=%d", len(manifest.HarvestedDNA), manifest.TotalPools)
	}
	return fmt.Sprintf("distillation: running=%v %s", dm.IsRunning(), manifestInfo)
}
