package per_card

import (
	"sort"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCleopatraExiledPharaoh wires Cleopatra, Exiled Pharaoh.
//
// Oracle text (Scryfall, verified):
//
//	Allies — At the beginning of your end step, put a +1/+1 counter on
//	each of up to two other target legendary creatures.
//	Betrayal — Whenever a legendary creature with counters on it dies,
//	draw a card for each counter on it. You lose 2 life.
//
// Implementation (R41 stub port):
//   - "end_step" trigger gated to active_seat == perm.Controller. AI
//     target policy: pick up to 2 of the strongest legendary creatures
//     the controller owns (own-team buff > opposing-team buff).
//     Excludes Cleopatra herself ("each of up to two OTHER target
//     legendary creatures").
//   - "creature_dies" trigger: fires when any legendary creature with
//     at least one counter dies. Total counter sum across all counter
//     kinds (oracle says "each counter on it", not just +1/+1).
//     Cleopatra's controller draws N cards then loses 2 life. The
//     2-life loss is a flat per-trigger cost (printed text), not
//     scaled by N.
func registerCleopatraExiledPharaoh(r *Registry) {
	r.OnTrigger("Cleopatra, Exiled Pharaoh", "end_step", cleopatraAlliesEndStep)
	r.OnTrigger("Cleopatra, Exiled Pharaoh", "creature_dies", cleopatraBetrayalDies)
}

func cleopatraAlliesEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cleopatra_allies_counter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	targets := cleopatraPickTargets(gs, perm)
	if len(targets) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_other_legendary_creature", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	names := make([]string, 0, len(targets))
	for _, t := range targets {
		t.AddCounter("+1/+1", 1)
		names = append(names, t.Card.DisplayName())
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"targets": names,
		"count":   len(names),
	})
}

// cleopatraPickTargets returns up to 2 other legendary creatures
// controlled by Cleopatra's controller, sorted by (power+toughness)
// descending. Stable sort keeps battlefield order on ties.
func cleopatraPickTargets(gs *gameengine.GameState, self *gameengine.Permanent) []*gameengine.Permanent {
	if gs == nil || self == nil || self.Controller < 0 || self.Controller >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[self.Controller]
	if s == nil {
		return nil
	}
	var own []*gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p == self || p.Card == nil {
			continue
		}
		if !p.IsCreature() || !p.IsLegendary() {
			continue
		}
		own = append(own, p)
	}
	sort.SliceStable(own, func(i, j int) bool {
		return own[i].Power()+own[i].Toughness() > own[j].Power()+own[j].Toughness()
	})
	if len(own) > 2 {
		own = own[:2]
	}
	return own
}

func cleopatraBetrayalDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cleopatra_betrayal_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil {
		return
	}
	// R49 batchD: self-betrayal now included. CR §603.10 LTB look-back
	// lets a creature's own dies-trigger see its pre-death state,
	// including counters. Cleopatra dying with counters draws the
	// controller cards (and they still lose 2 life).
	if dying.Card == nil {
		return
	}
	if !cardHasType(dying.Card, "legendary") || !cardHasType(dying.Card, "creature") {
		return
	}
	total := 0
	for _, n := range dying.Counters {
		if n > 0 {
			total += n
		}
	}
	if total <= 0 {
		return
	}
	owner := gs.Seats[perm.Controller]
	if owner == nil || owner.Lost {
		return
	}
	drawn := 0
	for i := 0; i < total; i++ {
		c := drawOne(gs, perm.Controller, perm.Card.DisplayName())
		if c == nil {
			break
		}
		drawn++
	}
	gameengine.LoseLife(gs, perm.Controller, 2, perm.Card.DisplayName())
	_ = gs.CheckEnd()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"dying":         dying.Card.DisplayName(),
		"counter_total": total,
		"drawn":         drawn,
		"life_lost":     2,
	})
}
