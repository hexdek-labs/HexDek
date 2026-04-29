package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// P1 #6: Ninjutsu Tests
// ---------------------------------------------------------------------------

func TestNinjutsu_SwapsUnblockedAttacker(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	// Seat 0 has an unblocked attacker.
	attacker := &Permanent{
		Card: &Card{
			Name:          "Ornithopter",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 2,
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	// Seat 0 has a ninja in hand with ninjutsu.
	ninja := &Card{
		Name:          "Ninja of the Deep Hours",
		Owner:         0,
		Types:         []string{"creature", "cost:4"},
		BasePower:     2,
		BaseToughness: 2,
		AST: &gameast.CardAST{
			Name: "Ninja of the Deep Hours",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
			},
		},
	}
	seat.Hand = append(seat.Hand, ninja)
	seat.ManaPool = 5

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{
		attacker: nil, // unblocked
	}

	result := CheckNinjutsu(gs, 0, attackers, blockerMap)

	// Attacker should be bounced to hand.
	foundBounced := false
	for _, c := range seat.Hand {
		if c != nil && c.Name == "Ornithopter" {
			foundBounced = true
		}
	}
	if !foundBounced {
		t.Fatal("bounced attacker should be in hand")
	}

	// Ninja should be on battlefield.
	foundNinja := false
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Ninja of the Deep Hours" {
			foundNinja = true
			if !permFlag(p, flagAttacking) {
				t.Fatal("ninja should be attacking")
			}
			if permFlag(p, flagDeclaredAttacker) {
				t.Fatal("ninja should NOT be a declared attacker (CR §702.49b)")
			}
			if !p.Tapped {
				t.Fatal("ninja should be tapped")
			}
			def, ok := AttackerDefender(p)
			if !ok || def != 1 {
				t.Fatalf("ninja should be attacking seat 1, got %d", def)
			}
		}
	}
	if !foundNinja {
		t.Fatal("ninja should be on battlefield")
	}

	// Result should contain the ninja and not the old attacker.
	foundNinjaInResult := false
	foundOldInResult := false
	for _, p := range result {
		if p.Card.Name == "Ninja of the Deep Hours" {
			foundNinjaInResult = true
		}
		if p.Card.Name == "Ornithopter" {
			foundOldInResult = true
		}
	}
	if !foundNinjaInResult {
		t.Fatal("result attackers should contain the ninja")
	}
	if foundOldInResult {
		t.Fatal("result attackers should NOT contain the bounced attacker")
	}

	// Mana should have been paid.
	if seat.ManaPool >= 5 {
		t.Fatal("mana should have been spent on ninjutsu cost")
	}

	// Verify ninjutsu event.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "ninjutsu" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'ninjutsu' event in log")
	}
}

func TestNinjutsu_NoActivationWithoutUnblocked(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card: &Card{
			Name:  "Ornithopter",
			Owner: 0,
			Types: []string{"creature"},
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	seat.Battlefield = append(seat.Battlefield, attacker)

	blocker := &Permanent{
		Card:       &Card{Name: "Bear", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, blocker)

	ninja := &Card{
		Name:  "Ninja",
		Owner: 0,
		Types: []string{"creature", "cost:4"},
		AST: &gameast.CardAST{
			Name: "Ninja",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
			},
		},
	}
	seat.Hand = append(seat.Hand, ninja)
	seat.ManaPool = 5

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{
		attacker: {blocker}, // blocked!
	}

	result := CheckNinjutsu(gs, 0, attackers, blockerMap)

	// Nothing should have changed.
	if len(result) != 1 || result[0] != attacker {
		t.Fatal("no ninjutsu should have activated with blocked attacker")
	}

	// Ninja should still be in hand.
	foundInHand := false
	for _, c := range seat.Hand {
		if c == ninja {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Fatal("ninja should still be in hand when no unblocked attackers")
	}
}

func TestNinjutsu_NoActivationWithoutMana(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	attacker := &Permanent{
		Card:       &Card{Name: "Ornithopter", Owner: 0, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	setPermFlag(attacker, flagAttacking, true)
	setAttackerDefender(attacker, 1)
	seat.Battlefield = append(seat.Battlefield, attacker)

	ninja := &Card{
		Name:  "Ninja",
		Owner: 0,
		Types: []string{"creature", "cost:4"},
		AST: &gameast.CardAST{
			Name: "Ninja",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ninjutsu"},
			},
		},
	}
	seat.Hand = append(seat.Hand, ninja)
	seat.ManaPool = 0 // no mana

	attackers := []*Permanent{attacker}
	blockerMap := map[*Permanent][]*Permanent{attacker: nil}

	result := CheckNinjutsu(gs, 0, attackers, blockerMap)

	if len(result) != 1 || result[0] != attacker {
		t.Fatal("no ninjutsu should have activated without mana")
	}
}
