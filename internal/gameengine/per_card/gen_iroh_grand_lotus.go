package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIrohGrandLotus wires Iroh, Grand Lotus.
//
// Oracle text:
//
//	Firebending 2
//	During your turn, each non-Lesson instant and sorcery card in your
//	graveyard has flashback. The flashback cost is equal to that card's
//	mana cost. (You may cast a card from your graveyard for its
//	flashback cost. Then exile it.)
//	During your turn, each Lesson card in your graveyard has flashback {1}.
//
// Implementation:
//
//   - Firebending 2 is a combat-damage trigger keyword handled (or
//     stubbed) by the AST pipeline; not touched here.
//
//   - The two flashback grants are registered as a single
//     GraveyardFlashbackGrant on ETB, with OnlyActiveTurn=true so the
//     "during your turn" gate is enforced at cast time. CostFor
//     returns -1 for non-instant/sorcery cards, 1 for Lesson subtypes,
//     and the card's mana cost otherwise.
//
//   - Cleanup is timestamp-driven: LTB removes any grant whose
//     SourceTimestamp matches the leaving permanent. The phases.go
//     EOT sweep also removes orphaned grants as a safety net.
func registerIrohGrandLotus(r *Registry) {
	r.OnETB("Iroh, Grand Lotus", irohGrandLotusETB)
	r.OnTrigger("Iroh, Grand Lotus", "permanent_ltb", irohGrandLotusLTB)
}

func irohGrandLotusETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	grant := &gameengine.GraveyardFlashbackGrant{
		Controller:      perm.Controller,
		SourceTimestamp: perm.Timestamp,
		SourceName:      perm.Card.DisplayName(),
		OnlyActiveTurn:  true,
		CostFor:         irohGrantCost,
	}
	gameengine.RegisterGraveyardFlashbackGrant(gs, grant)
	emit(gs, "iroh_grand_lotus_flashback_grant", perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"only_active_turn": true,
	})
}

func irohGrandLotusLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.ExpireGraveyardFlashbackGrantsBySource(gs, perm.Timestamp)
}

// irohGrantCost implements Iroh's two-tier flashback cost: non-Lesson
// instants/sorceries pay the card's mana cost; Lesson cards pay {1};
// anything else is ineligible. Lesson is a Strixhaven subtype.
func irohGrantCost(card *gameengine.Card) int {
	if card == nil {
		return -1
	}
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		return -1
	}
	if cardHasSubtype(card, "lesson") {
		return 1
	}
	return gameengine.ManaCostOf(card)
}
