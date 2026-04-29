package hat

// GreedyHat behavior tests — lock in byte-equivalent-to-inline-engine
// behavior so Phase 12 parity testing against the Python engine doesn't
// fail from policy drift.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestGreedy_ChooseMulligan_AlwaysKeeps(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	got := h.ChooseMulligan(gs, 0, gs.Seats[0].Hand)
	if got {
		t.Fatalf("GreedyHat.ChooseMulligan should keep every opener; got true")
	}
}

func TestGreedy_ChooseCastFromHand_BiggestFirst(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	cheap := newTestCardMinimal("Cheap", []string{"creature"}, 1, nil)
	big := newTestCardMinimal("Big", []string{"creature"}, 6, nil)
	mid := newTestCardMinimal("Mid", []string{"creature"}, 3, nil)
	castable := []*gameengine.Card{cheap, mid, big}
	got := h.ChooseCastFromHand(gs, 0, castable)
	if got == nil || got.DisplayName() != "Big" {
		t.Fatalf("want Big (highest CMC); got %v", got)
	}
}

func TestGreedy_ChooseCastFromHand_SkipsCounterspells(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	// Build a card whose AST contains a CounterSpell effect — biggest
	// CMC but should be skipped.
	counter := newTestCardMinimal("Counterspell", []string{"instant"}, 7,
		&gameast.CardAST{
			Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: &gameast.CounterSpell{Target: gameast.Filter{Base: "spell", Targeted: true}},
				},
			},
		})
	small := newTestCardMinimal("Small", []string{"creature"}, 2, nil)
	got := h.ChooseCastFromHand(gs, 0, []*gameengine.Card{counter, small})
	if got == nil || got.DisplayName() != "Small" {
		t.Fatalf("want Small (counter should be skipped); got %v", got)
	}
}

func TestGreedy_ChooseAttackers_ReturnsAllLegal(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	p1 := newTestPermanent(gs.Seats[0], newTestCardMinimal("A", []string{"creature"}, 1, nil), 2, 2)
	p2 := newTestPermanent(gs.Seats[0], newTestCardMinimal("B", []string{"creature"}, 2, nil), 3, 2)
	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{p1, p2})
	if len(got) != 2 {
		t.Fatalf("want 2 attackers; got %d", len(got))
	}
}

func TestGreedy_ChooseAttackTarget_LeadingPlayer(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 3)
	// Seat 2 is the leader (highest life + board power). GreedyHat now
	// targets the leading player to prevent runaway leaders, matching
	// real Commander politics.
	gs.Seats[1].Life = 5
	gs.Seats[2].Life = 20
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("X", []string{"creature"}, 1, nil), 3, 3)
	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("want seat 2 (leading player with life=20); got %d", got)
	}
}

func TestGreedy_ChooseResponse_CountersIfCounterInHand(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	// Seat 0 has enough mana + a counter in hand.
	gs.Seats[0].ManaPool = 3
	counter := newTestCardMinimal("Counterspell", []string{"instant"}, 2,
		&gameast.CardAST{
			Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: &gameast.CounterSpell{Target: gameast.Filter{Base: "spell", Targeted: true}},
				},
			},
		})
	gs.Seats[0].Hand = []*gameengine.Card{counter}
	top := &gameengine.StackItem{
		Controller: 1,
		Card:       newTestCardMinimal("EnemySpell", []string{"sorcery"}, 3, nil),
		Effect:     &gameast.Damage{},
	}
	got := h.ChooseResponse(gs, 0, top)
	if got == nil {
		t.Fatal("GreedyHat should counter an opponent's spell when a counter is in hand")
	}
	if got.Card.DisplayName() != "Counterspell" {
		t.Fatalf("counter returned wrong card: %v", got.Card.DisplayName())
	}
}

func TestGreedy_ChooseResponse_PassesOwnSpell(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	top := &gameengine.StackItem{Controller: 0, Card: &gameengine.Card{Name: "MySpell"}, Effect: &gameast.Damage{}}
	got := h.ChooseResponse(gs, 0, top)
	if got != nil {
		t.Fatalf("should not counter own spell; got %v", got)
	}
}

func TestGreedy_ChooseTarget_FirstLegal(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	legal := []gameengine.Target{
		{Kind: gameengine.TargetKindSeat, Seat: 1},
		{Kind: gameengine.TargetKindSeat, Seat: 2},
	}
	got := h.ChooseTarget(gs, 0, gameast.TargetPlayer(), legal)
	if got.Seat != 1 {
		t.Fatalf("want first legal target (seat 1); got %d", got.Seat)
	}
}

func TestGreedy_OrderReplacements_TimestampSort(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	r1 := &gameengine.ReplacementEffect{Timestamp: 3, ControllerSeat: 1}
	r2 := &gameengine.ReplacementEffect{Timestamp: 1, ControllerSeat: 0} // self-controlled
	r3 := &gameengine.ReplacementEffect{Timestamp: 2, ControllerSeat: 1}
	got := h.OrderReplacements(gs, 0, []*gameengine.ReplacementEffect{r1, r2, r3})
	// Self-controlled first (r2), then remaining by timestamp.
	if len(got) != 3 || got[0] != r2 || got[1] != r3 || got[2] != r1 {
		t.Fatalf("wrong order: %v", got)
	}
}

func TestGreedy_ChooseDiscard_HighestCMC(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	c1 := newTestCardMinimal("A", []string{"creature"}, 1, nil)
	c2 := newTestCardMinimal("B", []string{"creature"}, 5, nil)
	c3 := newTestCardMinimal("C", []string{"creature"}, 3, nil)
	got := h.ChooseDiscard(gs, 0, []*gameengine.Card{c1, c2, c3}, 2)
	if len(got) != 2 {
		t.Fatalf("want 2 discards; got %d", len(got))
	}
	if got[0].DisplayName() != "B" || got[1].DisplayName() != "C" {
		t.Fatalf("wrong discard order: %s, %s", got[0].DisplayName(), got[1].DisplayName())
	}
}

func TestGreedy_ChooseMode_Zero(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	modes := []gameast.Effect{&gameast.Damage{}, &gameast.Draw{}}
	if got := h.ChooseMode(gs, 0, modes); got != 0 {
		t.Fatalf("GreedyHat.ChooseMode should always return 0; got %d", got)
	}
	// Empty case: -1.
	if got := h.ChooseMode(gs, 0, nil); got != -1 {
		t.Fatalf("empty modes should return -1; got %d", got)
	}
}

func TestGreedy_ShouldCastCommander_AlwaysTrue(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	if !h.ShouldCastCommander(gs, 0, "Sen Triplets", 0) {
		t.Fatal("GreedyHat should always cast a commander when affordable")
	}
	if !h.ShouldCastCommander(gs, 0, "Sen Triplets", 4) {
		t.Fatal("GreedyHat should cast even with tax")
	}
}

func TestGreedy_ShouldRedirectCommanderZone_AlwaysTrue(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	if !h.ShouldRedirectCommanderZone(gs, 0, &gameengine.Card{Name: "Cmdr"}, "graveyard") {
		t.Fatal("GreedyHat should always redirect commander to command zone (§903.9a)")
	}
}

func TestGreedy_ObserveEvent_NoPanic(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "turn_start"})
	// Also nil-safe.
	h.ObserveEvent(gs, 0, nil)
}

// TestGreedy_AssignBlockers_DeadliestFirst_ChumpIfLethal — the core
// block-assignment heuristic. Mirrors the pre-Phase-10 DeclareBlockers
// inline heuristic: deadliest attacker first, chump iff defender dies.
func TestGreedy_AssignBlockers_ChumpsWhenLethal(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	// Defender: 2 life. Single attacker: 5-power.
	gs.Seats[1].Life = 2
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Bruiser", []string{"creature"}, 3, nil), 5, 5)
	// Blocker: 1/1.
	blk := newTestPermanent(gs.Seats[1], newTestCardMinimal("Hedger", []string{"creature"}, 1, nil), 1, 1)

	got := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(got[atk]) != 1 || got[atk][0] != blk {
		t.Fatalf("expected chump-block on Bruiser with Hedger; got %v", got[atk])
	}
}

func TestGreedy_AssignBlockers_NoChumpIfSafe(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	// Defender at 20 life; small attacker; no reason to trade.
	gs.Seats[1].Life = 20
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Nibbler", []string{"creature"}, 1, nil), 1, 1)
	// Blocker is even smaller so can't profitably kill.
	_ = newTestPermanent(gs.Seats[1], newTestCardMinimal("Worm", []string{"creature"}, 1, nil), 0, 1)

	got := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(got[atk]) != 0 {
		t.Fatalf("should not chump when life comfortable; got %v", got[atk])
	}
}
