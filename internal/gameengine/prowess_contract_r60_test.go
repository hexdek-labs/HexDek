package gameengine

// r60 — Prowess contract audit (CR §702.108).
//
// "Whenever you cast a noncreature spell, this creature gets +1/+1
// until end of turn."
//
// Audit finding: the prowess fire-site lives in
// FireCastTriggerObservers (cast_counts.go:150) and is gated on
// (a) controller match, (b) the cast spell being noncreature, and
// (c) fromCopy=false. FireCastTriggerObservers is invoked from six
// places — CastSpell (stack.go:455), the PriorityRound response-cast
// path (stack.go:799), the cost-paid path (costs.go:823), the
// paradigm copy cast (phases.go:926), the commander-zone cast
// (commander.go:473), and the zone-cast/impulse-play path
// (zone_cast.go:371). NONE of the triggered-ability paths
// (PushTriggeredAbility in stack.go, trigger_batch.go, the per-card
// dispatcher in trigger_stack_bridge.go) call it, and the activated-
// ability path (activation.go) doesn't either. So per the implementation:
//
//   - Prowess fires only on spell CASTS (correct per §111.1 "a spell
//     is a card on the stack").
//   - Prowess does NOT fire on triggered-ability resolutions.
//   - Prowess does NOT fire on activated-ability resolutions.
//   - Prowess does NOT fire on copies (fromCopy guard).
//   - Prowess does NOT fire on creature spell casts (noncreature gate).
//
// This file pins each of those facts so a future refactor that moves
// FireCastTriggerObservers into a generic "any stack item resolves"
// hook (or accidentally calls it from the trigger/activation paths)
// will be caught.

import (
	"testing"
)

// makeProwessPerm puts a creature with the prowess keyword on the
// given seat's battlefield and returns the permanent.
func makeProwessPerm(gs *GameState, seat int) *Permanent {
	return addCreature(gs, seat, "Monastery Swiftspear", 1, 2, "prowess")
}

// powerBuff returns the sum of +Power across all Modifications on the
// permanent — the visible signal that prowess fired.
func powerBuff(p *Permanent) int {
	if p == nil {
		return 0
	}
	sum := 0
	for _, m := range p.Modifications {
		sum += m.Power
	}
	return sum
}

// prowessEventCount counts prowess events in the log.
func prowessEventCount(gs *GameState) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "prowess" {
			n++
		}
	}
	return n
}

// TestProwess_FiresOnNoncreatureSpellCast (positive control) confirms
// the happy path still works — defends against a regression that
// over-narrows the gate.
func TestProwess_FiresOnNoncreatureSpellCast(t *testing.T) {
	gs := newCombatGame(t)
	p := makeProwessPerm(gs, 0)

	spell := &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	FireCastTriggerObservers(gs, spell, 0, false)

	if prowessEventCount(gs) != 1 {
		t.Fatalf("prowess should fire on a noncreature spell cast; events=%d", prowessEventCount(gs))
	}
	if powerBuff(p) != 1 {
		t.Errorf("prowess should apply +1/+1; got +%d power", powerBuff(p))
	}
}

// TestProwess_DoesNotFireOnCreatureSpellCast asserts the noncreature
// gate excludes creature casts.
func TestProwess_DoesNotFireOnCreatureSpellCast(t *testing.T) {
	gs := newCombatGame(t)
	p := makeProwessPerm(gs, 0)

	spell := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}
	FireCastTriggerObservers(gs, spell, 0, false)

	if prowessEventCount(gs) != 0 {
		t.Errorf("prowess must not fire on a creature spell cast; events=%d", prowessEventCount(gs))
	}
	if powerBuff(p) != 0 {
		t.Errorf("prowess creature should be unchanged after a creature spell cast; got +%d power", powerBuff(p))
	}
}

// TestProwess_DoesNotFireOnCopy asserts the fromCopy gate excludes
// Storm / Twinflame / Dualcaster Mage copies (CR §707.10: copies are
// not cast).
func TestProwess_DoesNotFireOnCopy(t *testing.T) {
	gs := newCombatGame(t)
	p := makeProwessPerm(gs, 0)

	spell := &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	FireCastTriggerObservers(gs, spell, 0, true /*fromCopy*/)

	if prowessEventCount(gs) != 0 {
		t.Errorf("prowess must not fire on a copy; events=%d", prowessEventCount(gs))
	}
	if powerBuff(p) != 0 {
		t.Errorf("prowess creature should be unchanged after a copied spell; got +%d power", powerBuff(p))
	}
}

// TestProwess_DoesNotFireOnTriggeredAbilityResolution is the key audit
// claim. Pushing a triggered ability via PushTriggeredAbility uses
// neither CastSpell nor FireCastTriggerObservers; resolving it must
// leave the prowess creature unaffected.
//
// This pins the engine's "prowess fires on CASTS, not on triggered-
// ability resolutions" invariant against a future refactor that moves
// the prowess hook into a generic stack-resolution observer.
func TestProwess_DoesNotFireOnTriggeredAbilityResolution(t *testing.T) {
	gs := newCombatGame(t)
	installStubHats(gs)
	p := makeProwessPerm(gs, 0)

	// A "noncreature" source permanent for the triggered ability — the
	// most adversarial shape, since a careless implementation might
	// match on "the source is noncreature" instead of "a spell was
	// cast".
	source := dummyTriggerPerm(0, "Some Artifact")
	source.Card.Types = []string{"artifact"}

	PushTriggeredAbility(gs, source, noopEffect())

	// PushTriggeredAbility resolves inline (priority round + resolve).
	// After the call returns, the stack should be drained.
	if got := len(gs.Stack); got != 0 {
		t.Logf("note: stack length after trigger push = %d (resolves inline)", got)
	}

	if prowessEventCount(gs) != 0 {
		t.Errorf("prowess must NOT fire on a triggered-ability resolution; events=%d", prowessEventCount(gs))
	}
	if powerBuff(p) != 0 {
		t.Errorf("prowess creature must NOT gain +1/+1 from a triggered ability; got +%d power", powerBuff(p))
	}
}

// TestProwess_DoesNotFireOnActivatedAbilityResolution pins the same
// invariant for activated abilities. We push a synthetic activated
// stack item and resolve the top — FireCastTriggerObservers must NOT
// be invoked along that path.
func TestProwess_DoesNotFireOnActivatedAbilityResolution(t *testing.T) {
	gs := newCombatGame(t)
	installStubHats(gs)
	p := makeProwessPerm(gs, 0)

	source := dummyTriggerPerm(0, "Mana Rock")
	source.Card.Types = []string{"artifact"}

	// Push as Kind="activated" with a noop effect. ResolveStackTop
	// routes activated items through resolveActivatedAbility, NOT
	// through any cast pipeline.
	item := &StackItem{
		Controller: 0,
		Source:     source,
		Card:       source.Card,
		Kind:       "activated",
		Effect:     noopEffect(),
	}
	PushStackItem(gs, item)
	ResolveStackTop(gs)

	if prowessEventCount(gs) != 0 {
		t.Errorf("prowess must NOT fire on an activated-ability resolution; events=%d", prowessEventCount(gs))
	}
	if powerBuff(p) != 0 {
		t.Errorf("prowess creature must NOT gain +1/+1 from an activated ability; got +%d power", powerBuff(p))
	}
}

// TestProwess_OnlyControllerCreaturesFire pins the controller gate:
// prowess on an OPPONENT's creature doesn't fire when YOU cast a
// noncreature spell.
func TestProwess_OnlyControllerCreaturesFire(t *testing.T) {
	gs := newCombatGame(t)
	mine := makeProwessPerm(gs, 0)
	theirs := makeProwessPerm(gs, 1)

	spell := &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	FireCastTriggerObservers(gs, spell, 0, false)

	if powerBuff(mine) != 1 {
		t.Errorf("controller's prowess creature should gain +1/+1; got +%d", powerBuff(mine))
	}
	if powerBuff(theirs) != 0 {
		t.Errorf("opponent's prowess creature must not gain +1/+1 from MY cast; got +%d", powerBuff(theirs))
	}
}
