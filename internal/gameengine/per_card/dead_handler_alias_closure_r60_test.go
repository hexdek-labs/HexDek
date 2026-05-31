package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 engine-event-registry dead-handler closure (docs/engine-event-registry.md
// §3.1). The audit found two parser/per_card author conventions
// registered handlers on event names the engine never fired:
//
//   - "untap_step"           — Seedborn Muse, Rasputin Dreamweaver
//   - "draw_step_controller" — Sylvan Library, Nekusar the Mindrazer
//
// The engine fires "untap" + "draw_step" respectively from
// phases.go:FirePhaseTriggers. Adding the aliases routes the
// handlers via NormalizeEventSingle on both sides of the
// registration/dispatch flow (registry.go:307 registers under
// canonical, registry.go:516 looks up by canonical).
//
// This file pins the end-to-end load-bearing behavior: firing the
// engine canonical event must now reach the per_card handler that
// was registered with the alias name. Pre-alias, the handler was
// silently dead — Seedborn Muse's "untap all your permanents on
// opponents' untap steps" never fired, breaking every cEDH deck
// running it.

// TestSeedbornMuse_FiresOnEngineUntapEvent is the load-bearing pin
// on the "untap_step" → "untap" alias closure. Seedborn's handler
// registers with "untap_step"; the engine fires "untap" from
// phases.go on the untap step boundary; alias routes them.
//
// Setup: seat 1 is on the active untap step (untap step of an
// opponent from Seedborn's seat-0 perspective). Seedborn is on seat
// 0's battlefield with three other tapped permanents. The trigger
// should untap all three.
func TestSeedbornMuse_FiresOnEngineUntapEvent(t *testing.T) {
	gs := newGame(t, 2)
	seedborn := addPerm(gs, 0, "Seedborn Muse", "creature")
	_ = seedborn
	// Three tapped permanents on seat 0.
	p1 := addPerm(gs, 0, "Sol Ring", "artifact")
	p1.Tapped = true
	p2 := addPerm(gs, 0, "Llanowar Elves", "creature")
	p2.Tapped = true
	p3 := addPerm(gs, 0, "Forest", "land")
	p3.Tapped = true

	// Engine fires "untap" with active_seat=1 (opponent's untap step).
	gameengine.FireCardTrigger(gs, "untap", map[string]interface{}{
		"active_seat": 1,
		"phase":       "beginning",
		"step":        "untap",
	})

	if p1.Tapped || p2.Tapped || p3.Tapped {
		t.Errorf("Seedborn Muse should have untapped all 3 of controller's permanents on opponent's untap step (alias closure for untap_step → untap)")
	}
}

// TestSeedbornMuse_DoesNotFireOnOwnUntapStep pins Seedborn's
// active-seat gate. The handler short-circuits when
// active_seat == controller (CR §503: your untap step handles
// untaps normally; Seedborn's bonus only fires on OTHER players').
// Defends against the alias closure accidentally over-firing the
// handler.
func TestSeedbornMuse_DoesNotFireOnOwnUntapStep(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Seedborn Muse", "creature")
	tapped := addPerm(gs, 0, "Sol Ring", "artifact")
	tapped.Tapped = true

	gameengine.FireCardTrigger(gs, "untap", map[string]interface{}{
		"active_seat": 0, // Seedborn's controller's own untap step
		"phase":       "beginning",
		"step":        "untap",
	})

	if !tapped.Tapped {
		t.Errorf("Seedborn Muse should NOT have re-untapped on its controller's own untap step (handler's active_seat gate must still fire post-alias)")
	}
}

// TestSylvanLibrary_FiresOnEngineDrawStepEvent pins the alias
// closure for "draw_step_controller" → "draw_step". Sylvan Library
// is registered with the controller-scoped phase name; engine fires
// the bare name from phases.go. Pre-alias, Sylvan Library's "draw
// two extra cards" beginning-of-draw-step trigger was dead in
// production.
//
// Setup: Sylvan Library on seat 0's battlefield, library has 4
// cards on top. Engine fires "draw_step" with active_seat=0
// (Sylvan's controller's own draw step). Sylvan should draw 2 more.
func TestSylvanLibrary_FiresOnEngineDrawStepEvent(t *testing.T) {
	gs := newGame(t, 2)
	sylvan := addPerm(gs, 0, "Sylvan Library", "enchantment")
	_ = sylvan
	addLibrary(gs, 0, "Top 1", "Top 2", "Top 3", "Top 4")
	preLibSize := len(gs.Seats[0].Library)

	gameengine.FireCardTrigger(gs, "draw_step", map[string]interface{}{
		"active_seat": 0,
		"phase":       "beginning",
		"step":        "draw",
	})

	// Sylvan draws 2 cards from the library (it may tuck some back, but
	// the LIBRARY shrinks by 2 net — drawOne pulls 2, MoveCard tucks
	// from hand to library_top which doesn't change library count for
	// our purposes since the tuck targets library_top via MoveCard
	// which appends. So minimum library delta is 2 - tucked.) The
	// robust assertion: the per_card_handler audit emit fires with the
	// sylvan_library_draw slug → proves the handler RAN at all (which
	// is the alias closure we're pinning).
	matched := 0
	for _, ev := range gs.EventLog {
		if ev.Kind != "per_card_handler" {
			continue
		}
		if slug, _ := ev.Details["slug"].(string); slug == "sylvan_library_draw" {
			matched++
		}
	}
	if matched == 0 {
		t.Errorf("Sylvan Library handler must run on draw_step (per_card_handler emit with slug=sylvan_library_draw missing); alias draw_step_controller → draw_step closure broken. lib_pre=%d lib_post=%d", preLibSize, len(gs.Seats[0].Library))
	}
}
