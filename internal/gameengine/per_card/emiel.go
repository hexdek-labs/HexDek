package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEmiel wires Emiel the Blessed.
//
// Oracle text:
//
//	{3}: Exile another target creature you control, then return it to
//	the battlefield under its owner's control.
//	When a creature enters the battlefield under your control, if it
//	was returned by Emiel, you may pay {G}{W}. If you do, create a 3/3
//	green Beast creature token.
//
// Combo significance: with Peregrine Drake / Great Whale / Palinchron
// and 4 mana available, the {3} flicker re-fires the ETB and refunds
// enough mana to net positive — enabling infinite flicker / mana / ETB
// triggers (Drake nets +2 per cycle).
//
// Activation pick: prefer creatures registered with an ETB handler
// (HasETB) since flickering them is the high-value play. Fall back to
// the highest base power+toughness if nothing has a registered ETB.
func registerEmiel(r *Registry) {
	r.OnActivated("Emiel the Blessed", emielActivate)
}

func emielActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "emiel_flicker"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	target := pickEmielFlickerTarget(s, src, ctx)
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_flicker_target", nil)
		return
	}

	owner := target.Owner
	if target.Card != nil && target.Card.Owner >= 0 && target.Card.Owner < len(gs.Seats) {
		owner = target.Card.Owner
	}
	card := target.Card
	if !removePermanent(gs, target) {
		emitFail(gs, slug, src.Card.DisplayName(), "not_on_battlefield", nil)
		return
	}
	gs.UnregisterReplacementsForPermanent(target)
	gs.UnregisterContinuousEffectsForPermanent(target)
	gameengine.FireZoneChangeTriggers(gs, target, card, "battlefield", "exile")

	newPerm := createPermanent(gs, owner, card, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "flicker",
		Seat:   src.Controller,
		Target: owner,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_card": card.DisplayName(),
			"reason":      "emiel_the_blessed",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"flickered_card": card.DisplayName(),
		"owner":          owner,
	})
	if newPerm != nil {
		newPerm.Flags["emiel_returned"] = 1
		gameengine.RegisterReplacementsForPermanent(gs, newPerm)
		gameengine.InvokeETBHook(gs, newPerm)
		gameengine.FireObserverETBTriggers(gs, newPerm)
		gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
			"perm":            newPerm,
			"controller_seat": newPerm.Controller,
			"card":            newPerm.Card,
		})
		if !newPerm.IsLand() {
			gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
				"perm":            newPerm,
				"controller_seat": newPerm.Controller,
				"card":            newPerm.Card,
			})
		}

		// {G}{W} rider: spawn a 3/3 Beast token if the controller has
		// at least 2 mana. ManaPool is colorless-int in this engine,
		// so we approximate a two-color pip cost as 2 generic mana.
		if newPerm.Controller >= 0 && newPerm.Controller < len(gs.Seats) {
			ctrl := gs.Seats[newPerm.Controller]
			if ctrl != nil && ctrl.ManaPool >= 2 {
				ctrl.ManaPool -= 2
				gameengine.CreateCreatureToken(gs, newPerm.Controller, "Beast Token",
					[]string{"creature", "beast"}, 3, 3)
				emit(gs, "emiel_beast_token_rider", src.Card.DisplayName(), map[string]interface{}{
					"seat":     newPerm.Controller,
					"paid":     "GW_approx_2_generic",
					"token":    "3/3 Beast",
				})
			}
		}
	}
}

// pickEmielFlickerTarget chooses the best other creature on seat to
// flicker. Priority:
//  1. ctx["target_perm"] if supplied by the hat.
//  2. Another creature with a registered ETB handler (HasETB).
//  3. Highest base P+T fallback.
//
// Returns nil if no eligible creature exists.
func pickEmielFlickerTarget(s *gameengine.Seat, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if v, ok := ctx["target_perm"].(*gameengine.Permanent); ok && v != nil && v != src && v.IsCreature() {
		return v
	}
	var withETB *gameengine.Permanent
	var fallback *gameengine.Permanent
	bestStat := -1
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if withETB == nil && HasETB(p.Card.DisplayName()) {
			withETB = p
		}
		stat := p.Card.BasePower + p.Card.BaseToughness
		if stat > bestStat {
			bestStat = stat
			fallback = p
		}
	}
	if withETB != nil {
		return withETB
	}
	return fallback
}
