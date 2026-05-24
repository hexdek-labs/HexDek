package gameengine

// Wave 2 — Cast-from-other-zones primitive.
//
// Comp-rules citations:
//
//   §702.33    Flashback — "You may cast this card from your graveyard
//              for its flashback cost. Then exile it." If a spell with
//              flashback would be put into the graveyard or any other
//              zone from the stack, exile it instead.
//   §702.138   Escape — "You may cast this card from your graveyard by
//              paying [escape cost] and exiling [N] other cards from
//              your graveyard." If the spell would be put into the
//              graveyard from the stack, exile it instead (same as
//              flashback).
//   §702.34    Madness — (future, not in this pass)
//   §112.6k    Static: "you may cast this card from exile"
//              (Misthollow Griffin, Squee the Immortal, Torrent
//              Elemental).
//   §601.3e    Playing from top of library (Bolas's Citadel, Future
//              Sight) — "you may play the top card of your library."
//
// This file provides:
//
//   - ZoneCastPermission struct
//   - CanCastFromZone check
//   - CastFromZone entry point (removes card from zone, pushes to stack)
//   - RegisterFlashback / RegisterEscape helpers
//   - Post-resolution exile logic (called from ResolveStackTop)
//
// Zone cast integrates with CostPaymentResult and the CostMeta field
// on StackItem to drive post-resolution zone routing.

import (
	"strings"
)

// ---------------------------------------------------------------------------
// Zone constants
// ---------------------------------------------------------------------------

const (
	ZoneHand        = "hand"
	ZoneGraveyard   = "graveyard"
	ZoneExile       = "exile"
	ZoneLibrary     = "library"
	ZoneCommandZone = "command_zone"
)

// drannithRestrictsZoneCast returns true if an opponent of `castingSeat`
// has a Drannith Magistrate on the battlefield, preventing zone casts.
// This is the engine-side consumer of the per_card flag — we check
// gs.Flags directly to avoid importing per_card (which would create a
// cycle). The flag key format matches drannith_magistrate.go's ETB hook.
func drannithRestrictsZoneCast(gs *GameState, castingSeat int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	for i := range gs.Seats {
		if i == castingSeat {
			continue
		}
		if gs.Flags["drannith_active_seat_"+itoa(i)] > 0 {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ZoneCastPermission — describes a card's ability to be cast from a
// non-hand zone.
// ---------------------------------------------------------------------------

// ZoneCastPermission describes a card's ability to be cast from a
// specific zone. Cards can have multiple permissions (e.g. Flashback
// from graveyard AND a static exile-cast permission).
type ZoneCastPermission struct {
	// Zone is the zone this permission allows casting from.
	Zone string

	// Keyword is the rules keyword granting this permission ("flashback",
	// "escape", "static_exile_cast", "library_cast").
	Keyword string

	// ManaCost is the mana cost to pay when casting from this zone.
	// -1 means "use the card's normal mana cost" (Misthollow Griffin).
	ManaCost int

	// AdditionalCosts are extra costs beyond mana (escape exiles N cards).
	AdditionalCosts []*AdditionalCost

	// LifeCostInsteadOfMana: if > 0, pay this much life instead of mana
	// (Bolas's Citadel).
	LifeCostInsteadOfMana int

	// ExileOnResolve: if true, the spell is exiled instead of going to
	// the graveyard after resolution. True for flashback and escape.
	ExileOnResolve bool

	// RequireController: if >= 0, only this seat may use this permission.
	// -1 means any player (typically the card's owner).
	RequireController int

	// SourceName is the card/permanent granting this permission (e.g.
	// "Underworld Breach" grants escape to graveyard instants/sorceries).
	SourceName string

	// Duration controls when this permission expires. Values:
	//   "until_end_of_turn"      — Light Up the Stage, Prosper, most impulse draw
	//   "until_end_of_next_turn" — Laelia, Faldorn
	//   "while_source_on_bf"     — Knowledge Pool, Gonti Lord of Luxury
	//   ""                       — permanent (Misthollow Griffin, flashback, escape)
	Duration string

	// GrantTurn is the turn number when this permission was created.
	// Used by cleanup to expire "until_end_of_turn" and "until_end_of_next_turn".
	GrantTurn int

	// SourceTimestamp is the Permanent.Timestamp of the source that granted
	// this permission. Used to expire "while_source_on_bf" grants when the
	// source leaves the battlefield.
	SourceTimestamp int

	// SpendAnyColor: if true, the caster may spend mana as though it were
	// any color to pay for this spell (CR §106.11 — Gonti, Fallen Shinobi,
	// Sen Triplets, Hostage Taker).
	SpendAnyColor bool

	// OncePerTurnPerSource: if true, only one spell may be cast through
	// permissions sharing this SourceTimestamp per turn (Kess, Maestros
	// Ascendancy, Karador, Lurrus, Gisa & Geralf, ...). The usage flag
	// lives on the source Permanent (gated by SourceTimestamp). CR §605
	// is not directly applicable — this is the "once during each of your
	// turns, you may cast" idiom which restricts the casting permission
	// itself, not the activated ability count.
	OncePerTurnPerSource bool
}

// permanentFlagKey is the slot on Permanent.Flags used to track that the
// once-per-turn cast permission has already been consumed this turn.
// Stored as the turn number it was used; 0 means "not used this turn".
const permanentOncePerTurnCastFlag = "zone_cast_once_per_turn_used"

// ---------------------------------------------------------------------------
// CanCastFromZone — check if a card can be cast from a given zone.
// ---------------------------------------------------------------------------

// CanCastFromZone returns the first applicable ZoneCastPermission for
// casting `card` from `zone` by `seatIdx`, or nil if not allowed.
//
// The caller provides a list of permissions (from per-card registration
// or from a global grants scan like Underworld Breach). This function
// checks zone match, controller match, and mana/cost affordability.
func CanCastFromZone(gs *GameState, seatIdx int, card *Card, zone string, perms []*ZoneCastPermission) *ZoneCastPermission {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	zLower := strings.ToLower(zone)

	for _, p := range perms {
		if p == nil {
			continue
		}
		if strings.ToLower(p.Zone) != zLower {
			continue
		}
		// Controller check.
		if p.RequireController >= 0 && p.RequireController != seatIdx {
			continue
		}
		// Mana affordability.
		manaCost := p.ManaCost
		if manaCost < 0 {
			manaCost = manaCostOf(card)
		}
		if p.LifeCostInsteadOfMana > 0 {
			// Bolas's Citadel: pay life instead of mana.
			if seat.Life <= p.LifeCostInsteadOfMana {
				continue
			}
		} else if seat.ManaPool < manaCost {
			continue
		}
		// Additional cost affordability.
		affordable := true
		for _, add := range p.AdditionalCosts {
			if !CanPayAdditionalCost(gs, seatIdx, add) {
				affordable = false
				break
			}
		}
		if !affordable {
			continue
		}
		// Once-per-turn-per-source budget (Kess, Maestros, Karador, ...).
		if oncePerTurnConsumed(gs, p) {
			continue
		}
		return p
	}
	return nil
}

// ---------------------------------------------------------------------------
// CastFromZone — cast a spell from a non-hand zone.
// ---------------------------------------------------------------------------

// CastFromZone casts a spell from the specified zone using the given
// ZoneCastPermission. The card is removed from its source zone, costs
// are paid, and it is pushed onto the stack.
//
// The CastZone field on the resulting StackItem is set so that
// ResolveStackTop knows to exile (flashback/escape) or apply other
// post-resolution zone routing.
//
// Returns (CostPaymentResult, error).
func CastFromZone(
	gs *GameState,
	seatIdx int,
	card *Card,
	zone string,
	perm *ZoneCastPermission,
	targets []Target,
) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if perm == nil {
		return nil, &CastError{Reason: "no zone_cast_permission"}
	}
	// Reject up-front when the once-per-turn budget on the granting
	// permanent is already spent this turn. We re-check inside the
	// payment block so the gate is honored even when callers bypass
	// CanCastFromZone (e.g. invariant tests).
	if oncePerTurnConsumed(gs, perm) {
		return nil, &CastError{Reason: "once_per_turn_consumed"}
	}
	seat := gs.Seats[seatIdx]

	// Timing checks.
	if SplitSecondActive(gs) {
		return nil, &CastError{Reason: "split_second"}
	}
	if len(gs.Stack) > 0 && OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return nil, &CastError{Reason: "sorcery_speed_restriction"}
	}

	// CR §601.2c target legality at announcement (mirrors CastSpell). See
	// ValidateTargetsAtAnnouncement for the §115.2 / §702.11 / §702.16 /
	// §702.18 contract this enforces.
	if len(targets) > 0 {
		if err := ValidateTargetsAtAnnouncement(gs, seatIdx, card, targets, nil); err != nil {
			return nil, err
		}
	}

	// Wave 3b: Drannith Magistrate — opponents can't cast spells from
	// anywhere other than their hands. This applies to ALL non-hand zone
	// casts (graveyard, exile, library, command zone). Check by scanning
	// for the drannith flag on opposing seats. CR §601.2a legality check.
	if zone != ZoneHand {
		if drannithRestrictsZoneCast(gs, seatIdx) {
			gs.LogEvent(Event{
				Kind:   "cast_suppressed",
				Seat:   seatIdx,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "drannith_magistrate",
					"zone":   zone,
					"rule":   "601.2a",
				},
			})
			return nil, &CastError{Reason: "drannith_magistrate"}
		}
	}

	// Remove from source zone. §601.2a.
	if !removeFromZone(seat, card, zone) {
		return nil, &CastError{Reason: "not_in_zone"}
	}

	result := &CostPaymentResult{}

	// Pay mana cost (or life cost for Bolas's Citadel).
	manaCost := perm.ManaCost
	if manaCost < 0 {
		manaCost = manaCostOf(card)
	}
	if perm.LifeCostInsteadOfMana > 0 {
		if seat.Life <= perm.LifeCostInsteadOfMana {
			// Put card back.
			addToZone(seat, card, zone)
			return nil, &CastError{Reason: "insufficient_life"}
		}
		LoseLife(gs, seatIdx, perm.LifeCostInsteadOfMana, card.DisplayName())
		seat.Turn.LifePaid += perm.LifeCostInsteadOfMana
		result.LifePaid += perm.LifeCostInsteadOfMana
	} else {
		if seat.ManaPool < manaCost {
			addToZone(seat, card, zone)
			return nil, &CastError{Reason: "insufficient_mana"}
		}
		seat.ManaPool -= manaCost
		SyncManaAfterSpend(seat)
		if manaCost > 0 {
			gs.LogEvent(Event{
				Kind:   "pay_mana",
				Seat:   seatIdx,
				Amount: manaCost,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"reason":  "zone_cast",
					"keyword": perm.Keyword,
					"rule":    "601.2f",
				},
			})
		}
	}

	// Pay additional costs (escape exile, etc.).
	for _, add := range perm.AdditionalCosts {
		if add == nil {
			continue
		}
		if !CanPayAdditionalCost(gs, seatIdx, add) {
			addToZone(seat, card, zone)
			return nil, &CastError{Reason: "cannot_pay_additional_cost"}
		}
		addResult := PayAdditionalCost(gs, seatIdx, card, add)
		if addResult == nil {
			addToZone(seat, card, zone)
			return nil, &CastError{Reason: "additional_cost_payment_failed"}
		}
		result.ExiledCards = append(result.ExiledCards, addResult.ExiledCards...)
		result.LifePaid += addResult.LifePaid
	}

	// Log cast.
	gs.LogEvent(Event{
		Kind:   "cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule":      "601.2",
			"from_zone": zone,
			"keyword":   perm.Keyword,
			"life_paid": result.LifePaid,
		},
	})

	// Cast-count bookkeeping.
	IncrementCastCount(gs, seatIdx)
	RecordCast(gs, seatIdx, card, 0)
	if zone == "exile" && seatIdx >= 0 && seatIdx < len(gs.Seats) && gs.Seats[seatIdx] != nil {
		gs.Seats[seatIdx].Turn.CastFromExile++
	}
	// Stamp the source permanent once costs are paid and the cast has
	// committed. We mark BEFORE stack resolution so cast-triggered re-
	// entry (e.g. a cast trigger that grants another cast) can't sneak a
	// second use through the same source this turn.
	markOncePerTurnConsumed(gs, perm)
	fireCastTriggersFromZone(gs, seatIdx, card, zone)
	FireCastTriggerObservers(gs, card, seatIdx, false)

	// Build stack item with zone metadata.
	eff := collectSpellEffect(card)
	item := &StackItem{
		Controller: seatIdx,
		Card:       card,
		Effect:     eff,
		Targets:    targets,
		CastZone:   zone,
		CostMeta:   map[string]interface{}{},
	}
	if perm.ExileOnResolve {
		item.CostMeta["exile_on_resolve"] = true
	}
	item.CostMeta["zone_cast_keyword"] = perm.Keyword
	PushStackItem(gs, item)

	// Storm.
	if HasStormKeyword(card) {
		ApplyStormCopies(gs, item, seatIdx)
	}
	InvokeCastHook(gs, item)

	// Priority + resolution (CR §117.4 + §608.2 + §727).
	PriorityRound(gs)
	DrainStack(gs)
	return result, nil
}

// ---------------------------------------------------------------------------
// ShouldExileOnResolve — called by ResolveStackTop to check if a
// spell should be exiled instead of going to the graveyard.
// ---------------------------------------------------------------------------

// ShouldExileOnResolve returns true if the stack item should be exiled
// after resolution instead of going to its normal post-resolution zone.
// This is true for flashback and escape casts.
func ShouldExileOnResolve(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["exile_on_resolve"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// ---------------------------------------------------------------------------
// Zone manipulation helpers
// ---------------------------------------------------------------------------

// removeFromZone removes a card (by pointer identity) from the given
// zone of the seat. Returns true on success.
func removeFromZone(seat *Seat, card *Card, zone string) bool {
	if seat == nil || card == nil {
		return false
	}
	var zoneSlice *[]*Card
	switch strings.ToLower(zone) {
	case "hand", "":
		zoneSlice = &seat.Hand
	case "graveyard":
		zoneSlice = &seat.Graveyard
	case "exile":
		zoneSlice = &seat.Exile
	case "library":
		// Remove from top of library only (Bolas's Citadel).
		if len(seat.Library) > 0 && seat.Library[0] == card {
			seat.Library = seat.Library[1:]
			return true
		}
		// Fallback: search the library.
		for i, c := range seat.Library {
			if c == card {
				seat.Library = append(seat.Library[:i], seat.Library[i+1:]...)
				return true
			}
		}
		return false
	case "command_zone":
		zoneSlice = &seat.CommandZone
	default:
		return false
	}
	for i, c := range *zoneSlice {
		if c == card {
			*zoneSlice = append((*zoneSlice)[:i], (*zoneSlice)[i+1:]...)
			return true
		}
	}
	return false
}

// addToZone adds a card back to a zone (for rollback on failed cost
// payment).
func addToZone(seat *Seat, card *Card, zone string) {
	if seat == nil || card == nil {
		return
	}
	switch strings.ToLower(zone) {
	case "hand", "":
		seat.Hand = append(seat.Hand, card)
	case "graveyard":
		seat.Graveyard = append(seat.Graveyard, card)
	case "exile":
		seat.Exile = append(seat.Exile, card)
	case "library":
		// Put back on top.
		seat.Library = append([]*Card{card}, seat.Library...)
	case "command_zone":
		// Idempotent insert — matches moveToZone (state.go:1576) and
		// the §903.9b redirect in commander.go. A rollback that races
		// an SBA §704.6d sweep would otherwise stack two copies of the
		// same *Card in command_zone.
		for _, existing := range seat.CommandZone {
			if existing == card {
				return
			}
		}
		seat.CommandZone = append(seat.CommandZone, card)
	}
}

// ---------------------------------------------------------------------------
// Flashback / Escape permission constructors
// ---------------------------------------------------------------------------

// NewFlashbackPermission creates a ZoneCastPermission for flashback.
// flashbackCost is the mana cost to pay (use -1 for "same as card CMC").
func NewFlashbackPermission(flashbackCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "flashback",
		ManaCost:          flashbackCost,
		ExileOnResolve:    true,
		RequireController: -1,
	}
}

// NewEscapePermission creates a ZoneCastPermission for escape.
// escapeCost is the mana cost, exileCount is the number of other
// graveyard cards to exile.
func NewEscapePermission(escapeCost int, exileCount int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:    ZoneGraveyard,
		Keyword: "escape",
		ManaCost: escapeCost,
		AdditionalCosts: []*AdditionalCost{
			{
				Kind:          AddCostKindExile,
				Label:         "exile cards from graveyard",
				ExileCount:    exileCount,
				ExileFromZone: "graveyard",
				ExileFilter:   "other",
			},
		},
		ExileOnResolve:    true,
		RequireController: -1,
	}
}

// NewExileCastPermission creates a ZoneCastPermission for static
// "you may cast this from exile" (Misthollow Griffin, Squee the
// Immortal).
func NewExileCastPermission() *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "static_exile_cast",
		ManaCost:          -1, // use card's normal mana cost
		ExileOnResolve:    false,
		RequireController: -1,
	}
}

// NewLibraryCastPermission creates a ZoneCastPermission for casting
// from top of library (Bolas's Citadel, Future Sight). lifeCost is
// the CMC-based life payment; set to 0 for Future Sight (which uses
// normal mana cost).
func NewLibraryCastPermission(lifeCost int) *ZoneCastPermission {
	p := &ZoneCastPermission{
		Zone:              ZoneLibrary,
		Keyword:           "library_cast",
		ExileOnResolve:    false,
		RequireController: -1,
	}
	if lifeCost > 0 {
		p.LifeCostInsteadOfMana = lifeCost
		p.ManaCost = 0
	} else {
		p.ManaCost = -1 // use card's normal mana cost
	}
	return p
}

// NewFreeCastFromExilePermission creates a ZoneCastPermission for
// "cast from exile without paying its mana cost" effects like Release
// to the Wind. The permission is controller-restricted.
func NewFreeCastFromExilePermission(controllerSeat int, sourceName string) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "free_exile_cast",
		ManaCost:          0,
		ExileOnResolve:    false,
		RequireController: controllerSeat,
		SourceName:        sourceName,
	}
}

// RegisterZoneCastGrant registers a per-card zone-cast permission on
// the game state. The AI/Hat consults this when deciding whether to
// cast a card from a non-hand zone.
func RegisterZoneCastGrant(gs *GameState, card *Card, perm *ZoneCastPermission) {
	if gs == nil || card == nil || perm == nil {
		return
	}
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
	}
	gs.ZoneCastGrants[card] = perm
	gs.LogEvent(Event{
		Kind:   "zone_cast_grant_registered",
		Seat:   perm.RequireController,
		Source: perm.SourceName,
		Details: map[string]interface{}{
			"card":    card.DisplayName(),
			"zone":    perm.Zone,
			"keyword": perm.Keyword,
			"cost":    perm.ManaCost,
		},
	})
}

// RemoveZoneCastGrant removes a per-card zone-cast permission. Called
// when the card leaves exile (it was cast or moved elsewhere).
func RemoveZoneCastGrant(gs *GameState, card *Card) {
	if gs == nil || card == nil || gs.ZoneCastGrants == nil {
		return
	}
	if p, ok := gs.ZoneCastGrants[card]; ok {
		emitGrantExpired(gs, card, p, "removed")
		delete(gs.ZoneCastGrants, card)
	}
}

// emitGrantExpired logs a zone_cast_grant_expired event so Heimdall can
// observe the lifecycle of zone-cast permissions.
func emitGrantExpired(gs *GameState, card *Card, p *ZoneCastPermission, reason string) {
	if gs == nil || card == nil || p == nil {
		return
	}
	gs.LogEvent(Event{
		Kind:   "zone_cast_grant_expired",
		Seat:   p.RequireController,
		Source: p.SourceName,
		Details: map[string]interface{}{
			"card":     card.DisplayName(),
			"zone":     p.Zone,
			"keyword":  p.Keyword,
			"duration": p.Duration,
			"reason":   reason,
		},
	})
}

// GetZoneCastGrant returns the zone-cast permission for a card, if any.
func GetZoneCastGrant(gs *GameState, card *Card) *ZoneCastPermission {
	if gs == nil || card == nil || gs.ZoneCastGrants == nil {
		return nil
	}
	return gs.ZoneCastGrants[card]
}

// NewBreachEscapePermission creates a ZoneCastPermission for
// Underworld Breach's granted escape (escape cost = card's mana cost
// + exile 3 other cards from graveyard).
func NewBreachEscapePermission(cardManaCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:    ZoneGraveyard,
		Keyword: "escape",
		ManaCost: cardManaCost,
		AdditionalCosts: []*AdditionalCost{
			{
				Kind:          AddCostKindExile,
				Label:         "exile three other cards from graveyard (Underworld Breach)",
				ExileCount:    3,
				ExileFromZone: "graveyard",
				ExileFilter:   "other",
			},
		},
		ExileOnResolve:    true,
		RequireController: -1,
		SourceName:        "Underworld Breach",
	}
}

// ---------------------------------------------------------------------------
// Zone-cast grant lifecycle — expiry and cleanup
// ---------------------------------------------------------------------------

// ExpireZoneCastGrants removes expired grants from gs.ZoneCastGrants.
// Called at end-of-turn cleanup (phases.go EndOfTurnCleanup).
func ExpireZoneCastGrants(gs *GameState) {
	if gs == nil || len(gs.ZoneCastGrants) == 0 {
		return
	}
	for card, p := range gs.ZoneCastGrants {
		if shouldExpireGrant(gs, p) {
			emitGrantExpired(gs, card, p, "duration_elapsed")
			delete(gs.ZoneCastGrants, card)
		}
	}
}

// ExpireSourceGrants removes all grants whose SourceTimestamp matches
// the given permanent's Timestamp. Called when a permanent with
// "while_source_on_bf" grants leaves the battlefield.
func ExpireSourceGrants(gs *GameState, sourceTimestamp int) {
	if gs == nil || len(gs.ZoneCastGrants) == 0 || sourceTimestamp == 0 {
		return
	}
	for card, p := range gs.ZoneCastGrants {
		if p.SourceTimestamp == sourceTimestamp {
			emitGrantExpired(gs, card, p, "source_left_battlefield")
			delete(gs.ZoneCastGrants, card)
		}
	}
}

func shouldExpireGrant(gs *GameState, p *ZoneCastPermission) bool {
	if p == nil {
		return true
	}
	switch p.Duration {
	case "until_end_of_turn":
		return gs.Turn >= p.GrantTurn
	case "until_end_of_next_turn":
		return gs.Turn > p.GrantTurn
	case "while_source_on_bf":
		return !permanentWithTimestampExists(gs, p.SourceTimestamp)
	}
	return false
}

// grantIsLeaked reports whether a grant SHOULD have been cleaned up by
// a previous turn's cleanup pass and is therefore a true invariant
// violation. Distinct from shouldExpireGrant (which the cleanup itself
// uses): the invariant must NOT flag a grant registered earlier in
// the current turn — that grant is still alive until its own turn's
// cleanup runs. Using `>=` (cleanup semantics) inside the invariant
// false-positives on every grant observed before end-of-turn cleanup,
// which is most of the time. See Loki r60 / game 536 (Illusionary
// Mask → Midnight Covenant grant flagged at turn 55 draw step).
func grantIsLeaked(gs *GameState, p *ZoneCastPermission) bool {
	if p == nil {
		return true
	}
	switch p.Duration {
	case "until_end_of_turn":
		return gs.Turn > p.GrantTurn
	case "until_end_of_next_turn":
		return gs.Turn > p.GrantTurn+1
	case "while_source_on_bf":
		return !permanentWithTimestampExists(gs, p.SourceTimestamp)
	}
	return false
}

func permanentWithTimestampExists(gs *GameState, ts int) bool {
	return findPermanentByTimestamp(gs, ts) != nil
}

// findPermanentByTimestamp returns the on-battlefield permanent whose
// Timestamp matches ts, or nil if no such permanent exists. Used to
// resolve a ZoneCastPermission's SourceTimestamp back to the granting
// permanent so we can read/write its Flags (for once-per-turn gating).
func findPermanentByTimestamp(gs *GameState, ts int) *Permanent {
	if gs == nil || ts == 0 {
		return nil
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm != nil && perm.Timestamp == ts {
				return perm
			}
		}
	}
	return nil
}

// NewOncePerTurnGraveyardCastPermission builds a ZoneCastPermission for
// the "Once during each of your turns, you may cast a [filter] spell
// from your graveyard" family (Kess Dissident Mage, Maestros Ascendancy,
// Karador, Lurrus of the Dream-Den, Gisa & Geralf, ...).
//
// The permission attaches to a single graveyard card via
// RegisterZoneCastGrant. Callers are expected to scan the controller's
// graveyard at ETB and register one permission per eligible card, then
// refresh on graveyard-change events. The once-per-turn budget is
// enforced across all permissions sharing the same SourceTimestamp via
// the source permanent's Flags slot.
//
// Parameters:
//
//   controllerSeat   — RequireController; only this seat may cast.
//   sourceName       — Card name granting the permission (for logs).
//   sourceTimestamp  — Permanent.Timestamp of the granting permanent;
//                      used for once-per-turn gating + lifecycle.
//   manaCost         — -1 to use the card's own mana cost, or a fixed
//                      override (e.g. an alternative flat cost).
//   exileOnResolve   — true for Kess/Maestros ("If a spell cast this
//                      way would be put into your graveyard, exile it
//                      instead"); false for plain reanimate-style
//                      privileges.
//   additionalCosts  — extra costs like "sacrifice a creature"
//                      (Maestros), or "exile three other cards"
//                      (Kotis). May be nil for Kess/Karador.
func NewOncePerTurnGraveyardCastPermission(
	controllerSeat int,
	sourceName string,
	sourceTimestamp int,
	manaCost int,
	exileOnResolve bool,
	additionalCosts []*AdditionalCost,
) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:                 ZoneGraveyard,
		Keyword:              "once_per_turn_cast_from_graveyard",
		ManaCost:             manaCost,
		AdditionalCosts:      additionalCosts,
		ExileOnResolve:       exileOnResolve,
		RequireController:    controllerSeat,
		SourceName:           sourceName,
		SourceTimestamp:      sourceTimestamp,
		Duration:             "while_source_on_bf",
		OncePerTurnPerSource: true,
	}
}

// oncePerTurnConsumed returns true if the once-per-turn budget on the
// source permanent (resolved via SourceTimestamp) has already been
// spent this turn.
func oncePerTurnConsumed(gs *GameState, p *ZoneCastPermission) bool {
	if gs == nil || p == nil || !p.OncePerTurnPerSource {
		return false
	}
	src := findPermanentByTimestamp(gs, p.SourceTimestamp)
	if src == nil {
		// Source no longer on battlefield — grant should have expired;
		// treat as consumed to avoid an off-by-one with cleanup timing.
		return true
	}
	if src.Flags == nil {
		return false
	}
	return src.Flags[permanentOncePerTurnCastFlag] == gs.Turn
}

// markOncePerTurnConsumed stamps the source permanent so subsequent
// CanCastFromZone calls reject further casts this turn.
func markOncePerTurnConsumed(gs *GameState, p *ZoneCastPermission) {
	if gs == nil || p == nil || !p.OncePerTurnPerSource {
		return
	}
	src := findPermanentByTimestamp(gs, p.SourceTimestamp)
	if src == nil {
		return
	}
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags[permanentOncePerTurnCastFlag] = gs.Turn
}
