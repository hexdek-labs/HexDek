package per_card

import (
	"testing"
)

func TestOlivia_PingMarksDamageAndAddsVampireType(t *testing.T) {
	gs := newGame(t, 2)
	olivia := addPerm(gs, 0, "Olivia Voldaren", "creature", "vampire")
	target := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")
	oliviaPingAndVampire(gs, olivia, map[string]interface{}{"target_perm": target})
	if target.MarkedDamage != 1 {
		t.Errorf("expected MarkedDamage 1, got %d", target.MarkedDamage)
	}
	if !cardHasType(target.Card, "vampire") {
		t.Errorf("expected target to have vampire type appended, got %v", target.Card.Types)
	}
	if olivia.Counters["+1/+1"] != 1 {
		t.Errorf("expected Olivia to have 1 +1/+1 counter, got %d", olivia.Counters["+1/+1"])
	}
}

func TestOlivia_DoesNotReAddVampireTypeIfAlreadyVampire(t *testing.T) {
	gs := newGame(t, 2)
	olivia := addPerm(gs, 0, "Olivia Voldaren", "creature", "vampire")
	target := addPerm(gs, 1, "Other Vampire", "creature", "vampire")
	preCount := 0
	for _, t := range target.Card.Types {
		if t == "vampire" {
			preCount++
		}
	}
	oliviaPingAndVampire(gs, olivia, map[string]interface{}{"target_perm": target})
	postCount := 0
	for _, t := range target.Card.Types {
		if t == "vampire" {
			postCount++
		}
	}
	if postCount != preCount {
		t.Errorf("vampire type should not be double-appended, pre=%d post=%d", preCount, postCount)
	}
}

func TestOlivia_RefusesSelfTarget(t *testing.T) {
	gs := newGame(t, 2)
	olivia := addPerm(gs, 0, "Olivia Voldaren", "creature", "vampire")
	oliviaPingAndVampire(gs, olivia, map[string]interface{}{"target_perm": olivia})
	if olivia.MarkedDamage != 0 {
		t.Errorf("Olivia must refuse to target herself (\"another target creature\")")
	}
	if olivia.Counters["+1/+1"] != 0 {
		t.Errorf("self-target should be rejected before counter add")
	}
}

func TestOlivia_RefusesNoncreatureTarget(t *testing.T) {
	gs := newGame(t, 2)
	olivia := addPerm(gs, 0, "Olivia Voldaren", "creature", "vampire")
	target := addPerm(gs, 1, "Mox Opal", "artifact")
	oliviaPingAndVampire(gs, olivia, map[string]interface{}{"target_perm": target})
	if target.MarkedDamage != 0 {
		t.Errorf("Olivia targets creatures only")
	}
}

func TestOlivia_ControlGrabEmitsPartial(t *testing.T) {
	gs := newGame(t, 2)
	olivia := addPerm(gs, 0, "Olivia Voldaren", "creature", "vampire")
	oliviaControlGrab(gs, olivia, nil)
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" {
			if d, ok := ev.Details["slug"].(string); ok && d == "olivia_grab_vampire" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_partial for the deferred control-grab")
	}
}
