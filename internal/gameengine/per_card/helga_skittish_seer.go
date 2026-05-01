package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHelgaSkittishSeer wires Helga, Skittish Seer.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Whenever you cast a creature spell with mana value 4 or greater,
//	you draw a card, gain 1 life, and put a +1/+1 counter on Helga.
//	{T}: Add X mana of any one color, where X is Helga's power. Spend
//	this mana only to cast creature spells with mana value 4 or
//	greater or creature spells with {X} in their mana costs.
//
// Implementation:
//   - "creature_spell_cast": gate on caster_seat == perm.Controller and
//     ManaCostOf(card) >= 4. Helga isn't on the battlefield when she's
//     cast (fireTrigger filters battlefield-only) so we don't need to
//     filter self-cast. On hit: drawOne, GainLife(1), AddCounter("+1/+1", 1).
//   - OnActivated(0, ...): {T}: Add X mana of any one color. We add
//     X = Helga.Power() of green to the controller's pool — green is
//     the most generally useful color in her identity (G/W/U). The
//     "spend only on creature spells with MV >= 4 or {X} cost" restriction
//     is not enforced (no per-mana taint tracking in this engine);
//     emitPartial flags the gap.
func registerHelgaSkittishSeer(r *Registry) {
	r.OnTrigger("Helga, Skittish Seer", "creature_spell_cast", helgaSkittishSeerCast)
	r.OnActivated("Helga, Skittish Seer", helgaSkittishSeerActivate)
}

func helgaSkittishSeerCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "helga_skittish_seer_cast_payoff"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || !cardHasType(card, "creature") {
		return
	}
	if gameengine.ManaCostOf(card) < 4 {
		return
	}

	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_spell": card.DisplayName(),
		"cast_cmc":   gameengine.ManaCostOf(card),
		"drawn_card": drawnName,
		"counters":   perm.Counters["+1/+1"],
	})
}

func helgaSkittishSeerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "helga_skittish_seer_mana"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
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
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	x := src.Power()
	if x <= 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "power_le_zero", map[string]interface{}{
			"power": x,
		})
		return
	}
	src.Tapped = true
	color := "G"
	gameengine.AddManaFromPermanent(gs, s, src, color, x)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"added":    x,
		"color":    color,
		"new_pool": s.ManaPool,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"spend_restriction_creature_mv_ge_4_or_X_cost_not_enforced")
}
