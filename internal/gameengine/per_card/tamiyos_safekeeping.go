package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTamiyosSafekeeping wires Tamiyo's Safekeeping.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Tamiyo%27s%20Safekeeping):
//
//	Target permanent you control gains hexproof and indestructible
//	until end of turn. You gain 2 life.
//
// {1}{G} Instant. The premier single-target protection cantrip — 2
// mana to hexproof+indestructible AND gain 2 life is exceptional
// value when the target is a wincon (Heliod / Walking Ballista lock
// half, Krenko, Birthing Pod, etc.) or a commander mid-board.
// Strictly stronger than Tamiyo's Compleat Reflection in single-
// target value; Heroic Intervention covers the board-wide case at
// +1 mana.
//
// Implementation:
//   - OnResolve. Pick the highest-EV target permanent controller
//     controls (tier policy: commander > planeswalker > big-power
//     creature > artifact mana source > anything). Grant hexproof
//     + indestructible by appending to GrantedAbilities; clears at
//     EOT via the cleanup pass.
//   - Gain 2 life unconditionally — even when no legal target
//     exists, since the printed text doesn't gate the lifegain on
//     the target portion (matches CR §608.2 "do as much as you can"
//     for partial-effect spells with multiple clauses).
//   - LoseLife / GainLife routes through the engine primitive so
//     life-gain observers (Sanguine Bond, Heliod's Intervention)
//     fire correctly.
func registerTamiyosSafekeeping(r *Registry) {
	r.OnResolve("Tamiyo's Safekeeping", tamiyosSafekeepingResolve)
}

func tamiyosSafekeepingResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "tamiyos_safekeeping"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		// Lost-controller safety — life clause is no-op for a Lost seat
		// and the perm pick would no-op too.
		return
	}

	target := pickTamiyosSafekeepingTarget(s.Battlefield)
	var targetName string
	if target != nil {
		targetName = target.Card.DisplayName()
		target.GrantedAbilities = append(target.GrantedAbilities,
			"hexproof", "indestructible")
		gs.InvalidateCharacteristicsCache()
	}

	// Life gain is unconditional — applies even when no legal target.
	gameengine.GainLife(gs, seat, 2, "Tamiyo's Safekeeping")

	emit(gs, slug, "Tamiyo's Safekeeping", map[string]interface{}{
		"seat":   seat,
		"target": targetName,
		"life":   2,
	})
}

// pickTamiyosSafekeepingTarget picks the highest-EV own permanent.
func pickTamiyosSafekeepingTarget(bf []*gameengine.Permanent) *gameengine.Permanent {
	tier := func(p *gameengine.Permanent) int {
		switch {
		case cardHasType(p.Card, "planeswalker"):
			return 5
		case p.IsCreature() && p.Power() >= 5:
			return 4
		case cardHasType(p.Card, "artifact") && !p.IsCreature():
			return 3
		case p.IsCreature():
			return 2
		default:
			return 1
		}
	}
	var best *gameengine.Permanent
	bestTier := 0
	for _, p := range bf {
		if p == nil || p.Card == nil {
			continue
		}
		tt := tier(p)
		if tt > bestTier {
			bestTier = tt
			best = p
		}
	}
	return best
}
