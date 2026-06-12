package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSandmanShiftingScoundrel wires Sandman, Shifting Scoundrel.
//
// Oracle text (Scryfall, verified 2026-05-09):
//
//	Sandman's power and toughness are each equal to the number of lands
//	you control.
//	Sandman can't be blocked by creatures with power 2 or less.
//	{3}{G}{G}: Return this card and target land card from your graveyard
//	to the battlefield tapped.
//
// Implementation:
//   - The */* characteristic-defining stat = lands controlled is a
//     continuous effect the AST layer handles via its CDA pipeline; not
//     wired here.
//   - The block-restriction (power 2 or less can't block) is engine-side
//     combat layer.
//   - The activated ability is graveyard-activated. The standard
//     OnActivated dispatch path expects `src` to be a battlefield
//     Permanent. If the engine's activation harness ever wraps a
//     graveyard card into a transient Permanent for the dispatch, this
//     handler will resolve cleanly. Otherwise the engine never calls us
//     and the breadcrumb in the ETB hook is the only signal.
//
// Effect when the handler does fire:
//   - Pay {3}{G}{G} = 5 (defensive top-up; engine usually pre-deducts).
//   - Find the highest-MV land card in controller's graveyard.
//   - Return Sandman from graveyard to battlefield tapped (if Sandman is
//     in graveyard at handler time).
//   - Return that land tapped.
func registerSandmanShiftingScoundrel(r *Registry) {
	r.OnETB("Sandman, Shifting Scoundrel", sandmanShiftingETB)
	r.OnActivated("Sandman, Shifting Scoundrel", sandmanShiftingActivate)
}

func sandmanShiftingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sandman_shifting_static_pt_and_block"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"cda_pt_equals_lands_and_unblockable_by_power_2_or_less_engine_side")
}

func sandmanShiftingActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sandman_shifting_self_and_land_recur"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Defensive cost top-up: {3}{G}{G} = 5 generic. Engine dispatcher
	// usually has already deducted; only deduct if the pool covers it.
	if seat.ManaPool >= 5 {
		seat.ManaPool -= 5
		gameengine.SyncManaAfterSpend(seat)
	}

	// Pick best land in graveyard: highest-MV (proxies for utility lands
	// like Bojuka Bog / Maze of Ith / Gaea's Cradle being worth more than
	// vanilla basics).
	var bestLand *gameengine.Card
	bestLandIdx := -1
	bestLandCMC := -1
	for i, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "land") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestLandCMC {
			bestLandCMC = cmc
			bestLand = c
			bestLandIdx = i
		}
	}

	// Return Sandman from graveyard if present. The activation should be
	// graveyard-only per the oracle's "this card... from your graveyard"
	// phrasing — if Sandman is on the battlefield instead (handler called
	// from a non-graveyard activation harness), skip the self-return.
	sandmanInGrave := false
	for i, c := range seat.Graveyard {
		if c == src.Card {
			seat.Graveyard = append(seat.Graveyard[:i], seat.Graveyard[i+1:]...)
			// Adjust bestLandIdx if it was past the removed slot.
			if bestLandIdx > i {
				bestLandIdx--
			}
			sandmanInGrave = true
			break
		}
	}
	if sandmanInGrave {
		gameengine.MoveCard(gs, src.Card, seatIdx, "graveyard", "battlefield_tapped", "sandman_self_recur")
	} else {
		// TODO: engine support needed for graveyard-activated abilities —
		// dispatch path currently keys on battlefield permanents.
		emitPartial(gs, slug, src.Card.DisplayName(),
			"sandman_not_in_graveyard_at_activation_self_return_skipped")
	}

	if bestLand != nil && bestLandIdx >= 0 && bestLandIdx < len(seat.Graveyard) && seat.Graveyard[bestLandIdx] == bestLand {
		seat.Graveyard = append(seat.Graveyard[:bestLandIdx], seat.Graveyard[bestLandIdx+1:]...)
		gameengine.MoveCard(gs, bestLand, seatIdx, "graveyard", "battlefield_tapped", "sandman_land_recur")
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              seatIdx,
		"sandman_returned":  sandmanInGrave,
		"land_returned":     bestLand != nil,
		"land_name":         sandmanLandName(bestLand),
	})
}

func sandmanLandName(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	return c.DisplayName()
}
