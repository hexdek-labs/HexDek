package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// addTributePerm is addPerm + a sane BasePower/BaseToughness so SBA
// 704.5f doesn't sweep the bearer between the trigger resolution and
// the test's post-condition check (the addPerm helper leaves P/T at 0,
// which makes the perm illegal the moment SBAs run after a trigger).
//
// `tributeN` > 0 also stamps an AST Keyword{Name:"tribute", Args:[N]}
// so engine paths that gate on TributeAmount (ApplyTribute, etc.) see
// the right N — handlers driven via direct FireCardTrigger only need
// the P/T fix, but the engine-integration tests need both.
func addTributePerm(gs *gameengine.GameState, seat int, name string, tributeN int) *gameengine.Permanent {
	p := addPerm(gs, seat, name, "creature")
	p.Card.BasePower = 4
	p.Card.BaseToughness = 4
	if tributeN > 0 {
		p.Card.AST = &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "tribute", Args: []interface{}{tributeN}},
			},
		}
	}
	return p
}

// -----------------------------------------------------------------------------
// tribute_resolved_consumers_r60 — 6 handlers wired in
// tribute_resolved_consumers_r60.go covering the BNG Tribute mechanic
// (CR §702.121).
//
// Each card has BOTH a Tribute-paid branch (engine added counters; the
// per_card refused-payoff handler must abstain) AND a Tribute-refused
// branch (handler fires the printed punishment effect).
// -----------------------------------------------------------------------------

// tributeCtx builds the ctx FireCardTrigger("tribute_resolved", ...)
// receives from ApplyTribute (keywords_tribute.go:286-293).
func tributeCtx(src *gameengine.Permanent, opponentSeat, n int, accepted bool) map[string]interface{} {
	name := ""
	if src != nil && src.Card != nil {
		name = src.Card.DisplayName()
	}
	return map[string]interface{}{
		"perm":            src,
		"card_name":       name,
		"controller_seat": src.Controller,
		"opponent_seat":   opponentSeat,
		"tribute_n":       n,
		"accepted":        accepted,
	}
}

// -----------------------------------------------------------------------------
// Pharagax Giant — refused → 5 damage to each opponent
// -----------------------------------------------------------------------------

func TestPharagaxGiant_RefusedBurnsEachOpponent(t *testing.T) {
	gs := newGame(t, 4)
	giant := addTributePerm(gs, 0, "Pharagax Giant", 2)

	startLives := []int{gs.Seats[1].Life, gs.Seats[2].Life, gs.Seats[3].Life}
	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(giant, 1, 2, false))

	for i, opp := range []int{1, 2, 3} {
		if gs.Seats[opp].Life != startLives[i]-5 {
			t.Errorf("seat %d: expected life %d, got %d", opp, startLives[i]-5, gs.Seats[opp].Life)
		}
	}
	if gs.Seats[0].Life != 20 {
		t.Errorf("controller untouched expected; got %d", gs.Seats[0].Life)
	}
}

func TestPharagaxGiant_AcceptedDoesNotBurn(t *testing.T) {
	gs := newGame(t, 4)
	giant := addTributePerm(gs, 0, "Pharagax Giant", 2)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(giant, 1, 2, true))

	for _, opp := range []int{1, 2, 3} {
		if gs.Seats[opp].Life != 20 {
			t.Errorf("seat %d should not take damage on Tribute-paid; got %d", opp, gs.Seats[opp].Life)
		}
	}
}

func TestPharagaxGiant_SiblingCopyDoesNotFire(t *testing.T) {
	gs := newGame(t, 2)
	g1 := addTributePerm(gs, 0, "Pharagax Giant", 2)
	g2 := addTributePerm(gs, 0, "Pharagax Giant", 2)
	_ = g2

	// Only g1 is the source perm; sibling g2's handler must not also
	// burn opponents.
	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(g1, 1, 2, false))

	// One fan-out should mean exactly 5 damage to seat 1, not 10.
	if gs.Seats[1].Life != 15 {
		t.Errorf("expected seat 1 life 15 (one burn), got %d — sibling g2 likely double-fired", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Snake of the Golden Grove — refused → controller gains 4 life
// -----------------------------------------------------------------------------

func TestSnakeGoldenGrove_RefusedGainsLife(t *testing.T) {
	gs := newGame(t, 2)
	snake := addTributePerm(gs, 0, "Snake of the Golden Grove", 3)
	startLife := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(snake, 1, 3, false))

	if gs.Seats[0].Life != startLife+4 {
		t.Errorf("expected life %d, got %d", startLife+4, gs.Seats[0].Life)
	}
}

func TestSnakeGoldenGrove_AcceptedNoLifeGain(t *testing.T) {
	gs := newGame(t, 2)
	snake := addTributePerm(gs, 0, "Snake of the Golden Grove", 3)
	startLife := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(snake, 1, 3, true))

	if gs.Seats[0].Life != startLife {
		t.Errorf("Tribute-paid: should not gain life; got %d (started %d)", gs.Seats[0].Life, startLife)
	}
}

// -----------------------------------------------------------------------------
// Ornitharch — refused → create two 1/1 white Bird tokens with flying
// -----------------------------------------------------------------------------

func TestOrnitharch_RefusedCreatesTwoBirds(t *testing.T) {
	gs := newGame(t, 2)
	archon := addTributePerm(gs, 0, "Ornitharch", 2)
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(archon, 1, 2, false))

	if len(gs.Seats[0].Battlefield) != pre+2 {
		t.Errorf("expected 2 new perms (Bird tokens), got delta %d",
			len(gs.Seats[0].Battlefield)-pre)
	}
	birdsFound := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Bird Token" {
			birdsFound++
			if !cardHasType(p.Card, "kw:flying") {
				t.Errorf("Bird token missing kw:flying type tag")
			}
		}
	}
	if birdsFound != 2 {
		t.Errorf("expected 2 Bird tokens on battlefield, found %d", birdsFound)
	}
}

func TestOrnitharch_AcceptedNoTokens(t *testing.T) {
	gs := newGame(t, 2)
	archon := addTributePerm(gs, 0, "Ornitharch", 2)
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(archon, 1, 2, true))

	if len(gs.Seats[0].Battlefield) != pre {
		t.Errorf("Tribute-paid: should not create tokens; delta=%d",
			len(gs.Seats[0].Battlefield)-pre)
	}
}

// -----------------------------------------------------------------------------
// Nessian Demolok — refused → destroy target noncreature permanent
// -----------------------------------------------------------------------------

func TestNessianDemolok_RefusedDestroysBiggestNoncreature(t *testing.T) {
	gs := newGame(t, 2)
	weak := addPerm(gs, 1, "Sol Ring", "artifact")
	weak.Card.Types = append(weak.Card.Types, "cmc:1")
	big := addPerm(gs, 1, "Mind's Eye", "artifact")
	big.Card.Types = append(big.Card.Types, "cmc:5")
	demolok := addTributePerm(gs, 0, "Nessian Demolok", 3)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(demolok, 1, 3, false))

	for _, p := range gs.Seats[1].Battlefield {
		if p == big {
			t.Fatalf("Mind's Eye should have been destroyed (highest CMC)")
		}
	}
	stillWeak := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == weak {
			stillWeak = true
		}
	}
	if !stillWeak {
		t.Errorf("Sol Ring (lower CMC) should remain")
	}
}

func TestNessianDemolok_NeverDestroysCreatures(t *testing.T) {
	gs := newGame(t, 2)
	oppCreature := addPerm(gs, 1, "Tarmogoyf", "creature")
	oppCreature.Card.BasePower = 3
	oppCreature.Card.BaseToughness = 3
	demolok := addTributePerm(gs, 0, "Nessian Demolok", 3)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(demolok, 1, 3, false))

	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppCreature {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("creature must not be destroyed (oracle: noncreature permanent)")
	}
}

func TestNessianDemolok_AcceptedDoesNotDestroy(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Mind's Eye", "artifact")
	demolok := addTributePerm(gs, 0, "Nessian Demolok", 3)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(demolok, 1, 3, true))

	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("Tribute-paid: artifact must not be destroyed")
	}
}

// -----------------------------------------------------------------------------
// Fanatic of Xenagos — refused → +1/+1 and haste UEOT
// -----------------------------------------------------------------------------

func TestFanaticXenagos_RefusedBuffsAndHastes(t *testing.T) {
	gs := newGame(t, 2)
	fanatic := addTributePerm(gs, 0, "Fanatic of Xenagos", 1)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(fanatic, 1, 1, false))

	if len(fanatic.Modifications) != 1 {
		t.Fatalf("expected 1 Modification, got %d", len(fanatic.Modifications))
	}
	m := fanatic.Modifications[0]
	if m.Power != 1 || m.Toughness != 1 {
		t.Errorf("expected +1/+1, got +%d/+%d", m.Power, m.Toughness)
	}
	if fanatic.Flags["kw:haste"] != 1 {
		t.Errorf("expected kw:haste=1, got %d", fanatic.Flags["kw:haste"])
	}
	if len(gs.DelayedTriggers) < 1 {
		t.Errorf("expected next_end_step cleanup for haste flag")
	}
}

func TestFanaticXenagos_AcceptedNoBuff(t *testing.T) {
	gs := newGame(t, 2)
	fanatic := addTributePerm(gs, 0, "Fanatic of Xenagos", 1)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(fanatic, 1, 1, true))

	if len(fanatic.Modifications) != 0 {
		t.Errorf("Tribute-paid: should not add Modification; got %d", len(fanatic.Modifications))
	}
	if fanatic.Flags["kw:haste"] == 1 {
		t.Errorf("Tribute-paid: should not stamp kw:haste")
	}
}

// -----------------------------------------------------------------------------
// Thunder Brute — refused → haste UEOT
// -----------------------------------------------------------------------------

func TestThunderBrute_RefusedGainsHasteUEOT(t *testing.T) {
	gs := newGame(t, 2)
	brute := addTributePerm(gs, 0, "Thunder Brute", 3)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(brute, 1, 3, false))

	if brute.Flags["kw:haste"] != 1 {
		t.Errorf("expected kw:haste=1, got %d", brute.Flags["kw:haste"])
	}
}

func TestThunderBrute_AcceptedNoHaste(t *testing.T) {
	gs := newGame(t, 2)
	brute := addTributePerm(gs, 0, "Thunder Brute", 3)

	gameengine.FireCardTrigger(gs, "tribute_resolved", tributeCtx(brute, 1, 3, true))

	if brute.Flags["kw:haste"] == 1 {
		t.Errorf("Tribute-paid: should not gain haste")
	}
}

// -----------------------------------------------------------------------------
// Engine integration — ApplyTribute drives the trigger end-to-end
// -----------------------------------------------------------------------------

func TestTributeResolved_ApplyTributeRefusedDrivesPayoff(t *testing.T) {
	gs := newGame(t, 2)
	snake := addTributePerm(gs, 0, "Snake of the Golden Grove", 3)
	startLife := gs.Seats[0].Life

	// Opp picks seat 1; opp refuses. Engine should fire the
	// tribute_resolved trigger which our handler should consume → +4 life.
	gameengine.ApplyTribute(gs, snake, 0,
		func() int { return 1 },
		func(opp int) bool { return false },
	)

	if gs.Seats[0].Life != startLife+4 {
		t.Errorf("expected life %d after refused ApplyTribute, got %d", startLife+4, gs.Seats[0].Life)
	}
}

func TestTributeResolved_ApplyTributeAcceptedSkipsPayoff(t *testing.T) {
	gs := newGame(t, 2)
	// Use Pharagax Giant: easier to detect "no payoff" via opp life.
	giant := addTributePerm(gs, 0, "Pharagax Giant", 2)

	gameengine.ApplyTribute(gs, giant, 0,
		func() int { return 1 },
		func(opp int) bool { return true }, // accept
	)

	// Counters added (engine path), but opp must not take 5 damage.
	if giant.Counters["+1/+1"] == 0 {
		t.Errorf("expected counters from accepted Tribute, got 0")
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("Tribute-paid: opp should not take 5 damage; got life %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — every handler reachable post-registerDefaults.
// -----------------------------------------------------------------------------

func TestTributeResolvedConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Pharagax Giant",
		"Snake of the Golden Grove",
		"Ornitharch",
		"Nessian Demolok",
		"Fanatic of Xenagos",
		"Thunder Brute",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler for %s", name)
		}
	}
}
