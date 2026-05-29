package gameengine

// R60 — SBA edge-case regressions surfaced by audit (goldilocks at 0
// covers the per-card-isolated surface; these pin multi-permanent /
// cross-rule interactions that goldilocks can't reach).
//
// Coverage gaps before this file:
//
//   1. §704.3 single-event semantics for §704.5g — two creatures both
//      at MarkedDamage ≥ toughness in the same pass.
//
//   2. §704.5q asymmetric +1/+1 / -1/-1 annihilation when OTHER counter
//      types (loyalty, charge, time, etc.) are also present. The
//      existing TestSBA_704_5q_CounterAnnihilation covers the symmetric
//      pair-only case; an implementation bug that walked every
//      Counters entry would silently delete unrelated counter types.
//
//   3. §702.16e: protection prevents damage being DEALT, not damage
//      already MARKED. A creature with lethal MarkedDamage that then
//      gains "protection from <source>" must still die — protection
//      doesn't roll back the damage register.
//
// Rule citations:
//   §704.3   simultaneous-SBA single event
//   §704.5g  lethal damage destroys creatures
//   §704.5q  +1/+1 and -1/-1 counters annihilate 1-for-1
//   §702.12b indestructible
//   §702.16e protection doesn't remove already-marked damage

import (
	"testing"
)

// -----------------------------------------------------------------------------
// §704.5g — simultaneous lethal-damage deaths in a single SBA pass.
//
// Per §704.3 SBAs happen as a single event. Two creatures both at lethal
// must both die in the same pass and both emit their destroy events; the
// snapshot iteration in sba704_5g must not short-circuit after the first
// destroy mutates the live battlefield.
// -----------------------------------------------------------------------------

func TestSBA_704_5g_SimultaneousLethalDeaths(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Runeclaw Bear", 2, 2, "creature")
	elf := addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")
	bear.MarkedDamage = 2
	elf.MarkedDamage = 1

	StateBasedActions(gs)

	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatalf("both creatures should be destroyed simultaneously; battlefield has %d",
			len(gs.Seats[0].Battlefield))
	}
	// §704.3 single-event: both destroys must surface in the same SBA call,
	// not one-per-pass. Two destroys + two 5g events.
	if got := countEvents(gs, "destroy"); got != 2 {
		t.Fatalf("expected 2 destroy events from simultaneous lethal, got %d", got)
	}
	if got := countEvents(gs, "sba_704_5g"); got != 2 {
		t.Fatalf("expected 2 sba_704_5g events from simultaneous lethal, got %d", got)
	}
	// Both should be in graveyard for their owners (seat 0 here).
	if got := len(gs.Seats[0].Graveyard); got != 2 {
		t.Fatalf("expected 2 creatures in graveyard, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// §704.5q — asymmetric +1/+1 / -1/-1 annihilation preserves OTHER
// counter types. The existing TestSBA_704_5q_CounterAnnihilation pins
// the symmetric removal; this guards against a regression that would
// touch any non-(+1/+1, -1/-1) key.
// -----------------------------------------------------------------------------

func TestSBA_704_5q_AsymmetricAnnihilationPreservesOtherCounterTypes(t *testing.T) {
	gs := newFixtureGame(t)
	// Use a planeswalker so loyalty is meaningful, with extra counter
	// types tagged on for the preservation assertion.
	p := addBattlefield(gs, 0, "Garruk, Cursed Huntsman", 0, 0, "planeswalker", "legendary")
	p.AddCounter("loyalty", 5)
	p.AddCounter("+1/+1", 5)
	p.AddCounter("-1/-1", 2)
	p.AddCounter("charge", 3)
	p.AddCounter("time", 4)

	StateBasedActions(gs)

	// Annihilation: 5 − 2 = 3 +1/+1; -1/-1 zeroed (key deleted by sba704_5q).
	if got := p.Counters["+1/+1"]; got != 3 {
		t.Errorf("expected 3 +1/+1 after asymmetric annihilation, got %d", got)
	}
	if _, has := p.Counters["-1/-1"]; has {
		t.Errorf("expected -1/-1 key removed after hitting zero, got %+v", p.Counters)
	}
	// Other counter types must be untouched — sba704_5q only mutates the
	// +1/-1 pair per CR §704.5q.
	if got := p.Counters["loyalty"]; got != 5 {
		t.Errorf("loyalty counters must be preserved by §704.5q; expected 5, got %d", got)
	}
	if got := p.Counters["charge"]; got != 3 {
		t.Errorf("charge counters must be preserved by §704.5q; expected 3, got %d", got)
	}
	if got := p.Counters["time"]; got != 4 {
		t.Errorf("time counters must be preserved by §704.5q; expected 4, got %d", got)
	}
	// The walker survives (loyalty > 0 → §704.5i no-op).
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("planeswalker should survive (loyalty=5 > 0); battlefield has %d",
			len(gs.Seats[0].Battlefield))
	}
}

// -----------------------------------------------------------------------------
// §702.16e — protection does NOT remove already-marked damage.
//
// Sequence the test pins: damage is dealt (and marked), THEN the
// creature gains protection from the source. §704.5g must still
// destroy it because the marked damage was lawfully applied before
// protection existed, and protection only prevents future damage.
//
// An incorrect "skip-if-protected" gate in sba704_5g would silently
// save the creature here.
// -----------------------------------------------------------------------------

func TestSBA_704_5g_ProtectionAfterDamageMarkedStillDestroys(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Runeclaw Bear", 2, 2, "creature")

	// Damage dealt and marked BEFORE protection existed.
	bear.MarkedDamage = 2

	// Now stamp protection from creature on the bear (e.g. resolved
	// Mother of Runes, Apostle's Blessing, etc. — modeled by the
	// prot_type:<X> flag the engine uses for protection bookkeeping).
	if bear.Flags == nil {
		bear.Flags = map[string]int{}
	}
	bear.Flags["prot_type:creature"] = 1
	if !HasProtectionFromType(bear, "creature") {
		t.Fatal("test setup: protection-from-creature flag should register")
	}

	StateBasedActions(gs)

	// CR §702.16e: protection does not remove marked damage. §704.5g
	// fires on MarkedDamage ≥ toughness, full stop.
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("Bear with 2 marked damage must die at SBA; protection doesn't roll back marked damage (§702.16e)")
	}
	if countEvents(gs, "sba_704_5g") == 0 {
		t.Fatal("missing sba_704_5g event — protection must not gate §704.5g")
	}
	if got := len(gs.Seats[0].Graveyard); got != 1 {
		t.Fatalf("expected bear in graveyard, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// Counter DB Phase 3 — §704.5q pair-removal MUST run before §704.5f / 5g
// death-from-toughness within a single SBA pass.
//
// A creature with N +1/+1 + N -1/-1 cancels cleanly per §704.5q; if a
// dispatch ordering bug ran §704.5f first against the raw stat
// (base toughness minus N from the -1/-1 stack), a small-toughness
// creature would die before the cancellation got its chance.
//
// Setup: 1/1 base + 3 +1/+1 + 3 -1/-1 — should survive at 1/1 after the
// pair-removal SBA, with both stacks fully drained.
// -----------------------------------------------------------------------------

func TestSBA_704_5q_BeforeToughnessSBA_CreatureSurvives(t *testing.T) {
	gs := newFixtureGame(t)
	c := addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")
	c.AddCounter("+1/+1", 3)
	c.AddCounter("-1/-1", 3)

	StateBasedActions(gs)

	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("creature should survive when 3 +1/+1 + 3 -1/-1 cancel; battlefield has %d",
			len(gs.Seats[0].Battlefield))
	}
	if _, has := c.Counters["+1/+1"]; has {
		t.Errorf("+1/+1 must drain to zero (key removed), got %+v", c.Counters)
	}
	if _, has := c.Counters["-1/-1"]; has {
		t.Errorf("-1/-1 must drain to zero (key removed), got %+v", c.Counters)
	}
	if countEvents(gs, "sba_704_5q") == 0 {
		t.Fatal("missing sba_704_5q event — pair-removal must log every fire for CR audit")
	}
	if countEvents(gs, "sba_704_5g") > 0 {
		t.Fatal("creature must NOT die at §704.5g; counters cancel before death-from-toughness")
	}
}

// Counter DB Phase 3 — base-0 toughness still dies after counters cancel.
//
// A creature with base 0/0 + 5 +1/+1 + 5 -1/-1 cancels to 0/0; §704.5f
// then sends it to the graveyard the same pass. Pins that the pair-
// removal hoist hasn't broken the §704.5f path for genuinely toughness-0
// creatures — the death must still fire once the +1/+1 buff is gone.
func TestSBA_704_5q_BaseZeroToughnessDiesAfterCancel(t *testing.T) {
	gs := newFixtureGame(t)
	c := addBattlefield(gs, 0, "Zero Toughness Token", 0, 0, "creature")
	c.AddCounter("+1/+1", 5)
	c.AddCounter("-1/-1", 5)

	StateBasedActions(gs)

	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatalf("base-0 toughness creature must die after counters cancel; battlefield has %d",
			len(gs.Seats[0].Battlefield))
	}
	if countEvents(gs, "sba_704_5q") == 0 {
		t.Fatal("missing sba_704_5q event — pair-removal must fire before §704.5f")
	}
	if countEvents(gs, "sba_704_5f") == 0 {
		t.Fatal("missing sba_704_5f event — toughness-0 death must still fire post-cancellation")
	}
}
