package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// destroy_two_mr_r60.go — shard M-R "destroy two target X" removal that
// parsed to inert parsed_effect_residual nodes (no structured Destroy)
// and destroyed NOTHING:
//
//   - Peace and Quiet: destroy two target enchantments
//   - Rack and Ruin:   destroy two target artifacts
//   - Rain of Salt:    destroy two target lands
//
// Hat policy: destroy up to two matching permanents controlled by
// opponents (never the caster's own), each via DestroyPermanent
// (indestructible check, §614 replacements, dies/LTB triggers, commander
// redirect). One new self-registering file (init() + AddResetHook); no
// shared registry edits.
func init() {
	registerDestroyTwoMRR60(Global())
	AddResetHook(registerDestroyTwoMRR60)
}

func registerDestroyTwoMRR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Peace and Quiet", destroyTwoFunc("Peace and Quiet", "peace_and_quiet",
		func(p *gameengine.Permanent) bool { return p.IsEnchantment() }))
	r.OnResolve("Rack and Ruin", destroyTwoFunc("Rack and Ruin", "rack_and_ruin",
		func(p *gameengine.Permanent) bool { return p.IsArtifact() }))
	r.OnResolve("Rain of Salt", destroyTwoFunc("Rain of Salt", "rain_of_salt",
		func(p *gameengine.Permanent) bool { return p.IsLand() }))
}

func destroyTwoFunc(card, slug string, match func(*gameengine.Permanent) bool) func(*gameengine.GameState, *gameengine.StackItem) {
	return func(gs *gameengine.GameState, item *gameengine.StackItem) {
		if gs == nil || item == nil {
			return
		}
		seat := item.Controller
		if seat < 0 || seat >= len(gs.Seats) {
			return
		}
		// Snapshot up to two matching opponent permanents.
		var targets []*gameengine.Permanent
		for _, opp := range gs.Opponents(seat) {
			s := gs.Seats[opp]
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p != nil && match(p) {
					targets = append(targets, p)
					if len(targets) == 2 {
						break
					}
				}
			}
			if len(targets) == 2 {
				break
			}
		}
		if len(targets) == 0 {
			emitFail(gs, slug, card, "no_legal_target", nil)
			return
		}
		destroyed := 0
		for _, p := range targets {
			if gameengine.DestroyPermanent(gs, p, nil) {
				destroyed++
			}
		}
		gameengine.StateBasedActions(gs)
		emit(gs, slug, card, map[string]interface{}{"seat": seat, "destroyed": destroyed})
	}
}
