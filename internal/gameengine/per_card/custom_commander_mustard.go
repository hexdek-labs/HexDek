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
const mustardSoldierAnthemMarker = "kw:from_commander_mustard"

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
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:vigilance"] = 1
		p.Flags["kw:trample"] = 1
		p.Flags["kw:haste"] = 1
		p.Flags[mustardSoldierAnthemMarker] = 1
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
// The anthem is applied by stamping Flags["kw:<name>"] onto each
// matching creature, with a provenance marker recording who stamped it.
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
		if p == nil || p == perm || p.Flags == nil {
			continue
		}
		if p.Flags[mustardSoldierAnthemMarker] != 1 {
			continue
		}
		delete(p.Flags, mustardSoldierAnthemMarker)
		delete(p.Flags, "kw:vigilance")
		delete(p.Flags, "kw:trample")
		delete(p.Flags, "kw:haste")
		stripped++
	}
	if stripped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller, "stripped": stripped,
	})
}
