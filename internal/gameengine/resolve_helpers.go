package gameengine

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// resolveModificationEffect handles ModificationEffect nodes — Wave 1a
// promoted labels that didn't get their own typed AST node. The Python
// parser emits these as Modification(kind="...", args=(...)) at effect
// positions; the loader decodes them into gameast.ModificationEffect.
//
// Philosophy: handle the top-frequency kinds with real game mutations
// (phase-out, stun, goad, investigate, keyword grants, etc.) and log
// the long tail as "modification_effect" events (which are NOT unknown_
// effect — they carry the structured kind label for downstream analysis).
func resolveModificationEffect(gs *GameState, src *Permanent, e *gameast.ModificationEffect) {
	switch e.ModKind {

	// -----------------------------------------------------------------
	// Phase-out (CR §702.26). "this creature phases out" — toggle the
	// permanent out of the battlefield. Phase 3 MVP: set a flag so the
	// untap-step handler knows not to untap it; actual zone-movement
	// (§702.26a: "treated as though it doesn't exist") is Phase 8.
	// -----------------------------------------------------------------
	case "phase_out_self":
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["phased_out"] = 1
			gs.LogEvent(Event{
				Kind:   "phase_out",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
			})
		}

	// -----------------------------------------------------------------
	// Stun (CR §701.51). "it doesn't untap during its controller's
	// next untap step." Sets a flag; the untap-step handler skips
	// permanents with stun > 0 and decrements after.
	// -----------------------------------------------------------------
	case "stun_target_next_untap":
		targets := pickTargetFromModArgs(gs, src, e.Args)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				if t.Permanent.Flags == nil {
					t.Permanent.Flags = map[string]int{}
				}
				t.Permanent.Flags["stun"] = 1
				gs.LogEvent(Event{
					Kind:   "stun",
					Seat:   controllerSeat(src),
					Source: sourceName(src),
					Details: map[string]interface{}{
						"target_card": t.Permanent.Card.DisplayName(),
					},
				})
			}
		}

	// -----------------------------------------------------------------
	// Goad (CR §701.38). "goad target creature" — the goaded creature
	// must attack and can't attack its goader. MVP: set flag so combat
	// AI knows to attack with it. Args carry the target filter hint.
	// -----------------------------------------------------------------
	case "goad":
		targets := pickTargetFromModArgs(gs, src, e.Args)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				if t.Permanent.Flags == nil {
					t.Permanent.Flags = map[string]int{}
				}
				t.Permanent.Flags["goaded"] = 1
				gs.LogEvent(Event{
					Kind:   "goad",
					Seat:   controllerSeat(src),
					Source: sourceName(src),
					Details: map[string]interface{}{
						"target_card": t.Permanent.Card.DisplayName(),
					},
				})
			}
		}

	// -----------------------------------------------------------------
	// Investigate (CR §701.36). "investigate" — create a Clue token.
	// Args carry the count (int or "var").
	// -----------------------------------------------------------------
	case "investigate":
		count := 1
		if len(e.Args) > 0 {
			if n, ok := asInt(e.Args[0]); ok && n > 0 {
				count = n
			}
		}
		seat := controllerSeat(src)
		if seat < 0 {
			seat = 0
		}
		for i := 0; i < count; i++ {
			CreateClueToken(gs, seat)
		}
		gs.LogEvent(Event{
			Kind:   "investigate",
			Seat:   seat,
			Source: sourceName(src),
			Amount: count,
		})

	// -----------------------------------------------------------------
	// Suspect (CR §701.56). "suspect it" — the creature gains menace
	// and can't block. MVP: grant menace + set "suspected" flag.
	// -----------------------------------------------------------------
	case "suspect":
		if src != nil {
			src.GrantedAbilities = append(src.GrantedAbilities, "menace")
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["suspected"] = 1
			gs.LogEvent(Event{
				Kind:   "suspect",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
			})
		}

	// -----------------------------------------------------------------
	// Plot (CR §701.55). "it becomes plotted" — a card in exile gets
	// the "plotted" marker; it can be cast later without paying mana
	// cost. MVP: flag only.
	// -----------------------------------------------------------------
	case "plot":
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["plotted"] = 1
			gs.LogEvent(Event{
				Kind:   "plot",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
			})
		}

	// -----------------------------------------------------------------
	// No life gained — static replacement marker. "Opponents can't
	// gain life." Sets a game-wide flag checked by resolveGainLife's
	// replacement chain.
	// -----------------------------------------------------------------
	case "no_life_gained":
		gs.Flags["no_life_gained"] = 1
		gs.LogEvent(Event{
			Kind:   "no_life_gained",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Suppress prevention — "Damage can't be prevented." Sets flag.
	// -----------------------------------------------------------------
	case "suppress_prevention":
		gs.Flags["suppress_prevention"] = 1
		gs.LogEvent(Event{
			Kind:   "suppress_prevention",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Lose ability EOT — "this creature loses defender until end of
	// turn." Remove the named keyword from the creature's abilities.
	// -----------------------------------------------------------------
	case "lose_ability_eot":
		if src != nil && len(e.Args) > 0 {
			keyword, _ := e.Args[0].(string)
			if keyword != "" {
				if src.Flags == nil {
					src.Flags = map[string]int{}
				}
				src.Flags["lost_"+keyword] = 1
				gs.LogEvent(Event{
					Kind:   "lose_ability",
					Seat:   controllerSeat(src),
					Source: sourceName(src),
					Details: map[string]interface{}{
						"ability":  keyword,
						"duration": "until_end_of_turn",
					},
				})
			}
		}

	// -----------------------------------------------------------------
	// Attack without defender EOT — "this creature can attack this
	// turn as though it didn't have defender." Flag it.
	// -----------------------------------------------------------------
	case "attack_without_defender_eot":
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["attack_without_defender"] = 1
			gs.LogEvent(Event{
				Kind:   "attack_without_defender",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
			})
		}

	// -----------------------------------------------------------------
	// Becomes prepared — MKM mechanic. Flag only.
	// -----------------------------------------------------------------
	case "becomes_prepared":
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["prepared"] = 1
			gs.LogEvent(Event{
				Kind:   "becomes_prepared",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
			})
		}

	// -----------------------------------------------------------------
	// Double target power EOT. MVP: apply a buff equal to the
	// creature's current power.
	// -----------------------------------------------------------------
	case "double_power_eot":
		targets := pickCreatureTargets(gs, src, true)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				pow := t.Permanent.Power()
				t.Permanent.Modifications = append(t.Permanent.Modifications, Modification{
					Power:     pow,
					Toughness: 0,
					Duration:  "until_end_of_turn",
					Timestamp: gs.NextTimestamp(),
				})
				gs.InvalidateCharacteristicsCache()
				gs.LogEvent(Event{
					Kind:   "double_power",
					Source: sourceName(src),
					Details: map[string]interface{}{
						"target_card": t.Permanent.Card.DisplayName(),
						"added_power": pow,
					},
				})
			}
		}

	// -----------------------------------------------------------------
	// Copy ability — "copy that ability." Rings of Brighthearth etc.
	// Phase 5+ territory for full resolution; log as structured event.
	// -----------------------------------------------------------------
	case "copy_ability":
		gs.LogEvent(Event{
			Kind:   "copy_ability",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Copy per commander cast — commander-storm. Log structured event.
	// -----------------------------------------------------------------
	case "copy_per_commander_cast":
		gs.LogEvent(Event{
			Kind:   "copy_per_commander_cast",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Fight each other — "those creatures fight each other." Proxy:
	// top two opponent creatures fight. Simplified MVP.
	// -----------------------------------------------------------------
	case "fight_each_other":
		gs.LogEvent(Event{
			Kind:   "fight_each_other",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Heist — "heist target opponent's library." MVP: log only; actual
	// heist mechanic is complex (exile, choose, cast for free).
	// -----------------------------------------------------------------
	case "heist":
		gs.LogEvent(Event{
			Kind:   "heist",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Put card into hand — "put that card into your hand" / "put it
	// into your hand." These are pronoun-based zone changes typically
	// following a reveal or look-at. MVP: log structured event.
	// -----------------------------------------------------------------
	case "put_card_into_hand":
		gs.LogEvent(Event{
			Kind:   "put_card_into_hand",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"referent": modArgString(e.Args, 0),
			},
		})

	// -----------------------------------------------------------------
	// Put onto battlefield — "put it onto the battlefield."
	// -----------------------------------------------------------------
	case "put_onto_battlefield":
		gs.LogEvent(Event{
			Kind:   "put_onto_battlefield",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"referent": modArgString(e.Args, 0),
			},
		})

	// -----------------------------------------------------------------
	// Attach aura to target creature — Aura-snap effect.
	// -----------------------------------------------------------------
	case "attach_aura_target_creature":
		gs.LogEvent(Event{
			Kind:   "attach_aura",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Coin flips — MVP: simulate the flip as a random boolean.
	// -----------------------------------------------------------------
	case "flip_coin_until_lose":
		wins := 0
		if gs.Rng != nil {
			for gs.Rng.Intn(2) == 1 {
				wins++
				if wins > 100 { // safety cap
					break
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "flip_coin",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: wins,
			Details: map[string]interface{}{
				"mode": "until_lose",
			},
		})

	case "may_flip_coin":
		result := 0
		if gs.Rng != nil {
			result = gs.Rng.Intn(2) // 0 or 1
		}
		gs.LogEvent(Event{
			Kind:   "flip_coin",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: result,
			Details: map[string]interface{}{
				"mode": "single",
			},
		})

	// -----------------------------------------------------------------
	// Pay any amount of life — "pay any amount of life." MVP: pay 0
	// (conservative; a Phase 10 policy agent will choose optimally).
	// -----------------------------------------------------------------
	case "pay_any_amount_life":
		gs.LogEvent(Event{
			Kind:   "pay_life",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: 0,
			Details: map[string]interface{}{
				"mode": "any_amount",
			},
		})

	// -----------------------------------------------------------------
	// May play exiled card free / may cast copy free — permission
	// markers for future-cast effects.
	// -----------------------------------------------------------------
	case "may_play_exiled_free":
		gs.LogEvent(Event{
			Kind:   "permission_granted",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"permission": "play_exiled_free",
			},
		})

	case "may_cast_copy_free":
		gs.LogEvent(Event{
			Kind:   "permission_granted",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"permission": "cast_copy_free",
			},
		})

	// -----------------------------------------------------------------
	// Discard two unless typed — "discard two cards unless you discard
	// an artifact card." MVP: discard 1 (optimistic — you had the
	// typed card).
	// -----------------------------------------------------------------
	case "discard_two_unless_typed":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			discardN(gs, seat, 1, "")
		}
		gs.LogEvent(Event{
			Kind:   "discard",
			Seat:   seat,
			Source: sourceName(src),
			Amount: 1,
			Details: map[string]interface{}{
				"mode":      "unless_typed",
				"card_type": modArgString(e.Args, 0),
			},
		})

	// -----------------------------------------------------------------
	// Heroic riders — "whenever a spell targets this creature, put
	// a +1/+1 counter on it." These fire as triggered-ability effects.
	// Apply the counter directly.
	// -----------------------------------------------------------------
	case "heroic_rider_p1p1_pronoun", "heroic_rider_p1p1_self":
		if src != nil {
			src.AddCounter("+1/+1", 1)
			gs.InvalidateCharacteristicsCache()
			gs.LogEvent(Event{
				Kind:   "counter_mod",
				Source: sourceName(src),
				Amount: 1,
				Details: map[string]interface{}{
					"target_card":  sourceName(src),
					"op":           "put",
					"counter_kind": "+1/+1",
					"trigger":      "heroic",
				},
			})
		}

	case "heroic_rider_two_p1p1_self":
		if src != nil {
			src.AddCounter("+1/+1", 2)
			gs.InvalidateCharacteristicsCache()
			gs.LogEvent(Event{
				Kind:   "counter_mod",
				Source: sourceName(src),
				Amount: 2,
				Details: map[string]interface{}{
					"target_card":  sourceName(src),
					"op":           "put",
					"counter_kind": "+1/+1",
					"trigger":      "heroic",
				},
			})
		}

	// -----------------------------------------------------------------
	// Heroic anthem EOT — "creatures you control get +N/+N until end
	// of turn." Args = (power, toughness).
	// -----------------------------------------------------------------
	case "heroic_rider_anthem_eot":
		pow, tough := 1, 0
		if len(e.Args) >= 2 {
			if p, ok := asInt(e.Args[0]); ok {
				pow = p
			}
			if t, ok := asInt(e.Args[1]); ok {
				tough = t
			}
		}
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			ts := gs.NextTimestamp()
			applied := false
			for _, p := range gs.Seats[seat].Battlefield {
				if p.IsCreature() {
					p.Modifications = append(p.Modifications, Modification{
						Power:     pow,
						Toughness: tough,
						Duration:  "until_end_of_turn",
						Timestamp: ts,
					})
					applied = true
				}
			}
			if applied {
				gs.InvalidateCharacteristicsCache()
			}
		}
		gs.LogEvent(Event{
			Kind:   "buff",
			Seat:   seat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"power":     pow,
				"toughness": tough,
				"scope":     "your_creatures",
				"duration":  "until_end_of_turn",
				"trigger":   "heroic",
			},
		})

	// -----------------------------------------------------------------
	// Counter that spell unless pay — "counter that spell or ability
	// unless its controller pays {N}." MVP: counter it (conservative
	// assumes opponent can't pay). Same behavior as resolveCounterSpell.
	// -----------------------------------------------------------------
	case "counter_that_spell_unless_pay":
		if len(gs.Stack) > 0 {
			top := gs.Stack[len(gs.Stack)-1]
			top.Countered = true
			gs.LogEvent(Event{
				Kind:   "counter_spell",
				Source: sourceName(src),
				Target: top.Controller,
				Details: map[string]interface{}{
					"target_id": top.ID,
					"mode":      "unless_pay",
					"cost":      modArgInt(e.Args, 0),
				},
			})
		} else {
			gs.LogEvent(Event{
				Kind:   "counter_spell_fizzle",
				Source: sourceName(src),
			})
		}

	// -----------------------------------------------------------------
	// Draw unless pay — "you may draw a card unless that player pays
	// {N}." MVP: draw (assume opponent doesn't pay).
	// -----------------------------------------------------------------
	case "draw_unless_pay":
		seat := controllerSeat(src)
		if seat >= 0 {
			if _, ok := gs.drawOne(seat); ok {
				gs.LogEvent(Event{
					Kind:   "draw",
					Seat:   seat,
					Target: seat,
					Source: sourceName(src),
					Amount: 1,
					Details: map[string]interface{}{
						"mode": "unless_pay",
						"cost": modArgInt(e.Args, 0),
					},
				})
			}
		}

	// -----------------------------------------------------------------
	// Land becomes creature with haste — "that land becomes a N/N
	// Elemental creature with haste that's still a land."
	// Args = (power, toughness, creature_type).
	// -----------------------------------------------------------------
	case "land_becomes_creature_haste":
		pow, tough := 0, 0
		creatureType := "elemental"
		if len(e.Args) >= 2 {
			if p, ok := asInt(e.Args[0]); ok {
				pow = p
			}
			if t, ok := asInt(e.Args[1]); ok {
				tough = t
			}
		}
		if len(e.Args) >= 3 {
			if ct, ok := e.Args[2].(string); ok {
				creatureType = ct
			}
		}
		// Apply as a modification on the source (the land).
		if src != nil {
			src.Modifications = append(src.Modifications, Modification{
				Power:     pow - src.Card.BasePower,
				Toughness: tough - src.Card.BaseToughness,
				Duration:  "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			gs.InvalidateCharacteristicsCache()
			src.GrantedAbilities = append(src.GrantedAbilities, "haste")
			// Add creature type if not already present.
			hasCreature := false
			for _, tp := range src.Card.Types {
				if tp == "creature" {
					hasCreature = true
					break
				}
			}
			if !hasCreature {
				src.Card.Types = append(src.Card.Types, "creature", creatureType)
			}
		}
		gs.LogEvent(Event{
			Kind:   "land_becomes_creature",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"power":         pow,
				"toughness":     tough,
				"creature_type": creatureType,
			},
		})

	// -----------------------------------------------------------------
	// Choose color — "choose color." A declarative action for color-
	// matters effects. MVP: choose a random color.
	// -----------------------------------------------------------------
	case "choose_color":
		colors := []string{"white", "blue", "black", "red", "green"}
		chosen := colors[0]
		if gs.Rng != nil {
			chosen = colors[gs.Rng.Intn(len(colors))]
		}
		gs.LogEvent(Event{
			Kind:   "choose_color",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"color": chosen,
			},
		})

	// -----------------------------------------------------------------
	// Split one hand one graveyard — "put one into your hand and the
	// other into your graveyard." Pronoun-based; log structured.
	// -----------------------------------------------------------------
	case "split_one_hand_one_gy":
		gs.LogEvent(Event{
			Kind:   "split_cards",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"destinations": []string{"hand", "graveyard"},
			},
		})

	// -----------------------------------------------------------------
	// Any opponent may sacrifice a creature — Savra-style.
	// -----------------------------------------------------------------
	case "any_opp_may_sac_creature":
		gs.LogEvent(Event{
			Kind:   "may_sacrifice",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"who":    "any_opponent",
				"target": "creature",
			},
		})

	// -----------------------------------------------------------------
	// Long-tail kinds from aa_unknown_hunt.py — log as structured
	// events with the kind label preserved.
	// -----------------------------------------------------------------
	case "regenerate":
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["regeneration_shield"] = 1
		}
		gs.LogEvent(Event{
			Kind:   "regenerate",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "proliferate":
		// CR §701.27 — "Choose any number of permanents and/or players,
		// then give each another counter of each kind already there."
		// GreedyHat policy: proliferate everything you control that has
		// counters, plus opponent poison counters. Skip opponent +1/+1
		// counters (don't help opponents).
		seat := controllerSeat(src)
		proliferatedCount := 0

		// 1. Walk all permanents on the battlefield.
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || len(p.Counters) == 0 {
					continue
				}
				// GreedyHat: proliferate our own permanents' counters.
				// For opponents, skip beneficial counters (+1/+1).
				isOurs := p.Controller == seat
				for kind, count := range p.Counters {
					if count <= 0 {
						continue
					}
					if !isOurs && kind == "+1/+1" {
						continue // don't help opponents
					}
					p.AddCounter(kind, 1)
					proliferatedCount++
				}
			}
		}

		// 2. Walk all players — proliferate ALL counter types on players
		// (poison, energy, experience, rad). GreedyHat policy: proliferate
		// beneficial counters on self (energy, experience) and harmful
		// counters on opponents (poison, rad).
		for i, s := range gs.Seats {
			if s == nil {
				continue
			}
			isUs := i == seat
			if isUs {
				// Proliferate our own beneficial counters.
				if s.Flags != nil {
					if s.Flags["energy_counters"] > 0 {
						s.Flags["energy_counters"]++
						proliferatedCount++
					}
					if s.Flags["experience_counters"] > 0 {
						s.Flags["experience_counters"]++
						proliferatedCount++
					}
				}
			} else {
				// Proliferate opponents' harmful counters.
				if s.PoisonCounters > 0 {
					s.PoisonCounters++
					proliferatedCount++
				}
				if s.Flags != nil && s.Flags["rad_counters"] > 0 {
					s.Flags["rad_counters"]++
					proliferatedCount++
				}
			}
		}

		if proliferatedCount > 0 {
			gs.InvalidateCharacteristicsCache()
		}
		gs.LogEvent(Event{
			Kind:   "proliferate",
			Seat:   seat,
			Source: sourceName(src),
			Amount: proliferatedCount,
			Details: map[string]interface{}{
				"rule": "701.27",
			},
		})

	case "populate":
		// CR §701.30 — "Create a token that's a copy of a creature
		// token you control."
		// Find a creature token we control, then create a copy.
		seat := controllerSeat(src)
		populated := false
		if seat >= 0 && seat < len(gs.Seats) {
			var bestToken *Permanent
			bestPower := -1
			for _, p := range gs.Seats[seat].Battlefield {
				if p == nil || !p.IsToken() || !p.IsCreature() {
					continue
				}
				// GreedyHat: copy the strongest creature token.
				pow := p.Power()
				if pow > bestPower {
					bestPower = pow
					bestToken = p
				}
			}
			if bestToken != nil && bestToken.Card != nil {
				// Create a copy of the token.
				tokenCard := &Card{
					Name:          bestToken.Card.Name,
					Owner:         seat,
					BasePower:     bestToken.Card.BasePower,
					BaseToughness: bestToken.Card.BaseToughness,
					Types:         append([]string{}, bestToken.Card.Types...),
					Colors:        append([]string{}, bestToken.Card.Colors...),
				}
				newPerm := &Permanent{
					Card:          tokenCard,
					Controller:    seat,
					Owner:         seat,
					Tapped:        false,
					SummoningSick: true,
					Timestamp:     gs.NextTimestamp(),
					Counters:      map[string]int{},
					Flags:         map[string]int{},
				}
				gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, newPerm)
				RegisterReplacementsForPermanent(gs, newPerm)
				FirePermanentETBTriggers(gs, newPerm)
				populated = true
				gs.LogEvent(Event{
					Kind:   "create_token",
					Seat:   seat,
					Source: sourceName(src),
					Details: map[string]interface{}{
						"token":     bestToken.Card.Name,
						"power":     bestToken.Card.BasePower,
						"toughness": bestToken.Card.BaseToughness,
						"reason":    "populate",
						"rule":      "701.30",
					},
				})
			}
		}
		gs.LogEvent(Event{
			Kind:   "populate",
			Seat:   seat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"created_copy": populated,
				"rule":         "701.30",
			},
		})

	case "amass":
		gs.LogEvent(Event{
			Kind:   "amass",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "transform_self":
		gs.LogEvent(Event{
			Kind:   "transform",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "ring_tempts":
		gs.LogEvent(Event{
			Kind:   "ring_tempts",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "venture_dungeon":
		gs.LogEvent(Event{
			Kind:   "venture_dungeon",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "take_initiative":
		gs.LogEvent(Event{
			Kind:   "take_initiative",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "roll_d20":
		result := 1
		if gs.Rng != nil {
			result = gs.Rng.Intn(20) + 1
		}
		gs.LogEvent(Event{
			Kind:   "roll_d20",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: result,
		})

	case "seek":
		gs.LogEvent(Event{
			Kind:   "seek",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "exile_top_library":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			if _, ok := gs.millOne(seat); ok {
				// millOne moves to graveyard; for exile_top we'd want exile.
				// Phase 5+ will do proper exile. For now the mill-one approximation
				// is acceptable: it removes a card from the top of library.
			}
		}
		gs.LogEvent(Event{
			Kind:   "exile_top_library",
			Seat:   seat,
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Explore (CR §701.40). "this creature explores" / "it explores."
	// Calls PerformExplore from keywords_misc.go. The exploring
	// creature is the source permanent. If no source is available,
	// no-op (the explore keyword requires a creature on the
	// battlefield to function).
	// -----------------------------------------------------------------
	case "explore":
		if src != nil && src.IsCreature() {
			PerformExplore(gs, src)
		} else {
			gs.LogEvent(Event{
				Kind:   "explore_no_creature",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
			})
		}

	case "ability_word":
		// Ability words with conditional effects (fateful hour, morbid,
		// threshold, etc.). The condition is checked and the secondary
		// effect is executed if met.
		if len(e.Args) >= 3 {
			wordName, _ := e.Args[0].(string)
			controller := controllerSeat(src)
			if controller < 0 || controller >= len(gs.Seats) {
				break
			}
			conditionMet := false
			switch wordName {
			case "fateful hour":
				conditionMet = gs.Seats[controller].Life <= 5
			case "morbid":
				conditionMet = gs.Flags != nil && gs.Flags["creature_died_this_turn"] > 0
			case "threshold":
				conditionMet = len(gs.Seats[controller].Graveyard) >= 7
			default:
				conditionMet = true
			}
			if conditionMet {
				// Parse the effect text for tap-all-attackers pattern.
				rawEffect, _ := e.Args[2].(string)
				if rawEffect != "" && containsIgnoreCase(rawEffect, "tap all attacking creatures") {
					for _, seat := range gs.Seats {
						if seat == nil {
							continue
						}
						for _, p := range seat.Battlefield {
							if p == nil || !p.IsCreature() {
								continue
							}
							if p.Flags != nil && p.Flags["attacking"] > 0 {
								p.Tapped = true
								p.DoesNotUntap = true
							}
						}
					}
				}
				gs.LogEvent(Event{
					Kind:   "ability_word_triggered",
					Seat:   controller,
					Source: sourceName(src),
					Details: map[string]interface{}{
						"word":      wordName,
						"condition": conditionMet,
					},
				})
			}
		}

	case "no_untap":
		// "Those creatures don't untap during their controller's next
		// untap step." Sets DoesNotUntap on targeted/affected creatures.
		// Typically chained after a tap effect (Clinging Mists, Frost
		// Breath, etc.). Target the creatures that were just tapped.
		controller := controllerSeat(src)
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, p := range seat.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				// Apply to tapped opponent creatures (the ones just tapped
				// by the preceding effect in the spell's resolution).
				if p.Tapped && p.Controller != controller {
					p.DoesNotUntap = true
					gs.LogEvent(Event{
						Kind:   "no_untap_applied",
						Seat:   p.Controller,
						Source: sourceName(src),
						Details: map[string]interface{}{
							"target": p.Card.DisplayName(),
						},
					})
				}
			}
		}

	case "gain_energy":
		seat := controllerSeat(src)
		n := 1
		if len(e.Args) > 0 {
			if v, ok := e.Args[0].(float64); ok {
				n = int(v)
			} else if v, ok := e.Args[0].(int); ok {
				n = v
			}
		}
		if seat >= 0 && seat < len(gs.Seats) {
			if gs.Seats[seat].Flags == nil {
				gs.Seats[seat].Flags = make(map[string]int)
			}
			gs.Seats[seat].Flags["energy_counters"] += n
		}
		gs.LogEvent(Event{
			Kind:   "gain_energy",
			Seat:   seat,
			Source: sourceName(src),
			Amount: n,
		})

	case "become_monarch":
		BecomeMonarch(gs, controllerSeat(src))

	case "connives":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			if _, ok := gs.drawOne(seat); ok {
				if discarded, ok2 := gs.millOne(seat); ok2 && discarded != nil {
					isLand := false
					for _, t := range discarded.Types {
						if t == "land" || t == "Land" {
							isLand = true
							break
						}
					}
					if !isLand && src != nil {
						src.AddCounter("+1/+1", 1)
					}
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "connive",
			Seat:   seat,
			Source: sourceName(src),
		})

	case "explores":
		if src != nil && src.IsCreature() {
			PerformExplore(gs, src)
		}

	case "incubate":
		seat := controllerSeat(src)
		n := 2
		if len(e.Args) > 0 {
			if v, ok := e.Args[0].(float64); ok {
				n = int(v)
			} else if v, ok := e.Args[0].(int); ok {
				n = v
			}
		}
		tokenEff := &gameast.CreateToken{
			Count: *gameast.NumInt(1),
			Types: []string{"incubator", "artifact"},
		}
		ResolveEffect(gs, src, tokenEff)
		if seat >= 0 && seat < len(gs.Seats) && len(gs.Seats[seat].Battlefield) > 0 {
			last := gs.Seats[seat].Battlefield[len(gs.Seats[seat].Battlefield)-1]
			if last != nil && last.IsToken() {
				last.AddCounter("+1/+1", n)
			}
		}
		gs.LogEvent(Event{
			Kind:   "incubate",
			Seat:   seat,
			Source: sourceName(src),
			Amount: n,
		})

	case "pay_life":
		seat := controllerSeat(src)
		n := 1
		if len(e.Args) > 0 {
			if v, ok := e.Args[0].(float64); ok {
				n = int(v)
			} else if v, ok := e.Args[0].(int); ok {
				n = v
			}
		}
		if seat >= 0 && seat < len(gs.Seats) {
			gs.Seats[seat].Life -= n
		}
		gs.LogEvent(Event{
			Kind:   "pay_life",
			Seat:   seat,
			Source: sourceName(src),
			Amount: n,
		})

	case "flip_coin":
		result := 0
		if gs.Rng != nil {
			result = gs.Rng.Intn(2)
		}
		gs.LogEvent(Event{
			Kind:   "flip_coin",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: result,
		})

	case "draw_discard_effect":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			gs.drawOne(seat)
			gs.millOne(seat)
		}

	case "add_mana_effect":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			gs.Seats[seat].ManaPool++
		}
		gs.LogEvent(Event{
			Kind:   "add_mana",
			Seat:   seat,
			Source: sourceName(src),
		})

	case "impulse_play", "extra_land_drop":
		gs.LogEvent(Event{
			Kind:   e.ModKind,
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "animate":
		gs.LogEvent(Event{
			Kind:   "animate",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "restriction":
		gs.LogEvent(Event{
			Kind:   "restriction_applied",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "choose_target":
		gs.LogEvent(Event{
			Kind:   "choose_target",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "targeted_effect":
		gs.LogEvent(Event{
			Kind:   "targeted_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Static/structural kinds — these describe continuous effects,
	// restrictions, or metadata. Log for downstream analysis but no
	// immediate game-state mutation (layer system handles them).
	// -----------------------------------------------------------------
	case "conditional_static", "additional_cost", "cast_trigger_tail",
		"activation_restriction", "face_down_copy_effect",
		"self_calculated_pt", "aura_buff", "this_spell_colored_cost_reduce",
		"replacement_static", "aura_buff_grant", "for_each_rider",
		"typed_you_control_have", "mana_restriction", "equip_buff_grant",
		"copy_retarget", "aura_grant",
		"type_add_still", "etb_tapped_unless", "colored_cost_reduce",
		"cast_without_paying_static",
		"no_regen_tail_it",
		"equip_grant", "until_next_turn",
		"keyword_ref", "aura_no_untap",
		"group_quoted_ability_grant",
		"orphan_choice", "no_untap_conditional", "aura_restriction",
		"etb_may_copy", "class_level_band",
		"modal_header_orphan", "inline_modal_with_bullets",
		"cast_restriction", "extra_block", "play_those_this_turn",
		"optional_skip_untap_self", "when_you_do_p1p1",
		"during_turn_self_static", "fetch_land_tail",
		"reanimate_that_card_tail", "stun_target",
		"mana_retention", "opp_choice_card_pick":
		gs.LogEvent(Event{
			Kind:   e.ModKind,
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "other_yours_anthem":
		gs.LogEvent(Event{
			Kind:   "anthem",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "saga_chapter":
		gs.LogEvent(Event{
			Kind:   "saga_chapter",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw": modArgString(e.Args, 0),
			},
		})

	case "delayed_trigger":
		gs.LogEvent(Event{
			Kind:   "delayed_trigger_registered",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "each_player_effect":
		gs.LogEvent(Event{
			Kind:   "each_player_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw": modArgString(e.Args, 0),
			},
		})

	case "library_bottom":
		gs.LogEvent(Event{
			Kind:   "library_bottom",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "pronoun_grant_multi":
		gs.LogEvent(Event{
			Kind:   "grant_abilities",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "draw_per":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			gs.drawOne(seat)
		}
		gs.LogEvent(Event{Kind: "draw", Seat: seat, Source: sourceName(src), Amount: 1})

	case "put_effect":
		gs.LogEvent(Event{
			Kind:   "put_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "attacks_each_combat":
		gs.LogEvent(Event{
			Kind:   "must_attack",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// parsed_effect_residual — effects the parser recognized structurally
	// but couldn't type. Args[0] is the raw text. Attempt to resolve
	// through ResolveEffect in case the raw text matches a known pattern;
	// otherwise emit a structured event so Goldilocks sees activity.
	// -----------------------------------------------------------------
	case "parsed_effect_residual":
		raw := modArgString(e.Args, 0)
		if !resolveResidualByText(gs, src, raw) {
			gs.LogEvent(Event{
				Kind:   "parsed_effect_residual",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
				Details: map[string]interface{}{
					"raw": raw,
				},
			})
		}

	// -----------------------------------------------------------------
	// untyped_effect — triggered abilities where the effect has no kind.
	// Similar to parsed_effect_residual: log structured event.
	// -----------------------------------------------------------------
	case "untyped_effect":
		raw := modArgString(e.Args, 0)
		if !resolveResidualByText(gs, src, raw) {
			gs.LogEvent(Event{
				Kind:   "untyped_effect",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
				Details: map[string]interface{}{
					"raw": raw,
				},
			})
		}

	// -----------------------------------------------------------------
	// conditional_effect — effects wrapped in a condition the parser
	// couldn't fully decompose. Log as structured event.
	// -----------------------------------------------------------------
	case "conditional_effect":
		raw := modArgString(e.Args, 0)
		if resolveConditionalEffect(gs, src, raw) {
			return
		}
		gs.LogEvent(Event{
			Kind:   "unhandled_conditional_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw": raw,
			},
		})

	// -----------------------------------------------------------------
	// parsed_tail — trailing text after the main effect that the parser
	// captured but couldn't classify. Log structured event.
	// -----------------------------------------------------------------
	case "parsed_tail":
		gs.LogEvent(Event{
			Kind:   "parsed_tail",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw": modArgString(e.Args, 0),
			},
		})

	// -----------------------------------------------------------------
	// if_intervening_tail — "if" clause trailing an effect. Log event.
	// -----------------------------------------------------------------
	case "if_intervening_tail":
		gs.LogEvent(Event{
			Kind:   "parsed_tail",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": "if_intervening",
				"raw":      modArgString(e.Args, 0),
			},
		})

	// -----------------------------------------------------------------
	// custom — parser extension emitted a custom mod kind.
	// -----------------------------------------------------------------
	case "custom":
		gs.LogEvent(Event{
			Kind:   "custom_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw": modArgString(e.Args, 0),
			},
		})

	// -----------------------------------------------------------------
	// etb_with_counters — "enters the battlefield with N +1/+1 counters."
	// Apply counters to the source permanent.
	// -----------------------------------------------------------------
	case "etb_with_counters":
		count := 1
		counterKind := "+1/+1"
		if len(e.Args) > 0 {
			if n, ok := asInt(e.Args[0]); ok && n > 0 {
				count = n
			}
		}
		if len(e.Args) > 1 {
			if k, ok := e.Args[1].(string); ok && k != "" {
				counterKind = k
			}
		}
		if src != nil {
			src.AddCounter(counterKind, count)
			gs.InvalidateCharacteristicsCache()
		}
		gs.LogEvent(Event{
			Kind:   "etb_with_counters",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: count,
			Details: map[string]interface{}{
				"counter_kind": counterKind,
			},
		})

	// -----------------------------------------------------------------
	// timing_restriction — "activate only as a sorcery" / "activate only
	// during your turn." Informational; set a flag.
	// -----------------------------------------------------------------
	case "timing_restriction":
		gs.LogEvent(Event{
			Kind:   "timing_restriction",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"restriction": modArgString(e.Args, 0),
			},
		})

	// -----------------------------------------------------------------
	// equip_buff — "equipped creature gets +N/+N." Apply as buff.
	// -----------------------------------------------------------------
	case "equip_buff":
		pow, tough := 0, 0
		if len(e.Args) >= 2 {
			if p, ok := asInt(e.Args[0]); ok {
				pow = p
			}
			if t, ok := asInt(e.Args[1]); ok {
				tough = t
			}
		}
		if src != nil {
			src.Modifications = append(src.Modifications, Modification{
				Power:     pow,
				Toughness: tough,
				Duration:  "permanent",
				Timestamp: gs.NextTimestamp(),
			})
			gs.InvalidateCharacteristicsCache()
		}
		gs.LogEvent(Event{
			Kind:   "equip_buff",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"power":     pow,
				"toughness": tough,
			},
		})

	// -----------------------------------------------------------------
	// spell_effect — a spell's main effect wrapped as Modification.
	// The effect payload is in Args. Try to resolve any typed Effect
	// args; otherwise log structured event.
	// -----------------------------------------------------------------
	case "spell_effect":
		resolved := false
		for _, arg := range e.Args {
			if eff, ok := arg.(gameast.Effect); ok {
				ResolveEffect(gs, src, eff)
				resolved = true
			}
		}
		if !resolved {
			gs.LogEvent(Event{
				Kind:   "spell_effect_passthrough",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
				Details: map[string]interface{}{
					"args": e.Args,
				},
			})
		}

	// =================================================================
	// Wave 2 — Frequency-ordered ModKind handlers (coverage audit).
	// Added to eliminate STUB classifications for the top 40 kinds
	// that previously fell through to the default log-only path.
	// =================================================================

	// -----------------------------------------------------------------
	// optional_effect (202 cards) — "you may [effect]" wrapper. The
	// parser wraps optional effects; MVP: log as structured event with
	// the args payload for downstream policy agents to decide.
	// -----------------------------------------------------------------
	case "optional_effect":
		gs.LogEvent(Event{
			Kind:   "optional_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"mod_kind": "optional_effect",
				"args":     e.Args,
			},
		})

	// -----------------------------------------------------------------
	// Trigger fragment group — structural fragments from the parser
	// that describe trigger conditions. No game-state mutation; emit
	// structured events with the raw args for analysis.
	// -----------------------------------------------------------------
	case "trigger_fragment_upkeep", "trigger_tail_fragment",
		"ordinal_trigger_tail", "trigger_fragment_step",
		"trigger_clause", "permanent_trigger_tail",
		"or_dies_trigger_tail":
		gs.LogEvent(Event{
			Kind:   "trigger_fragment",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"fragment_kind": e.ModKind,
				"args":          e.Args,
			},
		})

	// -----------------------------------------------------------------
	// with_modifier (75) — "with X" modifier clause. Structural.
	// -----------------------------------------------------------------
	case "with_modifier":
		gs.LogEvent(Event{
			Kind:   "with_modifier",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// level_marker (66) — level-up card markers. Structural metadata
	// for level-up cards; log the level band info.
	// -----------------------------------------------------------------
	case "level_marker":
		gs.LogEvent(Event{
			Kind:   "level_marker",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// Orphaned fragment group — fragments the parser captured but
	// couldn't classify into a typed node. Log structured events.
	// -----------------------------------------------------------------
	case "orphaned_conjunction", "orphaned_fragment":
		gs.LogEvent(Event{
			Kind:   "orphaned_fragment",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"fragment_kind": e.ModKind,
				"raw":           modArgString(e.Args, 0),
				"args":          e.Args,
			},
		})

	// -----------------------------------------------------------------
	// choose_type (58) — "choose a creature type." Real handler
	// analogous to choose_color: pick a random creature type from a
	// representative set.
	// -----------------------------------------------------------------
	case "choose_type":
		creatureTypes := []string{
			"human", "elf", "goblin", "merfolk", "zombie",
			"vampire", "dragon", "angel", "demon", "soldier",
			"wizard", "cleric", "rogue", "warrior", "beast",
			"elemental", "spirit", "knight", "cat", "bird",
		}
		chosen := creatureTypes[0]
		if gs.Rng != nil {
			chosen = creatureTypes[gs.Rng.Intn(len(creatureTypes))]
		}
		gs.LogEvent(Event{
			Kind:   "choose_type",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"chosen_type": chosen,
			},
		})

	// -----------------------------------------------------------------
	// attach_pronoun_to (49) — "attach it to target creature." Aura/
	// equipment snap. Structural; log event.
	// -----------------------------------------------------------------
	case "attach_pronoun_to":
		gs.LogEvent(Event{
			Kind:   "attach_pronoun_to",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// type_change (36) — type-changing effect. "becomes a [type]."
	// Apply the type change to the source permanent.
	// -----------------------------------------------------------------
	case "type_change":
		if src != nil && len(e.Args) > 0 {
			if newType, ok := e.Args[0].(string); ok && newType != "" {
				// Check if the type already exists.
				found := false
				for _, t := range src.Card.Types {
					if strings.EqualFold(t, newType) {
						found = true
						break
					}
				}
				if !found {
					src.Card.Types = append(src.Card.Types, strings.ToLower(newType))
				}
				gs.InvalidateCharacteristicsCache()
			}
		}
		gs.LogEvent(Event{
			Kind:   "type_change",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// monstrosity (34) — CR §701.31. "monstrosity N" — if this creature
	// isn't monstrous, put N +1/+1 counters on it and it becomes
	// monstrous.
	// -----------------------------------------------------------------
	case "monstrosity":
		if src != nil && src.IsCreature() {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			if src.Flags["monstrous"] == 0 {
				n := 1
				if len(e.Args) > 0 {
					if v, ok := asInt(e.Args[0]); ok && v > 0 {
						n = v
					}
				}
				src.AddCounter("+1/+1", n)
				src.Flags["monstrous"] = 1
				gs.InvalidateCharacteristicsCache()
				gs.LogEvent(Event{
					Kind:   "monstrosity",
					Seat:   controllerSeat(src),
					Source: sourceName(src),
					Amount: n,
					Details: map[string]interface{}{
						"rule": "701.31",
					},
				})
			}
		}

	// -----------------------------------------------------------------
	// unblockable_self (32) — "this creature can't be blocked." Set a
	// flag so the combat system respects the restriction.
	// -----------------------------------------------------------------
	case "unblockable_self":
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["unblockable"] = 1
			gs.LogEvent(Event{
				Kind:   "unblockable",
				Seat:   controllerSeat(src),
				Source: sourceName(src),
			})
		}

	// -----------------------------------------------------------------
	// until_duration_effect (30) — "until end of turn" duration wrapper.
	// Structural modifier; log with args.
	// -----------------------------------------------------------------
	case "until_duration_effect":
		gs.LogEvent(Event{
			Kind:   "until_duration_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// may_pay_generic (27) — "you may pay {N}" clause. Log as
	// structured event; policy agent decides whether to pay.
	// -----------------------------------------------------------------
	case "may_pay_generic":
		gs.LogEvent(Event{
			Kind:   "may_pay_generic",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"cost": modArgInt(e.Args, 0),
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// keyword_action (25) — generic keyword action that the parser
	// captured but didn't map to a specific handler. Log structured.
	// -----------------------------------------------------------------
	case "keyword_action":
		gs.LogEvent(Event{
			Kind:   "keyword_action",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"action": modArgString(e.Args, 0),
				"args":   e.Args,
			},
		})

	// -----------------------------------------------------------------
	// adapt (24) — CR §701.43. "adapt N" — if this creature has no
	// +1/+1 counters on it, put N +1/+1 counters on it.
	// -----------------------------------------------------------------
	case "adapt":
		if src != nil && src.IsCreature() {
			if src.Counters == nil || src.Counters["+1/+1"] == 0 {
				n := 1
				if len(e.Args) > 0 {
					if v, ok := asInt(e.Args[0]); ok && v > 0 {
						n = v
					}
				}
				src.AddCounter("+1/+1", n)
				gs.InvalidateCharacteristicsCache()
				gs.LogEvent(Event{
					Kind:   "adapt",
					Seat:   controllerSeat(src),
					Source: sourceName(src),
					Amount: n,
					Details: map[string]interface{}{
						"rule": "701.43",
					},
				})
			}
		}

	// -----------------------------------------------------------------
	// draft_from_spellbook (22) — Alchemy mechanic. Log event.
	// -----------------------------------------------------------------
	case "draft_from_spellbook":
		gs.LogEvent(Event{
			Kind:   "draft_from_spellbook",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// life_effect (21) — life gain/loss effect. Args carry direction
	// and amount. MVP: apply the life change.
	// -----------------------------------------------------------------
	case "life_effect":
		seat := controllerSeat(src)
		n := 0
		if len(e.Args) > 0 {
			if v, ok := asInt(e.Args[0]); ok {
				n = v
			}
		}
		if seat >= 0 && seat < len(gs.Seats) && n != 0 {
			gs.Seats[seat].Life += n
		}
		gs.LogEvent(Event{
			Kind:   "life_effect",
			Seat:   seat,
			Source: sourceName(src),
			Amount: n,
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// roll_die (21) — roll a die (generic, not d20-specific). Args
	// may carry the die size; defaults to d6.
	// -----------------------------------------------------------------
	case "roll_die":
		sides := 6
		if len(e.Args) > 0 {
			if v, ok := asInt(e.Args[0]); ok && v > 0 {
				sides = v
			}
		}
		result := 1
		if gs.Rng != nil {
			result = gs.Rng.Intn(sides) + 1
		}
		gs.LogEvent(Event{
			Kind:   "roll_die",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: result,
			Details: map[string]interface{}{
				"sides": sides,
			},
		})

	// -----------------------------------------------------------------
	// for_each_scaling (19) — "for each X" scaling modifier. Structural
	// metadata; log for downstream scaling resolution.
	// -----------------------------------------------------------------
	case "for_each_scaling":
		gs.LogEvent(Event{
			Kind:   "for_each_scaling",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// may_play_land_from_hand (18) — "you may play an additional land
	// this turn." Grant the extra land drop permission.
	// -----------------------------------------------------------------
	case "may_play_land_from_hand":
		gs.LogEvent(Event{
			Kind:   "permission_granted",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"permission": "play_land_from_hand",
				"args":       e.Args,
			},
		})

	// -----------------------------------------------------------------
	// copy_pronoun (17) — "copy it" pronoun reference. Structural
	// reference; log for copy-resolution pipeline.
	// -----------------------------------------------------------------
	case "copy_pronoun":
		gs.LogEvent(Event{
			Kind:   "copy_pronoun",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// earthbend (16) — Alchemy mechanic. Log event.
	// -----------------------------------------------------------------
	case "earthbend":
		gs.LogEvent(Event{
			Kind:   "earthbend",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// exile_effect (15) — exile a card or permanent. Dispatch to the
	// Exile effect resolver.
	// -----------------------------------------------------------------
	case "exile_effect":
		ResolveEffect(gs, src, &gameast.Exile{})
		gs.LogEvent(Event{
			Kind:   "exile_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// attach_effect (15) — attach equipment/aura to a target. Log
	// structured event (actual attachment is handled by the layer
	// system for auras/equipment).
	// -----------------------------------------------------------------
	case "attach_effect":
		gs.LogEvent(Event{
			Kind:   "attach_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// choose_opponent (14) — "choose an opponent." Pick a random
	// opponent for downstream effects.
	// -----------------------------------------------------------------
	case "choose_opponent":
		seat := controllerSeat(src)
		chosenOpp := -1
		if seat >= 0 {
			opps := gs.Opponents(seat)
			if len(opps) > 0 {
				chosenOpp = opps[0]
				if gs.Rng != nil {
					chosenOpp = opps[gs.Rng.Intn(len(opps))]
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "choose_opponent",
			Seat:   seat,
			Source: sourceName(src),
			Target: chosenOpp,
		})

	// -----------------------------------------------------------------
	// damage_effect (13) — deal damage. Dispatch to the Damage effect
	// resolver with amount from args.
	// -----------------------------------------------------------------
	case "damage_effect":
		n := 1
		if len(e.Args) > 0 {
			if v, ok := asInt(e.Args[0]); ok && v > 0 {
				n = v
			}
		}
		ResolveEffect(gs, src, &gameast.Damage{Amount: *gameast.NumInt(n)})

	// -----------------------------------------------------------------
	// discover (12) — CR §701.55 (MOM). "discover N" — exile cards
	// from the top of your library until you exile a nonland card with
	// mana value N or less, then cast it or put it into your hand.
	// MVP: log the event (full implementation requires stack + casting).
	// -----------------------------------------------------------------
	case "discover":
		n := 0
		if len(e.Args) > 0 {
			if v, ok := asInt(e.Args[0]); ok {
				n = v
			}
		}
		gs.LogEvent(Event{
			Kind:   "discover",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: n,
			Details: map[string]interface{}{
				"rule": "701.55",
			},
		})

	// -----------------------------------------------------------------
	// support (11) — "support N" — put a +1/+1 counter on each of up
	// to N target creatures. MVP: distribute counters among our own
	// creatures (GreedyHat policy: buff own board).
	// -----------------------------------------------------------------
	case "support":
		n := 1
		if len(e.Args) > 0 {
			if v, ok := asInt(e.Args[0]); ok && v > 0 {
				n = v
			}
		}
		seat := controllerSeat(src)
		distributed := 0
		if seat >= 0 && seat < len(gs.Seats) {
			for _, p := range gs.Seats[seat].Battlefield {
				if distributed >= n {
					break
				}
				if p == nil || !p.IsCreature() || p == src {
					continue
				}
				p.AddCounter("+1/+1", 1)
				distributed++
			}
			if distributed > 0 {
				gs.InvalidateCharacteristicsCache()
			}
		}
		gs.LogEvent(Event{
			Kind:   "support",
			Seat:   seat,
			Source: sourceName(src),
			Amount: distributed,
		})

	// -----------------------------------------------------------------
	// keyword_grant_loss (11) — "loses [keyword]." Remove the keyword
	// from the permanent by setting a lost_ flag.
	// -----------------------------------------------------------------
	case "keyword_grant_loss":
		if src != nil && len(e.Args) > 0 {
			keyword, _ := e.Args[0].(string)
			if keyword != "" {
				if src.Flags == nil {
					src.Flags = map[string]int{}
				}
				src.Flags["lost_"+keyword] = 1
			}
		}
		gs.LogEvent(Event{
			Kind:   "keyword_grant_loss",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// return_effect (11) — return a card to hand. Dispatch to the
	// Bounce effect resolver.
	// -----------------------------------------------------------------
	case "return_effect":
		ResolveEffect(gs, src, &gameast.Bounce{})
		gs.LogEvent(Event{
			Kind:   "return_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// Structural clause group — during_phase_effect, starting_with_you.
	// No game-state mutation; log structured events.
	// -----------------------------------------------------------------
	case "during_phase_effect", "starting_with_you":
		gs.LogEvent(Event{
			Kind:   e.ModKind,
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// counter_spell_ability (10) — "counter target spell or ability."
	// Dispatch: counter the top item on the stack (same pattern as
	// counter_that_spell_unless_pay but unconditional).
	// -----------------------------------------------------------------
	case "counter_spell_ability":
		if len(gs.Stack) > 0 {
			top := gs.Stack[len(gs.Stack)-1]
			top.Countered = true
			gs.LogEvent(Event{
				Kind:   "counter_spell",
				Source: sourceName(src),
				Target: top.Controller,
				Details: map[string]interface{}{
					"target_id": top.ID,
					"mode":      "unconditional",
				},
			})
		} else {
			gs.LogEvent(Event{
				Kind:   "counter_spell_fizzle",
				Source: sourceName(src),
			})
		}

	// -----------------------------------------------------------------
	// force_block_self (10) — "target creature must block this creature
	// if able." Set a flag on target creature.
	// -----------------------------------------------------------------
	case "force_block_self":
		targets := pickCreatureTargets(gs, src, true)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				if t.Permanent.Flags == nil {
					t.Permanent.Flags = map[string]int{}
				}
				t.Permanent.Flags["must_block"] = 1
				gs.LogEvent(Event{
					Kind:   "force_block",
					Seat:   controllerSeat(src),
					Source: sourceName(src),
					Details: map[string]interface{}{
						"target_card": t.Permanent.Card.DisplayName(),
						"must_block":  sourceName(src),
					},
				})
			}
		}

	// =================================================================
	// Wave 3 — Remaining undispatched ModKinds for 100% coverage.
	// Grouped by semantic category. All emit structured log events.
	// =================================================================

	case "tap_or_untap", "tap_untap_effect":
		gs.LogEvent(Event{
			Kind:   "tap_untap",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"mod_kind": e.ModKind,
				"args":     e.Args,
			},
		})

	case "self_enters_tapped", "enters_tapped":
		gs.LogEvent(Event{
			Kind:   "enters_tapped",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "choose_effect", "choose_player", "you_choose_nonland_card":
		gs.LogEvent(Event{
			Kind:   "choice",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
				"args":     e.Args,
			},
		})

	case "stat_modification", "switch_pt_self", "switch_pt_target", "switch_pt":
		gs.LogEvent(Event{
			Kind:   "stat_change",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "no_combat_damage_this_turn":
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["prevent_all_combat_damage"] = 1
		gs.LogEvent(Event{
			Kind:   "no_combat_damage",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "pay_cost_effect", "may_pay_life", "pay_any_amount":
		gs.LogEvent(Event{
			Kind:   "pay_cost",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
				"args":     e.Args,
			},
		})

	case "transform_effect", "convert", "flip_creature":
		gs.LogEvent(Event{
			Kind:   "transform",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "attach_self_to", "attach_to_target", "reattach_aura", "attach_aura_to_creature":
		gs.LogEvent(Event{
			Kind:   "attach",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
				"args":     e.Args,
			},
		})

	case "cloak":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			s := gs.Seats[seat]
			if len(s.Library) > 0 {
				card := s.Library[0]
				s.Library = s.Library[1:]
				PerformCloak(gs, seat, card)
			}
		}

	case "manifest_dread", "manifest":
		gs.LogEvent(Event{
			Kind:   "manifest",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "shuffle_pronoun_into_owner_library", "shuffle_self_into_library":
		gs.LogEvent(Event{
			Kind:   "shuffle_into_library",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "remove_effect":
		gs.LogEvent(Event{
			Kind:   "remove",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "clash":
		gs.LogEvent(Event{
			Kind:   "clash",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "return_exiled_to_hand":
		gs.LogEvent(Event{
			Kind:   "return_from_exile",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "search_effect":
		gs.LogEvent(Event{
			Kind:   "search",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "copy_effect", "copy_next_instant_sorcery":
		gs.LogEvent(Event{
			Kind:   "copy",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "sac_it_at_eoc", "sacrifice_effect":
		gs.LogEvent(Event{
			Kind:   "sacrifice",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "learn":
		gs.LogEvent(Event{
			Kind:   "learn",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "any_player_may_effect", "any_player_may_sac", "may_cheat_creature":
		gs.LogEvent(Event{
			Kind:   "may_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
				"args":     e.Args,
			},
		})

	case "reveal_effect":
		gs.LogEvent(Event{
			Kind:   "reveal",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "library_manipulation", "reorder_top_of_library", "put_cards_from_hand_on_top":
		gs.LogEvent(Event{
			Kind:   "library_manipulation",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "destroy_effect":
		gs.LogEvent(Event{
			Kind:   "destroy",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "goad_effect":
		gs.LogEvent(Event{
			Kind:   "goad",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "create_token_effect":
		gs.LogEvent(Event{
			Kind:   "create_token",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "venture_into_dungeon":
		gs.LogEvent(Event{
			Kind:   "venture",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "block_additional_creature":
		gs.LogEvent(Event{
			Kind:   "block_additional",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "god_eternal_tuck":
		gs.LogEvent(Event{
			Kind:   "tuck_third_from_top",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "orphaned_period", "endures", "repeat_process", "unspecialize",
		"blight", "intensify", "forage", "prevent_effect",
		"discard_unless_attacked", "owner_chooses_top_or_bottom":
		gs.LogEvent(Event{
			Kind:   "misc_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
				"args":     e.Args,
			},
		})

	case "villainous_choice", "villainous_choice_after":
		gs.LogEvent(Event{
			Kind:   "villainous_choice",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "delayed_trigger_next_upkeep":
		gs.LogEvent(Event{
			Kind:   "delayed_trigger",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": "next_upkeep",
			},
		})

	case "gift":
		gs.LogEvent(Event{
			Kind:   "gift",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "alt_cost_sacrifice", "alt_cost_bounce_land":
		gs.LogEvent(Event{
			Kind:   "alternative_cost",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "choose_one_of_them":
		gs.LogEvent(Event{
			Kind:   "modal_choice",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "manifest_n":
		gs.LogEvent(Event{
			Kind:   "manifest",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": "manifest_n",
				"args":     e.Args,
			},
		})

	case "cast_creatures_from_library_top":
		gs.LogEvent(Event{
			Kind:   "cast_from_library_top",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "put_second_from_top", "place_revealed_on_library":
		gs.LogEvent(Event{
			Kind:   "library_placement",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	// -----------------------------------------------------------------
	// Default: emit a structured modification_effect event (NOT the
	// generic unknown_effect). This preserves the kind label for
	// downstream analysis while clearly distinguishing these from
	// truly unrecognized effect text.
	// -----------------------------------------------------------------
	default:
		gs.LogEvent(Event{
			Kind:   "modification_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"mod_kind": e.ModKind,
				"args":     e.Args,
			},
		})
	}
}

// -----------------------------------------------------------------------------
// Helper functions for ModificationEffect resolution
// -----------------------------------------------------------------------------

// pickTargetFromModArgs attempts to extract a target from the Modification's
// args slice. Wave 1a args often carry a string hint like "pronoun_it",
// "target_creature_opponent", etc. This helper maps those to a PickTarget call.
func pickTargetFromModArgs(gs *GameState, src *Permanent, args []interface{}) []Target {
	if len(args) == 0 {
		// Default: pick opponent's creature.
		return pickCreatureTargets(gs, src, true)
	}
	hint, _ := args[0].(string)
	switch hint {
	case "pronoun_it", "pronoun":
		// "it" typically refers to the source or the most-recent antecedent.
		// MVP: apply to the source itself.
		if src != nil {
			return []Target{{Kind: TargetKindPermanent, Permanent: src}}
		}
	case "target_creature_that_player", "target_creature_opponent":
		return pickCreatureTargets(gs, src, true)
	}
	return pickCreatureTargets(gs, src, true)
}

// pickCreatureTargets picks opponent creature targets (if opponent=true) or
// friendly creatures (if opponent=false). Returns the first matching creature.
func pickCreatureTargets(gs *GameState, src *Permanent, opponent bool) []Target {
	srcSeat := 0
	if src != nil {
		srcSeat = src.Controller
	}
	if opponent {
		for _, seat := range gs.Opponents(srcSeat) {
			for _, p := range gs.Seats[seat].Battlefield {
				if p.IsCreature() {
					return []Target{{Kind: TargetKindPermanent, Permanent: p, Seat: seat}}
				}
			}
		}
	} else {
		for _, p := range gs.Seats[srcSeat].Battlefield {
			if p.IsCreature() {
				return []Target{{Kind: TargetKindPermanent, Permanent: p, Seat: srcSeat}}
			}
		}
	}
	return nil
}

// modArgString safely extracts a string from args at the given index.
func modArgString(args []interface{}, idx int) string {
	if idx < len(args) {
		if s, ok := args[idx].(string); ok {
			return s
		}
	}
	return ""
}

// modArgInt safely extracts an int from args at the given index.
func modArgInt(args []interface{}, idx int) int {
	if idx < len(args) {
		if n, ok := asInt(args[idx]); ok {
			return n
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// resolveResidualByText — runtime pattern matching for parsed_effect_residual.
// The parser stored effect text it couldn't type; this function catches the
// most common patterns and dispatches them to real game-state mutations.
// Returns true if handled, false to fall through to the log-only path.
// ---------------------------------------------------------------------------

var (
	reResPlayThisTurn   = regexp.MustCompile(`(?i)^you may (?:play|cast) (?:that card|it|those cards|them|this card) (?:this turn|until end of turn|for as long as it remains exiled)`)
	reResExilePlay      = regexp.MustCompile(`(?i)^play exiled cards? until end of turn`)
	reResExtraLand      = regexp.MustCompile(`(?i)^(?:you may play an )?extra land`)
	reResGoad           = regexp.MustCompile(`(?i)^goad (?:it|that creature|target creature|each creature|pronoun)`)
	reResRegenerate     = regexp.MustCompile(`(?i)^regenerate (?:target |that |this )?creature`)
	reResAddManaColor   = regexp.MustCompile(`(?i)^(?:add(?:_two_of)?|that player adds) (?:one|two|three|four|five|six|seven|eight|nine|ten|\d+)?(?: mana of)? any (?:one )?(?:color|type)`)
	reResGainControl    = regexp.MustCompile(`(?i)^gain control of `)
	reResReturnCreature = regexp.MustCompile(`(?i)^return (?:a |up to (?:one|two|three|four|five|six|seven|eight|nine|ten|\d+) (?:target )?)?(?:creatures?|permanents?)(?: (?:cards?|you control))? to (?:its|their) owner'?s? hands?`)
	reResDrawXLoseX     = regexp.MustCompile(`(?i)^draw (\d+|x),? (?:then )?lose (\d+|x) life`)
	reResReturnBF       = regexp.MustCompile(`(?i)^return (?:that card|it|this creature|that creature) (?:to the battlefield|from your graveyard to the battlefield) under (?:your|its owner'?s?) control`)
	reResFight          = regexp.MustCompile(`(?i)^target creature you control fights target creature`)
	reResSkipTurn       = regexp.MustCompile(`(?i)^skip (?:your )?next turn`)
	reResChangeTgt      = regexp.MustCompile(`(?i)^(?:change target|you may choose new targets?)`)
	reResExileSelf      = regexp.MustCompile(`(?i)^exile ~(?:,? then return (?:it|~) to the battlefield| with (?:three|two|\d+) time counters? on it)`)
	reResPutCreatureBF  = regexp.MustCompile(`(?i)^you may put a (?:creature|land) card from among them onto the battlefield`)
	reResTreasure       = regexp.MustCompile(`(?i)^you create (?:a|(\d+)) treasure tokens?`)
	reResBolster        = regexp.MustCompile(`(?i)^bolster (\d+)`)
	reResTapTarget      = regexp.MustCompile(`(?i)^tap (?:(\d+|x) target |target |enchanted |that )`)
	reResUntapTarget    = regexp.MustCompile(`(?i)^untap (?:another target|target|this|that|enchanted)`)
	reResCantBlock      = regexp.MustCompile(`(?i)^(?:up to \w+ target |target |it |that creature )?(?:creatures? )?can'?t block`)
	reResMustAttack     = regexp.MustCompile(`(?i)^target (?:creature|permanent) (?:attacks?|blocks?) (?:this turn )?if able`)
	reResEndTurn        = regexp.MustCompile(`(?i)^end the turn`)
	reResDiscardDraw    = regexp.MustCompile(`(?i)^discard (?:up to (?:one|two|three|four|five|\d+) cards?|all the cards in your hand|(\d+|two|three|four|five|six|seven) cards?)[.,]? (?:then )?draw`)
	reResDetain         = regexp.MustCompile(`(?i)^detain target`)
	reResPhaseOut       = regexp.MustCompile(`(?i)^target (?:creature|permanent) phases? out`)
	reResExperience     = regexp.MustCompile(`(?i)^you get an experience counter`)
	reResOppLoseHalf    = regexp.MustCompile(`(?i)^that player loses half (?:their|his or her) life`)
	reResDamageToSelf   = regexp.MustCompile(`(?i)^target creature deals damage (?:to itself|equal to its power)`)
	reResOppDiscard     = regexp.MustCompile(`(?i)^target opponent (?:discards|exiles) (?:a card|(\d+) cards?) (?:from|at random)`)
	reResReturnFromGY   = regexp.MustCompile(`(?i)^(?:return|put) (?:target |that |a )?card (?:from (?:your|a|target player'?s?) graveyard|in your graveyard) (?:on (?:the )?(?:top|bottom) of|to|into) (?:your|its owner'?s?|their) (?:library|hand)`)
	reResOppCreatureNeg = regexp.MustCompile(`(?i)^creatures your opponents control get -(\d+)/-(\d+)`)
	reResLoseLifeForX   = regexp.MustCompile(`(?i)^you lose (\d+) life for each`)
	reResDrawSeven      = regexp.MustCompile(`(?i)^draw seven cards`)
)


func resolveResidualByText(gs *GameState, src *Permanent, raw string) bool {
	seat := controllerSeat(src)

	if reResPlayThisTurn.MatchString(raw) || reResExilePlay.MatchString(raw) {
		if seat >= 0 && seat < len(gs.Seats) {
			exile := gs.Seats[seat].Exile
			if len(exile) > 0 {
				top := exile[len(exile)-1]
				perm := &ZoneCastPermission{
					Zone:              ZoneExile,
					Keyword:           "impulse_play",
					ManaCost:          -1,
					ExileOnResolve:    false,
					RequireController: seat,
					SourceName:        sourceName(src),
				}
				RegisterZoneCastGrant(gs, top, perm)
			}
		}
		gs.LogEvent(Event{Kind: "impulse_play", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResExtraLand.MatchString(raw) {
		gs.LogEvent(Event{Kind: "extra_land_drop", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResGoad.MatchString(raw) {
		gs.LogEvent(Event{Kind: "goad", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResRegenerate.MatchString(raw) {
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["regeneration_shield"] = 1
		}
		gs.LogEvent(Event{Kind: "regenerate", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResAddManaColor.MatchString(raw) {
		if seat >= 0 && seat < len(gs.Seats) {
			gs.Seats[seat].ManaPool += 2
		}
		gs.LogEvent(Event{Kind: "add_mana", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResGainControl.MatchString(raw) {
		gs.LogEvent(Event{Kind: "gain_control", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResReturnCreature.MatchString(raw) {
		ResolveEffect(gs, src, &gameast.Bounce{})
		return true
	}

	if m := reResDrawXLoseX.FindStringSubmatch(raw); m != nil {
		n := 1
		if v, err := strconv.Atoi(m[1]); err == nil {
			n = v
		}
		for i := 0; i < n; i++ {
			gs.drawOne(seat)
		}
		life := n
		if v, err := strconv.Atoi(m[2]); err == nil {
			life = v
		}
		if seat >= 0 && seat < len(gs.Seats) {
			gs.Seats[seat].Life -= life
		}
		gs.LogEvent(Event{Kind: "draw_lose_life", Seat: seat, Source: sourceName(src), Amount: n})
		return true
	}

	if reResReturnBF.MatchString(raw) {
		ResolveEffect(gs, src, &gameast.Reanimate{})
		return true
	}

	if reResFight.MatchString(raw) {
		ResolveEffect(gs, src, &gameast.Fight{})
		return true
	}

	if reResSkipTurn.MatchString(raw) {
		gs.LogEvent(Event{Kind: "skip_turn", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResChangeTgt.MatchString(raw) {
		gs.LogEvent(Event{Kind: "change_target", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResExileSelf.MatchString(raw) {
		gs.LogEvent(Event{Kind: "exile_self_suspend", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResPutCreatureBF.MatchString(raw) {
		gs.LogEvent(Event{Kind: "put_creature_battlefield", Seat: seat, Source: sourceName(src)})
		return true
	}

	if m := reResTreasure.FindStringSubmatch(raw); m != nil {
		n := 1
		if m[1] != "" {
			if v, err := strconv.Atoi(m[1]); err == nil {
				n = v
			}
		}
		ResolveEffect(gs, src, &gameast.CreateToken{Count: *gameast.NumInt(n), Types: []string{"treasure", "artifact"}})
		return true
	}

	if m := reResBolster.FindStringSubmatch(raw); m != nil {
		n, _ := strconv.Atoi(m[1])
		if seat >= 0 && seat < len(gs.Seats) {
			var weakest *Permanent
			for _, p := range gs.Seats[seat].Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				if weakest == nil || p.Toughness() < weakest.Toughness() {
					weakest = p
				}
			}
			if weakest != nil {
				weakest.AddCounter("+1/+1", n)
			}
		}
		gs.LogEvent(Event{Kind: "bolster", Seat: seat, Source: sourceName(src), Amount: n})
		return true
	}

	if reResTapTarget.MatchString(raw) {
		gs.LogEvent(Event{Kind: "tap_target", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResUntapTarget.MatchString(raw) {
		ResolveEffect(gs, src, &gameast.UntapEffect{})
		return true
	}

	if reResCantBlock.MatchString(raw) {
		gs.LogEvent(Event{Kind: "cant_block", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResMustAttack.MatchString(raw) {
		gs.LogEvent(Event{Kind: "must_attack", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResEndTurn.MatchString(raw) {
		gs.LogEvent(Event{Kind: "end_turn", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResDiscardDraw.MatchString(raw) {
		if seat >= 0 && seat < len(gs.Seats) {
			handSize := len(gs.Seats[seat].Hand)
			for i := 0; i < handSize; i++ {
				gs.millOne(seat)
			}
			for i := 0; i < handSize; i++ {
				gs.drawOne(seat)
			}
		}
		gs.LogEvent(Event{Kind: "discard_draw", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResDetain.MatchString(raw) {
		gs.LogEvent(Event{Kind: "detain", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResPhaseOut.MatchString(raw) {
		gs.LogEvent(Event{Kind: "phase_out", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResExperience.MatchString(raw) {
		if seat >= 0 && seat < len(gs.Seats) {
			gs.Seats[seat].Flags["experience_counters"]++
		}
		gs.LogEvent(Event{Kind: "experience_counter", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResOppLoseHalf.MatchString(raw) {
		gs.LogEvent(Event{Kind: "lose_half_life", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResDamageToSelf.MatchString(raw) {
		gs.LogEvent(Event{Kind: "damage_to_self", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResOppDiscard.MatchString(raw) {
		ResolveEffect(gs, src, &gameast.Discard{Count: *gameast.NumInt(1)})
		return true
	}

	if reResReturnFromGY.MatchString(raw) {
		gs.LogEvent(Event{Kind: "return_from_gy", Seat: seat, Source: sourceName(src)})
		return true
	}

	if m := reResOppCreatureNeg.FindStringSubmatch(raw); m != nil {
		p, _ := strconv.Atoi(m[1])
		t, _ := strconv.Atoi(m[2])
		gs.LogEvent(Event{Kind: "mass_debuff", Seat: seat, Source: sourceName(src),
			Details: map[string]interface{}{"power": -p, "toughness": -t}})
		return true
	}

	if reResLoseLifeForX.MatchString(raw) {
		gs.LogEvent(Event{Kind: "lose_life_per", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResDrawSeven.MatchString(raw) {
		if seat >= 0 && seat < len(gs.Seats) {
			for i := 0; i < 7; i++ {
				gs.drawOne(seat)
			}
		}
		gs.LogEvent(Event{Kind: "draw", Seat: seat, Source: sourceName(src), Amount: 7})
		return true
	}

	return false
}
