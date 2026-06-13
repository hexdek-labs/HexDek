package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Twinflame — {1}{R} Sorcery.
//
//	Strive — This spell costs {2}{R} more for each target beyond the first.
//	Choose any number of target creatures you control. For each of them,
//	create a token that's a copy of that creature, except it has haste.
//	Exile those tokens at the beginning of the next end step.
//
// A popular Strive copy spell (33 decks). The whole effect parsed to one
// inert `custom` slug, so it resolved to a no-op. This handler creates a
// hasty token copy of each targeted creature the caster controls and
// schedules them for exile at the next end step. (Strive's extra cost is a
// cast-time concern handled elsewhere; the chosen targets arrive in
// item.Targets.)
func init() {
	registerTwinflame(Global())
	AddResetHook(registerTwinflame)
}

func registerTwinflame(r *Registry) {
	r.OnResolve("Twinflame", twinflameResolve)
}

func twinflameResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "twinflame"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	var made []*gameengine.Permanent
	for _, t := range item.Targets {
		if t.Kind != gameengine.TargetKindPermanent || t.Permanent == nil {
			continue
		}
		src := t.Permanent
		if src.Card == nil || !src.IsCreature() || src.Controller != seat {
			continue
		}
		cp := gameengine.MintTokenAsCopyOf(gs, src.Card, seat, gameengine.CurrentMintEnablerID(gs))
		if cp == nil {
			continue
		}
		cp.IsCopy = true
		tok := &gameengine.Permanent{
			Card:          cp,
			Controller:    seat,
			Owner:         seat,
			SummoningSick: false, // "except it has haste"
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{"twinflame_copy": 1},
		}
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, tok)
		gameengine.RegisterReplacementsForPermanent(gs, tok)
		gameengine.FirePermanentETBTriggers(gs, tok)
		made = append(made, tok)
	}
	if len(made) > 0 {
		tokens := made
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: seat,
			SourceCardName: "Twinflame (token exile)",
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				for _, tok := range tokens {
					if tok != nil {
						gameengine.ExilePermanent(gs, tok, nil)
					}
				}
			},
		})
	}
	emit(gs, slug, "Twinflame", map[string]interface{}{
		"seat":   seat,
		"copies": len(made),
	})
}
