package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNormanOsbornGreenGoblin wires Norman Osborn // Green Goblin.
//
// Oracle text (front face — Norman Osborn):
//
//   Norman Osborn can't be blocked.
//   Whenever Norman Osborn deals combat damage to a player, he connives.
//   (Draw a card, then discard a card. If you discarded a nonland card,
//    put a +1/+1 counter on this creature.)
//   {1}{U}{B}{R}: Transform Norman Osborn. Activate only as a sorcery.
//
// Back face / front-rider clauses (deferred):
//
//   Flying, menace (back face — AST keyword pipeline).
//   Spells you cast from your graveyard cost {2} less to cast.
//   Goblin Formula — Each nonland card in your graveyard has mayhem.
//
// R37 port:
//
//   - "Can't be blocked": flag breadcrumb (engine doesn't yet honor
//     per-permanent unblockable from a per_card hook reliably; AST
//     keyword "unblockable" path is the canonical wiring). Emitted as
//     partial.
//   - "Whenever Norman deals combat damage to a player, he connives":
//     PORTED. Hook fires on the combat-damage-to-player trigger fan-
//     out from combat.go (event "creature_dealt_combat_damage_to_player"
//     or aliases). Calls PerformConnive which handles draw + discard
//     + +1/+1 counter on nonland discard per CR §701.48.
//   - Transform activated ability: deferred (DFC transform engine path
//     handles the swap itself but the "{1}{U}{B}{R}, sorcery speed"
//     gating is per-card setup we're not porting in this round).
//   - Graveyard-cast cost reduction + Mayhem grant on the back face:
//     deferred to the §702.187 Mayhem cast-helper work that's already
//     landed; back-face wiring is a separate per_card port.
func registerNormanOsbornGreenGoblin(r *Registry) {
	r.OnETB("Norman Osborn // Green Goblin", normanOsbornETBPartial)
	// "combat_damage_player" is the FireCardTrigger fan-out in
	// combat.go's fireCombatDamageTriggers — ctx carries source_card
	// (display name) and source_seat. We filter inside the handler.
	r.OnTrigger("Norman Osborn // Green Goblin",
		"combat_damage_player", normanOsbornCombatDamageConnive)
}

func normanOsbornETBPartial(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "norman_osborn_etb"
	if gs == nil || perm == nil {
		return
	}
	// "Norman Osborn can't be blocked." Stamp the canonical
	// Flags["unblockable"]=1 the combat blocker-legality check reads
	// (same wiring as Wraith, Vicious Vigilante). The flag is per-
	// permanent so it dies with the perm at LTB — no cleanup needed.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["unblockable"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"unblockable": true,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"transform + back-face graveyard-cast cost reduction not yet ported; combat-damage connive handled by trigger hook")
}

// normanOsbornCombatDamageConnive fires for every combat-damage-to-
// player event while a Norman is on the battlefield. The
// fireCombatDamageTriggers ctx in combat.go carries source_card
// (display name) and source_seat — we filter to Norman-self by
// matching both. Different creature dealing combat damage → ignore.
func normanOsbornCombatDamageConnive(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "norman_osborn_connive"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	srcCard, _ := ctx["source_card"].(string)
	srcSeat, ok := ctx["source_seat"].(int)
	if !ok || srcCard != perm.Card.DisplayName() || srcSeat != perm.Controller {
		return
	}
	connived := gameengine.PerformConnive(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"connived": connived,
	})
}
