package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// choose_scry_r60_test.go — regressions for the R60 ChooseScry
// archetype biases + finisher preserve-mode.
//
// Mirror of the ChooseSurveil R60 additions plus the finisher branch:
//
//   - Control archetype sends non-keeper creatures to the bottom.
//   - Combo archetype tightens the keep threshold to 0.50 (vs the
//     relPos-driven default of 0.35 ± 0.10).
//   - isFinisher in the keepers branch alongside combo / VE / star:
//     a finisher always stays on top regardless of val or archetype,
//     even when we're behind and the relPos-relaxed threshold would
//     otherwise have let a low-val finisher slip through. Finishers
//     and StarCards are populated from separate Freya fields so a
//     finisher card isn't always in StarCards — this closes the gap.

// scryContains is a small wrapper for readability in the slice-membership
// assertions; the helper itself is the same as containsCard from
// choose_surveil_r60_test.go but kept local so the two files stay
// independently runnable.
func scryContains(cards []*gameengine.Card, target *gameengine.Card) bool {
	for _, c := range cards {
		if c == target {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Control archetype: non-keeper creatures → bottom
// -----------------------------------------------------------------------------

func TestChooseScry_Control_BottomsNonKeeperCreature(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeControl}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	bear := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	// Pair with a keeper so the top-empty fallback doesn't pull the
	// bear back to top.
	keeper := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)
	sp.StarCards = []string{"Sol Ring"}
	h = NewYggdrasilHatWithNoise(sp, 0, 0)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{bear, keeper})

	if !scryContains(bottom, bear) {
		t.Fatalf("control should bottom non-keeper creatures; top=%v bottom=%v", top, bottom)
	}
}

func TestChooseScry_Control_KeepsComboCreatureOnTop(t *testing.T) {
	sp := &StrategyProfile{
		Archetype: ArchetypeControl,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Dramatic Reversal", "Isochron Scepter"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	comboCreature := newTestCardMinimal("Dramatic Reversal", []string{"creature"}, 2, nil)
	filler := newTestCardMinimal("Llanowar Elves", []string{"creature"}, 1, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{comboCreature, filler})
	if !scryContains(top, comboCreature) {
		t.Fatalf("combo creature should stay on top under control; top=%v bottom=%v",
			top, bottom)
	}
}

func TestChooseScry_Control_LeavesNonCreatureSpellAlone(t *testing.T) {
	sp := &StrategyProfile{
		Archetype: ArchetypeControl,
		StarCards: []string{"Wrath of God"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	sorcery := newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4, nil)
	filler := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{sorcery, filler})
	if !scryContains(top, sorcery) {
		t.Fatalf("control's creature filter should not bottom a non-creature star; top=%v bottom=%v",
			top, bottom)
	}
}

// -----------------------------------------------------------------------------
// Combo archetype: stricter top threshold (0.50)
// -----------------------------------------------------------------------------

func TestChooseScry_Combo_KeepsComboPieceOnTop(t *testing.T) {
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

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{piece, filler})
	if !scryContains(top, piece) {
		t.Fatalf("combo piece should stay on top under combo archetype; top=%v bottom=%v",
			top, bottom)
	}
}

func TestChooseScry_Combo_BottomsMidValueFiller(t *testing.T) {
	// Mid-value non-keeper goes to bottom under combo's stricter
	// 0.50 threshold, but stays on top under the default 0.35
	// threshold (covered in the midrange-baseline test below).
	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	piece := newTestCardMinimal("Thassa's Oracle", []string{"creature"}, 2, nil)
	filler := newTestCardMinimal("Doom Blade", []string{"sorcery"}, 2, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{piece, filler})
	if !scryContains(bottom, filler) {
		t.Fatalf("combo deck should bottom mid-value filler (Doom Blade); top=%v bottom=%v",
			top, bottom)
	}
}

func TestChooseScry_Midrange_KeepsSameFillerOnTop(t *testing.T) {
	// Pin the differential: same filler, midrange archetype, default
	// threshold = 0.35 → top.
	sp := &StrategyProfile{
		Archetype:     ArchetypeMidrange,
		CuttableCards: []string{"Mediocre Stuff"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	filler := newTestCardMinimal("Doom Blade", []string{"sorcery"}, 2, nil)
	cuttable := newTestCardMinimal("Mediocre Stuff", []string{"creature"}, 3, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{filler, cuttable})
	if !scryContains(top, filler) {
		t.Fatalf("midrange should keep filler on top; top=%v bottom=%v", top, bottom)
	}
	if !scryContains(bottom, cuttable) {
		t.Fatalf("midrange should still bottom the cuttable; top=%v bottom=%v",
			top, bottom)
	}
}

// -----------------------------------------------------------------------------
// Finisher preserve-mode
// -----------------------------------------------------------------------------

func TestChooseScry_Finisher_AlwaysKeptOnTop(t *testing.T) {
	// FinisherCards is a separate Freya field from StarCards — a card
	// can be a finisher without also being a star. The R60 addition
	// brings isFinisher into the keepers branch so the gap is closed.
	sp := &StrategyProfile{
		Archetype:     ArchetypeMidrange,
		FinisherCards: []string{"Craterhoof Behemoth"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	finisher := newTestCardMinimal("Craterhoof Behemoth", []string{"creature"}, 8, nil)
	// Pair with a definite bottom so the top-empty fallback doesn't
	// just promote the first card unconditionally.
	throwaway := newTestCardMinimal("Bad Card", []string{"creature"}, 6, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{finisher, throwaway})
	if !scryContains(top, finisher) {
		t.Fatalf("finisher should always stay on top; top=%v bottom=%v", top, bottom)
	}
}

func TestChooseScry_Finisher_KeptUnderControlArchetype(t *testing.T) {
	// The finisher branch fires BEFORE the control creature-filter
	// branch — a control deck's finisher (often a creature: Torment
	// of Hailfire, Approach of the Second Sun on a board with Doubling
	// Cube, etc.) must not be bottomed by the creature filter.
	sp := &StrategyProfile{
		Archetype:     ArchetypeControl,
		FinisherCards: []string{"Massacre Wurm"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	finisher := newTestCardMinimal("Massacre Wurm", []string{"creature"}, 6, nil)
	filler := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{finisher, filler})
	if !scryContains(top, finisher) {
		t.Fatalf("finisher should beat control's creature-bottom filter; top=%v bottom=%v",
			top, bottom)
	}
	if !scryContains(bottom, filler) {
		t.Fatalf("non-finisher control creature should still bottom; top=%v bottom=%v",
			top, bottom)
	}
}

func TestChooseScry_Finisher_KeptWhenBehindEvenAtLowVal(t *testing.T) {
	// Preserve-mode rationale: when we're behind, the next draw being
	// our finisher is the lifeline; the lowered behind-threshold (0.25)
	// would still let a sub-0.25-val finisher slip through without the
	// new isFinisher gate. Manufacture the scenario: behind relPos +
	// a finisher that lands a low val by deck context (high CMC under
	// Phase=Deploy with no mana sources).
	sp := &StrategyProfile{
		Archetype:     ArchetypeMidrange,
		FinisherCards: []string{"Worldspine Wurm"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	// Drop our life to make relPos < -0.3. Even simpler: stack the
	// opponent with a higher-eval position.
	gs.Seats[0].Life = 5
	for i := 0; i < 5; i++ {
		newTestPermanent(gs.Seats[1], newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil), 0, 0)
		newTestPermanent(gs.Seats[1], newTestCardMinimal("Big Stuff", []string{"creature"}, 5, nil), 5, 5)
	}

	finisher := newTestCardMinimal("Worldspine Wurm", []string{"creature"}, 11, nil)
	filler := newTestCardMinimal("Vanilla", []string{"creature"}, 6, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{finisher, filler})
	if !scryContains(top, finisher) {
		t.Fatalf("behind-the-curve finisher must be kept on top; top=%v bottom=%v",
			top, bottom)
	}
}

// -----------------------------------------------------------------------------
// Existing behavior preserved: combo/VE/star keep
// -----------------------------------------------------------------------------

func TestChooseScry_StarCardAlwaysOnTop(t *testing.T) {
	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		StarCards: []string{"Sol Ring"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	star := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)
	filler := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{star, filler})
	if !scryContains(top, star) {
		t.Fatalf("star card should always be on top; top=%v bottom=%v", top, bottom)
	}
}

func TestChooseScry_Cuttable_AlwaysBottomed(t *testing.T) {
	sp := &StrategyProfile{
		Archetype:     ArchetypeMidrange,
		CuttableCards: []string{"Bad Card"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	bad := newTestCardMinimal("Bad Card", []string{"sorcery"}, 3, nil)
	good := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)

	top, bottom := h.ChooseScry(gs, 0, []*gameengine.Card{bad, good})
	if !scryContains(bottom, bad) {
		t.Fatalf("cuttable should be bottomed; top=%v bottom=%v", top, bottom)
	}
}
