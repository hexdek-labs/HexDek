package hat

import (
	"math"
	"testing"
)

// r60 CardAdvantage retune. drawEngineCredit now weights each engine by
// rate class (drawEngineRate):
//
//	0.0  — not a persistent draw engine
//	0.6  — symmetric draw ("each player draws") — Howling Mine, Temple
//	       Bell, Font of Mythos
//	1.0  — upkeep / passive controller-only draw — Phyrexian Arena,
//	       Sylvan Library, Mind's Eye
//	1.5  — opponent-action-triggered draw — Rhystic Study, Mystic
//	       Remora, Esper Sentinel
//
// Pre-r60 every engine got 1.0 uniformly. The retune produces more
// honest CardAdvantage scoring (Rhystic Study > Phyrexian Arena >
// Howling Mine) without shifting the dimension's relative weight or
// the cap (still 4.0 total per seat).

// TestDrawEngineRate_RhysticStudyOppTrigger15 pins Rhystic Study at the
// 1.5 opp-trigger rate-class. Anchor card for the Tier-2 weight.
func TestDrawEngineRate_RhysticStudyOppTrigger15(t *testing.T) {
	rhystic := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
		"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], rhystic, 0, 0)

	got := drawEngineRate(p)
	if math.Abs(got-1.5) > 1e-9 {
		t.Errorf("Rhystic Study rate = %f, want 1.5 (opp-trigger)", got)
	}
}

// TestDrawEngineRate_PhyrexianArenaUpkeep1 pins Phyrexian Arena at the
// 1.0 upkeep-cadenced baseline.
func TestDrawEngineRate_PhyrexianArenaUpkeep1(t *testing.T) {
	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], arena, 0, 0)

	got := drawEngineRate(p)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("Phyrexian Arena rate = %f, want 1.0 (upkeep)", got)
	}
}

// TestDrawEngineRate_SymmetricEnginesGet06 pins Howling Mine and
// Temple Bell at the 0.6 symmetric-draw rate-class. Howling Mine's
// "each player's draw step" + "additional card" path covers the
// table-wide draw-step shape; Temple Bell's "{T}: each player draws"
// covers the tap-symmetric variant.
//
// Howling Mine is detected as an engine via the existing "additional
// card" gate in isPersistentDrawEngine. Temple Bell only matches the
// symmetric gate; the engine-detection helper doesn't currently
// classify it as persistent (no "additional card" / "upkeep" /
// "whenever" wording), so the symmetric branch returns 0 for it. The
// asymmetric pair Howling Mine vs Phyrexian Arena is the bit-stable
// anchor for the new rate-class table.
func TestDrawEngineRate_SymmetricEnginesGet06(t *testing.T) {
	mine := cardWithStaticText("Howling Mine", []string{"artifact"}, 2,
		"at the beginning of each player's draw step, if howling mine is untapped, that player draws an additional card")
	gs := newTestGame(t, 2)
	pm := newTestPermanent(gs.Seats[0], mine, 0, 0)

	if got := drawEngineRate(pm); math.Abs(got-0.6) > 1e-9 {
		t.Errorf("Howling Mine rate = %f, want 0.6 (each-player's-draw-step symmetric)", got)
	}

	font := cardWithStaticText("Font of Mythos", []string{"artifact"}, 4,
		"at the beginning of each player's draw step, that player draws an additional card")
	pf := newTestPermanent(gs.Seats[0], font, 0, 0)
	if got := drawEngineRate(pf); math.Abs(got-0.6) > 1e-9 {
		t.Errorf("Font of Mythos rate = %f, want 0.6 (each-player's-draw-step symmetric)", got)
	}
}

// TestDrawEngineRate_NotAnEngineReturns0 confirms non-engine permanents
// don't pick up a rate. A vanilla creature, a basic land, and a
// one-shot ETB-drawer (Elvish Visionary's "when this enters") all
// return 0.
func TestDrawEngineRate_NotAnEngineReturns0(t *testing.T) {
	gs := newTestGame(t, 2)
	bear := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	p1 := newTestPermanent(gs.Seats[0], bear, 2, 2)
	if got := drawEngineRate(p1); got != 0 {
		t.Errorf("vanilla bear rate = %f, want 0", got)
	}

	visionary := cardWithStaticText("Elvish Visionary", []string{"creature"}, 2,
		"when this creature enters, draw a card")
	p2 := newTestPermanent(gs.Seats[0], visionary, 1, 1)
	if got := drawEngineRate(p2); got != 0 {
		t.Errorf("ETB-only draw card rate = %f, want 0 (no whenever/upkeep)", got)
	}
}

// TestDrawEngineCredit_RhysticOutscoresArena pins the scoring outcome:
// a seat with Rhystic Study should score higher CardAdvantage credit
// than one with Phyrexian Arena, holding everything else equal.
func TestDrawEngineCredit_RhysticOutscoresArena(t *testing.T) {
	gs := newTestGame(t, 2)

	rhystic := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
		"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
	newTestPermanent(gs.Seats[0], rhystic, 0, 0)

	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	newTestPermanent(gs.Seats[1], arena, 0, 0)

	cRhystic := drawEngineCredit(gs.Seats[0])
	cArena := drawEngineCredit(gs.Seats[1])

	if !(cRhystic > cArena) {
		t.Errorf("Rhystic Study should out-credit Phyrexian Arena; got rhystic=%f arena=%f", cRhystic, cArena)
	}
	// Sanity: exact values match the rate table.
	if math.Abs(cRhystic-1.5) > 1e-9 {
		t.Errorf("seat with single Rhystic = %f, want 1.5", cRhystic)
	}
	if math.Abs(cArena-1.0) > 1e-9 {
		t.Errorf("seat with single Arena = %f, want 1.0", cArena)
	}
}

// TestDrawEngineCredit_SymmetricEngineBelowAsymmetric confirms the
// symmetric-draw discount: a seat with Howling Mine should score
// strictly LESS than one with Phyrexian Arena, because Howling Mine
// gives opponents the same draws via the each-player's-draw-step
// table-wide trigger.
func TestDrawEngineCredit_SymmetricEngineBelowAsymmetric(t *testing.T) {
	gs := newTestGame(t, 2)

	mine := cardWithStaticText("Howling Mine", []string{"artifact"}, 2,
		"at the beginning of each player's draw step, if howling mine is untapped, that player draws an additional card")
	newTestPermanent(gs.Seats[0], mine, 0, 0)

	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	newTestPermanent(gs.Seats[1], arena, 0, 0)

	cMine := drawEngineCredit(gs.Seats[0])
	cArena := drawEngineCredit(gs.Seats[1])

	if !(cMine < cArena) {
		t.Errorf("Howling Mine (symmetric) should under-credit Phyrexian Arena (asymmetric); got mine=%f arena=%f", cMine, cArena)
	}
	// Exact values: 0.6 vs 1.0.
	if math.Abs(cMine-0.6) > 1e-9 {
		t.Errorf("Howling Mine credit = %f, want 0.6", cMine)
	}
	if math.Abs(cArena-1.0) > 1e-9 {
		t.Errorf("Arena credit = %f, want 1.0", cArena)
	}
}

// TestDrawEngineCredit_CappedAt4 pins the per-seat 4.0 cap. Six Rhystic
// Studies (6×1.5 = 9.0) on the same battlefield cap at 4.0 — bounds
// stax/wheel-board outliers from blowing out the CardAdvantage
// dimension.
func TestDrawEngineCredit_CappedAt4(t *testing.T) {
	gs := newTestGame(t, 2)
	for i := 0; i < 6; i++ {
		c := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
			"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
		newTestPermanent(gs.Seats[0], c, 0, 0)
	}
	got := drawEngineCredit(gs.Seats[0])
	if math.Abs(got-4.0) > 1e-9 {
		t.Errorf("six Rhystic Studies should cap at 4.0; got %f", got)
	}
}

// TestScoreCards_RhysticSeatOutscoresArenaSeat pins the end-to-end
// integration: in scoreCards, a Rhystic-Study seat scores strictly
// higher CardAdvantage than an Arena seat at otherwise-equal state.
func TestScoreCards_RhysticSeatOutscoresArenaSeat(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	rhystic := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
		"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
	newTestPermanent(gs.Seats[0], rhystic, 0, 0)

	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	newTestPermanent(gs.Seats[1], arena, 0, 0)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if !(r0.CardAdvantage > r1.CardAdvantage) {
		t.Errorf("Rhystic seat should out-score Arena seat on CardAdvantage; got rhystic=%.3f arena=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

// TestScoreCards_SymmetricEngineSeatUnderscoresAsymmetric pins the
// symmetric-discount end-to-end: a Howling-Mine seat scores strictly
// LESS CardAdvantage than an Arena seat (the symmetric engine helps
// the opponent equally and the rate-class diffuser captures that).
func TestScoreCards_SymmetricEngineSeatUnderscoresAsymmetric(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	mine := cardWithStaticText("Howling Mine", []string{"artifact"}, 2,
		"at the beginning of each player's draw step, if howling mine is untapped, that player draws an additional card")
	newTestPermanent(gs.Seats[0], mine, 0, 0)

	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	newTestPermanent(gs.Seats[1], arena, 0, 0)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if !(r0.CardAdvantage < r1.CardAdvantage) {
		t.Errorf("Howling-Mine seat should under-score Arena seat on CardAdvantage; got mine=%.3f arena=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}
