package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMabelHeirToCragflameCustom wires Mabel's "Other Mice you
// control get +1/+1" anthem. The gen_*.go handler covers ETB Cragflame
// equipment-token creation; its partial flags the equip-AI gap and the
// AST-handled anthem. We close the anthem here via the Cynette
// Duration-tagged Modification refresh pattern.
//
// Refreshes the buff on:
//   - Mabel's ETB.
//   - Any permanent ETB (a new Mouse may have just entered, or a
//     friendly nonland creature whose type-line we need to recheck).
//   - Any permanent LTB (the dying Mouse was carrying a buff slot).
const mabelMouseAnthemTag = "mabel_mouse_anthem"

func registerMabelHeirToCragflameCustom(r *Registry) {
	r.OnETB("Mabel, Heir to Cragflame", mabelRefreshAnthemOnETB)
	r.OnTrigger("Mabel, Heir to Cragflame", "permanent_etb", mabelRefreshAnthemOnEvent)
	r.OnTrigger("Mabel, Heir to Cragflame", "permanent_ltb", mabelRefreshAnthemOnEvent)
}

func mabelRefreshAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	mabelRefreshMouseAnthem(gs, perm)
}

func mabelRefreshAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	mabelRefreshMouseAnthem(gs, perm)
}

func mabelRefreshMouseAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mabel_mouse_anthem_refresh"
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
			stripTaggedModifications(p, mabelMouseAnthemTag)
			continue
		}
		stripTaggedModifications(p, mabelMouseAnthemTag)
		if !cardHasSubtype(p.Card, "mouse") {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Toughness: 1,
			Duration:  mabelMouseAnthemTag,
			Timestamp: gs.NextTimestamp(),
		})
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

// stripTaggedModifications removes every Modification whose Duration
// matches tag, used by the tribal-lord refresh pattern so the anthem
// can be torn down + reapplied each event tick.
func stripTaggedModifications(p *gameengine.Permanent, tag string) {
	if p == nil || len(p.Modifications) == 0 {
		return
	}
	out := p.Modifications[:0]
	for _, m := range p.Modifications {
		if m.Duration == tag {
			continue
		}
		out = append(out, m)
	}
	p.Modifications = out
}
