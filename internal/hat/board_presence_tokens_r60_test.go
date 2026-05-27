package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// board_presence_tokens_r60_test.go — regressions for the per-creature
// token discount wired into effectiveBoardPower. Pre-r60 a board of 10
// 1/1 tokens contributed the same raw power as a single 10/10 nontoken
// (10 in both cases) despite three structural disadvantages: wipe
// blowout amplifies per-body, CR §110.5g deletes tokens on any zone
// change, and CR §111.1 excludes them from card-targeting recursion.
// See poker.go::tokenDiscount.

// -----------------------------------------------------------------------------
// tokenDiscount — unit
// -----------------------------------------------------------------------------

func TestTokenDiscount_NontokenFullCredit(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)

	if m := tokenDiscount(p); m != 1.0 {
		t.Errorf("nontoken creature should be 1.00; got %.2f", m)
	}
}

func TestTokenDiscount_TokenDeratedToSevenTenths(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Spirit Token", []string{"creature", "token"}, 0, nil), 1, 1)

	if m := tokenDiscount(p); m != 0.7 {
		t.Errorf("token creature should be 0.70; got %.2f", m)
	}
}

func TestTokenDiscount_NilSafe(t *testing.T) {
	if m := tokenDiscount(nil); m != 1.0 {
		t.Errorf("nil permanent should be 1.00 (no-op); got %.2f", m)
	}
}

// -----------------------------------------------------------------------------
// effectiveBoardPower — integration
// -----------------------------------------------------------------------------

// 10 1/1 vanilla tokens should score below 1 10/10 vanilla nontoken on
// the power axis. 10 * 1.0 * 0.7 = 7.0 → 7; vs 10 * 1.0 * 1.0 = 10.0 → 10.
// (scoreBoard's width bonus is a separate, smaller offset; this pin is
// just the power-side accounting.)
func TestEffectiveBoardPower_TenTokensBelowOneNontokenSamePower(t *testing.T) {
	tokenSide := func(t *testing.T) (*gameengine.GameState, *gameengine.Seat) {
		gs := newTestGame(t, 2)
		for i := 0; i < 10; i++ {
			_ = newTestPermanent(gs.Seats[0],
				newTestCardMinimal("Goblin Token", []string{"creature", "token"}, 0, nil), 1, 1)
		}
		return gs, gs.Seats[0]
	}
	nontokenSide := func(t *testing.T) (*gameengine.GameState, *gameengine.Seat) {
		gs := newTestGame(t, 2)
		_ = newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Fatty", []string{"creature"}, 6, nil), 10, 10)
		return gs, gs.Seats[0]
	}

	tgs, tseat := tokenSide(t)
	ngs, nseat := nontokenSide(t)

	tokenPower := effectiveBoardPower(tgs, tseat)
	nontokenPower := effectiveBoardPower(ngs, nseat)

	if !(tokenPower < nontokenPower) {
		t.Errorf("10 token bodies (%d) should report less effective power than 1 nontoken of equal raw power (%d) under r60 token discount",
			tokenPower, nontokenPower)
	}
	if tokenPower != 7 {
		t.Errorf("10x 1/1 tokens should be 10*0.7=7.0→7; got %d", tokenPower)
	}
	if nontokenPower != 10 {
		t.Errorf("1x 10/10 nontoken should remain 10 (no discount); got %d", nontokenPower)
	}
}

// Token discount stacks MULTIPLICATIVELY with evasion — a 1/1 token with
// flying is more valuable than a vanilla 1/1 token (flying connects) but
// less valuable than a nontoken 1/1 with flying (token still wipes out
// with no recovery). 1pw * 1.20 evasion * 0.7 token = 0.84.
//
// Use 5/5 bodies so the rounding actually surfaces the math:
//
//	flying nontoken 5/5: 5 * 1.20 = 6.0
//	flying token 5/5:    5 * 1.20 * 0.7 = 4.2 → 4
//	vanilla nontoken 5/5: 5 * 1.0 = 5
//	vanilla token 5/5:   5 * 1.0 * 0.7 = 3.5 → 4 (round-half-up)
func TestEffectiveBoardPower_TokenDiscountStacksWithEvasion(t *testing.T) {
	mk := func(t *testing.T, isToken, hasFlying bool) (*gameengine.GameState, *gameengine.Seat) {
		gs := newTestGame(t, 2)
		types := []string{"creature"}
		if isToken {
			types = append(types, "token")
		}
		p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Body", types, 4, nil), 5, 5)
		if hasFlying {
			addKeyword(p, "flying")
		}
		return gs, gs.Seats[0]
	}

	flyingNontoken := func() int { gs, s := mk(t, false, true); return effectiveBoardPower(gs, s) }()
	flyingToken := func() int { gs, s := mk(t, true, true); return effectiveBoardPower(gs, s) }()
	vanillaNontoken := func() int { gs, s := mk(t, false, false); return effectiveBoardPower(gs, s) }()

	if flyingNontoken != 6 {
		t.Errorf("flying nontoken 5/5 should be 5*1.20=6.0→6; got %d", flyingNontoken)
	}
	if flyingToken != 4 {
		t.Errorf("flying token 5/5 should be 5*1.20*0.7=4.2→4; got %d", flyingToken)
	}
	if !(flyingToken < flyingNontoken) {
		t.Errorf("flying token (%d) should score below flying nontoken (%d) — token discount applies on top of evasion bonus",
			flyingToken, flyingNontoken)
	}
	if !(flyingToken < vanillaNontoken) {
		t.Errorf("flying token (%d) should still score below vanilla nontoken (%d) — token discount outweighs evasion bump on 5pw bodies",
			flyingToken, vanillaNontoken)
	}
}

// Tap discount applies to tokens too — a tapped 1/1 token is 1*0.7*0.7
// = 0.49 → rounds to 0. Make sure the helper composes correctly with
// the existing tap/sick discounts (multiplicative, not exclusive).
// Use a 4/4 token to avoid the round-to-zero edge: 4*0.7*0.7 = 1.96 → 2.
func TestEffectiveBoardPower_TappedTokenDiscountStacks(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Big Token", []string{"creature", "token"}, 4, nil), 4, 4)
	p.Tapped = true

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 2 {
		t.Errorf("tapped 4/4 token should be 4*0.7*0.7=1.96→2; got %d", got)
	}
}

// Planeswalkers are scored via loyalty (loyalty + 2), not the creature
// power loop — they shouldn't be touched by tokenDiscount even if (in
// the unusual Daxos-Returned-style case) the planeswalker is also a
// token. Construct a planeswalker permanent and confirm the loyalty
// math is untouched.
func TestEffectiveBoardPower_TokenDiscountSkipsPlaneswalker(t *testing.T) {
	gs := newTestGame(t, 2)
	pw := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Daxos Token PW", []string{"planeswalker", "legendary", "token"}, 4, nil), 0, 0)
	pw.Counters = map[string]int{"loyalty": 3}

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 5 {
		t.Errorf("planeswalker (even token-typed) should contribute loyalty+2 = 3+2 = 5, undiscounted; got %d", got)
	}
}

// -----------------------------------------------------------------------------
// scoreBoard integration
// -----------------------------------------------------------------------------

// The headline ask: 10 1/1 tokens should score below 10 1/1 nontoken
// weenies vs an equal opponent. Pre-r60 they scored identically; under
// the token discount the token side reads as ~7 effective vs nontoken
// side's 10, so the power differential drops by ~3 against the same
// opponent.
func TestScoreBoard_TokenBoardScoresBelowNontokenWeenieBoard(t *testing.T) {
	mk := func(t *testing.T, ourTokens bool) (*GameStateEvaluator, *gameengine.GameState) {
		gs := newTestGame(t, 2)
		types := []string{"creature"}
		if ourTokens {
			types = append(types, "token")
		}
		for i := 0; i < 10; i++ {
			_ = newTestPermanent(gs.Seats[0], newTestCardMinimal("Body", types, 1, nil), 1, 1)
		}
		// Constant opponent: 3 vanilla 2/2s (raw 6).
		for i := 0; i < 3; i++ {
			_ = newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
		}
		return &GameStateEvaluator{}, gs
	}

	tok, tgs := mk(t, true)
	non, ngs := mk(t, false)

	tokScore := tok.scoreBoard(tgs, 0)
	nonScore := non.scoreBoard(ngs, 0)

	if !(tokScore < nonScore) {
		t.Errorf("our-token board (%.3f) should score below our-nontoken board (%.3f) — opp constant, only token discount differs",
			tokScore, nonScore)
	}
}
