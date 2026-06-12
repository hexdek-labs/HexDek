package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLyzoldaTheBloodWitch wires Lyzolda, the Blood Witch.
//
// Oracle text (Scryfall, verified vs ast_dataset 2026-06-12):
//
//	{2}, Sacrifice a creature: Lyzolda deals 2 damage to any target if
//	the sacrificed creature was red. Draw a card if the sacrificed
//	creature was black.
//
// Goldilocks A-M dead-effect backlog (round 2). Unblocked by the r63
// sacrificed_perm ctx threading (commit 6bc9caab): the activation
// dispatcher settles {2} + the sacrifice and hands the victim to the
// handler, which resolves the color-conditional riders. When the
// dispatcher didn't run (goldilocks InvokeActivatedHook, per_card-only
// activations), the handler pays the sacrifice itself — preferring a
// red or black victim, never Lyzolda — so the cost-and-effect pair
// stays atomic on both paths.
func registerLyzoldaTheBloodWitch(r *Registry) {
	r.OnActivated("Lyzolda, the Blood Witch", lyzoldaBloodWitchActivate)
}

func init() {
	registerLyzoldaTheBloodWitch(Global())
	AddResetHook(registerLyzoldaTheBloodWitch)
}

func lyzoldaBloodWitchActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "lyzolda_blood_witch_sac"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	// Victim: dispatcher-paid (real activation path) or handler-paid
	// (per_card-only / goldilocks path).
	var victim *gameengine.Permanent
	if ctx != nil {
		victim, _ = ctx["sacrificed_perm"].(*gameengine.Permanent)
	}
	if victim == nil {
		victim = lyzoldaPickSacVictim(gs, seat, src)
		if victim == nil {
			emitFail(gs, slug, src.Card.DisplayName(), "no_creature_to_sacrifice", map[string]interface{}{
				"seat": seat,
			})
			return
		}
		gameengine.SacrificePermanent(gs, victim, "lyzolda_activation_cost")
	}

	isRed, isBlack := false, false
	if victim.Card != nil {
		for _, c := range victim.Card.Colors {
			switch c {
			case "R":
				isRed = true
			case "B":
				isBlack = true
			}
		}
	}

	if isRed {
		// "any target" — greedy policy: first living opponent's face.
		for _, opp := range gs.Opponents(seat) {
			if gs.Seats[opp] != nil && !gs.Seats[opp].Lost {
				gameengine.DealDamage(gs, opp, 2, src.Card.DisplayName())
				break
			}
		}
	}
	if isBlack {
		drawOne(gs, seat, src.Card.DisplayName())
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"sacrificed": victim.Card.DisplayName(),
		"was_red":    isRed,
		"was_black":  isBlack,
	})
}

// lyzoldaPickSacVictim picks the handler-paid sacrifice: a red or black
// creature when one is available (both riders beat neither), never
// Lyzolda herself unless she is the only creature.
func lyzoldaPickSacVictim(gs *gameengine.GameState, seat int, src *gameengine.Permanent) *gameengine.Permanent {
	s := gs.Seats[seat]
	var fallback *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() || p == src {
			continue
		}
		for _, c := range p.Card.Colors {
			if c == "R" || c == "B" {
				return p
			}
		}
		if fallback == nil {
			fallback = p
		}
	}
	return fallback
}
