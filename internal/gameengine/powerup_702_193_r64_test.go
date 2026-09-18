package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mkPowerUpPerm: a creature whose own mana value is permMV (via cost:N) with a
// Power-up activated ability costing abilityCMC generic. Non-mana effect (Draw)
// so it goes on the stack and the cost is paid at activation.
func mkPowerUpPerm(gs *GameState, seat, permMV, abilityCMC int, timing string) *Permanent {
	card := &Card{
		Name:  "PowerUp Test",
		Owner: seat,
		Types: []string{"creature", "cost:" + itoa(permMV)},
		AST: &gameast.CardAST{
			Name: "PowerUp Test",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:              gameast.Cost{Mana: &gameast.ManaCost{Symbols: []gameast.ManaSymbol{{Raw: "{" + itoa(abilityCMC) + "}", Generic: abilityCMC}}}},
					Effect:            &gameast.Draw{},
					TimingRestriction: timing,
				},
			},
		},
	}
	p := &Permanent{Card: card, Controller: seat, Owner: seat, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// TestPowerUp_702_193 pins the Power-up keyword: detection, the entered-this-turn
// cost reduction by the permanent's mana value, and "activate only once".
func TestPowerUp_702_193(t *testing.T) {
	gs := newCombatGame(t)

	// Detection.
	pu := mkPowerUpPerm(gs, 0, 2, 4, "power_up")
	if !IsPowerUpAbility(pu, 0) {
		t.Fatal("power_up timing should be detected as a Power-up ability")
	}
	notPU := mkPowerUpPerm(gs, 0, 2, 4, "")
	if IsPowerUpAbility(notPU, 0) {
		t.Fatal("a plain activated ability must not be a Power-up ability")
	}

	// Cost reduction proven via AFFORDABILITY (the activation mana model
	// reconciles ManaPool with the typed pool, so exact-pool assertions are
	// unreliable; affordability reads ManaPool directly). Ability costs 4,
	// permanent's mana value is 2. Fund exactly 2 mana.
	fund := func(seat, n int) {
		EnsureTypedPool(gs.Seats[seat])
		gs.Seats[seat].Mana = &ColoredManaPool{}
		gs.Seats[seat].Mana.Add("C", n)
		gs.Seats[seat].ManaPool = n
	}

	// Entered this turn → cost reduced to 4-2=2, affordable with 2 mana.
	pu.EnteredThisTurn = true
	fund(0, 2)
	if err := ActivateAbility(gs, 0, pu, 0, nil); err != nil {
		t.Fatalf("entered-this-turn power-up (reduced to 2) should be affordable with 2 mana: %v", err)
	}

	// Did NOT enter this turn → full cost 4, NOT affordable with only 2 mana.
	fresh := mkPowerUpPerm(gs, 0, 2, 4, "power_up") // EnteredThisTurn defaults false
	fund(0, 2)
	if err := ActivateAbility(gs, 0, fresh, 0, nil); err == nil {
		t.Error("without entered-this-turn, full cost 4 must NOT be affordable with 2 mana")
	}

	// Activate only once: mark used, fund plenty, second activation refused.
	once := mkPowerUpPerm(gs, 0, 2, 4, "power_up")
	once.EnteredThisTurn = true
	MarkExhausted(once, 0)
	fund(0, 10)
	if err := ActivateAbility(gs, 0, once, 0, nil); err == nil {
		t.Error("a Power-up ability already used must refuse a second activation")
	}
}
