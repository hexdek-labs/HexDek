package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKaradorGhostChieftainCustom implements Karador's once-per-turn
// "cast a creature spell from your graveyard" activated effect. The
// auto-generated static stub leaves both lines as no-ops.
//
// Oracle text:
//
//	This spell costs {1} less to cast for each creature card in your
//	graveyard.
//	Once during each of your turns, you may cast a creature spell from
//	your graveyard.
//
// The cost reduction on Karador itself is wired through cost-modifier
// scanning at cast time (engine territory). What this handler covers is
// the activated/triggered "cast from graveyard" privilege:
//
//   - Tracks usage via perm.Flags["karador_used_this_turn"].
//   - Resets on the controller's untap step via a recurring delayed
//     trigger registered at ETB.
//   - When invoked via the activated hook, picks the best (highest CMC)
//     creature card from Karador's controller's graveyard and moves it
//     to the battlefield. The "cast" wording is approximated as a
//     direct ETB — we don't yet have a gameengine entry point that
//     casts a spell from a non-hand zone, and reanimate semantics give
//     the same board impact.
func registerKaradorGhostChieftainCustom(r *Registry) {
	r.OnETB("Karador, Ghost Chieftain", karadorETB)
	r.OnActivated("Karador, Ghost Chieftain", karadorCastFromGY)
}

func karadorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["karador_used_this_turn"] = 0
	scheduleKaradorReset(gs, perm)
}

func scheduleKaradorReset(gs *gameengine.GameState, perm *gameengine.Permanent) {
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_upkeep",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if perm.Flags != nil {
				perm.Flags["karador_used_this_turn"] = 0
			}
			// Re-arm next turn while Karador is still on the battlefield.
			seat := gs.Seats[perm.Controller]
			if seat == nil {
				return
			}
			for _, p := range seat.Battlefield {
				if p == perm {
					scheduleKaradorReset(gs, perm)
					return
				}
			}
		},
	})
}

func karadorCastFromGY(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "karador_cast_from_graveyard"
	if gs == nil || src == nil {
		return
	}
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	if src.Flags["karador_used_this_turn"] > 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "already_used_this_turn", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if cmc := cardCMC(c); cmc > bestCMC {
			best = c
			bestCMC = cmc
		}
	}
	if best == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_in_graveyard", nil)
		return
	}
	// enterBattlefieldWithETB → createPermanent sweeps `best` from every
	// private zone (graveyard included); the manual graveyard splice
	// pre-r60 was redundant (Wave 2 multi-step migration — same shape as
	// the Cluster 2 cleanups in PR #924).
	enterBattlefieldWithETB(gs, src.Controller, best, false)
	src.Flags["karador_used_this_turn"] = 1
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
		"cast": best.DisplayName(),
		"cmc":  bestCMC,
	})
}
