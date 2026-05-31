package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Atris, Oracle of Half-Truths — {2}{U}{B} 3/2 Legendary Creature — Human
// Wizard.
//
//   Menace
//   When Atris enters, target opponent looks at the top three cards of
//   your library and separates them into a face-down pile and a face-up
//   pile. Put one pile into your hand and the other into your graveyard.
//
// MVP heuristic — two AI-vs-AI decisions to make:
//   (1) Opponent's split: model an adversarial opponent. Sort the top 3
//       by CMC descending; pair-vs-singleton split where pile A = the
//       single highest-CMC card, pile B = the other two. This is the
//       canonical "bad-but-tempting" split — opponent forces Atris's
//       controller to choose between one juicy card vs two cheaper ones.
//   (2) Controller's pick: greedy — take the pile with the higher
//       aggregate CMC (more raw card value). Other pile goes to graveyard.
//
// Menace is keyword-pipeline. Library smaller than 3 — handle gracefully
// by working with whatever is on top.

func init() {
	registerAtrisOracleOfHalfTruths(Global())
	AddResetHook(registerAtrisOracleOfHalfTruths)
}

func registerAtrisOracleOfHalfTruths(r *Registry) {
	r.OnETB("Atris, Oracle of Half-Truths", atrisETB)
}

func atrisETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "atris_separate_piles"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	top := s.Library
	n := len(top)
	if n == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"reason": "library_empty",
		})
		return
	}
	if n > 3 {
		n = 3
	}
	revealed := append([]*gameengine.Card(nil), top[:n]...)
	// Opponent split: highest-CMC alone in pile A, others in pile B.
	highest := revealed[0]
	highestIdx := 0
	for i, c := range revealed {
		if c == nil {
			continue
		}
		if cardCMC(c) > cardCMC(highest) {
			highest = c
			highestIdx = i
		}
	}
	pileA := []*gameengine.Card{highest}
	pileB := []*gameengine.Card{}
	for i, c := range revealed {
		if i == highestIdx {
			continue
		}
		pileB = append(pileB, c)
	}
	// Controller's pick: aggregate-CMC greedy.
	cmcA := 0
	for _, c := range pileA {
		cmcA += cardCMC(c)
	}
	cmcB := 0
	for _, c := range pileB {
		cmcB += cardCMC(c)
	}
	toHand, toGraveyard := pileA, pileB
	if cmcB > cmcA {
		toHand, toGraveyard = pileB, pileA
	}
	for _, c := range toHand {
		if c == nil {
			continue
		}
		moveCardBetweenZones(gs, seat, c, "library", "hand", "atris_pile_to_hand")
	}
	for _, c := range toGraveyard {
		if c == nil {
			continue
		}
		moveCardBetweenZones(gs, seat, c, "library", "graveyard", "atris_pile_to_graveyard")
	}
	handNames := []string{}
	gyNames := []string{}
	for _, c := range toHand {
		if c != nil {
			handNames = append(handNames, c.DisplayName())
		}
	}
	for _, c := range toGraveyard {
		if c != nil {
			gyNames = append(gyNames, c.DisplayName())
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"revealed_count": len(revealed),
		"to_hand":        handNames,
		"to_graveyard":   gyNames,
	})
}
