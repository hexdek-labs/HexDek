package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTivit wires Tivit, Seller of Secrets.
//
// Oracle text:
//
//	Flying, ward {3}.
//	Whenever Tivit, Seller of Secrets enters the battlefield or deals
//	combat damage to a player, you secretly choose two of the following,
//	then starting with you, each player votes for one of those choices.
//	For each vote, create a Clue token, a Treasure token, or a Time
//	Sieve counter. You get a vote for each vote.
//
// Simulation: skip the voting flavor. The optimal pick in 4-player
// Commander is almost always Clue + Treasure (artifact density + mana
// fixing + card draw). We approximate with 2 Clue tokens and 2 Treasure
// tokens for the controller every time the trigger fires.
func registerTivit(r *Registry) {
	r.OnETB("Tivit, Seller of Secrets", tivitPayout)
	r.OnTrigger("Tivit, Seller of Secrets", "combat_damage_player", tivitCombatTrigger)
}

func tivitPayout(gs *gameengine.GameState, perm *gameengine.Permanent) {
	tivitCreateTokens(gs, perm, "etb")
}

func tivitCombatTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceName, _ := ctx["source_card"].(string)
	if sourceName != "" && !strings.EqualFold(sourceName, perm.Card.DisplayName()) {
		return
	}
	tivitCreateTokens(gs, perm, "combat_damage")
}

func tivitCreateTokens(gs *gameengine.GameState, perm *gameengine.Permanent, reason string) {
	const slug = "tivit_seller_of_secrets_payout"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.CreateClueToken(gs, seat)
	gameengine.CreateClueToken(gs, seat)
	gameengine.CreateTreasureToken(gs, seat)
	gameengine.CreateTreasureToken(gs, seat)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"reason":   reason,
		"clues":    2,
		"treasures": 2,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"voting_mechanic_skipped_assumes_clue_treasure_pick")
}
