package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/judge"
)

// Consolidation step 4 — every engine violation surface routes through
// judge.LogViolation as the canonical vocabulary, at origin.

func collectViolations(t *testing.T) (*[]judge.ValidationViolation, func()) {
	t.Helper()
	var got []judge.ValidationViolation
	unregister := judge.RegisterSink(func(v judge.ValidationViolation) {
		got = append(got, v)
	})
	return &got, unregister
}

func surfaces(vs []judge.ValidationViolation) map[string]int {
	m := map[string]int{}
	for _, v := range vs {
		m[v.Surface]++
	}
	return m
}

// TestRouter_InvariantsSurface — a fabricated InstanceID makes the
// census fail; RunAllInvariants must emit the canonical violation
// through the router with Surface=invariants.
func TestRouter_InvariantsSurface(t *testing.T) {
	got, done := collectViolations(t)
	defer done()

	gs := NewGameState(2, nil, nil)
	gs.MintedInstanceIDs = map[string]struct{}{"h0OG-real": {}}
	gs.CeasedInstanceIDs = map[string]struct{}{}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		&Card{Name: "Forged", Owner: 0, InstanceID: "h0OG-forged"})

	vs := RunAllInvariants(gs)
	if len(vs) == 0 {
		t.Fatal("fabricated InstanceID must produce an invariant violation")
	}
	if (*got)[0].Surface != judge.SurfaceInvariants {
		t.Errorf("router must see Surface=invariants; got %q", (*got)[0].Surface)
	}
	if vs[0].Surface != judge.SurfaceInvariants || vs[0].Severity != judge.SeverityCritical {
		t.Errorf("returned violations must be normalized canonical values: %+v", vs[0])
	}
}

// TestRouter_LegalitySurface — recording a legality violation routes
// the canonical view (Name=Rule, Seat, repro context) at origin.
func TestRouter_LegalitySurface(t *testing.T) {
	got, done := collectViolations(t)
	defer done()

	gs := NewGameState(2, nil, nil)
	lv := NewLegalityValidator(42)
	lv.record(gs, LegalityViolation{
		Turn: 7, Seat: 1, Action: "cast:Lightning Bolt", Rule: "601.2f", Detail: "underpaid by {1}",
	})

	if len(*got) != 1 {
		t.Fatalf("expected 1 routed violation, got %d", len(*got))
	}
	v := (*got)[0]
	if v.Surface != judge.SurfaceLegality || v.Name != "601.2f" || v.Seat != 1 {
		t.Errorf("canonical mapping wrong: %+v", v)
	}
	if v.Context["seed"] != int64(42) || v.Context["turn"] != 7 {
		t.Errorf("repro context must ride along: %+v", v.Context)
	}
}

// TestRouter_SeatOutcomeSurface — the seat-outcome checker routes its
// findings at origin.
func TestRouter_SeatOutcomeSurface(t *testing.T) {
	got, done := collectViolations(t)
	defer done()

	gs := NewGameState(2, nil, nil)
	gs.Turn = 9
	c := NewSeatOutcomeChecker()
	c.add(gs, SeatOutcomeViolation{Seat: 1, Kind: "loss_not_marked", Detail: "life -3 but not Lost", When: "sba"})

	if len(*got) != 1 {
		t.Fatalf("expected 1 routed violation, got %d", len(*got))
	}
	v := (*got)[0]
	if v.Surface != judge.SurfaceSeatOutcome || v.Name != "loss_not_marked" || v.Seat != 1 {
		t.Errorf("canonical mapping wrong: %+v", v)
	}
	if v.Context["when"] != "sba" || v.Context["turn"] != 9 {
		t.Errorf("context must carry when/turn: %+v", v.Context)
	}
}

// TestRouter_ConservationDimensionTag (Judge fold r63) — the strict
// census emits through the router tagged dimension=conservation; the
// other invariants default to state_integrity.
func TestRouter_ConservationDimensionTag(t *testing.T) {
	got, done := collectViolations(t)
	defer done()

	gs := NewGameState(2, nil, nil)
	gs.MintedInstanceIDs = map[string]struct{}{"h0OG-real": {}}
	gs.CeasedInstanceIDs = map[string]struct{}{}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		&Card{Name: "Forged", Owner: 0, InstanceID: "h0OG-forged"})

	RunAllInvariants(gs)

	foundConservation := false
	for _, v := range *got {
		if v.Name == "ZoneConservation" {
			if v.Dimension != judge.DimensionConservation {
				t.Errorf("ZoneConservation must carry dimension=conservation; got %q", v.Dimension)
			}
			foundConservation = true
		} else if v.Surface == judge.SurfaceInvariants && v.Dimension != judge.DimensionStateIntegrity && v.Dimension != judge.DimensionConservation {
			t.Errorf("invariant %q must default to state_integrity; got %q", v.Name, v.Dimension)
		}
	}
	if !foundConservation {
		t.Fatal("fabricated InstanceID must trip the conservation check")
	}
}
