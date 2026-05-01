package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFlubsTheFool wires Flubs, the Fool. Batch #31.
//
// Oracle text (Bloomburrow Commander, {G}{U}{R}, 0/5 Frog Scout):
//
//	You may play an additional land on each of your turns.
//	Whenever you play a land or cast a spell, draw a card if you have
//	no cards in hand. Otherwise, discard a card.
//
// Implementation:
//   - ETB: grants the seat an extra land drop (best-effort — the engine's
//     `extra_land_drops` flag is recorded but not currently consumed by
//     the land-play action, mirroring Hearthhull's pattern). The static
//     "additional land each turn" effect is otherwise handled (or
//     punted) by the AST engine's continuous-effect layer; emitPartial
//     records the gap.
//   - "spell_cast" trigger gated on caster_seat == perm.Controller and
//     skipping Flubs herself: hand-empty → draw 1; else discard 1.
//   - "permanent_etb" trigger gated on the entering permanent being a
//     land controlled by Flubs's controller: same draw-or-discard
//     resolution. permanent_etb is the closest signal we have to "you
//     played a land" — most lands enter via being played, and the
//     trigger does not distinguish between play vs put-into-play (which
//     is technically incorrect for cards like Field of the Dead's
//     Zombies, but Flubs's text only fires when YOUR LAND enters and
//     non-play land entries from your own effects are vanishingly rare).
func registerFlubsTheFool(r *Registry) {
	r.OnETB("Flubs, the Fool", flubsETB)
	r.OnTrigger("Flubs, the Fool", "spell_cast", flubsSpellCast)
	r.OnTrigger("Flubs, the Fool", "permanent_etb", flubsLandETB)
}

func flubsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "flubs_the_fool_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["extra_land_drops"]++
	gs.LogEvent(gameengine.Event{
		Kind:   "extra_land_drop",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug":   slug,
			"reason": "flubs_the_fool_static_additional_land",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"static_additional_land_per_turn_uses_one_shot_flag_engine_does_not_consume")
}

// flubsDrawOrDiscard implements the shared "draw a card if you have no
// cards in hand. Otherwise, discard a card." resolution.
func flubsDrawOrDiscard(gs *gameengine.GameState, perm *gameengine.Permanent, source string) {
	const slug = "flubs_the_fool_draw_or_discard"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if len(seat.Hand) == 0 {
		drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
		drawnName := ""
		if drawn != nil {
			drawnName = drawn.DisplayName()
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":           perm.Controller,
			"trigger_source": source,
			"action":         "draw",
			"drawn":          drawnName,
		})
		return
	}
	discarded := gameengine.DiscardN(gs, perm.Controller, 1, "random")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"trigger_source": source,
		"action":         "discard",
		"discarded":      discarded,
	})
}

func flubsSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	// Flubs cast from the command zone shouldn't trigger her own ability —
	// she isn't on the battlefield yet at cast time, but defensive.
	if card != nil && card == perm.Card {
		return
	}
	flubsDrawOrDiscard(gs, perm, "spell_cast")
}

func flubsLandETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	if !entering.IsLand() {
		return
	}
	flubsDrawOrDiscard(gs, perm, "land_etb")
}
