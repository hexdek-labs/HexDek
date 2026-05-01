package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKykarWindsFury wires Kykar, Wind's Fury.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Flying
//	Whenever you cast a noncreature spell, create a 1/1 white Spirit
//	creature token with flying.
//	Sacrifice a Spirit: Add {R}.
//
// Implementation:
//   - "noncreature_spell_cast": gate on caster_seat == controller; mint a
//     1/1 flying white Spirit token via the standard CreateCreatureToken
//     pipeline (token_created chain naturally fires for Anointed
//     Procession et al.).
//   - OnActivated(0): pick a Spirit on the controller's battlefield (prefer
//     ctx["creature_perm"] if a hat-supplied victim is provided), sacrifice
//     it via SacrificePermanent, and add {R} to the controller's pool.
//     This is a mana ability per CR §605.1a (no targets, produces mana).
func registerKykarWindsFury(r *Registry) {
	r.OnTrigger("Kykar, Wind's Fury", "noncreature_spell_cast", kykarNoncreatureCast)
	r.OnActivated("Kykar, Wind's Fury", kykarSacSpirit)
}

func kykarNoncreatureCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kykar_winds_fury_spirit_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}

	token := gameengine.CreateCreatureToken(gs, perm.Controller, "Spirit",
		[]string{"creature", "spirit", "pip:W"}, 1, 1)
	if token != nil {
		if token.Flags == nil {
			token.Flags = map[string]int{}
		}
		token.Flags["kw:flying"] = 1
	}

	spellName := ""
	if c, ok := ctx["card"].(*gameengine.Card); ok && c != nil {
		spellName = c.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"spell_name": spellName,
		"token":      "Spirit",
		"flying":     true,
	})
}

func kykarSacSpirit(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "kykar_winds_fury_sac_spirit_for_red"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := kykarPickSpiritVictim(gs, seat, ctx)
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_spirit_to_sacrifice", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "kykar_sac_spirit")
	gameengine.AddMana(gs, gs.Seats[seat], "R", 1, src.Card.DisplayName())

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
		"mana_added": "R",
	})
}

// kykarPickSpiritVictim selects a Spirit to sacrifice. Prefers a
// hat-provided ctx["creature_perm"] if it's a Spirit; otherwise picks the
// first Spirit on the controller's battlefield (token Spirits sort first
// since they're the freshest fodder).
func kykarPickSpiritVictim(gs *gameengine.GameState, seat int, ctx map[string]interface{}) *gameengine.Permanent {
	if ctx != nil {
		if p, ok := ctx["creature_perm"].(*gameengine.Permanent); ok && p != nil {
			if p.IsCreature() && cardHasType(p.Card, "spirit") {
				return p
			}
		}
	}
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}
	var nonTokenSpirit *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "spirit") {
			continue
		}
		if cardHasType(p.Card, "token") {
			return p
		}
		if nonTokenSpirit == nil {
			nonTokenSpirit = p
		}
	}
	return nonTokenSpirit
}
