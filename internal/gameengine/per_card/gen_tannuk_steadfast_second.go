package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTannukSteadfastSecond wires Tannuk, Steadfast Second.
//
// Oracle text:
//
//	Other creatures you control have haste.
//	Artifact cards and red creature cards in your hand have warp {2}{R}.
//	(You may cast a card from your hand for its warp cost. Exile that
//	permanent at the beginning of the next end step, then you may cast
//	it from exile on a later turn.)
//
// Implementation:
//   - "Other creatures have haste": stamp kw:haste on every other
//     creature on the controller's battlefield via permanent_etb refresh.
//   - Warp cost grant on hand cards: engine-deep alt-cost path; set a
//     per-seat flag and emit partial.
//   - LTB cleanup (R50 batch F + batch G merged): when the last Tannuk
//     leaves the controller's battlefield, strip the anthem-granted
//     kw:haste from creatures that don't have native haste and clear
//     the warp-grant seat flag.
func registerTannukSteadfastSecond(r *Registry) {
	r.OnETB("Tannuk, Steadfast Second", tannukETBHasteAnthem)
	r.OnTrigger("Tannuk, Steadfast Second", "permanent_etb", tannukRefreshHaste)
	r.OnTrigger("Tannuk, Steadfast Second", "permanent_ltb", tannukLTBCleanup)
}

// tannukLTBClearFlag is a kept-alive alias for the pre-merge function
// name. Removed in a follow-up sweep.
func tannukLTBClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	tannukLTBCleanup(gs, perm, ctx)
}

func tannukLTBCleanup(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tannuk_steadfast_second_ltb_cleanup"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	// Confirm no OTHER Tannuk still in play before clearing the anthem
	// or the seat flag.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p == perm || p.Card == nil {
				continue
			}
			if normalizeName(p.Card.DisplayName()) == normalizeName("Tannuk, Steadfast Second") {
				return
			}
		}
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	cleared := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Flags == nil {
			continue
		}
		// Only clear the anthem-granted haste; native-haste cards have
		// "haste" baked into their Types tag so the AST reapplies it.
		if p.Flags["kw:haste"] == 1 && p.Card != nil && !cardHasType(p.Card, "haste") {
			delete(p.Flags, "kw:haste")
			cleared++
		}
	}
	if seat.Flags != nil {
		delete(seat.Flags, "tannuk_warp_grant_2r_active")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"haste_cleared": cleared,
	})
}

func tannukETBHasteAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tannuk_steadfast_second_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["tannuk_warp_grant_2r_active"] = 1
	tannukApplyHaste(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"hand-card warp {2R} alt-cost grant needs cost-modifier hook; flag set for downstream")
}

func tannukRefreshHaste(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	tannukApplyHaste(gs, perm)
}

func tannukApplyHaste(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:haste"] = 1
	}
}
