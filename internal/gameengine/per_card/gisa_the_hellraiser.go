package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGisaTheHellraiser wires Gisa, the Hellraiser.
//
// Oracle text:
//
//	Ward—{2}, Pay 2 life.
//	Skeletons and Zombies you control get +1/+1 and have menace.
//	Whenever you commit a crime, create two tapped 2/2 blue and black
//	Zombie Rogue creature tokens. This ability triggers only once each
//	turn.
//
// "Commit a crime" tracking is non-trivial. Skeleton/Zombie +1/+1 +
// menace tribal anthem is wired in custom_gisa_the_hellraiser.go
// (R50 batchH) via Modification refresh on permanent_etb / _ltb.
func registerGisaTheHellraiser(r *Registry) {
	r.OnTrigger("Gisa, the Hellraiser", "commit_crime", gisaCommitCrime)
}

func gisaCommitCrime(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gisa_hellraiser_crime"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	criminalSeat, _ := ctx["seat"].(int)
	if criminalSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := "gisa_hellraiser_fired_turn"
	if perm.Flags[turnKey] == gs.Turn+1 {
		return
	}
	perm.Flags[turnKey] = gs.Turn + 1
	for i := 0; i < 2; i++ {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie Rogue Token",
			[]string{"creature", "zombie", "rogue", "pip:U", "pip:B"}, 2, 2)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tokens": 2,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"crime_commit_observer_unimplemented_in_engine")
}
