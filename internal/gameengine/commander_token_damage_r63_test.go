package gameengine

import "testing"

// CR §903.3a / §903.10a — a commander is a specific designated CARD. A token
// copy of a commander (Helm of the Host, Cackling Counterpart, Mimic Vat, …)
// shares the name but is NOT a commander: its combat damage must not count
// toward the 21-damage loss clock. Before r63, IsCommanderCard matched on name
// alone, so a name-sharing token wrongly accumulated commander damage.

func TestIsCommanderCard_TokenCopyIsNotCommander(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")

	realCmdr := &Card{Name: "B", Types: []string{"creature", "legendary"}}
	if !IsCommanderCard(gs, 1, realCmdr) {
		t.Fatal("the real commander card 'B' should be recognized as seat 1's commander")
	}

	tokenCopy := &Card{Name: "B", Types: []string{"token", "creature", "legendary"}}
	if IsCommanderCard(gs, 1, tokenCopy) {
		t.Fatal("a TOKEN named 'B' must NOT be treated as a commander (CR §903.3a)")
	}
}

func TestCommanderDamage_TokenCopyDoesNotCount(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")

	// A token copy of seat 1's commander "B" deals 21 combat damage to seat 0.
	tokenPerm := &Permanent{
		Card:       &Card{Name: "B", Owner: 1, Types: []string{"token", "creature", "legendary"}, BasePower: 21, BaseToughness: 21},
		Controller: 1,
		Owner:      1,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, tokenPerm)

	applyCombatDamageToPlayer(gs, tokenPerm, 21, 0)
	StateBasedActions(gs)

	if got := CommanderDamageFrom(gs.Seats[0], 1, "B"); got != 0 {
		t.Fatalf("token copy must not accumulate commander damage, got %d", got)
	}
	if gs.Seats[0].Lost {
		t.Fatal("seat 0 must NOT lose to 21 combat damage from a token copy of a commander")
	}
}

func TestCommanderDamage_RealCommanderStillCounts(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")

	// The real commander "B" (non-token) deals 21 combat damage to seat 0.
	realPerm := &Permanent{
		Card:       &Card{Name: "B", Owner: 1, Types: []string{"creature", "legendary"}, BasePower: 21, BaseToughness: 21},
		Controller: 1,
		Owner:      1,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, realPerm)

	applyCombatDamageToPlayer(gs, realPerm, 21, 0)
	StateBasedActions(gs)

	if got := CommanderDamageFrom(gs.Seats[0], 1, "B"); got != 21 {
		t.Fatalf("real commander must accumulate commander damage, got %d", got)
	}
	if !gs.Seats[0].Lost {
		t.Fatal("seat 0 must lose to 21 combat damage from the real commander (CR §704.6c)")
	}
}
