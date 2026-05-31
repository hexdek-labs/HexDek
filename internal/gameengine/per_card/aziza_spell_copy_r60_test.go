package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// aziza_spell_copy_r60_test.go — Phase G regression pin for the
// Aziza, Mage Tower Captain spell-copy fabrication closure.
//
// Loki r60 seed-42 5K verification (PR #851 + #853 + #773 / Phase F at
// scale) left a single residual signature: 34 ZoneConservation hits on
// game 2762 / Lash Out / h1OGVR200056 across turns 44-60. All 34 hits
// traced to azizaSpellCopy aliasing the originating *Card pointer
// directly into a §707.2 StackItem:
//
//	copyItem := &gameengine.StackItem{
//	    Card:   castCard,       // ← original pointer, no DeepCopy / no mint
//	    IsCopy: true,
//	}
//	gs.Stack = append(gs.Stack, copyItem)
//
// When the copy resolves, stack.go:1312 fires
// `MarkInstanceIDCeased(gs, item.Card.InstanceID)` to enforce the
// §707.10 "copy ceases to exist" rule — but item.Card IS the source
// card, so the source's InstanceID retires while the underlying Card
// is still in seat-N's hand / graveyard / wherever. Every subsequent
// invariant tick then walks seat N's zones, finds Lash Out present,
// and flags fabrication.
//
// Phase F closed 10 sibling sites (alania / zada / krark / mica /
// mendicant / rootha / kalamax / ivy / fire_lord_azula / ulalek) via
// the canonical MintSpellCopy chokepoint. Aziza was surfaced in the
// Phase F audit and intentionally deferred to Phase G as the only
// remaining offender that aliased the source pointer outright (the 10
// closed sites at least called DeepCopy first, sharing the ID via
// inheritance rather than pointer aliasing).
//
// Phase G fix routes Aziza through MintSpellCopy + PushStackItem,
// matching the canonical pattern.

// TestAziza_SpellCopy_DistinctInstanceID pins the Phase G fix at the
// per_card surface: after Aziza's trigger fires and pushes a copy, the
// copy's StackItem.Card has its OWN InstanceID, not the source's. This
// is the structural property whose violation drove the 34-hit Loki
// signature.
func TestAziza_SpellCopy_DistinctInstanceID(t *testing.T) {
	gs := newTestGS(4)
	gs.Turn = 1

	// Seat 0 controls Aziza + 3 untapped creatures (the trigger cost).
	azizaCard := &gameengine.Card{Name: "Aziza, Mage Tower Captain", Owner: 0, Types: []string{"legendary", "creature"}}
	gameengine.MintOGInstanceID(gs, azizaCard)
	aziza := &gameengine.Permanent{
		Card:       azizaCard,
		Controller: 0,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, aziza)

	for i := 0; i < 3; i++ {
		c := &gameengine.Card{Name: "Goblin Tap Cost Filler", Owner: 0, Types: []string{"creature"}}
		gameengine.MintOGInstanceID(gs, c)
		p := &gameengine.Permanent{
			Card:       c,
			Controller: 0,
			Owner:      0,
			Counters:   map[string]int{},
			Flags:      map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	}

	// Seat 0 casts Lash Out (instant). Stage the source on the stack.
	lashOut := &gameengine.Card{Name: "Lash Out", Owner: 0, Colors: []string{"R"}, CMC: 3, Types: []string{"instant"}}
	gameengine.MintOGInstanceID(gs, lashOut)
	srcID := lashOut.InstanceID
	if srcID == "" {
		t.Fatal("source mint failed")
	}
	origItem := &gameengine.StackItem{Controller: 0, Card: lashOut, Kind: "spell"}
	gameengine.PushStackItem(gs, origItem)

	stackSizeBefore := len(gs.Stack)

	// Fire Aziza's trigger as if from instant_or_sorcery_cast dispatch.
	azizaSpellCopy(gs, aziza, map[string]interface{}{
		"caster_seat": 0,
		"card":        lashOut,
	})

	if len(gs.Stack) != stackSizeBefore+1 {
		t.Fatalf("expected one new StackItem (the copy); stack: %d → %d", stackSizeBefore, len(gs.Stack))
	}
	copyItem := gs.Stack[len(gs.Stack)-1]
	if copyItem == nil || copyItem.Card == nil {
		t.Fatal("copy StackItem or its Card is nil")
	}
	if !copyItem.IsCopy {
		t.Fatal("copy StackItem must have IsCopy=true")
	}
	if copyItem.Card == lashOut {
		t.Fatal("copy aliased source *Card pointer (Phase G regression — must DeepCopy via MintSpellCopy)")
	}
	if copyItem.Card.InstanceID == "" {
		t.Fatal("copy *Card has empty InstanceID")
	}
	if copyItem.Card.InstanceID == srcID {
		t.Fatalf("copy InstanceID collides with source %q (Phase G regression — MintSpellCopy must mint a fresh ID)", srcID)
	}
	if !copyItem.Card.IsCopy {
		t.Fatal("copy *Card must carry IsCopy=true (MintSpellCopy contract)")
	}
}

// TestAziza_SourceIDSurvivesCopyResolution drives the end-to-end leak
// shape: after the copy is built AND ceased per CR §707.10, the source
// card's InstanceID must remain in (Minted - Ceased). This is the
// exact invariant violation observed in Loki r60 game 2762.
func TestAziza_SourceIDSurvivesCopyResolution(t *testing.T) {
	gs := newTestGS(4)
	gs.Turn = 1
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["instanceid_strict_census"] = 1

	azizaCard := &gameengine.Card{Name: "Aziza, Mage Tower Captain", Owner: 0, Types: []string{"legendary", "creature"}}
	gameengine.MintOGInstanceID(gs, azizaCard)
	aziza := &gameengine.Permanent{
		Card: azizaCard, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, aziza)
	for i := 0; i < 3; i++ {
		c := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"creature"}}
		gameengine.MintOGInstanceID(gs, c)
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
			&gameengine.Permanent{
				Card: c, Controller: 0, Owner: 0,
				Counters: map[string]int{}, Flags: map[string]int{},
			})
	}

	lashOut := &gameengine.Card{Name: "Lash Out", Owner: 0, Colors: []string{"R"}, CMC: 3, Types: []string{"instant"}}
	gameengine.MintOGInstanceID(gs, lashOut)
	srcID := lashOut.InstanceID
	// Source is briefly on the stack as the original cast.
	gameengine.PushStackItem(gs, &gameengine.StackItem{Controller: 0, Card: lashOut, Kind: "spell"})

	azizaSpellCopy(gs, aziza, map[string]interface{}{
		"caster_seat": 0,
		"card":        lashOut,
	})

	copyItem := gs.Stack[len(gs.Stack)-1]
	copyID := copyItem.Card.InstanceID

	// Drop both stack items and route the source to the graveyard (where
	// the original cast would land per CR §608.2g), then cease the copy
	// ID per CR §707.10 — mirroring stack.go:1312's branch.
	gs.Stack = gs.Stack[:0]
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, lashOut)
	gameengine.MarkInstanceIDCeased(gs, copyID)

	if _, ceased := gs.CeasedInstanceIDs[copyID]; !ceased {
		t.Fatalf("copy ID %q must be ceased after §707.10", copyID)
	}
	if _, ceased := gs.CeasedInstanceIDs[srcID]; ceased {
		t.Fatalf("source ID %q must NOT be ceased after copy resolves "+
			"(Phase G leak fingerprint — Loki r60 game 2762 / Lash Out)", srcID)
	}
}

// TestAziza_InsufficientCreatures_NoCopy asserts the cost-pay guard:
// when fewer than 3 untapped friendly creatures are available, the
// optional "may" goes unpaid and no copy is pushed. Defense against an
// over-eager fix that pushed copies regardless of cost satisfaction.
func TestAziza_InsufficientCreatures_NoCopy(t *testing.T) {
	gs := newTestGS(4)
	gs.Turn = 1

	azizaCard := &gameengine.Card{Name: "Aziza, Mage Tower Captain", Owner: 0, Types: []string{"legendary", "creature"}}
	gameengine.MintOGInstanceID(gs, azizaCard)
	aziza := &gameengine.Permanent{
		Card: azizaCard, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, aziza)

	// Only 2 friendly creatures available — short by 1.
	for i := 0; i < 2; i++ {
		c := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"creature"}}
		gameengine.MintOGInstanceID(gs, c)
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
			&gameengine.Permanent{
				Card: c, Controller: 0, Owner: 0,
				Counters: map[string]int{}, Flags: map[string]int{},
			})
	}

	lashOut := &gameengine.Card{Name: "Lash Out", Owner: 0, CMC: 3, Types: []string{"instant"}}
	gameengine.MintOGInstanceID(gs, lashOut)
	gameengine.PushStackItem(gs, &gameengine.StackItem{Controller: 0, Card: lashOut, Kind: "spell"})

	beforeStack := len(gs.Stack)
	azizaSpellCopy(gs, aziza, map[string]interface{}{
		"caster_seat": 0,
		"card":        lashOut,
	})
	if len(gs.Stack) != beforeStack {
		t.Fatalf("no copy should be pushed without 3 untapped creatures; stack: %d → %d", beforeStack, len(gs.Stack))
	}
}

// TestAziza_OpponentCast_NoCopy guards the controller-gating: Aziza
// only triggers off her CONTROLLER's casts, not opponents'. The handler
// early-returns when caster_seat != perm.Controller.
func TestAziza_OpponentCast_NoCopy(t *testing.T) {
	gs := newTestGS(4)
	gs.Turn = 1

	azizaCard := &gameengine.Card{Name: "Aziza, Mage Tower Captain", Owner: 0, Types: []string{"legendary", "creature"}}
	gameengine.MintOGInstanceID(gs, azizaCard)
	aziza := &gameengine.Permanent{
		Card: azizaCard, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, aziza)
	for i := 0; i < 5; i++ {
		c := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"creature"}}
		gameengine.MintOGInstanceID(gs, c)
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
			&gameengine.Permanent{
				Card: c, Controller: 0, Owner: 0,
				Counters: map[string]int{}, Flags: map[string]int{},
			})
	}

	// Opponent (seat 1) casts the spell.
	oppSpell := &gameengine.Card{Name: "Lightning Bolt", Owner: 1, CMC: 1, Types: []string{"instant"}}
	gameengine.MintOGInstanceID(gs, oppSpell)
	gameengine.PushStackItem(gs, &gameengine.StackItem{Controller: 1, Card: oppSpell, Kind: "spell"})

	beforeStack := len(gs.Stack)
	azizaSpellCopy(gs, aziza, map[string]interface{}{
		"caster_seat": 1,
		"card":        oppSpell,
	})
	if len(gs.Stack) != beforeStack {
		t.Fatalf("Aziza must not copy opponent casts; stack: %d → %d", beforeStack, len(gs.Stack))
	}
}
