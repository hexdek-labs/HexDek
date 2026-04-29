package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMirageMirror wires up Mirage Mirror.
//
// Oracle text:
//
//	{2}: Mirage Mirror becomes a copy of target artifact, creature,
//	enchantment, or land until end of turn.
//
// Implementation:
//   - OnActivated: copy the target permanent using the layer-1 copy
//     infrastructure with DurationEndOfTurn. At cleanup, the effect
//     expires and Mirage Mirror reverts to its printed characteristics.
func registerMirageMirror(r *Registry) {
	r.OnActivated("Mirage Mirror", mirageMirrorActivated)
}

func mirageMirrorActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mirage_mirror_copy"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller

	// Pay {2}.
	s := gs.Seats[seat]
	if s.ManaPool < 2 {
		emitFail(gs, slug, "Mirage Mirror", "insufficient_mana", nil)
		return
	}
	s.ManaPool -= 2
	gameengine.SyncManaAfterSpend(s)

	// Find the best copy target -- highest-power creature, or any
	// permanent if no creatures.
	var best *gameengine.Permanent
	bestPow := -1
	for _, st := range gs.Seats {
		if st == nil {
			continue
		}
		for _, p := range st.Battlefield {
			if p == nil || p == src {
				continue
			}
			if p.IsCreature() && p.Power() > bestPow {
				best = p
				bestPow = p.Power()
			}
		}
	}
	// Fall back to any permanent.
	if best == nil {
		for _, st := range gs.Seats {
			if st == nil {
				continue
			}
			for _, p := range st.Battlefield {
				if p == nil || p == src {
					continue
				}
				best = p
				break
			}
			if best != nil {
				break
			}
		}
	}
	if best == nil {
		emitFail(gs, slug, "Mirage Mirror", "no_copy_target", nil)
		return
	}

	// Use the layer-1 copy infrastructure with EOT duration.
	gameengine.CopyPermanentLayered(gs, src, best, gameengine.DurationEndOfTurn)

	emit(gs, slug, "Mirage Mirror", map[string]interface{}{
		"seat":   seat,
		"copied": best.Card.DisplayName(),
		"until":  "end_of_turn",
	})
}

// registerReleaseToTheWind wires up Release to the Wind.
//
// Oracle text:
//
//	Exile target nonland permanent. For as long as that card remains
//	exiled, its owner may cast it without paying its mana cost.
//
// Implementation:
//   - OnResolve: exile the target permanent.
//   - Register a ZoneCastPermission granting the card's owner the
//     ability to cast it from exile for free (ManaCost = 0).
//   - The ZoneCastGrant persists as long as the card remains in exile.
func registerReleaseToTheWind(r *Registry) {
	r.OnResolve("Release to the Wind", releaseToTheWindResolve)
}

func releaseToTheWindResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "release_to_the_wind"
	if gs == nil || item == nil {
		return
	}
	casterSeat := item.Controller

	// Find a nonland permanent to exile -- prefer opponent's.
	var target *gameengine.Permanent
	for _, oppIdx := range gs.Opponents(casterSeat) {
		opp := gs.Seats[oppIdx]
		if opp == nil {
			continue
		}
		for _, p := range opp.Battlefield {
			if p == nil || p.IsLand() {
				continue
			}
			target = p
			break
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		emitFail(gs, slug, "Release to the Wind", "no_nonland_target", nil)
		return
	}

	ownerSeat := target.Owner
	card := target.Card

	// Exile the permanent via ExilePermanent for proper zone-change handling:
	// replacement effects, LTB triggers, commander redirect.
	gameengine.ExilePermanent(gs, target, nil)

	// Register a ZoneCastPermission granting the owner the ability to
	// cast this card from exile without paying its mana cost.
	if card != nil {
		perm := gameengine.NewFreeCastFromExilePermission(ownerSeat, "Release to the Wind")
		gameengine.RegisterZoneCastGrant(gs, card, perm)
	}

	cardName := ""
	if card != nil {
		cardName = card.DisplayName()
	}
	emit(gs, slug, "Release to the Wind", map[string]interface{}{
		"caster":         casterSeat,
		"exiled":         cardName,
		"owner":          ownerSeat,
		"zone_cast_grant": "free_exile_cast",
	})
}
