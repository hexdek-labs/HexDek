package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDrivnodCarnageDominus wires Drivnod, Carnage Dominus.
//
// Oracle text (Scryfall, verified — Phyrexia: All Will Be One):
//
//	If a creature dying causes a triggered ability of a permanent you
//	control to trigger, that ability triggers an additional time.
//	{B/P}{B/P}, Exile three creature cards from your graveyard: Put
//	an indestructible counter on Drivnod. ({B/P} can be paid with
//	either {B} or 2 life.)
//
// Implementation:
//   - Death-trigger doubler static: per-seat flag at ETB; engine-layer
//     death-trigger dispatch reads "death_trigger_doubler_count".
//   - Activated cost — r51 fix: the prior handler called MoveCard
//     (battlefield→exile) on two creature permanents, which is wrong
//     on two fronts:
//       (a) printed cost is "Exile three creature cards from your
//           graveyard", not "Exile two creatures from the battlefield".
//       (b) MoveCard from battlefield is a no-op for Permanent
//           cleanup (sibling of the Mondrak game-59 CardIdentity leak).
//     Fixed by exiling three creature CARDS from the graveyard via
//     MoveCard(graveyard→exile), which IS a supported zone transition
//     (removeCardFromZone handles graveyard correctly). Counter count
//     corrected from 2 to 1, mana gate corrected from 4 to 2
//     ({B/P}{B/P} = 2 generic-equivalent).
func registerDrivnodCarnageDominus(r *Registry) {
	r.OnETB("Drivnod, Carnage Dominus", drivnodSetDeathDoublerFlag)
	r.OnActivated("Drivnod, Carnage Dominus", drivnodIndestructibleActivate)
}

func drivnodSetDeathDoublerFlag(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "drivnod_death_trigger_doubler_flag"
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
	seat.Flags["death_trigger_doubler_count"]++
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"doublers": seat.Flags["death_trigger_doubler_count"],
	})
	emitPartial(gs, "drivnod_doubles_death_triggers", perm.Card.DisplayName(),
		"death-trigger doubling requires resolve-time hook to duplicate stack copies; flag set for downstream")
}

func drivnodIndestructibleActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "drivnod_indestructible_activate"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	const manaCost = 2 // {B/P}{B/P} → 2 generic-equivalent
	if seat.ManaPool < manaCost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
			"mana_cost": manaCost,
		})
		return
	}
	// Collect three creature cards from the graveyard.
	var picks []*gameengine.Card
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		picks = append(picks, c)
		if len(picks) == 3 {
			break
		}
	}
	if len(picks) < 3 {
		emitFail(gs, slug, src.Card.DisplayName(), "fewer_than_3_creature_cards_in_graveyard", map[string]interface{}{
			"available": len(picks),
		})
		return
	}
	seat.ManaPool -= manaCost
	gameengine.SyncManaAfterSpend(seat)
	exiled := []string{}
	for _, c := range picks {
		gameengine.MoveCard(gs, c, src.Controller, "graveyard", "exile", "drivnod_exile_cost")
		exiled = append(exiled, c.DisplayName())
	}
	src.AddCounter("indestructible", 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           src.Controller,
		"exiled":         exiled,
		"indestructible": src.Counters["indestructible"],
	})
}
