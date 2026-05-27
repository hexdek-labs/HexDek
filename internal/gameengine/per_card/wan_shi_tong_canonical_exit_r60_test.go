package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// wan_shi_tong_canonical_exit_r60_test.go — regression tests for the r60
// canonical battlefield-exit sweep extension. Wan Shi Tong, All-Knowing
// was the last surviving offender from the May 11 grinder forensics
// sibling list (CLAUDE.md issue log 2026-05-16): its ETB tuck previously
// routed the target's library-bottom move through a manual
// removePermanent + DetachAll + library-append, bypassing
// UnregisterReplacementsForPermanent, UnregisterContinuousEffectsForPermanent,
// ExpireSourceGrants, the §614 replacement chain (which drives §903.9b
// commander redirect among other things), and FireZoneChangeTriggers.
//
// Each test below asserts a specific piece of canonical machinery now
// fires through BouncePermanent(..., "library_bottom").

// TestWanShiTong_TuckFiresZoneChangeReplacement pins that the §614
// replacement chain is now consulted on the battlefield → library move.
// Pre-r60, no replacement-watcher ever fired because the manual path
// went straight from gs.removePermanent to a library slice append.
func TestWanShiTong_TuckFiresZoneChangeReplacement(t *testing.T) {
	gs := newGame(t, 2)
	wst := addPerm(gs, 0, "Wan Shi Tong, All-Knowing", "creature", "legendary")
	target := addPerm(gs, 1, "Grizzly Bears", "creature")
	target.Card.BasePower = 2
	target.Card.BaseToughness = 2

	zoneWatch := registerReplacementWatcher(gs, "would_change_zone", false)

	wanShiTongETB(gs, wst)

	if *zoneWatch == 0 {
		t.Error("zone_change replacement was not consulted — handler still bypasses §614 on tuck")
	}
}

// TestWanShiTong_AuraDetachesOnTuck pins that DetachAll fires for the
// tucked permanent. Pre-r60 the manual path called DetachAll explicitly
// so this happened to work — but routing through BouncePermanent has to
// preserve the same behavior. Defensive regression so a future
// "simplify the handler" change doesn't drop DetachAll without
// rediscovering why it was load-bearing.
func TestWanShiTong_AuraDetachesOnTuck(t *testing.T) {
	gs := newGame(t, 2)
	wst := addPerm(gs, 0, "Wan Shi Tong, All-Knowing", "creature", "legendary")
	target := addPerm(gs, 1, "Grizzly Bears", "creature")
	target.Card.BasePower = 2
	target.Card.BaseToughness = 2
	aura := addPerm(gs, 1, "Pacifism", "enchantment", "aura")
	attachAura(aura, target)

	wanShiTongETB(gs, wst)

	if aura.AttachedTo == target {
		t.Error("aura still attached to tucked creature — DetachAll did not fire")
	}
}

// TestWanShiTong_TuckEmitsBounceEvent pins that BouncePermanent's
// canonical "bounce" event log entry is emitted (in addition to the
// handler's existing "tuck_to_library" semantic event). Pre-r60 only
// the latter was emitted, so post-game replay tooling and Loki
// invariant scanners that watch for canonical "bounce" events on
// battlefield-exits had no signal.
func TestWanShiTong_TuckEmitsBounceEvent(t *testing.T) {
	gs := newGame(t, 2)
	wst := addPerm(gs, 0, "Wan Shi Tong, All-Knowing", "creature", "legendary")
	addPerm(gs, 1, "Grizzly Bears", "creature")

	wanShiTongETB(gs, wst)

	if hasEvent(gs, "bounce") < 1 {
		t.Errorf("expected canonical 'bounce' event from BouncePermanent; got events: %+v",
			eventKinds(gs))
	}
	if hasEvent(gs, "tuck_to_library") < 1 {
		t.Errorf("expected handler 'tuck_to_library' event; got events: %+v",
			eventKinds(gs))
	}
}

// TestWanShiTong_TuckUnregistersReplacements pins the §614-Platinum-Angel
// bypass the manual path was creating. Wan Shi Tong tucks any nonland
// permanent — and "any nonland permanent" includes Platinum Angel
// itself, which registers a `would_lose_game` cancel replacement. Pre-r60
// the manual remove+append left the replacement in gs.Replacements after
// the source had left play; same phantom-replacement family as the
// 2026-05-25 SBACompleteness x6 fix (which added a pickReplacement
// backstop). The proper LTB-side unregister is what BouncePermanent
// already does for every other battlefield exit.
func TestWanShiTong_TuckUnregistersReplacements(t *testing.T) {
	gs := newGame(t, 2)
	wst := addPerm(gs, 0, "Wan Shi Tong, All-Knowing", "creature", "legendary")
	target := addPerm(gs, 1, "Platinum Angel", "creature", "legendary")
	target.Card.BasePower = 4
	target.Card.BaseToughness = 4

	// Register a `would_lose_game` cancel-handler whose SourcePerm is the
	// soon-to-be-tucked Platinum Angel. After the tuck, this handler must
	// no longer be present in gs.Replacements.
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_lose_game",
		HandlerID:      "test:platinum_angel:would_lose_game",
		ControllerSeat: 1,
		SourcePerm:     target,
		Timestamp:      target.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			return true
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.Cancelled = true
		},
	})

	wanShiTongETB(gs, wst)

	for _, r := range gs.Replacements {
		if r.HandlerID == "test:platinum_angel:would_lose_game" {
			t.Error("Platinum Angel's would_lose_game replacement survived tuck — handler is still phantom-leaking")
		}
	}
}
