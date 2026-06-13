package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 scaffold-kind regressions: generic colored_cost_reduce. Each pins
// that a representative card now actually reduces the cost of matching
// spells its controller casts — previously inert unless the card had a
// dedicated name-case in ScanCostModifiers.

// reducerPerm builds a battlefield permanent for `seat` carrying a single
// colored_cost_reduce static (filter + "{N}").
func reducerPerm(gs *GameState, seat int, name, filter, amount string) *Permanent {
	p := &Permanent{
		Card: &Card{
			Name:  name,
			Owner: seat,
			Types: []string{"enchantment"},
			AST: &gameast.CardAST{
				Name: name,
				Abilities: []gameast.Ability{
					&gameast.Static{Modification: &gameast.Modification{
						ModKind: "colored_cost_reduce",
						Args:    []interface{}{filter, amount},
					}},
				},
			},
		},
		Controller: seat,
		Owner:      seat,
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func TestScaffold_ColoredCostReduce_WhiteCreature(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	reducerPerm(gs, 0, "Oketra's Monument", "white creature", "{1}")

	whiteCreature := &Card{Name: "Devoted Crop-Mate", Types: []string{"creature", "cost:4"}, Colors: []string{"W"}}
	if c := CalculateTotalCost(gs, whiteCreature, 0); c != 3 {
		t.Fatalf("white creature should cost 4-1=3, got %d", c)
	}
	// Black creature: not white -> no reduction.
	blackCreature := &Card{Name: "Gravedigger", Types: []string{"creature", "cost:4"}, Colors: []string{"B"}}
	if c := CalculateTotalCost(gs, blackCreature, 0); c != 4 {
		t.Fatalf("black creature should be unreduced 4, got %d", c)
	}
	// White noncreature (instant): matches color but not the 'creature'
	// conjunct -> no reduction.
	whiteInstant := &Card{Name: "Swords to Plowshares", Types: []string{"instant", "cost:1"}, Colors: []string{"W"}}
	if c := CalculateTotalCost(gs, whiteInstant, 0); c != 1 {
		t.Fatalf("white instant should be unreduced 1, got %d", c)
	}
}

func TestScaffold_ColoredCostReduce_InstantAndSorceryOr(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	reducerPerm(gs, 0, "Mocking Sprite", "instant and sorcery", "{1}")

	inst := &Card{Name: "Counterspell", Types: []string{"instant", "cost:2"}, Colors: []string{"U"}}
	if c := CalculateTotalCost(gs, inst, 0); c != 1 {
		t.Fatalf("instant should cost 2-1=1, got %d", c)
	}
	sorc := &Card{Name: "Divination", Types: []string{"sorcery", "cost:3"}, Colors: []string{"U"}}
	if c := CalculateTotalCost(gs, sorc, 0); c != 2 {
		t.Fatalf("sorcery should cost 3-1=2, got %d", c)
	}
	creature := &Card{Name: "Grizzly Bears", Types: []string{"creature", "cost:2"}, Colors: []string{"G"}}
	if c := CalculateTotalCost(gs, creature, 0); c != 2 {
		t.Fatalf("creature should be unreduced 2, got %d", c)
	}
}

func TestScaffold_ColoredCostReduce_SubtypeOr(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	reducerPerm(gs, 0, "Brighthearth Banneret", "elemental spells and warrior", "{1}")

	elemental := &Card{Name: "Flamekin Harbinger", Types: []string{"creature", "elemental", "cost:1"}, Colors: []string{"R"}}
	if c := CalculateTotalCost(gs, elemental, 0); c != 0 {
		t.Fatalf("elemental should cost 1-1=0, got %d", c)
	}
	warrior := &Card{Name: "Some Warrior", Types: []string{"creature", "warrior", "cost:3"}, Colors: []string{"R"}}
	if c := CalculateTotalCost(gs, warrior, 0); c != 2 {
		t.Fatalf("warrior should cost 3-1=2, got %d", c)
	}
	goblin := &Card{Name: "Some Goblin", Types: []string{"creature", "goblin", "cost:3"}, Colors: []string{"R"}}
	if c := CalculateTotalCost(gs, goblin, 0); c != 3 {
		t.Fatalf("goblin should be unreduced 3, got %d", c)
	}
}

func TestScaffold_ColoredCostReduce_OnlyYouCast(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Reducer controlled by the OPPONENT (seat 1).
	reducerPerm(gs, 1, "Oketra's Monument", "white creature", "{1}")

	// Seat 0 casts a white creature — opponent's "you cast" reducer must
	// not apply.
	whiteCreature := &Card{Name: "Devoted Crop-Mate", Types: []string{"creature", "cost:4"}, Colors: []string{"W"}}
	if c := CalculateTotalCost(gs, whiteCreature, 0); c != 4 {
		t.Fatalf("opponent's reducer must not help my cast; want 4, got %d", c)
	}
}

func TestScaffold_ColoredCostReduce_NoDoubleCountWithNamedMedallion(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Ruby Medallion has a dedicated name-case AND carries
	// colored_cost_reduce in its AST. The generic default must NOT fire
	// for it (it's name-handled) — red spell reduced by exactly 1.
	reducerPerm(gs, 0, "Ruby Medallion", "red", "{1}")

	redSpell := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:3"}, Colors: []string{"R"}}
	if c := CalculateTotalCost(gs, redSpell, 0); c != 2 {
		t.Fatalf("Ruby Medallion must reduce red by exactly 1 (3->2), got %d (double-count if 1)", c)
	}
}

func TestScaffold_ColoredCostReduce_VariableAmountSkipped(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// "{X}" is not a plain numeric amount -> reducer skipped, no reduction.
	reducerPerm(gs, 0, "Weird Reducer", "red", "{X}")

	redSpell := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:3"}, Colors: []string{"R"}}
	if c := CalculateTotalCost(gs, redSpell, 0); c != 3 {
		t.Fatalf("variable-amount reducer should be skipped; want 3, got %d", c)
	}
}

func TestScaffold_ParseBraceAmount(t *testing.T) {
	cases := map[string]int{"{1}": 1, "{2}": 2, "{10}": 10, "{X}": 0, "": 0, "{}": 0, "1": 1}
	for in, want := range cases {
		if got := parseBraceAmount(in); got != want {
			t.Fatalf("parseBraceAmount(%q)=%d want %d", in, got, want)
		}
	}
}
