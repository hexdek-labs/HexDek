package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAsmoranomardicadaistinaculdacarCustom wires the ETB Cookbook
// tutor and the sacrifice-two-Foods activated ping. The auto-generated
// gen_asmoranomardicadaistinaculdacar.go was a partial-emit stub; both
// the file and its batch_generated.go register call were deleted in
// Versailles Phase 2I — this is now the sole registration site, called
// from registry.go::registerDefaults.
//
// Oracle text (Modern Horizons 2, {1}{B}{R}{G}; alt cost {B/R}):
//
//	As long as you've discarded a card this turn, you may pay {B/R} to
//	cast this spell.
//	When Asmoranomardicadaistinaculdacar enters, you may search your
//	library for a card named The Underworld Cookbook, reveal it, put
//	it into your hand, then shuffle.
//	Sacrifice two Foods: Target creature deals 6 damage to itself.
//
// Implementation:
//   - OnETB: tutor "The Underworld Cookbook" if present in the library.
//     The "you may" defaults to YES — the deck's gameplan is the
//     food/madness combo so the tutor is always strictly upside.
//   - OnActivated(0): sacrifice two Food tokens, pick the biggest
//     opposing creature, mark 6 damage on it (the oracle "deals 6 to
//     itself" is modeled by attributing the source to the target — the
//     SBA picks it up the same way).
//   - Alt-cost {B/R} after-discard cost reduction is on the cast path,
//     not per-card. emitPartial.
func registerAsmoranomardicadaistinaculdacarCustom(r *Registry) {
	r.OnETB("Asmoranomardicadaistinaculdacar", asmoranETBCookbookTutor)
	r.OnActivated("Asmoranomardicadaistinaculdacar", asmoranSacFoodsPing)
}

func asmoranETBCookbookTutor(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "asmoran_etb_cookbook_tutor"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	const target = "the underworld cookbook"
	tutorToHand(gs, seatIdx, func(c *gameengine.Card) bool {
		return c != nil && strings.EqualFold(strings.TrimSpace(c.DisplayName()), "The Underworld Cookbook")
	}, perm.Card.DisplayName())
	_ = target
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seatIdx,
		"target": "The Underworld Cookbook",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"alt_cast_cost_br_after_discard_not_modeled_in_per_card")
}

func asmoranSacFoodsPing(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "asmoran_sacrifice_two_foods_self_ping"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}

	// Find two Food tokens (artifacts with "food" subtype) on the
	// controller's battlefield.
	var foods []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		if !cardHasSubtype(p.Card, "food") {
			continue
		}
		foods = append(foods, p)
		if len(foods) >= 2 {
			break
		}
	}
	if len(foods) < 2 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_foods", map[string]interface{}{
			"foods_available": len(foods),
		})
		return
	}

	// Find the biggest opposing creature to point at.
	var target *gameengine.Permanent
	for i, s := range gs.Seats {
		if s == nil || i == seatIdx || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if target == nil || p.Power() > target.Power() {
				target = p
			}
		}
	}
	if target == nil {
		// No opposing creature — fall back to the controller's smallest
		// creature (rare; the activation is mostly used to remove an
		// opp threat). Skip rather than ping our own.
		emitFail(gs, slug, src.Card.DisplayName(), "no_opposing_creature_target", nil)
		return
	}

	// Pay the cost: sacrifice two Foods.
	for i := 0; i < 2; i++ {
		gameengine.SacrificePermanent(gs, foods[i], "asmoran_food_cost")
	}

	// Deal 6 damage to the target creature. The card text reads "deals 6
	// damage to itself" — the source is the target creature, which only
	// matters for lifelink / damage-replacement triggers. We model the
	// effect as marked damage; SBA picks up lethal-damage destruction.
	target.MarkedDamage += 6
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   target.Controller,
		Source: target.Card.DisplayName(),
		Amount: 6,
		Details: map[string]interface{}{
			"slug":   slug,
			"reason": "asmoran_food_sacrifice_self_damage",
			"target": target.Card.DisplayName(),
		},
	})
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":    seatIdx,
		"target":  target.Card.DisplayName(),
		"damage":  6,
		"foods_sacrificed": 2,
	})
}
