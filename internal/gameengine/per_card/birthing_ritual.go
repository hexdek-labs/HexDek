package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBirthingRitual wires Birthing Ritual.
//
// Oracle text (Scryfall, verified 2026-05-16):
//
//	At the beginning of your end step, if you control a creature, look
//	at the top seven cards of your library. Then you may sacrifice a
//	creature. If you do, you may put a creature card with mana value X
//	or less from among those cards onto the battlefield, where X is 1
//	plus the sacrificed creature's mana value. Put the rest on the
//	bottom of your library in a random order.
//
// AI policy: pick the highest-CMC creature in the top 7, then sacrifice
// the cheapest battlefield creature whose CMC+1 covers the target.
// If no creature in the top 7 outclasses anything we'd sacrifice, skip
// the sac (the may-sacrifice is optional). Always bottom the unpicked
// cards in original order — the engine doesn't yet have a deterministic
// random-bottom shim, and "in a random order" is sequencing flavor that
// the unpicked cards being bottomed satisfies regardless of order from
// the player's POV (they no longer know the order).
func registerBirthingRitual(r *Registry) {
	r.OnTrigger("Birthing Ritual", "end_step", birthingRitualEndStep)
}

func birthingRitualEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "birthing_ritual_end_step"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if !birthingRitualHasCreature(seat) {
		return
	}
	if len(seat.Library) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":  perm.Controller,
			"empty": true,
		})
		return
	}

	n := 7
	if n > len(seat.Library) {
		n = len(seat.Library)
	}
	top := append([]*gameengine.Card(nil), seat.Library[:n]...)

	bestIdx := -1
	bestCMC := -1
	for i, c := range top {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}

	sacVictim := birthingRitualPickSacrifice(seat, perm, bestCMC)
	var cheated *gameengine.Card
	cheatIdx := -1
	if sacVictim != nil && bestIdx >= 0 {
		cheated = top[bestIdx]
		cheatIdx = bestIdx
		gameengine.SacrificePermanent(gs, sacVictim, "birthing_ritual_cost")
	}

	// Wave 2 multi-step migration: drop the pre-splice + double-place.
	// Pre-r60 shape spliced top n off, then called MoveCard("library"→
	// "battlefield") AND enterBattlefieldWithETB on the same card — the
	// former placed a Permanent (via moveToZone's battlefield arm) and
	// the latter's createPermanent then deduped, but the redundancy
	// muddied which trigger chain owned the ETB fan-out. The clean
	// shape: leave the picked card in library and let
	// enterBattlefieldWithETB → createPermanent sweep the source via
	// RemoveCardFromAllPrivateZones (CR §400.7 owner-zone tracking).
	if cheated != nil {
		enterBattlefieldWithETB(gs, perm.Controller, cheated, false)
	}
	// Non-picked top cards stay in library; rotate them to the bottom.
	// After enterBattlefieldWithETB sweeps the picked card from library,
	// the remaining top-N are still on top in their original order.
	for i, c := range top {
		if i == cheatIdx || c == nil {
			continue
		}
		// Defensive: card should be at library[0]; if not, skip rotation.
		if len(seat.Library) == 0 || seat.Library[0] != c {
			continue
		}
		seat.Library = seat.Library[1:]
		seat.Library = append(seat.Library, c)
	}

	details := map[string]interface{}{
		"seat":      perm.Controller,
		"looked_at": n,
	}
	if sacVictim != nil {
		details["sacrificed"] = sacVictim.Card.DisplayName()
		details["sac_cmc"] = cardCMC(sacVictim.Card)
	}
	if cheated != nil {
		details["into_play"] = cheated.DisplayName()
		details["target_cmc"] = bestCMC
	}
	emit(gs, slug, perm.Card.DisplayName(), details)
}

func birthingRitualHasCreature(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			return true
		}
	}
	return false
}

// birthingRitualPickSacrifice returns the cheapest creature we control
// (other than Birthing Ritual itself, which isn't a creature anyway)
// whose CMC+1 ≥ targetCMC. Returns nil if no creature qualifies or if
// targetCMC < 0 (no valid cheat target in top 7).
//
// "Cheapest" because the upgrade-value of Birthing Ritual is
// (targetCMC - sacCMC). Among creatures that cover the target, the
// smallest sac yields the biggest tempo gain.
func birthingRitualPickSacrifice(seat *gameengine.Seat, self *gameengine.Permanent, targetCMC int) *gameengine.Permanent {
	if seat == nil || targetCMC < 0 {
		return nil
	}
	var pick *gameengine.Permanent
	pickCMC := 99
	for _, p := range seat.Battlefield {
		if p == nil || p == self || !p.IsCreature() || p.Card == nil {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc+1 < targetCMC {
			continue
		}
		if cmc < pickCMC {
			pickCMC = cmc
			pick = p
		}
	}
	return pick
}
