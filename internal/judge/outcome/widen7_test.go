package outcome

// Phase-7 unit suite — each new class exercised end-to-end through
// RunEffect against the live engine, plus skip-path pins.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestWiden7_ExileLibraryTop(t *testing.T) {
	// Pins the engine fix end-to-end: library −N, exile +N.
	mustPass(t, "abbot-exile-top", &gameast.Exile{
		Target: gameast.Filter{Base: "library_top", Quantifier: "one",
			Count: gameast.NumInt(1), Targeted: true},
	})
	mustPass(t, "exile-top-two", &gameast.Exile{
		Target: gameast.Filter{Base: "library_top", Quantifier: "one",
			Count: gameast.NumInt(2), Targeted: true},
	})
}

func TestWiden7_DamageWidened(t *testing.T) {
	mustPass(t, "each-opponent-burn", &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "opponent", Quantifier: "each"},
	})
	mustPass(t, "that-creature-burn", &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "that", Quantifier: "one"},
	})
}

func TestWiden7_SelfNamedCounters(t *testing.T) {
	mustPass(t, "counter-on-tilde", &gameast.CounterMod{
		Op: "put", Count: *gameast.NumInt(1), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "thing", Quantifier: "one"},
	})
}

func TestWiden7_ContextualDiscardDisjunction(t *testing.T) {
	mustPass(t, "that-player-discards", &gameast.Discard{
		Count:  *gameast.NumInt(1),
		Target: gameast.Filter{Base: "that_player", Quantifier: "one"},
	})
}

func TestWiden7_GainControl(t *testing.T) {
	mustPass(t, "agent-of-treachery", &gameast.GainControl{
		Target:   gameast.Filter{Base: "permanent", Quantifier: "one", Targeted: true},
		Duration: "permanent",
	})
	mustPass(t, "control-creature", &gameast.GainControl{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true},
	})
	// Mass theft: out of scope, never guessed.
	mustSkip(t, "insurrection", &gameast.GainControl{
		Target: gameast.Filter{Base: "creature", Quantifier: "all"},
	})
}

func TestWiden7_ZeroDeltaRegistrations(t *testing.T) {
	mustPass(t, "replacement-node", &gameast.Replacement{})
	mustPass(t, "transform-self", &gameast.ModificationEffect{
		ModKind: "transform_self", Args: []interface{}{},
	})
	mustPass(t, "roll-table-row", &gameast.ModificationEffect{
		ModKind: "roll_table_row", Args: []interface{}{},
	})
}

func TestWiden7_LevelMarker(t *testing.T) {
	// Class level-up: the engine sets the source's "level" counter to
	// N — delta +N on the fresh scaffold (the zero-delta guess was the
	// round-1 66-finding miss; engine was correct).
	// Corpus emission shape: args carry a plain JSON number (float64
	// after astload decode).
	mustPass(t, "class-level-2", &gameast.ModificationEffect{
		ModKind: "level_marker", Args: []interface{}{float64(2)},
	})
	mustSkip(t, "class-level-unparseable", &gameast.ModificationEffect{
		ModKind: "level_marker", Args: []interface{}{"x"},
	})
}

func TestWiden7_CounterSpellTails(t *testing.T) {
	mustPass(t, "counter-abilities-fizzle", &gameast.CounterSpell{
		Target: gameast.Filter{Base: "abilities", Quantifier: "one", Targeted: true},
	})
	mustPass(t, "counter-noncreature-spell", &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell", Quantifier: "one", Extra: []string{"non-creature"}},
	})
	mustPass(t, "annul-fizzles-on-instant", &gameast.CounterSpell{
		Target: gameast.Filter{Base: "artifact", Quantifier: "one"},
	})
}
