package hat

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r174 before/after benchmarks.
//
// Each pair keeps the OLD expression verbatim alongside the shipped
// one so both arms live in the same binary and `-count N` interleaves
// them across time — the box is under contention and separate runs
// would sample different load. -benchmem alloc counts are the
// load-independent signal; ns/op is advisory.

// --- old implementations, copied verbatim from HEAD -------------------

func r174OldTypeLineContains(card *gameengine.Card, substr string) bool {
	if card == nil {
		return false
	}
	ss := strings.ToLower(substr)
	for _, t := range card.Types {
		if strings.Contains(strings.ToLower(t), ss) {
			return true
		}
	}
	return false
}

func r174OldFastManaExtraYield(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	total := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.PhasedOut {
			continue
		}
		if extra, ok := fastManaExtraYieldByName[strings.ToLower(p.Card.DisplayName())]; ok {
			total += extra
		}
	}
	return total
}

func r174OldRitualNetMana(c *gameengine.Card) int {
	if c == nil {
		return 0
	}
	return ritualNetByName[strings.ToLower(c.DisplayName())]
}

// --- fixtures ---------------------------------------------------------

// r174BenchSeat: a mid-game board of 16 permanents, two of which are
// curated fast mana. Representative of what the evaluator walks.
func r174BenchSeat() *gameengine.Seat {
	seat := &gameengine.Seat{Idx: 0}
	names := []string{
		"Plains", "Island", "Swamp", "Command Tower", "Sol Ring",
		"Arcane Signet", "Birds of Paradise", "Llanowar Elves",
		"Rhystic Study", "Smothering Tithe", "Mana Vault", "Cultivate",
		"Lightning Greaves", "Swiftfoot Boots", "Esper Sentinel", "Forest",
	}
	for _, n := range names {
		c := &gameengine.Card{Name: n, Types: []string{"artifact"}}
		seat.Battlefield = append(seat.Battlefield, &gameengine.Permanent{Card: c, Controller: 0})
	}
	return seat
}

func r174BenchHand() []*gameengine.Card {
	names := []string{
		"Demonic Tutor", "Worldly Tutor", "Lightning Bolt", "Dark Ritual",
		"Counterspell", "Sol Ring", "Forest",
	}
	out := make([]*gameengine.Card, 0, len(names))
	for _, n := range names {
		out = append(out, &gameengine.Card{Name: n, Types: []string{"instant"}})
	}
	return out
}

var r174BenchTypeCard = &gameengine.Card{
	Name:  "Sheoldred, the Apocalypse",
	Types: []string{"legendary", "creature", "phyrexian", "praetor"},
}

var r174BenchSink int
var r174BenchSinkB bool

// --- fastManaExtraYield ----------------------------------------------

func BenchmarkR174_FastManaExtraYield_Old(b *testing.B) {
	seat := r174BenchSeat()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r174BenchSink += r174OldFastManaExtraYield(seat)
	}
}

func BenchmarkR174_FastManaExtraYield_New(b *testing.B) {
	seat := r174BenchSeat()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r174BenchSink += fastManaExtraYield(seat)
	}
}

// --- ritualNetMana ----------------------------------------------------

func BenchmarkR174_RitualNetMana_Old(b *testing.B) {
	hand := r174BenchHand()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range hand {
			r174BenchSink += r174OldRitualNetMana(c)
		}
	}
}

func BenchmarkR174_RitualNetMana_New(b *testing.B) {
	hand := r174BenchHand()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range hand {
			r174BenchSink += ritualNetMana(c)
		}
	}
}

// --- typeLineContains -------------------------------------------------

func BenchmarkR174_TypeLineContains_Old(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r174BenchSinkB = r174OldTypeLineContains(r174BenchTypeCard, "creature")
		r174BenchSinkB = r174OldTypeLineContains(r174BenchTypeCard, "land")
	}
}

func BenchmarkR174_TypeLineContains_New(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r174BenchSinkB = typeLineContains(r174BenchTypeCard, "creature")
		r174BenchSinkB = typeLineContains(r174BenchTypeCard, "land")
	}
}

// --- handQualityBonus tutor probe -------------------------------------

func BenchmarkR174_TutorNameProbe_Old(b *testing.B) {
	hand := r174BenchHand()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range hand {
			r174BenchSinkB = strings.Contains(strings.ToLower(c.DisplayName()), "tutor")
		}
	}
}

func BenchmarkR174_TutorNameProbe_New(b *testing.B) {
	hand := r174BenchHand()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, c := range hand {
			r174BenchSinkB = foldContainsASCII(c.DisplayName(), "tutor")
		}
	}
}

// --- cardHeuristic type-line lower ------------------------------------

func BenchmarkR174_TypeLineLower_Old(b *testing.B) {
	c := &gameengine.Card{Name: "Sheoldred", TypeLine: "Legendary Creature — Phyrexian Praetor"}
	var sink string
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = strings.ToLower(c.TypeLine)
	}
	r174BenchSink += len(sink)
}

func BenchmarkR174_TypeLineLower_New(b *testing.B) {
	c := &gameengine.Card{Name: "Sheoldred", TypeLine: "Legendary Creature — Phyrexian Praetor"}
	var sink string
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink = gameengine.TypeLineLower(c)
	}
	r174BenchSink += len(sink)
}

// --- whole-function: handQualityBonus ---------------------------------

func BenchmarkR174_HandQualityBonus_New(b *testing.B) {
	seat := &gameengine.Seat{Idx: 0, Hand: r174BenchHand()}
	var sink float64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink += handQualityBonus(seat)
	}
	r174BenchSink += int(sink)
}
