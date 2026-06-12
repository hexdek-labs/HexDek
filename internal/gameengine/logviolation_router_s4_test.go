package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/validation"
)

// Consolidation step 4 — every engine violation surface routes through
// validation.LogViolation as the canonical vocabulary, at origin.

func collectViolations(t *testing.T) (*[]validation.ValidationViolation, func()) {
	t.Helper()
	var got []validation.ValidationViolation
	unregister := validation.RegisterSink(func(v validation.ValidationViolation) {
		got = append(got, v)
	})
	return &got, unregister
}

func surfaces(vs []validation.ValidationViolation) map[string]int {
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
	if (*got)[0].Surface != validation.SurfaceInvariants {
		t.Errorf("router must see Surface=invariants; got %q", (*got)[0].Surface)
	}
	if vs[0].Surface != validation.SurfaceInvariants || vs[0].Severity != validation.SeverityCritical {
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
	if v.Surface != validation.SurfaceLegality || v.Name != "601.2f" || v.Seat != 1 {
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
	if v.Surface != validation.SurfaceSeatOutcome || v.Name != "loss_not_marked" || v.Seat != 1 {
		t.Errorf("canonical mapping wrong: %+v", v)
	}
	if v.Context["when"] != "sba" || v.Context["turn"] != 9 {
		t.Errorf("context must carry when/turn: %+v", v.Context)
	}
}
