package tournament

import (
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/muninn"
	"github.com/hexdek/hexdek/internal/trueskill"
)

// GameOutcome is the per-game result shuttled from workers to the
// aggregator goroutine via a buffered channel.
type GameOutcome struct {
	GameIdx int

	// Winner is the seat index that won, or -1 for a draw / unresolved.
	Winner int

	// WinnerCommanderIdx is the commander (deck) index that won — i.e.
	// after un-rotating the seat. -1 for draw.
	WinnerCommanderIdx int

	// Rot is `gameIdx % NSeats`. Used by post-hoc analysis to understand
	// seat bias.
	Rot int

	// EliminationOrder records each seat's elimination order by
	// commander index. Order 0 = first eliminated, Order len-1 = winner
	// (still alive). Size == NSeats.
	EliminationOrder []int

	// Turns is how many full turns the game ran.
	Turns int

	// EndReason is the human-readable reason ("last_seat_standing",
	// "turn_cap", "turn_cap_tie", "crash").
	EndReason string

	// ModeChanges is the number of player_mode_change events observed
	// (PokerHat analytics).
	ModeChanges int

	// CrashErr is non-empty on recover()-caught panics.
	CrashErr string

	// Concessions is the number of conviction concessions in this game.
	Concessions int

	// MinRelPos is the lowest relative position any hat saw (for conviction calibration).
	MinRelPos float64

	// Board density at end of game (for timeout diagnosis).
	MaxBoardSize   int // largest single-seat battlefield
	TotalBoardSize int // total permanents across all seats

	// ParserGapSnippets is a map of engine-emitted "parser_gap" snippet
	// → count, collected from gs.EventLog. Empty unless AuditEnabled.
	ParserGapSnippets map[string]int

	// ParticipantCommanderIdxs records which commander indices were
	// present in this game (length == NSeats). Used by the matchup
	// matrix aggregator to track pairwise head-to-head records.
	ParticipantCommanderIdxs []int

	// Analysis is the per-game deep analytics result. Only populated
	// when AnalyticsEnabled is true in the TournamentConfig.
	Analysis *analytics.GameAnalysis

	// KillRecords captures who eliminated whom in this game. Only
	// populated when AuditEnabled (needs event log for inference).
	KillRecords []analytics.KillRecord

	// PostGameStats holds per-seat summary statistics emitted after
	// every game (no event log required). Light enough for bulk runs.
	PostGameStats []SeatStats

	// ConcessionRecords captures board state at the moment of each
	// conviction concession in this game.
	ConcessionRecords []muninn.ConcessionRecord
}

// SeatStats is a per-seat summary emitted after every game regardless
// of audit/analytics flags. Designed for 7174n1c's stat analysis pipeline.
type SeatStats struct {
	CommanderIdx  int
	CommanderName string
	Won           bool
	FinalLife     int
	TurnOfDeath   int
	LandsPlayed   int
	SpellsCast    int
	CreaturesOnBoard int
	TotalBoardSize   int
	HandSize      int
	GraveyardSize int
	ManaSourceCount int
	DamageDealt   int
	DamageTaken   int
	Conceded      bool
}

// TournamentResult is the aggregated result returned from Run.
//
// All counters are ORIGINAL-commander-indexed — the runner un-rotates
// winners/eliminations back to their deck index before aggregating.
type TournamentResult struct {
	Games     int
	Crashes   int
	CrashLogs []string
	Draws     int
	AvgTurns float64

	// CommanderNames is parallel to deck index.
	CommanderNames []string

	// WinsByCommander keyed by commander name -> win count. Mirrors
	// Python gauntlet_poker's `seat_wins` after un-rotation.
	WinsByCommander map[string]int

	// EliminationByCommanderBySlot[commanderName][slot] = count of
	// times that commander was eliminated in that order slot. Slot 0
	// = first out, slot NSeats-1 = winner.
	EliminationByCommanderBySlot map[string][]int

	// TotalModeChanges sums `player_mode_change` events across all
	// games. PokerHat analytics signal.
	TotalModeChanges int

	// TotalConcessions sums conviction concession events across all games.
	TotalConcessions int

	// AuditViolations is the post-run rule-auditor count. Only non-zero
	// when AuditEnabled.
	AuditViolations int

	// ParserGapSnippets aggregated across all games (empty unless
	// AuditEnabled).
	ParserGapSnippets map[string]int

	// Duration is wall-clock for the Run() call.
	Duration time.Duration

	// GamesPerSecond = successful games / Duration.
	GamesPerSecond float64

	// NSeats is the seat count the tournament used.
	NSeats int

	// --- Winrate Dashboard fields ---

	// AvgTurnToWin maps commander name to the average turn count of
	// games that commander won. Only populated for commanders with wins.
	AvgTurnToWin map[string]float64

	// TurnDistribution buckets game lengths: [0]=turns 1-5, [1]=6-10,
	// [2]=11-20, [3]=21+. Values are game counts.
	TurnDistribution [4]int

	// MatchupMatrix tracks pairwise head-to-head records.
	// MatchupMatrix[A][B] = number of games A won when both A and B
	// were in the same game. Use with MatchupGames for percentages.
	MatchupMatrix map[string]map[string]int

	// MatchupGames[A][B] = total games where both A and B participated.
	MatchupGames map[string]map[string]int

	// GamesPlayedByCommander tracks per-commander game count. In round-
	// robin tournaments, each commander only plays in a subset of games.
	// When non-nil, reports use this for per-commander winrate instead of
	// dividing by total games.
	GamesPlayedByCommander map[string]int

	// --- ELO ratings ---

	// ELO tracks per-commander ELO ratings computed across the tournament.
	// Always populated (zero overhead — one float64 update per game per pair).
	ELO []ELOEntry

	// --- TrueSkill ratings ---

	// TrueSkill tracks per-commander Bayesian ratings. Always populated.
	TrueSkill []trueskill.TrueSkillEntry

	// --- Analytics fields (populated when AnalyticsEnabled) ---

	// Analyses holds per-game deep analytics. Only populated when
	// AnalyticsEnabled is true in the TournamentConfig.
	Analyses []*analytics.GameAnalysis

	// CardRankings aggregates card performance across all analyzed games,
	// sorted by win contribution. Only populated when AnalyticsEnabled.
	CardRankings []analytics.CardRanking

	// MatchupDetails provides deep per-matchup stats. Only populated
	// when AnalyticsEnabled.
	MatchupDetails []analytics.MatchupDetail

	// KillRecords aggregates all kill events across games. Only
	// populated when AuditEnabled (needs event log for inference).
	KillRecords []analytics.KillRecord

	// ConcessionRecords tracks board state at the moment of each
	// conviction concession. Populated regardless of audit/analytics
	// flags (lightweight — only one record per conceding seat).
	ConcessionRecords []muninn.ConcessionRecord
}
