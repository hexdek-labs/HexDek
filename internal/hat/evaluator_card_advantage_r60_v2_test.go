package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// CardAdvantage r60-v2 regressions covering the new handQualityBonus
// triad layered onto the existing dimension (hand count + library
// depth + drawEngineCredit + castableExileCount +
// opponentTriggerValueCount):
//
//   - Tutor-in-hand quality bonus (+0.15 per tutor, cap +0.45)
//   - Cantrip-in-hand quality bonus (+0.08 per cantrip, cap +0.32)
//   - Hand-flood dead-card penalty (-0.08 per dead, cap -0.40)
//
// Each signal pinned independently; the cross-signal pin confirms the
// triad applies symmetrically (my - opp avg) without double-counting
// when a card matches multiple shapes (cantrip wins over tutor).

// ---------------------------------------------------------------------
// Shared builders
// ---------------------------------------------------------------------

// caGameWithHand builds a 2-seat game with seat 0's hand seeded from
// the cards arg. Opp hand stays empty so handQualityBonus on the opp
// side returns 0 — keeps the math focused on seat 0.
func caGameWithHand(t *testing.T, cards []*gameengine.Card, lands int) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 2)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	gs.Seats[0].Hand = cards
	for i := 0; i < lands; i++ {
		newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
	return gs
}

// ---------------------------------------------------------------------
// Signal A — tutor-in-hand quality bonus
// ---------------------------------------------------------------------

func TestHandQualityBonus_TutorCounted(t *testing.T) {
	seat := &gameengine.Seat{Idx: 0}
	seat.Hand = []*gameengine.Card{
		gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand, then shuffle"),
	}
	got := handQualityBonus(seat)
	if got != 0.15 {
		t.Errorf("single tutor: got %.4f want 0.15", got)
	}
}

func TestHandQualityBonus_TutorCapped(t *testing.T) {
	seat := &gameengine.Seat{Idx: 0}
	for i := 0; i < 10; i++ {
		seat.Hand = append(seat.Hand,
			gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
				"search your library for a card, put it into your hand"))
	}
	got := handQualityBonus(seat)
	if got != 0.45 {
		t.Errorf("10 tutors should cap at 0.45: got %.4f", got)
	}
}

func TestScoreCardsR60v2_TutorInHandRaisesScore(t *testing.T) {
	mk := func(tutorInHand bool) *gameengine.GameState {
		hand := []*gameengine.Card{
			newTestCardMinimal("Vanilla 2-drop", []string{"creature"}, 2, nil),
			newTestCardMinimal("Vanilla 3-drop", []string{"creature"}, 3, nil),
		}
		if tutorInHand {
			hand = append(hand, gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
				"search your library for a card, put it into your hand"))
		} else {
			hand = append(hand, newTestCardMinimal("Vanilla 4-drop", []string{"creature"}, 4, nil))
		}
		return caGameWithHand(t, hand, 4)
	}

	ev := NewEvaluator(nil)
	withTutor := ev.scoreCards(mk(true), 0)
	noTutor := ev.scoreCards(mk(false), 0)

	if withTutor <= noTutor {
		t.Errorf("tutor in hand should out-score equivalent vanilla card; tutor=%.4f no=%.4f",
			withTutor, noTutor)
	}
}

// ---------------------------------------------------------------------
// Signal B — cantrip-in-hand quality bonus
// ---------------------------------------------------------------------

func TestHandQualityBonus_CantripCounted(t *testing.T) {
	seat := &gameengine.Seat{Idx: 0}
	seat.Hand = []*gameengine.Card{
		gyCardWithOracle("Ponder", []string{"sorcery"}, 1,
			"look at the top three cards of your library. then put them back in any order. you may shuffle your library. draw a card."),
	}
	got := handQualityBonus(seat)
	if got != 0.08 {
		t.Errorf("single cantrip: got %.4f want 0.08", got)
	}
}

func TestHandQualityBonus_CantripCapped(t *testing.T) {
	seat := &gameengine.Seat{Idx: 0}
	for i := 0; i < 10; i++ {
		seat.Hand = append(seat.Hand,
			gyCardWithOracle("Ponder", []string{"sorcery"}, 1, "draw a card"))
	}
	got := handQualityBonus(seat)
	if got != 0.32 {
		t.Errorf("10 cantrips should cap at 0.32: got %.4f", got)
	}
}

func TestHandQualityBonus_CantripBeatsTutorOnOverlap(t *testing.T) {
	// A card that is BOTH a cantrip and a tutor (rare but Sleight of Hand
	// + "search your library" — hypothetical) should count as cantrip
	// only, not double-count. Pin the rule: cantrip path uses `continue`
	// so tutor branch is skipped.
	seat := &gameengine.Seat{Idx: 0}
	seat.Hand = []*gameengine.Card{
		gyCardWithOracle("Hybrid Cantrip Tutor", []string{"sorcery"}, 1,
			"draw a card. search your library for a card."),
	}
	got := handQualityBonus(seat)
	// Cantrip-only credit, not cantrip + tutor.
	if got != 0.08 {
		t.Errorf("cantrip+tutor overlap: got %.4f want 0.08 (cantrip-only)", got)
	}
}

// ---------------------------------------------------------------------
// Signal C — hand-flood dead-card penalty
// ---------------------------------------------------------------------

func TestHandQualityBonus_DeadHighCMCCardCounted(t *testing.T) {
	// 3 lands on field → deadThreshold = 6. A CMC-9 card is dead.
	seat := &gameengine.Seat{Idx: 0}
	for i := 0; i < 3; i++ {
		newTestPermanent(seat,
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
	seat.Hand = []*gameengine.Card{
		newTestCardMinimal("Big Spell", []string{"sorcery"}, 9, nil),
	}
	got := handQualityBonus(seat)
	if got != -0.08 {
		t.Errorf("CMC-9 in hand vs 3 lands (deadThreshold=6): got %.4f want -0.08", got)
	}
}

func TestHandQualityBonus_FloodLandPenalty(t *testing.T) {
	// 5 lands on field + 2 lands in hand → flood. Each land counts dead.
	seat := &gameengine.Seat{Idx: 0}
	for i := 0; i < 5; i++ {
		newTestPermanent(seat,
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
	seat.Hand = []*gameengine.Card{
		newTestCardMinimal("Plains", []string{"land"}, 0, nil),
		newTestCardMinimal("Plains", []string{"land"}, 0, nil),
	}
	got := handQualityBonus(seat)
	if got != -0.16 {
		t.Errorf("2 lands flooding 5-land field: got %.4f want -0.16", got)
	}
}

func TestHandQualityBonus_DeadCardCapped(t *testing.T) {
	// Many high-CMC cards in hand on a 1-land field → dead count blows
	// past 5; penalty should cap at -0.40.
	seat := &gameengine.Seat{Idx: 0}
	newTestPermanent(seat,
		newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	for i := 0; i < 10; i++ {
		seat.Hand = append(seat.Hand,
			newTestCardMinimal("Big Spell", []string{"sorcery"}, 8, nil))
	}
	got := handQualityBonus(seat)
	if got != -0.40 {
		t.Errorf("10 dead big spells (cap at -0.40): got %.4f", got)
	}
}

func TestHandQualityBonus_CastableNotDead(t *testing.T) {
	// 4 lands → deadThreshold = 7. CMC-5 is castable, CMC-7 is the edge
	// (NOT dead — uses strict > comparison), CMC-8 IS dead.
	seat := &gameengine.Seat{Idx: 0}
	for i := 0; i < 4; i++ {
		newTestPermanent(seat,
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
	seat.Hand = []*gameengine.Card{
		newTestCardMinimal("CMC5", []string{"sorcery"}, 5, nil),
		newTestCardMinimal("CMC7", []string{"sorcery"}, 7, nil),
		newTestCardMinimal("CMC8", []string{"sorcery"}, 8, nil),
	}
	got := handQualityBonus(seat)
	if got != -0.08 {
		t.Errorf("only CMC-8 should be dead vs deadThreshold=7: got %.4f want -0.08", got)
	}
}

// ---------------------------------------------------------------------
// Cross-signal — triad applies symmetrically through scoreCards
// ---------------------------------------------------------------------

func TestScoreCardsR60v2_AllSignalsCombineCorrectly(t *testing.T) {
	// Seat 0: 4 lands, hand = 1 tutor + 1 cantrip + 1 dead CMC-9 (= +0.15 + 0.08 - 0.08 = +0.15).
	// Opp: empty hand, no field → handQualityBonus = 0.
	hand := []*gameengine.Card{
		gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand"),
		gyCardWithOracle("Ponder", []string{"sorcery"}, 1, "draw a card"),
		newTestCardMinimal("Big Spell", []string{"sorcery"}, 9, nil), // dead vs 4 lands
	}
	gs := caGameWithHand(t, hand, 4)

	// Comparable baseline: same 3-card hand, all vanilla CMC-4.
	baselineHand := []*gameengine.Card{
		newTestCardMinimal("V1", []string{"sorcery"}, 4, nil),
		newTestCardMinimal("V2", []string{"sorcery"}, 4, nil),
		newTestCardMinimal("V3", []string{"sorcery"}, 4, nil),
	}
	gsBase := caGameWithHand(t, baselineHand, 4)

	ev := NewEvaluator(nil)
	loaded := ev.scoreCards(gs, 0)
	baseline := ev.scoreCards(gsBase, 0)

	delta := loaded - baseline
	// Expected: tutor(+0.15) + cantrip(+0.08) - dead(-0.08) = +0.15.
	if delta < 0.10 || delta > 0.20 {
		t.Errorf("triad combined delta off: %.4f (want ~0.15)", delta)
	}
}

// TestScoreCardsR60v2_SymmetricOpponentSide: opp seat with 2 tutors in
// hand should make OUR score lower (opp's hand quality avg > ours).
func TestScoreCardsR60v2_SymmetricOpponentSide(t *testing.T) {
	mk := func(oppHasTutors bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].Hand = []*gameengine.Card{
			newTestCardMinimal("V1", []string{"sorcery"}, 3, nil),
			newTestCardMinimal("V2", []string{"sorcery"}, 3, nil),
		}
		for i := 0; i < 4; i++ {
			newTestPermanent(gs.Seats[0],
				newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
			newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
		}
		if oppHasTutors {
			gs.Seats[1].Hand = []*gameengine.Card{
				gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
					"search your library for a card, put it into your hand"),
				gyCardWithOracle("Vampiric Tutor", []string{"instant"}, 1,
					"search your library for a card and put that card on top of your library"),
			}
		} else {
			gs.Seats[1].Hand = []*gameengine.Card{
				newTestCardMinimal("V1", []string{"sorcery"}, 3, nil),
				newTestCardMinimal("V2", []string{"sorcery"}, 3, nil),
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	oppTutors := ev.scoreCards(mk(true), 0)
	oppVanilla := ev.scoreCards(mk(false), 0)

	if oppTutors >= oppVanilla {
		t.Errorf("opp with 2 tutors should LOWER our CardAdvantage; opp_tutors=%.4f opp_vanilla=%.4f",
			oppTutors, oppVanilla)
	}
}
