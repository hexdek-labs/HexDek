package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// creature_mutated_consumers_r60 — 6 handlers wired in
// creature_mutated_consumers_r60.go covering IKO Mutate triggers
// (CR §702.139).
// -----------------------------------------------------------------------------

func mutatedCtx(merged *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"controller_seat": merged.Controller,
		"mutated_perm":    merged,
	}
}

// -----------------------------------------------------------------------------
// Dreamtail Heron — draw on mutate
// -----------------------------------------------------------------------------

func TestDreamtailHeron_DrawsOnMutate(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top")
	heron := addPerm(gs, 0, "Dreamtail Heron", "creature")
	heron.Card.BasePower, heron.Card.BaseToughness = 3, 4

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(heron))

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn, got hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestDreamtailHeron_IgnoresOtherMutateEvent(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top")
	heron := addPerm(gs, 0, "Dreamtail Heron", "creature")
	heron.Card.BasePower, heron.Card.BaseToughness = 3, 4

	// A DIFFERENT creature just mutated — heron's handler must NOT
	// fire (oracle: "whenever this creature mutates").
	other := addPerm(gs, 0, "Cubwarden", "creature")
	other.Card.BasePower, other.Card.BaseToughness = 2, 2

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(other))

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("heron must not fire on another creature's mutate; hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Cubwarden — two 1/1 Cat tokens on mutate
// -----------------------------------------------------------------------------

func TestCubwarden_CreatesTwoCatsOnMutate(t *testing.T) {
	gs := newGame(t, 2)
	cub := addPerm(gs, 0, "Cubwarden", "creature")
	cub.Card.BasePower, cub.Card.BaseToughness = 2, 2
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(cub))

	if len(gs.Seats[0].Battlefield) != pre+2 {
		t.Errorf("expected battlefield +2 (Cat tokens), pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	cats := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Cat" {
			cats++
		}
	}
	if cats != 2 {
		t.Errorf("expected 2 Cat tokens, got %d", cats)
	}
}

// -----------------------------------------------------------------------------
// Majestic Auricorn — gain 4 life on mutate
// -----------------------------------------------------------------------------

func TestMajesticAuricorn_GainsFourLifeOnMutate(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	auri := addPerm(gs, 0, "Majestic Auricorn", "creature")
	auri.Card.BasePower, auri.Card.BaseToughness = 4, 4

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(auri))

	if gs.Seats[0].Life != 24 {
		t.Errorf("expected 24 life, got %d", gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Cavern Whisperer — each opp discards a card
// -----------------------------------------------------------------------------

func TestCavernWhisperer_EachOppDiscardsOnMutate(t *testing.T) {
	gs := newGame(t, 4)
	whisper := addPerm(gs, 0, "Cavern Whisperer", "creature")
	whisper.Card.BasePower, whisper.Card.BaseToughness = 3, 3

	for i := 1; i < 4; i++ {
		gs.Seats[i].Hand = append(gs.Seats[i].Hand,
			&gameengine.Card{Name: "Opp Card", Owner: i, Types: []string{"creature"}})
	}

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(whisper))

	for i := 1; i < 4; i++ {
		if len(gs.Seats[i].Hand) != 0 {
			t.Errorf("opp %d should have discarded; hand=%d", i, len(gs.Seats[i].Hand))
		}
	}
}

func TestCavernWhisperer_SkipsEmptyHandsAndLostOpps(t *testing.T) {
	gs := newGame(t, 4)
	whisper := addPerm(gs, 0, "Cavern Whisperer", "creature")
	whisper.Card.BasePower, whisper.Card.BaseToughness = 3, 3

	// Opp 1 has a card.
	gs.Seats[1].Hand = append(gs.Seats[1].Hand,
		&gameengine.Card{Name: "Opp1 Card", Owner: 1, Types: []string{"creature"}})
	// Opp 2 is eliminated.
	gs.Seats[2].Lost = true
	gs.Seats[2].Hand = append(gs.Seats[2].Hand,
		&gameengine.Card{Name: "Ghost Card", Owner: 2, Types: []string{"creature"}})
	// Opp 3 has empty hand.

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(whisper))

	if len(gs.Seats[1].Hand) != 0 {
		t.Errorf("opp 1 should have discarded")
	}
	if len(gs.Seats[2].Hand) != 1 {
		t.Errorf("eliminated opp should be skipped; hand=%d", len(gs.Seats[2].Hand))
	}
}

// -----------------------------------------------------------------------------
// Glowstone Recluse — two +1/+1 counters on self
// -----------------------------------------------------------------------------

func TestGlowstoneRecluse_AddsTwoCountersOnMutate(t *testing.T) {
	gs := newGame(t, 2)
	spider := addPerm(gs, 0, "Glowstone Recluse", "creature")
	spider.Card.BasePower, spider.Card.BaseToughness = 2, 3

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(spider))

	if spider.Counters["+1/+1"] != 2 {
		t.Errorf("expected 2 +1/+1 counters, got %d", spider.Counters["+1/+1"])
	}
	if spider.Power() != 4 || spider.Toughness() != 5 {
		t.Errorf("expected 4/5 after +2/+2 from counters, got %d/%d", spider.Power(), spider.Toughness())
	}
}

func TestGlowstoneRecluse_StacksCountersAcrossMutations(t *testing.T) {
	gs := newGame(t, 2)
	spider := addPerm(gs, 0, "Glowstone Recluse", "creature")
	spider.Card.BasePower, spider.Card.BaseToughness = 2, 3

	for i := 0; i < 3; i++ {
		gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(spider))
	}
	if spider.Counters["+1/+1"] != 6 {
		t.Errorf("expected 6 counters after 3 mutations, got %d", spider.Counters["+1/+1"])
	}
}

// -----------------------------------------------------------------------------
// Gemrazer — destroy target artifact or enchantment opp controls
// -----------------------------------------------------------------------------

func TestGemrazer_DestroysHighestCMCOppArtifactOrEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	weak := addPerm(gs, 1, "Sol Ring", "artifact")
	weak.Card.Types = append(weak.Card.Types, "cmc:1")
	big := addPerm(gs, 1, "Smothering Tithe", "enchantment")
	big.Card.Types = append(big.Card.Types, "cmc:4")

	gem := addPerm(gs, 0, "Gemrazer", "creature")
	gem.Card.BasePower, gem.Card.BaseToughness = 4, 4

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(gem))

	// Smothering Tithe should be gone (highest CMC).
	for _, p := range gs.Seats[1].Battlefield {
		if p == big {
			t.Fatalf("Smothering Tithe should have been destroyed (highest CMC)")
		}
	}
	// Sol Ring should still be on battlefield.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == weak {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Errorf("Sol Ring (lower CMC) should not have been destroyed")
	}
}

func TestGemrazer_IgnoresOwnArtifacts(t *testing.T) {
	gs := newGame(t, 2)
	// Only friendly artifacts → no eligible target → fail.
	addPerm(gs, 0, "Sol Ring", "artifact")
	gem := addPerm(gs, 0, "Gemrazer", "creature")
	gem.Card.BasePower, gem.Card.BaseToughness = 4, 4

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(gem))

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no opp art/ench; events=%+v", gs.EventLog)
	}
}

func TestGemrazer_IgnoresOppCreatures(t *testing.T) {
	gs := newGame(t, 2)
	// Opp has only a creature (not artifact/enchantment) → no target.
	opp := addPerm(gs, 1, "Big Bear", "creature")
	opp.Card.BasePower, opp.Card.BaseToughness = 5, 5
	gem := addPerm(gs, 0, "Gemrazer", "creature")
	gem.Card.BasePower, gem.Card.BaseToughness = 4, 4

	gameengine.FireCardTrigger(gs, "creature_mutated", mutatedCtx(gem))

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("opp creature must not be a Gemrazer target; expected fail; events=%+v", gs.EventLog)
	}
	// Opp creature still on battlefield.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == opp {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Errorf("opp creature should not be destroyed")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — every handler reachable post-registerDefaults.
// -----------------------------------------------------------------------------

func TestCreatureMutatedConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Dreamtail Heron",
		"Cubwarden",
		"Majestic Auricorn",
		"Cavern Whisperer",
		"Glowstone Recluse",
		"Gemrazer",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler for %s", name)
		}
	}
}
