package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBraidsArisenNightmare wires Braids, Arisen Nightmare.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	At the beginning of your end step, you may sacrifice an artifact,
//	creature, enchantment, land, or planeswalker. If you do, each
//	opponent may sacrifice a permanent that shares a card type with it.
//	For each opponent who doesn't, that player loses 2 life and you
//	draw a card.
//
// Implementation:
//   - OnTrigger("end_step_controller"): fires only on the controller's
//     own end step (active_seat == perm.Controller). Controller
//     sacrifices the least-valuable non-Braids permanent from their
//     battlefield. Priority: tokens > lowest-CMC non-commander
//     creatures > artifacts > enchantments. Never sacrifice Braids herself.
//   - For each living opponent: check whether they control at least one
//     permanent that shares a card type with the sacrificed permanent.
//     If they do, they sacrifice their least-valuable matching permanent.
//     If they don't have a matching permanent (or we model them as
//     choosing not to sacrifice), they lose 2 life and Braids' controller
//     draws a card.
//   - "Your end step" gate: active_seat == perm.Controller (same pattern
//     as Wilhelt, Prosper, and other end-step handlers in this package).
func registerBraidsArisenNightmare(r *Registry) {
	r.OnTrigger("Braids, Arisen Nightmare", "end_step_controller", braidsArisenNightmareEndStep)
	// Also wire the plain "end_step" event in case the dispatcher uses it
	// (some callers fire "end_step" with active_seat in ctx).
	r.OnTrigger("Braids, Arisen Nightmare", "end_step", braidsArisenNightmareEndStep)
}

func braidsArisenNightmareEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "braids_arisen_nightmare_end_step"
	if gs == nil || perm == nil {
		return
	}

	// Gate: only fire on Braids' controller's own end step.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}

	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	s := gs.Seats[controller]
	if s == nil || s.Lost {
		return
	}

	// Choose the least-valuable permanent to sacrifice (never Braids herself).
	victim := braidsPickVictim(gs, controller, perm)
	if victim == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_sacrifice_candidate", map[string]interface{}{
			"seat": controller,
		})
		return
	}

	// Record the sacrificed card's types before removing it from the battlefield.
	sacrificedTypes := braidsCollectTypes(victim)
	sacrificedName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "braids_arisen_nightmare")

	// For each living opponent: if they have a permanent sharing a card type,
	// sacrifice their worst matching permanent. Otherwise lose 2 and controller draws.
	type oppResult struct {
		seat      int
		sacrified string
		lostLife  bool
		drew      bool
	}
	var results []oppResult

	opps := gs.Opponents(controller)
	drew := 0
	for _, oppSeat := range opps {
		if oppSeat < 0 || oppSeat >= len(gs.Seats) {
			continue
		}
		os := gs.Seats[oppSeat]
		if os == nil || os.Lost {
			continue
		}

		// Find the worst matching permanent on this opponent's battlefield.
		match := braidsOppPickVictim(gs, oppSeat, sacrificedTypes)
		if match != nil {
			// Opponent sacrifices their worst matching permanent.
			matchName := match.Card.DisplayName()
			gameengine.SacrificePermanent(gs, match, "braids_arisen_nightmare_opp")
			results = append(results, oppResult{
				seat:      oppSeat,
				sacrified: matchName,
			})
		} else {
			// No matching permanent (or opponent can't/doesn't sacrifice):
			// opponent loses 2 life, Braids' controller draws a card.
			os.Life -= 2
			gs.LogEvent(gameengine.Event{
				Kind:   "life_change",
				Seat:   oppSeat,
				Target: -1,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"amount": -2,
					"cause":  "braids_arisen_nightmare_no_sac",
				},
			})
			drawn := drawOne(gs, controller, perm.Card.DisplayName())
			drawnName := ""
			if drawn != nil {
				drawnName = drawn.DisplayName()
			}
			drew++
			results = append(results, oppResult{
				seat:      oppSeat,
				sacrified: "",
				lostLife:  true,
				drew:      drawnName != "",
			})
			_ = drawnName // used only for logging below
		}
	}

	// Build a summary for the emit.
	oppSummary := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		entry := map[string]interface{}{
			"seat":      r.seat,
			"sacrified": r.sacrified,
			"lost_life": r.lostLife,
		}
		oppSummary = append(oppSummary, entry)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             controller,
		"sacrificed":       sacrificedName,
		"sacrificed_types": sacrificedTypes,
		"drew":             drew,
		"opponents":        oppSummary,
	})
	_ = gs.CheckEnd()
}

// braidsPickVictim selects the least-valuable permanent on the controller's
// battlefield, excluding Braids herself. Scoring (higher = more desirable
// to sacrifice):
//
//   - Tokens always go first (+100) — pure upside to sac.
//   - Low CMC non-commander creatures are next (+50 - CMC).
//   - Artifacts (+20).
//   - Enchantments (+10).
//   - Lands (+5, last resort — never sac lands unless nothing else exists).
//   - Commanders never sacrificed (-1000 penalty).
//   - Braids herself excluded entirely.
func braidsPickVictim(gs *gameengine.GameState, seat int, braids *gameengine.Permanent) *gameengine.Permanent {
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}

	var best *gameengine.Permanent
	bestScore := -999999

	for _, p := range s.Battlefield {
		if p == nil || p == braids || p.Card == nil {
			continue
		}

		// Must be one of the five valid sacrifice types.
		if !braidsPermIsEligible(p) {
			continue
		}

		score := braidsVictimScore(gs, seat, p)

		if best == nil || score > bestScore {
			bestScore = score
			best = p
		}
	}
	return best
}

// braidsVictimScore rates how attractive p is as a sacrifice for Braids.
// Higher score = better to sacrifice (less valuable to keep).
func braidsVictimScore(gs *gameengine.GameState, seat int, p *gameengine.Permanent) int {
	score := 0

	// Tokens are pure fodder.
	if p.IsToken() {
		score += 100
	}

	// Penalise commanders heavily — almost never correct to feed a commander.
	if p.Card != nil && gameengine.IsCommanderCard(gs, seat, p.Card) {
		score -= 1000
	}

	// Prefer creature sacrifices when creature is summoning-sick (no combat value yet).
	if p.IsCreature() {
		cmc := cardCMC(p.Card)
		score += 50 - cmc // lower CMC = higher score (cheaper = less value loss)
		if p.SummoningSick {
			score += 10
		}
	}

	// Tapped lands are slightly better sacrifice fodder than untapped ones.
	if p.IsLand() {
		if p.Tapped {
			score += 6
		} else {
			score += 4
		}
	}

	// Artifacts are decent fodder.
	if p.IsArtifact() && !p.IsCreature() {
		cmc := cardCMC(p.Card)
		score += 20 - cmc
	}

	// Enchantments are decent fodder.
	if p.IsEnchantment() && !p.IsCreature() {
		cmc := cardCMC(p.Card)
		score += 10 - cmc
	}

	return score
}

// braidsPermIsEligible returns true if p is one of the five types Braids
// can sacrifice: artifact, creature, enchantment, land, or planeswalker.
func braidsPermIsEligible(p *gameengine.Permanent) bool {
	if p == nil {
		return false
	}
	return p.IsArtifact() || p.IsCreature() || p.IsEnchantment() || p.IsLand() || p.IsPlaneswalker()
}

// braidsCollectTypes returns the set of Braids-relevant types present on the
// permanent: "creature", "artifact", "enchantment", "land", "planeswalker".
func braidsCollectTypes(p *gameengine.Permanent) []string {
	if p == nil {
		return nil
	}
	var types []string
	if p.IsCreature() {
		types = append(types, "creature")
	}
	if p.IsArtifact() {
		types = append(types, "artifact")
	}
	if p.IsEnchantment() {
		types = append(types, "enchantment")
	}
	if p.IsLand() {
		types = append(types, "land")
	}
	if p.IsPlaneswalker() {
		types = append(types, "planeswalker")
	}
	return types
}

// braidsTypeMatches returns true if permanent p shares at least one of the
// listed Braids types.
func braidsTypeMatches(p *gameengine.Permanent, types []string) bool {
	for _, t := range types {
		if permIsType(p, t) {
			return true
		}
	}
	return false
}

// braidsOppPickVictim selects the worst matching permanent from an opponent's
// battlefield. Opponent wants to minimise loss, so they sacrifice the
// least-valuable matching permanent: tokens first, then lowest CMC, then
// tapped lands, etc.
func braidsOppPickVictim(gs *gameengine.GameState, oppSeat int, types []string) *gameengine.Permanent {
	s := gs.Seats[oppSeat]
	if s == nil {
		return nil
	}

	var best *gameengine.Permanent
	bestScore := -999999

	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !braidsTypeMatches(p, types) {
			continue
		}

		// Opponent perspective: higher score = better to sacrifice (cheaper to lose).
		score := 0

		// Tokens are always the first to go.
		if p.IsToken() {
			score += 100
		}

		// Tapped lands first among lands.
		if p.IsLand() {
			if p.Tapped {
				score += 6
			} else {
				score += 4
			}
		}

		// Low-CMC permanents are cheaper to give up.
		cmc := cardCMC(p.Card)
		score += 20 - cmc // lower CMC = higher score

		// Avoid sacrificing a commander.
		if gameengine.IsCommanderCard(gs, oppSeat, p.Card) {
			score -= 1000
		}

		if best == nil || score > bestScore {
			bestScore = score
			best = p
		}
	}
	return best
}
