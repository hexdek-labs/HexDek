package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerProgenitus wires Progenitus.
//
// Oracle text:
//
//	Protection from everything
//	If Progenitus would be put into a graveyard from anywhere, reveal
//	Progenitus and shuffle it into its owner's library instead.
//
// R49 stub-batch-E port (defensive utility — graveyard-shuffle replacement):
//
//	The R37 implementation only fired on creature_dies (battlefield →
//	graveyard) and pulled the card BACK after the death zone change.
//	This port:
//	  - Registers a true §614 `would_be_put_into_graveyard`
//	    replacement at ETB. ApplyFn manually inserts the Card into the
//	    owner's library, shuffles, then cancels the event so the
//	    engine's downstream moveToZone is suppressed and the card
//	    doesn't get double-placed. Mirrors Rest in Peace's
//	    replacement-payload shape, but the destination is library (not
//	    exile) and we cancel because library writes aren't a destination
//	    the replacement-payload pipeline writes to.
//	  - Keeps the reactive creature_dies fallback for code paths that
//	    bypass §614 (e.g., direct removePermanent + moveCardBetweenZones
//	    from older per_card handlers). The fallback runs idempotently —
//	    if the replacement already moved the card, the graveyard scan
//	    no-ops.
//	  - Protection from everything: still set via Flags["prot:*"]; the
//	    universal-protection sentinel is read by combat + target dispatch.
//
//	UnregisterReplacementsForPermanent on LTB tears down the §614 hook
//	(LTB happens before the replacement payload is consulted again
//	because we cancel the event — but if Progenitus is exiled from the
//	battlefield directly, the unregister fires).
func registerProgenitus(r *Registry) {
	r.OnETB("Progenitus", progenitusETBSetProtectionAndRegisterReplacement)
	r.OnTrigger("Progenitus", "creature_dies", progenitusShuffleBackOnDeath)
}

func progenitusETBSetProtectionAndRegisterReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "progenitus_protection_from_everything"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["prot:*"] = 1

	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_be_put_into_graveyard",
		RedirectsZone:  true,
		HandlerID:      "progenitus:would_gy_shuffle:" + perm.Card.DisplayName(),
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			// Match by card-name (the Card pointer is what's being routed;
			// matching by DisplayName lets the cancel apply whether the
			// dying perm is the same *Permanent or a re-wrapped one).
			name, _ := ev.Payload["card_name"].(string)
			if name == "" && ev.TargetPerm != nil && ev.TargetPerm.Card != nil {
				name = ev.TargetPerm.Card.DisplayName()
			}
			if name != "Progenitus" {
				return false
			}
			toZone := ev.String("to_zone")
			return toZone == "" || toZone == "graveyard"
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			progenitusInsertAndShuffle(gs, perm, ev)
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"prot_star_set":    true,
		"shuffle_replcmt":  "registered",
	})
}

func progenitusInsertAndShuffle(gs *gameengine.GameState, perm *gameengine.Permanent, ev *gameengine.ReplEvent) {
	const slug = "progenitus_shuffle_replacement_apply"
	if perm == nil || perm.Card == nil {
		return
	}
	owner := perm.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		owner = perm.Controller
	}
	ownerSeat := gs.Seats[owner]
	if ownerSeat == nil {
		return
	}
	ownerSeat.Library = append(ownerSeat.Library, perm.Card)
	if len(ownerSeat.Library) > 1 && gs.Rng != nil {
		gs.Rng.Shuffle(len(ownerSeat.Library), func(i, j int) {
			ownerSeat.Library[i], ownerSeat.Library[j] = ownerSeat.Library[j], ownerSeat.Library[i]
		})
	}
	// Cancel the downstream moveToZone — we've placed the card in
	// library ourselves, and the engine's would_be_put_into_graveyard
	// consumer doesn't natively support library as a redirect destination.
	ev.Cancelled = true
	gs.LogEvent(gameengine.Event{
		Kind:   "replacement_applied",
		Seat:   perm.Controller,
		Source: "Progenitus",
		Details: map[string]interface{}{
			"rule":   "614",
			"effect": "shuffle_into_owner_library",
			"owner":  owner,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"owner": owner,
	})
}

func progenitusShuffleBackOnDeath(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "progenitus_shuffle_back_on_death_fallback"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil {
		card, _ := ctx["card"].(*gameengine.Card)
		if card == nil || card.DisplayName() != "Progenitus" {
			return
		}
		dying = perm
	}
	if dying != perm && dying.Card != nil && dying.Card.DisplayName() != "Progenitus" {
		return
	}
	owner := perm.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		owner = perm.Controller
	}
	ownerSeat := gs.Seats[owner]
	if ownerSeat == nil {
		return
	}
	// Fallback: if the §614 replacement already pulled the card to
	// library, this scan no-ops because it won't find Progenitus in the
	// graveyard. Otherwise, rescue it (covers code paths that bypass the
	// §614 chain).
	moved := false
	for i, c := range ownerSeat.Graveyard {
		if c == perm.Card {
			ownerSeat.Graveyard = append(ownerSeat.Graveyard[:i], ownerSeat.Graveyard[i+1:]...)
			ownerSeat.Library = append(ownerSeat.Library, c)
			moved = true
			break
		}
	}
	if moved && len(ownerSeat.Library) > 1 && gs.Rng != nil {
		gs.Rng.Shuffle(len(ownerSeat.Library), func(i, j int) {
			ownerSeat.Library[i], ownerSeat.Library[j] = ownerSeat.Library[j], ownerSeat.Library[i]
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  owner,
		"moved": moved,
	})
}
