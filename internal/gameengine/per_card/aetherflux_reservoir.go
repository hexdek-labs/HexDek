package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAetherfluxReservoir wires up Aetherflux Reservoir.
//
// Oracle text:
//
//	Whenever you cast a spell, you gain 1 life for each spell you've
//	cast this turn.
//	Pay 50 life: Aetherflux Reservoir deals 50 damage to any target.
//	Activate only as a sorcery.
//
// Wincon for Oloro-style lifegain decks. The "cast this turn" counter is
// already tracked by the storm agent's cast_counts.go (shared state). We
// re-compute it conservatively by scanning the event log for this turn's
// "cast" events from the controller — if cast_counts.go is not yet
// landed, we fall back to 1 (the triggering spell itself counts).
//
// Handlers:
//   - OnTrigger("spell_cast", ...) — gain N life where N = cast count.
//   - OnActivated(0, ...) — pay 50 life, deal 50 damage to ctx["target_seat"].
func registerAetherfluxReservoir(r *Registry) {
	r.OnTrigger("Aetherflux Reservoir", "spell_cast", aetherfluxOnSpellCast)
	r.OnActivated("Aetherflux Reservoir", aetherfluxActivate)
}

func aetherfluxOnSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aetherflux_reservoir_on_cast"
	if gs == nil || perm == nil {
		return
	}
	// Only trigger when the caster is the Reservoir's controller. Oracle
	// text is "Whenever YOU cast a spell".
	casterSeatAny, _ := ctx["caster_seat"]
	casterSeat, _ := casterSeatAny.(int)
	if casterSeat != perm.Controller {
		return
	}
	// Count "cast this turn" events by the controller.
	casts := countCastsThisTurnBy(gs, perm.Controller)
	if casts < 1 {
		casts = 1 // defensive: the triggering cast itself counts
	}
	// Gain `casts` life.
	gameengine.GainLife(gs, perm.Controller, casts, perm.Card.DisplayName())
	gs.LogEvent(gameengine.Event{
		Kind:   "gain_life",
		Seat:   perm.Controller,
		Target: perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: casts,
		Details: map[string]interface{}{
			"reason":       "aetherflux_reservoir",
			"casts_this_turn": casts,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"casts_this_turn": casts,
		"life_gained":     casts,
	})
}

func aetherfluxActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "aetherflux_reservoir_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	if s.Life < 50 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_life", map[string]interface{}{
			"life": s.Life,
		})
		return
	}
	// Pay 50 life.
	s.Life -= 50
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: src.Card.DisplayName(),
		Amount: 50,
		Details: map[string]interface{}{
			"reason": "aetherflux_activation_cost",
		},
	})
	// Deal 50 damage to target.
	targetSeat := -1
	if v, ok := ctx["target_seat"].(int); ok {
		targetSeat = v
	}
	if targetSeat < 0 || targetSeat >= len(gs.Seats) {
		// Default to first opponent.
		opps := gs.Opponents(seat)
		if len(opps) > 0 {
			targetSeat = opps[0]
		}
	}
	if targetSeat >= 0 && targetSeat < len(gs.Seats) {
		gs.Seats[targetSeat].Life -= 50
		gs.LogEvent(gameengine.Event{
			Kind:   "damage",
			Seat:   seat,
			Target: targetSeat,
			Source: src.Card.DisplayName(),
			Amount: 50,
		})
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"target_seat": targetSeat,
			"damage":      50,
		})
		_ = gs.CheckEnd()
	}
}

// countCastsThisTurnBy counts "cast" events on the event log for the
// current turn by the specified seat. When storm counters (cast_counts.go)
// are available, callers should prefer those; this is a fallback.
func countCastsThisTurnBy(gs *gameengine.GameState, seat int) int {
	if gs == nil {
		return 0
	}
	// Walk backwards from the tail until we cross a turn boundary
	// (turn_start / begin_turn event) — conservative but robust.
	n := 0
	for i := len(gs.EventLog) - 1; i >= 0; i-- {
		ev := gs.EventLog[i]
		if ev.Kind == "turn_start" || ev.Kind == "begin_turn" || ev.Kind == "untap_step" {
			break
		}
		if ev.Kind == "cast" && ev.Seat == seat {
			n++
		}
	}
	return n
}
