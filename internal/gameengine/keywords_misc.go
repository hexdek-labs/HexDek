package gameengine

// Miscellaneous keyword implementations — graveyard, counter/token,
// set-specific mechanics, and condition-check ("ability words").
//
// Batch 2 of the keyword sweep. Every keyword below has an engine-level
// handler so simulations observe the mechanic.
//
// GRAVEYARD keywords:
//   - Dredge N          — CR §702.52
//   - Embalm            — CR §702.128
//   - Eternalize        — CR §702.129
//   - Encore            — CR §702.142
//   - Delve             — CR §702.66
//   - Scavenge          — CR §702.97
//   - Retrace           — CR §702.81
//   - Jump-start        — CR §702.133
//
// COUNTER/TOKEN keywords:
//   - Adapt N           — CR §702.137
//   - Monstrosity N     — CR §702.94
//   - Fabricate N       — CR §702.123
//   - Reinforce N       — CR §702.77
//   - Bolster N         — CR §701.32
//   - Support N         — CR §701.35
//   - Modular N         — CR §702.43
//   - Graft N           — CR §702.58
//   - Amplify N         — CR §702.38
//   - Sunburst          — CR §702.44
//   - Living Weapon     — CR §702.92
//   - Reconfigure       — CR §702.151
//
// SET-SPECIFIC mechanics:
//   - Explore           — CR §701.40
//   - Connive           — CR §701.48
//   - Discover N        — CR §701.51
//   - Cloak             — CR §701.56 (face-down manifest variant)
//   - Venture / Initiative / The Ring (simplified)
//   - Class levels      — (simplified)
//   - Sagas             — (simplified)
//
// CONDITION-CHECK helpers (ability words):
//   - CheckThreshold    — 7+ cards in graveyard
//   - CheckDelirium     — 4+ card types in graveyard
//   - CheckMetalcraft   — 3+ artifacts you control
//   - CheckFerocious    — creature with power 4+ you control
//   - CheckSpellMastery — 2+ instant/sorcery in graveyard
//   - CheckRevolt       — permanent left battlefield this turn
//   - CheckFormidable   — total power of your creatures >= 8
//   - CheckRaid         — you attacked this turn
//   - CountDomain       — count basic land types among your lands
//   - CountConverge     — count colors of mana spent (simplified)
//   - CountDevotion     — count mana pips in costs of your permanents
//
// TRIGGER keywords:
//   - Landfall          — triggers when land ETBs under your control
//   - Constellation     — triggers when enchantment ETBs under your control
//   - Heroic            — triggers when you cast targeting this creature
//   - Alliance          — triggers when another creature ETBs under your control
//   - Magecraft         — triggers when you cast/copy instant/sorcery

import (
	"math"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Local itoa helper (each keyword file uses its own to avoid redeclaration)
// ---------------------------------------------------------------------------

func itoaMisc(n int) string {
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

// ===========================================================================
// GRAVEYARD KEYWORDS
// ===========================================================================

// ---------------------------------------------------------------------------
// Dredge N — CR §702.52
// ---------------------------------------------------------------------------
//
// "If you would draw a card, instead you may mill N cards, then return
// this card from your graveyard to your hand."
//
// The player replaces their draw with: mill N from library top → graveyard,
// then return the dredge card from graveyard to hand. If fewer than N
// cards remain in library, cannot dredge.

// GetDredgeN returns the dredge N value from a card's AST keywords.
// Returns (N, true) if found.
func GetDredgeN(card *Card) (int, bool) {
	if card == nil || card.AST == nil {
		return 0, false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) == "dredge" {
			n := 1
			if len(kw.Args) > 0 {
				if v, ok2 := kw.Args[0].(float64); ok2 {
					n = int(v)
				} else if v, ok2 := kw.Args[0].(int); ok2 {
					n = v
				}
			}
			return n, true
		}
	}
	return 0, false
}

// ActivateDredge replaces a draw with dredge: mill N cards from library,
// then return this card from graveyard to hand. Returns true on success.
// Fails if library has fewer than N cards.
func ActivateDredge(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	n, hasDredge := GetDredgeN(card)
	if !hasDredge {
		return false
	}
	seat := gs.Seats[seatIdx]

	// Must have enough cards in library to mill.
	if len(seat.Library) < n {
		return false
	}

	// Verify card is in graveyard.
	gyIdx := -1
	for i, c := range seat.Graveyard {
		if c == card {
			gyIdx = i
			break
		}
	}
	if gyIdx < 0 {
		return false
	}

	// Mill N cards (top of library → graveyard).
	for i := 0; i < n; i++ {
		if len(seat.Library) == 0 {
			break
		}
		milled := seat.Library[0]
		MoveCard(gs, milled, seatIdx, "library", "graveyard", "dredge")
	}

	// Return dredge card from graveyard to hand.
	MoveCard(gs, card, seatIdx, "graveyard", "hand", "return-from-graveyard")

	gs.LogEvent(Event{
		Kind:   "dredge",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"milled": n,
			"rule":   "702.52",
		},
	})
	return true
}

// DredgeChooser is an optional Hat capability (CR §702.52 — dredge is a
// "you may" draw-replacement). Given the dredge-eligible cards in the
// player's graveyard (the engine has already verified library size >= each
// card's N), the hat returns the card to dredge, or nil to draw normally.
// A hat that doesn't implement this never dredges (it just draws).
type DredgeChooser interface {
	ChooseDredge(gs *GameState, seatIdx int, candidates []*Card) *Card
}

// MaybeDredgeReplaceDraw offers the dredge draw-replacement at a would-draw
// event for seatIdx (CR §702.52 / §614: "if you would draw a card, instead
// you may mill N and return this card from your graveyard to your hand").
// Returns true if the draw was REPLACED by a dredge — the caller must then
// NOT draw and NOT increment the draw count (the draw did not happen).
//
// This is the single chokepoint every draw entry point (the turn draw step,
// DrawN, and the AST Draw effect) consults before drawing, so dredge fires
// on ANY would-draw regardless of source. Before r63 the ActivateDredge
// primitive existed but had zero callers — dredge was entirely inert.
func MaybeDredgeReplaceDraw(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return false
	}
	seat := gs.Seats[seatIdx]
	// Eligible: a card in the graveyard with dredge N where the library has
	// at least N cards (CR §702.52b — can't dredge with too few cards).
	var candidates []*Card
	seen := map[*Card]bool{}
	for _, c := range seat.Graveyard {
		if c == nil || seen[c] {
			continue
		}
		if n, ok := GetDredgeN(c); ok && n > 0 && len(seat.Library) >= n {
			candidates = append(candidates, c)
			seen[c] = true
		}
	}
	if len(candidates) == 0 {
		return false
	}
	chooser, ok := seat.Hat.(DredgeChooser)
	if !ok {
		return false
	}
	choice := chooser.ChooseDredge(gs, seatIdx, candidates)
	if choice == nil {
		return false
	}
	return ActivateDredge(gs, seatIdx, choice)
}

// ---------------------------------------------------------------------------
// Embalm — CR §702.128
// ---------------------------------------------------------------------------
//
// Activated ability from graveyard. Exile this creature card from graveyard:
// create a token that's a copy of it, except it's white, has no mana cost,
// and is a Zombie in addition to its other types.

// ActivateEmbalm exiles a creature from graveyard and creates a token copy
// that is white, has no mana cost, and gains the Zombie type.
// Returns the token permanent on success, nil on failure.
func ActivateEmbalm(gs *GameState, seatIdx int, card *Card, embalmCost int) *Permanent {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]

	// Pay mana.
	if seat.ManaPool < embalmCost {
		return nil
	}

	// Remove from graveyard.
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return nil
	}
	seat.ManaPool -= embalmCost
	SyncManaAfterSpend(seat)

	// Exile the original.
	MoveCard(gs, card, seatIdx, "graveyard", "exile", "effect")

	// Create token copy: add white to existing colors, no mana cost, Zombie type added.
	tokenTypes := append([]string{"token", "creature", "zombie"}, card.Types...)
	// Per CR §702.128, the token is white IN ADDITION TO its other colors.
	embalmColors := append([]string{}, card.Colors...)
	hasWhite := false
	for _, c := range embalmColors {
		if c == "W" {
			hasWhite = true
			break
		}
	}
	if !hasWhite {
		embalmColors = append(embalmColors, "W")
	}
	token := &Card{
		Name:          card.DisplayName() + " (Embalmed)",
		Owner:         seatIdx,
		BasePower:     card.BasePower,
		BaseToughness: card.BaseToughness,
		Types:         tokenTypes,
		Colors:        embalmColors,
		CMC:           0,
	}
	perm := &Permanent{
		Card:       token,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind:   "embalm",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"cost": embalmCost,
			"rule": "702.128",
		},
	})
	return perm
}

// ---------------------------------------------------------------------------
// Eternalize — CR §702.129
// ---------------------------------------------------------------------------
//
// Like embalm, but the token is always a 4/4 (ignoring original P/T).

// ActivateEternalize exiles a creature from graveyard and creates a 4/4
// token copy that is black, has no mana cost, and gains Zombie type.
func ActivateEternalize(gs *GameState, seatIdx int, card *Card, eternalizeCost int) *Permanent {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]

	if seat.ManaPool < eternalizeCost {
		return nil
	}
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return nil
	}
	seat.ManaPool -= eternalizeCost
	SyncManaAfterSpend(seat)
	MoveCard(gs, card, seatIdx, "graveyard", "exile", "effect")

	tokenTypes := append([]string{"token", "creature", "zombie"}, card.Types...)
	// Per CR §702.129, the token is black IN ADDITION TO its other colors.
	eternalizeColors := append([]string{}, card.Colors...)
	hasBlack := false
	for _, c := range eternalizeColors {
		if c == "B" {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		eternalizeColors = append(eternalizeColors, "B")
	}
	token := &Card{
		Name:          card.DisplayName() + " (Eternalized)",
		Owner:         seatIdx,
		BasePower:     4,
		BaseToughness: 4,
		Types:         tokenTypes,
		Colors:        eternalizeColors,
		CMC:           0,
	}
	perm := &Permanent{
		Card:       token,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind:   "eternalize",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"cost": eternalizeCost,
			"rule": "702.129",
		},
	})
	return perm
}

// ---------------------------------------------------------------------------
// Encore — CR §702.142
// ---------------------------------------------------------------------------
//
// Exile from graveyard: for each opponent, create a token copy that attacks
// that opponent if able. Sacrifice the tokens at end of turn.

// ActivateEncore exiles a creature from graveyard, creates token copies
// (one per opponent), gives them haste, forces them to attack, and
// registers a delayed trigger to sacrifice at EOT.
func ActivateEncore(gs *GameState, seatIdx int, card *Card, encoreCost int) []*Permanent {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]

	if seat.ManaPool < encoreCost {
		return nil
	}
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return nil
	}
	seat.ManaPool -= encoreCost
	SyncManaAfterSpend(seat)
	MoveCard(gs, card, seatIdx, "graveyard", "exile", "effect")

	opponents := gs.Opponents(seatIdx)
	if len(opponents) == 0 {
		opponents = []int{1 - seatIdx} // Fallback for 2-player
	}

	var tokens []*Permanent
	for _, oppIdx := range opponents {
		token := &Card{
			Name:          card.DisplayName() + " (Encore)",
			Owner:         seatIdx,
			BasePower:     card.BasePower,
			BaseToughness: card.BaseToughness,
			Types:         append([]string{"token", "creature"}, card.Types...),
			Colors:        card.Colors,
			CMC:           0,
		}
		perm := &Permanent{
			Card:       token,
			Controller: seatIdx,
			Owner:      seatIdx,
			Timestamp:  gs.NextTimestamp(),
			Counters:   map[string]int{},
			Flags: map[string]int{
				"kw:haste":           1,
				"encore_must_attack": oppIdx + 1, // +1 offset to distinguish from 0
			},
		}
		seat.Battlefield = append(seat.Battlefield, perm)
		RegisterReplacementsForPermanent(gs, perm)
		FirePermanentETBTriggers(gs, perm)
		tokens = append(tokens, perm)
	}

	// Register delayed trigger: sacrifice all encore tokens at EOT.
	encoreTokens := make([]*Permanent, len(tokens))
	copy(encoreTokens, tokens)
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seatIdx,
		SourceCardName: card.DisplayName() + " (encore)",
		OneShot:        true,
		EffectFn: func(gs *GameState) {
			for _, t := range encoreTokens {
				if alive(gs, t) {
					SacrificePermanent(gs, t, "encore EOT sacrifice")
				}
			}
		},
	})

	gs.LogEvent(Event{
		Kind:   "encore",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: len(tokens),
		Details: map[string]interface{}{
			"cost":      encoreCost,
			"opponents": len(opponents),
			"rule":      "702.142",
		},
	})
	return tokens
}

// ---------------------------------------------------------------------------
// Delve — CR §702.66
// ---------------------------------------------------------------------------
//
// As an additional cost, exile any number of cards from graveyard. Each
// exiled card pays for {1} of the spell's generic mana cost.

// DelveMaxReduction returns the maximum generic mana reduction from delve
// for a given seat (= number of cards in graveyard).
func DelveMaxReduction(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	return len(gs.Seats[seatIdx].Graveyard)
}

// HasDelve returns true if the card has the delve keyword.
func HasDelve(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	return astHasKeyword(card.AST, "delve")
}

// PayDelve exiles cards from graveyard to reduce the generic mana cost.
// Returns the number of cards exiled (= mana reduced).
func PayDelve(gs *GameState, seatIdx int, card *Card, maxToExile int) int {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	exiled := 0
	for exiled < maxToExile && len(seat.Graveyard) > 0 {
		// Exile from the end (most recently added).
		c := seat.Graveyard[len(seat.Graveyard)-1]
		MoveCard(gs, c, seatIdx, "graveyard", "exile", "exile-from-graveyard")
		exiled++
	}
	if exiled > 0 {
		gs.LogEvent(Event{
			Kind:   "delve",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Amount: exiled,
			Details: map[string]interface{}{
				"rule": "702.66",
			},
		})
	}
	return exiled
}

// ---------------------------------------------------------------------------
// Scavenge — CR §702.97
// ---------------------------------------------------------------------------
//
// Exile this card from graveyard: put a number of +1/+1 counters equal to
// its power on target creature.

// ActivateScavenge exiles a creature from graveyard to put +1/+1 counters
// on a target creature equal to the scavenged card's power.
func ActivateScavenge(gs *GameState, seatIdx int, card *Card, target *Permanent, scavengeCost int) bool {
	if gs == nil || card == nil || target == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	if seat.ManaPool < scavengeCost {
		return false
	}
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return false
	}
	seat.ManaPool -= scavengeCost
	SyncManaAfterSpend(seat)
	MoveCard(gs, card, seatIdx, "graveyard", "exile", "effect")

	counters := card.BasePower
	if counters < 0 {
		counters = 0
	}
	target.AddCounter("+1/+1", counters)
	gs.InvalidateCharacteristicsCache() // +1/+1 counters change P/T

	gs.LogEvent(Event{
		Kind:   "scavenge",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: counters,
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"cost":   scavengeCost,
			"rule":   "702.97",
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// Retrace — CR §702.81
// ---------------------------------------------------------------------------
//
// You may cast this card from your graveyard by discarding a land card
// as an additional cost.

// CastWithRetrace casts a spell from graveyard by discarding a land.
// Simplified version for engine testing.
func CastWithRetrace(gs *GameState, seatIdx int, card *Card, landToDiscard *Card) bool {
	if gs == nil || card == nil || landToDiscard == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	// CR §118.9 / §601.2f — retrace casts the spell from the graveyard for its
	// mana cost (discarding a land is the additional cost). Cost modifiers
	// apply to that mana cost.
	manaCost := EffectiveAlternativeCost(gs, card, seatIdx, card.CMC, CastContext{FromGraveyard: true})

	// Must afford mana.
	if seat.ManaPool < manaCost {
		return false
	}

	// Card must be in graveyard.
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return false
	}

	// Land must be in hand.
	landIdx := -1
	for i, c := range seat.Hand {
		if c == landToDiscard {
			landIdx = i
			break
		}
	}
	if landIdx < 0 {
		// Put card back in graveyard.
		seat.Graveyard = append(seat.Graveyard, card)
		return false
	}

	// Pay costs.
	seat.ManaPool -= manaCost
	SyncManaAfterSpend(seat)
	DiscardCard(gs, landToDiscard, seatIdx)

	// The spell resolves and goes back to graveyard (retrace allows
	// re-casting). Route through MoveCard for §614 replacements
	// (Rest in Peace, Leyline of the Void) and zone-change triggers.
	MoveCard(gs, card, seatIdx, "stack", "graveyard", "retrace-resolve")

	gs.LogEvent(Event{
		Kind:   "retrace",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"discarded_land": landToDiscard.DisplayName(),
			"cost":           manaCost,
			"rule":           "702.81",
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// Jump-start — CR §702.133
// ---------------------------------------------------------------------------
//
// You may cast this card from your graveyard by discarding a card as an
// additional cost. If you do, exile this spell instead of putting it into
// graveyard as it resolves.

// CastWithJumpStart casts an instant/sorcery from graveyard by discarding
// a card. The spell is exiled after resolution.
func CastWithJumpStart(gs *GameState, seatIdx int, card *Card, cardToDiscard *Card) bool {
	if gs == nil || card == nil || cardToDiscard == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	// CR §118.9 / §601.2f — jump-start casts the spell from the graveyard for
	// its mana cost (discarding a card is the additional cost). Cost modifiers
	// apply to that mana cost.
	manaCost := EffectiveAlternativeCost(gs, card, seatIdx, card.CMC, CastContext{FromGraveyard: true})

	if seat.ManaPool < manaCost {
		return false
	}

	// Card must be in graveyard.
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return false
	}

	// Discard card must be in hand.
	discIdx := -1
	for i, c := range seat.Hand {
		if c == cardToDiscard {
			discIdx = i
			break
		}
	}
	if discIdx < 0 {
		seat.Graveyard = append(seat.Graveyard, card)
		return false
	}

	// Pay costs.
	seat.ManaPool -= manaCost
	SyncManaAfterSpend(seat)
	DiscardCard(gs, cardToDiscard, seatIdx)

	// CR §702.131a — push the spell onto the stack so its EFFECT actually
	// resolves (the old path moved it straight to exile, so a jump-started
	// Chemister's Insight discarded a card and exiled itself without ever
	// drawing). The exile_on_resolve flag hands off to stack.go's
	// ShouldExileOnResolve branch, which routes the card to exile instead of
	// the graveyard as it leaves the stack.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneGraveyard,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"exile_on_resolve":  true,
			"zone_cast_keyword": "jump-start",
			"jump_start":        true,
		},
	}
	PushStackItem(gs, item)

	gs.LogEvent(Event{
		Kind:   "jump_start",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"discarded": cardToDiscard.DisplayName(),
			"cost":      manaCost,
			"rule":      "702.131a",
		},
	})
	return true
}

// ===========================================================================
// COUNTER/TOKEN KEYWORDS
// ===========================================================================

// ---------------------------------------------------------------------------
// Adapt N — CR §702.137
// ---------------------------------------------------------------------------
//
// If this creature has no +1/+1 counters on it, put N +1/+1 counters on it.

// ActivateAdapt puts N +1/+1 counters on perm if it has no +1/+1 counters.
// Returns true if counters were placed.
func ActivateAdapt(gs *GameState, perm *Permanent, n int, adaptCost int) bool {
	if gs == nil || perm == nil || n <= 0 {
		return false
	}
	seat := gs.Seats[perm.Controller]

	// Pay cost.
	if seat.ManaPool < adaptCost {
		return false
	}

	// Check: must have no +1/+1 counters.
	if perm.Counters != nil && perm.Counters["+1/+1"] > 0 {
		return false
	}

	seat.ManaPool -= adaptCost
	SyncManaAfterSpend(seat)
	perm.AddCounter("+1/+1", n)
	gs.InvalidateCharacteristicsCache() // +1/+1 counters change P/T

	gs.LogEvent(Event{
		Kind:   "adapt",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"cost": adaptCost,
			"rule": "702.137",
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// Monstrosity N — CR §702.94
// ---------------------------------------------------------------------------
//
// If this creature isn't monstrous, put N +1/+1 counters on it and it
// becomes monstrous.

// ActivateMonstrosity puts N +1/+1 counters on perm and marks it monstrous.
// Returns true if the creature became monstrous.
func ActivateMonstrosity(gs *GameState, perm *Permanent, n int, monstCost int) bool {
	if gs == nil || perm == nil || n <= 0 {
		return false
	}
	seat := gs.Seats[perm.Controller]

	if seat.ManaPool < monstCost {
		return false
	}

	// Check: must not already be monstrous.
	if perm.Flags != nil && perm.Flags["monstrous"] > 0 {
		return false
	}

	seat.ManaPool -= monstCost
	SyncManaAfterSpend(seat)
	perm.AddCounter("+1/+1", n)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["monstrous"] = 1
	gs.InvalidateCharacteristicsCache() // +1/+1 counters change P/T

	gs.LogEvent(Event{
		Kind:   "monstrosity",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"cost": monstCost,
			"rule": "702.94",
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// Fabricate N — CR §702.123
// ---------------------------------------------------------------------------
//
// ETB choice: put N +1/+1 counters on this creature, OR create N 1/1
// colorless Servo artifact creature tokens.

// ApplyFabricate applies the fabricate ETB choice. If chooseCounters is true,
// puts N +1/+1 counters. Otherwise creates N Servo tokens.
func ApplyFabricate(gs *GameState, perm *Permanent, n int, chooseCounters bool) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	if chooseCounters {
		PutCountersTriggered(gs, perm, "+1/+1", n, perm)
		gs.LogEvent(Event{
			Kind:   "fabricate_counters",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: n,
			Details: map[string]interface{}{
				"rule": "702.123",
			},
		})
	} else {
		// CR §111.10 — Servo tokens go through the §616 would_create_token
		// doubler chain (Doubling Season / Parallel Lives / Anointed
		// Procession). The prior raw loop bypassed every token doubler.
		created := CreateDoubledTokens(gs, perm.Controller, n, perm, func() *Permanent {
			return CreateServoToken(gs, perm.Controller)
		})
		if len(created) > 0 && (gs.Flags == nil || gs.Flags["in_token_trigger"] == 0) {
			if gs.Flags == nil {
				gs.Flags = map[string]int{}
			}
			gs.Flags["in_token_trigger"] = 1
			FireCardTrigger(gs, "token_created", map[string]interface{}{
				"controller_seat": perm.Controller,
				"count":           len(created),
				"types":           []string{"token", "artifact", "creature", "servo"},
				"source":          perm.Card.DisplayName(),
			})
			gs.Flags["in_token_trigger"] = 0
		}
		gs.LogEvent(Event{
			Kind:   "fabricate_tokens",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: len(created),
			Details: map[string]interface{}{
				"rule": "702.123",
			},
		})
	}
}

// CreateServoToken creates a 1/1 colorless Servo artifact creature token and
// returns the new permanent (nil on invalid seat). Mints exactly one token and
// does NOT fire the doubler chain — callers creating several use
// CreateDoubledTokens.
func CreateServoToken(gs *GameState, seatIdx int) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	token := &Card{
		Name:          "Servo Token",
		Owner:         seatIdx,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "artifact", "creature", "servo"},
	}
	perm := &Permanent{
		Card:       token,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	gs.LogEvent(Event{
		Kind:   "create_token",
		Seat:   seatIdx,
		Source: "Servo Token",
		Details: map[string]interface{}{
			"subtype": "servo",
			"rule":    "111.10",
		},
	})
	return perm
}

// ---------------------------------------------------------------------------
// Reinforce N — CR §702.77
// ---------------------------------------------------------------------------
//
// Discard this card from hand: put N +1/+1 counters on target creature.

// ActivateReinforce discards this card from hand and puts N +1/+1 counters
// on target creature. Returns true on success.
func ActivateReinforce(gs *GameState, seatIdx int, card *Card, target *Permanent, n int, reinforceCost int) bool {
	if gs == nil || card == nil || target == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	if seat.ManaPool < reinforceCost {
		return false
	}

	// Card must be in hand.
	handIdx := -1
	for i, c := range seat.Hand {
		if c == card {
			handIdx = i
			break
		}
	}
	if handIdx < 0 {
		return false
	}

	seat.ManaPool -= reinforceCost
	SyncManaAfterSpend(seat)
	DiscardCard(gs, card, seatIdx)

	PutCountersTriggered(gs, target, "+1/+1", n, nil)

	gs.LogEvent(Event{
		Kind:   "reinforce",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"cost":   reinforceCost,
			"rule":   "702.77",
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// Bolster N — CR §701.32
// ---------------------------------------------------------------------------
//
// Put N +1/+1 counters on the creature you control with the least toughness.

// ApplyBolster puts N +1/+1 counters on the creature with the least
// toughness controlled by seatIdx. Returns the bolstered permanent or nil.
func ApplyBolster(gs *GameState, seatIdx int, n int) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || n <= 0 {
		return nil
	}

	var lowest *Permanent
	lowestTough := 1<<31 - 1

	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		t := p.Toughness()
		if t < lowestTough {
			lowestTough = t
			lowest = p
		}
	}

	if lowest == nil {
		return nil
	}

	PutCountersTriggered(gs, lowest, "+1/+1", n, nil)

	gs.LogEvent(Event{
		Kind:   "bolster",
		Seat:   seatIdx,
		Source: lowest.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"target_toughness": lowestTough,
			"rule":             "701.32",
		},
	})
	return lowest
}

// ---------------------------------------------------------------------------
// Support N — CR §701.35
// ---------------------------------------------------------------------------
//
// Put a +1/+1 counter on each of up to N target creatures you control.

// ApplySupport puts one +1/+1 counter on each of up to N creatures
// controlled by seatIdx.
func ApplySupport(gs *GameState, seatIdx int, n int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || n <= 0 {
		return 0
	}

	count := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if count >= n {
			break
		}
		if p == nil || !p.IsCreature() {
			continue
		}
		PutCountersTriggered(gs, p, "+1/+1", 1, nil)
		count++
	}

	if count > 0 {
		gs.InvalidateCharacteristicsCache() // +1/+1 counters change P/T
		gs.LogEvent(Event{
			Kind:   "support",
			Seat:   seatIdx,
			Amount: count,
			Details: map[string]interface{}{
				"max_targets": n,
				"rule":        "701.35",
			},
		})
	}
	return count
}

// ---------------------------------------------------------------------------
// Modular N — CR §702.43
// ---------------------------------------------------------------------------
//
// ETB with N +1/+1 counters. When this creature dies, you may put its
// +1/+1 counters on target artifact creature.

// ApplyModularETB puts N +1/+1 counters on perm at ETB.
func ApplyModularETB(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	PutCountersTriggered(gs, perm, "+1/+1", n, perm)
	gs.LogEvent(Event{
		Kind:   "modular_etb",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.43",
		},
	})
}

// ApplyModularDeath transfers +1/+1 counters from the dying permanent
// to a target artifact creature.
func ApplyModularDeath(gs *GameState, dying *Permanent, target *Permanent) {
	if gs == nil || dying == nil || target == nil {
		return
	}
	counters := 0
	if dying.Counters != nil {
		counters = dying.Counters["+1/+1"]
	}
	if counters <= 0 {
		return
	}
	PutCountersTriggered(gs, target, "+1/+1", counters, dying)
	gs.LogEvent(Event{
		Kind:   "modular_death",
		Seat:   dying.Controller,
		Source: dying.Card.DisplayName(),
		Amount: counters,
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"rule":   "702.43",
		},
	})
}

// ---------------------------------------------------------------------------
// Graft N — CR §702.58
// ---------------------------------------------------------------------------
//
// ETB with N +1/+1 counters. Whenever another creature ETBs, you may move
// one +1/+1 counter from this permanent to that creature.

// ApplyGraftETB puts N +1/+1 counters on perm at ETB.
func ApplyGraftETB(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	PutCountersTriggered(gs, perm, "+1/+1", n, perm)
	gs.LogEvent(Event{
		Kind:   "graft_etb",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.58",
		},
	})
}

// ApplyGraftTransfer moves one +1/+1 counter from source to target.
// Returns true if transfer happened.
func ApplyGraftTransfer(gs *GameState, source *Permanent, target *Permanent) bool {
	if gs == nil || source == nil || target == nil {
		return false
	}
	if source.Counters == nil || source.Counters["+1/+1"] <= 0 {
		return false
	}
	source.AddCounter("+1/+1", -1)
	target.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache() // +1/+1 counter movement changes P/T
	gs.LogEvent(Event{
		Kind:   "graft_transfer",
		Seat:   source.Controller,
		Source: source.Card.DisplayName(),
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"rule":   "702.58",
		},
	})
	return true
}

// ApplyGraft wires both halves of graft (CR §702.58) when `perm` enters the
// battlefield. ApplyGraftETB / ApplyGraftTransfer existed but had no caller —
// graft was fully inert. Called from BOTH ETB-counter paths (the cast path
// resolvePermanentSpellETB and the non-cast FirePermanentETBTriggers), beside
// the other enters-with-counters helpers.
//
//  1. Self: if `perm` has graft N, it enters with N +1/+1 counters, routed
//     through the §616 doubler chain (ApplyGraftETB → PutCountersTriggered).
//  2. Others: `perm` entering is "another creature" for every OTHER graft
//     creature on the battlefield (CR §702.58c) — each such creature's
//     controller may move one +1/+1 counter from it onto `perm`. Auto-play
//     moves a counter only onto a creature the graft controller controls (you
//     don't feed an opponent's creature), and only while the graft creature
//     still has a +1/+1 counter. The move is raw (a counter MOVE, not a put —
//     it does not re-trigger the doubler chain).
//
// Both halves run synchronously at ETB, mirroring the engine's other
// enters-with-counters hooks; the CR models §702.58c as a triggered ability, a
// simplification shared with the rest of the ETB-counter family.
func ApplyGraft(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// (1) Self — enters with N +1/+1 counters.
	if n, ok := keywordArgCostStrict(perm.Card, "graft"); ok && n > 0 {
		ApplyGraftETB(gs, perm, n)
	}
	// (2) Others — only a creature can receive the moved counter.
	if !perm.IsCreature() {
		return
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, g := range seat.Battlefield {
			if g == nil || g == perm || g.Card == nil || g.PhasedOut {
				continue
			}
			if g.Counters == nil || g.Counters["+1/+1"] <= 0 {
				continue
			}
			if _, ok := keywordArgCostStrict(g.Card, "graft"); !ok {
				continue
			}
			// CR §702.58c — the graft creature's controller may move a counter
			// onto the entering creature. Auto-play: do so only when the entering
			// creature is one the graft controller controls.
			if perm.Controller != g.Controller {
				continue
			}
			ApplyGraftTransfer(gs, g, perm)
		}
	}
}

// ---------------------------------------------------------------------------
// Amplify N — CR §702.38
// ---------------------------------------------------------------------------
//
// As this creature ETBs, you may reveal any number of creature cards from
// your hand. For each card revealed, it enters with N additional +1/+1
// counters.

// ApplyAmplify puts amplifyN * revealCount +1/+1 counters on perm.
// revealCount is the number of cards revealed from hand sharing a creature
// type. The counters route through the §616 doubler chain (PutCountersTriggered:
// Doubling Season / Hardened Scales etc. apply + counter_placed fires), not raw
// AddCounter.
func ApplyAmplify(gs *GameState, perm *Permanent, amplifyN int, revealCount int) {
	if gs == nil || perm == nil || amplifyN <= 0 || revealCount <= 0 {
		return
	}
	counters := amplifyN * revealCount
	PutCountersTriggered(gs, perm, "+1/+1", counters, perm)
	gs.LogEvent(Event{
		Kind:   "amplify",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: counters,
		Details: map[string]interface{}{
			"amplify_n": amplifyN,
			"revealed":  revealCount,
			"rule":      "702.38",
		},
	})
}

// ApplyAmplifyETB performs amplify (CR §702.38b) as `perm` enters: reveal hand
// cards that share a creature type with perm; perm enters with amplifyN +1/+1
// counters for EACH revealed card. ApplyAmplify had no caller — amplify was
// inert (no reveal, no counters). Auto-play reveals ALL eligible cards (free
// counters, no downside); revealing zero is legal and places no counters.
func ApplyAmplifyETB(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	n, ok := keywordArgCostStrict(perm.Card, "amplify")
	if !ok || n <= 0 {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return
	}
	// CR §702.38b — only cards sharing one of perm's creature types count.
	mine := creatureSubtypesOf(perm.Card)
	if len(mine) == 0 {
		return
	}
	revealCount := 0
	for _, c := range gs.Seats[seatIdx].Hand {
		if c == nil {
			continue
		}
		for sub := range creatureSubtypesOf(c) {
			if _, shared := mine[sub]; shared {
				revealCount++
				break
			}
		}
	}
	if revealCount <= 0 {
		return // revealing zero is legal → zero counters
	}
	ApplyAmplify(gs, perm, n, revealCount)
}

// CountCreaturesInHand counts the number of creature cards in a seat's hand.
func CountCreaturesInHand(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	count := 0
	for _, c := range gs.Seats[seatIdx].Hand {
		if c != nil && hasTypeInSlice(c.Types, "creature") {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Sunburst — CR §702.44
// ---------------------------------------------------------------------------
//
// ETB with a +1/+1 counter (or charge counter on non-creatures) for each
// color of mana spent to cast it.

// ApplySunburst adds counters based on colors of mana spent. For creatures,
// adds +1/+1 counters. For non-creatures, adds charge counters.
// colorsSpent is the number of distinct colors used.
func ApplySunburst(gs *GameState, perm *Permanent, colorsSpent int) {
	if gs == nil || perm == nil || colorsSpent <= 0 {
		return
	}
	counterType := "+1/+1"
	if !perm.IsCreature() {
		counterType = "charge"
	}
	perm.AddCounter(counterType, colorsSpent)
	if counterType == "+1/+1" {
		gs.InvalidateCharacteristicsCache() // +1/+1 counters change P/T
	}
	gs.LogEvent(Event{
		Kind:   "sunburst",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: colorsSpent,
		Details: map[string]interface{}{
			"counter_type": counterType,
			"rule":         "702.44",
		},
	})
}

// ---------------------------------------------------------------------------
// Living Weapon — CR §702.92
// ---------------------------------------------------------------------------
//
// When this Equipment enters, create a 0/0 black Phyrexian Germ creature
// token, then attach this Equipment to it.

// ApplyLivingWeapon creates a 0/0 Phyrexian Germ token and attaches the
// equipment to it.
func ApplyLivingWeapon(gs *GameState, equipment *Permanent) *Permanent {
	if gs == nil || equipment == nil || equipment.Card == nil {
		return nil
	}
	seatIdx := equipment.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}

	// CR §702.91b — create a 0/0 black Phyrexian Germ creature token through
	// the canonical token path (CreateDoubledTokens → token doublers fire;
	// CreateCreatureToken → token_created + ETB + InstanceID mint), not a raw
	// &Permanent loop. The "token" type prefix is added by CreateCreatureToken.
	var germ *Permanent
	CreateDoubledTokens(gs, seatIdx, 1, equipment, func() *Permanent {
		t := CreateCreatureToken(gs, seatIdx, "Phyrexian Germ Token",
			[]string{"creature", "phyrexian", "germ"}, 0, 0)
		if t != nil && t.Card != nil {
			t.Card.Colors = []string{"B"}
			if germ == nil {
				germ = t
			}
		}
		return t
	})
	if germ == nil {
		return nil
	}

	// CR §702.91b — then attach this Equipment to the Germ (no equip cost is
	// paid). The equip's +X/+Y and ability grants are AttachedTo-scoped layer
	// effects, so with the equipment's continuous effects already registered
	// at its own ETB, setting AttachedTo makes the Germ a real creature; a
	// +N/+N equipment immediately lifts the 0/0 off the §704.5f SBA. When the
	// Equipment later leaves or unattaches, the Germ reverts to 0/0 and dies.
	equipment.AttachedTo = germ
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "living_weapon",
		Seat:   seatIdx,
		Source: equipment.Card.DisplayName(),
		Details: map[string]interface{}{
			"germ": "Phyrexian Germ Token",
			"rule": "702.91b",
		},
	})
	return germ
}

// HasLivingWeapon reports whether the card carries the living weapon keyword
// (CR §702.91). Normalizes spaces/underscores so it matches the parser token
// regardless of "living weapon" / "living_weapon" spelling.
func HasLivingWeapon(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		norm := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(kw.Name), " ", ""), "_", "")
		if norm == "livingweapon" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Reconfigure — CR §702.151
// ---------------------------------------------------------------------------
//
// Creature/equipment dual mode. Equip = stop being a creature and attach.
// Unattach = become a creature again.

// ActivateReconfigure attaches this creature/equipment to target creature,
// making it stop being a creature. If already attached, detaches and
// becomes a creature again.
func ActivateReconfigure(gs *GameState, perm *Permanent, target *Permanent, reconfigureCost int) bool {
	if gs == nil || perm == nil {
		return false
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	// Determine the action up front so an illegal attach target is rejected
	// BEFORE any cost is paid. Attached → unattach (§702.151b second mode);
	// unattached → attach to target (first mode).
	if perm.AttachedTo == nil {
		// CR §702.151c — reconfigure can only attach to a creature you
		// control, and an Equipment can't equip itself (a reconfigure
		// permanent is both a creature and an equipment, so the self-check
		// matters). Guarding here also keeps AttachmentConsistency intact:
		// an equipment must be attached to a creature.
		if target == nil || target == perm || !target.IsCreature() || target.Controller != seatIdx {
			return false
		}
	}

	if seat.ManaPool < reconfigureCost {
		return false
	}
	seat.ManaPool -= reconfigureCost
	SyncManaAfterSpend(seat)

	if perm.AttachedTo != nil {
		// Detach: becomes a creature again.
		perm.AttachedTo = nil
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		delete(perm.Flags, "reconfigured")
		gs.LogEvent(Event{
			Kind:   "reconfigure_detach",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"rule": "702.151",
			},
		})
	} else if target != nil {
		// Attach to target: stops being a creature.
		perm.AttachedTo = target
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		perm.Flags["reconfigured"] = 1
		gs.LogEvent(Event{
			Kind:   "reconfigure_attach",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"target": target.Card.DisplayName(),
				"rule":   "702.151",
			},
		})
	}
	return true
}

// IsReconfigured returns true if the permanent is currently attached
// (reconfigured into equipment mode, not a creature).
func IsReconfigured(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["reconfigured"] > 0
}

// ===========================================================================
// SET-SPECIFIC MECHANICS
// ===========================================================================

// ---------------------------------------------------------------------------
// Explore — CR §701.40
// ---------------------------------------------------------------------------
//
// Reveal the top card of your library. If it's a land card, put it into
// your hand. Otherwise, put a +1/+1 counter on this creature, then you
// may put the revealed card into your graveyard.

// PerformExplore performs the explore action for a creature.
// Returns (wasLand, revealedCard).
func PerformExplore(gs *GameState, perm *Permanent) (bool, *Card) {
	if gs == nil || perm == nil {
		return false, nil
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false, nil
	}
	seat := gs.Seats[seatIdx]

	if len(seat.Library) == 0 {
		// CR §701.40 — the creature still explores even with an empty
		// library (there's just no card to reveal); "whenever a creature you
		// control explores" triggers still fire.
		fireExploreTrigger(gs, perm, seatIdx, false, nil)
		return false, nil
	}

	// Reveal top card.
	revealed := seat.Library[0]

	isLand := hasTypeInSlice(revealed.Types, "land")

	if isLand {
		// Land: put into hand.
		MoveCard(gs, revealed, seatIdx, "library", "hand", "tutor-to-hand")
		gs.LogEvent(Event{
			Kind:   "explore_land",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"revealed": revealed.DisplayName(),
				"rule":     "701.40",
			},
		})
	} else {
		// Nonland: +1/+1 counter, may put to graveyard (greedy: always GY).
		perm.AddCounter("+1/+1", 1)
		gs.InvalidateCharacteristicsCache() // +1/+1 counter changes P/T
		MoveCard(gs, revealed, seatIdx, "library", "graveyard", "explore")
		gs.LogEvent(Event{
			Kind:   "explore_nonland",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"revealed": revealed.DisplayName(),
				"counter":  "+1/+1",
				"rule":     "701.40",
			},
		})
	}
	// CR §701.40 — fire the "explore" trigger event ONCE per explore (after
	// the reveal/counter resolves) so "whenever a creature you control
	// explores" abilities (Wildgrowth Walker, Tatyova-style, the ally_explore
	// alias family) react. Without this the explore-matters trigger family was
	// inert despite the event-alias plumbing.
	fireExploreTrigger(gs, perm, seatIdx, isLand, revealed)
	return isLand, revealed
}

// fireExploreTrigger dispatches the per-explore "explore" trigger event for
// the creature `perm` (controlled by seatIdx) that just explored. `revealed`
// is the revealed card (nil on an empty library).
func fireExploreTrigger(gs *GameState, perm *Permanent, seatIdx int, wasLand bool, revealed *Card) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	ctx := map[string]interface{}{
		"seat":            seatIdx,
		"controller_seat": seatIdx,
		"perm":            perm,
		"card":            perm.Card,
		"was_land":        wasLand,
	}
	if revealed != nil {
		ctx["revealed_card"] = revealed.DisplayName()
	}
	FireCardTrigger(gs, "explore", ctx)
}

// ---------------------------------------------------------------------------
// Connive — CR §701.48
// ---------------------------------------------------------------------------
//
// Draw a card, then discard a card. If you discarded a nonland card,
// put a +1/+1 counter on this creature.

// PerformConnive draws a card, discards a card, and if the discarded
// card is nonland, puts a +1/+1 counter on the creature.
func PerformConnive(gs *GameState, perm *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	// Draw a card.
	drawn, ok := gs.drawOne(seatIdx)
	if !ok {
		return false
	}

	// Discard a card (greedy: discard the last card in hand — often the
	// just-drawn card or the least useful).
	if len(seat.Hand) == 0 {
		return false
	}
	discarded := seat.Hand[len(seat.Hand)-1]
	DiscardCard(gs, discarded, seatIdx)

	isNonland := !hasTypeInSlice(discarded.Types, "land")
	if isNonland {
		perm.AddCounter("+1/+1", 1)
		gs.InvalidateCharacteristicsCache() // +1/+1 counter changes P/T
	}

	gs.LogEvent(Event{
		Kind:   "connive",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"drawn":     drawn.DisplayName(),
			"discarded": discarded.DisplayName(),
			"nonland":   isNonland,
			"rule":      "701.48",
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// Discover N — CR §701.51
// ---------------------------------------------------------------------------
//
// Exile cards from the top of your library until you exile a nonland card
// with mana value N or less. You may cast it without paying its mana cost.
// Put the rest on the bottom of your library in a random order.

// PerformDiscover exiles from library top until a nonland card with
// CMC <= N is found. Returns the discovered card (put into hand if not cast).
func PerformDiscover(gs *GameState, seatIdx int, n int) *Card {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || n < 0 {
		return nil
	}
	seat := gs.Seats[seatIdx]

	var exiled []*Card
	var found *Card

	for len(seat.Library) > 0 {
		top := seat.Library[0]
		seat.Library = seat.Library[1:]

		if top != nil && !hasTypeInSlice(top.Types, "land") && top.CMC <= n {
			found = top
			break
		}
		exiled = append(exiled, top)
	}

	// Put exiled cards on the bottom of library (random order).
	if gs.Rng != nil && len(exiled) > 1 {
		gs.Rng.Shuffle(len(exiled), func(i, j int) {
			exiled[i], exiled[j] = exiled[j], exiled[i]
		})
	}
	seat.Library = append(seat.Library, exiled...)

	if found != nil {
		// CR §701.51: "You may cast that card without paying its mana cost.
		// If you don't, put it into your hand." AI heuristic: cast for free
		// if CMC ≥ 3 (good value), otherwise put into hand for flexibility.
		castForFree := found.CMC >= 3
		if castForFree {
			// CR §701.51: the discovered card is CAST (without paying its mana
			// cost). A cast spell must run the full cast-time pipeline, not a
			// bare stack push: set its spell Effect so instants/sorceries
			// actually resolve, count toward storm / cast count, fire "whenever
			// you cast a spell" triggers (magecraft, prowess, Rhystic Study,
			// …), trigger the discovered card's OWN cast keywords (storm, a
			// chained cascade, a chained discover), open a priority window so it
			// can be countered, then resolve it. Mirrors cascade.go's corrected
			// free-cast block — the previous raw gs.Stack append skipped ALL of
			// this, so a discovered instant/sorcery resolved as a no-op (no
			// Effect), cast-triggers never fired, and discover off a discover
			// never chained.
			item := &StackItem{
				Card:       found,
				Controller: seatIdx,
				Effect:     collectSpellEffect(found),
				CastZone:   ZoneExile,
				CostMeta: map[string]interface{}{
					"discover_cast": true,
					"alt_cost":      "discover",
					"discover_n":    n,
					"free_cast":     true,
				},
			}
			PushStackItem(gs, item)
			IncrementCastCount(gs, seatIdx)
			RecordCast(gs, seatIdx, found, 0)
			fireCastTriggers(gs, seatIdx, found)
			if HasStormKeyword(found) {
				ApplyStormCopies(gs, item, seatIdx)
			}
			// CR §702.85b — a chained cascade fires once per cascade instance.
			if cc := CascadeCount(found); cc > 0 {
				foundMV := manaCostOf(found)
				for i := 0; i < cc; i++ {
					ApplyCascade(gs, seatIdx, foundMV, found.DisplayName())
				}
			}
			// CR §701.51 — a chained discover resolves independently. Prefer the
			// discovered card's own discover N from its keyword; fall back to
			// its mana value when the parser didn't capture an explicit N
			// (mirrors the cascade entry path's manaCostOf fallback).
			if cardHasKeyword(found, "discover") {
				chainN := DiscoverCount(found)
				if chainN <= 0 {
					chainN = manaCostOf(found)
				}
				PerformDiscover(gs, seatIdx, chainN)
			}
			InvokeCastHook(gs, item)
			gs.LogEvent(Event{
				Kind:   "discover_cast",
				Seat:   seatIdx,
				Source: found.DisplayName(),
				Amount: n,
				Details: map[string]interface{}{
					"action":       "cast_for_free",
					"exiled_count": len(exiled),
					"rule":         "701.51",
				},
			})
			// CR §117.3 / §117.4 — opponents receive priority before the
			// discovered spell resolves. Guarded resolve: if a response landed
			// on top, leave the stack for the outer DrainStack.
			PriorityRound(gs)
			if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == item {
				ResolveStackTop(gs)
				StateBasedActions(gs)
			}
		} else {
			MoveCard(gs, found, seatIdx, "library", "hand", "tutor-to-hand")
			gs.LogEvent(Event{
				Kind:   "discover_to_hand",
				Seat:   seatIdx,
				Source: found.DisplayName(),
				Amount: n,
				Details: map[string]interface{}{
					"action":       "put_into_hand",
					"exiled_count": len(exiled),
					"rule":         "701.51",
				},
			})
		}
	}
	return found
}

// ---------------------------------------------------------------------------
// Cloak — CR §701.56 (face-down manifest variant)
// ---------------------------------------------------------------------------
//
// Put a card from some zone onto the battlefield face down as a 2/2
// creature. It can be turned face up for its mana cost.

// CloakFaceDownWardCost is the ward cost a cloaked permanent has while
// face down (CR §702.171a — "a 2/2 creature with ward {2}"). This is the
// defining difference between cloak and manifest, which grants no ward.
const CloakFaceDownWardCost = 2

// PerformCloak puts a card face-down as a 2/2 creature with ward {2}.
func PerformCloak(gs *GameState, seatIdx int, card *Card) *Permanent {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]

	card.FaceDown = true
	perm := &Permanent{
		Card: &Card{
			Name:          "Face-Down Creature",
			Owner:         seatIdx,
			BasePower:     2,
			BaseToughness: 2,
			Types:         []string{"creature"},
			FaceDown:      true,
		},
		Controller:    seatIdx,
		Owner:         seatIdx,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags: map[string]int{
			"cloaked": 1,
			// CR §702.171a — a cloaked permanent is face down WITH ward {2}
			// (the key cloak-vs-manifest difference). Mirror the disguise
			// face-down ward flags so CheckWardOnTargeting enforces it.
			"kw:ward":   1,
			"ward_cost": CloakFaceDownWardCost,
		},
	}
	// Store original card reference for face-up.
	perm.FrontFaceAST = card.AST
	perm.FrontFaceName = card.DisplayName()

	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind:   "cloak",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "701.56",
		},
	})
	return perm
}

// TurnCloakedFaceUp turns a cloaked creature face up, delegating to the
// existing TurnFaceUp in dfc.go. Restores the original card name from
// cloak metadata.
func TurnCloakedFaceUp(gs *GameState, perm *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}
	// Delegate to dfc.go's TurnFaceUp.
	ok := TurnFaceUp(gs, perm, "cloak")
	if ok && perm.Flags != nil {
		delete(perm.Flags, "cloaked")
	}
	return ok
}

// ---------------------------------------------------------------------------
// Venture into the Dungeon (simplified)
// ---------------------------------------------------------------------------
//
// Simplified dungeon implementation: track room index per player.
// The full dungeon has branching rooms; we model it as a linear track
// of 4 rooms with simple effects.

// VentureIntoDungeon advances the player one room in the dungeon.
// Returns the room number reached (1-indexed).
func VentureIntoDungeon(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["dungeon_room"]++
	room := seat.Flags["dungeon_room"]

	// Simplified room effects.
	switch room {
	case 1:
		// Room 1: Scry 1 (simplified: put top card on bottom).
		if len(seat.Library) > 0 {
			top := seat.Library[0]
			seat.Library = seat.Library[1:]
			seat.Library = append(seat.Library, top)
		}
	case 2:
		// Room 2: Create a Treasure token.
		CreateTreasureToken(gs, seatIdx)
	case 3:
		// Room 3: Gain 1 life.
		GainLife(gs, seatIdx, 1, "dungeon_room_3")
	case 4:
		// Room 4: Draw a card (dungeon complete).
		gs.drawOne(seatIdx)
		seat.Flags["dungeon_completed"]++
		seat.Flags["dungeon_room"] = 0 // Reset for next dungeon.
	}

	gs.LogEvent(Event{
		Kind:   "venture",
		Seat:   seatIdx,
		Amount: room,
		Details: map[string]interface{}{
			"room": room,
			"rule": "309.1",
		},
	})
	return room
}

// ---------------------------------------------------------------------------
// Initiative (simplified) — Undercity dungeon
// ---------------------------------------------------------------------------

// TakeInitiative gives the initiative to a player. If they already have it,
// they venture into the Undercity.
func TakeInitiative(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}

	// Remove initiative from previous holder.
	for i := range gs.Seats {
		if gs.Seats[i] != nil && gs.Seats[i].Flags != nil {
			delete(gs.Seats[i].Flags, "has_initiative")
		}
	}

	seat := gs.Seats[seatIdx]
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["has_initiative"] = 1
	gs.Flags["initiative_holder"] = seatIdx

	// Venture into Undercity (uses same dungeon track).
	VentureIntoDungeon(gs, seatIdx)

	gs.LogEvent(Event{
		Kind: "take_initiative",
		Seat: seatIdx,
		Details: map[string]interface{}{
			"rule": "722.1",
		},
	})
}

// CheckInitiativeCombatSteal transfers the initiative to the attacker's
// controller when a creature deals combat damage to the player who has the
// initiative (CR §722 / Undercity). Mirrors CheckMonarchCombatSteal — this was
// the missing initiative analogue: the monarch's combat-steal was wired into
// combat damage but the initiative's was not, so the initiative could never be
// stolen by attacking its holder. Taking the initiative also ventures into the
// Undercity (TakeInitiative handles the prior-holder removal + venture).
func CheckInitiativeCombatSteal(gs *GameState, damagedSeat, attackerSeat int) {
	if gs == nil {
		return
	}
	if attackerSeat == damagedSeat {
		return
	}
	if !HasInitiative(gs, damagedSeat) {
		return
	}
	TakeInitiative(gs, attackerSeat)
}

// HasInitiative returns true if the given seat has the initiative.
func HasInitiative(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat.Flags == nil {
		return false
	}
	return seat.Flags["has_initiative"] > 0
}

// ---------------------------------------------------------------------------
// The Ring Tempts You (simplified)
// ---------------------------------------------------------------------------

// TheRingTemptsYou designates or levels up the ring-bearer. The ring
// has 4 levels, each granting cumulative abilities.
func TheRingTemptsYou(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["ring_level"]++
	level := seat.Flags["ring_level"]
	if level > 4 {
		level = 4
		seat.Flags["ring_level"] = 4
	}

	// CR §701.52a — each time the Ring tempts you, you choose a creature you
	// control to become the Ring-bearer (you may keep the same one). Derive
	// "do I have a Ring-bearer right now" from the battlefield rather than a
	// sticky seat flag: a creature you control flagged ring_bearer. Keep it if
	// it's still there; otherwise (first tempt, OR the previous bearer left —
	// §702, designation cleared until you choose again) choose a new one. Only
	// one bearer at a time, so clear any stray flags before designating.
	//
	// The pre-r63 sticky `ring_bearer_set` gate never re-designated after the
	// bearer left, so a seat whose Ring-bearer died could never get a new one
	// — every Ring-bearer-conditional ability went permanently dormant.
	hasLiveBearer := false
	for _, p := range seat.Battlefield {
		if p != nil && p.Flags != nil && p.Flags["ring_bearer"] == 1 && p.IsCreature() {
			hasLiveBearer = true
			break
		}
	}
	if !hasLiveBearer {
		// Clear stale designations (the prior bearer left), then choose one.
		for _, p := range seat.Battlefield {
			if p != nil && p.Flags != nil {
				delete(p.Flags, "ring_bearer")
			}
		}
		seat.Flags["ring_bearer_set"] = 0
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				if p.Flags == nil {
					p.Flags = map[string]int{}
				}
				p.Flags["ring_bearer"] = 1
				seat.Flags["ring_bearer_set"] = 1
				break
			}
		}
	}

	gs.LogEvent(Event{
		Kind:   "ring_tempts",
		Seat:   seatIdx,
		Amount: level,
		Details: map[string]interface{}{
			"rule": "701.52",
		},
	})
	FireCardTrigger(gs, "ring_tempt", map[string]interface{}{
		"seat":  seatIdx,
		"level": level,
	})
}

// GetRingLevel returns the ring level for a seat (0-4).
func GetRingLevel(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat.Flags == nil {
		return 0
	}
	return seat.Flags["ring_level"]
}

// ---------------------------------------------------------------------------
// Class Levels (simplified)
// ---------------------------------------------------------------------------

// AdvanceClassLevel advances a Class enchantment to the next level.
// Returns the new level.
func AdvanceClassLevel(gs *GameState, perm *Permanent, levelUpCost int) int {
	if gs == nil || perm == nil {
		return 0
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]

	if seat.ManaPool < levelUpCost {
		return 0
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	currentLevel := perm.Flags["class_level"]
	if currentLevel >= 3 {
		return currentLevel // Already at max.
	}

	seat.ManaPool -= levelUpCost
	SyncManaAfterSpend(seat)
	perm.Flags["class_level"] = currentLevel + 1
	newLevel := currentLevel + 1

	gs.LogEvent(Event{
		Kind:   "class_level_up",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: newLevel,
		Details: map[string]interface{}{
			"cost": levelUpCost,
		},
	})
	return newLevel
}

// ---------------------------------------------------------------------------
// Sagas (simplified)
// ---------------------------------------------------------------------------

// AddLoreCounters places n lore counters on a Saga and fires a chapter
// trigger for EACH chapter number the lore total newly reaches, in ascending
// order. CR §714.2b: a chapter ability triggers when the number of lore
// counters becomes greater than or equal to that chapter's number — so an add
// that crosses several chapters at once (proliferate stacking, a lore doubler,
// an "add two lore counters" effect, or read-ahead entry) fires each crossed
// chapter once, in order, not just the final one.
//
// Lore is written to perm.Counters (the authoritative store P/T and SBA
// §704.5s read). "perm" is threaded into the event so self-referential chapter
// handlers (dispatchSagaPhase7 et al.) gate on identity rather than firing for
// every controlled Saga.
func AddLoreCounters(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || perm.Card == nil || n <= 0 {
		return
	}
	prev := perm.Counters["lore"]
	perm.AddCounter("lore", n)
	next := perm.Counters["lore"]
	for chapter := prev + 1; chapter <= next; chapter++ {
		gs.LogEvent(Event{
			Kind:   "saga_chapter",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: chapter,
			Details: map[string]interface{}{
				"rule": "714.2",
			},
		})
		FireCardTrigger(gs, "lore_counter_added", map[string]interface{}{
			"seat":    perm.Controller,
			"card":    perm.Card.DisplayName(),
			"chapter": chapter,
			"perm":    perm,
		})
		dispatchGenericSagaChapter(gs, perm, chapter)
	}
}

// dispatchGenericSagaChapter resolves a Saga's chapter ability straight from
// its AST when no per_card handler owns the card (CR §714.2c). Before this,
// the lore-tick path only reached the ~25 phase-7 + 6 DFC per_card saga
// handlers; the other ~187 corpus Sagas fired lore_counter_added into the void
// — their chapter abilities were INERT (dead-dispatch / unwired primitive,
// r63 saga audit). It reuses the SAME canonical resolution the
// resolveModificationEffect "saga_chapter" case uses (parsed Effect →
// ResolveEffect, parser raw-text fallback → resolveResidualByText), minus that
// case's lore-counter add (AddLoreCounters already placed the counter — calling
// the modification case would double it). per_card-owned Sagas (History of
// Benalia, Sheoldred, Elesh Norn, …) are skipped so they never double-fire.
func dispatchGenericSagaChapter(gs *GameState, perm *Permanent, chapter int) {
	if gs == nil || perm == nil || perm.Card == nil || perm.Card.AST == nil || chapter <= 0 {
		return
	}
	if !perm.IsSaga() {
		return
	}
	// A dedicated per_card OnTrigger("…","lore_counter_added",…) owns this
	// card's chapter dispatch — let it run, don't double-resolve.
	if PerCardOwnsTrigger(perm.Card.DisplayName(), "lore_counter_added") {
		return
	}
	for _, ab := range perm.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "saga_chapter" {
			continue
		}
		args := st.Modification.Args
		if len(args) < 1 || !sagaChapterArgMatches(args[0], chapter) {
			continue
		}
		resolvedAny := false
		for _, arg := range args[1:] {
			switch a := arg.(type) {
			case gameast.Effect:
				ResolveEffect(gs, perm, a)
				resolvedAny = true
			case string:
				if a != "" && resolveResidualByText(gs, perm, a) {
					resolvedAny = true
				}
			}
		}
		gs.LogEvent(Event{
			Kind:   "saga_chapter_generic",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: chapter,
			Details: map[string]interface{}{
				"rule":     "714.2c",
				"resolved": resolvedAny,
			},
		})
	}
}

// sagaChapterArgMatches reports whether a saga_chapter Static's chapter arg
// (a roman string "II" or a list ["I","II"]) includes the given chapter.
func sagaChapterArgMatches(arg interface{}, chapter int) bool {
	switch v := arg.(type) {
	case string:
		n, ok := romanToInt[v]
		return ok && n == chapter
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if n, ok := romanToInt[s]; ok && n == chapter {
					return true
				}
			}
		}
	}
	return false
}

// AdvanceSagaChapter adds one lore counter and fires the newly-reached chapter
// ability. Returns the new chapter number. Thin wrapper over AddLoreCounters
// (the +1 case fires exactly one chapter).
func AdvanceSagaChapter(gs *GameState, perm *Permanent) int {
	if gs == nil || perm == nil {
		return 0
	}
	AddLoreCounters(gs, perm, 1)
	// If final chapter (typically 3), the saga will be sacrificed by SBA §704.5s.
	return perm.Counters["lore"]
}

// GetSagaChapter returns the current chapter of a Saga.
func GetSagaChapter(perm *Permanent) int {
	if perm == nil || perm.Counters == nil {
		return 0
	}
	return perm.Counters["lore"]
}

// TickSagaChapters places a lore counter on each Saga the active seat
// controls per CR §714.2b ("As your precombat main phase begins, you
// put a lore counter on each Saga you control"). Iterates over a
// snapshot so chapter abilities that modify the battlefield during
// dispatch don't reorder mid-loop. Skips phased-out and already-
// defeated sagas (lore counter past final_chapter — SBA §704.5s will
// sacrifice them on the next pass; ticking would just double-fire the
// final chapter).
//
// Called from internal/tournament/turn.go at the start of the
// precombat main phase, mirroring FireRadCounterTriggers.
func TickSagaChapters(gs *GameState, activeSeat int) int {
	if gs == nil || activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[activeSeat]
	if seat == nil || seat.Lost {
		return 0
	}
	ticked := 0
	for _, p := range snapshotBattlefield(seat) {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsSaga() {
			continue
		}
		if p.PhasedOut {
			continue
		}
		if p.Counters != nil {
			final, has := p.Counters["saga_final_chapter"]
			if has && final > 0 && p.Counters["lore"] >= final {
				continue
			}
		}
		AdvanceSagaChapter(gs, p)
		ticked++
	}
	return ticked
}

// ===========================================================================
// CONDITION-CHECK HELPERS (ability words)
// ===========================================================================

// ---------------------------------------------------------------------------
// Threshold — 7+ cards in graveyard
// ---------------------------------------------------------------------------

// CheckThreshold returns true if the seat has 7 or more cards in graveyard.
func CheckThreshold(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	return len(gs.Seats[seatIdx].Graveyard) >= 7
}

// ---------------------------------------------------------------------------
// Delirium — 4+ card types in graveyard
// ---------------------------------------------------------------------------

// CheckDelirium returns true if the seat has 4 or more distinct card types
// in their graveyard (creature, instant, sorcery, artifact, enchantment,
// land, planeswalker, tribal).
func CheckDelirium(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	types := make(map[string]bool)
	for _, c := range gs.Seats[seatIdx].Graveyard {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			lower := strings.ToLower(t)
			switch lower {
			case "creature", "instant", "sorcery", "artifact", "enchantment",
				"land", "planeswalker", "tribal", "battle", "kindred":
				types[lower] = true
			}
		}
	}
	return len(types) >= 4
}

// ---------------------------------------------------------------------------
// Metalcraft — 3+ artifacts you control
// ---------------------------------------------------------------------------
//
// Canonical implementation (HasMetalcraft + MetalcraftActive +
// ArtifactCount) lives in keywords_metalcraft.go. CheckMetalcraft is
// kept here as a thin alias so existing call sites in per_card/ and
// resolve-time rider lookup continue to work without an import rename.

// CheckMetalcraft is the legacy alias for MetalcraftActive. New code
// should call MetalcraftActive directly.
func CheckMetalcraft(gs *GameState, seatIdx int) bool {
	return MetalcraftActive(gs, seatIdx)
}

// ---------------------------------------------------------------------------
// Ferocious — you control a creature with power 4+
// ---------------------------------------------------------------------------

// CheckFerocious returns true if the seat controls a creature with
// power 4 or greater.
func CheckFerocious(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p != nil && p.IsCreature() && p.Power() >= 4 {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Spell Mastery — 2+ instant/sorcery in graveyard
// ---------------------------------------------------------------------------

// CheckSpellMastery returns true if the seat has 2 or more instant and/or
// sorcery cards in their graveyard.
func CheckSpellMastery(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	count := 0
	for _, c := range gs.Seats[seatIdx].Graveyard {
		if c == nil {
			continue
		}
		if hasTypeInSlice(c.Types, "instant") || hasTypeInSlice(c.Types, "sorcery") {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Revolt — a permanent left the battlefield this turn
// ---------------------------------------------------------------------------

// CheckRevolt returns true if any permanent left the battlefield this turn
// under seatIdx's control. Reads from the event log for the current turn.
func CheckRevolt(gs *GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	for _, ev := range gs.EventLog {
		if ev.Seat == seatIdx &&
			(ev.Kind == "destroy" || ev.Kind == "sacrifice" || ev.Kind == "exile" ||
				ev.Kind == "bounce") {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Formidable — total power of creatures you control >= 8
// ---------------------------------------------------------------------------

// CheckFormidable returns true if the total power of creatures controlled
// by seatIdx is 8 or greater.
func CheckFormidable(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	totalPower := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p != nil && p.IsCreature() {
			totalPower += p.Power()
		}
	}
	return totalPower >= 8
}

// ---------------------------------------------------------------------------
// Raid — you attacked this turn
// ---------------------------------------------------------------------------

// CheckRaid returns true if seatIdx attacked with any creature this turn.
// Reads from the event log.
func CheckRaid(gs *GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	for _, ev := range gs.EventLog {
		if ev.Seat == seatIdx && ev.Kind == "declare_attackers" {
			return true
		}
	}
	if seatIdx >= 0 && seatIdx < len(gs.Seats) {
		if gs.Seats[seatIdx].Turn.Attacked {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Domain — count basic land types among lands you control
// ---------------------------------------------------------------------------

// CountDomain counts the number of distinct basic land types (Plains,
// Island, Swamp, Mountain, Forest) among lands controlled by seatIdx.
// Returns 0-5.
func CountDomain(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	found := make(map[string]bool)
	basicTypes := []string{"plains", "island", "swamp", "mountain", "forest"}

	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsLand() || p.Card == nil {
			continue
		}
		name := strings.ToLower(p.Card.DisplayName())
		for _, bt := range basicTypes {
			if name == bt || hasTypeInSlice(p.Card.Types, bt) {
				found[bt] = true
			}
		}
	}
	return len(found)
}

// ---------------------------------------------------------------------------
// Converge — count colors of mana spent to cast
// ---------------------------------------------------------------------------

// CountConverge returns the number of distinct colors of mana spent.
// Simplified: reads from the typed mana pool if available, otherwise
// estimates from the caster's lands.
func CountConverge(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]

	// If typed mana pool exists, count non-zero colors.
	if seat.Mana != nil {
		count := 0
		if seat.Mana.W > 0 {
			count++
		}
		if seat.Mana.U > 0 {
			count++
		}
		if seat.Mana.B > 0 {
			count++
		}
		if seat.Mana.R > 0 {
			count++
		}
		if seat.Mana.G > 0 {
			count++
		}
		return count
	}

	// Fallback: count distinct basic land types on battlefield (estimate).
	return CountDomain(gs, seatIdx)
}

// ---------------------------------------------------------------------------
// Devotion — count mana pips in costs of permanents you control
// ---------------------------------------------------------------------------

// CountDevotion counts mana symbols of `color` in the mana costs of
// permanents `seatIdx` controls. CR §700.5.
//
// Color is one of "W", "U", "B", "R", "G" (case-insensitive). The real
// rule counts MANA SYMBOLS in the mana COST of each permanent — not
// the color identity. We delegate per-card pip parsing to
// DevotionPipsFromManaCost(card.ManaCostString, color) when the card
// has a printed cost recorded; otherwise we fall back to the legacy
// "1 per color-matching permanent" heuristic so older fixtures /
// tokens (which never populate ManaCostString) still produce a
// reasonable count.
//
// Hybrid pips: per §700.5b a hybrid symbol counts toward the
// permanent's controller's devotion for BOTH halves' colors. E.g.,
// {B/R} on a permanent contributes +1 to devotion to black AND +1 to
// devotion to red. Twobrid {2/B} and Phyrexian {B/P} count as +1
// to the relevant color. Pure generic / X / snow / colorless never
// contribute.
//
// Returns 0 for invalid args.
func CountDevotion(gs *GameState, seatIdx int, color string) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || color == "" {
		return 0
	}
	wantColor := strings.ToUpper(color)
	total := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		// CR §718.3b — a prototyped permanent contributes its reduced colored
		// pips while on the battlefield (CountDevotion iterates the battlefield,
		// so the overlay is in force here; a *Permanent is battlefield by
		// definition). Off-battlefield zones use the printed cost (CR §718.4).
		mcs := p.Card.ManaCostString
		if p.Card.Proto != nil {
			mcs = p.Card.Proto.ManaCostString
		}
		if mcs != "" {
			total += DevotionPipsFromManaCost(mcs, wantColor)
			continue
		}
		// A token has no mana cost (CR §700.5 counts mana symbols in mana
		// COSTS), so a colored token contributes 0 to devotion even though
		// it carries a color — skip the Colors fallback for tokens. Without
		// this a black Zombie token would wrongly inflate devotion to black
		// (e.g. pumping Gray Merchant's drain in a Jarad tokens deck).
		if p.IsToken() {
			continue
		}
		// Legacy fallback: count 1 per color-matching permanent. Used only
		// by hand-built test fixtures that set Colors but no ManaCostString;
		// real non-token cards always carry a printed mana cost.
		for _, c := range p.Card.Colors {
			if strings.ToUpper(c) == wantColor {
				total++
			}
		}
	}
	return total
}

// ===========================================================================
// TRIGGER KEYWORDS
// ===========================================================================

// ---------------------------------------------------------------------------
// Landfall — triggers when a land ETBs under your control
// ---------------------------------------------------------------------------

// FireLandfallTriggers checks all permanents controlled by seatIdx for
// landfall triggers and fires them. Called from ETB processing when a
// land enters the battlefield.
func FireLandfallTriggers(gs *GameState, seatIdx int, land *Permanent) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.HasKeyword("landfall") || permHasTriggerEvent(p, "landfall") {
			gs.LogEvent(Event{
				Kind:   "landfall_trigger",
				Seat:   seatIdx,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"land": landName(land),
					"rule": "LF",
				},
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Constellation — triggers when an enchantment ETBs under your control
// ---------------------------------------------------------------------------

// constellationTriggerFromCard extracts the inner Triggered ability from a
// "Constellation — …" parse. Thor emits the ability word as a
// Static{Modification.ModKind=="ability_word", Args=["constellation",
// "triggered", *Triggered]} wrapper, so the effect lives one level down and
// neither the top-level *Triggered scans nor permHasTriggerEvent reach it.
// Returns nil for cards without the wrapper (e.g. a bare "constellation"
// keyword tag with no nested effect).
func constellationTriggerFromCard(card *Card) *gameast.Triggered {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "ability_word" {
			continue
		}
		args := st.Modification.Args
		if len(args) < 3 {
			continue
		}
		if w, _ := args[0].(string); w != "constellation" {
			continue
		}
		if t, ok := args[2].(*gameast.Triggered); ok {
			return t
		}
	}
	return nil
}

// FireConstellationTriggers fires constellation triggers when an
// enchantment enters the battlefield under seatIdx's control (CR §702.131 —
// "whenever this or another enchantment you control enters"). Called once
// per enchantment ETB from OnConstellationETB, AFTER the entering enchantment
// is already on the battlefield, so the controller's own newly-entered
// constellation source self-triggers (property b).
//
// Before r63 this only LOGGED a constellation_trigger event and never
// resolved the effect — and it didn't even detect the Static[ability_word]
// wrapper most real cards parse to (Eidolon of Blossoms, Doomwake Giant,
// Underworld Coinsmith, …), so generic-AST constellation was fully inert.
// Now it also RESOLVES the wrapped trigger's effect.
func FireConstellationTriggers(gs *GameState, seatIdx int, enchantment *Permanent) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return
	}
	// Snapshot — resolving effects (token mints, draws) may mutate the
	// battlefield slice mid-iteration.
	perms := make([]*Permanent, len(gs.Seats[seatIdx].Battlefield))
	copy(perms, gs.Seats[seatIdx].Battlefield)
	for _, p := range perms {
		if p == nil || p.Card == nil {
			continue
		}
		trig := constellationTriggerFromCard(p.Card)
		if trig == nil && !p.HasKeyword("constellation") && !permHasTriggerEvent(p, "constellation") {
			continue
		}
		// Diagnostic event (retained — pre-r63 behavior, exercised by tests).
		gs.LogEvent(Event{
			Kind:   "constellation_trigger",
			Seat:   seatIdx,
			Source: p.Card.DisplayName(),
			Details: map[string]interface{}{
				"enchantment": enchantmentName(enchantment),
				"rule":        "702.131",
			},
		})
		// Resolve the actual effect (r63 fix). Skip if there's no wrapped
		// effect to push (bare-keyword fixtures), or a per_card handler
		// already owns this card's enchantment-ETB (Calix registers an
		// OnTrigger("permanent_etb") that does its own constellation —
		// guarding here prevents a double-fire).
		if trig == nil || trig.Effect == nil {
			continue
		}
		if HasTriggerHook != nil && HasTriggerHook(p.Card.DisplayName(), "permanent_etb") {
			continue
		}
		n, cancelled := FireETBTriggerEvent(gs, p)
		if cancelled {
			continue
		}
		for i := 0; i < n; i++ {
			PushTriggeredAbilityWithIf(gs, p, trig.Effect, trig.InterveningIf)
			if gs.CheckEnd() {
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Heroic — triggers when you cast a spell targeting this creature
// ---------------------------------------------------------------------------

// FireHeroicTrigger fires the heroic trigger on a creature when it's
// targeted by a spell its controller cast.
func FireHeroicTrigger(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if !perm.HasKeyword("heroic") && !permHasTriggerEvent(perm, "heroic") {
		return
	}
	gs.LogEvent(Event{
		Kind:   "heroic_trigger",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "HRC",
		},
	})
}

// ---------------------------------------------------------------------------
// Evolve — CR §702.100
// ---------------------------------------------------------------------------
//
// "Whenever a creature enters the battlefield under your control, if that
// creature has greater power or toughness than this creature, put a +1/+1
// counter on this creature."
//
// Key rules:
//   - Triggered on YOUR creatures only (not opponents')
//   - Condition: entering creature's power > evolve creature's power
//     OR entering creature's toughness > evolve creature's toughness
//   - Effect: +1/+1 counter on the EVOLVE creature (not the entering one)
//   - Each evolve permanent triggers independently

// FireEvolveTriggers fires evolve triggers when a creature enters the
// battlefield under seatIdx's control.
func FireEvolveTriggers(gs *GameState, seatIdx int, newCreature *Permanent) {
	if gs == nil || newCreature == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if !newCreature.IsCreature() {
		return
	}
	newPow := newCreature.Power()
	newTough := newCreature.Toughness()

	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.Card == nil || p == newCreature {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if !p.HasKeyword("evolve") && !permHasTriggerEvent(p, "evolve") {
			continue
		}
		// Check if the entering creature has greater power OR greater
		// toughness than the evolve creature (CR §702.100b).
		evolvePow := p.Power()
		evolveTough := p.Toughness()
		if newPow > evolvePow || newTough > evolveTough {
			// Exactly ONE +1/+1 counter per qualifying ETB (not one per stat),
			// routed through the canonical doubler-aware chokepoint: +1/+1
			// doublers (Doubling Season, Hardened Scales, Branching Evolution)
			// apply and counter_placed fires (Conclave Mentor / Abzan Falconer).
			// Raw AddCounter bypassed both — the r63 counter-pipeline-sweep
			// meta-pattern. PutCountersTriggered invalidates the cache.
			PutCountersTriggered(gs, p, "+1/+1", 1, p)
			gs.LogEvent(Event{
				Kind:   "evolve_trigger",
				Seat:   seatIdx,
				Source: p.Card.DisplayName(),
				Amount: 1,
				Details: map[string]interface{}{
					"creature":         creatureName(newCreature),
					"new_power":        newPow,
					"new_toughness":    newTough,
					"evolve_power":     evolvePow,
					"evolve_toughness": evolveTough,
					"counter_kind":     "+1/+1",
					"rule":             "702.100",
				},
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Alliance — triggers when another creature ETBs under your control
// ---------------------------------------------------------------------------

// FireAllianceTriggers fires alliance triggers when a creature enters
// the battlefield under seatIdx's control.
func FireAllianceTriggers(gs *GameState, seatIdx int, newCreature *Permanent) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.Card == nil || p == newCreature {
			continue
		}
		if p.HasKeyword("alliance") || permHasTriggerEvent(p, "alliance") {
			gs.LogEvent(Event{
				Kind:   "alliance_trigger",
				Seat:   seatIdx,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"creature": creatureName(newCreature),
					"rule":     "ALC",
				},
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Magecraft (CR §702.137) — HasMagecraft + FireMagecraftTriggers live in
// keywords_magecraft.go where the mechanic is implemented with proper
// instant/sorcery filtering, own-spells-only scoping, isCopy parameter,
// and FireCardTrigger fan-out for per_card payoffs.
// ---------------------------------------------------------------------------

// ===========================================================================
// NEW MECHANICS — Blight / Exhaust
// ===========================================================================

// ---------------------------------------------------------------------------
// Blight — new mechanic (31 cards)
// ---------------------------------------------------------------------------
//
// "Blight X, Return this enchantment from your graveyard to the battlefield:
// Put X blight counters on target nonland permanent. It becomes a Swamp in
// addition to its other types."
//
// This is a graveyard-activated ability:
//  1. Cost: return the enchantment from graveyard to battlefield.
//  2. Effect: put X blight counters on target nonland permanent.
//  3. The blighted permanent becomes a Swamp (layer-4 type change),
//     which means it taps for {B} and loses its original mana abilities.
//
// ActivateBlight performs the blight activation for an enchantment card
// in the graveyard. Returns true if the activation succeeded.
func ActivateBlight(gs *GameState, seatIdx int, cardInGrave *Card, blightX int, target *Permanent) bool {
	if gs == nil || cardInGrave == nil || target == nil {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	// Verify the card is in the graveyard.
	foundIdx := -1
	for i, c := range seat.Graveyard {
		if c == cardInGrave {
			foundIdx = i
			break
		}
	}
	if foundIdx < 0 {
		return false
	}

	// Target must be a nonland permanent.
	if target.IsLand() {
		gs.LogEvent(Event{
			Kind:   "blight_illegal_target",
			Seat:   seatIdx,
			Source: cardInGrave.DisplayName(),
			Details: map[string]interface{}{
				"reason": "target_is_land",
			},
		})
		return false
	}

	// Cost: remove from graveyard, put onto battlefield.
	removeCardFromZone(gs, seatIdx, cardInGrave, "graveyard")
	EnsureBattlefieldFrontFace(cardInGrave)
	// CR 304.4 / 307.1 — non-permanent cards can't enter the battlefield.
	if !CardCanEnterBattlefield(cardInGrave) {
		gs.moveToZone(seatIdx, cardInGrave, "graveyard")
		return false
	}
	perm := &Permanent{
		Card:       cardInGrave,
		Controller: seatIdx,
		Owner:      cardInGrave.Owner,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind:   "blight_return",
		Seat:   seatIdx,
		Source: cardInGrave.DisplayName(),
		Details: map[string]interface{}{
			"from_zone": "graveyard",
			"to_zone":   "battlefield",
		},
	})

	// Effect: put X blight counters on target.
	target.AddCounter("blight", blightX)
	gs.InvalidateCharacteristicsCache()

	// The blighted permanent becomes a Swamp in addition to its other types
	// (layer 4 type change). Track via a flag so the layers system knows.
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["is_swamp"] = 1
	// Grant the Swamp subtype to the permanent's types if not already present.
	if !hasTypeInSlice(target.GrantedAbilities, "swamp") {
		target.GrantedAbilities = append(target.GrantedAbilities, "swamp")
	}

	gs.LogEvent(Event{
		Kind:   "blight_counters",
		Seat:   seatIdx,
		Source: cardInGrave.DisplayName(),
		Amount: blightX,
		Details: map[string]interface{}{
			"target":       target.Card.DisplayName(),
			"counters":     blightX,
			"type_added":   "swamp",
			"counter_kind": "blight",
		},
	})
	return true
}

// IsBlighted returns true if the permanent has any blight counters.
func IsBlighted(perm *Permanent) bool {
	if perm == nil || perm.Counters == nil {
		return false
	}
	return perm.Counters["blight"] > 0
}

// ---------------------------------------------------------------------------
// Exhaust — new mechanic (43 cards)
// ---------------------------------------------------------------------------
//
// "Exhaust — {cost}: [effect]. Activate each exhaust ability only once."
//
// An activated ability with a ONE-TIME-USE restriction. After activation,
// the ability is "exhausted" and can't be activated again for the rest of
// the game. Tracked via perm.Flags["exhaust_used_<ability_idx>"].
//
// The activation dispatcher must check IsExhausted before allowing the
// activation. After the ability resolves, MarkExhausted sets the flag.

// IsExhausted returns true if the exhaust ability at abilityIdx on perm
// has already been used this game.
func IsExhausted(perm *Permanent, abilityIdx int) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	key := "exhaust_used_" + itoa(abilityIdx)
	return perm.Flags[key] > 0
}

// MarkExhausted marks the exhaust ability at abilityIdx as used. Once
// set, IsExhausted returns true for the rest of the game.
func MarkExhausted(perm *Permanent, abilityIdx int) {
	if perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := "exhaust_used_" + itoa(abilityIdx)
	perm.Flags[key] = 1
}

// ActivateExhaust attempts to activate an exhaust ability on a permanent.
// Returns true if the activation succeeded. The caller is responsible for
// resolving the effect — this function handles cost payment and flag
// tracking only.
func ActivateExhaust(gs *GameState, seatIdx int, perm *Permanent, abilityIdx int) bool {
	if gs == nil || perm == nil {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}

	// Check if already exhausted.
	if IsExhausted(perm, abilityIdx) {
		gs.LogEvent(Event{
			Kind:   "exhaust_already_used",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"ability_idx": abilityIdx,
				"rule":        "exhaust",
			},
		})
		return false
	}

	// Mark as exhausted (permanent, game-long).
	MarkExhausted(perm, abilityIdx)

	gs.LogEvent(Event{
		Kind:   "exhaust_activated",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"ability_idx": abilityIdx,
			"rule":        "exhaust",
		},
	})
	return true
}

// IsExhaustAbility returns true if the activated ability at abilityIdx
// on perm is an exhaust ability. Detection: the AST TimingRestriction
// is "exhaust" or the ability has the "exhaust" keyword flag.
func IsExhaustAbility(perm *Permanent, abilityIdx int) bool {
	if perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return false
	}
	abilities := perm.Card.AST.Abilities
	if abilityIdx < 0 || abilityIdx >= len(abilities) {
		return false
	}
	ab, ok := abilities[abilityIdx].(*gameast.Activated)
	if !ok {
		return false
	}
	return strings.ToLower(ab.TimingRestriction) == "exhaust"
}

// ===========================================================================
// INTERNAL HELPERS
// ===========================================================================

// hasTypeInSlice checks if a type is present in a slice (case-insensitive).
func hasTypeInSlice(types []string, want string) bool {
	w := strings.ToLower(want)
	for _, t := range types {
		if strings.ToLower(t) == w {
			return true
		}
	}
	return false
}

// ===========================================================================
// §702.105 — Dethrone
// ===========================================================================

// FireDethroneTriggers checks each declared attacker for dethrone. If the
// attacker is attacking the player with the most life (or tied), put a
// +1/+1 counter on it. Called from fireAttackTriggers.
func FireDethroneTriggers(gs *GameState, attackerSeat int, attackers []*Permanent) {
	if gs == nil || len(attackers) == 0 {
		return
	}
	maxLife := math.MinInt32
	for _, s := range gs.Seats {
		if s != nil && !s.Lost && s.Life > maxLife {
			maxLife = s.Life
		}
	}
	for _, atk := range attackers {
		if atk == nil || !atk.HasKeyword("dethrone") {
			continue
		}
		defSeat := -1
		if atk.Flags != nil {
			defSeat = atk.Flags["defender_seat"]
		}
		if defSeat < 0 || defSeat >= len(gs.Seats) || gs.Seats[defSeat] == nil {
			continue
		}
		if gs.Seats[defSeat].Life >= maxLife {
			atk.AddCounter("+1/+1", 1)
			gs.LogEvent(Event{
				Kind: "dethrone_trigger", Seat: attackerSeat,
				Source: atk.Card.DisplayName(),
				Details: map[string]interface{}{
					"defender_seat": defSeat,
					"defender_life": gs.Seats[defSeat].Life,
					"rule":          "702.105",
				},
			})
		}
	}
}

// ===========================================================================
// §702.136 — Riot
// ===========================================================================

// ApplyRiot gives a creature entering the battlefield the riot choice:
// either a +1/+1 counter or haste. The AI always picks +1/+1 for
// creatures with power ≥ 3, haste otherwise.
func ApplyRiot(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || !perm.HasKeyword("riot") {
		return
	}
	chooseHaste := perm.Power() < 3
	if chooseHaste {
		perm.GrantedAbilities = append(perm.GrantedAbilities, "haste")
		perm.SummoningSick = false
		gs.LogEvent(Event{
			Kind: "riot_choice", Seat: perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"choice": "haste",
				"rule":   "702.136",
			},
		})
	} else {
		perm.AddCounter("+1/+1", 1)
		gs.LogEvent(Event{
			Kind: "riot_choice", Seat: perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"choice": "+1/+1 counter",
				"rule":   "702.136",
			},
		})
	}
}

// ===========================================================================
// §721 — Monarch
// ===========================================================================

// BecomeMonarch makes the given seat the monarch. Removes monarch from
// the previous holder. The monarch draws a card at end step (fired by
// FireMonarchEndStep). If dealt combat damage, attacker's controller
// becomes monarch (fired by CheckMonarchCombatSteal).
func BecomeMonarch(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	prev := gs.Flags["monarch_seat"]
	gs.Flags["monarch_seat"] = seatIdx
	gs.Flags["has_monarch"] = 1
	gs.LogEvent(Event{
		Kind: "become_monarch", Seat: seatIdx,
		Details: map[string]interface{}{
			"previous_monarch": prev,
			"rule":             "721.2",
		},
	})
}

// IsMonarch returns true if the seat is the current monarch.
func IsMonarch(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil || gs.Flags["has_monarch"] != 1 {
		return false
	}
	return gs.Flags["monarch_seat"] == seatIdx
}

// FireMonarchEndStep draws a card for the monarch at the beginning of THE
// MONARCH'S end step. CR §720.5 scopes the draw to the monarch's OWN end step,
// so it self-gates on gs.Active (the active player owns the current end step) —
// without this the monarch drew at every player's end step (4× per cycle in a
// 4-player game) instead of once per their own turn.
func FireMonarchEndStep(gs *GameState) {
	if gs == nil || gs.Flags == nil || gs.Flags["has_monarch"] != 1 {
		return
	}
	mSeat := gs.Flags["monarch_seat"]
	if mSeat < 0 || mSeat >= len(gs.Seats) {
		return
	}
	// Only on the monarch's own end step.
	if mSeat != gs.Active {
		return
	}
	s := gs.Seats[mSeat]
	if s == nil || s.Lost {
		return
	}
	if len(s.Library) > 0 {
		drawn := s.Library[0]
		MoveCard(gs, drawn, mSeat, "library", "hand", "monarch-draw")
		gs.LogEvent(Event{
			Kind: "monarch_draw", Seat: mSeat,
			Source:  drawn.DisplayName(),
			Details: map[string]interface{}{"rule": "721.3"},
		})
	}
}

// CheckMonarchCombatSteal transfers monarch to the attacker's controller
// if the monarch was dealt combat damage.
func CheckMonarchCombatSteal(gs *GameState, damagedSeat, attackerSeat int) {
	if gs == nil || gs.Flags == nil || gs.Flags["has_monarch"] != 1 {
		return
	}
	if gs.Flags["monarch_seat"] == damagedSeat && attackerSeat != damagedSeat {
		BecomeMonarch(gs, attackerSeat)
	}
}

// ===========================================================================
// §701.35 — Detain
// ===========================================================================

// ===========================================================================
// Party mechanic (Zendikar Rising)
// ===========================================================================

// CountParty counts the number of unique party roles among creatures
// controlled by the given seat. Party roles: Cleric, Rogue, Warrior, Wizard.
// A creature counts for at most one role. Full party = 4.
func CountParty(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	roles := map[string]bool{}
	partyTypes := []string{"cleric", "rogue", "warrior", "wizard"}
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p.Card == nil {
			continue
		}
		tl := strings.ToLower(strings.Join(p.Card.Types, " "))
		for _, role := range partyTypes {
			if !roles[role] && strings.Contains(tl, role) {
				roles[role] = true
				break
			}
		}
		if len(roles) == 4 {
			break
		}
	}
	return len(roles)
}

// ===========================================================================
// §702.167 — Craft with
// ===========================================================================

// removePermanentFromBattlefield removes a permanent without triggering
// dies/LTB (used for exile-and-return effects like Craft / Meld / Aura
// swap, and for activation-cost ExileSelf shapes like Channel).
//
// r60: also detaches any auras/equipment pointing at p. The carrier is
// leaving the battlefield, so by §704.5m/§704.5n attached objects would
// be cleaned up at the next SBA — but the window before SBA runs is
// observed by checkAttachmentConsistency (Loki r41/r57 cluster). Run
// detachAll synchronously to close the window.
func removePermanentFromBattlefield(gs *GameState, p *Permanent) {
	if gs == nil || p == nil {
		return
	}
	ctrl := p.Controller
	if ctrl < 0 || ctrl >= len(gs.Seats) || gs.Seats[ctrl] == nil {
		return
	}
	gs.removePermanent(p)
	gs.UnregisterReplacementsForPermanent(p)
	gs.UnregisterContinuousEffectsForPermanent(p)
	detachAll(gs, p)
	// Phase D — token cessation (CR §704.5d). A token leaving the
	// battlefield via this non-trigger path (Craft / Meld / Aura swap /
	// activation-cost ExileSelf) still ceases to exist. Clear the *Card's
	// InstanceID so any re-add via enterBattlefieldWithETB picks up a
	// fresh TK mint from EnsureTokenInstanceID.
	if p.Card != nil && p.IsToken() && p.Card.InstanceID != "" {
		MarkInstanceIDCeased(gs, p.Card.InstanceID)
		p.Card.InstanceID = ""
	}
}

// permHasTriggerEvent checks if a permanent's AST has a triggered ability
// matching the given event name.
func permHasTriggerEvent(p *Permanent, event string) bool {
	if p == nil || p.Card == nil || p.Card.AST == nil {
		return false
	}
	for _, ab := range p.Card.AST.Abilities {
		t, ok := ab.(*gameast.Triggered)
		if !ok {
			continue
		}
		if t.Trigger.Event != "" && EventEquals(t.Trigger.Event, event) {
			return true
		}
	}
	return false
}

// landName safely gets the display name of a land permanent.
func landName(p *Permanent) string {
	if p == nil || p.Card == nil {
		return "<unknown>"
	}
	return p.Card.DisplayName()
}

// enchantmentName safely gets the display name of an enchantment permanent.
func enchantmentName(p *Permanent) string {
	if p == nil || p.Card == nil {
		return "<unknown>"
	}
	return p.Card.DisplayName()
}

// creatureName safely gets the display name of a creature permanent.
func creatureName(p *Permanent) string {
	if p == nil || p.Card == nil {
		return "<unknown>"
	}
	return p.Card.DisplayName()
}

// spellName safely gets the display name of a spell card.
func spellName(c *Card) string {
	if c == nil {
		return "<unknown>"
	}
	return c.DisplayName()
}

// SacrificePermanent is a forwarding helper that calls the internal
// sacrificePermanentImpl. Avoids import cycles for keywords_misc.go.
// NOTE: This function is already defined in zone_change.go if it exists;
// we conditionally check and use it. If not, we define it here.
// The actual zone_change.go has SacrificePermanent — we reference it.

// Ensure we use the strings import.
var _ = strings.ToLower

// Ensure we use the gameast import.
var _ gameast.Ability

// ---------------------------------------------------------------------------
// Ascend / City's Blessing — CR §702.131
// ---------------------------------------------------------------------------

// CheckAscend checks if a player controls 10+ permanents and should
// receive the city's blessing. Once set, it's permanent for the rest
// of the game. Called after ETB events and SBAs.
func CheckAscend(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}
	// Already has the blessing — it's permanent
	if seat.Flags != nil && seat.Flags["citys_blessing"] > 0 {
		return
	}
	// Count permanents controlled
	count := len(seat.Battlefield)
	if count >= 10 {
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["citys_blessing"] = 1
		gs.LogEvent(Event{
			Kind:   "citys_blessing",
			Seat:   seatIdx,
			Amount: count,
			Details: map[string]interface{}{
				"rule":       "702.131",
				"permanents": count,
			},
		})
	}
}

// HasCitysBlessing returns true if the player has the city's blessing.
func HasCitysBlessing(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Flags == nil {
		return false
	}
	return seat.Flags["citys_blessing"] > 0
}
