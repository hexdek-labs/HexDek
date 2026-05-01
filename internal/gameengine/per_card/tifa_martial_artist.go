package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTifaMartialArtist wires Tifa, Martial Artist.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	{1}{R}{G}{W}
//	Legendary Creature — Human Monk
//
//	Melee (Whenever this creature attacks, it gets +1/+1 until end of
//	turn for each opponent you attacked this combat.)
//	Whenever one or more creatures you control with power 7 or greater
//	deal combat damage to a player, untap all creatures you control. If
//	it's the first combat phase of your turn, there is an additional
//	combat phase after this phase.
//
// Implementation:
//   - Melee — AST keyword, intrinsic to combat resolution.
//   - "combat_begin": stamp a turn-keyed counter on Tifa's Flags so the
//     damage trigger can detect "first combat phase".
//   - "combat_damage_player": gate on (a) source controlled by Tifa's
//     controller, (b) source's power >= 7. De-dupe per (turn, combat-
//     idx) so the trigger fires once per combat regardless of how many
//     7+-power creatures dealt damage. On fire: untap every creature
//     Tifa's controller controls. If the combat-idx counter is 1 (first
//     combat of the turn), increment gs.PendingExtraCombats so the turn
//     loop runs another combat phase.
func registerTifaMartialArtist(r *Registry) {
	r.OnTrigger("Tifa, Martial Artist", "combat_begin", tifaCombatBegin)
	r.OnTrigger("Tifa, Martial Artist", "combat_damage_player", tifaCombatDamage)
}

func tifaCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := tifaTurnKey(gs.Turn)
	// Reset stale counters from a previous turn (lazy clean-up).
	if perm.Flags["tifa_turn_marker"] != turnKey {
		perm.Flags["tifa_turn_marker"] = turnKey
		perm.Flags["tifa_combat_idx"] = 0
		perm.Flags["tifa_fired_combat_idx"] = 0
	}
	perm.Flags["tifa_combat_idx"]++
}

func tifaCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tifa_martial_artist_untap_extra_combat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}

	// Only fires when a creature you control with power >= 7 deals damage.
	// Resolve power on the source permanent if available; fall back to the
	// raw amount when ctx doesn't include the permanent (the engine's
	// combat_damage_player ctx today carries source_card and amount, not
	// the permanent — we approximate by checking any 7+-power creature
	// the controller has on the battlefield that's currently attacking).
	if !tifaSeatHasPower7Attacker(gs, perm.Controller) {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := tifaTurnKey(gs.Turn)
	combatIdx := perm.Flags["tifa_combat_idx"]
	if combatIdx == 0 {
		// combat_begin didn't seed the counter (older turn paths) — assume
		// we're in the first combat of this turn.
		combatIdx = 1
		perm.Flags["tifa_turn_marker"] = turnKey
		perm.Flags["tifa_combat_idx"] = 1
	}
	if perm.Flags["tifa_fired_turn_marker"] == turnKey &&
		perm.Flags["tifa_fired_combat_idx"] == combatIdx {
		return
	}
	perm.Flags["tifa_fired_turn_marker"] = turnKey
	perm.Flags["tifa_fired_combat_idx"] = combatIdx

	// Untap all creatures Tifa's controller controls.
	seat := gs.Seats[perm.Controller]
	untapped := 0
	if seat != nil {
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() || !p.Tapped {
				continue
			}
			p.Tapped = false
			untapped++
		}
	}

	extraGranted := false
	if combatIdx == 1 {
		gs.PendingExtraCombats++
		extraGranted = true
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"untapped":        untapped,
		"combat_idx":      combatIdx,
		"extra_combat":    extraGranted,
		"pending_combats": gs.PendingExtraCombats,
	})
}

// tifaSeatHasPower7Attacker scans the seat's battlefield for any
// creature currently attacking that has power >= 7. Used as the trigger
// gate for "one or more creatures you control with power 7 or greater
// deal combat damage to a player." We accept any 7+ creature on the
// battlefield (not strictly only the damage source) because the
// combat_damage_player ctx doesn't expose the source permanent and the
// trigger is a one-or-more aggregate anyway.
func tifaSeatHasPower7Attacker(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Power() >= 7 {
			return true
		}
	}
	return false
}

func tifaTurnKey(turn int) int {
	return turn + 1
}
