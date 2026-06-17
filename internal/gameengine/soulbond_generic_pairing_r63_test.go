package gameengine

import (
	"math/rand"
	"testing"
)

// soulbond_generic_pairing_r63_test.go — pins the GENERIC soulbond pairing
// substrate (CR §702.97e/f). The prior pass wired Deadeye Navigator's per-card
// ETB through the canonical PairSoulbond, but the generic keyword was unwired:
// no non-Deadeye soulbond creature paired. FireSoulbondTriggers (the ETB
// observer hook, mirroring evolve) now pairs any soulbond creature on its own
// ETB and pairs an already-out soulbond creature with a later ordinary ETB.
//
// These tests construct a GENERIC (non-Deadeye) soulbond creature via the
// `kw:soulbond` runtime flag (the same way other engine tests declare keywords
// without an AST) and drive pairing through the real ETB chokepoint
// (FirePermanentETBTriggers) so the wiring at fireObserverETBTriggers is
// exercised end-to-end.

func newGenericSoulbondGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(63)), nil)
}

// place a vanilla creature on the battlefield WITHOUT firing ETB triggers
// (so a test can stage the board, then enter the creature-under-test through
// FirePermanentETBTriggers).
func sbPlaceVanilla(gs *GameState, seat int, name string) *Permanent {
	card := &Card{Name: name, Owner: seat, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// place a generic SOULBOND creature (keyword via the kw: runtime flag).
func sbPlaceSoulbond(gs *GameState, seat int, name string) *Permanent {
	p := sbPlaceVanilla(gs, seat, name)
	p.Flags["kw:soulbond"] = 1
	return p
}

// (§702.97e) A soulbond creature pairs on its OWN ETB when an eligible unpaired
// creature is already in play.
func TestSoulbond_Generic_PairsOnOwnETB(t *testing.T) {
	gs := newGenericSoulbondGame(t)
	// An ordinary creature already in play.
	mate := sbPlaceVanilla(gs, 0, "Vanilla Bear")
	// The soulbond creature enters.
	sb := sbPlaceSoulbond(gs, 0, "Generic Soulbonder")

	if !HasSoulbond(sb) {
		t.Fatal("setup: generic soulbond creature should report HasSoulbond")
	}
	FirePermanentETBTriggers(gs, sb)

	if !IsPaired(sb) || !IsPaired(mate) {
		t.Errorf("(e) soulbond creature should pair with the unpaired creature already in play on its own ETB (sb=%v mate=%v)", IsPaired(sb), IsPaired(mate))
	}
}

// (§702.97f) A soulbond creature already in play pairs when a LATER ordinary
// creature enters.
func TestSoulbond_Generic_PairsWhenLaterCreatureEnters(t *testing.T) {
	gs := newGenericSoulbondGame(t)
	// The soulbond creature enters first, with nothing to pair with.
	sb := sbPlaceSoulbond(gs, 0, "Lonely Soulbonder")
	FirePermanentETBTriggers(gs, sb)
	if IsPaired(sb) {
		t.Fatal("setup: soulbond creature should be unpaired with no eligible partner")
	}

	// A later ordinary creature enters → the already-out soulbond creature
	// pairs with it.
	later := sbPlaceVanilla(gs, 0, "Later Bear")
	FirePermanentETBTriggers(gs, later)

	if !IsPaired(sb) || !IsPaired(later) {
		t.Errorf("(f) the already-out soulbond creature should pair with the later ordinary ETB (sb=%v later=%v)", IsPaired(sb), IsPaired(later))
	}
}

// (§702.97e) The pair breaks (IsPaired false on the survivor) when a partner
// leaves the battlefield. Driven end-to-end through SacrificePermanent.
func TestSoulbond_Generic_PairBreaksWhenPartnerLeaves(t *testing.T) {
	gs := newGenericSoulbondGame(t)
	mate := sbPlaceVanilla(gs, 0, "Vanilla Bear")
	sb := sbPlaceSoulbond(gs, 0, "Generic Soulbonder")
	FirePermanentETBTriggers(gs, sb)
	if !IsPaired(sb) || !IsPaired(mate) {
		t.Fatal("setup: should be paired after ETB")
	}

	// The partner leaves.
	SacrificePermanent(gs, mate, "test")

	if IsPaired(sb) {
		t.Error("(e) the survivor must become unpaired when its partner leaves the battlefield")
	}
}

// A creature pairs with only ONE other at a time: an already-paired soulbond
// creature does not re-pair while still paired, and a fresh soulbond ETB does
// not steal an already-paired creature.
func TestSoulbond_Generic_OnlyOnePairAtATime(t *testing.T) {
	gs := newGenericSoulbondGame(t)
	a := sbPlaceSoulbond(gs, 0, "Soulbonder A")
	b := sbPlaceVanilla(gs, 0, "Bear B")
	FirePermanentETBTriggers(gs, a)
	if !IsPaired(a) || !IsPaired(b) {
		t.Fatal("setup: A should pair with B on ETB")
	}

	// A second soulbond creature enters; both A and B are already paired (to
	// each other), so the newcomer must NOT steal either of them and ends up
	// unpaired (no third eligible creature).
	c := sbPlaceSoulbond(gs, 0, "Soulbonder C")
	FirePermanentETBTriggers(gs, c)

	if IsPaired(c) {
		t.Error("a new soulbond creature must not pair with already-paired creatures")
	}
	// A and B's existing pair must be undisturbed.
	if !IsPaired(a) || !IsPaired(b) {
		t.Error("the existing A<->B pair must not be broken by the newcomer")
	}
	if a.Flags["paired_timestamp"] != b.Timestamp || b.Flags["paired_timestamp"] != a.Timestamp {
		t.Error("A<->B mutual pairing must remain intact")
	}
}

// Cross-controller: a soulbond creature does not pair with an opponent's
// creature (PairSoulbond's same-controller gate), even via the ETB hook.
func TestSoulbond_Generic_DoesNotPairAcrossControllers(t *testing.T) {
	gs := newGenericSoulbondGame(t)
	opp := sbPlaceVanilla(gs, 1, "Opponent Bear")
	sb := sbPlaceSoulbond(gs, 0, "My Soulbonder")
	FirePermanentETBTriggers(gs, sb)

	if IsPaired(sb) {
		t.Error("soulbond creature must not pair with an opponent's creature")
	}
	if IsPaired(opp) {
		t.Error("opponent's creature must not be paired")
	}
}
