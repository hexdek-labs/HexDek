package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// endrek_sahr_master_breeder.go — per_card handler for Endrek Sahr,
// Master Breeder.
//
// Oracle text (Legendary Creature — Human Wizard, {4}{B}):
//
//	Whenever you cast a creature spell, create X 1/1 black Thrull
//	creature tokens, where X is that spell's mana value.
//	When you control seven or more Thrulls, sacrifice Endrek Sahr.
//
// Why this needs a per_card handler: the ability nodes parsed to inert
// scaffold, so the Thrull engine never produced tokens. Verified inert:
// no registered handler for "Endrek Sahr, Master Breeder".
//
// Implemented here, both clauses:
//   - on each creature spell you cast, mint X = spell mana value 1/1
//     black Thrull tokens;
//   - the state-based "control 7+ Thrulls → sacrifice Endrek" check runs
//     immediately after minting (the common trigger point), so an
//     overshoot self-sacrifices Endrek as the rule intends.

func init() {
	registerEndrekSahr(Global())
	AddResetHook(registerEndrekSahr)
}

func registerEndrekSahr(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Endrek Sahr, Master Breeder", "creature_spell_cast", endrekSahrCast)
}

func endrekSahrCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "endrek_sahr_master_breeder"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || !cardHasType(card, "creature") {
		return
	}
	// X = that spell's mana value. Prefer the authoritative printed CMC
	// field; fall back to the per_card cardCMC proxy if unset.
	x := card.CMC
	if x <= 0 {
		x = cardCMC(card)
	}
	for i := 0; i < x; i++ {
		token := &gameengine.Card{
			Name:          "Thrull Token",
			Owner:         perm.Controller,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "thrull"},
			Colors:        []string{"B"},
			TypeLine:      "Token Creature — Thrull",
		}
		enterBattlefieldWithETB(gs, perm.Controller, token, false)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_spell": card.DisplayName(),
		"thrulls":    x,
	})

	// "When you control seven or more Thrulls, sacrifice Endrek Sahr."
	if countThrulls(gs, perm.Controller) >= 7 {
		gameengine.SacrificePermanent(gs, perm, "Endrek Sahr, Master Breeder")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"self_sac": true,
			"reason":   "seven_or_more_thrulls",
		})
	}
}

// countThrulls counts Thrull creatures the seat controls.
func countThrulls(gs *gameengine.GameState, seat int) int {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && cardHasType(p.Card, "thrull") {
			n++
		}
	}
	return n
}
