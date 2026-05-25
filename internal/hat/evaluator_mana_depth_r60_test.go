package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r60 ManaAdvantage retune. Two improvements to scoreMana:
//
//   (1) Depth-weighted color coverage — replaces the pre-r60 binary
//       "have any source of this color" check with a sqrt-decay weight
//       (1 source = 0.5 credit, 2 = 0.71, 3 = 0.87, 4+ = 1.0). A deck
//       with a {BBBB} pip demand and only 1 black source no longer
//       scores like one with 4 sources.
//
//   (2) Untapped-source bonus on opponent turns — on an opponent's
//       turn, sources you can ACTUALLY USE (untapped) for instants /
//       counters / removal weight more than the same total count
//       spread across mostly-tapped permanents. Bonus capped at +0.25.
//       On our own turn the bonus is 0 (tapped sources untap next).

// makeMountainPerm + makeIslandPerm + makeBlackPermanent build minimal
// permanents the colorDepth and untapped-source scorers can read.

func makeColoredLand(seat *gameengine.Seat, color string, tapped bool) *gameengine.Permanent {
	colorToBasic := map[string]string{
		"W": "Plains", "U": "Island", "B": "Swamp", "R": "Mountain", "G": "Forest",
	}
	name := colorToBasic[color]
	if name == "" {
		name = "Wastes"
	}
	c := newTestCardMinimal(name, []string{"land", "basic"}, 0, nil)
	// landProducesColorsMask reads the type line for the basic-land
	// subtype to detect produced colors. Set TypeLine so `fieldColorSources`
	// actually counts the land for the right color.
	c.TypeLine = "Basic Land — " + name
	p := newTestPermanent(seat, c, 0, 0)
	p.Tapped = tapped
	return p
}

func newScoreManaEvalGame(t *testing.T, demand map[string]int) (*GameStateEvaluator, *gameengine.GameState) {
	t.Helper()
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
		s.StartingLife = 40
	}
	// Default to seat 0's turn unless test overrides.
	gs.Active = 0
	sp := &StrategyProfile{ColorDemand: demand}
	ev := NewEvaluator(sp)
	return ev, gs
}

// TestColorDepthWeight_TableValues pins the exact sqrt-decay weights
// the depth-weighted coverage path produces. Anchors so future
// re-tunings of the curve see this test fail and the rationale
// re-evaluated explicitly.
func TestColorDepthWeight_TableValues(t *testing.T) {
	cases := []struct {
		n    int
		want float64
	}{
		{0, 0.0},
		{1, 0.5},
		{2, 0.5 * math.Sqrt(2)},
		{3, 0.5 * math.Sqrt(3)},
		{4, 1.0},
		{5, 1.0},
		{10, 1.0},
	}
	for _, c := range cases {
		got := colorDepthWeight(c.n)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("colorDepthWeight(%d) = %v, want %v", c.n, got, c.want)
		}
	}
}

// TestScoreMana_ColorDepth_4SourcesOutscores1Source pins the headline
// improvement: with a heavy {BBBB} demand profile, a seat with 4
// black sources should score strictly higher than a seat with 1.
// Pre-r60 the binary check made these identical.
func TestScoreMana_ColorDepth_4SourcesOutscores1Source(t *testing.T) {
	ev, gsDeep := newScoreManaEvalGame(t, map[string]int{"B": 4})
	for i := 0; i < 4; i++ {
		makeColoredLand(gsDeep.Seats[0], "B", false)
	}
	// Equalize opponent's mana so the count delta isolates the
	// depth effect.
	for i := 0; i < 4; i++ {
		makeColoredLand(gsDeep.Seats[1], "B", false)
	}
	deepScore := ev.scoreMana(gsDeep, 0)

	_, gsThin := newScoreManaEvalGame(t, map[string]int{"B": 4})
	makeColoredLand(gsThin.Seats[0], "B", false)
	// Same total mana count: pad with colorless lands so total source
	// count matches the deep seat.
	for i := 0; i < 3; i++ {
		c := newTestCardMinimal("Wastes", []string{"land", "basic"}, 0, nil)
		newTestPermanent(gsThin.Seats[0], c, 0, 0)
	}
	for i := 0; i < 4; i++ {
		makeColoredLand(gsThin.Seats[1], "B", false)
	}
	thinScore := ev.scoreMana(gsThin, 0)

	if deepScore <= thinScore {
		t.Fatalf("4 black sources should score strictly higher than 1 black source for {BBBB} demand; got deep=%.3f thin=%.3f", deepScore, thinScore)
	}
	// Sanity: the gap should be meaningful (>0.1 score units), not noise.
	if deepScore-thinScore < 0.1 {
		t.Errorf("color depth gap looks too thin: deep=%.3f thin=%.3f delta=%.3f (want > 0.1)",
			deepScore, thinScore, deepScore-thinScore)
	}
}

// TestScoreMana_ColorDepth_2SourcesBetweenThinAndFull pins the
// monotonicity: 2 sources should score between 1-source and 4-source
// on the same demand. Catches accidental binary-collapse regressions.
func TestScoreMana_ColorDepth_2SourcesBetweenThinAndFull(t *testing.T) {
	scoreFor := func(t *testing.T, blackCount int) float64 {
		t.Helper()
		ev, gs := newScoreManaEvalGame(t, map[string]int{"B": 4})
		for i := 0; i < blackCount; i++ {
			makeColoredLand(gs.Seats[0], "B", false)
		}
		for i := 0; i < (4 - blackCount); i++ {
			c := newTestCardMinimal("Wastes", []string{"land", "basic"}, 0, nil)
			newTestPermanent(gs.Seats[0], c, 0, 0)
		}
		for i := 0; i < 4; i++ {
			makeColoredLand(gs.Seats[1], "B", false)
		}
		return ev.scoreMana(gs, 0)
	}
	s1 := scoreFor(t, 1)
	s2 := scoreFor(t, 2)
	s4 := scoreFor(t, 4)
	if !(s1 < s2 && s2 < s4) {
		t.Fatalf("color depth must be monotonically increasing 1<2<4; got s1=%.3f s2=%.3f s4=%.3f", s1, s2, s4)
	}
}

// TestScoreMana_UntappedBonus_AppliesOnOppTurn pins the response-mana
// improvement: on opponent's turn, two seats with the same total
// source count but different untapped ratios should score
// differently — the untapped seat scores higher.
func TestScoreMana_UntappedBonus_AppliesOnOppTurn(t *testing.T) {
	ev, gs := newScoreManaEvalGame(t, nil)
	// Opponent's turn so the bonus applies.
	gs.Active = 1

	// Seat 0: 6 sources, 4 untapped.
	for i := 0; i < 4; i++ {
		makeColoredLand(gs.Seats[0], "U", false)
	}
	for i := 0; i < 2; i++ {
		makeColoredLand(gs.Seats[0], "U", true)
	}
	// Seat 1: 6 sources, all tapped (post-attack opponent).
	for i := 0; i < 6; i++ {
		makeColoredLand(gs.Seats[1], "U", true)
	}

	s0 := ev.scoreMana(gs, 0)
	s1 := ev.scoreMana(gs, 1)

	if s0 <= s1 {
		t.Fatalf("untapped-source seat should outscore tapped-source seat on opp turn; got s0=%.3f s1=%.3f", s0, s1)
	}
}

// TestScoreMana_UntappedBonus_NoEffectOnOwnTurn pins the scoping: the
// untapped bonus is ONLY active when it's NOT our turn. On our own
// turn the tapped/untapped distinction is noise (everything untaps
// next upkeep), and we don't want the AI second-guessing taps that
// generate the mana for the spell being cast.
func TestScoreMana_UntappedBonus_NoEffectOnOwnTurn(t *testing.T) {
	ev, gs := newScoreManaEvalGame(t, nil)
	gs.Active = 0 // our turn

	// Seat 0: 6 sources, 4 untapped.
	for i := 0; i < 4; i++ {
		makeColoredLand(gs.Seats[0], "U", false)
	}
	for i := 0; i < 2; i++ {
		makeColoredLand(gs.Seats[0], "U", true)
	}
	scoreUntappedHeavy := ev.scoreMana(gs, 0)

	// Reset and try all-tapped on the same seat.
	gs.Seats[0].Battlefield = nil
	for i := 0; i < 6; i++ {
		makeColoredLand(gs.Seats[0], "U", true)
	}
	scoreAllTapped := ev.scoreMana(gs, 0)

	if math.Abs(scoreUntappedHeavy-scoreAllTapped) > 1e-9 {
		t.Fatalf("on our own turn, tapped vs untapped should NOT shift scoreMana; got untapped=%.3f tapped=%.3f", scoreUntappedHeavy, scoreAllTapped)
	}
}

// TestScoreMana_UntappedBonus_CappedAt0_25 pins the bound on the
// response-mana bonus. Even with 100% untapped sources the bonus
// caps at +0.25 so it never outweighs the primary source-count
// delta.
func TestScoreMana_UntappedBonus_CappedAt0_25(t *testing.T) {
	ev, gs := newScoreManaEvalGame(t, nil)
	gs.Active = 1 // opp turn

	// Seat 0: 4 untapped sources (100% untapped ratio → max bonus).
	for i := 0; i < 4; i++ {
		makeColoredLand(gs.Seats[0], "U", false)
	}
	scoreFull := ev.scoreMana(gs, 0)

	// Now compare against 0-untapped baseline on same total count.
	gs.Seats[0].Battlefield = nil
	for i := 0; i < 4; i++ {
		makeColoredLand(gs.Seats[0], "U", true)
	}
	scoreZero := ev.scoreMana(gs, 0)

	gap := scoreFull - scoreZero
	// Tolerance for tiny float noise; bonus is 0.25 * (4/4) = 0.25 exact.
	if gap < 0.24 || gap > 0.26 {
		t.Fatalf("max untapped bonus should be ~0.25; got %.3f (full=%.3f zero=%.3f)", gap, scoreFull, scoreZero)
	}
}

// TestCountUntappedManaSources_BasicShape pins the helper directly.
// Tapped sources excluded, untapped lands counted, untapped cheap
// mana artifacts counted, non-mana artifacts excluded.
func TestCountUntappedManaSources_BasicShape(t *testing.T) {
	gs := newTestGame(t, 2)

	// 3 untapped lands + 1 tapped land = 3 untapped lands counted
	for i := 0; i < 3; i++ {
		makeColoredLand(gs.Seats[0], "U", false)
	}
	makeColoredLand(gs.Seats[0], "U", true)

	// Untapped Sol Ring (mana artifact, CMC 1) — counted.
	solRing := cardWithStaticText("Sol Ring", []string{"artifact"}, 1, "{t}: add {c}{c}")
	newTestPermanent(gs.Seats[0], solRing, 0, 0)

	// Untapped non-mana artifact — NOT counted (no "mana" / "{t}" text).
	hat := newTestCardMinimal("Stuffy Doll", []string{"artifact", "creature"}, 4, nil)
	newTestPermanent(gs.Seats[0], hat, 0, 5)

	got := CountUntappedManaSources(gs.Seats[0])
	want := 4 // 3 untapped lands + Sol Ring
	if got != want {
		t.Fatalf("CountUntappedManaSources = %d, want %d", got, want)
	}
}
