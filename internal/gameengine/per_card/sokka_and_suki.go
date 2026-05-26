package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSokkaAndSuki wires Sokka and Suki.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Whenever Sokka and Suki or another Ally you control enters,
//	attach up to one target Equipment you control to that creature.
//	Whenever an Equipment you control enters, create a 1/1 white
//	Ally creature token.
//
// Implementation: both handlers register on "permanent_etb" (the canonical
// engine ETB event); each filters internally by type so only the matching
// arm acts on a given ETB.
//   - sokkaSukiAllyEnters: when a creature controlled by Sokka's controller
//     enters and it's an Ally (or Sokka itself), pick the highest-MV
//     unattached friendly Equipment and attach it.
//   - sokkaSukiEquipmentEnters: when an Equipment controlled by Sokka's
//     controller enters, mint a 1/1 Ally token.
func registerSokkaAndSuki(r *Registry) {
	r.OnTrigger("Sokka and Suki", "permanent_etb", sokkaSukiAllyEnters)
	r.OnTrigger("Sokka and Suki", "permanent_etb", sokkaSukiEquipmentEnters)
}

func sokkaSukiAllyEnters(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sokka_suki_ally_enters_attach"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil || enteringPerm.Card == nil {
		return
	}
	if enteringPerm.Controller != perm.Controller {
		return
	}
	if enteringPerm != perm && !cardHasType(enteringPerm.Card, "ally") {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var bestEq *gameengine.Permanent
	bestCMC := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		isEq := false
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "equipment") {
				isEq = true
				break
			}
		}
		if !isEq {
			continue
		}
		if p.AttachedTo != nil {
			continue
		}
		cm := cardCMC(p.Card)
		if cm > bestCMC {
			bestCMC = cm
			bestEq = p
		}
	}
	if bestEq == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_unattached_equipment", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	bestEq.AttachedTo = enteringPerm
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"equipment": bestEq.Card.DisplayName(),
		"target":    enteringPerm.Card.DisplayName(),
	})
}

func sokkaSukiEquipmentEnters(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sokka_suki_equipment_enters_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil || enteringPerm.Card == nil {
		return
	}
	if enteringPerm.Controller != perm.Controller {
		return
	}
	isEq := false
	for _, t := range enteringPerm.Card.Types {
		if strings.EqualFold(t, "equipment") {
			isEq = true
			break
		}
	}
	if !isEq {
		return
	}
	gameengine.CreateCreatureToken(gs, perm.Controller, "Ally",
		[]string{"creature", "ally", "pip:W"}, 1, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"equipment": enteringPerm.Card.DisplayName(),
	})
}
