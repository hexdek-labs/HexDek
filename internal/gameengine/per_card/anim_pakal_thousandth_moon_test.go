package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestAnimPakal_FirstAttackSpawnsOneGnome(t *testing.T) {
	gs := newGame(t, 2)
	anim := addPerm(gs, 0, "Anim Pakal, Thousandth Moon", "creature")
	attacker := addPerm(gs, 0, "Some Creature", "creature")
	setAttacking(attacker)

	animPakalAttackers(gs, anim, map[string]interface{}{})

	if anim.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1 counter on Anim Pakal after first attack, got %d", anim.Counters["+1/+1"])
	}
	if got := countGnomeTokens(gs, 0); got != 1 {
		t.Errorf("expected 1 Gnome spawned (X=counter total after bump), got %d", got)
	}
}

func TestAnimPakal_SecondAttackSpawnsTwoGnomes(t *testing.T) {
	gs := newGame(t, 2)
	anim := addPerm(gs, 0, "Anim Pakal, Thousandth Moon", "creature")
	anim.Counters["+1/+1"] = 1 // already attacked once
	attacker := addPerm(gs, 0, "Some Creature", "creature")
	setAttacking(attacker)

	animPakalAttackers(gs, anim, map[string]interface{}{})

	if anim.Counters["+1/+1"] != 2 {
		t.Errorf("expected counter total = 2, got %d", anim.Counters["+1/+1"])
	}
	if got := countGnomeTokens(gs, 0); got != 2 {
		t.Errorf("expected 2 Gnomes (X=2 after bump), got %d", got)
	}
}

func TestAnimPakal_NoFireWithOnlyGnomeAttackers(t *testing.T) {
	gs := newGame(t, 2)
	anim := addPerm(gs, 0, "Anim Pakal, Thousandth Moon", "creature")
	gnome := addPerm(gs, 0, "Existing Gnome", "creature", "gnome")
	setAttacking(gnome)

	animPakalAttackers(gs, anim, map[string]interface{}{})

	if anim.Counters["+1/+1"] != 0 {
		t.Errorf("Anim Pakal must not pump if only Gnomes are attacking (\"non-Gnome creatures\")")
	}
	if got := countGnomeTokens(gs, 0); got != 0 {
		t.Errorf("no spawn expected when only Gnomes attack")
	}
}

func TestAnimPakal_NoFireWithNoAttackers(t *testing.T) {
	gs := newGame(t, 2)
	anim := addPerm(gs, 0, "Anim Pakal, Thousandth Moon", "creature")
	animPakalAttackers(gs, anim, map[string]interface{}{})
	if anim.Counters["+1/+1"] != 0 {
		t.Errorf("no pump expected with zero attackers")
	}
}

func TestAnimPakal_SpawnedGnomesAreTappedAttacking(t *testing.T) {
	gs := newGame(t, 2)
	anim := addPerm(gs, 0, "Anim Pakal, Thousandth Moon", "creature")
	attacker := addPerm(gs, 0, "Some Creature", "creature")
	setAttacking(attacker)
	animPakalAttackers(gs, anim, map[string]interface{}{"defender_seat": 1})
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Gnome Token" {
			if !p.Tapped {
				t.Errorf("Gnome Token must be tapped")
			}
			if p.Flags["attacking"] != 1 {
				t.Errorf("Gnome Token must be attacking")
			}
		}
	}
}

func countGnomeTokens(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Gnome Token" {
			n++
		}
	}
	return n
}
