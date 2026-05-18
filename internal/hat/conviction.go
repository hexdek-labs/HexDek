package hat

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Conviction diagnostic — non-acting.
//
// Round 15 (docs/conviction-reassessment-2026-05-17.md) concluded that the
// old score-window concession is unsafe to re-enable even with the new
// archetype-aware eval weights. Before any future conviction logic can ship,
// we need data on how often each candidate trigger *would have* fired and
// whether those games were actually unwinnable.
//
// This file is that instrumentation. ShouldConcede always returns false;
// each turn we record what two candidate triggers (a score-window trigger
// matching the removed implementation, and a win-line-extinction trigger
// per the doc's Condition 2) would have decided. Samples are emitted as
// "conviction_diagnostic" events on the game log so the tournament runner
// can collect them after the game ends and the actual winner is known.

// convictionScoreWindow is the sliding-window size for the score-based
// candidate trigger, kept identical to the removed implementation so the
// diagnostic measures the same trigger.
const (
	convictionScoreWindow    = 4
	convictionScoreThreshold = -0.35
	convictionScoreMinTurn   = 10
)

// convictionSample is one turn of diagnostic data. Recorded into the
// hat's per-game history and emitted as an event so the runner can
// correlate against game outcome.
type convictionSample struct {
	Turn               int
	RelativePosition   float64
	WindowSamples      int
	ScoreTriggered     bool
	WinLineExtinct     bool
	WinLineDetail      string
	AnyTriggered       bool
}

// convictionDiagnostic is per-game state. Reset at start of every game
// by NewYggdrasilHat-like constructors (we lazily reset when we see the
// turn counter go backwards, since YggdrasilHat is reused across games
// in some test harnesses).
type convictionDiagnostic struct {
	// relPosWindow is the rolling window of recent relativePosition samples.
	relPosWindow []float64

	// lastTurn is the turn number of the most recent recorded sample.
	// When the runner reuses a hat across games, gs.Turn will drop on a
	// new game and we lazily reset state.
	lastTurn int

	// totalSamples is the number of samples recorded this game. Useful
	// for runner summaries (e.g., "trigger fired on turn N of N").
	totalSamples int

	// firstScoreTriggerTurn / firstWinLineTriggerTurn record the earliest
	// turn each candidate trigger fired (0 = never). Game-final summary
	// gets emitted with these on the last call.
	firstScoreTriggerTurn   int
	firstWinLineTriggerTurn int
}

// recordConvictionSample is called from ShouldConcede each turn. It never
// affects the return value; it only logs.
func (h *YggdrasilHat) recordConvictionSample(gs *gameengine.GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost || seat.LeftGame {
		return
	}

	if h.convictionDiag == nil {
		h.convictionDiag = &convictionDiagnostic{}
	}
	d := h.convictionDiag

	// Detect game restart on a reused hat (turn counter goes backwards).
	if gs.Turn < d.lastTurn {
		*d = convictionDiagnostic{}
	}
	d.lastTurn = gs.Turn

	relPos := h.relativePosition(gs, seatIdx)

	// Score-window candidate trigger — exact match to the removed
	// implementation: every sample in a 4-turn window below threshold,
	// only after turn 10.
	d.relPosWindow = append(d.relPosWindow, relPos)
	if len(d.relPosWindow) > convictionScoreWindow {
		d.relPosWindow = d.relPosWindow[len(d.relPosWindow)-convictionScoreWindow:]
	}
	scoreTriggered := false
	if gs.Turn >= convictionScoreMinTurn && len(d.relPosWindow) >= convictionScoreWindow {
		scoreTriggered = true
		for _, v := range d.relPosWindow {
			if v >= convictionScoreThreshold {
				scoreTriggered = false
				break
			}
		}
	}

	// Win-line extinction candidate trigger (doc Condition 2). Defined
	// as: every named win-line card (combo pieces, finishers) is in some
	// seat's exile zone, AND the deck has known win lines to begin with.
	// This is a strict, false-positive-resistant signal — see doc §"Safe-
	// to-enable subset".
	winLineExtinct, winLineDetail := h.checkWinLineExtinct(gs)

	d.totalSamples++
	anyTriggered := scoreTriggered || winLineExtinct
	if scoreTriggered && d.firstScoreTriggerTurn == 0 {
		d.firstScoreTriggerTurn = gs.Turn
	}
	if winLineExtinct && d.firstWinLineTriggerTurn == 0 {
		d.firstWinLineTriggerTurn = gs.Turn
	}

	archetype := ""
	if h.Strategy != nil {
		archetype = h.Strategy.Archetype
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "conviction_diagnostic",
		Seat:   seatIdx,
		Target: -1,
		Source: "yggdrasil",
		Amount: gs.Turn,
		Details: map[string]interface{}{
			"turn":             gs.Turn,
			"relative_position": relPos,
			"window_samples":   len(d.relPosWindow),
			"score_triggered":  scoreTriggered,
			"winline_extinct":  winLineExtinct,
			"winline_detail":   winLineDetail,
			"any_triggered":    anyTriggered,
			"archetype":        archetype,
		},
	})

	pushConvictionEvent(ConvictionEvent{
		GameSeed:         gs.Seed,
		Seat:             seatIdx,
		Turn:             gs.Turn,
		RelativePosition: relPos,
		WindowSamples:    len(d.relPosWindow),
		ScoreTriggered:   scoreTriggered,
		WinLineExtinct:   winLineExtinct,
		WinLineDetail:    winLineDetail,
		AnyTriggered:     anyTriggered,
		Archetype:        archetype,
	})
}

// checkWinLineExtinct returns true if every named win-line card from the
// hat's Strategy is currently in some seat's exile zone. Returns false
// (with explanatory detail) when the deck has no declared win lines or
// any win-line card is still recoverable (in any non-exile zone).
//
// This is intentionally strict: a single combo piece anywhere outside
// exile (library, hand, graveyard, battlefield, command, stack) defeats
// the trigger. Per the doc, even a graveyard piece is recoverable with
// the right effect.
func (h *YggdrasilHat) checkWinLineExtinct(gs *gameengine.GameState) (bool, string) {
	if h.Strategy == nil {
		return false, "no_strategy"
	}
	if len(h.Strategy.ComboPieces) == 0 && len(h.Strategy.FinisherCards) == 0 {
		return false, "no_win_lines"
	}

	// Gather every card name across the win lines we care about.
	winLine := map[string]bool{}
	for _, cp := range h.Strategy.ComboPieces {
		for _, name := range cp.Pieces {
			if name != "" {
				winLine[name] = true
			}
		}
	}
	for _, name := range h.Strategy.FinisherCards {
		if name != "" {
			winLine[name] = true
		}
	}
	if len(winLine) == 0 {
		return false, "no_win_lines"
	}

	exiled := map[string]bool{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Exile {
			if c == nil {
				continue
			}
			name := c.DisplayName()
			if winLine[name] {
				exiled[name] = true
			}
		}
	}

	if len(exiled) < len(winLine) {
		return false, "win_lines_recoverable"
	}
	return true, "all_win_lines_exiled"
}
