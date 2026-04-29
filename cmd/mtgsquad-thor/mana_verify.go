package main

// Module 9: Mana Payment (--mana-verify)
//
// Tests mana pool operations:
// - Add colored mana, verify pool counts
// - Pay generic from colored pool
// - Pay colored cost with correct color
// - DrainAllPools at end of phase
// - Restricted mana (Food Chain creature-only, Powerstone noncreature)

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

type manaScenario struct {
	Name string
	Run  func() *failure
}

func runManaVerify(_ *astload.Corpus, _ []*oracleCard) []failure {
	scenarios := buildManaScenarios()
	var fails []failure

	for _, sc := range scenarios {
		f := runManaScenario(sc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	return fails
}

func runManaScenario(sc manaScenario) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			result = &failure{
				CardName:    sc.Name,
				Interaction: "mana_verify",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			}
		}
	}()

	return sc.Run()
}

func buildManaScenarios() []manaScenario {
	return []manaScenario{
		{
			Name: "AddColoredMana_VerifyCounts",
			Run: func() *failure {
				gs := makeBaseGameState()
				seat := gs.Seats[0]

				gameengine.AddMana(gs, seat, "W", 3, "Plains")
				gameengine.AddMana(gs, seat, "U", 2, "Island")
				gameengine.AddMana(gs, seat, "B", 1, "Swamp")

				pool := gameengine.EnsureTypedPool(seat)
				if pool.W != 3 {
					return &failure{
						CardName: "AddColoredMana_VerifyCounts", Interaction: "mana_verify",
						Invariant: "ManaCount", Message: fmt.Sprintf("expected W=3, got %d", pool.W),
					}
				}
				if pool.U != 2 {
					return &failure{
						CardName: "AddColoredMana_VerifyCounts", Interaction: "mana_verify",
						Invariant: "ManaCount", Message: fmt.Sprintf("expected U=2, got %d", pool.U),
					}
				}
				if pool.B != 1 {
					return &failure{
						CardName: "AddColoredMana_VerifyCounts", Interaction: "mana_verify",
						Invariant: "ManaCount", Message: fmt.Sprintf("expected B=1, got %d", pool.B),
					}
				}
				if pool.Total() != 6 {
					return &failure{
						CardName: "AddColoredMana_VerifyCounts", Interaction: "mana_verify",
						Invariant: "ManaTotal", Message: fmt.Sprintf("expected total=6, got %d", pool.Total()),
					}
				}

				// Verify legacy ManaPool is synced.
				if seat.ManaPool != 6 {
					return &failure{
						CardName: "AddColoredMana_VerifyCounts", Interaction: "mana_verify",
						Invariant: "LegacySync", Message: fmt.Sprintf("legacy ManaPool should be 6, got %d", seat.ManaPool),
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "AddColoredMana_VerifyCounts", Interaction: "mana_verify",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "PayGeneric_FromColoredPool",
			Run: func() *failure {
				gs := makeBaseGameState()
				seat := gs.Seats[0]

				gameengine.AddMana(gs, seat, "R", 5, "Mountain")

				// Pay 3 generic.
				ok := gameengine.PayGenericCost(gs, seat, 3, "creature", "test", "Test Spell")
				if !ok {
					return &failure{
						CardName: "PayGeneric_FromColoredPool", Interaction: "mana_verify",
						Invariant: "PayGeneric", Message: "should be able to pay 3 generic from 5R",
					}
				}

				pool := gameengine.EnsureTypedPool(seat)
				if pool.Total() != 2 {
					return &failure{
						CardName: "PayGeneric_FromColoredPool", Interaction: "mana_verify",
						Invariant: "ManaAfterPay", Message: fmt.Sprintf("expected 2 remaining, got %d", pool.Total()),
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "PayGeneric_FromColoredPool", Interaction: "mana_verify",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "PayGeneric_InsufficientMana",
			Run: func() *failure {
				gs := makeBaseGameState()
				seat := gs.Seats[0]

				gameengine.AddMana(gs, seat, "G", 2, "Forest")

				// Try to pay 5 generic from 2G — should fail.
				ok := gameengine.PayGenericCost(gs, seat, 5, "creature", "test", "Big Spell")
				if ok {
					return &failure{
						CardName: "PayGeneric_InsufficientMana", Interaction: "mana_verify",
						Invariant: "PayFail", Message: "should NOT be able to pay 5 generic from 2G",
					}
				}

				// Mana pool should be unchanged.
				pool := gameengine.EnsureTypedPool(seat)
				if pool.Total() != 2 {
					return &failure{
						CardName: "PayGeneric_InsufficientMana", Interaction: "mana_verify",
						Invariant: "ManaUnchanged", Message: fmt.Sprintf("pool should still be 2, got %d", pool.Total()),
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "PayGeneric_InsufficientMana", Interaction: "mana_verify",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "DrainAllPools_EmptiesMana",
			Run: func() *failure {
				gs := makeBaseGameState()
				seat := gs.Seats[0]

				gameengine.AddMana(gs, seat, "W", 3, "Plains")
				gameengine.AddMana(gs, seat, "U", 2, "Island")
				gameengine.AddMana(gs, seat, "R", 4, "Mountain")

				// Drain at phase boundary.
				gameengine.DrainAllPools(gs, "precombat_main", "")

				pool := gameengine.EnsureTypedPool(seat)
				if pool.Total() != 0 {
					return &failure{
						CardName: "DrainAllPools_EmptiesMana", Interaction: "mana_verify",
						Invariant: "DrainTotal", Message: fmt.Sprintf("pool should be 0 after drain, got %d", pool.Total()),
					}
				}
				if seat.ManaPool != 0 {
					return &failure{
						CardName: "DrainAllPools_EmptiesMana", Interaction: "mana_verify",
						Invariant: "LegacyDrain", Message: fmt.Sprintf("legacy pool should be 0, got %d", seat.ManaPool),
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "DrainAllPools_EmptiesMana", Interaction: "mana_verify",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "RestrictedMana_CreatureOnly",
			Run: func() *failure {
				gs := makeBaseGameState()
				seat := gs.Seats[0]

				// Add restricted mana like Food Chain produces.
				gameengine.AddRestrictedMana(gs, seat, 5, "", "creature_spell_only", "Food Chain")

				pool := gameengine.EnsureTypedPool(seat)

				// Should be payable for creature spells.
				if !pool.CanPayGeneric(5, "creature") {
					return &failure{
						CardName: "RestrictedMana_CreatureOnly", Interaction: "mana_verify",
						Invariant: "RestrictedPay", Message: "Food Chain mana should pay for creatures",
					}
				}

				// Should NOT be payable for noncreature spells.
				if pool.CanPayGeneric(5, "instant") {
					return &failure{
						CardName: "RestrictedMana_CreatureOnly", Interaction: "mana_verify",
						Invariant: "RestrictedBlock", Message: "Food Chain mana should NOT pay for instants",
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "RestrictedMana_CreatureOnly", Interaction: "mana_verify",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "RestrictedMana_NoncreatureOnly",
			Run: func() *failure {
				gs := makeBaseGameState()
				seat := gs.Seats[0]

				// Powerstone-style mana: noncreature only.
				gameengine.AddRestrictedMana(gs, seat, 3, "C", "noncreature_or_artifact_activation", "Powerstone Token")

				pool := gameengine.EnsureTypedPool(seat)

				// Should be payable for activated abilities.
				if !pool.CanPayGeneric(3, "activated") {
					return &failure{
						CardName: "RestrictedMana_NoncreatureOnly", Interaction: "mana_verify",
						Invariant: "RestrictedPay", Message: "Powerstone mana should pay for activated abilities",
					}
				}

				// Should NOT be payable for creature spells.
				if pool.CanPayGeneric(3, "creature") {
					return &failure{
						CardName: "RestrictedMana_NoncreatureOnly", Interaction: "mana_verify",
						Invariant: "RestrictedBlock", Message: "Powerstone mana should NOT pay for creature spells",
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "RestrictedMana_NoncreatureOnly", Interaction: "mana_verify",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "MixedPool_ColoredPlusGeneric",
			Run: func() *failure {
				gs := makeBaseGameState()
				seat := gs.Seats[0]

				gameengine.AddMana(gs, seat, "W", 2, "Plains")
				gameengine.AddMana(gs, seat, "any", 3, "Sol Ring")

				pool := gameengine.EnsureTypedPool(seat)

				if pool.W != 2 {
					return &failure{
						CardName: "MixedPool_ColoredPlusGeneric", Interaction: "mana_verify",
						Invariant: "MixedW", Message: fmt.Sprintf("expected W=2, got %d", pool.W),
					}
				}
				if pool.Any != 3 {
					return &failure{
						CardName: "MixedPool_ColoredPlusGeneric", Interaction: "mana_verify",
						Invariant: "MixedAny", Message: fmt.Sprintf("expected Any=3, got %d", pool.Any),
					}
				}
				if pool.Total() != 5 {
					return &failure{
						CardName: "MixedPool_ColoredPlusGeneric", Interaction: "mana_verify",
						Invariant: "MixedTotal", Message: fmt.Sprintf("expected total=5, got %d", pool.Total()),
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "MixedPool_ColoredPlusGeneric", Interaction: "mana_verify",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "ColorlessMana_Distinct",
			Run: func() *failure {
				gs := makeBaseGameState()
				seat := gs.Seats[0]

				// Colorless (C) is distinct from generic.
				gameengine.AddMana(gs, seat, "C", 4, "Wastes")

				pool := gameengine.EnsureTypedPool(seat)
				if pool.C != 4 {
					return &failure{
						CardName: "ColorlessMana_Distinct", Interaction: "mana_verify",
						Invariant: "ColorlessC", Message: fmt.Sprintf("expected C=4, got %d", pool.C),
					}
				}

				// Can pay generic with colorless.
				if !pool.CanPayGeneric(4, "creature") {
					return &failure{
						CardName: "ColorlessMana_Distinct", Interaction: "mana_verify",
						Invariant: "ColorlessGeneric", Message: "colorless should be able to pay generic costs",
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName: "ColorlessMana_Distinct", Interaction: "mana_verify",
						Invariant: violations[0].Name, Message: violations[0].Message,
					}
				}
				return nil
			},
		},
	}
}
