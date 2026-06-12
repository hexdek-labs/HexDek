package per_card

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// isSorcerySpeed reports whether `seatIdx` could legally take a
// sorcery-speed action right now: they're the active player, the stack
// is empty, and the phase is one of their two main phases. Mirrors the
// minimal subset CR §307.1 / §608 expect; per-card handlers use it to
// gate "Activate only as a sorcery" abilities defensively when callers
// reach them through paths that skip the engine's stack-time check
// (test fixtures, AI evaluator probes, replay rebuilds).
func isSorcerySpeed(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	if gs.Active != seatIdx {
		return false
	}
	if len(gs.Stack) > 0 {
		return false
	}
	switch gs.Phase {
	case "precombat_main", "postcombat_main", "main", "main1", "main2":
		return true
	}
	return false
}

// payManaFromPool deducts `amount` from the seat's mana pool if it can
// cover, returning true. Returns false (and does NOT decrement) when
// the pool is short. Use as a per-handler defensive cost gate when the
// engine's activated-ability dispatcher might be skipped (test paths,
// AI evaluator, replay rebuild).
func payManaFromPool(seat *gameengine.Seat, amount int) bool {
	if seat == nil {
		return false
	}
	if amount <= 0 {
		return true
	}
	if seat.ManaPool < amount {
		return false
	}
	seat.ManaPool -= amount
	// Keep the typed pool in lockstep with the scalar. Without this every
	// defensive payment left the typed pool rich by `amount`, and the NEXT
	// payment's SyncManaAfterSpend flushed the drift — the legality
	// validator then flagged that innocent action as over-paying (judge
	// round-5: Mageta the Lion announced 4 / measured 8, where the extra
	// 4 was Commander Mustard's earlier unsynced defensive payment).
	gameengine.SyncManaAfterSpend(seat)
	return true
}

// payDefensiveManaCost is the dispatcher-aware defensive cost gate.
// ActivateAbility already pays Cost.Mana whenever the AST Activated node
// at abilityIdx carries one (activation.go, step 2) — a handler that
// then ALSO pays via payManaFromPool double-charges the ability (judge
// round-5 static audit: Mayael {6}+{6}, Commander Mustard {4}+{4},
// Bristly Bill {5}+{5}, Ezrim {1}+{1}), and at exact budgets the second
// charge FAILS, silently no-opping an ability the player already paid
// for. This helper pays only when the dispatcher could not have: when
// the AST node at abilityIdx is missing or carries no mana cost (the
// per_card-only activation shape the defensive gate was written for).
func payDefensiveManaCost(src *gameengine.Permanent, seat *gameengine.Seat, abilityIdx, amount int) bool {
	if src != nil && src.Card != nil && src.Card.AST != nil &&
		abilityIdx >= 0 && abilityIdx < len(src.Card.AST.Abilities) {
		if act, ok := src.Card.AST.Abilities[abilityIdx].(*gameast.Activated); ok &&
			act.Cost.Mana != nil && act.Cost.Mana.CMC() > 0 {
			return true // dispatcher charged this cost at activation time
		}
	}
	return payManaFromPool(seat, amount)
}

// dispatcherHandledActivationCosts reports whether ActivateAbility
// processed this ability's Cost struct (it does whenever an AST
// Activated node exists at abilityIdx — paying Cost.Mana AND applying
// Cost.Tap before dispatching hooks). Handlers with defensive tap
// gates must skip them in that case: the dispatcher has already tapped
// the source, so a handler-side "if src.Tapped { abort }" guard makes
// the ability permanently dead through the live path (judge round-5:
// Mayael the Anima's look-five never ran via the dispatcher — it
// aborted as already_tapped on every legitimate activation).
func dispatcherHandledActivationCosts(src *gameengine.Permanent, abilityIdx int) bool {
	if src == nil || src.Card == nil || src.Card.AST == nil ||
		abilityIdx < 0 || abilityIdx >= len(src.Card.AST.Abilities) {
		return false
	}
	_, ok := src.Card.AST.Abilities[abilityIdx].(*gameast.Activated)
	return ok
}
