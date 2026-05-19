package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheLocustGod wires The Locust God.
//
// Oracle text (Scryfall, verified):
//
//	Flying
//	Whenever you draw a card, create a 1/1 blue and red Insect creature
//	token with flying and haste.
//	{2}{U}{R}: Draw a card, then discard a card.
//	When The Locust God dies, return it to its owner's hand at the
//	beginning of the next end step.
//
// Implementation (R41 stub port):
//   - Flying: AST keyword pipeline.
//   - Whenever-you-draw token: OnTrigger("card_drawn"), gated on
//     drawer == perm.Controller. Mints a 1/1 blue+red Insect with
//     flying and haste via CreateCreatureToken + kw flags.
//   - {2}{U}{R} activated loot: not modeled at per_card layer
//     (activated-ability surface) — emitPartial breadcrumb on ETB.
//   - "Dies → next end step return to hand" delayed-trigger: deferred
//     (the engine's dies trigger surface for self-recursion to owner's
//     hand is not yet exposed to per_card) — emitPartial breadcrumb.
func registerTheLocustGod(r *Registry) {
	r.OnETB("The Locust God", theLocustGodETB)
	r.OnTrigger("The Locust God", "card_drawn", theLocustGodOnDraw)
}

func theLocustGodETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_locust_god_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"loot_activated_ability_and_dies_to_hand_delayed_trigger_not_modeled_at_per_card_layer")
}

func theLocustGodOnDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_locust_god_insect_token"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	drawerSeat, _ := ctx["seat"].(int)
	if drawerSeat != perm.Controller {
		return
	}
	owner := gs.Seats[perm.Controller]
	if owner == nil || owner.Lost {
		return
	}
	tok := gameengine.CreateCreatureToken(
		gs,
		perm.Controller,
		"Insect",
		[]string{"creature", "insect"},
		1, 1,
	)
	if tok == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_creation_failed", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	if tok.Flags == nil {
		tok.Flags = map[string]int{}
	}
	tok.Flags["kw:flying"] = 1
	tok.Flags["kw:haste"] = 1
	// Haste implies not summoning sick for the attack legality check —
	// also clear the bit explicitly to match the printed effect.
	tok.SummoningSick = false
	if tok.Card != nil {
		tok.Card.Colors = []string{"U", "R"}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"token":  "Insect",
		"colors": []string{"U", "R"},
		"flying": true,
		"haste":  true,
	})
}
