package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrtetRemnantOfMemnarch wires Urtet, Remnant of Memnarch.
//
// Oracle text:
//
//	{3}
//	Legendary Artifact Creature — Myr
//	Whenever you cast a Myr spell, create a 1/1 colorless Myr artifact
//	  creature token.
//	At the beginning of combat on your turn, untap each Myr you control.
//	{W}{U}{B}{R}{G}, {T}: Put three +1/+1 counters on each Myr you control.
//	  Activate only during your turn.
//
// Implementation:
//   - "spell_cast" trigger gated to caster == controller and spell card
//     has type "myr": create a 1/1 colorless Myr artifact creature token.
//   - "begin_combat_controller" trigger gated to active_seat ==
//     controller: untap each Myr the controller controls.
//   - Activated WUBRG ability (R60 batch 8): tap Urtet, put 3 +1/+1
//     counters on each Myr we control. Mana cost ({W}{U}{B}{R}{G}) is
//     enforced upstream by the activation cost pipeline; tap cost is
//     enforced inline. Gates on gs.Active == src.Controller per
//     "Activate only during your turn."
func registerUrtetRemnantOfMemnarch(r *Registry) {
	r.OnTrigger("Urtet, Remnant of Memnarch", "spell_cast", urtetMyrCast)
	r.OnTrigger("Urtet, Remnant of Memnarch", "begin_combat_controller", urtetBeginCombat)
	r.OnActivated("Urtet, Remnant of Memnarch", urtetActivate)
}

func urtetActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "urtet_wubrg_myr_counters"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if gs.Active != src.Controller {
		emitFail(gs, slug, src.Card.DisplayName(), "not_your_turn", map[string]interface{}{
			"active_seat":     gs.Active,
			"controller_seat": src.Controller,
		})
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	src.Tapped = true
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "myr") {
			continue
		}
		p.AddCounter("+1/+1", 3)
		stamped++
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            src.Controller,
		"myrs_buffed":     stamped,
		"counters_per":    3,
	})
}

func urtetMyrCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "urtet_myr_cast_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "myr") {
		return
	}
	token := &gameengine.Card{
		Name:          "Myr Token",
		Owner:         perm.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "artifact", "creature", "myr"},
		Colors:        []string{},
		TypeLine:      "Token Artifact Creature — Myr",
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func urtetBeginCombat(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "urtet_begin_combat_untap_myr"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	untapped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "myr") {
			continue
		}
		if p.Tapped {
			p.Tapped = false
			untapped++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"untapped": untapped,
	})
}
