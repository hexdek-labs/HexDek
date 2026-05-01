package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYidrisMaelstromWielder wires Yidris, Maelstrom Wielder
// (Commander 2016).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Trample
//	Whenever Yidris, Maelstrom Wielder deals combat damage to a player,
//	as you cast spells from your hand this turn, they gain cascade.
//	(When you cast the spell, exile cards from the top of your library
//	until you exile a nonland card that costs less. You may cast it
//	without paying its mana cost. Put the exiled cards on the bottom
//	in a random order.)
//
// Implementation:
//   - Trample is wired through the AST keyword pipeline.
//   - "combat_damage_player" trigger: when Yidris is the source and the
//     defender is a player, mark the controller's seat with a turn-stamped
//     flag (`yidris_cascade_active_turn_N`). Storing the turn lets us
//     gate the spell-cast hook even if the engine doesn't fire end_step
//     for our seat. We additionally clear the flag on the next end_step
//     to avoid the flag persisting beyond the granting turn under any
//     edge case (turn skip, extra turn, etc.).
//   - "spell_cast" trigger: fires for every spell cast while Yidris is
//     on the battlefield. Gate on (a) caster == Yidris's controller,
//     (b) cast_zone == "hand" (CR §702.84 cascade-cast/exile-cast not
//     eligible), and (c) the seat flag for the current turn is set.
//     Then call ApplyCascade with the cast spell's mana value.
//   - "end_step" trigger: clear the cascade-active flag at end of turn
//     so it doesn't carry into the next round.
//
// Note on ordering: spell_cast triggers fire BEFORE the original spell
// is pushed onto the stack (see stack.go ~L360). Yidris's trigger is
// pushed via PushPerCardTrigger and resolves inline before control
// returns, so the cascade-cast resolves before the original spell —
// matching real cascade timing where the cascade trigger goes on the
// stack above the spell that triggered it.
func registerYidrisMaelstromWielder(r *Registry) {
	r.OnTrigger("Yidris, Maelstrom Wielder", "combat_damage_player", yidrisCombatDamageGrant)
	r.OnTrigger("Yidris, Maelstrom Wielder", "spell_cast", yidrisCascadeOnCast)
	r.OnTrigger("Yidris, Maelstrom Wielder", "end_step", yidrisClearFlag)
}

func yidrisCombatDamageGrant(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yidris_combat_damage_grant_cascade"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceCard, _ := ctx["source_card"].(string)
	defenderSeat, _ := ctx["defender_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	if perm.Card == nil || sourceCard != perm.Card.DisplayName() {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["yidris_cascade_active"] = gs.Turn + 1 // 1-based to distinguish from default zero
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"turn":          gs.Turn,
	})
}

func yidrisCascadeOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yidris_cascade_on_cast"
	if gs == nil || perm == nil || ctx == nil {
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
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	stamp := seat.Flags["yidris_cascade_active"]
	if stamp == 0 || stamp != gs.Turn+1 {
		return
	}

	cmc := gameengine.ManaCostOf(card)
	hit := gameengine.ApplyCascade(gs, perm.Controller, cmc, card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"spell":    card.DisplayName(),
		"spell_mv": cmc,
		"hit":      hit,
	})
}

func yidrisClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	if seat.Flags["yidris_cascade_active"] == 0 {
		return
	}
	delete(seat.Flags, "yidris_cascade_active")
	emit(gs, "yidris_cascade_clear", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"turn": gs.Turn,
	})
}
