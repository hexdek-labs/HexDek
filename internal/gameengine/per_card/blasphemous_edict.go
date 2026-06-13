package per_card

import (
	"sort"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Blasphemous Edict — {3}{B}{B} Sorcery.
//
//	(You may pay {B} rather than its mana cost if there are thirteen or
//	more creatures on the battlefield.)
//	Each player sacrifices thirteen creatures of their choice.
//
// A mass-edict finisher (44 decks). Both halves parsed to inert `custom`
// slugs, so it resolved to a no-op. This handler makes each player sacrifice
// up to thirteen creatures (all of them if they control fewer than 13).
//
// "Of their choice": a player keeps their best creatures, so when a player
// controls more than 13 we sacrifice the THIRTEEN WEAKEST (by power), leaving
// the strongest. The {B} alternative cost is a cast-time concern handled
// elsewhere and is not part of resolution.
func init() {
	registerBlasphemousEdict(Global())
	AddResetHook(registerBlasphemousEdict)
}

func registerBlasphemousEdict(r *Registry) {
	r.OnResolve("Blasphemous Edict", blasphemousEdictResolve)
}

func blasphemousEdictResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "blasphemous_edict"
	if gs == nil || item == nil {
		return
	}
	const n = 13
	total := 0
	for si, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		var creatures []*gameengine.Permanent
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.IsCreature() {
				creatures = append(creatures, p)
			}
		}
		if len(creatures) == 0 {
			continue
		}
		// Keep the strongest: sacrifice the weakest up to 13.
		sort.SliceStable(creatures, func(i, j int) bool {
			return creatures[i].Power() < creatures[j].Power()
		})
		k := n
		if k > len(creatures) {
			k = len(creatures)
		}
		for i := 0; i < k; i++ {
			gameengine.SacrificePermanent(gs, creatures[i], "blasphemous_edict")
			total++
		}
		_ = si
	}
	emit(gs, slug, "Blasphemous Edict", map[string]interface{}{
		"sacrificed_total": total,
	})
}
