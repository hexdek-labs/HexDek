package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSinSpirasPunishment wires Sin, Spira's Punishment.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{4}{B}{G}{U}
//	Legendary Creature — Leviathan Avatar
//	7/7
//	Flying
//	Whenever Sin enters or attacks, exile a permanent card from your
//	graveyard at random, then create a tapped token that's a copy of
//	that card. If the exiled card is a land card, repeat this process.
//
// Implementation:
//   - ETB and creature_attacks (gated to Sin itself): scan controller's
//     graveyard for permanent cards. Pick deterministically (highest CMC
//     non-land first, falling back to any permanent) — full random isn't
//     worth the simulation noise. Exile the card, mint a tapped copy.
//   - Land-repeat loop: bounded to 8 iterations to avoid degenerate
//     yard-stuffed loops.
func registerSinSpirasPunishment(r *Registry) {
	r.OnETB("Sin, Spira's Punishment", sinSpirasPunishmentETB)
	r.OnTrigger("Sin, Spira's Punishment", "creature_attacks", sinSpirasPunishmentAttacks)
}

func sinSpirasPunishmentETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	sinSpirasPunishmentApply(gs, perm, "sin_spiras_punishment_etb")
}

func sinSpirasPunishmentAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk != perm {
		return
	}
	sinSpirasPunishmentApply(gs, perm, "sin_spiras_punishment_attack")
}

func sinSpirasPunishmentApply(gs *gameengine.GameState, perm *gameengine.Permanent, slug string) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	created := []string{}
	for iter := 0; iter < 8; iter++ {
		// Pick best permanent card in graveyard. Prefer non-land high-CMC
		// (a copied Avenger of Zendikar > a copied Forest); only fall back
		// to land if no non-land permanent is present.
		bestIdx := -1
		bestScore := -1
		landFallback := -1
		for i, c := range seat.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasTypeAny(c, "creature", "artifact", "enchantment", "planeswalker", "land", "battle") {
				continue
			}
			if cardHasType(c, "land") {
				if landFallback < 0 {
					landFallback = i
				}
				continue
			}
			score := gameengine.ManaCostOf(c)
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			bestIdx = landFallback
		}
		if bestIdx < 0 {
			break
		}
		card := seat.Graveyard[bestIdx]
		isLand := cardHasType(card, "land")
		// MoveCard handles the graveyard removal. Prior manual splice
		// before this call was redundant (Wave 2 final-audit cleanup).
		gameengine.MoveCard(gs, card, perm.Controller, "graveyard", "exile", "sin_spiras_punishment")
		token := &gameengine.Card{
			Name:          card.DisplayName() + " Token",
			Owner:         perm.Controller,
			BasePower:     card.BasePower,
			BaseToughness: card.BaseToughness,
			Types:         append([]string{"token"}, card.Types...),
			Colors:        append([]string{}, card.Colors...),
			TypeLine:      "Token " + card.TypeLine,
		}
		enterBattlefieldWithETB(gs, perm.Controller, token, true)
		created = append(created, card.DisplayName())
		if !isLand {
			break
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"copied":   created,
		"count":    len(created),
	})
}
