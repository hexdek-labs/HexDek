package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// evaluator_color_screw_r60_test.go — pins the in-hand color-screw
// penalty path added to scoreMana. The pre-r60 ManaAdvantage signal
// measured (a) total source count vs opponents, (b) deck-level
// strategic color coverage from Strategy.ColorDemand, and (c) on
// opponent turns, the untapped-source ratio for response mana. None of
// those three saw the TACTICAL playability of the cards in our hand
// RIGHT NOW: a mono-blue deck whose hand holds Counterspell ({UU}) and
// whose battlefield has 5 lands but 0 untapped Islands scored identical
// to a sibling state with 2 untapped Islands. The fix detects that
// asymmetry and penalizes the screwed side.

// makeHandCard builds a Card with the ManaCostString set so
// gameengine.ParseCostRequirements can read the colored pips. CMC is
// passed via the "cost:N" type token (same pattern as the rest of the
// hat test scaffolding) so ManaCostOf still works for downstream
// callers, though scoreMana's penalty only reads ManaCostString.
func makeHandCard(name, manaCost string, totalCMC int) *gameengine.Card {
	c := newTestCardMinimal(name, []string{"instant"}, totalCMC, nil)
	c.ManaCostString = manaCost
	return c
}

// makeColorScrewEvalGame builds a fresh 2-seat game with no strategic
// ColorDemand so the in-hand-screw signal is isolated from the
// deck-level color-coverage path tested in evaluator_mana_depth_r60_test.go.
func makeColorScrewEvalGame(t *testing.T) (*GameStateEvaluator, *gameengine.GameState) {
	t.Helper()
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
		s.StartingLife = 40
	}
	gs.Active = 0
	ev := NewEvaluator(&StrategyProfile{}) // empty ColorDemand
	return ev, gs
}

// -----------------------------------------------------------------------------
// untappedFieldColorSources
// -----------------------------------------------------------------------------

func TestUntappedFieldColorSources_CountsOnlyUntappedOfColor(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// 2 untapped Islands, 1 tapped Island, 1 untapped Mountain.
	makeColoredLand(seat, "U", false)
	makeColoredLand(seat, "U", false)
	makeColoredLand(seat, "U", true)
	makeColoredLand(seat, "R", false)

	if got := untappedFieldColorSources(seat, "U"); got != 2 {
		t.Errorf("untapped U sources: got %d, want 2 (3 Islands, 1 tapped)", got)
	}
	if got := untappedFieldColorSources(seat, "R"); got != 1 {
		t.Errorf("untapped R sources: got %d, want 1", got)
	}
	if got := untappedFieldColorSources(seat, "G"); got != 0 {
		t.Errorf("untapped G sources: got %d, want 0", got)
	}
	if got := untappedFieldColorSources(nil, "U"); got != 0 {
		t.Errorf("nil seat must return 0, got %d", got)
	}
	if got := untappedFieldColorSources(seat, ""); got != 0 {
		t.Errorf("empty color must return 0, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// handColorScrewPenalty — happy + edge cases
// -----------------------------------------------------------------------------

func TestHandColorScrewPenalty_EmptyHandZero(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	if got := handColorScrewPenalty(seat); got != 0 {
		t.Errorf("empty hand: got %v, want 0", got)
	}
}

func TestHandColorScrewPenalty_ColorlessHandZero(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	seat.Hand = append(seat.Hand,
		makeHandCard("Sol Ring", "{1}", 1),
		makeHandCard("Mind Stone", "{2}", 2),
	)
	if got := handColorScrewPenalty(seat); got != 0 {
		t.Errorf("colorless hand: got %v, want 0", got)
	}
}

func TestHandColorScrewPenalty_FullySatisfiedZero(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Hand: Counterspell ({UU}). Board: 2 untapped Islands.
	seat.Hand = append(seat.Hand, makeHandCard("Counterspell", "{U}{U}", 2))
	makeColoredLand(seat, "U", false)
	makeColoredLand(seat, "U", false)
	if got := handColorScrewPenalty(seat); got != 0 {
		t.Errorf("fully satisfied {UU}: got %v, want 0", got)
	}
}

func TestHandColorScrewPenalty_OnePipShortfallPenalty(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Hand: Counterspell ({UU}). Board: 1 untapped Island, 2 untapped
	// Mountains. We need 2 U but have only 1 → shortfall of 1.
	seat.Hand = append(seat.Hand, makeHandCard("Counterspell", "{U}{U}", 2))
	makeColoredLand(seat, "U", false)
	makeColoredLand(seat, "R", false)
	makeColoredLand(seat, "R", false)

	got := handColorScrewPenalty(seat)
	want := -0.15
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("1-pip shortfall: got %v, want %v", got, want)
	}
}

func TestHandColorScrewPenalty_TappedSourcesDontCount(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Hand: Counterspell ({UU}). Board: 2 TAPPED Islands — they don't
	// satisfy the demand because we're measuring playability NOW.
	seat.Hand = append(seat.Hand, makeHandCard("Counterspell", "{U}{U}", 2))
	makeColoredLand(seat, "U", true)
	makeColoredLand(seat, "U", true)

	got := handColorScrewPenalty(seat)
	want := -0.30 // 2-pip shortfall
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("tapped islands don't count: got %v, want %v", got, want)
	}
}

func TestHandColorScrewPenalty_MaxPerColorAcrossHand(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Hand: Counterspell ({UU}) AND Brainstorm ({U}). Binding constraint
	// is max-per-color = 2 (Counterspell). Brainstorm's 1 U pip is
	// strictly weaker so it doesn't add to demand.
	seat.Hand = append(seat.Hand,
		makeHandCard("Counterspell", "{U}{U}", 2),
		makeHandCard("Brainstorm", "{U}", 1),
	)
	// Board: 1 untapped Island. Shortfall = 2 - 1 = 1.
	makeColoredLand(seat, "U", false)

	got := handColorScrewPenalty(seat)
	want := -0.15
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("max-per-color across hand: got %v, want %v (Counterspell binds, Brainstorm doesn't sum)", got, want)
	}
}

func TestHandColorScrewPenalty_MultiColorShortfallSums(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Hand: Counterspell ({UU}) AND Lightning Bolt ({R}). Two different
	// colors stack into the penalty.
	seat.Hand = append(seat.Hand,
		makeHandCard("Counterspell", "{U}{U}", 2),
		makeHandCard("Lightning Bolt", "{R}", 1),
	)
	// Board: 0 U, 0 R, only Plains. Shortfall = 2 U + 1 R = 3 pips.
	makeColoredLand(seat, "W", false)
	makeColoredLand(seat, "W", false)

	got := handColorScrewPenalty(seat)
	want := -0.45 // 3 * -0.15
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("multi-color shortfall sums: got %v, want %v", got, want)
	}
}

func TestHandColorScrewPenalty_HybridPipsIgnored(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Hand: card with {R/G} pip — flexible payment, ParseCostRequirements
	// puts hybrid pips into the Hybrid slice not Pure. The penalty
	// intentionally ignores Hybrid (counting it as binding would
	// double-penalize a flexible cost). Pure should read [0,0,0,0,0,0].
	seat.Hand = append(seat.Hand, makeHandCard("Burning-Tree Emissary", "{R/G}{R/G}", 2))
	// No relevant sources at all.
	makeColoredLand(seat, "W", false)

	if got := handColorScrewPenalty(seat); got != 0 {
		t.Errorf("hybrid {R/G}{R/G} must not penalize (flexible): got %v, want 0", got)
	}
}

func TestHandColorScrewPenalty_LandCardsInHandIgnored(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Hand: a land card. Lands don't cast — must be skipped from demand.
	land := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)
	land.ManaCostString = "" // lands have no cost
	seat.Hand = append(seat.Hand, land)
	// Even a hand card with a mana cost type but no ManaCostString
	// must be skipped (defensive: corrupt fixture data shouldn't NPE).
	noCost := newTestCardMinimal("Mystery Card", []string{"instant"}, 0, nil)
	seat.Hand = append(seat.Hand, noCost)

	if got := handColorScrewPenalty(seat); got != 0 {
		t.Errorf("lands + no-cost hand cards must be skipped: got %v, want 0", got)
	}
}

func TestHandColorScrewPenalty_CappedAtMinusOne(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Pathological rainbow hand: 5 colors × 5 pips each, 0 sources.
	// Raw shortfall = 25 pips × 0.15 = 3.75. Clamped to -1.0.
	seat.Hand = append(seat.Hand,
		makeHandCard("Storm of Souls", "{W}{W}{W}{W}{W}", 5),
		makeHandCard("Pact of Negation", "{U}{U}{U}{U}{U}", 5),
		makeHandCard("Massacre Wurm", "{B}{B}{B}{B}{B}", 5),
		makeHandCard("Comet Storm", "{R}{R}{R}{R}{R}", 5),
		makeHandCard("Primal Surge", "{G}{G}{G}{G}{G}", 5),
	)
	// 0 lands on board — full screw.
	got := handColorScrewPenalty(seat)
	if math.Abs(got-(-1.0)) > 1e-9 {
		t.Errorf("rainbow hand on empty board: got %v, want -1.0 (cap)", got)
	}
}

// -----------------------------------------------------------------------------
// Integration — scoreMana drops when seat is color-screwed in hand
// -----------------------------------------------------------------------------

func TestScoreMana_HandColorScrew_DropsScore(t *testing.T) {
	ev, gsScrewed := makeColorScrewEvalGame(t)
	seat := gsScrewed.Seats[0]
	// 5 lands total, only Mountains — but hand needs {UU}.
	for i := 0; i < 5; i++ {
		makeColoredLand(seat, "R", false)
	}
	seat.Hand = append(seat.Hand, makeHandCard("Counterspell", "{U}{U}", 2))
	// Opponent has 5 lands too so the count delta is 0; the only
	// remaining signal is the in-hand color-screw penalty.
	for i := 0; i < 5; i++ {
		makeColoredLand(gsScrewed.Seats[1], "R", false)
	}
	screwedScore := ev.scoreMana(gsScrewed, 0)

	ev2, gsOK := makeColorScrewEvalGame(t)
	seatOK := gsOK.Seats[0]
	// Same 5 lands but 2 Islands + 3 Mountains so the hand is castable.
	makeColoredLand(seatOK, "U", false)
	makeColoredLand(seatOK, "U", false)
	for i := 0; i < 3; i++ {
		makeColoredLand(seatOK, "R", false)
	}
	seatOK.Hand = append(seatOK.Hand, makeHandCard("Counterspell", "{U}{U}", 2))
	for i := 0; i < 5; i++ {
		makeColoredLand(gsOK.Seats[1], "R", false)
	}
	okScore := ev2.scoreMana(gsOK, 0)

	if !(screwedScore < okScore) {
		t.Errorf("color-screwed seat must score LOWER than well-mana'd seat: screwed=%v ok=%v", screwedScore, okScore)
	}
	// The delta is the penalty: -0.30 (2-pip U shortfall) on the
	// screwed side, 0 on the OK side. Sanity check the magnitude.
	delta := okScore - screwedScore
	if math.Abs(delta-0.30) > 1e-6 {
		t.Errorf("expected score delta of ~0.30 (2-pip shortfall × 0.15), got %v", delta)
	}
}

func TestScoreMana_NoColorScrew_NoPenalty(t *testing.T) {
	// Sanity that the existing depth-coverage path isn't disturbed when
	// hand is colorless. Two seats with identical 4-land boards, no
	// color demand, no hand pressure — should yield score 0.
	ev, gs := makeColorScrewEvalGame(t)
	for i := 0; i < 4; i++ {
		makeColoredLand(gs.Seats[0], "U", false)
		makeColoredLand(gs.Seats[1], "U", false)
	}
	score := ev.scoreMana(gs, 0)
	// No opponent delta, no color demand, no hand → score == 0.
	if math.Abs(score) > 1e-9 {
		t.Errorf("identical boards + colorless hand: score should be 0, got %v", score)
	}
}
