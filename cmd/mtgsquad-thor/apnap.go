package main

// Module 7: Trigger Ordering APNAP (--apnap)
//
// 4 players each have a triggered ability that fires on the same event.
// Verifies triggers are pushed in APNAP order (active player first,
// then clockwise).

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func runAPNAP(_ *astload.Corpus, _ []*oracleCard) []failure {
	scenarios := buildAPNAPScenarios()
	var fails []failure

	for _, sc := range scenarios {
		f := runAPNAPScenario(sc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	return fails
}

type apnapScenario struct {
	Name string
	Run  func() *failure
}

func runAPNAPScenario(sc apnapScenario) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			result = &failure{
				CardName:    sc.Name,
				Interaction: "apnap",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			}
		}
	}()

	return sc.Run()
}

func buildAPNAPScenarios() []apnapScenario {
	return []apnapScenario{
		{
			Name: "APNAP_4Players_OrderCheck",
			Run: func() *failure {
				gs := makeBaseGameState() // has 4 seats
				gs.Active = 2            // seat 2 is active player

				// Verify APNAP order should be: 2, 3, 0, 1.
				// Create triggered abilities from each seat.
				var triggers []*gameengine.StackItem
				for i := 0; i < 4; i++ {
					item := &gameengine.StackItem{
						Controller: i,
						Source: &gameengine.Permanent{
							Card: &gameengine.Card{
								Name: fmt.Sprintf("Trigger_Source_Seat_%d", i), Owner: i,
								Types: []string{"creature"},
							},
							Controller: i, Owner: i,
						},
						Kind: "triggered",
					}
					triggers = append(triggers, item)
				}

				// Order them via APNAP.
				ordered := gameengine.OrderTriggersAPNAP(gs, triggers)

				if len(ordered) != 4 {
					return &failure{
						CardName:    "APNAP_4Players_OrderCheck",
						Interaction: "apnap",
						Invariant:   "OrderCount",
						Message:     fmt.Sprintf("expected 4 ordered triggers, got %d", len(ordered)),
					}
				}

				// APNAP with active=2: push order should be 2, 3, 0, 1.
				// Element [0] pushed first (resolves last), element [3] pushed
				// last (resolves first).
				expectedOrder := []int{2, 3, 0, 1}
				for i, item := range ordered {
					if item.Controller != expectedOrder[i] {
						return &failure{
							CardName:    "APNAP_4Players_OrderCheck",
							Interaction: "apnap",
							Invariant:   "APNAPOrder",
							Message: fmt.Sprintf("position %d: expected controller %d, got %d (active=%d)",
								i, expectedOrder[i], item.Controller, gs.Active),
						}
					}
				}

				return nil
			},
		},
		{
			Name: "APNAP_ActiveSeat0",
			Run: func() *failure {
				gs := makeBaseGameState()
				gs.Active = 0

				var triggers []*gameengine.StackItem
				for i := 0; i < 4; i++ {
					item := &gameengine.StackItem{
						Controller: i,
						Source: &gameengine.Permanent{
							Card: &gameengine.Card{
								Name: fmt.Sprintf("Source_%d", i), Owner: i,
								Types: []string{"creature"},
							},
							Controller: i, Owner: i,
						},
						Kind: "triggered",
					}
					triggers = append(triggers, item)
				}

				ordered := gameengine.OrderTriggersAPNAP(gs, triggers)

				// APNAP with active=0: push order is 0, 1, 2, 3.
				expectedOrder := []int{0, 1, 2, 3}
				for i, item := range ordered {
					if item.Controller != expectedOrder[i] {
						return &failure{
							CardName:    "APNAP_ActiveSeat0",
							Interaction: "apnap",
							Invariant:   "APNAPOrder",
							Message: fmt.Sprintf("position %d: expected controller %d, got %d",
								i, expectedOrder[i], item.Controller),
						}
					}
				}
				return nil
			},
		},
		{
			Name: "APNAP_SameController_MultiTriggers",
			Run: func() *failure {
				gs := makeBaseGameState()
				gs.Active = 0

				// Seat 0 has 3 triggers, seat 1 has 2.
				var triggers []*gameengine.StackItem
				for i := 0; i < 3; i++ {
					item := &gameengine.StackItem{
						Controller: 0,
						Source: &gameengine.Permanent{
							Card: &gameengine.Card{
								Name: fmt.Sprintf("Seat0_Trigger_%d", i), Owner: 0,
								Types: []string{"creature"},
							},
							Controller: 0, Owner: 0,
						},
						Kind: "triggered",
					}
					triggers = append(triggers, item)
				}
				for i := 0; i < 2; i++ {
					item := &gameengine.StackItem{
						Controller: 1,
						Source: &gameengine.Permanent{
							Card: &gameengine.Card{
								Name: fmt.Sprintf("Seat1_Trigger_%d", i), Owner: 1,
								Types: []string{"creature"},
							},
							Controller: 1, Owner: 1,
						},
						Kind: "triggered",
					}
					triggers = append(triggers, item)
				}

				ordered := gameengine.OrderTriggersAPNAP(gs, triggers)

				if len(ordered) != 5 {
					return &failure{
						CardName:    "APNAP_SameController_MultiTriggers",
						Interaction: "apnap",
						Invariant:   "TriggerCount",
						Message:     fmt.Sprintf("expected 5 triggers, got %d", len(ordered)),
					}
				}

				// First 3 should be seat 0's (active player pushes first).
				for i := 0; i < 3; i++ {
					if ordered[i].Controller != 0 {
						return &failure{
							CardName:    "APNAP_SameController_MultiTriggers",
							Interaction: "apnap",
							Invariant:   "APNAPGrouping",
							Message: fmt.Sprintf("position %d should be seat 0, got seat %d",
								i, ordered[i].Controller),
						}
					}
				}
				// Last 2 should be seat 1's.
				for i := 3; i < 5; i++ {
					if ordered[i].Controller != 1 {
						return &failure{
							CardName:    "APNAP_SameController_MultiTriggers",
							Interaction: "apnap",
							Invariant:   "APNAPGrouping",
							Message: fmt.Sprintf("position %d should be seat 1, got seat %d",
								i, ordered[i].Controller),
						}
					}
				}

				return nil
			},
		},
		{
			Name: "APNAP_SingleTrigger_NoReorder",
			Run: func() *failure {
				gs := makeBaseGameState()
				gs.Active = 0

				item := &gameengine.StackItem{
					Controller: 2,
					Source: &gameengine.Permanent{
						Card: &gameengine.Card{
							Name: "LoneTrigger", Owner: 2,
							Types: []string{"creature"},
						},
						Controller: 2, Owner: 2,
					},
					Kind: "triggered",
				}

				ordered := gameengine.OrderTriggersAPNAP(gs, []*gameengine.StackItem{item})

				if len(ordered) != 1 || ordered[0].Controller != 2 {
					return &failure{
						CardName:    "APNAP_SingleTrigger_NoReorder",
						Interaction: "apnap",
						Invariant:   "SingleTrigger",
						Message:     "single trigger should pass through unmodified",
					}
				}
				return nil
			},
		},
	}
}
