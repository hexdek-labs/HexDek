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
	r.OnTrigger("Gisa, the Hellraiser", "permanent_ltb", gisaRefreshAnthemOnEvent)
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
