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
	// R52 batch K: Station activated ability. "Tap another creature
	// you control: Put charge counters equal to its power on this
	// Spacecraft." Sorcery-speed gate + tap-target chosen
	// heuristically as the highest-power friendly creature other
	// than the Station itself.
	r.OnActivated("Infinite Guideline Station", infiniteGuidelineStationActivate)
	// R55: 12+ flying / artifact-creature threshold via R54 Layer 4
	// + Layer 6 primitives.
	r.OnTrigger("Infinite Guideline Station", "counter_placed", infiniteGuidelineCheckSpacecraftThreshold)
}

// infiniteGuidelineCheckSpacecraftThreshold wires the 12+ Spacecraft
// transition. R55 port via Layer 4 (add creature type) + Layer 6
// (grant flying) primitives.
func infiniteGuidelineCheckSpacecraftThreshold(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Counters == nil || perm.Counters["charge"] < 12 {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["infinite_guideline_spacecraft_active"] == 1 {
		return
	}
	perm.Flags["infinite_guideline_spacecraft_active"] = 1
	gameengine.RegisterAddTypes(gs, perm, []string{"creature"},
		gameengine.DurationUntilSourceLeaves,
		"Infinite Guideline Station (Spacecraft 12+)",
		"infinite_guideline_spacecraft_creature")
	gameengine.RegisterGrantKeyword(gs, perm, "flying",
		gameengine.DurationUntilSourceLeaves,
		"Infinite Guideline Station (Spacecraft 12+)",
		"infinite_guideline_spacecraft_flying")
	emit(gs, "infinite_guideline_spacecraft_threshold_crossed", perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"charge_counters": perm.Counters["charge"],
	})
}

func infiniteGuidelineStationActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "infinite_guideline_station_station_activate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if !isSorcerySpeed(gs, src.Controller) {
		emitFail(gs, slug, src.Card.DisplayName(), "not_sorcery_speed", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	var pick *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Tapped {
			continue
		}
		pow := p.Power()
		if pow > bestPow {
			bestPow = pow
			pick = p
		}
	}
	if pick == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_untapped_friendly_creature", nil)
		return
	}
	pick.Tapped = true
	if bestPow > 0 {
		src.AddCounter("charge", bestPow)
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             src.Controller,
		"tapped":           pick.Card.DisplayName(),
		"charge_counters":  bestPow,
		"total_charge":     src.Counters["charge"],
	})
	// R55: re-check the Spacecraft 12+ threshold now that we just
	// stamped charge counters. The trigger-based counter_placed hook
	// also covers external counter placements (proliferate, etc.).
	infiniteGuidelineCheckSpacecraftThreshold(gs, src, nil)
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
	// Station charge-counter activated ability is wired by
	// infiniteGuidelineStationActivate (R52 batch K). The 12+
	// artifact-creature type change remains a layered effect deferred
	// to Phase 8 layers work.
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
