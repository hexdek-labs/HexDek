package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Meta positioning, deepened (R60).
//
// Layered atop the existing computeMetaPositioning archetype-vs-archetype
// matchup matrix:
//   1. ExpectedWinPct — derived from (Rating, Strength) via ELO math.
//      The strength comment band ("strong" → ~70-80%, "moderate" → ~60-70%,
//      "slight" → ~52-58%) is implemented as concrete ELO differentials
//      run through the standard 1/(1+10^(-d/400)) expected-score formula.
//   2. Empirical blend — when --elo-history loads an archetype-pair
//      history JSON, the static baseline is shrunk toward observed
//      win rate using a confidence-weighted average. Low-sample pairs
//      (< 10 games) keep the ELO baseline; high-sample pairs (>= 50
//      games) weight empirical at 0.8.
//   3. Tilt detection — matchups whose empirical win rate diverges from
//      the ELO baseline by >= tiltThresholdPP get flagged so the report
//      can surface "you under-perform vs Stax compared to the Combo
//      archetype baseline" as a coaching signal.
//   4. Tech-card recommendations — for the single worst matchup, the
//      hoserDB is queried in REVERSE direction: the conditions that
//      describe the OPPONENT archetype's vulnerabilities become add
//      candidates for this deck.
// ---------------------------------------------------------------------------

// MetaMatchupHistory is the optional JSON payload loaded via --elo-history.
// Schema is intentionally compact so an external aggregator (Heimdall,
// tournament runner) can dump it cheaply: a flat map of "A|B" → record
// where "A|B" means "deck of archetype A versus opponent archetype B"
// and WinsForFirst is A's win count.
type MetaMatchupHistory struct {
	Pairs map[string]MatchupRecord `json:"archetype_pairs"`
}

// MatchupRecord is the observed-history entry for one archetype pair.
type MatchupRecord struct {
	Games        int `json:"games"`
	WinsForFirst int `json:"wins_for_first"`
}

// LoadedMatchupHistory is the package-level loaded history. nil when no
// --elo-history file was provided (or the file failed to parse cleanly).
// computeMetaPositioning reads this directly so the empirical blend is
// transparent to callers — no second parameter to thread through.
var LoadedMatchupHistory *MetaMatchupHistory

// ELO differentials per (rating, strength) pair. Calibrated so the
// derived win % matches the strength comment band on matchupEntry:
//
//	strong   → ±200 ELO → 76% / 24%
//	moderate → ±100 ELO → 64% / 36%
//	slight   → ±50  ELO → 57% / 43%
//	neutral  →   0 ELO → 50% / 50%
//
// Using ELO math (rather than hardcoded ints) means downstream tools
// that want to blend external ratings can stay on the same scale.
const (
	eloDiffStrong   = 200.0
	eloDiffModerate = 100.0
	eloDiffSlight   = 50.0
)

// Tilt detection parameters. A matchup is "tilted" when (a) we have
// enough observed games to trust the empirical rate AND (b) the
// empirical rate diverges from the ELO baseline by at least this many
// percentage points. 20pp is a deliberate "this matters to coaching"
// threshold — 10pp drifts on small samples are noise.
const (
	tiltMinSampleSize = 12
	tiltThresholdPP   = 20
)

// Empirical-blend weights. Computed inline in blendEmpirical, but
// surfacing the boundaries as constants makes the calibration
// auditable. 50+ games is the "full empirical weight" threshold;
// below that we shrink toward the ELO prior.
const (
	emShrinkLow  = 10  // <10 games: ignore empirical entirely
	emShrinkHigh = 50  // >=50 games: 80% empirical, 20% prior
	emMaxWeight  = 0.8 // ceiling on empirical weight
)

// techSuggestionCap bounds how many tech cards the recommender surfaces
// per worst-matchup analysis. 5 keeps the report scannable; severity
// sort ensures the critical hosers come first.
const techSuggestionCap = 5

// expectedWinPctForMatchup converts (rating, strength) into a 0-100
// win-rate prediction via ELO math. Neutral always returns 50. The
// strength dimension can carry "" (which metaMatchupStrengthOrDefault
// normalizes to "moderate" for directional ratings).
func expectedWinPctForMatchup(rating, strength string) int {
	if rating == "neutral" {
		return 50
	}
	diff := eloDiffModerate
	switch strings.ToLower(strength) {
	case "strong":
		diff = eloDiffStrong
	case "slight":
		diff = eloDiffSlight
	case "moderate", "":
		diff = eloDiffModerate
	}
	if rating == "unfavored" {
		diff = -diff
	}
	// Standard ELO expected-score formula. d>0 means we're favored.
	score := 1.0 / (1.0 + math.Pow(10, -diff/400.0))
	pct := int(math.Round(score * 100))
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct
}

// blendEmpirical merges the ELO baseline with observed history. Returns
// the blended win % plus the EmpiricalGames + EmpiricalWinPct populated
// from the history record (zeros when no history is present for this
// pair).
func blendEmpirical(baseline int, deckArch, oppArch string, history *MetaMatchupHistory) (blended, games, empiricalPct int) {
	if history == nil || history.Pairs == nil {
		return baseline, 0, 0
	}
	key := matchupHistoryKey(deckArch, oppArch)
	rec, ok := history.Pairs[key]
	if !ok || rec.Games <= 0 {
		return baseline, 0, 0
	}
	empirical := int(math.Round(100.0 * float64(rec.WinsForFirst) / float64(rec.Games)))
	if empirical < 0 {
		empirical = 0
	}
	if empirical > 100 {
		empirical = 100
	}
	weight := empiricalWeight(rec.Games)
	blendedFloat := float64(empirical)*weight + float64(baseline)*(1.0-weight)
	return int(math.Round(blendedFloat)), rec.Games, empirical
}

// empiricalWeight interpolates the empirical-vs-prior mix as the sample
// size grows. Linear ramp from emShrinkLow → emShrinkHigh; flat at 0
// below the low bound and at emMaxWeight above the high bound.
func empiricalWeight(games int) float64 {
	if games < emShrinkLow {
		return 0
	}
	if games >= emShrinkHigh {
		return emMaxWeight
	}
	span := float64(emShrinkHigh - emShrinkLow)
	progress := float64(games-emShrinkLow) / span
	return emMaxWeight * progress
}

// matchupHistoryKey is the canonical key shape stored in
// MetaMatchupHistory.Pairs. Both sides are run through
// canonicaliseMetaArchetypeKey so the history JSON can be authored
// with any archetype spelling that resolves to a canonical key.
func matchupHistoryKey(deckArch, oppArch string) string {
	left := canonicaliseMetaArchetypeKey(deckArch)
	right := canonicaliseMetaArchetypeKey(oppArch)
	return left + "|" + right
}

// detectTilt flags matchups whose empirical win rate diverges from the
// ELO baseline by tiltThresholdPP or more, given a sufficient sample.
// direction is "over" (we're winning more than the baseline expects)
// or "under" (less). delta is the signed pp gap (empirical-baseline).
func detectTilt(baselinePct, empiricalPct, games int) (tilted bool, direction string, delta int) {
	if games < tiltMinSampleSize {
		return false, "", 0
	}
	delta = empiricalPct - baselinePct
	abs := delta
	if abs < 0 {
		abs = -abs
	}
	if abs < tiltThresholdPP {
		return false, "", delta
	}
	if delta > 0 {
		return true, "over", delta
	}
	return true, "under", delta
}

// LoadMetaMatchupHistory reads an MetaMatchupHistory JSON from disk and
// installs it as the package-level LoadedMatchupHistory. Missing files
// are tolerated (returns nil err so callers can call this unconditionally
// at startup); bad JSON returns the parse error so main.go can log it.
func LoadMetaMatchupHistory(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read elo-history %s: %w", path, err)
	}
	var h MetaMatchupHistory
	if err := json.Unmarshal(data, &h); err != nil {
		return fmt.Errorf("parse elo-history %s: %w", path, err)
	}
	if h.Pairs == nil {
		h.Pairs = map[string]MatchupRecord{}
	}
	LoadedMatchupHistory = &h
	return nil
}

// archetypeToHoserConditions maps an opponent archetype to the hoserDB
// condition tags that describe what hoses that archetype. Reverse of
// the existing VulnerableTo / threat-assessment direction: there we
// asked "what conditions does THIS deck satisfy → what hosers fear
// us?". Here we ask "what conditions does the OPPONENT satisfy → what
// hosers can I run?". The list is the same data set viewed from the
// opposite angle.
//
// Multiple conditions per archetype are common (Aristocrats trips
// both etb_heavy AND creature_heavy; Storm trips storm_explicit AND
// spellslinger). The recommender de-dupes by card name afterward.
func archetypeToHoserConditions(opp string) []string {
	switch canonicaliseMetaArchetypeKey(opp) {
	case "reanimator":
		return []string{"reanimator", "graveyard_heavy"}
	case "aristocrats":
		return []string{"etb_heavy", "creature_heavy"}
	case "combo":
		return []string{"combo_heavy", "tutor_heavy"}
	case "storm":
		return []string{"storm_explicit", "spellslinger", "combo_heavy"}
	case "aggro":
		return []string{"token_heavy", "creature_heavy"}
	case "voltron":
		return []string{"commander_centric", "creature_heavy"}
	case "control":
		return []string{} // generic interaction; no archetype-specific tech
	case "stax":
		return []string{} // stax is itself the hate-piece engine
	case "enchantress":
		return []string{"enchantment_heavy"}
	case "midrange":
		return []string{}
	}
	return nil
}

// suggestTechCardsForArchetype returns the top hoserDB cards that
// target the opponent archetype. Already-running cards (present in
// report.Profiles) are still returned but tagged AlreadyInDeck=true,
// so the JSON consumer can render them as "confirmed picks" rather
// than dropping them. Severity-descending sort with stable
// alphabetic tiebreaker; capped at techSuggestionCap.
func suggestTechCardsForArchetype(opp string, report *FreyaReport) []TechCardSuggestion {
	conditions := archetypeToHoserConditions(opp)
	if len(conditions) == 0 {
		return nil
	}
	condSet := map[string]bool{}
	for _, c := range conditions {
		condSet[c] = true
	}

	inDeck := cardsInDeck(report)

	seen := map[string]bool{}
	var out []TechCardSuggestion
	for _, h := range hoserDB {
		if !condSet[h.Condition] {
			continue
		}
		if seen[h.Hoser] {
			continue
		}
		seen[h.Hoser] = true
		out = append(out, TechCardSuggestion{
			Card:              h.Hoser,
			Reason:            h.Reason,
			OpponentArchetype: opp,
			Severity:          h.Severity,
			AlreadyInDeck:     inDeck[strings.ToLower(h.Hoser)],
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		// Confirmed picks (AlreadyInDeck=true) at same severity should
		// sort AFTER missing picks so the actionable adds surface first.
		if out[i].AlreadyInDeck != out[j].AlreadyInDeck {
			return !out[i].AlreadyInDeck
		}
		return out[i].Card < out[j].Card
	})

	if len(out) > techSuggestionCap {
		out = out[:techSuggestionCap]
	}
	return out
}

// cardsInDeck builds a lowercase name set from report.Profiles for
// the AlreadyInDeck check. Returns an empty map when report is nil
// or has no profiles loaded (partial-test cases) so the caller's
// lookup stays safe.
func cardsInDeck(report *FreyaReport) map[string]bool {
	out := map[string]bool{}
	if report == nil {
		return out
	}
	for _, p := range report.Profiles {
		out[strings.ToLower(p.Name)] = true
	}
	return out
}

// worstMatchupArchetype returns the archetype label tied to the
// lowest ExpectedWinPct in the matchup list, or empty when the list
// is empty / all matchups score >= worstMatchupCeiling. The ceiling
// (default 50) means tech advice only kicks in when there's a real
// gap to close — a deck that's at 55% vs every opponent doesn't need
// hosers, it's already fine.
const worstMatchupCeiling = 50

func worstMatchupArchetype(matchups []MetaMatchup) string {
	worst := -1
	worstPct := 101
	for i, m := range matchups {
		if m.ExpectedWinPct < worstPct {
			worstPct = m.ExpectedWinPct
			worst = i
		}
	}
	if worst < 0 || worstPct >= worstMatchupCeiling {
		return ""
	}
	return matchups[worst].Archetype
}
