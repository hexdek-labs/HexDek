package per_card

import (
	"testing"
)

func TestMarisi_ETBStampsNoCombatCastFlag(t *testing.T) {
	gs := newGame(t, 2)
	marisi := addPerm(gs, 0, "Marisi, Breaker of the Coil", "creature")
	marisiETB(gs, marisi)
	if gs.Flags["marisi_no_combat_cast_seat_0"] == 0 {
		t.Errorf("expected marisi_no_combat_cast_seat_0 flag set after ETB")
	}
}

func TestMarisi_LTBClearsFlag(t *testing.T) {
	gs := newGame(t, 2)
	marisi := addPerm(gs, 0, "Marisi, Breaker of the Coil", "creature")
	marisiETB(gs, marisi)
	marisiLTB(gs, marisi, map[string]interface{}{"perm": marisi})
	if _, ok := gs.Flags["marisi_no_combat_cast_seat_0"]; ok {
		t.Errorf("expected flag cleared after Marisi LTB")
	}
}

func TestMarisi_CombatDamageGoadsDefenderCreatures(t *testing.T) {
	gs := newGame(t, 2)
	marisi := addPerm(gs, 0, "Marisi, Breaker of the Coil", "creature")
	oppCreature := addPerm(gs, 1, "Opp Bear", "creature")
	oppOther := addPerm(gs, 1, "Opp Wolf", "creature")
	oppArtifact := addPerm(gs, 1, "Opp Artifact", "artifact")

	marisiCombatDamage(gs, marisi, map[string]interface{}{
		"source_seat":   0,
		"defender_seat": 1,
		"amount":        2,
	})

	if oppCreature.Flags["goaded"] != 1 {
		t.Errorf("expected Opp Bear goaded")
	}
	if oppOther.Flags["goaded"] != 1 {
		t.Errorf("expected Opp Wolf goaded")
	}
	if oppArtifact.Flags["goaded"] == 1 {
		t.Errorf("non-creature should not be goaded")
	}
}

func TestMarisi_DoesNotFireOnOpponentDamage(t *testing.T) {
	gs := newGame(t, 2)
	marisi := addPerm(gs, 0, "Marisi, Breaker of the Coil", "creature")
	oppCreature := addPerm(gs, 1, "Opp Bear", "creature")

	marisiCombatDamage(gs, marisi, map[string]interface{}{
		"source_seat":   1, // opp attacked
		"defender_seat": 0,
	})

	if oppCreature.Flags["goaded"] == 1 {
		t.Errorf("Marisi must only fire on her controller's damage")
	}
}
