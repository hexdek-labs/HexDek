package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func crewVehicleN(gs *GameState, seat int, name string, n int) *Permanent {
	v := addP0Battlefield(gs, seat, name, 4, 4, "artifact", "vehicle")
	v.Card.AST = &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "crew", Args: []interface{}{float64(n)}}},
	}
	return v
}

// (d) A summoning-sick creature CAN be tapped to crew (CR §702.122c).
func TestCrew_SummoningSickCreatureCanCrew(t *testing.T) {
	gs := newP0Game(t)
	v := crewVehicleN(gs, 0, "Heart of Kiran", 1)
	sick := addP0Battlefield(gs, 0, "Hasty Bear", 2, 2, "creature")
	sick.SummoningSick = true

	if err := CrewVehicle(gs, 0, v, []*Permanent{sick}); err != nil {
		t.Fatalf("a summoning-sick creature must be allowed to crew: %v", err)
	}
	if !sick.Tapped {
		t.Fatal("the crewing creature should be tapped")
	}
	if !v.IsCreature() {
		t.Fatal("vehicle should become a creature")
	}
	// (f) the Vehicle itself must NOT be tapped by crewing.
	if v.Tapped {
		t.Fatal("crewing must not tap the Vehicle")
	}
}

// (g) Crew 1 is satisfied by a single power-1 creature; a 0-power creature
// contributes 0 and cannot pay crew 1 alone.
func TestCrew_PowerOneAndZero(t *testing.T) {
	gs := newP0Game(t)
	v1 := crewVehicleN(gs, 0, "Vehicle One", 1)
	one := addP0Battlefield(gs, 0, "1-power", 1, 1, "creature")
	if err := CrewVehicle(gs, 0, v1, []*Permanent{one}); err != nil {
		t.Fatalf("a power-1 creature should pay crew 1: %v", err)
	}

	v2 := crewVehicleN(gs, 0, "Vehicle Two", 1)
	zero := addP0Battlefield(gs, 0, "0-power", 0, 0, "creature")
	if err := CrewVehicle(gs, 0, v2, []*Permanent{zero}); err == nil {
		t.Fatal("a 0-power creature contributes 0 and must not pay crew 1")
	}
}

// (e) A Vehicle that is already a creature can still be crewed (legal).
func TestCrew_AlreadyCreatureVehicleCanBeCrewed(t *testing.T) {
	gs := newP0Game(t)
	v := crewVehicleN(gs, 0, "Already Creature", 1)
	v.Card.Types = append(v.Card.Types, "creature") // already an artifact creature
	crewer := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")

	if err := CrewVehicle(gs, 0, v, []*Permanent{crewer}); err != nil {
		t.Fatalf("crewing an already-creature Vehicle should be legal: %v", err)
	}
	if !crewer.Tapped {
		t.Fatal("the crewer should be tapped even when the Vehicle was already a creature")
	}
}

// (h) Crewing fires the "crew" trigger ("whenever you crew" / "becomes crewed").
func TestCrew_FiresCrewTrigger(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "crew" {
			fired++
		}
	}
	gs := newP0Game(t)
	v := crewVehicleN(gs, 0, "Heart of Kiran", 1)
	c := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")
	if err := CrewVehicle(gs, 0, v, []*Permanent{c}); err != nil {
		t.Fatalf("crew should succeed: %v", err)
	}
	if fired != 1 {
		t.Fatalf("crewing should fire the 'crew' trigger once, fired %d", fired)
	}
}

// (b) "until end of turn" — a crewed Vehicle reverts to a non-creature at the
// cleanup step (the revert was never wired in).
func TestCrew_RevertsToNonCreatureAtCleanup(t *testing.T) {
	gs := newP0Game(t)
	v := crewVehicleN(gs, 0, "Heart of Kiran", 1)
	c := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")
	if err := CrewVehicle(gs, 0, v, []*Permanent{c}); err != nil {
		t.Fatalf("crew should succeed: %v", err)
	}
	if !v.IsCreature() {
		t.Fatal("vehicle should be a creature while crewed")
	}

	// Cleanup-step expiry (§514.2): the crewed-until-EOT animation wears off.
	ScanExpiredDurations(gs, "ending", "cleanup")

	if v.IsCreature() {
		t.Fatal("a crewed Vehicle must stop being a creature at end of turn (CR §702.122a)")
	}
	if v.Flags["crewed_until_eot"] != 0 {
		t.Fatalf("crewed_until_eot flag should be cleared, got %d", v.Flags["crewed_until_eot"])
	}
}
