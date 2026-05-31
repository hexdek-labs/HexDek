package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Wort, the Raidmother — {3}{R/G}{R/G} 3/3 Legendary Creature — Goblin
// Warrior.
//
//   When Wort enters, create two 1/1 red and green Goblin Warrior
//   creature tokens.
//   Each red or green instant or sorcery spell you cast has conspire.
//   (As you cast the spell, you may tap two untapped creatures you
//   control that share a color with it. When you do, copy it and you
//   may choose new targets for the copy.)
//
// Implementation:
//   - OnETB: spawn two 1/1 Goblin Warrior tokens with R+G pips via
//     CreateCreatureToken.
//   - Conspire grant: complex copy-on-cast mechanic that requires
//     engine-level alt-cost / copy-spell plumbing. emitPartial flags
//     the full mechanic as a future engine PR.

func init() {
	registerWortTheRaidmother(Global())
	AddResetHook(registerWortTheRaidmother)
}

func registerWortTheRaidmother(r *Registry) {
	r.OnETB("Wort, the Raidmother", wortETB)
}

func wortETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "wort_etb_two_goblin_warriors"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	for i := 0; i < 2; i++ {
		gameengine.CreateCreatureToken(gs, seat, "Goblin Warrior Token",
			[]string{"creature", "goblin", "warrior", "pip:R", "pip:G"}, 1, 1)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"tokens": 2,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"conspire_grant_for_red_or_green_instant_sorcery_needs_engine_alt_cost_and_copy_plumbing")
}
