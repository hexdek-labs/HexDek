package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// lifegain_doubler_coverage_r63_test.go — audit of the lifegain engine's
// doubling + trigger cardinality (josh Oloro lifegain deck).
//
// The central bug: the CR §614 would_gain_life replacement chain (lifegain
// doublers Boon Reflection / Rhox Faithmender / Alhammarret's Archive, additive
// deltas, can't-gain, opponent-gain→loss) was consulted ONLY in the AST
// resolveGainLife path. The 120+ bare GainLife() callers — combat lifelink,
// every ETB-lifegain trigger (Soul Warden …), drains, modal effects — bypassed
// it, so a doubler covered almost none of your actual lifegain sources. The fix
// routes EVERY GainLife through the chain, so doublers cover all sources and
// the life_gained payoff triggers see the doubled amount.

// A probe permanent that records the amount of every life_gained trigger it
// observes — stands in for Well of Lost Dreams / Archangel of Thune to prove
// (1) event cardinality and (2) that the payoff sees the post-doubler amount.
var lifegainProbeAmounts []int

func init() {
	registerLifegainProbe(Global())
	AddResetHook(registerLifegainProbe)
}

func registerLifegainProbe(r *Registry) {
	r.OnTrigger("Lifegain Probe", "life_gained",
		func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			amt, _ := ctx["amount"].(int)
			lifegainProbeAmounts = append(lifegainProbeAmounts, amt)
		})
}

func addProbe(gs *gameengine.GameState, seat int) {
	addPerm(gs, seat, "Lifegain Probe", "enchantment")
}

// (2) coverage: a bare GainLife (lifelink/ETB/drain fast-path) is now doubled.
func TestLifegain_DoublerCoversBareGainLife(t *testing.T) {
	gs := newGame(t, 2)
	boon := addPerm(gs, 0, "Boon Reflection", "enchantment")
	gameengine.RegisterBoonReflection(gs, boon)

	before := gs.Seats[0].Life
	gained := gameengine.GainLife(gs, 0, 3, "lifelink") // bare call, not the AST path
	if gained != 6 {
		t.Errorf("Boon Reflection must double a bare GainLife 3→6, got %d", gained)
	}
	if d := gs.Seats[0].Life - before; d != 6 {
		t.Errorf("life delta want 6, got %d", d)
	}
}

// (2) doublers MULTIPLY across sources.
func TestLifegain_DoublersMultiply(t *testing.T) {
	gs := newGame(t, 2)
	boon := addPerm(gs, 0, "Boon Reflection", "enchantment")
	rhox := addPerm(gs, 0, "Rhox Faithmender", "creature")
	gameengine.RegisterBoonReflection(gs, boon)
	gameengine.RegisterRhoxFaithmender(gs, rhox)
	if gained := gameengine.GainLife(gs, 0, 3, "drain"); gained != 12 {
		t.Errorf("Boon × Rhox must multiply 3→12, got %d", gained)
	}
}

// (1)+(2): the life_gained payoff trigger fires ONCE per event and sees the
// DOUBLED amount (double THEN trigger reads it).
func TestLifegain_TriggerOncePerEvent_SeesDoubled(t *testing.T) {
	gs := newGame(t, 2)
	addProbe(gs, 0)
	boon := addPerm(gs, 0, "Boon Reflection", "enchantment")
	gameengine.RegisterBoonReflection(gs, boon)

	lifegainProbeAmounts = nil
	gameengine.GainLife(gs, 0, 3, "single event of 3") // one event, amount 3

	if len(lifegainProbeAmounts) != 1 {
		t.Fatalf("gaining 3 in ONE event must fire the payoff exactly once, got %d fires %v",
			len(lifegainProbeAmounts), lifegainProbeAmounts)
	}
	if lifegainProbeAmounts[0] != 6 {
		t.Errorf("payoff must see the DOUBLED amount 6, got %d", lifegainProbeAmounts[0])
	}
}

// (1): several SEPARATE gains in a turn fire the payoff several times.
func TestLifegain_SeparateEventsFireSeparately(t *testing.T) {
	gs := newGame(t, 2)
	addProbe(gs, 0)
	lifegainProbeAmounts = nil
	gameengine.GainLife(gs, 0, 1, "a")
	gameengine.GainLife(gs, 0, 2, "b")
	gameengine.GainLife(gs, 0, 5, "c")
	if len(lifegainProbeAmounts) != 3 {
		t.Fatalf("3 separate gains must fire 3 payoffs, got %d %v", len(lifegainProbeAmounts), lifegainProbeAmounts)
	}
	// No doubler → amounts pass through unchanged (cardinality independent of amount).
	want := []int{1, 2, 5}
	for i, w := range want {
		if lifegainProbeAmounts[i] != w {
			t.Errorf("event %d amount want %d, got %d", i, w, lifegainProbeAmounts[i])
		}
	}
}

// Regression: the AST resolveGainLife path must double EXACTLY once (not twice)
// now that the chain moved into GainLife. Driven through the bare API the AST
// path uses; a double-double would read 12 here.
func TestLifegain_NoDoubleDouble(t *testing.T) {
	gs := newGame(t, 2)
	boon := addPerm(gs, 0, "Boon Reflection", "enchantment")
	gameengine.RegisterBoonReflection(gs, boon)
	if gained := gameengine.GainLife(gs, 0, 4, "gain 4 spell"); gained != 8 {
		t.Errorf("single doubler must apply exactly once 4→8, got %d", gained)
	}
}
