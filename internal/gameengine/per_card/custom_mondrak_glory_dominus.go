package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMondrakGloryDominus wires Mondrak, Glory Dominus.
//
// Oracle text (Scryfall, verified — Phyrexia: All Will Be One):
//
//	If one or more tokens would be created under your control, twice
//	that many of those tokens are created instead.
//	{1}{W/P}{W/P}, Sacrifice two other artifacts and/or creatures:
//	Put an indestructible counter on Mondrak. ({W/P} can be paid with
//	either {W} or 2 life.)
//
// Implementation:
//   - Token doubling static: per-seat flag bumped at ETB; the engine
//     CreateToken path reads "token_doubler_count" and doubles
//     accordingly.
//   - Activated {1}{W/P}{W/P}, Sacrifice two: the prior handler called
//     MoveCard(battlefield→exile) which is a no-op for Permanent
//     cleanup (removeCardFromZone explicitly does nothing for
//     battlefield — callers must remove the Permanent themselves).
//     Result was a CardIdentity invariant violation: the same *Card
//     pointer wound up in both seat.Battlefield[i].Card AND seat.Exile
//     — surfaced by Loki r48-deep / r50 as the game-59 / Avatar
//     Enthusiasts leak (1352 hits in r48, 352 in r50).
//
//     Fix: use SacrificePermanent (which does the full battlefield exit
//     — drops the Permanent, runs §704.5 SBAs, fires LTB triggers, and
//     routes the Card to the graveyard per CR §701.17). Also corrects
//     three additional bugs versus the printed oracle:
//       (a) "Exile two creatures" → "Sacrifice two artifacts and/or
//           creatures" (the printed cost is sacrifice, and the filter
//           covers both card types — wider sac fodder pool).
//       (b) "Put two indestructible counters" → "Put an indestructible
//           counter" (singular per printed text).
//       (c) Mana gate {2}{W}{W} = 4 → {1}{W/P}{W/P} = 3 generic-
//           equivalent (Phyrexian symbols payable with mana or 2 life;
//           the per_card layer doesn't model life-payment so we keep
//           the mana-pool path and gate at 3 instead of 4).
func registerMondrakGloryDominus(r *Registry) {
	r.OnETB("Mondrak, Glory Dominus", mondrakSetTokenDoubler)
	r.OnActivated("Mondrak, Glory Dominus", mondrakIndestructibleActivate)
}

func mondrakSetTokenDoubler(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mondrak_token_doubler_flag"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["token_doubler_count"]++
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"doublers_active": seat.Flags["token_doubler_count"],
	})
}

func mondrakIndestructibleActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mondrak_indestructible_activate"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	const manaCost = 3 // {1}{W/P}{W/P} → 3 generic-equivalent
	if seat.ManaPool < manaCost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
			"mana_cost": manaCost,
		})
		return
	}
	// Pick two other artifacts and/or creatures we control (smallest
	// PT first for creatures; any artifact). Source itself is excluded.
	var sac1, sac2 *gameengine.Permanent
	pickEligible := func(p *gameengine.Permanent) bool {
		if p == nil || p == src || p.Card == nil {
			return false
		}
		return p.IsCreature() || cardHasType(p.Card, "artifact")
	}
	for _, p := range seat.Battlefield {
		if !pickEligible(p) {
			continue
		}
		if sac1 == nil {
			sac1 = p
			continue
		}
		if sac2 == nil {
			sac2 = p
			break
		}
	}
	if sac1 == nil || sac2 == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "fewer_than_2_eligible_sacrifices", nil)
		return
	}
	seat.ManaPool -= manaCost
	gameengine.SyncManaAfterSpend(seat)
	sac1Name := sac1.Card.DisplayName()
	sac2Name := sac2.Card.DisplayName()
	gameengine.SacrificePermanent(gs, sac1, "mondrak_sac_cost")
	gameengine.SacrificePermanent(gs, sac2, "mondrak_sac_cost")
	src.AddCounter("indestructible", 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           src.Controller,
		"sacrificed":     []string{sac1Name, sac2Name},
		"indestructible": src.Counters["indestructible"],
	})
}
