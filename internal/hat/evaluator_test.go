package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
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

// TestEvaluator_ComboProximity_ClassAffinity verifies that scoreCombo
// damps off-class plans relative to plans matching the deck's primary
// combo class. Scenario: a deck with three drain-class plans and one
// off-class infinite_token plan. Holding pieces of a drain plan should
// outscore holding equivalent piece-count of the infinite_token plan.
//
// Why the test matters: prior to R60, scoreCombo took max(ratio) across
// all plans regardless of class. A deck whose primary line is drain
// would score an opportunistic Kiki-Jiki sighting (1/2 of the token
// plan) equal to a Sanguine Bond sighting (1/2 of a drain plan). Class
// affinity makes the drain piece score higher, biasing decisions toward
// the deck's intended line.
func TestEvaluator_ComboProximity_ClassAffinity(t *testing.T) {
	mkGame := func() *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Seats[0].Life = 40
		gs.Seats[0].StartingLife = 40
		gs.Seats[1].Life = 40
		gs.Seats[1].StartingLife = 40
		return gs
	}

	// Deck shape: three drain plans + one off-class token plan. By
	// PrimaryComboClass() math, infinite_drain is the dominant class
	// (count=3 vs token=1).
	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Exquisite Blood", "Sanguine Bond"}, Type: "infinite", Class: "infinite_drain"},
			{Pieces: []string{"Exquisite Blood", "Vito, Thorn of the Dusk Rose"}, Type: "infinite", Class: "infinite_drain"},
			{Pieces: []string{"Exquisite Blood", "Defiant Bloodlord"}, Type: "infinite", Class: "infinite_drain"},
			{Pieces: []string{"Kiki-Jiki, Mirror Breaker", "Zealous Conscripts"}, Type: "infinite", Class: "infinite_tokens"},
		},
	}
	if got := sp.PrimaryComboClass(); got != "infinite_drain" {
		t.Fatalf("PrimaryComboClass: want infinite_drain, got %q", got)
	}
	ev := NewEvaluator(sp)

	// On-class scenario: hold one drain piece (Sanguine Bond).
	gsOn := mkGame()
	gsOn.Seats[0].Hand = append(gsOn.Seats[0].Hand,
		newTestCardMinimal("Sanguine Bond", []string{"enchantment"}, 5, nil))
	onClass := ev.EvaluateDetailed(gsOn, 0).ComboProximity

	// Off-class scenario: hold one token piece (Kiki-Jiki).
	gsOff := mkGame()
	gsOff.Seats[0].Hand = append(gsOff.Seats[0].Hand,
		newTestCardMinimal("Kiki-Jiki, Mirror Breaker", []string{"creature"}, 5, nil))
	offClass := ev.EvaluateDetailed(gsOff, 0).ComboProximity

	if onClass <= offClass {
		t.Errorf("on-class drain piece (%.3f) should outscore off-class token piece (%.3f)",
			onClass, offClass)
	}
	// Damping factor is 0.7 in evaluator.go; the on/off ratio should be
	// roughly 1/0.7 ≈ 1.43. Allow some slack but reject pathological
	// drift (no damping = 1.0, full block = 0.0).
	if ratio := onClass / offClass; ratio < 1.3 || ratio > 1.6 {
		t.Errorf("on/off ratio %.3f outside expected damping band [1.3, 1.6]", ratio)
	}
}

// TestStrategyProfile_PrimaryComboClass covers the tie-break and
// fallback paths for PrimaryComboClass — every-class-unique returns
// empty, classless plans return empty, the dominant class wins.
func TestStrategyProfile_PrimaryComboClass(t *testing.T) {
	cases := []struct {
		name  string
		plans []ComboPlan
		want  string
	}{
		{
			"empty profile",
			nil,
			"",
		},
		{
			"single plan — no dominant class possible",
			[]ComboPlan{{Pieces: []string{"A", "B"}, Class: "infinite_drain"}},
			"",
		},
		{
			"all plans share one class",
			[]ComboPlan{
				{Pieces: []string{"A", "B"}, Class: "infinite_mana"},
				{Pieces: []string{"C", "D"}, Class: "infinite_mana"},
				{Pieces: []string{"E", "F"}, Class: "infinite_mana"},
			},
			"infinite_mana",
		},
		{
			"three drain vs one token — drain wins",
			[]ComboPlan{
				{Pieces: []string{"A", "B"}, Class: "infinite_drain"},
				{Pieces: []string{"C", "D"}, Class: "infinite_drain"},
				{Pieces: []string{"E", "F"}, Class: "infinite_drain"},
				{Pieces: []string{"G", "H"}, Class: "infinite_tokens"},
			},
			"infinite_drain",
		},
		{
			"every plan in its own class — no dominant",
			[]ComboPlan{
				{Pieces: []string{"A", "B"}, Class: "infinite_mana"},
				{Pieces: []string{"C", "D"}, Class: "infinite_drain"},
				{Pieces: []string{"E", "F"}, Class: "infinite_tokens"},
			},
			"",
		},
		{
			"all plans classless — no dominant",
			[]ComboPlan{
				{Pieces: []string{"A", "B"}, Class: ""},
				{Pieces: []string{"C", "D"}, Class: ""},
			},
			"",
		},
	}
	for _, tc := range cases {
		sp := &StrategyProfile{ComboPieces: tc.plans}
		got := sp.PrimaryComboClass()
		if got != tc.want {
			t.Errorf("%s: want %q got %q", tc.name, tc.want, got)
		}
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

func TestEvaluator_PartnerSynergy(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.CommanderFormat = true
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Non-partner deck should score 0.
	gs.Seats[0].CommanderNames = []string{"Korvold"}
	ev := NewEvaluator(nil)
	r := ev.EvaluateDetailed(gs, 0)
	if r.PartnerSynergy != 0 {
		t.Errorf("single commander should have 0 partner synergy, got %f", r.PartnerSynergy)
	}

	// Partner pair with both on field.
	gs.Seats[0].CommanderNames = []string{"Kraum", "Tymna"}
	gs.Seats[0].CommanderCastCounts = map[string]int{"Kraum": 0, "Tymna": 0}
	kraum := newTestCardMinimal("Kraum", []string{"creature"}, 5,
		&gameast.CardAST{Name: "Kraum", Abilities: []gameast.Ability{
			&gameast.Triggered{Raw: "whenever an opponent casts a spell, draw a card"},
		}})
	kraum.Colors = []string{"U", "R"}
	tymna := newTestCardMinimal("Tymna", []string{"creature"}, 3,
		&gameast.CardAST{Name: "Tymna", Abilities: []gameast.Ability{
			&gameast.Triggered{Raw: "at the beginning of your postcombat, draw cards for each combat damage"},
		}})
	tymna.Colors = []string{"W", "B"}
	newTestPermanent(gs.Seats[0], kraum, 4, 4)
	newTestPermanent(gs.Seats[0], tymna, 2, 2)

	r2 := ev.EvaluateDetailed(gs, 0)
	if r2.PartnerSynergy <= 0 {
		t.Errorf("partner pair both on field should be positive, got %f", r2.PartnerSynergy)
	}
}

func TestEvaluator_ActivationTempo(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// No activations = 0.
	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	if r0.ActivationTempo != 0 {
		t.Errorf("empty board should have 0 activation tempo, got %f", r0.ActivationTempo)
	}

	// Add a permanent with an activated ability (not mana-only).
	c := newTestCardMinimal("Staff of Domination", []string{"artifact"}, 3,
		&gameast.CardAST{Name: "Staff of Domination", Abilities: []gameast.Ability{
			&gameast.Activated{Raw: "{1}: untap staff of domination"},
			&gameast.Activated{Raw: "{2}, {t}: draw a card"},
		}})
	newTestPermanent(gs.Seats[0], c, 0, 0)

	r1 := ev.EvaluateDetailed(gs, 0)
	if r1.ActivationTempo <= 0 {
		t.Errorf("Staff of Domination should give positive activation tempo, got %f", r1.ActivationTempo)
	}
}

func TestEvaluator_ToolboxBreadth(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Add tutors and modal spells to hand.
	tutor := newTestCardMinimal("Demonic Tutor", []string{"sorcery"}, 2,
		&gameast.CardAST{Name: "Demonic Tutor", Abilities: []gameast.Ability{
			&gameast.Activated{Raw: "search your library for a card and put it into your hand"},
		}})
	modal := newTestCardMinimal("Cryptic Command", []string{"instant"}, 4,
		&gameast.CardAST{Name: "Cryptic Command", Abilities: []gameast.Ability{
			&gameast.Activated{Raw: "choose two — counter target spell; return target permanent; tap all creatures; draw a card"},
		}})
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, tutor, modal)

	ev := NewEvaluator(nil)
	r := ev.EvaluateDetailed(gs, 0)
	if r.ToolboxBreadth <= 0 {
		t.Errorf("tutor + modal spell should give positive toolbox breadth, got %f", r.ToolboxBreadth)
	}
}

func TestEvaluator_ThreatTrajectory(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Opponent with big board + full hand + lots of mana = high trajectory threat.
	for i := 0; i < 3; i++ {
		c := newTestCardMinimal("Dragon", []string{"creature"}, 6, nil)
		newTestPermanent(gs.Seats[1], c, 5, 5)
	}
	for i := 0; i < 6; i++ {
		c := newTestCardMinimal("Forest", []string{"land"}, 0, nil)
		newTestPermanent(gs.Seats[1], c, 0, 0)
	}
	for i := 0; i < 5; i++ {
		gs.Seats[1].Hand = append(gs.Seats[1].Hand,
			newTestCardMinimal("Spell", []string{"creature"}, 3, nil))
	}

	ev := NewEvaluator(nil)
	r := ev.EvaluateDetailed(gs, 0)
	if r.ThreatTrajectory >= 0 {
		t.Errorf("heavy opponent board+hand should give negative threat trajectory, got %f", r.ThreatTrajectory)
	}
}

func TestEvaluator_DynamicRescaling(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	ev := NewEvaluator(nil)

	// Early game: mana advantage weight should be boosted.
	gs.Turn = 2
	wEarly := ev.rescaleWeights(gs, 0)

	gs.Turn = 20
	wLate := ev.rescaleWeights(gs, 0)

	if wEarly.ManaAdvantage <= wLate.ManaAdvantage {
		t.Errorf("early game mana weight (%.3f) should exceed late game (%.3f)",
			wEarly.ManaAdvantage, wLate.ManaAdvantage)
	}
	if wLate.ComboProximity <= wEarly.ComboProximity {
		t.Errorf("late game combo weight (%.3f) should exceed early game (%.3f)",
			wLate.ComboProximity, wEarly.ComboProximity)
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

// Freya classifies decks into archetype strings the hat constants didn't
// cover (lifegain, voltron, lands matter, ...). Before this fix these all
// silently fell back to midrange. The test pins each to its own profile by
// checking the dimension that should be the archetype's signature dial.
func TestDefaultWeightsForArchetype_FreyaArchetypesDistinct(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	cases := []struct {
		arch    string
		field   string
		got     func(EvalWeights) float64
		baseGot float64
		minVal  float64
	}{
		{ArchetypeLifegain, "LifeResource", func(w EvalWeights) float64 { return w.LifeResource }, mid.LifeResource, 1.5},
		{ArchetypeVoltron, "CommanderProgress", func(w EvalWeights) float64 { return w.CommanderProgress }, mid.CommanderProgress, 1.5},
		{ArchetypeLandsMatter, "ManaAdvantage", func(w EvalWeights) float64 { return w.ManaAdvantage }, mid.ManaAdvantage, 1.5},
		{ArchetypeCountersMatter, "BoardPresence", func(w EvalWeights) float64 { return w.BoardPresence }, mid.BoardPresence, 1.1},
		{ArchetypeMill, "ComboProximity", func(w EvalWeights) float64 { return w.ComboProximity }, mid.ComboProximity, 1.0},
		{ArchetypeStorm, "ComboProximity", func(w EvalWeights) float64 { return w.ComboProximity }, mid.ComboProximity, 1.5},
		{ArchetypeSuperfriends, "PlaneswalkerProgress", func(w EvalWeights) float64 { return w.PlaneswalkerProgress }, mid.PlaneswalkerProgress, 1.5},
		{ArchetypeBlink, "ActivationTempo", func(w EvalWeights) float64 { return w.ActivationTempo }, mid.ActivationTempo, 0.7},
		{ArchetypeExtraCombats, "BoardPresence", func(w EvalWeights) float64 { return w.BoardPresence }, mid.BoardPresence, 1.2},
	}
	for _, tc := range cases {
		w := DefaultWeightsForArchetype(tc.arch)
		if w == mid {
			t.Errorf("archetype %q returned midrange weights — missing profile", tc.arch)
		}
		if tc.got(w) < tc.minVal {
			t.Errorf("archetype %q: %s = %.2f, want >= %.2f", tc.arch, tc.field, tc.got(w), tc.minVal)
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
