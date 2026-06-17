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
	MintTokenInstanceID(gs, token, "", currentMintEnablerID(gs))
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
	seat.Turn.TokensCreated++
	seat.Turn.ArtifactsEntered++
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

// CreateDoubledTokens fires the §614 would_create_token replacement chain for
// `count` tokens that `src` creates under `seat`'s control (Doubling Season,
// Parallel Lives, Anointed Procession, Primal Vigor, …), then invokes mkOne the
// resulting POST-DOUBLING number of times. It returns every created permanent
// so the caller can apply per-token setup (enters attacking / tapped / with
// keywords) to ALL of them — including the doubled copies.
//
// mkOne must mint exactly ONE token (e.g. via CreateCreatureToken) and must NOT
// itself fire the chain. This is the doubling-aware entry point per-card
// token-makers should use instead of calling CreateCreatureToken directly in a
// raw loop — the raw path bypasses every token doubler (the systemic coverage
// gap the r63 doubler audit surfaced).
func CreateDoubledTokens(gs *GameState, seat, count int, src *Permanent, mkOne func() *Permanent) []*Permanent {
	if gs == nil || count <= 0 || mkOne == nil {
		return nil
	}
	n, cancelled := FireCreateTokenEvent(gs, seat, count, src)
	if cancelled || n <= 0 {
		return nil
	}
	out := make([]*Permanent, 0, n)
	for i := 0; i < n; i++ {
		if p := mkOne(); p != nil {
			out = append(out, p)
		}
	}
	return out
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
	// CR §800.4a: a departed seat cannot create or control new tokens. This
	// path bypasses FireCreateTokenEvent, so it needs its own gate — otherwise
	// a token minted onto a LeftGame seat's battlefield (which the census skips)
	// orphans its InstanceID (r63 eliminated-seat token leak).
	if SeatHasLeftGame(gs, seatIdx) {
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
	MintTokenInstanceID(gs, token, "", currentMintEnablerID(gs))
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
	seat.Turn.TokensCreated++
	seat.Turn.CreaturesEntered++
	for _, tp := range types {
		if tp == "artifact" {
			seat.Turn.ArtifactsEntered++
			break
		}
	}
	for _, tp := range types {
		if tp == "enchantment" {
			seat.Turn.EnchantmentsEntered++
			break
		}
	}
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

// predefinedTokenSacSubtype returns the sacrifice-ability subtype of a
// predefined NON-mana artifact token (clue/food/blood/map) that carries no
// AST, or "" if `perm` isn't one. The mana tokens (treasure/gold/powerstone)
// are handled by ApplyArtifactMana and are intentionally excluded here.
func predefinedTokenSacSubtype(perm *Permanent) string {
	if perm == nil || perm.Card == nil {
		return ""
	}
	// Only synthesize the built-in ability for predefined tokens, which carry
	// no AST. A real card with one of these subtypes uses its own AST and is
	// dispatched normally by ActivateAbility.
	if perm.Card.AST != nil {
		return ""
	}
	if !perm.IsToken() {
		return ""
	}
	for _, t := range perm.Card.Types {
		switch t {
		case "clue", "food", "blood", "map":
			return t
		}
	}
	return ""
}

// ActivatePredefinedTokenAbility executes the built-in sacrifice ability of a
// predefined NON-mana artifact token (CR §111.10 / §701.x):
//
//	Clue  — {2}, Sacrifice: draw a card.
//	Food  — {2}, {T}, Sacrifice: you gain 3 life.
//	Blood — {1}, {T}, Discard a card, Sacrifice: draw a card.
//	Map   — {1}, {T}, Sacrifice: target creature you control explores.
//
// These tokens carry no AST, so ActivateAbility can't dispatch them; this is
// the canonical execution primitive (routed to from ActivateAbility and
// callable directly by per-card "sacrifice a Clue/Food/…" effects). All costs
// are checked before any is paid (CR §601.2f). Returns true on success.
//
// MVP: the effect resolves inline (these are non-mana abilities that would
// normally use the stack; the predefined-token model is simplified). The
// sacrifice + tap + discard + mana are paid as the activation cost.
func ActivatePredefinedTokenAbility(gs *GameState, seatIdx int, perm *Permanent) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || perm == nil || perm.Controller != seatIdx || perm.PhasedOut {
		return false
	}
	sub := predefinedTokenSacSubtype(perm)
	if sub == "" {
		return false
	}

	// Cost parameters per token.
	var manaCost int
	var needsTap, needsDiscard, needsExplore bool
	switch sub {
	case "clue":
		manaCost = 2
	case "food":
		manaCost, needsTap = 2, true
	case "blood":
		manaCost, needsTap, needsDiscard = 1, true, true
	case "map":
		manaCost, needsTap, needsExplore = 1, true, true
	}

	// Check ALL costs before paying any (CR §601.2f).
	if needsTap && perm.Tapped {
		return false
	}
	if !EnsureTypedPool(seat).CanPayGeneric(manaCost, "activated") {
		return false
	}
	var discardChoice *Card
	if needsDiscard {
		if len(seat.Hand) == 0 {
			return false // can't pay the discard cost
		}
		if seat.Hat != nil {
			if picks := seat.Hat.ChooseDiscard(gs, seatIdx, seat.Hand, 1); len(picks) > 0 {
				discardChoice = picks[0]
			}
		}
		if discardChoice == nil {
			discardChoice = seat.Hand[0]
		}
	}
	var exploreTarget *Permanent
	if needsExplore {
		for _, p := range seat.Battlefield {
			if p != nil && p != perm && p.IsCreature() && !p.PhasedOut {
				exploreTarget = p
				break
			}
		}
		if exploreTarget == nil {
			return false // "target creature you control" requires a legal target
		}
	}

	// Pay costs.
	if manaCost > 0 {
		if !PayGenericCost(gs, seat, manaCost, "activated", "token_ability", perm.Card.DisplayName()) {
			return false
		}
	}
	if needsTap {
		perm.Tapped = true
	}
	if needsDiscard && discardChoice != nil {
		DiscardCard(gs, discardChoice, seatIdx)
	}
	SacrificePermanent(gs, perm, sub+"_ability")

	gs.LogEvent(Event{
		Kind:   "token_ability",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{"subtype": sub, "rule": "111.10"},
	})

	// Resolve the effect.
	switch sub {
	case "clue", "blood":
		DrawN(gs, seatIdx, 1, perm)
	case "food":
		GainLife(gs, seatIdx, 3, perm.Card.DisplayName())
	case "map":
		PerformExplore(gs, exploreTarget)
	}
	return true
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
