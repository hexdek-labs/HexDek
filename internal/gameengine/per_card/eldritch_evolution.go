package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEldritchEvolution wires Eldritch Evolution.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Eldritch%20Evolution):
//
//	As an additional cost to cast this spell, sacrifice a creature.
//	Search your library for a creature card with mana value equal to
//	the sacrificed creature's mana value plus 2 or less, put it onto
//	the battlefield, then shuffle. Exile Eldritch Evolution.
//
// {1}{G/P} Sorcery. The premier creature-to-creature transformer. The
// canonical line in Persist combo decks: sac a 2-CMC dork, tutor up
// the 4-CMC combo piece (Anafenza/Melira-pattern persist combos), or
// sac the commander to grab a 4-CMC engine. Different from Birthing
// Pod in that the sac happens AT CAST, not as an activated cost — so
// observers see the sacrifice during cast resolution, NOT before
// targets are chosen.
//
// Implementation:
//   - OnResolve. The sac was an additional cost paid at cast time;
//     by resolve, the sacrificed creature's mana value should already
//     be recorded on item.CostMeta["sac_cmc"]. When the metadata is
//     absent (test fixtures synthesizing a StackItem directly), fall
//     back to the lowest-CMC creature on the controller's battlefield
//     as the implied sacrifice for picker-CMC purposes.
//   - Picker: highest-CMC creature in library matching CMC <= sac_cmc + 2.
//     The "+2" includes the sac'd creature's CMC, so a 2-CMC sac tutors
//     anything up to and including 4-CMC.
//   - MoveCard library→battlefield with full ETB observer firing via
//     enterBattlefieldWithETB; library shuffles.
//   - Self-exile via item.CostMeta["exile_on_resolve"] = true so the
//     spell goes to exile instead of graveyard per CR §608.2g override.
func registerEldritchEvolution(r *Registry) {
	r.OnResolve("Eldritch Evolution", eldritchEvolutionResolve)
}

func eldritchEvolutionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "eldritch_evolution"
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

	// Mark for exile-instead-of-graveyard so the post-resolve dispatch
	// in stack.go ShouldExileOnResolve routes correctly. Stamp even on
	// no-find paths since the printed text exiles unconditionally.
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["exile_on_resolve"] = true

	// Recover the sacrificed creature's CMC from the cast-cost record.
	// Test fixtures that synthesize the StackItem directly may not have
	// stamped sac_cmc — fall back to the lowest-CMC creature on the
	// controller's battlefield as the implied "would-be sacrifice."
	sacCMC := -1
	if v, ok := item.CostMeta["sac_cmc"]; ok {
		if n, ok := v.(int); ok {
			sacCMC = n
		}
	}
	if sacCMC < 0 {
		lowest := -1
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			c := cardCMC(p.Card)
			if lowest < 0 || c < lowest {
				lowest = c
			}
		}
		if lowest >= 0 {
			sacCMC = lowest
		} else {
			sacCMC = 0
		}
	}
	cap := sacCMC + 2

	// Pick highest-CMC creature in library at or under the cap.
	bestIdx := -1
	bestCMC := -1
	for i, c := range s.Library {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > cap {
			continue
		}
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, "Eldritch Evolution", "no_legal_target", map[string]interface{}{
			"sac_cmc":   sacCMC,
			"cap_cmc":   cap,
			"lib_size":  len(s.Library),
		})
		return
	}
	card := s.Library[bestIdx]
	s.Library = append(s.Library[:bestIdx], s.Library[bestIdx+1:]...)
	shuffleLibraryPerCard(gs, seat)
	enterBattlefieldWithETB(gs, seat, card, false)

	emit(gs, slug, "Eldritch Evolution", map[string]interface{}{
		"seat":         seat,
		"sac_cmc":      sacCMC,
		"cap_cmc":      cap,
		"into_play":    card.DisplayName(),
		"target_cmc":   bestCMC,
	})
}
