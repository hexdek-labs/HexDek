package gameengine

import (
	"math/rand"
	"testing"
)

// modal_handler_r63_test.go — regressions for the generic modal-spell
// handler (modal_handler_r63.go). Tests ResolveModalText directly (the
// generic path; per-card OnResolve handlers like Boros Charm's take
// precedence in live resolution, so the generic handler is exercised in
// isolation here) plus one integration test proving the
// resolveResidualByText hook routes modal text to it.

func modalGame(seats int) (*GameState, *Permanent) {
	gs := NewGameState(seats, rand.New(rand.NewSource(11)), nil)
	src := &Permanent{Card: &Card{Name: "Modal Spell", Owner: 0}, Controller: 0, Owner: 0, Flags: map[string]int{}}
	return gs, src
}

// Boros Charm body (choose one): the damage mode is mode 1, so the
// "first executable" policy picks it → 4 to the lowest-life opponent.
func TestModal_BorosCharm_ChooseOneDamage(t *testing.T) {
	gs, src := modalGame(3)
	gs.Seats[1].Life = 30
	gs.Seats[2].Life = 12 // lowest
	raw := "choose one — • ~ deals 4 damage to target player or planeswalker. ; • permanents you control gain indestructible until end of turn. ; • target creature gains double strike until end of turn"

	if !ResolveModalText(gs, src, raw) {
		t.Fatal("modal text should be recognized")
	}
	if gs.Seats[2].Life != 8 {
		t.Errorf("lowest-life opponent should take 4 (→8), got %d", gs.Seats[2].Life)
	}
	if gs.Seats[1].Life != 30 {
		t.Errorf("higher-life opponent untouched, got %d", gs.Seats[1].Life)
	}
}

// Choose-one where the FIRST mode is unexecutable (no legal target) falls
// through to the next executable mode — the spell still does something.
func TestModal_ChooseOne_SkipsUnexecutableFirstMode(t *testing.T) {
	gs, src := modalGame(2)
	// seat 0 has 30, opp seat 1 has 20.
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 20
	// Mode 1 destroys an artifact (none on board → unexecutable). Mode 2
	// draws. Mode 3 deals damage. choose-one → first executable = draw.
	gs.Seats[0].Library = []*Card{{Name: "Card", Owner: 0}}
	preHand := len(gs.Seats[0].Hand)
	raw := "choose one — • destroy target artifact ; • you draw a card ; • ~ deals 2 damage to target player"

	ResolveModalText(gs, src, raw)
	// choose-one executes exactly ONE executable mode: the draw (mode 2).
	if got := len(gs.Seats[0].Hand) - preHand; got != 1 {
		t.Errorf("expected draw mode to execute (hand +1), got +%d", got)
	}
	// Damage mode must NOT have also fired (choose-one = at most one).
	if gs.Seats[1].Life != 20 {
		t.Errorf("choose-one must not also fire the damage mode, opp life=%d", gs.Seats[1].Life)
	}
}

// A Command (choose two, distinct): exactly two executable modes run.
func TestModal_Command_ChooseTwo(t *testing.T) {
	gs, src := modalGame(2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 25
	gs.Seats[0].Library = []*Card{{Name: "A", Owner: 0}, {Name: "B", Owner: 0}}
	preHand := len(gs.Seats[0].Hand)
	// choose two — draw a card ; gain 3 life ; deal 3 to a player.
	raw := "choose two — • you draw a card ; • you gain 3 life ; • ~ deals 3 damage to target player"

	ResolveModalText(gs, src, raw)
	// First two executable modes: draw (+1 card) and gain 3 life (20→23).
	if got := len(gs.Seats[0].Hand) - preHand; got != 1 {
		t.Errorf("choose-two: draw should fire once (+1), got +%d", got)
	}
	if gs.Seats[0].Life != 23 {
		t.Errorf("choose-two: gain-3-life should fire (20→23), got %d", gs.Seats[0].Life)
	}
	// Third mode (damage) must NOT fire — only two modes chosen.
	if gs.Seats[1].Life != 25 {
		t.Errorf("choose-two must stop at 2 modes; damage mode wrongly fired, opp life=%d", gs.Seats[1].Life)
	}
}

// Entwine: ALL modes execute (greedy entwine policy).
func TestModal_Entwine_AllModes(t *testing.T) {
	gs, src := modalGame(2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 25
	gs.Seats[0].Library = []*Card{{Name: "A", Owner: 0}}
	preHand := len(gs.Seats[0].Hand)
	raw := "choose one — • you draw a card ; • you gain 2 life ; • ~ deals 5 damage to target player. entwine {2}"

	ResolveModalText(gs, src, raw)
	if got := len(gs.Seats[0].Hand) - preHand; got != 1 {
		t.Errorf("entwine: draw mode should fire, got +%d", got)
	}
	if gs.Seats[0].Life != 22 {
		t.Errorf("entwine: gain-2-life should fire (20→22), got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("entwine: 5-damage mode should fire (25→20), got %d", gs.Seats[1].Life)
	}
}

// Integration: modal text routed through resolveResidualByText (the
// parsed_tail dispatch path) reaches the generic handler.
func TestModal_IntegrationViaResidualDispatch(t *testing.T) {
	gs, src := modalGame(2)
	gs.Seats[1].Life = 18
	raw := "choose one — • ~ deals 3 damage to target player ; • you gain 5 life"
	handled := resolveResidualByText(gs, src, raw)
	if !handled {
		t.Fatal("resolveResidualByText should report the modal text as handled")
	}
	if gs.Seats[1].Life != 15 {
		t.Errorf("integration: damage mode should hit opponent (18→15), got %d", gs.Seats[1].Life)
	}
}

// A non-modal residual must NOT be hijacked by the modal handler.
func TestModal_NonModalUnaffected(t *testing.T) {
	gs, src := modalGame(2)
	if ResolveModalText(gs, src, "draw a card") {
		t.Error("non-modal text ('draw a card') must not be treated as modal")
	}
}
