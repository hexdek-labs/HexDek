package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// =============================================================================
// Ragost Combo Pieces — Crime Novelist, Nuka-Cola Vending Machine,
// Penregon Strongbull.
//
// These three cards form an infinite combo with Ragost (who makes all
// artifacts also Foods):
//
//   Strongbull: {1}, sac artifact -> +1/+1 until EOT, 1 damage to each opp
//   Crime Novelist: whenever you sacrifice an artifact -> +1/+1 counter, add {R}
//   Nuka-Cola: whenever you sacrifice a Food -> create tapped Treasure token
//
// With Ragost making artifacts into Foods, sacrificing a Treasure (artifact
// + Food) triggers both Crime Novelist (+1/+1, {R}) AND Nuka-Cola (new
// tapped Treasure), creating an infinite loop of damage via Strongbull.
// =============================================================================

// ---------------------------------------------------------------------------
// Crime Novelist
//
// Oracle text:
//   Whenever you sacrifice an artifact, put a +1/+1 counter on
//   Crime Novelist and add {R}.
//
// Triggers on the "artifact_sacrificed" event emitted by sacrificePermanentImpl.
// ---------------------------------------------------------------------------

func registerCrimeNovelist(r *Registry) {
	r.OnTrigger("Crime Novelist", "artifact_sacrificed", crimeNovelistTrigger)
}

func crimeNovelistTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	// Only trigger on YOUR artifacts being sacrificed.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	// +1/+1 counter on Crime Novelist.
	perm.AddCounter("+1/+1", 1)
	// Add {R}.
	seat := perm.Controller
	if seat >= 0 && seat < len(gs.Seats) {
		gameengine.AddMana(gs, gs.Seats[seat], "R", 1, "Crime Novelist")
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "triggered_ability",
		Seat:   seat,
		Source: "Crime Novelist",
		Details: map[string]interface{}{
			"counter": "+1/+1",
			"mana":    "R",
			"rule":    "603.3",
		},
	})
}

// ---------------------------------------------------------------------------
// Nuka-Cola Vending Machine
//
// Oracle text:
//   Whenever you sacrifice a Food, create a tapped Treasure token.
//
// Triggers on the "food_sacrificed" event emitted by sacrificePermanentImpl.
// ---------------------------------------------------------------------------

func registerNukaColaVendingMachine(r *Registry) {
	r.OnTrigger("Nuka-Cola Vending Machine", "food_sacrificed", nukaColaTrigger)
}

func nukaColaTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	// Only trigger on YOUR Foods being sacrificed.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	// Create a tapped Treasure token.
	seat := perm.Controller
	if seat >= 0 && seat < len(gs.Seats) {
		gameengine.CreateTreasureToken(gs, seat)
		// Mark the treasure as tapped (it enters tapped per oracle text).
		bf := gs.Seats[seat].Battlefield
		if len(bf) > 0 {
			lastPerm := bf[len(bf)-1]
			if lastPerm != nil {
				lastPerm.Tapped = true
			}
		}
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "triggered_ability",
		Seat:   seat,
		Source: "Nuka-Cola Vending Machine",
		Details: map[string]interface{}{
			"token":  "Treasure",
			"tapped": true,
			"rule":   "603.3",
		},
	})
}

// ---------------------------------------------------------------------------
// Penregon Strongbull
//
// Oracle text:
//   {1}, Sacrifice an artifact: Penregon Strongbull gets +1/+1 until
//   end of turn. It deals 1 damage to each opponent.
//
// Activated ability: pay {1} generic + sacrifice an artifact you control
// (not Strongbull itself).
// ---------------------------------------------------------------------------

func registerPenregonStrongbull(r *Registry) {
	r.OnActivated("Penregon Strongbull", strongbullActivated)
}

func strongbullActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Cost check: need at least {1} generic mana available.
	p := gameengine.EnsureTypedPool(s)
	if p == nil || p.Total() < 1 {
		// Also check the legacy ManaPool integer.
		if s.ManaPool < 1 {
			emitFail(gs, "strongbull", "Penregon Strongbull", "insufficient_mana", nil)
			return
		}
	}

	// Find an artifact to sacrifice (not Strongbull itself).
	// Allow context override for test/Hat control.
	var victim *gameengine.Permanent
	if ctx != nil {
		if v, ok := ctx["artifact_perm"].(*gameengine.Permanent); ok && v != nil && v != src {
			victim = v
		}
	}
	if victim == nil {
		for _, perm := range s.Battlefield {
			if perm == nil || perm == src || perm.Card == nil {
				continue
			}
			tl := strings.ToLower(strings.Join(perm.Card.Types, " "))
			if strings.Contains(tl, "artifact") {
				victim = perm
				break
			}
		}
	}
	if victim == nil {
		emitFail(gs, "strongbull", "Penregon Strongbull", "no_artifact_to_sacrifice", nil)
		return
	}

	// Pay {1} generic mana.
	if !gameengine.PayGenericCost(gs, s, 1, "activated", "strongbull", src.Card.DisplayName()) {
		// Fallback: deduct from legacy ManaPool.
		if s.ManaPool >= 1 {
			s.ManaPool--
			gameengine.SyncManaAfterSpend(s)
		} else {
			emitFail(gs, "strongbull", "Penregon Strongbull", "mana_payment_failed", nil)
			return
		}
	}

	// Sacrifice the artifact.
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "strongbull_activated")

	// +1/+1 until end of turn.
	src.Modifications = append(src.Modifications, gameengine.Modification{
		Power: 1, Toughness: 1, Duration: "until_end_of_turn",
	})

	// 1 damage to each opponent.
	for _, opp := range gs.LivingOpponents(seat) {
		gs.Seats[opp].Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   opp,
			Source: "Penregon Strongbull",
			Amount: -1,
			Details: map[string]interface{}{"cause": "strongbull"},
		})
	}

	emit(gs, "strongbull_activated", src.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
	})
}
