package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 EDICT / forced-sacrifice mechanic audit.

// sacStub is a SacrificeChooser test hat: records the seat-call order and can
// pick a specific creature by name (to prove the AFFECTED player chooses).
type sacStub struct {
	GreedyHatStub
	calls *[]int
	pick  map[int]string
}

func (h *sacStub) ChooseSacrifice(gs *GameState, seat int, src *Permanent, reason string, cands []*Permanent) *Permanent {
	if h.calls != nil {
		*h.calls = append(*h.calls, seat)
	}
	if name, ok := h.pick[seat]; ok {
		for _, c := range cands {
			if c != nil && c.Card != nil && c.Card.Name == name {
				return c
			}
		}
	}
	if len(cands) > 0 {
		return cands[0]
	}
	return nil
}

func sacEvents(gs *GameState) []int {
	var seats []int
	for i := range gs.EventLog {
		if gs.EventLog[i].Kind == "sacrifice" {
			seats = append(seats, gs.EventLog[i].Seat)
		}
	}
	return seats
}

// (a) "each player sacrifices" resolves in APNAP order — active player first.
func TestEdict_APNAPOrder(t *testing.T) {
	gs := NewGameState(3, nil, nil)
	gs.Active = 1 // non-zero active to distinguish APNAP from seat-index order
	var order []int
	for i := 0; i < 3; i++ {
		gs.Seats[i].Hat = &sacStub{calls: &order}
		addBattlefield(gs, i, "Bear", 2, 2, "creature")
	}
	src := addBattlefield(gs, 1, "Edict Source", 0, 0, "sorcery")
	resolveSacrifice(gs, src, &gameast.Sacrifice{Actor: "each_player", Query: gameast.Filter{Base: "creature"}})

	want := []int{1, 2, 0} // APNAP from active=1
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Fatalf("edict must poll seats in APNAP order %v; got %v", want, order)
	}
	if se := sacEvents(gs); len(se) != 3 || se[0] != 1 || se[1] != 2 || se[2] != 0 {
		t.Fatalf("sacrifice events must fire in APNAP order [1 2 0]; got %v", se)
	}
}

// (a) each affected player CHOOSES which of their own creatures to sacrifice —
// not an engine "lowest power" pick.
func TestEdict_ControllerChoosesNotLowestPower(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	// Seat 1 holds a weak 1/1 and a strong 5/5. Engine default would sac the
	// 1/1; the controller's hat instead chooses the 5/5 (its decision).
	weak := addBattlefield(gs, 1, "Weakling", 1, 1, "creature")
	strong := addBattlefield(gs, 1, "Bruiser", 5, 5, "creature")
	_ = weak
	gs.Seats[1].Hat = &sacStub{pick: map[int]string{1: "Bruiser"}}
	gs.Seats[0].Hat = &sacStub{}
	addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	src := addBattlefield(gs, 0, "Edict", 0, 0, "sorcery")

	resolveSacrifice(gs, src, &gameast.Sacrifice{Actor: "each_opponent", Query: gameast.Filter{Base: "creature"}})

	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == strong {
			stillThere = true
		}
	}
	if stillThere {
		t.Fatal("the affected player chose to sacrifice Bruiser (5/5); it must be gone, not the 1/1")
	}
}

// (b) hexproof / shroud do NOT prevent an edict — nothing is targeted.
func TestEdict_HexproofDoesNotPrevent(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	hexed := addBattlefield(gs, 1, "Hexproof Bear", 2, 2, "creature")
	hexed.Flags["hexproof"] = 1
	hexed.Flags["shroud"] = 1
	src := addBattlefield(gs, 0, "Edict", 0, 0, "sorcery")

	resolveSacrifice(gs, src, &gameast.Sacrifice{Actor: "each_opponent", Query: gameast.Filter{Base: "creature"}})

	for _, p := range gs.Seats[1].Battlefield {
		if p == hexed {
			t.Fatal("edict must sacrifice the hexproof/shroud creature — sacrifice is not targeted")
		}
	}
}

// (c) a player with zero matching creatures sacrifices nothing, and the effect
// still resolves for the other players (no error/skip).
func TestEdict_ZeroCreaturesNoError(t *testing.T) {
	gs := NewGameState(3, nil, nil)
	gs.Active = 0
	// seat 1 has no creature; seats 0 and 2 do.
	addBattlefield(gs, 0, "Bear0", 2, 2, "creature")
	addBattlefield(gs, 2, "Bear2", 2, 2, "creature")
	src := addBattlefield(gs, 0, "Edict", 0, 0, "sorcery")

	resolveSacrifice(gs, src, &gameast.Sacrifice{Actor: "each_player", Query: gameast.Filter{Base: "creature"}})

	if len(gs.Seats[0].Battlefield) != 1 { // only the Edict source remains
		t.Errorf("seat 0 should have sacrificed its creature; bf=%d", len(gs.Seats[0].Battlefield))
	}
	if c := sacEvents(gs); len(c) != 2 {
		t.Errorf("exactly the two players with creatures sacrifice (seat 1 has none); got %v", c)
	}
}

// (e) a sacrificed TOKEN is a legal sacrifice and leaves the battlefield
// (graveyard then ceases via SBA).
func TestEdict_TokenIsLegalSacrificeAndCeases(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	tok := addBattlefield(gs, 1, "Saproling", 1, 1, "token", "creature")
	src := addBattlefield(gs, 0, "Edict", 0, 0, "sorcery")

	resolveSacrifice(gs, src, &gameast.Sacrifice{Actor: "each_opponent", Query: gameast.Filter{Base: "creature"}})
	StateBasedActions(gs)

	for _, p := range gs.Seats[1].Battlefield {
		if p == tok {
			t.Fatal("token must have been sacrificed off the battlefield")
		}
	}
	gone := true
	for _, c := range gs.Seats[1].Graveyard {
		if c == tok.Card {
			gone = false
		}
	}
	if !gone {
		t.Error("a sacrificed token must cease to exist (not linger in graveyard) after SBA")
	}
}
