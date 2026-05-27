package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// per_card_batch_k_r60_test.go — one regression per handler in
// per_card_batch_k_r60.go.

// ---------------------------------------------------------------------------
// Kambal, Consul of Allocation
// ---------------------------------------------------------------------------

func TestPerCardBatchK_Kambal_NoncreatureOppCastDrains(t *testing.T) {
	gs := newGame(t, 2)
	kambal := addPerm(gs, 0, "Kambal, Consul of Allocation", "creature", "legendary")
	kambal.Card.BasePower = 2
	kambal.Card.BaseToughness = 3
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	_ = kambal

	// Opp casts an instant — drain fires.
	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"card":        &gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}},
	})
	if gs.Seats[1].Life != 38 {
		t.Errorf("expected opp life 38 (40 - 2), got %d", gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != 42 {
		t.Errorf("expected controller life 42 (40 + 2), got %d", gs.Seats[0].Life)
	}

	// Opp casts a creature spell — no drain.
	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 1,
		"card":        &gameengine.Card{Name: "Bear", Types: []string{"creature"}},
	})
	if gs.Seats[1].Life != 38 || gs.Seats[0].Life != 42 {
		t.Errorf("expected creature cast to not trigger drain; opp=%d ctrl=%d",
			gs.Seats[1].Life, gs.Seats[0].Life)
	}

	// Own (non-opp) cast — no drain even on noncreature.
	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "Self Sorcery", Types: []string{"sorcery"}},
	})
	if gs.Seats[1].Life != 38 || gs.Seats[0].Life != 42 {
		t.Errorf("expected own cast to not trigger; opp=%d ctrl=%d",
			gs.Seats[1].Life, gs.Seats[0].Life)
	}
}

// ---------------------------------------------------------------------------
// Reyav, Master Smith
// ---------------------------------------------------------------------------

func TestPerCardBatchK_Reyav_EquippedAttackerGainsDoubleStrike(t *testing.T) {
	gs := newGame(t, 2)
	reyav := addPerm(gs, 0, "Reyav, Master Smith", "creature", "legendary")
	reyav.Card.BasePower = 2
	reyav.Card.BaseToughness = 2
	_ = reyav
	attacker := addPerm(gs, 0, "Knight", "creature")
	attacker.Card.BasePower = 2
	attacker.Card.BaseToughness = 2
	equip := addPerm(gs, 0, "Bonesplitter", "artifact", "equipment")
	equip.AttachedTo = attacker

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": attacker,
		"defender_seat": 1,
	})
	if attacker.Flags["kw:double_strike"] != 1 {
		t.Errorf("expected double_strike on equipped attacker, got %v", attacker.Flags)
	}

	// Unequipped attacker — no grant.
	plain := addPerm(gs, 0, "Bear", "creature")
	plain.Card.BasePower = 2
	plain.Card.BaseToughness = 2
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": plain,
		"defender_seat": 1,
	})
	if plain.Flags["kw:double_strike"] != 0 {
		t.Errorf("expected no double_strike on unequipped attacker, got %v", plain.Flags)
	}

	// Opp's equipped attacker — no grant (controller mismatch).
	oppCre := addPerm(gs, 1, "Bandit", "creature")
	oppCre.Card.BasePower = 2
	oppCre.Card.BaseToughness = 2
	oppEquip := addPerm(gs, 1, "Sword", "artifact", "equipment")
	oppEquip.AttachedTo = oppCre
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": oppCre,
		"defender_seat": 0,
	})
	if oppCre.Flags["kw:double_strike"] != 0 {
		t.Errorf("expected no grant for opp's attacker, got %v", oppCre.Flags)
	}
}

// ---------------------------------------------------------------------------
// April, Reporter of the Weird
// ---------------------------------------------------------------------------

func TestPerCardBatchK_April_CombatDamageDrawsThenDiscards(t *testing.T) {
	gs := newGame(t, 2)
	april := addPerm(gs, 0, "April, Reporter of the Weird", "creature", "legendary")
	april.Card.BasePower = 3
	april.Card.BaseToughness = 3
	addLibrary(gs, 0, "Top1", "Top2", "Top3", "Top4", "Top5")
	gs.Seats[0].Hand = []*gameengine.Card{}

	gameengine.FireCardTrigger(gs, "combat_damage_to_player", map[string]interface{}{
		"damager_perm": april,
		"damager_seat": 0,
		"target_seat":  1,
		"amount":       3,
	})
	// Drew 3, then discarded 1 → hand size 2.
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected hand size 2 (drew 3 then discarded 1), got %d (%v)",
			len(gs.Seats[0].Hand), gs.Seats[0].Hand)
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("expected 1 card in graveyard from discard, got %d", len(gs.Seats[0].Graveyard))
	}

	// Non-April attacker's damage: no fire.
	bear := addPerm(gs, 0, "Bear", "creature")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2
	pre := len(gs.Seats[0].Hand)
	gameengine.FireCardTrigger(gs, "combat_damage_to_player", map[string]interface{}{
		"damager_perm": bear,
		"damager_seat": 0,
		"target_seat":  1,
		"amount":       2,
	})
	if len(gs.Seats[0].Hand) != pre {
		t.Errorf("expected non-April attacker NOT to fire, hand went %d → %d",
			pre, len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Donatello, Rad Scientist
// ---------------------------------------------------------------------------

func TestPerCardBatchK_Donatello_ETBTapsAndStunsThreeOppCreatures(t *testing.T) {
	gs := newGame(t, 2)
	donatello := addPerm(gs, 0, "Donatello, Rad Scientist", "creature", "legendary")
	donatello.Card.BasePower = 3
	donatello.Card.BaseToughness = 3
	mkGoblin := func(name string) *gameengine.Permanent {
		p := addPerm(gs, 1, name, "creature")
		p.Card.BasePower = 1
		p.Card.BaseToughness = 1
		return p
	}
	a := mkGoblin("Goblin A")
	b := mkGoblin("Goblin B")
	c := mkGoblin("Goblin C")
	d := mkGoblin("Goblin D") // 4th — should NOT be touched

	gameengine.InvokeETBHook(gs, donatello)

	tappedCount := 0
	for _, p := range []*gameengine.Permanent{a, b, c} {
		if p.Tapped {
			tappedCount++
		}
		if p.Counters["stun"] != 1 {
			t.Errorf("expected stun counter on %s, got %v", p.Card.Name, p.Counters)
		}
	}
	if tappedCount != 3 {
		t.Errorf("expected 3 opp creatures tapped, got %d", tappedCount)
	}
	if d.Tapped || d.Counters["stun"] != 0 {
		t.Errorf("expected 4th creature untouched, tapped=%v counters=%v", d.Tapped, d.Counters)
	}

	// Own creatures must NOT be tapped.
	myCre := addPerm(gs, 0, "Friend", "creature")
	myCre.Card.BasePower = 2
	myCre.Card.BaseToughness = 2
	donatello2 := addPerm(gs, 0, "Donatello, Rad Scientist (copy)", "creature", "legendary")
	donatello2.Card.BasePower = 3
	donatello2.Card.BaseToughness = 3
	gameengine.InvokeETBHook(gs, donatello2)
	if myCre.Tapped || myCre.Counters["stun"] != 0 {
		t.Errorf("expected own creature untouched, tapped=%v counters=%v", myCre.Tapped, myCre.Counters)
	}
}

// ---------------------------------------------------------------------------
// Veronica, Dissident Scribe
// ---------------------------------------------------------------------------

func TestPerCardBatchK_Veronica_AttackTriggersDiscardThenDraw(t *testing.T) {
	gs := newGame(t, 2)
	veronica := addPerm(gs, 0, "Veronica, Dissident Scribe", "creature", "legendary")
	veronica.Card.BasePower = 3
	veronica.Card.BaseToughness = 2
	// Seed hand + library.
	gs.Seats[0].Hand = []*gameengine.Card{{Name: "Filler1"}, {Name: "Filler2"}}
	addLibrary(gs, 0, "Topdeck")

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": veronica,
		"defender_seat": 1,
	})
	// Hand: started with 2, discarded 1 → 1, then drew Topdeck → 2.
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected hand size 2 (-1 discard +1 draw), got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Graveyard) != 1 || gs.Seats[0].Graveyard[0].Name != "Filler1" {
		t.Errorf("expected Filler1 in graveyard, got %v", gs.Seats[0].Graveyard)
	}
	// Library now empty (Topdeck moved to hand).
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("expected empty library, got %d", len(gs.Seats[0].Library))
	}

	// Non-Veronica attacker — no fire.
	bear := addPerm(gs, 0, "Bear", "creature")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2
	preHand := len(gs.Seats[0].Hand)
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": bear,
		"defender_seat": 1,
	})
	if len(gs.Seats[0].Hand) != preHand {
		t.Errorf("expected non-Veronica attacker to not fire, hand %d → %d",
			preHand, len(gs.Seats[0].Hand))
	}

	// Empty hand → no discard, emitFail breadcrumb.
	gs.Seats[0].Hand = []*gameengine.Card{}
	addLibrary(gs, 0, "Topdeck2")
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": veronica,
		"defender_seat": 1,
	})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected hand still empty on empty-hand path, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected library untouched (no draw without discard), got %d", len(gs.Seats[0].Library))
	}
}
