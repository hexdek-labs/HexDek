package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// enlist_aradesh_payoff_r63_test.go — end-to-end regression that the new
// enlist primitive (gameengine.ApplyEnlist) makes Aradesh's "if it enlisted a
// creature this combat" payoff functional. Before r63 nothing set
// enlisted_this_combat, so Aradesh's double-strike + draw never fired.
func TestAradesh_EnlistPayoff_FiresAfterApplyEnlist(t *testing.T) {
	gs := newGame(t, 2)
	aradesh := addPerm(gs, 0, "Aradesh, the Founder", "creature", "human", "soldier")
	aradesh.Card.BasePower, aradesh.Card.BaseToughness = 3, 3
	aradesh.Flags["attacking"] = 1
	// A power-1 helper so the post-enlist attacker power is 3+1=4 → draws.
	helper := addPerm(gs, 0, "Helper", "creature")
	helper.Card.BasePower, helper.Card.BaseToughness = 1, 1
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{Name: "Top", Owner: 0})
	handBefore := len(gs.Seats[0].Hand)

	// Enlist the helper as Aradesh attacks.
	if !gameengine.ApplyEnlist(gs, 0, aradesh, helper) {
		t.Fatal("ApplyEnlist should succeed")
	}
	// Aradesh's attack trigger now sees enlisted_this_combat.
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": aradesh,
		"attacker_seat": 0,
		"attacker_card": aradesh.Card,
	})

	if aradesh.Flags["kw:double_strike"] != 1 {
		t.Error("Aradesh should gain double strike after enlisting a creature this combat")
	}
	if len(gs.Seats[0].Hand) != handBefore+1 {
		t.Errorf("Aradesh should draw (power 3+1=4 >= 4); hand %d → %d", handBefore, len(gs.Seats[0].Hand))
	}
}
