package gameengine

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// scaffold_aura_no_untap_r63.go — generic handler for the inert
// `aura_no_untap` AST scaffold KIND (r63 scaffold-kind coverage, ~63
// cards): an Aura whose static reads
//
//	"Enchanted creature/permanent doesn't untap during its controller's
//	 untap step."
//
// e.g. Waterknot, Shackles, Apathy, Roots, Capture Sphere, Bonds of
// Quicksilver, Tangle Kelp, Immobilizing Ink, Hold for Questioning. These
// are real lock-down / pseudo-removal auras; before this the static was
// logged and dropped (resolve_helpers no-op block), so the enchanted
// permanent untapped normally next turn — the aura did nothing lasting.
//
// The engine already honors a per-permanent DoesNotUntap flag in UntapAll
// (CR §502.2). Rather than write a boolean flag onto the enchanted
// permanent at ETB (which would go stale when the aura leaves, or fight
// other untap effects), this is a DYNAMIC check evaluated at the untap
// step: a permanent is held down iff an Aura with the aura_no_untap static
// is currently attached to it. No persistent state, so detachment / aura
// destruction / control change all resolve correctly for free.

// auraHoldsDownUntap reports whether some Aura currently attached to p
// carries the aura_no_untap static — i.e. p "doesn't untap during its
// controller's untap step." Called from UntapAll.
func auraHoldsDownUntap(gs *GameState, p *Permanent) bool {
	if gs == nil || p == nil {
		return false
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, a := range seat.Battlefield {
			if a == nil || a.AttachedTo != p {
				continue
			}
			if cardASTHasStaticKind(a.Card, "aura_no_untap") {
				return true
			}
		}
	}
	return false
}

// cardASTHasStaticKind reports whether card's AST declares a Static
// ability whose Modification.ModKind == kind. Layer-agnostic (these
// scaffold kinds carry no layer).
func cardASTHasStaticKind(card *Card, kind string) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil {
			continue
		}
		if st.Modification.ModKind == kind {
			return true
		}
	}
	return false
}
