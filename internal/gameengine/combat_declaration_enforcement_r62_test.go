package gameengine

// Regression tests for the CR §508.1/§509.1 combat-declaration engine
// backstop (r62). The phase-2 legality validator flagged that the engine
// APPLIES hat attacker/blocker maps with no validation — #1028 stopped
// hats from CHOOSING illegal landwalk blocks, but the engine still
// executed whatever map it was handed. These tests pin:
//
//   1. Legal declarations are NEVER dropped (no false positives).
//   2. An off-pool attacker (tapped here; the pool encodes all of
//      canAttack) is dropped while its legal sibling still attacks.
//   3. Illegal blocks — flyer vs groundling (§509.1b), menace blocked by
//      one (§702.110b), islandwalk vs Island controller (§702.14),
//      tapped blocker (§509.1a) — are dropped while a legal block in
//      the SAME combat survives untouched.
//   4. One blocker committed to two attackers keeps only the first
//      assignment (§509.1).
//   5. The ride-along validator still sees the RAW policy output, so a
//      dropped declaration remains visible as a violation.

import (
	"math/rand"
	"testing"
)

// scriptedCombatHat returns exactly the attacker list / blocker map it
// was loaded with — the "buggy policy" the backstop must contain.
type scriptedCombatHat struct {
	GreedyHatStub
	attackers []*Permanent
	blockMap  map[*Permanent][]*Permanent
}

func (h *scriptedCombatHat) ChooseAttackers(gs *GameState, seatIdx int, legal []*Permanent) []*Permanent {
	return h.attackers
}
func (h *scriptedCombatHat) AssignBlockers(gs *GameState, seatIdx int, attackers []*Permanent) map[*Permanent][]*Permanent {
	return h.blockMap
}

func combatPerm(gs *GameState, seat int, name string, pow, tough int, kws ...string) *Permanent {
	p := &Permanent{
		Card: &Card{
			Name: name, Owner: seat,
			Types:         []string{"creature"},
			BasePower:     pow,
			BaseToughness: tough,
		},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	for _, kw := range kws {
		p.Flags["kw:"+kw] = 1
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func newCombatFixture(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(21)), nil)
	gs.Seed = 21
	gs.Phase = "combat"
	gs.Active = 0
	gs.EventPolicy = EventLogFull
	return gs
}

func countCombatDropEvents(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// 1 + 2: legal attacker commits, off-pool (tapped) attacker drops.
func TestCombatEnforcement_Attackers(t *testing.T) {
	gs := newCombatFixture(t)
	bear := combatPerm(gs, 0, "Legal Bear", 2, 2)
	ogre := combatPerm(gs, 0, "Tapped Ogre", 3, 3)
	ogre.Tapped = true
	hat := &scriptedCombatHat{attackers: []*Permanent{bear, ogre, bear}} // dup bear too
	gs.Seats[0].Hat = hat
	gs.Legality = NewLegalityValidator(21)

	declared := DeclareAttackers(gs, 0)

	if len(declared) != 1 || declared[0] != bear {
		t.Fatalf("expected exactly the legal bear declared, got %d attackers", len(declared))
	}
	if permFlag(ogre, flagAttacking) {
		t.Error("tapped ogre is attacking — off-pool declaration executed")
	}
	if n := countCombatDropEvents(gs, "attack_declaration_dropped"); n != 2 {
		t.Errorf("expected 2 drop events (tapped + duplicate), got %d", n)
	}
	// Raw attempt stays visible to the validator (508.1c tapped).
	found := false
	for _, v := range gs.Legality.Violations {
		if v.Rule == "508.1c" {
			found = true
		}
	}
	if !found {
		t.Errorf("validator lost sight of the dropped tapped attacker; violations=%v", gs.Legality.Violations)
	}
	// No false drops: the legal bear is attacking and untouched by drops.
	if !permFlag(bear, flagAttacking) {
		t.Error("legal bear was not committed")
	}
}

// 3: illegal block shapes drop; the legal block in the same combat
// survives. 5: validator sees the raw map.
func TestCombatEnforcement_IllegalBlocksDropped(t *testing.T) {
	gs := newCombatFixture(t)
	// Attackers (seat 0).
	flyer := combatPerm(gs, 0, "Sky Terror", 2, 2, "flying")
	menacer := combatPerm(gs, 0, "Alley Menace", 3, 3, "menace")
	walker := combatPerm(gs, 0, "Bog Strider", 2, 2, "islandwalk")
	vanilla := combatPerm(gs, 0, "Plain Bruiser", 3, 3)
	attackers := []*Permanent{flyer, menacer, walker, vanilla}
	for _, a := range attackers {
		setPermFlag(a, flagAttacking, true)
	}
	// Defender (seat 1): an Island on the battlefield enables islandwalk.
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, &Permanent{
		Card:       &Card{Name: "Island", Owner: 1, Types: []string{"basic", "land", "island"}},
		Controller: 1, Owner: 1,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	})
	groundling := combatPerm(gs, 1, "Groundling", 2, 2)
	lone := combatPerm(gs, 1, "Lone Guard", 2, 4)
	wallish := combatPerm(gs, 1, "Walk Watcher", 1, 5)
	tappedB := combatPerm(gs, 1, "Tapped Sentry", 2, 2)
	tappedB.Tapped = true
	legalB := combatPerm(gs, 1, "Honest Blocker", 3, 3)

	gs.Seats[1].Hat = &scriptedCombatHat{blockMap: map[*Permanent][]*Permanent{
		flyer:   {groundling},        // §509.1b — no flying/reach
		menacer: {lone},              // §702.110b — menace needs 2
		walker:  {wallish},           // §702.14 — defender controls an Island
		vanilla: {legalB, tappedB},   // legal + a tapped tag-along (§509.1a)
	}}
	gs.Legality = NewLegalityValidator(21)

	out := DeclareBlockers(gs, attackers, 1)

	if len(out[flyer]) != 0 || groundling.IsBlocking() {
		t.Error("groundling block on flyer executed — §509.1b not enforced")
	}
	if len(out[menacer]) != 0 || lone.IsBlocking() {
		t.Error("single block on menace executed — §702.110b not enforced")
	}
	if len(out[walker]) != 0 || wallish.IsBlocking() {
		t.Error("block on islandwalker executed — §702.14 not enforced")
	}
	if tappedB.IsBlocking() {
		t.Error("tapped blocker executed — §509.1a not enforced")
	}
	// The legal block must be COMPLETELY unaffected.
	if len(out[vanilla]) != 1 || out[vanilla][0] != legalB || !legalB.IsBlocking() {
		t.Fatalf("legal block was dropped or altered: %v", out[vanilla])
	}
	if n := countCombatDropEvents(gs, "block_declaration_dropped"); n != 4 {
		t.Errorf("expected 4 drop events, got %d", n)
	}
	// Raw map stays visible: validator recorded the evasion + menace hits.
	rules := map[string]bool{}
	for _, v := range gs.Legality.Violations {
		rules[v.Rule] = true
	}
	for _, want := range []string{"509.1b", "702.110b", "509.1a"} {
		if !rules[want] {
			t.Errorf("validator lost the %s raw-map violation; got rules=%v", want, rules)
		}
	}
}

// 4: the same blocker on two attackers keeps only the first assignment.
func TestCombatEnforcement_DoubleCommittedBlockerDropped(t *testing.T) {
	gs := newCombatFixture(t)
	a1 := combatPerm(gs, 0, "First Raider", 2, 2)
	a2 := combatPerm(gs, 0, "Second Raider", 2, 2)
	setPermFlag(a1, flagAttacking, true)
	setPermFlag(a2, flagAttacking, true)
	greedy := combatPerm(gs, 1, "Overcommitted", 4, 4)

	gs.Seats[1].Hat = &scriptedCombatHat{blockMap: map[*Permanent][]*Permanent{
		a1: {greedy},
		a2: {greedy},
	}}

	out := DeclareBlockers(gs, []*Permanent{a1, a2}, 1)

	total := len(out[a1]) + len(out[a2])
	if total != 1 {
		t.Fatalf("one blocker must hold exactly one assignment, got %d (a1=%d a2=%d)",
			total, len(out[a1]), len(out[a2]))
	}
	if n := countCombatDropEvents(gs, "block_declaration_dropped"); n != 1 {
		t.Errorf("expected 1 drop event for the double commit, got %d", n)
	}
}

// Legal-only combat produces ZERO drop events end-to-end (the no-false-
// positive pin for the common path).
func TestCombatEnforcement_AllLegal_NoDrops(t *testing.T) {
	gs := newCombatFixture(t)
	a1 := combatPerm(gs, 0, "Fair Fighter", 2, 2)
	a2 := combatPerm(gs, 0, "Sky Knight", 2, 2, "flying")
	hat0 := &scriptedCombatHat{attackers: []*Permanent{a1, a2}}
	gs.Seats[0].Hat = hat0

	declared := DeclareAttackers(gs, 0)
	if len(declared) != 2 {
		t.Fatalf("expected both legal attackers declared, got %d", len(declared))
	}

	b1 := combatPerm(gs, 1, "Fair Blocker", 2, 2)
	b2 := combatPerm(gs, 1, "Reach Spider", 1, 4, "reach")
	gs.Seats[1].Hat = &scriptedCombatHat{blockMap: map[*Permanent][]*Permanent{
		a1: {b1},
		a2: {b2}, // reach legally blocks the flyer
	}}
	out := DeclareBlockers(gs, declared, 1)

	if len(out[a1]) != 1 || len(out[a2]) != 1 {
		t.Fatalf("legal blocks dropped: a1=%v a2=%v", out[a1], out[a2])
	}
	if n := countCombatDropEvents(gs, "attack_declaration_dropped") + countCombatDropEvents(gs, "block_declaration_dropped"); n != 0 {
		t.Errorf("legal-only combat produced %d drop events (false positives)", n)
	}
}
