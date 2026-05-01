package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKaaliaOfTheVast wires Kaalia of the Vast.
//
// Oracle text:
//
//	Flying
//	Whenever Kaalia of the Vast attacks an opponent, you may put an
//	Angel, Demon, or Dragon creature card from your hand onto the
//	battlefield tapped and attacking that opponent.
//
// Implementation:
//   - "creature_attacks": when Kaalia herself attacks, scan hand for an
//     Angel / Demon / Dragon creature card, prefer the highest base
//     power for aggression, remove from hand, and route through
//     enterBattlefieldWithETB so ETB triggers fire (CR §603.6a). The
//     cheated permanent enters tapped and stamped Flags["attacking"]=1
//     to mirror the satya pattern (combat damage rules see attackers via
//     gs.Combat; the flag is for analytics + downstream handlers).
func registerKaaliaOfTheVast(r *Registry) {
	r.OnTrigger("Kaalia of the Vast", "creature_attacks", kaaliaOfTheVastAttack)
}

func kaaliaOfTheVastAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kaalia_of_the_vast_attack_cheat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm != perm {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || len(s.Hand) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "empty_hand", nil)
		return
	}

	bestIdx := -1
	bestPow := -1
	for i, c := range s.Hand {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if !cardHasType(c, "angel") && !cardHasType(c, "demon") && !cardHasType(c, "dragon") {
			continue
		}
		if c.BasePower > bestPow {
			bestPow = c.BasePower
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_angel_demon_dragon_in_hand", map[string]interface{}{
			"seat":      seat,
			"hand_size": len(s.Hand),
		})
		return
	}

	card := s.Hand[bestIdx]
	s.Hand = append(s.Hand[:bestIdx], s.Hand[bestIdx+1:]...)

	cheated := enterBattlefieldWithETB(gs, seat, card, true)
	if cheated == nil {
		return
	}
	if cheated.Flags == nil {
		cheated.Flags = map[string]int{}
	}
	cheated.Flags["attacking"] = 1

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"cheated":   card.DisplayName(),
		"power":     card.BasePower,
		"toughness": card.BaseToughness,
		"tapped":    true,
		"attacking": true,
	})
}
