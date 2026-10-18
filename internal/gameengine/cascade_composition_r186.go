package gameengine

import (
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

// r186 — what SHAPE is a capped cascade?
//
// THE QUESTION r182/r185 LEFT OPEN. capfirst= says which event a game
// FIRST capped on, once per game. Over 701 attributed capped games it
// said creature_dies in ~64% of them, and 81% of games put ALL their
// swallows inside ONE top-level cascade. That is a strong hint that
// the thing worth fixing is a single chain, but it cannot say WHICH
// chain, because three completely different failures print the same
// event name:
//
//	(a) one event type RE-ENTERING — creature_dies -> creature_dies
//	    -> ... a self-feeding loop. The fix is a loop break.
//	(b) two event types FEEDING EACH OTHER — creature_dies ->
//	    nonland_permanent_etb -> creature_dies ... This is the
//	    Gerrard, Weatherlight Hero / Yarok, the Desecrated shape
//	    (a dies-trigger that mass-returns creatures, and a literal
//	    ETB-trigger doubler). The fix is a pairing break.
//	(c) one WIDE WAVE — many distinct permanents each firing the same
//	    event ONCE, at depth, with no re-entry at all. Nothing is
//	    looping; the cap is simply set below the width of a legitimate
//	    board. The fix is the cap, or nothing.
//
// The existing per-game `mixed` flag cannot separate these. It is
// game-scoped, so two unrelated cascades in one game make it true,
// and in production ~100% of capped games are "mixed" — a field that
// is always true discriminates nothing.
//
// WHY COUNTING EVENT NAMES IS NOT ENOUGH, AND WHAT IS. (a) and (c)
// are INDISTINGUISHABLE by event name: both show exactly one distinct
// event among the cascade's swallows. The name count separates {a,c}
// from {b} and nothing more — and, as the next-but-one paragraph
// shows, it does not even manage that reliably.
//
// The second axis has to be something that is BOUNDED in a wave and
// UNBOUNDED in a loop, and the bound is the board. A wide wave is N
// permanents each causing one dispatch, so its swallow count cannot
// exceed the number of permanents in play. A self-feeding loop
// re-enters the same frontier and its swallow count has no such
// ceiling — the worst production game swallowed 3,128 dispatches,
// which no Commander board can supply as distinct permanents.
//
// So this records, per TOP-LEVEL cascade, two numbers and crosses
// them:
//
//	                  n <= boardMax        n > boardMax
//	 1 distinct event  wave      (c)        loop1     (a)
//	 2+ distinct       widemix              loopN     (b)
//
// boardMax is the largest battlefield population observed at any
// swallow IN THAT CASCADE, not at its start: a cascade that creates
// tokens as it runs genuinely has more permanents to fire, and
// sampling only the start would call every token wave a loop.
//
// (a)-VERSUS-(b) IS THE PAIR THIS CANNOT SEPARATE, AND THE SHAPE
// NAMES SAY SO. The composition is over SWALLOWED events — the names
// carried by the depth-9 dispatches the cap refused — and NOT over
// the chain that produced them. In a strictly alternating two-event
// cycle (A fires B fires A ...) with branches of equal length, every
// depth-9 attempt descends from a depth-8 node and all depth-8 nodes
// share the same parity, so exactly ONE of the two names is ever
// swallowed. A textbook Gerrard/Yarok mutual pair therefore reports
// distinct==1 and lands in the same cell as a self-feeding loop. It
// is pinned as a test rather than hidden:
// TestR186_StrictAlternationIsIndistinguishableFromSelfFeeding.
//
// So the first axis is NOT "self versus mutual" and the cells are not
// named as if it were. loop1 means "unbounded re-entry whose
// swallowed dispatches all carried one event name"; loopN means "...
// carried several". A mutual pair shows up as loopN only when its
// branches are UNEQUAL in length, which mixes the parity of the
// depth-8 frontier — common in real cascades, guaranteed in none.
// Read loopN as positive evidence of a pairing and loop1 as no
// evidence either way between (a) and (b).
//
// THE ERROR IS ONE-SIDED, AND THAT IS THE POINT. `n <= boardMax` is a
// RELIABLE NEGATIVE: a cascade that swallowed fewer dispatches than
// there were permanents to produce them cannot be an unbounded
// re-entry, so `wave` and `widemix` are trustworthy. The loop verdicts
// are a SUSPICION, not a proof: a finite cascade in which the same
// permanent legitimately fires twice, or one that outruns its own
// token production, lands in `loop1`/`loopN` without looping. The
// error cannot run the other way — a true loop's n grows without
// bound while boardMax does not — so a cascade classified `wave` is
// evidence the cap is too low, and a cascade classified `loop1` is
// a candidate for a loop hunt rather than a verdict on one.
//
// WHAT IS NOT HERE. The honest version of BOTH open pairs is the
// NESTING
// PATH — the stack of events in flight from the cascade root down to
// the cap. event@1..event@9 all equal is a loop by construction, and
// no board heuristic is needed. That stack is not available at the
// cap site and CANNOT BE MADE available within this round's cost
// budget: maintaining it means a push and a pop in fireTrigger on
// every dispatch, i.e. on the hot UNCAPPED path that r148/r163/r182
// all spent their effort keeping free of exactly that kind of work.
// r163's header is the precedent for saying so plainly rather than
// shipping a field that pretends. The board cross is what IS
// affordable, and its limits are stated above rather than papered
// over.
//
// COST, AND WHERE IT RUNS. Every line below runs inside
// NoteDepthCapSwallow — the already-rare capped branch, which 2% of
// games ever reach — or inside the `depthCapCascade != 0` arm of
// NoteCascadeRoot, which can only be entered by a cascade that
// already swallowed. The UNCAPPED dispatch path gains nothing: the
// one function it still calls, NoteCascadeRoot, now does an int
// COMPARE where it used to do an int STORE, and only takes the new
// work when that compare says the previous cascade swallowed. No
// allocation, no map, no string building anywhere.
//
// BOUNDED MEMORY. Event names are a fixed engine vocabulary in
// practice but nothing structurally forces that, so the per-cascade
// name table is a FIXED ARRAY of cascadeCompSlots entries on the
// GameState — not a map, and it cannot grow. The
// cascadeCompSlots+1'th distinct name in one cascade is not stored:
// its swallows go to an overflow counter, the cascade is marked
// name-capped, and the process-wide `namecap` cell counts such
// cascades so the ceiling is visible in the output exactly as r162's
// "~other" is. Name-capping cannot change a shape verdict — it
// implies 2+ distinct events, which is the only thing the first axis
// asks — it only makes the reported distinct count a lower bound.
//
// Clone note: CloneForRollout builds from an explicit field list, so
// a rollout clone starts with an empty composition and is classified
// as its own game. Same trade r163/r182/r185 took; grinder clones=0.

// cascadeCompSlots bounds the per-cascade distinct-event table. Four
// is deliberately tight: the shapes this file exists to separate turn
// on 1-vs-2+, and production has never shown a capped cascade carrying
// more than two events. A fifth name overflows rather than allocating.
const cascadeCompSlots = 4

// Cascade shape codes. The zero value is deliberately "unclassified"
// so a GameState that never capped reports nothing rather than
// reporting the first shape by accident.
const (
	cascadeShapeNone = iota
	cascadeShapeWave
	cascadeShapeWideMix
	cascadeShapeLoop1
	cascadeShapeLoopN
	cascadeShapeCount
)

var cascadeShapeNames = [cascadeShapeCount]string{
	"", "wave", "widemix", "loop1", "loopN",
}

// cascadeComp is the live composition of the CURRENT top-level
// cascade. Reset wholesale by NoteCascadeRoot.
type cascadeComp struct {
	names    [cascadeCompSlots]string
	counts   [cascadeCompSlots]int
	used     int  // slots filled
	overflow int  // swallows whose name did not fit a slot
	capped   bool // a distinct name was dropped for want of a slot
	boardMax int  // largest battlefield population seen in this cascade
}

// cascadePeak is the composition of the cascade that set this game's
// depthCapCascadeGameMax — the one chain worth naming in a per-game
// log line.
type cascadePeak struct {
	shape    int
	distinct int
	capped   bool
	dominant int // swallows carried by the busiest tracked event
	total    int // swallows in that cascade
	boardMax int
}

var (
	// cascadeShapeCascades counts CLASSIFIED CASCADES (not games, not
	// dispatches) by shape.
	cascadeShapeCascades [cascadeShapeCount]atomic.Uint64
	// cascadeShapeLopsided counts multi-event cascades whose dominant
	// event carried >=90% of the swallows. A loopN whose two names
	// are near-balanced is the interesting pairing shape; one that is
	// 19-to-1 is really a single-event cascade with a sprinkle, and
	// should not be counted as evidence of a pairing.
	cascadeShapeLopsided atomic.Uint64
	// cascadeShapeNameCapped counts cascades that exhausted the name
	// table. Non-zero means cascadeCompSlots is too small and the
	// distinct counts below it are lower bounds.
	cascadeShapeNameCapped atomic.Uint64
)

// battlefieldPopulation totals the permanents in play across every
// seat. O(seats) slice-length reads, no allocation; it is called only
// from the capped branch.
func battlefieldPopulation(gs *GameState) int {
	if gs == nil {
		return 0
	}
	n := 0
	for _, s := range gs.Seats {
		if s != nil {
			n += len(s.Battlefield)
		}
	}
	return n
}

// noteCascadeEvent folds one swallowed event name into the current
// cascade's composition and refreshes its board high-water mark.
// Called ONLY from NoteDepthCapSwallow.
func (gs *GameState) noteCascadeEvent(event string) {
	c := &gs.cascadeCur
	if n := battlefieldPopulation(gs); n > c.boardMax {
		c.boardMax = n
	}
	for i := 0; i < c.used; i++ {
		if c.names[i] == event {
			c.counts[i]++
			return
		}
	}
	if c.used < cascadeCompSlots {
		c.names[c.used] = event
		c.counts[c.used] = 1
		c.used++
		return
	}
	c.capped = true
	c.overflow++
}

// classify renders the finished composition as a shape plus the
// numbers that justify it.
func (c *cascadeComp) classify() cascadePeak {
	total := c.overflow
	dominant := 0
	for i := 0; i < c.used; i++ {
		total += c.counts[i]
		if c.counts[i] > dominant {
			dominant = c.counts[i]
		}
	}
	if total == 0 {
		return cascadePeak{}
	}
	distinct := c.used
	if c.capped {
		distinct++ // a lower bound; see the header
	}
	// AXIS 1: one event, or several feeding each other.
	// AXIS 2: did the cascade outrun the board that had to supply it.
	bounded := total <= c.boardMax
	var shape int
	switch {
	case distinct <= 1 && bounded:
		shape = cascadeShapeWave
	case distinct <= 1:
		shape = cascadeShapeLoop1
	case bounded:
		shape = cascadeShapeWideMix
	default:
		shape = cascadeShapeLoopN
	}
	return cascadePeak{
		shape:    shape,
		distinct: distinct,
		capped:   c.capped,
		dominant: dominant,
		total:    total,
		boardMax: c.boardMax,
	}
}

// finishCascade classifies the cascade that just ended and folds it
// into the process-wide histogram. Called ONLY from NoteCascadeRoot,
// and only when the cascade that is ending actually swallowed. The
// caller owns the reset.
//
// KNOWN UNDER-COUNT, stated rather than hidden: a cascade is
// classified when the NEXT top-level cascade starts, so a game whose
// VERY LAST top-level dispatch was a capping one contributes that one
// cascade to the per-game peak (which is updated per swallow) but not
// to the process-wide histogram. The loss is bounded by one cascade
// per game, against a median of many; closing it would need an
// end-of-game hook that mutates the GameState from a logging path,
// which r182 deliberately refused for the per-game accessors.
func (gs *GameState) finishCascade() {
	p := gs.cascadeCur.classify()
	if p.shape == cascadeShapeNone {
		return
	}
	cascadeShapeCascades[p.shape].Add(1)
	if p.distinct > 1 && p.total > 0 && p.dominant*10 >= p.total*9 {
		cascadeShapeLopsided.Add(1)
	}
	if p.capped {
		cascadeShapeNameCapped.Add(1)
	}
}

// notePeakCascade snapshots the current cascade's composition as this
// game's peak. Called from NoteDepthCapSwallow at the same moment
// r185's depthCapCascadeGameMax takes a new record.
func (gs *GameState) notePeakCascade() {
	gs.cascadePeak = gs.cascadeCur.classify()
}

// DepthCapPeakCascadeShape describes the cascade that produced this
// game's largest swallow run: the shape name, how many distinct
// events it carried, the dominant event's percentage of its swallows,
// and the largest battlefield population seen while it ran.
//
// shape is "" when this game never capped, so the caller can omit the
// fields entirely rather than printing a fabricated verdict — the
// same choice r185's CAPPED GAME line made by not existing for a
// clean game.
//
// COST. Pure field reads behind a nil check, read once per game from a
// deferred log. Nothing on any dispatch path, no reset, no side
// effect, nothing the engine can branch on.
func (gs *GameState) DepthCapPeakCascadeShape() (shape string, distinct, domPct, boardMax int) {
	if gs == nil || gs.cascadePeak.shape == cascadeShapeNone {
		return "", 0, 0, 0
	}
	p := gs.cascadePeak
	if p.total > 0 {
		domPct = p.dominant * 100 / p.total
	}
	return cascadeShapeNames[p.shape], p.distinct, domPct, p.boardMax
}

// CascadeShapeStats snapshots the process-wide cascade histogram:
// shape name -> cascades, plus the lopsided and name-capped counts.
func CascadeShapeStats() (shapes map[string]uint64, lopsided, nameCapped uint64) {
	shapes = map[string]uint64{}
	for i := cascadeShapeNone + 1; i < cascadeShapeCount; i++ {
		if n := cascadeShapeCascades[i].Load(); n > 0 {
			shapes[cascadeShapeNames[i]] = n
		}
	}
	return shapes, cascadeShapeLopsided.Load(), cascadeShapeNameCapped.Load()
}

// DepthCapShapeSummary renders the cascade histogram for the grinder
// line, or "" when no cascade has ever been classified so the caller
// omits the field entirely.
//
// It is a SEPARATE field from r182's capswallow= rather than more
// columns inside it: capswallow= counts GAMES and this counts
// CASCADES, and splicing two different denominators into one
// bracketed list is how a log line stops being readable.
//
// Shape: wave:12,loop1:88,loopN:401,lopsided:9
func DepthCapShapeSummary() string {
	shapes, lopsided, nameCapped := CascadeShapeStats()
	if len(shapes) == 0 {
		return ""
	}
	keys := make([]string, 0, len(shapes))
	for k := range shapes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if shapes[keys[i]] != shapes[keys[j]] {
			return shapes[keys[i]] > shapes[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(shapes[k], 10))
	}
	if lopsided > 0 {
		b.WriteString(",lopsided:")
		b.WriteString(strconv.FormatUint(lopsided, 10))
	}
	// Printed only when it fires: its presence is the signal that
	// cascadeCompSlots is too small, exactly as r162's "~other" is.
	if nameCapped > 0 {
		b.WriteString(",namecap:")
		b.WriteString(strconv.FormatUint(nameCapped, 10))
	}
	return b.String()
}

// ResetCascadeShapeForTest clears the process-wide cascade histogram.
func ResetCascadeShapeForTest() {
	for i := range cascadeShapeCascades {
		cascadeShapeCascades[i].Store(0)
	}
	cascadeShapeLopsided.Store(0)
	cascadeShapeNameCapped.Store(0)
}
