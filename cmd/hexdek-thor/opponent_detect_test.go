package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// DetectOpponentRequirements — oracle text pattern detection
// ---------------------------------------------------------------------------

func TestDetectOpponentRequirements_Creatures(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"destroy target creature opp controls", "Destroy target creature an opponent controls.", true},
		{"creatures your opponents control", "All creatures your opponents control get -1/-1.", true},
		{"each opponent's creature", "Exile each opponent's creature.", true},
		{"nonland permanent", "Destroy target nonland permanent an opponent controls.", true},
		{"permanent opp controls", "Gain control of target permanent an opponent controls.", true},
		{"your own creature", "Sacrifice a creature you control.", false},
		{"no opponent ref", "Destroy target creature.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsCreatures != tc.wants {
				t.Errorf("NeedsCreatures: got %v, want %v", req.NeedsCreatures, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Artifacts(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"artifact opp controls", "Destroy target artifact an opponent controls.", true},
		{"artifacts your opponents control", "Destroy all artifacts your opponents control.", true},
		{"nonland permanent", "Exile target nonland permanent an opponent controls.", true},
		{"your artifact", "Sacrifice an artifact.", false},
		{"no opponent", "Destroy target artifact.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsArtifacts != tc.wants {
				t.Errorf("NeedsArtifacts: got %v, want %v", req.NeedsArtifacts, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Enchantments(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"enchantment opp controls", "Destroy target enchantment an opponent controls.", true},
		{"nonland permanent", "Return target nonland permanent an opponent controls to its owner's hand.", true},
		{"your enchantment", "Destroy target enchantment you control.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsEnchantments != tc.wants {
				t.Errorf("NeedsEnchantments: got %v, want %v", req.NeedsEnchantments, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Life(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"each opponent loses", "Each opponent loses 2 life.", true},
		{"damage to opponent", "Deal 3 damage to an opponent.", true},
		{"deals damage to opp", "Whenever this creature deals damage to an opponent, draw a card.", true},
		{"opponent loses life", "Whenever an opponent loses life, you gain that much life.", true},
		{"you lose life", "You lose 3 life.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsLife != tc.wants {
				t.Errorf("NeedsLife: got %v, want %v", req.NeedsLife, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Cast(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"opponent casts", "Whenever an opponent casts a spell, draw a card.", true},
		{"opponents cast", "Whenever opponents cast spells, scry 1.", true},
		{"you cast", "Whenever you cast a spell, put a charge counter on this.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsCast != tc.wants {
				t.Errorf("NeedsCast: got %v, want %v", req.NeedsCast, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Attack(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"opponent attacks", "Whenever an opponent attacks, create a 1/1 token.", true},
		{"opponents attack", "Whenever opponents attack you, gain 1 life.", true},
		{"you attack", "Whenever you attack, draw a card.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsAttack != tc.wants {
				t.Errorf("NeedsAttack: got %v, want %v", req.NeedsAttack, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Hand(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"opponent discards", "Each opponent discards a card.", true},
		{"opponent's hand", "Look at target opponent's hand.", true},
		{"your hand", "Draw a card.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsHand != tc.wants {
				t.Errorf("NeedsHand: got %v, want %v", req.NeedsHand, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Graveyard(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"opp graveyard", "Exile target card from an opponent's graveyard.", true},
		{"opponents graveyards", "Exile all cards from opponents' graveyards.", true},
		{"opponent mills", "Each opponent mills three cards.", true},
		{"your graveyard", "Return target creature from your graveyard to your hand.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsGraveyard != tc.wants {
				t.Errorf("NeedsGraveyard: got %v, want %v", req.NeedsGraveyard, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Library(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		wants bool
	}{
		{"opp library", "Look at the top of an opponent's library.", true},
		{"opponent mills", "Each opponent mills five cards.", true},
		{"your library", "Search your library for a card.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := DetectOpponentRequirements(tc.text)
			if req.NeedsLibrary != tc.wants {
				t.Errorf("NeedsLibrary: got %v, want %v", req.NeedsLibrary, tc.wants)
			}
		})
	}
}

func TestDetectOpponentRequirements_Planeswalker(t *testing.T) {
	req := DetectOpponentRequirements("Destroy target planeswalker an opponent controls.")
	if !req.NeedsPlaneswalker {
		t.Error("NeedsPlaneswalker: expected true")
	}
}

func TestDetectOpponentRequirements_Land(t *testing.T) {
	req := DetectOpponentRequirements("Destroy target land an opponent controls.")
	if !req.NeedsLand {
		t.Error("NeedsLand: expected true")
	}
}

func TestDetectOpponentRequirements_NoOpponentRef(t *testing.T) {
	req := DetectOpponentRequirements("Draw two cards. You lose 2 life.")
	if req.HasAny() {
		t.Error("expected HasAny() == false for text with no opponent reference")
	}
}

func TestDetectOpponentRequirements_MultipleNeeds(t *testing.T) {
	text := "Destroy target creature an opponent controls. Each opponent discards a card. Each opponent mills two cards."
	req := DetectOpponentRequirements(text)
	if !req.NeedsCreatures {
		t.Error("NeedsCreatures expected")
	}
	if !req.NeedsHand {
		t.Error("NeedsHand expected")
	}
	if !req.NeedsGraveyard {
		t.Error("NeedsGraveyard expected (mills)")
	}
	if !req.NeedsLibrary {
		t.Error("NeedsLibrary expected (mills)")
	}
}

func TestDetectOpponentRequirements_HasAny(t *testing.T) {
	empty := OpponentRequirement{}
	if empty.HasAny() {
		t.Error("zero-value should return HasAny() == false")
	}
	withCreatures := OpponentRequirement{NeedsCreatures: true}
	if !withCreatures.HasAny() {
		t.Error("NeedsCreatures=true should return HasAny() == true")
	}
}

// ---------------------------------------------------------------------------
// EnrichOpponentSeat — board-state materialization
// ---------------------------------------------------------------------------

func makeTestGameStateForEnrich(seats int) *gameengine.GameState {
	gs := &gameengine.GameState{
		Turn:   1,
		Active: 0,
		Phase:  "precombat_main",
		Flags:  map[string]int{},
	}
	for i := 0; i < seats; i++ {
		seat := &gameengine.Seat{
			Life:  40,
			Flags: map[string]int{},
		}
		gs.Seats = append(gs.Seats, seat)
	}
	return gs
}

func TestEnrichOpponentSeat_Creatures(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsCreatures: true}
	EnrichOpponentSeat(gs, 1, req)

	creatures := countPermanentsByType(gs.Seats[1], "creature")
	if creatures < 3 {
		t.Errorf("expected at least 3 creatures on seat 1, got %d", creatures)
	}
}

func TestEnrichOpponentSeat_Artifacts(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsArtifacts: true}
	EnrichOpponentSeat(gs, 1, req)

	artifacts := countPermanentsByType(gs.Seats[1], "artifact")
	if artifacts < 1 {
		t.Errorf("expected at least 1 artifact on seat 1, got %d", artifacts)
	}
}

func TestEnrichOpponentSeat_Enchantments(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsEnchantments: true}
	EnrichOpponentSeat(gs, 1, req)

	enchantments := countPermanentsByType(gs.Seats[1], "enchantment")
	if enchantments < 1 {
		t.Errorf("expected at least 1 enchantment on seat 1, got %d", enchantments)
	}
}

func TestEnrichOpponentSeat_Planeswalker(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsPlaneswalker: true}
	EnrichOpponentSeat(gs, 1, req)

	pws := countPermanentsByType(gs.Seats[1], "planeswalker")
	if pws < 1 {
		t.Errorf("expected at least 1 planeswalker on seat 1, got %d", pws)
	}
	// Check loyalty counters.
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card != nil {
			for _, ty := range p.Card.Types {
				if ty == "planeswalker" {
					if p.Counters == nil || p.Counters["loyalty"] < 1 {
						t.Error("planeswalker should have loyalty counters")
					}
				}
			}
		}
	}
}

func TestEnrichOpponentSeat_Land(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsLand: true}
	EnrichOpponentSeat(gs, 1, req)

	lands := countPermanentsByType(gs.Seats[1], "land")
	if lands < 3 {
		t.Errorf("expected at least 3 lands on seat 1, got %d", lands)
	}
}

func TestEnrichOpponentSeat_Hand(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsHand: true}
	EnrichOpponentSeat(gs, 1, req)

	if len(gs.Seats[1].Hand) < 5 {
		t.Errorf("expected at least 5 hand cards on seat 1, got %d", len(gs.Seats[1].Hand))
	}
}

func TestEnrichOpponentSeat_Graveyard(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsGraveyard: true}
	EnrichOpponentSeat(gs, 1, req)

	if len(gs.Seats[1].Graveyard) < 4 {
		t.Errorf("expected at least 4 graveyard cards on seat 1, got %d", len(gs.Seats[1].Graveyard))
	}
}

func TestEnrichOpponentSeat_Library(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsLibrary: true}
	EnrichOpponentSeat(gs, 1, req)

	if len(gs.Seats[1].Library) < 10 {
		t.Errorf("expected at least 10 library cards on seat 1, got %d", len(gs.Seats[1].Library))
	}
}

func TestEnrichOpponentSeat_Life(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	gs.Seats[1].Life = 5 // low life
	req := OpponentRequirement{NeedsLife: true}
	EnrichOpponentSeat(gs, 1, req)

	if gs.Seats[1].Life < 20 {
		t.Errorf("expected life >= 20, got %d", gs.Seats[1].Life)
	}
}

func TestEnrichOpponentSeat_Idempotent(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{NeedsCreatures: true, NeedsArtifacts: true}

	// First enrichment.
	EnrichOpponentSeat(gs, 1, req)
	creaturesAfterFirst := countPermanentsByType(gs.Seats[1], "creature")
	artifactsAfterFirst := countPermanentsByType(gs.Seats[1], "artifact")
	totalAfterFirst := len(gs.Seats[1].Battlefield)

	// Second enrichment — should not add more.
	EnrichOpponentSeat(gs, 1, req)
	creaturesAfterSecond := countPermanentsByType(gs.Seats[1], "creature")
	artifactsAfterSecond := countPermanentsByType(gs.Seats[1], "artifact")
	totalAfterSecond := len(gs.Seats[1].Battlefield)

	if creaturesAfterSecond != creaturesAfterFirst {
		t.Errorf("creature count changed: %d -> %d (should be idempotent)",
			creaturesAfterFirst, creaturesAfterSecond)
	}
	if artifactsAfterSecond != artifactsAfterFirst {
		t.Errorf("artifact count changed: %d -> %d (should be idempotent)",
			artifactsAfterFirst, artifactsAfterSecond)
	}
	if totalAfterSecond != totalAfterFirst {
		t.Errorf("total battlefield changed: %d -> %d (should be idempotent)",
			totalAfterFirst, totalAfterSecond)
	}
}

func TestEnrichOpponentSeat_NilSafety(t *testing.T) {
	// "Doesn't panic" assertion made explicit — pre-Phase-2B this
	// test relied on the absence of a runtime panic without any
	// recover() guard, which the Phase 2B audit flagged as a no-
	// assertion test. The defer makes the assertion explicit.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EnrichOpponentSeat must not panic; got %v", r)
		}
	}()

	EnrichOpponentSeat(nil, 1, OpponentRequirement{NeedsCreatures: true})
	gs := makeTestGameStateForEnrich(2)
	EnrichOpponentSeat(gs, 5, OpponentRequirement{NeedsCreatures: true})
	EnrichOpponentSeat(gs, 1, OpponentRequirement{})
}

func TestEnrichOpponentSeat_MultipleRequirements(t *testing.T) {
	gs := makeTestGameStateForEnrich(2)
	req := OpponentRequirement{
		NeedsCreatures:    true,
		NeedsArtifacts:    true,
		NeedsEnchantments: true,
		NeedsHand:         true,
		NeedsGraveyard:    true,
		NeedsLibrary:      true,
	}
	EnrichOpponentSeat(gs, 1, req)

	creatures := countPermanentsByType(gs.Seats[1], "creature")
	if creatures < 3 {
		t.Errorf("expected >= 3 creatures, got %d", creatures)
	}
	artifacts := countPermanentsByType(gs.Seats[1], "artifact")
	if artifacts < 1 {
		t.Errorf("expected >= 1 artifact, got %d", artifacts)
	}
	enchantments := countPermanentsByType(gs.Seats[1], "enchantment")
	if enchantments < 1 {
		t.Errorf("expected >= 1 enchantment, got %d", enchantments)
	}
	if len(gs.Seats[1].Hand) < 5 {
		t.Errorf("expected >= 5 hand cards, got %d", len(gs.Seats[1].Hand))
	}
	if len(gs.Seats[1].Graveyard) < 4 {
		t.Errorf("expected >= 4 graveyard cards, got %d", len(gs.Seats[1].Graveyard))
	}
	if len(gs.Seats[1].Library) < 10 {
		t.Errorf("expected >= 10 library cards, got %d", len(gs.Seats[1].Library))
	}
}

// ---------------------------------------------------------------------------
// Integration: DetectOpponentRequirements -> EnrichOpponentSeat
// ---------------------------------------------------------------------------

func TestEndToEnd_DestroyCreatureOpponentControls(t *testing.T) {
	oracleText := "Destroy target creature an opponent controls."
	req := DetectOpponentRequirements(oracleText)
	if !req.NeedsCreatures {
		t.Fatal("expected NeedsCreatures for 'destroy target creature an opponent controls'")
	}

	gs := makeTestGameStateForEnrich(4)
	EnrichOpponentSeat(gs, 1, req)

	creatures := countPermanentsByType(gs.Seats[1], "creature")
	if creatures < 1 {
		t.Errorf("expected opponent to have creatures after enrichment, got %d", creatures)
	}
}

func TestEndToEnd_EachOpponentDiscards(t *testing.T) {
	oracleText := "Each opponent discards two cards."
	req := DetectOpponentRequirements(oracleText)
	if !req.NeedsHand {
		t.Fatal("expected NeedsHand for 'each opponent discards'")
	}

	gs := makeTestGameStateForEnrich(4)
	for i := 1; i < 4; i++ {
		EnrichOpponentSeat(gs, i, req)
		if len(gs.Seats[i].Hand) < 5 {
			t.Errorf("seat %d: expected >= 5 hand cards, got %d", i, len(gs.Seats[i].Hand))
		}
	}
}

func TestEndToEnd_ExileFromOpponentGraveyard(t *testing.T) {
	oracleText := "Exile target card from an opponent's graveyard."
	req := DetectOpponentRequirements(oracleText)
	if !req.NeedsGraveyard {
		t.Fatal("expected NeedsGraveyard")
	}

	gs := makeTestGameStateForEnrich(2)
	EnrichOpponentSeat(gs, 1, req)

	if len(gs.Seats[1].Graveyard) < 1 {
		t.Error("expected graveyard cards after enrichment")
	}
}
