package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 — Aang and Katara's attack trigger fires "Whenever Aang and Katara ...
// attack", i.e. ONLY when Aang itself is the attacker. The "creature_attacks"
// event is dispatched once per declared attacker, so the missing self-gate
// made the handler mint X tokens for EVERY creature the controller attacked
// with; freshly-minted tokens enter as new attackers / tapped permanents that
// bump X, so the cascade never terminated — game 0 / seed 7 / turn 33 of the
// full-corpus Feynman 50-game zone-accounting test hung here forever.

func countAllyTokens(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Ally Token" {
			n++
		}
	}
	return n
}

// When a DIFFERENT creature attacks, Aang and Katara must NOT mint tokens
// (this is the loop-prevention gate).
func TestAangAndKatara_AttackNoFireOnNonSelfAttacker(t *testing.T) {
	gs := newGame(t, 2)
	aang := addPerm(gs, 0, "Aang and Katara", "creature")
	other := addPerm(gs, 0, "Other Attacker", "creature")
	// A tapped artifact so X would be ≥1 if the trigger wrongly fired.
	art := addPerm(gs, 0, "Mox", "artifact")
	art.Tapped = true

	aangAndKataraAttackTokens(gs, aang, map[string]interface{}{
		"attacker_perm": other,
		"seat":          0,
	})
	if got := countAllyTokens(gs, 0); got != 0 {
		t.Fatalf("Aang must not mint tokens when a DIFFERENT creature attacks; got %d tokens", got)
	}
}

// When Aang and Katara itself attacks, it mints X tokens (X = tapped
// artifacts/creatures you control) — exactly once.
func TestAangAndKatara_AttackFiresForSelf(t *testing.T) {
	gs := newGame(t, 2)
	aang := addPerm(gs, 0, "Aang and Katara", "creature")
	a1 := addPerm(gs, 0, "Mox A", "artifact")
	a1.Tapped = true
	a2 := addPerm(gs, 0, "Mox B", "artifact")
	a2.Tapped = true

	aangAndKataraAttackTokens(gs, aang, map[string]interface{}{
		"attacker_perm": aang,
		"seat":          0,
	})
	// X = 2 tapped artifacts → 2 Ally tokens.
	if got := countAllyTokens(gs, 0); got != 2 {
		t.Fatalf("Aang attacking with 2 tapped artifacts should mint 2 tokens; got %d", got)
	}
}
