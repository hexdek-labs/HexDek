package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// defend_vs_aggro_signals_r60_test.go — pins the R60 round 7+
// defensive-pressure signals added to cardHeuristic. Two complementary
// signals share the same defenseUrgencyVsAggro pressure threshold:
//   A. defensive cards (removal, sweepers, real blockers) get a bonus
//   B. expensive non-defensive cards get a penalty
// Both fire only when projected single-turn opp damage ≥ 33% of life.

// mkCreaturePerm builds a creature permanent on a seat with the given
// power/toughness. Used to construct opponent boards for the urgency
// computation.
func mkCreaturePerm(seat *gameengine.Seat, name string, power, toughness int, keywords ...string) *gameengine.Permanent {
	ast := &gameast.CardAST{Name: name}
	for _, kw := range keywords {
		ast.Abilities = append(ast.Abilities, &gameast.Keyword{Name: kw})
	}
	card := &gameengine.Card{
		Name:          name,
		AST:           ast,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
	}
	p := &gameengine.Permanent{
		Card:       card,
		Controller: seat.Idx,
		Owner:      seat.Idx,
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, p)
	return p
}

// -----------------------------------------------------------------------
// defenseUrgencyVsAggro
// -----------------------------------------------------------------------

func TestDefenseUrgency_NoOpponentBoardIsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if got := h.defenseUrgencyVsAggro(gs, 0); got != 0 {
		t.Errorf("empty opp board should be 0 urgency, got %f", got)
	}
}

func TestDefenseUrgency_ScalesWithProjectedDamage(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	// Opp has 10 power total — 10/40 = 0.25, below the 0.33 threshold.
	mkCreaturePerm(gs.Seats[1], "Bear A", 5, 5)
	mkCreaturePerm(gs.Seats[1], "Bear B", 5, 5)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	low := h.defenseUrgencyVsAggro(gs, 0)
	if low < 0.24 || low > 0.26 {
		t.Errorf("10/40 should be ~0.25 urgency; got %f", low)
	}
	// Add more pressure: 14 power total / 40 life = 0.35 — above threshold.
	mkCreaturePerm(gs.Seats[1], "Bear C", 4, 4)
	high := h.defenseUrgencyVsAggro(gs, 0)
	if high < 0.34 || high > 0.36 {
		t.Errorf("14/40 should be ~0.35 urgency; got %f", high)
	}
	if high <= low {
		t.Errorf("urgency should grow with more opp power: low=%f high=%f", low, high)
	}
}

func TestDefenseUrgency_DoubleStrikeCountsTwice(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	mkCreaturePerm(gs.Seats[1], "DS Bear", 5, 5, "double strike")
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	got := h.defenseUrgencyVsAggro(gs, 0)
	// 5 × 2 = 10 / 40 = 0.25
	if got < 0.24 || got > 0.26 {
		t.Errorf("DS power 5 should project 10 damage / 40 life = 0.25 urgency, got %f", got)
	}
}

func TestDefenseUrgency_CapsAtOne(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 10
	gs.Seats[1].Life = 40
	mkCreaturePerm(gs.Seats[1], "Big", 50, 50)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if got := h.defenseUrgencyVsAggro(gs, 0); got != 1.0 {
		t.Errorf("50 damage / 10 life should cap at 1.0, got %f", got)
	}
}

func TestDefenseUrgency_TakesMaxAcrossOpponents(t *testing.T) {
	gs := newTestGame(t, 4)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	// Seat 1: 4 power (low).
	mkCreaturePerm(gs.Seats[1], "Bear", 4, 4)
	// Seat 2: 20 power (lethal-ish).
	mkCreaturePerm(gs.Seats[2], "Huge", 20, 20)
	// Seat 3: 0 power.
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	got := h.defenseUrgencyVsAggro(gs, 0)
	// 20/40 = 0.5 (from seat 2 — the worst).
	if got < 0.49 || got > 0.51 {
		t.Errorf("urgency should reflect max-opp seat 2 (20/40 = 0.5), got %f", got)
	}
}

// -----------------------------------------------------------------------
// isDefensiveCard
// -----------------------------------------------------------------------

func TestIsDefensiveCard_RemovalViaCardRole(t *testing.T) {
	c := newTestCardMinimal("Doom Blade", []string{"instant"}, 2, nil)
	sp := &StrategyProfile{
		Archetype: ArchetypeMidrange,
		CardRoles: map[string]string{"Doom Blade": "Removal"},
	}
	h := NewYggdrasilHat(sp, 0)
	if !h.isDefensiveCard(c) {
		t.Error("CardRole=Removal should flag as defensive")
	}
}

func TestIsDefensiveCard_BoardWipeViaCardRole(t *testing.T) {
	c := newTestCardMinimal("Wrath of God", []string{"sorcery"}, 4, nil)
	sp := &StrategyProfile{
		Archetype: ArchetypeMidrange,
		CardRoles: map[string]string{"Wrath of God": "BoardWipe"},
	}
	h := NewYggdrasilHat(sp, 0)
	if !h.isDefensiveCard(c) {
		t.Error("CardRole=BoardWipe (also CatRemoval) should flag as defensive")
	}
}

func TestIsDefensiveCard_ToughBlockerByPT(t *testing.T) {
	c := newTestCardMinimal("Wall of Omens", []string{"creature"}, 2, nil)
	c.BasePower = 0
	c.BaseToughness = 4
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if !h.isDefensiveCard(c) {
		t.Error("creature with toughness ≥ 3 should flag as defensive")
	}
}

func TestIsDefensiveCard_KeywordReach(t *testing.T) {
	ast := &gameast.CardAST{
		Name:      "Spike-Worn",
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "reach"}},
	}
	c := newTestCardMinimal("Spike-Worn", []string{"creature"}, 2, ast)
	c.BasePower = 2
	c.BaseToughness = 2 // < 3 — only the keyword should rescue it
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if !h.isDefensiveCard(c) {
		t.Error("creature with reach (sub-3 toughness) should still flag defensive")
	}
}

func TestIsDefensiveCard_VanillaSmallCreatureNotDefensive(t *testing.T) {
	c := newTestCardMinimal("Goblin", []string{"creature"}, 1, nil)
	c.BasePower = 2
	c.BaseToughness = 1
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if h.isDefensiveCard(c) {
		t.Error("vanilla 2/1 creature should NOT flag as defensive")
	}
}

func TestIsDefensiveCard_RandomNonCreatureNonRemoval(t *testing.T) {
	c := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if h.isDefensiveCard(c) {
		t.Error("Sol Ring (artifact ramp) should NOT flag as defensive")
	}
}

// -----------------------------------------------------------------------
// Integration: cardHeuristic shifts under pressure
// -----------------------------------------------------------------------

// TestCardHeuristic_DefensiveBonusUnderPressure pins Signal A: when the
// urgency threshold is crossed, a defensive card's score rises relative
// to its no-pressure baseline. Compare the same card's score in two
// game states: opp empty vs opp with 16 power.
func TestCardHeuristic_DefensiveBonusUnderPressure(t *testing.T) {
	mkGS := func(oppPower int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Seats[0].Life = 40
		gs.Seats[1].Life = 40
		// Give the active hat 5 lands so cardHeuristic's mana-efficiency
		// term doesn't vary across the comparison.
		for i := 0; i < 5; i++ {
			land := newTestCardMinimal("Plains", []string{"land"}, 0, nil)
			newTestPermanent(gs.Seats[0], land, 0, 0)
		}
		if oppPower > 0 {
			mkCreaturePerm(gs.Seats[1], "Big", oppPower, oppPower)
		}
		return gs
	}

	doomBlade := newTestCardMinimal("Doom Blade", []string{"instant"}, 2, nil)
	sp := &StrategyProfile{
		Archetype: ArchetypeMidrange,
		CardRoles: map[string]string{"Doom Blade": "Removal"},
	}
	h := NewYggdrasilHat(sp, 0)

	baseline := h.cardHeuristic(mkGS(0), 0, doomBlade)
	pressured := h.cardHeuristic(mkGS(16), 0, doomBlade) // 16/40 = 0.40 urgency
	if pressured <= baseline {
		t.Errorf("Doom Blade should score HIGHER under pressure; baseline=%.3f pressured=%.3f",
			baseline, pressured)
	}
}

// TestCardHeuristic_ExpensiveNonDefensivePenaltyUnderPressure pins
// Signal B: a 7-mana value engine without defensive value should score
// LOWER under pressure.
func TestCardHeuristic_ExpensiveNonDefensivePenaltyUnderPressure(t *testing.T) {
	mkGS := func(oppPower int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Seats[0].Life = 40
		gs.Seats[1].Life = 40
		for i := 0; i < 7; i++ {
			land := newTestCardMinimal("Forest", []string{"land"}, 0, nil)
			newTestPermanent(gs.Seats[0], land, 0, 0)
		}
		if oppPower > 0 {
			mkCreaturePerm(gs.Seats[1], "Big", oppPower, oppPower)
		}
		return gs
	}

	// Expensive non-defensive: 7-CMC artifact (e.g. Mind's Eye). Not a
	// creature, no removal role, just card draw.
	mindsEye := newTestCardMinimal("Mind's Eye", []string{"artifact"}, 7, nil)
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHat(sp, 0)

	baseline := h.cardHeuristic(mkGS(0), 0, mindsEye)
	pressured := h.cardHeuristic(mkGS(16), 0, mindsEye) // 16/40 = 0.40 urgency
	if pressured >= baseline {
		t.Errorf("Mind's Eye should score LOWER under pressure; baseline=%.3f pressured=%.3f",
			baseline, pressured)
	}
}

// TestCardHeuristic_BelowThresholdNoBias pins the gating: a small opp
// board (< 33% life threshold) should NOT shift defensive or expensive
// scoring. Both pressure-signal arms only fire above the threshold.
func TestCardHeuristic_BelowThresholdNoBias(t *testing.T) {
	mkGS := func(oppPower int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Seats[0].Life = 40
		gs.Seats[1].Life = 40
		for i := 0; i < 5; i++ {
			land := newTestCardMinimal("Plains", []string{"land"}, 0, nil)
			newTestPermanent(gs.Seats[0], land, 0, 0)
		}
		if oppPower > 0 {
			mkCreaturePerm(gs.Seats[1], "Bear", oppPower, oppPower)
		}
		return gs
	}

	doomBlade := newTestCardMinimal("Doom Blade", []string{"instant"}, 2, nil)
	sp := &StrategyProfile{
		Archetype: ArchetypeMidrange,
		CardRoles: map[string]string{"Doom Blade": "Removal"},
	}
	h := NewYggdrasilHat(sp, 0)

	baseline := h.cardHeuristic(mkGS(0), 0, doomBlade)
	mild := h.cardHeuristic(mkGS(8), 0, doomBlade) // 8/40 = 0.20, below threshold
	if mild != baseline {
		t.Errorf("8/40 = 0.20 urgency is below 0.33 gate — Doom Blade score should be unchanged; baseline=%.3f mild=%.3f",
			baseline, mild)
	}
}
