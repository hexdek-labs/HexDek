package gameengine

import (
	"testing"
)

// instanceid_phase_g_sweep_test.go — Phase G sibling-site closure pins
// for the 4 spell-copy bypasses surfaced by the audit that followed the
// Aziza fix (PR #873).
//
// The Aziza pattern — push an IsCopy=true StackItem onto gs.Stack with
// `Card: sourcePointer` (no DeepCopy, no MintSpellCopy) — also lived in
// the engine. When the copy resolved, stack.go's §707.10 cease branch
// fired `MarkInstanceIDCeased(item.Card.InstanceID)` on the SOURCE's
// ID, then the source card living elsewhere was flagged as fabrication.
// Phase G sweep audit found two such bypasses:
//
//   - ApplyConspire (keywords_batch4.go) — CR §702.78
//   - Epic delayed trigger (keywords_batch6.go ApplyEpic) — CR §702.50
//
// Plus two inline manual-mint sites that worked but didn't clear the
// full lineage field set (`SourceInstanceID`, `EnablerInstanceID`) that
// `MintSpellCopy` zeroes:
//
//   - resolve.go's copy_spell handler (CR §707.2)
//   - phases.go's paradigm-copy loop
//
// All four now route through MintSpellCopy. These tests pin the
// structural property and the end-to-end leak shape per site.

// pinDistinctIDViaMintSpellCopy is a shared property: after calling
// MintSpellCopy on a source *Card with an OG InstanceID, the returned
// copy must have its own non-empty CP-provenance ID, distinct from the
// source's. This is the central invariant Phase F + G ride on.
func pinDistinctIDViaMintSpellCopy(t *testing.T, name string) (src, cp *Card, srcID string) {
	t.Helper()
	gs := newPhase2GameState(t)
	src = &Card{Name: name, Owner: 0, Colors: []string{"R"}, CMC: 2, Types: []string{"instant"}}
	MintOGInstanceID(gs, src)
	srcID = src.InstanceID
	if srcID == "" {
		t.Fatal("source mint failed")
	}
	cp = MintSpellCopy(gs, src)
	if cp == nil {
		t.Fatal("MintSpellCopy returned nil")
	}
	if cp == src {
		t.Fatal("copy must be a fresh *Card, not the source pointer")
	}
	if cp.InstanceID == "" {
		t.Fatal("copy has empty InstanceID")
	}
	if cp.InstanceID == srcID {
		t.Fatalf("copy InstanceID %q collides with source", srcID)
	}
	if !cp.IsCopy {
		t.Fatal("copy must have IsCopy=true")
	}
	_ = gs
	return
}

// TestPhaseG_Conspire_RoutesThroughMintSpellCopy pins the Conspire fix:
// ApplyConspire now builds the copy StackItem via MintSpellCopy, so the
// copy's InstanceID is distinct from the source. Pre-fix it aliased
// `item.Card` directly — Aziza-shape bypass.
func TestPhaseG_Conspire_RoutesThroughMintSpellCopy(t *testing.T) {
	gs := newPhase2GameState(t)

	// Seat 0 controls 2 untapped colored creatures + 1 commander stand-in
	// to anchor the seat; ApplyConspire wants 2 friendly creatures sharing
	// a color with the spell.
	for i := 0; i < 2; i++ {
		c := &Card{Name: "Goblin Conspirator", Owner: 0, Colors: []string{"R"}, Types: []string{"creature"}}
		MintOGInstanceID(gs, c)
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
			&Permanent{
				Card: c, Controller: 0, Owner: 0,
				Counters: map[string]int{}, Flags: map[string]int{},
			})
	}

	src := &Card{Name: "Lightning Bolt", Owner: 0, Colors: []string{"R"}, CMC: 1, Types: []string{"instant"}}
	MintOGInstanceID(gs, src)
	srcID := src.InstanceID
	srcItem := &StackItem{Controller: 0, Card: src, Kind: "spell"}
	PushStackItem(gs, srcItem)

	stackBefore := len(gs.Stack)
	if !ApplyConspire(gs, 0, srcItem) {
		t.Fatal("ApplyConspire returned false despite cost-payable board")
	}
	if len(gs.Stack) != stackBefore+1 {
		t.Fatalf("expected one copy pushed; stack: %d → %d", stackBefore, len(gs.Stack))
	}
	copyItem := gs.Stack[len(gs.Stack)-1]
	if !copyItem.IsCopy {
		t.Fatal("copy StackItem must have IsCopy=true")
	}
	if copyItem.Card == src {
		t.Fatal("Conspire copy aliased source *Card pointer (Phase G regression — must route through MintSpellCopy)")
	}
	if copyItem.Card.InstanceID == srcID {
		t.Fatalf("Conspire copy ID %q collides with source", srcID)
	}

	// End-to-end leak: cease the copy ID (mirroring §707.10 at resolve),
	// the source ID must remain in (Minted - Ceased).
	MarkInstanceIDCeased(gs, copyItem.Card.InstanceID)
	if _, ceased := gs.CeasedInstanceIDs[srcID]; ceased {
		t.Fatalf("Conspire source ID %q must NOT be ceased after copy resolves", srcID)
	}
}

// TestPhaseG_Epic_RoutesThroughMintSpellCopy pins the Epic delayed-
// trigger fix: when the upkeep EffectFn fires, the pushed copy must be
// freshly minted, not aliased to the captured epicCard. Each upkeep
// fires a new copy — pre-fix every upkeep ceased the SOURCE on resolve,
// breaking the source's zone presence in the graveyard.
func TestPhaseG_Epic_RoutesThroughMintSpellCopy(t *testing.T) {
	gs := newPhase2GameState(t)
	gs.Turn = 1

	src := &Card{Name: "Eternal Dominion", Owner: 0, Colors: []string{"U"}, CMC: 9, Types: []string{"sorcery"}}
	MintOGInstanceID(gs, src)
	srcID := src.InstanceID
	srcItem := &StackItem{Controller: 0, Card: src, Kind: "spell"}
	PushStackItem(gs, srcItem)

	delayedBefore := len(gs.DelayedTriggers)
	ApplyEpic(gs, 0, srcItem)
	if len(gs.DelayedTriggers) != delayedBefore+1 {
		t.Fatalf("ApplyEpic must register exactly one DelayedTrigger; got %d → %d",
			delayedBefore, len(gs.DelayedTriggers))
	}
	epicDT := gs.DelayedTriggers[len(gs.DelayedTriggers)-1]
	if epicDT.EffectFn == nil {
		t.Fatal("Epic DelayedTrigger has nil EffectFn")
	}

	// Drive the EffectFn directly: in the real engine ApplyEpic registers
	// the trigger but the `TriggerAt: "upkeep"` value isn't currently
	// matched by `delayedTriggerMatches` — wiring the matcher is out of
	// Phase G scope. The Phase G contract is about the COPY-CREATION
	// shape inside the closure, regardless of dispatch wiring. Active
	// seat must equal the registered controller for the closure's own
	// gate to pass.
	gs.Active = 0
	stackBefore := len(gs.Stack)
	epicDT.EffectFn(gs)
	if len(gs.Stack) != stackBefore+1 {
		t.Fatalf("expected Epic EffectFn to push one upkeep copy; stack: %d → %d", stackBefore, len(gs.Stack))
	}
	copyItem := gs.Stack[len(gs.Stack)-1]
	if !copyItem.IsCopy {
		t.Fatal("Epic copy StackItem must have IsCopy=true")
	}
	if copyItem.Card == src {
		t.Fatal("Epic copy aliased source *Card pointer (Phase G regression — must route through MintSpellCopy)")
	}
	if copyItem.Card.InstanceID == srcID {
		t.Fatalf("Epic copy ID %q collides with source", srcID)
	}

	// Cease the copy ID; source must survive.
	MarkInstanceIDCeased(gs, copyItem.Card.InstanceID)
	if _, ceased := gs.CeasedInstanceIDs[srcID]; ceased {
		t.Fatalf("Epic source ID %q must NOT be ceased after upkeep copy resolves", srcID)
	}
}

// TestPhaseG_MintSpellCopy_ClearsFullLineage pins the inline-mint
// closure for resolve.go's copy_spell handler and phases.go's paradigm-
// copy loop. Pre-fix, both sites zeroed only InstanceID + EnablerHistory
// before calling MintCopyInstanceID. SourceInstanceID + EnablerInstanceID
// could leak from the source onto the copy. Routing through MintSpellCopy
// clears all four. This test pins MintSpellCopy's contract.
func TestPhaseG_MintSpellCopy_ClearsFullLineage(t *testing.T) {
	gs := newPhase2GameState(t)
	src := &Card{
		Name:              "Twincast",
		Owner:             0,
		Colors:            []string{"U"},
		CMC:               4,
		Types:             []string{"instant"},
		SourceInstanceID:  "h0OGVU0000xx", // simulated prior-lineage stamp
		EnablerInstanceID: "h0OGVU0000yy",
		EnablerHistory:    []string{"h0OGVU0000zz"},
	}
	MintOGInstanceID(gs, src)
	cp := MintSpellCopy(gs, src)
	if cp == nil {
		t.Fatal("MintSpellCopy returned nil")
	}
	if cp.SourceInstanceID == "h0OGVU0000xx" {
		t.Errorf("MintSpellCopy must clear SourceInstanceID (got %q)", cp.SourceInstanceID)
	}
	if cp.EnablerInstanceID == "h0OGVU0000yy" {
		t.Errorf("MintSpellCopy must clear EnablerInstanceID (got %q)", cp.EnablerInstanceID)
	}
	if len(cp.EnablerHistory) > 0 {
		// MintCopyInstanceID may append the source's enabler to the copy's
		// history (legitimate lineage). The Phase G contract: any pre-mint
		// history from the source must be cleared first. Acceptable if the
		// post-mint history contains only the OG enabler IDs the helper
		// itself appended.
		for _, e := range cp.EnablerHistory {
			if e == "h0OGVU0000zz" {
				t.Errorf("MintSpellCopy must clear pre-mint EnablerHistory entries; saw stale %q", e)
			}
		}
	}
}
