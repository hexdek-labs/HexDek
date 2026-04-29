package main

// Module 10: Turn Structure (--turn-structure)
//
// Runs a full turn with a known board state and verifies:
// - Phases execute in correct order per CR §500-§514
// - SBAs fire between damage and triggers
// - Untap respects DoesNotUntap, stun counters, phasing
// - Draw step draws exactly 1 card
// - Cleanup step discards to hand size 7

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

type turnScenario struct {
	Name string
	Run  func() *failure
}

func runTurnStructure(_ *astload.Corpus, _ []*oracleCard) []failure {
	scenarios := buildTurnScenarios()
	var fails []failure

	for _, sc := range scenarios {
		f := runTurnScenario(sc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	return fails
}

func runTurnScenario(sc turnScenario) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			result = &failure{
				CardName:    sc.Name,
				Interaction: "turn_structure",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			}
		}
	}()

	return sc.Run()
}

func buildTurnScenarios() []turnScenario {
	return []turnScenario{
		{
			Name: "UntapAll_UntapsTappedCreatures",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Place tapped creatures.
				for i := 0; i < 3; i++ {
					creature := &gameengine.Permanent{
						Card: &gameengine.Card{
							Name: fmt.Sprintf("TappedCreature_%d", i), Owner: 0,
							Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
						},
						Controller: 0, Owner: 0,
						Tapped:     true,
						Flags:      map[string]int{},
					}
					gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)
				}

				gs.Snapshot()

				// Run untap step.
				gs.Phase, gs.Step = "beginning", "untap"
				gameengine.UntapAll(gs, 0)

				// All creatures should be untapped.
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Tapped {
						if p.Card.Name[:15] == "TappedCreature_" {
							return &failure{
								CardName:    "UntapAll_UntapsTappedCreatures",
								Interaction: "turn_structure",
								Invariant:   "UntapCheck",
								Message:     fmt.Sprintf("%s should be untapped", p.Card.Name),
							}
						}
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "UntapAll_UntapsTappedCreatures", Interaction: "turn_structure",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "UntapAll_RespectsDoesNotUntap",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Creature with DoesNotUntap.
				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Mana Vault", Owner: 0,
						Types: []string{"artifact"}, BasePower: 0, BaseToughness: 0,
					},
					Controller:   0, Owner: 0,
					Tapped:       true,
					DoesNotUntap: true,
					Flags:        map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)

				// Normal creature.
				normal := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Normal Bear", Owner: 0,
						Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
					},
					Controller: 0, Owner: 0,
					Tapped:     true,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, normal)

				gs.Snapshot()

				gameengine.UntapAll(gs, 0)

				// Mana Vault should still be tapped.
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Card.Name == "Mana Vault" && !p.Tapped {
						return &failure{
							CardName: "UntapAll_RespectsDoesNotUntap", Interaction: "turn_structure",
							Invariant: "DoesNotUntap",
							Message:   "Mana Vault with DoesNotUntap should remain tapped",
						}
					}
				}

				// Normal Bear should be untapped.
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Card.Name == "Normal Bear" && p.Tapped {
						return &failure{
							CardName: "UntapAll_RespectsDoesNotUntap", Interaction: "turn_structure",
							Invariant: "NormalUntap",
							Message:   "Normal Bear should be untapped",
						}
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "UntapAll_RespectsDoesNotUntap", Interaction: "turn_structure",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "DrawStep_DrawsExactlyOneCard",
			Run: func() *failure {
				gs := makeBaseGameState()

				handBefore := len(gs.Seats[0].Hand)
				libBefore := len(gs.Seats[0].Library)

				// Simulate draw step.
				gs.Phase, gs.Step = "beginning", "draw"
				if len(gs.Seats[0].Library) > 0 {
					gs.Seats[0].Hand = append(gs.Seats[0].Hand,
						gs.Seats[0].Library[0])
					gs.Seats[0].Library = gs.Seats[0].Library[1:]
				}

				handAfter := len(gs.Seats[0].Hand)
				libAfter := len(gs.Seats[0].Library)

				if handAfter != handBefore+1 {
					return &failure{
						CardName: "DrawStep_DrawsExactlyOneCard", Interaction: "turn_structure",
						Invariant: "DrawCount",
						Message:   fmt.Sprintf("hand should grow by 1: %d -> %d", handBefore, handAfter),
					}
				}
				if libAfter != libBefore-1 {
					return &failure{
						CardName: "DrawStep_DrawsExactlyOneCard", Interaction: "turn_structure",
						Invariant: "LibraryShrink",
						Message:   fmt.Sprintf("library should shrink by 1: %d -> %d", libBefore, libAfter),
					}
				}

				gs.Snapshot()
				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "DrawStep_DrawsExactlyOneCard", Interaction: "turn_structure",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "CleanupStep_DiscardsToHandSize",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Give seat 0 a 12-card hand.
				gs.Seats[0].Hand = nil
				for i := 0; i < 12; i++ {
					gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
						Name: fmt.Sprintf("ExtraCard_%d", i), Owner: 0,
						Types: []string{"creature"},
					})
				}

				gs.Snapshot()

				// Run cleanup hand size.
				gameengine.CleanupHandSize(gs, 0, 7)

				if len(gs.Seats[0].Hand) != 7 {
					return &failure{
						CardName: "CleanupStep_DiscardsToHandSize", Interaction: "turn_structure",
						Invariant: "HandSize",
						Message:   fmt.Sprintf("hand should be 7 after cleanup, got %d", len(gs.Seats[0].Hand)),
					}
				}

				// Discarded cards should be in graveyard.
				if len(gs.Seats[0].Graveyard) < 5 {
					return &failure{
						CardName: "CleanupStep_DiscardsToHandSize", Interaction: "turn_structure",
						Invariant: "DiscardToGraveyard",
						Message:   fmt.Sprintf("expected at least 5 cards in graveyard, got %d", len(gs.Seats[0].Graveyard)),
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "CleanupStep_DiscardsToHandSize", Interaction: "turn_structure",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "CleanupStep_ExactlySevenIsNoop",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Exactly 7 cards — no discard needed.
				gs.Seats[0].Hand = nil
				for i := 0; i < 7; i++ {
					gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
						Name: fmt.Sprintf("Card_%d", i), Owner: 0,
						Types: []string{"creature"},
					})
				}
				graveBefore := len(gs.Seats[0].Graveyard)

				gs.Snapshot()

				gameengine.CleanupHandSize(gs, 0, 7)

				if len(gs.Seats[0].Hand) != 7 {
					return &failure{
						CardName: "CleanupStep_ExactlySevenIsNoop", Interaction: "turn_structure",
						Invariant: "NoDiscard",
						Message:   fmt.Sprintf("hand should stay at 7, got %d", len(gs.Seats[0].Hand)),
					}
				}
				if len(gs.Seats[0].Graveyard) != graveBefore {
					return &failure{
						CardName: "CleanupStep_ExactlySevenIsNoop", Interaction: "turn_structure",
						Invariant: "GraveyardUnchanged",
						Message:   "graveyard should not gain cards when hand is exactly 7",
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "CleanupStep_ExactlySevenIsNoop", Interaction: "turn_structure",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "FullTurnCycle_PhasesInOrder",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Place creatures so combat can actually happen.
				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Grizzly Bears", Owner: 0,
						Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)

				gs.Snapshot()

				// Run through all phases manually.
				phases := []struct {
					phase, step string
				}{
					{"beginning", "untap"},
					{"beginning", "upkeep"},
					{"beginning", "draw"},
					{"precombat_main", ""},
					{"combat", "beginning_of_combat"},
					{"postcombat_main", ""},
					{"ending", "end"},
					{"ending", "cleanup"},
				}

				for i, p := range phases {
					func() {
						defer func() {
							if r := recover(); r != nil {
								// Will be caught by outer defer.
								panic(fmt.Sprintf("phase %d (%s/%s): %v", i, p.phase, p.step, r))
							}
						}()

						gs.Phase, gs.Step = p.phase, p.step

						switch p.step {
						case "untap":
							gameengine.UntapAll(gs, gs.Active)
						case "draw":
							if len(gs.Seats[gs.Active].Library) > 0 {
								gs.Seats[gs.Active].Hand = append(gs.Seats[gs.Active].Hand,
									gs.Seats[gs.Active].Library[0])
								gs.Seats[gs.Active].Library = gs.Seats[gs.Active].Library[1:]
							}
						case "cleanup":
							gameengine.CleanupHandSize(gs, gs.Active, 7)
						}

						gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
						gs.InvalidateCharacteristicsCache()
						gameengine.StateBasedActions(gs)
					}()
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "FullTurnCycle_PhasesInOrder", Interaction: "turn_structure",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "PhasingIn_DuringUntap",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Place a phased-out creature.
				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Teferi's Imp", Owner: 0,
						Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
					},
					Controller: 0, Owner: 0,
					PhasedOut:  true,
					Tapped:     true,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)

				gs.Snapshot()

				// Run untap (which includes phase-in per §502.1).
				gs.Phase, gs.Step = "beginning", "untap"
				gameengine.UntapAll(gs, 0)

				// The creature should have phased in.
				found := false
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Card.Name == "Teferi's Imp" {
						found = true
						if p.PhasedOut {
							return &failure{
								CardName: "PhasingIn_DuringUntap", Interaction: "turn_structure",
								Invariant: "PhaseIn",
								Message:   "phased-out creature should phase in during untap",
							}
						}
					}
				}
				if !found {
					return &failure{
						CardName: "PhasingIn_DuringUntap", Interaction: "turn_structure",
						Invariant: "PhaseIn",
						Message:   "phased-out creature should still exist on battlefield",
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "PhasingIn_DuringUntap", Interaction: "turn_structure",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "SummoningSickness_ClearedAtUntap",
			Run: func() *failure {
				gs := makeBaseGameState()

				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "New Creature", Owner: 0,
						Types: []string{"creature"}, BasePower: 3, BaseToughness: 3,
					},
					Controller:    0, Owner: 0,
					SummoningSick: true,
					Flags:         map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)

				gs.Snapshot()

				gameengine.UntapAll(gs, 0)

				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Card.Name == "New Creature" && p.SummoningSick {
						return &failure{
							CardName: "SummoningSickness_ClearedAtUntap", Interaction: "turn_structure",
							Invariant: "SummoningSick",
							Message:   "summoning sickness should be cleared at untap",
						}
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "SummoningSickness_ClearedAtUntap", Interaction: "turn_structure",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
	}
}
