package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// batch_aj_win_the_game.go — five high-impact win-the-game / lose-the-
// game cards wired through the r60 §704.5c master-check pattern (#687):
//
//   1. Approach of the Second Sun — sorcery, cast-count + win on second
//      cast from hand; first cast goes to library 7th from top + 7
//      life + scry 1.
//   2. Felidar Sovereign — upkeep + life ≥ 40 → win.
//   3. Test of Endurance — upkeep + life ≥ 50 → win.
//   4. Mayael's Aria — upkeep + your-greatest-power ≥ 10 → win
//      (plus the +1/+1-counter and life-gain modes).
//   5. Triskaidekaphobia — upkeep + each player at exactly 13 life
//      loses (the loss-side cousin of the win pattern). Wires through
//      s.Lost+LossReason since §704.5 master-check handles the rest
//      via CheckEnd's §104.2a aggregator.
//
// All five cards share the §704.5c CheckEnd-driven win/loss path: per_card
// handlers flip Seat.Won (via emitWin) or Seat.Lost (directly), then
// CheckEnd resolves the §104.2c / §104.2a / §104.3b end-rule on the next
// SBA pass. None of these handlers call CheckEnd directly — the engine
// runs it after every state-changing operation.

// =============================================================================
// 1. Approach of the Second Sun
// =============================================================================

// registerApproachOfTheSecondSun — sorcery. Tracked cast count per seat
// lives in gs.Flags["approach_cast_count:<seat>"]. First cast: library-
// place + 7 life + scry 1. Second cast (or later) from HAND: win.
//
// Scryfall oracle (verified 2026-05-27):
//
//	If this spell was cast from your hand and you've cast another
//	spell named Approach of the Second Sun this game, you win the game.
//	Otherwise, put Approach of the Second Sun into its owner's library
//	seventh from the top, you gain 7 life, and you scry 1.
func registerApproachOfTheSecondSun(r *Registry) {
	r.OnResolve("Approach of the Second Sun", approachOfTheSecondSunResolve)
}

func approachOfTheSecondSunResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "approach_of_the_second_sun_resolve"
	if gs == nil || item == nil || item.Card == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	// CastZone defaults to "" which represents cast-from-hand per the
	// StackItem docstring; treat both "" and explicit "hand" as the
	// hand cast.
	castFromHand := item.CastZone == "" || item.CastZone == "hand"
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	key := fmt.Sprintf("approach_cast_count:%d", seat)
	prior := gs.Flags[key]

	if castFromHand && prior >= 1 {
		// Second-cast-from-hand win. Increment first so logs reflect
		// the final count.
		gs.Flags[key] = prior + 1
		emitWin(gs, seat, slug, item.Card.DisplayName(),
			"Approach of the Second Sun — second cast from hand")
		return
	}

	// First-cast path (or non-hand cast that doesn't trip the win):
	// put into owner's library 7th from top + 7 life + scry 1.
	owner := item.Card.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		owner = seat
	}
	os := gs.Seats[owner]
	if os == nil {
		os = s
	}
	card := item.Card
	// Insert at index 6 (0-indexed = 7th from top), clamping to tail
	// when library < 6 cards.
	insertIdx := 6
	if insertIdx > len(os.Library) {
		insertIdx = len(os.Library)
	}
	os.Library = append(os.Library[:insertIdx], append([]*gameengine.Card{card}, os.Library[insertIdx:]...)...)
	gameengine.GainLife(gs, seat, 7, "Approach of the Second Sun")
	// Scry 1 — peek top, keep or bottom. MVP: always keep (deterministic).
	// Future hat hook can decide; "always keep" matches the historical
	// Lab-Maniac-class scry default in this codebase.
	gs.Flags[key] = prior + 1
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat":             seat,
		"cast_from_zone":   item.CastZone,
		"prior_cast_count": prior,
		"new_cast_count":   gs.Flags[key],
		"placed_at_index":  insertIdx,
		"life_gained":      7,
	})
	// Don't pass to graveyard — the card went back to the library.
	item.Card = nil
}

// =============================================================================
// 2. Felidar Sovereign
// =============================================================================

// registerFelidarSovereign — Creature 4/6 Cat Beast. Lifelink + vigilance
// (granted via the AST keyword line; no per_card wiring needed for those).
//
//	At the beginning of your upkeep, if you have 40 or more life,
//	you win the game.
func registerFelidarSovereign(r *Registry) {
	r.OnTrigger("Felidar Sovereign", "upkeep_controller", felidarSovereignUpkeep)
}

func felidarSovereignUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "felidar_sovereign_upkeep_win_check"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || s.Lost {
		return
	}
	if s.Life >= 40 {
		emitWin(gs, perm.Controller, slug, perm.Card.DisplayName(),
			"Felidar Sovereign — life ≥ 40 at controller's upkeep")
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"life":      s.Life,
		"triggered": false,
	})
}

// =============================================================================
// 3. Test of Endurance
// =============================================================================

// registerTestOfEndurance — Enchantment.
//
//	At the beginning of your upkeep, if you have 50 or more life,
//	you win the game.
func registerTestOfEndurance(r *Registry) {
	r.OnTrigger("Test of Endurance", "upkeep_controller", testOfEnduranceUpkeep)
}

func testOfEnduranceUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "test_of_endurance_upkeep_win_check"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || s.Lost {
		return
	}
	if s.Life >= 50 {
		emitWin(gs, perm.Controller, slug, perm.Card.DisplayName(),
			"Test of Endurance — life ≥ 50 at controller's upkeep")
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"life":      s.Life,
		"triggered": false,
	})
}

// =============================================================================
// 4. Mayael's Aria
// =============================================================================

// registerMayaelsAria — Enchantment.
//
//	At the beginning of your upkeep, choose a creature you control with
//	the greatest power. You gain 4 life and put a +1/+1 counter on that
//	creature. Then if that creature's power is 10 or greater, you win
//	the game.
func registerMayaelsAria(r *Registry) {
	r.OnTrigger("Mayael's Aria", "upkeep_controller", mayaelsAriaUpkeep)
}

func mayaelsAriaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mayaels_aria_upkeep"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || s.Lost {
		return
	}
	// Pick the controller's highest-power creature. Ties: first seen.
	var pick *gameengine.Permanent
	pickPower := -(1 << 30)
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		pw := p.Power()
		if pw > pickPower {
			pickPower = pw
			pick = p
		}
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"reason": "no_creature_to_pick",
		})
		return
	}
	// Life gain + counter ALWAYS (the conditional is only on the win).
	gameengine.GainLife(gs, perm.Controller, 4, "Mayael's Aria")
	pick.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	postPower := pick.Power() // counter just added, re-read effective power
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"chosen_card": pick.Card.DisplayName(),
		"pre_power":   pickPower,
		"post_power":  postPower,
		"life_gained": 4,
	})
	if postPower >= 10 {
		emitWin(gs, perm.Controller, slug+"_win", perm.Card.DisplayName(),
			"Mayael's Aria — chosen creature power ≥ 10 at upkeep")
	}
}

// =============================================================================
// 5. Triskaidekaphobia — loss-side master-check
// =============================================================================

// registerTriskaidekaphobia — Enchantment.
//
//	At the beginning of your upkeep, choose one. You can't choose the
//	same mode two upkeeps in a row.
//	  • Each player loses 1 life.
//	  • Each player gains 1 life.
//	Whenever a player has exactly 13 life, that player loses the game.
//
// The "Whenever a player has exactly 13 life" clause fires AFTER the
// chosen mode resolves; per CR §603.6e it's a state-triggered ability
// that checks after each life change. For simplicity (and to align
// with the §704.5c master-check pattern), we evaluate the loss check
// once at upkeep AFTER the chosen mode applies, AND when any life-
// changing event happens via a separate observer hook.
//
// Mode policy: alternates each call. The hat is given a chance to
// pick a different mode via flag "triskaidekaphobia_last_mode:<seat>".
// MVP heuristic: choose the mode that pushes ANY opponent (and not
// the controller) toward 13 — favor lose-1-life when an opponent is
// at 14, gain-1-life when an opponent is at 12, else default to
// lose-1-life (faster game).
func registerTriskaidekaphobia(r *Registry) {
	r.OnTrigger("Triskaidekaphobia", "upkeep_controller", triskaidekaphobiaUpkeep)
}

func triskaidekaphobiaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "triskaidekaphobia_upkeep"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	lastModeKey := fmt.Sprintf("triskaidekaphobia_last_mode:%d", perm.Controller)
	lastMode := gs.Flags[lastModeKey]

	// Mode selection. 1 = lose 1, 2 = gain 1. Prefer the mode that
	// puts an opponent (NOT controller) at 13.
	mode := 1
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == perm.Controller {
			continue
		}
		if s.Life == 14 {
			mode = 1
			break
		}
		if s.Life == 12 {
			mode = 2
			break
		}
	}
	if mode == lastMode {
		// Forced alternation — flip to the other mode.
		if mode == 1 {
			mode = 2
		} else {
			mode = 1
		}
	}

	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if mode == 1 {
			gameengine.LoseLife(gs, i, 1, "Triskaidekaphobia")
		} else {
			gameengine.GainLife(gs, i, 1, "Triskaidekaphobia")
		}
	}
	gs.Flags[lastModeKey] = mode

	// State-triggered loss check — any seat at exactly 13 life loses.
	losers := []int{}
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if s.Life == 13 {
			s.Lost = true
			s.LossReason = "Triskaidekaphobia — exactly 13 life"
			losers = append(losers, i)
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"mode":   mode,
		"losers": losers,
	})
}
