package hat

import (
	"sync"
	"sync/atomic"
	"time"
)

// Live-debug ring buffer for conviction diagnostic samples.
//
// recordConvictionSample emits a "conviction_diagnostic" event on the
// game log every turn, which the tournament runner harvests after the
// game ends. That batch flow is great for offline correlation against
// winners, but it gives operators no way to peek at what the conviction
// system is doing *right now*. This buffer is the live-debug counterpart:
// every sample push lands in a process-wide ring that the admin endpoint
// /api/admin/conviction-events reads back out.
//
// The buffer is intentionally small and lossy — the goal is "what has
// the hat decided in the last N samples across all running games," not
// a durable audit log. Persistent capture remains the game log's job.

// ConvictionEvent is the on-the-wire shape of a single recorded sample.
// Exported so the hexapi layer can marshal it without re-shaping.
type ConvictionEvent struct {
	// Sequence is a monotonically increasing per-process counter assigned
	// at push time. Lets clients page ("give me everything since seq=N")
	// without relying on wall-clock collisions.
	Sequence uint64 `json:"seq"`

	// CapturedAt is the wall-clock time the sample was recorded, in
	// RFC3339Nano. Useful as a human-readable timestamp; the seq field
	// is the authoritative ordering.
	CapturedAt time.Time `json:"captured_at"`

	// GameSeed is gs.Seed at the time of the sample. 0 means "unknown
	// seed" per the gameengine convention, not "seeded with 0".
	GameSeed int64 `json:"game_seed,omitempty"`

	// Seat is the seat index the hat was deciding for.
	Seat int `json:"seat"`

	// Turn is gs.Turn at sample time.
	Turn int `json:"turn"`

	// RelativePosition is the hat's score relative to opponents — same
	// number that feeds the score-window trigger.
	RelativePosition float64 `json:"relative_position"`

	// WindowSamples is the size of the rolling window at sample time
	// (saturates at convictionScoreWindow).
	WindowSamples int `json:"window_samples"`

	// ScoreTriggered / WinLineExtinct are the two candidate triggers.
	// AnyTriggered is the union, repeated for client convenience.
	ScoreTriggered bool   `json:"score_triggered"`
	WinLineExtinct bool   `json:"winline_extinct"`
	WinLineDetail  string `json:"winline_detail,omitempty"`
	AnyTriggered   bool   `json:"any_triggered"`

	// Archetype is the hat's belief about its own deck archetype at sample
	// time (Strategy.Archetype, e.g. "combo", "control"). Empty when no
	// strategy profile was loaded. Lets post-game correlation answer "did
	// the win-line-extinct trigger fire disproportionately on combo decks?"
	// without rejoining against the deck index. Optional — older clients
	// see the field absent rather than a wrong default.
	Archetype string `json:"archetype,omitempty"`
}

// convictionRingCapacity is the max number of samples retained in the
// process-wide buffer. 1024 covers ~200 turns × 4 seats × ~1 game in
// flight, with room for parallel tournaments. Bump if operators report
// the window getting evicted before they can inspect it.
const convictionRingCapacity = 1024

type convictionRing struct {
	mu       sync.RWMutex
	buf      []ConvictionEvent
	next     int    // next write index
	full     bool   // true once we've wrapped at least once
	totalSeq uint64 // monotonic sequence, also doubles as "events seen ever"
}

var globalConvictionRing = &convictionRing{
	buf: make([]ConvictionEvent, convictionRingCapacity),
}

// convictionEnabled gates the ring entirely. Defaults to enabled; tests
// or callers that want to bypass capture can flip it off. atomic.Bool
// to avoid taking the ring mutex on every sample.
var convictionEnabled atomic.Bool

func init() {
	convictionEnabled.Store(true)
}

// SetConvictionTelemetryEnabled toggles whether new samples are
// captured. Existing buffered samples are left in place.
func SetConvictionTelemetryEnabled(on bool) {
	convictionEnabled.Store(on)
}

// pushConvictionEvent is called from recordConvictionSample. It is a
// best-effort write — if the ring mutex is contended we still take it
// because the critical section is a handful of field assignments and
// the contention domain (active hats) is small.
func pushConvictionEvent(e ConvictionEvent) {
	if !convictionEnabled.Load() {
		return
	}
	r := globalConvictionRing
	r.mu.Lock()
	r.totalSeq++
	e.Sequence = r.totalSeq
	if e.CapturedAt.IsZero() {
		e.CapturedAt = time.Now().UTC()
	}
	r.buf[r.next] = e
	r.next = (r.next + 1) % len(r.buf)
	if r.next == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

// SnapshotConvictionEvents returns a copy of the buffered samples in
// chronological order (oldest first). If sinceSeq > 0, only events
// with Sequence > sinceSeq are returned. limit ≤ 0 means "all". The
// total cumulative count of events ever seen is also returned so
// callers can size their next paginated request.
func SnapshotConvictionEvents(sinceSeq uint64, limit int) (events []ConvictionEvent, totalSeen uint64) {
	r := globalConvictionRing
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalSeen = r.totalSeq
	var ordered []ConvictionEvent
	if r.full {
		ordered = make([]ConvictionEvent, 0, len(r.buf))
		ordered = append(ordered, r.buf[r.next:]...)
		ordered = append(ordered, r.buf[:r.next]...)
	} else {
		ordered = make([]ConvictionEvent, 0, r.next)
		ordered = append(ordered, r.buf[:r.next]...)
	}

	if sinceSeq > 0 {
		filtered := ordered[:0:0]
		for _, ev := range ordered {
			if ev.Sequence > sinceSeq {
				filtered = append(filtered, ev)
			}
		}
		ordered = filtered
	}

	if limit > 0 && len(ordered) > limit {
		ordered = ordered[len(ordered)-limit:]
	}
	return ordered, totalSeen
}

// TestingPushConvictionEvent is a test-only seam that pushes a synthetic
// sample into the ring without running an actual hat decision. The
// production push path (pushConvictionEvent) is unexported on purpose;
// cross-package tests use this wrapper.
func TestingPushConvictionEvent(e ConvictionEvent) {
	pushConvictionEvent(e)
}

// ResetConvictionTelemetry clears the ring. Test-only — production code
// has no reason to drop the buffer, since it self-evicts.
func ResetConvictionTelemetry() {
	r := globalConvictionRing
	r.mu.Lock()
	for i := range r.buf {
		r.buf[i] = ConvictionEvent{}
	}
	r.next = 0
	r.full = false
	r.totalSeq = 0
	r.mu.Unlock()
}
