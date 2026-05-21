package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOldOneEyeCustom wires Old One Eye's "Other creatures you
// control have trample" anthem. The auto-generated handler in
// gen_old_one_eye.go covers ETB token creation; the partial it emits
// is the trample-anthem gap.
//
// The AST keyword pipeline doesn't pick up Old One Eye's anthem because
// the printed text encodes it as a static ability that needs to react
// to permanents entering/leaving. We approximate by stamping
// kw:trample on every other own creature on ETB and refreshing on
// permanent_etb (new creatures arrive) and permanent_ltb (Old One Eye
// itself leaves — we need to drop the stamps then).
//
// "Fast Healing" first-main-phase recursion is a delayed trigger out of
// scope here — the gen_*.go partial still documents that gap.
func registerOldOneEyeCustom(r *Registry) {
	r.OnETB("Old One Eye", oldOneEyeApplyAnthemOnETB)
	r.OnTrigger("Old One Eye", "permanent_etb", oldOneEyeApplyAnthemOnEvent)
	r.OnTrigger("Old One Eye", "permanent_ltb", oldOneEyeApplyAnthemOnEvent)
}

func oldOneEyeApplyAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	oldOneEyeRefreshAnthem(gs, perm)
}

func oldOneEyeApplyAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	oldOneEyeRefreshAnthem(gs, perm)
}

func oldOneEyeRefreshAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "old_one_eye_trample_anthem"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if p.Flags["kw:trample"] != 1 {
			p.Flags["kw:trample"] = 1
			p.Flags["kw:trample_from_old_one_eye"] = 1
			stamped++
		}
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"stamped":  stamped,
			"keyword":  "trample",
		})
	}
}
