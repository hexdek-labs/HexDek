package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// endure_r63_test.go — Endure N (CR §701.63, Tarkir: Dragonstorm).
//
// Before this fix the "endures" Modification kind was a no-op `misc_effect`
// log: parsing was clean (Triggered{etb} → Modification{kind:"endures",
// args:[N]}) but the engine placed no counters and made no token. These
// regressions pin the counter-OR-token choice, the canonical doubler routing
// of both branches, token_created firing, and the left-the-battlefield edge
// case.

func doEndure(gs *GameState, src *Permanent, n int) {
	ResolveEffect(gs, src, &gameast.ModificationEffect{ModKind: "endures", Args: []interface{}{float64(n)}})
}

// (1) Counter branch: endure N on a creature still on the battlefield places
// N +1/+1 counters on it and creates no token.
func TestEndure_CounterBranchOnBattlefield(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Dusyut Earthcarver", 5, 5, "creature")
	before := len(gs.Seats[0].Battlefield)

	doEndure(gs, c, 3)

	if c.Counters["+1/+1"] != 3 {
		t.Fatalf("endure 3 should place 3 +1/+1 counters, got %d", c.Counters["+1/+1"])
	}
	if got := len(gs.Seats[0].Battlefield); got != before {
		t.Fatalf("counter branch must not create a token (battlefield %d → %d)", before, got)
	}
}

// (2) Token branch (edge case): endure N on a creature that has LEFT the
// battlefield makes one N/N white Spirit creature token — a single N/N body,
// not N 1/1s — and the counter branch no-ops gracefully.
func TestEndure_TokenBranchWhenSourceGone(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Sinkhole Surveyor", 2, 2, "creature")
	removePermanentFromBattlefield(gs, c) // creature left before endure resolves

	doEndure(gs, c, 2)

	if c.Counters["+1/+1"] != 0 {
		t.Fatalf("counter branch must no-op for a creature off the battlefield, got %d counters", c.Counters["+1/+1"])
	}
	var spirit *Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Spirit" {
			spirit = p
			break
		}
	}
	if spirit == nil {
		t.Fatal("token branch should have created a Spirit token")
	}
	if spirit.Card.BasePower != 2 || spirit.Card.BaseToughness != 2 {
		t.Fatalf("endure 2 token should be a single 2/2, got %d/%d", spirit.Card.BasePower, spirit.Card.BaseToughness)
	}
	if len(spirit.Card.Colors) != 1 || spirit.Card.Colors[0] != "W" {
		t.Fatalf("Spirit token should be white, got colors %v", spirit.Card.Colors)
	}
	if !spirit.IsCreature() {
		t.Fatal("Spirit token should be a creature")
	}
}

// (3) Counter doubler: Doubling Season doubles the endure counters (proves the
// counter branch goes through the §616 would_put_counter chain, not raw AddCounter).
func TestEndure_CounterDoublerApplies(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	c := addBattlefield(gs, 0, "Inspirited Vanguard", 2, 2, "creature")

	doEndure(gs, c, 2)

	if c.Counters["+1/+1"] != 4 {
		t.Fatalf("endure 2 with Doubling Season should place 4 counters, got %d", c.Counters["+1/+1"])
	}
}

// (4) Token doubler: Doubling Season doubles the endure token (proves the
// token branch goes through the would_create_token chain, not a raw mint).
func TestEndure_TokenDoublerApplies(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	c := addBattlefield(gs, 0, "Sinkhole Surveyor", 2, 2, "creature")
	removePermanentFromBattlefield(gs, c) // force the token branch

	doEndure(gs, c, 1)

	spirits := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Spirit" {
			spirits++
		}
	}
	if spirits != 2 {
		t.Fatalf("endure token with Doubling Season should make 2 Spirits, got %d", spirits)
	}
}

// (5) The token branch fires token_created so token-matters payoffs see it.
func TestEndure_TokenBranchFiresTokenCreated(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "token_created" {
			fired++
		}
	}
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Sinkhole Surveyor", 2, 2, "creature")
	removePermanentFromBattlefield(gs, c)

	doEndure(gs, c, 1)

	if fired == 0 {
		t.Fatal("token branch should fire token_created for token-matters payoffs")
	}
}

// (6) Granting trigger fires and routes to endure: an ETB-triggered "it endures
// N" (Dusyut Earthcarver shape) resolves through the trigger → stack → endure
// path and grows the creature.
func TestEndure_ETBTriggerFiresAndRoutes(t *testing.T) {
	gs := newCombatGame(t)
	ast := &gameast.CardAST{
		Name: "Dusyut Earthcarver",
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "etb"},
				Effect:  &gameast.ModificationEffect{ModKind: "endures", Args: []interface{}{float64(3)}},
				Raw:     "when this creature enters, it endures 3",
			},
		},
	}
	c := addBattlefieldWithAST(gs, 0, "Dusyut Earthcarver", 5, 5, ast, "creature")

	FirePermanentETBTriggers(gs, c)
	for i := 0; i < 80 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}

	if c.Counters["+1/+1"] != 3 {
		t.Fatalf("ETB endure 3 should grow the creature to 3 counters, got %d", c.Counters["+1/+1"])
	}
}
