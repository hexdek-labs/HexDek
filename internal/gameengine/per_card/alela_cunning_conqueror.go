package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlelaCunningConqueror wires Alela, Cunning Conqueror.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Flying. Whenever you cast your first spell during each opponent's
//	turn, create a 1/1 black Faerie Rogue creature token with flying.
//	Whenever one or more Faeries you control deal combat damage to a
//	player, you may tap or untap target nonland permanent.
//
// Implementation:
//   - OnTrigger("spell_cast"): fires on every spell cast. Gate on:
//     (a) caster_seat == perm.Controller (Alela's controller is casting),
//     (b) gs.Active != perm.Controller (it's an opponent's turn), and
//     (c) perm.Flags["alela_cast_this_turn"] != gs.Turn (first spell only).
//     When the gate passes, stamp the flag and mint a 1/1 black Faerie
//     Rogue with flying via CreateCreatureToken + Flags["kw:flying"].
//
//   - OnTrigger("combat_damage_player"): fires when any creature deals
//     combat damage to a player. Gate on: the source creature is controlled
//     by Alela's controller AND the source is a Faerie. AI policy: tap the
//     highest-power untapped nonland permanent that an opponent controls
//     (best threat removal). The untap option is logged as an emitPartial
//     (player-choice interactive; AI always chooses tap for tempo).
//
// Token spec:
//
//	Name="Faerie Rogue", Power=1, Toughness=1, Colors=["B"],
//	Types=["token","creature","faerie","rogue"], Keywords=["flying"].
func registerAlelaCunningConqueror(r *Registry) {
	r.OnTrigger("Alela, Cunning Conqueror", "spell_cast", alelaCunningConquerorSpellCast)
	r.OnTrigger("Alela, Cunning Conqueror", "combat_damage_player", alelaCunningConquerorFaerieDamage)
}

// alelaCunningConquerorSpellCast creates a 1/1 black Faerie Rogue with flying
// the first time Alela's controller casts a spell on each opponent's turn.
func alelaCunningConquerorSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "alela_cunning_conqueror_faerie_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}

	// Only triggers during an opponent's turn.
	if gs.Active == perm.Controller {
		return
	}

	// Only fires once per opponent's turn (first spell gate).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["alela_cast_this_turn"] == gs.Turn {
		return
	}
	perm.Flags["alela_cast_this_turn"] = gs.Turn

	// Mint the 1/1 black Faerie Rogue token with flying.
	tok := gameengine.CreateCreatureToken(
		gs,
		perm.Controller,
		"Faerie Rogue",
		[]string{"creature", "faerie", "rogue"},
		1, 1,
	)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["kw:flying"] = 1
		if tok.Card != nil {
			tok.Card.Colors = []string{"B"}
		}
	}

	spellName := ""
	if c, ok := ctx["card"].(*gameengine.Card); ok && c != nil {
		spellName = c.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"active":     gs.Active,
		"spell_name": spellName,
		"token":      "Faerie Rogue",
		"flying":     true,
	})
}

// alelaCunningConquerorFaerieDamage triggers when a Faerie Alela's controller
// controls deals combat damage to a player. AI taps the best opponent
// nonland permanent; the untap option is marked partial.
func alelaCunningConquerorFaerieDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "alela_cunning_conqueror_tap_untap"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	defenderSeat, _ := ctx["defender_seat"].(int)

	// Source must be controlled by Alela's controller.
	if sourceSeat != perm.Controller {
		return
	}

	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}

	// Verify the source is a Faerie.
	if !alelaDealerIsFaerie(gs, sourceSeat, sourceName) {
		return
	}

	// Log that the untap option is not modeled (interactive player choice).
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"untap_option_not_modeled_ai_always_taps")

	// Tap the highest-power untapped nonland permanent an opponent controls.
	target := alelaBestTapTarget(gs, perm.Controller)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_tap_target", map[string]interface{}{
			"source_seat": sourceSeat,
			"source_card": sourceName,
		})
		return
	}

	target.Tapped = true

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"source_card": sourceName,
		"tapped":      target.Card.DisplayName(),
		"tap_target":  target.Card.DisplayName(),
	})
}

// alelaDealerIsFaerie checks whether the named permanent on sourceSeat is a
// Faerie. Checks Card.Types and Card.TypeLine.
func alelaDealerIsFaerie(gs *gameengine.GameState, sourceSeat int, sourceName string) bool {
	if gs == nil || sourceSeat < 0 || sourceSeat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[sourceSeat]
	if s == nil {
		return false
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() != sourceName {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "faerie") {
				return true
			}
		}
		if strings.Contains(strings.ToLower(p.Card.TypeLine), "faerie") {
			return true
		}
		return false
	}
	return false
}

// alelaBestTapTarget returns the highest-power untapped nonland permanent
// controlled by any opponent of controllerSeat. Returns nil if none found.
func alelaBestTapTarget(gs *gameengine.GameState, controllerSeat int) *gameengine.Permanent {
	if gs == nil {
		return nil
	}
	var best *gameengine.Permanent
	for _, oppSeat := range gs.Opponents(controllerSeat) {
		s := gs.Seats[oppSeat]
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || p.Tapped {
				continue
			}
			if p.IsLand() {
				continue
			}
			if best == nil || p.Power() > best.Power() {
				best = p
			}
		}
	}
	return best
}
