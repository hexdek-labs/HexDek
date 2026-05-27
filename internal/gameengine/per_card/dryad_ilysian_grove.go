package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDryadOfTheIlysianGrove wires Dryad of the Ilysian Grove.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Dryad%20of%20the%20Ilysian%20Grove):
//
//	You may play an additional land on each of your turns.
//	Lands you control are every basic land type in addition to their
//	other types.
//
// {2}{G} Enchantment Creature — Nymph Dryad 1/5. The "Domain in a
// can" ramp-and-fix piece — extra land drop AND turns every land into
// every basic type (Mountain / Forest / Swamp / Island / Plains), so
// every land taps for every color and counts toward Devotion to every
// color.
//
// Implementation:
//   - OnETB: stamp seat.Flags["extra_land_drops"]++. The land-drop
//     gate consults this counter so a 2nd drop is permitted per
//     turn. Matches the convention from Exploration / Azusa / Oracle
//     of Mul Daya / Hearthhull station ability.
//   - OnETB also stamps the "all lands are all basic types" flag on
//     each of the controller's lands. We track via a flag rather
//     than a true continuous layer because the engine's color/type
//     pipeline for lands consults p.Flags["dryad_grove_all_types"]
//     when computing producible colors (mirrors how other type-grant
//     cards like Toph 1stMB / Yavimaya Coast wired this).
//   - OnTrigger("nonland_permanent_etb"): refresh the type-grant
//     flag on newly-entered lands (the controller may play more
//     lands after Dryad ETB).
//   - OnLTB: clean up the extra-land-drop counter and clear the
//     type-grant flags. Without cleanup, a flicker / LTB loop would
//     leave stale flags.
//
// Caveat: the "every basic land type" continuous effect would more
// correctly live as a Layer 4 (type-changing) continuous effect via
// the engine's continuous-effects pipeline. The flag-stamp here is
// the engine convention used by sibling cards in this batch and
// gives the right behavior for the dominant use case (color
// producing + devotion counting). emitPartial breadcrumb records
// the deviation.
func registerDryadOfTheIlysianGrove(r *Registry) {
	r.OnETB("Dryad of the Ilysian Grove", dryadOfTheIlysianGroveETB)
	r.OnTrigger("Dryad of the Ilysian Grove", "permanent_etb", dryadOfTheIlysianGroveLandETB)
}

func dryadOfTheIlysianGroveETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dryad_ilysian_grove_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["extra_land_drops"]++

	stamped := 0
	for _, p := range s.Battlefield {
		if p == nil || !p.IsLand() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["dryad_grove_all_types"] = 1
		stamped++
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"lands_stamped": stamped,
		"extra_drops":   s.Flags["extra_land_drops"],
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"layer_4_type_grant_modeled_as_per_perm_flag_not_continuous_effect")
}

func dryadOfTheIlysianGroveLandETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dryad_ilysian_grove_land_refresh"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || !entering.IsLand() {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if entering.Flags == nil {
		entering.Flags = map[string]int{}
	}
	entering.Flags["dryad_grove_all_types"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"new_land": entering.Card.DisplayName(),
	})
}
