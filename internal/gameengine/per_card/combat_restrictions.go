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

// PropagandaTax returns the per-attacker mana cost to attack a given
// defending seat. Called from combat.go after DeclareAttackers to filter
// out attackers the controller can't afford. Returns 0 if no propaganda
// effect applies.
func PropagandaTax(gs *gameengine.GameState, defendingSeat int) int {
	if gs == nil || gs.Flags == nil {
		return 0
	}
	tax := 0
	key := "propaganda_seat_" + itoa(defendingSeat)
	if gs.Flags[key] > 0 {
		// Verify at least one propaganda-type card is still on the battlefield.
		for _, p := range gs.Seats[defendingSeat].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			name := p.Card.DisplayName()
			if name == "Propaganda" || name == "Ghostly Prison" ||
				name == "Windborn Muse" || name == "Baird, Steward of Argive" {
				tax += 2
				break
			}
		}
	}
	return tax
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

// SilentArbiterActive returns true if a Silent Arbiter is on any
// battlefield. Called from combat.go to limit declared attackers to 1.
func SilentArbiterActive(gs *gameengine.GameState) bool {
	if gs == nil || gs.Flags == nil || gs.Flags["silent_arbiter_active"] == 0 {
		return false
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.Card.DisplayName() == "Silent Arbiter" {
				return true
			}
		}
	}
	return false
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
	r := Global()
	registerPropaganda(r)
	registerSilentArbiter(r)
}
