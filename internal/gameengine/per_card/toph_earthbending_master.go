package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTophEarthbendingMaster wires Toph, Earthbending Master.
//
// Oracle text:
//
//	{3}{G}
//	Legendary Creature — Human Warrior Ally
//	Landfall — Whenever a land you control enters, you get an experience
//	  counter.
//	Whenever you attack, earthbend X, where X is the number of experience
//	  counters you have. (Target land you control becomes a 0/0 creature
//	  with haste that's still a land. Put X +1/+1 counters on it. When it
//	  dies or is exiled, return it to the battlefield tapped.)
//
// Implementation:
//   - "permanent_etb" trigger gated to controller == Toph's controller
//     and the entering perm is a land: increment experience counters.
//   - "attacks" trigger (any attack by controller): pick a non-creature
//     own land, mark it as a 0/0 creature with haste (Flags), and add
//     X +1/+1 counters where X = experience.
//   - The "return on die/exile" rider is an LTB replacement effect not
//     surfaced cleanly to per_card; emitPartial covers it.
func registerTophEarthbendingMaster(r *Registry) {
	r.OnTrigger("Toph, Earthbending Master", "permanent_etb", tophLandfall)
	r.OnTrigger("Toph, Earthbending Master", "attacks", tophEarthbend)
}

func tophLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toph_landfall_experience"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	ctrl, _ := ctx["controller_seat"].(int)
	if ctrl != perm.Controller {
		return
	}
	enterPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enterPerm == nil || !enterPerm.IsLand() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["experience"]++
	gs.LogEvent(gameengine.Event{
		Kind:   "counter_mod",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"counter_kind": "experience",
			"op":           "put",
			"on_player":    true,
			"reason":       "toph_landfall",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"total": seat.Flags["experience"],
	})
}

func tophEarthbend(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toph_earthbend"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkSeat, _ := ctx["seat"].(int)
	if atkSeat != perm.Controller {
		return
	}
	// "Whenever you attack" fires once per declare-attackers step, but the
	// engine fires "attacks" once per declared attacker. Gate to the first
	// hit per turn so earthbend resolves once per combat, not once per
	// attacker (Caesar/Raffine once-per-turn dedup pattern).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["toph_earthbend_fired"] >= gs.Turn {
		return
	}
	perm.Flags["toph_earthbend_fired"] = gs.Turn
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	x := seat.Flags["experience"]
	if x <= 0 {
		return
	}
	// Pick a non-creature land we control.
	var target *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsLand() && !p.IsCreature() {
			target = p
			break
		}
	}
	if target == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"x":      x,
			"reason": "no_land_target",
		})
		return
	}
	// R54 layer-7b port: earthbend N — register the canonical
	// "becomes a 0/0 creature with haste that's still a land"
	// continuous-effect chain. Layer 4 adds "creature", Layer 7b
	// sets base P/T to 0/0, Layer 6 grants haste. Each effect's
	// SourcePerm = target, so UnregisterContinuousEffectsForPermanent
	// tears them down naturally when the land leaves the battlefield.
	// X +1/+1 counters stack on top of the 0/0 base via §613.4c
	// (applyCountersAndMods), so combat math reads X/X correctly.
	target.Flags = ensureFlags(target.Flags)
	target.Flags["earthbent"] = 1
	gameengine.RegisterAddTypes(gs, target, []string{"creature"},
		gameengine.DurationUntilSourceLeaves, "Toph, Earthbending Master", "earthbend")
	gameengine.RegisterSetPT(gs, target, 0, 0,
		gameengine.DurationUntilSourceLeaves, "Toph, Earthbending Master", "earthbend")
	gameengine.RegisterGrantKeyword(gs, target, "haste",
		gameengine.DurationUntilSourceLeaves, "Toph, Earthbending Master", "earthbend")
	target.AddCounter("+1/+1", x)
	target.SummoningSick = false
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"target":   target.Card.DisplayName(),
		"counters": x,
		"layer_7b": [2]int{0, 0},
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"return_on_die_or_exile_replacement_partial")
}

// ensureFlags ensures p.Flags is non-nil so subsequent indexing is safe.
// Helper local to the earthbend port for ergonomic flag stamping.
func ensureFlags(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}
