package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase-1D residue #3 regression pins. Five filter-arm scenarios from
// targets.go and tutor_resolve.go that Phase-1D flagged as unreachable
// because the audit-tool can't trace data flow from gameast.Filter.Extra
// (JSON-loaded) to the local `exLow` / `prefix` variables used in
// matchesPermanent / cardMatchesTutorFilter switches. Each test drives
// the filter with a real Extra value the parser COULD emit, exercising
// the flagged arm.

func newCreature(types ...string) *Permanent {
	return &Permanent{Card: &Card{Types: types}}
}

func newCard(types ...string) *Card {
	return &Card{Types: types}
}

// TestMatchesPermanent_NonEnchantmentExtra pins the `non-enchantment`
// / `nonenchantment` filter arms at targets.go:724. If the arm
// disappears, a parser-emitted "destroy target nonenchantment
// permanent" effect would silently match enchantments.
func TestMatchesPermanent_NonEnchantmentExtra(t *testing.T) {
	enchantment := newCreature("enchantment")
	creature := newCreature("creature")
	for _, ex := range []string{"nonenchantment", "non-enchantment"} {
		f := gameast.Filter{Base: "permanent", Extra: []string{ex}}
		if matchesPermanent(f, enchantment) {
			t.Errorf("filter %q must reject an enchantment", ex)
		}
		if !matchesPermanent(f, creature) {
			t.Errorf("filter %q must accept a non-enchantment creature", ex)
		}
	}
}

// TestMatchesPermanent_NonLegendaryExtra pins both spellings of the
// non-legendary supertype filter.
func TestMatchesPermanent_NonLegendaryExtra(t *testing.T) {
	legendary := newCreature("creature", "legendary")
	plain := newCreature("creature")
	for _, ex := range []string{"nonlegendary", "non-legendary"} {
		f := gameast.Filter{Base: "creature", Extra: []string{ex}}
		if matchesPermanent(f, legendary) {
			t.Errorf("filter %q must reject a legendary creature", ex)
		}
		if !matchesPermanent(f, plain) {
			t.Errorf("filter %q must accept a non-legendary creature", ex)
		}
	}
}

// TestMatchesPermanent_NonBasicExtra pins the non-basic supertype
// filter (Phyrexian Tower's land-sac-cost analog).
func TestMatchesPermanent_NonBasicExtra(t *testing.T) {
	basicLand := newCreature("land", "basic")
	nonbasic := newCreature("land")
	for _, ex := range []string{"nonbasic", "non-basic"} {
		f := gameast.Filter{Base: "land", Extra: []string{ex}}
		if matchesPermanent(f, basicLand) {
			t.Errorf("filter %q must reject a basic land", ex)
		}
		if !matchesPermanent(f, nonbasic) {
			t.Errorf("filter %q must accept a non-basic land", ex)
		}
	}
}

// TestCardMatchesTutorFilter_NonTokenLibrarySemantics pins the
// tutor_resolve.go non-token arm. Library cards are never tokens, so
// the filter is a no-op for tutors — but the arm needs to EXIST so
// the parser-emitted "search your library for a nontoken creature"
// shape doesn't fall through to a no-match.
func TestCardMatchesTutorFilter_NonTokenLibrarySemantics(t *testing.T) {
	creature := newCard("creature")
	for _, ex := range []string{"nontoken", "non-token"} {
		f := gameast.Filter{Base: "creature", Extra: []string{ex}}
		if !cardMatchesTutorFilter(creature, f) {
			t.Errorf("tutor filter %q on library creature: should pass (library cards aren't tokens)", ex)
		}
	}
}

// TestCardMatchesTutorFilter_HistoricArm pins the `historic` arm at
// tutor_resolve.go:497 — historic = legendary OR artifact OR saga.
// The arm has no spelling variants, but it's only reachable when the
// parser emits "historic" as a Filter.Extra value (e.g. "search your
// library for a historic card"). Test each of the three qualifying
// types.
func TestCardMatchesTutorFilter_HistoricArm(t *testing.T) {
	cases := []struct {
		name  string
		types []string
		want  bool
	}{
		{"legendary creature", []string{"creature", "legendary"}, true},
		{"artifact", []string{"artifact"}, true},
		{"saga", []string{"enchantment", "saga"}, true},
		{"plain creature", []string{"creature"}, false},
		{"non-saga enchantment", []string{"enchantment"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := gameast.Filter{Base: "card", Extra: []string{"historic"}}
			got := cardMatchesTutorFilter(newCard(tc.types...), f)
			if got != tc.want {
				t.Errorf("historic filter on %v: got %v want %v", tc.types, got, tc.want)
			}
		})
	}
}
