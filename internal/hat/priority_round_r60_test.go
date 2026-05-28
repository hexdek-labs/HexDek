package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// priority_round_r60_test.go — regressions for the two R60 ChooseResponse
// signals: a "nothing meaningful to interrupt" early pass when the
// incoming spell is a low-impact cantrip-shaped effect, and eager
// mustCounter triggers for extra-turn spells and tutors.
//
// Pre-R60 the hat would burn a counter on any spell scoring above the
// archetype-tuned minScore gate, which made it spend Counterspell on a
// 1-mana Brainstorm while letting a 2-mana Mystic Tutor (CMC-2, score-2,
// minScore-3-ish) resolve unanswered. Both behaviors flipped this round.

// counterInHand builds a vanilla 2-cmc Counterspell card the
// ChooseResponse fast-path will accept.
func counterInHand() *gameengine.Card {
	return newTestCardMinimal("Counterspell", []string{"instant"}, 2,
		&gameast.CardAST{
			Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: &gameast.CounterSpell{
						Target: gameast.Filter{Base: "spell", Targeted: true},
					},
				},
			},
		})
}

// spellWithOracle builds a card whose Static ability raw text seeds the
// engine's OracleTextLower reconstruction. cmc controls ManaCostOf via
// the test-fixture "cost:N" type tag.
func spellWithOracle(name string, cmc int, oracle string) *gameengine.Card {
	return newTestCardMinimal(name, []string{"sorcery"}, cmc,
		&gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{&gameast.Static{Raw: oracle}},
		})
}

// setupResponder wires a YggdrasilHat onto seat 0 with the Counterspell
// in hand, mana to cast it, and the incoming spell on the stack from
// seat 1. Returns the hat + stack top so tests can call ChooseResponse.
func setupResponder(t *testing.T, incoming *gameengine.Card) (*YggdrasilHat, *gameengine.GameState, *gameengine.StackItem) {
	t.Helper()
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{counterInHand()}
	gs.Seats[0].ManaPool = 6
	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	top := &gameengine.StackItem{
		Controller: 1,
		Card:       incoming,
		Effect:     &gameast.Damage{},
	}
	return h, gs, top
}

// -----------------------------------------------------------------------------
// Signal A — low-impact pass
// -----------------------------------------------------------------------------

func TestChooseResponse_R60_PassesOnCantripDraw(t *testing.T) {
	h, gs, top := setupResponder(t, spellWithOracle("Opt", 1, "Scry 1, then draw a card."))
	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("hat should pass on a 1-mana cantrip; got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_PassesOnScrySpell(t *testing.T) {
	h, gs, top := setupResponder(t, spellWithOracle("Preordain", 1, "Scry 2. Draw a card."))
	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("hat should pass on a scry/draw cantrip; got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_DoesNotPassOnLowCMCRemoval(t *testing.T) {
	// "Destroy target creature" at 2 mana — looks cheap but is real
	// removal; the low-impact gate must not swallow it. Asserted on
	// the helper to keep the test independent of minScore tuning.
	card := spellWithOracle("Doom Blade", 2, "Destroy target nonblack creature.")
	if isLowImpactCantripSpell(card) {
		t.Fatalf("removal spell must not be classified as low-impact cantrip")
	}
}

func TestIsLowImpactCantripSpell_PositiveCases(t *testing.T) {
	cases := []struct {
		name   string
		cmc    int
		oracle string
	}{
		{"Opt", 1, "Scry 1, then draw a card."},
		{"Brainstorm", 1, "Draw a card."},
		{"Ponder", 1, "Look at the top three cards of your library."},
		{"Spectral Sailor cantrip", 2, "Scry 1."},
		{"Twiddle", 1, "Tap target permanent."},
	}
	for _, c := range cases {
		card := spellWithOracle(c.name, c.cmc, c.oracle)
		if !isLowImpactCantripSpell(card) {
			t.Errorf("%q should be classified low-impact (oracle=%q)", c.name, c.oracle)
		}
	}
}

func TestIsLowImpactCantripSpell_NegativeCases(t *testing.T) {
	cases := []struct {
		name   string
		cmc    int
		oracle string
	}{
		{"Doom Blade", 2, "Destroy target nonblack creature."},
		{"Swords to Plowshares", 1, "Exile target creature."},
		{"Lightning Bolt", 1, "Lightning Bolt deals 3 damage to any target."},
		{"Counterspell", 2, "Counter target spell."},
		{"Time Walk", 2, "Take an extra turn after this one."},
		// CMC-3 cantrip is too expensive to be considered trivial.
		{"Big draw", 3, "Draw two cards."},
		// Empty AST → no signal either way.
		{"Vanilla", 1, ""},
	}
	for _, c := range cases {
		card := spellWithOracle(c.name, c.cmc, c.oracle)
		if isLowImpactCantripSpell(card) {
			t.Errorf("%q should NOT be low-impact (oracle=%q)", c.name, c.oracle)
		}
	}
}

func TestIsLowImpactCantripSpell_NilSafe(t *testing.T) {
	if isLowImpactCantripSpell(nil) {
		t.Fatal("nil card must not classify as low-impact")
	}
}

// -----------------------------------------------------------------------------
// Signal B — eager mustCounter on extra-turn and tutor
// -----------------------------------------------------------------------------

func TestChooseResponse_R60_EagerCountersExtraTurn(t *testing.T) {
	h, gs, top := setupResponder(t,
		spellWithOracle("Time Walk", 2, "Take an extra turn after this one."))
	got := h.ChooseResponse(gs, 0, top)
	if got == nil {
		t.Fatal("hat should eagerly counter an extra-turn spell (mustCounter)")
	}
	if got.Card.DisplayName() != "Counterspell" {
		t.Fatalf("expected Counterspell response, got %v", got.Card.DisplayName())
	}
}

func TestChooseResponse_R60_EagerCountersAdditionalTurnPhrasing(t *testing.T) {
	// Modern templating uses "take an additional turn"; the gate covers
	// both phrasings.
	h, gs, top := setupResponder(t,
		spellWithOracle("Temporal Manipulation", 5,
			"Take an additional turn after this one."))
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("hat should counter the 'additional turn' phrasing too")
	}
}

func TestChooseResponse_R60_EagerCountersTutorAtScoreTwo(t *testing.T) {
	// 2-mana tutor: stackItemScore = 2 ≥ 2, so the search-your-library
	// clause fires.
	h, gs, top := setupResponder(t,
		spellWithOracle("Mystical Tutor", 2,
			"Search your library for an instant or sorcery card."))
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("hat should eagerly counter a 2-mana tutor")
	}
}

func TestChooseResponse_R60_OneManaComboTutorMustCounter(t *testing.T) {
	// Updated r60 — combo tutors (open "search your library for a card"
	// shape, plus typed variants for creature/instant/sorcery/artifact/
	// enchantment/aura/planeswalker) now mustCounter regardless of CMC.
	// Pre-r60 the score >= 2 gate let 1-CMC tutors slip past — Vampiric /
	// Mystical / Imperial Seal / Worldly Tutor scored 1, fell below the
	// gate, and resolved. Tutors are the highest-leverage combo enablers
	// in Commander (they convert "we'll draw it eventually" into "we
	// have it next turn") and warrant a hard counter at any cost.
	h, gs, top := setupResponder(t,
		spellWithOracle("Vampiric Tutor", 1,
			"Search your library for a card."))
	if got := h.ChooseResponse(gs, 0, top); got == nil {
		t.Fatal("1-mana combo tutor (Vampiric) should mustCounter under the r60 isComboTutorOracle classifier")
	}
}

func TestChooseResponse_R60_OneManaRampTutorFallsThroughGate(t *testing.T) {
	// Ramp tutors (Crop Rotation: "Search your library for a land card,
	// put it onto the battlefield, then shuffle") DON'T match the combo-
	// tutor classifier — search target is a land, not a card-type-anything.
	// They fall through to the legacy `search && score>=2` gate; at score=1
	// (Crop Rotation is 1cmc with no destroy/win-the-game bonus) the gate
	// fails, and midrange minScore=3 means the hat passes. Verifies the
	// classifier is narrow enough that ramp tutors don't burn counters.
	h, gs, top := setupResponder(t,
		spellWithOracle("Crop Rotation", 1,
			"As an additional cost to cast this spell, sacrifice a land. "+
				"Search your library for a land card, put that card onto the battlefield, then shuffle."))
	if got := h.ChooseResponse(gs, 0, top); got != nil {
		t.Fatalf("1-mana ramp tutor (Crop Rotation) should fall through; got %v",
			got.Card.DisplayName())
	}
}
