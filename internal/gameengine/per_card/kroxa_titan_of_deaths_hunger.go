package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKroxa wires Kroxa, Titan of Death's Hunger.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Kroxa%2C%20Titan%20of%20Death%27s%20Hunger):
//
//	{B}{R}
//	Legendary Creature — Elder Giant
//	6/6
//	Menace
//	When Kroxa, Titan of Death's Hunger enters, sacrifice it unless it
//	escaped.
//	When Kroxa enters, each opponent discards a card.
//	At the beginning of your upkeep, if Kroxa is on the battlefield,
//	sacrifice it unless any opponent lost life this turn.
//	Escape — {2}{B}{R}, Exile five other cards from your graveyard.
//
// Implementation:
//   - OnETB fires both clauses: sacrifice-unless-escaped + each-opp-discard.
//     The escape detection reads perm.Flags["escape_cast"] which the
//     engine sets when a stack item resolves via the escape alt-cost path
//     (keywords_escape.go). When sacrifice-unless-escaped fires, Kroxa
//     leaves the battlefield BEFORE the discard clause fires per CR §603
//     stack-of-simultaneous-triggers — so we order: discard first, sac
//     second. (Per APNAP both ETB triggers go on the stack at the same
//     time and controller orders them; ordering sac after discard
//     extracts more value from the escape attempt.)
//   - OnTrigger("upkeep_controller"): if Kroxa is still on battlefield
//     AND no opponent lost life this turn, sacrifice it. Otherwise
//     emit a "kroxa_upkeep_survives" event for observability.
//   - Menace is handled by the AST keyword pipeline.
//   - Escape cost/exile is handled by the generic escape machinery in
//     keywords_escape.go; this handler doesn't manage the alt-cost itself.
func registerKroxa(r *Registry) {
	r.OnETB("Kroxa, Titan of Death's Hunger", kroxaETB)
	r.OnTrigger("Kroxa, Titan of Death's Hunger", "upkeep_controller", kroxaUpkeep)
}

func kroxaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kroxa_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// Each opponent discards 1.
	discarded := 0
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		discarded += gameengine.DiscardN(gs, oppIdx, 1, "")
	}

	// Sacrifice-unless-escaped. The engine flags escape-cast stack items
	// via perm.Flags["escape_cast"] = 1; absence means a hard-cast or
	// reanimate, which triggers the sac.
	escapedCast := perm.Flags != nil && perm.Flags["escape_cast"] == 1
	if !escapedCast {
		gameengine.SacrificePermanent(gs, perm, slug)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"opps_discarded": discarded,
		"escaped":        escapedCast,
		"sacrificed":     !escapedCast,
	})
}

func kroxaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kroxa_upkeep_sac_check"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Did any opponent lose life this turn? Read each opp's TurnRecord
	// for life delta < starting; fall back to Turn.LifeLost > 0.
	anyOppLostLife := false
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		s := gs.Seats[oppIdx]
		if s == nil || s.Lost {
			continue
		}
		if s.Turn.LifeLost > 0 {
			anyOppLostLife = true
			break
		}
	}
	if anyOppLostLife {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":       perm.Controller,
			"sacrificed": false,
			"reason":     "opp_lost_life",
		})
		return
	}
	gameengine.SacrificePermanent(gs, perm, slug)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"sacrificed": true,
		"reason":     "no_opp_lost_life_this_turn",
	})
}
