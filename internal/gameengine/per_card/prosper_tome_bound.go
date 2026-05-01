package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerProsperTomeBound wires Prosper, Tome-Bound.
//
// Oracle text (Scryfall):
//
//	Ward {2}
//	Whenever Prosper, Tome-Bound enters the battlefield or attacks,
//	exile the top card of your library. You may play that card this
//	turn.
//	At the beginning of your end step, for each card exiled with
//	Prosper, Tome-Bound this turn, each opponent loses 1 life and you
//	create a Treasure token.
//
// Implementation:
//   - Ward {2} is granted by the AST keyword pipeline; this handler does
//     not implement it.
//   - OnETB exiles the top card and grants a zone-cast permission so
//     Prosper's controller may cast it from exile this turn for its
//     normal mana cost. Lands cannot be cast as spells; for those we
//     emit a partial event, but they still count toward the end-step
//     trigger because the rules require only that the card was exiled
//     "with Prosper" this turn.
//   - OnTrigger("creature_attacks") fires the same exile when Prosper
//     declares as an attacker.
//   - OnTrigger("end_step") drains 1 life from each living opponent and
//     mints a Treasure for the controller, repeated per card exiled with
//     Prosper this turn (tracked via a turn-keyed Flags counter).
func registerProsperTomeBound(r *Registry) {
	r.OnETB("Prosper, Tome-Bound", prosperETB)
	r.OnTrigger("Prosper, Tome-Bound", "creature_attacks", prosperAttacks)
	r.OnTrigger("Prosper, Tome-Bound", "end_step", prosperEndStep)
}

func prosperETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	prosperImpulseExile(gs, perm, "etb")
}

func prosperAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	prosperImpulseExile(gs, perm, "attack")
}

// prosperImpulseExile exiles the top card of Prosper's controller's library
// and grants a free-cost zone-cast permission for nonland spells. Bumps the
// turn-keyed exile counter on Prosper for the end-step drain.
func prosperImpulseExile(gs *gameengine.GameState, perm *gameengine.Permanent, source string) {
	const slug = "prosper_tome_bound_impulse_exile"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || len(s.Library) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"source": source,
			"reason": "library_empty",
		})
		return
	}
	card := s.Library[0]
	gameengine.MoveCard(gs, card, seat, "library", "exile", "prosper_impulse_exile")

	if !cardHasType(card, "land") {
		if gs.ZoneCastGrants == nil {
			gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
		}
		gs.ZoneCastGrants[card] = &gameengine.ZoneCastPermission{
			Zone:              gameengine.ZoneExile,
			Keyword:           "prosper_play_this_turn",
			ManaCost:          -1, // pay the card's normal cost
			RequireController: seat,
			SourceName:        perm.Card.DisplayName(),
		}
	} else {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"play_land_from_exile_grant_unimplemented")
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := "prosper_exiled_t" + strconv.Itoa(gs.Turn)
	perm.Flags[key]++

	gs.LogEvent(gameengine.Event{
		Kind:   "exile_from_library",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"card":   card.DisplayName(),
			"reason": "prosper_impulse_exile",
			"source": source,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"source":       source,
		"exiled_card":  card.DisplayName(),
		"is_land":      cardHasType(card, "land"),
		"exiled_count": perm.Flags[key],
	})
}

func prosperEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "prosper_tome_bound_end_step_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		return
	}
	key := "prosper_exiled_t" + strconv.Itoa(gs.Turn)
	count := perm.Flags[key]
	if count <= 0 {
		return
	}
	// Reset so re-entries / replays of the trigger don't double-fire.
	delete(perm.Flags, key)

	opps := gs.LivingOpponents(perm.Controller)
	for i := 0; i < count; i++ {
		for _, o := range opps {
			if o < 0 || o >= len(gs.Seats) {
				continue
			}
			gs.Seats[o].Life--
			gs.LogEvent(gameengine.Event{
				Kind:   "life_loss",
				Seat:   perm.Controller,
				Target: o,
				Source: perm.Card.DisplayName(),
				Amount: 1,
				Details: map[string]interface{}{
					"reason": "prosper_end_step",
				},
			})
		}
		gameengine.CreateTreasureToken(gs, perm.Controller)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"exiled":    count,
		"opponents": len(opps),
		"treasures": count,
	})
}
