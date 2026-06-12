// Package huginn discovers emergent card interactions from Heimdall's
// co-trigger observations. Named after Odin's raven of thought, it
// compresses raw observations into patterns, graduates them through
// confidence tiers, and persists the learned interactions for Freya
// consumption.
//
// Tier lifecycle:
//   - Tier 1 OBSERVED: seen once. Pruned after 200 games without recurrence.
//   - Tier 2 RECURRING: seen 3+ times across 2+ decks. Pruned after 500 games.
//   - Tier 3 CONFIRMED: seen 5+ times, avg impact above threshold. Permanent.
package huginn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
)

const (
	TierObserved  = 1
	TierRecurring = 2
	TierConfirmed = 3

	tier1PruneGames = 200
	tier2PruneGames = 500

	tier2MinObs   = 3
	tier2MinDecks = 2

	tier3MinObs    = 5
	tier3MinImpact = 5.0

	maxTier1And2 = 500

	// N-card chain inference bounds.
	chainMinLength     = 3
	chainMaxLength     = 5
	maxChainCandidates = 50000 // hard stop to keep DFS bounded
	maxChainsExport    = 200

	rawObsFile     = "raw_observations.json"
	learnedFile    = "learned_interactions.json"
	tier3FreyaFile = "tier3_for_freya.json"
)

// FreyaInteraction is a confirmed interaction exported for Freya's
// combo/synergy detection. Written when a pattern graduates to Tier 3.
type FreyaInteraction struct {
	CardA      string  `json:"card_a"`
	CardB      string  `json:"card_b"`
	Pattern    string  `json:"pattern"`
	AvgImpact  float64 `json:"avg_impact"`
	Confidence int     `json:"observation_count"`
}

// FreyaChain is an N-card combo line (3-5 cards) inferred by chaining
// consecutive tier 3 confirmed pairs through shared cards. If A+B and
// B+C are both confirmed, A→B→C is emitted as a 3-card chain.
type FreyaChain struct {
	Cards         []string `json:"cards"`          // ordered chain; Cards[i] connects to Cards[i+1]
	Patterns      []string `json:"patterns"`       // Patterns[i] is the resource flow between Cards[i] and Cards[i+1]
	Length        int      `json:"length"`         // number of cards in the chain (3, 4, or 5)
	AvgImpact     float64  `json:"avg_impact"`     // mean of segment avg impacts
	MinConfidence int      `json:"min_confidence"` // weakest segment's observation count
}

// Tier3Export is the on-disk shape of tier3_for_freya.json. Pairs holds
// each tier 3 example pair (one entry per example card pair); Chains
// holds N-card lines inferred from chained pairs.
type Tier3Export struct {
	Pairs  []FreyaInteraction `json:"pairs"`
	Chains []FreyaChain       `json:"chains"`
}

// RawCycleObservation is a single cycle observation persisted from a
// tournament run — engine-detected mandatory loops (§727) plus
// post-game graph-walker hits (combo shapes that resolved finitely).
// Mirrors analytics.CycleObservation with deck context, matching the
// RawObservation shape pattern used for co-trigger pairs.
//
// Append-only on-disk, written to data/huginn/raw_cycles.json by
// PersistRawCycles. The Huginn ingestion pipeline can promote
// recurring cycles to confirmed combo lines using the same tier
// progression as ordinary observations.
type RawCycleObservation struct {
	CycleLength        int      `json:"cycle_length"`
	ParticipatingIIDs  []string `json:"participating_iids"`
	ParticipatingCards []string `json:"participating_cards"`
	TurnWindow         int      `json:"turn_window"`
	DetectedBy         string   `json:"detected_by"` // "engine_no_op_loop" / "engine_cr_727" / "graph_walker"
	GameID             string   `json:"game_id"`
	DeckNames          []string `json:"deck_names"`
	Timestamp          string   `json:"timestamp"`
}

// rawCyclesFile is the on-disk filename for cycle observations.
// Sibling to rawObsFile in the same data/huginn/ directory.
const rawCyclesFile = "raw_cycles.json"

// PersistRawCycles appends cycle observations from a tournament run
// to data/huginn/raw_cycles.json. Append-only, mirroring
// PersistRawObservations. Skips when there's nothing to write.
func PersistRawCycles(dir string, analyses []*analytics.GameAnalysis, commanderNames []string) error {
	var all []analytics.CycleObservation
	for _, ga := range analyses {
		if ga == nil {
			continue
		}
		all = append(all, ga.CycleObservations...)
	}
	if len(all) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("huginn: mkdir %s: %w", dir, err)
	}
	existing, err := ReadRawCycles(dir)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, c := range all {
		existing = append(existing, RawCycleObservation{
			CycleLength:        c.CycleLength,
			ParticipatingIIDs:  append([]string(nil), c.ParticipatingIIDs...),
			ParticipatingCards: append([]string(nil), c.ParticipatingCards...),
			TurnWindow:         c.TurnWindow,
			DetectedBy:         c.DetectedBy,
			GameID:             c.GameID,
			DeckNames:          append([]string(nil), commanderNames...),
			Timestamp:          now,
		})
	}
	return atomicWriteJSON(filepath.Join(dir, rawCyclesFile), existing)
}

// ReadRawCycles reads the raw cycles file. Empty file / missing
// path returns an empty slice (no error) — matches ReadRawObservations.
func ReadRawCycles(dir string) ([]RawCycleObservation, error) {
	var out []RawCycleObservation
	if err := readJSON(filepath.Join(dir, rawCyclesFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []RawCycleObservation{}
	}
	return out, nil
}

// RawObservation is a single co-trigger observation persisted from a
// tournament run. Mirrors analytics.CoTriggerObservation with deck context.
type RawObservation struct {
	CardA         string   `json:"card_a"`
	CardB         string   `json:"card_b"`
	ImpactScore   float64  `json:"impact_score"`
	TurnWindow    int      `json:"turn_window"`
	EffectPattern string   `json:"effect_pattern"`
	GameID        string   `json:"game_id"`
	DeckNames     []string `json:"deck_names"`
	Timestamp     string   `json:"timestamp"`
}

// LearnedInteraction is a graduated interaction pattern stored across runs.
type LearnedInteraction struct {
	Pattern            string   `json:"pattern"`
	ExampleCards       []string `json:"example_cards"`
	ObservationCount   int      `json:"observation_count"`
	UniqueDeckCount    int      `json:"unique_deck_count"`
	AvgImpactScore     float64  `json:"avg_impact_score"`
	TotalImpact        float64  `json:"total_impact"`
	FirstSeen          string   `json:"first_seen"`
	LastSeen           string   `json:"last_seen"`
	Tier               int      `json:"tier"`
	GamesSinceLastSeen int      `json:"games_since_last_seen"`

	seenDecks map[string]bool
}

// PersistRawObservations appends co-trigger observations from a tournament
// run to data/huginn/raw_observations.json. Append-only.
func PersistRawObservations(dir string, analyses []*analytics.GameAnalysis, commanderNames []string) error {
	var allObs []analytics.CoTriggerObservation
	for _, ga := range analyses {
		if ga == nil {
			continue
		}
		allObs = append(allObs, ga.CoTriggerObservations...)
	}
	if len(allObs) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("huginn: mkdir %s: %w", dir, err)
	}

	existing, err := ReadRawObservations(dir)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, obs := range allObs {
		existing = append(existing, RawObservation{
			CardA:         obs.CardA,
			CardB:         obs.CardB,
			ImpactScore:   obs.ImpactScore,
			TurnWindow:    obs.TurnWindow,
			EffectPattern: obs.EffectPattern,
			GameID:        obs.GameID,
			DeckNames:     append([]string(nil), commanderNames...),
			Timestamp:     now,
		})
	}

	return atomicWriteJSON(filepath.Join(dir, rawObsFile), existing)
}

// PersistRawObservationsRaw writes pre-built raw observations to disk.
// Used by the Huginn adapter when observations are already in the
// RawObservation format (e.g., converted from Heimdall's CoTriggerPair).
func PersistRawObservationsRaw(dir string, observations []RawObservation) error {
	if len(observations) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("huginn: mkdir %s: %w", dir, err)
	}
	return atomicWriteJSON(filepath.Join(dir, rawObsFile), observations)
}

// ReadRawObservations reads the raw observations file.
func ReadRawObservations(dir string) ([]RawObservation, error) {
	var out []RawObservation
	if err := readJSON(filepath.Join(dir, rawObsFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []RawObservation{}
	}
	return out, nil
}

// ReadLearnedInteractions reads the graduated interactions file.
func ReadLearnedInteractions(dir string) ([]LearnedInteraction, error) {
	var out []LearnedInteraction
	if err := readJSON(filepath.Join(dir, learnedFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []LearnedInteraction{}
	}
	return out, nil
}

// exportTier3ForFreya writes all tier 3 (confirmed) interactions to
// tier3_for_freya.json as a Tier3Export object. Pairs holds one
// FreyaInteraction per example card pair; Chains holds N-card lines
// inferred from chained pairs.
func exportTier3ForFreya(dir string, interactions []LearnedInteraction) error {
	var pairs []FreyaInteraction
	for _, li := range interactions {
		if li.Tier < TierConfirmed {
			continue
		}
		for _, example := range li.ExampleCards {
			parts := strings.SplitN(example, " + ", 2)
			if len(parts) != 2 {
				continue
			}
			pairs = append(pairs, FreyaInteraction{
				CardA:      parts[0],
				CardB:      parts[1],
				Pattern:    li.Pattern,
				AvgImpact:  li.AvgImpactScore,
				Confidence: li.ObservationCount,
			})
		}
	}
	chains := InferChains(interactions)
	if len(pairs) == 0 && len(chains) == 0 {
		// Don't write an empty file; remove stale file if it exists.
		os.Remove(filepath.Join(dir, tier3FreyaFile))
		return nil
	}
	return atomicWriteJSON(filepath.Join(dir, tier3FreyaFile), Tier3Export{Pairs: pairs, Chains: chains})
}

// ReadTier3Export reads tier3_for_freya.json. Accepts both the current
// object form ({pairs:[...], chains:[...]}) and the legacy bare-array
// form (treated as Pairs). Returns a zero value if the file is missing.
func ReadTier3Export(dir string) (Tier3Export, error) {
	path := filepath.Join(dir, tier3FreyaFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Tier3Export{}, nil
		}
		return Tier3Export{}, fmt.Errorf("huginn: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return Tier3Export{}, nil
	}
	// Object form first.
	var exp Tier3Export
	if err := json.Unmarshal(data, &exp); err == nil && (exp.Pairs != nil || exp.Chains != nil) {
		return exp, nil
	}
	// Legacy array form.
	var pairs []FreyaInteraction
	if err := json.Unmarshal(data, &pairs); err != nil {
		return Tier3Export{}, fmt.Errorf("huginn: parse %s: %w", path, err)
	}
	return Tier3Export{Pairs: pairs}, nil
}

// ReadTier3ForFreya returns just the pair interactions from the tier 3
// export. Kept for compatibility with callers that don't need chains.
func ReadTier3ForFreya(dir string) ([]FreyaInteraction, error) {
	exp, err := ReadTier3Export(dir)
	if err != nil {
		return nil, err
	}
	return exp.Pairs, nil
}

// InferChains builds an undirected graph of tier 3 confirmed card pairs
// (using each LearnedInteraction's ExampleCards) and enumerates simple
// paths of length 3..5 cards. Each chain is canonicalized (lex-smaller
// of forward/reverse), sorted by avg impact desc with deterministic
// tiebreaks, and capped at maxChainsExport.
//
// The premise: if A+B and B+C are independently confirmed at tier 3,
// the trio A→B→C is a plausible N-card combo line worth surfacing for
// Freya's deck analysis even if it has never been directly observed.
func InferChains(interactions []LearnedInteraction) []FreyaChain {
	type chainEdge struct {
		pattern    string
		impact     float64
		confidence int
	}
	adj := make(map[string]map[string]*chainEdge)
	addEdge := func(a, b, pattern string, impact float64, conf int) {
		if a == "" || b == "" || a == b {
			return
		}
		if adj[a] == nil {
			adj[a] = make(map[string]*chainEdge)
		}
		if adj[b] == nil {
			adj[b] = make(map[string]*chainEdge)
		}
		// If the same card pair appears in multiple confirmed patterns,
		// keep the strongest (by impact) edge.
		if e, ok := adj[a][b]; !ok || impact > e.impact {
			edge := &chainEdge{pattern: pattern, impact: impact, confidence: conf}
			adj[a][b] = edge
			adj[b][a] = edge
		}
	}
	for _, li := range interactions {
		if li.Tier < TierConfirmed {
			continue
		}
		for _, ex := range li.ExampleCards {
			parts := strings.SplitN(ex, " + ", 2)
			if len(parts) != 2 {
				continue
			}
			addEdge(parts[0], parts[1], li.Pattern, li.AvgImpactScore, li.ObservationCount)
		}
	}
	if len(adj) < chainMinLength {
		return nil
	}

	cards := make([]string, 0, len(adj))
	for c := range adj {
		cards = append(cards, c)
	}
	sort.Strings(cards)

	seen := make(map[string]bool)
	var out []FreyaChain
	hitCap := false

	record := func(path []string) {
		key := canonicalChainKey(path)
		if seen[key] {
			return
		}
		seen[key] = true
		segments := len(path) - 1
		patterns := make([]string, segments)
		var sumImpact float64
		minConf := 0
		for i := 0; i < segments; i++ {
			e := adj[path[i]][path[i+1]]
			patterns[i] = e.pattern
			sumImpact += e.impact
			if i == 0 || e.confidence < minConf {
				minConf = e.confidence
			}
		}
		out = append(out, FreyaChain{
			Cards:         append([]string(nil), path...),
			Patterns:      patterns,
			Length:        len(path),
			AvgImpact:     sumImpact / float64(segments),
			MinConfidence: minConf,
		})
	}

	var dfs func(path []string)
	dfs = func(path []string) {
		if hitCap {
			return
		}
		if len(path) >= chainMinLength {
			record(path)
			if len(out) >= maxChainCandidates {
				hitCap = true
				return
			}
		}
		if len(path) >= chainMaxLength {
			return
		}
		last := path[len(path)-1]
		nbrs := make([]string, 0, len(adj[last]))
		for n := range adj[last] {
			nbrs = append(nbrs, n)
		}
		sort.Strings(nbrs)
		for _, n := range nbrs {
			visited := false
			for _, p := range path {
				if p == n {
					visited = true
					break
				}
			}
			if visited {
				continue
			}
			next := make([]string, len(path)+1)
			copy(next, path)
			next[len(path)] = n
			dfs(next)
			if hitCap {
				return
			}
		}
	}

	for _, c := range cards {
		dfs([]string{c})
		if hitCap {
			break
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AvgImpact != out[j].AvgImpact {
			return out[i].AvgImpact > out[j].AvgImpact
		}
		if out[i].Length != out[j].Length {
			return out[i].Length > out[j].Length
		}
		return strings.Join(out[i].Cards, "|") < strings.Join(out[j].Cards, "|")
	})
	if len(out) > maxChainsExport {
		out = out[:maxChainsExport]
	}
	return out
}

// canonicalChainKey returns a stable identifier for a chain regardless
// of traversal direction: A→B→C and C→B→A collapse to the same key.
func canonicalChainKey(path []string) string {
	fwd := strings.Join(path, "→")
	rev := make([]string, len(path))
	for i := range path {
		rev[i] = path[len(path)-1-i]
	}
	revStr := strings.Join(rev, "→")
	if fwd < revStr {
		return fwd
	}
	return revStr
}

// atomicWriteJSON writes data as indented JSON via temp file + rename.
func atomicWriteJSON(path string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("huginn: marshal: %w", err)
	}
	out = append(out, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("huginn: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("huginn: rename: %w", err)
	}
	return nil
}

func readJSON(path string, dst interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("huginn: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, dst)
}
