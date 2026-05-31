package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestGeist_AttackSpawnsTappedAttackingAngelToken(t *testing.T) {
	gs := newGame(t, 2)
	geist := addPerm(gs, 0, "Geist of Saint Traft", "creature")
	geistAttack(gs, geist, map[string]interface{}{
		"attacker_perm": geist,
		"defender_seat": 1,
	})
	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Angel Token" {
			token = p
		}
	}
	if token == nil {
		t.Fatal("expected Angel Token on battlefield")
	}
	if !token.Tapped {
		t.Errorf("Angel token must be tapped")
	}
	if token.Flags["attacking"] != 1 {
		t.Errorf("Angel token must be attacking")
	}
	if token.Flags["attacking_defender_seat"] != 2 {
		t.Errorf("expected attacking_defender_seat=2 (1+1), got %d", token.Flags["attacking_defender_seat"])
	}
	if token.SummoningSick {
		t.Errorf("Angel token must not be summoning-sick (entering tapped+attacking)")
	}
}

func TestGeist_AttackRegistersEOCDelayedExile(t *testing.T) {
	gs := newGame(t, 2)
	geist := addPerm(gs, 0, "Geist of Saint Traft", "creature")
	pre := len(gs.DelayedTriggers)
	geistAttack(gs, geist, map[string]interface{}{"attacker_perm": geist})
	if len(gs.DelayedTriggers) != pre+1 {
		t.Fatalf("expected 1 new delayed trigger, got %d", len(gs.DelayedTriggers)-pre)
	}
	dt := gs.DelayedTriggers[len(gs.DelayedTriggers)-1]
	if dt.TriggerAt != "end_of_combat" {
		t.Errorf("expected TriggerAt=end_of_combat, got %q", dt.TriggerAt)
	}
}

func TestGeist_AttackerFilterRequiresSelf(t *testing.T) {
	gs := newGame(t, 2)
	geist := addPerm(gs, 0, "Geist of Saint Traft", "creature")
	other := addPerm(gs, 0, "Other Creature", "creature")
	geistAttack(gs, geist, map[string]interface{}{"attacker_perm": other})
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Angel Token" {
			t.Errorf("Geist must not fire when another creature attacks")
		}
	}
}
