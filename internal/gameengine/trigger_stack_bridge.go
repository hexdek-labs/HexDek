package gameengine

// trigger_stack_bridge.go — Bridge between per-card Go-function trigger
// handlers and the stack system.
//
// Per CR section 603.3, triggered abilities must go on the stack so that
// players have priority to respond (Stifle, Disallow, Angel's Grace,
// etc.). Per-card handlers are Go functions, not gameast.Effect values,
// so they cannot use PushTriggeredAbility directly. This file provides
// the wrapper types and push function that let Go-function handlers
// participate in the stack system.

// TriggerHandlerStackItem represents a per-card triggered ability that
// was pushed to the stack instead of resolving immediately. When the
// stack resolves this item, it calls the handler function.
type TriggerHandlerStackItem struct {
	HandlerFunc func(gs *GameState, perm *Permanent, ctx map[string]interface{})
	SourcePerm  *Permanent
	Ctx         map[string]interface{}
}

// PushPerCardTrigger pushes a per-card trigger handler onto the stack as
// a proper triggered ability per CR section 603.3, then opens a priority
// round and resolves (mirroring PushTriggeredAbility's auto-resolve
// pattern so that triggers still fire inline for callers that expect
// immediate resolution).
func PushPerCardTrigger(gs *GameState, perm *Permanent, handler func(*GameState, *Permanent, map[string]interface{}), ctx map[string]interface{}) {
	if gs == nil || perm == nil || handler == nil {
		return
	}
	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}

	item := &StackItem{
		Card:       perm.Card,
		Controller: perm.Controller,
		Kind:       "triggered",
		Source:     perm,
		CostMeta: map[string]interface{}{
			"per_card_trigger": true,
			"trigger_handler": &TriggerHandlerStackItem{
				HandlerFunc: handler,
				SourcePerm:  perm,
				Ctx:         ctx,
			},
		},
	}

	// Stack trace: log triggered ability push for CR audit.
	GlobalStackTrace.Log("trigger_push", cardName, perm.Controller, len(gs.Stack), "per_card_trigger")

	PushStackItem(gs, item)

	gs.LogEvent(Event{
		Kind:   "triggered_ability",
		Seat:   perm.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"rule": "603.3",
			"via":  "per_card_trigger",
		},
	})

	// Per CR section 117.3a priority opens on triggers — open a priority
	// round then resolve. This matches PushTriggeredAbility's inline
	// resolve pattern so callers that fire triggers mid-SBA or mid-
	// zone-change see effects resolve before control returns.
	PriorityRound(gs)
	if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == item {
		ResolveStackTop(gs)
	}
}
