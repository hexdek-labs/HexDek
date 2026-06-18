package gameengine

import (
	"testing"
)

// reconfigure_type_toggle_r63_test.go — r63 mechanic-probe (CR §702.151).
// While a reconfigure permanent is ATTACHED it stops being a creature
// (§702.151e) — it's just Equipment; UNATTACHED it is a creature again.
// The prior code set a `reconfigured` flag but nothing consulted it, so an
// attached reconfigure equipment still counted as a creature (could attack/
// block / be targeted). Fix wires the flag into IsCreature() (mirroring the
// bestow/impending toggles) and guards the attach legal-target.

// (UNWIRED fix) the attached↔creature type toggle.
func TestReconfigure_TypeToggleAttachedNotCreature(t *testing.T) {
	gs := newMiscGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10

	perm := addMiscBattlefield(gs, 0, "Lizard Blades", 1, 1, "artifact", "creature", "equipment")
	target := addMiscBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	// Clean attack eligibility so the "can't attack while attached" check
	// isolates the type toggle (not summoning sickness / tapped).
	perm.SummoningSick = false
	perm.Tapped = false

	// (1) Enters as an artifact CREATURE, not attached.
	if !perm.IsCreature() {
		t.Fatal("(1) reconfigure permanent must be a creature while unattached")
	}
	if perm.AttachedTo != nil {
		t.Fatal("(1) reconfigure permanent must enter unattached")
	}
	if !canAttack(perm) {
		t.Fatal("(1) unattached reconfigure creature should be able to attack (eligibility baseline)")
	}

	// (2)+(3) Attach → stops being a creature (§702.151e), still Equipment.
	if !ActivateReconfigure(gs, perm, target, 2) {
		t.Fatal("reconfigure attach should succeed on a legal creature target")
	}
	if perm.IsCreature() {
		t.Error("(3) attached reconfigure permanent must NOT be a creature (§702.151e)")
	}
	if !perm.IsEquipment() {
		t.Error("(3) attached reconfigure permanent must still be an Equipment")
	}
	if perm.AttachedTo != target {
		t.Error("(3) reconfigure permanent should be attached to the target")
	}
	// Combat uses IsCreature(): an attached reconfigure equipment can't be
	// declared as an attacker.
	if canAttack(perm) {
		t.Error("(3) attached reconfigure equipment must not be eligible to attack")
	}
	// AttachmentConsistency (and every other invariant) must hold while attached.
	for _, v := range RunAllInvariants(gs) {
		if v.Name == "AttachmentConsistency" {
			t.Errorf("(5) AttachmentConsistency violated while attached: %s", v.Message)
		}
	}

	// (4) Detach → it is a creature again.
	if !ActivateReconfigure(gs, perm, nil, 2) {
		t.Fatal("reconfigure detach should succeed")
	}
	if !perm.IsCreature() {
		t.Error("(4) detached reconfigure permanent must be a creature again")
	}
	if perm.AttachedTo != nil {
		t.Error("(4) reconfigure permanent should be unattached after detach")
	}
}

// (BYPASS fix) reconfigure attach is legal-target gated — only a creature you
// control, never itself; an illegal attach is rejected with no cost paid and
// no AttachmentConsistency-breaking attachment created.
func TestReconfigure_IllegalAttachTargetRejected(t *testing.T) {
	gs := newMiscGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10
	gs.Seats[1].ManaPool = 10

	perm := addMiscBattlefield(gs, 0, "Lizard Blades", 1, 1, "artifact", "creature", "equipment")
	nonCreature := addMiscBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	oppCreature := addMiscBattlefield(gs, 1, "Opp Bear", 2, 2, "creature")

	before := gs.Seats[0].ManaPool

	// Non-creature target → rejected.
	if ActivateReconfigure(gs, perm, nonCreature, 2) {
		t.Error("reconfigure must not attach to a non-creature (§702.151c)")
	}
	// Self target → rejected (§702.6b — can't equip itself).
	if ActivateReconfigure(gs, perm, perm, 2) {
		t.Error("reconfigure must not attach to itself")
	}
	// Opponent's creature → rejected (must be a creature YOU control).
	if ActivateReconfigure(gs, perm, oppCreature, 2) {
		t.Error("reconfigure must not attach to a creature you don't control")
	}

	if perm.AttachedTo != nil {
		t.Error("rejected reconfigure must leave the permanent unattached")
	}
	if perm.IsCreature() != true {
		t.Error("rejected reconfigure must leave the permanent a creature")
	}
	if gs.Seats[0].ManaPool != before {
		t.Errorf("rejected reconfigure must not spend mana: before=%d after=%d", before, gs.Seats[0].ManaPool)
	}
}
