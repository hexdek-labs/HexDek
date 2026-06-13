package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Culling Ritual — {2}{B}{G} Sorcery.
//
//	Destroy each nonland permanent with mana value 2 or less. Add {B} or
//	{G} for each permanent destroyed this way.
//
// A popular Golgari sweeper/ritual (58 decks). Both halves parsed to inert
// `custom` slugs, so it resolved to a no-op. This handler destroys every
// nonland permanent (all players') with mana value ≤ 2 and adds that many
// mana to the caster's pool.
//
// The printed mana is "{B} or {G}" per permanent (caster's choice); the
// engine's mana pool is generic (MVP), so this adds the count as green —
// the strategically relevant fact (N mana added) is preserved.
func init() {
	registerCullingRitual(Global())
	AddResetHook(registerCullingRitual)
}

func registerCullingRitual(r *Registry) {
	r.OnResolve("Culling Ritual", cullingRitualResolve)
}

func cullingRitualResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "culling_ritual"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Collect first; DestroyPermanent mutates battlefield slices.
	var victims []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsLand() {
				continue
			}
			if p.Card.CMC <= 2 {
				victims = append(victims, p)
			}
		}
	}
	destroyed := 0
	for _, p := range victims {
		if gameengine.DestroyPermanent(gs, p, nil) {
			destroyed++
		}
	}
	if destroyed > 0 {
		gameengine.AddMana(gs, gs.Seats[seat], "G", destroyed, "Culling Ritual")
	}
	emit(gs, slug, "Culling Ritual", map[string]interface{}{
		"seat":       seat,
		"destroyed":  destroyed,
		"mana_added": destroyed,
	})
}
