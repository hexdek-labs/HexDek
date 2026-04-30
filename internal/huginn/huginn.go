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

	rawObsFile          = "raw_observations.json"
	learnedFile         = "learned_interactions.json"
)

// RawObservation is a single co-trigger observation persisted from a
// tournament run. Mirrors analytics.CoTriggerObservation with deck context.
type RawObservation struct {
	CardA         string  `json:"card_a"`
	CardB         string  `json:"card_b"`
	ImpactScore   float64 `json:"impact_score"`
	TurnWindow    int     `json:"turn_window"`
	EffectPattern string  `json:"effect_pattern"`
	GameID        string  `json:"game_id"`
	DeckNames     []string `json:"deck_names"`
	Timestamp     string  `json:"timestamp"`
}

// LearnedInteraction is a graduated interaction pattern stored across runs.
type LearnedInteraction struct {
	Pattern          string   `json:"pattern"`
	ExampleCards     []string `json:"example_cards"`
	ObservationCount int      `json:"observation_count"`
	UniqueDeckCount  int      `json:"unique_deck_count"`
	AvgImpactScore   float64  `json:"avg_impact_score"`
	TotalImpact      float64  `json:"total_impact"`
	FirstSeen        string   `json:"first_seen"`
	LastSeen         string   `json:"last_seen"`
	Tier             int      `json:"tier"`
	GamesSinceLastSeen int    `json:"games_since_last_seen"`

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

// NormalizePattern extracts the resource flow pattern from an effect
// pattern string. Strips card names and keeps only the resource type
// and direction: "produces X, consumes X" → "produces X → consumes X".
func NormalizePattern(effectPattern string) string {
	parts := strings.SplitN(effectPattern, ", ", 2)
	if len(parts) != 2 {
		return effectPattern
	}
	produces := extractVerb(parts[0])
	consumes := extractVerb(parts[1])
	if produces == "" || consumes == "" {
		return effectPattern
	}
	return produces + " → " + consumes
}

func extractVerb(s string) string {
	// "CardName produces mana" → "produces mana"
	if idx := strings.Index(s, " produces "); idx >= 0 {
		return s[idx+1:]
	}
	if idx := strings.Index(s, " consumes "); idx >= 0 {
		return s[idx+1:]
	}
	return ""
}

// Ingest processes raw observations and updates the learned interactions
// database. Returns newly promoted tier 3 entries (promotion candidates).
func Ingest(dir string, gamesSinceRun int) (promotions []LearnedInteraction, err error) {
	raw, err := ReadRawObservations(dir)
	if err != nil {
		return nil, fmt.Errorf("huginn: read raw: %w", err)
	}

	existing, err := ReadLearnedInteractions(dir)
	if err != nil {
		return nil, fmt.Errorf("huginn: read learned: %w", err)
	}

	// Index existing by pattern.
	byPattern := make(map[string]*LearnedInteraction, len(existing))
	for i := range existing {
		existing[i].seenDecks = make(map[string]bool)
		byPattern[existing[i].Pattern] = &existing[i]
	}

	// Track which patterns were tier 2 before ingestion (for promotion detection).
	wasTier := make(map[string]int)
	for _, li := range existing {
		wasTier[li.Pattern] = li.Tier
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Process raw observations.
	for _, obs := range raw {
		pattern := NormalizePattern(obs.EffectPattern)
		if pattern == "" {
			continue
		}

		li, ok := byPattern[pattern]
		if !ok {
			li = &LearnedInteraction{
				Pattern:      pattern,
				FirstSeen:    now,
				Tier:         TierObserved,
				seenDecks:    make(map[string]bool),
			}
			byPattern[pattern] = li
		}

		li.ObservationCount++
		li.TotalImpact += obs.ImpactScore
		li.LastSeen = now
		li.GamesSinceLastSeen = 0

		// Track unique decks.
		for _, d := range obs.DeckNames {
			li.seenDecks[d] = true
		}

		// Add example cards (dedup, cap at 10).
		a, b := obs.CardA, obs.CardB
		if a > b {
			a, b = b, a
		}
		example := a + " + " + b
		found := false
		for _, ex := range li.ExampleCards {
			if ex == example {
				found = true
				break
			}
		}
		if !found && len(li.ExampleCards) < 10 {
			li.ExampleCards = append(li.ExampleCards, example)
		}
	}

	// Finalize deck counts and averages, run tier promotion.
	result := make([]LearnedInteraction, 0, len(byPattern))
	for _, li := range byPattern {
		if len(li.seenDecks) > 0 {
			li.UniqueDeckCount = len(li.seenDecks)
		}
		if li.ObservationCount > 0 {
			li.AvgImpactScore = li.TotalImpact / float64(li.ObservationCount)
		}

		// Tier promotion (never demote).
		oldTier := wasTier[li.Pattern]
		if li.Tier < TierConfirmed && li.ObservationCount >= tier3MinObs && li.AvgImpactScore >= tier3MinImpact {
			li.Tier = TierConfirmed
		} else if li.Tier < TierRecurring && li.ObservationCount >= tier2MinObs && li.UniqueDeckCount >= tier2MinDecks {
			li.Tier = TierRecurring
		}

		// Detect new promotions to tier 3.
		if li.Tier == TierConfirmed && oldTier < TierConfirmed {
			promotions = append(promotions, *li)
		}

		// Age tracking for pruning.
		li.GamesSinceLastSeen += gamesSinceRun
		li.seenDecks = nil // don't serialize
		result = append(result, *li)
	}

	// Sort by tier desc, then impact desc.
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Tier != result[j].Tier {
			return result[i].Tier > result[j].Tier
		}
		return result[i].TotalImpact > result[j].TotalImpact
	})

	if err := atomicWriteJSON(filepath.Join(dir, learnedFile), result); err != nil {
		return nil, fmt.Errorf("huginn: write learned: %w", err)
	}

	return promotions, nil
}

// Prune removes stale tier 1 and tier 2 interactions, and enforces the
// combined cap. Returns the number of entries removed.
func Prune(dir string) (int, error) {
	interactions, err := ReadLearnedInteractions(dir)
	if err != nil {
		return 0, err
	}

	before := len(interactions)
	var kept []LearnedInteraction

	for _, li := range interactions {
		switch li.Tier {
		case TierConfirmed:
			kept = append(kept, li)
		case TierRecurring:
			if li.GamesSinceLastSeen < tier2PruneGames {
				kept = append(kept, li)
			}
		case TierObserved:
			if li.GamesSinceLastSeen < tier1PruneGames {
				kept = append(kept, li)
			}
		}
	}

	// Enforce cap on tier 1+2 combined.
	var tier3, tier12 []LearnedInteraction
	for _, li := range kept {
		if li.Tier == TierConfirmed {
			tier3 = append(tier3, li)
		} else {
			tier12 = append(tier12, li)
		}
	}

	if len(tier12) > maxTier1And2 {
		sort.SliceStable(tier12, func(i, j int) bool {
			return tier12[i].TotalImpact > tier12[j].TotalImpact
		})
		tier12 = tier12[:maxTier1And2]
	}

	result := append(tier3, tier12...)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Tier != result[j].Tier {
			return result[i].Tier > result[j].Tier
		}
		return result[i].TotalImpact > result[j].TotalImpact
	})

	if err := atomicWriteJSON(filepath.Join(dir, learnedFile), result); err != nil {
		return 0, err
	}

	return before - len(result), nil
}

// Stats returns tier counts.
func Stats(dir string) (tier1, tier2, tier3, total int, err error) {
	interactions, err := ReadLearnedInteractions(dir)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	for _, li := range interactions {
		switch li.Tier {
		case TierObserved:
			tier1++
		case TierRecurring:
			tier2++
		case TierConfirmed:
			tier3++
		}
	}
	total = len(interactions)
	return
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
