package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// political_goad_deal_r60_test.go — regressions for the goad-deal
// heuristic added to cardHeuristic. Pre-r60 a goad spell in hand was
// scored as generic CatUtility — sometimes penalized as non-strategic
// by stax/control/combo archetypes. The "deal" heuristic elevates
// goad when an opponent's threat could be redirected to lethal a
// different opponent while also damaging us. See yggdrasil.go::
// cardIsGoad + goadDealOpportunity.
//
// Helpers (newTestGame, newTestPermanent, newTestCardMinimal,
// cardWithRawOracle, addKeyword) come from hat_test.go and
// mulligan_r60_test.go.

// goadSpell returns a 2cmc instant whose oracle text contains "goad".
func goadSpell(name string) *gameengine.Card {
	return cardWithRawOracle(name, []string{"instant"}, 2, "Goad target creature.")
}

// nonGoadSpell returns a 2cmc instant whose text does NOT contain goad.
func nonGoadSpell(name string) *gameengine.Card {
	return cardWithRawOracle(name, []string{"instant"}, 2, "Draw a card.")
}

// -----------------------------------------------------------------------------
// cardIsGoad — unit
// -----------------------------------------------------------------------------

func TestCardIsGoad_DetectsGoadInOracle(t *testing.T) {
	if !cardIsGoad(goadSpell("Disrupt Decorum")) {
		t.Error("oracle text 'Goad target creature.' should match")
	}
}

func TestCardIsGoad_RejectsNonGoad(t *testing.T) {
	if cardIsGoad(nonGoadSpell("Brainstorm")) {
		t.Error("non-goad oracle should not match")
	}
}

func TestCardIsGoad_NilSafe(t *testing.T) {
	if cardIsGoad(nil) {
		t.Error("nil card should return false")
	}
}

// -----------------------------------------------------------------------------
// goadDealOpportunity — unit
// -----------------------------------------------------------------------------

// No opponent at low life → no deal value, returns 0.
func TestGoadDealOpportunity_NoLowLifeOppReturnsZero(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[1].Life = 40
	gs.Seats[2].Life = 40
	gs.Seats[3].Life = 40
	// Put a giant creature on seat 2 (would be a great redirect candidate
	// IF there were a low-life seat to redirect TO, but everyone's at 40).
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Eldrazi", []string{"creature"}, 10, nil), 10, 10)

	h := NewYggdrasilHat(nil, 0)
	if got := h.goadDealOpportunity(gs, 0); got != 0 {
		t.Errorf("no low-life opp → 0 bonus; got %.2f", got)
	}
}

// Low-life opp exists but no creature is large enough to lethal them
// → no deal value.
func TestGoadDealOpportunity_NoLargeThreatReturnsZero(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 5  // low-life opp
	gs.Seats[2].Life = 40 // potential redirector
	gs.Seats[3].Life = 40
	// Seat 2's creature is too small to lethal seat 1 even on redirect.
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)

	h := NewYggdrasilHat(nil, 0)
	if got := h.goadDealOpportunity(gs, 0); got != 0 {
		t.Errorf("threat (3 pow) < lowest-life-opp (5 life) → 0 bonus; got %.2f", got)
	}
}

// Headline ask: low-life opp + large threat owned by a different
// opponent → bonus fires. Threat 10-power vs lowest-life-opp 8 life
// → kill-on-redirect. In a 4-seat game where we have an empty board
// and opp 2 has a 10/10, relativePosition reads us as BEHIND, so the
// board-behind bump (+0.10) fires too. Baseline 0.20 + behind 0.10 =
// 0.30. Power*2 = 20 < myLife = 30 → fast-clock bump does NOT fire.
func TestGoadDealOpportunity_BaselineLethalRedirectGetsBonus(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 30 // me — threshold = max(30/3, 6) = 10
	gs.Seats[1].Life = 8  // low-life opp (will die to redirect)
	gs.Seats[2].Life = 40 // threat owner
	gs.Seats[3].Life = 40
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Threat", []string{"creature"}, 6, nil), 10, 10)

	h := NewYggdrasilHat(nil, 0)
	got := h.goadDealOpportunity(gs, 0)
	if got < 0.29 || got > 0.31 {
		t.Errorf("lethal redirect with board-behind should be +0.30 (0.20 base + 0.10 behind); got %.3f", got)
	}
}

// Fast-clock bump: power*2 >= myLife adds +0.15 on top of baseline.
// With board-behind also firing, the cap kicks in: 0.20 + 0.15 + 0.10
// = 0.45 (exactly the cap).
func TestGoadDealOpportunity_FastClockBumpFires(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 20 // me — threshold = max(20/3, 6) = 6
	gs.Seats[1].Life = 10 // low-life opp
	gs.Seats[2].Life = 40
	gs.Seats[3].Life = 40
	// 12/12 on seat 2 — lethal to seat 1 (12 >= 10), threat to us
	// (12 >= 6 threshold), and 12*2 = 24 >= 20 myLife → fast-clock bump
	// fires. We're board-behind too. Total capped at +0.45.
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Beast", []string{"creature"}, 6, nil), 12, 12)

	h := NewYggdrasilHat(nil, 0)
	got := h.goadDealOpportunity(gs, 0)
	if got < 0.44 || got > 0.46 {
		t.Errorf("fast-clock + behind should cap at +0.45; got %.3f", got)
	}
}

// Board-behind bump gating: when we're AHEAD of the table, the behind
// bonus does NOT fire. Construct a game where we have a larger board
// than opponents while the deal opportunity still exists. Bonus should
// drop to the baseline 0.20 (no behind bump).
func TestGoadDealOpportunity_BehindBumpSkippedWhenAhead(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 8  // low-life opp
	gs.Seats[2].Life = 40 // threat owner
	gs.Seats[3].Life = 40

	// Stack our own board so relativePosition reads us as ahead.
	for i := 0; i < 5; i++ {
		newTestPermanent(gs.Seats[0], newTestCardMinimal("Our Bear", []string{"creature"}, 2, nil), 3, 3)
	}
	// Single 10/10 on seat 2 — deal target exists, but we're ahead.
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Threat", []string{"creature"}, 6, nil), 10, 10)

	h := NewYggdrasilHat(nil, 0)
	got := h.goadDealOpportunity(gs, 0)
	if got < 0.19 || got > 0.21 {
		t.Errorf("ahead on board → no behind bump; expected baseline +0.20; got %.3f", got)
	}
}

// Defender exclusion: a defender, even if huge and owned by a different
// seat, can't satisfy the "must attack" rider so goad on it is a no-op.
// The function must skip defenders.
func TestGoadDealOpportunity_DefenderIsSkipped(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 8 // low-life opp
	gs.Seats[2].Life = 40
	gs.Seats[3].Life = 40
	wall := newTestPermanent(gs.Seats[2], newTestCardMinimal("Wall of Light", []string{"creature"}, 4, nil), 10, 10)
	addKeyword(wall, "defender")

	h := NewYggdrasilHat(nil, 0)
	if got := h.goadDealOpportunity(gs, 0); got != 0 {
		t.Errorf("defender on potential redirect seat should be skipped → 0 bonus; got %.2f", got)
	}
}

// Lowest-life opp's OWN creatures are not redirect candidates — goading
// their creature would just send it at a different opponent (typically
// us or seat 3), not at themselves. The exclusion must hold.
func TestGoadDealOpportunity_SkipsLowLifeOppsOwnCreature(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 8 // low-life opp ALSO owns the big creature
	gs.Seats[2].Life = 40
	gs.Seats[3].Life = 40
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Threat", []string{"creature"}, 6, nil), 10, 10)

	h := NewYggdrasilHat(nil, 0)
	if got := h.goadDealOpportunity(gs, 0); got != 0 {
		t.Errorf("lowest-life-opp's own creature isn't a redirect candidate → 0; got %.2f", got)
	}
}

// Dead seats are excluded from the low-life search and from the
// candidate-owner search. A lost seat at 0 life shouldn't be the
// "redirect target".
func TestGoadDealOpportunity_LostSeatsExcluded(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 0 // already lost
	gs.Seats[1].Lost = true
	gs.Seats[2].Life = 30 // potential redirector
	gs.Seats[3].Life = 30
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Threat", []string{"creature"}, 6, nil), 10, 10)

	h := NewYggdrasilHat(nil, 0)
	// Lowest LIVING opp is seat 3 at 30 life. Threat is 10-power — not
	// lethal to a 30-life opp → no bonus.
	if got := h.goadDealOpportunity(gs, 0); got != 0 {
		t.Errorf("lost seats excluded from low-life search; remaining opps too healthy → 0; got %.2f", got)
	}
}

// -----------------------------------------------------------------------------
// cardHeuristic integration
// -----------------------------------------------------------------------------

// Goad spell with a viable redirect target should out-score the same
// goad spell when no redirect is available.
func TestCardHeuristic_GoadWithDealBeatsGoadWithoutDeal(t *testing.T) {
	// Game A: low-life opp + large threat on another opp = deal available.
	gsA := newTestGame(t, 4)
	gsA.Seats[0].Life = 30
	gsA.Seats[1].Life = 8
	gsA.Seats[2].Life = 40
	gsA.Seats[3].Life = 40
	newTestPermanent(gsA.Seats[2], newTestCardMinimal("Threat", []string{"creature"}, 6, nil), 10, 10)

	// Game B: same opp life totals, no large threat on the board.
	gsB := newTestGame(t, 4)
	gsB.Seats[0].Life = 30
	gsB.Seats[1].Life = 8
	gsB.Seats[2].Life = 40
	gsB.Seats[3].Life = 40
	// Tiny creature — too small to redirect for lethal.
	newTestPermanent(gsB.Seats[2], newTestCardMinimal("Squirrel", []string{"creature"}, 1, nil), 1, 1)

	h := NewYggdrasilHat(nil, 0)
	goad := goadSpell("Disrupt Decorum")

	scoreA := h.cardHeuristic(gsA, 0, goad)
	scoreB := h.cardHeuristic(gsB, 0, goad)

	if !(scoreA > scoreB) {
		t.Errorf("goad with deal available (%.3f) should out-score goad without deal (%.3f)", scoreA, scoreB)
	}
	// Delta is baseline 0.20 + board-behind 0.10 = 0.30. (gsA puts us
	// strictly behind seat 2's 10/10 board, so the relPos < 0 branch
	// fires alongside the baseline lethal-redirect bonus.)
	delta := scoreA - scoreB
	if delta < 0.28 || delta > 0.32 {
		t.Errorf("delta should be ≈0.30 (0.20 base + 0.10 behind); got %.3f", delta)
	}
}

// A non-goad card with the same CMC and shape should NOT be lifted by
// goadDealOpportunity even when the deal-condition board state exists.
func TestCardHeuristic_NonGoadCardNotAffectedByDealConditions(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 8
	gs.Seats[2].Life = 40
	gs.Seats[3].Life = 40
	newTestPermanent(gs.Seats[2], newTestCardMinimal("Threat", []string{"creature"}, 6, nil), 10, 10)

	h := NewYggdrasilHat(nil, 0)

	goad := goadSpell("Disrupt Decorum")
	nonGoad := nonGoadSpell("Divination Variant")

	scoreGoad := h.cardHeuristic(gs, 0, goad)
	scoreNonGoad := h.cardHeuristic(gs, 0, nonGoad)

	if !(scoreGoad > scoreNonGoad) {
		t.Errorf("under deal conditions, goad (%.3f) should out-score non-goad of equal cmc (%.3f)",
			scoreGoad, scoreNonGoad)
	}
}
