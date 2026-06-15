package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// choose_surveil_r60_test.go — regressions for the R60 ChooseSurveil
// archetype biases:
//
//   - Control archetype sends non-keeper creatures to the graveyard
//     (control doesn't want creature density; what reaches play is
//     usually the commander + a single finisher).
//
//   - Combo archetype uses a stricter top-of-library threshold (0.50
//     vs 0.35 default) so mid-quality filler defaults to the graveyard
//     and the next draw is more likely to hit a combo piece or tutor.
//
//   - Existing behavior preserved: graveyard-recursion-potential cards
//     and reanimator-fatty CMC≥5 creatures still go to graveyard;
//     combo/value-engine/star cards still pinned to top; midrange
//     baseline keeps its 0.35 threshold.

// containsCard returns true if `target` is in the slice.
func containsCard(cards []*gameengine.Card, target *gameengine.Card) bool {
	for _, c := range cards {
		if c == target {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Control archetype: creatures (other than keepers) → graveyard
// -----------------------------------------------------------------------------

func TestChooseSurveil_Control_SendsVanillaCreatureToGraveyard(t *testing.T) {
	// Pair the bear with a real KEEPER (star card) so a card actually stays on
	// top — otherwise BOTH cards surveil to the graveyard, top is empty, and
	// the §701.46 "keep at least one on top" fallback correctly yanks cards[0]
	// (the bear) back. (The pre-r63 fallback masked this by leaving the bear in
	// BOTH top and graveyard — the within-zone CardIdentity dup; with that
	// fixed, the test needs a genuine keeper to exercise the bottom path.)
	sp := &StrategyProfile{Archetype: ArchetypeControl, StarCards: []string{"Sol Ring"}}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	bear := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	keeper := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)

	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{bear, keeper})

	if !containsCard(gy, bear) {
		t.Fatalf("control should send vanilla creatures to graveyard; bear in top=%v gy=%v",
			top, gy)
	}
	if containsCard(top, bear) {
		t.Fatalf("the bear must not be in BOTH top and graveyard (within-zone dup); top=%v gy=%v", top, gy)
	}
}

func TestChooseSurveil_Control_KeepsComboCreatureOnTop(t *testing.T) {
	sp := &StrategyProfile{
		Archetype: ArchetypeControl,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Dramatic Reversal", "Isochron Scepter"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	// Even on a control deck, a combo creature is a keeper — the
	// archetype bias must NOT override the combo / VE / star branch.
	comboCreature := newTestCardMinimal("Dramatic Reversal", []string{"creature"}, 2, nil)
	filler := newTestCardMinimal("Llanowar Elves", []string{"creature"}, 1, nil)

	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{comboCreature, filler})

	if !containsCard(top, comboCreature) {
		t.Fatalf("combo-relevant creature should stay on top for control; top=%v gy=%v", top, gy)
	}
}

func TestChooseSurveil_Control_LeavesNonCreatureSpellAlone(t *testing.T) {
	// Control bias only fires on creatures. We pin this by making the
	// sorcery a Star card so it's expected on top via the star branch;
	// the control creature-bias branch must not preempt that with the
	// creature-only filter.
	sp := &StrategyProfile{
		Archetype: ArchetypeControl,
		StarCards: []string{"Wrath of God"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	sorcery := newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4, nil)
	filler := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)
	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{sorcery, filler})

	if !containsCard(top, sorcery) {
		t.Fatalf("control's creature bias should not bottom a non-creature star card; top=%v gy=%v",
			top, gy)
	}
}

// -----------------------------------------------------------------------------
// Combo archetype: stricter top threshold sends filler to graveyard
// -----------------------------------------------------------------------------

func TestChooseSurveil_Combo_KeepsComboPieceOnTop(t *testing.T) {
	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	piece := newTestCardMinimal("Thassa's Oracle", []string{"creature"}, 2, nil)
	filler := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)

	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{piece, filler})
	if !containsCard(top, piece) {
		t.Fatalf("combo piece should stay on top under combo archetype; top=%v gy=%v", top, gy)
	}
}

func TestChooseSurveil_Combo_BottomsMidValueFiller(t *testing.T) {
	// A non-combo non-cuttable non-recursion creature whose
	// cardHeuristic sits in [0.35, 0.50) is sent to the graveyard
	// under combo's stricter threshold but kept on top under
	// midrange's default. Use a single-card surveil with a guaranteed
	// keeper alongside so the empty-top fallback doesn't promote the
	// filler.
	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	piece := newTestCardMinimal("Thassa's Oracle", []string{"creature"}, 2, nil)
	// Vanilla 2-mana sorcery — base 0.35 + small efficiency bonus
	// under Phase=Deploy / no mana available. Sits in the [0.35, 0.50)
	// window that the combo threshold lifts to the graveyard.
	filler := newTestCardMinimal("Doom Blade", []string{"sorcery"}, 2, nil)

	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{piece, filler})
	if !containsCard(gy, filler) {
		t.Fatalf("combo deck should bottom mid-value filler (Doom Blade); top=%v gy=%v",
			top, gy)
	}
}

func TestChooseSurveil_Midrange_KeepsSameFillerOnTop(t *testing.T) {
	// Baseline: same filler under midrange (default 0.35 threshold)
	// should go to top — pin the differential between midrange and
	// combo behavior on identical input. CuttableCards must be set
	// before constructing the hat — the hat builds its lookup sets at
	// construction and doesn't rebuild on Strategy mutation.
	sp := &StrategyProfile{
		Archetype:     ArchetypeMidrange,
		CuttableCards: []string{"Mediocre Stuff"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	filler := newTestCardMinimal("Doom Blade", []string{"sorcery"}, 2, nil)
	cuttable := newTestCardMinimal("Mediocre Stuff", []string{"creature"}, 3, nil)

	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{filler, cuttable})

	if !containsCard(top, filler) {
		t.Fatalf("midrange should keep Doom Blade on top (val ≥ 0.35); top=%v gy=%v", top, gy)
	}
	if !containsCard(gy, cuttable) {
		t.Fatalf("midrange should still bottom the cuttable; top=%v gy=%v", top, gy)
	}
}

// -----------------------------------------------------------------------------
// Pin existing behavior: reanimator-fatty + graveyard-recursion
// -----------------------------------------------------------------------------

func TestChooseSurveil_Reanimator_CMC5CreatureGoesToGraveyard(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeReanimator}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	fatty := newTestCardMinimal("Worldspine Wurm", []string{"creature"}, 11, nil)
	// Add a non-creature so the fallback doesn't yank the fatty back.
	land := newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil)

	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{fatty, land})
	if !containsCard(gy, fatty) {
		t.Fatalf("reanimator should send CMC=11 creature to graveyard; top=%v gy=%v", top, gy)
	}
}

func TestChooseSurveil_GraveyardRecursionPotentialAlwaysToGraveyard(t *testing.T) {
	// Pre-existing deck-agnostic behavior: flashback / unearth / escape
	// cards go to the graveyard for any archetype because the recursion
	// path lets us cast them from there. Run under a control archetype
	// to verify the recursion branch fires before the new control
	// creature-bias branch.
	sp := &StrategyProfile{Archetype: ArchetypeControl}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	flashback := cardWithKeyword("Lingering Souls", []string{"sorcery"}, 3, "flashback")
	keep := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)

	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{flashback, keep})
	if !containsCard(gy, flashback) {
		t.Fatalf("flashback card should go to graveyard under any archetype; top=%v gy=%v",
			top, gy)
	}
}

func TestChooseSurveil_Combo_StarCardOverridesStricterThreshold(t *testing.T) {
	// The stricter combo threshold only applies to the fallthrough
	// val-based branch — star cards are picked up earlier and pinned
	// to top regardless of threshold. Pin that ordering.
	sp := &StrategyProfile{
		Archetype:  ArchetypeCombo,
		StarCards:  []string{"Sol Ring"},
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	star := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)
	filler := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)

	gy, top := h.ChooseSurveil(gs, 0, []*gameengine.Card{star, filler})
	if !containsCard(top, star) {
		t.Fatalf("star card should be pinned to top under combo archetype; top=%v gy=%v",
			top, gy)
	}
}
