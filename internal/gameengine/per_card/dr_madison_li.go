package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDrMadisonLi wires Dr. Madison Li.
//
// Oracle text:
//
//	Whenever you cast an artifact spell, you get {E} (an energy
//	counter).
//	{T}, Pay {E}: Target creature gets +1/+0 and gains trample and
//	haste until end of turn.
//	{T}, Pay {E}{E}{E}: Draw a card.
//	{T}, Pay {E}{E}{E}{E}{E}: Return target artifact card from your
//	graveyard to the battlefield tapped.
func registerDrMadisonLi(r *Registry) {
	r.OnTrigger("Dr. Madison Li", "spell_cast", drMadisonLiArtifactCast)
	r.OnActivated("Dr. Madison Li", drMadisonLiActivated)
}

func drMadisonLiArtifactCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dr_madison_li_energy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || !cardHasType(card, "artifact") {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["energy"]++
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"energy": seat.Flags["energy"],
	})
}

func drMadisonLiActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "dr_madison_li_activated"
	if gs == nil || src == nil {
		return
	}
	// All three abilities cost {T}.
	if src.Tapped {
		return
	}
	src.Tapped = true
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}

	switch abilityIdx {
	case 0:
		// {T}, Pay {E}: Target creature gets +1/+0 and gains trample and
		// haste until end of turn.
		if seat.Flags["energy"] < 1 {
			emitFail(gs, slug, src.Card.DisplayName(), "insufficient_energy", map[string]interface{}{
				"seat":     src.Controller,
				"required": 1,
				"have":     seat.Flags["energy"],
			})
			return
		}
		target := pickMadisonLiPumpTarget(seat, src, ctx)
		if target == nil {
			emitFail(gs, slug, src.Card.DisplayName(), "no_creature_target", map[string]interface{}{
				"seat": src.Controller,
			})
			return
		}
		seat.Flags["energy"]--
		ts := gs.NextTimestamp()
		target.Modifications = append(target.Modifications, gameengine.Modification{
			Power:     1,
			Duration:  "until_end_of_turn",
			Timestamp: ts,
		})
		if target.Flags == nil {
			target.Flags = map[string]int{}
		}
		grantedTrample := target.Flags["kw:trample"] == 0
		grantedHaste := target.Flags["kw:haste"] == 0
		if grantedTrample {
			target.Flags["kw:trample"] = 1
		}
		if grantedHaste {
			target.Flags["kw:haste"] = 1
		}
		gs.InvalidateCharacteristicsCache()
		captured := target
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: src.Controller,
			SourceCardName: src.Card.DisplayName(),
			EffectFn: func(gs *gameengine.GameState) {
				if captured == nil || captured.Flags == nil {
					return
				}
				if grantedTrample {
					gameengine.ClearKeywordFlags(gs, captured, "trample")
				}
				if grantedHaste {
					gameengine.ClearKeywordFlags(gs, captured, "haste")
				}
			},
		})
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":          src.Controller,
			"ability":       "pump",
			"target":        target.Card.DisplayName(),
			"energy_spent":  1,
			"energy_after":  seat.Flags["energy"],
		})
	case 1:
		// {T}, Pay {E}{E}{E}: Draw a card.
		if seat.Flags["energy"] < 3 {
			emitFail(gs, slug, src.Card.DisplayName(), "insufficient_energy", map[string]interface{}{
				"seat":     src.Controller,
				"required": 3,
				"have":     seat.Flags["energy"],
			})
			return
		}
		seat.Flags["energy"] -= 3
		drawn := 0
		if len(seat.Library) > 0 {
			top := seat.Library[0]
			gameengine.MoveCard(gs, top, src.Controller, "library", "hand", "dr_madison_li_draw")
			drawn = 1
		}
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":         src.Controller,
			"ability":      "draw",
			"energy_spent": 3,
			"energy_after": seat.Flags["energy"],
			"drawn":        drawn,
		})
	case 2:
		// {T}, Pay {E}{E}{E}{E}{E}: Return target artifact card from your
		// graveyard to the battlefield tapped.
		if seat.Flags["energy"] < 5 {
			emitFail(gs, slug, src.Card.DisplayName(), "insufficient_energy", map[string]interface{}{
				"seat":     src.Controller,
				"required": 5,
				"have":     seat.Flags["energy"],
			})
			return
		}
		target := pickMadisonLiRecurArtifact(seat, ctx)
		if target == nil {
			emitFail(gs, slug, src.Card.DisplayName(), "no_artifact_in_graveyard", map[string]interface{}{
				"seat": src.Controller,
			})
			return
		}
		seat.Flags["energy"] -= 5
		gameengine.MoveCard(gs, target, src.Controller, "graveyard", "battlefield", "dr_madison_li_recur")
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":         src.Controller,
			"ability":      "recur_artifact",
			"target":       target.DisplayName(),
			"energy_spent": 5,
			"energy_after": seat.Flags["energy"],
		})
	default:
		emitPartial(gs, slug, src.Card.DisplayName(), "unknown_ability_index")
	}
}

func pickMadisonLiPumpTarget(s *gameengine.Seat, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if ctx != nil {
		if v, ok := ctx["target_perm"].(*gameengine.Permanent); ok && v != nil && v.IsCreature() {
			return v
		}
	}
	// Prefer an attacking creature; fall back to highest power.
	var best *gameengine.Permanent
	bestScore := -1
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		score := p.Power()
		if p.IsAttacking() {
			score += 100
		}
		if score > bestScore {
			bestScore = score
			best = p
		}
	}
	return best
}

func pickMadisonLiRecurArtifact(s *gameengine.Seat, ctx map[string]interface{}) *gameengine.Card {
	if ctx != nil {
		if v, ok := ctx["target_card"].(*gameengine.Card); ok && v != nil && cardHasType(v, "artifact") {
			return v
		}
	}
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range s.Graveyard {
		if c == nil || !cardHasType(c, "artifact") {
			continue
		}
		if cardCMC(c) > bestCMC {
			bestCMC = cardCMC(c)
			best = c
		}
	}
	return best
}
