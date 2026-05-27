package gameengine

import (
	"testing"
)

// stack_flashback_offboard_r60_test.go — CR §702.33a / §702.143b regression.
//
// CR §702.33a (flashback): "If the flashback cost was paid, exile this
// card instead of putting it anywhere else any time it would leave the
// stack." CR §702.143b (escape) carries identical language. The phrase
// "any time it would leave the stack" covers every off-stack path —
// successful resolution, counterspell, and fizzle-on-illegal-targets.
//
// Before this fix, ResolveStackTop only honored ShouldExileOnResolve on
// the successful-resolve branch; the counter and fizzle branches hard-
// coded "graveyard" via MoveCard. A flashback-cast Lightning Bolt
// pointed at a creature that died in response would FIZZLE to the
// GRAVEYARD instead of exile, meaning the controller could re-flashback
// it on a later turn (or recur it with a graveyard engine). That's a
// real card-advantage bug — flashback is supposed to be a one-shot.
//
// The same break applies to a counterspell on a flashback-cast spell
// (Counterspell → flashback Past in Flames → spell goes to graveyard
// instead of exile, then Past in Flames can be re-flashed). The fix is
// `postStackZoneForOffboard` in stack.go which centralizes the exile-
// instead carveout across counter, fizzle, and successful-resolve
// branches.

// makeFlashbackTargetableInstant builds a flashback-cast targeted instant
// stack item. We hand-construct the StackItem to avoid pulling in the
// full PriorityRound / counterspell machinery — the regression is in
// ResolveStackTop's off-stack routing, which only consults item.Card,
// item.Countered, item.Targets, and item.CostMeta.
func makeFlashbackTargetableInstant(t *testing.T, owner int) *Card {
	t.Helper()
	return newFlashbackCard("Test Flashback Bolt", owner, 1, "{1}{R}")
}

// TestResolveStackTop_CounteredFlashbackSpellExilesInsteadOfGraveyard
// pins CR §702.33a's "any time it would leave the stack" carveout on the
// counterspell path. Without the fix the spell goes to graveyard and
// could be re-flashed; with the fix it exiles.
func TestResolveStackTop_CounteredFlashbackSpellExilesInsteadOfGraveyard(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := makeFlashbackTargetableInstant(t, 0)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	// Cast for flashback so the resolve path stamps CostMeta correctly.
	if _, err := CastFlashback(gs, 0, card, 5); err != nil {
		t.Fatalf("CastFlashback: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item after CastFlashback, got %d", len(gs.Stack))
	}

	// Counter the spell from outside (e.g. a Counterspell resolved
	// targeting this item). Mark .Countered = true to short-circuit
	// resolution; that's the same flag a real counterspell's effect sets.
	gs.Stack[0].Countered = true

	ResolveStackTop(gs)

	// The flashback card must not be in any graveyard.
	for seatIdx, s := range gs.Seats {
		for _, c := range s.Graveyard {
			if c == card {
				t.Fatalf("countered flashback spell landed in seat %d graveyard; "+
					"CR §702.33a requires exile when the card would leave the stack any way", seatIdx)
			}
		}
	}
	// And must be in the owner's exile.
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			foundInExile = true
			break
		}
	}
	if !foundInExile {
		t.Fatal("countered flashback spell should be in owner's exile (CR §702.33a)")
	}
}

// TestResolveStackTop_FizzledFlashbackSpellExilesInsteadOfGraveyard pins
// the §608.2b fizzle path — when all targets are illegal at resolution,
// the spell is countered ("fizzles"); a fizzled flashback spell still
// honors §702.33a's exile-instead carveout.
func TestResolveStackTop_FizzledFlashbackSpellExilesInsteadOfGraveyard(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := makeFlashbackTargetableInstant(t, 0)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	if _, err := CastFlashback(gs, 0, card, 5); err != nil {
		t.Fatalf("CastFlashback: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item after CastFlashback, got %d", len(gs.Stack))
	}

	// Attach a TargetKindPermanent with a nil Permanent — isTargetStillLegal
	// returns false for nil permanents, so CheckTargetLegality reports
	// allIllegal=true and the fizzle branch in ResolveStackTop fires.
	gs.Stack[0].Targets = []Target{{
		Kind:      TargetKindPermanent,
		Permanent: nil,
		Seat:      -1,
	}}

	ResolveStackTop(gs)

	for seatIdx, s := range gs.Seats {
		for _, c := range s.Graveyard {
			if c == card {
				t.Fatalf("fizzled flashback spell landed in seat %d graveyard; "+
					"CR §702.33a requires exile when the card would leave the stack any way", seatIdx)
			}
		}
	}
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			foundInExile = true
			break
		}
	}
	if !foundInExile {
		t.Fatal("fizzled flashback spell should be in owner's exile (CR §702.33a + §608.2b)")
	}
}

// TestResolveStackTop_CounteredNonFlashbackSpellStillGoesToGraveyard
// pins the converse — the carveout must NOT apply to a vanilla
// (non-flashback) spell. CR §701.5a is the default: countered spells
// go to their owner's graveyard. Defends against the helper being too
// aggressive.
func TestResolveStackTop_CounteredNonFlashbackSpellStillGoesToGraveyard(t *testing.T) {
	gs := newFlashbackGame(t)
	gs.Active = 0

	card := newInstantCard("Vanilla Bolt", 0, 1)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	// Push a stack item by hand — no CostMeta["exile_on_resolve"].
	gs.Stack = append(gs.Stack, &StackItem{
		Card:       card,
		Controller: 0,
		Countered:  true,
	})

	ResolveStackTop(gs)

	foundInGraveyard := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			foundInGraveyard = true
			break
		}
	}
	if !foundInGraveyard {
		t.Fatal("countered vanilla spell should land in owner's graveyard (CR §701.5a)")
	}
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			t.Fatal("countered vanilla spell should NOT exile — no flashback carveout applies")
		}
	}
}
