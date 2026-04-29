package gameengine

// Wave 2 — Alternative and additional cost enforcement.
//
// Comp-rules citations:
//
//   §118.6     Alternative costs — "you may pay [alt] rather than pay
//              this spell's mana cost." Only one alternative cost may
//              apply to a given spell (§118.6a). Commander tax stacks on
//              top of alt costs per §903.8 errata, but we defer that
//              interaction to a future pass.
//   §118.8     Additional costs — "as an additional cost to cast this
//              spell, [do something]." Additional costs are cumulative.
//   §601.2b    Choose modes/targets, THEN §601.2f pay costs.
//   §702.73    Evoke — "you may cast this spell for its evoke cost. If
//              you do, it's sacrificed when it enters the battlefield."
//
// This file provides:
//
//   - AlternativeCost / AdditionalCost structs
//   - CanPayAlternativeCost / PayAlternativeCost helpers
//   - CanPayAdditionalCost / PayAdditionalCost helpers
//   - ControlsCommander check for Fierce Guardianship et al.
//   - CardHasColor exported check for pitch-spell exile requirement
//
// The CastSpell path in stack.go calls into these when the card has
// registered alt/additional costs. Per-card handlers in per_card/
// populate the cost definitions on the Card or via a cost registry.

import (
	"strings"
)

// ---------------------------------------------------------------------------
// Cost kind constants
// ---------------------------------------------------------------------------

const (
	// AltCostKindPitch — exile a card from hand (optionally with color
	// requirement) + optionally pay life. Force of Will, Force of
	// Negation, Commandeer.
	AltCostKindPitch = "pitch"

	// AltCostKindCommanderFree — free if you control your commander.
	// Fierce Guardianship, Deadly Rollick, Deflecting Swat, Flawless
	// Maneuver.
	AltCostKindCommanderFree = "commander_free"

	// AltCostKindEvoke — pay the evoke cost instead of mana cost;
	// sacrifice on ETB. Endurance, Subtlety, Grief, Fury, Solitude.
	AltCostKindEvoke = "evoke"

	// AltCostKindSpellCount — free if N+ spells were cast this turn.
	// Mindbreak Trap (3+ opponent spells).
	AltCostKindSpellCount = "spell_count"

	// AddCostKindSacrifice — sacrifice a permanent as additional cost.
	// Culling the Weak, Natural Order, Eldritch Evolution, Diabolic
	// Intent, Soldevi Adnate.
	AddCostKindSacrifice = "sacrifice"

	// AddCostKindBargain — CR §702.166 — sacrifice an artifact,
	// enchantment, or token as you cast this spell. Optional additional
	// cost that enhances the spell if paid.
	AddCostKindBargain = "bargain"

	// AddCostKindExile — exile card(s) from a zone as additional cost.
	// Delve, Escape (exile N from graveyard), Treasure Cruise.
	AddCostKindExile = "exile"

	// AddCostKindPayLife — pay life as additional cost. Bolas's Citadel
	// (pay life = CMC), Dismember, Snuff Out.
	AddCostKindPayLife = "pay_life"
)

// ---------------------------------------------------------------------------
// AlternativeCost — §118.6
// ---------------------------------------------------------------------------

// AlternativeCost describes a way to cast a spell WITHOUT paying its
// normal mana cost. Only ONE alternative cost may apply per spell
// (§118.6a). The engine chooses among alternatives via the Hat (or
// an inline heuristic when no Hat is present).
type AlternativeCost struct {
	Kind string // AltCostKind* constant

	// Label is the human-readable description ("Force of Will pitch",
	// "evoke {1}{G}"). Used for event logging.
	Label string

	// --- Pitch costs ---

	// ExileColor: the color the exiled card must have (e.g. "U" for
	// Force of Will). Empty string = any card.
	ExileColor string

	// LifeCost: life to pay as part of the alt cost (1 for FoW).
	LifeCost int

	// --- Evoke costs ---

	// EvokeMana: the mana cost of the evoke alternative. Stored as a
	// simple int (generic mana) for MVP; typed mana is a Phase 8+ concern.
	EvokeMana int

	// --- Spell-count ---

	// SpellCountThreshold: the number of spells that must have been
	// cast this turn for this alt cost to be available.
	SpellCountThreshold int

	// SpellCountOpponentOnly: if true, only opponent spells count
	// toward the threshold (Mindbreak Trap).
	SpellCountOpponentOnly bool

	// --- Commander free ---
	// No extra fields needed — the engine checks ControlsCommander().

	// CanPayFn is an optional custom predicate. When non-nil, it
	// overrides the built-in CanPay logic. Used for exotic costs.
	CanPayFn func(gs *GameState, seatIdx int) bool

	// PayFn is an optional custom payment function. When non-nil, it
	// overrides the built-in Pay logic.
	PayFn func(gs *GameState, seatIdx int) bool
}

// ---------------------------------------------------------------------------
// AdditionalCost — §118.8
// ---------------------------------------------------------------------------

// AdditionalCost describes an extra cost paid ON TOP of the mana cost
// (or an alternative cost). Multiple additional costs can stack.
type AdditionalCost struct {
	Kind string // AddCostKind* constant

	// Label is the human-readable description.
	Label string

	// --- Sacrifice ---

	// SacrificeFilter: what kind of permanent to sacrifice. Supported
	// values: "creature", "green creature", "artifact", "permanent", "".
	// Empty string = any permanent.
	SacrificeFilter string

	// --- Exile from zone ---

	// ExileCount: number of cards to exile.
	ExileCount int

	// ExileFromZone: zone to exile from ("graveyard", "hand", "exile").
	ExileFromZone string

	// ExileFilter: optional filter ("other" = not this card).
	ExileFilter string

	// --- Pay life ---

	// LifeAmount: life to pay.
	LifeAmount int

	// CanPayFn is an optional custom predicate.
	CanPayFn func(gs *GameState, seatIdx int) bool

	// PayFn is an optional custom payment function.
	PayFn func(gs *GameState, seatIdx int) bool
}

// ---------------------------------------------------------------------------
// CostPaymentResult tracks what was paid so the engine can log and apply
// downstream effects (e.g. evoke → sacrifice on ETB).
// ---------------------------------------------------------------------------

// CostPaymentResult captures the outcome of paying an alternative or
// additional cost. CastSpell and CastFromZone use this to drive
// post-payment effects like evoke sacrifice triggers.
type CostPaymentResult struct {
	// UsedAlternativeCost is true when an alternative cost was chosen
	// instead of the normal mana cost.
	UsedAlternativeCost bool

	// AltCostKind records which alternative cost was used (empty if none).
	AltCostKind string

	// AltCostLabel is the human-readable label of the alt cost used.
	AltCostLabel string

	// EvokeUsed is true when evoke was the chosen alternative cost.
	// The ETB path reads this to register the sacrifice trigger.
	EvokeUsed bool

	// SacrificedPermanents holds pointers to permanents sacrificed as
	// additional costs.
	SacrificedPermanents []*Permanent

	// ExiledCards holds pointers to cards exiled as part of costs.
	ExiledCards []*Card

	// LifePaid is the total life paid across all costs.
	LifePaid int
}

// ---------------------------------------------------------------------------
// CanPay / Pay — AlternativeCost
// ---------------------------------------------------------------------------

// CanPayAlternativeCost returns true if the given alternative cost is
// currently payable by seatIdx.
func CanPayAlternativeCost(gs *GameState, seatIdx int, card *Card, alt *AlternativeCost) bool {
	if gs == nil || alt == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if alt.CanPayFn != nil {
		return alt.CanPayFn(gs, seatIdx)
	}
	seat := gs.Seats[seatIdx]
	switch alt.Kind {
	case AltCostKindPitch:
		return canPayPitchCost(gs, seat, card, alt)
	case AltCostKindCommanderFree:
		return ControlsCommander(gs, seatIdx)
	case AltCostKindEvoke:
		return seat.ManaPool >= alt.EvokeMana
	case AltCostKindSpellCount:
		return meetsSpellCountThreshold(gs, seatIdx, alt)
	}
	return false
}

// PayAlternativeCost executes the given alternative cost. Returns a
// CostPaymentResult on success, or nil if payment failed (state is NOT
// rolled back on failure — caller should check CanPay first).
func PayAlternativeCost(gs *GameState, seatIdx int, card *Card, alt *AlternativeCost) *CostPaymentResult {
	if gs == nil || alt == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	if alt.PayFn != nil {
		if alt.PayFn(gs, seatIdx) {
			return &CostPaymentResult{
				UsedAlternativeCost: true,
				AltCostKind:         alt.Kind,
				AltCostLabel:        alt.Label,
			}
		}
		return nil
	}
	seat := gs.Seats[seatIdx]
	result := &CostPaymentResult{
		UsedAlternativeCost: true,
		AltCostKind:         alt.Kind,
		AltCostLabel:        alt.Label,
	}

	switch alt.Kind {
	case AltCostKindPitch:
		exiled := payPitchCost(gs, seat, card, alt)
		if exiled == nil {
			return nil
		}
		result.ExiledCards = append(result.ExiledCards, exiled)
		if alt.LifeCost > 0 {
			seat.Life -= alt.LifeCost
			result.LifePaid += alt.LifeCost
			gs.LogEvent(Event{
				Kind:   "pay_life",
				Seat:   seatIdx,
				Amount: alt.LifeCost,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "alternative_cost_pitch",
					"rule":   "118.6",
				},
			})
		}
	case AltCostKindCommanderFree:
		// No actual payment — being free is the point.
		gs.LogEvent(Event{
			Kind:   "alternative_cost_free",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "commander_on_battlefield",
				"rule":   "118.6",
			},
		})
	case AltCostKindEvoke:
		if seat.ManaPool < alt.EvokeMana {
			return nil
		}
		seat.ManaPool -= alt.EvokeMana
		SyncManaAfterSpend(seat)
		result.EvokeUsed = true
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: alt.EvokeMana,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "evoke",
				"rule":   "702.73",
			},
		})
	case AltCostKindSpellCount:
		// Free — the threshold check was the gate.
		gs.LogEvent(Event{
			Kind:   "alternative_cost_free",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":    "spell_count_threshold",
				"threshold": alt.SpellCountThreshold,
				"rule":      "118.6",
			},
		})
	default:
		return nil
	}
	return result
}

// ---------------------------------------------------------------------------
// CanPay / Pay — AdditionalCost
// ---------------------------------------------------------------------------

// CanPayAdditionalCost returns true if the given additional cost is
// currently payable by seatIdx.
func CanPayAdditionalCost(gs *GameState, seatIdx int, add *AdditionalCost) bool {
	if gs == nil || add == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if add.CanPayFn != nil {
		return add.CanPayFn(gs, seatIdx)
	}
	seat := gs.Seats[seatIdx]
	switch add.Kind {
	case AddCostKindSacrifice:
		return findSacrificeCandidate(gs, seat, add.SacrificeFilter) != nil
	case AddCostKindBargain:
		return findBargainCandidate(gs, seat) != nil
	case AddCostKindExile:
		return countExilableCards(gs, seat, add) >= add.ExileCount
	case AddCostKindPayLife:
		return seat.Life > add.LifeAmount // must survive paying
	}
	return false
}

// PayAdditionalCost executes the given additional cost. Returns a partial
// CostPaymentResult merged into the caller's aggregate result.
func PayAdditionalCost(gs *GameState, seatIdx int, card *Card, add *AdditionalCost) *CostPaymentResult {
	if gs == nil || add == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	if add.PayFn != nil {
		if add.PayFn(gs, seatIdx) {
			return &CostPaymentResult{}
		}
		return nil
	}
	seat := gs.Seats[seatIdx]
	result := &CostPaymentResult{}

	switch add.Kind {
	case AddCostKindSacrifice:
		victim := findSacrificeCandidate(gs, seat, add.SacrificeFilter)
		if victim == nil {
			return nil
		}
		// Route through SacrificePermanent for proper §701.17 handling:
		// replacement effects (Rest in Peace, etc.), dies/LTB triggers,
		// and commander redirect. Fixes ZoneConservation violations where
		// the old removePermanent + manual graveyard add could leak cards.
		if sacrificePermanentImpl(gs, victim, nil, "additional_cost") {
			result.SacrificedPermanents = append(result.SacrificedPermanents, victim)
		} else {
			return nil
		}
	case AddCostKindBargain:
		victim := findBargainCandidate(gs, seat)
		if victim == nil {
			return nil
		}
		if sacrificePermanentImpl(gs, victim, nil, "bargain") {
			result.SacrificedPermanents = append(result.SacrificedPermanents, victim)
			gs.LogEvent(Event{
				Kind:   "bargain_paid",
				Seat:   seatIdx,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"sacrificed": victim.Card.DisplayName(),
					"rule":       "702.166",
				},
			})
		} else {
			return nil
		}
	case AddCostKindExile:
		exiled := exileCardsForCost(gs, seat, card, add)
		if len(exiled) < add.ExileCount {
			return nil
		}
		result.ExiledCards = append(result.ExiledCards, exiled...)
	case AddCostKindPayLife:
		if seat.Life <= add.LifeAmount {
			return nil
		}
		seat.Life -= add.LifeAmount
		result.LifePaid += add.LifeAmount
		gs.LogEvent(Event{
			Kind:   "pay_life",
			Seat:   seatIdx,
			Amount: add.LifeAmount,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "additional_cost",
				"rule":   "118.8",
			},
		})
	default:
		return nil
	}
	return result
}

// ---------------------------------------------------------------------------
// ControlsCommander — battlefield check for Fierce Guardianship et al.
// ---------------------------------------------------------------------------

// ControlsCommander returns true if seatIdx controls at least one of
// their own commanders on the battlefield. This gates the "free if you
// control a commander" family (Fierce Guardianship, Deadly Rollick,
// Deflecting Swat, Flawless Maneuver).
func ControlsCommander(gs *GameState, seatIdx int) bool {
	if gs == nil || !gs.CommanderFormat || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if len(seat.CommanderNames) == 0 {
		return false
	}
	for _, perm := range seat.Battlefield {
		if perm == nil || perm.Card == nil || perm.Controller != seatIdx {
			continue
		}
		for _, cname := range seat.CommanderNames {
			if DFCCardMatchesName(perm.Card, cname) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// CardHasColor — exported color check for pitch spells
// ---------------------------------------------------------------------------

// CardHasColor returns true if the card has the specified color in its
// Colors slice. Color codes: "W", "U", "B", "R", "G".
func CardHasColor(card *Card, color string) bool {
	if card == nil || color == "" {
		return false
	}
	want := strings.ToUpper(color)
	for _, c := range card.Colors {
		if strings.ToUpper(c) == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// canPayPitchCost checks if the seat has a card in hand (other than the
// spell being cast) that matches the required color for pitch.
func canPayPitchCost(gs *GameState, seat *Seat, spellCard *Card, alt *AlternativeCost) bool {
	if seat == nil {
		return false
	}
	// Need at least 1 life if LifeCost > 0.
	if alt.LifeCost > 0 && seat.Life <= alt.LifeCost {
		return false
	}
	// Need a card in hand matching the color (not the spell itself).
	for _, c := range seat.Hand {
		if c == spellCard {
			continue
		}
		if c == nil {
			continue
		}
		if alt.ExileColor == "" || CardHasColor(c, alt.ExileColor) {
			return true
		}
	}
	return false
}

// payPitchCost exiles one card from hand matching the color requirement
// and returns the exiled card. Returns nil on failure.
func payPitchCost(gs *GameState, seat *Seat, spellCard *Card, alt *AlternativeCost) *Card {
	if seat == nil {
		return nil
	}
	for _, c := range seat.Hand {
		if c == spellCard || c == nil {
			continue
		}
		if alt.ExileColor != "" && !CardHasColor(c, alt.ExileColor) {
			continue
		}
		// Exile this card.
		MoveCard(gs, c, seat.Idx, "hand", "exile", "exile-from-hand")
		gs.LogEvent(Event{
			Kind:   "exile",
			Seat:   seat.Idx,
			Source: spellCard.DisplayName(),
			Details: map[string]interface{}{
				"exiled":  c.DisplayName(),
				"from":    "hand",
				"reason":  "pitch_cost",
				"rule":    "118.6",
				"color":   alt.ExileColor,
			},
		})
		return c
	}
	return nil
}

// meetsSpellCountThreshold checks if enough spells have been cast this
// turn to meet the alt cost threshold.
func meetsSpellCountThreshold(gs *GameState, seatIdx int, alt *AlternativeCost) bool {
	if alt.SpellCountOpponentOnly {
		// Count spells cast by opponents this turn.
		total := gs.SpellsCastThisTurn
		if seatIdx >= 0 && seatIdx < len(gs.Seats) && gs.Seats[seatIdx] != nil {
			total -= gs.Seats[seatIdx].SpellsCastThisTurn
		}
		return total >= alt.SpellCountThreshold
	}
	return gs.SpellsCastThisTurn >= alt.SpellCountThreshold
}

// findSacrificeCandidate returns the first permanent on seat's
// battlefield matching the filter, or nil. For Hat integration, a
// future pass would let the Hat choose which creature to sacrifice.
func findSacrificeCandidate(gs *GameState, seat *Seat, filter string) *Permanent {
	if seat == nil {
		return nil
	}
	f := strings.ToLower(filter)
	for _, perm := range seat.Battlefield {
		if perm == nil || perm.Card == nil {
			continue
		}
		if perm.Controller != seat.Idx {
			continue
		}
		switch f {
		case "creature":
			if perm.IsCreature() {
				return perm
			}
		case "green creature":
			if perm.IsCreature() && CardHasColor(perm.Card, "G") {
				return perm
			}
		case "artifact":
			if perm.IsArtifact() {
				return perm
			}
		case "enchantment":
			if perm.IsEnchantment() {
				return perm
			}
		case "permanent", "":
			return perm
		default:
			// Unknown filter — try creature as fallback.
			if perm.IsCreature() {
				return perm
			}
		}
	}
	return nil
}

// countExilableCards returns how many cards in the specified zone match
// the exile filter.
func countExilableCards(gs *GameState, seat *Seat, add *AdditionalCost) int {
	if seat == nil {
		return 0
	}
	var zone []*Card
	switch strings.ToLower(add.ExileFromZone) {
	case "graveyard":
		zone = seat.Graveyard
	case "hand":
		zone = seat.Hand
	case "exile":
		zone = seat.Exile
	default:
		zone = seat.Graveyard
	}
	count := 0
	for _, c := range zone {
		if c == nil {
			continue
		}
		// "other" filter means the card must not be the spell itself.
		// For escape, this is "exile three OTHER cards from your graveyard".
		// We don't have the spell card reference here, so "other" is
		// approximated as "count all" — the cast path excludes the spell
		// card before calling.
		count++
	}
	return count
}

// exileCardsForCost exiles N cards from the specified zone for the
// additional cost. Returns the exiled cards.
func exileCardsForCost(gs *GameState, seat *Seat, spellCard *Card, add *AdditionalCost) []*Card {
	if seat == nil || add.ExileCount <= 0 {
		return nil
	}
	zoneName := strings.ToLower(add.ExileFromZone)
	if zoneName == "" {
		zoneName = "graveyard"
	}
	var zonePtr *[]*Card
	switch zoneName {
	case "graveyard":
		zonePtr = &seat.Graveyard
	case "hand":
		zonePtr = &seat.Hand
	case "exile":
		zonePtr = &seat.Exile
	default:
		zonePtr = &seat.Graveyard
	}

	exiled := make([]*Card, 0, add.ExileCount)
	remaining := make([]*Card, 0, len(*zonePtr))
	for _, c := range *zonePtr {
		if c == nil {
			continue
		}
		// Skip the spell card itself for "other" filter.
		if add.ExileFilter == "other" && c == spellCard {
			remaining = append(remaining, c)
			continue
		}
		if len(exiled) < add.ExileCount {
			exiled = append(exiled, c)
		} else {
			remaining = append(remaining, c)
		}
	}
	// Don't double-add to exile zone if we're exiling FROM exile.
	if zoneName == "exile" {
		// Cards stay in exile; no zone move needed.
		// But we still need to mark them as "used" — for escape, they
		// just stay exiled. No removal from exile zone.
		return exiled
	}
	*zonePtr = remaining
	// Route each exiled card through MoveCard so §614 replacements /
	// §903.9b commander redirect / trigger dispatch fire. Source zone
	// already drained via *zonePtr = remaining, so MoveCard's internal
	// removal is a no-op.
	for _, c := range exiled {
		MoveCard(gs, c, seat.Idx, zoneName, "exile", "effect")
	}

	if len(exiled) > 0 {
		names := make([]string, 0, len(exiled))
		for _, c := range exiled {
			names = append(names, c.DisplayName())
		}
		gs.LogEvent(Event{
			Kind:   "exile",
			Seat:   seat.Idx,
			Source: spellCard.DisplayName(),
			Amount: len(exiled),
			Details: map[string]interface{}{
				"exiled_names": names,
				"from":         zoneName,
				"reason":       "additional_cost",
				"rule":         "118.8",
			},
		})
	}
	return exiled
}

// ---------------------------------------------------------------------------
// CastSpellWithCosts — high-level entry point that wraps CastSpell with
// alt/additional cost support.
// ---------------------------------------------------------------------------

// CastSpellWithCosts casts a spell from hand, optionally paying an
// alternative cost instead of the normal mana cost, and paying any
// additional costs. This is the preferred entry point for cards that
// have alt/additional costs defined.
//
// Parameters:
//   - altCost: if non-nil and chosen, replaces the mana cost payment.
//   - addCosts: additional costs paid on top (sacrifice, exile, etc.).
//   - useAlt: if true, the alternative cost is used; if false, normal
//     mana cost is paid.
//
// Returns error on failure. On success, the spell is on the stack and
// resolving per normal CastSpell flow.
func CastSpellWithCosts(
	gs *GameState,
	seatIdx int,
	card *Card,
	targets []Target,
	altCost *AlternativeCost,
	addCosts []*AdditionalCost,
	useAlt bool,
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
	seat := gs.Seats[seatIdx]

	// Timing checks — same as CastSpell.
	if SplitSecondActive(gs) {
		return nil, &CastError{Reason: "split_second"}
	}
	if len(gs.Stack) > 0 && OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return nil, &CastError{Reason: "sorcery_speed_restriction"}
	}

	// Remove from hand. §601.2a.
	if !removeFromHand(seat, card) {
		return nil, &CastError{Reason: "not_in_hand"}
	}

	result := &CostPaymentResult{}

	// Pay costs.
	if useAlt && altCost != nil {
		// Alternative cost — replaces mana cost entirely.
		if !CanPayAlternativeCost(gs, seatIdx, card, altCost) {
			seat.Hand = append(seat.Hand, card)
			return nil, &CastError{Reason: "cannot_pay_alternative_cost"}
		}
		altResult := PayAlternativeCost(gs, seatIdx, card, altCost)
		if altResult == nil {
			seat.Hand = append(seat.Hand, card)
			return nil, &CastError{Reason: "alternative_cost_payment_failed"}
		}
		result.UsedAlternativeCost = altResult.UsedAlternativeCost
		result.AltCostKind = altResult.AltCostKind
		result.AltCostLabel = altResult.AltCostLabel
		result.EvokeUsed = altResult.EvokeUsed
		result.ExiledCards = append(result.ExiledCards, altResult.ExiledCards...)
		result.LifePaid += altResult.LifePaid
	} else {
		// Normal mana cost.
		cost := manaCostOf(card)
		if seat.ManaPool < cost {
			seat.Hand = append(seat.Hand, card)
			return nil, &CastError{Reason: "insufficient_mana"}
		}
		seat.ManaPool -= cost
		SyncManaAfterSpend(seat)
		if cost > 0 {
			gs.LogEvent(Event{
				Kind:   "pay_mana",
				Seat:   seatIdx,
				Amount: cost,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "cast",
					"rule":   "601.2f",
				},
			})
		}
	}

	// Pay additional costs.
	for _, add := range addCosts {
		if add == nil {
			continue
		}
		if !CanPayAdditionalCost(gs, seatIdx, add) {
			// Roll back: put card back in hand. In a real engine we'd
			// roll back ALL costs, but for MVP this is sufficient.
			seat.Hand = append(seat.Hand, card)
			return nil, &CastError{Reason: "cannot_pay_additional_cost"}
		}
		addResult := PayAdditionalCost(gs, seatIdx, card, add)
		if addResult == nil {
			seat.Hand = append(seat.Hand, card)
			return nil, &CastError{Reason: "additional_cost_payment_failed"}
		}
		result.SacrificedPermanents = append(result.SacrificedPermanents, addResult.SacrificedPermanents...)
		result.ExiledCards = append(result.ExiledCards, addResult.ExiledCards...)
		result.LifePaid += addResult.LifePaid
	}

	// Log the cast event.
	gs.LogEvent(Event{
		Kind:   "cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule":                 "601.2",
			"used_alternative":     result.UsedAlternativeCost,
			"alternative_cost":     result.AltCostKind,
			"evoke":                result.EvokeUsed,
			"life_paid":            result.LifePaid,
			"exiled_count":         len(result.ExiledCards),
			"sacrificed_count":     len(result.SacrificedPermanents),
		},
	})

	// Cast-count bookkeeping.
	IncrementCastCount(gs, seatIdx)
	fireCastTriggers(gs, seatIdx, card)
	FireCastTriggerObservers(gs, card, seatIdx, false)

	// Build stack item.
	eff := collectSpellEffect(card)
	item := &StackItem{
		Controller: seatIdx,
		Card:       card,
		Effect:     eff,
		Targets:    targets,
	}
	// Tag the stack item with cost metadata for downstream effects.
	if result.EvokeUsed {
		if item.CostMeta == nil {
			item.CostMeta = map[string]interface{}{}
		}
		item.CostMeta["evoke"] = true
	}
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
// findBargainCandidate — CR §702.166
// ---------------------------------------------------------------------------

// findBargainCandidate returns the cheapest artifact, enchantment, or token
// on the seat's battlefield that can be sacrificed for bargain. Returns nil
// if no valid candidate exists. GreedyHat policy: sacrifice the cheapest
// eligible permanent.
func findBargainCandidate(gs *GameState, seat *Seat) *Permanent {
	if seat == nil {
		return nil
	}
	var best *Permanent
	bestCMC := 999
	for _, perm := range seat.Battlefield {
		if perm == nil || perm.Card == nil {
			continue
		}
		if perm.Controller != seat.Idx {
			continue
		}
		// Bargain accepts artifacts, enchantments, or tokens.
		eligible := perm.IsArtifact() || perm.IsEnchantment() || perm.IsToken()
		if !eligible {
			continue
		}
		cmc := perm.Card.CMC
		if best == nil || cmc < bestCMC {
			best = perm
			bestCMC = cmc
		}
	}
	return best
}

// BargainAdditionalCost creates an AdditionalCost for the bargain keyword.
// The caller's cast path can pass this to CastSpellWithCosts.
func BargainAdditionalCost() *AdditionalCost {
	return &AdditionalCost{
		Kind:  AddCostKindBargain,
		Label: "bargain (sacrifice an artifact, enchantment, or token)",
	}
}
