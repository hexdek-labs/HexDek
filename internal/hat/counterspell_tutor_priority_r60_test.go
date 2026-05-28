package hat

import (
	"testing"
)

// counterspell_tutor_priority_r60_test.go — regressions for the
// combo-tutor classifier in ChooseResponse. Pre-r60 the
// `search your library && score >= 2` gate let cheap 1-CMC tutors
// (Vampiric, Mystical, Imperial Seal, Worldly) slip past the
// mustCounter check. Tutors are higher-leverage than equivalent-cost
// removal — they compound across turns and become the wincon they
// fetched — so the new classifier elevates them above the score gate.
// See poker.go::isComboTutorOracle + yggdrasil.go::ChooseResponse.
//
// Helpers (setupResponder, spellWithOracle) come from
// priority_round_r60_test.go.

// -----------------------------------------------------------------------------
// isComboTutorOracle — unit
// -----------------------------------------------------------------------------

func TestIsComboTutorOracle_OpenTutorMatches(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
	}{
		{"Demonic Tutor", "search your library for a card, put it into your hand, then shuffle."},
		{"Vampiric Tutor", "search your library for a card, then shuffle, then put that card on top of your library."},
		{"Imperial Seal", "search your library for a card, then shuffle, then put that card on top of your library."},
		{"Diabolic Intent", "as an additional cost, sacrifice a creature. search your library for a card, put it into your hand, then shuffle."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !isComboTutorOracle(c.oracle) {
				t.Errorf("open tutor %q should match isComboTutorOracle", c.name)
			}
		})
	}
}

func TestIsComboTutorOracle_TypedTutorsMatch(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
	}{
		{"Mystical Tutor", "search your library for an instant or sorcery card, reveal it, then shuffle and put that card on top of your library."},
		{"Worldly Tutor", "search your library for a creature card, reveal it, then shuffle and put that card on top of your library."},
		{"Enlightened Tutor", "search your library for an artifact or enchantment card, reveal it, then shuffle and put that card on top of your library."},
		{"Idyllic Tutor", "search your library for an enchantment card, reveal it, put it into your hand, then shuffle."},
		{"Fabricate", "search your library for an artifact card, reveal it, put it into your hand, then shuffle."},
		{"Heliod's Pilgrim", "when this enters, search your library for an aura card, reveal it, put it into your hand, then shuffle."},
		{"Call the Gatewatch", "search your library for a planeswalker card, reveal it, put it into your hand, then shuffle."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !isComboTutorOracle(c.oracle) {
				t.Errorf("typed tutor %q should match", c.name)
			}
		})
	}
}

func TestIsComboTutorOracle_RampTutorsExcluded(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
	}{
		{"Cultivate", "search your library for up to two basic land cards, reveal those cards, put one onto the battlefield tapped and the other into your hand, then shuffle."},
		{"Kodama's Reach", "search your library for up to two basic land cards, reveal them, put one onto the battlefield tapped and the other into your hand, then shuffle."},
		{"Nature's Lore", "search your library for a forest card, put that card onto the battlefield, then shuffle."},
		{"Three Visits", "search your library for a forest or plains card, put it onto the battlefield, then shuffle."},
		{"Crop Rotation", "as an additional cost, sacrifice a land. search your library for a land card, put that card onto the battlefield, then shuffle."},
		{"Rampant Growth", "search your library for a basic land card, put it onto the battlefield tapped, then shuffle."},
		{"Skyshroud Claim", "search your library for up to two forest cards, put them onto the battlefield, then shuffle."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if isComboTutorOracle(c.oracle) {
				t.Errorf("ramp tutor %q must NOT match — burning counters on mana fixing is the bug we're avoiding", c.name)
			}
		})
	}
}

func TestIsComboTutorOracle_NonTutorReturnsFalse(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
	}{
		{"Counterspell", "counter target spell."},
		{"Doom Blade", "destroy target nonblack creature."},
		{"Brainstorm", "draw three cards, then put two cards from your hand on top of your library in any order."},
		{"empty", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if isComboTutorOracle(c.oracle) {
				t.Errorf("non-tutor %q must not match", c.name)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// ChooseResponse: tutor vs removal at equal CMC
// -----------------------------------------------------------------------------

// Headline ask: a 1-mana combo tutor and a 1-mana targeted removal
// (Swords to Plowshares) cost the same to counter, but the tutor is
// strictly higher leverage. Pre-r60 the hat would counter neither
// (both score 1, below midrange minScore=3); post-r60 it counters the
// tutor and passes on the equally-cheap removal — matching the
// "counter a tutor over counter a removal" intuition.
func TestChooseResponse_R60_TutorMustCounter_RemovalDoesNot(t *testing.T) {
	// 1-mana tutor case: mustCounter via the new classifier.
	h, gs, top := setupResponder(t,
		spellWithOracle("Vampiric Tutor", 1,
			"Search your library for a card, then shuffle, then put that card on top of your library."))
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("Vampiric Tutor (1-mana combo tutor) must mustCounter")
	}

	// 1-mana removal case: score=1, NO mustCounter trigger, midrange
	// minScore=3 → hat passes. Different scenario instance to avoid
	// state leak between calls.
	h2, gs2, top2 := setupResponder(t,
		spellWithOracle("Swords to Plowshares", 1,
			"Exile target creature. Its controller gains life equal to its power."))
	if got := h2.ChooseResponse(gs2, 0, top2); got != nil {
		t.Fatalf("equally-cheap removal (Swords) should NOT counter under default midrange minScore; got %v",
			got.Card.DisplayName())
	}
}

// 2-mana tutor (Mystical Tutor) still mustCounters via the new
// classifier — the score-based legacy path would also fire at score=2,
// but we want the classifier path to take precedence so deck-specific
// counter strategies don't regress with future score-gate tuning.
func TestChooseResponse_R60_TwoManaTutorStillMustCounters(t *testing.T) {
	h, gs, top := setupResponder(t,
		spellWithOracle("Mystical Tutor", 2,
			"Search your library for an instant or sorcery card, reveal it, then shuffle and put that card on top of your library."))
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("Mystical Tutor (2-mana combo tutor) should still mustCounter")
	}
}

// Expensive ramp tutor (Skyshroud Claim 4-mana) falls through the
// combo classifier but the legacy score gate STILL fires at score 4
// >= 2. Pinned so the legacy path isn't accidentally removed when the
// combo classifier is in place.
func TestChooseResponse_R60_ExpensiveRampTutorStillCountersViaLegacyGate(t *testing.T) {
	h, gs, top := setupResponder(t,
		spellWithOracle("Skyshroud Claim", 4,
			"Search your library for up to two forest cards, put them onto the battlefield, then shuffle."))
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("4-mana ramp tutor (Skyshroud Claim) should counter via the legacy score>=2 gate (combo classifier doesn't match ramp)")
	}
}
