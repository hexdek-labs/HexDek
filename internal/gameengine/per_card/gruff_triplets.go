package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGruffTriplets wires Gruff Triplets (Muninn parser-gap #105, 1.1K
// hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{3}{G}{G}{G}
//	Creature — Satyr Warrior
//	Trample
//	When this creature enters, if it isn't a token, create two tokens
//	that are copies of it.
//	When this creature dies, put a number of +1/+1 counters equal to its
//	power on each creature you control named Gruff Triplets.
//
// Implementation:
//   - Trample handled by AST keyword pipeline.
//   - OnETB: gate on perm.Card not having "token" type (matches Phoenix
//     Fleet Airship's token-copy guard exactly: tokens are flagged in
//     Card.Types). Token copies enter via enterBattlefieldWithETB; they
//     will themselves be tokens so the ETB clause won't recurse.
//   - OnTrigger("dies"): grab the dying perm's power (incl. any counters
//     it had at death — read BasePower + any "+1/+1" counter delta), then
//     add that many +1/+1 counters to each OTHER battlefield permanent
//     named "Gruff Triplets" the controller still has.
func registerGruffTriplets(r *Registry) {
	r.OnETB("Gruff Triplets", gruffTripletsETB)
	r.OnTrigger("Gruff Triplets", "dies", gruffTripletsDies)
}

func gruffTripletsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "gruff_triplets_token_copies"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	for _, t := range perm.Card.Types {
		if strings.EqualFold(t, "token") {
			emitFail(gs, slug, perm.Card.DisplayName(), "self_is_token", nil)
			return
		}
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	tokens := 0
	for i := 0; i < 2; i++ {
		// InstanceID Phase 5: token-as-copy chokepoint.
		card := gameengine.MintTokenAsCopyOf(gs, perm.Card, seat, "")
		if card == nil {
			continue
		}
		if enterBattlefieldWithETB(gs, seat, card, false) != nil {
			tokens++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"tokens": tokens,
	})
}

func gruffTripletsDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gruff_triplets_death_buff"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// r63 self-gate: "When THIS creature dies" — the creature_dies event is
	// dispatched once per dying creature and fireTrigger walks every
	// battlefield permanent, so without this gate a living Gruff Triplets
	// buffs its siblings on EVERY creature death. The isLeavingPermEvent
	// fallback delivers ctx["perm"]==perm on the bearer's own death.
	if p, _ := ctx["perm"].(*gameengine.Permanent); p != perm {
		return
	}
	power := perm.Card.BasePower
	if perm.Counters != nil {
		power += perm.Counters["+1/+1"]
	}
	if power <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":  perm.Controller,
			"power": 0,
		})
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	buffed := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if p.Card.DisplayName() != "Gruff Triplets" {
			continue
		}
		p.AddCounter("+1/+1", power)
		buffed++
	}
	if buffed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"power":         power,
		"siblings_buf":  buffed,
	})
}
