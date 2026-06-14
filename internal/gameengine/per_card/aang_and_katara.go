package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAangAndKatara wires Aang and Katara.
//
// Oracle text:
//
//	Whenever Aang and Katara enter or attack, create X 1/1 white Ally
//	creature tokens, where X is the number of tapped artifacts and/or
//	creatures you control.
func registerAangAndKatara(r *Registry) {
	r.OnETB("Aang and Katara", aangAndKataraTokens)
	r.OnTrigger("Aang and Katara", "attacks", aangAndKataraAttackTokens)
}

func aangAndKataraCount(gs *gameengine.GameState, seat int) int {
	if seat < 0 || seat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seat]
	if s == nil {
		return 0
	}
	count := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.Tapped {
			continue
		}
		if cardHasType(p.Card, "artifact") || cardHasType(p.Card, "creature") {
			count++
		}
	}
	return count
}

func aangAndKataraMakeTokens(gs *gameengine.GameState, seat, n int, source string) {
	for i := 0; i < n; i++ {
		token := &gameengine.Card{
			Name:          "Ally Token",
			Owner:         seat,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "ally"},
			Colors:        []string{"W"},
			TypeLine:      "Token Creature — Ally",
		}
		enterBattlefieldWithETB(gs, seat, token, false)
	}
	emit(gs, source, "Aang and Katara", map[string]interface{}{
		"seat":   seat,
		"tokens": n,
	})
}

func aangAndKataraTokens(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	n := aangAndKataraCount(gs, seat)
	aangAndKataraMakeTokens(gs, seat, n, "aang_and_katara_etb")
}

func aangAndKataraAttackTokens(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 — "Whenever Aang and Katara ... attack" triggers ONLY when
	// Aang and Katara itself attacks, not on every creature you control
	// attacking. The "creature_attacks" event is dispatched once per declared
	// attacker, so without this self-gate the handler fired N times per combat
	// (one per attacker), each minting X tokens, and because the freshly-minted
	// tokens enter as new declared attackers and tapped permanents bump X, the
	// cascade was unbounded — a real non-terminating game (game 0, seed 7,
	// turn 33 of the Feynman 50-game zone-accounting test hung here). Mirrors
	// the canonical self-gate every sibling attack-trigger handler uses
	// (anikthea, alesha, anafenza, …).
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	n := aangAndKataraCount(gs, perm.Controller)
	aangAndKataraMakeTokens(gs, perm.Controller, n, "aang_and_katara_attack")
}
