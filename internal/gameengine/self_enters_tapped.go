package gameengine

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// self_enters_tapped.go — generic handler for the unconditional
// `self_enters_tapped` / `enters_tapped` static ability KIND (CR §614.1d):
// "This permanent enters the battlefield tapped."
//
// ~562 permanents (Guildgates, Temples, Karoo bouncelands, the cycle of
// "comes into play tapped" mana rocks like Moss Diamond, tapped-entering
// creatures, etc.) carry this as a Static ability whose Modification.ModKind
// is self_enters_tapped/enters_tapped with NO args. Before this handler the
// kind only had an effect-path case in resolveModificationEffect, which is
// never reached for a self-static at ETB — so every one of these permanents
// entered UNTAPPED. ApplySelfEntersTapped fixes that at the ETB chokepoint,
// mirroring ApplyStaticETBCounters / ApplyEntersTappedUnless.
//
// Scope guard: ONLY the unconditional (zero-arg) form is handled here. The
// parser also mis-buckets ~20 "This creature enters PREPARED" cards under
// this kind with Args == ["prepared"] — a distinct mechanic (handled by the
// becomes_prepared path). Those are skipped so this handler never interferes
// with prepared.
func ApplySelfEntersTapped(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return
	}
	// Face-down permanents have no abilities (CR §708.4).
	if perm.Flags != nil && perm.Flags["face_down"] != 0 {
		return
	}
	// Already tapped (e.g. by etb_tapped_unless or another replacement) — no
	// need to scan, and tapping twice is harmless anyway.
	if perm.Tapped {
		return
	}
	for _, ab := range perm.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil {
			continue
		}
		k := st.Modification.ModKind
		if k != "self_enters_tapped" && k != "enters_tapped" {
			continue
		}
		// Only the unconditional, zero-arg form. The "enters prepared"
		// mis-bucket carries Args == ["prepared"]; leave it alone.
		if len(st.Modification.Args) != 0 {
			continue
		}
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		perm.Flags["enters_tapped"] = 1
		perm.Tapped = true
		gs.LogEvent(Event{
			Kind:   "enters_tapped",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"mod_kind": k,
				"path":     "static_self_replacement",
			},
		})
		return
	}
}
