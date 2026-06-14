package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestEvalScalingDevotion_CountsPipsNotCMC pins the fix to scaling.go's
// "devotion" resolver, the path that powers "X = your devotion to <color>"
// effects such as Gray Merchant of Asphodel's drain. It previously summed the
// mana VALUE of colored permanents instead of counting colored mana SYMBOLS
// (CR §700.5), wildly over-scaling the effect.
func TestEvalScalingDevotion_CountsPipsNotCMC(t *testing.T) {
	gs := newDevotionGame(t)

	// Gray Merchant {3}{B}{B}: 2 black pips, but CMC 5.
	gary := addCostedPermanent(gs, 0, "Gray Merchant of Asphodel", "{3}{B}{B}")
	gary.Card.CMC = 5
	// A second black card {1}{B}: 1 black pip, CMC 2.
	cheap := addCostedPermanent(gs, 0, "Cheap Black", "{1}{B}")
	cheap.Card.CMC = 2

	// Correct devotion to black = 2 + 1 = 3 pips. The old CMC approximation
	// would have returned 5 + 2 = 7.
	src := gs.Seats[0].Battlefield[0]
	sa := &gameast.ScalingAmount{ScalingKind: "devotion", Args: []interface{}{"black"}}
	got, ok := evalScaling(gs, src, sa)
	if !ok {
		t.Fatal("evalScaling(devotion) should resolve")
	}
	if got != 3 {
		t.Fatalf("devotion to black = colored pip count (3), not CMC sum (7); got %d", got)
	}
}

// TestEvalScalingDevotion_HybridAndGeneric verifies the scaling resolver now
// honors hybrid pips (count toward both colors) and ignores generic/X.
func TestEvalScalingDevotion_HybridAndGeneric(t *testing.T) {
	gs := newDevotionGame(t)
	// Nightveil Specter {1}{U/B}{U/B}: 2 black pips (hybrid), 2 blue pips.
	addCostedPermanent(gs, 0, "Nightveil Specter", "{1}{U/B}{U/B}")
	// Pure generic {4}: 0 devotion to anything.
	addCostedPermanent(gs, 0, "Colorless Lump", "{4}")
	src := gs.Seats[0].Battlefield[0]

	black, _ := evalScaling(gs, src, &gameast.ScalingAmount{ScalingKind: "devotion", Args: []interface{}{"black"}})
	if black != 2 {
		t.Errorf("hybrid {U/B} counts toward black; want 2, got %d", black)
	}
	blue, _ := evalScaling(gs, src, &gameast.ScalingAmount{ScalingKind: "devotion", Args: []interface{}{"blue"}})
	if blue != 2 {
		t.Errorf("hybrid {U/B} counts toward blue; want 2, got %d", blue)
	}
	red, _ := evalScaling(gs, src, &gameast.ScalingAmount{ScalingKind: "devotion", Args: []interface{}{"red"}})
	if red != 0 {
		t.Errorf("no red pips on board; want 0, got %d", red)
	}
}

// TestCountDevotion_IgnoresColoredTokens pins the token-skip fix. A token has
// no mana cost (CR §700.5 counts symbols in mana COSTS), so a colored token
// contributes 0 devotion even though it carries a color.
func TestCountDevotion_IgnoresColoredTokens(t *testing.T) {
	gs := newDevotionGame(t)

	// One real black card {1}{B} → 1 black devotion.
	addCostedPermanent(gs, 0, "Real Black", "{1}{B}")

	// A black Zombie token: Colors=[B], no mana cost. Must contribute 0.
	tok := addCostedPermanent(gs, 0, "Zombie Token", "")
	tok.Card.Types = []string{"token", "creature"}
	tok.Card.Colors = []string{"B"}

	if got := CountDevotion(gs, 0, "B"); got != 1 {
		t.Fatalf("a colored token has no mana cost → 0 devotion; want 1 (only the real card), got %d", got)
	}
}
