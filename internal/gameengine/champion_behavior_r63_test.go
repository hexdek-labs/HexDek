package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// champion_behavior_r63_test.go — behavioral regressions for the Champion
// primitive (CR §702.72), which was entirely unimplemented before r63 (only a
// header comment + a recognized keyword). These pin the ETB exile-or-sacrifice
// and the LTB return, built on the canonical ExileLinked / ReturnLinkedExile
// linked-exile infra. (Live ETB/LTB auto-invoke is a documented follow-up.)

func newChampionGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(72)), nil)
}

func championPerm(gs *GameState, seat int, name, typ string) *Permanent {
	p := addKWCombatBattlefield(gs, seat, name, 3, 3, "creature")
	p.Card.AST = &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "champion", Args: []interface{}{typ}}},
	}
	return p
}

func champCardInZone(seat *Seat, name, zone string) bool {
	var z []*Card
	switch zone {
	case "exile":
		z = seat.Exile
	case "graveyard":
		z = seat.Graveyard
	}
	for _, c := range z {
		if c != nil && c.Name == name {
			return true
		}
	}
	return false
}

func champOnBattlefield(gs *GameState, seat int, name string) bool {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			return true
		}
	}
	return false
}

// (a)+(d)+(f) Champion's ETB exiles another creature of the named type you
// control (to EXILE, not graveyard), linked to the champion.
func TestChampion_ExilesEligibleCreatureLinked(t *testing.T) {
	gs := newChampionGame(t)
	champ := championPerm(gs, 0, "Mistbind Clique", "faerie")
	elf := addKWCombatBattlefield(gs, 0, "Faerie Token", 1, 1, "creature", "faerie")

	returned := ApplyChampion(gs, champ)

	if returned != elf.Card {
		t.Fatalf("(a) ApplyChampion should exile and return the eligible faerie's card")
	}
	if champOnBattlefield(gs, 0, "Faerie Token") {
		t.Error("(a) the championed creature must leave the battlefield")
	}
	if !champCardInZone(gs.Seats[0], "Faerie Token", "exile") {
		t.Error("(a) the championed card must go to EXILE")
	}
	if champCardInZone(gs.Seats[0], "Faerie Token", "graveyard") {
		t.Error("(a) the championed card must NOT go to the graveyard")
	}
	// (f) linked to the champion.
	if elf.Card.ExiledByTimestamp != champ.Timestamp {
		t.Errorf("(f) championed card must be linked to the champion's timestamp; got %d want %d",
			elf.Card.ExiledByTimestamp, champ.Timestamp)
	}
	if len(champ.LinkedExile) != 1 || champ.LinkedExile[0] != elf.Card {
		t.Error("(f) champion must track the championed card in LinkedExile")
	}
	// champion stays on the battlefield (it championed successfully).
	if !champOnBattlefield(gs, 0, "Mistbind Clique") {
		t.Error("(a) the champion stays on the battlefield after championing")
	}
}

// (b) If you control no eligible creature of the type, the champion is
// SACRIFICED.
func TestChampion_SacrificedWhenNoEligible(t *testing.T) {
	gs := newChampionGame(t)
	champ := championPerm(gs, 0, "Mistbind Clique", "faerie")
	// Only a non-matching creature is present.
	addKWCombatBattlefield(gs, 0, "Goblin", 2, 2, "creature", "goblin")

	returned := ApplyChampion(gs, champ)

	if returned != nil {
		t.Error("(b) with no eligible creature, nothing should be championed")
	}
	if champOnBattlefield(gs, 0, "Mistbind Clique") {
		t.Error("(b) the champion must be sacrificed when it can't champion")
	}
	if !champCardInZone(gs.Seats[0], "Mistbind Clique", "graveyard") {
		t.Error("(b) the sacrificed champion should be in the graveyard")
	}
	// The non-matching creature is untouched.
	if !champOnBattlefield(gs, 0, "Goblin") {
		t.Error("(b) a non-matching creature must not be championed")
	}
}

// (c)+(e) When the champion leaves the battlefield, the championed card returns
// to the battlefield under its owner's control.
func TestChampion_ReturnsChampionedOnLTB(t *testing.T) {
	gs := newChampionGame(t)
	champ := championPerm(gs, 0, "Mistbind Clique", "faerie")
	addKWCombatBattlefield(gs, 0, "Faerie Token", 1, 1, "creature", "faerie")
	ApplyChampion(gs, champ)
	if !champCardInZone(gs.Seats[0], "Faerie Token", "exile") {
		t.Fatal("setup: faerie should be exiled")
	}

	// Champion leaves — the championed card returns.
	ChampionReturnOnLTB(gs, champ)

	if !champOnBattlefield(gs, 0, "Faerie Token") {
		t.Error("(c) the championed card must return to the battlefield when the champion leaves")
	}
	if champCardInZone(gs.Seats[0], "Faerie Token", "exile") {
		t.Error("(c) the championed card must leave exile on return")
	}
	if len(champ.LinkedExile) != 0 {
		t.Error("(c) the champion's linked-exile tracking should clear after the return")
	}
}

// Type filter: champion only exiles a creature of the named type (an Elf
// champion can't champion a Goblin).
func TestChampion_TypeFilterRespected(t *testing.T) {
	gs := newChampionGame(t)
	champ := championPerm(gs, 0, "Elvish Champion", "elf")
	addKWCombatBattlefield(gs, 0, "Goblin", 2, 2, "creature", "goblin")
	elf := addKWCombatBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature", "elf")

	returned := ApplyChampion(gs, champ)

	if returned != elf.Card {
		t.Error("type filter: champion must exile the matching Elf, not the Goblin")
	}
	if !champOnBattlefield(gs, 0, "Goblin") {
		t.Error("type filter: the non-matching Goblin must be left alone")
	}
}
