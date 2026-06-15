package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// block_additional_cap_r63_test.go — firespot (7174n1c): the
// "can block an additional N creatures" / "can block any number of
// creatures" capability. Before this pass the multi-block gate was a bare
// boolean text-match (legalityCanMultiBlock) with NO numeric cap: a
// creature whose text contained "block an additional" was permitted to
// block UNLIMITED attackers (the card's actual N was never read), while
// the effect-granted blocks_additional flag was dead. These pin the cap.

// addBlockerWithText puts a creature carrying the given oracle text (as a
// Static.Raw ability) onto seat's battlefield so multiBlockCap can read it.
func addBlockerWithText(gs *GameState, seat int, name string, pow, tough int, oracle string) *Permanent {
	p := addCreature(gs, seat, name, pow, tough)
	p.Card.AST = &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Static{Raw: oracle}},
	}
	return p
}

func TestMultiBlockCap_ReadsCardN(t *testing.T) {
	gs := newCombatGame(t)

	vanilla := addCreature(gs, 1, "Grizzly Bears", 2, 2)
	if got := multiBlockCap(vanilla); got != 1 {
		t.Fatalf("vanilla creature cap = %d, want 1", got)
	}

	one := addBlockerWithText(gs, 1, "Wall of Caltrops", 1, 4, "Wall of Caltrops can block an additional creature each combat.")
	if got := multiBlockCap(one); got != 2 {
		t.Fatalf("'block an additional creature' cap = %d, want 2", got)
	}

	// Hundred-Handed One must read 100 (1 + ninety-nine), NOT a hardcoded 2.
	hundred := addBlockerWithText(gs, 1, "Hundred-Handed One", 3, 5,
		"Reach. Hundred-Handed One can block an additional ninety-nine creatures each combat as long as it's monstrous.")
	if got := multiBlockCap(hundred); got != 100 {
		t.Fatalf("Hundred-Handed One cap = %d, want 100", got)
	}

	any := addBlockerWithText(gs, 1, "Mr. Orfeo, the Boulder", 6, 6, "Mr. Orfeo can block any number of creatures.")
	if got := multiBlockCap(any); got < 1<<20 {
		t.Fatalf("'block any number' cap = %d, want unbounded sentinel", got)
	}
}

func TestParseAdditionalBlockCount(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"can block an additional creature each combat.", 1},
		{"can block an additional two creatures.", 2},
		{"can block an additional ninety-nine creatures each combat", 99},
		{"can block an additional ninety nine creatures", 99},
		{"can block an additional 5 creatures", 5},
		{"can block an additional twenty-five creatures", 25},
		{"reach. flying.", 0}, // phrase absent
	}
	for _, c := range cases {
		if got := parseAdditionalBlockCount(c.text); got != c.want {
			t.Errorf("parseAdditionalBlockCount(%q) = %d, want %d", c.text, got, c.want)
		}
	}
}

// Property (b): Hundred-Handed One legitimately blocks THREE different
// attackers — simulating the per-attacker DeclareBlockers commit loop.
func TestSanitize_HundredHandedOne_BlocksThreeAttackers(t *testing.T) {
	gs := newCombatGame(t)
	a1 := addCreature(gs, 0, "Attacker One", 2, 2)
	a2 := addCreature(gs, 0, "Attacker Two", 2, 2)
	a3 := addCreature(gs, 0, "Attacker Three", 2, 2)
	hundred := addBlockerWithText(gs, 1, "Hundred-Handed One", 3, 5,
		"Reach. Hundred-Handed One can block an additional ninety-nine creatures each combat as long as it's monstrous.")

	for i, atk := range []*Permanent{a1, a2, a3} {
		kept := sanitizeDeclaredBlockers(gs, 1, atk, []*Permanent{hundred})
		if len(kept) != 1 || kept[0] != hundred {
			t.Fatalf("attacker %d: Hundred-Handed One should be a legal blocker, got %d kept", i+1, len(kept))
		}
		// mirror the engine: commit between attacker declarations.
		commitBlock(hundred)
	}
	if got := blockCommitCountOf(hundred); got != 3 {
		t.Fatalf("commit count = %d after 3 blocks, want 3", got)
	}
}

// Property (a): a normal creature blocks exactly ONE attacker — the second
// attacker's declaration drops it as over-cap.
func TestSanitize_NormalCreature_OnlyOneAttacker(t *testing.T) {
	gs := newCombatGame(t)
	a1 := addCreature(gs, 0, "Attacker One", 2, 2)
	a2 := addCreature(gs, 0, "Attacker Two", 2, 2)
	b := addCreature(gs, 1, "Grizzly Bears", 2, 2)

	kept1 := sanitizeDeclaredBlockers(gs, 1, a1, []*Permanent{b})
	if len(kept1) != 1 {
		t.Fatalf("first block should be legal, got %d kept", len(kept1))
	}
	commitBlock(b)

	kept2 := sanitizeDeclaredBlockers(gs, 1, a2, []*Permanent{b})
	if len(kept2) != 0 {
		t.Fatalf("normal creature must NOT block a second attacker, got %d kept", len(kept2))
	}
}

// Property (d): the §509.1 same blocker+attacker pair double-commit is STILL
// rejected — even for a creature that may block additional attackers.
func TestSanitize_SamePairTwice_RejectedEvenForMultiBlocker(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Lone Attacker", 3, 3)
	hundred := addBlockerWithText(gs, 1, "Hundred-Handed One", 3, 5,
		"Hundred-Handed One can block an additional ninety-nine creatures each combat.")

	kept := sanitizeDeclaredBlockers(gs, 1, atk, []*Permanent{hundred, hundred})
	if len(kept) != 1 || kept[0] != hundred {
		t.Fatalf("same pair twice must collapse to ONE even for a multi-blocker, got %d kept", len(kept))
	}
}

// Property (d) over-cap: a finite-N multi-blocker is dropped once it would
// exceed 1+N total attackers (the bug: previously unlimited).
func TestSanitize_FiniteN_DroppedBeyondCap(t *testing.T) {
	gs := newCombatGame(t)
	a1 := addCreature(gs, 0, "Attacker One", 2, 2)
	a2 := addCreature(gs, 0, "Attacker Two", 2, 2)
	a3 := addCreature(gs, 0, "Attacker Three", 2, 2)
	// cap = 1 + 1 = 2.
	wall := addBlockerWithText(gs, 1, "Wall of Caltrops", 1, 4, "Wall of Caltrops can block an additional creature each combat.")

	for i, atk := range []*Permanent{a1, a2} {
		kept := sanitizeDeclaredBlockers(gs, 1, atk, []*Permanent{wall})
		if len(kept) != 1 {
			t.Fatalf("attacker %d within cap should keep the blocker, got %d", i+1, len(kept))
		}
		commitBlock(wall)
	}
	kept3 := sanitizeDeclaredBlockers(gs, 1, a3, []*Permanent{wall})
	if len(kept3) != 0 {
		t.Fatalf("third attacker exceeds cap 2; blocker must be dropped, got %d kept", len(kept3))
	}
}

// Property (e): the ride-along legality validator agrees — within-cap is
// clean, beyond-cap is flagged §509.1.
func TestCheckLegality_MultiBlock_CapHonored(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Attacker", 4, 4)
	hundred := addBlockerWithText(gs, 1, "Hundred-Handed One", 3, 5,
		"Hundred-Handed One can block an additional ninety-nine creatures each combat.")
	normal := addCreature(gs, 1, "Grizzly Bears", 2, 2)

	// Hundred-Handed One already committed to 2 attackers — still within cap 100.
	cleanObs := &LegalityObservation{
		Kind:                    "block",
		Seat:                    1,
		Attacker:                atk,
		Blockers:                []*Permanent{hundred},
		BlockersPriorBlockCount: []int{2},
	}
	if viols := checkLegalityBlockDecl(gs, cleanObs); len(viols) != 0 {
		t.Fatalf("within-cap multi-block must be clean, got %+v", viols)
	}

	// Normal creature already blocking 1 attacker — a second is illegal.
	overObs := &LegalityObservation{
		Kind:                    "block",
		Seat:                    1,
		Attacker:                atk,
		Blockers:                []*Permanent{normal},
		BlockersPriorBlockCount: []int{1},
	}
	viols := checkLegalityBlockDecl(gs, overObs)
	found := false
	for _, v := range viols {
		if v.Rule == "509.1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("normal creature blocking a 2nd attacker must be flagged §509.1, got %+v", viols)
	}
}

// The effect-granted block_additional_creature modification raises the cap
// (previously the blocks_additional flag was written but never read).
func TestEffectGrantedBlockAdditional_RaisesCap(t *testing.T) {
	gs := newCombatGame(t)
	b := addCreature(gs, 1, "Granted Blocker", 2, 2)
	if got := multiBlockCap(b); got != 1 {
		t.Fatalf("ungranted cap = %d, want 1", got)
	}
	resolveModificationEffect(gs, b, &gameast.ModificationEffect{ModKind: "block_additional_creature"})
	if got := b.Flags["blocks_additional"]; got != 1 {
		t.Fatalf("blocks_additional flag = %d, want 1", got)
	}
	if got := multiBlockCap(b); got != 2 {
		t.Fatalf("granted cap = %d, want 2", got)
	}
}
