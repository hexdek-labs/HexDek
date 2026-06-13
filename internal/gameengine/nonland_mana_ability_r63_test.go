package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// nonland_mana_ability_r63_test.go — companion to the land-mana audit
// (land_mana_ability_r63_test.go). Pins CR §605.3 inline resolution for
// NON-LAND mana sources: dorks/rocks (already covered generically by the
// add_mana / add_mana_per fix), the untyped_effect count-scaled dork
// mis-parse (Elvish Archdruid), and cost-bearing mana abilities whose
// "discard your hand" cost lived in Cost.Extra and was being skipped
// (Lion's Eye Diamond / Diamond Lion).

// --- Mana dorks / rocks resolve inline and retain priority ------------

// A plain AddMana dork tapped for mana is a §605.1a mana ability: it
// resolves inline (no stack item) so the controller keeps priority and
// can spend the mana mid-payment. This is the generic guarantee the
// modkind fix relies on for every Birds/Llanowar/Sol Ring/Signet.
func TestNonLandMana_Dork_ResolvesInlineRetainsPriority(t *testing.T) {
	gs := newTestGameState(2)
	birds := lmPerm(gs, 0, "Birds of Paradise",
		activatedManaAST("Birds of Paradise", &gameast.AddMana{AnyColorCount: 1}),
		"creature")
	// Not summoning-sick — a real board would have it under control a turn,
	// and the tap-ability path rejects sick creatures (CR §302.6).
	birds.SummoningSick = false

	if !IsManaAbility(birds, 0) {
		t.Fatalf("a {T}: Add one mana of any color dork must be a mana ability")
	}
	if err := ActivateAbility(gs, 0, birds, 0, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	// Inline per §605.3a — never on the stack, so priority is retained.
	if len(gs.Stack) != 0 {
		t.Errorf("mana dork must resolve inline, got %d stack items", len(gs.Stack))
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("expected 1 mana from the dork, got %d", gs.Seats[0].ManaPool)
	}
	if !birds.Tapped {
		t.Errorf("dork should be tapped after paying the {T} cost")
	}
}

// --- Elvish Archdruid: untyped_effect count-scaled dork ----------------

// "Add {G} for each Elf you control" mis-parses to an untyped_effect
// ModKind with the count-scaled payload smuggled into args[0] as
// "add_{g}_per:elf you control". Before the fix this was (a) not a mana
// ability (pushed to the stack, violating §605.3) and (b) inert (the
// untyped_effect resolver produced zero mana). Both must now be correct.
func archdruidAST() *gameast.CardAST {
	return &gameast.CardAST{
		Name: "Elvish Archdruid",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{Tap: true},
				Effect: &gameast.ModificationEffect{
					ModKind: "untyped_effect",
					Args:    []interface{}{"add_{g}_per:elf you control"},
				},
			},
		},
	}
}

func TestNonLandMana_ElvishArchdruid_InlineAndScaled(t *testing.T) {
	gs := newTestGameState(2)
	// Elf typing lives in Types (CountPermanentsByType reads Card.Types).
	// The Archdruid itself plus two more Elves → 3 Elves controlled.
	druid := lmPerm(gs, 0, "Elvish Archdruid", archdruidAST(), "creature", "elf")
	druid.SummoningSick = false
	for i := 0; i < 2; i++ {
		lmPerm(gs, 0, "Llanowar Elves", nil, "creature", "elf")
	}

	if !IsManaAbility(druid, 0) {
		t.Fatalf("untyped_effect add_{g}_per must be classified as a mana ability (§605.1a)")
	}
	if err := ActivateAbility(gs, 0, druid, 0, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("count-scaled dork must resolve inline, got %d stack items", len(gs.Stack))
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("expected 3 green (3 Elves), got %d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Mana == nil || gs.Seats[0].Mana.G != 3 {
		t.Errorf("expected 3 green in typed pool, got %+v", gs.Seats[0].Mana)
	}
}

// --- Lion's Eye Diamond: cost-bearing mana ability ---------------------

// "Discard your hand, Sacrifice this artifact: Add three mana of any one
// color." The discard cost lives in Cost.Extra (not the numeric
// Cost.Discard), so it was being skipped entirely — LED minted 3 mana
// while keeping its hand. It must resolve inline (AddMana, §605.1a) AND
// pay BOTH costs: hand emptied + artifact sacrificed.
func ledAST() *gameast.CardAST {
	return &gameast.CardAST{
		Name: "Lion's Eye Diamond",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{
					Sacrifice: &gameast.Filter{Base: "this", Quantifier: "one"},
					Extra:     []string{"discard your hand"},
				},
				Effect: &gameast.AddMana{AnyColorCount: 3},
			},
		},
	}
}

func TestNonLandMana_LionsEyeDiamond_InlineCostPaid(t *testing.T) {
	gs := newTestGameState(2)
	led := lmPerm(gs, 0, "Lion's Eye Diamond", ledAST(), "artifact")
	// Stock a three-card hand.
	for i := 0; i < 3; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			&Card{Name: "Filler", Owner: 0, Types: []string{"instant"}})
	}

	if !IsManaAbility(led, 0) {
		t.Fatalf("LED is a §605.1a mana ability despite its sac+discard cost")
	}
	if err := ActivateAbility(gs, 0, led, 0, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	// Inline — no stack item, can't be responded to (§605.3a).
	if len(gs.Stack) != 0 {
		t.Errorf("LED must resolve inline, got %d stack items", len(gs.Stack))
	}
	// Mana minted.
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected 3 mana from LED, got %d", gs.Seats[0].ManaPool)
	}
	// Discard cost paid — hand emptied.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("LED must discard the whole hand, %d cards remain", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Turn.Discarded != 3 {
		t.Errorf("expected 3 cards discarded (Turn.Discarded), got %d", gs.Seats[0].Turn.Discarded)
	}
	// Sacrifice cost paid — LED off the battlefield, in graveyard.
	for _, p := range gs.Seats[0].Battlefield {
		if p == led {
			t.Errorf("LED must be sacrificed (off the battlefield)")
		}
	}
}

// Empty-hand LED still activates (discarding zero cards is a legal
// "discard your hand") and still mints mana + sacrifices itself.
func TestNonLandMana_LionsEyeDiamond_EmptyHand(t *testing.T) {
	gs := newTestGameState(2)
	led := lmPerm(gs, 0, "Lion's Eye Diamond", ledAST(), "artifact")
	if err := ActivateAbility(gs, 0, led, 0, nil); err != nil {
		t.Fatalf("activate with empty hand: %v", err)
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected 3 mana even with empty hand, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("LED must resolve inline, got %d stack items", len(gs.Stack))
	}
}
