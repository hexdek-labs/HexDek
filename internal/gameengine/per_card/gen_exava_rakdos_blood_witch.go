package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerExavaRakdosBloodWitch wires Exava, Rakdos Blood Witch.
//
// Oracle text:
//
//	First strike, haste
//	Unleash (You may have this creature enter with a +1/+1 counter on
//	it. It can't block as long as it has a +1/+1 counter on it.)
//	Each other creature you control with a +1/+1 counter on it has haste.
//
// Implementation:
//   - First strike, haste, unleash itself: AST keyword pipeline owns
//     these. We default-take the unleash by stamping a +1/+1 counter on
//     Exava at ETB (the AI gain almost always wants the bigger body).
//   - Anthem: scan controller's battlefield, give kw:haste to every
//     other creature with at least one +1/+1 counter. Refresh on
//     permanent_etb (new creatures entering with counters) and on
//     counter changes via creature_dies (cheap proxy — we recompute
//     when the board state shifts).
func registerExavaRakdosBloodWitch(r *Registry) {
	r.OnETB("Exava, Rakdos Blood Witch", exavaETBUnleashAndAnthem)
	r.OnTrigger("Exava, Rakdos Blood Witch", "permanent_etb", exavaRefreshHasteAnthem)
	r.OnTrigger("Exava, Rakdos Blood Witch", "creature_dies", exavaRefreshHasteAnthemTrigger)
}

func exavaETBUnleashAndAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "exava_etb_unleash_and_anthem"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// Default-take unleash: stamp a +1/+1 on Exava and lock blocking.
	perm.AddCounter("+1/+1", 1)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["unleash_cant_block"] = 1
	gs.InvalidateCharacteristicsCache()
	exavaApplyHasteAnthem(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"unleash":  true,
		"counters": perm.Counters["+1/+1"],
	})
}

func exavaRefreshHasteAnthem(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	exavaApplyHasteAnthem(gs, perm)
}

func exavaRefreshHasteAnthemTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	exavaApplyHasteAnthem(gs, perm)
}

func exavaApplyHasteAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Counters == nil || p.Counters["+1/+1"] <= 0 {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, p, "haste")
	}
}
