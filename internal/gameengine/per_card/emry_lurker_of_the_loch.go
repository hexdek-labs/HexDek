package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEmryLurkerOfTheLoch wires up Emry, Lurker of the Loch.
//
// Oracle text:
//
//	This spell costs {1} less to cast for each artifact you control.
//	When Emry, Lurker of the Loch enters the battlefield, mill four
//	cards.
//	{T}: You may cast target artifact card from your graveyard. If
//	that spell would be put into a graveyard this turn, exile it
//	instead. Activate only once each turn.
//
// Urza / Sai / artifact-combo staple. The mill-4 ETB seeds the grave,
// and the tap ability recurs artifacts — Mox Opal, Mox Amber, Chromatic
// Sphere, etc., often combo'd with Isochron Scepter or Aetherflux
// Reservoir.
//
// Batch #2 scope:
//   - OnETB: mill 4 (top 4 of controller's library → graveyard).
//   - OnActivated(0, ctx["target_card"]): register a ZoneCastPermission
//     on a target artifact card in graveyard (cast from graveyard, exile
//     on resolve). End-of-turn cleanup removes unused permission.
//     Set perm.Flags["emry_activated_this_turn"] = 1 to enforce the
//     "once each turn" clause at the caller level.
//
// Cost reduction clause ({1} less per artifact you control) is handled
// by the cast pipeline — not per_card business.
func registerEmryLurkerOfTheLoch(r *Registry) {
	r.OnETB("Emry, Lurker of the Loch", emryETB)
	r.OwnsETBTrigger("Emry, Lurker of the Loch")
	r.OnActivated("Emry, Lurker of the Loch", emryActivate)
}

func emryETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "emry_mill_four"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	milled := 0
	for i := 0; i < 4 && len(s.Library) > 0; i++ {
		c := s.Library[0]
		gameengine.MoveCard(gs, c, seat, "library", "graveyard", "mill")
		milled++
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "mill",
		Seat:   seat,
		Target: seat,
		Source: perm.Card.DisplayName(),
		Amount: milled,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"milled": milled,
	})
}

func emryActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "emry_cast_artifact_from_grave"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		return
	}
	src.Tapped = true
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	if src.Flags["emry_activated_this_turn"] > 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "already_activated_this_turn", nil)
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	var target *gameengine.Card
	if v, ok := ctx["target_card"].(*gameengine.Card); ok {
		target = v
	}
	if target == nil {
		// Fallback: pick the most recently milled artifact in graveyard.
		for i := len(s.Graveyard) - 1; i >= 0; i-- {
			c := s.Graveyard[i]
			if cardHasType(c, "artifact") {
				target = c
				break
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_artifact_in_graveyard", nil)
		return
	}
	// Oracle, re-fetched r98: "{T}: Choose target artifact card in your
	// graveyard. You may cast that card this turn. (You still pay its
	// costs. Timing rules still apply.)"
	//
	// ExileOnResolve is FALSE (r98). Until r98 this was true, and the
	// comment justified it with the clause "if that spell would be put
	// into a graveyard this turn, exile it instead" — quoted as if
	// printed on Emry. IT IS NOT ON THE CARD. Kess, Maestros
	// Ascendancy and Wrexial all carry that exact sentence and were
	// verified in the same pass; Emry has no exile clause of any kind,
	// which is the point of the card — the artifact goes back to the
	// graveyard and can be cast again next turn.
	//
	// The flag was inert on the RESOLVE path (an artifact is a
	// permanent spell, and the exile branch in stack.go is an else-if
	// after the ETB arm, so it never ran — measured). It was NOT inert
	// on the off-board path: postStackZoneForOffboard consults
	// ShouldExileOnResolve with no permanent-spell check, so a
	// COUNTERED Emry-cast artifact was exiled instead of returned to
	// the graveyard — and the resolve event tagged it §702.34a,
	// flashback's rule, on a card with no flashback. Same split-path
	// shape as the r96 escape defect.
	//
	// Oracle "you may cast it this turn" → stamp Duration + GrantTurn so
	// ExpireZoneCastGrants reaps at EOT cleanup of the grant turn even
	// if the delayed-trigger fallback below races or is skipped.
	gameengine.RegisterZoneCastGrant(gs, target, &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneGraveyard,
		Keyword:           "emry_artifact_cast",
		ManaCost:          -1, // pay normal mana cost
		ExileOnResolve:    false,
		RequireController: seat,
		SourceName:        "Emry, Lurker of the Loch",
		Duration:          "until_end_of_turn",
		GrantTurn:         gs.Turn,
	})
	src.Flags["emry_activated_this_turn"] = 1

	// Clean up the zone-cast permission at end of turn if not used.
	targetRef := target
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seat,
		SourceCardName: "Emry, Lurker of the Loch",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.RemoveZoneCastGrant(gs, targetRef)
		},
	})

	gs.LogEvent(gameengine.Event{
		Kind:   "per_card_cast_from_grave",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"card":   target.DisplayName(),
			"zone":   "graveyard",
			"reason": "emry_activated",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              seat,
		"cast_card":         target.DisplayName(),
		"zone_cast_granted": true,
	})
}
