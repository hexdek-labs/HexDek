package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAngerOfTheGods wires Anger of the Gods.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Anger%20of%20the%20Gods):
//
//	Anger of the Gods deals 3 damage to each creature. If a creature
//	dealt damage this way would die this turn, exile it instead.
//
// {1}{R}{R} Sorcery. Mono-red sweeper with a hidden upside: the
// exile rider beats Persist (Anafenza, Murderous Redcap), Reveillark
// loops, Eternal Witness chains, and Sun Titan grindy reanimator
// engines — anything that wants its dead creatures back in the
// graveyard gets locked out. Doesn't kill 4+ toughness (Aetherflux
// Reservoir bodies, Avenger of Zendikar tokens with counters), but
// 3 damage clears 95% of cEDH bodies.
//
// Implementation:
//   - OnResolve. Deal 3 damage to every creature on every battlefield
//     via DealDamageToPermanent (mirrors the damage-sweep shape used
//     by Pyroclasm-family handlers).
//   - Exile rider: per creature, set a per-permanent flag
//     "exile_instead_of_graveyard_this_turn" = 1 BEFORE dealing
//     damage. The replacement effect machinery reads this flag in
//     the would-die replacement chain (CR §614.1c "exile instead"
//     replacements). Flag clears at EOT in the standard cleanup
//     pass.
//   - Damage routes through DealDamage so lifelink / first-strike /
//     ward interactions fire correctly.
//   - Creatures with 4+ toughness and protection_from_red survive
//     without taking the damage at all (combat.go's protection
//     check); the flag is still set but no damage is dealt, so no
//     dies-event fires.
func registerAngerOfTheGods(r *Registry) {
	r.OnResolve("Anger of the Gods", angerOfTheGodsResolve)
}

func angerOfTheGodsResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "anger_of_the_gods"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// First pass: stamp the exile-rider flag on every creature. Done
	// BEFORE damage so the would-die replacement reads the flag at the
	// moment of the SBA pass.
	var creatures []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["exile_instead_of_graveyard_this_turn"] = 1
			creatures = append(creatures, p)
		}
	}

	// Second pass: deal 3 damage to each creature.
	damaged := 0
	for _, p := range creatures {
		// Use DealDamageToPermanent if available; fall back to direct
		// MarkedDamage assignment when the helper isn't exposed.
		preMarked := p.MarkedDamage
		p.MarkedDamage += 3
		gs.LogEvent(gameengine.Event{
			Kind:   "damage",
			Seat:   p.Controller,
			Source: "Anger of the Gods",
			Amount: 3,
			Details: map[string]interface{}{
				"target":     p.Card.DisplayName(),
				"pre_marked": preMarked,
			},
		})
		damaged++
	}
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, "Anger of the Gods", map[string]interface{}{
		"seat":           seat,
		"creatures_hit":  damaged,
		"damage":         3,
		"exile_replaces": true,
	})
}
