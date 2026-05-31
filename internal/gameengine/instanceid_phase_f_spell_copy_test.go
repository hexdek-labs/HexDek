package gameengine

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// Phase F regression suite for the §400.7c duplicate-pointer fabrication
// class — the residual 52 Loki r60 seed-42 fabrications surfaced after
// Phase E's universal token cessation + orphan sweep. Root cause: 8
// per_card spell-copy handlers (Alania / Zada / Krark / Mica / Mendicant
// Core / Rootha / Kalamax / Ivy) used the anti-pattern
//
//	copyCard := source.DeepCopy()  // inherits source InstanceID
//	copyCard.IsCopy = true
//	... push to stack ...
//
// When the copy resolved, stack.go:1312 fired
// `MarkInstanceIDCeased(item.Card.InstanceID)`, ceasing the SOURCE's
// InstanceID because the copy shared the same ID. Every subsequent
// invariant tick then reported the source as fabricated. The fix routes
// all 8 sites (plus Riku, opportunistically simplified) through the new
// `MintSpellCopy` chokepoint which clears the inherited ID and mints a
// fresh CP-provenance ID with source lineage preserved.

func TestPhaseF_MintSpellCopy_ClearsInheritedInstanceID(t *testing.T) {
	gs := newPhase2GameState(t)
	src := &Card{Name: "Lightning Bolt", Owner: 1, Colors: []string{"R"}, CMC: 1}
	MintOGInstanceID(gs, src)
	if src.InstanceID == "" {
		t.Fatal("expected OG mint to stamp an InstanceID on source")
	}
	srcID := src.InstanceID

	cp := MintSpellCopy(gs, src)
	if cp == nil {
		t.Fatal("MintSpellCopy returned nil for non-nil source")
	}
	if cp == src {
		t.Fatal("MintSpellCopy returned the source pointer; must be a deep copy")
	}
	if cp.InstanceID == "" {
		t.Fatal("expected copy to carry a fresh InstanceID, got empty")
	}
	if cp.InstanceID == srcID {
		t.Fatalf("copy InstanceID %q must differ from source %q (the leak this test pins)",
			cp.InstanceID, srcID)
	}
	if !strings.Contains(cp.InstanceID, "CP") {
		t.Fatalf("expected CP-provenance ID, got %q", cp.InstanceID)
	}
	if cp.Provenance != instanceid.ProvCP {
		t.Fatalf("Provenance: want ProvCP, got %v", cp.Provenance)
	}
	if !cp.IsCopy {
		t.Fatal("expected IsCopy=true on the copy (CR §704.5e cessation gate)")
	}
	if cp.SourceInstanceID != srcID {
		t.Fatalf("SourceInstanceID: want %q (source lineage), got %q",
			srcID, cp.SourceInstanceID)
	}
}

func TestPhaseF_MintSpellCopy_NilSafe(t *testing.T) {
	if got := MintSpellCopy(nil, nil); got != nil {
		t.Fatalf("MintSpellCopy(nil, nil): want nil, got %+v", got)
	}
	gs := newPhase2GameState(t)
	if got := MintSpellCopy(gs, nil); got != nil {
		t.Fatalf("MintSpellCopy(gs, nil): want nil, got %+v", got)
	}
}

// TestPhaseF_SourceIDSurvivesCopyResolution is the end-to-end shape of
// the leak. Before the fix: a spell copy with the SAME InstanceID as the
// source ceased the source ID on resolution. After the fix: the copy
// has its own CP ID, ceasing it on resolution leaves the source intact.
//
// Drives the exact stack.go:1312 cease path: build a copy via
// MintSpellCopy, hand it to MarkInstanceIDCeased (mirroring the §707.10
// branch's call), then assert the source's ID is still in expected
// (Minted - Ceased).
func TestPhaseF_SourceIDSurvivesCopyResolution(t *testing.T) {
	gs := newPhase2GameState(t)
	src := &Card{Name: "Cryptic Command", Owner: 0, Colors: []string{"U"}, CMC: 4}
	MintOGInstanceID(gs, src)
	srcID := src.InstanceID

	cp := MintSpellCopy(gs, src)
	if cp == nil {
		t.Fatal("MintSpellCopy returned nil")
	}
	copyID := cp.InstanceID

	// Mirror stack.go's §707.10 cease-on-resolution branch.
	MarkInstanceIDCeased(gs, copyID)

	if _, ceased := gs.CeasedInstanceIDs[copyID]; !ceased {
		t.Fatal("copy InstanceID should be ceased after §707.10 path")
	}
	if _, ceased := gs.CeasedInstanceIDs[srcID]; ceased {
		t.Fatalf("source InstanceID %q must NOT be ceased after copy resolves "+
			"(this is the Phase F leak — if it fires, the helper is regressing)",
			srcID)
	}
	if _, minted := gs.MintedInstanceIDs[srcID]; !minted {
		t.Fatalf("source InstanceID %q must remain in MintedInstanceIDs", srcID)
	}
}

// TestPhaseF_MultipleCopies_DistinctIDs pins that Zada / Krark style
// "copy the spell N times" patterns produce N distinct InstanceIDs, so
// resolution of any single copy does not cease any sibling. Closes the
// shape where Zada's `for each other creature, push a copy` loop would
// have collided every copy's ID with the source.
func TestPhaseF_MultipleCopies_DistinctIDs(t *testing.T) {
	gs := newPhase2GameState(t)
	src := &Card{Name: "Expedite", Owner: 1, Colors: []string{"R"}, CMC: 1}
	MintOGInstanceID(gs, src)

	seen := map[string]bool{src.InstanceID: true}
	for i := 0; i < 5; i++ {
		cp := MintSpellCopy(gs, src)
		if cp == nil {
			t.Fatalf("copy %d: MintSpellCopy returned nil", i)
		}
		if cp.InstanceID == "" {
			t.Fatalf("copy %d: empty InstanceID", i)
		}
		if seen[cp.InstanceID] {
			t.Fatalf("copy %d: duplicate InstanceID %q (collides with prior mint)",
				i, cp.InstanceID)
		}
		seen[cp.InstanceID] = true
	}
}

// TestPhaseF_ZoneCensusCleanAfterCopyResolution drives the
// checkZoneConservation invariant end-to-end. Before the fix: the
// source-cease leak made every post-resolve invariant tick report the
// source as fabricated. After the fix: the census stays clean.
func TestPhaseF_ZoneCensusCleanAfterCopyResolution(t *testing.T) {
	gs := newPhase2GameState(t)
	// Ensure strict-census is on so disappearance is also validated.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["instanceid_strict_census"] = 1

	src := &Card{Name: "Twincast", Owner: 0, Colors: []string{"U"}, CMC: 4}
	MintOGInstanceID(gs, src)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, src)

	cp := MintSpellCopy(gs, src)
	if cp == nil {
		t.Fatal("MintSpellCopy returned nil")
	}
	// Census BEFORE copy resolution: source on hand, copy on stack — both
	// IDs should be minted-and-not-ceased and present in some zone.
	gs.Stack = append(gs.Stack, &StackItem{
		Controller: 0, Card: cp, Kind: "spell", IsCopy: true,
	})
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("census BEFORE resolve must be clean; got %v", err)
	}
	// §707.10 cease branch — copy ID retires, source ID must remain.
	gs.Stack = gs.Stack[:0]
	MarkInstanceIDCeased(gs, cp.InstanceID)
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("census AFTER resolve must be clean (Phase F leak fingerprint); got %v", err)
	}
}
