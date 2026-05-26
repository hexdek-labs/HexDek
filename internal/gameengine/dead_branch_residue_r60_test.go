package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase-1D residue fixes: regression pins for the four engine-side
// behaviour changes / documentation upgrades. Each fix from the
// dead-branch-residue investigation needs a test that would catch a
// future regression — either via direct call (RestrictionAllows,
// matchesCounterFilter, findSacrificeCandidate) or via the canonical
// emitter that drives the affected switch.

// TestRestrictionAllows_LiveTagsStillWork pins the two live restriction
// tags (creature_spell_only via Food Chain, noncreature_or_artifact_activation
// via Powerstone tokens). Phase 1D dropped three formerly-defensive
// alias arms; this guards that the live tags themselves continue to
// route correctly through the simplified switch.
func TestRestrictionAllows_LiveTagsStillWork(t *testing.T) {
	cases := []struct {
		restriction string
		spellType   string
		want        bool
	}{
		// creature_spell_only: only creature spells pay.
		{"creature_spell_only", "creature", true},
		{"creature_spell_only", "instant", false},
		{"creature_spell_only", "sorcery", false},

		// noncreature_or_artifact_activation: noncreature spells AND
		// activated abilities.
		{"noncreature_or_artifact_activation", "noncreature", true},
		{"noncreature_or_artifact_activation", "activated", true},
		{"noncreature_or_artifact_activation", "instant", true},
		{"noncreature_or_artifact_activation", "sorcery", true},
		{"noncreature_or_artifact_activation", "creature", false},

		// artifact_only: artifact spells + activated abilities.
		{"artifact_only", "artifact", true},
		{"artifact_only", "activated", true},
		{"artifact_only", "creature", false},

		// Unknown restriction → conservative allow (covers any future
		// emitter that uses one of the three deleted alias spellings).
		{"non_creature_activation_only", "creature", true},
		{"noncreature_activation_only", "creature", true},
		{"instant_or_sorcery_only", "creature", true},
		{"any_brand_new_tag", "anything", true},

		// Empty restriction always allows.
		{"", "anything", true},
	}
	for _, c := range cases {
		if got := RestrictionAllows(c.restriction, c.spellType, false); got != c.want {
			t.Errorf("RestrictionAllows(%q, %q): got %v, want %v",
				c.restriction, c.spellType, got, c.want)
		}
	}
}

// TestMatchesCounterFilter_ActivatedAbilityAlias pins the defensive
// "activated_ability" alias kept in matchesCounterFilter. If the
// alternate spelling ever drops out of the grouped case arm, a
// Stifle-style counter targeting "activated_ability" would silently
// fail to match — caught here.
func TestMatchesCounterFilter_ActivatedAbilityAlias(t *testing.T) {
	abilityItem := &StackItem{Kind: "activated", Card: &Card{}}
	cases := []struct {
		filterBase string
		want       bool
	}{
		{"activated", true},
		{"activated_ability", true}, // pinned defensive alias
		{"triggered", false},
	}
	for _, c := range cases {
		got := matchesCounterFilter(abilityItem, gameast.Filter{Base: c.filterBase})
		if got != c.want {
			t.Errorf("matchesCounterFilter base=%q on activated ability: got %v want %v",
				c.filterBase, got, c.want)
		}
	}
}

// TestMatchesCounterFilter_EdgeCaseBaseValues pins "non" / "other" / "or"
// — the parser-fallback edge cases that route to "any spell" matching.
// A regression here would mean a counterspell with a weird Filter.Base
// silently fails to match instead of degrading to broad matching.
func TestMatchesCounterFilter_EdgeCaseBaseValues(t *testing.T) {
	spellItem := &StackItem{
		Kind: "spell",
		Card: &Card{Types: []string{"instant"}},
	}
	for _, base := range []string{"non", "other", "or"} {
		if !matchesCounterFilter(spellItem, gameast.Filter{Base: base}) {
			t.Errorf("matchesCounterFilter(base=%q) on spell should fall to broad-match true", base)
		}
	}
}

// TestFindSacrificeCandidate_GreenCreatureFilter pins the "green
// creature" arm Phase-1D flagged as unreachable. Currently no per_card
// handler emits this filter, but the AST parser COULD emit it (the
// docstring lists it as supported); without the arm, the default
// fallback would accept off-color creatures and break the cost.
func TestFindSacrificeCandidate_GreenCreatureFilter(t *testing.T) {
	gs := &GameState{Seats: []*Seat{{Idx: 0}}}
	seat := gs.Seats[0]

	greenCreature := &Permanent{
		Card:       &Card{Types: []string{"creature"}, Colors: []string{"G"}},
		Controller: 0,
	}
	blackCreature := &Permanent{
		Card:       &Card{Types: []string{"creature"}, Colors: []string{"B"}},
		Controller: 0,
	}

	// With only a black creature: no candidate.
	seat.Battlefield = []*Permanent{blackCreature}
	if got := findSacrificeCandidate(gs, seat, "green creature"); got != nil {
		t.Errorf("green-creature filter should reject black creature, got %+v", got.Card)
	}

	// With a green creature: it's the candidate.
	seat.Battlefield = []*Permanent{blackCreature, greenCreature}
	if got := findSacrificeCandidate(gs, seat, "green creature"); got != greenCreature {
		t.Errorf("green-creature filter should pick the green creature; got %+v",
			func() any {
				if got == nil {
					return nil
				}
				return got.Card
			}())
	}
}

// TestRestrictionAllows_UnknownTagConservativeAllow pins the deliberate
// fallback semantics — any restriction the function doesn't recognize
// returns true. The three deleted alias arms (non_creature_activation_only,
// noncreature_activation_only, instant_or_sorcery_only) hit this path
// after the cleanup; the test guards the conservative-allow shape that
// makes that cleanup safe.
func TestRestrictionAllows_UnknownTagConservativeAllow(t *testing.T) {
	if !RestrictionAllows("not_a_real_restriction", "creature", false) {
		t.Error("unknown restriction must conservatively allow (any spell type)")
	}
}
