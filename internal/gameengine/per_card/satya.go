package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSatya wires up Satya, Aetherflux Genius.
//
// Oracle text:
//
//	Whenever Satya, Aetherflux Genius attacks, create a tapped and
//	attacking token that's a copy of up to one other target nontoken
//	creature you control. You get {E}{E}. At the beginning of the
//	next end step, sacrifice that token unless you pay an amount of
//	{E} equal to its mana value.
//
// Regression test for all 3 failure categories:
//   - CardIdentity: token copy creation (CR §111.1 — not a zone move)
//   - ReplacementCompleteness: enters tapped and attacking (§508 replacement)
//   - TriggerCompleteness: delayed end-step sacrifice trigger (§603.7)
func registerSatya(r *Registry) {
	r.OnTrigger("Satya, Aetherflux Genius", "creature_attacks", satyaOnAttack)
}

func satyaOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "satya_aetherflux_genius_attack"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Only fire when Satya herself is the attacker.
	attackerPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attackerPerm != perm {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Find best nontoken creature to copy — "another target nontoken
	// creature you control" (not Satya herself).
	var best *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || p == perm {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if isToken(p) {
			continue
		}
		if best == nil || p.Timestamp > best.Timestamp {
			best = p
		}
	}
	if best == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_nontoken_creature_target", nil)
		// Still grant energy even with no target — "you get {E}{E}" is
		// not contingent on the copy per oracle text.
		gameengine.GainEnergy(gs, seat, 2)
		return
	}

	// Create a token copy of the target creature.
	tokenCard := best.Card.DeepCopy()
	tokenCard.Owner = seat
	if !hasType(tokenCard.Types, "token") {
		tokenCard.Types = append([]string{"token"}, tokenCard.Types...)
	}

	// Enter tapped and attacking — CR §508. Use enterBattlefieldWithETB
	// for full ETB cascade, then stamp the attacking flag.
	tokenPerm := enterBattlefieldWithETB(gs, seat, tokenCard, true)
	if tokenPerm == nil {
		gameengine.GainEnergy(gs, seat, 2)
		return
	}
	// Mark as attacking (entered "tapped and attacking").
	if tokenPerm.Flags == nil {
		tokenPerm.Flags = map[string]int{}
	}
	tokenPerm.Flags["attacking"] = 1

	// Grant {E}{E}.
	gameengine.GainEnergy(gs, seat, 2)

	// Register delayed trigger: sacrifice at next end step unless
	// energy equal to mana value is paid.
	capturedToken := tokenPerm
	capturedMV := tokenCard.CMC
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seat,
		SourceCardName: "Satya, Aetherflux Genius",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			// Check if token is still on the battlefield.
			found := false
			for _, p := range gs.Seats[seat].Battlefield {
				if p == capturedToken {
					found = true
					break
				}
			}
			if !found {
				return
			}
			// Attempt to pay energy equal to mana value.
			if capturedMV > 0 && gameengine.PayEnergy(gs, seat, capturedMV) {
				emit(gs, "satya_energy_paid", "Satya, Aetherflux Genius", map[string]interface{}{
					"seat":      seat,
					"token":     capturedToken.Card.DisplayName(),
					"energy_paid": capturedMV,
				})
				return
			}
			// Can't or won't pay — sacrifice.
			gameengine.SacrificePermanent(gs, capturedToken, "satya_end_step")
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"copied":        best.Card.DisplayName(),
		"token_tapped":  true,
		"token_attacking": true,
		"energy_gained": 2,
		"delayed":       "sacrifice_at_next_end_step_unless_energy_paid",
	})
}

func hasType(types []string, t string) bool {
	for _, tt := range types {
		if tt == t {
			return true
		}
	}
	return false
}

func isToken(p *gameengine.Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	return hasType(p.Card.Types, "token")
}
