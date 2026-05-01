package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAbigalePoetLaureate wires Abigale, Poet Laureate // Heroic
// Stanza (Secrets of Strixhaven, Prepare DFC). Batch #31.
//
// Front face — Abigale, Poet Laureate ({1}{W}{B}, Legendary Creature —
// Bird Bard, 2/3):
//
//	Flying
//	Whenever you cast a creature spell, Abigale becomes prepared.
//	(While it's prepared, you may cast a copy of its spell. Doing
//	so unprepares it.)
//
// Back face — Heroic Stanza ({1}{W/B}, Sorcery):
//
//	Put a +1/+1 counter on target creature.
//
// Implementation:
//   - Front face — "spell_cast" gated on caster_seat == perm.Controller and
//     a CREATURE spell other than Abigale. Mark Abigale prepared via a
//     flag, then immediately resolve a copy of Heroic Stanza: pick the
//     best +1/+1-receiver creature on the controller's battlefield and
//     add a +1/+1 counter. Doing so "unprepares" her (we clear the flag
//     within the same trigger). This bypasses the literal "may cast a
//     copy of the spell" stack interaction (the AI always wants the
//     +1/+1 and the back face has no targets it could fizzle on if at
//     least one creature is in play), but captures the value cleanly.
//     emitPartial flags the simplification.
//   - Back face dispatch: the registry's " // " split fallback dispatches
//     a normalized "Abigale, Poet Laureate" hit on either face. We
//     register both the slash name and the front face explicitly so
//     resolution after a hypothetical flip still hits us.
func registerAbigalePoetLaureate(r *Registry) {
	r.OnTrigger("Abigale, Poet Laureate // Heroic Stanza", "spell_cast", abigalePreparedTrigger)
	r.OnTrigger("Abigale, Poet Laureate", "spell_cast", abigalePreparedTrigger)
}

func abigalePreparedTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "abigale_poet_laureate_prepared_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "creature") {
		return
	}
	// "another creature spell" is not strictly required by oracle text, but
	// Abigale isn't on the battlefield when she's cast from the command
	// zone — the trigger fires from the battlefield only, so a self-cast
	// trigger would never fire anyway. Guard defensively.
	if card == perm.Card {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["abigale_prepared"] = 1

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	target := abigaleBestPlusOneTarget(seat, perm)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_creature_target", map[string]interface{}{
			"trigger_spell": card.DisplayName(),
		})
		// "Doing so unprepares it." — if you don't cast the copy you stay
		// prepared. Leave the flag set; next creature cast still gets one
		// shot at the +1/+1 counter.
		return
	}

	target.AddCounter("+1/+1", 1)
	perm.Flags["abigale_prepared"] = 0
	gs.LogEvent(gameengine.Event{
		Kind:   "counter_added",
		Seat:   perm.Controller,
		Target: target.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"slug":          slug,
			"counter_kind":  "+1/+1",
			"target_card":   target.Card.DisplayName(),
			"trigger_spell": card.DisplayName(),
			"reason":        "heroic_stanza_copy_via_prepared",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"trigger_spell": card.DisplayName(),
		"target_card":   target.Card.DisplayName(),
		"copied_back":   "Heroic Stanza",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"prepare_keyword_resolves_back_face_directly_skipping_stack_and_mana_cost")
}

// abigaleBestPlusOneTarget picks the controller's best landing spot for a
// +1/+1 counter. Prefers creatures with combat damage (high power),
// breaks ties on toughness. Skips Abigale herself (she's a fine target
// but tokens / attackers benefit more) only when better targets exist.
func abigaleBestPlusOneTarget(seat *gameengine.Seat, abigale *gameengine.Permanent) *gameengine.Permanent {
	if seat == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestScore := -1
	var fallback *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		score := p.Power()*2 + p.Toughness()
		if p == abigale {
			if fallback == nil {
				fallback = p
			}
			continue
		}
		if score > bestScore {
			bestScore = score
			best = p
		}
	}
	if best != nil {
		return best
	}
	return fallback
}
