package hat

// Anti-cheat Phase 1 — statistical anomaly detection scaffolding.
//
// This file is detection + logging only; no actor takes punitive
// action on a flagged result. The contract:
//
//   - Per-contributor stats track lifetime games + a rolling
//     AnomalyRollingWindow-game window per deck.
//   - After each game the runner records the result via
//     (*AnomalyDetector).Record(deckID, won) (or the package-level
//     CheckAnomaly when only a snapshot is available).
//   - When a deck has played at least AnomalyMinGames games, its
//     win rate is compared against the population (other decks with
//     >= AnomalyMinGames each). Anything beyond ±AnomalyZScore from
//     the population mean is flagged via LogAnomaly.
//
// "Contributor" today means "deck" — a single deck on a single
// machine grinding self-play games. When the BOINC distributed-compute
// client lands, the same shape extends naturally: contributors are
// then per-machine + per-deck pairs, and the population includes
// every active grinder. The math doesn't change; the key in
// PerContributorStats does.
//
// References:
//   - docs/HexDek TODO Board.md → "Statistical anomaly detection
//     (per-contributor distribution tracking, 3σ flagging) #anticheat"
//   - docs/architecture-learning-loop.md (Heimdall seed capture is
//     the Phase 0 prerequisite)

import (
	"log"
	"math"
	"sync"
	"time"
)

// AnomalyMinGames — minimum lifetime games a deck must have played
// before it is eligible for the population-level z-score check. Below
// this threshold the win-rate sample size is too small for the test
// to mean anything: a 4-game streak of 4-1=4-W in a 4-player pod is
// a 100% win rate on n=4 and tells us nothing.
const AnomalyMinGames = 30

// AnomalyZScore — flag at >|3.0| standard deviations from the
// population mean. Under a normal distribution that's ~0.27% of
// samples, which at HexDek's grinder volume (≈30K games/min) is
// still a few false positives per minute even with no cheating —
// this is detection scaffolding, not auto-cauterize. Phase 2's
// spot-check + replay verification provides the second screen.
const AnomalyZScore = 3.0

// AnomalyRollingWindow — number of recent game results retained per
// contributor. 50 ≈ "last few minutes of grinder activity per deck"
// without unbounded memory growth, useful for diagnosing whether a
// flagged anomaly is a sudden spike (recent window diverges from
// lifetime) or a steady-state outlier.
const AnomalyRollingWindow = 50

// PerContributorStats holds the rolling win/loss history for one
// contributor (today: one deck, keyed by deckID). The lifetime
// counters drive the z-score check; the rolling window powers
// follow-up diagnostics.
type PerContributorStats struct {
	DeckID string
	Games  int
	Wins   int
	// Recent is a rolling AnomalyRollingWindow-entry slice of bools,
	// one per game in arrival order; true = win, false = loss/draw.
	Recent []bool
}

// WinRate returns the lifetime win rate. Zero when no games played.
func (s *PerContributorStats) WinRate() float64 {
	if s == nil || s.Games == 0 {
		return 0
	}
	return float64(s.Wins) / float64(s.Games)
}

// RollingWinRate returns the win rate within the rolling window.
// Falls back to lifetime rate when the window is empty.
func (s *PerContributorStats) RollingWinRate() float64 {
	if s == nil || len(s.Recent) == 0 {
		return s.WinRate()
	}
	wins := 0
	for _, w := range s.Recent {
		if w {
			wins++
		}
	}
	return float64(wins) / float64(len(s.Recent))
}

// RecordGame appends one result and trims the rolling window to
// AnomalyRollingWindow entries (drops oldest first).
func (s *PerContributorStats) RecordGame(won bool) {
	if s == nil {
		return
	}
	s.Games++
	if won {
		s.Wins++
	}
	s.Recent = append(s.Recent, won)
	if len(s.Recent) > AnomalyRollingWindow {
		s.Recent = s.Recent[len(s.Recent)-AnomalyRollingWindow:]
	}
}

// AnomalyFlag is one detected anomaly, ready to log. All fields are
// snapshotted at flag time so downstream consumers (log scrapers, the
// future spot-check pipeline) get a self-contained record.
type AnomalyFlag struct {
	DeckID    string
	WinRate   float64
	PopMean   float64
	PopStdDev float64
	ZScore    float64
	Games     int
	Detected  time.Time
}

// AnomalyDetector tracks per-contributor stats and runs the
// population-level z-score check after each game. Safe for
// concurrent callers.
type AnomalyDetector struct {
	mu    sync.Mutex
	stats map[string]*PerContributorStats
	flags []AnomalyFlag
}

// NewAnomalyDetector constructs an empty detector.
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{stats: map[string]*PerContributorStats{}}
}

// DefaultAnomalyDetector is the package-level instance the post-game
// hook uses. Tests should construct their own NewAnomalyDetector()
// to avoid cross-test state.
var DefaultAnomalyDetector = NewAnomalyDetector()

// Record updates one deck's rolling stats and runs the population
// check. Returns the AnomalyFlag if one was raised, nil otherwise.
// Mutates the detector's state.
func (d *AnomalyDetector) Record(deckID string, won bool) *AnomalyFlag {
	if d == nil || deckID == "" {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.stats[deckID]
	if !ok {
		s = &PerContributorStats{DeckID: deckID}
		d.stats[deckID] = s
	}
	s.RecordGame(won)
	return d.checkRawLocked(s.DeckID, s.WinRate(), s.Games)
}

// CheckAnomaly is the spec-defined entry point: given a deck's
// observed win rate and game count, flag if it sits beyond
// ±AnomalyZScore from the population mean (computed over decks
// with >= AnomalyMinGames each). Pure-read on the detector — does
// NOT update state, so callers can probe a hypothetical without
// touching the rolling window.
//
// Returns nil when:
//   - gameCount < AnomalyMinGames (deck not eligible),
//   - the population has fewer than 2 eligible decks (z-score
//     against a single point has zero stddev),
//   - the population stddev is zero (every deck identical),
//   - or |z| < AnomalyZScore.
func (d *AnomalyDetector) CheckAnomaly(deckID string, winRate float64, gameCount int) *AnomalyFlag {
	if d == nil || deckID == "" {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.checkRawLocked(deckID, winRate, gameCount)
}

// CheckAnomaly (package-level) routes through DefaultAnomalyDetector.
// Matches the function signature called out in the design spec; thin
// wrapper kept so tests can use NewAnomalyDetector() in isolation.
func CheckAnomaly(deckID string, winRate float64, gameCount int) *AnomalyFlag {
	return DefaultAnomalyDetector.CheckAnomaly(deckID, winRate, gameCount)
}

// PopulationMean returns (mean, n) where n is the count of decks
// with >= AnomalyMinGames games. Exported for diagnostics. Includes
// every eligible deck (no exclusion); use the leave-one-out variant
// internally for the z-score check itself.
func (d *AnomalyDetector) PopulationMean() (mean float64, n int) {
	if d == nil {
		return 0, 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.populationMeanLocked("")
}

// Stats returns a defensive copy of every tracked deck's stats.
func (d *AnomalyDetector) Stats() map[string]PerContributorStats {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]PerContributorStats, len(d.stats))
	for k, v := range d.stats {
		cp := *v
		cp.Recent = append([]bool(nil), v.Recent...)
		out[k] = cp
	}
	return out
}

// Flags returns a defensive copy of every flag raised so far.
// Persists across game runs for the lifetime of the detector;
// downstream consumers can drain it on their own cadence.
func (d *AnomalyDetector) Flags() []AnomalyFlag {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]AnomalyFlag, len(d.flags))
	copy(out, d.flags)
	return out
}

// LogAnomaly emits one structured log line per flag. Stable format
// for log-scraper consumers: `anomaly: deck=<id> wr=<float> games=<n>
// z=<sign><float>σ (<mean> mean, <stddev> stddev) — <above|below>
// population`.
func LogAnomaly(f *AnomalyFlag) {
	if f == nil {
		return
	}
	direction := "above"
	if f.ZScore < 0 {
		direction = "below"
	}
	log.Printf("anomaly: deck=%s wr=%.3f games=%d z=%+.2fσ (%.3f mean, %.3f stddev) — %s population",
		f.DeckID, f.WinRate, f.Games, f.ZScore, f.PopMean, f.PopStdDev, direction)
}

// populationMeanLocked + populationStdDevLocked iterate the eligible
// deck set twice (once each). Eligible = lifetime Games >= AnomalyMinGames.
// excludeID, when non-empty, is omitted from the population — used
// internally so the z-score check evaluates a deck against the
// "everyone else" baseline rather than letting the deck's own outlier
// rate contaminate the mean and stddev (a deck at 100% WR pulls the
// mean up so much that its own z-score collapses below threshold,
// which is the textbook leave-one-out failure mode for outlier
// detection). mu must be held.
func (d *AnomalyDetector) populationMeanLocked(excludeID string) (mean float64, n int) {
	sum := 0.0
	for k, s := range d.stats {
		if k == excludeID {
			continue
		}
		if s.Games < AnomalyMinGames {
			continue
		}
		sum += s.WinRate()
		n++
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), n
}

func (d *AnomalyDetector) populationStdDevLocked(mean float64, excludeID string) float64 {
	n := 0
	sumSq := 0.0
	for k, s := range d.stats {
		if k == excludeID {
			continue
		}
		if s.Games < AnomalyMinGames {
			continue
		}
		diff := s.WinRate() - mean
		sumSq += diff * diff
		n++
	}
	if n < 2 {
		return 0
	}
	// Bessel-corrected sample stddev (n-1) — we're estimating the
	// true population stddev from a sample of decks, not measuring
	// every grinder in existence.
	return math.Sqrt(sumSq / float64(n-1))
}

// checkRawLocked is the inner z-score routine. Builds the population
// mean + stddev, computes z, returns AnomalyFlag if |z| >= threshold.
// Side effect: appends to d.flags on a flag. mu must be held.
func (d *AnomalyDetector) checkRawLocked(deckID string, winRate float64, gameCount int) *AnomalyFlag {
	if gameCount < AnomalyMinGames {
		return nil
	}
	// Leave-one-out: exclude the deck under test from the population.
	mean, n := d.populationMeanLocked(deckID)
	if n < 2 {
		return nil
	}
	sd := d.populationStdDevLocked(mean, deckID)
	if sd <= 0 {
		return nil
	}
	z := (winRate - mean) / sd
	if math.Abs(z) < AnomalyZScore {
		return nil
	}
	flag := &AnomalyFlag{
		DeckID:    deckID,
		WinRate:   winRate,
		PopMean:   mean,
		PopStdDev: sd,
		ZScore:    z,
		Games:     gameCount,
		Detected:  time.Now(),
	}
	d.flags = append(d.flags, *flag)
	return flag
}
