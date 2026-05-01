package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHakbal wires Hakbal of the Surging Soul.
//
// Oracle text:
//
//	At the beginning of combat on your turn, each Merfolk creature you
//	control explores.
//	Whenever Hakbal attacks, you may put a land card from your hand
//	onto the battlefield. If you don't, draw a card.
//
// Implementation:
//
//   - "combat_begin": fires once per beginning-of-combat step. We gate on
//     Hakbal's controller being the active player and walk the
//     controller's battlefield, calling PerformExplore for each Merfolk
//     creature.
//   - "creature_attacks": filtered to Hakbal himself. Greedy choice: if
//     the controller has a land in hand, put it onto the battlefield;
//     otherwise draw. Engine-side mana is more valuable than a card on
//     average, so the "may" here defaults to YES.
func registerHakbal(r *Registry) {
	r.OnTrigger("Hakbal of the Surging Soul", "combat_begin", hakbalCombatBegin)
	r.OnTrigger("Hakbal of the Surging Soul", "creature_attacks", hakbalAttacks)
}

func hakbalCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hakbal_combat_begin_merfolk_explore"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// De-dupe: combat_begin fires once but engine may dispatch twice if
	// extra-combat phases ever land. Coalesce on (turn).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := "hakbal_combat_explore_" + strconv.Itoa(gs.Turn)
	if perm.Flags[key] > 0 {
		return
	}
	perm.Flags[key] = 1

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	merfolk := append([]*gameengine.Permanent(nil), seat.Battlefield...)
	count := 0
	for _, p := range merfolk {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "creature") {
			continue
		}
		if !cardHasType(p.Card, "merfolk") {
			continue
		}
		gameengine.PerformExplore(gs, p)
		count++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"explored": count,
	})
}

func hakbalAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hakbal_attack_land_drop_or_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Greedy: prefer dropping a land if hand has one (engine treats
	// permanent acceleration as worth more than one card on average).
	var landIdx = -1
	for i, c := range seat.Hand {
		if c != nil && cardHasType(c, "land") {
			landIdx = i
			break
		}
	}

	if landIdx >= 0 {
		landCard := seat.Hand[landIdx]
		seat.Hand = append(seat.Hand[:landIdx], seat.Hand[landIdx+1:]...)
		enterBattlefieldWithETB(gs, perm.Controller, landCard, false)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"choice":    "land_drop",
			"land_card": landCard.DisplayName(),
		})
		return
	}

	// No land in hand → draw a card.
	if len(seat.Library) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"choice": "draw",
			"drawn":  0,
			"reason": "library_empty",
		})
		return
	}
	top := seat.Library[0]
	gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "hakbal_attack_draw")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"choice": "draw",
		"drawn":  1,
	})
}
