package gameengine

// Counter DB Phase 2 — engine-side wiring tests for the keyword-grant
// family. The package-level counters tests pin the predicate; these
// tests pin that the predicate is consulted by *Permanent.HasKeyword,
// IsIndestructible, CanBeTargetedBy (hexproof), combat block legality
// (flying/reach), combat damage (lifelink/deathtouch), and the §122.6
// type-strip persistence across the real Permanent shape.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/counters"
)

// placeKeywordCounter is the test-side AddCounters convenience that
// drives the new CounterStacks path — distinct from the legacy
// Permanent.Counters map so we're confident the keyword grant flows
// through the §122.1c API, not the map fallback.
func placeKeywordCounter(t *testing.T, p *Permanent, kind string, count int) {
	t.Helper()
	placed, err := counters.AddCounters(p.AsCounterTarget(), kind, count, "h-test", 0)
	if err != nil {
		t.Fatalf("counters.AddCounters(%s, %d): %v", kind, count, err)
	}
	if placed != count {
		t.Fatalf("counters.AddCounters(%s, %d): placed %d", kind, count, placed)
	}
}

// TestHasKeywordViaCounterStack pins the chokepoint extension: a flying
// counter placed via the Counter DB satisfies Permanent.HasKeyword.
func TestHasKeywordViaCounterStack(t *testing.T) {
	gs := newCombatGame(t)
	p := addCreature(gs, 0, "Test Beast", 2, 2)

	if p.HasKeyword("flying") {
		t.Fatal("baseline HasKeyword(flying) = true, want false")
	}
	placeKeywordCounter(t, p, "flying", 1)
	if !p.HasKeyword("flying") {
		t.Errorf("post-placement HasKeyword(flying) = false, want true")
	}
}

// TestHasKeywordEveryKeywordCounterFamily sweeps the full 12-counter
// family through the engine chokepoint. If any keyword name fails the
// HasKeyword check, the wiring at that name (registry alias, predicate
// lookup, or chokepoint extension) is broken.
func TestHasKeywordEveryKeywordCounterFamily(t *testing.T) {
	kws := []string{
		"lifelink", "deathtouch", "flying", "first strike", "double strike",
		"hexproof", "indestructible", "menace", "reach", "trample",
		"vigilance", "ward",
	}
	for _, kw := range kws {
		gs := newCombatGame(t)
		p := addCreature(gs, 0, "Test "+kw, 2, 2)
		if p.HasKeyword(kw) {
			t.Errorf("%s: baseline HasKeyword = true, want false", kw)
			continue
		}
		placeKeywordCounter(t, p, kw, 1)
		if !p.HasKeyword(kw) {
			t.Errorf("%s: HasKeyword after counter placement = false, want true", kw)
		}
	}
}

// TestIsIndestructibleViaCounterStack pins SBA §704.5g respects the
// keyword-counter grant via the CounterStacks path. The Phase 2
// IsIndestructible extension is what closes the previously-open gap
// where this predicate didn't consult the Counter DB.
func TestIsIndestructibleViaCounterStack(t *testing.T) {
	gs := newCombatGame(t)
	p := addCreature(gs, 0, "Test Wall", 1, 1)
	if p.IsIndestructible() {
		t.Fatal("baseline IsIndestructible = true, want false")
	}
	placeKeywordCounter(t, p, "indestructible", 1)
	if !p.IsIndestructible() {
		t.Errorf("IsIndestructible after counter placement = false, want true")
	}
}

// TestHexproofViaCounterStack pins CanBeTargetedBy's §702.11 gate
// flows through the counter-granted keyword.
func TestHexproofViaCounterStack(t *testing.T) {
	gs := newCombatGame(t)
	p := addCreature(gs, 0, "Test Druid", 2, 2)
	if !CanBeTargetedBy(p, 1) {
		t.Fatal("baseline opponent-target = false, want true")
	}
	placeKeywordCounter(t, p, "hexproof", 1)
	if CanBeTargetedBy(p, 1) {
		t.Errorf("opponent-target with hexproof counter = true, want false (§702.11)")
	}
	if !CanBeTargetedBy(p, 0) {
		t.Errorf("controller-target with hexproof counter = false, want true")
	}
}

// TestFlyingCounterBlocksAsFlier exercises the canonical §509.1b block-
// legality call through CanBlock to confirm a flying-counter creature
// can no longer be blocked by a vanilla ground blocker.
func TestFlyingCounterBlocksAsFlier(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Test Lump", 2, 2)
	blk := addCreature(gs, 1, "Test Bear", 2, 2)
	if !CanBlock(atk, blk) {
		t.Fatal("baseline ground vs ground block = false, want true")
	}
	placeKeywordCounter(t, atk, "flying", 1)
	if CanBlock(atk, blk) {
		t.Errorf("ground blocker vs flying-counter attacker: CanBlock = true, want false")
	}

	// A reach-counter blocker should be able to block the flying attacker.
	reachBlk := addCreature(gs, 1, "Test Spider", 1, 3)
	placeKeywordCounter(t, reachBlk, "reach", 1)
	if !CanBlock(atk, reachBlk) {
		t.Errorf("reach-counter blocker vs flying-counter attacker: CanBlock = false, want true")
	}
}

// TestLifelinkCounterGainsLifeOnDamage exercises the §702.15 damage-
// dealing path through ApplyCombatDamageWithCombatTriggers indirectly
// via a full combat phase. The lifelink counter must cause the
// controller's life total to rise by the damage dealt.
func TestLifelinkCounterGainsLifeOnDamage(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Test Cleric", 3, 3)
	placeKeywordCounter(t, atk, "lifelink", 1)

	startLife := gs.Seats[0].Life
	CombatPhase(gs)
	gotGain := gs.Seats[0].Life - startLife
	if gotGain < 3 {
		t.Errorf("lifelink counter: seat 0 life gain = %d, want >= 3", gotGain)
	}
}

// TestVigilanceCounterSkipsTap exercises CR §702.20 / §508.1f via
// DeclareAttackers. A creature with a vigilance counter must not be
// tapped after declaring an attack.
func TestVigilanceCounterSkipsTap(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Test Knight", 2, 2)
	placeKeywordCounter(t, atk, "vigilance", 1)

	CombatPhase(gs)
	if atk.Tapped {
		t.Errorf("attacker with vigilance counter ended combat Tapped = true, want false")
	}
}

// TestKeywordCounterPersistsThroughTypeStripPermanent pins §122.6 on
// the real Permanent shape. A type-strip simulation (clearing
// Card.Types) must not erase the granted keyword — the counter is
// authoritative until removed.
func TestKeywordCounterPersistsThroughTypeStripPermanent(t *testing.T) {
	gs := newCombatGame(t)
	p := addCreature(gs, 0, "Test Beast", 2, 2)
	placeKeywordCounter(t, p, "lifelink", 1)
	if !p.HasKeyword("lifelink") {
		t.Fatal("baseline HasKeyword(lifelink) = false")
	}

	// Humility-style strip: clear card types entirely.
	p.Card.Types = nil

	if !p.HasKeyword("lifelink") {
		t.Errorf("post-strip HasKeyword(lifelink) = false, want true (§122.6)")
	}
	if counters.CounterCount(p.AsCounterTarget(), "lifelink") != 1 {
		t.Errorf("post-strip CounterCount(lifelink) = %d, want 1",
			counters.CounterCount(p.AsCounterTarget(), "lifelink"))
	}
}

// TestPermanentTargetAdapter pins the wrapper translates correctly so
// future counters package APIs Permanent passes through end-to-end.
func TestPermanentTargetAdapter(t *testing.T) {
	gs := newCombatGame(t)
	p := addCreature(gs, 0, "Test Creature", 1, 1)
	tgt := p.AsCounterTarget()
	if !tgt.HasCardType("creature") {
		t.Errorf("HasCardType(creature) = false, want true")
	}
	if tgt.HasCardType("artifact") {
		t.Errorf("HasCardType(artifact) = true, want false")
	}
	tgt.SetCounterStacks([]counters.CounterStack{{Type: "flying", Count: 1}})
	if len(p.CounterStacks) != 1 || p.CounterStacks[0].Type != "flying" {
		t.Errorf("SetCounterStacks did not flow to Permanent: %+v", p.CounterStacks)
	}
	got := tgt.CounterStacks()
	if len(got) != 1 || got[0].Type != "flying" {
		t.Errorf("CounterStacks() = %+v, want one flying stack", got)
	}
}
