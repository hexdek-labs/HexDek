package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSunSpiderNimbleWebber wires Sun-Spider, Nimble Webber.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	During your turn, Sun-Spider has flying.
//	When Sun-Spider enters, search your library for an Aura or Equipment
//	  card, reveal it, put it into your hand, then shuffle.
//
// Implementation:
//   - OnETB: search our library for an Aura or Equipment card. Heuristic
//     pick: highest-CMC Equipment first, else highest-CMC Aura. Put it
//     into our hand. (Library shuffle is implicit — we don't reorder
//     post-removal.)
//   - "During your turn, has flying" (R60 batch 9): wired via
//     upkeep_controller (stamp Flags["kw:flying"]=1 on own upkeep) +
//     end_step (own end step strips it). Per-flag marker
//     "sun_spider_self_flying" so the strip only removes flags we
//     granted (native AST flying from any future reprint stays).
func registerSunSpiderNimbleWebber(r *Registry) {
	r.OnETB("Sun-Spider, Nimble Webber", sunSpiderETB)
	r.OnTrigger("Sun-Spider, Nimble Webber", "upkeep_controller", sunSpiderUpkeepStampFlying)
	r.OnTrigger("Sun-Spider, Nimble Webber", "end_step", sunSpiderEndStepStripFlying)
}

func sunSpiderUpkeepStampFlying(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sun_spider_upkeep_grant_flying"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.HasKeyword("flying") {
		// Already has flying via AST or some other grant — don't stamp
		// our marker (we'd strip a native keyword on end_step).
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["sun_spider_self_flying"] == 1 {
		return
	}
	perm.Flags["kw:flying"] = 1
	perm.Flags["sun_spider_self_flying"] = 1
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func sunSpiderEndStepStripFlying(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sun_spider_end_step_strip_flying"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Flags == nil || perm.Flags["sun_spider_self_flying"] != 1 {
		return
	}
	delete(perm.Flags, "kw:flying")
	delete(perm.Flags, "sun_spider_self_flying")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func sunSpiderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sun_spider_etb_tutor"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	var bestEq *gameengine.Card
	bestEqCMC := -1
	var bestAura *gameengine.Card
	bestAuraCMC := -1
	for _, c := range seat.Library {
		if c == nil {
			continue
		}
		cm := gameengine.ManaCostOf(c)
		if cardHasType(c, "equipment") && cm > bestEqCMC {
			bestEqCMC = cm
			bestEq = c
		} else if cardHasType(c, "aura") && cm > bestAuraCMC {
			bestAuraCMC = cm
			bestAura = c
		}
	}
	pick := bestEq
	if pick == nil {
		pick = bestAura
	}
	if pick == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_aura_or_equipment_in_library", nil)
		return
	}
	gameengine.MoveCard(gs, pick, perm.Controller, "library", "hand", "sun_spider_tutor")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"found": pick.DisplayName(),
	})
}
