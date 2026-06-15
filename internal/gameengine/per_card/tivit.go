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
// Implemented via the canonical CR §701.20 Council's Dilemma tally: the
// controller secretly picks Clue + Treasure (the strongest pair; all created
// tokens go to the controller regardless of who votes), then every player
// votes in controller-first turn order, and "you get a vote for each vote"
// adds one controller vote per vote cast. Each vote makes the chosen token, so
// the payout scales with the table size (≈ 2× the living-player count) instead
// of the old fixed 4 — a 4-player game yields ~8 tokens, not 4.
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

	// CR §701.20 Council's Dilemma — controller secretly chose Clue + Treasure.
	// Each player votes (controller-first turn order); the vote choice is
	// immaterial here since every created token goes to the controller, so
	// each voter picks Clue.
	options := []string{"Clue", "Treasure"}
	tally := gameengine.TallyCouncilsDilemma(gs, seat, options,
		func(_ int, _ []string) int { return 0 })
	if tally == nil {
		tally = map[string]int{"Clue": 1, "Treasure": 0}
	}
	// "You get a vote for each vote" — the controller gets one additional vote
	// per vote cast (Gatherer ruling: N extra votes in an N-player game),
	// assigned to Treasure here. Net payout ≈ 2× the living-player count.
	baseVotes := tally["Clue"] + tally["Treasure"]
	tally["Treasure"] += baseVotes

	clues, treasures := 0, 0
	gameengine.ApplyCouncilsDilemma(gs, options, tally, func(opt string, votes int) {
		for i := 0; i < votes; i++ {
			if opt == "Treasure" {
				gameengine.CreateTreasureToken(gs, seat)
				treasures++
			} else {
				gameengine.CreateClueToken(gs, seat)
				clues++
			}
		}
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"reason":    reason,
		"clues":     clues,
		"treasures": treasures,
	})
}
