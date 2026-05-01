package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSyrGwynHeroOfAshvale wires Syr Gwyn, Hero of Ashvale.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Vigilance, menace, lifelink
//	As long as you control a Knight, Equipment you control have equip {0}.
//	Whenever equipped creature you control attacks, draw a card and each
//	opponent loses 1 life.
//
// Implementation:
//   - Vigilance / menace / lifelink: AST keyword pipeline.
//   - "Equip {0} while you control a Knight": modeled as a flag on the
//     controlling seat (syr_gwyn_equip_zero) that the cost-modifier path
//     consults. Set on ETB if a Knight is present and refreshed on each
//     trigger evaluation; Knight presence is also rechecked at ETB so a
//     Syr Gwyn that enters first wakes up the moment a Knight follows
//     (re-checked via a permanent_etb listener as well).
//   - "creature_attacks" trigger: when an attacking creature this
//     controller controls has any Equipment attached to it, draw a card
//     and each opponent loses 1 life. Multiple equipped attackers each
//     fire the trigger separately (one draw + one drain per attacker).
func registerSyrGwynHeroOfAshvale(r *Registry) {
	r.OnETB("Syr Gwyn, Hero of Ashvale", syrGwynETB)
	r.OnTrigger("Syr Gwyn, Hero of Ashvale", "permanent_etb", syrGwynPermETB)
	r.OnTrigger("Syr Gwyn, Hero of Ashvale", "creature_attacks", syrGwynAttackTrigger)
}

func syrGwynETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	syrGwynRefreshEquipZero(gs, perm.Controller, perm.Card.DisplayName())
}

// syrGwynPermETB re-evaluates the Knight static when any permanent enters
// under Syr Gwyn's controller (a freshly-played Knight should immediately
// flip equip {0} on).
func syrGwynPermETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	syrGwynRefreshEquipZero(gs, perm.Controller, perm.Card.DisplayName())
}

func syrGwynRefreshEquipZero(gs *gameengine.GameState, seatIdx int, source string) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	hasKnight := false
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "knight") {
				hasKnight = true
				break
			}
		}
		if hasKnight {
			break
		}
	}
	want := 0
	if hasKnight {
		want = 1
	}
	if seat.Flags["syr_gwyn_equip_zero"] == want {
		return
	}
	seat.Flags["syr_gwyn_equip_zero"] = want
	emit(gs, "syr_gwyn_equip_zero_static", source, map[string]interface{}{
		"seat":   seatIdx,
		"active": want,
	})
}

func syrGwynAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "syr_gwyn_attack_draw_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm == nil || atkPerm.Controller != perm.Controller {
		return
	}
	if !syrGwynHasAttachedEquipment(gs, atkPerm) {
		return
	}

	seat := perm.Controller
	drawOne(gs, seat, perm.Card.DisplayName())

	drained := 0
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		s.Life -= 1
		gs.LogEvent(gameengine.Event{
			Kind:   "life_loss",
			Seat:   seat,
			Target: opp,
			Source: perm.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"reason": "syr_gwyn_attack_drain",
			},
		})
		drained++
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"attacker": atkPerm.Card.DisplayName(),
		"drained":  drained,
	})
}

// syrGwynHasAttachedEquipment reports whether any Equipment on the
// battlefield is attached to host. Equipment lives on its own permanent
// row with AttachedTo pointing at the equipped creature.
func syrGwynHasAttachedEquipment(gs *gameengine.GameState, host *gameengine.Permanent) bool {
	if gs == nil || host == nil {
		return false
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || p.AttachedTo != host {
				continue
			}
			for _, t := range p.Card.Types {
				if strings.EqualFold(t, "equipment") {
					return true
				}
			}
		}
	}
	return false
}
