package gameengine

// exile_tag_dangling_ltb_r63_test.go — STATE_INTEGRITY frontier (r63),
// seed-1337 Heliod orphan: ExileLinkageIntegrity fired on
// "Heliod, Sun-Crowned in seat 2 exile linked to source timestamp 47
// which is no longer on any battlefield — LTB return missed".
//
// Root cause: a source that COUNTER-and-EXILES (Transcendent Dragon) or
// otherwise stamps card.ExiledByTimestamp as a recast / permanent-exile
// tag — and never forms an LTBReturn link — leaves that tag dangling
// when it leaves the battlefield via a normal LTB path. The card is
// correctly, permanently exiled (nothing should return), but the
// orphaned-linked-exile backstop reads the stale timestamp as a missed
// LTB return. The fix mirrors the §800.4a elimination cleanup on the
// normal-LTB exit: clear non-LTBReturn sources' exile tags.

import "testing"

// A permanent-exile/recast tag source (LinkageNone) that leaves the
// battlefield must clear the dangling ExiledByTimestamp it stamped — the
// exiled card stays in exile, but the linkage backstop must go quiet.
func TestExileTag_NonLTBReturnSourceClearsTagOnNormalLTB(t *testing.T) {
	gs := &GameState{
		Seats: []*Seat{
			{Idx: 0, Life: 40},
			{Idx: 1, Life: 40},
		},
		Flags: map[string]int{},
	}
	// Transcendent Dragon on seat 0, having countered+exiled seat 1's
	// Heliod (stamped ExiledByTimestamp directly — LinkageNone, no
	// LinkedExile, the per-card handler's exact shape).
	dragon := &Permanent{
		Card:       &Card{Name: "Transcendent Dragon", Types: []string{"creature"}, Owner: 0},
		Controller: 0, Owner: 0,
		Timestamp:   47,
		LinkageKind: LinkageNone,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, dragon)

	heliod := &Card{Name: "Heliod, Sun-Crowned", Types: []string{"creature", "enchantment", "legendary"}, Owner: 1}
	heliod.ExiledByTimestamp = 47
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, heliod)

	// While the dragon lives, the linkage is consistent.
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("baseline must be clean while the source is live: %v", err)
	}

	// The dragon dies (normal LTB: battlefield -> graveyard).
	gs.Seats[0].Battlefield = gs.Seats[0].Battlefield[:0]
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, dragon.Card)
	FireZoneChangeTriggers(gs, dragon, dragon.Card, "battlefield", "graveyard")

	// Heliod stays exiled (nothing returns), but its dangling tag must
	// be cleared so the orphaned-linked-exile backstop stays quiet.
	if heliod.ExiledByTimestamp != 0 {
		t.Errorf("dangling exile tag not cleared on normal LTB: ExiledByTimestamp=%d", heliod.ExiledByTimestamp)
	}
	stillExiled := false
	for _, c := range gs.Seats[1].Exile {
		if c == heliod {
			stillExiled = true
		}
	}
	if !stillExiled {
		t.Error("Heliod must remain in exile — the tag clear must not move the card")
	}
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Errorf("ExileLinkageIntegrity must be clean after the source leaves: %v", err)
	}
}

// The clear must NOT mask a genuine LTBReturn miss: an LTBReturn source
// (Banisher Priest / Oblivion Ring class) that left the battlefield
// WITHOUT returning its linked card must still trip the invariant.
func TestExileTag_LTBReturnMissStillFlagged(t *testing.T) {
	gs := &GameState{
		Seats: []*Seat{
			{Idx: 0, Life: 40},
			{Idx: 1, Life: 40},
		},
		Flags: map[string]int{},
	}
	victim := &Card{Name: "Grizzly Bears", Types: []string{"creature"}, Owner: 1}
	victim.ExiledByTimestamp = 51
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, victim)

	// The exiling O-Ring (LTBReturn) has already left the battlefield
	// and — bug — its return handler never ran. The tag dangles.
	oRing := &Permanent{
		Card:       &Card{Name: "Oblivion Ring", Types: []string{"enchantment"}, Owner: 0},
		Controller: 0, Owner: 0,
		Timestamp:   51,
		LinkageKind: LTBReturn,
	}

	// Fire the LTB for the (already off-battlefield) LTBReturn source.
	FireZoneChangeTriggers(gs, oRing, oRing.Card, "battlefield", "graveyard")

	// The fix skips LTBReturn sources, so the genuine miss is preserved.
	if victim.ExiledByTimestamp != 51 {
		t.Fatalf("LTBReturn tag must NOT be auto-cleared (would mask a real miss): got %d", victim.ExiledByTimestamp)
	}
	if err := checkExileLinkageIntegrity(gs); err == nil {
		t.Error("a genuine LTBReturn miss must still trip ExileLinkageIntegrity")
	}
}
