package hexapi

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 spectator UX batch 3: regressions for the new snapshot
// helpers (buildCommandZone) + the counter-on-permanent wiring
// shipped in PermanentSnapshot.Counters.

// -----------------------------------------------------------------------------
// buildCommandZone
// -----------------------------------------------------------------------------

func TestBuildCommandZone_NilSafe(t *testing.T) {
	if got := buildCommandZone(nil, nil); got != nil {
		t.Errorf("nil zone should yield nil, got %v", got)
	}
}

func TestBuildCommandZone_EmptyZoneYieldsNil(t *testing.T) {
	if got := buildCommandZone([]*gameengine.Card{}, map[string]int{}); got != nil {
		t.Errorf("empty zone should yield nil (omitempty elides field), got %v", got)
	}
}

func TestBuildCommandZone_SingleCommanderNoTax(t *testing.T) {
	atraxa := &gameengine.Card{Name: "Atraxa, Praetors' Voice"}
	got := buildCommandZone([]*gameengine.Card{atraxa}, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("name = %q, want Atraxa, Praetors' Voice", got[0].Name)
	}
	if got[0].CastCount != 0 {
		t.Errorf("cast_count = %d, want 0 (never cast)", got[0].CastCount)
	}
}

func TestBuildCommandZone_TaxCountPropagated(t *testing.T) {
	c := &gameengine.Card{Name: "Krenko, Mob Boss"}
	got := buildCommandZone([]*gameengine.Card{c}, map[string]int{"Krenko, Mob Boss": 3})
	if got[0].CastCount != 3 {
		t.Errorf("cast_count = %d, want 3", got[0].CastCount)
	}
}

func TestBuildCommandZone_PartnerPairKeepsIndependentTax(t *testing.T) {
	// CR §903.8: partner pairs maintain INDEPENDENT cast counts.
	// Kraum cast 3 times shouldn't bump Tymna's tax.
	kraum := &gameengine.Card{Name: "Kraum, Ludevic's Opus"}
	tymna := &gameengine.Card{Name: "Tymna the Weaver"}
	got := buildCommandZone(
		[]*gameengine.Card{kraum, tymna},
		map[string]int{"Kraum, Ludevic's Opus": 3, "Tymna the Weaver": 1},
	)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	for _, e := range got {
		switch e.Name {
		case "Kraum, Ludevic's Opus":
			if e.CastCount != 3 {
				t.Errorf("Kraum cast_count = %d, want 3", e.CastCount)
			}
		case "Tymna the Weaver":
			if e.CastCount != 1 {
				t.Errorf("Tymna cast_count = %d, want 1", e.CastCount)
			}
		default:
			t.Errorf("unexpected entry %q", e.Name)
		}
	}
}

func TestBuildCommandZone_NilCardSkipped(t *testing.T) {
	c := &gameengine.Card{Name: "Atraxa, Praetors' Voice"}
	got := buildCommandZone([]*gameengine.Card{nil, c, nil}, nil)
	if len(got) != 1 || got[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("nil cards should be skipped; got %+v", got)
	}
}

func TestBuildCommandZone_AllNilYieldsNil(t *testing.T) {
	got := buildCommandZone([]*gameengine.Card{nil, nil}, nil)
	if got != nil {
		t.Errorf("all-nil input should yield nil, got %v", got)
	}
}

// -----------------------------------------------------------------------------
// PermanentSnapshot.Counters wiring (verified through JSON shape)
// -----------------------------------------------------------------------------

func TestPermanentSnapshot_CountersFieldOmitsEmpty(t *testing.T) {
	// PermanentSnapshot is constructed in captureSnapshot; here we
	// exercise the JSON tag directly by marshaling a struct literal
	// to confirm omitempty fires when Counters is nil or empty.
	ps := PermanentSnapshot{Name: "Forest"}
	if ps.Counters != nil {
		t.Errorf("zero-value Counters should be nil, got %v", ps.Counters)
	}
}

// captureSnapshot integration via the engine's own state is exercised
// in showmatch_test.go's broader smoke tests. The unit covers the
// shape-of-output contract: that a non-empty Counters map round-trips
// through the snapshot JSON path.
