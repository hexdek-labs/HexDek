package main

// Module 4: Layer Stress Boards (--layer-stress)
//
// Creates boards with multiple continuous effects at different layers.
// Calls GetEffectiveCharacteristics twice and verifies idempotency.
// Also runs SBAs.

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

type layerScenario struct {
	Name  string
	Setup func(gs *gameengine.GameState)
}

func runLayerStress(_ *astload.Corpus, _ []*oracleCard) []failure {
	scenarios := buildLayerScenarios()
	var fails []failure

	for _, sc := range scenarios {
		f := runLayerScenario(sc)
		fails = append(fails, f...)
	}

	return fails
}

func runLayerScenario(sc layerScenario) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			fails = append(fails, failure{
				CardName:    sc.Name,
				Interaction: "layer_stress",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			})
		}
	}()

	gs := makeBaseGameState()
	sc.Setup(gs)
	gs.Snapshot()

	// Run SBAs first.
	gameengine.StateBasedActions(gs)

	// Check layer idempotency: call GetEffectiveCharacteristics twice for
	// every permanent and verify they produce identical results.
	for seatIdx, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.PhasedOut {
				continue
			}

			gs.InvalidateCharacteristicsCache()
			c1 := gameengine.GetEffectiveCharacteristics(gs, p)
			gs.InvalidateCharacteristicsCache()
			c2 := gameengine.GetEffectiveCharacteristics(gs, p)

			if c1.Power != c2.Power || c1.Toughness != c2.Toughness {
				name := "<unknown>"
				if p.Card != nil {
					name = p.Card.Name
				}
				fails = append(fails, failure{
					CardName:    sc.Name,
					Interaction: "layer_stress",
					Invariant:   "LayerIdempotency",
					Message: fmt.Sprintf("seat %d perm %q: P/T differs between calls (%d/%d vs %d/%d)",
						seatIdx, name, c1.Power, c1.Toughness, c2.Power, c2.Toughness),
				})
			}

			if len(c1.Types) != len(c2.Types) {
				name := "<unknown>"
				if p.Card != nil {
					name = p.Card.Name
				}
				fails = append(fails, failure{
					CardName:    sc.Name,
					Interaction: "layer_stress",
					Invariant:   "LayerIdempotency",
					Message: fmt.Sprintf("seat %d perm %q: Types differ between calls (%v vs %v)",
						seatIdx, name, c1.Types, c2.Types),
				})
			}
		}
	}

	// Check all invariants.
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName:    sc.Name,
			Interaction: "layer_stress",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}

	return fails
}

func buildLayerScenarios() []layerScenario {
	return []layerScenario{
		{
			Name: "Humility_Alone",
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

				// Layer 7b: set all creatures to 1/1.
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 7, Sublayer: "b", Timestamp: ts,
					SourcePerm: humility, SourceCardName: "Humility",
					ControllerSeat: 0, HandlerID: "humility_pt",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
						return t.IsCreature()
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						c.Power = 1
						c.Toughness = 1
					},
				})

				// Layer 6: remove all abilities from creatures.
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 6, Timestamp: ts,
					SourcePerm: humility, SourceCardName: "Humility",
					ControllerSeat: 0, HandlerID: "humility_abilities",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
						return t.IsCreature()
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						c.Abilities = nil
						c.Keywords = nil
					},
				})

				// Several creatures with different P/T.
				for _, spec := range []struct {
					name  string
					pow   int
					tough int
				}{
					{"Tarmogoyf", 4, 5},
					{"Emrakul, the Aeons Torn", 15, 15},
					{"Birds of Paradise", 0, 1},
					{"Wall of Omens", 0, 4},
				} {
					p := &gameengine.Permanent{
						Card: &gameengine.Card{
							Name: spec.name, Owner: 0,
							Types: []string{"creature"}, BasePower: spec.pow, BaseToughness: spec.tough,
						},
						Controller: 0, Owner: 0,
						Flags:      map[string]int{},
					}
					gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
				}
			},
		},
		{
			Name: "Humility_Plus_Opalescence",
			Setup: func(gs *gameengine.GameState) {
				// Opalescence: each non-Aura enchantment is a creature with
				// P/T equal to its CMC.
				// Humility: all creatures are 1/1 with no abilities.
				// Dependency: Humility makes itself lose its ability if
				// Opalescence entered first (layer 7 dependency).
				ts1 := gs.NextTimestamp()
				opalescence := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Opalescence", Owner: 0,
						Types: []string{"enchantment"}, CMC: 4,
					},
					Controller: 0, Owner: 0, Timestamp: ts1,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, opalescence)

				// Opalescence effect: layer 4 (type) + layer 7b (set P/T).
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 4, Timestamp: ts1,
					SourcePerm: opalescence, SourceCardName: "Opalescence",
					ControllerSeat: 0, HandlerID: "opalescence_type",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
						return t.IsEnchantment() && !t.IsAura()
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						hasCreature := false
						for _, tp := range c.Types {
							if tp == "creature" {
								hasCreature = true
								break
							}
						}
						if !hasCreature {
							c.Types = append(c.Types, "creature")
						}
					},
				})
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 7, Sublayer: "b", Timestamp: ts1,
					SourcePerm: opalescence, SourceCardName: "Opalescence",
					ControllerSeat: 0, HandlerID: "opalescence_pt",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
						return t.IsEnchantment() && !t.IsAura()
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						c.Power = c.CMC
						c.Toughness = c.CMC
					},
				})

				ts2 := gs.NextTimestamp()
				humility := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Humility", Owner: 0,
						Types: []string{"enchantment"}, CMC: 4,
					},
					Controller: 0, Owner: 0, Timestamp: ts2,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, humility)

				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 7, Sublayer: "b", Timestamp: ts2,
					SourcePerm: humility, SourceCardName: "Humility",
					ControllerSeat: 0, HandlerID: "humility_pt_opal",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
						return t.IsCreature()
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						c.Power = 1
						c.Toughness = 1
					},
				})
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 6, Timestamp: ts2,
					SourcePerm: humility, SourceCardName: "Humility",
					ControllerSeat: 0, HandlerID: "humility_abilities_opal",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
						return t.IsCreature()
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						c.Abilities = nil
						c.Keywords = nil
					},
				})

				// Place a creature to stress the system.
				p := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Grizzly Bears", Owner: 1,
						Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
					},
					Controller: 1, Owner: 1,
					Flags:      map[string]int{},
				}
				gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, p)
			},
		},
		{
			Name: "BloodMoon_NonbasicsMountains",
			Setup: func(gs *gameengine.GameState) {
				ts := gs.NextTimestamp()
				bloodMoon := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Blood Moon", Owner: 0,
						Types: []string{"enchantment"},
					},
					Controller: 0, Owner: 0, Timestamp: ts,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bloodMoon)

				// Layer 4: nonbasic lands become Mountains (lose all land subtypes,
				// gain Mountain subtype).
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 4, Timestamp: ts,
					SourcePerm: bloodMoon, SourceCardName: "Blood Moon",
					ControllerSeat: 0, HandlerID: "blood_moon_type",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
						if !t.IsLand() {
							return false
						}
						// Check if it's a basic land (has "basic" supertype).
						for _, tp := range t.Card.Types {
							if tp == "basic" {
								return false
							}
						}
						return true
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						c.Subtypes = []string{"mountain"}
					},
				})

				// Place some nonbasic lands.
				for _, name := range []string{"Tropical Island", "Volcanic Island", "Command Tower"} {
					land := &gameengine.Permanent{
						Card: &gameengine.Card{
							Name: name, Owner: 1,
							Types: []string{"land"},
						},
						Controller: 1, Owner: 1,
						Flags:      map[string]int{},
					}
					gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, land)
				}

				// Place a basic land (should not be affected).
				basic := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Forest", Owner: 1,
						Types: []string{"land", "basic", "forest"},
					},
					Controller: 1, Owner: 1,
					Flags:      map[string]int{},
				}
				gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, basic)
			},
		},
		{
			Name: "MycosynthLattice_EverythingArtifact",
			Setup: func(gs *gameengine.GameState) {
				ts := gs.NextTimestamp()
				lattice := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Mycosynth Lattice", Owner: 0,
						Types: []string{"artifact"}, CMC: 6,
					},
					Controller: 0, Owner: 0, Timestamp: ts,
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, lattice)

				// Layer 4: all permanents are artifacts in addition to their other types.
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 4, Timestamp: ts,
					SourcePerm: lattice, SourceCardName: "Mycosynth Lattice",
					ControllerSeat: 0, HandlerID: "lattice_artifact",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, _ *gameengine.Permanent) bool {
						return true // affects everything
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						hasArtifact := false
						for _, tp := range c.Types {
							if tp == "artifact" {
								hasArtifact = true
								break
							}
						}
						if !hasArtifact {
							c.Types = append(c.Types, "artifact")
						}
					},
				})

				// Layer 5: all permanents are colorless.
				gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
					Layer: 5, Timestamp: ts,
					SourcePerm: lattice, SourceCardName: "Mycosynth Lattice",
					ControllerSeat: 0, HandlerID: "lattice_colorless",
					Duration: "permanent",
					Predicate: func(_ *gameengine.GameState, _ *gameengine.Permanent) bool {
						return true
					},
					ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, c *gameengine.Characteristics) {
						c.Colors = nil
					},
				})

				// Place various permanent types.
				for _, spec := range []struct {
					name  string
					types []string
				}{
					{"Grizzly Bears", []string{"creature"}},
					{"Tropical Island", []string{"land"}},
					{"Rhystic Study", []string{"enchantment"}},
				} {
					p := &gameengine.Permanent{
						Card: &gameengine.Card{
							Name: spec.name, Owner: 1,
							Types: spec.types, BasePower: 2, BaseToughness: 2,
						},
						Controller: 1, Owner: 1,
						Flags:      map[string]int{},
					}
					gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, p)
				}
			},
		},
		{
			Name: "PlusPlusCounters_Plus_MinusCounters",
			Setup: func(gs *gameengine.GameState) {
				// Creature with +1/+1 and -1/-1 counters. §704.5q says
				// pairs annihilate each other.
				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Test Creature", Owner: 0,
						Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
					},
					Controller: 0, Owner: 0,
					Counters: map[string]int{
						"+1/+1": 5,
						"-1/-1": 3,
					},
					Flags: map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)
			},
		},
		{
			Name: "MultipleBuffs_UntilEOT_StackCorrectly",
			Setup: func(gs *gameengine.GameState) {
				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Hill Giant", Owner: 0,
						Types: []string{"creature"}, BasePower: 3, BaseToughness: 3,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
					Modifications: []gameengine.Modification{
						{Power: 2, Toughness: 2, Duration: "until_end_of_turn", Timestamp: 1},
						{Power: -1, Toughness: -1, Duration: "until_end_of_turn", Timestamp: 2},
						{Power: 3, Toughness: 0, Duration: "until_end_of_turn", Timestamp: 3},
					},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)
			},
		},
	}
}
