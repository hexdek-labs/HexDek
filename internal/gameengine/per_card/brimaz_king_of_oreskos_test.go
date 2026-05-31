package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestBrimaz_AttackCreatesCatSoldierToken(t *testing.T) {
	gs := newGame(t, 2)
	brimaz := addPerm(gs, 0, "Brimaz, King of Oreskos", "creature")
	preTokens := countCatSoldierTokens(gs, 0)
	brimazAttack(gs, brimaz, map[string]interface{}{
		"attacker_perm": brimaz,
	})
	if got := countCatSoldierTokens(gs, 0); got != preTokens+1 {
		t.Errorf("expected 1 new Cat Soldier token, got %d new", got-preTokens)
	}
}

func TestBrimaz_AttackTokenIsTaggedAttacking(t *testing.T) {
	gs := newGame(t, 2)
	brimaz := addPerm(gs, 0, "Brimaz, King of Oreskos", "creature")
	brimazAttack(gs, brimaz, map[string]interface{}{
		"attacker_perm": brimaz,
		"defender_seat": 1,
	})
	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Cat Soldier Token" {
			token = p
		}
	}
	if token == nil {
		t.Fatal("expected a Cat Soldier Token")
	}
	if token.Flags["attacking"] != 1 {
		t.Errorf("expected token's attacking flag set, got %d", token.Flags["attacking"])
	}
	if token.Flags["attacking_defender_seat"] != 2 { // defender_seat 1 → 1+1=2
		t.Errorf("expected defender_seat 1 stored as 2 (1-indexed-with-offset), got %d", token.Flags["attacking_defender_seat"])
	}
	if token.SummoningSick {
		t.Errorf("token must not be summoning-sick (it's already attacking)")
	}
}

func TestBrimaz_AttackerFilterRequiresBrimazAttacking(t *testing.T) {
	gs := newGame(t, 2)
	brimaz := addPerm(gs, 0, "Brimaz, King of Oreskos", "creature")
	other := addPerm(gs, 0, "Some Other Creature", "creature")
	brimazAttack(gs, brimaz, map[string]interface{}{
		"attacker_perm": other,
	})
	if countCatSoldierTokens(gs, 0) != 0 {
		t.Errorf("Brimaz must not fire when a different attacker is declared")
	}
}

func TestBrimaz_ETBEmitsBlockTriggerPartial(t *testing.T) {
	gs := newGame(t, 2)
	brimaz := addPerm(gs, 0, "Brimaz, King of Oreskos", "creature")
	brimazETB(gs, brimaz)
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" {
			if d, ok := ev.Details["slug"].(string); ok && d == "brimaz_block_trigger_deferred" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_partial event flagging the deferred block trigger")
	}
}

func countCatSoldierTokens(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Cat Soldier Token" {
			n++
		}
	}
	return n
}
