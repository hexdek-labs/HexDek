package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWanShiTongAllKnowing wires Wan Shi Tong, All-Knowing.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Flying
//	When Wan Shi Tong enters, target nonland permanent's owner puts it
//	into their library second from the top or on the bottom.
//	Whenever one or more cards are put into a library from anywhere,
//	create two 1/1 colorless Spirit creature tokens with "This token
//	can't block or be blocked by non-Spirit creatures."
//
// Implementation:
//   - Flying: AST keyword pipeline.
//   - ETB: pick the highest-threat opponent-controlled nonland permanent
//     and tuck it on the bottom of its owner's library. Tucking on the
//     bottom is strictly stronger than second-from-top in 99% of cases
//     (target won't be redrawn naturally), so we always pick bottom.
//   - Library-trigger: the engine doesn't emit a general "card put into a
//     library from anywhere" event, so this handler fires the Spirit
//     token creation INLINE on the ETB tuck (the most common library-add
//     vector while Wan Shi Tong is on the battlefield). External library
//     adds (tutors that put cards on top, Memory Lapse, etc.) are NOT
//     wired and will not fire the trigger.
func registerWanShiTongAllKnowing(r *Registry) {
	r.OnETB("Wan Shi Tong, All-Knowing", wanShiTongETB)
}

func wanShiTongETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "wan_shi_tong_etb_tuck"
	if gs == nil || perm == nil {
		return
	}
	target := wanShiTongPickTuckTarget(gs, perm.Controller)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		// Even with no target, the second ability cannot fire (nothing
		// went to a library), so just return.
		return
	}
	owner := target.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		return
	}
	tuckedName := target.Card.DisplayName()
	tuckedCard := target.Card

	// Remove from battlefield, place on bottom of owner's library.
	if !removePermanent(gs, target) {
		emitFail(gs, slug, perm.Card.DisplayName(), "remove_failed", nil)
		return
	}
	ownerSeat := gs.Seats[owner]
	if ownerSeat == nil {
		return
	}
	ownerSeat.Library = append(ownerSeat.Library, tuckedCard)

	gs.LogEvent(gameengine.Event{
		Kind:   "tuck_to_library",
		Seat:   perm.Controller,
		Target: owner,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"target":   tuckedName,
			"position": "bottom",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"target":     tuckedName,
		"owner_seat": owner,
		"position":   "bottom",
	})

	// Second ability: a card was put into a library. Create two 1/1
	// colorless Spirit tokens with the non-Spirit block restriction.
	wanShiTongMakeSpirits(gs, perm, "etb_tuck")
}

// wanShiTongPickTuckTarget chooses the highest-threat nonland permanent
// controlled by any opponent. Heuristic: prefer commanders, then highest
// CMC nonland, then highest power among creatures.
func wanShiTongPickTuckTarget(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	var best *gameengine.Permanent
	bestScore := -1
	for i, s := range gs.Seats {
		if s == nil || i == seat || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || p.IsLand() {
				continue
			}
			score := cardCMC(p.Card)
			if p.IsCreature() && p.Power() > 0 {
				score += p.Power()
			}
			// Commander bonus: commanders are the highest-value tuck target
			// since they enable the per-game pressure loop.
			if p.Flags != nil && p.Flags["is_commander"] == 1 {
				score += 100
			}
			if score > bestScore {
				bestScore = score
				best = p
			}
		}
	}
	return best
}

// wanShiTongMakeSpirits drops two 1/1 colorless Spirit tokens with the
// "can't block or be blocked by non-Spirit creatures" restriction. The
// engine does not enforce arbitrary block-restriction text on tokens, so
// the restriction is recorded as a token flag but is informational only.
func wanShiTongMakeSpirits(gs *gameengine.GameState, src *gameengine.Permanent, reason string) {
	const slug = "wan_shi_tong_spirit_tokens"
	if gs == nil || src == nil {
		return
	}
	for i := 0; i < 2; i++ {
		token := gameengine.CreateCreatureToken(gs, src.Controller, "Spirit",
			[]string{"creature", "spirit"}, 1, 1)
		if token == nil {
			continue
		}
		if token.Flags == nil {
			token.Flags = map[string]int{}
		}
		// Marker for the printed restriction; combat code does not
		// currently consume this flag, but it lets future engine work
		// honor the "non-Spirit only" block clause.
		token.Flags["wan_shi_tong_spirit_restricted"] = 1
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"count":  2,
		"reason": reason,
	})
}
