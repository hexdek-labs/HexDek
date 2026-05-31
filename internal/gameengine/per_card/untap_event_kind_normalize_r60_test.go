package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 event-Kind normalization wave 2 (follow-up to PR #830 lose_game
// normalize). Three per_card handlers — Palinchron, Peregrine Drake,
// and Dramatic Reversal — were direct-setting Tapped=false on each
// permanent they untapped and emitting a non-canonical Kind="untap"
// audit event, silently bypassing the canonical UntapPermanent helper
// which owns:
//
//   - CR §702.124 Inspired trigger on tapped→untapped transition
//   - CR §122.4   stun-counter replacement (consume one stun counter
//                 instead of untapping)
//   - canonical Kind="untap_done" audit event with reason carried in
//                 Details for replay observability
//
// Of the three, Dramatic Reversal is the only one untapping creatures
// (Palinchron and Peregrine Drake only untap lands, where Inspired
// can't apply). So the load-bearing rules bug surfaced by this
// normalization specifically affected Dramatic Reversal: an Inspired
// creature your Isochron Scepter+Dramatic Reversal loop untapped did
// not trigger Inspired pre-fix. Post-fix, routing through
// UntapPermanent fires Inspired as §702.124 requires.
//
// This file pins the per_card paths. The engine-side resolveUntap
// path (sibling normalization in the same PR) is pinned by the
// parallel test file at internal/gameengine/untap_event_kind_normalize_r60_test.go.

// untapTestTappedPerm puts a tapped permanent on `seat`'s battlefield
// and returns it. Supplied `granted` ability strings (e.g. "inspired")
// are appended to GrantedAbilities so HasInspiredPerm picks them up.
func untapTestTappedPerm(gs *gameengine.GameState, seat int, name string, types []string, granted ...string) *gameengine.Permanent {
	p := addPerm(gs, seat, name, types...)
	p.Tapped = true
	p.GrantedAbilities = append(p.GrantedAbilities, granted...)
	return p
}

// TestPalinchron_EmitsCanonicalUntapDoneKind pins the Kind
// normalization on the Palinchron ETB path. Lands only — Inspired
// won't fire, but the canonical helper is mandatory regardless so
// invariants and Heimdall observers see one consistent untap Kind
// across every untap surface.
func TestPalinchron_EmitsCanonicalUntapDoneKind(t *testing.T) {
	gs := newGame(t, 2)
	palinchron := addPerm(gs, 0, "Palinchron", "creature")
	// Three tapped lands.
	for _, name := range []string{"Island #1", "Island #2", "Island #3"} {
		untapTestTappedPerm(gs, 0, name, []string{"land"})
	}

	gameengine.InvokeETBHook(gs, palinchron)

	if c := hasEvent(gs, "untap_done"); c != 3 {
		t.Errorf("expected 3 untap_done events (one per land), got %d", c)
	}
	if c := hasEvent(gs, "untap"); c != 0 {
		t.Errorf("expected 0 untap events (legacy Kind, normalized out), got %d", c)
	}
}

// TestPeregrineDrake_EmitsCanonicalUntapDoneKind mirrors the
// Palinchron test for the Peregrine Drake ETB path. Same shape,
// 5-land limit instead of 7.
func TestPeregrineDrake_EmitsCanonicalUntapDoneKind(t *testing.T) {
	gs := newGame(t, 2)
	drake := addPerm(gs, 0, "Peregrine Drake", "creature")
	for _, name := range []string{"Island #1", "Island #2"} {
		untapTestTappedPerm(gs, 0, name, []string{"land"})
	}

	gameengine.InvokeETBHook(gs, drake)

	if c := hasEvent(gs, "untap_done"); c != 2 {
		t.Errorf("expected 2 untap_done events, got %d", c)
	}
	if c := hasEvent(gs, "untap"); c != 0 {
		t.Errorf("expected 0 untap events, got %d", c)
	}
}

// TestDramaticReversal_EmitsCanonicalUntapDoneKind pins the Kind
// normalization on the Dramatic Reversal resolve path. Mixes
// artifacts and a tapped creature to exercise the all-nonland sweep.
func TestDramaticReversal_EmitsCanonicalUntapDoneKind(t *testing.T) {
	gs := newGame(t, 2)
	untapTestTappedPerm(gs, 0, "Sol Ring", []string{"artifact"})
	untapTestTappedPerm(gs, 0, "Mox Pearl", []string{"artifact"})
	untapTestTappedPerm(gs, 0, "Llanowar Elves", []string{"creature"})

	card := addCard(gs, 0, "Dramatic Reversal", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if c := hasEvent(gs, "untap_done"); c != 3 {
		t.Errorf("expected 3 untap_done events (artifact + artifact + creature), got %d", c)
	}
	if c := hasEvent(gs, "untap"); c != 0 {
		t.Errorf("expected 0 untap events (legacy Kind, normalized out), got %d", c)
	}
}

// TestDramaticReversal_FiresInspiredTrigger is the load-bearing pin
// on the rules bug surfaced by this normalization. Inspired (§702.124)
// triggers when a permanent becomes untapped "for any reason,
// including untapping during an untap step or being untapped by an
// effect." Pre-normalize, Isochron Scepter + Dramatic Reversal loops
// untapped Inspired creatures without firing their abilities. Post-
// normalize, routing through UntapPermanent fires Inspired correctly.
func TestDramaticReversal_FiresInspiredTrigger(t *testing.T) {
	gs := newGame(t, 2)
	pain := untapTestTappedPerm(gs, 0, "Pain Seer", []string{"creature"}, "inspired")
	// Sanity: confirm the granted-ability route exposes Inspired.
	if !gameengine.HasInspiredPerm(pain) {
		t.Fatalf("test fixture: HasInspiredPerm returned false on a permanent with `inspired` in GrantedAbilities — granted-ability detection regressed")
	}

	card := addCard(gs, 0, "Dramatic Reversal", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if pain.Tapped {
		t.Errorf("Pain Seer should be untapped post-resolve")
	}
	if c := hasEvent(gs, "inspired_trigger"); c != 1 {
		t.Errorf("expected 1 inspired_trigger from §702.124 on Pain Seer (UntapPermanent now wired into Dramatic Reversal), got %d", c)
	}
}

// TestDramaticReversal_RespectsStunCounter pins §122.4 on the
// Dramatic Reversal path: a tapped creature with a stun counter must
// stay tapped and consume one stun counter rather than untap. Pre-
// normalize this rule was silently bypassed on every Dramatic Reversal
// resolve.
func TestDramaticReversal_RespectsStunCounter(t *testing.T) {
	gs := newGame(t, 2)
	stunned := untapTestTappedPerm(gs, 0, "Stunned Goblin", []string{"creature"})
	stunned.Counters["stun"] = 1
	regular := untapTestTappedPerm(gs, 0, "Sol Ring", []string{"artifact"})

	card := addCard(gs, 0, "Dramatic Reversal", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !stunned.Tapped {
		t.Errorf("Stunned Goblin must STAY tapped — §122.4 stun counter consumes the would-untap")
	}
	if stunned.Counters["stun"] != 0 {
		t.Errorf("stun counters: got %d, want 0 (one consumed by would-untap)", stunned.Counters["stun"])
	}
	if regular.Tapped {
		t.Errorf("Sol Ring should still be untapped — no stun counter on it")
	}
	if c := hasEvent(gs, "untap_done"); c != 1 {
		t.Errorf("expected exactly 1 untap_done (Sol Ring; the Goblin was vetoed by stun), got %d", c)
	}
	if c := hasEvent(gs, "stun_counter_removed"); c != 1 {
		t.Errorf("expected 1 stun_counter_removed from §122.4, got %d", c)
	}
}

// TestPalinchron_RespectsStunCounter cross-checks the stun-counter
// path on the Palinchron handler. Lands can carry stun counters too
// (Brine Elemental, Hex Parasite + dark depths shenanigans), so the
// rule applies. Defends against over-narrow normalization that wires
// the helper for Dramatic Reversal but skips the land-only handlers.
func TestPalinchron_RespectsStunCounter(t *testing.T) {
	gs := newGame(t, 2)
	palinchron := addPerm(gs, 0, "Palinchron", "creature")
	stunned := untapTestTappedPerm(gs, 0, "Stunned Island", []string{"land"})
	stunned.Counters["stun"] = 1
	regular := untapTestTappedPerm(gs, 0, "Regular Island", []string{"land"})

	gameengine.InvokeETBHook(gs, palinchron)

	if !stunned.Tapped {
		t.Errorf("Stunned Island must STAY tapped — §122.4 stun counter consumes the would-untap")
	}
	if regular.Tapped {
		t.Errorf("Regular Island should be untapped")
	}
	if c := hasEvent(gs, "untap_done"); c != 1 {
		t.Errorf("expected exactly 1 untap_done (Regular Island; Stunned Island vetoed), got %d", c)
	}
}
