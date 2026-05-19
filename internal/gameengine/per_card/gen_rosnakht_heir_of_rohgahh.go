package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRosnakhtHeirOfRohgahh wires Rosnakht, Heir of Rohgahh.
//
// Oracle text (Scryfall, verified):
//
//	Battle cry (Whenever this creature attacks, each other attacking
//	creature gets +1/+0 until end of turn.)
//	Heroic — Whenever you cast a spell that targets Rosnakht, create a
//	0/1 red Kobold creature token named Kobolds of Kher Keep.
//
// Implementation (R42b stub port):
//   - Battle cry: AST keyword pipeline (keywords_combat.go fires the
//     attack-time +1/+0 to other attackers). Per_card layer only
//     emits a breadcrumb on ETB so the registration-coverage lint
//     stays satisfied.
//   - Heroic Kobold token: OnTrigger("spell_cast") gated to
//     caster_seat == Rosnakht's controller. Walks the stack item's
//     Targets looking for Rosnakht herself; if found, mints a
//     "Kobolds of Kher Keep" 0/1 red Kobold via CreateCreatureToken.
//     Multiple Heroic targets in the same spell still trigger once
//     per spell-cast (CR §702.85a — "whenever ... cast a spell that
//     targets ..." fires once per spell regardless of target count).
func registerRosnakhtHeirOfRohgahh(r *Registry) {
	r.OnETB("Rosnakht, Heir of Rohgahh", rosnakhtHeirOfRohgahhETB)
	r.OnTrigger("Rosnakht, Heir of Rohgahh", "spell_cast", rosnakhtHeroicKoboldToken)
}

func rosnakhtHeirOfRohgahhETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rosnakht_heir_of_rohgahh_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"battle_cry_attacking_buff_handled_by_AST_keyword_pipeline")
}

func rosnakhtHeroicKoboldToken(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rosnakht_heroic_kobold_token"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}

	// Locate the StackItem so we can inspect Targets. ctx["stack_item"]
	// is the canonical handoff; fall back to scanning the stack for a
	// frame carrying the spell's card pointer.
	var item *gameengine.StackItem
	if si, ok := ctx["stack_item"].(*gameengine.StackItem); ok && si != nil {
		item = si
	} else if spellCard, ok := ctx["card"].(*gameengine.Card); ok && spellCard != nil {
		for i := len(gs.Stack) - 1; i >= 0; i-- {
			s := gs.Stack[i]
			if s != nil && s.Card == spellCard {
				item = s
				break
			}
		}
	}
	if item == nil {
		return
	}

	hits := false
	for _, t := range item.Targets {
		if t.Kind != gameengine.TargetKindPermanent || t.Permanent == nil {
			continue
		}
		if t.Permanent == perm {
			hits = true
			break
		}
	}
	if !hits {
		return
	}

	tok := gameengine.CreateCreatureToken(
		gs,
		perm.Controller,
		"Kobolds of Kher Keep",
		[]string{"creature", "kobold"},
		0, 1,
	)
	if tok == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_creation_failed", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	if tok.Card != nil {
		tok.Card.Colors = []string{"R"}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"token":  "Kobolds of Kher Keep",
		"colors": []string{"R"},
	})
}
