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

// maxPerCardInlineResolveDepth bounds how many PushPerCardTrigger frames
// may nest on the Go call stack at once. The inline resolve below
// (PriorityRound + ResolveStackTop) re-enters StateBasedActions, and an
// SBA↔trigger feedback loop (e.g. §704.5y role-uniqueness destroying a
// Role whose death trigger re-grants a Role) nests a fresh
// PushPerCardTrigger frame per cycle. The sibling guards bound the wrong
// dimension for that failure: maxTriggerFiresPerTurn (stack.go:67) caps
// the COUNT of fires at 1000, but 1000 NESTED cycles overflow the
// goroutine stack first — a Go stack overflow is fatal and not
// recover()-able, so one degenerate game killed the whole server
// (the sba704_5y / sba704_5g / sba704_5j DARKSTAR crash family).
// Legitimate nesting is shallow — fireTrigger's own dispatch-depth cap is
// 8 — so 25 is comfortably above real play and far below stack
// exhaustion. Past the bound the game is aborted as a draw via the same
// machinery as trigger_loop_cap below: never the process.
const maxPerCardInlineResolveDepth = 25

// PushPerCardTrigger pushes a per-card trigger handler onto the stack as
// a proper triggered ability per CR section 603.3, then opens a priority
// round and resolves (mirroring PushTriggeredAbility's auto-resolve
// pattern so that triggers still fire inline for callers that expect
// immediate resolution).
func PushPerCardTrigger(gs *GameState, perm *Permanent, handler func(*GameState, *Permanent, map[string]interface{}), ctx map[string]interface{}) {
	if gs == nil || perm == nil || handler == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["ended"] == 1 {
		return
	}
	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}

	// Recursion-depth guard: count nested PushPerCardTrigger frames.
	// All SBA↔trigger re-entrancy funnels through the inline resolve at
	// the bottom of this function, so frames of this function currently
	// on the call stack measure that recursion exactly. Increment-with-
	// defer keeps sequential sibling triggers (fireTrigger's handler
	// loop) at the same depth; only nesting through ResolveStackTop
	// deepens it.
	gs.Flags["_percard_inline_depth"]++
	defer func() { gs.Flags["_percard_inline_depth"]-- }()
	if gs.Flags["_percard_inline_depth"] > maxPerCardInlineResolveDepth {
		LogLoopGuardFired(gs, "percard_inline_depth_cap", map[string]interface{}{
			"depth": gs.Flags["_percard_inline_depth"], "cap": maxPerCardInlineResolveDepth,
			"card":  cardName,
		})
		gs.LogEvent(Event{
			Kind:   "loop_anomaly",
			Seat:   perm.Controller,
			Source: cardName,
			Details: map[string]interface{}{
				"reason": "percard_inline_depth_cap",
				"depth":  gs.Flags["_percard_inline_depth"],
				"cap":    maxPerCardInlineResolveDepth,
				"rule":   "603.3",
			},
		})
		for i, s := range gs.Seats {
			if s != nil && !s.Lost && !s.Won {
				s.Lost = true
				gs.LogEvent(Event{Kind: "game_draw", Seat: i, Details: map[string]interface{}{"reason": "percard_inline_depth_cap"}})
			}
		}
		gs.Stack = gs.Stack[:0]
		gs.Flags["ended"] = 1
		return
	}

	gs.Flags["_trigger_fires_this_turn"]++
	if gs.Flags["_trigger_fires_this_turn"] > triggerCapForGame(gs) {
		LogLoopGuardFired(gs, "trigger_loop_cap", map[string]interface{}{
			"fires": gs.Flags["_trigger_fires_this_turn"], "site": "per_card_trigger",
			"card":  cardName,
		})
		for i, s := range gs.Seats {
			if s != nil && !s.Lost && !s.Won {
				s.Lost = true
				gs.LogEvent(Event{Kind: "game_draw", Seat: i, Details: map[string]interface{}{"reason": "trigger_loop_cap"}})
			}
		}
		gs.Stack = gs.Stack[:0]
		gs.Flags["ended"] = 1
		return
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
