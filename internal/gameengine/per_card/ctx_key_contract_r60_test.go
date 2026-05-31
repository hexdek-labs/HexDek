package per_card

import (
	"strconv"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Ctx-key contract regression sweep (R60)
//
// PR #815 (permanent_etb) and PR #824 (combat_damage_player) fixed 9
// per_card handlers that were silently inert because they read the wrong
// ctx-map key. Each fix added canonical-key reads with a legacy-key
// fallback. These tests pin BOTH halves of the contract per card:
//
//   1. CanonicalCtxKey  — fire with ONLY the engine-canonical ctx shape;
//      the handler must still observe and fire its side effect.
//   2. LegacyCtxKeyFallback — fire with ONLY the legacy/pre-fix keys; the
//      handler must STILL observe via the fallback branch (no regression
//      for old test/Hat callers).
//
// permanent_etb canonical: ctx["perm"]; legacy: ctx["permanent"].
// combat_damage_player canonical: source_seat/source_card/defender_seat/amount;
// legacy: damager_seat/damager_perm (and target_seat for Angel of Destiny).
// ---------------------------------------------------------------------------

// hasSlug returns the count of per_card events whose Details["slug"]
// matches the given slug. emit() always logs Kind="per_card_handler" and
// puts the discriminator in Details["slug"].
func hasSlug(gs *gameengine.GameState, slug string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Details == nil {
			continue
		}
		if s, _ := ev.Details["slug"].(string); s == slug {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Adrix and Nev, Twincasters — permanent_etb token-doubling
// ---------------------------------------------------------------------------

func TestAdrixAndNev_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 2)
	adrix := addPerm(gs, 0, "Adrix and Nev, Twincasters", "creature", "legendary", "merfolk", "wizard")
	adrix.Card.BasePower = 3
	adrix.Card.BaseToughness = 3

	tok := addPerm(gs, 0, "Faerie Rogue", "token", "creature", "faerie", "rogue")
	tok.Card.BasePower = 1
	tok.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            tok,
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Fatalf("canonical ctx[perm]: expected doubled token (BF %d → %d)", before, len(gs.Seats[0].Battlefield))
	}
}

func TestAdrixAndNev_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 2)
	adrix := addPerm(gs, 0, "Adrix and Nev, Twincasters", "creature", "legendary", "merfolk", "wizard")
	adrix.Card.BasePower = 3
	adrix.Card.BaseToughness = 3

	tok := addPerm(gs, 0, "Faerie Rogue", "token", "creature", "faerie", "rogue")
	tok.Card.BasePower = 1
	tok.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"permanent":       tok, // legacy key
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Fatalf("legacy ctx[permanent] fallback: expected doubled token (BF %d → %d)", before, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Anafenza, Kin-Tree Spirit — permanent_etb bolster 1
// ---------------------------------------------------------------------------

func TestAnafenza_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 2)
	anaf := addPerm(gs, 0, "Anafenza, Kin-Tree Spirit", "creature", "legendary", "spirit", "soldier")
	anaf.Card.BasePower = 2
	anaf.Card.BaseToughness = 2
	runt := addPerm(gs, 0, "Squire", "creature", "human")
	runt.Card.BasePower = 1
	runt.Card.BaseToughness = 1

	// Non-token entering creature avoids the DoesNotTriggerOnToken gate.
	newby := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	newby.Card.BasePower = 2
	newby.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            newby,
	})

	if runt.Counters["+1/+1"] != 1 {
		t.Fatalf("canonical ctx[perm]: expected runt bolstered, counters=%v", runt.Counters)
	}
}

func TestAnafenza_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 2)
	anaf := addPerm(gs, 0, "Anafenza, Kin-Tree Spirit", "creature", "legendary", "spirit", "soldier")
	anaf.Card.BasePower = 2
	anaf.Card.BaseToughness = 2
	runt := addPerm(gs, 0, "Squire", "creature", "human")
	runt.Card.BasePower = 1
	runt.Card.BaseToughness = 1

	newby := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	newby.Card.BasePower = 2
	newby.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"permanent":       newby, // legacy key
	})

	if runt.Counters["+1/+1"] != 1 {
		t.Fatalf("legacy ctx[permanent] fallback: expected runt bolstered, counters=%v", runt.Counters)
	}
}

// ---------------------------------------------------------------------------
// Genesis Chamber — permanent_etb Myr token on nontoken creature
// ---------------------------------------------------------------------------

func TestGenesisChamber_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Genesis Chamber", "artifact")
	newby := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	newby.Card.BasePower = 1
	newby.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            newby,
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Fatalf("canonical ctx[perm]: expected Myr token minted (BF %d → %d)", before, len(gs.Seats[0].Battlefield))
	}
}

func TestGenesisChamber_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Genesis Chamber", "artifact")
	newby := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	newby.Card.BasePower = 1
	newby.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"permanent":       newby, // legacy key
	})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Fatalf("legacy ctx[permanent] fallback: expected Myr token minted (BF %d → %d)", before, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Rivaz of the Claw — permanent_etb dragon exile-on-dies rider
// ---------------------------------------------------------------------------

func TestRivaz_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 2)
	rivaz := addPerm(gs, 0, "Rivaz of the Claw", "creature", "legendary", "lizard", "warlock")
	_ = rivaz
	// Pre-seed pending-rider flag: spell_cast normally stamps this; we
	// short-circuit straight to the ETB rider path.
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["rivaz_pending_rider_t"+strconv.Itoa(gs.Turn)] = 1

	dragon := addPerm(gs, 0, "Bladewing the Risen", "creature", "dragon", "zombie")
	dragon.Card.BasePower = 4
	dragon.Card.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            dragon,
	})

	if hasSlug(gs, "rivaz_dragon_exile_rider") < 1 {
		t.Fatalf("canonical ctx[perm]: expected rivaz_dragon_exile_rider slug, got %d", hasSlug(gs, "rivaz_dragon_exile_rider"))
	}
}

func TestRivaz_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Rivaz of the Claw", "creature", "legendary", "lizard", "warlock")
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["rivaz_pending_rider_t"+strconv.Itoa(gs.Turn)] = 1

	dragon := addPerm(gs, 0, "Bladewing the Risen", "creature", "dragon", "zombie")
	dragon.Card.BasePower = 4
	dragon.Card.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"permanent":       dragon, // legacy key
	})

	if hasSlug(gs, "rivaz_dragon_exile_rider") < 1 {
		t.Fatalf("legacy ctx[permanent] fallback: expected rivaz_dragon_exile_rider slug, got %d", hasSlug(gs, "rivaz_dragon_exile_rider"))
	}
}

// ---------------------------------------------------------------------------
// Zinnia, Valley's Voice — permanent_etb offspring token mint
// ---------------------------------------------------------------------------

func TestZinnia_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 2)
	zinnia := addPerm(gs, 0, "Zinnia, Valley's Voice", "creature", "legendary", "bird", "bard")
	zinnia.Card.BasePower = 1
	zinnia.Card.BaseToughness = 3

	newby := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	newby.Card.BasePower = 1
	newby.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            newby,
	})

	if hasSlug(gs, "zinnia_valleys_voice_offspring_token") < 1 {
		t.Fatalf("canonical ctx[perm]: expected zinnia_valleys_voice_offspring_token slug")
	}
	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Fatalf("canonical ctx[perm]: expected offspring token minted (BF %d → %d)", before, len(gs.Seats[0].Battlefield))
	}
}

func TestZinnia_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 2)
	zinnia := addPerm(gs, 0, "Zinnia, Valley's Voice", "creature", "legendary", "bird", "bard")
	zinnia.Card.BasePower = 1
	zinnia.Card.BaseToughness = 3

	newby := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	newby.Card.BasePower = 1
	newby.Card.BaseToughness = 1
	before := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"permanent":       newby, // legacy key
	})

	if hasSlug(gs, "zinnia_valleys_voice_offspring_token") < 1 {
		t.Fatalf("legacy ctx[permanent] fallback: expected zinnia_valleys_voice_offspring_token slug")
	}
	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Fatalf("legacy ctx[permanent] fallback: expected offspring token minted (BF %d → %d)", before, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Archpriest of Shadows — combat_damage_player reanimate
// ---------------------------------------------------------------------------

func TestArchpriest_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 2)
	ap := addPerm(gs, 0, "Archpriest of Shadows", "creature", "human", "warlock")
	ap.Card.BasePower = 3
	ap.Card.BaseToughness = 4
	high := addCardToGraveyard(gs, 0, "Grave Titan", "creature", "giant", "cost:6")
	high.BasePower = 6
	high.BaseToughness = 5

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Archpriest of Shadows",
		"defender_seat": 1,
		"amount":        3,
	})

	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == high {
			found = true
		}
	}
	if !found {
		t.Fatalf("canonical combat-damage ctx: expected Grave Titan reanimated")
	}
}

func TestArchpriest_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 2)
	ap := addPerm(gs, 0, "Archpriest of Shadows", "creature", "human", "warlock")
	ap.Card.BasePower = 3
	ap.Card.BaseToughness = 4
	high := addCardToGraveyard(gs, 0, "Grave Titan", "creature", "giant", "cost:6")
	high.BasePower = 6
	high.BaseToughness = 5

	// Legacy: damager_perm = Archpriest itself.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"damager_perm": ap,
		"amount":       3,
	})

	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == high {
			found = true
		}
	}
	if !found {
		t.Fatalf("legacy ctx[damager_perm] fallback: expected Grave Titan reanimated")
	}
}

// ---------------------------------------------------------------------------
// April, Reporter of the Weird — combat_damage_player draw N / discard 1
// ---------------------------------------------------------------------------

func TestApril_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 2)
	april := addPerm(gs, 0, "April, Reporter of the Weird", "creature", "legendary", "human", "advisor")
	april.Card.BasePower = 2
	april.Card.BaseToughness = 2
	addLibrary(gs, 0, "A", "B", "C", "D")
	startHand := len(gs.Seats[0].Hand)
	startYard := len(gs.Seats[0].Graveyard)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "April, Reporter of the Weird",
		"defender_seat": 1,
		"amount":        2,
	})

	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Fatalf("canonical combat-damage ctx: expected hand +1 (draw 2 / discard 1), %d → %d", startHand, len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Graveyard) != startYard+1 {
		t.Fatalf("canonical combat-damage ctx: expected graveyard +1 (discard), %d → %d", startYard, len(gs.Seats[0].Graveyard))
	}
}

func TestApril_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 2)
	april := addPerm(gs, 0, "April, Reporter of the Weird", "creature", "legendary", "human", "advisor")
	april.Card.BasePower = 2
	april.Card.BaseToughness = 2
	addLibrary(gs, 0, "A", "B", "C", "D")
	startHand := len(gs.Seats[0].Hand)
	startYard := len(gs.Seats[0].Graveyard)

	// Legacy: damager_perm = April herself.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"damager_perm": april,
		"amount":       2,
	})

	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Fatalf("legacy ctx[damager_perm] fallback: expected hand +1, %d → %d", startHand, len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Graveyard) != startYard+1 {
		t.Fatalf("legacy ctx[damager_perm] fallback: expected graveyard +1, %d → %d", startYard, len(gs.Seats[0].Graveyard))
	}
}

// ---------------------------------------------------------------------------
// Quilled Greatwurm — combat_damage_player +1/+1 counters on attacker
// ---------------------------------------------------------------------------

func TestQuilledGreatwurm_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	qg := addPerm(gs, 0, "Quilled Greatwurm", "creature", "wurm")
	qg.Card.BasePower = 8
	qg.Card.BaseToughness = 8

	atkr := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	atkr.Card.BasePower = 1
	atkr.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Llanowar Elves",
		"defender_seat": 1,
		"amount":        4,
	})

	if atkr.Counters["+1/+1"] != 4 {
		t.Fatalf("canonical combat-damage ctx: expected +1/+1 x4 on Elves, got %v", atkr.Counters)
	}
}

func TestQuilledGreatwurm_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	qg := addPerm(gs, 0, "Quilled Greatwurm", "creature", "wurm")
	qg.Card.BasePower = 8
	qg.Card.BaseToughness = 8

	atkr := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	atkr.Card.BasePower = 1
	atkr.Card.BaseToughness = 2

	// Legacy: damager_seat + damager_perm.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"damager_seat": 0,
		"damager_perm": atkr,
		"amount":       4,
	})

	if atkr.Counters["+1/+1"] != 4 {
		t.Fatalf("legacy ctx[damager_seat/damager_perm] fallback: expected +1/+1 x4, got %v", atkr.Counters)
	}
}

// ---------------------------------------------------------------------------
// Angel of Destiny — combat_damage_player symmetric lifegain (cross-seat case)
// ---------------------------------------------------------------------------

func TestAngelOfDestiny_CanonicalCtxKey(t *testing.T) {
	gs := newGame(t, 3)
	// Put Angel on seat 1 (non-zero) — pre-fix the handler defaulted
	// dmgSeat to 0 from a missing key. Cross-seat case exercises the fix.
	angel := addPerm(gs, 1, "Angel of Destiny", "creature", "angel")
	angel.Card.BasePower = 2
	angel.Card.BaseToughness = 4
	startCtrl := gs.Seats[1].Life
	startTgt := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   1,
		"source_card":   "Angel of Destiny",
		"defender_seat": 0,
		"amount":        2,
	})

	if gs.Seats[1].Life != startCtrl+2 {
		t.Fatalf("canonical combat-damage ctx: Angel controller gains 2 life, %d → %d", startCtrl, gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != startTgt+2 {
		t.Fatalf("canonical combat-damage ctx: defender gains 2 life (symmetric), %d → %d", startTgt, gs.Seats[0].Life)
	}
}

func TestAngelOfDestiny_LegacyCtxKeyFallback(t *testing.T) {
	gs := newGame(t, 3)
	angel := addPerm(gs, 1, "Angel of Destiny", "creature", "angel")
	angel.Card.BasePower = 2
	angel.Card.BaseToughness = 4
	startCtrl := gs.Seats[1].Life
	startTgt := gs.Seats[0].Life

	// Legacy: damager_seat + target_seat.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"damager_seat": 1,
		"target_seat":  0,
		"amount":       2,
	})

	if gs.Seats[1].Life != startCtrl+2 {
		t.Fatalf("legacy ctx[damager_seat/target_seat] fallback: Angel controller gains 2, %d → %d", startCtrl, gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != startTgt+2 {
		t.Fatalf("legacy ctx[damager_seat/target_seat] fallback: defender gains 2, %d → %d", startTgt, gs.Seats[0].Life)
	}
}
