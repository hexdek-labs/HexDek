package gameengine

// CR §301 / §111 — artifact token family.
//
// The modern card pool spawns a large family of predefined artifact
// tokens. Each has a stereotyped activated ability and is referenced
// by name from card oracle text ("create a Treasure token", "create
// three Food tokens", etc.). Implementations live here so card
// handlers (per_card/*) can drop tokens without duplicating the
// boilerplate.
//
// Tokens supported:
//
//   Treasure    — {T}, Sacrifice: add one mana of any color.
//   Gold        — Sacrifice: add one mana of any color.
//   Clue        — {2}, Sacrifice: draw a card.
//   Food        — {2}, {T}, Sacrifice: gain 3 life.
//   Blood       — {1}, {T}, Discard a card, Sacrifice: draw a card.
//   Map         — {1}, {T}, Sacrifice: target creature explores.
//   Powerstone  — {T}: Add {C}. Spend only on noncreature.
//   Junk        — {2}, {T}, Exile: exile top of library, may play
//                 this turn with flash.
//
// The mana-producing ones (Treasure, Gold, Powerstone) are wired into
// mana_artifacts.go's ApplyArtifactMana — the token creators here only
// put the Permanent on the battlefield.

// CreateTreasureToken drops a Treasure onto the battlefield. A thin
// re-export of the existing cast_counts.go helper so callers don't
// need to cross-import.
func CreateTreasureToken(gs *GameState, seatIdx int) {
	createTreasureToken(gs, seatIdx)
}

// CreateGoldToken drops a Gold token onto the battlefield.
// Sacrifice: add one mana of any color. No tap required.
func CreateGoldToken(gs *GameState, seatIdx int) {
	createSimpleArtifactToken(gs, seatIdx, "Gold Token", "gold")
}

// CreateClueToken drops a Clue. {2}, Sacrifice: draw a card.
func CreateClueToken(gs *GameState, seatIdx int) {
	createSimpleArtifactToken(gs, seatIdx, "Clue Token", "clue")
}

// CreateFoodToken drops a Food. {2}, {T}, Sacrifice: gain 3 life.
func CreateFoodToken(gs *GameState, seatIdx int) {
	createSimpleArtifactToken(gs, seatIdx, "Food Token", "food")
}

// CreateBloodToken drops a Blood. {1}, {T}, Discard, Sacrifice: draw.
func CreateBloodToken(gs *GameState, seatIdx int) {
	createSimpleArtifactToken(gs, seatIdx, "Blood Token", "blood")
}

// CreateMapToken drops a Map. {1}, {T}, Sacrifice: target creature
// explores.
func CreateMapToken(gs *GameState, seatIdx int) {
	createSimpleArtifactToken(gs, seatIdx, "Map Token", "map")
}

// CreatePowerstoneToken drops a Powerstone. {T}: Add {C}; spend only
// on noncreature.
func CreatePowerstoneToken(gs *GameState, seatIdx int) {
	createSimpleArtifactToken(gs, seatIdx, "Powerstone Token", "powerstone")
}

// CreateJunkToken drops a Junk. {2}, {T}, Exile: exile top of library,
// may play this turn with flash.
func CreateJunkToken(gs *GameState, seatIdx int) {
	createSimpleArtifactToken(gs, seatIdx, "Junk Token", "junk")
}

// createSimpleArtifactToken is the shared helper for non-mana-producing
// (or mana-producing-via-artifact-branch) artifact tokens. It creates
// a Permanent with the right Types tag so callers / mana helpers /
// state-based actions see the token type correctly.
func createSimpleArtifactToken(gs *GameState, seatIdx int,
	name, subtypeTag string) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	token := &Card{
		Name:     name,
		Owner:    seatIdx,
		Types:    []string{"token", "artifact", subtypeTag},
		TypeLine: "Token Artifact — " + capitalize(subtypeTag),
	}
	perm := &Permanent{
		Card:          token,
		Controller:    seatIdx,
		Owner:         seatIdx,
		Tapped:        false,
		SummoningSick: false,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	// Fire token_created trigger with re-entrancy guard.
	if gs.Flags == nil || gs.Flags["in_token_trigger"] == 0 {
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["in_token_trigger"] = 1
		FireCardTrigger(gs, "token_created", map[string]interface{}{
			"controller_seat": seatIdx,
			"count":           1,
			"types":           token.Types,
			"source":          name,
		})
		gs.Flags["in_token_trigger"] = 0
	}
	gs.LogEvent(Event{
		Kind:   "create_token",
		Seat:   seatIdx,
		Source: name,
		Details: map[string]interface{}{
			"subtype": subtypeTag,
			"rule":    "111.10",
		},
	})
}

// CreateCreatureToken drops a creature token with the given name, types,
// and base P/T onto the battlefield.
func CreateCreatureToken(gs *GameState, seatIdx int, name string, types []string, power, toughness int) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}
	allTypes := append([]string{"token"}, types...)
	token := &Card{
		Name:          name,
		Owner:         seatIdx,
		Types:         allTypes,
		BasePower:     power,
		BaseToughness: toughness,
	}
	perm := &Permanent{
		Card:          token,
		Controller:    seatIdx,
		Owner:         seatIdx,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, perm)
	RegisterReplacementsForPermanent(gs, perm)
	FirePermanentETBTriggers(gs, perm)
	// Fire token_created trigger with re-entrancy guard.
	if gs.Flags == nil || gs.Flags["in_token_trigger"] == 0 {
		if gs.Flags == nil {
			gs.Flags = map[string]int{}
		}
		gs.Flags["in_token_trigger"] = 1
		FireCardTrigger(gs, "token_created", map[string]interface{}{
			"controller_seat": seatIdx,
			"count":           1,
			"types":           allTypes,
			"source":          name,
		})
		gs.Flags["in_token_trigger"] = 0
	}
	gs.LogEvent(Event{
		Kind:   "create_token",
		Seat:   seatIdx,
		Source: name,
		Details: map[string]interface{}{
			"power":     power,
			"toughness": toughness,
			"rule":      "111.10",
		},
	})
	return perm
}

// capitalize returns its argument with the first byte in upper case,
// used for building human-readable type-line strings.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 32
	}
	return string(b)
}
