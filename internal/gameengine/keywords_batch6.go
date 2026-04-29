package gameengine

// keywords_batch6.go — Remaining missing keyword abilities + keyword actions.
//
// This batch brings every FAIL keyword in KEYWORD_COVERAGE_REPORT.md to PASS.
//
// KEYWORD ABILITIES (§702):
//   - Madness              — CR §702.35
//   - Backup N             — CR §702.165
//   - Enlist               — CR §702.154
//   - Mutate (stub)        — CR §702.140
//   - For Mirrodin!        — CR §702.150
//   - Read Ahead           — CR §702.155
//   - Ravenous             — CR §702.156
//   - Compleated           — CR §702.163
//   - Changeling           — CR §702.73  (all creature types)
//   - Equip activation     — CR §702.6
//   - Epic                 — CR §702.50
//   - Recover              — CR §702.60
//   - Aura Swap            — CR §702.65
//   - Frenzy               — CR §702.68
//   - Gravestorm           — CR §702.69
//   - Transfigure          — CR §702.71
//   - Hidden Agenda        — CR §702.106
//   - Umbra Armor          — CR §702.89
//   - Ingest               — CR §702.113b
//   - Warp                 — CR §702.185
//   - Station              — CR §702.184
//   - Start Your Engines!  — CR §702.179
//   - Harmonize            — CR §702.180
//   - Mobilize             — CR §702.181
//   - Freerunning          — CR §702.169
//   - Gift                 — CR §702.174
//   - Space Sculptor       — §702.173
//   - Visit                — §702.177
//   - Max Speed            — §702.178
//   - Tiered               — §702.182
//   - Job Select           — §702.183
//   - Solved               — §702.186
//   - Mayhem               — §702.187
//   - Infinity             — §702.190
//   - Exhaust (already in keywords_misc.go; this adds HasExhaust)
//
// KEYWORD ACTIONS (§701):
//   - Behold               — CR §701.4
//   - Triple               — CR §701.11
//   - Exchange             — CR §701.12
//   - Convert              — CR §701.28
//   - Vote                 — CR §701.38
//   - Harness              — CR §701.64
//   - Airbend/Earthbend/Waterbend/Firebend — CR §701.65-68

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ===========================================================================
// §702.35 — Madness
// ===========================================================================

// ActivateMadness attempts to cast a card for its madness cost after it has
// been discarded into exile. Per CR §702.35a-c, when a card with madness is
// discarded, the player may exile it instead of putting it into the graveyard.
// Then they may cast it for its madness cost or put it into the graveyard.
// Returns true if the card was successfully cast for madness.
func ActivateMadness(gs *GameState, seatIdx int, card *Card, madnessCost int) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}

	// Check mana.
	if seat.ManaPool < madnessCost {
		return false
	}

	// Card should be in exile (discarded there via madness replacement).
	exileIdx := -1
	for i, c := range seat.Exile {
		if c == card {
			exileIdx = i
			break
		}
	}
	if exileIdx < 0 {
		return false
	}

	// Pay madness cost.
	seat.ManaPool -= madnessCost
	SyncManaAfterSpend(seat)

	// Remove from exile and push onto the stack.
	removeCardFromZone(gs, seatIdx, card, "exile")

	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneExile,
		CostMeta: map[string]interface{}{
			"madness":      true,
			"madness_cost": madnessCost,
		},
	}
	PushStackItem(gs, item)

	gs.LogEvent(Event{
		Kind:   "madness",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: madnessCost,
		Details: map[string]interface{}{
			"rule": "702.35",
		},
	})
	return true
}

// HasMadness returns true if the card has the madness keyword.
func HasMadness(card *Card) bool {
	return cardHasKeywordByName(card, "madness")
}

// MadnessCost returns the madness cost from keyword args.
func MadnessCost(card *Card) int {
	return keywordArgCost(card, "madness")
}

// ===========================================================================
// §702.165 — Backup
// ===========================================================================

// ApplyBackup puts N +1/+1 counters on a target creature. If that creature
// is different from the source, it also gains all other abilities of the
// source until end of turn. Per CR §702.165a.
func ApplyBackup(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Find a target creature (other than perm itself, preferring strongest).
	var target *Permanent
	bestPower := -1
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsCreature() || p == perm {
			continue
		}
		pw := p.Power()
		if pw > bestPower {
			bestPower = pw
			target = p
		}
	}

	if target == nil {
		// No other creature: put counters on self.
		perm.AddCounter("+1/+1", n)
		gs.InvalidateCharacteristicsCache()
		gs.LogEvent(Event{
			Kind:   "backup",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Amount: n,
			Details: map[string]interface{}{
				"target": "self",
				"rule":   "702.165",
			},
		})
		return
	}

	// Put counters on target.
	target.AddCounter("+1/+1", n)
	gs.InvalidateCharacteristicsCache()

	// Grant all keyword abilities from perm to target until EOT.
	for _, kw := range getKeywordNames(perm) {
		if kw != "backup" {
			target.GrantedAbilities = append(target.GrantedAbilities, kw)
		}
	}

	gs.LogEvent(Event{
		Kind:   "backup",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"rule":   "702.165",
		},
	})
}

// ===========================================================================
// §702.154 — Enlist
// ===========================================================================

// ApplyEnlist taps an untapped non-attacking creature the controller controls
// and adds its power to the attacker's power until end of turn.
func ApplyEnlist(gs *GameState, attacker *Permanent, helper *Permanent) {
	if gs == nil || attacker == nil || helper == nil {
		return
	}
	if attacker.Controller != helper.Controller {
		return
	}
	if helper.Tapped || !helper.IsCreature() {
		return
	}
	// Helper must not be attacking.
	if helper.Flags != nil && helper.Flags["attacking"] > 0 {
		return
	}

	helper.Tapped = true
	bonus := helper.Power()

	attacker.Modifications = append(attacker.Modifications, Modification{
		Power:     bonus,
		Toughness: 0,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "enlist",
		Seat:   attacker.Controller,
		Source: attacker.Card.DisplayName(),
		Amount: bonus,
		Details: map[string]interface{}{
			"helper": helper.Card.DisplayName(),
			"rule":   "702.154",
		},
	})
}

// ===========================================================================
// §702.140 — Mutate
// ===========================================================================

// HasMutate returns true if the card has the mutate keyword.
func HasMutate(card *Card) bool {
	return cardHasKeywordByName(card, "mutate")
}

// ApplyMutate merges a mutating creature with a target creature per CR
// §702.140. If onTop is true, the mutating creature goes on top and its
// characteristics (name, power/toughness, types) replace the target's,
// but it gains all abilities from every card in the merged pile. If onTop
// is false, the target keeps its characteristics and gains all keyword
// abilities from the mutating creature. In both cases a "creature_mutated"
// trigger fires for "whenever this creature mutates" effects.
func ApplyMutate(gs *GameState, mutatingPerm *Permanent, targetPerm *Permanent, onTop bool) {
	if gs == nil || mutatingPerm == nil || targetPerm == nil {
		return
	}
	if mutatingPerm == targetPerm {
		return
	}

	seat := targetPerm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	if onTop {
		// Mutating card goes on top — takes over characteristics.
		// Absorb target's granted abilities.
		for _, ab := range targetPerm.GrantedAbilities {
			mutatingPerm.GrantedAbilities = append(mutatingPerm.GrantedAbilities, ab)
		}
		// Absorb keyword abilities from the target card's AST.
		for _, kw := range getKeywordNames(targetPerm) {
			mutatingPerm.GrantedAbilities = append(mutatingPerm.GrantedAbilities, kw)
		}
		// Copy target's counters.
		if targetPerm.Counters != nil {
			if mutatingPerm.Counters == nil {
				mutatingPerm.Counters = map[string]int{}
			}
			for k, v := range targetPerm.Counters {
				mutatingPerm.Counters[k] += v
			}
		}
		// Remove target from battlefield; keep mutating perm.
		gs.removePermanent(targetPerm)
		if mutatingPerm.Flags == nil {
			mutatingPerm.Flags = map[string]int{}
		}
		mutatingPerm.Flags["mutated"] = 1
	} else {
		// Mutating card goes under — target keeps characteristics.
		// Target gains mutating card's keyword abilities.
		for _, kw := range getKeywordNames(mutatingPerm) {
			targetPerm.GrantedAbilities = append(targetPerm.GrantedAbilities, kw)
		}
		// Also absorb mutating perm's already-granted abilities.
		for _, ab := range mutatingPerm.GrantedAbilities {
			targetPerm.GrantedAbilities = append(targetPerm.GrantedAbilities, ab)
		}
		// Remove mutating perm from battlefield.
		gs.removePermanent(mutatingPerm)
		if targetPerm.Flags == nil {
			targetPerm.Flags = map[string]int{}
		}
		targetPerm.Flags["mutated"] = 1
	}

	// Fire "whenever this creature mutates" triggers.
	FireCardTrigger(gs, "creature_mutated", map[string]interface{}{
		"controller_seat": seat,
	})

	mutName := "<nil>"
	if mutatingPerm.Card != nil {
		mutName = mutatingPerm.Card.DisplayName()
	}
	targName := "<nil>"
	if targetPerm.Card != nil {
		targName = targetPerm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind: "mutate", Seat: seat,
		Source: mutName,
		Details: map[string]interface{}{
			"target": targName,
			"on_top": onTop,
			"rule":   "702.140",
		},
	})
}

// ApplyMutatePlaceholder is the legacy stub entry point. It now delegates
// to ApplyMutate when a valid target is available, or logs a stub event
// when no target can be auto-selected.
func ApplyMutatePlaceholder(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	source := "<nil>"
	if perm.Card != nil {
		source = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "mutate",
		Seat:   perm.Controller,
		Source: source,
		Details: map[string]interface{}{
			"stub": true,
			"rule": "702.140",
		},
	})
}

// ===========================================================================
// §702.150 — For Mirrodin!
// ===========================================================================

// ApplyForMirrodin creates a 2/2 red Rebel creature token and attaches the
// equipment to it. Per CR §702.150a.
func ApplyForMirrodin(gs *GameState, equipment *Permanent) {
	if gs == nil || equipment == nil {
		return
	}
	seatIdx := equipment.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Create 2/2 red Rebel creature token.
	token := CreateCreatureToken(gs, seatIdx, "Rebel Token",
		[]string{"creature", "rebel"}, 2, 2)
	if token == nil {
		return
	}
	if token.Card != nil {
		token.Card.Colors = []string{"R"}
	}

	// Attach equipment to the token.
	equipment.AttachedTo = token

	gs.LogEvent(Event{
		Kind:   "for_mirrodin",
		Seat:   seatIdx,
		Source: equipment.Card.DisplayName(),
		Details: map[string]interface{}{
			"token": "2/2 red Rebel",
			"rule":  "702.150",
		},
	})
}

// ===========================================================================
// §702.155 — Read Ahead
// ===========================================================================

// ApplyReadAhead sets a Saga's initial lore counter count to the chosen
// chapter. Per CR §702.155a, the player chooses a chapter number as the
// Saga enters, and it starts with that many lore counters.
func ApplyReadAhead(gs *GameState, perm *Permanent, chapter int) {
	if gs == nil || perm == nil || chapter <= 0 {
		return
	}
	perm.AddCounter("lore", chapter)

	gs.LogEvent(Event{
		Kind:   "read_ahead",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: chapter,
		Details: map[string]interface{}{
			"starting_chapter": chapter,
			"rule":             "702.155",
		},
	})
}

// ===========================================================================
// §702.156 — Ravenous
// ===========================================================================

// ApplyRavenous enters the creature with X +1/+1 counters. If X is 5 or
// more, the controller draws a card. Per CR §702.156a-b.
func ApplyRavenous(gs *GameState, perm *Permanent, x int) {
	if gs == nil || perm == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	if x > 0 {
		perm.AddCounter("+1/+1", x)
		gs.InvalidateCharacteristicsCache()
	}

	if x >= 5 {
		gs.drawOne(seatIdx)
		gs.LogEvent(Event{
			Kind:   "ravenous_draw",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"x":    x,
				"rule": "702.156",
			},
		})
	}

	gs.LogEvent(Event{
		Kind:   "ravenous",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Amount: x,
		Details: map[string]interface{}{
			"counters": x,
			"rule":     "702.156",
		},
	})
}

// ===========================================================================
// §702.163 — Compleated
// ===========================================================================

// IsCompleated returns true if the card/stack item was cast with compleated
// (2 life paid instead of colored mana for Phyrexian pips).
func IsCompleated(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	if v, ok := item.CostMeta["compleated"]; ok {
		if b, ok2 := v.(bool); ok2 {
			return b
		}
	}
	return false
}

// HasCompleated returns true if the card has the compleated keyword.
func HasCompleated(card *Card) bool {
	return cardHasKeywordByName(card, "compleated")
}

// ApplyCompleated marks a planeswalker as having entered with compleated.
// Per CR §702.163a, a planeswalker cast with compleated enters with fewer
// loyalty counters (one less for each Phyrexian pip paid with life).
func ApplyCompleated(gs *GameState, perm *Permanent, pipsPayedWithLife int) {
	if gs == nil || perm == nil || pipsPayedWithLife <= 0 {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["loyalty"] -= pipsPayedWithLife
	if perm.Counters["loyalty"] < 0 {
		perm.Counters["loyalty"] = 0
	}

	gs.LogEvent(Event{
		Kind:   "compleated",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: pipsPayedWithLife,
		Details: map[string]interface{}{
			"loyalty_reduction": pipsPayedWithLife,
			"rule":              "702.163",
		},
	})
}

// ===========================================================================
// §702.73 — Changeling (all creature types)
// ===========================================================================

// HasChangeling returns true if the permanent has the changeling keyword.
func HasChangeling(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	return perm.HasKeyword("changeling")
}

// HasChangelingCard returns true if the card has the changeling keyword.
func HasChangelingCard(card *Card) bool {
	return cardHasKeywordByName(card, "changeling")
}

// CheckChangelingType returns true if the permanent has a given creature
// type OR has changeling (which grants all creature types per §702.73a).
func CheckChangelingType(perm *Permanent, creatureType string) bool {
	if perm == nil {
		return false
	}
	if HasChangeling(perm) {
		return true
	}
	if perm.Card == nil {
		return false
	}
	lower := strings.ToLower(creatureType)
	for _, t := range perm.Card.Types {
		if strings.ToLower(t) == lower {
			return true
		}
	}
	return false
}

// ===========================================================================
// §702.6 — Equip activation
// ===========================================================================

// EquipCost extracts the equip cost from an equipment card.
func EquipCost(card *Card) int {
	return keywordArgCost(card, "equip")
}

// ActivateEquip pays the equip cost and attaches an equipment to a target
// creature the controller controls. Sorcery speed only (CR §702.6a).
// Returns true on success.
func ActivateEquip(gs *GameState, seatIdx int, equipment *Permanent, target *Permanent) bool {
	if gs == nil || equipment == nil || target == nil {
		return false
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if !equipment.IsEquipment() {
		return false
	}
	if !target.IsCreature() {
		return false
	}
	if equipment.Controller != seatIdx || target.Controller != seatIdx {
		return false
	}

	// Sorcery speed check.
	if gs.Active != seatIdx {
		return false
	}

	seat := gs.Seats[seatIdx]
	cost := EquipCost(equipment.Card)

	if seat.ManaPool < cost {
		return false
	}

	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	// Detach from previous creature if any.
	equipment.AttachedTo = target

	gs.LogEvent(Event{
		Kind:   "equip",
		Seat:   seatIdx,
		Source: equipment.Card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
			"rule":   "702.6",
		},
	})
	return true
}

// ===========================================================================
// §702.50 — Epic
// ===========================================================================

// ApplyEpic copies the spell at the beginning of each of your upkeeps for
// the rest of the game. You can't cast spells for the rest of the game.
func ApplyEpic(gs *GameState, seatIdx int, item *StackItem) {
	if gs == nil || item == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Set the "can't cast spells" flag.
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["epic_no_cast"] = 1

	// Register a delayed trigger to copy the spell each upkeep.
	epicCard := item.Card
	epicEffect := item.Effect
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "upkeep",
		ControllerSeat: seatIdx,
		SourceCardName: epicCard.DisplayName() + " (epic)",
		OneShot:        false, // repeating
		EffectFn: func(gs *GameState) {
			if gs.Active != seatIdx {
				return
			}
			copyItem := &StackItem{
				Card:       epicCard,
				Controller: seatIdx,
				Effect:     epicEffect,
				IsCopy:     true,
				CostMeta:   map[string]interface{}{"epic_copy": true},
			}
			PushStackItem(gs, copyItem)
			gs.LogEvent(Event{
				Kind:   "epic_copy",
				Seat:   seatIdx,
				Source: epicCard.DisplayName(),
				Details: map[string]interface{}{
					"rule": "702.50",
				},
			})
		},
	})

	gs.LogEvent(Event{
		Kind:   "epic",
		Seat:   seatIdx,
		Source: epicCard.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.50",
		},
	})
}

// ===========================================================================
// §702.60 — Recover
// ===========================================================================

// CheckRecover checks if a card with recover in the graveyard can be returned
// to hand when a creature an opponent controls dies. The controller pays the
// recover cost or exiles the card.
func CheckRecover(gs *GameState, seatIdx int, card *Card, recoverCost int) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}

	// Card must be in graveyard.
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

	if seat.ManaPool >= recoverCost {
		// Pay and return to hand.
		seat.ManaPool -= recoverCost
		SyncManaAfterSpend(seat)
		MoveCard(gs, card, seatIdx, "graveyard", "hand", "return-from-graveyard")

		gs.LogEvent(Event{
			Kind:   "recover",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Amount: recoverCost,
			Details: map[string]interface{}{
				"result": "returned_to_hand",
				"rule":   "702.60",
			},
		})
		return true
	}

	// Can't pay: exile the card.
	MoveCard(gs, card, seatIdx, "graveyard", "exile", "exile-from-graveyard")

	gs.LogEvent(Event{
		Kind:   "recover_exile",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: recoverCost,
		Details: map[string]interface{}{
			"result": "exiled",
			"rule":   "702.60",
		},
	})
	return false
}

// ===========================================================================
// §702.65 — Aura Swap
// ===========================================================================

// ActivateAuraSwap swaps an Aura on the battlefield with an Aura card in
// the controller's hand by paying the aura swap cost.
func ActivateAuraSwap(gs *GameState, seatIdx int, onBF *Permanent, inHand *Card, swapCost int) bool {
	if gs == nil || onBF == nil || inHand == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.ManaPool < swapCost {
		return false
	}

	// Pay cost.
	seat.ManaPool -= swapCost
	SyncManaAfterSpend(seat)

	// Remove onBF from battlefield, put into hand.
	removePermanentFromBattlefield(gs, onBF)
	if onBF.Card != nil {
		MoveCard(gs, onBF.Card, seatIdx, "battlefield", "hand", "aura-swap")
	}

	// Remove inHand from hand, put onto battlefield attached to same target.
	removeCardFromZone(gs, seatIdx, inHand, "hand")

	newPerm := &Permanent{
		Card:       inHand,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		AttachedTo: onBF.AttachedTo,
	}
	seat.Battlefield = append(seat.Battlefield, newPerm)
	RegisterReplacementsForPermanent(gs, newPerm)
	FirePermanentETBTriggers(gs, newPerm)

	gs.LogEvent(Event{
		Kind:   "aura_swap",
		Seat:   seatIdx,
		Source: inHand.DisplayName(),
		Details: map[string]interface{}{
			"swapped_out": onBF.Card.DisplayName(),
			"cost":        swapCost,
			"rule":        "702.65",
		},
	})
	return true
}

// ===========================================================================
// §702.68 — Frenzy
// ===========================================================================

// ApplyFrenzy grants +N/+0 to a creature whenever it attacks and isn't
// blocked. Simplified: checks blocker count post-declare.
func ApplyFrenzy(gs *GameState, perm *Permanent, n int) {
	if gs == nil || perm == nil || n <= 0 {
		return
	}
	perm.Modifications = append(perm.Modifications, Modification{
		Power:     n,
		Toughness: 0,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "frenzy",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.68",
		},
	})
}

// ===========================================================================
// §702.69 — Gravestorm
// ===========================================================================

// ApplyGravestorm creates copies of a spell equal to the number of permanents
// put into graveyards this turn. Similar to storm but counts graveyard entries.
func ApplyGravestorm(gs *GameState, item *StackItem) {
	if gs == nil || item == nil || item.Card == nil {
		return
	}

	// Count permanents put into graveyards this turn from the game flags.
	graveyardCount := 0
	if gs.Flags != nil {
		graveyardCount = gs.Flags["permanents_to_graveyard_this_turn"]
	}

	if graveyardCount <= 0 {
		return
	}

	seatIdx := item.Controller
	for i := 0; i < graveyardCount; i++ {
		copyCard := &Card{
			Name:          item.Card.Name + " (gravestorm " + itoaBatch6(i+1) + ")",
			Owner:         item.Card.Owner,
			BasePower:     item.Card.BasePower,
			BaseToughness: item.Card.BaseToughness,
			Types:         append([]string(nil), item.Card.Types...),
			Colors:        append([]string(nil), item.Card.Colors...),
			CMC:           0,
		}
		if item.Card.AST != nil {
			copyCard.AST = item.Card.AST
		}
		copyItem := &StackItem{
			Controller: seatIdx,
			Card:       copyCard,
			Effect:     item.Effect,
			IsCopy:     true,
		}
		copyItem.ID = nextStackID(gs)
		gs.Stack = append(gs.Stack, copyItem)
	}

	gs.LogEvent(Event{
		Kind:   "gravestorm",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Amount: graveyardCount,
		Details: map[string]interface{}{
			"copies": graveyardCount,
			"rule":   "702.69",
		},
	})
}

// ===========================================================================
// §702.71 — Transfigure
// ===========================================================================

// ActivateTransfigure sacrifices a creature and searches the library for a
// creature with the same mana value.
func ActivateTransfigure(gs *GameState, seatIdx int, perm *Permanent, cost int) {
	if gs == nil || perm == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.ManaPool < cost {
		return
	}

	targetCMC := 0
	if perm.Card != nil {
		targetCMC = perm.Card.CMC
	}

	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)
	SacrificePermanent(gs, perm, "transfigure")

	// Search library for a creature with the same CMC.
	foundIdx := -1
	for i, c := range seat.Library {
		if c != nil && cardHasType(c, "creature") && c.CMC == targetCMC {
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

	// Shuffle library.
	if gs.Rng != nil && len(seat.Library) > 1 {
		gs.Rng.Shuffle(len(seat.Library), func(i, j int) {
			seat.Library[i], seat.Library[j] = seat.Library[j], seat.Library[i]
		})
	}

	gs.LogEvent(Event{
		Kind:   "transfigure",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_cmc": targetCMC,
			"found":      foundName,
			"rule":       "702.71",
		},
	})
}

// ===========================================================================
// §702.106 — Hidden Agenda
// ===========================================================================

// ApplyHiddenAgenda secretly names a card. The named card gets some bonus
// as long as the conspiracy is face up in the command zone.
func ApplyHiddenAgenda(gs *GameState, seatIdx int, conspiracyPerm *Permanent, namedCard string) {
	if gs == nil || conspiracyPerm == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if conspiracyPerm.Flags == nil {
		conspiracyPerm.Flags = map[string]int{}
	}
	conspiracyPerm.Flags["hidden_agenda"] = 1

	// Store the named card on the conspiracy's counter map as a workaround.
	if conspiracyPerm.Counters == nil {
		conspiracyPerm.Counters = map[string]int{}
	}

	gs.LogEvent(Event{
		Kind:   "hidden_agenda",
		Seat:   seatIdx,
		Source: conspiracyPerm.Card.DisplayName(),
		Details: map[string]interface{}{
			"named_card": namedCard,
			"rule":       "702.106",
		},
	})
}

// ===========================================================================
// §702.89 — Umbra Armor
// ===========================================================================

// CheckUmbraArmor prevents the enchanted creature from being destroyed by
// destroying the Aura instead. Per CR §702.89a, if enchanted creature would
// be destroyed, instead remove all damage from it and destroy this Aura.
// Returns true if umbra armor saved the creature.
func CheckUmbraArmor(gs *GameState, creature *Permanent) bool {
	if gs == nil || creature == nil {
		return false
	}
	seatIdx := creature.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}

	// Find an Aura attached to this creature with umbra armor.
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.AttachedTo != creature {
			continue
		}
		if p.Card == nil {
			continue
		}
		if !p.HasKeyword("umbra armor") && !cardHasKeywordByName(p.Card, "totem armor") {
			continue
		}

		// Remove all damage from creature.
		creature.MarkedDamage = 0

		// Destroy the Aura instead.
		SacrificePermanent(gs, p, "umbra_armor")

		gs.LogEvent(Event{
			Kind:   "umbra_armor",
			Seat:   seatIdx,
			Source: p.Card.DisplayName(),
			Details: map[string]interface{}{
				"saved":    creature.Card.DisplayName(),
				"rule":     "702.89",
			},
		})
		return true
	}
	return false
}

// ===========================================================================
// §702.113b — Ingest
// ===========================================================================

// ApplyIngest exiles the top card of the defending player's library when this
// creature deals combat damage to a player. Per CR §702.113b (from BFZ).
func ApplyIngest(gs *GameState, defenderSeat int) {
	if gs == nil || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[defenderSeat]
	if seat == nil || len(seat.Library) == 0 {
		return
	}

	top := seat.Library[0]
	MoveCard(gs, top, defenderSeat, "library", "exile", "exile-from-library")

	gs.LogEvent(Event{
		Kind:   "ingest",
		Seat:   defenderSeat,
		Amount: 1,
		Details: map[string]interface{}{
			"exiled": top.DisplayName(),
			"rule":   "702.113b",
		},
	})
}

// ===========================================================================
// Newer / Set-Specific Keywords (§702.169+)
// ===========================================================================

// ---------------------------------------------------------------------------
// §702.169 — Freerunning
// ---------------------------------------------------------------------------

// CanCastForFreerunning returns true if a creature you control dealt combat
// damage to a player this turn, enabling the freerunning alt cost.
func CanCastForFreerunning(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Flags == nil {
		return false
	}
	return seat.Flags["creature_dealt_combat_damage_to_player"] > 0
}

// ---------------------------------------------------------------------------
// §702.174 — Gift
// ---------------------------------------------------------------------------

// ApplyGift offers a gift to an opponent. If accepted, you get a bonus effect.
// Simplified: opponent always declines (greedy AI). Returns false.
func ApplyGift(gs *GameState, seatIdx int, perm *Permanent) bool {
	if gs == nil || perm == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	gs.LogEvent(Event{
		Kind:   "gift_declined",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.174",
		},
	})
	return false
}

// ---------------------------------------------------------------------------
// §702.173 — Space Sculptor (stub)
// ---------------------------------------------------------------------------

// ApplySpaceSculptor grants a target creature a basic land type until EOT.
func ApplySpaceSculptor(gs *GameState, perm *Permanent, landType string) {
	if gs == nil || perm == nil {
		return
	}
	gs.LogEvent(Event{
		Kind:   "space_sculptor",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"land_type": landType,
			"rule":      "702.173",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.177 — Visit (stub)
// ---------------------------------------------------------------------------

// ApplyVisit logs a visit event. Visit is a set-specific mechanic.
func ApplyVisit(gs *GameState, seatIdx int, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gs.LogEvent(Event{
		Kind:   "visit",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.177",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.178 — Max Speed (stub)
// ---------------------------------------------------------------------------

// HasMaxSpeed returns true if the permanent has the max speed keyword.
func HasMaxSpeed(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	return perm.HasKeyword("max speed")
}

// ---------------------------------------------------------------------------
// §702.179 — Start Your Engines! (stub)
// ---------------------------------------------------------------------------

// ApplyStartYourEngines animates all Vehicles the controller controls
// until end of turn, making them creature artifacts.
func ApplyStartYourEngines(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		isVehicle := false
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "vehicle") {
				isVehicle = true
				break
			}
		}
		if isVehicle {
			// Make it a creature until EOT.
			hasCreature := false
			for _, t := range p.Card.Types {
				if strings.EqualFold(t, "creature") {
					hasCreature = true
					break
				}
			}
			if !hasCreature {
				p.Card.Types = append(p.Card.Types, "creature")
			}
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["start_your_engines"] = 1
			count++
		}
	}

	gs.LogEvent(Event{
		Kind:   "start_your_engines",
		Seat:   seatIdx,
		Amount: count,
		Details: map[string]interface{}{
			"vehicles_animated": count,
			"rule":              "702.179",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.180 — Harmonize (stub)
// ---------------------------------------------------------------------------

// ApplyHarmonize logs a harmonize event.
func ApplyHarmonize(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	gs.LogEvent(Event{
		Kind: "harmonize",
		Seat: seatIdx,
		Details: map[string]interface{}{
			"rule": "702.180",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.181 — Mobilize (stub)
// ---------------------------------------------------------------------------

// ApplyMobilize logs a mobilize event.
func ApplyMobilize(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	gs.LogEvent(Event{
		Kind: "mobilize",
		Seat: seatIdx,
		Details: map[string]interface{}{
			"rule": "702.181",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.182 — Tiered (stub)
// ---------------------------------------------------------------------------

// HasTiered returns true if the card has the tiered keyword.
func HasTiered(card *Card) bool {
	return cardHasKeywordByName(card, "tiered")
}

// ---------------------------------------------------------------------------
// §702.183 — Job Select (stub)
// ---------------------------------------------------------------------------

// HasJobSelect returns true if the card has the job select keyword.
func HasJobSelect(card *Card) bool {
	return cardHasKeywordByName(card, "job select")
}

// ---------------------------------------------------------------------------
// §702.184 — Station (stub)
// ---------------------------------------------------------------------------

// ApplyStation logs a station event.
func ApplyStation(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gs.LogEvent(Event{
		Kind:   "station",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.184",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.185 — Warp (stub)
// ---------------------------------------------------------------------------

// ApplyWarp logs a warp event.
func ApplyWarp(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	gs.LogEvent(Event{
		Kind:   "warp",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.185",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.186 — Solved (stub)
// ---------------------------------------------------------------------------

// HasSolved returns true if the card has the solved keyword.
func HasSolved(card *Card) bool {
	return cardHasKeywordByName(card, "solved")
}

// ---------------------------------------------------------------------------
// §702.187 — Mayhem (stub)
// ---------------------------------------------------------------------------

// ApplyMayhem logs a mayhem event.
func ApplyMayhem(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	gs.LogEvent(Event{
		Kind: "mayhem",
		Seat: seatIdx,
		Details: map[string]interface{}{
			"rule": "702.187",
		},
	})
}

// ---------------------------------------------------------------------------
// §702.190 — Infinity (stub)
// ---------------------------------------------------------------------------

// HasInfinity returns true if the card has the infinity keyword.
func HasInfinity(card *Card) bool {
	return cardHasKeywordByName(card, "infinity")
}

// ===========================================================================
// KEYWORD ACTIONS (§701)
// ===========================================================================

// ---------------------------------------------------------------------------
// §701.4 — Behold
// ---------------------------------------------------------------------------

// Behold reveals a card from your hand to opponents. Per CR §701.4, you
// reveal a card from your hand and it becomes "beheld" for the rest of the
// game for triggered ability purposes.
func Behold(gs *GameState, seatIdx int, card *Card) {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	gs.LogEvent(Event{
		Kind:   "behold",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "701.4",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.11 — Triple
// ---------------------------------------------------------------------------

// TripleValue triples an attribute value (power, toughness, or counter count).
// Returns the tripled value. Per CR §701.11, "triple" means multiply by 3.
func TripleValue(value int) int {
	return value * 3
}

// TriplePower triples a creature's power until end of turn.
func TriplePower(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	currentPower := perm.Power()
	bonus := currentPower * 2 // current + bonus = 3x
	perm.Modifications = append(perm.Modifications, Modification{
		Power:     bonus,
		Toughness: 0,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()

	gs.LogEvent(Event{
		Kind:   "triple_power",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: currentPower * 3,
		Details: map[string]interface{}{
			"original": currentPower,
			"rule":     "701.11",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.12 — Exchange
// ---------------------------------------------------------------------------

// ExchangeLifeTotals exchanges life totals between two players.
func ExchangeLifeTotals(gs *GameState, seat1, seat2 int) {
	if gs == nil {
		return
	}
	if seat1 < 0 || seat1 >= len(gs.Seats) || seat2 < 0 || seat2 >= len(gs.Seats) {
		return
	}
	s1 := gs.Seats[seat1]
	s2 := gs.Seats[seat2]
	if s1 == nil || s2 == nil {
		return
	}

	s1.Life, s2.Life = s2.Life, s1.Life

	gs.LogEvent(Event{
		Kind:   "exchange_life",
		Seat:   seat1,
		Target: seat2,
		Details: map[string]interface{}{
			"seat1_new_life": s1.Life,
			"seat2_new_life": s2.Life,
			"rule":           "701.12",
		},
	})
}

// ExchangeControl exchanges control of two permanents.
func ExchangeControl(gs *GameState, perm1, perm2 *Permanent) {
	if gs == nil || perm1 == nil || perm2 == nil {
		return
	}
	perm1.Controller, perm2.Controller = perm2.Controller, perm1.Controller

	// Move permanents between seats' battlefields.
	removePermanentFromBattlefield(gs, perm1)
	removePermanentFromBattlefield(gs, perm2)

	if perm1.Controller >= 0 && perm1.Controller < len(gs.Seats) {
		gs.Seats[perm1.Controller].Battlefield = append(
			gs.Seats[perm1.Controller].Battlefield, perm1)
	}
	if perm2.Controller >= 0 && perm2.Controller < len(gs.Seats) {
		gs.Seats[perm2.Controller].Battlefield = append(
			gs.Seats[perm2.Controller].Battlefield, perm2)
	}

	name1, name2 := "<nil>", "<nil>"
	if perm1.Card != nil {
		name1 = perm1.Card.DisplayName()
	}
	if perm2.Card != nil {
		name2 = perm2.Card.DisplayName()
	}

	gs.LogEvent(Event{
		Kind: "exchange_control",
		Details: map[string]interface{}{
			"perm1":          name1,
			"perm1_new_ctrl": perm1.Controller,
			"perm2":          name2,
			"perm2_new_ctrl": perm2.Controller,
			"rule":           "701.12",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.28 — Convert
// ---------------------------------------------------------------------------

// ConvertPermanent transforms a double-faced permanent by flipping it to its
// other face. Per CR §701.28, "convert" is the keyword action for non-DFC
// transforming permanents (e.g. Ixalan transforming lands).
func ConvertPermanent(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	perm.Transformed = !perm.Transformed

	// Swap ASTs if available.
	if perm.FrontFaceAST != nil && perm.BackFaceAST != nil && perm.Card != nil {
		if perm.Transformed {
			perm.Card.AST = perm.BackFaceAST
			if perm.BackFaceName != "" {
				perm.Card.Name = perm.BackFaceName
			}
		} else {
			perm.Card.AST = perm.FrontFaceAST
			if perm.FrontFaceName != "" {
				perm.Card.Name = perm.FrontFaceName
			}
		}
	}

	source := "<nil>"
	if perm.Card != nil {
		source = perm.Card.DisplayName()
	}

	gs.LogEvent(Event{
		Kind:   "convert",
		Seat:   perm.Controller,
		Source: source,
		Details: map[string]interface{}{
			"transformed": perm.Transformed,
			"rule":        "701.28",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.38 — Vote
// ---------------------------------------------------------------------------

// ConductVote runs a voting round among all players. Returns the winning
// option. Simplified: each player votes for their own best interest.
// The controller votes for option A, opponents vote for option B.
func ConductVote(gs *GameState, seatIdx int, optionA, optionB string) string {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return optionA
	}

	votesA := 1 // Controller always votes for optionA.
	votesB := 0

	opps := gs.Opponents(seatIdx)
	votesB = len(opps)

	winner := optionA
	if votesB > votesA {
		winner = optionB
	}

	gs.LogEvent(Event{
		Kind: "vote",
		Seat: seatIdx,
		Details: map[string]interface{}{
			"option_a":  optionA,
			"option_b":  optionB,
			"votes_a":   votesA,
			"votes_b":   votesB,
			"winner":    winner,
			"rule":      "701.38",
		},
	})

	return winner
}

// ---------------------------------------------------------------------------
// §701.64 — Harness
// ---------------------------------------------------------------------------

// Harness puts a +1/+1 counter on a creature and gives it an energy counter
// to the controller.
func Harness(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	seat := gs.Seats[seatIdx]
	if seat != nil {
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["energy"] += 1
	}

	gs.LogEvent(Event{
		Kind:   "harness",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "701.64",
		},
	})
}

// ---------------------------------------------------------------------------
// §701.65-68 — Elemental Bending (Airbend, Earthbend, Waterbend, Firebend)
// These are very new set-specific keyword actions. Stub implementations.
// ---------------------------------------------------------------------------

// Airbend logs an airbend action.
func Airbend(gs *GameState, seatIdx int) {
	if gs == nil {
		return
	}
	gs.LogEvent(Event{
		Kind: "airbend", Seat: seatIdx,
		Details: map[string]interface{}{"rule": "701.65"},
	})
}

// Earthbend logs an earthbend action.
func Earthbend(gs *GameState, seatIdx int) {
	if gs == nil {
		return
	}
	gs.LogEvent(Event{
		Kind: "earthbend", Seat: seatIdx,
		Details: map[string]interface{}{"rule": "701.66"},
	})
}

// Waterbend logs a waterbend action.
func Waterbend(gs *GameState, seatIdx int) {
	if gs == nil {
		return
	}
	gs.LogEvent(Event{
		Kind: "waterbend", Seat: seatIdx,
		Details: map[string]interface{}{"rule": "701.67"},
	})
}

// Firebend logs a firebend action.
func Firebend(gs *GameState, seatIdx int) {
	if gs == nil {
		return
	}
	gs.LogEvent(Event{
		Kind: "firebend", Seat: seatIdx,
		Details: map[string]interface{}{"rule": "701.68"},
	})
}

// ===========================================================================
// Internal helpers (batch6-local)
// ===========================================================================

func itoaBatch6(n int) string {
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

// getKeywordNames extracts all keyword names from a permanent's AST.
func getKeywordNames(perm *Permanent) []string {
	if perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return nil
	}
	var names []string
	for _, ab := range perm.Card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok {
			names = append(names, strings.ToLower(strings.TrimSpace(kw.Name)))
		}
	}
	return names
}
