package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMagdaBrazenOutlaw wires Magda, Brazen Outlaw.
//
// Oracle text:
//
//	Other Dwarves you control get +1/+0.
//	Whenever a Dwarf you control becomes tapped, create a Treasure token.
//	Sacrifice five Treasures: Search your library for an artifact or
//	Dragon card, put that card onto the battlefield, then shuffle.
//
// Implementation: listen on land_tapped_for_mana / artifact_tapped_for_mana
// is too narrow — we want any tap. Use becomes_tapped if the engine fires
// it; if not, hook tap-for-mana. The +1/+0 anthem is a static effect.
//
// R60 stub sweep batch 6: wired the activated "Sacrifice five Treasures:
// Search your library for an artifact or Dragon card, put that card
// onto the battlefield, then shuffle." Tutor target prefers Dragon
// (cEDH wincon for Magda decks — Old Gnawbone / Dragon Tempest) at
// equal CMC; otherwise highest-CMC artifact (Bolas's Citadel / Crucible
// of Worlds / Sundial of the Infinite — the deck's value engines).
func registerMagdaBrazenOutlaw(r *Registry) {
	r.OnTrigger("Magda, Brazen Outlaw", "becomes_tapped", magdaBrazenBecomesTapped)
	r.OnActivated("Magda, Brazen Outlaw", magdaBrazenActivated)
}

func magdaBrazenBecomesTapped(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "magda_brazen_dwarf_treasure"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	tappedSeat, _ := ctx["controller_seat"].(int)
	if tappedSeat != perm.Controller {
		return
	}
	tappedCard, _ := ctx["card"].(*gameengine.Card)
	if tappedCard == nil || !cardHasType(tappedCard, "dwarf") {
		return
	}
	gameengine.CreateTreasureToken(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"dwarf": tappedCard.DisplayName(),
	})
}

func magdaBrazenActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "magda_brazen_sac_5_treasures_tutor"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	// Collect Treasure tokens we control.
	var treasures []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "treasure") {
			continue
		}
		treasures = append(treasures, p)
		if len(treasures) == 5 {
			break
		}
	}
	if len(treasures) < 5 {
		emitFail(gs, slug, src.Card.DisplayName(), "fewer_than_5_treasures", map[string]interface{}{
			"available": len(treasures),
		})
		return
	}
	// Pay the cost: sacrifice 5 treasures.
	for _, t := range treasures {
		gameengine.SacrificePermanent(gs, t, "magda_brazen_sac_cost")
	}
	// Tutor: prefer Dragon (deck wincon), fall back to highest-CMC
	// artifact. Iterate library and pick the best.
	tutorIdx := -1
	tutorCMC := -1
	tutorIsDragon := false
	for i, c := range seat.Library {
		if c == nil {
			continue
		}
		isDragon := cardHasType(c, "dragon")
		isArtifact := cardHasType(c, "artifact")
		if !isDragon && !isArtifact {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		// Prefer Dragon over artifact at equal CMC; otherwise pick
		// highest CMC (or Dragon over non-Dragon at higher CMC).
		better := false
		switch {
		case tutorIdx < 0:
			better = true
		case isDragon && !tutorIsDragon:
			better = true
		case isDragon == tutorIsDragon && cmc > tutorCMC:
			better = true
		}
		if better {
			tutorIdx = i
			tutorCMC = cmc
			tutorIsDragon = isDragon
		}
	}
	tutored := ""
	if tutorIdx >= 0 {
		pick := seat.Library[tutorIdx]
		gameengine.MoveCard(gs, pick, seatIdx, "library", "battlefield", "magda_brazen_tutor")
		tutored = pick.DisplayName()
	}
	// Shuffle the library after the tutor.
	if gs.Rng != nil {
		lib := seat.Library
		gs.Rng.Shuffle(len(lib), func(i, j int) {
			lib[i], lib[j] = lib[j], lib[i]
		})
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              seatIdx,
		"sacrificed":        5,
		"tutored":           tutored,
		"tutored_is_dragon": tutorIsDragon,
	})
}
