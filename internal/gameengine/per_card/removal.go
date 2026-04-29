package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Path to Exile
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   Exile target creature. Its controller may search their library for
//   a basic land card, put that card onto the battlefield tapped, then
//   shuffle.
//
// W instant. Premium white removal — exiles instead of destroying.

func registerPathToExile(r *Registry) {
	r.OnResolve("Path to Exile", pathToExileResolve)
}

func pathToExileResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "path_to_exile"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Find target creature (opponent's best creature by power).
	var target *gameengine.Permanent
	var targetSeat int
	for _, opp := range gs.Opponents(seat) {
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if target == nil || p.Power() > target.Power() {
				target = p
				targetSeat = opp
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, "Path to Exile", "no_valid_target", nil)
		return
	}

	targetName := target.Card.DisplayName()
	gameengine.ExilePermanent(gs, target, nil)

	// Controller of exiled creature may search for a basic land.
	ts := gs.Seats[targetSeat]
	for i, c := range ts.Library {
		if c == nil {
			continue
		}
		if landMatchesFetchTypes(c, []string{"plains", "island", "swamp", "mountain", "forest"}) {
			land := ts.Library[i]
			ts.Library = append(ts.Library[:i], ts.Library[i+1:]...)
			enterBattlefieldWithETB(gs, targetSeat, land, true)
			shuffleLibraryPerCard(gs, targetSeat)
			break
		}
	}

	emit(gs, slug, "Path to Exile", map[string]interface{}{
		"seat":          seat,
		"exiled":        targetName,
		"target_seat":   targetSeat,
	})
}

// -----------------------------------------------------------------------------
// Swords to Plowshares
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   Exile target creature. Its controller gains life equal to its power.
//
// W instant. The original premium removal spell.

func registerSwordsToPlowshares(r *Registry) {
	r.OnResolve("Swords to Plowshares", swordsToPlowsharesResolve)
}

func swordsToPlowsharesResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "swords_to_plowshares"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Find target creature.
	var target *gameengine.Permanent
	var targetSeat int
	for _, opp := range gs.Opponents(seat) {
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if target == nil || p.Power() > target.Power() {
				target = p
				targetSeat = opp
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, "Swords to Plowshares", "no_valid_target", nil)
		return
	}

	targetName := target.Card.DisplayName()
	power := target.Power()

	gameengine.ExilePermanent(gs, target, nil)

	// Controller gains life equal to the creature's power.
	if power > 0 {
		gameengine.GainLife(gs, targetSeat, power, "Swords to Plowshares")
		gs.LogEvent(gameengine.Event{
			Kind:   "gain_life",
			Seat:   targetSeat,
			Target: targetSeat,
			Source: "Swords to Plowshares",
			Amount: power,
			Details: map[string]interface{}{
				"reason": "swords_to_plowshares",
			},
		})
	}

	emit(gs, slug, "Swords to Plowshares", map[string]interface{}{
		"seat":       seat,
		"exiled":     targetName,
		"life_gain":  power,
		"target_seat": targetSeat,
	})
}

// -----------------------------------------------------------------------------
// Cyclonic Rift
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   Return target nonland permanent you don't control to its owner's
//   hand.
//   Overload {6}{U} (You may cast this spell for its overload cost.
//   If you do, change "target" to "each".)
//
// 1U instant. One of the most powerful board wipes in commander when
// overloaded — bounces ALL opponents' nonland permanents.
//
// MVP: we determine mode (single target vs overload) from the mana
// paid. If the controller spent 7+ mana, we treat it as overload.
// Otherwise, single-target bounce.

func registerCyclonicRift(r *Registry) {
	r.OnResolve("Cyclonic Rift", cyclonicRiftResolve)
}

func cyclonicRiftResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "cyclonic_rift"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Check for overload hint. We use a flag on the card or the gs.Flags
	// to indicate overload was paid. MVP heuristic: if the game is past
	// turn 3, assume overload (cEDH plays Rift for overload most of the time).
	overload := gs.Turn >= 3

	// Also check for explicit overload flag from the casting context.
	if item.Card != nil {
		for _, t := range item.Card.Types {
			if t == "overloaded" {
				overload = true
			}
		}
	}

	if overload {
		// Overload: bounce all opponents' nonland permanents.
		bounced := 0
		for _, opp := range gs.Opponents(seat) {
			// Collect permanents to bounce (snapshot to avoid mutation during iteration).
			var toBounce []*gameengine.Permanent
			for _, p := range gs.Seats[opp].Battlefield {
				if p == nil || p.IsLand() {
					continue
				}
				toBounce = append(toBounce, p)
			}
			for _, p := range toBounce {
				gameengine.BouncePermanent(gs, p, nil, "hand")
				bounced++
			}
		}
		emit(gs, slug, "Cyclonic Rift", map[string]interface{}{
			"seat":     seat,
			"mode":     "overload",
			"bounced":  bounced,
		})
	} else {
		// Single target: bounce one nonland permanent you don't control.
		var target *gameengine.Permanent
		for _, opp := range gs.Opponents(seat) {
			for _, p := range gs.Seats[opp].Battlefield {
				if p == nil || p.IsLand() {
					continue
				}
				if target == nil {
					target = p
				}
			}
		}
		if target == nil {
			emitFail(gs, slug, "Cyclonic Rift", "no_valid_target", nil)
			return
		}
		bounced := target.Card.DisplayName()
		gameengine.BouncePermanent(gs, target, nil, "hand")
		emit(gs, slug, "Cyclonic Rift", map[string]interface{}{
			"seat":     seat,
			"mode":     "single",
			"bounced":  bounced,
		})
	}
}
