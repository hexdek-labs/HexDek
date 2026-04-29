package gameengine

// Wave 3c — APNAP trigger ordering (CR §603.3b).
//
// When multiple triggered abilities fire simultaneously (e.g. two ETBs
// from a mass-blink, or three cast-triggers from different seats), the
// rules specify a precise ordering for putting them on the stack:
//
//   CR §603.3b: "If multiple abilities have triggered since the last time
//   a player received priority, each player, in APNAP order, puts triggered
//   abilities he or she controls on the stack in any order he or she
//   chooses."
//
// APNAP = Active Player, then Non-Active Player(s) in turn order.
//
// The consequence is that the active player's triggers go on the stack
// FIRST (and thus resolve LAST because the stack is LIFO), and the last
// non-active player's triggers go on top (resolving first).
//
// Within each controller's set, the controller chooses the order.
// We delegate that choice to Hat.OrderTriggers.
//
// This file provides:
//
//   - OrderTriggersAPNAP(gs, triggers) []*StackItem
//     Groups by controller, sorts groups in APNAP order, delegates
//     intra-group ordering to each seat's Hat, returns the final push
//     order (first element = pushed first = resolves last).
//
//   - PushSimultaneousTriggers(gs, triggers)
//     Orders triggers via APNAP, pushes them onto the stack, and opens
//     a priority round.

// OrderTriggersAPNAP takes a set of simultaneously-triggered abilities
// and returns them in the correct push order per CR §603.3b:
//
//  1. Group triggers by controller.
//  2. Walk seats in APNAP order (active player first).
//  3. For each seat, ask Hat.OrderTriggers for the intra-group order.
//  4. Concatenate: AP's triggers first, then NAP(s) in turn order.
//
// The returned order is the PUSH order: element [0] is pushed onto the
// stack first and therefore resolves LAST (LIFO). This means the last
// player in APNAP order has their triggers on top, resolving first —
// which is the correct CR behavior.
func OrderTriggersAPNAP(gs *GameState, triggers []*StackItem) []*StackItem {
	if gs == nil || len(triggers) <= 1 {
		return triggers
	}

	// Group by controller.
	groups := make(map[int][]*StackItem)
	for _, t := range triggers {
		if t == nil {
			continue
		}
		groups[t.Controller] = append(groups[t.Controller], t)
	}

	// Walk in APNAP order.
	order := apnapOrder(gs)
	result := make([]*StackItem, 0, len(triggers))

	for _, seat := range order {
		grp, ok := groups[seat]
		if !ok || len(grp) == 0 {
			continue
		}

		// Delegate intra-group ordering to the seat's Hat.
		if seat >= 0 && seat < len(gs.Seats) && gs.Seats[seat] != nil && gs.Seats[seat].Hat != nil {
			grp = gs.Seats[seat].Hat.OrderTriggers(gs, seat, grp)
		}

		result = append(result, grp...)
	}

	return result
}

// PushSimultaneousTriggers orders a batch of simultaneously-triggered
// abilities per CR §603.3b (APNAP + controller choice) and pushes them
// onto the stack. A priority round opens after all triggers are pushed.
//
// This replaces the pattern of pushing triggers one-by-one in arrival
// order. Callers should collect all simultaneously-firing triggers into
// a slice, then call this once.
func PushSimultaneousTriggers(gs *GameState, triggers []*StackItem) {
	if gs == nil || len(triggers) == 0 {
		return
	}

	ordered := OrderTriggersAPNAP(gs, triggers)
	for _, item := range ordered {
		if item == nil {
			continue
		}
		if item.Kind == "" {
			item.Kind = "triggered"
		}
		PushStackItem(gs, item)
	}

	gs.LogEvent(Event{
		Kind: "triggers_ordered",
		Seat: gs.Active,
		Details: map[string]interface{}{
			"count": len(ordered),
			"rule":  "603.3b",
		},
	})
}
