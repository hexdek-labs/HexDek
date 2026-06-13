package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// tcover_shard_r60_test.go — regression pins for the shard-T per-card
// coverage batch (Time Stretch, Traumatize, Tasha's Hideous Laughter,
// Terminus, Torgaar). Each card previously parsed to an inert
// raw-text fallback node and produced NO observable effect; these tests
// assert the new per_card handlers now produce the printed effect.

// -----------------------------------------------------------------------------
// Time Stretch — two extra turns
// -----------------------------------------------------------------------------

func TestTimeStretch_GrantsTwoExtraTurns(t *testing.T) {
	gs := newGame(t, 2)
	pre := gs.Flags["extra_turns_pending"]

	card := addCard(gs, 0, "Time Stretch", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := gs.Flags["extra_turns_pending"]; got != pre+2 {
		t.Errorf("expected extra_turns_pending to bump by 2; pre=%d post=%d", pre, got)
	}
	if hasEvent(gs, "extra_turn") != 2 {
		t.Errorf("expected exactly 2 extra_turn events, got %d", hasEvent(gs, "extra_turn"))
	}
}

// -----------------------------------------------------------------------------
// Traumatize — mill half (rounded down)
// -----------------------------------------------------------------------------

func TestTraumatize_MillsHalfOpponentLibrary(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 1, "a", "b", "c", "d", "e", "f", "g") // 7 cards -> mill 3
	preGY := len(gs.Seats[1].Graveyard)

	card := addCard(gs, 0, "Traumatize", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := len(gs.Seats[1].Library); got != 4 {
		t.Errorf("expected 4 cards left in library (7 - floor(7/2)=3), got %d", got)
	}
	if got := len(gs.Seats[1].Graveyard) - preGY; got != 3 {
		t.Errorf("expected 3 cards milled to graveyard, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// Tasha's Hideous Laughter — exile from top until total MV >= 20
// -----------------------------------------------------------------------------

func TestTashasHideousLaughter_ExilesUntilMV20(t *testing.T) {
	gs := newGame(t, 2)
	// 6 cards of CMC 5 each. Running total: 5,10,15,20 -> stops after 4.
	for i := 0; i < 6; i++ {
		gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{
			Name: "Five Drop", Owner: 1, CMC: 5,
		})
	}
	preExile := len(gs.Seats[1].Exile)

	card := addCard(gs, 0, "Tasha's Hideous Laughter", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := len(gs.Seats[1].Exile) - preExile; got != 4 {
		t.Errorf("expected 4 cards exiled (5+5+5+5=20), got %d", got)
	}
	if got := len(gs.Seats[1].Library); got != 2 {
		t.Errorf("expected 2 cards left in library, got %d", got)
	}
}

func TestTashasHideousLaughter_EmptyLibraryNoOverflow(t *testing.T) {
	gs := newGame(t, 2)
	// Only 2 cards of CMC 1 (total 2 < 20) — must stop at library end.
	for i := 0; i < 2; i++ {
		gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{
			Name: "One Drop", Owner: 1, CMC: 1,
		})
	}
	card := addCard(gs, 0, "Tasha's Hideous Laughter", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := len(gs.Seats[1].Library); got != 0 {
		t.Errorf("expected empty library after exhausting it, got %d", got)
	}
	if got := len(gs.Seats[1].Exile); got != 2 {
		t.Errorf("expected 2 cards exiled, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// Terminus — all creatures to bottom of owners' libraries
// -----------------------------------------------------------------------------

func TestTerminus_TucksAllCreaturesToLibraryBottom(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Grizzly Bears", "creature")
	addPerm(gs, 0, "Llanowar Elves", "creature")
	addPerm(gs, 1, "Savannah Lions", "creature")
	// A non-creature should survive.
	addPerm(gs, 0, "Sol Ring", "artifact")

	card := addCard(gs, 0, "Terminus", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// All creatures gone from both battlefields.
	for seat := 0; seat < 2; seat++ {
		for _, p := range gs.Seats[seat].Battlefield {
			if p != nil && p.IsCreature() {
				t.Errorf("seat %d still has creature %s on battlefield", seat, p.Card.Name)
			}
		}
	}
	// Non-creature survives.
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Errorf("expected Sol Ring to survive (1 perm), got %d", len(gs.Seats[0].Battlefield))
	}
	// Creatures went to their owners' libraries.
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 creatures tucked into seat 0 library, got %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[1].Library) != 1 {
		t.Errorf("expected 1 creature tucked into seat 1 library, got %d", len(gs.Seats[1].Library))
	}
}

// -----------------------------------------------------------------------------
// Torgaar, Famine Incarnate — target player's life becomes half starting
// -----------------------------------------------------------------------------

func TestTorgaar_HalvesTargetStartingLife(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].StartingLife, gs.Seats[0].Life = 40, 40
	gs.Seats[1].StartingLife, gs.Seats[1].Life = 40, 38

	perm := addPerm(gs, 0, "Torgaar, Famine Incarnate", "legendary", "creature")
	gameengine.InvokeETBHook(gs, perm)

	if got := gs.Seats[1].Life; got != 20 {
		t.Errorf("expected opponent life set to 20 (half of 40 starting), got %d", got)
	}
	if got := gs.Seats[0].Life; got != 40 {
		t.Errorf("expected caster life unchanged at 40, got %d", got)
	}
}

func TestTorgaar_DeclinesWhenNoReduction(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].StartingLife, gs.Seats[0].Life = 40, 40
	gs.Seats[1].StartingLife, gs.Seats[1].Life = 40, 12 // already below half

	perm := addPerm(gs, 0, "Torgaar, Famine Incarnate", "legendary", "creature")
	gameengine.InvokeETBHook(gs, perm)

	if got := gs.Seats[1].Life; got != 12 {
		t.Errorf("expected opponent life unchanged at 12 (no beneficial target), got %d", got)
	}
}
