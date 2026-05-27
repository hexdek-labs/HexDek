package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// per_card_batch_m_r60_test.go — one regression per handler in
// per_card_batch_m_r60.go. All P/T-stamped — see batch K commit note
// on the SBA 704.5f gotcha during stack-routed triggers.

// ---------------------------------------------------------------------------
// Vega, the Watcher
// ---------------------------------------------------------------------------

func TestPerCardBatchM_Vega_NonHandCastDrawsOne(t *testing.T) {
	gs := newGame(t, 2)
	vega := addPerm(gs, 0, "Vega, the Watcher", "creature", "legendary")
	vega.Card.BasePower = 3
	vega.Card.BaseToughness = 3
	addLibrary(gs, 0, "Top1", "Top2")

	// Cast from graveyard → draws.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"cast_zone":   "graveyard",
		"card":        &gameengine.Card{Name: "Flashbacked Spell", Types: []string{"sorcery"}},
	})
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 draw from graveyard cast, hand=%d", len(gs.Seats[0].Hand))
	}

	// Cast from hand — should NOT trigger.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"cast_zone":   "hand",
		"card":        &gameengine.Card{Name: "Bolt", Types: []string{"instant"}},
	})
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected hand-cast to NOT trigger, hand=%d", len(gs.Seats[0].Hand))
	}

	// Opp cast from exile — should NOT trigger (controller mismatch).
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"cast_zone":   "exile",
		"card":        &gameengine.Card{Name: "Opp Spell", Types: []string{"sorcery"}},
	})
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected opp's non-hand cast to NOT trigger Vega, hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Toothy, Imaginary Friend
// ---------------------------------------------------------------------------

func TestPerCardBatchM_Toothy_OnDrawAddsCounter(t *testing.T) {
	gs := newGame(t, 2)
	toothy := addPerm(gs, 0, "Toothy, Imaginary Friend", "creature", "legendary")
	toothy.Card.BasePower = 1
	toothy.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{"seat": 0})
	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{"seat": 0})
	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{"seat": 0})

	if toothy.Counters["+1/+1"] != 3 {
		t.Errorf("expected 3 +1/+1 counters, got %d", toothy.Counters["+1/+1"])
	}

	// Opp draws — no counter.
	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{"seat": 1})
	if toothy.Counters["+1/+1"] != 3 {
		t.Errorf("expected opp draw not to add counter, got %d", toothy.Counters["+1/+1"])
	}
}

// ---------------------------------------------------------------------------
// Alandra, Sky Dreamer
// ---------------------------------------------------------------------------

func TestPerCardBatchM_Alandra_SecondDrawMintsDrakeOnly(t *testing.T) {
	gs := newGame(t, 2)
	alandra := addPerm(gs, 0, "Alandra, Sky Dreamer", "creature", "legendary")
	alandra.Card.BasePower = 3
	alandra.Card.BaseToughness = 3

	// First draw of the turn — no Drake.
	gs.Seats[0].Turn.CardsDrawn = 1
	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{"seat": 0})
	if countTokensOnSeat(gs, 0, "Drake") != 0 {
		t.Errorf("expected NO Drake on first draw, got %d", countTokensOnSeat(gs, 0, "Drake"))
	}

	// Second draw — Drake mints.
	gs.Seats[0].Turn.CardsDrawn = 2
	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{"seat": 0})
	if countTokensOnSeat(gs, 0, "Drake") != 1 {
		t.Errorf("expected 1 Drake on second draw, got %d", countTokensOnSeat(gs, 0, "Drake"))
	}

	// Third draw — no additional Drake.
	gs.Seats[0].Turn.CardsDrawn = 3
	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{"seat": 0})
	if countTokensOnSeat(gs, 0, "Drake") != 1 {
		t.Errorf("expected still 1 Drake on third draw, got %d", countTokensOnSeat(gs, 0, "Drake"))
	}

	// Opp's second draw — no Drake on Alandra's side.
	gs.Seats[1].Turn.CardsDrawn = 2
	gameengine.FireCardTrigger(gs, "card_drawn", map[string]interface{}{"seat": 1})
	if countTokensOnSeat(gs, 0, "Drake") != 1 {
		t.Errorf("expected opp's draw to not mint Drake for us, got %d", countTokensOnSeat(gs, 0, "Drake"))
	}
}

func countTokensOnSeat(gs *gameengine.GameState, seat int, name string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Tuvasa the Sunlit
// ---------------------------------------------------------------------------

func TestPerCardBatchM_Tuvasa_FirstEnchantmentEachTurnDraws(t *testing.T) {
	gs := newGame(t, 2)
	tuvasa := addPerm(gs, 0, "Tuvasa the Sunlit", "creature", "legendary")
	tuvasa.Card.BasePower = 3
	tuvasa.Card.BaseToughness = 3
	addLibrary(gs, 0, "Top1", "Top2", "Top3", "Top4")
	gs.Turn = 5

	// First enchantment of the turn → draw.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Aura A", Types: []string{"enchantment", "aura"}},
	})
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 draw after first enchantment, hand=%d", len(gs.Seats[0].Hand))
	}

	// Second enchantment same turn → no extra draw.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Aura B", Types: []string{"enchantment"}},
	})
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected second enchantment same turn NOT to draw, hand=%d", len(gs.Seats[0].Hand))
	}

	// Non-enchantment cast — no draw.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Bear", Types: []string{"creature"}},
	})
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected creature cast NOT to draw, hand=%d", len(gs.Seats[0].Hand))
	}

	// Next turn — first enchantment of new turn draws again.
	gs.Turn = 6
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Aura C", Types: []string{"enchantment"}},
	})
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected new-turn first enchantment to draw, hand=%d", len(gs.Seats[0].Hand))
	}

	// Opp's enchantment cast — never triggers Tuvasa.
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"card":        &gameengine.Card{Name: "Opp Aura", Types: []string{"enchantment"}},
	})
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected opp enchantment NOT to fire Tuvasa, hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Grolnok, the Omnivore
// ---------------------------------------------------------------------------

func TestPerCardBatchM_Grolnok_FrogAttackMillsThree(t *testing.T) {
	gs := newGame(t, 2)
	grolnok := addPerm(gs, 0, "Grolnok, the Omnivore", "creature", "legendary")
	grolnok.Card.BasePower = 4
	grolnok.Card.BaseToughness = 4
	addLibrary(gs, 0, "L1", "L2", "L3", "L4", "L5")
	frog := addPerm(gs, 0, "Croaking Counterpart Token", "creature", "frog")
	frog.Card.BasePower = 1
	frog.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": frog,
		"defender_seat": 1,
	})
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected library size 2 after mill 3, got %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Graveyard) != 3 {
		t.Errorf("expected 3 cards milled to graveyard, got %d", len(gs.Seats[0].Graveyard))
	}

	// Non-Frog attacker — no mill.
	bear := addPerm(gs, 0, "Bear", "creature", "bear")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2
	preGY := len(gs.Seats[0].Graveyard)
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": bear,
		"defender_seat": 1,
	})
	if len(gs.Seats[0].Graveyard) != preGY {
		t.Errorf("expected non-Frog attacker to NOT mill, graveyard went %d → %d",
			preGY, len(gs.Seats[0].Graveyard))
	}

	// Opp's Frog attacker — no mill on our library.
	oppFrog := addPerm(gs, 1, "Opp Frog", "creature", "frog")
	oppFrog.Card.BasePower = 1
	oppFrog.Card.BaseToughness = 1
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": oppFrog,
		"defender_seat": 0,
	})
	if len(gs.Seats[0].Graveyard) != preGY {
		t.Errorf("expected opp's Frog to NOT mill us, graveyard went %d → %d",
			preGY, len(gs.Seats[0].Graveyard))
	}
}
