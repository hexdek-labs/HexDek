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
	rawNTupleFile     = "raw_ntuples.json"
	learnedNTupleFile = "learned_ntuples.json"
	tier3NTupleFile   = "tier3_ntuples_for_freya.json"

	maxTier1And2NTuples = 500
)

// FreyaCombo is a confirmed N-card combo line exported for Freya's
// consumption. Written when an NTuple graduates to Tier 3.
type FreyaCombo struct {
	Cards      []string `json:"cards"`
	AvgImpact  float64  `json:"avg_impact"`
	Confidence int      `json:"observation_count"`
}

// Tier3NTupleExport is the on-disk shape of tier3_ntuples_for_freya.json.
// Holds confirmed N-card combo lines observed directly (not inferred).
type Tier3NTupleExport struct {
	NTuples []FreyaCombo `json:"ntuples"`
}

// RawNTuple is a single N-card co-firing observation persisted from a
// tournament run. Mirrors analytics.CoTriggerNTuple with deck context.
type RawNTuple struct {
	Cards          []string `json:"cards"`
	ImpactScore    float64  `json:"impact_score"`
	TurnWindow     int      `json:"turn_window"`
	EffectPatterns []string `json:"effect_patterns"`
	GameID         string   `json:"game_id"`
	DeckNames      []string `json:"deck_names"`
	Timestamp      string   `json:"timestamp"`
}

// LearnedNTuple is a graduated N-card co-firing pattern stored across
// runs. Mirrors LearnedInteraction but keyed on the sorted cards tuple
// rather than a normalized resource pattern.
type LearnedNTuple struct {
	Cards              []string `json:"cards"`
	NormalizedKey      string   `json:"normalized_key"`
	ObservationCount   int      `json:"observation_count"`
	UniqueDeckCount    int      `json:"unique_deck_count"`
	AvgImpactScore     float64  `json:"avg_impact_score"`
	TotalImpact        float64  `json:"total_impact"`
	FirstSeen          string   `json:"first_seen"`
	LastSeen           string   `json:"last_seen"`
	Tier               int      `json:"tier"`
	GamesSinceLastSeen int      `json:"games_since_last_seen"`
	// SeenDecks persists distinct deck identities across drained-inbox
	// ingest batches — see LearnedInteraction.SeenDecks.
	SeenDecks []string `json:"seen_decks,omitempty"`

	seenDecks map[string]bool
}

// nTupleKey produces a canonical key from a tuple's cards by sorting and
// joining with NUL. Tuples that contain the same cards in any order
// collapse to the same key.
func nTupleKey(cards []string) string {
	c := append([]string(nil), cards...)
	sort.Strings(c)
	return strings.Join(c, "\x00")
}

// PersistRawNTuples appends N-tuple observations from a tournament run to
// data/huginn/raw_ntuples.json. Append-only.
func PersistRawNTuples(dir string, analyses []*analytics.GameAnalysis, commanderNames []string) error {
	var allObs []analytics.CoTriggerNTuple
	for _, ga := range analyses {
		if ga == nil {
			continue
		}
		allObs = append(allObs, ga.CoTriggerNTuples...)
	}
	if len(allObs) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("huginn: mkdir %s: %w", dir, err)
	}

	existing, err := ReadRawNTuples(dir)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, obs := range allObs {
		existing = append(existing, RawNTuple{
			Cards:          append([]string(nil), obs.Cards...),
			ImpactScore:    obs.ImpactScore,
			TurnWindow:     obs.TurnWindow,
			EffectPatterns: append([]string(nil), obs.EffectPatterns...),
			GameID:         obs.GameID,
			DeckNames:      append([]string(nil), commanderNames...),
			Timestamp:      now,
		})
	}

	return atomicWriteJSON(filepath.Join(dir, rawNTupleFile), existing)
}

// ReadRawNTuples reads the raw n-tuples file.
func ReadRawNTuples(dir string) ([]RawNTuple, error) {
	var out []RawNTuple
	if err := readJSON(filepath.Join(dir, rawNTupleFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []RawNTuple{}
	}
	return out, nil
}

// ReadLearnedNTuples reads the graduated n-tuples file.
func ReadLearnedNTuples(dir string) ([]LearnedNTuple, error) {
	var out []LearnedNTuple
	if err := readJSON(filepath.Join(dir, learnedNTupleFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []LearnedNTuple{}
	}
	return out, nil
}

// IngestNTuples processes raw n-tuple observations and updates the learned
// n-tuples database. Returns newly promoted tier 3 entries.
func IngestNTuples(dir string, gamesSinceRun int) (promotions []LearnedNTuple, err error) {
	raw, err := ReadRawNTuples(dir)
	if err != nil {
		return nil, fmt.Errorf("huginn: read raw ntuples: %w", err)
	}

	existing, err := ReadLearnedNTuples(dir)
	if err != nil {
		return nil, fmt.Errorf("huginn: read learned ntuples: %w", err)
	}

	byKey := make(map[string]*LearnedNTuple, len(existing))
	for i := range existing {
		existing[i].seenDecks = loadSeenDecks(existing[i].SeenDecks)
		if existing[i].NormalizedKey == "" {
			existing[i].NormalizedKey = nTupleKey(existing[i].Cards)
		}
		byKey[existing[i].NormalizedKey] = &existing[i]
	}

	wasTier := make(map[string]int)
	for _, ln := range existing {
		wasTier[ln.NormalizedKey] = ln.Tier
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for _, obs := range raw {
		if len(obs.Cards) < 3 {
			continue
		}
		key := nTupleKey(obs.Cards)
		ln, ok := byKey[key]
		if !ok {
			cards := append([]string(nil), obs.Cards...)
			sort.Strings(cards)
			ln = &LearnedNTuple{
				Cards:         cards,
				NormalizedKey: key,
				FirstSeen:     now,
				Tier:          TierObserved,
				seenDecks:     make(map[string]bool),
			}
			byKey[key] = ln
		}

		ln.ObservationCount++
		ln.TotalImpact += obs.ImpactScore
		ln.LastSeen = now
		ln.GamesSinceLastSeen = 0

		for _, d := range obs.DeckNames {
			ln.seenDecks[d] = true
		}
	}

	result := make([]LearnedNTuple, 0, len(byKey))
	for _, ln := range byKey {
		if len(ln.seenDecks) > 0 {
			ln.UniqueDeckCount = len(ln.seenDecks)
			ln.SeenDecks = persistSeenDecks(ln.seenDecks)
		}
		if ln.ObservationCount > 0 {
			ln.AvgImpactScore = ln.TotalImpact / float64(ln.ObservationCount)
		}

		oldTier := wasTier[ln.NormalizedKey]
		if ln.Tier < TierConfirmed && ln.ObservationCount >= tier3MinObs && ln.AvgImpactScore >= tier3MinImpact {
			ln.Tier = TierConfirmed
		} else if ln.Tier < TierRecurring && ln.ObservationCount >= tier2MinObs && ln.UniqueDeckCount >= tier2MinDecks {
			ln.Tier = TierRecurring
		}

		if ln.Tier == TierConfirmed && oldTier < TierConfirmed {
			promotions = append(promotions, *ln)
		}

		ln.GamesSinceLastSeen += gamesSinceRun
		ln.seenDecks = nil
		result = append(result, *ln)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Tier != result[j].Tier {
			return result[i].Tier > result[j].Tier
		}
		return result[i].TotalImpact > result[j].TotalImpact
	})

	if err := atomicWriteJSON(filepath.Join(dir, learnedNTupleFile), result); err != nil {
		return nil, fmt.Errorf("huginn: write learned ntuples: %w", err)
	}

	if err := exportTier3NTuplesForFreya(dir, result); err != nil {
		fmt.Fprintf(os.Stderr, "huginn: export tier3 ntuples for freya: %v\n", err)
	}

	// Drain the inbox — same once-only fold contract as Ingest. Without
	// this, every batch run re-counts the entire append-only raw file.
	if len(raw) > 0 {
		if err := os.Remove(filepath.Join(dir, rawNTupleFile)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "huginn: drain raw ntuples: %v\n", err)
		}
	}

	return promotions, nil
}

// PruneNTuples removes stale tier 1 and tier 2 n-tuples and enforces the
// combined cap. Returns the number of entries removed.
func PruneNTuples(dir string) (int, error) {
	entries, err := ReadLearnedNTuples(dir)
	if err != nil {
		return 0, err
	}

	before := len(entries)
	var kept []LearnedNTuple
	for _, ln := range entries {
		switch ln.Tier {
		case TierConfirmed:
			kept = append(kept, ln)
		case TierRecurring:
			if ln.GamesSinceLastSeen < tier2PruneGames {
				kept = append(kept, ln)
			}
		case TierObserved:
			if ln.GamesSinceLastSeen < tier1PruneGames {
				kept = append(kept, ln)
			}
		}
	}

	var tier3, tier12 []LearnedNTuple
	for _, ln := range kept {
		if ln.Tier == TierConfirmed {
			tier3 = append(tier3, ln)
		} else {
			tier12 = append(tier12, ln)
		}
	}

	if len(tier12) > maxTier1And2NTuples {
		sort.SliceStable(tier12, func(i, j int) bool {
			return tier12[i].TotalImpact > tier12[j].TotalImpact
		})
		tier12 = tier12[:maxTier1And2NTuples]
	}

	result := append(tier3, tier12...)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Tier != result[j].Tier {
			return result[i].Tier > result[j].Tier
		}
		return result[i].TotalImpact > result[j].TotalImpact
	})

	if err := atomicWriteJSON(filepath.Join(dir, learnedNTupleFile), result); err != nil {
		return 0, err
	}

	return before - len(result), nil
}

// exportTier3NTuplesForFreya writes all tier 3 n-tuples to
// tier3_ntuples_for_freya.json so Freya can incorporate directly observed
// emergent N-card combo lines into its analysis.
func exportTier3NTuplesForFreya(dir string, entries []LearnedNTuple) error {
	var combos []FreyaCombo
	for _, ln := range entries {
		if ln.Tier < TierConfirmed {
			continue
		}
		combos = append(combos, FreyaCombo{
			Cards:      append([]string(nil), ln.Cards...),
			AvgImpact:  ln.AvgImpactScore,
			Confidence: ln.ObservationCount,
		})
	}
	path := filepath.Join(dir, tier3NTupleFile)
	if len(combos) == 0 {
		os.Remove(path)
		return nil
	}
	return atomicWriteJSON(path, Tier3NTupleExport{NTuples: combos})
}

// ReadTier3NTupleExport reads tier3_ntuples_for_freya.json. Returns a
// zero value if the file does not exist.
func ReadTier3NTupleExport(dir string) (Tier3NTupleExport, error) {
	path := filepath.Join(dir, tier3NTupleFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Tier3NTupleExport{}, nil
		}
		return Tier3NTupleExport{}, fmt.Errorf("huginn: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return Tier3NTupleExport{}, nil
	}
	var exp Tier3NTupleExport
	if err := json.Unmarshal(data, &exp); err != nil {
		return Tier3NTupleExport{}, fmt.Errorf("huginn: parse %s: %w", path, err)
	}
	return exp, nil
}
