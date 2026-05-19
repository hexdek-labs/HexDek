package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGaleaKindlerOfHope wires Galea, Kindler of Hope.
//
// Oracle text (Scryfall, verified):
//
//	Vigilance
//	You may look at the top card of your library any time.
//	You may cast Aura and Equipment spells from the top of your
//	library. When you cast an Equipment spell this way, it gains
//	"When this Equipment enters, attach it to target creature you
//	control."
//
// Implementation (R43 stub port):
//   - Vigilance: AST keyword pipeline.
//   - "Look at top of library": no-op in current observation model
//     (no hidden information modeled).
//   - "Cast Aura/Equipment from top of library": OnETB registers a
//     ZoneCastPermission on the controller's top library card iff
//     that card is an Aura or Equipment. The "while_source_on_bf"
//     duration ties the grant to Galea, so it expires when she
//     leaves. Refresh hook on card_drawn (after each draw the new
//     top changes) re-registers if the fresh top qualifies, drops a
//     stale grant otherwise.
//   - Auto-attach-on-Equipment-cast rider: emitPartial. The cast
//     pipeline doesn't yet thread "cast via galea" CostMeta to the
//     post-resolve attach step.
func registerGaleaKindlerOfHope(r *Registry) {
	r.OnETB("Galea, Kindler of Hope", galeaKindlerOfHopeETB)
	r.OnTrigger("Galea, Kindler of Hope", "card_drawn", galeaRefreshTopGrant)
}

func galeaKindlerOfHopeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "galea_kindler_of_hope_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	granted := galeaRegisterTopGrant(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"top_eligible": granted,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"equipment_auto_attach_on_galea_cast_rider_not_wired_at_per_card_layer")
}

func galeaRefreshTopGrant(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	drawerSeat, _ := ctx["seat"].(int)
	if drawerSeat != perm.Controller {
		return
	}
	galeaRegisterTopGrant(gs, perm)
}

// galeaRegisterTopGrant inspects the controller's top library card. If
// it's an Aura or Equipment, register a library-cast permission (pay
// normal mana cost from exile-of-library zone). Returns true iff a
// grant was registered.
func galeaRegisterTopGrant(gs *gameengine.GameState, perm *gameengine.Permanent) bool {
	if gs == nil || perm == nil || perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[perm.Controller]
	if s == nil || len(s.Library) == 0 {
		return false
	}
	top := s.Library[0]
	if top == nil {
		return false
	}
	isAura := cardHasType(top, "aura") || cardHasSubtype(top, "aura")
	isEquip := cardHasSubtype(top, "equipment")
	if !isAura && !isEquip {
		// Top isn't eligible — drop any stale grant we previously
		// installed for this card (cleanup keeps the grants map tight
		// when the top rotates to a non-eligible card).
		if gs.ZoneCastGrants != nil {
			if g, ok := gs.ZoneCastGrants[top]; ok && g != nil && g.SourceName == "Galea, Kindler of Hope" {
				gameengine.RemoveZoneCastGrant(gs, top)
			}
		}
		return false
	}
	gameengine.RegisterZoneCastGrant(gs, top, &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneLibrary,
		Keyword:           "galea_top_cast",
		ManaCost:          -1, // pay normal mana cost
		RequireController: perm.Controller,
		SourceName:        "Galea, Kindler of Hope",
		Duration:          "while_source_on_bf",
		SourceTimestamp:   perm.Timestamp,
		GrantTurn:         gs.Turn,
	})
	return true
}
