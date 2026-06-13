package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// draw_lose_life_mr_r60.go — per_card handlers for shard M-R
// "draw X, lose X life (X = some board count)" spells.
//
// Both parse to inert nodes whose X source the text fallback cannot
// recover: Minions' Murmurs reduces to "draw X, lose X life" (the
// "where X is the number of creatures you control" clause is dropped),
// which the runtime reResDrawXLoseX matcher resolves with X defaulting
// to 1 (it can't parse the literal "X") — wrong; Monumental Corruption's
// "target player draws X … loses X life" doesn't match any shape at all
// (drew/lost ZERO). These handlers compute the correct X and route the
// draw through the shared gameengine.DrawN primitive.
//
// One new self-registering file (init() + AddResetHook); no shared
// registry edits.
func init() {
	registerDrawLoseLifeMRR60(Global())
	AddResetHook(registerDrawLoseLifeMRR60)
}

func registerDrawLoseLifeMRR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Minions' Murmurs", minionsMurmursResolve)
	r.OnResolve("Monumental Corruption", monumentalCorruptionResolve)
}

func countControlled(gs *gameengine.GameState, seat int, pred func(*gameengine.Permanent) bool) int {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && pred(p) {
			n++
		}
	}
	return n
}

func spellSource(item *gameengine.StackItem) *gameengine.Permanent {
	if item == nil || item.Card == nil {
		return nil
	}
	return &gameengine.Permanent{Card: item.Card, Controller: item.Controller, Owner: item.Card.Owner}
}

// Minions' Murmurs — you draw X and lose X life, X = creatures you control.
func minionsMurmursResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "minions_murmurs"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	x := countControlled(gs, seat, func(p *gameengine.Permanent) bool { return p.IsCreature() })
	drawn := gameengine.DrawN(gs, seat, x, spellSource(item))
	if x > 0 {
		gameengine.LoseLife(gs, seat, x, "Minions' Murmurs")
	}
	emit(gs, slug, "Minions' Murmurs", map[string]interface{}{"seat": seat, "x": x, "drawn": drawn})
}

// Monumental Corruption — target player draws X and loses X life,
// X = artifacts you control. Hat policy: aim the symmetric draw+drain at
// the opponent with the lowest life (closest to lethal); the artifact
// count is the CASTER's.
func monumentalCorruptionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "monumental_corruption"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	x := countControlled(gs, seat, func(p *gameengine.Permanent) bool { return p.IsArtifact() })

	// Lowest-life opponent (the life loss is the relevant lever).
	target := -1
	low := 1 << 30
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		if os.Life < low {
			low = os.Life
			target = opp
		}
	}
	if target < 0 {
		emitFail(gs, slug, "Monumental Corruption", "no_target", nil)
		return
	}
	drawn := gameengine.DrawN(gs, target, x, spellSource(item))
	if x > 0 {
		gameengine.LoseLife(gs, target, x, "Monumental Corruption")
	}
	emit(gs, slug, "Monumental Corruption", map[string]interface{}{
		"seat": seat, "target_seat": target, "x": x, "drawn": drawn,
	})
}
