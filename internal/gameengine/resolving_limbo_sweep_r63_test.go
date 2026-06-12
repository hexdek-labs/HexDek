package gameengine

import (
	"math/rand"
	"testing"
)

// resolving_limbo_sweep_r63_test.go — seed-7777 game-76 Biorhythm
// fabrication.
//
// Root cause: ResolveStackTop pops a spell off the stack BEFORE running
// its resolution effects; until the final zone routing the *Card is
// absent from every census-walked zone. Biorhythm's resolution (mass
// life-set) eliminated two seats mid-resolution — CheckEnd →
// HandleSeatElimination → SweepOrphanedInstanceIDs — and the sweep
// ceased the in-flight card's ID as a "minted-but-absent orphan". The
// card then routed to its owner's graveyard, leaving a ceased ID
// present in a live zone: a ZoneConservation fabrication on every
// census tick to game end.
//
// Fix pinned here: gs.ResolvingCards tracks the in-flight card; both
// the orphan sweep's present-walk and the census count it as
// zone-presence.

// TestOrphanSweep_SparesMidResolutionCard pins the sweep surface: a card
// in the resolution limbo window must survive a mid-resolution seat
// elimination's orphan sweep. FAILS pre-fix.
func TestOrphanSweep_SparesMidResolutionCard(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(76)), nil)

	// Seat 1's sorcery, mid-resolution: popped off the stack, in no zone.
	resolving := &Card{
		Name:  "Biorhythm",
		Owner: 1,
		Types: []string{"sorcery"},
	}
	MintOGInstanceID(gs, resolving)
	gs.ResolvingCards = append(gs.ResolvingCards, resolving)

	// The resolution eliminates seat 3 (the game-76 shape: mass life-set
	// kills a player; CheckEnd → HandleSeatElimination runs while the
	// spell is still in flight).
	gs.Seats[3].Lost = true
	gs.Seats[3].LossReason = "life total 0 or less (CR 704.5a)"
	HandleSeatElimination(gs, 3)

	if _, ceased := gs.CeasedInstanceIDs[resolving.InstanceID]; ceased {
		t.Fatalf("mid-resolution card %q was orphan-swept during seat elimination — limbo window not counted as present", resolving.InstanceID)
	}

	// Resolution completes: card routes to its owner's graveyard, the
	// limbo tracker pops. The census must stay clean.
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, resolving)
	gs.ResolvingCards = gs.ResolvingCards[:len(gs.ResolvingCards)-1]
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation after resolution completed: %v", err)
	}
}

// TestOrphanSweep_StandaloneSweepSparesResolvingCard pins the sweep
// helper directly (the elimination path is one of three sweep callers;
// game-end CheckEnd and the cleanup step share the walk).
func TestOrphanSweep_StandaloneSweepSparesResolvingCard(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(77)), nil)

	resolving := &Card{Name: "Exsanguinate", Owner: 2, Types: []string{"sorcery"}}
	MintOGInstanceID(gs, resolving)
	gs.ResolvingCards = append(gs.ResolvingCards, resolving)

	if swept := SweepOrphanedInstanceIDs(gs); swept != 0 {
		t.Fatalf("sweep ceased %d ID(s); the resolving card must count as present", swept)
	}
	if _, ceased := gs.CeasedInstanceIDs[resolving.InstanceID]; ceased {
		t.Fatalf("resolving card's ID was ceased by a standalone sweep")
	}
}

// TestSeatElimination_CeasesOwnInFlightCard pins the inverse case
// (seed-7777 game 1937): the resolving spell's OWNER is the seat being
// eliminated — Biorhythm kills its own caster. The card is in no zone
// (limbo), so the elimination's zone-walk ceases miss it, and the
// post-resolution graveyard routing is refused by the MoveCard LeftGame
// guard (PR #1041) — without an explicit ResolvingCards cease the ID
// reads as "disappeared" forever. FAILS pre-fix.
func TestSeatElimination_CeasesOwnInFlightCard(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(1937)), nil)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["instanceid_strict_census"] = 1 // arm the disappearance check

	resolving := &Card{
		Name:  "Biorhythm",
		Owner: 2,
		Types: []string{"sorcery"},
	}
	MintOGInstanceID(gs, resolving)
	gs.ResolvingCards = append(gs.ResolvingCards, resolving)

	// The resolution kills its own caster: seat 2 eliminated while the
	// spell is still in flight.
	gs.Seats[2].Lost = true
	gs.Seats[2].LossReason = "life total 0 or less (CR 704.5a)"
	HandleSeatElimination(gs, 2)

	if _, ceased := gs.CeasedInstanceIDs[resolving.InstanceID]; !ceased {
		t.Fatalf("eliminated owner's in-flight card %q was not ceased — §800.4a gap", resolving.InstanceID)
	}

	// Resolution completes: the LeftGame guard refuses the graveyard
	// routing (the card left the game with its owner), the limbo tracker
	// pops, and the census must be clean — ceased + absent is the
	// expected state.
	res := MoveCard(gs, resolving, 2, "stack", "graveyard", "resolved")
	if res.FinalZone != "" {
		t.Fatalf("dead owner's in-flight card was routed to %q — LeftGame guard bypassed", res.FinalZone)
	}
	gs.ResolvingCards = gs.ResolvingCards[:len(gs.ResolvingCards)-1]
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation after in-flight owner elimination: %v", err)
	}
}

// TestResolveStackTop_BalancesResolvingCards pins the ResolveStackTop
// push/pop: after any resolution — including one that ends the game —
// the limbo tracker must be empty again.
func TestResolveStackTop_BalancesResolvingCards(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(78)), nil)

	spell := &Card{Name: "Shock", Owner: 0, Types: []string{"instant"}}
	MintOGInstanceID(gs, spell)
	gs.Stack = append(gs.Stack, &StackItem{Card: spell, Controller: 0})

	ResolveStackTop(gs)

	if len(gs.ResolvingCards) != 0 {
		t.Fatalf("ResolvingCards not balanced after resolution: %d entries", len(gs.ResolvingCards))
	}
	if _, ceased := gs.CeasedInstanceIDs[spell.InstanceID]; ceased {
		t.Fatalf("resolved spell's ID was ceased")
	}
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation after plain resolution: %v", err)
	}
}
