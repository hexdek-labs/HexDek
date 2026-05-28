package main

import (
	"fmt"
	"sort"
	"strings"
)

// DeckTwinSuggestion is the "your deck is N% precon X — these are
// the cards you added and removed" identity narrative. Combines
// the canonical-precon-overlap signal (PR #711) with set-difference
// computation to surface the explicit delta from a stock-precon
// baseline.
//
// Contrast with SuggestedChanges (PR #721): SuggestedChanges
// recommends what the deck SHOULD do based on bracket-scaled
// targets; DeckTwinSuggestion describes what the deck IS based on
// its closest stock-precon ancestor. The two are complementary —
// the twin signal is the deck's identity ("Sigarda Heron's Grace
// precon with 13 upgrades"); the suggestion signal is its
// improvement runway ("you still need 4 more interaction").
//
// Use cases:
//   - Decks-screen identity blurb: "Your deck is 87% the Sigarda
//     Heron's Grace precon. The 13% delta is 13 cards added
//     (Smothering Tithe, Land Tax, Heliod's Intervention, ...)
//     and 8 cards removed (Open the Armory, Boon Reflection, ...)."
//   - Onboarding: a new user uploading a barely-upgraded precon
//     gets immediate "we recognize this — here's what's changed"
//     framing rather than treating it as a fully novel build.
//   - Upgrade-path UI: the cards a player REMOVED from the stock
//     precon are clues about what they didn't like; the cards
//     they ADDED are clues about their actual play preferences.
//     Both signals inform better future recommendations.
//
// Always non-nil when returned; the .HasTwin field flags whether
// a precon match was found (false when the commander doesn't ship
// in any precon, e.g. a fresh-spoilers commander or a custom one).
// The narrative string handles both cases ("no precon twin
// available" vs the full identity blurb).
type DeckTwinSuggestion struct {
	// HasTwin is true when a precon with the same commander was
	// found in the corpus. When false, the rest of the fields
	// hold zero values and Narrative carries a "no twin" message.
	HasTwin bool

	// PreconName is the canonical precon's deck-file name (e.g.
	// "blessed_sanctuary_innistrad_crimson_vow_commander_precon_decklist").
	// Empty when HasTwin is false.
	PreconName string

	// PreconCommander is the commander the twin precon was built
	// around — same as the user's commander since the match
	// criterion is commander-equality.
	PreconCommander string

	// SimilarityPercent is the SharedPercent from the underlying
	// PreconOverlap — what fraction of the user's nonland slots
	// are still stock-precon. Reads as "87% precon".
	SimilarityPercent float64

	// DeltaPercent is 100 - SimilarityPercent — the percentage
	// upgrade signature. Reads as "13% delta from precon".
	DeltaPercent float64

	// Added is the alphabetized list of cards in the user's deck
	// that are NOT in the precon — the upgrades the user made
	// (or the slots they swapped for personal preference).
	// Lower-cased keys preserved from the fingerprint; UIs that
	// want display-case names should re-lookup via the oracle.
	Added []string

	// Removed is the alphabetized list of cards in the PRECON
	// that are NOT in the user's deck — the stock slots the user
	// CUT. Useful signal for "they took out the precon's draw
	// engine, so they probably prefer X over Y" inference.
	Removed []string

	// SharedCount mirrors the underlying PreconOverlap value —
	// the nonland intersection size.
	SharedCount int

	// AddedCount / RemovedCount are |Added| / |Removed| respectively
	// — surfaced as scalars so UIs can short-circuit "no changes"
	// rendering without walking the slices.
	AddedCount   int
	RemovedCount int

	// Narrative is the rendered identity blurb suitable for the
	// Decks screen. Format:
	//
	//   "Your deck is 87% the Sigarda Heron's Grace precon. The
	//    13% delta is 13 cards added (Smothering Tithe, ...) and
	//    8 precon cards removed (Open the Armory, ...)."
	//
	// Card lists in the narrative are capped at 5 names to keep
	// the blurb readable; Added / Removed carry the full lists.
	Narrative string
}

// BuildDeckTwinSuggestion produces the deck-twin identity rollup
// for a target deck against a precon corpus. Wraps
// FindCanonicalPreconOverlap (PR #711) and adds the set-difference
// computation that surfaces the explicit delta cards.
//
// Inputs:
//   - target: the user's deck fingerprint (BuildDeckFingerprint on
//     a completed analysis)
//   - precons: the precon corpus fingerprints (same shape as the
//     canonical-precon-overlap pass in main.go's all-decks mode)
//
// Returns a non-nil struct in all cases. When no twin is found
// (commander doesn't ship in any precon), HasTwin=false and
// Narrative reads "No canonical precon ships {commander} — deck
// has no stock-precon ancestor."
func BuildDeckTwinSuggestion(target *DeckFingerprint, precons []*DeckFingerprint) *DeckTwinSuggestion {
	out := &DeckTwinSuggestion{}
	if target == nil {
		out.Narrative = "No deck fingerprint — twin lookup skipped."
		return out
	}
	overlap := FindCanonicalPreconOverlap(target, precons)
	if overlap == nil {
		commanderLabel := strings.TrimSpace(target.Commander)
		if commanderLabel == "" {
			commanderLabel = "this commander"
		}
		out.Narrative = fmt.Sprintf("No canonical precon ships %s — deck has no stock-precon ancestor.",
			commanderLabel)
		return out
	}

	// Locate the matched precon fingerprint by name so we can do
	// set-difference math. FindCanonicalPreconOverlap returns the
	// summary but not the fingerprint itself; re-find by name.
	var twin *DeckFingerprint
	for _, p := range precons {
		if p != nil && p.Name == overlap.PreconName {
			twin = p
			break
		}
	}
	if twin == nil {
		// Defensive: shouldn't happen since overlap was non-nil,
		// but if precons mutated between calls or there's a name-
		// collision edge case, render the overlap signal without
		// the delta lists.
		out.HasTwin = true
		out.PreconName = overlap.PreconName
		out.PreconCommander = overlap.PreconCommander
		out.SimilarityPercent = overlap.SharedPercent
		out.DeltaPercent = 100.0 - overlap.SharedPercent
		out.SharedCount = overlap.SharedCount
		out.Narrative = fmt.Sprintf("Your deck is %.0f%% the %s precon (twin fingerprint unavailable for delta listing).",
			overlap.SharedPercent, preconDisplayName(overlap.PreconName))
		return out
	}

	// Set differences over the nonland fingerprints.
	added := stringSetDiff(target.NonlandCards, twin.NonlandCards)
	removed := stringSetDiff(twin.NonlandCards, target.NonlandCards)
	sort.Strings(added)
	sort.Strings(removed)

	out.HasTwin = true
	out.PreconName = overlap.PreconName
	out.PreconCommander = overlap.PreconCommander
	out.SimilarityPercent = overlap.SharedPercent
	out.DeltaPercent = 100.0 - overlap.SharedPercent
	out.SharedCount = overlap.SharedCount
	out.Added = added
	out.Removed = removed
	out.AddedCount = len(added)
	out.RemovedCount = len(removed)
	out.Narrative = buildDeckTwinNarrative(out)
	return out
}

// stringSetDiff returns the sorted slice of keys present in `a`
// but not in `b`. Operates on map[string]bool which is the
// NonlandCards shape; values are ignored.
func stringSetDiff(a, b map[string]bool) []string {
	out := make([]string, 0, len(a))
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	return out
}

// buildDeckTwinNarrative renders the identity blurb. Card lists
// in the narrative are capped at 5 names per side ("Smothering
// Tithe, Land Tax, ... and 8 more") so the blurb stays readable
// even on a heavy-delta deck; full lists live in Added / Removed.
func buildDeckTwinNarrative(t *DeckTwinSuggestion) string {
	if t == nil || !t.HasTwin {
		return ""
	}
	preconLabel := preconDisplayName(t.PreconName)
	if t.AddedCount == 0 && t.RemovedCount == 0 {
		return fmt.Sprintf("Your deck is identical to the %s precon (100%% match — no upgrades or cuts).",
			preconLabel)
	}
	addedFrag := ""
	if t.AddedCount > 0 {
		addedFrag = fmt.Sprintf("%d cards added (%s)",
			t.AddedCount, sampleList(t.Added, 5))
	}
	removedFrag := ""
	if t.RemovedCount > 0 {
		removedFrag = fmt.Sprintf("%d precon cards removed (%s)",
			t.RemovedCount, sampleList(t.Removed, 5))
	}
	deltaFrag := ""
	switch {
	case addedFrag != "" && removedFrag != "":
		deltaFrag = addedFrag + " and " + removedFrag
	case addedFrag != "":
		deltaFrag = addedFrag
	case removedFrag != "":
		deltaFrag = removedFrag
	}
	return fmt.Sprintf("Your deck is %.0f%% the %s precon. The %.0f%% delta is %s.",
		t.SimilarityPercent, preconLabel, t.DeltaPercent, deltaFrag)
}

// sampleList renders the first `cap` names from `items`, comma-
// separated, with "... and N more" appended when the slice exceeds
// the cap. Title-cases each name (the fingerprint keys are
// lowercased; the narrative should read naturally).
func sampleList(items []string, capN int) string {
	if len(items) == 0 {
		return ""
	}
	n := capN
	if len(items) < n {
		n = len(items)
	}
	rendered := make([]string, n)
	for i := 0; i < n; i++ {
		rendered[i] = titleCaseCardName(items[i])
	}
	out := strings.Join(rendered, ", ")
	if len(items) > capN {
		out += fmt.Sprintf(", ... and %d more", len(items)-capN)
	}
	return out
}

// titleCaseCardName converts the lowercased fingerprint key
// ("smothering tithe") to a display-readable form ("Smothering
// Tithe"). The fingerprint normalization in BuildDeckFingerprint
// lowercases everything for matching; the narrative needs the
// human-readable rendering.
//
// Naive title-casing — capitalizes the first letter of each
// whitespace-separated word. Card names with apostrophes
// (Yawgmoth's Will) or hyphens (Stax-of-the-month) are not
// special-cased; the result is "good enough" for UI display
// and any consumer needing canonical capitalization should
// re-resolve via the oracle.
func titleCaseCardName(s string) string {
	if s == "" {
		return ""
	}
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		runes := []rune(w)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// preconDisplayName converts a precon file-name slug
// ("blessed_sanctuary_innistrad_crimson_vow_commander_precon_decklist.txt")
// into a friendlier display string. Drops the trailing
// "_commander_precon_decklist[.txt]" suffix and the trailing
// set-name segment when one is recognizable, then title-cases the
// remainder. Falls back to the raw slug on edge cases (custom
// precons added to the corpus with non-standard names).
func preconDisplayName(slug string) string {
	if slug == "" {
		return "precon"
	}
	s := strings.TrimSuffix(slug, ".txt")
	s = strings.TrimSuffix(s, "_commander_precon_decklist")
	s = strings.TrimSuffix(s, "_precon_decklist")
	// Keep the deck name portion — typically the first 2-4 underscore-
	// separated tokens before the set tag. Don't over-trim; the set
	// tag adds useful disambiguation ("Sigarda" vs "Sigarda Heron's
	// Grace" — different precons).
	if s == "" {
		return slug
	}
	tokens := strings.Split(s, "_")
	for i, t := range tokens {
		if t == "" {
			continue
		}
		runes := []rune(t)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		tokens[i] = string(runes)
	}
	return strings.Join(tokens, " ")
}
