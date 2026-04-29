package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ============================================================================
// keywords_batch3.go — Alternative casting cost keywords (Batch 3)
//
// Implements: Emerge, Replicate, Prototype, Casualty, Squad.
// Also provides requested API wrappers for keywords already implemented
// under different function names in other files.
// ============================================================================

// ---------------------------------------------------------------------------
// Emerge — CR §702.119
// "You may cast this spell by sacrificing a creature and paying the emerge
// cost reduced by that creature's mana value."
// ---------------------------------------------------------------------------

// HasEmerge returns true if the card has the emerge keyword.
func HasEmerge(card *Card) bool {
	return cardHasKeywordByName(card, "emerge")
}

// EmergeCost returns the emerge cost from keyword args.
func EmergeCost(card *Card) int {
	return keywordArgCost(card, "emerge")
}

// CalculateEmergeCost computes the final mana cost when casting with emerge.
// The emerge cost is reduced by the sacrificed creature's CMC.
// Per CR §702.119b, the total cost is max(emergeCost - sacrificedCMC, 0).
func CalculateEmergeCost(gs *GameState, seatIdx int, card *Card, sacrificed *Permanent) int {
	if card == nil {
		return 0
	}
	emerge := EmergeCost(card)
	if sacrificed == nil || sacrificed.Card == nil {
		return emerge
	}
	reduction := sacrificed.Card.CMC
	result := emerge - reduction
	if result < 0 {
		result = 0
	}
	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "emerge_cost",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Amount: result,
			Details: map[string]interface{}{
				"emerge_base":  emerge,
				"sacrificed":   sacrificed.Card.DisplayName(),
				"sac_cmc":      reduction,
				"rule":         "702.119",
			},
		})
	}
	return result
}

// CanPayEmerge checks if the player has a creature on the battlefield to
// sacrifice for emerge and enough mana after the reduction.
func CanPayEmerge(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	emerge := EmergeCost(card)
	seat := gs.Seats[seatIdx]
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		cost := emerge - p.Card.CMC
		if cost < 0 {
			cost = 0
		}
		if seat.ManaPool >= cost {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Offering — CR §702.48  (wrapper for existing OfferingReduction)
// "You may cast this as though it had flash by sacrificing a [type].
// The total cost is reduced by the sacrificed permanent's mana cost."
// ---------------------------------------------------------------------------

// CalculateOfferingCost computes the spell's mana cost after offering
// reduction from the sacrificed permanent.
func CalculateOfferingCost(card *Card, sacrificed *Permanent) int {
	if card == nil {
		return 0
	}
	base := card.CMC
	if sacrificed == nil || sacrificed.Card == nil {
		return base
	}
	result := base - sacrificed.Card.CMC
	if result < 0 {
		result = 0
	}
	return result
}

// ---------------------------------------------------------------------------
// Splice — CR §702.47  (wrappers for existing ApplySplice/HasSplice)
// "As you cast an Arcane spell, you may reveal this card from your hand
// and pay its splice cost. If you do, add this card's text to that spell."
// ---------------------------------------------------------------------------

// CanSplice returns true if the card has the splice keyword.
// Wrapper for HasSplice — the splice-onto-arcane check is done in ApplySplice.
func CanSplice(card *Card) bool {
	return HasSplice(card)
}

// ---------------------------------------------------------------------------
// Replicate — CR §702.56
// "When you cast this spell, copy it for each time you paid its replicate
// cost. You may choose new targets for the copies."
// ---------------------------------------------------------------------------

// HasReplicate returns true if the card has the replicate keyword.
func HasReplicate(card *Card) bool {
	return cardHasKeywordByName(card, "replicate")
}

// ReplicateCost returns the replicate cost from keyword args.
func ReplicateCost(card *Card) int {
	return keywordArgCost(card, "replicate")
}

// ApplyReplicate pays the replicate cost `copies` times and creates that
// many copies of the spell on the stack. Per CR §702.56a, each copy is
// put on the stack as a copy (not cast), similar to storm (§706.10).
func ApplyReplicate(gs *GameState, item *StackItem, copies int) {
	if gs == nil || item == nil || item.Card == nil || copies <= 0 {
		return
	}
	cost := ReplicateCost(item.Card)
	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	// Pay replicate cost N times.
	totalCost := cost * copies
	if seat.ManaPool < totalCost {
		return
	}
	seat.ManaPool -= totalCost
	SyncManaAfterSpend(seat)

	gs.LogEvent(Event{
		Kind:   "replicate_pay",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Amount: copies,
		Details: map[string]interface{}{
			"cost_each":  cost,
			"total_cost": totalCost,
			"rule":       "702.56",
		},
	})

	// Create N copies on the stack (CR §706.10 — copies, not cast).
	baseName := item.Card.Name
	for i := 0; i < copies; i++ {
		copyCard := &Card{
			Name:          baseName + " (replicate " + itoaBatch(i+1) + ")",
			Owner:         item.Card.Owner,
			BasePower:     item.Card.BasePower,
			BaseToughness: item.Card.BaseToughness,
			Types:         append([]string(nil), item.Card.Types...),
			Colors:        append([]string(nil), item.Card.Colors...),
			CMC:           0, // copies cost nothing
		}
		if item.Card.AST != nil {
			copyCard.AST = item.Card.AST
		}
		copyItem := &StackItem{
			Controller: seatIdx,
			Card:       copyCard,
			Effect:     item.Effect,
			Targets:    append([]Target(nil), item.Targets...),
			IsCopy:     true, // CR §706.10
		}
		copyItem.ID = nextStackID(gs)
		gs.Stack = append(gs.Stack, copyItem)

		gs.LogEvent(Event{
			Kind:   "replicate_copy",
			Seat:   seatIdx,
			Source: copyCard.Name,
			Details: map[string]interface{}{
				"stack_id":   copyItem.ID,
				"stack_size": len(gs.Stack),
				"copy_index": i + 1,
				"rule":       "702.56+706.10",
			},
		})
	}
}

// ---------------------------------------------------------------------------
// Retrace — CR §702.81  (wrappers for existing CanCastRetrace)
// ---------------------------------------------------------------------------

// CanRetrace checks if the card has retrace and the player has a land to
// discard. Wrapper combining HasRetrace + CanCastRetrace.
func CanRetrace(gs *GameState, seatIdx int, card *Card) bool {
	if card == nil || !HasRetrace(card) {
		return false
	}
	return CanCastRetrace(gs, seatIdx)
}

// PayRetraceCost pays the retrace additional cost by discarding a land
// from hand. Returns true on success.
func PayRetraceCost(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	for _, c := range seat.Hand {
		if c != nil && cardHasType(c, "land") {
			DiscardCard(gs, c, seatIdx)
			gs.LogEvent(Event{
				Kind:   "retrace_discard",
				Seat:   seatIdx,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"discarded": c.DisplayName(),
					"rule":      "702.81",
				},
			})
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Jump-Start — CR §702.133  (wrappers for existing HasJumpStart)
// ---------------------------------------------------------------------------

// CanJumpStart checks if the card has jump-start and the player has a card
// to discard.
func CanJumpStart(gs *GameState, seatIdx int, card *Card) bool {
	if card == nil || !HasJumpStart(card) {
		return false
	}
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	return len(gs.Seats[seatIdx].Hand) > 0
}

// PayJumpStartCost pays the jump-start additional cost by discarding a
// card from hand. Returns true on success.
func PayJumpStartCost(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if len(seat.Hand) == 0 {
		return false
	}
	// Discard the last card (greedy: pick least valuable).
	discarded := seat.Hand[len(seat.Hand)-1]
	DiscardCard(gs, discarded, seatIdx)

	gs.LogEvent(Event{
		Kind:   "jump_start_discard",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"discarded": discarded.DisplayName(),
			"rule":      "702.133",
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// Entwine — CR §702.42  (wrappers for existing CanPayEntwine)
// ---------------------------------------------------------------------------

// IsEntwined checks if a stack item was cast with entwine paid.
func IsEntwined(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	if v, ok := item.CostMeta["entwined"]; ok {
		if b, ok2 := v.(bool); ok2 {
			return b
		}
	}
	return false
}

// ApplyEntwine marks a stack item as entwined, so both modes will be used
// on resolution.
func ApplyEntwine(item *StackItem) {
	if item == nil {
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["entwined"] = true
}

// ---------------------------------------------------------------------------
// Surge — CR §702.117  (wrapper for existing CanPaySurge)
// ---------------------------------------------------------------------------

// CanCastForSurge checks if the surge condition is met: the player (or a
// teammate) has cast another spell this turn.
func CanCastForSurge(gs *GameState, seatIdx int) bool {
	return CanPaySurge(gs, seatIdx)
}

// ---------------------------------------------------------------------------
// Undaunted — CR §702.125  (already implemented; no new wrapper needed)
// UndauntedReduction is in keywords_combat.go.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Assist — CR §702.132  (already implemented; no new wrapper needed)
// AssistReduction is in keywords_combat.go.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Miracle — CR §702.94  (wrapper for existing CanCastMiracle)
// ---------------------------------------------------------------------------

// IsMiracle checks if the given card can currently be cast for its miracle
// cost (it must be the first card drawn this turn).
func IsMiracle(gs *GameState, seatIdx int, card *Card) bool {
	return CanCastMiracle(gs, seatIdx, card)
}

// ---------------------------------------------------------------------------
// Prototype — CR §702.160
// "You may cast this spell with different mana cost, color, power, and
// toughness. It keeps its abilities and other characteristics."
// ---------------------------------------------------------------------------

// HasPrototype returns true if the card has the prototype keyword.
func HasPrototype(card *Card) bool {
	return cardHasKeywordByName(card, "prototype")
}

// ApplyPrototype modifies a permanent to use its prototype characteristics.
// Per CR §702.160, the prototype replaces P/T and colors but keeps all
// abilities. We read prototype args from the keyword:
//   - Args[0] = prototype mana cost (int)
//   - Args[1] = prototype power (int)
//   - Args[2] = prototype toughness (int)
// Colors become colorless unless the keyword specifies otherwise.
func ApplyPrototype(perm *Permanent) {
	if perm == nil || perm.Card == nil {
		return
	}
	if !HasPrototype(perm.Card) {
		return
	}

	// Extract prototype args from the AST keyword.
	var protoPower, protoTough, protoCMC int
	if perm.Card.AST != nil {
		for _, ab := range perm.Card.AST.Abilities {
			kw, ok := ab.(*gameast.Keyword)
			if !ok {
				continue
			}
			if strings.ToLower(strings.TrimSpace(kw.Name)) != "prototype" {
				continue
			}
			// Parse args: [cost, power, toughness]
			if len(kw.Args) >= 1 {
				switch v := kw.Args[0].(type) {
				case float64:
					protoCMC = int(v)
				case int:
					protoCMC = v
				}
			}
			if len(kw.Args) >= 2 {
				switch v := kw.Args[1].(type) {
				case float64:
					protoPower = int(v)
				case int:
					protoPower = v
				}
			}
			if len(kw.Args) >= 3 {
				switch v := kw.Args[2].(type) {
				case float64:
					protoTough = int(v)
				case int:
					protoTough = v
				}
			}
			break
		}
	}

	// Apply prototype characteristics.
	perm.Card.BasePower = protoPower
	perm.Card.BaseToughness = protoTough
	perm.Card.CMC = protoCMC
	// Prototype makes the permanent its prototype colors (simplified:
	// if no color args, becomes colorless).
	perm.Card.Colors = nil

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["prototype"] = 1
}

// ---------------------------------------------------------------------------
// Bargain — CR §702.166
// Already implemented in costs.go as BargainAdditionalCost().
// No additional functions needed.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Casualty — CR §702.153
// "As an additional cost to cast this spell, you may sacrifice a creature
// with power N or greater. When you do, copy this spell."
// ---------------------------------------------------------------------------

// HasCasualty returns true if the card has the casualty keyword.
func HasCasualty(card *Card) bool {
	return cardHasKeywordByName(card, "casualty")
}

// CasualtyMinPower returns the minimum power required for the sacrificed
// creature from the keyword args.
func CasualtyMinPower(card *Card) int {
	return keywordArgCost(card, "casualty")
}

// PayCasualty sacrifices a creature with power >= minPower as an additional
// cost for casualty. Returns true if a creature was sacrificed.
func PayCasualty(gs *GameState, seatIdx int, minPower int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	// Find the cheapest creature (lowest CMC) with sufficient power.
	var best *Permanent
	bestCMC := 999
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Power() < minPower {
			continue
		}
		if p.Card.CMC < bestCMC {
			best = p
			bestCMC = p.Card.CMC
		}
	}
	if best == nil {
		return false
	}

	name := best.Card.DisplayName()
	SacrificePermanent(gs, best, "casualty")

	gs.LogEvent(Event{
		Kind:   "casualty",
		Seat:   seatIdx,
		Source: name,
		Amount: minPower,
		Details: map[string]interface{}{
			"min_power":  minPower,
			"sac_power":  best.Power(),
			"rule":       "702.153",
		},
	})
	return true
}

// CanPayCasualty checks if the player has a creature with sufficient power
// to sacrifice for casualty.
func CanPayCasualty(gs *GameState, seatIdx int, minPower int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Power() >= minPower {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Squad — CR §702.157
// "As an additional cost to cast this spell, you may pay {cost} any number
// of times. When this creature enters the battlefield, create a token
// that's a copy of it for each time the squad cost was paid."
// ---------------------------------------------------------------------------

// HasSquad returns true if the card has the squad keyword.
func HasSquad(card *Card) bool {
	return cardHasKeywordByName(card, "squad")
}

// SquadCost returns the squad cost from keyword args.
func SquadCost(card *Card) int {
	return keywordArgCost(card, "squad")
}

// ApplySquad creates `copies` token copies of the permanent on the
// battlefield. Each token is a copy of the original with the same P/T,
// types, and abilities (CR §702.157b).
func ApplySquad(gs *GameState, perm *Permanent, copies int) {
	if gs == nil || perm == nil || perm.Card == nil || copies <= 0 {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]

	for i := 0; i < copies; i++ {
		tokenCard := &Card{
			Name:          perm.Card.Name + " (squad token)",
			Owner:         seatIdx,
			BasePower:     perm.Card.BasePower,
			BaseToughness: perm.Card.BaseToughness,
			Types:         append([]string(nil), perm.Card.Types...),
			Colors:        append([]string(nil), perm.Card.Colors...),
			CMC:           perm.Card.CMC,
		}
		if perm.Card.AST != nil {
			tokenCard.AST = perm.Card.AST
		}
		// Ensure "token" type is present.
		hasToken := false
		for _, t := range tokenCard.Types {
			if t == "token" {
				hasToken = true
				break
			}
		}
		if !hasToken {
			tokenCard.Types = append([]string{"token"}, tokenCard.Types...)
		}

		tokenPerm := &Permanent{
			Card:       tokenCard,
			Controller: seatIdx,
			Owner:      seatIdx,
			Timestamp:  gs.NextTimestamp(),
			Counters:   map[string]int{},
			Flags:      map[string]int{},
		}
		seat.Battlefield = append(seat.Battlefield, tokenPerm)
		RegisterReplacementsForPermanent(gs, tokenPerm)
		FirePermanentETBTriggers(gs, tokenPerm)
	}
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "squad",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: copies,
		Details: map[string]interface{}{
			"tokens_created": copies,
			"rule":           "702.157",
		},
	})
}
