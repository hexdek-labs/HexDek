package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVarolzTheScarStriped wires Varolz, the Scar-Striped.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{1}{B}{G}
//	Legendary Creature — Troll Warrior
//	2/2
//	Each creature card in your graveyard has scavenge. The scavenge cost
//	is equal to its mana cost. (Exile a creature card from your
//	graveyard and pay its mana cost: Put a number of +1/+1 counters
//	equal to that card's power on target creature.)
//	Sacrifice another creature: Regenerate Varolz.
//
// Implementation:
//   - OnActivated abilityIdx 0: scavenge — find best (highest power /
//     CMC ratio) creature in graveyard, exile it, dump its power as
//     +1/+1 counters onto Varolz himself (the controller's biggest
//     beater). Doesn't track mana availability — emitPartial.
//   - OnActivated abilityIdx 1: sacrifice another creature to regenerate.
//     Engine has no general regenerate primitive in per_card scope —
//     emitPartial; we still consume the sac so the AI can model the cost.
func registerVarolzTheScarStriped(r *Registry) {
	r.OnActivated("Varolz, the Scar-Striped", varolzTheScarStripedActivate)
}

func varolzTheScarStripedActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "varolz_scar_striped_scavenge"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx == 1 {
		seat := gs.Seats[src.Controller]
		if seat == nil {
			return
		}
		var fodder *gameengine.Permanent
		fodderPower := 1 << 30
		for _, p := range seat.Battlefield {
			if p == nil || p == src || p.Card == nil || !p.IsCreature() {
				continue
			}
			pw := p.Power()
			if pw < fodderPower {
				fodderPower = pw
				fodder = p
			}
		}
		if fodder == nil {
			emitFail(gs, "varolz_regenerate", src.Card.DisplayName(), "no_creature_to_sacrifice", nil)
			return
		}
		gameengine.SacrificePermanent(gs, fodder, "varolz_regenerate")
		emitPartial(gs, "varolz_regenerate", src.Card.DisplayName(),
			"regenerate_replacement_effect_not_implemented_sac_consumed")
		return
	}

	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	bestIdx := -1
	bestRatio := -1
	for i, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		// Scavenge value: raw power; ties broken by lower CMC.
		power := int(c.BasePower)
		if power <= 0 {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		score := power*100 - cmc
		if score > bestRatio {
			bestRatio = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_scavenge_target_in_graveyard", nil)
		return
	}
	scavenged := seat.Graveyard[bestIdx]
	// MoveCard handles the graveyard removal + §614 replacement chain +
	// commander redirect + zone-change trigger fan-out. Prior manual splice
	// before this call was redundant (Wave 2 final-audit cleanup).
	gameengine.MoveCard(gs, scavenged, src.Controller, "graveyard", "exile", "varolz_scavenge")
	power := int(scavenged.BasePower)
	src.AddCounter("+1/+1", power)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"scavenged": scavenged.DisplayName(),
		"counters":  power,
		"target":    src.Card.DisplayName(),
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"scavenge_mana_cost_payment_not_modeled_target_locked_to_self")
}
