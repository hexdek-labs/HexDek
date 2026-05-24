package main

import (
	"strings"
	"testing"
)

// archetypeLabelToKey maps display-case vsArchetype labels (used as
// keys into MetaMatchup.Archetype) back to the lowercase DB keys.
// Only labels naming a real archetype need to be present — meta-call
// labels like "Graveyard Hate" / "Enchantment Hate" / "Token/Go Wide"
// don't have inverse entries and are excluded from reciprocity checks.
var archetypeLabelToKey = map[string]string{
	"Aggro":       "aggro",
	"Combo":       "combo",
	"Control":     "control",
	"Aristocrats": "aristocrats",
	"Voltron":     "voltron",
	"Stax":        "stax",
	"Reanimator":  "reanimator",
	"Storm":       "storm",
	"Enchantress": "enchantress",
	"Midrange":    "midrange",
}

// TestMetaMatchups_ReciprocityInvariant pins the structural invariant
// that if archetype A predicts itself favored vs B, then B must predict
// itself unfavored vs A (and vice versa). Neutral-vs-neutral is the
// only same-rating pair allowed. Catches the asymmetric drift that the
// pre-R60 DB had — combo claimed it beat midrange while midrange's own
// entry only listed unfavored matchups, never naming combo as the
// favored side from midrange's perspective.
func TestMetaMatchups_ReciprocityInvariant(t *testing.T) {
	type pair struct{ a, b string }
	checked := map[pair]bool{}

	for archA, matchups := range metaMatchupDB {
		for _, m := range matchups {
			archB, ok := archetypeLabelToKey[m.vsArchetype]
			if !ok {
				// Meta-call label (e.g. "Graveyard Hate") — no
				// reciprocal entry expected.
				continue
			}
			if archA == archB {
				t.Errorf("%s lists itself as a matchup — self-reciprocity not meaningful", archA)
				continue
			}
			p := pair{archA, archB}
			if checked[p] {
				continue
			}
			checked[p] = true

			// Look up the reciprocal entry.
			reciprocal := metaMatchupDB[archB]
			var inverse *matchupEntry
			for i := range reciprocal {
				if archetypeLabelToKey[reciprocal[i].vsArchetype] == archA {
					inverse = &reciprocal[i]
					break
				}
			}
			if inverse == nil {
				t.Errorf("reciprocity gap: %s→%s exists (%s) but %s→%s missing",
					archA, archB, m.rating, archB, archA)
				continue
			}
			// favored ↔ unfavored, unfavored ↔ favored, neutral ↔ neutral.
			switch m.rating {
			case "favored":
				if inverse.rating != "unfavored" {
					t.Errorf("reciprocity violation: %s→%s=favored but %s→%s=%q (want unfavored)",
						archA, archB, archB, archA, inverse.rating)
				}
			case "unfavored":
				if inverse.rating != "favored" {
					t.Errorf("reciprocity violation: %s→%s=unfavored but %s→%s=%q (want favored)",
						archA, archB, archB, archA, inverse.rating)
				}
			case "neutral":
				if inverse.rating != "neutral" {
					t.Errorf("reciprocity violation: %s→%s=neutral but %s→%s=%q (want neutral)",
						archA, archB, archB, archA, inverse.rating)
				}
			default:
				t.Errorf("%s→%s has invalid rating %q", archA, archB, m.rating)
			}
		}
	}
}

// TestMetaMatchups_RatingValidity asserts every rating is one of the
// three sanctioned values. Catches typos at test time rather than
// having the consumer (deckprofile renderer) silently swallow a
// "favored " (trailing space) or "favoured" (British spelling).
func TestMetaMatchups_RatingValidity(t *testing.T) {
	valid := map[string]bool{"favored": true, "neutral": true, "unfavored": true}
	for arch, matchups := range metaMatchupDB {
		for _, m := range matchups {
			if !valid[m.rating] {
				t.Errorf("%s vs %s: invalid rating %q (want favored/neutral/unfavored)",
					arch, m.vsArchetype, m.rating)
			}
		}
	}
}

// TestMetaMatchups_CoverageDepth asserts every archetype in the DB has
// at least 4 matchups. Pre-R60 several archetypes (voltron, reanimator,
// enchantress, midrange, aggro) had only 3, leaving big blind spots in
// the position report.
func TestMetaMatchups_CoverageDepth(t *testing.T) {
	const minMatchups = 4
	for arch, matchups := range metaMatchupDB {
		if len(matchups) < minMatchups {
			t.Errorf("%s has only %d matchups, want >=%d",
				arch, len(matchups), minMatchups)
		}
	}
}

// TestMetaMatchups_NoDuplicateOpponents asserts no archetype's matchup
// list names the same opponent twice — duplicates would silently
// generate inconsistent MetaMatchup entries in the rendered report.
func TestMetaMatchups_NoDuplicateOpponents(t *testing.T) {
	for arch, matchups := range metaMatchupDB {
		seen := map[string]bool{}
		for _, m := range matchups {
			if seen[m.vsArchetype] {
				t.Errorf("%s lists %s twice", arch, m.vsArchetype)
			}
			seen[m.vsArchetype] = true
		}
	}
}

// TestComputeMetaPositioning_KnownPairings pins specific high-value
// matchup outputs so future DB edits don't silently drift them. Each
// case is an archetype + an expected opponent + the rating + a
// substring the reason MUST contain. The substring assertions name
// canonical cards so a reason rewrite that drops the named mechanic
// (the sharpening the R60 refinement specifically added) fails loudly.
func TestComputeMetaPositioning_KnownPairings(t *testing.T) {
	type expectation struct {
		archetype       string
		vsArch          string
		wantRating      string
		wantReasonHas   string
	}
	cases := []expectation{
		{"Aggro", "Control", "unfavored", "Cyclonic Rift"},
		{"Aggro", "Combo", "favored", "fast clock"},
		{"Combo", "Stax", "unfavored", "Drannith Magistrate"},
		{"Control", "Storm", "favored", "counterspells"},
		{"Control", "Voltron", "favored", "Path to Exile"},
		{"Stax", "Storm", "favored", "Rule of Law"},
		{"Stax", "Combo", "favored", "Drannith Magistrate"},
		{"Reanimator", "Graveyard Hate", "unfavored", "Rest in Peace"},
		{"Voltron", "Control", "unfavored", "Path to Exile"},
		{"Aristocrats", "Graveyard Hate", "unfavored", "Rest in Peace"},
		{"Storm", "Control", "unfavored", "counterspells"},
		{"Enchantress", "Enchantment Hate", "unfavored", "Aura Shards"},
		{"Midrange", "Storm", "unfavored", "midrange clock"},
	}
	for _, tc := range cases {
		dp := &DeckProfile{PrimaryArchetype: tc.archetype}
		computeMetaPositioning(dp)
		var hit *MetaMatchup
		for i := range dp.MetaMatchups {
			if dp.MetaMatchups[i].Archetype == tc.vsArch {
				hit = &dp.MetaMatchups[i]
				break
			}
		}
		if hit == nil {
			t.Errorf("%s deck: expected matchup vs %s, none found in %v",
				tc.archetype, tc.vsArch, dp.MetaMatchups)
			continue
		}
		if hit.Rating != tc.wantRating {
			t.Errorf("%s vs %s: want rating %q, got %q",
				tc.archetype, tc.vsArch, tc.wantRating, hit.Rating)
		}
		if !strings.Contains(strings.ToLower(hit.Reason), strings.ToLower(tc.wantReasonHas)) {
			t.Errorf("%s vs %s: reason %q missing substring %q",
				tc.archetype, tc.vsArch, hit.Reason, tc.wantReasonHas)
		}
	}
}

// TestComputeMetaPositioning_ArchetypeNormalisation asserts the
// archetype-name canonicalisation switch in computeMetaPositioning —
// "Go Wide Tokens" should map to aggro, "Infinite Combo" to combo,
// "Spellslinger" to storm, etc. Catches regressions where new
// archetype strings emitted by Freya don't reach a matchup list.
func TestComputeMetaPositioning_ArchetypeNormalisation(t *testing.T) {
	cases := []struct {
		input         string
		expectVsArch  string // an opponent the resolved bucket should match against
	}{
		{"Go Wide Tokens", "Control"},   // resolves to aggro → has Control matchup
		{"Tribal Aggro", "Combo"},       // resolves to aggro → has Combo matchup
		{"Infinite Combo", "Stax"},      // resolves to combo → has Stax matchup
		{"Spellslinger", "Stax"},        // resolves to storm → has Stax matchup
		{"Discard Control", "Aggro"},    // resolves to control → has Aggro matchup
		{"Random Made Up Archetype", "Storm"}, // falls through to midrange → has Storm
	}
	for _, tc := range cases {
		dp := &DeckProfile{PrimaryArchetype: tc.input}
		computeMetaPositioning(dp)
		found := false
		for _, m := range dp.MetaMatchups {
			if m.Archetype == tc.expectVsArch {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("archetype %q should normalise to a bucket with %q matchup, got: %v",
				tc.input, tc.expectVsArch, dp.MetaMatchups)
		}
	}
}
