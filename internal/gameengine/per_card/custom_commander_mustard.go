package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCommanderMustardCustom wires Commander Mustard's "Other
// Soldiers you control have vigilance, trample, and haste" anthem.
// The gen_*.go handler covers the {2}{R}{W} attack-pinger activated
// ability; its partial breadcrumb covers the static-anthem half.
//
// Implementation: stamp kw:vigilance / kw:trample / kw:haste on every
// other Soldier creature the controller controls, refreshed on
// Mustard's ETB and on permanent_etb. Stamps are idempotent, so a
// re-run simply re-applies them.
//
// Mustard's OWN departure is handled by mustardOwnLTB, which strips the
// grants carrying this card's provenance marker. The previous comment
// here claimed removal happened because "removal on Mustard leaving the
// battlefield naturally drops the next refresh" — there is no next
// refresh once Mustard is gone, and nothing dropped anything, so the
// anthem was permanent (r65).
// mustardSoldierAnthemMarker identifies Mustard as a source of the
// refcounted vigilance / trample / haste grants (r89). It replaces the
// private provenance flag this handler kept beside the shared stamps,
// which was written as "kw:from_commander_mustard" — INSIDE the kw:
// namespace, which runtimeKeywordSources folds VERBATIM, so it reached
// chars.Keywords as a keyword literally named "from_commander_mustard".
// Measured, not inferred: see TestKeywordFlagMarkersR89_ProvenanceTag…
// in the gameengine package. The kwsrc: namespace is outside the fold.
const mustardSoldierAnthemMarker = "commander_mustard_soldier_anthem"

func registerCommanderMustardCustom(r *Registry) {
	r.OnETB("Commander Mustard", mustardRefreshAnthemOnETB)
	r.OnTrigger("Commander Mustard", "permanent_etb", mustardRefreshAnthemOnEvent)
	r.OnTrigger("Commander Mustard", "permanent_ltb", mustardOwnLTB)
}

func mustardRefreshAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	mustardRefreshSoldierAnthem(gs, perm)
}

func mustardRefreshAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	mustardRefreshSoldierAnthem(gs, perm)
}

func mustardRefreshSoldierAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mustard_soldier_anthem_refresh"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasSubtype(p.Card, "soldier") {
			continue
		}
		// r89: refcounted grant; the marker IS the provenance record,
		// so the separate provenance flag is gone.
		gameengine.GrantKeywordFlag(p, "vigilance", mustardSoldierAnthemMarker)
		gameengine.GrantKeywordFlag(p, "trample", mustardSoldierAnthemMarker)
		gameengine.GrantKeywordFlag(p, "haste", mustardSoldierAnthemMarker)
		stamped++
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"stamped": stamped,
		})
	}
}

// ---------------------------------------------------------------------------
// r65 — static anthem stamps outlived their own source.
//
// The anthem is applied by granting the shared Flags["kw:<name>"] stamp
// to each matching creature through the r87 refcount, whose per-source
// marker is also the provenance record (r89; it used to be a separate
// flag of its own).
// The refresh function was registered for permanent_etb AND
// permanent_ltb, so when the ANTHEM SOURCE ITSELF left the battlefield
// the handler re-ran and RE-STAMPED instead of stripping: the buff
// survived its source for the rest of the game.
//
// The provenance marker existed precisely to make the strip possible;
// nothing ever performed it. Fixed by following the in-repo canonical
// shape (brudiclad_telchor_engineer.go): a dedicated own-LTB handler
// that compares ctx["perm"] against the source and strips the grants it
// owns, falling through to the ordinary recompute when some OTHER
// permanent is the one leaving.
// ---------------------------------------------------------------------------

func mustardOwnLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if leaving, _ := ctx["perm"].(*gameengine.Permanent); leaving == perm {
		mustardStripSoldierAnthem(gs, perm)
		return
	}
	mustardRefreshSoldierAnthem(gs, perm)
}

// mustardStripSoldierAnthem drops the vigilance / trample / haste
// grants this Mustard stamped, keyed on its own provenance marker.
func mustardStripSoldierAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mustard_soldier_anthem_strip"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stripped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm {
			continue
		}
		if !gameengine.HasKeywordFlagGrant(p, "vigilance", mustardSoldierAnthemMarker) &&
			!gameengine.HasKeywordFlagGrant(p, "trample", mustardSoldierAnthemMarker) &&
			!gameengine.HasKeywordFlagGrant(p, "haste", mustardSoldierAnthemMarker) {
			continue
		}
		gameengine.RevokeKeywordFlag(p, "vigilance", mustardSoldierAnthemMarker)
		gameengine.RevokeKeywordFlag(p, "trample", mustardSoldierAnthemMarker)
		gameengine.RevokeKeywordFlag(p, "haste", mustardSoldierAnthemMarker)
		stripped++
	}
	if stripped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller, "stripped": stripped,
	})
}
