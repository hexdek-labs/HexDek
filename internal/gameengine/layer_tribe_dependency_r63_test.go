package gameengine

// r63 — §613.8 dependency foundation regressions. The canonical example:
// Conspiracy ("all creatures are Elves", a layer-4 type-changing effect) must
// apply BEFORE an Elvish lord's layer-7 "Elves you control get +1/+1", so the
// lord pumps a creature that is an Elf only via Conspiracy. The lord's old
// subtype gate read the PRINTED type line and missed layer-4-granted types.

import (
	"math/rand"
	"testing"
)

func layerDepGame() *GameState {
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	return gs
}

func addCreaturePerm(gs *GameState, seat int, name, typeLine string, pow, tough int) *Permanent {
	p := &Permanent{
		Card: &Card{
			Name: name, Owner: seat, Types: []string{"creature"},
			TypeLine: typeLine, BasePower: pow, BaseToughness: tough,
		},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// addElfLord registers an "other Elves you control get +1/+1" layer-7 anthem
// sourced from `lord`, using the §613.8-correct chars-aware tribe gate.
func addElfLord(gs *GameState, lord *Permanent) {
	src := lord
	registerTribeAnthemPT(gs, lord, 1, 1, "test-elf-lord", "elf", false, func(_ *GameState, t *Permanent) bool {
		return t != src && t.Controller == src.Controller
	})
}

// CANONICAL §613.8: Conspiracy (L4, "all creatures are Elves") feeds the Elvish
// lord (L7). A vanilla Bear becomes an Elf and IS pumped.
func TestLayerDep_ConspiracyFeedsElfLord(t *testing.T) {
	gs := layerDepGame()
	lord := addCreaturePerm(gs, 0, "Elvish Lord", "Creature — Elf", 2, 2)
	addElfLord(gs, lord)
	bear := addCreaturePerm(gs, 0, "Grizzly Bears", "Creature — Bear", 2, 2)

	// Conspiracy choosing Elf, on seat 0.
	consp := &Permanent{
		Card:       &Card{Name: "Conspiracy", Owner: 0, Types: []string{"enchantment"}, TypeLine: "Enchantment"},
		Controller: 0, Owner: 0, Counters: map[string]int{},
		Flags:     map[string]int{"conspiracy_type_elf": 1},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, consp)
	RegisterConspiracy(gs, consp)

	ch := GetEffectiveCharacteristics(gs, bear)
	if ch.Power != 3 || ch.Toughness != 3 {
		t.Fatalf("Conspiracy should make the Bear an Elf so the lord pumps it to 3/3; got %d/%d", ch.Power, ch.Toughness)
	}
}

// No-regression: a PRINTED Elf is still pumped by the lord (and the lord does
// not pump itself; "other Elves").
func TestLayerDep_PrintedElfStillPumped(t *testing.T) {
	gs := layerDepGame()
	lord := addCreaturePerm(gs, 0, "Elvish Lord", "Creature — Elf", 2, 2)
	addElfLord(gs, lord)
	elf := addCreaturePerm(gs, 0, "Llanowar Elves", "Creature — Elf Druid", 1, 1)
	bear := addCreaturePerm(gs, 0, "Grizzly Bears", "Creature — Bear", 2, 2)

	if ch := GetEffectiveCharacteristics(gs, elf); ch.Power != 2 {
		t.Fatalf("printed Elf should be pumped 1→2 by the lord; got %d", ch.Power)
	}
	if ch := GetEffectiveCharacteristics(gs, bear); ch.Power != 2 {
		t.Fatalf("non-Elf Bear should NOT be pumped (no Conspiracy); got %d", ch.Power)
	}
	if ch := GetEffectiveCharacteristics(gs, lord); ch.Power != 2 {
		t.Fatalf("lord should not pump itself (other Elves); got %d", ch.Power)
	}
}

// Property (e): a "creatures you control get +1/+1" anthem tracks the CURRENT
// controller — after a control change the creature is pumped by its NEW
// controller's anthem, not its original controller's.
func TestLayerDep_ControlChangeFeedsAnthem(t *testing.T) {
	gs := layerDepGame()
	// Seat 0's anthem: creatures you (seat 0) control get +1/+1.
	anthemSrc := addCreaturePerm(gs, 0, "Glorious Anthem Bearer", "Creature — Soldier", 0, 0)
	src := anthemSrc
	registerAnthemPT(gs, anthemSrc, 1, 1, "test-yours-anthem", func(_ *GameState, t *Permanent) bool {
		return t.Controller == src.Controller
	})
	// A creature initially controlled by seat 1 (opponent) — NOT pumped.
	creature := addCreaturePerm(gs, 1, "Borrowed Beast", "Creature — Beast", 3, 3)
	if ch := GetEffectiveCharacteristics(gs, creature); ch.Power != 3 {
		t.Fatalf("opponent's creature should not be pumped by seat 0's anthem; got %d", ch.Power)
	}

	// Control change: seat 0 gains control (physical move + controller field).
	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[:0]
	creature.Controller = 0
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)
	gs.InvalidateCharacteristicsCache()

	if ch := GetEffectiveCharacteristics(gs, creature); ch.Power != 4 {
		t.Fatalf("after control change to seat 0, the anthem should pump the creature 3→4; got %d", ch.Power)
	}
}
