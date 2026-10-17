package gameengine

import (
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

// r163 — how deep does per-card trigger dispatch ACTUALLY nest?
//
// r162 attributed percard_dispatch_depth_cap's 17,046 firings to their
// events and found 87% on five ordinary ones — zone_change,
// permanent_ltb, creature_dies, permanent_etb, nonland_permanent_etb.
// That narrowed the question without answering it: an ordinary event is
// not an ordinary depth, and a genuine death-trigger loop fires
// creature_dies too.
//
// The obvious next move was a histogram of how far past 8 the depth
// runs — a tight cluster at 9 meaning a cap set below ordinary play, a
// run to hundreds meaning a real loop. THAT MEASUREMENT CANNOT EXIST.
// fireTrigger increments, tests `> 8`, and returns WITHOUT dispatching,
// so nothing nested can run and the depth observed at the cap site is
// always exactly 9. Probed with a deliberately infinite recursive
// handler: guard fired 4 times, max depth recorded 9.
//
// So the question has to be asked from the other side: not how far the
// cap is exceeded, but what depth ORDINARY games reach. If the
// distribution dies out at 3 or 4 and only a handful of games ever
// touch 9, the cap sits far above real play and its firings are real
// loops. If games routinely sit at 7 and 8, the cap is clipping
// ordinary cascades and every firing is a triggered ability that
// silently did not happen.
//
// COST. fireTrigger is a hot path, so this must not add a map lookup
// per call. The high-water mark lives in an unexported GameState FIELD
// (one int compare, inlineable) and the shared atomic is touched only
// when a game sets a NEW maximum — at most once per level, so ~9 times
// per game against ~430,000 characteristic lookups. Measured as
// invisible in a CPU profile, the same as r160's counters.
//
// READING IT. The published number for depth d is "games that reached
// AT LEAST depth d", because a game climbing to 7 passes through
// 1..7 and bumps each once. That is a survival curve, which is the
// shape that answers the question directly.
//
// Clone note: CloneForRollout builds its result from an explicit field
// list, so a rollout clone starts at hwm 0 and may re-bump levels its
// parent already counted. The grinder reports clones=0, and a clone
// re-counting a level it genuinely re-reached is not wrong for a
// survival curve. Deliberately NOT stored in gs.Flags, which would cost
// a map hash on every fireTrigger call to save nothing.

// triggerDepthMax bounds the histogram defensively. The cap in
// fireTrigger is 8 so depth cannot be observed above 9 today, but this
// must not become an unbounded array if that constant ever moves.
const triggerDepthMax = 32

var triggerDepthHits [triggerDepthMax + 1]atomic.Uint64

func bumpTriggerDepth(d int) {
	if d < 1 {
		return
	}
	if d > triggerDepthMax {
		d = triggerDepthMax
	}
	triggerDepthHits[d].Add(1)
}

// NoteTriggerDepth records that this game has reached nesting depth d.
// It is called from per_card's fireTrigger on every dispatch, so the
// common path is one comparison against a field and nothing else.
func (gs *GameState) NoteTriggerDepth(d int) {
	if gs == nil || d <= gs.triggerDepthHWM {
		return
	}
	gs.triggerDepthHWM = d
	bumpTriggerDepth(d)
}

// TriggerDepthHistogram snapshots the survival curve: depth -> number
// of games that reached at least that depth. Depths never reached are
// absent rather than zero.
func TriggerDepthHistogram() map[int]uint64 {
	out := map[int]uint64{}
	for d := 1; d <= triggerDepthMax; d++ {
		if n := triggerDepthHits[d].Load(); n > 0 {
			out[d] = n
		}
	}
	return out
}

// TriggerDepthSummary renders the curve shallowest-first as
// "1:28000,2:27000,...,9:120". Empty when nothing has been recorded, so
// the caller omits the field entirely.
func TriggerDepthSummary() string {
	h := TriggerDepthHistogram()
	if len(h) == 0 {
		return ""
	}
	depths := make([]int, 0, len(h))
	for d := range h {
		depths = append(depths, d)
	}
	sort.Ints(depths)
	var b strings.Builder
	for i, d := range depths {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(d))
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(h[d], 10))
	}
	return b.String()
}

// ResetTriggerDepthForTest clears the histogram.
func ResetTriggerDepthForTest() {
	for d := range triggerDepthHits {
		triggerDepthHits[d].Store(0)
	}
}
