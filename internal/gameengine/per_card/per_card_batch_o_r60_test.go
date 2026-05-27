package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// per_card_batch_o_r60_test.go — one regression per handler in
// per_card_batch_o_r60.go. P/T stamped on every test perm so SBA
// 704.5f doesn't kill 0/0 sources between trigger push and resolve.

// ---------------------------------------------------------------------------
// Saradoc, Master of Buckland
// ---------------------------------------------------------------------------

func TestPerCardBatchO_Saradoc_ETBMintsAndPower2ETBMintsButPower3Doesnt(t *testing.T) {
	gs := newGame(t, 2)
	saradoc := addPerm(gs, 0, "Saradoc, Master of Buckland", "creature", "legendary", "halfling")
	saradoc.Card.BasePower = 2
	saradoc.Card.BaseToughness = 3

	// Saradoc's own ETB mints a Halfling token.
	gameengine.InvokeETBHook(gs, saradoc)
	if c := countTokensOnSeat(gs, 0, "Halfling"); c != 1 {
		t.Errorf("expected 1 Halfling from Saradoc's own ETB, got %d", c)
	}

	// Power-2 nontoken creature ETBs — mints another Halfling.
	weak := addPerm(gs, 0, "Tiny Hero", "creature")
	weak.Card.BasePower = 2
	weak.Card.BaseToughness = 2
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            weak,
		"controller_seat": 0,
	})
	if c := countTokensOnSeat(gs, 0, "Halfling"); c != 2 {
		t.Errorf("expected 2 Halflings after power-2 ETB, got %d", c)
	}

	// Power-3 creature ETBs — no mint (over threshold).
	beefy := addPerm(gs, 0, "Big Hero", "creature")
	beefy.Card.BasePower = 3
	beefy.Card.BaseToughness = 3
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            beefy,
		"controller_seat": 0,
	})
	if c := countTokensOnSeat(gs, 0, "Halfling"); c != 2 {
		t.Errorf("expected NO additional Halfling for power-3, got %d total", c)
	}

	// Opp's power-2 creature ETBs — no mint on our side.
	oppCre := addPerm(gs, 1, "Opp Squirrel", "creature")
	oppCre.Card.BasePower = 1
	oppCre.Card.BaseToughness = 1
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            oppCre,
		"controller_seat": 1,
	})
	if c := countTokensOnSeat(gs, 0, "Halfling"); c != 2 {
		t.Errorf("expected opp ETB to NOT mint for Saradoc, got %d", c)
	}
}

// ---------------------------------------------------------------------------
// Sokka, Lateral Strategist
// ---------------------------------------------------------------------------

func TestPerCardBatchO_Sokka_GroupAttackDrawsOnlyWithOtherAttacker(t *testing.T) {
	gs := newGame(t, 2)
	sokka := addPerm(gs, 0, "Sokka, Lateral Strategist", "creature", "legendary")
	sokka.Card.BasePower = 2
	sokka.Card.BaseToughness = 3
	addLibrary(gs, 0, "Top1", "Top2")
	sokka.Flags["attacking"] = 1

	// Sokka attacks alone — no other attacker on bf — no draw.
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": sokka,
		"defender_seat": 1,
	})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected NO draw when Sokka attacks alone, hand=%d", len(gs.Seats[0].Hand))
	}

	// Add a second attacker — now Sokka's attack trigger draws.
	wingman := addPerm(gs, 0, "Wingman", "creature")
	wingman.Card.BasePower = 2
	wingman.Card.BaseToughness = 2
	wingman.Flags["attacking"] = 1
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": sokka,
		"defender_seat": 1,
	})
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 draw on group attack, hand=%d", len(gs.Seats[0].Hand))
	}

	// Wingman is the attacker (Sokka not the trigger source) — no fire.
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": wingman,
		"defender_seat": 1,
	})
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected non-Sokka attacker to NOT fire, hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Zhang He, Wei General
// ---------------------------------------------------------------------------

func TestPerCardBatchO_ZhangHe_AttackPumpsOtherCreatures(t *testing.T) {
	gs := newGame(t, 2)
	zhangHe := addPerm(gs, 0, "Zhang He, Wei General", "creature", "legendary")
	zhangHe.Card.BasePower = 3
	zhangHe.Card.BaseToughness = 4
	soldier := addPerm(gs, 0, "Wei Soldier A", "creature")
	soldier.Card.BasePower = 2
	soldier.Card.BaseToughness = 2
	soldier2 := addPerm(gs, 0, "Wei Soldier B", "creature")
	soldier2.Card.BasePower = 2
	soldier2.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": zhangHe,
		"defender_seat": 1,
	})

	if soldier.Flags["plus_power_until_eot"] != 1 {
		t.Errorf("expected soldier +1 power EOT, got %v", soldier.Flags)
	}
	if soldier2.Flags["plus_power_until_eot"] != 1 {
		t.Errorf("expected soldier2 +1 power EOT, got %v", soldier2.Flags)
	}
	// Zhang He herself NOT bumped (printed says "each other creature").
	if zhangHe.Flags["plus_power_until_eot"] != 0 {
		t.Errorf("expected Zhang He NOT pumped, got %v", zhangHe.Flags)
	}

	// Non-Zhang-He attacker — no anthem.
	rando := addPerm(gs, 0, "Random", "creature")
	rando.Card.BasePower = 1
	rando.Card.BaseToughness = 1
	preSoldier := soldier.Flags["plus_power_until_eot"]
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": rando,
		"defender_seat": 1,
	})
	if soldier.Flags["plus_power_until_eot"] != preSoldier {
		t.Errorf("expected non-Zhang-He attack to NOT pump, soldier flag %d → %d",
			preSoldier, soldier.Flags["plus_power_until_eot"])
	}
}

// ---------------------------------------------------------------------------
// Loran of the Third Path
// ---------------------------------------------------------------------------

func TestPerCardBatchO_Loran_ETBDestroysOppEnchantmentPreferred(t *testing.T) {
	gs := newGame(t, 2)
	loran := addPerm(gs, 0, "Loran of the Third Path", "creature", "legendary")
	loran.Card.BasePower = 1
	loran.Card.BaseToughness = 3
	// Opp has both — enchantment should be destroyed first.
	oppArt := addPerm(gs, 1, "Opp Artifact", "artifact")
	oppEnch := addPerm(gs, 1, "Opp Enchantment", "enchantment")

	gameengine.InvokeETBHook(gs, loran)

	// Enchantment should be gone, artifact still standing.
	if findOnBF(gs, 1, "Opp Enchantment") != nil {
		t.Errorf("expected enchantment destroyed first, still on bf")
	}
	if findOnBF(gs, 1, "Opp Artifact") == nil {
		t.Errorf("expected artifact untouched, gone from bf")
	}
	_ = oppArt
	_ = oppEnch

	// Fresh game: opp has only an artifact — Loran takes it instead.
	gs2 := newGame(t, 2)
	loran2 := addPerm(gs2, 0, "Loran of the Third Path", "creature", "legendary")
	loran2.Card.BasePower = 1
	loran2.Card.BaseToughness = 3
	addPerm(gs2, 1, "Solo Artifact", "artifact")
	gameengine.InvokeETBHook(gs2, loran2)
	if findOnBF(gs2, 1, "Solo Artifact") != nil {
		t.Errorf("expected fallback artifact destroyed")
	}

	// Opp has NEITHER — handler no-ops (no panic).
	gs3 := newGame(t, 2)
	loran3 := addPerm(gs3, 0, "Loran of the Third Path", "creature", "legendary")
	loran3.Card.BasePower = 1
	loran3.Card.BaseToughness = 3
	addPerm(gs3, 1, "Plain Creature", "creature")
	gameengine.InvokeETBHook(gs3, loran3)
	if findOnBF(gs3, 1, "Plain Creature") == nil {
		t.Errorf("expected non-artifact/non-enchantment creature untouched")
	}
}

func findOnBF(gs *gameengine.GameState, seat int, name string) *gameengine.Permanent {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			return p
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Captain America, Team Leader
// ---------------------------------------------------------------------------

func TestPerCardBatchO_CaptainAmerica_AnotherHeroETBGrantsAndCounters(t *testing.T) {
	gs := newGame(t, 2)
	cap := addPerm(gs, 0, "Captain America, Team Leader", "creature", "legendary", "hero")
	cap.Card.BasePower = 3
	cap.Card.BaseToughness = 4

	// Friendly Hero ETBs — vigilance + haste + +1/+1 on it, +1/+1 on Cap.
	hero := addPerm(gs, 0, "Wasp", "creature", "hero")
	hero.Card.BasePower = 2
	hero.Card.BaseToughness = 2
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            hero,
		"controller_seat": 0,
	})
	if hero.Flags["kw:vigilance"] != 1 || hero.Flags["kw:haste"] != 1 {
		t.Errorf("expected hero to gain vigilance + haste, got %v", hero.Flags)
	}
	if hero.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 on hero, got %d", hero.Counters["+1/+1"])
	}
	if cap.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 on Captain America, got %d", cap.Counters["+1/+1"])
	}

	// Non-Hero creature ETB — no fire.
	bear := addPerm(gs, 0, "Bear", "creature", "bear")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            bear,
		"controller_seat": 0,
	})
	if bear.Flags["kw:vigilance"] != 0 {
		t.Errorf("expected non-Hero to NOT get vigilance, got %v", bear.Flags)
	}
	if cap.Counters["+1/+1"] != 1 {
		t.Errorf("expected Cap's counter NOT to grow on non-Hero ETB, got %d", cap.Counters["+1/+1"])
	}

	// Captain America's OWN ETB (self) — should NOT self-fire (entering == perm).
	preSelf := cap.Counters["+1/+1"]
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            cap,
		"controller_seat": 0,
	})
	if cap.Counters["+1/+1"] != preSelf {
		t.Errorf("expected self-ETB to NOT self-fire, counter %d → %d", preSelf, cap.Counters["+1/+1"])
	}

	// Opp's Hero ETBs — no fire (controller mismatch).
	oppHero := addPerm(gs, 1, "Opp Hero", "creature", "hero")
	oppHero.Card.BasePower = 2
	oppHero.Card.BaseToughness = 2
	preCap := cap.Counters["+1/+1"]
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            oppHero,
		"controller_seat": 1,
	})
	if cap.Counters["+1/+1"] != preCap {
		t.Errorf("expected opp Hero ETB to NOT fire, Cap counter %d → %d",
			preCap, cap.Counters["+1/+1"])
	}
	if oppHero.Flags["kw:vigilance"] != 0 {
		t.Errorf("expected opp Hero to NOT get our grant, got %v", oppHero.Flags)
	}
}
