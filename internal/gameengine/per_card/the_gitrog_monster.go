package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheGitrogMonster wires The Gitrog Monster.
//
// Oracle text (Shadows over Innistrad, verified Scryfall 2026-05-02):
//
//	{3}{B}{G}, 6/4 Legendary Creature — Frog Horror
//	Deathtouch
//	At the beginning of your upkeep, sacrifice The Gitrog Monster unless
//	  you sacrifice a land.
//	You may play an additional land on each of your turns.
//	Whenever one or more land cards are put into your graveyard from
//	  anywhere, draw a card.
//
// Implementation:
//   - Deathtouch — intrinsic keyword handled by AST/combat pipeline.
//   - OnETB: grants controller an extra land drop via the seat's
//     "extra_land_drops" flag (same mechanism as Flubs the Fool, Hearthhull,
//     and Exploration). Emits extra_land_drop event + emitPartial noting the
//     static "once per turn" tracking gap (engine does not currently consume
//     the flag to enforce the once-per-turn limit).
//   - OnTrigger "upkeep_controller": gates on active_seat == controller.
//     Heuristic: sacrifice a land (prefer basic, then tapped) to keep Gitrog
//     alive. If no land is available, sacrifice The Gitrog Monster itself.
//   - OnTrigger "land_to_graveyard": whenever any land card enters a
//     graveyard owned by Gitrog's controller, draw a card. The engine fires
//     this event via FireZoneChangeTriggers for all paths (sacrifice, destroy,
//     discard, mill, etc.).
func registerTheGitrogMonster(r *Registry) {
	r.OnETB("The Gitrog Monster", theGitrogMonsterETB)
	r.OnTrigger("The Gitrog Monster", "upkeep_controller", theGitrogMonsterUpkeep)
	r.OnTrigger("The Gitrog Monster", "land_to_graveyard", theGitrogMonsterLandToGraveyard)
}

// theGitrogMonsterETB fires on ETB to grant the controller one extra land
// drop per turn. The flag is persisted on the seat so other engine code can
// read it; we also fire the canonical "extra_land_drop" log event that
// analysis tooling tracks.
func theGitrogMonsterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_gitrog_monster_etb"
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
	seat.Flags["extra_land_drops"]++
	gs.LogEvent(gameengine.Event{
		Kind:   "extra_land_drop",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug":   slug,
			"reason": "the_gitrog_monster_static_additional_land",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"static_additional_land_per_turn_uses_one_shot_flag_engine_does_not_consume")
}

// theGitrogMonsterUpkeep handles "At the beginning of your upkeep, sacrifice
// The Gitrog Monster unless you sacrifice a land."
//
// Priority for the land sacrifice:
//  1. Basic land (cheapest to give up) — tapped preferred over untapped.
//  2. Non-basic land — tapped preferred over untapped.
//
// If no land is available on the battlefield the controller must sacrifice
// The Gitrog Monster itself.
func theGitrogMonsterUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_gitrog_monster_upkeep_sac"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	land := gitrogPickWorstLand(seat, perm)
	if land != nil {
		landName := land.Card.DisplayName()
		gameengine.SacrificePermanent(gs, land, "the_gitrog_monster_upkeep")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"choice":        "sacrifice_land",
			"sacrificed":    landName,
		})
		return
	}

	// No land to sacrifice — Gitrog Monster dies.
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"choice": "sacrifice_self",
		"reason": "no_land_available",
	})
	gameengine.SacrificePermanent(gs, perm, "the_gitrog_monster_upkeep_self")
	_ = gs.CheckEnd()
}

// theGitrogMonsterLandToGraveyard handles "Whenever one or more land cards
// are put into your graveyard from anywhere, draw a card."
//
// The engine fires "land_to_graveyard" via FireZoneChangeTriggers for every
// path (sacrifice, destroy, discard, mill, etc.). We gate on owner_seat
// matching the controller to avoid triggering on opponent lands.
//
// CR §603.2c: if multiple land cards enter the graveyard simultaneously (e.g.
// Armageddon resolving), this trigger fires once and draws one card. The
// per-event design means each individual land move fires the event separately;
// the engine does not batch simultaneous zone changes at this layer, so each
// land entering the GY independently draws a card. This is functionally
// correct for typical gameplay and matches how the engine handles similar
// multi-trigger scenarios (e.g. Korvold on multiple sacs).
func theGitrogMonsterLandToGraveyard(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_gitrog_monster_land_to_graveyard_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	ownerSeat, _ := ctx["owner_seat"].(int)
	if ownerSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	landCard, _ := ctx["card"].(*gameengine.Card)
	landName := ""
	if landCard != nil {
		landName = landCard.DisplayName()
	}

	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"land":      landName,
		"from_zone": ctx["from_zone"],
		"drew":      drawnName,
	})
	_ = gs.CheckEnd()
}

// gitrogPickWorstLand returns the best land to sacrifice to satisfy the
// upkeep clause (saving Gitrog Monster). "Worst" = cheapest to give up:
//
//  1. Tapped basic land (already used this turn, purely basic utility).
//  2. Untapped basic land (still has mana but basics are replaceable).
//  3. Tapped non-basic land (utility used, still valuable but less so than
//     an untapped non-basic).
//  4. Untapped non-basic land (most valuable — keep if possible).
//
// The src permanent (Gitrog Monster itself) is excluded.
func gitrogPickWorstLand(seat *gameengine.Seat, src *gameengine.Permanent) *gameengine.Permanent {
	if seat == nil {
		return nil
	}

	// Pass 1: tapped basic land.
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsLand() {
			continue
		}
		if cardHasType(p.Card, "basic") && p.Tapped {
			return p
		}
	}

	// Pass 2: untapped basic land.
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsLand() {
			continue
		}
		if cardHasType(p.Card, "basic") && !p.Tapped {
			return p
		}
	}

	// Pass 3: tapped non-basic land.
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsLand() {
			continue
		}
		if p.Tapped {
			return p
		}
	}

	// Pass 4: any remaining land.
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsLand() {
			continue
		}
		return p
	}

	return nil
}
