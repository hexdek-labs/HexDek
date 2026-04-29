package gameengine

// Batch 5 — Remaining keyword actions (CR §701) + misc keyword abilities (CR §702).
//
// KEYWORD ACTIONS (§701):
//   - Fateseal N        — CR §701.29
//   - Clash             — CR §701.30
//   - Manifest          — CR §701.40
//   - Support N         — CR §701.41
//   - Meld              — CR §701.42
//   - Learn             — CR §701.48
//   - Collect Evidence  — CR §701.59
//   - Forage            — CR §701.61
//   - Manifest Dread    — CR §701.62
//   - Endure            — CR §701.63
//
// KEYWORD ABILITIES (§702):
//   - Forecast          — CR §702.57
//   - Transmute         — CR §702.53
//   - Dredge            — CR §702.52 (already exists; skipped)
//   - Rebound           — CR §702.88
//   - Fuse              — CR §702.102
//   - Aftermath         — CR §702.127
//   - Awaken            — CR §702.113
//   - Escalate          — CR §702.115
//   - More Than Meets the Eye — CR §702.162
//   - Living Metal      — CR §702.161

import "strings"

// ---------------------------------------------------------------------------
// §701.29 — Fateseal
// ---------------------------------------------------------------------------

// Fateseal looks at the top N cards of an opponent's library and puts any
// number on the bottom in any order. Simplified: the engine puts non-creature
// cards on the bottom as a heuristic (Hat override possible in the future).
func Fateseal(gs *GameState, seatIdx, targetSeat, n int) {
	if gs == nil || n <= 0 {
		return
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if targetSeat < 0 || targetSeat >= len(gs.Seats) {
		return
	}
	target := gs.Seats[targetSeat]
	if target == nil || len(target.Library) == 0 {
		return
	}

	count := n
	if count > len(target.Library) {
		count = len(target.Library)
	}

	looked := make([]*Card, count)
	copy(looked, target.Library[:count])

	// Simplified decision: put non-creature cards on the bottom.
	var keepTop, toBottom []*Card
	for _, c := range looked {
		if c != nil && !cardHasType(c, "creature") {
			toBottom = append(toBottom, c)
		} else {
			keepTop = append(keepTop, c)
		}
	}

	rest := make([]*Card, len(target.Library)-count)
	copy(rest, target.Library[count:])

	target.Library = append(keepTop, rest...)
	target.Library = append(target.Library, toBottom...)

	gs.LogEvent(Event{
		Kind:   "fateseal",
		Seat:   seatIdx,
		Target: targetSeat,
		Amount: count,
		Details: map[string]interface{}{
			"to_bottom": len(toBottom),
			"rule":      "701.29",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.30 — Clash
// ---------------------------------------------------------------------------

// Clash reveals the top card of each clashing player's library. The player
// whose revealed card has the higher mana value wins the clash. Returns true
// if seatIdx won. On a tie, neither player wins (returns false).
func Clash(gs *GameState, seatIdx, opponentSeat int) bool {
	if gs == nil {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if opponentSeat < 0 || opponentSeat >= len(gs.Seats) {
		return false
	}
	mySeat := gs.Seats[seatIdx]
	oppSeat := gs.Seats[opponentSeat]
	if mySeat == nil || oppSeat == nil {
		return false
	}

	myCMC := 0
	if len(mySeat.Library) > 0 && mySeat.Library[0] != nil {
		myCMC = mySeat.Library[0].CMC
	}
	oppCMC := 0
	if len(oppSeat.Library) > 0 && oppSeat.Library[0] != nil {
		oppCMC = oppSeat.Library[0].CMC
	}

	won := myCMC > oppCMC

	// After revealing, each player may put the card on top or bottom.
	// Simplified: leave on top (per §701.30d, the default).
	gs.LogEvent(Event{
		Kind:   "clash",
		Seat:   seatIdx,
		Target: opponentSeat,
		Details: map[string]interface{}{
			"my_cmc":  myCMC,
			"opp_cmc": oppCMC,
			"won":     won,
			"rule":    "701.30",
		},
	})

	return won
}

// ---------------------------------------------------------------------------
// §701.40 — Manifest
// ---------------------------------------------------------------------------

// ManifestTopCard puts the top card of the player's library onto the
// battlefield face down as a 2/2 creature. The actual card is tracked so
// it can be turned face up if it's a creature.
func ManifestTopCard(gs *GameState, seatIdx int) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Library) == 0 {
		return nil
	}

	top := seat.Library[0]
	seat.Library = seat.Library[1:]

	if top != nil {
		top.FaceDown = true
	}

	perm := &Permanent{
		Card: &Card{
			Name:          "Manifested Creature",
			Owner:         seatIdx,
			Types:         []string{"creature"},
			BasePower:     2,
			BaseToughness: 2,
			FaceDown:      true,
		},
		Controller:    seatIdx,
		Owner:         seatIdx,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{"manifested": 1},
	}

	// Store the real card reference so it can be turned face up later.
	// The Permanent.Card is the face-down shell; the actual card is
	// stored in the flag system indirectly. For engine purposes, we
	// keep the original card pointer accessible by re-attaching it on
	// face-up. The real card is parked on the perm via a secondary ref.
	if top != nil {
		perm.Flags["manifest_real_card_exists"] = 1
		// Flag whether the original card is a creature, enabling the
		// engine to check if this manifest can be turned face-up by
		// paying its mana cost (CR §701.40e).
		if cardHasType(top, "creature") {
			perm.Flags["manifest_is_creature"] = 1
		}
		// We use the BackFaceAST slot to hold the real card's AST when
		// available, providing a path for face-up logic.
		if top.AST != nil {
			perm.BackFaceAST = top.AST
		}
	}

	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind:   "manifest",
		Seat:   seatIdx,
		Source: "Manifested Creature",
		Details: map[string]interface{}{
			"face_down": true,
			"power":     2,
			"toughness": 2,
			"rule":      "701.40",
		},
	})

	return perm
}

// ---------------------------------------------------------------------------
// §701.41 — Support
// ---------------------------------------------------------------------------

// Support puts a +1/+1 counter on each of up to N target creatures you
// control. Simplified: distributes one counter each to up to N creatures,
// preferring those with the fewest counters.
func Support(gs *GameState, seatIdx, n int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || n <= 0 {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Gather creatures.
	var creatures []*Permanent
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			creatures = append(creatures, p)
		}
	}
	if len(creatures) == 0 {
		return
	}

	count := n
	if count > len(creatures) {
		count = len(creatures)
	}

	// Simplified: pick the first N creatures (stable ordering).
	for i := 0; i < count; i++ {
		creatures[i].AddCounter("+1/+1", 1)
	}

	gs.LogEvent(Event{
		Kind:   "support",
		Seat:   seatIdx,
		Amount: count,
		Details: map[string]interface{}{
			"n":    n,
			"rule": "701.41",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.42 — Meld
// ---------------------------------------------------------------------------

// Meld combines two permanents into a single melded permanent. The two
// components are exiled and a new combined permanent enters the battlefield.
// Returns the melded permanent, or nil on failure.
func Meld(gs *GameState, perm1, perm2 *Permanent) *Permanent {
	if gs == nil || perm1 == nil || perm2 == nil {
		return nil
	}
	if perm1.Card == nil || perm2.Card == nil {
		return nil
	}

	seatIdx := perm1.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}

	name1 := perm1.Card.DisplayName()
	name2 := perm2.Card.DisplayName()

	// Remove both permanents from battlefield.
	removePermanentFromBattlefield(gs, perm1)
	removePermanentFromBattlefield(gs, perm2)

	// Exile both cards (already removed from battlefield above).
	if perm1.Card != nil {
		FireZoneChange(gs, perm1, perm1.Card, seatIdx, "battlefield", "exile")
		FireZoneChangeTriggers(gs, perm1, perm1.Card, "battlefield", "exile")
	}
	if perm2.Card != nil {
		FireZoneChange(gs, perm2, perm2.Card, seatIdx, "battlefield", "exile")
		FireZoneChangeTriggers(gs, perm2, perm2.Card, "battlefield", "exile")
	}

	// Create the melded creature. The actual melded card's stats come from
	// the card data; as a simplified default, sum power/toughness.
	meldedPower := 0
	meldedToughness := 0
	if perm1.Card != nil {
		meldedPower += perm1.Card.BasePower
		meldedToughness += perm1.Card.BaseToughness
	}
	if perm2.Card != nil {
		meldedPower += perm2.Card.BasePower
		meldedToughness += perm2.Card.BaseToughness
	}

	meldedName := name1 + " // " + name2
	melded := CreateCreatureToken(gs, seatIdx, meldedName,
		[]string{"creature"}, meldedPower, meldedToughness)
	if melded != nil {
		if melded.Flags == nil {
			melded.Flags = map[string]int{}
		}
		melded.Flags["melded"] = 1
	}

	gs.LogEvent(Event{
		Kind:   "meld",
		Seat:   seatIdx,
		Source: meldedName,
		Details: map[string]interface{}{
			"component_1": name1,
			"component_2": name2,
			"rule":        "701.42",
		},
	})

	return melded
}

// ---------------------------------------------------------------------------
// §701.48 — Learn
// ---------------------------------------------------------------------------

// Learn lets a player either discard a card to draw a card, or (if a
// sideboard were available) tutor a Lesson card from outside the game.
// Since the engine has no Sideboard zone, this always does discard-to-draw
// when the hand is non-empty.
func Learn(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Option A: Discard a card, then draw a card.
	if len(seat.Hand) > 0 {
		// Discard last card in hand (simplified decision).
		discarded := seat.Hand[len(seat.Hand)-1]
		DiscardCard(gs, discarded, seatIdx)

		discardedName := "<nil>"
		if discarded != nil {
			discardedName = discarded.DisplayName()
		}

		// Draw one.
		drawn, ok := gs.drawOne(seatIdx)
		drawnName := "<nil>"
		if ok && drawn != nil {
			drawnName = drawn.DisplayName()
		}

		gs.LogEvent(Event{
			Kind: "learn",
			Seat: seatIdx,
			Details: map[string]interface{}{
				"mode":      "discard_draw",
				"discarded": discardedName,
				"drawn":     drawnName,
				"rule":      "701.48",
			},
		})
		return
	}

	// Hand is empty — nothing to discard, and no sideboard to tutor from.
	gs.LogEvent(Event{
		Kind: "learn",
		Seat: seatIdx,
		Details: map[string]interface{}{
			"mode": "no_action",
			"rule": "701.48",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.59 — Collect Evidence
// ---------------------------------------------------------------------------

// CollectEvidence exiles cards from the player's graveyard whose total mana
// values sum to at least N. Returns true if evidence was successfully
// collected. Simplified: greedily exiles cards from the graveyard starting
// with the highest CMC until the threshold is met.
func CollectEvidence(gs *GameState, seatIdx, n int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || n <= 0 {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Graveyard) == 0 {
		return false
	}

	// Check if total CMC in graveyard is enough.
	totalAvailable := 0
	for _, c := range seat.Graveyard {
		if c != nil {
			totalAvailable += c.CMC
		}
	}
	if totalAvailable < n {
		return false
	}

	// Greedy: exile cards from back to front until threshold is met.
	collected := 0
	var exiled []*Card
	var remaining []*Card
	for i := len(seat.Graveyard) - 1; i >= 0 && collected < n; i-- {
		c := seat.Graveyard[i]
		if c == nil {
			remaining = append(remaining, c)
			continue
		}
		exiled = append(exiled, c)
		collected += c.CMC
	}
	// Keep cards we didn't exile.
	for i := 0; i < len(seat.Graveyard); i++ {
		c := seat.Graveyard[i]
		isExiled := false
		for _, e := range exiled {
			if e == c {
				isExiled = true
				break
			}
		}
		if !isExiled {
			remaining = append(remaining, c)
		}
	}

	seat.Graveyard = remaining
	for _, c := range exiled {
		MoveCard(gs, c, seatIdx, "graveyard", "exile", "exile-from-graveyard")
	}

	gs.LogEvent(Event{
		Kind:   "collect_evidence",
		Seat:   seatIdx,
		Amount: collected,
		Details: map[string]interface{}{
			"threshold":    n,
			"cards_exiled": len(exiled),
			"rule":         "701.59",
		},
	})

	return true
}

// ---------------------------------------------------------------------------
// §701.61 — Forage
// ---------------------------------------------------------------------------

// Forage performs the forage action: exile three cards from your graveyard,
// or sacrifice a Food you control. Returns true if successful.
//
// AI choice heuristic per CR §701.61: the player CHOOSES between the two
// options. We prefer sacrificing Food if available (Food tokens are generally
// less valuable than graveyard cards for recursion-heavy decks), unless the
// graveyard has 10+ cards (then exile to thin it).
func Forage(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}

	// Evaluate options.
	var foodPerm *Permanent
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil && cardHasSubtype(p.Card, "food") {
			foodPerm = p
			break
		}
	}
	hasFood := foodPerm != nil
	canExile := len(seat.Graveyard) >= 3

	if !hasFood && !canExile {
		return false
	}

	// AI choice: prefer Food sacrifice unless graveyard is huge (10+),
	// in which case exile to thin it. If Food unavailable, must exile.
	chooseFoodSac := hasFood && (len(seat.Graveyard) < 10 || !canExile)

	if chooseFoodSac {
		// Sacrifice a Food.
		name := foodPerm.Card.DisplayName()
		SacrificePermanent(gs, foodPerm, "forage")
		gs.LogEvent(Event{
			Kind: "forage",
			Seat: seatIdx,
			Details: map[string]interface{}{
				"mode":       "sacrifice_food",
				"choice":     "sacrifice_food",
				"sacrificed": name,
				"rule":       "701.61",
			},
		})
		return true
	}

	// Exile three cards from graveyard.
	exiled := append([]*Card(nil), seat.Graveyard[len(seat.Graveyard)-3:]...)
	seat.Graveyard = seat.Graveyard[:len(seat.Graveyard)-3]
	for _, c := range exiled {
		MoveCard(gs, c, seatIdx, "graveyard", "exile", "exile-from-graveyard")
	}

	gs.LogEvent(Event{
		Kind: "forage",
		Seat: seatIdx,
		Details: map[string]interface{}{
			"mode":         "exile_graveyard",
			"choice":       "exile_graveyard",
			"cards_exiled": 3,
			"rule":         "701.61",
		},
	})

	return true
}

// ---------------------------------------------------------------------------
// §701.62 — Manifest Dread
// ---------------------------------------------------------------------------

// ManifestDread looks at the top two cards of the player's library, manifests
// one face down as a 2/2 creature, and puts the other into the graveyard.
// Simplified: manifests the first creature found (or the first card), puts
// the other in the graveyard.
func ManifestDread(gs *GameState, seatIdx int) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Library) == 0 {
		return nil
	}

	count := 2
	if count > len(seat.Library) {
		count = len(seat.Library)
	}

	looked := make([]*Card, count)
	copy(looked, seat.Library[:count])
	seat.Library = seat.Library[count:]

	// Choose which to manifest: prefer creatures (they can be turned face up).
	manifestIdx := 0
	for i, c := range looked {
		if c != nil && cardHasType(c, "creature") {
			manifestIdx = i
			break
		}
	}

	// Put the non-chosen card(s) into the graveyard.
	for i, c := range looked {
		if i != manifestIdx {
			MoveCard(gs, c, seatIdx, "library", "graveyard", "manifest-dive")
		}
	}

	// Manifest the chosen card face-down as a 2/2.
	chosen := looked[manifestIdx]
	if chosen != nil {
		chosen.FaceDown = true
	}

	perm := &Permanent{
		Card: &Card{
			Name:          "Manifested Creature",
			Owner:         seatIdx,
			Types:         []string{"creature"},
			BasePower:     2,
			BaseToughness: 2,
			FaceDown:      true,
		},
		Controller:    seatIdx,
		Owner:         seatIdx,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{"manifested": 1, "manifest_dread": 1},
	}
	if chosen != nil && chosen.AST != nil {
		perm.BackFaceAST = chosen.AST
	}

	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)

	gs.LogEvent(Event{
		Kind:   "manifest_dread",
		Seat:   seatIdx,
		Source: "Manifested Creature",
		Details: map[string]interface{}{
			"looked":    count,
			"face_down": true,
			"rule":      "701.62",
		},
	})

	return perm
}

// ---------------------------------------------------------------------------
// §701.63 — Endure
// ---------------------------------------------------------------------------

// Endure causes a creature to gain indestructible until end of turn and
// become tapped. Per CR §701.63, this is a keyword action that stabilizes
// a creature that would otherwise be destroyed.
func Endure(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}

	// Grant indestructible until end of turn.
	perm.GrantedAbilities = append(perm.GrantedAbilities, "indestructible")

	// Tap the creature.
	perm.Tapped = true

	source := "<nil>"
	if perm.Card != nil {
		source = perm.Card.DisplayName()
	}

	gs.LogEvent(Event{
		Kind:   "endure",
		Seat:   perm.Controller,
		Source: source,
		Details: map[string]interface{}{
			"indestructible": true,
			"tapped":         true,
			"rule":           "701.63",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.57 — Forecast
// ---------------------------------------------------------------------------

// ActivateForecast activates the forecast ability of a card in hand. Per
// §702.57, forecast can only be activated during the controller's upkeep
// by revealing the card from hand. Returns true if the forecast was
// successfully activated.
func ActivateForecast(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}

	// Must be during controller's upkeep.
	if gs.Active != seatIdx || gs.Step != "upkeep" {
		gs.LogEvent(Event{
			Kind: "forecast_failed",
			Seat: seatIdx,
			Details: map[string]interface{}{
				"reason": "not_upkeep",
				"rule":   "702.57",
			},
		})
		return false
	}

	// Card must be in hand.
	inHand := false
	for _, c := range seat.Hand {
		if c == card {
			inHand = true
			break
		}
	}
	if !inHand {
		return false
	}

	// Card must have the forecast keyword.
	if !cardHasKeyword(card, "forecast") {
		return false
	}

	// Reveal and activate. The card stays in hand (forecast only reveals).
	gs.LogEvent(Event{
		Kind:   "forecast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"revealed": true,
			"rule":     "702.57",
		},
	})

	return true
}

// ---------------------------------------------------------------------------
// §702.53 — Transmute
// ---------------------------------------------------------------------------

// ActivateTransmute discards a card with transmute and searches the library
// for a card with the same mana value. Simplified: finds the first card in
// the library matching the CMC and puts it into hand.
func ActivateTransmute(gs *GameState, seatIdx int, card *Card) {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
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
		return
	}

	targetCMC := card.CMC

	// Discard the transmute card.
	DiscardCard(gs, card, seatIdx)

	// Search library for a card with the same CMC.
	foundIdx := -1
	for i, c := range seat.Library {
		if c != nil && c.CMC == targetCMC {
			foundIdx = i
			break
		}
	}

	foundName := "<none>"
	if foundIdx >= 0 {
		found := seat.Library[foundIdx]
		MoveCard(gs, found, seatIdx, "library", "hand", "tutor-to-hand")
		if found != nil {
			foundName = found.DisplayName()
		}
	}

	// Shuffle library after searching.
	if gs.Rng != nil && len(seat.Library) > 1 {
		gs.Rng.Shuffle(len(seat.Library), func(i, j int) {
			seat.Library[i], seat.Library[j] = seat.Library[j], seat.Library[i]
		})
	}

	gs.LogEvent(Event{
		Kind:   "transmute",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"target_cmc": targetCMC,
			"found":      foundName,
			"rule":       "702.53",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.88 — Rebound
// ---------------------------------------------------------------------------

// ApplyRebound handles rebound resolution for a spell on the stack. If the
// spell was cast from hand, it is exiled instead of going to the graveyard,
// and a delayed trigger is registered to cast it again on the next upkeep.
func ApplyRebound(gs *GameState, item *StackItem) {
	if gs == nil || item == nil || item.Card == nil {
		return
	}

	// Rebound only applies to spells cast from hand.
	if item.CastZone != "" && item.CastZone != "hand" {
		return
	}

	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Exile the card instead of putting it in the graveyard.
	MoveCard(gs, item.Card, seatIdx, "stack", "exile", "replace-to-exile")

	gs.LogEvent(Event{
		Kind:   "rebound_exile",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.88",
		},
	})

	// Register a delayed trigger to cast again at next upkeep.
	reboundCard := item.Card
	reboundEffect := item.Effect
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "next_upkeep",
		ControllerSeat: seatIdx,
		SourceCardName: reboundCard.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *GameState) {
			if seatIdx < 0 || seatIdx >= len(gs.Seats) {
				return
			}
			s := gs.Seats[seatIdx]
			if s == nil {
				return
			}
			// Remove from exile.
			removeCardFromZone(gs, seatIdx, reboundCard, "exile")
			// Push the spell onto the stack as a free cast (§702.88b).
			reboundItem := &StackItem{
				Card:       reboundCard,
				Controller: seatIdx,
				Effect:     reboundEffect,
				CostMeta:   map[string]interface{}{"free_cast": true},
			}
			gs.Stack = append(gs.Stack, reboundItem)
			gs.LogEvent(Event{
				Kind:   "rebound_cast",
				Seat:   seatIdx,
				Source: reboundCard.DisplayName(),
				Details: map[string]interface{}{
					"rule": "702.88b",
				},
			})
		},
	})
}

// ---------------------------------------------------------------------------
// §702.102 — Fuse
// ---------------------------------------------------------------------------

// IsFused returns true if the given stack item represents a fused split card
// (both halves being cast together). The engine marks fused spells via the
// CostMeta "fused" key.
func IsFused(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["fused"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// ---------------------------------------------------------------------------
// §702.127 — Aftermath
// ---------------------------------------------------------------------------

// CanCastAftermath returns true if the card can be cast from the graveyard
// using its aftermath ability. Per §702.127, aftermath cards can only be
// cast from the graveyard (the second half).
func CanCastAftermath(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}

	// Card must be in graveyard.
	inGraveyard := false
	for _, c := range seat.Graveyard {
		if c == card {
			inGraveyard = true
			break
		}
	}
	if !inGraveyard {
		return false
	}

	// Card must have the aftermath keyword.
	if !cardHasKeyword(card, "aftermath") {
		return false
	}

	gs.LogEvent(Event{
		Kind:   "aftermath_check",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"can_cast": true,
			"rule":     "702.127",
		},
	})

	return true
}

// ---------------------------------------------------------------------------
// §702.113 — Awaken
// ---------------------------------------------------------------------------

// ApplyAwaken puts N +1/+1 counters on a target land and turns it into a
// 0/0 Elemental creature in addition to its other types. This is typically
// used as an alternative cost for instants/sorceries with awaken.
func ApplyAwaken(gs *GameState, seatIdx int, land *Permanent, n int) {
	if gs == nil || land == nil || n <= 0 {
		return
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Put +1/+1 counters on the land.
	land.AddCounter("+1/+1", n)

	// Make it a creature — add creature and Elemental types.
	if land.Card != nil {
		hasCreature := false
		for _, t := range land.Card.Types {
			if strings.EqualFold(t, "creature") {
				hasCreature = true
				break
			}
		}
		if !hasCreature {
			land.Card.Types = append(land.Card.Types, "creature", "elemental")
		}
		// Set base P/T to 0/0 (counters provide the actual stats).
		land.Card.BasePower = 0
		land.Card.BaseToughness = 0
	}

	source := "<nil>"
	if land.Card != nil {
		source = land.Card.DisplayName()
	}

	gs.LogEvent(Event{
		Kind:   "awaken",
		Seat:   seatIdx,
		Source: source,
		Amount: n,
		Details: map[string]interface{}{
			"counters": n,
			"rule":     "702.113",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.115 — Escalate
// ---------------------------------------------------------------------------

// CalculateEscalateCost computes the total cost for a spell with escalate
// given the number of modes chosen and the per-mode escalate cost. The first
// mode is free; each additional mode costs baseCost more.
func CalculateEscalateCost(modesChosen int, baseCost int) int {
	if modesChosen <= 1 || baseCost <= 0 {
		return 0
	}
	return (modesChosen - 1) * baseCost
}

// ---------------------------------------------------------------------------
// §702.162 — More Than Meets the Eye
// ---------------------------------------------------------------------------

// ApplyMoreThanMeetsTheEye applies the "More Than Meets the Eye" mechanic:
// the permanent enters the battlefield transformed (back face up). This is
// used for Transformers-series cards that can be cast for an alternative
// cost and enter already flipped.
func ApplyMoreThanMeetsTheEye(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}

	perm.Transformed = true

	// If back face AST exists, swap the card's AST to the back face.
	if perm.BackFaceAST != nil && perm.Card != nil {
		perm.FrontFaceAST = perm.Card.AST
		perm.Card.AST = perm.BackFaceAST
		if perm.BackFaceName != "" {
			perm.FrontFaceName = perm.Card.Name
			perm.Card.Name = perm.BackFaceName
		}
	}

	source := "<nil>"
	if perm.Card != nil {
		source = perm.Card.DisplayName()
	}

	gs.LogEvent(Event{
		Kind:   "more_than_meets_the_eye",
		Seat:   perm.Controller,
		Source: source,
		Details: map[string]interface{}{
			"transformed": true,
			"rule":        "702.162",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.161 — Living Metal
// ---------------------------------------------------------------------------

// CheckLivingMetal returns true if a permanent with Living Metal should
// currently be a creature. Per §702.161, a Vehicle with Living Metal is
// also a creature during its controller's turn.
func CheckLivingMetal(gs *GameState, perm *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}

	// Living Metal only makes the permanent a creature during its
	// controller's turn.
	isControllersTurn := gs.Active == perm.Controller

	if isControllersTurn {
		// Ensure it has creature type.
		if perm.Card != nil {
			hasCreature := false
			for _, t := range perm.Card.Types {
				if strings.EqualFold(t, "creature") {
					hasCreature = true
					break
				}
			}
			if !hasCreature {
				perm.Card.Types = append(perm.Card.Types, "creature")
			}
		}
		gs.LogEvent(Event{
			Kind:   "living_metal_active",
			Seat:   perm.Controller,
			Source: func() string {
				if perm.Card != nil {
					return perm.Card.DisplayName()
				}
				return "<nil>"
			}(),
			Details: map[string]interface{}{
				"is_creature": true,
				"rule":        "702.161",
			},
		})
	}

	return isControllersTurn
}
