package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mulligan_commander_castable_r60_test.go — regressions for the
// commander-castability gate added to ChooseMulligan. The deck's
// primary plan is usually "land the commander, then execute its
// text" — a 7-card hand that can't reach commander CMC by turn 4
// throws that plan away unless there's an engine card in hand to
// fall back on. See yggdrasil.go::ChooseMulligan +
// projectedManaAtTurn + minCommanderCMC.

// Helpers (newTestGame, newTestCardMinimal, landNamed, cardWithRawOracle,
// putCommanderInZone) come from hat_test.go / mulligan_r60_test.go.

// solRingLikeRamp is a 1cmc artifact that categorizeWithFreya/categorizeCard
// will classify as CatRamp (cmc<=3 artifact branch in isRampCard).
func solRingLikeRamp(name string) *gameengine.Card {
	return newTestCardMinimal(name, []string{"artifact"}, 1, nil)
}

// -----------------------------------------------------------------------------
// minCommanderCMC — unit
// -----------------------------------------------------------------------------

func TestMinCommanderCMC_NilSeatReturnsZero(t *testing.T) {
	if got := minCommanderCMC(nil); got != 0 {
		t.Errorf("nil seat should return 0; got %d", got)
	}
}

func TestMinCommanderCMC_EmptyZoneReturnsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	if got := minCommanderCMC(gs.Seats[0]); got != 0 {
		t.Errorf("empty command zone should return 0 (skips the gate); got %d", got)
	}
}

func TestMinCommanderCMC_SingleCommanderReadsCMC(t *testing.T) {
	gs := newTestGame(t, 2)
	cmd := newTestCardMinimal("Sliver Overlord", []string{"creature", "legendary"}, 6, nil)
	putCommanderInZone(gs.Seats[0], cmd)
	if got := minCommanderCMC(gs.Seats[0]); got != 6 {
		t.Errorf("single 6-CMC commander should return 6; got %d", got)
	}
}

// Partner decks: minCommanderCMC picks the CHEAPER commander — you can
// cast either to begin executing the plan.
func TestMinCommanderCMC_PartnerReturnsCheaper(t *testing.T) {
	gs := newTestGame(t, 2)
	thrasios := newTestCardMinimal("Thrasios, Triton Hero", []string{"creature", "legendary"}, 2, nil)
	tymna := newTestCardMinimal("Tymna the Weaver", []string{"creature", "legendary"}, 4, nil)
	putCommanderInZone(gs.Seats[0], thrasios)
	putCommanderInZone(gs.Seats[0], tymna)
	if got := minCommanderCMC(gs.Seats[0]); got != 2 {
		t.Errorf("partner pair (2-CMC + 4-CMC) should return 2 (cheaper); got %d", got)
	}
}

// -----------------------------------------------------------------------------
// projectedManaAtTurn — unit
// -----------------------------------------------------------------------------

func TestProjectedManaAtTurn_LandsOnlyCappedAtTurn(t *testing.T) {
	// 7 lands in hand at turn 4: capped at one drop per turn → 4.
	if got := projectedManaAtTurn(7, 0, 4); got != 4.0 {
		t.Errorf("7 lands, 0 ramp, turn 4 should cap at 4.0 (one drop / turn); got %.2f", got)
	}
}

func TestProjectedManaAtTurn_DrawSupplementsThinLandHand(t *testing.T) {
	// 2 lands in hand, 3 draw steps (turn 4) at 0.4 land density:
	// 2 + 3*0.4 = 3.2, below turn cap of 4 → 3.2.
	got := projectedManaAtTurn(2, 0, 4)
	if got < 3.15 || got > 3.25 {
		t.Errorf("2 lands + 3 draws @ 0.4 = 3.2 ± rounding; got %.3f", got)
	}
}

func TestProjectedManaAtTurn_RampBonusAddsAtTurnThreePlus(t *testing.T) {
	// 3 lands + 2 ramp pieces at turn 4: lands = min(4, 3+1.2)=4, ramp +2 = 6.
	if got := projectedManaAtTurn(3, 2, 4); got != 6.0 {
		t.Errorf("3 lands + 2 ramp at turn 4 should project 6.0; got %.2f", got)
	}
}

func TestProjectedManaAtTurn_NoRampBonusBeforeTurnThree(t *testing.T) {
	// Ramp doesn't contribute at turn 2 (can't be cast + active that fast
	// under the heuristic). 2 lands + 2 ramp at turn 2 = 2 lands only.
	if got := projectedManaAtTurn(2, 2, 2); got != 2.0 {
		t.Errorf("2 lands + 2 ramp at turn 2 should still project 2.0 (ramp not yet active); got %.2f", got)
	}
}

func TestProjectedManaAtTurn_RampCappedAtTwo(t *testing.T) {
	// 5 ramp pieces in hand: only 2 contribute (can't curve more than 2
	// rocks profitably before turn 4).
	withFive := projectedManaAtTurn(3, 5, 4)
	withTwo := projectedManaAtTurn(3, 2, 4)
	if withFive != withTwo {
		t.Errorf("5-ramp hand (%.2f) should match 2-ramp hand (%.2f) — capped at 2", withFive, withTwo)
	}
}

func TestProjectedManaAtTurn_ZeroTurnGuard(t *testing.T) {
	if got := projectedManaAtTurn(7, 2, 0); got != 0 {
		t.Errorf("turn 0 should defensively return 0; got %.2f", got)
	}
}

// -----------------------------------------------------------------------------
// ChooseMulligan: commander-castability integration
// -----------------------------------------------------------------------------

// Headline ask: a 7-card hand with 3 lands, no ramp, no engine — and
// a 6-CMC commander — should mulligan. Pre-r60 this hand would pass
// the "any-archetype: 2 lands + no VE/star" fallthrough as keepable
// because no archetype rule fired and no engine triggered the keep
// branch; the gate now demands that the deck's primary plan (the
// commander) is reachable.
func TestChooseMulligan_HeavyCommanderUncastable_NoBackup_Mulligans(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	cmd := newTestCardMinimal("Sliver Overlord", []string{"creature", "legendary"}, 6, nil)
	putCommanderInZone(gs.Seats[0], cmd)

	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
	}
	if !h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("7-card hand with 3 lands / 0 ramp / 0 engine / 6-CMC commander should mulligan — commander uncastable by T4 with no backup plan")
	}
}

// Same hand but with 2 mana rocks (Sol Ring-likes) — projected mana at
// turn 4 jumps to ~6, commander becomes reachable, hand keeps.
func TestChooseMulligan_HeavyCommanderCastableViaRamp_Keeps(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	cmd := newTestCardMinimal("Sliver Overlord", []string{"creature", "legendary"}, 6, nil)
	putCommanderInZone(gs.Seats[0], cmd)

	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		solRingLikeRamp("Sol Ring"),
		solRingLikeRamp("Arcane Signet"),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
	}
	if h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("3 lands + 2 mana rocks reaches 6 mana by T4 — should KEEP heavy-commander hand")
	}
}

// Same uncastable-commander hand but with a value-engine key in hand —
// engine backup means we have a non-commander game plan, so the gate
// stays its hand. Use a card name that NewYggdrasilHat will treat as a
// value-engine key.
func TestChooseMulligan_HeavyCommanderUncastable_EngineBackup_Keeps(t *testing.T) {
	sp := &StrategyProfile{
		Archetype: ArchetypeMidrange,
		// Mark Rhystic Study as the deck's star + value-engine so the
		// engine-backup branch picks it up.
		StarCards: []string{"Rhystic Study"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	cmd := newTestCardMinimal("Sliver Overlord", []string{"creature", "legendary"}, 6, nil)
	putCommanderInZone(gs.Seats[0], cmd)

	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		newTestCardMinimal("Rhystic Study", []string{"enchantment"}, 3, nil),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
	}
	if h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("uncastable commander but a star/engine card in hand provides a backup plan — should KEEP")
	}
}

// A cheap commander (2 CMC) trivially passes the gate regardless of
// land/ramp count — the check never fires for low-CMC commanders.
func TestChooseMulligan_CheapCommander_GateDoesNotFire(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	cmd := newTestCardMinimal("Yuriko, the Tiger's Shadow", []string{"creature", "legendary"}, 2, nil)
	putCommanderInZone(gs.Seats[0], cmd)

	// Identical to the mulligan-fire hand above — but commander is 2-CMC.
	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
	}
	if h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("2-CMC commander hits T2 trivially — gate must not fire and hand should keep")
	}
}

// Partial mull (6-card hand): the gate is intentionally NOT applied —
// 6-card hands accept thinner plans. Same shape as the mulligan-fire
// case but trimmed to 6 cards.
func TestChooseMulligan_PartialMull_GateSkipped(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)

	cmd := newTestCardMinimal("Sliver Overlord", []string{"creature", "legendary"}, 6, nil)
	putCommanderInZone(gs.Seats[0], cmd)

	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
	}
	if h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("6-card hand should not trigger commander-castability gate (partial mulls accept thinner plans)")
	}
}

// Edge case: no commanders in zone (free-form / non-Commander format).
// The gate must short-circuit so the hand is judged on the normal path,
// not mistreat zero as a "free commander".
func TestChooseMulligan_NoCommander_GateSkipped(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	gs := newTestGame(t, 2)
	// No putCommanderInZone — seat 0's CommandZone stays empty.

	hand := []*gameengine.Card{
		landNamed("Forest"), landNamed("Mountain"), landNamed("Island"),
		newTestCardMinimal("Hill Giant", []string{"creature"}, 4, nil),
		newTestCardMinimal("Wind Drake", []string{"creature"}, 3, nil),
		newTestCardMinimal("Serra Angel", []string{"creature"}, 5, nil),
		newTestCardMinimal("Air Elemental", []string{"creature"}, 5, nil),
	}
	if h.ChooseMulligan(gs, 0, hand) {
		t.Fatal("commander-less seat should skip the gate (free-form game / edge test) and keep on the normal path")
	}
}
