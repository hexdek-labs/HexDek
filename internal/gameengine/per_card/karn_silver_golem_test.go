package per_card

import (
	"testing"
)

func TestKarn_AnimatesNoncreatureArtifact(t *testing.T) {
	gs := newGame(t, 2)
	karn := addPerm(gs, 0, "Karn, Silver Golem", "artifact", "creature")
	mox := addPerm(gs, 0, "Mox Opal", "artifact", "cmc:0")
	karnActivate(gs, karn, 0, map[string]interface{}{"target_perm": mox})
	if !cardHasType(mox.Card, "creature") {
		t.Errorf("expected Mox Opal to become a creature, types=%v", mox.Card.Types)
	}
	// CMC 0 → power/toughness 0; Modifications carries the delta.
	if len(mox.Modifications) != 1 {
		t.Errorf("expected 1 Modification on Mox Opal, got %d", len(mox.Modifications))
	}
}

func TestKarn_SetsPowerToughnessFromManaValue(t *testing.T) {
	gs := newGame(t, 2)
	karn := addPerm(gs, 0, "Karn, Silver Golem", "artifact", "creature")
	wurm := addPerm(gs, 0, "Wurmcoil Engine", "artifact", "cmc:6")
	karnActivate(gs, karn, 0, map[string]interface{}{"target_perm": wurm})
	if !cardHasType(wurm.Card, "creature") {
		t.Errorf("expected animation, got types=%v", wurm.Card.Types)
	}
	m := wurm.Modifications[len(wurm.Modifications)-1]
	if m.Power != 6 || m.Toughness != 6 {
		t.Errorf("expected +6/+6 modification (CMC 6), got +%d/+%d", m.Power, m.Toughness)
	}
	if m.Duration != "until_end_of_turn" {
		t.Errorf("expected UEOT duration, got %q", m.Duration)
	}
}

func TestKarn_RefusesNonArtifactTarget(t *testing.T) {
	gs := newGame(t, 2)
	karn := addPerm(gs, 0, "Karn, Silver Golem", "artifact", "creature")
	land := addPerm(gs, 0, "Forest", "land")
	karnActivate(gs, karn, 0, map[string]interface{}{"target_perm": land})
	if cardHasType(land.Card, "creature") {
		t.Errorf("Karn must not animate non-artifacts")
	}
}

func TestKarn_RefusesAlreadyCreatureArtifact(t *testing.T) {
	gs := newGame(t, 2)
	karn := addPerm(gs, 0, "Karn, Silver Golem", "artifact", "creature")
	// Already an artifact creature.
	wurm := addPerm(gs, 0, "Steel Hellkite", "artifact", "creature", "cmc:6")
	pre := len(wurm.Modifications)
	karnActivate(gs, karn, 0, map[string]interface{}{"target_perm": wurm})
	if len(wurm.Modifications) != pre {
		t.Errorf("Karn must not re-animate already-creature artifacts")
	}
}

func TestKarn_NoTargetEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	karn := addPerm(gs, 0, "Karn, Silver Golem", "artifact", "creature")
	karnActivate(gs, karn, 0, map[string]interface{}{})
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_failed" {
			if d, ok := ev.Details["slug"].(string); ok && d == "karn_animate_artifact" {
				if r, ok := ev.Details["reason"].(string); ok && r == "no_target_in_ctx" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_failed event for no_target_in_ctx")
	}
}
