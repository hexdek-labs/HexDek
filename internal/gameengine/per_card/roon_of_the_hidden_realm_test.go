package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestRoon_ActivateExilesTargetAndSchedulesReturn(t *testing.T) {
	gs := newGame(t, 2)
	roon := addPerm(gs, 0, "Roon of the Hidden Realm", "creature")
	target := addPerm(gs, 0, "Mulldrifter", "creature")
	preDT := len(gs.DelayedTriggers)
	roonActivate(gs, roon, 0, map[string]interface{}{"target_perm": target})

	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == target {
			stillOnBF = true
		}
	}
	if stillOnBF {
		t.Errorf("target should be off the battlefield after exile")
	}
	if len(gs.DelayedTriggers) != preDT+1 {
		t.Fatalf("expected 1 new delayed trigger, got %d", len(gs.DelayedTriggers)-preDT)
	}
	dt := gs.DelayedTriggers[len(gs.DelayedTriggers)-1]
	if dt.TriggerAt != "next_end_step" {
		t.Errorf("expected TriggerAt=next_end_step, got %q", dt.TriggerAt)
	}
}

func TestRoon_RefusesSelfTarget(t *testing.T) {
	gs := newGame(t, 2)
	roon := addPerm(gs, 0, "Roon of the Hidden Realm", "creature")
	pre := len(gs.DelayedTriggers)
	roonActivate(gs, roon, 0, map[string]interface{}{"target_perm": roon})
	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == roon {
			stillOnBF = true
		}
	}
	if !stillOnBF {
		t.Errorf("Roon must refuse to target himself (\"another target creature\")")
	}
	if len(gs.DelayedTriggers) != pre {
		t.Errorf("self-target rejection should not schedule a return")
	}
}

func TestRoon_RefusesNoncreatureTarget(t *testing.T) {
	gs := newGame(t, 2)
	roon := addPerm(gs, 0, "Roon of the Hidden Realm", "creature")
	artifact := addPerm(gs, 0, "Mox Opal", "artifact")
	pre := len(gs.DelayedTriggers)
	roonActivate(gs, roon, 0, map[string]interface{}{"target_perm": artifact})
	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == artifact {
			stillOnBF = true
		}
	}
	if !stillOnBF {
		t.Errorf("Roon must refuse noncreature target")
	}
	if len(gs.DelayedTriggers) != pre {
		t.Errorf("noncreature rejection should not schedule a return")
	}
}

func TestRoon_DelayedTriggerReturnsTargetUnderOwnerControl(t *testing.T) {
	gs := newGame(t, 2)
	roon := addPerm(gs, 0, "Roon of the Hidden Realm", "creature")
	// Opponent's creature with cmc tag for CardCanEnterBattlefield satisfaction.
	target := addPerm(gs, 1, "Opp Mulldrifter", "creature")
	roonActivate(gs, roon, 0, map[string]interface{}{"target_perm": target})

	dt := gs.DelayedTriggers[len(gs.DelayedTriggers)-1]
	dt.EffectFn(gs)

	foundUnderOwner := false
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Opp Mulldrifter" {
			foundUnderOwner = true
		}
	}
	if !foundUnderOwner {
		t.Errorf("target should return under owner's (seat 1) control, got owner battlefield: %v",
			permNames(gs.Seats[1].Battlefield))
	}
}

func TestRoon_NoTargetEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	roon := addPerm(gs, 0, "Roon of the Hidden Realm", "creature")
	roonActivate(gs, roon, 0, map[string]interface{}{})
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_failed" {
			if d, ok := ev.Details["slug"].(string); ok && d == "roon_exile_and_return" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_failed for no_target_in_ctx")
	}
}

// Suppress unused-import lint on gameengine.
var _ = gameengine.GameState{}
