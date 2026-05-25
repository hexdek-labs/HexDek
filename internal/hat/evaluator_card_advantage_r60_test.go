package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// equalizeLifeAndZones puts each seat at 40/40 starting life and gives
// both seats the same hand size + library depth, so scoreCards' "vs.
// opponent average" arithmetic isolates whatever differential the test
// is actually exercising.
func equalizeLifeAndZones(t *testing.T, gs *gameengine.GameState, handPerSeat, libPerSeat int) {
	t.Helper()
	for _, s := range gs.Seats {
		s.Life = 40
		s.StartingLife = 40
		s.Hand = nil
		s.Library = nil
		for i := 0; i < handPerSeat; i++ {
			s.Hand = append(s.Hand, newTestCardMinimal("Filler", []string{"instant"}, 1, nil))
		}
		for i := 0; i < libPerSeat; i++ {
			s.Library = append(s.Library, newTestCardMinimal("Topdeck", []string{"sorcery"}, 1, nil))
		}
	}
}

func TestScoreCards_R60_RhysticStudyOnlyHelpsControllerSide(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	rhystic := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
		"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
	newTestPermanent(gs.Seats[0], rhystic, 0, 0)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if r0.CardAdvantage <= r1.CardAdvantage {
		t.Errorf("Rhystic-Study seat should out-score empty seat on CardAdvantage; got s0=%.3f s1=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

func TestScoreCards_R60_PhyrexianArenaUpkeepDraw(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	newTestPermanent(gs.Seats[0], arena, 0, 0)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if r0.CardAdvantage <= r1.CardAdvantage {
		t.Errorf("Phyrexian-Arena seat should out-score empty seat on CardAdvantage; got s0=%.3f s1=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

func TestScoreCards_R60_SylvanLibraryAdditionalCards(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	sylvan := cardWithStaticText("Sylvan Library", []string{"enchantment"}, 2,
		"at the beginning of your draw step, you may draw two additional cards")
	newTestPermanent(gs.Seats[0], sylvan, 0, 0)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if r0.CardAdvantage <= r1.CardAdvantage {
		t.Errorf("Sylvan-Library seat should out-score empty seat on CardAdvantage; got s0=%.3f s1=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

func TestScoreCards_R60_OneShotETBDrawIsNotAnEngine(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	// "When ~ enters, you draw a card" — Elvish Visionary shape. The
	// "you draw a card" payoff exists but it is gated by a one-shot
	// "when ~ enters" trigger, not a recurring "whenever" / upkeep
	// cadence, so drawEngineCredit should NOT credit it.
	visionary := cardWithStaticText("Elvish Visionary", []string{"creature"}, 2,
		"when elvish visionary enters, you draw a card")
	newTestPermanent(gs.Seats[0], visionary, 1, 1)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if r0.CardAdvantage != r1.CardAdvantage {
		t.Errorf("one-shot ETB drawer must not register as a persistent draw engine; got s0=%.3f s1=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

func TestScoreCards_R60_DrawEngineCreditCappedAtFour(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)
	for i := 0; i < 8; i++ {
		eng := cardWithStaticText("Howling Mine Copy", []string{"artifact"}, 2,
			"that player draws an additional card")
		newTestPermanent(gs.Seats[0], eng, 0, 0)
	}
	if got := drawEngineCredit(gs.Seats[0]); got != 4 {
		t.Errorf("drawEngineCredit should cap at 4, got %.1f for 8 engines", got)
	}
}

func TestScoreCards_R60_CastableExileGrantCreditsController(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	// Two foretold cards in seat 0's exile with a "may cast from exile"
	// ZoneCastGrant scoped to seat 0.
	for i := 0; i < 2; i++ {
		c := newTestCardMinimal("Saw It Coming", []string{"instant"}, 4, nil)
		c.Owner = 0
		gs.Seats[0].Exile = append(gs.Seats[0].Exile, c)
		gameengine.RegisterZoneCastGrant(gs, c, &gameengine.ZoneCastPermission{
			Zone:              "exile",
			Keyword:           "foretell",
			ManaCost:          1,
			RequireController: 0,
			SourceName:        "foretell",
		})
	}

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if r0.CardAdvantage <= r1.CardAdvantage {
		t.Errorf("seat with 2 castable-exile cards should out-score empty seat; got s0=%.3f s1=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

func TestScoreCards_R60_CastableExileGrantRespectsRequireController(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	// Card sits in seat 0's exile but the grant is locked to seat 1
	// (Gonti / Hostage Taker take-control pattern). Seat 0 must NOT
	// get credit for a card it cannot cast.
	stolen := newTestCardMinimal("Hostage", []string{"creature"}, 3, nil)
	stolen.Owner = 0
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, stolen)
	gameengine.RegisterZoneCastGrant(gs, stolen, &gameengine.ZoneCastPermission{
		Zone:              "exile",
		Keyword:           "hostage_cast",
		ManaCost:          -1,
		RequireController: 1,
		SourceName:        "Hostage Taker",
	})

	if got := castableExileCount(gs, 0); got != 0 {
		t.Errorf("seat 0 should NOT get credit for a grant locked to seat 1, got %d", got)
	}
	if got := castableExileCount(gs, 1); got != 0 {
		// Card isn't in seat 1's exile, so seat 1 also doesn't see it.
		t.Errorf("seat 1's exile is empty, count should be 0, got %d", got)
	}
}

func TestScoreCards_R60_NoEnginesPreservesBaselineBehavior(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if r0.CardAdvantage != r1.CardAdvantage {
		t.Errorf("equal hands + libraries + no engines must produce equal CardAdvantage; got s0=%.3f s1=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}
