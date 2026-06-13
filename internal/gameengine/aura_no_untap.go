package gameengine

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// aura_no_untap.go — generic handler for the `aura_no_untap` static ability
// KIND: "Enchanted permanent doesn't untap during its controller's untap
// step" (Waterknot, Claustrophobia, Bonds of Quicksilver, Sleep-style tapper
// auras — ~63 cards). The kind sat in the log-only static bucket, so these
// auras conferred NOTHING: the enchanted creature untapped every turn,
// neutering the whole tap-lock card class.
//
// collectNoUntapAuraTargets does one pass over all battlefields, finds auras
// carrying an aura_no_untap static, and returns the set of permanents they are
// attached to. UntapAll consults this set and skips untapping those permanents
// (alongside the existing DoesNotUntap flag). The check is a pure read of
// current attachment state (via AttachedTo), so it needs no persistent flag
// and self-corrects the moment the aura leaves / moves.
func collectNoUntapAuraTargets(gs *GameState) map[*Permanent]bool {
	if gs == nil {
		return nil
	}
	var out map[*Permanent]bool
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, aura := range s.Battlefield {
			if aura == nil || aura.AttachedTo == nil || aura.Card == nil || aura.Card.AST == nil {
				continue
			}
			if aura.Flags != nil && aura.Flags["face_down"] != 0 {
				continue // CR §708.4 — face-down has no abilities
			}
			if !auraHasNoUntapStatic(aura) {
				continue
			}
			if out == nil {
				out = map[*Permanent]bool{}
			}
			out[aura.AttachedTo] = true
		}
	}
	return out
}

func auraHasNoUntapStatic(aura *Permanent) bool {
	for _, ab := range aura.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if ok && st.Modification != nil && st.Modification.ModKind == "aura_no_untap" {
			return true
		}
	}
	return false
}
