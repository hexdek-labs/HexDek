package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZaxaraTheExemplary wires Zaxara, the Exemplary.
//
// Oracle text (Ikoria Commander, {1}{B}{G}{U}, Legendary Creature — Nightmare Hydra,
// verified 2026-05-02):
//
//	Deathtouch
//	{T}: Add two mana of any one color.
//	Whenever you cast a spell with {X} in its mana cost, create a 0/0
//	green Hydra creature token, then put X +1/+1 counters on it.
//
// Implementation:
//   - Deathtouch: handled by AST keyword pipeline.
//   - {T}: Add two mana — mana ability, handled by the mana system
//     (emitPartial coverage note below).
//   - OnTrigger("spell_cast"): gate on caster_seat == perm.Controller
//     (Zaxara only triggers for its controller's spells) and
//     ManaCostContainsX(card). X is read from gs.Flags["_cast_chosen_x"],
//     set by CastSpell before firing triggers. If X == 0, a 0/0 Hydra
//     token is still created (oracle says "put X counters", so X=0 →
//     no counters, valid 0/0 Hydra).
//
// Coverage gaps:
//   - {T}: Add two mana is a mana ability (CR §605.1a), not a triggered
//     or activated ability — handled generically by the mana system.
//     Noted via emitPartial.
//   - Doubling Season / Parallel Lives double token creation and counter
//     placement respectively via the replacement layer (CreateCreatureToken
//     fires would_create_token; AddCounter doesn't hook would_put_counter
//     per current per_card convention — consistent with Zimone, Primo,
//     Walking Ballista).
//
// Token spec:
//
//	Name="Hydra Token", Power=0, Toughness=0,
//	Types=["creature","hydra"], Colors=["G"].
func registerZaxaraTheExemplary(r *Registry) {
	r.OnTrigger("Zaxara, the Exemplary", "spell_cast", zaxaraTheExemplarySpellCast)
}

func zaxaraTheExemplarySpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zaxara_the_exemplary_hydra_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Only trigger when Zaxara's controller casts the spell.
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}

	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}

	// Gate: spell must contain {X} in its mana cost (CR §107.3).
	if !gameengine.ManaCostContainsX(card) {
		return
	}

	// CastSpell sets gs.Flags["_cast_chosen_x"] before firing triggers
	// and deletes it immediately after — always available regardless of
	// RetainEvents mode.
	x := gs.Flags["_cast_chosen_x"]

	// Create a 0/0 green Hydra creature token (oracle: "create a 0/0 green
	// Hydra creature token").
	tok := gameengine.CreateCreatureToken(
		gs,
		perm.Controller,
		"Hydra Token",
		[]string{"creature", "hydra"},
		0, 0,
	)
	if tok == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_creation_failed", nil)
		return
	}
	if tok.Card != nil {
		tok.Card.Colors = []string{"G"}
	}

	// Put X +1/+1 counters on the token (oracle: "put X +1/+1 counters on it").
	if x > 0 {
		tok.AddCounter("+1/+1", x)
		gs.InvalidateCharacteristicsCache()
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"spell_name":     card.DisplayName(),
		"x_value":        x,
		"token_name":     "Hydra Token",
		"token_pt":       "0/0",
		"counters_added": x,
		"token_color":    "G",
	})

}

