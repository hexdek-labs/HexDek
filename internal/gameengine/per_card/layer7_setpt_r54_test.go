package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// dev/layer7-setpt-r54 — Layer 7b set-PT primitive + 5 ports.
// Verifies the becomes-N/N effect path produces the right effective
// characteristics under GetEffectiveCharacteristics (CR §613).
// ---------------------------------------------------------------------------

// 1. Great Hall of the Biblioplex — {5} animates to 2/4 Wizard creature
//    (still a land).
func TestLayer7SetPT_GreatHallBiblioplex_Animate24Wizard(t *testing.T) {
	gs := newGame(t, 2)
	hall := addPerm(gs, 0, "Great Hall of the Biblioplex", "land")
	gs.Seats[0].ManaPool = 5

	gameengine.InvokeActivatedHook(gs, hall, 2, nil)

	chars := gameengine.GetEffectiveCharacteristics(gs, hall)
	if chars.Power != 2 || chars.Toughness != 4 {
		t.Errorf("expected 2/4 after animate, got %d/%d", chars.Power, chars.Toughness)
	}
	if !gs.HasTypeOf(hall, "creature") {
		t.Errorf("expected creature type after animate, got types=%v", chars.Types)
	}
	// Engine convention: subtypes share the Types slice (no separate
	// Subtypes population path for runtime-added subtypes today).
	if !gs.HasTypeOf(hall, "wizard") {
		t.Errorf("expected wizard subtype in Types, got types=%v", chars.Types)
	}
	if !gs.HasTypeOf(hall, "land") {
		t.Errorf("expected land type retained (printed 'still a land'), got %v", chars.Types)
	}
}

// 2. Toph, Earthbending Master — earthbend on attack sets the chosen
//    land to 0/0 + creature type + haste. With X=2 experience, the
//    final stats read 2/2 (0/0 base + 2× +1/+1 counters via §613.4c).
func TestLayer7SetPT_TophEarthbendingMaster_LandBecomesXX(t *testing.T) {
	gs := newGame(t, 2)
	toph := addPerm(gs, 0, "Toph, Earthbending Master", "creature", "legendary")
	land := addPerm(gs, 0, "Mountain", "land", "basic", "mountain")
	gs.Seats[0].Flags = map[string]int{"experience": 2}

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"seat": 0,
	})
	_ = toph

	chars := gameengine.GetEffectiveCharacteristics(gs, land)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("expected 2/2 (0/0 + 2× +1/+1), got %d/%d", chars.Power, chars.Toughness)
	}
	if !gs.HasTypeOf(land, "creature") {
		t.Errorf("expected creature type added, got %v", chars.Types)
	}
	if !gs.HasKeywordOf(land, "haste") {
		t.Errorf("expected haste, got keywords=%v", chars.Keywords)
	}
	if !gs.HasTypeOf(land, "land") {
		t.Errorf("expected land type retained, got %v", chars.Types)
	}
}

// 3. Toph, the First Metalbender — end_step earthbend 2 sets a friendly
//    land to 0/0 creature with haste + 2× +1/+1 → reads 2/2.
func TestLayer7SetPT_TophFirstMetalbender_EarthbendsLand(t *testing.T) {
	gs := newGame(t, 2)
	toph := addPerm(gs, 0, "Toph, the First Metalbender", "creature", "legendary")
	land := addPerm(gs, 0, "Plains", "land", "basic", "plains")

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})
	_ = toph

	chars := gameengine.GetEffectiveCharacteristics(gs, land)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("expected 2/2 (0/0 + 2× +1/+1), got %d/%d", chars.Power, chars.Toughness)
	}
	if !gs.HasTypeOf(land, "creature") {
		t.Errorf("expected creature type added, got %v", chars.Types)
	}
	if !gs.HasKeywordOf(land, "haste") {
		t.Errorf("expected haste, got %v", chars.Keywords)
	}
}

// 4. Toph, Hardheaded Teacher — spell_cast triggers earthbend 1; Lesson
//    bonus adds another +1/+1. Final reads 2/2 (0/0 + 1 + 1 Lesson).
func TestLayer7SetPT_TophHardheaded_LessonBonus(t *testing.T) {
	gs := newGame(t, 2)
	toph := addPerm(gs, 0, "Toph, Hardheaded Teacher", "creature", "legendary")
	land := addPerm(gs, 0, "Forest", "land", "basic", "forest")
	_ = toph

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Test Lesson", Types: []string{"sorcery", "lesson"}},
	})

	chars := gameengine.GetEffectiveCharacteristics(gs, land)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("expected 2/2 (0/0 + 1 earthbend + 1 lesson), got %d/%d", chars.Power, chars.Toughness)
	}
	if !gs.HasTypeOf(land, "creature") {
		t.Errorf("expected creature type, got %v", chars.Types)
	}
}

// 5. Kolodin, Triumph Caster — Vehicle ETB stamps Layer 4 creature+artifact
//    until end of turn; effective characteristics include creature type.
func TestLayer7SetPT_Kolodin_VehicleBecomesCreatureUEOT(t *testing.T) {
	gs := newGame(t, 2)
	kolodin := addPerm(gs, 0, "Kolodin, Triumph Caster", "creature", "legendary")
	kolodin.Card.BasePower = 4
	kolodin.Card.BaseToughness = 4
	vehicle := addPerm(gs, 0, "Test Truck", "artifact", "vehicle")
	vehicle.Card.BasePower = 4
	vehicle.Card.BaseToughness = 3

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            vehicle,
		"controller_seat": 0,
	})

	chars := gameengine.GetEffectiveCharacteristics(gs, vehicle)
	if !gs.HasTypeOf(vehicle, "creature") {
		t.Errorf("expected vehicle to be a creature after Kolodin trigger, got %v", chars.Types)
	}
	// Cleanup at end of turn should remove the Layer 4 effect.
	gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
	chars = gameengine.GetEffectiveCharacteristics(gs, vehicle)
	if gs.HasTypeOf(vehicle, "creature") {
		// Vehicle's printed types are artifact + vehicle (no creature).
		// If the AST keyword pipeline doesn't add creature back, this should fail.
		// Some test fixtures default to no AST → creature add only via the L4 effect.
		// If the card was registered with "creature" in printed types, the assert is OK.
		// Here we manually verified the Card.Types doesn't include "creature".
		hasPrintedCreature := false
		for _, tt := range vehicle.Card.Types {
			if tt == "creature" {
				hasPrintedCreature = true
			}
		}
		if !hasPrintedCreature {
			t.Errorf("expected creature type cleared after cleanup, got %v", chars.Types)
		}
	}
}
