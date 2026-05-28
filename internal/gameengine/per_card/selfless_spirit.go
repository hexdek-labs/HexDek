package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSelflessSpirit wires Selfless Spirit.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Selfless%20Spirit):
//
//	Flying
//	Sacrifice this creature: Creatures you control gain indestructible
//	until end of turn.
//
// {1}{W} Creature — Spirit 2/1. The "flash-equivalent" sac protection
// — drop on curve, sac in response to a Wrath / Toxic Deluge to save
// the rest of the board. Flying makes the body relevant when the
// sweeper isn't pending (chip damage / evasion). Pairs with token-
// engine commanders (Hapatra-tokens, Edgar Markov, Sigarda Host of
// Herons protection chain).
//
// Implementation:
//   - OnActivated index 0: cost is sacrifice self (no mana cost).
//     Sacrifice Selfless Spirit via SacrificePermanent so §704.5g
//     LTB observers and graveyard-trigger observers (Karmic Guide
//     re-fetch chains) fire correctly.
//   - For every creature controller controls, append "indestructible"
//     to GrantedAbilities. Granted abilities clear at cleanup per
//     §514.2; no per_card EOT bookkeeping needed.
//   - Creatures only — non-creature permanents do NOT get the grant
//     (printed "creatures you control" restriction).
//   - Selfless Spirit itself is in the graveyard by the time the
//     grant lands (sacrifice precedes the effect resolution), so it
//     doesn't grant indestructible to itself — matches printed text.
func registerSelflessSpirit(r *Registry) {
	r.OnActivated("Selfless Spirit", selflessSpiritActivate)
}

func selflessSpiritActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "selfless_spirit_sac"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Sacrifice first (the printed cost). After SacrificePermanent the
	// source is in the graveyard and won't be in the battlefield scan
	// below — matches the printed "creatures you control" exclusion
	// of Selfless Spirit itself.
	gameengine.SacrificePermanent(gs, src, "selfless_spirit_sac_cost")

	granted := 0
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.GrantedAbilities = append(p.GrantedAbilities, "indestructible")
		granted++
	}
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, "Selfless Spirit", map[string]interface{}{
		"seat":             seat,
		"creatures_saved":  granted,
		"duration":         "until_end_of_turn",
	})
}
