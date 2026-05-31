package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRaphMikeyTroublemakers wires Raph & Mikey, Troublemakers.
//
// Oracle text:
//
//	Trample, haste
//	Whenever Raph & Mikey attack, reveal cards from the top of your
//	library until you reveal a creature card. Put that card onto the
//	battlefield tapped and attacking and the rest on the bottom of your
//	library in a random order.
//
// Implementation:
//   - "creature_attacks" trigger gated on the attacker being Raph &
//     Mikey themselves. Walk the library top-down, take the first
//     creature, dump preceding non-creatures to the bottom (in original
//     order — the random shuffle is best modeled at the engine layer).
//   - The summoned creature enters tapped+attacking and inherits Raph
//     & Mikey's attack target so the rest of combat resolves normally.
//   - Trample/haste handled by the AST keyword pipeline.
func registerRaphMikeyTroublemakers(r *Registry) {
	r.OnTrigger("Raph & Mikey, Troublemakers", "creature_attacks", raphMikeyTroublemakersAttack)
}

func raphMikeyTroublemakersAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "raph_mikey_attack_dig"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || len(seat.Library) == 0 {
		return
	}
	// Find first creature from the top.
	pickIdx := -1
	for i, c := range seat.Library {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") {
			pickIdx = i
			break
		}
	}
	if pickIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_creature_in_library", nil)
		return
	}
	pick := seat.Library[pickIdx]
	// Bottom the preceding non-creatures in original order.
	bottomed := append([]*gameengine.Card(nil), seat.Library[:pickIdx]...)
	// Drop them from library + the picked card.
	seat.Library = append([]*gameengine.Card(nil), seat.Library[pickIdx+1:]...)
	seat.Library = append(seat.Library, bottomed...)

	// Drop pick onto battlefield tapped + attacking.
	newPerm := createPermanent(gs, perm.Controller, pick, true)
	if newPerm != nil {
		gameengine.RegisterReplacementsForPermanent(gs, newPerm)
		gameengine.FirePermanentETBTriggers(gs, newPerm)
		// CR §508.1g — the creature is placed onto the battlefield
		// in an attacking state by an effect, bypassing §508.1a-f
		// restrictions (defender, summoning sickness, tapped). The
		// MarkEnteredAttacking helper stamps both flagAttacking and
		// flagEnteredAttacking so the CombatLegality invariant
		// honors the §508.1g carve-out for the duration of combat.
		//
		// Live bug fix per PR #950 / Loki r60 pathological-gauntlet
		// game 1317: Raph & Mikey dug up Wall of Tanglecord
		// (Defender), tripping the bare-fact "defender && attacking"
		// invariant. Per rules the Wall is legitimately attacking;
		// per engine, the flag now tells the invariant so.
		gameengine.MarkEnteredAttacking(newPerm)
		// CR §302.1 summoning sickness — same §508.1g carve-out
		// rationale. The dominant prior repro was "Behemoth of Vault
		// 0" on seat-42 game 1611 (docs/loki-r60-round2-report.md);
		// clearing SS was the pre-MarkEnteredAttacking workaround,
		// preserved for double-defense.
		newPerm.SummoningSick = false
		if def, ok := gameengine.AttackerDefender(perm); ok {
			gameengine.SetAttackerDefender(newPerm, def)
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"creature":  pick.DisplayName(),
		"bottomed":  len(bottomed),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"bottom_in_random_order_modeled_as_original_order_for_determinism")
}
