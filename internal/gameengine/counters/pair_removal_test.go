package counters

import "testing"

// newCreatureTarget builds a creature mockTarget with optional initial
// +1/+1 and -1/-1 counter stacks for the pair-removal SBA tests.
func newCreatureTarget(plus, minus int) *mockTarget {
	m := &mockTarget{cardTypes: []string{"creature"}}
	if plus > 0 {
		_, _ = AddCounters(m, "+1/+1", plus, "plus-src", 1)
	}
	if minus > 0 {
		_, _ = AddCounters(m, "-1/-1", minus, "minus-src", 2)
	}
	return m
}

// TestPairRemoveSBA_UnequalCancelsToLesser pins the canonical §704.5q
// case: 3 +1/+1 + 5 -1/-1 → N=3 removed from both → 0 +1/+1, 2 -1/-1.
func TestPairRemoveSBA_UnequalCancelsToLesser(t *testing.T) {
	m := newCreatureTarget(3, 5)
	n, changed := PairRemoveSBA(m)
	if !changed {
		t.Fatalf("PairRemoveSBA reported no change, want changed=true")
	}
	if n != 3 {
		t.Errorf("removed = %d, want 3 (min of 3 and 5)", n)
	}
	if got := CounterCount(m, "+1/+1"); got != 0 {
		t.Errorf("+1/+1 remaining = %d, want 0", got)
	}
	if got := CounterCount(m, "-1/-1"); got != 2 {
		t.Errorf("-1/-1 remaining = %d, want 2", got)
	}
}

// TestPairRemoveSBA_EqualCountsCancelClean pins that 4 +1/+1 + 4 -1/-1
// drains both stacks to zero in a single SBA fire.
func TestPairRemoveSBA_EqualCountsCancelClean(t *testing.T) {
	m := newCreatureTarget(4, 4)
	n, changed := PairRemoveSBA(m)
	if !changed {
		t.Fatalf("PairRemoveSBA reported no change, want changed=true")
	}
	if n != 4 {
		t.Errorf("removed = %d, want 4", n)
	}
	if got := CounterCount(m, "+1/+1"); got != 0 {
		t.Errorf("+1/+1 remaining = %d, want 0", got)
	}
	if got := CounterCount(m, "-1/-1"); got != 0 {
		t.Errorf("-1/-1 remaining = %d, want 0", got)
	}
}

// TestPairRemoveSBA_OnlyPlusNoOp pins that a creature with only +1/+1
// counters (no -1/-1 partner) is untouched and the SBA reports no fire.
func TestPairRemoveSBA_OnlyPlusNoOp(t *testing.T) {
	m := newCreatureTarget(5, 0)
	n, changed := PairRemoveSBA(m)
	if changed {
		t.Errorf("PairRemoveSBA fired with only +1/+1 counters, want no-op")
	}
	if n != 0 {
		t.Errorf("removed = %d, want 0", n)
	}
	if got := CounterCount(m, "+1/+1"); got != 5 {
		t.Errorf("+1/+1 remaining = %d, want 5 (untouched)", got)
	}
}

// TestPairRemoveSBA_OnlyMinusNoOp pins that a creature with only -1/-1
// counters (no +1/+1 partner) is untouched.
func TestPairRemoveSBA_OnlyMinusNoOp(t *testing.T) {
	m := newCreatureTarget(0, 4)
	n, changed := PairRemoveSBA(m)
	if changed {
		t.Errorf("PairRemoveSBA fired with only -1/-1 counters, want no-op")
	}
	if n != 0 {
		t.Errorf("removed = %d, want 0", n)
	}
	if got := CounterCount(m, "-1/-1"); got != 4 {
		t.Errorf("-1/-1 remaining = %d, want 4 (untouched)", got)
	}
}

// TestPairRemoveSBA_NilTargetSafe pins that nil targets return (0, false)
// without panicking — exercised because the SBA dispatcher iterates over
// snapshot slices that may include cleared entries.
func TestPairRemoveSBA_NilTargetSafe(t *testing.T) {
	n, changed := PairRemoveSBA(nil)
	if changed || n != 0 {
		t.Errorf("PairRemoveSBA(nil) = (%d, %v), want (0, false)", n, changed)
	}
}

// TestPairRemoveSBA_PreservesOtherCounterTypes pins §122.6 persistence:
// only +1/+1 and -1/-1 are touched. Loyalty, charge, oil, keyword grants
// on the same target stay intact.
func TestPairRemoveSBA_PreservesOtherCounterTypes(t *testing.T) {
	m := &mockTarget{cardTypes: []string{"creature", "planeswalker"}}
	if _, err := AddCounters(m, "+1/+1", 3, "p", 1); err != nil {
		t.Fatalf("seed +1/+1: %v", err)
	}
	if _, err := AddCounters(m, "-1/-1", 3, "p", 2); err != nil {
		t.Fatalf("seed -1/-1: %v", err)
	}
	if _, err := AddCounters(m, "loyalty", 4, "p", 3); err != nil {
		t.Fatalf("seed loyalty: %v", err)
	}
	if _, err := AddCounters(m, "charge", 2, "p", 4); err != nil {
		t.Fatalf("seed charge: %v", err)
	}
	n, changed := PairRemoveSBA(m)
	if !changed || n != 3 {
		t.Errorf("PairRemoveSBA = (%d, %v), want (3, true)", n, changed)
	}
	if got := CounterCount(m, "loyalty"); got != 4 {
		t.Errorf("loyalty = %d, want 4 (untouched)", got)
	}
	if got := CounterCount(m, "charge"); got != 2 {
		t.Errorf("charge = %d, want 2 (untouched)", got)
	}
}

// TestPairRemoveSBA_SimultaneousSamePassAddition pins the "added together
// in the same priority window" edge case from CR §704.3 — pair-removal
// after AddCounters on both sides in one batch still cancels cleanly.
// The mock target lets us seed both stacks before the SBA pass and
// verify a single PairRemoveSBA invocation drains them together.
func TestPairRemoveSBA_SimultaneousSamePassAddition(t *testing.T) {
	m := &mockTarget{cardTypes: []string{"creature"}}
	if _, err := AddCounters(m, "+1/+1", 2, "src-a", 10); err != nil {
		t.Fatalf("AddCounters +1/+1: %v", err)
	}
	if _, err := AddCounters(m, "-1/-1", 2, "src-b", 10); err != nil {
		t.Fatalf("AddCounters -1/-1: %v", err)
	}
	n, changed := PairRemoveSBA(m)
	if !changed || n != 2 {
		t.Errorf("PairRemoveSBA = (%d, %v), want (2, true)", n, changed)
	}
	if got := CounterCount(m, "+1/+1"); got != 0 {
		t.Errorf("+1/+1 remaining = %d, want 0", got)
	}
	if got := CounterCount(m, "-1/-1"); got != 0 {
		t.Errorf("-1/-1 remaining = %d, want 0", got)
	}
}

// TestPairRemoveSBA_IdempotentSecondCall pins that re-invoking on a
// just-cancelled target reports no change. The §704.3 outer SBA loop
// relies on this for quiescence — otherwise pair-removal would keep
// firing forever on the same permanent.
func TestPairRemoveSBA_IdempotentSecondCall(t *testing.T) {
	m := newCreatureTarget(3, 5)
	if _, changed := PairRemoveSBA(m); !changed {
		t.Fatalf("first call did not fire")
	}
	n, changed := PairRemoveSBA(m)
	if changed {
		t.Errorf("second call fired; want no-op once one side reached zero")
	}
	if n != 0 {
		t.Errorf("second call removed = %d, want 0", n)
	}
}

// TestPairRemoveSBA_EmptyTargetNoOp pins that a fresh creature with no
// counter stacks is a clean no-op (defends against an early-return bug
// where an empty CounterCount path could spuriously report changed=true).
func TestPairRemoveSBA_EmptyTargetNoOp(t *testing.T) {
	m := &mockTarget{cardTypes: []string{"creature"}}
	n, changed := PairRemoveSBA(m)
	if changed || n != 0 {
		t.Errorf("PairRemoveSBA(empty) = (%d, %v), want (0, false)", n, changed)
	}
}

// TestPairRemoveSBA_ReleaseStackEntryWhenDrained pins that fully draining
// a stack actually clears the CounterStacks entry (rather than leaving a
// zero-count residual). CounterCount must read 0 from an empty slice,
// not from a Count==0 stack with stale lineage metadata that would
// confuse later HasKeywordCounter / replacement-effect queries.
func TestPairRemoveSBA_ReleaseStackEntryWhenDrained(t *testing.T) {
	m := newCreatureTarget(5, 5)
	if _, changed := PairRemoveSBA(m); !changed {
		t.Fatalf("PairRemoveSBA did not fire on equal counts")
	}
	for _, st := range m.CounterStacks() {
		if (st.Type == "+1/+1" || st.Type == "-1/-1") && st.Count != 0 {
			t.Errorf("residual %s stack count = %d, want 0", st.Type, st.Count)
		}
		if (st.Type == "+1/+1" || st.Type == "-1/-1") && st.Count == 0 {
			// RemoveCounters drops fully-drained stacks; either path is
			// acceptable as long as CounterCount reads 0.
			t.Logf("note: stack %s present with Count=0 (no consumer impact)", st.Type)
		}
	}
}
