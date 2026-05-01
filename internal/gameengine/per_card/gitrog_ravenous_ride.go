package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGitrogRavenousRide wires The Gitrog, Ravenous Ride
// (Outlaws of Thunder Junction, verified Scryfall 2026-05-01).
//
// Oracle text:
//
//	{3}{B}{G}, 6/5 Legendary Creature — Frog Horror Mount
//	Trample, haste
//	Whenever The Gitrog deals combat damage to a player, you may
//	  sacrifice a creature that saddled it this turn. If you do, draw
//	  X cards, then put up to X land cards from your hand onto the
//	  battlefield tapped, where X is the sacrificed creature's power.
//	Saddle 1
//
// Implementation:
//   - Trample, haste, Saddle 1 — intrinsic AST keywords. Saddle is
//     resolved via gameengine.ActivateSaddle (CR §702.171), which now
//     records the saddler perms on `mount.SaddlersThisTurn` so this
//     handler can find them.
//   - "combat_damage_player": gates on (a) source seat == controller and
//     (b) source_card == "The Gitrog, Ravenous Ride". The handler picks
//     the highest-power surviving saddler from
//     `perm.SaddlersThisTurn`, sacrifices it, then draws X and puts up
//     to X lands from hand into play tapped, where X = sacrificed
//     creature's power. The "may" defaults to YES whenever a saddler
//     with power >= 1 is alive — value is monotonic.
func registerGitrogRavenousRide(r *Registry) {
	r.OnTrigger("The Gitrog, Ravenous Ride", "combat_damage_player", gitrogRideCombatDamage)
}

func gitrogRideCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gitrog_ravenous_ride_sacrifice_draw_lands"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if !strings.EqualFold(sourceName, perm.Card.DisplayName()) {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Pick the highest-power saddler still on the battlefield. Filter to
	// creatures the controller still controls (saddlers may have died /
	// been bounced since saddling).
	var victim *gameengine.Permanent
	bestPower := 0
	for _, s := range perm.SaddlersThisTurn {
		if s == nil || s.Card == nil {
			continue
		}
		if s.Controller != perm.Controller {
			continue
		}
		// Confirm s is still on the controller's battlefield.
		stillThere := false
		for _, p := range seat.Battlefield {
			if p == s {
				stillThere = true
				break
			}
		}
		if !stillThere {
			continue
		}
		pw := s.Power()
		if victim == nil || pw > bestPower {
			victim = s
			bestPower = pw
		}
	}
	if victim == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"choice": "decline",
			"reason": "no_living_saddler",
		})
		return
	}
	if bestPower <= 0 {
		// Sacrificing for X=0 would still allow drawing 0 / playing 0
		// lands but leaves us a creature down — decline the optional
		// trigger.
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"choice": "decline",
			"reason": "saddler_power_zero",
		})
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "gitrog_ravenous_ride")

	x := bestPower

	// Draw X cards.
	drawn := 0
	for i := 0; i < x; i++ {
		if len(seat.Library) == 0 {
			break
		}
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "gitrog_ride_draw")
		drawn++
	}

	// Put up to X land cards from hand onto the battlefield tapped.
	landsPlayed := 0
	for landsPlayed < x {
		// Re-scan hand each iteration since enterBattlefieldWithETB may
		// shift indices indirectly via ETB triggers.
		idx := -1
		for i, c := range seat.Hand {
			if c != nil && cardHasType(c, "land") {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		landCard := seat.Hand[idx]
		seat.Hand = append(seat.Hand[:idx], seat.Hand[idx+1:]...)
		enterBattlefieldWithETB(gs, perm.Controller, landCard, true)
		landsPlayed++
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"sacrificed":     victimName,
		"x":              x,
		"cards_drawn":    drawn,
		"lands_played":   landsPlayed,
	})
	_ = gs.CheckEnd()
}
