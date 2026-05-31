package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestKnowledgePool_LTBClearsExiledByTimestamp pins the Loki r60
// follow-up to PR #800 — 30 ExileLinkageIntegrity hits across 3
// games traced to Knowledge Pool stamping ExiledByTimestamp on its
// imprinted cards as a discovery tag, then dying and leaving the
// timestamp orphaned. (Game 2029 Leonardo, Sewer Samurai × 26;
// game 1044 Myr Prototype × 2; game 149 Great Hall of the
// Biblioplex × 2.)
//
// Knowledge Pool's exile semantics is CastGrant, NOT LTBReturn —
// the cards STAY exiled forever per oracle, but the engine's tagging
// key (ExiledByTimestamp) is reused by the LTBReturn invariant
// machinery. Fix: clear the tag on LTB; cards remain in exile.
//
// Depends on PR #800 — fireTrigger now dispatches permanent_ltb
// against ctx["perm"] for leaving perms, so Knowledge Pool's
// permanent_ltb handler runs even after KP itself has been removed.
func TestKnowledgePool_LTBClearsExiledByTimestamp(t *testing.T) {
	gs := newTestGS(2)
	kpCard := &gameengine.Card{Name: "Knowledge Pool", Owner: 0, InstanceID: "hKP_test", Types: []string{"artifact"}}
	kp := &gameengine.Permanent{
		Card: kpCard, Controller: 0, Owner: 0, Timestamp: 100,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{kp}

	// Seat 0's library top 3 cards — these get exiled at ETB.
	impr0a := &gameengine.Card{Name: "Imprint A", Owner: 0, Types: []string{"creature"}}
	impr0b := &gameengine.Card{Name: "Imprint B", Owner: 0, Types: []string{"creature"}}
	impr0c := &gameengine.Card{Name: "Imprint C", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*gameengine.Card{impr0a, impr0b, impr0c}
	// Seat 1 library top 3.
	impr1a := &gameengine.Card{Name: "Imprint D", Owner: 1, Types: []string{"creature"}}
	impr1b := &gameengine.Card{Name: "Imprint E", Owner: 1, Types: []string{"creature"}}
	impr1c := &gameengine.Card{Name: "Imprint F", Owner: 1, Types: []string{"creature"}}
	gs.Seats[1].Library = []*gameengine.Card{impr1a, impr1b, impr1c}

	knowledgePoolETB(gs, kp)

	// Every imprinted card must now carry ExiledByTimestamp = KP.Timestamp.
	allImprints := []*gameengine.Card{impr0a, impr0b, impr0c, impr1a, impr1b, impr1c}
	for _, c := range allImprints {
		if c.ExiledByTimestamp != kp.Timestamp {
			t.Fatalf("%s: post-ETB ExiledByTimestamp=%d, want %d",
				c.Name, c.ExiledByTimestamp, kp.Timestamp)
		}
	}

	// Pre-fix counterfactual: if we destroy KP NOW, the invariant should
	// flag every imprinted card. Without the LTB handler, ELI fires.
	// Post-fix: the LTB handler clears the tags and ELI stays clean.
	gameengine.DestroyPermanent(gs, kp, nil)

	for _, c := range allImprints {
		if c.ExiledByTimestamp != 0 {
			t.Fatalf("%s: post-LTB ExiledByTimestamp=%d, want 0 (KP LTB-clear missed)",
				c.Name, c.ExiledByTimestamp)
		}
	}

	// ExileLinkageIntegrity must be clean.
	for _, inv := range gameengine.AllInvariants() {
		if inv.Name != "ExileLinkageIntegrity" {
			continue
		}
		if err := inv.Check(gs); err != nil {
			t.Fatalf("ExileLinkageIntegrity: %v", err)
		}
	}

	// Cards must STILL be in their owner's exile zone (KP doesn't return
	// imprinted cards — they stay exiled per oracle; only the linkage
	// tag was cleared).
	for _, c := range allImprints {
		found := false
		for _, ex := range gs.Seats[c.Owner].Exile {
			if ex == c {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: post-LTB no longer in seat %d exile — LTB should NOT return KP imprints",
				c.Name, c.Owner)
		}
	}
}

// TestKnowledgePool_SeatEliminationClearsTags pins the Loki r60 2nd-
// round residual: KP's controller (seat 1) was eliminated in game
// 1044 before KP itself was destroyed, so the per_card permanent_ltb
// handler never ran — HandleSeatElimination removes permanents
// directly without firing trigger dispatch. The fix routes through
// the new ClearLinkedExileTagsForSource helper, called inline with
// ExpireSourceGrants in the elimination loop.
func TestKnowledgePool_SeatEliminationClearsTags(t *testing.T) {
	gs := newTestGS(2)
	kpCard := &gameengine.Card{Name: "Knowledge Pool", Owner: 0, InstanceID: "hKP_elim", Types: []string{"artifact"}}
	kp := &gameengine.Permanent{
		Card: kpCard, Controller: 0, Owner: 0, Timestamp: 200,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{kp}

	imprint := &gameengine.Card{Name: "Imprinted Card", Owner: 1, Types: []string{"creature"}}
	gs.Seats[1].Library = []*gameengine.Card{imprint}

	knowledgePoolETB(gs, kp)
	if imprint.ExiledByTimestamp != kp.Timestamp {
		t.Fatalf("post-ETB ExiledByTimestamp=%d, want %d", imprint.ExiledByTimestamp, kp.Timestamp)
	}

	// Eliminate KP's controller (seat 0). HandleSeatElimination removes
	// KP without firing permanent_ltb. The ClearLinkedExileTagsForSource
	// call now wired into the elimination loop must reset the tag.
	gs.Seats[0].Life = 0
	gameengine.HandleSeatElimination(gs, 0)

	if imprint.ExiledByTimestamp != 0 {
		t.Fatalf("post-seat-elim ExiledByTimestamp=%d, want 0 (ClearLinkedExileTagsForSource missed)",
			imprint.ExiledByTimestamp)
	}

	// ExileLinkageIntegrity must be clean.
	for _, inv := range gameengine.AllInvariants() {
		if inv.Name != "ExileLinkageIntegrity" {
			continue
		}
		if err := inv.Check(gs); err != nil {
			t.Fatalf("ExileLinkageIntegrity post-seat-elim: %v", err)
		}
	}
}
