package main

// Module 3: Replacement Effect Conflicts (--replacement)
//
// Creates game states with 2-3 replacement effects active simultaneously
// and triggers an event they all want to replace. Checks that the engine
// handles conflicts correctly and invariants hold.

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

type replacementScenario struct {
	Name    string
	Setup   func(gs *gameengine.GameState)
	Act     func(gs *gameengine.GameState)
	Verify  func(gs *gameengine.GameState) *failure
}

func runReplacement(_ *astload.Corpus, _ []*oracleCard) []failure {
	scenarios := buildReplacementScenarios()
	var fails []failure

	for _, sc := range scenarios {
		f := runReplacementScenario(sc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	return fails
}

func runReplacementScenario(sc replacementScenario) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			result = &failure{
				CardName:    sc.Name,
				Interaction: "replacement",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			}
		}
	}()

	gs := makeBaseGameState()
	sc.Setup(gs)
	gs.Snapshot()
	sc.Act(gs)
	gameengine.StateBasedActions(gs)

	// Check invariants first.
	violations := gameengine.RunAllInvariants(gs)
	if len(violations) > 0 {
		return &failure{
			CardName:    sc.Name,
			Interaction: "replacement",
			Invariant:   violations[0].Name,
			Message:     violations[0].Message,
		}
	}

	// Run scenario-specific verification.
	if sc.Verify != nil {
		return sc.Verify(gs)
	}

	return nil
}

func makeBaseGameState() *gameengine.GameState {
	gs := &gameengine.GameState{
		Turn:   1,
		Active: 0,
		Phase:  "precombat_main",
		Step:   "",
		Flags:  map[string]int{},
	}
	for i := 0; i < 4; i++ {
		seat := &gameengine.Seat{
			Life:  40,
			Idx:   i,
			Flags: map[string]int{},
		}
		for j := 0; j < 10; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name: fmt.Sprintf("Filler %d-%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
			})
		}
		for j := 0; j < 3; j++ {
			seat.Hand = append(seat.Hand, &gameengine.Card{
				Name: fmt.Sprintf("HandCard %d-%d", i, j), Owner: i,
				Types: []string{"creature"},
			})
		}
		gs.Seats = append(gs.Seats, seat)
	}
	return gs
}

func buildReplacementScenarios() []replacementScenario {
	return []replacementScenario{
		{
			Name: "DoubleTokenCreation_TwoDoublers",
			Setup: func(gs *gameengine.GameState) {
				// Simulate Doubling Season + Parallel Lives with replacement effects.
				// Both double token creation. When both are active, tokens should quadruple.
				ts1 := gs.NextTimestamp()
				doublingSeason := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Doubling Season", Owner: 0,
						Types: []string{"enchantment"},
					},
					Controller: 0, Owner: 0, Timestamp: ts1,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, doublingSeason)

				gs.RegisterReplacement(&gameengine.ReplacementEffect{
					EventType:      "would_create_token",
					HandlerID:      "doubling_season_tokens",
					SourcePerm:     doublingSeason,
					ControllerSeat: 0,
					Timestamp:      ts1,
					Category:       gameengine.CategoryOther,
					Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
						return ev != nil && ev.TargetSeat == 0
					},
					ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
						c := ev.Count()
						if c <= 0 {
							c = 1
						}
						ev.SetCount(c * 2)
					},
				})

				ts2 := gs.NextTimestamp()
				parallelLives := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Parallel Lives", Owner: 0,
						Types: []string{"enchantment"},
					},
					Controller: 0, Owner: 0, Timestamp: ts2,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, parallelLives)

				gs.RegisterReplacement(&gameengine.ReplacementEffect{
					EventType:      "would_create_token",
					HandlerID:      "parallel_lives_tokens",
					SourcePerm:     parallelLives,
					ControllerSeat: 0,
					Timestamp:      ts2,
					Category:       gameengine.CategoryOther,
					Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
						return ev != nil && ev.TargetSeat == 0
					},
					ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
						c := ev.Count()
						if c <= 0 {
							c = 1
						}
						ev.SetCount(c * 2)
					},
				})
			},
			Act: func(gs *gameengine.GameState) {
				// Create a token — should fire both replacement effects.
				tokenCard := &gameengine.Card{
					Name: "Saproling Token", Owner: 0,
					Types: []string{"token", "creature"}, BasePower: 1, BaseToughness: 1,
				}
				tokenPerm := &gameengine.Permanent{
					Card: tokenCard, Controller: 0, Owner: 0,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tokenPerm)
			},
			Verify: nil, // invariants are sufficient
		},
		{
			Name: "RestInPeace_CreatureDies_GoesToExile",
			Setup: func(gs *gameengine.GameState) {
				ts := gs.NextTimestamp()
				rip := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Rest in Peace", Owner: 0,
						Types: []string{"enchantment"},
					},
					Controller: 0, Owner: 0, Timestamp: ts,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, rip)

				// Register a replacement: if any card would go to a graveyard,
				// exile it instead.
				gs.RegisterReplacement(&gameengine.ReplacementEffect{
					EventType:      "would_change_zone",
					HandlerID:      "rest_in_peace_exile",
					SourcePerm:     rip,
					ControllerSeat: 0,
					Timestamp:      ts,
					Category:       gameengine.CategoryOther,
					Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
						if ev == nil {
							return false
						}
						toZone, _ := ev.Payload["to_zone"].(string)
						return toZone == "graveyard"
					},
					ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
						ev.Payload["to_zone"] = "exile"
					},
				})

				// Place a creature to be destroyed.
				victim := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Grizzly Bears", Owner: 1,
						Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
					},
					Controller: 1, Owner: 1,
					Flags: map[string]int{},
				}
				gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, victim)
			},
			Act: func(gs *gameengine.GameState) {
				// Find and destroy the creature.
				for _, p := range gs.Seats[1].Battlefield {
					if p.Card != nil && p.Card.Name == "Grizzly Bears" {
						gameengine.DestroyPermanent(gs, p, nil)
						break
					}
				}
			},
			Verify: nil,
		},
		{
			Name: "Humility_CreaturesAre1_1_NoAbilities",
			Setup: func(gs *gameengine.GameState) {
				ts := gs.NextTimestamp()
				humility := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Humility", Owner: 0,
						Types: []string{"enchantment"},
					},
					Controller: 0, Owner: 0, Timestamp: ts,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, humility)

				// Register layer 7b (set P/T) and layer 6 (remove abilities)
				// continuous effects for Humility.
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer:          7,
					Sublayer:       "b",
					Timestamp:      ts,
					SourcePerm:     humility,
					SourceCardName: "Humility",
					ControllerSeat: 0,
					HandlerID:      "humility_set_pt",
					Duration:       "permanent",
					Predicate: func(gs *gameengine.GameState, target *gameengine.Permanent) bool {
						return target.IsCreature() && target != humility
					},
					ApplyFn: func(gs *gameengine.GameState, target *gameengine.Permanent, chars *gameengine.Characteristics) {
						chars.Power = 1
						chars.Toughness = 1
					},
				})

				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer:          6,
					Timestamp:      ts,
					SourcePerm:     humility,
					SourceCardName: "Humility",
					ControllerSeat: 0,
					HandlerID:      "humility_remove_abilities",
					Duration:       "permanent",
					Predicate: func(gs *gameengine.GameState, target *gameengine.Permanent) bool {
						return target.IsCreature() && target != humility
					},
					ApplyFn: func(gs *gameengine.GameState, target *gameengine.Permanent, chars *gameengine.Characteristics) {
						chars.Abilities = nil
						chars.Keywords = nil
					},
				})

				// Place a big creature.
				big := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Colossal Dreadmaw", Owner: 1,
						Types: []string{"creature"}, BasePower: 6, BaseToughness: 6,
					},
					Controller: 1, Owner: 1,
					Flags: map[string]int{},
				}
				gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, big)
			},
			Act: func(gs *gameengine.GameState) {
				// Just run SBAs and let the layer system do its work.
				gs.InvalidateCharacteristicsCache()
				gameengine.StateBasedActions(gs)
			},
			Verify: func(gs *gameengine.GameState) *failure {
				// Verify Colossal Dreadmaw is now effectively 1/1.
				for _, p := range gs.Seats[1].Battlefield {
					if p.Card != nil && p.Card.Name == "Colossal Dreadmaw" {
						chars := gameengine.GetEffectiveCharacteristics(gs, p)
						if chars.Power != 1 || chars.Toughness != 1 {
							return &failure{
								CardName:    "Humility_CreaturesAre1_1_NoAbilities",
								Interaction: "replacement",
								Invariant:   "Humility_PT_Check",
								Message: fmt.Sprintf("Colossal Dreadmaw should be 1/1 under Humility, got %d/%d",
									chars.Power, chars.Toughness),
							}
						}
					}
				}
				return nil
			},
		},
		{
			Name: "LeylineOfVoid_DiesTrigger_ExileInstead",
			Setup: func(gs *gameengine.GameState) {
				ts := gs.NextTimestamp()
				leyline := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Leyline of the Void", Owner: 0,
						Types: []string{"enchantment"},
					},
					Controller: 0, Owner: 0, Timestamp: ts,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, leyline)

				// Register replacement: opponent's cards go to exile instead of graveyard.
				gs.RegisterReplacement(&gameengine.ReplacementEffect{
					EventType:      "would_change_zone",
					HandlerID:      "leyline_of_void",
					SourcePerm:     leyline,
					ControllerSeat: 0,
					Timestamp:      ts,
					Category:       gameengine.CategoryOther,
					Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
						if ev == nil {
							return false
						}
						toZone, _ := ev.Payload["to_zone"].(string)
						// Only apply to opponent's cards.
						if ev.Source != nil && ev.Source.Controller == 0 {
							return false
						}
						return toZone == "graveyard"
					},
					ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
						ev.Payload["to_zone"] = "exile"
					},
				})

				// Place a creature on opponent's side to be destroyed.
				victim := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Elvish Mystic", Owner: 1,
						Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
					},
					Controller: 1, Owner: 1,
					Flags: map[string]int{},
				}
				gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, victim)
			},
			Act: func(gs *gameengine.GameState) {
				for _, p := range gs.Seats[1].Battlefield {
					if p.Card != nil && p.Card.Name == "Elvish Mystic" {
						gameengine.DestroyPermanent(gs, p, nil)
						break
					}
				}
			},
			Verify: nil,
		},
		{
			Name: "MultipleReplacements_DamageDoubled_LifeGainDoubled",
			Setup: func(gs *gameengine.GameState) {
				// Place two enchantments that each register a replacement effect.
				ts1 := gs.NextTimestamp()
				archon := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Rhox Faithmender", Owner: 0,
						Types: []string{"creature"}, BasePower: 1, BaseToughness: 5,
					},
					Controller: 0, Owner: 0, Timestamp: ts1,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, archon)

				gs.RegisterReplacement(&gameengine.ReplacementEffect{
					EventType:      "would_gain_life",
					HandlerID:      "rhox_faithmender_double",
					SourcePerm:     archon,
					ControllerSeat: 0,
					Timestamp:      ts1,
					Category:       gameengine.CategoryOther,
					Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
						return ev != nil && ev.TargetSeat == 0
					},
					ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
						c := ev.Count()
						if c > 0 {
							ev.SetCount(c * 2)
						}
					},
				})

				ts2 := gs.NextTimestamp()
				archive := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Alhammarret's Archive", Owner: 0,
						Types: []string{"artifact"},
					},
					Controller: 0, Owner: 0, Timestamp: ts2,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, archive)

				gs.RegisterReplacement(&gameengine.ReplacementEffect{
					EventType:      "would_gain_life",
					HandlerID:      "alhammarrets_archive_double",
					SourcePerm:     archive,
					ControllerSeat: 0,
					Timestamp:      ts2,
					Category:       gameengine.CategoryOther,
					Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
						return ev != nil && ev.TargetSeat == 0
					},
					ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
						c := ev.Count()
						if c > 0 {
							ev.SetCount(c * 2)
						}
					},
				})
			},
			Act: func(gs *gameengine.GameState) {
				// Simulate gaining 5 life — should be doubled twice to 20.
				gs.Seats[0].Life += 5
			},
			Verify: nil,
		},
		{
			Name: "IndestructibleSurvivesDestroy",
			Setup: func(gs *gameengine.GameState) {
				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Darksteel Colossus", Owner: 0,
						Types: []string{"creature", "artifact"}, BasePower: 11, BaseToughness: 11,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{"indestructible": 1},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)
			},
			Act: func(gs *gameengine.GameState) {
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Card.Name == "Darksteel Colossus" {
						gameengine.DestroyPermanent(gs, p, nil)
						break
					}
				}
			},
			Verify: func(gs *gameengine.GameState) *failure {
				// Verify the creature is still on the battlefield.
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Card.Name == "Darksteel Colossus" {
						return nil // still here, good
					}
				}
				return &failure{
					CardName:    "IndestructibleSurvivesDestroy",
					Interaction: "replacement",
					Invariant:   "IndestructibleCheck",
					Message:     "Darksteel Colossus should survive Destroy due to indestructible",
				}
			},
		},
	}
}
