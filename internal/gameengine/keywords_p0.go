package gameengine

// P0 keyword implementations — the 10 highest-impact unhandled keywords
// by card count. Each keyword is implemented at engine level so that
// cards with these keywords actually DO something in simulation.
//
// Keywords implemented here:
//
//   1. Cycling (300 cards)       — CR §702.29
//   2. Crew (161 cards)          — CR §702.122
//   3. Convoke (99 cards)        — CR §702.51
//   4. Infect (45 cards)         — CR §702.90
//   5. Shroud (36 cards)         — CR §702.18
//   6. Affinity for artifacts    — CR §702.41
//   7. Exalted (34 cards)        — CR §702.83
//   8. Landwalk family (102)     — CR §702.14
//   9. Bestow (42 cards)         — CR §702.103
//  10. Devoid (131 cards)        — CR §702.114

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// 1. Cycling — CR §702.29
// ---------------------------------------------------------------------------
//
// "Pay cost, discard this card: Draw a card."
// Activated ability from HAND (not battlefield). Pay cycling cost + discard
// self → draw 1. Typecycling variant: search library for matching type
// instead of drawing.

// CyclingCost extracts the cycling cost from a card's AST keywords.
// Returns (cost, isCycling). If the keyword has args, args[0] is the
// generic mana cost as a float64 (JSON number). Returns -1 cost if no
// cycling keyword found.
func CyclingCost(card *Card) (int, bool) {
	if card == nil || card.AST == nil {
		return -1, false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(kw.Name))
		if name == "cycling" || strings.HasSuffix(name, "cycling") {
			cost := 0
			if len(kw.Args) > 0 {
				switch v := kw.Args[0].(type) {
				case float64:
					cost = int(v)
				case int:
					cost = v
				}
			}
			return cost, true
		}
	}
	return -1, false
}

// TypecyclingType returns the land/card type for typecycling keywords
// like "swampcycling", "islandcycling", "slivercycling". Returns ""
// if this is plain cycling.
func TypecyclingType(card *Card) string {
	if card == nil || card.AST == nil {
		return ""
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(kw.Name))
		if name == "cycling" {
			return ""
		}
		if strings.HasSuffix(name, "cycling") {
			return strings.TrimSuffix(name, "cycling")
		}
	}
	return ""
}

// ActivateCycling performs the cycling activation from hand for a given
// seat. CR §702.29a: "Cycling is an activated ability that functions
// only while the card with cycling is in a player's hand."
//
// Steps:
//   1. Verify card is in hand and has cycling keyword
//   2. Pay cycling cost (mana)
//   3. Discard the card (move hand → graveyard)
//   4. Draw a card (plain cycling) OR search library (typecycling)
//   5. Fire "when you cycle" triggers
//
// Returns error if activation is illegal.
func ActivateCycling(gs *GameState, seatIdx int, card *Card) error {
	if gs == nil || card == nil {
		return &CastError{Reason: "nil game or card"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	seat := gs.Seats[seatIdx]

	// Verify card is in hand.
	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		return &CastError{Reason: "card not in hand"}
	}

	// Get cycling cost.
	cost, hasCycling := CyclingCost(card)
	if !hasCycling {
		return &CastError{Reason: "card does not have cycling"}
	}

	// Pay mana cost.
	if seat.ManaPool < cost {
		return &CastError{Reason: "insufficient_mana"}
	}
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	// Discard the card (move from hand to graveyard).
	DiscardCard(gs, card, seatIdx)

	gs.LogEvent(Event{
		Kind:   "cycling",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"rule": "702.29a",
		},
	})

	// Typecycling: search library for matching type.
	typeSearch := TypecyclingType(card)
	if typeSearch != "" {
		found := searchLibraryForType(gs, seatIdx, typeSearch)
		if found != nil {
			gs.LogEvent(Event{
				Kind:   "typecycling_search",
				Seat:   seatIdx,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"type":  typeSearch,
					"found": found.DisplayName(),
					"rule":  "702.29b",
				},
			})
		}
	} else {
		// Plain cycling: draw a card.
		if _, ok := gs.drawOne(seatIdx); ok {
			gs.LogEvent(Event{
				Kind:   "draw",
				Seat:   seatIdx,
				Target: seatIdx,
				Source: card.DisplayName(),
				Amount: 1,
				Details: map[string]interface{}{
					"reason": "cycling",
				},
			})
		}
	}

	// Fire "when you cycle" triggers on permanents.
	fireCyclingTriggers(gs, seatIdx, card)

	return nil
}

// searchLibraryForType finds the first card in library matching a given
// type (for typecycling) and moves it to hand. Returns the found card
// or nil.
func searchLibraryForType(gs *GameState, seatIdx int, cardType string) *Card {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	want := strings.ToLower(cardType)
	for _, c := range seat.Library {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			if strings.ToLower(t) == want {
				MoveCard(gs, c, seatIdx, "library", "hand", "tutor-to-hand")
				return c
			}
		}
	}
	return nil
}

// fireCyclingTriggers fires "whenever you cycle a card" or "when you
// cycle this card" triggers on the active player's permanents.
//
// Per CR §702.29: triggers that say "whenever you cycle a card" match
// any cycle event. Triggers that say "when you cycle THIS card" only
// match when the specific card is cycled. The cycled card identity is
// passed in context so per-card handlers can distinguish the two cases.
func fireCyclingTriggers(gs *GameState, seatIdx int, cycledCard *Card) {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Fire the per-card trigger hook with the cycled card in context,
	// so per-card handlers can distinguish "any cycle" vs "this card cycled".
	FireCardTrigger(gs, "card_cycled", map[string]interface{}{
		"cycled_card":      cycledCard,
		"cycled_card_name": cycledCard.DisplayName(),
		"controller_seat":  seatIdx,
	})

	perms := append([]*Permanent{}, gs.Seats[seatIdx].Battlefield...)
	for _, p := range perms {
		if p == nil || p.Card == nil || p.Card.AST == nil {
			continue
		}
		for _, ab := range p.Card.AST.Abilities {
			t, ok := ab.(*gameast.Triggered)
			if !ok {
				continue
			}
			if EventEquals(t.Trigger.Event, "cycle") || t.Trigger.Event == "cycling" {
				// If the trigger has an Actor filter set to "self", it means
				// "when you cycle THIS card" — only fire when the cycled
				// card matches the permanent's card.
				if t.Trigger.Actor != nil && t.Trigger.Actor.Base == "self" {
					if cycledCard != p.Card {
						continue
					}
				}
				if t.Effect != nil {
					PushTriggeredAbility(gs, p, t.Effect)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Crew — CR §702.122 (Vehicles)
// ---------------------------------------------------------------------------
//
// "Crew N: Tap any number of creatures you control with total power N
// or greater: This Vehicle becomes an artifact creature until end of turn."

// IsVehicle returns true if the permanent is a Vehicle (artifact subtype).
func IsVehicle(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	for _, t := range p.Card.Types {
		if strings.ToLower(t) == "vehicle" {
			return true
		}
	}
	if p.Card.TypeLine != "" && strings.Contains(strings.ToLower(p.Card.TypeLine), "vehicle") {
		return true
	}
	return false
}

// CrewCost extracts the crew cost N from the card's keywords.
// Returns (N, true) if found.
func CrewCost(card *Card) (int, bool) {
	if card == nil || card.AST == nil {
		return 0, false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) == "crew" {
			n := 0
			if len(kw.Args) > 0 {
				switch v := kw.Args[0].(type) {
				case float64:
					n = int(v)
				case int:
					n = v
				}
			}
			return n, true
		}
	}
	return 0, false
}

// CrewVehicle activates the crew ability on a Vehicle permanent.
// CR §702.122a: tap creatures with total power >= N to crew.
// The vehicle becomes an artifact creature until end of turn.
func CrewVehicle(gs *GameState, seatIdx int, vehicle *Permanent, crewCreatures []*Permanent) error {
	if gs == nil || vehicle == nil {
		return &CastError{Reason: "nil game or vehicle"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if vehicle.Controller != seatIdx {
		return &CastError{Reason: "not controller"}
	}
	if !IsVehicle(vehicle) {
		return &CastError{Reason: "not a vehicle"}
	}

	crewN, hasCrew := CrewCost(vehicle.Card)
	if !hasCrew {
		return &CastError{Reason: "no crew ability"}
	}

	// Calculate total power of crew creatures.
	totalPower := 0
	for _, c := range crewCreatures {
		if c == nil || !c.IsCreature() || c.Controller != seatIdx {
			return &CastError{Reason: "invalid crew creature"}
		}
		if c.Tapped {
			return &CastError{Reason: "crew creature already tapped"}
		}
		totalPower += c.Power()
	}
	if totalPower < crewN {
		return &CastError{Reason: "insufficient power to crew"}
	}

	// Tap the crew creatures.
	for _, c := range crewCreatures {
		c.Tapped = true
	}

	// Vehicle becomes an artifact creature until end of turn.
	if !vehicle.IsCreature() {
		vehicle.GrantedAbilities = append(vehicle.GrantedAbilities, "creature_type_granted")
		if vehicle.Card != nil {
			hasCreature := false
			for _, t := range vehicle.Card.Types {
				if t == "creature" {
					hasCreature = true
					break
				}
			}
			if !hasCreature {
				vehicle.Card.Types = append(vehicle.Card.Types, "creature")
			}
		}
		if vehicle.Flags == nil {
			vehicle.Flags = map[string]int{}
		}
		vehicle.Flags["crewed_until_eot"] = 1
	}

	gs.LogEvent(Event{
		Kind:   "crew",
		Seat:   seatIdx,
		Source: vehicle.Card.DisplayName(),
		Amount: crewN,
		Details: map[string]interface{}{
			"total_power": totalPower,
			"rule":        "702.122a",
		},
	})

	return nil
}

// UncrewVehiclesAtEOT removes the creature type from vehicles whose
// crewed_until_eot flag is set. Called during cleanup step.
func UncrewVehiclesAtEOT(gs *GameState) {
	if gs == nil {
		return
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Flags == nil {
				continue
			}
			if p.Flags["crewed_until_eot"] > 0 {
				delete(p.Flags, "crewed_until_eot")
				// Remove the creature type grant.
				if p.Card != nil {
					filtered := p.Card.Types[:0]
					for _, t := range p.Card.Types {
						if t != "creature" || !IsVehicle(p) {
							filtered = append(filtered, t)
						}
					}
					// Only remove "creature" if it was the granted type.
					// Keep if the vehicle naturally has creature type.
					p.Card.Types = filtered
				}
				kept := p.GrantedAbilities[:0]
				for _, g := range p.GrantedAbilities {
					if g != "creature_type_granted" {
						kept = append(kept, g)
					}
				}
				p.GrantedAbilities = kept
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Convoke — CR §702.51
// ---------------------------------------------------------------------------
//
// "Each creature you tap while casting this spell pays for {1} or one
// mana of that creature's color."
//
// Convoke cost reduction is handled in cost_modifiers.go via
// ConvokeCostReduction. This section provides the helper to detect
// and compute the convoke discount.

// HasConvoke returns true if the card has the convoke keyword.
func HasConvoke(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) == "convoke" {
			return true
		}
	}
	return false
}

// ConvokeCostReduction counts untapped creatures on seatIdx's battlefield
// for convoke cost reduction. Each untapped creature contributes {1} or
// one mana of its color. Returns the maximum generic mana reduction.
func ConvokeCostReduction(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	count := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsCreature() || p.Tapped {
			continue
		}
		count++
	}
	return count
}

// ---------------------------------------------------------------------------
// 4. Infect — CR §702.90
// ---------------------------------------------------------------------------
//
// Damage dealt to players → poison counters instead of life loss.
// Damage dealt to creatures → -1/-1 counters instead of marked damage.
//
// Core infect logic is wired into combat.go's applyCombatDamageToPlayer
// and applyCombatDamageToCreature functions. See the modifications there.

// HasInfect returns true if the permanent has the infect keyword.
func HasInfect(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("infect")
}

// ---------------------------------------------------------------------------
// 5. Shroud — CR §702.18
// ---------------------------------------------------------------------------
//
// "This permanent can't be the target of spells or abilities."
// Unlike hexproof, shroud applies to BOTH opponents AND the controller.
//
// HasShroud, HasHexproof, and CanBeTargetedBy are defined in
// keywords_p1p2.go. Targeting check is wired into targets.go's
// pickPermanentTarget.

// ---------------------------------------------------------------------------
// 6. Affinity for artifacts — CR §702.41
// ---------------------------------------------------------------------------
//
// "This spell costs {1} less to cast for each artifact you control."
// Cost reduction is wired into cost_modifiers.go's ScanCostModifiers.

// HasAffinityForArtifacts returns true if the card has "affinity for
// artifacts" keyword.
func HasAffinityForArtifacts(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(kw.Name))
		raw := strings.TrimSpace(kw.Raw)
		if name == "affinity" && strings.Contains(raw, "artifact") {
			return true
		}
		if name == "affinity for artifacts" {
			return true
		}
	}
	return false
}

// CountArtifacts counts the number of artifacts on seatIdx's battlefield.
func CountArtifacts(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	count := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p != nil && p.IsArtifact() {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// 7. Exalted — CR §702.83
// ---------------------------------------------------------------------------
//
// "Whenever a creature you control attacks alone, that creature gets
// +1/+1 until end of turn."
//
// Each instance of exalted triggers separately. Wired into combat.go's
// DeclareAttackers (fires after attack triggers).

// CountExalted counts the number of exalted instances on permanents
// controlled by seatIdx.
func CountExalted(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	count := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil {
			continue
		}
		if p.HasKeyword("exalted") {
			count++
		}
	}
	return count
}

// ApplyExalted gives the lone attacker +1/+1 until end of turn for
// each exalted instance on permanents controlled by seatIdx.
func ApplyExalted(gs *GameState, seatIdx int, attacker *Permanent) {
	if gs == nil || attacker == nil {
		return
	}
	n := CountExalted(gs, seatIdx)
	if n <= 0 {
		return
	}
	attacker.Modifications = append(attacker.Modifications, Modification{
		Power:     n,
		Toughness: n,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache() // P/T modification changes characteristics
	gs.LogEvent(Event{
		Kind:   "exalted",
		Seat:   seatIdx,
		Source: attacker.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.83",
		},
	})
}

// ---------------------------------------------------------------------------
// 8. Landwalk family — CR §702.14
// ---------------------------------------------------------------------------
//
// Swampwalk, Islandwalk, Forestwalk, Mountainwalk, Plainswalk.
// "This creature can't be blocked as long as defending player controls
// a [land type]."
//
// Evasion check during declare-blockers is wired into combat.go's canBlock.

// LandwalkType returns the land type if this permanent has a landwalk
// keyword, or "" if none.
func LandwalkType(p *Permanent) string {
	if p == nil {
		return ""
	}
	for _, kw := range landwalkKeywords {
		if p.HasKeyword(kw.keyword) {
			return kw.landType
		}
	}
	return ""
}

type landwalkEntry struct {
	keyword  string
	landType string
}

var landwalkKeywords = []landwalkEntry{
	{"swampwalk", "swamp"},
	{"islandwalk", "island"},
	{"forestwalk", "forest"},
	{"mountainwalk", "mountain"},
	{"plainswalk", "plains"},
}

// DefenderControlsLandType returns true if the defending seat controls
// at least one land of the given basic land type.
func DefenderControlsLandType(gs *GameState, defenderSeat int, landType string) bool {
	if gs == nil || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return false
	}
	want := strings.ToLower(landType)
	for _, p := range gs.Seats[defenderSeat].Battlefield {
		if p == nil || !p.IsLand() {
			continue
		}
		if p.Card != nil {
			for _, t := range p.Card.Types {
				if strings.ToLower(t) == want {
					return true
				}
			}
			// Also check card name for basic lands.
			if strings.ToLower(p.Card.DisplayName()) == want {
				return true
			}
		}
	}
	return false
}

// HasLandwalkEvasion returns true if the attacker has landwalk and the
// defending player controls the matching land type. When true, the
// attacker can't be blocked by the defender.
func HasLandwalkEvasion(gs *GameState, attacker *Permanent, defenderSeat int) bool {
	lt := LandwalkType(attacker)
	if lt == "" {
		return false
	}
	return DefenderControlsLandType(gs, defenderSeat, lt)
}

// ---------------------------------------------------------------------------
// 9. Bestow — CR §702.103
// ---------------------------------------------------------------------------
//
// "You may cast this card targeting a creature. If you do, it enters
// as an Aura enchantment with 'enchanted creature gets +P/+T and has
// [abilities].' If the enchanted creature leaves, this becomes a creature."

// HasBestow returns true if the card has the bestow keyword.
func HasBestow(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) == "bestow" {
			return true
		}
	}
	return false
}

// BestowCost returns the bestow cost from the keyword args. If no
// explicit bestow cost, returns the card's CMC as fallback.
func BestowCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) == "bestow" {
			if len(kw.Args) > 0 {
				switch v := kw.Args[0].(type) {
				case float64:
					return int(v)
				case int:
					return v
				}
			}
		}
	}
	return card.CMC
}

// CastWithBestow casts a card with bestow, attaching it to a target
// creature as an Aura enchantment. CR §702.103a.
func CastWithBestow(gs *GameState, seatIdx int, card *Card, target *Permanent) error {
	if gs == nil || card == nil || target == nil {
		return &CastError{Reason: "nil game, card, or target"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if !HasBestow(card) {
		return &CastError{Reason: "card does not have bestow"}
	}
	if !target.IsCreature() {
		return &CastError{Reason: "bestow target must be a creature"}
	}

	seat := gs.Seats[seatIdx]
	cost := BestowCost(card)

	// Find card in hand.
	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		return &CastError{Reason: "card not in hand"}
	}

	// Pay cost.
	if seat.ManaPool < cost {
		return &CastError{Reason: "insufficient_mana"}
	}
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	// Remove from hand.
	removeCardFromZone(gs, seatIdx, card, "hand")

	// Enter as Aura attached to target.
	perm := &Permanent{
		Card:       card,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{"bestowed": 1},
		AttachedTo: target,
	}

	// Register a continuous effect (layer 4 — type-changing) that adds
	// "aura" and "enchantment" types while the bestowed flag is set.
	// This avoids mutating Card.Types directly per CR §702.103.
	gs.RegisterContinuousEffect(&ContinuousEffect{
		SourcePerm:     perm,
		Layer:          LayerType,
		HandlerID:      "bestow_" + card.DisplayName() + "_type",
		ControllerSeat: seatIdx,
		Duration:       DurationUntilConditionChanges,
		Predicate: func(gs *GameState, target *Permanent) bool {
			return target == perm && perm.Flags != nil && perm.Flags["bestowed"] == 1
		},
		ApplyFn: func(gs *GameState, target *Permanent, chars *Characteristics) {
			// Add aura/enchantment types without mutating the card
			hasAura := false
			hasEnchantment := false
			for _, t := range chars.Types {
				if t == "aura" {
					hasAura = true
				}
				if t == "enchantment" {
					hasEnchantment = true
				}
			}
			if !hasAura {
				chars.Types = append(chars.Types, "aura")
			}
			if !hasEnchantment {
				chars.Types = append(chars.Types, "enchantment")
			}
		},
	})

	// Grant +P/+T to enchanted creature.
	target.Modifications = append(target.Modifications, Modification{
		Power:     card.BasePower,
		Toughness: card.BaseToughness,
		Duration:  "permanent",
		Timestamp: perm.Timestamp,
	})
	gs.InvalidateCharacteristicsCache() // P/T modification changes characteristics

	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind:   "bestow",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"target":    target.Card.DisplayName(),
			"power":     card.BasePower,
			"toughness": card.BaseToughness,
			"rule":      "702.103a",
		},
	})

	return nil
}

// BestowFalloff handles the case where a bestowed creature's enchanted
// target leaves the battlefield. The bestow aura "falls off" and
// becomes a creature. CR §702.103e.
// Called from zone_change or SBA when an enchanted permanent leaves.
func BestowFalloff(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Flags == nil {
		return
	}
	if perm.Flags["bestowed"] == 0 {
		return
	}

	// Clear the bestowed flag — the continuous effect checks this flag,
	// so it stops applying aura/enchantment types automatically.
	// No need to mutate Card.Types directly (CR §702.103e).
	perm.AttachedTo = nil
	perm.Flags["bestowed"] = 0

	gs.LogEvent(Event{
		Kind:   "bestow_falloff",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.103e",
		},
	})
}

// CheckBestowFalloffs checks all bestowed permanents and triggers
// falloff for any whose enchanted target is no longer on the battlefield.
// Called from SBA or after zone changes.
func CheckBestowFalloffs(gs *GameState) bool {
	if gs == nil {
		return false
	}
	changed := false
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Flags == nil || p.Flags["bestowed"] == 0 {
				continue
			}
			if p.AttachedTo == nil || !permanentOnBattlefield(gs, p.AttachedTo) {
				BestowFalloff(gs, p)
				changed = true
			}
		}
	}
	return changed
}

// permanentOnBattlefield is defined in sba.go — reused here for bestow
// falloff checks.

// ---------------------------------------------------------------------------
// 10. Devoid — CR §702.114
// ---------------------------------------------------------------------------
//
// "This card has no color." Simple color-override. Sets card's colors to
// empty/colorless regardless of mana cost.
//
// CardHasDevoid, HasDevoid, GetDevoidColors, and the continuous-effect
// registration are all implemented in keywords_p1p2.go.

// ApplyDevoid zeroes out a card's colors if it has devoid. Should be
// called at card load time or before color checks. CR §702.114a.
func ApplyDevoid(card *Card) {
	if CardHasDevoid(card) {
		card.Colors = nil
	}
}

// ---------------------------------------------------------------------------
// 11. Toxic — CR §702.165 (46 cards)
// ---------------------------------------------------------------------------
//
// "Toxic N — Whenever this creature deals combat damage to a player,
// that player gets N poison counters." This is IN ADDITION to normal
// damage (unlike infect which REPLACES damage with poison).

// HasToxic returns true and the toxic N value if the permanent has the
// toxic keyword. Checks AST keywords (Keyword{Name: "toxic", Args: [N]}),
// runtime flags ("kw:toxic"), and GrantedAbilities.
func HasToxic(p *Permanent) (bool, int) {
	if p == nil {
		return false, 0
	}
	// 1) AST-declared keywords.
	if p.Card != nil && p.Card.AST != nil {
		for _, ab := range p.Card.AST.Abilities {
			kw, ok := ab.(*gameast.Keyword)
			if !ok {
				continue
			}
			if strings.ToLower(strings.TrimSpace(kw.Name)) == "toxic" {
				n := 1 // default toxic 1
				if len(kw.Args) > 0 {
					switch v := kw.Args[0].(type) {
					case float64:
						n = int(v)
					case int:
						n = v
					}
				}
				return true, n
			}
		}
	}
	// 2) Runtime flag — "kw:toxic" with value = N.
	if p.Flags != nil {
		if n, ok := p.Flags["kw:toxic"]; ok && n > 0 {
			return true, n
		}
	}
	return false, 0
}
