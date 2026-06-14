package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTemmetNaktamunsWill wires Temmet, Naktamun's Will.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Vigilance, menace
//	Whenever you attack, draw a card, then discard a card.
//	Whenever you draw a card, Zombies you control get +1/+1 until end
//	  of turn.
//
// Implementation:
//   - "attack_declared": draw + discard (loot via highest-CMC heuristic).
//   - "player_would_draw": gate on draw_seat == perm.Controller. Tag
//     Zombie creatures we control with a UEOT +1/+1 buff. UEOT cleanup
//     lives in the layers pipeline; we approximate by stamping +1/+1
//     counters that the engine's end-of-turn cleanup may or may not
//     remove. emitPartial flags the gap.
//   - Vigilance/menace handled by AST keyword pipeline.
func registerTemmetNaktamunsWill(r *Registry) {
	r.OnTrigger("Temmet, Naktamun's Will", "attack_declared", temmetOnAttack)
	r.OnTrigger("Temmet, Naktamun's Will", "player_would_draw", temmetOnDraw)
}

func temmetOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "temmet_attack_loot"
	if gs == nil || perm == nil {
		return
	}
	// "Whenever you attack" fires once per declare-attackers step, but the
	// engine fires "attack_declared" once per declared attacker. Gate to the
	// first hit per turn so the loot resolves once per combat, not once per
	// attacker (Caesar/Raffine once-per-turn dedup pattern).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["temmet_attack_fired"] >= gs.Turn {
		return
	}
	perm.Flags["temmet_attack_fired"] = gs.Turn
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	discardName := ""
	if len(seat.Hand) > 0 {
		idx := 0
		bestCMC := -1
		for i, c := range seat.Hand {
			if c == nil {
				continue
			}
			if cm := gameengine.ManaCostOf(c); cm > bestCMC {
				bestCMC = cm
				idx = i
			}
		}
		card := seat.Hand[idx]
		discardName = card.DisplayName()
		gameengine.MoveCard(gs, card, perm.Controller, "hand", "graveyard", "temmet_loot_discard")
	}
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"drawn":   drawnName,
		"discard": discardName,
	})
}

func temmetOnDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "temmet_zombie_pump"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	drawSeat, _ := ctx["draw_seat"].(int)
	if drawSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	pumped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if cardHasType(p.Card, "zombie") {
			p.AddCounter("+1/+1", 1)
			pumped++
		}
	}
	if pumped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"pumped": pumped,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"ueot_pump_via_persistent_counter_only_at_end_of_turn_cleanup_not_modeled")
}
