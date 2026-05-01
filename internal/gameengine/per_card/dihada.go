package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDihada wires Dihada, Binder of Wills.
//
// Oracle text:
//
//	Loyalty 5.
//	+2: Up to one target legendary creature gains vigilance, lifelink,
//	    and indestructible until your next turn.
//	-3: Reveal the top four cards of your library. Put any number of
//	    legendary cards from among them into your hand and the rest
//	    into your graveyard.
//	-11: Gain control of all nonland permanents until end of turn.
//	     Untap them. They gain haste until end of turn.
//
// Implementation:
//   - abilityIdx 0 (+2): grant kw flags on best legendary creature
//     controller controls; clear flags via "your_next_turn" delayed
//     trigger.
//   - abilityIdx 1 (-3): reveal top 4, route legendaries to hand and
//     non-legendaries to graveyard.
//   - abilityIdx 2 (-11): "threaten all" — move every nonland permanent
//     onto Dihada's controller's battlefield, untap, grant haste.
//     Register a "next_end_step" delayed trigger to return permanents
//     to their original controllers. Edge cases (permanents that die,
//     phase out, change zones) emit partial.
//
// Engine-side caveats: planeswalker loyalty cost adjustment (paying
// +2/-3/-11) is handled by the activation pipeline. Loyalty cost
// counter movement is the engine's responsibility (see
// activation.go §606 enforcement); this handler provides only the
// effect.
func registerDihada(r *Registry) {
	r.OnActivated("Dihada, Binder of Wills", dihadaActivated)
}

func dihadaActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	switch abilityIdx {
	case 0:
		dihadaPlusTwo(gs, src)
	case 1:
		dihadaMinusThree(gs, src)
	case 2:
		dihadaMinusEleven(gs, src)
	}
}

func dihadaPlusTwo(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "dihada_plus_two_legendary_buff"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Pick the best legendary creature controller controls. Prefer the
	// strongest body (power+toughness) — granting indestructible to a
	// 1/1 is wasted compared to a 7/7 attacker.
	var target *gameengine.Permanent
	bestScore := -1
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() || !p.IsLegendary() {
			continue
		}
		score := p.Power() + p.Toughness()
		if score > bestScore {
			bestScore = score
			target = p
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_legendary_creature", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["kw:vigilance"] = 1
	target.Flags["kw:lifelink"] = 1
	target.Flags["kw:indestructible"] = 1

	captured := target
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_turn",
		ControllerSeat: seat,
		SourceCardName: "Dihada, Binder of Wills",
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			delete(captured.Flags, "kw:vigilance")
			delete(captured.Flags, "kw:lifelink")
			delete(captured.Flags, "kw:indestructible")
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"target":   target.Card.DisplayName(),
		"keywords": []string{"vigilance", "lifelink", "indestructible"},
		"duration": "until_your_next_turn",
	})
}

func dihadaMinusThree(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "dihada_minus_three_reveal_four"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	n := 4
	if len(s.Library) < n {
		n = len(s.Library)
	}
	if n == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "library_empty", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	revealed := make([]*gameengine.Card, n)
	copy(revealed, s.Library[:n])
	s.Library = s.Library[n:]

	var toHand []string
	var toGrave []string
	for _, c := range revealed {
		if c == nil {
			continue
		}
		if cardHasType(c, "legendary") {
			s.Hand = append(s.Hand, c)
			toHand = append(toHand, c.DisplayName())
			gs.LogEvent(gameengine.Event{
				Kind:   "draw",
				Seat:   seat,
				Source: src.Card.DisplayName(),
				Details: map[string]interface{}{
					"slug":   slug,
					"reason": "dihada_legendary_pick",
					"card":   c.DisplayName(),
				},
			})
		} else {
			s.Graveyard = append(s.Graveyard, c)
			toGrave = append(toGrave, c.DisplayName())
			gs.LogEvent(gameengine.Event{
				Kind:   "mill",
				Seat:   seat,
				Source: src.Card.DisplayName(),
				Details: map[string]interface{}{
					"slug":   slug,
					"reason": "dihada_non_legendary_dump",
					"card":   c.DisplayName(),
				},
			})
		}
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"revealed":       n,
		"to_hand_count":  len(toHand),
		"to_hand_cards":  toHand,
		"to_grave_count": len(toGrave),
		"to_grave_cards": toGrave,
	})
}

// dihadaThreatenedPerm tracks a stolen permanent so the end-of-turn
// reverter can restore controller identity.
type dihadaThreatenedPerm struct {
	perm           *gameengine.Permanent
	originalSeat   int
	hadHaste       bool
	originalTimest int
}

func dihadaMinusEleven(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "dihada_minus_eleven_threaten_all"
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	stolen := []dihadaThreatenedPerm{}
	allHaste := []*gameengine.Permanent{}
	allUntap := 0

	// Collect every nonland permanent across all seats first to avoid
	// mutating slices we're iterating over.
	type pendMove struct {
		perm     *gameengine.Permanent
		fromSeat int
	}
	var pending []pendMove
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsLand() {
				continue
			}
			pending = append(pending, pendMove{perm: p, fromSeat: i})
		}
	}

	for _, pm := range pending {
		p := pm.perm
		fromSeat := pm.fromSeat

		// Untap (text: "Untap them.")
		if p.Tapped {
			p.Tapped = false
			allUntap++
		}

		// Haste flag — record prior state so we can restore.
		hadHaste := false
		if p.Flags != nil && p.Flags["kw:haste"] == 1 {
			hadHaste = true
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:haste"] = 1
		p.SummoningSick = false
		allHaste = append(allHaste, p)

		// Move to Dihada's controller's battlefield if not already.
		if fromSeat != seat {
			origTS := p.Timestamp
			// Detach from original controller.
			origSeat := gs.Seats[fromSeat]
			for idx, q := range origSeat.Battlefield {
				if q == p {
					origSeat.Battlefield = append(origSeat.Battlefield[:idx], origSeat.Battlefield[idx+1:]...)
					break
				}
			}
			p.Controller = seat
			p.Timestamp = gs.NextTimestamp()
			gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
			gs.LogEvent(gameengine.Event{
				Kind:   "gain_control",
				Seat:   seat,
				Target: fromSeat,
				Source: src.Card.DisplayName(),
				Details: map[string]interface{}{
					"slug":        slug,
					"target_card": p.Card.DisplayName(),
					"duration":    "until_end_of_turn",
				},
			})
			stolen = append(stolen, dihadaThreatenedPerm{
				perm:           p,
				originalSeat:   fromSeat,
				hadHaste:       hadHaste,
				originalTimest: origTS,
			})
		}
	}

	// Capture for the delayed trigger so haste is removed from non-stolen
	// permanents too at end of turn.
	capturedHasteList := make([]*gameengine.Permanent, len(allHaste))
	copy(capturedHasteList, allHaste)
	hadHasteSet := make(map[*gameengine.Permanent]bool, len(stolen))
	for _, st := range stolen {
		hadHasteSet[st.perm] = st.hadHaste
	}
	// For non-stolen permanents, record whether they already had haste so
	// we don't strip a real keyword.
	preStateHaste := make(map[*gameengine.Permanent]bool, len(allHaste))
	for _, p := range allHaste {
		if had, ok := hadHasteSet[p]; ok {
			preStateHaste[p] = had
			continue
		}
		// Non-stolen: we can no longer reconstruct prior flag, but we
		// just set kw:haste so removing it is safe unless the AST
		// itself grants haste — HasKeyword still returns true via AST
		// even after we delete the flag. So unconditionally clear our
		// runtime flag.
		preStateHaste[p] = false
	}

	capturedStolen := make([]dihadaThreatenedPerm, len(stolen))
	copy(capturedStolen, stolen)

	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seat,
		SourceCardName: "Dihada, Binder of Wills",
		EffectFn: func(gs *gameengine.GameState) {
			// Return stolen permanents to original controllers.
			for _, st := range capturedStolen {
				p := st.perm
				if p == nil || p.Card == nil {
					continue
				}
				// Skip if it's no longer on Dihada's controller's
				// battlefield (died, exiled, bounced, re-stolen).
				ourSeat := gs.Seats[seat]
				stillHere := false
				ourIdx := -1
				if ourSeat != nil {
					for idx, q := range ourSeat.Battlefield {
						if q == p {
							stillHere = true
							ourIdx = idx
							break
						}
					}
				}
				if !stillHere {
					continue
				}
				if st.originalSeat < 0 || st.originalSeat >= len(gs.Seats) {
					continue
				}
				dest := gs.Seats[st.originalSeat]
				if dest == nil {
					continue
				}
				ourSeat.Battlefield = append(ourSeat.Battlefield[:ourIdx], ourSeat.Battlefield[ourIdx+1:]...)
				p.Controller = st.originalSeat
				p.Timestamp = gs.NextTimestamp()
				dest.Battlefield = append(dest.Battlefield, p)
				gs.LogEvent(gameengine.Event{
					Kind:   "lose_control",
					Seat:   seat,
					Target: st.originalSeat,
					Source: "Dihada, Binder of Wills",
					Details: map[string]interface{}{
						"target_card": p.Card.DisplayName(),
						"reason":      "until_end_of_turn_expired",
					},
				})
			}
			// Strip runtime haste flag where we added it.
			for _, p := range capturedHasteList {
				if p == nil || p.Flags == nil {
					continue
				}
				if !preStateHaste[p] {
					delete(p.Flags, "kw:haste")
				}
			}
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"stolen_count":   len(stolen),
		"untapped_count": allUntap,
		"haste_count":    len(allHaste),
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"control_revert_skips_clones_phaseouts_and_zone_changes")
}
