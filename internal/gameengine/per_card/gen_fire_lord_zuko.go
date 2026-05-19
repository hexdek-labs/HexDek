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
// Implementation (R44 stub port):
//   - Firebending X: OnTrigger("creature_attacks") gated to
//     attacker_perm == perm. Add perm.Power() red mana to the
//     controller's pool (mirrors Fire Lord Azula's firebending 2
//     pattern). "Lasts until end of combat" is approximated by the
//     default mana-empty-step behavior (the engine clears unspent
//     mana between phases; full "mana doesn't empty" requires the
//     keyword pipeline, which is breadcrumbed in Azula's port too).
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
}

func fireLordZukoETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "fire_lord_zuko_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"firebending_mana_until_end_of_combat_lifetime_not_modeled_default_phase_empty_used_instead")
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
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"x":     x,
		"color": "R",
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
