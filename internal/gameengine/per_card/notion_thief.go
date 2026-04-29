package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNotionThief wires Notion Thief.
//
// Oracle text:
//
//	Flash
//	If an opponent would draw a card except the first one they draw in
//	each of their draw steps, instead that player skips that draw and
//	you draw a card instead.
//
// The replacement effect logic lives in gameengine/replacement.go
// (RegisterNotionThiefReplacement). The ETB handler here triggers
// registration, and RegisterReplacementsForPermanent also calls it
// for the standard ETB path.
func registerNotionThief(r *Registry) {
	r.OnETB("Notion Thief", notionThiefETB)
}

func notionThiefETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gameengine.RegisterNotionThiefReplacement(gs, perm)
	emit(gs, "notion_thief_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"timestamp": perm.Timestamp,
		"effect":    "opponent_draw_redirect",
	})
}
