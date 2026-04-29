package gameengine

// Cast-count increment hooks + cast-trigger observer dispatcher.
//
// Comp-rules citations:
//   - §700.4      — cast-count bookkeeping (implicit; "this turn" scoping)
//   - §702.40     — Storm keyword (see storm.go)
//   - §706.10     — copies of spells are not cast
//   - §601        — casting a spell (the event cast-triggers observe)
//   - §603.2/.3   — triggered abilities on-stack placement
//
// The IncrementCastCount function is called by CastSpell (stack.go) and by
// the commander-zone cast path (commander.go, owned by the Partner agent
// so we don't modify it here; commander.go callers should IncrementCastCount
// before pushing their stack item). Copies created by Storm / Twinflame /
// Dualcaster Mage MUST NOT call this.
//
// FireCastTriggerObservers is the bridge that runs the "whenever you cast…"
// style triggers for cards that don't yet have per-card handlers wired into
// the proper §603 Triggered-on-stack path. For the Tier 1 storm infra, we
// implement six observer cards inline here: Storm-Kiln Artist, Young
// Pyromancer, Third Path Iconoclast, Monastery Mentor, Runaway Steam-Kin,
// Birgi (God of Storytelling), Niv-Mizzet Parun. Each matches Python's
// _fire_cast_trigger_observers in scripts/playloop.py exactly so Go/Python
// parity holds.
//
// Long-term: these handlers should migrate to internal/gameengine/per_card/
// (owned by the per-card agent) and be dispatched via the normal
// RegisterCastTriggerObserver pipeline when that lands.

import (
	"strings"
)

// IncrementCastCount bumps the global + per-seat cast counters. Called by
// CastSpell AFTER cost has been paid and the card is en route to the stack,
// BEFORE the storm trigger is evaluated. Must NOT be called for copies
// (CR §706.10).
func IncrementCastCount(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	gs.SpellsCastThisTurn++
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	seat.SpellsCastThisTurn++
}

// FireCastTriggerObservers fires every "whenever a spell is cast" style
// observer permanent for the cast of `cast`. `fromCopy` MUST be true when
// called from Storm copy propagation — copies are not cast (§706.10) and
// do not trigger observers.
//
// Mirror for scripts/playloop.py _fire_cast_trigger_observers.
func FireCastTriggerObservers(gs *GameState, cast *Card, controller int, fromCopy bool) {
	if gs == nil || cast == nil || fromCopy {
		return
	}
	// Derive the filters every observer reads off the cast spell. Both
	// Types and TypeLine on Card are authoritative (Types is the canonical
	// slice; TypeLine is a cache for tokens/copies that want a human-
	// readable string). We accept either so callers don't have to
	// double-populate.
	lowerTypes := map[string]bool{}
	for _, t := range cast.Types {
		lowerTypes[strings.ToLower(t)] = true
	}
	typeLine := strings.ToLower(cast.TypeLine)
	if strings.Contains(typeLine, "instant") {
		lowerTypes["instant"] = true
	}
	if strings.Contains(typeLine, "sorcery") {
		lowerTypes["sorcery"] = true
	}
	if strings.Contains(typeLine, "creature") {
		lowerTypes["creature"] = true
	}
	isInstant := lowerTypes["instant"]
	isSorcery := lowerTypes["sorcery"]
	isCreature := lowerTypes["creature"]
	isInstantOrSorcery := isInstant || isSorcery
	isNoncreature := !isCreature
	// Color — check Card.Colors (populated by corpus loader / token
	// creation). For Runaway Steam-Kin's "whenever you cast a red spell".
	isRed := false
	for _, col := range cast.Colors {
		if strings.ToUpper(col) == "R" {
			isRed = true
			break
		}
	}

	// Walk every battlefield permanent. Snapshot battlefield first so
	// observer-created tokens don't get iterated as observers themselves.
	type permNamed struct {
		perm *Permanent
		name string
	}
	var observers []permNamed
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm == nil || perm.Card == nil {
				continue
			}
			name := perm.Card.DisplayName()
			observers = append(observers, permNamed{perm: perm, name: name})
		}
	}
	castName := cast.DisplayName()

	for _, o := range observers {
		perm := o.perm
		name := o.name

		// --- Prowess keyword (CR §702.108) ---
		// "Whenever you cast a noncreature spell, this creature gets
		// +1/+1 until end of turn."
		// Check BEFORE the name switch so prowess fires on any permanent
		// that has the keyword, regardless of card name.
		if perm.Controller == controller && isNoncreature && perm.IsCreature() && perm.HasKeyword("prowess") {
			perm.Modifications = append(perm.Modifications, Modification{
				Power:     1,
				Toughness: 1,
				Duration:  "until_end_of_turn",
				Timestamp: gs.NextTimestamp(),
			})
			gs.InvalidateCharacteristicsCache()
			gs.LogEvent(Event{
				Kind:   "prowess",
				Seat:   perm.Controller,
				Source: name,
				Details: map[string]interface{}{
					"cast":   castName,
					"effect": "+1/+1 until end of turn",
					"rule":   "702.108",
				},
			})
		}

		// --- Extort keyword (CR §702.101) ---
		// "Whenever you cast a spell, you may pay {W/B}. If you do,
		// each opponent loses 1 life and you gain that much life."
		// Each instance of extort on each permanent triggers separately.
		// GreedyHat: pay if affordable (1 generic mana as MVP proxy
		// for {W/B}).
		if perm.Controller == controller && perm.HasKeyword("extort") {
			// Check if the caster can pay 1 mana for the extort trigger.
			casterSeat := gs.Seats[controller]
			if casterSeat != nil && casterSeat.ManaPool >= 1 {
				casterSeat.ManaPool -= 1
				SyncManaAfterSpend(casterSeat)
				// Each opponent loses 1 life, controller gains that much.
				opps := gs.Opponents(controller)
				totalDrained := 0
				for _, oppIdx := range opps {
					opp := gs.Seats[oppIdx]
					if opp == nil {
						continue
					}
					opp.Life -= 1
					totalDrained++
					gs.LogEvent(Event{
						Kind:   "life_change",
						Seat:   oppIdx,
						Amount: -1,
						Source: name,
						Details: map[string]interface{}{
							"reason": "extort",
						},
					})
				}
				if totalDrained > 0 {
					GainLife(gs, controller, totalDrained, name)
					gs.LogEvent(Event{
						Kind:   "life_change",
						Seat:   controller,
						Amount: totalDrained,
						Source: name,
						Details: map[string]interface{}{
							"reason": "extort",
						},
					})
				}
				gs.LogEvent(Event{
					Kind:   "extort_trigger",
					Seat:   controller,
					Source: name,
					Amount: totalDrained,
					Details: map[string]interface{}{
						"cast":     castName,
						"drained":  totalDrained,
						"mana_paid": 1,
						"rule":     "702.101",
					},
				})
			}
		}

		switch name {
		case "Storm-Kiln Artist":
			if perm.Controller == controller && isInstantOrSorcery {
				createTreasureToken(gs, perm.Controller)
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "treasure_token",
					},
				})
			}
		case "Young Pyromancer":
			if perm.Controller == controller && isInstantOrSorcery {
				createSimpleCreatureToken(gs, perm.Controller,
					"Elemental Token", 1, 1, []string{"R"})
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "elemental_token",
					},
				})
			}
		case "Third Path Iconoclast":
			if perm.Controller == controller && isNoncreature {
				createSimpleCreatureToken(gs, perm.Controller,
					"Soldier Artifact Token", 1, 1, nil)
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "soldier_token",
					},
				})
			}
		case "Monastery Mentor":
			if perm.Controller == controller && isNoncreature {
				createSimpleCreatureToken(gs, perm.Controller,
					"Monk Token", 1, 1, []string{"W"})
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "monk_token",
					},
				})
			}
		case "Runaway Steam-Kin":
			if perm.Controller == controller && isRed {
				cur := 0
				if perm.Counters != nil {
					cur = perm.Counters["+1/+1"]
				}
				if cur < 3 {
					perm.AddCounter("+1/+1", 1)
					gs.LogEvent(Event{
						Kind:   "cast_trigger_observer",
						Seat:   perm.Controller,
						Source: name,
						Details: map[string]interface{}{
							"cast":   castName,
							"effect": "plus_one_counter",
						},
					})
				}
			}
		case "Birgi, God of Storytelling":
			if perm.Controller == controller {
				gs.Seats[perm.Controller].ManaPool++
				SyncManaAfterAdd(gs.Seats[perm.Controller], 1)
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Amount: 1,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "add_mana_R",
					},
				})
			}
		case "Niv-Mizzet, Parun":
			if perm.Controller == controller && isInstantOrSorcery {
				// Draw a card. Leave the draw-trigger side (damage on draw)
				// to the existing draw-trigger infrastructure; we only
				// fire the cast-trigger half here.
				gs.drawOne(perm.Controller)
				gs.LogEvent(Event{
					Kind:   "cast_trigger_observer",
					Seat:   perm.Controller,
					Source: name,
					Details: map[string]interface{}{
						"cast":   castName,
						"effect": "draw_card",
					},
				})
			}

		// ------------------------------------------------------------------
		// Rhystic Study (Wave 1b)
		// "Whenever an opponent casts a spell, that player may pay {1}.
		//  If that player doesn't, you draw a card."
		// Policy: greedy pay when affordable — opponent pays 1 generic if
		// they can, otherwise controller draws. Mirrors Python.
		// ------------------------------------------------------------------
		case "Rhystic Study":
			if perm.Controller != controller {
				opp := gs.Seats[controller]
				if opp != nil && opp.ManaPool >= 1 {
					opp.ManaPool -= 1
					SyncManaAfterSpend(opp)
					gs.LogEvent(Event{
						Kind:   "pay_mana",
						Seat:   controller,
						Source: name,
						Amount: 1,
						Details: map[string]interface{}{
							"reason":    "rhystic",
							"card_name": castName,
						},
					})
					gs.LogEvent(Event{
						Kind:   "cast_trigger_observer",
						Seat:   perm.Controller,
						Source: name,
						Details: map[string]interface{}{
							"cast":        castName,
							"effect":      "rhystic_tax_paid",
							"caster_seat": controller,
						},
					})
				} else {
					gs.drawOne(perm.Controller)
					gs.LogEvent(Event{
						Kind:   "draw",
						Seat:   perm.Controller,
						Source: name,
						Amount: 1,
						Details: map[string]interface{}{
							"reason": "rhystic_study",
						},
					})
					gs.LogEvent(Event{
						Kind:   "cast_trigger_observer",
						Seat:   perm.Controller,
						Source: name,
						Details: map[string]interface{}{
							"cast":        castName,
							"effect":      "rhystic_draw",
							"caster_seat": controller,
						},
					})
					FireDrawTriggerObservers(gs, perm.Controller, 1, false)
				}
			}

		// ------------------------------------------------------------------
		// Mystic Remora (Wave 1b)
		// "Whenever an opponent casts a noncreature spell, that player
		//  may pay {4}. If the player doesn't, you draw a card."
		// ------------------------------------------------------------------
		case "Mystic Remora":
			if perm.Controller != controller && isNoncreature {
				opp := gs.Seats[controller]
				if opp != nil && opp.ManaPool >= 4 {
					opp.ManaPool -= 4
					SyncManaAfterSpend(opp)
					gs.LogEvent(Event{
						Kind:   "pay_mana",
						Seat:   controller,
						Source: name,
						Amount: 4,
						Details: map[string]interface{}{
							"reason":    "remora",
							"card_name": castName,
						},
					})
					gs.LogEvent(Event{
						Kind:   "cast_trigger_observer",
						Seat:   perm.Controller,
						Source: name,
						Details: map[string]interface{}{
							"cast":        castName,
							"effect":      "remora_tax_paid",
							"caster_seat": controller,
						},
					})
				} else {
					gs.drawOne(perm.Controller)
					gs.LogEvent(Event{
						Kind:   "draw",
						Seat:   perm.Controller,
						Source: name,
						Amount: 1,
						Details: map[string]interface{}{
							"reason": "mystic_remora",
						},
					})
					gs.LogEvent(Event{
						Kind:   "cast_trigger_observer",
						Seat:   perm.Controller,
						Source: name,
						Details: map[string]interface{}{
							"cast":        castName,
							"effect":      "remora_draw",
							"caster_seat": controller,
						},
					})
					FireDrawTriggerObservers(gs, perm.Controller, 1, false)
				}
			}

		// ------------------------------------------------------------------
		// Esper Sentinel (Wave 1b)
		// "Whenever an opponent casts their first noncreature spell each
		//  turn, unless that player pays {X} where X is Esper Sentinel
		//  controller's creature count, you draw a card."
		// Per-turn per-opponent first-noncreature tracking via perm.Flags.
		// ------------------------------------------------------------------
		case "Esper Sentinel":
			if perm.Controller != controller && isNoncreature {
				// Track "first noncreature this turn" per opponent.
				flagKey := "esper_sentinel_fired_turn"
				firedTurn := -1
				if perm.Flags != nil {
					firedTurn = perm.Flags[flagKey]
				}
				if firedTurn != gs.Turn {
					if perm.Flags == nil {
						perm.Flags = map[string]int{}
					}
					perm.Flags[flagKey] = gs.Turn
					// X = number of creatures controller has.
					xCost := 0
					if perm.Controller >= 0 && perm.Controller < len(gs.Seats) {
						for _, cp := range gs.Seats[perm.Controller].Battlefield {
							if cp != nil && cp.IsCreature() {
								xCost++
							}
						}
					}
					opp := gs.Seats[controller]
					if opp != nil && opp.ManaPool >= xCost && xCost > 0 {
						opp.ManaPool -= xCost
						SyncManaAfterSpend(opp)
						gs.LogEvent(Event{
							Kind:   "pay_mana",
							Seat:   controller,
							Source: name,
							Amount: xCost,
							Details: map[string]interface{}{
								"reason":    "sentinel",
								"card_name": castName,
							},
						})
						gs.LogEvent(Event{
							Kind:   "cast_trigger_observer",
							Seat:   perm.Controller,
							Source: name,
							Details: map[string]interface{}{
								"cast":        castName,
								"effect":      "sentinel_tax_paid",
								"caster_seat": controller,
								"x":           xCost,
							},
						})
					} else {
						gs.drawOne(perm.Controller)
						gs.LogEvent(Event{
							Kind:   "draw",
							Seat:   perm.Controller,
							Source: name,
							Amount: 1,
							Details: map[string]interface{}{
								"reason": "esper_sentinel",
							},
						})
						gs.LogEvent(Event{
							Kind:   "cast_trigger_observer",
							Seat:   perm.Controller,
							Source: name,
							Details: map[string]interface{}{
								"cast":        castName,
								"effect":      "sentinel_draw",
								"caster_seat": controller,
								"x":           xCost,
							},
						})
						FireDrawTriggerObservers(gs, perm.Controller, 1, false)
					}
				}
			}
		}
	}
}

// createTreasureToken drops a Treasure artifact token onto the battlefield
// under `seatIdx`'s control. MVP: the token is just a marker; a full
// Treasure mana-ability wiring is a separate task (the existing mana pool
// MVP doesn't distinguish between tapped and untapped mana sources, and
// Treasure's {T}, sacrifice: add one mana of any color plays through the
// normal tap-mana scanner once that lands).
func createTreasureToken(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	token := &Card{
		Name:  "Treasure Token",
		Owner: seatIdx,
		Types: []string{"token", "artifact", "treasure"},
	}
	perm := &Permanent{
		Card:          token,
		Controller:    seatIdx,
		Owner:         seatIdx,
		Tapped:        false,
		SummoningSick: false,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	gs.LogEvent(Event{
		Kind:   "create_token",
		Seat:   seatIdx,
		Source: "Treasure Token",
		Details: map[string]interface{}{
			"token": "Treasure Token",
		},
	})
}

// FireDrawTriggerObservers fires "whenever a player draws a card" style
// reactive triggers for the `drawerSeat`. Mirrors scripts/playloop.py's
// _fire_draw_trigger_observers. Covers cards that have a dedicated
// per-card handler (Orcish Bowmasters via per_card.registerOrcishBowmasters)
// and falls back to inline name-based dispatch for Smothering Tithe
// until a proper per-card handler lands.
//
// CR §614.6 note: "the first draw in each of their draw steps" clause
// (Orcish Bowmasters, Notion Thief) is governed by the skipFirstDrawStep
// flag — callers set gs.Flags["_suppress_first_draw_trigger_seat"] =
// drawerSeat+1 (0 = unset) when emitting a turn-draw-step draw. The
// marker is consumed + cleared here.
func FireDrawTriggerObservers(gs *GameState, drawerSeat int, count int, fromReplacement bool) {
	if gs == nil || drawerSeat < 0 || drawerSeat >= len(gs.Seats) {
		return
	}
	if count <= 0 {
		count = 1
	}
	// Consume the turn-draw-step marker so only the single first draw
	// from a draw step suppresses Bowmasters / Notion Thief.
	skipBowmasters := false
	if gs.Flags != nil {
		if v, ok := gs.Flags["_suppress_first_draw_trigger_seat"]; ok && v == drawerSeat+1 {
			skipBowmasters = true
			delete(gs.Flags, "_suppress_first_draw_trigger_seat")
		}
	}

	// Fire the "player_would_draw" event for per-card handlers like
	// Chains of Mephistopheles that replace or modify draws.
	FireCardTrigger(gs, "player_would_draw", map[string]interface{}{
		"draw_seat":         drawerSeat,
		"count":             count,
		"from_replacement":  fromReplacement,
		"is_draw_step_draw": skipBowmasters,
	})

	// Per-draw cycle — apply count times so a two-card draw fires
	// Bowmasters twice (CR §614.6).
	for i := 0; i < count; i++ {
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, perm := range seat.Battlefield {
				if perm == nil || perm.Card == nil {
					continue
				}
				name := perm.Card.DisplayName()
				switch name {
				case "Smothering Tithe":
					if perm.Controller == drawerSeat {
						continue // opponent-only trigger
					}
					opp := gs.Seats[drawerSeat]
					if opp == nil {
						continue
					}
					if opp.ManaPool >= 2 {
						opp.ManaPool -= 2
						SyncManaAfterSpend(opp)
						gs.LogEvent(Event{
							Kind:   "pay_mana",
							Seat:   drawerSeat,
							Source: name,
							Amount: 2,
							Details: map[string]interface{}{
								"reason": "tithe",
							},
						})
						gs.LogEvent(Event{
							Kind:   "draw_trigger_observer",
							Seat:   perm.Controller,
							Source: name,
							Details: map[string]interface{}{
								"drawer_seat": drawerSeat,
								"effect":      "tithe_tax_paid",
							},
						})
					} else {
						createTreasureToken(gs, perm.Controller)
						gs.LogEvent(Event{
							Kind:   "draw_trigger_observer",
							Seat:   perm.Controller,
							Source: name,
							Details: map[string]interface{}{
								"drawer_seat": drawerSeat,
								"effect":      "tithe_treasure",
							},
						})
					}
				case "Orcish Bowmasters":
					if perm.Controller == drawerSeat {
						continue // opponent-only
					}
					if skipBowmasters {
						continue // first draw of the draw step
					}
					// Make 1/1 Zombie Archer token + 1 damage to drawer.
					createSimpleCreatureToken(gs, perm.Controller,
						"Zombie Archer Token", 1, 1, []string{"B"})
					tgt := gs.Seats[drawerSeat]
					if tgt != nil {
						tgt.Life -= 1
						gs.LogEvent(Event{
							Kind:   "damage",
							Seat:   perm.Controller,
							Target: drawerSeat,
							Source: name,
							Amount: 1,
						})
						gs.LogEvent(Event{
							Kind:   "life_change",
							Seat:   drawerSeat,
							Amount: -1,
							Source: name,
						})
					}
					gs.LogEvent(Event{
						Kind:   "draw_trigger_observer",
						Seat:   perm.Controller,
						Source: name,
						Details: map[string]interface{}{
							"drawer_seat": drawerSeat,
							"effect":      "bowmasters_ping",
						},
					})
				}
			}
		}
	}
}

// createSimpleCreatureToken drops a vanilla creature token onto the
// battlefield. Used by Young Pyromancer / Monastery Mentor / Third Path
// Iconoclast cast-trigger observers.
func createSimpleCreatureToken(gs *GameState, seatIdx int, name string,
	power, toughness int, colors []string) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	types := []string{"token", "creature"}
	token := &Card{
		Name:          name,
		Owner:         seatIdx,
		BasePower:     power,
		BaseToughness: toughness,
		Types:         types,
	}
	perm := &Permanent{
		Card:          token,
		Controller:    seatIdx,
		Owner:         seatIdx,
		Tapped:        false,
		SummoningSick: true, // §302.1 — creatures enter with summoning sickness
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	gs.LogEvent(Event{
		Kind:   "create_token",
		Seat:   seatIdx,
		Source: name,
		Amount: power,
		Details: map[string]interface{}{
			"token":     name,
			"power":     power,
			"toughness": toughness,
			"colors":    colors,
		},
	})
}
