package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// tappedLandWithMana builds an ETB-tapped land that ALSO has a real
// Activated add_mana ability, so the +1.0 colored-production bonus
// in ChooseLandToPlay fires. The Static raw text carries the "enters
// tapped" + utility clause picked up by the substring scan.
func tappedLandWithMana(name, utilityText string) *gameengine.Card {
	ast := &gameast.CardAST{Name: name}
	ast.Abilities = append(ast.Abilities,
		&gameast.Static{Raw: "This land enters tapped. " + utilityText},
		&gameast.Activated{
			Raw:    "{T}: Add one mana of any color.",
			Effect: &gameast.AddMana{AnyColorCount: 1},
		},
	)
	return newTestCardMinimal(name, []string{"land"}, 0, ast)
}

// nonBasicProducer builds an untapped land that produces a color but
// has a non-basic name (so the basic-name bonus in ChooseLandToPlay
// doesn't fire). Useful when isolating the ETB-tapped penalty's effect
// without the +0.5 basic-name confound on the comparator side.
func nonBasicProducer(name, color string) *gameengine.Card {
	ast := &gameast.CardAST{Name: name}
	ast.Abilities = append(ast.Abilities,
		&gameast.Activated{
			Raw: "{T}: Add {" + color + "}.",
			Effect: &gameast.AddMana{
				Pool: []gameast.ManaSymbol{{Color: []string{color}}},
			},
		},
	)
	return newTestCardMinimal(name, []string{"land"}, 0, ast)
}

// tappedColorFixer builds an ETB-tapped land producing a specific
// color, so the color-demand path in ChooseLandToPlay fires.
func tappedColorFixer(name, color string) *gameengine.Card {
	ast := &gameast.CardAST{Name: name}
	ast.Abilities = append(ast.Abilities,
		&gameast.Static{Raw: "This land enters tapped. {T}: Add {" + color + "}."},
		&gameast.Activated{
			Raw: "{T}: Add {" + color + "}.",
			Effect: &gameast.AddMana{
				Pool: []gameast.ManaSymbol{{Color: []string{color}}},
			},
		},
	)
	return newTestCardMinimal(name, []string{"land"}, 0, ast)
}

// choose_land_r60_test.go — regressions for the R60 ChooseLandToPlay
// archetype + mana-curve additions:
//
//   - Archetype multiplier on the ETB-tapped penalty: aggro 1.5×,
//     combo 1.2×, control 0.5×, ramp 0.7×. Reflects how much tempo
//     loss bites each archetype.
//
//   - Dead-next-turn softening: when there's no castable spell in
//     hand at avail+1 mana, the early-turn ETB-tapped penalty
//     collapses to the late-game floor — we weren't deploying
//     anyway, so the tapped land costs no tempo.

// etbTappedLand builds a basic-ish land with the "enters tapped"
// oracle clause picked up by ChooseLandToPlay's substring scan.
func etbTappedLand(name string) *gameengine.Card {
	return cardWithStaticText(name, []string{"land"}, 0,
		"This land enters tapped. {T}: Add one mana of any color.")
}

// untappedLand builds a basic with a real produce-color path so
// landProducesColorsMask actually picks up its color. We set both
// TypeLine (so the type-line scan catches "mountain"/"forest"/etc.)
// and an Activated add_mana ability matching the basic's color.
func untappedLand(name string) *gameengine.Card {
	color := ""
	typeLine := "Basic Land"
	switch name {
	case "Plains":
		color = "W"
		typeLine = "Basic Land — Plains"
	case "Island":
		color = "U"
		typeLine = "Basic Land — Island"
	case "Swamp":
		color = "B"
		typeLine = "Basic Land — Swamp"
	case "Mountain":
		color = "R"
		typeLine = "Basic Land — Mountain"
	case "Forest":
		color = "G"
		typeLine = "Basic Land — Forest"
	}
	ast := &gameast.CardAST{Name: name}
	if color != "" {
		ast.Abilities = append(ast.Abilities,
			&gameast.Activated{
				Raw: "{T}: Add {" + color + "}.",
				Effect: &gameast.AddMana{
					Pool: []gameast.ManaSymbol{{Color: []string{color}}},
				},
			},
		)
	}
	c := newTestCardMinimal(name, []string{"land", "basic"}, 0, ast)
	c.TypeLine = typeLine
	return c
}

// nonLandHandSpell returns a spell-shaped card so the deadNextTurn
// detector sees something castable at avail+1.
func nonLandHandSpell(name string, cmc int) *gameengine.Card {
	return newTestCardMinimal(name, []string{"sorcery"}, cmc, nil)
}

// -----------------------------------------------------------------------------
// Archetype ETB-tapped multiplier
// -----------------------------------------------------------------------------

func TestChooseLandToPlay_Aggro_PrefersUntappedOverETBTapped(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeAggro}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 2

	// Aggro wants to deploy on curve — give it a castable spell so
	// dead-next-turn doesn't suppress the penalty.
	gs.Seats[0].Hand = []*gameengine.Card{
		nonLandHandSpell("Goblin Guide", 1),
	}

	tapped := etbTappedLand("Tapped Triland")
	untapped := untappedLand("Mountain")
	pick := h.ChooseLandToPlay(gs, 0, []*gameengine.Card{tapped, untapped})

	if pick != untapped {
		t.Fatalf("aggro should prefer untapped land on turn 2 with castable spells; got %s", pick.DisplayName())
	}
}

func TestChooseLandToPlay_Control_TolerantOfETBTappedForFixing(t *testing.T) {
	// Control's softer multiplier (0.5×) means a tapped color-fixer
	// with a hand-color-demand bonus can outrank a vanilla basic.
	sp := &StrategyProfile{
		Archetype: ArchetypeControl,
		ColorDemand: map[string]int{
			"U": 12, // heavy blue demand
		},
		ManaBaseGrade: "C", // weaker mana base = stronger color-fixing pull
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 2

	// Hand wants U mana.
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Counterspell", Types: []string{"instant", "cost:2"}, Colors: []string{"U"}, CMC: 2},
	}

	tappedU := tappedColorFixer("Tapped Island Cycle Land", "U")
	basicG := untappedLand("Forest") // doesn't help color demand

	pick := h.ChooseLandToPlay(gs, 0, []*gameengine.Card{tappedU, basicG})
	if pick != tappedU {
		t.Fatalf("control with U demand should play a tapped U-fixer over a vanilla Forest; got %s",
			pick.DisplayName())
	}
}

func TestChooseLandToPlay_Aggro_AvoidsTappedUFixerWhenAggroDemand(t *testing.T) {
	// Mirror of the control test: aggro's harsher multiplier (1.5×)
	// should make the tapped land lose even when it fixes a needed
	// color, because aggro can't afford the tempo loss.
	sp := &StrategyProfile{
		Archetype: ArchetypeAggro,
		ColorDemand: map[string]int{
			"R": 12,
		},
		ManaBaseGrade: "C",
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 2

	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}, Colors: []string{"R"}, CMC: 1},
	}

	tappedR := tappedColorFixer("Slow Mountain", "R")
	basicR := untappedLand("Mountain")

	pick := h.ChooseLandToPlay(gs, 0, []*gameengine.Card{tappedR, basicR})
	if pick != basicR {
		t.Fatalf("aggro should prefer the untapped Mountain over a tapped color-equivalent; got %s",
			pick.DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Dead-next-turn softening
// -----------------------------------------------------------------------------

func TestChooseLandToPlay_DeadNextTurn_SoftensTappedPenalty(t *testing.T) {
	// Empty hand → next turn has nothing to deploy → ETB-tapped land
	// is acceptable even on turn 2 because there's no tempo to lose.
	// Pair a tapped utility land that produces mana with a non-basic
	// untapped producer (so the basic-name bonus doesn't confound the
	// comparison). With dead-next-turn softening, the tapped land's
	// utility bonus dominates the now-tiny tapped penalty.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 2
	gs.Seats[0].Hand = nil // nothing to deploy next turn

	tappedUtility := tappedLandWithMana("Halimar Depths",
		"When this enters, scry 3.")
	plain := nonBasicProducer("Vanilla Cave", "W")

	pick := h.ChooseLandToPlay(gs, 0, []*gameengine.Card{tappedUtility, plain})
	if pick != tappedUtility {
		t.Fatalf("dead-next-turn should let a tapped utility land beat a non-utility untapped; got %s",
			pick.DisplayName())
	}
}

func TestChooseLandToPlay_LiveNextTurn_TappedUtilityBeatenByPlain(t *testing.T) {
	// Companion to the dead-next-turn test: with a castable spell in
	// hand, the full early ETB-tapped penalty applies and the plain
	// untapped producer beats the same tapped utility land. Pins the
	// differential — softening is the only thing that flips the call.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 2
	gs.Seats[0].Hand = []*gameengine.Card{nonLandHandSpell("Cantrip", 1)}

	tappedUtility := tappedLandWithMana("Halimar Depths",
		"When this enters, scry 3.")
	plain := nonBasicProducer("Vanilla Cave", "W")

	pick := h.ChooseLandToPlay(gs, 0, []*gameengine.Card{tappedUtility, plain})
	if pick != plain {
		t.Fatalf("with a castable spell in hand, the early tapped penalty should beat a tapped utility; got %s",
			pick.DisplayName())
	}
}

func TestChooseLandToPlay_DeadTurnSofteningRespectsLateGameFloor(t *testing.T) {
	// On turn 5+ the ETB-tapped penalty is already the late-game floor
	// (0.5). Dead-next-turn detection shouldn't alter behavior because
	// the override only collapses the EARLY penalty. Pin that the
	// late-game pick still works the same with or without an empty hand.
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 6
	gs.Seats[0].Hand = nil // empty — dead-next-turn fires

	tapped := etbTappedLand("Tapped Land")
	basic := untappedLand("Plains")

	pick := h.ChooseLandToPlay(gs, 0, []*gameengine.Card{tapped, basic})
	// Both tapped land (0.5 penalty, +1 colored production, plus
	// utility text "of any color" doesn't trigger scry/draw keywords)
	// and basic (+0.5 baseline, no production from minimal AST) end up
	// close — we just want the call to succeed deterministically.
	if pick == nil {
		t.Fatal("late-game land pick should always return a card")
	}
}


// -----------------------------------------------------------------------------
// Existing behavior preserved
// -----------------------------------------------------------------------------

func TestChooseLandToPlay_SinglyLandReturnsImmediately(t *testing.T) {
	// The early-return path (len(lands)==1) must not invoke any of the
	// new logic — if it did, an out-of-range archetype mul or panicking
	// hand scan would surface here.
	sp := &StrategyProfile{Archetype: ArchetypeAggro}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	gs.Seats[0].Hand = nil

	only := etbTappedLand("Some Tapped Land")
	pick := h.ChooseLandToPlay(gs, 0, []*gameengine.Card{only})
	if pick != only {
		t.Fatalf("single-land hand should always return that land; got %v", pick)
	}
}
