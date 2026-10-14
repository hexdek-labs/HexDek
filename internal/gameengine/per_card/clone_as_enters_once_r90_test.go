package per_card

// clone_as_enters_once_r90_test.go — regressions for the r90 prod crash:
// an "enters as a copy" replacement effect applying more than once to the
// same permanent (CR 614.1c + 614.5).
//
// MEASUREMENT NOTE ON THE PRE-FIX FAILURE MODE. The three termination
// tests below do NOT merely fail against a semantics-only revert — they
// take the whole test binary down with `fatal error: stack overflow`,
// because that is precisely the prod failure being pinned. A reverted run
// reports a crashed package, not a red test. That is the correct and
// expected shape here, and it is worth stating because "the suite went
// red" and "the suite died" look different in CI output and only one of
// them is this bug.
//
// The fixtures deliberately give each clone body a real 1/1 so the
// §704.5f SBA can't quietly empty the battlefield mid-dispatch (the r87
// harness trap), even though these paths don't open a priority round.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r90Clone places a clone-family permanent that is still named after its
// own printed card — the state that makes it a legal copy target for the
// next one, and the state the prod crash depended on.
func r90Clone(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Permanent {
	p := addPerm(gs, seat, name, types...)
	p.Card.BasePower = 1
	p.Card.BaseToughness = 1
	return p
}

// TestCloneAsEntersR90_CopyEnchantmentCopyingCopyEnchantmentTerminates is
// the prod crash itself. A Copy Enchantment that entered with no legal
// target stays named "Copy Enchantment" and is therefore a legal
// enchantment for the next one. Copying it leaves the copy named "Copy
// Enchantment" too, so fireETBOnCopy re-selects copyEnchantmentETB by
// name — forever, because scanCloneTargets excludes only `self` and the
// first one never stops being a legal target.
func TestCloneAsEntersR90_CopyEnchantmentCopyingCopyEnchantmentTerminates(t *testing.T) {
	gs := newGame(t, 2)

	first := r90Clone(gs, 0, "Copy Enchantment", "enchantment")
	second := r90Clone(gs, 0, "Copy Enchantment", "enchantment")

	// Entry point: the registered ETB handler, exactly as the engine
	// dispatches it. If the replacement can re-apply, this never returns.
	copyEnchantmentETB(gs, second)

	if second.Flags[cloneAsEntersMarker] != 1 {
		t.Fatalf("second Copy Enchantment did not claim its as-enters opportunity")
	}
	// The first one never entered as a copy in this fixture, so it must
	// not have been marked by the second one's resolution.
	if first.Flags[cloneAsEntersMarker] == 1 {
		t.Errorf("resolving the second Copy Enchantment wrongly spent the first one's opportunity")
	}
}

// TestCloneAsEntersR90_CloneCopyingCloneTerminates is the same shape on
// the creature side of the shared helper.
func TestCloneAsEntersR90_CloneCopyingCloneTerminates(t *testing.T) {
	gs := newGame(t, 2)

	r90Clone(gs, 0, "Clone", "creature")
	second := r90Clone(gs, 0, "Clone", "creature")

	cloneETB(gs, second)

	if second.Flags[cloneAsEntersMarker] != 1 {
		t.Fatalf("Clone did not claim its as-enters opportunity")
	}
}

// TestCloneAsEntersR90_CrossNameCycleTerminates is the case a name-based
// guard structurally cannot catch, and the reason the fix is a
// once-per-permanent claim rather than a self-name check: Phyrexian
// Metamorph enters as a copy of a Clone, whose handler would then run a
// second as-enters copy on it under a DIFFERENT name than its own.
func TestCloneAsEntersR90_CrossNameCycleTerminates(t *testing.T) {
	gs := newGame(t, 2)

	r90Clone(gs, 0, "Clone", "creature")
	metamorph := r90Clone(gs, 0, "Phyrexian Metamorph", "artifact", "creature")

	phyrexianMetamorphETB(gs, metamorph)

	if metamorph.Flags[cloneAsEntersMarker] != 1 {
		t.Fatalf("Phyrexian Metamorph did not claim its as-enters opportunity")
	}
	if got := metamorph.Card.DisplayName(); got != "Clone" {
		t.Fatalf("Metamorph should have entered as a copy of Clone, got %q", got)
	}
}

// TestCloneAsEntersR90_OrdinaryCopyStillFiresTheCopiedETB is the
// grant-direction half: the fix must not make fireETBOnCopy quiet.
//
// MEASURED: this test is QUIET against a semantics-only revert, by
// design. It is an over-suppression guard — it pins what must keep
// working, so it passes both before and after. Five of the seven tests
// in this file do fail against the revert; this one and
// FreshPermanentGetsItsOwnOpportunity are the two that must not. A
// Clone copying a real creature still becomes it, and the copied card's
// own ETB still runs — here proved through the as-enters counters that
// r63 wired onto this same chokepoint.
func TestCloneAsEntersR90_OrdinaryCopyStillFiresTheCopiedETB(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)

	target := &gameengine.Permanent{
		Card: scourgeLikeCard(0), Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)

	clone := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Clone", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, clone)

	cloneETB(gs, clone)

	if got := clone.Card.DisplayName(); got != "Scourge of Skola Vale" {
		t.Fatalf("Clone did not become a copy of the target, got %q", got)
	}
	if got := clone.Counters["+1/+1"]; got != 2 {
		t.Fatalf("copied card's as-enters counters did not fire: %d, want 2 — the r90 claim must not silence fireETBOnCopy", got)
	}
}

// TestCloneAsEntersR90_MimeoplasmDoesNotExileTwice is the finite sibling
// of the crash, driven through the real entry point: a Clone enters as a
// copy of The Mimeoplasm, so fireETBOnCopy dispatches Mimeoplasm's own
// "as this enters, exile two creature cards from graveyards and enter as
// a copy of one" replacement onto a permanent that has ALREADY entered.
//
// This one never hung — nothing in the Mimeoplasm handler calls
// fireETBOnCopy — so there was no crash to find it by. It just quietly
// exiled two cards out of a graveyard every time it happened. The cost
// is what makes it visible, and it is why the fix is the rule rather
// than a recursion guard: a recursion guard would still have let this
// one through.
func TestCloneAsEntersR90_MimeoplasmDoesNotExileTwice(t *testing.T) {
	gs := newGame(t, 2)

	// Two creature cards in a graveyard — the cost Mimeoplasm's
	// replacement pays as it applies.
	for _, n := range []string{"Bone Shredder", "Gravecrawler"} {
		gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, &gameengine.Card{
			Name: n, Owner: 1, Types: []string{"creature"},
			BasePower: 2, BaseToughness: 2,
		})
	}
	wantGY := len(gs.Seats[1].Graveyard)

	// The Mimeoplasm is on the battlefield under its own name, already
	// entered. A Clone enters and copies it.
	r90Clone(gs, 0, "The Mimeoplasm", "creature")
	clone := r90Clone(gs, 0, "Clone", "creature")

	cloneETB(gs, clone)

	if got := clone.Card.DisplayName(); got != "The Mimeoplasm" {
		t.Fatalf("setup: Clone did not enter as a copy of The Mimeoplasm, got %q", got)
	}
	if got := len(gs.Seats[1].Graveyard); got != wantGY {
		t.Errorf("a second as-enters-copy replacement applied and paid its cost again: graveyard %d, want %d (CR 614.5)", got, wantGY)
	}
}

// TestCloneAsEntersR90_SecondAsEntersCopyIsRefused pins the rule at the
// shared helper with an observable difference. The earlier draft of this
// test was VACUOUS and I kept the corrected version rather than the
// finding: it re-ran the second handler against a board whose only legal
// target was the card already copied, so the name never changed and the
// assertion held pre-fix too. The fixture now offers the second handler
// a target the first one could not take — Phyrexian Metamorph's
// predicate admits artifacts, Clone's does not — so a second application
// is visible as a different name.
func TestCloneAsEntersR90_SecondAsEntersCopyIsRefused(t *testing.T) {
	gs := newGame(t, 2)

	bear := r90Clone(gs, 1, "Grizzly Bears", "creature")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2

	clone := r90Clone(gs, 0, "Clone", "creature")
	cloneETB(gs, clone)
	if got := clone.Card.DisplayName(); got != "Grizzly Bears" {
		t.Fatalf("setup: Clone did not copy the bear, got %q", got)
	}

	// A noncreature artifact under the clone's own controller: invisible
	// to Clone's predicate, preferred by Metamorph's (same-controller
	// scan runs first).
	r90Clone(gs, 0, "Ornithopter", "artifact")

	phyrexianMetamorphETB(gs, clone)

	if got := clone.Card.DisplayName(); got != "Grizzly Bears" {
		t.Errorf("a second as-enters copy applied to a permanent that already entered: became %q, want it left as %q (CR 614.5)", got, "Grizzly Bears")
	}
}

// TestCloneAsEntersR90_FreshPermanentGetsItsOwnOpportunity guards against
// over-suppression: the claim is per-Permanent, so a permanent that
// leaves and re-enters — a new object with fresh Flags — must get a fresh
// opportunity. A marker parked on the Card instead would break blink.
//
// MEASURED: quiet against a semantics-only revert, by design — the
// second of the two over-suppression guards.
func TestCloneAsEntersR90_FreshPermanentGetsItsOwnOpportunity(t *testing.T) {
	gs := newGame(t, 2)

	victim := r90Clone(gs, 1, "Grizzly Bears", "creature")
	victim.Card.BasePower = 2
	victim.Card.BaseToughness = 2

	firstEntry := r90Clone(gs, 0, "Clone", "creature")
	cloneETB(gs, firstEntry)
	if firstEntry.Card.DisplayName() != "Grizzly Bears" {
		t.Fatalf("setup: first Clone entry did not copy")
	}

	// Same card, new battlefield object (what a blink produces).
	secondEntry := r90Clone(gs, 0, "Clone", "creature")
	cloneETB(gs, secondEntry)
	if got := secondEntry.Card.DisplayName(); got != "Grizzly Bears" {
		t.Fatalf("a re-entering permanent was denied its own as-enters opportunity, got %q", got)
	}
}
