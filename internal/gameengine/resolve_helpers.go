package gameengine

import (
	"fmt"
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
		// CR §707 — "copy that ability." Rings of Brighthearth, Strionic
		// Resonator, etc. The ability stack isn't fully modeled in the
		// current engine, so log-only is acceptable. A future Phase 5+
		// will model triggered/activated ability objects on the stack.
		gs.LogEvent(Event{
			Kind:   "copy_ability",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Copy per commander cast — commander-storm (Replication Technique,
	// etc.). Copy the spell N times where N = total commander cast count
	// for this seat. Uses CommanderCastCounts which tracks per-commander
	// cast history.
	// -----------------------------------------------------------------
	case "copy_per_commander_cast":
		seat := controllerSeat(src)
		copies := 0
		if seat >= 0 && seat < len(gs.Seats) {
			for _, n := range gs.Seats[seat].CommanderCastCounts {
				copies += n
			}
			for i := 0; i < copies; i++ {
				ResolveEffect(gs, src, &gameast.CopySpell{})
			}
		}
		gs.LogEvent(Event{
			Kind:   "copy_per_commander_cast",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: copies,
		})

	// -----------------------------------------------------------------
	// Fight each other — "those creatures fight each other." Delegate
	// to the Fight effect handler which performs mutual damage exchange
	// between two creatures (CR §701.12).
	// -----------------------------------------------------------------
	case "fight_each_other":
		ResolveEffect(gs, src, &gameast.Fight{})
		gs.LogEvent(Event{
			Kind:   "fight_each_other",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	// -----------------------------------------------------------------
	// Heist — "heist target opponent." Pick a random opponent, exile the
	// top card of their library face-down, and grant cast permission to
	// the controller. CR §701.XX (heist keyword action).
	// -----------------------------------------------------------------
	case "heist":
		seat := controllerSeat(src)
		heisted := false
		if seat >= 0 && seat < len(gs.Seats) {
			opps := gs.LivingOpponents(seat)
			if len(opps) > 0 {
				oppIdx := 0
				if gs.Rng != nil && len(opps) > 1 {
					oppIdx = gs.Rng.Intn(len(opps))
				}
				opp := opps[oppIdx]
				oppSeat := gs.Seats[opp]
				if len(oppSeat.Library) > 0 {
					card := oppSeat.Library[0]
					MoveCard(gs, card, opp, "library", "exile", "heist")
					// Grant the controller permission to cast it for free.
					perm := NewFreeCastFromExilePermission(seat, sourceName(src))
					RegisterZoneCastGrant(gs, card, perm)
					heisted = true
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "heist",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"heisted": heisted,
			},
		})

	// -----------------------------------------------------------------
	// Put card into hand — "put that card into your hand" / "put it
	// into your hand." These are pronoun-based zone changes typically
	// following a reveal or look-at. MVP: log structured event.
	// -----------------------------------------------------------------
	case "put_card_into_hand":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			exile := gs.Seats[seat].Exile
			if len(exile) > 0 {
				// Move last exiled card to hand (pronoun-based "put it
				// into your hand" typically refers to the most recently
				// exiled/revealed card).
				card := exile[len(exile)-1]
				MoveCard(gs, card, seat, "exile", "hand", "put_card_into_hand")
			} else {
				// No exile context — draw a card as fallback.
				gs.drawOne(seat)
			}
		}
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
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			s := gs.Seats[seat]
			var card *Card
			fromZone := ""
			// Prefer exile (pronoun "put it onto the battlefield" after
			// exile), then hand, then top of library.
			if len(s.Exile) > 0 {
				card = s.Exile[len(s.Exile)-1]
				fromZone = "exile"
			} else if len(s.Hand) > 0 {
				card = s.Hand[len(s.Hand)-1]
				fromZone = "hand"
			} else if len(s.Library) > 0 {
				card = s.Library[0]
				fromZone = "library"
			}
			if card != nil {
				removeCardFromZone(gs, seat, card, fromZone)
				p := &Permanent{
					Card:          card,
					Controller:    seat,
					Tapped:        false,
					SummoningSick: true,
					Timestamp:     gs.NextTimestamp(),
					Counters:      map[string]int{},
					Flags:         map[string]int{},
				}
				s.Battlefield = append(s.Battlefield, p)
				RegisterReplacementsForPermanent(gs, p)
				FirePermanentETBTriggers(gs, p)
			}
		}
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
		// CR §303.4 — attach an Aura to a target creature. The source
		// permanent (the Aura) sets its AttachedTo pointer to the target
		// creature. Pick a friendly creature as the target (GreedyHat
		// policy: buff our own creatures).
		auraTargetName := ""
		if src != nil {
			targets := pickCreatureTargets(gs, src, false) // prefer our own creatures
			if len(targets) == 0 {
				targets = pickCreatureTargets(gs, src, true) // fall back to opponent
			}
			for _, t := range targets {
				if t.Kind == TargetKindPermanent && t.Permanent != nil && t.Permanent != src {
					src.AttachedTo = t.Permanent
					if t.Permanent.Card != nil {
						auraTargetName = t.Permanent.Card.DisplayName()
					}
					break
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "attach_aura",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"attached_to": auraTargetName,
			},
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
	// Pay any amount of life — "pay any amount of life." Simulation
	// heuristic: pay half (aggressive posture maximizes the effect while
	// keeping the player in the game). A Phase 10 policy agent will
	// choose optimally.
	// -----------------------------------------------------------------
	case "pay_any_amount_life":
		paid := 0
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			paid = gs.Seats[seat].Life / 2
			if paid < 0 {
				paid = 0
			}
			gs.Seats[seat].Life -= paid
		}
		gs.LogEvent(Event{
			Kind:   "pay_life",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: paid,
			Details: map[string]interface{}{
				"mode": "any_amount",
			},
		})

	// -----------------------------------------------------------------
	// May play exiled card free / may cast copy free — permission
	// markers for future-cast effects.
	// -----------------------------------------------------------------
	case "may_play_exiled_free":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			exile := gs.Seats[seat].Exile
			if len(exile) > 0 {
				top := exile[len(exile)-1]
				perm := NewFreeCastFromExilePermission(seat, sourceName(src))
				RegisterZoneCastGrant(gs, top, perm)
			}
		}
		gs.LogEvent(Event{
			Kind:   "permission_granted",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"permission": "play_exiled_free",
			},
		})

	case "may_cast_copy_free":
		ResolveEffect(gs, src, &gameast.CopySpell{})
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
		// "Put one into your hand and the other into your graveyard."
		// Approximate: draw one card (to hand), mill one card (to graveyard).
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			gs.drawOne(seat)
			gs.millOne(seat)
		}
		gs.LogEvent(Event{
			Kind:   "split_cards",
			Seat:   seat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"destinations": []string{"hand", "graveyard"},
			},
		})

	// -----------------------------------------------------------------
	// Any opponent may sacrifice a creature — Savra-style.
	// -----------------------------------------------------------------
	case "any_opp_may_sac_creature":
		// CR §701.17 — "Each opponent may sacrifice a creature." Optional
		// effect for opponents. Simulation heuristic: opponents decline to
		// sacrifice (conservative — opponents wouldn't voluntarily give up
		// resources unless specifically advantageous). No state mutation.
		gs.LogEvent(Event{
			Kind:   "may_sacrifice",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"who":      "any_opponent",
				"target":   "creature",
				"decision": "declined",
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
		// CR §701.44 — "Amass N" (with optional creature type). If you
		// control an Army creature, put N +1/+1 counters on it. Otherwise,
		// create a 0/0 black Zombie Army creature token, then put N +1/+1
		// counters on it.
		amassN := 1
		amassType := "zombie"
		if len(e.Args) > 0 {
			if n, ok := asInt(e.Args[0]); ok && n > 0 {
				amassN = n
			}
		}
		if len(e.Args) > 1 {
			if t, ok := e.Args[1].(string); ok && t != "" {
				amassType = strings.ToLower(t)
			}
		}
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			// Look for an existing Army creature we control.
			var army *Permanent
			for _, p := range gs.Seats[seat].Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				for _, tp := range p.Card.Types {
					if strings.EqualFold(tp, "army") {
						army = p
						break
					}
				}
				if army != nil {
					break
				}
			}
			if army == nil {
				// Create a 0/0 Army token.
				army = CreateCreatureToken(gs, seat,
					capitalize(amassType)+" Army",
					[]string{"creature", amassType, "army"},
					0, 0)
			}
			if army != nil {
				army.AddCounter("+1/+1", amassN)
				gs.InvalidateCharacteristicsCache()
			}
		}
		gs.LogEvent(Event{
			Kind:   "amass",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: amassN,
			Details: map[string]interface{}{
				"creature_type": amassType,
				"rule":          "701.44",
			},
		})

	case "transform_self":
		// CR §712 — transform the source permanent. Delegate to
		// TransformPermanent for DFC cards; toggle flag for non-DFC.
		if src != nil {
			if src.FrontFaceAST != nil && src.BackFaceAST != nil {
				TransformPermanent(gs, src, "transform_self")
			} else {
				if src.Flags == nil {
					src.Flags = map[string]int{}
				}
				if src.Flags["transformed"] != 0 {
					src.Flags["transformed"] = 0
				} else {
					src.Flags["transformed"] = 1
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "transform",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "ring_tempts":
		// CR §701.52 — "The Ring tempts you." Increment the player's ring
		// temptation level. Each level unlocks an additional ring ability
		// for the designated ring-bearer. Levels 0-3 correspond to the four
		// abilities on The One Ring.
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			s := gs.Seats[seat]
			if s.Flags == nil {
				s.Flags = map[string]int{}
			}
			s.Flags["ring_temptation"]++
		}
		gs.LogEvent(Event{
			Kind:   "ring_tempts",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "venture_dungeon":
		// CR §701.46 — "venture into the dungeon." Track dungeon progress
		// via a per-seat flag. Each venture increments the dungeon level.
		// Dungeon rooms have effects at each level; MVP: grant a small
		// bonus at each level (scry/draw/life gain as approximation).
		vdSeat := controllerSeat(src)
		if vdSeat >= 0 && vdSeat < len(gs.Seats) {
			s := gs.Seats[vdSeat]
			if s.Flags == nil {
				s.Flags = make(map[string]int)
			}
			s.Flags["dungeon_level"]++
			level := s.Flags["dungeon_level"]
			// Apply a level-based bonus as an approximation of dungeon rooms.
			switch {
			case level <= 1:
				// Room 1: scry-like effect — no visible mutation, just log.
			case level == 2:
				// Room 2: draw a card.
				gs.drawOne(vdSeat)
			case level == 3:
				// Room 3: gain 3 life.
				s.Life += 3
			default:
				// Deeper rooms: draw a card (completed dungeon bonus).
				gs.drawOne(vdSeat)
			}
		}
		gs.LogEvent(Event{
			Kind:   "venture_dungeon",
			Seat:   vdSeat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"dungeon_level": func() int {
					if vdSeat >= 0 && vdSeat < len(gs.Seats) && gs.Seats[vdSeat].Flags != nil {
						return gs.Seats[vdSeat].Flags["dungeon_level"]
					}
					return 0
				}(),
				"rule": "701.46",
			},
		})

	case "take_initiative":
		// CR §722 — "You take the initiative." Clear initiative from all
		// other players, then grant it to the controller.
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			for i, s := range gs.Seats {
				if s == nil {
					continue
				}
				if s.Flags == nil {
					s.Flags = map[string]int{}
				}
				if i == seat {
					s.Flags["has_initiative"] = 1
				} else {
					delete(s.Flags, "has_initiative")
				}
			}
		}
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
		// Determine tier based on result: 1-9 low, 10-19 mid, 20 high.
		tier := "low"
		if result >= 10 && result <= 19 {
			tier = "mid"
		} else if result >= 20 {
			tier = "high"
		}
		// Apply tier-based bonus to controller.
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			switch tier {
			case "mid":
				// Mid tier: draw a card as a moderate bonus.
				gs.drawOne(seat)
			case "high":
				// High tier (nat 20): draw two cards as a strong bonus.
				gs.drawOne(seat)
				gs.drawOne(seat)
			}
		}
		gs.LogEvent(Event{
			Kind:   "roll_d20",
			Seat:   seat,
			Source: sourceName(src),
			Amount: result,
			Details: map[string]interface{}{
				"tier": tier,
			},
		})

	case "seek":
		// CR §701.42 — "Seek a card with [quality]": look at cards in library
		// matching criteria, pick one at random, put into hand. Library is NOT
		// shuffled afterwards (that's the key difference from tutor).
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			lib := gs.Seats[seat].Library
			if len(lib) > 0 {
				idx := 0
				if gs.Rng != nil {
					idx = gs.Rng.Intn(len(lib))
				}
				card := lib[idx]
				MoveCard(gs, card, seat, "library", "hand", "seek")
			}
		}
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
		n := 1
		if len(e.Args) > 0 {
			if v, ok := e.Args[0].(float64); ok {
				n = int(v)
			} else if v, ok := e.Args[0].(int); ok {
				n = v
			}
		}
		if src != nil {
			Connive(gs, src, n)
		}

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
		result := 0 // 0 = heads (win), 1 = tails (lose)
		if gs.Rng != nil {
			result = gs.Rng.Intn(2)
		}
		seat := controllerSeat(src)
		won := result == 0
		if won && seat >= 0 && seat < len(gs.Seats) {
			// On heads (win), apply bonus from args if available.
			// Default bonus: draw a card.
			bonusApplied := false
			if len(e.Args) > 0 {
				for _, arg := range e.Args {
					if eff, ok := arg.(gameast.Effect); ok {
						ResolveEffect(gs, src, eff)
						bonusApplied = true
					}
				}
			}
			if !bonusApplied {
				gs.drawOne(seat)
			}
		}
		gs.LogEvent(Event{
			Kind:   "flip_coin",
			Seat:   seat,
			Source: sourceName(src),
			Amount: result,
			Details: map[string]interface{}{
				"won": won,
			},
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

	case "impulse_play":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			s := gs.Seats[seat]
			if len(s.Library) > 0 {
				top := s.Library[0]
				MoveCard(gs, top, seat, "library", "exile", "impulse_play")
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
		gs.LogEvent(Event{
			Kind:   "impulse_play",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "extra_land_drop":
		// CR §305.2 — grant an additional land play this turn. Track via
		// the seat's "extra_land_drops" flag; the land-play action checks
		// this flag to allow more than one land per turn.
		seat := controllerSeat(src)
		n := 1
		if len(e.Args) > 0 {
			if v, ok := asInt(e.Args[0]); ok && v > 0 {
				n = v
			}
		}
		if seat >= 0 && seat < len(gs.Seats) {
			if gs.Seats[seat].Flags == nil {
				gs.Seats[seat].Flags = make(map[string]int)
			}
			gs.Seats[seat].Flags["extra_land_drops"] += n
		}
		gs.LogEvent(Event{
			Kind:   "extra_land_drop",
			Seat:   seat,
			Source: sourceName(src),
			Amount: n,
		})

	case "animate":
		// CR §706 — turn a non-creature permanent into a creature with P/T.
		// "becomes a N/N creature" — typically from Gideon, Nissa, or
		// artifact-animate effects. Args: (power, toughness, [creature_type]).
		// Apply as a modification on the source permanent and add the
		// "creature" type if not already present.
		animPow, animTough := 0, 0
		animType := ""
		if len(e.Args) >= 2 {
			if p, ok := asInt(e.Args[0]); ok {
				animPow = p
			}
			if t, ok := asInt(e.Args[1]); ok {
				animTough = t
			}
		}
		if len(e.Args) >= 3 {
			if ct, ok := e.Args[2].(string); ok {
				animType = ct
			}
		}
		if src != nil {
			src.Modifications = append(src.Modifications, Modification{
				Power:     animPow - src.Card.BasePower,
				Toughness: animTough - src.Card.BaseToughness,
				Duration:  "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			gs.InvalidateCharacteristicsCache()
			// Add creature type if not already present.
			hasCreatureType := false
			for _, tp := range src.Card.Types {
				if tp == "creature" {
					hasCreatureType = true
					break
				}
			}
			if !hasCreatureType {
				src.Card.Types = append(src.Card.Types, "creature")
				if animType != "" {
					src.Card.Types = append(src.Card.Types, strings.ToLower(animType))
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "animate",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"power":         animPow,
				"toughness":     animTough,
				"creature_type": animType,
			},
		})

	case "restriction":
		// CR §802 — restrictions on actions: "can't attack", "can't block",
		// "can't be the target", "can't cast spells", etc. Set a flag on
		// the source permanent (for creature restrictions) or on the game
		// state (for global restrictions) so combat/targeting/casting
		// systems respect the constraint.
		restrictionKind := modArgString(e.Args, 0)
		restrictionTarget := modArgString(e.Args, 1)
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			switch {
			case containsIgnoreCase(restrictionKind, "can't attack"):
				src.Flags["cant_attack"] = 1
			case containsIgnoreCase(restrictionKind, "can't block"):
				src.Flags["cant_block"] = 1
			case containsIgnoreCase(restrictionKind, "can't be blocked"):
				src.Flags["unblockable"] = 1
			case containsIgnoreCase(restrictionKind, "can't be the target"), containsIgnoreCase(restrictionKind, "hexproof"):
				src.Flags["hexproof"] = 1
			case containsIgnoreCase(restrictionKind, "can't be sacrificed"):
				src.Flags["cant_sacrifice"] = 1
			default:
				// Generic restriction flag.
				if restrictionKind != "" {
					src.Flags["restriction_"+strings.ReplaceAll(strings.ToLower(restrictionKind), " ", "_")] = 1
				}
			}
		}
		// Global restrictions (e.g. "players can't gain life", "creatures can't attack").
		if containsIgnoreCase(restrictionTarget, "player") || containsIgnoreCase(restrictionTarget, "opponent") {
			if gs.Flags == nil {
				gs.Flags = map[string]int{}
			}
			if restrictionKind != "" {
				gs.Flags["restriction_"+strings.ReplaceAll(strings.ToLower(restrictionKind), " ", "_")] = 1
			}
		}
		gs.LogEvent(Event{
			Kind:   "restriction_applied",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"restriction": restrictionKind,
				"target":      restrictionTarget,
			},
		})

	case "choose_target":
		// Resolve targeted sub-effects. The parser wraps "target creature
		// gets -N/-N" etc. as choose_target with the sub-effect in args.
		// Attempt to resolve any Effect args; otherwise pick a target and
		// apply the most common targeted pattern (damage or debuff).
		ctResolved := false
		for _, arg := range e.Args {
			if eff, ok := arg.(gameast.Effect); ok {
				ResolveEffect(gs, src, eff)
				ctResolved = true
			}
		}
		if !ctResolved {
			// Parse args for common targeted patterns.
			raw := modArgString(e.Args, 0)
			if raw != "" {
				ctResolved = resolveResidualByText(gs, src, raw)
			}
		}
		gs.LogEvent(Event{
			Kind:   "choose_target",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"resolved": ctResolved,
				"args":     e.Args,
			},
		})

	case "targeted_effect":
		// Resolve a targeted sub-effect. Similar to choose_target but the
		// target has already been chosen. Attempt to resolve Effect args;
		// fall back to text-based resolution.
		teResolved := false
		for _, arg := range e.Args {
			if eff, ok := arg.(gameast.Effect); ok {
				ResolveEffect(gs, src, eff)
				teResolved = true
			}
		}
		if !teResolved {
			raw := modArgString(e.Args, 0)
			if raw != "" {
				teResolved = resolveResidualByText(gs, src, raw)
			}
		}
		gs.LogEvent(Event{
			Kind:   "targeted_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"resolved": teResolved,
				"args":     e.Args,
			},
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
		// "Other creatures you control get +1/+1." Apply a permanent buff
		// to all friendly creatures except the source.
		pow, tough := 1, 1
		if len(e.Args) >= 2 {
			if p, ok := asInt(e.Args[0]); ok {
				pow = p
			}
			if t, ok := asInt(e.Args[1]); ok {
				tough = t
			}
		}
		seat := controllerSeat(src)
		buffed := 0
		if seat >= 0 && seat < len(gs.Seats) {
			ts := gs.NextTimestamp()
			for _, p := range gs.Seats[seat].Battlefield {
				if p == nil || p == src || !p.IsCreature() {
					continue
				}
				p.Modifications = append(p.Modifications, Modification{
					Power:     pow,
					Toughness: tough,
					Duration:  "permanent",
					Timestamp: ts,
				})
				buffed++
			}
			if buffed > 0 {
				gs.InvalidateCharacteristicsCache()
			}
		}
		gs.LogEvent(Event{
			Kind:   "anthem",
			Seat:   seat,
			Source: sourceName(src),
			Amount: buffed,
			Details: map[string]interface{}{
				"power":     pow,
				"toughness": tough,
				"scope":     "other_yours",
			},
		})

	case "saga_chapter":
		// CR §714 — saga chapter ability triggered. Increment the lore
		// counter on the saga and attempt to resolve the chapter effect.
		if src != nil {
			src.AddCounter("lore", 1)
		}
		// Try to resolve any Effect args (the chapter's actual effect).
		scResolved := false
		for _, arg := range e.Args {
			if eff, ok := arg.(gameast.Effect); ok {
				ResolveEffect(gs, src, eff)
				scResolved = true
			}
		}
		if !scResolved {
			raw := modArgString(e.Args, 0)
			if raw != "" {
				scResolved = resolveResidualByText(gs, src, raw)
			}
		}
		gs.LogEvent(Event{
			Kind:   "saga_chapter",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw":      modArgString(e.Args, 0),
				"resolved": scResolved,
				"lore":     func() int {
					if src != nil && src.Counters != nil {
						return src.Counters["lore"]
					}
					return 0
				}(),
			},
		})

	case "delayed_trigger":
		// CR §603.7 — set up a delayed triggered ability. The trigger
		// fires at a specific game event or phase boundary. Parse args
		// for the trigger timing and effect.
		dtSeat := controllerSeat(src)
		dtCardName := sourceName(src)
		dtTiming := "next_end_step"
		dtRaw := modArgString(e.Args, 0)
		// Detect timing from args text.
		dtRawLower := strings.ToLower(dtRaw)
		switch {
		case containsIgnoreCase(dtRawLower, "end of combat"):
			dtTiming = "end_of_combat"
		case containsIgnoreCase(dtRawLower, "next upkeep"), containsIgnoreCase(dtRawLower, "your next upkeep"):
			dtTiming = "next_upkeep"
		case containsIgnoreCase(dtRawLower, "next end step"), containsIgnoreCase(dtRawLower, "end of turn"):
			dtTiming = "next_end_step"
		case containsIgnoreCase(dtRawLower, "next turn"), containsIgnoreCase(dtRawLower, "your next turn"):
			dtTiming = "your_next_turn"
		}
		// Check for Effect args to use as the delayed trigger's effect.
		var dtEffectFn func(gs *GameState)
		for _, arg := range e.Args {
			if eff, ok := arg.(gameast.Effect); ok {
				capturedEff := eff
				capturedSrc := src
				dtEffectFn = func(gs *GameState) {
					ResolveEffect(gs, capturedSrc, capturedEff)
				}
				break
			}
		}
		if dtEffectFn == nil {
			// Default: log the trigger firing with the raw text.
			capturedRaw := dtRaw
			dtEffectFn = func(gs *GameState) {
				gs.LogEvent(Event{
					Kind:   "delayed_trigger_fired",
					Seat:   dtSeat,
					Source: dtCardName,
					Details: map[string]interface{}{
						"raw": capturedRaw,
					},
				})
			}
		}
		gs.RegisterDelayedTrigger(&DelayedTrigger{
			TriggerAt:      dtTiming,
			ControllerSeat: dtSeat,
			SourceCardName: dtCardName,
			OneShot:        true,
			EffectFn:       dtEffectFn,
		})
		gs.LogEvent(Event{
			Kind:   "delayed_trigger_registered",
			Seat:   dtSeat,
			Source: dtCardName,
			Details: map[string]interface{}{
				"trigger_at": dtTiming,
				"raw":        dtRaw,
			},
		})

	case "each_player_effect":
		// Apply an effect to each player. Parse the raw text for common
		// patterns (draw, mill, discard, life loss/gain).
		raw := strings.ToLower(modArgString(e.Args, 0))
		affected := 0
		for i, s := range gs.Seats {
			if s == nil || s.LeftGame {
				continue
			}
			if containsIgnoreCase(raw, "draw") {
				gs.drawOne(i)
			} else if containsIgnoreCase(raw, "mill") {
				gs.millOne(i)
			} else if containsIgnoreCase(raw, "discard") {
				discardN(gs, i, 1, "")
			} else if containsIgnoreCase(raw, "lose") && containsIgnoreCase(raw, "life") {
				n := 1
				if len(e.Args) > 1 {
					if v, ok := asInt(e.Args[1]); ok && v > 0 {
						n = v
					}
				}
				s.Life -= n
			} else if containsIgnoreCase(raw, "gain") && containsIgnoreCase(raw, "life") {
				n := 1
				if len(e.Args) > 1 {
					if v, ok := asInt(e.Args[1]); ok && v > 0 {
						n = v
					}
				}
				s.Life += n
			}
			affected++
		}
		gs.LogEvent(Event{
			Kind:   "each_player_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Amount: affected,
			Details: map[string]interface{}{
				"raw": modArgString(e.Args, 0),
			},
		})

	case "library_bottom":
		// CR §401 — put cards on the bottom of a library. The pronoun
		// context typically refers to a card that was just revealed, looked
		// at, or exiled. Move the most recent exile/hand card to the bottom
		// of the library.
		lbSeat := controllerSeat(src)
		lbCount := 1
		if len(e.Args) > 0 {
			if n, ok := asInt(e.Args[0]); ok && n > 0 {
				lbCount = n
			}
		}
		lbMoved := 0
		if lbSeat >= 0 && lbSeat < len(gs.Seats) {
			s := gs.Seats[lbSeat]
			for i := 0; i < lbCount; i++ {
				var card *Card
				fromZone := ""
				// Prefer hand (most common: scry/look-at puts unwanted cards
				// on the bottom), then exile.
				if len(s.Hand) > 0 {
					card = s.Hand[len(s.Hand)-1]
					fromZone = "hand"
				} else if len(s.Exile) > 0 {
					card = s.Exile[len(s.Exile)-1]
					fromZone = "exile"
				}
				if card != nil {
					removeCardFromZone(gs, lbSeat, card, fromZone)
					s.Library = append(s.Library, card)
					lbMoved++
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "library_bottom",
			Seat:   lbSeat,
			Source: sourceName(src),
			Amount: lbMoved,
		})

	case "pronoun_grant_multi":
		// Grant multiple abilities from args to the source permanent.
		if src != nil && len(e.Args) > 0 {
			for _, arg := range e.Args {
				if kw, ok := arg.(string); ok && kw != "" {
					src.GrantedAbilities = append(src.GrantedAbilities, kw)
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "grant_abilities",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"abilities": e.Args,
			},
		})

	case "draw_per":
		seat := controllerSeat(src)
		count := 0
		if seat >= 0 && seat < len(gs.Seats) {
			// Try to extract count from args; default to creature count.
			if len(e.Args) > 0 {
				if n, ok := asInt(e.Args[0]); ok && n > 0 {
					count = n
				}
			}
			if count == 0 {
				// Count creatures on our battlefield.
				for _, p := range gs.Seats[seat].Battlefield {
					if p != nil && p.IsCreature() {
						count++
					}
				}
			}
			if count < 1 {
				count = 1
			}
			for i := 0; i < count; i++ {
				if _, ok := gs.drawOne(seat); !ok {
					break
				}
			}
		}
		gs.LogEvent(Event{Kind: "draw", Seat: seat, Source: sourceName(src), Amount: count})

	case "put_effect":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			// Inspect args for zone hints.
			argText := strings.ToLower(modArgString(e.Args, 0))
			switch {
			case containsIgnoreCase(argText, "battlefield"):
				// Delegate to put_onto_battlefield logic: pick from exile/hand/library.
				s := gs.Seats[seat]
				var card *Card
				fromZone := ""
				if len(s.Exile) > 0 {
					card = s.Exile[len(s.Exile)-1]
					fromZone = "exile"
				} else if len(s.Hand) > 0 {
					card = s.Hand[len(s.Hand)-1]
					fromZone = "hand"
				} else if len(s.Library) > 0 {
					card = s.Library[0]
					fromZone = "library"
				}
				if card != nil {
					removeCardFromZone(gs, seat, card, fromZone)
					p := &Permanent{
						Card:          card,
						Controller:    seat,
						Tapped:        false,
						SummoningSick: true,
						Timestamp:     gs.NextTimestamp(),
						Counters:      map[string]int{},
						Flags:         map[string]int{},
					}
					s.Battlefield = append(s.Battlefield, p)
					RegisterReplacementsForPermanent(gs, p)
					FirePermanentETBTriggers(gs, p)
				}
			case containsIgnoreCase(argText, "graveyard"):
				// Put top of library into graveyard (e.g. "put that card
				// into your graveyard").
				gs.millOne(seat)
			default:
				// Default: draw a card (put into hand).
				gs.drawOne(seat)
			}
		}
		gs.LogEvent(Event{
			Kind:   "put_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "attacks_each_combat":
		// CR §508.1d — "attacks each combat if able" (goaded, Berserker,
		// etc.). Set the must_attack flag on the source creature so the
		// combat AI forces it to attack. Similar to goad but permanent
		// (not just until next turn).
		if src != nil && src.IsCreature() {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["must_attack"] = 1
		}
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
	// captured but couldn't classify. Attempt to dispatch as residual
	// text via resolveResidualByText; fall back to structured log.
	// -----------------------------------------------------------------
	case "parsed_tail":
		ptRaw := modArgString(e.Args, 0)
		ptResolved := false
		if ptRaw != "" {
			ptResolved = resolveResidualByText(gs, src, ptRaw)
		}
		gs.LogEvent(Event{
			Kind:   "parsed_tail",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw":      ptRaw,
				"resolved": ptResolved,
			},
		})

	// -----------------------------------------------------------------
	// if_intervening_tail — "if [condition], [effect]" tail. Args are
	// (condition_text, body_text). Evaluate the body part as residual
	// text; the condition is assumed true (GreedyHat policy: the card
	// is in the deck, so the controller expects the condition to hold).
	// -----------------------------------------------------------------
	case "if_intervening_tail":
		iitCond := modArgString(e.Args, 0)
		iitBody := modArgString(e.Args, 1)
		iitResolved := false
		if iitBody != "" {
			iitResolved = resolveResidualByText(gs, src, iitBody)
		}
		// If the body didn't match residual patterns, try dispatching
		// any typed Effect args that the loader may have attached.
		if !iitResolved {
			for _, arg := range e.Args {
				if eff, ok := arg.(gameast.Effect); ok {
					ResolveEffect(gs, src, eff)
					iitResolved = true
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "if_intervening_tail",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"condition": iitCond,
				"body":      iitBody,
				"resolved":  iitResolved,
			},
		})

	// -----------------------------------------------------------------
	// custom — per-card custom effect slug from the parser extension
	// layer (per_card.py). Args[0] is the slug, remaining args are
	// parameters. Attempt to resolve any typed Effect args; fall back
	// to text-based residual dispatch on the slug.
	// -----------------------------------------------------------------
	case "custom":
		custSlug := modArgString(e.Args, 0)
		custResolved := false
		// First pass: resolve any typed Effect args.
		for _, arg := range e.Args {
			if eff, ok := arg.(gameast.Effect); ok {
				ResolveEffect(gs, src, eff)
				custResolved = true
			}
		}
		// Second pass: try slug as residual text.
		if !custResolved && custSlug != "" {
			custResolved = resolveResidualByText(gs, src, custSlug)
		}
		gs.LogEvent(Event{
			Kind:   "custom_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"slug":     custSlug,
				"resolved": custResolved,
				"args":     e.Args,
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
	// during your turn." Intentional no-op: these are activation
	// constraints validated at cast/activation time, NOT effects that
	// mutate game state during resolution. Log for analysis only.
	// -----------------------------------------------------------------
	case "timing_restriction":
		// Intentional no-op — timing restrictions are checked before
		// resolution, not applied as effects. See CR §602.2.
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
		// "You may [effect]" — optional effects. GreedyHat policy: always
		// accept optional effects that are beneficial (draw, +1/+1, etc.)
		// because the controller chose to include the card in their deck.
		// Attempt to resolve any Effect args; fall back to text-based.
		oeResolved := false
		for _, arg := range e.Args {
			if eff, ok := arg.(gameast.Effect); ok {
				ResolveEffect(gs, src, eff)
				oeResolved = true
			}
		}
		if !oeResolved {
			raw := modArgString(e.Args, 0)
			if raw != "" {
				oeResolved = resolveResidualByText(gs, src, raw)
			}
		}
		gs.LogEvent(Event{
			Kind:   "optional_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"resolved": oeResolved,
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
	// with_modifier (75) — "with [keyword]" modifier on a creature
	// entering. E.g. "enters with haste", "with flying". Extract the
	// keyword and grant it to the source permanent.
	// -----------------------------------------------------------------
	case "with_modifier":
		wmRaw := modArgString(e.Args, 0)
		wmKeyword := ""
		if wmRaw != "" {
			// Strip the leading "with " prefix to get the keyword.
			wmClean := strings.TrimPrefix(strings.ToLower(wmRaw), "with ")
			// Common keywords that can be granted directly.
			wmKnown := map[string]bool{
				"flying": true, "haste": true, "trample": true,
				"vigilance": true, "lifelink": true, "deathtouch": true,
				"first strike": true, "double strike": true,
				"menace": true, "reach": true, "hexproof": true,
				"indestructible": true, "flash": true, "defender": true,
				"ward": true, "prowess": true, "persist": true,
				"undying": true, "wither": true, "infect": true,
			}
			// Check for exact match or prefix match (e.g. "with haste and trample").
			for kw := range wmKnown {
				if strings.Contains(wmClean, kw) {
					wmKeyword = kw
					if src != nil {
						src.GrantedAbilities = append(src.GrantedAbilities, kw)
					}
				}
			}
		}
		// If we didn't find a known keyword, try residual text dispatch.
		wmResolved := wmKeyword != ""
		if !wmResolved && wmRaw != "" {
			wmResolved = resolveResidualByText(gs, src, wmRaw)
		}
		gs.LogEvent(Event{
			Kind:   "with_modifier",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw":      wmRaw,
				"keyword":  wmKeyword,
				"resolved": wmResolved,
			},
		})

	// -----------------------------------------------------------------
	// level_marker (66) — Level Up creature level markers (CR §702.87).
	// Args[0] is the level number (int). Update the creature's level
	// counter to reflect the marker — this is the structural metadata
	// that tells the engine which bracket applies.
	// -----------------------------------------------------------------
	case "level_marker":
		lmLevel := modArgInt(e.Args, 0)
		if src != nil && lmLevel > 0 {
			if src.Counters == nil {
				src.Counters = map[string]int{}
			}
			// Set level counter to this marker's value if it's higher
			// than current level (progressive level-up).
			if src.Counters["level"] < lmLevel {
				src.Counters["level"] = lmLevel
			}
			gs.InvalidateCharacteristicsCache()
		}
		gs.LogEvent(Event{
			Kind:   "level_marker",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"level": lmLevel,
			},
		})

	// -----------------------------------------------------------------
	// Orphaned fragment group — conjunctions/fragments that lost their
	// parent during parsing. Try to dispatch as residual text; fall
	// back to structured log.
	// -----------------------------------------------------------------
	case "orphaned_conjunction", "orphaned_fragment":
		ocRaw := modArgString(e.Args, 0)
		ocResolved := false
		if ocRaw != "" {
			ocResolved = resolveResidualByText(gs, src, ocRaw)
		}
		// Also try any typed Effect args.
		if !ocResolved {
			for _, arg := range e.Args {
				if eff, ok := arg.(gameast.Effect); ok {
					ResolveEffect(gs, src, eff)
					ocResolved = true
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "orphaned_fragment",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"fragment_kind": e.ModKind,
				"raw":           ocRaw,
				"resolved":      ocResolved,
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
		// "Attach it to target creature." — same aura/equipment attachment
		// logic as attach_aura_target_creature. Set AttachedTo on the source.
		apTargetName := ""
		if src != nil {
			targets := pickTargetFromModArgs(gs, src, e.Args)
			for _, t := range targets {
				if t.Kind == TargetKindPermanent && t.Permanent != nil && t.Permanent != src {
					src.AttachedTo = t.Permanent
					if t.Permanent.Card != nil {
						apTargetName = t.Permanent.Card.DisplayName()
					}
					break
				}
			}
			// Fallback: pick any friendly creature.
			if src.AttachedTo == nil {
				apSeat := controllerSeat(src)
				if apSeat >= 0 && apSeat < len(gs.Seats) {
					for _, p := range gs.Seats[apSeat].Battlefield {
						if p != nil && p != src && p.IsCreature() {
							src.AttachedTo = p
							if p.Card != nil {
								apTargetName = p.Card.DisplayName()
							}
							break
						}
					}
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "attach_pronoun_to",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"attached_to": apTargetName,
				"args":        e.Args,
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
	// until_duration_effect (30) — "until end of turn" / "until your
	// next turn" duration wrapper. Args[0] is the raw text like
	// "until end of turn, target creature gets +2/+2". Parse out the
	// duration and attempt to resolve the sub-effect via residual text.
	// -----------------------------------------------------------------
	case "until_duration_effect":
		udeRaw := modArgString(e.Args, 0)
		udeResolved := false
		if udeRaw != "" {
			// Strip the duration prefix to get the effect body.
			udeBody := udeRaw
			for _, prefix := range []string{
				"until end of turn, ",
				"until end of turn ",
				"until your next turn, ",
				"until your next turn ",
				"until your next upkeep, ",
				"until your next upkeep ",
			} {
				if strings.HasPrefix(strings.ToLower(udeBody), prefix) {
					udeBody = udeBody[len(prefix):]
					break
				}
			}
			udeResolved = resolveResidualByText(gs, src, udeBody)
		}
		// Also try any typed Effect args.
		if !udeResolved {
			for _, arg := range e.Args {
				if eff, ok := arg.(gameast.Effect); ok {
					ResolveEffect(gs, src, eff)
					udeResolved = true
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "until_duration_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw":      udeRaw,
				"resolved": udeResolved,
			},
		})

	// -----------------------------------------------------------------
	// may_pay_generic (27) — "you may pay {N}" clause. Log as
	// structured event; policy agent decides whether to pay.
	// -----------------------------------------------------------------
	case "may_pay_generic":
		// "You may pay {N}" choice effects. GreedyHat policy: pay the cost
		// if we can afford it (mana pool >= cost), then resolve the
		// associated effect from args.
		mpgCost := modArgInt(e.Args, 0)
		mpgSeat := controllerSeat(src)
		mpgPaid := false
		if mpgSeat >= 0 && mpgSeat < len(gs.Seats) {
			if gs.Seats[mpgSeat].ManaPool >= mpgCost {
				gs.Seats[mpgSeat].ManaPool -= mpgCost
				mpgPaid = true
				// Resolve the effect (args may contain Effect or raw text).
				for _, arg := range e.Args {
					if eff, ok := arg.(gameast.Effect); ok {
						ResolveEffect(gs, src, eff)
					}
				}
				if len(e.Args) > 1 {
					raw := modArgString(e.Args, 1)
					if raw != "" {
						resolveResidualByText(gs, src, raw)
					}
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "may_pay_generic",
			Seat:   mpgSeat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"cost": mpgCost,
				"paid": mpgPaid,
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// keyword_action (25) — generic keyword action that the parser
	// captured but didn't map to a specific handler. Args[0] is the
	// raw text like "scry 2", "surveil 1", "investigate", "explore",
	// "mill 3", etc. Map common keyword actions to concrete effects.
	// -----------------------------------------------------------------
	case "keyword_action":
		kaRaw := strings.ToLower(modArgString(e.Args, 0))
		kaResolved := false
		kaSeat := controllerSeat(src)
		if kaSeat < 0 {
			kaSeat = 0
		}
		// Extract a trailing count (e.g. "scry 2" -> count=2).
		kaCount := 1
		if m := reKeywordActionCount.FindStringSubmatch(kaRaw); m != nil {
			if v, err := strconv.Atoi(m[1]); err == nil && v > 0 {
				kaCount = v
			}
		}
		switch {
		case strings.HasPrefix(kaRaw, "scry"):
			ResolveEffect(gs, src, &gameast.Scry{Count: *gameast.NumInt(kaCount)})
			kaResolved = true
		case strings.HasPrefix(kaRaw, "surveil"):
			ResolveEffect(gs, src, &gameast.Surveil{Count: *gameast.NumInt(kaCount)})
			kaResolved = true
		case strings.HasPrefix(kaRaw, "investigate"):
			for i := 0; i < kaCount; i++ {
				CreateClueToken(gs, kaSeat)
			}
			kaResolved = true
		case strings.HasPrefix(kaRaw, "explore"):
			// CR §701.40: reveal top card; if land, put to hand; else +1/+1
			// counter and optionally put to graveyard. MVP: draw one card.
			gs.drawOne(kaSeat)
			kaResolved = true
		case strings.HasPrefix(kaRaw, "mill"):
			for i := 0; i < kaCount; i++ {
				gs.millOne(kaSeat)
			}
			kaResolved = true
		case strings.HasPrefix(kaRaw, "proliferate"):
			// CR §701.27: add one counter of each kind to any number of
			// permanents/players that already have counters. MVP: add a
			// +1/+1 counter to each friendly creature that has any counters.
			if kaSeat >= 0 && kaSeat < len(gs.Seats) {
				for _, p := range gs.Seats[kaSeat].Battlefield {
					if p != nil && p.IsCreature() && len(p.Counters) > 0 {
						for kind := range p.Counters {
							p.Counters[kind]++
							break // one counter type per permanent for MVP
						}
					}
				}
				gs.InvalidateCharacteristicsCache()
			}
			kaResolved = true
		case strings.HasPrefix(kaRaw, "connive"):
			// CR §701.47: draw a card, then discard a card. If you discarded
			// a nonland card, put a +1/+1 counter on the conniving creature.
			gs.drawOne(kaSeat)
			gs.millOne(kaSeat) // simplified discard
			if src != nil && src.IsCreature() {
				src.AddCounter("+1/+1", 1)
				gs.InvalidateCharacteristicsCache()
			}
			kaResolved = true
		case strings.HasPrefix(kaRaw, "venture"):
			// Dungeon mechanic — log only, dungeon state not modeled.
			kaResolved = false
		case strings.HasPrefix(kaRaw, "discover"):
			// CR §701.55-ish: exile cards from top of library until you hit
			// one with MV <= N, then cast or put to hand. MVP: draw one.
			gs.drawOne(kaSeat)
			kaResolved = true
		case strings.HasPrefix(kaRaw, "adapt"):
			// Adapt N — already has its own case above, but might arrive
			// via keyword_action too. Apply if no +1/+1 counters.
			if src != nil && src.IsCreature() {
				if src.Counters == nil || src.Counters["+1/+1"] == 0 {
					src.AddCounter("+1/+1", kaCount)
					gs.InvalidateCharacteristicsCache()
				}
			}
			kaResolved = true
		case strings.HasPrefix(kaRaw, "bolster"):
			// Bolster N — put N +1/+1 counters on the creature you control
			// with the least toughness.
			if kaSeat >= 0 && kaSeat < len(gs.Seats) {
				var weakest *Permanent
				for _, p := range gs.Seats[kaSeat].Battlefield {
					if p == nil || !p.IsCreature() {
						continue
					}
					if weakest == nil || p.Toughness() < weakest.Toughness() {
						weakest = p
					}
				}
				if weakest != nil {
					weakest.AddCounter("+1/+1", kaCount)
					gs.InvalidateCharacteristicsCache()
				}
			}
			kaResolved = true
		case strings.HasPrefix(kaRaw, "populate"):
			// CR §701.30: create a copy of a creature token you control.
			// MVP: log only (token copying not fully modeled).
			kaResolved = false
		case strings.HasPrefix(kaRaw, "amass"):
			// CR §701.44: create an Army token or put +1/+1 counters on
			// one you already control. MVP: create a 0/0 token and add
			// N +1/+1 counters.
			if kaSeat >= 0 && kaSeat < len(gs.Seats) {
				// Look for existing Army token.
				var army *Permanent
				for _, p := range gs.Seats[kaSeat].Battlefield {
					if p != nil && p.Flags != nil && p.Flags["army_token"] == 1 {
						army = p
						break
					}
				}
				if army != nil {
					army.AddCounter("+1/+1", kaCount)
				} else {
					// Create new Army token.
					armyPerm := &Permanent{
						Card: &Card{
							Name:          "Zombie Army",
							Owner:         kaSeat,
							BasePower:     0,
							BaseToughness: 0,
							Types:         []string{"creature"},
							TypeLine:      "Creature Token — Zombie Army",
						},
						Controller:    kaSeat,
						Owner:         kaSeat,
						Timestamp:     gs.NextTimestamp(),
						Counters:      map[string]int{"+1/+1": kaCount},
						Flags:         map[string]int{"token": 1, "army_token": 1},
						SummoningSick: true,
					}
					gs.Seats[kaSeat].Battlefield = append(gs.Seats[kaSeat].Battlefield, armyPerm)
					RegisterReplacementsForPermanent(gs, armyPerm)
					FirePermanentETBTriggers(gs, armyPerm)
				}
				gs.InvalidateCharacteristicsCache()
			}
			kaResolved = true
		default:
			// Fall back to text-based residual dispatch.
			kaResolved = resolveResidualByText(gs, src, modArgString(e.Args, 0))
		}
		gs.LogEvent(Event{
			Kind:   "keyword_action",
			Seat:   kaSeat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"action":   modArgString(e.Args, 0),
				"resolved": kaResolved,
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
	// draft_from_spellbook (22) — Arena/Alchemy digital-only mechanic.
	// Intentional no-op: not relevant for paper MTG simulation.
	// Spellbooks are curated card pools that don't exist in tabletop.
	// -----------------------------------------------------------------
	case "draft_from_spellbook":
		// Intentional no-op — digital-only Alchemy mechanic (Arena).
		gs.LogEvent(Event{
			Kind:   "draft_from_spellbook",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"skipped": "digital_only",
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
		// "For each X" scaling modifier — count relevant permanents and
		// apply a scaled effect. Args[0] is typically the thing to count,
		// Args[1] may be the effect to scale.
		seat := controllerSeat(src)
		count := 0
		countWhat := strings.ToLower(modArgString(e.Args, 0))
		if seat >= 0 && seat < len(gs.Seats) {
			// Count matching permanents on our battlefield.
			for _, p := range gs.Seats[seat].Battlefield {
				if p == nil {
					continue
				}
				if containsIgnoreCase(countWhat, "creature") && p.IsCreature() {
					count++
				} else if containsIgnoreCase(countWhat, "artifact") && p.IsArtifact() {
					count++
				} else if containsIgnoreCase(countWhat, "enchantment") && p.IsEnchantment() {
					count++
				} else if containsIgnoreCase(countWhat, "land") && p.IsLand() {
					count++
				} else if countWhat == "" {
					// No filter specified — count all permanents.
					count++
				}
			}
			// Extract multiplier from args if present.
			multiplier := 1
			if len(e.Args) > 1 {
				if m, ok := asInt(e.Args[1]); ok && m > 0 {
					multiplier = m
				}
			}
			scaledAmount := count * multiplier
			// Apply scaled damage to an opponent as the default effect.
			if scaledAmount > 0 {
				opps := gs.Opponents(seat)
				if len(opps) > 0 {
					opp := opps[0]
					gs.Seats[opp].Life -= scaledAmount
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "for_each_scaling",
			Seat:   seat,
			Source: sourceName(src),
			Amount: count,
			Details: map[string]interface{}{
				"count_what": countWhat,
				"args":       e.Args,
			},
		})

	// -----------------------------------------------------------------
	// may_play_land_from_hand (18) — "you may play an additional land
	// this turn." Grant the extra land drop permission.
	// -----------------------------------------------------------------
	case "may_play_land_from_hand":
		// "You may play an additional land this turn." Grant the extra
		// land drop via the seat's flag (same mechanism as extra_land_drop).
		mplSeat := controllerSeat(src)
		if mplSeat >= 0 && mplSeat < len(gs.Seats) {
			if gs.Seats[mplSeat].Flags == nil {
				gs.Seats[mplSeat].Flags = make(map[string]int)
			}
			gs.Seats[mplSeat].Flags["extra_land_drops"]++
		}
		gs.LogEvent(Event{
			Kind:   "extra_land_drop",
			Seat:   mplSeat,
			Source: sourceName(src),
			Amount: 1,
			Details: map[string]interface{}{
				"permission": "play_land_from_hand",
			},
		})

	// -----------------------------------------------------------------
	// copy_pronoun (17) — "copy it" pronoun reference. Structural
	// reference; log for copy-resolution pipeline.
	// -----------------------------------------------------------------
	case "copy_pronoun":
		// "Copy it" / "copy that permanent" — create a token copy of the
		// referenced permanent/spell. Delegate to CopySpell for spell copies;
		// for permanent copies, create a token copy of the source.
		cpSeat := controllerSeat(src)
		if src != nil && src.Card != nil && cpSeat >= 0 && cpSeat < len(gs.Seats) {
			// Create a token copy of the source permanent.
			tokenCard := &Card{
				Name:          src.Card.Name,
				Owner:         cpSeat,
				BasePower:     src.Card.BasePower,
				BaseToughness: src.Card.BaseToughness,
				Types:         append([]string{}, src.Card.Types...),
				Colors:        append([]string{}, src.Card.Colors...),
				CMC:           src.Card.CMC,
			}
			copyPerm := &Permanent{
				Card:          tokenCard,
				Controller:    cpSeat,
				Owner:         cpSeat,
				SummoningSick: true,
				Timestamp:     gs.NextTimestamp(),
				Counters:      map[string]int{},
				Flags:         map[string]int{},
			}
			gs.Seats[cpSeat].Battlefield = append(gs.Seats[cpSeat].Battlefield, copyPerm)
			RegisterReplacementsForPermanent(gs, copyPerm)
			FirePermanentETBTriggers(gs, copyPerm)
		} else {
			// Fallback: delegate to CopySpell for spell-on-stack copies.
			ResolveEffect(gs, src, &gameast.CopySpell{})
		}
		gs.LogEvent(Event{
			Kind:   "copy_pronoun",
			Seat:   cpSeat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	// -----------------------------------------------------------------
	// earthbend (16) — Alchemy/digital-only supplemental mechanic.
	// Intentional no-op: not relevant for paper MTG simulation.
	// Earthbend modifies basic land types in ways specific to Arena.
	// -----------------------------------------------------------------
	case "earthbend":
		// Intentional no-op — digital-only Alchemy mechanic (Arena).
		gs.LogEvent(Event{
			Kind:   "earthbend",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"skipped": "digital_only",
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
		// Attach equipment/aura to a target. Same pattern as attach_pronoun_to:
		// set AttachedTo on the source permanent.
		aeTargetName := ""
		if src != nil {
			targets := pickTargetFromModArgs(gs, src, e.Args)
			for _, t := range targets {
				if t.Kind == TargetKindPermanent && t.Permanent != nil && t.Permanent != src {
					src.AttachedTo = t.Permanent
					if t.Permanent.Card != nil {
						aeTargetName = t.Permanent.Card.DisplayName()
					}
					break
				}
			}
			if src.AttachedTo == nil {
				aeSeat := controllerSeat(src)
				if aeSeat >= 0 && aeSeat < len(gs.Seats) {
					for _, p := range gs.Seats[aeSeat].Battlefield {
						if p != nil && p != src && p.IsCreature() {
							src.AttachedTo = p
							if p.Card != nil {
								aeTargetName = p.Card.DisplayName()
							}
							break
						}
					}
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "attach_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"attached_to": aeTargetName,
				"args":        e.Args,
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
		// CR §701.55 (LCI) — "Discover N": exile cards from the top of
		// your library until you exile a nonland card with mana value N
		// or less. You may cast that card without paying its mana cost,
		// or put it into your hand. Put the remaining exiled cards on the
		// bottom of your library in a random order.
		discN := 0
		if len(e.Args) > 0 {
			if v, ok := asInt(e.Args[0]); ok {
				discN = v
			}
		}
		discSeat := controllerSeat(src)
		discFound := false
		if discSeat >= 0 && discSeat < len(gs.Seats) {
			s := gs.Seats[discSeat]
			var exiled []*Card
			var foundCard *Card
			// Exile cards until we find a nonland with CMC <= N.
			for len(s.Library) > 0 && !discFound {
				top := s.Library[0]
				s.Library = s.Library[1:]
				isLand := false
				if top != nil {
					for _, tp := range top.Types {
						if strings.EqualFold(tp, "land") {
							isLand = true
							break
						}
					}
				}
				if !isLand && top != nil && top.CMC <= discN {
					foundCard = top
					discFound = true
				} else {
					exiled = append(exiled, top)
				}
				// Safety cap to prevent infinite loops on malformed libraries.
				if len(exiled) > 200 {
					break
				}
			}
			// Cast the found card (put into hand — the card was removed from
			// library above, so just add it directly to hand).
			if foundCard != nil {
				// GreedyHat: put into hand (casting from discover requires
				// stack modeling; putting into hand is the fallback).
				s.Hand = append(s.Hand, foundCard)
			}
			// Put remaining exiled cards on the bottom in random order.
			if len(exiled) > 0 {
				if gs.Rng != nil && len(exiled) > 1 {
					gs.Rng.Shuffle(len(exiled), func(i, j int) {
						exiled[i], exiled[j] = exiled[j], exiled[i]
					})
				}
				s.Library = append(s.Library, exiled...)
			}
		}
		gs.LogEvent(Event{
			Kind:   "discover",
			Seat:   discSeat,
			Source: sourceName(src),
			Amount: discN,
			Details: map[string]interface{}{
				"found": discFound,
				"rule":  "701.55",
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
	// during_phase_effect — "during your [phase]" timing. Args[0] is
	// raw text like "during your upkeep, draw a card". Parse the phase
	// and resolve the sub-effect via residual text if the game is
	// currently in the specified phase. Also handles starting_with_you
	// which is a structural turn-order marker (no-op).
	// -----------------------------------------------------------------
	case "during_phase_effect", "starting_with_you":
		dpeRaw := modArgString(e.Args, 0)
		dpeResolved := false
		if e.ModKind == "during_phase_effect" && dpeRaw != "" {
			dpeLower := strings.ToLower(dpeRaw)
			// Extract the phase and sub-effect from "during your [phase], [effect]"
			dpeBody := ""
			dpePhaseMatch := false
			for _, ph := range []struct{ prefix, phase string }{
				{"during your upkeep, ", "beginning"},
				{"during your upkeep ", "beginning"},
				{"during each player's upkeep, ", "beginning"},
				{"during each player's upkeep ", "beginning"},
				{"during combat, ", "combat"},
				{"during combat ", "combat"},
				{"during your end step, ", "ending"},
				{"during your end step ", "ending"},
				{"during your draw step, ", "beginning"},
				{"during your draw step ", "beginning"},
			} {
				if strings.HasPrefix(dpeLower, ph.prefix) {
					dpeBody = dpeRaw[len(ph.prefix):]
					// Check if we're currently in the matching phase.
					if gs.Phase == ph.phase || ph.phase == "" {
						dpePhaseMatch = true
					}
					break
				}
			}
			// If no phase prefix matched, try resolving the whole text.
			if dpeBody == "" {
				dpeBody = dpeRaw
				dpePhaseMatch = true // no phase constraint extracted
			}
			if dpePhaseMatch && dpeBody != "" {
				dpeResolved = resolveResidualByText(gs, src, dpeBody)
			}
			// Also try any typed Effect args.
			if !dpeResolved {
				for _, arg := range e.Args {
					if eff, ok := arg.(gameast.Effect); ok {
						ResolveEffect(gs, src, eff)
						dpeResolved = true
					}
				}
			}
		}
		// starting_with_you is a structural turn-order marker — no mutation.
		gs.LogEvent(Event{
			Kind:   e.ModKind,
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw":      dpeRaw,
				"resolved": dpeResolved,
			},
		})

	// -----------------------------------------------------------------
	// counter_spell_ability (10) — "counter target spell or ability."
	// Dispatch: counter the top item on the stack (same pattern as
	// counter_that_spell_unless_pay but unconditional).
	// -----------------------------------------------------------------
	case "counter_spell_ability":
		// Set flag so downstream handlers know an ability was countered.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["last_ability_countered"] = 1
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
		// Mark the source as "must be blocked" — combat AI should force
		// an opponent creature to block it if able (CR §509.1c).
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["must_be_blocked"] = 1
		}
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
		// CR §701.21 — "tap or untap target permanent." Simulation heuristic:
		// untap is almost always more valuable than tapping, so default to
		// untapping the source permanent.
		if src != nil {
			src.Tapped = false
		}
		gs.LogEvent(Event{
			Kind:   "tap_untap",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"mod_kind": e.ModKind,
				"args":     e.Args,
				"action":   "untap",
			},
		})

	case "self_enters_tapped", "enters_tapped":
		// CR §614.1d — replacement effect: the permanent enters the
		// battlefield tapped instead of untapped. Set the flag so other
		// ETB replacement handlers can see it, and tap the permanent.
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["enters_tapped"] = 1
			src.Tapped = true
		}
		gs.LogEvent(Event{
			Kind:   "enters_tapped",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "choose_effect", "choose_player", "you_choose_nonland_card":
		// Simulation auto-choice: pick first available option.
		chosenOption := "first"
		seat := controllerSeat(src)
		if e.ModKind == "choose_player" {
			// Pick the first opponent as the chosen player.
			if seat >= 0 && seat < len(gs.Seats) {
				opps := gs.Opponents(seat)
				if len(opps) > 0 {
					chosenOption = fmt.Sprintf("seat_%d", opps[0])
				}
			}
		} else if e.ModKind == "you_choose_nonland_card" {
			// Pick the first nonland card from the target's hand (sim heuristic).
			if seat >= 0 && seat < len(gs.Seats) {
				opps := gs.Opponents(seat)
				for _, opp := range opps {
					for _, c := range gs.Seats[opp].Hand {
						if c != nil {
							isLand := false
							for _, t := range c.Types {
								if t == "land" {
									isLand = true
									break
								}
							}
							if !isLand {
								chosenOption = c.DisplayName()
								break
							}
						}
					}
					if chosenOption != "first" {
						break
					}
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "choice",
			Seat:   seat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
				"args":     e.Args,
				"chosen":   chosenOption,
			},
		})

	case "stat_modification", "switch_pt_self", "switch_pt_target", "switch_pt":
		switch e.ModKind {
		case "switch_pt_self":
			// Swap the source creature's base power and toughness (CR §613.4 layer 7e).
			if src != nil && src.Card != nil {
				src.Card.BasePower, src.Card.BaseToughness = src.Card.BaseToughness, src.Card.BasePower
			}
		case "switch_pt_target":
			// Swap target creature's base power and toughness.
			targets := pickTargetFromModArgs(gs, src, e.Args)
			for _, t := range targets {
				if t.Kind == TargetKindPermanent && t.Permanent != nil && t.Permanent.Card != nil {
					t.Permanent.Card.BasePower, t.Permanent.Card.BaseToughness = t.Permanent.Card.BaseToughness, t.Permanent.Card.BasePower
				}
			}
		case "switch_pt":
			// Generic switch P/T — apply to source if no target hint in args.
			targets := pickTargetFromModArgs(gs, src, e.Args)
			if len(targets) > 0 {
				for _, t := range targets {
					if t.Kind == TargetKindPermanent && t.Permanent != nil && t.Permanent.Card != nil {
						t.Permanent.Card.BasePower, t.Permanent.Card.BaseToughness = t.Permanent.Card.BaseToughness, t.Permanent.Card.BasePower
					}
				}
			} else if src != nil && src.Card != nil {
				src.Card.BasePower, src.Card.BaseToughness = src.Card.BaseToughness, src.Card.BasePower
			}
		}
		// stat_modification is a log-only stub — full stat mods flow through
		// Modifications and the §613 layer system.
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
		seat := controllerSeat(src)
		lifePaid := 0
		if e.ModKind == "may_pay_life" {
			// Simulation heuristic: auto-pay 1 life for optional life payments.
			if seat >= 0 && seat < len(gs.Seats) && gs.Seats[seat].Life > 1 {
				gs.Seats[seat].Life -= 1
				lifePaid = 1
			}
		} else if e.ModKind == "pay_any_amount" {
			// Simulation heuristic: pay 1 life for "pay any amount" effects.
			if seat >= 0 && seat < len(gs.Seats) && gs.Seats[seat].Life > 1 {
				gs.Seats[seat].Life -= 1
				lifePaid = 1
			}
		}
		// pay_cost_effect is generic — no auto-pay for simulation.
		gs.LogEvent(Event{
			Kind:   "pay_cost",
			Seat:   seat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind":  e.ModKind,
				"args":      e.Args,
				"life_paid": lifePaid,
			},
		})

	case "transform_effect", "convert", "flip_creature":
		// CR §712 — toggle the permanent's transformed state. Delegate to
		// TransformPermanent which handles DFC face swapping, timestamp
		// updates, and characteristics cache invalidation. For non-DFC
		// permanents it toggles the Flags["transformed"] counter.
		if src != nil {
			if src.FrontFaceAST != nil && src.BackFaceAST != nil {
				TransformPermanent(gs, src, e.ModKind)
			} else {
				// Non-DFC: toggle a flag so other handlers can observe.
				if src.Flags == nil {
					src.Flags = map[string]int{}
				}
				if src.Flags["transformed"] != 0 {
					src.Flags["transformed"] = 0
				} else {
					src.Flags["transformed"] = 1
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "transform",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "attach_self_to", "attach_to_target", "reattach_aura", "attach_aura_to_creature":
		// Pick a target creature and attach src to it. For auras/equipment
		// the AttachedTo pointer tracks the host permanent.
		targetName := ""
		if src != nil {
			targets := pickTargetFromModArgs(gs, src, e.Args)
			for _, t := range targets {
				if t.Kind == TargetKindPermanent && t.Permanent != nil && t.Permanent != src {
					src.AttachedTo = t.Permanent
					if t.Permanent.Card != nil {
						targetName = t.Permanent.Card.DisplayName()
					}
					break
				}
			}
			// Fallback: pick any friendly creature if mod args didn't resolve.
			if src.AttachedTo == nil {
				seat := controllerSeat(src)
				if seat >= 0 && seat < len(gs.Seats) {
					for _, p := range gs.Seats[seat].Battlefield {
						if p != nil && p != src && p.IsCreature() {
							src.AttachedTo = p
							if p.Card != nil {
								targetName = p.Card.DisplayName()
							}
							break
						}
					}
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "attach",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind":    e.ModKind,
				"args":        e.Args,
				"attached_to": targetName,
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
		// CR §701.34 — "manifest the top card of your library": put it
		// onto the battlefield face down as a 2/2 creature. If it's a
		// creature card, it can be turned face up for its mana cost.
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			s := gs.Seats[seat]
			if len(s.Library) > 0 {
				card := s.Library[0]
				s.Library = s.Library[1:]
				card.FaceDown = true
				perm := &Permanent{
					Card: &Card{
						Name:          "Face-Down Creature",
						Owner:         seat,
						BasePower:     2,
						BaseToughness: 2,
						Types:         []string{"creature"},
						FaceDown:      true,
					},
					Controller:    seat,
					Owner:         seat,
					Timestamp:     gs.NextTimestamp(),
					Counters:      map[string]int{},
					Flags:         map[string]int{"manifested": 1},
					SummoningSick: true,
				}
				perm.FrontFaceAST = card.AST
				perm.FrontFaceName = card.DisplayName()
				s.Battlefield = append(s.Battlefield, perm)
				RegisterReplacementsForPermanent(gs, perm)
				FirePermanentETBTriggers(gs, perm)
			}
		}
		gs.LogEvent(Event{
			Kind:   "manifest",
			Seat:   seat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
				"rule":     "701.34",
			},
		})

	case "shuffle_pronoun_into_owner_library", "shuffle_self_into_library":
		if src != nil {
			owner := src.Owner
			if owner < 0 || owner >= len(gs.Seats) {
				owner = src.Controller
			}
			// Remove from battlefield, add card to owner's library, shuffle.
			removePermanentFromBattlefield(gs, src)
			gs.moveToZone(owner, src.Card, "library_bottom")
			shuffleLibrary(gs, owner)
			FireZoneChangeTriggers(gs, src, src.Card, "battlefield", "library")
		}
		gs.LogEvent(Event{
			Kind:   "shuffle_into_library",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "remove_effect":
		// "Remove N counters from ..." / "remove all abilities" / etc.
		// Parse args to determine what to remove.
		reRaw := strings.ToLower(modArgString(e.Args, 0))
		reCount := 1
		if len(e.Args) > 1 {
			if n, ok := asInt(e.Args[1]); ok && n > 0 {
				reCount = n
			}
		}
		if src != nil {
			switch {
			case containsIgnoreCase(reRaw, "counter"):
				// Remove counters. Try to identify counter kind.
				counterKind := "+1/+1"
				if containsIgnoreCase(reRaw, "-1/-1") {
					counterKind = "-1/-1"
				} else if containsIgnoreCase(reRaw, "loyalty") {
					counterKind = "loyalty"
				} else if containsIgnoreCase(reRaw, "charge") {
					counterKind = "charge"
				} else if containsIgnoreCase(reRaw, "time") {
					counterKind = "time"
				}
				if src.Counters != nil && src.Counters[counterKind] > 0 {
					removed := reCount
					if removed > src.Counters[counterKind] {
						removed = src.Counters[counterKind]
					}
					src.Counters[counterKind] -= removed
					if src.Counters[counterKind] <= 0 {
						delete(src.Counters, counterKind)
					}
					gs.InvalidateCharacteristicsCache()
				}
			case containsIgnoreCase(reRaw, "all abilities"), containsIgnoreCase(reRaw, "all other abilities"):
				// Remove all granted abilities.
				src.GrantedAbilities = nil
			case containsIgnoreCase(reRaw, "all counters"):
				// Remove all counters from the permanent.
				src.Counters = map[string]int{}
				gs.InvalidateCharacteristicsCache()
			}
		}
		gs.LogEvent(Event{
			Kind:   "remove",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"raw":   reRaw,
				"count": reCount,
				"args":  e.Args,
			},
		})

	case "clash":
		// CR §701.23 — "Clash with an opponent." Each player reveals the
		// top card of their library. Higher mana value wins. Winner may
		// put their card on top or bottom; loser puts on bottom.
		// Simulation: peek, compare, put both back on top (no net change).
		seat := controllerSeat(src)
		myCMC := 0
		oppCMC := 0
		won := false
		if seat >= 0 && seat < len(gs.Seats) && len(gs.Seats[seat].Library) > 0 {
			myCMC = gs.Seats[seat].Library[0].CMC
			opps := gs.LivingOpponents(seat)
			if len(opps) > 0 {
				opp := opps[0]
				if len(gs.Seats[opp].Library) > 0 {
					oppCMC = gs.Seats[opp].Library[0].CMC
				}
			}
			won = myCMC > oppCMC
		}
		gs.LogEvent(Event{
			Kind:   "clash",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"my_cmc":  myCMC,
				"opp_cmc": oppCMC,
				"won":     won,
			},
		})

	case "return_exiled_to_hand":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			exile := gs.Seats[seat].Exile
			if len(exile) > 0 {
				card := exile[len(exile)-1]
				MoveCard(gs, card, seat, "exile", "hand", "return_exiled_to_hand")
			}
		}
		gs.LogEvent(Event{
			Kind:   "return_from_exile",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "search_effect":
		ResolveEffect(gs, src, &gameast.Tutor{})
		gs.LogEvent(Event{
			Kind:   "search",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "copy_effect", "copy_next_instant_sorcery":
		ResolveEffect(gs, src, &gameast.CopySpell{})
		gs.LogEvent(Event{
			Kind:   "copy",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "sac_it_at_eoc", "sacrifice_effect":
		ResolveEffect(gs, src, &gameast.Sacrifice{})
		gs.LogEvent(Event{
			Kind:   "sacrifice",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "learn":
		// CR §701.45 — "Learn": you may reveal a Lesson card from outside
		// the game and put it into your hand, or you may discard a card
		// to draw a card. MVP: discard-then-draw (rummage), since the
		// sideboard/outside-the-game zone isn't modeled.
		learnSeat := controllerSeat(src)
		if learnSeat >= 0 && learnSeat < len(gs.Seats) {
			s := gs.Seats[learnSeat]
			if len(s.Hand) > 0 {
				// Discard worst card (last in hand), then draw.
				discardN(gs, learnSeat, 1, "")
				gs.drawOne(learnSeat)
			} else {
				// Empty hand — just draw.
				gs.drawOne(learnSeat)
			}
		}
		gs.LogEvent(Event{
			Kind:   "learn",
			Seat:   learnSeat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"mode": "rummage",
				"rule": "701.45",
			},
		})

	case "any_player_may_effect", "any_player_may_sac", "may_cheat_creature":
		// For may_cheat_creature: put a creature from hand onto the
		// battlefield (Elvish Piper, Quicksilver Amulet). Controller always
		// accepts this — it's pure upside.
		// For any_player_may_effect / any_player_may_sac: optional effects
		// default to "no" for opponents in simulation.
		cheated := false
		if e.ModKind == "may_cheat_creature" {
			seat := controllerSeat(src)
			if seat >= 0 && seat < len(gs.Seats) {
				hand := gs.Seats[seat].Hand
				for i, c := range hand {
					if c == nil {
						continue
					}
					for _, t := range c.Types {
						if t == "creature" {
							MoveCard(gs, c, seat, "hand", "battlefield", "cheat_creature")
							_ = i // MoveCard handles removal
							cheated = true
							break
						}
					}
					if cheated {
						break
					}
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "may_effect",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
				"args":     e.Args,
				"cheated":  cheated,
			},
		})

	case "reveal_effect":
		ResolveEffect(gs, src, &gameast.Reveal{})
		gs.LogEvent(Event{
			Kind:   "reveal",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "library_manipulation", "reorder_top_of_library", "put_cards_from_hand_on_top":
		seat := controllerSeat(src)
		if seat >= 0 && seat < len(gs.Seats) {
			s := gs.Seats[seat]
			switch e.ModKind {
			case "reorder_top_of_library":
				// Shuffle the top N cards (MVP: top 3 or library size, whichever is smaller).
				n := 3
				if len(s.Library) < n {
					n = len(s.Library)
				}
				if n > 1 && gs.Rng != nil {
					top := s.Library[:n]
					gs.Rng.Shuffle(len(top), func(i, j int) {
						top[i], top[j] = top[j], top[i]
					})
				}
			case "put_cards_from_hand_on_top":
				// Move the last card from hand to top of library.
				if len(s.Hand) > 0 {
					card := s.Hand[len(s.Hand)-1]
					s.Hand = s.Hand[:len(s.Hand)-1]
					s.Library = append([]*Card{card}, s.Library...)
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "library_manipulation",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	case "destroy_effect":
		ResolveEffect(gs, src, &gameast.Destroy{})
		gs.LogEvent(Event{
			Kind:   "destroy",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "goad_effect":
		targets := pickTargetFromModArgs(gs, src, e.Args)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				if t.Permanent.Flags == nil {
					t.Permanent.Flags = map[string]int{}
				}
				t.Permanent.Flags["goaded"] = 1
			}
		}
		gs.LogEvent(Event{
			Kind:   "goad",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "create_token_effect":
		ResolveEffect(gs, src, &gameast.CreateToken{Count: *gameast.NumInt(1), Types: []string{"creature"}})
		gs.LogEvent(Event{
			Kind:   "create_token",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "venture_into_dungeon":
		// CR §701.46 — "venture into the dungeon." Dungeon rooms aren't
		// fully modeled yet; track progress with a counter on the source.
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["dungeon_level"]++
		}
		gs.LogEvent(Event{
			Kind:   "venture",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"dungeon_level": func() int {
					if src != nil && src.Flags != nil {
						return src.Flags["dungeon_level"]
					}
					return 0
				}(),
			},
		})

	case "block_additional_creature":
		// Mark the source as able to block one additional creature (CR §702.XXb).
		if src != nil {
			if src.Flags == nil {
				src.Flags = map[string]int{}
			}
			src.Flags["blocks_additional"] = 1
		}
		gs.LogEvent(Event{
			Kind:   "block_additional",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
		})

	case "god_eternal_tuck":
		// CR §(God-Eternal cycle) — "put it third from the top of its
		// owner's library." Remove from battlefield and insert into library
		// at position 2 (0-indexed, so behind two cards).
		if src != nil && src.Card != nil {
			owner := src.Owner
			if owner < 0 || owner >= len(gs.Seats) {
				owner = src.Controller
			}
			card := src.Card
			gs.removePermanent(src)
			if owner >= 0 && owner < len(gs.Seats) {
				lib := gs.Seats[owner].Library
				insertAt := 2
				if insertAt > len(lib) {
					insertAt = len(lib)
				}
				// Insert card at position insertAt (third from top).
				newLib := make([]*Card, 0, len(lib)+1)
				newLib = append(newLib, lib[:insertAt]...)
				newLib = append(newLib, card)
				newLib = append(newLib, lib[insertAt:]...)
				gs.Seats[owner].Library = newLib
			}
		}
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
		// Simulation: each opponent is forced to take the first (worst-case)
		// option. Log which option was imposed on each opponent.
		seat := controllerSeat(src)
		chosenOption := "option_a"
		if seat >= 0 && seat < len(gs.Seats) {
			opps := gs.Opponents(seat)
			for _, opp := range opps {
				gs.LogEvent(Event{
					Kind:   "villainous_choice_resolved",
					Seat:   opp,
					Source: sourceName(src),
					Details: map[string]interface{}{
						"chosen": chosenOption,
						"args":   e.Args,
					},
				})
			}
		}
		gs.LogEvent(Event{
			Kind:   "villainous_choice",
			Seat:   seat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args":   e.Args,
				"chosen": chosenOption,
			},
		})

	case "delayed_trigger_next_upkeep":
		// Register a delayed trigger that fires at the next upkeep (CR §603.7).
		seat := controllerSeat(src)
		cardName := sourceName(src)
		gs.RegisterDelayedTrigger(&DelayedTrigger{
			TriggerAt:      "next_upkeep",
			ControllerSeat: seat,
			SourceCardName: cardName,
			OneShot:        true,
			EffectFn: func(gs *GameState) {
				gs.LogEvent(Event{
					Kind:   "delayed_trigger_fired",
					Seat:   seat,
					Source: cardName,
					Details: map[string]interface{}{
						"trigger_at": "next_upkeep",
					},
				})
			},
		})
		gs.LogEvent(Event{
			Kind:   "delayed_trigger",
			Seat:   seat,
			Source: cardName,
			Details: map[string]interface{}{
				"sub_kind": "next_upkeep",
			},
		})

	case "gift":
		// CR §702.XXX — Bloomburrow gift mechanic. An opponent receives a
		// benefit (e.g., draw a card, gain life) and in return the controller
		// gets a more powerful effect. The actual gift/reward pair varies per
		// card and is encoded in Args. Simulation: log with structured args;
		// the sub-effects are resolved by nested ModificationEffects if present.
		gs.LogEvent(Event{
			Kind:   "gift",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"args": e.Args,
			},
		})

	case "alt_cost_sacrifice", "alt_cost_bounce_land":
		// CR §118.9 — alternative costs. For sacrifice: delegate to the
		// Sacrifice effect handler. For bounce land: bounce a land the
		// controller controls back to hand (Ravnica bouncelands, etc.).
		if e.ModKind == "alt_cost_sacrifice" {
			ResolveEffect(gs, src, &gameast.Sacrifice{})
		} else if e.ModKind == "alt_cost_bounce_land" {
			seat := controllerSeat(src)
			if seat >= 0 && seat < len(gs.Seats) {
				bf := gs.Seats[seat].Battlefield
				for _, p := range bf {
					if p == nil || p.Card == nil {
						continue
					}
					for _, t := range p.Card.Types {
						if t == "land" {
							ResolveEffect(gs, p, &gameast.Bounce{To: "owners_hand"})
							goto altCostDone
						}
					}
				}
			altCostDone:
			}
		}
		gs.LogEvent(Event{
			Kind:   "alternative_cost",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"sub_kind": e.ModKind,
			},
		})

	// -----------------------------------------------------------------
	// choose_one_of_them — modal "choose one" among sibling effects.
	// The parser emits this as a structural marker (args are usually
	// empty). If any typed Effect args are present, pick the first
	// valid one (greedy strategy). Otherwise try text residual.
	// -----------------------------------------------------------------
	case "choose_one_of_them":
		cotResolved := false
		// Try to resolve the first typed Effect arg (greedy: pick first).
		for _, arg := range e.Args {
			if eff, ok := arg.(gameast.Effect); ok {
				ResolveEffect(gs, src, eff)
				cotResolved = true
				break // choose ONE
			}
		}
		// Fall back to text residual if available.
		if !cotResolved {
			cotRaw := modArgString(e.Args, 0)
			if cotRaw != "" {
				cotResolved = resolveResidualByText(gs, src, cotRaw)
			}
		}
		gs.LogEvent(Event{
			Kind:   "modal_choice",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"resolved": cotResolved,
				"args":     e.Args,
			},
		})

	case "manifest_n":
		// CR §701.34 variant — "manifest the top N cards of your library."
		// Extract count from args, then create N face-down 2/2 creatures.
		count := 1
		if len(e.Args) > 0 {
			if n, ok := asInt(e.Args[0]); ok && n > 0 {
				count = n
			}
		}
		seat := controllerSeat(src)
		manifested := 0
		if seat >= 0 && seat < len(gs.Seats) {
			s := gs.Seats[seat]
			for i := 0; i < count && len(s.Library) > 0; i++ {
				card := s.Library[0]
				s.Library = s.Library[1:]
				card.FaceDown = true
				perm := &Permanent{
					Card: &Card{
						Name:          "Face-Down Creature",
						Owner:         seat,
						BasePower:     2,
						BaseToughness: 2,
						Types:         []string{"creature"},
						FaceDown:      true,
					},
					Controller:    seat,
					Owner:         seat,
					Timestamp:     gs.NextTimestamp(),
					Counters:      map[string]int{},
					Flags:         map[string]int{"manifested": 1},
					SummoningSick: true,
				}
				perm.FrontFaceAST = card.AST
				perm.FrontFaceName = card.DisplayName()
				s.Battlefield = append(s.Battlefield, perm)
				RegisterReplacementsForPermanent(gs, perm)
				FirePermanentETBTriggers(gs, perm)
				manifested++
			}
		}
		gs.LogEvent(Event{
			Kind:   "manifest",
			Seat:   seat,
			Source: sourceName(src),
			Amount: manifested,
			Details: map[string]interface{}{
				"sub_kind":  "manifest_n",
				"requested": count,
				"rule":      "701.34",
			},
		})

	case "cast_creatures_from_library_top":
		// CR §601 — Future Sight / Courser of Kruphix / Vizier of the
		// Menagerie effects: "You may cast creature spells from the top of
		// your library." Grant a library-cast permission on the top card
		// of the controller's library if it's a creature.
		cftSeat := controllerSeat(src)
		cftGranted := false
		if cftSeat >= 0 && cftSeat < len(gs.Seats) {
			lib := gs.Seats[cftSeat].Library
			if len(lib) > 0 {
				topCard := lib[0]
				// Check if it's a creature card.
				isCreature := false
				if topCard != nil {
					for _, tp := range topCard.Types {
						if strings.EqualFold(tp, "creature") {
							isCreature = true
							break
						}
					}
				}
				if isCreature {
					perm := NewLibraryCastPermission(0) // use normal mana cost
					perm.RequireController = cftSeat
					perm.SourceName = sourceName(src)
					RegisterZoneCastGrant(gs, topCard, perm)
					cftGranted = true
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "cast_from_library_top",
			Seat:   cftSeat,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"permission_granted": cftGranted,
			},
		})

	case "put_second_from_top", "place_revealed_on_library":
		// Put a card at a specific position in the library. "put_second_from_top"
		// inserts at position 1 (behind the top card); "place_revealed_on_library"
		// puts on top. The pronoun typically refers to a recently revealed/exiled card.
		psSeat := controllerSeat(src)
		if psSeat >= 0 && psSeat < len(gs.Seats) {
			s := gs.Seats[psSeat]
			// Find a card to place — check exile first (revealed cards are often exiled).
			var card *Card
			fromZone := ""
			if len(s.Exile) > 0 {
				card = s.Exile[len(s.Exile)-1]
				fromZone = "exile"
			} else if len(s.Hand) > 0 {
				card = s.Hand[len(s.Hand)-1]
				fromZone = "hand"
			}
			if card != nil {
				removeCardFromZone(gs, psSeat, card, fromZone)
				if e.ModKind == "put_second_from_top" && len(s.Library) > 0 {
					// Insert at position 1 (second from top).
					newLib := make([]*Card, 0, len(s.Library)+1)
					newLib = append(newLib, s.Library[0])
					newLib = append(newLib, card)
					newLib = append(newLib, s.Library[1:]...)
					s.Library = newLib
				} else {
					// Place on top.
					s.Library = append([]*Card{card}, s.Library...)
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "library_placement",
			Seat:   psSeat,
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
	// reKeywordActionCount extracts a trailing integer from keyword actions
	// like "scry 2", "surveil 3", "mill 5", "amass 2".
	reKeywordActionCount = regexp.MustCompile(`(?i)\b(\d+)\s*$`)

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
		// Pick an opponent's creature and steal it.
		targets := pickCreatureTargets(gs, src, true)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil && t.Permanent.Controller != seat {
				p := t.Permanent
				oldController := p.Controller
				gs.removePermanent(p)
				p.Controller = seat
				p.Timestamp = gs.NextTimestamp()
				if seat >= 0 && seat < len(gs.Seats) {
					gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
				}
				gs.LogEvent(Event{Kind: "gain_control", Seat: seat, Target: oldController, Source: sourceName(src),
					Details: map[string]interface{}{"target_card": p.Card.DisplayName()}})
				return true
			}
		}
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
		targets := pickCreatureTargets(gs, src, true)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				if t.Permanent.Flags == nil {
					t.Permanent.Flags = map[string]int{}
				}
				t.Permanent.Flags["cant_block"] = 1
			}
		}
		gs.LogEvent(Event{Kind: "cant_block", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResMustAttack.MatchString(raw) {
		targets := pickCreatureTargets(gs, src, true)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				if t.Permanent.Flags == nil {
					t.Permanent.Flags = map[string]int{}
				}
				t.Permanent.Flags["must_attack"] = 1
			}
		}
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
		targets := pickCreatureTargets(gs, src, true)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				if t.Permanent.Flags == nil {
					t.Permanent.Flags = map[string]int{}
				}
				t.Permanent.Flags["detained"] = 1
			}
		}
		gs.LogEvent(Event{Kind: "detain", Seat: seat, Source: sourceName(src)})
		return true
	}

	if reResPhaseOut.MatchString(raw) {
		targets := pickCreatureTargets(gs, src, true)
		for _, t := range targets {
			if t.Kind == TargetKindPermanent && t.Permanent != nil {
				if t.Permanent.Flags == nil {
					t.Permanent.Flags = map[string]int{}
				}
				t.Permanent.Flags["phased_out"] = 1
			}
		}
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
		for _, opp := range gs.Opponents(seat) {
			if opp >= 0 && opp < len(gs.Seats) && gs.Seats[opp] != nil {
				gs.Seats[opp].Life = gs.Seats[opp].Life / 2
			}
		}
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
