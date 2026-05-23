package gameengine

// keywords_flashback_grant.go — graveyard-wide flashback grants
// (CR §702.34a as a continuous granted ability).
//
// Some cards grant flashback to a whole class of cards in a player's
// graveyard rather than to a single targeted card. Examples:
//
//   - Iroh, Grand Lotus — during your turn, each non-Lesson instant
//     and sorcery in your graveyard has flashback equal to its mana
//     cost, and each Lesson has flashback {1}.
//   - Lier, Disciple of the Drowned — "You may cast instant and
//     sorcery cards from your graveyard." (always-on flashback at
//     the printed mana cost; Lier's exile-replacement is handled
//     separately.)
//
// The per-card ZoneCastGrants map (zone_cast.go) is keyed by a
// specific *Card pointer, which doesn't fit a grant that must apply
// to any qualifying card in a graveyard — graveyard contents change
// every turn. GraveyardFlashbackGrant is a predicate-style grant:
// the source registers one entry, and CastFlashback evaluates the
// predicate against the cast candidate at cast time.
//
// Cleanup is timestamp-driven. The source permanent's Timestamp is
// stored on the grant; LTB hooks (or end-of-turn cleanup) call
// ExpireGraveyardFlashbackGrantsBySource to drop them when the
// source leaves the battlefield.

// GraveyardFlashbackGrant describes a continuous "cards in player X's
// graveyard have flashback" effect emitted by a permanent.
type GraveyardFlashbackGrant struct {
	// Controller is the seat whose graveyard cards are eligible.
	Controller int

	// SourceTimestamp is the granting permanent's Timestamp, used
	// for LTB cleanup. 0 means "not tied to a permanent" — e.g. a
	// one-shot sorcery-resolution grant (Past in Flames, Will of
	// the Jeskai). Such grants set ExpiresAtCleanup=true and are
	// swept at end-of-turn cleanup; they are skipped by the
	// permanent-LTB and orphan-source passes.
	SourceTimestamp int

	// SourceName is the granting card's name (for logs).
	SourceName string

	// OnlyActiveTurn gates the grant to the controller's turn
	// (Iroh's "during your turn"). When false the grant is always
	// on (Lier).
	OnlyActiveTurn bool

	// ExpiresAtCleanup is true for one-shot "until end of turn"
	// grants emitted by spell resolution (Past in Flames-class).
	// ExpireEOTGraveyardFlashbackGrants drops these at the cleanup
	// step. Combined with SourceTimestamp==0 this isolates them
	// from the permanent-LTB pipeline that owns continuous grants
	// like Iroh / Lier.
	ExpiresAtCleanup bool

	// GrantTurn records the turn the grant was registered on. Used
	// by the EOT sweep so a grant registered on turn N is only
	// expired during cleanup of turn N (not on any future turn we
	// happen to revisit cleanup for).
	GrantTurn int

	// CostFor returns the flashback mana cost for `card`, or -1 if
	// the card is not eligible under this grant. Lets each source
	// encode its own filter and tiered costs:
	//
	//   - Iroh: returns -1 for non-instant/sorcery, 1 for Lesson,
	//     card.CMC otherwise.
	//   - Lier: returns -1 for non-instant/sorcery, card.CMC
	//     otherwise.
	//   - Past in Flames / Will of the Jeskai: returns -1 for
	//     non-instant/sorcery, card.CMC otherwise (one-shot).
	CostFor func(card *Card) int
}

// RegisterGraveyardFlashbackGrant appends a graveyard-flashback grant
// to gs. Idempotent under (SourceTimestamp, Controller): if a grant
// from the same source already exists it is replaced.
func RegisterGraveyardFlashbackGrant(gs *GameState, g *GraveyardFlashbackGrant) {
	if gs == nil || g == nil {
		return
	}
	for i, existing := range gs.GraveyardFlashbackGrants {
		if existing != nil &&
			existing.SourceTimestamp != 0 &&
			existing.SourceTimestamp == g.SourceTimestamp &&
			existing.Controller == g.Controller {
			gs.GraveyardFlashbackGrants[i] = g
			gs.LogEvent(Event{
				Kind:   "graveyard_flashback_grant_refreshed",
				Seat:   g.Controller,
				Source: g.SourceName,
				Details: map[string]interface{}{
					"only_active_turn": g.OnlyActiveTurn,
				},
			})
			return
		}
	}
	gs.GraveyardFlashbackGrants = append(gs.GraveyardFlashbackGrants, g)
	gs.LogEvent(Event{
		Kind:   "graveyard_flashback_grant_registered",
		Seat:   g.Controller,
		Source: g.SourceName,
		Details: map[string]interface{}{
			"only_active_turn": g.OnlyActiveTurn,
		},
	})
}

// ExpireGraveyardFlashbackGrantsBySource removes all grants whose
// SourceTimestamp matches the given permanent timestamp. Called from
// the LTB pipeline when the source leaves the battlefield.
func ExpireGraveyardFlashbackGrantsBySource(gs *GameState, sourceTimestamp int) {
	if gs == nil || sourceTimestamp == 0 || len(gs.GraveyardFlashbackGrants) == 0 {
		return
	}
	kept := gs.GraveyardFlashbackGrants[:0]
	for _, g := range gs.GraveyardFlashbackGrants {
		if g == nil {
			continue
		}
		if g.SourceTimestamp == sourceTimestamp {
			gs.LogEvent(Event{
				Kind:   "graveyard_flashback_grant_expired",
				Seat:   g.Controller,
				Source: g.SourceName,
				Details: map[string]interface{}{
					"reason": "source_left_battlefield",
				},
			})
			continue
		}
		kept = append(kept, g)
	}
	gs.GraveyardFlashbackGrants = kept
}

// ExpireOrphanedGraveyardFlashbackGrants drops any grant whose source
// permanent no longer exists on the battlefield. Called at end-of-turn
// cleanup as a safety net: per_card LTB handlers should call
// ExpireGraveyardFlashbackGrantsBySource directly, but this catches
// any missed paths the same way ExpireZoneCastGrants does for
// `while_source_on_bf` ZoneCastGrants.
func ExpireOrphanedGraveyardFlashbackGrants(gs *GameState) {
	if gs == nil || len(gs.GraveyardFlashbackGrants) == 0 {
		return
	}
	kept := gs.GraveyardFlashbackGrants[:0]
	for _, g := range gs.GraveyardFlashbackGrants {
		if g == nil {
			continue
		}
		// One-shot sorcery grants (Past in Flames, Will of the
		// Jeskai) have no source permanent — leave them to the
		// EOT-cleanup sweep below.
		if g.ExpiresAtCleanup {
			kept = append(kept, g)
			continue
		}
		if g.SourceTimestamp != 0 && !permanentWithTimestampExists(gs, g.SourceTimestamp) {
			gs.LogEvent(Event{
				Kind:   "graveyard_flashback_grant_expired",
				Seat:   g.Controller,
				Source: g.SourceName,
				Details: map[string]interface{}{
					"reason": "source_not_on_battlefield_eot_sweep",
				},
			})
			continue
		}
		kept = append(kept, g)
	}
	gs.GraveyardFlashbackGrants = kept
}

// RegisterEOTGraveyardFlashbackGrant registers a one-shot
// "until end of turn" graveyard-flashback grant from a spell that
// has no associated battlefield permanent (Past in Flames, Will of
// the Jeskai, Flashback the instant if a future variant wants the
// mass form). The grant is removed by the cleanup-step sweep
// ExpireEOTGraveyardFlashbackGrants on the same turn it was
// registered.
//
// `controller` is the seat whose graveyard the grant applies to.
// `sourceName` is the spell name for logs. `costFor` is the
// per-card cost predicate — return -1 for ineligible cards.
//
// The standard Past in Flames-class predicate
// (PrintedMassFlashbackCost) returns card.CMC for every instant
// and sorcery and -1 otherwise; callers needing that exact
// behavior should pass it instead of re-deriving.
func RegisterEOTGraveyardFlashbackGrant(gs *GameState, controller int, sourceName string, costFor func(card *Card) int) {
	if gs == nil || costFor == nil {
		return
	}
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	grant := &GraveyardFlashbackGrant{
		Controller:       controller,
		SourceTimestamp:  0,
		SourceName:       sourceName,
		OnlyActiveTurn:   false,
		ExpiresAtCleanup: true,
		GrantTurn:        gs.Turn,
		CostFor:          costFor,
	}
	gs.GraveyardFlashbackGrants = append(gs.GraveyardFlashbackGrants, grant)
	gs.LogEvent(Event{
		Kind:   "graveyard_flashback_grant_registered",
		Seat:   controller,
		Source: sourceName,
		Details: map[string]interface{}{
			"only_active_turn": false,
			"duration":         "until_end_of_turn",
			"grant_turn":       gs.Turn,
		},
	})
}

// ExpireEOTGraveyardFlashbackGrants removes every EOT grant whose
// GrantTurn matches the current turn — fired from the cleanup-step
// pipeline alongside ExpireOrphanedGraveyardFlashbackGrants. A
// grant registered on turn N during seat A's turn naturally expires
// at the next cleanup we see; if turn order shifts (extra turns,
// skip turn), grants registered earlier than the current turn are
// also flushed defensively.
func ExpireEOTGraveyardFlashbackGrants(gs *GameState) {
	if gs == nil || len(gs.GraveyardFlashbackGrants) == 0 {
		return
	}
	kept := gs.GraveyardFlashbackGrants[:0]
	for _, g := range gs.GraveyardFlashbackGrants {
		if g == nil {
			continue
		}
		if g.ExpiresAtCleanup && g.GrantTurn <= gs.Turn {
			gs.LogEvent(Event{
				Kind:   "graveyard_flashback_grant_expired",
				Seat:   g.Controller,
				Source: g.SourceName,
				Details: map[string]interface{}{
					"reason":     "until_end_of_turn",
					"grant_turn": g.GrantTurn,
				},
			})
			continue
		}
		kept = append(kept, g)
	}
	gs.GraveyardFlashbackGrants = kept
}

// PrintedMassFlashbackCost is the canonical CostFor predicate for
// Past in Flames-class mass grants: every instant and sorcery in
// the graveyard gets flashback at its printed mana cost; nothing
// else is eligible.
func PrintedMassFlashbackCost(card *Card) int {
	if card == nil {
		return -1
	}
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		return -1
	}
	return ManaCostOf(card)
}

// EffectiveFlashbackCostFromGraveyardGrants returns (cost, ok) — the
// flashback cost provided by any active GraveyardFlashbackGrant for
// `card` in `seatIdx`'s graveyard, evaluated against the current
// active turn. Returns the lowest cost across applicable grants (a
// gift to the caster when multiple sources stack — Iroh + Lier both
// active means the {1} Lesson cost beats the printed mana cost).
func EffectiveFlashbackCostFromGraveyardGrants(gs *GameState, seatIdx int, card *Card) (int, bool) {
	if gs == nil || card == nil || len(gs.GraveyardFlashbackGrants) == 0 {
		return 0, false
	}
	best := -1
	for _, g := range gs.GraveyardFlashbackGrants {
		if g == nil || g.CostFor == nil {
			continue
		}
		if g.Controller != seatIdx {
			continue
		}
		if g.OnlyActiveTurn && gs.Active != g.Controller {
			continue
		}
		cost := g.CostFor(card)
		if cost < 0 {
			continue
		}
		if best < 0 || cost < best {
			best = cost
		}
	}
	if best < 0 {
		return 0, false
	}
	return best, true
}
