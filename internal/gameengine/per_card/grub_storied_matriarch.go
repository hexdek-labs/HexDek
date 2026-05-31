package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGrubStoriedMatriarch wires Grub, Storied Matriarch // Grub,
// Notorious Auntie.
//
// Front face — Grub, Storied Matriarch:
//
//	Menace
//	Whenever this creature enters or transforms into Grub, Storied
//	Matriarch, return up to one target Goblin card from your
//	graveyard to your hand.
//	At the beginning of your first main phase, you may pay {R}. If
//	you do, transform Grub.
//
// Back face — Grub, Notorious Auntie:
//
//	Menace
//	Whenever Grub attacks, you may blight 1 (a copy-token mechanic).
//	At the beginning of your first main phase, you may pay {B}. If
//	you do, transform Grub.
//
// Implementation:
//   - ETB: return the highest-power Goblin from controller's graveyard
//     to hand.
//   - Transform mechanics and the blight attack trigger are unsupported
//     by the engine — emitPartial.
func registerGrubStoriedMatriarch(r *Registry) {
	r.OnETB("Grub, Storied Matriarch", grubStoriedMatriarchETB)
	r.OnETB("Grub, Storied Matriarch // Grub, Notorious Auntie", grubStoriedMatriarchETB)
}

func grubStoriedMatriarchETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "grub_storied_matriarch_etb"
	if gs == nil || perm == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	bestIdx := -1
	bestPower := -1
	for i, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "goblin") {
			continue
		}
		p := int(c.BasePower)
		if p > bestPower {
			bestPower = p
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      seatIdx,
			"recovered": false,
		})
		emitPartial(gs, "grub_transform_partial", perm.Card.DisplayName(),
			"first_main_transform_and_blight_attack_unimplemented")
		return
	}
	card := seat.Graveyard[bestIdx]
	gameengine.MoveCard(gs, card, seatIdx, "graveyard", "hand", "grub_recover")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"recovered": true,
		"card":      card.DisplayName(),
	})
	emitPartial(gs, "grub_transform_partial", perm.Card.DisplayName(),
		"first_main_transform_and_blight_attack_unimplemented")
}
