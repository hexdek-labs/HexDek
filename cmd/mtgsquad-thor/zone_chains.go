package main

// Module 8: Zone Chain Reactions (--zone-chains)
//
// Creates boards with dies-triggers that cause more deaths. Runs SBAs
// and verifies the chain terminates and invariants hold.

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

type zoneChainScenario struct {
	Name string
	Run  func() *failure
}

func runZoneChains(_ *astload.Corpus, _ []*oracleCard) []failure {
	scenarios := buildZoneChainScenarios()
	var fails []failure

	for _, sc := range scenarios {
		f := runZoneChainScenario(sc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	return fails
}

func runZoneChainScenario(sc zoneChainScenario) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			result = &failure{
				CardName:    sc.Name,
				Interaction: "zone_chains",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			}
		}
	}()

	return sc.Run()
}

func buildZoneChainScenarios() []zoneChainScenario {
	return []zoneChainScenario{
		{
			Name: "CreatureDies_BloodArtist_LifeDrain",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Blood Artist: whenever a creature dies, drain 1 life.
				bloodArtist := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Blood Artist", Owner: 0,
						Types: []string{"creature"}, BasePower: 0, BaseToughness: 1,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bloodArtist)

				// Zulaport Cutthroat: whenever a creature you control dies, drain 1.
				zulaport := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Zulaport Cutthroat", Owner: 0,
						Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, zulaport)

				// A creature that will die.
				victim := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Doomed Traveler", Owner: 0,
						Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, victim)

				gs.Snapshot()

				// Destroy the victim.
				gameengine.DestroyPermanent(gs, victim, nil)
				gameengine.StateBasedActions(gs)

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "CreatureDies_BloodArtist_LifeDrain",
						Interaction: "zone_chains",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "GravePact_CascadingSacrifice",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Grave Pact on seat 0.
				gravePact := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Grave Pact", Owner: 0,
						Types: []string{"enchantment"},
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, gravePact)

				// Creatures on seat 0 and seat 1.
				sacVictim := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Goblin Token", Owner: 0,
						Types: []string{"token", "creature"}, BasePower: 1, BaseToughness: 1,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, sacVictim)

				oppCreature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Opponent Goblin", Owner: 1,
						Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
					},
					Controller: 1, Owner: 1,
					Flags:      map[string]int{},
				}
				gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, oppCreature)

				gs.Snapshot()

				// Sacrifice the token.
				gameengine.SacrificePermanent(gs, sacVictim, "test")
				gameengine.StateBasedActions(gs)

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "GravePact_CascadingSacrifice",
						Interaction: "zone_chains",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "TokenDying_NoGraveyard",
			Run: func() *failure {
				gs := makeBaseGameState()

				// A token creature.
				token := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Saproling", Owner: 0,
						Types: []string{"token", "creature"}, BasePower: 1, BaseToughness: 1,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, token)

				gs.Snapshot()

				// Destroy the token.
				gameengine.DestroyPermanent(gs, token, nil)
				gameengine.StateBasedActions(gs)

				// Token should not be in graveyard.
				for _, c := range gs.Seats[0].Graveyard {
					if c != nil && c.Name == "Saproling" {
						found := false
						for _, t := range c.Types {
							if t == "token" {
								found = true
								break
							}
						}
						if found {
							return &failure{
								CardName:    "TokenDying_NoGraveyard",
								Interaction: "zone_chains",
								Invariant:   "TokenZone",
								Message:     "token should not persist in graveyard",
							}
						}
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "TokenDying_NoGraveyard",
						Interaction: "zone_chains",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "MultipleCreaturesDie_SBAs_Terminate",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Place 10 creatures with lethal damage already marked.
				for i := 0; i < 10; i++ {
					creature := &gameengine.Permanent{
						Card: &gameengine.Card{
							Name: fmt.Sprintf("Doomed_%d", i), Owner: 0,
							Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
						},
						Controller: 0, Owner: 0,
						Flags:      map[string]int{},
						MarkedDamage: 5, // way over lethal
					}
					gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)
				}

				gs.Snapshot()

				// Run SBAs — should kill all 10 creatures and terminate.
				gameengine.StateBasedActions(gs)

				// Verify battlefield is clean of the doomed creatures.
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil {
						for _, nm := range []string{"Doomed_"} {
							if len(p.Card.Name) > len(nm) && p.Card.Name[:len(nm)] == nm {
								return &failure{
									CardName:    "MultipleCreaturesDie_SBAs_Terminate",
									Interaction: "zone_chains",
									Invariant:   "SBACompleteness",
									Message: fmt.Sprintf("creature %s should have been removed by SBA", p.Card.Name),
								}
							}
						}
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "MultipleCreaturesDie_SBAs_Terminate",
						Interaction: "zone_chains",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "LethalDamage_Plus_MinusCounters",
			Run: func() *failure {
				gs := makeBaseGameState()

				// Creature with -1/-1 counters reducing toughness to 0.
				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Weakened Elf", Owner: 0,
						Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
					},
					Controller: 0, Owner: 0,
					Counters:   map[string]int{"-1/-1": 3}, // 2-3 = -1 toughness
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)

				gs.Snapshot()

				gameengine.StateBasedActions(gs)

				// Creature should have been removed (0 or negative toughness).
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Card.Name == "Weakened Elf" {
						return &failure{
							CardName:    "LethalDamage_Plus_MinusCounters",
							Interaction: "zone_chains",
							Invariant:   "SBACompleteness",
							Message:     "Weakened Elf with toughness <= 0 should have been killed by SBA",
						}
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "LethalDamage_Plus_MinusCounters",
						Interaction: "zone_chains",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "DestroyIndestructible_Survives",
			Run: func() *failure {
				gs := makeBaseGameState()

				creature := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Stuffy Doll", Owner: 0,
						Types: []string{"creature"}, BasePower: 0, BaseToughness: 1,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{"indestructible": 1},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)

				gs.Snapshot()

				// Mark lethal damage.
				creature.MarkedDamage = 10

				gameengine.StateBasedActions(gs)

				// Indestructible creature should still be on the battlefield
				// (lethal damage doesn't destroy indestructible).
				found := false
				for _, p := range gs.Seats[0].Battlefield {
					if p.Card != nil && p.Card.Name == "Stuffy Doll" {
						found = true
						break
					}
				}
				if !found {
					return &failure{
						CardName:    "DestroyIndestructible_Survives",
						Interaction: "zone_chains",
						Invariant:   "IndestructibleCheck",
						Message:     "indestructible creature should survive lethal damage",
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "DestroyIndestructible_Survives",
						Interaction: "zone_chains",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
	}
}
