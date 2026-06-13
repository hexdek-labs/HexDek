package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// champion_of_lambholt.go — per_card handler for Champion of Lambholt.
//
// Oracle text (Creature — Human Warrior, {1}{G}{G}):
//
//	Creatures with power less than this creature's power can't block
//	creatures you control.
//	Whenever another creature you control enters, put a +1/+1 counter on
//	this creature.
//
// Why this needs a per_card handler: the card's ability nodes parsed to
// inert scaffold (no observable engine effect), so Champion never grew —
// it sat as a vanilla 1/1 and its signature snowball never happened.
// Verified inert: no registered handler for "Champion of Lambholt".
//
// Implemented here (the growth engine — the game-relevant half):
// on each other creature you control entering, add a +1/+1 counter.
//
// Deliberately deferred (logged): the "can't be blocked by creatures
// with power less than Champion's" evasion static, which needs a combat
// blocking-legality predicate keyed off Champion's live power. The
// counter accumulation is the dominant, snowballing effect; the evasion
// is conditional and only matters in combat math.

func init() {
	registerChampionOfLambholt(Global())
	AddResetHook(registerChampionOfLambholt)
}

func registerChampionOfLambholt(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Champion of Lambholt", "permanent_etb", championOfLambholtETB)
}

func championOfLambholtETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "champion_of_lambholt"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	// "another creature you control" — same controller, is a creature,
	// and not Champion herself.
	if entering == perm {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	if !entering.IsCreature() {
		return
	}

	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"entering": entering.Card.DisplayName(),
		"counters": perm.Counters["+1/+1"],
	})
}
