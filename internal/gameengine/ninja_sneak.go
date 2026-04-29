package gameengine

// Shared "unblocked-attacker bounce" primitive for Ninjutsu (CR 702.49) and
// Sneak (TMNT/Final Fantasy cross-set keyword, alternative casting cost).
//
// Both mechanics share the same prerequisite: an unblocked attacker
// controlled by the activating player during the declare-blockers step.
// The attacker is returned to its owner's hand as part of the cost.
//
// Key differences:
//
//   Ninjutsu (activated ability, NOT a cast):
//     - Does NOT go on the stack as a spell
//     - Does NOT increment commander tax, storm count, or fire "cast" triggers
//     - The ninja enters tapped and attacking (CR 702.49a)
//     - The ninja was NOT declared as an attacker (CR 702.49b) — no "attacks" triggers
//     - ETB triggers fire normally
//     - Commander ninjutsu: same, but may also activate from command zone
//
//   Sneak (alternative casting cost, IS a cast):
//     - Goes on the stack, resolves normally via CastSpellWithCosts
//     - Increments commander tax, storm count, fires "cast" triggers
//     - Creature enters tapped and attacking (per oracle text reminder)
//     - Bouncing the attacker is part of the cost
//     - ETB triggers fire normally
//
// Both pathways share FindUnblockedAttacker and BounceUnblockedAttacker.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// FindUnblockedAttacker scans the combat attacker list for an unblocked
// attacker controlled by the given seat. Prefers the smallest-power
// creature (least value to lose) when multiple are available.
// Returns the permanent or nil.
func FindUnblockedAttacker(gs *GameState, seatIdx int, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) *Permanent {
	if gs == nil || len(attackers) == 0 {
		return nil
	}
	var best *Permanent
	for _, atk := range attackers {
		if atk == nil || atk.Controller != seatIdx {
			continue
		}
		if !permFlag(atk, flagAttacking) {
			continue
		}
		blockers := blockerMap[atk]
		if len(blockers) != 0 {
			continue
		}
		if best == nil || atk.Power() < best.Power() {
			best = atk
		}
	}
	return best
}

// FindAllUnblockedAttackers returns every unblocked attacker controlled by
// seatIdx, sorted by ascending power (cheapest to bounce first).
func FindAllUnblockedAttackers(gs *GameState, seatIdx int, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) []*Permanent {
	if gs == nil || len(attackers) == 0 {
		return nil
	}
	var out []*Permanent
	for _, atk := range attackers {
		if atk == nil || atk.Controller != seatIdx {
			continue
		}
		if !permFlag(atk, flagAttacking) {
			continue
		}
		if blockers := blockerMap[atk]; len(blockers) != 0 {
			continue
		}
		out = append(out, atk)
	}
	// Sort ascending by power.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Power() < out[j-1].Power(); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// BounceUnblockedAttacker returns an attacking creature to its owner's hand.
// Clears combat flags and delegates to BouncePermanent for proper zone-change
// handling (replacement effects, LTB triggers, commander redirect).
// Returns the defender seat the attacker was attacking, or -1.
func BounceUnblockedAttacker(gs *GameState, attacker *Permanent) int {
	if gs == nil || attacker == nil {
		return -1
	}
	defSeat, _ := AttackerDefender(attacker)

	// Clear combat flags before bouncing.
	setPermFlag(attacker, flagAttacking, false)
	setPermFlag(attacker, flagDeclaredAttacker, false)
	setPermFlag(attacker, flagAttackedThisCombat, false)
	setAttackerDefender(attacker, -1)

	BouncePermanent(gs, attacker, nil, "hand")
	return defSeat
}

// removeAttackerFromTracking removes the bounced attacker from the attackers
// slice and blockerMap, returning the updated attackers slice.
func removeAttackerFromTracking(attackers []*Permanent, bounced *Permanent, blockerMap map[*Permanent][]*Permanent) []*Permanent {
	attackers = removePermanentFromSlice(attackers, bounced)
	delete(blockerMap, bounced)
	return attackers
}

// ---------------------------------------------------------------------------
// Ninjutsu — CR 702.49 (activated ability, NOT a cast)
// ---------------------------------------------------------------------------

// CheckNinjutsuRefactored scans the attacking player's hand (and command zone
// for commander ninjutsu) for creatures with ninjutsu. For each, if there's
// an unblocked attacker and sufficient mana, the ninjutsu activation happens:
//  1. Return an unblocked attacker to hand (cost)
//  2. Put the ninja onto the battlefield tapped and attacking
//  3. "Whenever ~ attacks" triggers do NOT fire (CR 702.49b)
//  4. ETB triggers fire normally
//
// NOT a cast: no commander tax, no storm, no cast triggers.
//
// Returns the updated attackers slice.
func CheckNinjutsuRefactored(gs *GameState, attackerSeat int, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) []*Permanent {
	if gs == nil || attackerSeat < 0 || attackerSeat >= len(gs.Seats) {
		return attackers
	}
	seat := gs.Seats[attackerSeat]
	if seat == nil {
		return attackers
	}

	// Build the unblocked list.
	unblocked := FindAllUnblockedAttackers(gs, attackerSeat, attackers, blockerMap)
	if len(unblocked) == 0 {
		return attackers
	}

	// Scan hand for ninjutsu cards. Process at most one per combat (MVP).
	attackers = tryNinjutsuFromZone(gs, attackerSeat, seat.Hand, "hand", attackers, blockerMap, &unblocked)

	// Commander ninjutsu: also scan command zone.
	if len(unblocked) > 0 && len(seat.CommandZone) > 0 {
		attackers = tryNinjutsuFromZone(gs, attackerSeat, seat.CommandZone, "command_zone", attackers, blockerMap, &unblocked)
	}

	return attackers
}

// tryNinjutsuFromZone scans a zone (hand or command_zone) for ninjutsu cards
// and activates at most one.
func tryNinjutsuFromZone(gs *GameState, seatIdx int, zone []*Card, zoneName string, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent, unblocked *[]*Permanent) []*Permanent {
	if len(*unblocked) == 0 {
		return attackers
	}
	seat := gs.Seats[seatIdx]

	for i := 0; i < len(zone); i++ {
		c := zone[i]
		if c == nil {
			continue
		}

		isCommander := zoneName == "command_zone"
		if !cardHasNinjutsuOrCommanderNinjutsu(c, isCommander) {
			continue
		}

		// Determine ninjutsu cost.
		ninjutsuCost := ninjutsuCostFor(c)
		if seat.ManaPool < ninjutsuCost {
			continue
		}
		if len(*unblocked) == 0 {
			break
		}

		// Pick the smallest-power unblocked attacker to bounce.
		bounceAtk := (*unblocked)[0]

		// Pay mana cost.
		seat.ManaPool -= ninjutsuCost
		SyncManaAfterSpend(seat)
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Source: c.DisplayName(),
			Amount: ninjutsuCost,
			Details: map[string]interface{}{
				"reason": "ninjutsu",
				"zone":   zoneName,
				"rule":   "702.49",
			},
		})

		// Get defender info from the attacker being bounced.
		defSeat := BounceUnblockedAttacker(gs, bounceAtk)

		// Remove bounced attacker from tracking.
		attackers = removeAttackerFromTracking(attackers, bounceAtk, blockerMap)
		*unblocked = removePermanentFromSlice(*unblocked, bounceAtk)

		// Remove ninjutsu card from its zone.
		if isCommander {
			seat.CommandZone = removeCardFromSlice(seat.CommandZone, c)
			// Commander ninjutsu is NOT a cast — no commander tax increment.
		} else {
			seat.Hand = removeCardFromSlice(seat.Hand, c)
		}

		// Put ninja onto battlefield tapped and attacking.
		perm := &Permanent{
			Card:          c,
			Controller:    seatIdx,
			Owner:         c.Owner,
			Tapped:        true,
			SummoningSick: false, // enters attacking, irrelevant
			Timestamp:     gs.NextTimestamp(),
			Counters:      map[string]int{},
			Flags:         map[string]int{},
		}
		setPermFlag(perm, flagAttacking, true)
		// Do NOT set flagDeclaredAttacker — CR 702.49b.
		if defSeat >= 0 {
			setAttackerDefender(perm, defSeat)
		}
		// Mark ninjutsu entry for Yuriko's trigger detection.
		perm.Flags["ninjutsu_entry"] = 1

		seat.Battlefield = append(seat.Battlefield, perm)
		RegisterReplacementsForPermanent(gs, perm)
		FirePermanentETBTriggers(gs, perm)
		attackers = append(attackers, perm)
		blockerMap[perm] = nil // ninja is unblocked

		eventKind := "ninjutsu"
		if isCommander {
			eventKind = "commander_ninjutsu"
		}
		gs.LogEvent(Event{
			Kind:   eventKind,
			Seat:   seatIdx,
			Source: c.DisplayName(),
			Details: map[string]interface{}{
				"ninja":         c.DisplayName(),
				"ninjutsu_cost": ninjutsuCost,
				"defender_seat": defSeat,
				"from_zone":     zoneName,
				"rule":          "702.49",
			},
		})

		// Fire ETB triggers for the ninja.
		FireZoneChangeTriggers(gs, perm, c, zoneName, "battlefield")

		// MVP: one ninjutsu activation per combat.
		break
	}

	return attackers
}

// cardHasNinjutsuOrCommanderNinjutsu checks if a card has the ninjutsu (or
// commander_ninjutsu) keyword. When fromCommandZone is true, also matches
// commander_ninjutsu.
func cardHasNinjutsuOrCommanderNinjutsu(c *Card, fromCommandZone bool) bool {
	if c == nil {
		return false
	}
	if c.AST != nil {
		for _, ab := range c.AST.Abilities {
			if kw, ok := ab.(*gameast.Keyword); ok {
				name := strings.ToLower(strings.TrimSpace(kw.Name))
				if name == "ninjutsu" {
					return true
				}
				if fromCommandZone && name == "commander_ninjutsu" {
					return true
				}
			}
		}
	}
	// Test convention: check Types.
	for _, t := range c.Types {
		tl := strings.ToLower(t)
		if tl == "ninjutsu" {
			return true
		}
		if fromCommandZone && tl == "commander_ninjutsu" {
			return true
		}
	}
	return false
}

// ninjutsuCostFor determines the ninjutsu activation cost for a card.
// Checks for a ninjutsu keyword with cost arg first, then falls back to
// CMC - 1 (minimum 1).
func ninjutsuCostFor(c *Card) int {
	if c == nil {
		return 1
	}
	// Check AST keyword args for explicit cost.
	if c.AST != nil {
		for _, ab := range c.AST.Abilities {
			if kw, ok := ab.(*gameast.Keyword); ok {
				name := strings.ToLower(strings.TrimSpace(kw.Name))
				if name == "ninjutsu" || name == "commander_ninjutsu" {
					if len(kw.Args) > 0 {
						if cost, ok := kw.Args[0].(float64); ok && cost > 0 {
							return int(cost)
						}
						if cost, ok := kw.Args[0].(int); ok && cost > 0 {
							return cost
						}
					}
				}
			}
		}
	}
	// Fallback: CMC - 1, minimum 1.
	cost := manaCostOf(c)
	if cost > 1 {
		cost--
	}
	if cost < 1 {
		cost = 1
	}
	return cost
}

// ---------------------------------------------------------------------------
// Sneak — alternative casting cost (IS a cast)
// ---------------------------------------------------------------------------

// CheckSneak scans the attacking player's hand for cards with the sneak
// keyword. For each, if there's an unblocked attacker and sufficient mana,
// the sneak activation happens:
//  1. Return an unblocked attacker to hand (part of the cost)
//  2. Cast the creature spell for the sneak cost via CastSpellWithCosts
//  3. IS a cast: increments commander tax, storm count, fires cast triggers
//  4. The creature enters tapped and attacking (per oracle text)
//  5. ETB triggers fire via normal resolution
//
// Returns the updated attackers slice.
func CheckSneak(gs *GameState, attackerSeat int, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) []*Permanent {
	if gs == nil || attackerSeat < 0 || attackerSeat >= len(gs.Seats) {
		return attackers
	}
	seat := gs.Seats[attackerSeat]
	if seat == nil || len(seat.Hand) == 0 {
		return attackers
	}

	// Find unblocked attackers.
	unblocked := FindAllUnblockedAttackers(gs, attackerSeat, attackers, blockerMap)
	if len(unblocked) == 0 {
		return attackers
	}

	// Scan hand for sneak cards. Process at most one per combat (MVP).
	for i := 0; i < len(seat.Hand); i++ {
		c := seat.Hand[i]
		if c == nil {
			continue
		}
		if !cardHasSneak(c) {
			continue
		}

		// Determine sneak cost.
		sneakCost := sneakCostFor(c)
		if seat.ManaPool < sneakCost {
			continue
		}
		if len(unblocked) == 0 {
			break
		}

		// Pick smallest-power unblocked attacker to bounce.
		bounceAtk := unblocked[0]

		// Bounce the attacker as part of the cost (BEFORE casting).
		defSeat := BounceUnblockedAttacker(gs, bounceAtk)

		// Remove bounced attacker from tracking.
		attackers = removeAttackerFromTracking(attackers, bounceAtk, blockerMap)
		unblocked = removePermanentFromSlice(unblocked, bounceAtk)

		// Set a game-state flag so that resolution knows to place this
		// creature tapped-and-attacking.
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["sneak_pending_seat"] = attackerSeat + 1      // +1 offset to distinguish from 0
		gs.Flags["sneak_pending_defender"] = defSeat + 1000     // encode defender seat

		// Cast the creature spell via the full cast pipeline using the
		// sneak cost as an alternative cost.
		altCost := &AlternativeCost{
			Kind:  AltCostKindSneak,
			Label: "sneak " + c.DisplayName(),
			PayFn: func(gs *GameState, seatIdx int) bool {
				// The attacker bounce was already paid above.
				// Pay the sneak mana cost.
				if gs.Seats[seatIdx].ManaPool < sneakCost {
					return false
				}
				gs.Seats[seatIdx].ManaPool -= sneakCost
				SyncManaAfterSpend(gs.Seats[seatIdx])
				gs.LogEvent(Event{
					Kind:   "pay_mana",
					Seat:   seatIdx,
					Amount: sneakCost,
					Source: c.DisplayName(),
					Details: map[string]interface{}{
						"reason": "sneak",
						"rule":   "sneak_alt_cost",
					},
				})
				return true
			},
			CanPayFn: func(gs *GameState, seatIdx int) bool {
				return gs.Seats[seatIdx].ManaPool >= sneakCost
			},
		}

		_, err := CastSpellWithCosts(gs, attackerSeat, c, nil, altCost, nil, true)
		if err != nil {
			// Cast failed — put card back in hand (CastSpellWithCosts handles
			// rollback internally), restore the unblocked list state.
			gs.LogEvent(Event{
				Kind:   "sneak_failed",
				Seat:   attackerSeat,
				Source: c.DisplayName(),
				Details: map[string]interface{}{
					"reason": err.Error(),
				},
			})
			// Clear sneak flags.
			delete(gs.Flags, "sneak_pending_seat")
			delete(gs.Flags, "sneak_pending_defender")
			continue
		}

		// After successful cast + resolution, find the creature on the
		// battlefield and ensure it's tapped and attacking.
		sneakPerm := findPermanentByCard(gs, attackerSeat, c)
		if sneakPerm != nil {
			sneakPerm.Tapped = true
			setPermFlag(sneakPerm, flagAttacking, true)
			// Like ninjutsu, do NOT set flagDeclaredAttacker.
			if defSeat >= 0 {
				setAttackerDefender(sneakPerm, defSeat)
			}
			sneakPerm.Flags["sneak_entry"] = 1
			attackers = append(attackers, sneakPerm)
			blockerMap[sneakPerm] = nil // unblocked
		}

		// Clear sneak flags.
		delete(gs.Flags, "sneak_pending_seat")
		delete(gs.Flags, "sneak_pending_defender")

		gs.LogEvent(Event{
			Kind:   "sneak",
			Seat:   attackerSeat,
			Source: c.DisplayName(),
			Details: map[string]interface{}{
				"creature":     c.DisplayName(),
				"sneak_cost":   sneakCost,
				"defender_seat": defSeat,
				"rule":          "sneak_keyword",
			},
		})

		// MVP: one sneak activation per combat.
		break
	}

	return attackers
}

// cardHasSneak checks if a card has the sneak keyword.
func cardHasSneak(c *Card) bool {
	if c == nil {
		return false
	}
	if c.AST != nil {
		for _, ab := range c.AST.Abilities {
			if kw, ok := ab.(*gameast.Keyword); ok {
				if strings.EqualFold(strings.TrimSpace(kw.Name), "sneak") {
					return true
				}
			}
		}
	}
	// Test convention.
	for _, t := range c.Types {
		if strings.EqualFold(t, "sneak") {
			return true
		}
	}
	return false
}

// sneakCostFor determines the sneak alternative casting cost.
// Checks AST keyword args for explicit cost, falls back to CMC - 1.
func sneakCostFor(c *Card) int {
	if c == nil {
		return 1
	}
	if c.AST != nil {
		for _, ab := range c.AST.Abilities {
			if kw, ok := ab.(*gameast.Keyword); ok {
				if strings.EqualFold(strings.TrimSpace(kw.Name), "sneak") {
					if len(kw.Args) > 0 {
						if cost, ok := kw.Args[0].(float64); ok && cost > 0 {
							return int(cost)
						}
						if cost, ok := kw.Args[0].(int); ok && cost > 0 {
							return cost
						}
					}
				}
			}
		}
	}
	// Fallback: CMC - 1, minimum 1.
	cost := manaCostOf(c)
	if cost > 1 {
		cost--
	}
	if cost < 1 {
		cost = 1
	}
	return cost
}

// findPermanentByCard locates a permanent on a seat's battlefield by card
// pointer identity.
func findPermanentByCard(gs *GameState, seatIdx int, card *Card) *Permanent {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p != nil && p.Card == card {
			return p
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// removeCardFromSlice removes a card by pointer identity from a slice.
func removeCardFromSlice(slice []*Card, c *Card) []*Card {
	for i, x := range slice {
		if x == c {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// AltCostKindSneak is the alternative cost kind for the sneak keyword.
const AltCostKindSneak = "sneak"
