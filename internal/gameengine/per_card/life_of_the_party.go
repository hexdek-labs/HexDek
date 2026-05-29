package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLifeOfTheParty wires Life of the Party (Muninn parser-gap #73,
// ~9.1K hits).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{3}{R}
//	Creature — Elemental
//	First strike, trample, haste
//	Whenever this creature attacks, it gets +X/+0 until end of turn,
//	where X is the number of creatures you control.
//	When this creature enters, if it's not a token, each opponent
//	creates a token that's a copy of it. The tokens are goaded for the
//	rest of the game.
//
// Implementation:
//   - First strike / trample / haste are AST-side.
//   - creature_attacks (gated on attacker_perm == perm): count creatures
//     the controller controls and add a temporary modification of +X/0
//     until end of turn.
//   - ETB if not token: for each living opponent, deep-copy this card's
//     printable characteristics into a fresh token Card, drop it on
//     their battlefield via enterBattlefieldWithETB, and stamp the
//     persistent goaded flag (engine never clears Flags["goaded"], so a
//     single set lasts "for the rest of the game" by accident-of-design).
func registerLifeOfTheParty(r *Registry) {
	r.OnETB("Life of the Party", lifeOfThePartyETB)
	r.OnTrigger("Life of the Party", "creature_attacks", lifeOfThePartyOnAttack)
}

func lifeOfThePartyETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "life_of_the_party_etb_goaded_copies"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.IsToken() {
		emitFail(gs, slug, perm.Card.DisplayName(), "self_is_token", nil)
		return
	}
	src := perm.Card
	made := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		if opp < 0 || opp >= len(gs.Seats) {
			continue
		}
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		// InstanceID Phase 5: token-as-copy chokepoint.
		token := gameengine.MintTokenAsCopyOf(gs, src, opp, "")
		if token == nil {
			continue
		}
		token.Name = src.DisplayName() + " (Life-of-the-Party token)"
		tok := enterBattlefieldWithETB(gs, opp, token, false)
		if tok != nil {
			if tok.Flags == nil {
				tok.Flags = map[string]int{}
			}
			tok.Flags["goaded"] = 1
		}
		made++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tokens": made,
	})
}

func lifeOfThePartyOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "life_of_the_party_attack_pump"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	x := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			x++
		}
	}
	if x == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
			"x":    0,
		})
		return
	}
	perm.Modifications = append(perm.Modifications, gameengine.Modification{
		Power:     x,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"x":    x,
	})
}
