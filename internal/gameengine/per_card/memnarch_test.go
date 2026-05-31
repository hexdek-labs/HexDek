package per_card

import (
	"testing"
)

func TestMemnarch_AddsArtifactTypeToTarget(t *testing.T) {
	gs := newGame(t, 2)
	memnarch := addPerm(gs, 0, "Memnarch", "artifact", "creature")
	target := addPerm(gs, 1, "Llanowar Elves", "creature", "elf")
	memnarchActivate(gs, memnarch, 0, map[string]interface{}{"target_perm": target})
	if !cardHasType(target.Card, "artifact") {
		t.Errorf("expected target to have artifact type appended, got %v", target.Card.Types)
	}
	if !cardHasType(target.Card, "creature") {
		t.Errorf("original types must be preserved (\"in addition to\"), got %v", target.Card.Types)
	}
}

func TestMemnarch_DoesNotDoubleAppendArtifact(t *testing.T) {
	gs := newGame(t, 2)
	memnarch := addPerm(gs, 0, "Memnarch", "artifact", "creature")
	target := addPerm(gs, 0, "Sol Ring", "artifact")
	pre := 0
	for _, tp := range target.Card.Types {
		if tp == "artifact" {
			pre++
		}
	}
	memnarchActivate(gs, memnarch, 0, map[string]interface{}{"target_perm": target})
	post := 0
	for _, tp := range target.Card.Types {
		if tp == "artifact" {
			post++
		}
	}
	if post != pre {
		t.Errorf("must not double-append artifact, pre=%d post=%d", pre, post)
	}
}

func TestMemnarch_NoTargetEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	memnarch := addPerm(gs, 0, "Memnarch", "artifact", "creature")
	memnarchActivate(gs, memnarch, 0, map[string]interface{}{})
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_failed" {
			if d, ok := ev.Details["slug"].(string); ok && d == "memnarch_becomes_artifact" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_failed for no_target_in_ctx")
	}
}

func TestMemnarch_ControlGrabEmitsPartial(t *testing.T) {
	gs := newGame(t, 2)
	memnarch := addPerm(gs, 0, "Memnarch", "artifact", "creature")
	memnarchActivate(gs, memnarch, 1, nil)
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" {
			if d, ok := ev.Details["slug"].(string); ok && d == "memnarch_gain_control_artifact" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_partial for deferred control-grab")
	}
}
