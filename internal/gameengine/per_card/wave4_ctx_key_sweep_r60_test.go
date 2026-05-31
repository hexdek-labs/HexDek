package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Wave 4 ctx-key audit (R60) — broader event family sweep
//
// Continues the audit that closed PR #815 (permanent_etb), PR #824
// (combat_damage_player), and PR #841 (creature_attacks). This wave
// covers card_exiled / permanent_phased_out / additional attacks
// handlers missed in wave 3.
//
// Confirmed bugs fixed in this PR:
//
//   - Carmen, Cruel Skymarcher (attacks fn): silently inert — reads
//     ctx["seat"] (engine writes "attacker_seat" for creature_attacks).
//   - Syr Vondam, Sunstar Exemplar (card_exiled fn): silently inert —
//     reads ctx["controller_seat"] (engine writes "seat") AND ctx["card"]
//     as *Card (engine writes string). Helper was shared with
//     creature_dies which has different ctx shape.
//   - The War Doctor (card_exiled + permanent_phased_out self-skip):
//     partial bug — reads ctx["card"]/ctx["perm"] as wrong type; gate
//     fails so War Doctor wrongly gets a time counter on her own
//     exile/phase-out.
//
// Wave 4 audited but found CLEAN: counter_placed handlers
// (Mikey & Leo, Lord Jyscal, Lasting Tarfire, Hapatra, Shalai &
// Hallar — all read source_seat/target_perm/counter_kind which engine
// writes correctly); sacrifice handlers (Korvold, Mazirek, Carmen
// sac-trigger, Savra, It That Betrays, Elanor, Hearthhull, Szarel,
// Crime Novelist, Nuka-Cola — all read controller_seat/card/perm
// correctly); spell_cast handlers (Vega, Tuvasa, Sai, Door of
// Destinies, Sauron, Yidris combat fn, Storm Force of Nature combat fn
// — all use caster_seat or fallbacks); card_drawn handlers (Toothy,
// Alandra, Trouble in Pairs — read seat/drawer_seat/nth_this_turn
// correctly); lore_counter_added handlers (Sheoldred, Terra, Joshua,
// Urza's Saga, the sagas_phase7 dispatcher — all read chapter
// correctly).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Carmen, Cruel Skymarcher — attack-trigger graveyard reanimate
// ---------------------------------------------------------------------------

func TestCarmen_ReanimatesPermanentWithinPowerFromGraveyardOnCanonicalCtx(t *testing.T) {
	gs := newGame(t, 2)
	carmen := addPerm(gs, 0, "Carmen, Cruel Skymarcher", "creature", "legendary", "human", "rogue")
	carmen.Card.BasePower = 3
	carmen.Card.BaseToughness = 3
	gameengine.SetAttackerDefender(carmen, 1)

	// Two candidates in graveyard: one cheap (CMC 2), one too big (CMC 5).
	cheap := addCardToGraveyard(gs, 0, "Llanowar Elves", "creature", "elf", "druid", "cost:2")
	cheap.BasePower = 1
	cheap.BaseToughness = 1
	tooExpensive := addCardToGraveyard(gs, 0, "Akroma, Angel of Wrath", "creature", "angel", "cost:5")
	tooExpensive.BasePower = 6
	tooExpensive.BaseToughness = 6

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": carmen,
		"attacker_seat": carmen.Controller,
		"attacker_card": carmen.Card,
	})

	foundCheap := false
	foundTooExpensive := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == cheap {
			foundCheap = true
		}
		if p.Card == tooExpensive {
			foundTooExpensive = true
		}
	}
	if !foundCheap {
		t.Errorf("expected Llanowar Elves (CMC 2 ≤ power 3) reanimated by Carmen")
	}
	if foundTooExpensive {
		t.Errorf("Akroma (CMC 5 > power 3) should not be reanimable")
	}
}

func TestCarmen_DoesNotReanimateWhenOtherCreatureAttacks(t *testing.T) {
	gs := newGame(t, 2)
	carmen := addPerm(gs, 0, "Carmen, Cruel Skymarcher", "creature", "legendary", "human", "rogue")
	carmen.Card.BasePower = 3
	carmen.Card.BaseToughness = 3

	other := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	other.Card.BasePower = 2
	other.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(other, 1)

	body := addCardToGraveyard(gs, 0, "Llanowar Elves", "creature", "elf", "cost:1")
	body.BasePower = 1
	body.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": other, // not Carmen
		"attacker_seat": other.Controller,
		"attacker_card": other.Card,
	})

	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == body {
			t.Errorf("Carmen should only trigger on her own attack, not on %s", other.Card.DisplayName())
		}
	}
}

// ---------------------------------------------------------------------------
// Syr Vondam, Sunstar Exemplar — card_exiled "other creature" trigger
// ---------------------------------------------------------------------------

func TestSyrVondam_CountsAndGainsLifeOnOtherCreatureExileCanonicalCtx(t *testing.T) {
	gs := newGame(t, 2)
	sv := addPerm(gs, 0, "Syr Vondam, Sunstar Exemplar", "creature", "legendary", "human", "knight")
	sv.Card.BasePower = 2
	sv.Card.BaseToughness = 2

	// Put a creature card in seat 0's exile, then fire card_exiled.
	other := &gameengine.Card{
		Name:  "Llanowar Elves",
		Owner: 0,
		Types: []string{"creature", "elf", "druid"},
	}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, other)

	startLife := gs.Seats[0].Life
	gameengine.FireCardTrigger(gs, "card_exiled", map[string]interface{}{
		"seat":      0,
		"card":      "Llanowar Elves",
		"from_zone": "battlefield",
		"source":    "exile_effect",
	})

	if sv.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 counter on Syr Vondam after other-creature exile, got %v", sv.Counters)
	}
	if gs.Seats[0].Life != startLife+1 {
		t.Errorf("expected +1 life on Syr Vondam's controller, %d → %d", startLife, gs.Seats[0].Life)
	}
}

func TestSyrVondam_DoesNotTriggerOnSelfExile(t *testing.T) {
	gs := newGame(t, 2)
	sv := addPerm(gs, 0, "Syr Vondam, Sunstar Exemplar", "creature", "legendary", "human", "knight")
	sv.Card.BasePower = 2
	sv.Card.BaseToughness = 2

	gs.Seats[0].Exile = append(gs.Seats[0].Exile, sv.Card)

	startLife := gs.Seats[0].Life
	gameengine.FireCardTrigger(gs, "card_exiled", map[string]interface{}{
		"seat":      0,
		"card":      sv.Card.DisplayName(),
		"from_zone": "battlefield",
		"source":    "exile_effect",
	})

	if sv.Counters["+1/+1"] != 0 {
		t.Errorf("Syr Vondam should NOT trigger on her own exile, got %v", sv.Counters)
	}
	if gs.Seats[0].Life != startLife {
		t.Errorf("Syr Vondam should NOT gain life on her own exile, %d → %d", startLife, gs.Seats[0].Life)
	}
}

func TestSyrVondam_DoesNotTriggerOnOpponentExile(t *testing.T) {
	gs := newGame(t, 2)
	sv := addPerm(gs, 0, "Syr Vondam, Sunstar Exemplar", "creature", "legendary", "human", "knight")
	sv.Card.BasePower = 2
	sv.Card.BaseToughness = 2

	// Opponent's creature gets exiled — Syr Vondam's "creature you control"
	// gate must block (engine writes seat = the OWNER of the exited card).
	other := &gameengine.Card{Name: "Goblin Guide", Owner: 1, Types: []string{"creature", "goblin"}}
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, other)

	gameengine.FireCardTrigger(gs, "card_exiled", map[string]interface{}{
		"seat":      1,
		"card":      "Goblin Guide",
		"from_zone": "battlefield",
		"source":    "exile_effect",
	})

	if sv.Counters["+1/+1"] != 0 {
		t.Errorf("Syr Vondam should NOT trigger on opp creature exile, got %v", sv.Counters)
	}
}

// ---------------------------------------------------------------------------
// The War Doctor — exile / phase-out self-skip gates
// ---------------------------------------------------------------------------

func TestTheWarDoctor_GainsTimeCounterOnOtherExileCanonicalCtx(t *testing.T) {
	gs := newGame(t, 2)
	wd := addPerm(gs, 0, "The War Doctor", "creature", "legendary", "time lord", "doctor")
	wd.Card.BasePower = 4
	wd.Card.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "card_exiled", map[string]interface{}{
		"seat":      0,
		"card":      "Grizzly Bears",
		"from_zone": "battlefield",
		"source":    "exile_effect",
	})

	if wd.Counters["time"] != 1 {
		t.Errorf("expected time counter +1 on War Doctor after other-card exile, got %v", wd.Counters)
	}
}

func TestTheWarDoctor_DoesNotGainTimeCounterOnSelfExile(t *testing.T) {
	gs := newGame(t, 2)
	wd := addPerm(gs, 0, "The War Doctor", "creature", "legendary", "time lord", "doctor")
	wd.Card.BasePower = 4
	wd.Card.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "card_exiled", map[string]interface{}{
		"seat":      0,
		"card":      wd.Card.DisplayName(),
		"from_zone": "battlefield",
		"source":    "exile_effect",
	})

	if wd.Counters["time"] != 0 {
		t.Errorf("War Doctor should NOT count her own exile, got %v", wd.Counters)
	}
}

func TestTheWarDoctor_DoesNotGainTimeCounterOnSelfPhaseOut(t *testing.T) {
	gs := newGame(t, 2)
	wd := addPerm(gs, 0, "The War Doctor", "creature", "legendary", "time lord", "doctor")
	wd.Card.BasePower = 4
	wd.Card.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "permanent_phased_out", map[string]interface{}{
		"seat": 0,
		"card": wd.Card.DisplayName(),
	})

	if wd.Counters["time"] != 0 {
		t.Errorf("War Doctor should NOT count her own phase-out, got %v", wd.Counters)
	}
}

func TestTheWarDoctor_GainsTimeCounterOnOtherPhaseOut(t *testing.T) {
	gs := newGame(t, 2)
	wd := addPerm(gs, 0, "The War Doctor", "creature", "legendary", "time lord", "doctor")
	wd.Card.BasePower = 4
	wd.Card.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "permanent_phased_out", map[string]interface{}{
		"seat": 0,
		"card": "Teferi, Mage of Zhalfir",
	})

	if wd.Counters["time"] != 1 {
		t.Errorf("expected time counter +1 on War Doctor after other-perm phase-out, got %v", wd.Counters)
	}
}
