package gameengine

import (
	"math/rand"
	"testing"
)

// myriad_doubler_r63_test.go — r63 mechanic-probe (CR §702.116): myriad token
// creation must route through the canonical CreateDoubledTokens chokepoint so
// (1) token-count doublers (Doubling Season) double the copies per opponent and
// (2) token_created fires for token-matters payoffs. The prior direct-append
// path bypassed both (canonical-helper-bypass class).

// (BYPASS fix) Doubling Season doubles the per-opponent myriad copies, and
// token_created fires once reporting the full batch.
func TestMyriad_RespectsTokenDoublerAndFiresTokenCreated(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(11)), nil)

	// Doubling Season on the attacker's side.
	ds := addKWCombatBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
	RegisterDoublingSeason(gs, ds)

	// Seat 0 attacks seat 1; the other opponents are seats 2 and 3.
	atk := addMyriadAttacker(gs, 0, "Doubled Myriad", 3, 3, 1, "myriad")

	var tokenCreated int
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	TriggerHook = func(hgs *GameState, event string, ctx map[string]interface{}) {
		if event != "token_created" {
			return
		}
		if c, ok := ctx["count"].(int); ok {
			tokenCreated += c
		}
	}

	ApplyMyriad(gs, atk, 0)

	// 2 other opponents × 2 (Doubling Season) = 4 myriad tokens.
	tokens := myriadTokensOf(gs, 0)
	if len(tokens) != 4 {
		t.Errorf("Doubling Season should double myriad to 4 tokens (2 opponents × 2), got %d", len(tokens))
	}
	// token_created fired, reporting the full batch.
	if tokenCreated != 4 {
		t.Errorf("token_created should fire reporting 4 myriad tokens, got %d", tokenCreated)
	}
	// Doubling must preserve the per-token state: every copy stays tapped,
	// attacking, and a faithful token copy.
	perOpp := map[int]int{}
	for _, tok := range tokens {
		if !tok.Tapped {
			t.Error("doubled myriad token must be tapped")
		}
		if tok.Flags["attacking"] != 1 {
			t.Error("doubled myriad token must be attacking")
		}
		if !tok.IsToken() || !tok.HasKeyword("myriad") {
			t.Error("doubled myriad token must be a faithful token copy")
		}
		def, _ := AttackerDefender(tok)
		perOpp[def]++
	}
	// Two copies attacking each of seats 2 and 3.
	if perOpp[2] != 2 || perOpp[3] != 2 {
		t.Errorf("each other opponent should get 2 attacking copies, got seat2=%d seat3=%d", perOpp[2], perOpp[3])
	}
}

// Guard against masking a real miscount: with NO doubler, the per-opponent
// count must remain exactly 1 (the fix is additive, not a blanket ×2).
func TestMyriad_NoDoublerStaysOnePerOpponent(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(12)), nil)
	atk := addMyriadAttacker(gs, 0, "Plain Myriad", 2, 2, 1, "myriad")

	ApplyMyriad(gs, atk, 0)

	if got := len(myriadTokensOf(gs, 0)); got != 2 {
		t.Errorf("with no token doubler, 3 opponents should make exactly 2 myriad tokens, got %d", got)
	}
}
