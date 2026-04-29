package main

// Module 5: Stack Torture (--stack-torture)
//
// Pushes multiple items onto the stack and resolves them.
// Checks LIFO order, fizzle detection, target legality.

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

type stackScenario struct {
	Name string
	Run  func(gs *gameengine.GameState) *failure
}

func runStackTorture(_ *astload.Corpus, _ []*oracleCard) []failure {
	scenarios := buildStackScenarios()
	var fails []failure

	for _, sc := range scenarios {
		f := runStackScenario(sc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	return fails
}

func runStackScenario(sc stackScenario) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			result = &failure{
				CardName:    sc.Name,
				Interaction: "stack_torture",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			}
		}
	}()

	gs := makeBaseGameState()
	gs.Snapshot()
	return sc.Run(gs)
}

func buildStackScenarios() []stackScenario {
	return []stackScenario{
		{
			Name: "PushResolve_LIFO",
			Run: func(gs *gameengine.GameState) *failure {
				// Push 3 items, verify they resolve in LIFO order.
				for i := 0; i < 3; i++ {
					item := &gameengine.StackItem{
						Controller: 0,
						Card: &gameengine.Card{
							Name: fmt.Sprintf("Spell_%d", i), Owner: 0,
							Types: []string{"instant"},
						},
					}
					gameengine.PushStackItem(gs, item)
				}

				if len(gs.Stack) != 3 {
					return &failure{
						CardName:    "PushResolve_LIFO",
						Interaction: "stack_torture",
						Invariant:   "StackSize",
						Message:     fmt.Sprintf("expected 3 items on stack, got %d", len(gs.Stack)),
					}
				}

				// Top of stack should be Spell_2 (last pushed).
				top := gs.Stack[len(gs.Stack)-1]
				if top.Card.Name != "Spell_2" {
					return &failure{
						CardName:    "PushResolve_LIFO",
						Interaction: "stack_torture",
						Invariant:   "LIFOOrder",
						Message:     fmt.Sprintf("expected top to be Spell_2, got %s", top.Card.Name),
					}
				}

				// Resolve top — should remove Spell_2.
				gameengine.ResolveStackTop(gs)
				if len(gs.Stack) != 2 {
					return &failure{
						CardName:    "PushResolve_LIFO",
						Interaction: "stack_torture",
						Invariant:   "StackSize",
						Message:     fmt.Sprintf("expected 2 items after resolve, got %d", len(gs.Stack)),
					}
				}

				// Verify invariants.
				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "PushResolve_LIFO",
						Interaction: "stack_torture",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}

				return nil
			},
		},
		{
			Name: "CounteredSpell_Fizzles",
			Run: func(gs *gameengine.GameState) *failure {
				// Push a spell, mark it countered, resolve.
				spell := &gameengine.StackItem{
					Controller: 0,
					Card: &gameengine.Card{
						Name: "Lightning Bolt", Owner: 0,
						Types: []string{"instant"},
					},
				}
				gameengine.PushStackItem(gs, spell)

				// Counter it.
				gs.Stack[len(gs.Stack)-1].Countered = true

				// Resolve — should remove without effect.
				gameengine.ResolveStackTop(gs)

				if len(gs.Stack) != 0 {
					return &failure{
						CardName:    "CounteredSpell_Fizzles",
						Interaction: "stack_torture",
						Invariant:   "StackEmpty",
						Message:     fmt.Sprintf("expected empty stack after resolving countered spell, got %d", len(gs.Stack)),
					}
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "CounteredSpell_Fizzles",
						Interaction: "stack_torture",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "MultipleResolves_StackDrains",
			Run: func(gs *gameengine.GameState) *failure {
				// Push 5 items, resolve all.
				for i := 0; i < 5; i++ {
					item := &gameengine.StackItem{
						Controller: i % len(gs.Seats),
						Card: &gameengine.Card{
							Name: fmt.Sprintf("Spell_%d", i), Owner: i % len(gs.Seats),
							Types: []string{"instant"},
						},
					}
					gameengine.PushStackItem(gs, item)
				}

				for len(gs.Stack) > 0 {
					gameengine.ResolveStackTop(gs)
					gameengine.StateBasedActions(gs)
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "MultipleResolves_StackDrains",
						Interaction: "stack_torture",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "TriggeredAbility_OnStack",
			Run: func(gs *gameengine.GameState) *failure {
				// Place a permanent, push a triggered ability from it.
				src := &gameengine.Permanent{
					Card: &gameengine.Card{
						Name: "Blood Artist", Owner: 0,
						Types: []string{"creature"}, BasePower: 0, BaseToughness: 1,
					},
					Controller: 0, Owner: 0,
					Flags:      map[string]int{},
				}
				gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

				// Push a triggered ability (using nil effect which is safe).
				item := &gameengine.StackItem{
					Controller: 0,
					Source:     src,
					Kind:       "triggered",
				}
				gameengine.PushStackItem(gs, item)

				if len(gs.Stack) != 1 {
					return &failure{
						CardName:    "TriggeredAbility_OnStack",
						Interaction: "stack_torture",
						Invariant:   "StackSize",
						Message:     fmt.Sprintf("expected 1 item, got %d", len(gs.Stack)),
					}
				}

				gameengine.ResolveStackTop(gs)
				gameengine.StateBasedActions(gs)

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "TriggeredAbility_OnStack",
						Interaction: "stack_torture",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "EmptyStack_ResolveNoop",
			Run: func(gs *gameengine.GameState) *failure {
				// Resolving an empty stack should not panic.
				gameengine.ResolveStackTop(gs)

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "EmptyStack_ResolveNoop",
						Interaction: "stack_torture",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
		{
			Name: "MassivePush_10Items",
			Run: func(gs *gameengine.GameState) *failure {
				for i := 0; i < 10; i++ {
					item := &gameengine.StackItem{
						Controller: i % len(gs.Seats),
						Card: &gameengine.Card{
							Name: fmt.Sprintf("MassSpell_%d", i), Owner: i % len(gs.Seats),
							Types: []string{"instant"},
						},
					}
					gameengine.PushStackItem(gs, item)
				}

				// Resolve all, checking SBAs between each.
				for len(gs.Stack) > 0 {
					gameengine.ResolveStackTop(gs)
					gameengine.StateBasedActions(gs)
				}

				violations := gameengine.RunAllInvariants(gs)
				if len(violations) > 0 {
					return &failure{
						CardName:    "MassivePush_10Items",
						Interaction: "stack_torture",
						Invariant:   violations[0].Name,
						Message:     violations[0].Message,
					}
				}
				return nil
			},
		},
	}
}
