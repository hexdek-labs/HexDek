package gameengine

// Counter-accumulator threshold primitive — R58.
//
// Many cards have the shape "this permanent accumulates counters via
// some trigger or activation, and at the beginning of your upkeep, if
// it has N or more counters of that type on it, [effect]." The
// canonical examples are alt-win conditions:
//
//   - Helix Pinnacle    — tower counters; 100+  → you win the game
//   - Darksteel Reactor — charge counters; 20+  → you win the game
//   - Azor's Elocutors  — filibuster counters; 5+ → you win the game
//
// And recurring-effect variants:
//
//   - Quest for Ula's Temple — quest counters; 3+ → each upkeep,
//                              put a Kraken/Leviathan/Octopus/Serpent
//                              card from hand onto the battlefield.
//
// The shared shape — "evaluate the source's counter count at the
// controller's upkeep, fire OnReach if >= threshold" — lives here
// as a registry primitive. Per-card handlers register on ETB,
// unregister on LTB, and call EvaluateCounterThresholds from their
// upkeep_controller trigger.
//
// CR §117.5  General check — "an effect of this type is checked every
// time the game state changes." The HexDek model evaluates at upkeep
// for the alt-win cycle (matches the printed trigger timing) — true
// continuous evaluation is engine territory.

// CounterThresholdEffect is one registered "counter threshold → effect"
// rule keyed to a source permanent.
type CounterThresholdEffect struct {
	// SourcePerm is the permanent carrying the counters.
	// UnregisterCounterThresholdsForPermanent matches on this pointer.
	SourcePerm *Permanent

	// HandlerID — diagnostic tag.
	HandlerID string

	// Counter is the counter name to count on SourcePerm (e.g.
	// "tower", "charge", "filibuster", "quest").
	Counter string

	// Threshold is the count required to trigger OnReach.
	Threshold int

	// OnReach runs when SourcePerm.Counters[Counter] >= Threshold at
	// evaluation time. For one-shot effects (alt-win conditions),
	// keep Repeatable = false so it fires once and never again. For
	// recurring effects (Quest for Ula's Temple — fires every upkeep
	// while at threshold), set Repeatable = true.
	OnReach func(*GameState, *Permanent)

	// Repeatable — if true, OnReach fires at every evaluation while
	// the threshold is met. If false (default), OnReach fires once
	// and Fired is flipped to true to prevent re-firing.
	Repeatable bool

	// Fired records whether OnReach has fired for a one-shot
	// (Repeatable=false) effect. Internal — callers shouldn't touch.
	Fired bool
}

// RegisterCounterThreshold appends a threshold effect to the registry.
func (gs *GameState) RegisterCounterThreshold(eff *CounterThresholdEffect) {
	if gs == nil || eff == nil || eff.OnReach == nil || eff.Counter == "" {
		return
	}
	gs.CounterThresholds = append(gs.CounterThresholds, eff)
	gs.LogEvent(Event{
		Kind:   "counter_threshold_registered",
		Source: eff.HandlerID,
		Details: map[string]interface{}{
			"counter":    eff.Counter,
			"threshold":  eff.Threshold,
			"repeatable": eff.Repeatable,
		},
	})
}

// UnregisterCounterThresholdsForPermanent drops every effect whose
// SourcePerm == p. Called from LTB hooks.
func (gs *GameState) UnregisterCounterThresholdsForPermanent(p *Permanent) {
	if gs == nil || p == nil || len(gs.CounterThresholds) == 0 {
		return
	}
	out := gs.CounterThresholds[:0]
	for _, eff := range gs.CounterThresholds {
		if eff != nil && eff.SourcePerm == p {
			continue
		}
		out = append(out, eff)
	}
	for i := len(out); i < len(gs.CounterThresholds); i++ {
		gs.CounterThresholds[i] = nil
	}
	gs.CounterThresholds = out
}

// EvaluateCounterThresholds walks the registry and fires OnReach for
// any effect whose source has Counters[Counter] >= Threshold.
//
//   - One-shot (Repeatable=false): fires once, sets Fired=true, never
//     fires again even if the count drops below threshold and climbs
//     back. (Win conditions don't unwin.)
//   - Recurring (Repeatable=true): fires every time the source meets
//     the threshold at evaluation. The handler is responsible for any
//     per-turn / per-evaluation rate limiting (Quest for Ula's Temple
//     gates on upkeep_controller and self-resets the quest counter
//     payout policy each fire as appropriate to the printed text).
//
// Callers (per_card handlers) typically invoke this from an
// upkeep_controller trigger so the timing matches the printed
// "At the beginning of your upkeep" clause.
func (gs *GameState) EvaluateCounterThresholds() {
	if gs == nil {
		return
	}
	for _, eff := range gs.CounterThresholds {
		if eff == nil || eff.SourcePerm == nil {
			continue
		}
		if !eff.Repeatable && eff.Fired {
			continue
		}
		if eff.SourcePerm.Counters == nil {
			continue
		}
		count := eff.SourcePerm.Counters[eff.Counter]
		if count < eff.Threshold {
			continue
		}
		eff.OnReach(gs, eff.SourcePerm)
		if !eff.Repeatable {
			eff.Fired = true
		}
		gs.LogEvent(Event{
			Kind:   "counter_threshold_fired",
			Source: eff.HandlerID,
			Details: map[string]interface{}{
				"counter":    eff.Counter,
				"threshold":  eff.Threshold,
				"count":      count,
				"repeatable": eff.Repeatable,
			},
		})
	}
}

// CounterThresholdCount is a thin accessor that returns the count for
// SourcePerm.Counters[Counter] safely. Useful for per_card handlers
// that need to display the current accumulation in their emit calls
// or to gate other behavior on the count (e.g. Helix Pinnacle's
// shroud-while-under-100 rider — separate from the threshold).
func CounterThresholdCount(perm *Permanent, counter string) int {
	if perm == nil || perm.Counters == nil {
		return 0
	}
	return perm.Counters[counter]
}
