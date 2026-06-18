package counters

import "testing"

// stackTarget is a minimal Target carrying a fixed set of counter stacks.
type stackTarget struct{ stacks []CounterStack }

func (t *stackTarget) HasCardType(string) bool          { return true }
func (t *stackTarget) HasSubtype(string) bool           { return false }
func (t *stackTarget) CounterStacks() []CounterStack    { return t.stacks }
func (t *stackTarget) SetCounterStacks(s []CounterStack) { t.stacks = s }

// TestGrantsKeyword_DivergentName pins CR §122.1c for the one registered
// keyword counter whose TYPE NAME differs from the keyword it grants: a
// phyresis counter grants infect (§702.91). The pre-r63 name-only lookup
// asked the registry for a counter literally named "infect" and found
// none, silently dropping the grant.
func TestGrantsKeyword_DivergentName(t *testing.T) {
	if !GrantsKeyword("phyresis", "infect") {
		t.Error("a phyresis counter must grant infect (CR §122.1c / §702.91)")
	}
	// It must NOT grant infect under the wrong query, nor grant unrelated
	// keywords.
	if GrantsKeyword("phyresis", "flying") {
		t.Error("a phyresis counter must not grant flying")
	}
	if GrantsKeyword("infect", "infect") {
		t.Error("there is no counter type named \"infect\"; lookup must fail")
	}
}

// TestGrantsKeyword_DirectName pins the common case where the counter's
// own canonical name IS the keyword (flying counter → flying, haste
// counter → haste) — unchanged by the divergent-grant fix.
func TestGrantsKeyword_DirectName(t *testing.T) {
	for _, kw := range []string{"flying", "haste", "deathtouch", "lifelink", "first strike", "double strike"} {
		if !GrantsKeyword(kw, kw) {
			t.Errorf("a %q counter must grant %q", kw, kw)
		}
	}
	if GrantsKeyword("flying", "haste") {
		t.Error("a flying counter must not grant haste")
	}
	if GrantsKeyword("", "infect") || GrantsKeyword("phyresis", "") {
		t.Error("empty inputs must return false")
	}
}

// TestHasKeywordCounter_DivergentName verifies the divergent grant is
// observable through the Target-facing HasKeywordCounter (the CounterStacks
// surface), not only the registry helper.
func TestHasKeywordCounter_DivergentName(t *testing.T) {
	tgt := &stackTarget{stacks: []CounterStack{{Type: "phyresis", Count: 1}}}
	if !HasKeywordCounter(tgt, "infect") {
		t.Error("a phyresis counter stack must satisfy HasKeywordCounter(\"infect\")")
	}
	// Zero-count stacks confer nothing.
	empty := &stackTarget{stacks: []CounterStack{{Type: "phyresis", Count: 0}}}
	if HasKeywordCounter(empty, "infect") {
		t.Error("a 0-count phyresis stack must not grant infect")
	}
}
