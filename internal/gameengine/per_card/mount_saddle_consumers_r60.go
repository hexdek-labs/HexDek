package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// mount_saddle_consumers_r60.go — per_card OnTrigger handlers for the
// OTJ Mount/Saddle (CR §702.171) payoff family.
//
// Per the Versailles Phase 1B audit (PR #477 §3) `mount_saddled` is one
// of the 29 engine-emitted events with no per_card consumer wired
// despite ~10 mount payoff cards in the AST corpus. This file wires
// 7 handlers across two trigger shapes:
//
//   1) "Whenever this creature becomes saddled (for the first time)
//      each turn, ..." — listens on the `mount_saddled` event fired by
//      FireMountSaddledTriggers in keywords_mount.go:95. Ctx keys:
//      "mount" (*Permanent), "controller" (int).
//
//   2) "Whenever this creature attacks while saddled, ..." — listens
//      on the `attack` event (alias of `creature_attacks` fired by
//      combat.go:691). Ctx keys: "attacker_perm" (*Permanent),
//      "attacker_seat" (int), "attacker_card" (*Card). Handler gates
//      on `attacker_perm == perm` (this creature, not an ally) AND
//      `PermIsSaddled(perm)` (the §702.171 status condition).

func init() {
	registerMountSaddleConsumersR60(Global())
	AddResetHook(registerMountSaddleConsumersR60)
}

func registerMountSaddleConsumersR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Stubborn Burrowfiend", "mount_saddled", stubbornBurrowfiendBecomesSaddled)
	r.OnTrigger("Alacrian Jaguar", "attack", alacrianJaguarAttack)
	r.OnTrigger("Brightfield Mustang", "attack", brightfieldMustangAttack)
	r.OnTrigger("Gilded Ghoda", "attack", gildedGhodaAttack)
	r.OnTrigger("Venomsac Lagac", "attack", venomsacLagacAttack)
	r.OnTrigger("Bridled Bighorn", "attack", bridledBighornAttack)
	r.OnTrigger("District Mascot", "attack", districtMascotAttack)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// attacksWhileSaddledGuard returns the bearer permanent if the trigger
// fires for the bearer's own attack AND the bearer is currently
// saddled. Returns nil when the trigger is ambient (another creature
// attacking) OR the bearer isn't saddled this combat. Centralising the
// shape keeps each per-card body focused on the payoff.
func attacksWhileSaddledGuard(perm *gameengine.Permanent, ctx map[string]interface{}) *gameengine.Permanent {
	if perm == nil || perm.Card == nil || ctx == nil {
		return nil
	}
	attacker, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attacker != perm {
		return nil
	}
	if !gameengine.PermIsSaddled(perm) {
		return nil
	}
	return perm
}

// -----------------------------------------------------------------------------
// Stubborn Burrowfiend — {2}{B} Creature — Badger Beast Mount, Saddle 2
//
// Whenever this creature becomes saddled for the first time each turn,
// mill two cards, then this creature gets +X/+X until end of turn,
// where X is the number of creature cards in your graveyard.
// -----------------------------------------------------------------------------

func stubbornBurrowfiendBecomesSaddled(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "stubborn_burrowfiend_mill_pump"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	mount, _ := ctx["mount"].(*gameengine.Permanent)
	if mount != perm {
		return
	}
	// "for the first time each turn" — gate on turn-stamped flag.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["burrowfiend_saddle_turn"] == gs.Turn+1 {
		// Already fired this turn (we store Turn+1 so the zero value
		// can't accidentally match Turn 0).
		return
	}
	perm.Flags["burrowfiend_saddle_turn"] = gs.Turn + 1

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Mill 2.
	milled := 0
	for i := 0; i < 2 && len(seat.Library) > 0; i++ {
		top := seat.Library[0]
		gameengine.MoveCard(gs, top, perm.Controller, "library", "graveyard", "stubborn_burrowfiend_mill")
		milled++
	}
	// X = creature cards in your graveyard.
	x := 0
	for _, c := range seat.Graveyard {
		if c != nil && cardHasType(c, "creature") {
			x++
		}
	}
	if x > 0 {
		ts := gs.NextTimestamp()
		perm.Modifications = append(perm.Modifications, gameengine.Modification{
			Power:     x,
			Toughness: x,
			Duration:  "until_end_of_turn",
			Timestamp: ts,
		})
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"milled": milled,
		"x":      x,
	})
}

// -----------------------------------------------------------------------------
// Alacrian Jaguar — {4}{G} Creature — Cat Mount, Saddle 1
//
// Whenever this creature attacks while saddled, it gets +2/+2 until end of turn.
// -----------------------------------------------------------------------------

func alacrianJaguarAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "alacrian_jaguar_pump"
	src := attacksWhileSaddledGuard(perm, ctx)
	if src == nil {
		return
	}
	ts := gs.NextTimestamp()
	src.Modifications = append(src.Modifications, gameengine.Modification{
		Power:     2,
		Toughness: 2,
		Duration:  "until_end_of_turn",
		Timestamp: ts,
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// -----------------------------------------------------------------------------
// Brightfield Mustang — {3}{W} Creature — Horse Mount, Saddle 1
//
// Whenever this creature attacks while saddled, untap it and put a
// +1/+1 counter on it.
// -----------------------------------------------------------------------------

func brightfieldMustangAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "brightfield_mustang_untap_counter"
	src := attacksWhileSaddledGuard(perm, ctx)
	if src == nil {
		return
	}
	src.Tapped = false
	src.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"counters": src.Counters["+1/+1"],
	})
}

// -----------------------------------------------------------------------------
// Gilded Ghoda — {1}{R} Creature — Horse Mount, Saddle 1
//
// Whenever this creature attacks while saddled, create a Treasure token.
// -----------------------------------------------------------------------------

func gildedGhodaAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gilded_ghoda_treasure"
	src := attacksWhileSaddledGuard(perm, ctx)
	if src == nil {
		return
	}
	gameengine.CreateTreasureToken(gs, src.Controller)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// -----------------------------------------------------------------------------
// Venomsac Lagac — {2}{B} Creature — Lizard Mount, Saddle 2
//
// Deathtouch. Whenever this creature attacks while saddled, it gets
// +0/+3 until end of turn.
// -----------------------------------------------------------------------------

func venomsacLagacAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "venomsac_lagac_pump"
	src := attacksWhileSaddledGuard(perm, ctx)
	if src == nil {
		return
	}
	ts := gs.NextTimestamp()
	src.Modifications = append(src.Modifications, gameengine.Modification{
		Power:     0,
		Toughness: 3,
		Duration:  "until_end_of_turn",
		Timestamp: ts,
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// -----------------------------------------------------------------------------
// Bridled Bighorn — {3}{W} Creature — Sheep Mount, Saddle 2
//
// Vigilance. Whenever this creature attacks while saddled, create a 1/1
// white Sheep creature token.
// -----------------------------------------------------------------------------

func bridledBighornAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bridled_bighorn_sheep_token"
	src := attacksWhileSaddledGuard(perm, ctx)
	if src == nil {
		return
	}
	gameengine.CreateCreatureToken(gs, src.Controller, "Sheep",
		[]string{"creature", "sheep", "pip:W"}, 1, 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}

// -----------------------------------------------------------------------------
// District Mascot — {G} Creature — Dog Mount, Saddle 1
//
// District Mascot enters with a +1/+1 counter on it. (Engine-handled
// via the etb_with_counters AST path; see etb_static_counters.go.)
// Whenever this creature attacks while saddled, put a +1/+1 counter on it.
// -----------------------------------------------------------------------------

func districtMascotAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "district_mascot_counter"
	src := attacksWhileSaddledGuard(perm, ctx)
	if src == nil {
		return
	}
	src.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"counters": src.Counters["+1/+1"],
	})
}
