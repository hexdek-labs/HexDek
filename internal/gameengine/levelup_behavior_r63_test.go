package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// levelup_behavior_r63_test.go — behavioral regressions for Leveler creatures
// (CR §702.87 / §711). Before r63 the leveler system was entirely unwired:
// ActivateLevelUp had no callers and ApplyLevelBracketEffects was never called,
// so a leveler's band P/T + abilities never applied. These drive the activated
// ability and assert the band characteristics via the authoritative
// Permanent.Power()/Toughness()/HasKeyword paths (what combat/SBA read).

// levelerPerm builds a leveler creature with two bands parsed from oracle Raw:
//   LEVEL lo1-hi1 → p1/t1 + kw1 ; LEVEL lo2+ → p2/t2 + kw2.
// Printed (level-0) P/T is basePT/basePT.
func levelerPerm(gs *GameState, seat, basePT int, band1 string, p1, t1 int, kw1 string,
	band2 string, p2, t2 int, kw2 string) *Permanent {
	card := &Card{
		Name:          "Test Leveler",
		Owner:         seat,
		Types:         []string{"creature"},
		BasePower:     basePT,
		BaseToughness: basePT,
		AST: &gameast.CardAST{
			Name: "Test Leveler",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "level up", Raw: "Level up {1}"},
				&gameast.Static{Raw: band1},
				&gameast.Static{Raw: ptString(p1, t1)},
				&gameast.Static{Raw: kw1},
				&gameast.Static{Raw: band2},
				&gameast.Static{Raw: ptString(p2, t2)},
				&gameast.Static{Raw: kw2},
			},
		},
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func ptString(p, t int) string {
	return itoa(p) + "/" + itoa(t)
}

func newLevelGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(63)), nil)
	gs.Active = 0
	gs.Step = "precombat_main"
	return gs
}

// (g) A leveler with 0 level counters uses its printed (level-0) P/T and has
// no band abilities.
func TestLeveler_Level0UsesBaseStats(t *testing.T) {
	gs := newLevelGame(t)
	p := levelerPerm(gs, 0, 1, "LEVEL 2-6", 3, 3, "first strike", "LEVEL 7+", 4, 4, "double strike")
	if p.Power() != 1 || p.Toughness() != 1 {
		t.Errorf("(g) level-0 leveler should be 1/1 (printed), got %d/%d", p.Power(), p.Toughness())
	}
	if p.HasKeyword("first strike") || p.HasKeyword("double strike") {
		t.Error("(g) level-0 leveler must not have any band abilities")
	}
}

// (a)+(b)+(c)+(f) Level up adds a level counter at sorcery speed, and crossing
// into the first band immediately sets the band base P/T and grants its ability.
func TestLeveler_LevelUpEntersFirstBand(t *testing.T) {
	gs := newLevelGame(t)
	p := levelerPerm(gs, 0, 1, "LEVEL 2-6", 3, 3, "first strike", "LEVEL 7+", 4, 4, "double strike")
	gs.Seats[0].ManaPool = 10

	// One level-up: level 1, still below the first band (min 2) → base stats.
	if err := ActivateLevelUp(gs, 0, p); err != nil {
		t.Fatalf("(a) ActivateLevelUp failed: %v", err)
	}
	if p.Counters["level"] != 1 {
		t.Fatalf("(a) expected 1 level counter, got %d", p.Counters["level"])
	}
	if p.Power() != 1 {
		t.Errorf("(c) at level 1 (below band min 2) P/T should still be base 1/1, got %d/%d", p.Power(), p.Toughness())
	}

	// Second level-up: level 2 → enters band 2-6 → 3/3, first strike.
	if err := ActivateLevelUp(gs, 0, p); err != nil {
		t.Fatalf("ActivateLevelUp failed: %v", err)
	}
	if p.Power() != 3 || p.Toughness() != 3 {
		t.Errorf("(c)/(f) at level 2 the band sets base P/T to 3/3, got %d/%d", p.Power(), p.Toughness())
	}
	if !p.HasKeyword("first strike") {
		t.Error("(d) at level 2 the creature should have first strike from the band")
	}
	if p.HasKeyword("double strike") {
		t.Error("(d) the higher band's ability must not be active yet")
	}
}

// (d)+(f) Crossing a band threshold updates P/T and abilities; the lower band's
// ability is no longer active.
func TestLeveler_CrossingBandThresholdUpdates(t *testing.T) {
	gs := newLevelGame(t)
	p := levelerPerm(gs, 0, 1, "LEVEL 2-6", 3, 3, "first strike", "LEVEL 7+", 4, 4, "double strike")
	p.Counters["level"] = 6
	ApplyLevelBracketEffects(gs, p)
	if p.Power() != 3 || !p.HasKeyword("first strike") || p.HasKeyword("double strike") {
		t.Fatalf("setup: at level 6 expect 3/3 first strike only; got %d/%d fs=%v ds=%v",
			p.Power(), p.Toughness(), p.HasKeyword("first strike"), p.HasKeyword("double strike"))
	}
	// Cross to level 7 → band 7+ → 4/4, double strike, drop first strike.
	p.Counters["level"] = 7
	ApplyLevelBracketEffects(gs, p)
	if p.Power() != 4 || p.Toughness() != 4 {
		t.Errorf("(f) crossing to band 7+ should set 4/4, got %d/%d", p.Power(), p.Toughness())
	}
	if !p.HasKeyword("double strike") {
		t.Error("(d) band 7+ should grant double strike")
	}
	if p.HasKeyword("first strike") {
		t.Error("(d) the lower band's first strike must drop when leaving band 2-6")
	}
}

// (c) Band base P/T is a base set, so +1/+1 counters stack on top.
func TestLeveler_BandPTStacksWithCounters(t *testing.T) {
	gs := newLevelGame(t)
	p := levelerPerm(gs, 0, 1, "LEVEL 2-6", 3, 3, "first strike", "LEVEL 7+", 4, 4, "double strike")
	p.Counters["level"] = 3
	ApplyLevelBracketEffects(gs, p)
	p.AddCounter("+1/+1", 2)
	if p.Power() != 5 || p.Toughness() != 5 {
		t.Errorf("(c) band 3/3 + two +1/+1 counters should be 5/5, got %d/%d", p.Power(), p.Toughness())
	}
}

// (e) Level counters respect general counter doublers (Doubling Season).
func TestLeveler_LevelCounterRespectsDoublers(t *testing.T) {
	gs := newLevelGame(t)
	ds := levelerPerm(gs, 0, 0, "LEVEL 99-99", 0, 0, "", "LEVEL 100+", 0, 0, "") // dummy carrier for the doubler
	_ = ds
	// Register Doubling Season off a separate permanent.
	dsPerm := &Permanent{Card: &Card{Name: "Doubling Season", Owner: 0, Types: []string{"enchantment"}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, dsPerm)
	RegisterDoublingSeason(gs, dsPerm)

	p := levelerPerm(gs, 0, 1, "LEVEL 2-6", 3, 3, "first strike", "LEVEL 7+", 4, 4, "double strike")
	gs.Seats[0].ManaPool = 10

	if err := ActivateLevelUp(gs, 0, p); err != nil {
		t.Fatalf("ActivateLevelUp failed: %v", err)
	}
	if p.Counters["level"] != 2 {
		t.Errorf("(e) one level-up under Doubling Season should add 2 level counters, got %d", p.Counters["level"])
	}
	// And the doubled count (2) immediately puts it in band 2-6 → 3/3.
	if p.Power() != 3 {
		t.Errorf("(e)/(f) doubled level counter should reach band 2-6 (3/3), got %d/%d", p.Power(), p.Toughness())
	}
}

// (a) Level up is sorcery-speed only — rejected when the stack is non-empty.
func TestLeveler_SorcerySpeedOnly(t *testing.T) {
	gs := newLevelGame(t)
	p := levelerPerm(gs, 0, 1, "LEVEL 2-6", 3, 3, "first strike", "LEVEL 7+", 4, 4, "double strike")
	gs.Seats[0].ManaPool = 10
	gs.Stack = append(gs.Stack, &StackItem{Card: &Card{Name: "Something"}, Controller: 0})

	if err := ActivateLevelUp(gs, 0, p); err == nil {
		t.Error("(a) level up must be rejected at instant speed (non-empty stack)")
	}
	if p.Counters["level"] != 0 {
		t.Errorf("(a) a rejected level up must not add a counter, got %d", p.Counters["level"])
	}
}
