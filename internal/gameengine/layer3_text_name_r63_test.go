package gameengine

// r63 — layer-3 text-change probe (CR §613.1c), property (d): a text-changing
// effect rewrites the referenced WORD in a card's rules text but does NOT
// change the card's NAME (names are not "text" for §613.1c unless the effect
// says so). This was the one probed property with no existing coverage; the
// other layer-3 behaviors (chars swap, runs-before-layer-4, revert-on-unregister)
// are pinned by the TestLayer3_* suite in layers_test.go.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// A creature whose NAME contains the swapped land-type word keeps its name
// while its ability text (the swampwalk keyword) is rewritten by Swirl the
// Mists (swamp → island).
func TestLayer3_TextChange_DoesNotChangeCardName(t *testing.T) {
	gs := newFixtureGame(t)
	swirl := addBattlefield(gs, 0, "Swirl the Mists", 0, 0, "enchantment")
	swirl.Flags["text_from_swamp"] = 1
	swirl.Flags["text_to_island"] = 1
	RegisterContinuousEffectsForPermanent(gs, swirl)

	// "Swamp Mosquito" — the card NAME contains "swamp", and it has swampwalk.
	creature := addBattlefield(gs, 0, "Swamp Mosquito", 1, 1, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Swamp Mosquito",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampwalk", Raw: "Swampwalk"},
		},
	}

	chars := GetEffectiveCharacteristics(gs, creature)

	// (d) the NAME is untouched — "Swamp Mosquito" stays "Swamp Mosquito".
	if chars.Name != "Swamp Mosquito" {
		t.Errorf("layer-3 text-change must NOT rewrite the card name; got %q, want \"Swamp Mosquito\"", chars.Name)
	}

	// ...while the ability TEXT is rewritten (proving the swap is active and the
	// name-preservation above is meaningful, not just an inert effect).
	foundIslandwalk := false
	for _, kw := range chars.Keywords {
		if kw == "islandwalk" {
			foundIslandwalk = true
		}
		if kw == "swampwalk" {
			t.Errorf("layer-3: swampwalk should have been rewritten to islandwalk in the ability text")
		}
	}
	if !foundIslandwalk {
		t.Errorf("layer-3: expected the ability keyword to become islandwalk; got %v", chars.Keywords)
	}
}
