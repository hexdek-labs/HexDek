package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGreatHallOfTheBiblioplex wires Great Hall of the Biblioplex.
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{T}: Add {C}.
//	{T}, Pay 1 life: Add one mana of any color. Spend this mana only to
//	cast an instant or sorcery spell.
//	{5}: If this land isn't a creature, it becomes a 2/4 Wizard creature
//	with "Whenever you cast an instant or sorcery spell, this creature
//	gets +1/+0 until end of turn." It's still a land.
//
// Implementation (Muninn gap #8 — 111K hits):
//   - abilityIdx 0 ({T}: Add {C}) is handled by the generic AST land
//     tap-for-mana pipeline; we no-op so the generic path runs.
//   - abilityIdx 1 ({T}, Pay 1 life: any color, instant/sorcery only):
//     tap, lose 1 life, add 1 mana to the pool. The "spend only on
//     instant or sorcery" restriction is typed-mana enforcement that
//     the untyped ManaPool can't model; emitPartial.
//   - abilityIdx 2 ({5}: become 2/4 Wizard with cast-trigger pump):
//     pay {5}, then emit a partial. Permanent-level type overlays
//     ("becomes a 2/4 Wizard creature") require the Phase 8 layers
//     pass that's not yet implemented (see state.go:1042-1051's MVP
//     note), so we don't mutate the Card's Types slice — that would
//     pollute every copy of the land. The animation is logged but
//     not realised on the battlefield; downstream tooling can pick up
//     the slug for future engine work.
func registerGreatHallOfTheBiblioplex(r *Registry) {
	r.OnActivated("Great Hall of the Biblioplex", greatHallOfTheBiblioplexActivated)
}

func greatHallOfTheBiblioplexActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	switch abilityIdx {
	case 0:
		// {T}: Add {C} — let the generic land tap-for-mana pipeline handle it.
		return

	case 1:
		const slug = "biblioplex_pay_life_any_color"
		if src.Tapped {
			emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
			return
		}
		src.Tapped = true
		gameengine.LoseLife(gs, seat, 1, src.Card.DisplayName())
		s.ManaPool++
		gameengine.SyncManaAfterSpend(s)
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":     seat,
			"life_paid": 1,
			"mana_added": 1,
		})
		emitPartial(gs, slug, src.Card.DisplayName(),
			"spend_only_on_instant_or_sorcery_typed_mana_restriction_unmodeled")

	case 2:
		const slug = "biblioplex_animate_wizard"
		if s.ManaPool < 5 {
			emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
				"seat":      seat,
				"mana_pool": s.ManaPool,
			})
			return
		}
		if src.IsCreature() {
			emitFail(gs, slug, src.Card.DisplayName(), "already_a_creature", nil)
			return
		}
		s.ManaPool -= 5
		gameengine.SyncManaAfterSpend(s)
		// R54 layer-7b port: register the becomes-2/4-Wizard-creature
		// continuous effect chain. Layer 4 adds "creature" + "wizard"
		// types in addition to land; Layer 7b sets base P/T to 2/4.
		// Duration is "until_source_leaves" — printed text doesn't
		// limit to end of turn, so the animation persists until Great
		// Hall leaves the battlefield (UnregisterContinuousEffectsForPermanent
		// auto-cleans then).
		gameengine.RegisterAddTypes(gs, src, []string{"creature", "wizard"},
			gameengine.DurationUntilSourceLeaves, src.Card.DisplayName(), "biblioplex_animate")
		gameengine.RegisterSetPT(gs, src, 2, 4,
			gameengine.DurationUntilSourceLeaves, src.Card.DisplayName(), "biblioplex_animate")
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      seat,
			"cost":      5,
			"animated":  "2/4 Wizard creature (still a land)",
			"layer_4":   []string{"creature", "wizard"},
			"layer_7b":  [2]int{2, 4},
		})
		emitPartial(gs, slug, src.Card.DisplayName(),
			"cast_trigger_plus_1_0_eot_pump_rider_needs_separate_per_card_trigger_wiring")

	default:
		emitFail(gs, "biblioplex_unknown_ability", src.Card.DisplayName(), "unknown_ability_idx", map[string]interface{}{
			"ability_idx": abilityIdx,
		})
	}
}
