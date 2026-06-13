package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// lucid_dreams_r60.go — per_card handler for Lucid Dreams.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Draw X cards, where X is the number of card types among cards in
//	your graveyard.
//
// {3}{U} Sorcery. A graveyard-payoff draw spell (artifact/creature/
// enchantment/instant/land/planeswalker/sorcery diversity = up to 7+
// cards). Parses to a single inert `parsed_effect_residual` node and no
// structured Draw node, so it drew ZERO cards: the text fallback only
// recognizes specific draw shapes (draw-N-lose-N, discard-then-draw,
// draw-seven, draw-per) — not "draw X cards where X = …".
//
// Implementation:
//   - OnResolve. Counts the DISTINCT card types (the eight + kindred,
//     not supertypes/subtypes) present across the controller's
//     graveyard, then draws that many via the shared gameengine.DrawN
//     primitive (full §614.11 replacement + draw-trigger fidelity).
func init() {
	registerLucidDreamsR60(Global())
	AddResetHook(registerLucidDreamsR60)
}

func registerLucidDreamsR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Lucid Dreams", lucidDreamsResolve)
}

// canonicalCardTypes is the set of CR §300.1 card types counted by
// "number of card types" effects (Lucid Dreams, Tarmogoyf-style). It
// deliberately excludes supertypes (legendary/basic/snow/world) and
// subtypes, which the resolver folds into Card.Types at ETB.
var canonicalCardTypes = map[string]struct{}{
	"artifact": {}, "battle": {}, "creature": {}, "enchantment": {},
	"instant": {}, "land": {}, "planeswalker": {}, "sorcery": {},
	"kindred": {}, "tribal": {},
}

func lucidDreamsResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "lucid_dreams"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	seen := map[string]struct{}{}
	for _, c := range gs.Seats[seat].Graveyard {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			if _, ok := canonicalCardTypes[t]; ok {
				seen[t] = struct{}{}
			}
		}
	}
	x := len(seen)

	var src *gameengine.Permanent
	if item.Card != nil {
		src = &gameengine.Permanent{Card: item.Card, Controller: seat, Owner: item.Card.Owner}
	}
	drawn := gameengine.DrawN(gs, seat, x, src)

	emit(gs, slug, "Lucid Dreams", map[string]interface{}{
		"seat":       seat,
		"card_types": x,
		"drawn":      drawn,
	})
}
