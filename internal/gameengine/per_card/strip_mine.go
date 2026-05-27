package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStripMineFamily wires Strip Mine and Wasteland (sibling
// land-destruction utility lands).
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Strip%20Mine
// and /Wasteland):
//
//   Strip Mine:
//     {T}: Add {C}.
//     {T}, Sacrifice this land: Destroy target land.
//
//   Wasteland:
//     {T}: Add {C}.
//     {T}, Sacrifice this land: Destroy target nonbasic land.
//
// The {T}: Add {C} mana ability is the engine's standard untyped-tap-
// for-colorless that the lands pipeline handles; this file implements
// only the sac-for-LD activation (ability index 0 of the per_card
// dispatch — first hand-written ability beyond the inherited basic
// tap mana).
//
// Implementation:
//   - OnActivated(0): tap, sacrifice the source land, destroy a
//     legal target land. Strip Mine destroys any land; Wasteland
//     only destroys nonbasic lands.
//   - Target picker: prefer the most-impactful opponent land
//     (utility lands first — Gaea's Cradle / Mox-style lands / fetch
//     lands with a counter — then any nonbasic, then any land). For
//     Wasteland the basic-land prefilter excludes basic targets.
//   - LD routes through DestroyPermanent so dies/LTB triggers and
//     replacement effects (Rest in Peace doesn't apply but commander
//     redirect-by-controller might, in theory) fire correctly.
func registerStripMineFamily(r *Registry) {
	r.OnActivated("Strip Mine", stripMineActivate)
	r.OnActivated("Wasteland", wastelandActivate)
}

func stripMineActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	runStripMineLD(gs, src, abilityIdx, ctx, false, "Strip Mine", "strip_mine_ld")
}

func wastelandActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	runStripMineLD(gs, src, abilityIdx, ctx, true, "Wasteland", "wasteland_ld")
}

func runStripMineLD(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}, nonbasicOnly bool, cardName, slug string) {
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, cardName, "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Pick target. Caller may pre-specify via ctx["target_perm"].
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target != nil {
		if !target.IsLand() {
			emitFail(gs, slug, cardName, "target_not_land", nil)
			return
		}
		if nonbasicOnly && cardHasType(target.Card, "basic") {
			emitFail(gs, slug, cardName, "target_is_basic", nil)
			return
		}
	} else {
		target = pickStripMineLDTarget(gs, seat, nonbasicOnly)
	}
	if target == nil {
		emitFail(gs, slug, cardName, "no_legal_target", nil)
		return
	}

	// Pay cost: tap, sacrifice the source. Then destroy the target.
	src.Tapped = true
	gameengine.SacrificePermanent(gs, src, slug+"_sac_cost")
	gameengine.DestroyPermanent(gs, target, src)

	emit(gs, slug, cardName, map[string]interface{}{
		"seat":            seat,
		"target_land":     target.Card.DisplayName(),
		"target_controller": target.Controller,
	})
}

// pickStripMineLDTarget picks the most-impactful opponent land to
// destroy. Heuristic:
//
//	1. Utility lands by name (cEDH-known names with strong abilities)
//	2. Any opponent's nonbasic land
//	3. (Strip Mine only) any opponent's basic land
//	4. Fall back to own nonbasic if no opponent land exists (corner
//	   case — Strip Mine is sometimes activated on own dual to enable
//	   shuffle / specific land-fall interaction)
func pickStripMineLDTarget(gs *gameengine.GameState, seat int, nonbasicOnly bool) *gameengine.Permanent {
	utilityLands := map[string]bool{
		"Gaea's Cradle":       true,
		"Cabal Coffers":       true,
		"Nykthos, Shrine to Nyx": true,
		"Maze of Ith":         true,
		"The Tabernacle at Pendrell Vale": true,
		"Glacial Chasm":       true,
		"Ancient Tomb":        true,
		"City of Traitors":    true,
		"Wasteland":           true,
		"Strip Mine":          true,
		"Bojuka Bog":          true,
		"Boseiju, Who Endures": true,
		"Field of the Dead":   true,
	}

	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsLand() {
				continue
			}
			if nonbasicOnly && cardHasType(p.Card, "basic") {
				continue
			}
			if utilityLands[p.Card.DisplayName()] {
				return p
			}
		}
	}
	// Tier 2: any opponent nonbasic.
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsLand() {
				continue
			}
			if cardHasType(p.Card, "basic") {
				continue
			}
			return p
		}
	}
	if nonbasicOnly {
		return nil
	}
	// Tier 3: any opponent land (Strip Mine only).
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsLand() {
				continue
			}
			return p
		}
	}
	return nil
}
