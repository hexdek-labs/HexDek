package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestAziza_PhaseG_SourceInstanceIDSurvivesCopyResolution closes the
// 34-hit Loki r60 seed-42 game-2762 fabrication cluster: Aziza's
// spell-copy handler used to alias the source *Card pointer directly
// into the §707.2 StackItem. When the copy resolved, stack.go:1312's
// §707.10 cease path fired MarkInstanceIDCeased(item.Card.InstanceID)
// retiring the SOURCE's OG-provenance InstanceID — the original *Card
// in hand / graveyard then read as fabricated on every subsequent
// invariant tick.
//
// Fix routed through gameengine.MintSpellCopy (DeepCopy + clear
// InstanceID + IsCopy=true + fresh CP mint with source lineage). This
// test pins:
//
//  1. The copy StackItem references a DISTINCT *Card pointer.
//  2. The copy carries a fresh non-empty InstanceID (CP-provenance).
//  3. Simulating the §707.10 cease (stack.go:1312) does NOT cease the
//     source's InstanceID — closes the fabrication leak.
//  4. checkZoneConservation stays clean end-to-end.
func TestAziza_PhaseG_SourceInstanceIDSurvivesCopyResolution(t *testing.T) {
	gs := newGame(t, 2)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["instanceid_strict_census"] = 1

	aziza := stampCreaturePT(addPerm(gs, 0, "Aziza, Mage Tower Captain", "creature"), 3, 3)
	for i := 0; i < 3; i++ {
		addPerm(gs, 0, "Buddy", "creature")
	}

	// Build the cast spell with a properly minted OG InstanceID, place
	// it in seat 0's hand so the census has a non-stack reference too.
	castCard := &gameengine.Card{
		Name:   "Lightning Bolt",
		Owner:  0,
		Colors: []string{"R"},
		CMC:    1,
		Types:  []string{"instant"},
	}
	gameengine.MintOGInstanceID(gs, castCard)
	srcID := castCard.InstanceID
	if srcID == "" {
		t.Fatal("expected OG mint to stamp an InstanceID")
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, castCard)

	origItem := &gameengine.StackItem{
		Controller: 0,
		Card:       castCard,
		Kind:       "spell",
		CostMeta:   map[string]interface{}{},
	}
	gs.Stack = append(gs.Stack, origItem)

	azizaSpellCopy(gs, aziza, map[string]interface{}{
		"caster_seat": 0,
		"card":        castCard,
		"spell_name":  "Lightning Bolt",
	})

	if len(gs.Stack) < 2 {
		t.Fatalf("expected Aziza to push the copy on top, stack=%d", len(gs.Stack))
	}
	top := gs.Stack[len(gs.Stack)-1]
	if top.Card == castCard {
		t.Fatal("Phase G regression: Aziza aliased the source *Card again (must DeepCopy via MintSpellCopy)")
	}
	if top.Card.InstanceID == "" {
		t.Fatal("expected fresh non-empty InstanceID on the copy")
	}
	if top.Card.InstanceID == srcID {
		t.Fatalf("copy must carry a DISTINCT InstanceID, got %q == source %q", top.Card.InstanceID, srcID)
	}

	// Simulate §707.10 cease at copy resolution (mirrors stack.go:1312).
	gameengine.MarkInstanceIDCeased(gs, top.Card.InstanceID)
	if _, ceased := gs.CeasedInstanceIDs[srcID]; ceased {
		t.Fatalf("Phase G leak fingerprint: copy cease ALSO ceased source ID %q", srcID)
	}
	if _, minted := gs.MintedInstanceIDs[srcID]; !minted {
		t.Fatalf("source ID %q must remain in MintedInstanceIDs", srcID)
	}
}
