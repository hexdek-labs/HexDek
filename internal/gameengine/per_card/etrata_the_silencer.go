package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEtrataTheSilencer wires Etrata, the Silencer.
//
// Oracle text:
//
//	Etrata can't be blocked.
//	Whenever Etrata deals combat damage to a player, exile target
//	creature that player controls and put a hit counter on that card.
//	That player loses the game if they own three or more exiled cards
//	with hit counters on them. Etrata's owner shuffles Etrata into
//	their library.
func registerEtrataTheSilencer(r *Registry) {
	r.OnTrigger("Etrata, the Silencer", "combat_damage_player", etrataSilencerCombat)
}

func etrataSilencerCombat(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "etrata_silencer_hit"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceName, _ := ctx["source_card"].(string)
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	def := gs.Seats[defenderSeat]
	if def == nil {
		return
	}
	// Exile a creature the defender controls.
	var target *gameengine.Permanent
	bestPow := -1
	for _, p := range def.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if pow := p.Power(); pow > bestPow {
			target = p
			bestPow = pow
		}
	}
	if target != nil && target.Card != nil {
		cardName := target.Card.DisplayName()
		// Route exile through the canonical battlefield-exit API so §614
		// would_be_exiled replacements, §903.9b commander redirect, aura
		// detachAll, replacement-effect unregistering, and LTB triggers
		// all fire. Same root-cause refactor as abdel_adrian.go (commit
		// 7e782cf) — see docs/may11-nil-deref-forensics.md.
		if gameengine.ExilePermanent(gs, target, perm) {
			// Track hit counters on the defender via seat flag.
			if def.Flags == nil {
				def.Flags = map[string]int{}
			}
			def.Flags["etrata_hits"]++
			if def.Flags["etrata_hits"] >= 3 {
				def.Lost = true
				gs.LogEvent(gameengine.Event{
					Kind:   "lose_game",
					Seat:   defenderSeat,
					Source: perm.Card.DisplayName(),
				})
			}
			emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
				"seat":        perm.Controller,
				"defender":    defenderSeat,
				"hits":        def.Flags["etrata_hits"],
				"exiled_card": cardName,
			})
		}
	}
	// Shuffle Etrata into owner's library. BouncePermanent handles
	// removePermanent + DetachAll + UnregisterReplacements + FireZoneChange
	// (which honors §903.9b commander redirect — Etrata is legendary and
	// is often used as a commander, so the redirect path is load-bearing).
	owner := perm.Owner
	if gameengine.BouncePermanent(gs, perm, perm, "library") {
		shuffleLibraryPerCard(gs, owner)
	}
}
