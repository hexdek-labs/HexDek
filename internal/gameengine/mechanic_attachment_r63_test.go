package gameengine

// r63 — Aura / Equipment ATTACHMENT consistency (CR 301/303/701.3/702.6).
// Pins the properties around the open Loki r41 "AttachmentConsistency"
// cross-seat-control cluster (stale AttachedTo after Control Magic). The
// engine keeps a single source of truth (Permanent.AttachedTo); these tests
// assert it stays consistent through control change, host-leaves, equip moves,
// and illegal attachment — and that checkAttachmentConsistency stays clean.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func attR63Game() *GameState {
	gs := NewGameState(2, rand.New(rand.NewSource(6)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func attR63Perm(gs *GameState, seat int, name string, types ...string) *Permanent {
	p := &Permanent{
		Card: &Card{Name: name, Owner: seat, Types: types, TypeLine: "x",
			BasePower: 2, BaseToughness: 2},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func attR63BfHas(gs *GameState, seat int, p *Permanent) bool {
	for _, q := range gs.Seats[seat].Battlefield {
		if q == p {
			return true
		}
	}
	return false
}

// (c) THE KNOWN BUG AREA: a creature carrying equipment + aura changes control
// across seats. AttachedTo must NOT go stale and the invariant must stay clean.
func TestAttach_ControlChangeKeepsConsistency(t *testing.T) {
	gs := attR63Game()
	// Seat 0: creature C (first, so the gain-control fallback targets it),
	// equipment E and aura A attached to it.
	c := attR63Perm(gs, 0, "Bear", "creature")
	e := attR63Perm(gs, 0, "Bonesplitter", "artifact", "equipment")
	a := attR63Perm(gs, 0, "Holy Strength", "enchantment", "aura")
	e.AttachedTo = c
	a.AttachedTo = c

	// Seat 1 controls a Control-Magic source and steals C.
	src := attR63Perm(gs, 1, "Control Magic", "enchantment", "aura")
	resolveGainControl(gs, src, &gameast.GainControl{})

	if c.Controller != 1 {
		t.Fatalf("creature should be controlled by seat 1 after the steal; got %d", c.Controller)
	}
	if !attR63BfHas(gs, 1, c) {
		t.Fatalf("stolen creature should be on seat 1's battlefield")
	}
	// Equipment/aura controllers do NOT change (CR §108.3 / control is the
	// creature's), but they remain attached to C (now on seat 1).
	if e.AttachedTo != c || a.AttachedTo != c {
		t.Fatalf("equipment/aura must still be attached to the stolen creature (no stale AttachedTo)")
	}
	if e.Controller != 0 || a.Controller != 0 {
		t.Fatalf("equipment/aura controllers should stay with their owner (0); got e=%d a=%d", e.Controller, a.Controller)
	}
	if err := checkAttachmentConsistency(gs); err != nil {
		t.Fatalf("AttachmentConsistency must stay clean after cross-seat control change: %v", err)
	}
}

// (b)+(f) host leaves: all auras go to the graveyard (SBA 704.5m), equipment
// unattaches but stays on the battlefield (704.5n).
func TestAttach_HostLeavesAurasDieEquipmentStays(t *testing.T) {
	gs := attR63Game()
	c := attR63Perm(gs, 0, "Big Creature", "creature")
	e := attR63Perm(gs, 0, "Sword", "artifact", "equipment")
	a1 := attR63Perm(gs, 0, "Aura One", "enchantment", "aura")
	a2 := attR63Perm(gs, 0, "Aura Two", "enchantment", "aura")
	e.AttachedTo, a1.AttachedTo, a2.AttachedTo = c, c, c

	DestroyPermanent(gs, c, nil)
	StateBasedActions(gs)

	if attR63BfHas(gs, 0, c) {
		t.Fatalf("destroyed creature should be gone")
	}
	if !attR63InGrave(gs, 0, a1) || !attR63InGrave(gs, 0, a2) {
		t.Fatalf("auras should be put into the graveyard when the host leaves (704.5m)")
	}
	if !attR63BfHas(gs, 0, e) {
		t.Fatalf("equipment should REMAIN on the battlefield when the host leaves (704.5n)")
	}
	if e.AttachedTo != nil {
		t.Fatalf("equipment should be UNATTACHED after the host leaves; AttachedTo=%v", e.AttachedTo)
	}
}

// (d) equip move detaches from the old host cleanly.
func TestAttach_EquipMoveDetachesOldHost(t *testing.T) {
	gs := attR63Game()
	gs.Active = 0
	c1 := attR63Perm(gs, 0, "Host One", "creature")
	c2 := attR63Perm(gs, 0, "Host Two", "creature")
	e := attR63Perm(gs, 0, "Equip", "artifact", "equipment")
	e.AttachedTo = c1
	gs.Seats[0].ManaPool = 5

	if !ActivateEquip(gs, 0, e, c2) {
		t.Fatalf("equip to a new host should succeed")
	}
	if e.AttachedTo != c2 {
		t.Fatalf("equipment should now be attached to the new host")
	}
	// The single-pointer model means c1 has nothing pointing at it.
	for _, p := range gs.Seats[0].Battlefield {
		if p.AttachedTo == c1 {
			t.Fatalf("old host should have no attachments after the equip move")
		}
	}
	if err := checkAttachmentConsistency(gs); err != nil {
		t.Fatalf("consistency after equip move: %v", err)
	}
}

// (e) an aura whose host is gone is put into the graveyard by SBA.
func TestAttach_IllegalAuraToGraveyard(t *testing.T) {
	gs := attR63Game()
	c := attR63Perm(gs, 0, "Host", "creature")
	a := attR63Perm(gs, 0, "Lonely Aura", "enchantment", "aura")
	a.AttachedTo = c
	// Host leaves the battlefield (removed without detach).
	gs.removePermanent(c)

	StateBasedActions(gs)

	if attR63BfHas(gs, 0, a) {
		t.Fatalf("aura with no legal host should leave the battlefield")
	}
	if !attR63InGrave(gs, 0, a) {
		t.Fatalf("illegally-attached aura should be put into the graveyard (704.5m)")
	}
}

func attR63InGrave(gs *GameState, seat int, p *Permanent) bool {
	for _, c := range gs.Seats[seat].Graveyard {
		if p.Card != nil && c == p.Card {
			return true
		}
	}
	return false
}
