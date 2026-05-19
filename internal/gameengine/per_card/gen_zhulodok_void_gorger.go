package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZhulodokVoidGorger wires Zhulodok, Void Gorger.
//
// Oracle text (Scryfall, verified, {5}{C}, 6/6 legendary Eldrazi):
//
//	Colorless spells you cast from your hand with mana value 7 or
//	greater have "Cascade, cascade." (When you cast one, exile cards
//	from the top of your library until you exile a nonland card that
//	costs less. You may cast it without paying its mana cost. Put the
//	exiled cards on the bottom in a random order. Then do it again.)
//
// Implementation (R42 stub port):
//   - spell_cast trigger gated on caster == controller, cast_zone ==
//     hand, the cast spell is colorless (len Card.Colors == 0), and
//     CMC >= 7.
//   - When all gates pass we fire gameengine.ApplyCascade twice — the
//     "Cascade, cascade." grant. ApplyCascade handles the §702.84a
//     exile / find-nonland-less-than-CMC / free-cast / bottom-shuffle
//     loop, plus the cascade_trigger / cascade_hit observability
//     events the analytics layer reads.
//   - Each cascade invocation passes the ORIGINAL spell's CMC as the
//     bound (the granted "Cascade" inherits the cast spell's mana
//     value, not the previous cascade hit's value — the second
//     cascade is "cascade off the same 7-MV spell," not a stacked
//     chain). Matches the official rulings on Apex Devastator /
//     Throes of Chaos when granted "cascade, cascade" by Maelstrom.
func registerZhulodokVoidGorger(r *Registry) {
	r.OnTrigger("Zhulodok, Void Gorger", "spell_cast", zhulodokDoubleCascade)
}

func zhulodokDoubleCascade(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zhulodok_void_gorger_double_cascade"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	castZone, _ := ctx["cast_zone"].(string)
	if castZone != "" && castZone != "hand" {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if len(card.Colors) != 0 {
		return
	}
	cmc := cardCMC(card)
	if cmc < 7 {
		return
	}
	// Don't cascade off self — Zhulodok is itself a {5}{C} 6-MV cast,
	// below the 7-MV threshold, so this is defensive depth for any
	// future copy/clone that lands on the stack with a different CMC.
	if card == perm.Card {
		return
	}

	spellName := card.DisplayName()
	hit1 := gameengine.ApplyCascade(gs, perm.Controller, cmc, spellName)
	hit2 := gameengine.ApplyCascade(gs, perm.Controller, cmc, spellName)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"spell":  spellName,
		"cmc":    cmc,
		"hit_1":  hit1,
		"hit_2":  hit2,
	})
}
