package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch #17 — Combat restriction permanents.
//
// These cards modify how combat works by restricting attackers/blockers
// or imposing costs to attack. They stamp flags on ETB that the engine
// reads during DeclareAttackers.

// ---------------------------------------------------------------------------
// Propaganda / Ghostly Prison / Windborn Muse / Baird, Steward of Argive
//
// "Creatures can't attack you unless their controller pays {2} for each
// creature they control that's attacking you."
// Implementation: flag on gs.Flags["propaganda_seat_N"] — combat.go
// checks this and deducts mana per attacker. If can't pay, attacker is
// removed from the declared list.
// ---------------------------------------------------------------------------

func registerPropaganda(r *Registry) {
	r.OnETB("Propaganda", propagandaETB)
	r.OnETB("Ghostly Prison", propagandaETB)
	r.OnETB("Windborn Muse", propagandaETB)
	r.OnETB("Baird, Steward of Argive", propagandaETB)
}

func propagandaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["propaganda_seat_"+itoa(perm.Controller)] = 1
	emit(gs, "propaganda_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "attack_tax_2_per_creature",
	})
}

// ---------------------------------------------------------------------------
// Silent Arbiter
//
// "No more than one creature can attack each combat."
// "No more than one creature can block each combat."
// Implementation: flag on gs.Flags["silent_arbiter_active"].
// ---------------------------------------------------------------------------

func registerSilentArbiter(r *Registry) {
	r.OnETB("Silent Arbiter", silentArbiterETB)
}

func silentArbiterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["silent_arbiter_active"] = 1
	emit(gs, "silent_arbiter_etb", "Silent Arbiter", map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "max_one_attacker_one_blocker",
	})
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func init() {
	registerCombatRestrictions(Global())
	AddResetHook(registerCombatRestrictions)
}

func registerCombatRestrictions(r *Registry) {
	registerPropaganda(r)
	registerSilentArbiter(r)
}
