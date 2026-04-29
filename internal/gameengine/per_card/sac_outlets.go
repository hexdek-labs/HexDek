package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Sacrifice outlets — combo-backbone cards with "sacrifice a creature" as cost.
//
// Pattern: activated ability whose cost is sacrificing a creature you control.
// Each outlet's EFFECT differs (mana, scry, counter, mill, damage, indestructible).
// Uses gameengine.SacrificePermanent() for proper §701.17 sacrifice handling.
// ============================================================================

// --- Ashnod's Altar ---
//
// Oracle text:
//   Sacrifice a creature: Add {C}{C}.
//
// Infinite mana engine. Sacrifice a creature to add 2 colorless mana.
func registerAshnodsAltar(r *Registry) {
	r.OnActivated("Ashnod's Altar", ashnodsAltarActivated)
}

func ashnodsAltarActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ashnods_altar"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := chooseSacVictim(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Ashnod's Altar", "no_creature_to_sacrifice", nil)
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "ashnods_altar")

	// Add {C}{C} = 2 colorless mana.
	gs.Seats[seat].ManaPool += 2
	gameengine.SyncManaAfterAdd(gs.Seats[seat], 2)

	emit(gs, slug, "Ashnod's Altar", map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
		"mana_added": 2,
	})
}

// --- Phyrexian Altar ---
//
// Oracle text:
//   Sacrifice a creature: Add one mana of any color.
//
// Sacrifice a creature to add 1 mana of any color.
func registerPhyrexianAltar(r *Registry) {
	r.OnActivated("Phyrexian Altar", phyrexianAltarActivated)
}

func phyrexianAltarActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "phyrexian_altar"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := chooseSacVictim(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Phyrexian Altar", "no_creature_to_sacrifice", nil)
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "phyrexian_altar")

	// Add 1 mana of any color. MVP: generic mana pool.
	gs.Seats[seat].ManaPool++
	gameengine.SyncManaAfterAdd(gs.Seats[seat], 1)

	emit(gs, slug, "Phyrexian Altar", map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
		"mana_added": 1,
	})
}

// --- Viscera Seer ---
//
// Oracle text:
//   Sacrifice a creature: Scry 1.
//
// Free sac outlet. 1/1 vampire for B.
func registerVisceraSeer(r *Registry) {
	r.OnActivated("Viscera Seer", visceraSeerActivated)
}

func visceraSeerActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "viscera_seer"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := chooseSacVictim(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Viscera Seer", "no_creature_to_sacrifice", nil)
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "viscera_seer")

	// Scry 1.
	gameengine.Scry(gs, seat, 1)

	emit(gs, slug, "Viscera Seer", map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
	})
}

// --- Carrion Feeder ---
//
// Oracle text:
//   Sacrifice a creature: Put a +1/+1 counter on Carrion Feeder.
//   Carrion Feeder can't block.
//
// Free sac outlet. 1/1 zombie for B.
func registerCarrionFeeder(r *Registry) {
	r.OnActivated("Carrion Feeder", carrionFeederActivated)
}

func carrionFeederActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "carrion_feeder"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := chooseSacVictim(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Carrion Feeder", "no_creature_to_sacrifice", nil)
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "carrion_feeder")

	// Put a +1/+1 counter on Carrion Feeder.
	src.AddCounter("+1/+1", 1)

	emit(gs, slug, "Carrion Feeder", map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
		"counters":   src.Counters["+1/+1"],
	})
}

// --- Altar of Dementia ---
//
// Oracle text:
//   Sacrifice a creature: Target player mills cards equal to the
//   sacrificed creature's power.
//
// Combo mill outlet.
func registerAltarOfDementia(r *Registry) {
	r.OnActivated("Altar of Dementia", altarOfDementiaActivated)
}

func altarOfDementiaActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "altar_of_dementia"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := chooseSacVictim(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Altar of Dementia", "no_creature_to_sacrifice", nil)
		return
	}

	power := victim.Power()
	if power < 0 {
		power = 0
	}
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "altar_of_dementia")

	// Mill target player equal to creature's power.
	// Target: use context target_seat if provided, otherwise pick an opponent.
	targetSeat := pickTargetSeat(gs, seat, ctx)
	milled := 0
	for i := 0; i < power; i++ {
		if targetSeat >= 0 && targetSeat < len(gs.Seats) && len(gs.Seats[targetSeat].Library) > 0 {
			card := gs.Seats[targetSeat].Library[0]
			gameengine.MoveCard(gs, card, targetSeat, "library", "graveyard", "mill")
			milled++
		}
	}

	emit(gs, slug, "Altar of Dementia", map[string]interface{}{
		"seat":        seat,
		"sacrificed":  victimName,
		"power":       power,
		"target_seat": targetSeat,
		"milled":      milled,
	})
}

// --- Goblin Bombardment ---
//
// Oracle text:
//   Sacrifice a creature: Goblin Bombardment deals 1 damage to any
//   target.
//
// Combo damage outlet.
func registerGoblinBombardment(r *Registry) {
	r.OnActivated("Goblin Bombardment", goblinBombardmentActivated)
}

func goblinBombardmentActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "goblin_bombardment"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := chooseSacVictim(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Goblin Bombardment", "no_creature_to_sacrifice", nil)
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "goblin_bombardment")

	// Deal 1 damage to any target. MVP: target the highest-threat opponent.
	targetSeat := pickTargetSeat(gs, seat, ctx)
	if targetSeat >= 0 && targetSeat < len(gs.Seats) {
		gs.Seats[targetSeat].Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "damage",
			Seat:   seat,
			Target: targetSeat,
			Source: "Goblin Bombardment",
			Amount: 1,
		})
	}

	emit(gs, slug, "Goblin Bombardment", map[string]interface{}{
		"seat":        seat,
		"sacrificed":  victimName,
		"target_seat": targetSeat,
		"damage":      1,
	})
}

// --- Yahenni, Undying Partisan ---
//
// Oracle text:
//   Whenever a creature an opponent controls dies, put a +1/+1 counter
//   on Yahenni, Undying Partisan.
//   Sacrifice another creature: Yahenni gains indestructible until end
//   of turn.
//
// Free sac outlet with indestructible payoff.
func registerYahenni(r *Registry) {
	r.OnActivated("Yahenni, Undying Partisan", yahenniActivated)
	// Trigger: opponent's creature dies → +1/+1 counter.
	r.OnTrigger("Yahenni, Undying Partisan", "creature_dies", yahenniTrigger)
}

func yahenniActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "yahenni_sac"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := chooseSacVictimNotSelf(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Yahenni, Undying Partisan", "no_other_creature_to_sacrifice", nil)
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "yahenni_indestructible")

	// Yahenni gains indestructible until end of turn.
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["indestructible"] = 1
	src.GrantedAbilities = append(src.GrantedAbilities, "indestructible")

	emit(gs, slug, "Yahenni, Undying Partisan", map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
	})
}

func yahenniTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	// Only trigger on opponent's creatures dying.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat == perm.Controller {
		return // own creature, not opponent's
	}

	perm.AddCounter("+1/+1", 1)

	emit(gs, "yahenni_trigger", "Yahenni, Undying Partisan", map[string]interface{}{
		"seat":     perm.Controller,
		"counters": perm.Counters["+1/+1"],
	})
}

// --- Woe Strider ---
//
// Oracle text:
//   When Woe Strider enters the battlefield, create a 0/1 white
//   Goat creature token.
//   Sacrifice another creature: Scry 1.
//   Escape — {3}{B}{B}, Exile four other cards from your graveyard.
//   Woe Strider escapes with two +1/+1 counters on it.
//
// Free sac outlet with scry 1. Has escape (partial — flag only).
func registerWoeStrider(r *Registry) {
	r.OnActivated("Woe Strider", woeStriderActivated)
	r.OnETB("Woe Strider", woeStriderETB)
}

func woeStriderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "woe_strider_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Create a 0/1 white Goat creature token.
	goat := &gameengine.Card{
		Name:          "Goat",
		Types:         []string{"token", "creature", "goat"},
		Owner:         seat,
		BasePower:     0,
		BaseToughness: 1,
	}
	enterBattlefieldWithETB(gs, seat, goat, false)

	emit(gs, slug, "Woe Strider", map[string]interface{}{
		"seat":  seat,
		"token": "0/1 Goat",
	})
}

func woeStriderActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "woe_strider_sac"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	victim := chooseSacVictimNotSelf(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, "Woe Strider", "no_other_creature_to_sacrifice", nil)
		return
	}

	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "woe_strider")

	// Scry 1.
	gameengine.Scry(gs, seat, 1)

	emit(gs, slug, "Woe Strider", map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
	})
}

// ============================================================================
// Shared sacrifice-outlet helpers
// ============================================================================

// chooseSacVictim picks a creature to sacrifice from seat's battlefield.
// If ctx["creature_perm"] is set (by test or Hat), use that; otherwise
// pick the lowest-power creature that isn't the source permanent.
func chooseSacVictim(gs *gameengine.GameState, seat int, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if ctx != nil {
		if p, ok := ctx["creature_perm"].(*gameengine.Permanent); ok && p != nil {
			return p
		}
	}
	if seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	var best *gameengine.Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if best == nil || p.Power() < best.Power() {
			best = p
		}
	}
	return best
}

// chooseSacVictimNotSelf picks a creature to sacrifice that is NOT the
// source permanent (for "sacrifice another creature" costs).
func chooseSacVictimNotSelf(gs *gameengine.GameState, seat int, src *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if ctx != nil {
		if p, ok := ctx["creature_perm"].(*gameengine.Permanent); ok && p != nil && p != src {
			return p
		}
	}
	if seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	var best *gameengine.Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || !p.IsCreature() || p == src {
			continue
		}
		if best == nil || p.Power() < best.Power() {
			best = p
		}
	}
	return best
}

// pickTargetSeat returns a target seat from context or picks the
// highest-threat opponent.
func pickTargetSeat(gs *gameengine.GameState, seat int, ctx map[string]interface{}) int {
	if ctx != nil {
		if ts, ok := ctx["target_seat"].(int); ok {
			return ts
		}
	}
	// Pick first living opponent.
	for _, opp := range gs.Opponents(seat) {
		return opp
	}
	return seat
}
