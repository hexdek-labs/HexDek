package gameengine

import (
	"strings"
	"testing"
)

// TestPhaseE_OrphanSweep_TokenAbsentFromAllZones pins the dominant case:
// a TK-provenance token is minted, lands on the battlefield, then its
// *Card pointer is dropped from every zone via a non-canonical path (the
// classic shape behind the 2,336 TK disappearances in PR #773's
// residual). The sweep should cease the ID and emit an audit event.
func TestPhaseE_OrphanSweep_TokenAbsentFromAllZones(t *testing.T) {
	gs := newPhase4GameState(t)
	tok := &Card{Name: "Treasure", Owner: 0, Types: []string{"artifact", "token"}}
	MintTokenInstanceID(gs, tok, "", "")
	if tok.InstanceID == "" {
		t.Fatalf("expected MintTokenInstanceID to stamp an ID")
	}
	id := tok.InstanceID
	// Token is registered in MintedInstanceIDs but is in no zone — simulating
	// the leak shape where a bypass-the-chokepoint removal lost the *Card.
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 1 {
		t.Fatalf("expected exactly 1 orphan swept, got %d", swept)
	}
	if _, ceased := gs.CeasedInstanceIDs[id]; !ceased {
		t.Fatalf("expected token ID %q to be ceased after sweep", id)
	}
	// Audit event must be emitted with the right shape.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind != "iid_orphan_sweep" {
			continue
		}
		if ev.Details["instance_id"] != id {
			continue
		}
		if ev.Details["provenance"] != "TK" {
			t.Fatalf("audit event provenance = %v, want TK", ev.Details["provenance"])
		}
		if ev.Details["card_name"] != "Treasure" {
			t.Fatalf("audit event card_name = %v, want Treasure", ev.Details["card_name"])
		}
		found = true
	}
	if !found {
		t.Fatal("expected iid_orphan_sweep audit event")
	}
}

// TestPhaseE_OrphanSweep_PresentIDNotSwept pins the negative path: when a
// minted card is still in a zone the invariant walks, the sweep MUST NOT
// cease it. Defends against the over-sweep regression where the sweep
// would silently retire live cards.
func TestPhaseE_OrphanSweep_PresentIDNotSwept(t *testing.T) {
	gs := newPhase4GameState(t)
	// One present in library, one orphaned.
	live := &Card{Name: "Mountain", Owner: 0, Colors: []string{"R"}}
	MintOGInstanceID(gs, live)
	gs.Seats[0].Library = append(gs.Seats[0].Library, live)
	orph := &Card{Name: "Forest", Owner: 0, Colors: []string{"G"}}
	MintOGInstanceID(gs, orph)
	// `orph` is intentionally NOT added to any zone.
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 1 {
		t.Fatalf("expected exactly 1 orphan swept, got %d", swept)
	}
	if _, ceased := gs.CeasedInstanceIDs[live.InstanceID]; ceased {
		t.Fatalf("live card %q must NOT be ceased by the sweep", live.InstanceID)
	}
	if _, ceased := gs.CeasedInstanceIDs[orph.InstanceID]; !ceased {
		t.Fatalf("orphaned card %q should be ceased", orph.InstanceID)
	}
}

// TestPhaseE_OrphanSweep_IdempotentReRun pins that re-running the sweep
// against an already-swept state is a no-op (no double-cease, no extra
// events). Important because StateBasedActions can be invoked many times
// per turn — each call must terminate cleanly when the state is stable.
func TestPhaseE_OrphanSweep_IdempotentReRun(t *testing.T) {
	gs := newPhase4GameState(t)
	tok := &Card{Name: "Clue", Owner: 0, Types: []string{"artifact", "token"}}
	MintTokenInstanceID(gs, tok, "", "")
	first := SweepOrphanedInstanceIDs(gs)
	if first != 1 {
		t.Fatalf("first sweep expected 1, got %d", first)
	}
	second := SweepOrphanedInstanceIDs(gs)
	if second != 0 {
		t.Fatalf("second sweep should be a no-op, got %d", second)
	}
	third := SweepOrphanedInstanceIDs(gs)
	if third != 0 {
		t.Fatalf("third sweep should be a no-op, got %d", third)
	}
}

// TestPhaseE_OrphanSweep_SkipsABProvenance pins that AbilityInstance-
// provenance IDs (encoded with "AB" at positions 2–3) are NOT swept.
// They're ephemeral by §603.10 and excluded from the (Minted - Ceased)
// expected set at invariants.go:282 — the sweep mirrors that filter.
func TestPhaseE_OrphanSweep_SkipsABProvenance(t *testing.T) {
	gs := newPhase4GameState(t)
	// Synthesize an AB-provenance ID directly into the mint map (bypassing
	// the typed AbilityInstance API to keep the test focused).
	abID := "h0ABVC000001"
	RecordMintedInstanceID(gs, abID)
	if _, ok := gs.MintedInstanceIDs[abID]; !ok {
		t.Fatalf("expected AB ID to be in MintedInstanceIDs")
	}
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 0 {
		t.Fatalf("AB provenance must be skipped, got %d swept", swept)
	}
	if _, ceased := gs.CeasedInstanceIDs[abID]; ceased {
		t.Fatalf("AB ID %q must not be ceased by the sweep", abID)
	}
}

// TestPhaseE_OrphanSweep_LeftGameSeatZonesIgnored pins that a card sitting
// in a LeftGame seat's zone is NOT counted as present (mirrors the
// invariant's s.LeftGame skip at invariants.go:184). If a card's only
// reference is in a LeftGame seat's zone, the sweep treats it as orphaned
// and ceases.
func TestPhaseE_OrphanSweep_LeftGameSeatZonesIgnored(t *testing.T) {
	gs := newPhase4GameState(t)
	stale := &Card{Name: "Lich", Owner: 0}
	MintOGInstanceID(gs, stale)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, stale)
	gs.Seats[0].LeftGame = true
	// HandleSeatElimination would normally have ceased this ID; simulate
	// the residual case where the cease was missed.
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 1 {
		t.Fatalf("expected 1 orphan swept from LeftGame seat, got %d", swept)
	}
	if _, ceased := gs.CeasedInstanceIDs[stale.InstanceID]; !ceased {
		t.Fatalf("LeftGame-seat card ID %q should be ceased", stale.InstanceID)
	}
}

// TestPhaseE_OrphanSweep_SidebandZoneCountsAsPresent pins that sideband
// zones the invariant walks (ForetellExile, Companion, ZoneCastGrants,
// MadnessExile, PlotExile, MayhemDiscards, ParadigmExile) count as zone
// presence for the sweep. Without this guarantee, every foretell / madness
// / plot card would be falsely swept.
func TestPhaseE_OrphanSweep_SidebandZoneCountsAsPresent(t *testing.T) {
	gs := newPhase4GameState(t)
	a := &Card{Name: "Foretold", Owner: 0}
	MintOGInstanceID(gs, a)
	gs.Seats[0].ForetellExile = append(gs.Seats[0].ForetellExile, a)
	b := &Card{Name: "Companion", Owner: 0}
	MintOGInstanceID(gs, b)
	gs.Seats[0].Companion = b
	c := &Card{Name: "Madness", Owner: 0}
	MintOGInstanceID(gs, c)
	if gs.MadnessExile == nil {
		gs.MadnessExile = map[*Card]*MadnessWindow{}
	}
	gs.MadnessExile[c] = &MadnessWindow{Seat: 0, Turn: 1}
	d := &Card{Name: "Plot", Owner: 0}
	MintOGInstanceID(gs, d)
	if gs.PlotExile == nil {
		gs.PlotExile = map[*Card]*PlotMeta{}
	}
	gs.PlotExile[d] = &PlotMeta{}
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 0 {
		t.Fatalf("sideband-zone cards should NOT be swept, got %d", swept)
	}
}

// TestPhaseE_OrphanSweep_MergedCardLineagePreserved pins that Mutate/Meld
// merged-card lineage (Permanent.MergedCardPtrs) counts as zone presence.
// The absorbed *Cards retain their InstanceIDs per CR §702.139c; the
// sweep must follow MergedCardPtrs to avoid false-ceasing them.
func TestPhaseE_OrphanSweep_MergedCardLineagePreserved(t *testing.T) {
	gs := newPhase4GameState(t)
	base := &Card{Name: "Mutate Base", Owner: 0, Types: []string{"creature"}}
	MintOGInstanceID(gs, base)
	absorbed := &Card{Name: "Mutate Top", Owner: 0, Types: []string{"creature"}}
	MintOGInstanceID(gs, absorbed)
	perm := &Permanent{Card: base, Owner: 0, Controller: 0, MergedCardPtrs: map[string]*Card{absorbed.InstanceID: absorbed}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 0 {
		t.Fatalf("merged-card lineage should be tracked as present, got %d swept", swept)
	}
	if _, ceased := gs.CeasedInstanceIDs[absorbed.InstanceID]; ceased {
		t.Fatalf("absorbed card ID %q must NOT be ceased", absorbed.InstanceID)
	}
}

// TestPhaseE_OrphanSweep_StackSpellItemCountsAsPresent pins that *Card
// pointers riding the stack as spell items (Source==nil, Kind not
// triggered/activated) count as present. Triggered/activated ability
// items reference their source Card as a log label, NOT zone occupancy —
// those are ignored by the sweep (mirrors invariants.go:263).
func TestPhaseE_OrphanSweep_StackSpellItemCountsAsPresent(t *testing.T) {
	gs := newPhase4GameState(t)
	spellCard := &Card{Name: "Counterspell", Owner: 0}
	MintOGInstanceID(gs, spellCard)
	// Spell item: Source==nil, Kind=="spell" by convention.
	gs.Stack = append(gs.Stack, &StackItem{Card: spellCard, Controller: 0, Kind: "spell"})
	if n := SweepOrphanedInstanceIDs(gs); n != 0 {
		t.Fatalf("spell-item stack card should be tracked as present, got %d swept", n)
	}

}

// TestPhaseE_OrphanSweep_StackAbilityItemDoesNotProtectOrphan pins that
// triggered/activated ability items on the stack reference their source
// Card as a log LABEL (per the StackItem.Card convention documented at
// stack.go's PushTriggeredAbility), NOT as zone occupancy. A *Card whose
// only reference is a Kind=="triggered" ability item is still orphaned.
// Mirrors invariants.go:263 which skips ability items from the present set.
func TestPhaseE_OrphanSweep_StackAbilityItemDoesNotProtectOrphan(t *testing.T) {
	gs := newPhase4GameState(t)
	abilityCard := &Card{Name: "Trigger Bearer", Owner: 0}
	MintOGInstanceID(gs, abilityCard)
	src := &Permanent{Card: abilityCard, Owner: 0, Controller: 0}
	// Ability item with non-nil Source (the convention for a triggered
	// ability whose log label points at the source card).
	gs.Stack = append(gs.Stack, &StackItem{Card: abilityCard, Source: src, Kind: "triggered"})
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 1 {
		t.Fatalf("ability-item stack card should be considered orphaned, got %d swept", swept)
	}
}

// TestPhaseE_OrphanSweep_NilSafety pins defensive nil / empty-state
// behavior — sweep must never panic and must return 0 swept when there's
// nothing to do.
func TestPhaseE_OrphanSweep_NilSafety(t *testing.T) {
	if n := SweepOrphanedInstanceIDs(nil); n != 0 {
		t.Fatalf("nil GameState: want 0, got %d", n)
	}
	gs := newPhase4GameState(t)
	if n := SweepOrphanedInstanceIDs(gs); n != 0 {
		t.Fatalf("empty Minted map: want 0, got %d", n)
	}
}

// TestPhaseE_OrphanSweep_AlreadyCeasedSkipped pins that an ID already in
// CeasedInstanceIDs is skipped — the sweep never re-emits an audit event
// for an ID that's already retired.
func TestPhaseE_OrphanSweep_AlreadyCeasedSkipped(t *testing.T) {
	gs := newPhase4GameState(t)
	c := &Card{Name: "Already Ceased", Owner: 0}
	MintOGInstanceID(gs, c)
	MarkInstanceIDCeased(gs, c.InstanceID)
	// Pre-sweep audit-event count.
	preCount := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "iid_orphan_sweep" {
			preCount++
		}
	}
	if n := SweepOrphanedInstanceIDs(gs); n != 0 {
		t.Fatalf("already-ceased ID should be skipped, got %d swept", n)
	}
	postCount := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "iid_orphan_sweep" {
			postCount++
		}
	}
	if postCount != preCount {
		t.Fatalf("no audit events expected for already-ceased ID (pre=%d post=%d)",
			preCount, postCount)
	}
}

// TestPhaseE_OrphanSweep_OGProvenanceCeased pins that OG (real-card)
// provenance is swept too, not just TK tokens. The residual disappearance
// class includes 28% OG-provenance hits (basic lands, commanders, real
// creatures) — those are zone-leak bugs the sweep formally retires while
// preserving evidence via the audit event for Phase F bug-hunting.
func TestPhaseE_OrphanSweep_OGProvenanceCeased(t *testing.T) {
	gs := newPhase4GameState(t)
	land := &Card{Name: "Mountain", Owner: 0, Colors: []string{"R"}}
	MintOGInstanceID(gs, land)
	id := land.InstanceID
	if !strings.Contains(id, "OG") {
		t.Fatalf("expected OG provenance in ID %q", id)
	}
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 1 {
		t.Fatalf("OG orphan should be swept, got %d", swept)
	}
	// Verify the audit event captures the OG provenance so Phase F bug-
	// hunting can replay the leak shape.
	foundOG := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "iid_orphan_sweep" && ev.Details["provenance"] == "OG" {
			foundOG = true
			break
		}
	}
	if !foundOG {
		t.Fatal("expected iid_orphan_sweep audit event with provenance=OG")
	}
}

// TestPhaseE_OrphanSweep_SatisfiesPostSBACensus is the integration-level
// pin: a state with an orphaned token ID that would trip
// checkZoneConservationByInstanceID (with strict-census enabled) becomes
// clean after the sweep runs. The sweep is the StateBasedActions tail
// hook; this test exercises that the post-sweep census passes.
func TestPhaseE_OrphanSweep_SatisfiesPostSBACensus(t *testing.T) {
	gs := newPhase4GameState(t)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["instanceid_strict_census"] = 1
	tok := &Card{Name: "Powerstone Token", Owner: 0, Types: []string{"artifact", "token"}}
	MintTokenInstanceID(gs, tok, "", "")
	// Pre-sweep: invariant flags the orphan.
	if err := checkZoneConservation(gs); err == nil {
		t.Fatal("expected invariant to flag the orphan pre-sweep")
	}
	SweepOrphanedInstanceIDs(gs)
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("expected clean census post-sweep, got: %v", err)
	}
}

// TestPhaseE_OrphanSweep_WiredIntoCleanupStep pins that the §514.2
// cleanup step invokes the sweep — without this, the chokepoint exists
// but never fires in production. Mid-turn placement (e.g., inside
// StateBasedActions) over-cessed because spells transitioning between
// stack and graveyard are briefly absent from every zone; cleanup-step
// is the most stable point in the turn cycle.
func TestPhaseE_OrphanSweep_WiredIntoCleanupStep(t *testing.T) {
	gs := newPhase4GameState(t)
	tok := &Card{Name: "Wired Test Token", Owner: 0, Types: []string{"artifact", "token"}}
	MintTokenInstanceID(gs, tok, "", "")
	id := tok.InstanceID
	ScanExpiredDurations(gs, "ending", "cleanup")
	if _, ceased := gs.CeasedInstanceIDs[id]; !ceased {
		t.Fatalf("cleanup-step sweep should have ceased orphan ID %q", id)
	}
}

// TestPhaseE_OrphanSweep_NotRunMidTurn pins that ScanExpiredDurations at
// a NON-cleanup step does NOT run the sweep. The cleanup-step gate is
// load-bearing — mid-turn invocation would cause the same over-cease
// regression that prompted the placement choice.
func TestPhaseE_OrphanSweep_NotRunMidTurn(t *testing.T) {
	gs := newPhase4GameState(t)
	tok := &Card{Name: "Mid-Turn Token", Owner: 0, Types: []string{"artifact", "token"}}
	MintTokenInstanceID(gs, tok, "", "")
	id := tok.InstanceID
	// Try every non-cleanup phase/step boundary that ScanExpiredDurations
	// handles. None should cease the orphan.
	for _, pair := range [][2]string{
		{"beginning", "untap"},
		{"beginning", "upkeep"},
		{"precombat_main", ""},
		{"combat", "declare_attackers"},
		{"postcombat_main", ""},
		{"ending", "end_of_turn"},
	} {
		ScanExpiredDurations(gs, pair[0], pair[1])
	}
	if _, ceased := gs.CeasedInstanceIDs[id]; ceased {
		t.Fatalf("orphan ID %q should NOT have been ceased mid-turn (any of: untap, upkeep, mains, combat, end_of_turn)", id)
	}
}
