package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFinalParting wires Final Parting.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Final%20Parting):
//
//	Search your library for two cards, put one into your hand and the
//	other into your graveyard, then shuffle.
//
// {3}{B} Sorcery. The hybrid tutor — fetch a tutor target to hand AND
// stage a graveyard piece in a single card. Classic reanimator line:
// hand gets Reanimate / Animate Dead / Necromancy; graveyard gets the
// fat reanimation target (Razaketh, Griselbrand, Atraxa Grand Unifier).
// The two-card split is what makes it stronger than vanilla Diabolic
// Tutor — you trade 1 mana and instant speed for guaranteed two-card
// value.
//
// Implementation:
//   - OnResolve. Two picker passes:
//     (a) hand pick — the canonical "actually cast this" card. Prefer
//         tutors / wincons / interaction (proxy: instant or sorcery
//         with CMC <= 3, then any spell card). Concretely we score
//         instants/sorceries highest, then artifacts/enchantments,
//         then high-CMC creatures last (since creatures are usually
//         the graveyard pick).
//     (b) graveyard pick — the fat reanimation target. Pick the
//         highest-CMC creature in library AFTER the hand pick is
//         removed. Most reanimator decks pack 4-6 huge bodies; pick
//         the highest CMC available.
//   - MoveCard library→hand for pick 1, library→graveyard for pick 2.
//   - Shuffle library after both moves.
//   - Empty library / no creature targets: still legal — printed
//     "Search your library for two cards" allows finding fewer.
func registerFinalParting(r *Registry) {
	r.OnResolve("Final Parting", finalPartingResolve)
}

func finalPartingResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "final_parting"
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

	// Pick hand-target first: instant/sorcery prioritized, else
	// artifact/enchantment, else lowest-CMC creature (high-CMC
	// creatures go to graveyard).
	handIdx := pickFinalPartingHandTarget(s.Library)
	var handCard *gameengine.Card
	if handIdx >= 0 {
		handCard = s.Library[handIdx]
		s.Library = append(s.Library[:handIdx], s.Library[handIdx+1:]...)
		gameengine.MoveCard(gs, handCard, seat, "library", "hand", slug)
	}

	// Pick graveyard-target second: highest-CMC creature (reanimator
	// fat). If no creature, take any highest-CMC card.
	graveIdx := pickFinalPartingGraveyardTarget(s.Library)
	var graveCard *gameengine.Card
	if graveIdx >= 0 {
		graveCard = s.Library[graveIdx]
		s.Library = append(s.Library[:graveIdx], s.Library[graveIdx+1:]...)
		gameengine.MoveCard(gs, graveCard, seat, "library", "graveyard", slug)
	}

	shuffleLibraryPerCard(gs, seat)

	details := map[string]interface{}{
		"seat":      seat,
		"hand":      cardNameOr(handCard, ""),
		"graveyard": cardNameOr(graveCard, ""),
	}
	if handCard == nil && graveCard == nil {
		emitFail(gs, slug, "Final Parting", "no_legal_target", details)
		return
	}
	emit(gs, slug, "Final Parting", details)
}

// pickFinalPartingHandTarget scans library and returns index of best
// hand-bound card (tutors/answers to re-cast). Tier policy:
//   spell (instant/sorcery): 100 + CMC
//   artifact/enchantment:    50 + CMC
//   creature:                CMC
//   land/planeswalker/etc:   10 + CMC (catch-all)
func pickFinalPartingHandTarget(lib []*gameengine.Card) int {
	bestIdx := -1
	bestScore := -1
	for i, c := range lib {
		if c == nil {
			continue
		}
		score := cardCMC(c)
		switch {
		case isInstantOrSorcery(c):
			score += 100
		case isArtifactOrEnchantment(c):
			score += 50
		case isCreatureCard(c):
			// Plain creature score — no boost. Used as fallback.
		default:
			score += 10
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return bestIdx
}

// pickFinalPartingGraveyardTarget scans library for the highest-CMC
// creature (canonical reanimator target). Falls back to highest-CMC
// any card type when no creature is available.
func pickFinalPartingGraveyardTarget(lib []*gameengine.Card) int {
	bestCreatureIdx := -1
	bestCreatureCMC := -1
	bestAnyIdx := -1
	bestAnyCMC := -1
	for i, c := range lib {
		if c == nil {
			continue
		}
		cmc := cardCMC(c)
		if isCreatureCard(c) && cmc > bestCreatureCMC {
			bestCreatureCMC = cmc
			bestCreatureIdx = i
		}
		if cmc > bestAnyCMC {
			bestAnyCMC = cmc
			bestAnyIdx = i
		}
	}
	if bestCreatureIdx >= 0 {
		return bestCreatureIdx
	}
	return bestAnyIdx
}

func cardNameOr(c *gameengine.Card, fallback string) string {
	if c == nil {
		return fallback
	}
	return c.DisplayName()
}
