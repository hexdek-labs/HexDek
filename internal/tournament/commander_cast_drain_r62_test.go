package tournament

// Regression tests for the commander-cast stack squatter (r62 — the
// legality validator's highest-volume finding: 302 of 316 chaos-run
// violations, rule 117.1a, "sorcery-speed spell cast with 1 item on the
// stack", clustered on commander-cast turns).
//
// Root cause: CastCommanderFromCommandZone PUSHES the commander spell and
// returns — its documented contract is "the caller drives the stack
// resolution via PriorityRound + ResolveStackTop" (commander.go). The
// turn runner's tryCastCommander was the only production caller and only
// ran StateBasedActions after the cast: the commander spell sat
// unresolved on the stack while the main-phase loop kept casting. Every
// later cast that turn announced with an item already on the stack
// (CR §117.1a / §307.1 sequencing violations), later spells LIFO-resolved
// BEFORE the commander, and the commander only resolved as a bystander of
// the next cast's internal drain or the phase-boundary drain.
//
// The fix mirrors CastSpell's own tail inside tryCastCommander:
// PriorityRound + DrainStack after a successful commander cast.
//
// Empirical validation (loki -legality, seed 42): 117.1a count 98 → 0
// across 85 chaos games; the pre-fix resident items were exactly the
// commander spells.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// vanillaCommander builds a plain mono-color legendary creature commander.
func vanillaCommander() *gameengine.Card {
	return &gameengine.Card{
		Name:          "Test Warchief",
		Types:         []string{"legendary", "creature"},
		TypeLine:      "legendary creature — orc warrior",
		CMC:           3,
		Colors:        []string{"R"},
		BasePower:     3,
		BaseToughness: 3,
	}
}

// TestTryCastCommander_DrainsStack is the headline pin: after
// tryCastCommander returns, the commander spell must be RESOLVED — on
// the battlefield, not squatting on the stack. Pre-fix this fails with
// the spell still on the stack and the battlefield empty.
func TestTryCastCommander_DrainsStack(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(3)), nil)
	cmdr := vanillaCommander()
	cmdr.Owner = 0
	deck := &gameengine.CommanderDeck{CommanderCards: []*gameengine.Card{cmdr}}
	gameengine.SetupCommanderGame(gs, []*gameengine.CommanderDeck{deck, {}})
	for i := range gs.Seats {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}
	gs.Active = 0
	gs.Phase = "main"
	gs.Seats[0].ManaPool = 3
	gameengine.EnsureTypedPool(gs.Seats[0])

	tryCastCommander(gs, 0)

	if len(gs.Stack) != 0 {
		t.Fatalf("commander spell left squatting on the stack (%d item(s)) — tryCastCommander must drain after casting", len(gs.Stack))
	}
	onBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == cmdr {
			onBF = true
			break
		}
	}
	if !onBF {
		t.Fatal("commander not on battlefield after tryCastCommander — cast didn't resolve")
	}
	if len(gs.Seats[0].CommandZone) != 0 {
		t.Errorf("commander still in command zone (%d cards)", len(gs.Seats[0].CommandZone))
	}
}

// TestTryCastCommander_NoMidStackCastAfterwards pins the legality-facing
// consequence end-to-end: with the ride-along validator attached, a
// commander cast followed by a normal sorcery-speed cast in the same
// main phase must produce ZERO 117.1a violations. Pre-fix the second
// cast announces with the unresolved commander on the stack and flags.
func TestTryCastCommander_NoMidStackCastAfterwards(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(3)), nil)
	gs.Seed = 3
	gs.Legality = gameengine.NewLegalityValidator(3)
	cmdr := vanillaCommander()
	cmdr.Owner = 0
	deck := &gameengine.CommanderDeck{CommanderCards: []*gameengine.Card{cmdr}}
	gameengine.SetupCommanderGame(gs, []*gameengine.CommanderDeck{deck, {}})
	for i := range gs.Seats {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}
	gs.Active = 0
	gs.Phase = "main"
	gs.Seats[0].ManaPool = 6
	gameengine.EnsureTypedPool(gs.Seats[0])

	tryCastCommander(gs, 0)

	// The follow-up cast the main-phase loop would make.
	bear := &gameengine.Card{
		Name: "Followup Bear", Owner: 0,
		Types:         []string{"creature", "cost:3"},
		BasePower:     2,
		BaseToughness: 2,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, bear)
	if err := gameengine.CastSpell(gs, 0, bear, nil); err != nil {
		t.Fatalf("follow-up cast failed: %v", err)
	}

	for _, v := range gs.Legality.Violations {
		if v.Rule == "117.1a" || v.Rule == "307.1" {
			t.Errorf("mid-stack cast violation after commander cast (the r62 302-hit cluster): %v", v)
		}
	}
}
