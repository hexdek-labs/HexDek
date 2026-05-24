package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// mulligan_r60_test.go — regressions for the R60 mulligan additions:
//
//   1. Opponent-combo-aware tightening in ChooseMulligan — when any
//      opponent's commander reads like a combo win condition, a
//      marginal hand with no interaction and no engine should be
//      mulliganed rather than kept.
//
//   2. ChooseBottomCards combo bias + 2-land floor — bottoming
//      respects the same combo/star/cuttable weights as ChooseDiscard,
//      and never bottoms into a sub-2-land keep when ≥2 lands were
//      available.
//
// Helpers (newTestGame, newTestCardMinimal, newTestPermanent,
// cardWithKeyword, cardWithStaticText) come from hat_test.go /
// graveyard_recursion_test.go.

// cardWithRawOracle builds a minimal card whose oracle text is the
// supplied raw string, exposed via a single Static AST node so
// OracleTextLower picks it up.
func cardWithRawOracle(name string, types []string, cmc int, raw string) *gameengine.Card {
	ast := &gameast.CardAST{Name: name}
	ast.Abilities = append(ast.Abilities, &gameast.Static{Raw: raw})
	return newTestCardMinimal(name, types, cmc, ast)
}

// landNamed returns a basic land card with the given subtype.
func landNamed(name string) *gameengine.Card {
	return newTestCardMinimal(name, []string{"land", "basic"}, 0, nil)
}

// putCommanderInZone places `cmd` in seat's command zone and records
// its name in seat.CommanderNames.
func putCommanderInZone(seat *gameengine.Seat, cmd *gameengine.Card) {
	seat.CommandZone = append(seat.CommandZone, cmd)
	seat.CommanderNames = append(seat.CommanderNames, cmd.DisplayName())
}

// -----------------------------------------------------------------------------
// someOpponentLooksCombo helper
// -----------------------------------------------------------------------------

func TestSomeOpponentLooksCombo_DetectsInfiniteText(t *testing.T) {
	gs := newTestGame(t, 2)
	cmd := cardWithRawOracle("Niv-Mizzet, Parun",
		[]string{"creature", "legendary"}, 6,
		"Whenever you draw a card, Niv-Mizzet, Parun deals 1 damage to any target. Whenever a player casts an instant or sorcery spell, you may create infinite copies.")
	putCommanderInZone(gs.Seats[1], cmd)

	if !someOpponentLooksCombo(gs, 0) {
		t.Fatal("expected opponent's 'infinite' commander oracle text to trigger combo detection")
	}
}

func TestSomeOpponentLooksCombo_DetectsExtraTurn(t *testing.T) {
	gs := newTestGame(t, 2)
	cmd := cardWithRawOracle("Narset, Enlightened Master",
		[]string{"creature", "legendary"}, 6,
		"Whenever Narset attacks, exile the top four cards of your library, then you may cast a noncreature spell from among them. Take an extra turn after this one.")
	putCommanderInZone(gs.Seats[1], cmd)

	if !someOpponentLooksCombo(gs, 0) {
		t.Fatal("expected 'extra turn' to trigger combo detection")
	}
}

func TestSomeOpponentLooksCombo_IgnoresSelfSeat(t *testing.T) {
	gs := newTestGame(t, 2)
	// OUR commander has combo-flavor text; we shouldn't flag ourselves.
	cmd := cardWithRawOracle("Krark-Clan Ironworks",
		[]string{"creature", "legendary"}, 4,
		"Win the game if you control infinite mana.")
	putCommanderInZone(gs.Seats[0], cmd)

	if someOpponentLooksCombo(gs, 0) {
		t.Fatal("someOpponentLooksCombo must not flag the asking seat's own commander")
	}
}

func TestSomeOpponentLooksCombo_VanillaCommanderIsNotCombo(t *testing.T) {
	gs := newTestGame(t, 2)
	cmd := cardWithRawOracle("Grizzly Beargolem",
		[]string{"creature", "legendary"}, 3,
		"At the beginning of your upkeep, draw a card.")
	putCommanderInZone(gs.Seats[1], cmd)

	if someOpponentLooksCombo(gs, 0) {
		t.Fatal("vanilla commander shouldn't trigger combo detection")
	}
}

func TestSomeOpponentLooksCombo_SkipsLostSeats(t *testing.T) {
	gs := newTestGame(t, 3)
	cmd := cardWithRawOracle("Niv-Mizzet, Reborn",
		[]string{"creature", "legendary"}, 6,
		"Whenever you cast an instant, take an extra turn.")
	putCommanderInZone(gs.Seats[1], cmd)
	gs.Seats[1].Lost = true

	if someOpponentLooksCombo(gs, 0) {
		t.Fatal("lost opponents shouldn't contribute to combo threat")
	}
}

// -----------------------------------------------------------------------------
// handHasInteraction helper
// -----------------------------------------------------------------------------

func TestHandHasInteraction_FindsRemoval(t *testing.T) {
	hand := []*gameengine.Card{
		newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil),
		cardWithRawOracle("Swords to Plowshares", []string{"instant"}, 1,
			"Exile target creature. Its controller gains life equal to its power."),
	}
	if !handHasInteraction(hand) {
		t.Fatal("expected exile-target removal to be detected as interaction")
	}
}

func TestHandHasInteraction_FindsCounterspell(t *testing.T) {
	hand := []*gameengine.Card{
		cardWithRawOracle("Counterspell", []string{"instant"}, 2, "Counter target spell."),
	}
	if !handHasInteraction(hand) {
		t.Fatal("expected counterspell to be detected as interaction")
	}
}

func TestHandHasInteraction_VanillaHandHasNone(t *testing.T) {
	hand := []*gameengine.Card{
		newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil),
		newTestCardMinimal("Forest", []string{"land", "basic"}, 0, nil),
	}
	if handHasInteraction(hand) {
		t.Fatal("vanilla hand should not register as interaction")
	}
}

// -----------------------------------------------------------------------------
// ChooseMulligan: combo-threat tightening
// -----------------------------------------------------------------------------

func TestChooseMulligan_VsCombo_MulligansMarginalHandWithoutInteractionOrEngine(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	// Opponent has a combo commander.
	cmd := cardWithRawOracle("Thrasios, Triton Hero",
		[]string{"creature", "legendary"}, 2,
		"{4}: Scry 1, then reveal the top card of your library. If it's a land, put it onto the battlefield tapped. Otherwise, draw a card. Take an extra turn after this one.")
	putCommanderInZone(gs.Seats[1], cmd)

	// Hand: 3 lands, 4 mid-range fliers — no removal, no counters, no
	// star cards. The default "any-archetype" keep path would keep this
	// (lands>=2 + no veCount/starCount), but with a combo opponent we
	// want to dig for a wrath/counter/clock.
	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
	}
	if !h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("interaction-less / engine-less 7 vs a combo opponent should mulligan")
	}
}

func TestChooseMulligan_VsCombo_KeepsHandWithInteraction(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	cmd := cardWithRawOracle("Combo Commander", []string{"creature", "legendary"}, 4,
		"At the beginning of your upkeep, take an extra turn.")
	putCommanderInZone(gs.Seats[1], cmd)

	counter := cardWithRawOracle("Counterspell", []string{"instant"}, 2, "Counter target spell.")
	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		counter,
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
	}
	if h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("hand with a counterspell should not be mulliganed against combo opponent")
	}
}

func TestChooseMulligan_NoComboOpponent_KeepsMarginalHand(t *testing.T) {
	// Baseline: same marginal hand as the combo-tightening test, but
	// no combo opponent. Pre-r60 behavior keeps the hand and that
	// behavior must not change.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	cmd := cardWithRawOracle("Vanilla Commander", []string{"creature", "legendary"}, 4,
		"At the beginning of your upkeep, draw a card.")
	putCommanderInZone(gs.Seats[1], cmd)

	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
	}
	if h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("marginal but reasonable hand against non-combo opponent should not be mulliganed")
	}
}

// -----------------------------------------------------------------------------
// ChooseBottomCards: combo bias + land floor
// -----------------------------------------------------------------------------

func TestChooseBottomCards_RespectsLandFloorOfTwo(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	// Hand: 2 lands + 5 spells. Bottoming 2 (the London-mulligan-down-1
	// case starts at 6 of 7 in a 7→6 mull). The two lowest-heuristic
	// cards might include one of the lands; the floor must protect both
	// lands.
	land1 := landNamed("Forest")
	land2 := landNamed("Mountain")
	weakSpell1 := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	weakSpell2 := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)
	weakSpell3 := newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil)
	weakSpell4 := newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil)
	weakSpell5 := newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil)

	hand := []*gameengine.Card{land1, land2, weakSpell1, weakSpell2, weakSpell3, weakSpell4, weakSpell5}
	bottoms := h.ChooseBottomCards(gs, 0, hand, 2)

	// Neither land should end up in the bottom pile.
	for _, b := range bottoms {
		if b == land1 || b == land2 {
			t.Fatalf("land floor violated: bottomed %s with only 2 lands in hand",
				b.DisplayName())
		}
	}
	if len(bottoms) != 2 {
		t.Fatalf("expected 2 bottoms, got %d", len(bottoms))
	}
}

func TestChooseBottomCards_LandFloorYieldsToSupply(t *testing.T) {
	// If the hand has fewer than 2 lands to begin with, the floor is
	// moot — we can't materialize supply we don't have. Verify the
	// guard doesn't loop forever or panic.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	hand := []*gameengine.Card{
		landNamed("Forest"),
		newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil),
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
		newTestCardMinimal("Hulking Devil", []string{"creature"}, 6, nil),
	}
	bottoms := h.ChooseBottomCards(gs, 0, hand, 2)
	if len(bottoms) != 2 {
		t.Fatalf("expected 2 bottoms, got %d", len(bottoms))
	}
}

func TestChooseBottomCards_KeepsComboPieceOverVanilla(t *testing.T) {
	// Combo pieces should outrank vanilla creatures for the keep pile.
	// We seed the StrategyProfile's combo list so isComboRelevant fires.
	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	combo := newTestCardMinimal("Thassa's Oracle", []string{"creature"}, 2, nil)
	vanilla := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	// 2 lands so the land floor doesn't kick in.
	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Island"),
		combo, vanilla,
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
	}
	bottoms := h.ChooseBottomCards(gs, 0, hand, 1)
	if len(bottoms) != 1 {
		t.Fatalf("expected 1 bottom, got %d", len(bottoms))
	}
	if bottoms[0] == combo {
		t.Fatalf("combo piece should never be bottomed when a vanilla card is available; got %s",
			bottoms[0].DisplayName())
	}
}

func TestChooseBottomCards_BottomsCuttableBeforeNeutralCard(t *testing.T) {
	sp := &StrategyProfile{
		Archetype:     ArchetypeMidrange,
		CuttableCards: []string{"Hill Giant"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	cuttable := newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil)
	keep := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)

	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Island"),
		cuttable, keep,
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
	}
	bottoms := h.ChooseBottomCards(gs, 0, hand, 1)
	if len(bottoms) != 1 || bottoms[0] != cuttable {
		t.Fatalf("cuttable should be the first bottom; got %v", bottoms)
	}
}
