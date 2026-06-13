package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// karona_false_god.go — per_card handler for Karona, False God.
//
// Oracle text (Legendary Creature — Avatar, {2}{W}{U}{B}{R}{G}):
//
//	Haste
//	At the beginning of each player's upkeep, that player untaps Karona
//	and gains control of it.
//	Whenever Karona attacks, creatures of the creature type of your
//	choice get +3/+3 until end of turn.
//
// Why this needs a per_card handler: the parser emitted a
// `trigger_fragment_upkeep` + `untyped_effect` + `parsed_effect_residual`
// for the upkeep clause, none of which the engine acted on — so Karona's
// signature "passes around the table" control-swap never happened and
// the card behaved like a vanilla hasty 5/5. Verified inert: no prior
// handler.
//
// Implemented here: the each-player-upkeep untap + control gain (the
// defining mechanic). The "whenever Karona attacks, chosen tribe +3/+3"
// rider is deliberately omitted (requires an interactive creature-type
// choice and is decorative relative to the control-swap); logged in the
// shard report.

func init() {
	registerKaronaFalseGod(Global())
	AddResetHook(registerKaronaFalseGod)
}

func registerKaronaFalseGod(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Karona, False God", "upkeep", karonaUpkeep)
}

func karonaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "karona_false_god"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat < 0 || activeSeat >= len(gs.Seats) || gs.Seats[activeSeat] == nil {
		return
	}

	// "That player untaps Karona..."
	perm.Tapped = false

	// "...and gains control of it." Reparent the Permanent to the active
	// player (CR §110.2a — controller may differ from owner). On a control
	// change the creature is summoning-sick under its new controller, but
	// Karona has haste so it can still attack.
	if perm.Controller != activeSeat {
		removePermanent(gs, perm)
		perm.Controller = activeSeat
		perm.SummoningSick = true
		gs.Seats[activeSeat].Battlefield = append(gs.Seats[activeSeat].Battlefield, perm)
		perm.Timestamp = gs.NextTimestamp()
	}

	emit(gs, slug, "Karona, False God", map[string]interface{}{
		"seat":      activeSeat,
		"untapped":  true,
		"new_owner": activeSeat,
	})
}
