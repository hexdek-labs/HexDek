package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 BLINK / FLICKER behavioral audit (per_card hand-rolled blinkers).

// (f) A blinked TOKEN ceases to exist — it does NOT return.
func TestBlink_Deadeye_TokenCeases(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	deadeye := stampCreaturePT(addPerm(gs, 0, "Deadeye Navigator", "creature", "legendary"), 3, 3)
	tok := gameengine.CreateCreatureToken(gs, 0, "Beast Token", []string{"creature", "beast"}, 3, 3)
	deadeyeNavigatorActivate(gs, deadeye, 0, map[string]interface{}{"target_perm": tok})
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Beast Token" {
			t.Fatal("blinked token must cease to exist, not return")
		}
	}
}

func TestBlink_Displacer_TokenCeases(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	addPerm(gs, 0, "Displacer Kitten", "creature", "legendary")
	gameengine.CreateCreatureToken(gs, 0, "Beast Token", []string{"creature", "beast"}, 3, 3)
	displacerKittenOnCast(gs, gs.Seats[0].Battlefield[0], map[string]interface{}{"caster_seat": 0})
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Beast Token" {
			t.Fatal("Displacer-blinked token must cease, not return")
		}
	}
}

// stolenCreature puts a creature owned by `owner` onto `controller`'s
// battlefield (Control-Magic style).
func stolenCreature(gs *gameengine.GameState, controller, owner int, name string) *gameengine.Permanent {
	c := &gameengine.Card{Name: name, Owner: owner, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	p := &gameengine.Permanent{Card: c, Controller: controller, Owner: owner, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[controller].Battlefield = append(gs.Seats[controller].Battlefield, p)
	return p
}

// (d) "return under YOUR control" keeps a stolen creature under the blinker's
// control — it is NOT handed back to its owner.
func TestBlink_Emiel_StolenStaysUnderYourControl(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	emiel := stampCreaturePT(addPerm(gs, 0, "Emiel the Blessed", "creature", "legendary"), 3, 4)
	stolen := stolenCreature(gs, 0, 1, "Stolen Bear")
	emielActivate(gs, emiel, 0, map[string]interface{}{"target_perm": stolen})

	onMine, onOwner := 0, 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Stolen Bear" {
			onMine++
		}
	}
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card != nil && p.Card.Name == "Stolen Bear" {
			onOwner++
		}
	}
	if onMine != 1 || onOwner != 0 {
		t.Fatalf("Emiel 'under your control' must keep the stolen creature on seat 0 (got mine=%d owner=%d)", onMine, onOwner)
	}
}

func TestBlink_Displacer_StolenStaysUnderYourControl(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	addPerm(gs, 0, "Displacer Kitten", "creature", "legendary")
	kitten := gs.Seats[0].Battlefield[0]
	stolen := stolenCreature(gs, 0, 1, "Stolen Bear")
	_ = stolen
	_ = kitten
	displacerKittenOnCast(gs, kitten, map[string]interface{}{"caster_seat": 0})

	onOwner := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card != nil && p.Card.Name == "Stolen Bear" {
			onOwner++
		}
	}
	if onOwner != 0 {
		t.Fatalf("Displacer 'under your control' must NOT hand the stolen creature back to its owner (seat 1 has %d)", onOwner)
	}
}
