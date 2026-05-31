package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPraetorsGrasp wires Praetor's Grasp.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Praetor%27s%20Grasp):
//
//	Search target opponent's library for a card and exile it face down.
//	Then that player shuffles. You may play that card for as long as it
//	remains exiled.
//
// {1}{B}{B} Sorcery. The premier "yoink opponent's best card" hate
// piece — exile a key combo line (Thassa's Oracle, Demonic Consultation,
// Worldgorger Dragon, Walking Ballista) and either deny the opponent
// or cast it yourself. cEDH staple in B-color shells.
//
// Implementation:
//   - OnResolve: pick target opponent (highest-threat opp by hand size
//     proxy — biggest hand likely most threatening), pick best card in
//     their library (highest CMC creature or 0-1 CMC interaction), move
//     it to controller's exile, register ZoneCastGrant for free cast
//     from exile, shuffle that opponent's library per §701.18b.
//
// Card picker heuristic for cEDH:
//   - Tier 1: known wincon names (Thassa's Oracle, Walking Ballista,
//     Aetherflux Reservoir, Underworld Breach, Doomsday)
//   - Tier 2: known tutors (Demonic Tutor, Vampiric Tutor, Mystical
//     Tutor, Enlightened Tutor)
//   - Tier 3: any noncreature non-land (the most likely "bomb" slot)
//   - Tier 4: any non-land
//
// "Face down" exile is modeled as plain exile in the controller's
// Exile zone — face-down identity preservation is a cosmetic
// engine-level concern not yet plumbed through Move semantics.
// emitPartial breadcrumbs the deviation.
func registerPraetorsGrasp(r *Registry) {
	r.OnResolve("Praetor's Grasp", praetorsGraspResolve)
}

func praetorsGraspResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "praetors_grasp"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Pick target opponent: largest hand as a heuristic for "most
	// threatening" (assumes hand size correlates with threat).
	targetOpp := -1
	bestHand := -1
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		if len(s.Hand) > bestHand {
			bestHand = len(s.Hand)
			targetOpp = i
		}
	}
	if targetOpp < 0 {
		emitFail(gs, slug, "Praetor's Grasp", "no_living_opponent", nil)
		return
	}

	oppLib := gs.Seats[targetOpp].Library
	if len(oppLib) == 0 {
		emitFail(gs, slug, "Praetor's Grasp", "opponent_library_empty", nil)
		return
	}

	pickIdx := praetorsGraspPickCard(oppLib)
	if pickIdx < 0 {
		// Fall back to first card.
		pickIdx = 0
	}
	picked := oppLib[pickIdx]

	// Remove from opp's library, append to controller's exile.
	// MoveCard handles the library removal. Prior manual splice before
	// this call was redundant (Wave 2 final-audit cleanup).
	gameengine.MoveCard(gs, picked, targetOpp, "library", "exile", "praetors_grasp_exile")

	// Shuffle target opponent's library per §701.18b.
	shuffleLibraryPerCard(gs, targetOpp)

	// Register ZoneCastGrant — free cast from exile, while exiled.
	freePerm := gameengine.NewFreeCastFromExilePermission(seat, "Praetor's Grasp")
	gameengine.RegisterZoneCastGrant(gs, picked, freePerm)

	emit(gs, slug, "Praetor's Grasp", map[string]interface{}{
		"seat":         seat,
		"target_opp":   targetOpp,
		"exiled_card":  picked.DisplayName(),
		"free_cast":    true,
	})
	emitPartial(gs, slug, "Praetor's Grasp",
		"face_down_exile_identity_not_modeled_treated_as_face_up")
}

func praetorsGraspPickCard(lib []*gameengine.Card) int {
	wincons := map[string]bool{
		"Thassa's Oracle":        true,
		"Walking Ballista":       true,
		"Aetherflux Reservoir":   true,
		"Underworld Breach":      true,
		"Doomsday":               true,
		"Laboratory Maniac":      true,
		"Jace, Wielder of Mysteries": true,
		"Demonic Consultation":   true,
		"Tainted Pact":           true,
	}
	tutors := map[string]bool{
		"Demonic Tutor":     true,
		"Vampiric Tutor":    true,
		"Mystical Tutor":    true,
		"Enlightened Tutor": true,
		"Worldly Tutor":     true,
		"Imperial Seal":     true,
		"Grim Tutor":        true,
	}
	// Tier 1: wincon.
	for i, c := range lib {
		if c == nil {
			continue
		}
		if wincons[c.DisplayName()] {
			return i
		}
	}
	// Tier 2: tutor.
	for i, c := range lib {
		if c == nil {
			continue
		}
		if tutors[c.DisplayName()] {
			return i
		}
	}
	// Tier 3: any noncreature non-land.
	for i, c := range lib {
		if c == nil {
			continue
		}
		hasCreature := false
		hasLand := false
		for _, t := range c.Types {
			if t == "creature" {
				hasCreature = true
			}
			if t == "land" {
				hasLand = true
			}
		}
		if !hasCreature && !hasLand {
			return i
		}
	}
	// Tier 4: any non-land.
	for i, c := range lib {
		if c == nil {
			continue
		}
		hasLand := false
		for _, t := range c.Types {
			if t == "land" {
				hasLand = true
				break
			}
		}
		if !hasLand {
			return i
		}
	}
	return -1
}
