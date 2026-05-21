package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArchelosLagoonMystic wires Archelos, Lagoon Mystic.
//
// Oracle text:
//
//	As long as Archelos, Lagoon Mystic is tapped, other permanents
//	enter tapped.
//	As long as Archelos, Lagoon Mystic is untapped, other permanents
//	enter untapped (or replace any other "enters tapped" effects).
//
// Implementation:
//   - Game-level flag tracking which seat (if any) controls a tapped
//     Archelos: gs.Flags["archelos_tapped_seat"] = controller+1.
//   - On ETB and on permanent_etb refresh, recompute the flag based on
//     current Archelos tap state. The actual "enter tapped" replacement
//     is engine-deep (every other ETB has to consult this flag); we set
//     it for downstream consumers and emit a partial breadcrumb.
//   - For permanents entering AFTER Archelos while Archelos is tapped,
//     stamp them with Tapped=true at our end of the ETB chain. The
//     engine's permanent_etb fires for the new perm; we read perm
//     from ctx and tap it if Archelos is currently tapped.
func registerArchelosLagoonMystic(r *Registry) {
	r.OnETB("Archelos, Lagoon Mystic", archelosETBSetFlag)
	r.OnTrigger("Archelos, Lagoon Mystic", "permanent_etb", archelosOnPermanentETB)
	// R50 batch G: LTB hook clears the game-level archelos_tapped_seat
	// + archelos_etb_mode flags so a removed Archelos doesn't leave the
	// ETB-tap replacement contract active for downstream consumers.
	r.OnTrigger("Archelos, Lagoon Mystic", "permanent_ltb", archelosLTBClearFlag)
}

func archelosLTBClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	if gs.Flags == nil {
		return
	}
	delete(gs.Flags, "archelos_tapped_seat")
	delete(gs.Flags, "archelos_etb_mode")
}

func archelosETBSetFlag(gs *gameengine.GameState, perm *gameengine.Permanent) {
	archelosRecomputeFlag(gs, perm)
}

func archelosOnPermanentETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	archelosRecomputeFlag(gs, perm)
	// Best-effort retroactive ETB-tap: when Archelos is currently tapped,
	// tap the new permanent that just entered (any controller — Archelos
	// affects ALL other permanents).
	if !perm.Tapped {
		return
	}
	newcomer, _ := ctx["perm"].(*gameengine.Permanent)
	if newcomer == nil || newcomer == perm {
		return
	}
	newcomer.Tapped = true
}

func archelosRecomputeFlag(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "archelos_lagoon_mystic_etb_replacement"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if perm.Tapped {
		gs.Flags["archelos_tapped_seat"] = perm.Controller + 1
		gs.Flags["archelos_etb_mode"] = 1 // 1 = enter tapped
	} else {
		gs.Flags["archelos_tapped_seat"] = 0
		gs.Flags["archelos_etb_mode"] = 2 // 2 = enter untapped
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tapped": perm.Tapped,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"true ETB-tap replacement needs ETB replacement hook; flag set for downstream + retroactive tap on permanent_etb")
}
