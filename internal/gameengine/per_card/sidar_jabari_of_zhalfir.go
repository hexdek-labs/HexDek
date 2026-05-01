package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSidarJabariOfZhalfir wires Sidar Jabari of Zhalfir. Batch #33.
//
// Oracle text (Scryfall, verified 2026-05-01; March of the Machine Commander):
//
//	{1}{W}{U}{B} Legendary Creature — Human Knight 4/3
//	Eminence — Whenever you attack with one or more Knights, if Sidar
//	Jabari is in the command zone or on the battlefield, draw a card,
//	then discard a card.
//	Flying, first strike
//	Whenever Sidar Jabari deals combat damage to a player, return target
//	Knight creature card from your graveyard to the battlefield.
//
// Implementation:
//   - Flying / first strike — AST keyword pipeline.
//   - Eminence loot: trigger on "creature_attacks". The engine fires
//     creature_attacks once per declared attacker, so we dedupe per turn
//     using the Raffine pattern (perm-scoped flag stamped with gs.Turn).
//     The trigger only fires if at least one of controller's attacking
//     creatures is a Knight (Sidar herself counts).
//   - Architectural caveat: command-zone Eminence is not modeled — the
//     dispatch only walks battlefield (mirrors Edgar Markov / Inalla).
//     Logged as emitPartial so analysis tooling sees the gap.
//   - Combat-damage Knight reanimate: gates on source == Sidar and
//     source_seat == controller. Picks the highest-CMC Knight creature
//     card from controller's graveyard and returns it via the standard
//     graveyard→battlefield + enterBattlefieldWithETB cascade.
func registerSidarJabariOfZhalfir(r *Registry) {
	r.OnTrigger("Sidar Jabari of Zhalfir", "creature_attacks", sidarJabariEminenceLoot)
	r.OnTrigger("Sidar Jabari of Zhalfir", "combat_damage_player", sidarJabariCombatDamage)
}

func sidarJabariEminenceLoot(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sidar_jabari_eminence_attack_loot"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm == nil || atkPerm.Controller != perm.Controller {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	const flagKey = "sidar_jabari_eminence_fired"
	if perm.Flags[flagKey] >= gs.Turn {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	// Verify at least one attacking Knight controlled by perm.Controller.
	knightAttacker := ""
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsAttacking() {
			continue
		}
		if cardHasType(p.Card, "knight") {
			knightAttacker = p.Card.DisplayName()
			break
		}
	}
	if knightAttacker == "" && atkPerm.Card != nil {
		// Single-attacker fallback — flagAttacking may not be visible yet.
		for _, t := range atkPerm.Card.Types {
			if strings.EqualFold(t, "knight") {
				knightAttacker = atkPerm.Card.DisplayName()
				break
			}
		}
	}
	if knightAttacker == "" {
		return
	}

	perm.Flags[flagKey] = gs.Turn

	drew := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	discarded := ""
	if len(seat.Hand) > 0 {
		card := seat.Hand[len(seat.Hand)-1]
		if card != nil {
			discarded = card.DisplayName()
			gameengine.DiscardCard(gs, card, perm.Controller)
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"knight_attacker": knightAttacker,
		"drew":            drew != nil,
		"discarded":       discarded,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"command_zone_eminence_dispatch_walks_battlefield_only")
}

func sidarJabariCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sidar_jabari_combat_damage_knight_reanimate"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if perm.Card == nil || sourceName != perm.Card.DisplayName() {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	var best *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		if !cardHasType(c, "knight") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	if best == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"reason": "no_knight_in_graveyard",
		})
		return
	}

	gameengine.MoveCard(gs, best, perm.Controller, "graveyard", "battlefield", "sidar_jabari_reanimate")
	enterBattlefieldWithETB(gs, perm.Controller, best, false)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"reanimated": best.DisplayName(),
		"cmc":        bestCMC,
	})
}
