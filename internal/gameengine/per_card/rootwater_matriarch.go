package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRootwaterMatriarch wires Rootwater Matriarch (Muninn parser-gap
// rank ~153, old-frame Aura-control combo piece).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{2}{U}{U}
//	Creature — Merfolk
//	{T}: Gain control of target creature for as long as that creature is
//	enchanted.
//
// Implementation:
//   - OnActivated: tap Rootwater Matriarch, pick best enchanted opponent
//     creature (highest power, then CMC) — the "is enchanted" precondition
//     is checked by counting attached Auras on the target. Reassign
//     Controller to perm.Controller. The conditional "for as long as
//     that creature is enchanted" duration is not yet tracked by the
//     engine (see Roil Elemental partial); the steal persists past
//     enchantment removal. emitPartial documents that.
//   - If no enchanted opponent creature exists, the activation still
//     pays the tap cost (rules-legal) but no steal happens — emitFail
//     with reason.
func registerRootwaterMatriarch(r *Registry) {
	r.OnActivated("Rootwater Matriarch", rootwaterMatriarchActivate)
}

func rootwaterMatriarchActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "rootwater_matriarch_steal_enchanted"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	src.Tapped = true

	var target *gameengine.Permanent
	bestScore := -1
	for _, opp := range gs.Opponents(src.Controller) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if !rootwaterMatriarchIsEnchanted(s.Battlefield, p) {
				continue
			}
			score := p.Power()*10 + cardCMC(p.Card)
			if score > bestScore {
				bestScore = score
				target = p
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_enchanted_opponent_creature", map[string]interface{}{
			"seat": src.Controller,
		})
		return
	}
	oldController := target.Controller
	oldSeat := gs.Seats[oldController]
	newSeat := gs.Seats[src.Controller]
	if oldSeat == nil || newSeat == nil {
		return
	}
	// r102 — canonical reparent (CR §302.6 stamp, §702.95e unpair, §613
	// invalidation). Rootwater Matriarch's steal is permanent and prints
	// no haste, so the creature is summoning-sick for its new controller.
	gameengine.ApplyControlChange(gs, target, src.Controller)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"stolen":    target.Card.DisplayName(),
		"from_seat": oldController,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"control_duration_tied_to_enchanted_status_not_yet_modelled_steal_is_permanent")
}

func rootwaterMatriarchIsEnchanted(battlefield []*gameengine.Permanent, target *gameengine.Permanent) bool {
	for _, p := range battlefield {
		if p == nil || p == target || p.AttachedTo != target {
			continue
		}
		if p.IsEnchantment() {
			return true
		}
	}
	return false
}
