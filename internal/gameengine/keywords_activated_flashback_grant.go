package gameengine

// keywords_flashback_grant.go — Activated-ability flashback-grant primitive.
//
// Cards in this family pay an activated-ability cost (tap, {2}{U}, exile-
// self, etc.) to grant flashback until end of turn to instant/sorcery
// cards in a player's graveyard. The Snapcaster Mage idiom
// (GrantFlashbackUntilEOT in keywords_flashback.go) is the ETB-triggered
// shape of the same effect; this primitive wraps it for the activated-
// ability case and adds two modes the trigger shape doesn't need:
//
//   - AllInZone:  Yawgmoth's Will-style "each instant and sorcery card in
//                 your graveyard gains flashback until end of turn" (also
//                 Past in Flames, Will of the Jeskai modal, Backdraft
//                 Hellkite, Yawgmoth's Will / Gaea's Will / Magus of the
//                 Will when narrowed to instants/sorceries).
//   - Auto pick:  when the caller didn't supply an explicit target_card
//                 (AI hasn't bound a target yet), pick the highest-CMC
//                 instant/sorcery in the activator's graveyard. Lets the
//                 handler do something useful even without targeting.
//
// CR §702.34a (flashback) and §702.34c (exile on resolve) — both are
// honored by the underlying GrantFlashbackUntilEOT, which registers a
// ZoneCastPermission with ExileOnResolve=true and Duration="until_end_of_
// turn". ExpireZoneCastGrants (phases.go EndOfTurnCleanup) removes the
// permission at end of turn.

// ActivatedFlashbackGrantOptions configures a call to
// ActivatedFlashbackGrant. The zero value (all defaults) means: pick one
// target in seat's own graveyard, use the card's mana cost as the
// flashback cost.
type ActivatedFlashbackGrantOptions struct {
	// Source is the granting card's display name. Logged on each grant.
	Source string
	// Seat is the activator's seat — the graveyard whose cards become
	// flashback-castable. Activated flashback grants in this family
	// always target the activator's own graveyard.
	Seat int
	// Target, if non-nil, is the explicit instant/sorcery card to grant.
	// Used when the AI binds a target via ctx["target_card"]. Ignored
	// when AllInZone is true.
	Target *Card
	// AllInZone, if true, grants flashback to every instant or sorcery
	// card in Seat's graveyard (Yawgmoth's Will mode). Target is ignored.
	AllInZone bool
}

// ActivatedFlashbackGrant grants flashback until end of turn to one (or
// all) instant/sorcery card(s) in the activator's graveyard. Returns the
// cards that received a grant.
//
// Single-target mode (default):
//   - If opts.Target is non-nil and is an instant/sorcery in Seat's
//     graveyard, grant it.
//   - Otherwise auto-pick the highest-CMC instant/sorcery in Seat's
//     graveyard (ties broken by most-recently-milled), grant that.
//   - If no eligible card exists, returns an empty slice — caller should
//     log emitFail.
//
// AllInZone mode:
//   - Iterate Seat's graveyard, grant every instant/sorcery card.
//     Returns the full list. The opts.Target field is ignored.
//
// Always logs one "activated_flashback_grant" event summarizing the
// operation. Individual ZoneCastPermission registrations also produce
// "zone_cast_grant_registered" events via RegisterZoneCastGrant.
func ActivatedFlashbackGrant(gs *GameState, opts ActivatedFlashbackGrantOptions) []*Card {
	if gs == nil {
		return nil
	}
	if opts.Seat < 0 || opts.Seat >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[opts.Seat]
	if seat == nil {
		return nil
	}

	var granted []*Card

	if opts.AllInZone {
		for _, c := range seat.Graveyard {
			if !flashbackGrantEligible(c) {
				continue
			}
			GrantFlashbackUntilEOT(gs, c, opts.Seat, opts.Source)
			granted = append(granted, c)
		}
	} else {
		target := opts.Target
		if target == nil || !flashbackGrantEligible(target) || !cardInGraveyard(seat, target) {
			target = pickFlashbackGrantTarget(seat)
		}
		if target != nil {
			GrantFlashbackUntilEOT(gs, target, opts.Seat, opts.Source)
			granted = append(granted, target)
		}
	}

	names := make([]string, 0, len(granted))
	for _, c := range granted {
		names = append(names, c.DisplayName())
	}
	gs.LogEvent(Event{
		Kind:   "activated_flashback_grant",
		Seat:   opts.Seat,
		Source: opts.Source,
		Amount: len(granted),
		Details: map[string]interface{}{
			"mode":   ternaryStr(opts.AllInZone, "all_in_zone", "single_target"),
			"cards":  names,
			"rule":   "702.34a",
			"source": opts.Source,
		},
	})
	return granted
}

// flashbackGrantEligible reports whether `card` is a legal target for a
// flashback grant — i.e. it has the instant or sorcery card type. The
// grant primitive does NOT re-check whether the card already has
// flashback; doubling a grant is harmless because RegisterZoneCastGrant
// replaces the existing entry by card pointer.
func flashbackGrantEligible(card *Card) bool {
	if card == nil {
		return false
	}
	return cardHasType(card, "instant") || cardHasType(card, "sorcery")
}

// cardInGraveyard reports whether `card` is in `seat`'s graveyard right
// now. Cheap pointer-equality scan — graveyards are small.
func cardInGraveyard(seat *Seat, card *Card) bool {
	if seat == nil || card == nil {
		return false
	}
	for _, c := range seat.Graveyard {
		if c == card {
			return true
		}
	}
	return false
}

// pickFlashbackGrantTarget returns the highest-CMC instant/sorcery card
// in seat's graveyard, ties broken by most-recent (last index wins). Nil
// if none. Used as the auto-pick fallback when the AI hasn't bound a
// target via ctx["target_card"].
func pickFlashbackGrantTarget(seat *Seat) *Card {
	if seat == nil {
		return nil
	}
	var best *Card
	bestCMC := -1
	for i := len(seat.Graveyard) - 1; i >= 0; i-- {
		c := seat.Graveyard[i]
		if !flashbackGrantEligible(c) {
			continue
		}
		cmc := manaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	return best
}

// ternaryStr is a tiny inline conditional for log details.
func ternaryStr(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

