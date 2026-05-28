package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch W (R60) — tests for 5 new draw-engine handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Phyrexian Arena
// -----------------------------------------------------------------------------

func TestPhyrexianArena_UpkeepDrawAndLoseOne(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Phyrexian Arena", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")
	pre := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected Arena to draw 1, hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != pre-1 {
		t.Errorf("expected -1 life from Arena, life=%d (was %d)", gs.Seats[0].Life, pre)
	}
}

func TestPhyrexianArena_DoesNotFireOnOpponentsUpkeep(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Phyrexian Arena", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")
	pre := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 1,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Arena must only fire on its controller's upkeep, hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != pre {
		t.Errorf("Arena must not cost life on opp's upkeep, life=%d", gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Toski, Bearer of Secrets
// -----------------------------------------------------------------------------

func TestToski_DrawsOnOwnCreatureHittingOpponent(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Toski, Bearer of Secrets", "creature", "legendary")
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"defender_seat": 1,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Toski should draw on own->opp combat damage, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestToski_IgnoresOpponentCombatDamage(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Toski, Bearer of Secrets", "creature", "legendary")
	addLibrary(gs, 0, "A", "B", "C")

	// Opponent's creature hits a different opponent — Toski should not fire.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   1,
		"defender_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Toski must not fire when opponent's creature deals damage, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

func TestToski_IgnoresSelfDamage(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Toski, Bearer of Secrets", "creature", "legendary")
	addLibrary(gs, 0, "A", "B")

	// Phantom case: source == defender == controller (shouldn't happen in
	// normal play, but guards a logic regression).
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"defender_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Toski must require defender != controller, hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Beast Whisperer
// -----------------------------------------------------------------------------

func TestBeastWhisperer_DrawsOnOwnCreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Beast Whisperer", "creature")
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.FireCardTrigger(gs, "creature_spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Llanowar Elves",
		"is_creature": true,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Beast Whisperer should draw on own creature cast, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestBeastWhisperer_NoDrawOnOpponentCreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Beast Whisperer", "creature")
	addLibrary(gs, 0, "A", "B", "C")

	gameengine.FireCardTrigger(gs, "creature_spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Grizzly Bears",
		"is_creature": true,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Beast Whisperer must not fire on opp creature cast, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Guardian Project
// -----------------------------------------------------------------------------

func TestGuardianProject_DrawsOnUniqueCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Guardian Project", "enchantment")
	addLibrary(gs, 0, "A", "B", "C")

	entering := addPerm(gs, 0, "Llanowar Elves", "creature")
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Guardian Project should cantrip on unique creature ETB, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

func TestGuardianProject_NoDrawOnDuplicateName(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Guardian Project", "enchantment")
	addLibrary(gs, 0, "A", "B")

	// Existing creature with same name as the entering one.
	addPerm(gs, 0, "Llanowar Elves", "creature")
	entering := addPerm(gs, 0, "Llanowar Elves", "creature")

	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Guardian Project must not cantrip on duplicate-named creature, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

func TestGuardianProject_NoDrawOnGraveyardCollision(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Guardian Project", "enchantment")
	addLibrary(gs, 0, "A", "B")

	// Put a same-named creature card in the graveyard.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &gameengine.Card{
		Name:  "Llanowar Elves",
		Owner: 0,
		Types: []string{"creature"},
	})
	entering := addPerm(gs, 0, "Llanowar Elves", "creature")

	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Guardian Project must not cantrip when graveyard has same-named creature, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

func TestGuardianProject_IgnoresTokens(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Guardian Project", "enchantment")
	addLibrary(gs, 0, "A", "B")

	// "nontoken creature" requirement: a token ETB must not trigger.
	entering := addPerm(gs, 0, "Squirrel Token", "creature", "token")
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Guardian Project must skip token creature ETB, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

func TestGuardianProject_IgnoresNoncreatures(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Guardian Project", "enchantment")
	addLibrary(gs, 0, "A", "B")

	// A non-creature artifact ETB must not trigger.
	entering := addPerm(gs, 0, "Sol Ring", "artifact")
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Guardian Project must skip noncreature ETB, hand=%d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// The Great Henge
// -----------------------------------------------------------------------------

func TestTheGreatHenge_ETBGainsLifeEqualToPower(t *testing.T) {
	gs := newGame(t, 2)
	henge := addPerm(gs, 0, "The Great Henge", "artifact", "legendary")
	henge.Card.BasePower = 5
	henge.Card.BaseToughness = 5
	pre := gs.Seats[0].Life

	gameengine.InvokeETBHook(gs, henge)

	if gs.Seats[0].Life != pre+5 {
		t.Errorf("expected +5 life on Henge ETB (power=5), life=%d (was %d)",
			gs.Seats[0].Life, pre)
	}
}

func TestTheGreatHenge_CreatureETBAddsCounterAndDraws(t *testing.T) {
	gs := newGame(t, 2)
	henge := addPerm(gs, 0, "The Great Henge", "artifact", "legendary")
	addLibrary(gs, 0, "A", "B", "C")

	entering := addPerm(gs, 0, "Llanowar Elves", "creature")
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})

	if entering.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 counter on entering creature, got %d",
			entering.Counters["+1/+1"])
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("Henge should cantrip on creature ETB, hand=%d", len(gs.Seats[0].Hand))
	}
	_ = henge
}

func TestTheGreatHenge_DoesNotTriggerOnSelfETB(t *testing.T) {
	gs := newGame(t, 2)
	henge := addPerm(gs, 0, "The Great Henge", "artifact", "legendary")
	addLibrary(gs, 0, "A", "B", "C")

	// Fire the creature-ETB trigger with Henge as the entering perm —
	// the "ANOTHER nontoken creature" guard must skip self-reference
	// (also Henge isn't a creature, but the perm==self check should fire first).
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            henge,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Henge must not trigger on itself entering, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestTheGreatHenge_IgnoresOpponentCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "The Great Henge", "artifact", "legendary")
	addLibrary(gs, 0, "A", "B")

	// Opponent's creature ETB shouldn't pump or cantrip.
	entering := addPerm(gs, 1, "Grizzly Bears", "creature")
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            entering,
		"controller_seat": 1,
	})

	if entering.Counters["+1/+1"] != 0 {
		t.Errorf("Henge must not pump opponent's creature, counters=%d",
			entering.Counters["+1/+1"])
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Henge must not cantrip on opp creature ETB, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Registry smoke test — confirm all 5 handlers are wired
// -----------------------------------------------------------------------------

func TestBatchWR60_AllRegistered(t *testing.T) {
	// ETB handlers
	if !HasETB("The Great Henge") {
		t.Errorf("expected ETB handler for The Great Henge")
	}
	// Trigger-only cards — exercise via FireCardTrigger and check event log
	// movement (mirrors batch_u_r60_test.go convention).
	cases := []struct {
		card  string
		event string
		ctx   map[string]interface{}
		setup func(*testing.T, *gameengine.GameState)
	}{
		{
			card:  "Phyrexian Arena",
			event: "upkeep_controller",
			ctx:   map[string]interface{}{"active_seat": 0},
			setup: func(_ *testing.T, gs *gameengine.GameState) { addLibrary(gs, 0, "X") },
		},
		{
			card:  "Toski, Bearer of Secrets",
			event: "combat_damage_player",
			ctx:   map[string]interface{}{"source_seat": 0, "defender_seat": 1},
			setup: func(_ *testing.T, gs *gameengine.GameState) { addLibrary(gs, 0, "X") },
		},
		{
			card:  "Beast Whisperer",
			event: "creature_spell_cast",
			ctx:   map[string]interface{}{"caster_seat": 0},
			setup: func(_ *testing.T, gs *gameengine.GameState) { addLibrary(gs, 0, "X") },
		},
	}
	for _, c := range cases {
		gs := newGame(t, 2)
		addPerm(gs, 0, c.card, "creature", "enchantment")
		c.setup(t, gs)
		before := len(gs.EventLog)
		gameengine.FireCardTrigger(gs, c.event, c.ctx)
		if len(gs.EventLog) <= before {
			t.Errorf("%s: no events fired on %q (before=%d after=%d)",
				c.card, c.event, before, len(gs.EventLog))
		}
	}
	// Guardian Project needs a perm argument — separate fixture.
	gs := newGame(t, 2)
	addPerm(gs, 0, "Guardian Project", "enchantment")
	addLibrary(gs, 0, "X")
	entering := addPerm(gs, 0, "Llanowar Elves", "creature")
	before := len(gs.EventLog)
	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})
	if len(gs.EventLog) <= before {
		t.Errorf("Guardian Project: no events fired (before=%d after=%d)",
			before, len(gs.EventLog))
	}
}
