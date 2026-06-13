package gameengine

// CR §603.3b batched simultaneous-trigger placement + §608.2c
// during-resolution deferral.
//
// R38 (commit 03d7450) introduced re-entrant batch frames so that a single
// game event's worth of triggered abilities are grouped, ordered APNAP +
// controller-choice via OrderTriggersAPNAP, then pushed together via
// PushSimultaneousTriggers. R39 extends that work with CR §608.2c:
// "If an ability's effect causes an event to happen during the resolution
// of an ability, any triggered abilities that trigger as a result of that
// event are placed on the stack after the resolving ability finishes
// resolving." In other words: triggers that fire DURING resolution must
// not resolve until the outer spell/ability has finished. They wait in
// the pending queue and the outer ResolveStackTop drains them on the way
// out.
//
// Concretely:
//
//   - BeginTriggerBatch increments gs.triggerBatchDepth and returns whether
//     this caller opened the outermost frame.
//   - While depth > 0, PushTriggeredAbility appends to gs.pendingTriggers
//     instead of push+resolve.
//   - EndTriggerBatch on the outermost frame normally orders, pushes, and
//     drains the batch. But if we're inside a ResolveStackTop frame
//     (gs.Flags["_resolve_frame_depth"] > 0), the drain is deferred — the
//     pending queue is left intact for the outer ResolveStackTop's defer
//     to flush per §608.2c.
//   - ResolveStackTop bumps _resolve_frame_depth on entry and on its way
//     out (depth back to 0) calls drainPendingTriggers if any siblings
//     accumulated during the resolution.
//
// Nested fire sites still observe an open batch and append into the same
// pending slice; the outermost batch frame OR the outermost resolve frame
// (whichever closes last) drives the drain.
//
// Standard idiom at fire sites:
//
//   defer EndTriggerBatch(gs, BeginTriggerBatch(gs))
//
// Single-trigger callers that invoke PushTriggeredAbility directly continue
// to work: outside both a batch and a resolve frame, the legacy push+resolve
// path is unchanged. Inside a resolve frame the single trigger is deferred
// to maintain §608.2c semantics.

// BeginTriggerBatch opens a §603.3b batch frame. Returns true if this call
// opened the outermost frame (the caller is responsible for the matching
// EndTriggerBatch with opened=true); returns false if a batch was already
// active (the caller still must call EndTriggerBatch with the same value).
//
// Safe with nil gs (returns false).
func BeginTriggerBatch(gs *GameState) (opened bool) {
	if gs == nil {
		return false
	}
	gs.triggerBatchDepth++
	return gs.triggerBatchDepth == 1
}

// EndTriggerBatch closes a §603.3b batch frame. When opened==true (the
// outermost frame), it normally orders the collected triggers per APNAP +
// controller-choice, pushes them via PushSimultaneousTriggers, then drains
// the stack through priority/resolve cycles until those items are gone.
//
// CR §608.2c exception: if we are currently inside a ResolveStackTop frame
// (some outer spell or ability is mid-resolution), the drain is deferred.
// The pending triggers remain queued and the outer ResolveStackTop's
// outermost-frame defer flushes them once that resolution finishes.
//
// Pair with BeginTriggerBatch via defer:
//
//	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))
func EndTriggerBatch(gs *GameState, opened bool) {
	if gs == nil {
		return
	}
	if gs.triggerBatchDepth > 0 {
		gs.triggerBatchDepth--
	}
	if !opened {
		return
	}
	if len(gs.pendingTriggers) == 0 {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["ended"] == 1 {
		gs.pendingTriggers = nil
		return
	}

	// CR §608.2c: triggers that fired during the resolution of an outer
	// spell or ability go on the stack AFTER that resolution completes.
	// If we're inside a ResolveStackTop frame, leave the pending queue
	// alone — the outer frame's defer (see ResolveStackTop in stack.go)
	// will drain it when the resolution returns.
	if gs.Flags["_resolve_frame_depth"] > 0 {
		return
	}

	drainPendingTriggers(gs)
}

// drainPendingTriggers orders, pushes, and resolves any triggers sitting
// in gs.pendingTriggers. Called from two sites:
//
//   - EndTriggerBatch, when the outermost batch closes outside any
//     resolution frame (normal case).
//   - ResolveStackTop's outermost-frame defer, after an outer spell or
//     ability finishes resolving (the §608.2c case).
//
// The drain itself runs ResolveStackTop in a loop; if those resolutions
// fire further triggers they collect into pendingTriggers again, and the
// outer-loop check picks them up. resolve_depth gates infinite recursion
// via maxResolveDepth, matching the legacy PushTriggeredAbility pattern.
func drainPendingTriggers(gs *GameState) {
	if gs == nil || len(gs.pendingTriggers) == 0 {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["ended"] == 1 {
		gs.pendingTriggers = nil
		return
	}

	gs.Flags["resolve_depth"]++
	defer func() { gs.Flags["resolve_depth"]-- }()
	if gs.Flags["resolve_depth"] > maxResolveDepth {
		return
	}

	// Drain loop. New triggers may accumulate during resolution; we keep
	// pulling batches off pendingTriggers until it stays empty.
	//
	// LIVENESS guard (r63, game-216 stall): an unbounded cascade can spin
	// here forever — each resolution re-fills pendingTriggers and/or pushes
	// more onto the stack (e.g. a trigger resolves a permanent whose ETB
	// processing — resolvePermanentSpellETB → ApplyRiot, token/copy creation,
	// further ETB triggers — produces the next resolution). That work is NOT a
	// counted "trigger fire", so the per-turn trigger_loop_cap (stack.go) never
	// trips, and the live grinder abandoned such a game at the 10-minute
	// wall-clock budget. The spin can be in EITHER loop: the outer batch-pull
	// OR the inner stack-drain within a single batch. So cap TOTAL resolutions
	// across both loops (one counter, incremented per ResolveStackTop) the same
	// way DrainStack caps maxStackDrainIterations — a finite bound far above any
	// legitimate turn. On trip, emit the uniform loop-guard event, drop the
	// runaway stack + pending queue, and return so the turn (and game) ends.
	resolves := 0
	for len(gs.pendingTriggers) > 0 {
		batch := gs.pendingTriggers
		gs.pendingTriggers = nil

		stackBefore := len(gs.Stack)
		PushSimultaneousTriggers(gs, batch)
		if len(gs.Stack) <= stackBefore {
			continue
		}

		for len(gs.Stack) > stackBefore {
			if gs.Flags["ended"] == 1 {
				return
			}
			resolves++
			if resolves > maxTriggerDrainIterations {
				LogLoopGuardFired(gs, "trigger_drain_cap", map[string]interface{}{
					"resolves": resolves - 1,
					"cap":      maxTriggerDrainIterations,
					"stack":    len(gs.Stack),
					"pending":  len(gs.pendingTriggers),
				})
				// CR §104.4b — the cascade re-fills the stack/queue every
				// resolution and won't terminate (an unbroken spell-copy loop:
				// game-216 was Veyran, Voice of Duality doubling magecraft/copy
				// triggers so each copy spawned more). End the game as a draw,
				// mirroring the per-turn trigger_loop_cap.
				gs.pendingTriggers = nil
				EndGameAsLoopDraw(gs, "trigger_drain_cap")
				return
			}
			PriorityRound(gs)
			if len(gs.Stack) <= stackBefore {
				break
			}
			ResolveStackTop(gs)
		}
	}
}

// collectTrigger is the engine-internal entry point used by
// PushTriggeredAbility when a batch is open OR we are inside an outer
// ResolveStackTop frame. It appends to the pending slice without touching
// the stack or opening priority.
func collectTrigger(gs *GameState, item *StackItem) {
	if gs == nil || item == nil {
		return
	}
	if item.Kind == "" {
		item.Kind = "triggered"
	}
	gs.pendingTriggers = append(gs.pendingTriggers, item)
	name := ""
	if item.Card != nil {
		name = item.Card.DisplayName()
	}
	GlobalStackTrace.Log("trigger_collect", name, item.Controller, len(gs.pendingTriggers), "603.3b_pending")
}
