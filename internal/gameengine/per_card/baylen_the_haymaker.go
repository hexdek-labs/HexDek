package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBaylenTheHaymaker wires Baylen, the Haymaker.
//
// Oracle text:
//
//	Tap two untapped tokens you control: Add one mana of any color.
//	Tap three untapped tokens you control: Draw a card.
//	Tap four untapped tokens you control: Put three +1/+1 counters on
//	Baylen. It gains trample until end of turn.
//
// Implementation (R53 batch N port):
//   - OnActivated with three abilityIdx variants (0/1/2). Each gates
//     on having enough untapped tokens the controller controls.
//   - Ability 0: tap 2 untapped tokens → add 1 mana (default U for
//     the engine's untyped pool, matching the Galazeth pattern).
//   - Ability 1: tap 3 untapped tokens → draw 1 card.
//   - Ability 2: tap 4 untapped tokens → 3× +1/+1 counters on Baylen
//     + kw:trample + trample_until_eot for the cleanup pass.
func registerBaylenTheHaymaker(r *Registry) {
	r.OnActivated("Baylen, the Haymaker", baylenActivated)
}

func baylenActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "baylen_token_tap_ability"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}
	var need int
	switch abilityIdx {
	case 0:
		need = 2
	case 1:
		need = 3
	case 2:
		need = 4
	default:
		return
	}
	tokens := baylenPickUntappedTokens(seat, need)
	if len(tokens) < need {
		emitFail(gs, slug, src.Card.DisplayName(), "not_enough_untapped_tokens", map[string]interface{}{
			"need": need,
			"have": len(tokens),
		})
		return
	}
	for _, t := range tokens {
		t.Tapped = true
	}
	switch abilityIdx {
	case 0:
		gameengine.AddManaFromPermanent(gs, seat, src, "U", 1)
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":          src.Controller,
			"tokens_tapped": need,
			"mana_added":    1,
		})
	case 1:
		drawOne(gs, src.Controller, src.Card.DisplayName())
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":          src.Controller,
			"tokens_tapped": need,
			"drew":          1,
		})
	case 2:
		src.AddCounter("+1/+1", 3)
		if src.Flags == nil {
			src.Flags = map[string]int{}
		}
		src.Flags["kw:trample"] = 1
		src.Flags["trample_until_eot"] = 1
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":           src.Controller,
			"tokens_tapped":  need,
			"counters_added": 3,
			"trample_eot":    true,
		})
	}
}

func baylenPickUntappedTokens(seat *gameengine.Seat, need int) []*gameengine.Permanent {
	if seat == nil || need <= 0 {
		return nil
	}
	out := make([]*gameengine.Permanent, 0, need)
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.Tapped {
			continue
		}
		if !cardHasType(p.Card, "token") {
			continue
		}
		out = append(out, p)
		if len(out) >= need {
			break
		}
	}
	return out
}
