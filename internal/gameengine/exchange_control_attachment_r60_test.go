package gameengine

import (
	"testing"
)

// TestExchangeControl_PreservesAttachedAuras pins the r60 ExchangeControl
// fix. Control exchange (CR §701.10) is NOT a leaves-the-battlefield event
// — both permanents stay on the battlefield, only their controllers swap.
// Auras and equipment attached to either permanent must remain attached
// (CR §702.6: control change alone does not detach).
//
// Before the fix, ExchangeControl routed both permanents through
// removePermanentFromBattlefield, which after the 2026-05-23
// AttachmentConsistency fix runs detachAll. Auras therefore had their
// AttachedTo silently zeroed mid-exchange and were then destroyed by
// SBA §704.5n on the next pass.
func TestExchangeControl_PreservesAttachedAuras(t *testing.T) {
	gs := newMultiplayerGame(t, 3)
	creature1 := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	creature2 := addBattlefield(gs, 2, "Hill Giant", 3, 3, "creature")

	aura := addBattlefield(gs, 0, "Holy Strength", 0, 0, "enchantment", "aura")
	aura.AttachedTo = creature1
	gear := addBattlefield(gs, 2, "Bone Saw", 0, 0, "artifact", "equipment")
	gear.AttachedTo = creature2

	ExchangeControl(gs, creature1, creature2)

	if aura.AttachedTo != creature1 {
		t.Fatalf("aura should remain attached to creature1 across control exchange, got %v", aura.AttachedTo)
	}
	if gear.AttachedTo != creature2 {
		t.Fatalf("equipment should remain attached to creature2 across control exchange, got %v", gear.AttachedTo)
	}
	if err := checkAttachmentConsistency(gs); err != nil {
		t.Fatalf("AttachmentConsistency violation after ExchangeControl: %v", err)
	}
}

// TestExchangeControl_MovesBetweenBattlefields pins the corollary: each
// exchanged permanent ends up on exactly one battlefield — its new
// controller's — and is gone from its old controller's. Before the r60
// fix, ExchangeControl swapped Controller fields BEFORE calling
// removePermanentFromBattlefield, so the removal scanned the post-swap
// (wrong) slice and silently no-op'd, leaving both perms duplicated
// across both battlefields and tripping CardIdentity.
func TestExchangeControl_MovesBetweenBattlefields(t *testing.T) {
	gs := newMultiplayerGame(t, 3)
	creature1 := addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")
	creature2 := addBattlefield(gs, 2, "Shock Troops", 2, 1, "creature")

	ExchangeControl(gs, creature1, creature2)

	if creature1.Controller != 2 {
		t.Fatalf("creature1 controller should be 2, got %d", creature1.Controller)
	}
	if creature2.Controller != 0 {
		t.Fatalf("creature2 controller should be 0, got %d", creature2.Controller)
	}

	countOn := func(seat int, p *Permanent) int {
		n := 0
		for _, q := range gs.Seats[seat].Battlefield {
			if q == p {
				n++
			}
		}
		return n
	}
	if got := countOn(0, creature1); got != 0 {
		t.Fatalf("creature1 should be gone from seat 0 battlefield, found %d copies", got)
	}
	if got := countOn(2, creature1); got != 1 {
		t.Fatalf("creature1 should be on seat 2 battlefield exactly once, found %d copies", got)
	}
	if got := countOn(2, creature2); got != 0 {
		t.Fatalf("creature2 should be gone from seat 2 battlefield, found %d copies", got)
	}
	if got := countOn(0, creature2); got != 1 {
		t.Fatalf("creature2 should be on seat 0 battlefield exactly once, found %d copies", got)
	}
}

// TestExchangeControl_KeepsReplacementsRegistered guards against
// regressing into the LTB-equivalent helper. Replacement effects sourced
// from the exchanged permanents must keep firing — they are not LTB and
// the source is still on the battlefield, just under a different
// controller. The prior code called UnregisterReplacementsForPermanent
// via removePermanentFromBattlefield, silently zeroing every replacement
// the perms had registered.
func TestExchangeControl_KeepsReplacementsRegistered(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	perm1 := addBattlefield(gs, 0, "Furnace of Rath", 0, 0, "enchantment")
	perm2 := addBattlefield(gs, 1, "Cradle of Vitality", 0, 0, "enchantment")

	gs.RegisterReplacement(&ReplacementEffect{SourcePerm: perm1})
	gs.RegisterReplacement(&ReplacementEffect{SourcePerm: perm2})

	if got := len(gs.Replacements); got != 2 {
		t.Fatalf("preconditions: expected 2 replacements, got %d", got)
	}

	ExchangeControl(gs, perm1, perm2)

	if got := len(gs.Replacements); got != 2 {
		t.Fatalf("ExchangeControl should NOT unregister replacements; got %d (want 2)", got)
	}
}

// TestExchangeControl_SamePermNoOps defends against perm1==perm2 edge
// cases (e.g. an effect picks the same target twice). The function
// should leave the single perm in place untouched rather than
// double-removing or double-appending it.
func TestExchangeControl_SamePermNoOps(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	p := addBattlefield(gs, 0, "Plains", 0, 0, "land")

	ExchangeControl(gs, p, p)

	if p.Controller != 0 {
		t.Fatalf("controller should be unchanged on self-exchange, got %d", p.Controller)
	}
	count := 0
	for _, q := range gs.Seats[0].Battlefield {
		if q == p {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("self-exchange should leave perm on its battlefield exactly once, got %d", count)
	}
}
