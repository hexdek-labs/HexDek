package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// choose_mode_r60_test.go — regressions for the R60 ChooseMode
// additions:
//
//   1. counter_spell mode scoring — previously fell into the default
//      0.30 bucket. Now scales by hostile stack contents (0.85 with a
//      targetable spell, 0.05 with nothing to counter) and bumps for
//      control / spellslinger archetypes.
//
//   2. Library-low penalty on draw mode — drawing into a near-empty
//      library accelerates a §704.5b loss. A 0.60x multiplier kicks
//      in at lib<=7, dropping to 0.25x at lib<=3.
//
//   3. Archetype bumps on existing modes — control / stax get +0.05
//      on destroy/exile; aggro / spellslinger get +0.05 on damage /
//      lose_life.

// fillHand stuffs the seat's hand with n filler cards so hand-size-
// scaled scoring fires at the right tier.
func fillHand(gs *gameengine.GameState, seat, n int) {
	gs.Seats[seat].Hand = make([]*gameengine.Card, n)
	for i := range gs.Seats[seat].Hand {
		gs.Seats[seat].Hand[i] = newTestCardMinimal("X", []string{"sorcery"}, 1, nil)
	}
}

// fillLibrary stuffs the seat's library with n filler cards.
func fillLibrary(gs *gameengine.GameState, seat, n int) {
	gs.Seats[seat].Library = make([]*gameengine.Card, n)
	for i := range gs.Seats[seat].Library {
		gs.Seats[seat].Library[i] = newTestCardMinimal("L", []string{"sorcery"}, 1, nil)
	}
}

// -----------------------------------------------------------------------------
// counter_spell mode
// -----------------------------------------------------------------------------

func TestScoreModeEffect_CounterSpell_HighWhenHostileSpellOnStack(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)

	// Opponent has a spell on the stack.
	gs.Stack = append(gs.Stack, &gameengine.StackItem{
		Controller: 1,
		Card:       newTestCardMinimal("Cultivate", []string{"sorcery"}, 3, nil),
	})

	score := h.scoreModeEffect(gs, 0, &gameast.CounterSpell{}, 0)
	if score < 0.80 {
		t.Fatalf("counter_spell with a hostile spell on stack should be ≥ 0.80; got %.2f", score)
	}
}

func TestScoreModeEffect_CounterSpell_LowWhenStackEmpty(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)

	score := h.scoreModeEffect(gs, 0, &gameast.CounterSpell{}, 0)
	if score > 0.20 {
		t.Fatalf("counter_spell with empty stack should be ≤ 0.20; got %.2f", score)
	}
}

func TestScoreModeEffect_CounterSpell_IgnoresOwnSpellOnStack(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)

	// Our own spell on the stack — we don't want to counter ourselves.
	gs.Stack = append(gs.Stack, &gameengine.StackItem{
		Controller: 0,
		Card:       newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil),
	})

	score := h.scoreModeEffect(gs, 0, &gameast.CounterSpell{}, 0)
	if score > 0.20 {
		t.Fatalf("counter_spell vs our own spell should be near-zero; got %.2f", score)
	}
}

func TestScoreModeEffect_CounterSpell_IgnoresStackCopies(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)

	// CR §707.10 — copies aren't legal counter targets ("can't be
	// countered as it's not on the stack as a real spell" is technically
	// inaccurate, but for mode-pick purposes a copy is a worse target
	// than a real spell, and we should not score the mode highly off a
	// copy alone). The hat takes a conservative read here.
	gs.Stack = append(gs.Stack, &gameengine.StackItem{
		Controller: 1,
		Card:       newTestCardMinimal("Counterspell Copy", []string{"instant"}, 2, nil),
		IsCopy:     true,
	})

	score := h.scoreModeEffect(gs, 0, &gameast.CounterSpell{}, 0)
	if score > 0.20 {
		t.Fatalf("counter_spell vs only-a-copy on stack should be near-zero; got %.2f", score)
	}
}

func TestScoreModeEffect_CounterSpell_ControlArchetypeBump(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Stack = append(gs.Stack, &gameengine.StackItem{
		Controller: 1,
		Card:       newTestCardMinimal("Cultivate", []string{"sorcery"}, 3, nil),
	})

	baseline := NewYggdrasilHat(nil, 0)
	baselineScore := baseline.scoreModeEffect(gs, 0, &gameast.CounterSpell{}, 0)

	control := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeControl}, 0)
	controlScore := control.scoreModeEffect(gs, 0, &gameast.CounterSpell{}, 0)

	if controlScore <= baselineScore {
		t.Fatalf("control archetype should bump counter_spell over baseline; baseline=%.2f control=%.2f",
			baselineScore, controlScore)
	}
}

// -----------------------------------------------------------------------------
// Library-low penalty on draw
// -----------------------------------------------------------------------------

func TestScoreModeEffect_Draw_LibraryLowAppliesPenalty(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)

	// Empty hand baseline (best case for draw): full library.
	gs.Seats[0].Hand = nil
	fillLibrary(gs, 0, 80)
	fullLib := h.scoreModeEffect(gs, 0, &gameast.Draw{}, 0)

	// Same empty hand, but library is at 7 cards.
	fillLibrary(gs, 0, 7)
	lowLib := h.scoreModeEffect(gs, 0, &gameast.Draw{}, 0)

	if lowLib >= fullLib {
		t.Fatalf("low-library draw should score below full-library draw; full=%.2f low=%.2f",
			fullLib, lowLib)
	}
	// At lib=7 the multiplier is 0.60x — so empty-hand 0.90 base becomes
	// ~0.54. Let some slack for DNA nudges; just assert clearly below.
	if lowLib > 0.65 {
		t.Fatalf("at lib=7 with empty hand, draw should sit ≤ 0.65 after the 0.60x penalty; got %.2f",
			lowLib)
	}
}

func TestScoreModeEffect_Draw_LibraryDangerouslyLow(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	gs.Seats[0].Hand = nil
	fillLibrary(gs, 0, 2) // dangerously low — 0.25x multiplier

	score := h.scoreModeEffect(gs, 0, &gameast.Draw{}, 0)
	if score > 0.30 {
		t.Fatalf("at lib=2 with empty hand, draw should be ≤ 0.30 after the 0.25x penalty; got %.2f", score)
	}
}

func TestScoreModeEffect_Draw_EmptyLibraryNoPenaltyMultiplier(t *testing.T) {
	// Defensive: an empty library is a §704.5b death sentence on the
	// next draw, but for mode scoring we shouldn't multiply by 0 (that
	// would make the mode literally unreachable in tied races with
	// other zero-scored modes). The guard `lib > 0` keeps the empty
	// case at the base hand-size score.
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	gs.Seats[0].Hand = nil
	gs.Seats[0].Library = nil

	score := h.scoreModeEffect(gs, 0, &gameast.Draw{}, 0)
	if score < 0.80 {
		t.Fatalf("lib=0 should fall through the penalty guard and hold base score; got %.2f", score)
	}
}

func TestScoreModeEffect_Draw_NoLibraryPenaltyAtFullLibrary(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	fillHand(gs, 0, 4) // mid-hand → base 0.55
	fillLibrary(gs, 0, 80)

	score := h.scoreModeEffect(gs, 0, &gameast.Draw{}, 0)
	if score < 0.50 || score > 0.60 {
		t.Fatalf("at hand=4 / lib=80, score should sit near base 0.55; got %.2f", score)
	}
}

// -----------------------------------------------------------------------------
// Archetype bumps on destroy / damage
// -----------------------------------------------------------------------------

func TestScoreModeEffect_DestroyControlArchetypeBump(t *testing.T) {
	gs := newTestGame(t, 2)
	// Opponent has a target.
	bear := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	newTestPermanent(gs.Seats[1], bear, 2, 2)

	baseline := NewYggdrasilHat(nil, 0)
	baselineScore := baseline.scoreModeEffect(gs, 0, &gameast.Destroy{}, 0)

	control := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeControl}, 0)
	controlScore := control.scoreModeEffect(gs, 0, &gameast.Destroy{}, 0)

	if controlScore <= baselineScore {
		t.Fatalf("control should bump destroy over baseline; baseline=%.2f control=%.2f",
			baselineScore, controlScore)
	}
}

func TestScoreModeEffect_DamageAggroArchetypeBump(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Life = 20 // not in lethal range from a small burn

	baseline := NewYggdrasilHat(nil, 0)
	baselineScore := baseline.scoreModeEffect(gs, 0, &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}}, 0)

	aggro := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeAggro}, 0)
	aggroScore := aggro.scoreModeEffect(gs, 0, &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}}, 0)

	if aggroScore <= baselineScore {
		t.Fatalf("aggro should bump damage over baseline; baseline=%.2f aggro=%.2f",
			baselineScore, aggroScore)
	}
}

func TestScoreModeEffect_DamageNonAggroArchetypeNoBump(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Life = 20

	baseline := NewYggdrasilHat(nil, 0)
	baselineScore := baseline.scoreModeEffect(gs, 0, &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}}, 0)

	ramp := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeRamp}, 0)
	rampScore := ramp.scoreModeEffect(gs, 0, &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}}, 0)

	if rampScore != baselineScore {
		t.Fatalf("non-aggro/non-spellslinger archetype should leave damage unchanged; baseline=%.2f ramp=%.2f",
			baselineScore, rampScore)
	}
}
