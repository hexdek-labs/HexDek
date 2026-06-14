package gameengine

// r63 — Theros God creature-toggle (CR 711.2). A god is a creature ONLY while
// devotion to its color(s) is at/above its threshold; below it the god stays on
// the battlefield as an indestructible enchantment but is NOT a creature. The
// `devotion_gated_not_creature` modkind was previously UNWIRED, so gods (printed
// "Enchantment Creature") were ALWAYS creatures regardless of devotion.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func godTestGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	return gs
}

// addGod builds a Theros-style god permanent on `seat` with the given gate.
func addGod(gs *GameState, seat int, name, manaCost string, colorWords []interface{}, thresholdWord string) *Permanent {
	ast := &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Static{
				Modification: &gameast.Modification{
					ModKind: "devotion_gated_not_creature",
					Args:    []interface{}{colorWords, thresholdWord},
				},
			},
		},
	}
	card := &Card{
		Name:           name,
		Owner:          seat,
		Types:          []string{"legendary", "enchantment", "creature", "god"},
		ManaCostString: manaCost,
		BasePower:      5,
		BaseToughness:  7,
		AST:            ast,
	}
	p := &Permanent{
		Card: card, Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// addPip adds a permanent contributing one colored pip toward devotion.
func addPip(gs *GameState, seat int, manaCost string) *Permanent {
	p := &Permanent{
		Card:      &Card{Name: "Pip " + manaCost, Owner: seat, Types: []string{"enchantment"}, ManaCostString: manaCost},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Erebos {3}{B}{B} = 2 devotion alone; threshold 5. Below → NOT a creature, but
// still on the battlefield as an enchantment.
func TestGodToggle_BelowThreshold_NotCreatureButStaysEnchantment(t *testing.T) {
	gs := godTestGame(t)
	erebos := addGod(gs, 0, "Erebos, God of the Dead", "{3}{B}{B}", []interface{}{"black"}, "five")

	StateBasedActions(gs)

	if erebos.IsCreature() {
		t.Fatalf("Erebos at devotion 2 (<5) must NOT be a creature")
	}
	if !erebos.hasType("enchantment") {
		t.Fatalf("Erebos must remain an enchantment permanent below threshold")
	}
	// Still on the battlefield (not sacrificed/destroyed).
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == erebos {
			found = true
		}
	}
	if !found {
		t.Fatalf("Erebos must remain on the battlefield below threshold")
	}
}

// At devotion >= 5 Erebos IS a creature.
func TestGodToggle_AtThreshold_IsCreature(t *testing.T) {
	gs := godTestGame(t)
	erebos := addGod(gs, 0, "Erebos, God of the Dead", "{3}{B}{B}", []interface{}{"black"}, "five")
	// Erebos = 2; add three {B} permanents → total devotion 5.
	addPip(gs, 0, "{B}")
	addPip(gs, 0, "{B}")
	addPip(gs, 0, "{B}")

	StateBasedActions(gs)

	if !erebos.IsCreature() {
		t.Fatalf("Erebos at devotion 5 (>=5) MUST be a creature")
	}
}

// Devotion dropping below threshold reverts the god to non-creature.
func TestGodToggle_DropReverts(t *testing.T) {
	gs := godTestGame(t)
	erebos := addGod(gs, 0, "Erebos, God of the Dead", "{3}{B}{B}", []interface{}{"black"}, "five")
	p1 := addPip(gs, 0, "{B}")
	addPip(gs, 0, "{B}")
	addPip(gs, 0, "{B}")
	StateBasedActions(gs)
	if !erebos.IsCreature() {
		t.Fatalf("precondition: Erebos should be a creature at devotion 5")
	}

	// Remove one black pip → devotion 4 (<5).
	bf := gs.Seats[0].Battlefield[:0]
	for _, p := range gs.Seats[0].Battlefield {
		if p == p1 {
			continue
		}
		bf = append(bf, p)
	}
	gs.Seats[0].Battlefield = bf

	StateBasedActions(gs)

	if erebos.IsCreature() {
		t.Fatalf("Erebos must REVERT to non-creature when devotion drops below 5")
	}
}

// Two-color god (Phenax, {3}{U}{B}, threshold seven) uses combined devotion.
func TestGodToggle_TwoColor_CombinedThreshold(t *testing.T) {
	gs := godTestGame(t)
	// Phenax {3}{U}{B} = 1 blue + 1 black = 2 combined.
	phenax := addGod(gs, 0, "Phenax, God of Deception", "{3}{U}{B}", []interface{}{"blue", "black"}, "seven")

	StateBasedActions(gs)
	if phenax.IsCreature() {
		t.Fatalf("Phenax at combined devotion 2 (<7) must NOT be a creature")
	}

	// Raise combined blue+black devotion to 7: add 5 more single-color pips.
	addPip(gs, 0, "{U}")
	addPip(gs, 0, "{U}")
	addPip(gs, 0, "{B}")
	addPip(gs, 0, "{B}")
	addPip(gs, 0, "{B}")

	StateBasedActions(gs)
	if !phenax.IsCreature() {
		t.Fatalf("Phenax at combined devotion 7 (>=7) MUST be a creature")
	}
}
