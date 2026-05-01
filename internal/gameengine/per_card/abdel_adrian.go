package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAbdelAdrian wires Abdel Adrian, Gorion's Ward.
//
// Oracle text:
//
//	When Abdel Adrian, Gorion's Ward enters the battlefield, exile any
//	number of other nonland permanents you control until Abdel Adrian
//	leaves the battlefield. Create a 1/1 white Soldier creature token
//	for each permanent exiled this way.
//	Partner
//
// Heuristic exile choice: cheap (CMC <= 3), non-land, non-self
// permanents — typically mana rocks and small utility creatures that
// reload as 1/1 Soldiers (and ETB-trigger again on return). Capped at 4
// exiles to avoid over-extending board state in simulation.
//
// LTB return is not yet wired — the engine's LTB observer system
// doesn't yet expose the per-permanent exile pool we'd need to bring
// the cards back. emitPartial flags this so audits can find it.
func registerAbdelAdrian(r *Registry) {
	r.OnETB("Abdel Adrian, Gorion's Ward", abdelAdrianETB)
}

func abdelAdrianETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "abdel_adrian_exile_for_soldiers"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	const maxExiles = 4
	var picks []*gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if p.IsLand() {
			continue
		}
		if gameengine.ManaCostOf(p.Card) > 3 {
			continue
		}
		picks = append(picks, p)
		if len(picks) >= maxExiles {
			break
		}
	}

	exiledNames := make([]string, 0, len(picks))
	for _, p := range picks {
		card := p.Card
		removePermanent(gs, p)
		moveCardBetweenZones(gs, seat, card, "battlefield", "exile", "abdel_adrian")
		exiledNames = append(exiledNames, card.DisplayName())
	}

	for range picks {
		token := &gameengine.Card{
			Name:          "Soldier Token",
			Owner:         seat,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "soldier"},
			Colors:        []string{"W"},
			TypeLine:      "Token Creature — Soldier",
		}
		enterBattlefieldWithETB(gs, seat, token, false)
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["abdel_exiled_count"] = len(picks)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"exiled_count":  len(picks),
		"exiled_cards":  exiledNames,
		"tokens_created": len(picks),
	})
	if len(picks) > 0 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"ltb_return_of_exiled_permanents_not_wired_pending_observer_pool")
	}
}
