package gameengine

// R60 — play-lands-and-cast-spells-from-graveyard primitive with
// exile-on-resolve and a graveyard→exile replacement effect.
//
// Models the Yawgmoth's Will family:
//
//   - Yawgmoth's Will (sorcery, until end of turn)
//   - Gaea's Will (sorcery + suspend, until end of turn)
//   - Magus of the Will (creature, {2}{B}+{T}+exile-self, until end of turn)
//   - Yawgmoth's Agenda (enchantment, permanent while on battlefield,
//     additional "you can't cast more than one spell each turn" clause
//     handled separately by the per-card handler)
//
// All four cards share the canonical body:
//
//   You may play lands and cast spells from your graveyard.
//   If a card would be put into your graveyard from anywhere, exile
//   that card instead.
//
// This primitive layers three pieces on top of the existing R55 +
// ZoneCastGrant + §614 replacement infrastructure:
//
//   1. Per-Card ZoneCastGrants on every nonland card currently in the
//      seat's graveyard at activation time (so the existing
//      CastFromZone pipeline can cast them for their printed mana
//      cost; ExileOnResolve routes them to exile on resolution per the
//      family's exile-instead-of-graveyard rider).
//   2. A ZoneCastPolicy (R55) with Zone=graveyard, OwnerScope=self,
//      ExileOnResolve=true to cover the predicate case for any card
//      that the per-Card grant pass missed (defense-in-depth; the GY
//      membership shouldn't change during the effect window because
//      the GY→exile replacement intercepts new arrivals).
//   3. A §614 ReplacementEffect on would_die / would_be_put_into_graveyard
//      that redirects the seat's cards from graveyard to exile, matching
//      Rest in Peace's shape (but seat-scoped to the activating player).
//
// The land-play half is recorded as a seat flag
// (play_lands_from_graveyard_eot_seat_N for the turn-scoped variant;
// play_lands_from_graveyard_perm_seat_N for the static permanent
// variant). The flag is set so downstream consumers (tryPlayLand,
// Freya / hat) can read it; the engine's existing tryPlayLand only
// looks at hand today, so the land-play permission is registered but
// not yet wired into the play-land action. This matches the R58
// partial-residual pattern (e.g. The Master of Keys) where the
// primitive captures intent but the cast/play consumer isn't yet
// reading it. A per_card_partial breadcrumb is emitted for tracking.
//
// CR citations:
//
//   §117.4   You may play a land or cast a spell from any zone if an
//            effect grants the permission. Yawgmoth's Will grants both
//            for one turn.
//   §305.1   Land plays are a special action, not a spell cast. The
//            land-play permission requires its own grant separate
//            from the cast permission.
//   §614     Replacement effects — the "exile instead of graveyard"
//            clause is a single sourceless replacement registered for
//            the duration of the effect.

// PlayFromGraveyardOptions configures a graveyard-play-and-cast grant.
type PlayFromGraveyardOptions struct {
	// SeatIdx — the seat receiving the grant.
	SeatIdx int

	// SourceName — display name of the granting card (for logs).
	SourceName string

	// SourcePerm — if non-nil, the grant is tied to this permanent
	// (Yawgmoth's Agenda style). UnregisterPlayFromGraveyardForPermanent
	// removes the bundle when the source LTBs. If nil (Yawgmoth's Will
	// / Gaea's Will / Magus of the Will), the grant is turn-scoped and
	// swept by ExpirePlayFromGraveyardForTurn at end-of-turn cleanup.
	SourcePerm *Permanent

	// Permanent — true for Yawgmoth's Agenda (Duration=while_source_on_bf).
	// False for the sorcery / activated-ability sources
	// (Duration=until_end_of_turn).
	Permanent bool
}

// RegisterPlayFromGraveyard installs the bundle: per-Card ZoneCastGrants
// for every nonland card in the seat's graveyard at call time, a
// ZoneCastPolicy for late-arriving cards, the GY→exile replacement
// effect, and the play-lands seat flag.
//
// Returns the number of per-Card grants registered (callers may want
// this for logging).
func RegisterPlayFromGraveyard(gs *GameState, opts PlayFromGraveyardOptions) int {
	if gs == nil {
		return 0
	}
	if opts.SeatIdx < 0 || opts.SeatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[opts.SeatIdx]
	if seat == nil {
		return 0
	}

	duration := "until_end_of_turn"
	flagKey := "play_lands_from_graveyard_eot_seat_" + itoa(opts.SeatIdx)
	if opts.Permanent {
		duration = "while_source_on_bf"
		flagKey = "play_lands_from_graveyard_perm_seat_" + itoa(opts.SeatIdx)
	}

	// 1. Per-Card ZoneCastGrants on every nonland card currently in
	//    the seat's graveyard. Lands aren't cast, so they get the
	//    seat-flag pathway instead.
	granted := 0
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "land") {
			continue
		}
		// Don't clobber a stronger grant if one already exists for
		// this card (e.g. Underworld Breach's escape with the exile-3
		// cost rider). The cast pipeline picks the first applicable
		// grant; ours is "pay printed mana cost", which is strictly
		// easier than escape, so the stronger grant being there
		// already is fine.
		if GetZoneCastGrant(gs, c) != nil {
			continue
		}
		perm := &ZoneCastPermission{
			Zone:              ZoneGraveyard,
			Keyword:           "play_from_graveyard",
			ManaCost:          -1, // use the card's printed mana cost
			ExileOnResolve:    true,
			RequireController: opts.SeatIdx,
			SourceName:        opts.SourceName,
			Duration:          duration,
			GrantTurn:         gs.Turn,
		}
		if opts.SourcePerm != nil {
			perm.SourceTimestamp = opts.SourcePerm.Timestamp
		}
		RegisterZoneCastGrant(gs, c, perm)
		granted++
	}

	// 2. ZoneCastPolicy (R55) — covers any card that enters the
	//    graveyard later in the same window (Yawgmoth's Agenda case
	//    primarily; for the EOT variants the GY→exile replacement
	//    means new cards SHOULDN'T arrive, but the policy is
	//    defense-in-depth so a card that slips past the replacement
	//    — e.g. a self-replacement that fires first — still inherits
	//    the cast grant).
	policy := &ZoneCastPolicy{
		SourcePerm:     opts.SourcePerm,
		HandlerID:      "play_from_graveyard:" + opts.SourceName,
		Zone:           ZoneGraveyard,
		OwnerScope:     "self",
		CasterScope:    "controller",
		ControllerSeat: opts.SeatIdx,
		Predicate: func(c *Card) bool {
			// Spells only; the land-play half is a seat flag, not a
			// cast policy.
			return c != nil && !cardHasType(c, "land")
		},
		ManaCost:       -1,
		ExileOnResolve: true,
		Duration:       duration,
		GrantTurn:      gs.Turn,
	}
	if opts.SourcePerm != nil {
		policy.SourceTimestamp = opts.SourcePerm.Timestamp
	}
	gs.RegisterZoneCastPolicy(policy)

	// 3. GY→exile replacement effect. ControllerSeat = the activating
	//    seat. For the permanent variant we tie SourcePerm to the
	//    enchantment so LTB cleanup handles removal; for the
	//    turn-scoped variant SourcePerm stays nil and the effect is
	//    swept by ExpirePlayFromGraveyardForTurn at end-of-turn
	//    cleanup. The Applies predicate gates on TargetSeat /
	//    TargetPerm.Controller plus (turn-scoped) gs.Turn ==
	//    grantTurn so a late-firing event after the cleanup sweep
	//    doesn't accidentally still apply.
	grantTurn := gs.Turn
	apply := func(gs *GameState, ev *ReplEvent) {
		ev.Payload["to_zone"] = "exile"
		gs.LogEvent(Event{
			Kind:   "replacement_applied",
			Seat:   opts.SeatIdx,
			Source: opts.SourceName,
			Details: map[string]interface{}{
				"rule":   "614",
				"effect": "play_from_graveyard_redirect",
			},
		})
	}
	applies := func(gs *GameState, ev *ReplEvent) bool {
		if ev.String("to_zone") != "graveyard" {
			return false
		}
		// Seat ownership check: redirect only the activating seat's
		// cards. TargetPerm (for would_die) carries Controller;
		// TargetSeat (for would_be_put_into_graveyard) is set by the
		// firing site.
		if ev.TargetPerm != nil {
			if ev.TargetPerm.Controller != opts.SeatIdx {
				return false
			}
		} else if ev.TargetSeat != opts.SeatIdx {
			return false
		}
		// Turn-scoped variant: only fire on the activating turn.
		if !opts.Permanent && gs.Turn != grantTurn {
			return false
		}
		return true
	}
	ts := gs.NextTimestamp()
	suffix := itoa(opts.SeatIdx) + ":" + itoa(grantTurn) + ":" + itoa(ts)
	dieHandler := "play_from_graveyard_die_redirect:" + opts.SourceName + ":" + suffix
	gyHandler := "play_from_graveyard_gy_redirect:" + opts.SourceName + ":" + suffix
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      dieHandler,
		RedirectsZone:  true,
		SourcePerm:     opts.SourcePerm,
		ControllerSeat: opts.SeatIdx,
		Timestamp:      ts,
		Category:       CategoryOther,
		Applies:        applies,
		ApplyFn:        apply,
	})
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_be_put_into_graveyard",
		HandlerID:      gyHandler,
		RedirectsZone:  true,
		SourcePerm:     opts.SourcePerm,
		ControllerSeat: opts.SeatIdx,
		Timestamp:      ts,
		Category:       CategoryOther,
		Applies:        applies,
		ApplyFn:        apply,
	})

	// 4. Land-play seat flag. The engine's tryPlayLand only scans
	//    hand today; this flag is registered for downstream
	//    consumers (Freya valuation, future tryPlayLand extension).
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags[flagKey] = grantTurn
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags[flagKey] = grantTurn

	gs.LogEvent(Event{
		Kind:   "play_from_graveyard_granted",
		Seat:   opts.SeatIdx,
		Source: opts.SourceName,
		Details: map[string]interface{}{
			"rule":          "117.4",
			"duration":      duration,
			"grant_turn":    grantTurn,
			"per_card_grants": granted,
			"policy":        policy.HandlerID,
			"die_handler":   dieHandler,
			"gy_handler":    gyHandler,
		},
	})

	return granted
}

// ExpirePlayFromGraveyardForTurn sweeps the turn-scoped replacement
// effects and seat flags at end-of-turn cleanup. Called from
// phases.go EndOfTurnCleanup alongside ExpireZoneCastGrants.
//
// The per-Card ZoneCastGrants registered by the primitive carry
// Duration="until_end_of_turn" and are swept by the existing
// ExpireZoneCastGrants pass. The ZoneCastPolicy entries with the same
// Duration are swept by ExpireZoneCastPoliciesByDuration (also called
// from EndOfTurnCleanup). This function only handles the
// ReplacementEffect entries (which the existing pipeline doesn't
// have a duration sweep for) and the seat flags.
func ExpirePlayFromGraveyardForTurn(gs *GameState) {
	if gs == nil {
		return
	}
	// Sweep §614 replacement entries whose HandlerID identifies them
	// as turn-scoped from this primitive AND whose grantTurn has
	// elapsed. The HandlerID embeds grantTurn after the second colon
	// pair; we sweep any entry whose prefix matches our convention
	// and whose SourcePerm is nil (permanent-anchored entries are
	// cleaned up via UnregisterReplacementsForPermanent on LTB).
	if len(gs.Replacements) > 0 {
		kept := gs.Replacements[:0]
		for _, re := range gs.Replacements {
			if re == nil {
				continue
			}
			if re.SourcePerm == nil && isPlayFromGraveyardTurnHandler(re.HandlerID) {
				gs.LogEvent(Event{
					Kind:   "replacement_expired",
					Source: re.HandlerID,
					Details: map[string]interface{}{
						"rule":   "614",
						"reason": "play_from_graveyard_eot",
					},
				})
				continue
			}
			kept = append(kept, re)
		}
		gs.Replacements = kept
	}
	// Clear the seat flags. The permanent variant (perm_seat_) is
	// cleared by UnregisterPlayFromGraveyardForPermanent on LTB, not
	// here.
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		key := "play_lands_from_graveyard_eot_seat_" + itoa(i)
		if s.Flags != nil {
			delete(s.Flags, key)
		}
		if gs.Flags != nil {
			delete(gs.Flags, key)
		}
	}
}

// UnregisterPlayFromGraveyardForPermanent removes the
// permanent-anchored bundle when the source LTBs (Yawgmoth's Agenda
// being destroyed / exiled / bounced). The §614 replacements with
// SourcePerm == p are dropped by the engine's existing
// UnregisterReplacementsForPermanent path; this helper covers the
// seat flag, the per-Card ZoneCastGrants tied to this source's
// timestamp, AND the ZoneCastPolicy that the primitive registered
// at play_from_graveyard.go:172. The policy has SourcePerm bound but
// the engine LTB pathway in zone_change.go does NOT call
// UnregisterZoneCastPoliciesForPermanent — so without an explicit
// call here, the policy survived Agenda's death and any card entering
// the controller's graveyard later in the game still matched it as
// free-castable (silent correctness leak, no invariant coverage).
func UnregisterPlayFromGraveyardForPermanent(gs *GameState, p *Permanent) {
	if gs == nil || p == nil {
		return
	}
	seatIdx := p.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	key := "play_lands_from_graveyard_perm_seat_" + itoa(seatIdx)
	if s := gs.Seats[seatIdx]; s != nil && s.Flags != nil {
		delete(s.Flags, key)
	}
	if gs.Flags != nil {
		delete(gs.Flags, key)
	}
	// Drop per-Card ZoneCastGrants tied to this source's timestamp.
	// The engine's main grant expiry sweep keys off Duration, not
	// SourceTimestamp directly — drop them here so a bounced Agenda
	// cleans up its grants immediately rather than waiting for the
	// next end-of-turn.
	if len(gs.ZoneCastGrants) > 0 {
		for card, perm := range gs.ZoneCastGrants {
			if perm != nil && perm.SourceTimestamp == p.Timestamp &&
				perm.Keyword == "play_from_graveyard" {
				RemoveZoneCastGrant(gs, card)
			}
		}
	}
	// Drop the ZoneCastPolicy registered at play_from_graveyard.go:172.
	// Its Duration is "while_source_on_bf" and ExpireZoneCastPoliciesByDuration
	// intentionally leaves source-bound policies alone, so without this
	// the policy survives the source LTB.
	gs.UnregisterZoneCastPoliciesForPermanent(p)
}

// isPlayFromGraveyardTurnHandler matches the HandlerID prefix used by
// the primitive's turn-scoped replacement entries.
func isPlayFromGraveyardTurnHandler(handlerID string) bool {
	const dieP = "play_from_graveyard_die_redirect:"
	const gyP = "play_from_graveyard_gy_redirect:"
	return hasPrefix(handlerID, dieP) || hasPrefix(handlerID, gyP)
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

