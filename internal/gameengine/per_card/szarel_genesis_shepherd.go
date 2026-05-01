package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSzarelGenesisShepherd wires Szarel, Genesis Shepherd.
//
// Oracle text:
//
//	Flying
//	You may play lands from your graveyard.
//	Whenever you sacrifice another nontoken permanent during your turn,
//	put a number of +1/+1 counters equal to Szarel's power on up to one
//	other target creature.
//
// Implementation:
//   - Flying — handled by the AST keyword pipeline.
//   - "You may play lands from your graveyard" — the static graveyard-play
//     permission isn't directly modeled in the cast-grant layer, so on ETB
//     we set a seat flag (`szarel_lands_from_gy`) mirroring Muldrotha's
//     approach. Downstream consumers (Freya / hat) read the flag to value
//     graveyard lands.
//   - OnTrigger("permanent_sacrificed") — gates on
//     (a) the sacrificing player is Szarel's controller,
//     (b) it is currently that player's turn (gs.Active),
//     (c) the sacrificed permanent is not Szarel herself,
//     (d) the sacrificed permanent is not a token,
//     then puts a number of +1/+1 counters equal to Szarel's current power
//     on the best other creature controlled by Szarel's controller.
func registerSzarelGenesisShepherd(r *Registry) {
	r.OnETB("Szarel, Genesis Shepherd", szarelETB)
	r.OnTrigger("Szarel, Genesis Shepherd", "permanent_sacrificed", szarelOnSacrifice)
}

func szarelETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["szarel_lands_from_gy"] = 1
	emit(gs, "szarel_etb_lands_from_gy", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "graveyard_land_play_permission_set",
	})
}

func szarelOnSacrifice(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "szarel_sacrifice_growth"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	if gs.Active != perm.Controller {
		return
	}

	sacCard, _ := ctx["card"].(*gameengine.Card)
	sacPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if sacPerm == perm {
		return
	}
	if sacCard != nil && perm.Card != nil && sacCard.DisplayName() == perm.Card.DisplayName() {
		return
	}
	if sacCard != nil && cardHasType(sacCard, "token") {
		return
	}

	target := chooseSzarelGrowthTarget(gs, perm)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_target_creature", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	power := perm.Power()
	if power <= 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "nonpositive_power", map[string]interface{}{
			"seat":  perm.Controller,
			"power": power,
		})
		return
	}
	target.AddCounter("+1/+1", power)
	gs.InvalidateCharacteristicsCache()

	sacName := ""
	if sacCard != nil {
		sacName = sacCard.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"sacrificed":     sacName,
		"target":         target.Card.DisplayName(),
		"counters_added": power,
	})
}

// chooseSzarelGrowthTarget returns the best creature controlled by Szarel's
// controller (excluding Szarel herself) to receive the +1/+1 buff. Picks
// the strongest non-token creature first (counters compound), falling back
// to any creature.
func chooseSzarelGrowthTarget(gs *gameengine.GameState, src *gameengine.Permanent) *gameengine.Permanent {
	if gs == nil || src == nil {
		return nil
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestScore := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		score := p.Power() * 2
		if !cardHasType(p.Card, "token") {
			score += 1
		}
		if score > bestScore {
			bestScore = score
			best = p
		}
	}
	return best
}
