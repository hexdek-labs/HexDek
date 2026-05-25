package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// "Decks like yours" recommendations.
//
// Builds on the deck-similarity scorer (similarity.go) by attaching
// runtime performance metadata (winrate from gauntlet/tournament games)
// to the top-N most-similar corpus entries. The Decks screen renders
// each row as one line: name + bracket + winrate + similarity verdict.
//
// Performance data lives in the engine's SQLite DB (gauntlet_runs +
// matchups). Freya itself is static-analysis only, so we accept the
// stats as an optional callback — the Decks screen wires its own DB
// lookup, the CLI passes nil and renders bracket-only rows. The
// recommendation ranking is independent of performance; performance
// is purely display metadata so a never-played but highly-similar
// deck still surfaces (the user might want to see decks they haven't
// played yet).
// ---------------------------------------------------------------------------

// DeckPerformance is the runtime stats slice attached to each
// recommendation row when the caller supplies a PerformanceLookup.
// WinRate is wins/games × 100 (e.g. 27.4 for 27.4%) to match the
// matchups / gauntlet_runs DB convention.
type DeckPerformance struct {
	Games   int
	Wins    int
	WinRate float64
}

// PerformanceLookup returns the runtime stats for the named deck, or
// nil if no game data exists. Implementations typically wrap a SQL
// query against gauntlet_runs / matchups.
type PerformanceLookup func(deckName string) *DeckPerformance

// DeckRecommendation is one row of the "decks like yours" panel.
// Score carries the full similarity breakdown so the UI can render
// "73% similar" with the component drilldown; Bracket and BracketLabel
// are surfaced for the one-line summary; Performance is nil-tolerant
// (the row still renders without winrate data); Reason is a pre-
// composed one-line summary the text renderer + simple UIs can use
// directly.
type DeckRecommendation struct {
	DeckName     string
	Commander    string
	Bracket      int
	BracketLabel string
	Score        *SimilarityScore
	Performance  *DeckPerformance
	Reason       string
}

// RecommendSimilarDecks finds the top `limit` most-similar decks in
// `corpus` and attaches their bracket + (optional) performance data
// for display. Skips the target by name so a self-comparison doesn't
// return #1=itself. Pass `stats=nil` for static-analysis-only mode
// (the Performance field on each row will be nil).
//
// Recommendation ranking is by similarity Overall descending; ties
// break alphabetically by name. Performance is NOT a ranking input —
// the user explicitly asked for "most similar," not "best similar."
// A 30%-winrate deck that's a near-clone outranks a 60%-winrate deck
// that's only loosely related, which is what the Decks screen wants.
func RecommendSimilarDecks(target *DeckFingerprint, corpus []*DeckFingerprint, limit int, stats PerformanceLookup) []*DeckRecommendation {
	if target == nil {
		return nil
	}
	scored := FindSimilarDecks(target, corpus, limit)
	if len(scored) == 0 {
		return nil
	}

	// Index corpus by name for bracket/label lookup on hits.
	byName := make(map[string]*DeckFingerprint, len(corpus))
	for _, fp := range corpus {
		if fp != nil {
			byName[fp.Name] = fp
		}
	}

	out := make([]*DeckRecommendation, 0, len(scored))
	for _, s := range scored {
		fp := byName[s.OtherDeck]
		rec := &DeckRecommendation{
			DeckName: s.OtherDeck,
			Score:    s,
			Reason:   composeRecommendationReason(s),
		}
		if fp != nil {
			rec.Commander = fp.Commander
			rec.Bracket = fp.Bracket
			rec.BracketLabel = fp.BracketLabel
		}
		if stats != nil {
			rec.Performance = stats(s.OtherDeck)
		}
		out = append(out, rec)
	}
	return out
}

// composeRecommendationReason builds the human-readable one-liner
// the Decks screen renders under the deck name. Names the dominant
// component drivers (shared cards, archetype match, bracket gap) so
// the user understands WHY this deck surfaced.
func composeRecommendationReason(s *SimilarityScore) string {
	if s == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if s.SharedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d shared cards", s.SharedCount))
	}
	switch s.ArchetypeMatch {
	case 1.0:
		parts = append(parts, "same archetype")
	case 0.5:
		parts = append(parts, "archetype crossover")
	}
	if s.BracketDelta == 0 {
		parts = append(parts, "same bracket")
	} else if s.BracketDelta == 1 {
		parts = append(parts, "1-bracket gap")
	} else if s.BracketDelta >= 2 {
		parts = append(parts, fmt.Sprintf("%d-bracket gap", s.BracketDelta))
	}
	if len(parts) == 0 {
		return s.Verdict
	}
	return fmt.Sprintf("%s (%.0f%%): %s", s.Verdict, s.Overall*100, strings.Join(parts, ", "))
}

// FormatDeckRecommendation renders one recommendation as a compact
// two-line block: headline (name + bracket + winrate + verdict) and
// the reason. Used by the text-mode Decks panel.
func FormatDeckRecommendation(r *DeckRecommendation) string {
	if r == nil {
		return ""
	}
	bracket := fmt.Sprintf("B%d", r.Bracket)
	if r.BracketLabel != "" {
		bracket = fmt.Sprintf("B%d %s", r.Bracket, r.BracketLabel)
	}
	perf := "no game data"
	if r.Performance != nil {
		if r.Performance.Games > 0 {
			perf = fmt.Sprintf("%d games, %.1f%% winrate",
				r.Performance.Games, r.Performance.WinRate)
		} else {
			perf = "0 games"
		}
	}
	return fmt.Sprintf("%s [%s] — %s\n    %s",
		r.DeckName, bracket, perf, r.Reason)
}
