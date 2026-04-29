package gameengine

// P1+P2 keyword implementations — medium-to-low impact keywords.
//
// P1 — Medium impact:
//   1. Unearth (54 cards)  — CR §702.84
//   2. Foretell (49 cards) — CR §702.143
//   3. Entwine (31 cards)  — CR §702.42
//   4. Buyback (29 cards)  — CR §702.27
//   5. Wither  (27 cards)  — CR §702.80
//   6. Disturb (32 cards)  — CR §702.146
//
// P2 — Niche/historic:
//   7.  Bushido      (35 cards) — CR §702.45
//   8.  Flanking     (27 cards) — CR §702.25
//   9.  Horsemanship (28 cards) — CR §702.30
//   10. Devoid       (131 cards) — CR §702.114
//   11. Shroud       (verify alongside hexproof in targeting)
//
// This file is intentionally SEPARATE from keywords_p0.go to avoid
// merge conflicts with the P0 agent.

import (
	"strings"
)

// ---------------------------------------------------------------------------
// 1. Unearth — CR §702.84
//
// Activated ability from graveyard. Returns creature to battlefield,
// gains haste, exile at EOT or if it would leave the battlefield.
// ---------------------------------------------------------------------------

// NewUnearthPermission creates a ZoneCastPermission for unearth.
// Unearth is an activated ability from the graveyard, not a cast — but
// the engine models it as a zone-cast for simplicity (the creature moves
// from graveyard to battlefield). The unearthCost is the mana to pay.
func NewUnearthPermission(unearthCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "unearth",
		ManaCost:          unearthCost,
		ExileOnResolve:    false, // Unearth puts on battlefield, not stack
		RequireController: -1,
	}
}

// ApplyUnearth returns a creature from the graveyard to the battlefield
// with haste, registers a delayed trigger to exile at EOT, and registers
// a replacement effect to exile instead of going to any other zone.
// CR §702.84a-d.
//
// Returns the new Permanent on success, nil on failure.
func ApplyUnearth(gs *GameState, seatIdx int, card *Card, unearthCost int) *Permanent {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]

	// Check mana.
	if seat.ManaPool < unearthCost {
		return nil
	}

	// Remove from graveyard.
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return nil
	}

	// Pay mana.
	seat.ManaPool -= unearthCost
	SyncManaAfterSpend(seat)
	gs.LogEvent(Event{
		Kind:   "pay_mana",
		Seat:   seatIdx,
		Amount: unearthCost,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"reason": "unearth",
			"rule":   "702.84",
		},
	})

	// Put on battlefield with haste. §702.84a.
	perm := &Permanent{
		Card:          card,
		Controller:    seatIdx,
		Owner:         card.Owner,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{"kw:haste": 1, "unearthed": 1},
		SummoningSick: false, // Haste overrides summoning sickness.
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind:   "unearth",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.84a",
			"cost": unearthCost,
		},
	})

	// §702.84b: At the beginning of the next end step, exile it.
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seatIdx,
		SourceCardName: card.DisplayName() + " (unearth)",
		OneShot:        true,
		EffectFn: func(gs *GameState) {
			// Exile the permanent if it's still on the battlefield.
			if alive(gs, perm) {
				ExilePermanent(gs, perm, nil)
				gs.LogEvent(Event{
					Kind:   "unearth_exile_eot",
					Seat:   seatIdx,
					Source: card.DisplayName(),
					Details: map[string]interface{}{
						"rule": "702.84b",
					},
				})
			}
		},
	})

	// §702.84c: If it would leave the battlefield, exile it instead.
	// Register a replacement effect for any zone change from battlefield.
	RegisterUnearthExileReplacement(gs, perm)

	return perm
}

// RegisterUnearthExileReplacement registers a replacement effect so that
// if the unearthed permanent would leave the battlefield for any reason,
// it is exiled instead. CR §702.84c.
func RegisterUnearthExileReplacement(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_change_zone",
		HandlerID:      "unearth_exile:" + perm.Card.DisplayName() + ":" + itoaP1P2(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev == nil || ev.Source != perm {
				return false
			}
			fromZone, _ := ev.Payload["from_zone"].(string)
			toZone, _ := ev.Payload["to_zone"].(string)
			return fromZone == "battlefield" && toZone != "exile"
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Payload["to_zone"] = "exile"
			gs.LogEvent(Event{
				Kind:   "unearth_exile_replacement",
				Seat:   perm.Controller,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"rule": "702.84c",
				},
			})
		},
	})
}

// ---------------------------------------------------------------------------
// 2. Foretell — CR §702.143
//
// During your turn, pay {2} and exile from hand face-down (sorcery speed).
// Later, cast from exile for the foretell cost.
// ---------------------------------------------------------------------------

// ForetellExile exiles a card from hand face-down at sorcery speed for {2}.
// CR §702.143a: "During your turn, you may pay {2} and exile this card
// from your hand face down."
// Returns true on success.
func ForetellExile(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	// Must be this player's turn (sorcery speed).
	if gs.Active != seatIdx {
		return false
	}

	// Pay {2}.
	if seat.ManaPool < 2 {
		return false
	}

	// Remove from hand.
	if !removeFromZone(seat, card, ZoneHand) {
		return false
	}

	seat.ManaPool -= 2
	SyncManaAfterSpend(seat)

	// Exile face-down. (MoveCard clears FaceDown during zone transition;
	// re-set after placement so foretell semantics hold.)
	MoveCard(gs, card, seatIdx, "hand", "exile", "foretell-exile")
	card.FaceDown = true

	// Mark as foretold for later cast permission.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}

	gs.LogEvent(Event{
		Kind:   "foretell_exile",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.143a",
		},
	})

	return true
}

// NewForetellCastPermission creates a ZoneCastPermission for casting a
// foretold card from exile. The foretellCost is the discounted mana cost.
// CR §702.143b: "On a later turn, you may cast it from exile for its
// foretell cost."
func NewForetellCastPermission(foretellCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "foretell",
		ManaCost:          foretellCost,
		ExileOnResolve:    false, // Goes to graveyard normally after cast.
		RequireController: -1,
	}
}

// ---------------------------------------------------------------------------
// 3. Entwine — CR §702.42
//
// Modal spells: pay entwine cost to choose ALL modes instead of one.
// ---------------------------------------------------------------------------

// EntwineDecision represents the choice to pay entwine on a modal spell.
type EntwineDecision struct {
	// EntwineCost is the additional mana to pay for all modes.
	EntwineCost int

	// Entwined is true if the player chose (and can afford) to entwine.
	Entwined bool
}

// CanPayEntwine checks if a player can afford the entwine cost ON TOP OF
// the spell's normal mana cost. CR §702.42a.
func CanPayEntwine(gs *GameState, seatIdx int, spellCost int, entwineCost int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	return gs.Seats[seatIdx].ManaPool >= spellCost+entwineCost
}

// PayEntwineCost pays the additional entwine cost. Must be called after
// the normal mana cost is paid. Returns true on success.
func PayEntwineCost(gs *GameState, seatIdx int, card *Card, entwineCost int) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat.ManaPool < entwineCost {
		return false
	}
	seat.ManaPool -= entwineCost
	SyncManaAfterSpend(seat)
	gs.LogEvent(Event{
		Kind:   "pay_mana",
		Seat:   seatIdx,
		Amount: entwineCost,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"reason": "entwine",
			"rule":   "702.42",
		},
	})
	return true
}

// ShouldEntwine decides whether the GreedyHat should pay entwine.
// Greedy policy: if affordable, always entwine (more value).
func ShouldEntwine(gs *GameState, seatIdx int, spellCost int, entwineCost int) bool {
	return CanPayEntwine(gs, seatIdx, spellCost, entwineCost)
}

// ---------------------------------------------------------------------------
// 4. Buyback — CR §702.27
//
// Additional cost at cast time. On resolution, return to hand instead of
// graveyard.
// ---------------------------------------------------------------------------

// NewBuybackCost creates an AdditionalCost for buyback. The buybackMana
// is the extra mana to pay at cast time. When paid, the spell returns to
// hand on resolution instead of going to the graveyard.
func NewBuybackCost(buybackMana int) *AdditionalCost {
	return &AdditionalCost{
		Kind:       AddCostKindPayLife, // Reuse pay structure; actual cost is mana.
		Label:      "buyback",
		LifeAmount: 0, // We override with a custom CanPayFn/PayFn below.
		CanPayFn: func(gs *GameState, seatIdx int) bool {
			if seatIdx < 0 || seatIdx >= len(gs.Seats) {
				return false
			}
			return gs.Seats[seatIdx].ManaPool >= buybackMana
		},
		PayFn: func(gs *GameState, seatIdx int) bool {
			if seatIdx < 0 || seatIdx >= len(gs.Seats) {
				return false
			}
			seat := gs.Seats[seatIdx]
			if seat.ManaPool < buybackMana {
				return false
			}
			seat.ManaPool -= buybackMana
			SyncManaAfterSpend(seat)
			gs.LogEvent(Event{
				Kind:   "pay_mana",
				Seat:   seatIdx,
				Amount: buybackMana,
				Details: map[string]interface{}{
					"reason": "buyback",
					"rule":   "702.27",
				},
			})
			return true
		},
	}
}

// CastSpellWithBuyback casts a spell with the buyback additional cost.
// If buyback was paid, the stack item is tagged so ResolveStackTop returns
// it to hand instead of graveyard.
func CastSpellWithBuyback(gs *GameState, seatIdx int, card *Card, targets []Target, buybackMana int) (*CostPaymentResult, error) {
	buybackCost := NewBuybackCost(buybackMana)

	// Check if we can afford normal cost + buyback.
	normalCost := manaCostOf(card)
	canBuyback := gs != nil && seatIdx >= 0 && seatIdx < len(gs.Seats) &&
		gs.Seats[seatIdx].ManaPool >= normalCost+buybackMana

	var addCosts []*AdditionalCost
	if canBuyback {
		addCosts = []*AdditionalCost{buybackCost}
	}

	result, err := CastSpellWithCosts(gs, seatIdx, card, targets, nil, addCosts, false)
	if err != nil {
		return result, err
	}

	// Tag the stack item for buyback return-to-hand on resolution.
	if canBuyback && len(gs.Stack) > 0 {
		top := gs.Stack[len(gs.Stack)-1]
		if top != nil && top.Card == card {
			if top.CostMeta == nil {
				top.CostMeta = map[string]interface{}{}
			}
			top.CostMeta["buyback"] = true
		}
	}

	return result, nil
}

// ShouldReturnToHandOnResolve checks the stack item for buyback.
// Called by ResolveStackTop to determine post-resolution zone routing.
func ShouldReturnToHandOnResolve(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["buyback"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// ---------------------------------------------------------------------------
// 5. Wither — CR §702.80
//
// Damage dealt to creatures is dealt in the form of -1/-1 counters.
// Unlike infect, damage to players is normal.
// ---------------------------------------------------------------------------

// ApplyWitherDamageToCreature applies wither damage to a creature: instead
// of marking normal damage, place -1/-1 counters. CR §702.80a.
//
// Returns the number of -1/-1 counters placed.
func ApplyWitherDamageToCreature(gs *GameState, src *Permanent, target *Permanent, amount int) int {
	if gs == nil || target == nil || amount <= 0 {
		return 0
	}
	if target.Counters == nil {
		target.Counters = map[string]int{}
	}
	target.Counters["-1/-1"] += amount
	gs.InvalidateCharacteristicsCache() // -1/-1 counters change P/T

	gs.LogEvent(Event{
		Kind:   "wither_damage",
		Seat:   controllerSeat(src),
		Target: target.Controller,
		Source: sourceName(src),
		Amount: amount,
		Details: map[string]interface{}{
			"target_card": target.Card.DisplayName(),
			"counters":    target.Counters["-1/-1"],
			"rule":        "702.80a",
		},
	})
	return amount
}

// HasWither returns true if the permanent has the wither keyword.
func HasWither(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("wither")
}

// ---------------------------------------------------------------------------
// 6. Disturb — CR §702.146
//
// Cast from graveyard, enters transformed (back face). If it would be
// put into graveyard from battlefield, exile instead.
// ---------------------------------------------------------------------------

// NewDisturbPermission creates a ZoneCastPermission for disturb.
// The disturb cost is the mana to pay when casting from graveyard.
// CR §702.146a: "You may cast this card transformed from your graveyard
// for its disturb cost."
func NewDisturbPermission(disturbCost int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "disturb",
		ManaCost:          disturbCost,
		ExileOnResolve:    false, // Goes to battlefield transformed.
		RequireController: -1,
	}
}

// ApplyDisturbETB handles the disturb ETB: the permanent enters
// transformed (back face up). Also registers a replacement effect so
// if it would go to graveyard from battlefield, exile instead.
// CR §702.146b.
func ApplyDisturbETB(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}

	// Transform to back face.
	if perm.BackFaceAST != nil {
		perm.Transformed = true
		perm.Card.AST = perm.BackFaceAST
		if perm.BackFaceName != "" {
			perm.Card.Name = perm.BackFaceName
		}
		perm.Timestamp = gs.NextTimestamp()
	}

	// Mark as disturbed.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["disturbed"] = 1

	gs.LogEvent(Event{
		Kind:   "disturb_etb",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":        "702.146b",
			"transformed": true,
		},
	})

	// §702.146c: If it would be put into a graveyard from anywhere,
	// exile it instead.
	RegisterDisturbExileReplacement(gs, perm)
}

// RegisterDisturbExileReplacement registers a replacement effect: if the
// disturbed permanent would be put into a graveyard from the battlefield,
// exile it instead. CR §702.146c.
func RegisterDisturbExileReplacement(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_change_zone",
		HandlerID:      "disturb_exile:" + perm.Card.DisplayName() + ":" + itoaP1P2(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if ev == nil || ev.Source != perm {
				return false
			}
			fromZone, _ := ev.Payload["from_zone"].(string)
			toZone, _ := ev.Payload["to_zone"].(string)
			return fromZone == "battlefield" && toZone == "graveyard"
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Payload["to_zone"] = "exile"
			gs.LogEvent(Event{
				Kind:   "disturb_exile_replacement",
				Seat:   perm.Controller,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"rule": "702.146c",
				},
			})
		},
	})
}

// ---------------------------------------------------------------------------
// 7. Bushido — CR §702.45
//
// Whenever this creature blocks or becomes blocked, it gets +N/+N
// until end of turn.
// ---------------------------------------------------------------------------

// ApplyBushido adds the bushido P/T buff to a creature. Called from
// the declare-blockers step for creatures with bushido that block or
// become blocked. CR §702.45a.
func ApplyBushido(gs *GameState, perm *Permanent, bushidoN int) {
	if gs == nil || perm == nil || bushidoN <= 0 {
		return
	}
	perm.Modifications = append(perm.Modifications, Modification{
		Power:     bushidoN,
		Toughness: bushidoN,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache() // P/T modification changes characteristics
	gs.LogEvent(Event{
		Kind:   "bushido",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: bushidoN,
		Details: map[string]interface{}{
			"rule": "702.45a",
			"buff": "+" + itoaP1P2(bushidoN) + "/+" + itoaP1P2(bushidoN),
		},
	})
}

// GetBushidoN returns the bushido N value for a permanent.
// Reads from Flags["bushido_n"] or parses from keyword raw text.
// Returns 0 if no bushido.
func GetBushidoN(p *Permanent) int {
	if p == nil {
		return 0
	}
	// Check flags first (runtime override).
	if p.Flags != nil {
		if v, ok := p.Flags["bushido_n"]; ok && v > 0 {
			return v
		}
	}
	// Check keyword flag.
	if !p.HasKeyword("bushido") {
		return 0
	}
	// Default bushido 1 if keyword present but no N specified.
	return 1
}

// FireBushidoTriggers checks all creatures in a blocker assignment for
// bushido and applies the buff. Called from DeclareBlockers in combat.go.
// Handles both "this creature blocks" and "this creature becomes blocked".
func FireBushidoTriggers(gs *GameState, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) {
	if gs == nil {
		return
	}
	// Attackers that became blocked.
	for atk, blockers := range blockerMap {
		if len(blockers) == 0 {
			continue
		}
		n := GetBushidoN(atk)
		if n > 0 {
			ApplyBushido(gs, atk, n)
		}
		// Blockers that are blocking.
		for _, blk := range blockers {
			bn := GetBushidoN(blk)
			if bn > 0 {
				ApplyBushido(gs, blk, bn)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 8. Flanking — CR §702.25
//
// Whenever this creature becomes blocked by a creature without flanking,
// the blocking creature gets -1/-1 until end of turn.
// ---------------------------------------------------------------------------

// ApplyFlanking applies the flanking debuff to a blocker. CR §702.25a:
// "Whenever a creature without flanking blocks this creature, the
// blocking creature gets -1/-1 until end of turn."
func ApplyFlanking(gs *GameState, blocker *Permanent) {
	if gs == nil || blocker == nil {
		return
	}
	blocker.Modifications = append(blocker.Modifications, Modification{
		Power:     -1,
		Toughness: -1,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache() // -1/-1 modification changes P/T
	gs.LogEvent(Event{
		Kind:   "flanking",
		Seat:   blocker.Controller,
		Source: blocker.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.25a",
			"debuff": "-1/-1",
		},
	})
}

// FireFlankingTriggers checks for flanking interactions in a blocker
// assignment. For each attacker with flanking, any blocker WITHOUT
// flanking gets -1/-1 until end of turn. Called from combat.go.
func FireFlankingTriggers(gs *GameState, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) {
	if gs == nil {
		return
	}
	for _, atk := range attackers {
		if !atk.HasKeyword("flanking") {
			continue
		}
		blockers := blockerMap[atk]
		for _, blk := range blockers {
			if blk.HasKeyword("flanking") {
				continue // Creatures with flanking are immune.
			}
			ApplyFlanking(gs, blk)
		}
	}
}

// ---------------------------------------------------------------------------
// 9. Horsemanship — CR §702.30
//
// Evasion: can't be blocked except by creatures with horsemanship.
// Identical to flying but with a different keyword.
// ---------------------------------------------------------------------------

// CanBlockHorsemanship checks if a blocker can block an attacker with
// horsemanship. Only creatures with horsemanship can block creatures
// with horsemanship. CR §702.30a.
//
// This is wired into canBlock in combat.go.
func CanBlockHorsemanship(attacker, blocker *Permanent) bool {
	if attacker == nil || blocker == nil {
		return true
	}
	if !attacker.HasKeyword("horsemanship") {
		return true // No horsemanship on attacker, no restriction.
	}
	return blocker.HasKeyword("horsemanship")
}

// ---------------------------------------------------------------------------
// 10. Devoid — CR §702.114
//
// "This card has no color." Characteristic-defining ability in layer 5.
// ---------------------------------------------------------------------------

// RegisterDevoidEffect registers a layer-5 continuous effect that sets
// the permanent's colors to empty (colorless). CR §702.114a.
func RegisterDevoidEffect(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	handlerID := "devoid_" + perm.Card.DisplayName() + "_" + itoaP1P2(perm.Timestamp)

	eff := &ContinuousEffect{
		Layer:          LayerColor,
		Sublayer:       "",
		Timestamp:      perm.Timestamp,
		SourcePerm:     perm,
		SourceCardName: perm.Card.DisplayName(),
		ControllerSeat: perm.Controller,
		HandlerID:      handlerID,
		Duration:       DurationPermanent,
		Predicate: func(gs *GameState, target *Permanent) bool {
			return target == perm
		},
		ApplyFn: func(gs *GameState, target *Permanent, chars *Characteristics) {
			chars.Colors = []string{} // No color.
		},
	}
	gs.RegisterContinuousEffect(eff)

	gs.LogEvent(Event{
		Kind:   "devoid_registered",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.114a",
		},
	})
}

// HasDevoid returns true if the permanent has the devoid keyword.
func HasDevoid(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("devoid")
}

// ---------------------------------------------------------------------------
// 11. Shroud — CR §702.18
//
// "This permanent or player can't be the target of spells or abilities."
// Unlike hexproof, shroud prevents targeting by the controller too.
// ---------------------------------------------------------------------------

// HasShroud returns true if the permanent has the shroud keyword.
func HasShroud(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("shroud")
}

// HasHexproof returns true if the permanent has the hexproof keyword.
func HasHexproof(p *Permanent) bool {
	if p == nil {
		return false
	}
	return p.HasKeyword("hexproof")
}

// CanBeTargetedBy checks if a permanent can be targeted by a spell or
// ability controlled by seatIdx. Integrates both shroud (CR §702.18)
// and hexproof (CR §702.11). Protection is handled separately in combat.
//
//   - Shroud: can't be targeted by ANYONE (including controller).
//   - Hexproof: can't be targeted by OPPONENTS (controller can target).
//
// Returns true if targeting is legal.
func CanBeTargetedBy(perm *Permanent, seatIdx int) bool {
	if perm == nil {
		return false
	}
	if HasShroud(perm) {
		return false // No one can target.
	}
	if HasHexproof(perm) && perm.Controller != seatIdx {
		return false // Opponents can't target.
	}
	return true
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// itoaP1P2 is a local int-to-string helper to avoid importing strconv or
// conflicting with itoa in other files (each file in the package defines
// its own local variant to avoid redeclaration).
func itoaP1P2(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [12]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// sourceName returns the display name of a source permanent, or "<nil>"
// if nil. This is a forwarding helper — the same function exists in
// zone_change.go, but we define it here conditionally to avoid
// redeclaration since both files live in the same package.
// NOTE: This is already defined in zone_change.go; we reference it
// directly rather than redefining.

// controllerSeat returns the controller seat of a permanent, or -1 if nil.
// NOTE: This is already defined in zone_change.go; we reference it
// directly rather than redefining.

// ---------------------------------------------------------------------------
// Combat integration hooks
// ---------------------------------------------------------------------------

// CheckCombatKeywordsP1P2 is called from the combat code after blockers
// are declared. It fires bushido and flanking triggers for P1P2 keywords.
// Separated into its own function so combat.go only needs a single call.
func CheckCombatKeywordsP1P2(gs *GameState, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) {
	FireBushidoTriggers(gs, attackers, blockerMap)
	FireFlankingTriggers(gs, attackers, blockerMap)
}

// CanBlockP1P2 performs P1P2-specific blocking legality checks.
// Returns false if the block is illegal due to horsemanship.
// This is called from canBlock in combat.go.
func CanBlockP1P2(attacker, blocker *Permanent) bool {
	// Horsemanship evasion.
	if !CanBlockHorsemanship(attacker, blocker) {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Wither/combat damage integration
// ---------------------------------------------------------------------------

// ShouldApplyWitherDamage returns true if the source has wither and the
// target is a creature. Used by combat damage code to decide whether to
// route damage through ApplyWitherDamageToCreature.
func ShouldApplyWitherDamage(src *Permanent, target *Permanent) bool {
	if src == nil || target == nil {
		return false
	}
	if !HasWither(src) {
		return false
	}
	return target.IsCreature()
}

// ---------------------------------------------------------------------------
// DevoidCardCheck — for card-level devoid (before battlefield)
// ---------------------------------------------------------------------------

// CardHasDevoid checks if a card has devoid in its keyword list.
// Used for cards in hand/stack where there's no Permanent yet.
func CardHasDevoid(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	return astHasKeyword(card.AST, "devoid")
}

// GetDevoidColors returns empty slice for devoid cards, or the card's
// normal colors otherwise. Used by targeting and color checks.
func GetDevoidColors(card *Card) []string {
	if CardHasDevoid(card) {
		return []string{}
	}
	if card == nil {
		return nil
	}
	return card.Colors
}

// ---------------------------------------------------------------------------
// P1P2 keyword name constants
// ---------------------------------------------------------------------------

const (
	KeywordUnearth      = "unearth"
	KeywordForetell     = "foretell"
	KeywordEntwine      = "entwine"
	KeywordBuyback      = "buyback"
	KeywordWither       = "wither"
	KeywordDisturb      = "disturb"
	KeywordBushido      = "bushido"
	KeywordFlanking     = "flanking"
	KeywordHorsemanship = "horsemanship"
	KeywordDevoid       = "devoid"
	KeywordShroud       = "shroud"
)

// AllP1P2Keywords returns a list of all P1+P2 keyword names for
// registration and enumeration.
func AllP1P2Keywords() []string {
	return []string{
		KeywordUnearth,
		KeywordForetell,
		KeywordEntwine,
		KeywordBuyback,
		KeywordWither,
		KeywordDisturb,
		KeywordBushido,
		KeywordFlanking,
		KeywordHorsemanship,
		KeywordDevoid,
		KeywordShroud,
	}
}

// HasP1P2Keyword returns true if the permanent has any of the P1P2
// keywords. Used for fast-path checks.
func HasP1P2Keyword(p *Permanent) bool {
	if p == nil {
		return false
	}
	for _, kw := range AllP1P2Keywords() {
		if p.HasKeyword(kw) {
			return true
		}
	}
	return false
}

// Ensure we use the strings import (for future extension).
var _ = strings.ToLower
