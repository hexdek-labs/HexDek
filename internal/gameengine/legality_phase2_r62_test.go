package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// r62 — legality validator PHASE 2: combat-declaration legality (CR 508.1
// / 509.1) and sorcery-speed activated-ability timing (CR 606.3 /
// 602.5d). All checks ride the existing default-off validator; every
// test drives the REAL engine paths (DeclareAttackers / DeclareBlockers
// / ActivateAbility), never synthetic observations.
// -----------------------------------------------------------------------------

func phase2Game(t *testing.T) (*GameState, *LegalityValidator) {
	t.Helper()
	gs := newFixtureGame(t)
	v := NewLegalityValidator(99)
	gs.Legality = v
	return gs, v
}

func violationCountsByRule(v *LegalityValidator) map[string]int {
	out := map[string]int{}
	for _, viol := range v.Violations {
		out[viol.Rule]++
	}
	return out
}

func phase2Creature(name string, seat, pow, tough int, kws ...string) *Permanent {
	types := []string{"creature"}
	abilities := []gameast.Ability{}
	for _, k := range kws {
		abilities = append(abilities, &gameast.Keyword{Name: k})
	}
	return &Permanent{
		Card: &Card{
			Name:          name,
			Owner:         seat,
			Types:         types,
			BasePower:     pow,
			BaseToughness: tough,
			AST:           &gameast.CardAST{Name: name, Abilities: abilities},
		},
		Controller: seat,
		Owner:      seat,
	}
}

// cheatingAttackHat returns a fixed attacker set, ignoring the legal pool
// — the exact policy-bug shape DeclareAttackers applies unverified.
type cheatingAttackHat struct {
	GreedyHatStub
	force []*Permanent
}

func (h *cheatingAttackHat) ChooseAttackers(gs *GameState, seatIdx int, legal []*Permanent) []*Permanent {
	return h.force
}

func TestLegalityPhase2_IllegalAttackers_Flagged(t *testing.T) {
	gs, v := phase2Game(t)
	gs.Active = 0
	gs.Phase = "combat"

	tapped := phase2Creature("Tapped Bear", 0, 2, 2)
	tapped.Tapped = true
	sick := phase2Creature("Sick Bear", 0, 2, 2)
	sick.SummoningSick = true
	wall := phase2Creature("Wall of Stone", 0, 0, 8, "defender")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tapped, sick, wall)
	gs.Seats[0].Hat = &cheatingAttackHat{force: []*Permanent{tapped, sick, wall}}

	DeclareAttackers(gs, 0)

	got := violationCountsByRule(v)
	if got["508.1c"] != 1 {
		t.Errorf("tapped attacker: want one 508.1c, got %v", v.Violations)
	}
	if got["302.6"] != 1 {
		t.Errorf("summoning-sick attacker: want one 302.6, got %v", v.Violations)
	}
	if got["508.1a"] != 1 {
		t.Errorf("defender attacker: want one 508.1a, got %v", v.Violations)
	}
}

func TestLegalityPhase2_LegalAttacker_Clean(t *testing.T) {
	gs, v := phase2Game(t)
	gs.Active = 0
	gs.Phase = "combat"
	bear := phase2Creature("Honest Bear", 0, 2, 2)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bear)
	gs.Seats[0].Hat = &cheatingAttackHat{force: []*Permanent{bear}}

	DeclareAttackers(gs, 0)

	if len(v.Violations) != 0 {
		t.Fatalf("legal attacker flagged: %v", v.Violations)
	}
	// Sanity: a hasty fresh creature is also clean.
	hasty := phase2Creature("Hasty Bear", 0, 2, 2, "haste")
	hasty.SummoningSick = true
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, hasty)
	gs.Seats[0].Hat = &cheatingAttackHat{force: []*Permanent{hasty}}
	DeclareAttackers(gs, 0)
	if len(v.Violations) != 0 {
		t.Fatalf("haste attacker flagged: %v", v.Violations)
	}
}

// cheatingBlockHat assigns a fixed block map, unvalidated — the engine
// hat path applies it with no checks.
type cheatingBlockHat struct {
	GreedyHatStub
	assign map[*Permanent][]*Permanent
}

func (h *cheatingBlockHat) AssignBlockers(gs *GameState, seatIdx int, attackers []*Permanent) map[*Permanent][]*Permanent {
	return h.assign
}

func TestLegalityPhase2_IllegalBlocks_Flagged(t *testing.T) {
	gs, v := phase2Game(t)
	gs.Active = 0
	gs.Phase = "combat"

	flyer := phase2Creature("Sky Bear", 0, 2, 2, "flying")
	menacer := phase2Creature("Menace Bear", 0, 2, 2, "menace")
	vanilla := phase2Creature("Plain Bear", 0, 2, 2)
	setAttackerDefender(flyer, 1)
	setAttackerDefender(menacer, 1)
	setAttackerDefender(vanilla, 1)

	ground := phase2Creature("Ground Ape", 1, 2, 2)
	tappedB := phase2Creature("Tapped Ape", 1, 2, 2)
	tappedB.Tapped = true
	doubleDuty := phase2Creature("Busy Ape", 1, 2, 2)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, ground, tappedB, doubleDuty)
	gs.Seats[1].Hat = &cheatingBlockHat{assign: map[*Permanent][]*Permanent{
		flyer:   {ground},              // evasion unsatisfied (flying vs ground)
		menacer: {tappedB},             // tapped blocker AND menace single-block
		vanilla: {doubleDuty, doubleDuty}, // same creature committed twice
	}}

	DeclareBlockers(gs, []*Permanent{flyer, menacer, vanilla}, 1)

	got := violationCountsByRule(v)
	if got["509.1b"] < 1 {
		t.Errorf("ground creature blocking flyer: want 509.1b, got %v", v.Violations)
	}
	if got["509.1a"] < 1 {
		t.Errorf("tapped blocker: want 509.1a, got %v", v.Violations)
	}
	if got["702.110b"] < 1 {
		t.Errorf("menace single-block: want 702.110b, got %v", v.Violations)
	}
	if got["509.1"] < 1 {
		t.Errorf("double-committed blocker: want 509.1 multi-block, got %v", v.Violations)
	}
}

func TestLegalityPhase2_LegalBlocks_Clean(t *testing.T) {
	gs, v := phase2Game(t)
	gs.Active = 0
	gs.Phase = "combat"

	flyer := phase2Creature("Sky Bear", 0, 2, 2, "flying")
	menacer := phase2Creature("Menace Bear", 0, 2, 2, "menace")
	setAttackerDefender(flyer, 1)
	setAttackerDefender(menacer, 1)

	reacher := phase2Creature("Reach Ape", 1, 2, 4, "reach")
	g1 := phase2Creature("Ape One", 1, 2, 2)
	g2 := phase2Creature("Ape Two", 1, 2, 2)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, reacher, g1, g2)
	gs.Seats[1].Hat = &cheatingBlockHat{assign: map[*Permanent][]*Permanent{
		flyer:   {reacher}, // reach legally blocks flying
		menacer: {g1, g2},  // menace satisfied with two
	}}

	DeclareBlockers(gs, []*Permanent{flyer, menacer}, 1)

	if len(v.Violations) != 0 {
		t.Fatalf("legal blocks flagged: %v", v.Violations)
	}
}

func phase2Walker(seat int) (*Permanent, *gameast.Activated) {
	plus := &gameast.Activated{
		Cost:   gameast.Cost{Extra: []string{"+1"}},
		Effect: &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}},
	}
	pw := &Permanent{
		Card: &Card{
			Name:  "Test Walker",
			Owner: seat,
			Types: []string{"planeswalker"},
			AST:   &gameast.CardAST{Name: "Test Walker", Abilities: []gameast.Ability{plus}},
		},
		Controller: seat,
		Owner:      seat,
		Counters:   map[string]int{"loyalty": 3},
	}
	return pw, plus
}

func TestLegalityPhase2_LoyaltyAtInstantSpeed_Flagged(t *testing.T) {
	gs, v := phase2Game(t)
	gs.Active = 0
	gs.Phase = "main"

	// Non-active seat 1 activates its walker during seat 0's turn with a
	// spell on the stack — three 606.3 violations (seat, phase ok, stack).
	pw, _ := phase2Walker(1)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, pw)
	gs.Stack = append(gs.Stack, &StackItem{
		Kind: "spell", Controller: 0,
		Card: &Card{Name: "Resident Spell", Types: []string{"instant"}},
	})

	if err := ActivateAbility(gs, 1, pw, 0, nil); err != nil {
		t.Fatalf("engine rejected the activation (no engine gate expected): %v", err)
	}
	got := violationCountsByRule(v)
	if got["606.3"] < 2 {
		t.Fatalf("loyalty at instant speed by non-active seat: want >=2 606.3 violations (non-active + mid-stack), got %v", v.Violations)
	}
}

func TestLegalityPhase2_LoyaltyAtSorcerySpeed_Clean(t *testing.T) {
	gs, v := phase2Game(t)
	gs.Active = 0
	gs.Phase = "main"
	pw, _ := phase2Walker(0)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, pw)

	if err := ActivateAbility(gs, 0, pw, 0, nil); err != nil {
		t.Fatalf("legal loyalty activation rejected: %v", err)
	}
	if len(v.Violations) != 0 {
		t.Fatalf("legal loyalty activation flagged: %v", v.Violations)
	}
}

func TestLegalityPhase2_SorceryRestrictedAbility_Flagged(t *testing.T) {
	gs, v := phase2Game(t)
	gs.Active = 0
	gs.Phase = "combat" // not a main phase

	ab := &gameast.Activated{
		Cost:              gameast.Cost{Tap: true},
		Effect:            &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}},
		TimingRestriction: "sorcery",
	}
	src := &Permanent{
		Card: &Card{
			Name:  "Slow Engine",
			Owner: 0,
			Types: []string{"artifact"},
			AST:   &gameast.CardAST{Name: "Slow Engine", Abilities: []gameast.Ability{ab}},
		},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	if err := ActivateAbility(gs, 0, src, 0, nil); err != nil {
		t.Fatalf("engine rejected the activation: %v", err)
	}
	got := violationCountsByRule(v)
	if got["602.5d"] < 1 {
		t.Fatalf(`"activate only as a sorcery" in combat: want 602.5d, got %v`, v.Violations)
	}
}

// Default-off pin, UPDATED for the r62 §508.1 engine backstop
// (sanitizeDeclaredAttackers in combat.go): with gs.Legality nil the
// phase-2 OBSERVATION hooks remain nil-receiver no-ops (no panic, no
// violation bookkeeping) — but declaration legality is now ENFORCED by
// the engine itself, independent of the validator. The cheating hat's
// tapped attacker is dropped rather than declared; as originally
// written this test pinned the validator-can-see-but-nothing-stops-it
// gap that the backstop exists to close.
func TestLegalityPhase2_DefaultOff_NoOp(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	tapped := phase2Creature("Tapped Bear", 0, 2, 2)
	tapped.Tapped = true
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tapped)
	gs.Seats[0].Hat = &cheatingAttackHat{force: []*Permanent{tapped}}
	declared := DeclareAttackers(gs, 0)
	if len(declared) != 0 {
		t.Fatalf("engine backstop missing: tapped forced attacker was declared (%d)", len(declared))
	}
	if permFlag(tapped, flagAttacking) {
		t.Fatal("tapped forced attacker carries flagAttacking despite the drop")
	}
}
