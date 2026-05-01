package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArcumDagsson wires Arcum Dagsson.
//
// Oracle text:
//
//	{T}: Target artifact creature's controller sacrifices it. That player
//	may search their library for a noncreature artifact card, put it onto
//	the battlefield, then shuffle.
//
// One of the strongest artifact commanders ever printed: every tap turns
// a low-impact artifact creature (Ornithopter, Memnite, Phyrexian Walker,
// even an emptied Walking Ballista or Steel Overseer) into the best
// noncreature artifact in your deck (Mycosynth Lattice, Time Sieve,
// Aetherflux Reservoir, etc.). The shuffle hides the search.
//
// Implementation:
//   - OnActivated picks the LOWEST-CMC artifact creature controller
//     controls (preferring tokens). That's the card most worth trading
//     up. Self-target is allowed if no other artifact creature exists,
//     but that's rarely correct.
//   - Their controller sacrifices it (we route through SacrificePermanent
//     so §614 replacements + LTB triggers + commander redirect fire).
//   - That player searches their library for the highest-CMC noncreature
//     artifact card and puts it onto the battlefield via the full ETB
//     cascade. Library is then shuffled.
//
// Arcum's text targets ANY artifact creature, not just yours — but the
// activator picks the target. In simulation we always target a creature
// the activator (Arcum's controller) controls, since giving an opponent
// a free artifact tutor is strictly worse than the alternative.
func registerArcumDagsson(r *Registry) {
	r.OnActivated("Arcum Dagsson", arcumDagssonActivate)
}

func arcumDagssonActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "arcum_dagsson_artifact_tutor"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Pick target artifact creature on controller's battlefield.
	// Prefer tokens (cease-to-exist on sac), then lowest-CMC permanents.
	// Avoid sacrificing Arcum itself — leaves no body to keep tapping.
	var target *gameengine.Permanent
	bestScore := 1 << 30 // smaller is better
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsArtifact() || !p.IsCreature() {
			continue
		}
		score := gameengine.ManaCostOf(p.Card)
		if p.IsToken() {
			score -= 100 // tokens are pure upside to sac
		}
		if score < bestScore {
			bestScore = score
			target = p
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_artifact_creature_target", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	targetSeat := target.Controller
	targetCardName := target.Card.DisplayName()

	// Sacrifice the artifact creature (its controller does the sacrificing).
	gameengine.SacrificePermanent(gs, target, "arcum_dagsson")

	// That player (the sac'd creature's controller) searches their library
	// for a noncreature artifact and puts it onto the battlefield.
	if targetSeat < 0 || targetSeat >= len(gs.Seats) {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":         seat,
			"sacrificed":   targetCardName,
			"target_seat":  targetSeat,
			"tutored":      nil,
		})
		return
	}
	ts := gs.Seats[targetSeat]
	if ts == nil {
		return
	}

	foundIdx := -1
	bestCMC := -1
	for i, c := range ts.Library {
		if c == nil {
			continue
		}
		if !cardHasType(c, "artifact") {
			continue
		}
		if cardHasType(c, "creature") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			foundIdx = i
		}
	}

	if foundIdx < 0 {
		// No noncreature artifact found — still shuffle (you searched).
		shuffleLibraryPerCard(gs, targetSeat)
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"sacrificed":  targetCardName,
			"target_seat": targetSeat,
			"tutored":     nil,
		})
		return
	}

	tutored := ts.Library[foundIdx]
	ts.Library = append(ts.Library[:foundIdx], ts.Library[foundIdx+1:]...)
	shuffleLibraryPerCard(gs, targetSeat)
	enterBattlefieldWithETB(gs, targetSeat, tutored, false)

	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   targetSeat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"found_card": tutored.DisplayName(),
			"reason":     "arcum_dagsson",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"sacrificed":   targetCardName,
		"target_seat":  targetSeat,
		"tutored":      tutored.DisplayName(),
		"tutored_cmc":  bestCMC,
	})
	_ = gs.CheckEnd()
}
