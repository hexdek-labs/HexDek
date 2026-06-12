package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBreyaEtheriumShaper wires Breya, Etherium Shaper.
//
// Oracle text:
//
//	When Breya enters, create two 1/1 blue Thopter artifact creature
//	tokens with flying.
//	{2}, Sacrifice two artifacts: Choose one —
//	  • Breya deals 3 damage to target player or planeswalker.
//	  • Target creature gets -4/-4 until end of turn.
//	  • You gain 5 life.
//
// Implementation:
//   - ETB: create TWO 1/1 blue Thopter artifact creature tokens with
//     flying. (The auto-generated stub conflated the activated mode 3
//     life gain into ETB and only minted one token; both bugs fixed.)
//   - Activated abilityIdx 0: choose mode via ctx["mode"] (0/1/2). The
//     {2} cost and the two-artifact sacrifice are exercised by the
//     activation pipeline / hat heuristics; if ctx supplies
//     ctx["sacrifice_artifacts"] as a []*Permanent we sacrifice them
//     here as a convenience for direct callers (tests, AI fast paths).
//     Default mode is 2 (gain 5 life — the safest non-target choice)
//     when ctx["mode"] is missing.
//
// Mode targeting:
//   - Mode 0 (3 damage): ctx["target_seat"] (int) or
//     ctx["target_planeswalker"] (*Permanent). Falls back to a random
//     opponent if neither is set.
//   - Mode 1 (-4/-4): ctx["target_perm"] (*Permanent). Falls back to
//     the largest opponent creature.
//   - Mode 2 (gain 5 life): no target.
func registerBreyaEtheriumShaper(r *Registry) {
	r.OnETB("Breya, Etherium Shaper", breyaEtheriumShaperETB)
	r.OwnsETBTrigger("Breya, Etherium Shaper")
	r.OnActivated("Breya, Etherium Shaper", breyaEtheriumShaperActivate)
}

func breyaEtheriumShaperETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "breya_etherium_shaper_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for i := 0; i < 2; i++ {
		token := &gameengine.Card{
			Name:          "Thopter Token",
			Owner:         seat,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "artifact", "creature", "thopter"},
			Colors:        []string{"U"},
			TypeLine:      "Token Artifact Creature — Thopter",
		}
		newPerm := enterBattlefieldWithETB(gs, seat, token, false)
		if newPerm != nil {
			if newPerm.Flags == nil {
				newPerm.Flags = map[string]int{}
			}
			newPerm.Flags["kw:flying"] = 1
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"tokens": 2,
	})
}

func breyaEtheriumShaperActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "breya_etherium_shaper_activated"
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
	// Optional: sacrifice the two artifacts the caller supplied.
	if ctx != nil {
		if sacs, ok := ctx["sacrifice_artifacts"].([]*gameengine.Permanent); ok {
			for _, p := range sacs {
				if p == nil || p.Controller != seat {
					continue
				}
				gameengine.SacrificePermanent(gs, p, "breya_cost")
			}
		}
	}
	mode := 2
	if ctx != nil {
		if m, ok := ctx["mode"].(int); ok {
			mode = m
		}
	}
	switch mode {
	case 0:
		breyaModeDamage(gs, src, ctx)
	case 1:
		breyaModeMinusFour(gs, src, ctx)
	default:
		gameengine.GainLife(gs, seat, 5, src.Card.DisplayName())
		emit(gs, slug+"_gain_life", src.Card.DisplayName(), map[string]interface{}{
			"seat": seat,
			"life": 5,
		})
	}
}

func breyaModeDamage(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "breya_etherium_shaper_damage"
	const dmg = 3
	seat := src.Controller
	// Planeswalker target wins when supplied.
	if ctx != nil {
		if pw, ok := ctx["target_planeswalker"].(*gameengine.Permanent); ok && pw != nil {
			pw.AddCounter("loyalty", -dmg)
			emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
				"seat":         seat,
				"target_pw":    pw.Card.DisplayName(),
				"loyalty_lost": dmg,
			})
			return
		}
	}
	target := -1
	if ctx != nil {
		if t, ok := ctx["target_seat"].(int); ok && t >= 0 && t < len(gs.Seats) {
			target = t
		}
	}
	if target < 0 {
		opps := gs.Opponents(seat)
		if len(opps) == 0 {
			emitFail(gs, slug, src.Card.DisplayName(), "no_target", nil)
			return
		}
		target = opps[0]
	}
	gameengine.LoseLife(gs, target, dmg, src.Card.DisplayName())
	_ = gs.CheckEnd()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"target_seat": target,
		"damage":      dmg,
	})
}

func breyaModeMinusFour(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "breya_etherium_shaper_minus_four"
	seat := src.Controller
	var target *gameengine.Permanent
	if ctx != nil {
		if t, ok := ctx["target_perm"].(*gameengine.Permanent); ok {
			target = t
		}
	}
	if target == nil {
		// Pick the largest opponent creature as a reasonable fallback.
		bestPow := -1
		for _, opp := range gs.Opponents(seat) {
			s := gs.Seats[opp]
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil || !p.IsCreature() {
					continue
				}
				if p.Power() > bestPow {
					bestPow = p.Power()
					target = p
				}
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_creature", nil)
		return
	}
	target.Modifications = append(target.Modifications, gameengine.Modification{
		Power:     -4,
		Toughness: -4,
		Duration:  "until_end_of_turn",
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"target": target.Card.DisplayName(),
	})
}
