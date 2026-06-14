package gameengine

import "testing"

// r63 DOUBLING replacement audit.

// (b) Doubling Season doubles a planeswalker's STARTING loyalty (§306.5g) — the
// old direct assignment skipped the doubler chain entirely.
func TestDoubler_LoyaltyDoublesUnderDoublingSeason(t *testing.T) {
	gs := newFixtureGame(t)
	ds := addBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
	RegisterDoublingSeason(gs, ds)
	pw := addBattlefield(gs, 0, "Garruk", 0, 0, "planeswalker")

	if got := EnterCounterCountWithDoublers(gs, pw, "loyalty", 3); got != 6 {
		t.Fatalf("Doubling Season must double starting loyalty 3 -> 6, got %d", got)
	}
}

// (c) two Doubling Seasons MULTIPLY loyalty (4x, not 3x).
func TestDoubler_TwoDoublingSeasons_LoyaltyQuadruples(t *testing.T) {
	gs := newFixtureGame(t)
	for _, name := range []string{"Doubling Season A", "Doubling Season B"} {
		ds := addBattlefield(gs, 0, name, 0, 0, "enchantment")
		ds.Card.Name = "Doubling Season" // both register as Doubling Season
		RegisterDoublingSeason(gs, ds)
	}
	pw := addBattlefield(gs, 0, "Garruk", 0, 0, "planeswalker")
	if got := EnterCounterCountWithDoublers(gs, pw, "loyalty", 3); got != 12 {
		t.Fatalf("two Doubling Seasons must MULTIPLY loyalty 3 -> 12 (not 9), got %d", got)
	}
}

// (f) without any doubler, the count passes through unchanged.
func TestDoubler_NoDoubler_LoyaltyPassthrough(t *testing.T) {
	gs := newFixtureGame(t)
	pw := addBattlefield(gs, 0, "Garruk", 0, 0, "planeswalker")
	if got := EnterCounterCountWithDoublers(gs, pw, "loyalty", 4); got != 4 {
		t.Fatalf("no doubler: loyalty must pass through unchanged 4 -> 4, got %d", got)
	}
}

// (d) Vorinclex HALVES an opponent's incoming loyalty (asymmetric arm).
func TestDoubler_VorinclexHalvesOpponentLoyalty(t *testing.T) {
	gs := newFixtureGame(t)
	vor := addBattlefield(gs, 0, "Vorinclex, Monstrous Raider", 0, 0, "creature")
	RegisterVorinclexMonstrousRaider(gs, vor)
	oppPW := addBattlefield(gs, 1, "Garruk", 0, 0, "planeswalker") // opponent's PW
	if got := EnterCounterCountWithDoublers(gs, oppPW, "loyalty", 5); got != 2 {
		t.Fatalf("Vorinclex must halve an opponent's starting loyalty 5 -> 2 (round down), got %d", got)
	}
}

// (c) counter doubling stacks multiplicatively via AddCountersToPermanent too
// (two Doubling Seasons → 4x +1/+1).
func TestDoubler_TwoDoublingSeasons_CountersQuadruple(t *testing.T) {
	gs := newFixtureGame(t)
	for i := 0; i < 2; i++ {
		ds := addBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
		RegisterDoublingSeason(gs, ds)
	}
	bear := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	placed, _ := AddCountersToPermanent(gs, bear, "+1/+1", 1, "src", false)
	if placed != 4 {
		t.Fatalf("two Doubling Seasons must put 4 +1/+1 counters (1*2*2), got %d", placed)
	}
}
