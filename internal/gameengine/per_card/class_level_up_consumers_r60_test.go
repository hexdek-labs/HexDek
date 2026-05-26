package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// class_level_up_consumers_r60 — 6 handlers wired in
// class_level_up_consumers_r60.go covering AFR Class enchantments
// (CR §716, the "level up" ability).
// -----------------------------------------------------------------------------

// classLevelCtx builds the ctx FireCardTrigger("class_level_up", ctx)
// forwards from ActivateLevelUp (keywords_class.go:270).
func classLevelCtx(src *gameengine.Permanent, prevLevel, newLevel int) map[string]interface{} {
	return map[string]interface{}{
		"source":          src,
		"controller_seat": src.Controller,
		"previous_level":  prevLevel,
		"new_level":       newLevel,
	}
}

// addClassPerm is addPerm + the "class" subtype + a "level X" flag so
// the bearer looks like a properly-modeled Class on the battlefield.
func addClassPerm(gs *gameengine.GameState, seat int, name string, startLevel int) *gameengine.Permanent {
	p := addPerm(gs, seat, name, "enchantment", "class")
	if startLevel > 1 {
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["class_level"] = startLevel
	}
	return p
}

// -----------------------------------------------------------------------------
// Wizard Class — becomes level 2 → draw 2
// -----------------------------------------------------------------------------

func TestWizardClass_DrawsTwoOnLevel2(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top", "Next", "Third")
	wiz := addClassPerm(gs, 0, "Wizard Class", 1)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(wiz, 1, 2))

	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected 2 cards drawn on level 2, got hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestWizardClass_DoesNothingOnLevel3(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top", "Next")
	wiz := addClassPerm(gs, 0, "Wizard Class", 2)

	// Level 3 is the +1/+1 counter on draw trigger — out of scope for
	// this handler. Level-up to 3 must NOT fire the draw-two payoff.
	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(wiz, 2, 3))

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("level-3 should not trigger draw-two; hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestWizardClass_IgnoresAnotherClassLevelUp(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top", "Next")
	wiz := addClassPerm(gs, 0, "Wizard Class", 1)
	other := addClassPerm(gs, 0, "Warlock Class", 1)

	// Warlock Class levels up — Wizard Class's handler should not
	// see itself as the source and must abstain.
	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(other, 1, 2))

	// Note: Warlock Class's own handler MAY fire (warlockClassLevelUp
	// also targets level 2). It dredges top-3 from the library.
	// We just verify Wizard's draw-two didn't fire — Wizard's library
	// should not have lost more than the up-to-3 Warlock dredge.
	// Easiest check: hand size <=1 (Warlock puts 1 to hand).
	if len(gs.Seats[0].Hand) > 1 {
		t.Errorf("Wizard's draw-two must not fire on another Class's level-up; hand=%d", len(gs.Seats[0].Hand))
	}
	_ = wiz
}

// -----------------------------------------------------------------------------
// Warlock Class — becomes level 2 → top 3: 1 to hand, 2 to graveyard
// -----------------------------------------------------------------------------

func TestWarlockClass_DigsTopThreeOnLevel2(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top", "Mid", "Bottom", "Untouched")
	war := addClassPerm(gs, 0, "Warlock Class", 1)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(war, 1, 2))

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand, got %d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Hand[0].DisplayName() != "Top" {
		t.Errorf("expected Top in hand, got %s", gs.Seats[0].Hand[0].DisplayName())
	}
	if len(gs.Seats[0].Graveyard) != 2 {
		t.Errorf("expected 2 milled, got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Library) != 1 || gs.Seats[0].Library[0].DisplayName() != "Untouched" {
		t.Errorf("expected only Untouched left in library, got %v", libraryNames(gs.Seats[0]))
	}
}

func TestWarlockClass_EmptyLibraryEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	war := addClassPerm(gs, 0, "Warlock Class", 1)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(war, 1, 2))

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on empty library; events=%+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// Artificer Class — becomes level 2 → reveal until artifact, put in hand
// -----------------------------------------------------------------------------

func TestArtificerClass_DigsToFirstArtifactOnLevel2(t *testing.T) {
	gs := newGame(t, 2)
	// Library top-down: noise, noise, ARTIFACT, more_noise.
	addLibrary(gs, 0, "Llanowar Elves", "Lightning Bolt")
	gs.Seats[0].Library = append(gs.Seats[0].Library,
		&gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}},
		&gameengine.Card{Name: "Mana Crypt", Owner: 0, Types: []string{"artifact"}},
	)
	art := addClassPerm(gs, 0, "Artificer Class", 1)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(art, 1, 2))

	// Sol Ring → hand. The two non-artifacts go to the bottom.
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Sol Ring" {
			foundInHand = true
			break
		}
	}
	if !foundInHand {
		t.Errorf("expected Sol Ring in hand; hand=%v", handNames(gs.Seats[0].Hand))
	}
	// Library still has Mana Crypt (was after Sol Ring) + 2 bottom
	// reveals. Total = 3.
	if len(gs.Seats[0].Library) != 3 {
		t.Errorf("expected 3 cards remaining in library, got %d (%v)",
			len(gs.Seats[0].Library), libraryNames(gs.Seats[0]))
	}
}

func TestArtificerClass_NoArtifactEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Llanowar Elves", "Lightning Bolt")
	art := addClassPerm(gs, 0, "Artificer Class", 1)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(art, 1, 2))

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no artifact in library; events=%+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// Monk Class — becomes level 2 → bounce target nonland permanent
// -----------------------------------------------------------------------------

func TestMonkClass_BouncesBiggestOppNonlandOnLevel2(t *testing.T) {
	gs := newGame(t, 2)
	weak := addPerm(gs, 1, "Sol Ring", "artifact")
	weak.Card.Types = append(weak.Card.Types, "cmc:1")
	big := addPerm(gs, 1, "Mind's Eye", "artifact")
	big.Card.Types = append(big.Card.Types, "cmc:5")
	monk := addClassPerm(gs, 0, "Monk Class", 1)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(monk, 1, 2))

	for _, p := range gs.Seats[1].Battlefield {
		if p == big {
			t.Fatalf("Mind's Eye should have been bounced (highest CMC)")
		}
	}
	// Weak Sol Ring should still be on battlefield.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == weak {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Errorf("Sol Ring (lower CMC) should not have been bounced")
	}
}

func TestMonkClass_LandsAreNeverBounced(t *testing.T) {
	gs := newGame(t, 2)
	oppLand := addPerm(gs, 1, "Plains", "land")
	monk := addClassPerm(gs, 0, "Monk Class", 1)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(monk, 1, 2))

	stillOnBf := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppLand {
			stillOnBf = true
			break
		}
	}
	if !stillOnBf {
		t.Errorf("land should never be bounced (oracle: nonland permanent)")
	}
}

func TestMonkClass_LevelOtherThan2DoesNothing(t *testing.T) {
	gs := newGame(t, 2)
	oppArt := addPerm(gs, 1, "Sol Ring", "artifact")
	monk := addClassPerm(gs, 0, "Monk Class", 2)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(monk, 2, 3))

	stillOnBf := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppArt {
			stillOnBf = true
			break
		}
	}
	if !stillOnBf {
		t.Errorf("level 3 must not trigger the bounce payoff")
	}
}

// -----------------------------------------------------------------------------
// Stormchaser's Talent — becomes level 2 → return target i/s from grave
// -----------------------------------------------------------------------------

func TestStormchasersTalent_ReturnsBestInstantOnLevel2(t *testing.T) {
	gs := newGame(t, 2)
	cheap := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant", "cmc:1"}}
	big := &gameengine.Card{Name: "Decree of Pain", Owner: 0, Types: []string{"sorcery", "cmc:8"}}
	noise := &gameengine.Card{Name: "Tarmogoyf", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, cheap, big, noise)

	talent := addClassPerm(gs, 0, "Stormchaser's Talent", 1)
	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(talent, 1, 2))

	foundDecree := false
	for _, c := range gs.Seats[0].Hand {
		if c == big {
			foundDecree = true
			break
		}
	}
	if !foundDecree {
		t.Errorf("expected Decree of Pain in hand (highest-CMC i/s); hand=%v", handNames(gs.Seats[0].Hand))
	}
}

func TestStormchasersTalent_NoIsTargetEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	// Graveyard has only a creature — no i/s.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature"}},
	)
	talent := addClassPerm(gs, 0, "Stormchaser's Talent", 1)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(talent, 1, 2))

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no i/s in graveyard; events=%+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// Builder's Talent — becomes level 3 → reanimate noncreature/nonland
// -----------------------------------------------------------------------------

func TestBuildersTalent_ReanimatesBestArtifactOnLevel3(t *testing.T) {
	gs := newGame(t, 2)
	weakArt := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact", "cmc:1"}}
	bigArt := &gameengine.Card{Name: "Mind's Eye", Owner: 0, Types: []string{"artifact", "cmc:5"}}
	creature := &gameengine.Card{Name: "Tarmogoyf", Owner: 0, Types: []string{"creature"}}
	land := &gameengine.Card{Name: "Wasteland", Owner: 0, Types: []string{"land"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, weakArt, bigArt, creature, land)

	talent := addClassPerm(gs, 0, "Builder's Talent", 2)
	pre := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(talent, 2, 3))

	// Mind's Eye should be on battlefield.
	foundBig := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == bigArt {
			foundBig = true
			break
		}
	}
	if !foundBig {
		t.Errorf("expected Mind's Eye (highest-CMC noncreature/nonland) on battlefield")
	}
	if len(gs.Seats[0].Battlefield) != pre+1 {
		t.Errorf("expected battlefield +1 (the reanimated perm), pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
	// Creature stays in graveyard.
	stillCreature := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == creature {
			stillCreature = true
			break
		}
	}
	if !stillCreature {
		t.Errorf("creature should stay in graveyard (oracle: noncreature, nonland)")
	}
}

func TestBuildersTalent_LevelTwoDoesNotReanimate(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact", "cmc:1"}},
	)
	talent := addClassPerm(gs, 0, "Builder's Talent", 1)
	pre := len(gs.Seats[0].Battlefield)

	gameengine.FireCardTrigger(gs, "class_level_up", classLevelCtx(talent, 1, 2))

	if len(gs.Seats[0].Battlefield) != pre {
		t.Errorf("level 2 must not trigger reanimation; pre=%d post=%d",
			pre, len(gs.Seats[0].Battlefield))
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — every handler reachable post-registerDefaults.
// -----------------------------------------------------------------------------

func TestClassLevelUpConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Wizard Class",
		"Warlock Class",
		"Artificer Class",
		"Monk Class",
		"Stormchaser's Talent",
		"Builder's Talent",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler for %s", name)
		}
	}
}

// -----------------------------------------------------------------------------
// Test helper
// -----------------------------------------------------------------------------

func libraryNames(seat *gameengine.Seat) []string {
	out := []string{}
	for _, c := range seat.Library {
		if c == nil {
			continue
		}
		out = append(out, c.DisplayName())
	}
	return out
}
