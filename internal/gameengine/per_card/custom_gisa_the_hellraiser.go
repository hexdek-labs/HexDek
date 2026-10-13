package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGisaTheHellraiserCustom wires Gisa's
// "Skeletons and Zombies you control get +1/+1 and have menace"
// anthem. The existing gisa_the_hellraiser.go handler covers the
// commit_crime token spawn; its inline doc-comment notes the
// tribal pump is AST-handled, but the AST keyword pipeline doesn't
// actually apply Skeleton/Zombie tribal-conditional anthems —
// stamping the Modification + kw:menace here makes the buff real.
const gisaTribeAnthemTag = "gisa_skeleton_zombie_anthem"

func registerGisaTheHellraiserCustom(r *Registry) {
	r.OnETB("Gisa, the Hellraiser", gisaRefreshAnthemOnETB)
	r.OnTrigger("Gisa, the Hellraiser", "permanent_etb", gisaRefreshAnthemOnEvent)
	r.OnTrigger("Gisa, the Hellraiser", "permanent_ltb", gisaOwnLTB)
}

func gisaRefreshAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	gisaRefreshTribeAnthem(gs, perm)
}

func gisaRefreshAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	gisaRefreshTribeAnthem(gs, perm)
}

func gisaRefreshTribeAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "gisa_skeleton_zombie_anthem_refresh"
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
			stripTaggedModifications(p, gisaTribeAnthemTag)
			if p != nil && p.Flags != nil && p.Flags["kw:menace_from_gisa"] == 1 {
				delete(p.Flags, "kw:menace_from_gisa")
				delete(p.Flags, "kw:menace")
			}
			continue
		}
		stripTaggedModifications(p, gisaTribeAnthemTag)
		if p.Flags != nil && p.Flags["kw:menace_from_gisa"] == 1 {
			delete(p.Flags, "kw:menace_from_gisa")
			delete(p.Flags, "kw:menace")
		}
		if !cardHasSubtype(p.Card, "skeleton") && !cardHasSubtype(p.Card, "zombie") {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Toughness: 1,
			Duration:  gisaTribeAnthemTag,
			Timestamp: gs.NextTimestamp(),
		})
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:menace"] = 1
		p.Flags["kw:menace_from_gisa"] = 1
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

func gisaOwnLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if leaving, _ := ctx["perm"].(*gameengine.Permanent); leaving == perm {
		gisaStripTribeAnthem(gs, perm)
		return
	}
	// Some other permanent left — the anthem population changed, so the
	// ordinary recompute still applies.
	gisaRefreshTribeAnthem(gs, perm)
}

// gisaStripTribeAnthem removes every +1/+1 and menace grant this Gisa
// stamped, leaving grants from any OTHER source untouched.
func gisaStripTribeAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "gisa_skeleton_zombie_anthem_strip"
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
		if p.Flags["kw:menace_from_gisa"] != 1 {
			continue
		}
		stripTaggedModifications(p, gisaTribeAnthemTag)
		delete(p.Flags, "kw:menace_from_gisa")
		delete(p.Flags, "kw:menace")
		stripped++
	}
	if stripped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller, "stripped": stripped,
	})
}
