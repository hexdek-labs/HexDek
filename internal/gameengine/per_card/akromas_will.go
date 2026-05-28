package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAkromasWill wires Akroma's Will.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Akroma%27s%20Will):
//
//	Until end of turn, creatures you control gain flying, vigilance,
//	double strike, lifelink, indestructible, and protection from black
//	and from red. Convoke.
//
// {3}{W} Sorcery. The "alpha-strike-with-protection" finisher — turn
// your board into evasive lifelink-double-strike threats with built-in
// removal protection (the black + red protection covers most cEDH
// removal spectrum). Convoke makes it castable with 5+ creatures
// tapping in for the colorless slots, often at zero net mana.
//
// Implementation:
//   - OnResolve. For every creature controller controls, append the
//     full keyword stack to GrantedAbilities: flying, vigilance,
//     double strike, lifelink, indestructible. Stamp Flags for
//     protection_from_black and protection_from_red (matches
//     combat.go's color-protection check shape).
//   - Convoke cost-reduction is engine-side (in the cast pipeline)
//     and isn't this handler's responsibility.
//   - Cleanup at EOT clears GrantedAbilities + protection flags
//     (§514.2 wear-off, mirrors Heroic Intervention shape).
//   - Creatures only — non-creature permanents don't get the grants
//     per printed "creatures you control" restriction.
func registerAkromasWill(r *Registry) {
	r.OnResolve("Akroma's Will", akromasWillResolve)
}

func akromasWillResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "akromas_will"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	keywords := []string{
		"flying",
		"vigilance",
		"double_strike",
		"lifelink",
		"indestructible",
	}

	granted := 0
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.GrantedAbilities = append(p.GrantedAbilities, keywords...)
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["protection_from_black"] = 1
		p.Flags["protection_from_red"] = 1
		granted++
	}
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, "Akroma's Will", map[string]interface{}{
		"seat":       seat,
		"granted":    granted,
		"duration":   "until_end_of_turn",
		"keywords":   keywords,
		"protection": []string{"black", "red"},
	})
}
