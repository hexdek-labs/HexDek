package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// dev/percard-stubs-batchA-r49 — 10 deck-pool stubs ported to real handlers.
// One regression test per port.
// ---------------------------------------------------------------------------

// 1. Alisaie Leveilleur — Partner-with tutor on ETB.
func TestStubsBatchA_AlisaieLeveilleur_PartnerWithTutorFindsAlphinaud(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Plains", "Alphinaud Leveilleur", "Mountain")
	alisaie := addPerm(gs, 0, "Alisaie Leveilleur", "creature", "legendary")

	gameengine.InvokeETBHook(gs, alisaie)

	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c != nil && c.DisplayName() == "Alphinaud Leveilleur" {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Errorf("expected Alphinaud Leveilleur in hand after Alisaie ETB; hand=%v", cardNames(gs.Seats[0].Hand))
	}
	for _, c := range gs.Seats[0].Library {
		if c != nil && c.DisplayName() == "Alphinaud Leveilleur" {
			t.Errorf("Alphinaud should be removed from library")
		}
	}
}

func TestStubsBatchA_AlisaieLeveilleur_NoAlphinaudShufflesAnyway(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Plains", "Mountain")
	alisaie := addPerm(gs, 0, "Alisaie Leveilleur", "creature", "legendary")

	gameengine.InvokeETBHook(gs, alisaie)

	// Hand stays empty; library size preserved.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected empty hand on whiff, got %v", cardNames(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected library size 2 (no card removed), got %d", len(gs.Seats[0].Library))
	}
}

// 2. The Earth King — power-4+ attackers trigger basic-land tutor.
func TestStubsBatchA_TheEarthKing_AttackPowerFourBasicLandSearch(t *testing.T) {
	gs := newGame(t, 2)
	king := addPerm(gs, 0, "The Earth King", "creature", "legendary")
	// Two attackers with power 4+ controlled by seat 0.
	atk1 := addPerm(gs, 0, "Bear A", "creature")
	atk1.Card.BasePower = 4
	atk1.Card.BaseToughness = 4
	atk1.Flags["attacking"] = 1
	atk2 := addPerm(gs, 0, "Bear B", "creature")
	atk2.Card.BasePower = 5
	atk2.Card.BaseToughness = 5
	atk2.Flags["attacking"] = 1
	// One sub-4 attacker that shouldn't count.
	atk3 := addPerm(gs, 0, "Small", "creature")
	atk3.Card.BasePower = 2
	atk3.Card.BaseToughness = 2
	atk3.Flags["attacking"] = 1

	addLibraryWithTypes(gs, 0, "Plains", []string{"basic", "land", "plains"})
	addLibraryWithTypes(gs, 0, "Forest", []string{"basic", "land", "forest"})
	addLibrary(gs, 0, "Lightning Bolt")
	addLibraryWithTypes(gs, 0, "Mountain", []string{"basic", "land", "mountain"})

	gameengine.FireCardTrigger(gs, "combat_attackers_declared", map[string]interface{}{
		"active_seat": 0,
	})
	_ = king

	tappedBasics := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "basic") && cardHasType(p.Card, "land") {
			tappedBasics++
			if !p.Tapped {
				t.Errorf("expected fetched basic %q tapped", p.Card.DisplayName())
			}
		}
	}
	if tappedBasics != 2 {
		t.Errorf("expected exactly 2 fetched basic lands (one per ≥4 attacker), got %d", tappedBasics)
	}
}

// 3. Galazeth Prismari — activated mana ability adds 1 mana + flag tag.
func TestStubsBatchA_GalazethPrismari_ActivatedTapAddsMana(t *testing.T) {
	gs := newGame(t, 2)
	gal := addPerm(gs, 0, "Galazeth Prismari", "creature", "legendary")
	before := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, gal, 0, nil)

	if !gal.Tapped {
		t.Errorf("expected Galazeth tapped after activation")
	}
	if gs.Seats[0].ManaPool != before+1 {
		t.Errorf("expected mana pool +1 (was %d, now %d)", before, gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Flags == nil || gs.Seats[0].Flags["galazeth_instant_sorcery_mana"] == 0 {
		t.Errorf("expected galazeth_instant_sorcery_mana seat flag set")
	}
}

// 4. The Dawning Archaic — self-cast cost reduction by inst/sorc count
//    and attack-trigger recurs target instant/sorcery to hand.
func TestStubsBatchA_TheDawningArchaic_SelfCastCostReduction(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}},
		&gameengine.Card{Name: "Counterspell", Types: []string{"instant"}},
		&gameengine.Card{Name: "Wrath of God", Types: []string{"sorcery"}},
		&gameengine.Card{Name: "Llanowar Elves", Types: []string{"creature"}}, // ignored
	)
	card := &gameengine.Card{Name: "The Dawning Archaic", Types: []string{"creature", "legendary", "cost:10"}}
	got := gameengine.CalculateTotalCost(gs, card, 0)
	// Base 10 minus 3 (instant/sorcery yard count) = 7.
	if got != 7 {
		t.Errorf("expected cost 7 (10 - 3 inst/sorc in yard), got %d", got)
	}
}

func TestStubsBatchA_TheDawningArchaic_AttackTriggerReturnsHighestCMCInstantSorcery(t *testing.T) {
	gs := newGame(t, 2)
	archaic := addPerm(gs, 0, "The Dawning Archaic", "creature", "legendary")
	archaic.Flags["attacking"] = 1
	// Yard: a low-CMC instant, a high-CMC sorcery, a creature card.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant", "cmc:1"}},
		&gameengine.Card{Name: "Tinkerer's Tome", Types: []string{"sorcery", "cmc:5"}},
		&gameengine.Card{Name: "Bear", Types: []string{"creature", "cmc:2"}},
	)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": archaic,
		"active_seat":   0,
	})

	foundTome := false
	for _, c := range gs.Seats[0].Hand {
		if c != nil && c.DisplayName() == "Tinkerer's Tome" {
			foundTome = true
		}
	}
	if !foundTome {
		t.Errorf("expected highest-CMC inst/sorc (Tinkerer's Tome) in hand; hand=%v", cardNames(gs.Seats[0].Hand))
	}
}

// 5. Eon Frolicker — bumps extra_turns_pending and stamps protection flag.
func TestStubsBatchA_EonFrolicker_CastBumpsExtraTurnAndStampsProtection(t *testing.T) {
	gs := newGame(t, 3)
	gs.Seats[1].Life = 12
	gs.Seats[2].Life = 30 // higher; should NOT be the target
	frolicker := addPerm(gs, 0, "Eon Frolicker", "creature")
	frolicker.Flags["was_cast"] = 1

	beforeQueue := gs.Flags["extra_turns_pending"]
	gameengine.InvokeETBHook(gs, frolicker)

	if got := gs.Flags["extra_turns_pending"]; got != beforeQueue+1 {
		t.Errorf("expected extra_turns_pending +1, was %d now %d", beforeQueue, got)
	}
	// Target = seat 1 (lowest-life opponent), encoded as target+1.
	if got := gs.Flags["extra_turn_target_seat"]; got != 2 {
		t.Errorf("expected extra_turn_target_seat=2 (seat 1 +1 offset), got %d", got)
	}
	ct := gs.Seats[0].Flags
	if ct == nil || ct["eon_frolicker_protection_from_seat"] != 2 {
		t.Errorf("expected protection flag pointing at seat 1 (+1 offset)")
	}
}

func TestStubsBatchA_EonFrolicker_NotCastDoesNothing(t *testing.T) {
	gs := newGame(t, 2)
	frolicker := addPerm(gs, 0, "Eon Frolicker", "creature")
	// no was_cast flag
	gameengine.InvokeETBHook(gs, frolicker)
	if gs.Flags["extra_turns_pending"] != 0 {
		t.Errorf("expected no extra turn when not cast, got %d", gs.Flags["extra_turns_pending"])
	}
}

// 6. Magnus the Red — Unearthly Power cost reduction (per creature token controlled).
func TestStubsBatchA_MagnusTheRed_UnearthlyPowerReducesByTokenCount(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	magnus := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Magnus the Red", Types: []string{"creature", "legendary"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, magnus)
	// 3 creature tokens
	for i := 0; i < 3; i++ {
		tok := &gameengine.Permanent{
			Card:       &gameengine.Card{Name: "Spawn Token", Types: []string{"token", "creature"}},
			Controller: 0,
			Owner:      0,
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tok)
	}
	bolt := &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	cost := gameengine.CalculateTotalCost(gs, bolt, 0)
	if cost != 0 {
		t.Errorf("expected cost 0 (1 base - 3 tokens, floored at 0), got %d", cost)
	}
	// Creature spell is unaffected (Unearthly Power is instant/sorcery only).
	bear := &gameengine.Card{Name: "Grizzly Bears", Types: []string{"creature", "cost:2"}}
	cost = gameengine.CalculateTotalCost(gs, bear, 0)
	if cost != 2 {
		t.Errorf("expected creature cost 2 (Unearthly Power doesn't reduce creatures), got %d", cost)
	}
}

// 7. Lyse Hext — noncreature cost reduction.
func TestStubsBatchA_LyseHext_NoncreatureSpellsCostOneLess(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	lyse := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Lyse Hext", Types: []string{"creature", "legendary"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, lyse)
	bolt := &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:3"}}
	cost := gameengine.CalculateTotalCost(gs, bolt, 0)
	if cost != 2 {
		t.Errorf("expected cost 2 (3 - 1 Lyse), got %d", cost)
	}
	// Creature spell is unaffected.
	bear := &gameengine.Card{Name: "Grizzly Bears", Types: []string{"creature", "cost:2"}}
	cost = gameengine.CalculateTotalCost(gs, bear, 0)
	if cost != 2 {
		t.Errorf("expected creature cost 2 (Lyse doesn't reduce creatures), got %d", cost)
	}
}

// 8. The Ancient One — descend 8 gate toggles cant_attack / cant_block.
func TestStubsBatchA_TheAncientOne_DescendGateLocksUnderEight(t *testing.T) {
	gs := newGame(t, 2)
	ancient := addPerm(gs, 0, "The Ancient One", "creature", "legendary")
	// Yard has only 3 permanent cards.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Bear A", Types: []string{"creature"}},
		&gameengine.Card{Name: "Bear B", Types: []string{"creature"}},
		&gameengine.Card{Name: "Plains", Types: []string{"land"}},
	)

	gameengine.InvokeETBHook(gs, ancient)
	if ancient.Flags["cant_attack"] != 1 || ancient.Flags["cant_block"] != 1 {
		t.Errorf("expected cant_attack/cant_block set when yard<8, got %v", ancient.Flags)
	}
}

func TestStubsBatchA_TheAncientOne_DescendGateUnlocksAtEight(t *testing.T) {
	gs := newGame(t, 2)
	ancient := addPerm(gs, 0, "The Ancient One", "creature", "legendary")
	// Yard with 8 permanent cards.
	for i := 0; i < 8; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			&gameengine.Card{Name: "Bear", Types: []string{"creature"}})
	}

	gameengine.InvokeETBHook(gs, ancient)
	if ancient.Flags["cant_attack"] != 0 || ancient.Flags["cant_block"] != 0 {
		t.Errorf("expected cant_attack/cant_block cleared when yard>=8, got %v", ancient.Flags)
	}
}

// 9. Lorehold Archivist — end_step clears the prepared flag.
func TestStubsBatchA_LoreholdArchivist_EndStepUnprepares(t *testing.T) {
	gs := newGame(t, 2)
	arch := addPerm(gs, 0, "Lorehold Archivist", "creature", "legendary")
	arch.Flags["prepared"] = 1

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 0,
	})
	if arch.Flags["prepared"] != 0 {
		t.Errorf("expected prepared flag cleared at end_step, still set")
	}
}

// 10. Sunderflock — self-cast cost reduction by greatest Elemental MV controlled.
func TestStubsBatchA_Sunderflock_CostReductionByGreatestElementalMV(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
		&gameengine.Permanent{
			Card:       &gameengine.Card{Name: "Mulldrifter", Types: []string{"creature", "elemental", "cost:5"}},
			Controller: 0,
			Owner:      0,
		},
		&gameengine.Permanent{
			Card:       &gameengine.Card{Name: "Snapcaster Mage", Types: []string{"creature", "wizard", "cost:2"}}, // not elemental
			Controller: 0,
			Owner:      0,
		},
		&gameengine.Permanent{
			Card:       &gameengine.Card{Name: "Spitebellows", Types: []string{"creature", "elemental", "cost:7"}},
			Controller: 0,
			Owner:      0,
		},
	)
	sunder := &gameengine.Card{Name: "Sunderflock", Types: []string{"creature", "elemental", "cost:9"}}
	cost := gameengine.CalculateTotalCost(gs, sunder, 0)
	// Base 9 - 7 (greatest elemental MV) = 2.
	if cost != 2 {
		t.Errorf("expected cost 2 (9 - 7 max elem MV), got %d", cost)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func cardNames(cards []*gameengine.Card) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		if c == nil {
			continue
		}
		out = append(out, c.DisplayName())
	}
	return out
}
