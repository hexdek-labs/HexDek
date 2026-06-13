package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// whispering_madness_r63.go — per_card handler for Whispering Madness.
//
// Oracle text (Scryfall):
//
//	Each player discards their hand, then draws cards equal to the
//	greatest number of cards a player discarded this way.
//	Cipher (…)
//
// {2}{U}{B} Sorcery. A symmetric Dimir wheel whose draw count is the
// MAX hand size discarded across the table — the player with the fattest
// hand sets everyone's redraw. The cross-player "greatest number
// discarded" computation has no structured AST node, so the generic
// dispatch logs it inert and no one discards or draws (DOA).
//
// Implementation (OnResolve; the per_card resolve hook is the
// authoritative spell body):
//   - Pass 1: every living player discards their whole hand (canonical
//     DiscardCard, so discard triggers fire); record each player's count.
//   - n = max of those counts.
//   - Pass 2: every living player draws n (bounded by library size).
//
// Cipher is the card's SEPARATE encode ability (exile-on-a-creature,
// recast-on-combat-damage), not part of this resolve body; it is out of
// scope for this DOA fix and unaffected.
func init() {
	registerWhisperingMadnessR63(Global())
	AddResetHook(registerWhisperingMadnessR63)
}

func registerWhisperingMadnessR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Whispering Madness", whisperingMadnessResolve)
}

func whisperingMadnessResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "whispering_madness"
	if gs == nil || item == nil {
		return
	}
	// Pass 1: each player discards their hand; track the greatest count.
	maxDiscarded := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		discarded := len(s.Hand)
		for len(gs.Seats[i].Hand) > 0 {
			gameengine.DiscardCard(gs, gs.Seats[i].Hand[0], i)
		}
		if discarded > maxDiscarded {
			maxDiscarded = discarded
		}
	}
	// Pass 2: each player draws the greatest-discarded count.
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for j := 0; j < maxDiscarded && len(gs.Seats[i].Library) > 0; j++ {
			top := gs.Seats[i].Library[0]
			gameengine.MoveCard(gs, top, i, "library", "hand", "draw")
		}
	}
	emit(gs, slug, "Whispering Madness", map[string]interface{}{
		"seat":          item.Controller,
		"max_discarded": maxDiscarded,
	})
}
