package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestNashi_FiresThroughRealCombatDamagePath is the end-to-end regression
// for the combat_damage_player ctx-enrichment fix. Nashi (and 8 sibling
// self-gated combat-damage handlers) read ctx["source_perm"] / ctx["damage_seat"];
// before the fix the engine's combat path never supplied those keys, so the
// handler early-returned on a nil source_perm and was INERT in real games —
// its own per_card unit tests passed only because they hand-built the ctx.
//
// This test drives the genuine engine path (DealCombatDamageStep ->
// fireCombatDamageTriggers -> FireCardTrigger("combat_damage_player")), so it
// fails if the ctx is missing source_perm/damage_seat. Property (c) of the
// ninjutsu audit: combat-damage triggers fire normally when the ninja connects.
func TestNashi_FiresThroughRealCombatDamagePath(t *testing.T) {
	gs := newGame(t, 3)

	// Nashi attacking seat 1, unblocked, with real power so it deals damage.
	nashi := addPerm(gs, 0, "Nashi, Moon Sage's Scion", "creature", "ninja")
	nashi.Card.BasePower, nashi.Card.BaseToughness = 2, 2
	gameengine.MarkEnteredAttacking(nashi)
	gameengine.SetAttackerDefender(nashi, 1)

	// Each player's library has a top card to exile.
	for i := 0; i < 3; i++ {
		gs.Seats[i].Library = []*gameengine.Card{
			{Name: "Top" + string(rune('A'+i)), Owner: i},
			{Name: "Second" + string(rune('A'+i)), Owner: i},
		}
	}

	attackers := []*gameengine.Permanent{nashi}
	blockerMap := map[*gameengine.Permanent][]*gameengine.Permanent{nashi: nil}

	gameengine.DealCombatDamageStep(gs, attackers, blockerMap, false)

	// Nashi exiles the top card of EACH player's library when it connects.
	for i := 0; i < 3; i++ {
		if got := len(gs.Seats[i].Exile); got != 1 {
			t.Errorf("seat %d: expected 1 exiled card from Nashi; got %d (handler inert?)", i, got)
		}
		if got := len(gs.Seats[i].Library); got != 1 {
			t.Errorf("seat %d: expected library milled from 2 to 1; got %d", i, len(gs.Seats[i].Library))
		}
	}
}
