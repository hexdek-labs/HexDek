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
//   - First-main-phase pay-and-transform (R60 stub sweep batch 6):
//     wired via upkeep_controller as proxy for first-main (mirrors
//     Ashling Rekindled). Front face pays {R}, back face pays {B}.
//     AI policy: always opt in when the mana is available — the
//     alternation is generally upside-only.
//   - Back-face blight-on-attack still unimplemented (blight is a
//     token-copy mechanic; separate primitive — flagged on the back-
//     face transform path only).
func registerGrubStoriedMatriarch(r *Registry) {
	r.OnETB("Grub, Storied Matriarch", grubStoriedMatriarchETB)
	r.OnETB("Grub, Storied Matriarch // Grub, Notorious Auntie", grubStoriedMatriarchETB)
	r.OnTrigger("Grub, Storied Matriarch", "upkeep_controller", grubFirstMainTransform)
	r.OnTrigger("Grub, Storied Matriarch // Grub, Notorious Auntie", "upkeep_controller", grubFirstMainTransform)
	r.OnTrigger("Grub, Notorious Auntie", "upkeep_controller", grubFirstMainTransform)
}

func grubFirstMainTransform(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "grub_first_main_pay_transform"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	color := "R"
	if perm.Transformed {
		color = "B"
	}
	if !grubTryPayColored(gs, seat, color) {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"paid":     false,
			"need":     color,
			"face":     grubFaceName(perm),
		})
		return
	}
	if !gameengine.TransformPermanent(gs, perm, "grub_first_main_pay_"+color) {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":         perm.Controller,
			"transformed":  false,
			"reason":       "transform_failed_no_back_face",
		})
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"paid":        color,
		"transformed": true,
		"new_face":    grubFaceName(perm),
	})
}

func grubFaceName(perm *gameengine.Permanent) string {
	if perm == nil || perm.Card == nil {
		return ""
	}
	return perm.Card.DisplayName()
}

func grubTryPayColored(gs *gameengine.GameState, seat *gameengine.Seat, color string) bool {
	if seat == nil {
		return false
	}
	if seat.Mana != nil {
		switch color {
		case "R":
			if seat.Mana.R > 0 {
				seat.Mana.R--
				gameengine.SyncManaAfterSpend(seat)
				return true
			}
		case "B":
			if seat.Mana.B > 0 {
				seat.Mana.B--
				gameengine.SyncManaAfterSpend(seat)
				return true
			}
		}
		if seat.Mana.Any > 0 {
			seat.Mana.Any--
			gameengine.SyncManaAfterSpend(seat)
			return true
		}
	}
	if seat.ManaPool >= 1 {
		seat.ManaPool--
		gameengine.SyncManaAfterSpend(seat)
		return true
	}
	return false
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
		emitPartial(gs, "grub_back_face_blight_attack_partial", perm.Card.DisplayName(),
			"back_face_blight_on_attack_unimplemented_token_copy_mechanic")
		return
	}
	card := seat.Graveyard[bestIdx]
	seat.Graveyard = append(seat.Graveyard[:bestIdx], seat.Graveyard[bestIdx+1:]...)
	seat.Hand = append(seat.Hand, card)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"recovered": true,
		"card":      card.DisplayName(),
	})
	emitPartial(gs, "grub_back_face_blight_attack_partial", perm.Card.DisplayName(),
		"back_face_blight_on_attack_unimplemented_token_copy_mechanic")
}
