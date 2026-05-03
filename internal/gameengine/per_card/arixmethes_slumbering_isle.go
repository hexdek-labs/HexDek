package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArixmethesSlumberingIsle wires Arixmethes, Slumbering Isle.
//
// Oracle text (Scryfall):
//
//	Arixmethes enters tapped with five slumber counters on it.
//	As long as Arixmethes has a slumber counter on it, it's a land.
//	(It's not a creature.)
//	Whenever you cast a spell, you may remove a slumber counter from
//	Arixmethes.
//	{T}: Add {G}{U}.
//
// Implementation notes:
//
//   - OnETB: sets Tapped = true and Flags["slumber_counters"] = 5.
//     Removes "creature" from Card.Types and adds "land" so the engine's
//     IsLand/IsCreature predicates reflect the correct in-play state.
//     EmitPartial flags that the full §613 layer-4 continuous type effect
//     is approximated via direct Card.Types mutation rather than a
//     registered ContinuousEffect — accurate enough for simulation but
//     not wired into the formal layer-dependency resolver.
//
//   - OnTrigger("spell_cast"): fires when the controller casts any spell.
//     Decrements Flags["slumber_counters"] by 1. When the count reaches
//     zero, "creature" is restored to Card.Types (Arixmethes wakes up).
//
//   - OnActivated(0): {T}: Add {G}{U}. Usable in both the slumbering
//     (land-only) and awake (creature+land) states because the mana
//     ability is printed on the card in both. Guards against tapped
//     permanents and missing seat bounds.
func registerArixmethesSlumberingIsle(r *Registry) {
	r.OnETB("Arixmethes, Slumbering Isle", arixmethesETB)
	r.OnTrigger("Arixmethes, Slumbering Isle", "spell_cast", arixmethesSpellCast)
	r.OnActivated("Arixmethes, Slumbering Isle", arixmethesActivate)
}

func arixmethesETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "arixmethes_slumbering_isle_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller

	// Enters the battlefield tapped.
	perm.Tapped = true

	// Initialise the slumber-counter flag map.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["slumber_counters"] = 5

	// Continuous type effect (layer 4 approximation via Card.Types mutation):
	// "As long as Arixmethes has a slumber counter on it, it's a land. (It's
	// not a creature.)"
	//
	// Remove "creature" from the printed types and ensure "land" is present
	// so that IsLand() returns true and IsCreature() returns false while the
	// counters are present.
	arixmethesApplySlumbering(perm, true)
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            seat,
		"slumber_counters": 5,
		"tapped":          true,
	})

	// The layer-4 continuous type effect is approximated via direct
	// Card.Types mutation rather than a registered ContinuousEffect entry;
	// flag this so the audit can track the gap.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"layer_4_continuous_type_effect_not_registered_in_ce_table")
}

func arixmethesSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "arixmethes_slumbering_isle_spell_cast"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if ctx == nil {
		return
	}

	// "Whenever YOU cast a spell" — only triggers for the controller.
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	counters := perm.Flags["slumber_counters"]
	if counters <= 0 {
		// Already awake; nothing to remove.
		return
	}

	counters--
	perm.Flags["slumber_counters"] = counters

	if counters == 0 {
		// All slumber counters removed — Arixmethes wakes up and becomes
		// a creature again (and remains a land).
		arixmethesApplySlumbering(perm, false)
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":            perm.Controller,
			"slumber_counters": 0,
			"woke_up":         true,
		})
		return
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"slumber_counters": counters,
		"woke_up":         false,
	})
}

func arixmethesActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "arixmethes_slumbering_isle_tap_mana"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	// Ability index 0 is the only registered ability: {T}: Add {G}{U}.
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
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

	src.Tapped = true
	gameengine.AddManaFromPermanent(gs, s, src, "G", 1)
	gameengine.AddManaFromPermanent(gs, s, src, "U", 1)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            seat,
		"added_G":         1,
		"added_U":         1,
		"slumber_counters": src.Flags["slumber_counters"],
		"new_pool":        s.ManaPool,
	})
}

// arixmethesApplySlumbering mutates perm.Card.Types to reflect the
// slumbering (slumbeuring=true) or awake (slumbering=false) state.
//
//   - slumbering=true:  remove "creature", ensure "land" is present.
//   - slumbering=false: add "creature" back; keep "land" so the mana
//     ability continues to work and the permanent is correctly
//     categorised as a creature land.
func arixmethesApplySlumbering(perm *gameengine.Permanent, slumbering bool) {
	if perm == nil || perm.Card == nil {
		return
	}

	if slumbering {
		// Remove "creature"; add "land" if absent.
		filtered := perm.Card.Types[:0]
		for _, t := range perm.Card.Types {
			if t != "creature" {
				filtered = append(filtered, t)
			}
		}
		perm.Card.Types = filtered
		if !cardHasType(perm.Card, "land") {
			perm.Card.Types = append(perm.Card.Types, "land")
		}
	} else {
		// Restore "creature" if it was removed; keep everything else.
		if !cardHasType(perm.Card, "creature") {
			perm.Card.Types = append(perm.Card.Types, "creature")
		}
	}
}
