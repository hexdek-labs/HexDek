package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Loki r60 residual cluster (round 1 / 2 / 3, bit-stable seed-42 game
// 1611): "Behemoth of Vault 0" (seat 0) attacking with summoning
// sickness and no haste. The dominant repro is Raph & Mikey, Troublemakers'
// attack trigger pulling Behemoth of Vault 0 out of the library and
// dropping it "tapped and attacking" per CR §506.3. The handler
// stamped Flags["attacking"]=1 but left SummoningSick=true; the
// scoop-in at combat.go:533 only runs during DeclareAttackers (not
// during a trigger-fired ETB), so the bad SS+attacking combo persisted
// until checkCombatLegality flagged it at end_of_combat when the game
// ended mid-combat (lethal damage triggered before EndOfCombatStep
// cleared flagAttacking).
//
// Fix (r60): every per_card handler that stamps Flags["attacking"]=1
// also clears SummoningSick (Kaalia, Sauron, Satya, Raph & Mikey;
// Strefan + Winota already did this; ninja_sneak's second site
// hardened defensively). Plus combat.go scoop-in clears SS for any
// permanent that arrives flagAttacking via a path that pre-stamps the
// flag (e.g. CreateToken e.Tapped+e.Attacking).

func TestRaphMikey_PicksCreatureAndClearsSummoningSickness(t *testing.T) {
	gs := newGame(t, 2)
	gs.Phase = "combat"
	gs.Step = "declare_attackers"
	gs.Active = 0

	// Opponent must exist to be the attack target.
	addPerm(gs, 1, "Bear", "creature")

	// Seat 0: Raph & Mikey on the battlefield, attacking.
	raph := addPerm(gs, 0, "Raph & Mikey, Troublemakers", "creature")
	raph.Flags["attacking"] = 1
	gameengine.SetAttackerDefender(raph, 1)

	// Library: a non-creature filler on top, then Behemoth of Vault 0 (creature).
	behemoth := &gameengine.Card{
		Name:          "Behemoth of Vault 0",
		Owner:         0,
		Types:         []string{"creature", "artifact"},
		BasePower:     6,
		BaseToughness: 6,
	}
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Filler", Owner: 0, Types: []string{"sorcery"}},
		behemoth,
		{Name: "Bottom 1", Owner: 0, Types: []string{"land"}},
	}

	// Fire Raph & Mikey's attack trigger.
	raphMikeyTroublemakersAttack(gs, raph, map[string]interface{}{
		"attacker_perm": raph,
	})

	// Behemoth should be on seat 0 battlefield, attacking, NOT summoning-sick.
	var found *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Behemoth of Vault 0" {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatalf("Behemoth of Vault 0 not placed on seat 0 battlefield")
	}
	if found.Flags["attacking"] != 1 {
		t.Errorf("Behemoth should be flagged attacking, got Flags=%v", found.Flags)
	}
	if found.SummoningSick {
		t.Errorf("Behemoth SummoningSick must be cleared (CR §506.3); leaving it true trips CombatLegality on mid-combat game end")
	}
}

// TestRaphMikey_CombatLegalityCleanAfterMidCombatGameEnd is the
// cross-validation test that mirrors the loki repro: Raph & Mikey
// pulls Behemoth, game ends mid-combat (no EndOfCombatStep), invariant
// must stay clean.
func TestRaphMikey_CombatLegalityCleanAfterMidCombatGameEnd(t *testing.T) {
	gs := newGame(t, 2)
	gs.Phase = "combat"
	gs.Step = "declare_attackers"
	gs.Active = 0

	addPerm(gs, 1, "Bear", "creature")

	raph := addPerm(gs, 0, "Raph & Mikey, Troublemakers", "creature")
	raph.Flags["attacking"] = 1
	gameengine.SetAttackerDefender(raph, 1)

	behemoth := &gameengine.Card{
		Name: "Behemoth of Vault 0", Owner: 0,
		Types: []string{"creature", "artifact"},
		BasePower: 6, BaseToughness: 6,
	}
	gs.Seats[0].Library = []*gameengine.Card{behemoth}

	raphMikeyTroublemakersAttack(gs, raph, map[string]interface{}{
		"attacker_perm": raph,
	})

	// Simulate mid-combat game end — phase stays combat, flagAttacking
	// stays set (EndOfCombatStep didn't run).
	gs.Step = "end_of_combat"

	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		if v.Name == "CombatLegality" {
			t.Fatalf("CombatLegality must stay clean — the SS-clear fix didn't take: %s", v.Message)
		}
	}
}
