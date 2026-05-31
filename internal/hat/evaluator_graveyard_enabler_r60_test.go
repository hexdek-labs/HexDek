package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 GraveyardValue enabler-bonus extensions — covers the two
// surfaces layered on top of the pre-existing four facets
// (recursion-keyword scaling, reanimator-target ramp, delirium,
// threshold):
//
//   - Facet 5: spell-recursion enabler bonus. Snapcaster / Past in
//     Flames / Underworld Breach / Lurrus / Sun Titan family on the
//     battlefield makes non-keyword instants/sorceries in graveyard
//     into recurring resources.
//   - Facet 6: land-recursion enabler bonus. Crucible of Worlds /
//     Ramunap Excavator / Lord Windgrace makes lands in the graveyard
//     into tempo resources WITHOUT requiring a self-mill payoff.
//
// Each facet has a "no enabler" counterfactual that pins the bonus to
// the enabler being on the battlefield, not a flat per-card scalar.

// ---------------------------------------------------------------------
// Facet 5 — spell-recursion enabler bonus
// ---------------------------------------------------------------------

func TestGraveyardR60_SpellRecursionEnabler_FiresWithSnapcaster(t *testing.T) {
	// Three plain instants in graveyard, no keyword. With Snapcaster on
	// board, the graveyard value should jump meaningfully. Without it,
	// they only contribute the size differential (~0.05 per card vs
	// opponents).
	mkBolts := func(gs *gameengine.GameState) {
		for i := 0; i < 3; i++ {
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
				newTestCardMinimal("Lightning Bolt", []string{"instant"}, 1, nil))
		}
	}

	noEnabler := newTestGame(t, 2)
	mkBolts(noEnabler)
	noScore := graveyardValueFor(t, noEnabler, 0)

	withEnabler := newTestGame(t, 2)
	mkBolts(withEnabler)
	snap := gyCardWithOracle("Snapcaster Mage", []string{"creature"}, 2,
		"when this enters, target instant or sorcery card in your graveyard gains flashback until end of turn")
	newTestPermanent(withEnabler.Seats[0], snap, 2, 1)
	withScore := graveyardValueFor(t, withEnabler, 0)

	if withScore <= noScore {
		t.Fatalf("Snapcaster on board should raise GY value; no=%.3f with=%.3f", noScore, withScore)
	}
	// 3 instants × 0.10 = 0.30 expected bonus (Snapcaster itself is a
	// creature in the graveyard? no — it's on the battlefield. The
	// battlefield permanent does NOT count toward graveyard creature
	// contributions).
	delta := withScore - noScore
	if delta < 0.25 {
		t.Errorf("Snapcaster bonus too small: delta=%.3f (want ≥0.25 for 3 instants)", delta)
	}
}

func TestGraveyardR60_SpellRecursionEnabler_PastInFlamesShape(t *testing.T) {
	// "Each instant and sorcery card in your graveyard gains flashback..."
	gs := newTestGame(t, 2)
	for i := 0; i < 2; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal("Ponder", []string{"sorcery"}, 1, nil))
	}
	pif := gyCardWithOracle("Past in Flames", []string{"sorcery"}, 4,
		"each instant and sorcery card in your graveyard gains flashback until end of turn. exile past in flames.")
	newTestPermanent(gs.Seats[0], pif, 0, 0)
	withScore := graveyardValueFor(t, gs, 0)

	// Counterfactual: no Past in Flames on battlefield.
	base := newTestGame(t, 2)
	for i := 0; i < 2; i++ {
		base.Seats[0].Graveyard = append(base.Seats[0].Graveyard,
			newTestCardMinimal("Ponder", []string{"sorcery"}, 1, nil))
	}
	baseScore := graveyardValueFor(t, base, 0)

	if withScore <= baseScore {
		t.Errorf("Past in Flames enabler must raise GY value: base=%.3f with=%.3f", baseScore, withScore)
	}
}

func TestGraveyardR60_SpellRecursionEnabler_LurrusPermanentShape(t *testing.T) {
	// Lurrus / Sun Titan "return ... permanent ... from your graveyard
	// ... to the battlefield" shape. Two CMC-2 artifacts in graveyard,
	// counterfactual without enabler.
	mkSlots := func(gs *gameengine.GameState) {
		for i := 0; i < 2; i++ {
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
				newTestCardMinimal("Mind Stone", []string{"artifact"}, 2, nil))
		}
		// Also one instant so the bonus has something to bite on (Lurrus
		// would recur the artifact, but the spell-recursion bonus is
		// scoped to instants/sorceries by design — keep the shape match
		// orthogonal to scope intentionally).
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal("Counterspell", []string{"instant"}, 2, nil))
	}

	noEnabler := newTestGame(t, 2)
	mkSlots(noEnabler)
	noScore := graveyardValueFor(t, noEnabler, 0)

	withEnabler := newTestGame(t, 2)
	mkSlots(withEnabler)
	lurrus := gyCardWithOracle("Lurrus of the Dream-Den", []string{"creature"}, 3,
		"during each of your turns, you may cast one permanent spell with mana value 2 or less from your graveyard")
	newTestPermanent(withEnabler.Seats[0], lurrus, 3, 2)
	withScore := graveyardValueFor(t, withEnabler, 0)

	if withScore <= noScore {
		t.Errorf("Lurrus/Sun-Titan shape must raise GY value: no=%.3f with=%.3f", noScore, withScore)
	}
}

func TestGraveyardR60_SpellRecursionEnabler_NoDoubleCountVsKeyword(t *testing.T) {
	// A flashback spell in graveyard with Snapcaster ALSO on board:
	// the spell-recursion enabler must NOT also bump the flashback card
	// (already counted via hasGraveyardCastKeyword). Compare to the
	// same flashback spell in GY without Snapcaster — Snapcaster's
	// presence should not push the per-card recursion value higher
	// than the keyword path alone, because the enabler bonus only
	// fires for cards that DIDN'T already score via the keyword.
	withSnap := newTestGame(t, 2)
	withSnap.Seats[0].Graveyard = append(withSnap.Seats[0].Graveyard,
		gyCardWithKeyword("Cackling Counterpart", []string{"instant"}, 3, "flashback"))
	snap := gyCardWithOracle("Snapcaster Mage", []string{"creature"}, 2,
		"target instant or sorcery card in your graveyard gains flashback until end of turn")
	newTestPermanent(withSnap.Seats[0], snap, 2, 1)
	withSnapScore := graveyardValueFor(t, withSnap, 0)

	noSnap := newTestGame(t, 2)
	noSnap.Seats[0].Graveyard = append(noSnap.Seats[0].Graveyard,
		gyCardWithKeyword("Cackling Counterpart", []string{"instant"}, 3, "flashback"))
	noSnapScore := graveyardValueFor(t, noSnap, 0)

	// Allow a tiny delta from misc bookkeeping but assert the enabler
	// does NOT also stack a +0.10 per-card bump on a keyword card.
	delta := withSnapScore - noSnapScore
	if delta >= 0.10 {
		t.Errorf("spell-recursion enabler double-counted vs keyword: snap=%.3f no_snap=%.3f delta=%.3f (want <0.10)",
			withSnapScore, noSnapScore, delta)
	}
}

func TestGraveyardR60_SpellRecursionEnabler_BonusCapped(t *testing.T) {
	// 15 plain instants in graveyard — bonus is 15×0.10 = 1.50 before
	// cap; the cap should clamp it to 0.50.
	gs := newTestGame(t, 2)
	for i := 0; i < 15; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal("Lightning Bolt", []string{"instant"}, 1, nil))
	}
	snap := gyCardWithOracle("Snapcaster Mage", []string{"creature"}, 2,
		"target instant or sorcery card in your graveyard gains flashback until end of turn")
	newTestPermanent(gs.Seats[0], snap, 2, 1)
	withScore := graveyardValueFor(t, gs, 0)

	base := newTestGame(t, 2)
	for i := 0; i < 15; i++ {
		base.Seats[0].Graveyard = append(base.Seats[0].Graveyard,
			newTestCardMinimal("Lightning Bolt", []string{"instant"}, 1, nil))
	}
	baseScore := graveyardValueFor(t, base, 0)

	// The enabler bonus contribution alone is capped at 0.50. The
	// observed delta may include the archetype-scalar pathway etc., but
	// it must NOT exceed 0.55 (cap + small slack).
	delta := withScore - baseScore
	if delta > 0.55 {
		t.Errorf("enabler bonus not capped: delta=%.3f (want ≤0.55 with the 0.50 cap)", delta)
	}
	if delta < 0.40 {
		t.Errorf("enabler bonus saturating too low: delta=%.3f (want ≥0.40 near the cap)", delta)
	}
}

// ---------------------------------------------------------------------
// Facet 6 — land-recursion enabler bonus
// ---------------------------------------------------------------------

func TestGraveyardR60_LandRecursionEnabler_FiresWithCrucible(t *testing.T) {
	// 4 lands in graveyard, no self-mill payoff signal. Crucible of
	// Worlds on battlefield should raise the value. Without Crucible,
	// the lands score nothing (selfMillPayoff false, no engine).
	mkLands := func(gs *gameengine.GameState) {
		for i := 0; i < 4; i++ {
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
				newTestCardMinimal("Wasteland", []string{"land"}, 0, nil))
		}
	}

	noEnabler := newTestGame(t, 2)
	mkLands(noEnabler)
	noScore := graveyardValueFor(t, noEnabler, 0)

	withEnabler := newTestGame(t, 2)
	mkLands(withEnabler)
	crucible := gyCardWithOracle("Crucible of Worlds", []string{"artifact"}, 3,
		"you may play lands from your graveyard")
	newTestPermanent(withEnabler.Seats[0], crucible, 0, 0)
	withScore := graveyardValueFor(t, withEnabler, 0)

	if withScore <= noScore {
		t.Fatalf("Crucible should raise GY land value; no=%.3f with=%.3f", noScore, withScore)
	}
	// 4 lands × 0.08 = 0.32 expected bonus.
	delta := withScore - noScore
	if delta < 0.25 {
		t.Errorf("Crucible bonus too small: delta=%.3f (want ≥0.25 for 4 lands)", delta)
	}
}

func TestGraveyardR60_LandRecursionEnabler_RamunapExcavatorShape(t *testing.T) {
	gs := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal("Strip Mine", []string{"land"}, 0, nil))
	}
	excavator := gyCardWithOracle("Ramunap Excavator", []string{"creature"}, 3,
		"you may play lands from your graveyard")
	newTestPermanent(gs.Seats[0], excavator, 2, 3)
	withScore := graveyardValueFor(t, gs, 0)

	base := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		base.Seats[0].Graveyard = append(base.Seats[0].Graveyard,
			newTestCardMinimal("Strip Mine", []string{"land"}, 0, nil))
	}
	baseScore := graveyardValueFor(t, base, 0)
	if withScore <= baseScore {
		t.Errorf("Ramunap Excavator must raise GY land value: base=%.3f with=%.3f", baseScore, withScore)
	}
}

func TestGraveyardR60_LandRecursionEnabler_GatedOnSelfMillPayoff(t *testing.T) {
	// When selfMillPayoff is already true (e.g., Muldrotha commander),
	// lands already score +0.05 each — we must NOT also add the
	// land-recursion bonus on top. Build a Muldrotha-commander seat
	// with Crucible + 4 lands, then strip the enabler and confirm the
	// score is unchanged (lands still scored via selfMillPayoff).
	withBoth := newTestGame(t, 2)
	withBoth.Seats[0].CommanderNames = []string{"Muldrotha, the Gravetide"}
	for i := 0; i < 4; i++ {
		withBoth.Seats[0].Graveyard = append(withBoth.Seats[0].Graveyard,
			newTestCardMinimal("Wasteland", []string{"land"}, 0, nil))
	}
	crucible := gyCardWithOracle("Crucible of Worlds", []string{"artifact"}, 3,
		"you may play lands from your graveyard")
	newTestPermanent(withBoth.Seats[0], crucible, 0, 0)
	bothScore := graveyardValueFor(t, withBoth, 0)

	muldrothaOnly := newTestGame(t, 2)
	muldrothaOnly.Seats[0].CommanderNames = []string{"Muldrotha, the Gravetide"}
	for i := 0; i < 4; i++ {
		muldrothaOnly.Seats[0].Graveyard = append(muldrothaOnly.Seats[0].Graveyard,
			newTestCardMinimal("Wasteland", []string{"land"}, 0, nil))
	}
	muldrothaScore := graveyardValueFor(t, muldrothaOnly, 0)

	// The two should be ~equal (within rounding) — the land-recursion
	// arm is gated on !selfMillPayoff and the Muldrotha branch already
	// applied the per-land bonus.
	delta := bothScore - muldrothaScore
	if delta > 0.05 {
		t.Errorf("land-recursion arm double-counted under Muldrotha: muldrotha=%.3f both=%.3f delta=%.3f",
			muldrothaScore, bothScore, delta)
	}
}

func TestGraveyardR60_LandRecursionEnabler_NoEnablerNoFire(t *testing.T) {
	// 5 lands in graveyard, no enabler, no self-mill payoff. The
	// land-recursion bonus must NOT fire — the only credit to the
	// graveyard score comes from the size-vs-opponent differential.
	gs := newTestGame(t, 2)
	for i := 0; i < 5; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal("Wasteland", []string{"land"}, 0, nil))
	}
	score := graveyardValueFor(t, gs, 0)
	// 5 cards differential vs opp's 0 cards = 5 × 0.05 = 0.25. The land
	// bonus would have added 5 × 0.08 = 0.40 on top; the score must NOT
	// exceed the differential-only ceiling by that much.
	if score > 0.30 {
		t.Errorf("lands without enabler scored too high: %.3f (want ≤0.30, only the size differential)", score)
	}
}

// ---------------------------------------------------------------------
// Helper-level sanity — direct exercise of the two new predicates,
// guards against silent regressions in the oracle-text matchers.
// ---------------------------------------------------------------------

func TestSeatHasSpellRecursionEnabler_Matchers(t *testing.T) {
	cases := []struct {
		name    string
		oracle  string
		want    bool
		comment string
	}{
		{"Snapcaster", "target instant or sorcery card in your graveyard gains flashback", true, "Snapcaster ETB shape"},
		{"PastInFlames", "each instant and sorcery card in your graveyard gains flashback", true, "Past in Flames shape — matches via the cast/play-from-graveyard arm too"},
		{"Lurrus", "you may cast one permanent spell with mana value 2 or less from your graveyard", true, "Lurrus permanent-recursion"},
		{"SunTitan", "return target permanent card with mana value 3 or less from your graveyard to the battlefield", true, "Sun Titan return-from-GY"},
		{"YawgWill", "until end of turn, you may play cards from your graveyard", false, "Yawg's Will is sorcery-only; not on battlefield"},
		{"NoGraveyard", "tap: add one mana of any color", false, "no GY reference"},
		{"GraveyardButNotRecursion", "exile target card from a graveyard", false, "GY hate, not recursion"},
		{"AnimateDead", "return target creature card from a graveyard to the battlefield", false, "creature-only reanimator — caught by seatHasReanimatorEngine, not this predicate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seat := &gameengine.Seat{Idx: 0}
			perm := newTestPermanent(seat,
				gyCardWithOracle(tc.name, []string{"artifact"}, 3, tc.oracle), 0, 0)
			_ = perm
			got := seatHasSpellRecursionEnabler(seat)
			if got != tc.want {
				t.Errorf("%s (%s): seatHasSpellRecursionEnabler = %v want %v\n  oracle: %q",
					tc.name, tc.comment, got, tc.want, tc.oracle)
			}
		})
	}
}

func TestSeatHasLandRecursionEnabler_Matchers(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
		want   bool
	}{
		{"Crucible", "you may play lands from your graveyard", true},
		{"Excavator", "you may play lands from your graveyard", true},
		{"GitrogClause", "you may play an additional land each turn. you may play lands from your graveyard.", true},
		{"PutNotPlay", "return target land card from your graveyard to the battlefield", false},
		{"NoGraveyard", "search your library for a land card", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seat := &gameengine.Seat{Idx: 0}
			_ = newTestPermanent(seat,
				gyCardWithOracle(tc.name, []string{"artifact"}, 3, tc.oracle), 0, 0)
			got := seatHasLandRecursionEnabler(seat)
			if got != tc.want {
				t.Errorf("%s: seatHasLandRecursionEnabler = %v want %v\n  oracle: %q",
					tc.name, got, tc.want, tc.oracle)
			}
		})
	}
}
