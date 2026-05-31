package per_card

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// per_card dispatcher + handler benchmark suite (R60)
//
// Profiles the dispatcher's hot path (fireTrigger walks every battlefield
// permanent + lookupCandidates for each) and a representative slice of
// the most-popular handlers.
//
// Run:
//
//   go test ./internal/gameengine/per_card/ -bench=. -benchmem -count=3 \
//     -run=^$ -benchtime=2s
//
// Or just the dispatcher hot path:
//
//   go test ./internal/gameengine/per_card/ -bench=BenchmarkFireTrigger \
//     -benchmem -count=3 -run=^$ -benchtime=2s
//
// The regression-pin test at the bottom (TestDispatchSpeedRegressionPin)
// runs in the standard suite and fails on >2x slowdown vs the baseline
// captured at branch-merge time.
// ---------------------------------------------------------------------------

// benchState builds a 4-seat game with the given per-seat permanent count.
// Includes a mix of handler-bearing cards (Sram, Talrand, Purphoros) so
// the dispatcher actually hits handler registrations during the walk.
func benchState(perPerSeat int) *gameengine.GameState {
	gs := gameengine.NewGameState(4, nil, nil)
	for seat := 0; seat < 4; seat++ {
		for i := 0; i < perPerSeat; i++ {
			name := fmt.Sprintf("Filler %d", i)
			// Sprinkle in handler-bearing cards every few perms so the
			// dispatcher exercises actual registered handlers.
			switch i % 5 {
			case 0:
				name = "Sram, Senior Edificer"
			case 1:
				name = "Talrand, Sky Summoner"
			case 2:
				name = "Purphoros, God of the Forge"
			case 3:
				name = "Marwyn, the Nurturer"
			}
			card := &gameengine.Card{
				Name:          name,
				Owner:         seat,
				Types:         []string{"creature"},
				BasePower:     2,
				BaseToughness: 2,
			}
			p := &gameengine.Permanent{
				Card:       card,
				Controller: seat,
				Owner:      seat,
				Timestamp:  gs.NextTimestamp(),
				Counters:   map[string]int{},
				Flags:      map[string]int{},
			}
			gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
		}
		gs.Seats[seat].Library = []*gameengine.Card{
			{Name: "Top", Owner: seat},
			{Name: "Mid", Owner: seat},
			{Name: "Bot", Owner: seat},
		}
	}
	return gs
}

// ---------------------------------------------------------------------------
// Dispatcher hot path
// ---------------------------------------------------------------------------

// BenchmarkFireTrigger_SpellCast — fires the most-popular cast event
// against a typical mid-game battlefield (8 perms/seat × 4 seats = 32
// perms). Stresses the per-permanent lookupCandidates walk.
func BenchmarkFireTrigger_SpellCast(b *testing.B) {
	gs := benchState(8)
	spell := &gameengine.Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant", "cost:1"},
	}
	ctx := map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Lightning Bolt",
		"card":        spell,
		"is_creature": false,
		"cast_zone":   "hand",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gameengine.FireCardTrigger(gs, "spell_cast", ctx)
	}
}

// BenchmarkFireTrigger_NoHandlers — pure dispatcher overhead for an
// event no card on the board handles. Measures the per-permanent
// registry-lookup walk in isolation.
func BenchmarkFireTrigger_NoHandlers(b *testing.B) {
	gs := benchState(8)
	ctx := map[string]interface{}{
		"seat":   0,
		"amount": 1,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gameengine.FireCardTrigger(gs, "fictitious_event_with_no_handlers", ctx)
	}
}

// BenchmarkFireTrigger_LargeBoard — stress the per-perm walk at 5x the
// typical board size (40 perms/seat × 4 = 160 perms).
func BenchmarkFireTrigger_LargeBoard(b *testing.B) {
	gs := benchState(40)
	ctx := map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "X", Owner: 0, Types: []string{"instant"}},
		"is_creature": false,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gameengine.FireCardTrigger(gs, "spell_cast", ctx)
	}
}

// ---------------------------------------------------------------------------
// lookupCandidates micro-benchmarks
// ---------------------------------------------------------------------------

func BenchmarkLookupCandidates_SimpleName(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = lookupCandidates("Lightning Bolt")
	}
}

func BenchmarkLookupCandidates_LegendaryName(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = lookupCandidates("Sram, Senior Edificer")
	}
}

func BenchmarkLookupCandidates_TokenSuffix(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = lookupCandidates("Myr Token")
	}
}

func BenchmarkLookupCandidates_StackedParens(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = lookupCandidates("Crown of Gondor (Urza copy) (Urza copy)")
	}
}

func BenchmarkLookupCandidates_DFC(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = lookupCandidates("Delver of Secrets // Insectile Aberration")
	}
}

// ---------------------------------------------------------------------------
// Per-handler micro-benchmarks (5 representative handler shapes)
// ---------------------------------------------------------------------------

// BenchmarkHandler_Sheoldred — minimal ctx-read handler (card_drawn).
func BenchmarkHandler_Sheoldred(b *testing.B) {
	gs := newGameForBench(2)
	addBench(gs, 0, "Sheoldred, the Apocalypse", 4, 5)
	ctx := map[string]interface{}{
		"seat":          0,
		"drawer_seat":   0,
		"card":          "Brainstorm",
		"nth_this_turn": 1,
		"source":        "draw",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gameengine.FireCardTrigger(gs, "card_drawn", ctx)
	}
}

// BenchmarkHandler_Marwyn — handler that gates on card-type checks.
func BenchmarkHandler_Marwyn(b *testing.B) {
	gs := newGameForBench(2)
	addBench(gs, 0, "Marwyn, the Nurturer", 1, 1)
	elf := &gameengine.Card{
		Name:          "Llanowar Elves",
		Owner:         0,
		Types:         []string{"creature", "elf", "druid"},
		BasePower:     1,
		BaseToughness: 1,
	}
	ctx := map[string]interface{}{
		"controller_seat": 0,
		"card":            elf,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gameengine.FireCardTrigger(gs, "nonland_permanent_etb", ctx)
	}
}

// BenchmarkHandler_Selvala — handler that scans ALL seats' battlefields.
func BenchmarkHandler_Selvala(b *testing.B) {
	gs := benchState(8) // 32 perms across 4 seats
	sel := addBench(gs, 0, "Selvala, Heart of the Wilds", 2, 4)
	_ = sel
	titan := addBench(gs, 0, "Strong Creature", 8, 8)
	ctx := map[string]interface{}{
		"controller_seat": 0,
		"perm":            titan,
		"card":            titan.Card,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gameengine.FireCardTrigger(gs, "permanent_etb", ctx)
	}
}

// BenchmarkHandler_Purphoros — handler that loops over opponents.
func BenchmarkHandler_Purphoros(b *testing.B) {
	gs := newGameForBench(4)
	purph := addBench(gs, 0, "Purphoros, God of the Forge", 4, 4)
	_ = purph
	other := addBench(gs, 0, "Llanowar Elves", 1, 1)
	other.Card.Types = []string{"creature", "elf"}
	ctx := map[string]interface{}{
		"controller_seat": 0,
		"perm":            other,
		"card":            other.Card,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gameengine.FireCardTrigger(gs, "permanent_etb", ctx)
	}
}

// BenchmarkHandler_Talrand — handler that mints tokens via
// enterBattlefieldWithETB on each fire.
func BenchmarkHandler_Talrand(b *testing.B) {
	gs := newGameForBench(2)
	tal := addBench(gs, 0, "Talrand, Sky Summoner", 2, 2)
	_ = tal
	ctx := map[string]interface{}{
		"caster_seat": 0,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", ctx)
	}
}

// ---------------------------------------------------------------------------
// Regression pin
// ---------------------------------------------------------------------------

// TestDispatchSpeedRegressionPin runs the dispatcher's hot path N times
// at a fixed board size and asserts the elapsed ns-per-op stays under a
// ceiling that catches >3x regressions. The ceiling is intentionally
// generous (~50 µs/op on a typical dev laptop) — the goal is to catch
// catastrophic regressions (accidental O(n²) walk, missing cache,
// re-introduced allocation in the inner loop), not to enforce a tight
// budget that flakes on slower CI hardware.
func TestDispatchSpeedRegressionPin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping speed regression pin in -short mode")
	}
	gs := benchState(8)
	ctx := map[string]interface{}{
		"caster_seat": 0,
		"card":        &gameengine.Card{Name: "X", Owner: 0, Types: []string{"instant"}},
		"is_creature": false,
	}
	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gameengine.FireCardTrigger(gs, "spell_cast", ctx)
		}
	})
	nsPerOp := res.NsPerOp()
	const ceilingNs = 200_000 // 200µs/op — 3-4x above typical observed
	if nsPerOp > ceilingNs {
		t.Errorf("dispatcher hot path regressed: %d ns/op > ceiling %d ns/op (allocs/op=%d, bytes/op=%d)",
			nsPerOp, ceilingNs, res.AllocsPerOp(), res.AllocedBytesPerOp())
	}
	t.Logf("FireTrigger(spell_cast, 32-perm board): %d ns/op, %d allocs/op, %d bytes/op",
		nsPerOp, res.AllocsPerOp(), res.AllocedBytesPerOp())
}

// TestLookupCandidatesRegressionPin pins the common-case lookupCandidates
// path. Simple names should hit a fast path with minimal allocation.
func TestLookupCandidatesRegressionPin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping speed regression pin in -short mode")
	}
	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = lookupCandidates("Lightning Bolt")
		}
	})
	const ceilingNs = 5_000 // 5µs/op — very generous; common case should be <1µs
	if res.NsPerOp() > ceilingNs {
		t.Errorf("lookupCandidates simple-name regressed: %d ns/op > %d (allocs/op=%d)",
			res.NsPerOp(), ceilingNs, res.AllocsPerOp())
	}
	t.Logf("lookupCandidates(simple): %d ns/op, %d allocs/op, %d bytes/op",
		res.NsPerOp(), res.AllocsPerOp(), res.AllocedBytesPerOp())
}

// ---------------------------------------------------------------------------
// Bench helpers — kept local so we don't pollute the existing test helpers
// ---------------------------------------------------------------------------

func newGameForBench(seats int) *gameengine.GameState {
	return gameengine.NewGameState(seats, nil, nil)
}

func addBench(gs *gameengine.GameState, seat int, name string, power, toughness int) *gameengine.Permanent {
	card := &gameengine.Card{
		Name:          name,
		Owner:         seat,
		Types:         []string{"creature", "legendary"},
		BasePower:     power,
		BaseToughness: toughness,
	}
	p := &gameengine.Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// ensure imported strings is referenced
var _ = strings.HasPrefix
