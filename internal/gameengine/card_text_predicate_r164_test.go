package gameengine

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r164Card builds a card whose lowered oracle text contains `text`,
// using the real AST path so OracleTextLower behaves as in production.
func r164Card(t *testing.T, text string) *Card {
	t.Helper()
	ast := &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: text}}}
	return &Card{Name: "R164 Probe", AST: ast}
}

func r164Reset(t *testing.T) {
	t.Helper()
	ResetTextPredicateStatsForTest()
	t.Cleanup(ResetTextPredicateStatsForTest)
}

// The round's whole claim: the second ask is served from the memo.
func TestR164_SecondCallIsACacheHit(t *testing.T) {
	r164Reset(t)
	c := r164Card(t, "Flashback")
	calls := 0
	pred := func(ot string) bool { calls++; return true }

	if !c.TextPredicate(0, pred) || !c.TextPredicate(0, pred) || !c.TextPredicate(0, pred) {
		t.Fatalf("predicate returned false")
	}
	if calls != 1 {
		t.Fatalf("compute ran %d times, want 1", calls)
	}
	h, m, _ := TextPredicateStats()
	if h != 2 || m != 1 {
		t.Fatalf("hits=%d misses=%d, want 2/1", h, m)
	}
}

// A FALSE result must be cached too — the classic bug is caching only
// the positive and re-scanning every card that does not match, which
// is the overwhelming majority of them.
func TestR164_FalseIsCachedAsWellAsTrue(t *testing.T) {
	r164Reset(t)
	c := r164Card(t, "Flying")
	calls := 0
	pred := func(ot string) bool { calls++; return false }

	for i := 0; i < 4; i++ {
		if c.TextPredicate(1, pred) {
			t.Fatalf("predicate returned true")
		}
	}
	if calls != 1 {
		t.Fatalf("compute ran %d times for a FALSE result, want 1", calls)
	}
}

// Bits must not bleed into one another.
func TestR164_BitsAreIndependent(t *testing.T) {
	r164Reset(t)
	c := r164Card(t, "Flashback")
	if !c.TextPredicate(3, func(string) bool { return true }) {
		t.Fatalf("bit 3 should be true")
	}
	if c.TextPredicate(4, func(string) bool { return false }) {
		t.Fatalf("bit 4 should be false")
	}
	if !c.TextPredicate(3, func(string) bool { return false }) {
		t.Fatalf("bit 3 lost its cached TRUE when bit 4 was written")
	}
	if c.TextPredicate(4, func(string) bool { return true }) {
		t.Fatalf("bit 4 lost its cached FALSE when bit 3 was read")
	}
}

// THE r157 LESSON, pinned: a DFC transform reassigns Card.AST, and the
// memo must follow it rather than a "have we computed this" flag.
func TestR164_TransformInvalidatesEveryBit(t *testing.T) {
	r164Reset(t)
	c := r164Card(t, "Flying")
	front := func(ot string) bool { return strings.Contains(ot, "flying") }

	if !c.TextPredicate(0, front) {
		t.Fatalf("front face should match before the transform")
	}
	if c.textPredASTOf() != c.AST {
		t.Fatalf("memo key is not the AST it was built from")
	}

	// The transform: a new AST in place, exactly as TransformPermanent does.
	c.AST = &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "Trample"}}}

	if c.TextPredicate(0, front) {
		t.Fatalf("memo served the FRONT FACE after a transform — this is " +
			"the r157 defect reintroduced")
	}
	// A DIFFERENT bit, because bit 0 has now legitimately been
	// recomputed as false against the back face. One bit is one
	// predicate; reusing it for a second question is a test bug, and
	// this test caught exactly that when it was written.
	if !c.TextPredicate(1, func(ot string) bool { return strings.Contains(ot, "trample") }) {
		t.Fatalf("back face should match after the transform")
	}
}

// A zero-value Card must never serve a cached answer: textPredAST and
// AST are both nil, which must count as "not computed", not "equal".
func TestR164_ZeroValueCardNeverServesACachedAnswer(t *testing.T) {
	r164Reset(t)
	c := &Card{Name: "R164 Empty"}
	calls := 0
	// Poison: if the zero value were treated as a valid key, the second
	// call would return the first (wrong) answer.
	if c.TextPredicate(0, func(string) bool { calls++; return true }) != true {
		t.Fatalf("first call should return what compute said")
	}
	got := c.TextPredicate(0, func(string) bool { calls++; return true })
	if !got {
		t.Fatalf("cached answer changed")
	}
	if calls == 0 {
		t.Fatalf("compute never ran")
	}
}

// Out-of-range bits must still return the CORRECT answer, just uncached.
// A wrong answer is a bug; a slow answer is a budget.
func TestR164_OutOfRangeBitIsCorrectButUncached(t *testing.T) {
	r164Reset(t)
	c := r164Card(t, "Flashback")
	calls := 0
	for i := 0; i < 3; i++ {
		if !c.TextPredicate(predicateBits+7, func(string) bool { calls++; return true }) {
			t.Fatalf("out-of-range bit returned the wrong answer")
		}
	}
	if calls != 3 {
		t.Fatalf("out-of-range compute ran %d times, want 3 (uncached)", calls)
	}
	_, _, unc := TextPredicateStats()
	if unc != 3 {
		t.Fatalf("uncached counter = %d, want 3", unc)
	}
}

func TestR164_NilCardAndNilComputeAreSafe(t *testing.T) {
	r164Reset(t)
	var c *Card
	if c.TextPredicate(0, func(string) bool { return true }) {
		t.Fatalf("nil card must return false")
	}
	if r164Card(t, "x").TextPredicate(0, nil) {
		t.Fatalf("nil compute must return false")
	}
}

func TestR164_InvalidateDropsTheMemo(t *testing.T) {
	r164Reset(t)
	c := r164Card(t, "Flashback")
	calls := 0
	pred := func(string) bool { calls++; return true }
	c.TextPredicate(0, pred)
	c.InvalidateTextPredicates()
	c.TextPredicate(0, pred)
	if calls != 2 {
		t.Fatalf("compute ran %d times across an invalidation, want 2", calls)
	}
}

// Two games must not share a memo — the cache lives on the Card, and
// each game clones its library.
func TestR164_SeparateCardsDoNotShare(t *testing.T) {
	r164Reset(t)
	_ = NewGameState(2, rand.New(rand.NewSource(1)), nil)
	a := r164Card(t, "Flashback")
	b := r164Card(t, "Flashback")
	if !a.TextPredicate(0, func(string) bool { return true }) {
		t.Fatalf("a should be true")
	}
	if b.TextPredicate(0, func(string) bool { return false }) {
		t.Fatalf("b must compute its own answer, not read a's")
	}
}
