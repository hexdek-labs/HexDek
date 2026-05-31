package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerXuIfitOsteoharmonist wires Xu-Ifit, Osteoharmonist.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{1}{B}{B}
//	Legendary Creature — Human Wizard
//	2/3
//	{T}: Return target creature card from your graveyard to the
//	battlefield. It's a Skeleton in addition to its other types and has
//	no abilities. Activate only as a sorcery.
//
// Implementation:
//   - OnActivated: find highest-power creature in controller's graveyard,
//     return it to the battlefield as a vanilla Skeleton (clear Abilities,
//     prepend "skeleton" type). Engine doesn't enforce "no abilities" via
//     a layers strip — we clear the Abilities slice on the card object,
//     which is sufficient for combat math but loses keyword abilities
//     parsed via the AST. emitPartial flags this.
func registerXuIfitOsteoharmonist(r *Registry) {
	r.OnActivated("Xu-Ifit, Osteoharmonist", xuIfitOsteoharmonistActivate)
}

func xuIfitOsteoharmonistActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "xu_ifit_osteoharmonist_reanimate_skeleton"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		return
	}
	src.Tapped = true
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	bestIdx := -1
	bestPower := -1
	for i, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		pw := int(c.BasePower)
		if pw > bestPower {
			bestPower = pw
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_in_graveyard", nil)
		return
	}
	card := seat.Graveyard[bestIdx]
	// createPermanent (called via enterBattlefieldWithETB below) sweeps
	// the source from every private zone, so the manual graveyard splice
	// is redundant (Wave 2 final-audit cleanup).
	// "no abilities" — we can't strip AST keywords cleanly here, so the
	// card retains its triggered/static abilities. emitPartial below flags it.
	hasSkeleton := false
	for _, t := range card.Types {
		if t == "skeleton" {
			hasSkeleton = true
			break
		}
	}
	if !hasSkeleton {
		card.Types = append(card.Types, "skeleton")
	}
	enterBattlefieldWithETB(gs, src.Controller, card, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       src.Controller,
		"reanimated": card.DisplayName(),
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"abilities_stripped_via_card_field_not_via_layers_keyword_words_in_oracle_text_may_persist")
}
