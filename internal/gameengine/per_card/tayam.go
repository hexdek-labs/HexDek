package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTayam wires Tayam, Luminous Enigma.
//
// Oracle text:
//
//	Each other creature you control enters the battlefield with an
//	additional vigilance counter on it.
//	{3}, Remove three counters from among creatures you control: Mill
//	three cards, then return a permanent card with mana value 3 or less
//	from your graveyard to the battlefield.
//
// Tayam is a grindable recursion engine: every creature ETB seeds
// removable fuel, and the activated ability spends 3 counters to mill 3
// and reanimate a CMC<=3 permanent. Pairs with sac outlets (Phyrexian
// Altar, Viscera Seer) and persist creatures (Woodfall Primus,
// Murderous Redcap) for arbitrarily large value loops.
//
// Implementation:
//   - OnTrigger("permanent_etb") seeds a "vigilance" counter on each
//     OTHER creature entering under Tayam's controller. Modeled as a
//     trigger rather than a §614 replacement so the counter is visible
//     to subsequent ETB-trigger observers.
//   - OnActivated picks 3 counters off the controller's most-stacked
//     creatures, mills 3 from the top of the library, then reanimates
//     the highest-CMC permanent <= 3 in the graveyard.
func registerTayam(r *Registry) {
	r.OnTrigger("Tayam, Luminous Enigma", "permanent_etb", tayamETBCounter)
	r.OnActivated("Tayam, Luminous Enigma", tayamActivate)
}

func tayamETBCounter(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tayam_etb_vigilance_counter"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering == perm {
		return
	}
	if !entering.IsCreature() {
		return
	}
	entering.AddCounter("vigilance", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"target":   entering.Card.DisplayName(),
		"counters": entering.Counters["vigilance"],
	})
}

func tayamActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "tayam_recursion_activate"
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

	// Step 1: remove 3 counters from among creatures controller controls.
	// Greedy: repeatedly pick the creature with the highest single-type
	// counter pile and decrement.
	removed := 0
	removedDetails := []map[string]interface{}{}
	for removed < 3 {
		var bestPerm *gameengine.Permanent
		var bestKey string
		bestCount := 0
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			for k, v := range p.Counters {
				if v > bestCount {
					bestCount = v
					bestPerm = p
					bestKey = k
				}
			}
		}
		if bestPerm == nil || bestCount <= 0 {
			break
		}
		bestPerm.AddCounter(bestKey, -1)
		removedDetails = append(removedDetails, map[string]interface{}{
			"creature": bestPerm.Card.DisplayName(),
			"counter":  bestKey,
		})
		removed++
	}
	if removed < 3 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_counters", map[string]interface{}{
			"seat":    seat,
			"removed": removed,
		})
		return
	}

	// Step 2: mill 3 cards.
	milled := 0
	for i := 0; i < 3 && len(s.Library) > 0; i++ {
		c := s.Library[0]
		gameengine.MoveCard(gs, c, seat, "library", "graveyard", "mill")
		milled++
	}
	if milled > 0 {
		gs.LogEvent(gameengine.Event{
			Kind:   "mill",
			Seat:   seat,
			Target: seat,
			Source: src.Card.DisplayName(),
			Amount: milled,
		})
	}

	// Step 3: pick best permanent card with CMC <= 3 from graveyard.
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if !isPermanentCard(c) {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > 3 {
			continue
		}
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}

	if best == nil {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":             seat,
			"counters_removed": removed,
			"milled":           milled,
			"reanimated":       nil,
		})
		return
	}

	gameengine.MoveCard(gs, best, seat, "graveyard", "battlefield", "tayam_recursion")
	enterBattlefieldWithETB(gs, seat, best, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              seat,
		"counters_removed":  removed,
		"removed_breakdown": removedDetails,
		"milled":            milled,
		"reanimated":        best.DisplayName(),
		"reanimated_cmc":    bestCMC,
	})
}

// isPermanentCard returns true if the card is one of the permanent types
// (creature, artifact, enchantment, land, planeswalker, battle).
func isPermanentCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return cardHasType(c, "creature") ||
		cardHasType(c, "artifact") ||
		cardHasType(c, "enchantment") ||
		cardHasType(c, "land") ||
		cardHasType(c, "planeswalker") ||
		cardHasType(c, "battle")
}
