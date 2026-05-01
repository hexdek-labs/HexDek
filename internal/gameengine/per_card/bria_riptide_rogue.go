package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBriaRiptideRogue wires Bria, Riptide Rogue.
//
// Oracle text (Bloomburrow, {2}{U}{R}, 3/3 Legendary Otter Rogue):
//
//	Prowess (Whenever you cast a noncreature spell, this creature gets
//	+1/+1 until end of turn.)
//	Other creatures you control have prowess.
//	Whenever you cast a noncreature spell, target creature you control
//	can't be blocked this turn.
//
// Implementation:
//   - OnETB: stamp kw:prowess on Bria herself (covers test fixtures
//     without an AST) and on every other creature already on the
//     controller's battlefield. cast_counts.go reads HasKeyword("prowess")
//     to fire the +1/+1, so the grant is automatic from there.
//   - OnTrigger("permanent_etb"): when a creature enters under Bria's
//     controller, stamp kw:prowess on it so the grant covers post-ETB
//     creatures too.
//   - OnTrigger("noncreature_spell_cast"): pick the best attacker we
//     control (highest power, prefer not Bria so we can swing the team)
//     and stamp the unblockable flag for the turn. Honors the printed
//     "target creature you control" — emitFail when no creatures.
func registerBriaRiptideRogue(r *Registry) {
	r.OnETB("Bria, Riptide Rogue", briaRiptideRogueETB)
	r.OnTrigger("Bria, Riptide Rogue", "permanent_etb", briaRiptideRogueGrantOnETB)
	r.OnTrigger("Bria, Riptide Rogue", "noncreature_spell_cast", briaRiptideRogueUnblockable)
}

func briaRiptideRogueETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "bria_riptide_rogue_prowess_grant"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stampProwess(perm)
	granted := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() {
			continue
		}
		stampProwess(p)
		granted++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"granted":  granted,
		"keyword":  "prowess",
	})
}

func briaRiptideRogueGrantOnETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entry, _ := ctx["perm"].(*gameengine.Permanent)
	if entry == nil || entry == perm || !entry.IsCreature() {
		return
	}
	stampProwess(entry)
}

func briaRiptideRogueUnblockable(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bria_riptide_rogue_unblockable"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Pick the best attacker: highest power creature; prefer not Bria
	// herself so the team can chain the buff. Tiebreak by earliest
	// timestamp.
	var best *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		pow := p.Power()
		switch {
		case best == nil:
			best = p
			bestPow = pow
		case pow > bestPow:
			best = p
			bestPow = pow
		case pow == bestPow:
			// Prefer non-Bria over Bria.
			if best == perm && p != perm {
				best = p
			} else if p != perm && best != perm && p.Timestamp < best.Timestamp {
				best = p
			}
		}
	}
	if best == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_creatures", nil)
		return
	}
	if best.Flags == nil {
		best.Flags = map[string]int{}
	}
	best.Flags["unblockable"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": best.Card.DisplayName(),
		"power":  bestPow,
	})
}

// stampProwess marks a permanent as having the prowess keyword via the
// runtime flag channel that combat.HasKeyword honors.
func stampProwess(p *gameengine.Permanent) {
	if p == nil {
		return
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	if _, ok := p.Flags["kw:"+strings.ToLower("prowess")]; ok {
		return
	}
	p.Flags["kw:prowess"] = 1
}
