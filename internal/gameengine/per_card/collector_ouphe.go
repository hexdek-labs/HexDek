package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCollectorOuphe wires up Collector Ouphe.
//
// Oracle text:
//
//	Activated abilities of artifacts can't be activated unless they
//	are mana abilities.
//
// Functionally identical to Null Rod but on a 2/2 for {1}{G}. The
// body matters: Ouphe blocks, dies to removal, gets flickered. In
// cEDH the two are interchangeable by color availability. Green decks
// run Ouphe; non-green decks run Rod.
//
// We ride the same gs.Flags["null_rod_count"] counter and
// NullRodSuppresses helper from null_rod.go — semantically these are
// the same effect.
func registerCollectorOuphe(r *Registry) {
	r.OnETB("Collector Ouphe", collectorOupheETB)
}

func collectorOupheETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "collector_ouphe_static"
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	// Shares the null_rod_count bucket — identical effect.
	gs.Flags["null_rod_count"]++
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"rod_count":  gs.Flags["null_rod_count"],
		"suppresses": "artifact_activated_non_mana",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"shares_null_rod_counter_activation_dispatch_must_consult_NullRodSuppresses")
}
