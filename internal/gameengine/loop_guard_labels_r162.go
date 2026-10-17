package gameengine

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// r162 — attribute a loop-guard firing to WHAT it fired on.
//
// r160 made the nine guards countable and the first hour of data put
// percard_dispatch_depth_cap at 30,201 firings over 33,192 games —
// continuous, not episodic, and three orders of magnitude louder than
// the game_deadline firings r160 went looking for. Every one of those
// is a per-card triggered ability that never reached the stack:
// fireTrigger returns WITHOUT dispatching any handler once
// gs.Flags["trigger_depth"] exceeds 8.
//
// A total cannot answer the question that matters. "Genuine runaway,
// correctly contained" and "ordinary trigger chain, silently
// truncated" produce the same number. The discriminator is the EVENT
// being dispatched — a `creature_etb` in 95% of games is a cap set too
// low; a single exotic event is a real loop. That event is already
// passed to LogLoopGuardFired in its details map, and already thrown
// away with the rest of the event under EventLogNone.
//
// So this counts (guard, label) pairs alongside r160's (guard) counts,
// where the label is drawn from the details map. The existing
// guards= field is deliberately untouched: it stays a stable, small,
// always-parseable column, and the attribution rides in its own field.
//
// CARDINALITY IS BOUNDED ON PURPOSE. These labels come from details
// maps written at a dozen call sites, and nothing structurally
// prevents a future one from passing something unbounded. Past
// loopGuardLabelMax distinct labels for a guard, further new labels
// collapse into one overflow bucket, so neither the map nor the log
// line can grow without limit. The bound being visible in the output
// is the point: "~other" appearing at all says the allow-list below
// picked a key it should not have.

// loopGuardLabelKeys are the details keys considered low-cardinality
// and meaningful, in priority order. "event" is the engine's
// normalized event vocabulary; "handler" is a per-card slug. Both are
// drawn from fixed sets in the source. Keys carrying a count, a depth,
// a seat or a timestamp are deliberately absent — those are unbounded
// or near-unbounded and would make the attribution useless.
var loopGuardLabelKeys = [...]string{"event", "handler"}

const (
	loopGuardLabelMax      = 64
	loopGuardLabelOverflow = "~other"
)

var (
	loopGuardLabelCounts sync.Map // "guard/label" -> *atomic.Uint64
	loopGuardLabelCard   sync.Map // guard -> *atomic.Int64, distinct labels seen
)

// loopGuardLabel picks the label for a firing, or "" when the details
// carry nothing from the allow-list. A guard with no usable label is
// simply not attributed; it still counts in r160's per-guard total.
func loopGuardLabel(details map[string]interface{}) string {
	if len(details) == 0 {
		return ""
	}
	for _, k := range loopGuardLabelKeys {
		if v, ok := details[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func bumpLoopGuardLabel(guard, label string) {
	if guard == "" || label == "" {
		return
	}
	key := guard + "/" + label
	if v, ok := loopGuardLabelCounts.Load(key); ok {
		v.(*atomic.Uint64).Add(1)
		return
	}
	// First sighting of this label. Enforce the per-guard bound before
	// creating a new entry. The check is racy under concurrency and
	// that is acceptable: the bound is a safety ceiling, not an exact
	// quota, and overshooting it by the number of live workers cannot
	// matter.
	cv, _ := loopGuardLabelCard.LoadOrStore(guard, new(atomic.Int64))
	if cv.(*atomic.Int64).Load() >= loopGuardLabelMax {
		v, _ := loopGuardLabelCounts.LoadOrStore(guard+"/"+loopGuardLabelOverflow, new(atomic.Uint64))
		v.(*atomic.Uint64).Add(1)
		return
	}
	v, loaded := loopGuardLabelCounts.LoadOrStore(key, new(atomic.Uint64))
	if !loaded {
		cv.(*atomic.Int64).Add(1)
	}
	v.(*atomic.Uint64).Add(1)
}

// LoopGuardLabelCounts snapshots every (guard, label) pair that has
// fired at least once, keyed "guard/label".
func LoopGuardLabelCounts() map[string]uint64 {
	out := map[string]uint64{}
	loopGuardLabelCounts.Range(func(k, v any) bool {
		if n := v.(*atomic.Uint64).Load(); n > 0 {
			out[k.(string)] = n
		}
		return true
	})
	return out
}

// LoopGuardLabelTop renders the n busiest pairs as a sorted,
// log-line-friendly string, heaviest first and ties broken by name so
// the output is stable across runs. It returns "" when nothing has
// fired, so the caller omits the field entirely.
func LoopGuardLabelTop(n int) string {
	counts := LoopGuardLabelCounts()
	if len(counts) == 0 || n <= 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(counts[k], 10))
	}
	return b.String()
}

// ResetLoopGuardLabelsForTest clears the label counters AND the
// cardinality bookkeeping, so a test can exercise the overflow bound
// more than once in one process.
func ResetLoopGuardLabelsForTest() {
	loopGuardLabelCounts.Range(func(k, v any) bool {
		loopGuardLabelCounts.Delete(k)
		return true
	})
	loopGuardLabelCard.Range(func(k, v any) bool {
		loopGuardLabelCard.Delete(k)
		return true
	})
}

// LoopGuardLabelsFor renders EVERY label recorded for one named guard,
// sorted heaviest first with ties broken by name, as "label:n" pairs.
// It returns "" when that guard has never fired, so the caller omits
// the field entirely.
//
// r167 — why this exists, when LoopGuardLabelTop(5) already prints
// labels. On 2026-10-17 a grinder game ran 599.9s against the 600s cap
// — the first bounded stall after r166 moved the deadline check inside
// resolveCreateToken's per-token loop. The log said
// `guards=game_deadline:1`, and that was ALL it could say: guardtop= is
// a top-5 column, and the five busiest pairs were percard dispatch
// counters in the thousands. The one firing that mattered was rank
// ~15th by volume and therefore invisible.
//
// So the question the round existed to answer — did r166's new
// token_create site fire, or did one of the three pre-existing sites
// catch it? — could not be read from the log at all, and the verdict
// had to be published as "probable" on circumstantial evidence.
//
// game_deadline fires at most once per game and is rare (five firings
// in 2.7 hours pre-fix, one in two hours post-fix), so rendering ALL of
// its labels costs a few bytes on a line that is already ~400, and only
// on the windows where a game actually hit its cap.
func LoopGuardLabelsFor(guard string) string {
	if guard == "" {
		return ""
	}
	// An exact "guard/" prefix, not a substring match: a Contains test
	// would also match a DIFFERENT guard whose own name happens to end
	// with this one, and would match the label half of every pair.
	prefix := guard + "/"
	counts := LoopGuardLabelCounts()
	keys := make([]string, 0, 4)
	for k := range counts {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		// Drop the redundant "guard/" — the field name carries it.
		b.WriteString(strings.TrimPrefix(k, prefix))
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(counts[k], 10))
	}
	return b.String()
}
