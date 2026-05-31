package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestBloodchiefAscension_EndStepStampsQuestCounter pins the
// canonical end-step quest-counter add when an opponent has lost ≥2
// life this turn. The R60 batch 4 cleanup dropped the "may choice
// auto-accepted" emitPartial but the trigger behavior itself is
// unchanged — this pin guards against accidental dispatch breakage
// during the callout cleanup.
func TestBloodchiefAscension_EndStepStampsQuestCounter(t *testing.T) {
	gs := newGame(t, 2)
	asc := addPerm(gs, 0, "Bloodchief Ascension", "enchantment")
	gs.Seats[1].Turn.LifeLost = 2

	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": 1,
	})

	if got := asc.Counters["quest"]; got != 1 {
		t.Errorf("expected 1 quest counter after opp lost 2+ life; got %d", got)
	}
}

// TestBloodchiefAscension_NoPartialOnDrainBranch pins the cleanup —
// when the drain branch fires it should emit ONLY the per_card_handler
// event with the drain payload, NOT a stale per_card_partial.
func TestBloodchiefAscension_NoPartialOnDrainBranch(t *testing.T) {
	gs := newGame(t, 2)
	asc := addPerm(gs, 0, "Bloodchief Ascension", "enchantment")
	asc.Counters["quest"] = 3
	gs.Seats[1].Life = 40

	// Card lands in opponent's graveyard from somewhere.
	gameengine.FireCardTrigger(gs, "zone_change", map[string]interface{}{
		"to_zone":   "graveyard",
		"from_zone": "library",
		"seat":      1,
		"card":      "Some Card",
	})

	if gs.Seats[1].Life != 38 {
		t.Errorf("expected opp life 38 after 2-drain; got %d", gs.Seats[1].Life)
	}
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == "Bloodchief Ascension" {
			t.Errorf("expected no per_card_partial for Bloodchief after batch 4 cleanup; got %+v", ev)
		}
	}
}

// TestAshlingTheLimitless_NoPartialAfterCleanup pins the R60 batch 4
// stale-callout drop: the grant lifecycle handles the time-bound via
// Duration=until_end_of_turn + ExpireZoneCastGrants. No partial emit.
func TestAshlingTheLimitless_NoPartialAfterCleanup(t *testing.T) {
	gs := newGame(t, 2)
	ashling := addPerm(gs, 0, "Ashling the Limitless", "creature", "elemental")
	exiled1 := addCard(gs, 0, "Top Card 1")
	exiled2 := addCard(gs, 0, "Top Card 2")
	gs.Seats[0].Library = append(gs.Seats[0].Library, exiled1, exiled2)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat": 0,
		"source_card": ashling.Card.DisplayName(),
		"source_perm": ashling,
		"target_seat": 1,
		"amount":      2,
	})

	// Grant registered with Duration=until_end_of_turn.
	if g, ok := gs.ZoneCastGrants[exiled1]; !ok {
		t.Errorf("expected grant for exiled1")
	} else if g.Duration != "until_end_of_turn" {
		t.Errorf("expected Duration=until_end_of_turn, got %q", g.Duration)
	}
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" && ev.Source == "Ashling the Limitless" {
			t.Errorf("expected no per_card_partial after batch 4 cleanup; got %+v", ev)
		}
	}
}

// TestBrudiclad_CreatureTokensGetHasteOnETB pins the anthem: when
// Brudiclad enters, every creature token his controller owns gets
// Flags["kw:haste"].
func TestBrudiclad_CreatureTokensGetHasteOnETB(t *testing.T) {
	gs := newGame(t, 2)
	tok1 := addPerm(gs, 0, "Treasure Token", "token", "artifact", "creature")
	tok2 := addPerm(gs, 0, "Myr Token", "token", "artifact", "creature")
	nonTokenCreature := addPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	nonCreatureToken := addPerm(gs, 0, "Food Token", "token", "artifact")
	bru := addPerm(gs, 0, "Brudiclad, Telchor Engineer", "legendary", "creature", "human", "artificer")

	gameengine.InvokeETBHook(gs, bru)

	if tok1.Flags["kw:haste"] != 1 {
		t.Errorf("token creature 1 should have kw:haste; flags=%+v", tok1.Flags)
	}
	if tok2.Flags["kw:haste"] != 1 {
		t.Errorf("token creature 2 should have kw:haste; flags=%+v", tok2.Flags)
	}
	if nonTokenCreature.Flags["kw:haste"] == 1 {
		t.Errorf("non-token creature must not get haste; flags=%+v", nonTokenCreature.Flags)
	}
	if nonCreatureToken.Flags["kw:haste"] == 1 {
		t.Errorf("non-creature token must not get haste; flags=%+v", nonCreatureToken.Flags)
	}
}

// TestBrudiclad_LTBStripsGrantedHasteOnly pins the per-flag strip
// (native AST haste survives Brudiclad leaving).
func TestBrudiclad_LTBStripsGrantedHasteOnly(t *testing.T) {
	gs := newGame(t, 2)
	grantedTok := addPerm(gs, 0, "Treasure Token", "token", "artifact", "creature")
	nativeHasteTok := addPerm(gs, 0, "Native Haste Token", "token", "creature")
	nativeHasteTok.Flags["kw:haste"] = 1
	bru := addPerm(gs, 0, "Brudiclad, Telchor Engineer", "legendary", "creature", "human", "artificer")

	gameengine.InvokeETBHook(gs, bru)
	if grantedTok.Flags["brudiclad_haste_grant"] != 1 {
		t.Fatalf("setup: granted token should carry marker; flags=%+v", grantedTok.Flags)
	}

	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            bru,
		"card":            bru.Card,
		"to_zone":         "graveyard",
		"controller_seat": 0,
	})

	if grantedTok.Flags["kw:haste"] == 1 {
		t.Errorf("granted token should lose kw:haste on Brudiclad LTB; flags=%+v", grantedTok.Flags)
	}
	if nativeHasteTok.Flags["kw:haste"] != 1 {
		t.Errorf("native-haste token must keep kw:haste; flags=%+v", nativeHasteTok.Flags)
	}
}

// TestAtemsis_LootActivationDrawsTwoDiscardsHighestCMC pins the
// {2}{U}, {T}: draw 2 discard 1 wiring. Pre-batch-4 the activation
// only tapped and emitted partial.
func TestAtemsis_LootActivationDrawsTwoDiscardsHighestCMC(t *testing.T) {
	gs := newGame(t, 2)
	atem := addPerm(gs, 0, "Atemsis, All-Seeing", "creature", "sphinx")
	// Library: a cheap card and an expensive card to draw.
	cheap := addCard(gs, 0, "Bolt", "instant", "cost:1")
	expensive := addCard(gs, 0, "Big Spell", "sorcery", "cost:8")
	gs.Seats[0].Library = append(gs.Seats[0].Library, cheap, expensive)

	gameengine.InvokeActivatedHook(gs, atem, 0, nil)

	if !atem.Tapped {
		t.Errorf("Atemsis should be tapped after activation")
	}
	// We drew 2 then discarded 1 — net hand size = +1.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected hand size 1 (drew 2, discarded 1); got %d", len(gs.Seats[0].Hand))
	}
	// Discarded should be the highest-CMC card.
	if len(gs.Seats[0].Hand) == 1 {
		kept := gs.Seats[0].Hand[0]
		if kept.DisplayName() == expensive.DisplayName() {
			t.Errorf("expected to discard expensive card; kept=%s", kept.DisplayName())
		}
	}
	// And the expensive card should be in the graveyard.
	foundInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c != nil && c.DisplayName() == expensive.DisplayName() {
			foundInGY = true
			break
		}
	}
	if !foundInGY {
		t.Errorf("expensive card should be in graveyard after discard; gy=%+v", gs.Seats[0].Graveyard)
	}
}

// TestAtemsis_LootActivationFailsIfTapped pins the tap-cost gate:
// activating a tapped Atemsis emits a fail event without re-drawing.
func TestAtemsis_LootActivationFailsIfTapped(t *testing.T) {
	gs := newGame(t, 2)
	atem := addPerm(gs, 0, "Atemsis, All-Seeing", "creature", "sphinx")
	atem.Tapped = true

	gameengine.InvokeActivatedHook(gs, atem, 0, nil)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected no draw when activation fails (already tapped); hand=%+v", gs.Seats[0].Hand)
	}
}
