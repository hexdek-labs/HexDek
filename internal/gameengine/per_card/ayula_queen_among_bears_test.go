package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Ayula, Queen Among Bears — fight-mode heuristic.
//
// Pre-R60 the handler always picked counters mode. These tests pin the
// new heuristic: fight when the entering Bear can kill an opposing
// creature without dying; otherwise fall back to counters.
// ---------------------------------------------------------------------------

func TestAyula_DefaultsToCountersWhenNoFavorableTarget(t *testing.T) {
	gs := newGame(t, 2)
	ayula := addPerm(gs, 0, "Ayula, Queen Among Bears", "creature", "legendary", "bear")
	ayula.Card.BasePower = 2
	ayula.Card.BaseToughness = 2

	// Entering Grizzly is a 2/2; opponent has a 3/3 that would kill it
	// in a fight. Strictly-favorable trade does not exist → counters mode.
	grizzly := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	grizzly.Card.BasePower = 2
	grizzly.Card.BaseToughness = 2

	bigCat := addPerm(gs, 1, "Sabertooth", "creature", "cat")
	bigCat.Card.BasePower = 3
	bigCat.Card.BaseToughness = 3

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            grizzly,
	})

	if grizzly.Counters["+1/+1"] != 2 {
		t.Errorf("expected +1/+1 counters x2 on grizzly (counters mode), got %v", grizzly.Counters)
	}
	if bigCat.MarkedDamage != 0 || grizzly.MarkedDamage != 0 {
		t.Errorf("expected no marked damage when defaulting to counters, got bear=%d cat=%d",
			grizzly.MarkedDamage, bigCat.MarkedDamage)
	}
}

func TestAyula_PicksFightWhenStrictlyFavorable(t *testing.T) {
	gs := newGame(t, 2)
	ayula := addPerm(gs, 0, "Ayula, Queen Among Bears", "creature", "legendary", "bear")
	ayula.Card.BasePower = 2
	ayula.Card.BaseToughness = 2

	// Entering 4/4 Bear can kill a 1/2 opp without dying (4>=2 and 4>1).
	bigBear := addPerm(gs, 0, "Werebear", "creature", "bear")
	bigBear.Card.BasePower = 4
	bigBear.Card.BaseToughness = 4

	wimp := addPerm(gs, 1, "Mogg Fanatic", "creature", "goblin")
	wimp.Card.BasePower = 1
	wimp.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            bigBear,
	})

	if bigBear.Counters["+1/+1"] != 0 {
		t.Errorf("expected fight mode (no counters), got counters=%v", bigBear.Counters)
	}
	if wimp.MarkedDamage != 4 {
		t.Errorf("expected victim to take 4 marked damage, got %d", wimp.MarkedDamage)
	}
	if bigBear.MarkedDamage != 1 {
		t.Errorf("expected bear to take 1 marked damage, got %d", bigBear.MarkedDamage)
	}
}

func TestAyula_PrefersHighestPowerVictim(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Ayula, Queen Among Bears", "creature", "legendary", "bear").Card.BasePower = 2

	// Entering 5/5: can favorably kill both opp1 (2/2) and opp2 (4/3),
	// but opp2 is the bigger threat. Heuristic picks opp2.
	hulk := addPerm(gs, 0, "Hulking Bear", "creature", "bear")
	hulk.Card.BasePower = 5
	hulk.Card.BaseToughness = 5

	weakOpp := addPerm(gs, 1, "Bear Cub", "creature", "bear")
	weakOpp.Card.BasePower = 2
	weakOpp.Card.BaseToughness = 2

	bigThreat := addPerm(gs, 1, "Drake Familiar", "creature", "drake")
	bigThreat.Card.BasePower = 4
	bigThreat.Card.BaseToughness = 3

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            hulk,
	})

	if bigThreat.MarkedDamage != 5 {
		t.Errorf("expected big threat to be fought (5 damage), got %d", bigThreat.MarkedDamage)
	}
	if weakOpp.MarkedDamage != 0 {
		t.Errorf("expected weak opp untouched, got %d", weakOpp.MarkedDamage)
	}
	if hulk.MarkedDamage != 4 {
		t.Errorf("expected bear to take 4 marked damage from big threat, got %d", hulk.MarkedDamage)
	}
}

func TestAyula_IgnoresAyulaHerselfAndNonBearETBs(t *testing.T) {
	gs := newGame(t, 2)
	ayula := addPerm(gs, 0, "Ayula, Queen Among Bears", "creature", "legendary", "bear")
	ayula.Card.BasePower = 2
	ayula.Card.BaseToughness = 2

	// Ayula's own ETB shouldn't trigger her on herself.
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            ayula,
	})
	if ayula.Counters["+1/+1"] != 0 {
		t.Errorf("Ayula should not buff herself, got counters=%v", ayula.Counters)
	}

	// A non-Bear ETB shouldn't trigger her.
	wolf := addPerm(gs, 0, "Wolf Token", "creature", "wolf")
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            wolf,
	})
	if wolf.Counters["+1/+1"] != 0 {
		t.Errorf("non-bear should not be buffed by Ayula, got counters=%v", wolf.Counters)
	}
}

func TestAyula_OpponentBearETBDoesNotTrigger(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Ayula, Queen Among Bears", "creature", "legendary", "bear")

	// Opponent Bear enters — Ayula's controller_seat gate must reject.
	oppBear := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")
	oppBear.Card.BasePower = 2
	oppBear.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 1,
		"perm":            oppBear,
	})
	if oppBear.Counters["+1/+1"] != 0 {
		t.Errorf("opponent's bear should not be buffed by Ayula, got %v", oppBear.Counters)
	}
}
