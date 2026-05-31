package tournament

import (
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/muninn"
	"github.com/hexdek/hexdek/internal/seedcontract"
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

	// SeedContract is the per-game cryptographic commitment binding
	// inputs (RNG seed, deck keys, engine version, n seats) to the
	// outcome (winner, turns, kill method, elimination order, final
	// life). Sealed and signed in runOneGame when TournamentConfig
	// supplies a ContractKey; nil otherwise. Phase 1 anti-cheat: the
	// seed JSONL stream and downstream verifiers consume this field.
	SeedContract *seedcontract.SeedContract
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

// SchemaVersion identifies the canonical TournamentResult JSON
// schema. Bumped on breaking shape changes (field removed, semantic
// flipped, type changed). Consumers should refuse outputs from a
// version they don't recognize. See docs/tournament-output-schema.md
// for the per-mode field-population matrix and the bump policy.
const SchemaVersion = "1.0.0"

// Mode identifies which tournament entry point produced a result.
// Surfaced on TournamentResult.Mode so downstream code can switch
// on the producing path explicitly instead of guessing from which
// populated-field side effects fired. See ValidateSchema for the
// per-mode field-population invariants.
const (
	ModeRotate       = "rotate"
	ModePool         = "pool"
	ModeLazyPool     = "lazy_pool"
	ModeSwiss        = "swiss"
	ModeDoubleElim   = "double_elim"
	ModeBalancedPool = "balanced_pool"
	ModeRoundRobin   = "round_robin"
)

// TournamentResult is the aggregated result returned from every
// Run* entry point in this package. The JSON shape is canonical and
// pinned by docs/tournament-output-schema.md plus ValidateSchema.
//
// All counters are ORIGINAL-commander-indexed — the runner un-rotates
// winners/eliminations back to their deck index before aggregating.
type TournamentResult struct {
	// SchemaVersion identifies the JSON shape (semver, see
	// SchemaVersion const). Always set by the canonical constructors;
	// hand-built test fixtures may leave it empty and ValidateSchema
	// will surface the gap.
	SchemaVersion string `json:"schema_version"`

	// Mode identifies which entry point produced this result. One of
	// the Mode* constants. Always set by the canonical constructors.
	Mode string `json:"mode"`

	Games     int      `json:"games"`
	Crashes   int      `json:"crashes"`
	CrashLogs []string `json:"crash_logs,omitempty"`
	Draws     int      `json:"draws"`
	AvgTurns  float64  `json:"avg_turns"`

	// CommanderNames is parallel to deck index.
	CommanderNames []string `json:"commander_names"`

	// WinsByCommander keyed by commander name -> win count. Mirrors
	// Python gauntlet_poker's `seat_wins` after un-rotation.
	WinsByCommander map[string]int `json:"wins_by_commander"`

	// WinsBySeat[seatIdx] = count of games won by seat position
	// AFTER rotation (i.e., play order, not deck identity). With
	// fair seat rotation the expectation is len(games) / NSeats per
	// seat; deviations measure structural seat-position bias.
	// Added 2026-05-24 per 7174n1c proposal — see
	// docs/seat-bias-measurement-r60.md.
	WinsBySeat []int `json:"wins_by_seat,omitempty"`

	// WinsByCommanderBySeat[commanderName][seatIdx] = win count for
	// that commander when played from that seat position. Used to
	// derive the archetype × seat penalty matrix without re-running
	// the tournament. Sums across seatIdx equal WinsByCommander[name].
	WinsByCommanderBySeat map[string][]int `json:"wins_by_commander_by_seat,omitempty"`

	// GamesByCommanderBySeat[commanderName][seatIdx] = total games
	// played by that commander in that seat position. Needed because
	// fair rotation only holds in expectation — short runs have noise.
	// Divide WinsByCommanderBySeat by GamesByCommanderBySeat to get
	// per-(commander, seat) winrate.
	GamesByCommanderBySeat map[string][]int `json:"games_by_commander_by_seat,omitempty"`

	// EliminationByCommanderBySlot[commanderName][slot] = count of
	// times that commander was eliminated in that order slot. Slot 0
	// = first out, slot NSeats-1 = winner.
	EliminationByCommanderBySlot map[string][]int `json:"elimination_by_commander_by_slot,omitempty"`

	// TotalModeChanges sums `player_mode_change` events across all
	// games. PokerHat analytics signal.
	TotalModeChanges int `json:"total_mode_changes,omitempty"`

	// TotalConcessions sums conviction concession events across all games.
	TotalConcessions int `json:"total_concessions"`

	// AuditViolations is the post-run rule-auditor count. Only non-zero
	// when AuditEnabled.
	AuditViolations int `json:"audit_violations,omitempty"`

	// ParserGapSnippets aggregated across all games (empty unless
	// AuditEnabled).
	ParserGapSnippets map[string]int `json:"parser_gap_snippets,omitempty"`

	// Duration is wall-clock for the Run() call.
	Duration time.Duration `json:"duration_ns"`

	// GamesPerSecond = successful games / Duration.
	GamesPerSecond float64 `json:"games_per_second"`

	// NSeats is the seat count the tournament used.
	NSeats int `json:"n_seats"`

	// --- Winrate Dashboard fields ---

	// AvgTurnToWin maps commander name to the average turn count of
	// games that commander won. Only populated for commanders with wins.
	AvgTurnToWin map[string]float64 `json:"avg_turn_to_win,omitempty"`

	// TurnDistribution buckets game lengths: [0]=turns 1-5, [1]=6-10,
	// [2]=11-20, [3]=21+. Values are game counts.
	TurnDistribution [4]int `json:"turn_distribution"`

	// MatchupMatrix tracks pairwise head-to-head records.
	// MatchupMatrix[A][B] = number of games A won when both A and B
	// were in the same game. Use with MatchupGames for percentages.
	MatchupMatrix map[string]map[string]int `json:"matchup_matrix,omitempty"`

	// MatchupGames[A][B] = total games where both A and B participated.
	MatchupGames map[string]map[string]int `json:"matchup_games,omitempty"`

	// GamesPlayedByCommander tracks per-commander game count. In round-
	// robin tournaments, each commander only plays in a subset of games.
	// When non-nil, reports use this for per-commander winrate instead of
	// dividing by total games.
	GamesPlayedByCommander map[string]int `json:"games_played_by_commander,omitempty"`

	// --- ELO ratings ---

	// ELO tracks per-commander ELO ratings computed across the tournament.
	// Always populated (zero overhead — one float64 update per game per pair).
	ELO []ELOEntry `json:"elo,omitempty"`

	// --- TrueSkill ratings ---

	// TrueSkill tracks per-commander Bayesian ratings. Always populated.
	TrueSkill []trueskill.TrueSkillEntry `json:"trueskill,omitempty"`

	// --- Analytics fields (populated when AnalyticsEnabled) ---

	// Analyses holds per-game deep analytics. Only populated when
	// AnalyticsEnabled is true in the TournamentConfig.
	Analyses []*analytics.GameAnalysis `json:"analyses,omitempty"`

	// CardRankings aggregates card performance across all analyzed games,
	// sorted by win contribution. Only populated when AnalyticsEnabled.
	CardRankings []analytics.CardRanking `json:"card_rankings,omitempty"`

	// MatchupDetails provides deep per-matchup stats. Only populated
	// when AnalyticsEnabled.
	MatchupDetails []analytics.MatchupDetail `json:"matchup_details,omitempty"`

	// KillRecords aggregates all kill events across games. Only
	// populated when AuditEnabled (needs event log for inference).
	KillRecords []analytics.KillRecord `json:"kill_records,omitempty"`

	// ConcessionRecords tracks board state at the moment of each
	// conviction concession. Populated regardless of audit/analytics
	// flags (lightweight — only one record per conceding seat).
	ConcessionRecords []muninn.ConcessionRecord `json:"concession_records,omitempty"`

	// SwissStandings is populated by RunSwiss with the cumulative
	// per-deck points + win/loss/draw + bye counts across all rounds,
	// sorted by Points desc → Wins desc → CommanderName asc. Nil for
	// non-Swiss tournament runs.
	SwissStandings []SwissStanding `json:"swiss_standings,omitempty"`

	// EliminationStandings is populated by RunDoubleElimination with
	// each deck's final rank, loss count, and round-of-elimination so
	// the frontend can render the classic DE bracket placement table
	// (1st = champion, 2nd = runner-up, etc). Nil for non-DE runs.
	EliminationStandings []EliminationStanding `json:"elimination_standings,omitempty"`

	// BalancedPodPlan is populated by RunBalancedPool with the
	// snake-draft pod assignments derived from per-deck strength
	// scores. One BalancedPod per pod, in the order pods were
	// scheduled. Nil for non-balanced-pool runs.
	BalancedPodPlan []BalancedPod `json:"balanced_pod_plan,omitempty"`
}
