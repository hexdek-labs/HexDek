package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// etb_drain_target_opponent_family.go — generic handler for the
// "When this creature enters, target opponent loses N life and you gain
// N life" family.
//
// Shape (Skymarch Bloodletter, Vampire Sovereign, Highway Robber, ...):
//
//	When this creature enters, target opponent loses N life and you
//	gain N life.
//
// Every member runs the same algorithm:
//   1. Pick a target opponent — lowest-life living opponent. Lowest-life
//      maximizes lethal-pressure (lethal triggers feed into win-line
//      detection) and matches the choose-target heuristic used by
//      Athreos, Belakor, Ajani Nacatl Pariah, and the other drain
//      handlers in this package.
//   2. Lose N life on the opponent (via gameengine.LoseLife, which fires
//      life_lost triggers and feeds Bloodchief Ascension / Vito).
//   3. Gain N life on the controller (via gameengine.GainLife, which
//      fires life_gained triggers and feeds the Soul Sister / Karlov /
//      lifegain_counter_family observers).
//   4. CheckEnd so SBAs see any lethal life-total change before the
//      stack continues resolving.
//
// Hand-rolled siblings (Kalastria Healer with allied gate, Tithe Drinker
// extort, Blood Artist with creature-died gate, Falkenrath Noble with
// damage gate, Vito with conversion) keep their bespoke handlers — this
// family only owns the un-gated "creature enters → drain N" shape.
//
// Adding a new family member is one row in etbDrainTargetOpponentEntries.

type etbDrainEntry struct {
	cardName string
	amount   int
}

var etbDrainTargetOpponentEntries = []etbDrainEntry{
	{
		// Skymarch Bloodletter — {2}{B}, 2/2 Vampire Soldier with flying.
		//   When this creature enters, target opponent loses 1 life and
		//   you gain 1 life.
		cardName: "Skymarch Bloodletter",
		amount:   1,
	},
	{
		// Vampire Sovereign — {4}{B}{B}, 3/3 Vampire with flying.
		//   When this creature enters, target opponent loses 3 life and
		//   you gain 3 life.
		cardName: "Vampire Sovereign",
		amount:   3,
	},
	{
		// Highway Robber — {3}{B}, 2/2 Human Rogue.
		//   When this creature enters, target opponent loses 2 life and
		//   you gain 2 life.
		cardName: "Highway Robber",
		amount:   2,
	},
	{
		// Dakmor Ghoul — {3}{B}, 2/3 Zombie.
		//   When this creature enters, target opponent loses 2 life and
		//   you gain 2 life.
		cardName: "Dakmor Ghoul",
		amount:   2,
	},
	{
		// Bloodborn Scoundrels — {5}{B} (Assist), 5/4 Vampire Pirate.
		//   When this creature enters, target opponent loses 2 life and
		//   you gain 2 life.
		// Assist is declarative cost-routing handled by the engine — the
		// drain body is identical to the rest of the family.
		cardName: "Bloodborn Scoundrels",
		amount:   2,
	},
}

func registerEtbDrainTargetOpponentFamily(r *Registry) {
	for _, e := range etbDrainTargetOpponentEntries {
		e := e
		r.OnETB(e.cardName, func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			runEtbDrainTargetOpponent(gs, perm, e)
		})
		// This handler FULLY implements the printed self-ETB drain — declare
		// ownership so the engine's generic AST push is suppressed (Judge r63
		// double-fire gate). Without it the AST "loses N / gain N" trigger
		// ALSO resolves, draining 2N in real games (PROGRESSION r63:
		// Bloodborn Scoundrels, Skymarch Bloodletter, Vampire Sovereign,
		// Highway Robber, Dakmor Ghoul).
		r.OwnsETBTrigger(e.cardName)
	}
}

func runEtbDrainTargetOpponent(gs *gameengine.GameState, perm *gameengine.Permanent, e etbDrainEntry) {
	slug := "etb_drain_target_opponent_family:" + landFetchSlug(e.cardName)
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	mySeat := perm.Controller
	if mySeat < 0 || mySeat >= len(gs.Seats) {
		return
	}
	if e.amount <= 0 {
		return
	}

	target := pickLowestLifeOpponent(gs, mySeat)
	if target < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent", map[string]interface{}{
			"seat":   mySeat,
			"amount": e.amount,
		})
		return
	}

	gameengine.LoseLife(gs, target, e.amount, perm.Card.DisplayName())
	gameengine.GainLife(gs, mySeat, e.amount, perm.Card.DisplayName())
	_ = gs.CheckEnd()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   mySeat,
		"target": target,
		"amount": e.amount,
	})
}

// pickLowestLifeOpponent returns the seat index of the living opponent
// with the lowest Life total, or -1 if no living opponent exists. Ties
// resolve by lowest seat index (deterministic, matches Athreos / Belakor
// shape).
func pickLowestLifeOpponent(gs *gameengine.GameState, mySeat int) int {
	target := -1
	bestLife := 1 << 30
	for _, opp := range gs.Opponents(mySeat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	return target
}
