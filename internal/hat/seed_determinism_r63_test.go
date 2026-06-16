package hat_test

// Seed-determinism audit (r63). CR-compliance: the same game played twice
// from the same seed MUST evolve identically — same event/turn trace, same
// terminal board. This pins that against the classic nondeterminism sources
// (map-iteration order affecting trigger/effect/target ordering, wall-clock
// in a decision path, an unseeded rand). It reuses the synthetic four-seat
// self-play harness (runSelfPlay) so it runs corpus-free under the green-gate.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// detailsStr serializes an event Details map deterministically (keys sorted)
// so the trace surfaces divergence that hides in Details (e.g. an attack's
// defender seat, a block's attacker/blocker pairing).
func detailsStr(d map[string]interface{}) string {
	if len(d) == 0 {
		return ""
	}
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%v,", k, d[k])
	}
	return b.String()
}

// canonicalTrace serializes a finished game to a stable string: every event
// (kind|seat|target|source|amount) in order, then each seat's terminal state.
// Details maps are intentionally omitted — a map serializes in randomized
// order, and the kind/seat/target/source/amount tuple already pins the
// game's evolution.
func canonicalTrace(gs *gameengine.GameState) string {
	var b strings.Builder
	for _, e := range gs.EventLog {
		b.WriteString(e.Kind)
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(e.Seat))
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(e.Target))
		b.WriteByte('|')
		b.WriteString(e.Source)
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(e.Amount))
		b.WriteByte('|')
		b.WriteString(detailsStr(e.Details))
		b.WriteByte('\n')
	}
	b.WriteString("== terminal ==\n")
	for i, s := range gs.Seats {
		if s == nil {
			fmt.Fprintf(&b, "seat%d nil\n", i)
			continue
		}
		fmt.Fprintf(&b, "seat%d life=%d lost=%v won=%v left=%v bf=%d hand=%d gy=%d lib=%d\n",
			i, s.Life, s.Lost, s.Won, s.LeftGame, len(s.Battlefield), len(s.Hand), len(s.Graveyard), len(s.Library))
	}
	fmt.Fprintf(&b, "turn=%d events=%d\n", gs.Turn, len(gs.EventLog))
	return b.String()
}

// firstDivergence returns the 1-based line index where a and b first differ,
// plus the two lines, for a readable failure that points at the culprit.
func firstDivergence(a, b string) (int, string, string) {
	la := strings.Split(a, "\n")
	lb := strings.Split(b, "\n")
	n := len(la)
	if len(lb) < n {
		n = len(lb)
	}
	for i := 0; i < n; i++ {
		if la[i] != lb[i] {
			return i + 1, la[i], lb[i]
		}
	}
	if len(la) != len(lb) {
		return n + 1, fmt.Sprintf("<len %d>", len(la)), fmt.Sprintf("<len %d>", len(lb))
	}
	return 0, "", ""
}

// TestSeedDeterminism_SameSeedIdenticalTrace plays the same synthetic game
// twice from each of several fixed seeds and asserts byte-identical traces.
func TestSeedDeterminism_SameSeedIdenticalTrace(t *testing.T) {
	const turnCap = 40
	for _, seed := range []int64{1, 42, 1337, 0x5eed, 987654321} {
		a := canonicalTrace(runSelfPlay(t, seed, turnCap))
		b := canonicalTrace(runSelfPlay(t, seed, turnCap))
		if a != b {
			line, la, lb := firstDivergence(a, b)
			t.Fatalf("seed=%d: two same-seed runs DIVERGED at trace line %d:\n  run A: %q\n  run B: %q\n"+
				"a same-seed replay must be identical — a nondeterminism source leaked into the game state "+
				"(map-iteration order, wall-clock, or an unseeded rand)", seed, line, la, lb)
		}
	}
}
