package gameengine

import (
	"testing"
)

// TestAttachmentConsistency_RemovePermanentFromBattlefield pins the r60
// AttachmentConsistency fix. Engine paths that route through
// removePermanentFromBattlefield (Craft, Meld, exile-self activation cost,
// Aura swap) move a permanent off the battlefield but, prior to the fix,
// did NOT detach auras/equipment pointing at it — leaving stale AttachedTo
// references for the window before SBA §704.5m/§704.5n cleaned them up.
// The invariant fires in that window. Adding detachAll inside
// removePermanentFromBattlefield closes the gap.
func TestAttachmentConsistency_RemovePermanentFromBattlefield(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	carrier := addBattlefield(gs, 0, "Squire", 1, 2, "creature")
	aura := addBattlefield(gs, 0, "Ghoulish Impetus", 0, 0, "enchantment", "aura")
	aura.AttachedTo = carrier
	gear := addBattlefield(gs, 0, "Hero's Blade", 0, 0, "artifact", "equipment")
	gear.AttachedTo = carrier

	// Exile-and-return path (Craft / activation-cost ExileSelf shape).
	removePermanentFromBattlefield(gs, carrier)

	if aura.AttachedTo != nil {
		t.Fatalf("aura should be detached after carrier left battlefield, got %v", aura.AttachedTo)
	}
	if gear.AttachedTo != nil {
		t.Fatalf("equipment should be detached after carrier left battlefield, got %v", gear.AttachedTo)
	}
	if err := checkAttachmentConsistency(gs); err != nil {
		t.Fatalf("AttachmentConsistency violation after exile-and-return removal: %v", err)
	}
}

// TestAttachmentConsistency_SweepReturnDetachesAura covers the sweep alt-cost
// path (gs.removePermanent in keywords_sweep.go) where a permanent is returned
// to hand without going through the normal Bounce path. Auras attached to it
// must be detached at the same moment the carrier leaves the battlefield.
func TestAttachmentConsistency_SweepReturnDetachesAura(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	carrier := addBattlefield(gs, 0, "Forest", 0, 0, "land")
	aura := addBattlefield(gs, 0, "Wild Growth", 0, 0, "enchantment", "aura")
	aura.AttachedTo = carrier

	// Simulate the sweep return path: remove + route the Card to hand.
	gs.removePermanent(carrier)
	DetachAll(gs, carrier)
	if carrier.Card != nil {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, carrier.Card)
	}

	if aura.AttachedTo != nil {
		t.Fatalf("aura should be detached after carrier returned to hand, got %v", aura.AttachedTo)
	}
	if err := checkAttachmentConsistency(gs); err != nil {
		t.Fatalf("AttachmentConsistency violation after sweep return: %v", err)
	}
}

// TestAttachmentConsistency_GainControlKeepsAttachment confirms the fix does
// NOT over-correct: gaining control of a permanent is not a zone change, the
// carrier remains on the battlefield (under a different controller), and the
// aura's AttachedTo should remain pointed at it. CR §702 / §702.x do not
// detach auras for control changes alone.
func TestAttachmentConsistency_GainControlKeepsAttachment(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	carrier := addBattlefield(gs, 1, "Stolen Bear", 2, 2, "creature")
	aura := addBattlefield(gs, 1, "Control Magic", 0, 0, "enchantment", "aura")
	aura.AttachedTo = carrier

	// Gain-control move: remove from old seat, re-add to new seat. Carrier
	// stays on the battlefield, just under a new controller.
	gs.removePermanent(carrier)
	carrier.Controller = 0
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, carrier)

	if aura.AttachedTo != carrier {
		t.Fatalf("aura should remain attached across control change, got %v", aura.AttachedTo)
	}
	if err := checkAttachmentConsistency(gs); err != nil {
		t.Fatalf("AttachmentConsistency violation after gain-control: %v", err)
	}
}

// TestAttachmentConsistency_DetachAllPublic confirms the engine exposes a
// public DetachAll for per_card handlers (flicker/blink/exile-self families
// in displacer_kitten, emiel, deadeye_navigator, brago, thassa, yorion, ...)
// that route through the per_card-local removePermanent helper.
func TestAttachmentConsistency_DetachAllPublic(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	carrier := addBattlefield(gs, 0, "Squire", 1, 1, "creature")
	aura := addBattlefield(gs, 0, "Brilliant Wings", 0, 0, "enchantment", "aura")
	aura.AttachedTo = carrier

	// Simulate per_card flicker: remove + DetachAll + (new perm would be
	// minted elsewhere — irrelevant to this invariant).
	gs.removePermanent(carrier)
	DetachAll(gs, carrier)

	if aura.AttachedTo != nil {
		t.Fatalf("DetachAll(gs, p) should nil AttachedTo on other perms pointing at p, got %v", aura.AttachedTo)
	}
	if err := checkAttachmentConsistency(gs); err != nil {
		t.Fatalf("AttachmentConsistency violation after public DetachAll: %v", err)
	}
}
