package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Regression coverage for the A–C per-card inert-ability sweep
// (dev/percard-cover-shardAC). Each card below previously parsed its
// principal ability to an inert `custom` slug and produced NO observable
// engine effect; these tests pin the implemented behavior.

func countByName(gs *gameengine.GameState, seat int, name string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			n++
		}
	}
	return n
}

func TestArmyOfTheDamned_CreatesThirteenTappedZombies(t *testing.T) {
	if !HasResolve("Army of the Damned") {
		t.Fatal("Army of the Damned not registered")
	}
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Army of the Damned", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	got := countByName(gs, 0, "Zombie Token")
	if got != 13 {
		t.Fatalf("expected 13 Zombie tokens, got %d", got)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Zombie Token" {
			if !p.Tapped {
				t.Error("Zombie token should enter tapped")
			}
			if p.Power() != 2 || p.Toughness() != 2 {
				t.Errorf("Zombie token should be 2/2, got %d/%d", p.Power(), p.Toughness())
			}
		}
	}
}

func TestAcornHarvest_CreatesTwoSquirrels(t *testing.T) {
	if !HasResolve("Acorn Harvest") {
		t.Fatal("Acorn Harvest not registered")
	}
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Acorn Harvest", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := countByName(gs, 0, "Squirrel Token"); got != 2 {
		t.Fatalf("expected 2 Squirrel tokens, got %d", got)
	}
}

func TestBattleScreech_CreatesTwoFlyingBirds(t *testing.T) {
	if !HasResolve("Battle Screech") {
		t.Fatal("Battle Screech not registered")
	}
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Battle Screech", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	birds := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Bird Token" {
			birds++
			if !gameengine.CardHasKeyword(p.Card, "flying") {
				t.Error("Bird token should have flying")
			}
		}
	}
	if birds != 2 {
		t.Fatalf("expected 2 Bird tokens, got %d", birds)
	}
}

func TestChaosWarp_ShufflesTargetAndRevealsPermanent(t *testing.T) {
	if !HasResolve("Chaos Warp") {
		t.Fatal("Chaos Warp not registered")
	}
	gs := newGame(t, 2)
	// Target an opponent's (seat 1) creature.
	victim := addPerm(gs, 1, "Big Threat", "creature")
	// Stack seat 1's library with permanent cards so the revealed top is
	// always a permanent (and gets put onto the battlefield).
	for i := 0; i < 5; i++ {
		gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{
			Name:  "Forest",
			Owner: 1,
			Types: []string{"land"},
		})
	}
	card := addCard(gs, 0, "Chaos Warp", "instant")
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		Targets:    []gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: victim}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Target permanent should be gone from seat 1's battlefield.
	if countByName(gs, 1, "Big Threat") != 0 {
		t.Error("Chaos Warp should remove the targeted permanent from the battlefield")
	}
	// The shuffled card should now be in seat 1's library somewhere
	// (5 lands - 1 revealed/put + 1 shuffled-in = 5).
	if len(gs.Seats[1].Library) != 5 {
		t.Errorf("expected library size 5 after shuffle+reveal, got %d", len(gs.Seats[1].Library))
	}
	// A permanent (the revealed Forest) should have entered seat 1's bf.
	if countByName(gs, 1, "Forest") != 1 {
		t.Errorf("expected the revealed permanent to enter the battlefield, got %d Forests", countByName(gs, 1, "Forest"))
	}
	if hasEvent(gs, "chaos_warp") == 0 {
		t.Error("expected a chaos_warp event")
	}
}

func TestArterialAlchemy_CreatesBloodPerOpponent(t *testing.T) {
	if !HasETB("Arterial Alchemy") {
		t.Skip("Arterial Alchemy OnETB not registered")
	}
	gs := newGame(t, 4) // controller + 3 opponents
	perm := addPerm(gs, 0, "Arterial Alchemy", "enchantment")
	arterialAlchemyETB(gs, perm)

	if got := countByName(gs, 0, "Blood Token"); got != 3 {
		t.Fatalf("expected 3 Blood tokens (one per opponent), got %d", got)
	}
}

func TestArterialAlchemy_SkipsDeadOpponents(t *testing.T) {
	gs := newGame(t, 4)
	gs.Seats[2].Lost = true
	perm := addPerm(gs, 0, "Arterial Alchemy", "enchantment")
	arterialAlchemyETB(gs, perm)

	if got := countByName(gs, 0, "Blood Token"); got != 2 {
		t.Fatalf("expected 2 Blood tokens (dead opponent skipped), got %d", got)
	}
}

func TestBeaconOfImmortality_DoublesTargetLife(t *testing.T) {
	if !HasResolve("Beacon of Immortality") {
		t.Fatal("Beacon of Immortality not registered")
	}
	gs := newGame(t, 2)
	gs.Seats[0].Life = 17
	card := addCard(gs, 0, "Beacon of Immortality", "instant")
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		Targets:    []gameengine.Target{{Kind: gameengine.TargetKindSeat, Seat: 0}},
	}
	gameengine.InvokeResolveHook(gs, item)

	if gs.Seats[0].Life != 34 {
		t.Fatalf("expected life doubled to 34, got %d", gs.Seats[0].Life)
	}
}

func TestChaosWarp_TokenTargetDoesNotEnterLibrary(t *testing.T) {
	gs := newGame(t, 2)
	tok := addPerm(gs, 1, "Goblin Token", "token", "creature")
	gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{
		Name: "Island", Owner: 1, Types: []string{"land"},
	})
	card := addCard(gs, 0, "Chaos Warp", "instant")
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		Targets:    []gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: tok}},
	}
	gameengine.InvokeResolveHook(gs, item)

	for _, c := range gs.Seats[1].Library {
		if c != nil && c.Name == "Goblin Token" {
			t.Error("a token target must not be shuffled into the library")
		}
	}
}
