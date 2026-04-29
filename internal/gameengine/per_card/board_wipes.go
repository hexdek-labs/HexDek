package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Board wipes — mass creature/permanent removal.
//
// These are essential Commander cards that clear the board. Several bypass
// indestructible (Toxic Deluge via -X/-X, Farewell via exile).
// ============================================================================

// --- Wrath of God ---
//
// Oracle text:
//   Destroy all creatures. They can't be regenerated.
//
// 2WW sorcery. The canonical board wipe.
func registerWrathOfGod(r *Registry) {
	r.OnResolve("Wrath of God", wrathOfGodResolve)
}

func wrathOfGodResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "wrath_of_god"
	if gs == nil || item == nil {
		return
	}
	destroyed := destroyAllCreatures(gs)
	emit(gs, slug, "Wrath of God", map[string]interface{}{
		"seat":      item.Controller,
		"destroyed": destroyed,
	})
}

// --- Damnation ---
//
// Oracle text:
//   Destroy all creatures. They can't be regenerated.
//
// 2BB sorcery. Black Wrath of God.
func registerDamnation(r *Registry) {
	r.OnResolve("Damnation", damnationResolve)
}

func damnationResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "damnation"
	if gs == nil || item == nil {
		return
	}
	destroyed := destroyAllCreatures(gs)
	emit(gs, slug, "Damnation", map[string]interface{}{
		"seat":      item.Controller,
		"destroyed": destroyed,
	})
}

// --- Toxic Deluge ---
//
// Oracle text:
//   As an additional cost to cast this spell, pay X life.
//   All creatures get -X/-X until end of turn.
//
// 2B sorcery. Bypasses indestructible via -X/-X (toughness reduction
// makes SBAs kill creatures with 0 or less toughness). X is chosen by
// the caster.
func registerToxicDeluge(r *Registry) {
	r.OnResolve("Toxic Deluge", toxicDelugeResolve)
}

func toxicDelugeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "toxic_deluge"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// X is the life paid. Use ChosenX from the stack item, or default
	// to a heuristic based on the largest creature toughness.
	x := item.ChosenX
	if x <= 0 {
		// Heuristic: pay enough to kill the biggest creature.
		x = biggestCreatureToughness(gs)
		if x <= 0 {
			x = 1
		}
		// Don't pay more life than we have minus 1.
		if seat >= 0 && seat < len(gs.Seats) && x >= gs.Seats[seat].Life {
			x = gs.Seats[seat].Life - 1
			if x < 1 {
				x = 1
			}
		}
	}

	// Pay X life as additional cost.
	gs.Seats[seat].Life -= x
	gs.LogEvent(gameengine.Event{
		Kind:   "life_change",
		Seat:   seat,
		Source: "Toxic Deluge",
		Amount: -x,
		Details: map[string]interface{}{
			"reason": "additional_cost",
		},
	})

	// Apply -X/-X to ALL creatures until end of turn.
	modified := 0
	ts := gs.NextTimestamp()
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power:     -x,
				Toughness: -x,
				Duration:  "until_end_of_turn",
				Timestamp: ts,
			})
			modified++
		}
	}

	// SBAs will handle creatures with 0 or less toughness.
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, "Toxic Deluge", map[string]interface{}{
		"seat":     seat,
		"x":       x,
		"modified": modified,
	})
}

// --- Blasphemous Act ---
//
// Oracle text:
//   This spell costs {1} less to cast for each creature on the
//   battlefield.
//   Blasphemous Act deals 13 damage to each creature.
//
// 8R sorcery. Often costs {R} in creature-heavy games.
func registerBlasphemousAct(r *Registry) {
	r.OnResolve("Blasphemous Act", blasphemousActResolve)
}

func blasphemousActResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "blasphemous_act"
	if gs == nil || item == nil {
		return
	}

	// Deal 13 damage to each creature.
	damaged := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		// Snapshot creatures to avoid mutation during iteration.
		creatures := make([]*gameengine.Permanent, 0)
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				creatures = append(creatures, p)
			}
		}
		for _, p := range creatures {
			p.MarkedDamage += 13
			damaged++
			gs.LogEvent(gameengine.Event{
				Kind:   "damage",
				Seat:   item.Controller,
				Target: p.Controller,
				Source: "Blasphemous Act",
				Amount: 13,
				Details: map[string]interface{}{
					"target_card": p.Card.DisplayName(),
				},
			})
		}
	}

	emit(gs, slug, "Blasphemous Act", map[string]interface{}{
		"seat":    item.Controller,
		"damaged": damaged,
		"damage":  13,
	})
}

// --- Vanquish the Horde ---
//
// Oracle text:
//   This spell costs {2} less to cast for each creature on the
//   battlefield.
//   Destroy all creatures.
//
// 6WW sorcery.
func registerVanquishTheHorde(r *Registry) {
	r.OnResolve("Vanquish the Horde", vanquishTheHordeResolve)
}

func vanquishTheHordeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "vanquish_the_horde"
	if gs == nil || item == nil {
		return
	}
	destroyed := destroyAllCreatures(gs)
	emit(gs, slug, "Vanquish the Horde", map[string]interface{}{
		"seat":      item.Controller,
		"destroyed": destroyed,
	})
}

// --- Farewell ---
//
// Oracle text:
//   Choose one or more —
//   • Exile all artifacts.
//   • Exile all creatures.
//   • Exile all enchantments.
//   • Exile all graveyards.
//
// 4WW sorcery. Modal exile-based wipe. Bypasses indestructible.
// MVP: choose all four modes (most common line in Commander).
func registerFarewell(r *Registry) {
	r.OnResolve("Farewell", farewellResolve)
}

func farewellResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "farewell"
	if gs == nil || item == nil {
		return
	}

	// MVP: all four modes. Exile artifacts, creatures, enchantments,
	// and all graveyards.
	exiledPerms := 0
	exiledGY := 0

	// Phase 1: Collect all permanents to exile (artifacts + creatures + enchantments).
	var toExile []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			if p.IsCreature() || p.IsArtifact() || p.IsEnchantment() {
				toExile = append(toExile, p)
			}
		}
	}

	// Exile permanents. Use ExilePermanent for proper zone-change handling.
	for _, p := range toExile {
		if gameengine.ExilePermanent(gs, p, nil) {
			exiledPerms++
		}
	}

	// Phase 2: Exile all graveyards.
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		exiledGY += len(s.Graveyard)
		gyCards := append([]*gameengine.Card(nil), s.Graveyard...)
		s.Graveyard = nil
		for _, c := range gyCards {
			gameengine.MoveCard(gs, c, seatIdx, "graveyard", "exile", "exile-from-graveyard")
		}
	}

	emit(gs, slug, "Farewell", map[string]interface{}{
		"seat":           item.Controller,
		"exiled_perms":   exiledPerms,
		"exiled_gy_cards": exiledGY,
	})
}

// --- Austere Command ---
//
// Oracle text:
//   Choose two —
//   • Destroy all artifacts.
//   • Destroy all enchantments.
//   • Destroy all creatures with mana value 3 or less.
//   • Destroy all creatures with mana value 4 or greater.
//
// 4WW sorcery. Modal destroy.
// MVP: choose "destroy all creatures with MV 3 or less" + "destroy all
// creatures with MV 4 or greater" (= destroy all creatures). This is
// the most common Commander line.
func registerAustereCommand(r *Registry) {
	r.OnResolve("Austere Command", austereCommandResolve)
}

func austereCommandResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "austere_command"
	if gs == nil || item == nil {
		return
	}

	// MVP: destroy all creatures (both CMC modes).
	// A more sophisticated version would let the Hat choose modes.
	destroyed := destroyAllCreatures(gs)

	emit(gs, slug, "Austere Command", map[string]interface{}{
		"seat":      item.Controller,
		"modes":     "creatures_leq3_and_geq4",
		"destroyed": destroyed,
	})
}

// ============================================================================
// Shared board-wipe helpers
// ============================================================================

// destroyAllCreatures destroys every creature on the battlefield using
// the proper DestroyPermanent path (respects indestructible, fires
// dies/LTB triggers). Returns the count destroyed.
func destroyAllCreatures(gs *gameengine.GameState) int {
	if gs == nil {
		return 0
	}
	// Snapshot all creatures across all seats.
	var creatures []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				creatures = append(creatures, p)
			}
		}
	}
	destroyed := 0
	for _, p := range creatures {
		if gameengine.DestroyPermanent(gs, p, nil) {
			destroyed++
		}
	}
	return destroyed
}

// biggestCreatureToughness returns the highest toughness among all
// creatures on the battlefield. Used by Toxic Deluge as a heuristic
// for choosing X.
func biggestCreatureToughness(gs *gameengine.GameState) int {
	if gs == nil {
		return 0
	}
	max := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			t := p.Toughness()
			if t > max {
				max = t
			}
		}
	}
	return max
}
