package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestSimultaneousDeaths_BothDiesEventsDispatched pins the engine's
// behavior for CR §700.4 + §704.3: "All applicable state-based actions
// are performed simultaneously as a single event." When a single SBA
// pass kills two creatures by different §704.5 clauses — one via
// §704.5g (lethal damage marked ≥ toughness) and one via §704.5f
// (toughness ≤ 0) — the engine must dispatch a "creature_dies" event
// for each death, not consolidate into one.
//
// This is a pin, not a fix: the engine currently fans out both deaths
// (each destroyPermSBA call invokes its own FireZoneChangeTriggers →
// FireCardTrigger("creature_dies", ctx)). The test guards against
// future regressions that would (a) bail the SBA helper loop early
// after the first death, (b) coalesce both creature_dies dispatches
// into a single call, or (c) drop the second-card dispatch because a
// shared bookkeeping flag was left dirty.
//
// Test mechanism: a single "Watcher" perm registers a creature_dies
// observer handler. Two distinct creatures die in the same SBA pass
// (one §704.5g, one §704.5f). The Watcher handler must be invoked
// exactly TWICE — once per dying creature.
func TestSimultaneousDeaths_BothDiesEventsDispatched(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	var observedDeaths []string
	Global().OnTrigger("DeathWatcher", "creature_dies",
		func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			if p, ok := ctx["perm"].(*gameengine.Permanent); ok && p != nil && p.Card != nil {
				observedDeaths = append(observedDeaths, p.Card.DisplayName())
			}
		})

	gs := newGame(t, 2)

	// Observer: stays on the battlefield through the SBA pass.
	watcher := addPerm(gs, 0, "DeathWatcher", "creature")
	watcher.Card.BasePower = 3
	watcher.Card.BaseToughness = 3

	// Lethal-damage dier: 2/2 with 2 marked damage → §704.5g destroys.
	lethal := addPerm(gs, 0, "LethalDamageDier", "creature")
	lethal.Card.BasePower = 2
	lethal.Card.BaseToughness = 2
	lethal.MarkedDamage = 2

	// Zero-toughness dier: base 1/0 → §704.5f destroys.
	zero := addPerm(gs, 0, "ZeroToughnessDier", "creature")
	zero.Card.BasePower = 1
	zero.Card.BaseToughness = 0

	gameengine.StateBasedActions(gs)

	// Both dead permanents must have left the battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == lethal {
			t.Fatalf("LethalDamageDier should have been destroyed by §704.5g and removed from battlefield")
		}
		if p == zero {
			t.Fatalf("ZeroToughnessDier should have been destroyed by §704.5f and removed from battlefield")
		}
	}

	// Observer must have been notified of BOTH deaths.
	if len(observedDeaths) != 2 {
		t.Fatalf("expected DeathWatcher to observe 2 dies events (one per dying creature), got %d: %v",
			len(observedDeaths), observedDeaths)
	}
	sawLethal := false
	sawZero := false
	for _, name := range observedDeaths {
		switch name {
		case "LethalDamageDier":
			sawLethal = true
		case "ZeroToughnessDier":
			sawZero = true
		}
	}
	if !sawLethal {
		t.Fatalf("DeathWatcher should have observed LethalDamageDier death, saw %v", observedDeaths)
	}
	if !sawZero {
		t.Fatalf("DeathWatcher should have observed ZeroToughnessDier death, saw %v", observedDeaths)
	}

	// Both must have produced a destroy event with the right SBA reason.
	sawLethalDestroy := false
	sawZeroDestroy := false
	for _, ev := range gs.EventLog {
		if ev.Kind != "destroy" {
			continue
		}
		reason, _ := ev.Details["reason"].(string)
		switch reason {
		case "lethal_damage":
			sawLethalDestroy = true
		case "toughness_zero_or_less":
			sawZeroDestroy = true
		}
	}
	if !sawLethalDestroy {
		t.Fatalf("expected destroy event with reason=lethal_damage")
	}
	if !sawZeroDestroy {
		t.Fatalf("expected destroy event with reason=toughness_zero_or_less")
	}
}

// TestSimultaneousDeaths_DiesCtxCarriesPreDeathState pins a corollary
// of §603.10: "An ability that triggers when an object that's on the
// battlefield leaves the battlefield uses the existence of that object
// PRIOR to the event." The ctx["perm"] passed to a creature_dies
// observer handler must carry the dying perm's controller / card
// identity, even though by handler-invocation time gs.Seats[*]
// .Battlefield has already had the perm removed.
//
// Without this guarantee, "whenever a creature you control dies"
// triggers (Blood Artist, Cruel Celebrant, Grave Pact) can't tell
// whose creature died.
func TestSimultaneousDeaths_DiesCtxCarriesPreDeathState(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	var observedController = -1
	var observedName string
	Global().OnTrigger("CtxObserver", "creature_dies",
		func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			if p, ok := ctx["perm"].(*gameengine.Permanent); ok && p != nil {
				observedController = p.Controller
				if p.Card != nil {
					observedName = p.Card.DisplayName()
				}
			}
		})

	gs := newGame(t, 3)

	// Observer on seat 0 (independent of the dier's seat).
	observer := addPerm(gs, 0, "CtxObserver", "creature")
	observer.Card.BasePower = 1
	observer.Card.BaseToughness = 1

	// Dier on seat 1 (so we can prove controller wasn't defaulted to 0).
	dier := addPerm(gs, 1, "OtherDier", "creature")
	dier.Card.BasePower = 1
	dier.Card.BaseToughness = 1
	dier.MarkedDamage = 1 // §704.5g

	gameengine.StateBasedActions(gs)

	if observedController != 1 {
		t.Fatalf("observer should see dier's pre-death Controller=1, got %d", observedController)
	}
	if observedName != "OtherDier" {
		t.Fatalf("observer should see dying card identity preserved, got %q", observedName)
	}
}
