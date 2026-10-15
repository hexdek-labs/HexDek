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
// collectAuraNoUntapHosts returns the set of permanents currently held down
// by an attached `aura_no_untap` Aura, in ONE pass over the battlefields.
//
// r116 — PERFORMANCE, semantics unchanged. This replaces a per-permanent
// predicate (`auraHoldsDownUntap`) that walked every seat's entire
// battlefield on each call. `UntapAll` called it once per permanent,
// UNGATED, so the untap step was O(n^2) in board size — every turn of every
// game, and paid in full on the overwhelming majority of boards that contain
// no such Aura at all. Measured before the change: UntapAll ~n^1.97 at the
// large end, 7.4ms per untap step at n=2560, with allocations linear (so the
// cost was CPU in the scan). See docs/untap-step-quadratic-r116.md.
//
// Same shape as r112's sba704_5y fix, and found by the static detector
// written to enforce that round's SBA SCAN CONTRACT.
//
// Returns nil when no such Aura is on the battlefield, which is the common
// case; a nil map reads as empty, so the caller needs no special case.
func collectAuraNoUntapHosts(gs *GameState) map[*Permanent]bool {
	if gs == nil {
		return nil
	}
	var held map[*Permanent]bool
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, a := range seat.Battlefield {
			if a == nil || a.AttachedTo == nil {
				continue
			}
			if !cardASTHasStaticKind(a.Card, "aura_no_untap") {
				continue
			}
			if held == nil {
				held = map[*Permanent]bool{}
			}
			held[a.AttachedTo] = true
		}
	}
	return held
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
