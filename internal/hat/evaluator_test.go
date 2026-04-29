package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestEvaluator_LostSeatReturnsNeg1(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Lost = true
	ev := NewEvaluator(nil)
	if s := ev.Evaluate(gs, 0); s != -1 {
		t.Errorf("lost seat should be -1, got %f", s)
	}
}

func TestEvaluator_WonSeatReturnsPos1(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Won = true
	ev := NewEvaluator(nil)
	if s := ev.Evaluate(gs, 0); s != 1 {
		t.Errorf("won seat should be 1, got %f", s)
	}
}

func TestEvaluator_BoardAdvantage(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Seat 0 has big board, seat 1 has nothing.
	for i := 0; i < 3; i++ {
		c := newTestCardMinimal("Bear", []string{"creature"}, 2, nil)
		newTestPermanent(gs.Seats[0], c, 3, 3)
	}

	ev := NewEvaluator(nil)
	s0 := ev.Evaluate(gs, 0)
	s1 := ev.Evaluate(gs, 1)

	if s0 <= 0 {
		t.Errorf("seat 0 with 9 power should be positive, got %f", s0)
	}
	if s1 >= 0 {
		t.Errorf("seat 1 with 0 power facing 9 should be negative, got %f", s1)
	}
	if s0 <= s1 {
		t.Errorf("board advantage seat (%.3f) should score higher than empty (%.3f)", s0, s1)
	}
}

func TestEvaluator_ArchetypeDifferentiation(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Give seat 0 a decent board.
	for i := 0; i < 4; i++ {
		c := newTestCardMinimal("Soldier", []string{"creature"}, 2, nil)
		newTestPermanent(gs.Seats[0], c, 2, 2)
	}
	// Give seat 0 some hand cards.
	for i := 0; i < 5; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, newTestCardMinimal("Card", []string{"instant"}, 1, nil))
	}
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, newTestCardMinimal("Card", []string{"instant"}, 1, nil))

	aggroEv := NewEvaluator(&StrategyProfile{Archetype: ArchetypeAggro})
	controlEv := NewEvaluator(&StrategyProfile{Archetype: ArchetypeControl})

	aggroScore := aggroEv.EvaluateDetailed(gs, 0)
	controlScore := controlEv.EvaluateDetailed(gs, 0)

	// Aggro weights board presence at 1.5, control at 0.5.
	// With a strong board, aggro's weighted board contribution should be higher.
	aggroBoardContrib := aggroScore.BoardPresence * DefaultWeightsForArchetype(ArchetypeAggro).BoardPresence
	controlBoardContrib := controlScore.BoardPresence * DefaultWeightsForArchetype(ArchetypeControl).BoardPresence
	if aggroBoardContrib <= controlBoardContrib {
		t.Errorf("aggro board contribution (%.3f) should exceed control (%.3f)",
			aggroBoardContrib, controlBoardContrib)
	}

	// Control weights card advantage at 1.5, aggro at 0.4.
	controlCardContrib := controlScore.CardAdvantage * DefaultWeightsForArchetype(ArchetypeControl).CardAdvantage
	aggroCardContrib := aggroScore.CardAdvantage * DefaultWeightsForArchetype(ArchetypeAggro).CardAdvantage
	if controlCardContrib <= aggroCardContrib {
		t.Errorf("control card advantage contribution (%.3f) should exceed aggro (%.3f)",
			controlCardContrib, aggroCardContrib)
	}
}

func TestEvaluator_ComboProximity(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
	}
	ev := NewEvaluator(sp)

	// No pieces: combo proximity should be 0.
	r0 := ev.EvaluateDetailed(gs, 0)
	if r0.ComboProximity != 0 {
		t.Errorf("no combo pieces should be 0 proximity, got %f", r0.ComboProximity)
	}

	// One piece in hand.
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		newTestCardMinimal("Thassa's Oracle", []string{"creature"}, 2, nil))
	r1 := ev.EvaluateDetailed(gs, 0)
	if r1.ComboProximity <= 0 {
		t.Errorf("1/2 combo pieces should have positive proximity, got %f", r1.ComboProximity)
	}

	// Both pieces available.
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		newTestCardMinimal("Demonic Consultation", []string{"instant"}, 1, nil))
	r2 := ev.EvaluateDetailed(gs, 0)
	if r2.ComboProximity <= r1.ComboProximity {
		t.Errorf("2/2 pieces (%.3f) should exceed 1/2 (%.3f)", r2.ComboProximity, r1.ComboProximity)
	}
	if r2.ComboProximity < 1.5 {
		t.Errorf("full combo should score at least 1.5, got %f", r2.ComboProximity)
	}
}

func TestEvaluator_LowLifeThreat(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 5
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Opponent has lethal board.
	c := newTestCardMinimal("Dragon", []string{"creature"}, 6, nil)
	newTestPermanent(gs.Seats[1], c, 8, 8)

	ev := NewEvaluator(nil)
	s := ev.Evaluate(gs, 0)
	if s >= 0 {
		t.Errorf("5 life facing 8 power should be negative, got %f", s)
	}
}

func TestEvaluator_CommanderDamageProgress(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.CommanderFormat = true
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40
	gs.Seats[0].CommanderNames = []string{"Korvold"}

	// Put commander on battlefield.
	cmdCard := newTestCardMinimal("Korvold", []string{"creature"}, 5,
		&gameast.CardAST{Name: "Korvold"})
	newTestPermanent(gs.Seats[0], cmdCard, 4, 4)

	// 14 commander damage dealt to seat 1.
	gs.Seats[1].CommanderDamage = map[int]map[string]int{
		0: {"Korvold": 14},
	}

	ev := NewEvaluator(nil)
	r := ev.EvaluateDetailed(gs, 0)
	if r.CommanderProgress <= 0 {
		t.Errorf("commander on field + 14 damage should be positive, got %f", r.CommanderProgress)
	}
}

func TestEvaluator_GraveyardValue(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Graveyard with flashback cards is more valuable.
	for i := 0; i < 5; i++ {
		ast := &gameast.CardAST{Name: "Lingering Souls"}
		ast.Abilities = append(ast.Abilities, &gameast.Keyword{Name: "flashback"})
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal("Lingering Souls", []string{"sorcery"}, 3, ast))
	}

	ev := NewEvaluator(nil)
	r := ev.EvaluateDetailed(gs, 0)
	if r.GraveyardValue <= 0 {
		t.Errorf("5 flashback cards in yard should be positive value, got %f", r.GraveyardValue)
	}
}

func TestEvaluator_OutputRange(t *testing.T) {
	gs := newTestGame(t, 4)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}

	ev := NewEvaluator(nil)
	for i := range gs.Seats {
		s := ev.Evaluate(gs, i)
		if s < -1 || s > 1 {
			t.Errorf("seat %d score %.3f outside [-1, 1]", i, s)
		}
	}
}

func TestEvaluator_TanhNormalization(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Even with extreme board advantage, score stays in [-1, 1].
	for i := 0; i < 20; i++ {
		c := newTestCardMinimal("Giant", []string{"creature"}, 5, nil)
		newTestPermanent(gs.Seats[0], c, 10, 10)
	}

	ev := NewEvaluator(nil)
	s := ev.Evaluate(gs, 0)
	if s <= 0 || s > 1 {
		t.Errorf("extreme board: expected (0, 1], got %f", s)
	}
	if math.Abs(s) > 1.0 {
		t.Errorf("tanh should clamp to [-1, 1], got %f", s)
	}
}

func TestDefaultWeightsForArchetype_UnknownFallsToMidrange(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	unk := DefaultWeightsForArchetype("some_unknown_archetype")
	if mid != unk {
		t.Errorf("unknown archetype should return midrange weights")
	}
}

func TestDefaultWeightsForArchetype_AllDefined(t *testing.T) {
	for _, arch := range []string{ArchetypeAggro, ArchetypeCombo, ArchetypeControl, ArchetypeMidrange, ArchetypeRamp} {
		w := DefaultWeightsForArchetype(arch)
		if w.BoardPresence == 0 && w.CardAdvantage == 0 && w.ManaAdvantage == 0 {
			t.Errorf("archetype %q has all-zero weights", arch)
		}
	}
}

func TestEvaluator_ManaAdvantage(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Seat 0 has 6 lands, seat 1 has 2.
	for i := 0; i < 6; i++ {
		c := newTestCardMinimal("Forest", []string{"land"}, 0, nil)
		newTestPermanent(gs.Seats[0], c, 0, 0)
	}
	for i := 0; i < 2; i++ {
		c := newTestCardMinimal("Forest", []string{"land"}, 0, nil)
		newTestPermanent(gs.Seats[1], c, 0, 0)
	}

	rampEv := NewEvaluator(&StrategyProfile{Archetype: ArchetypeRamp})
	aggroEv := NewEvaluator(&StrategyProfile{Archetype: ArchetypeAggro})

	rampScore := rampEv.EvaluateDetailed(gs, 0)
	aggroScore := aggroEv.EvaluateDetailed(gs, 0)

	rampManaContrib := rampScore.ManaAdvantage * DefaultWeightsForArchetype(ArchetypeRamp).ManaAdvantage
	aggroManaContrib := aggroScore.ManaAdvantage * DefaultWeightsForArchetype(ArchetypeAggro).ManaAdvantage
	if rampManaContrib <= aggroManaContrib {
		t.Errorf("ramp mana contribution (%.3f) should exceed aggro (%.3f)",
			rampManaContrib, aggroManaContrib)
	}
}
