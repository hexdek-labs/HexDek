package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSakashimaOfAThousandFacesCustom implements the ETB-as-a-copy
// replacement effect. Partner and legend-rule suppression are engine
// territory.
//
// Oracle text:
//
//	You may have Sakashima enter as a copy of another creature you
//	control, except it has Sakashima's other abilities.
//	The "legend rule" doesn't apply to permanents you control.
//	Partner (You can have two commanders if both have partner.)
//
// We approximate the as-enters replacement by: pick the highest-power
// non-Sakashima creature we control, copy its base power/toughness onto
// Sakashima's permanent, and tag perm.Flags["sakashima_copy_of"] with
// the source's display name for downstream observers.
func registerSakashimaOfAThousandFacesCustom(r *Registry) {
	r.OnETB("Sakashima of a Thousand Faces", sakashimaCopyETB)
}

func sakashimaCopyETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sakashima_copy_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var target *gameengine.Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if normalizeName(p.Card.DisplayName()) == normalizeName(perm.Card.DisplayName()) {
			continue
		}
		if p.Power() > bestPower {
			target = p
			bestPower = p.Power()
		}
	}
	if target == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
			"note": "no_copy_target_kept_as_self",
		})
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Card.BasePower = target.Card.BasePower
	perm.Card.BaseToughness = target.Card.BaseToughness
	perm.OriginalCard = perm.Card
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copy_of": target.Card.DisplayName(),
		"power":  perm.Power(),
	})
	// Batch AL upgrade: delegate to full DeepCopy + legend-rule-suppression
	// rider so Sakashima copies abilities/types, not just P/T. CR §706.2.
	sakashimaFullCopyUpgrade(gs, perm, target)
}
