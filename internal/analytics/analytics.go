// Package analytics processes game event logs to produce deep per-card,
// per-player, and per-game analysis. It answers the question "WHY did
// this deck win?" by walking the structured event stream after each game.
//
// This package is PURE -- it reads event logs and game-end state, never
// modifies game state. It is designed for batch processing: analyzing
// 1000+ game event logs should complete in well under a second.
package analytics

// GameAnalysis is the full analysis of a single completed game.
type GameAnalysis struct {
	// Per-player stats, indexed by seat.
	Players []PlayerAnalysis

	// Game-level metadata.
	WinCondition string // "combat_damage", "commander_damage", "combo", "decking", "alternate_wincon", "life_drain", "turn_cap", "timeout", "draw", "unknown"
	WinningCard  string // the card that dealt the killing blow or triggered the win
	WinnerSeat   int    // seat index of winner, -1 if draw
	TotalTurns   int
	TotalEvents  int

	// Key moments (turn number; 0 means "did not happen").
	FirstBlood     int // turn of first damage dealt to a player
	FirstDeath     int // turn of first creature death
	FirstWipe      int // turn of first board wipe (3+ creatures destroyed same turn)
	ComboAssembled int // turn a per_card_win event fired (0 if no combo)

	// MissedCombos lists turns where a known combo was live (all pieces on
	// battlefield, mana available) but was not executed. These represent Hat
	// intelligence gaps — the AI had the win available but didn't see it.
	MissedCombos []MissedCombo

	// MissedFinishers lists turns where a seat had a Freya-classified
	// finisher on board with adequate position but didn't close the game.
	MissedFinishers []MissedFinisher

	// StallIndicators flags games that exhibit board-stall patterns:
	// hit the turn cap, multiple survivors, no eliminations in late game.
	StallIndicators *StallReport
}

// StallReport describes a game that stalled — no decisive winner emerged
// before the turn cap. Like an aircraft stall warning: the game lost
// forward momentum.
type StallReport struct {
	HitTurnCap      bool   // game reached MaxTurnsPerGame
	SurvivorsAtEnd  int    // seats still alive at game end
	TurnsSinceLastKill int // turns between last elimination and game end
	LifeLeader      int    // seat index of highest life at end
	LifeLeaderTotal int    // that seat's life total
	LifeSpread      int    // difference between highest and lowest surviving life
	Cause           string // "pillow_fort", "board_parity", "low_aggression", "unknown"
}

// MissedCombo records a single instance where a known combo was available
// on a seat's battlefield but was not executed.
type MissedCombo struct {
	Seat      int      // seat index that had the combo available
	ComboName string   // human-readable combo name from KnownCombos
	Turn      int      // turn number when the combo was live
	Pieces    []string // card names that were on the battlefield
	WinType   string   // "infinite_damage", "infinite_mana", "win_game", "infinite_tokens"
	ManaAvail int      // mana available when the combo was live
}

// MissedFinisher records when a Freya-classified finisher was on the
// battlefield but its controller didn't win.
type MissedFinisher struct {
	Seat         int
	FinisherName string
	Turn         int
	BoardPower   int
	OppLifeMin   int
}

// PlayerAnalysis holds deep stats for a single player in a game.
type PlayerAnalysis struct {
	Seat          int
	CommanderName string
	Won           bool

	// Resource efficiency.
	LandsPlayed int
	ManaSpent   int
	ManaWasted  int // mana that drained unused at end of phases (pool_drain events)

	// Card performance.
	CardsPlayed []CardPerformance
	CardsInHand int // cards still in hand at game end (never cast or played)

	// Combat stats.
	AttacksDeclared int
	DamageDealt     int
	DamageTaken     int
	CreaturesLost   int
	CreaturesKilled int // opponent creatures this player killed

	// Interaction.
	SpellsCast      int
	SpellsCountered int // this player's spells that got countered
	CountersCast    int // counterspells this player cast
	RemovalCast     int // destroy/exile removal this player cast
	TriggersFired   int // total triggered abilities this player's cards fired

	// Timing.
	TurnOfDeath   int // 0 if survived
	PeakBoardSize int // max battlefield permanent count at any turn end
	PeakLife      int // highest life total reached
	FinalLife     int // life at game end
}

// CardPerformance tracks how a single card performed in a game.
type CardPerformance struct {
	Name             string
	TurnCast         int  // turn it was cast (0 if never cast / never played)
	TurnDestroyed    int  // turn it was destroyed/sacrificed (0 if survived)
	DamageDealt      int  // total damage attributed to this card
	KillsAttributed  int  // creatures killed by this card
	WasCountered     bool // was this card countered when cast?
	TimesActivated   int  // number of activated ability uses
	TriggeredCount   int  // number of triggered ability fires
	ContributedToWin bool // was this card involved in the winning play?
	IsLand           bool // true if this card was played as a land (play_land event)
	IsToken          bool // true if this card was created by a create_token event
}

// CardRanking aggregates a card's performance across multiple games.
type CardRanking struct {
	Name            string
	GamesPlayed     int     // games where this card was in a deck
	TimesCast       int     // total times cast across all games
	GamesWon        int     // games where this card's controller won
	WinContribution float64 // fraction of wins this card was involved in (0.0-1.0)
	AvgTurnCast     float64
	AvgDamageDealt  float64
	AvgKills        float64
	DeadInHandRate  float64 // fraction of games where card was never cast
	CounteredRate   float64 // fraction of cast attempts that were countered
	KillShotRate    float64 // fraction of wins where this was the kill card
}

// MatchupDetail provides deep stats for a specific commander pairing.
type MatchupDetail struct {
	Deck1         string
	Deck2         string
	Deck1WinRate  float64
	AvgTurnToWin1 float64
	AvgTurnToWin2 float64
	Deck1Wins     int
	Deck2Wins     int
	TotalGames    int
	Deck1WinsByType map[string]int // "combat_damage": 15, "life_drain": 3
	Deck2WinsByType map[string]int
	KeyCards1       []string // top 5 cards that appear in Deck1 wins
	KeyCards2       []string
}
