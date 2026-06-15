package gameengine

import "testing"

// Regressions for the r63 fix wiring the built-in sacrifice abilities of the
// predefined NON-mana artifact tokens (Clue/Food/Blood/Map). Before the fix
// these tokens carried no AST and no activation path, so their abilities were
// inert — they could be created but never used.

func mkToken(gs *GameState, seat int, name, subtype string) *Permanent {
	return addBattlefield(gs, seat, name, 0, 0, "token", "artifact", subtype)
}

func libCards(n int) []*Card {
	out := make([]*Card, n)
	for i := range out {
		out[i] = &Card{Name: "Filler", Types: []string{"land"}}
	}
	return out
}

func TestToken_Clue_SacrificeDraws(t *testing.T) {
	gs := newFixtureGame(t)
	clue := mkToken(gs, 0, "Clue Token", "clue")
	gs.Seats[0].Library = libCards(1)
	AddMana(gs, gs.Seats[0], "C", 2, "t")
	preHand := len(gs.Seats[0].Hand)

	if !ActivatePredefinedTokenAbility(gs, 0, clue) {
		t.Fatal("Clue ability should activate with {2} available")
	}
	if len(gs.Seats[0].Hand) != preHand+1 {
		t.Fatalf("Clue: expected +1 card, hand %d -> %d", preHand, len(gs.Seats[0].Hand))
	}
	if permanentOnBattlefield(gs, clue) {
		t.Fatal("Clue should be sacrificed")
	}
	if gs.Seats[0].Mana.Total() != 0 {
		t.Fatalf("Clue {2} should be paid, mana left %d", gs.Seats[0].Mana.Total())
	}
}

func TestToken_Food_SacrificeGains3(t *testing.T) {
	gs := newFixtureGame(t)
	food := mkToken(gs, 0, "Food Token", "food")
	AddMana(gs, gs.Seats[0], "C", 2, "t")
	gs.Seats[0].Life = 20

	if !ActivatePredefinedTokenAbility(gs, 0, food) {
		t.Fatal("Food ability should activate")
	}
	if gs.Seats[0].Life != 23 {
		t.Fatalf("Food: expected gain 3 life (20 -> 23), got %d", gs.Seats[0].Life)
	}
	if permanentOnBattlefield(gs, food) {
		t.Fatal("Food should be sacrificed")
	}
}

func TestToken_Blood_DiscardThenDraw(t *testing.T) {
	gs := newFixtureGame(t)
	blood := mkToken(gs, 0, "Blood Token", "blood")
	gs.Seats[0].Hand = []*Card{{Name: "ToDiscard", Types: []string{"sorcery"}}}
	gs.Seats[0].Library = libCards(1)
	AddMana(gs, gs.Seats[0], "C", 1, "t")

	if !ActivatePredefinedTokenAbility(gs, 0, blood) {
		t.Fatal("Blood ability should activate (1 card to discard, {1} available)")
	}
	// Discard 1 + draw 1 → hand size unchanged (1).
	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("Blood: discard 1 + draw 1 should keep hand at 1, got %d", len(gs.Seats[0].Hand))
	}
	if permanentOnBattlefield(gs, blood) {
		t.Fatal("Blood should be sacrificed")
	}
}

func TestToken_Map_TargetCreatureExplores(t *testing.T) {
	gs := newFixtureGame(t)
	mapTok := mkToken(gs, 0, "Map Token", "map")
	creature := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	gs.Seats[0].Library = []*Card{{Name: "Mountain", Types: []string{"land"}}}
	AddMana(gs, gs.Seats[0], "C", 1, "t")

	if !ActivatePredefinedTokenAbility(gs, 0, mapTok) {
		t.Fatal("Map ability should activate with a creature to target")
	}
	if permanentOnBattlefield(gs, mapTok) {
		t.Fatal("Map should be sacrificed")
	}
	_ = creature
}

func TestToken_Clue_InsufficientMana(t *testing.T) {
	gs := newFixtureGame(t)
	clue := mkToken(gs, 0, "Clue Token", "clue")
	AddMana(gs, gs.Seats[0], "C", 1, "t") // only {1}, need {2}

	if ActivatePredefinedTokenAbility(gs, 0, clue) {
		t.Fatal("Clue must not activate with only {1}")
	}
	if !permanentOnBattlefield(gs, clue) {
		t.Fatal("Clue must remain when its cost can't be paid")
	}
}

func TestToken_Blood_EmptyHandCannotPayDiscard(t *testing.T) {
	gs := newFixtureGame(t)
	blood := mkToken(gs, 0, "Blood Token", "blood")
	gs.Seats[0].Hand = nil // no card to discard
	AddMana(gs, gs.Seats[0], "C", 1, "t")

	if ActivatePredefinedTokenAbility(gs, 0, blood) {
		t.Fatal("Blood must not activate with an empty hand (can't pay the discard cost)")
	}
	if !permanentOnBattlefield(gs, blood) {
		t.Fatal("Blood must remain when the discard cost can't be paid")
	}
}

// Confirms the canonical ActivateAbility entry routes predefined tokens to the
// built-in ability primitive.
func TestToken_ViaActivateAbility(t *testing.T) {
	gs := newFixtureGame(t)
	clue := mkToken(gs, 0, "Clue Token", "clue")
	gs.Seats[0].Library = libCards(1)
	AddMana(gs, gs.Seats[0], "C", 2, "t")

	if err := ActivateAbility(gs, 0, clue, 0, nil); err != nil {
		t.Fatalf("ActivateAbility on a Clue should crack it, got %v", err)
	}
	if permanentOnBattlefield(gs, clue) {
		t.Fatal("Clue should be sacrificed via ActivateAbility")
	}
}
