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
	// R52 batchM: Saga back-face chapter dispatch via lore_counter_added.
	r.OnTrigger("Elesh Norn // The Argent Etchings", "lore_counter_added", eleshNornEtchingsSagaChapter)
	r.OnTrigger("The Argent Etchings", "lore_counter_added", eleshNornEtchingsSagaChapter)
}

func eleshNornArgentEtchingsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Transformed {
		// Saga chapter dispatch is wired via lore_counter_added below
		// (R52 batchM); ETB on the back face is a no-op breadcrumb.
		emit(gs, "elesh_norn_argent_etchings_saga_etb", perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
		})
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

	gameengine.LoseLife(gs, sourceSeat, 2, perm.Card.DisplayName())
	_ = gs.CheckEnd()
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

// eleshNornEtchingsSagaChapter dispatches the Saga back face chapter
// abilities (CR §714). Wired via lore_counter_added in R52 batchM.
//   I  — Incubate 2 five times, then transform all Incubator tokens.
//        Approximation: create 5 Incubator tokens (2/2 phyrexians via
//        the engine helper) and flip them creature-side immediately.
//   II — Creatures you control get +1/+1 and gain double strike UEOT.
//   III — Destroy all other permanents except artifacts, lands, and
//         Phyrexians; flip Etchings front face up.
func eleshNornEtchingsSagaChapter(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "elesh_norn_etchings_saga_chapter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// r63 self-gate: lore_counter_added walks the whole battlefield; gate on
	// the advancing saga so a second transformed DFC saga doesn't run this
	// chapter effect (nil-tolerant for pre-r63 callers).
	if p, _ := ctx["perm"].(*gameengine.Permanent); p != nil && p != perm {
		return
	}
	if !perm.Transformed {
		return
	}
	chapter, _ := ctx["chapter"].(int)
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	switch chapter {
	case 1:
		for i := 0; i < 5; i++ {
			tok := gameengine.CreateCreatureToken(gs, perm.Controller,
				"Phyrexian Token",
				[]string{"creature", "phyrexian", "incubator", "pip:colorless"}, 2, 2)
			if tok != nil {
				if tok.Flags == nil {
					tok.Flags = map[string]int{}
				}
				tok.Flags["incubated"] = 1
			}
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"chapter": chapter,
			"tokens":  5,
		})
	case 2:
		buffed := 0
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power:     1,
				Toughness: 1,
				Duration:  "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["kw:double_strike"] = 1
			buffed++
		}
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"chapter": chapter,
			"buffed":  buffed,
		})
	case 3:
		destroyed := 0
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			var survivors []*gameengine.Permanent
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil || p == perm {
					survivors = append(survivors, p)
					continue
				}
				if cardHasType(p.Card, "artifact") || cardHasType(p.Card, "land") || cardHasSubtype(p.Card, "phyrexian") {
					survivors = append(survivors, p)
					continue
				}
				gameengine.MoveCard(gs, p.Card, p.Controller, "battlefield", "graveyard", "etchings_chapter_iii_wipe")
				destroyed++
			}
			s.Battlefield = survivors
		}
		gameengine.TransformPermanent(gs, perm, "etchings_chapter_iii_flip_front")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"chapter":   chapter,
			"destroyed": destroyed,
		})
	}
}
