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
func registerTannukSteadfastSecond(r *Registry) {
	r.OnETB("Tannuk, Steadfast Second", tannukETBHasteAnthem)
	r.OnTrigger("Tannuk, Steadfast Second", "permanent_etb", tannukRefreshHaste)
	// R50 batch G: LTB clears the warp-grant seat flag so a stolen /
	// exiled Tannuk doesn't leave the alt-cost grant contract active
	// for downstream consumers.
	r.OnTrigger("Tannuk, Steadfast Second", "permanent_ltb", tannukLTBClearFlag)
}

func tannukLTBClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	delete(seat.Flags, "tannuk_warp_grant_2r_active")
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
