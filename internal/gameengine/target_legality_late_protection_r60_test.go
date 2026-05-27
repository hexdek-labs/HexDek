package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r60 — regression tests for the §608.2b "still legal at resolution"
// gap on protection / hexproof / shroud acquired AFTER announcement
// but BEFORE resolution. Pre-r60 isTargetStillLegal only checked
// battlefield presence for TargetKindPermanent — so a target that
// gained hexproof / shroud / protection in response to a spell stayed
// "legal" at resolution and the spell resolved through the freshly-
// acquired carveout. That violated CR §608.2b: "A target is illegal if
// it doesn't match the targeting requirements specified by the spell
// or ability."
//
// The matching announcement-side check (ValidateTargetsAtAnnouncement
// → CanBeTargetedByCombat) was already correct; the resolution-time
// re-check just wasn't running the same predicate. Threading the
// StackItem through isTargetStillLegal and routing permanent targets
// through CanBeTargetedByCombat closes the gap with the source card
// available (item.Card for spells, item.Source.Card for abilities).

// TestTargetLegality_LateHexproofFizzlesSpell pins the headline case:
// Lightning Bolt at Grizzly Bears, opponent grants the Bears hexproof
// in response (Mother of Runes / Spectra Ward / bestow), Bolt fizzles
// at resolution.
func TestTargetLegality_LateHexproofFizzlesSpell(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	// Seat 1's Grizzly Bears, target of Lightning Bolt.
	bears := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	// Lightning Bolt cast by seat 0, targeting Bears.
	bolt := &StackItem{
		Controller: 0,
		Card: &Card{
			Name: "Lightning Bolt", Owner: 0,
			Types:  []string{"instant"},
			Colors: []string{"R"},
		},
		Effect: &gameast.Damage{
			Amount: numRef(3),
			Target: gameast.Filter{Base: "creature", Targeted: true},
		},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: bears},
		},
	}
	gs.Stack = append(gs.Stack, bolt)

	// Opponent (controller of Bears = seat 1) grants the Bears hexproof
	// IN RESPONSE — runtime flag mirrors what a Mother of Runes
	// activation or a bestow Aura would produce in the engine.
	bears.Flags["kw:hexproof"] = 1

	ResolveStackTop(gs)

	if !findEvent(gs, "fizzle") {
		t.Errorf("Lightning Bolt should fizzle per §608.2b after target gained hexproof; events: %v",
			collectEventKinds(gs))
	}
	// Spell should have moved to graveyard (countered on resolution).
	foundInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Lightning Bolt" {
			foundInGY = true
			break
		}
	}
	if !foundInGY {
		t.Errorf("fizzled Lightning Bolt should be in seat 0 graveyard; graveyard contents: %v",
			cardNamesInGraveyard(gs, 0))
	}
}

// TestTargetLegality_LateShroudFizzlesSpell pins the shroud variant —
// shroud blocks targeting by EVERYONE including the controller, so
// even a self-target Giant Growth-on-own-creature scenario fizzles.
func TestTargetLegality_LateShroudFizzlesSpell(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	// Seat 0's own creature, target of seat 0's own pump spell.
	mine := addBattlefield(gs, 0, "My Creature", 1, 1, "creature")

	pump := &StackItem{
		Controller: 0,
		Card: &Card{
			Name: "Giant Growth", Owner: 0,
			Types:  []string{"instant"},
			Colors: []string{"G"},
		},
		Effect: &gameast.Damage{
			Amount: numRef(0),
			Target: gameast.Filter{Base: "creature", Targeted: true},
		},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: mine},
		},
	}
	gs.Stack = append(gs.Stack, pump)

	// Mid-stack effect grants shroud — even controller can't target now
	// (shroud is symmetric, unlike hexproof).
	mine.Flags["kw:shroud"] = 1

	ResolveStackTop(gs)

	if !findEvent(gs, "fizzle") {
		t.Errorf("Giant Growth should fizzle per §608.2b after own target gained shroud; events: %v",
			collectEventKinds(gs))
	}
}

// TestTargetLegality_LateProtectionFromColorFizzles pins the protection
// case: a creature gains protection from red in response to a red
// targeted spell. §702.16 → CanBeTargetedByCombat treats the target
// as illegal once protection matches the source card's color.
func TestTargetLegality_LateProtectionFromColorFizzles(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	target := addBattlefield(gs, 1, "Suspicious Bears", 2, 2, "creature")

	bolt := &StackItem{
		Controller: 0,
		Card: &Card{
			Name: "Lightning Bolt", Owner: 0,
			Types:  []string{"instant"},
			Colors: []string{"R"},
		},
		Effect: &gameast.Damage{
			Amount: numRef(3),
			Target: gameast.Filter{Base: "creature", Targeted: true},
		},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: target},
		},
	}
	gs.Stack = append(gs.Stack, bolt)

	// Apostle's Blessing / Mother of Runes — grant protection from red.
	target.Flags["kw:protection"] = 1
	target.Flags["protection_from_red"] = 1

	ResolveStackTop(gs)

	if !findEvent(gs, "fizzle") {
		t.Errorf("Lightning Bolt should fizzle per §608.2b/§702.16 after target gained protection from red; events: %v",
			collectEventKinds(gs))
	}
}

// TestTargetLegality_LateHexproofNotFromControllerStillLegal pins the
// asymmetry: hexproof blocks OPPONENTS only. A self-targeted spell on
// a creature that just gained hexproof is STILL legal — controller can
// target their own hexproof creature. Defense against over-narrowing
// the fix (rejecting all hexproof targets regardless of controller).
func TestTargetLegality_LateHexproofNotFromControllerStillLegal(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	mine := addBattlefield(gs, 0, "My Creature", 1, 1, "creature")

	pump := &StackItem{
		Controller: 0,
		Card: &Card{
			Name: "Giant Growth", Owner: 0,
			Types:  []string{"instant"},
			Colors: []string{"G"},
		},
		Effect: &gameast.Damage{
			Amount: numRef(0),
			Target: gameast.Filter{Base: "creature", Targeted: true},
		},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: mine},
		},
	}
	gs.Stack = append(gs.Stack, pump)

	// Grant hexproof — should NOT block self-targeting per CR §702.11.
	mine.Flags["kw:hexproof"] = 1

	ResolveStackTop(gs)

	if findEvent(gs, "fizzle") {
		t.Errorf("self-target spell should NOT fizzle when own creature has hexproof (hexproof only blocks opponents); events: %v",
			collectEventKinds(gs))
	}
}

// findEvent reports whether the event log contains at least one entry
// of the given kind. Test-local helper.
func findEvent(gs *GameState, kind string) bool {
	for _, e := range gs.EventLog {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

// collectEventKinds returns all logged event kinds for readable
// failure messages.
func collectEventKinds(gs *GameState) []string {
	out := make([]string, 0, len(gs.EventLog))
	for _, e := range gs.EventLog {
		out = append(out, e.Kind)
	}
	return out
}

// cardNamesInGraveyard returns the names of cards in a seat's graveyard
// for readable failure messages.
func cardNamesInGraveyard(gs *GameState, seat int) []string {
	if seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	out := make([]string, 0, len(gs.Seats[seat].Graveyard))
	for _, c := range gs.Seats[seat].Graveyard {
		out = append(out, c.Name)
	}
	return out
}
