package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGerrardWeatherlightHero wires Gerrard, Weatherlight Hero.
//
// Oracle text (Scryfall, verified — Dominaria United):
//
//	First strike
//	When Gerrard dies, exile it and return to the battlefield all
//	artifact and creature cards in your graveyard that were put there
//	from the battlefield this turn.
//
// Implementation (r53 fix for Loki r50/r51/r52 game-2432 CardIdentity
// leak):
//
//   - Approximates "from battlefield this turn" by reanimating every
//     artifact / creature card in the controller's graveyard at trigger
//     resolution time. A precise per-card "left bf this turn" timestamp
//     would need a side channel; the broader population still captures
//     the intent for chaos fuzz.
//
//   - Self-exile (r53 fix): the prior handler unconditionally called
//     `moveCardBetweenZones(perm.Card, "graveyard", "exile", ...)`. When
//     Gerrard is a COMMANDER and his die fires, the actual card may not
//     be in the graveyard by the time the trigger resolves:
//       (a) CR §903.9b commander-zone redirect on death — owner chose
//           to put Gerrard in the command zone instead of the
//           graveyard. He's already gone from the graveyard.
//       (b) SBA §704.6d swept Gerrard out of the graveyard/exile and
//           into the command zone, then the owner recast him onto the
//           battlefield. By the time the queued "dies" trigger fans
//           out to per_card hooks, Gerrard's *Card is wrapped by a
//           fresh Permanent on the battlefield.
//
//     The pre-r53 `moveCardBetweenZones(graveyard→exile)` was a no-op
//     for source removal in both cases (Card not in graveyard) but the
//     destination append still ran — leaving the *Card pointer in
//     `seat.Exile` while the same pointer was already in CommandZone
//     (case a) or wrapped by a battlefield Permanent (case b). The
//     §704.6d SBA then routed the stray exile entry to CommandZone,
//     surfacing the "appears in both seat 2 command_zone and seat 2
//     battlefield" CardIdentity invariant violation (Loki r48 — 1352
//     hits on the old Avatar Enthusiasts shape, r50/r51 — 318+ hits on
//     Gerrard once the Mondrak fix unmasked this cluster).
//
//     Fix: verify the *Card is actually in `seat.Graveyard` before
//     calling the exile move. If it's already elsewhere (CZ, BF wrap,
//     other-seat library via tuck, etc.), the "exile it" half of the
//     printed trigger is a no-op per CR §400.7 ("an object that moves
//     from one zone to another becomes a new object with no memory of
//     its previous existence"; the trigger's "it" refers to whichever
//     object is now in the graveyard, which is none).
//
//   - Recur loop is fine as-is — it iterates `seat.Graveyard` directly,
//     so stale references can't drive a phantom move.
func registerGerrardWeatherlightHero(r *Registry) {
	r.OnTrigger("Gerrard, Weatherlight Hero", "dies", gerrardDies)
}

func gerrardDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gerrard_dies_mass_recur"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	// "exile it" — only if Gerrard's *Card is still in the graveyard at
	// trigger resolution. See header comment for why a stale trigger
	// can reach here with the *Card already in CZ / on the battlefield /
	// otherwise relocated.
	exiledSelf := false
	for _, c := range seat.Graveyard {
		if c == perm.Card {
			moveCardBetweenZones(gs, perm.Controller, perm.Card, "graveyard", "exile", "gerrard_self_exile")
			exiledSelf = true
			break
		}
	}

	// Reanimate each artifact / creature card in graveyard. Snapshot
	// the slice before iterating — MoveCard's graveyard→battlefield
	// path mutates seat.Graveyard in place via removeFromSlice, which
	// shifts the underlying array and would cause range-based iteration
	// to skip the element shifted into the just-vacated slot.
	snap := append([]*gameengine.Card(nil), seat.Graveyard...)
	returned := 0
	for _, c := range snap {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") || cardHasType(c, "artifact") {
			gameengine.MoveCard(gs, c, perm.Controller, "graveyard", "battlefield", "gerrard_recur")
			returned++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"exiled_self":  exiledSelf,
		"returned":     returned,
	})
	if !exiledSelf {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"self_exile_skipped_card_not_in_graveyard_at_trigger_resolution")
	}
}
