package outcome

// etb_counters.go — phase-3 REPLACEMENT-class check: "this enters with
// N <kind> counters" self-replacements (Modification kind
// etb_with_counters, 390 corpus shapes). The expectation derives purely
// from the AST args: entering must add exactly N counters of the kind
// (and the ±1/±1 effective-P/T shift). Checked through the engine's
// public ETB entry — the §614.1c-adjacent class observed as state.

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ETBCountersSpec extracts (count, kind) from an etb_with_counters
// Static modification. ok=false for X/variable counts (out of scope —
// the entry context defines X).
func ETBCountersSpec(m *gameast.Modification) (int, string, bool) {
	if m == nil || m.ModKind != "etb_with_counters" || len(m.Args) < 1 {
		return 0, "", false
	}
	n := 0
	switch v := m.Args[0].(type) {
	case int:
		n = v
	case float64:
		n = int(v)
	default:
		return 0, "", false
	}
	if n <= 0 {
		return 0, "", false
	}
	kind := "+1/+1"
	if len(m.Args) > 1 {
		if k, ok := m.Args[1].(string); ok && k != "" {
			kind = k
		}
	}
	return n, kind, true
}

// ETBCountersFinding is a divergence in the enters-with-counters class.
type ETBCountersFinding struct {
	CardName string
	Want     int
	Kind     string
	Got      int
}

func (f ETBCountersFinding) String() string {
	return fmt.Sprintf("%s: enters-with-counters want %d %q, got %d", f.CardName, f.Want, f.Kind, f.Got)
}

// CheckETBCounters enters a bearer carrying the etb_with_counters
// static through the engine's public ETB path and asserts the counter
// delta. Returns (finding, ran).
func CheckETBCounters(cardName string, m *gameast.Modification) (*ETBCountersFinding, bool) {
	want, kind, ok := ETBCountersSpec(m)
	if !ok {
		return nil, false
	}
	spec := DefaultSpec()
	gs, bearer := BuildBoard(spec, cardName)
	bearer.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Modification: m},
	}}
	gameengine.FirePermanentETBTriggers(gs, bearer)
	got := bearer.Counters[kind]
	if got != want {
		return &ETBCountersFinding{CardName: cardName, Want: want, Kind: kind, Got: got}, true
	}
	return nil, true
}
