package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSarumanTheWhiteHand wires Saruman the White Hand.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Ward {2}
//	Whenever you cast an instant or sorcery spell from your graveyard,
//	create two 1/1 white Human Soldier creature tokens.
//	At the beginning of your upkeep, surveil 2.
//
// Implementation:
//   - OnETB: stamp ward {2} into perm.Flags so the ward-on-target machinery
//     finds it (tests/tokens without an AST keyword still need this flag).
//   - "instant_or_sorcery_cast": gate on caster_seat == controller and
//     ctx["cast_zone"] == graveyard, then mint two white Human Soldier
//     tokens via the standard token pipeline (so Anointed Procession et al.
//     can chain off token_created).
//   - "upkeep_controller": surveil 2 on the active player's upkeep when
//     it's Saruman's controller.
func registerSarumanTheWhiteHand(r *Registry) {
	r.OnETB("Saruman the White Hand", sarumanWhiteHandETB)
	r.OnTrigger("Saruman the White Hand", "instant_or_sorcery_cast", sarumanWhiteHandGraveCast)
	r.OnTrigger("Saruman the White Hand", "upkeep_controller", sarumanWhiteHandUpkeep)
}

func sarumanWhiteHandETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kw:ward"] = 1
	perm.Flags["ward_cost"] = 2
}

func sarumanWhiteHandGraveCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "saruman_white_hand_graveyard_cast"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	zone, _ := ctx["cast_zone"].(string)
	if zone != gameengine.ZoneGraveyard {
		return
	}

	for i := 0; i < 2; i++ {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Human Soldier",
			[]string{"creature", "human", "soldier", "pip:W"}, 1, 1)
	}

	spellName := ""
	if c, ok := ctx["card"].(*gameengine.Card); ok && c != nil {
		spellName = c.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"spell_name": spellName,
		"tokens":     2,
		"token_type": "Human Soldier",
	})
}

func sarumanWhiteHandUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "saruman_white_hand_upkeep_surveil"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	gameengine.Surveil(gs, perm.Controller, 2)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"surveil": 2,
	})
}
