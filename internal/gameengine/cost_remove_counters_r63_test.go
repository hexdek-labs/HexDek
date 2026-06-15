package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// makeRemoveCounterPerm builds a creature with a single activated ability whose
// ONLY cost is a raw-Extra "remove N <kind> counters from this creature" string
// (the unstructured shape the parser emits for Elvish Farmer & ~346 siblings),
// resolving to a 1/1 Saproling token.
func makeRemoveCounterPerm(gs *GameState, seat int, name, extraCost string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"creature"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost: gameast.Cost{Extra: []string{extraCost}},
					Effect: &gameast.CreateToken{
						Count: gameast.NumberOrRef{IsInt: true, Int: 1},
						PT:    &[2]int{1, 1},
						Types: []string{"green", "saproling"},
					},
				},
			},
		},
	}
	perm := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

func countTokensNamed(gs *GameState, seat int, substr string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsToken() {
			n++
		}
	}
	_ = substr
	return n
}

// TestRemoveCounterCostSpec_Parsing pins the cost parser: structured fields and
// the raw-Extra forms parse; ambiguous / variable shapes decline (ok=false) so
// enforcement is purely additive and never blocks a legal activation.
func TestRemoveCounterCostSpec_Parsing(t *testing.T) {
	three := 3
	cases := []struct {
		name     string
		cost     gameast.Cost
		wantN    int
		wantKind string
		wantOK   bool
	}{
		{"structured", gameast.Cost{RemoveCountersN: &three, RemoveCountersKnd: "spore"}, 3, "spore", true},
		{"extra-three-spore", gameast.Cost{Extra: []string{"remove three spore counters from this creature"}}, 3, "spore", true},
		{"extra-a-plus", gameast.Cost{Extra: []string{"remove a +1/+1 counter"}}, 1, "+1/+1", true},
		{"extra-two-charge", gameast.Cost{Extra: []string{"Remove two charge counters from this artifact"}}, 2, "charge", true},
		{"extra-no-kind", gameast.Cost{Extra: []string{"remove a counter"}}, 0, "", false},
		{"extra-variable-x", gameast.Cost{Extra: []string{"remove x loyalty counters"}}, 0, "", false},
		{"unrelated-extra", gameast.Cost{Extra: []string{"discard your hand"}}, 0, "", false},
		{"empty", gameast.Cost{}, 0, "", false},
	}
	for _, c := range cases {
		n, kind, ok := RemoveCounterCostSpec(c.cost)
		if ok != c.wantOK || n != c.wantN || kind != c.wantKind {
			t.Errorf("%s: got (n=%d kind=%q ok=%v), want (n=%d kind=%q ok=%v)",
				c.name, n, kind, ok, c.wantN, c.wantKind, c.wantOK)
		}
	}
}

// TestActivate_RemoveCounterCost_ElvishFarmer pins the firespot fix end-to-end:
// the "remove three spore counters: create a Saproling" ability is FREE no
// longer — it rejects with < 3 spore counters (no token), and pays exactly 3
// when affordable. This is the board-blowup root cause (it could be activated
// unboundedly while spore counters accrue only 1/turn).
func TestActivate_RemoveCounterCost_ElvishFarmer(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Life, gs.Seats[1].Life = 20, 20
	perm := makeRemoveCounterPerm(gs, 0, "Elvish Farmer", "remove three spore counters from this creature")

	// Two spore counters — cannot pay.
	perm.Counters["spore"] = 2
	if err := ActivateAbility(gs, 0, perm, 0, nil); err == nil {
		t.Fatal("activation should REJECT with only 2 spore counters (cost is 3)")
	}
	if perm.Counters["spore"] != 2 {
		t.Errorf("rejected activation must not consume counters, got %d", perm.Counters["spore"])
	}
	if got := countTokensNamed(gs, 0, "saproling"); got != 0 {
		t.Errorf("rejected activation must not create a token, got %d", got)
	}

	// Third spore counter — now affordable.
	perm.Counters["spore"] = 3
	if err := ActivateAbility(gs, 0, perm, 0, nil); err != nil {
		t.Fatalf("activation should SUCCEED with 3 spore counters, got %v", err)
	}
	if perm.Counters["spore"] != 0 {
		t.Errorf("activation should consume exactly 3 spore counters (3→0), got %d", perm.Counters["spore"])
	}
	if got := countTokensNamed(gs, 0, "saproling"); got != 1 {
		t.Errorf("affordable activation should create 1 Saproling, got %d", got)
	}
}

// TestActivate_RemoveCounterCost_Generic2 pins a generic "remove two <kind>
// counters: effect" ability — requires 2 of that counter.
func TestActivate_RemoveCounterCost_Generic2(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Life, gs.Seats[1].Life = 20, 20
	perm := makeRemoveCounterPerm(gs, 0, "Charge Engine", "remove two charge counters from this creature")

	perm.Counters["charge"] = 1
	if err := ActivateAbility(gs, 0, perm, 0, nil); err == nil {
		t.Fatal("should reject with 1 charge counter (cost is 2)")
	}
	if perm.Counters["charge"] != 1 {
		t.Errorf("rejected activation must not consume counters, got %d", perm.Counters["charge"])
	}

	perm.Counters["charge"] = 2
	if err := ActivateAbility(gs, 0, perm, 0, nil); err != nil {
		t.Fatalf("should succeed with 2 charge counters, got %v", err)
	}
	if perm.Counters["charge"] != 0 {
		t.Errorf("should consume 2 charge counters (2→0), got %d", perm.Counters["charge"])
	}
}
