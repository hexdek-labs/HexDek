package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// deadeye_soulbond_canonical_r63_test.go — pins that Deadeye Navigator's ETB
// auto-pair routes through the canonical gameengine.PairSoulbond primitive
// (CR §702.97) instead of re-implementing the mutual paired_timestamp wiring
// inline. The load-bearing consequence: a pair established by the primitive is
// readable by gameengine.IsPaired, so the engine's LTB / control-change
// UnpairOnLeave hooks correctly break the pair when a creature leaves — the
// behavior an inline-only paired_timestamp set risked diverging from.

// Deadeye's ETB pairs it with an unpaired creature already on the battlefield,
// and the pair is canonical (both ends report IsPaired).
func TestDeadeye_ETBAutoPairs_Canonical(t *testing.T) {
	gs := newGame(t, 2)
	drake := addPerm(gs, 0, "Peregrine Drake", "creature")
	deadeye := addPerm(gs, 0, "Deadeye Navigator", "creature")

	gameengine.InvokeETBHook(gs, deadeye)

	if !gameengine.IsPaired(deadeye) {
		t.Error("Deadeye should be paired after ETB auto-pair")
	}
	if !gameengine.IsPaired(drake) {
		t.Error("the partner should be paired after Deadeye's ETB auto-pair")
	}
	if hasEvent(gs, "soulbond_pair") == 0 {
		t.Error("canonical PairSoulbond should emit a soulbond_pair event")
	}
}

// When the auto-paired partner LEAVES the battlefield, the engine's LTB
// chokepoint (UnpairOnLeave) breaks the pair — Deadeye becomes unpaired again.
// This is the behavior that depends on the pair being canonical: an inline
// paired_timestamp set that the engine's IsPaired-driven unpair hooks didn't
// recognize would leave Deadeye permanently "paired" to a gone creature.
func TestDeadeye_PartnerLeaves_UnpairsCanonically(t *testing.T) {
	gs := newGame(t, 2)
	drake := addPerm(gs, 0, "Peregrine Drake", "creature")
	deadeye := addPerm(gs, 0, "Deadeye Navigator", "creature")

	gameengine.InvokeETBHook(gs, deadeye)
	if !gameengine.IsPaired(deadeye) || !gameengine.IsPaired(drake) {
		t.Fatal("setup: both should be paired after ETB")
	}

	// Drake dies — the LTB chokepoint must unpair Deadeye.
	gameengine.SacrificePermanent(gs, drake, "test")

	if gameengine.IsPaired(deadeye) {
		t.Error("Deadeye must become unpaired when its auto-paired partner leaves")
	}
}

// Deadeye does not pair with an already-paired creature (the canonical gate):
// a creature already paired to someone else is skipped, and Deadeye stays
// unpaired when no other eligible creature exists.
func TestDeadeye_SkipsAlreadyPaired(t *testing.T) {
	gs := newGame(t, 2)
	a := addPerm(gs, 0, "Bonded A", "creature")
	b := addPerm(gs, 0, "Bonded B", "creature")
	if !gameengine.PairSoulbond(gs, a, b) {
		t.Fatal("setup: PairSoulbond should pair a and b")
	}

	deadeye := addPerm(gs, 0, "Deadeye Navigator", "creature")
	gameengine.InvokeETBHook(gs, deadeye)

	if gameengine.IsPaired(deadeye) {
		t.Error("Deadeye must not pair when the only other creatures are already paired")
	}
	// The pre-existing pair is untouched.
	if !gameengine.IsPaired(a) || !gameengine.IsPaired(b) {
		t.Error("Deadeye's ETB must not disturb an existing soulbond pair")
	}
}
