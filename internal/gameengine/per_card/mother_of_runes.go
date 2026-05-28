package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMotherOfRunes wires Mother of Runes.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Mother%20of%20Runes):
//
//	{T}: Target creature you control gains protection from the color
//	of your choice until end of turn.
//
// {W} Creature — Human Cleric 1/1. THE single-target protection
// engine — every turn, Mom locks one threat against the most
// problematic opp color. Mono-W staple in Voltron / Hatebears /
// Sisay-led commander shells. Pairs with Lightning Greaves for the
// Mom-targets-self loop (haste from Greaves, tap Mom turn 2 to
// protect a key creature from the table's predominant color).
//
// Implementation:
//   - OnActivated index 0: tap, pick a target creature controller
//     controls, pick a color based on opponents' aggregate color
//     identity (the most-represented opp color across all opp
//     battlefields — the policy default when no caller-specified
//     color was stamped).
//   - GrantedAbilities receives "protection" + the perm.Flags gets
//     "protection_from_<color>" set (matches the engine's combat.go
//     damage-prevention path at keywords_combat.go:1787-1792).
//   - Color picker: scan opp battlefields, count pip:X color
//     occurrences (cards with each color identity), pick the
//     maximum. Tie-break in WUBRG order for determinism.
//   - Cleanup at EOT: GrantedAbilities gets cleared by the
//     phases.go cleanup step (§514.2). The protection_from_X flag
//     gets cleared the same pass; we stamp it as a Flag (int) which
//     the existing cleanup logic clears alongside GrantedAbilities.
func registerMotherOfRunes(r *Registry) {
	r.OnActivated("Mother of Runes", motherOfRunesActivate)
}

func motherOfRunesActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mother_of_runes_grant_protection"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Mother of Runes", "already_tapped", nil)
		return
	}
	if src.SummoningSick && src.IsCreature() {
		emitFail(gs, slug, "Mother of Runes", "summoning_sick", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Pick target: highest-power creature controller controls.
	var target *gameengine.Permanent
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if target == nil || p.Power() > target.Power() {
			target = p
		}
	}
	if target == nil {
		emitFail(gs, slug, "Mother of Runes", "no_target_creature", nil)
		return
	}

	// Pick color: most-represented across opp battlefields, WUBRG
	// priority for ties.
	color := pickMomColorByOppBoard(gs, seat)
	if color == "" {
		// No opp colors on board — fall back to W (Mom's own color)
		// as a safe protective default.
		color = "W"
	}

	src.Tapped = true
	target.GrantedAbilities = append(target.GrantedAbilities, "protection")
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["protection_from_"+color] = 1
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, "Mother of Runes", map[string]interface{}{
		"seat":   seat,
		"target": target.Card.DisplayName(),
		"color":  color,
	})
}

// pickMomColorByOppBoard counts color pips across opp battlefields and
// returns the most-represented color. WUBRG-priority tie-break.
func pickMomColorByOppBoard(gs *gameengine.GameState, seat int) string {
	counts := map[string]int{}
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			for _, t := range p.Card.Types {
				switch t {
				case "pip:W", "color:W", "white":
					counts["W"]++
				case "pip:U", "color:U", "blue":
					counts["U"]++
				case "pip:B", "color:B", "black":
					counts["B"]++
				case "pip:R", "color:R", "red":
					counts["R"]++
				case "pip:G", "color:G", "green":
					counts["G"]++
				}
			}
		}
	}
	best := ""
	bestN := 0
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		if counts[c] > bestN {
			bestN = counts[c]
			best = c
		}
	}
	return best
}
