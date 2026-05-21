package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// dev/percard-stubs-batchN-r53 — 10 fresh stub ports.
// One regression test per port. Wider-net survey: avoids A–M.
// ---------------------------------------------------------------------------

// 1. Akroma, Angel of Wrath — ETB stamps the canonical keyword + protection flags.
func TestStubsBatchN_Akroma_KeywordFlags(t *testing.T) {
	gs := newGame(t, 2)
	akroma := addPerm(gs, 0, "Akroma, Angel of Wrath", "creature", "legendary")
	gameengine.InvokeETBHook(gs, akroma)
	for _, k := range []string{"kw:flying", "kw:first_strike", "kw:vigilance", "kw:trample", "kw:haste", "prot:B", "prot:R"} {
		if akroma.Flags[k] != 1 {
			t.Errorf("expected %s flag set, got %v", k, akroma.Flags)
		}
	}
}

// 2. Don Andres, the Renegade — opponent-owned noncreature spell cast mints 2 tapped Treasures.
func TestStubsBatchN_DonAndres_NoncreatureSpawnsTappedTreasures(t *testing.T) {
	gs := newGame(t, 2)
	don := addPerm(gs, 0, "Don Andres, the Renegade", "creature", "legendary")
	don.Card.BasePower = 4
	don.Card.BaseToughness = 4
	beforeLen := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "noncreature_spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card_owner":  1, // foreign-owned spell (Etali-style, Gonti, etc.)
	})

	tappedTreasures := 0
	for i := beforeLen; i < len(gs.Seats[0].Battlefield); i++ {
		p := gs.Seats[0].Battlefield[i]
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "treasure") && p.Tapped {
			tappedTreasures++
		}
	}
	if tappedTreasures != 2 {
		t.Errorf("expected 2 tapped Treasures, got %d", tappedTreasures)
	}
}

// 3. Baylen, the Haymaker — tap 2 untapped tokens to add 1 mana.
func TestStubsBatchN_Baylen_Ability0Add1Mana(t *testing.T) {
	gs := newGame(t, 2)
	baylen := addPerm(gs, 0, "Baylen, the Haymaker", "creature", "legendary")
	baylen.Card.BasePower = 4
	baylen.Card.BaseToughness = 4
	// Add 3 untapped friendly tokens so ability 0 (need 2) has room.
	for i := 0; i < 3; i++ {
		tok := addPerm(gs, 0, "Spawn Token", "creature", "token")
		tok.Tapped = false
	}
	before := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, baylen, 0, nil)

	if gs.Seats[0].ManaPool != before+1 {
		t.Errorf("expected mana pool +1, got %d (was %d)", gs.Seats[0].ManaPool, before)
	}
	tappedTokens := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "token") && p.Tapped {
			tappedTokens++
		}
	}
	if tappedTokens != 2 {
		t.Errorf("expected 2 tokens tapped, got %d", tappedTokens)
	}
}

// 4. Lavinia, Azorius Renegade — counters opponent's free spell on stack.
func TestStubsBatchN_Lavinia_CountersFreeSpell(t *testing.T) {
	gs := newGame(t, 2)
	lavinia := addPerm(gs, 0, "Lavinia, Azorius Renegade", "creature", "legendary")
	lavinia.Card.BasePower = 2
	lavinia.Card.BaseToughness = 3
	// Opponent casts a free (CMC 0) spell. The stack has the corresponding StackItem.
	freeCard := &gameengine.Card{Name: "Mishra's Bauble", Types: []string{"artifact"}, Owner: 1}
	item := &gameengine.StackItem{Controller: 1, Card: freeCard}
	gs.Stack = append(gs.Stack, item)

	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"card":        freeCard,
	})

	if !item.Countered {
		t.Errorf("expected free CMC-0 spell to be countered by Lavinia")
	}
}

// 5. Kadena, Slinking Sorcerer — face-down creature cast cost reduced by {3}.
func TestStubsBatchN_Kadena_FaceDownCostReduction(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	kadena := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Kadena, Slinking Sorcerer", Types: []string{"creature", "legendary"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, kadena)
	morph := &gameengine.Card{
		Name:  "Morph Creature",
		Types: []string{"creature", "face_down", "cost:5"},
	}
	cost := gameengine.CalculateTotalCost(gs, morph, 0)
	if cost != 2 {
		t.Errorf("expected cost 2 (5 - 3 Kadena), got %d", cost)
	}
	// After "first cast" used, second cast same turn doesn't discount.
	gs.Seats[0].Flags = map[string]int{"kadena_used_turn": gs.Turn + 1}
	cost = gameengine.CalculateTotalCost(gs, morph, 0)
	if cost != 5 {
		t.Errorf("expected cost 5 on second cast (no discount), got %d", cost)
	}
}

// 6. Gargos, Vicious Watcher — fight trigger applies mutual MarkedDamage.
func TestStubsBatchN_Gargos_FightTriggerMarksDamage(t *testing.T) {
	gs := newGame(t, 2)
	gargos := addPerm(gs, 0, "Gargos, Vicious Watcher", "creature", "legendary")
	gargos.Card.BasePower = 8
	gargos.Card.BaseToughness = 7
	friendly := addPerm(gs, 0, "Friendly Hydra", "creature")
	friendly.Card.BasePower = 3
	friendly.Card.BaseToughness = 3
	oppCreature := addPerm(gs, 1, "Frail Bird", "creature")
	oppCreature.Card.BasePower = 1
	oppCreature.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "creature_targeted", map[string]interface{}{
		"target_perm": friendly,
		"caster_seat": 1, // opponent's spell targeted our creature
	})

	if oppCreature.MarkedDamage < 8 {
		t.Errorf("expected at least 8 marked damage on opp creature, got %d", oppCreature.MarkedDamage)
	}
	if gargos.MarkedDamage < 1 {
		t.Errorf("expected at least 1 marked damage on Gargos (mutual fight), got %d", gargos.MarkedDamage)
	}
}

// 7. Rakdos, Patron of Chaos — end_step draws 2 cards for the controller.
func TestStubsBatchN_RakdosPatron_EndStepDraws2(t *testing.T) {
	gs := newGame(t, 2)
	rakdos := addPerm(gs, 0, "Rakdos, Patron of Chaos", "creature", "legendary")
	rakdos.Card.BasePower = 6
	rakdos.Card.BaseToughness = 6
	addLibrary(gs, 0, "Card A", "Card B", "Card C")
	beforeHand := len(gs.Seats[0].Hand)

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})

	if len(gs.Seats[0].Hand)-beforeHand != 2 {
		t.Errorf("expected controller drew 2 cards, hand grew by %d", len(gs.Seats[0].Hand)-beforeHand)
	}
}

// 8. Mikey & Leo, Chaos & Order — counter_placed on friendly creature draws 1
//    (only first per turn).
func TestStubsBatchN_MikeyAndLeo_CounterPlacedDraws(t *testing.T) {
	gs := newGame(t, 2)
	mikey := addPerm(gs, 0, "Mikey & Leo, Chaos & Order", "creature", "legendary")
	mikey.Card.BasePower = 4
	mikey.Card.BaseToughness = 4
	target := addPerm(gs, 0, "Friendly Bear", "creature")
	addLibrary(gs, 0, "Topdeck1", "Topdeck2")
	beforeHand := len(gs.Seats[0].Hand)

	gameengine.FireCardTrigger(gs, "counter_placed", map[string]interface{}{
		"target_perm":  target,
		"target_seat":  0,
		"counter_kind": "+1/+1",
		"source_seat":  0,
		"amount":       1,
	})
	if len(gs.Seats[0].Hand)-beforeHand != 1 {
		t.Errorf("expected first counter_placed draws 1, got %d", len(gs.Seats[0].Hand)-beforeHand)
	}
	// Second counter same turn — no draw.
	gameengine.FireCardTrigger(gs, "counter_placed", map[string]interface{}{
		"target_perm":  target,
		"target_seat":  0,
		"counter_kind": "+1/+1",
		"source_seat":  0,
		"amount":       1,
	})
	if len(gs.Seats[0].Hand)-beforeHand != 1 {
		t.Errorf("expected only first counter draws (idempotent), got total grow=%d", len(gs.Seats[0].Hand)-beforeHand)
	}
}

// 9. Page, Loose Leaf — Grandeur tutors top inst/sorc from library to hand.
func TestStubsBatchN_PageLooseLeaf_GrandeurTutorsInstantOrSorcery(t *testing.T) {
	gs := newGame(t, 2)
	page := addPerm(gs, 0, "Page, Loose Leaf", "creature")
	// Discard cost: another Page in hand.
	otherPage := &gameengine.Card{Name: "Page, Loose Leaf", Types: []string{"creature"}, Owner: 0}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, otherPage)
	// Library: 2 non-matches, then a sorcery.
	addLibrary(gs, 0, "Mountain")
	addLibraryWithTypes(gs, 0, "Lightning Strike", []string{"instant"})
	addLibrary(gs, 0, "Forest")
	addLibraryWithTypes(gs, 0, "Wrath of God", []string{"sorcery"})

	beforeHand := len(gs.Seats[0].Hand)
	gameengine.InvokeActivatedHook(gs, page, 1, nil)

	// Hand should grow by 1 (the instant). The other-Page is discarded
	// (still net -0 for the Page count + 1 for the new instant), so
	// hand grows by 0 cards (1 added, 1 removed). Actually: started
	// with otherPage in hand, otherPage discarded, instant added →
	// net 0 change.
	if len(gs.Seats[0].Hand) != beforeHand {
		t.Errorf("expected hand net 0 change (Page discarded, instant added), got delta=%d", len(gs.Seats[0].Hand)-beforeHand)
	}
	// Verify the instant ended up in hand.
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c != nil && c.DisplayName() == "Lightning Strike" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Lightning Strike in hand after Grandeur tutor; hand=%v", cardNames(gs.Seats[0].Hand))
	}
}

// 10. Jan Jansen, Chaos Crafter — sacrifice artifact creature → 2 Treasures.
func TestStubsBatchN_JanJansen_SacArtifactCreatureMintsTreasures(t *testing.T) {
	gs := newGame(t, 2)
	jan := addPerm(gs, 0, "Jan Jansen, Chaos Crafter", "creature", "legendary")
	victim := addPerm(gs, 0, "Bomb Bot", "artifact", "creature")
	beforeBattlefieldLen := len(gs.Seats[0].Battlefield)

	gameengine.InvokeActivatedHook(gs, jan, 0, nil)

	if !jan.Tapped {
		t.Errorf("expected Jan tapped after activation")
	}
	// Victim sacrificed → graveyard.
	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			stillOnBF = true
		}
	}
	if stillOnBF {
		t.Errorf("expected victim sacrificed to graveyard")
	}
	// 2 Treasures minted.
	treasures := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "treasure") {
			treasures++
		}
	}
	if treasures != 2 {
		t.Errorf("expected 2 Treasures, got %d (battlefield len before=%d, after=%d)",
			treasures, beforeBattlefieldLen, len(gs.Seats[0].Battlefield))
	}
}
