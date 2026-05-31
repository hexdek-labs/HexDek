package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// R60 event-Kind normalization wave 2 (follow-up to PR #830 lose_game
// normalize). resolveUntap was direct-setting Tapped=false and emitting
// a non-canonical Kind="untap" event, silently bypassing the canonical
// UntapPermanent helper that owns:
//
//   - CR §702.124 Inspired trigger on tapped→untapped transition
//   - CR §122.4   stun-counter replacement (consume one stun counter
//                 instead of untapping)
//   - canonical Kind="untap_done" audit event with reason carried in
//                 Details for replay observability
//
// This file pins the post-normalize behavior on the engine-side
// resolveUntap path (driven by gameast.UntapEffect — the AST shape the
// parser emits for every "Untap target permanent" / "Untap target X"
// oracle phrase). Per-card sibling paths (Palinchron, Peregrine Drake,
// Dramatic Reversal) are pinned by the parallel test file in the
// per_card package.

// untapTestPerm builds a permanent on `seat`'s battlefield, tapped, with
// the supplied granted abilities (e.g. "inspired") attached. Returns
// the permanent. Test-local helper distinct from addBattlefield so
// tests reading this file see exactly what's being constructed without
// chasing fixture defaults.
func untapTestPerm(gs *GameState, seat int, name string, granted ...string) *Permanent {
	p := addBattlefield(gs, seat, name, 1, 1, "creature")
	p.Tapped = true
	p.GrantedAbilities = append(p.GrantedAbilities, granted...)
	return p
}

// TestResolveUntap_EmitsCanonicalUntapDoneKind pins the Kind
// normalization: resolveUntap must emit `untap_done` (the canonical
// helper's Kind), NOT the legacy `untap` Kind. Single-target case.
func TestResolveUntap_EmitsCanonicalUntapDoneKind(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Twiddle", 0, 0, "instant")
	target := untapTestPerm(gs, 1, "Mox Pearl")

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "permanent", OpponentControls: true, Targeted: true},
	})

	if target.Tapped {
		t.Errorf("target should be untapped after resolveUntap")
	}
	if c := countEvents(gs, "untap_done"); c != 1 {
		t.Errorf("expected 1 untap_done event (canonical), got %d", c)
	}
	if c := countEvents(gs, "untap"); c != 0 {
		t.Errorf("expected 0 untap events (legacy Kind, normalized out), got %d", c)
	}
}

// TestResolveUntap_FiresInspiredTriggerOnTransition is the load-bearing
// pin on the rules bug surfaced by this normalization. Inspired (CR
// §702.124) triggers when a permanent becomes untapped "for any reason,
// including untapping during an untap step or being untapped by an
// effect." Pre-normalize the resolveUntap path direct-set Tapped=false
// and Inspired was silently inert against every parsed "untap target
// creature" effect — Quicken Spirit, Fanatic of Xenagos, Felhide Spiritbinder,
// etc. were broken. Post-normalize, routing through UntapPermanent
// fires Inspired as the rules require.
func TestResolveUntap_FiresInspiredTriggerOnTransition(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Twiddle", 0, 0, "instant")
	target := untapTestPerm(gs, 1, "Pain Seer", "inspired")

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "permanent", OpponentControls: true, Targeted: true},
	})

	if target.Tapped {
		t.Errorf("target should be untapped post-resolve")
	}
	if c := countEvents(gs, "inspired_trigger"); c != 1 {
		t.Errorf("expected 1 inspired_trigger from §702.124 (helper now wired into resolveUntap), got %d", c)
	}
}

// TestResolveUntap_RespectsStunCounter pins §122.4: a permanent with a
// stun counter would-be-untapped → instead remove one stun counter and
// stay tapped. Pre-normalize this rule was bypassed for every parsed
// "untap target permanent" effect (stun counters were untouchable via
// direct effects, only honored at the untap step in phases.go).
func TestResolveUntap_RespectsStunCounter(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Twiddle", 0, 0, "instant")
	target := untapTestPerm(gs, 1, "Stunned Goblin")
	target.Counters["stun"] = 2

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "permanent", OpponentControls: true, Targeted: true},
	})

	if !target.Tapped {
		t.Errorf("target must STAY tapped — §122.4 stun counter consumes the would-untap")
	}
	if target.Counters["stun"] != 1 {
		t.Errorf("stun counters: got %d, want 1 (one consumed by would-untap)", target.Counters["stun"])
	}
	if c := countEvents(gs, "untap_done"); c != 0 {
		t.Errorf("expected 0 untap_done events when stun consumes the untap, got %d", c)
	}
	if c := countEvents(gs, "stun_counter_removed"); c != 1 {
		t.Errorf("expected 1 stun_counter_removed event from §122.4, got %d", c)
	}
}

// TestResolveUntap_NoOpOnAlreadyUntapped pins the negative: an
// already-untapped target should not re-fire Inspired (canonical
// helper has a `if !perm.Tapped { return false }` early-return). This
// defends against an over-eager normalization that would fire Inspired
// on every resolveUntap call regardless of starting state.
func TestResolveUntap_NoOpOnAlreadyUntapped(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Twiddle", 0, 0, "instant")
	target := untapTestPerm(gs, 1, "Pain Seer", "inspired")
	target.Tapped = false // pre-untapped

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "permanent", OpponentControls: true, Targeted: true},
	})

	if c := countEvents(gs, "untap_done"); c != 0 {
		t.Errorf("expected 0 untap_done events on already-untapped target, got %d", c)
	}
	if c := countEvents(gs, "inspired_trigger"); c != 0 {
		t.Errorf("expected 0 inspired_triggers on already-untapped target (no transition), got %d", c)
	}
}

// TestResolveUntap_LegacyKindNotEmittedByOtherUntapPaths cross-checks
// that no OTHER engine code path resurrects the legacy `untap` Kind
// during a resolveUntap call. Combined with the per_card sibling tests
// (palinchron / peregrine_drake / dramatic_reversal), this pins the
// invariant that `untap` is fully retired from the engine-driven untap
// surface.
func TestResolveUntap_LegacyKindNotEmittedByOtherUntapPaths(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Twiddle", 0, 0, "instant")
	// Two opponent permanents, one tapped, one already untapped.
	tapped := untapTestPerm(gs, 1, "Mox Pearl")
	untapped := addBattlefield(gs, 1, "Sol Ring", 0, 0, "artifact")
	untapped.Tapped = false

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "permanent", OpponentControls: true, Targeted: true},
	})

	_ = tapped
	if c := countEvents(gs, "untap"); c != 0 {
		t.Errorf("legacy `untap` Kind must not appear in event log after resolveUntap normalization; got %d", c)
	}
}
