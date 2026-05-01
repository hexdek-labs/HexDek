package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEleshNornArgentEtchings wires Elesh Norn // The Argent Etchings
// (March of the Machine, transforming legendary creature / saga DFC).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
// Front — Elesh Norn (Legendary Creature — Phyrexian Praetor):
//
//	Vigilance
//	Whenever a source an opponent controls deals damage to you or a
//	permanent you control, that source's controller loses 2 life
//	unless they pay {1}.
//	{2}{W}, Sacrifice three other creatures: Exile Elesh Norn, then
//	return it to the battlefield transformed under its owner's
//	control. Activate only as a sorcery.
//
// Back — The Argent Etchings (Enchantment — Saga):
//
//	(As this Saga enters and after your draw step, add a lore counter.)
//	I — Incubate 2 five times, then transform all Incubator tokens you
//	    control.
//	II — Creatures you control get +1/+1 and gain double strike until
//	     end of turn.
//	III — Destroy all other permanents except for artifacts, lands, and
//	      Phyrexians. Exile this Saga, then return it to the battlefield
//	      (front face up).
//
// Implementation:
//   - Vigilance is wired through the AST keyword pipeline.
//   - "Whenever a source an opponent controls deals damage..." — the
//     engine has no general damage-event trigger today (only
//     combat_damage_player). We approximate by punishing opponents on
//     combat damage to Elesh Norn's controller via the combat damage
//     trigger. Damage to permanents and non-combat damage are not
//     covered; emitPartial flags the gap.
//   - Activated transform — abilityIdx 0 implements the {2}{W}, Sacrifice
//     three creatures cost (sacrifice payment only — mana cost is
//     handled by the activation pipeline) and TransformPermanent. Edge:
//     fewer than three other creatures available → emitFail.
//   - Saga back face chapter abilities — emitPartial; saga lore-counter
//     scheduling and chapter-trigger dispatch isn't expressible without
//     additional engine scaffolding (see terra.go for the same pattern).
//
// DFC dispatch: register all three name forms. perm.Card.Name swaps on
// TransformPermanent and the registry's " // " split fallback only
// catches pre-transform names.
func registerEleshNornArgentEtchings(r *Registry) {
	r.OnETB("Elesh Norn // The Argent Etchings", eleshNornArgentEtchingsETB)
	r.OnETB("Elesh Norn", eleshNornArgentEtchingsETB)
	r.OnETB("The Argent Etchings", eleshNornArgentEtchingsETB)
	r.OnTrigger("Elesh Norn // The Argent Etchings", "combat_damage_player", eleshNornArgentDamagePunish)
	r.OnTrigger("Elesh Norn", "combat_damage_player", eleshNornArgentDamagePunish)
	r.OnActivated("Elesh Norn // The Argent Etchings", eleshNornArgentActivate)
	r.OnActivated("Elesh Norn", eleshNornArgentActivate)
}

func eleshNornArgentEtchingsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Transformed {
		emitPartial(gs, "elesh_norn_argent_etchings_saga", perm.Card.DisplayName(),
			"saga_chapter_abilities_I_II_III_not_dispatched_via_per_card")
		return
	}
	emit(gs, "elesh_norn_argent_etchings_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, "elesh_norn_argent_etchings_damage_trigger", perm.Card.DisplayName(),
		"non_combat_damage_and_damage_to_permanents_not_modeled")
}

func eleshNornArgentDamagePunish(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "elesh_norn_argent_etchings_damage_punish"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if perm.Transformed {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat != perm.Controller {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat == perm.Controller {
		return
	}
	if sourceSeat < 0 || sourceSeat >= len(gs.Seats) {
		return
	}
	src := gs.Seats[sourceSeat]
	if src == nil || src.Lost {
		return
	}

	// "loses 2 life unless they pay {1}" — model the unwilling-payment
	// decision: pay only when the opponent has at least 1 mana floating
	// and isn't already at low life. Otherwise eat the 2 life loss.
	pay := false
	if src.Mana != nil {
		floating := src.Mana.Any + src.Mana.W + src.Mana.U + src.Mana.B + src.Mana.R + src.Mana.G + src.Mana.C
		if floating >= 1 && src.Life > 4 {
			pay = true
		}
	} else if src.ManaPool >= 1 && src.Life > 4 {
		pay = true
	}

	if pay {
		if src.Mana != nil {
			if src.Mana.Any > 0 {
				src.Mana.Any--
			} else if src.Mana.C > 0 {
				src.Mana.C--
			} else if src.Mana.W > 0 {
				src.Mana.W--
			} else if src.Mana.U > 0 {
				src.Mana.U--
			} else if src.Mana.B > 0 {
				src.Mana.B--
			} else if src.Mana.R > 0 {
				src.Mana.R--
			} else if src.Mana.G > 0 {
				src.Mana.G--
			}
			gameengine.SyncManaAfterSpend(src)
		} else if src.ManaPool >= 1 {
			src.ManaPool--
			gameengine.SyncManaAfterSpend(src)
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"source_seat":   sourceSeat,
			"paid":          true,
			"life_lost":     0,
		})
		return
	}

	src.Life -= 2
	gs.LogEvent(gameengine.Event{
		Kind:   "life_change",
		Seat:   sourceSeat,
		Target: -1,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"amount": -2,
			"cause":  "elesh_norn_argent_etchings_punish",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"source_seat": sourceSeat,
		"paid":        false,
		"life_lost":   2,
	})
}

func eleshNornArgentActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "elesh_norn_argent_etchings_transform"
	if gs == nil || src == nil {
		return
	}
	if src.Transformed {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Pick three other creatures to sacrifice. Prefer the worst three
	// (low power+toughness, tokens, summoning-sick).
	victims := chooseEleshNornArgentSacVictims(gs, src, 3)
	if len(victims) < 3 {
		emitFail(gs, slug, src.Card.DisplayName(), "fewer_than_three_other_creatures", map[string]interface{}{
			"seat":      src.Controller,
			"available": len(victims),
		})
		return
	}
	sacced := make([]string, 0, 3)
	for _, v := range victims {
		name := ""
		if v.Card != nil {
			name = v.Card.DisplayName()
		}
		sacced = append(sacced, name)
		gameengine.SacrificePermanent(gs, v, "elesh_norn_argent_etchings_activate")
	}

	if !gameengine.TransformPermanent(gs, src, "elesh_norn_argent_etchings_activate") {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"transform_failed_face_data_missing")
		return
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       src.Controller,
		"sacrificed": sacced,
		"to":         "The Argent Etchings",
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"saga_chapter_abilities_not_dispatched_via_per_card")
}

// chooseEleshNornArgentSacVictims selects up to n creatures to feed the
// transform cost, excluding src itself. Greedily picks the weakest
// bodies first so the Praetor is preserved and engine pieces stay on
// board.
func chooseEleshNornArgentSacVictims(gs *gameengine.GameState, src *gameengine.Permanent, n int) []*gameengine.Permanent {
	if gs == nil || src == nil || n <= 0 {
		return nil
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return nil
	}
	type scored struct {
		p     *gameengine.Permanent
		score int
	}
	var pool []scored
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		s := p.Power() + p.Toughness()
		if cardHasType(p.Card, "token") {
			s -= 100
		}
		if p.SummoningSick {
			s -= 5
		}
		if p.IsLegendary() {
			s += 50
		}
		pool = append(pool, scored{p: p, score: s})
	}
	for i := 0; i < len(pool); i++ {
		for j := i + 1; j < len(pool); j++ {
			if pool[j].score < pool[i].score {
				pool[i], pool[j] = pool[j], pool[i]
			}
		}
	}
	out := make([]*gameengine.Permanent, 0, n)
	for i := 0; i < len(pool) && len(out) < n; i++ {
		out = append(out, pool[i].p)
	}
	return out
}
