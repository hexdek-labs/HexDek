package gameengine

import "testing"

// TestHasNonDeckType pins the r63 deep-sweep fix: the chaos corpus excludes
// non-deck card types (Plane / Phenomenon / Scheme / Vanguard / Conspiracy /
// Dungeon / Emblem) so they can't leak into a fuzz deck and get illegally
// "cast" (the Plane "Oteclán" tripped LEGALITY §117.1a, seed 25450043).
// Critically, "plane" must NOT match "planeswalker" — planeswalkers ARE
// deck cards.
func TestHasNonDeckType(t *testing.T) {
	excluded := [][]string{
		{"plane", "ixalan"},   // Oteclán
		{"phenomenon"},        // Archenemy
		{"scheme"},            // Archenemy
		{"ongoing", "scheme"}, // ongoing scheme
		{"vanguard"},          // Vanguard
		{"conspiracy"},        // Conspiracy draft
		{"dungeon"},           // venture
		{"emblem"},            // emblem token
	}
	for _, tl := range excluded {
		if !hasNonDeckType(tl) {
			t.Errorf("types %v should be excluded as a non-deck type", tl)
		}
	}

	kept := [][]string{
		{"legendary", "planeswalker", "jace"}, // planeswalkers ARE deck cards
		{"creature", "golem"},
		{"artifact"},
		{"instant"},
		{"sorcery"},
		{"legendary", "creature", "human", "wizard"},
		{"land"},
		{"enchantment"},
		{"battle", "siege"}, // battles are deck-legal
	}
	for _, tl := range kept {
		if hasNonDeckType(tl) {
			t.Errorf("types %v should NOT be excluded — it is a real deck card type", tl)
		}
	}
}
