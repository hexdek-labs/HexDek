package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHeroicIntervention wires Heroic Intervention.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Heroic%20Intervention):
//
//	Permanents you control gain hexproof and indestructible until end
//	of turn.
//
// {1}{G}{G} Instant. THE green protection spell — saves the whole
// board from a Wrath of God / Toxic Deluge / Cyclonic Rift. Two
// printed keywords cover both targeted and mass-removal cases:
// hexproof beats single-target spells (Swords to Plowshares,
// Anguished Unmaking); indestructible beats sweepers that destroy
// (Wrath, Damnation, Vandalblast). NOT a save against exile sweepers
// (Farewell, Merciless Eviction) — those slip past indestructible
// per CR §702.12b.
//
// Implementation:
//   - OnResolve. For every permanent controller controls, append
//     "hexproof" and "indestructible" to Permanent.GrantedAbilities.
//     The phases.go cleanup step clears GrantedAbilities at EOT
//     (§514.2 "until end of turn" wear-off), so no per_card EOT
//     bookkeeping is needed.
//   - Phased-out and tokens both get the grant — the printed
//     "permanents you control" is unrestricted.
//   - No-op safe on empty board: no permanents to grant, but the
//     spell still resolves cleanly (matches printed semantics —
//     the effect simply has no targets to grant).
func registerHeroicIntervention(r *Registry) {
	r.OnResolve("Heroic Intervention", heroicInterventionResolve)
}

func heroicInterventionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "heroic_intervention"
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

	granted := 0
	for _, p := range s.Battlefield {
		if p == nil {
			continue
		}
		p.GrantedAbilities = append(p.GrantedAbilities, "hexproof", "indestructible")
		granted++
	}
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, "Heroic Intervention", map[string]interface{}{
		"seat":     seat,
		"granted":  granted,
		"duration": "until_end_of_turn",
		"keywords": []string{"hexproof", "indestructible"},
	})
}
