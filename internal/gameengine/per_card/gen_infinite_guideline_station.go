package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerInfiniteGuidelineStation wires Infinite Guideline Station.
//
// Oracle text (Scryfall, verified):
//
//	When Infinite Guideline Station enters, create a tapped 2/2
//	colorless Robot artifact creature token for each multicolored
//	permanent you control.
//	Station (Tap another creature you control: Put charge counters
//	equal to its power on this Spacecraft. Station only as a sorcery.
//	It's an artifact creature at 12+.)
//	12+ | Flying
//	Whenever Infinite Guideline Station attacks, draw a card for each
//	multicolored permanent you control.
//
// Implementation (R46 stub port):
//   - ETB: count multicolored permanents the controller controls
//     (perms whose Card.Colors slice has ≥2 entries); mint that many
//     tapped 2/2 colorless Robot artifact creature tokens via
//     CreateCreatureToken (replaces the auto-gen stub's single
//     unnamed token plus a stray draw).
//   - Attack trigger: OnTrigger("creature_attacks") gated on
//     attacker_perm == perm; draw N=multicolored-perms cards.
//   - Station charge-counter activated ability and the 12+ flying
//     threshold are emitPartial'd. The Station tap-another-creature
//     for power-many charge counters is an activated-ability surface
//     the per_card layer can model, but the "becomes a creature at
//     12+" Spacecraft-type-change rider is a layered effect Phase 8
//     work; we breadcrumb both.
func registerInfiniteGuidelineStation(r *Registry) {
	r.OnETB("Infinite Guideline Station", infiniteGuidelineStationETB)
	r.OnTrigger("Infinite Guideline Station", "creature_attacks", infiniteGuidelineAttackDraw)
}

func infiniteGuidelineStationETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "infinite_guideline_station_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	multi := countMulticoloredPerms(gs, seat, perm)
	for i := 0; i < multi; i++ {
		tok := gameengine.CreateCreatureToken(
			gs,
			seat,
			"Robot",
			[]string{"artifact", "creature", "robot"},
			2, 2,
		)
		if tok != nil {
			tok.Tapped = true
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"multicolored":  multi,
		"robots_minted": multi,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"station_charge_counter_activated_ability_and_12_plus_spacecraft_type_change_not_modeled")
}

func infiniteGuidelineAttackDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "infinite_guideline_attack_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	multi := countMulticoloredPerms(gs, perm.Controller, perm)
	drawn := 0
	for i := 0; i < multi; i++ {
		c := drawOne(gs, perm.Controller, perm.Card.DisplayName())
		if c == nil {
			break
		}
		drawn++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"multicolored": multi,
		"drawn":        drawn,
	})
}

// countMulticoloredPerms returns the number of perms on `seat`'s
// battlefield whose card carries ≥2 colors. `exclude` is skipped so
// callers can avoid counting the trigger source itself when it
// doesn't qualify (Infinite Guideline Station is colorless, but the
// guard keeps the count stable if a future printing changes that).
func countMulticoloredPerms(gs *gameengine.GameState, seatIdx int, exclude *gameengine.Permanent) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return 0
	}
	count := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p == exclude {
			continue
		}
		if len(p.Card.Colors) >= 2 {
			count++
		}
	}
	return count
}
