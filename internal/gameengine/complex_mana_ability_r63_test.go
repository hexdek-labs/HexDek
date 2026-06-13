package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// complex_mana_ability_r63_test.go — CR §605.1a/§605.3a audit of COMPLEX
// mana-ability shapes (7174n1c follow-up). Two root-cause fixes, both
// verified end-to-end through ActivateAbility:
//
//   BUG A (classification): "{T}: Add {W} or {U}" parses to a Choice of
//     two AddMana. effectProducesMana didn't recognize a Choice, so ~405
//     dual/painland/talisman mana abilities were classified as NON-mana
//     and pushed onto the STACK — illegally responseable and untappable
//     mid-cost. They must resolve INLINE per §605.3a.
//
//   BUG B (dropped rider): the pain-land downside ("This land deals N
//     damage to you") parses to a standalone Static{self_damage} sibling,
//     disconnected from the mana ability it modifies. A Static is never
//     resolved on its own, so 25 pain lands tapped for mana with no life
//     cost. The rider must be applied INLINE with the painful mana tap.

// painlandAST mirrors the real parsed shape of a pain land: ability 0 =
// {T}: Add {C} (free); ability 1 = {T}: Add c1 or c2 (Choice, painful);
// ability 2 = standalone Static self_damage rider (amt damage).
func painlandAST(name, c1, c2 string, amt int) *gameast.CardAST {
	return &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost:   gameast.Cost{Tap: true},
				Effect: &gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{"C"}}}},
			},
			&gameast.Activated{
				Cost: gameast.Cost{Tap: true},
				Effect: &gameast.Choice{
					Options: []gameast.Effect{
						&gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{c1}}}},
						&gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{c2}}}},
					},
					Pick: *gameast.NumInt(1),
				},
			},
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "self_damage", Args: []interface{}{amt}}},
		},
	}
}

// BUG A — the colored Choice tap is a mana ability and resolves inline.
func TestComplexMana_PainlandColored_InlineMana(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 40
	gs.Seats[0].ManaPool = 0
	ast := painlandAST("Adarkar Wastes", "W", "U", 1)
	p := addBattlefieldWithAST(gs, 0, "Adarkar Wastes", 0, 0, ast, "land")
	p.SummoningSick = false

	if !IsManaAbility(p, 1) {
		t.Fatal("'{T}: Add {W} or {U}' (Choice of AddMana) must be a §605.1a mana ability")
	}
	before := len(gs.EventLog)
	if err := ActivateAbility(gs, 0, p, 1, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if n := stackPushesSince(gs, before); n != 0 {
		t.Errorf("mana ability must resolve inline (§605.3a), not push the stack; pushes=%d", n)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("colored tap must add 1 mana; pool=%d", gs.Seats[0].ManaPool)
	}
}

// BUG B — the colored tap applies the pain-land self_damage rider inline.
func TestComplexMana_PainlandColored_RiderDamage(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 40
	ast := painlandAST("Shivan Reef", "U", "R", 1)
	p := addBattlefieldWithAST(gs, 0, "Shivan Reef", 0, 0, ast, "land")
	p.SummoningSick = false

	if err := ActivateAbility(gs, 0, p, 1, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if gs.Seats[0].Life != 39 {
		t.Errorf("colored pain-land tap must deal 1 damage to controller: life=%d, want 39", gs.Seats[0].Life)
	}
}

// The free "{T}: Add {C}" tap on a pain land deals NO damage — the rider
// belongs to the colored tap only.
func TestComplexMana_PainlandFreeColorless_NoDamage(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 40
	ast := painlandAST("Karplusan Forest", "R", "G", 1)
	p := addBattlefieldWithAST(gs, 0, "Karplusan Forest", 0, 0, ast, "land")
	p.SummoningSick = false

	if !IsManaAbility(p, 0) {
		t.Fatal("'{T}: Add {C}' must be a mana ability")
	}
	if err := ActivateAbility(gs, 0, p, 0, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if gs.Seats[0].Life != 40 {
		t.Errorf("free {C} tap must deal NO damage: life=%d, want 40", gs.Seats[0].Life)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("free {C} tap must add 1 mana; pool=%d", gs.Seats[0].ManaPool)
	}
}

// A single-ability "{T}: Add {C}{C}" + self_damage Static (Ancient-Tomb
// shape) with NO per_card handler: that ability is the card's SOLE mana
// ability, so it is the painful one and the rider applies even though the
// mana is pure colorless.
func TestComplexMana_SoleColorlessAbility_RiderApplies(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 40
	ast := &gameast.CardAST{
		Name: "Generic Painful Tomb", // no per_card handler
		Abilities: []gameast.Ability{
			&gameast.Activated{Cost: gameast.Cost{Tap: true},
				Effect: &gameast.AddMana{Pool: []gameast.ManaSymbol{
					{Color: []string{"C"}}, {Color: []string{"C"}}}}},
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "self_damage", Args: []interface{}{2}}},
		},
	}
	p := addBattlefieldWithAST(gs, 0, "Generic Painful Tomb", 0, 0, ast, "land")
	p.SummoningSick = false

	if err := ActivateAbility(gs, 0, p, 0, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if gs.Seats[0].Life != 38 {
		t.Errorf("sole colorless painful tap must deal 2 damage: life=%d, want 38", gs.Seats[0].Life)
	}
}

// A dual land with NO rider (check/temple shape "{T}: Add {U} or {R}")
// resolves inline with no spurious damage.
func TestComplexMana_DualNoRider_InlineNoDamage(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 40
	ast := &gameast.CardAST{
		Name: "Temple of Epiphany",
		Abilities: []gameast.Ability{
			&gameast.Activated{Cost: gameast.Cost{Tap: true},
				Effect: &gameast.Choice{Options: []gameast.Effect{
					&gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{"U"}}}},
					&gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{"R"}}}},
				}, Pick: *gameast.NumInt(1)}},
		},
	}
	p := addBattlefieldWithAST(gs, 0, "Temple of Epiphany", 0, 0, ast, "land")
	p.SummoningSick = false

	before := len(gs.EventLog)
	if err := ActivateAbility(gs, 0, p, 0, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if n := stackPushesSince(gs, before); n != 0 {
		t.Errorf("dual mana ability must resolve inline; stack pushes=%d", n)
	}
	if gs.Seats[0].Life != 40 {
		t.Errorf("dual land with no rider must deal no damage: life=%d", gs.Seats[0].Life)
	}
}

// CLASSIFICATION EDGE — effectProducesMana unit coverage of the Choice arm.
func TestComplexMana_EffectProducesMana_Choice(t *testing.T) {
	allMana := &gameast.Choice{Options: []gameast.Effect{
		&gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{"W"}}}},
		&gameast.AddMana{AnyColorCount: 1},
	}}
	if !effectProducesMana(allMana) {
		t.Error("Choice of all-AddMana options must produce mana")
	}
	// A modal ability with a non-mana mode keeps the conservative
	// stack-using classification (NOT a mana ability).
	mixed := &gameast.Choice{Options: []gameast.Effect{
		&gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{"W"}}}},
		&gameast.Draw{},
	}}
	if effectProducesMana(mixed) {
		t.Error("Choice with a non-mana mode must NOT be classified as mana-producing")
	}
	empty := &gameast.Choice{}
	if effectProducesMana(empty) {
		t.Error("empty Choice must not be mana-producing")
	}
}
