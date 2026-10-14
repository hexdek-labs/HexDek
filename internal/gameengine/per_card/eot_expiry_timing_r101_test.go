package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// r101 — "until end of turn" expiry ran in the §513 END step instead of
// the §514 CLEANUP step.
//
// CR §514.2 ends these effects as the cleanup step begins. The end step
// comes first, and the §513.1 priority window sits between the two: for
// the whole of that window the keyword was already gone and a stolen
// creature was already handed back.
//
// Each test drives the real handler entry point, then separates the two
// steps explicitly: FireDelayedTriggers(gs, "ending", "end") is §513,
// ScanExpiredDurations(gs, "ending", "cleanup") is §514.
// ---------------------------------------------------------------------------

// Keyword arm. Skithiryx, the Blight Dragon — "{B}: Skithiryx gains
// haste until end of turn."
func TestEOTTiming_r101_KeywordSurvivesEndStepAndDiesAtCleanup(t *testing.T) {
	gs := newGame(t, 2)
	skit := addPerm(gs, 0, "Skithiryx, the Blight Dragon", "creature", "legendary", "dragon")
	skit.Card.BasePower, skit.Card.BaseToughness = 4, 4
	gs.Seats[0].ManaPool = 5

	skithiryxGrantHaste(gs, skit)
	if !gs.HasKeywordOf(skit, "haste") {
		t.Fatal("precondition: {B} should grant haste")
	}

	// §513 end step — the grant must OUTLIVE it.
	gameengine.FireDelayedTriggers(gs, "ending", "end")
	if !gs.HasKeywordOf(skit, "haste") {
		t.Error("CR 514.2: haste must still be present through the §513.1 priority window")
	}

	// §514 cleanup — and die here.
	gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
	if gs.HasKeywordOf(skit, "haste") {
		t.Error("CR 514.2: haste must be gone once the cleanup step begins")
	}
}

// Control arm. Insurrection — "Untap all creatures and gain control of
// them until end of turn." A control change is an until-EOT effect too,
// so it ends at the same moment, not a step earlier.
func TestEOTTiming_r101_StolenControlRevertsAtCleanupNotEndStep(t *testing.T) {
	gs := newGame(t, 2)
	victim := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")
	victim.Card.BasePower, victim.Card.BaseToughness = 2, 2

	card := &gameengine.Card{Name: "Insurrection"}
	insurrectionResolve(gs, &gameengine.StackItem{Card: card, Controller: 0})

	if victim.Controller != 0 {
		t.Fatalf("precondition: Insurrection should steal the creature; controller = %d", victim.Controller)
	}

	gameengine.FireDelayedTriggers(gs, "ending", "end")
	if victim.Controller != 0 {
		t.Error("CR 514.2: control must not revert until the cleanup step begins")
	}
	if !gs.HasKeywordOf(victim, "haste") {
		t.Error("CR 514.2: the haste grant must also outlive the end step")
	}

	gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
	if victim.Controller != 1 {
		t.Errorf("control must revert at cleanup; controller = %d", victim.Controller)
	}
}

// CR §724.1d. "End the turn" (Sundial of the Infinite) skips straight
// to the cleanup step, so the §513 end step never happens — and
// Sundial's own handler purges every end-step delayed trigger on the
// way. Before r101 that meant the undo never fired at all and the
// grant was kept for the rest of the game.
func TestEOTTiming_r101_FastForwardCleanupStillExpires(t *testing.T) {
	gs := newGame(t, 2)
	skit := addPerm(gs, 0, "Skithiryx, the Blight Dragon", "creature", "legendary", "dragon")
	skit.Card.BasePower, skit.Card.BaseToughness = 4, 4
	gs.Seats[0].ManaPool = 5

	skithiryxGrantHaste(gs, skit)
	if !gs.HasKeywordOf(skit, "haste") {
		t.Fatal("precondition: {B} should grant haste")
	}

	// Drive the real card. Sundial's handler purges every end-step
	// delayed trigger, so this also pins that it must NOT purge the
	// cleanup-step expiry along with them.
	gs.Active = 0
	sundial := addPerm(gs, 0, "Sundial of the Infinite", "artifact")
	sundialActivate(gs, sundial, 0, map[string]interface{}{})
	if gs.Flags["turn_ending_now"] == 0 {
		t.Fatal("precondition: Sundial should have signalled the turn ending")
	}

	// No end step at all — straight to cleanup, as fastForwardCleanup does.
	gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
	if gs.HasKeywordOf(skit, "haste") {
		t.Error("CR 724.1d: the fast-forward cleanup must still expire the grant")
	}
}

// Negative guard. Dihada's +2 is "until your NEXT TURN", not until end
// of turn — §514.2 does not govern it, and neither the end step nor the
// cleanup step may take it away. This is the assertion that stops a
// future sweep migrating every delayed trigger it finds.
func TestEOTTiming_r101_UntilYourNextTurnIsNotTouchedByCleanup(t *testing.T) {
	gs := newGame(t, 2)
	dihada := addPerm(gs, 0, "Dihada, Binder of Wills", "planeswalker", "legendary")
	target := addPerm(gs, 0, "Sword Dancer", "creature", "legendary", "human")
	target.Card.BasePower, target.Card.BaseToughness = 2, 2

	dihadaPlusTwo(gs, dihada)
	if !gs.HasKeywordOf(target, "indestructible") {
		t.Fatal("precondition: +2 should grant indestructible")
	}

	gameengine.FireDelayedTriggers(gs, "ending", "end")
	gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
	if !gs.HasKeywordOf(target, "indestructible") {
		t.Error("an 'until your next turn' grant must survive this turn's cleanup")
	}
}
