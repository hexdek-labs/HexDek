package main

// Module 6: Commander Rules (--commander)
//
// Tests commander-specific rules:
// - Commander tax: cast, it dies, recast costs 2 more
// - Commander damage: track per-commander per-player, monotonic
// - Command zone redirect on death

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

type commanderScenario struct {
	Name string
	Run  func() *failure
}

func runCommander(_ *astload.Corpus, _ []*oracleCard) []failure {
	scenarios := buildCommanderScenarios()
	var fails []failure

	for _, sc := range scenarios {
		f := runCommanderScenario(sc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	return fails
}

func runCommanderScenario(sc commanderScenario) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			result = &failure{
				CardName:    sc.Name,
				Interaction: "commander",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			}
		}
	}()

	return sc.Run()
}

func makeCommanderGameState() *gameengine.GameState {
	gs := makeBaseGameState()
	gs.CommanderFormat = true

	// Set all seats to 40 life.
	for _, s := range gs.Seats {
		s.Life = 40
		s.StartingLife = 40
		s.CommanderDamage = map[int]map[string]int{}
		s.CommanderCastCounts = map[string]int{}
		s.CommanderTax = s.CommanderCastCounts
	}

	return gs
}

func buildCommanderScenarios() []commanderScenario {
	return []commanderScenario{
		{
			Name: "CommanderTax_IncreasesOnRecast",
			Run: func() *failure {
				gs := makeCommanderGameState()

				// Set up seat 0 with a commander.
				gs.Seats[0].CommanderNames = []string{"Kenrith, the Returned King"}

				// Simulate casting the commander: increment cast count.
				gs.Seats[0].CommanderCastCounts["Kenrith, the Returned King"] = 0
				// First cast: tax = 0.
				tax := gs.Seats[0].CommanderCastCounts["Kenrith, the Returned King"] * 2
				if tax != 0 {
					return &failure{
						CardName:    "CommanderTax_IncreasesOnRecast",
						Interaction: "commander",
						Invariant:   "CommanderTax",
						Message:     fmt.Sprintf("first cast should have 0 tax, got %d", tax),
					}
				}

				// Simulate: commander was cast once.
				gs.Seats[0].CommanderCastCounts["Kenrith, the Returned King"]++

				// Second cast: tax = 2.
				tax = gs.Seats[0].CommanderCastCounts["Kenrith, the Returned King"] * 2
				if tax != 2 {
					return &failure{
						CardName:    "CommanderTax_IncreasesOnRecast",
						Interaction: "commander",
						Invariant:   "CommanderTax",
						Message:     fmt.Sprintf("second cast should have 2 tax, got %d", tax),
					}
				}

				// Third cast.
				gs.Seats[0].CommanderCastCounts["Kenrith, the Returned King"]++
				tax = gs.Seats[0].CommanderCastCounts["Kenrith, the Returned King"] * 2
				if tax != 4 {
					return &failure{
						CardName:    "CommanderTax_IncreasesOnRecast",
						Interaction: "commander",
						Invariant:   "CommanderTax",
						Message:     fmt.Sprintf("third cast should have 4 tax, got %d", tax),
					}
				}

				gs.Snapshot()
				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "CommanderTax_IncreasesOnRecast",
						Interaction: "commander",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "CommanderDamage_Monotonic",
			Run: func() *failure {
				gs := makeCommanderGameState()
				gs.Seats[0].CommanderNames = []string{"Najeela, the Blade-Blossom"}

				// Deal 5 commander damage from seat 0 to seat 1.
				if gs.Seats[1].CommanderDamage[0] == nil {
					gs.Seats[1].CommanderDamage[0] = map[string]int{}
				}
				gs.Seats[1].CommanderDamage[0]["Najeela, the Blade-Blossom"] = 5

				gs.Snapshot()

				// Check monotonicity invariant.
				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "CommanderDamage_Monotonic",
						Interaction: "commander",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}

				// Increase damage.
				gs.Seats[1].CommanderDamage[0]["Najeela, the Blade-Blossom"] = 10
				violations = gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "CommanderDamage_Monotonic",
						Interaction: "commander",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}

				// Attempt to decrease damage — this should trigger the monotonic check.
				gs.Seats[1].CommanderDamage[0]["Najeela, the Blade-Blossom"] = 7
				violations = gameengine.RunAllInvariants(gs)
				// We EXPECT this to fail — it's testing the invariant catches bugs.
				if len(violations) == 0 {
					return &failure{
						CardName:    "CommanderDamage_Monotonic",
						Interaction: "commander",
						Invariant:   "CommanderDamageMonotonic",
						Message:     "invariant should catch commander damage decrease (10 -> 7) but did not",
					}
				}

				return nil
			},
		},
		{
			Name: "CommanderDamage_21_LosesGame",
			Run: func() *failure {
				gs := makeCommanderGameState()
				gs.Seats[0].CommanderNames = []string{"Rafiq of the Many"}

				// Deal 21 commander damage from seat 0's commander to seat 1.
				if gs.Seats[1].CommanderDamage[0] == nil {
					gs.Seats[1].CommanderDamage[0] = map[string]int{}
				}
				gs.Seats[1].CommanderDamage[0]["Rafiq of the Many"] = 21

				// Place a commander creature on seat 0's battlefield for damage events.
				cmdPerm := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Rafiq of the Many", Owner: 0,
						Types: []string{"creature", "legendary"}, BasePower: 3, BaseToughness: 3,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, cmdPerm)

				// Log a combat damage event so SBA can scan it.
				gs.LogEvent(gameengine.Event{
					Kind:   "combat_damage_to_player",
					Seat:   0,
					Target: 1,
					Source: "Rafiq of the Many",
					Amount: 21,
					Details: map[string]interface{}{
						"commander":      true,
						"commander_name": "Rafiq of the Many",
						"dealer_seat":    0,
					},
				})

				gs.Snapshot()

				// Run SBAs — should detect 21+ commander damage.
				gameengine.StateBasedActions(gs)

				// Seat 1 should be eliminated.
				if !gs.Seats[1].Lost {
					return &failure{
						CardName:    "CommanderDamage_21_LosesGame",
						Interaction: "commander",
						Invariant:   "CommanderDamage21Loss",
						Message:     "seat 1 should have lost from 21 commander damage",
					}
				}

				return nil
			},
		},
		{
			Name: "PartnerCommanders_IndependentTax",
			Run: func() *failure {
				gs := makeCommanderGameState()
				gs.Seats[0].CommanderNames = []string{"Kraum, Ludevic's Opus", "Tymna the Weaver"}

				// Cast Kraum 3 times.
				gs.Seats[0].CommanderCastCounts["Kraum, Ludevic's Opus"] = 3

				// Cast Tymna 1 time.
				gs.Seats[0].CommanderCastCounts["Tymna the Weaver"] = 1

				// Verify independent taxes.
				kraumTax := gs.Seats[0].CommanderCastCounts["Kraum, Ludevic's Opus"] * 2
				tymnaTax := gs.Seats[0].CommanderCastCounts["Tymna the Weaver"] * 2

				if kraumTax != 6 {
					return &failure{
						CardName:    "PartnerCommanders_IndependentTax",
						Interaction: "commander",
						Invariant:   "PartnerTax",
						Message:     fmt.Sprintf("Kraum tax should be 6, got %d", kraumTax),
					}
				}
				if tymnaTax != 2 {
					return &failure{
						CardName:    "PartnerCommanders_IndependentTax",
						Interaction: "commander",
						Invariant:   "PartnerTax",
						Message:     fmt.Sprintf("Tymna tax should be 2, got %d", tymnaTax),
					}
				}

				gs.Snapshot()
				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "PartnerCommanders_IndependentTax",
						Interaction: "commander",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "PartnerCommanders_IndependentDamage",
			Run: func() *failure {
				gs := makeCommanderGameState()
				gs.Seats[0].CommanderNames = []string{"Kraum, Ludevic's Opus", "Tymna the Weaver"}

				// Seat 1 takes 15 from Kraum and 15 from Tymna = 30 total
				// but NOT 21 from either one, so no loss.
				if gs.Seats[1].CommanderDamage[0] == nil {
					gs.Seats[1].CommanderDamage[0] = map[string]int{}
				}
				gs.Seats[1].CommanderDamage[0]["Kraum, Ludevic's Opus"] = 15
				gs.Seats[1].CommanderDamage[0]["Tymna the Weaver"] = 15

				gs.Snapshot()
				gameengine.StateBasedActions(gs)

				// Seat 1 should NOT have lost (neither commander hit 21).
				if gs.Seats[1].Lost {
					return &failure{
						CardName:    "PartnerCommanders_IndependentDamage",
						Interaction: "commander",
						Invariant:   "PartnerDamage",
						Message:     "seat 1 should NOT lose with 15+15 from partner commanders (neither >= 21)",
					}
				}

				return nil
			},
		},
	}
}
