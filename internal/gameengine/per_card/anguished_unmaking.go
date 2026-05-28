package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAnguishedUnmaking wires Anguished Unmaking.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Anguished%20Unmaking):
//
//	Exile target nonland permanent. You lose 3 life.
//
// {1}{W}{B} Instant. Orzhov's premier catch-all removal: exiles
// (beats indestructible, regen, Worldspine-style "leaves shuffle into
// library" wormholes), hits any nonland (creature / planeswalker /
// artifact / enchantment / commander), at instant speed for 3 mana.
// The 3-life clause is non-negotiable — pay it to remove the threat.
//
// Implementation:
//   - OnResolve. Picker prefers the highest-EV opp NONLAND permanent
//     by the same tiering as Beast Within (planeswalker > big
//     creature > artifact > enchantment / other), but excludes lands
//     (printed restriction).
//   - ExilePermanent through the canonical path — fires §614 would-
//     be-exiled replacements, §603.6c LTB triggers, §903.9b commander
//     redirect, and routes the Card to exile.
//   - Controller loses 3 life via LoseLife so SBA 704.5a fires, life-
//     loss triggers (Sanguine Bond, Blood Artist) observe, and the
//     life-loss accumulator (seat.Turn.LifeLost) bumps for downstream
//     payoffs (Marauding Blight-Priest pattern).
func registerAnguishedUnmaking(r *Registry) {
	r.OnResolve("Anguished Unmaking", anguishedUnmakingResolve)
}

func anguishedUnmakingResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "anguished_unmaking"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	target := pickAnguishedUnmakingTarget(gs, seat)
	if target == nil {
		emitFail(gs, slug, "Anguished Unmaking", "no_valid_target", nil)
		// Note: the 3-life cost in the printed text is part of the
		// resolution effect, NOT a cast-time cost. Per CR §608.2 we
		// still apply the life loss even when the target portion
		// fizzles — but only if there was any legal target to begin
		// with. With no nonland permanent in play across the table,
		// the spell does nothing — the picker no-op covers this.
		// When a target existed but was illegal by resolution time
		// (became hexproof, etc.), the life loss still applies. The
		// picker returning nil here means no opp had any nonland —
		// nothing to do.
		return
	}
	targetName := target.Card.DisplayName()
	gameengine.ExilePermanent(gs, target, nil)
	gameengine.LoseLife(gs, seat, 3, "Anguished Unmaking")

	emit(gs, slug, "Anguished Unmaking", map[string]interface{}{
		"seat":         seat,
		"exiled":       targetName,
		"life_paid":    3,
	})
}

// pickAnguishedUnmakingTarget chooses an opponent's nonland permanent
// by EV tier: planeswalker > big creature > artifact > anything.
// Returns nil for empty boards / lands-only.
func pickAnguishedUnmakingTarget(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	tier := func(p *gameengine.Permanent) int {
		switch {
		case cardHasType(p.Card, "planeswalker"):
			return 5
		case p.IsCreature() && p.Power() >= 5:
			return 4
		case cardHasType(p.Card, "artifact") && !p.IsCreature():
			return 3
		case cardHasType(p.Card, "enchantment"):
			return 2
		default:
			return 1
		}
	}
	var best *gameengine.Permanent
	bestTier := 0
	for _, opp := range gs.Opponents(seat) {
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsLand() {
				continue
			}
			tt := tier(p)
			if tt > bestTier {
				bestTier = tt
				best = p
			}
		}
	}
	return best
}
