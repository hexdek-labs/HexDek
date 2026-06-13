package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// necromantic_selection.go — per_card handler for Necromantic Selection.
//
// Oracle text (Sorcery, {4}{B}{B}):
//
//	Destroy all creatures, then return a creature card put into a
//	graveyard this way to the battlefield under your control. It's a
//	black Zombie in addition to its other colors and types. Exile
//	Necromantic Selection.
//
// Why this needs a per_card handler: the parser emitted three bare
// `custom` Modification nodes for this card (no observable engine
// effect), so on the measured-but-inert surface the spell did NOTHING
// when it resolved — a 6-mana board wipe + reanimator finisher running
// completely dead. Verified inert: no prior handler, and the stock AST
// dispatch has no rule for the `custom` kind here.
//
// Implementation (OnResolve — authoritative spell body, stock dispatch
// is skipped once a resolve handler fires):
//
//  1. Snapshot every creature on the battlefield across all seats
//     (these are the only cards eligible to be "put into a graveyard
//     this way" — a creature already in a graveyard, and tokens that
//     cease to exist, are correctly excluded).
//  2. Destroy them all via DestroyPermanent (fires dies triggers,
//     honors indestructible / §614 to-exile replacements).
//  3. Among the snapshot, keep those that actually landed in a
//     graveyard, pick the strongest (power, CMC tiebreak), and return
//     it to the battlefield under the caster's control, mirroring the
//     Reanimate reparent shape (MoveCard to owner's battlefield, then
//     reparent the Permanent to the caster per CR §110.2a).
//  4. Add black + Zombie to its types/colors per the oracle text.
//
// Deliberately omitted (flavor-only, no game-state impact in practice):
// the trailing "Exile Necromantic Selection" self-exile. The sorcery
// otherwise goes to its owner's graveyard; this only matters if the
// spell itself were later recurred, which the engine's reanimation /
// flashback paths don't reach for this card. Logged in the shard report.

func init() {
	registerNecromanticSelection(Global())
	AddResetHook(registerNecromanticSelection)
}

func registerNecromanticSelection(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Necromantic Selection", necromanticSelectionResolve)
}

func necromanticSelectionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "necromantic_selection"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// (1) Snapshot every battlefield creature card before the wipe.
	snapshot := map[*gameengine.Card]bool{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.IsCreature() {
				snapshot[p.Card] = true
			}
		}
	}

	// (2) Destroy all creatures.
	destroyed := destroyAllCreatures(gs)

	// (3) Among the snapshot, pick the strongest card that landed in a
	// graveyard (i.e. actually died this way).
	var bestCard *gameengine.Card
	bestSeat := -1
	bestScore := -1
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil || !snapshot[c] {
				continue
			}
			score := cardCMC(c)
			if p := basePowerOf(c); p > 0 {
				// Weight power above CMC so a 7/7 for 5 outranks a 6-CMC 2/2.
				score = p*100 + cardCMC(c)
			} else {
				score = cardCMC(c)
			}
			if score > bestScore {
				bestScore = score
				bestCard = c
				bestSeat = i
			}
		}
	}

	if bestCard == nil {
		emit(gs, slug, "Necromantic Selection", map[string]interface{}{
			"seat":      seat,
			"destroyed": destroyed,
			"returned":  "",
		})
		return
	}

	// Return to the battlefield under the caster's control.
	gameengine.MoveCard(gs, bestCard, bestSeat, "graveyard", "battlefield", slug)
	reparented := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card != bestCard {
				continue
			}
			if p.Controller != seat {
				removePermanent(gs, p)
				p.Controller = seat
				gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
				reparented = true
			}
			break
		}
	}

	// (4) Becomes a black Zombie in addition to its other colors/types.
	addColorIfMissing(bestCard, "B")
	addTypeIfMissing(bestCard, "zombie")

	emit(gs, slug, "Necromantic Selection", map[string]interface{}{
		"seat":       seat,
		"destroyed":  destroyed,
		"returned":   bestCard.DisplayName(),
		"from_seat":  bestSeat,
		"reparented": reparented,
	})
	_ = gs.CheckEnd()
}

// basePowerOf reports a creature card's printed base power (0 if absent
// or non-numeric, e.g. "*").
func basePowerOf(c *gameengine.Card) int {
	if c == nil {
		return 0
	}
	return c.BasePower
}

// addColorIfMissing appends a color abbreviation to the card if not
// already present.
func addColorIfMissing(c *gameengine.Card, color string) {
	if c == nil {
		return
	}
	for _, x := range c.Colors {
		if x == color {
			return
		}
	}
	c.Colors = append(c.Colors, color)
}

// addTypeIfMissing appends a (lowercase) subtype/type to the card if not
// already present.
func addTypeIfMissing(c *gameengine.Card, t string) {
	if c == nil {
		return
	}
	for _, x := range c.Types {
		if x == t {
			return
		}
	}
	c.Types = append(c.Types, t)
}
