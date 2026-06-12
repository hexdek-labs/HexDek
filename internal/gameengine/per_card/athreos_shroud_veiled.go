package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAthreosShroudVeiled wires Athreos, Shroud-Veiled.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Indestructible
//	As long as your devotion to white and black is less than seven,
//	  Athreos isn't a creature.
//	At the beginning of your end step, put a coin counter on another
//	  target creature.
//	Whenever a creature with a coin counter on it dies or is put into
//	  exile, return that card to the battlefield under your control.
//
// Implementation:
//   - "end_step_controller": at our end step, pick a juicy target —
//     prefer the highest-power opposing creature without a coin counter
//     yet (so we can steal it on death); fall back to our own biggest
//     creature. Add coin counter via AddCounter.
//   - "creature_dies": if the dying perm had a coin counter and the card
//     is non-token, route the card from graveyard onto our battlefield
//     under our control. r54 fix: the prior handler did
//     `MoveCard(card, owner_seat, gy→bf)` followed by
//     `createPermanent(athreos_seat, card)`. MoveCard's "battlefield"
//     arm wraps the card in a fresh Permanent on the OWNER's seat, and
//     createPermanent's dedup guard only checks the TARGET seat — so
//     when the dying creature's owner ≠ Athreos's controller (the
//     normal steal case), we ended up with two Permanents wrapping the
//     same *Card pointer on two seats' battlefield slices. Surfaced by
//     Loki r53 as the game-3107 "Eager Cadet appears in both seat 0
//     and seat 3 battlefield" CardIdentity violation (206 hits in 5K).
//     Fixed by using `enterBattlefieldWithETB` directly — createPermanent
//     sweeps the card from owner's private zones (incl. graveyard) and
//     wraps it in a single Permanent on Athreos's controller's seat;
//     enterBattlefieldWithETB then fires the ETB cascade.
//   - The exile branch is approximated via emitPartial — the engine
//     doesn't yet expose a per-card "creature_exiled" pipeline.
//   - Devotion/indestructible handled at static-effect level — we flag
//     the gap on ETB.
func registerAthreosShroudVeiled(r *Registry) {
	r.OnETB("Athreos, Shroud-Veiled", athreosShroudETB)
	r.OnTrigger("Athreos, Shroud-Veiled", "end_step", athreosShroudEndStep)
	r.OnTrigger("Athreos, Shroud-Veiled", "creature_dies", athreosShroudDies)
}

func athreosShroudETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, "athreos_shroud_veiled_static", perm.Card.DisplayName(),
		"devotion_isnt_a_creature_clause_not_enforced")
	emitPartial(gs, "athreos_shroud_veiled_exile_branch", perm.Card.DisplayName(),
		"coin_counter_exile_return_branch_not_modeled")
}

func athreosShroudEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "athreos_shroud_veiled_coin_counter"
	if gs == nil || perm == nil {
		return
	}

	var bestOpp *gameengine.Permanent
	bestOppPow := -1
	var bestOwn *gameengine.Permanent
	bestOwnPow := -1
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p == perm || !p.IsCreature() {
				continue
			}
			if p.Counters != nil && p.Counters["coin"] > 0 {
				continue
			}
			pw := p.Power()
			if i != perm.Controller {
				if pw > bestOppPow {
					bestOppPow = pw
					bestOpp = p
				}
			} else {
				if pw > bestOwnPow {
					bestOwnPow = pw
					bestOwn = p
				}
			}
		}
	}

	target := bestOpp
	if target == nil {
		target = bestOwn
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_creature_target", nil)
		return
	}

	target.AddCounter("coin", 1)
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"target":       target.Card.DisplayName(),
		"target_seat":  target.Controller,
		"target_power": target.Power(),
	})
}

func athreosShroudDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "athreos_shroud_veiled_steal"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	if dyingCard == nil || dyingPerm == nil {
		return
	}
	if dyingPerm.Counters == nil || dyingPerm.Counters["coin"] <= 0 {
		return
	}
	if dyingPerm.IsToken() {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_ceases_to_exist", map[string]interface{}{
			"creature": dyingCard.DisplayName(),
		})
		return
	}
	owner := dyingCard.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		return
	}
	// CR §800.4a (r63, seed-42 game 283): an eliminated player's cards
	// left the game with them — the dead seat's graveyard keeps the
	// pointers (and ceased InstanceIDs) for forensic clarity, so without
	// this check Athreos "returns" a card that no longer exists and the
	// census flags a ceased ID on a live battlefield. createPermanent
	// also refuses at the chokepoint; this keeps the trigger from
	// claiming a dead card at all.
	if gs.Seats[owner] != nil && gs.Seats[owner].LeftGame {
		emitFail(gs, slug, perm.Card.DisplayName(), "owner_left_game", map[string]interface{}{
			"creature": dyingCard.DisplayName(),
		})
		return
	}
	// Defensive: validate the card is still in the owner's graveyard
	// before claiming it. When TWO Athreos, Shroud-Veiled handlers fire
	// on the same creature_dies event (both controllers placed coin
	// counters on the dying creature), the first handler pulls the
	// *Card out of owner's graveyard via createPermanent's zone sweep
	// and places it on its controller's battlefield. The second handler
	// then races: the *Card is no longer in graveyard, but
	// createPermanent's dedup only checks the TARGET seat's
	// battlefield — so the race-losing handler still wraps the already-
	// stolen *Card in a fresh Permanent on its own seat. Result: same
	// *Card pointer on two seats' battlefields. Loki r60 deep-stress /
	// seed 2024 game 2798: Woodland Liege + Athreos appearing on seats
	// 2 and 3 simultaneously, 24 CardIdentity hits.
	//
	// Same race surface as the Adric / Oketra / Gisa / The Reaper §704.6d
	// fixes earlier this r60 cycle. Match the established Gisa pattern:
	// scan owner's graveyard for the dyingCard before delegating to
	// enterBattlefieldWithETB. If absent, a sibling Athreos won the
	// race — no-op.
	stillInGraveyard := false
	if gs.Seats[owner] != nil {
		for _, c := range gs.Seats[owner].Graveyard {
			if c == dyingCard {
				stillInGraveyard = true
				break
			}
		}
	}
	if !stillInGraveyard {
		emitFail(gs, slug, perm.Card.DisplayName(), "card_already_stolen", map[string]interface{}{
			"creature":   dyingCard.DisplayName(),
			"from_owner": owner,
		})
		return
	}
	// Single-step zone change: createPermanent (via enterBattlefieldWithETB)
	// sweeps the card from the owner's private zones — including the
	// graveyard — and wraps it in exactly one Permanent on Athreos's
	// controller's seat. No separate MoveCard call (that anti-pattern
	// left the card on the owner's battlefield too).
	stolen := enterBattlefieldWithETB(gs, perm.Controller, dyingCard, false)
	if stolen == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "etb_failed", map[string]interface{}{
			"creature": dyingCard.DisplayName(),
		})
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"creature":   dyingCard.DisplayName(),
		"from_owner": owner,
	})
}
