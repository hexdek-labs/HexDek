package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheDawningArchaic wires The Dawning Archaic.
//
// Oracle text:
//
//	{10}
//	Legendary Creature — Avatar
//	This spell costs {1} less to cast for each instant and sorcery card
//	  in your graveyard.
//	Reach
//	Whenever The Dawning Archaic attacks, you may cast target instant or
//	  sorcery card from your graveyard without paying its mana cost. If
//	  that spell would be put into your graveyard, exile it instead.
//
// Implementation (R49 stub port — batch A):
//   - Reach via AST.
//   - Self-cast cost reduction "costs {1} less for each instant and
//     sorcery card in your graveyard" is wired in cost_modifiers.go
//     under "The Dawning Archaic" self-cast.
//   - Attack trigger approximation: "may cast target instant or sorcery
//     card from your graveyard without paying its mana cost. If that
//     spell would be put into your graveyard, exile it instead." The
//     full spell-replay path (push StackItem with alt-cost zero and
//     "exile if would die" replacement) isn't surfaced to per_card.
//     We approximate the value swing by returning the highest-CMC
//     instant or sorcery from the controller's graveyard to hand —
//     captures the recursion piece, omits the cast-this-turn timing
//     and exile-instead replacement. emitPartial flags the gap.
func registerTheDawningArchaic(r *Registry) {
	r.OnETB("The Dawning Archaic", theDawningArchaicETB)
	r.OnTrigger("The Dawning Archaic", "creature_attacks", theDawningArchaicAttack)
}

func theDawningArchaicETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "the_dawning_archaic_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func theDawningArchaicAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_dawning_archaic_attack_recur"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm == nil || atkPerm != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var pick *gameengine.Card
	pickCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > pickCMC {
			pickCMC = cmc
			pick = c
		}
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"found":  false,
			"reason": "no_is_in_graveyard",
		})
		return
	}
	gameengine.MoveCard(gs, pick, perm.Controller, "graveyard", "hand", slug)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"recurred":  pick.DisplayName(),
		"cmc":       pickCMC,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"cast_without_mana_cost_and_exile_if_would_die_approximated_as_to_hand")
}
