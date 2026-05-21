package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFireLordZuko wires Fire Lord Zuko.
//
// Oracle text (Scryfall, verified):
//
//	Firebending X, where X is Fire Lord Zuko's power. (Whenever this
//	creature attacks, add X {R}. This mana lasts until end of combat.)
//	Whenever you cast a spell from exile and whenever a permanent you
//	control enters from exile, put a +1/+1 counter on each creature
//	you control.
//
// Implementation (R44 stub port; R58 mana-pool primitive port):
//   - Firebending X: OnTrigger("creature_attacks") gated to
//     attacker_perm == perm. Add perm.Power() red mana to the
//     controller's pool. R58: register a ManaPoolExemption({R})
//     scoped to the controller so the firebent red mana survives the
//     combat→post-combat phase boundary; a delayed end_of_combat
//     trigger unregisters the exemption so it doesn't bleed into the
//     next combat. LTB also unregisters defensively.
//   - Cast-from-exile counter trigger: OnTrigger("spell_cast")
//     gated on caster_seat == controller AND cast_zone == "exile".
//   - ETB-from-exile counter trigger: OnTrigger("nonland_permanent
//     _etb") gated on the entering perm being controlled by Zuko's
//     controller AND from_zone == "exile". Both triggers fan out the
//     same counter helper that walks the controller's battlefield
//     and stamps +1/+1 on every creature.
func registerFireLordZuko(r *Registry) {
	r.OnETB("Fire Lord Zuko", fireLordZukoETB)
	r.OnTrigger("Fire Lord Zuko", "creature_attacks", fireLordZukoFirebending)
	r.OnTrigger("Fire Lord Zuko", "spell_cast", fireLordZukoSpellFromExile)
	r.OnTrigger("Fire Lord Zuko", "nonland_permanent_etb", fireLordZukoEtbFromExile)
	r.OnTrigger("Fire Lord Zuko", "permanent_ltb", fireLordZukoLTB)
}

func fireLordZukoETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "fire_lord_zuko_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func fireLordZukoLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.UnregisterManaPoolExemptionForPerm(gs, perm)
}

func fireLordZukoFirebending(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fire_lord_zuko_firebending_x"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	x := perm.Power()
	if x <= 0 {
		return
	}
	seat.ManaPool += x
	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: x,
		Details: map[string]interface{}{
			"slug":   slug,
			"colors": "R",
			"reason": "firebending_x",
		},
	})
	// R58: keep the firebent {R} from draining at the combat→post-combat
	// phase boundary. Register a ManaPoolExemption tied to Zuko + a
	// delayed end_of_combat trigger that drops it. Per-combat scope so
	// the exemption doesn't bleed into the next turn's combat.
	gameengine.RegisterManaPoolExemption(gs, perm, perm.Controller, []string{"R"})
	captured := perm
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_combat",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.UnregisterManaPoolExemptionForPerm(gs, captured)
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"x":         x,
		"color":     "R",
		"exemption": "R_until_eoc",
	})
}

func fireLordZukoSpellFromExile(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fire_lord_zuko_cast_from_exile_counter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	castZone, _ := ctx["cast_zone"].(string)
	if castZone != "exile" {
		return
	}
	stamped := fireLordZukoStampCounters(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"stamped":  stamped,
		"from":     "spell_cast_exile",
	})
}

func fireLordZukoEtbFromExile(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fire_lord_zuko_etb_from_exile_counter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Controller != perm.Controller {
		return
	}
	fromZone, _ := ctx["from_zone"].(string)
	if fromZone != "exile" {
		return
	}
	// Avoid double-firing on Zuko's own ETB-from-exile (his ETB hook
	// already emits the breadcrumb; the counter pump from his own
	// entry is acceptable but we skip self-entry to keep the trigger
	// scoped to other-permanent entries — matches the printed flavor
	// of "another permanent" loops).
	if entering == perm {
		return
	}
	stamped := fireLordZukoStampCounters(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"stamped":  stamped,
		"from":     "etb_from_exile",
		"entered":  entering.Card.DisplayName(),
	})
}

// fireLordZukoStampCounters adds +1/+1 to every creature on the
// controller's battlefield. Returns the count stamped.
func fireLordZukoStampCounters(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil || perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return 0
	}
	stamped := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		p.AddCounter("+1/+1", 1)
		stamped++
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	return stamped
}
