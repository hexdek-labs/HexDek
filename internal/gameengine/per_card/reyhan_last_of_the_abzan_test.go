package per_card

import (
	"testing"
)

// Reyhan's enters-with-counters now resolves via the generic AST static
// path only (the per_card duplicate was deleted r63 — both ran and she
// entered with 6). The exactly-once pin is TestETBCounters_ExactlyPrinted
// in etb_counters_once_r63_test.go; the tests below seed the counters
// directly as setup.

func TestReyhan_TransfersCountersOnSameSideDeath(t *testing.T) {
	gs := newGame(t, 2)
	reyhan := addPerm(gs, 0, "Reyhan, Last of the Abzan", "creature")
	reyhan.AddCounter("+1/+1", 3)
	dying := addPerm(gs, 0, "Buffed Creature", "creature")
	dying.Counters["+1/+1"] = 4
	target := addPerm(gs, 0, "Big Threat", "creature")
	target.Counters["+1/+1"] = 2 // existing stack — Reyhan should prefer this concentrator

	reyhanCreatureDies(gs, reyhan, map[string]interface{}{"perm": dying})

	if target.Counters["+1/+1"] != 6 {
		t.Errorf("expected target +1/+1 = 6 (2 existing + 4 moved), got %d", target.Counters["+1/+1"])
	}
	if reyhan.Counters["+1/+1"] != 3 {
		t.Errorf("Reyhan should not get the counters when a better target exists, got %d", reyhan.Counters["+1/+1"])
	}
}

func TestReyhan_NoTransferOnUncountereDeath(t *testing.T) {
	gs := newGame(t, 2)
	reyhan := addPerm(gs, 0, "Reyhan, Last of the Abzan", "creature")
	reyhan.AddCounter("+1/+1", 3)
	dying := addPerm(gs, 0, "No Counters", "creature")
	// No +1/+1 counters on dying.
	target := addPerm(gs, 0, "Other", "creature")
	reyhanCreatureDies(gs, reyhan, map[string]interface{}{"perm": dying})
	if target.Counters["+1/+1"] != 0 {
		t.Errorf("no transfer when dying had no +1/+1 counters")
	}
}

func TestReyhan_NoTransferOnOppDeath(t *testing.T) {
	gs := newGame(t, 2)
	reyhan := addPerm(gs, 0, "Reyhan, Last of the Abzan", "creature")
	reyhan.AddCounter("+1/+1", 3)
	oppDying := addPerm(gs, 1, "Opp Creature", "creature")
	oppDying.Counters["+1/+1"] = 3
	target := addPerm(gs, 0, "My Creature", "creature")
	reyhanCreatureDies(gs, reyhan, map[string]interface{}{"perm": oppDying})
	if target.Counters["+1/+1"] != 0 {
		t.Errorf("Reyhan only fires on her controller's creature deaths")
	}
}

func TestReyhan_FallsBackToSelfWhenNoOtherTarget(t *testing.T) {
	gs := newGame(t, 2)
	reyhan := addPerm(gs, 0, "Reyhan, Last of the Abzan", "creature")
	reyhan.AddCounter("+1/+1", 3)
	dying := addPerm(gs, 0, "Buffed", "creature")
	dying.Counters["+1/+1"] = 2
	reyhanCreatureDies(gs, reyhan, map[string]interface{}{"perm": dying})
	if reyhan.Counters["+1/+1"] != 5 {
		t.Errorf("Reyhan should receive counters when no other valid target (3 + 2), got %d", reyhan.Counters["+1/+1"])
	}
}
