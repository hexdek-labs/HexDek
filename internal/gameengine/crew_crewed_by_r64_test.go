package gameengine

import "testing"

// TestCrew_CrewedByTracked_702_122c pins the Aug 7 2026 CR §702.122c change:
// a Vehicle is "crewed by" each creature tapped to pay its crew cost. Before
// this, crew tracked nothing (Mounts already tracked SaddlersThisTurn; crew
// was the one behind). Now CrewersThisTurn / CrewedByThisTurn record it.
func TestCrew_CrewedByTracked_702_122c(t *testing.T) {
	gs := newP0Game(t)
	v := crewVehicleN(gs, 0, "Smuggler's Copter", 2)
	a := addP0Battlefield(gs, 0, "Crewer A", 2, 2, "creature")
	b := addP0Battlefield(gs, 0, "Crewer B", 1, 1, "creature")

	if len(CrewedByThisTurn(v)) != 0 {
		t.Fatal("a Vehicle should have no recorded crewers before being crewed")
	}
	if err := CrewVehicle(gs, 0, v, []*Permanent{a, b}); err != nil {
		t.Fatalf("crew failed: %v", err)
	}

	crewers := CrewedByThisTurn(v)
	if len(crewers) != 2 {
		t.Fatalf("expected 2 crewers recorded (CR §702.122c), got %d", len(crewers))
	}
	seen := map[*Permanent]bool{}
	for _, c := range crewers {
		seen[c] = true
	}
	if !seen[a] || !seen[b] {
		t.Error("CrewedByThisTurn must hold the exact creatures tapped to crew")
	}
	if CrewedByThisTurn(nil) != nil {
		t.Error("CrewedByThisTurn(nil) should be nil")
	}
}
