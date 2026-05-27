package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFaithlessLooting wires Faithless Looting.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Faithless%20Looting):
//
//	Draw two cards, then discard two cards.
//	Flashback {2}{R}
//
// {R} Sorcery. The premier red loot — turns a card into 2 known
// useful cards, with the flashback giving the spell a second life
// from the graveyard. Backbone of every red dredge / madness /
// graveyard-recursion shell.
//
// Implementation:
//   - OnResolve: draw two, then discard two. The flashback alt-cost
//     is handled by the engine's keyword pipeline (Flashback is in
//     the AST keyword set); this resolve handler is the same shape
//     whether cast from hand or graveyard.
//   - Discard heuristic: pick the lowest-keep-score cards in hand
//     using sylvanCardKeepScore (lands score 0, 1-CMC cards score 1,
//     bombs score 4 — same ranking used by Sylvan Library, Scroll
//     Rack, etc.). Critically, the freshly-drawn cards are at the
//     END of the hand (drawOne appends), so we score the whole hand
//     and pick the 2 worst — which is the correct "loot for value"
//     decision (don't auto-discard the new cards we just drew if
//     older cards are worse).
func registerFaithlessLooting(r *Registry) {
	r.OnResolve("Faithless Looting", faithlessLootingResolve)
}

func faithlessLootingResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "faithless_looting"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Draw 2.
	drew := 0
	for i := 0; i < 2; i++ {
		if drawOne(gs, seat, "Faithless Looting") != nil {
			drew++
		}
	}

	// Discard 2. Score the entire hand by sylvanCardKeepScore and
	// discard the 2 lowest-scoring cards.
	type ranked struct {
		card  *gameengine.Card
		score int
	}
	ranks := make([]ranked, 0, len(s.Hand))
	for _, c := range s.Hand {
		if c == nil {
			continue
		}
		ranks = append(ranks, ranked{card: c, score: sylvanCardKeepScore(c)})
	}
	// Selection sort ascending.
	for i := 0; i < len(ranks)-1; i++ {
		for j := i + 1; j < len(ranks); j++ {
			if ranks[j].score < ranks[i].score {
				ranks[i], ranks[j] = ranks[j], ranks[i]
			}
		}
	}

	discarded := 0
	discardedNames := []string{}
	for i := 0; i < 2 && i < len(ranks); i++ {
		gameengine.DiscardCard(gs, ranks[i].card, seat)
		discarded++
		discardedNames = append(discardedNames, ranks[i].card.DisplayName())
	}

	emit(gs, slug, "Faithless Looting", map[string]interface{}{
		"seat":      seat,
		"drew":      drew,
		"discarded": discarded,
		"discards":  discardedNames,
	})
}
