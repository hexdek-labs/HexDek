package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBirgiGodOfStorytelling wires Birgi, God of Storytelling.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Birgi%2C%20God%20of%20Storytelling):
//
//	Whenever you cast a spell, add {R}.
//
// {1}{R} Legendary Creature — God 3/2. (Back face: Harnfel, Horn of
// Bounty — exile-cast loot artifact, handled separately when stubbed.)
// The on-cast {R} trigger is a "free ritual on every cast" engine —
// in storm/wheel decks it pays for the next cast on the chain. Pairs
// with Underworld Breach for free-cast-from-graveyard plays; pairs
// with Storm-Kiln Artist as a permanent-side Treasure engine analog.
//
// Implementation:
//   - OnTrigger("spell_cast", ...) — fires on every spell cast under
//     Birgi's controller, including Birgi's own cast (the trigger
//     resolves AFTER Birgi resolves, so a Birgi-cast doesn't feed
//     its own mana; but if Birgi was already in play and a different
//     spell is cast, this fires correctly). Gate on caster_seat ==
//     controller per "you cast" semantics.
//   - AddMana(seat, "R", 1, ...) — single red mana to controller's
//     pool. Note: per_card layer treats mana pool as generic; the
//     "R" color tag is logged for storm-pile auditors but doesn't
//     constrain downstream spending in the current pool model.
//   - No once-per-turn gate — the printed text triggers on EVERY
//     spell cast, which is exactly the storm-engine value.
func registerBirgiGodOfStorytelling(r *Registry) {
	r.OnTrigger("Birgi, God of Storytelling", "spell_cast", birgiSpellCastTrigger)
}

func birgiSpellCastTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "birgi_god_of_storytelling_add_mana"
	if gs == nil || perm == nil {
		return
	}
	casterSeatAny := ctx["caster_seat"]
	casterSeat, _ := casterSeatAny.(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	gameengine.AddMana(gs, seat, "R", 1, "Birgi, God of Storytelling")
	emit(gs, slug, "Birgi, God of Storytelling", map[string]interface{}{
		"seat":  perm.Controller,
		"added": 1,
		"color": "R",
	})
}
